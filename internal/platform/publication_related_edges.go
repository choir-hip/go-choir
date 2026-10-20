package platform

// The story-to-capture edge of the related-stories design
// (docs/reports/related-stories-graph-design-2026-10-19.md, 5.2): every
// published version gets one cites_capture edge to each capture its anchors
// cite. Before it, no edge reached a capture, and "which stories cite this
// capture" was a scan of transclusion JSON. The reverse lookup is the
// (to_id, kind) index of og_edges.

import (
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// edgeCitesCapture is the edge from a publication version to a capture one
// of its anchors cites or embeds. It is read only by the related-stories
// walk: it feeds no publish gate, no front depth and no bundle citation.
const edgeCitesCapture objectgraph.EdgeKind = "cites_capture"

// citesCaptureMetadata is the edge's whole metadata. It is fixed so the
// edge id (derived from its metadata) is the same on every computation:
// a replay or a backfill writes the same edge, not another.
const citesCaptureMetadata = `{"relation":"cites"}`

// citedCaptureIDs are the distinct captures a version's anchors cite,
// sorted. An anchor is a GraphTransclusion with a Ref; a legacy
// transclusion has none. A Ref.Doc that is not a capture id is skipped:
// publish refuses such an anchor before this point, and the edge must not
// point at anything else if it ever reached here.
func citedCaptureIDs(transclusions []GraphTransclusion) []string {
	seen := map[string]bool{}
	var out []string
	for _, tr := range transclusions {
		if tr.Ref == nil {
			continue
		}
		if id := tr.Ref.Doc; isCaptureCanonicalID(id) && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func isCaptureCanonicalID(id string) bool {
	kind, _, _, err := objectgraph.ParseCanonicalID(id)
	return err == nil && kind == objectgraph.WebCaptureObjectKind
}

// citesCaptureEdges are a version's cites_capture edges, one per distinct
// cited capture.
func citesCaptureEdges(versionCanonicalID string, transclusions []GraphTransclusion, now time.Time) []objectgraph.Edge {
	ids := citedCaptureIDs(transclusions)
	edges := make([]objectgraph.Edge, 0, len(ids))
	for _, id := range ids {
		edges = append(edges, makeEdge(versionCanonicalID, id, edgeCitesCapture, now, []byte(citesCaptureMetadata)))
	}
	return edges
}
