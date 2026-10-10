package git

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// backend 是 *Repo 之下的版本控制实现层。
//
// 为什么要抽这层：小说仓库是 goink 的核心数据路径（每本一个 git 仓库），而
// Android 上不存在 git 可执行文件（随包只带 MinGit/Linux 二进制，bionic 跑不了）。
// 原来 ResolveGit() 一失败 New() 就整体报错，表现为"保存配置失败 / failed to init
// novel repo: git: 找不到 git 可执行文件"，手机上整个功能不可用。这里留两条实现：
//
//	cliBackend     —— fork git 子进程，桌面三平台的既有行为（默认）
//	goGitBackend   —— 纯 Go 的 github.com/go-git/go-git，Android 与"找不到 git"时降级用
//
// 选择规则见 newBackend；GOINK_GIT_BACKEND 可强制指定，好让桌面 CI 也能跑 go-git 路径。
//
// 接口刻意按"能力"而非 git 子命令划分（不是 run(args) 那种字符串透传）：git 的
// CLI 输出格式会变（--format 占位符、numstat、rename 检测），把格式差异挡在实现里，
// Repo 与上层只看到 CommitInfo / FileEntry / FileDiff。
type backend interface {
	// ensureRepo 在 dir 下还不是 git 仓库时初始化并落一个 initial commit；
	// 已是仓库则只保证仓库级 user.name / user.email 存在。幂等。
	ensureRepo(gitName, gitEmail string) error
	setConfig(gitName, gitEmail string) error

	diffContent(relPath, proposed string) (string, error)
	stageAll() error
	commit(msg string) (hash string, err error)
	hasUncommitted() (bool, error)

	// revertNoCommit 逆序把若干 commit 的反向变更落到工作区+索引，不产生 commit；
	// 任一步失败要自己回滚到调用前的暂存状态（CLI 版靠 git revert --abort）。
	revertNoCommit(hashes []string) error
	// revertAbort 丢弃 revertNoCommit 的结果，回到 HEAD。
	revertAbort() error

	log(relPath string, n int) ([]CommitInfo, error)
	logDetailed(n int, afterHash string) ([]CommitInfo, error)
	commitFileList(hash string) (*CommitInfo, []FileEntry, error)
	showFile(hash, filePath string) (*FileDiff, error)
}

// backendKind 是 GOINK_GIT_BACKEND 的取值。
type backendKind string

const (
	backendAuto backendKind = ""
	backendCLI   backendKind = "cli"
	backendGoGit backendKind = "gogit"
)

// resolveBackendKind 读 GOINK_GIT_BACKEND。写错值时不静默按 auto 走：拼错环境变量
// 会让桌面开发者在以为测 go-git 的情况下实际测了 CLI，那比报错更糟。
func resolveBackendKind() (backendKind, error) {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv("GOINK_GIT_BACKEND"))); v {
	case "", "auto":
		return backendAuto, nil
	case string(backendCLI), "git", "cli-backend":
		return backendCLI, nil
	case string(backendGoGit), "go-git", "gogit-backend":
		return backendGoGit, nil
	default:
		return backendAuto, fmt.Errorf("git: 未知的 GOINK_GIT_BACKEND=%q（可用值：cli / gogit）", v)
	}
}

// newBackend 决定用哪条实现。
//
// 优先级：GOINK_GIT_BACKEND 显式指定 > 找到 git 则用 CLI > 降级 go-git。
// 降级是必须的而不是兜底摆设：Android 上 ResolveGit 必然失败，而 libwails.so 里
// 没有 PATH 上的 git，即使我们把 HOME 修到可写目录也改变不了这一点。
// gitBin 为空表示"没有可用二进制"，此时返回 goGitBackend 且不报错。
func newBackend(dir, gitBin string, logger *slog.Logger) (backend, error) {
	kind, err := resolveBackendKind()
	if err != nil {
		return nil, err
	}
	switch kind {
	case backendCLI:
		if gitBin == "" {
			// 显式要 CLI 却没有二进制：不降级，报错让人去查打包，因为 CLI 与
			// go-git 在 revert/numstat 上有细节差异，测试环境被静默换后端会误判。
			return nil, fmt.Errorf("git: GOINK_GIT_BACKEND=cli 但未找到 git 可执行文件")
		}
		return &cliBackend{dir: dir, gitBin: gitBin, logger: logger}, nil
	case backendGoGit:
		return &goGitBackend{dir: dir, logger: logger}, nil
	default:
		if gitBin != "" {
			return &cliBackend{dir: dir, gitBin: gitBin, logger: logger}, nil
		}
		return &goGitBackend{dir: dir, logger: logger}, nil
	}
}
