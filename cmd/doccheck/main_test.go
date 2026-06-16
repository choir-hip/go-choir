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

func TestTextureRetiredNameAllowlist(t *testing.T) {
	retiredName := "v" + "text"
	retiredDisplay := "V" + "Text"
	docs := map[string]*docInfo{
		"docs/historical.md":                      {Path: "docs/historical.md", Scope: "historical", IsEvidence: true},
		"docs/current.md":                         {Path: "docs/current.md", Scope: "current", IsEvidence: false},
		"docs/mission-texture-hard-cutover-v0.md": {Path: "docs/mission-texture-hard-cutover-v0.md", Scope: "current", IsEvidence: false},
	}
	tests := []struct {
		path string
		line string
		want bool
	}{
		{"docs/why-texture-background-2026-06-15.md", retiredDisplay + " was the old name.", true},
		{"docs/historical.md", retiredDisplay + " was used here.", true},
		{"docs/mission-texture-hard-cutover-v0.md", "Retired " + retiredDisplay + " evidence remains in the mission.", true},
		{"docs/mission-texture-hard-cutover-v0.md", "Selected affordance line counts: /api/" + retiredName + " 505.", true},
		{"docs/mission-texture-hard-cutover-v0.md", "  `edit_" + retiredName + "` 390, `request_super_execution` 122.", true},
		{"cmd/doccheck/main.go", `for _, term := range []string{"` + retiredName + `"}`, true},
		{"internal/runtime/texture.go", "// texture-cutover-allow: " + retiredName + " route shim; delete-by texture-hard-cutover-v0", true},
		{"docs/current.md", retiredDisplay + " owns canonical versions.", false},
		{"internal/runtime/texture.go", "const path = \"/api/" + retiredName + "/documents\"", false},
	}
	for _, tt := range tests {
		if got := isAllowedTextureRetiredNameLine(tt.path, tt.line, docs); got != tt.want {
			t.Fatalf("isAllowedTextureRetiredNameLine(%q, %q) = %v, want %v", tt.path, tt.line, got, tt.want)
		}
	}
}

func TestHasTextureRetiredName(t *testing.T) {
	retiredName := "v" + "text"
	retiredDisplay := "V" + "Text"
	tests := []struct {
		line string
		want bool
	}{
		{"Open the Texture document.", false},
		{"Open the " + retiredDisplay + " document.", true},
		{"POST /api/" + retiredName + "/documents", true},
		{"data-" + retiredName + "-editor", true},
		{"edit_" + retiredName, true},
	}
	for _, tt := range tests {
		if got := hasTextureRetiredName(tt.line); got != tt.want {
			t.Fatalf("hasTextureRetiredName(%q) = %v, want %v", tt.line, got, tt.want)
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
