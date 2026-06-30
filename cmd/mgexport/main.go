// Command mgexport is the reverse path for the beads migration (C6).
//
// It reads the beads store (every epic carrying external-ref "mg:*") and
// regenerates a docs/mission-graph.yaml-shaped document. This is the
// fidelity experiment for conjecture C6: "beads can become the source of
// truth and mission-graph.yaml can be demoted to a GENERATED projection."
//
// mgexport reverses what cmd/mgimport stored:
//   - preamble  <- committed template preamble.yaml (schema_version,
//     description, the 5 governance rules, node_fields), re-emitted verbatim
//   - id        <- external-ref, "mg:" prefix stripped
//   - title     <- bead title
//   - path      <- parsed from description "Paradoc: ..." line
//   - ledger    <- parsed from description "Ledger: ..." line
//   - status    <- reverse C4 vocab map (status + labels + close-reason)
//   - kind      <- "kind:<kind>" label
//   - depends_on<- `bd dep list <id>` (blocked-by edges), mapped to node ids
//   - enables   <- RECONSTRUCTED as the exact inverse of depends_on (the
//     ratified canonical-edge policy; authored enables are NOT stored)
//   - sources   <- parsed from description "Sources:" block
//   - comments  <- per-node head/section blocks, pre-depends_on triage
//     verdicts + lineage rationale, and inline allow-directives, recovered
//     from the bead's --design field (captured by mgimport via yaml.v3)
//
// Output goes to /tmp/mission-graph.generated.yaml. It never touches the
// real file. Like mgimport this shells to `bd` (authoring tool, not CI).
//
// Because enables is derived as the exact inverse of depends_on, the
// authored enables edges that no depends_on mirrors are reconciled away.
// That reconciliation is ratified doctrine but MUST be audited: mgexport
// also writes /tmp/enables-reconciliation-audit.md listing every dropped and
// added enables edge so a human can review what changed.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/mgcomment"
	"gopkg.in/yaml.v2"
)

const externalRefPfx = "mg:"

//go:embed preamble.yaml
var preamble string

// bead is the subset of `bd list --json` / `bd dep list --json` we need.
type bead struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Design      string   `json:"design"`
	Status      string   `json:"status"`
	CloseReason string   `json:"close_reason"`
	CreatedAt   string   `json:"created_at"`
	ExternalRef string   `json:"external_ref"`
	Labels      []string `json:"labels"`
}

// node mirrors the mission-graph.yaml node schema and field order.
type node struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Path      string   `yaml:"path"`
	Ledger    string   `yaml:"ledger"`
	Status    string   `yaml:"status"`
	Kind      string   `yaml:"kind"`
	DependsOn []string `yaml:"depends_on"`
	Enables   []string `yaml:"enables"`
	Sources   []string `yaml:"sources"`
}

const (
	dest             = "/tmp/mission-graph.generated.yaml"
	auditPath        = "/tmp/enables-reconciliation-audit.md"
	missionGraphPath = "docs/mission-graph.yaml"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mgexport:", err)
		os.Exit(1)
	}
}

