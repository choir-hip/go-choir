package platform

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// The takes service's internal routes (docs/reference/AUTORADIO-TAKES.md).
// The proxy names the caller from the credential and calls these with the
// host credential; corpusd accepts nobody else.
const (
	TakesPath        = "/internal/platform/takes"
	TakesAudioPath   = "/internal/platform/takes/audio"
	TakesDeletePath  = "/internal/platform/takes/delete"
	TakesRecordPath  = "/internal/platform/takes/record"
	TakesOutcomePath = "/internal/platform/takes/outcome"
	TakesYankPath    = "/internal/platform/operator/takes/yank"

	takesMaxBody = 1 << 20
)

func takeFail(w http.ResponseWriter, what string, err error) {
	switch {
	case errors.Is(err, ErrTakeInvalid):
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
	case errors.Is(err, ErrTakeDailyLimit):
		writeJSON(w, http.StatusTooManyRequests, apiError{Error: err.Error()})
	case errors.Is(err, ErrTakeNotFound):
		writeJSON(w, http.StatusNotFound, apiError{Error: "take not found"})
	case errors.Is(err, ErrTakeGone), errors.Is(err, ErrTakeRemoved):
		writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
	case errors.Is(err, ErrTakeOwn):
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
	default:
		log.Printf("corpusd: takes %s: %v", what, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "takes service failed"})
	}
}

func decodeTakeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, takesMaxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.More() {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return false
	}
	return true
}

// HandleTakes serves POST (publish) and GET (list; with
// publisher_key, that publisher's own list) of TakesPath.
func (h *Handler) HandleTakes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if r.Method == http.MethodPost {
		var in TakeIn
		if !decodeTakeBody(w, r, &in) {
			return
		}
		take, err := h.service.PublishTake(r.Context(), in)
		if err != nil {
			takeFail(w, "publish", err)
			return
		}
		writeJSON(w, http.StatusCreated, take)
		return
	}
	if key := strings.TrimSpace(r.URL.Query().Get("publisher_key")); key != "" {
		resp, err := h.service.ListMyTakes(r.Context(), key)
		if err != nil {
			takeFail(w, "list mine", err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
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
	resp, err := h.service.ListTakes(r.Context(), limit, r.URL.Query().Get("include_drill") == "1")
	if err != nil {
		takeFail(w, "list", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleTakeAudio serves GET TakesAudioPath?take_id=: the bytes of a live take.
func (h *Handler) HandleTakeAudio(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodGet) {
		return
	}
	audio, err := h.service.TakeAudio(r.Context(), r.URL.Query().Get("take_id"))
	if err != nil {
		takeFail(w, "audio", err)
		return
	}
	if audio == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "take not found"})
		return
	}
	w.Header().Set("Content-Type", "audio/mp4")
	w.Header().Set("Content-Length", strconv.Itoa(len(audio)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

// HandleTakeDelete serves POST TakesDeletePath: a publisher deletes their own.
func (h *Handler) HandleTakeDelete(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodPost) {
		return
	}
	var in struct {
		TakeID       string `json:"take_id"`
		PublisherKey string `json:"publisher_key"`
	}
	if !decodeTakeBody(w, r, &in) {
		return
	}
	if err := h.service.DeleteTake(r.Context(), in.TakeID, in.PublisherKey); err != nil {
		takeFail(w, "delete", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"take_id": in.TakeID, "state": TakeStateDeleted})
}

// HandleTakeRecord serves GET TakesRecordPath?publisher=&as=.
func (h *Handler) HandleTakeRecord(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodGet) {
		return
	}
	rec, err := h.service.TakeRecord(r.Context(), r.URL.Query().Get("publisher"), r.URL.Query().Get("as"))
	if err != nil {
		takeFail(w, "record", err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// HandleTakeOutcome serves POST TakesOutcomePath.
func (h *Handler) HandleTakeOutcome(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodPost) {
		return
	}
	var in TakeOutcomeIn
	if !decodeTakeBody(w, r, &in) {
		return
	}
	stored, err := h.service.RecordTakeOutcome(r.Context(), in)
	if err != nil {
		takeFail(w, "outcome", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"take_id": in.TakeID, "stored": stored})
}

// HandleTakeYank serves POST TakesYankPath: the operator removes a take.
func (h *Handler) HandleTakeYank(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodPost) {
		return
	}
	var in struct {
		TakeID string `json:"take_id"`
	}
	if !decodeTakeBody(w, r, &in) {
		return
	}
	if err := h.service.YankTake(r.Context(), in.TakeID); err != nil {
		takeFail(w, "yank", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"take_id": in.TakeID, "state": TakeStateYanked})
}
