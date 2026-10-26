package platform

// Ways the related_story edge can fail (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md,
// "Build: the related_story edge"), written before the code. The edge is the
// research desk's judged pair relation, exported to Store B and read by the
// related-stories walk behind CORPUSD_RELATED_STORY.
//
//  S1 The read switch is off (absent, "off", anything but "on") and an edge
//     still changes a list, or its basis, or a cached answer.
//  S2 The switch is on and an edge does not return its pair, in both
//     directions, or only when the story also cites a capture (the walk's
//     early return for a story with no captures hides the edges).
//  S3 A self edge, a duplicate of a pair the capture walk already lists, an
//     edge to a retracted story, an edge to a story that is not a published
//     Autopaper story, or a tombstoned edge links.
//  S4 The edge lowers the capture threshold or the hub cutoff: a hub capture
//     links its citers, or a below-threshold capture pair links, because the
//     switch is on.
//  S5 A story with more related_story edges than MaxRelatedEdges is a hub for
//     this signal and still votes; the write route lets a story pass the
//     bound.
//  S6 The record route accepts a self pair, an unknown relation, a malformed
//     id, more than 12 pairs, stories of another owner, stories that are not
//     published Autopaper stories, or a computer that is not platform tier;
//     the same pair in the two orders writes two edges; a repeat writes
//     another edge instead of the same one.
//  S7 A judged pair ranks behind a capture link, or ranks by its age before
//     its relation.
//  S8 The record route answers a non-internal caller or any method but POST.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const labComputer = "computer-paper"

// relatedStoryLab is a related lab whose computer is platform tier and whose
// read switch is on.
func newRelatedStoryLab(t *testing.T) *relatedLab {
	t.Helper()
	l := newRealisticLab(t)
	l.svc.relationTier = func(computerID string) (bool, error) { return computerID == labComputer, nil }
	t.Setenv(relatedStoryEnv, "on")
	return l
}

func (l *relatedLab) relateReq(relation string, a, b *PublishTextureResponse) types.StoryRelationsRequest {
	return types.StoryRelationsRequest{
		OwnerID: labOwner, ComputerID: labComputer,
		Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: relation}},
	}
}

func (l *relatedLab) relate(relation string, a, b *PublishTextureResponse) {
	l.t.Helper()
	if _, err := l.svc.RecordStoryRelations(context.Background(), l.relateReq(relation, a, b)); err != nil {
		l.t.Fatalf("record %s %s-%s: %v", relation, a.PublicationID, b.PublicationID, err)
	}
}

func (l *relatedLab) relatedStoryEdges(pub string) []objectgraph.Edge {
	l.t.Helper()
	live := false
	var out []objectgraph.Edge
	for _, f := range []objectgraph.EdgeFilter{
		{FromID: pub, Kind: edgeRelatedStory, Tombstone: &live, Limit: 200},
		{ToID: pub, Kind: edgeRelatedStory, Tombstone: &live, Limit: 200},
	} {
		edges, err := l.svc.ogStore().ListEdges(context.Background(), f)
		if err != nil {
			l.t.Fatalf("list related_story edges: %v", err)
		}
		out = append(out, edges...)
	}
	return out
}

func titlesOf(r *RelatedStories) []string {
	out := []string{}
	for _, link := range r.Links {
		out = append(out, link.Title)
	}
	return out
}

