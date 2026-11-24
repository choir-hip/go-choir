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

// phoneModelListBody is the served list. It is the app's built-in list
// written out; the phone-side test ModelListTests.testServedListIsTheBuiltInList
// fails when the two differ and rewrites this file when run with
// AR_WRITE_MODEL_LIST=1. Change the list by changing the app's tables and
// regenerating, or by editing this file to offer phones in the field a model
// their build does not carry (that test then names the difference, and the
// build's tables are brought to match).
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