func run() error {
	beads, err := listBeads()
	if err != nil {
		return err
	}

	// Index mission beads by bead-id -> node-id, and collect them.
	idToNode := make(map[string]string)
	comments := make(map[string]mgcomment.NodeComments)
	var mg []bead
	for _, b := range beads {
		if strings.HasPrefix(b.ExternalRef, externalRefPfx) {
			nid := strings.TrimPrefix(b.ExternalRef, externalRefPfx)
			idToNode[b.ID] = nid
			comments[nid] = mgcomment.Decode(b.Design)
			mg = append(mg, b)
		}
	}

	// Stable order: by the authored node index captured at import time (beads
	// owns the projection order). Fall back to created_at then node id when an
	// index is absent (e.g. a bead imported before index capture existed).
	idxOf := func(b bead) int {
		if nc, ok := comments[idToNode[b.ID]]; ok && nc.Index >= 0 {
			return nc.Index
		}
		return 1 << 30
	}
	sort.SliceStable(mg, func(i, j int) bool {
		ii, jj := idxOf(mg[i]), idxOf(mg[j])
		if ii != jj {
			return ii < jj
		}
		if mg[i].CreatedAt != mg[j].CreatedAt {
			return mg[i].CreatedAt < mg[j].CreatedAt
		}
		return idToNode[mg[i].ID] < idToNode[mg[j].ID]
	})

	// First pass: build nodes + depends_on. Accumulate inverse for enables.
	enables := make(map[string][]string)
	nodes := make([]node, 0, len(mg))
	for _, b := range mg {
		nid := idToNode[b.ID]
		path, ledger, sources := parseDescription(b.Description)
		deps, err := dependsOn(b.ID, idToNode)
		if err != nil {
			return err
		}
		deps = reorderDeps(deps, comments[nid].DepOrder)
		for _, d := range deps {
			enables[d] = append(enables[d], nid)
		}
		nodes = append(nodes, node{
			ID:        nid,
			Title:     b.Title,
			Path:      path,
			Ledger:    ledger,
			Status:    reverseStatus(b),
			Kind:      kindFromLabels(b.Labels),
			DependsOn: deps,
			Sources:   sources,
		})
	}

	// Second pass: attach reconstructed enables (exact inverse of
	// depends_on), in the same created_at order the nodes were emitted.
	order := make(map[string]int, len(nodes))
	for i, n := range nodes {
		order[n.ID] = i
	}
	for i := range nodes {
		e := enables[nodes[i].ID]
		sort.SliceStable(e, func(a, b int) bool { return order[e[a]] < order[e[b]] })
		nodes[i].Enables = e
	}

	// Emit: committed preamble (incl. the 5 rules) verbatim, then nodes with
	// their recovered comment prose.
	var buf bytes.Buffer
	buf.WriteString(preamble)
	if !strings.HasSuffix(preamble, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString("nodes:\n")
	for _, n := range nodes {
		block, err := emitNode(n, comments[n.ID])
		if err != nil {
			return err
		}
		buf.WriteString(block)
	}

	if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	fmt.Printf("wrote %s: %d nodes\n", dest, len(nodes))

	dropped, added, err := writeEnablesAudit()
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s: enables reconciliation dropped=%d added=%d\n", auditPath, dropped, added)
	return nil
}

// emitNode serializes one node to mission-graph.yaml form, re-injecting the
// recovered comment prose: the head/section block before "- id:", the
// pre-depends_on triage/lineage block, and inline allow-directives on the
// id/title/path/ledger lines and sources items. Field formatting comes from
// yaml.v2 (the marshaller the hand-authored file matches), with empty scalars
// normalized to single-quote style ('' not "") to match the source file.
func emitNode(n node, nc mgcomment.NodeComments) (string, error) {
	raw, err := yaml.Marshal([]node{n})
	if err != nil {
		return "", fmt.Errorf("marshal node %s: %w", n.ID, err)
	}
	text := strings.ReplaceAll(string(raw), `: ""`, `: ''`)
	lines := unwrap(strings.Split(strings.TrimRight(text, "\n"), "\n"))

	var b strings.Builder
	if nc.Head != "" {
		for _, h := range strings.Split(nc.Head, "\n") {
			b.WriteString(h)
			b.WriteString("\n")
		}
	}

	curKey := ""
	for _, ln := range lines {
		trimmed := strings.TrimLeft(ln, " ")

		switch {
		case strings.HasPrefix(ln, "- id:"):
			curKey = "id"
			ln = appendInline(ln, nc.InlineFor("id"))

		case strings.HasPrefix(ln, "  ") && strings.HasPrefix(trimmed, "- "):
			// sequence item under the most recent key
			val := strings.TrimPrefix(trimmed, "- ")
			switch curKey {
			case "sources":
				ln = appendInline(ln, nc.InlineFor("src:"+val))
			case "depends_on":
				ln = appendInline(ln, nc.InlineFor("dep:"+val))
			}

		case strings.HasPrefix(ln, "  ") && strings.Contains(trimmed, ":"):
			key := trimmed[:strings.IndexByte(trimmed, ':')]
			curKey = key
			if key == "depends_on" && nc.Predep != "" {
				for _, p := range strings.Split(nc.Predep, "\n") {
					b.WriteString("  ")
					b.WriteString(p)
					b.WriteString("\n")
				}
			}
			switch key {
			case "title", "path", "ledger":
				ln = appendInline(ln, nc.InlineFor(key))
			}
		}

		b.WriteString(ln)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// unwrap rejoins yaml.v2 line-wrapped scalar continuations. yaml.v2 folds long
// quoted scalars (the long single-quoted titles) at ~80 columns, emitting the
// continuation indented by 4 spaces. Node fields and list items never use a
// 4-space indent, so any 4-space line is a wrap continuation: rejoin it onto
// the previous logical line with a single space (the fold semantics).
func unwrap(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(ln, "    ") && len(out) > 0 {
			out[len(out)-1] += " " + strings.TrimLeft(ln, " ")
			continue
		}
		out = append(out, ln)
	}
	return out
}

func appendInline(line, comment string) string {
	if comment == "" {
		return line
	}
	return line + " " + comment
}

// reverseStatus inverts the C4 vocab freeze that mgimport applied.
func reverseStatus(b bead) string {
	has := func(l string) bool {
		for _, x := range b.Labels {
			if x == l {
				return true
			}
		}
		return false
	}
	switch b.Status {
	case "open":
		switch {
		case has("blocked"):
			return "blocked"
		default:
			return "planned"
		}
	case "in_progress":
		if has("handoff") {
			return "open_handoff"
		}
		return "working"
	case "closed":
		if has("superseded") || strings.HasPrefix(b.CloseReason, "superseded") {
			return "superseded"
		}
		return "settled"
	}
	return b.Status
}

func kindFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, "kind:") {
			return strings.TrimPrefix(l, "kind:")
		}
	}
	return ""
}

