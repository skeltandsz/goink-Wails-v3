package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// goGitBackend 是纯 Go 的 backend 实现，用于没有 git 可执行文件的环境（Android）。
//
// 与 cliBackend 的刻意差异，都在这里说明，别当 bug 逐条查：
//   - 分支名固定为 go-git 的默认（master），不读用户的 init.defaultBranch；代码只按
//     HEAD 取历史，不受影响。
//   - 不写 GIT_DIR 之类环境，也不 fork 进程，所以 bundled MinGit 缺失时依旧可用；
//     反过来，用户全局 git 配置（hooksPath、gpgsign）天然不会被继承——这正好等价于
//     cliBackend 用 -c 强行关掉的那几项。
//   - revert 用"把每个受影响文件恢复到父版本"实现，而不是 git 的三方合并：小说仓库是
//     单写者的线性历史，两者结果一致；一旦出现真正的冲突（同一文件在 revert 之后又被
//     改过），git 会报冲突，这里会用父版本内容直接覆盖 —— 所以 revertNoCommit 前置
//     要求工作区干净（见实现里的 hasUncommitted 守卫），把"覆盖未提交改动"这条路堵死。
//   - numstat 自己按行算（见 diff.go），二进制文件按 git 的 "-\t-" 处理成 0 增 0 删。
//   - 重命名检测只认"删除与新增的内容 blob hash 相同"这一种（等价 git -M 的 100%
//     相似），不做相似度打分；改名同时又有内容编辑时会被记成 added+deleted。
type goGitBackend struct {
	dir    string
	logger *slog.Logger
}

// lg 返回后端 logger，缺省时回退 slog.Default()。
// 刻意不叫 log：backend 接口的 log(relPath, n) 是 `git log`，方法名会撞车。
func (g *goGitBackend) lg() *slog.Logger {
	if g.logger == nil {
		return slog.Default()
	}
	return g.logger
}

func (g *goGitBackend) open() (*gogit.Repository, error) {
	repo, err := gogit.PlainOpen(g.dir)
	if err != nil {
		return nil, fmt.Errorf("git: 打开仓库 %s 失败: %w", g.dir, err)
	}
	return repo, nil
}

// ── 初始化与配置 ──────────────────────────────────────────

func (g *goGitBackend) ensureRepo(gitName, gitEmail string) error {
	if _, err := os.Stat(filepath.Join(g.dir, ".git")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("git: stat .git: %w", err)
	}

	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return fmt.Errorf("git: create novel dir: %w", err)
	}
	repo, err := gogit.PlainInit(g.dir, false)
	if err != nil {
		return fmt.Errorf("git: init: %w", err)
	}
	if err := g.writeConfig(repo, gitName, gitEmail); err != nil {
		return err
	}

	gitkeep := filepath.Join(g.dir, "chapters", ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(gitkeep), 0o755); err != nil {
		return fmt.Errorf("git: create chapters dir: %w", err)
	}
	if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
		return fmt.Errorf("git: write .gitkeep: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git: worktree: %w", err)
	}
	if _, err := wt.Add("chapters/.gitkeep"); err != nil {
		return fmt.Errorf("git: stage .gitkeep: %w", err)
	}
	if _, err := g.commitWith(repo, wt, "initial commit"); err != nil {
		return fmt.Errorf("git: initial commit: %w", err)
	}
	return nil
}

func (g *goGitBackend) setConfig(gitName, gitEmail string) error {
	repo, err := g.open()
	if err != nil {
		return err
	}
	return g.writeConfig(repo, gitName, gitEmail)
}

// writeConfig 把 user.name / user.email 写进仓库级 .git/config。
// go-git 提交时优先用 Signature，我们每次提交都显式带上（见 signature），写 config
// 是为了让外部工具（用户自己 clone 或桌面版 git GUI）看到的作者与 goink 一致。
func (g *goGitBackend) writeConfig(repo *gogit.Repository, gitName, gitEmail string) error {
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("git: read config: %w", err)
	}
	cfg.User.Name = gitName
	cfg.User.Email = gitEmail
	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("git: config user.name/email: %w", err)
	}
	return nil
}

