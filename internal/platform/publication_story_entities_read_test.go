package platform

// Failure list: publication_story_entities_read.go (E1-E5).

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestFrontAndBundleCarryCentralEntitiesOnly(t *testing.T) {
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded the election"))
	l.publish("doc-b", 1, "Quiet harbour", l.capture("https://wire.example/b", "", "The harbour was quiet"))
	vucic := newEntity("Aleksandar Vucic", types.EntityTypePerson, types.EntityRoleCentral)
	serbia := newEntity("Serbia", types.EntityTypePlace, types.EntityRoleIncidental)
	l.attachEntities(a, vucic, serbia)

	page, err := l.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	byTitle := map[string]FrontStory{}
	for _, s := range page.Stories {
		byTitle[s.Title] = s
	}
	got := byTitle["Vucic concedes"].Entities
	if len(got) != 1 || got[0].ID != vucic.EntityID || got[0].Label != "Aleksandar Vucic" { // E1, E5
		t.Fatalf("front entities = %+v, want only the central Vucic", got)
	}
	if e := byTitle["Quiet harbour"].Entities; e != nil { // E3
		t.Fatalf("story without entities = %+v, want absent", e)
	}
	bundle, err := l.svc.GetPublicationBundleByRoute(context.Background(), a.RoutePath)
	if err != nil || len(bundle.Entities) != 1 || bundle.Entities[0].ID != vucic.EntityID {
		t.Fatalf("bundle entities = %+v, %v", bundle, err)
	}

	// E2: a role change to incidental retracts the central mention.
	l.attachEntities(a, types.StoryEntity{EntityID: vucic.EntityID, Role: types.EntityRoleIncidental})
	page, _ = l.svc.ListFrontStories(context.Background(), 0, "")
	for _, s := range page.Stories {
		if s.Title == "Vucic concedes" && len(s.Entities) != 0 {
			t.Fatalf("incidental entity still served: %+v", s.Entities)
		}
	}
}

// E6, E7: more than 12 central entities serve 12, the same 12 in id order;
// the edge reads for a page are one query per 40 stories, not one per row.
func TestStoryEntitiesAreCappedInIDOrderAndBatched(t *testing.T) {
	l := newEntityLab(t)
	var pubs []string
	for i := 0; i < 5; i++ {
		s := l.publish(fmt.Sprintf("doc-%d", i), 1, fmt.Sprintf("Story %d", i), l.capture(fmt.Sprintf("https://wire.example/%d", i), "", fmt.Sprintf("body %d", i)))
		pubs = append(pubs, l.canon(labOwner, s.PublicationID))
	}
	first := l.publish("doc-many", 1, "Many", l.capture("https://wire.example/many", "", "many"))
	var ents []types.StoryEntity
	for i := 0; i < 15; i++ {
		ents = append(ents, newEntity(fmt.Sprintf("Subject Number %02d", i), types.EntityTypePerson, types.EntityRoleCentral))
	}
	l.attachEntities(first, ents[:12]...)
	l.attachEntities(first, ents[12:]...)
	manyID := l.canon(labOwner, first.PublicationID)
	pubs = append(pubs, manyID)

	og := l.svc.ogStore()
	calls := func() int64 { return l.svc.store.RetrievalSnapshot().Ops[retrievalListEdges].Calls }
	before := calls()
	got := storiesCentralEntities(context.Background(), og, pubs)
	if d := calls() - before; d != 1 {
		t.Fatalf("E7: %d edge queries for %d stories, want 1", d, len(pubs))
	}
	many := got[manyID]
	if len(many) != maxStoryEntitiesPerRequest {
		t.Fatalf("E6: %d entities served, want %d", len(many), maxStoryEntitiesPerRequest)
	}
	canon := func(i int) string { c, _ := entityCanonicalFor(labOwner, many[i].ID); return c }
	if !sort.SliceIsSorted(many, func(i, j int) bool { return canon(i) < canon(j) }) {
		t.Fatalf("E6: not in entity-key order: %+v", many)
	}
	if again := storiesCentralEntities(context.Background(), og, pubs)[manyID]; fmt.Sprint(again) != fmt.Sprint(many) {
		t.Fatalf("E6: the 12 changed between reads")
	}
	ids := make([]string, 45)
	for i := range ids {
		ids[i] = fmt.Sprintf("obj:none:%d", i)
	}
	before = calls()
	storiesCentralEntities(context.Background(), og, ids)
	if d := calls() - before; d != 2 {
		t.Fatalf("E7: 45 stories took %d edge queries, want 2", d)
	}
}
