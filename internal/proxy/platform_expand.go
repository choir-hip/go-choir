package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Story expansion requests (docs/reports/story-expansion-request-design-
// 2026-10-20.md, phase 1). An allowlisted signed-in owner asks one public
// Autopaper story to be expanded with more research. The proxy checks the
// allowlist, asks corpusd which paper document the route publishes, applies
// the limits, and calls the paper's research door, POST .../research, as the
// story's owner. The paper casts the story's research desk with the story
// itself; research reports to Texture as it goes, and Texture expands the
// story (docs/design/research-more-goes-to-research-2026-10-21.md). The
// reader sees a fate line. Management is not a hop.
//
// Dark by default: PROXY_EXPAND_REQUESTERS empty and PROXY_EXPAND_PUBLIC off
// means off, and PROXY_EXPAND=off is the kill switch.
//
// The public door (owner 2026-10-20, section 32 of the owner notes; problem
// doc "Public door"): with PROXY_EXPAND_PUBLIC=on a reader with no owner
// session may ask too. The paper still sees the owner's act, with the
// same request an owner's ask makes; nothing about the reader reaches the
// guest. A public reader is throttled per address on top of the
// global limits. The counters live in proxy memory, so a restart resets them
// (an accepted residual).

const (
	// expandInFlightMax and expandDailyMax are soft limits held in proxy
	// memory (the proxy has no state directory and restarts on every
	// deploy); the hard limit is one request per published version, which
	// lives in the paper's chain.
	expandInFlightMax = 3
	expandDailyMax    = 12
	// expandSilenceAfter is how long a request may go without a new public
	// version before the fate line says so.
	expandSilenceAfter = 30 * time.Minute
	// expandKeep bounds the in-memory ledger.
	expandKeep = 48 * time.Hour
	// expandShowRecent is how many of a story's requests a read returns.
	expandShowRecent = 5
	// Per-address limits for public readers. The address is auth.ClientIP
	// (the edge's last X-Forwarded-For entry, else the peer). Owners are not
	// counted. Held in proxy memory beside the ledger: a restart resets them.
	expandAddrHourMax = 3
	expandAddrDayMax  = 10
	expandAddrWindow  = time.Hour
)

// Request states a reader can see.
const (
	expandStateAsked    = "asked"
	expandStateAccepted = "accepted"
	expandStateRevised  = "revised"
	expandStateNoNew    = "no_new_version"
	expandStateUnknown  = "unknown"
	// expandStateInterrupted is a request a crash closed before research
	// reported; the reader may ask once more for the same version.
	expandStateInterrupted = "interrupted"
	expandStatusAsked      = "asked"
	expandStatusAlready    = "already_asked"
	expandStatusLimit      = "limit"
	expandStatusBusy       = "busy"
	expandStatusChanged    = "changed"
	expandStatusUnavail    = "unavailable"
	// expandStatusAddrLimit is the typed refusal for a public reader whose
	// address has used its allowance.
	expandStatusAddrLimit = "address_limit"
	expandAddrLimitMsg    = "too many requests from your address; try later"
	expandNotFoundBody    = "not found"
	expandAskedHeaderMsg  = "owner session required: asking a story to be expanded needs the owner's signed-in session, not an API key"
)

// expandRequest is one remembered request.
type expandRequest struct {
	RequestID        string
	Route            string
	PublicationID    string
	StoryOwnerID     string
	DocID            string
	SourceRevisionID string
	// PaperRequestID is the request's id at the paper (the client request
	// id, or its second attempt after an interrupted first); empty while the
	// request is being made.
	PaperRequestID string
	FromVersion    int
	AskedAt        time.Time
	// Addr is a hash of a public reader's address, for the per-address
	// throttle only; empty for an owner. It is never logged or sent on.
	Addr string
	// settled is set once a read found the story revised, which frees the
	// in-flight slot and stops the reads that go to the paper.
	settled bool
	// interrupted is set once a read found the request closed by a crash:
	// it frees the slot and lets the version be asked again.
	interrupted bool
}

// expandLedger is the proxy's soft memory of requests: the limits and the
// fate line. The paper's chain is the authority for "already asked".
type expandLedger struct {
	mu    sync.Mutex
	now   func() time.Time
	byKey map[string]*expandRequest
}

func newExpandLedger() *expandLedger {
	return &expandLedger{now: time.Now, byKey: map[string]*expandRequest{}}
}

func expandKey(publicationID, sourceRevisionID string) string {
	return publicationID + "\x00" + sourceRevisionID
}

// expandClientRequestID is the request identity (design 4.5): one request per
// published version of a story.
func expandClientRequestID(publicationID, sourceRevisionID string) string {
	return "expand:" + publicationID + ":" + sourceRevisionID
}

func expandRequestID(clientRequestID string) string {
	sum := sha256.Sum256([]byte(clientRequestID))
	return "rx-" + hex.EncodeToString(sum[:])[:12]
}

// expandSecondAttempt is the paper request id of the one re-ask a version
// gets after its first request was interrupted by a restart.
func expandSecondAttempt(clientRequestID string) string {
	return clientRequestID + ":2"
}

type expandVerdict int

const (
	expandOK expandVerdict = iota
	expandAlready
	expandInFlightFull
	expandDayFull
	expandAddrHourFull
	expandAddrDayFull
)

// reserve records a request about to be made. A second request for the same
// published version is refused with the first.
func (l *expandLedger) reserve(req *expandRequest) (*expandRequest, expandVerdict) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, old := range l.byKey {
		if now.Sub(old.AskedAt) > expandKeep {
			delete(l.byKey, key)
		}
	}
	key := expandKey(req.PublicationID, req.SourceRevisionID)
	if existing, ok := l.byKey[key]; ok && !existing.interrupted {
		return existing, expandAlready
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if req.Addr != "" {
		hour, day := 0, 0
		for _, old := range l.byKey {
			if old.Addr != req.Addr {
				continue
			}
			if now.Sub(old.AskedAt) < expandAddrWindow {
				hour++
			}
			if !old.AskedAt.Before(dayStart) {
				day++
			}
		}
		if hour >= expandAddrHourMax {
			return nil, expandAddrHourFull
		}
		if day >= expandAddrDayMax {
			return nil, expandAddrDayFull
		}
	}
	inFlight, today := 0, 0
	for _, old := range l.byKey {
		if !old.settled && !old.interrupted && now.Sub(old.AskedAt) < expandSilenceAfter {
			inFlight++
		}
		if !old.AskedAt.Before(dayStart) {
			today++
		}
	}
	if inFlight >= expandInFlightMax {
		return nil, expandInFlightFull
	}
	if today >= expandDailyMax {
		return nil, expandDayFull
	}
	req.AskedAt = now
	l.byKey[key] = req
	return req, expandOK
}

// addrRetryAfter is how long until a public reader's address may ask again,
// for the verdict that refused it.
func (l *expandLedger) addrRetryAfter(addr string, verdict expandVerdict) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if verdict == expandAddrHourFull {
		var oldest time.Time
		for _, old := range l.byKey {
			if old.Addr == addr && now.Sub(old.AskedAt) < expandAddrWindow && (oldest.IsZero() || old.AskedAt.Before(oldest)) {
				oldest = old.AskedAt
			}
		}
		if !oldest.IsZero() {
			return oldest.Add(expandAddrWindow).Sub(now)
		}
	}
	return nextUTCMidnight(now).Sub(now)
}

