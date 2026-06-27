package objectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// openTestDoltStore creates an embedded Dolt workspace in a temp directory and
// returns a DoltStore backed by it. The workspace is isolated per-test by
// using t.TempDir().
func openTestDoltStore(t *testing.T) *DoltStore {
	t.Helper()
	workspacePath := filepath.Join(t.TempDir(), "dolt-workspace")
	dbName := "objectgraph_test"
	store, err := OpenDoltStore(workspacePath, dbName)
	if err != nil {
		t.Fatalf("OpenDoltStore() error = %v", err)
	}
	return store
}

func TestDoltStorePutGetObjectRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	obj := Object{
		CanonicalID:  "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-abc",
		ObjectKind:   "choir.source_entity",
		OwnerID:      "user:alice",
		ComputerID:   "computer:local",
		VersionID:    "v1",
		ContentHash:  "sha256:deadbeef",
		Body:         []byte("source body content"),
		Metadata:     json.RawMessage(`{"display_title":"Test","kind":"web_url"}`),
		CreatedAt:    now,
		UpdatedAt:    now,
		Tombstone:    false,
		SupersededBy: "",
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}

	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if got.CanonicalID != obj.CanonicalID {
		t.Errorf("CanonicalID = %q, want %q", got.CanonicalID, obj.CanonicalID)
	}
	if got.ObjectKind != obj.ObjectKind {
		t.Errorf("ObjectKind = %q, want %q", got.ObjectKind, obj.ObjectKind)
	}
	if got.OwnerID != obj.OwnerID {
		t.Errorf("OwnerID = %q, want %q", got.OwnerID, obj.OwnerID)
	}
	if got.ComputerID != obj.ComputerID {
		t.Errorf("ComputerID = %q, want %q", got.ComputerID, obj.ComputerID)
	}
	if got.VersionID != obj.VersionID {
		t.Errorf("VersionID = %q, want %q", got.VersionID, obj.VersionID)
	}
	if got.ContentHash != obj.ContentHash {
		t.Errorf("ContentHash = %q, want %q", got.ContentHash, obj.ContentHash)
	}
	if string(got.Body) != string(obj.Body) {
		t.Errorf("Body = %q, want %q", got.Body, obj.Body)
	}
	if string(got.Metadata) != string(obj.Metadata) {
		t.Errorf("Metadata = %q, want %q", got.Metadata, obj.Metadata)
	}
	if !got.CreatedAt.Equal(obj.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, obj.CreatedAt)
	}
	if !got.UpdatedAt.Equal(obj.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, obj.UpdatedAt)
	}
	if got.Tombstone != obj.Tombstone {
		t.Errorf("Tombstone = %v, want %v", got.Tombstone, obj.Tombstone)
	}
}

func TestDoltStoreGetObjectNotFound(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	_, err := store.GetObject(ctx, "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetObject(nonexistent) error = %v, want ErrNotFound", err)
	}
}

func TestDoltStoreListObjectsByKind(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	objs := []Object{
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-a",
			ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
			ContentHash: "sha256:a", Body: []byte("a"),
			Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
		},
		{
			CanonicalID: "obj:choir.web_capture:dXNlcjphbGljZQ:sha256-b",
			ObjectKind:  "choir.web_capture", OwnerID: "user:alice",
			ContentHash: "sha256:b", Body: []byte("b"),
			Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
		},
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-c",
			ObjectKind:  "choir.source_entity", OwnerID: "user:bob",
			ContentHash: "sha256:c", Body: []byte("c"),
			Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, obj := range objs {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("PutObject(%s) error = %v", obj.CanonicalID, err)
		}
	}

	got, err := store.ListObjects(ctx, ListFilter{Kind: "choir.source_entity"})
	if err != nil {
		t.Fatalf("ListObjects(kind=source_entity) error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d objects, want 2", len(got))
	}
	for _, obj := range got {
		if obj.ObjectKind != "choir.source_entity" {
			t.Errorf("unexpected kind %q in results", obj.ObjectKind)
		}
	}
}

func TestDoltStoreListObjectsByOwner(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	objs := []Object{
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-a",
			ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
			ContentHash: "sha256:a", Metadata: json.RawMessage(`{}`),
			CreatedAt: now, UpdatedAt: now,
		},
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-b",
			ObjectKind:  "choir.source_entity", OwnerID: "user:bob",
			ContentHash: "sha256:b", Metadata: json.RawMessage(`{}`),
			CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, obj := range objs {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("PutObject(%s) error = %v", obj.CanonicalID, err)
		}
	}

	got, err := store.ListObjects(ctx, ListFilter{OwnerID: "user:alice"})
	if err != nil {
		t.Fatalf("ListObjects(owner=alice) error = %v", err)
	}
	if len(got) != 1 || got[0].OwnerID != "user:alice" {
		t.Fatalf("got %#v, want 1 object owned by alice", got)
	}
}

