# Dev 分支代码审查报告

审查日期: 2026-07-04
审查范围: dev 领先 master 的全部变更（147 文件, +15579/-1687 行）
审查状态: 所有问题已通过源码验证

## 统计

| 严重程度 | 数量 | 已确认 | 部分确认 | 已否认 |
|----------|------|--------|----------|--------|
| Critical | 2 | 1 | 1 | 0 |
| High | 5 | 4 | 0 | 1 |
| Medium | 17 | 15 | 1 | 1 |
| Low | 12 | 12 | 0 | 0 |

---

## Critical

### C1. ChapterRangeInput 挂载时清空父组件选择状态 [已确认]

- **文件**: `frontend/src/components/extract/ChapterRangeInput.tsx` L43-45 + `frontend/src/components/extract/PatternExtractView.tsx` L357-362
- **分类**: React bug
- **描述**: `ChapterRangeInput` 的 `useEffect(() => { onSelect(selectedIds) }, [selectedIds, onSelect])` 在组件首次挂载时以空 Set 调用 `onSelect`。当用户在「整书」模式下点击某个章节卡片切换到「自选」模式时，父组件刚设置的「全选减一」选择状态会被此 useEffect 立即清空。用户点击章节后所有选择被清空而非排除该章节，**功能完全失效**。

### C2. ONNX Runtime 版本不匹配 [部分确认]

- **文件**: `scripts/download-onnx.sh` L6
- **分类**: 构建问题
- **描述**: 下载脚本使用 `ONNX_VERSION="1.26.0"`，但 `go.mod` 中 `onnxruntime_go v1.30.1` 在 `docs/rag-feasibility-report.md` 中记录的验证版本为 **1.25.1**。版本不一致确实存在，但 ONNX Runtime 小版本间通常向后兼容，实际运行时是否出错取决于 1.26.0 与 onnxruntime_go v1.30.1 的兼容性。存在潜在 RAG 功能运行时 ABI 不兼容风险。

---

## High

### H1. style_sample Update/Load 缺少 ID 格式验证 [已确认]

- **文件**: `internal/style_sample/store.go` L108-123
- **分类**: 安全
- **描述**: `Delete` 函数有 `idRe.MatchString(id)` 验证（`^[a-zA-Z0-9_-]+$`），但 `Update` 和 `Load` 完全没有。`Update` 使用 `filepath.Join(dirName, id+".md")` 拼接路径，恶意 id 可能导致路径穿越。虽然 `git.WriteFile` 内部有 `SafePath` 防护，但违反纵深防御原则，与 Delete 不一致。

### H2. WindowToggleMaximise 是 toggle 而非 set [已确认]

- **文件**: `frontend/src/hooks/useWindowState.ts` L16
- **分类**: 逻辑错误
- **描述**: 恢复窗口状态时调用 `WindowToggleMaximise()` 最大化窗口，但这是一个 toggle 操作。如果 OS 已将窗口最大化，调用此函数反而会取消最大化，与预期行为相反。应先通过 `WindowIsMaximised()` 检查当前状态，仅在未最大化时才调用 toggle。

### H3. GitHistoryList toggleExpand 竞态条件 [已确认]

- **文件**: `frontend/src/components/git/GitHistoryList.tsx` L107-129
- **分类**: 逻辑错误 / 并发
- **描述**: 快速连续点击两个 commit 时，先发出的 `GetCommitDiff` 请求响应会覆盖后一个 commit 的展开状态，短暂显示错误的文件列表。没有任何请求 ID 或 stale-check 机制来防止过期响应覆盖新请求的结果。

### ~~H4. InitView onInitialized 异常未捕获~~ [已否认]

- **文件**: `frontend/src/views/InitView.tsx` L69-79
- **描述**: 经验证，`onInitialized()` 在 try 块内部，异常会被 catch 捕获，不存在此问题。

### H5. useImportNovel onImported 异常未捕获 [已确认]

- **文件**: `frontend/src/hooks/useImportNovel.ts` L104
- **分类**: 逻辑错误
- **描述**: `await onImported(result)` 在 try-catch 块之外。若 `onImported` 抛出异常，将成为未处理的 Promise rejection，进度对话框不会关闭，调用者无错误反馈。

---

## Medium

