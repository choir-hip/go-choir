package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// ReportedRelease is what a computer last told vmctl it runs (P6,
// docs/archive/reports/p6-p8-release-adopt-2026-10-15.md): its effective release,
// tree, event head and CodeRef. It is a report, not an authority: vmctl
// never probes a guest to list it (I6), never derives divergence or route
// state from it (I8), and shows it next to its own construction_version so
// a disagreement is visible.
type ReportedRelease struct {
	ReleaseDigest      string `json:"release_digest"`
	CodeRef            string `json:"code_ref,omitempty"`
	TreeHash           string `json:"tree_hash,omitempty"`
	BuildCommit        string `json:"build_commit,omitempty"`
	OperationID        string `json:"operation_id,omitempty"`
	AdoptedReleaseID   string `json:"adopted_release_id,omitempty"`
	Layered            bool   `json:"layered,omitempty"`
	CodeCommit         string `json:"code_commit,omitempty"`
	EventHeadSequence  uint64 `json:"event_head_sequence,omitempty"`
	CanonicalEventHead string `json:"canonical_event_head,omitempty"`
	EffectiveEventHead string `json:"effective_event_head,omitempty"`
	ReportedAt         string `json:"reported_at,omitempty"`
}

// EffectiveReleaseReport is the guest's report request.
type EffectiveReleaseReport struct {
	ComputerID string          `json:"computer_id"`
	Release    ReportedRelease `json:"release"`
}

const effectiveReleaseReportPath = "/internal/vmctl/computers/effective-release"

func (r ReportedRelease) validate() error {
	if !computerevent.IsSHA256(r.ReleaseDigest) {
		return fmt.Errorf("release_digest must be a sha256")
	}
	for _, v := range []string{r.CanonicalEventHead, r.EffectiveEventHead, r.AdoptedReleaseID} {
		if v != "" && !computerevent.IsSHA256(v) {
			return fmt.Errorf("event heads and adopted_release_id must be sha256 digests")
		}
	}
	for _, v := range []string{r.CodeRef, r.TreeHash, r.BuildCommit, r.OperationID, r.CodeCommit} {
		if len(v) > 256 || strings.ContainsAny(v, "\x00\n\r") {
			return fmt.Errorf("report fields must be single short lines")
		}
	}
	return nil
}

// SetReportedRelease records a computer's report and persists it.
func (r *OwnershipRegistry) SetReportedRelease(computerID string, report ReportedRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.byComputerIDLocked(computerID)
	if own == nil {
		return fmt.Errorf("vmctl: no ownership for computer %s", computerID)
	}
	copied := report
	own.ReportedRelease = &copied
	return r.writePersistenceLocked()
}

// HandleEffectiveReleaseReport serves POST
// /internal/vmctl/computers/effective-release: a host caller, or the guest
// whose tap address binds to exactly the computer it names (I7).
func (h *Handler) HandleEffectiveReleaseReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if h == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ownership registry unavailable"})
		return
	}
	var request EffectiveReleaseReport
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.ComputerID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid effective release report"})
		return
	}
	if !isInternalCaller(r) {
		if err := h.bindRequestToGuestComputer(r, request.ComputerID); err != nil {
			writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "caller not bound to the reported computer: " + err.Error()})
			return
		}
	}
	if err := request.Release.validate(); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	request.Release.ReportedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := h.registry.SetReportedRelease(strings.TrimSpace(request.ComputerID), request.Release); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// ReportEffectiveRelease posts this computer's identity to vmctl.
func (c *Client) ReportEffectiveRelease(ctx context.Context, computerID string, report ReportedRelease) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("vmctl client: not configured")
	}
	payload, err := json.Marshal(EffectiveReleaseReport{ComputerID: computerID, Release: report})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+effectiveReleaseReportPath, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vmctl client: effective release report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("vmctl client: effective release report returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
