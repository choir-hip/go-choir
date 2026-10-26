package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// Releases at the proxy (P7, docs/reports/p6-p8-release-adopt-2026-10-15.md).
//
//	POST /api/computers/<id>/releases   publish an applied operation   owner act
//	GET  /api/releases                  list published releases (the caller's, under owner reads)   read
//	GET  /api/releases/<release id>     one published release (the caller's, under owner reads)    read
//
// Publish is the owner's act from the originating computer: the proxy reads
// the record the computer drafts from its own tape and bundle, verifies it,
// and hands corpusd the patch and the record with the publisher the proxy
// itself authenticated. No guest credential reaches the write.

const releaseReadScope = "computer:self_development:read"

// releasePublishScope is the route scope of the two release acts (publish,
// adopt). The owner-act gate is checked on top of it.
const releasePublishScope = "computer:self_development:approve"

func computerReleasePublishID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/releases"
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

func isComputerReleasePublishPath(path string) bool {
	_, ok := computerReleasePublishID(path)
	return ok
}

func isReleasesPath(path string) bool {
	return path == "/api/releases" || strings.HasPrefix(path, "/api/releases/")
}

type releasePublishBody struct {
	OperationID string `json:"operation_id"`
	TestCommand string `json:"test_command,omitempty"`
}

type releaseDraftBody struct {
	ReleaseID   string          `json:"release_id"`
	Record      json.RawMessage `json:"record"`
	PatchSHA256 string          `json:"patch_sha256"`
	PatchBytes  int             `json:"patch_bytes"`
	PatchBase64 string          `json:"patch_base64"`
}

// HandleComputerReleasePublish publishes one applied operation of the
// caller's own computer as a release record.
func (h *Handler) HandleComputerReleasePublish(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerReleasePublishID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" && !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, releasePublishScope) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + releasePublishScope})
		return
	}
	if !authorizeOwnerAct(w, authResult, "release publish") {
		return
	}
	var body releasePublishBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.OperationID) == "" || strings.ContainsAny(body.OperationID, "/?#") {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "operation_id is required"})
		return
	}
	guestURL, ok := h.ownedComputerURL(w, r, authResult, computerID)
	if !ok {
		return
	}

	// The computer drafts the record from its own tape and bundle.
	draftPath := "/api/computers/" + url.PathEscape(computerID) + "/self-development/operations/" + url.PathEscape(strings.TrimSpace(body.OperationID)) + "/release-draft"
	if cmd := strings.TrimSpace(body.TestCommand); cmd != "" {
		draftPath += "?test_command=" + url.QueryEscape(cmd)
	}
	status, raw, err := h.guestJSON(r.Context(), guestURL, http.MethodGet, draftPath, authResult, computerID, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "release draft unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	var draft releaseDraftBody
	if err := json.Unmarshal(raw, &draft); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed release draft"})
		return
	}
	record, err := release.Decode(draft.Record)
	if err == nil {
		err = release.VerifyID(record, draft.ReleaseID)
	}
	patch, decodeErr := base64.StdEncoding.DecodeString(draft.PatchBase64)
	if err == nil && decodeErr != nil {
		err = decodeErr
	}
	if err == nil {
		err = release.VerifyPatch(record, patch)
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's release draft does not verify: " + err.Error()})
		return
	}
	if record.Publisher.OwnerID != authResult.UserID || record.Publisher.ComputerID != computerID {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's release draft names another publisher"})
		return
	}

	// The patch, then the record, with the publisher the proxy authorized.
	sha := record.Source.PatchSHA256
	if status, raw, err := h.corpusdCall(r.Context(), http.MethodPut, "/internal/platform/releases/blobs/"+sha, "application/octet-stream", patch); err != nil || (status != http.StatusCreated && status != http.StatusOK) {
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "release store unavailable"})
			return
		}
		relayJSON(w, status, raw)
		return
	}
	publish, _ := json.Marshal(map[string]any{
		"release_id": strings.ToLower(strings.TrimSpace(draft.ReleaseID)), "record": draft.Record,
		"owner_id": authResult.UserID, "computer_id": computerID,
	})
	status, raw, err = h.corpusdCall(r.Context(), http.MethodPost, "/internal/platform/releases", "application/json", publish)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "release store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// HandleReleases serves the release list and one release to an authenticated
