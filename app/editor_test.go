package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
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
		if got := callers[name]; len(got) != 1 || got[0] != "pushEditor" {
			t.Errorf("%s is called from %q, want only pushEditor", name, got)
		}
	}
}
