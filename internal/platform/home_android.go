//go:build android

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EnsureWritableHome 把 $HOME 换到应用真正可写的目录。
//
// 为什么必须存在：Android 进程里的 $HOME 是 /sdcard，而 scoped storage（API 29+）
// 之后应用不能往卷根写。config 的指针目录 ~/.goink、数据目录 ~/Goink、MCP 的
// ~/.goink/skills、git 读全局配置用的 HOME 全部来自 os.UserHomeDir()，于是一起
// 落在不可写的位置，表现就是运行期那句
// "保存配置失败: 创建配置目录失败: mkdir /sdcard/.goink: permission denied"。
// 这里只改一次 HOME，其余路径逻辑一行都不用动 —— 所以它必须是 main() 里第一件事，
// 早于 logger.Default()（logger 会往 DataDir 写 goink.log）。
//
// 目标目录按优先级：
//  1. /storage/emulated/0/Android/data/<包名>/files —— 应用专属外部目录。免权限
//     （scoped storage 只禁止别的应用访问我们的 Android/data，自己读写不受限），
//     且 adb / MTP 能看到章节 .md，用户能自己备份。
//  2. application.Android.StoragePath()（= Java activity.getFilesDir()）—— 内部私有
//     目录，个别 ROM / 工作资料档 / 主存储未挂载的机型上兜底，目录一定存在。
//
// 为什么不加一个 getExternalFilesDir 的 Java 桥方法（那是最正统的取法）：wails 的
// JNI 桥按名字反射调用 WailsBridge 的方法，但入口 androidBridgeString 是包内私有，
// 导出给应用的只有固定的几个 facility（StoragePath 就是其一）。我们新加 Java 方法
// 也调不到，所以只能自己拼路径，并用"能不能真的创建文件"当唯一判据（见 isWritableDir），
// 拼错了就自动落到第 2 个候选，不会写死在错路径上。
//
// 已知代价：两个候选都在应用私有范围内，卸载应用时被系统一并删除（Android 不提示）。
// 需要长期留存的用户数据要靠应用内导出/同步，那条路还没做。
//
// 已经有可写 HOME 的环境（Termux、带完整 Linux 的调试设备）保持原样不覆盖，
// 免得把用户自己配好的路径改掉。
func EnsureWritableHome() error {
	if home, err := os.UserHomeDir(); err == nil && home != "" && isWritableDir(home) {
		return nil
	}

	// 只问一次桥：StoragePath() 是 JNI 往返，候选拼装与试写都用这个返回值。
	filesDir := application.Android.StoragePath()

	var tried []string
	for _, dir := range candidateHomes(filesDir) {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", dir, err))
			continue
		}
		// MkdirAll 成功不等于可写：SELinux 允许建目录却拒绝建文件是真实存在的机型行为。
		if !isWritableDir(dir) {
			tried = append(tried, dir+": 目录存在但不可写")
			continue
		}
		if err := os.Setenv("HOME", dir); err != nil {
			return fmt.Errorf("设置 HOME=%s 失败: %w", dir, err)
		}
		return nil
	}
	return fmt.Errorf("找不到可写的用户目录（当前 HOME=%q；尝试过: %s）",
		os.Getenv("HOME"), strings.Join(tried, "; "))
}

// candidateHomes 按优先级给出候选目录；空字符串代表该候选拿不到，由调用方跳过。
// filesDir 是 application.Android.StoragePath() 的结果（可能为空，例如桥还没就绪）。
func candidateHomes(filesDir string) []string {
	out := make([]string, 0, 4)
	for _, pkg := range packageNames(filesDir) {
		rel := filepath.Join("Android", "data", pkg, "files")
		for _, root := range storageVolumeRoots() {
			out = append(out, filepath.Join(root, rel))
		}
	}
	// 最后兜底：应用私有内部目录。它由系统创建，必定可写。
	return append(out, filesDir)
}

// packageNames 返回包名的候选来源，按可靠性排序：
//  1. 从 getFilesDir() 反推（/data/user/0/<pkg>/files、/data/data/<pkg>/files 都成立），
//     这是系统给这个进程的真实归属，不会被 ROM 改名；
//  2. /proc/self/cmdline 里的进程名（Android 上就是包名，子进程带 ":remote" 后缀）。
//
// 两个来源都拿不到时返回空，candidateHomes 会直接退到内部目录。
func packageNames(filesDir string) []string {
	var out []string
	seen := make(map[string]bool, 2)
	add := func(pkg string) {
		if pkg == "" || seen[pkg] {
			return
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	add(packageFromFilesDir(filesDir))
	add(packageFromCmdline())
	return out
}

// packageFromFilesDir 从应用私有目录路径里取出包名段。
func packageFromFilesDir(filesDir string) string {
	if filesDir == "" {
		return ""
	}
	cleaned := filepath.Clean(filesDir)
	if filepath.Base(cleaned) != "files" {
		return ""
	}
	pkg := filepath.Base(filepath.Dir(cleaned))
	if !strings.Contains(pkg, ".") {
		return ""
	}
	return pkg
}

// packageFromCmdline 读 /proc/self/cmdline（NUL 分隔的 argv）取包名。
func packageFromCmdline() string {
	raw, err := os.ReadFile("/proc/self/cmdline")
	if err != nil {
		return ""
	}
	name := strings.TrimRight(string(raw), "\x00")
	if i := strings.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	// 子进程形如 "com.sigpanic.goink:gpu"，数据目录要用主包名。
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	if !strings.Contains(name, ".") {
		// 拿到的是 "app_process" 之类，不是反向域名形式的包名。
		return ""
	}
	return name
}

// storageVolumeRoots 主存储卷的候选挂载点。/storage/emulated/0 是近几代 Android 的
// 事实标准，但 AOSP 留了 EXTERNAL_STORAGE / ANDROID_STORAGE 给厂商改，所以三个都试，
// 由调用方用"是否真能写"判定谁是真的 —— 不猜死、不 panic。
func storageVolumeRoots() []string {
	roots := make([]string, 0, 3)
	if v := os.Getenv("EXTERNAL_STORAGE"); v != "" {
		roots = append(roots, v)
	}
	roots = append(roots, "/storage/emulated/0")
	if v := os.Getenv("ANDROID_STORAGE"); v != "" {
		roots = append(roots, filepath.Join(v, "0"))
	}
	return roots
}