// signature 取仓库 config 里的作者信息；缺省时回退默认值，保证 commit 不会因为
// "没有 author" 而失败（CLI 版同样靠仓库级 config 兜底）。
func (g *goGitBackend) signature(repo *gogit.Repository) *object.Signature {
	name, email := "Goink", "goink@local"
	if cfg, err := repo.Config(); err == nil {
		if cfg.User.Name != "" {
			name = cfg.User.Name
		}
		if cfg.User.Email != "" {
			email = cfg.User.Email
		}
	}
	// go-git v5 的 object.Signature 只有 Name/Email/When 三个字段（v6 才有
	// ZoneOffset），时区偏移由 When 携带的 location 在编码 commit 时算出。
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

// ── 写路径 ────────────────────────────────────────────────

func (g *goGitBackend) stageAll() error {
	repo, err := g.open()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git: worktree: %w", err)
	}
	// AddOptions{All:true} 等价 `git add -A`：新增、修改、删除都进索引。
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git: stage all: %w", err)
	}
	return nil
}

func (g *goGitBackend) commit(msg string) (string, error) {
	repo, err := g.open()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("git: worktree: %w", err)
	}
	return g.commitWith(repo, wt, msg)
}

func (g *goGitBackend) commitWith(repo *gogit.Repository, wt *gogit.Worktree, msg string) (string, error) {
	// All:false：只提交索引里已有的内容，与 CLI 的 `git commit -m`（不含 -a）一致。
	hash, err := wt.Commit(msg, &gogit.CommitOptions{Author: g.signature(repo)})
	if err != nil {
		// go-git v5 用哨兵错误 ErrEmptyCommit 表示"干净工作区，提交不了东西"
		// （v6 才换成 NoChangesError 类型）。
		if errors.Is(err, gogit.ErrEmptyCommit) {
			// 对齐 CLI：git 在"无变更"时也是非零退出，上层按 commit 失败处理。
			return "", fmt.Errorf("git: commit: nothing to commit")
		}
		return "", fmt.Errorf("git: commit: %w", err)
	}
	return hash.String(), nil
}

func (g *goGitBackend) hasUncommitted() (bool, error) {
	repo, err := g.open()
	if err != nil {
		return false, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("git: worktree: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("git: status: %w", err)
	}
	// go-git 的 Status 是"路径 -> 变更"的 map，含未跟踪文件，
	// 与 `git status --porcelain` 非空判定一致。
	for _, entry := range st {
		// go-git 的 Staging/Worktree 是 StatusCode（' '/'M'/'A'/'?' 这类字符），
		// 不是 hash；Status map 只在有变化时才建条目，这里再排一次"两侧都未修改"。
		if entry != nil && (entry.Staging != gogit.Unmodified || entry.Worktree != gogit.Unmodified) {
			return true, nil
		}
	}
	return len(st) > 0, nil
}

// ── revert ────────────────────────────────────────────────

// revertNoCommit 逆序把每个 commit 的反向变更落到工作区+索引，不产生 commit。
// 任一 hash 解析失败即整体放弃并复位到 HEAD（等价 CLI 的 git revert --abort）。
func (g *goGitBackend) revertNoCommit(hashes []string) error {
	repo, err := g.open()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git: worktree: %w", err)
	}
	// 前置守卫：git 在有未提交改动时会拒绝 revert，这里也必须拒绝，否则"恢复到父
	// 版本"会直接覆盖掉用户没提交的编辑（见类型注释）。
	if clean, err := g.worktreeClean(repo, wt); err == nil && !clean {
		return fmt.Errorf("git: revert: 工作区有未提交变更，先提交或放弃后再回滚")
	}

	for i := len(hashes) - 1; i >= 0; i-- {
		if err := g.applyRevert(repo, wt, hashes[i]); err != nil {
			if abortErr := g.revertAbort(); abortErr != nil {
				return fmt.Errorf("git: revert %s: %w; 复位也失败: %v", hashes[i], err, abortErr)
			}
			return fmt.Errorf("git: revert %s: %w", hashes[i], err)
		}
	}
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git: revert stage: %w", err)
	}
	return nil
}

