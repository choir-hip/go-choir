package platform

// Ways the story-to-capture edge (cites_capture, related-stories design
// 5.2) can fail, written before the code:
//
//  E1 The kind is not registered, so CheckWrite refuses the version's
//     batch and every publish fails (R13). A story that cites captures
//     publishes, and the registry accepts the edge.
//  E2 A capture cited by several anchors gets several edges, or a capture
//     cited once gets none. Exactly one edge per distinct cited capture.
//  E3 An anchor whose Ref.Doc is not a capture (empty, a Texture doc id)
//     becomes an edge to something that is not a capture.
//  E4 The edge leaks into the bundle's Citations as one external reference
//     per cited capture (graphCitationEdges treats unknown kinds as
//     citations), and the front depth counts change.
//  E5 A replayed publish, or a second computation, mints different edge
//     ids, so a retry writes duplicates.
//  E6 A later version inherits an older version's edges, or drops a
//     source and keeps its edge: each version carries exactly its own
//     cited captures, and old versions keep theirs (versions are
//     immutable, R3).
//  E7 A story that cites nothing fails to publish, or writes an edge.
//  E8 Publishing the edges changes what the reader sees: the version's
//     manifest hash must not depend on whether edges were written (R11).

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCitesCaptureIsARegisteredEdge(t *testing.T) { // E1
	edge := objectgraph.Edge{EdgeID: "e", FromID: "obj:choir.publication_version:o:v", ToID: "obj:choir.web_capture:o:c", Kind: edgeCitesCapture}
	if err := objectgraph.CheckWrite(nil, []objectgraph.Edge{edge}); err != nil {
		t.Fatalf("cites_capture refused by the write gate: %v", err)
	}
	reg, err := objectgraph.DefaultRegistry().LookupEdge(edgeCitesCapture)
	if err != nil || reg.Owner != objectgraph.OwnerCorpusd {
		t.Fatalf("cites_capture registration = %+v, %v; want owner %q", reg, err, objectgraph.OwnerCorpusd)
	}
}

func TestPublishWritesOneEdgePerDistinctCitedCapture(t *testing.T) { // E1, E2, E6, E7
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	b := l.capture("https://wire.example/b", "", "Ferry operators confirmed Monday service")
	c := l.capture("https://wire.example/c", "", "Inspectors cleared the east pier")

	// v1 cites A twice (two anchors on one capture) and B once.
	v1 := l.publishDoubleCite("doc-harbor", 1, "Harbor reopens", a, b)
	got := l.citesCapture(l.versionCanonicalID(labOwner, v1))
	sort.Strings(got)
	want := []string{a, b}
	sort.Strings(want)
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("v1 cites_capture targets = %v, want exactly %v (one edge per distinct capture)", got, want)
	}

	// v2 drops B and adds C: its edges are its own; v1's stay as they were.
	v2 := l.publish("doc-harbor", 2, "Harbor reopens", a, c)
	got2 := l.citesCapture(l.versionCanonicalID(labOwner, v2))
	sort.Strings(got2)
	want2 := []string{a, c}
	sort.Strings(want2)
	if len(got2) != 2 || got2[0] != want2[0] || got2[1] != want2[1] {
		t.Fatalf("v2 cites_capture targets = %v, want %v", got2, want2)
	}
	again := l.citesCapture(l.versionCanonicalID(labOwner, v1))
	sort.Strings(again)
	if len(again) != 2 || again[0] != want[0] || again[1] != want[1] {
		t.Fatalf("v1 edges changed after v2 published: %v", again)
	}

	// A story that cites nothing publishes and writes no edge.
	v0 := l.publish("doc-bare", 1, "Bare story")
	if edges := l.citesCapture(l.versionCanonicalID(labOwner, v0)); len(edges) != 0 {
		t.Fatalf("a story with no citations wrote cites_capture edges: %v", edges)
	}
}

// publishDoubleCite publishes a story that cites the first capture twice.
func (l *relatedLab) publishDoubleCite(doc string, n int, title string, first, second string) *PublishTextureResponse {
	l.t.Helper()
	return l.publish(doc, n, title, first, second, first)
}

