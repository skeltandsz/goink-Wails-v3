package main

import (
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sigpanic/goink/app"
	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/logger"
	"github.com/sigpanic/goink/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	log := logger.Default()
	// 将项目 logger 注册为 slog 全局默认：让未注入 logger 的代码（agentcfg 包级 slog.Warn、
	// git/skill/update 包的 nil-fallback 等）也走项目 logger 配置（写 goink.log + stderr、
	// AddSource=true、Debug 级）。生产路径仍以依赖注入为主（App/Agent/MCP 工具显式拿 logger），
	// SetDefault 作为漏注入代码的兜底。紧接 logger.Default() 后调用，时序最干净。
	slog.SetDefault(log)

	if lib, err := platform.ResolveOnnxLib(); err == nil {
		ort.SetSharedLibraryPath(lib)
		log.Info("ONNX 运行库已设置", "path", lib)
	} else {
		log.Warn("未找到 ONNX Runtime 库，向量检索将不可用", "err", err)
	}

	wapp := app.New(log)

	// 自定义资源服务中间件：拦截 /covers/<novelID> 与 /avatar 两个本地图片请求，
	// 其余请求交给默认的静态资源处理器。逻辑与 v2 版本保持一致。
	assetMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if idStr, ok := strings.CutPrefix(r.URL.Path, "/covers/"); ok {
				novelID, err := strconv.ParseInt(idStr, 10, 64)
				if err != nil || novelID <= 0 {
					http.NotFound(w, r)
					return
				}
				coverPath := filepath.Join(config.DataDirPath(), "novels",
					strconv.FormatInt(novelID, 10), "cover.jpg")
				http.ServeFile(w, r, coverPath)
				return
			}
			if r.URL.Path == "/avatar" {
				avatarPath := filepath.Join(config.DataDirPath(), "user", "avatar.jpg")
				http.ServeFile(w, r, avatarPath)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	wailsApp := application.New(application.Options{
		Name: "Goink",
		Services: []application.Service{
			application.NewService(wapp),
		},
		Assets: application.AssetOptions{
			Handler:    application.BundledAssetFileServer(assets),
			Middleware: assetMiddleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Goink",
		Width:     1400,
		Height:    900,
		MinWidth:  900,
		MinHeight: 600,
		Frameless: runtime.GOOS != "darwin", // macOS 用原生标题栏
	})

	// 关窗确认（替代 v2 的 OnBeforeClose）：初始化期间拦截一次关闭并请求前端弹窗。
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if wapp.OnWindowClosing() {
			e.Cancel()
		}
	})

	err := wailsApp.Run()
	if err != nil {
		log.Error("应用退出", "err", err)
		os.Exit(1)
	}
}