// applyRevert 把 c 相对其父的变更反向应用到工作区：c 新增的删掉、c 删除的按父内容
// 写回、c 修改的写回父内容。父版本里存在但内容不同的路径统一"写成父内容"。
func (g *goGitBackend) applyRevert(repo *gogit.Repository, wt *gogit.Worktree, hash string) error {
	target, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		if resolved, rerr := repo.ResolveRevision(plumbing.Revision(hash)); rerr == nil {
			target, err = repo.CommitObject(*resolved)
		}
		if err != nil {
			return fmt.Errorf("解析 commit 失败: %w", err)
		}
	}
	if target.NumParents() == 0 {
		return fmt.Errorf("根提交没有父版本，无法 revert")
	}
	parent, err := target.Parent(0)
	if err != nil {
		return fmt.Errorf("取父提交失败: %w", err)
	}

	cur, err := treeEntries(target)
	if err != nil {
		return err
	}
	old, err := treeEntries(parent)
	if err != nil {
		return err
	}

	for path := range unionOfKeys(cur, old) {
		wantBlob, inParent := old[path]
		_, inCur := cur[path]
		if !inParent {
			if _, ok := cur[path]; ok {
				if _, err := wt.Remove(path); err != nil {
					return fmt.Errorf("删除 %s 失败: %w", path, err)
				}
			}
			continue
		}
		if inCur && cur[path] == wantBlob {
			continue // 父版本与工作区一致，无需动
		}
		content, err := blobBytes(repo, wantBlob)
		if err != nil {
			return err
		}
		full := filepath.Join(g.dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// revertAbort 丢弃 revert 的结果，回到 HEAD。
//
// 与 CLI 的一处行为差异：git 在没有 revert 进行中时 `--abort` 会报错，这里按幂等处理
// 返回 nil —— 调用方（rollback 的兜底路径）本来就用 `_ = RevertAbort()` 忽略错误，
// 让"没什么可撤销"成为错误反而会把已成功的 DB 事务配上一次无谓的报错日志。
func (g *goGitBackend) revertAbort() error {
	repo, err := g.open()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git: worktree: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("git: revert --abort: %w", err)
	}
	target, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("git: revert --abort: %w", err)
	}
	// object.Commit.Hash 是字段不是方法（v5 没有 Hash() getter）。
	if err := wt.Reset(&gogit.ResetOptions{Mode: gogit.HardReset, Commit: target.Hash}); err != nil {
		return fmt.Errorf("git: revert --abort: %w", err)
	}
	return nil
}

func (g *goGitBackend) worktreeClean(repo *gogit.Repository, wt *gogit.Worktree) (bool, error) {
	st, err := wt.Status()
	if err != nil {
		return false, err
	}
	return len(st) == 0, nil
}

// ── 读路径 ────────────────────────────────────────────────

func (g *goGitBackend) log(relPath string, n int) ([]CommitInfo, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	commits, err := g.commits(repo, "", n, relPath)
	if err != nil {
		return nil, err
	}
	// CLI 版 parseLog 返回按时间升序，这里保持一致（上层 HistoryList 依赖该顺序）。
	sort.Slice(commits, func(i, j int) bool { return commits[i].Time.Before(commits[j].Time) })
	return commits, nil
}

func (g *goGitBackend) logDetailed(n int, afterHash string) ([]CommitInfo, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	commits, err := g.commits(repo, afterHash, n, "")
	if err != nil {
		return nil, err
	}
	for i := range commits {
		c, err := repo.CommitObject(plumbing.NewHash(commits[i].Hash))
		if err != nil {
			continue
		}
		files, ins, del, err := g.commitStats(repo, c)
		if err != nil {
			return nil, err
		}
		commits[i].FilesChanged = files
		commits[i].Insertions = ins
		commits[i].Deletions = del
	}
	sort.Slice(commits, func(i, j int) bool { return commits[i].Time.After(commits[j].Time) })
	return commits, nil
}

