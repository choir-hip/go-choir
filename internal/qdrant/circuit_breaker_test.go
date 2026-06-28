package qdrant

import (
	"context"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/health"
)

type stubAPI struct {
	searchFn func(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error)
}

func (s *stubAPI) CreateCollection(ctx context.Context, name string, cfg CollectionConfig) error {
	return nil
}
func (s *stubAPI) DeleteCollection(ctx context.Context, name string) error { return nil }
func (s *stubAPI) GetCollectionInfo(ctx context.Context, name string) (CollectionInfo, error) {
	return CollectionInfo{}, nil
}
func (s *stubAPI) UpsertPoints(ctx context.Context, collectionName string, points []Point) error {
	return nil
}
func (s *stubAPI) Search(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, collectionOrAlias, vector, limit)
	}
	return nil, nil
}
func (s *stubAPI) ListAliases(ctx context.Context) ([]AliasInfo, error) { return nil, nil }
func (s *stubAPI) UpdateAliases(ctx context.Context, actions []AliasAction) error {
	return nil
}
func (s *stubAPI) CreatePayloadIndex(ctx context.Context, collectionName, fieldName, fieldType string) error {
	return nil
}

type stubEmbedder struct {
	embedFn func(ctx context.Context, texts []string) ([][]float32, error)
}

func (e *stubEmbedder) Model() EmbeddingModel { return EmbeddingModel{Name: "stub", Version: "1", Dimensions: 8} }
func (e *stubEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if e.embedFn != nil {
		return e.embedFn(ctx, texts)
	}
	return make([][]float32, len(texts)), nil
}

func TestCircuitBreakingAPI_ForwardsWhenClosed(t *testing.T) {
	api := &stubAPI{}
	c := NewCircuitBreakingAPI(api, health.BreakerConfig{FailureThreshold: 3, OpenTimeout: 3600})
	if _, err := c.Search(context.Background(), "col", []float32{1}, 1); err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if c.Breaker().State() != health.StateClosed {
		t.Fatalf("state = %v, want closed", c.Breaker().State())
	}
}

func TestCircuitBreakingAPI_OpensOnFailures(t *testing.T) {
	api := &stubAPI{searchFn: func(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error) {
		return nil, errors.New("qdrant unreachable")
	}}
	c := NewCircuitBreakingAPI(api, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: 3600})
	_, _ = c.Search(context.Background(), "col", []float32{1}, 1)
	_, _ = c.Search(context.Background(), "col", []float32{1}, 1)
	if c.Breaker().State() != health.StateOpen {
		t.Fatalf("state = %v, want open", c.Breaker().State())
	}
	_, err := c.Search(context.Background(), "col", []float32{1}, 1)
	if err == nil {
		t.Fatal("expected circuit-open error, got nil")
	}
}

func TestCircuitBreakingEmbedder_ForwardsWhenClosed(t *testing.T) {
	e := &stubEmbedder{}
	ce := NewCircuitBreakingEmbedder(e, health.BreakerConfig{FailureThreshold: 3, OpenTimeout: 3600})
	if _, err := ce.EmbedTexts(context.Background(), []string{"a"}); err != nil {
		t.Fatalf("EmbedTexts error: %v", err)
	}
	if ce.Model().Name != "stub" {
		t.Fatalf("Model name = %q", ce.Model().Name)
	}
}

func TestCircuitBreakingEmbedder_OpensOnFailures(t *testing.T) {
	e := &stubEmbedder{embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
		return nil, errors.New("ollama unreachable")
	}}
	ce := NewCircuitBreakingEmbedder(e, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: 3600})
	_, _ = ce.EmbedTexts(context.Background(), []string{"a"})
	_, _ = ce.EmbedTexts(context.Background(), []string{"a"})
	if ce.Breaker().State() != health.StateOpen {
		t.Fatalf("state = %v, want open", ce.Breaker().State())
	}
	_, err := ce.EmbedTexts(context.Background(), []string{"a"})
	if err == nil {
		t.Fatal("expected circuit-open error, got nil")
	}
}
