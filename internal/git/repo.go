package git

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sigpanic/goink/internal/config"
)

// Repo 管理单部小说的 Git 仓库，提供文件读写和版本控制。
//
// Repo 只做参数/结果转发，具体实现由 backend 决定（外部 git CLI 或内置 go-git，
// 选择逻辑见 newBackend）。对 app/ 与 internal/ 的调用方而言，Repo 的导出方法
// 签名与抽出 backend 之前完全一致。
type Repo struct {
	dir     string
	backend backend
	logger  *slog.Logger
}

// CommitInfo 是 git log 的单条记录。
type CommitInfo struct {
	Hash         string    `json:"hash"`
	ShortHash    string    `json:"shortHash"`
	Message      string    `json:"message"`
	Time         time.Time `json:"time"`
	AuthorName   string    `json:"authorName"`
	AuthorEmail  string    `json:"authorEmail"`
	FilesChanged int       `json:"filesChanged"`
	Insertions   int       `json:"insertions"`
	Deletions    int       `json:"deletions"`
}

// FileDiff 表示一次 commit 中单个文件的变更。
type FileDiff struct {
	Path            string `json:"path"`
	ChangeType      string `json:"changeType"`
	OriginalContent string `json:"original"`
	ModifiedContent string `json:"modified"`
}

// FileEntry 表示 commit 中一个变更文件的简要信息（不含内容）。
type FileEntry struct {
	Path       string `json:"path"`
	ChangeType string `json:"changeType"` // "added" | "modified" | "deleted" | "renamed"
	OldPath    string `json:"oldPath"`    // 仅 renamed 时有值，表示重命名前的路径
}

// New 为指定小说打开已有仓库，不存在则 git init + 首次空 commit。
// gitName 和 gitEmail 用于设置仓库级 git config。空字符串时使用默认值。
// logger 为 nil 时使用 slog.Default()。
//
// 找不到外部 git 可执行文件不再视为致命错误：Android 等环境会自动改用内置
// go-git 后端（见 newBackend），建库/历史/回滚照常工作。
func New(novelID int64, gitName, gitEmail string, logger *slog.Logger) (*Repo, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("git: config not initialized")
	}

	if gitName == "" {
		gitName = "Goink"
	}
	if gitEmail == "" {
		gitEmail = "goink@local"
	}

	if logger == nil {
		logger = slog.Default()
	}

	dir := config.NovelDirPath(novelID)
	be, err := newBackend(dir, logger)
	if err != nil {
		return nil, err
	}
	r := &Repo{dir: dir, backend: be, logger: logger}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("git: stat .git: %w", err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("git: create novel dir: %w", err)
		}
		if err := be.initFresh(gitName, gitEmail); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// BackendName 返回当前生效的后端标识（"cli" / "gogit"），供日志与诊断使用。
func (r *Repo) BackendName() string { return r.backend.name() }

// SetGitConfig 在已有仓库上更新仓库级 user.name / user.email 配置。
func (r *Repo) SetGitConfig(gitName, gitEmail string) error {
	return r.backend.setIdentity(gitName, gitEmail)
}

// ── Diff ──────────────────────────────────────────────────

// DiffContent 对比当前工作区文件与 proposed 内容，返回 unified diff。
// 文件不存在时以空内容为基准。
func (r *Repo) DiffContent(relPath, proposed string) (string, error) {
	return r.backend.diffContent(relPath, proposed)
}

// ── Git 操作 ──────────────────────────────────────────────

func (r *Repo) StageAll() error {
	return r.backend.stageAll()
}

func (r *Repo) Commit(msg string) (string, error) {
	return r.backend.commit(msg)
}

func (r *Repo) HasUncommitted() (bool, error) {
	return r.backend.hasUncommitted()
}

// RevertNoCommit 逆序 revert 所有 hash，仅暂存不提交。冲突时自动 abort。
// 调用方负责最终 commit（或 abort）。
func (r *Repo) RevertNoCommit(hashes []string) error {
	return r.backend.revertNoCommit(hashes)
}

// RevertAbort 取消进行中的 revert，丢弃所有暂存的 revert 内容。
func (r *Repo) RevertAbort() error {
	return r.backend.revertAbort()
}

func (r *Repo) Log(relPath string, n int) ([]CommitInfo, error) {
	return r.backend.log(relPath, n)
}

// LogDetailed 返回带详细统计的提交历史，按时间降序（最新在前）。
// afterHash 非空时，返回该 commit 之后的更早提交。
func (r *Repo) LogDetailed(n int, afterHash string) ([]CommitInfo, error) {
	return r.backend.logDetailed(n, afterHash)
}

