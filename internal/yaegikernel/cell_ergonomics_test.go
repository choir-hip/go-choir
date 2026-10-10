package yaegikernel

import (
	"strings"
	"testing"
)

// Demo traces 2026-10-10: desks burned cells on "undefined: fmt",
// "undefined: json" and "json/_.go redeclared", and on probing values the
// cell never showed (a bare `updates` or `choir.Help("Report")` returned an
// empty stdout, so models resorted to panic(string(b))). Failure modes
// pinned: fmt, strings and encoding/json need an import; a model-authored
// import of them fails as a redeclaration; a final bare expression's value is
// not shown; a cell ending in println or an assignment gains spurious output;
// a huge value floods the result.
func TestCellPredeclaresCommonPackagesAndShowsTheFinalValue(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	cell := func(source string) SessionResult {
		t.Helper()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
		if err != nil || out.Error != "" {
			t.Fatalf("cell %q failed: %v %s", source, err, out.Error)
		}
		return out
	}
	if out := cell("b, _ := json.Marshal(map[string]int{\"a\": 1})\nfmt.Println(strings.ToUpper(string(b)))"); out.Stdout != "{\"A\":1}\n" {
		t.Fatalf("predeclared packages stdout = %q", out.Stdout)
	}
	cell("import \"fmt\"\nimport \"encoding/json\"\nfmt.Print(\"\")")
	cell("import (\n\t\"encoding/json\"\n\t\"strings\"\n)\n_ = json.Valid\n_ = strings.ToUpper")
	if out := cell("x := 41\nx + 1"); !strings.Contains(out.Stdout, "42") {
		t.Fatalf("final expression value not shown: %q", out.Stdout)
	}
	if out := cell(`choir.Help("Report")`); !strings.Contains(out.Stdout, "Report") {
		t.Fatalf("final string value not shown: %q", out.Stdout)
	}
	if out := cell(`println("only this")`); strings.Contains(out.Stdout, "=>") {
		t.Fatalf("println cell gained a value line: %q", out.Stdout)
	}
	if out := cell(`y := "assigned"`); out.Stdout != "" {
		t.Fatalf("assignment cell gained output: %q", out.Stdout)
	}
	if out := cell(`strings.Repeat("z", 100000)`); len(out.Stdout) > 9000 {
		t.Fatalf("huge value not truncated: %d bytes", len(out.Stdout))
	}
}

// K4 (docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md):
// management's cells began with `import "fmt"` (already installed) and
// re-declared objective and spec. The import+statement path hoisted each new
// `x := e` to a package-level var, so `ref, err := choir.Cast(..., objective,
// spec)` initialised before the body reassigned objective and spec, and
// yaegi refused it as a "variable definition loop", poisoning the worker.
// Failure modes pinned: the second cell errors or poisons; statements run
// out of order (the result reflects the first cell's values).
func TestKnownImportCellsRedeclareInOrder(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	cell := func(source string) SessionResult {
		t.Helper()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
		if err != nil || out.Error != "" || out.Reuse == ReuseUnsafeToReuse {
			t.Fatalf("cell failed: %v %q reuse=%q\n%s", err, out.Error, out.Reuse, source)
		}
		return out
	}
	cell("import \"fmt\"\nobjective := \"first\"\nspec := map[string]interface{}{\"objective\": objective}\nout, err := json.Marshal(spec)\nfmt.Println(string(out), err)")
	got := cell("import \"fmt\"\nobjective := \"second\"\nspec := map[string]interface{}{\"objective\": objective}\nref, err := json.Marshal(spec)\nfmt.Println(string(ref), err)")
	if got.Stdout != "{\"objective\":\"second\"} <nil>\n" {
		t.Fatalf("second cell stdout = %q, want the second cell's values in order", got.Stdout)
	}
	if out := cell("fmt.Println(objective, len(ref) > 0)"); out.Stdout != "second true\n" {
		t.Fatalf("bindings after the cell = %q", out.Stdout)
	}
}

// K5/K7 (docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md):
// yaegi's top-level statement mode reported "undefined: b" for a valid
// `b, err := json.Marshal(x)` inside a range body, and a cell ending in
// `func() { ... }()` showed a pointer as its value. Failure modes pinned:
// a definition inside a top-level compound statement fails to compile; a
// top-level loop no longer sees or updates the cell's earlier bindings; a
// void closure call gains a value line; `y := y * 2` on a bound name reads a
// fresh zero y.
func TestTopLevelCompoundStatementsCompileAndKeepBindings(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	cell := func(source string) SessionResult {
		t.Helper()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
		if err != nil || out.Error != "" {
			t.Fatalf("cell failed: %v %q\n%s", err, out.Error, source)
		}
		return out
	}
	out := cell("total := 0\nfor i, x := range []int{1, 2} {\n\tb, err := json.Marshal(x)\n\ttotal += len(b)\n\tfmt.Println(i, string(b), err)\n}\nif total > 1 {\n\tfmt.Println(\"total\", total)\n}")
	if out.Stdout != "0 1 <nil>\n1 2 <nil>\ntotal 2\n" {
		t.Fatalf("compound statements stdout = %q", out.Stdout)
	}
	if out := cell("func() {\n\tfmt.Print(\"\")\n}()"); out.Stdout != "" {
		t.Fatalf("void closure call gained output: %q", out.Stdout)
	}
	if out := cell("total := total * 10\ntotal"); !strings.Contains(out.Stdout, "20") {
		t.Fatalf("self-referencing rebind = %q, want 20", out.Stdout)
	}
}
