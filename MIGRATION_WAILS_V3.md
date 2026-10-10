# Wails v2 → v3 迁移说明（goink v1.6.0）

本项目已从 **Wails v2.12.0** 升级到 **Wails v3.0.0-beta.28**。Go 后端、前端绑定与构建脚本均已改造并通过验证。

## 一、后端（Go）

- 入口 `main.go` 重写：
  - `options := application.Options{Name, Services, Assets, Mac}` 创建应用；
  - `application.NewService(&app.App{})` 注册服务；
  - 静态资源用 `application.BundledAssetFileServer(assets)`，保留 SPA 中间件；
  - 窗口通过 `app.Window.NewWithOptions(...)` 创建；
  - 关窗拦截改为 `win.RegisterHook(events.Common.WindowClosing, ...)`，调用服务的 `OnWindowClosing()`。
- 服务生命周期：`OnStartup` → `ServiceStartup(ctx, application.ServiceOptions)`，`OnShutdown` → `ServiceShutdown()`。
- 事件：所有 `runtime.EventsEmit(ctx, ...)` 统一改为新增包 `internal/wailsevent` 的 `Emit`（`application.Get()` 为 nil 时静默跳过，兼容纯函数测试）；`runtime.Quit(ctx)` → `wailsevent.Quit`。
- 文件对话框：改用 `application.Get().Dialog` 的链式 API（`OpenFile()` / `SaveFile()`）。
- 验证：`go build` / `go vet` / `go test` 在 `-tags gtk3` 下全部通过。

## 二、前端（React + Vite）

- 官方 v3 绑定生成到 `frontend/src/lib/bindings`（`wails3 generate bindings`，1 服务 / 118 方法 / 3 枚举 / 107 模型）。
- 为不改写 100+ 个业务文件，在原路径 `frontend/src/lib/wailsjs/` 下提供兼容层：
  - `go/app/App.ts`：保留 v2 具名方法，类型解析到 v3 模型；
  - `go/models.ts`：按 v2 包命名空间组织模型；
  - `runtime/runtime.ts`：v2 runtime 函数映射到 `@wailsio/runtime`（事件回调、窗口尺寸形状差异已处理）。
- 兼容层由 `scripts/gen_shims.mjs` 生成，可在重新生成官方绑定后重跑。
- 已安装 `@wailsio/runtime`；`tsconfig.app.json` 增加 `allowJs`。
- `[]byte` 在 v3 中为 base64 字符串，`SaveCover` / `SaveAvatar` 已在兼容层内做字节数组→base64 转换。

## 三、构建

Wails v3 使用 Taskfile 任务体系，已补充标准脚手架：

- `Taskfile.yml`（根）、`build/config.yml`、`build/Taskfile.yml`、`build/{linux,windows,darwin}/Taskfile.yml`。

常用命令：

```bash
# 开发模式（Go + Vite HMR）
wails3 dev -config ./build/config.yml

# 生产构建（Linux/GTK3）
wails3 build -tags gtk3

# 仅前端
cd frontend && npm install && npm run build

# 直接用 Go（前端产物已内置在 frontend/dist）
CGO_ENABLED=1 go build -tags gtk3 -o goink .
```

> 平台依赖：Linux 需 GTK3 + WebKit2GTK 4.1 开发库（`libgtk-3-dev`、`libwebkit2gtk-4.1-dev`）；Windows/macOS 分别按平台 Taskfile 构建。
