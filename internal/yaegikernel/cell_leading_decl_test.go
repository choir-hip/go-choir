package yaegikernel

import (
	"context"
	"strings"
	"testing"
)

// A cell that starts with a type, var or const declaration and goes on with
// statements failed "expected declaration": yaegi picks its parse mode from
// the first token. In the three Cut 0 trace bundles this was the largest
// class of failed desk cells (22 of 67), and it repeated: the model's next
// cell often had the same shape (scratchpad repl-errors analysis,
// 2026-10-13).
//
// Ways this could fail, written first: the cell runs but its type or
// variables do not persist into the next cell; a pure declaration cell
// (which parses as a file) is rewritten and changes meaning; the leading
// guard breaks a cell that starts with a comment.
func TestCellStartingWithDeclarationRunsAndPersists(t *testing.T) {
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	ctx := context.Background()
	for _, c := range []struct{ src, want string }{
		{"type pair struct{ a int }\nps := []pair{{1}, {2}}\nfmt.Println(len(ps))", "2"},
		{"var total int\nfor _, p := range ps { total += p.a }\nfmt.Println(total)", "3"},
		{"const scale = 10\nfmt.Println(total * scale)", "30"},
		{"// a comment first\ntype other struct{ b string }\nos := other{b: \"x\"}\nfmt.Println(os.b, len(ps))", "x 2"},
		{"type onlyDecl struct{ c int }", ""},
		{"fmt.Println(onlyDecl{c: 4}.c, pair{a: 5}.a)", "4 5"},
	} {
		out, err := sess.Eval(ctx, c.src)
		if err != nil {
			t.Fatalf("cell %q: %v", c.src, err)
		}
		if c.want != "" && strings.TrimSpace(out.Stdout) != c.want {
			t.Fatalf("cell %q printed %q, want %q", c.src, out.Stdout, c.want)
		}
	}
}
