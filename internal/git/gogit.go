package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// goGitBackend 用纯 Go 的 go-git 实现 backend，不依赖任何外部可执行文件。
//
// 为什么必须存在：Android APK 里带不了 git 二进制（见 backend.go 顶部说明），
// 而小说仓库的版本控制是 Goink 的核心能力（章节历史、按 turn 回滚），不能因为
// 环境没有 git 就整体不可用。go-git 是纯 Go 实现，直接读写 .git 对象库，
// 产生的仓库与真实 git 完全互通 —— 同一部小说先在手机上写、再拷到桌面用 git CLI
// 打开，历史照常可读。
//
// 与 CLI 后端的取舍：
//   - 不读用户全局 git 配置，也不执行 hook、不做 GPG 签名。CLI 后端靠 runCmd 的
//     -c 覆盖项达成同样效果，这里天然达成。
//   - revert 用"逐文件精确反向应用"代替三方合并，冲突判定更严格（见 revertOne）。
//   - rename 检测走 go-git 的 DefaultDiffTreeOptions（相似度 60），与 CLI 的 -M 等价。
type goGitBackend struct {
	dir    string
	logger *slog.Logger
	repo   *gogit.Repository

	// reverts 记录本次 revertNoCommit 已经改过的工作区文件，供 revertAbort 逐文件还原。
	// 之所以能放在实例上：调用方（internal/rollback）在同一个 Repo 实例内完成
	// revert → DB 事务 → commit/abort，不跨请求。
	reverts []revertUndo
}

// revertUndo 是 revert 应用一步之前、单个文件的工作区快照。
type revertUndo struct {
	path    string // 工作区绝对路径
	content []byte
	existed bool // 之前文件不存在（revert 新增了它）
}

func newGoGitBackend(dir string, logger *slog.Logger) *goGitBackend {
	if logger == nil {
		logger = slog.Default()
	}
	return &goGitBackend{dir: dir, logger: logger}
}

func (g *goGitBackend) name() string { return "gogit" }

// ── 打开与基础工具 ────────────────────────────────────────

func (g *goGitBackend) open() (*gogit.Repository, error) {
	if g.repo != nil {
		return g.repo, nil
	}
	repo, err := gogit.PlainOpen(g.dir)
	if err != nil {
		return nil, fmt.Errorf("git: open %s: %w", g.dir, err)
	}
	g.repo = repo
	return repo, nil
}

func (g *goGitBackend) worktree() (*gogit.Worktree, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git: worktree %s: %w", g.dir, err)
	}
	return w, nil
}

// commitObject 解析 commit 引用（目前只支持完整 hash，与 CLI 后端的 hash 入参一致）。
func (g *goGitBackend) commitObject(hash string) (*object.Commit, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	c, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, fmt.Errorf("git: commit %s: %w", hash, err)
	}
	return c, nil
}

// identity 返回仓库级 user.name / user.email，缺失时用 Goink 的默认值，
// 与 New 里给 CLI 后端填的默认值保持一致。
func (g *goGitBackend) identity() (name, email string) {
	name, email = "Goink", "goink@local"
	repo, err := g.open()
	if err != nil {
		return name, email
	}
	cfg, err := repo.Config()
	if err != nil {
		return name, email
	}
	if cfg.User.Name != "" {
		name = cfg.User.Name
	}
	if cfg.User.Email != "" {
		email = cfg.User.Email
	}
	return name, email
}

// treeOf 返回 commit 的树；c 为 nil（根 commit 没有父）时返回空树，
// 与 CLI 后端把根 commit 的全部文件当 added 的行为等价。
func treeOf(c *object.Commit) (*object.Tree, error) {
	if c == nil {
		return &object.Tree{}, nil
	}
	t, err := c.Tree()
	if err != nil {
		return nil, fmt.Errorf("git: tree of %s: %w", c.Hash.String(), err)
	}
	return t, nil
}

// parentCommit 返回首父；根 commit 返回 (nil, nil)。
func parentCommit(c *object.Commit) (*object.Commit, error) {
	if c == nil || c.NumParents() == 0 {
		return nil, nil
	}
	p, err := c.Parent(0)
	if err != nil {
		return nil, fmt.Errorf("git: parent of %s: %w", c.Hash.String(), err)
	}
	return p, nil
}

