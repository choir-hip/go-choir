package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

// The Choir Library at the proxy (S2,
// docs/design/library-package-propagation-2026-10-26.md 4.1 and 14).
//
//	POST /api/computers/<id>/packages/releases   publish one applied operation's package   owner act
//	GET  /api/catalog/packages[?q=]               the packages the caller may see           read
//	GET  /api/catalog/packages/<name>/releases    a package's history                       read
//	GET  /api/catalog/releases/<release id>       one package release and its listing       read
//	GET  /api/catalog/releases/<id>/media/<sha>    one screenshot or video of a release      read
//	GET  /api/catalog/releases/<id>/blobs/<sha>    one patch the release record names        read
//	PATCH /api/catalog/records/<release id>/listing  withdraw or reinstate                owner act
//
// A published release is public: every signed-in owner sees it in the
// catalog and may install it (owner, 2026-11-03). There is no share; a
// share answers share_retired. Publish is the owner's act from the
// originating computer. The proxy reads
// the owner's track head from corpusd, has the computer draft the release
// on it from its own tape and source, verifies the draft, and hands corpusd
// the patches and the record with the publisher it authenticated, the
// owner's confirmation of the paths that become public, and an owner-act
// receipt binding the release and its public scope. No guest credential
// and no desk reaches the write: a desk's product API never POSTs a
// computer route. Every read names the authenticated owner and only it.

// maxPackageDraftBytes bounds the guest's draft: three patches of up to
// 2 MiB each in base64, and the record.
const maxPackageDraftBytes = 12 << 20

func computerPackagePublishID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/packages/releases"
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

func isComputerPackagePublishPath(path string) bool {
	_, ok := computerPackagePublishID(path)
	return ok
}

func isCatalogPath(path string) bool {
	return path == "/api/catalog" || strings.HasPrefix(path, "/api/catalog/")
}

type packagePublishBody struct {
	OperationID string `json:"operation_id"`
	Package     string `json:"package"`
	TestCommand string `json:"test_command,omitempty"`
	// ConfirmPaths is the paths digest of corpusd's preview of what
	// becomes public; without it corpusd answers the preview.
	ConfirmPaths string `json:"confirm_paths,omitempty"`
}

type packageDraftBody struct {
	ReleaseID         string          `json:"release_id"`
	Record            json.RawMessage `json:"record"`
	TreePatchBase64   string          `json:"tree_patch_base64"`
	ReviewPatchBase64 string          `json:"review_patch_base64"`
	ParentPatchBase64 string          `json:"parent_patch_base64"`
}

