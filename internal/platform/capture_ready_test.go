package platform

// The ready list and the entity candidates (capture_ready.go, station E2 of
// docs/design/entity-layer-2026-11-20.md). Failure list, written before the
// tests:
//
//  Y1 a capture without a vector is listed as ready, or a ready one is not
//     (stored, refused, owner-private), or waiting is not counted;
//  Y2 a capture with a proj_capture_resolution row is listed, or a refused
//     row is not honoured; another computer's, a superseded or a tombstoned
//     capture is listed;
//  Y3 the list is not newest first, ignores its limit, or does not say more;
//  Y4 an old guest's request (no ready) is answered differently, or carries a
//     ready object;
//  Y5 title, lead, outlet (registrable domain) or language is wrong;
//  Y6 a neighbour's event and entity links are missing, or tentative links
//     count as known;
//  Y7 "U.S." and "United States", "Trump" and "Donald Trump" are not each
//     other's candidates;
//  Y8 "Parliament", "Supreme Court" and "Washington" come out as one
//     confident candidate;
//  Y9 "Chinese", "Health" or "Pedestrian" is proposed as a new entity or
//     offered as an existing one;
//  Y10 a computer that is not platform tier is given candidates;
//  Y11 the candidates are unbounded, or not strongest lookup first, or not
//     the more central entity first within a lookup;
//  Y12 the resolution table is not idempotent, or takes an outcome that is
//     neither resolved nor refused;
//  Y13 a tentative role counts as a story, or a story may attach under it.

import (
	"bytes"
	"context"
	"database/sql"
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

const readyOwner, readyComputer = "o1", "c1"

// entityRowObject writes a choir.entity row for owner (as a story export
// does), and returns its canonical id.
func (f *vecFixture) entityRow(t *testing.T, owner, name, typ string, aliases ...string) string {
	t.Helper()
	id := types.MintedEntityID(name)
	canonical, err := entityCanonicalFor(owner, id)
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]any{"entity_id": id, "name": name, "type": typ, "normalized": types.NormalizeEntityName(name), "wikidata_id": "", "computer_id": "c1", "aliases": aliases})
	body, _ := json.Marshal(entityRow{EntityID: id, Name: name, Type: typ, Normalized: types.NormalizeEntityName(name), ComputerID: "c1"})
	now := time.Now().UTC()
	obj := objectgraph.Object{CanonicalID: canonical, ObjectKind: ogKindEntity, OwnerID: owner, ComputerID: "c1", ContentHash: "h-" + id, Metadata: meta, Body: body, CreatedAt: now, UpdatedAt: now}
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("write entity %s: %v", name, err)
	}
	return canonical
}

// mention writes a mention edge from a story (a publication id the layer
// need not hold, the count only reads the edge) or a capture to an entity.
func (f *vecFixture) mention(t *testing.T, from, entityCanonical, role string) {
	t.Helper()
	if err := f.og.PutEdge(context.Background(), makeEdge(from, entityCanonical, edgeMentionsEntity, time.Now().UTC(), mentionMetadata(role))); err != nil {
		t.Fatalf("mention edge: %v", err)
	}
}

func readyReq(limit int) CaptureNeighboursRequest {
	return CaptureNeighboursRequest{OwnerID: readyOwner, ComputerID: readyComputer, Ready: true, ReadyLimit: limit, Limit: 5}
}

func (f *vecFixture) ready(t *testing.T, req CaptureNeighboursRequest) CaptureNeighboursResponse {
	t.Helper()
	f.svc.relationTier = func(c string) (bool, error) { return c == readyComputer, nil }
	res, err := f.svc.CaptureReadyList(context.Background(), req)
	if err != nil {
		t.Fatalf("CaptureReadyList: %v", err)
	}
	if res.Ready == nil {
		t.Fatalf("no ready list: %+v", res)
	}
	return res
}

