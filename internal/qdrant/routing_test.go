package qdrant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRoutingSemanticDedupAndAgentRouting validates that Qdrant similarity
// search can route source captures to durable agents and perform semantic dedup.
//
// Prerequisites:
//   - Qdrant running (default http://localhost:6333 or QDRANT_URL env)
//   - Ollama running with batiai/qwen3-embedding:0.6b pulled
//   - QDRANT_TEST=1 env var set to enable this integration test
func TestRoutingSemanticDedupAndAgentRouting(t *testing.T) {
	if os.Getenv("QDRANT_TEST") == "" {
		t.Skip("skipping routing test; set QDRANT_TEST=1 to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	qdrantURL := os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		qdrantURL = "http://localhost:6333"
	}
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	modelName := "batiai/qwen3-embedding:0.6b"

	client := NewClient(qdrantURL)
	if err := client.Health(ctx); err != nil {
		t.Fatalf("Qdrant unavailable at %s: %v", qdrantURL, err)
	}

	embedder := NewOllamaEmbedder(ollamaURL, modelName)

	// Connectivity check: embed a probe string and verify dimensions.
	probeVecs, err := embedder.EmbedTexts(ctx, []string{"connectivity check"})
	if err != nil {
		t.Fatalf("Ollama embedder unavailable at %s: %v", ollamaURL, err)
	}
	if len(probeVecs) != 1 || len(probeVecs[0]) != 1024 {
		t.Fatalf("expected 1024-dim embedding, got %d", len(probeVecs[0]))
	}

	collectionName := "wire_captures_test"

	// Clean up any leftover collection from a prior run, then create fresh.
	_ = client.DeleteCollection(ctx, collectionName)

	model := embedder.Model()
	cfg, err := CollectionConfigForModel(model)
	if err != nil {
		t.Fatalf("CollectionConfigForModel: %v", err)
	}
	if cfg.VectorSize != 1024 {
		t.Fatalf("expected 1024 dimensions, got %d", cfg.VectorSize)
	}
	if err := client.CreateCollection(ctx, collectionName, cfg); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteCollection(context.Background(), collectionName)
	})

	info, err := client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	t.Logf("collection %s status=%s points=%d", collectionName, info.Status, info.PointsCount)

	// --- Sample headlines: 25 captures across 5 topics, 3 VMs ---
	type capture struct {
		text    string
		topic   string
		vmOwner string
	}

	captures := []capture{
		// Politics — vm-1
		{text: "Senate passes infrastructure bill", topic: "politics", vmOwner: "vm-1"},
		{text: "Congress debates budget deal", topic: "politics", vmOwner: "vm-1"},
		{text: "House approves defense spending bill", topic: "politics", vmOwner: "vm-1"},
		{text: "Senate confirms new federal judge", topic: "politics", vmOwner: "vm-1"},
		{text: "Infrastructure legislation clears Senate in bipartisan vote", topic: "politics", vmOwner: "vm-1"},

		// Tech — vm-1
		{text: "Apple announces new M5 chip", topic: "tech", vmOwner: "vm-1"},
		{text: "Google launches AI search feature", topic: "tech", vmOwner: "vm-1"},
		{text: "Microsoft unveils Windows 12 features", topic: "tech", vmOwner: "vm-1"},
		{text: "Apple's M5 processor promises 40% performance gain", topic: "tech", vmOwner: "vm-1"},
		{text: "Tesla unveils new autonomous driving system", topic: "tech", vmOwner: "vm-1"},

		// Health — vm-2
		{text: "WHO declares new pandemic protocol", topic: "health", vmOwner: "vm-2"},
		{text: "FDA approves new cancer drug", topic: "health", vmOwner: "vm-2"},
		{text: "CDC issues flu season guidelines", topic: "health", vmOwner: "vm-2"},
		{text: "New pandemic preparedness framework announced by WHO", topic: "health", vmOwner: "vm-2"},
		{text: "Breakthrough Alzheimer's treatment shows promise", topic: "health", vmOwner: "vm-2"},

		// Sports — vm-2
		{text: "Lakers win NBA finals", topic: "sports", vmOwner: "vm-2"},
		{text: "World Cup 2026 qualification rounds begin", topic: "sports", vmOwner: "vm-2"},
		{text: "Olympic committee announces 2032 host city", topic: "sports", vmOwner: "vm-2"},
		{text: "NBA championship: Lakers claim title in game seven", topic: "sports", vmOwner: "vm-2"},
		{text: "Manchester United signs new striker", topic: "sports", vmOwner: "vm-2"},

		// Climate — vm-3
		{text: "Global climate summit reaches carbon agreement", topic: "climate", vmOwner: "vm-3"},
		{text: "Arctic ice melt accelerates beyond predictions", topic: "climate", vmOwner: "vm-3"},
		{text: "Renewable energy investment hits record high", topic: "climate", vmOwner: "vm-3"},
		{text: "Carbon emissions agreement signed at climate summit", topic: "climate", vmOwner: "vm-3"},
		{text: "Electric vehicle sales surge in European markets", topic: "climate", vmOwner: "vm-3"},
	}

	if len(captures) < 20 || len(captures) > 30 {
		t.Fatalf("expected 20-30 captures, got %d", len(captures))
	}

	// Embed all headlines in a single batch.
	texts := make([]string, len(captures))
	for i, c := range captures {
		texts[i] = c.text
	}
	t.Logf("embedding %d headlines via Ollama %s ...", len(texts), modelName)
	vecs, err := embedder.EmbedTexts(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}
	if len(vecs) != len(captures) {
		t.Fatalf("expected %d embeddings, got %d", len(captures), len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 1024 {
			t.Fatalf("embedding %d: expected 1024 dims, got %d", i, len(v))
		}
	}

	// Build Qdrant points with vm_owner in payload and metadata.
	points := make([]Point, len(captures))
	for i, c := range captures {
		canonicalID := fmt.Sprintf("cap:%s:%d", c.topic, i)
		meta, _ := json.Marshal(map[string]string{
			"vm_owner": c.vmOwner,
			"topic":    c.topic,
		})
		points[i] = Point{
			ID:     PointIDForCanonicalID(canonicalID),
			Vector: vecs[i],
			Payload: PointPayload{
				CanonicalID:      canonicalID,
				ObjectKind:       "wire_capture",
				ContentHash:      sha256Hex(c.text),
				OwnerID:          c.vmOwner,
				Text:             c.text,
				EmbeddingModel:   model.Name,
				EmbeddingVersion: model.Version,
				Metadata:         meta,
			},
		}
	}

	if err := client.UpsertPoints(ctx, collectionName, points); err != nil {
		t.Fatalf("UpsertPoints: %v", err)
	}

	info, err = client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		t.Fatalf("GetCollectionInfo after upsert: %v", err)
	}
	if info.PointsCount != len(captures) {
		t.Fatalf("expected %d points, got %d", len(captures), info.PointsCount)
	}
	t.Logf("verified %d points upserted to %s", info.PointsCount, collectionName)

	// Helper: embed a query string and search the collection.
	searchQuery := func(t *testing.T, query string, limit int) []ScoredPoint {
		t.Helper()
		qvecs, err := embedder.EmbedTexts(ctx, []string{query})
		if err != nil {
			t.Fatalf("embed query %q: %v", query, err)
		}
		results, err := client.Search(ctx, collectionName, qvecs[0], limit)
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		return results
	}

	logResults := func(t *testing.T, query string, results []ScoredPoint) {
		t.Helper()
		t.Logf("query: %q", query)
		for _, r := range results {
			m := unmarshalMeta(r.Payload.Metadata)
			t.Logf("  score=%.4f topic=%s vm=%s text=%q",
				r.Score, m["topic"], r.Payload.OwnerID, r.Payload.Text)
		}
	}

	// --- SEARCH TEST A: New headline about same topic → high score, same topic ---
	t.Run("SameTopicNewHeadline", func(t *testing.T) {
		results := searchQuery(t, "Senate passes $1.2T infrastructure bill", 5)
		if len(results) == 0 {
			t.Fatal("no results returned")
		}
		logResults(t, "Senate passes $1.2T infrastructure bill", results)

		top := results[0]
		meta := unmarshalMeta(top.Payload.Metadata)
		if meta["topic"] != "politics" {
			t.Errorf("expected nearest topic=politics, got %q", meta["topic"])
		}
		if top.Score < 0.50 {
			t.Errorf("expected high score (>=0.50) for same-topic query, got %.4f", top.Score)
		}
	})

	// --- SEARCH TEST B: Headline about different/unindexed topic → low score ---
	t.Run("DifferentTopicLowScore", func(t *testing.T) {
		results := searchQuery(t, "Local bakery wins award for best croissant", 5)
		if len(results) == 0 {
			t.Fatal("no results returned")
		}
		logResults(t, "Local bakery wins award for best croissant", results)

		top := results[0]
		if top.Score > 0.80 {
			t.Logf("WARNING: different-topic top score unexpectedly high: %.4f", top.Score)
		}
		t.Logf("different-topic top score = %.4f", top.Score)
	})

	// --- SEARCH TEST C: Paraphrased headline → high score (semantic dedup) ---
	t.Run("SemanticDedupParaphrase", func(t *testing.T) {
		results := searchQuery(t, "Bipartisan infrastructure package clears Senate floor", 5)
		if len(results) == 0 {
			t.Fatal("no results returned")
		}
		logResults(t, "Bipartisan infrastructure package clears Senate floor", results)

		top := results[0]
		lower := strings.ToLower(top.Payload.Text)
		if !strings.Contains(lower, "infrastructure") && !strings.Contains(lower, "senate") {
			t.Errorf("expected infrastructure/senate-related top result, got %q", top.Payload.Text)
		}
		if top.Score < 0.50 {
			t.Errorf("expected high score for semantic dedup (>=0.50), got %.4f", top.Score)
		}
		t.Logf("semantic dedup validated: paraphrase top score = %.4f for %q", top.Score, top.Payload.Text)
	})

	// --- SEARCH TEST D: Related but different story in same topic → boundary score ---
	t.Run("RelatedButDifferentStory", func(t *testing.T) {
		results := searchQuery(t, "Congress debates defense authorization act", 5)
		if len(results) == 0 {
			t.Fatal("no results returned")
		}
		logResults(t, "Congress debates defense authorization act", results)

		top := results[0]
		meta := unmarshalMeta(top.Payload.Metadata)
		t.Logf("BOUNDARY: related-but-different-story top score = %.4f (topic=%s, text=%q)",
			top.Score, meta["topic"], top.Payload.Text)
	})

	// --- SCORE DISTRIBUTION ANALYSIS ---
	t.Run("ScoreDistribution", func(t *testing.T) {
		// Paraphrase pairs: (original index, paraphrase index)
		paraphrasePairs := [][2]int{
			{0, 4},   // infrastructure bill
			{5, 8},   // Apple M5
			{10, 13}, // WHO pandemic
			{15, 18}, // Lakers NBA
			{20, 23}, // climate summit
		}

		// Same story, different source: search with original, find paraphrase score.
		var sameStoryScores []float32
		for _, pair := range paraphrasePairs {
			qvecs, err := embedder.EmbedTexts(ctx, []string{captures[pair[0]].text})
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			results, err := client.Search(ctx, collectionName, qvecs[0], len(captures))
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			for _, r := range results {
				if r.Payload.Text == captures[pair[1]].text {
					sameStoryScores = append(sameStoryScores, r.Score)
					break
				}
			}
		}
		reportScoreRange(t, "Same story, different source", sameStoryScores)

		// Build a set of paraphrase indices to exclude from same-topic-different-story.
		paraphraseSet := map[int]bool{}
		for _, pair := range paraphrasePairs {
			paraphraseSet[pair[1]] = true
		}

		// Group captures by topic.
		topicMembers := map[string][]int{}
		for i, c := range captures {
			topicMembers[c.topic] = append(topicMembers[c.topic], i)
		}

		// Same topic, different story: search with one headline, find other
		// same-topic headlines that are NOT paraphrases.
		var sameTopicDiffScores []float32
		for topic, members := range topicMembers {
			if len(members) < 3 {
				continue
			}
			queryIdx := -1
			for _, m := range members {
				if !paraphraseSet[m] {
					queryIdx = m
					break
				}
			}
			if queryIdx == -1 {
				continue
			}
			qvecs, err := embedder.EmbedTexts(ctx, []string{captures[queryIdx].text})
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			results, err := client.Search(ctx, collectionName, qvecs[0], len(captures))
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			for _, r := range results {
				if r.Payload.Text == captures[queryIdx].text {
					continue
				}
				rMeta := unmarshalMeta(r.Payload.Metadata)
				if rMeta["topic"] != topic {
					continue
				}
				for i, c := range captures {
					if c.text == r.Payload.Text && !paraphraseSet[i] && i != queryIdx {
						sameTopicDiffScores = append(sameTopicDiffScores, r.Score)
						break
					}
				}
			}
		}
		reportScoreRange(t, "Same topic, different story", sameTopicDiffScores)

		// Different topic: search with one headline, find cross-topic scores.
		var diffTopicScores []float32
		for _, queryIdx := range []int{0, 5, 10, 15, 20} {
			qvecs, err := embedder.EmbedTexts(ctx, []string{captures[queryIdx].text})
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			results, err := client.Search(ctx, collectionName, qvecs[0], len(captures))
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			queryTopic := captures[queryIdx].topic
			for _, r := range results {
				if r.Payload.Text == captures[queryIdx].text {
					continue
				}
				rMeta := unmarshalMeta(r.Payload.Metadata)
				if rMeta["topic"] != queryTopic {
					diffTopicScores = append(diffTopicScores, r.Score)
				}
			}
		}
		reportScoreRange(t, "Different topic", diffTopicScores)

		// Propose routing threshold based on observed score distributions.
		proposeRoutingThreshold(t, sameStoryScores, sameTopicDiffScores, diffTopicScores)
	})
}

