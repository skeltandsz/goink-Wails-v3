package git

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/platform"
)

// 两个后端的 parity 测试。
//
// 为什么这样组织：go-git 后端存在的唯一理由是没有 git 可执行文件的环境（Android），
// 但它的输出会被 app/history.go 直接序列化给前端的 GitHistoryList/GitCommitView，
// 所以"和 CLI 后端给出同样的结构与内容"是它最重要的正确性标准。每个用例都跑两遍
// （cli / gogit），断言只看共同语义，不去比对 diff 文本的字面格式
// （CLI 的 `git diff --no-index` 表头与 unidiff.go 生成的不同，那是已知差异）。

const (
	backendCaseCLI   = "cli"
	backendCaseGoGit = "gogit"
)

// parityBackends 是所有 parity 用例的取值。cli 用例要求系统有 git
// （repo_test.go 的 TestMain 已经保证，缺 git 时整个包直接 panic 跳过）。
var parityBackends = []string{backendCaseCLI, backendCaseGoGit}

// newParityRepo 建一个"刚完成首次 commit"的仓库，返回 Repo 与工作目录。
// 等价于生产路径里 git.New 对一个全新小说目录做的事。
func newParityRepo(t *testing.T, kind string) (*Repo, string) {
	t.Helper()

	dir := t.TempDir()
	var be backend
	switch kind {
	case backendCaseCLI:
		be = &cliBackend{dir: dir, gitBin: "git", logger: slog.Default()}
	case backendCaseGoGit:
		be = newGoGitBackend(dir, slog.Default())
	default:
		t.Fatalf("unknown backend kind %q", kind)
	}

	if err := be.initFresh("Goink", "goink@local"); err != nil {
		t.Fatalf("initFresh(%s): %v", kind, err)
	}
	return &Repo{dir: dir, backend: be, logger: slog.Default()}, dir
}

// write 在工作区写一个文件（自动建父目录），路径用 "/" 分隔。
func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// eqContent 跨后端比较文件/内容时忽略 CRLF。
//
// 为什么需要：CLI 后端把工作区文件交给真实 git 写回（revert / abort 会 checkout），
// 而开发机上用户的 global core.autocrlf 通常是 true，checkout 出来就是 CRLF；
// go-git 不做任何 EOL 转换，永远是 LF。这是 git 自身的既有差异，不是 backend 抽象引入的，
// 所以断言统一按 LF 归一化比较，不去要求两个后端字节级一致。
func eqContent(got, want string) bool {
	return strings.ReplaceAll(got, "\r\n", "\n") == want
}

// commitAll 等价于调用方的 StageAll + Commit 组合。
func commitAll(t *testing.T, r *Repo, msg string) string {
	t.Helper()
	if err := r.StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}
	hash, err := r.Commit(msg)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(hash) != 40 {
		t.Fatalf("Commit 返回的 hash 不是 40 位: %q", hash)
	}
	return hash
}

func headHash(t *testing.T, r *Repo) string {
	t.Helper()
	out, _, err := r.headRevision()
	if err != nil {
		t.Fatalf("HEAD 解析失败: %v", err)
	}
	return strings.TrimSpace(out)
}

// headRevision 是两个后端共用的"当前 HEAD hash"取法：CLI 走 rev-parse，
// go-git 没有 raw 通道，直接用它的 HEAD 引用。放在包内而非 backend 接口里，
// 是因为它只服务测试。
func (r *Repo) headRevision() (string, string, error) {
	if cb, ok := r.backend.(*cliBackend); ok {
		return cb.run("rev-parse", "HEAD")
	}
	repo, err := r.backend.(*goGitBackend).open()
	if err != nil {
		return "", "", err
	}
	ref, err := repo.Head()
	if err != nil {
		return "", "", err
	}
	return ref.Hash().String(), "", nil
}

