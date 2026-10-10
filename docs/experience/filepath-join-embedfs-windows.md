# filepath.Join 导致 embed.FS 在 Windows 上失败

## 现象

Windows 构建版所有内置 skill 加载失败，日志显示：

```
skill: 读取 builtin\\brainstorm-composer.md 失败: open builtin\\brainstorm-composer.md: file does not exist
```

## 根因

`internal/skill/parse.go:109` 在扫描内置 skill 时使用 `filepath.Join(dir, entry.Name())` 拼接路径。Windows 上 `filepath.Join` 使用反斜杠 `\`，但 Go 的 `embed.FS` 强制要求正斜杠 `/`（即使 Windows 平台也如此）。

```go
// 错误：Windows 上产生 "builtin\\foo.md"
ParseFS(fsys, filepath.Join("builtin", "foo.md"))

// 正确：始终产生 "builtin/foo.md"
ParseFS(fsys, "builtin"+"/"+"foo.md")
```

## 教训

涉及 `fs.FS` 接口（包括 `embed.FS`、`os.DirFS` 等）的路径拼接，必须用正斜杠 `/`，不能依赖 `filepath.Join`。`io/fs` 包的文档明确：

> Path names passed to Open are UTF-8, unrooted, slash-separated sequences.

- Go 标准库的 `embed.FS` 内部存储路径始终用 `/`
- `os.DirFS` 在 Windows 上也接受 `/`
- `fs.ValidPath()` 拒绝 `\` 分隔符

## 同类案例：EPUB 导入

`internal/import/epub.go` 中 `resolvePath` 函数同样使用了 `filepath.Join` + `filepath.Clean` 处理 ZIP 内部路径。

```go
// 错误：Windows 上 EPUB 路径 "OEBPS/Text/chapter.html" 被错误转成 "OEBPS\Text\chapter.html"
filepath.Join("OEBPS/Text", "chapter.html")
filepath.Clean("OEBPS/Text/chapter.html")

// 正确：始终用 /
path.Join("OEBPS/Text", "chapter.html")
path.Clean("OEBPS/Text/chapter.html")
```

ZIP 格式（以及所有继承了 ZIP 路径规范的格式，如 EPUB、DOCX）内部路径强制使用 `/`。在 Windows 上用 `filepath` 包处理这些路径会导致：

- `filepath.Dir("OEBPS/content.opf")` → `"OEBPS/content.opf"`（无法正确提取目录）
- `filepath.Join("OEBPS", "chapter.html")` → `"OEBPS\\chapter.html"`（zip reader 找不到）
- `filepath.Base("OEBPS/chapter.html")` → `"OEBPS/chapter.html"`（无法提取文件名）

修复方式：涉及 ZIP/EPUB 内部路径时，统一使用 `path` 包（`path.Join`、`path.Dir`、`path.Base`、`path.Clean`）或手动 `/` 拼接。

## 通用原则

凡是操作**跨平台标准格式内部的路径**（`embed.FS`、`archive/zip`、URL 路径段等），必须用 `path` 包或手动 `/`，不能用 `filepath`。`filepath` 仅适用于**操作系统本地文件系统路径**。
