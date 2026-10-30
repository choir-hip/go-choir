package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

// Autopaper captures are shared content (W1b2,
// docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// the paper's computer writes them here, into Store B, and any bound
// computer reads them. Bodies live on host disk by sha256, never in Dolt
// rows (the 104 G lesson).
const (
	maxCaptureItemsPerRequest = 100
	maxCaptureBodyBytes       = 1 << 20
	// maxCaptureRawBytes bounds one item's fetched bytes (a page or a PDF
	// read by choir.Read); base64 in the request JSON keeps one such item
	// well under maxCaptureRequestBytes.
	maxCaptureRawBytes      = 16 << 20
	maxCaptureRequestBytes  = 64 << 20
	defaultCaptureReadChars = 8000
)

// CaptureReadResponse is one shared capture. A block capture (Cut 2 unit
// 2a) returns its blocks from FromBlock, as many as fit in max_chars of
// text (at least one), with NextBlock naming where the rest start (0 when
// none remain); its Body is empty, since the blocks are its text. A capture
// written before Cut 2 has no blocks and returns its flat body text.
type CaptureReadResponse struct {
	CaptureID     string              `json:"capture_id"`
	OwnerID       string              `json:"owner_id"`
	ComputerID    string              `json:"computer_id,omitempty"`
	SourceID      string              `json:"source_id,omitempty"`
	Title         string              `json:"title,omitempty"`
	URL           string              `json:"url"`
	PublishedAt   string              `json:"published_at,omitempty"`
	FetchedAt     string              `json:"fetched_at,omitempty"`
	ContentBlobID string              `json:"content_blob_id"`
	MediaType     string              `json:"media_type,omitempty"`
	RawBytes      int                 `json:"raw_bytes,omitempty"`
	Extractor     string              `json:"extractor,omitempty"`
	DocHash       string              `json:"doc_hash,omitempty"`
	BlockCount    int                 `json:"block_count,omitempty"`
	FromBlock     int                 `json:"from_block,omitempty"`
	Blocks        []sourcefetch.Block `json:"blocks,omitempty"`
	NextBlock     int                 `json:"next_block,omitempty"`
	Body          string              `json:"body,omitempty"`
	BodyChars     int                 `json:"body_chars"`
	Truncated     bool                `json:"truncated,omitempty"`
}

// CaptureWriteRequest is one poll's items, or one page read, from a
// computer.
type CaptureWriteRequest struct {
	OwnerID    string                    `json:"owner_id"`
	ComputerID string                    `json:"computer_id"`
	Items      []sourcegraph.CaptureItem `json:"items"`
}

// CaptureWriteResponse counts what the host wrote.
type CaptureWriteResponse = sourcegraph.CaptureWriteResult

// CaptureBodyStore keeps capture bodies on host disk, addressed by sha256.
type CaptureBodyStore struct {
	root string
}