### M1. app/style_sample.go 前端 API 透传未验证的 id [已确认]

- **文件**: `app/style_sample.go` L86-96
- **分类**: 安全 / API 契约
- **描述**: `GetStyleSample` 和 `UpdateStyleSample` 直接将前端传入的 id 透传给 `style_sample.Load` 和 `style_sample.Update`，未做格式校验，与 `DeleteStyleSample` 调用的 `Delete` 函数（有 idRe 校验）不一致。

### M2. epub.go resolvePath 无路径穿越防护 [已确认]

- **文件**: `internal/import/epub.go` L233-243
- **分类**: 安全
- **描述**: `resolvePath` 使用 `path.Clean` 清理路径但未检测 `..` 穿越。恶意 EPUB 的 OPF manifest href 可包含 `../../` 等，`path.Clean` 不会阻止 `..` 前缀，可能越出 EPUB 目录。后续 `zip.File.Open` 限制了实际穿越范围（仅 zip 内），但仍可能读取 zip 中非预期文件。

### M3. epub.go title 标签在 body 内被错误过滤 [已确认]

- **文件**: `internal/import/epub.go` L202
- **分类**: 逻辑错误
- **描述**: `extractHTMLText` 中 `<title>` 与 `<script>`、`<style>`、`<head>` 同在 skip 列表，`<title>` 出现在 `<body>` 内时其文本内容被完全丢弃。由于 `<head>` 已在 skip 列表，`<title>` 的单独 skip 只对 `<body>` 内的 `<title>` 产生误杀效果。

### M4. txt.go TrimLeft 误用 [已确认]

- **文件**: `internal/import/txt.go` L85
- **分类**: Bug 风险
- **描述**: `strings.TrimLeft(titleLine, "# ")` 按字符集合删除而非删除前缀。例如 `"#1 重点"` 会变成 `"重点"`（`1` 在集合 `"# "` 中被删除）。应使用 `strings.TrimPrefix` 循环去除 `# ` 前缀。

### M5. import_novel.go ImportNovel 接受前端任意文件路径 [部分确认]

- **文件**: `app/import_novel.go` L46-53
- **分类**: 安全
- **描述**: `ImportNovel` API 不验证文件路径。`imp.Parse()` 内部有扩展名校验（不支持的格式返回错误），文件不存在也会报错，但 API 层面没有路径限制，理论上可被前端传入任意路径读取文件内容。正常流程通过 `PickAndImportNovel` 的文件对话框限制，风险较低。

### ~~M6. cleanupImport 删除 Novel 但不处理 Chapters~~ [已否认]

- **文件**: `app/import_novel.go` L158-167
- **描述**: 经验证，当前流程中 `cleanupImport` 只在 Chapter 未写入 DB 或事务已回滚时被调用，不会遗漏 Chapter 记录。

### M7. PatternExtractView 异步操作缺少卸载取消保护 [已确认]

- **文件**: `frontend/src/components/extract/PatternExtractView.tsx` L51-87
- **分类**: React bug
- **描述**: 多个 `useEffect` 中的异步 API 调用（`GetNovels`、`GetChapters`、`GetModels`、`GetSettings`）无 `cancelled` 标志或 `AbortController`，组件卸载后仍会 `setState`，导致 React "update on unmounted component" 警告。

### M8. StyleSampleView 异步操作缺少卸载取消保护 [已确认]

- **文件**: `frontend/src/components/style-sample/StyleSampleView.tsx` L80-91
- **分类**: React bug
- **描述**: 与 M7 相同模式，`GetModels` 和 `GetSettings` 均无取消机制。

### M9. GitHistoryList Tooltip 位置在滚动后失效 [已确认]

- **文件**: `frontend/src/components/git/GitHistoryList.tsx` L148-178
- **分类**: UX bug
- **描述**: Tooltip 的 `tooltipRect` 仅在 `onMouseEnter` 时计算一次。若用户 hover 后滚动列表，commit 行移动但 Tooltip 保持在原始绝对位置，与行脱离。

### M10. GitHistoryList IntersectionObserver 频繁重建 [已确认]

