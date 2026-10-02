package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestBoundaries holds the package boundaries in AGENTS.md: forge and turn
// import neither agent nor tickets, nor the review runner that drives agents,
// so the inbox builds without any agent tool. Only tickets imports git-ticket.
func TestBoundaries(t *testing.T) {
	const mod = "github.com/terva-sh/vuoro/"
	deps, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./forge/...").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range strings.Fields(string(deps)) {
		for _, banned := range []string{mod + "agent", mod + "review", mod + "tickets", "github.com/terva-sh/git-ticket"} {
			if d == banned || strings.HasPrefix(d, banned+"/") {
				t.Errorf("forge depends on %s", d)
			}
		}
	}
	all, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(all)), "\n") {
		f := strings.Fields(line)
		for _, d := range f[1:] {
			if strings.HasPrefix(d, "github.com/terva-sh/git-ticket") && !strings.HasPrefix(f[0], mod+"tickets") {
				t.Errorf("%s imports %s; only tickets may", f[0], d)
			}
		}
	}
}