func nextUTCMidnight(now time.Time) time.Time {
	next := now.UTC().Add(24 * time.Hour)
	return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
}

// release forgets a request that never reached the paper's desk.
func (l *expandLedger) release(req *expandRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byKey[expandKey(req.PublicationID, req.SourceRevisionID)] == req {
		delete(l.byKey, expandKey(req.PublicationID, req.SourceRevisionID))
	}
}

func (l *expandLedger) confirm(req *expandRequest, paperRequestID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	req.PaperRequestID = paperRequestID
}

// forStory returns copies of a story's remembered requests, newest first.
func (l *expandLedger) forStory(publicationID string) []expandRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []expandRequest
	for _, req := range l.byKey {
		if req.PublicationID == publicationID {
			out = append(out, *req)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AskedAt.After(out[j].AskedAt) })
	if len(out) > expandShowRecent {
		out = out[:expandShowRecent]
	}
	return out
}

func (l *expandLedger) lookup(publicationID, sourceRevisionID string) (expandRequest, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if req, ok := l.byKey[expandKey(publicationID, sourceRevisionID)]; ok {
		return *req, true
	}
	return expandRequest{}, false
}

// expandLedger returns the handler's ledger, made on first use.
func (h *Handler) expandLedger() *expandLedger {
	h.expandMu.Lock()
	defer h.expandMu.Unlock()
	if h.expand == nil {
		h.expand = newExpandLedger()
	}
	return h.expand
}

