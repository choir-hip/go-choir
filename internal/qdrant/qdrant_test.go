package qdrant

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestProjectObjectsUsesObjectGraphTruth(t *testing.T) {
	ctx := context.Background()
	store := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: store, Durable: store})
	defer svc.Close()

	obj, err := svc.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:       "choir.source_entity",
		OwnerID:    "user:alice",
		ComputerID: "computer:local",
		VersionID:  "v1",
		Body:       []byte(" Source-grounded retrieval text. "),
		Metadata:   map[string]any{"display_title": "Source A"},
		Now:        time.Unix(10, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:     "choir.media_item",
		OwnerID:  "user:alice",
		Body:     []byte{0xff, 0xfe},
		Metadata: nil,
	}); err != nil {
		t.Fatal(err)
	}

	indexed, err := ListIndexableObjects(ctx, svc, objectgraph.ListFilter{OwnerID: "user:alice"})
	if err != nil {
		t.Fatalf("ListIndexableObjects() error = %v", err)
	}
	if len(indexed) != 1 {
		t.Fatalf("indexed %d objects, want 1", len(indexed))
	}
	got := indexed[0]
	if got.CanonicalID != obj.CanonicalID || got.ContentHash != obj.ContentHash {
		t.Fatalf("projection lost object identity: %#v vs %#v", got, obj)
	}
	if got.Text != "Source-grounded retrieval text." {
		t.Fatalf("projected text = %q", got.Text)
	}
	if !json.Valid(got.Metadata) || !strings.Contains(string(got.Metadata), "display_title") {
		t.Fatalf("metadata not preserved: %s", got.Metadata)
	}
}

func TestNamingAndPointIDAreQdrantSafeAndStable(t *testing.T) {
	name := CollectionName("user:alice", "choir.source_entity", 2)
	if strings.ContainsAny(name, ":.") {
		t.Fatalf("collection name contains unsafe punctuation: %s", name)
	}
	if name != CollectionName("user:alice", "choir.source_entity", 2) {
		t.Fatal("collection name should be stable")
	}

	first := PointIDForCanonicalID("obj:choir.source_entity:dXNlcjphbGljZQ:abc")
	second := PointIDForCanonicalID("obj:choir.source_entity:dXNlcjphbGljZQ:abc")
	if first != second || len(first) != 36 {
		t.Fatalf("point id not stable UUID-shaped value: %q %q", first, second)
	}
}

func TestPipelineBuildsFromObjectGraphAndCreatesAlias(t *testing.T) {
	ctx := context.Background()
	store := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: store, Durable: store})
	defer svc.Close()

	source, err := svc.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:     "choir.source_entity",
		OwnerID:  "user:alice",
		Body:     []byte("qdrant derived index source"),
		Metadata: map[string]any{"kind": "web_url"},
	})
	if err != nil {
		t.Fatal(err)
	}

	api := newFakeAPI()
	pipeline := NewPipeline(api, newTestEmbedder(8))
	result, err := pipeline.BuildFromObjectSource(ctx, svc, BuildSpec{
		OwnerID:      "user:alice",
		ObjectKind:   "choir.source_entity",
		IndexVersion: 1,
	})
	if err != nil {
		t.Fatalf("BuildFromObjectSource() error = %v", err)
	}
	if result.PointCount != 1 || result.PreviousCollection != "" {
		t.Fatalf("result = %#v, want one point and no previous collection", result)
	}
	if len(api.upserts) != 1 || len(api.upserts[0].points) != 1 {
		t.Fatalf("upserts = %#v", api.upserts)
	}
	point := api.upserts[0].points[0]
	if point.ID == source.CanonicalID {
		t.Fatal("point id should be deterministic Qdrant-safe id, not canonical id")
	}
	if point.Payload.CanonicalID != source.CanonicalID || point.Payload.ContentHash != source.ContentHash {
		t.Fatalf("payload lost source-of-truth refs: %#v", point.Payload)
	}
	if len(api.aliasUpdates) != 1 || len(api.aliasUpdates[0]) != 1 || api.aliasUpdates[0][0].CreateAlias == nil {
		t.Fatalf("create alias actions = %#v", api.aliasUpdates)
	}
}

