package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Only the person's confirmed push route pushes the editor's work or opens a
// merge request (D39): no other function in the server calls either.
func TestOnlyTheConfirmedRoutePushesOrOpensAMergeRequest(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	callers := map[string][]string{}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "PushEditor" || sel.Sel.Name == "OpenStacked") {
					callers[sel.Sel.Name] = append(callers[sel.Sel.Name], fn.Name.Name)
				}
				return true
			})
		}
	}
	for _, name := range []string{"PushEditor", "OpenStacked"} {
		if got := slices.Compact(callers[name]); len(got) != 1 || got[0] != "pushEditor" {
			t.Errorf("%s is called from %q, want only pushEditor", name, got)
		}
	}
}

// The push warns about the CI configuration of the change's own forge.
func TestCIFilesAreTheForgesOwn(t *testing.T) {
	for _, c := range []struct {
		kind, path string
		want       bool
	}{
		{"gitlab", ".gitlab-ci.yml", true},
		{"gitlab", "ci/.gitlab-ci.yml", true},
		{"gitlab", ".gitlab/ci/build.yml", true},
		{"gitlab", ".forgejo/workflows/ci.yml", false},
		{"forgejo", ".forgejo/workflows/ci.yml", true},
		{"forgejo", ".gitea/workflows/ci.yml", true},
		{"forgejo", ".github/workflows/ci.yml", true},
		{"forgejo", ".gitlab-ci.yml", false},
		{"forgejo", "main.go", false},
		{"github", ".github/workflows/ci.yml", true},
		{"github", ".github/actions/setup/action.yml", true},
		{"github", ".github/CODEOWNERS", false},
		{"github", ".forgejo/workflows/ci.yml", false},
		{"github", ".gitlab-ci.yml", false},
	} {
		if got := ciFile(c.kind, c.path); got != c.want {
			t.Errorf("ciFile(%s, %s) = %v", c.kind, c.path, got)
		}
	}
}
