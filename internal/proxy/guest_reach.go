// Guest-reach decisions of this package, moved here whole from handlers.go so the
// security-epoch gate (nix/guest-security-surfaces.txt) sees a change to them. A
// change to this file changes what a guest computer may reach on the host.
// Keep it to those decisions; put anything else in another file.

package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandlePlatformObjectGraph forwards object graph API requests from the
// runtime (inside a VM) to corpusd. Guest requests are bound by their tap
// source address to the owning computer before the proxy becomes corpusd's
// loopback internal caller.
func (h *Handler) HandlePlatformObjectGraph(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformObjectGraphCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: err.Error()})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd URL"})
		return
	}
	rawQuery, relayBody := r.URL.RawQuery, io.Reader(r.Body)
	if isGuestFeedPath(r.URL.Path) && !trustedInternalProxyTransport(r) {
		var scopeErr error
		if rawQuery, relayBody, scopeErr = h.scopeGuestFeedRequest(r); scopeErr != nil {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: scopeErr.Error()})
			return
		}
	}
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, relayBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd request"})
		return
	}
	// The proxy is the trusted internal caller to corpusd. Strip any
	// caller-supplied X-Internal-Caller header and set it ourselves so a
	// VM process cannot forge internal-caller privileges.
	hostauth.MarkInternal(httpReq)
	if v := r.Header.Get("Content-Type"); v != "" {
		httpReq.Header.Set("Content-Type", v)
	}

	resp, err := h.corpusd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: corpusd objectgraph: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to reach corpusd"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy: corpusd objectgraph copy: %v", err)
	}
}

// RegisterRoutes registers all proxy routes on the given server.
// The proxy /health handler is registered via SetHealthHandler to
// override the default server health handler with one that reports
// upstream autoputer reachability.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc(appleAppSiteAssociationPath, h.HandleAppleAppSiteAssociation)
	s.HandleFunc("/api/shell/bootstrap", h.HandleBootstrap)
	s.HandleFunc("/api/ws", h.HandleWS)
	s.HandleFunc("/api/", h.HandleAPI)
	// VAL-GATEWAY-002: Deny all browser access to /provider/* routes.
	// The gateway is the only component authorized to call upstream
	// providers; browser callers must never bypass the runtime/proxy
	// boundary to invoke inference directly.
	s.HandleFunc("/provider/", h.HandleProviderDeny)
	// VAL-VM-012: Deny all browser access to /internal/vmctl/* routes.
	// vmctl control endpoints are internal-only; they must not be
	// exposed as public browser-facing routes.
	s.HandleFunc("/internal/vmctl/", h.HandleVMctlDeny)
	// Object graph API: forward runtime objectgraph HTTPStore calls to
	// corpusd. The runtime (inside a VM) reaches corpusd through the
	// proxy; these routes pass through /internal/platform/objects and
	// /internal/platform/edges to corpusd's object graph handler.
	s.HandleFunc("/internal/platform/objects", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/objects/", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/edges", h.HandlePlatformObjectGraph)
	// Autopaper captures (W1b2): a guest writes one poll's captures and reads
	// shared capture bodies through the same bound-guest pass-through.
	s.HandleFunc("/internal/platform/captures", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/captures/bodies/", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/captures/", h.HandlePlatformObjectGraph)
	// Research's read-only search over published stories and opinion pieces
	// (guest_authority.go carves out this one GET path).
	s.HandleFunc("/internal/platform/published/search", h.HandlePlatformObjectGraph)
	// The Autoradio desk's ask of the paper: the same carve-out, one more path.
	s.HandleFunc("/internal/platform/published/ask", h.HandlePlatformObjectGraph)
	// A computer's polls of its op-ed and notice feeds, and the paper's
	// verdict post (the proxy scopes each to the owner it bound).
	s.HandleFunc("/internal/platform/opeds", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/notices", h.HandlePlatformObjectGraph)
	s.HandleFunc(opedVerdictsPathProxy, h.HandlePlatformObjectGraph)
	// One published version by id, for Texture's take, merge and fork
	// (guest_authority.go carves out this one GET path shape).
	s.HandleFunc("/internal/platform/publications/versions/", h.HandlePlatformObjectGraph)
	// Living stories (Cut 2 2b): a guest publishes its own story revisions
	// through the owner's publish path, and writes its stories' support
	// verdicts to Store B, both bound to the guest's owner and computer.
	s.HandleFunc(types.StoryPublishPath, h.HandleGuestTexturePublication)
	s.HandleFunc(types.StoryVerdictsPath, h.HandlePlatformObjectGraph)
	s.HandleFunc(types.StoryRelationsPath, h.HandlePlatformObjectGraph)
	s.HandleFunc(types.StoryEntitiesPath, h.HandlePlatformObjectGraph)
	s.HandleFunc(types.StoryEntityCandidatesPath, h.HandlePlatformObjectGraph)
	s.HandleFunc("/assets/", h.HandleComputerSurface)
	s.HandleFunc("/", h.HandleComputerSurface)
}

// HandleGuestTexturePublication serves POST types.StoryPublishPath: a
// computer publishing one revision of its own story (Cut 2 2b). The proxy
// binds the calling guest to its owner and computer and runs the owner's
// publish path for it, reading the document and revision from that
// computer; the guest names only the document and revision.
func (h *Handler) HandleGuestTexturePublication(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	ownership, err := h.boundProxyGuest(r)
	if err != nil || strings.TrimSpace(ownership.UserID) == "" || strings.TrimSpace(ownership.ComputerURL) == "" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "guest caller is not bound to a live computer"})
		h.lifecycle.record("platform_publish.guest", "forbidden", time.Since(started))
		return
	}
	var story types.StoryPublishRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&story); err != nil || strings.TrimSpace(story.DocID) == "" || strings.TrimSpace(story.RevisionID) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "doc_id and revision_id are required"})
		return
	}
	req := publishTextureRequest{DocID: strings.TrimSpace(story.DocID), RevisionID: strings.TrimSpace(story.RevisionID)}
	// A computer's own publish is an Autopaper story: publish_mode auto.
	h.publishTextureRevision(w, r, strings.TrimSpace(ownership.UserID), strings.TrimSpace(ownership.ComputerURL), req, platform.PublishModeAuto, started)
}
