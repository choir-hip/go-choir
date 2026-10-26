package platform

// The lexicon's learning writes and the growth read
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0 §6,
// §7 and §8). A story's entities export carries the review that produced it:
// its counts by source, the phrases System 1 judged not to be entities, the
// aliases research's attachments taught. Corpusd records the review once per
// tape record, learns stopwords and aliases into the lexicon in place, and
// serves the daily growth series. Ways it can fail, written before the code:
//
//  G7 The stoplist blocks an entity's name or alias; a phrase is stopped
//     before N = 3 distinct stories at p >= 0.8; a revoked entry is relearned
//     at once (within 30 days).
//  G8 An alias that is another entity's name is recorded; an alias is not
//     matched by the next request; aliases grow without bound.
//  G11 Counts by source are lost, double counted on a re-export, or missing
//     from the growth read.
//  R  The growth read shows another owner's stories than the paper's, or
//     recomputes on every request.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func reviewReq(l *relatedLab, story *PublishTextureResponse, record string, review types.StoryEntityReview, entities ...types.StoryEntity) types.StoryEntitiesRequest {
	req := l.entitiesReq(story, entities...)
	review.RecordID = record
	req.Review = &review
	return req
}

func (l *relatedLab) record(req types.StoryEntitiesRequest) {
	l.t.Helper()
	if _, err := l.svc.RecordStoryEntities(context.Background(), req); err != nil {
		l.t.Fatalf("record story entities: %v", err)
	}
}

func (l *relatedLab) stories(n int) []*PublishTextureResponse {
	var out []*PublishTextureResponse
	for i := 0; i < n; i++ {
		out = append(out, l.publish(fmt.Sprintf("doc-learn-%d", i), 1, fmt.Sprintf("Learning story %d", i), l.capture(fmt.Sprintf("https://wire.example/learn-%d", i), "", fmt.Sprintf("learn %d", i))))
	}
	return out
}

func notEntity(phrase string, p float64) types.StoryEntityReview {
	return types.StoryEntityReview{NotEntities: []types.StoryEntityNotEntity{{Phrase: phrase, P: p}}}
}

func TestEntityStoplistLearnsAfterThreeStoriesAndNeverBlocksAnEntity(t *testing.T) { // G7
	l := newEntityLab(t)
	s := l.stories(4)
	text := "Later, Ukrainian forces advanced. Later, Ukrainian officials spoke."
	offered := func() bool {
		_, ok := candidateBy(l.candidates(l.svc, labOwner, "", text), "Ukrainian")
		return ok
	}
	if !offered() {
		t.Fatal("a phrase with no evidence is not offered")
	}
	l.record(reviewReq(l, s[0], "rec-0", notEntity("Ukrainian", 0.9)))
	l.record(reviewReq(l, s[0], "rec-0b", notEntity("Ukrainian", 0.95))) // the same story again
	l.record(reviewReq(l, s[1], "rec-1", notEntity("Ukrainian", 0.7)))   // under p 0.8
	l.record(reviewReq(l, s[2], "rec-2", notEntity("Ukrainian", 0.85)))
	if !offered() {
		t.Fatal("stopped before three distinct stories at p >= 0.8")
	}
	l.record(reviewReq(l, s[3], "rec-3", notEntity("ukrainian", 0.9)))
	if offered() {
		t.Fatal("not stopped after three distinct stories")
	}
	if got := l.candidates(l.svc, labOwner, "", text).Lexicon.Stopwords; got != 1 {
		t.Fatalf("stopwords = %d, want 1", got)
	}

	// An entity's name is never stopped, at write or at match.
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(s[0], h)
	for i := 0; i < 3; i++ {
		l.record(reviewReq(l, s[i], fmt.Sprintf("rec-h-%d", i), notEntity("Halvorsen Dynamics", 0.99)))
	}
	if c, ok := candidateBy(l.candidates(l.svc, labOwner, "", "Later, Halvorsen Dynamics shipped."), h.EntityID); !ok || c.Source != types.EntityCandidateExisting {
		t.Fatal("an entity's name was stopped")
	}
	// A stopped phrase later minted as an entity is matched as the entity.
	u := newEntity("Ukrainian", types.EntityTypeOrganization, types.EntityRoleIncidental)
	l.attachEntities(s[1], u)
	if _, ok := candidateBy(l.candidates(l.svc, labOwner, "", text), u.EntityID); !ok {
		t.Fatal("a stopword blocked an entity minted after it")
	}
}

