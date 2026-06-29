// Command mgimport is the shadow-mode importer for the beads migration (C5).
//
// It mirrors every node of docs/mission-graph.yaml into the beads store as a
// top-level epic (1:1), preserving dependency edges. Beads is derived from the
// YAML and the import is fully reversible. The importer is idempotent: running
// it twice yields the same state with no duplicate epics or edges.
//
// Idempotency key: each epic carries external-ref "mg:<node.id>". Before
// creating, the importer indexes existing beads by external-ref and skips any
// node already present. Dependency edges are likewise skipped if already wired.
//
// This is an authoring tool, not the CI checker: it shells out to the `bd`
// binary (os/exec), so the doccheck hermeticity rule does not apply here.
//
// Special case: node beads-mission-state-v0 corresponds to the EXISTING epic
// choir-pfg (this migration itself). Instead of creating a new epic, the
// importer stamps choir-pfg with external-ref mg:beads-mission-state-v0 and
// treats it as that node for dependency edges.
//
// NOTE: the importer does NOT create conjecture sub-issues (that is P3/C7).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/mgcomment"
	"gopkg.in/yaml.v2"
)

// missionGraphNode mirrors the schema parsed in cmd/doccheck/main.go.
type missionGraphNode struct {
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

type missionGraphFile struct {
	SchemaVersion int                `yaml:"schema_version"`
	Status        string             `yaml:"status"`
	Nodes         []missionGraphNode `yaml:"nodes"`
}

// bead is the subset of `bd list --json` we need.
type bead struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ExternalRef string   `json:"external_ref"`
	Status      string   `json:"status"`
	Design      string   `json:"design"`
	Labels      []string `json:"labels"`
}

// depRecord is the subset of `bd dep list --json` we need. The command returns
// the dependency issues directly as bead objects, so the dependency's bead id
// is in the `id` field.
type depRecord struct {
	ID string `json:"id"`
}

const (
	missionGraphPath = "docs/mission-graph.yaml"
	// existingBead is the pre-existing epic for node beads-mission-state-v0.
	existingBead     = "choir-pfg"
	existingNodeID   = "beads-mission-state-v0"
	externalRefPfx   = "mg:"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mgimport:", err)
		os.Exit(1)
	}
}

func run() error {
	raw, err := os.ReadFile(missionGraphPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", missionGraphPath, err)
	}
	var graph missionGraphFile
	if err := yaml.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("parse %s: %w", missionGraphPath, err)
	}

	existing, err := indexExistingBeads()
	if err != nil {
		return err
	}

	// nodeBead maps mission node id -> resolved bead id (for edge wiring).
	nodeBead := make(map[string]string, len(graph.Nodes))

	var created, skipped, closed int

	for _, n := range graph.Nodes {
		ref := externalRefPfx + n.ID

		// Special case: the migration epic itself.
		if n.ID == existingNodeID {
			if err := ensureExistingEpic(existing, n); err != nil {
				return err
			}
			nodeBead[n.ID] = existingBead
			skipped++
			continue
		}

		if b, ok := existing[ref]; ok {
			// Already present: converge labels + the projected body fields so
			// edits to the mission graph (e.g. a path added to a node after the
			// original import) round-trip back through mgexport. Status itself
			// is applied only on creation; close/in_progress already persisted.
			if err := ensureLabels(b.ID, b.Labels, desiredLabels(n)...); err != nil {
				return err
			}
			if err := ensureBody(b, n); err != nil {
				return err
			}
			nodeBead[n.ID] = b.ID
			skipped++
			continue
		}

		id, didClose, err := createEpic(n, ref)
		if err != nil {
			return err
		}
		nodeBead[n.ID] = id
		created++
		if didClose {
			closed++
		}
	}

	edgesAdded, edgesSkipped, err := wireDeps(graph.Nodes, nodeBead)
	if err != nil {
		return err
	}

	commentsUpdated, err := captureComments(raw, nodeBead)
	if err != nil {
		return err
	}

	fmt.Printf("nodes=%d created=%d skipped=%d closed_on_create=%d edges_added=%d edges_skipped=%d comments_updated=%d\n",
		len(graph.Nodes), created, skipped, closed, edgesAdded, edgesSkipped, commentsUpdated)
	return nil
}