// expandEnabled is the feature switch: dark when nobody is allowlisted and
// the public door is shut, or PROXY_EXPAND=off.
func (h *Handler) expandEnabled() bool {
	return h != nil && !h.expandOff && (len(h.expandRequesters) > 0 || h.expandPublic)
}

// expandOwnerSession authenticates an allowlisted browser session. An API key
// never qualifies: asking is an owner act that needs the signed-in session
// (compare isEmailOwnerActRoute).
func (h *Handler) expandOwnerSession(r *http.Request) (*AuthResult, int) {
	authResult, err := h.authenticate(r)
	if err != nil {
		return nil, http.StatusUnauthorized
	}
	if authResult.AuthMethod == "api_key" || !h.expandRequesters[strings.TrimSpace(authResult.UserID)] {
		return nil, http.StatusForbidden
	}
	return authResult, 0
}

// expandCaller is who is asking: an allowlisted owner session, or, with the
// public door open, any other reader. A public reader carries only a hash of
// the address the edge reports, for the per-address throttle; the session
// user id of a signed-in stranger is deliberately dropped.
type expandCaller struct {
	owner bool
	// requester is the log label: a hash of the owner id, or "public".
	requester string
	addr      string
}

// expandAskers resolves the caller. A nil caller carries the refusal status
// (401 when there is no session, 403 when the session may not ask).
func (h *Handler) expandAskers(r *http.Request) (*expandCaller, int) {
	authResult, refused := h.expandOwnerSession(r)
	if authResult != nil {
		return &expandCaller{owner: true, requester: hashedRequester(authResult.UserID)}, 0
	}
	if !h.expandPublic {
		return nil, refused
	}
	ip := strings.TrimSpace(auth.ClientIP(r))
	if ip == "" {
		ip = "unknown"
	}
	sum := sha256.Sum256([]byte("expand-addr\x00" + ip))
	return &expandCaller{requester: "public", addr: hex.EncodeToString(sum[:])[:16]}, 0
}

// expansionSource asks corpusd which paper document a public route publishes.
// notFound is true for anything that is not an expandable story.
func (h *Handler) expansionSource(r *http.Request, route string) (src platform.ExpansionSource, notFound bool, err error) {
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/expansion-source")
	if err != nil {
		return src, false, err
	}
	u, err := url.Parse(target)
	if err != nil {
		return src, false, err
	}
	q := u.Query()
	q.Set("route", route)
	u.RawQuery = q.Encode()
	if _, err := h.getPlatformJSON(r, u.String(), &src); err != nil {
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusNotFound {
			return src, true, nil
		}
		return src, false, err
	}
	if strings.TrimSpace(src.PublicationID) == "" || strings.TrimSpace(src.OwnerID) == "" || strings.TrimSpace(src.SourceDocID) == "" || strings.TrimSpace(src.SourceRevisionID) == "" {
		return src, false, fmt.Errorf("corpusd named an incomplete expansion source")
	}
	return src, false, nil
}