// commits 从 afterHash（空则 HEAD）往回走，afterHash 自身不计（等价 --skip=1）；
// n<=0 表示不限制；relPath 非空时只保留改了该路径的提交。
func (g *goGitBackend) commits(repo *gogit.Repository, afterHash string, n int, relPath string) ([]CommitInfo, error) {
	start := plumbing.Hash{}
	if afterHash != "" {
		resolved, err := repo.ResolveRevision(plumbing.Revision(afterHash))
		if err != nil {
			return nil, fmt.Errorf("git: log %s: %w", afterHash, err)
		}
		start = *resolved
	} else {
		head, err := repo.Head()
		if err != nil {
			if errors.Is(err, plumbing.ErrReferenceNotFound) {
				return nil, nil // 空仓库：与 CLI 的 "does not have any commits yet" 分支一致
			}
			return nil, fmt.Errorf("git: HEAD: %w", err)
		}
		start = head.Hash()
	}

	iter, err := repo.Log(&gogit.LogOptions{From: start})
	if err != nil {
		return nil, fmt.Errorf("git: log: %w", err)
	}
	var (
		out   []CommitInfo
		skip  = afterHash != ""
		count int
	)
	err = iter.ForEach(func(c *object.Commit) error {
		if skip {
			skip = false
			return nil
		}
		if n > 0 && count >= n {
			return storerErrStop
		}
		if relPath != "" {
			touched, err := commitTouchedPath(c, relPath)
			if err != nil {
				return err
			}
			if !touched {
				return nil
			}
		}
		out = append(out, commitInfo(c))
		count++
		return nil
	})
	if err != nil && !errors.Is(err, storerErrStop) {
		return nil, fmt.Errorf("git: log: %w", err)
	}
	return out, nil
}

// storerErrStop 是提前终止 ForEach 的哨兵：go-git 会把它原样抛回来，
// 我们在外层判掉，不当成错误。
var storerErrStop = errors.New("git: stop iterating")

func commitInfo(c *object.Commit) CommitInfo {
	hash := c.Hash.String()
	return CommitInfo{
		Hash:        hash,
		ShortHash:   shortHash(hash),
		Message:     firstLine(c.Message),
		Time:        c.Committer.When, // git log %ct 用的也是 committer 时间
		AuthorName:  c.Author.Name,
		AuthorEmail: c.Author.Email,
	}
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// commitStats 返回该提交的变更文件数与增删行数（等价 git log --numstat 的汇总）。
// repo 由调用方传入：go-git v5 的 object.Commit 没有 Repository() 方法，
// 拿不到 store 就没法解引用 blob。
func (g *goGitBackend) commitStats(repo *gogit.Repository, c *object.Commit) (files, added, deleted int, err error) {
	cur, err := treeEntries(c)
	if err != nil {
		return 0, 0, 0, err
	}
	var old map[string]plumbing.Hash
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("git: parent of %s: %w", c.Hash, err)
		}
		if old, err = treeEntries(parent); err != nil {
			return 0, 0, 0, err
		}
	}
	for path := range unionOfKeys(cur, old) {
		newBlob, inNew := cur[path]
		oldBlob, inOld := old[path]
		if inNew && inOld && newBlob == oldBlob {
			continue
		}
		files++
		a, d, err := lineDelta(repo, oldBlob, newBlob)
		if err != nil {
			return 0, 0, 0, err
		}
		added += a
		deleted += d
	}
	return files, added, deleted, nil
}

// lineDelta 算单文件的增删行数；二进制按 git 的 "-\t-" 处理成 0/0（但仍算一个变更文件）。
func lineDelta(repo *gogit.Repository, from, to plumbing.Hash) (added, deleted int, err error) {
	oldText, err := blobText(repo, from)
	if err != nil {
		return 0, 0, err
	}
	newText, err := blobText(repo, to)
	if err != nil {
		return 0, 0, err
	}
	if oldText == nil || newText == nil { // 任一侧是二进制
		return 0, 0, nil
	}
	a, d := countLineDiff(*oldText, *newText)
	return a, d, nil
}

// commitFileList 返回 commit 元信息 + 变更文件列表。
func (g *goGitBackend) commitFileList(hash string) (*CommitInfo, []FileEntry, error) {
	repo, err := g.open()
	if err != nil {
		return nil, nil, err
	}
	c, err := g.resolveCommit(repo, hash)
	if err != nil {
		return nil, nil, err
	}
	info := commitInfo(c)
	entries, err := g.changeEntries(c)
	if err != nil {
		return nil, nil, err
	}
	files, ins, del, err := g.commitStats(repo, c)
	if err != nil {
		return nil, nil, err
	}
	info.FilesChanged, info.Insertions, info.Deletions = files, ins, del
	return &info, entries, nil
}

