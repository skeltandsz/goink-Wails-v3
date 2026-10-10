# Dev 分支代码审查报告（第二轮）

审查日期: 2026-07-04
审查范围: dev 领先 master 的全部变更（173 文件, +19096/-2006 行）
审查方式: 4 个并行 review agent 覆盖后端、前端、CI/脚本、i18n，后逐条源码验证
审查状态: 24 条初始发现，3 条否认，7 条部分确认，14 条确认

## 统计

| 严重程度 | 初始 | 确认 | 部分确认 | 否认 |
|----------|------|------|----------|------|
| CRITICAL | 2 | 1 | 1 | 0 |
| WARNING | 18 | 10 | 5 | 3 |
| INFO | 4 | 3 | 1 | 0 |

---

## CRITICAL

### #2. WebFetchCard count 传入字符串导致英文复数化失效 [确认]

- **文件**: `frontend/src/components/chat/WebFetchCard.tsx` L59
- **分类**: i18n bug
- **描述**: `t('chat.pageContent', { count: wordCount.toLocaleString() })` 将数字通过 `.toLocaleString()` 转为字符串（如 `"1,234"`），而 i18next 复数化依赖 `count` 为 number 类型。传入字符串后 i18next 无法匹配 `_one` 分支，英文环境下 "1 char" 永远显示为 "1 chars"。
- **影响**: 英文环境复数形式错误，中文不受影响（无复数形态）
- **修复方向**: 改为 `t('chat.pageContent', { count: wordCount })`，在翻译模板中处理格式化

### #1. GitHistoryList toggleExpand 闭包过期 [部分确认]

- **文件**: `frontend/src/components/git/GitHistoryList.tsx` L128-165
- **分类**: React bug
- **描述**: `toggleExpand` 入口处读取 `expandedHash`（闭包捕获），快速双击同一 commit 时可能读到旧值。但 `await` 之后的守卫检查（L143 `if (expandedHash !== hash) return`）缓解了大部分问题。
- **影响**: 仅在快速连续点击时可能出现状态不一致，严重性低于初审判断
- **修复方向**: 使用 ref 存储 `expandedHash` 最新值，在异步回调中读取 ref

---

## WARNING

### #14. ModelConfigTab saveMsg 中文串匹配在英文环境失效 [确认]

- **文件**: `frontend/src/components/settings/ModelConfigTab.tsx` L243
- **分类**: i18n bug
- **描述**: `saveMsg.includes('失败') || saveMsg.includes('测试')` 通过中文字符判断消息类型，英文环境下永远不匹配，失败消息显示为绿色成功样式。
- **修复方向**: 使用独立 state 标记保存成功/失败，而非依赖字符串内容

### #13. GitHistoryList tooltip 定位硬编码 17rem [确认]

- **文件**: `frontend/src/components/git/GitHistoryList.tsx` L191-197
- **分类**: UX bug
- **描述**: tooltip `left` 硬编码为 `17rem`（ActivityBar 3rem + SidePanel 14rem），但 SidePanel 支持拖拽调整宽度（180-480px），实际宽度可能不是 14rem。
- **修复方向**: 将实际侧栏宽度作为 prop 传入，或用 DOM 查询获取侧栏位置

### #10. SessionHistory timeAgo 使用固定 now 快照 [确认]

- **文件**: `frontend/src/components/chat/SessionHistory.tsx` L18
- **分类**: React bug
- **描述**: `const [now] = useState(() => Date.now())` 将 now 固定为组件首次渲染时刻，之后永不更新。长时间停留后相对时间显示逐渐失真。
- **修复方向**: 添加定时器（如每分钟）更新 now，或在组件可见时刷新

### #12. useLayoutState 拖拽时频繁同步写 localStorage [确认]

- **文件**: `frontend/src/hooks/useLayoutState.ts` L28-34
- **分类**: 性能
- **描述**: useEffect 在每次 state 变化后同步写入 localStorage，拖拽期间每秒可能触发数十次。
- **修复方向**: 对 localStorage 写入添加 debounce（如 300ms）

### #4. operation_log 缺少 (table_name, entity_id, id) 复合索引 [部分确认]

- **文件**: `internal/storage/operation_log.go` L58-59, L242-246
- **分类**: 性能
- **描述**: 现有索引 `idx_oplog_entity(table_name, entity_id)` 缺少 `id` 列，`resolveTurnForEntity` 的 `ORDER BY id DESC` 需要额外 filesort。但此查询只在无 TurnInfo 的前端 CRUD 时触发，频率不高。
- **修复方向**: 将索引扩展为 `(table_name, entity_id, id)` 或添加覆盖索引

### #5. epub.go io.ReadAll 无大小限制 [部分确认]

- **文件**: `internal/import/epub.go` L168, L196
- **分类**: 安全
- **描述**: `parseContainerXML` 和 `parseOPF` 使用 `io.ReadAll` 无大小限制。但仅用于读取 zip 内的小型元数据文件（container.xml 和 content.opf），实际章节内容通过 `html.Parse` 流式解析。恶意 epub 可构造巨大 XML 致 OOM，但风险很低。
- **修复方向**: 使用 `io.LimitReader(f, maxOPFSize)` 限制读取量（如 1MB）

### #6. style.Service 声明 RWMutex 但从未使用 [确认]

