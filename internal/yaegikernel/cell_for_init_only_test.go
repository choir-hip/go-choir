package yaegikernel

import (
	"context"
	"strings"
	"testing"
)

// Yaegi v0.16.1 jumps back to the init statement of `for init; ; {}` on
// every iteration, so a body that advances the loop variable never ends. On
// the paper, cells scanning a string with `for i := 0; ; { … i = pos + n }`
// grew until the guest OOM killer took a worker (12:46) and then the daemon
// (18:55) (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
//
// Ways this could fail, written first: a broken shape is missed (a `=`
// init, two variables, a call or index in the init, a loop inside a closure
// or a func declaration, several loops in one cell, a cell with imports);
// a loop that is not the shape is rewritten (no init, a condition, a post
// statement, a range clause, a bare `for {`); text in a string or comment
// is rewritten; a header holding a func literal or spanning lines is
// corrupted; a compile error's line number moves; the rewrite panics on
// source that does not scan.
func TestGuardInitOnlyForLoopsRewritesOnlyTheBrokenShape(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"define", "for i := 0; ; {\n}", "for i := 0; true; {\n}"},
		{"assign", "for i = 0; ; {}", "for i = 0; true; {}"},
		{"two vars", "for i, k := 0, 0; ; {}", "for i, k := 0, 0; true; {}"},
		{"call and index init", "for i := f(a, b[0]); ; {}", "for i := f(a, b[0]); true; {}"},
		{"no space", "for i := 0;;{}", "for i := 0;true;{}"},
		{"two loops", "for i := 0; ; {}\nfor j := 1; ; { x() }", "for i := 0; true; {}\nfor j := 1; true; { x() }"},
		{"inside closure", "f := func(s string) { for i := 0; ; { break } }", "f := func(s string) { for i := 0; true; { break } }"},
		{"nested", "for i := 0; ; { for j := 0; ; { break }; break }", "for i := 0; true; { for j := 0; true; { break }; break }"},
		{"no init", "for ; ; {}", "for ; ; {}"},
		{"condition", "for i := 0; i < 3; {}", "for i := 0; i < 3; {}"},
		{"post", "for i := 0; ; i++ {}", "for i := 0; ; i++ {}"},
		{"range", "for k, v := range m {}", "for k, v := range m {}"},
		{"bare", "for {}", "for {}"},
		{"cond only", "for ok {}", "for ok {}"},
		{"string", `s := "for i := 0; ; {"`, `s := "for i := 0; ; {"`},
		{"raw string", "s := `for i := 0; ; {`", "s := `for i := 0; ; {`"},
		{"comment", "// for i := 0; ; {\nx := 1", "// for i := 0; ; {\nx := 1"},
		{"func literal in header", "for f := func() int { return 0 }; ; {}", "for f := func() int { return 0 }; ; {}"},
		{"header across lines", "for i := 0;\n; {}", "for i := 0;\ntrue; {}"},
		{"unterminated", "for i := 0; ; { s := \"", "for i := 0; true; { s := \""},
		{"empty", "", ""},
	} {
		if got := guardInitOnlyForLoops(c.src); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// The three loops the paper's runs 512f3f66, 7349d774 and c81bb26b ran when
// their workers died, verbatim but for a guard that turns a runaway into a
// printed failure instead of an OOM.
func TestInitOnlyForLoopsFromThePaperEndInACell(t *testing.T) {
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	ctx := context.Background()
	for _, c := range []struct{ name, src, want string }{
		{"512f3f66 find", `
guard := 0
find := func(s, kw string) []string {
	var out []string
	for i := 0; ; {
		guard++
		if guard > 100 {
			fmt.Println("RUNAWAY")
			break
		}
		j := strings.Index(s[i:], kw)
		if j < 0 {
			break
		}
		pos := i + j
		lo := pos - 2
		if lo < 0 {
			lo = 0
		}
		hi := pos + 3
		if hi > len(s) {
			hi = len(s)
		}
		out = append(out, s[lo:hi])
		i = pos + len(kw)
	}
	return out
}
fmt.Println(len(find("a thin, thinner, thinnest", "thin")))`, "3"},
		{"7349d774 extractCites", `
guard := 0
extractCites := func(s string) []string {
	var out []string
	for i := 0; ; {
		guard++
		if guard > 100 {
			fmt.Println("RUNAWAY")
			break
		}
		j := strings.Index(s[i:], "{{cite")
		if j < 0 {
			break
		}
		j += i
		k := strings.Index(s[j:], "}}")
		if k < 0 {
			break
		}
		out = append(out, s[j:j+k+2])
		i = j + k + 2
	}
	return out
}
fmt.Println(extractCites("a {{cite x}} b {{cite y}} c"))`, "[{{cite x}} {{cite y}}]"},
		{"c81bb26b idxs", `
out := "one Super Pac, two Super Pac"
guard := 0
idxs := []int{}
for i := 0; ; {
    guard++
    if guard > 100 { fmt.Println("RUNAWAY"); break }
    j := strings.Index(out[i:], "Super Pac")
    if j < 0 { break }
    idxs = append(idxs, i+j)
    i = i + j + 9
}
fmt.Println("hits", idxs)`, "hits [4 19]"},
	} {
		res, err := sess.Eval(ctx, c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := strings.TrimSpace(res.Stdout); got != c.want {
			t.Fatalf("%s printed %q, want %q", c.name, got, c.want)
		}
	}
}

// The rewrite inserts on the loop's own line, so a compile error later in
// the cell reads exactly as it does for the cell written with `true`.
func TestInitOnlyForLoopRewriteKeepsCompileErrorLines(t *testing.T) {
	compileErr := func(src string) string {
		sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		_, err = sess.Eval(context.Background(), src)
		if err == nil {
			t.Fatalf("cell %q: want a compile error", src)
		}
		return err.Error()
	}
	got := compileErr("for i := 0; ; {\n\tbreak\n}\nundefinedName()")
	want := compileErr("for i := 0; true; {\n\tbreak\n}\nundefinedName()")
	if got != want {
		t.Fatalf("rewritten cell's error %q, hand-written cell's error %q", got, want)
	}
}
