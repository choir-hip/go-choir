package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// writeRevisionSourceGraph writes one entity and one ref pinning it for a
// revision of computer computerID through the lifecycle writer, as a Texture
// commit does.
func writeRevisionSourceGraph(t *testing.T, s *Store, ownerID, computerID, docID, revisionID, target string) TextureSourceEntityGraphRecord {
	t.Helper()
	ctx := context.Background()
	ownerScope := ownerID + "\x00" + computerID
	entityID, err := BuildTextureSourceEntityCanonicalID(ownerID, ownerScope, "web_url", target)
	if err != nil {
		t.Fatal(err)
	}
	refID, err := BuildTextureSourceRefCanonicalIDByScope(ownerID, computerID, revisionID, "doc/"+target)
	if err != nil {
		t.Fatal(err)
	}
	entityMeta := json.RawMessage(fmt.Sprintf(`{"computer_id":%q,"source_kind":"web_url","target":{"identity":%q},"texture_doc_id":%q,"texture_revision_id":%q}`, computerID, target, docID, revisionID))
	refMeta := json.RawMessage(fmt.Sprintf(`{"computer_id":%q,"identity_key":%q,"doc_id":%q,"texture_revision_id":%q}`, computerID, "doc/"+target, docID, revisionID))
	entity := TextureSourceEntityGraphRecord{CanonicalID: entityID, OwnerID: ownerID, ComputerID: computerID, Body: []byte("evidence " + target), Metadata: entityMeta, LegacySourceEntityID: "src-" + target}
	versionID, _, _, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, entity.Body, entity.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	graph := TextureSourceGraphWriteSet{
		SourceEntities: []TextureSourceEntityGraphRecord{entity},
		SourceRefs: []TextureSourceRefGraphRecord{{
			CanonicalID: refID, OwnerID: ownerID, ComputerID: computerID, DocID: docID, TextureRevisionID: revisionID,
			BodyNodeID: "ref-" + target, SourceEntityCanonicalID: entityID, SourceEntityVersionID: versionID,
			DisplayMode: TextureSourceRefDisplayNumbered, CitationState: "cited", Metadata: refMeta,
		}},
	}
	rev := types.Revision{RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, CreatedAt: time.Now().UTC()}
	objects, conditions, err := s.lifecycleSourceGraphBatch(ctx, rev, graph, rev.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatal(err)
	}
	entity.VersionID = versionID
	return entity
}

// The load of one revision's entities reads the entities its refs pin by id
// and does not read the scope. A scope that cannot be read whole (one entity
// object with an undecodable body) must not fail it: the old read decoded
// every entity of the scope and so failed on it.
func TestSourceEntitiesForRevisionDoNotReadTheScope(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer, doc = "owner-keyed", "computer-keyed", "doc-keyed"
	want := writeRevisionSourceGraph(t, s, owner, computer, doc, "rev-keyed", "https://example.com/a")
	writeRevisionSourceGraph(t, s, owner, computer, doc, "rev-other", "https://example.com/b")

	poisonID, err := objectgraph.BuildCanonicalID(TextureSourceEntityObjectKind, owner, "poison")
	if err != nil {
		t.Fatal(err)
	}
	poison := objectgraph.Object{
		CanonicalID: poisonID, ObjectKind: TextureSourceEntityObjectKind, OwnerID: owner, ComputerID: computer,
		Body: []byte("not json"), Metadata: json.RawMessage(`{"entity_version_key":"poison"}`),
		ContentHash: objectgraph.ContentHash(TextureSourceEntityObjectKind, []byte("not json"), []byte(`{"entity_version_key":"poison"}`)),
		CreatedAt:   time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.ogStore.PutBatchConditional(ctx, nil, objectgraph.Batch{Objects: []objectgraph.Object{poison}}); err != nil {
		t.Fatalf("write poison: %v", err)
	}
	if _, err := s.ListTextureSourceEntitiesByScope(ctx, owner, computer); err == nil {
		t.Fatal("test setup: the scope read should fail on the poisoned entity")
	}

	got, err := s.ListTextureSourceEntitiesForRevisionByScope(ctx, owner, computer, doc, "rev-keyed")
	if err != nil {
		t.Fatalf("revision entities read the scope: %v", err)
	}
	if len(got) != 1 || got[0].CanonicalID != want.CanonicalID || got[0].VersionID != want.VersionID {
		t.Fatalf("entities = %+v, want only %s/%s", got, want.CanonicalID, want.VersionID)
	}
}

// An entity the revision wrote but did not cite (its metadata names the
// revision, no ref pins it) is not returned: the old scope read returned it.
// A revision with no refs has no entities, for every revision, not only then.
func TestSourceEntitiesForRevisionOmitsUncitedEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer, doc, revisionID = "owner-uncited", "computer-uncited", "doc-uncited", "rev-uncited"
	cited := writeRevisionSourceGraph(t, s, owner, computer, doc, revisionID, "https://example.com/cited")
	// An uncited entity of the same revision: written with no ref.
	uncitedID, err := BuildTextureSourceEntityCanonicalID(owner, owner+"\x00"+computer, "web_url", "https://example.com/uncited")
	if err != nil {
		t.Fatal(err)
	}
	meta := json.RawMessage(fmt.Sprintf(`{"computer_id":%q,"source_kind":"web_url","target":{"identity":"https://example.com/uncited"},"texture_doc_id":%q,"texture_revision_id":%q}`, computer, doc, revisionID))
	rev := types.Revision{RevisionID: revisionID, DocID: doc, OwnerID: owner, ComputerID: computer, CreatedAt: time.Now().UTC()}
	objects, conditions, err := s.lifecycleSourceGraphBatch(ctx, rev, TextureSourceGraphWriteSet{SourceEntities: []TextureSourceEntityGraphRecord{{
		CanonicalID: uncitedID, OwnerID: owner, ComputerID: computer, Body: []byte("uncited"), Metadata: meta,
	}}}, rev.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatal(err)
	}
	if all, err := s.ListTextureSourceEntitiesByScope(ctx, owner, computer); err != nil || len(all) != 2 {
		t.Fatalf("setup: scope entities = %d, %v; want 2", len(all), err)
	}
	if old, err := s.listTextureSourceEntitiesForRevisionByScan(ctx, owner, computer, doc, revisionID, nil); err != nil || len(old) != 2 {
		t.Fatalf("setup: the old read returned %d, %v; want both", len(old), err)
	}
	got, err := s.ListTextureSourceEntitiesForRevisionByScope(ctx, owner, computer, doc, revisionID)
	if err != nil || len(got) != 1 || got[0].CanonicalID != cited.CanonicalID {
		t.Fatalf("entities = %+v, %v; want only the cited one", got, err)
	}
	none, err := s.ListTextureSourceEntitiesForRevisionByScope(ctx, owner, computer, doc, "rev-without-refs")
	if err != nil || len(none) != 0 {
		t.Fatalf("a revision with no refs got %+v, %v", none, err)
	}
}

