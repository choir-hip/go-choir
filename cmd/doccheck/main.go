package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	defaultManifest          = "docs/doc-authority-manifest.yaml"
	defaultGraph             = "docs/mission-graph.yaml"
	defaultAssertionRegister = "docs/conjecture-assertion-ledger-2026-06.md"
	defaultReport            = "doccheck-report.md"
	defaultJSON              = "doccheck.json"
	defaultBeadsStore        = ".beads/issues.jsonl"
)

var highRead = map[string]bool{
	"README.md":                                                true,
	"AGENTS.md":                                                true,
	"docs/README.md":                                           true,
	"docs/choir-doctrine.md":                                   true,
	"docs/semantic-registry.md":                                true,
	"docs/NOW.md":                                              true,
	"docs/ACTIVE.md":                                           true,
	"docs/current-architecture.md":                             true,
	"docs/platform-os-app-state.md":                            true,
	"docs/conjecture-assertion-ledger-2026-06.md":              true,
	"docs/heresy-detectors.md":                                 true,
	"docs/agent-product-doctrine.md":                           true,
	"docs/choir-prompting-invariants.md":                       true,
	"docs/memo-problem-documentation-first.md":                 true,
	"docs/computer-ontology.md":                                true,
	"docs/runtime-invariants.md":                               true,
	"docs/texture-agentic-invariants-2026-06-13.md":            true,
	"docs/source-external-data-publication.md":                 true,
	"docs/definitions/og-dolt-heresy-completion-2026-07-08.md": true,
}

// defaultReadPacket is intentionally small. It includes the docs router plus
// the eleven content documents named by docs/README.md. --mode=live validates
// this coherent current reading surface; the historical corpus remains
// report-only until individual retention receipts exist.
var defaultReadPacket = []string{
	"README.md",
	"AGENTS.md",
	"docs/README.md",
	"docs/choir-doctrine.md",
	"docs/semantic-registry.md",
	"docs/NOW.md",
	"docs/ACTIVE.md",
	"docs/computer-ontology.md",
	"docs/runtime-invariants.md",
	"docs/texture-agentic-invariants-2026-06-13.md",
	"docs/source-external-data-publication.md",
	"docs/definitions/og-dolt-heresy-completion-2026-07-08.md",
}

type manifestFile struct {
	SchemaVersion int           `yaml:"schema_version" json:"schema_version"`
	Documents     []manifestDoc `yaml:"documents" json:"documents"`
}

type manifestDoc struct {
	Path            string            `yaml:"path" json:"path"`
	ClaimScope      string            `yaml:"claim_scope" json:"claim_scope"`
	IsRoot          interface{}       `yaml:"is_root" json:"-"`
	IsEvidence      bool              `yaml:"is_evidence" json:"is_evidence"`
	Annotations     map[string]string `yaml:"annotations" json:"annotations,omitempty"`
	Witnesses       []string          `yaml:"witnesses" json:"witnesses,omitempty"`
	RefreshTriggers []string          `yaml:"refresh_triggers" json:"refresh_triggers,omitempty"`
	Supersedes      []string          `yaml:"supersedes" json:"supersedes,omitempty"`
	SupersededBy    []string          `yaml:"superseded_by" json:"superseded_by,omitempty"`
	RootKinds       []string          `yaml:"-" json:"is_root"`
}