func TestEntityStoplistRevokeHoldsForThirtyDays(t *testing.T) { // G7
	l := newEntityLab(t)
	clock := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	l.svc.lexicons.now = func() time.Time { return clock }
	s := l.stories(6)
	text := "Later, Ukrainian forces advanced."
	offered := func() bool {
		_, ok := candidateBy(l.candidates(l.svc, labOwner, "", text), "Ukrainian")
		return ok
	}
	for i := 0; i < 3; i++ {
		l.record(reviewReq(l, s[i], fmt.Sprintf("rec-%d", i), notEntity("Ukrainian", 0.9)))
	}
	if offered() {
		t.Fatal("not stopped")
	}
	if err := l.svc.RevokeEntityStopword(context.Background(), labOwner, "Ukrainian"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !offered() {
		t.Fatal("a revoked stopword still stops")
	}
	for i := 3; i < 6; i++ {
		l.record(reviewReq(l, s[i], fmt.Sprintf("rec-%d", i), notEntity("Ukrainian", 0.9)))
	}
	if !offered() {
		t.Fatal("a revoked stopword was relearned within 30 days")
	}
	clock = clock.Add(31 * 24 * time.Hour)
	l.record(reviewReq(l, s[5], "rec-late", notEntity("Ukrainian", 0.9)))
	if offered() {
		t.Fatal("after 30 days new evidence does not stop the phrase again")
	}
	// A restart reads the stoplist back.
	l.svc.lexicons = entityLexicons{now: l.svc.lexicons.now}
	if offered() {
		t.Fatal("the stoplist was lost on a restart")
	}
}

func TestEntityAliasesAreLearnedBoundedAndNeverAnotherEntitysName(t *testing.T) { // G8
	l := newEntityLab(t)
	s := l.stories(1)
	vz := newEntity("Volodymyr Zelenskyy", types.EntityTypePerson, types.EntityRoleCentral)
	ze := newEntity("Zelenska Foundation", types.EntityTypeOrganization, types.EntityRoleIncidental)
	l.attachEntities(s[0], vz, ze)

	review := types.StoryEntityReview{Aliases: []types.StoryEntityAlias{
		{EntityID: vz.EntityID, Alias: "Zelensky"},
		{EntityID: vz.EntityID, Alias: "Zelenska Foundation"},   // another entity's name
		{EntityID: vz.EntityID, Alias: "Volodymyr Zelenskyy's"}, // its own name
	}}
	l.record(reviewReq(l, s[0], "rec-a", review, types.StoryEntity{EntityID: vz.EntityID, Role: types.EntityRoleCentral}))

	resp := l.candidates(l.svc, labOwner, "", "Later, President Zelensky spoke.")
	if c, ok := candidateBy(resp, vz.EntityID); !ok || c.Count != 1 {
		t.Fatalf("the learned alias is not matched: %+v", resp.Candidates)
	}
	row, _ := l.entityRow(labOwner, vz.EntityID)
	var meta struct {
		Aliases []string `json:"aliases"`
	}
	_ = json.Unmarshal(row.Metadata, &meta)
	if len(meta.Aliases) != 1 || meta.Aliases[0] != "Zelensky" {
		t.Fatalf("aliases on the row = %v, want only Zelensky", meta.Aliases)
	}
	resp = l.candidates(l.svc, labOwner, "", "Later, Zelenska Foundation met.")
	if c, ok := candidateBy(resp, ze.EntityID); !ok || c.Ambiguous {
		t.Fatalf("another entity's name became an alias: %+v", resp.Candidates)
	}

	var many []types.StoryEntityAlias
	for i := 0; i < types.EntityAliasesMax+5; i++ {
		many = append(many, types.StoryEntityAlias{EntityID: vz.EntityID, Alias: fmt.Sprintf("Zelensky Form%c", 'A'+i)})
	}
	l.record(reviewReq(l, s[0], "rec-b", types.StoryEntityReview{Aliases: many}, types.StoryEntity{EntityID: vz.EntityID, Role: types.EntityRoleCentral}))
	row, _ = l.entityRow(labOwner, vz.EntityID)
	_ = json.Unmarshal(row.Metadata, &meta)
	if len(meta.Aliases) != types.EntityAliasesMax {
		t.Fatalf("%d aliases, want the bound %d", len(meta.Aliases), types.EntityAliasesMax)
	}
	// A restart reads aliases back.
	l.svc.lexicons = entityLexicons{}
	if _, ok := candidateBy(l.candidates(l.svc, labOwner, "", "Later, Zelensky spoke."), vz.EntityID); !ok {
		t.Fatal("aliases lost on a restart")
	}
}

func TestEntityReviewsCountOncePerRecordAndFeedTheGrowthRead(t *testing.T) { // G11, R
	l := newEntityLab(t)
	l.svc.SetAutopaperOwners([]string{labOwner})
	s := l.stories(2)
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	review := types.StoryEntityReview{
		Counts:       types.StoryEntityCounts{Candidates: 10, Hits: 2, Existing: 2, Extracted: 3, ResearchNamed: 1, Overridden: 1, CategoryGap: 1, ExistingUnmatched: 1},
		CategoryGaps: []string{"event"},
		NoveltyYes:   0.4,
	}
	l.record(reviewReq(l, s[0], "rec-0", review, h))
	l.record(reviewReq(l, s[0], "rec-0", review, h)) // a re-export of the same record
	review.NoveltyYes = 0.1
	l.record(reviewReq(l, s[1], "rec-1", review, types.StoryEntity{EntityID: h.EntityID, Role: types.EntityRoleCentral}))

	g, err := l.svc.EntityGrowth(context.Background(), 7)
	if err != nil {
		t.Fatalf("growth: %v", err)
	}
	if len(g.Days) != 7 {
		t.Fatalf("%d days, want 7", len(g.Days))
	}
	today := g.Days[len(g.Days)-1]
	if today.StoriesReviewed != 2 || today.Attachments.Existing != 4 || today.Attachments.Extracted != 6 || today.Attachments.ResearchNamed != 2 || today.ExistingUnmatched != 2 {
		t.Fatalf("today = %+v, want two reviews counted once each", today)
	}
	if today.CategoryGaps["event"] != 2 || today.NoveltyYesRate != 0.5 {
		t.Fatalf("gaps %v, novelty yes rate %v (want 2 and 0.5: one of two at p >= 0.35)", today.CategoryGaps, today.NoveltyYesRate)
	}
	if today.KeywordsAdded != 1 || g.Lexicon.Entities != 1 {
		t.Fatalf("keywords added %d, lexicon %+v", today.KeywordsAdded, g.Lexicon)
	}

	// Cached: a new review is not seen until the cache expires.
	l.record(reviewReq(l, s[1], "rec-2", review))
	again, _ := l.svc.EntityGrowth(context.Background(), 7)
	if again.Days[len(again.Days)-1].StoriesReviewed != 2 {
		t.Fatal("the growth read recomputed inside its cache window")
	}

	rec := httptest.NewRecorder()
	NewHandler(l.svc).HandlePublicEntityGrowth(rec, httptest.NewRequest(http.MethodGet, "/api/platform/entities/growth?days=3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public growth = %d %s", rec.Code, rec.Body.String())
	}
	var pub types.EntityGrowthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pub); err != nil || len(pub.Days) != 3 {
		t.Fatalf("public growth body = %s", rec.Body.String())
	}
}