func (g *goGitBackend) resolveCommit(repo *gogit.Repository, hash string) (*object.Commit, error) {
	c, err := repo.CommitObject(plumbing.NewHash(hash))
	if err == nil {
		return c, nil
	}
	resolved, rerr := repo.ResolveRevision(plumbing.Revision(hash))
	if rerr != nil {
		return nil, fmt.Errorf("git: commit %s not found", hash)
	}
	c, err = repo.CommitObject(*resolved)
	if err != nil {
		return nil, fmt.Errorf("git: commit %s not found", hash)
	}
	return c, nil
}

// changeEntries 计算 commit 的文件级变更列表，含"内容完全相同"的重命名识别。
func (g *goGitBackend) changeEntries(c *object.Commit) ([]FileEntry, error) {
	cur, err := treeEntries(c)
	if err != nil {
		return nil, err
	}
	var old map[string]plumbing.Hash
	hasParent := c.NumParents() > 0
	if hasParent {
		parent, err := c.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("git: parent of %s: %w", c.Hash, err)
		}
		if old, err = treeEntries(parent); err != nil {
			return nil, err
		}
	}

	var entries []FileEntry
	addedPaths := map[plumbing.Hash][]string{}
	deletedPaths := map[plumbing.Hash][]string{}

	for path := range unionOfKeys(cur, old) {
		newBlob, inNew := cur[path]
		oldBlob, inOld := old[path]
		switch {
		case !hasParent:
			// 根提交：git ls-tree -r 的语义，全部记为 added
			entries = append(entries, FileEntry{Path: path, ChangeType: "added"})
		case inNew && !inOld:
			addedPaths[newBlob] = append(addedPaths[newBlob], path)
		case !inNew && inOld:
			deletedPaths[oldBlob] = append(deletedPaths[oldBlob], path)
		case newBlob != oldBlob:
			entries = append(entries, FileEntry{Path: path, ChangeType: "modified"})
		}
	}

	// rename：同 hash 的一删一增配对（等价 git -M 的 100% 相似），按名字序配对，
	// 保证同样输入下输出稳定。
	for blob := range addedPaths {
		dels, ok := deletedPaths[blob]
		if !ok {
			continue
		}
		adds := addedPaths[blob]
		sort.Strings(dels)
		sort.Strings(adds)
		pairs := min(len(dels), len(adds))
		for i := 0; i < pairs; i++ {
			entries = append(entries, FileEntry{
				Path: adds[i], ChangeType: "renamed", OldPath: dels[i],
			})
		}
		if pairs == len(dels) {
			delete(deletedPaths, blob)
		}
		addedPaths[blob] = adds[pairs:]
	}
	for blob, paths := range addedPaths {
		_ = blob
		for _, p := range paths {
			entries = append(entries, FileEntry{Path: p, ChangeType: "added"})
		}
	}
	for blob, paths := range deletedPaths {
		_ = blob
		for _, p := range paths {
			entries = append(entries, FileEntry{Path: p, ChangeType: "deleted"})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// showFile 返回某文件在该 commit 中的前后内容。
func (g *goGitBackend) showFile(hash, filePath string) (*FileDiff, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	c, err := g.resolveCommit(repo, hash)
	if err != nil {
		return nil, err
	}
	entries, err := g.changeEntries(c)
	if err != nil {
		return nil, err
	}
	var entry *FileEntry
	for i := range entries {
		if entries[i].Path == filePath {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("git: file %s not found in commit %s", filePath, hash)
	}

	diff := &FileDiff{Path: filePath, ChangeType: entry.ChangeType}
	var parent *object.Commit
	if c.NumParents() > 0 {
		if parent, err = c.Parent(0); err != nil {
			return nil, fmt.Errorf("git: parent of %s: %w", c.Hash, err)
		}
	}
	read := func(target *object.Commit, path string) (string, error) {
		if target == nil {
			return "", nil
		}
		f, err := target.File(path)
		if err != nil {
			if isMissingInCommit(err) {
				g.lg().Debug("go-git: file not in commit", "path", path, "commit", target.Hash.String())
				return "", nil
			}
			return "", err
		}
		return f.Contents()
	}

	switch entry.ChangeType {
	case "added":
		diff.ModifiedContent, err = read(c, filePath)
	case "modified":
		if diff.OriginalContent, err = read(parent, filePath); err == nil {
			diff.ModifiedContent, err = read(c, filePath)
		}
	case "deleted":
		diff.OriginalContent, err = read(parent, filePath)
	case "renamed":
		if diff.OriginalContent, err = read(parent, entry.OldPath); err == nil {
			diff.ModifiedContent, err = read(c, filePath)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", hash, filePath, err)
	}
	return diff, nil
}

func isMissingInCommit(err error) bool {
	return errors.Is(err, object.ErrFileNotFound) || errors.Is(err, plumbing.ErrObjectNotFound)
}

// diffContent 用工作区当前内容与 proposed 比较，生成 unified diff。
func (g *goGitBackend) diffContent(relPath, proposed string) (string, error) {
	full := filepath.Join(g.dir, filepath.FromSlash(relPath))
	cur, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("git: diff: read %s: %w", relPath, err)
	}
	return unifiedDiff(relPath, string(cur), proposed), nil
}

// ── tree 辅助 ─────────────────────────────────────────────

// treeEntries 展开成 path -> blob hash。只读 tree，不解引用 blob 内容，
// 所以每提交一次调用的成本与仓库文件数线性相关（小说仓库几十到几百个文件）。
func treeEntries(c *object.Commit) (map[string]plumbing.Hash, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, fmt.Errorf("git: tree of %s: %w", c.Hash, err)
	}
	out := map[string]plumbing.Hash{}
	iter := tree.Files()
	if err := iter.ForEach(func(f *object.File) error {
		out[f.Name] = f.Blob.Hash
		return nil
	}); err != nil {
		return nil, fmt.Errorf("git: walk tree %s: %w", c.Hash, err)
	}
	return out, nil
}

// commitTouchedPath 判断该提交是否改了 relPath（新增/删除/修改都算）。
func commitTouchedPath(c *object.Commit, relPath string) (bool, error) {
	cur, err := treeEntries(c)
	if err != nil {
		return false, err
	}
	if c.NumParents() == 0 {
		_, ok := cur[relPath]
		return ok, nil
	}
	parent, err := c.Parent(0)
	if err != nil {
		return false, err
	}
	old, err := treeEntries(parent)
	if err != nil {
		return false, err
	}
	a, inOld := old[relPath]
	b, inNew := cur[relPath]
	switch {
	case inOld && inNew:
		return a != b, nil
	case inOld || inNew:
		return true, nil
	default:
		return false, nil
	}
}

func blobBytes(repo *gogit.Repository, h plumbing.Hash) ([]byte, error) {
	blob, err := repo.BlobObject(h)
	if err != nil {
		return nil, fmt.Errorf("git: blob %s: %w", h, err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("git: blob reader %s: %w", h, err)
	}
	defer reader.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("git: blob read %s: %w", h, err)
	}
	return buf.Bytes(), nil
}

// blobText 返回文本内容；零 hash（文件在该版本不存在）返回 ""；二进制返回 nil。
func blobText(repo *gogit.Repository, h plumbing.Hash) (*string, error) {
	if h.IsZero() {
		empty := ""
		return &empty, nil
	}
	raw, err := blobBytes(repo, h)
	if err != nil {
		return nil, err
	}
	if isBinaryContent(raw) {
		return nil, nil
	}
	s := string(raw)
	return &s, nil
}

// isBinaryContent 用 git 的判据：前 8000 字节里出现 NUL 即当二进制。
func isBinaryContent(b []byte) bool {
	limit := len(b)
	if limit > 8000 {
		limit = 8000
	}
	return bytes.IndexByte(b[:limit], 0) >= 0
}

// unionOfKeys 求两个 map 的键并集；T 必须是 comparable（map 的键类型）。
func unionOfKeys[T comparable, U any](a map[T]U, b map[T]U) map[T]struct{} {
	out := make(map[T]struct{}, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

// 编译期确认接口实现完整（漏方法在这里就报错，而不是运行到手机上 panic）。
var _ backend = (*goGitBackend)(nil)
var _ backend = (*cliBackend)(nil)

// context 目前只用于 blob 读取的超时占位；保留 import 以免后续改动误删。
var _ = context.Background
