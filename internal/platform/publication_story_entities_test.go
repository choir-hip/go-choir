package platform

// The entity layer's write route (docs/problems/related-stories-no-entity-layer-2026-10-26.md,
// slice a; ontology docs/reports/claims-events-entities-ontology-2026-10-14.md
// D3/D4: the paper's entities are public rows in Store B). The research desk
// attaches a published story's entities (choir.Relate); the computer exports
// them here as choir.entity rows and mentions_entity edges from the
// publication. Ways it can fail, written before the code:
//
//  N1 The route accepts a computer that is not platform tier, a story of
//     another owner, a story that is not a published Autopaper story or not
//     on the public route, more than 12 entities, an unknown role or type, a
//     malformed entity id, or a new entity whose id is not the one its name
//     (or Wikidata id) derives.
//  N2 Sprawl (R1): a new entity whose normalised name matches an existing
//     entity of another id is minted beside it. Refused with the match named.
//     The same id again is the same node, not a duplicate.
//  N3 An attachment names an entity id Store B does not hold.
//  N4 A repeat writes a second entity row or a second edge; a role change
//     leaves the old role's edge live.
//  N5 The validate-only call writes, skips a check the write runs, or
//     answers like a record.
//  N6 The route answers a non-internal caller or any method but POST.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func newEntityLab(t *testing.T) *relatedLab {
	t.Helper()
	l := newRealisticLab(t)
	l.svc.relationTier = func(computerID string) (bool, error) { return computerID == labComputer, nil }
	return l
}

func newEntity(name, typ, role string) types.StoryEntity {
	return types.StoryEntity{EntityID: types.MintedEntityID(name), Name: name, Type: typ, New: true, Role: role}
}

func (l *relatedLab) entitiesReq(story *PublishTextureResponse, entities ...types.StoryEntity) types.StoryEntitiesRequest {
	return types.StoryEntitiesRequest{OwnerID: labOwner, ComputerID: labComputer, Story: story.PublicationID, Entities: entities}
}

func (l *relatedLab) attachEntities(story *PublishTextureResponse, entities ...types.StoryEntity) {
	l.t.Helper()
	if _, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(story, entities...)); err != nil {
		l.t.Fatalf("attach entities to %s: %v", story.PublicationID, err)
	}
}

func (l *relatedLab) entityRow(owner, entityID string) (objectgraph.Object, bool) {
	l.t.Helper()
	id, err := entityCanonicalFor(owner, entityID)
	if err != nil {
		l.t.Fatalf("entity canonical id of %s: %v", entityID, err)
	}
	obj, err := l.svc.ogStore().GetObject(context.Background(), id)
	if err != nil {
		return objectgraph.Object{}, false
	}
	return obj, true
}

func (l *relatedLab) mentionEdges(publicationID string, live bool) []objectgraph.Edge {
	l.t.Helper()
	tomb := !live
	edges, err := l.svc.ogStore().ListEdges(context.Background(), objectgraph.EdgeFilter{FromID: l.canon(labOwner, publicationID), Kind: edgeMentionsEntity, Tombstone: &tomb, Limit: 200})
	if err != nil {
		l.t.Fatalf("list mentions_entity edges: %v", err)
	}
	return edges
}

func edgeRole(e objectgraph.Edge) string {
	var meta struct {
		Role string `json:"role"`
	}
	_ = json.Unmarshal(e.Metadata, &meta)
	return meta.Role
}