// paperCall is a host-to-guest call as the story's owner, trusted only from
// the host peer (the same trust as fetchAutoputerJSON). It returns the status
// and body of any HTTP answer; err is a transport failure.
func (h *Handler) paperCall(ctx context.Context, base, method, path, userID string, body any) (int, []byte, error) {
	target, err := joinBasePath(base, path)
	if err != nil {
		return 0, nil, err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal paper request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build paper request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Authenticated-User", userID)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("call paper: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read paper response: %w", err)
	}
	return resp.StatusCode, data, nil
}

func (h *Handler) paperURL(ctx context.Context, ownerID string) (string, error) {
	return h.resolveComputerURL(ctx, ownerID, vmctl.PrimaryDesktopID)
}

// paperDocument reads the story's document from the paper and checks it is
// the owner's own and a lifecycle document.
func (h *Handler) paperDocument(ctx context.Context, base string, src platform.ExpansionSource) (autoputerTextureDocument, error) {
	var doc autoputerTextureDocument
	status, body, err := h.paperCall(ctx, base, http.MethodGet, "/api/texture/documents/"+url.PathEscape(src.SourceDocID), src.OwnerID, nil)
	if err != nil {
		return doc, err
	}
	if status != http.StatusOK {
		return doc, fmt.Errorf("paper document read status %d", status)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return doc, fmt.Errorf("decode paper document: %w", err)
	}
	if doc.OwnerID != src.OwnerID || doc.DocID != src.SourceDocID {
		return doc, fmt.Errorf("paper document does not belong to the story's owner")
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" || strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return doc, fmt.Errorf("paper document has no lifecycle head")
	}
	return doc, nil
}

type expandPostRequest struct {
	Route string `json:"route"`
}

type expandAnswer struct {
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Route     string `json:"route,omitempty"`
	FromV     int    `json:"from_version,omitempty"`
}

// paperResearchState is the paper's read of one research request
// (GET /api/texture/documents/{doc}/research/{id}).
type paperResearchState struct {
	RecordID    string    `json:"record_id"`
	CommittedAt time.Time `json:"committed_at"`
	Open        bool      `json:"open"`
	Reports     int       `json:"reports"`
	Interrupted bool      `json:"interrupted"`
}

// paperResearch reads a research request; found is false for a 404.
func (h *Handler) paperResearch(ctx context.Context, base, ownerID, docID, paperRequestID string) (state paperResearchState, found bool, err error) {
	status, body, err := h.paperCall(ctx, base, http.MethodGet, "/api/texture/documents/"+url.PathEscape(docID)+"/research/"+url.PathEscape(paperRequestID), ownerID, nil)
	if err != nil {
		return state, false, err
	}
	switch status {
	case http.StatusNotFound:
		return state, false, nil
	case http.StatusOK:
		if err := json.Unmarshal(body, &state); err != nil {
			return state, false, fmt.Errorf("decode research request: %w", err)
		}
		return state, true, nil
	default:
		return state, false, fmt.Errorf("research request read status %d", status)
	}
}

