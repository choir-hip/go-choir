package agentcore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The self-test's document-tool probe and seccomp leg run the binary they are
// given with arguments only the daemon's own main dispatches
// (doctool.HelperArg and ProbeArg, capsule.SeccompSelfTestArg). The
// dedicated desk-worker binary (RES-17), which deskWorkerBinary prefers when
// it sits beside the daemon, serves only desk-session; given it, the probe
// failed on deploy P (docs/problems/doctool-selftest-runs-desk-worker-2026-10-22.md).
// agentcore's tests cannot see this at run time: no desk-worker sits beside
// the test binary. So the call sites are pinned: each passes
// daemonSelfTestBinary(), never deskWorkerBinary().
func TestSelfTestLegsRunTheDaemonsOwnBinary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]bool{"doctool.SelfTest": false, "capsule.SeccompSelfTest": false}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			fn := pkg.Name + "." + sel.Sel.Name
			if _, ok := pinned[fn]; !ok {
				return true
			}
			pinned[fn] = true
			arg := ""
			if len(call.Args) > 1 {
				if inner, ok := call.Args[1].(*ast.CallExpr); ok {
					if id, ok := inner.Fun.(*ast.Ident); ok && len(inner.Args) == 0 {
						arg = id.Name
					}
				}
			}
			if arg != "daemonSelfTestBinary" {
				t.Errorf("%s: %s is given %s, want daemonSelfTestBinary() (the daemon's own executable, which dispatches its argument)",
					fset.Position(call.Pos()), fn, sourceOf(fset, src, call.Args))
			}
			return true
		})
	}
	for fn, seen := range pinned {
		if !seen {
			t.Errorf("no call to %s in agentcore; the pin no longer reads the self-test", fn)
		}
	}
}

// sourceOf is the binary argument's source text, for the failure message.
func sourceOf(fset *token.FileSet, src []byte, args []ast.Expr) string {
	if len(args) < 2 {
		return "no binary"
	}
	return string(src[fset.Position(args[1].Pos()).Offset:fset.Position(args[1].End()).Offset])
}