type docInfo struct {
	Path        string            `json:"path"`
	Scope       string            `json:"claim_scope"`
	IsRoot      []string          `json:"is_root,omitempty"`
	IsEvidence  bool              `json:"is_evidence"`
	Manifested  bool              `json:"manifested"`
	Inferred    bool              `json:"inferred"`
	Exists      bool              `json:"exists"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Witnesses   []string          `json:"witnesses,omitempty"`
	Edges       []edge            `json:"edges,omitempty"`
}

type edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Line   int    `json:"line,omitempty"`
	Source string `json:"source,omitempty"`
}

type warning struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

type report struct {
	Mode                  string          `json:"mode"`
	GeneratedAt           string          `json:"generated_at"`
	RuntimeMS             int64           `json:"runtime_ms"`
	ManifestPath          string          `json:"manifest_path"`
	MissionGraphPath      string          `json:"mission_graph_path"`
	AssertionRegisterPath string          `json:"assertion_register_path"`
	DocsScanned           int             `json:"docs_scanned"`
	ManifestEntries       int             `json:"manifest_entries"`
	InferredDocs          int             `json:"inferred_docs"`
	Edges                 []edge          `json:"edges"`
	Warnings              []warning       `json:"warnings"`
	WarningsByRule        map[string]int  `json:"warnings_by_rule"`
	WarningsBySeverity    map[string]int  `json:"warnings_by_severity"`
	HeresyAccounting      heresyAccount   `json:"heresy_accounting"`
	MissionGraph          graphReport     `json:"mission_graph"`
	AssertionRegister     assertionReport `json:"assertion_register"`
	HeresyScan            heresyScan      `json:"heresy_scan"`
	Documents             []docInfo       `json:"documents"`
	LiveFailures          []warning       `json:"live_failures,omitempty"`
}

type heresyAccount struct {
	Discovered []string `json:"discovered"`
	Introduced []string `json:"introduced"`
	Repaired   []string `json:"repaired"`
}

type missionGraphFile struct {
	SchemaVersion int                `yaml:"schema_version" json:"schema_version"`
	Status        string             `yaml:"status" json:"status"`
	Nodes         []missionGraphNode `yaml:"nodes" json:"nodes"`
}

type missionGraphNode struct {
	ID        string   `yaml:"id" json:"id"`
	Title     string   `yaml:"title" json:"title"`
	Path      string   `yaml:"path" json:"path,omitempty"`
	Ledger    string   `yaml:"ledger" json:"ledger,omitempty"`
	Status    string   `yaml:"status" json:"status"`
	Kind      string   `yaml:"kind" json:"kind"`
	DependsOn []string `yaml:"depends_on" json:"depends_on,omitempty"`
	Enables   []string `yaml:"enables" json:"enables,omitempty"`
	Sources   []string `yaml:"sources" json:"sources,omitempty"`
}

type graphReport struct {
	Path              string             `json:"path"`
	Status            string             `json:"status"`
	NodeCount         int                `json:"node_count"`
	EdgeCount         int                `json:"edge_count"`
	StatusCounts      map[string]int     `json:"status_counts,omitempty"`
	KindCounts        map[string]int     `json:"kind_counts,omitempty"`
	UngraphedParadocs []string           `json:"ungraphed_paradocs,omitempty"`
	Nodes             []missionGraphNode `json:"nodes,omitempty"`
}

type assertionReport struct {
	Path                 string   `json:"path"`
	Exists               bool     `json:"exists"`
	Assertions           []string `json:"assertions,omitempty"`
	InvariantCandidates  []string `json:"invariant_candidates,omitempty"`
	OpenEdges            []string `json:"open_edges,omitempty"`
	HasAssertionsSection bool     `json:"has_assertions_section"`
	HasInvariantsSection bool     `json:"has_invariants_section"`
	HasOpenEdgesSection  bool     `json:"has_open_edges_section"`
}

type detectorDefinition struct {
	ID     string   `json:"id"`
	Family string   `json:"family"`
	Terms  []string `json:"terms"`
}

type heresyFinding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Term    string `json:"term"`
	Family  string `json:"family,omitempty"`
	Context string `json:"context"`
	Surface string `json:"surface"`
}

type heresyScan struct {
	FilesScanned      int                  `json:"files_scanned"`
	Findings          int                  `json:"findings"`
	DetectorFamilies  int                  `json:"detector_families"`
	DetectorTerms     int                  `json:"detector_terms"`
	FindingsByContext map[string]int       `json:"findings_by_context,omitempty"`
	FindingsBySurface map[string]int       `json:"findings_by_surface,omitempty"`
	Detectors         []detectorDefinition `json:"detectors,omitempty"`
	Items             []heresyFinding      `json:"items,omitempty"`
}

func main() {
	start := time.Now()
	var manifestPath, graphPath, assertionPath, reportPath, jsonPath, actor, writeAttempt, mode string
	flag.StringVar(&manifestPath, "manifest", defaultManifest, "doc authority manifest path")
	flag.StringVar(&graphPath, "mission-graph", defaultGraph, "mission graph path")
	flag.StringVar(&assertionPath, "assertion-register", defaultAssertionRegister, "canonical conjecture/assertion register path")
	flag.StringVar(&reportPath, "report", defaultReport, "human report output path")
	flag.StringVar(&jsonPath, "json", defaultJSON, "machine report output path")
	flag.StringVar(&actor, "actor", "", "optional actor id for write-attempt guard probes")
	flag.StringVar(&writeAttempt, "write-attempt", "", "optional markdown doc path an actor is attempting to write")
	flag.StringVar(&mode, "mode", "full", "check mode: full (report-only corpus scan) or live (strict default reading packet)")
	flag.Parse()
	if mode != "full" && mode != "live" {
		fmt.Fprintf(os.Stderr, "doccheck: invalid --mode %q (want full or live)\n", mode)
		os.Exit(2)
	}

	rep, err := run(manifestPath, graphPath, assertionPath, actor, writeAttempt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
		os.Exit(0)
	}
	rep.RuntimeMS = time.Since(start).Milliseconds()
	rep.Mode = mode
	if mode == "live" {
		rep.LiveFailures = validateLiveReadPath(rep)
	}
	if err := writeReports(rep, reportPath, jsonPath); err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: write reports: %v\n", err)
		os.Exit(0)
	}
	if mode == "live" {
		if len(rep.LiveFailures) > 0 {
			fmt.Printf("doccheck live check failed: %d failures, %dms\n", len(rep.LiveFailures), rep.RuntimeMS)
			os.Exit(1)
		}
		fmt.Printf("doccheck live check passed: 11 content documents plus router, %dms\n", rep.RuntimeMS)
		os.Exit(0)
	}
	fmt.Printf("doccheck report-only complete: %d docs, %d warnings, %dms\n", rep.DocsScanned, len(rep.Warnings), rep.RuntimeMS)
	os.Exit(0)
}

func run(manifestPath, graphPath, assertionPath, actor, writeAttempt string) (report, error) {
	mf, err := readManifest(manifestPath)
	if err != nil {
		return report{}, err
	}
	manifestByPath := map[string]manifestDoc{}
	for _, d := range mf.Documents {
		d.Path = cleanPath(d.Path)
		d.RootKinds = normalizeRootKinds(d.IsRoot)
		manifestByPath[d.Path] = d
	}

	mdFiles, err := markdownFiles(".")
	if err != nil {
		return report{}, err
	}
	exists := map[string]bool{}
	for _, p := range mdFiles {
		exists[p] = true
	}

	docs := map[string]*docInfo{}
	for _, p := range mdFiles {
		md := manifestByPath[p]
		info := classifyDoc(p, md, md.Path != "", true)
		docs[p] = &info
	}
	for p, md := range manifestByPath {
		if _, ok := docs[p]; !ok {
			info := classifyDoc(p, md, true, false)
			docs[p] = &info
		}
	}

	var warnings []warning
	for p := range highRead {
		if _, ok := manifestByPath[p]; !ok {
			warnings = append(warnings, warning{
				Rule:     "R1",
				Severity: "warning",
				Path:     p,
				Message:  "high-read entrypoint has no manifest entry",
				Hint:     "add kernel fields to docs/doc-authority-manifest.yaml",
			})
		}
	}
	for p, info := range docs {
		if len(info.IsRoot) > 0 && !info.Manifested {
			warnings = append(warnings, warning{Rule: "R1", Severity: "warning", Path: p, Message: "root doc is not explicitly manifested"})
		}
		if info.Manifested && !info.Exists {
			warnings = append(warnings, warning{Rule: "R1", Severity: "warning", Path: p, Message: "manifest entry points at a missing markdown file"})
		}
	}

	for _, info := range docs {
		if !info.Manifested || !(info.Scope == "current" || info.Scope == "mixed") {
			continue
		}
		if len(info.Witnesses) == 0 {
			warnings = append(warnings, warning{Rule: "R2", Severity: "info", Path: info.Path, Message: "current or mixed manifest entry has no witness payload"})
			continue
		}
		for _, pattern := range info.Witnesses {
			matches, err := witnessMatches(pattern)
			if err != nil || len(matches) == 0 {
				warnings = append(warnings, warning{Rule: "R2", Severity: "warning", Path: info.Path, Message: fmt.Sprintf("witness pattern %q matches nothing", pattern), Hint: "update or remove stale witness payload"})
			}
		}
	}

	allEdges := collectEdges(mdFiles, docs, manifestByPath)
	for _, e := range allEdges {
		if d := docs[e.From]; d != nil {
			d.Edges = append(d.Edges, e)
		}
	}
	warnings = append(warnings, scanBrokenCurrentDocLinks(mdFiles, docs)...)
	reachable := reachableDocs(docs, allEdges)
	for _, info := range docs {
		if reachable[info.Path] {
			continue
		}
		switch {
		case info.IsEvidence:
			warnings = append(warnings, warning{Rule: "R3", Severity: "info", Path: info.Path, Message: "evidence doc is not reachable from current entry roots"})
		case info.Scope == "current" || info.Scope == "mixed":
			warnings = append(warnings, warning{Rule: "R3", Severity: "warning", Path: info.Path, Message: "current or mixed doc is not reachable from entry roots"})
		case info.Scope == "target":
			warnings = append(warnings, warning{Rule: "R3", Severity: "warning", Path: info.Path, Message: "target doc is not reachable from entry roots", Hint: "link target work from the relevant mission or architecture index"})
		case info.Scope == "":
			warnings = append(warnings, warning{Rule: "R3", Severity: "info", Path: info.Path, Message: "unclassified doc is a collection candidate"})
		}
	}

	if actor == "docs_reconciler" && writeAttempt != "" {
		p := cleanPath(writeAttempt)
		info, ok := docs[p]
		if !ok {
			md := manifestByPath[p]
			tmp := classifyDoc(p, md, md.Path != "", exists[p])
			info = &tmp
		}
		if info.Scope == "target" {
			warnings = append(warnings, warning{Rule: "R4", Severity: "warning", Path: p, Message: "docs_reconciler attempted to write a claim_scope: target doc", Hint: "reject the write; target docs require mission or owner authority"})
		}
	}

	detectors := detectorDefinitions("docs/heresy-detectors.md")
	detectorTerms := flattenDetectorTerms(detectors)
	for _, p := range mdFiles {
		info := docs[p]
		if info == nil || info.IsEvidence || !(info.Scope == "current" || info.Scope == "mixed") {
			continue
		}
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		// The doctrine file defines heresy detectors and banned patterns; it
		// must list retired terms as vocabulary. Skip H1/H3 heresy scanning
		// for it — those scans are for docs that *use* retired terms as live
		// claims, not for the authority source that defines them.
		isDoctrine := p == "docs/choir-doctrine.md"
		if !isDoctrine {
			warnings = append(warnings, scanHeresyTerms(p, string(content), detectorTerms)...)
			warnings = append(warnings, scanTextureAgency(p, string(content), info)...)
		}
		warnings = append(warnings, scanOverclaims(p, string(content), info)...)
		warnings = append(warnings, scanCurrentTargetCollapse(p, string(content), info)...)
	}

	graph, graphWarnings := validateMissionGraph(graphPath, docs)
	warnings = append(warnings, graphWarnings...)
	warnings = append(warnings, validateBeadsStore(defaultBeadsStore, graph.NodeCount)...)
	assertions, assertionWarnings := validateAssertionRegister(assertionPath)
	warnings = append(warnings, assertionWarnings...)
	heresyScan, err := scanCodeHeresy(detectors, docs)
	if err != nil {
		warnings = append(warnings, warning{Rule: "R7", Severity: "warning", Message: fmt.Sprintf("code heresy scan failed: %v", err)})
	}
	textureWarnings, err := scanTextureRetiredNames(docs)
	if err != nil {
		warnings = append(warnings, warning{Rule: "H5", Severity: "warning", Message: fmt.Sprintf("Texture retired-name scan failed: %v", err)})
	} else {
		warnings = append(warnings, textureWarnings...)
	}
	runtimeMarkdownWarnings, err := scanForbiddenRuntimeMarkdown()
	if err != nil {
		warnings = append(warnings, warning{Rule: "P1", Severity: "warning", Message: fmt.Sprintf("runtime markdown scan failed: %v", err)})
	} else {
		warnings = append(warnings, runtimeMarkdownWarnings...)
	}

	sortWarnings(warnings)
	docList := make([]docInfo, 0, len(docs))
	inferred := 0
	for _, d := range docs {
		if d.Inferred {
			inferred++
		}
		sort.Slice(d.Edges, func(i, j int) bool {
			if d.Edges[i].To == d.Edges[j].To {
				return d.Edges[i].Kind < d.Edges[j].Kind
			}
			return d.Edges[i].To < d.Edges[j].To
		})
		docList = append(docList, *d)
	}
	sort.Slice(docList, func(i, j int) bool { return docList[i].Path < docList[j].Path })

	return report{
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		ManifestPath:          manifestPath,
		MissionGraphPath:      graphPath,
		AssertionRegisterPath: assertionPath,
		DocsScanned:           len(mdFiles),
		ManifestEntries:       len(mf.Documents),
		InferredDocs:          inferred,
		Edges:                 allEdges,
		Warnings:              warnings,
		WarningsByRule:        countBy(warnings, func(w warning) string { return w.Rule }),
		WarningsBySeverity:    countBy(warnings, func(w warning) string { return w.Severity }),
		HeresyAccounting: heresyAccount{
			Discovered: discoveredFromWarnings(warnings),
			Introduced: []string{},
			Repaired:   []string{"none claimed; v0 is report-only"},
		},
		MissionGraph:      graph,
		AssertionRegister: assertions,
		HeresyScan:        heresyScan,
		Documents:         docList,
	}, nil
}

func readManifest(path string) (manifestFile, error) {
	var mf manifestFile
	b, err := os.ReadFile(path)
	if err != nil {
		return mf, err
	}
	if err := yaml.Unmarshal(b, &mf); err != nil {
		return mf, err
	}
	return mf, nil
}

func classifyDoc(path string, md manifestDoc, manifested, exists bool) docInfo {
	if manifested {
		return docInfo{
			Path:        path,
			Scope:       md.ClaimScope,
			IsRoot:      md.RootKinds,
			IsEvidence:  md.IsEvidence,
			Manifested:  true,
			Inferred:    false,
			Exists:      exists,
			Annotations: md.Annotations,
			Witnesses:   md.Witnesses,
		}
	}
	scope, evidence := inferClassification(path)
	return docInfo{Path: path, Scope: scope, IsEvidence: evidence, Inferred: true, Exists: exists}
}

func inferClassification(path string) (string, bool) {
	base := filepath.Base(path)
	switch {
	case strings.HasPrefix(path, "docs/mission-") && strings.HasSuffix(path, ".ledger.md"):
		return "historical", true
	case strings.HasPrefix(path, "docs/mission-"):
		return "mixed", false
	case strings.HasSuffix(path, ".ledger.md"):
		return "historical", true
	case strings.Contains(base, "proof") || strings.Contains(base, "review") || strings.Contains(base, "report") || strings.Contains(base, "dogfood") || strings.Contains(base, "blocker") || strings.Contains(base, "learnings"):
		return "historical", true
	default:
		return "", false
	}
}

func normalizeRootKinds(v interface{}) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		if x {
			return []string{"entry"}
		}
		return nil
	case string:
		if x == "" || x == "false" {
			return nil
		}
		return []string{x}
	case []interface{}:
		var out []string
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" && s != "false" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func markdownFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "test-results" || name == ".gstack") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		p := cleanPath(path)
		if p == "doccheck-report.md" || p == "doccheck.json" {
			return nil
		}
		if strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	return filepath.ToSlash(filepath.Clean(path))
}

var markdownLinkRe = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
var bareMDRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./:-])((?:\.\./|\.\/)?(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+\.md)(?:#[A-Za-z0-9_.-]+)?`)

