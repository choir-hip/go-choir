package platform

// The public read of one entity (entity_page.go). Failure list, written
// before the code:
//
//  P1 an entity of the paper is not served with its stories, newest first,
//     with the role in each, or its counts are wrong;
//  P2 a person with no Wikidata id and fewer than two central public stories
//     is served (owner question 1), or a person who meets either rule is not;
//     organisations and places are not always served;
//  P3 an unknown id answers differently from a person with no page (404 both),
//     or a malformed id reaches a read;
//  P4 a story that is not on the public route is listed;
//  P5 the stories bound is not enforced, or a stranger can ask for more than
//     entityPageMaxStories;
//  P6 a tentative mention is listed as a story or counted;
//  P7 an internal route answers a caller that is not internal.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func newPageLab(t *testing.T) *relatedLab {
	t.Helper()
	l := newEntityLab(t)
	l.svc.SetAutopaperOwners([]string{labOwner})
	return l
}

func (l *relatedLab) page(id string, stories int) *EntityPage {
	l.t.Helper()
	// The read is cached for a minute; a test that changes the graph between
	// reads starts each read clean (the cache has its own test).
	l.svc.entityPages.mu.Lock()
	l.svc.entityPages.m = nil
	l.svc.entityPages.mu.Unlock()
	p, err := l.svc.EntityPage(context.Background(), id, stories)
	if err != nil {
		l.t.Fatalf("EntityPage(%s): %v", id, err)
	}
	return p
}

func TestEntityPageServesAnOrganisationWithItsStoriesNewestFirst(t *testing.T) { // P1
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Harbor reopens", l.capture("https://wire.example/a", "", "Harbor reopens"))
	b := l.publish("doc-b", 1, "Turbines ship", l.capture("https://wire.example/b", "", "Turbines ship"))
	c := l.publish("doc-c", 1, "Bridge closes", l.capture("https://wire.example/c", "", "Bridge closes"))
	l.attachEntities(a, newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral))
	l.attachEntities(b, newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleIncidental), newEntity("Quay Authority", types.EntityTypeOrganization, types.EntityRoleCentral))
	l.attachEntities(c, newEntity("Quay Authority", types.EntityTypeOrganization, types.EntityRoleCentral))
	id := types.MintedEntityID("Halvorsen Dynamics")
	p := l.page(id, 0)
	if p == nil || p.Name != "Halvorsen Dynamics" || p.Type != types.EntityTypeOrganization || p.Mentions != 2 || p.CentralMentions != 1 || p.CountsAtLeast {
		t.Fatalf("page = %+v", p)
	}
	if len(p.StoryList) != 2 || p.StoryList[0].PublicationID != b.PublicationID || p.StoryList[0].Role != types.EntityRoleIncidental ||
		p.StoryList[1].PublicationID != a.PublicationID || p.StoryList[1].Role != types.EntityRoleCentral || p.StoryList[0].Route.Bare == "" {
		t.Fatalf("stories = %+v, want b (incidental) then a (central)", p.StoryList)
	}
	// The entities that share its stories: Quay Authority is central to b.
	if len(p.CoEntities) != 1 || p.CoEntities[0].Name != "Quay Authority" {
		t.Fatalf("co-entities = %+v", p.CoEntities)
	}
	if p1 := l.page(id, 1); len(p1.StoryList) != 1 || p1.Mentions != 2 {
		t.Fatalf("one story asked for: %+v", p1)
	}
}

