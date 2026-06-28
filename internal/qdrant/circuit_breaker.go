package qdrant

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/health"
)

// CircuitBreakingAPI wraps a qdrant.API with a circuit breaker. When the
// breaker is open, every method returns a circuit-open error without
// contacting Qdrant. Callers in the ingestion/dedup path already treat Qdrant
// errors as "skip semantic dedup, pass items through", so an open breaker
// degrades gracefully: content-hash dedup still runs and ingestion is not
// blocked (production-readiness checklist: "Qdrant failures degrade
// gracefully (skip dedup, don't block ingestion)").
//
// The breaker short-circuits faster than waiting for a TCP timeout once
// Qdrant has been observed failing repeatedly, which keeps the ingestion
// path responsive during outages.
type CircuitBreakingAPI struct {
	inner   API
	breaker *health.CircuitBreaker
}

// NewCircuitBreakingAPI wraps c with a circuit breaker.
func NewCircuitBreakingAPI(c API, cfg health.BreakerConfig) *CircuitBreakingAPI {
	return &CircuitBreakingAPI{inner: c, breaker: health.NewCircuitBreaker(cfg)}
}

// Breaker returns the underlying breaker for observability.
func (c *CircuitBreakingAPI) Breaker() *health.CircuitBreaker { return c.breaker }

func (c *CircuitBreakingAPI) CreateCollection(ctx context.Context, name string, cfg CollectionConfig) error {
	return c.breaker.Execute(func() error { return c.inner.CreateCollection(ctx, name, cfg) })
}

func (c *CircuitBreakingAPI) DeleteCollection(ctx context.Context, name string) error {
	return c.breaker.Execute(func() error { return c.inner.DeleteCollection(ctx, name) })
}

func (c *CircuitBreakingAPI) GetCollectionInfo(ctx context.Context, name string) (CollectionInfo, error) {
	var info CollectionInfo
	err := c.breaker.Execute(func() error {
		var callErr error
		info, callErr = c.inner.GetCollectionInfo(ctx, name)
		return callErr
	})
	if err == health.ErrCircuitOpen {
		return CollectionInfo{}, fmt.Errorf("qdrant: circuit open (upstream unhealthy)")
	}
	return info, err
}

func (c *CircuitBreakingAPI) UpsertPoints(ctx context.Context, collectionName string, points []Point) error {
	return c.breaker.Execute(func() error {
		return c.inner.UpsertPoints(ctx, collectionName, points)
	})
}

func (c *CircuitBreakingAPI) Search(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error) {
	var results []ScoredPoint
	err := c.breaker.Execute(func() error {
		var callErr error
		results, callErr = c.inner.Search(ctx, collectionOrAlias, vector, limit)
		return callErr
	})
	if err == health.ErrCircuitOpen {
		return nil, fmt.Errorf("qdrant: circuit open (upstream unhealthy)")
	}
	return results, err
}

func (c *CircuitBreakingAPI) ListAliases(ctx context.Context) ([]AliasInfo, error) {
	var aliases []AliasInfo
	err := c.breaker.Execute(func() error {
		var callErr error
		aliases, callErr = c.inner.ListAliases(ctx)
		return callErr
	})
	if err == health.ErrCircuitOpen {
		return nil, fmt.Errorf("qdrant: circuit open (upstream unhealthy)")
	}
	return aliases, err
}

func (c *CircuitBreakingAPI) UpdateAliases(ctx context.Context, actions []AliasAction) error {
	return c.breaker.Execute(func() error {
		return c.inner.UpdateAliases(ctx, actions)
	})
}

func (c *CircuitBreakingAPI) CreatePayloadIndex(ctx context.Context, collectionName, fieldName, fieldType string) error {
	return c.breaker.Execute(func() error {
		return c.inner.CreatePayloadIndex(ctx, collectionName, fieldName, fieldType)
	})
}

// CircuitBreakingEmbedder wraps a qdrant.Embedder with a circuit breaker. When
// the breaker is open, EmbedTexts returns a circuit-open error without
// contacting Ollama. The semantic-dedup path treats embedder errors as "skip
// semantic dedup", so an open breaker lets ingestion continue with
// content-hash dedup only (production-readiness checklist: "if Ollama is
// down, either skip semantic dedup or queue items for later embedding").
type CircuitBreakingEmbedder struct {
	inner   Embedder
	breaker *health.CircuitBreaker
}

// NewCircuitBreakingEmbedder wraps e with a circuit breaker.
func NewCircuitBreakingEmbedder(e Embedder, cfg health.BreakerConfig) *CircuitBreakingEmbedder {
	return &CircuitBreakingEmbedder{inner: e, breaker: health.NewCircuitBreaker(cfg)}
}

// Breaker returns the underlying breaker for observability.
func (c *CircuitBreakingEmbedder) Breaker() *health.CircuitBreaker { return c.breaker }

// Model returns the wrapped embedder's model metadata. This does not contact
// Ollama and is always available.
func (c *CircuitBreakingEmbedder) Model() EmbeddingModel { return c.inner.Model() }

// EmbedTexts calls the wrapped embedder through the circuit breaker.
func (c *CircuitBreakingEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	var vectors [][]float32
	err := c.breaker.Execute(func() error {
		var callErr error
		vectors, callErr = c.inner.EmbedTexts(ctx, texts)
		return callErr
	})
	if err == health.ErrCircuitOpen {
		return nil, fmt.Errorf("ollama: circuit open (upstream unhealthy)")
	}
	return vectors, err
}