// A ref cannot reach another computer's or another owner's entity: the id is
// built under the reader's scope, so it is not found, the scope read answers,
// and nothing of the other scope comes back.
func TestSourceEntitiesForRefsDoNotCrossComputerOrOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const doc, revisionID = "doc-cross", "rev-cross"
	otherComputer := writeRevisionSourceGraph(t, s, "owner-cross", "computer-b", doc, revisionID, "https://example.com/x")
	otherOwner := writeRevisionSourceGraph(t, s, "owner-other", "computer-a", doc, revisionID, "https://example.com/x")
	for name, pinned := range map[string]TextureSourceEntityGraphRecord{"computer": otherComputer, "owner": otherOwner} {
		refs := []TextureSourceRefGraphRecord{{SourceEntityCanonicalID: pinned.CanonicalID, SourceEntityVersionID: pinned.VersionID}}
		got, scanned, err := s.ListTextureSourceEntitiesForRefsByScope(ctx, "owner-cross", "computer-a", doc, revisionID, refs)
		if err != nil || len(got) != 0 || !scanned {
			t.Fatalf("%s: entities = %+v scanned=%v err=%v; want none, by the scan", name, got, scanned, err)
		}
	}
}

// Two refs to one entity version return it once, without a scan.
func TestSourceEntitiesForRefsDeduplicate(t *testing.T) {
	s := openTestStore(t)
	const owner, computer, doc, revisionID = "owner-dup", "computer-dup", "doc-dup", "rev-dup"
	e := writeRevisionSourceGraph(t, s, owner, computer, doc, revisionID, "https://example.com/dup")
	ref := TextureSourceRefGraphRecord{SourceEntityCanonicalID: e.CanonicalID, SourceEntityVersionID: e.VersionID}
	got, scanned, err := s.ListTextureSourceEntitiesForRefsByScope(context.Background(), owner, computer, doc, revisionID, []TextureSourceRefGraphRecord{ref, ref})
	if err != nil || scanned || len(got) != 1 {
		t.Fatalf("entities = %d scanned=%v err=%v; want one, by id", len(got), scanned, err)
	}
}

// A ref that pins a version the id read cannot find falls back to the scope
// read, so an entity the revision wrote is still returned.
func TestSourceEntitiesForRevisionFallsBackWhenPinnedVersionIsNotFoundByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer, doc, revisionID = "owner-fb", "computer-fb", "doc-fb", "rev-fb"
	own := writeRevisionSourceGraph(t, s, owner, computer, doc, revisionID, "https://example.com/fb")
	refs := []TextureSourceRefGraphRecord{{SourceEntityCanonicalID: own.CanonicalID, SourceEntityVersionID: "missing-version"}}
	got, scanned, err := s.ListTextureSourceEntitiesForRefsByScope(ctx, owner, computer, doc, revisionID, refs)
	if err != nil || !scanned {
		t.Fatalf("scanned=%v err=%v; want the scope scan to have run", scanned, err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].VersionID != own.VersionID {
		t.Fatalf("fallback entities = %+v, want the revision's own %s", got, own.VersionID)
	}
}