func TestEntityPageServesAPersonOnlyWithAWikidataIdOrTwoCentralStories(t *testing.T) { // P2 P3
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Story A", l.capture("https://wire.example/a", "", "Story A"))
	b := l.publish("doc-b", 1, "Story B", l.capture("https://wire.example/b", "", "Story B"))
	c := l.publish("doc-c", 1, "Story C", l.capture("https://wire.example/c", "", "Story C"))
	jane := newEntity("Jane Roe", types.EntityTypePerson, types.EntityRoleCentral)
	pat := types.StoryEntity{EntityID: types.WikidataEntityID("Q4242"), Name: "Pat Poe", Type: types.EntityTypePerson, WikidataID: "Q4242", New: true, Role: types.EntityRoleCentral}
	sam := newEntity("Sam Doe", types.EntityTypePerson, types.EntityRoleCentral)
	l.attachEntities(a, jane, pat, sam)
	if l.page(jane.EntityID, 0) != nil {
		t.Fatal("a person with one central story has a page")
	}
	if p := l.page(pat.EntityID, 0); p == nil || p.WikidataID != "Q4242" || len(p.StoryList) != 1 {
		t.Fatalf("a person with a Wikidata id has no page: %+v", p)
	}
	// An incidental mention is not a subject: Jane is incidental in b.
	l.attachEntities(b, types.StoryEntity{EntityID: jane.EntityID, Role: types.EntityRoleIncidental})
	if l.page(jane.EntityID, 0) != nil {
		t.Fatal("an incidental mention made a page")
	}
	l.attachEntities(c, types.StoryEntity{EntityID: jane.EntityID, Role: types.EntityRoleCentral})
	if p := l.page(jane.EntityID, 0); p == nil || p.CentralMentions != 2 || len(p.StoryList) != 3 {
		t.Fatalf("a person central in two stories has no page: %+v", p)
	}
	// A person with a page is a co-entity; one without is left out.
	l.attachEntities(a, types.StoryEntity{EntityID: types.MintedEntityID("Halvorsen Dynamics"), Name: "Halvorsen Dynamics", Type: types.EntityTypeOrganization, New: true, Role: types.EntityRoleCentral})
	org := l.page(types.MintedEntityID("Halvorsen Dynamics"), 0)
	if org == nil {
		t.Fatal("an organisation has no page")
	}
	for _, co := range org.CoEntities {
		if co.EntityID == sam.EntityID {
			t.Fatalf("a person without a page is a co-entity: %+v", org.CoEntities)
		}
	}
	// P3: an unknown id has no page, the same as a person without one.
	if l.page(types.MintedEntityID("Nobody Known"), 0) != nil || l.page(sam.EntityID, 0) != nil {
		t.Fatal("an unknown entity or a person without a page was served")
	}
	if _, err := l.svc.EntityPage(context.Background(), "ent:../etc", 0); err == nil {
		t.Fatal("a malformed id was read")
	}
}

