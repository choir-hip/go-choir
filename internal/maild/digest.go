package maild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// The Autopaper daily digest (docs/reports/autopaper-daily-digest-design-
// 2026-10-20.md; docs/problems/autopaper-daily-digest-phase1-2026-10-20.md).
//
// A fixed program, not an agent: a host timer calls DigestRunPath once a day;
// maild reads the front list, picks the stories with a deterministic function
// and sends one fixed-template mail through the Resend sender it already
// uses, to each owner who opted in. No model is called. Nobody is opted in by
// default. A missing Resend key fails closed and visibly.
const (
	// DigestPath is the owner's status / subscribe / unsubscribe route
	// (GET, PUT, DELETE), reached through the authenticated email proxy.
	DigestPath = "/api/email/digest"
	// DigestSendNowPath sends (or, with ?preview=1, previews) the digest to
	// the signed-in owner now. POST.
	DigestSendNowPath = "/api/email/digest/send-now"
	// DigestRunPath is the host-internal scheduled run. Like the recovery
	// link route it lives under /internal/, which Caddy refuses at the edge
	// and the proxy never forwards, and it needs the host credential.
	DigestRunPath = "/internal/maild/digest/run"

	digestFrontLimit     = 60 // the page's first load (FRONT_FIRST_PAGE_SIZE), so edition and score coverage match
	digestManualInterval = time.Hour
	digestRetryDefault   = 30 * time.Second
	digestAttemptTimeout = 30 * time.Second
	digestScheduleText   = "12:00 UTC daily"
	// digestSlotHour is the UTC hour of the daily slot. The host timer fires
	// at 12:00, 12:10 ... 12:50; a run that ended transiently is retried by
	// those fires and by nothing else, and never after the hour.
	digestSlotHour = 12
)

var digestKeyOwnerRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func (h *Handler) digestClock() time.Time {
	if h != nil && h.digestNow != nil {
		return h.digestNow()
	}
	return time.Now()
}

func (h *Handler) digestEnabled() bool { return h != nil && h.cfg != nil && !h.cfg.DigestOff }

func (h *Handler) digestSenderConfigured() bool {
	return h != nil && strings.TrimSpace(h.resend.apiKey) != ""
}

func (h *Handler) digestDomain() string {
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.PrimaryDomain) != "" {
		return strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	}
	return DefaultPrimaryDomain
}

func digestDay(t time.Time) string { return t.UTC().Format("2006-01-02") }

// maskedAddress shows enough of an address to recognise it: "o***@example.com".
func maskedAddress(address string) string {
	local, domain, ok := strings.Cut(address, "@")
	if !ok || domain == "" {
		return "***"
	}
	if local == "" {
		return "***@" + domain
	}
	return string([]rune(local)[:1]) + "***@" + domain
}

// digestIdempotencyKey names one scheduled send: choir_digest_<owner>_<date>.
func digestIdempotencyKey(ownerID string, day string) string {
	owner := ownerID
	if !digestKeyOwnerRe.MatchString(owner) {
		sum := sha256.Sum256([]byte(ownerID))
		owner = hex.EncodeToString(sum[:8])
	}
	return "choir_digest_" + owner + "_" + day
}

// ---- front list ----