func TestStoryEntitiesWriteTheEntityRowAndTheMentionEdge(t *testing.T) { // N4
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded the election"))
	vucic := newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral)
	serbia := newEntity("Serbia", types.EntityTypePlace, types.EntityRoleIncidental)

	n, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(a, vucic, serbia))
	if err != nil || n != 2 {
		t.Fatalf("record = %d, %v; want 2", n, err)
	}
	row, ok := l.entityRow(labOwner, vucic.EntityID)
	if !ok || row.ObjectKind != "choir.entity" || row.OwnerID != labOwner {
		t.Fatalf("entity row = %+v (found %v), want a choir.entity of the owner", row, ok)
	}
	var meta map[string]any
	if err := json.Unmarshal(row.Metadata, &meta); err != nil || meta["name"] != "Aleksandar Vucic" || meta["type"] != "person" || meta["entity_id"] != vucic.EntityID || meta["normalized"] != types.NormalizeEntityName("Aleksandar Vucic") {
		t.Fatalf("entity metadata = %s", row.Metadata)
	}
	edges := l.mentionEdges(a.PublicationID, true)
	if len(edges) != 2 {
		t.Fatalf("%d live mentions_entity edges, want 2", len(edges))
	}
	roles := map[string]string{}
	for _, e := range edges {
		roles[e.ToID] = edgeRole(e)
	}
	vid, _ := entityCanonicalFor(labOwner, vucic.EntityID)
	sid, _ := entityCanonicalFor(labOwner, serbia.EntityID)
	if roles[vid] != "central" || roles[sid] != "incidental" {
		t.Fatalf("roles = %v", roles)
	}

	// A repeat, new or by id, writes nothing new.
	l.attachEntities(a, vucic, serbia)
	l.attachEntities(a, types.StoryEntity{EntityID: vucic.EntityID, Role: types.EntityRoleCentral})
	if got := l.mentionEdges(a.PublicationID, true); len(got) != 2 {
		t.Fatalf("after repeats %d live edges, want 2", len(got))
	}
	rows, err := l.svc.ogStore().ListObjects(context.Background(), objectgraph.ListFilter{Kind: "choir.entity", OwnerID: labOwner, Limit: 50})
	if err != nil || len(rows) != 2 {
		t.Fatalf("%d entity rows after repeats (%v), want 2", len(rows), err)
	}

	// A role change leaves one live edge, with the new role.
	l.attachEntities(a, types.StoryEntity{EntityID: serbia.EntityID, Role: types.EntityRoleCentral})
	live := 0
	for _, e := range l.mentionEdges(a.PublicationID, true) {
		if e.ToID == sid {
			live++
			if edgeRole(e) != "central" {
				t.Fatalf("Serbia's live edge has role %q, want central", edgeRole(e))
			}
		}
	}
	if live != 1 {
		t.Fatalf("Serbia has %d live edges after the role change, want 1", live)
	}
}

func TestStoryEntitiesRefuseADuplicateOfAnotherID(t *testing.T) { // N2, N3
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	b := l.publish("doc-b", 1, "Opposition wins", l.capture("https://other.example/b", "", "The opposition won"))
	l.attachEntities(a, newEntity("Anthropic", types.EntityTypeOrganization, types.EntityRoleCentral))

	// The same normalised name is the same id: attached, not refused.
	same := newEntity("Anthropic PBC", types.EntityTypeOrganization, types.EntityRoleCentral)
	if same.EntityID != types.MintedEntityID("Anthropic") {
		t.Fatalf("fixture: %s and Anthropic mint different ids", same.Name)
	}
	l.attachEntities(b, same)

	// A Wikidata node whose name matches the minted one is a second node: refused, the match named.
	wd := types.StoryEntity{EntityID: types.WikidataEntityID("Q116758847"), Name: "Anthropic", Type: types.EntityTypeOrganization, WikidataID: "Q116758847", New: true, Role: types.EntityRoleCentral}
	_, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(b, wd))
	if err == nil || !strings.Contains(err.Error(), types.MintedEntityID("Anthropic")) || !strings.Contains(err.Error(), "entity_id") {
		t.Fatalf("wikidata duplicate: err = %v, want the existing id named and attach-by-entity_id", err)
	}
	if _, ok := l.entityRow(labOwner, wd.EntityID); ok {
		t.Fatal("a refused duplicate was written")
	}

	// An id Store B does not hold.
	_, err = l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(b, types.StoryEntity{EntityID: types.MintedEntityID("Nobody"), Role: types.EntityRoleCentral}))
	if err == nil || !strings.Contains(err.Error(), "unknown entity") {
		t.Fatalf("unknown id: err = %v", err)
	}
}

