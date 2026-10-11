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
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// Autopaper captures are shared content (W1b2,
// docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// the paper's computer writes them here, into Store B, and any bound
// computer reads them. Bodies live on host disk by sha256, never in Dolt
// rows (the 104 G lesson).
const (
	maxCaptureItemsPerRequest = 100
	maxCaptureBodyBytes       = 1 << 20
	maxCaptureRequestBytes    = 64 << 20
)

// CaptureWriteRequest is one poll's items from the paper's computer.
type CaptureWriteRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id"`
	Items      []sources.Item `json:"items"`
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

// ConfigureCaptureBodies sets the host directory for capture bodies.
func (h *ObjectGraphHandler) ConfigureCaptureBodies(root string) {
	h.captureBodies = &CaptureBodyStore{root: filepath.Clean(root)}
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
