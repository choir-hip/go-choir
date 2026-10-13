package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

const (
	// readMaxRawBytes matches corpusd's bound on one item's raw bytes.
	readMaxRawBytes = 16 << 20
	// readShownChars bounds the block text one Read returns to the cell;
	// ReadCapture(captureID, nextBlock, maxChars) reads the rest.
	readShownChars = 6000
	readUserAgent  = "Mozilla/5.0 (compatible; ChoirResearch/1.0; +https://choir.news)"
)

// ReadBlock is one block as Read and the trace show it: its cid, kind,
// heading level, PDF page and text.
type ReadBlock struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Level int    `json:"level,omitempty"`
	Page  int    `json:"page,omitempty"`
	Text  string `json:"text"`
}

// ReadHeading is one heading of a long page's digest, with its block index.
type ReadHeading struct {
	ID    string `json:"id"`
	Index int    `json:"index"`
	Level int    `json:"level,omitempty"`
	Text  string `json:"text"`
}

// ReadResult is what choir.Read returns: the capture it made or matched,
// and the page's first blocks. When the page is longer than one read shows,
// NextBlock names the first block not shown and Headings is a digest of
// every heading with its block index.
type ReadResult struct {
	CaptureID  string        `json:"capture_id"`
	New        bool          `json:"new"`
	URL        string        `json:"url"`
	Title      string        `json:"title,omitempty"`
	MediaType  string        `json:"media_type"`
	Extractor  string        `json:"extractor"`
	DocHash    string        `json:"doc_hash"`
	RawBytes   int           `json:"raw_bytes"`
	BlockCount int           `json:"block_count"`
	Blocks     []ReadBlock   `json:"blocks"`
	Truncated  bool          `json:"truncated,omitempty"`
	NextBlock  int           `json:"next_block,omitempty"`
	Headings   []ReadHeading `json:"headings,omitempty"`
	Warnings   []string      `json:"warnings,omitempty"`
}

// newReadTool is research's one world-read verb (Cut 2 unit 2a; it
// replaces fetch_url, import_url_content, import_document_content and the
// content-item selector reads). It fetches a page or PDF through the
// address-guarded client, extracts its blocks, and writes it through the
// one capture writer (corpusd, Store B), so every page research reads is a
// capture it can cite: a block document with raw bytes, block JSON and
// projection. An unchanged re-read returns the existing capture.
func newReadTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		URL string `json:"url"`
	}
	return toolregistry.Tool{Name: "read",
		Description: "Fetch a web page or PDF into a shared capture and return its id and first blocks.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"url": map[string]any{"type": "string"},
		}, []string{"url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode read args: %w", err)
			}
			exec := toolregistry.ExecutionContextFrom(ctx)
			if exec.OwnerID == "" {
				return "", fmt.Errorf("read missing owner context")
			}
			if deps.Captures == nil {
				return "", fmt.Errorf("read: the shared capture store is not configured")
			}
			out, err := readPage(ctx, deps, exec.OwnerID, exec.ComputerID, strings.TrimSpace(in.URL))
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(out)
		}}
}

func readPage(ctx context.Context, deps Dependencies, ownerID, computerID, target string) (ReadResult, error) {
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ReadResult{}, fmt.Errorf("read: %q is not an http(s) URL", target)
	}
	client := deps.HTTP
	if client == nil {
		client = sourcefetch.Client(30 * time.Second)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read %s: %w", target, err)
	}
	req.Header.Set("User-Agent", readUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf,text/plain;q=0.9,*/*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read %s: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return ReadResult{}, fmt.Errorf("read %s: HTTP %d (a page that refuses a read is not missing; another copy or the publisher's own page may read)", target, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, readMaxRawBytes+1))
	if err != nil {
		return ReadResult{}, fmt.Errorf("read %s: %w", target, err)
	}
	if len(body) > readMaxRawBytes {
		return ReadResult{}, fmt.Errorf("read %s: larger than %d MiB", target, readMaxRawBytes>>20)
	}
	finalURL := target
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	mediaType := readMediaType(resp.Header.Get("Content-Type"), finalURL, body)

	var doc sourcefetch.Document
	var warnings []string
	switch {
	case mediaType == "text/html" || mediaType == "application/xhtml+xml":
		doc = sourcefetch.ExtractBlocks(body)
	case mediaType == "application/pdf":
		doc, warnings = contentowner.PDFBlocks(ctx, body)
	case mediaType == "text/plain" || mediaType == "text/markdown":
		doc = sourcefetch.Document{Extractor: sourcefetch.TextExtractorVersion, Blocks: sourcefetch.TextBlocks(string(body))}
	default:
		return ReadResult{}, fmt.Errorf("read %s: cannot read %s; Read takes HTML, PDF and plain text", target, mediaType)
	}
	if len(doc.Blocks) == 0 {
		return ReadResult{}, fmt.Errorf("read %s: no readable text (the page may need a browser, or block automated readers)", target)
	}

	item := sourcegraph.ReadCaptureItem(target, body, mediaType, doc, time.Now().UTC())
	item.CanonicalURL = finalURL
	written, err := deps.Captures.WriteCaptures(ctx, ownerID, computerID, []sourcegraph.CaptureItem{item})
	if err != nil {
		return ReadResult{}, fmt.Errorf("read %s: capture: %w", target, err)
	}
	out := ReadResult{URL: finalURL, Title: doc.Title, MediaType: mediaType, Extractor: doc.Extractor,
		DocHash: doc.DocHash(), RawBytes: len(body), BlockCount: len(doc.Blocks), Warnings: warnings}
	switch {
	case len(written.Captured) > 0:
		out.CaptureID, out.New = written.Captured[0].CaptureID, true
	case len(written.Repeats) > 0:
		out.CaptureID = written.Repeats[0].CaptureID
	default:
		return ReadResult{}, fmt.Errorf("read %s: the capture was not written: %s", target, strings.Join(written.Errors, "; "))
	}
	chars := 0
	for i, b := range doc.Blocks {
		n := len([]rune(b.Text))
		if i > 0 && chars+n > readShownChars {
			out.Truncated, out.NextBlock = true, i
			break
		}
		chars += n
		out.Blocks = append(out.Blocks, ReadBlock{ID: b.ID, Kind: b.Kind, Level: b.Level, Page: b.Page, Text: b.Text})
	}
	if out.Truncated {
		for i, b := range doc.Blocks {
			if b.Kind == sourcefetch.KindHeading {
				out.Headings = append(out.Headings, ReadHeading{ID: b.ID, Index: i, Level: b.Level, Text: b.Text})
			}
		}
	}
	return out, nil
}

// readMediaType names what was fetched: the Content-Type's media type,
// or, when that is missing or generic, the URL's extension or a sniff.
func readMediaType(header, pageURL string, body []byte) string {
	mediaType, _, _ := mime.ParseMediaType(header)
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType != "" && mediaType != "application/octet-stream" && mediaType != "binary/octet-stream" {
		return mediaType
	}
	if parsed, err := url.Parse(pageURL); err == nil {
		switch strings.ToLower(path.Ext(parsed.Path)) {
		case ".pdf":
			return "application/pdf"
		case ".html", ".htm":
			return "text/html"
		case ".txt":
			return "text/plain"
		case ".md":
			return "text/markdown"
		}
	}
	sniffed, _, _ := mime.ParseMediaType(http.DetectContentType(body))
	return sniffed
}