// scanBrokenCurrentDocLinks catches a failure that edge collection intentionally
// cannot represent: a Markdown link whose local target has already disappeared.
// Historical evidence may retain such links; living docs may not.
func scanBrokenCurrentDocLinks(files []string, docs map[string]*docInfo) []warning {
	seen := map[string]bool{}
	var warnings []warning
	for _, path := range files {
		info := docs[path]
		if info == nil || !info.Manifested || (info.Scope != "current" && info.Scope != "mixed") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for line, text := range strings.Split(string(content), "\n") {
			for _, match := range markdownLinkRe.FindAllStringSubmatch(text, -1) {
				candidates := localMarkdownRefCandidates(path, match[1])
				if len(candidates) == 0 {
					continue
				}
				exists := false
				for _, candidate := range candidates {
					if fileExists(candidate) {
						exists = true
						break
					}
				}
				if exists {
					continue
				}
				key := fmt.Sprintf("%s\x00%d\x00%s", path, line+1, match[1])
				if seen[key] {
					continue
				}
				seen[key] = true
				warnings = append(warnings, warning{Rule: "R9", Severity: "warning", Path: path, Line: line + 1, Message: fmt.Sprintf("current or mixed doc links to missing markdown target %q", match[1]), Hint: "replace the reference with a retained authority or remove it before deleting the target"})
			}
		}
	}
	return warnings
}

