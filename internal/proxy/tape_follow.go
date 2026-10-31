package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// GET /api/computers/<id>/tape/bundle?branch=main[&have_seq=<n>&have_head=<sha256>][&max_events=<n>]
//
// The owner's follower (the Mac app, choir-bundle follow) pulls the next
// bundle of its own computer's tape: a bootstrap without have_seq, then
// deltas after the head it holds (corpusd tape_follow.go has the shape and
// the status codes). The body is a tar stream passed through unbuffered.
//
// Authority: the owner's own session, or an API key with admin or
// computer:lifecycle scope whose binding (if any) is this computer. Either
// way the caller is joined to the computer's owner in vmctl, the same join
// every computer route uses; another owner's computer is refused 403. The
// join is a lookup only: unlike requireAPIKeyComputerTarget it does not
// touch the computer's activity, so a follower polling every half minute
// never keeps a computer from hibernating, and the export reads only host
// state, so it never wakes one.
const tapeFollowPathSuffix = "/tape/bundle"

func tapeFollowComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, tapeFollowPathSuffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), tapeFollowPathSuffix)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	id, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) || strings.ContainsAny(id, `/\`) {
		return "", false
	}
	return id, true
}

func isTapeFollowPath(path string) bool {
	_, ok := tapeFollowComputerID(path)
	return ok
}

// tapeFollowHTTP streams: no whole-request timeout, a bound on the wait for
// corpusd's headers (an export of one page) instead.
var tapeFollowHTTP = &http.Client{Transport: &http.Transport{
	Proxy:                 nil,
	DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	ResponseHeaderTimeout: 5 * time.Minute,
	MaxIdleConns:          4,
	IdleConnTimeout:       30 * time.Second,
}}

var tapeFollowQueryKeys = []string{"branch", "have_seq", "have_head", "max_events"}

func (h *Handler) HandleTapeFollow(w http.ResponseWriter, r *http.Request) {
	computerID, ok := tapeFollowComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil || authResult == nil || strings.TrimSpace(authResult.UserID) == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	switch authResult.AuthMethod {
	case "cookie":
	case "api_key":
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, "computer:lifecycle") {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing computer:lifecycle scope"})
			return
		}
		if bound := strings.TrimSpace(authResult.ComputerID); bound != "" && bound != computerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
			return
		}
	default:
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return
	}
	ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, computerID)
	if err != nil {
		if errors.Is(err, vmctl.ErrComputerLookupIdentityMismatch) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
		} else {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		}
		return
	}
	if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != computerID ||
		(ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
		return
	}

	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/tape/bundle")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build tape follow request"})
		return
	}
	q := url.Values{"computer_id": {computerID}}
	for _, key := range tapeFollowQueryKeys {
		if v := r.URL.Query().Get(key); v != "" {
			q.Set(key, v)
		}
	}
	// The edge (Caddy) bounds the whole request at 15 minutes; so does this.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(15 * time.Minute))
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target+"?"+q.Encode(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build tape follow request"})
		return
	}
	hostauth.MarkInternal(upstream)
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	response, err := tapeFollowHTTP.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "tape follow authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	for _, name := range []string{"Content-Type", "Cache-Control", "Retry-After", "X-Choir-Source-Head-Seq", "X-Choir-Bundle-Start-Seq", "X-Choir-Bundle-Head-Seq", "X-Choir-Bundle-Head-Digest"} {
		if v := response.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.WriteHeader(response.StatusCode)
	if response.StatusCode == http.StatusOK {
		if _, err := io.Copy(w, response.Body); err != nil {
			// The status is sent: cutting the stream is the signal.
			panic(http.ErrAbortHandler)
		}
		return
	}
	_, _ = io.Copy(w, io.LimitReader(response.Body, 64<<10))
}