// R: the growth read is unauthenticated, so it aggregates the paper owners
// alone: another owner's reviews, mints and stopwords never reach it, and
// with no paper owner configured it shows nothing.
func TestEntityGrowthShowsThePaperOwnersAlone(t *testing.T) {
	l := newEntityLab(t)
	const other = "owner-two"
	l.svc.SetAutopaperOwners([]string{labOwner, other})
	theirs := l.publishAs(other, PublishModeAuto, nil, "doc-other", 1, "Other story", l.captureAs(other, labComputer, "https://wire.example/other", "", "other"))
	req := types.StoryEntitiesRequest{OwnerID: other, ComputerID: labComputer, Story: theirs.PublicationID,
		Entities: []types.StoryEntity{newEntity("Private Holdings", types.EntityTypeOrganization, types.EntityRoleCentral)}}
	for i, s := range []string{"rec-o0", "rec-o1", "rec-o2"} {
		r := req
		r.Review = &types.StoryEntityReview{RecordID: s, Counts: types.StoryEntityCounts{Existing: 7}, NotEntities: []types.StoryEntityNotEntity{{Phrase: "Secretphrase", P: 0.9}}}
		if i > 0 {
			r.Entities = nil
		}
		l.record(r)
	}

	// The control: while owner-two is a paper owner, its learning shows.
	both, err := l.svc.EntityGrowth(context.Background(), 3)
	if err != nil || both.Days[2].Attachments.Existing != 21 || both.Lexicon.Entities != 1 {
		t.Fatalf("control read = %+v, %v", both, err)
	}

	l.svc.growth = growthCache{}
	l.svc.SetAutopaperOwners(nil)
	empty, err := l.svc.EntityGrowth(context.Background(), 3)
	if err != nil {
		t.Fatalf("growth: %v", err)
	}
	if empty.Lexicon.Entities != 0 || len(empty.Stoplist) != 0 || empty.Days[2].StoriesReviewed != 0 {
		t.Fatalf("with no paper owner configured the read shows %+v", empty)
	}

	l.svc.growth = growthCache{}
	l.svc.SetAutopaperOwners([]string{labOwner})
	rec := httptest.NewRecorder()
	NewHandler(l.svc).HandlePublicEntityGrowth(rec, httptest.NewRequest(http.MethodGet, "/api/platform/entities/growth?days=3", nil))
	var g types.EntityGrowthResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &g) != nil {
		t.Fatalf("anonymous growth = %d %s", rec.Code, rec.Body.String())
	}
	if g.Lexicon.Entities != 0 || len(g.Stoplist) != 0 || g.Days[2].StoriesReviewed != 0 || g.Days[2].Attachments.Existing != 0 || g.Days[2].Minted != 0 {
		t.Fatalf("another owner's learning reached the public read: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("Secretphrase")) || bytes.Contains(rec.Body.Bytes(), []byte("Private Holdings")) {
		t.Fatalf("another owner's phrase or entity name is in the public read")
	}
}
