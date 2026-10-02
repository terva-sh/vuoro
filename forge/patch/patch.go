// Package patch builds a unified diff from two versions of a file. An adapter
// uses it when a forge collapses or truncates a file's diff and sends the two
// versions instead.
package patch

import (
	"fmt"
	"strings"
)

// Context is the number of unchanged lines around each change.
const Context = 3

// Unified returns the hunks of a unified diff from a to b, without the ---
// and +++ header, in the form a forge's diff field uses. Identical inputs give
// "".
func Unified(a, b string) string {
	x, y := lines(a), lines(b)
	ops := diff(x, y)

	var out strings.Builder
	for i := 0; i < len(ops); {
		// Find the next change.
		for i < len(ops) && ops[i].kind == ' ' {
			i++
		}
		if i == len(ops) {
			break
		}
		start := max(i-Context, 0)
		// Extend the hunk while changes are within 2*Context of each other.
		end := i
		for {
			for end < len(ops) && ops[end].kind != ' ' {
				end++
			}
			next := end
			for next < len(ops) && ops[next].kind == ' ' {
				next++
			}
			if next == len(ops) || next-end > 2*Context {
				end = min(end+Context, len(ops))
				break
			}
			end = next
		}
		writeHunk(&out, ops[start:end])
		i = end
	}
	return out.String()
}

type op struct {
	kind   byte // ' ', '-', '+'
	text   string
	ai, bi int // line numbers, from 1, in a and b
}

func writeHunk(w *strings.Builder, ops []op) {
	var aStart, bStart, aLen, bLen int
	for _, o := range ops {
		if o.kind != '+' {
			if aLen == 0 {
				aStart = o.ai
			}
			aLen++
		}
		if o.kind != '-' {
			if bLen == 0 {
				bStart = o.bi
			}
			bLen++
		}
	}
	if aLen == 0 { // a pure insertion starts after the line before it
		aStart = ops[0].ai - 1
	}
	if bLen == 0 {
		bStart = ops[0].bi - 1
	}
	fmt.Fprintf(w, "@@ -%s +%s @@\n", span(aStart, aLen), span(bStart, bLen))
	for _, o := range ops {
		w.WriteByte(o.kind)
		w.WriteString(o.text)
		w.WriteByte('\n')
	}
}

func span(start, n int) string {
	if n == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, n)
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diff is Myers' O(ND) algorithm, returning every line of a and b in order as
// kept, removed, or added.
func diff(a, b []string) []op {
	n, m := len(a), len(b)
	maxD := n + m
	v := make([]int, 2*maxD+2)
	var trace [][]int
	off := maxD + 1
	for d := 0; d <= maxD; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || k != d && v[off+k-1] < v[off+k+1] {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				return backtrack(a, b, trace, off, d)
			}
		}
	}
	return nil
}

func backtrack(a, b []string, trace [][]int, off, d int) []op {
	x, y := len(a), len(b)
	var rev []op
	for ; d > 0; d-- {
		v := trace[d]
		k := x - y
		var pk int
		if k == -d || k != d && v[off+k-1] < v[off+k+1] {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := v[off+pk]
		py := px - pk
		for x > px && y > py {
			rev = append(rev, op{' ', a[x-1], x, y})
			x--
			y--
		}
		if x == px {
			rev = append(rev, op{'+', b[y-1], x + 1, y})
			y--
		} else {
			rev = append(rev, op{'-', a[x-1], x, y + 1})
			x--
		}
	}
	for x > 0 && y > 0 {
		rev = append(rev, op{' ', a[x-1], x, y})
		x--
		y--
	}
	out := make([]op, len(rev))
	for i, o := range rev {
		out[len(rev)-1-i] = o
	}
	return out
}
