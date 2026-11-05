package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// TestNoErrorTextReachesAFiveHundredBody: an upstream's own words (an rpc
// error, an add-on body, a SQL string) never go into a 5xx response by way of
// err.Error(). A failure of Zitadel or an add-on goes through writeUpstream,
// which logs the raw text and sends a sentence. The one exemption is a plain
// 500 through jsonErrorResponse, which sanitises it itself.
func TestNoErrorTextReachesAFiveHundredBody(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok || (id.Name != "jsonErrorResponse" && id.Name != "jsonResponse") {
					return true
				}
				// jsonErrorResponse(w, status, code, msg); jsonResponse(w, status, body).
				if len(call.Args) < 3 {
					return true
				}
				sel, ok := call.Args[1].(*ast.SelectorExpr)
				if !ok || !fiveHundred(sel.Sel.Name) {
					return true
				}
				if id.Name == "jsonErrorResponse" && sel.Sel.Name == "StatusInternalServerError" {
					return true
				}
				checked++
				for _, body := range call.Args[2:] {
					ast.Inspect(body, func(m ast.Node) bool {
						c, ok := m.(*ast.CallExpr)
						if !ok {
							return true
						}
						if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Error" && len(c.Args) == 0 {
							t.Errorf("%s: err.Error() in a %s body; use writeUpstream", fset.Position(c.Pos()), sel.Sel.Name)
						}
						return true
					})
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("guard saw no 5xx call; it is parsing the wrong thing")
	}
}

func fiveHundred(name string) bool {
	switch name {
	case "StatusInternalServerError", "StatusNotImplemented", "StatusBadGateway",
		"StatusServiceUnavailable", "StatusGatewayTimeout":
		return true
	}
	return false
}