func TestDoltStoreListObjectsByTombstone(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	objs := []Object{
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-a",
			ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
			ContentHash: "sha256:a", Metadata: json.RawMessage(`{}`),
			CreatedAt: now, UpdatedAt: now, Tombstone: false,
		},
		{
			CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-b",
			ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
			ContentHash: "sha256:b", Metadata: json.RawMessage(`{}`),
			CreatedAt: now, UpdatedAt: now, Tombstone: true,
		},
	}
	for _, obj := range objs {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("PutObject(%s) error = %v", obj.CanonicalID, err)
		}
	}

	active := false
	got, err := store.ListObjects(ctx, ListFilter{Tombstone: &active})
	if err != nil {
		t.Fatalf("ListObjects(tombstone=false) error = %v", err)
	}
	if len(got) != 1 || got[0].Tombstone != false {
		t.Fatalf("got %#v, want 1 active object", got)
	}

	tombstoned := true
	got, err = store.ListObjects(ctx, ListFilter{Tombstone: &tombstoned})
	if err != nil {
		t.Fatalf("ListObjects(tombstone=true) error = %v", err)
	}
	if len(got) != 1 || got[0].Tombstone != true {
		t.Fatalf("got %#v, want 1 tombstoned object", got)
	}
}

func TestDoltStorePutEdgeListEdges(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	source := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-src",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:src", Body: []byte("source"),
		Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	capture := Object{
		CanonicalID: "obj:choir.web_capture:dXNlcjphbGljZQ:sha256-cap",
		ObjectKind:  "choir.web_capture", OwnerID: "user:alice",
		ContentHash: "sha256:cap", Body: []byte("capture"),
		Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	for _, obj := range []Object{source, capture} {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("PutObject(%s) error = %v", obj.CanonicalID, err)
		}
	}

	edgeID, err := BuildEdgeID(capture.CanonicalID, source.CanonicalID, "captured_from", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildEdgeID() error = %v", err)
	}
	edge := Edge{
		EdgeID:    edgeID,
		FromID:    capture.CanonicalID,
		ToID:      source.CanonicalID,
		Kind:      "captured_from",
		Metadata:  json.RawMessage(`{"relation":"original"}`),
		CreatedAt: now,
	}
	if err := store.PutEdge(ctx, edge); err != nil {
		t.Fatalf("PutEdge() error = %v", err)
	}

	got, err := store.ListEdges(ctx, EdgeFilter{FromID: capture.CanonicalID})
	if err != nil {
		t.Fatalf("ListEdges(from=capture) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d edges, want 1", len(got))
	}
	if got[0].EdgeID != edge.EdgeID || got[0].ToID != source.CanonicalID {
		t.Errorf("edge = %#v, want toID=%s", got[0], source.CanonicalID)
	}
	if string(got[0].Metadata) != string(edge.Metadata) {
		t.Errorf("metadata = %q, want %q", got[0].Metadata, edge.Metadata)
	}

	gotByKind, err := store.ListEdges(ctx, EdgeFilter{Kind: "captured_from"})
	if err != nil {
		t.Fatalf("ListEdges(kind=captured_from) error = %v", err)
	}
	if len(gotByKind) != 1 {
		t.Fatalf("got %d edges by kind, want 1", len(gotByKind))
	}

	gotByTo, err := store.ListEdges(ctx, EdgeFilter{ToID: source.CanonicalID})
	if err != nil {
		t.Fatalf("ListEdges(to=source) error = %v", err)
	}
	if len(gotByTo) != 1 {
		t.Fatalf("got %d edges by to, want 1", len(gotByTo))
	}
}

func TestDoltStoreTombstoneBehavior(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	obj := Object{
		CanonicalID:  "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-tomb",
		ObjectKind:   "choir.source_entity",
		OwnerID:      "user:alice",
		ContentHash:  "sha256:tomb",
		Body:         []byte("original"),
		Metadata:     json.RawMessage(`{}`),
		CreatedAt:    now,
		UpdatedAt:    now,
		Tombstone:    false,
		SupersededBy: "",
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}

	obj.Tombstone = true
	obj.SupersededBy = "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-new"
	obj.UpdatedAt = now.Add(time.Hour)
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject(tombstone) error = %v", err)
	}

	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if !got.Tombstone {
		t.Errorf("Tombstone = false, want true")
	}
	if got.SupersededBy != obj.SupersededBy {
		t.Errorf("SupersededBy = %q, want %q", got.SupersededBy, obj.SupersededBy)
	}

	active := false
	activeObjs, err := store.ListObjects(ctx, ListFilter{Tombstone: &active})
	if err != nil {
		t.Fatalf("ListObjects(active) error = %v", err)
	}
	if len(activeObjs) != 0 {
		t.Errorf("got %d active objects, want 0", len(activeObjs))
	}

	tombstoned := true
	tombObjs, err := store.ListObjects(ctx, ListFilter{Tombstone: &tombstoned})
	if err != nil {
		t.Fatalf("ListObjects(tombstoned) error = %v", err)
	}
	if len(tombObjs) != 1 {
		t.Errorf("got %d tombstoned objects, want 1", len(tombObjs))
	}
}