func TestPipelineSwitchesExistingAliasWithDeleteCreateTransaction(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI()
	api.aliases = []AliasInfo{{
		AliasName:      AliasName("user:alice", "choir.source_entity"),
		CollectionName: "old_collection",
	}}
	pipeline := NewPipeline(api, newTestEmbedder(8))

	result, err := pipeline.BuildFromIndexedObjects(ctx, "user:alice", "choir.source_entity", 2, []IndexedObject{{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:abc",
		ObjectKind:  "choir.source_entity",
		ContentHash: "sha256:abc",
		OwnerID:     "user:alice",
		Text:        "second index build",
		Metadata:    json.RawMessage(`{"n":2}`),
	}})
	if err != nil {
		t.Fatalf("BuildFromIndexedObjects() error = %v", err)
	}
	if result.PreviousCollection != "old_collection" {
		t.Fatalf("previous collection = %q", result.PreviousCollection)
	}
	if len(api.aliasUpdates) != 1 {
		t.Fatalf("alias updates = %#v", api.aliasUpdates)
	}
	actions := api.aliasUpdates[0]
	if len(actions) != 2 || actions[0].DeleteAlias == nil || actions[1].CreateAlias == nil {
		t.Fatalf("switch actions = %#v, want delete_alias then create_alias", actions)
	}
	if actions[0].DeleteAlias.AliasName != result.Alias || actions[1].CreateAlias.AliasName != result.Alias {
		t.Fatalf("actions target wrong alias: %#v", actions)
	}
}

func TestPipelineDeletesShadowCollectionOnVerificationFailure(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI()
	wrongCount := -1
	api.forcePointCount = &wrongCount
	pipeline := NewPipeline(api, newTestEmbedder(8))

	_, err := pipeline.BuildFromIndexedObjects(ctx, "user:alice", "choir.source_entity", 1, []IndexedObject{{
		CanonicalID: "obj:choir.source_entity:dXNlcjphbGljZQ:abc",
		ObjectKind:  "choir.source_entity",
		ContentHash: "sha256:abc",
		OwnerID:     "user:alice",
		Text:        "bad count",
		Metadata:    json.RawMessage(`{}`),
	}})
	if err == nil {
		t.Fatal("BuildFromIndexedObjects() succeeded, want verification error")
	}
	if len(api.deleted) != 1 {
		t.Fatalf("deleted collections = %#v, want shadow collection cleanup", api.deleted)
	}
	if len(api.aliasUpdates) != 0 {
		t.Fatalf("alias should not change on verification failure: %#v", api.aliasUpdates)
	}
}

func TestLocalQdrantBuildAndSwitchIfAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping local Qdrant integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient("http://localhost:6333")
	if err := client.Health(ctx); err != nil {
		t.Skipf("local Qdrant unavailable: %v", err)
	}

	ownerID := "user:o2-local-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	objectKind := objectgraph.ObjectKind("choir.source_entity")
	pipeline := NewPipeline(client, newTestEmbedder(8))
	objects := []IndexedObject{{
		CanonicalID: "obj:choir.source_entity:dXNlcjpvMi1sb2NhbA:o2local1",
		ObjectKind:  objectKind,
		ContentHash: "sha256:o2local1",
		OwnerID:     ownerID,
		Text:        "local qdrant o2 integration text",
		Metadata:    json.RawMessage(`{"test":"o2"}`),
	}}

	first, err := pipeline.BuildFromIndexedObjects(ctx, ownerID, objectKind, 1, objects)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	defer func() { _ = client.DeleteCollection(context.Background(), first.NewCollection) }()

	second, err := pipeline.BuildFromIndexedObjects(ctx, ownerID, objectKind, 2, objects)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	defer func() { _ = client.DeleteCollection(context.Background(), second.NewCollection) }()
	if second.PreviousCollection != first.NewCollection {
		t.Fatalf("second previous collection = %q, want %q", second.PreviousCollection, first.NewCollection)
	}
	if err := pipeline.RollbackAlias(ctx, second.Alias, second.PreviousCollection); err != nil {
		t.Fatalf("rollback alias: %v", err)
	}
}