func TestParity_InitFreshCreatesRepoWithInitialCommit(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				t.Fatalf("initFresh 后应存在 .git: %v", err)
			}
			if got := read(t, dir, "chapters/.gitkeep"); got != "" {
				t.Fatalf("chapters/.gitkeep 应为空文件，实际 %q", got)
			}

			logs, err := r.LogDetailed(10, "")
			if err != nil {
				t.Fatalf("LogDetailed: %v", err)
			}
			if len(logs) != 1 {
				t.Fatalf("首次 commit 后应有 1 条历史，实际 %d", len(logs))
			}
			if logs[0].Message != "initial commit" {
				t.Errorf("首条 commit message = %q，期望 %q", logs[0].Message, "initial commit")
			}
			if logs[0].AuthorName != "Goink" || logs[0].AuthorEmail != "goink@local" {
				t.Errorf("作者 = %q/%q，期望 Goink/goink@local", logs[0].AuthorName, logs[0].AuthorEmail)
			}
			if logs[0].ShortHash == "" || !strings.HasPrefix(logs[0].Hash, logs[0].ShortHash) {
				t.Errorf("ShortHash %q 不是 Hash %q 的前缀", logs[0].ShortHash, logs[0].Hash)
			}

			info, entries, err := r.CommitFileList(logs[0].Hash)
			if err != nil {
				t.Fatalf("CommitFileList: %v", err)
			}
			if info.Hash != logs[0].Hash {
				t.Errorf("CommitFileList 返回的 hash %q 与请求 %q 不一致", info.Hash, logs[0].Hash)
			}
			if len(entries) != 1 || entries[0].Path != "chapters/.gitkeep" || entries[0].ChangeType != "added" {
				t.Errorf("首 commit 的变更列表 = %+v，期望单条 chapters/.gitkeep added", entries)
			}
		})
	}
}

func TestParity_StageCommitAndStatus(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			dirty, err := r.HasUncommitted()
			if err != nil {
				t.Fatalf("HasUncommitted: %v", err)
			}
			if dirty {
				t.Fatal("刚初始化完不应有未提交变更")
			}

			write(t, dir, "chapters/id_1.md", "第一章\n正文\n")
			dirty, err = r.HasUncommitted()
			if err != nil {
				t.Fatalf("HasUncommitted: %v", err)
			}
			if !dirty {
				t.Fatal("新增未跟踪文件应被 HasUncommitted 看到（等价 status --porcelain）")
			}

			h1 := commitAll(t, r, "add chapter 1")
			dirty, _ = r.HasUncommitted()
			if dirty {
				t.Fatal("提交后不应还有未提交变更")
			}

			write(t, dir, "chapters/id_1.md", "第一章\n正文\n新增一段\n")
			h2 := commitAll(t, r, "update chapter 1")
			if h2 == h1 {
				t.Fatal("两次 commit 返回了同一个 hash")
			}

			logs, err := r.LogDetailed(10, "")
			if err != nil {
				t.Fatalf("LogDetailed: %v", err)
			}
			if len(logs) != 3 {
				t.Fatalf("应有 3 条历史，实际 %d", len(logs))
			}
			// 降序：最新在前
			if logs[0].Hash != h2 || logs[1].Hash != h1 {
				t.Errorf("历史顺序不对: %q %q %q", logs[0].Hash, logs[1].Hash, logs[2].Hash)
			}
			if logs[0].FilesChanged != 1 {
				t.Errorf("FilesChanged = %d，期望 1", logs[0].FilesChanged)
			}
			if logs[0].Insertions != 1 || logs[0].Deletions != 0 {
				t.Errorf("numstat = +%d/-%d，期望 +1/-0", logs[0].Insertions, logs[0].Deletions)
			}
			if logs[0].Message != "update chapter 1" {
				t.Errorf("Message = %q", logs[0].Message)
			}
		})
	}
}

