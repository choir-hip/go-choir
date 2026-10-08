package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// recoveryJobReadTimeout bounds the status-surface job read so a hung job
// authority cannot wedge owner status responses.
const recoveryJobReadTimeout = 2 * time.Second

// recoveryJobStatus mirrors the platform checkpoint/repair job document at
// /internal/computers/projection-base/jobs. Owner-visible status carries the
// real job JSON, never a synthesized summary.
type recoveryJobStatus struct {
	ComputerID        string `json:"computer_id"`
	Generation        uint64 `json:"generation,omitempty"`
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
	TargetSequence    uint64 `json:"target_sequence,omitempty"`
	WatermarkSequence uint64 `json:"watermark_sequence,omitempty"`
	SeedBaseRef       string `json:"seed_base_ref,omitempty"`
	BaseRef           string `json:"base_ref,omitempty"`
	Error             string `json:"error,omitempty"`
	Failures          int    `json:"failures,omitempty"`
	Alert             string `json:"alert,omitempty"`
	ColdTail          uint64 `json:"cold_tail,omitempty"`
}

func computerRecoveryJobComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/recovery/job"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	computerID, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false
	}
	return strings.TrimSpace(computerID), true
}

func isComputerRecoveryJobPath(path string) bool {
	_, ok := computerRecoveryJobComputerID(path)
	return ok
}

// HandleComputerRecoveryJob is the owner-visible checkpoint/repair job
// surface: GET reports the job bound to the caller's computer; POST requests
// one (deduplicated platform-side). Owner authorization reuses the lifecycle
// ownership join; the platform call carries the trusted internal caller plus
// the owner attestation, never raw internal privilege to the browser.
func (h *Handler) HandleComputerRecoveryJob(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerRecoveryJobComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:lifecycle"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
			return
		}
	}

	var target *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
	} else {
		target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}
	if h.cfg == nil || strings.TrimSpace(h.cfg.CorpusdURL) == "" || h.corpusd == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "recovery job authority unavailable"})
		return
	}

	reason := ""
	if r.Method == http.MethodPost {
		raw, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if readErr != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid recovery job request"})
			return
		}
		if len(bytes.TrimSpace(raw)) > 0 {
			var request struct {
				Reason        string `json:"reason"`
				GenesisRepair bool   `json:"genesis_repair"`
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid recovery job request"})
				return
			}
			if request.GenesisRepair {
				// Genesis repair is an exceptional operator mode; it is not
				// reachable from any owner-facing surface.
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "genesis repair is an operator-only recovery mode"})
				return
			}
			reason = strings.TrimSpace(request.Reason)
		}
	}

	targetURL, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/projection-base/jobs")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build recovery job request"})
		return
	}
	var body io.Reader
	if r.Method == http.MethodPost {
		payload, marshalErr := json.Marshal(map[string]any{
			"computer_id":    computerID,
			"reason":         reason,
			"genesis_repair": false,
		})
		if marshalErr != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build recovery job request"})
			return
		}
		body = bytes.NewReader(payload)
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL+"?"+url.Values{"computer_id": {computerID}}.Encode(), body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build recovery job request"})
		return
	}
	upstream.Header.Set("X-Internal-Caller", "true")
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	if r.Method == http.MethodPost {
		upstream.Header.Set("Content-Type", "application/json")
	}
	response, err := h.corpusd.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "recovery job authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid recovery job response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(raw)
}

// recoveryJobForComputer reads the owner-visible repair job for one computer
// as the trusted host service. Absent jobs and unreachable authority return
// nil so status surfaces degrade without inventing job state. The read is
// bounded so a hung job authority cannot wedge the status endpoint.
func (h *Handler) recoveryJobForComputer(r *http.Request, ownerID, computerID string) *recoveryJobStatus {
	if h == nil || h.cfg == nil || strings.TrimSpace(h.cfg.CorpusdURL) == "" || h.corpusd == nil {
		return nil
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil
	}
	targetURL, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/projection-base/jobs")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), recoveryJobReadTimeout)
	defer cancel()
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL+"?"+url.Values{"computer_id": {computerID}}.Encode(), nil)
	if err != nil {
		return nil
	}
	upstream.Header.Set("X-Internal-Caller", "true")
	if strings.TrimSpace(ownerID) != "" {
		upstream.Header.Set("X-Authenticated-User", strings.TrimSpace(ownerID))
	}
	response, err := h.corpusd.Do(upstream)
	if err != nil {
		return nil
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	var job recoveryJobStatus
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&job); err != nil {
		return nil
	}
	if strings.TrimSpace(job.ComputerID) == "" {
		job.ComputerID = computerID
	}
	return &job
}
