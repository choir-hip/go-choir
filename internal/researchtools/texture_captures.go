package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

// TextureCaptures resolves Texture's citation handles to captures through
// the same capture read path choir.Read and choir.ReadCapture use (corpusd's
// capture route, Store B). It is the production texturewrite.CaptureSource
// (Cut 2 2a, A2): a {{cite}} quote is checked against a capture's own
// blocks, and a handle with no block capture is refused as not captured.
type TextureCaptures struct {
	sink       CaptureSink
	computerID string
}

// NewTextureCaptures resolves through sink; a URL handle is looked up among
// computerID's captures, where research's Read writes them.
func NewTextureCaptures(sink CaptureSink, computerID string) *TextureCaptures {
	return &TextureCaptures{sink: sink, computerID: strings.TrimSpace(computerID)}
}

// textureCaptureListLimit bounds the URL lookup: the computer's newest
// captures are searched for the URL.
const textureCaptureListLimit = 1000

// textureCaptureWindow is the window the URL lookup reads when the capture
// index refuses or misses. A variable so tests can shrink it.
var textureCaptureWindow = textureCaptureListLimit

// textureCapturePageChars is one read's text budget; corpusd caps a read
// below 1 MiB of characters.
const textureCapturePageChars = 900_000

// CaptureBlocks resolves a capture id or a URL (the newest capture of it)
// to the capture's blocks. A capture written before Cut 2 has no blocks to
// quote, so it is not captured for citation.
func (t *TextureCaptures) CaptureBlocks(ctx context.Context, ownerID, handle string) (texturewrite.Capture, error) {
	if t == nil || t.sink == nil {
		return texturewrite.Capture{}, texturewrite.ErrNotCaptured
	}
	handle = strings.TrimSpace(handle)
	id := handle
	if kind, _, _, err := objectgraph.ParseCanonicalID(handle); err != nil || kind != objectgraph.WebCaptureObjectKind {
		found, err := t.newestCaptureOf(ctx, ownerID, handle)
		if err != nil {
			return texturewrite.Capture{}, err
		}
		id = found
	}
	return t.read(ctx, id)
}

// newestCaptureOf finds the newest block capture of a URL among the
// computer's captures: through corpusd's capture index over all of them,
// and, when the index refuses or misses (a capture written a moment ago
// can precede its projection row), among the newest
// textureCaptureListLimit, as before. CHOIR_CORPUS_SEARCH=window skips the
// index.
func (t *TextureCaptures) newestCaptureOf(ctx context.Context, ownerID, pageURL string) (string, error) {
	if resolver, ok := t.sink.(CaptureURLResolver); ok && corpusIndexEnabled() {
		if id, found, err := resolver.NewestCaptureOfURL(ctx, ownerID, t.computerID, pageURL, true); err == nil && found {
			return id, nil
		}
	}
	objs, err := t.sink.ListCaptures(ctx, ownerID, t.computerID, textureCaptureWindow)
	if err != nil {
		return "", fmt.Errorf("look up captures of %s: %w", pageURL, err)
	}
	want := normalizeCaptureURL(pageURL)
	type candidate struct {
		id      string
		fetched time.Time
		order   int
	}
	var found []candidate
	for i, obj := range objs {
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil || meta.BlocksBlobID == "" {
			continue
		}
		if normalizeCaptureURL(meta.URL) == want || normalizeCaptureURL(meta.CanonicalURL) == want {
			fetched, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(meta.FetchedAt))
			found = append(found, candidate{id: obj.CanonicalID, fetched: fetched, order: i})
		}
	}
	if len(found) == 0 {
		return "", texturewrite.ErrNotCaptured
	}
	// Newest first: by fetch time, then by the store's own newest-first
	// order.
	sort.SliceStable(found, func(i, j int) bool {
		if !found[i].fetched.Equal(found[j].fetched) {
			return found[i].fetched.After(found[j].fetched)
		}
		return found[i].order < found[j].order
	})
	return found[0].id, nil
}

// read reads every block of one capture, page by page.
func (t *TextureCaptures) read(ctx context.Context, id string) (texturewrite.Capture, error) {
	var out texturewrite.Capture
	from := 0
	for page := 0; ; page++ {
		raw, err := t.sink.ReadCapture(ctx, id, from, textureCapturePageChars)
		if errors.Is(err, ErrCaptureNotFound) {
			return texturewrite.Capture{}, texturewrite.ErrNotCaptured
		}
		if err != nil {
			return texturewrite.Capture{}, err
		}
		var got struct {
			CaptureID string              `json:"capture_id"`
			URL       string              `json:"url"`
			Title     string              `json:"title"`
			DocHash   string              `json:"doc_hash"`
			Blocks    []sourcefetch.Block `json:"blocks"`
			NextBlock int                 `json:"next_block"`
			Truncated bool                `json:"truncated"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return texturewrite.Capture{}, fmt.Errorf("read capture %s: %w", id, err)
		}
		if page == 0 {
			if got.DocHash == "" || len(got.Blocks) == 0 {
				// A flat-text capture from before Cut 2: no blocks to quote.
				return texturewrite.Capture{}, texturewrite.ErrNotCaptured
			}
			out = texturewrite.Capture{Doc: firstNonEmptyString(got.CaptureID, id), Version: got.DocHash, URL: got.URL, Title: got.Title}
		}
		for _, b := range got.Blocks {
			out.Blocks = append(out.Blocks, quotematch.Block{ID: b.ID, Text: b.Text})
		}
		if !got.Truncated || got.NextBlock <= from {
			return out, nil
		}
		from = got.NextBlock
	}
}

func normalizeCaptureURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}
