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
