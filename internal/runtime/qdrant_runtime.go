package runtime

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/qdrant"
)

// ProductionQdrantCollection is the Qdrant collection name for production wire
// capture indexing and semantic routing.
const ProductionQdrantCollection = "wire_captures"

// QdrantPipeline returns a Qdrant indexing pipeline configured with the
// OllamaEmbedder pointing at the configured Ollama instance and the Qdrant
// client pointing at the configured Qdrant instance (node-b by default).
// The pipeline is initialized lazily so tests and lightweight runtimes that
// never touch Qdrant do not open a connection.
func (rt *Runtime) QdrantPipeline() *qdrant.Pipeline {
	if rt == nil {
		return nil
	}
	rt.qdrantPipelineMu.Lock()
	defer rt.qdrantPipelineMu.Unlock()
	if rt.qdrantPipeline != nil {
		return rt.qdrantPipeline
	}
	if rt.qdrantPipelineInitErr != nil {
		return nil
	}
	client := qdrant.NewClient(rt.cfg.QdrantURL)
	embedder := qdrant.NewOllamaEmbedder(rt.cfg.OllamaURL, rt.cfg.OllamaEmbeddingModel)
	pipeline := qdrant.NewPipeline(client, embedder)
	rt.qdrantPipeline = pipeline
	return pipeline
}

func (rt *Runtime) closeQdrantPipeline() {
	if rt == nil {
		return
	}
	rt.qdrantPipelineMu.Lock()
	defer rt.qdrantPipelineMu.Unlock()
	rt.qdrantPipeline = nil
}

// EnsureProductionQdrantCollection creates the production Qdrant collection if
// it does not already exist, with 1024-dim Cosine vectors and payload indexes
// on vm_owner and content_hash. This is idempotent and safe to call at startup.
func (rt *Runtime) EnsureProductionQdrantCollection(ctx context.Context) error {
	pipeline := rt.QdrantPipeline()
	if pipeline == nil {
		return nil
	}
	return qdrant.EnsureProductionCollection(ctx, pipeline.Client(), ProductionQdrantCollection)
}
