package platform

// Failure list: publication_story_entities_read.go (E1-E5).

import (
	"context"
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
