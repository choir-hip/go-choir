package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBeadsFixture(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "issues.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func rulesOf(ws []warning, rule string) []warning {
	var out []warning
	for _, w := range ws {
		if w.Rule == rule {
			out = append(out, w)
		}
	}
	return out
}

func TestR8MissingStoreSkips(t *testing.T) {
	ws := validateBeadsStore(filepath.Join(t.TempDir(), "nope.jsonl"), 0)
	r8 := rulesOf(ws, "R8")
	if len(r8) != 1 || r8[0].Severity != "info" {
		t.Fatalf("expected single info skip note, got %+v", r8)
	}
}

func TestR8DanglingAndCycle(t *testing.T) {
	// a -> b (blocks) where b exists; a -> ghost (dangling);
	// c <-> d cycle via blocks.
	lines := []string{
		`{"id":"a","issue_type":"epic","status":"open","external_ref":"mg:a","dependencies":[{"issue_id":"a","depends_on_id":"b","type":"blocks"},{"issue_id":"a","depends_on_id":"ghost","type":"blocks"}]}`,
		`{"id":"b","issue_type":"epic","status":"open","external_ref":"mg:b"}`,
		`{"id":"c","issue_type":"epic","status":"open","dependencies":[{"issue_id":"c","depends_on_id":"d","type":"blocks"}]}`,
		`{"id":"d","issue_type":"epic","status":"open","dependencies":[{"issue_id":"d","depends_on_id":"c","type":"blocks"}]}`,
	}
	p := writeBeadsFixture(t, lines)
	ws := validateBeadsStore(p, 2) // graph has 2 nodes; 2 mg:* epics -> no drift
	r8 := rulesOf(ws, "R8")

	var dangling, cycle, drift bool
	for _, w := range r8 {
		switch {
		case strings.Contains(w.Message, "unknown issue \"ghost\""):
			dangling = true
		case strings.Contains(w.Message, "cycle detected"):
			cycle = true
		case strings.Contains(w.Message, "binding drift"):
			drift = true
		}
	}
	if !dangling {
		t.Errorf("expected dangling-edge warning, got %+v", r8)
	}
	if !cycle {
		t.Errorf("expected cycle warning, got %+v", r8)
	}
	if drift {
		t.Errorf("did not expect binding drift (2 mg:* epics vs 2 nodes), got %+v", r8)
	}
}

func TestR8VariantAndDrift(t *testing.T) {
	// epic e with one open + one closed child -> V=1.
	// 1 mg:* epic vs graphNodeCount=3 -> drift.
	lines := []string{
		`{"id":"e","issue_type":"epic","status":"open","external_ref":"mg:e"}`,
		`{"id":"e.1","issue_type":"task","status":"open","dependencies":[{"issue_id":"e.1","depends_on_id":"e","type":"parent-child"}]}`,
		`{"id":"e.2","issue_type":"task","status":"closed","dependencies":[{"issue_id":"e.2","depends_on_id":"e","type":"parent-child"}]}`,
	}
	p := writeBeadsFixture(t, lines)
	ws := validateBeadsStore(p, 3)
	r8 := rulesOf(ws, "R8")

	var vOK, drift bool
	for _, w := range r8 {
		if strings.Contains(w.Message, `epic "e" V=1`) {
			vOK = true
		}
		if strings.Contains(w.Message, "binding drift: 1 mg:* epics vs 3 graph nodes") {
			drift = true
		}
	}
	if !vOK {
		t.Errorf("expected V=1 for epic e, got %+v", r8)
	}
	if !drift {
		t.Errorf("expected binding drift warning, got %+v", r8)
	}
}
