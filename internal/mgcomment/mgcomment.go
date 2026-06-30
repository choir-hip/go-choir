// Package mgcomment recovers and round-trips the comment prose that the
// beads migration (C5/C6) would otherwise drop when mirroring
// docs/mission-graph.yaml into the beads store.
//
// Three kinds of comment data live in the hand-authored mission graph:
//
//   - Head block: the comment lines (section dividers like "# === ... ==="
//     plus scope/lineage prose) that appear immediately BEFORE a node's
//     "- id:" line. yaml.v3 attaches these as the HeadComment of the
//     sequence element.
//
//   - Pre-depends_on block: the per-node triage verdicts / supersession and
//     lineage rationale that appear INSIDE a node, between "kind:" and
//     "depends_on:". yaml.v3 attaches these as the HeadComment of the
//     depends_on key.
//
//   - Inline comments: the "# texture-cutover-allow: ..." doccheck
//     allow-directives that trail individual id/title/path/ledger lines and
//     sources items. yaml.v3 attaches these as the LineComment of the value
//     (or sequence item) node.
//
// yaml.v2 discards all comments, so capture uses yaml.v3's yaml.Node API.
// The recovered prose is serialized (Encode) into a single text blob stored
// in the bead's --design field, and reconstructed (Decode) on export.
package mgcomment

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	yaml3 "gopkg.in/yaml.v3"
)

// InlinePair is one inline (LineComment) directive bound to a node line.
// Key is one of "id", "title", "path", "ledger", or "src:<value>".
type InlinePair struct {
	Key     string
	Comment string // includes the leading '#'
}

// NodeComments is the full comment + ordering payload recovered for a single
// mission node. Ordering (Index, DepOrder) is real authoring information that
// beads must own once it becomes the source of truth, so the projection can be
// regenerated in the authored order.
type NodeComments struct {
	Head     string       // before-node block (column 0), no trailing newline
	Predep   string       // pre-depends_on block, no trailing newline, no indent
	Inline   []InlinePair // ordered inline directives
	Index    int          // authored position in the nodes sequence (-1 if unknown)
	DepOrder []string      // authored depends_on order (for stable re-emit)
}

// Empty reports whether there is nothing to store/emit for this node. A known
// authored Index is itself worth storing (ordering fidelity), so a node with
// only an Index is not empty.
func (nc NodeComments) Empty() bool {
	return nc.Head == "" && nc.Predep == "" && len(nc.Inline) == 0 &&
		nc.Index < 0 && len(nc.DepOrder) == 0
}

const (
	markHead    = "@@MG-HEAD"
	markPredep  = "@@MG-PREDEP"
	markInline  = "@@MG-INLINE"
	markIndex   = "@@MG-IDX"
	markDepOrd  = "@@MG-DEPORD"
	markEnd     = "@@MG-END"
)

// Encode serializes the comment payload into the design-field blob. The
// encoding is deterministic so re-running capture converges (idempotent).
func (nc NodeComments) Encode() string {
	if nc.Empty() {
		return ""
	}
	var b strings.Builder
	if nc.Index >= 0 {
		fmt.Fprintf(&b, "%s\n%d\n", markIndex, nc.Index)
	}
	if len(nc.DepOrder) > 0 {
		fmt.Fprintf(&b, "%s\n%s\n", markDepOrd, strings.Join(nc.DepOrder, ","))
	}
	if nc.Head != "" {
		b.WriteString(markHead)
		b.WriteString("\n")
		b.WriteString(nc.Head)
		b.WriteString("\n")
	}
	if nc.Predep != "" {
		b.WriteString(markPredep)
		b.WriteString("\n")
		b.WriteString(nc.Predep)
		b.WriteString("\n")
	}
	if len(nc.Inline) > 0 {
		b.WriteString(markInline)
		b.WriteString("\n")
		for _, p := range nc.Inline {
			// Key cannot contain a tab; comment is single-line.
			fmt.Fprintf(&b, "%s\t%s\n", p.Key, p.Comment)
		}
	}
	b.WriteString(markEnd)
	return b.String()
}