func readyIDs(l *CaptureReadyList) []string {
	out := []string{}
	for _, c := range l.Captures {
		out = append(out, c.CaptureID)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReadyListHasVectoredRefusedAndPrivateCapturesAndCountsTheWaiting(t *testing.T) { // Y1 Y2 Y3 Y5
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, readyOwner, readyComputer, "stored", "Stored", "quorum valdoria amber treaty ratified", now.Add(-1*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "refused", "Refused", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "private", "Private", "quorum valdoria secret", now.Add(-3*time.Hour), true)
	f.capture(t, readyOwner, readyComputer, "old", "Old", "quorum valdoria older", now.Add(-4*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "elsewhere", "Elsewhere", "quorum valdoria far", now.Add(-90*time.Minute), false)
	gone := f.capture(t, readyOwner, readyComputer, "gone", "Gone", "quorum valdoria gone", now.Add(-5*time.Hour), false)
	f.pass(t)
	// Captures after the pass have no vector yet: waiting.
	f.capture(t, readyOwner, readyComputer, "wait1", "Wait one", "quorum valdoria new one", now.Add(-10*time.Minute), false)
	f.capture(t, readyOwner, readyComputer, "wait2", "Wait two", "quorum valdoria new two", now.Add(-5*time.Minute), false)
	// Another computer's capture is not this computer's.
	f.capture(t, readyOwner, "c2", "othercomputer", "Else", "quorum valdoria other", now.Add(-time.Hour), false)
	id := func(k string) string { return mustCaptureID(t, readyOwner, k) }
	f.refuse(t, id("refused"))
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET superseded_by = ? WHERE canonical_id = ?`, id("stored"), gone.CanonicalID); err != nil {
		t.Fatal(err)
	}

	res := f.ready(t, readyReq(0))
	l := res.Ready
	want := []string{id("stored"), id("elsewhere"), id("refused"), id("private"), id("old")}
	if !sameStrings(readyIDs(l), want) {
		t.Fatalf("ready = %v, want %v (newest first; waiting, other computers' and superseded ones left out)", readyIDs(l), want)
	}
	if l.Waiting != 2 || l.More || l.Scanned != 7 {
		t.Fatalf("waiting %d more %v scanned %d, want 2, false, 7 (the superseded one is not read)", l.Waiting, l.More, l.Scanned)
	}
	byID := map[string]ReadyCapture{}
	for _, c := range l.Captures {
		byID[c.CaptureID] = c
	}
	if byID[id("stored")].Vector != ReadyVectorStored || byID[id("refused")].Vector != ReadyVectorRefused || byID[id("private")].Vector != ReadyVectorExcluded {
		t.Fatalf("vector states: %+v", l.Captures)
	}
	// Y5: the text and the facts of a capture.
	st := byID[id("stored")]
	if st.Title != "Stored" || !strings.Contains(st.Lead, "quorum valdoria amber treaty ratified") || st.Outlet != "example.com" || st.FetchedAt == "" {
		t.Fatalf("stored capture: %+v", st)
	}
	// The neighbours of the stored capture were answered in the same response,
	// and the others' neighbour lists are on the ready items.
	if len(res.Neighbours[id("stored")]) == 0 || len(st.Neighbours) != len(res.Neighbours[id("stored")]) {
		t.Fatalf("neighbours: map %v, item %v", res.Neighbours[id("stored")], st.Neighbours)
	}
	if len(byID[id("private")].Neighbours) != 0 {
		t.Fatalf("a private capture has neighbours: %+v", byID[id("private")])
	}

	// Y3: a limit of two lists the two newest and says there is more.
	res = f.ready(t, readyReq(2))
	if got := readyIDs(res.Ready); !sameStrings(got, want[:2]) || !res.Ready.More {
		t.Fatalf("limit 2: %v more %v", got, res.Ready.More)
	}
}

func TestReadyListLeavesOutResolvedAndRefusedCapturesAndTheTableIsIdempotent(t *testing.T) { // Y2 Y12
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, readyOwner, readyComputer, "a", "A", "quorum valdoria amber", now.Add(-1*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "c", "C", "quorum valdoria garden", now.Add(-3*time.Hour), false)
	f.pass(t)
	id := func(k string) string { return mustCaptureID(t, readyOwner, k) }
	ctx := context.Background()
	for i := 0; i < 2; i++ { // twice: the same rows, no error
		if err := f.store.ensureCaptureIndexTables(ctx); err != nil {
			t.Fatal(err)
		}
		if err := f.store.RecordCaptureResolutions(ctx, []CaptureResolution{
			{CaptureID: id("a"), OwnerID: readyOwner, ComputerID: readyComputer, Outcome: ResolutionResolved, RecordID: "rec-1"},
			{CaptureID: id("b"), OwnerID: readyOwner, ComputerID: readyComputer, Outcome: ResolutionRefused, RecordID: "rec-1"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if f.count(t, captureResolutionTable) != 2 {
		t.Fatalf("rows = %d, want 2", f.count(t, captureResolutionTable))
	}
	if got := readyIDs(f.ready(t, readyReq(0)).Ready); !sameStrings(got, []string{id("c")}) {
		t.Fatalf("ready after resolutions = %v, want only c", got)
	}
	if err := f.store.RecordCaptureResolutions(ctx, []CaptureResolution{{CaptureID: id("c"), OwnerID: readyOwner, ComputerID: readyComputer, Outcome: "maybe"}}); err == nil {
		t.Fatal("an outcome that is neither resolved nor refused was written")
	}
}

func TestAnOldGuestsRequestGetsTheOldAnswerAndNoReadyObject(t *testing.T) { // Y4
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, readyOwner, readyComputer, "a", "A", "quorum valdoria amber", now.Add(-1*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.pass(t)
	f.svc.relationTier = func(c string) (bool, error) { return true, nil }
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: f.og}), f.og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": {Found: true, UserID: readyOwner, ComputerID: readyComputer}}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	h.ConfigureReadyList(f.svc)
	post := func(body string) map[string]json.RawMessage {
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/neighbours", bytes.NewReader([]byte(body)))
		r.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureNeighbours(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		var out map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	a := mustCaptureID(t, readyOwner, "a")
	old := post(`{"owner_id":"o1","computer_id":"c1","capture_ids":["` + a + `"]}`)
	if _, has := old["ready"]; has || len(old["neighbours"]) == 0 {
		t.Fatalf("old request answered with %v", old)
	}
	cur := post(`{"owner_id":"o1","computer_id":"c1","ready":true}`)
	var l CaptureReadyList
	if err := json.Unmarshal(cur["ready"], &l); err != nil || len(l.Captures) != 2 {
		t.Fatalf("new request: %v %s", err, cur["ready"])
	}
	// A handler with no service (an older wiring) answers the plain answer.
	h.ConfigureReadyList(nil)
	if plain := post(`{"owner_id":"o1","computer_id":"c1","ready":true}`); plain["ready"] != nil {
		t.Fatalf("a handler without the service sent a ready object: %s", plain["ready"])
	}
}

func TestReadyNeighboursCarryTheirEventAndEntityLinks(t *testing.T) { // Y6
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, readyOwner, readyComputer, "q", "Quorum", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "n1", "Near one", "quorum valdoria amber treaty ratified today", now.Add(-2*time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "n2", "Near two", "quorum valdoria harvest", now.Add(-3*time.Hour), false)
	f.pass(t)
	id := func(k string) string { return mustCaptureID(t, readyOwner, k) }
	val := f.entityRow(t, readyOwner, "Valdoria", types.EntityTypePlace)
	tent := f.entityRow(t, readyOwner, "Quorum Labs", types.EntityTypeOrganization)
	f.mention(t, id("n1"), val, types.EntityRoleCentral)
	f.mention(t, id("n1"), tent, types.EntityRoleTentative)
	// An in_event edge: the event projection's id is the edge's target.
	// in_event is not a registered kind yet (the event projection is station
	// E5's), so the row is written under the registry, as E5's export will.
	if _, err := f.store.corpus().Exec(`INSERT INTO og_edges (edge_id, from_id, to_id, kind, metadata, created_at, tombstone) VALUES (?, ?, ?, ?, ?, ?, FALSE)`,
		"edge:in_event:test", id("n1"), "obj:choir.event_projection:o1:ev1", string(edgeInEvent), `{"role":"member"}`, now); err != nil {
		t.Fatal(err)
	}
	res := f.ready(t, readyReq(1))
	if got := readyIDs(res.Ready); !sameStrings(got, []string{id("q")}) {
		t.Fatalf("ready = %v", got)
	}
	var n1 *ReadyNeighbour
	for i, n := range res.Ready.Captures[0].Neighbours {
		if n.CaptureID == id("n1") {
			n1 = &res.Ready.Captures[0].Neighbours[i]
		}
	}
	if n1 == nil || n1.EventID != "obj:choir.event_projection:o1:ev1" {
		t.Fatalf("n1 = %+v, want its event", n1)
	}
	roles := map[string]string{}
	for _, e := range n1.Entities {
		roles[e.EntityID] = e.Role
	}
	if roles[types.MintedEntityID("Valdoria")] != types.EntityRoleCentral || roles[types.MintedEntityID("Quorum Labs")] != types.EntityRoleTentative {
		t.Fatalf("n1 entities = %+v", n1.Entities)
	}
}

// ---- candidates ----

func baselineLexicon(t *testing.T, f *vecFixture) {
	t.Helper()
	for _, e := range []struct{ name, typ string }{
		{"U.S.", types.EntityTypePlace}, {"United States", types.EntityTypePlace},
		{"Trump", types.EntityTypePerson}, {"Donald Trump", types.EntityTypePerson},
		{"Parliament", types.EntityTypeOrganization}, {"Supreme Court", types.EntityTypeOrganization},
		{"Washington", types.EntityTypePlace}, {"George Washington", types.EntityTypePerson},
		{"Washington Post", types.EntityTypeOrganization}, {"Washington State", types.EntityTypePlace},
		{"Chinese", types.EntityTypeOrganization}, {"Health", types.EntityTypeOrganization}, {"Pedestrian", types.EntityTypePerson},
		{"United Arab Emirates", types.EntityTypePlace},
	} {
		f.entityRow(t, readyOwner, e.name, e.typ)
	}
}

func (f *vecFixture) mentionsOf(t *testing.T, title, lead string) map[string]ReadyMention {
	t.Helper()
	ctx := context.Background()
	lex, err := f.svc.entityLexicon(ctx, f.svc.ogStore(), readyOwner)
	if err != nil {
		t.Fatal(err)
	}
	raws := lex.mentionsFor(title, lead, nil)
	canon := map[string]bool{}
	for _, m := range raws {
		for _, c := range m.cands {
			e, _ := lex.entity(c.id)
			canon[e.Canonical] = true
		}
	}
	counts, err := f.svc.entityMentionCounts(ctx, canon)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]ReadyMention{}
	for _, m := range raws {
		out[m.key] = lex.finishMention(m, counts, nil)
	}
	return out
}

func hasCandidate(m ReadyMention, name string) (ReadyCandidate, bool) {
	for _, c := range m.Candidates {
		if c.Name == name {
			return c, true
		}
	}
	return ReadyCandidate{}, false
}

func TestCandidatesLinkTheBaselinesDuplicatePairsBothWays(t *testing.T) { // Y7
	f := newVecFixture(t)
	baselineLexicon(t, f)
	ms := f.mentionsOf(t, "Talks in the capital",
		"Officials of the U.S. said on Monday that the United States will not pause talks. Trump spoke later, and Donald Trump added a word. The UAE sent a delegation.")
	us := ms["us"]
	if c, ok := hasCandidate(us, "U.S."); !ok || c.Via != "exact" {
		t.Fatalf("mention U.S.: %+v", us)
	}
	if c, ok := hasCandidate(us, "United States"); !ok || c.Via != "initials" {
		t.Fatalf("U.S. does not offer United States: %+v", us.Candidates)
	}
	states := ms["united states"]
	if c, ok := hasCandidate(states, "United States"); !ok || c.Via != "exact" {
		t.Fatalf("mention United States: %+v", states)
	}
	if c, ok := hasCandidate(states, "U.S."); !ok || c.Via != "initials" {
		t.Fatalf("United States does not offer U.S.: %+v", states.Candidates)
	}
	if c, ok := hasCandidate(ms["trump"], "Donald Trump"); !ok || c.Via != "near" {
		t.Fatalf("Trump does not offer Donald Trump: %+v", ms["trump"].Candidates)
	}
	if c, ok := hasCandidate(ms["donald trump"], "Trump"); !ok || c.Via != "subset" {
		t.Fatalf("Donald Trump does not offer Trump: %+v", ms["donald trump"].Candidates)
	}
	if c, ok := hasCandidate(ms["uae"], "United Arab Emirates"); !ok || c.Via != "initials" {
		t.Fatalf("UAE does not offer United Arab Emirates: %+v", ms["uae"])
	}
}

func TestAmbiguousNamesAreNeverOneConfidentCandidate(t *testing.T) { // Y8
	f := newVecFixture(t)
	baselineLexicon(t, f)
	ms := f.mentionsOf(t, "Politics",
		"Members of Parliament voted. The Supreme Court ruled on Friday. Washington said it would wait, and Washington Post reported it.")
	for _, key := range []string{"parliament", "supreme court", "washington"} {
		m, ok := ms[key]
		if !ok || !m.Ambiguous || m.AmbiguousWhy == "" || len(m.Candidates) == 0 {
			t.Fatalf("%s: %+v, want an ambiguous mention with its candidates", key, m)
		}
		for _, c := range m.Candidates {
			if !c.Ambiguous {
				t.Fatalf("%s offers %s as an unambiguous candidate", key, c.Name)
			}
		}
	}
	// Washington is offered with the names built on it, so the judge sees them.
	for _, name := range []string{"Washington", "George Washington", "Washington Post", "Washington State"} {
		if _, ok := hasCandidate(ms["washington"], name); !ok {
			t.Fatalf("Washington does not offer %s: %+v", name, ms["washington"].Candidates)
		}
	}
	// A name that is not ambiguous is not flagged.
	if ms2 := f.mentionsOf(t, "", "The UAE sent a delegation. UAE officials left."); ms2["uae"].Ambiguous {
		t.Fatalf("UAE flagged: %+v", ms2["uae"])
	}
}

func TestDemonymsAndCommonNounsAreNeitherProposedNorOffered(t *testing.T) { // Y9
	f := newVecFixture(t)
	baselineLexicon(t, f)
	text := "Chinese officials met on Monday in Geneva. Health ministers from Halvorsen Dynamics joined them. A Pedestrian was struck by a car. " +
		"Later, Chinese delegates and Halvorsen Dynamics engineers spoke. The Chinese side left."
	ms := f.mentionsOf(t, "Chinese Health Pedestrian", text)
	for _, key := range []string{"chinese", "health", "pedestrian"} {
		if m, ok := ms[key]; ok {
			t.Fatalf("%s is a mention: %+v", key, m)
		}
	}
	if m, ok := ms["halvorsen dynamics"]; !ok || !m.New || len(m.Candidates) != 0 {
		t.Fatalf("a real unknown name is not proposed as new: %+v", ms)
	}
}

func TestAComputerThatIsNotPlatformTierGetsNoCandidates(t *testing.T) { // Y10
	f := newVecFixture(t)
	baselineLexicon(t, f)
	f.capture(t, readyOwner, readyComputer, "a", "The U.S. and Trump", "The U.S. said Trump spoke. Later, Trump left the U.S.", time.Now().UTC().Add(-time.Hour), false)
	f.pass(t)
	f.svc.relationTier = func(c string) (bool, error) { return false, nil }
	res, err := f.svc.CaptureReadyList(context.Background(), readyReq(0))
	if err != nil || res.Ready == nil || len(res.Ready.Captures) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if res.Ready.CandidatesUnavailable == "" || len(res.Ready.Captures[0].Mentions) != 0 {
		t.Fatalf("a computer outside the platform tier was given mentions: %+v", res.Ready)
	}
	// On the platform tier the same capture has them, end to end.
	res = f.ready(t, readyReq(0))
	got := map[string]bool{}
	for _, m := range res.Ready.Captures[0].Mentions {
		got[m.Key] = true
	}
	if !got["us"] || !got["trump"] || res.Ready.CandidatesUnavailable != "" {
		t.Fatalf("mentions of the capture: %+v", res.Ready.Captures[0].Mentions)
	}
}

func TestCandidatesAreBoundedStrongestLookupFirstAndHeavierEntityFirst(t *testing.T) { // Y11
	f := newVecFixture(t)
	// Twelve entities whose names hold "Quorum", one named Quorum.
	exact := f.entityRow(t, readyOwner, "Quorum", types.EntityTypeOrganization)
	var heavy string
	for i := 0; i < 12; i++ {
		name := "Quorum " + string(rune('A'+i)) + "tlas"
		c := f.entityRow(t, readyOwner, name, types.EntityTypeOrganization)
		if i == 7 {
			heavy = name
			f.mention(t, "obj:choir.publication:o1:s1", c, types.EntityRoleCentral)
			f.mention(t, "obj:choir.publication:o1:s2", c, types.EntityRoleCentral)
		}
	}
	_ = exact
	ms := f.mentionsOf(t, "Quorum news", "Quorum met on Monday. Quorum voted again, and Quorum adjourned.")
	m := ms["quorum"]
	if len(m.Candidates) != readyCandidatesPerMention {
		t.Fatalf("%d candidates, want the bound %d: %+v", len(m.Candidates), readyCandidatesPerMention, m.Candidates)
	}
	if m.Candidates[0].Name != "Quorum" || m.Candidates[0].Via != "exact" {
		t.Fatalf("the exact name is not first: %+v", m.Candidates[0])
	}
	rest := m.Candidates[1:]
	if rest[0].Name != heavy || rest[0].Central != 2 || rest[0].Mentions != 2 {
		t.Fatalf("the more central entity is not first among its lookup: %+v", rest[0])
	}
}

func TestATentativeRoleIsNotAStoryRoleAndIsCountedNowhere(t *testing.T) { // Y13
	if !types.EntityRoles[types.EntityRoleTentative] || types.StoryEntityRole(types.EntityRoleTentative) {
		t.Fatal("tentative must be a mention role and not a story role")
	}
	edges := []objectgraph.Edge{
		{FromID: "s1", Metadata: mentionMetadata(types.EntityRoleCentral)},
		{FromID: "s2", Metadata: mentionMetadata(types.EntityRoleTentative)},
		{FromID: "c1", Metadata: mentionMetadata(types.EntityRoleTentative)},
	}
	if stories, central := mentionCounts(edges); stories != 1 || central != 1 {
		t.Fatalf("counts = %d, %d, want 1, 1", stories, central)
	}
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Story", l.capture("https://wire.example/a", "", "A story"))
	req := l.entitiesReq(a, newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleTentative))
	if _, err := l.svc.RecordStoryEntities(context.Background(), req); err == nil {
		t.Fatal("a story attached an entity under tentative")
	}
}

func TestRegistrableDomainIsTheOutlet(t *testing.T) {
	for in, want := range map[string]string{
		"www.bbc.co.uk": "bbc.co.uk", "news.bbc.co.uk": "bbc.co.uk", "example.com": "example.com",
		"WWW.Example.COM": "example.com", "127.0.0.1": "127.0.0.1", "localhost": "localhost", "": "",
	} {
		if got := registrableDomain(in); got != want {
			t.Errorf("registrableDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadyRequestWithItsOwnIdsNeverFailsWhereTheOldRouteServed(t *testing.T) { // Y3 bound
	f := newVecFixture(t)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		f.capture(t, readyOwner, readyComputer, fmt.Sprint("r", i), "R", "quorum valdoria number "+fmt.Sprint(i), now.Add(-time.Duration(i+1)*time.Hour), false)
	}
	f.pass(t)
	named := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("obj:choir.web_capture:%s:none-%d", readyOwner, i)
		}
		return out
	}
	// At the old route's bound: it served 5,000 ids; with ready:true the list
	// takes the room that is left (none), says more, and nothing is refused.
	req := readyReq(0)
	req.CaptureIDs = named(captureNeighboursMaxIDs)
	res := f.ready(t, req)
	if len(res.Ready.Captures) != 0 || !res.Ready.More {
		t.Fatalf("a full request: %d listed, more %v", len(res.Ready.Captures), res.Ready.More)
	}
	// One short of the bound: room for exactly one.
	req.CaptureIDs = named(captureNeighboursMaxIDs - 1)
	res = f.ready(t, req)
	if len(res.Ready.Captures) != 1 || !res.Ready.More {
		t.Fatalf("one room: %d listed, more %v", len(res.Ready.Captures), res.Ready.More)
	}
	// Over the bound the old route refused too: still refused, not truncated.
	req.CaptureIDs = named(captureNeighboursMaxIDs + 1)
	f.svc.relationTier = func(c string) (bool, error) { return true, nil }
	if _, err := f.svc.CaptureReadyList(context.Background(), req); err == nil {
		t.Fatal("a request over the bound was served")
	}
}

func TestReadyScanIsOneStatementAndSupersededCapturesDoNotFillItsWindow(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	var dead []string
	for i := 0; i < 5; i++ {
		o := f.capture(t, readyOwner, readyComputer, fmt.Sprint("dead", i), "D", "quorum dead "+fmt.Sprint(i), now.Add(-time.Duration(i)*time.Minute), false)
		dead = append(dead, o.CanonicalID)
	}
	f.capture(t, readyOwner, readyComputer, "live", "L", "quorum live", now.Add(-24*time.Hour), false)
	f.pass(t)
	for _, id := range dead {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET superseded_by = 'x' WHERE canonical_id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	old := readyScanMaxForTest(2)
	defer old()
	res := f.ready(t, readyReq(0))
	if got := readyIDs(res.Ready); !sameStrings(got, []string{mustCaptureID(t, readyOwner, "live")}) || res.Ready.Scanned != 1 || res.Ready.More {
		t.Fatalf("a window of 2 full of superseded captures hid the live one: %v scanned %d more %v", got, res.Ready.Scanned, res.Ready.More)
	}
}

func TestReadyScanBoundSaysMore(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		f.capture(t, readyOwner, readyComputer, fmt.Sprint("w", i), "W", "quorum w "+fmt.Sprint(i), now.Add(-time.Duration(i)*time.Minute), false)
	}
	defer readyScanMaxForTest(3)()
	f.svc.relationTier = func(c string) (bool, error) { return true, nil }
	res, err := f.svc.CaptureReadyList(context.Background(), readyReq(0))
	if err != nil || res.Ready == nil || res.Ready.Waiting != 3 || res.Ready.Scanned != 3 || !res.Ready.More {
		t.Fatalf("%+v %v", res.Ready, err)
	}
}

// The plans the new predicates use (the index each rests on).
func TestReadyAndEntityReadsUseTheirIndexes(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	ctx := context.Background()
	for i := 0; i < 80; i++ {
		f.capture(t, readyOwner, readyComputer, fmt.Sprint("p", i), "P", "quorum plan "+fmt.Sprint(i), now.Add(-time.Duration(i)*time.Minute), false)
		ent := f.entityRow(t, readyOwner, fmt.Sprint("Planner ", i%4), types.EntityTypeOrganization)
		f.mention(t, fmt.Sprintf("obj:choir.publication:%s:s%d", readyOwner, i), ent, types.EntityRoleCentral)
	}
	f.pass(t)
	if err := f.store.ensureEdgesToKindCreatedIndex(ctx); err != nil {
		t.Fatal(err)
	}
	db := f.store.corpus()
	canonical, _ := entityCanonicalFor(readyOwner, types.MintedEntityID("Planner 1"))
	// A table of 80 rows prefers a bitmap scan, or the older index and a sort,
	// whatever the new index is worth. With bitmap scans and sorts off, only an
	// index that delivers the order itself can answer: the plan shows the
	// index serves the shape (the filter, the order and the limit) whole.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, set := range []string{`SET enable_bitmapscan = off`, `SET enable_sort = off`} {
		if _, err := conn.ExecContext(ctx, set); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, `RESET enable_bitmapscan`)
		_, _ = conn.ExecContext(ctx, `RESET enable_sort`)
	}()
	assertPlanIndex(t, "entity mentions newest", explainPlanOn(t, conn, `SELECT edge_id, from_id, metadata, created_at FROM og_edges
		WHERE to_id = ? AND kind = ? AND NOT tombstone AND from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'
		ORDER BY created_at DESC, edge_id LIMIT ?`, canonical, string(edgeMentionsEntity), 200), idxEdgesToKindCreated)
	// A second call finds it valid and builds nothing.
	if err := f.store.ensureEdgesToKindCreatedIndex(ctx); err != nil {
		t.Fatal(err)
	}
}

// readyScanMaxForTest moves the scan bound; the returned func restores it.
func readyScanMaxForTest(n int) func() {
	old := readyScanMax
	readyScanMax = n
	return func() { readyScanMax = old }
}

// explainPlanOn is explainPlan on a connection the caller has set up.
func explainPlanOn(t *testing.T, conn *sql.Conn, query string, args ...any) string {
	t.Helper()
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "RESET enable_seqscan") }()
	rows, err := conn.QueryContext(ctx, "EXPLAIN "+query, args...)
	if err != nil {
		t.Fatalf("explain %q: %v", query, err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
