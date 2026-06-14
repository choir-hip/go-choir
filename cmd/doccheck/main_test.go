package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMissionGraphDetectsCycle(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.MkdirAll("docs", 0755); err != nil {
		t.Fatal(err)
	}
	graph := `schema_version: 0
status: active
nodes:
  - id: a
    title: A
    path: ""
    ledger: ""
    status: working
    kind: spine
    depends_on: [b]
  - id: b
    title: B
    path: ""
    ledger: ""
    status: planned
    kind: spine
    depends_on: [a]
`
	if err := os.WriteFile("docs/mission-graph.yaml", []byte(graph), 0644); err != nil {
		t.Fatal(err)
	}
	_, warnings := validateMissionGraph("docs/mission-graph.yaml", nil)
	if !hasWarningMessage(warnings, "dependency cycle") {
		t.Fatalf("expected cycle warning, got %#v", warnings)
	}
}

func TestValidateAssertionRegisterExtractsSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.md")
	content := `# Register

## Assertions

### A1 - thing

## Invariant candidates

### I1 - invariant

## Open hyperthesis edges

### E1 - edge
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	rep, warnings := validateAssertionRegister(path)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if len(rep.Assertions) != 1 || rep.Assertions[0] != "A1" {
		t.Fatalf("unexpected assertions: %#v", rep.Assertions)
	}
	if len(rep.InvariantCandidates) != 1 || rep.InvariantCandidates[0] != "I1" {
		t.Fatalf("unexpected invariants: %#v", rep.InvariantCandidates)
	}
	if len(rep.OpenEdges) != 1 || rep.OpenEdges[0] != "E1" {
		t.Fatalf("unexpected edges: %#v", rep.OpenEdges)
	}
}

func TestClassifyHeresyContext(t *testing.T) {
	docs := map[string]*docInfo{
		"docs/evidence.md": {Path: "docs/evidence.md", Scope: "historical", IsEvidence: true},
	}
	tests := []struct {
		path string
		line string
		want string
	}{
		{"docs/heresy-detectors.md", "`Trace app`", "detector-definition"},
		{"docs/evidence.md", "Trace app happened", "historical-evidence"},
		{"internal/foo.go", "// deprecated BrowserApp compatibility shim", "explicitly-deprecated"},
		{"internal/foo.go", "// transitional BrowserApp shim", "implementation-transitional"},
		{"internal/foo.go", "BrowserApp()", "current-violation"},
	}
	for _, tt := range tests {
		if got := classifyHeresyContext(tt.path, tt.line, docs); got != tt.want {
			t.Fatalf("classifyHeresyContext(%q, %q) = %q, want %q", tt.path, tt.line, got, tt.want)
		}
	}
}

func hasWarningMessage(warnings []warning, needle string) bool {
	for _, w := range warnings {
		if strings.Contains(w.Message, needle) {
			return true
		}
	}
	return false
}