func TestParity_DeletionAndModificationChangeTypes(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "一\n")
			write(t, dir, "outlines/id_1.md", "大纲一\n")
			h1 := commitAll(t, r, "add chapter and outline")

			if err := os.Remove(filepath.Join(dir, "outlines", "id_1.md")); err != nil {
				t.Fatalf("remove: %v", err)
			}
			write(t, dir, "chapters/id_1.md", "一\n改过\n")
			h2 := commitAll(t, r, "delete outline, edit chapter")

			_, entries, err := r.CommitFileList(h2)
			if err != nil {
				t.Fatalf("CommitFileList: %v", err)
			}
			got := map[string]string{}
			for _, e := range entries {
				got[e.Path] = e.ChangeType
			}
			if got["outlines/id_1.md"] != "deleted" {
				t.Errorf("outlines/id_1.md = %q，期望 deleted（%+v）", got["outlines/id_1.md"], entries)
			}
			if got["chapters/id_1.md"] != "modified" {
				t.Errorf("chapters/id_1.md = %q，期望 modified（%+v）", got["chapters/id_1.md"], entries)
			}

			diff, err := r.ShowFile(h2, "chapters/id_1.md")
			if err != nil {
				t.Fatalf("ShowFile modified: %v", err)
			}
			if !eqContent(diff.OriginalContent, "一\n") || !eqContent(diff.ModifiedContent, "一\n改过\n") {
				t.Errorf("modified 文件前后内容 = %q / %q", diff.OriginalContent, diff.ModifiedContent)
			}

			diff, err = r.ShowFile(h2, "outlines/id_1.md")
			if err != nil {
				t.Fatalf("ShowFile deleted: %v", err)
			}
			if !eqContent(diff.OriginalContent, "大纲一\n") || diff.ModifiedContent != "" {
				t.Errorf("deleted 文件前后内容 = %q / %q", diff.OriginalContent, diff.ModifiedContent)
			}

			diff, err = r.ShowFile(h1, "chapters/id_1.md")
			if err != nil {
				t.Fatalf("ShowFile added: %v", err)
			}
			if diff.ChangeType != "added" || diff.OriginalContent != "" || !eqContent(diff.ModifiedContent, "一\n") {
				t.Errorf("added 文件 = %+v", diff)
			}

			if _, err := r.ShowFile(h2, "chapters/不存在.md"); err == nil {
				t.Error("查询不在该 commit 里的文件应报错")
			}
		})
	}
}

func TestParity_RenameDetected(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			content := strings.Repeat("这一章的内容足够长以便相似度过阈值\n", 20)
			write(t, dir, "chapters/id_1.md", content)
			commitAll(t, r, "add chapter")

			if err := os.Rename(filepath.Join(dir, "chapters", "id_1.md"),
				filepath.Join(dir, "chapters", "id_2.md")); err != nil {
				t.Fatalf("rename: %v", err)
			}
			h := commitAll(t, r, "rename id_1 to id_2")

			_, entries, err := r.CommitFileList(h)
			if err != nil {
				t.Fatalf("CommitFileList: %v", err)
			}
			var renamed *FileEntry
			for i := range entries {
				if entries[i].ChangeType == "renamed" {
					renamed = &entries[i]
				}
			}
			if renamed == nil {
				t.Fatalf("重命名应识别为 renamed，实际 %+v", entries)
			}
			if renamed.Path != "chapters/id_2.md" || renamed.OldPath != "chapters/id_1.md" {
				t.Errorf("renamed 条目 = %+v，期望 chapters/id_2.md ← chapters/id_1.md", renamed)
			}

			diff, err := r.ShowFile(h, "chapters/id_2.md")
			if err != nil {
				t.Fatalf("ShowFile renamed: %v", err)
			}
			if diff.OriginalContent != content || diff.ModifiedContent != content {
				t.Errorf("renamed 文件前后内容不一致：len %d / %d", len(diff.OriginalContent), len(diff.ModifiedContent))
			}
		})
	}
}

func TestParity_NonASCIIPath(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/我的章节.md", "正文\n")
			h := commitAll(t, r, "add 中文章节")

			_, entries, err := r.CommitFileList(h)
			if err != nil {
				t.Fatalf("CommitFileList: %v", err)
			}
			if len(entries) != 1 || entries[0].Path != "chapters/我的章节.md" {
				t.Fatalf("变更列表 = %+v，期望原样 UTF-8 路径（不能是 \\346\\210\\221 转义）", entries)
			}

			diff, err := r.ShowFile(h, "chapters/我的章节.md")
			if err != nil {
				t.Fatalf("ShowFile 应能用前端传入的 UTF-8 路径命中: %v", err)
			}
			if !eqContent(diff.ModifiedContent, "正文\n") {
				t.Errorf("内容 = %q", diff.ModifiedContent)
			}
		})
	}
}