func TestDoltStoreUpsertUpdatesExistingObject(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	obj := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-up",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:v1", Body: []byte("v1"),
		Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject(v1) error = %v", err)
	}

	obj.ContentHash = "sha256:v2"
	obj.Body = []byte("v2")
	obj.UpdatedAt = now.Add(time.Hour)
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject(v2) error = %v", err)
	}

	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if string(got.Body) != "v2" {
		t.Errorf("Body = %q, want %q", got.Body, "v2")
	}
	if got.ContentHash != "sha256:v2" {
		t.Errorf("ContentHash = %q, want %q", got.ContentHash, "sha256:v2")
	}
}

func TestDoltStoreEdgeTombstoneFilter(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	from := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-ef-from",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:ef-from", Metadata: json.RawMessage(`{}`),
		CreatedAt: now, UpdatedAt: now,
	}
	to := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-ef-to",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:ef-to", Metadata: json.RawMessage(`{}`),
		CreatedAt: now, UpdatedAt: now,
	}
	for _, obj := range []Object{from, to} {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("PutObject(%s) error = %v", obj.CanonicalID, err)
		}
	}

	edgeID, _ := BuildEdgeID(from.CanonicalID, to.CanonicalID, "references", json.RawMessage(`{}`))
	edge := Edge{
		EdgeID:    edgeID,
		FromID:    from.CanonicalID,
		ToID:      to.CanonicalID,
		Kind:      "references",
		Metadata:  json.RawMessage(`{}`),
		CreatedAt: now,
	}
	if err := store.PutEdge(ctx, edge); err != nil {
		t.Fatalf("PutEdge() error = %v", err)
	}

	edge.Tombstone = true
	if err := store.PutEdge(ctx, edge); err != nil {
		t.Fatalf("PutEdge(tombstone) error = %v", err)
	}

	active := false
	got, err := store.ListEdges(ctx, EdgeFilter{Tombstone: &active})
	if err != nil {
		t.Fatalf("ListEdges(active) error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d active edges, want 0", len(got))
	}

	tombstoned := true
	got, err = store.ListEdges(ctx, EdgeFilter{Tombstone: &tombstoned})
	if err != nil {
		t.Fatalf("ListEdges(tombstoned) error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d tombstoned edges, want 1", len(got))
	}
}

func TestDoltStoreReopenPreservesData(t *testing.T) {
	ctx := context.Background()
	workspacePath := filepath.Join(t.TempDir(), "dolt-workspace")
	dbName := "objectgraph_reopen"

	store, err := OpenDoltStore(workspacePath, dbName)
	if err != nil {
		t.Fatalf("OpenDoltStore() error = %v", err)
	}
	now := time.Unix(1700000000, 0).UTC()
	obj := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-reopen",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:reopen", Body: []byte("persistent"),
		Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := OpenDoltStore(workspacePath, dbName)
	if err != nil {
		t.Fatalf("reopen DoltStore error = %v", err)
	}
	defer reopened.Close()

	got, err := reopened.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() after reopen error = %v", err)
	}
	if string(got.Body) != "persistent" {
		t.Errorf("Body = %q, want %q", got.Body, "persistent")
	}
}

func TestDoltStoreNilMetadata(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	obj := Object{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-nilmeta",
		ObjectKind:  "choir.source_entity", OwnerID: "user:alice",
		ContentHash: "sha256:nilmeta",
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now, UpdatedAt: now,
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if string(got.Metadata) != "{}" {
		t.Errorf("Metadata = %q, want {}", got.Metadata)
	}
}

func TestDoltStoreLargeBody(t *testing.T) {
	ctx := context.Background()
	store := openTestDoltStore(t)
	defer store.Close()

	now := time.Unix(1700000000, 0).UTC()
	largeBody := make([]byte, 64*1024)
	for i := range largeBody {
		largeBody[i] = byte(i % 256)
	}
	obj := Object{
		CanonicalID: "obj:choir.web_capture:dXNlcjphbGljZQ:sha256-large",
		ObjectKind:  "choir.web_capture", OwnerID: "user:alice",
		ContentHash: "sha256:large", Body: largeBody,
		Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if len(got.Body) != len(largeBody) {
		t.Errorf("Body length = %d, want %d", len(got.Body), len(largeBody))
	}
}

func TestDoltStoreImplementsStoreInterface(t *testing.T) {
	var _ Store = (*DoltStore)(nil)
}

// Ensure the test binary can find the Dolt ICU data directory. On NixOS
// (and the repo dev shell), this is handled by the flake. This check is a
// no-op when the env is already correct.
func TestMain(m *testing.M) {
	// The embedded Dolt driver needs ICU data. In the Nix dev shell this is
	// on PATH. Outside it, tests may fail — that's expected.
	os.Exit(m.Run())
}
