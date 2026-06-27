package qdrant

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

type Pipeline struct {
	client   API
	embedder Embedder
}

type BuildSpec struct {
	OwnerID      string
	ObjectKind   objectgraph.ObjectKind
	IndexVersion int
	Filter       objectgraph.ListFilter
}

type BuildResult struct {
	Alias              string
	NewCollection      string
	PreviousCollection string
	PointCount         int
	EmbeddingModel     EmbeddingModel
}

func NewPipeline(client API, embedder Embedder) *Pipeline {
	return &Pipeline{client: client, embedder: embedder}
}

// Client returns the underlying Qdrant API client.
func (p *Pipeline) Client() API {
	return p.client
}

// Embedder returns the underlying embedder used by the pipeline. Callers that
// need to embed arbitrary texts (e.g. semantic dedup probes) use this instead
// of constructing a separate embedder.
func (p *Pipeline) Embedder() Embedder {
	return p.embedder
}

func (p *Pipeline) BuildFromObjectSource(ctx context.Context, source ObjectSource, spec BuildSpec) (BuildResult, error) {
	if spec.Filter.Kind == "" {
		spec.Filter.Kind = spec.ObjectKind
	}
	if spec.Filter.OwnerID == "" {
		spec.Filter.OwnerID = spec.OwnerID
	}
	objects, err := ListIndexableObjects(ctx, source, spec.Filter)
	if err != nil {
		return BuildResult{}, err
	}
	return p.BuildFromIndexedObjects(ctx, spec.OwnerID, spec.ObjectKind, spec.IndexVersion, objects)
}

func (p *Pipeline) BuildFromIndexedObjects(ctx context.Context, ownerID string, objectKind objectgraph.ObjectKind, indexVersion int, objects []IndexedObject) (BuildResult, error) {
	if ownerID == "" {
		return BuildResult{}, fmt.Errorf("owner_id is required")
	}
	if objectKind == "" {
		return BuildResult{}, fmt.Errorf("object_kind is required")
	}
	if indexVersion <= 0 {
		return BuildResult{}, fmt.Errorf("index_version must be positive")
	}
	model := p.embedder.Model()
	cfg, err := CollectionConfigForModel(model)
	if err != nil {
		return BuildResult{}, err
	}
	alias := AliasName(ownerID, string(objectKind))
	newCollection := CollectionName(ownerID, string(objectKind), indexVersion)
	result := BuildResult{
		Alias:          alias,
		NewCollection:  newCollection,
		PointCount:     len(objects),
		EmbeddingModel: model,
	}

	if err := p.client.CreateCollection(ctx, newCollection, cfg); err != nil {
		return BuildResult{}, fmt.Errorf("create shadow collection: %w", err)
	}
	created := true
	defer func() {
		if created {
			_ = p.client.DeleteCollection(ctx, newCollection)
		}
	}()

	points, err := p.pointsForObjects(ctx, objects, model)
	if err != nil {
		return BuildResult{}, err
	}
	if err := p.client.UpsertPoints(ctx, newCollection, points); err != nil {
		return BuildResult{}, fmt.Errorf("upsert points: %w", err)
	}
	if err := p.VerifyCollection(ctx, newCollection, len(points)); err != nil {
		return BuildResult{}, fmt.Errorf("verify shadow collection: %w", err)
	}

	previous, err := p.currentCollection(ctx, alias)
	if err != nil {
		return BuildResult{}, err
	}
	result.PreviousCollection = previous
	if err := p.switchAlias(ctx, alias, previous, newCollection); err != nil {
		return BuildResult{}, fmt.Errorf("switch alias: %w", err)
	}
	created = false
	return result, nil
}

func (p *Pipeline) VerifyCollection(ctx context.Context, collectionName string, expectedCount int) error {
	info, err := p.client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("get collection info: %w", err)
	}
	if info.PointsCount != expectedCount {
		return fmt.Errorf("point count mismatch: got %d, expected %d", info.PointsCount, expectedCount)
	}
	if expectedCount == 0 {
		return nil
	}
	model := p.embedder.Model()
	vectors, err := p.embedder.EmbedTexts(ctx, []string{"qdrant collection verification"})
	if err != nil {
		return fmt.Errorf("embed verification query: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != model.Dimensions {
		return fmt.Errorf("verification vector dimensions mismatch")
	}
	results, err := p.client.Search(ctx, collectionName, vectors[0], 1)
	if err != nil {
		return fmt.Errorf("sample search: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("sample search returned no results despite %d points", expectedCount)
	}
	return nil
}

func (p *Pipeline) RollbackAlias(ctx context.Context, alias, previousCollection string) error {
	if alias == "" {
		return fmt.Errorf("alias is required")
	}
	if previousCollection == "" {
		return fmt.Errorf("previous collection is required")
	}
	current, err := p.currentCollection(ctx, alias)
	if err != nil {
		return err
	}
	return p.switchAlias(ctx, alias, current, previousCollection)
}

func (p *Pipeline) GarbageCollectCollection(ctx context.Context, collectionName string) error {
	if collectionName == "" {
		return nil
	}
	return p.client.DeleteCollection(ctx, collectionName)
}

func (p *Pipeline) pointsForObjects(ctx context.Context, objects []IndexedObject, model EmbeddingModel) ([]Point, error) {
	texts := make([]string, len(objects))
	for i, obj := range objects {
		texts[i] = obj.Text
	}
	vectors, err := p.embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed texts: %w", err)
	}
	if len(vectors) != len(objects) {
		return nil, fmt.Errorf("embedder returned %d vectors for %d texts", len(vectors), len(objects))
	}
	points := make([]Point, len(objects))
	for i, obj := range objects {
		if len(vectors[i]) != model.Dimensions {
			return nil, fmt.Errorf("vector %d has %d dimensions, want %d", i, len(vectors[i]), model.Dimensions)
		}
		points[i] = Point{
			ID:     PointIDForCanonicalID(obj.CanonicalID),
			Vector: vectors[i],
			Payload: PointPayload{
				CanonicalID:      obj.CanonicalID,
				ObjectKind:       string(obj.ObjectKind),
				ContentHash:      obj.ContentHash,
				OwnerID:          obj.OwnerID,
				ComputerID:       obj.ComputerID,
				VersionID:        obj.VersionID,
				Text:             obj.Text,
				EmbeddingModel:   model.Name,
				EmbeddingVersion: model.Version,
				Metadata:         obj.Metadata,
			},
		}
	}
	return points, nil
}

func (p *Pipeline) currentCollection(ctx context.Context, alias string) (string, error) {
	aliases, err := p.client.ListAliases(ctx)
	if err != nil {
		return "", fmt.Errorf("list aliases: %w", err)
	}
	for _, info := range aliases {
		if info.AliasName == alias {
			return info.CollectionName, nil
		}
	}
	return "", nil
}

func (p *Pipeline) switchAlias(ctx context.Context, alias, previousCollection, newCollection string) error {
	if alias == "" || newCollection == "" {
		return fmt.Errorf("alias and new collection are required")
	}
	actions := []AliasAction{}
	if previousCollection != "" {
		actions = append(actions, AliasAction{DeleteAlias: &DeleteAlias{AliasName: alias}})
	}
	actions = append(actions, AliasAction{CreateAlias: &CreateAlias{
		CollectionName: newCollection,
		AliasName:      alias,
	}})
	return p.client.UpdateAliases(ctx, actions)
}
