package sourcegraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
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

// CaptureItem is one item for the capture writer: a polled feed item, or a
// page read by choir.Read carrying the bytes it fetched and its blocks.
// Raw is the bytes as fetched (HTML, a PDF); a feed item has none, and its
// body text is kept as its raw bytes. Blocks and Extractor are the page's
// block document as the reading computer extracted it; a feed item has
// none, and the writer splits its body with sourcefetch.TextBlocks. The
// writer recomputes every block id from the block's text.
type CaptureItem struct {
	sources.Item
	Raw       []byte              `json:"raw,omitempty"`
	MediaType string              `json:"media_type,omitempty"`
	Extractor string              `json:"extractor,omitempty"`
	Blocks    []sourcefetch.Block `json:"blocks,omitempty"`
}

// ReadSourceID is the source id of captures choir.Read makes.
const ReadSourceID = "web:read"

// ReadCaptureItem is the capture item for one page read by choir.Read.
func ReadCaptureItem(pageURL string, raw []byte, mediaType string, doc sourcefetch.Document, fetchedAt time.Time) CaptureItem {
	pageURL = strings.TrimSpace(pageURL)
	return CaptureItem{
		Item: sources.Item{ID: "read:" + pageURL, SourceID: ReadSourceID, Title: doc.Title, URL: pageURL,
			FetchedAt: fetchedAt, BodyKind: sources.BodyKindSourceBody},
		Raw: raw, MediaType: strings.TrimSpace(mediaType), Extractor: doc.Extractor, Blocks: doc.Blocks,
	}
}

// CaptureBlocksSchema names the block JSON blob a capture keeps.
const CaptureBlocksSchema = "choir.capture_blocks.v1"

// CaptureBlocks is a capture's block document as stored beside its raw
// bytes and projection.
type CaptureBlocks struct {
	Schema    string              `json:"schema"`
	CaptureID string              `json:"capture_id"`
	URL       string              `json:"url"`
	Title     string              `json:"title,omitempty"`
	MediaType string              `json:"media_type,omitempty"`
	Extractor string              `json:"extractor"`
	DocHash   string              `json:"doc_hash"`
	Blocks    []sourcefetch.Block `json:"blocks"`
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
	// Repeats lists the existing capture each repeated item matched.
	Repeats []CapturedItem `json:"repeats,omitempty"`
	// Errors names items skipped for a reason other than a missing body or
	// URL.
	Errors []string `json:"errors,omitempty"`
}

type CapturedItem struct {
	CaptureID   string `json:"capture_id"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at,omitempty"`
	DocHash     string `json:"doc_hash,omitempty"`
	BlockCount  int    `json:"block_count,omitempty"`
}

