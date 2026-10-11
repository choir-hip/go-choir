package sourcegraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capturesegment"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

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
// lives in the day's capture segment, not in the graph (Autopaper W1). The
// capture id is stable per item, so an item already captured is a repeat and
// writes nothing. The body is appended and synced before the object is
// written, so an object never names bytes that are not there.
func WriteCaptures(ctx context.Context, graph *objectgraph.Service, segments *capturesegment.Appender, items []sources.Item, cfg CaptureWriteConfig) (CaptureWriteResult, error) {
	var result CaptureWriteResult
	ownerID := strings.TrimSpace(cfg.OwnerID)
	if graph == nil || segments == nil || ownerID == "" {
		return result, fmt.Errorf("write captures: graph, segments and owner are required")
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
		ref, err := segments.Append(req.FetchedAt, req.ExtractedText)
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
		obj, err := graph.CreateWebCapture(ctx, req)
		if err != nil {
			return result, fmt.Errorf("write captures: capture %s: %w", item.ID, err)
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
