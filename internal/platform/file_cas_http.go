package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/filecas"
)

const maxFileCASChunkBytes = 8 << 20

type fileCASRootRequest struct {
	ComputerID   string `json:"computer_id"`
	Manifest     string `json:"manifest"`
	HeadSequence int64  `json:"head_sequence"`
}

type fileCASWatermarkRequest struct {
	ComputerID        string `json:"computer_id"`
	WatermarkSequence int64  `json:"watermark_sequence"`
	BaseRef           string `json:"base_ref"`
}

func (h *Handler) HandleFileCASChunk(w http.ResponseWriter, r *http.Request) {
	digest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/computers/files/chunks/"), "/")
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if !validFileCASDigest(digest) || computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id and chunk digest are required"})
		return
	}
	scope := "event:read"
	if r.Method == http.MethodPut {
		scope = "event:append"
	}
	if r.Method != http.MethodPut && r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !h.authorizeFileCAS(r, computerID, scope) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, maxFileCASChunkBytes)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "chunk exceeds 8 MiB limit"})
			return
		}
		sum := sha256.Sum256(data)
		if digest != hex.EncodeToString(sum[:]) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "chunk digest mismatch"})
			return
		}
		if err := h.service.PinFileChunk(r.Context(), computerID, digest, data); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"digest": digest})
	case http.MethodGet:
		data, err := h.service.GetFileChunk(r.Context(), computerID, digest)
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read chunk"})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func (h *Handler) HandleFileCASRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var input fileCASRootRequest
		if !decodeFileCASJSON(r, &input) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
			return
		}
		if !h.authorizeFileCAS(r, input.ComputerID, "event:append") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		manifest, err := filecas.ParseManifest([]byte(input.Manifest))
		if err != nil || manifest.ComputerID != input.ComputerID || input.HeadSequence < 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid file manifest"})
			return
		}
		if h == nil || h.service == nil || h.service.store == nil {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
			return
		}
		manifestRef, err := h.service.pinFileManifest(input.ComputerID, manifest.Root, []byte(input.Manifest))
		if err == nil {
			err = h.service.store.RecordFileRoot(r.Context(), input.ComputerID, manifest.Root, manifestRef, input.HeadSequence)
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"root": manifest.Root, "manifest_ref": manifestRef})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID, root := strings.TrimSpace(r.URL.Query().Get("computer_id")), strings.TrimSpace(r.URL.Query().Get("root"))
	if !h.authorizeFileCAS(r, computerID, "event:read") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
		return
	}
	data, err := h.service.getFileManifest(computerID, root)
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid file root"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) HandleFileCASWatermark(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
		return
	}
	if r.Method == http.MethodPost {
		var input fileCASWatermarkRequest
		if !decodeFileCASJSON(r, &input) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
			return
		}
		if !h.authorizeFileCAS(r, input.ComputerID, "event:append") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		if err := h.service.store.RecordReplayWatermark(r.Context(), input.ComputerID, input.WatermarkSequence, input.BaseRef); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"watermark_sequence": input.WatermarkSequence, "base_ref": input.BaseRef})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if !h.authorizeFileCAS(r, computerID, "event:read") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	seq, baseRef, err := h.service.store.ReplayWatermark(r.Context(), computerID)
	if errors.Is(err, ErrNoWatermark) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read watermark"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"watermark_sequence": seq, "base_ref": baseRef})
}

func (h *Handler) HandleFileCASRoots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if !h.authorizeFileCAS(r, computerID, "event:read") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid limit"})
			return
		}
		limit = parsed
	}
	roots, err := h.service.store.LatestFileRoots(r.Context(), computerID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list roots"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roots": roots})
}

func (h *Handler) authorizeFileCAS(r *http.Request, computerID, scope string) bool {
	return h != nil && strings.TrimSpace(computerID) != "" && (r.Header.Get("X-Internal-Caller") == "true" || (h.eventAuth != nil && h.eventAuth.Authorize(r, computerID, scope) == nil))
}

func decodeFileCASJSON(r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}
