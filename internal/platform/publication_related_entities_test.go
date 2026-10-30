package platform

// The related-stories walk reads shared central entities
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice b;
// design docs/reports/related-stories-graph-design-2026-10-19.md
// §4.1 and §5). Ways it can fail, written before the code:
//
//  X1 Two stories centrally about one entity and sharing no capture are not
//     linked, or only one way, or only when the story also cites a capture.
//  X2 An incidental entity links (R2): both incidental, or one central and
//     one incidental; a retracted (tombstoned) mention still votes.
//  X3 An entity central to more than HubEntityStories stories still votes.
//  X4 The kill switch (CORPUSD_RELATED_ENTITIES=off) leaves entity links in
//     the answer, its basis or a cached answer; the default is not on.
//  X5 Ranking: an entity link ranks behind a capture-only link, or ahead of
//     a judged related_story pair; more shared entities do not rank first.
//  X6 An entity link to a story the walk would not show (retracted, not
//     Autopaper, private) or to the story itself.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func central(name, typ string) types.StoryEntity {
	return newEntity(name, typ, types.EntityRoleCentral)
}

func incidental(name, typ string) types.StoryEntity {
	return newEntity(name, typ, types.EntityRoleIncidental)
}

func TestRelatedLinksStoriesThatShareACentralEntity(t *testing.T) { // X1
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes", l.capture("https://wire.example/a", "", "Vucic conceded the election"))
	b := l.publish("doc-b", 1, "Vucic meets the opposition", l.capture("https://other.example/b", "", "The president met opposition leaders"))
	c := l.publish("doc-c", 1, "Bridge closes", l.capture("https://wire.example/c", "", "A bridge closed for repairs"))
	l.attachEntities(a, central("Aleksandar Vucic", types.EntityTypePerson))
	l.attachEntities(b, central("Aleksandar Vucic", types.EntityTypePerson))
	l.attachEntities(c, central("Golden Gate Bridge", types.EntityTypePlace))

	gotA := l.related(a.RoutePath)
	if gotA.Basis.Mode != relatedModeCaptures+relatedModeEntitiesSuffix {
		t.Fatalf("A basis mode = %q, want the entities mode by default", gotA.Basis.Mode)
	}
	if len(gotA.Links) != 1 || gotA.Links[0].Title != "Vucic meets the opposition" {
		t.Fatalf("A links = %v, want only B", titlesOf(gotA))
	}
	if link := gotA.Links[0]; link.Reason != relatedReasonEntity || link.Shared.Entities != 1 || link.Shared.Sources != 0 {
		t.Fatalf("link = %+v, want reason entity, one shared entity, no shared source", link)
	}
	if gotB := l.related(b.RoutePath); len(gotB.Links) != 1 || gotB.Links[0].Title != "Vucic concedes" {
		t.Fatalf("B links = %v, want A", titlesOf(gotB))
	}
	if gotC := l.related(c.RoutePath); len(gotC.Links) != 0 {
		t.Fatalf("C links = %v, want none", titlesOf(gotC))
	}

	// A story that cites no capture still gets its entity links.
	d := l.publish("doc-d", 1, "Nothing cited")
	l.attachEntities(d, central("Aleksandar Vucic", types.EntityTypePerson))
	if gotD := l.related(d.RoutePath); len(gotD.Links) != 2 || gotD.Basis.Captures != 0 {
		t.Fatalf("D (no captures) links = %v basis %+v, want A and B", titlesOf(gotD), gotD.Basis)
	}
}

func TestRelatedIgnoresIncidentalAndRetractedMentions(t *testing.T) { // X2
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Trade talks", l.capture("https://wire.example/a", "", "Trade talks resumed"))
	b := l.publish("doc-b", 1, "Farm subsidies", l.capture("https://other.example/b", "", "Farm subsidies rose"))
	c := l.publish("doc-c", 1, "Tariff ruling", l.capture("https://third.example/c", "", "A court ruled on tariffs"))
	l.attachEntities(a, central("European Union", types.EntityTypeOrganization), incidental("Donald Trump", types.EntityTypePerson))
	l.attachEntities(b, incidental("European Union", types.EntityTypeOrganization), incidental("Donald Trump", types.EntityTypePerson))
	l.attachEntities(c, central("World Trade Organization", types.EntityTypeOrganization))

	if got := l.related(a.RoutePath); len(got.Links) != 0 {
		t.Fatalf("A links = %v; an incidental mention on either side links nothing", titlesOf(got))
	}

	// C becomes central to the EU, then is changed to incidental: the
	// retracted central edge gives no vote.
	l.attachEntities(c, types.StoryEntity{EntityID: types.MintedEntityID("European Union"), Role: types.EntityRoleCentral})
	if got := l.related(a.RoutePath); len(got.Links) != 1 || got.Links[0].Title != "Tariff ruling" {
		t.Fatalf("A links = %v, want C once C is centrally about the EU", titlesOf(got))
	}
	l.attachEntities(c, types.StoryEntity{EntityID: types.MintedEntityID("European Union"), Role: types.EntityRoleIncidental})
	if got := l.related(a.RoutePath); len(got.Links) != 0 {
		t.Fatalf("A links = %v after C's role changed to incidental, want none", titlesOf(got))
	}
}

