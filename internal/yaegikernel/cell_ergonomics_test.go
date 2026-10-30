package yaegikernel

import (
	"context"
	"encoding/json"
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
	if out := cell(`strings.Repeat("z", 100000)`); len(out.Stdout) > maxShownCellValue+100 {
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

// K9-K11 (docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md),
// from the first bound engineering run. Failure modes pinned: a one-line
// parenthesized import list followed by statements fails to compile; a
// bound helper cannot be redefined with a new func type; a compile error
// does not show the line it points at.
func TestOneLineImportListsTypedRedefinitionAndCompileErrorLines(t *testing.T) {
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
	got := cell("import (\"encoding/json\"; \"fmt\"; \"strconv\")\ndump := func(label string, v any) {\n\tb, _ := json.Marshal(v)\n\tfmt.Println(label, string(b), strconv.Itoa(1))\n}\ndump(\"x\", 2)")
	if got.Stdout != "x 2 1\n" {
		t.Fatalf("one-line import list cell stdout = %q", got.Stdout)
	}
	cell("sh := func(cmd string, args ...string) string { return cmd }\nfmt.Println(sh(\"a\"))")
	got = cell("sh := func(cmd string, args []string, env []string) string { return cmd + env[0] }\nfmt.Println(sh(\"a\", nil, []string{\"b\"}))")
	if got.Stdout != "ab\n" {
		t.Fatalf("redefined helper stdout = %q", got.Stdout)
	}
	cell("y := 3")
	if out := cell("y := y * 2\nfmt.Println(y)"); out.Stdout != "6\n" {
		t.Fatalf("K8 still holds: stdout = %q", out.Stdout)
	}

	bad := "r := []string{\"a\", \"b\"]\nfmt.Println(r)"
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: bad}, nil, &hooks)
	if err != nil || out.Error == "" {
		t.Fatalf("typo cell: err=%v out=%+v", err, out)
	}
	if !strings.Contains(out.Error, `r := []string{"a", "b"]`) {
		t.Fatalf("compile error does not show the offending line:\n%s", out.Error)
	}
}

// Research-only demo 2026-10-10 (cell-shows-only-last-value-research-reads-
// results-as-nil-2026-10-10.md): research wrote `r, err := choir.WebSearch(...);
// r; err`, saw only `=> <nil>` (err), and reported every source dead.
// Failure modes pinned: a listed name before the last statement is not
// shown; JSON bytes show as a list of numbers; a name listed twice shows
// twice; a listed func value prints an address; the final value stops
// showing.
func TestCellShowsEveryListedNameAndJSONBytesAsText(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: "r := json.RawMessage(`{\"results\":[{\"title\":\"x\"}]}`); var e error; f := strings.ToUpper; r; f; r; e"}, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out.Error)
	}
	want := "=> r: {\"results\":[{\"title\":\"x\"}]}\n=> <nil>\n"
	if out.Stdout != want {
		t.Fatalf("stdout = %q, want %q", out.Stdout, want)
	}
	out, err = serveCell(sess, SessionFrame{ID: "cell2", Source: "r"}, nil, &hooks)
	if err != nil || !strings.Contains(out.Stdout, "=> {\"results\"") {
		t.Fatalf("final JSON bytes not shown as text: %v %q", err, out.Stdout)
	}
}