- **文件**: `frontend/src/components/git/GitHistoryList.tsx` L80-104
- **分类**: 性能
- **描述**: `useEffect` 依赖数组包含 `commits` 和 `loadingMore`，每次加载更多 commits 导致 Observer 被销毁重建至少 3 次（loadingMore: false -> true -> false），造成不必要性能开销。应使用 `ref` 存储数据，将 Observer 生命周期与数据变化解耦。

### M11. WorkspaceView setIsMaximised 竞态 [已确认]

- **文件**: `frontend/src/views/WorkspaceView.tsx` L289
- **分类**: React bug
- **描述**: `setIsMaximised(!isMaximised)` 使用当前渲染周期的值，快速双击时第二次点击可能读取到旧状态，导致 React 状态与实际窗口状态不同步。应使用函数式更新 `setIsMaximised(prev => !prev)`。

### M12. WorkspaceView 多个 async handler 缺少 try-catch [已确认]

- **文件**: `frontend/src/views/WorkspaceView.tsx` L218-265
- **分类**: 逻辑错误
- **描述**: `handleSelectNovel`、`handleCreateNovel`、`handleCreateNovelFromDialog`、`handleUpdateNovel`、`handleDeleteNovel`、`handleExportNovel`、`handleSaveCover` 均 async 但无 try-catch，API 失败时产生 unhandled promise rejection，用户无错误反馈，UI 状态可能不一致。

### M13. GeneralConfigTab gitError 重复渲染 [已确认]

- **文件**: `frontend/src/components/settings/GeneralConfigTab.tsx` L106 + L115
- **分类**: 逻辑错误（复制粘贴 bug）
- **描述**: `gitError` 在同一行被渲染了两次——一次在左侧 div，一次在按钮右侧。当 `gitError` 存在时用户会看到同一错误消息在同一行出现两次。

### M14. frontend-checks.yml 路径过滤器遗漏配置文件 [已确认]

- **文件**: `.github/workflows/frontend-checks.yml` L7-9
- **分类**: CI 问题
- **描述**: 路径过滤器仅包含 `frontend/src/**`、`package.json`、`package-lock.json`，遗漏 `eslint.config.js`、`tsconfig.app.json`、`frontend/scripts/**`、`vite.config.ts` 等配置文件，修改这些文件后 CI 不会运行前端检查。

### M15. Node.js 版本在不同工作流间不一致 [已确认]

- **文件**: `test.yml` L17 vs `frontend-checks.yml` L28 vs `release.yml` L21/77/121
- **分类**: 构建问题
- **描述**: `test.yml` 使用 Node 22，`frontend-checks.yml` 和 `release.yml` 使用 Node 24，同一项目不同 CI 流水线使用不同 Node.js 大版本，可能导致 ESLint 行为差异和依赖解析不一致。

### M16. 下载脚本使用第三方代理 ghproxy.net [已确认]

- **文件**: `scripts/download-onnx.sh` L46 + `scripts/download-git.sh` L16
- **分类**: 安全
- **描述**: 两个脚本在 GitHub 直连失败时使用 `ghproxy.net` 作为 fallback 镜像。该代理是第三方服务，存在中间人攻击风险。脚本虽有 HTML 页面检测，但无法防御精心构造的恶意二进制文件。ONNX 共享库和 MinGit 可执行文件被篡改后果严重。

### M17. release.yml sha256sum glob 扩展失败 [已确认]

- **文件**: `.github/workflows/release.yml` L167
- **分类**: CI 问题
- **描述**: `sha256sum *.{exe,AppImage,dmg}` 在某个扩展名无匹配文件时，bash 会将 `*.exe` 作为字面量传给 sha256sum，导致 "No such file or directory" 错误。应添加 `shopt -s nullglob` 或改用 `sha256sum *`。

---

## Low

### L1. style_sample 文件名清洗与 pattern 模块不一致 [已确认]

- **文件**: `app/style_sample.go` L199-204 vs `internal/pattern/extract.go` L762-774
- **描述**: `ExtractStyle` 只过滤 `/ \` `:` 三个字符，而 `sanitizeFileName` 过滤 9 个字符（含 `* ? " < > |`），还会 TrimSpace 和提供默认值。两处功能类似但清洗策略不一致，`ExtractStyle` 的宽松清洗可能导致 Windows 上非法文件名字符通过。

### L2. style_sample ID 基于 UnixNano 无唯一性保证 [已确认]