// --- helpers ---

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

func unmarshalMeta(raw json.RawMessage) map[string]string {
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	if m == nil {
		m = map[string]string{}
	}
	return m
}

func reportScoreRange(t *testing.T, label string, scores []float32) {
	t.Helper()
	if len(scores) == 0 {
		t.Logf("%s: no scores collected", label)
		return
	}
	min, max := scores[0], scores[0]
	var sum float32
	for _, s := range scores {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
		sum += s
	}
	avg := sum / float32(len(scores))
	t.Logf("%s: n=%d min=%.4f max=%.4f avg=%.4f", label, len(scores), min, max, avg)
}

func proposeRoutingThreshold(t *testing.T, sameStory, sameTopicDiff, diffTopic []float32) {
	t.Helper()

	sameStoryMin := minScore(sameStory)
	sameStoryMax := maxScore(sameStory)
	sameTopicMax := maxScore(sameTopicDiff)
	sameTopicMin := minScore(sameTopicDiff)
	diffTopicMax := maxScore(diffTopic)
	diffTopicMin := minScore(diffTopic)

	t.Log("--- ROUTING THRESHOLD PROPOSAL ---")
	t.Logf("Same-story score range:              [%.4f, %.4f]", sameStoryMin, sameStoryMax)
	t.Logf("Same-topic-different-story range:    [%.4f, %.4f]", sameTopicMin, sameTopicMax)
	t.Logf("Different-topic score range:         [%.4f, %.4f]", diffTopicMin, diffTopicMax)

	var threshold float32
	switch {
	case sameStoryMin > sameTopicMax:
		threshold = (sameStoryMin + sameTopicMax) / 2
		t.Logf("Clean separation: threshold = %.4f (midpoint of same-story-min and same-topic-max)", threshold)
	case sameStoryMin > diffTopicMax:
		threshold = (sameStoryMin + diffTopicMax) / 2
		t.Logf("Partial overlap with same-topic; threshold = %.4f (midpoint of same-story-min and diff-topic-max)", threshold)
	default:
		threshold = sameStoryMin * 0.85
		t.Logf("WARNING: score ranges overlap; conservative threshold = %.4f (85%% of same-story-min)", threshold)
	}

	t.Log("ROUTING POLICY:")
	t.Logf("  score >= %.4f → semantic duplicate → route to same VM as nearest neighbor", threshold)
	t.Logf("  score <  %.4f → new story → route to least-loaded VM", threshold)
	t.Logf("  different-topic max = %.4f (below threshold confirms topic separation)", diffTopicMax)
}

func minScore(scores []float32) float32 {
	if len(scores) == 0 {
		return 0
	}
	m := scores[0]
	for _, s := range scores {
		if s < m {
			m = s
		}
	}
	return m
}

func maxScore(scores []float32) float32 {
	if len(scores) == 0 {
		return 0
	}
	m := scores[0]
	for _, s := range scores {
		if s > m {
			m = s
		}
	}
	return m
}
