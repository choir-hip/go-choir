package platform

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type Handler struct {
	service              *Service
	eventCAS             *ComputerEventCAS
	eventArtifacts       *EventArtifactService
	eventAuth            EventCapabilityAuthorizer
	selfDevelopmentModes *SelfDevelopmentModeCAS
	consensus            *ConsensusActuator
	checkpointAuthority  *CheckpointAuthority
	keyEscrow            *keyEscrowRuntime
	guestLookup          guestOwnershipLookup
	hostBuild            *hostbuild.Service
	releaseReads         ReleaseReadsMode
}
type healthResponse struct {
	Status  string         `json:"status"`
	Service string         `json:"service"`
	Store   string         `json:"store"`
	Build   buildinfo.Info `json:"build"`
	// Retrieval counts and times Store B retrieval calls since start
	// (docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
	Retrieval *RetrievalSnapshot `json:"retrieval,omitempty"`
	// Tape is the tape segment flag and the file/record counters of the pin
	// path and the reader (docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md
	// section 4.5): files created per 1,000 pins without SSH.
	Tape *TapeDiagnostics `json:"tape,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type EventCapabilityAuthorizer interface {
	Authorize(r *http.Request, computerID, requiredScope string) error
	AuthorizeRenewal(r *http.Request, computerID string) error
}

func (h *Handler) ConfigureComputerEvents(cas *ComputerEventCAS, artifacts *EventArtifactService, auth EventCapabilityAuthorizer) error {
	if h == nil || cas == nil || artifacts == nil || auth == nil {
		return fmt.Errorf("corpusd handler: complete computer event dependencies are required")
	}
	h.eventCAS = cas
	h.eventArtifacts = artifacts
	h.eventAuth = auth
	checkpoints, err := NewCheckpointAuthority(cas, h.service)
	if err != nil {
		return err
	}
	h.checkpointAuthority = checkpoints
	return nil
}

func (h *Handler) ConfigureSelfDevelopmentModes(modes *SelfDevelopmentModeCAS) error {
	if h == nil || modes == nil {
		return fmt.Errorf("corpusd handler: self-development mode authority is required")
	}
	h.selfDevelopmentModes = modes
	return nil
}

// ConfigureGuestBinding installs vmctl's ownership lookup for guest-scoped
// corpusd routes. Without it, tap-sourced requests fail closed.
func (h *Handler) ConfigureGuestBinding(lookup guestOwnershipLookup) error {
	if h == nil || lookup == nil {
		return fmt.Errorf("corpusd handler: guest ownership lookup is required")
	}
	h.guestLookup = lookup
	return nil
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	status := "ok"
	storeStatus := "ok"
	if h == nil || h.service == nil || h.service.Health(r.Context()) != nil {
		status = "degraded"
		storeStatus = "unreachable"
	}
	resp := healthResponse{
		Status:  status,
		Service: "corpusd",
		Store:   storeStatus,
		Build:   buildinfo.Snapshot("corpusd"),
	}
	if h != nil && h.service != nil && h.service.store != nil {
		snap := h.service.store.RetrievalSnapshot()
		resp.Retrieval = &snap
	}
	if h != nil && h.service != nil {
		tape := h.service.TapeDiagnostics()
		resp.Tape = &tape
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalPublishTexture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req PublishTextureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.PublishTexture(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: publish texture: %v", err)
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, ErrOpinionDailyLimit):
			status = http.StatusTooManyRequests
		case errors.Is(err, ErrOpinionListingOff), errors.Is(err, ErrOpinionModeConflict):
			status = http.StatusConflict
		}
		writeJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// HandleInternalOwnedPublication serves one owner's publication of one doc,
// ?owner_id=&doc_id=, whatever its mode and whether or not the front list
// shows it. The proxy names the owner from the caller's credential.
func (h *Handler) HandleInternalOwnedPublication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	docID := strings.TrimSpace(r.URL.Query().Get("doc_id"))
	if ownerID == "" || docID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and doc_id are required"})
		return
	}
	pub, err := h.service.OwnedPublication(r.Context(), ownerID, docID)
	if errors.Is(err, ErrPublicationNotFound) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "publication not found"})
		return
	}
	if err != nil {
		log.Printf("corpusd: owned publication: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read publication"})
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

func (h *Handler) HandleInternalResolvePublication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	resolve := h.service.GetPublicationBundleByRoute
	if r.URL.Query().Get("view") == "nav" {
		// The reader's poll for a newer version needs the nav alone.
		resolve = h.service.GetPublicationNavByRoute
	}
	bundle, err := resolve(r.Context(), routePath)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: resolve publication %s: %v", routePath, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve publication"})
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

// HandleInternalFrontStories serves the Autopaper front list: one row per
// story publication, newest first, paged by ?limit=&cursor=, optionally only
// one section's stories (?section=world|us_politics|business|tech_ai). It
// reads Store B alone (Cut 2 unit 2b, Reader W3a).
func (h *Handler) HandleInternalFrontStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	page, err := h.service.ListFrontStoriesInSection(r.Context(), limit, r.URL.Query().Get("cursor"), r.URL.Query().Get("section"))
	if err != nil {
		if errors.Is(err, ErrInvalidFrontSection) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid section"})
			return
		}
		if errors.Is(err, ErrInvalidFrontCursor) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid cursor"})
			return
		}
		log.Printf("corpusd: front stories: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list stories"})
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) HandleInternalExportPublication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	out, err := h.service.ExportPublicationByRoute(r.Context(), routePath, format)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: export publication %s: %v", routePath, err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) HandleInternalRetrievalSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	resp, err := h.service.SearchPublished(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		log.Printf("corpusd: retrieval search: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to search published retrieval sources"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleInternalPublishedSearch serves GET /internal/platform/published/search
// ?q=&limit=: the published Autopaper stories and opinion pieces whose words
// match, for research (a bound guest reaches it through the proxy's read-only
// carve-out). It reads Store B alone.
func (h *Handler) HandleInternalPublishedSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	resp, err := h.service.SearchPublishedPieces(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		log.Printf("corpusd: published search: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to search published pieces"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalPublicationProposal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	publicationID := publicationIDFromProposalPath(r.URL.Path)
	if publicationID == "" {
		http.NotFound(w, r)
		return
	}
	var req SubmitPublicationProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.PublicationID = publicationID
	resp, err := h.service.SubmitPublicationProposal(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: submit publication proposal: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandleInternalProposalDeliveryState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req UpdateProposalDeliveryStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.UpdateProposalDeliveryState(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: update proposal delivery: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalSyncTextureDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req SyncTextureDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.SyncTextureDocument(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: sync texture document: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalGetTextureDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	docID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents/")
	docID = strings.TrimSuffix(docID, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	doc, err := h.service.GetPlatformTextureDocument(r.Context(), docID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: get texture document %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get texture document"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) HandleInternalListTextureRevisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/revisions") {
		h.HandleInternalGetTextureDocument(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents/")
	docID := strings.TrimSuffix(path, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	revisions, err := h.service.ListPlatformTextureRevisions(r.Context(), docID)
	if err != nil {
		log.Printf("corpusd: list texture revisions %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list texture revisions"})
		return
	}
	writeJSON(w, http.StatusOK, PlatformTextureRevisionListResponse{Revisions: revisions})
}

func (h *Handler) HandleInternalGetTextureRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	revisionID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/revisions/")
	revisionID = strings.Trim(revisionID, "/")
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		http.NotFound(w, r)
		return
	}
	rev, err := h.service.GetPlatformTextureRevision(r.Context(), revisionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: get texture revision %s: %v", revisionID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get texture revision"})
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

// HandleArtifactGC runs one platform-artifacts reachability sweep on demand.
// POST {"mode":"dry-run"|"active", "grace", "max_deletes", "max_bytes"} —
// internal only. The request is clamped to the configured policy
// (SetArtifactGCPolicy): active only when the policy is active (409
// otherwise), grace never below the policy window, caps never above the
// policy caps, and the policy's namespace allowlist. Dry-run returns the
// full eligible report without deleting.
func (h *Handler) HandleArtifactGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req ArtifactGCConfig
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	cfg, err := h.service.clampArtifactGCRequest(req)
	if err != nil {
		writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	report, err := h.service.RunArtifactGC(r.Context(), cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// HandleTapeSweep starts (POST) or reads (GET ?job=<id>, latest when empty)
// a tape sweep job: dry-run by default; apply names a dry-run's run_id and
// manifest_sha256 and needs PLATFORM_TAPE_SWEEP_MODE=active
// (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md). The run
// is asynchronous: a classification outlasts the HTTP write timeout.
func (h *Handler) HandleTapeSweep(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		job, ok := h.service.TapeSweepJobStatus(strings.TrimSpace(r.URL.Query().Get("job")))
		if !ok {
			writeJSON(w, http.StatusNotFound, apiError{Error: "tape sweep job not found"})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case http.MethodPost:
		var req TapeSweepRequest
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid tape sweep request: " + err.Error()})
				return
			}
		}
		job, err := h.service.StartTapeSweep(req)
		switch {
		case errors.Is(err, errTapeSweepBusy), errors.Is(err, errTapeSweepActiveNotConfigured):
			writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		case err != nil:
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		default:
			writeJSON(w, http.StatusAccepted, job)
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/computers/credentials/exchange", h.HandleComputerCredentialExchange)
	s.HandleFunc("/internal/computers/credentials/consume", h.HandleComputerCredentialConsume)
	s.HandleFunc("/internal/computers/credentials/issue", h.HandleComputerCredentialIssue)
	s.HandleFunc("/internal/computers/credentials/renew", h.HandleComputerCredentialRenew)
	s.HandleFunc("/internal/computers/lifecycle/control", h.HandleComputerLifecycleControl)
	s.HandleFunc("/internal/computers/self-development/mode", h.HandleSelfDevelopmentMode)
	s.HandleFunc("/internal/computers/self-development/consensus", h.HandleSelfDevelopmentConsensus)
	s.HandleFunc("/internal/computers/events/head", h.HandleComputerEventHead)
	s.HandleFunc("/internal/computers/events/pin", h.HandleComputerEventPin)
	s.HandleFunc("/internal/computers/events/append", h.HandleComputerEventAppend)
	s.HandleFunc("/internal/computers/events/payload", h.HandleComputerEventPayload)
	s.HandleFunc("/internal/computers/events/replay", h.HandleComputerEventReplay)
	s.HandleFunc("/internal/computers/checkpoints", h.HandleComputerCheckpoint)
	s.HandleFunc("/internal/computers/route-projection-certificates", h.HandleRouteProjectionCertificate)
	s.HandleFunc("/internal/computers/platform-updates/offer", h.HandlePlatformUpdateOfferMint)
	s.HandleFunc("/internal/computers/platform-updates/blob/", h.HandlePlatformUpdateBlob)
	s.HandleFunc(hostbuild.Route, h.HandleHostBuilds)
	s.HandleFunc(hostbuild.Route+"/", h.HandleHostBuilds)
	s.HandleFunc(releasesRoute, h.HandleReleases)
	s.HandleFunc(releasesRoute+"/", h.HandleReleases)
	s.HandleFunc("/internal/computers/keys/escrow-public-key", h.HandleKeyEscrowPublicKey)
	s.HandleFunc("/internal/computers/keys/escrow", h.HandleKeyEscrow)
	s.HandleFunc("/internal/computers/keys/escrow/status", h.HandleKeyEscrowStatus)
	s.HandleFunc("/internal/computers/keys/realization-delivery", h.HandleRealizationKeyDelivery)
	s.HandleFunc("/internal/computers/keys/unwrap-requests", h.HandleKeyUnwrapRequests)
	s.HandleFunc("/internal/computers/keys/unwrap-requests/", h.HandleKeyUnwrapRequestAction)
	s.HandleFunc("/internal/computers/keys/transparency-head", h.HandleKeyEscrowTransparencyHead)
	s.HandleFunc("/internal/computers/files/chunks/", h.HandleFileCASChunk)
	s.HandleFunc("/internal/computers/files/root", h.HandleFileCASRoot)
	s.HandleFunc("/internal/computers/files/watermark", h.HandleFileCASWatermark)
	s.HandleFunc("/internal/computers/projection-base/jobs", h.HandleProjectionJobs)
	s.HandleFunc("/internal/computers/projection-base/pins", h.HandleProjectionBasePins)
	s.HandleFunc("/internal/computers/files/projection-base/descriptor", h.HandleProjectionBaseDescriptor)
	s.HandleFunc("/internal/computers/files/projection-base/blob", h.HandleProjectionBaseBlob)
	s.HandleFunc("/internal/computers/files/roots", h.HandleFileCASRoots)
	s.HandleFunc("/internal/platform/control-key", h.HandlePlatformControlPublicKey)
	s.HandleFunc("/internal/platform/execution-identity/attest", h.HandleExecutionIdentityAttestation)
	s.HandleFunc("/internal/platform/publications/texture", h.HandleInternalPublishTexture)
	s.HandleFunc(types.StoryVerdictsPath, h.HandleSupportVerdicts)
	s.HandleFunc(types.StoryRelationsPath, h.HandleStoryRelations)
	s.HandleFunc("/internal/platform/publications/resolve", h.HandleInternalResolvePublication)
	s.HandleFunc("/internal/platform/publications/export", h.HandleInternalExportPublication)
	s.HandleFunc("/internal/platform/publications/front", h.HandleInternalFrontStories)
	s.HandleFunc("/internal/platform/publications/owned", h.HandleInternalOwnedPublication)
	s.HandleFunc("/internal/platform/publications/expansion-source", h.HandleInternalExpansionSource)
	s.HandleFunc("/internal/platform/publications/related", h.HandleInternalRelatedStories)
	s.HandleFunc("/internal/platform/publications/related-backfill", h.HandleInternalRelatedBackfill)
	s.HandleFunc("/internal/platform/retrieval/search", h.HandleInternalRetrievalSearch)
	s.HandleFunc("/internal/platform/published/search", h.HandleInternalPublishedSearch)
	s.HandleFunc("/internal/platform/proposal-deliveries/state", h.HandleInternalProposalDeliveryState)
	s.HandleFunc("/internal/platform/publications/", h.HandleInternalPublicationProposal)
	s.HandleFunc("/internal/platform/texture/sync", h.HandleInternalSyncTextureDocument)
	s.HandleFunc("/internal/platform/texture/revisions/", h.HandleInternalGetTextureRevision)
	s.HandleFunc("/internal/platform/texture/documents/", h.HandleInternalListTextureRevisions)
	s.HandleFunc("/internal/platform/artifact-gc", h.HandleArtifactGC)
	s.HandleFunc("/internal/platform/artifact-gc/tape", h.HandleTapeSweep)
	s.HandleFunc("/internal/platform/artifact-gc/tape/extra-refs", h.HandleTapeExtraRefs)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("corpusd: json encode: %v", err)
	}
}

func publicationIDFromProposalPath(path string) string {
	const prefix = "/internal/platform/publications/"
	const suffix = "/proposals"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	return strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix), "/")
}
