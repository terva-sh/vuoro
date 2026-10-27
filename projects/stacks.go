package projects

import (
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

// Stack is a change's place in a chain of changes that build on each other.
type Stack struct {
	// Root is the number of the change at the bottom of the chain.
	Root int `json:"root"`
	// Depth is how many changes are beneath this one: zero at the bottom.
	Depth int `json:"depth"`
	// On and Above are the changes directly beneath and above this one.
	On    []int `json:"on"`
	Above []int `json:"above"`
}

// stacks works out what each change in a project is stacked on. A change sits
// on another when its target branch is the other's source branch in this
// project, or when the forge's dependencies name the other. A blocker the
// person cannot see, or in another project, is counted but not linked. Links
// that form a cycle are dropped rather than followed.
func stacks(project string, changes []forge.Change) (map[int]below, map[int]*Stack) {
	open := map[int]bool{}
	bySource := map[string][]int{}
	for _, ch := range changes {
		open[ch.Ref.Number] = true
		// A fork's branch names are its own: a change from a fork is never
		// beneath another change, however its branch is named. An unknown
		// source project is treated the same way.
		if strings.EqualFold(ch.SourceProject, project) && ch.SourceBranch != "" {
			bySource[ch.SourceBranch] = append(bySource[ch.SourceBranch], ch.Ref.Number)
		}
	}
	on := map[int][]int{}
	hidden := map[int]int{}
	for _, ch := range changes {
		n := ch.Ref.Number
		for _, m := range bySource[ch.TargetBranch] {
			if m != n && !slices.Contains(on[n], m) {
				on[n] = append(on[n], m)
			}
		}
		for _, b := range ch.BlockedBy {
			switch {
			case strings.EqualFold(b.Project, project) && open[b.Number]:
				if b.Number != n && !slices.Contains(on[n], b.Number) {
					on[n] = append(on[n], b.Number)
				}
			case strings.EqualFold(b.Project, project):
				// Listed as blocking but no longer open here: merged or
				// closed since, so it no longer holds this one.
			default:
				hidden[n]++
			}
		}
	}
	// Drop the links of a cycle rather than follow them. Every cycle has a
	// back edge, which a depth-first search finds with both of its ends on
	// the search's path, so deleting the links between marked changes
	// breaks every cycle.
	cyclic := map[int]bool{}
	seen := map[int]int{} // 1 on the path, 2 done
	var path []int
	var visit func(n int)
	visit = func(n int) {
		seen[n] = 1
		path = append(path, n)
		for _, m := range on[n] {
			switch seen[m] {
			case 0:
				visit(m)
			case 1:
				for _, x := range path[slices.Index(path, m):] {
					cyclic[x] = true
				}
			}
		}
		path = path[:len(path)-1]
		seen[n] = 2
	}
	for _, ch := range changes {
		if seen[ch.Ref.Number] == 0 {
			visit(ch.Ref.Number)
		}
	}
	for n := range cyclic {
		on[n] = slices.DeleteFunc(on[n], func(m int) bool { return cyclic[m] })
	}

	belows := map[int]below{}
	out := map[int]*Stack{}
	for _, ch := range changes {
		n := ch.Ref.Number
		slices.Sort(on[n])
		belows[n] = below{On: on[n], Hidden: hidden[n]}
	}
	above := map[int][]int{}
	for n, ms := range on {
		for _, m := range ms {
			above[m] = append(above[m], n)
		}
	}
	var depth func(n int) (int, int)
	depth = func(n int) (int, int) {
		if len(on[n]) == 0 {
			return 0, n
		}
		d, root := depth(on[n][0])
		return d + 1, root
	}
	for _, ch := range changes {
		n := ch.Ref.Number
		if len(on[n]) == 0 && len(above[n]) == 0 {
			continue
		}
		d, root := depth(n)
		ups := slices.Clone(above[n])
		slices.Sort(ups)
		out[n] = &Stack{Root: root, Depth: d, On: slices.Clone(on[n]), Above: ups}
	}
	return belows, out
}
