package cycle

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestWriteWebCaptureGraphObjectsProjectsSourceItems(t *testing.T) {
	ctx := context.Background()
	store := objectgraph.NewMemoryStore()
	graph := objectgraph.NewService(objectgraph.Config{Memory: store, SQLite: store})
	defer graph.Close()

	fetchedAt := time.Date(2026, 6, 26, 10, 15, 0, 0, time.UTC)
	now := fetchedAt.Add(time.Minute)
	item := sources.Item{
		ID:           "srcitem_policy_1",
		SourceID:     "rss:policy",
		SourceType:   sources.SourceTypeRSS,
		FetchID:      "fetch_policy_1",
		OriginalID:   "policy-1",
		Title:        "Policy story",
		Body:         "A sourcecycled body that Universal Wire can project from the object graph.",
		URL:          "https://example.com/policy?utm=1#section",
		CanonicalURL: "https://example.com/policy",
		Published:    fetchedAt.Add(-time.Hour),
		FetchedAt:    fetchedAt,
		ContentHash:  sources.ContentHash("Policy story", "body"),
		BodyKind:     sources.BodyKindReaderSnapshot,
	}

	result, err := WriteWebCaptureGraphObjects(ctx, graph, []sources.Item{
		item,
		{ID: "srcitem_empty_body", SourceID: "rss:policy", URL: "https://example.com/empty"},
	}, WebCaptureGraphProjectionConfig{
		OwnerID:    "universal-wire-platform",
		ComputerID: "computer:wire",
		Now:        now,
	})
	if err != nil {
		t.Fatalf("WriteWebCaptureGraphObjects() error = %v", err)
	}
	if len(result.Captures) != 1 || len(result.SourceEntities) != 1 || result.EdgeCount != 1 || result.Skipped != 1 {
		t.Fatalf("projection result = %+v, want one capture/entity/edge and one skipped", result)
	}

	capture := result.Captures[0]
	if capture.ObjectKind != objectgraph.WebCaptureObjectKind || capture.OwnerID != "universal-wire-platform" || string(capture.Body) != item.Body {
		t.Fatalf("capture = %+v body=%q", capture, capture.Body)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(capture)
	if err != nil {
		t.Fatalf("WebCaptureMetadataFromObject() error = %v", err)
	}
	if meta.CanonicalURL != item.CanonicalURL || meta.URL != "https://example.com/policy?utm=1" || meta.Title != item.Title {
		t.Fatalf("metadata = %+v", meta)
	}
	if !strings.Contains(meta.ContentBlobID, item.ID) || !strings.Contains(meta.ExtractedTextBlobID, item.ID) {
		t.Fatalf("blob refs do not carry source item id: %+v", meta)
	}

	var sourceMeta struct {
		SchemaVersion string `json:"schema_version"`
		SourceKind    string `json:"source_kind"`
		Target        struct {
			TargetKind string `json:"target_kind"`
			ItemID     string `json:"item_id"`
			FetchID    string `json:"fetch_id"`
		} `json:"target"`
		Evidence struct {
			DefaultOpenSurface string `json:"default_open_surface"`
			ExplicitLive       string `json:"explicit_live_surface"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(result.SourceEntities[0].Metadata, &sourceMeta); err != nil {
		t.Fatalf("decode source entity metadata: %v", err)
	}
	if sourceMeta.SourceKind != sourcecontract.SourceKindSourceServiceItem ||
		sourceMeta.Target.TargetKind != sourcecontract.SourceKindSourceServiceItem ||
		sourceMeta.Target.ItemID != item.ID ||
		sourceMeta.Target.FetchID != item.FetchID ||
		sourceMeta.Evidence.DefaultOpenSurface != sourcecontract.OpenSurfaceSource ||
		sourceMeta.Evidence.ExplicitLive != sourcecontract.OpenSurfaceWebLens {
		t.Fatalf("source entity metadata = %+v", sourceMeta)
	}

	edges, err := graph.ListEdges(ctx, objectgraph.EdgeFilter{FromID: capture.CanonicalID, Kind: "captured_from"})
	if err != nil {
		t.Fatalf("ListEdges(captured_from) error = %v", err)
	}
	if len(edges) != 1 || edges[0].ToID != result.SourceEntities[0].CanonicalID {
		t.Fatalf("captured_from edges = %+v", edges)
	}
}
