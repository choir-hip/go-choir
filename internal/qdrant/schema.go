package qdrant

import (
	"context"
	"encoding/json"
	"fmt"
)

const DefaultDistance = "Cosine"

type EmbeddingModel struct {
	Name       string
	Version    string
	Dimensions int
}

func (m EmbeddingModel) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("embedding model name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("embedding model version is required")
	}
	if m.Dimensions <= 0 {
		return fmt.Errorf("embedding model dimensions must be positive")
	}
	return nil
}

// Embedder is the provider boundary for Qdrant indexing. Implementations may
// call any provider or local model whose capabilities match the turn; callers
// should not encode Choir role names or provider-specific routing here.
type Embedder interface {
	Model() EmbeddingModel
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}

type CollectionConfig struct {
	VectorSize int
	Distance   string
	OnDisk     bool
}

func CollectionConfigForModel(model EmbeddingModel) (CollectionConfig, error) {
	if err := model.Validate(); err != nil {
		return CollectionConfig{}, err
	}
	return CollectionConfig{
		VectorSize: model.Dimensions,
		Distance:   DefaultDistance,
		OnDisk:     false,
	}, nil
}

type Point struct {
	ID      string       `json:"id"`
	Vector  []float32    `json:"vector"`
	Payload PointPayload `json:"payload"`
}

type PointPayload struct {
	CanonicalID      string          `json:"canonical_id"`
	ObjectKind       string          `json:"object_kind"`
	ContentHash      string          `json:"content_hash"`
	OwnerID          string          `json:"owner_id"`
	ComputerID       string          `json:"computer_id,omitempty"`
	VersionID        string          `json:"version_id,omitempty"`
	Text             string          `json:"text,omitempty"`
	EmbeddingModel   string          `json:"embedding_model"`
	EmbeddingVersion string          `json:"embedding_version"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}

type CollectionInfo struct {
	PointsCount int
	Status      string
}

type ScoredPoint struct {
	ID      string
	Score   float32
	Payload PointPayload
}

type AliasInfo struct {
	AliasName      string `json:"alias_name"`
	CollectionName string `json:"collection_name"`
}

type CreateAlias struct {
	CollectionName string `json:"collection_name"`
	AliasName      string `json:"alias_name"`
}

type DeleteAlias struct {
	AliasName string `json:"alias_name"`
}

type AliasAction struct {
	CreateAlias *CreateAlias `json:"create_alias,omitempty"`
	DeleteAlias *DeleteAlias `json:"delete_alias,omitempty"`
}

type API interface {
	CreateCollection(ctx context.Context, name string, cfg CollectionConfig) error
	DeleteCollection(ctx context.Context, name string) error
	GetCollectionInfo(ctx context.Context, name string) (CollectionInfo, error)
	UpsertPoints(ctx context.Context, collectionName string, points []Point) error
	Search(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error)
	ListAliases(ctx context.Context) ([]AliasInfo, error)
	UpdateAliases(ctx context.Context, actions []AliasAction) error
	CreatePayloadIndex(ctx context.Context, collectionName, fieldName, fieldType string) error
}
