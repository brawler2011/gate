package access_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Commands must decide inside the transaction that writes, so use cases open transactions only through access.Within*.
func TestCommandsOpenTransactionsThroughAccess(t *testing.T) {
	files, err := filepath.Glob("../*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	// identity and judging run on behalf of the account or the worker; postgres implements Within.
	skip := map[string]bool{"access": true, "identity": true, "judging": true, "postgres": true}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || skip[filepath.Base(filepath.Dir(path))] {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Within" {
				t.Errorf("%s: open command transactions with access.Within*", fset.Position(sel.Pos()))
			}
			return true
		})
	}
}