func (h *Handler) fetchDigestFront(ctx context.Context) ([]FrontRow, error) {
	base := DefaultCorpusdURL
	if h.cfg != nil && strings.TrimSpace(h.cfg.CorpusdURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(h.cfg.CorpusdURL), "/")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/internal/platform/publications/front?limit=%d", base, digestFrontLimit), nil)
	if err != nil {
		return nil, err
	}
	hostauth.MarkInternal(req)
	req.Header.Set("Accept", "application/json")
	client := h.digestHTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("front list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("front list returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("front list read: %w", err)
	}
	var page struct {
		Stories []FrontRow `json:"stories"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("front list decode: %w", err)
	}
	return page.Stories, nil
}

// ---- running one recipient ----

type digestOutcome struct {
	Owner             string   `json:"owner"`
	Kind              string   `json:"kind"`
	Status            string   `json:"status"`
	Reason            string   `json:"reason,omitempty"`
	ProviderMessageID string   `json:"provider_message_id,omitempty"`
	StoryIDs          []string `json:"story_ids,omitempty"`
	RankedBy          string   `json:"ranked_by,omitempty"`
}

func (h *Handler) logDigest(o digestOutcome) {
	log.Printf("maild: operation=autopaper_digest owner_hash=%s kind=%s result=%s reason=%q provider_message_id=%s ranked_by=%s stories=%d",
		o.Owner, o.Kind, o.Status, o.Reason, o.ProviderMessageID, o.RankedBy, len(o.StoryIDs))
}

// finishDigest closes a claimed run and returns its outcome.
func (h *Handler) finishDigest(ctx context.Context, runID, ownerID, kind, status, reason, messageID string, storyIDs []string, rankedBy string) digestOutcome {
	o := digestOutcome{Owner: authMailAddressHash(ownerID), Kind: kind, Status: status, Reason: reason,
		ProviderMessageID: messageID, StoryIDs: storyIDs, RankedBy: rankedBy}
	if err := h.store.FinishDigestRun(ctx, runID, status, reason, messageID, storyIDs, rankedBy, h.digestClock()); err != nil {
		log.Printf("maild: operation=autopaper_digest owner_hash=%s kind=%s result=record_error error=%q", o.Owner, kind, err)
	}
	h.logDigest(o)
	return o
}

// digestSlotOpen reports whether now is inside the slot hour.
func digestSlotOpen(now time.Time) bool { return now.UTC().Hour() == digestSlotHour }

// digestSlotRetryable reports whether a scheduled row that did not send may be
// reopened by a later fire of the same slot: the transient ends only. A
// deliberate skip (no address, too few new stories) is final for the slot.
func digestSlotRetryable(status, reason string) bool {
	switch status {
	case digestStatusFailed, digestStatusInterrupted:
		return true
	case digestStatusSkipped:
		return reason == "front_unavailable" || reason == "too_few"
	}
	return false
}

func digestRetryable(err error) bool {
	var httpErr *resendHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
	}
	// A transport failure (sendEmail wraps it "send email: ..."). A missing
	// key or a malformed request is not worth a second try.
	return strings.HasPrefix(err.Error(), "send email: ")
}

// sendDigestMail sends once and retries once after a delay when Resend
// throttles, errors or cannot be reached. No catch-up loop beyond that.
func (h *Handler) sendDigestMail(ctx context.Context, req resendSendRequest) (resendSendResponse, error) {
	attempt := func() (resendSendResponse, error) {
		actx, cancel := context.WithTimeout(ctx, digestAttemptTimeout)
		defer cancel()
		return h.resend.sendEmail(actx, req)
	}
	resp, err := attempt()
	if err == nil || !digestRetryable(err) {
		return resp, err
	}
	delay := h.digestRetryDelay
	if delay <= 0 {
		delay = digestRetryDefault
	}
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return resp, err
	}
	return attempt()
}

// executeDigest runs a claimed digest for one recipient: select, render, send,
// record. scheduled runs honour and extend the no-repeat ledger; manual runs
// (send-now) neither read nor write it.
func (h *Handler) executeDigest(ctx context.Context, runID, ownerID, address, kind string, rows []FrontRow, now time.Time) digestOutcome {
	var sent map[string]int
	if kind == digestKindScheduled {
		var err error
		if sent, err = h.store.DigestSentVersions(ctx, ownerID); err != nil {
			return h.finishDigest(ctx, runID, ownerID, kind, digestStatusFailed, "ledger: "+err.Error(), "", nil, "")
		}
	}
	sel := selectDigest(rows, now, sent)
	if sel.Skipped != "" {
		return h.finishDigest(ctx, runID, ownerID, kind, digestStatusSkipped, sel.Skipped, "", nil, sel.RankedBy)
	}
	mail := renderDigest(sel, h.digestDomain(), now, now)
	key := digestIdempotencyKey(ownerID, digestDay(now))
	if kind == digestKindManual {
		key = "choir_digest_manual_" + runID
	}
	resp, err := h.sendDigestMail(ctx, resendSendRequest{
		From:           "Choir <updates@" + h.digestDomain() + ">",
		To:             []string{address},
		Subject:        mail.Subject,
		Text:           mail.Text,
		HTML:           mail.HTML,
		Headers:        mail.Headers,
		idempotencyKey: key,
	})
	ids := make([]string, 0, len(sel.Stories))
	versions := map[string]int{}
	for _, s := range sel.Stories {
		ids = append(ids, s.PublicationID)
		versions[s.PublicationID] = s.Revision
	}
	if err != nil {
		return h.finishDigest(ctx, runID, ownerID, kind, digestStatusFailed, "provider: "+err.Error(), "", ids, sel.RankedBy)
	}
	if kind == digestKindScheduled {
		if err := h.store.RecordDigestSent(ctx, ownerID, versions, now); err != nil {
			log.Printf("maild: operation=autopaper_digest owner_hash=%s result=ledger_error error=%q", authMailAddressHash(ownerID), err)
		}
	}
	return h.finishDigest(ctx, runID, ownerID, kind, digestStatusSent, "", resp.ID, ids, sel.RankedBy)
}

// ---- the scheduled run (host-internal) ----

type digestRunResponse struct {
	Status   string          `json:"status"`
	Date     string          `json:"date,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Outcomes []digestOutcome `json:"outcomes"`
}

// HandleDigestRun is called once a day by the host timer
// (go-choir-digest.timer). It accepts only a host service on loopback that
// holds the host-internal credential, and refuses anything forwarded for a
// user: a guest on the tap, or a signed-in user, must never be able to cause
// mail from here.
func (h *Handler) HandleDigestRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !isMaildLoopbackHost(maildRemoteAddrHost(r)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "host service caller required"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
		return
	}
	// Loopback is not a credential on its own (docs/problems/loopback-is-a-
	// credential-2026-10-14.md); the timer sends the host token.
	if !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
		return
	}
	if strings.TrimSpace(r.Header.Get("X-Authenticated-User")) != "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available to user requests"})
		return
	}
	if !h.digestEnabled() {
		log.Printf("maild: operation=autopaper_digest result=disabled reason=%q", "MAILD_DIGEST=off")
		writeJSON(w, http.StatusOK, digestRunResponse{Status: "disabled", Outcomes: []digestOutcome{}})
		return
	}

	ctx := r.Context()
	h.digestMu.Lock()
	defer h.digestMu.Unlock()
	now := h.digestClock()
	day := digestDay(now)
	resp := digestRunResponse{Status: "ok", Date: day, Outcomes: []digestOutcome{}}

	subscribers, err := h.store.ListDigestSubscribers(ctx)
	if err != nil {
		log.Printf("maild: operation=autopaper_digest result=error error=%q", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read subscribers"})
		return
	}

	// Fail closed and visibly: without the sender key nothing can be sent.
	// The route answers 503, so the timer's curl fails and the unit shows
	// failed in systemd, even before anyone has opted in.
	if !h.digestSenderConfigured() {
		const reason = "provider: RESEND_API_KEY is not configured"
		log.Printf("maild: operation=autopaper_digest result=failed reason=%q subscribers=%d (the digest cannot be sent until the Resend key is configured)", reason, len(subscribers))
		for _, owner := range subscribers {
			runID, claimed, err := h.store.ClaimDigestRun(ctx, owner, day, digestKindScheduled, now)
			if err != nil || !claimed {
				continue
			}
			resp.Outcomes = append(resp.Outcomes, h.finishDigest(ctx, runID, owner, digestKindScheduled, digestStatusFailed, reason, "", nil, ""))
		}
		resp.Status, resp.Reason = "failed", reason
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	if len(subscribers) == 0 {
		log.Printf("maild: operation=autopaper_digest result=nothing_to_do reason=%q", "no one has opted in")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rows, frontErr := h.fetchDigestFront(ctx)
	if frontErr != nil {
		log.Printf("maild: operation=autopaper_digest result=front_unavailable error=%q", frontErr)
	}
	failed := false
	for _, owner := range subscribers {
		// Retry bounds: a row that already ended is reopened only for a
		// transient end and only inside the slot hour. After the hour the
		// last outcome stays as the slot's final record.
		if status, reason, exists, serr := h.store.ScheduledDigestRunState(ctx, owner, day); serr != nil {
			log.Printf("maild: operation=autopaper_digest owner_hash=%s result=claim_error error=%q", authMailAddressHash(owner), serr)
			failed = true
			continue
		} else if exists && status != digestStatusStarted && status != digestStatusSent {
			if !digestSlotRetryable(status, reason) {
				resp.Outcomes = append(resp.Outcomes, digestOutcome{Owner: authMailAddressHash(owner), Kind: digestKindScheduled,
					Status: digestStatusSkipped, Reason: "already_ran"})
				continue
			}
			if !digestSlotOpen(now) {
				resp.Outcomes = append(resp.Outcomes, digestOutcome{Owner: authMailAddressHash(owner), Kind: digestKindScheduled,
					Status: digestStatusSkipped, Reason: "slot_closed"})
				continue
			}
		}
		runID, claimed, err := h.store.ClaimDigestRun(ctx, owner, day, digestKindScheduled, now)
		if err != nil {
			log.Printf("maild: operation=autopaper_digest owner_hash=%s result=claim_error error=%q", authMailAddressHash(owner), err)
			failed = true
			continue
		}
		if !claimed {
			resp.Outcomes = append(resp.Outcomes, digestOutcome{Owner: authMailAddressHash(owner), Kind: digestKindScheduled,
				Status: digestStatusSkipped, Reason: "already_ran"})
			continue
		}
		address, err := h.store.OwnerAddressOfRecord(ctx, owner)
		if err != nil || strings.TrimSpace(address) == "" {
			resp.Outcomes = append(resp.Outcomes, h.finishDigest(ctx, runID, owner, digestKindScheduled, digestStatusSkipped, "no_address", "", nil, ""))
			continue
		}
		if frontErr != nil {
			resp.Outcomes = append(resp.Outcomes, h.finishDigest(ctx, runID, owner, digestKindScheduled, digestStatusSkipped, "front_unavailable", "", nil, ""))
			continue
		}
		o := h.executeDigest(ctx, runID, owner, address, digestKindScheduled, rows, now)
		if o.Status == digestStatusFailed {
			failed = true
		}
		resp.Outcomes = append(resp.Outcomes, o)
	}
	if failed {
		resp.Status = "failed"
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- owner routes ----

type digestStatusResponse struct {
	Enabled          bool        `json:"enabled"`
	Subscribed       bool        `json:"subscribed"`
	AddressMasked    string      `json:"address_masked,omitempty"`
	SenderConfigured bool        `json:"sender_configured"`
	Schedule         string      `json:"schedule"`
	LastRun          *DigestRun  `json:"last_run"`
	RecentRuns       []DigestRun `json:"recent_runs"`
}

// requireDigestOwner authenticates the caller and refuses anything but the
// owner's session. Unlike the draft send gate it does not relax in the legacy
// mode: a guest can never opt the owner in or cause the owner's mail.
func (h *Handler) requireDigestOwner(w http.ResponseWriter, r *http.Request) (maildCaller, bool) {
	caller, ok := h.authenticateCaller(w, r)
	if !ok {
		return maildCaller{}, false
	}
	if caller.Class != callerOwnerSession {
		log.Printf("maild: digest: refused %s from %s caller owner=%s: owner session required", r.Method, caller.Class, caller.OwnerID)
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "owner session required: the daily digest is an owner act and cannot come from the computer",
		})
		return maildCaller{}, false
	}
	return caller, true
}

// HandleDigest serves GET (status), PUT (subscribe) and DELETE (unsubscribe).
func (h *Handler) HandleDigest(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.requireDigestOwner(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		if caller.SessionEmail == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a verified account email is required to subscribe"})
			return
		}
		if err := h.store.SetDigestSubscription(ctx, caller.OwnerID, true, h.digestClock()); err != nil {
			log.Printf("maild: digest: subscribe owner=%s: %v", caller.OwnerID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to subscribe"})
			return
		}
		log.Printf("maild: operation=autopaper_digest owner_hash=%s result=subscribed", authMailAddressHash(caller.OwnerID))
	case http.MethodDelete:
		if err := h.store.SetDigestSubscription(ctx, caller.OwnerID, false, h.digestClock()); err != nil {
			log.Printf("maild: digest: unsubscribe owner=%s: %v", caller.OwnerID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to unsubscribe"})
			return
		}
		log.Printf("maild: operation=autopaper_digest owner_hash=%s result=unsubscribed", authMailAddressHash(caller.OwnerID))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	subscribed, err := h.store.DigestSubscribed(ctx, caller.OwnerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read subscription"})
		return
	}
	runs, err := h.store.LatestDigestRuns(ctx, caller.OwnerID, 7)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read runs"})
		return
	}
	out := digestStatusResponse{
		Enabled:          h.digestEnabled(),
		Subscribed:       subscribed,
		SenderConfigured: h.digestSenderConfigured(),
		Schedule:         digestScheduleText,
		RecentRuns:       runs,
	}
	if out.RecentRuns == nil {
		out.RecentRuns = []DigestRun{}
	}
	if address := h.ownerAddressOfRecord(ctx, caller); address != "" {
		out.AddressMasked = maskedAddress(address)
	}
	if len(runs) > 0 {
		out.LastRun = &runs[0]
	}
	writeJSON(w, http.StatusOK, out)
}

type digestPreviewStory struct {
	PublicationID string `json:"publication_id"`
	Title         string `json:"title"`
	Section       string `json:"section,omitempty"`
	SourceCount   *int   `json:"source_count,omitempty"`
	Rank          int    `json:"rank"`
	Revision      int    `json:"revision"`
	Slug          string `json:"slug"`
	Link          string `json:"link"`
	Updated       bool   `json:"updated,omitempty"`
}

type digestPreviewResponse struct {
	Preview  bool                 `json:"preview"`
	Status   string               `json:"status"`
	Reason   string               `json:"reason,omitempty"`
	RankedBy string               `json:"ranked_by"`
	Subject  string               `json:"subject,omitempty"`
	Text     string               `json:"text,omitempty"`
	Stories  []digestPreviewStory `json:"stories"`
}

// HandleDigestSendNow sends the digest to the signed-in owner now, or with
// ?preview=1 shows what would be sent. It is the acceptance and preview path.
// A real send needs a subscription, is limited to one an hour, and is not part
// of the scheduled series (it neither reads nor writes the no-repeat ledger).
func (h *Handler) HandleDigestSendNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	caller, ok := h.requireDigestOwner(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	preview := false
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("preview"))) {
	case "1", "true", "yes":
		preview = true
	}

	if preview {
		rows, err := h.fetchDigestFront(ctx)
		if err != nil {
			log.Printf("maild: operation=autopaper_digest kind=preview result=front_unavailable error=%q", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "front list unavailable"})
			return
		}
		now := h.digestClock()
		sel := selectDigest(rows, now, nil)
		out := digestPreviewResponse{Preview: true, Status: "ok", RankedBy: sel.RankedBy, Stories: []digestPreviewStory{}}
		if sel.Skipped != "" {
			out.Status, out.Reason = "skipped", sel.Skipped
		} else {
			mail := renderDigest(sel, h.digestDomain(), now, now)
			out.Subject, out.Text = mail.Subject, mail.Text
			for i, s := range sel.Stories {
				out.Stories = append(out.Stories, digestPreviewStory{
					PublicationID: s.PublicationID, Title: cleanDigestText(s.Title, digestTitleMaxRunes), Section: s.Section,
					SourceCount: s.SourceCount, Rank: i + 1, Revision: s.Revision, Slug: s.Slug,
					Link: digestStoryLink(h.digestDomain(), s.Slug), Updated: s.Updated,
				})
			}
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	if !h.digestEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the daily digest is switched off (MAILD_DIGEST=off)"})
		return
	}
	h.digestMu.Lock()
	defer h.digestMu.Unlock()
	subscribed, err := h.store.DigestSubscribed(ctx, caller.OwnerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read subscription"})
		return
	}
	if !subscribed {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "not subscribed: turn the daily digest on first"})
		return
	}
	address := caller.SessionEmail
	if address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a verified account email is required"})
		return
	}
	now := h.digestClock()
	if recent, err := h.store.RecentManualDigestSend(ctx, caller.OwnerID, now.Add(-digestManualInterval)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read recent sends"})
		return
	} else if recent {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "send-now is limited to one an hour"})
		return
	}
	runID, _, err := h.store.ClaimDigestRun(ctx, caller.OwnerID, digestDay(now), digestKindManual, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record the run"})
		return
	}
	if !h.digestSenderConfigured() {
		o := h.finishDigest(ctx, runID, caller.OwnerID, digestKindManual, digestStatusFailed, "provider: RESEND_API_KEY is not configured", "", nil, "")
		writeJSON(w, http.StatusServiceUnavailable, o)
		return
	}
	rows, err := h.fetchDigestFront(ctx)
	if err != nil {
		log.Printf("maild: operation=autopaper_digest kind=manual result=front_unavailable error=%q", err)
		o := h.finishDigest(ctx, runID, caller.OwnerID, digestKindManual, digestStatusSkipped, "front_unavailable", "", nil, "")
		writeJSON(w, http.StatusBadGateway, o)
		return
	}
	o := h.executeDigest(ctx, runID, caller.OwnerID, address, digestKindManual, rows, now)
	code := http.StatusOK
	if o.Status == digestStatusFailed {
		code = http.StatusBadGateway
	}
	writeJSON(w, code, o)
}
