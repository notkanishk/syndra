package repoguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A closure must not re-declare the variable its caller is going to read.
//
// `MoveHolders` did, and the shape is worth stating exactly, because it is
// legal Go and both the compiler and `go vet` are silent about it:
//
//	func MoveHolders(...) (BulkPlan, error) {
//	    var plan BulkPlan                              // what gets RETURNED
//	    if err := withLockedAccess(ctx, func(ctx context.Context) error {
//	        var err error
//	        plan, err := RehearseMoveHolders(ctx, req)  // a SECOND plan
//	        ...
//	    }); err != nil { ... }
//	    return plan, nil                               // still the zero value
//	}
//
// The move itself worked. The caller got `op: ""`, `outcomes: null` and every
// count nought, so the operator was told "Applied to 0 people" about somebody
// who had just been repinned. `PublishBundleVersion` has the same shape twenty
// lines away and uses `=`, which is what made the difference invisible on
// review: the two read almost identically.
//
// It survived because nobody could reach the screen that renders the result —
// the shared dialog disables Apply without a plan_id and that endpoint issued
// none. So the guard exists for the reason the Zitadel-writer guard does: not
// to fix the one instance, but to stop a second arriving the same way.
//
// `err` is exempt. Shadowing it inside an inner scope is ordinary Go and the
// inner one is checked where it is declared; flagging it would bury this in
// forty findings nobody reads. The dangerous case is a NON-error value whose
// assignment is silently discarded.

// closureShadowExempt lists deliberate shadows, each with its reason. Empty is
// the intended state; an entry here is a claim that the outer variable is not
// read afterwards.
var closureShadowExempt = map[string]string{}

func TestAClosureDoesNotShadowWhatItsCallerReads(t *testing.T) {
	root := repoRoot(t)

	var findings []string
	err := filepath.Walk(filepath.Join(root, "backend"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".gomodcache", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// A file this guard cannot parse is a problem for the compiler to
			// report, not for the guard to swallow silently.
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for _, f := range shadowsInFunc(fset, rel, fn) {
				findings = append(findings, f)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	var unexplained []string
	for _, f := range findings {
		location := strings.SplitN(f, " ", 2)[0]
		if _, ok := closureShadowExempt[location]; !ok {
			unexplained = append(unexplained, f)
		}
	}
	sort.Strings(unexplained)

	if len(unexplained) > 0 {
		t.Fatalf("a closure re-declares a variable its enclosing function also holds, so the "+
			"assignment never reaches the caller — use `=` rather than `:=`, or rename the inner "+
			"variable if it is genuinely a different thing:\n  %s",
			strings.Join(unexplained, "\n  "))
	}
}

// shadowsInFunc reports closure bodies that re-declare a name the enclosing
// function declared.
//
// Only names the enclosing function declares with `var` or a named result are
// considered. A name introduced by the function's own `:=` is deliberately out
// of scope: those are usually loop and branch locals whose lifetime does not
// span a closure, and including them produced noise without producing findings.
func shadowsInFunc(fset *token.FileSet, rel string, fn *ast.FuncDecl) []string {
	outer := map[string]bool{}

	// Named results. A shadowed named result is the worst version of this bug:
	// the function returns the zero value with nothing on screen to say so.
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			for _, name := range field.Names {
				if name.Name != "_" && name.Name != "err" {
					outer[name.Name] = true
				}
			}
		}
	}
	// Function-scope `var` declarations, at the top level of the body only.
	for _, stmt := range fn.Body.List {
		declStmt, ok := stmt.(*ast.DeclStmt)
		if !ok {
			continue
		}
		genDecl, ok := declStmt.Decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if name.Name != "_" && name.Name != "err" {
					outer[name.Name] = true
				}
			}
		}
	}
	if len(outer) == 0 {
		return nil
	}

	var findings []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		// Names the closure declares in its own signature are its own; a
		// parameter called `plan` is not shadowing anything by accident.
		shielded := map[string]bool{}
		for _, field := range lit.Type.Params.List {
			for _, name := range field.Names {
				shielded[name.Name] = true
			}
		}

		ast.Inspect(lit.Body, func(inner ast.Node) bool {
			assign, ok := inner.(*ast.AssignStmt)
			if !ok || assign.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range assign.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || !outer[ident.Name] || shielded[ident.Name] {
					continue
				}
				pos := fset.Position(ident.Pos())
				findings = append(findings, fmtShadow(rel, pos.Line, fn.Name.Name, ident.Name))
			}
			return true
		})
		return true
	})
	return findings
}

func fmtShadow(rel string, line int, fn, name string) string {
	return rel + ":" + itoa(line) + " — " + fn + " holds " + name +
		", and a closure inside it declares another with `:=`"
}