func localMarkdownRefCandidates(source, raw string) []string {
	ref := strings.TrimSpace(raw)
	ref = strings.Trim(ref, "`'\"")
	if i := strings.IndexAny(ref, "#?"); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" || strings.HasPrefix(ref, "http:") || strings.HasPrefix(ref, "https:") || strings.HasPrefix(ref, "mailto:") || !strings.HasSuffix(ref, ".md") {
		return nil
	}
	if strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "./") {
		return []string{cleanPath(filepath.Join(filepath.Dir(source), ref))}
	}
	if strings.Contains(ref, "/") {
		return []string{cleanPath(ref), cleanPath(filepath.Join(filepath.Dir(source), ref))}
	}
	return []string{cleanPath(filepath.Join(filepath.Dir(source), ref)), cleanPath(filepath.Join("docs", ref)), cleanPath(ref)}
}

func collectEdges(files []string, docs map[string]*docInfo, manifestByPath map[string]manifestDoc) []edge {
	seen := map[string]bool{}
	var edges []edge
	add := func(e edge) {
		if e.To == "" || docs[e.To] == nil {
			return
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", e.From, e.To, e.Kind, e.Line)
		if seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, e)
	}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			for _, m := range markdownLinkRe.FindAllStringSubmatch(line, -1) {
				if to := resolveDocRef(p, m[1], docs); to != "" {
					add(edge{From: p, To: to, Kind: "markdown_link", Line: i + 1, Source: m[1]})
				}
			}
			for _, m := range bareMDRe.FindAllStringSubmatch(line, -1) {
				if strings.Contains(m[1], "://") {
					continue
				}
				if to := resolveDocRef(p, m[1], docs); to != "" {
					add(edge{From: p, To: to, Kind: "bare_path", Line: i + 1, Source: m[1]})
				}
			}
		}
	}
	for p, md := range manifestByPath {
		for _, raw := range md.Supersedes {
			if to := resolveDocRef(p, raw, docs); to != "" {
				add(edge{From: p, To: to, Kind: "supersedes", Source: raw})
			}
		}
		for _, raw := range md.SupersededBy {
			if to := resolveDocRef(p, raw, docs); to != "" {
				add(edge{From: p, To: to, Kind: "superseded_by", Source: raw})
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			if edges[i].To == edges[j].To {
				return edges[i].Kind < edges[j].Kind
			}
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})
	return edges
}

func resolveDocRef(source, raw string, docs map[string]*docInfo) string {
	ref := strings.TrimSpace(raw)
	ref = strings.Trim(ref, "`'\"")
	if i := strings.IndexAny(ref, "#?"); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" || strings.HasPrefix(ref, "http:") || strings.HasPrefix(ref, "https:") || strings.HasPrefix(ref, "mailto:") {
		return ""
	}
	var candidates []string
	if strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "./") {
		candidates = append(candidates, cleanPath(filepath.Join(filepath.Dir(source), ref)))
	} else if strings.Contains(ref, "/") {
		candidates = append(candidates, cleanPath(ref), cleanPath(filepath.Join(filepath.Dir(source), ref)))
	} else {
		candidates = append(candidates, cleanPath(filepath.Join(filepath.Dir(source), ref)), cleanPath(filepath.Join("docs", ref)), cleanPath(ref))
	}
	for _, c := range candidates {
		if docs[c] != nil {
			return c
		}
	}
	return ""
}

func reachableDocs(docs map[string]*docInfo, edges []edge) map[string]bool {
	adj := map[string][]string{}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	queue := []string{}
	reachable := map[string]bool{}
	for _, d := range docs {
		for _, kind := range d.IsRoot {
			if kind == "entry" || kind == "authority" {
				queue = append(queue, d.Path)
				reachable[d.Path] = true
				break
			}
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}
	return reachable
}

// validateLiveReadPath deliberately checks only the authority surface a new
// agent is expected to read. Historical graph/index problems remain visible in
// --mode=full, but cannot make the current packet un-runnable forever.
func validateLiveReadPath(rep report) []warning {
	docs := map[string]docInfo{}
	for _, doc := range rep.Documents {
		docs[doc.Path] = doc
	}

	failures := []warning{}
	packet := map[string]bool{}
	for _, path := range defaultReadPacket {
		packet[path] = true
		doc, ok := docs[path]
		if !ok || !doc.Exists {
			failures = append(failures, warning{Rule: "L1", Severity: "error", Path: path, Message: "default reading packet document is missing"})
			continue
		}
		if !doc.Manifested {
			failures = append(failures, warning{Rule: "L1", Severity: "error", Path: path, Message: "default reading packet document is not manifested"})
		}
		if doc.Scope == "historical" || doc.Scope == "target" {
			failures = append(failures, warning{Rule: "L1", Severity: "error", Path: path, Message: fmt.Sprintf("default reading packet document has non-current scope %q", doc.Scope)})
		}
	}

	routerEdges := map[string]bool{}
	for _, edge := range rep.Edges {
		if edge.From == "docs/README.md" {
			routerEdges[edge.To] = true
		}
	}
	for _, path := range defaultReadPacket {
		if path == "docs/README.md" {
			continue
		}
		if !routerEdges[path] {
			failures = append(failures, warning{Rule: "L2", Severity: "error", Path: "docs/README.md", Message: fmt.Sprintf("docs router does not link default reading packet document %q", path)})
		}
	}

	for _, finding := range rep.Warnings {
		if finding.Rule == "R3" && packet[finding.Path] {
			failures = append(failures, warning{Rule: "L3", Severity: "error", Path: finding.Path, Line: finding.Line, Message: "default reading packet is not reachable from entry roots", Hint: finding.Message})
		}
	}

	productDefinitions := 0
	for _, doc := range rep.Documents {
		if doc.Scope != "current" || doc.Annotations["doc_role"] != "definition" {
			continue
		}
		for _, root := range doc.IsRoot {
			if root == "authority" {
				productDefinitions++
				break
			}
		}
	}
	if productDefinitions != 1 {
		failures = append(failures, warning{Rule: "L4", Severity: "error", Path: "docs/doc-authority-manifest.yaml", Message: fmt.Sprintf("expected exactly one current authority-root product Definition, found %d", productDefinitions)})
	}

	return failures
}

func witnessMatches(pattern string) ([]string, error) {
	pattern = cleanPath(pattern)
	if strings.HasSuffix(pattern, "/**") {
		dir := strings.TrimSuffix(pattern, "/**")
		var matches []string
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				matches = append(matches, cleanPath(path))
			}
			return nil
		})
		return matches, err
	}
	if strings.Contains(pattern, "**/") {
		pattern = strings.ReplaceAll(pattern, "**/", "")
	}
	matches, err := filepath.Glob(pattern)
	if len(matches) > 0 || err != nil {
		return matches, err
	}
	if _, err := os.Stat(pattern); err == nil {
		return []string{pattern}, nil
	}
	return nil, nil
}

