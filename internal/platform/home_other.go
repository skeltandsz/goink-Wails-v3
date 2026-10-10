//go:build !android

package platform

// EnsureWritableHome 在非 Android 平台是空操作：桌面/移动 Web 的 $HOME 本来就
// 可写（Windows 走 LOCALAPPDATA 分支，见 resolveDataDir）。保留同名函数是为了
// main() 里那句调用不用加构建标签，Android 的真实实现在 home_android.go。
func EnsureWritableHome() error { return nil }
