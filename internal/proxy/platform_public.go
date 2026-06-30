package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

type proposalRequest struct {
	DocID                string                     `json:"doc_id"`
	RevisionID           string                     `json:"revision_id,omitempty"`
	PublicationVersionID string                     `json:"publication_version_id,omitempty"`
	Transclusions        []platform.TransclusionRef `json:"transclusions,omitempty"`
}

type authorProposalDeliveryRequest struct {
	OwnerID              string `json:"owner_id"`
	ProposalID           string `json:"proposal_id"`
	PublicationID        string `json:"publication_id"`
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	SubmitterID          string `json:"submitter_id,omitempty"`
	DeliveryID           string `json:"delivery_id,omitempty"`
}

type authorProposalDeliveryResponse struct {
	DeliveryID    string `json:"delivery_id"`
	TargetAgentID string `json:"target_agent_id"`
	ChannelID     string `json:"channel_id"`
	State         string `json:"state"`
	RunID         string `json:"loop_id,omitempty"`
}

type publicationProposalClientResponse struct {
	ProposalID           string   `json:"proposal_id"`
	PublicationID        string   `json:"publication_id"`
	PublicationVersionID string   `json:"publication_version_id"`
	SubmitterID          string   `json:"submitter_id"`
	ContentHash          string   `json:"content_hash"`
	ProposalRevisionHash string   `json:"proposal_revision_hash"`
	ArtifactManifestID   string   `json:"artifact_manifest_id"`
	TransclusionIDs      []string `json:"transclusion_ids"`
	CitationIDs          []string `json:"citation_ids"`
	DeliveryID           string   `json:"delivery_id"`
	DeliveryState        string   `json:"delivery_state"`
	State                string   `json:"state"`
}

func (h *Handler) HandlePlatformPublicationResolve(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "route is required"})
		return
	}
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/publications/resolve")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	q := u.Query()
	q.Set("route", routePath)
	u.RawQuery = q.Encode()
	var out platform.PublicationBundle
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusNotFound {
			http.NotFound(w, r)
			h.lifecycle.record("platform_publication.resolve", lifecycleHTTPStatus(http.StatusNotFound), time.Since(started))
			return
		}
		log.Printf("proxy: platform publication resolve: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve publication"})
		return
	}
	writeJSON(w, status, out)
	h.lifecycle.record("platform_publication.resolve", lifecycleHTTPStatus(status), time.Since(started))
}

func (h *Handler) HandlePlatformPublicationExport(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "route is required"})
		return
	}
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/publications/export")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	q := u.Query()
	q.Set("route", routePath)
	q.Set("format", r.URL.Query().Get("format"))
	u.RawQuery = q.Encode()
	var out platform.PublicationExport
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusNotFound {
			http.NotFound(w, r)
			h.lifecycle.record("platform_publication.export", lifecycleHTTPStatus(http.StatusNotFound), time.Since(started))
			return
		}
		log.Printf("proxy: platform publication export: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to export publication"})
		return
	}
	writeJSON(w, status, out)
	h.lifecycle.record("platform_publication.export", lifecycleHTTPStatus(status), time.Since(started))
}

func (h *Handler) HandlePlatformRetrievalSearch(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/retrieval/search")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	q := u.Query()
	q.Set("q", r.URL.Query().Get("q"))
	u.RawQuery = q.Encode()
	var out platform.RetrievalSearchResponse
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		log.Printf("proxy: platform retrieval search: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to search published retrieval sources"})
		return
	}
	writeJSON(w, status, out)
	h.lifecycle.record("platform_retrieval.search", lifecycleHTTPStatus(status), time.Since(started))
}