func TestRelatedStoryEdgeIsInertUntilTheReadSwitchIsOn(t *testing.T) { // S1
	l := newRelatedStoryLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded the election to the opposition"))
	b := l.publish("doc-b", 1, "Opposition claims win", l.capture("https://other.example/b", "", "The opposition claimed victory in Serbia"))
	l.relate(types.StoryRelationSameIncident, a, b)

	for _, off := range []string{"", "off", "1", "true", "yes"} {
		t.Setenv(relatedStoryEnv, off)
		got := l.related(a.RoutePath)
		if len(got.Links) != 0 || got.Basis.Mode != relatedModeCaptures {
			t.Fatalf("switch %q: links %v mode %q, want none and the captures mode", off, titlesOf(got), got.Basis.Mode)
		}
	}

	// The cache key carries the mode: an answer cached with the switch off is
	// not served once it is on, and the other way round.
	t.Setenv(relatedStoryEnv, "off")
	l.svc.related.reset()
	if first, err := l.svc.RelatedStories(context.Background(), a.RoutePath, 0); err != nil || len(first.Links) != 0 {
		t.Fatalf("off: %v %v", first, err)
	}
	t.Setenv(relatedStoryEnv, "on")
	on, err := l.svc.RelatedStories(context.Background(), a.RoutePath, 0)
	if err != nil || len(on.Links) != 1 {
		t.Fatalf("on after an off answer was cached: links %v, %v; the cache served the off answer", titlesOf(on), err)
	}
	t.Setenv(relatedStoryEnv, "off")
	back, err := l.svc.RelatedStories(context.Background(), a.RoutePath, 0)
	if err != nil || len(back.Links) != 0 {
		t.Fatalf("off after an on answer was cached: links %v, %v", titlesOf(back), err)
	}
}

func TestRelatedStoryEdgeReturnsItsPairBothWaysWithNoSharedCapture(t *testing.T) { // S2
	l := newRelatedStoryLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded the election to the opposition"))
	b := l.publish("doc-b", 1, "Opposition claims win", l.capture("https://other.example/b", "", "The opposition claimed victory in Serbia"))
	c := l.publish("doc-c", 1, "Bridge closes", l.capture("https://wire.example/c", "", "A bridge closed for repairs"))
	l.relate(types.StoryRelationSameIncident, a, b)

	gotA := l.related(a.RoutePath)
	if gotA.Basis.Mode != relatedModeRelations || len(gotA.Links) != 1 || gotA.Links[0].Title != "Opposition claims win" {
		t.Fatalf("A: mode %q links %v, want only B in the relations mode", gotA.Basis.Mode, titlesOf(gotA))
	}
	if link := gotA.Links[0]; link.Reason != types.StoryRelationSameIncident || link.Shared.Sources != 0 || link.PublicationID == "" || link.Route.Bare == "" {
		t.Fatalf("link = %+v, want reason same_incident, no shared source, a FrontStory row", link)
	}
	if gotB := l.related(b.RoutePath); len(gotB.Links) != 1 || gotB.Links[0].Title != "Vucic concedes" {
		t.Fatalf("B links = %v, want A (the edge reads both ways)", titlesOf(gotB))
	}
	if gotC := l.related(c.RoutePath); len(gotC.Links) != 0 {
		t.Fatalf("C links = %v, want none", titlesOf(gotC))
	}

	// A story that cites no capture still gets its edges (the walk returns
	// early for a story with nothing to walk).
	d := l.publish("doc-d", 1, "Nothing cited")
	e := l.publish("doc-e", 1, "Also nothing cited")
	l.relate(types.StoryRelationSameSubject, d, e)
	if gotD := l.related(d.RoutePath); len(gotD.Links) != 1 || gotD.Links[0].Title != "Also nothing cited" || gotD.Basis.Captures != 0 {
		t.Fatalf("D (no captures) links = %v basis %+v, want E", titlesOf(gotD), gotD.Basis)
	}
}