// Two new entities of one normalised name in one request (review 298 F1):
// the committed rows hold neither yet, so the check must also compare the
// request's own new names.
func TestStoryEntitiesRefuseTwoNewDuplicatesInOneRequest(t *testing.T) { // N2
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Anthropic raises", l.capture("https://wire.example/a", "", "Anthropic raised money"))
	minted := newEntity("Anthropic", types.EntityTypeOrganization, types.EntityRoleCentral)
	wd := types.StoryEntity{EntityID: types.WikidataEntityID("Q116758847"), Name: "Anthropic, Inc.", Type: types.EntityTypeOrganization, WikidataID: "Q116758847", New: true, Role: types.EntityRoleIncidental}
	for _, order := range [][]types.StoryEntity{{minted, wd}, {wd, minted}} {
		for _, validate := range []bool{true, false} {
			req := l.entitiesReq(a, order...)
			req.ValidateOnly = validate
			_, err := l.svc.RecordStoryEntities(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), order[0].EntityID) || !strings.Contains(err.Error(), "this request") {
				t.Fatalf("validate=%v order %s, %s: err = %v, want the second refused with the first id named", validate, order[0].EntityID, order[1].EntityID, err)
			}
		}
		for _, e := range order {
			if _, ok := l.entityRow(labOwner, e.EntityID); ok {
				t.Fatalf("a refused request wrote %s", e.EntityID)
			}
		}
	}
	// One of them alone is taken.
	if _, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(a, minted)); err != nil {
		t.Fatal(err)
	}
}

// R1 is a seek on the lookup key, not a scan of every entity's metadata
// (review 298 F2).
func TestStoryEntitiesDuplicateCheckIsALookupKey(t *testing.T) {
	if !lookupKeyFor(string(ogKindEntity), "$.normalized") {
		t.Fatal("choir.entity $.normalized is not a registered lookup key")
	}
}