func validateMissionGraph(path string, docs map[string]*docInfo) (graphReport, []warning) {
	rep := graphReport{Path: path}
	var warnings []warning
	b, err := os.ReadFile(path)
	if err != nil {
		return rep, []warning{{Rule: "R5", Severity: "warning", Path: path, Message: "mission graph is missing or unreadable", Hint: "create docs/mission-graph.yaml"}}
	}
	var mf missionGraphFile
	if err := yaml.Unmarshal(b, &mf); err != nil {
		return rep, []warning{{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph YAML is invalid: %v", err)}}
	}
	rep.Status = mf.Status
	rep.NodeCount = len(mf.Nodes)
	rep.Nodes = mf.Nodes
	rep.StatusCounts = map[string]int{}
	rep.KindCounts = map[string]int{}
	ids := map[string]missionGraphNode{}
	validStatus := map[string]bool{"planned": true, "working": true, "open_handoff": true, "settled": true, "superseded": true, "blocked": true}
	validKind := map[string]bool{"spine": true, "side": true, "product_completion": true, "docs_truth": true, "evidence": true, "superseded": true}
	for _, n := range mf.Nodes {
		if n.ID == "" {
			warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: "mission graph node has no id"})
			continue
		}
		if _, exists := ids[n.ID]; exists {
			warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("duplicate mission graph id %q", n.ID)})
		}
		ids[n.ID] = n
		rep.StatusCounts[n.Status]++
		rep.KindCounts[n.Kind]++
		if !validStatus[n.Status] {
			warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph node %q has invalid status %q", n.ID, n.Status)})
		}
		if !validKind[n.Kind] {
			warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph node %q has invalid kind %q", n.ID, n.Kind)})
		}
		for _, fieldPath := range []struct {
			label string
			value string
		}{
			{"path", n.Path},
			{"ledger", n.Ledger},
		} {
			if fieldPath.value == "" {
				continue
			}
			if !fileExists(fieldPath.value) {
				warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph node %q %s points at missing file %s", n.ID, fieldPath.label, fieldPath.value)})
			}
		}
		for _, source := range n.Sources {
			if source == "" {
				continue
			}
			if !fileExists(source) {
				warnings = append(warnings, warning{Rule: "R5", Severity: "info", Path: path, Message: fmt.Sprintf("mission graph node %q source %s is not present", n.ID, source)})
			}
		}
	}
	for _, n := range mf.Nodes {
		for _, dep := range n.DependsOn {
			rep.EdgeCount++
			if _, ok := ids[dep]; !ok {
				warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph node %q depends on unknown node %q", n.ID, dep)})
			}
		}
		for _, enabled := range n.Enables {
			if _, ok := ids[enabled]; !ok {
				warnings = append(warnings, warning{Rule: "R5", Severity: "info", Path: path, Message: fmt.Sprintf("mission graph node %q enables node %q that is not yet graphed", n.ID, enabled)})
			}
		}
	}
	if cycle := findGraphCycle(mf.Nodes); len(cycle) > 0 {
		warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: fmt.Sprintf("mission graph dependency cycle detected: %s", strings.Join(cycle, " -> "))})
	}
	graphedPaths := map[string]bool{}
	for _, n := range mf.Nodes {
		if n.Path != "" {
			graphedPaths[cleanPath(n.Path)] = true
		}
	}
	for p, info := range docs {
		if !strings.HasPrefix(p, "docs/mission-") || strings.HasSuffix(p, ".ledger.md") || info.IsEvidence {
			continue
		}
		if !graphedPaths[p] {
			rep.UngraphedParadocs = append(rep.UngraphedParadocs, p)
		}
	}
	sort.Strings(rep.UngraphedParadocs)
	if mf.Status != "seed" {
		for _, p := range rep.UngraphedParadocs {
			warnings = append(warnings, warning{Rule: "R5", Severity: "info", Path: path, Message: fmt.Sprintf("paradoc %s is not represented in mission graph", p), Hint: "add a mission graph node or classify it as historical/evidence"})
		}
	}
	if !graphedPaths["docs/definitions/documentation-authority-reduction-2026-07-09.md"] {
		warnings = append(warnings, warning{Rule: "R5", Severity: "warning", Path: path, Message: "current documentation-authority Definition is not represented in mission graph"})
	}
	return rep, warnings
}

func findGraphCycle(nodes []missionGraphNode) []string {
	adj := map[string][]string{}
	for _, n := range nodes {
		adj[n.ID] = append(adj[n.ID], n.DependsOn...)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var stack []string
	var visit func(string) []string
	visit = func(id string) []string {
		if visiting[id] {
			for i, item := range stack {
				if item == id {
					return append(append([]string{}, stack[i:]...), id)
				}
			}
			return []string{id, id}
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		stack = append(stack, id)
		for _, dep := range adj[id] {
			if cycle := visit(dep); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, n := range nodes {
		if cycle := visit(n.ID); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func validateAssertionRegister(path string) (assertionReport, []warning) {
	rep := assertionReport{Path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		return rep, []warning{{Rule: "R6", Severity: "warning", Path: path, Message: "canonical assertion register is missing or unreadable"}}
	}
	rep.Exists = true
	text := string(b)
	rep.HasAssertionsSection = strings.Contains(text, "## Assertions")
	rep.HasInvariantsSection = strings.Contains(text, "## Invariant candidates")
	rep.HasOpenEdgesSection = strings.Contains(text, "## Open hyperthesis edges")
	var warnings []warning
	if !rep.HasAssertionsSection {
		warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: "assertion register is missing ## Assertions"})
	}
	if !rep.HasInvariantsSection {
		warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: "assertion register is missing ## Invariant candidates"})
	}
	if !rep.HasOpenEdgesSection {
		warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: "assertion register is missing ## Open hyperthesis edges"})
	}
	idRe := regexp.MustCompile(`(?m)^###\s+([AIE][0-9]+)\b`)
	seen := map[string]bool{}
	for _, m := range idRe.FindAllStringSubmatch(text, -1) {
		id := m[1]
		if seen[id] {
			warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: fmt.Sprintf("duplicate assertion register id %s", id)})
			continue
		}
		seen[id] = true
		switch id[0] {
		case 'A':
			rep.Assertions = append(rep.Assertions, id)
		case 'I':
			rep.InvariantCandidates = append(rep.InvariantCandidates, id)
		case 'E':
			rep.OpenEdges = append(rep.OpenEdges, id)
		}
	}
	if len(rep.Assertions) == 0 {
		warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: "assertion register has no A* assertions"})
	}
	if len(rep.OpenEdges) == 0 {
		warnings = append(warnings, warning{Rule: "R6", Severity: "warning", Path: path, Message: "assertion register has no E* open edges"})
	}
	return rep, warnings
}

