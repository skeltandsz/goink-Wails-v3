package git

import (
	"fmt"
	"strings"
)

// 本文件为 go-git 后端计算"行级"差异：git 的 numstat（增删行数）与 DiffContent
// 需要的 unified diff 都按行比较，而 go-git 只给到 blob/tree 级差异。
//
// 为什么不用 go-git 自带的 utils/diff：那是 diffmatchpatch 的字符级算法，会把一行
// 拆成"等/增/删"混在中间，统计结果与 git 的行级 numstat 对不上（git 的 xdiff 先按行
// 做 Myers）。这里同源，按行做 Myers。
//
// 与 CLI 后端的已知差异（记在这里，免得以后当 bug 查）：
//   - DiffContent 的文本里没有 `index <sha>..<sha>` 与 mode 行（需要 blob hash 与
//     文件权限）。前端只按 ---/+++/@@ 解析；目前 DiffContent 也没有生产调用方
//     （只有 e2e 断言，而 e2e 跑在桌面 = CLI 后端）。
//   - 差异规模超过 myersLineLimit 时退化成"整块删 + 整块增"：行数依旧正确，hunk 变粗。

const (
	// 行级 Myers 的规模上限（n+m）。章节一般几百到几千行、差异几十行，远达不到；
	// 上限是为了 AI 一次性重写超长章节时不把 GUI 进程内存吃满（OOM 是整个应用消失）。
	myersLineLimit = 60000
	diffContext    = 3
)

type lineOp struct {
	kind  byte // '=' 相同, '-' 删除（a 侧）, '+' 新增（b 侧）
	aLine int  // a 中 1-based 行号；kind=='+' 时无意义
	bLine int  // b 中 1-based 行号；kind=='-' 时无意义
}

// splitLines 按 \n 切行，保留行内原有的 \r —— 与 git 一致：只改行尾也算差异。
// 空串表示"没有内容"，返回 nil（用于把新增/删除文件算成 0 行基准）。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// diffLines 返回把 a 变成 b 的编辑脚本（按行，正序，行号 1-based）。
func diffLines(a, b []string) []lineOp {
	n, m := len(a), len(b)

	// 先剥公共前缀/后缀：章节编辑几乎都是"中间改几行"，这一步把要跑 Myers 的规模
	// 直接压到差异区，比任何 heuristics 都有效。
	head := 0
	for head < n && head < m && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < n-head && tail < m-head && a[n-1-tail] == b[m-1-tail] {
		tail++
	}

	ops := make([]lineOp, 0, head+tail+(n-head-tail)+(m-head-tail))
	for i := 0; i < head; i++ {
		ops = append(ops, lineOp{kind: '=', aLine: i + 1, bLine: i + 1})
	}
	ops = append(ops, myersOps(a[head:n-tail], b[head:m-tail], head+1, head+1)...)
	for i := 0; i < tail; i++ {
		ops = append(ops, lineOp{kind: '=', aLine: n - tail + i + 1, bLine: m - tail + i + 1})
	}
	return ops
}

// myersOps 对差异区跑 Myers O(ND)，行号从 aBase/bBase 起算。
func myersOps(a, b []string, aBase, bBase int) []lineOp {
	n, m := len(a), len(b)
	switch {
	case n == 0 && m == 0:
		return nil
	case n == 0:
		return insertAll(m, bBase)
	case m == 0:
		return deleteAll(n, aBase)
	case n+m > myersLineLimit:
		// 退化路径：见文件头注释。行数仍与真实差异一致，只是 hunk 不再最小。
		return append(deleteAll(n, aBase), insertAll(m, bBase)...)
	}

	max := n + m
	offset := max
	v := make([]int, 2*max+1)
	trace := make([][]int, 0, max+1)

	found := -1
	for d := 0; d <= max && found < 0; d++ {
		// 存"本条 snake 扩展之前"的 V，回溯时按轮次取用。
		snapshot := make([]int, len(v))
		copy(snapshot, v)
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				found = d
				break
			}
		}
	}
	if found < 0 {
		// D 的上界就是 n+m，理论不可达。真走到这里说明界算错了：宁可退化成大块
		// 差异也不能 panic —— 版本历史比 diff 精度重要。
		return append(deleteAll(n, aBase), insertAll(m, bBase)...)
	}
	return backtrack(trace, a, b, offset)
}