func TestStoryEntitiesRefuseAMalformedOrForeignRequest(t *testing.T) { // N1
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	private := l.publishAs(labOwner, PublishModeAuto, json.RawMessage(`{"visibility":"private","route":"public"}`), "doc-private", 1, "Private", l.capture("https://wire.example/p", "", "private source"))
	ownerPub := l.publishAs(labOwner, PublishModeOwner, nil, "doc-owner", 1, "Owner published", l.capture("https://wire.example/o", "", "owner source"))
	other := l.publishAs("other-owner", PublishModeAuto, nil, "doc-other", 1, "Other owner", l.captureAs("other-owner", labComputer, "https://wire.example/x", "", "other source"))
	ok := newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral)
	many := make([]types.StoryEntity, 0, 13)
	for i := 0; i < 13; i++ {
		many = append(many, newEntity("Entity "+strings.Repeat("x", i+1), types.EntityTypePerson, types.EntityRoleCentral))
	}
	with := func(f func(*types.StoryEntity)) types.StoryEntity {
		e := ok
		f(&e)
		return e
	}
	cases := []struct {
		name string
		req  types.StoryEntitiesRequest
		want string
	}{
		{"not platform tier", func() types.StoryEntitiesRequest {
			r := l.entitiesReq(a, ok)
			r.ComputerID = "private-computer"
			return r
		}(), "platform"},
		{"no owner", func() types.StoryEntitiesRequest { r := l.entitiesReq(a, ok); r.OwnerID = ""; return r }(), "owner_id"},
		{"no entities", l.entitiesReq(a), "1 to 12"},
		{"13 entities", l.entitiesReq(a, many...), "1 to 12"},
		{"malformed story", func() types.StoryEntitiesRequest { r := l.entitiesReq(a, ok); r.Story = "pub-1"; return r }(), "publication"},
		{"another owner's story", l.entitiesReq(other, ok), "not a published Autopaper story"},
		{"owner published, not Autopaper", l.entitiesReq(ownerPub, ok), "not a published Autopaper story"},
		{"not on the public route", l.entitiesReq(private, ok), "public route"},
		{"unknown role", l.entitiesReq(a, with(func(e *types.StoryEntity) { e.Role = "mentioned" })), "role"},
		{"unknown type", l.entitiesReq(a, with(func(e *types.StoryEntity) { e.Type = "topic" })), "type"},
		{"malformed id", l.entitiesReq(a, with(func(e *types.StoryEntity) { e.EntityID = "ent:xyz" })), "entity id"},
		{"id not derived from the name", l.entitiesReq(a, with(func(e *types.StoryEntity) { e.EntityID = types.MintedEntityID("Someone Else") })), "derive"},
		{"wikidata id not derived", l.entitiesReq(a, with(func(e *types.StoryEntity) { e.EntityID, e.WikidataID = "wd:Q1", "Q2" })), "derive"},
		{"one entity twice", l.entitiesReq(a, ok, with(func(e *types.StoryEntity) { e.Role = types.EntityRoleIncidental })), "twice"},
	}
	for _, tc := range cases {
		if _, err := l.svc.RecordStoryEntities(context.Background(), tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
	if rows, _ := l.svc.ogStore().ListObjects(context.Background(), objectgraph.ListFilter{Kind: "choir.entity", Limit: 50}); len(rows) != 0 {
		t.Fatalf("refused requests wrote %d entity rows", len(rows))
	}
}

func TestStoryEntitiesValidateOnlyRunsEveryCheckAndWritesNothing(t *testing.T) { // N5
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	l.attachEntities(a, newEntity("Anthropic", types.EntityTypeOrganization, types.EntityRoleCentral))

	req := l.entitiesReq(a, newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral))
	req.ValidateOnly = true
	if n, err := l.svc.RecordStoryEntities(context.Background(), req); err != nil || n != 1 {
		t.Fatalf("validate = %d, %v; want 1", n, err)
	}
	if _, ok := l.entityRow(labOwner, types.MintedEntityID("Aleksandar Vucic")); ok {
		t.Fatal("validate-only wrote an entity row")
	}
	if got := l.mentionEdges(a.PublicationID, true); len(got) != 1 {
		t.Fatalf("validate-only wrote edges: %d live, want the 1 from before", len(got))
	}
	dup := l.entitiesReq(a, types.StoryEntity{EntityID: "wd:Q116758847", Name: "Anthropic", Type: types.EntityTypeOrganization, WikidataID: "Q116758847", New: true, Role: types.EntityRoleCentral})
	dup.ValidateOnly = true
	if _, err := l.svc.RecordStoryEntities(context.Background(), dup); err == nil {
		t.Fatal("validate-only took a duplicate the write refuses")
	}

	// Over HTTP the validate-only answer says validated, never recorded.
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, types.StoryEntitiesPath, bytes.NewReader(body))
	r.Header.Set("X-Internal-Caller", "true")
	NewHandler(l.svc).HandleStoryEntities(rec, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"validated"`) || strings.Contains(rec.Body.String(), `"recorded"`) {
		t.Fatalf("validate-only over HTTP = %d %s", rec.Code, rec.Body.String())
	}
}

func TestStoryEntitiesRouteIsInternalAndPostOnly(t *testing.T) { // N6
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	body, _ := json.Marshal(l.entitiesReq(a, newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral)))

	rec := httptest.NewRecorder()
	NewHandler(l.svc).HandleStoryEntities(rec, httptest.NewRequest(http.MethodGet, types.StoryEntitiesPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	NewHandler(l.svc).HandleStoryEntities(rec, httptest.NewRequest(http.MethodPost, types.StoryEntitiesPath, bytes.NewReader(body)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST without the internal header = %d, want 403", rec.Code)
	}
	rec = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, types.StoryEntitiesPath, bytes.NewReader(body))
	r.Header.Set("X-Internal-Caller", "true")
	NewHandler(l.svc).HandleStoryEntities(rec, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recorded":1`) {
		t.Fatalf("internal POST = %d %s", rec.Code, rec.Body.String())
	}
}

// Review 298 (DeepSeek seat) item 4: the route enforces the name bound the
// verb does.
func TestStoryEntitiesRefuseAnOverlongName(t *testing.T) {
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Long", l.capture("https://wire.example/a", "", "long"))
	name := strings.Repeat("Ä", types.EntityNameMaxRunes+1)
	_, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(a, newEntity(name, types.EntityTypeOrganization, types.EntityRoleCentral)))
	if err == nil || !strings.Contains(err.Error(), "characters") {
		t.Fatalf("a %d-character name: err = %v", types.EntityNameMaxRunes+1, err)
	}
	if _, err := l.svc.RecordStoryEntities(context.Background(), l.entitiesReq(a, newEntity(strings.Repeat("Ä", types.EntityNameMaxRunes), types.EntityTypeOrganization, types.EntityRoleCentral))); err != nil {
		t.Fatalf("a name at the bound: %v", err)
	}
}