- **文件**: `internal/style_sample/store.go` L62
- **描述**: `Create` 使用 `time.Now().UnixNano()` 作为 ID，高并发或时钟回拨场景下存在 ID 冲突可能。冲突时 `git.WriteFile` 会静默覆盖已有文件。桌面单用户应用实际风险很低。

### L3. yaml.Marshal 错误被忽略 [已确认]

- **文件**: `internal/style_sample/store.go` L253
- **描述**: `fmBytes, _ := yaml.Marshal(&fm)` 显式忽略错误。如果序列化失败，`fmBytes` 为 nil，生成的文件 frontmatter 为空，后续 `load` 解析时 Name 和 Tags 将为零值。

### L4. epub.go 使用标准库 log 而非项目统一 slog [已确认]

- **文件**: `internal/import/epub.go` L8, L105
- **描述**: 使用 `log.Printf` 而非项目其他模块统一使用的 `slog` 结构化日志，日志格式不一致，无法被日志系统统一收集过滤。

### L5. epub.go 只取第一个 rootfile [已确认]

- **文件**: `internal/import/epub.go` L153
- **描述**: `parseContainerXML` 在 `container.xml` 包含多个 `rootfile` 时只取第一个。双格式 EPUB（含 renditions）可能有多个 rootfile，第一个可能指向固定版式而非流式版式。

### L6. style_sample.List 静默跳过失败文件 [已确认]

- **文件**: `internal/style_sample/store.go` L48-50
- **描述**: `List` 在 `load` 返回错误时 `continue` 跳过但不记录日志，用户无法感知某些素材文件加载失败，可能误以为数据丢失。

### L7. git.CommitDiff 忽略文件读取错误 [已确认]

- **文件**: `internal/git/repo.go` L381, L409-414
- **描述**: 多处 `content, _, _ := r.runInDir("show", ...)` 忽略错误，`git show` 失败时 content 为空字符串，用户无法区分"文件确实为空"和"读取失败"。

### L8. pattern callTool 重试无等待间隔 [已确认]

- **文件**: `internal/pattern/extract.go` L510-559
- **描述**: LLM 未调用目标工具时立即重试，无退避机制，连续重试可能导致 API 限流。`attempts` 通常为 1-2，影响有限。

### L9. StyleSampleView models 使用 any[] 类型 [已确认]

- **文件**: `frontend/src/components/style-sample/StyleSampleView.tsx` L35
- **描述**: `useState<any[]>([])` 应使用后端 `AvailableModel[]` 类型，`any` 绕过了类型检查，`m.Key`/`m.ModelName` 属性访问无类型安全保障。

### L10. StyleSampleView 变量遮蔽 [已确认]

- **文件**: `frontend/src/components/style-sample/StyleSampleView.tsx` L200
- **描述**: `.map((t: string) => ...)` 中参数 `t` 遮蔽了 `useTranslation()` 解构的 `t` 函数，后续在该回调内添加翻译调用将产生难以察觉的 bug。

### L11. WorkspaceView ExtractWorkspaceView 始终挂载 [已确认]

- **文件**: `frontend/src/views/WorkspaceView.tsx` L413-415
- **描述**: 使用 CSS `hidden` 类控制显隐而非条件渲染，`ExtractWorkspaceView` 始终挂载，其 `usePatternProgress` hook 和 `StyleSampleView` 中的 `EventsOn` 监听器在非活跃标签页时仍持续运行。

### L12. Makefile clean 在原生 Windows 上不可用 [已确认]

- **文件**: `Makefile` L66-67
- **描述**: `clean` 目标使用 `rm -rf`，原生 Windows PowerShell/CMD 中不可用。其他目标如 `package` 有 `uname -s` 平台检测，`clean` 没有。

---

## 建议优先修复顺序

1. **C1** — ChapterRangeInput 清空选择状态（功能完全失效）
2. **H2** — WindowToggleMaximise 竞态（可能导致启动时窗口状态错误）
3. **H5** — useImportNovel onImported 异常未捕获
4. **H1** — style_sample ID 验证缺失（安全漏洞）
5. **M4** — TrimLeft 误用（经典 Go 陷阱）
6. **M13** — gitError 重复渲染（明显的复制粘贴 bug）
7. **C2** — ONNX 版本不匹配（确认兼容性）