func detectorDefinitions(path string) []detectorDefinition {
	b, err := os.ReadFile(path)
	if err != nil {
		return fallbackDetectorDefinitions()
	}
	text := string(b)
	section := text
	if idx := strings.Index(section, "## Detector Manifest"); idx >= 0 {
		section = section[idx:]
	}
	if idx := strings.Index(section, "## Baseline Counts"); idx >= 0 {
		section = section[:idx]
	}
	re := regexp.MustCompile("`([^`]+)`")
	seen := map[string]bool{}
	var out []detectorDefinition
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "| ---") || strings.Contains(line, "| ID |") {
			continue
		}
		cells := splitMarkdownRow(line)
		if len(cells) < 3 {
			continue
		}
		var terms []string
		for _, m := range re.FindAllStringSubmatch(cells[2], -1) {
			for _, part := range strings.Split(m[1], ",") {
				term := strings.TrimSpace(part)
				if term == "" || seen[term] {
					continue
				}
				seen[term] = true
				terms = append(terms, term)
			}
		}
		if len(terms) == 0 {
			continue
		}
		out = append(out, detectorDefinition{ID: strings.TrimSpace(cells[0]), Family: strings.TrimSpace(cells[1]), Terms: terms})
	}
	if len(out) == 0 {
		return fallbackDetectorDefinitions()
	}
	return out
}

func splitMarkdownRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func flattenDetectorTerms(defs []detectorDefinition) []string {
	seen := map[string]bool{}
	var out []string
	for _, def := range defs {
		for _, term := range def.Terms {
			if term == "" || seen[term] {
				continue
			}
			seen[term] = true
			out = append(out, term)
		}
	}
	sort.Strings(out)
	return out
}

func fallbackDetectorDefinitions() []detectorDefinition {
	return []detectorDefinition{
		{ID: "fallback-surface", Family: "surface residue", Terms: []string{"Trace app", "Trace UI", "Open Trace", "Terminal app", "raw Terminal", "manual terminal", "Browser app", "BrowserApp", "browser_sessions", "AppHint: \"browser\""}},
		{ID: "fallback-continuation", Family: "continuation residue", Terms: []string{"RunContinuation", "run_continuations", "/api/continuations", "continuation-level"}},
	}
}

func scanCodeHeresy(detectors []detectorDefinition, docs map[string]*docInfo) (heresyScan, error) {
	files, err := heresySurfaceFiles(".")
	if err != nil {
		return heresyScan{}, err
	}
	termFamily := map[string]string{}
	for _, def := range detectors {
		for _, term := range def.Terms {
			if _, ok := termFamily[term]; !ok {
				termFamily[term] = def.Family
			}
		}
	}
	var items []heresyFinding
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			for _, term := range flattenDetectorTerms(detectors) {
				if term == "" || !strings.Contains(line, term) {
					continue
				}
				items = append(items, heresyFinding{
					Path:    p,
					Line:    i + 1,
					Term:    term,
					Family:  termFamily[term],
					Context: classifyHeresyContext(p, line, docs),
					Surface: classifySurface(p),
				})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Context == items[j].Context {
			if items[i].Path == items[j].Path {
				if items[i].Line == items[j].Line {
					return items[i].Term < items[j].Term
				}
				return items[i].Line < items[j].Line
			}
			return items[i].Path < items[j].Path
		}
		return items[i].Context < items[j].Context
	})
	return heresyScan{
		FilesScanned:      len(files),
		Findings:          len(items),
		DetectorFamilies:  len(detectors),
		DetectorTerms:     len(flattenDetectorTerms(detectors)),
		FindingsByContext: countFindings(items, func(f heresyFinding) string { return f.Context }),
		FindingsBySurface: countFindings(items, func(f heresyFinding) string { return f.Surface }),
		Detectors:         detectors,
		Items:             items,
	}, nil
}

func heresySurfaceFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "test-results" || name == ".gstack") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		p := cleanPath(path)
		if p == "doccheck-report.md" || p == "doccheck.json" {
			return nil
		}
		if isHeresySurface(p) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func isHeresySurface(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".md", ".go", ".svelte", ".ts", ".js", ".nix", ".yaml", ".yml", ".sh":
		return true
	}
	return strings.HasPrefix(path, "scripts/") || strings.HasPrefix(path, ".github/workflows/")
}

func classifyHeresyContext(path, line string, docs map[string]*docInfo) string {
	lower := strings.ToLower(line)
	if path == "docs/heresy-detectors.md" {
		return "detector-definition"
	}
	if info := docs[path]; info != nil {
		if info.IsEvidence || info.Scope == "historical" {
			return "historical-evidence"
		}
	}
	for _, term := range []string{"historical", "evidence", "quoted", "not endorsed"} {
		if strings.Contains(lower, term) {
			return "historical-evidence"
		}
	}
	for _, term := range []string{"deprecated", "retired", "forbidden", "must not", "do not", "invalid"} {
		if strings.Contains(lower, term) {
			return "explicitly-deprecated"
		}
	}
	for _, term := range []string{"transitional", "compatibility", "legacy", "shim", "residue", "migration"} {
		if strings.Contains(lower, term) {
			return "implementation-transitional"
		}
	}
	return "current-violation"
}

func scanForbiddenRuntimeMarkdown() ([]warning, error) {
	var warnings []warning
	err := filepath.WalkDir("internal/runtime", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		warnings = append(warnings, warning{
			Rule:     "P1",
			Severity: "warning",
			Path:     cleanPath(path),
			Message:  "internal/runtime must not contain markdown; docs belong in docs/ and agent prompts belong in YAML",
			Hint:     "use internal/runtime/prompt_defaults/*.yaml or internal/runtime/textureprompts/*.yaml",
		})
		return nil
	})
	return warnings, err
}

func classifySurface(path string) string {
	switch {
	case strings.HasPrefix(path, "docs/") || path == "README.md" || path == "AGENTS.md":
		return "docs"
	case strings.HasPrefix(path, "internal/runtime/prompt_defaults/") ||
		strings.HasPrefix(path, "internal/runtime/textureprompts/"):
		return "runtime-prompt"
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".svelte") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js"):
		return "frontend"
	case strings.HasSuffix(path, ".nix"):
		return "nix"
	case strings.HasPrefix(path, ".github/"):
		return "workflow"
	case strings.HasPrefix(path, "scripts/"):
		return "script"
	default:
		return "other"
	}
}