- **文件**: `internal/style/service.go` L30
- **分类**: 死代码
- **描述**: `Service` 结构体声明了 `mu sync.RWMutex`，但所有方法（List/Create/Delete/Update/Load/ComputeStats）均未加锁。当前是单进程桌面应用，并发风险极低。
- **修复方向**: 删除未使用的 `mu` 字段，或在各方法中使用它

### #7. ComputeStats 循环变量 s 遮蔽 receiver [确认]

- **文件**: `internal/style/service.go` L171, L188
- **分类**: 代码质量
- **描述**: `for _, s := range samples` 和 `for i, s := range sentences` 中变量 `s` 遮蔽了 receiver `s *Service`。当前循环体内不需访问 receiver，功能不受影响，但 Go vet 会报告此警告。
- **修复方向**: 将循环变量改名为 `sent` 或 `sentence`

### #8. PatchAndSave First+Save 非原子操作 [部分确认]

- **文件**: `internal/storage/patch.go` L32-53
- **分类**: 数据一致性
- **描述**: `PatchAndSave` 先 `First` 加载再 `Save` 保存，两步之间无事务保护，理论上存在 lost update 风险。但这是桌面单进程应用，并发概率极低。
- **修复方向**: 如需严格处理，用 GORM 事务包装 First+Save

### #15. StyleView models 使用 any[] 类型 [确认]

- **文件**: `frontend/src/components/style/StyleView.tsx` L34
- **分类**: TypeScript 类型安全
- **描述**: `const [models, setModels] = useState<any[]>([])` 使用 `any[]`，`m.Key`/`m.ModelName` 属性访问无类型保障。
- **修复方向**: 定义接口或从后端类型导入，替代 `any[]`

### #18. macOS CI 缺少 sqlite3 头文件安装 [确认]

- **文件**: `.github/workflows/test.yml` L39-45
- **分类**: CI 问题
- **描述**: test.yml matrix 包含 `macos-latest`，但仅在 Windows 条件下安装 sqlite3 headers。macOS runner 可能缺少 `libsqlite3-dev` 头文件，CGO 编译 `mattn/go-sqlite3` 时可能失败。
- **修复方向**: 为 macOS runner 添加 sqlite3 头文件安装步骤

### #20. check_omitempty 正则在循环内反复编译 [确认]

- **文件**: `scripts/check_omitempty/main.go` L64, L122, L138, L147
- **分类**: 性能
- **描述**: 四个 `regexp.MustCompile` 调用在循环路径内，每次迭代重新编译正则。
- **修复方向**: 将正则提升为包级变量，只编译一次

---

## INFO

### #21. CancelExtract 与 CancelExtractPattern 取消键语义不一致 [确认]

- **文件**: `app/style.go` L162-163 vs `app/pattern_api.go` L29, L53
- **描述**: `CancelExtract` 使用裸 `TaskID` 作为 key，未使用已定义的 `CancelPrefixExtract` 前缀；而 `CancelExtractPattern` 使用了 `CancelPrefixPattern` 前缀。功能正确但语义不一致，未来可能产生 key 冲突。

### #22. i18next.d.ts TFunction override 使 key 类型检查失效 [部分确认]

- **文件**: `frontend/src/i18n/i18next.d.ts` L27-43
- **描述**: `Key extends string = string` 使所有 `t()` 调用接受任意字符串，`TypedKeys` 未被 TFunction 签名引用。但注释表明这是有意的权衡（支持动态 key），且 `CustomTypeOptions.resources` 仍提供自动补全。

### #24. ESLint --max-warnings 100 阈值宽松 [确认]

- **文件**: `.github/workflows/test.yml` L23
- **描述**: `npx eslint . --max-warnings 100` 允许 100 个 warning 通过，几乎等于无约束，warning 会不断累积。

---

## 否认的发现

| # | 问题 | 否认原因 |
|---|------|----------|
| #3 | PatchAndSave 主键可被覆盖 | 代码已有 backupPK/restorePK 机制，且 Input 结构体无 ID/NovelID 字段 |
| #11 | isMaximised 乐观更新与窗口不同步 | hook 中无乐观更新模式，`isMaximised` 来自 localStorage 恢复逻辑 |
| #19 | 下载脚本移除镜像回退 | BGE 模型下载仍保留 hf-mirror.com 镜像回退，未被移除 |

---

## 与第一轮 review 的关系

本轮发现的 #1（GitHistoryList 闭包）对应第一轮 H3 的同类问题（已修复 commit `94fd5a0`），但本次验证的是 `toggleExpand` 入口处闭包，非 `GetCommitDiff` 的 stale response，是不同的问题点。

本轮其余发现均为新发现，与第一轮不重叠。

## 建议优先修复顺序

1. **#2** — WebFetchCard count 类型错误（英文复数化完全失效）
2. **#14** — ModelConfigTab 中文串匹配（英文环境错误样式）
3. **#13** — tooltip 硬编码位置（侧栏调整后 tooltip 偏移）
4. **#10** — SessionHistory 时间不更新（长时间使用体验差）
5. **#12** — localStorage 写入无防抖（拖拽性能）
6. **#6** — 删除未使用 RWMutex 或实际使用
7. **#7** — 循环变量遮蔽 receiver（Go vet 警告）