// parseDescription extracts path/ledger/sources from the mgimport body.
func parseDescription(d string) (path, ledger string, sources []string) {
	inSources := false
	for _, line := range strings.Split(d, "\n") {
		switch {
		case strings.HasPrefix(line, "Paradoc: "):
			path = strings.TrimPrefix(line, "Paradoc: ")
		case strings.HasPrefix(line, "Ledger: "):
			ledger = strings.TrimPrefix(line, "Ledger: ")
		case strings.TrimSpace(line) == "Sources:":
			inSources = true
		case inSources && strings.HasPrefix(strings.TrimSpace(line), "- "):
			sources = append(sources, strings.TrimPrefix(strings.TrimSpace(line), "- "))
		default:
			if inSources && strings.TrimSpace(line) != "" {
				inSources = false
			}
		}
	}
	return path, ledger, sources
}

// reorderDeps returns deps reordered to match the authored depends_on order
// captured at import time. Any dep not in want (should not happen) is appended
// in its original order, so no edge is dropped.
func reorderDeps(deps, want []string) []string {
	if len(want) == 0 {
		return deps
	}
	have := make(map[string]bool, len(deps))
	for _, d := range deps {
		have[d] = true
	}
	out := make([]string, 0, len(deps))
	placed := make(map[string]bool, len(deps))
	for _, w := range want {
		if have[w] && !placed[w] {
			out = append(out, w)
			placed[w] = true
		}
	}
	for _, d := range deps {
		if !placed[d] {
			out = append(out, d)
		}
	}
	return out
}

// dependsOn returns the node ids this bead is blocked-by (its depends_on).
func dependsOn(id string, idToNode map[string]string) ([]string, error) {
	out, err := bdOutput("dep", "list", id, "--json")
	if err != nil {
		return nil, err
	}
	var recs []bead
	if err := json.Unmarshal(out, &recs); err != nil {
		return nil, fmt.Errorf("parse dep list %s: %w", id, err)
	}
	var deps []string
	for _, r := range recs {
		if nid, ok := idToNode[r.ID]; ok {
			deps = append(deps, nid)
		}
	}
	return deps, nil
}

// --- enables reconciliation audit ---

type auditNode struct {
	ID        string   `yaml:"id"`
	DependsOn []string `yaml:"depends_on"`
	Enables   []string `yaml:"enables"`
}