// captureComments recovers the per-node comment prose (head/section blocks,
// pre-depends_on triage verdicts and lineage rationale, and inline
// allow-directives) that yaml.v2 discards, and stores each node's payload in
// its bead's --design field. Idempotent: a bead is only updated when its
// recovered design blob differs from what is already stored, so re-runs
// converge to a fixed point with zero updates.
func captureComments(raw []byte, nodeBead map[string]string) (int, error) {
	perNode, err := mgcomment.Parse(raw)
	if err != nil {
		return 0, err
	}

	// Current design per bead id, for the idempotency comparison.
	beads, err := listBeadsFull()
	if err != nil {
		return 0, err
	}
	curDesign := make(map[string]string, len(beads))
	for _, b := range beads {
		curDesign[b.ID] = b.Design
	}

	updated := 0
	for _, nid := range mgcomment.SortedIDs(perNode) {
		beadID, ok := nodeBead[nid]
		if !ok {
			// Node has no mirrored bead (should not happen); skip rather than fail.
			continue
		}
		desired := perNode[nid].Encode()
		if desired == "" {
			// Nothing to capture for this node.
			continue
		}
		if curDesign[beadID] == desired {
			continue // already converged
		}
		if err := bdRun("update", beadID, "--design", desired); err != nil {
			return updated, fmt.Errorf("capture comments for %s (%s): %w", nid, beadID, err)
		}
		updated++
	}
	return updated, nil
}