// caller with the read scope. The caller's owner rides every read to corpusd,
// which scopes it to that owner unless CHOIR_RELEASE_READS=open (Library S1; owner is the default).
func (h *Handler) HandleReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	// The admin scope passes this scope check and nothing more: the owner
	// below is always the key's own user, so private means private even to
	// admin (Library S1; TestAdminKeyReadsOnlyItsOwnersReleases).
	if authResult.AuthMethod == "api_key" && !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, releaseReadScope) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + releaseReadScope})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/releases"), "/")
	var upstream string
	switch {
	case rest == "":
		// The proxy names the owner it authenticated; corpusd scopes the list
		// to that owner (the default; CHOIR_RELEASE_READS=open lifts it). A caller-supplied
		// owner is never forwarded.
		query := url.Values{"owner_id": {authResult.UserID}}
		if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
			query.Set("limit", limit)
		}
		if publisher := strings.TrimSpace(r.URL.Query().Get("publisher_computer_id")); publisher != "" {
			query.Set("publisher_computer_id", publisher)
		}
		upstream = "/internal/platform/releases?" + query.Encode()
	case computerevent.IsSHA256(strings.ToLower(rest)):
		// corpusd binds every single-release read to a computer, and the
		// proxy has no guest's capability: it is the host-internal caller
		// (computer_id=host) carrying the authenticated owner, so corpusd
		// can answer 404 for a release that owner did not publish.
		upstream = "/internal/platform/releases/" + strings.ToLower(rest) + "?" + url.Values{"computer_id": {"host"}, "owner_id": {authResult.UserID}}.Encode()
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "release id must be a sha256"})
		return
	}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, upstream, "", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "release store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// ownedComputerURL resolves the caller's own active computer for the publish
// and package acts; they are owner acts and never wake it.
func (h *Handler) ownedComputerURL(w http.ResponseWriter, r *http.Request, authResult *AuthResult, computerID string) (string, bool) {
	var target *resolvedComputerTarget
	var err error
	if authResult.AuthMethod == "api_key" {
		var ok bool
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return "", false
		}
		guestURL, err := h.resolveActiveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
		if err != nil {
			logComputerAuthorityFailure(r, computerID, target, err)
			writeAutoputerResolveFailure(w, http.StatusBadGateway, "computer authority unavailable", err)
			return "", false
		}
		return guestURL, true
	}
	target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
	if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
		return "", false
	}
	if target.State != "active" || strings.TrimSpace(target.ComputerURL) == "" {
		logComputerAuthorityFailure(r, computerID, target, nil)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return "", false
	}
	return target.ComputerURL, true
}

func (h *Handler) guestJSON(ctx context.Context, guestURL, method, path string, authResult *AuthResult, computerID string, body []byte) (int, []byte, error) {
	return h.guestJSONLimit(ctx, guestURL, method, path, authResult, computerID, body, 4<<20)
}

// guestJSONLimit is guestJSON reading at most limit bytes of the answer.
func (h *Handler) guestJSONLimit(ctx context.Context, guestURL, method, path string, authResult *AuthResult, computerID string, body []byte, limit int64) (int, []byte, error) {
	target, err := joinBasePath(guestURL, strings.SplitN(path, "?", 2)[0])
	if err != nil {
		return 0, nil, err
	}
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 {
		target += "?" + parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("X-Authenticated-User", authResult.UserID)
	request.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		request.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		request.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.autoputerHTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit))
	return response.StatusCode, raw, err
}

func (h *Handler) corpusdCall(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error) {
	return h.corpusdCallHeaders(ctx, method, path, contentType, nil, body)
}

// corpusdCallHeaders is corpusdCall with extra request headers (the
// owner-act receipt).
func (h *Handler) corpusdCallHeaders(ctx context.Context, method, path, contentType string, headers map[string]string, body []byte) (int, []byte, error) {
	base, err := joinBasePath(h.cfg.CorpusdURL, strings.SplitN(path, "?", 2)[0])
	if err != nil {
		return 0, nil, err
	}
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 && parts[1] != "" {
		base += "?" + parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, method, base, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	hostauth.MarkInternal(request)
	client := h.corpusd
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("call corpusd: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	return response.StatusCode, raw, err
}

func relayJSON(w http.ResponseWriter, status int, raw []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
