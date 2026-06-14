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
	defaultManifest = "docs/doc-authority-manifest.yaml"
	defaultReport   = "doccheck-report.md"
	defaultJSON     = "doccheck.json"
)

var highRead = map[string]bool{
	"README.md":                                true,
	"AGENTS.md":                                true,
	"docs/README.md":                           true,
	"docs/choir-doctrine.md":                   true,
	"docs/current-architecture.md":             true,
	"docs/platform-os-app-state.md":            true,
	"docs/mission-portfolio-2026-06-11.md":     true,
	"docs/runtime-invariants.md":               true,
	"docs/source-external-data-publication.md": true,
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
	GeneratedAt        string         `json:"generated_at"`
	RuntimeMS          int64          `json:"runtime_ms"`
	ManifestPath       string         `json:"manifest_path"`
	DocsScanned        int            `json:"docs_scanned"`
	ManifestEntries    int            `json:"manifest_entries"`
	InferredDocs       int            `json:"inferred_docs"`
	Edges              []edge         `json:"edges"`
	Warnings           []warning      `json:"warnings"`
	WarningsByRule     map[string]int `json:"warnings_by_rule"`
	WarningsBySeverity map[string]int `json:"warnings_by_severity"`
	HeresyAccounting   heresyAccount  `json:"heresy_accounting"`
	Documents          []docInfo      `json:"documents"`
}

type heresyAccount struct {
	Discovered []string `json:"discovered"`
	Introduced []string `json:"introduced"`
	Repaired   []string `json:"repaired"`
}

func main() {
	start := time.Now()
	var manifestPath, reportPath, jsonPath, actor, writeAttempt string
	flag.StringVar(&manifestPath, "manifest", defaultManifest, "doc authority manifest path")
	flag.StringVar(&reportPath, "report", defaultReport, "human report output path")
	flag.StringVar(&jsonPath, "json", defaultJSON, "machine report output path")
	flag.StringVar(&actor, "actor", "", "optional actor id for write-attempt guard probes")
	flag.StringVar(&writeAttempt, "write-attempt", "", "optional markdown doc path an actor is attempting to write")
	flag.Parse()

	rep, err := run(manifestPath, actor, writeAttempt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
		os.Exit(0)
	}
	rep.RuntimeMS = time.Since(start).Milliseconds()
	if err := writeReports(rep, reportPath, jsonPath); err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: write reports: %v\n", err)
		os.Exit(0)
	}
	fmt.Printf("doccheck report-only complete: %d docs, %d warnings, %dms\n", rep.DocsScanned, len(rep.Warnings), rep.RuntimeMS)
	os.Exit(0)
}

func run(manifestPath, actor, writeAttempt string) (report, error) {
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

	detectors := detectorTerms("docs/heresy-detectors.md")
	for _, p := range mdFiles {
		info := docs[p]
		if info == nil || info.IsEvidence || !(info.Scope == "current" || info.Scope == "mixed") {
			continue
		}
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		warnings = append(warnings, scanHeresyTerms(p, string(content), detectors)...)
		warnings = append(warnings, scanOverclaims(p, string(content), info)...)
		warnings = append(warnings, scanVTextAgency(p, string(content), info)...)
		warnings = append(warnings, scanCurrentTargetCollapse(p, string(content), info)...)
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
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		ManifestPath:       manifestPath,
		DocsScanned:        len(mdFiles),
		ManifestEntries:    len(mf.Documents),
		InferredDocs:       inferred,
		Edges:              allEdges,
		Warnings:           warnings,
		WarningsByRule:     countBy(warnings, func(w warning) string { return w.Rule }),
		WarningsBySeverity: countBy(warnings, func(w warning) string { return w.Severity }),
		HeresyAccounting: heresyAccount{
			Discovered: discoveredFromWarnings(warnings),
			Introduced: []string{},
			Repaired:   []string{"none claimed; v0 is report-only"},
		},
		Documents: docList,
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

func detectorTerms(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fallbackDetectorTerms()
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
	var out []string
	for _, m := range re.FindAllStringSubmatch(section, -1) {
		for _, part := range strings.Split(m[1], ",") {
			term := strings.TrimSpace(part)
			if term == "" || seen[term] {
				continue
			}
			seen[term] = true
			out = append(out, term)
		}
	}
	if len(out) == 0 {
		return fallbackDetectorTerms()
	}
	return out
}

func fallbackDetectorTerms() []string {
	return []string{"Trace app", "Trace UI", "Open Trace", "Terminal app", "raw Terminal", "manual terminal", "Browser app", "BrowserApp", "browser_sessions", "AppHint: \"browser\"", "RunContinuation", "run_continuations", "/api/continuations", "continuation-level"}
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
	for _, term := range []string{"historical", "deprecated", "retired", "detector", "residue", "transitional", "target-only", "target architecture", "successor", "not endorsed", "quoted", "evidence", "forbidden pattern"} {
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

func scanVTextAgency(path, content string, info *docInfo) []warning {
	if info.IsEvidence {
		return nil
	}
	patterns := []string{"VText workflow", "VText pipeline", "VText must call", "VText always calls", "requiredContinuationAfterVTextEdit", "initialVTextToolChoice"}
	return scanLinePatterns("H3", path, content, patterns, "VText agency may be collapsed into a fixed workflow", "preserve VText as canonical document actor with authority to choose delegation")
}

func scanCurrentTargetCollapse(path, content string, info *docInfo) []warning {
	if !(info.Scope == "current" || info.Scope == "mixed") {
		return nil
	}
	patterns := []string{"five Go services"}
	return scanLinePatterns("H4", path, content, patterns, "target/current service topology may be stated as current reality", "compare against cmd/ and docs/current-architecture.md")
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
	fmt.Fprintf(&b, "Report-only verdict: completed with %d warnings; v0 exits 0.\n\n", len(rep.Warnings))
	fmt.Fprintf(&b, "Generated: %s\n\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "Runtime: %dms\n\n", rep.RuntimeMS)
	fmt.Fprintf(&b, "Docs scanned: %d\n\n", rep.DocsScanned)
	fmt.Fprintf(&b, "Manifest entries: %d\n\n", rep.ManifestEntries)
	fmt.Fprintf(&b, "Inferred docs: %d\n\n", rep.InferredDocs)
	fmt.Fprintf(&b, "Warnings by rule: %s\n\n", formatCounts(rep.WarningsByRule))
	fmt.Fprintf(&b, "Warnings by severity: %s\n\n", formatCounts(rep.WarningsBySeverity))
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
