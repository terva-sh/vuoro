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
				if sel, ok := n.(*ast.SelectorExpr); ok {
					callers[sel.Sel.Name] = append(callers[sel.Sel.Name], fn.Name.Name)
				}
				if id, ok := n.(*ast.Ident); ok && id.Name == "writeFix" && fn.Name.Name != "writeFix" {
					callers["writeFix"] = append(callers["writeFix"], fn.Name.Name)
				}
				return true
			})
		}
	}
	// Suggestions go back the same way (D59): only the confirmed suggest
	// route delivers them, and only its writer attaches a patch. An update
	// is pushed, and its draft change opened, only by its own confirmed
	// route (D66).
	for name, want := range map[string][]string{"PushEditor": {"pushEditor"}, "OpenStacked": {"pushEditor", "pushUpdate"},
		"PushUpdate": {"pushUpdate"}, "DeliverSuggestions": {"suggestEditor"}, "writeFix": {"suggestEditor"}, "Upload": {"writeFix"}} {
		got := slices.Compact(slices.Sorted(slices.Values(callers[name])))
		if !slices.Equal(got, want) {
			t.Errorf("%s is called from %q, want only %q", name, got, want)
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