func TestParity_RevertNoCommitAndCommit(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "第一版\n")
			h1 := commitAll(t, r, "turn 1")
			write(t, dir, "chapters/id_1.md", "第二版\n")
			h2 := commitAll(t, r, "turn 2")
			write(t, dir, "chapters/id_2.md", "新章\n")
			h3 := commitAll(t, r, "turn 3")

			// 回滚 turn 2..3（rollback.RollbackBeforeTurn 的调用形状：逆序、只暂存）
			if err := r.RevertNoCommit([]string{h2, h3}); err != nil {
				t.Fatalf("RevertNoCommit: %v", err)
			}
			if got := read(t, dir, "chapters/id_1.md"); !eqContent(got, "第一版\n") {
				t.Errorf("revert 后 id_1 = %q，期望回到第一版", got)
			}
			if _, err := os.Stat(filepath.Join(dir, "chapters", "id_2.md")); !os.IsNotExist(err) {
				t.Errorf("revert 新增文件后应删除它，stat = %v", err)
			}
			dirty, err := r.HasUncommitted()
			if err != nil {
				t.Fatalf("HasUncommitted: %v", err)
			}
			if !dirty {
				t.Fatal("revert --no-commit 的结果应留在索引/工作区，未提交")
			}
			if head := headHash(t, r); head != h3 {
				t.Fatalf("revert --no-commit 不该动 HEAD: %q != %q", head, h3)
			}

			h4 := commitAll(t, r, "revert turn 2-3")
			logs, err := r.LogDetailed(10, "")
			if err != nil {
				t.Fatalf("LogDetailed: %v", err)
			}
			if len(logs) != 5 || logs[0].Hash != h4 {
				t.Fatalf("历史 = %d 条，期望 5 条且最新是 revert", len(logs))
			}
			if got := read(t, dir, "chapters/id_1.md"); !eqContent(got, "第一版\n") {
				t.Errorf("提交 revert 后 id_1 = %q", got)
			}
			if logs[1].Hash != h3 || logs[2].Hash != h2 || logs[3].Hash != h1 {
				t.Errorf("revert 之后的历史顺序不对: %q %q %q %q",
					logs[0].Hash, logs[1].Hash, logs[2].Hash, logs[3].Hash)
			}
		})
	}
}

func TestParity_RevertConflictAbortsEverything(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "A\n")
			write(t, dir, "chapters/id_2.md", "B\n")
			h1 := commitAll(t, r, "turn 1")
			write(t, dir, "chapters/id_1.md", "A2\n")
			h2 := commitAll(t, r, "turn 2")

			// 只回退 turn 1：id_1 在 turn 2 又被改过，两个后端都判冲突
			// （CLI 由 git 的合并机制报冲突，go-git 由内容精确匹配失败报冲突）。
			if err := r.RevertNoCommit([]string{h1}); err == nil {
				t.Fatal("同一文件被后续 commit 改过后 revert 应报冲突")
			}
			// 失败必须整体回滚：HEAD 不动、工作区保持原样、id_2 不被单独改掉
			if got := read(t, dir, "chapters/id_1.md"); !eqContent(got, "A2\n") {
				t.Errorf("冲突 abort 后 id_1 = %q，期望保持 A2", got)
			}
			if got := read(t, dir, "chapters/id_2.md"); !eqContent(got, "B\n") {
				t.Errorf("冲突 abort 后 id_2 = %q，期望保持 B", got)
			}
			if head := headHash(t, r); head != h2 {
				t.Errorf("冲突后 HEAD = %q，期望仍是 %q", head, h2)
			}
		})
	}
}

// TestGoGit_RevertAbortRestoresWorktree 只跑 go-git 后端。
//
// 为什么不做 CLI parity：CLI 的 RevertAbort 直接透传 `git revert --abort`，而 git 只在
// "有 revert 序列在进行中"（即冲突）时接受这条命令；一次干净成功的 revert 之后调用它，
// git 会报 "there is no checkout in progress"。也就是说"revert 成功后再由上层决定
// 放弃"这个调用形状在 CLI 里本来就不成立（rollback.go 在 DB 事务失败时正是这样调的，
// 属于既有问题，见 docs/todo/rollback-known-issues.md，不在本次改动范围内）。
// go-git 后端按快照逐文件还原，这个形状是成立且可测的，所以单独覆盖。
func TestGoGit_RevertAbortRestoresWorktree(t *testing.T) {
	for _, kind := range []string{backendCaseGoGit} {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "A\n")
			commitAll(t, r, "turn 1")
			write(t, dir, "chapters/id_1.md", "B\n")
			h2 := commitAll(t, r, "turn 2")
			// 未跟踪文件不能被 revert 流程碰掉
			write(t, dir, "cover-draft.jpg", "还没进 git 的草稿")

			if err := r.RevertNoCommit([]string{h2}); err != nil {
				t.Fatalf("RevertNoCommit: %v", err)
			}
			if err := r.RevertAbort(); err != nil {
				t.Fatalf("RevertAbort: %v", err)
			}
			if got := read(t, dir, "chapters/id_1.md"); !eqContent(got, "B\n") {
				t.Errorf("abort 后 id_1 = %q，期望回到 abort 前的 B", got)
			}
			if got := read(t, dir, "cover-draft.jpg"); got != "还没进 git 的草稿" {
				t.Errorf("未跟踪文件被 abort 流程影响: %q", got)
			}
			dirty, err := r.HasUncommitted()
			if err != nil {
				t.Fatalf("HasUncommitted: %v", err)
			}
			if !dirty {
				t.Error("未跟踪的 cover-draft.jpg 应让工作区仍是 dirty")
			}
			if head := headHash(t, r); head != h2 {
				t.Errorf("abort 不该动 HEAD: %q != %q", head, h2)
			}
		})
	}
}