// The computer names a story by the id corpusd gives its publication (slice
// c: a story desk passes "" and the host derives it without a read).
func TestStoryPublicationIDMatchesThePlatform(t *testing.T) {
	for _, c := range [][2]string{{"owner-1", "doc-1"}, {labOwner, "doc-a"}} {
		ident, err := publicationIdentityFor(c[0], c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := types.StoryPublicationID(c[0], c[1]); got != ident.PublicationID {
			t.Fatalf("types.StoryPublicationID(%q, %q) = %s, platform %s", c[0], c[1], got, ident.PublicationID)
		}
	}
}

// Slice c, F6 and review 304 item 4: a story not yet published is checked
// with pending, validate-only: every entity check runs (types, ids, R1) and
// only the published-story check is skipped. Pending is never a write.
func TestStoryEntitiesPendingIsValidateOnlyAndKeepsEveryEntityCheck(t *testing.T) {
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	l.attachEntities(a, newEntity("Anthropic", types.EntityTypeOrganization, types.EntityRoleCentral))
	unpublished := types.StoryPublicationID(labOwner, "doc-unpublished")
	req := types.StoryEntitiesRequest{OwnerID: labOwner, ComputerID: labComputer, Story: unpublished, Pending: true, ValidateOnly: true,
		Entities: []types.StoryEntity{newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)}}
	if n, err := l.svc.RecordStoryEntities(context.Background(), req); err != nil || n != 1 {
		t.Fatalf("pending validate of an unpublished story = %d, %v; want 1", n, err)
	}
	dup := req
	dup.Entities = []types.StoryEntity{{EntityID: "wd:Q116758847", Name: "Anthropic", Type: types.EntityTypeOrganization, WikidataID: "Q116758847", New: true, Role: types.EntityRoleCentral}}
	if _, err := l.svc.RecordStoryEntities(context.Background(), dup); err == nil || !strings.Contains(err.Error(), "matches the existing entity") {
		t.Fatalf("pending skipped R1: %v", err)
	}
	bad := req
	bad.Entities = []types.StoryEntity{{EntityID: types.MintedEntityID("X Corp"), Name: "X Corp", Type: "event", New: true, Role: types.EntityRoleCentral}}
	if _, err := l.svc.RecordStoryEntities(context.Background(), bad); err == nil {
		t.Fatal("pending skipped the type check")
	}
	write := req
	write.ValidateOnly = false
	if _, err := l.svc.RecordStoryEntities(context.Background(), write); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("a pending write was accepted: %v", err)
	}
	notPub := req
	notPub.Pending = false
	if _, err := l.svc.RecordStoryEntities(context.Background(), notPub); err == nil {
		t.Fatal("an unpublished story was validated without pending")
	}
	if _, ok := l.entityRow(labOwner, types.MintedEntityID("Halvorsen Dynamics")); ok {
		t.Fatal("pending validation wrote a row")
	}
}

// Review 304 item 2: the computer frees a story's final Report only when the
// platform fails, so the route tells its failures from its refusals: a check
// the request fails is a 400, a store that cannot be read or written is a
// 5xx.
func TestStoryEntitiesStoreFailureIsAServerErrorAndARefusalIsNot(t *testing.T) {
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded"))
	l.attachEntities(a, newEntity("Anthropic", types.EntityTypeOrganization, types.EntityRoleCentral))
	post := func(h *Handler, req types.StoryEntitiesRequest) *httptest.ResponseRecorder {
		body, _ := json.Marshal(req)
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, types.StoryEntitiesPath, bytes.NewReader(body))
		r.Header.Set("X-Internal-Caller", "true")
		h.HandleStoryEntities(rec, r)
		return rec
	}
	dup := l.entitiesReq(a, types.StoryEntity{EntityID: "wd:Q116758847", Name: "Anthropic", Type: types.EntityTypeOrganization, WikidataID: "Q116758847", New: true, Role: types.EntityRoleCentral})
	if rec := post(NewHandler(l.svc), dup); rec.Code != http.StatusBadRequest {
		t.Fatalf("a duplicate = %d %s, want 400", rec.Code, rec.Body.String())
	}
	broken := &Service{relationTier: l.svc.relationTier}
	if rec := post(NewHandler(broken), l.entitiesReq(a, newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral))); rec.Code < 500 {
		t.Fatalf("no store = %d %s, want a 5xx", rec.Code, rec.Body.String())
	}
}
