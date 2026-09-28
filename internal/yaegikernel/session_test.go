package yaegikernel

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestSessionPersistsVariablesAcrossCells is Def 2 acceptance item 2: cell 1
// defines a variable, cell 2 computes on it without re-import or
// redeclaration, proving the persistent interpreter (not interp.New per eval).
func TestSessionPersistsVariablesAcrossCells(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	if _, err := sess.Eval(ctx, `base := 21`); err != nil {
		t.Fatalf("cell 1: %v", err)
	}
	res, err := sess.Eval(ctx, `base * 2`)
	if err != nil {
		t.Fatalf("cell 2: %v", err)
	}
	if !res.Value.IsValid() || res.Value.Interface() != 42 {
		t.Fatalf("cell 2 value = %v, want 42", res.Value)
	}
}

// TestSessionPoisonsOnFailure proves a failed cell retires the session: the
// next Eval refuses so the broker respawns instead of running on a possibly
// inconsistent interpreter. Only failures that may have executed poison;
// compile-phase rejections preserve (see TestSessionCompileRejectionPreservesHeap).
func TestSessionPoisonsOnFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	if _, err := sess.Eval(ctx, `panic("boom")`); err == nil {
		t.Fatal("panicking cell must fail")
	}
	if _, err := sess.Eval(ctx, `1 + 1`); err == nil || !strings.Contains(err.Error(), "poisoned") {
		t.Fatalf("post-failure eval = %v, want poisoned refusal", err)
	}
}

// TestSessionCompileRejectionPreservesHeap (settlement-gate item 1): a
// compile-phase rejection proves nothing executed, so variables and imports
// survive and the session is reusable. Each failing cell shape is checked.
func TestSessionCompileRejectionPreservesHeap(t *testing.T) {
	for _, src := range []string{
		`func broken(((`,
		`"a" + 1`,
		`undefined_symbol_xyz`,
		"x := 1\nx := 2",
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		sess, err := NewSession(nil, nil)
		if err != nil {
			cancel()
			t.Fatalf("new session: %v", err)
		}
		if _, err := sess.Eval(ctx, `base := 21`); err != nil {
			cancel()
			t.Fatalf("setup: %v", err)
		}
		if _, err := sess.Eval(ctx, `import "strings"`); err != nil {
			cancel()
			t.Fatalf("setup import: %v", err)
		}
		_, evalErr := sess.Eval(ctx, src)
		if evalErr == nil {
			cancel()
			t.Fatalf("cell %q must fail", src)
		}
		ee, ok := AsEvalError(evalErr)
		if !ok || ee.Reuse != ReusePreserve || ee.Kind != DiagCompile {
			cancel()
			t.Fatalf("cell %q = reuse/ok %v %+v, want preserve/compile", src, ok, ee)
		}
		res, err := sess.Eval(ctx, `base * 2`)
		if err != nil {
			cancel()
			t.Fatalf("cell %q poisoned session: %v", src, err)
		}
		if !res.Value.IsValid() || res.Value.Interface() != 42 {
			cancel()
			t.Fatalf("cell %q successor = %v, want 42", src, res.Value)
		}
		res, err = sess.Eval(ctx, `strings.ToUpper("rlm")`)
		if err != nil || !res.Value.IsValid() || res.Value.Interface() != "RLM" {
			cancel()
			t.Fatalf("cell %q successor import = %v, %v, want RLM", src, res.Value, err)
		}
		sess.Close()
		cancel()
	}
}

