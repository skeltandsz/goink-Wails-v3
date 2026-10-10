package git

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/sigpanic/goink/internal/platform"
)

// backend 是小说仓库版本控制能力的实现缝隙。
//
// 为什么需要它：所有版本控制操作原先都直接由 `git` CLI 子进程完成（见 cliBackend），
// 但 Android（Wails v3 移动端）的 APK 里不可能带 git 可执行文件 —— 打包侧的
// .github/workflows/release-android.yml 明确不下载 MinGit/onnxruntime，且 targetSdk
// ≥29 受 Android 10 的 W^X 限制，应用连"把二进制解压到自己可写目录再 exec"都做不到
// （execve 只允许发生在 APK 解出的 nativeLibraryDir）。于是 git.New 里的
// platform.ResolveGit() 必然失败，建库/保存章节/历史/回滚全线报
// "git: 找不到 git 可执行文件"。
//
// 因此把能力面抽成接口，两个实现：
//   - cliBackend：外部 git 子进程，桌面三平台沿用，行为与历史上完全一致；
//   - goGitBackend：纯 Go 的 go-git 实现，无子进程、无 exec 权限要求，
//     在没有 git 可执行文件的环境（Android）里自动顶上。
//
// 接口方法全部小写：它是包内实现细节，`*Repo` 才是 internal/git 对外的稳定契约，
// app/ 与 internal/rollback 等调用方一律不感知后端类型，避免把选择逻辑漏到上层。
//
// 已知行为差异（go-git 相对 CLI）：
//   - revert 冲突判定更严格：只按"当前文件内容 == 该 commit 的结果内容"精确匹配，
//     不做三方合并。回滚区间 [targetTurn, lastTurn] 是连续 commit 且按时间逆序应用，
//     逐步应用后工作区状态天然等于上一个 commit 的树，所以正常流程不会触发；
//     只有在 revert 区间之外又改过同一文件时才会报冲突并整体 abort（CLI 可能合并成功）。
//   - DiffContent 的 unified diff 文本是自己生成的（表头/index 行与 `git diff --no-index`
//     不完全一致），该方法目前只被 internal/e2e 用到，前端不消费其原文。
//   - core.hooksPath / commit.gpgsign 这类"用 -c 切断全局配置"的隔离手段不需要：
//     go-git 根本不读 hook、不做签名。但反过来它也不会执行用户的 hook，这是有意的。
type backend interface {
	// name 返回后端标识（"cli" / "gogit"），只用于日志与错误信息。
	name() string

	// initFresh 在仓库目录下初始化一个非 bare 仓库，写入仓库级 user.name/user.email，
	// 创建 chapters/.gitkeep 并完成首次 commit。调用方保证目录已存在。
	initFresh(gitName, gitEmail string) error

	// setIdentity 更新仓库级 user.name / user.email。
	setIdentity(gitName, gitEmail string) error

	// stageAll 等价于 `git add -A`：新增、修改、删除全部进索引。
	stageAll() error

	// commit 提交当前索引内容，返回 commit hash。等价于 `git commit -m msg`（不含 -a）。
	commit(msg string) (string, error)

	// hasUncommitted 报告工作区/索引是否有未提交变更（含未跟踪文件），
	// 等价于 `git status --porcelain` 非空。
	hasUncommitted() (bool, error)

	// revertNoCommit 逆序 revert 所有 hash，只暂存不提交；任一步失败时自行回滚已应用的内容。
	revertNoCommit(hashes []string) error

	// revertAbort 撤销 revertNoCommit 已暂存/已写入工作区的内容。
	revertAbort() error

	// log 返回某路径（空串表示全仓库）的提交历史，按时间升序，与 CLI 后端 parseLog 一致。
	log(relPath string, n int) ([]CommitInfo, error)

	// logDetailed 返回带统计的提交历史，按时间降序（最新在前）；
	// afterHash 非空时从该 commit 的父提交开始取 n 条（分页用）。
	logDetailed(n int, afterHash string) ([]CommitInfo, error)

	// commitFileList 返回指定 commit 的信息与变更文件列表（不含内容）。
	commitFileList(hash string) (*CommitInfo, []FileEntry, error)

	// showFile 返回指定 commit 中某个文件的前后内容。
	showFile(hash, filePath string) (*FileDiff, error)

	// diffContent 对比工作区文件与 proposed 内容，返回 unified diff 文本。
	diffContent(relPath, proposed string) (string, error)
}

// newBackend 为 dir 选择可用的版本控制后端。
//
// 选择顺序：
//  1. GOINK_GIT_BACKEND 显式指定（"cli" / "gogit"）——测试与排障用的强制开关，
//     指定 cli 但找不到 git 时直接报错，不静默降级。
//  2. platform.ResolveGit() 成功 → CLI 后端（桌面行为与历史完全一致）。
//  3. ResolveGit 失败 → go-git 后端，并 Info 一条日志说明降级。
//
// GOINK_E2E_STRICT 下不做第 3 步降级：E2E 的既定语义是"必须用 bundled git 跑真实
// git 行为"（见 internal/e2e/e2e_test.go 的前置检查），悄悄换成另一个实现会让
// 通过的测试不再等于生产 CLI 路径。
func newBackend(dir string, logger *slog.Logger) (backend, error) {
	if logger == nil {
		logger = slog.Default()
	}

	switch mode := strings.ToLower(strings.TrimSpace(os.Getenv(envBackend))); mode {
	case "gogit":
		return newGoGitBackend(dir, logger), nil
	case "cli":
		gitBin, err := platform.ResolveGit()
		if err != nil {
			return nil, fmt.Errorf("git: %s=cli 需要外部 git，但 %w", envBackend, err)
		}
		return &cliBackend{dir: dir, gitBin: gitBin, logger: logger}, nil
	case "":
		// 未指定：走下面的自动选择
	default:
		return nil, fmt.Errorf("git: %s=%q 不是合法取值（只支持 cli / gogit）", envBackend, mode)
	}

	gitBin, err := platform.ResolveGit()
	if err == nil {
		return &cliBackend{dir: dir, gitBin: gitBin, logger: logger}, nil
	}
	if os.Getenv("GOINK_E2E_STRICT") != "" {
		return nil, fmt.Errorf("git: 找不到 git 可执行文件: %w", err)
	}
	logger.Info("未找到 git 可执行文件，使用内置 go-git 后端",
		"dir", dir, "err", err.Error())
	return newGoGitBackend(dir, logger), nil
}

// envBackend 强制指定后端的环境变量名，与 platform.ResolveGit 读取的
// GOINK_GIT_BIN / GOINK_E2E_STRICT 同级，供测试显式注入实现。
const envBackend = "GOINK_GIT_BACKEND"
