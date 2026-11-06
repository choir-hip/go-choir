package proxy

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

// The platform operator's acts from the operator's own account
// (docs/design/platform-operator-account-2026-11-06.md).
//
//	POST /api/platform/operator/packages/yank   yank a package release, or force its installers onto another release
//
// The act is made from a signed-in session only: an API key, even one
// minted with owner:acts, is a bearer string on a disk and never carries
// the platform operator role. The proxy checks the body's shape, mints an
// owner-act receipt for the session's account with the platform scope
// (owneract.PlatformComputerID) binding the exact request, and hands
// corpusd the request. corpusd alone decides whether the account holds the
// role (CHOIR_PLATFORM_OPERATOR_USERS) and records the receipt before it
// writes; the proxy holds no copy of the roster.
const platformOperatorYankPath = "/api/platform/operator/packages/yank"

// HandlePlatformOperatorYank serves the operator account's yank.
func (h *Handler) HandlePlatformOperatorYank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod != "cookie" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "platform operator act: needs the operator's signed-in session; no API key carries the role"})
		return
	}
	// A passkey session only: an email-code or email-link session is as
	// strong as the inbox, and the role acts on every computer.
	if authResult.SignInMethod != "passkey" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "Sign in with a passkey to do this."})
		return
	}
	if h.ownerActSigner == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "owner act receipt unavailable"})
		return
	}
	var req platform.PlatformYankRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid yank request"})
		return
	}
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	subject := platform.PlatformYankSubject(req)
	receipt, err := h.ownerActSigner.Mint(owneract.Act{
		Kind: owneract.ActPlatformYank, OwnerUserID: authResult.UserID, AuthMethod: owneract.AuthMethodSession,
		CredentialRef: authResult.CredentialRef, ComputerID: owneract.PlatformComputerID, Subject: subject,
	}, time.Now())
	headers := map[string]string{}
	if err == nil {
		headers[owneract.Header], err = owneract.Encode(receipt)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "owner act receipt unavailable"})
		return
	}
	// The body corpusd reads is the normalized request the receipt binds.
	body, _ := json.Marshal(platform.PlatformYankRequest{
		ReleaseID: subject["release_id"].(string), Reason: req.Reason, ForcedRelease: subject["forced_release"].(string),
	})
	status, raw, err := h.corpusdCallHeaders(r.Context(), http.MethodPost, "/internal/platform/operator/packages/yank", "application/json", headers, body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "package store unavailable"})
		return
	}
	relayJSON(w, status, raw)
}