// CommitFileList 返回指定 commit 的详细信息和变更文件列表（不含文件内容）。
func (r *Repo) CommitFileList(hash string) (*CommitInfo, []FileEntry, error) {
	return r.backend.commitFileList(hash)
}

// ShowFile 返回指定 commit 中某个文件的前后内容。
// 对于 added 文件 original 为空，对于 deleted 文件 modified 为空。
func (r *Repo) ShowFile(hash, filePath string) (*FileDiff, error) {
	return r.backend.showFile(hash, filePath)
}

// ── log 解析（CLI 后端专用，两个后端共用排序约定）──────────

func parseLog(out string) []CommitInfo {
	if strings.TrimSpace(out) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var commits []CommitInfo
	for _, line := range lines {
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Message: strings.SplitN(parts[1], "\n", 2)[0],
			Time:    time.Unix(ts, 0),
		})
	}
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Time.Before(commits[j].Time)
	})
	return commits
}

func parseDetailedLog(out string) []CommitInfo {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	blocks := strings.Split(out, "\n---\n")
	var commits []CommitInfo
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		// 第一个 block 可能以 "---" 开头（因前导的分隔符）
		if strings.HasPrefix(block, "---") {
			block = strings.TrimPrefix(block, "---")
			block = strings.TrimSpace(block)
		}
		lines := strings.Split(block, "\n")
		if len(lines) < 6 {
			continue
		}
		hash := strings.TrimSpace(lines[0])
		shortHash := strings.TrimSpace(lines[1])
		authorName := strings.TrimSpace(lines[2])
		authorEmail := strings.TrimSpace(lines[3])
		subject := strings.TrimSpace(lines[4])
		ts, err := strconv.ParseInt(strings.TrimSpace(lines[5]), 10, 64)
		if err != nil {
			continue
		}

		var filesChanged, insertions, deletions int
		for _, line := range lines[6:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 3 {
				continue
			}
			add := parts[0]
			del := parts[1]
			if add != "-" {
				if n, err := strconv.Atoi(add); err == nil {
					insertions += n
				}
			}
			if del != "-" {
				if n, err := strconv.Atoi(del); err == nil {
					deletions += n
				}
			}
			filesChanged++
		}

		commits = append(commits, CommitInfo{
			Hash:         hash,
			ShortHash:    shortHash,
			Message:      subject,
			Time:         time.Unix(ts, 0),
			AuthorName:   authorName,
			AuthorEmail:  authorEmail,
			FilesChanged: filesChanged,
			Insertions:   insertions,
			Deletions:    deletions,
		})
	}
	// 降序：最新在前
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Time.After(commits[j].Time)
	})
	return commits
}

// shortHashOf 取 commit hash 的短形式，与 `git log %h` 保持同样的 7 位约定。
// 两个后端共用：CLI 后端由 git 自己输出，go-git 后端没有等价的 --abbrev 实现。
func shortHashOf(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}

// subjectOf 取 commit message 的首行，与 CLI 的 %s 一致。
func subjectOf(message string) string {
	return strings.SplitN(message, "\n", 2)[0]
}

// ── 测试与诊断用的原始 CLI 通道 ────────────────────────────

// newCLIRepo 直接以 CLI 后端构造 Repo，绕过 config 与 platform.ResolveGit 查找。
// 仅供包内测试（repo_test.go 的白盒用例）使用。
func newCLIRepo(dir, gitBin string, logger *slog.Logger) *Repo {
	if logger == nil {
		logger = slog.Default()
	}
	return &Repo{dir: dir, backend: &cliBackend{dir: dir, gitBin: gitBin, logger: logger}, logger: logger}
}

// runInDir 把参数直接透传给 git 子进程，供白盒测试构造特定仓库状态。
// 非 CLI 后端（go-git）没有对应的 raw 通道，调用会得到明确错误而不是 panic。
func (r *Repo) runInDir(args ...string) (stdout, stderr string, err error) {
	cb, ok := r.backend.(*cliBackend)
	if !ok {
		return "", "", fmt.Errorf("git: raw git 调用只支持 cli 后端，当前是 %s 后端", r.backend.name())
	}
	return cb.run(args...)
}

// Dir 返回小说仓库根目录（绝对路径），供需要与工作区文件直接打交道的后端实现复用。
func (r *Repo) Dir() string { return r.dir }
