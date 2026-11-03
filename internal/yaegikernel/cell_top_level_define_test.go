package yaegikernel

import (
	"strings"
	"testing"
)

// Live deepseek calendar r1 (2026-11-02): management's cell
// `recs := choir.Records()` then `r := recs[0]` was refused "constant
// definition loop" though the prompt and the hint name the shape. yaegi
// cannot compile a top-level `x := v[i]`, `x := v.F.G` or `a, ok := ...`
// on such values, and `var r = recs[0]` fails the same way. Recompiling a
// rewritten cell in the same session after the refusal is not safe: yaegi
// then re-runs the rewritten statements on every later cell (found while
// building this). So the refusal hands back the cell rewritten with the
// statements from the first such definition inside func() { ... }(), for
// the model to send as is.
//
// Ways this could fail, written first: the hint keeps naming the shape
// without the corrected cell; it suggests `var r = recs[0]`, which also
// fails; the rewritten cell does not compile and run in a fresh session; a
// final bare expression inside the wrapper fails "evaluated but not used";
// the hint does not say the wrapped names will not persist.
func TestTopLevelDefineRefusalHandsBackTheCorrectedCell(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	records := []InboxRecord{{Seq: 1, RecordID: "rec-1", Kind: "cast", From: "texture:doc"}, {Seq: 2, RecordID: "rec-2", Kind: "note", From: "texture:doc"}}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	cell := func(source string) SessionResult {
		t.Helper()
		hooks := scope.BindCell()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source, Records: records}, nil, &hooks)
		if err != nil {
			t.Fatalf("cell %q: %v", source, err)
		}
		return out
	}

	out := cell("recs := choir.Records()\nr := recs[0]\nfmt.Println(r.RecordID, r.Kind)\nrecs[1].Kind")
	if !strings.Contains(out.Error, "constant definition loop") {
		t.Fatalf("the shape compiled; this test pins its refusal: %q %q", out.Error, out.Stdout)
	}
	const marker = "Send this cell instead"
	at := strings.Index(out.Error, marker)
	if at < 0 || !strings.Contains(out.Error, "line 2") || !strings.Contains(out.Error, "do not persist") {
		t.Fatalf("the refusal does not hand back the corrected cell: %q", out.Error)
	}
	if strings.Contains(out.Error, "var r = recs[0]") {
		t.Fatalf("the hint suggests a form that fails the same way: %q", out.Error)
	}
	corrected := out.Error[at:]
	corrected = corrected[strings.Index(corrected, "\n")+1:]
	if !strings.HasPrefix(corrected, "recs := choir.Records()\nfunc() {\n") {
		t.Fatalf("corrected cell = %q", corrected)
	}
	got := cell(corrected)
	if got.Error != "" || got.Stdout != "rec-1 cast\nnote\n" {
		t.Fatalf("the corrected cell: error %q stdout %q (cell %q)", got.Error, got.Stdout, corrected)
	}
	if again := cell(`fmt.Println("next")`); again.Stdout != "next\n" {
		t.Fatalf("the corrected cell re-ran on the next cell: %q", again.Stdout)
	}

	// Review r1 (deepseek B1): a wrapped definition nothing reads must not
	// fail "declared and not used"; yaegi does not enforce it.
	only := cell("recs3 := choir.Records()\nr := recs3[0]")
	if at := strings.Index(only.Error, marker); at < 0 {
		t.Fatalf("define-only refusal: %q", only.Error)
	} else {
		fixed := only.Error[at:]
		if got := cell(fixed[strings.Index(fixed, "\n")+1:]); got.Error != "" {
			t.Fatalf("corrected define-only cell: %q", got.Error)
		}
	}

	m := cell("m := map[string]int{\"a\": 1}\na, ok := m[\"a\"]\nfmt.Println(a, ok)")
	if at := strings.Index(m.Error, marker); at < 0 {
		t.Fatalf("comma-ok refusal: %q", m.Error)
	} else {
		fixed := m.Error[at:]
		fixed = fixed[strings.Index(fixed, "\n")+1:]
		if got := cell(fixed); got.Error != "" || got.Stdout != "1 true\n" {
			t.Fatalf("corrected comma-ok cell: %q %q", got.Error, got.Stdout)
		}
	}
}