// Decode parses a design-field blob produced by Encode. Blobs that do not
// carry the markers decode to an empty NodeComments (no panic), so unrelated
// design text is ignored gracefully.
func Decode(s string) NodeComments {
	nc := NodeComments{Index: -1}
	if !strings.Contains(s, markEnd) {
		return nc
	}
	lines := strings.Split(s, "\n")
	section := ""
	var head, predep []string
	for _, ln := range lines {
		switch ln {
		case markIndex:
			section = "index"
			continue
		case markDepOrd:
			section = "deporder"
			continue
		case markHead:
			section = "head"
			continue
		case markPredep:
			section = "predep"
			continue
		case markInline:
			section = "inline"
			continue
		case markEnd:
			section = ""
			continue
		}
		switch section {
		case "index":
			if v, err := strconv.Atoi(strings.TrimSpace(ln)); err == nil {
				nc.Index = v
			}
			section = ""
		case "deporder":
			if ln != "" {
				nc.DepOrder = strings.Split(ln, ",")
			}
			section = ""
		case "head":
			head = append(head, ln)
		case "predep":
			predep = append(predep, ln)
		case "inline":
			if i := strings.IndexByte(ln, '\t'); i >= 0 {
				nc.Inline = append(nc.Inline, InlinePair{Key: ln[:i], Comment: ln[i+1:]})
			}
		}
	}
	nc.Head = strings.Join(head, "\n")
	nc.Predep = strings.Join(predep, "\n")
	return nc
}

// Inline returns the directive comment for a given key, or "" if absent.
func (nc NodeComments) InlineFor(key string) string {
	for _, p := range nc.Inline {
		if p.Key == key {
			return p.Comment
		}
	}
	return ""
}

// Parse recovers per-node comment payloads from the raw mission-graph.yaml
// bytes. It returns a map keyed by mission node id.
func Parse(raw []byte) (map[string]NodeComments, error) {
	var root yaml3.Node
	if err := yaml3.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("yaml.v3 parse: %w", err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("empty yaml document")
	}
	doc := root.Content[0]
	var nodesSeq *yaml3.Node
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "nodes" {
			nodesSeq = doc.Content[i+1]
			break
		}
	}
	if nodesSeq == nil {
		return nil, fmt.Errorf("no nodes sequence found")
	}

	out := make(map[string]NodeComments, len(nodesSeq.Content))
	for idx, el := range nodesSeq.Content {
		if el.Kind != yaml3.MappingNode {
			continue
		}
		nc := NodeComments{Index: idx}
		nc.Head = strings.TrimRight(el.HeadComment, "\n")
		id := ""
		for i := 0; i+1 < len(el.Content); i += 2 {
			k := el.Content[i]
			v := el.Content[i+1]
			switch k.Value {
			case "id":
				id = v.Value
				if v.LineComment != "" {
					nc.Inline = append(nc.Inline, InlinePair{Key: "id", Comment: v.LineComment})
				}
			case "title", "path", "ledger":
				if v.LineComment != "" {
					nc.Inline = append(nc.Inline, InlinePair{Key: k.Value, Comment: v.LineComment})
				}
			case "depends_on":
				if k.HeadComment != "" {
					nc.Predep = strings.TrimRight(k.HeadComment, "\n")
				}
				for _, it := range v.Content {
					nc.DepOrder = append(nc.DepOrder, it.Value)
					if it.LineComment != "" {
						nc.Inline = append(nc.Inline, InlinePair{Key: "dep:" + it.Value, Comment: it.LineComment})
					}
				}
			case "sources":
				for _, it := range v.Content {
					if it.LineComment != "" {
						nc.Inline = append(nc.Inline, InlinePair{Key: "src:" + it.Value, Comment: it.LineComment})
					}
				}
			}
		}
		if id == "" {
			continue
		}
		out[id] = nc
	}
	return out, nil
}

// SortedIDs returns the node ids of m in deterministic order (for stable
// iteration in callers that need it).
func SortedIDs(m map[string]NodeComments) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
