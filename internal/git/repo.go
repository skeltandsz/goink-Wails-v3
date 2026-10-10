package git

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/platform"
)

// Repo 管理单部小说的 Git 仓库，提供文件读写和版本控制。
//
// 版本控制的实际执行在 backend 层（cliBackend / goGitBackend），这里只是门面：
// 上层（app/、internal/rollback）看到的方法签名与拆分前完全一致。
type Repo struct {
	dir    string
	gitBin string // 空 = 没有可用的 git 二进制，后端降级为 go-git
	logger *slog.Logger

	// 后端惰性创建：测试里有 &Repo{dir: ..., gitBin: "git"} 这种直接构造、不走 New()，
	// 所以不能要求在构造函数里填好。sync.Once 保证并发调用绑定方法时只建一个后端
	// （go-git 的 Repository 不是并发安全的，重复打开也没必要）。
	backendOnce sync.Once
	backendRef  backend
	backendErr  error
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

// New 为指定小说打开已有仓库，不存在则初始化 + 首次空 commit。
// gitName 和 gitEmail 用于设置仓库级 git config。空字符串时使用默认值。
// logger 为 nil 时使用 slog.Default()。
//
// 找不到 git 可执行文件时不再一律失败：Android 上这是常态（随包只带 MinGit/Linux
// 二进制，bionic 跑不了），此时降级用纯 Go 后端，能力面见 goGitBackend 的注释。
func New(novelID int64, gitName, gitEmail string, logger *slog.Logger) (*Repo, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("git: config not initialized")
	}

	gitBin, err := platform.ResolveGit()
	if err != nil {
		if runtime.GOOS != "android" {
			// 桌面三平台本该有 bundled git，找不到属于安装/打包出了问题。
			// 静默换后端会把配置错误伪装成"能用"，所以仍然报错。
			return nil, fmt.Errorf("git: 找不到 git 可执行文件: %w", err)
		}
		// Android：预期内的降级。这条日志是真机上唯一能看出"走了内置后端"的记录。
		if logger == nil {
			logger = slog.Default()
		}
		logger.Info("未找到 git 可执行文件，小说仓库使用内置 go-git 后端", "err", err)
		gitBin = ""
	}

	if gitName == "" {
		gitName = "Goink"
	}
	if gitEmail == "" {
		gitEmail = "goink@local"
	}

	dir := config.NovelDirPath(novelID)
	if logger == nil {
		logger = slog.Default()
	}
	r := &Repo{dir: dir, gitBin: gitBin, logger: logger}

	be, err := r.backendChecked()
	if err != nil {
		return nil, err
	}
	if err := be.ensureRepo(gitName, gitEmail); err != nil {
		return nil, err
	}
	return r, nil
}

// backendChecked 返回（必要时创建）后端实例。构造失败会被记住并重复返回同一错误：
// 后端选择只依赖 gitBin 与 GOINK_GIT_BACKEND，运行期不会变，重试也只会得到同样结果，
// 而"一次走 CLI、一次走 go-git"反而更糟。
func (r *Repo) backendChecked() (backend, error) {
	r.backendOnce.Do(func() {
		r.backendRef, r.backendErr = newBackend(r.dir, r.gitBin, r.log())
	})
	return r.backendRef, r.backendErr
}

func (r *Repo) log() *slog.Logger {
	if r.logger == nil {
		return slog.Default()
	}
	return r.logger
}

// SetGitConfig 在已有仓库上更新仓库级 user.name / user.email 配置。
func (r *Repo) SetGitConfig(gitName, gitEmail string) error {
	be, err := r.backendChecked()
	if err != nil {
		return err
	}
	return be.setConfig(gitName, gitEmail)
}

// DiffContent 对比当前工作区文件与 proposed 内容，返回 unified diff。
// 文件不存在时以空内容为基准。
func (r *Repo) DiffContent(relPath, proposed string) (string, error) {
	be, err := r.backendChecked()
	if err != nil {
		return "", err
	}
	return be.diffContent(relPath, proposed)
}

// StageAll 把工作区的全部变更（含删除）暂存到索引。
func (r *Repo) StageAll() error {
	be, err := r.backendChecked()
	if err != nil {
		return err
	}
	return be.stageAll()
}

// Commit 提交索引内容并返回新 commit 的 hash。
func (r *Repo) Commit(msg string) (string, error) {
	be, err := r.backendChecked()
	if err != nil {
		return "", err
	}
	return be.commit(msg)
}

// HasUncommitted 判断是否有未提交变更（含未跟踪文件，语义同 git status --porcelain）。
func (r *Repo) HasUncommitted() (bool, error) {
	be, err := r.backendChecked()
	if err != nil {
		return false, err
	}
	return be.hasUncommitted()
}

// RevertNoCommit 逆序 revert 所有 hash，仅暂存不提交。冲突时自动 abort。
// 调用方负责最终 commit（或 abort）。
func (r *Repo) RevertNoCommit(hashes []string) error {
	be, err := r.backendChecked()
	if err != nil {
		return err
	}
	return be.revertNoCommit(hashes)
}

// RevertAbort 取消进行中的 revert，丢弃所有暂存的 revert 内容。
func (r *Repo) RevertAbort() error {
	be, err := r.backendChecked()
	if err != nil {
		return err
	}
	return be.revertAbort()
}

// Log 返回（可选限定某文件路径的）提交历史，按时间升序。
func (r *Repo) Log(relPath string, n int) ([]CommitInfo, error) {
	be, err := r.backendChecked()
	if err != nil {
		return nil, err
	}
	return be.log(relPath, n)
}

// LogDetailed 返回带详细统计的提交历史，按时间降序（最新在前）。
// afterHash 非空时，返回该 commit 之后的更早提交。
func (r *Repo) LogDetailed(n int, afterHash string) ([]CommitInfo, error) {
	be, err := r.backendChecked()
	if err != nil {
		return nil, err
	}
	return be.logDetailed(n, afterHash)
}

// CommitFileList 返回指定 commit 的详细信息和变更文件列表（不含文件内容）。
func (r *Repo) CommitFileList(hash string) (*CommitInfo, []FileEntry, error) {
	be, err := r.backendChecked()
	if err != nil {
		return nil, nil, err
	}
	return be.commitFileList(hash)
}

// ShowFile 返回指定 commit 中某个文件的前后内容。
// 对于 added 文件 original 为空，对于 deleted 文件 modified 为空。
func (r *Repo) ShowFile(hash, filePath string) (*FileDiff, error) {
	be, err := r.backendChecked()
	if err != nil {
		return nil, err
	}
	return be.showFile(hash, filePath)
}

// runInDir 直接跑一条 git 子命令，仅在 CLI 后端下可用。
//
// 存在的理由：测试要搭真实仓库（init/config/add/commit），历史上这些调用就挂在
// Repo 上。业务代码不该用它 —— 那等于绕过 backend 抽象。
func (r *Repo) runInDir(args ...string) (stdout, stderr string, err error) {
	if _, err := r.backendChecked(); err != nil {
		return "", "", err
	}
	cb, ok := r.backendRef.(*cliBackend)
	if !ok {
		return "", "", fmt.Errorf("git: 当前后端（go-git）不支持直接执行 git 子命令")
	}
	return cb.runInDir(args...)
}