// PutBody stores body under its hash and returns "sha256:<hex>". A body
// already present is not rewritten; a new one is written to a temp file,
// synced and renamed, so a crash never leaves a partial body under its hash.
func (s *CaptureBodyStore) PutBody(body []byte) (string, error) {
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	path := s.path(hash)
	if _, err := os.Stat(path); err == nil {
		return "sha256:" + hash, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("capture body: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".body-*")
	if err != nil {
		return "", fmt.Errorf("capture body: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("capture body: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("capture body: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("capture body: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", fmt.Errorf("capture body: %w", err)
	}
	return "sha256:" + hash, nil
}

// Body returns the stored bytes for hash, verifying them.
func (s *CaptureBodyStore) Body(hash string) ([]byte, error) {
	if len(hash) != 64 {
		return nil, os.ErrNotExist
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return nil, os.ErrNotExist
	}
	body, err := os.ReadFile(s.path(hash))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("capture body %s fails its hash", hash)
	}
	return body, nil
}

func (s *CaptureBodyStore) path(hash string) string {
	return filepath.Join(s.root, hash[:2], hash)
}

// CaptureBodiesRoot is the host directory of capture bodies under the
// platform artifacts root: the capture routes write it and publish reads it
// to pin cited blocks.
func CaptureBodiesRoot(artifactsRoot string) string {
	return filepath.Join(filepath.Clean(artifactsRoot), "capture-bodies")
}

// ConfigureCaptureBodies sets the host directory for capture bodies.
func (h *ObjectGraphHandler) ConfigureCaptureBodies(root string) {
	h.captureBodies = &CaptureBodyStore{root: filepath.Clean(root)}
}

// CaptureBodyStore returns the configured capture body store, or nil.
func (h *ObjectGraphHandler) CaptureBodyStore() *CaptureBodyStore {
	if h == nil {
		return nil
	}
	return h.captureBodies
}

// HandleCaptures handles POST /internal/platform/captures: one poll's items
// from a bound guest (or an internal caller). The guest must own the
// captures it writes, as for every guest object write.
func (h *ObjectGraphHandler) HandleCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.service == nil || h.captureBodies == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "capture store unavailable"})
		return
	}
	var req CaptureWriteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCaptureRequestBytes)).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "capture request too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, req.OwnerID, req.ComputerID, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if len(req.Items) > maxCaptureItemsPerRequest {
		writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: fmt.Sprintf("at most %d items per request", maxCaptureItemsPerRequest)})
		return
	}
	for i := range req.Items {
		if len(req.Items[i].Raw) > maxCaptureRawBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: fmt.Sprintf("an item's raw bytes exceed %d", maxCaptureRawBytes)})
			return
		}
		if len(req.Items[i].Body) > maxCaptureBodyBytes {
			req.Items[i].Body = req.Items[i].Body[:maxCaptureBodyBytes]
		}
		req.Items[i].RawJSON = ""
	}
	result, err := sourcegraph.WriteCaptures(r.Context(), h.service, h.captureBodies, req.Items, sourcegraph.CaptureWriteConfig{
		OwnerID: strings.TrimSpace(req.OwnerID), ComputerID: strings.TrimSpace(req.ComputerID), Now: time.Now().UTC(),
	})
	if err != nil {
		log.Printf("corpusd: write captures for %s: %v", req.ComputerID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleCaptureBody handles GET /internal/platform/captures/bodies/<sha256>
// for any bound guest or internal caller: captures are shared content.
func (h *ObjectGraphHandler) HandleCaptureBody(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, "", "", true); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if h == nil || h.captureBodies == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "capture store unavailable"})
		return
	}
	hash := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/platform/captures/bodies/"), "/")
	body, err := h.captureBodies.Body(strings.TrimPrefix(hash, "sha256:"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.Copy(w, strings.NewReader(string(body)))
}

// HandleCaptureRead handles GET /internal/platform/captures/<capture id>
// (?max_chars=) for any bound guest or internal caller: one shared capture's
// metadata and body text, read from Store B and host disk.
func (h *ObjectGraphHandler) HandleCaptureRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, "", "", true); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if h == nil || h.service == nil || h.captureBodies == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "capture store unavailable"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/platform/captures/"), "/")
	kind, _, _, err := objectgraph.ParseCanonicalID(id)
	if err != nil || kind != objectgraph.WebCaptureObjectKind {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "not a capture id"})
		return
	}
	obj, err := h.service.GetObject(r.Context(), id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "capture not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	// ExtractedTextBlobID is the text in both shapes: the block projection,
	// or the one flat body of a capture written before Cut 2.
	body, err := h.captureBodies.Body(strings.TrimPrefix(meta.ExtractedTextBlobID, "sha256:"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture body: " + err.Error()})
		return
	}
	maxChars := defaultCaptureReadChars
	if v, convErr := strconv.Atoi(r.URL.Query().Get("max_chars")); convErr == nil && v > 0 && v < maxCaptureBodyBytes {
		maxChars = v
	}
	text := []rune(string(body))
	out := CaptureReadResponse{
		CaptureID: obj.CanonicalID, OwnerID: obj.OwnerID, ComputerID: obj.ComputerID,
		SourceID: meta.SourceID, Title: meta.Title, URL: meta.URL, PublishedAt: meta.PublishedAt,
		FetchedAt: meta.FetchedAt, ContentBlobID: meta.ContentBlobID, BodyChars: len(text),
		MediaType: meta.MediaType, RawBytes: meta.RawBytes, Extractor: meta.Extractor, DocHash: meta.DocHash, BlockCount: meta.BlockCount,
	}
	if meta.BlocksBlobID == "" {
		if len(text) > maxChars {
			text, out.Truncated = text[:maxChars], true
		}
		out.Body = string(text)
		writeJSON(w, http.StatusOK, out)
		return
	}
	raw, err := h.captureBodies.Body(strings.TrimPrefix(meta.BlocksBlobID, "sha256:"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture blocks: " + err.Error()})
		return
	}
	var doc sourcegraph.CaptureBlocks
	if err := json.Unmarshal(raw, &doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture blocks: " + err.Error()})
		return
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from_block"))
	from = max(0, min(from, len(doc.Blocks)))
	out.FromBlock, out.Blocks = from, []sourcefetch.Block{}
	chars := 0
	for i := from; i < len(doc.Blocks); i++ {
		b := doc.Blocks[i]
		n := len([]rune(b.Text))
		if len(out.Blocks) > 0 && chars+n > maxChars {
			out.NextBlock, out.Truncated = i, true
			break
		}
		chars += n
		b.Links = nil // the blob keeps them; a reader quotes text
		out.Blocks = append(out.Blocks, b)
	}
	writeJSON(w, http.StatusOK, out)
}
