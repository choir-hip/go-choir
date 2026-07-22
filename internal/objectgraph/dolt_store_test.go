package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	embedded "github.com/dolthub/driver"
)

func openTestDoltDB(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS testdb"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=testdb&multistatements=true&clientfoundrows=true", root)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	t.Cleanup(func() {
		_ = db.Close()
		_ = dbConnector.Close()
	})
	return db
}

func TestDoltStoreEnsureSchema(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Verify tables exist by inserting and reading back.
	obj := Object{
		CanonicalID: "obj:choir.agent:test-owner:key-test",
		ObjectKind:  "choir.agent",
		OwnerID:     "test-owner",
		ContentHash: "sha256:abc123",
		Body:        []byte(`{"name":"test"}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.PutObject(ctx, obj); err != nil {
		t.Fatalf("put object: %v", err)
	}

	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	if got.CanonicalID != obj.CanonicalID {
		t.Errorf("canonical_id: got %q, want %q", got.CanonicalID, obj.CanonicalID)
	}
	if got.ObjectKind != obj.ObjectKind {
		t.Errorf("object_kind: got %q, want %q", got.ObjectKind, obj.ObjectKind)
	}
	if string(got.Body) != string(obj.Body) {
		t.Errorf("body: got %q, want %q", got.Body, obj.Body)
	}
}

func TestDoltStoreListObjects(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC()
	for i := range 5 {
		obj := Object{
			CanonicalID: fmt.Sprintf("obj:choir.agent:owner%d:key-%d", i, i),
			ObjectKind:  "choir.agent",
			OwnerID:     "owner0",
			ContentHash: fmt.Sprintf("sha256:hash%d", i),
			Body:        []byte(`{}`),
			Metadata:    json.RawMessage(`{}`),
			CreatedAt:   now.Add(time.Duration(i) * time.Second),
			UpdatedAt:   now.Add(time.Duration(i) * time.Second),
		}
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("put object %d: %v", i, err)
		}
	}

	// Add a different owner's object to verify filtering.
	other := Object{
		CanonicalID: "obj:choir.agent:otherowner:key-other",
		ObjectKind:  "choir.agent",
		OwnerID:     "otherowner",
		ContentHash: "sha256:other",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.PutObject(ctx, other); err != nil {
		t.Fatalf("put other object: %v", err)
	}

	// List by owner.
	objs, err := store.ListObjects(ctx, ListFilter{Kind: "choir.agent", OwnerID: "owner0", Limit: 10})
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}
	for _, o := range objs {
		if o.OwnerID != "owner0" {
			t.Errorf("unexpected owner_id %q", o.OwnerID)
		}
	}
}

func TestDoltStoreListObjectsByMetadataPage(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	tiedTimestamp := time.Now().UTC()
	objects := []Object{
		{CanonicalID: "obj:choir.run:owner:key-01", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:01", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-02", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:02", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"other"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-03", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:03", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.agent:owner:key-04", ObjectKind: "choir.agent", OwnerID: "owner", ContentHash: "sha256:04", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-05", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:05", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-06", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:06", Body: []byte(`{}`), Metadata: json.RawMessage(`{"different_field":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-07", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:07", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
		{CanonicalID: "obj:choir.run:owner:key-09", ObjectKind: "choir.run", OwnerID: "owner", ContentHash: "sha256:09", Body: []byte(`{}`), Metadata: json.RawMessage(`{"trajectory_id":"target"}`), CreatedAt: tiedTimestamp, UpdatedAt: tiedTimestamp},
	}
	for _, obj := range objects {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("put object %s: %v", obj.CanonicalID, err)
		}
	}

	var gotIDs []string
	cursor := ""
	for {
		page, err := store.ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", cursor, 2)
		if err != nil {
			t.Fatalf("list page after %q: %v", cursor, err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("page after %q exceeded limit: got %d objects", cursor, len(page))
		}
		for _, obj := range page {
			if obj.CanonicalID <= cursor {
				t.Fatalf("page after %q returned non-advancing ID %q", cursor, obj.CanonicalID)
			}
			gotIDs = append(gotIDs, obj.CanonicalID)
		}
		cursor = page[len(page)-1].CanonicalID
	}

	wantIDs := []string{
		"obj:choir.run:owner:key-01",
		"obj:choir.run:owner:key-03",
		"obj:choir.run:owner:key-05",
		"obj:choir.run:owner:key-07",
		"obj:choir.run:owner:key-09",
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("paged IDs: got %v, want %v", gotIDs, wantIDs)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("paged IDs: got %v, want %v", gotIDs, wantIDs)
		}
	}

	strictPage, err := store.ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", "obj:choir.run:owner:key-03", 2)
	if err != nil {
		t.Fatalf("list strict cursor page: %v", err)
	}
	if len(strictPage) != 2 {
		t.Fatalf("strict cursor page: got %d objects, want 2", len(strictPage))
	}
	if strictPage[0].CanonicalID != "obj:choir.run:owner:key-05" || strictPage[1].CanonicalID != "obj:choir.run:owner:key-07" {
		t.Fatalf("strict cursor page: got IDs %q and %q", strictPage[0].CanonicalID, strictPage[1].CanonicalID)
	}

	finalPage, err := store.ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", wantIDs[len(wantIDs)-1], 2)
	if err != nil {
		t.Fatalf("list final page: %v", err)
	}
	if len(finalPage) != 0 {
		t.Fatalf("final page: got %d objects, want 0", len(finalPage))
	}

	defaultLimitPage, err := store.ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", "", 0)
	if err != nil {
		t.Fatalf("list page with normalized limit: %v", err)
	}
	if len(defaultLimitPage) != len(wantIDs) {
		t.Fatalf("normalized limit page: got %d objects, want %d", len(defaultLimitPage), len(wantIDs))
	}
	var allRunIDs []string
	cursor = ""
	for {
		page, err := store.ListObjectsPage(ctx, "choir.run", cursor, 3)
		if err != nil {
			t.Fatalf("list object-kind page after %q: %v", cursor, err)
		}
		if len(page) == 0 {
			break
		}
		for _, obj := range page {
			allRunIDs = append(allRunIDs, obj.CanonicalID)
		}
		cursor = page[len(page)-1].CanonicalID
	}
	if len(allRunIDs) != 7 || allRunIDs[0] != "obj:choir.run:owner:key-01" || allRunIDs[6] != "obj:choir.run:owner:key-09" {
		t.Fatalf("object-kind pages = %v, want all seven run objects in canonical order", allRunIDs)
	}
}

func TestDoltStoreListObjectsByMetadataPageErrors(t *testing.T) {
	ctx := context.Background()

	var nilStore *DoltStore
	_, err := nilStore.ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", "", 2)
	if err == nil {
		t.Fatal("nil receiver: expected error")
	}
	if got, want := err.Error(), "objectgraph dolt: nil store"; got != want {
		t.Fatalf("nil receiver error: got %q, want %q", got, want)
	}

	_, err = NewDoltStore(nil).ListObjectsByMetadataPage(ctx, "choir.run", "$.trajectory_id", "target", "", 2)
	if err == nil {
		t.Fatal("nil database: expected error")
	}
	if got, want := err.Error(), "objectgraph dolt: nil store"; got != want {
		t.Fatalf("nil database error: got %q, want %q", got, want)
	}

	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := store.ListObjectsByMetadataPage(ctx, "choir.run", "not-a-json-path", "target", "", 2); err == nil {
		t.Fatal("invalid JSON path: expected error")
	}
}

func TestDoltStorePutAndListEdges(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC()
	fromObj := Object{
		CanonicalID: "obj:choir.run:owner1:key-run1",
		ObjectKind:  "choir.run",
		OwnerID:     "owner1",
		ContentHash: "sha256:run1",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	toObj := Object{
		CanonicalID: "obj:choir.agent:owner1:key-agent1",
		ObjectKind:  "choir.agent",
		OwnerID:     "owner1",
		ContentHash: "sha256:agent1",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.PutObject(ctx, fromObj); err != nil {
		t.Fatalf("put from object: %v", err)
	}
	if err := store.PutObject(ctx, toObj); err != nil {
		t.Fatalf("put to object: %v", err)
	}

	edgeID, err := BuildEdgeID(fromObj.CanonicalID, toObj.CanonicalID, "run_agent", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("build edge id: %v", err)
	}
	edge := Edge{
		EdgeID:    edgeID,
		FromID:    fromObj.CanonicalID,
		ToID:      toObj.CanonicalID,
		Kind:      "run_agent",
		Metadata:  json.RawMessage(`{}`),
		CreatedAt: now,
	}
	if err := store.PutEdge(ctx, edge); err != nil {
		t.Fatalf("put edge: %v", err)
	}

	// List edges from the run object.
	edges, err := store.ListEdges(ctx, EdgeFilter{FromID: fromObj.CanonicalID, Limit: 10})
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].ToID != toObj.CanonicalID {
		t.Errorf("to_id: got %q, want %q", edges[0].ToID, toObj.CanonicalID)
	}
	if edges[0].Kind != "run_agent" {
		t.Errorf("kind: got %q, want %q", edges[0].Kind, "run_agent")
	}
}

func TestDoltStorePutBatch(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC()
	obj1 := Object{
		CanonicalID: "obj:choir.run:owner2:key-run2",
		ObjectKind:  "choir.run",
		OwnerID:     "owner2",
		ContentHash: "sha256:run2",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	obj2 := Object{
		CanonicalID: "obj:choir.agent:owner2:key-agent2",
		ObjectKind:  "choir.agent",
		OwnerID:     "owner2",
		ContentHash: "sha256:agent2",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	edgeID, _ := BuildEdgeID(obj1.CanonicalID, obj2.CanonicalID, "run_agent", json.RawMessage(`{}`))
	edge := Edge{
		EdgeID:    edgeID,
		FromID:    obj1.CanonicalID,
		ToID:      obj2.CanonicalID,
		Kind:      "run_agent",
		Metadata:  json.RawMessage(`{}`),
		CreatedAt: now,
	}

	batch := Batch{Objects: []Object{obj1, obj2}, Edges: []Edge{edge}}
	if err := store.PutBatch(ctx, batch); err != nil {
		t.Fatalf("put batch: %v", err)
	}

	// Verify all writes landed.
	got1, _ := store.GetObject(ctx, obj1.CanonicalID)
	if got1.CanonicalID != obj1.CanonicalID {
		t.Errorf("obj1 canonical_id: got %q", got1.CanonicalID)
	}
	got2, _ := store.GetObject(ctx, obj2.CanonicalID)
	if got2.CanonicalID != obj2.CanonicalID {
		t.Errorf("obj2 canonical_id: got %q", got2.CanonicalID)
	}
	edges, _ := store.ListEdges(ctx, EdgeFilter{FromID: obj1.CanonicalID, Limit: 10})
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}

func TestDoltStorePutBatchConditionalIsAtomic(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC()
	current := Object{
		CanonicalID: "obj:choir.trajectory:owner:key-current",
		ObjectKind:  "choir.trajectory",
		OwnerID:     "owner",
		VersionID:   "v1",
		ContentHash: "sha256:v1",
		Body:        []byte(`{"version":1}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.PutObject(ctx, current); err != nil {
		t.Fatalf("seed object: %v", err)
	}

	next := current
	next.VersionID = "v2"
	next.ContentHash = "sha256:v2"
	next.Body = []byte(`{"version":2}`)
	created := Object{
		CanonicalID: "obj:choir.work_item:owner:key-created",
		ObjectKind:  "choir.work_item",
		OwnerID:     "owner",
		VersionID:   "v1",
		ContentHash: "sha256:created",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.PutBatchConditional(ctx, []ObjectCondition{
		{CanonicalID: current.CanonicalID, Exists: true, ExpectedVersionID: "v1"},
		{CanonicalID: created.CanonicalID, Exists: false},
	}, Batch{Objects: []Object{next, created}}); err != nil {
		t.Fatalf("conditional batch: %v", err)
	}

	staleWrite := next
	staleWrite.VersionID = "v3"
	staleWrite.ContentHash = "sha256:v3"
	orphan := created
	orphan.CanonicalID = "obj:choir.work_item:owner:key-orphan"
	err := store.PutBatchConditional(ctx, []ObjectCondition{
		{CanonicalID: current.CanonicalID, Exists: true, ExpectedVersionID: "v1"},
		{CanonicalID: orphan.CanonicalID, Exists: false},
	}, Batch{Objects: []Object{staleWrite, orphan}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale conditional error = %v, want ErrConflict", err)
	}
	got, err := store.GetObject(ctx, current.CanonicalID)
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if got.VersionID != "v2" {
		t.Fatalf("current version = %q, want v2", got.VersionID)
	}
	if _, err := store.GetObject(ctx, orphan.CanonicalID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("orphan lookup error = %v, want ErrNotFound", err)
	}
}

func TestDoltStoreGetObjectNotFound(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	_, err := store.GetObject(ctx, "obj:choir.agent:no-such-owner:key-missing")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDoltStoreListEdgesByKind(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC()
	fromObj := Object{
		CanonicalID: "obj:choir.run:owner3:key-run3",
		ObjectKind:  "choir.run",
		OwnerID:     "owner3",
		ContentHash: "sha256:run3",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	agentObj := Object{
		CanonicalID: "obj:choir.agent:owner3:key-agent3",
		ObjectKind:  "choir.agent",
		OwnerID:     "owner3",
		ContentHash: "sha256:agent3",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	trajObj := Object{
		CanonicalID: "obj:choir.trajectory:owner3:key-traj3",
		ObjectKind:  "choir.trajectory",
		OwnerID:     "owner3",
		ContentHash: "sha256:traj3",
		Body:        []byte(`{}`),
		Metadata:    json.RawMessage(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, obj := range []Object{fromObj, agentObj, trajObj} {
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("put object: %v", err)
		}
	}

	edge1, _ := BuildEdgeID(fromObj.CanonicalID, agentObj.CanonicalID, "run_agent", json.RawMessage(`{}`))
	edge2, _ := BuildEdgeID(fromObj.CanonicalID, trajObj.CanonicalID, "run_trajectory", json.RawMessage(`{}`))
	for _, edge := range []Edge{
		{EdgeID: edge1, FromID: fromObj.CanonicalID, ToID: agentObj.CanonicalID, Kind: "run_agent", Metadata: json.RawMessage(`{}`), CreatedAt: now},
		{EdgeID: edge2, FromID: fromObj.CanonicalID, ToID: trajObj.CanonicalID, Kind: "run_trajectory", Metadata: json.RawMessage(`{}`), CreatedAt: now},
	} {
		if err := store.PutEdge(ctx, edge); err != nil {
			t.Fatalf("put edge: %v", err)
		}
	}

	// List only run_agent edges.
	edges, err := store.ListEdgesByKind(ctx, fromObj.CanonicalID, "run_agent")
	if err != nil {
		t.Fatalf("list edges by kind: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 run_agent edge, got %d", len(edges))
	}
	if edges[0].ToID != agentObj.CanonicalID {
		t.Errorf("to_id: got %q, want %q", edges[0].ToID, agentObj.CanonicalID)
	}
}

func TestDoltStoreWithService(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	svc := NewService(Config{Durable: store})
	obj, err := svc.CreateObject(ctx, CreateObjectRequest{
		Kind:        "choir.agent",
		OwnerID:     "svc-owner",
		IdentityKey: "agent-42",
		Body:        []byte(`{"name":"agent-42"}`),
		Metadata:    map[string]any{"role": "researcher"},
	})
	if err != nil {
		t.Fatalf("create object: %v", err)
	}

	// Read it back through the store directly.
	got, err := store.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	if got.OwnerID != "svc-owner" {
		t.Errorf("owner_id: got %q", got.OwnerID)
	}

	// List through the service.
	objs, err := svc.ListObjects(ctx, ListFilter{Kind: "choir.agent", OwnerID: "svc-owner", Limit: 10})
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
}
