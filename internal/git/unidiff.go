package git

import (
	"fmt"
	"path/filepath"
	"strings"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// unidiff.go 是 go-git 后端的 unified diff 生成器。
//
// 为什么自己写：CLI 后端靠 `git diff --no-index -- <当前文件> <临时文件>` 得到
// unified diff，go-git 没有等价入口（它的 diff 需要对象库里的两个 blob/tree）。
// 直接借用 go-git 已经在用的 diffmatchpatch 做行级比较，再按 git 的 hunk 规则
// 输出，能保持前端 GitCommitView/useFileDiff 拿到的文本格式一致。
//
// 与 `git diff` 的已知差异（都不影响现有消费方）：
//   - 没有 `index abc..def 100644` 这类需要对象 hash 的表头行；
//   - 末行缺换行符时补一个换符输出，不加 `\ No newline at end of file` 标记；
//   - 上下文行数固定 3（与 git 默认一致）。
//
// 消费方：Repo.DiffContent 目前只被 internal/e2e 的 git_operations_test 用到，
// 前端只把它当字符串展示，不解析结构。

const diffContextLines = 3

// unifiedDiff 返回 from → to 的 unified diff 文本；内容相同（或都为空）时返回空串。
func unifiedDiff(relPath, from, to string) string {
	if from == to {
		return ""
	}

	lines := diffLines(from, to)
	hunks := buildHunks(lines, diffContextLines)
	if len(hunks) == 0 {
		return ""
	}

	slashed := filepath.ToSlash(relPath)
	fromLabel := "a/" + slashed
	toLabel := "b/" + slashed
	if from == "" {
		fromLabel = "/dev/null"
	}
	if to == "" {
		toLabel = "/dev/null"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", fromLabel, toLabel)
	for _, h := range hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount)
		for _, ln := range lines[h.start:h.end] {
			b.WriteByte(ln.op)
			b.WriteString(ln.text)
			if !strings.HasSuffix(ln.text, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// diffLine 是带 diff 标记的单个物理行（text 含自身的换行符）。
type diffLine struct {
	op   byte // ' ' 上下文、'-' 删除、'+' 新增
	text string
}

// diffLines 用 diffmatchpatch 的行模式做比较，展开成逐行标记。
func diffLines(from, to string) []diffLine {
	d := dmp.New()
	src, dst, lineArray := d.DiffLinesToRunes(from, to)
	// 这里已经按行编码（每个 rune 代表一行），checklines 传 false 就是"逐行比较"。
	// 不要再调 DiffCleanupMerge：它对"已还原成真实文本"的 Diff 做字符级前后缀清理，
	// 会把一行从中间劈开（第一次实现就出现过 "原第二" / "行" 这种碎片）。
	diffs := d.DiffMainRunes(src, dst, false)
	diffs = d.DiffCharsToLines(diffs, lineArray)

	out := make([]diffLine, 0, len(lineArray))
	for _, diff := range diffs {
		op := byte(' ')
		switch diff.Type {
		case dmp.DiffDelete:
			op = '-'
		case dmp.DiffInsert:
			op = '+'
		}
		for _, ln := range splitPhysicalLines(diff.Text) {
			out = append(out, diffLine{op: op, text: ln})
		}
	}
	return out
}

// splitPhysicalLines 把一段 diff 文本切成"含各自换行符"的行。
func splitPhysicalLines(s string) []string {
	var res []string
	for s != "" {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			res = append(res, s)
			break
		}
		res = append(res, s[:i+1])
		s = s[i+1:]
	}
	return res
}

// hunk 是输出的一段 @@ 块，用 [start,end) 指向 lines 里的区间。
type hunk struct {
	start, end         int
	oldStart, oldCount int
	newStart, newCount int
}

// buildHunks 按 git 规则分组：每个变更点上下各留 ctx 行上下文，
// 间隔不超过 2*ctx 的相邻变更合并进同一个 hunk。
func buildHunks(lines []diffLine, ctx int) []hunk {
	changed := make([]int, 0, len(lines))
	for i, ln := range lines {
		if ln.op != ' ' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}

	// old/new 起始行号要在整份文件里数出来：op != '+' 占旧文件一行，op != '-' 占新文件一行。
	// hunks 按顺序生成，所以用 scanned 增量计数，不必每个 hunk 从头数。
	var oldNo, newNo, scanned int
	mark := func(upTo int) (int, int) {
		for ; scanned < upTo; scanned++ {
			if lines[scanned].op != '+' {
				oldNo++
			}
			if lines[scanned].op != '-' {
				newNo++
			}
		}
		return oldNo, newNo
	}

	var hunks []hunk
	flush := func(groupStart, lastChanged int) {
		start := groupStart - ctx
		if start < 0 {
			start = 0
		}
		end := lastChanged + 1 + ctx
		if end > len(lines) {
			end = len(lines)
		}
		o, n := mark(start)
		h := hunk{start: start, end: end, oldStart: o + 1, newStart: n + 1}
		for _, ln := range lines[start:end] {
			if ln.op != '+' {
				h.oldCount++
			}
			if ln.op != '-' {
				h.newCount++
			}
		}
		// git 的空侧约定：count 为 0 时起始行号也写 0（@@ -0,0 +1,3 @@）
		if h.oldCount == 0 {
			h.oldStart = 0
		}
		if h.newCount == 0 {
			h.newStart = 0
		}
		hunks = append(hunks, h)
	}

	groupStart, prev := changed[0], changed[0]
	for _, i := range changed[1:] {
		if i-prev <= 2*ctx {
			prev = i
			continue
		}
		flush(groupStart, prev)
		groupStart, prev = i, i
	}
	flush(groupStart, prev)
	return hunks
}