// HandleComputerPackagePublish publishes one applied operation's package
// change of the caller's own computer. Publishing makes it public.
func (h *Handler) HandleComputerPackagePublish(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerPackagePublishID(r.URL.Path)
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
	if !authorizeOwnerAct(w, authResult, "package publish") {
		return
	}
	var body packagePublishBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.OperationID) == "" || strings.ContainsAny(body.OperationID, "/?#") {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "operation_id and package are required"})
		return
	}
	name := strings.TrimSpace(body.Package)
	if !objectgraph.ValidPackageName(name) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "package must be a package name"})
		return
	}
	guestURL, ok := h.ownedComputerURL(w, r, authResult, computerID)
	if !ok {
		return
	}

	// The owner's track head, read for the owner the proxy authenticated.
	draftQuery := url.Values{"package": {name}}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, "/internal/platform/packages/tracks/"+url.PathEscape(name)+"/head?"+url.Values{"owner_id": {authResult.UserID}}.Encode(), "", nil)
	switch {
	case err != nil:
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	case status == http.StatusOK:
		var head platform.PackageTrackHead
		if err := json.Unmarshal(raw, &head); err != nil || !computerevent.IsSHA256(head.ReleaseID) {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed track head"})
			return
		}
		draftQuery.Set("head_release_id", head.ReleaseID)
		draftQuery.Set("head_tree", head.Tree)
	case status != http.StatusNotFound:
		relayJSON(w, status, raw)
		return
	}
	if cmd := strings.TrimSpace(body.TestCommand); cmd != "" {
		draftQuery.Set("test_command", cmd)
	}

	// The computer drafts the release from its own tape and source.
	draftPath := "/api/computers/" + url.PathEscape(computerID) + "/packages/operations/" + url.PathEscape(strings.TrimSpace(body.OperationID)) + "/release-draft?" + draftQuery.Encode()
	status, raw, err = h.guestJSONLimit(r.Context(), guestURL, http.MethodGet, draftPath, authResult, computerID, nil, maxPackageDraftBytes)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package draft unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	var draft packageDraftBody
	if err := json.Unmarshal(raw, &draft); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed package draft"})
		return
	}
	record, patches, err := verifyPackageDraft(draft)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's package draft does not verify: " + err.Error()})
		return
	}
	if record.Publisher != (pkgrelease.Publisher{OwnerID: authResult.UserID, ComputerID: computerID}) || record.Track != authResult.UserID || record.Package.Name != name {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's package draft names another publisher, track or package"})
		return
	}
	// The owner's act names one operation; a draft of another (or of an
	// operation the computer made up) is not what the owner published. The
	// request carries no bundle digest, so the operation id is the binding.
	if record.Operation.OperationID != strings.TrimSpace(body.OperationID) {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's package draft is of operation " + record.Operation.OperationID + ", not " + strings.TrimSpace(body.OperationID)})
		return
	}

	// A release is listed only with a screenshot; refuse before the owner's
	// publish stores anything.
	if err := pkgrelease.RequireScreenshot(record); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "reason": pkgrelease.ReasonScreenshotRequired})
		return
	}

	// The release's screenshots and video: each fetched from the
	// publisher computer, which answers only the items its bound frozen
	// bundle carries, and verified against the record before any is stored
	// (docs/problems/library-releases-have-no-screenshots-2026-11-04.md).
	media := make([][]byte, 0, len(record.Media))
	for _, item := range record.Media {
		mediaPath := "/api/computers/" + url.PathEscape(computerID) + "/packages/operations/" + url.PathEscape(record.Operation.OperationID) + "/media/" + item.SHA256
		status, raw, err := h.guestJSONLimit(r.Context(), guestURL, http.MethodGet, mediaPath, authResult, computerID, nil, pkgrelease.MaxMediaBytes+1)
		if err != nil || status != http.StatusOK {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer did not answer the release's media item " + item.Name})
			return
		}
		if err := pkgrelease.VerifyMedia(item, raw); err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's media item does not verify: " + err.Error()})
			return
		}
		media = append(media, raw)
	}
	for _, raw := range media {
		status, body, err := h.corpusdCall(r.Context(), http.MethodPut, "/internal/platform/packages/media/"+computerevent.DigestBytes(raw), "application/octet-stream", raw)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
			return
		}
		if status != http.StatusCreated && status != http.StatusOK {
			relayJSON(w, status, body)
			return
		}
	}

	// The patches, then the record, with the publisher the proxy authorized.
	for _, patch := range patches {
		status, raw, err := h.corpusdCall(r.Context(), http.MethodPut, "/internal/platform/packages/blobs/"+computerevent.DigestBytes(patch), "application/octet-stream", patch)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
			return
		}
		if status != http.StatusCreated && status != http.StatusOK {
			relayJSON(w, status, raw)
			return
		}
	}
	releaseID := strings.ToLower(strings.TrimSpace(draft.ReleaseID))
	headers := map[string]string{}
	if h.ownerActSigner != nil {
		method := owneract.AuthMethodSession
		if authResult.AuthMethod == "api_key" {
			method = owneract.AuthMethodAPIKey
		}
		receipt, err := h.ownerActSigner.Mint(owneract.Act{
			Kind: owneract.ActPackagePublish, OwnerUserID: authResult.UserID, AuthMethod: method,
			CredentialRef: authResult.CredentialRef, ComputerID: computerID, Subject: platform.PackagePublishSubject(releaseID),
		}, time.Now())
		if err == nil {
			headers[owneract.Header], err = owneract.Encode(receipt)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "owner act receipt unavailable"})
			return
		}
	}
	request := map[string]any{
		"release_id": releaseID, "record": draft.Record,
		"owner_id": authResult.UserID, "computer_id": computerID,
	}
	if confirm := strings.TrimSpace(body.ConfirmPaths); confirm != "" {
		request["confirm_paths"] = confirm
	}
	publish, _ := json.Marshal(request)
	status, raw, err = h.corpusdCallHeaders(r.Context(), http.MethodPost, "/internal/platform/packages/releases", "application/json", headers, publish)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// verifyPackageDraft checks a guest's draft: the record hashes to its id and
