package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Takes (docs/reference/AUTORADIO-TAKES.md): a listener's recorded voice.
//
//	POST   /api/platform/takes                  publish one take (multipart), signed in
//	GET    /api/platform/takes?limit=           the newest live takes, signed in
//	GET    /api/platform/takes?mine=1           the caller's own live takes with their outcomes
//	GET    /api/platform/takes/record?publisher=  a publisher's record (rows only for the publisher)
//	GET    /api/platform/takes/{id}/audio       the .m4a bytes
//	DELETE /api/platform/takes/{id}             the publisher deletes their own take
//	POST   /api/platform/takes/{id}/outcome     a listener's computer reports heard or skipped
//	POST   /api/platform/operator/takes/yank    the operator removes a take
//
// The proxy names the caller from the credential; the publisher and listener
// keys are derived here and never taken from a request body. corpusd takes
// only the host credential.

const (
	takesPublicPath  = "/api/platform/takes"
	takesPathPrefix  = "/api/platform/takes/"
	takesYankPath    = "/api/platform/operator/takes/yank"
	takesMaxUpload   = 700 << 10
	takesMaxFormMem  = 1 << 20
	takesMaxRespond  = 2048
	takesAudioMaxOut = 1 << 20
)

func isTakesPath(path string) bool {
	return path == takesPublicPath || strings.HasPrefix(path, takesPathPrefix)
}

// takesCaller authenticates the caller of a takes route. It returns the
// account and, for an API key, the computer the key acts for (an owner-wide
// key must name one, as on every route). A cookie session has no computer.
func (h *Handler) takesCaller(w http.ResponseWriter, r *http.Request) (*AuthResult, string, bool) {
	authResult, err := h.authenticate(r)
	if err != nil || authResult == nil || strings.TrimSpace(authResult.UserID) == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return nil, "", false
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return nil, "", false
	}
	if authResult.AuthMethod != "api_key" {
		return authResult, "", true
	}
	target, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", requestDesktopID(r))
	if !ok {
		return nil, "", false
	}
	return authResult, target.ComputerID, true
}

// HandlePlatformTakes dispatches the takes routes.
func (h *Handler) HandlePlatformTakes(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	path := r.URL.Path
	switch {
	case path == takesPublicPath && r.Method == http.MethodPost:
		h.takePublish(w, r)
	case path == takesPublicPath && r.Method == http.MethodGet:
		h.takeList(w, r)
	case path == takesPublicPath+"/record" && r.Method == http.MethodGet:
		h.takeRecord(w, r)
	case strings.HasPrefix(path, takesPathPrefix) && strings.Count(strings.TrimPrefix(path, takesPathPrefix), "/") <= 1:
		rest := strings.TrimPrefix(path, takesPathPrefix)
		id, sub, _ := strings.Cut(rest, "/")
		switch {
		case !types.TakeIDRe.MatchString(id):
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		case sub == "audio" && r.Method == http.MethodGet:
			h.takeAudio(w, r, id)
		case sub == "" && r.Method == http.MethodDelete:
			h.takeDelete(w, r, id)
		case sub == "outcome" && r.Method == http.MethodPost:
			h.takeOutcome(w, r, id)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	}
	h.lifecycle.record("platform_takes.total", "done", time.Since(started))
}

func (h *Handler) takeCorpusd(w http.ResponseWriter, r *http.Request, method, path string, body any) {
	var raw []byte
	contentType := ""
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "takes request failed"})
			return
		}
		contentType = "application/json"
	}
	status, resp, err := h.corpusdCall(r.Context(), method, path, contentType, raw)
	if err != nil {
		log.Printf("proxy: takes %s %s: %v", method, path, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "takes service unavailable"})
		return
	}
	relayJSON(w, status, resp)
}

func (h *Handler) takePublish(w http.ResponseWriter, r *http.Request) {
	authResult, computerID, ok := h.takesCaller(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, takesMaxUpload)
	if err := r.ParseMultipartForm(takesMaxFormMem); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "a take is multipart: audio, transcript, duration_seconds, sha256 and optional responds_to, at most 700 KB"})
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "audio is required"})
		return
	}
	defer func() { _ = file.Close() }()
	audio, err := io.ReadAll(io.LimitReader(file, platform.TakeMaxBytes+1))
	if err != nil || len(audio) == 0 || len(audio) > platform.TakeMaxBytes {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "audio must be 1 to 524288 bytes"})
		return
	}
	for name := range r.MultipartForm.Value {
		switch name {
		case "transcript", "duration_seconds", "sha256", "responds_to":
		default:
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown field " + name})
			return
		}
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("duration_seconds")), 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "duration_seconds is required"})
		return
	}
	sum := sha256.Sum256(audio)
	if claimed := strings.TrimSpace(r.FormValue("sha256")); claimed != "" && claimed != hex.EncodeToString(sum[:]) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the audio does not match its sha256"})
		return
	}
	in := platform.TakeIn{
		PublisherKey: types.TakePublisherKey(authResult.UserID), Publisher: types.TakePublisherHandle(authResult.UserID),
		DurationSeconds: duration, SHA256: hex.EncodeToString(sum[:]), AudioBase64: base64.StdEncoding.EncodeToString(audio),
		Transcript: r.FormValue("transcript"),
		// A take published by a drill computer's credential is drill data:
		// the proxy decides, never the request.
		Drill: computerID != "" && platform.TakeDrillComputer(computerID),
	}
	if raw := strings.TrimSpace(r.FormValue("responds_to")); raw != "" {
		var resp platform.TakeRespondsTo
		dec := json.NewDecoder(io.LimitReader(strings.NewReader(raw), takesMaxRespond))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&resp); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "responds_to is {route, title}"})
			return
		}
		in.RespondsTo = &resp
	}
	h.takeCorpusd(w, r, http.MethodPost, platform.TakesPath, in)
}

