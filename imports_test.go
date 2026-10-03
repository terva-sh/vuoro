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
	// No code an agent run reaches can write to a forge: the agent drivers
	// and the review runner depend on no forge adapter and not on the API
	// that holds the write routes. An agent drafts, and a person posts.
	runDeps, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./agent/...", "./review/...").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range strings.Fields(string(runDeps)) {
		if strings.HasPrefix(d, mod+"forge/") || d == mod+"app" || strings.HasPrefix(d, mod+"app/") {
			t.Errorf("the agent or review packages depend on %s", d)
		}
	}
	all, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(all)), "\n") {
		f := strings.Fields(line)
		for _, d := range f[1:] {
			if f[0] == mod+"app" && strings.HasPrefix(d, mod+"forge/") && d != mod+"forge/adapters" && d != mod+"forge/patch" {
				t.Errorf("app imports individual forge adapter %s", d)
			}
			if strings.HasPrefix(d, "github.com/terva-sh/git-ticket") && !strings.HasPrefix(f[0], mod+"tickets") {
				t.Errorf("%s imports %s; only tickets may", f[0], d)
			}
		}
	}
}