func TestRelatedStoryEdgeIgnoresSelfDuplicateRetractedAndNonAutopaper(t *testing.T) { // S3
	l := newRelatedStoryLab(t)
	ctx := context.Background()
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	dup := l.publish("doc-dup", 1, "Ferries resume", x) // shares a capture with A, and gets an edge too
	retracted := l.publish("doc-retracted", 1, "Retracted sibling", l.capture("https://wire.example/r", "", "A retracted story's source"))
	private := l.publishAs(labOwner, PublishModeAuto, json.RawMessage(`{"visibility":"private","route":"public"}`), "doc-private", 1, "Private sibling", l.capture("https://wire.example/p", "", "A private story's source"))
	ownerPub := l.publishAs(labOwner, PublishModeOwner, nil, "doc-owner", 1, "Owner published sibling", l.capture("https://wire.example/o", "", "An owner published story's source"))
	good := l.publish("doc-good", 1, "Good sibling", l.capture("https://wire.example/g", "", "A good story's source"))

	for _, other := range []*PublishTextureResponse{dup, retracted, private, ownerPub, good} {
		// Written straight to the graph: the route would refuse some of these.
		l.putRelatedStoryEdge(a.PublicationID, other.PublicationID, types.StoryRelationSameSubject, false)
	}
	l.putRelatedStoryEdge(a.PublicationID, a.PublicationID, types.StoryRelationSameIncident, false) // a self edge
	tomb := l.publish("doc-tomb", 1, "Tombstoned edge sibling", l.capture("https://wire.example/t", "", "A story whose edge was retracted"))
	l.putRelatedStoryEdge(a.PublicationID, tomb.PublicationID, types.StoryRelationSameSubject, true)

	// Retract the retracted story: its publication is no longer published.
	pub, err := l.svc.ogStore().GetObject(ctx, retracted.PublicationID)
	if err != nil {
		t.Fatalf("get publication: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(pub.Metadata, &meta); err != nil {
		t.Fatalf("metadata: %v", err)
	}
	meta["state"] = "retracted"
	pub.Metadata, _ = json.Marshal(meta)
	if err := l.svc.ogStore().PutObject(ctx, pub); err != nil {
		t.Fatalf("retract: %v", err)
	}

	got := l.related(a.RoutePath)
	titles := titlesOf(got)
	if len(titles) != 2 || !contains(titles, "Ferries resume") || !contains(titles, "Good sibling") {
		t.Fatalf("A links = %v, want exactly the duplicate-pair story once and the good sibling", titles)
	}
	for _, link := range got.Links {
		if link.PublicationID == a.PublicationID {
			t.Fatalf("A links to itself: %v", titles)
		}
		if link.Title == "Ferries resume" && link.Reason != types.StoryRelationSameSubject {
			t.Fatalf("the pair both walks found has reason %q, want the judged relation", link.Reason)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// putRelatedStoryEdge writes an edge as the export would, bypassing the route.
func (l *relatedLab) putRelatedStoryEdge(from, to, relation string, tombstone bool) {
	l.t.Helper()
	meta, err := json.Marshal(map[string]string{"relation": relation})
	if err != nil {
		l.t.Fatal(err)
	}
	edge := makeEdge(from, to, edgeRelatedStory, time.Now().UTC(), meta)
	edge.Tombstone = tombstone
	if err := l.svc.ogStore().PutEdge(context.Background(), edge); err != nil {
		l.t.Fatalf("put related_story edge: %v", err)
	}
}

func TestRelatedStoryEdgeLeavesTheThresholdAndTheHubCutoffAlone(t *testing.T) { // S4
	// The hub: seven stories cite one capture. With the switch on and no
	// edge, nothing links, as with it off.
	l := newRelatedStoryLab(t)
	hub := l.capture("https://live.example/blog", "", "Live blog of the day")
	var routes []string
	for i := 0; i < 7; i++ {
		own := l.capture(fmt.Sprintf("https://wire.example/own%d", i), "", fmt.Sprintf("Own source number %d", i))
		routes = append(routes, l.publish(fmt.Sprintf("doc-hub-%d", i), 1, fmt.Sprintf("Hub citer %d", i), hub, own).RoutePath)
	}
	for _, route := range routes {
		if got := l.related(route); len(got.Links) != 0 {
			t.Fatalf("%s linked through a hub with the switch on: %v", route, titlesOf(got))
		}
	}

	// An edge between two hub citers still links that pair, and only it.
	pairA := l.publish("doc-pair-a", 1, "Pair A", hub)
	pairB := l.publish("doc-pair-b", 1, "Pair B", hub)
	l.relate(types.StoryRelationLaterDevelopment, pairA, pairB)
	if got := l.related(pairA.RoutePath); len(got.Links) != 1 || got.Links[0].Title != "Pair B" {
		t.Fatalf("pair A links = %v, want only the judged pair B, not the hub's other citers", titlesOf(got))
	}

	// A judged pair ranks ahead of the capture links of a source six stories
	// cite (exactly six is not yet a hub), and the capture links stay.
	l2 := newRelatedStoryLab(t)
	shared := l2.capture("https://wire.example/shared", "", "A source six stories cite")
	var first *PublishTextureResponse
	for i := 0; i < 6; i++ {
		r := l2.publish(fmt.Sprintf("doc-six-%d", i), 1, fmt.Sprintf("Six citer %d", i), shared)
		if first == nil {
			first = r
		}
	}
	lone := l2.publish("doc-lone", 1, "Lone story", l2.capture("https://wire.example/lone", "", "Nobody else cites this"))
	l2.relate(types.StoryRelationSameSubject, first, lone)
	got := l2.related(first.RoutePath)
	if len(got.Links) != 5 || got.Links[0].Title != "Lone story" || got.Links[0].Reason != types.StoryRelationSameSubject || got.Links[1].Reason != relatedReasonCapture {
		t.Fatalf("first's links = %v, want the judged pair first, then capture links (the default 5)", titlesOf(got))
	}
	if gotLone := l2.related(lone.RoutePath); len(gotLone.Links) != 1 || gotLone.Links[0].Title != "Six citer 0" {
		t.Fatalf("lone links = %v, want only the judged pair", titlesOf(gotLone))
	}

	// The threshold: in a paper of two stories a shared capture scores below
	// it (idf is small), and the switch being on does not lift it.
	small := newRelatedLab(t)
	t.Setenv(relatedStoryEnv, "on")
	sx := small.capture("https://wire.example/sx", "", "A source two stories cite")
	sa := small.publish("doc-small-a", 1, "Small A", sx)
	small.publish("doc-small-b", 1, "Small B", sx)
	if got := small.related(sa.RoutePath); len(got.Links) != 0 {
		t.Fatalf("below-threshold pair linked with the switch on and no edge: %v", titlesOf(got))
	}
}

func TestRelatedStoryEdgeHubDegreeGivesNoVoteAndTheRouteStopsAtTheBound(t *testing.T) { // S5
	l := newRelatedStoryLab(t)
	limit := l.svc.related.tune().MaxRelatedEdges
	if limit != 20 {
		t.Fatalf("MaxRelatedEdges = %d, want 20", limit)
	}
	center := l.publish("doc-center", 1, "Center", l.capture("https://wire.example/center", "", "The center story's source"))
	// Fill the center to the bound through the route.
	var others []*PublishTextureResponse
	for i := 0; i < limit; i++ {
		o := l.publish(fmt.Sprintf("doc-o-%d", i), 1, fmt.Sprintf("Other %d", i), l.capture(fmt.Sprintf("https://wire.example/o%d", i), "", fmt.Sprintf("Other source %d", i)))
		others = append(others, o)
		l.relate(types.StoryRelationSameSubject, center, o)
	}
	if n := len(l.relatedStoryEdges(center.PublicationID)); n != limit {
		t.Fatalf("center has %d edges, want %d", n, limit)
	}
	extra := l.publish("doc-extra", 1, "One too many", l.capture("https://wire.example/extra", "", "One too many source"))
	if _, err := l.svc.RecordStoryRelations(context.Background(), l.relateReq(types.StoryRelationSameSubject, center, extra)); err == nil || !strings.Contains(err.Error(), "relations") {
		t.Fatalf("a %dth edge for the center: err = %v, want a refusal naming the bound", limit+1, err)
	}
	if n := len(l.relatedStoryEdges(center.PublicationID)); n != limit {
		t.Fatalf("center has %d edges after the refusal, want %d", n, limit)
	}

	// Past the bound by another writer: the center is a hub for this signal
	// and its edges give no vote, in its own list and in the others'.
	l.putRelatedStoryEdge(center.PublicationID, extra.PublicationID, types.StoryRelationSameSubject, false)
	if got := l.related(center.RoutePath); len(got.Links) != 0 {
		t.Fatalf("the hub's list = %d links, want none", len(got.Links))
	}
	if got := l.related(others[0].RoutePath); len(got.Links) != 0 {
		t.Fatalf("a hub's neighbour links %v; the hub's edge must not vote", titlesOf(got))
	}
}

func TestRecordStoryRelationsRefusesWhatItShould(t *testing.T) { // S6
	l := newRelatedStoryLab(t)
	ctx := context.Background()
	a := l.publish("doc-a", 1, "Story A", l.capture("https://wire.example/a", "", "Story A source"))
	b := l.publish("doc-b", 1, "Story B", l.capture("https://wire.example/b", "", "Story B source"))
	foreign := l.publishAs("other-owner", PublishModeAuto, nil, "doc-foreign", 1, "Foreign story", l.capture("https://wire.example/f", "", "Foreign source"))
	owner := l.publishAs(labOwner, PublishModeOwner, nil, "doc-owner", 1, "Owner piece", l.capture("https://wire.example/o", "", "Owner source"))
	rel := types.StoryRelationSameIncident

	cases := []struct {
		name string
		req  types.StoryRelationsRequest
		want string
	}{
		{"self pair", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: a.PublicationID, Relation: rel}}}, "itself"},
		{"unknown relation", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: "unrelated"}}}, "relation"},
		{"malformed id", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: "nonsense", StoryB: b.PublicationID, Relation: rel}}}, "publication"},
		{"no owner", types.StoryRelationsRequest{ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: rel}}}, "owner_id"},
		{"empty", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer}, "relations"},
		{"another owner's story", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: foreign.PublicationID, Relation: rel}}}, "owner"},
		{"not an autopaper story", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: owner.PublicationID, Relation: rel}}}, "published"},
		{"not a platform computer", types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: "computer-private", Relations: []types.StoryRelation{{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: rel}}}, "platform"},
	}
	many := types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer}
	for i := 0; i < 13; i++ {
		many.Relations = append(many.Relations, types.StoryRelation{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: rel})
	}
	cases = append(cases, struct {
		name string
		req  types.StoryRelationsRequest
		want string
	}{"13 pairs", many, "12"})
	for _, tc := range cases {
		if n, err := l.svc.RecordStoryRelations(ctx, tc.req); err == nil || !strings.Contains(err.Error(), tc.want) || n != 0 {
			t.Errorf("%s: recorded %d err = %v, want a refusal naming %q", tc.name, n, err, tc.want)
		}
	}
	if edges := l.relatedStoryEdges(a.PublicationID); len(edges) != 0 {
		t.Fatalf("refused requests left %d edges", len(edges))
	}

	// The same pair in both orders, twice, is one edge; a repeat is the same
	// edge; a different relation for the pair is a second edge, and the read
	// lists the pair once, by its strongest relation.
	both := types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: labComputer, Relations: []types.StoryRelation{
		{StoryA: a.PublicationID, StoryB: b.PublicationID, Relation: rel},
		{StoryA: b.PublicationID, StoryB: a.PublicationID, Relation: rel},
	}}
	for i := 0; i < 2; i++ {
		if _, err := l.svc.RecordStoryRelations(ctx, both); err != nil {
			t.Fatalf("record both orders: %v", err)
		}
	}
	edges := l.relatedStoryEdges(a.PublicationID)
	if len(edges) != 1 {
		t.Fatalf("both orders twice wrote %d edges, want 1", len(edges))
	}
	if e := edges[0]; e.FromID >= e.ToID || e.Kind != edgeRelatedStory || string(e.Metadata) != `{"relation":"same_incident"}` {
		t.Fatalf("edge = %+v, want from < to, kind related_story, metadata exactly {\"relation\":\"same_incident\"}", e)
	}
	l.relate(types.StoryRelationSameSubject, a, b)
	if got := l.related(a.RoutePath); len(got.Links) != 1 || got.Links[0].Reason != types.StoryRelationSameIncident {
		t.Fatalf("a pair with two relations lists %d links reason %v, want one, by the strongest", len(got.Links), got.Links)
	}
}

