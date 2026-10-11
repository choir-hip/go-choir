package sourcegraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// BodyStore keeps capture bodies outside Dolt. PutBody stores the bytes
// durably and returns the ref the capture metadata names.
type BodyStore interface {
	PutBody(body []byte) (string, error)
}

// CaptureWriteConfig names the computer a poll writes into.
type CaptureWriteConfig struct {
	OwnerID    string
	ComputerID string
	Now        time.Time
}

// CaptureWriteResult counts what one poll wrote.
type CaptureWriteResult struct {
	New        int      `json:"new"`
	Repeated   int      `json:"repeated"`
	Skipped    int      `json:"skipped"`
	CaptureIDs []string `json:"capture_ids,omitempty"`
	// Captured lists each new capture's id, title, URL and publish time,
	// never its body.
	Captured []CapturedItem `json:"captured,omitempty"`
}

type CapturedItem struct {
	CaptureID   string `json:"capture_id"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at,omitempty"`
}

// WriteCaptures writes each polled item as one choir.web_capture whose body
// lives in the body store, never in the graph row (Autopaper W1; on the host
// the graph is Store B). The capture id is stable per item, so an item
// already captured is a repeat and writes nothing. The body is stored before
// the object is written, so an object never names bytes that are not there.
func WriteCaptures(ctx context.Context, graph *objectgraph.Service, bodies BodyStore, items []sources.Item, cfg CaptureWriteConfig) (CaptureWriteResult, error) {
	var result CaptureWriteResult
	ownerID := strings.TrimSpace(cfg.OwnerID)
	if graph == nil || bodies == nil || ownerID == "" {
		return result, fmt.Errorf("write captures: graph, body store and owner are required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, item := range items {
		req, _, ok := webCaptureGraphRequestsFromItem(item, ownerID, strings.TrimSpace(cfg.ComputerID), now)
		if !ok {
			result.Skipped++
			continue
		}
		key := "autopaper:capture:" + strings.TrimSpace(item.ID)
		id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, ownerID, objectgraph.StableSuffixFromKey(key))
		if err != nil {
			return result, fmt.Errorf("write captures: %w", err)
		}
		if _, err := graph.GetObject(ctx, id); err == nil {
			result.Repeated++
			continue
		} else if !errors.Is(err, objectgraph.ErrNotFound) {
			return result, fmt.Errorf("write captures: look up %s: %w", item.ID, err)
		}
		ref, err := bodies.PutBody(req.ExtractedText)
		if err != nil {
			return result, fmt.Errorf("write captures: body for %s: %w", item.ID, err)
		}
		item = sources.NormalizeItemBodyClassification(item)
		req.IdentityKey = key
		req.ContentBlobID = ref
		req.ExtractedTextBlobID = ref
		req.ExtractedText = nil
		req.SourceID = item.SourceID
		req.SourceType = string(item.SourceType)
		req.ItemID = item.ID
		req.PublishedAt = item.Published
		req.Language = item.Language
		req.Region = item.Region
		req.BodyKind = item.BodyKind
		obj, created, err := graph.CreateWebCaptureIfAbsent(ctx, req)
		if err != nil {
			return result, fmt.Errorf("write captures: capture %s: %w", item.ID, err)
		}
		if !created {
			// Another writer created it between the lookup and this insert;
			// its capture stands (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
			result.Repeated++
			continue
		}
		result.New++
		result.CaptureIDs = append(result.CaptureIDs, obj.CanonicalID)
		captured := CapturedItem{CaptureID: obj.CanonicalID, Title: req.Title, URL: req.CanonicalURL}
		if !item.Published.IsZero() {
			captured.PublishedAt = item.Published.UTC().Format(time.RFC3339)
		}
		result.Captured = append(result.Captured, captured)
	}
	return result, nil
}
