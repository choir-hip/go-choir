package platform

// Production's og_edges.from_id is compared in the database's own collation,
// and the paper's is not the C collation the test databases use: there ':' sorts
// after ';' (ICU root, glibc en_US), so a range
// `from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'`
// is EMPTY. The entity read's mention edges and the candidates' weight probe
// both used that range, so on staging every entity had no stories and weight
// 0 (docs/problems/entity-read-lists-no-stories-in-production-2026-11-20.md).
// Every test here builds its edges through the production writers
// (RecordStoryEntities through a published story, the capture and publication id
// builders) and runs with the column in a non-C collation.
//
//  K1 the entity page lists the stories of an entity whose edges were written
//     by the story export, under a collation where the range is empty;
//  K2 a person's gate (central edges) sees the same;
//  K3 the candidates' weights (mentions, central) are not zero;
//  K4 the ready list's neighbour links (from_id equality over canonical capture
//     ids) are found under it.

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// nonCCollation moves og_edges.from_id to a collation in which the prefix
// range is empty, as on the paper, or skips when the server has none.
func nonCCollation(t *testing.T, db *sql.DB) {
	t.Helper()
	var done bool
	for _, coll := range []string{`"und-x-icu"`, `"en_US.utf8"`, `"en_US.UTF-8"`} {
		if _, err := db.Exec(`ALTER TABLE og_edges ALTER COLUMN from_id TYPE text COLLATE ` + coll); err == nil {
			done = true
			break
		}
	}
	if !done {
		t.Skip("the test server has no non-C collation to reproduce the paper's")
	}
	// Pooled connections hold plans made for the old column type: drop them.
	db.SetMaxIdleConns(0)
	db.SetMaxIdleConns(8)
	// The premise: a probe row shows the range is empty here.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM (SELECT 'obj:choir.publication:x'::text COLLATE "und-x-icu" AS from_id) t WHERE from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'`).Scan(&n); err == nil && n != 0 {
		t.Skip("this collation keeps the prefix range intact")
	}
}

func TestEntityPageUnderTheProductionCollationListsTheStoriesTheExportWrote(t *testing.T) { // K1
	l := newPageLab(t)
	nonCCollation(t, l.store.corpus())
	a := l.publish("doc-a", 1, "Harbor reopens", l.capture("https://wire.example/a", "", "Harbor reopens"))
	b := l.publish("doc-b", 1, "Turbines ship", l.capture("https://wire.example/b", "", "Turbines ship"))
	h := newEntity("Halvorsen Dynamics", types.EntityTypeOrganization, types.EntityRoleCentral)
	l.attachEntities(a, h)
	l.attachEntities(b, types.StoryEntity{EntityID: h.EntityID, Role: types.EntityRoleIncidental})
	p := l.page(h.EntityID, 0)
	if p == nil || p.Stories != 2 || p.CentralStories != 1 || len(p.StoryList) != 2 {
		t.Fatalf("the page lists no stories the export wrote: %+v", p)
	}
}

func TestPersonGateUnderTheProductionCollation(t *testing.T) { // K2
	l := newPageLab(t)
	nonCCollation(t, l.store.corpus())
	a := l.publish("doc-a", 1, "Story A", l.capture("https://wire.example/a", "", "Story A"))
	b := l.publish("doc-b", 1, "Story B", l.capture("https://wire.example/b", "", "Story B"))
	jane := newEntity("Jane Roe", types.EntityTypePerson, types.EntityRoleCentral)
	l.attachEntities(a, jane)
	l.attachEntities(b, types.StoryEntity{EntityID: jane.EntityID, Role: types.EntityRoleCentral})
	if p := l.page(jane.EntityID, 0); p == nil || p.CentralStories != 2 {
		t.Fatalf("a person central in two public stories has no page: %+v", p)
	}
}

func TestCandidateWeightsUnderTheProductionCollation(t *testing.T) { // K3
	f := newVecFixture(t)
	nonCCollation(t, f.store.corpus())
	ent := f.entityRow(t, readyOwner, "Halvorsen Dynamics", types.EntityTypeOrganization)
	for i := 0; i < 3; i++ {
		// The publication ids the story export builds.
		pub, err := publicationCanonicalFor(readyOwner, fmt.Sprintf("pub-%032x", i+1))
		if err != nil {
			t.Fatal(err)
		}
		role := types.EntityRoleCentral
		if i == 2 {
			role = types.EntityRoleIncidental
		}
		f.mention(t, pub, ent, role)
	}
	ms := f.mentionsOf(t, "Halvorsen", "Later, Halvorsen Dynamics shipped a turbine. Halvorsen Dynamics engineers said tests went well.")
	m, ok := ms["halvorsen dynamics"]
	if !ok || len(m.Candidates) == 0 || m.Candidates[0].Mentions != 3 || m.Candidates[0].Central != 2 {
		t.Fatalf("weights are not the stories' mention counts: %+v", m)
	}
}

func TestReadyNeighbourLinksUnderTheProductionCollation(t *testing.T) { // K4
	f := newVecFixture(t)
	nonCCollation(t, f.store.corpus())
	now := time.Now().UTC()
	f.capture(t, readyOwner, readyComputer, "q", "Quorum", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, readyOwner, readyComputer, "n1", "Near one", "quorum valdoria amber treaty ratified today", now.Add(-2*time.Hour), false)
	f.pass(t)
	val := f.entityRow(t, readyOwner, "Valdoria", types.EntityTypePlace)
	// The capture's canonical id, as the capture writer builds it.
	f.mention(t, mustCaptureID(t, readyOwner, "n1"), val, types.EntityRoleCentral)
	res := f.ready(t, readyReq(1))
	found := false
	for _, n := range res.Ready.Captures[0].Neighbours {
		for _, e := range n.Entities {
			found = found || e.EntityID == types.MintedEntityID("Valdoria")
		}
	}
	if !found {
		t.Fatalf("a neighbour's entity link is not found: %+v", res.Ready.Captures[0].Neighbours)
	}
}

var _ = context.Background