func TestRelatedStoryEdgeRanksByRelationAheadOfCaptureLinks(t *testing.T) { // S7
	l := newRelatedStoryLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	captureSibling := l.publish("doc-cap", 1, "Capture sibling", x)
	oldSubject := l.publish("doc-subject", 1, "Same subject story", l.capture("https://wire.example/s", "", "A subject story's source"))
	newDev := l.publish("doc-dev", 1, "Later development story", l.capture("https://wire.example/d", "", "A development story's source"))
	incident := l.publish("doc-incident", 1, "Same incident story", l.capture("https://wire.example/i", "", "An incident story's source"))
	l.relate(types.StoryRelationSameSubject, a, oldSubject)
	l.relate(types.StoryRelationLaterDevelopment, a, newDev)
	l.relate(types.StoryRelationSameIncident, a, incident)
	_ = captureSibling

	got := l.related(a.RoutePath)
	want := []string{"Same incident story", "Later development story", "Same subject story", "Capture sibling"}
	have := titlesOf(got)
	if strings.Join(have, "|") != strings.Join(want, "|") {
		t.Fatalf("order = %v, want %v (relation first, the capture link after)", have, want)
	}
	if got.Links[3].Reason != relatedReasonCapture {
		t.Fatalf("last link reason = %q, want capture", got.Links[3].Reason)
	}
}

