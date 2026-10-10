# 回滚链路已知问题

本文档记录回滚 operation_log 集成后，已知问题的修复状态。

## 问题 1：ClearParent 不走 GORM 回调 ✅ 已修复

**位置**：`app/location_view.go` UpdateLocation

**修复方案**：UpdateLocation 改回手动 `First + if + Save`，正确处理 ClearParent。
一次 Save 触发回调，operation_log 记录完整。

## 问题 2：级联删除关系不被追踪

**位置**：
- `app/character_view.go` DeleteCharacter 中级联删除 CharacterRelation
- `app/location_view.go` DeleteLocation 中清空子地点 parent + 删除关系
- `app/storyarc_view.go` DeleteStoryArc 中删除 ArcNode

**现象**：`tx.Where(...).Delete(&Relation{})` 是批量删除，Dest 是零值 struct，`getPKValues` 无法提取主键，回调跳过日志记录。

**影响**：回滚时只能恢复实体本身，无法恢复被级联删除的关系/子节点。

**修复方案**：把批量删除改为 per-row 循环——先 `Find` 查出所有受影响行，再逐行 `Delete(&row)`，每行都触发回调。

**严重程度**：中（仅在"用户删除 AI 曾操作过的实体，随后回滚"时出现，发生率低）

## 问题 3：omitempty 导致无法清空字段

**位置**：所有 `UpdateXxxInput` 结构体的 `omitempty` tag

**现象**：PATCH 语义下，`omitempty` 使 json.Marshal 跳过零值字段。用户无法通过 PATCH 将 string 字段清空为 `""` 或将 int 字段清零。

**影响**：与回滚无关，是 PATCH API 的固有限制。

**修复方案**：将 optional 字段改为指针类型（`*string`、`*int`），或添加显式的 `ClearXxx bool` flag（模仿 ClearParent 模式）。

**严重程度**：低（影响 API 功能，不影响回滚）

## 问题 4：索引列顺序 ✅ 已修复（仅改 struct tag）

**位置**：`internal/storage/operation_log.go` OperationLogRecord 的 `idx_oplog_rollback` 索引

**修复方案**：用 `priority:1`/`priority:2` 指定列顺序为 `(session_id, turn_id)`。
仅改 struct tag，未做数据库迁移（旧索引仍可用，不影响功能）。
新数据库 AutoMigrate 会创建正确顺序的索引。

## 问题 5：PatchAndSave 的 PK 防御 — 撤回（非问题）

**位置**：`internal/storage/patch.go` PatchAndSave 函数

**原始担忧**：担心 input 包含 `id` 字段会覆盖 entity 的主键。

**撤回原因**：Input 结构体（如 `UpdateCharacterInput`）的 json tag 刻意不含 `id` 字段，
`json.Marshal(input)` 产出的 JSON 不会有 `id` key，`json.Unmarshal` 不会动 entity.ID。
PK 覆盖的前提是"input 同时有 `id` 和 finder 字段"——这本身是设计错误，当前设计不会出现。
MCP 工具的 Args 同理（finder 字段用 `character_id` 而非 `id`，与 DB 模型的 `ID` 字段 json tag 不同名）。

**结论**：PK 防御是无中生有，已删除 `backupPK`/`restorePK` 和 `TestPatchAndSave_PKDefense` 测试。

## 修复优先级

| 优先级 | 问题 | 状态 |
|--------|------|------|
| P1 | ClearParent | ✅ 已修复 |
| P2 | 级联删除 | 待修复 |
| P2 | PK 防御 | ✅ 已修复 |
| P3 | 索引顺序 | ✅ 已修复（仅改 tag） |
| P3 | omitempty | 待修复（与回滚无关） |
