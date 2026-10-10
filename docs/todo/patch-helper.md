# PATCH 辅助函数 — 让前端 CRUD 触发 GORM 回调

## 问题

前端 CRUD（`UpdateCharacter`、`UpdateLocation` 等）用了 `Model().Where().Updates(&input)`，这种写法：
- GORM `Update` 回调链虽然触发，但 `Dest` 中只有 input 结构体，没有完整 entity
- `fetchOldRow` 拿不到主键 → 无法查旧值 → 无法写 operation_log
- `serializeDest` 只序列化 input 结构体 → new_values 不完整

详见 `rollback-review.md #5`。

## MCP 工具已用的模式

参考 `internal/mcp_tools/character_tools.go:236-256`：

```go
// 1. First 加载当前完整行
var ch character.Character
tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.CharacterID, tc.NovelID).First(&ch)

// 2. RawArgs (LLM 传入的 JSON) Unmarshal 到 entity → 只覆盖传入字段
json.Unmarshal(tc.RawArgs, &ch)

// 3. Save → 触发 GORM Create/Update 回调，拿到完整 old_values + new_values
tc.DB.WithContext(ctx).Save(&ch)
```

核心：`json.Unmarshal` 只覆盖 JSON 中出现的字段，没出现的保持原值。恰好等价于 PATCH 语义。

## 前端适用方案

前端 input 结构体已有 `omitempty` tag，`Marshal` 后自然只包含非零字段：

```go
func Patch[T any](db *gorm.DB, id int64, input any, entity *T) error {
    // 1. 加载当前行
    if err := db.First(entity, id).Error; err != nil {
        return err
    }
    // 2. input (patch) → JSON → entity (只覆盖有值的字段)
    data, _ := json.Marshal(input)
    json.Unmarshal(data, entity)
    // 3. Save 触发 GORM 回调
    return db.Save(entity).Error
}
```

前端 CRUD 调用处改为：

```go
func (a *App) UpdateCharacter(novelID int64, charID int64, input UpdateCharacterInput) error {
    var ch character.Character
    if err := Patch(a.db.WithContext(a.ctx), charID, &input, &ch); err != nil {
        return fmt.Errorf("update character: %w", err)
    }
    return nil
}
```

## 注意

- input 中必须有 `omitempty` tag，否则零值字段也会被序列化 → Unmarshal 覆盖 entity → 不符合 PATCH 语义
- 前端 input 不包含主键字段（ID 通过函数参数传）→ 不会意外覆盖 entity.ID，比 MCP 工具的 RawArgs 更安全
- 已用 `Updates()` 的方法（~10 个）需要逐个改成此模式

## 已使用 `Save()` 的方法

以下方法已经触发 GORM 回调，不需要改：
- `app/novel.go:CreatePreference` — `Save()`
- `app/novel.go:UpdatePreference` — `Save()`
- `app/chapter.go:CreateChapter` — `Create()`
- `app/chapter.go:UpdateChapterTitle` — `Save()`

## 待改方法

| 方法 | 当前写法 |
|---|---|
| `app/character_view.go:UpdateCharacter` | `Updates()` |
| `app/location_view.go:UpdateLocation` | `Updates()` |
| `app/location_view.go:DeleteLocation` | `Delete()` ✅ 已触发回调 |
| `app/timeline_view.go:UpdateChapterPlan` | `Save()` ✅ 已触发 |
| `app/timeline_view.go:CreateTimelineEntry` | `Create()` ✅ 已触发 |
| `app/timeline_view.go:UpdateTimelineEntry` | `Updates()` |
| `app/storyarc_view.go:CreateStoryArc` | `Create()` ✅ 已触发 |
| `app/storyarc_view.go:UpdateStoryArc` | 待确认 |
| `app/storyarc_view.go:CreateArcNode` | `Create()` ✅ 已触发 |
| `app/storyarc_view.go:UpdateArcNode` | 待确认 |
| `app/storyarc_view.go:DeleteStoryArc` | `Delete()` ✅ 已触发 |
| `app/storyarc_view.go:DeleteArcNode` | `Delete()` ✅ 已触发 |
| `app/reader.go:UpdateReaderPerspective` | 待确认 |
| `app/reader.go:DeleteReaderPerspective` | `Delete()` ✅ 已触发 |