func TestParity_RevertRename(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			content := strings.Repeat("重命名前的正文\n", 20)
			write(t, dir, "chapters/id_1.md", content)
			commitAll(t, r, "add chapter")

			if err := os.Rename(filepath.Join(dir, "chapters", "id_1.md"),
				filepath.Join(dir, "chapters", "id_9.md")); err != nil {
				t.Fatalf("rename: %v", err)
			}
			h := commitAll(t, r, "rename id_1 to id_9")

			if err := r.RevertNoCommit([]string{h}); err != nil {
				t.Fatalf("RevertNoCommit(重命名): %v", err)
			}
			// revert 一次重命名 = 旧路径回来、新路径消失，且内容一致
			if got := read(t, dir, "chapters/id_1.md"); !eqContent(got, content) {
				t.Errorf("revert 后 id_1 内容长度 %d，期望 %d", len(got), len(content))
			}
			if _, err := os.Stat(filepath.Join(dir, "chapters", "id_9.md")); !os.IsNotExist(err) {
				t.Errorf("revert 后 id_9 应已删除，stat = %v", err)
			}
		})
	}
}

// TestParity_BinaryFileStats 锁住二进制文件（封面 jpg）的 numstat 语义：
// git CLI 对二进制输出 "-\t-\tpath"，parseDetailedLog 解析后增删行数都是 0，
// 但文件仍算一个变更文件。go-git 后端必须给同样的结果，否则手机上提交一次封面，
// 历史列表就会显示"+123456 / -123456 行"这种荒谬数字。
func TestParity_BinaryFileStats(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			binary := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x00, 0x01, 0x02, 0x03, 0x00, 0x00, 0xff, 0xd9}
			if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), binary, 0o644); err != nil {
				t.Fatalf("write cover: %v", err)
			}
			h := commitAll(t, r, "add cover")

			logs, err := r.LogDetailed(10, "")
			if err != nil {
				t.Fatalf("LogDetailed: %v", err)
			}
			if logs[0].Hash != h {
				t.Fatalf("最新历史不是刚提交的 %q", h)
			}
			if logs[0].FilesChanged != 1 {
				t.Errorf("FilesChanged = %d，期望 1（二进制仍算一个变更文件）", logs[0].FilesChanged)
			}
			if logs[0].Insertions != 0 || logs[0].Deletions != 0 {
				t.Errorf("二进制文件行数统计 = +%d/-%d，期望 +0/-0（与 git 的 -\\t- 一致）",
					logs[0].Insertions, logs[0].Deletions)
			}

			// 覆盖一次"改封面"：内容变化要仍算 1 个文件、0 行
			if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), append(binary, 0x00, 0x01), 0o644); err != nil {
				t.Fatalf("rewrite cover: %v", err)
			}
			h2 := commitAll(t, r, "replace cover")
			logs, err = r.LogDetailed(10, "")
			if err != nil {
				t.Fatalf("LogDetailed: %v", err)
			}
			if logs[0].Hash != h2 || logs[0].FilesChanged != 1 {
				t.Errorf("换封面后的历史 = %+v，期望 FilesChanged 1", logs[0])
			}
			if logs[0].Insertions != 0 || logs[0].Deletions != 0 {
				t.Errorf("换封面的行数统计 = +%d/-%d，期望 +0/-0",
					logs[0].Insertions, logs[0].Deletions)
			}
		})
	}
}