type auditFile struct {
	Nodes []auditNode `yaml:"nodes"`
}

type edge struct{ from, to string }

// writeEnablesAudit compares the authored enables edges in
// docs/mission-graph.yaml against the depends_on-inverse policy and records
// every edge the policy DROPS (authored, no mirroring depends_on) and ADDS
// (a depends_on inverse that was never authored as enables).
func writeEnablesAudit() (dropped, added int, err error) {
	raw, err := os.ReadFile(missionGraphPath)
	if err != nil {
		return 0, 0, fmt.Errorf("read %s: %w", missionGraphPath, err)
	}
	var af auditFile
	if err := yaml.Unmarshal(raw, &af); err != nil {
		return 0, 0, fmt.Errorf("parse %s: %w", missionGraphPath, err)
	}

	authored := map[edge]bool{}
	inverse := map[edge]bool{}
	for _, n := range af.Nodes {
		for _, e := range n.Enables {
			authored[edge{n.ID, e}] = true
		}
		for _, d := range n.DependsOn {
			inverse[edge{d, n.ID}] = true // d enables n.ID
		}
	}

	var droppedEdges, addedEdges []edge
	for e := range authored {
		if !inverse[e] {
			droppedEdges = append(droppedEdges, e)
		}
	}
	for e := range inverse {
		if !authored[e] {
			addedEdges = append(addedEdges, e)
		}
	}
	sortEdges(droppedEdges)
	sortEdges(addedEdges)

	var b strings.Builder
	fmt.Fprintf(&b, "# Enables Reconciliation Audit (C6)\n\n")
	fmt.Fprintf(&b, "Ratified policy: `depends_on` is the canonical edge; `enables` is derived\n")
	fmt.Fprintf(&b, "as the exact inverse of `depends_on`. This audit lists the authoring drift\n")
	fmt.Fprintf(&b, "the policy reconciles when `docs/mission-graph.yaml` is regenerated from beads.\n\n")
	fmt.Fprintf(&b, "- DROPPED (authored `enables` with no mirroring `depends_on`): **%d**\n", len(droppedEdges))
	fmt.Fprintf(&b, "- ADDED (inverse of `depends_on` never authored as `enables`): **%d**\n\n", len(addedEdges))

	fmt.Fprintf(&b, "## DROPPED enables edges (%d)\n\n", len(droppedEdges))
	fmt.Fprintf(&b, "These authored `enables` edges disappear because no node declares the\n")
	fmt.Fprintf(&b, "reciprocal `depends_on`. Review: either the edge was authoring noise, or the\n")
	fmt.Fprintf(&b, "missing `depends_on` should be added to the canonical side.\n\n")
	for _, e := range droppedEdges {
		fmt.Fprintf(&b, "- `%s` -> `%s`\n", e.from, e.to)
	}

	fmt.Fprintf(&b, "\n## ADDED enables edges (%d)\n\n", len(addedEdges))
	fmt.Fprintf(&b, "These `enables` edges are invented by inverting an authored `depends_on`\n")
	fmt.Fprintf(&b, "that had no reciprocal authored `enables`. They are correct by policy.\n\n")
	for _, e := range addedEdges {
		fmt.Fprintf(&b, "- `%s` -> `%s`\n", e.from, e.to)
	}
	b.WriteString("\n")

	if err := os.WriteFile(auditPath, []byte(b.String()), 0o644); err != nil {
		return 0, 0, fmt.Errorf("write %s: %w", auditPath, err)
	}
	return len(droppedEdges), len(addedEdges), nil
}

func sortEdges(es []edge) {
	sort.Slice(es, func(i, j int) bool {
		if es[i].from != es[j].from {
			return es[i].from < es[j].from
		}
		return es[i].to < es[j].to
	})
}

func listBeads() ([]bead, error) {
	out, err := bdOutput("list", "--all", "-n", "0", "--json")
	if err != nil {
		return nil, err
	}
	var beads []bead
	if err := json.Unmarshal(out, &beads); err != nil {
		return nil, fmt.Errorf("parse bd list json: %w", err)
	}
	return beads, nil
}

func bdOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bd %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
