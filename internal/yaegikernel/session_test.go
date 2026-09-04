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
// inconsistent interpreter.
func TestSessionPoisonsOnFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sess, err := NewSession(nil, nil)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()
	if _, err := sess.Eval(ctx, `undefined_symbol_xyz`); err == nil {
		t.Fatal("bad cell must fail")
	}
	if _, err := sess.Eval(ctx, `1 + 1`); err == nil || !strings.Contains(err.Error(), "poisoned") {
		t.Fatalf("post-failure eval = %v, want poisoned refusal", err)
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
