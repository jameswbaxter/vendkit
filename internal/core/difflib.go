// Minimal unified diff for the human tier (cli spec) — LCS-based, unified
// format with 3 lines of context. Human-tier output is exempt from byte
// stability; this only has to be a correct, readable diff.

package core

import (
	"fmt"
	"strings"
)

func splitKeepEnds(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type diffOp struct {
	Kind   byte // ' ', '-', '+'
	Line   string
	AIndex int
	BIndex int
}

func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// LCS table (files here are small; O(n*m) is fine).
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], i, j})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i], i, j})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], i, j})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i], i, j})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j], i, j})
	}
	return ops
}

// UnifiedDiff renders a unified diff of two texts (context 3), with
// from/to labels. Empty string when identical.
func UnifiedDiff(oldText, newText, fromFile, toFile string) string {
	a, b := splitKeepEnds(oldText), splitKeepEnds(newText)
	ops := diffOps(a, b)
	changed := false
	for _, op := range ops {
		if op.Kind != ' ' {
			changed = true
			break
		}
	}
	if !changed {
		return ""
	}
	const ctx = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", fromFile, toFile)

	// Group ops into hunks separated by > 2*ctx unchanged lines.
	type hunk struct{ start, end int }
	var hunks []hunk
	last := -1
	for idx, op := range ops {
		if op.Kind != ' ' {
			if last >= 0 && idx-last <= 2*ctx {
				hunks[len(hunks)-1].end = idx
			} else {
				hunks = append(hunks, hunk{idx, idx})
			}
			last = idx
		}
	}
	for _, h := range hunks {
		start := h.start - ctx
		if start < 0 {
			start = 0
		}
		end := h.end + ctx
		if end > len(ops)-1 {
			end = len(ops) - 1
		}
		aStart, bStart := ops[start].AIndex, ops[start].BIndex
		aCount, bCount := 0, 0
		for _, op := range ops[start : end+1] {
			if op.Kind != '+' {
				aCount++
			}
			if op.Kind != '-' {
				bCount++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n",
			aStart+1, aCount, bStart+1, bCount)
		for _, op := range ops[start : end+1] {
			line := op.Line
			noNL := !strings.HasSuffix(line, "\n")
			out.WriteByte(op.Kind)
			out.WriteString(line)
			if noNL {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return out.String()
}