// WriteCaptures writes each item as one choir.web_capture whose blobs live
// in the body store, never in the graph row (Autopaper W1; on the host the
// graph is Store B). A capture is a block document (Cut 2 unit 2a): it keeps
// the raw bytes, the block JSON and the block projection as three blobs.
// Its identity is its URL and doc_hash, so a re-capture whose blocks are
// unchanged is a repeat (markup, nonce and ad churn included) and a
// re-capture whose text changed is a new capture beside the old one. The
// blobs are stored before the object is written, so an object never names
// bytes that are not there.
func WriteCaptures(ctx context.Context, graph *objectgraph.Service, bodies BodyStore, items []CaptureItem, cfg CaptureWriteConfig) (CaptureWriteResult, error) {
	var result CaptureWriteResult
	ownerID := strings.TrimSpace(cfg.OwnerID)
	if graph == nil || bodies == nil || ownerID == "" {
		return result, fmt.Errorf("write captures: graph, body store and owner are required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, in := range items {
		item := in.Item
		blocks, extractor, err := captureBlocks(in)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", item.URL, err))
			continue
		}
		if len(blocks) == 0 {
			result.Skipped++
			continue
		}
		raw, mediaType := in.Raw, strings.TrimSpace(in.MediaType)
		if len(raw) == 0 {
			raw = []byte(item.Body)
			if mediaType == "" {
				mediaType = "text/plain; charset=utf-8"
			}
		}
		projection := sourcefetch.Projection(blocks)
		item.Body = projection
		req, _, ok := webCaptureGraphRequestsFromItem(item, ownerID, strings.TrimSpace(cfg.ComputerID), now)
		if !ok {
			result.Skipped++
			continue
		}
		docHash := sourcefetch.DocHash(blocks)
		key := "autopaper:capture:" + req.CanonicalURL + "#" + docHash
		id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, ownerID, objectgraph.StableSuffixFromKey(key))
		if err != nil {
			return result, fmt.Errorf("write captures: %w", err)
		}
		captured := CapturedItem{CaptureID: id, Title: req.Title, URL: req.CanonicalURL, DocHash: docHash, BlockCount: len(blocks)}
		if !item.Published.IsZero() {
			captured.PublishedAt = item.Published.UTC().Format(time.RFC3339)
		}
		if _, err := graph.GetObject(ctx, id); err == nil {
			result.Repeated++
			result.Repeats = append(result.Repeats, captured)
			continue
		} else if !errors.Is(err, objectgraph.ErrNotFound) {
			return result, fmt.Errorf("write captures: look up %s: %w", item.ID, err)
		}
		if err := texturedoc.ValidateWorld(texturedoc.WorldDocument(id, blocks)); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", req.CanonicalURL, err))
			continue
		}
		rawRef, err := bodies.PutBody(raw)
		if err != nil {
			return result, fmt.Errorf("write captures: raw bytes for %s: %w", item.ID, err)
		}
		blocksJSON, err := json.Marshal(CaptureBlocks{Schema: CaptureBlocksSchema, CaptureID: id, URL: req.CanonicalURL, Title: req.Title,
			MediaType: mediaType, Extractor: extractor, DocHash: docHash, Blocks: blocks})
		if err != nil {
			return result, fmt.Errorf("write captures: blocks for %s: %w", item.ID, err)
		}
		blocksRef, err := bodies.PutBody(blocksJSON)
		if err != nil {
			return result, fmt.Errorf("write captures: blocks for %s: %w", item.ID, err)
		}
		textRef, err := bodies.PutBody([]byte(projection))
		if err != nil {
			return result, fmt.Errorf("write captures: text for %s: %w", item.ID, err)
		}
		item = sources.NormalizeItemBodyClassification(item)
		req.IdentityKey = key
		req.ContentBlobID = rawRef
		req.ExtractedTextBlobID = textRef
		req.BlocksBlobID = blocksRef
		req.Extractor = extractor
		req.DocHash = docHash
		req.BlockCount = len(blocks)
		req.MediaType = mediaType
		req.RawBytes = len(raw)
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
			result.Repeats = append(result.Repeats, captured)
			continue
		}
		result.New++
		result.CaptureIDs = append(result.CaptureIDs, obj.CanonicalID)
		result.Captured = append(result.Captured, captured)
	}
	return result, nil
}

// captureBlocks returns an item's blocks with ids recomputed from their
// text and links clipped to it, and the extractor version that made them:
// the item's own blocks, or its body split by TextBlocks.
func captureBlocks(in CaptureItem) ([]sourcefetch.Block, string, error) {
	if len(in.Blocks) == 0 {
		return sourcefetch.TextBlocks(in.Body), sourcefetch.TextExtractorVersion, nil
	}
	extractor := strings.TrimSpace(in.Extractor)
	if extractor == "" {
		return nil, "", fmt.Errorf("blocks carry no extractor version")
	}
	blocks := make([]sourcefetch.Block, 0, len(in.Blocks))
	for _, b := range in.Blocks {
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		var links []sourcefetch.Link
		for _, l := range b.Links {
			if l.Start >= 0 && l.Start < l.End && l.End <= len(b.Text) && strings.TrimSpace(l.Href) != "" {
				links = append(links, l)
			}
		}
		b.Links = links
		blocks = append(blocks, b)
	}
	sourcefetch.AssignIDs(blocks)
	return blocks, extractor, nil
}
