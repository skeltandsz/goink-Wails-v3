# 回滚全链路代码审查 — 待解决项

## 1. `RollbackBeforeTurn` 硬编码 `context.Background()`

**位置**: `internal/rollback/rollback.go:53`
```go
storage.RollbackInTx(context.Background(), tx, sessionID, targetTurn, lastTurn)
```

**问题**: 传入的是全新的 `context.Background()`，父 context 的所有值（tracing、logger 注入等）全部丢失。

**方案**: 把 `storage` 包的 `withoutTurn` 导出为公共函数（如 `storage.WithoutTurn`），改为 `storage.RollbackInTx(storage.WithoutTurn(ctx), tx, ...)`，既剥离 TurnInfo 又透传其他 context 值。

**优先级**: 低

---

## 2. 回滚期间无并发保护

**位置**: `internal/rollback/rollback.go`（整体），`app/chat.go`（Chat 入口）

**问题**: 
- Rollback 执行期间（尤其是 git revert 阶段耗时不定），同 session 的新 Chat 可能通过 `loadOrCreateSession` + `NextTurn` 启动
- 会导致 `last_turn_id` 在 rollback 读完之后被递增，`active_version` 计算不一致

**方案**: 
- 在 `App` 层加互斥锁（如 `sync.Mutex`），Rollback 前 Lock()，结束 Unlock()
- Chat 入口（`app/chat.go:Chat`）检查同一把锁，发现 rollback 进行中直接返回错误
- 更精细的可以是 `sync.RWMutex`：读不受限，写（rollback）互斥

**优先级**: 中

---

## 3. 回滚时工作树不干净导致 `revert --no-commit` 失败

**位置**: `internal/git/repo.go:RevertNoCommit`

**问题**: 如果用户在回滚前手动编辑了章节文件（未提交），`git revert --no-commit` 会因为 dirty tree 失败。

**方案**: stash 策略
1. `git stash push -um "rollback-pre-stash"` 保存所有未提交 + 未跟踪变更
2. 执行回滚（工作树干净，revert 不会冲突）
3. `git stash pop` 还原用户修改
   - pop 有冲突 → 标准 git 冲突标记，用户逐段选择保留/放弃
   - 如果回滚本身失败（步骤 2 报错）→ 同样 stash pop 把用户修改拿回来

**边界 case**: pop 冲突时用户修改保留（`<<<<<<< Updated upstream` 标记），AI 的修改被 revert。

**优先级**: 中

---

## 4. `cleanupTurnCommits` 无影响行数检查

**位置**: `internal/rollback/store.go:37-39`

**问题**: `tx.Where(...).Delete(&TurnCommit{})` 在条件未匹配到行时不会报错。如果 `turn_commits` 因为某种原因已经空了、但 git 有 commit：
- `ListForRollback` 返回空 → `hashes` 为空 → git revert 跳过
- DB 事务照常执行（回滚 DB 但 git 没动）
- 两边不一致

**方案**: 如果 `len(hashes) > 0`，在 `cleanupTurnCommits` 后检查 `RowsAffected` 应等于 `len(hashes)`，不一致时打 warning 日志。

**优先级**: 低

---

## 5. 前端 CRUD 不记录 operation_log → 回滚后 DB/git 不一致 ★★★

**这是最严重的问题**

### 现状

前端 CRUD 方法（`CreateCharacter`、`UpdateCharacter`、`DeleteCharacter`、`CreateLocation`、`UpdateLocation`、`DeleteTimelineEntry` 等）使用的 context 是 `a.ctx`（App 根 context），**没有 TurnInfo**。

GORM 回调 `afterCreate`/`afterUpdate`/`afterDelete` 中 `getTurnInfo()` 返回 false → **直接 return，完全不写入 operation_log**。

### 后果

| 层 | AI 修改 | 用户前端 CRUD |
|---|---|---|
| **git** | `revert --no-commit` 全退 | 不涉及（CRUD 不改文件） |
| **DB** | operation_log 有记录 → 全退 | 无记录 → **原地保留** |

### 冲突场景举例

1. 场景 A（用户修改被连带删除）：
   - Turn 4: AI 创建角色 Alice（operation_log: create）
   - 用户手动编辑 Alice 描述（无 operation_log）
   - 回滚 → 逆向 create → DELETE Alice → 用户的编辑连带丢失

2. 场景 B（DB 残留 vs git 回退）：
   - 用户手动创建角色 Bob（无 operation_log）
   - AI 在章节文件里写了 Bob 出场
   - 回滚 → git 还原了章节（删了 Bob 出场），但 DB 里 Bob 还在

### 延伸问题：批量更新不走回调

