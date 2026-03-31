package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Diff returns a tool for computing text diffs.
func Diff() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "text_diff", Desc: "Compute a unified diff between two texts",
			Params: agnogo.Params{
				"text_a": {Type: "string", Desc: "First text", Required: true},
				"text_b": {Type: "string", Desc: "Second text", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				textA := args["text_a"]
				textB := args["text_b"]
				linesA := strings.Split(textA, "\n")
				linesB := strings.Split(textB, "\n")
				diff := computeDiff(linesA, linesB)
				return diff, nil
			},
		},
	}
}

// computeDiff produces a unified diff using an LCS-based algorithm.
func computeDiff(a, b []string) string {
	// Compute LCS table
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff operations
	type diffOp struct {
		op   byte // ' ', '-', '+'
		line string
	}
	var ops []diffOp
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append(ops, diffOp{' ', a[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, diffOp{'+', b[j-1]})
			j--
		} else if i > 0 {
			ops = append(ops, diffOp{'-', a[i-1]})
			i--
		}
	}
	// Reverse
	for left, right := 0, len(ops)-1; left < right; left, right = left+1, right-1 {
		ops[left], ops[right] = ops[right], ops[left]
	}

	// Format as unified diff with context
	var sb strings.Builder
	sb.WriteString("--- a\n+++ b\n")

	const contextLines = 3
	// Find hunks
	for idx := 0; idx < len(ops); idx++ {
		if ops[idx].op == ' ' {
			continue
		}
		// Found a change, build a hunk
		start := idx - contextLines
		if start < 0 {
			start = 0
		}
		end := idx
		// Extend to cover all nearby changes
		for end < len(ops) {
			if ops[end].op != ' ' {
				end++
				continue
			}
			// Look ahead for more changes within context
			lookahead := end + 2*contextLines + 1
			if lookahead > len(ops) {
				lookahead = len(ops)
			}
			found := false
			for k := end; k < lookahead; k++ {
				if ops[k].op != ' ' {
					end = k + 1
					found = true
					break
				}
			}
			if !found {
				end += contextLines
				if end > len(ops) {
					end = len(ops)
				}
				break
			}
		}

		// Count lines for header
		aStart, bStart := 1, 1
		for k := 0; k < start; k++ {
			if ops[k].op != '+' {
				aStart++
			}
			if ops[k].op != '-' {
				bStart++
			}
		}
		aCount, bCount := 0, 0
		for k := start; k < end; k++ {
			if ops[k].op != '+' {
				aCount++
			}
			if ops[k].op != '-' {
				bCount++
			}
		}
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount))
		for k := start; k < end; k++ {
			sb.WriteByte(ops[k].op)
			sb.WriteString(ops[k].line)
			sb.WriteByte('\n')
		}
		idx = end - 1
	}
	return sb.String()
}