func TestEntityPageLeavesOutStoriesNotOnThePublicRouteAndTentativeMentions(t *testing.T) { // P4 P6
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Public story", l.capture("https://wire.example/a", "", "Public story"))
	private := l.publishAs(labOwner, PublishModeAuto, json.RawMessage(`{"visibility":"private","route":"public"}`), "doc-p", 1, "Private story", l.capture("https://wire.example/p", "", "Private story"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)
	canonical, _ := entityCanonicalFor(labOwner, h.EntityID)
	// The export refuses a private story's entities; the read leaves such a
	// story out even if an edge for it is there.
	privCanonical, err := publicationCanonicalFor(labOwner, private.PublicationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.svc.ogStore().PutEdge(context.Background(), makeEdge(privCanonical, canonical, edgeMentionsEntity, time.Now().UTC(), mentionMetadata(types.EntityRoleCentral))); err != nil {
		t.Fatal(err)
	}
	// A tentative edge from a capture is neither a story nor counted.
	cap := l.capture("https://wire.example/t", "", "Tentative capture")
	if err := l.svc.ogStore().PutEdge(context.Background(), makeEdge(cap, canonical, edgeMentionsEntity, time.Now().UTC(), mentionMetadata(types.EntityRoleTentative))); err != nil {
		t.Fatal(err)
	}
	p := l.page(h.EntityID, 0)
	// The list is of what is served; the counts are of the mentions recorded
	// (the private story's edge counts, the tentative capture's does not).
	if p == nil || len(p.StoryList) != 1 || p.StoryList[0].PublicationID != a.PublicationID || p.Mentions != 2 || p.CentralMentions != 2 {
		t.Fatalf("page = %+v, want the one public story listed and 2 recorded", p)
	}
}

func TestEntityPageRouteBoundsWhatAStrangerCanAsk(t *testing.T) { // P3 P5 P7
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Harbor reopens", l.capture("https://wire.example/a", "", "Harbor reopens"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)
	hd := NewHandler(l.svc)
	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		hd.HandlePublicEntityPage(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}
	rec := get(EntityPageInternalPrefix + h.EntityID + "?stories=5")
	var p EntityPage
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Name != "Halvorsen Dynamics" || len(p.StoryList) != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	for _, target := range []string{"?stories=0", "?stories=51", "?stories=x"} {
		if rec := get(EntityPageInternalPrefix + h.EntityID + target); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d", target, rec.Code)
		}
	}
	if rec := get(EntityPageInternalPrefix + "ent:nothex"); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed id = %d", rec.Code)
	}
	if rec := get(EntityPageInternalPrefix + types.MintedEntityID("Nobody Known")); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d", rec.Code)
	}
	post := httptest.NewRecorder()
	hd.HandlePublicEntityPage(post, httptest.NewRequest(http.MethodPost, EntityPageInternalPrefix+h.EntityID, nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", post.Code)
	}
	// P7: the internal route wants an internal caller.
	rec = httptest.NewRecorder()
	hd.HandleInternalEntityPage(rec, httptest.NewRequest(http.MethodGet, EntityPageInternalPrefix+h.EntityID, nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a caller that is not internal = %d", rec.Code)
	}
}

func TestEntityPageAnswersAreCachedForAMinuteAndBounded(t *testing.T) {
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Harbor reopens", l.capture("https://wire.example/a", "", "Harbor reopens"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)
	first, err := l.svc.EntityPage(context.Background(), h.EntityID, 0)
	if err != nil || first == nil || first.Mentions != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	b := l.publish("doc-b", 1, "Turbines ship", l.capture("https://wire.example/b", "", "Turbines ship"))
	l.attachEntities(b, types.StoryEntity{EntityID: h.EntityID, Role: types.EntityRoleCentral})
	if again, _ := l.svc.EntityPage(context.Background(), h.EntityID, 0); again == nil || again.Mentions != 1 {
		t.Fatalf("a second read within the minute was not the cached one: %+v", again)
	}
	// A miss is cached too, so ids that do not exist cost one read a minute.
	missing := types.MintedEntityID("Nobody Known")
	for i := 0; i < 2; i++ {
		if p, err := l.svc.EntityPage(context.Background(), missing, 0); p != nil || err != nil {
			t.Fatalf("%+v %v", p, err)
		}
	}
	if e, ok := l.svc.entityPages.m[missing+"|20"]; !ok || e.page != nil {
		t.Fatalf("the miss is not cached: %+v", l.svc.entityPages.m)
	}
	// The table is bounded: past entityPageCacheMax it starts over.
	l.svc.entityPages.mu.Lock()
	l.svc.entityPages.m = map[string]entityPageEntry{}
	for i := 0; i < entityPageCacheMax; i++ {
		l.svc.entityPages.m[fmt.Sprint("k", i)] = entityPageEntry{at: time.Now().Add(time.Duration(i) * time.Millisecond)}
	}
	l.svc.entityPages.mu.Unlock()
	if _, err := l.svc.EntityPage(context.Background(), h.EntityID, 0); err != nil {
		t.Fatal(err)
	}
	// Full: the oldest entry goes, not the table, and the table stays bounded.
	if n := len(l.svc.entityPages.m); n != entityPageCacheMax {
		t.Fatalf("cache holds %d entries after it was full, want %d", n, entityPageCacheMax)
	}
	if _, ok := l.svc.entityPages.m[h.EntityID+"|20"]; !ok {
		t.Fatal("the new entry is not cached")
	}
}

func TestEntityPageReadsPublicationMentionsAmongCaptureLinks(t *testing.T) { // P6, E3's capture links
	l := newPageLab(t)
	a := l.publish("doc-a", 1, "Harbor reopens", l.capture("https://wire.example/a", "", "Harbor reopens"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)
	canonical, _ := entityCanonicalFor(labOwner, h.EntityID)
	// Later than the story: a busy entity's newest edges are captures'.
	for i := 0; i < entityPageEdgeLimit+20; i++ {
		cap := fmt.Sprintf("obj:choir.web_capture:%s:cap-%d", labOwner, i)
		role := types.EntityRoleCentral
		if i%3 == 0 {
			role = types.EntityRoleIncidental
		}
		if err := l.svc.ogStore().PutEdge(context.Background(), makeEdge(cap, canonical, edgeMentionsEntity, time.Now().UTC().Add(time.Duration(i+1)*time.Second), mentionMetadata(role))); err != nil {
			t.Fatal(err)
		}
	}
	p := l.page(h.EntityID, 0)
	if p == nil || p.Mentions != 1 || p.CentralMentions != 1 || p.CountsAtLeast || len(p.StoryList) != 1 || p.StoryList[0].PublicationID != a.PublicationID {
		t.Fatalf("capture links changed the page: %+v", p)
	}
	// A person's gate still sees the publication's mentions.
	jane := newEntity("Jane Roe", types.EntityTypePerson, types.EntityRoleCentral)
	b := l.publish("doc-b", 1, "Story B", l.capture("https://wire.example/b", "", "Story B"))
	l.attachEntities(a, jane)
	l.attachEntities(b, types.StoryEntity{EntityID: jane.EntityID, Role: types.EntityRoleCentral})
	jc, _ := entityCanonicalFor(labOwner, jane.EntityID)
	for i := 0; i < entityPageEdgeLimit+5; i++ {
		cap := fmt.Sprintf("obj:choir.web_capture:%s:jcap-%d", labOwner, i)
		if err := l.svc.ogStore().PutEdge(context.Background(), makeEdge(cap, jc, edgeMentionsEntity, time.Now().UTC().Add(time.Duration(i+1)*time.Second), mentionMetadata(types.EntityRoleCentral))); err != nil {
			t.Fatal(err)
		}
	}
	if pj := l.page(jane.EntityID, 0); pj == nil || pj.CentralMentions != 2 {
		t.Fatalf("capture links hid a person's page: %+v", pj)
	}
}