// blobAt 读取树里某个文件的内容。路径不存在时返回 (nil, nil)，
// 对应 CLI 后端 showFileContent 把 "does not exist" 当正常情况处理。
func (g *goGitBackend) blobAt(t *object.Tree, path string) ([]byte, error) {
	if t == nil || path == "" {
		return nil, nil
	}
	f, err := t.File(filepath.ToSlash(path))
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) || errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("git: read %s from tree: %w", path, err)
	}
	r, err := f.Reader()
	if err != nil {
		return nil, fmt.Errorf("git: open %s: %w", path, err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// changePath 取一条 tree change 的路径。go-git 对 Insert 会把 From 置零，
// 对 Delete 会把 To 置零，所以非空的那一侧就是路径。
func changePath(ch *object.Change) string {
	if ch.To.Name != "" {
		return ch.To.Name
	}
	return ch.From.Name
}

// ── 初始化与配置 ──────────────────────────────────────────

func (g *goGitBackend) initFresh(gitName, gitEmail string) error {
	repo, err := gogit.PlainInit(g.dir, false)
	if err != nil {
		return fmt.Errorf("git: init: %w", err)
	}
	g.repo = repo

	// 仓库级 git config，避免用户未全局配置时 commit 失败
	if err := g.setIdentity(gitName, gitEmail); err != nil {
		return err
	}

	gitkeep := filepath.Join(g.dir, "chapters", ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(gitkeep), 0755); err != nil {
		return fmt.Errorf("git: create chapters dir: %w", err)
	}
	if err := os.WriteFile(gitkeep, nil, 0644); err != nil {
		return fmt.Errorf("git: write .gitkeep: %w", err)
	}
	if err := g.stageAll(); err != nil {
		return fmt.Errorf("git: stage .gitkeep: %w", err)
	}
	if _, err := g.commit("initial commit"); err != nil {
		return fmt.Errorf("git: initial commit: %w", err)
	}
	return nil
}

func (g *goGitBackend) setIdentity(gitName, gitEmail string) error {
	repo, err := g.open()
	if err != nil {
		return err
	}
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("git: config user.name: %w", err)
	}
	cfg.User.Name = gitName
	cfg.User.Email = gitEmail
	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("git: config user.email: %w", err)
	}
	return nil
}

// ── 暂存与提交 ────────────────────────────────────────────

func (g *goGitBackend) stageAll() error {
	w, err := g.worktree()
	if err != nil {
		return err
	}
	// AddOptions.All 等价于 `git add -A`：工作区有文件的位置新增/修改，
	// 索引有条目而工作区没有的位置删除，让索引与工作区对齐。
	if err := w.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git: stage all: %w", err)
	}
	return nil
}

func (g *goGitBackend) commit(msg string) (string, error) {
	w, err := g.worktree()
	if err != nil {
		return "", err
	}
	name, email := g.identity()
	sig := &object.Signature{Name: name, Email: email, When: time.Now()}
	h, err := w.Commit(msg, &gogit.CommitOptions{Author: sig})
	if err != nil {
		return "", fmt.Errorf("git: commit: %w", err)
	}
	// 索引/工作区已随提交对齐，revert 快照也不再有意义
	g.reverts = nil
	return h.String(), nil
}

func (g *goGitBackend) hasUncommitted() (bool, error) {
	w, err := g.worktree()
	if err != nil {
		return false, err
	}
	st, err := w.Status()
	if err != nil {
		return false, fmt.Errorf("git: status: %w", err)
	}
	// go-git 的 Status 含未跟踪文件（Worktree: Untracked），与 `status --porcelain` 同义。
	return len(st) > 0, nil
}

// ── 历史查询 ──────────────────────────────────────────────