// backtrack 从最后一轮 V 快照倒推编辑脚本，返回正序结果。
func backtrack(trace [][]int, a, b []string, offset int) []lineOp {
	x, y := len(a), len(b)
	rev := make([]lineOp, 0, len(a)+len(b))
	for d := len(trace) - 1; d >= 0 && (x > 0 || y > 0); d-- {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			rev = append(rev, lineOp{kind: '=', aLine: x + 1, bLine: y + 1})
		}
		if d > 0 {
			if x == prevX {
				y--
				rev = append(rev, lineOp{kind: '+', bLine: y + 1})
			} else {
				x--
				rev = append(rev, lineOp{kind: '-', aLine: x + 1})
			}
		}
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func insertAll(m, bBase int) []lineOp {
	ops := make([]lineOp, 0, m)
	for i := 0; i < m; i++ {
		ops = append(ops, lineOp{kind: '+', bLine: bBase + i})
	}
	return ops
}

func deleteAll(n, aBase int) []lineOp {
	ops := make([]lineOp, 0, n)
	for i := 0; i < n; i++ {
		ops = append(ops, lineOp{kind: '-', aLine: aBase + i})
	}
	return ops
}

// countLineDiff 给出行级 numstat（新增/删除行数），语义对齐 git：新增或删除文件
// 以空侧为 0 行基准。
func countLineDiff(a, b string) (added, deleted int) {
	for _, op := range diffLines(splitLines(a), splitLines(b)) {
		switch op.kind {
		case '+':
			added++
		case '-':
			deleted++
		}
	}
	return added, deleted
}

// unifiedDiff 生成 git 风格的 unified diff（-U3）。relPath 用斜杠路径。
func unifiedDiff(relPath, a, b string) string {
	aLines, bLines := splitLines(a), splitLines(b)
	ops := diffLines(aLines, bLines)

	var sb strings.Builder
	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", relPath, relPath)
	if a == "" {
		sb.WriteString("--- /dev/null\n")
	} else {
		fmt.Fprintf(&sb, "--- a/%s\n", relPath)
	}
	if b == "" {
		sb.WriteString("+++ /dev/null\n")
	} else {
		fmt.Fprintf(&sb, "+++ b/%s\n", relPath)
	}

	for _, h := range groupHunks(ops, len(aLines), len(bLines)) {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", h.aStart, h.aCount, h.bStart, h.bCount)
		for _, op := range h.ops {
			switch op.kind {
			case '=':
				sb.WriteByte(' ')
				writeLine(&sb, aLines, op.aLine)
			case '-':
				sb.WriteByte('-')
				writeLine(&sb, aLines, op.aLine)
			case '+':
				sb.WriteByte('+')
				writeLine(&sb, bLines, op.bLine)
			}
		}
	}
	return sb.String()
}

func writeLine(sb *strings.Builder, lines []string, line int) {
	if line >= 1 && line <= len(lines) {
		sb.WriteString(lines[line-1])
	}
	sb.WriteByte('\n')
}

type hunk struct {
	ops                      []lineOp
	aStart, aCount, bStart, bCount int
}

// groupHunks 把编辑脚本切成 hunk：两段差异相隔 <= 2*diffContext 行相同内容就合并，
// 每段前后各带最多 diffContext 行上下文 —— 与 git 默认 -U3 一致。
func groupHunks(ops []lineOp, aLen, bLen int) []hunk {
	var changed []int
	for i, op := range ops {
		if op.kind != '=' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}

	var hunks []hunk
	start, prev := changed[0], changed[0]
	flush := func(from, to int) {
		lo, hi := from, to
		for taken := 0; lo > 0 && taken < diffContext && ops[lo-1].kind == '='; lo-- {
			taken++
		}
		for taken := 0; taken < diffContext && hi < len(ops) && ops[hi].kind == '='; hi++ {
			taken++
		}
		h := hunk{ops: ops[lo:hi]}
		for _, op := range h.ops {
			switch op.kind {
			case '=':
				if h.aCount == 0 && h.bCount == 0 {
					h.aStart, h.bStart = op.aLine, op.bLine
				}
				h.aCount++
				h.bCount++
			case '-':
				if h.aCount == 0 && h.bCount == 0 {
					h.aStart = op.aLine
				}
				h.aCount++
			case '+':
				if h.aCount == 0 && h.bCount == 0 {
					h.bStart = op.bLine
				}
				h.bCount++
			}
		}
		// 纯新增（aCount==0）时 git 用"插入点前一行"作为起点；空文件时为 0。
		if h.aCount == 0 {
			h.aStart = firstBLine(h.ops)
			if h.aStart > 0 {
				h.aStart--
			}
		}
		if h.bCount == 0 {
			h.bStart = firstALine(h.ops)
			if h.bStart > 0 {
				h.bStart--
			}
		}
		if h.aStart == 0 && aLen == 0 {
			h.aStart = 0
		}
		if h.bStart == 0 && bLen == 0 {
			h.bStart = 0
		}
		hunks = append(hunks, h)
	}

	for i := 1; i < len(changed); i++ {
		if changed[i]-prev <= 2*diffContext {
			prev = changed[i]
			continue
		}
		flush(start, prev+1)
		start, prev = changed[i], changed[i]
	}
	flush(start, prev+1)
	return hunks
}

func firstBLine(ops []lineOp) int {
	for _, op := range ops {
		if op.kind == '+' {
			return op.bLine
		}
	}
	return 0
}

func firstALine(ops []lineOp) int {
	for _, op := range ops {
		if op.kind == '-' {
			return op.aLine
		}
	}
	return 0
}
