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
    entrypoint: true
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

func TestValidateMissionGraphEntrypointCardinality(t *testing.T) {
	tests := []struct {
		name         string
		nodes        string
		wantWarnings []string
		noWarnings   []string
	}{
		{
			name: "zero rejected",
			nodes: `  - id: a
    title: A
    status: working
    kind: spine
`,
			wantWarnings: []string{"expected exactly one product mission graph entrypoint, found 0"},
		},
		{
			name: "one product accepted",
			nodes: `  - id: a
    title: A
    entrypoint: true
    execution_mode: mission_orchestrator
    status: working
    kind: spine
`,
			noWarnings: []string{"entrypoint", "invalid kind"},
		},
		{
			name: "product plus maintenance accepted",
			nodes: `  - id: a
    title: A
    entrypoint: true
    execution_mode: mission_orchestrator
    status: working
    kind: spine
  - id: b
    title: B
    entrypoint: true
    execution_mode: scope_disjoint_maintenance
    status: working
    kind: ci_maintenance
`,
			noWarnings: []string{"entrypoint", "invalid kind"},
		},
		{
			name: "second product rejected",
			nodes: "  - id: a\n" +
				"    title: A\n" +
				"    entrypoint: true\n" +
				"    execution_mode: mission_orchestrator\n" +
				"    status: working\n" +
				"    kind: spine\n" +
				"  - id: b\n" +
				"    title: B\n" +
				"    entrypoint: true\n" +
				"    execution_mode: mission_orchestrator\n" +
				"    status: working\n" +
				"    kind: spine\n",
			wantWarnings: []string{"expected exactly one product mission graph entrypoint, found 2"},
		},
		{
			name: "stale maintenance rejected",
			nodes: "  - id: a\n" +
				"    title: A\n" +
				"    entrypoint: true\n" +
				"    execution_mode: mission_orchestrator\n" +
				"    status: working\n" +
				"    kind: spine\n" +
				"  - id: maintenance\n" +
				"    title: Maintenance\n" +
				"    entrypoint: true\n" +
				"    execution_mode: scope_disjoint_maintenance\n" +
				"    status: settled\n" +
				"    kind: ci_maintenance\n",
			wantWarnings: []string{`mission graph entrypoint "maintenance" must be working`},
		},
		{
			name: "arbitrary extra entrypoint rejected",
			nodes: "  - id: a\n" +
				"    title: A\n" +
				"    entrypoint: true\n" +
				"    execution_mode: mission_orchestrator\n" +
				"    status: working\n" +
				"    kind: spine\n" +
				"  - id: legacy\n" +
				"    title: Legacy\n" +
				"    entrypoint: true\n" +
				"    execution_mode: subordinate_only\n" +
				"    status: working\n" +
				"    kind: side\n",
			wantWarnings: []string{`mission graph entrypoint "legacy" has invalid shape`},
		},
		{
			name: "second maintenance rejected",
			nodes: "  - id: a\n" +
				"    title: A\n" +
				"    entrypoint: true\n" +
				"    execution_mode: mission_orchestrator\n" +
				"    status: working\n" +
				"    kind: spine\n" +
				"  - id: maintenance-a\n" +
				"    title: Maintenance A\n" +
				"    entrypoint: true\n" +
				"    execution_mode: scope_disjoint_maintenance\n" +
				"    status: working\n" +
				"    kind: ci_maintenance\n" +
				"  - id: maintenance-b\n" +
				"    title: Maintenance B\n" +
				"    entrypoint: true\n" +
				"    execution_mode: scope_disjoint_maintenance\n" +
				"    status: planned\n" +
				"    kind: ci_maintenance\n",
			wantWarnings: []string{"expected at most one scope-disjoint CI maintenance entrypoint, found 2", `mission graph entrypoint "maintenance-b" must be working`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mission-graph.yaml")
			graph := "schema_version: 0\nstatus: active\nnodes:\n" + tt.nodes
			if err := os.WriteFile(path, []byte(graph), 0644); err != nil {
				t.Fatal(err)
			}

			_, warnings := validateMissionGraph(path, nil)
			for _, expected := range tt.wantWarnings {
				if !hasWarningMessage(warnings, expected) {
					t.Fatalf("expected warning %q: %#v", expected, warnings)
				}
			}
			for _, forbidden := range tt.noWarnings {
				if hasWarningMessage(warnings, forbidden) {
					t.Fatalf("unexpected warning %q: %#v", forbidden, warnings)
				}
			}
		})
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

func TestScanForbiddenSourceMarkdownAllowsYAMLPrompts(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	for _, root := range []string{"internal/agentcore", "internal/textureowner"} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll("internal/promptstore/defaults", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/promptstore/defaults/texture.yaml", []byte("version: 1\nbody: |\n  hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"internal/agentcore/legacy.md",
		"internal/textureowner/legacy.md",
		"internal/promptstore/defaults/legacy.md",
	} {
		if err := os.WriteFile(path, []byte("# no\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	warnings, err := scanForbiddenSourceMarkdown()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, path := range []string{
		"internal/agentcore/legacy.md",
		"internal/textureowner/legacy.md",
		"internal/promptstore/defaults/legacy.md",
	} {
		if !hasWarningPath(warnings, path) {
			t.Fatalf("expected %s warning, got %#v", path, warnings)
		}
	}
}

func hasWarningPath(warnings []warning, path string) bool {
	for _, w := range warnings {
		if w.Path == path {
			return true
		}
	}
	return false
}

func TestClassifySurfaceRecognizesCurrentPromptPathsOnly(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/promptstore/defaults/texture.yaml", "runtime-prompt"},
		{"internal/textureprompts/texture.yaml", "runtime-prompt"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := classifySurface(tt.path); got != tt.want {
				t.Fatalf("classifySurface(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
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

func TestInferClassificationTreatsArchiveAsHistoricalEvidence(t *testing.T) {
	scope, evidence := inferClassification("docs/archive/north-star.md")
	if scope != "historical" || !evidence {
		t.Fatalf("inferClassification(archive) = (%q, %v), want (historical, true)", scope, evidence)
	}
	if !isExpectedUnreachableHistoricalEvidence(&docInfo{Path: "docs/archive/north-star.md", IsEvidence: true}) {
		t.Fatal("historical archive evidence should not require current-entry reachability")
	}
	if isExpectedUnreachableHistoricalEvidence(&docInfo{Path: "docs/current.md", IsEvidence: true}) {
		t.Fatal("non-archive evidence should retain its reachability diagnostic")
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
		{"internal/textureowner/texture.go", "// texture-cutover-allow: " + retiredName + " route shim; delete-by texture-hard-cutover-v0", true},
		{"docs/current.md", retiredDisplay + " owns canonical versions.", false},
		{"internal/textureowner/texture.go", "const path = \"/api/" + retiredName + "/documents\"", false},
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

func TestValidateLiveReadPathAcceptsCompletePacket(t *testing.T) {
	rep := completeLivePacketReport()
	if failures := validateLiveReadPath(rep); len(failures) != 0 {
		t.Fatalf("validateLiveReadPath() failures = %#v, want none", failures)
	}
}

func TestValidateLiveReadPathRejectsMissingRouterLink(t *testing.T) {
	rep := completeLivePacketReport()
	rep.Edges = rep.Edges[1:]

	failures := validateLiveReadPath(rep)
	if len(failures) != 1 {
		t.Fatalf("validateLiveReadPath() failure count = %d, want 1: %#v", len(failures), failures)
	}
	if failures[0].Rule != "L2" {
		t.Fatalf("validateLiveReadPath() failure rule = %q, want L2", failures[0].Rule)
	}
}

func TestValidateLiveReadPathRejectsInvalidMissionGraphEntrypointCardinality(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []missionGraphNode
		wantCount string
	}{
		{name: "zero", wantCount: "found 0"},
		{
			name: "second product",
			nodes: []missionGraphNode{
				{ID: "old", EntryPoint: true, ExecutionMode: "mission_orchestrator", Status: "working", Kind: "spine"},
				{ID: "current", EntryPoint: true, ExecutionMode: "mission_orchestrator", Status: "working", Kind: "spine"},
			},
			wantCount: "found 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := completeLivePacketReport()
			rep.MissionGraph.Nodes = tt.nodes

			failures := validateLiveReadPath(rep)
			if !hasWarningMessage(failures, tt.wantCount) {
				t.Fatalf("validateLiveReadPath() failures = %#v, want L5 containing %q", failures, tt.wantCount)
			}
		})
	}
}

func TestValidateLiveReadPathRejectsInvalidMissionGraphEntrypointIdentity(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*missionGraphNode)
		wantDetail string
	}{
		{name: "status", mutate: func(node *missionGraphNode) { node.Status = "superseded" }, wantDetail: `status="superseded"`},
		{name: "kind", mutate: func(node *missionGraphNode) { node.Kind = "product_completion" }, wantDetail: `kind="product_completion"`},
		{name: "execution mode", mutate: func(node *missionGraphNode) { node.ExecutionMode = "subordinate_only" }, wantDetail: `execution_mode="subordinate_only"`},
		{name: "path", mutate: func(node *missionGraphNode) { node.Path = "docs/definitions/old.md" }, wantDetail: `path="docs/definitions/old.md"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := completeLivePacketReport()
			tt.mutate(&rep.MissionGraph.Nodes[0])

			failures := validateLiveReadPath(rep)
			if !hasWarningMessage(failures, tt.wantDetail) {
				t.Fatalf("validateLiveReadPath() failures = %#v, want L5 containing %q", failures, tt.wantDetail)
			}
		})
	}
}

func TestValidateLiveReadPathAcceptsScopeDisjointMaintenanceEntrypoint(t *testing.T) {
	rep := completeLivePacketReport()
	rep.Documents = append(rep.Documents, scopeDisjointMaintenanceDoc([]string{"entry"}))
	rep.MissionGraph.Nodes = append(rep.MissionGraph.Nodes, scopeDisjointMaintenanceNode())

	if failures := validateLiveReadPath(rep); len(failures) != 0 {
		t.Fatalf("validateLiveReadPath() failures = %#v, want none", failures)
	}
}

func TestValidateLiveReadPathRejectsInvalidScopeDisjointMaintenanceEntrypoint(t *testing.T) {
	tests := []struct {
		name       string
		mutateNode func(*missionGraphNode)
		mutateDoc  func(*docInfo)
		wantDetail string
	}{
		{
			name:       "stale maintenance",
			mutateNode: func(node *missionGraphNode) { node.Status = "settled" },
			wantDetail: `must be working, got status="settled"`,
		},
		{
			name:       "maintenance authority root",
			mutateDoc:  func(doc *docInfo) { doc.IsRoot = []string{"entry", "authority"} },
			wantDetail: "entry-only scope_disjoint_ci_maintenance Definition",
		},
		{
			name:       "maintenance arbitrary extra root",
			mutateDoc:  func(doc *docInfo) { doc.IsRoot = []string{"entry", "other"} },
			wantDetail: "entry-only scope_disjoint_ci_maintenance Definition",
		},
		{
			name:       "maintenance missing manifest identity",
			mutateDoc:  func(doc *docInfo) { doc.Annotations["authority"] = "mission" },
			wantDetail: "entry-only scope_disjoint_ci_maintenance Definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := completeLivePacketReport()
			doc := scopeDisjointMaintenanceDoc([]string{"entry"})
			node := scopeDisjointMaintenanceNode()
			if tt.mutateDoc != nil {
				tt.mutateDoc(&doc)
			}
			if tt.mutateNode != nil {
				tt.mutateNode(&node)
			}
			rep.Documents = append(rep.Documents, doc)
			rep.MissionGraph.Nodes = append(rep.MissionGraph.Nodes, node)

			if failures := validateLiveReadPath(rep); !hasWarningMessage(failures, tt.wantDetail) {
				t.Fatalf("validateLiveReadPath() failures = %#v, want L5 containing %q", failures, tt.wantDetail)
			}
		})
	}
}

func completeLivePacketReport() report {
	rep := report{}
	for _, path := range defaultReadPacket {
		doc := docInfo{
			Path:        path,
			Scope:       "current",
			Manifested:  true,
			Exists:      true,
			Annotations: map[string]string{},
		}
		if path == "docs/definitions/choir-audited-autoputer-construction-2026-07-15.md" {
			doc.Annotations["doc_role"] = "definition"
			doc.IsRoot = []string{"authority", "entry"}
		}
		rep.Documents = append(rep.Documents, doc)
		if path != "docs/README.md" {
			rep.Edges = append(rep.Edges, edge{From: "docs/README.md", To: path, Kind: "markdown"})
		}
	}
	rep.MissionGraph = graphReport{
		Path: defaultGraph,
		Nodes: []missionGraphNode{{
			ID:            "current",
			Path:          "docs/definitions/choir-audited-autoputer-construction-2026-07-15.md",
			EntryPoint:    true,
			ExecutionMode: "mission_orchestrator",
			Status:        "working",
			Kind:          "spine",
		}},
	}
	return rep
}

func scopeDisjointMaintenanceDoc(roots []string) docInfo {
	return docInfo{
		Path:       "docs/definitions/choir-ci-optimization-2026-07-16.md",
		Scope:      "current",
		IsRoot:     roots,
		Manifested: true,
		Exists:     true,
		Annotations: map[string]string{
			"doc_role":  "definition",
			"authority": "scope_disjoint_ci_maintenance",
		},
	}
}

func scopeDisjointMaintenanceNode() missionGraphNode {
	return missionGraphNode{
		ID:            "ci-maintenance",
		Path:          "docs/definitions/choir-ci-optimization-2026-07-16.md",
		EntryPoint:    true,
		ExecutionMode: "scope_disjoint_maintenance",
		Status:        "working",
		Kind:          "ci_maintenance",
	}
}

func TestScanBrokenCurrentDocLinksFindsMissingTarget(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	if err := os.MkdirAll("docs", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("docs/current.md", []byte("[missing](gone.md)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	warnings := scanBrokenCurrentDocLinks([]string{"docs/current.md"}, map[string]*docInfo{
		"docs/current.md": {Path: "docs/current.md", Scope: "current", Manifested: true},
	})
	if len(warnings) != 1 || warnings[0].Rule != "R9" {
		t.Fatalf("scanBrokenCurrentDocLinks() = %#v, want one R9 warning", warnings)
	}
}
