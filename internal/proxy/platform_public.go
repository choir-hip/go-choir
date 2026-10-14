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

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

type proposalRequest struct {
	DocID                string                     `json:"doc_id"`
	RevisionID           string                     `json:"revision_id,omitempty"`
	PublicationVersionID string                     `json:"publication_version_id,omitempty"`
	Transclusions        []platform.TransclusionRef `json:"transclusions,omitempty"`
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
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/resolve")
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

// HandlePlatformFrontStories serves the Autopaper front list to anyone,
// signed in or not. It forwards to corpusd, which reads Store B alone: no
// computer is resolved or called, so the front page stays up while the
// paper computer is stopped (Cut 2 unit 2b, Reader W3a).
func (h *Handler) HandlePlatformFrontStories(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/front")
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
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		q.Set("limit", limit)
	}
	if cursor := strings.TrimSpace(r.URL.Query().Get("cursor")); cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()
	var out platform.FrontPage
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusBadRequest {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid front list request"})
			h.lifecycle.record("platform_front.list", lifecycleHTTPStatus(http.StatusBadRequest), time.Since(started))
			return
		}
		log.Printf("proxy: platform front stories: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to list stories"})
		h.lifecycle.record("platform_front.list", "platform_error", time.Since(started))
		return
	}
	writeJSON(w, status, out)
	h.lifecycle.record("platform_front.list", lifecycleHTTPStatus(status), time.Since(started))
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
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/export")
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
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/retrieval/search")
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

	desktopID := requestDesktopID(r)
	computerTarget, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", desktopID)
	if !ok {
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

	autoputerURL, err := h.resolveComputerURLForComputerTarget(r.Context(), authResult, computerTarget, desktopID)
	if err != nil {
		log.Printf("proxy: platform proposal resolve autoputer: %v", err)
		writeAutoputerResolveFailure(w, http.StatusBadGateway, "failed to resolve user autoputer", err)
		return
	}
	var doc autoputerTextureDocument
	if err := h.fetchAutoputerJSON(r, autoputerURL, "/api/texture/documents/"+url.PathEscape(req.DocID), authResult.UserID, &doc); err != nil {
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
	var rev autoputerTextureRevision
	if err := h.fetchAutoputerJSON(r, autoputerURL, "/api/texture/revisions/"+url.PathEscape(req.RevisionID), authResult.UserID, &rev); err != nil {
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
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/"+url.PathEscape(publicationID)+"/proposals")
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
	hostauth.MarkInternal(httpReq)
	resp, err := h.corpusd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: platform proposal post corpusd: %v", err)
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
		// corpusd records the proposal for its author (delivery_state
		// recorded_for_author). There is no guest delivery: R3d deleted the
		// author computer's /internal/texture/proposals route, and the proxy
		// must not resolve (and so start) the author's computer for a reader.
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
			out.Error = fmt.Sprintf("corpusd status %d", resp.StatusCode)
		}
	}
	writeJSON(w, resp.StatusCode, out)
	h.lifecycle.record("platform_proposal.total", lifecycleHTTPStatus(resp.StatusCode), time.Since(started))
}

type platformStatusError struct {
	status int
	body   string
}

func (e *platformStatusError) Error() string {
	return fmt.Sprintf("corpusd status %d: %s", e.status, strings.TrimSpace(e.body))
}

func (h *Handler) getPlatformJSON(r *http.Request, target string, out any) (int, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return 0, fmt.Errorf("build platform request: %w", err)
	}
	hostauth.MarkInternal(req)
	resp, err := h.corpusd.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call corpusd: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read corpusd response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, &platformStatusError{status: resp.StatusCode, body: string(body)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return resp.StatusCode, fmt.Errorf("decode corpusd response: %w", err)
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