// listBeadsFull returns every bead (including the design field).
func listBeadsFull() ([]bead, error) {
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

// indexExistingBeads returns a map of external-ref -> bead for every bead
// (including closed ones, so re-runs see settled/superseded nodes).
func indexExistingBeads() (map[string]bead, error) {
	out, err := bdOutput("list", "--all", "-n", "0", "--json")
	if err != nil {
		return nil, err
	}
	var beads []bead
	if err := json.Unmarshal(out, &beads); err != nil {
		return nil, fmt.Errorf("parse bd list json: %w", err)
	}
	idx := make(map[string]bead, len(beads))
	for _, b := range beads {
		if b.ExternalRef != "" {
			idx[b.ExternalRef] = b
		}
		// Also index by id for the existing-epic special case.
		idx["id:"+b.ID] = b
	}
	return idx, nil
}

// ensureExistingEpic stamps choir-pfg with the migration node's external-ref
// and ensures it carries the kind:spine + mission labels. Idempotent.
func ensureExistingEpic(existing map[string]bead, n missionGraphNode) error {
	b, ok := existing["id:"+existingBead]
	if !ok {
		return fmt.Errorf("expected existing epic %s not found", existingBead)
	}
	if b.ExternalRef != externalRefPfx+existingNodeID {
		if err := bdRun("update", existingBead, "--external-ref", externalRefPfx+existingNodeID); err != nil {
			return err
		}
	}
	// node beads-mission-state-v0 is kind:spine, status planned -> stays open.
	if err := ensureLabels(existingBead, b.Labels, "mission", "kind:spine"); err != nil {
		return err
	}
	// Converge title/description so the migration node (aliased onto the
	// pre-existing choir-pfg epic) round-trips through mgexport like any other.
	if err := ensureBody(b, n); err != nil {
		return err
	}
	return nil
}

// desiredLabels returns the full label set a node's epic must carry: the
// kind:<kind> label, the bare mission label, and any status-derived label
// (handoff / blocked / superseded) from the C4 vocab freeze.
func desiredLabels(n missionGraphNode) []string {
	labels := []string{"mission", "kind:" + n.Kind}
	switch n.Status {
	case "open_handoff":
		labels = append(labels, "handoff")
	case "blocked":
		labels = append(labels, "blocked")
	case "superseded":
		labels = append(labels, "superseded")
	}
	return labels
}

// createEpic creates one epic for node n and applies the status mapping.
// Returns the new bead id and whether it was closed on creation.
func createEpic(n missionGraphNode, ref string) (string, bool, error) {
	args := []string{
		"create", n.Title,
		"-t", "epic",
		"-p", "2",
		"--external-ref", ref,
		"-l", strings.Join(desiredLabels(n), ","),
		"-d", description(n),
		"--silent",
	}
	out, err := bdOutput(args...)
	if err != nil {
		return "", false, err
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", false, fmt.Errorf("create %s: empty id returned", n.ID)
	}

	// Status transition (C4 vocab freeze). Labels are already set above.
	switch n.Status {
	case "planned", "blocked":
		// leave open (blocked carries only the label)
	case "working", "open_handoff":
		if err := bdRun("update", id, "--status", "in_progress"); err != nil {
			return id, false, err
		}
	case "settled":
		if err := bdRun("close", id, "-r", "settled: imported from mission-graph"); err != nil {
			return id, false, err
		}
		return id, true, nil
	case "superseded":
		if err := bdRun("close", id, "-r", "superseded: imported from mission-graph"); err != nil {
			return id, false, err
		}
		return id, true, nil
	default:
		return id, false, fmt.Errorf("node %s: unknown status %q", n.ID, n.Status)
	}
	return id, false, nil
}

// description packs title/path/ledger/sources into the epic body so the
// paradoc and ledger links are preserved in the shadow store.
func description(n missionGraphNode) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Mission: %s\n", n.Title)
	fmt.Fprintf(&b, "Mission node: %s (kind=%s, status=%s)\n", n.ID, n.Kind, n.Status)
	if n.Path != "" {
		fmt.Fprintf(&b, "Paradoc: %s\n", n.Path)
	}
	if n.Ledger != "" {
		fmt.Fprintf(&b, "Ledger: %s\n", n.Ledger)
	}
	if len(n.Sources) > 0 {
		b.WriteString("Sources:\n")
		for _, s := range n.Sources {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	b.WriteString("\nImported from docs/mission-graph.yaml (shadow mode, reversible).")
	return b.String()
}

// ensureBody converges an existing bead's title and description to the values
// projected from mission node n. Idempotent: only issues bd updates when a
// field actually differs. This keeps path/ledger/sources/title round-tripping
// when the mission graph is edited after the original import.
func ensureBody(b bead, n missionGraphNode) error {
	if b.Title != n.Title {
		if err := bdRun("update", b.ID, "--title", n.Title); err != nil {
			return err
		}
	}
	if want := description(n); b.Description != want {
		if err := bdRun("update", b.ID, "--description", want); err != nil {
			return err
		}
	}
	return nil
}

// ensureLabels adds any of want not already present in have. Idempotent.
func ensureLabels(id string, have []string, want ...string) error {
	set := make(map[string]bool, len(have))
	for _, l := range have {
		set[l] = true
	}
	for _, w := range want {
		if !set[w] {
			// Syntax: bd label add <issue-id> <label>.
			if err := bdRun("label", "add", id, w); err != nil {
				return err
			}
		}
	}
	return nil
}

// wireDeps adds blocked-by edges for every depends_on, skipping existing ones.
func wireDeps(nodes []missionGraphNode, nodeBead map[string]string) (added, skippedN int, err error) {
	for _, n := range nodes {
		if len(n.DependsOn) == 0 {
			continue
		}
		fromID, ok := nodeBead[n.ID]
		if !ok {
			return added, skippedN, fmt.Errorf("node %s has no bead", n.ID)
		}
		existingDeps, err := listDeps(fromID)
		if err != nil {
			return added, skippedN, err
		}
		for _, dep := range n.DependsOn {
			toID, ok := nodeBead[dep]
			if !ok {
				return added, skippedN, fmt.Errorf("node %s depends_on unknown node %s", n.ID, dep)
			}
			if existingDeps[toID] {
				skippedN++
				continue
			}
			if err := bdRun("dep", "add", fromID, "--blocked-by", toID); err != nil {
				return added, skippedN, fmt.Errorf("dep add %s -> %s (%s -> %s): %w", n.ID, dep, fromID, toID, err)
			}
			existingDeps[toID] = true
			added++
		}
	}
	return added, skippedN, nil
}

// listDeps returns the set of bead ids that id currently depends on.
func listDeps(id string) (map[string]bool, error) {
	out, err := bdOutput("dep", "list", id, "--json")
	if err != nil {
		return nil, err
	}
	var recs []depRecord
	if err := json.Unmarshal(out, &recs); err != nil {
		return nil, fmt.Errorf("parse dep list json for %s: %w", id, err)
	}
	set := map[string]bool{}
	for _, r := range recs {
		if r.ID != "" {
			set[r.ID] = true
		}
	}
	return set, nil
}

// --- bd exec helpers ---

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

func bdRun(args ...string) error {
	_, err := bdOutput(args...)
	return err
}
