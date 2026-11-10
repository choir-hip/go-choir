package platform

// Ways the reliance link (relies_on, docs/problems/published-search-recall-2026-11-10.md
// part 2; docs/precommitments/autopaper-materiality-hypotheses-2026-11-10.md)
// can fail, written before the code:
//
//  R1. A story that cites a capture of an earlier published story of the
//      paper gets no relies_on link.
//  R2. A capture of some other host's page whose path happens to name a
//      route of the paper gets a link.
//  R3. A capture on the paper's host that names no published story (an
//      unknown route, a withdrawn story) gets a link, or fails the publish.
//  R4. A republish writes a second link, or drops the first.
//  R5. A story gets a link to itself (it cites its own earlier version).
//  R6. The link is read: by a ranker, a front list, a search, or put in a
//      prompt. It is data only, so no non-test source outside its writer
//      and its registration may name it.
//  R7. The link is not registered, so the write gate refuses the batch and
//      every publish that cites an earlier story fails.
//  R8. The link carries a key the vocabulary write guard refuses on a
//      computer (internal/store test).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func (l *relatedLab) reliesOn(pubCanonicalID string) []objectgraph.Edge {
	l.t.Helper()
	edges, err := l.svc.ogStore().ListEdgesByKind(context.Background(), pubCanonicalID, objectgraph.EdgeReliesOn)
	if err != nil {
		l.t.Fatalf("list relies_on: %v", err)
	}
	return edges
}

func (l *relatedLab) pubID(doc string) string {
	l.t.Helper()
	ident, err := publicationIdentityFor(labOwner, doc)
	if err != nil {
		l.t.Fatal(err)
	}
	return ident.CanonicalID
}

func TestReliesOnIsARegisteredEdge(t *testing.T) { // R7
	edge := objectgraph.Edge{EdgeID: "e", FromID: "obj:choir.publication:o:b", ToID: "obj:choir.publication:o:a", Kind: objectgraph.EdgeReliesOn}
	if err := objectgraph.CheckWrite(nil, []objectgraph.Edge{edge}); err != nil {
		t.Fatalf("relies_on refused by the write gate: %v", err)
	}
	reg, err := objectgraph.DefaultRegistry().LookupEdge(objectgraph.EdgeReliesOn)
	if err != nil || reg.Owner != objectgraph.OwnerCorpusd {
		t.Fatalf("relies_on registration = %+v, %v; want owner %q", reg, err, objectgraph.OwnerCorpusd)
	}
}

func TestStoryCitingAnEarlierStoryRecordsReliesOn(t *testing.T) { // R1, R4, R5
	l := newRelatedLab(t)
	earlier := l.publish("doc-earlier", 1, "Harbor reopens")
	captured := l.capture(l.svc.publicationExportReadURL(earlier.RoutePath), "", "The harbor reopened at dawn")
	external := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")

	later := l.publish("doc-later", 1, "Ferries resume", captured, external)
	edges := l.reliesOn(l.pubID("doc-later"))
	if len(edges) != 1 || edges[0].ToID != l.pubID("doc-earlier") || edges[0].FromID != l.pubID("doc-later") {
		t.Fatalf("relies_on edges = %+v, want one from the later story to the earlier", edges)
	}
	if string(edges[0].Metadata) != objectgraph.ReliesOnMetadata {
		t.Fatalf("edge metadata = %s, want the fixed form %s", edges[0].Metadata, objectgraph.ReliesOnMetadata)
	}
	// The direction is one way: the earlier story relies on nothing.
	if back := l.reliesOn(l.pubID("doc-earlier")); len(back) != 0 {
		t.Fatalf("the earlier story got a relies_on edge: %+v", back)
	}

	// R4: a republish that still cites the story leaves exactly one edge, with
	// the same id; one that drops the citation keeps what was recorded.
	l.publish("doc-later", 2, "Ferries resume", captured, external)
	again := l.reliesOn(l.pubID("doc-later"))
	if len(again) != 1 || again[0].EdgeID != edges[0].EdgeID {
		t.Fatalf("after a republish relies_on edges = %+v, want the same single edge %s", again, edges[0].EdgeID)
	}
	l.publish("doc-later", 3, "Ferries resume", external)
	if kept := l.reliesOn(l.pubID("doc-later")); len(kept) != 1 {
		t.Fatalf("a republish that dropped the citation changed the record: %+v", kept)
	}
	_ = later

	// R5: a story that cites its own earlier version records no link to itself.
	selfCap := l.capture(l.svc.publicationExportReadURL(later.RoutePath), "", "Ferries resume Monday")
	l.publish("doc-later", 4, "Ferries resume", selfCap, external)
	for _, e := range l.reliesOn(l.pubID("doc-later")) {
		if e.ToID == l.pubID("doc-later") {
			t.Fatalf("a story relies on itself: %+v", e)
		}
	}
}

func TestReliesOnIgnoresForeignHostsAndUnknownRoutes(t *testing.T) { // R2, R3
	l := newRelatedLab(t)
	earlier := l.publish("doc-earlier", 1, "Harbor reopens")
	foreign := l.capture("https://evil.example/api/platform/publications/export?route="+earlier.RoutePath+"&format=txt&raw=1", "", "A page on another host")
	unknown := l.capture(l.svc.publicationExportReadURL("/pub/texture/no-such-story"), "", "A route the paper never published")
	plain := l.capture("https://wire.example/a", "", "An ordinary source")
	l.publish("doc-later", 1, "Ferries resume", foreign, unknown, plain)
	if edges := l.reliesOn(l.pubID("doc-later")); len(edges) != 0 {
		t.Fatalf("relies_on edges = %+v, want none for a foreign host, an unknown route or an ordinary source", edges)
	}

	// A story withdrawn after it was published is not relied on.
	withdrawn := l.publish("doc-gone", 1, "Withdrawn story")
	wcap := l.capture(l.svc.publicationExportReadURL(withdrawn.RoutePath), "", "Withdrawn text")
	if _, err := l.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, l.pubID("doc-gone")); err != nil {
		t.Fatal(err)
	}
	l.publish("doc-after", 1, "After the withdrawal", wcap)
	if edges := l.reliesOn(l.pubID("doc-after")); len(edges) != 0 {
		t.Fatalf("relies_on to a withdrawn story: %+v", edges)
	}
}

// R6: the link is data. No source outside its writer and its registration
// names the edge kind, and no prompt or overlay mentions it.
func TestReliesOnIsReadByNothing(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := map[string]bool{
		filepath.Join("internal", "objectgraph", "registry.go"):          true,
		filepath.Join("internal", "objectgraph", "reliance.go"):          true,
		filepath.Join("internal", "platform", "publication_reliance.go"): true,
	}
	var offenders []string
	for _, dir := range []string{"internal", "cmd"} {
		_ = filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			name := info.Name()
			if strings.HasSuffix(name, "_test.go") || !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".md")) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if allowed[rel] {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if s := string(data); strings.Contains(s, "relies_on") || strings.Contains(s, "EdgeReliesOn") || strings.Contains(s, "ReliesOnMetadata") {
				offenders = append(offenders, rel)
			}
			return nil
		})
	}
	if len(offenders) > 0 {
		t.Fatalf("the reliance link is read or named outside its writer and registration: %v", offenders)
	}
}