func (g *goGitBackend) log(relPath string, n int) ([]CommitInfo, error) {
	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	iter, err := repo.Log(&gogit.LogOptions{Order: gogit.LogOrderCommitterTime})
	if err != nil {
		if isNoCommits(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("git: log: %w", err)
	}
	defer iter.Close()

	var out []CommitInfo
	err = iter.ForEach(func(c *object.Commit) error {
		if relPath != "" && !commitTouchesPath(g, c, relPath) {
			return nil
		}
		out = append(out, CommitInfo{
			Hash:    c.Hash.String(),
			Message: subjectOf(c.Message),
			Time:    c.Committer.When,
		})
		if n > 0 && len(out) >= n {
			return storer.ErrStop
		}
		return nil
	})
	if isNoCommits(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git: log: %w", err)
	}
	// 与 CLI 后端 parseLog 一致：升序（最旧在前）
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}

// commitTouchesPath 判断某 commit 相对首父是否动过 relPath（精确文件或目录前缀）。
// 对应 CLI 的 `git log -- <path>`（go-git 的 LogOptions.PathFilter 不做历史简化，
// 语义上更接近 --follow=false 的普通过滤）。
func commitTouchesPath(g *goGitBackend, c *object.Commit, relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	p, err := parentCommit(c)
	if err != nil {
		return false
	}
	from, err := treeOf(p)
	if err != nil {
		return false
	}
	to, err := treeOf(c)
	if err != nil {
		return false
	}
	changes, err := from.DiffContext(context.Background(), to)
	if err != nil {
		return false
	}
	for _, ch := range changes {
		name := changePath(ch)
		if name == relPath || strings.HasPrefix(name, relPath+"/") {
			return true
		}
	}
	return false
}

// logDetailed 返回带统计的历史，最新在前。afterHash 非空时从它的父 commit 起算，
// 与 CLI 的 `git log <afterHash> --skip=1 -n <n>` 等价（分页取更早的一页）。
func (g *goGitBackend) logDetailed(n int, afterHash string) ([]CommitInfo, error) {
	from := plumbing.ZeroHash
	if afterHash != "" {
		c, err := g.commitObject(afterHash)
		if err != nil {
			return nil, err
		}
		p, err := parentCommit(c)
		if err != nil {
			return nil, err
		}
		if p == nil {
			// afterHash 是根 commit：它之前没有更多历史，CLI 同样返回空。
			return nil, nil
		}
		from = p.Hash
	}

	repo, err := g.open()
	if err != nil {
		return nil, err
	}
	iter, err := repo.Log(&gogit.LogOptions{From: from, Order: gogit.LogOrderCommitterTime})
	if err != nil {
		if isNoCommits(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("git: log detailed: %w", err)
	}
	defer iter.Close()

	var out []CommitInfo
	err = iter.ForEach(func(c *object.Commit) error {
		info := CommitInfo{
			Hash:        c.Hash.String(),
			ShortHash:   shortHashOf(c.Hash.String()),
			Message:     subjectOf(c.Message),
			Time:        c.Committer.When,
			AuthorName:  c.Author.Name,
			AuthorEmail: c.Author.Email,
		}
		// 统计自己算，不用 object.Commit.Stats()：go-git 的 Stats 基于文本 patch，
		// 二进制文件（封面 jpg）会被整个跳过，导致 FilesChanged 少算 —— 与 CLI 的
		// "-\t-\tpath"（算一个文件、0 行）不一致。见 commitStats。
		files, ins, del, err := g.commitStats(c)
		if err != nil {
			return err
		}
		info.FilesChanged = files
		info.Insertions = ins
		info.Deletions = del
		out = append(out, info)
		if n > 0 && len(out) >= n {
			return storer.ErrStop
		}
		return nil
	})
	if isNoCommits(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git: log detailed: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// fileEntries 返回 commit 相对首父的变更文件列表，带 rename 检测（等价 CLI 的 -M）。
func (g *goGitBackend) fileEntries(c *object.Commit) ([]FileEntry, error) {
	p, err := parentCommit(c)
	if err != nil {
		return nil, err
	}
	from, err := treeOf(p)
	if err != nil {
		return nil, err
	}
	to, err := treeOf(c)
	if err != nil {
		return nil, err
	}
	// Tree.DiffContext 用的就是 DefaultDiffTreeOptions（DetectRenames: true, 阈值 60）。
	changes, err := from.DiffContext(context.Background(), to)
	if err != nil {
		return nil, fmt.Errorf("git: diff-tree %s: %w", c.Hash.String(), err)
	}

	entries := make([]FileEntry, 0, len(changes))
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			continue
		}
		switch action {
		case merkletrie.Insert:
			entries = append(entries, FileEntry{Path: ch.To.Name, ChangeType: "added"})
		case merkletrie.Delete:
			entries = append(entries, FileEntry{Path: ch.From.Name, ChangeType: "deleted"})
		default:
			if ch.From.Name != ch.To.Name {
				entries = append(entries, FileEntry{
					Path: ch.To.Name, ChangeType: "renamed", OldPath: ch.From.Name,
				})
				continue
			}
			entries = append(entries, FileEntry{Path: ch.To.Name, ChangeType: "modified"})
		}
	}
	return entries, nil
}

// commitStats 返回 commit 相对首父的 (变更文件数, 新增行, 删除行)，对齐 `git log --numstat`。
//
// 为什么不直接用 go-git 的 Commit.Stats()：它建立在文本 patch 上，二进制文件不产生
// patch，于是"只改了一次封面"在历史里显示成 0 个变更文件；CLI 则输出
// "-\t-\tcover.jpg"，即 1 个文件、0 行。这里按 CLI 的语义来。
// rename 与 CLI 一致：按旧内容与新内容比行数（纯改名就是 0/0）。
func (g *goGitBackend) commitStats(c *object.Commit) (files, added, deleted int, err error) {
	entries, err := g.fileEntries(c)
	if err != nil {
		return 0, 0, 0, err
	}
	p, err := parentCommit(c)
	if err != nil {
		return 0, 0, 0, err
	}
	parentTree, err := treeOf(p)
	if err != nil {
		return 0, 0, 0, err
	}
	commitTree, err := treeOf(c)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, e := range entries {
		files++
		oldPath := e.Path
		if e.OldPath != "" {
			oldPath = e.OldPath
		}
		before, err := g.blobAt(parentTree, oldPath)
		if err != nil {
			return 0, 0, 0, err
		}
		after, err := g.blobAt(commitTree, e.Path)
		if err != nil {
			return 0, 0, 0, err
		}
		a, d := countLineDiff(before, after)
		added += a
		deleted += d
	}
	return files, added, deleted, nil
}

// countLineDiff 数出 before → after 的增删行数；任一侧是二进制就按 0/0（同 git）。
func countLineDiff(before, after []byte) (added, deleted int) {
	if isBinaryContent(before) || isBinaryContent(after) {
		return 0, 0
	}
	for _, ln := range diffLines(string(before), string(after)) {
		switch ln.op {
		case '+':
			added++
		case '-':
			deleted++
		}
	}
	return added, deleted
}

// isBinaryContent 采用 git 的判据：前 8000 字节内出现 NUL 即视为二进制。
func isBinaryContent(b []byte) bool {
	limit := len(b)
	if limit > 8000 {
		limit = 8000
	}
	return bytes.IndexByte(b[:limit], 0) >= 0
}

func (g *goGitBackend) commitFileList(hash string) (*CommitInfo, []FileEntry, error) {
	c, err := g.commitObject(hash)
	if err != nil {
		return nil, nil, err
	}
	entries, err := g.fileEntries(c)
	if err != nil {
		return nil, nil, err
	}
	// CLI 后端这里不带 --numstat，统计字段保持零值，前端按 entries 自行计数。
	return &CommitInfo{
		Hash:        c.Hash.String(),
		ShortHash:   shortHashOf(c.Hash.String()),
		Message:     subjectOf(c.Message),
		Time:        c.Committer.When,
		AuthorName:  c.Author.Name,
		AuthorEmail: c.Author.Email,
	}, entries, nil
}

// showFile 返回指定 commit 中某个文件的前后内容。
// 对于 added 文件 original 为空，对于 deleted 文件 modified 为空。
func (g *goGitBackend) showFile(hash, filePath string) (*FileDiff, error) {
	c, err := g.commitObject(hash)
	if err != nil {
		return nil, err
	}
	entries, err := g.fileEntries(c)
	if err != nil {
		return nil, err
	}

	rel := filepath.ToSlash(filePath)
	var entry *FileEntry
	for i := range entries {
		if entries[i].Path == rel {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("git: file %s not found in commit %s", filePath, hash)
	}

	p, err := parentCommit(c)
	if err != nil {
		return nil, err
	}
	parentTree, err := treeOf(p)
	if err != nil {
		return nil, err
	}
	commitTree, err := treeOf(c)
	if err != nil {
		return nil, err
	}

	at := func(t *object.Tree, path string) (string, error) {
		b, err := g.blobAt(t, path)
		if err != nil {
			return "", err
		}
		if b == nil {
			// 与 CLI 后端 showFileContent 同样的处理：文件不在该 commit 里是正常情况
			g.logger.Debug("go-git show: file not in commit", "path", path)
			return "", nil
		}
		return string(b), nil
	}

	var original, modified string
	switch entry.ChangeType {
	case "added":
		modified, err = at(commitTree, entry.Path)
	case "modified":
		original, err = at(parentTree, entry.Path)
		if err == nil {
			modified, err = at(commitTree, entry.Path)
		}
	case "deleted":
		original, err = at(parentTree, entry.Path)
	case "renamed":
		original, err = at(parentTree, entry.OldPath)
		if err == nil {
			modified, err = at(commitTree, entry.Path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", hash, filePath, err)
	}

	return &FileDiff{
		Path:            filePath,
		ChangeType:      entry.ChangeType,
		OriginalContent: original,
		ModifiedContent: modified,
	}, nil
}

// ── revert ────────────────────────────────────────────────

// revertNoCommit 逆序 revert 所有 hash，仅暂存不提交。任一步失败时整体回滚。
//
// 这里刻意不做"工作区必须整体干净"的前置守卫：git CLI 的 `revert --no-commit` 只在
// revert 真正会覆盖到某个已跟踪文件时才拒绝，未跟踪文件与无关路径的改动都放行
// （Goink 的封面、导出物常常还没进 git）。下面按路径做内容比对已经覆盖了"会动到
// 的文件不允许有未提交改动"这一半语义，整体守卫反而会让回滚在常见场景下失败。
func (g *goGitBackend) revertNoCommit(hashes []string) error {
	for i := len(hashes) - 1; i >= 0; i-- {
		if err := g.revertOne(hashes[i]); err != nil {
			if abortErr := g.revertAbort(); abortErr != nil {
				return fmt.Errorf("git: revert %s: %w; abort 也失败: %v", hashes[i], err, abortErr)
			}
			return fmt.Errorf("git: revert %s: %w", hashes[i], err)
		}
	}
	return nil
}

// revertOp 是撤销单个文件变更所需的最小信息。
type revertOp struct {
	path     string // 仓库内相对路径，"/" 分隔
	expected []byte // 撤销前工作区应有的内容（nil = 该文件当时不存在）
	target   []byte // 撤销后的内容（nil = 应删除该文件）
}

// revertOne 把单个 commit 的变更反向应用到工作区并暂存。
//
// 与 CLI 的差别（有意为之，见 backend.go 顶部"已知行为差异"）：这里不做三方合并，
// 而是要求"文件当前内容 == 该 commit 的结果内容"，否则判为冲突。回滚流程 revert
// 的是连续区间且按时间逆序，每步应用完工作区就等于下一个 commit 的树，正常路径
// 必然满足；只有在区间之外又改过同一文件时才会走到冲突分支，此时整体 abort，
// 与 CLI 冲突后 abort 的表现一致。
func (g *goGitBackend) revertOne(hash string) error {
	c, err := g.commitObject(hash)
	if err != nil {
		return err
	}
	p, err := parentCommit(c)
	if err != nil {
		return err
	}
	commitTree, err := treeOf(c)
	if err != nil {
		return err
	}
	parentTree, err := treeOf(p)
	if err != nil {
		return err
	}

	// 反向遍历：from = commit 之后的树，to = commit 之前的树。
	changes, err := commitTree.DiffContext(context.Background(), parentTree)
	if err != nil {
		return fmt.Errorf("git: revert diff %s: %w", hash, err)
	}

	ops := make([]revertOp, 0, len(changes))
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return fmt.Errorf("git: revert %s: %w", hash, err)
		}
		// rename（在反向 diff 里表现为 Modify 且两侧路径不同）要拆成两步：
		// 恢复旧路径的内容，并删除 commit 引入的新路径。
		// 不假定哪一侧是旧路径 —— DetectRenames 在不同方向上的 From/To 约定很脆，
		// 直接按"两个路径各自在 commit 树与父树里的存在情况"生成操作，两种朝向都对。
		if action == merkletrie.Modify && ch.From.Name != ch.To.Name {
			for _, path := range []string{ch.From.Name, ch.To.Name} {
				expected, err := g.blobAt(commitTree, path)
				if err != nil {
					return fmt.Errorf("git: revert %s: %w", hash, err)
				}
				target, err := g.blobAt(parentTree, path)
				if err != nil {
					return fmt.Errorf("git: revert %s: %w", hash, err)
				}
				if expected == nil && target == nil {
					continue // 两棵树里都没有它：不是这条 change 关心的路径
				}
				ops = append(ops, revertOp{path: path, expected: expected, target: target})
			}
			continue
		}

		path := changePath(ch)
		expected, err := g.blobAt(commitTree, path)
		if err != nil {
			return fmt.Errorf("git: revert %s: %w", hash, err)
		}
		target, err := g.blobAt(parentTree, path)
		if err != nil {
			return fmt.Errorf("git: revert %s: %w", hash, err)
		}
		ops = append(ops, revertOp{path: path, expected: expected, target: target})
	}

	for _, op := range ops {
		full := filepath.Join(g.dir, filepath.FromSlash(op.path))
		cur, readErr := os.ReadFile(full)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("git: revert %s: read %s: %w", hash, op.path, readErr)
		}
		if !bytes.Equal(cur, op.expected) {
			return fmt.Errorf("git: revert %s: %s 已被之后的修改改动，无法干净回退（冲突）", hash, op.path)
		}

		g.reverts = append(g.reverts, revertUndo{
			path: full, content: cur, existed: readErr == nil,
		})

		if op.target == nil {
			if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("git: revert %s: remove %s: %w", hash, op.path, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return fmt.Errorf("git: revert %s: mkdir %s: %w", hash, op.path, err)
		}
		if err := os.WriteFile(full, op.target, 0644); err != nil {
			return fmt.Errorf("git: revert %s: write %s: %w", hash, op.path, err)
		}
	}

	if err := g.stageAll(); err != nil {
		return fmt.Errorf("git: revert %s: stage: %w", hash, err)
	}
	return nil
}

// revertAbort 逐文件还原 revertNoCommit 写过的工作区内容，再让索引与工作区对齐。
//
// 这里不用 `Reset(HardReset)`：go-git 的硬重置会连带删除目标树里没有的未跟踪文件
// （封面、临时导出物都可能还没进 git），而 CLI 的 `revert --abort` 只回退 revert
// 涉及的文件。逐文件还原更贴近 CLI 语义，也更容易推理。
func (g *goGitBackend) revertAbort() error {
	if len(g.reverts) == 0 {
		return nil
	}
	var errs []string
	for i := len(g.reverts) - 1; i >= 0; i-- {
		u := g.reverts[i]
		if !u.existed {
			if err := os.Remove(u.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, fmt.Sprintf("%s: %v", u.path, err))
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(u.path), 0755); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u.path, err))
			continue
		}
		if err := os.WriteFile(u.path, u.content, 0644); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u.path, err))
		}
	}
	g.reverts = nil
	if len(errs) > 0 {
		return fmt.Errorf("git: revert --abort: %s", strings.Join(errs, "; "))
	}
	return g.stageAll()
}

// ── diff ──────────────────────────────────────────────────

// diffContent 对比工作区文件与 proposed 内容。go-git 后端没有 `--no-index` 可用，
// 直接读工作区文件再自己生成 unified diff（见 unidiff.go）。
func (g *goGitBackend) diffContent(relPath, proposed string) (string, error) {
	full := filepath.Join(g.dir, relPath)
	data, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("git: diff: read %s: %w", relPath, err)
	}
	return unifiedDiff(relPath, string(data), proposed), nil
}

// ── 错误判定 ──────────────────────────────────────────────

// isNoCommits 判断"仓库还没有任何 commit"（HEAD 指向未出生分支）。
// CLI 后端靠 stderr 里的 "does not have any commits yet" 判空，这里对应
// go-git 遍历不到任何 commit 时返回的 ErrReferenceNotFound。
func isNoCommits(err error) bool {
	return errors.Is(err, plumbing.ErrReferenceNotFound)
}