// every patch to the record. It returns the patches to upload.
func verifyPackageDraft(draft packageDraftBody) (pkgrelease.Record, [][]byte, error) {
	record, err := pkgrelease.Decode(draft.Record)
	if err == nil {
		err = pkgrelease.VerifyID(record, draft.ReleaseID)
	}
	if err != nil {
		return pkgrelease.Record{}, nil, err
	}
	decode := func(in string, blob pkgrelease.Blob) ([]byte, error) {
		raw, err := base64.StdEncoding.DecodeString(in)
		if err != nil {
			return nil, err
		}
		return raw, pkgrelease.VerifyBlob(blob, raw)
	}
	tree, err := decode(draft.TreePatchBase64, record.Source.TreePatch)
	if err != nil {
		return pkgrelease.Record{}, nil, err
	}
	review, err := decode(draft.ReviewPatchBase64, record.Source.ReviewPatch)
	if err != nil {
		return pkgrelease.Record{}, nil, err
	}
	patches := [][]byte{tree, review}
	switch {
	case record.ParentPatch != nil:
		parent, err := decode(draft.ParentPatchBase64, *record.ParentPatch)
		if err != nil {
			return pkgrelease.Record{}, nil, err
		}
		patches = append(patches, parent)
	case draft.ParentPatchBase64 != "":
		return pkgrelease.Record{}, nil, &pkgrelease.Refusal{Reason: pkgrelease.ReasonInvalid, Detail: "a parent_patch for a record that names none"}
	}
	return record, patches, nil
}

// HandleCatalog serves the package catalog to an authenticated caller with
// the read scope, and the listing acts (PATCH .../records/<id>/listing). The
// proxy names the owner it authenticated on every read; a caller-supplied
// owner is never forwarded, and the admin scope passes the scope check and
// nothing more (L4).
func (h *Handler) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/catalog"), "/"), "/")
	if len(parts) == 3 && parts[0] == "records" && parts[2] == "listing" && r.Method == http.MethodPatch {
		h.handlePackageListingAct(w, r, strings.ToLower(parts[1]))
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" && !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, releaseReadScope) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + releaseReadScope})
		return
	}
	owner := url.Values{"owner_id": {authResult.UserID}}
	var upstream string
	switch {
	case len(parts) == 1 && parts[0] == "packages":
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			owner.Set("q", q)
		}
		upstream = "/internal/platform/packages/catalog?" + owner.Encode()
	case len(parts) == 3 && parts[0] == "packages" && parts[2] == "releases" && objectgraph.ValidPackageName(parts[1]):
		// One track's history: the caller's own unless track names
		// another, a page at a time (S3).
		for _, key := range []string{"track", "limit", "before_seq"} {
			if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
				owner.Set(key, v)
			}
		}
		upstream = "/internal/platform/packages/catalog/" + parts[1] + "/releases?" + owner.Encode()
	case len(parts) == 2 && parts[0] == "releases" && computerevent.IsSHA256(strings.ToLower(parts[1])):
		upstream = "/internal/platform/packages/releases/" + strings.ToLower(parts[1]) + "?" + owner.Encode()
	case len(parts) == 4 && parts[0] == "releases" && parts[2] == "media" && computerevent.IsSHA256(strings.ToLower(parts[1])) && computerevent.IsSHA256(strings.ToLower(parts[3])):
		h.relayPackageMedia(w, r, "/internal/platform/packages/releases/"+strings.ToLower(parts[1])+"/media/"+strings.ToLower(parts[3])+"?"+owner.Encode())
		return
	case len(parts) == 4 && parts[0] == "releases" && parts[2] == "blobs" && computerevent.IsSHA256(strings.ToLower(parts[1])) && computerevent.IsSHA256(strings.ToLower(parts[3])):
		h.relayPackageBlob(w, r, "/internal/platform/packages/releases/"+strings.ToLower(parts[1])+"/blobs/"+strings.ToLower(parts[3])+"?"+owner.Encode())
		return
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, upstream, "", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	if status == http.StatusOK {
		raw = h.decoratePublisherEmail(raw, authResult.UserID)
	}
	relayJSON(w, status, raw)
}

// packageListingBody is the owner's listing act: a state (withdraw,
// reinstate) and the owner's computer the act is made from. Scopes,
// shared_with and confirm_paths are what a retired share sent; any of them
// answers share_retired.
type packageListingBody struct {
	ComputerID   string                 `json:"computer_id"`
	Scopes       []string               `json:"scopes"`
	SharedWith   *pkgrelease.SharedWith `json:"shared_with"`
	State        string                 `json:"state"`
	Reason       string                 `json:"reason"`
	ConfirmPaths string                 `json:"confirm_paths"`
}

