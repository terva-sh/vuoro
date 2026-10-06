package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Limits bound a scan (docs/plan.md section 6.8).
type Limits struct {
	Depth     int           // directories below a root
	Checkouts int           // checkouts found, across every root
	Time      time.Duration // the whole scan
}

// DefaultLimits are the plan's: four directories deep, 500 checkouts, and
// ten seconds.
var DefaultLimits = Limits{Depth: 4, Checkouts: 500, Time: 10 * time.Second}

// skipped are directories a scan never enters: dependency trees, which hold
// other people's checkouts and a great many files.
var skipped = map[string]bool{"node_modules": true, "vendor": true, "target": true}

// ScanResult is what a scan found, and why it stopped early, if it did.
type ScanResult struct {
	Checkouts []string `json:"checkouts"`
	Stopped   string   `json:"stopped,omitempty"`
}

// Scan finds the checkouts under roots: folders holding a .git directory or
// file. It does not look inside a checkout, follow a symbolic link, or enter
// a hidden or dependency directory, and it stops at lim.
func Scan(ctx context.Context, roots []string, lim Limits) ScanResult {
	ctx, cancel := context.WithTimeout(ctx, lim.Time)
	defer cancel()
	res := ScanResult{Checkouts: []string{}}
	seen := map[string]bool{}
	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		if ctx.Err() != nil {
			res.Stopped = fmt.Sprintf("the scan stopped after %s", lim.Time)
			return false
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			if !seen[dir] {
				seen[dir] = true
				res.Checkouts = append(res.Checkouts, dir)
				if len(res.Checkouts) >= lim.Checkouts {
					res.Stopped = fmt.Sprintf("the scan stopped at %d checkouts", lim.Checkouts)
					return false
				}
			}
			return true
		}
		if depth >= lim.Depth {
			return true
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return true
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || e.Type()&os.ModeSymlink != 0 || strings.HasPrefix(name, ".") || skipped[name] {
				continue
			}
			if !walk(filepath.Join(dir, name), depth+1) {
				return false
			}
		}
		return true
	}
	for _, root := range roots {
		if !walk(root, 0) {
			break
		}
	}
	return res
}
