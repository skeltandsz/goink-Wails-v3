// Package wailsevent 封装 Wails v3 的事件发射与应用退出，供 app / agent / mcp_tools
// 等不直接持有 *application.App 的代码使用。
//
// v2 中这些代码通过 wails runtime 的 context 调用 EventsEmit；v3 改为从
// application.Get() 取得全局应用再发射。单元测试环境下没有应用实例，此时静默跳过，
// 替代 v2 里 ctx.Value("events") 的判空逻辑。
package wailsevent

import "github.com/wailsapp/wails/v3/pkg/application"

// Emit 发射一个自定义事件。data 为 0 个参数时事件数据为 nil，1 个参数时即该数据，
// 多个参数时数据为参数切片（与 application.EventManager.Emit 语义一致）。
// 无 Wails 运行时（单元测试）时静默跳过。
func Emit(name string, data ...any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, data...)
}

// Quit 请求退出整个应用。无 Wails 运行时（单元测试）时静默跳过。
func Quit() {
	app := application.Get()
	if app == nil {
		return
	}
	app.Quit()
}
