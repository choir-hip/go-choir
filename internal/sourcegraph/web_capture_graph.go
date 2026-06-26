package sourcegraph

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

const SourcecycledWebCaptureProjectionSchema = "sourcecycled.web_capture_projection.v1"

type WebCaptureGraphProjectionConfig struct {
	OwnerID    string
	ComputerID string
	Now        time.Time
}

type WebCaptureGraphProjectionResult struct {
	Captures       []objectgraph.Object
	SourceEntities []objectgraph.Object
	EdgeCount      int
	Skipped        int
}

// WriteWebCaptureGraphObjects projects persisted sourcecycled source items into
// graph-native web captures. It does not create Texture publications or body
// source_ref citations; those remain downstream decisions.
func WriteWebCaptureGraphObjects(ctx context.Context, graph *objectgraph.Service, items []sources.Item, cfg WebCaptureGraphProjectionConfig) (WebCaptureGraphProjectionResult, error) {
	var result WebCaptureGraphProjectionResult
	if graph == nil || len(items) == 0 {
		return result, nil
	}
	ownerID := strings.TrimSpace(cfg.OwnerID)
	if ownerID == "" {
		return result, fmt.Errorf("owner_id is required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, item := range items {
		captureReq, sourceMeta, ok := webCaptureGraphRequestsFromItem(item, ownerID, strings.TrimSpace(cfg.ComputerID), now)
		if !ok {
			result.Skipped++
			continue
		}
		sourceEntity, err := graph.CreateObject(ctx, objectgraph.CreateObjectRequest{
			Kind:       "choir.source_entity",
			OwnerID:    ownerID,
			ComputerID: cfg.ComputerID,
			Body:       []byte(strings.TrimSpace(item.Body)),
			Metadata:   sourceMeta,
			Now:        now,
		})
		if err != nil {
			return result, fmt.Errorf("create source entity for %s: %w", item.ID, err)
		}
		capture, err := graph.CreateWebCapture(ctx, captureReq)
		if err != nil {
			return result, fmt.Errorf("create web capture for %s: %w", item.ID, err)
		}
		if _, err := graph.PutEdge(ctx, capture.CanonicalID, sourceEntity.CanonicalID, "captured_from", map[string]any{
			"schema_version": SourcecycledWebCaptureProjectionSchema,
			"relation":       "sourcecycled_source_item",
			"source_item_id": strings.TrimSpace(item.ID),
			"source_id":      strings.TrimSpace(item.SourceID),
			"fetch_id":       strings.TrimSpace(item.FetchID),
		}); err != nil {
			return result, fmt.Errorf("link web capture to source item %s: %w", item.ID, err)
		}
		result.Captures = append(result.Captures, capture)
		result.SourceEntities = append(result.SourceEntities, sourceEntity)
		result.EdgeCount++
	}
	return result, nil
}

func webCaptureGraphRequestsFromItem(item sources.Item, ownerID, computerID string, now time.Time) (objectgraph.CreateWebCaptureRequest, map[string]any, bool) {
	item = sources.NormalizeItemBodyClassification(item)
	itemID := strings.TrimSpace(item.ID)
	body := strings.TrimSpace(item.Body)
	if itemID == "" || strings.TrimSpace(item.SourceID) == "" || body == "" {
		return objectgraph.CreateWebCaptureRequest{}, nil, false
	}
	itemURL := firstNonEmptyString(item.URL, item.CanonicalURL)
	canonicalURL := firstNonEmptyString(item.CanonicalURL, item.URL)
	if !isHTTPURL(itemURL) || !isHTTPURL(canonicalURL) {
		return objectgraph.CreateWebCaptureRequest{}, nil, false
	}
	fetchedAt := item.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = item.Published
	}
	if fetchedAt.IsZero() {
		fetchedAt = now
	}
	contentHash := strings.TrimSpace(item.ContentHash)
	if contentHash == "" {
		contentHash = sources.ContentHash(item.Title, item.Body, item.CanonicalURL, item.URL)
	}
	contentBlobID := "sourcecycled:item:" + itemID + ":content:" + contentHash
	extractedTextBlobID := "sourcecycled:item:" + itemID + ":extracted_text:" + contentHash
	sourceMeta := map[string]any{
		"schema_version": "choir.source_entity.v1",
		"source_kind":    sourcecontract.SourceKindSourceServiceItem,
		"target": map[string]any{
			"target_kind":   sourcecontract.SourceKindSourceServiceItem,
			"item_id":       itemID,
			"source_id":     strings.TrimSpace(item.SourceID),
			"source_type":   strings.TrimSpace(string(item.SourceType)),
			"fetch_id":      strings.TrimSpace(item.FetchID),
			"url":           itemURL,
			"canonical_url": canonicalURL,
			"language":      strings.TrimSpace(item.Language),
			"region":        strings.TrimSpace(item.Region),
		},
		"display": map[string]any{
			"title": strings.TrimSpace(item.Title),
			"url":   canonicalURL,
		},
		"evidence": map[string]any{
			"state":                 sourcecontract.EvidenceStateAvailable,
			"content_hash":          contentHash,
			"body_kind":             item.BodyKind,
			"reader_snapshot":       item.ReaderSnapshot,
			"default_open_surface":  sourcecontract.OpenSurfaceSource,
			"explicit_live_surface": sourcecontract.OpenSurfaceWebLens,
		},
		"provenance": map[string]any{
			"created_by":        "sourcecycled",
			"source_system":     "source_service",
			"projection_schema": SourcecycledWebCaptureProjectionSchema,
			"fetched_at":        fetchedAt.UTC().Format(time.RFC3339Nano),
		},
	}
	return objectgraph.CreateWebCaptureRequest{
		OwnerID:             ownerID,
		ComputerID:          computerID,
		URL:                 itemURL,
		CanonicalURL:        canonicalURL,
		Title:               strings.TrimSpace(item.Title),
		FetchedAt:           fetchedAt,
		ContentBlobID:       contentBlobID,
		ExtractedTextBlobID: extractedTextBlobID,
		ExtractedText:       []byte(body),
		Now:                 now,
	}, sourceMeta, true
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