type testEmbedder struct {
	dims int
}

func newTestEmbedder(dims int) *testEmbedder {
	return &testEmbedder{dims: dims}
}

func (e *testEmbedder) Model() EmbeddingModel {
	return EmbeddingModel{Name: "test-hash", Version: "v1", Dimensions: e.dims}
}

func (e *testEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = hashVector(text, e.dims)
	}
	return out, nil
}

func hashVector(text string, dims int) []float32 {
	vec := make([]float32, dims)
	for i := range dims {
		var counter [4]byte
		binary.BigEndian.PutUint32(counter[:], uint32(i))
		sum := sha256.Sum256(append(counter[:], []byte(text)...))
		raw := binary.BigEndian.Uint32(sum[:4])
		vec[i] = (float32(raw) / float32(^uint32(0))) - 0.5
	}
	return vec
}

type fakeAPI struct {
	collections     map[string]CollectionConfig
	deleted         []string
	upserts         []fakeUpsert
	aliases         []AliasInfo
	aliasUpdates    [][]AliasAction
	forcePointCount *int
}

type fakeUpsert struct {
	collection string
	points     []Point
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		collections: map[string]CollectionConfig{},
	}
}

func (f *fakeAPI) CreateCollection(_ context.Context, name string, cfg CollectionConfig) error {
	f.collections[name] = cfg
	return nil
}

func (f *fakeAPI) DeleteCollection(_ context.Context, name string) error {
	f.deleted = append(f.deleted, name)
	delete(f.collections, name)
	return nil
}

func (f *fakeAPI) GetCollectionInfo(_ context.Context, name string) (CollectionInfo, error) {
	if f.forcePointCount != nil {
		return CollectionInfo{PointsCount: *f.forcePointCount, Status: "green"}, nil
	}
	count := 0
	for _, upsert := range f.upserts {
		if upsert.collection == name {
			count += len(upsert.points)
		}
	}
	return CollectionInfo{PointsCount: count, Status: "green"}, nil
}

func (f *fakeAPI) UpsertPoints(_ context.Context, collectionName string, points []Point) error {
	f.upserts = append(f.upserts, fakeUpsert{
		collection: collectionName,
		points:     slices.Clone(points),
	})
	return nil
}

func (f *fakeAPI) Search(_ context.Context, collectionOrAlias string, _ []float32, _ int) ([]ScoredPoint, error) {
	for _, upsert := range f.upserts {
		if upsert.collection == collectionOrAlias && len(upsert.points) > 0 {
			return []ScoredPoint{{ID: upsert.points[0].ID, Score: 1, Payload: upsert.points[0].Payload}}, nil
		}
	}
	return nil, nil
}

func (f *fakeAPI) ListAliases(_ context.Context) ([]AliasInfo, error) {
	return slices.Clone(f.aliases), nil
}

func (f *fakeAPI) UpdateAliases(_ context.Context, actions []AliasAction) error {
	f.aliasUpdates = append(f.aliasUpdates, slices.Clone(actions))
	for _, action := range actions {
		if action.DeleteAlias != nil {
			f.aliases = slices.DeleteFunc(f.aliases, func(info AliasInfo) bool {
				return info.AliasName == action.DeleteAlias.AliasName
			})
		}
		if action.CreateAlias != nil {
			f.aliases = append(f.aliases, AliasInfo{
				AliasName:      action.CreateAlias.AliasName,
				CollectionName: action.CreateAlias.CollectionName,
			})
		}
	}
	return nil
}

func (f *fakeAPI) CreatePayloadIndex(_ context.Context, _, _, _ string) error {
	return nil
}