// TestSessionProgramShapedCells (2026-09-27 texture starvation regression):
// models emit full `package main` programs, not fragments. Repeated imports
// must dedupe silently, func main must be renamed-and-called once, and
// session state must persist across every shape.
func TestSessionProgramShapedCells(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()

	// Two full-program cells: the second re-imports fmt and re-declares
	// main. Pre-normalization the second failed `fmt/_.go redeclared` and
	// the desk retried the identical program forever.
	for i, src := range []string{
		"package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"one\") }",
		"package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"two\") }",
	} {
		res, err := sess.Eval(ctx, src)
		if err != nil {
			t.Fatalf("program cell %d: %v", i, err)
		}
		want := map[int]string{0: "one\n", 1: "two\n"}[i]
		if res.Stdout != want {
			t.Fatalf("program cell %d stdout = %q, want %q", i, res.Stdout, want)
		}
	}

	// A func main from an earlier cell must not re-execute on later cells.
	res, err := sess.Eval(ctx, `fmt.Println("after")`)
	if err != nil || res.Stdout != "after\n" {
		t.Fatalf("post-main cell stdout = %q err=%v, want after only", res.Stdout, err)
	}
	// A mid-session fresh import installs as its own decl fragment; the
	// following statement cells use it, and session state persists.
	if _, err := sess.Eval(ctx, `import "strings"`); err != nil {
		t.Fatalf("mid-session import: %v", err)
	}
	if _, err := sess.Eval(ctx, `x := strings.ToUpper("rlm")`); err != nil {
		t.Fatalf("new-import use: %v", err)
	}
	res, err = sess.Eval(ctx, `x`)
	if err != nil || !res.Value.IsValid() || res.Value.Interface() != "RLM" {
		t.Fatalf("persisted value = %v %v, want RLM", res.Value, err)
	}
}

// TestSessionFailureKinds (settlement-gate item 1): the typed disposition and
// kind travel separately from the verbatim message; timeout and panic are
// unsafe-to-reuse with their own kinds, preflight preserves.
func TestSessionFailureKinds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	newSess := func(t *testing.T) *Session {
		sess, err := NewSession(nil, nil)
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		return sess
	}
	sess := newSess(t)
	_, err := sess.Eval(ctx, "import \"os\"\nfunc main() {}")
	ee, ok := AsEvalError(err)
	if !ok || ee.Reuse != ReusePreserve || ee.Kind != DiagImportPreflight {
		t.Fatalf("preflight = %v %+v, want preserve/import_preflight", ok, ee)
	}
	sess.Close()
	sess = newSess(t)
	_, err = sess.Eval(ctx, `panic("boom")`)
	ee, ok = AsEvalError(err)
	if !ok || ee.Reuse != ReuseUnsafeToReuse || ee.Kind != DiagPanic {
		t.Fatalf("panic = %v %+v, want unsafe/panic", ok, ee)
	}
	if ee.Error() != "boom" {
		t.Fatalf("panic message = %q, want verbatim boom", ee.Error())
	}
	sess = newSess(t)
	defer sess.Close()
	// Index-out-of-range surfaces as a Yaegi Panic value: still unsafe, kind
	// panic (not runtime). The classifier keys on the Panic type, never text.
	_, err = sess.Eval(ctx, `a := []int{1}; _ = a[99]`)
	ee, ok = AsEvalError(err)
	if !ok || ee.Reuse != ReuseUnsafeToReuse || ee.Kind != DiagPanic {
		t.Fatalf("index panic = %v %+v, want unsafe/panic", ok, ee)
	}
}

