package agentcore

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The job park through its wired path (review gap on aeb469034/acd0ac5ed):
// TestCellJobPark and the engineeringJobPark tests call the park's functions
// directly, so reverting the two lines that connect them left every test
// green:
//
//  1. tools_capsule.go: capsule_go_eval's `result.JobPark = cellJobPark(result)`;
//  2. runtime.go: executeWithToolLoop's `WithToolBatchWait(... engineeringJobPark ...)`
//     in the assigned-engineering overlay only.
//
// Neither site can be driven through a real call in a unit test. The tool's
// executor is the concrete *capsule.Executor (a stub off linux, a real
// capsule and broker on it) and its registry is rebuilt by
// buildAssignedEngineeringRegistry; executeWithToolLoop keeps its tool-loop
// options in locals. So these tests pin the wiring in the source of the two
// sites, and pin the contract between them (the tool's real JSON encoding is
// what the park reads) by running that encoding through the park.

func parseAgentcoreFile(t *testing.T, name string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return file
}

func funcDecl(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}

// selectorText renders a pure selector chain (`a.b.c`); anything else is "".
func selectorText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if base := selectorText(x.X); base != "" {
			return base + "." + x.Sel.Name
		}
	}
	return ""
}

// walkWithAncestors calls visit for every node under root with the nodes
// enclosing it, outermost first.
func walkWithAncestors(root ast.Node, visit func(n ast.Node, ancestors []ast.Node)) {
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		visit(n, stack)
		stack = append(stack, n)
		return true
	})
}

// enclosedByIf reports whether the ancestors include an `if` whose condition
// is exactly cond and whose body (not its else) holds the node.
func enclosedByIf(ancestors []ast.Node, cond string) bool {
	for i, a := range ancestors {
		ifStmt, ok := a.(*ast.IfStmt)
		if !ok || selectorText(ifStmt.Cond) != cond || i+1 >= len(ancestors) {
			continue
		}
		if ancestors[i+1] == ast.Node(ifStmt.Body) {
			return true
		}
	}
	return false
}

// The capsule_go_eval tool sets job_park from the cell's result, only for a
// reduction-bound (RLM) call, before it returns its JSON; and the JSON it
// returns is what the park consumes.
func TestCapsuleGoEvalToolSetsJobParkOnItsResult(t *testing.T) {
	tool := funcDecl(t, parseAgentcoreFile(t, "tools_capsule.go"), "newCapsuleGoEvalTool")
	var assignPos, returnPos token.Pos
	assigns := 0
	walkWithAncestors(tool.Body, func(n ast.Node, ancestors []ast.Node) {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) != 1 || len(x.Rhs) != 1 || selectorText(x.Lhs[0]) != "result.JobPark" {
				return
			}
			call, ok := x.Rhs[0].(*ast.CallExpr)
			if !ok || selectorText(call.Fun) != "cellJobPark" || len(call.Args) != 1 || selectorText(call.Args[0]) != "result" {
				t.Errorf("result.JobPark is not set from cellJobPark(result)")
				return
			}
			if !enclosedByIf(ancestors, "reduction.active") {
				t.Errorf("result.JobPark is set outside `if reduction.active`")
			}
			assigns++
			assignPos = x.Pos()
		case *ast.CallExpr:
			if selectorText(x.Fun) == "toolregistry.ResultJSON" {
				returnPos = x.Pos()
			}
		}
	})
	if assigns != 1 {
		t.Fatalf("capsule_go_eval sets result.JobPark %d times, want once: the cell's poll no longer parks the turn", assigns)
	}
	if !returnPos.IsValid() || assignPos > returnPos {
		t.Fatalf("result.JobPark is not set before the result is encoded")
	}

	// The encoding the tool returns is what the park reads.
	out, err := toolregistry.ResultJSON(capsule.GoEvalResult{JobsAwaited: []string{"job-build"}, JobPark: cellJobPark(capsule.GoEvalResult{JobsAwaited: []string{"job-build"}})})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"job_park":"job-build"`) {
		t.Fatalf("the tool's JSON carries no job_park: %s", out)
	}
	rt, _ := testRuntime(t)
	table := runningBuild(time.Now().Add(20 * time.Minute))
	table.end(capsulejob.StateDone, 0) // ended: the park resumes at once, with no wait
	rt.jobTable = table
	calls, results := evalCall(out, false)
	got := awaitPark(t, parkAsync(rt, context.Background(), &parkEvents{}, calls, results), 2*time.Second)
	if got.err != nil || !got.ok || !got.res.Waited {
		t.Fatalf("the park did not take the tool's own JSON: %+v", got)
	}
}

// The engineering overlay installs the batch wait, calling engineeringJobPark,
// and nothing else in the runtime does: no other desk holds its next turn on
// a job.
func TestOnlyTheEngineeringOverlayInstallsTheJobParkBatchWait(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	installs := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file := parseAgentcoreFile(t, name)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			walkWithAncestors(fn.Body, func(n ast.Node, ancestors []ast.Node) {
				call, ok := n.(*ast.CallExpr)
				if !ok || selectorText(call.Fun) != "toolregistry.WithToolBatchWait" {
					return
				}
				installs++
				if name != "runtime.go" || fn.Name.Name != "executeWithToolLoop" {
					t.Errorf("WithToolBatchWait installed in %s %s, want only executeWithToolLoop in runtime.go", name, fn.Name.Name)
					return
				}
				if !enclosedByIf(ancestors, "assignedEngineeringOverlay") {
					t.Errorf("WithToolBatchWait is installed outside `if assignedEngineeringOverlay`")
				}
				parks := false
				ast.Inspect(call, func(inner ast.Node) bool {
					if c, ok := inner.(*ast.CallExpr); ok && selectorText(c.Fun) == "rt.engineeringJobPark" {
						parks = true
					}
					return true
				})
				if !parks {
					t.Errorf("the installed batch wait does not call rt.engineeringJobPark")
				}
			})
		}
	}
	if installs != 1 {
		t.Fatalf("WithToolBatchWait installed %d times, want once (the engineering overlay): the next model turn is no longer held on a polled job", installs)
	}
}
