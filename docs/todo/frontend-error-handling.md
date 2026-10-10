# 前端错误处理现状与改进计划

## 背景

2026-07-05 审查发现 WorkspaceView 的 novel CRUD handler 用 `catch (err) { console.error(err) }` 吞掉错误，
导致 NovelEditDialog 和 NovelDeleteDialog 内置的 `setError()` 永远无法触发。
已修复：改为 `catch (err) { console.error(err); throw err }`，错误可冒泡到 Dialog 展示。

## 现状分类

### 1. 内部完整处理（有 try/catch + setError + finally）

用户可见错误提示，体验良好。

- CharacterListView — Create/Update/Delete
- LocationListView — Create/Update/Delete
- ReaderView — Create/Update/Delete
- PreferenceView — Create/Update/Delete
- TimelineView — Create/Update/Delete/UpdateChapterPlan
- ArcListView — Create/Update/Delete（故事弧 + 节点，共 7 个写操作）
- ModelConfigTab — SaveLLMConfig
- StyleView — Create/Update/Save/Extract（handleDelete 除外）
- PatternExtractView — Save/Extract

### 2. 内部静默吞错（有 try/catch 但无 UI 反馈）

操作失败后用户看不到任何提示，不知道是否成功。

- ~~CharacterList — DeleteCharacter，catch 注释"静默失败，主视图会处理"~~ → 已修复
- ~~LocationList — DeleteLocation，catch 注释"静默失败"~~ → 已修复
- ~~SkillList — DeleteSkill，仅 console.error~~ → 已修复
- StyleView — DeleteStyleSample，仅 console.error（暂不修改，其他人在重构）

### 3. 已修复（原裸 await 或吞错）

- [x] ContentPanel.doSave — 加 try/catch + toast.error（sonner），保存失败时保留 isDirty 不清
- [x] ChapterList.loadChapters — 加 try/catch + 空状态区域红色错误提示 + 重试按钮
- [x] ChapterList.handleCreateChapter — 加 try/catch + 创建表单下方红色提示
- [x] ChapterList.commitEdit — 加 try/catch + toast.error（行内编辑重命名失败时 toast 提示）
- [x] ProfileView.handleFileChange — 加 try/catch + 头像下方红色提示
- [x] ProfileView.handleNameSave — 加 try/catch + 昵称 input 下方红色提示，失败时保留编辑状态
- [x] WorkspaceView.handleCreateNovelFromDialog — re-throw → NovelEditDialog 展示
- [x] WorkspaceView.handleUpdateNovel — re-throw → NovelEditDialog 展示
- [x] WorkspaceView.handleDeleteNovel — re-throw → NovelDeleteDialog 展示
- [x] WorkspaceView.handleExportNovel — 加 try/catch + re-throw → ExportDialog 展示
- [x] 引入 sonner toast 库，App.tsx 配置 `<Toaster position="bottom-right" richColors />`
- [x] CharacterList.DeleteCharacter — 加 toast.error
- [x] LocationList.DeleteLocation — 加 toast.error
- [x] SkillList.DeleteSkill — 加 toast.error + i18n key

### 4. WorkspaceView 中保留原样的操作

- handleSelectNovel — console.error（无对应 Dialog，切换失败不影响用户）
- handleCreateNovel — console.error（老版内联创建，无 Dialog）
- handleSaveCover — console.error（无对应 Dialog）

## 改进计划

### 优先级中：静默吞错组件加用户可见提示

- [x] CharacterList.DeleteCharacter — 加 toast.error
- [x] LocationList.DeleteLocation — 加 toast.error
- [x] SkillList.DeleteSkill — 加 toast.error + i18n key
- [ ] StyleView.DeleteStyleSample — 统一使用 setError（与同组件其他操作一致，暂不修改，其他人在重构）

### 优先级低：统一错误提示方式

已引入 sonner toast 库，后续可逐步统一：
- 编辑器保存失败等一次性操作用 toast
- 表单验证错误用内联红色文字
- Dialog 内操作失败用 Dialog 内部错误展示
- 网络错误、权限错误等通用错误统一处理