// HandlePlatformPublicationExpand serves POST
// /api/platform/publications/expand {"route": "/pub/texture/<slug>"}.
func (h *Handler) HandlePlatformPublicationExpand(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	w.Header().Set("Cache-Control", "no-store")
	outcome := "error"
	requester, requestID, publicationID := "-", "-", "-"
	defer func() {
		h.lifecycle.record("platform_expand.request", outcome, time.Since(started))
		if outcome != "not_found_dark" && outcome != "unauthorized" {
			log.Printf("proxy: expand: request=%s publication=%s requester=%s outcome=%s", requestID, publicationID, requester, outcome)
		}
	}()

	if !h.expandEnabled() {
		outcome = "not_found_dark"
		writeJSON(w, http.StatusNotFound, errorResponse{Error: expandNotFoundBody})
		return
	}
	if r.Method != http.MethodPost {
		outcome = "bad_method"
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	caller, refused := h.expandAskers(r)
	if caller == nil {
		if refused == http.StatusUnauthorized {
			outcome = "unauthorized"
			writeJSON(w, refused, errorResponse{Error: "authentication required"})
			return
		}
		outcome = "forbidden"
		msg := "not allowed"
		if a, err := h.authenticate(r); err == nil && a.AuthMethod == "api_key" {
			msg = expandAskedHeaderMsg
		}
		writeJSON(w, refused, errorResponse{Error: msg})
		return
	}
	requester = caller.requester

	var in expandPostRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&in); err != nil || strings.TrimSpace(in.Route) == "" {
		outcome = "bad_request"
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "route is required"})
		return
	}
	src, notFound, err := h.expansionSource(r, strings.TrimSpace(in.Route))
	if notFound {
		outcome = "not_found"
		writeJSON(w, http.StatusNotFound, errorResponse{Error: expandNotFoundBody})
		return
	}
	if err != nil {
		log.Printf("proxy: expand: corpusd source: %v", err)
		outcome = "platform_error"
		writeJSON(w, http.StatusBadGateway, expandAnswer{Status: expandStatusUnavail, Error: "could not look up the story"})
		return
	}
	publicationID = src.PublicationID
	clientRequestID := expandClientRequestID(src.PublicationID, src.SourceRevisionID)
	requestID = expandRequestID(clientRequestID)

	led := h.expandLedger()
	entry := &expandRequest{
		RequestID: requestID, Route: src.Route, PublicationID: src.PublicationID, StoryOwnerID: src.OwnerID,
		DocID: src.SourceDocID, SourceRevisionID: src.SourceRevisionID, FromVersion: src.LatestRevision,
		Addr: caller.addr,
	}
	existing, verdict := led.reserve(entry)
	switch verdict {
	case expandAlready:
		outcome = "already_asked"
		writeJSON(w, http.StatusConflict, expandAnswer{Status: expandStatusAlready, Error: "this version of the story was already asked to be expanded", RequestID: existing.RequestID, Route: src.Route})
		return
	case expandAddrHourFull, expandAddrDayFull:
		outcome = "address_limit"
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(led.addrRetryAfter(caller.addr, verdict).Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, expandAnswer{Status: expandStatusAddrLimit, Error: expandAddrLimitMsg})
		return
	case expandInFlightFull, expandDayFull:
		outcome = "limit"
		retry := "1800"
		if verdict == expandDayFull {
			retry = fmt.Sprintf("%d", int(nextUTCMidnight(led.now()).Sub(led.now()).Seconds()))
		}
		w.Header().Set("Retry-After", retry)
		writeJSON(w, http.StatusTooManyRequests, expandAnswer{Status: expandStatusLimit, Error: "too many expansion requests right now; try again later"})
		return
	}
	// From here the slot is held until the paper's desk has the request.
	accepted := false
	defer func() {
		if !accepted {
			led.release(entry)
		}
	}()

	unavailable := func(why string, err error) {
		log.Printf("proxy: expand: %s: %v", why, err)
		outcome = "paper_unavailable"
		writeJSON(w, http.StatusBadGateway, expandAnswer{Status: expandStatusUnavail, Error: "the paper is not reachable right now; ask again shortly"})
	}
	base, err := h.paperURL(r.Context(), src.OwnerID)
	if err != nil {
		unavailable("resolve paper", err)
		return
	}
	for attempt := 0; ; attempt++ {
		doc, err := h.paperDocument(r.Context(), base, src)
		if err != nil {
			unavailable("read document", err)
			return
		}
		// The request may already be at the paper: a proxy restart forgets
		// the ledger, the paper does not. A first request a crash closed gets
		// one second attempt.
		paperID := clientRequestID
		state, found, err := h.paperResearch(r.Context(), base, src.OwnerID, src.SourceDocID, paperID)
		if err == nil && found && state.Interrupted {
			paperID = expandSecondAttempt(clientRequestID)
			state, found, err = h.paperResearch(r.Context(), base, src.OwnerID, src.SourceDocID, paperID)
		}
		if err != nil {
			unavailable("check request", err)
			return
		}
		if found {
			outcome = "already_asked"
			led.confirm(entry, paperID)
			accepted = true // the request exists; keep it remembered
			writeJSON(w, http.StatusConflict, expandAnswer{Status: expandStatusAlready, Error: "this version of the story was already asked to be expanded", RequestID: requestID, Route: src.Route})
			return
		}

		status, body, err := h.paperCall(r.Context(), base, http.MethodPost, "/api/texture/documents/"+url.PathEscape(src.SourceDocID)+"/research", src.OwnerID, map[string]string{
			"client_request_id":         paperID,
			"expected_head_revision_id": doc.CurrentRevisionID,
		})
		if err != nil {
			unavailable("research", err)
			return
		}
		switch {
		case status == http.StatusAccepted:
			var receipt struct {
				Replay bool `json:"replay"`
			}
			_ = json.Unmarshal(body, &receipt)
			led.confirm(entry, paperID)
			accepted = true
			if receipt.Replay {
				outcome = "already_asked"
				writeJSON(w, http.StatusConflict, expandAnswer{Status: expandStatusAlready, Error: "this version of the story was already asked to be expanded", RequestID: requestID, Route: src.Route})
				return
			}
			outcome = "asked"
			writeJSON(w, http.StatusAccepted, expandAnswer{Status: expandStatusAsked, RequestID: requestID, Route: src.Route, FromV: src.LatestRevision})
			return
		case status == http.StatusConflict:
			var refusal struct {
				Reason string `json:"reason"`
			}
			_ = json.Unmarshal(body, &refusal)
			log.Printf("proxy: expand: request=%s paper refused research (409): reason=%q", requestID, refusal.Reason)
			if refusal.Reason == "changed" {
				if attempt == 0 {
					continue // Texture committed between the read and the ask: read again, once
				}
				outcome = "changed"
				writeJSON(w, http.StatusConflict, expandAnswer{Status: expandStatusChanged, Error: "the story just changed; try again"})
				return
			}
			outcome = "busy"
			writeJSON(w, http.StatusConflict, expandAnswer{Status: expandStatusBusy, Error: "research is already working on this story; try again shortly"})
			return
		default:
			unavailable("research", fmt.Errorf("status %d", status))
			return
		}
	}
}

