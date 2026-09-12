package yaegikernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/traefik/yaegi/interp"
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

// TestSessionImportsSurviveAcrossCells proves imports resolve once and stay
// available: cell 1 imports, cell 2 uses without re-import.
func TestSessionImportsSurviveAcrossCells(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	if _, err := sess.Eval(ctx, `import "strings"`); err != nil {
		t.Fatalf("cell 1 import: %v", err)
	}
	res, err := sess.Eval(ctx, `strings.ToUpper("rlm")`)
	if err != nil {
		t.Fatalf("cell 2 use: %v", err)
	}
	if !res.Value.IsValid() || res.Value.Interface() != "RLM" {
		t.Fatalf("cell 2 value = %v, want RLM", res.Value)
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

// TestClassifyExecuteError covers the residual runtime branch: any
// Execute-phase error that is not a panic is runtime, without message
// inspection.
func TestClassifyExecuteError(t *testing.T) {
	if got := classifyExecuteError(fmt.Errorf("boom"), false); got != DiagRuntime {
		t.Fatalf("plain error = %v, want runtime", got)
	}
	if got := classifyExecuteError(interp.Panic{Value: "x"}, false); got != DiagPanic {
		t.Fatalf("Panic value = %v, want panic", got)
	}
	if got := classifyExecuteError(fmt.Errorf("boom"), true); got != DiagPanic {
		t.Fatalf("recovered panic = %v, want panic", got)
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

// TestSessionLoopPersistsAcrossFrames drives the framed worker loop over
// buffers: two cells share state, close ends cleanly.
func TestSessionLoopPersistsAcrossFrames(t *testing.T) {
	var input strings.Builder
	for _, frame := range []SessionFrame{
		{ID: "1", Source: `total := 40`},
		{ID: "2", Source: `total + 2`},
		{ID: "3", Close: true},
	} {
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(raw)
		input.WriteByte('\n')
	}
	var output bytes.Buffer
	err := RunSessionLoop(strings.NewReader(input.String()), &output, func() (*Session, error) {
		return NewSession(nil, nil)
	})
	if err != nil {
		t.Fatalf("session loop: %v", err)
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
	if len(seen) != 2 {
		t.Fatalf("results = %d, want 2", len(seen))
	}
	if seen["1"].Error != "" || seen["2"].Error != "" {
		t.Fatalf("results = %+v", seen)
	}
}
