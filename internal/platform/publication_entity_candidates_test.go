package platform

// The learned lexicon and its candidates read (docs/problems/related-stories-
// no-entity-layer-2026-10-26.md, slice c0 §2 and §8). Corpusd keeps each
// owner's gazetteer in memory, built from Store B on first use and updated in
// place by every write; a computer sends a story's title, text and the
// phrases it proposed, and gets back the known entities found (with world
// facts) and the phrases that are not entities yet, ranked and bounded.
// Ways it can fail, written before the code:
//
//  G5 Hubs crowd out the slots, or are dropped altogether.
//  G6 A mint in story A is not a hit in story B on the next request; a
//     corpusd restart has no lexicon; another owner's entities are offered.
//  G8 R1 misses a near form: a proposal that is an entity's name, or that
//     contains or is part of exactly one entity's name, is offered as a new
//     phrase beside it; a surname two entities share is given to one.
//  G9 The answer carries story text, or unbounded facts.
//  C1 The route answers a non-internal caller, a method other than POST, a
//     computer that is not platform tier, or a request over its bounds.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func candidatesReq(owner, title, text string) types.EntityCandidatesRequest {
	return types.EntityCandidatesRequest{
		OwnerID: owner, ComputerID: labComputer, Title: title, Text: text,
		Proposals: entitytext.WireProposals(entitytext.Propose(entitytext.Parse(title, text), nil)),
	}
}

func (l *relatedLab) candidates(svc *Service, owner, title, text string) types.EntityCandidatesResponse {
	l.t.Helper()
	resp, err := svc.EntityCandidates(context.Background(), candidatesReq(owner, title, text))
	if err != nil {
		l.t.Fatalf("entity candidates: %v", err)
	}
	return resp
}

func candidateBy(resp types.EntityCandidatesResponse, phraseOrID string) (types.EntityCandidate, bool) {
	for _, c := range resp.Candidates {
		if c.EntityID == phraseOrID || (c.EntityID == "" && entitytext.Key(c.Phrase) == entitytext.Key(phraseOrID)) {
			return c, true
		}
	}
	return types.EntityCandidate{}, false
}

