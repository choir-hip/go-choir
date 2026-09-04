package yaegikernel

import (
	"context"
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