func (h *Handler) takeList(w http.ResponseWriter, r *http.Request) {
	authResult, computerID, ok := h.takesCaller(w, r)
	if !ok {
		return
	}
	q := url.Values{}
	if r.URL.Query().Get("mine") != "" {
		q.Set("publisher_key", types.TakePublisherKey(authResult.UserID))
	} else {
		if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
			if n, err := strconv.Atoi(limit); err != nil || n < 0 {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid limit"})
				return
			}
			q.Set("limit", limit)
		}
		if computerID != "" && platform.TakeDrillComputer(computerID) {
			q.Set("include_drill", "1")
		}
	}
	h.takeCorpusd(w, r, http.MethodGet, platform.TakesPath+"?"+q.Encode(), nil)
}

func (h *Handler) takeRecord(w http.ResponseWriter, r *http.Request) {
	authResult, _, ok := h.takesCaller(w, r)
	if !ok {
		return
	}
	publisher := r.URL.Query().Get("publisher")
	if !types.TakePublisherRe.MatchString(publisher) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "publisher is a voice- handle"})
		return
	}
	q := url.Values{"publisher": {publisher}, "as": {types.TakePublisherKey(authResult.UserID)}}
	h.takeCorpusd(w, r, http.MethodGet, platform.TakesRecordPath+"?"+q.Encode(), nil)
}

func (h *Handler) takeAudio(w http.ResponseWriter, r *http.Request, id string) {
	if _, _, ok := h.takesCaller(w, r); !ok {
		return
	}
	status, ctype, raw, err := h.corpusdFetch(r.Context(), platform.TakesAudioPath+"?take_id="+url.QueryEscape(id), takesAudioMaxOut)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "takes service unavailable"})
		return
	}
	if status != http.StatusOK {
		relayJSON(w, status, raw)
		return
	}
	if !strings.HasPrefix(ctype, "audio/mp4") {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "takes service answered something that is not audio"})
		return
	}
	w.Header().Set("Content-Type", "audio/mp4")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = bytes.NewReader(raw).WriteTo(w)
}

func (h *Handler) takeDelete(w http.ResponseWriter, r *http.Request, id string) {
	authResult, _, ok := h.takesCaller(w, r)
	if !ok {
		return
	}
	h.takeCorpusd(w, r, http.MethodPost, platform.TakesDeletePath, map[string]string{"take_id": id, "publisher_key": types.TakePublisherKey(authResult.UserID)})
}

// takeOutcome is a listener's act: it takes an API key (a computer's own
// credential, or a device key naming the computer), never a browser session,
// because the outcome must come from the listener's own sheet op on their
// computer (rule 1 of the learning engine).
func (h *Handler) takeOutcome(w http.ResponseWriter, r *http.Request, id string) {
	authResult, computerID, ok := h.takesCaller(w, r)
	if !ok {
		return
	}
	if authResult.AuthMethod != "api_key" || computerID == "" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "an outcome is reported by the listener's computer, under its own credential"})
		return
	}
	var body struct {
		Outcome string `json:"outcome"`
		ClaimID string `json:"claim_id"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<12))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the body is {outcome, claim_id}"})
		return
	}
	h.takeCorpusd(w, r, http.MethodPost, platform.TakesOutcomePath, platform.TakeOutcomeIn{
		TakeID: id, ListenerUserKey: types.TakePublisherKey(authResult.UserID), ListenerComputer: computerID,
		Outcome: body.Outcome, ClaimID: body.ClaimID,
	})
}

// HandlePlatformOperatorTakeYank serves the operator's removal of a take.
func (h *Handler) HandlePlatformOperatorTakeYank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, ok := h.authorizePlatformOperator(w, r)
	if !ok {
		return
	}
	var body struct {
		TakeID string `json:"take_id"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<12))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !types.TakeIDRe.MatchString(body.TakeID) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the body is {take_id}"})
		return
	}
	log.Printf("proxy: platform operator take yank %s by account %s via %s", body.TakeID, authResult.UserID, authResult.AuthMethod)
	h.takeCorpusd(w, r, http.MethodPost, platform.TakesYankPath, map[string]string{"take_id": body.TakeID})
}