// handlePackageListingAct is withdraw and reinstate (design 4.2, 14). It is
// the owner's act: the proxy checks the named computer is the owner's, then
// hands corpusd the change for the owner it authenticated with an owner-act
// receipt binding the release and the change. corpusd decides the rest from
// the release's own listing: a release of another owner answers as missing.
// Share is retired (owner, 2026-11-03): a published release is public.
func (h *Handler) handlePackageListingAct(w http.ResponseWriter, r *http.Request, releaseID string) {
	if !computerevent.IsSHA256(releaseID) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
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
	if !authorizeOwnerAct(w, authResult, "package listing act") {
		return
	}
	var body packageListingBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid listing act"})
		return
	}
	if len(body.Scopes) > 0 || body.SharedWith != nil || strings.TrimSpace(body.ConfirmPaths) != "" {
		writeJSON(w, http.StatusGone, map[string]string{"error": pkgrelease.ReasonShareRetired + ": " + pkgrelease.ShareRetiredDetail, "reason": pkgrelease.ReasonShareRetired})
		return
	}
	computerID := strings.TrimSpace(body.ComputerID)
	if computerID == "" || strings.ContainsAny(computerID, "/?#") || strings.TrimSpace(body.State) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "computer_id and state (withdraw, reinstate) are required"})
		return
	}
	if _, ok := h.ownedComputerURL(w, r, authResult, computerID); !ok {
		return
	}
	change := platform.ListingChange{State: strings.TrimSpace(body.State), Reason: strings.TrimSpace(body.Reason)}
	headers := map[string]string{}
	if h.ownerActSigner != nil {
		method := owneract.AuthMethodSession
		if authResult.AuthMethod == "api_key" {
			method = owneract.AuthMethodAPIKey
		}
		receipt, err := h.ownerActSigner.Mint(owneract.Act{
			Kind: owneract.ActPackageListing, OwnerUserID: authResult.UserID, AuthMethod: method,
			CredentialRef: authResult.CredentialRef, ComputerID: computerID, Subject: platform.PackageListingSubject(releaseID, change),
		}, time.Now())
		if err == nil {
			headers[owneract.Header], err = owneract.Encode(receipt)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "owner act receipt unavailable"})
			return
		}
	}
	act, _ := json.Marshal(platform.PackageListingRequest{OwnerID: authResult.UserID, ComputerID: computerID, ListingChange: change})
	status, raw, err := h.corpusdCallHeaders(r.Context(), http.MethodPut, "/internal/platform/listings/"+releaseID, "application/json", headers, act)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// relayPackageBlob serves one patch a release record names to a signed-in
// owner (choir package export rebuilds the release tree from it). corpusd
// decides visibility for the owner the proxy authenticated and serves only
// a digest the record names. The proxy relays only octet-stream within the
// patch cap, and checks the bytes hash to the digest it asked for, so a
// wrong answer is never relayed as the blob.
func (h *Handler) relayPackageBlob(w http.ResponseWriter, r *http.Request, upstream string) {
	status, contentType, raw, err := h.corpusdFetch(r.Context(), upstream, pkgrelease.MaxBlobBytes+1)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	want := upstream[strings.LastIndex(upstream, "/blobs/")+len("/blobs/"):]
	want, _, _ = strings.Cut(want, "?")
	if contentType != "application/octet-stream" || len(raw) > pkgrelease.MaxBlobBytes || computerevent.DigestBytes(raw) != want {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the package store answered a blob that is not the one asked for"})
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// relayPackageMedia serves one release media item to a signed-in owner:
// public like the release (corpusd decides visibility for the owner the
// proxy authenticated). Only the two media types are ever relayed, with
// nosniff, so an answer can never be served as a page.
func (h *Handler) relayPackageMedia(w http.ResponseWriter, r *http.Request, upstream string) {
	status, contentType, raw, err := h.corpusdFetch(r.Context(), upstream, pkgrelease.MaxMediaBytes+1)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	if (contentType != pkgrelease.MediaTypePNG && contentType != pkgrelease.MediaTypeWebM) || len(raw) > pkgrelease.MaxMediaBytes {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the package store answered a media item that is not one"})
		return
	}
	// Revalidated on every use: a withdrawn release's media turns private,
	// and a cached copy must not outlive that. Visibility was just decided
	// upstream; the ETag is the digest, so a revalidation of bytes the
	// browser holds answers 304 with no body.
	etag := `"` + computerevent.DigestBytes(raw) + `"`
	w.Header().Set("Cache-Control", "private, no-cache, must-revalidate")
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	// ServeContent answers Range (206, Content-Range), If-Range and
	// If-None-Match (304); Safari's <video> will not play without ranges.
	// The zero modtime keeps Last-Modified out of the answer.
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(raw))
}