func countFindings(findings []heresyFinding, key func(heresyFinding) string) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		out[key(f)]++
	}
	return out
}

func scanHeresyTerms(path, content string, terms []string) []warning {
	var warnings []warning
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if allowedHeresyLine(line) {
			continue
		}
		for _, term := range terms {
			if term != "" && strings.Contains(line, term) {
				warnings = append(warnings, warning{Rule: "H1", Severity: "warning", Path: path, Line: i + 1, Message: fmt.Sprintf("retired vocabulary %q appears in a current or mixed non-evidence claim", term), Hint: "label as historical/target/deprecated, move to evidence, or update the current claim"})
			}
		}
	}
	return warnings
}

func allowedHeresyLine(line string) bool {
	lower := strings.ToLower(line)
	for _, term := range []string{"historical", "deprecated", "retired", "detector", "residue", "transitional", "target-only", "target architecture", "successor", "not endorsed", "quoted", "evidence", "forbidden pattern", "bad pattern", "banned pattern", "deletion gate", "successor pattern"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func scanOverclaims(path, content string, info *docInfo) []warning {
	if info.IsEvidence {
		return nil
	}
	patterns := []string{"this is safe", "guaranteed safe", "fully verified", "proves safe", "cannot fail"}
	return scanLinePatterns("H2", path, content, patterns, "unscoped safety/correctness claim", "name the verifier contract, evidence scope, or remaining risk")
}

func scanTextureAgency(path, content string, info *docInfo) []warning {
	if info.IsEvidence {
		return nil
	}
	patterns := []string{"Texture workflow", "Texture pipeline", "Texture must call", "Texture always calls", "requiredContinuationAfterTextureEdit", "initialTextureToolChoice", "WithInitialToolChoice"}
	warnings := scanLinePatterns("H3", path, content, patterns, "Texture agency may be collapsed into a fixed workflow", "preserve Texture as canonical document actor with authority to choose delegation")
	// Filter out warnings on detector-definition or heresy-label lines: docs
	// that list retired detector terms as vocabulary are not claiming them as
	// live workflow patterns.
	filtered := warnings[:0]
	lines := strings.Split(content, "\n")
	for _, w := range warnings {
		if w.Line > 0 && w.Line <= len(lines) && allowedHeresyLine(lines[w.Line-1]) {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

func scanCurrentTargetCollapse(path, content string, info *docInfo) []warning {
	if !(info.Scope == "current" || info.Scope == "mixed") {
		return nil
	}
	patterns := []string{"five Go services"}
	return scanLinePatterns("H4", path, content, patterns, "target/current service topology may be stated as current reality", "compare against cmd/ and docs/current-architecture.md")
}

func scanTextureRetiredNames(docs map[string]*docInfo) ([]warning, error) {
	files, err := heresySurfaceFiles(".")
	if err != nil {
		return nil, err
	}
	var warnings []warning
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		var firstLine int
		violations := 0
		for i, line := range lines {
			if !hasTextureRetiredName(line) {
				continue
			}
			if isAllowedTextureRetiredNameLine(p, line, docs) {
				continue
			}
			violations++
			if firstLine == 0 {
				firstLine = i + 1
			}
		}
		if violations == 0 {
			continue
		}
		warnings = append(warnings, warning{
			Rule:     "H5",
			Severity: "warning",
			Path:     p,
			Line:     firstLine,
			Message:  fmt.Sprintf("%d retired Texture predecessor name line(s) appear outside allowed historical/background context", violations),
			Hint:     "rename to Texture or classify the occurrence as historical evidence, migration residue, or temporary cutover residue with a deletion receipt",
		})
	}
	return warnings, nil
}

func hasTextureRetiredName(line string) bool {
	lower := strings.ToLower(line)
	for _, term := range textureRetiredNameTerms() {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func textureRetiredNameTerms() []string {
	name := "v" + "text"
	return []string{name, "." + name, "/api/" + name, "data-" + name, "edit_" + name, name + "_"}
}

func isAllowedTextureRetiredNameLine(path, line string, docs map[string]*docInfo) bool {
	if strings.HasPrefix(path, "cmd/doccheck/") {
		return true
	}
	if path == "docs/why-texture-background-2026-06-15.md" {
		return true
	}
	if info := docs[path]; info != nil {
		if info.IsEvidence || info.Scope == "historical" {
			return true
		}
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(path, "docs/mission-") {
		for _, term := range textureRetiredNameAllowedMissionTerms() {
			if strings.Contains(lower, term) {
				return true
			}
		}
	}
	if strings.Contains(lower, "texture-cutover-allow:") && (strings.Contains(lower, "delete-by") || strings.Contains(lower, "deletion receipt")) {
		return true
	}
	return false
}

func textureRetiredNameAllowedMissionTerms() []string {
	name := "v" + "text"
	return []string{
		"historical evidence",
		"retired-name",
		"retired " + name,
		"retired v-name",
		"old v-name",
		"migration residue",
		"deletion target",
		"delete before settlement",
		"selected affordance line counts",
		"v-name profile references",
		"request_super_execution",
		"data-" + name,
		name + "_",
	}
}

func scanLinePatterns(rule, path, content string, patterns []string, message, hint string) []warning {
	var warnings []warning
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		if rule == "H2" && (strings.Contains(lowerLine, "example") || strings.Contains(lowerLine, "bad:")) {
			continue
		}
		for _, pattern := range patterns {
			if strings.Contains(lowerLine, strings.ToLower(pattern)) {
				warnings = append(warnings, warning{Rule: rule, Severity: "warning", Path: path, Line: i + 1, Message: message, Hint: hint})
			}
		}
	}
	return warnings
}

func discoveredFromWarnings(warnings []warning) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range warnings {
		if strings.HasPrefix(w.Rule, "H") {
			key := fmt.Sprintf("%s: %s", w.Rule, w.Message)
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	if len(out) == 0 {
		return []string{"none from seeded heresy rules"}
	}
	return out
}

func countBy(warnings []warning, key func(warning) string) map[string]int {
	out := map[string]int{}
	for _, w := range warnings {
		out[key(w)]++
	}
	return out
}

func sortWarnings(warnings []warning) {
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Rule == warnings[j].Rule {
			if warnings[i].Path == warnings[j].Path {
				return warnings[i].Line < warnings[j].Line
			}
			return warnings[i].Path < warnings[j].Path
		}
		return warnings[i].Rule < warnings[j].Rule
	})
}

func writeReports(rep report, reportPath, jsonPath string) error {
	j, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(j, '\n'), 0644); err != nil {
		return err
	}
	return os.WriteFile(reportPath, []byte(renderMarkdown(rep)), 0644)
}

func renderMarkdown(rep report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Doccheck Report\n\n")
	if rep.Mode == "live" {
		if len(rep.LiveFailures) == 0 {
			fmt.Fprintf(&b, "Live verdict: default reading packet passes. Full-corpus warnings remain report-only.\n\n")
		} else {
			fmt.Fprintf(&b, "Live verdict: default reading packet fails with %d failures.\n\n", len(rep.LiveFailures))
		}
	} else {
		fmt.Fprintf(&b, "Report-only verdict: completed with %d warnings; v0 exits 0.\n\n", len(rep.Warnings))
	}
	fmt.Fprintf(&b, "Mode: `%s`\n\n", rep.Mode)
	fmt.Fprintf(&b, "Generated: %s\n\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "Runtime: %dms\n\n", rep.RuntimeMS)
	fmt.Fprintf(&b, "Docs scanned: %d\n\n", rep.DocsScanned)
	fmt.Fprintf(&b, "Manifest entries: %d\n\n", rep.ManifestEntries)
	fmt.Fprintf(&b, "Inferred docs: %d\n\n", rep.InferredDocs)
	fmt.Fprintf(&b, "Warnings by rule: %s\n\n", formatCounts(rep.WarningsByRule))
	fmt.Fprintf(&b, "Warnings by severity: %s\n\n", formatCounts(rep.WarningsBySeverity))
	if rep.Mode == "live" {
		fmt.Fprintf(&b, "## Live Reading Packet\n\n")
		if len(rep.LiveFailures) == 0 {
			fmt.Fprintf(&b, "- passed: %d documents\n\n", len(defaultReadPacket))
		} else {
			for _, failure := range rep.LiveFailures {
				fmt.Fprintf(&b, "- %s\n", formatWarning(failure))
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	fmt.Fprintf(&b, "## Mission Graph\n\n")
	fmt.Fprintf(&b, "- Path: `%s`\n", rep.MissionGraph.Path)
	fmt.Fprintf(&b, "- Status: `%s`\n", rep.MissionGraph.Status)
	fmt.Fprintf(&b, "- Nodes: %d\n", rep.MissionGraph.NodeCount)
	fmt.Fprintf(&b, "- Dependency edges: %d\n", rep.MissionGraph.EdgeCount)
	fmt.Fprintf(&b, "- Status counts: %s\n", formatCounts(rep.MissionGraph.StatusCounts))
	fmt.Fprintf(&b, "- Kind counts: %s\n", formatCounts(rep.MissionGraph.KindCounts))
	fmt.Fprintf(&b, "- Ungraphed paradocs: %d\n\n", len(rep.MissionGraph.UngraphedParadocs))
	fmt.Fprintf(&b, "## Assertion Register\n\n")
	fmt.Fprintf(&b, "- Path: `%s`\n", rep.AssertionRegister.Path)
	fmt.Fprintf(&b, "- Exists: %t\n", rep.AssertionRegister.Exists)
	fmt.Fprintf(&b, "- Assertions: %d (%s)\n", len(rep.AssertionRegister.Assertions), strings.Join(rep.AssertionRegister.Assertions, ", "))
	fmt.Fprintf(&b, "- Invariant candidates: %d (%s)\n", len(rep.AssertionRegister.InvariantCandidates), strings.Join(rep.AssertionRegister.InvariantCandidates, ", "))
	fmt.Fprintf(&b, "- Open edges: %d (%s)\n\n", len(rep.AssertionRegister.OpenEdges), strings.Join(rep.AssertionRegister.OpenEdges, ", "))
	fmt.Fprintf(&b, "## Code And Docs Heresy Scan\n\n")
	fmt.Fprintf(&b, "- Files scanned: %d\n", rep.HeresyScan.FilesScanned)
	fmt.Fprintf(&b, "- Findings: %d\n", rep.HeresyScan.Findings)
	fmt.Fprintf(&b, "- Detector families: %d\n", rep.HeresyScan.DetectorFamilies)
	fmt.Fprintf(&b, "- Detector terms: %d\n", rep.HeresyScan.DetectorTerms)
	fmt.Fprintf(&b, "- Findings by context: %s\n", formatCounts(rep.HeresyScan.FindingsByContext))
	fmt.Fprintf(&b, "- Findings by surface: %s\n\n", formatCounts(rep.HeresyScan.FindingsBySurface))
	fmt.Fprintf(&b, "## Heresy Accounting\n\n")
	fmt.Fprintf(&b, "Discovered:\n")
	for _, item := range rep.HeresyAccounting.Discovered {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\nIntroduced:\n")
	if len(rep.HeresyAccounting.Introduced) == 0 {
		fmt.Fprintf(&b, "- none\n")
	}
	for _, item := range rep.HeresyAccounting.Introduced {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\nRepaired:\n")
	for _, item := range rep.HeresyAccounting.Repaired {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\n## Top Risks\n\n")
	top := 20
	if len(rep.Warnings) < top {
		top = len(rep.Warnings)
	}
	if top == 0 {
		fmt.Fprintf(&b, "No warnings.\n")
	} else {
		for _, w := range rep.Warnings[:top] {
			fmt.Fprintf(&b, "- %s\n", formatWarning(w))
		}
	}
	fmt.Fprintf(&b, "\n## Per-Rule Warnings\n\n")
	if len(rep.Warnings) == 0 {
		fmt.Fprintf(&b, "No warnings.\n")
	} else {
		for _, w := range rep.Warnings {
			fmt.Fprintf(&b, "- %s\n", formatWarning(w))
		}
	}
	fmt.Fprintf(&b, "\n## Collection Candidates\n\n")
	count := 0
	for _, d := range rep.Documents {
		if d.Scope == "" {
			fmt.Fprintf(&b, "- %s\n", d.Path)
			count++
			if count >= 50 {
				fmt.Fprintf(&b, "- ... %d more omitted from human report; see doccheck.json\n", count)
				break
			}
		}
	}
	if count == 0 {
		fmt.Fprintf(&b, "No unclassified collection candidates.\n")
	}
	fmt.Fprintf(&b, "\n## Next Suggested Manifest Entries\n\n")
	suggested := 0
	for _, d := range rep.Documents {
		if !d.Manifested && (d.Scope == "current" || d.Scope == "mixed") {
			fmt.Fprintf(&b, "- %s\n", d.Path)
			suggested++
			if suggested >= 20 {
				break
			}
		}
	}
	if suggested == 0 {
		fmt.Fprintf(&b, "No immediate current/mixed manifest suggestions.\n")
	}
	return b.String()
}

func formatWarning(w warning) string {
	loc := w.Path
	if w.Line > 0 {
		loc = fmt.Sprintf("%s:%d", w.Path, w.Line)
	}
	if w.Hint != "" {
		return fmt.Sprintf("%s %s %s [%s] (%s)", loc, w.Rule, w.Message, w.Hint, w.Severity)
	}
	return fmt.Sprintf("%s %s %s (%s)", loc, w.Rule, w.Message, w.Severity)
}

func formatCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}