func TestEntityCandidatesFindAMintInTheNextRequest(t *testing.T) { // G6
	l := newEntityLab(t)
	text := "Later, Halvorsen Dynamics shipped a new turbine. Engineers at Halvorsen Dynamics said tests went well."

	before := l.candidates(l.svc, labOwner, "Turbines ship", text)
	c, ok := candidateBy(before, "Halvorsen Dynamics")
	if !ok || c.Source != types.EntityCandidateExtracted || c.EntityID != "" || c.Count != 2 {
		t.Fatalf("before the mint: %+v (found %v), want an extracted phrase counted 2", c, ok)
	}

	a := l.publish("doc-a", 1, "Halvorsen story", l.capture("https://wire.example/a", "", "Halvorsen Dynamics shipped"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)

	after := l.candidates(l.svc, labOwner, "Turbines ship", text)
	c, ok = candidateBy(after, h.EntityID)
	if !ok || c.Source != types.EntityCandidateExisting || c.Name != "Halvorsen Dynamics" || c.Type != types.EntityTypeOrganization || c.Count != 2 {
		t.Fatalf("after the mint: %+v (found %v), want an existing hit", c, ok)
	}
	if _, dup := candidateBy(after, "Halvorsen Dynamics"); dup {
		t.Fatal("the mint is offered as a new phrase beside the hit")
	}
	if c.Facts == nil || c.Facts.Stories != 1 || c.Facts.CentralStories != 1 || len(c.Facts.PriorStories) != 1 || c.Facts.PriorStories[0].Title != "Halvorsen story" || c.Facts.FirstSeen.IsZero() {
		t.Fatalf("facts = %+v", c.Facts)
	}
	if after.Lexicon.Entities != 1 || after.Lexicon.Keys != 1 {
		t.Fatalf("lexicon size = %+v, want 1 entity and 1 key", after.Lexicon)
	}
}

func TestEntityCandidatesLexiconIsRebuiltAfterARestart(t *testing.T) { // G6
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Halvorsen story", l.capture("https://wire.example/a", "", "Halvorsen Dynamics shipped"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)

	restarted := NewService(l.store, l.artifactsRoot, filepath.Join(filepath.Dir(l.artifactsRoot), "signing.key"))
	restarted.relationTier = l.svc.relationTier
	resp := l.candidates(restarted, labOwner, "", "Later, Halvorsen Dynamics shipped.")
	if c, ok := candidateBy(resp, h.EntityID); !ok || c.Source != types.EntityCandidateExisting {
		t.Fatalf("after a restart: %+v, want the entity found", resp.Candidates)
	}
}

func TestEntityCandidatesNeverOfferAnotherOwnersEntities(t *testing.T) { // G6
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Halvorsen story", l.capture("https://wire.example/a", "", "Halvorsen Dynamics shipped"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)

	resp := l.candidates(l.svc, "owner-two", "", "Later, Halvorsen Dynamics shipped.")
	if _, ok := candidateBy(resp, h.EntityID); ok {
		t.Fatal("another owner's entity was offered")
	}
	if c, ok := candidateBy(resp, "Halvorsen Dynamics"); !ok || c.Source != types.EntityCandidateExtracted {
		t.Fatalf("for owner-two the phrase is new: %+v", resp.Candidates)
	}
	if resp.Lexicon.Entities != 0 {
		t.Fatalf("owner-two's lexicon = %+v", resp.Lexicon)
	}
}

func TestEntityCandidatesFoldNearFormsIntoTheOneEntity(t *testing.T) { // G8
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Names", l.capture("https://wire.example/a", "", "names"))
	vz := newEntity("Volodymyr Zelenskyy", types.EntityTypePerson, types.EntityRoleCentral)
	kw := newEntity("Kowalczyk", types.EntityTypePerson, types.EntityRoleIncidental)
	john := newEntity("John Smith", types.EntityTypePerson, types.EntityRoleIncidental)
	jane := newEntity("Jane Smith", types.EntityTypePerson, types.EntityRoleIncidental)
	l.attachEntities(a, vz, kw, john, jane)

	// The short form alone finds the one entity whose name it ends.
	resp := l.candidates(l.svc, labOwner, "", "Later, Zelenskyy spoke in Kyiv. Then Zelenskyy left.")
	c, ok := candidateBy(resp, vz.EntityID)
	if !ok || c.Source != types.EntityCandidateExisting || c.Count != 2 || !containsString(c.AliasCandidates, "Zelenskyy") {
		t.Fatalf("Zelenskyy alone: %+v (found %v), want the existing entity with the alias candidate", c, ok)
	}
	if p, ok := candidateBy(resp, "Zelenskyy"); ok && p.EntityID == "" {
		t.Fatal("the short form is also offered as a new phrase")
	}

	// A longer form around one entity's name folds into it.
	resp = l.candidates(l.svc, labOwner, "", "Later, Marta Kowalczyk spoke.")
	c, ok = candidateBy(resp, kw.EntityID)
	if !ok || !containsString(c.AliasCandidates, "Marta Kowalczyk") {
		t.Fatalf("Marta Kowalczyk: %+v (found %v), want Kowalczyk with the alias candidate", c, ok)
	}
	if p, ok := candidateBy(resp, "Marta Kowalczyk"); ok && p.EntityID == "" {
		t.Fatal("the longer form is also offered as a new phrase")
	}

	// A surname two entities share is given to neither.
	resp = l.candidates(l.svc, labOwner, "", "Later, Smith resigned.")
	if _, ok := candidateBy(resp, john.EntityID); ok {
		t.Fatal("a shared surname was given to John Smith")
	}
	if _, ok := candidateBy(resp, jane.EntityID); ok {
		t.Fatal("a shared surname was given to Jane Smith")
	}
	if c, ok := candidateBy(resp, "Smith"); !ok || c.Source != types.EntityCandidateExtracted {
		t.Fatalf("Smith: %+v, want an extracted phrase", resp.Candidates)
	}
}

func TestEntityCandidatesBoundHubsAndTheTotal(t *testing.T) { // G5
	l := newEntityLab(t)
	l.svc.related.tuning.HubEntityStories = 2
	var hubs []types.StoryEntity
	for i := 0; i < 6; i++ {
		hubs = append(hubs, newEntity(fmt.Sprintf("Hubcorp Number%c", 'A'+i), types.EntityTypeOrganization, types.EntityRoleCentral))
	}
	for i := 0; i < 3; i++ {
		s := l.publish(fmt.Sprintf("doc-%d", i), 1, fmt.Sprintf("Hub story %d", i), l.capture(fmt.Sprintf("https://wire.example/%d", i), "", fmt.Sprintf("hub %d", i)))
		l.attachEntities(s, hubs...)
	}
	var b strings.Builder
	for _, h := range hubs {
		fmt.Fprintf(&b, "Later, %s spoke twice; then %s spoke again. ", h.Name, h.Name)
	}
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "Later, Placename%c%c met. ", 'A'+i%26, 'a'+i/26)
	}
	resp := l.candidates(l.svc, labOwner, "", b.String())
	if n := len(resp.Candidates); n != types.EntityCandidatesMax {
		t.Fatalf("%d candidates, want %d", n, types.EntityCandidatesMax)
	}
	n := 0
	for _, c := range resp.Candidates {
		if c.Hub {
			n++
			if c.Source != types.EntityCandidateExisting {
				t.Fatalf("a hub that is not an existing entity: %+v", c)
			}
		}
	}
	if n != types.EntityCandidatesHubSlots {
		t.Fatalf("%d hubs kept, want %d", n, types.EntityCandidatesHubSlots)
	}
}