func hashedRequester(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(sum[:])[:8]
}

// expansionFate derives what a reader sees for one request (design 6). A
// request is revised when a Texture revision followed its commit at the paper
// and a higher public version exists; that says the story was revised since
// the request, not that the request caused it.
// Silence for expandSilenceAfter is "no new version", which is a fate and not
// an error.
func expansionFate(req expandRequest, accepted, texturedAfter bool, latestVersion int, now time.Time) string {
	if accepted && texturedAfter && latestVersion > req.FromVersion {
		return expandStateRevised
	}
	if now.Sub(req.AskedAt) >= expandSilenceAfter {
		return expandStateNoNew
	}
	if accepted {
		return expandStateAccepted
	}
	return expandStateAsked
}

type expandFateItem struct {
	RequestID   string `json:"request_id"`
	State       string `json:"state"`
	AskedAt     string `json:"asked_at"`
	FromVersion int    `json:"from_version"`
	ToVersion   int    `json:"to_version,omitempty"`
}

type expansionRead struct {
	Allowed  bool             `json:"allowed"`
	Route    string           `json:"route,omitempty"`
	Asked    bool             `json:"asked"`
	Requests []expandFateItem `json:"requests,omitempty"`
}

// HandlePlatformPublicationExpansion serves GET
// /api/platform/publications/expansion?route=: whether the caller may ask
// this story to be expanded, and the fate of what was asked. It answers 200
// allowed:false to anyone not allowed, so the page needs no 401 handling and
// the button never shows. It reads the paper only for an allowed caller who
// has a request to follow.
func (h *Handler) HandlePlatformPublicationExpansion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	route := strings.TrimSpace(r.URL.Query().Get("route"))
	if route == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "route is required"})
		return
	}
	deny := expansionRead{Allowed: false}
	if !h.expandEnabled() {
		writeJSON(w, http.StatusOK, deny)
		return
	}
	caller, _ := h.expandAskers(r)
	if caller == nil {
		writeJSON(w, http.StatusOK, deny)
		return
	}
	src, notFound, err := h.expansionSource(r, route)
	if notFound {
		writeJSON(w, http.StatusOK, deny)
		return
	}
	if err != nil {
		log.Printf("proxy: expansion read: corpusd source: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "could not look up the story"})
		return
	}
	led := h.expandLedger()
	out := expansionRead{Allowed: true, Route: src.Route, Requests: []expandFateItem{}}
	recent := led.forStory(src.PublicationID)
	if !caller.owner && len(recent) > 1 {
		// A public poll follows only the newest request, so an anonymous
		// reader cannot fan one read out into several reads of the paper.
		recent = recent[:1]
	}
	for _, req := range recent {
		out.Requests = append(out.Requests, h.expansionFateItem(r.Context(), led, req, src))
	}
	// After the fate reads: an interrupted request may be asked again.
	req, asked := led.lookup(src.PublicationID, src.SourceRevisionID)
	out.Asked = asked && !req.interrupted
	writeJSON(w, http.StatusOK, out)
}