func TestCitedCapturesSkipsNonCaptureRefs(t *testing.T) { // E3, E2
	capA := "obj:choir.web_capture:paper-owner:aaaa"
	capB := "obj:choir.web_capture:paper-owner:bbbb"
	ref := func(doc string) *types.Ref { r := types.Ref{Doc: doc}; return &r }
	trs := []GraphTransclusion{
		{AnchorID: "1", Ref: ref(capB)},
		{AnchorID: "2", Ref: ref(capA)},
		{AnchorID: "3", Ref: ref(capA)},                                       // the same capture again
		{AnchorID: "4", Ref: ref("")},                                         // no doc
		{AnchorID: "5", Ref: ref("obj:choir.texture_document:paper-owner:x")}, // not a capture
		{AnchorID: "6", Ref: nil},                                             // a legacy transclusion has no Ref
		{SourceEntityID: "legacy"},
	}
	got := citedCaptureIDs(trs)
	if len(got) != 2 || got[0] != capA || got[1] != capB {
		t.Fatalf("citedCaptureIDs = %v, want [%s %s] (distinct, captures only, sorted)", got, capA, capB)
	}
}

func TestCitesCaptureEdgeIDsAreDeterministic(t *testing.T) { // E5
	version := "obj:choir.publication_version:paper-owner:v1"
	trs := []GraphTransclusion{{AnchorID: "1", Ref: &types.Ref{Doc: "obj:choir.web_capture:paper-owner:aaaa"}}}
	e1 := citesCaptureEdges(version, trs, time.Unix(100, 0))
	e2 := citesCaptureEdges(version, trs, time.Unix(900, 0))
	if len(e1) != 1 || len(e2) != 1 || e1[0].EdgeID != e2[0].EdgeID || e1[0].EdgeID == "" {
		t.Fatalf("edge ids differ between computations: %+v vs %+v", e1, e2)
	}
	if e1[0].Kind != edgeCitesCapture || e1[0].FromID != version || e1[0].ToID != "obj:choir.web_capture:paper-owner:aaaa" {
		t.Fatalf("edge = %+v", e1[0])
	}
	if string(e1[0].Metadata) != `{"relation":"cites"}` {
		t.Fatalf("edge metadata = %s, want the small deterministic form", e1[0].Metadata)
	}
}

func TestCitesCaptureStaysOutOfBundleCitationsAndDepth(t *testing.T) { // E4, E8
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	b := l.capture("https://wire.example/b", "", "Ferry operators confirmed Monday service")
	resp := l.publish("doc-harbor", 1, "Harbor reopens", a, b)
	if n := len(l.citesCapture(l.versionCanonicalID(labOwner, resp))); n != 2 {
		t.Fatalf("fixture: %d cites_capture edges, want 2", n)
	}
	bundle, err := l.svc.GetPublicationBundleByRoute(context.Background(), resp.PermalinkPath)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	for _, c := range bundle.Citations {
		if c.RelationType == string(edgeCitesCapture) {
			t.Fatalf("cites_capture leaked into the bundle's citations: %+v", c)
		}
	}
	page, err := l.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil || len(page.Stories) != 1 {
		t.Fatalf("front: %v %+v", err, page)
	}
	if got := page.Stories[0].SourceCount; got == nil || *got != 2 {
		t.Fatalf("front source_count = %v, want 2 (distinct captures; the edges do not feed it)", got)
	}

	// The manifest hash is a function of the manifest alone: deleting the
	// edges changes nothing the reader is served.
	before, _ := json.Marshal(bundle.Payload)
	edges, _ := l.svc.ogStore().ListEdgesByKind(context.Background(), l.versionCanonicalID(labOwner, resp), edgeCitesCapture)
	for i := range edges {
		edges[i].Tombstone = true
	}
	if err := l.svc.ogStore().PutBatch(context.Background(), objectgraph.Batch{Edges: edges}); err != nil {
		t.Fatalf("tombstone edges: %v", err)
	}
	bundle2, err := l.svc.GetPublicationBundleByRoute(context.Background(), resp.PermalinkPath)
	if err != nil {
		t.Fatalf("bundle after: %v", err)
	}
	after, _ := json.Marshal(bundle2.Payload)
	if string(before) != string(after) {
		t.Fatalf("payload depends on the cites_capture edges")
	}
}
