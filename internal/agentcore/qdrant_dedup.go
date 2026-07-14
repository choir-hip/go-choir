package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/qdrant"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// qdrantDedupResult records the outcome of a semantic dedup pass over a batch
// of ingested items. Kept items proceed to objectgraph projection and
// processor dispatch; dropped items are logged with their nearest-neighbor
// score for threshold calibration.
type qdrantDedupResult struct {
	Kept       []sources.Item
	Dropped    []sources.Item
	Threshold  float32
	Skipped    bool
	SkipReason string
}

// dedupSourceItemsSemantically runs a Qdrant semantic dedup pass over items
// before they are projected into the object graph and dispatched to the
// processor. For each item it embeds the item text, searches the production
// Qdrant collection for a near-duplicate, and drops items whose top match
// score meets or exceeds the configured threshold. Items that pass are
// upserted into Qdrant so future captures can be compared against them.
//
// The pass is best-effort and non-blocking: if the Qdrant pipeline is
// unavailable, the production collection cannot be ensured, embedding fails,
// or the threshold is non-positive, all items are returned as kept with a
// skip reason. This keeps the ingestion path resilient when Qdrant or Ollama
// are down or not yet deployed.
func (rt *Runtime) dedupSourceItemsSemantically(ctx context.Context, items []sources.Item, ownerID string) qdrantDedupResult {
	threshold := rt.dedupThreshold()
	if len(items) == 0 {
		return qdrantDedupResult{Kept: items, Threshold: threshold}
	}
	if threshold <= 0 {
		return qdrantDedupResult{Kept: items, Threshold: threshold, Skipped: true, SkipReason: "threshold <= 0 (semantic dedup disabled)"}
	}
	pipeline := rt.QdrantPipeline()
	if pipeline == nil {
		return qdrantDedupResult{Kept: items, Threshold: threshold, Skipped: true, SkipReason: "qdrant pipeline unavailable"}
	}
	if err := rt.EnsureProductionQdrantCollection(ctx); err != nil {
		log.Printf("qdrant semantic dedup: ensure production collection failed: %v (passing items through)", err)
		return qdrantDedupResult{Kept: items, Threshold: threshold, Skipped: true, SkipReason: fmt.Sprintf("ensure collection: %v", err)}
	}

	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = dedupItemText(item)
	}
	vectors, err := pipeline.Embedder().EmbedTexts(ctx, texts)
	if err != nil {
		log.Printf("qdrant semantic dedup: embed failed: %v (passing items through)", err)
		return qdrantDedupResult{Kept: items, Threshold: threshold, Skipped: true, SkipReason: fmt.Sprintf("embed: %v", err)}
	}
	if len(vectors) != len(items) {
		log.Printf("qdrant semantic dedup: embedder returned %d vectors for %d items (passing items through)", len(vectors), len(items))
		return qdrantDedupResult{Kept: items, Threshold: threshold, Skipped: true, SkipReason: fmt.Sprintf("embedder vector count mismatch: %d != %d", len(vectors), len(items))}
	}

	kept := make([]sources.Item, 0, len(items))
	var dropped []sources.Item
	keepVectors := make([]sources.Item, 0, len(items))
	keepVecs := make([][]float32, 0, len(items))

	for i, item := range items {
		results, err := pipeline.Client().Search(ctx, ProductionQdrantCollection, vectors[i], 1)
		if err != nil {
			log.Printf("qdrant semantic dedup: search failed for item %s: %v (keeping item)", item.ID, err)
			kept = append(kept, item)
			keepVectors = append(keepVectors, item)
			keepVecs = append(keepVecs, vectors[i])
			continue
		}
		if len(results) > 0 && results[0].Score >= threshold {
			dropped = append(dropped, item)
			log.Printf("qdrant semantic dedup: dropped item %s (score=%.4f threshold=%.4f nearest=%q)",
				item.ID, results[0].Score, threshold, truncateDedupText(results[0].Payload.Text, 80))
			continue
		}
		kept = append(kept, item)
		keepVectors = append(keepVectors, item)
		keepVecs = append(keepVecs, vectors[i])
	}

	if len(keepVectors) > 0 {
		if err := upsertDedupPoints(ctx, pipeline, keepVectors, keepVecs, ownerID); err != nil {
			log.Printf("qdrant semantic dedup: upsert failed: %v (items still proceed to objectgraph)", err)
		}
	}

	log.Printf("qdrant semantic dedup: threshold=%.4f kept=%d dropped=%d skipped=%v",
		threshold, len(kept), len(dropped), false)
	return qdrantDedupResult{Kept: kept, Dropped: dropped, Threshold: threshold}
}

func (rt *Runtime) dedupThreshold() float32 {
	if rt == nil {
		return 0
	}
	return rt.cfg.QdrantDedupThreshold
}

// dedupItemText builds the text used for embedding and dedup comparison. It
// favors the title plus a trimmed body so the embedding captures the story
// angle, not just the headline.
func dedupItemText(item sources.Item) string {
	title := strings.TrimSpace(item.Title)
	body := strings.TrimSpace(item.Body)
	if body == "" {
		return title
	}
	if len(body) > 2000 {
		body = body[:2000]
	}
	if title == "" {
		return body
	}
	return title + "\n\n" + body
}

func upsertDedupPoints(ctx context.Context, pipeline *qdrant.Pipeline, items []sources.Item, vectors [][]float32, ownerID string) error {
	if pipeline == nil || len(items) == 0 {
		return nil
	}
	model := pipeline.Embedder().Model()
	points := make([]qdrant.Point, len(items))
	for i, item := range items {
		canonicalID := dedupItemCanonicalID(item)
		points[i] = qdrant.Point{
			ID:     qdrant.PointIDForCanonicalID(canonicalID),
			Vector: vectors[i],
			Payload: qdrant.PointPayload{
				CanonicalID:      canonicalID,
				ObjectKind:       "choir.web_capture",
				ContentHash:      item.ContentHash,
				OwnerID:          ownerID,
				Text:             dedupItemText(item),
				EmbeddingModel:   model.Name,
				EmbeddingVersion: model.Version,
			},
		}
	}
	if err := pipeline.Client().UpsertPoints(ctx, ProductionQdrantCollection, points); err != nil {
		return fmt.Errorf("upsert dedup points: %w", err)
	}
	return nil
}

func dedupItemCanonicalID(item sources.Item) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return id
	}
	if id := strings.TrimSpace(item.CanonicalURL); id != "" {
		return id
	}
	return sources.ContentHash(item.SourceID, item.OriginalID, item.URL)
}

func truncateDedupText(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

// dedupSourceItemsWithTimeout is a convenience for callers that want a bounded
// dedup pass. It wraps dedupSourceItemsSemantically with a deadline so a slow
// Qdrant or Ollama cannot stall the ingestion handoff indefinitely.
func (rt *Runtime) dedupSourceItemsWithTimeout(ctx context.Context, items []sources.Item, ownerID string, timeout time.Duration) qdrantDedupResult {
	if timeout <= 0 {
		return rt.dedupSourceItemsSemantically(ctx, items, ownerID)
	}
	dedupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return rt.dedupSourceItemsSemantically(dedupCtx, items, ownerID)
}
