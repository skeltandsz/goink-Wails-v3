# sqlite-vec macOS Deprecation Warning

记录日期: 2026-07-04
来源: CI code review (#18)

## 现状

macOS CI 运行 `go test` 时出现 deprecation warning：

```
cgo-gcc-prolog:55:11: warning: 'sqlite3_auto_extension' is deprecated: first deprecated in macOS 10.10
cgo-gcc-prolog:74:11: warning: 'sqlite3_cancel_auto_extension' is deprecated: first deprecated in macOS 10.14
```

## 原因

当前使用 `github.com/asg017/sqlite-vec-go-bindings/cgo`，其 C 核心调用 `sqlite3_auto_extension` / `sqlite3_cancel_auto_extension`，这两个 API 在 macOS 上被标记为废弃（Apple 不支持进程级 auto extension），但功能正常。

## 影响评估

- **当前**：仅编译 warning，不影响功能
- **短期**：Apple 不会删除此 API（大量软件依赖）
- **长期**：如果未来 macOS 版本彻底移除，RAG 功能会初始化失败，其他功能不受影响

## 迁移方案（未来）

`github.com/viant/sqlite-vec`（v0.3.0, 2026-02 发布）是替代方案：
- 纯 Go 实现，使用 `modernc.org/sqlite`（无 CGO）
- 无 macOS deprecation 问题
- 跨平台构建更简单（不需要 CGO）

但该库目前不够成熟，等 Apple 彻底删除 API 或该库稳定后再迁移。

## 结论

当前不处理，等未来必要时再迁移。
