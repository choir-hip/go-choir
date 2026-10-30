package yaegikernel

import (
	"strings"
	"testing"
)

// S1e (docs/archive/problems/desk-verb-error-vanishes-as-final-expression-2026-10-11.md):
// a research round ended its cell with a bare choir.ReportPacket call that
// was refused; the cell showed "=> " and the round's leads never reached
// Texture. Failure modes pinned: a refused verb ending a cell shows nothing;
// a refusal leaks into the next cell; a successful verb gains a refusal
// line; an error the cell already printed shows twice; a variadic verb
// breaks through the wrapper.
func TestRefusedVerbIsShownInTheCellResult(t *testing.T) {
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

	out := cell(`choir.Note("", "x")`)
	if !strings.Contains(out.Stdout, "choir.Note refused:") || !strings.Contains(out.Stdout, "destination") {
		t.Fatalf("refused verb not shown: %q", out.Stdout)
	}
	if out := cell(`x := 1
x`); strings.Contains(out.Stdout, "refused") {
		t.Fatalf("refusal leaked into the next cell: %q", out.Stdout)
	}
	if out := cell(`choir.WriteFile("ok.txt", "hi")`); strings.Contains(out.Stdout, "refused") {
		t.Fatalf("successful verb gained a refusal line: %q", out.Stdout)
	}
	out = cell(`_, err := choir.ReadFile("missing.txt")
fmt.Println(err)`)
	if strings.Contains(out.Stdout, "refused") {
		t.Fatalf("an error the cell printed is shown twice: %q", out.Stdout)
	}
	if out := cell(`choir.Help("Report")`); !strings.Contains(out.Stdout, "Report") {
		t.Fatalf("variadic verb broken by the wrapper: %q", out.Stdout)
	}
}