func TestEntityCandidatesCarryBoundedFactsAndNoStoryText(t *testing.T) { // G9
	l := newEntityLab(t)
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	for i := 0; i < 7; i++ {
		s := l.publish(fmt.Sprintf("doc-%d", i), 1, fmt.Sprintf("Halvorsen story %d", i), l.capture(fmt.Sprintf("https://wire.example/%d", i), "", fmt.Sprintf("story %d", i)))
		co := newEntity(fmt.Sprintf("Partner Number%c", 'A'+i), types.EntityTypeOrganization, types.EntityRoleCentral)
		l.attachEntities(s, h, co)
	}
	secret := "the quarterly turbine ledger shows unusual entries"
	resp := l.candidates(l.svc, labOwner, "", "Later, Halvorsen Dynamics shipped; "+secret+".")
	c, ok := candidateBy(resp, h.EntityID)
	if !ok || c.Facts == nil {
		t.Fatalf("no hit: %+v", resp.Candidates)
	}
	if c.Facts.Stories != 7 || c.Facts.CentralStories != 7 {
		t.Fatalf("facts = %+v, want 7 stories, 7 central", c.Facts)
	}
	if len(c.Facts.PriorStories) != types.EntityFactsMaxStories || len(c.Facts.CoEntities) != types.EntityFactsMaxCoEntities {
		t.Fatalf("facts not bounded: %d stories, %d co-entities", len(c.Facts.PriorStories), len(c.Facts.CoEntities))
	}
	for _, co := range c.Facts.CoEntities {
		if co.EntityID == h.EntityID || co.Name == "" {
			t.Fatalf("co-entity %+v", co)
		}
	}
	raw, _ := json.Marshal(resp)
	if bytes.Contains(raw, []byte("quarterly turbine ledger")) {
		t.Fatal("the answer carries story text")
	}
}

func TestEntityCandidatesRouteIsBoundAndBounded(t *testing.T) { // C1
	l := newEntityLab(t)
	h := NewHandler(l.svc)
	body, _ := json.Marshal(candidatesReq(labOwner, "", "Later, Halvorsen Dynamics shipped."))

	rec := httptest.NewRecorder()
	h.HandleStoryEntityCandidates(rec, httptest.NewRequest(http.MethodGet, types.StoryEntityCandidatesPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.HandleStoryEntityCandidates(rec, httptest.NewRequest(http.MethodPost, types.StoryEntityCandidatesPath, bytes.NewReader(body)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST without the internal header = %d", rec.Code)
	}
	post := func(req types.EntityCandidatesRequest) *httptest.ResponseRecorder {
		b, _ := json.Marshal(req)
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, types.StoryEntityCandidatesPath, bytes.NewReader(b))
		r.Header.Set("X-Internal-Caller", "true")
		h.HandleStoryEntityCandidates(rec, r)
		return rec
	}
	if rec := post(candidatesReq(labOwner, "", "Later, Halvorsen Dynamics shipped.")); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"candidates"`) {
		t.Fatalf("internal POST = %d %s", rec.Code, rec.Body.String())
	}
	other := candidatesReq(labOwner, "", "Later, Halvorsen Dynamics shipped.")
	other.ComputerID = "computer-reader"
	if rec := post(other); rec.Code != http.StatusBadRequest {
		t.Fatalf("a computer that is not platform tier = %d", rec.Code)
	}
	tooMany := candidatesReq(labOwner, "", "x")
	for i := 0; i <= entitytext.MaxProposals; i++ {
		tooMany.Proposals = append(tooMany.Proposals, types.EntityProposal{Phrase: fmt.Sprintf("Phrase Number%d", i), Count: 1})
	}
	if rec := post(tooMany); rec.Code != http.StatusBadRequest {
		t.Fatalf("%d proposals = %d", len(tooMany.Proposals), rec.Code)
	}
	long := candidatesReq(labOwner, "", "Later, Halvorsen Dynamics shipped. "+strings.Repeat("the market moved. ", entitytext.MaxTextRunes/18+5)+"Later, Quentaro Holdings shipped.")
	long.Proposals = append(long.Proposals, types.EntityProposal{Phrase: "Quentaro Holdings", Count: 1})
	rec = post(long)
	if rec.Code != http.StatusOK {
		t.Fatalf("a long text = %d %s", rec.Code, rec.Body.String())
	}
	var resp types.EntityCandidatesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := candidateBy(resp, "Quentaro Holdings"); ok {
		t.Fatal("a proposal not found in the bounded text was offered")
	}
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