func TestRelatedEntityOnTooManyStoriesGivesNoVote(t *testing.T) { // X3
	l := newEntityLab(t)
	hub := l.svc.related.tune().HubEntityStories
	var first *PublishTextureResponse
	for i := 0; i <= hub; i++ { // hub+1 stories centrally about one entity
		s := l.publish(fmt.Sprintf("doc-hub-%d", i), 1, fmt.Sprintf("Hub story %d", i))
		l.attachEntities(s, central("Donald Trump", types.EntityTypePerson))
		if first == nil {
			first = s
		}
	}
	if got := l.related(first.RoutePath); len(got.Links) != 0 {
		t.Fatalf("a hub entity linked %v", titlesOf(got))
	}

	// A second, rare entity still links.
	other := l.publish("doc-rare", 1, "Rare story")
	l.attachEntities(other, central("Rare Commission", types.EntityTypeOrganization))
	l.attachEntities(first, types.StoryEntity{EntityID: types.MintedEntityID("Rare Commission"), Role: types.EntityRoleCentral})
	if got := l.related(first.RoutePath); len(got.Links) != 1 || got.Links[0].Title != "Rare story" {
		t.Fatalf("links = %v, want only the rare entity's story", titlesOf(got))
	}
}

func TestRelatedEntityKillSwitch(t *testing.T) { // X4
	l := newEntityLab(t)
	a := l.publish("doc-a", 1, "Vucic concedes")
	b := l.publish("doc-b", 1, "Vucic meets the opposition")
	l.attachEntities(a, central("Aleksandar Vucic", types.EntityTypePerson))
	l.attachEntities(b, central("Aleksandar Vucic", types.EntityTypePerson))

	for _, on := range []string{"", "on", "1", "yes"} {
		t.Setenv(relatedEntitiesEnv, on)
		if got := l.related(a.RoutePath); len(got.Links) != 1 {
			t.Fatalf("switch %q: links %v, want the entity link (on unless off)", on, titlesOf(got))
		}
	}
	t.Setenv(relatedEntitiesEnv, "off")
	got := l.related(a.RoutePath)
	if len(got.Links) != 0 || got.Basis.Mode != relatedModeCaptures {
		t.Fatalf("off: links %v mode %q, want none and the captures mode", titlesOf(got), got.Basis.Mode)
	}

	// The cache key carries the mode.
	l.svc.related.reset()
	if first, err := l.svc.RelatedStories(context.Background(), a.RoutePath, 0); err != nil || len(first.Links) != 0 {
		t.Fatalf("off: %v %v", first, err)
	}
	t.Setenv(relatedEntitiesEnv, "")
	if on, err := l.svc.RelatedStories(context.Background(), a.RoutePath, 0); err != nil || len(on.Links) != 1 {
		t.Fatalf("on after an off answer was cached: %v, %v", on, err)
	}
}

func TestRelatedEntityRanking(t *testing.T) { // X5
	l := newEntityLab(t)
	t.Setenv(relatedStoryEnv, "on")
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	self := l.publish("doc-self", 1, "Harbor reopens", x)
	capOnly := l.publish("doc-cap", 1, "Ferries resume", x)
	oneEntity := l.publish("doc-one", 1, "Port strike", l.capture("https://other.example/o", "", "Dockers struck"))
	twoEntities := l.publish("doc-two", 1, "Port budget", l.capture("https://third.example/t", "", "The port budget grew"))
	judged := l.publish("doc-judged", 1, "Harbor history", l.capture("https://fourth.example/j", "", "The harbor opened in 1890"))

	l.attachEntities(self, central("Port of Oakland", types.EntityTypePlace), central("Oakland Harbor Authority", types.EntityTypeOrganization))
	l.attachEntities(oneEntity, central("Port of Oakland", types.EntityTypePlace))
	l.attachEntities(twoEntities, central("Port of Oakland", types.EntityTypePlace), central("Oakland Harbor Authority", types.EntityTypeOrganization))
	l.relate(types.StoryRelationSameSubject, self, judged)

	got := l.related(self.RoutePath)
	want := []string{"Harbor history", "Port budget", "Port strike", "Ferries resume"}
	if fmt.Sprint(titlesOf(got)) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v (judged, then more shared entities, then fewer, then capture-only)", titlesOf(got), want)
	}
	if got.Links[1].Shared.Entities != 2 || got.Links[2].Shared.Entities != 1 || got.Links[3].Reason != relatedReasonCapture {
		t.Fatalf("links = %+v", got.Links)
	}
	_ = capOnly
}

func TestRelatedEntityIgnoresSelfAndStoriesTheWalkWouldNotShow(t *testing.T) { // X6
	l := newEntityLab(t)
	ctx := context.Background()
	a := l.publish("doc-a", 1, "Vucic concedes")
	retracted := l.publish("doc-r", 1, "Retracted Vucic story")
	good := l.publish("doc-g", 1, "Good Vucic story")
	for _, s := range []*PublishTextureResponse{a, retracted, good} {
		l.attachEntities(s, central("Aleksandar Vucic", types.EntityTypePerson))
	}
	pub, err := l.svc.ogStore().GetObject(ctx, l.canon(labOwner, retracted.PublicationID))
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	_ = json.Unmarshal(pub.Metadata, &meta)
	meta["state"] = "retracted"
	pub.Metadata, _ = json.Marshal(meta)
	if err := l.svc.ogStore().PutObject(ctx, pub); err != nil {
		t.Fatal(err)
	}
	got := l.related(a.RoutePath)
	if len(got.Links) != 1 || got.Links[0].Title != "Good Vucic story" {
		t.Fatalf("A links = %v, want only the good story (not itself, not the retracted one)", titlesOf(got))
	}
}