// problems/research-parent-cannot-read-its-children-2026-10-11.md. Failure
// modes pinned: println, fmt.Println or %v of a world verb's JSON prints
// byte values; json.Marshal of a struct holding it base64-encodes it;
// string(r), json.Unmarshal(r, ...) or passing r to a JSON parameter stop
// compiling; a listed value of a few thousand characters (a child's
// findings) is cut off well inside the cell's output limit.
func TestWorldVerbJSONPrintsAsTextAndListedValuesShowInFull(t *testing.T) {
	root := t.TempDir()
	secret := make([]byte, 32)
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: root,
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"results":[{"title":"x"}]}`), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, SessionRoleResearchChild, "")
	if err != nil {
		t.Fatal(err)
	}
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
	const want = `{"results":[{"title":"x"}]}`
	out := cell("r, err := choir.SearchWireCorpus(\"q\", 3)\nprintln(r)\nfmt.Println(r, err)\nfmt.Printf(\"%v|%s\\n\", r, r)\nb, _ := json.Marshal(map[string]any{\"r\": r})\nfmt.Println(string(b))\nvar v map[string]any\nfmt.Println(json.Unmarshal(r, &v), len(string(r)) > 0)")
	wantOut := want + "\n" + want + " <nil>\n" + want + "|" + want + "\n" + `{"r":` + want + "}\n<nil> true\n"
	if out.Stdout != wantOut {
		t.Fatalf("stdout = %q\nwant     %q", out.Stdout, wantOut)
	}
	out = cell("long := strings.Repeat(\"f\", 20000)\nlong\nlen(long)")
	if !strings.Contains(out.Stdout, strings.Repeat("f", 20000)) {
		t.Fatalf("a 20,000-character listed value was cut: %d bytes shown", len(out.Stdout))
	}
}

// Cluster B (docs/archive/reports/cross-model-cell-errors-2026-10-13.md): the
// bound-define rewrite turned every `x := e` on a bound x into `x = e`, so
// a new type failed at compile time ("cannot use type") or at runtime
// through reflect.Set, which killed the worker. Failure modes pinned: a
// bound name redefined with a new type fails; the redefinition is not
// what later cells see; the self-referencing rebind (`y := y * 2`) reads
// a fresh zero y again.
func TestBoundNameRedefinedWithNewTypeRuns(t *testing.T) {
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
	cell("v := []int{1, 2}\nn := 3")
	if out := cell("v := \"text\"\nfmt.Println(v + \"!\")"); out.Stdout != "text!\n" {
		t.Fatalf("redefined with a new type: stdout = %q", out.Stdout)
	}
	cell("pair := func() (map[string]int, string) { return map[string]int{\"a\": 1}, \"four\" }")
	if out := cell("v, n := pair()\nfmt.Println(v[\"a\"], n)"); out.Stdout != "1 four\n" {
		t.Fatalf("two names redefined: stdout = %q", out.Stdout)
	}
	if out := cell("fmt.Println(len(v), n)"); out.Stdout != "1 four\n" {
		t.Fatalf("later cell sees = %q, want the redefinition", out.Stdout)
	}
	cell("k := 4")
	if out := cell("k := k * 2\nfmt.Println(k)"); out.Stdout != "8\n" {
		t.Fatalf("self-referencing rebind = %q, want 8", out.Stdout)
	}
}

// Clusters E, F and G (docs/archive/reports/cross-model-cell-errors-2026-10-13.md).
// Failure modes pinned: an import of the choir package by a module-style
// path ("go-choir/choir", "github.com/.../choir") is refused instead of
// naming the predeclared package; a path that only ends in "choir" as part
// of a longer element is treated as choir; min and max are undefined; a
// Search has no Total.
func TestChoirImportPathsMinMaxAndSearchTotal(t *testing.T) {
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
	for _, src := range []string{
		"import \"go-choir/choir\"\nfmt.Println(choir.Help != nil)",
		"import (\n\t\"fmt\"\n\t\"github.com/yusefmosiah/go-choir/choir\"\n)\nfmt.Println(choir.Help != nil)",
		"package main\n\nimport \"choir.com/choir\"\n\nfunc main() { fmt.Println(choir.Help != nil) }",
	} {
		if out := cell(src); !strings.Contains(out.Stdout, "true") {
			t.Fatalf("module-path choir import: stdout %q\n%s", out.Stdout, src)
		}
	}
	if out, err := serveCell(sess, SessionFrame{ID: "cell", Source: "import \"example.com/notchoir\"\nx := 1"}, nil, &hooks); err == nil && out.Error == "" {
		t.Fatalf("a path ending in notchoir was treated as choir")
	}
	if out := cell("s := \"abcdef\"\nfmt.Println(s[:min(3, len(s))], max(2, 7, 4), min(5))"); out.Stdout != "abc 7 5\n" {
		t.Fatalf("min/max stdout = %q", out.Stdout)
	}
	got := copySearch(&Search{ID: "s1", Providers: []string{"a", "b", "c"}, Answered: 2})
	if got.Total != 3 {
		t.Fatalf("Search.Total = %d, want 3", got.Total)
	}
}

// Cluster D (docs/archive/reports/cross-model-cell-errors-2026-10-13.md): models
// read a record's or update's id as .ID (15 of 16 cells in Texture's first
// survey cell); the fields are RecordID and UpdateID, and "undefined
// method: ID" named neither. Failure modes pinned: r.ID does not compile
// or is empty; u.ID likewise; an unknown field on a choir result names no
// real field.
func TestRecordAndUpdateIDAndFieldHint(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	// Updates is a Texture and management observation (engineering reads
	// records since C1e B1).
	scope.desk = "management"
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	frame := SessionFrame{ID: "cell", Source: "for _, r := range choir.Records() { fmt.Println(r.ID, r.RecordID) }\nfor _, u := range choir.Updates() { fmt.Println(u.ID) }",
		Records: []InboxRecord{{Seq: 1, RecordID: "rec-1", Kind: "cast"}},
		Updates: []PendingUpdate{{UpdateID: "upd-1"}}}
	out, err := serveCell(sess, frame, nil, &hooks)
	if err != nil || out.Error != "" || out.Stdout != "rec-1 rec-1\nupd-1\n" {
		t.Fatalf("record and update ids: %v %q stdout %q", err, out.Error, out.Stdout)
	}
	hint := sess.ErrorHint("for _, r := range choir.Records() { fmt.Println(r.Titel) }", "1:51: undefined method: Titel")
	if !strings.Contains(hint, ".Title") {
		t.Fatalf("unknown-field hint names no real field: %q", hint)
	}
	// A field several result types have (ID) on a value of another type:
	// name the types and how to see the value's own (paper-3, s.ID on a
	// source); a case slip (Id, ConversationId) names the exact spelling.
	hint = sess.ErrorHint("for _, s := range srcs { fmt.Println(s.ID) }", "1:40: undefined method: ID")
	if !strings.Contains(hint, "InboxRecord") || !strings.Contains(hint, "%T") {
		t.Fatalf("shared-field hint = %q, want the types with ID and a %%T probe", hint)
	}
	hint = sess.ErrorHint("for _, r := range choir.Records() { fmt.Println(r.ConversationId) }", "1:51: undefined method: ConversationId")
	if !strings.Contains(hint, ".ConversationID") || !strings.Contains(hint, "case") {
		t.Fatalf("case-slip hint = %q, want .ConversationID", hint)
	}
}