type expandHistory struct {
	Entries []struct {
		RevisionID string    `json:"revision_id"`
		AuthorKind string    `json:"author_kind"`
		CreatedAt  time.Time `json:"created_at"`
	} `json:"entries"`
}

// expansionFateItem evaluates one request against the paper's read of it and
// the public version. A paper that cannot be read leaves the state "unknown"
// rather than inventing one.
func (h *Handler) expansionFateItem(ctx context.Context, led *expandLedger, req expandRequest, src platform.ExpansionSource) expandFateItem {
	item := expandFateItem{RequestID: req.RequestID, AskedAt: req.AskedAt.UTC().Format(time.RFC3339), FromVersion: req.FromVersion}
	if req.settled {
		item.State, item.ToVersion = expandStateRevised, src.LatestRevision
		return item
	}
	if req.interrupted {
		item.State = expandStateInterrupted
		return item
	}
	if req.PaperRequestID == "" {
		item.State = expansionFate(req, false, false, src.LatestRevision, led.now())
		return item
	}
	base, err := h.paperURL(ctx, req.StoryOwnerID)
	if err != nil {
		item.State = expandStateUnknown
		return item
	}
	state, found, err := h.paperResearch(ctx, base, req.StoryOwnerID, req.DocID, req.PaperRequestID)
	if err != nil {
		item.State = expandStateUnknown
		return item
	}
	if found && state.Interrupted {
		item.State = expandStateInterrupted
		led.markByKey(req.PublicationID, req.SourceRevisionID, func(r *expandRequest) { r.interrupted = true })
		return item
	}
	texturedAfter := false
	if found {
		hstatus, body, herr := h.paperCall(ctx, base, http.MethodGet, "/api/texture/documents/"+url.PathEscape(req.DocID)+"/history", req.StoryOwnerID, nil)
		var hist expandHistory
		if herr == nil && hstatus == http.StatusOK && json.Unmarshal(body, &hist) == nil {
			for _, entry := range hist.Entries {
				if entry.AuthorKind == string(types.AuthorAppAgent) && entry.CreatedAt.After(state.CommittedAt) {
					texturedAfter = true
				}
			}
		}
	}
	item.State = expansionFate(req, found, texturedAfter, src.LatestRevision, led.now())
	if item.State == expandStateRevised {
		item.ToVersion = src.LatestRevision
		led.settleByKey(req.PublicationID, req.SourceRevisionID)
	}
	return item
}

func (l *expandLedger) settleByKey(publicationID, sourceRevisionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if req, ok := l.byKey[expandKey(publicationID, sourceRevisionID)]; ok {
		req.settled = true
	}
}

func (l *expandLedger) markByKey(publicationID, sourceRevisionID string, mark func(*expandRequest)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if req, ok := l.byKey[expandKey(publicationID, sourceRevisionID)]; ok {
		mark(req)
	}
}
