package proxy

import (
	_ "embed"
	"net/http"
	"strconv"
)

// phoneModelListPath is where the phone app fetches the list of on-device
// models: each model's job, where its files come from (repository, pinned
// revision, each file's size and SHA-256), its licence, and which devices it
// is recommended for. Public and unauthenticated on purpose: voice setup runs
// before sign-in, and the list names only public model files. The phone
// verifies every file against the digests, keeps the copy built into the app
// when this route fails, and drops any entry that breaks its rules
// (mobile/AutoradioKit/Sources/AutoradioKit/Voice/ModelList.swift).
const phoneModelListPath = "/api/phone/model-list"

// phoneModelListBody is the served list: the app's built-in list written out
// by the phone-side test ModelListTests.testServedListIsTheBuiltInList run
// with AR_WRITE_MODEL_LIST=1 (Mac only), which also writes
// phone_model_list.sources.json, the digests of the list and of the Swift
// files it is built from. TestPhoneModelListSourcesUnchanged (CI) fails when
// the list was edited by hand or one of those files changed since, so a
// change to the app's model tables cannot land without regenerating. The
// two are equal at every landing; they are not equal by construction.
//
// BEFORE THE PHONE FETCHES THIS LIST (it does not today), all three must be
// built, because the digests in the list are integrity, not authenticity:
// they arrive in the same document, so a host that is compromised, or a bad
// deploy, could serve a self-consistent list naming any public repository.
//  1. The list is signed with a key built into the app: Ed25519 over the
//     canonical bytes; the phone verifies before any use and keeps the
//     built-in list on any failure.
//  2. The list carries a version that only rises, and the phone refuses one
//     older than the highest it has accepted, so a replay cannot undo a
//     withdrawal.
//  3. The allowed effect is stated and enforced on the phone: whether a
//     served list may only add entries, or may also change which built-in
//     model is recommended (today it would do the second implicitly).
//
// This path is a platform route on the public host, ahead of the
// per-computer /api/ routes: a computer's own app cannot serve this path.
//
//go:embed phone_model_list.json
var phoneModelListBody []byte

// HandlePhoneModelList serves the list at the exact path, ahead of the /api/
// catch-all, so no sign-in check or computer route can wrap it.
func (h *Handler) HandlePhoneModelList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(phoneModelListBody)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(phoneModelListBody)
	}
}