func TestParity_SetGitConfigChangesAuthor(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			if err := r.SetGitConfig("笔名", "bi@ming.local"); err != nil {
				t.Fatalf("SetGitConfig: %v", err)
			}
			write(t, dir, "chapters/id_1.md", "A\n")
			h := commitAll(t, r, "by 笔名")

			info, _, err := r.CommitFileList(h)
			if err != nil {
				t.Fatalf("CommitFileList: %v", err)
			}
			if info.AuthorName != "笔名" || info.AuthorEmail != "bi@ming.local" {
				t.Errorf("作者 = %q/%q，期望 笔名/bi@ming.local", info.AuthorName, info.AuthorEmail)
			}
		})
	}
}

func TestParity_LogByPath(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "1\n")
			h1 := commitAll(t, r, "c1")
			write(t, dir, "chapters/id_2.md", "2\n")
			commitAll(t, r, "c2")
			write(t, dir, "chapters/id_1.md", "1b\n")
			h3 := commitAll(t, r, "c3")

			logs, err := r.Log("chapters/id_1.md", 0)
			if err != nil {
				t.Fatalf("Log: %v", err)
			}
			seen := map[string]bool{}
			for _, l := range logs {
				seen[l.Hash] = true
			}
			if !seen[h1] || !seen[h3] {
				t.Errorf("按路径过滤历史缺 commit: %+v", logs)
			}
			// c2 只动 id_2.md，不该出现在 id_1.md 的历史里
			for _, l := range logs {
				if l.Hash != h1 && l.Hash != h3 {
					t.Errorf("历史里出现了不相关的 commit %q", l.Hash)
				}
			}

			// 分页语义（app/history.go 用 afterHash 取更早一页）：
			// h3 之后是 c2、c1，再加初始化的 initial commit，共 3 条。
			page, err := r.LogDetailed(10, h3)
			if err != nil {
				t.Fatalf("LogDetailed afterHash: %v", err)
			}
			for _, l := range page {
				if l.Hash == h3 {
					t.Error("afterHash 之后的那一页不该包含 afterHash 本身")
				}
			}
			if len(page) != 3 {
				t.Errorf("afterHash 之后应有 3 条，实际 %d", len(page))
			}
		})
	}
}

func TestParity_DiffContent(t *testing.T) {
	for _, kind := range parityBackends {
		t.Run(kind, func(t *testing.T) {
			r, dir := newParityRepo(t, kind)

			write(t, dir, "chapters/id_1.md", "原第一行\n原第二行\n")
			commitAll(t, r, "base")

			out, err := r.DiffContent("chapters/id_1.md", "原第一行\n改过的第二行\n新增第三行\n")
			if err != nil {
				t.Fatalf("DiffContent: %v", err)
			}
			if !strings.Contains(out, "+改过的第二行") || !strings.Contains(out, "+新增第三行") {
				t.Errorf("diff 缺少新增行: %q", out)
			}
			if !strings.Contains(out, "-原第二行") {
				t.Errorf("diff 缺少删除行: %q", out)
			}

			// 文件不存在时以空内容为基准（CLI 走 /dev/null）
			out, err = r.DiffContent("chapters/id_9.md", "全新内容\n")
			if err != nil {
				t.Fatalf("DiffContent 新文件: %v", err)
			}
			if !strings.Contains(out, "+全新内容") {
				t.Errorf("新文件 diff = %q", out)
			}
		})
	}
}

// TestNewWithoutSystemGitStillWorks 复现并锁死用户报的那个故障：
// Android APK 里没有 git 可执行文件，git.New 直接返回
// "git: 找不到 git 可执行文件: git: 找不到可用的 git 可执行文件，请安装 Git"，
// 建库/保存章节/历史/回滚全线失败。这里把 PATH 换成空目录制造"系统里没有 git"，
// 断言 New 不再报错、自动落到 go-git 后端，并且能完成一次真实的写章节→提交→查历史。
func TestNewWithoutSystemGitStillWorks(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("GOINK_DATA_DIR", dataDir)
	t.Setenv("GOINK_GIT_BIN", "")
	t.Setenv("GOINK_GIT_BACKEND", "")
	t.Setenv("GOINK_E2E_STRICT", "")
	t.Setenv("PATH", t.TempDir())

	if _, err := exec.LookPath("git"); err == nil {
		t.Skip("清空 PATH 后仍能找到 git（环境特殊），跳过")
	}

	// git.New 依赖 config 单例与 config.NovelDirPath，测试里直接注入
	config.Set(&config.AppConfig{})
	t.Cleanup(func() { config.Set(nil) })

	const novelID = int64(7)
	r, err := New(novelID, "", "", nil)
	if err != nil {
		t.Fatalf("没有 git 时 New 不该失败: %v", err)
	}
	if got := r.BackendName(); got != "gogit" {
		t.Fatalf("BackendName = %s，期望 gogit", got)
	}
	if _, err := os.Stat(filepath.Join(config.NovelDirPath(novelID), ".git")); err != nil {
		t.Fatalf("New 应完成建库: %v", err)
	}

	// 走一遍 app/novel.go + app/chat.go 的实际调用形状：写文件 → StageAll → Commit → 查历史
	if err := WriteFile(novelID, ChapterPath(1), "第一章正文\n"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := r.StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}
	hash, err := r.Commit("保存章节 1")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	logs, err := r.LogDetailed(10, "")
	if err != nil {
		t.Fatalf("LogDetailed: %v", err)
	}
	if len(logs) != 2 || logs[0].Hash != hash {
		t.Fatalf("历史 = %+v，期望 2 条且最新是刚才的提交", logs)
	}
	diff, err := r.ShowFile(hash, ChapterPath(1))
	if err != nil {
		t.Fatalf("ShowFile: %v", err)
	}
	if diff.ModifiedContent != "第一章正文\n" {
		t.Errorf("ShowFile 内容 = %q", diff.ModifiedContent)
	}
}

