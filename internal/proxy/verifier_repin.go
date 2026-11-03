package proxy

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The owner's verifier re-pin
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md):
// POST /api/computers/<id>/verifier-key/repin {"reason": "..."}.
//
// The proxy authenticates the owner (owner:acts), reads the key the
// computer's verifier presents from the guest, asks corpusd's preflight
// whether another key is pinned, and only then signs an owner-act receipt
// binding the presented key, the pin it replaces and the reason. corpusd
// re-pins under that receipt alone; the guest is then handed corpusd's pin
// receipt to record on its tape and to lift its hold. The caller never names
// the key: the guest presents it, the preflight confirms it, the owner
// authorizes it. The guest cannot start this: it is not the proxy, holds no
// owner-act key, and corpusd refuses a re-pin from anything but a
// host-internal caller.

const maxVerifierRepinReason = 500

// verifierRepinOldRelease is the answer for a computer whose guest release
// predates the verifier-key route (it answers 404).
const verifierRepinOldRelease = "this computer's release predates the verifier re-pin (its guest has no verifier-key route), so the key it presents cannot be read. " +
	"Update it first: abandon the held operation (choir self-dev abandon --computer X --operation OP --reason R) so the computer is idle, let the deploy refresh (or choir computer refresh) boot the current release, " +
	"then run choir computer repin-verifier again and retry the change."

func computerVerifierRepinID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/verifier-key/repin"
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

func isComputerVerifierRepinPath(path string) bool {
	_, ok := computerVerifierRepinID(path)
	return ok
}

// HandleComputerVerifierRepin is the owner's re-pin of a computer's verifier key.
func (h *Handler) HandleComputerVerifierRepin(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerVerifierRepinID(r.URL.Path)
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
	if !authorizeOwnerAct(w, authResult, "verifier re-pin") {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Reason) == "" || len(strings.TrimSpace(body.Reason)) > maxVerifierRepinReason {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "reason is required (at most 500 characters) and is the only field"})
		return
	}
	if h.ownerActSigner == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "owner act receipt unavailable: the proxy has no owner-act key"})
		return
	}
	guestURL, ok := h.ownedComputerURL(w, r, authResult, computerID)
	if !ok {
		return
	}
	base := "/api/computers/" + url.PathEscape(computerID) + "/self-development/verifier-key"
	status, raw, err := h.guestJSON(r.Context(), guestURL, http.MethodGet, base, authResult, computerID, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer's verifier key is unavailable"})
		return
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		// A guest release from before the re-pin cannot present its key.
		// No compatibility path: the owner updates the computer first.
		writeJSON(w, http.StatusConflict, errorResponse{Error: verifierRepinOldRelease})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	var presented struct {
		ComputerID string `json:"computer_id"`
		KeyID      string `json:"key_id"`
		PublicKey  string `json:"public_key"`
	}
	if err := json.Unmarshal(raw, &presented); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed verifier key answer"})
		return
	}
	if presented.ComputerID != computerID {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the verifier key answer names another computer"})
		return
	}
	if _, err := selfdevprotocol.DecodeVerifierKey(presented.KeyID, presented.PublicKey); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the computer presents an invalid verifier key: " + err.Error()})
		return
	}
	preflight, _ := json.Marshal(selfdevprotocol.VerifierKeyPreflightRequest{ComputerID: computerID, KeyID: presented.KeyID, PublicKey: presented.PublicKey})
	status, raw, err = h.corpusdCall(r.Context(), http.MethodPost, "/internal/computers/verifier-key/preflight", "application/json", preflight)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "checkpoint authority unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	var pin selfdevprotocol.VerifierKeyPin
	if err := json.Unmarshal(raw, &pin); err != nil || pin.ComputerID != computerID {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed preflight answer"})
		return
	}
	switch pin.State {
	case selfdevprotocol.VerifierKeyMatch:
		writeJSON(w, http.StatusOK, map[string]any{"status": selfdevprotocol.VerifierKeyAlreadyPinned, "pin": pin})
		return
	case selfdevprotocol.VerifierKeyMismatch:
	default:
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nothing to re-pin: the host's pin state for the presented key is " + pin.State, "pin": pin})
		return
	}
	request := selfdevprotocol.VerifierKeyRepinRequest{
		ComputerID: computerID, OwnerID: authResult.UserID, KeyID: presented.KeyID, PublicKey: presented.PublicKey,
		ReplacesKeyID: pin.PinnedKeyID, Reason: strings.TrimSpace(body.Reason),
	}
	method := owneract.AuthMethodSession
	if authResult.AuthMethod == "api_key" {
		method = owneract.AuthMethodAPIKey
	}
	receipt, err := h.ownerActSigner.Mint(owneract.Act{
		Kind: owneract.ActVerifierRepin, OwnerUserID: authResult.UserID, AuthMethod: method,
		CredentialRef: authResult.CredentialRef, ComputerID: computerID, Subject: selfdevprotocol.VerifierKeyRepinSubject(request),
	}, time.Now())
	headers := map[string]string{}
	if err == nil {
		headers[owneract.Header], err = owneract.Encode(receipt)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "owner act receipt unavailable"})
		return
	}
	repinBody, _ := json.Marshal(request)
	status, raw, err = h.corpusdCallHeaders(r.Context(), http.MethodPost, "/internal/computers/verifier-key/repin", "application/json", headers, repinBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "checkpoint authority unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	var repinned selfdevprotocol.VerifierKeyRepinResponse
	if err := json.Unmarshal(raw, &repinned); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "malformed re-pin answer"})
		return
	}
	// The pin has changed. The guest records it on its tape and lifts its
	// hold; a failure here is reported, and the guest's next preflight sees
	// the new pin anyway.
	answer := map[string]any{"status": repinned.Status, "pin": repinned.Pin, "replaced_key_id": request.ReplacesKeyID}
	record, _ := json.Marshal(map[string]any{"pin_receipt": repinned.PinReceipt})
	recordStatus, recordRaw, recordErr := h.guestJSON(r.Context(), guestURL, http.MethodPost, base+"/repinned", authResult, computerID, record)
	switch {
	case recordErr != nil:
		answer["tape_record_error"] = recordErr.Error()
	case recordStatus != http.StatusOK:
		answer["tape_record_error"] = strings.TrimSpace(string(recordRaw))
	default:
		answer["tape_record"] = json.RawMessage(recordRaw)
	}
	writeJSON(w, http.StatusOK, answer)
}