func TestStoryRelationsRouteIsInternalPostOnly(t *testing.T) { // S8
	l := newRelatedStoryLab(t)
	a := l.publish("doc-a", 1, "Story A", l.capture("https://wire.example/a", "", "Story A source"))
	b := l.publish("doc-b", 1, "Story B", l.capture("https://wire.example/b", "", "Story B source"))
	body, _ := json.Marshal(l.relateReq(types.StoryRelationSameIncident, a, b))
	h := NewHandler(l.svc)
	call := func(method string, internal bool, payload []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, types.StoryRelationsPath, bytes.NewReader(payload))
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		h.HandleStoryRelations(w, req)
		return w
	}
	if w := call(http.MethodGet, true, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", w.Code)
	}
	if w := call(http.MethodPost, false, body); w.Code != http.StatusForbidden {
		t.Fatalf("non-internal = %d, want 403", w.Code)
	}
	if w := call(http.MethodPost, true, []byte("{")); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d", w.Code)
	}
	w := call(http.MethodPost, true, body)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"recorded":1`) {
		t.Fatalf("internal POST = %d %s, want 200 recorded 1", w.Code, w.Body.String())
	}
	if n := len(l.relatedStoryEdges(a.PublicationID)); n != 1 {
		t.Fatalf("%d edges after the route, want 1", n)
	}
	refused := types.StoryRelationsRequest{OwnerID: labOwner, ComputerID: "computer-private", Relations: l.relateReq(types.StoryRelationSameIncident, a, b).Relations}
	raw, _ := json.Marshal(refused)
	if w := call(http.MethodPost, true, raw); w.Code != http.StatusBadRequest && w.Code != http.StatusForbidden {
		t.Fatalf("a private computer = %d, want a refusal", w.Code)
	}
}