// TestNewBackendSelection 覆盖 newBackend 的三条分支：显式指定、自动降级到 go-git、
// E2E strict 下拒绝降级。这是 Android 能跑起来的关键判定点。
func TestNewBackendSelection(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("本机没有 git，跳过后端选择测试")
	}

	dataDir := t.TempDir()
	t.Setenv("GOINK_DATA_DIR", dataDir)
	platform.ResetDataDirCache()
	defer platform.ResetDataDirCache()

	dir := t.TempDir()
	logger := slog.Default()

	t.Run("gogit 显式指定", func(t *testing.T) {
		t.Setenv(envBackend, "gogit")
		be, err := newBackend(dir, logger)
		if err != nil {
			t.Fatalf("newBackend: %v", err)
		}
		if be.name() != "gogit" {
			t.Errorf("backend = %s，期望 gogit", be.name())
		}
	})

	t.Run("cli 显式指定但找不到 git 时报错不降级", func(t *testing.T) {
		t.Setenv(envBackend, "cli")
		t.Setenv("GOINK_GIT_BIN", filepath.Join(t.TempDir(), "no-such-git"))
		if _, err := newBackend(dir, logger); err == nil {
			t.Fatal("GOINK_GIT_BACKEND=cli + 无效 GOINK_GIT_BIN 应报错")
		}
	})

	t.Run("非法取值报错", func(t *testing.T) {
		t.Setenv(envBackend, "svn")
		if _, err := newBackend(dir, logger); err == nil {
			t.Fatal("GOINK_GIT_BACKEND 非法取值应报错")
		}
	})

	t.Run("默认有 git 时用 cli", func(t *testing.T) {
		t.Setenv(envBackend, "")
		t.Setenv("GOINK_GIT_BIN", gitBin)
		be, err := newBackend(dir, logger)
		if err != nil {
			t.Fatalf("newBackend: %v", err)
		}
		if be.name() != "cli" {
			t.Errorf("backend = %s，期望 cli", be.name())
		}
	})

	// 下面两例要制造"系统里没有 git"的环境：把 PATH 换成空目录，
	// 同时保证 bundled / DataDir/runtime/git 都不存在。
	t.Run("无 git 时降级到 gogit", func(t *testing.T) {
		t.Setenv(envBackend, "")
		t.Setenv("GOINK_GIT_BIN", "")
		t.Setenv("GOINK_E2E_STRICT", "")
		t.Setenv("PATH", t.TempDir())
		be, err := newBackend(dir, logger)
		if err != nil {
			t.Fatalf("无 git 环境里 newBackend 不该失败: %v", err)
		}
		if be.name() != "gogit" {
			t.Errorf("backend = %s，期望 gogit", be.name())
		}
	})

	t.Run("E2E strict 下无 git 直接报错", func(t *testing.T) {
		t.Setenv(envBackend, "")
		t.Setenv("GOINK_GIT_BIN", "")
		t.Setenv("GOINK_E2E_STRICT", "1")
		t.Setenv("PATH", t.TempDir())
		if _, err := newBackend(dir, logger); err == nil {
			t.Fatal("GOINK_E2E_STRICT 下必须拒绝降级到 go-git")
		}
	})
}