func (h *Handler) HandlePublicationProposal(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	publicationID := publicationIDFromAPIProposalPath(r.URL.Path)
	if publicationID == "" {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("platform_proposal.auth", "unauthorized", time.Since(started))
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record("platform_proposal.authz", "forbidden", time.Since(started))
		return
	}
	var req proposalRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.RevisionID = strings.TrimSpace(req.RevisionID)
	if req.DocID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "doc_id is required"})
		return
	}

	desktopID := requestDesktopID(r)
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy: platform proposal resolve sandbox: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		return
	}
	var doc sandboxTextureDocument
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/texture/documents/"+url.PathEscape(req.DocID), authResult.UserID, &doc); err != nil {
		log.Printf("proxy: platform proposal fetch document: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private texture document"})
		return
	}
	if doc.OwnerID != authResult.UserID || doc.DocID != req.DocID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "document does not belong to authenticated user"})
		return
	}
	if req.RevisionID == "" {
		req.RevisionID = doc.CurrentRevisionID
	}
	if req.RevisionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "document has no revision to propose"})
		return
	}
	var rev sandboxTextureRevision
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/texture/revisions/"+url.PathEscape(req.RevisionID), authResult.UserID, &rev); err != nil {
		log.Printf("proxy: platform proposal fetch revision: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private texture revision"})
		return
	}
	if rev.OwnerID != authResult.UserID || rev.DocID != req.DocID || rev.RevisionID != req.RevisionID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision does not belong to authenticated document"})
		return
	}

	platformReq := platform.SubmitPublicationProposalRequest{
		PublicationID:        publicationID,
		PublicationVersionID: strings.TrimSpace(req.PublicationVersionID),
		SubmitterID:          authResult.UserID,
		SubmitterDocID:       doc.DocID,
		SubmitterRevisionID:  rev.RevisionID,
		Title:                doc.Title,
		Content:              rev.Content,
		Transclusions:        req.Transclusions,
		Citations:            rev.Citations,
		RequestedBy:          authResult.UserID,
	}
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/publications/"+url.PathEscape(publicationID)+"/proposals")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	data, err := json.Marshal(platformReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := h.platformd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: platform proposal post platformd: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to submit publication proposal"})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to read platform response"})
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out platform.SubmitPublicationProposalResponse
		if err := json.Unmarshal(body, &out); err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to decode platform response"})
			return
		}
		if state := h.deliverPublicationProposalToAuthor(r, out); state != "" && state != out.DeliveryState {
			if err := h.recordPublicationProposalDeliveryState(r, out, state); err != nil {
				log.Printf("proxy: platform proposal delivery state update failed: %v", err)
			} else {
				out.DeliveryState = state
			}
		} else if state != "" {
			out.DeliveryState = state
		}
		writeJSON(w, resp.StatusCode, publicationProposalClientResponse{
			ProposalID:           out.ProposalID,
			PublicationID:        out.PublicationID,
			PublicationVersionID: out.PublicationVersionID,
			SubmitterID:          out.SubmitterID,
			ContentHash:          out.ContentHash,
			ProposalRevisionHash: out.ProposalRevisionHash,
			ArtifactManifestID:   out.ArtifactManifestID,
			TransclusionIDs:      out.TransclusionIDs,
			CitationIDs:          out.CitationIDs,
			DeliveryID:           out.DeliveryID,
			DeliveryState:        out.DeliveryState,
			State:                out.State,
		})
		h.lifecycle.record("platform_proposal.total", lifecycleHTTPStatus(resp.StatusCode), time.Since(started))
		return
	}
	var out errorResponse
	if err := json.Unmarshal(body, &out); err != nil || out.Error == "" {
		out.Error = strings.TrimSpace(string(body))
		if out.Error == "" {
			out.Error = fmt.Sprintf("platformd status %d", resp.StatusCode)
		}
	}
	writeJSON(w, resp.StatusCode, out)
	h.lifecycle.record("platform_proposal.total", lifecycleHTTPStatus(resp.StatusCode), time.Since(started))
}

func (h *Handler) deliverPublicationProposalToAuthor(r *http.Request, proposal platform.SubmitPublicationProposalResponse) string {
	if h == nil || r == nil || strings.TrimSpace(proposal.SourceOwnerID) == "" || strings.TrimSpace(proposal.ProposalID) == "" {
		return ""
	}
	sandboxURL, err := h.resolveSandboxURL(r.Context(), proposal.SourceOwnerID, "")
	if err != nil {
		log.Printf("proxy: platform proposal author sandbox resolve failed: %v", err)
		return "recorded_for_author"
	}
	target, err := joinBasePath(sandboxURL, "/internal/texture/proposals")
	if err != nil {
		log.Printf("proxy: platform proposal author delivery target failed: %v", err)
		return "recorded_for_author"
	}
	payload := authorProposalDeliveryRequest{
		OwnerID:              proposal.SourceOwnerID,
		ProposalID:           proposal.ProposalID,
		PublicationID:        proposal.PublicationID,
		PublicationVersionID: proposal.PublicationVersionID,
		SubmitterID:          proposal.SubmitterID,
		DeliveryID:           proposal.DeliveryID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "recorded_for_author"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return "recorded_for_author"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("proxy: platform proposal author delivery failed: %v", err)
		return "recorded_for_author"
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Printf("proxy: platform proposal author delivery status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		return "recorded_for_author"
	}
	var out authorProposalDeliveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "delivered"
	}
	if strings.TrimSpace(out.State) != "" {
		return out.State
	}
	return "delivered"
}

func (h *Handler) recordPublicationProposalDeliveryState(r *http.Request, proposal platform.SubmitPublicationProposalResponse, state string) error {
	state = strings.TrimSpace(state)
	if state == "" || proposal.ProposalID == "" || proposal.DeliveryID == "" {
		return nil
	}
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/proposal-deliveries/state")
	if err != nil {
		return err
	}
	reqBody := platform.UpdateProposalDeliveryStateRequest{
		ProposalID:    proposal.ProposalID,
		DeliveryID:    proposal.DeliveryID,
		DeliveryState: state,
		DeliveryRef:   "author-runtime:" + state,
		RecordedBy:    "proxy",
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal platform delivery update: %w", err)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build platform delivery update: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.platformd.Do(req)
	if err != nil {
		return fmt.Errorf("call platform delivery update: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("platform delivery update status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type platformStatusError struct {
	status int
	body   string
}

func (e *platformStatusError) Error() string {
	return fmt.Sprintf("platformd status %d: %s", e.status, strings.TrimSpace(e.body))
}

func (h *Handler) getPlatformJSON(r *http.Request, target string, out any) (int, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return 0, fmt.Errorf("build platform request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.platformd.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call platformd: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read platformd response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, &platformStatusError{status: resp.StatusCode, body: string(body)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return resp.StatusCode, fmt.Errorf("decode platformd response: %w", err)
	}
	return resp.StatusCode, nil
}

func publicationIDFromAPIProposalPath(path string) string {
	const prefix = "/api/platform/publications/"
	const suffix = "/proposals"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	return strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix), "/")
}
