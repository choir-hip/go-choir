package proxy

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
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
//
// Publish is the owner's act from the originating computer. The proxy reads
// the owner's track head from corpusd, has the computer draft the release
// on it from its own tape and source, verifies the draft, and hands corpusd
// the patches and the record with the publisher it authenticated and an
// owner-act receipt binding the release and the scope. No guest credential
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
}

type packageDraftBody struct {
	ReleaseID         string          `json:"release_id"`
	Record            json.RawMessage `json:"record"`
	TreePatchBase64   string          `json:"tree_patch_base64"`
	ReviewPatchBase64 string          `json:"review_patch_base64"`
	ParentPatchBase64 string          `json:"parent_patch_base64"`
}

// HandleComputerPackagePublish publishes one applied operation's package
// change of the caller's own computer at scope owner.
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
	publish, _ := json.Marshal(map[string]any{
		"release_id": releaseID, "record": draft.Record,
		"owner_id": authResult.UserID, "computer_id": computerID,
	})
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
// the read scope. The proxy names the owner it authenticated on every read;
// a caller-supplied owner is never forwarded, and the admin scope passes the
// scope check and nothing more (L4).
func (h *Handler) HandleCatalog(w http.ResponseWriter, r *http.Request) {
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
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/catalog"), "/"), "/")
	var upstream string
	switch {
	case len(parts) == 1 && parts[0] == "packages":
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			owner.Set("q", q)
		}
		upstream = "/internal/platform/packages/catalog?" + owner.Encode()
	case len(parts) == 3 && parts[0] == "packages" && parts[2] == "releases" && objectgraph.ValidPackageName(parts[1]):
		upstream = "/internal/platform/packages/catalog/" + parts[1] + "/releases?" + owner.Encode()
	case len(parts) == 2 && parts[0] == "releases" && computerevent.IsSHA256(strings.ToLower(parts[1])):
		upstream = "/internal/platform/packages/releases/" + strings.ToLower(parts[1]) + "?" + owner.Encode()
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, upstream, "", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}