// TestSessionTimeoutPoisons (settlement-gate item 1): a cell that outlives its
// context is unsafe-to-reuse with kind timeout.
func TestSessionTimeoutPoisons(t *testing.T) {
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := sess.Eval(ctx, `m := 1`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	tctx, tcancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer tcancel()
	_, err = sess.Eval(tctx, `for {}`)
	ee, ok := AsEvalError(err)
	if !ok || ee.Reuse != ReuseUnsafeToReuse || ee.Kind != DiagTimeout {
		t.Fatalf("timeout = %v %+v, want unsafe/timeout", ok, ee)
	}
	if _, err := sess.Eval(ctx, `m`); err == nil {
		t.Fatal("timed-out session must refuse")
	}
}

// TestSessionLoopSurvivesCompileRejection (settlement-gate item 1): the
// framed loop keeps the worker alive across a compile rejection and serves
// the successor on the same heap; a runtime failure still ends the loop.
func TestSessionLoopSurvivesCompileRejection(t *testing.T) {
	frames := []SessionFrame{
		{ID: "1", Source: `total := 40`},
		{ID: "2", Source: `undefined_symbol_xyz`},
		{ID: "3", Source: `total + 2`},
		{ID: "4", Close: true},
	}
	var input strings.Builder
	for _, frame := range frames {
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(raw)
		input.WriteByte('\n')
	}
	var output bytes.Buffer
	if err := RunSessionLoop(strings.NewReader(input.String()), &output, func() (*Session, error) {
		return NewSession(nil, nil)
	}); err != nil {
		t.Fatalf("loop must survive compile rejection: %v", err)
	}
	dec := json.NewDecoder(&output)
	seen := map[string]SessionResult{}
	for dec.More() {
		var res SessionResult
		if err := dec.Decode(&res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		seen[res.ID] = res
	}
	if len(seen) != 3 {
		t.Fatalf("results = %d, want 3", len(seen))
	}
	if seen["2"].Error == "" {
		t.Fatal("rejected cell must carry an error")
	}
	if seen["2"].Reuse != ReusePreserve || seen["2"].DiagKind != DiagCompile {
		t.Fatalf("rejected cell = %+v, want preserve/compile", seen["2"])
	}
	if seen["3"].Error != "" {
		t.Fatalf("successor must run on the preserved heap: %+v", seen["3"])
	}
}

// TestSessionRejectsDisallowedImports proves the session enforces the same
// allowlist as one-shot evaluation.
func TestSessionRejectsDisallowedImports(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	if _, err := sess.Eval(ctx, "import \"os\"\nfunc main() {}"); err == nil {
		t.Fatal("disallowed import must fail")
	}
}

// TestSessionImportStatementMix hoists the import+statement cell shape — the
// exact source form models emit (`import "x"` followed by bare statements)
// and the shape that starved the texture desk's required-write loop on
// staging (guest run-memory pull, M11): file-mode compile rejected every
// statement after the import decl. Imports lift into a decl, top-level
// `x := e` hoists to `var x = e` so bindings persist across cells, and the
// remaining statements run once inside the renamed cell main.
func TestSessionImportStatementMix(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	res, err := sess.Eval(ctx, "import \"fmt\"\ny := 8\nfmt.Println(y)\n")
	if err != nil {
		t.Fatalf("mixed import/statement cell: %v", err)
	}
	if res.Stdout != "8\n" {
		t.Fatalf("mixed cell stdout=%q", res.Stdout)
	}
	// Hoisted y must persist — a later bare-statement cell reads it.
	res, err = sess.Eval(ctx, "fmt.Println(y*3)")
	if err != nil {
		t.Fatalf("follow-up cell: %v", err)
	}
	if res.Stdout != "24\n" {
		t.Fatalf("follow-up stdout=%q — := did not hoist to a persistent var", res.Stdout)
	}
	// A later cell re-importing fmt dedupes instead of redeclaring.
	res, err = sess.Eval(ctx, "import \"fmt\"\nfmt.Println(\"again\")\n")
	if err != nil {
		t.Fatalf("re-import cell: %v", err)
	}
	if res.Stdout != "again\n" {
		t.Fatalf("re-import stdout=%q", res.Stdout)
	}
	// Re-`:=` on an already-hoisted name inside another import-mix cell
	// degrades to assignment, not a redeclare — the staging retry pattern.
	res, err = sess.Eval(ctx, "import \"strings\"\ny := y * 2\nfmt.Println(strings.Repeat(\"x\", y/4))")
	if err != nil {
		t.Fatalf("re-declare cell: %v", err)
	}
	if res.Stdout != "xxxx\n" {
		t.Fatalf("re-declare stdout=%q — := did not degrade to assignment", res.Stdout)
	}
	// The allowlist preflight still gates the mix shape: a disallowed import
	// followed by statements must be refused before normalization rescues it.
	sess2, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess2.Close()
	if _, err := sess2.Eval(ctx, "import \"os\"\nx := 1\nfmt.Println(x)"); err == nil {
		t.Fatal("disallowed import inside import+statement mix bypassed preflight")
	}
}