部分 CRUD 方法用了 `Model().Where().Updates()`，**不走 GORM create/update/delete 回调**（`operation_log.go` 注释第 72-75 行已自述）。比如：
- `app/character_view.go:60` — `UpdateCharacter`
- `app/location_view.go:67` — `UpdateLocation`
- `app/timeline_view.go:111` — `UpdateTimelineEntry`

即使加了 TurnInfo，这些方法也不会触发回调。

### 可选方案

#### 方案 A：前端 CRUD 也走 operation_log（推荐）

放开 GORM 回调的 TurnInfo 限制，无条件记录 operation_log，没有 TurnInfo 的记 `turn_id=0`。回滚时 `RollbackInTx` 只操作 `[targetTurn, lastTurn]` 区间，`turn_id=0` 的记录不受影响。

需要修改：
- 去除 `getTurnInfo` 检查（或增加无 TurnInfo 时的 fallback 路径）
- 将 `UpdateCharacter` 等从 `Model().Where().Updates()` 改为 `Save()` 方式以触发回调
- 或者为这些批量更新场景注册独立的 `BeforeUpdate`/`AfterUpdate` 回调（但 GORM 批量更新不走这些回调，需要注册到 `Update` 的回调链而不是 `Save`/`Updates` 回调链——需要验证）

#### 方案 B：允许无 TurnInfo 也记录，回滚自然过滤

GORM 回调始终记录操作日志，不检查 TurnInfo。`RollbackInTx` 的 WHERE 条件是 `turn_id >= ? AND turn_id <= ?`，`turn_id=0` 的记录自然被排除。

优点：改动最小（只改回调）。
缺点：
- 仍然需要处理 `Model().Where().Updates()` 不触发回调的问题
- operation_log 会膨胀（记录所有前端 CRUD 操作）

#### 方案 C：不改 operation_log，前端 CRUD 参与回滚时不撤销

定义"用户手动 CRUD 不被对话回滚影响"为预期行为。接受回滚后 DB 和 git 可能不一致的状态：
- git 还原到过去 → 用户手动在 DB 中创建/修改的实体仍然存在但不被章节引用
- 用户需要手动清理这些"孤儿"实体

优点：零改动。
缺点：语义不干净，可能造成用户困惑。

### 相关文件

- `app/character_view.go` — Create/Update/Delete Character
- `app/location_view.go` — Create/Update/Delete Location
- `app/timeline_view.go` — Update Chapter Plan / Create/Update/Delete Timeline
- `app/storyarc_view.go` — Create/Update/Delete StoryArc / ArcNode
- `app/reader.go` — Create/Update/Delete ReaderPerspective
- `app/novel.go` — Create/Update/Delete Preference
- `app/chapter.go` — Create/Update Chapter
- `internal/storage/operation_log.go` — GORM 回调（RegisterOplogHooks / afterCreate / afterUpdate / afterDelete）

**优先级**: 高

---

## 6. 回滚范围语义未定义

**问题**: 回滚到底"只退 AI 改的"还是"退 AI+User 全部"？

**已提交的 commit（在回滚区间内）**: 建议全部 revert，不分 AI/user。因为 `turn_commits` 不区分，operation_log 也不区分同一个 turn 里 AI 工具和用户操作的 DB 变更。

**未提交的工作树修改**: 通过 stash 保留（见第 3 项）。

**优先级**: 高（需要确认决策）

---

## 7. Wails 绑定未暴露

**位置**: `app/` 目录（无 `RollbackToTurn` 方法）

**问题**: 回滚引擎已完整实现 `rollback.RollbackBeforeTurn`，但没有通过 Wails 绑定暴露给前端。

**方案**: 新增 `func (a *App) RollbackToTurn(sessionID string, targetTurn int) error`：
1. 并发检查（见第 2 项）
2. `HasUncommitted` → stash（见第 3 项）
3. 打开 git repo
4. 调用 `rollback.RollbackBeforeTurn`
5. `stash pop`
6. EventsEmit 通知前端刷新

**优先级**: 需配合步骤 3（前端）一起做

---

## 已确认无问题的设计

- `reverseOne` 用 `map[string]any` 做 Updates → 零值正确恢复
- `RevertNoCommit` 错误路径自动 abort → 不会留下半 revert 脏状态
- `withoutTurn` 防止逆向操作递归日志 → 正确
- `afterCreate` 的 UPSERT 检测 → 自然键实体（chapter_plans）回滚正确
- 仓库级 git config（`git.Repo.New` 设置 user.name / user.email）→ 不污染全局 gitconfig
- `sessions.last_turn_id` 不回退 → turn ID 空洞无害只是不连续
- `skipOperLog` 排除 sessions/novels/writing_log → 回滚范围合理
