package maild

// Failure modes this file guards (design section 8 plus the two added in
// docs/problems/autopaper-daily-digest-phase1-2026-10-20.md). Written before
// digest.go, digest_store.go and the route wiring.
//
//	O1  Mail goes to someone who did not opt in. Nobody is subscribed by
//	    default; a run with no subscriber sends nothing and does not even read
//	    the front list. An unsubscribed owner is not sent to.
//	M1  Duplicate sends. A rerun, a timer double-fire or a retry sends once:
//	    UNIQUE(owner, run_date, scheduled) in the table, and an explicit
//	    Resend Idempotency-Key of choir_digest_<owner>_<date> that does not
//	    change when the front list does between attempts.
//	M2  The Resend key is missing. The run fails closed: no provider call,
//	    the run row says failed/provider with the reason, the route answers
//	    503 (the timer's unit fails and the journal shows it), and the Mail
//	    app can see sender_configured:false.
//	M3  Resend throttles or is down (429, 5xx). One retry, then failed; a 4xx
//	    is not retried; no catch-up loop.
//	M4  The front list is unavailable. skipped/front_unavailable, nothing
//	    sent, not a failure.
//	M6  Repeats across days: see digest_select_test.go; here the handler
//	    records what it sent and a second day reads it back.
//	M8  No address of record. skipped/no_address, nothing sent.
//	M9  A restart mid-run. No boot resumer: at boot a started run is closed as
//	    "interrupted by a restart" and nothing is sent. An explicit rerun
//	    (not a boot) may then retry; Resend's key dedups a send that did
//	    land.
//	M12 The gate is weakened. The run route needs loopback, the internal
//	    marker and the host-internal credential, and refuses any request that
//	    carries X-Authenticated-User. A guest on the tap, a user request and
//	    an unmarked loopback call all get 403 and send nothing. The owner
//	    routes need the owner session; a guest cannot subscribe the owner.
//	M14 The timer calls without the credential: covered by the enforce-mode
//	    case below and by digest_nix_test.go for the unit.
//	K1  The kill switch MAILD_DIGEST=off: the run route does nothing (200,
//	    status disabled), send-now is refused.
//	P1  Personal data in logs or responses. The full address is never in the
//	    status response (masked) or in a log line (hashed).
//	X1  send-now spams. Preview never sends and writes nothing; a real
//	    send-now needs a subscription and is limited to one an hour; it does
//	    not write the no-repeat ledger and does not block the scheduled run.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

type digestHarness struct {
	t       *testing.T
	h       *Handler
	store   *Store
	cfg     *Config
	resend  *scriptedResend
	front   *scriptedFront
	nowMu   sync.Mutex
	nowTime time.Time
}

type scriptedResend struct {
	mu       sync.Mutex
	statuses []int // consumed one per call; empty means 200
	payloads []resendSendRequest
	keys     []string
	ids      int
}

func (s *scriptedResend) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.payloads)
}

type scriptedFront struct {
	mu     sync.Mutex
	rows   []FrontRow
	status int
	hits   int
}

func (f *scriptedFront) hitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}

func (f *scriptedFront) set(rows []FrontRow) {
	f.mu.Lock()
	f.rows = rows
	f.mu.Unlock()
}

func digestFrontRows(now time.Time) []FrontRow {
	mk := func(id string, minsAgo int, imp float64, section string) FrontRow {
		row := digestRow(id, 0, withImportance(imp), withSection(section))
		row.UpdatedAt = now.Add(-time.Duration(minsAgo) * time.Minute).Format(time.RFC3339)
		return row
	}
	return []FrontRow{
		mk("s1", 30, 9, "world"),
		mk("s2", 60, 8, "business"),
		mk("s3", 90, 7, "tech_ai"),
		mk("s4", 120, 6, "us_politics"),
	}
}

func newDigestHarness(t *testing.T) *digestHarness {
	t.Helper()
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test_key"
	d := &digestHarness{t: t, store: store, cfg: cfg, nowTime: digestTestNow}
	d.resend = &scriptedResend{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendSendRequest
		_ = json.NewDecoder(r.Body).Decode(&payload)
		d.resend.mu.Lock()
		d.resend.payloads = append(d.resend.payloads, payload)
		d.resend.keys = append(d.resend.keys, r.Header.Get("Idempotency-Key"))
		status := http.StatusOK
		if len(d.resend.statuses) > 0 {
			status = d.resend.statuses[0]
			d.resend.statuses = d.resend.statuses[1:]
		}
		d.resend.ids++
		id := d.resend.ids
		d.resend.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"id":"digest-msg-` + string(rune('0'+id)) + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"provider said no"}`))
	}))
	t.Cleanup(provider.Close)
	d.front = &scriptedFront{rows: digestFrontRows(digestTestNow), status: http.StatusOK}
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.front.mu.Lock()
		d.front.hits++
		status, rows := d.front.status, d.front.rows
		d.front.mu.Unlock()
		if r.URL.Path != "/internal/platform/publications/front" {
			t.Errorf("front path = %s", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Errorf("front request lacks the internal marker")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{"stories": rows})
		}
	}))
	t.Cleanup(front.Close)
	vmctl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/internal/vmctl/lookup-guest" && body["remote_addr"] == gateTestGuestAddr {
			_, _ = w.Write([]byte(`{"found":true,"user_id":"` + gateTestOwner + `","computer_id":"computer-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"found":false}`))
	}))
	t.Cleanup(vmctl.Close)
	cfg.ResendBaseURL = provider.URL
	cfg.CorpusdURL = front.URL
	cfg.VmctlURL = vmctl.URL
	d.h = NewHandler(cfg, store)
	d.h.resend = newResendClient(cfg, provider.Client())
	d.h.digestNow = func() time.Time {
		d.nowMu.Lock()
		defer d.nowMu.Unlock()
		return d.nowTime
	}
	d.h.digestRetryDelay = time.Nanosecond
	return d
}

func (d *digestHarness) setNow(now time.Time) {
	d.nowMu.Lock()
	d.nowTime = now
	d.nowMu.Unlock()
}

// subscribe records the owner's address of record and opts them in, as the
// owner's own PUT would.
func (d *digestHarness) subscribe() {
	d.t.Helper()
	w := d.ownerCall(http.MethodPut, "/api/email/digest", "")
	if w.Code != http.StatusOK {
		d.t.Fatalf("subscribe = %d %s", w.Code, w.Body.String())
	}
}

func (d *digestHarness) ownerCall(method, target, body string) *httptest.ResponseRecorder {
	d.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	asOwnerSession(req)
	w := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(d.route(target))).ServeHTTP(w, req)
	return w
}

func (d *digestHarness) route(target string) http.HandlerFunc {
	if strings.HasPrefix(target, DigestSendNowPath) {
		return d.h.HandleDigestSendNow
	}
	return d.h.HandleDigest
}

func (d *digestHarness) run() *httptest.ResponseRecorder {
	d.t.Helper()
	req := httptest.NewRequest(http.MethodPost, DigestRunPath, nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(d.h.HandleDigestRun)).ServeHTTP(w, req)
	return w
}

type digestRunReply struct {
	Status   string `json:"status"`
	Outcomes []struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"outcomes"`
}

func decodeRunReply(t *testing.T, w *httptest.ResponseRecorder) digestRunReply {
	t.Helper()
	var reply digestRunReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode run reply %q: %v", w.Body.String(), err)
	}
	return reply
}

func (d *digestHarness) lastRun(kind string) DigestRun {
	d.t.Helper()
	runs, err := d.store.LatestDigestRuns(context.Background(), gateTestOwner, 20)
	if err != nil {
		d.t.Fatalf("LatestDigestRuns: %v", err)
	}
	for _, run := range runs {
		if run.Kind == kind {
			return run
		}
	}
	d.t.Fatalf("no %s run among %+v", kind, runs)
	return DigestRun{}
}

// ---- opt-in ----

func TestDigestRunSendsNothingWhenNobodyIsSubscribed(t *testing.T) {
	d := newDigestHarness(t)
	w := d.run()
	if w.Code != http.StatusOK {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 0 {
		t.Fatalf("sent %d mails with nobody opted in", d.resend.calls())
	}
	if d.front.hitCount() != 0 {
		t.Fatalf("read the front list %d times with nobody opted in", d.front.hitCount())
	}
}

func TestDigestRunSendsOneMailToTheSubscriber(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	w := d.run()
	if w.Code != http.StatusOK {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 1 {
		t.Fatalf("resend calls = %d, want 1", d.resend.calls())
	}
	p := d.resend.payloads[0]
	if len(p.To) != 1 || p.To[0] != gateTestOwnerMail {
		t.Fatalf("to = %v, want the address of record only", p.To)
	}
	if p.From != "Choir <updates@choir.news>" || !strings.HasPrefix(p.Subject, "Autopaper, Tue 20 Oct: Story s1") {
		t.Fatalf("from/subject = %q / %q", p.From, p.Subject)
	}
	if len(p.Cc)+len(p.Bcc)+len(p.ReplyTo) != 0 {
		t.Fatalf("unexpected cc/bcc/reply-to: %+v", p)
	}
	run := d.lastRun("scheduled")
	if run.Status != "sent" || run.ProviderMessageID == "" || run.RankedBy != "importance" || len(run.StoryIDs) != 4 || run.RunDate != "2026-10-20" {
		t.Fatalf("run row = %+v", run)
	}
}

func TestDigestUnsubscribedOwnerIsNotSent(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	if w := d.ownerCall(http.MethodDelete, "/api/email/digest", ""); w.Code != http.StatusOK {
		t.Fatalf("unsubscribe = %d %s", w.Code, w.Body.String())
	}
	d.run()
	if d.resend.calls() != 0 {
		t.Fatalf("sent after unsubscribe")
	}
}

// ---- M1 ----

func TestDigestRunTwiceSendsOnceWithTheDayKey(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.run()
	w := d.run()
	if w.Code != http.StatusOK {
		t.Fatalf("second run = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 1 {
		t.Fatalf("resend calls = %d after two runs, want 1", d.resend.calls())
	}
	if got, want := d.resend.keys[0], "choir_digest_"+gateTestOwner+"_2026-10-20"; got != want {
		t.Fatalf("Idempotency-Key = %q, want %q", got, want)
	}
	reply := decodeRunReply(t, w)
	if len(reply.Outcomes) != 1 || reply.Outcomes[0].Status != "skipped" || reply.Outcomes[0].Reason != "already_ran" {
		t.Fatalf("second run outcomes = %+v", reply.Outcomes)
	}
}

func TestDigestRetryKeepsTheSameKeyEvenWhenTheFrontListChanges(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.resend.statuses = []int{500, 500}
	w := d.run()
	if w.Code != http.StatusBadGateway {
		t.Fatalf("failed run = %d %s, want 502", w.Code, w.Body.String())
	}
	if d.resend.calls() != 2 {
		t.Fatalf("attempts = %d, want 2 (one retry)", d.resend.calls())
	}
	if run := d.lastRun("scheduled"); run.Status != "failed" || !strings.Contains(run.Reason, "provider") {
		t.Fatalf("run row = %+v", run)
	}
	changed := digestFrontRows(digestTestNow)
	changed[0].Title = "A different headline now"
	d.front.set(changed)
	d.run() // an explicit rerun the same day
	if d.resend.calls() != 3 {
		t.Fatalf("attempts = %d, want 3", d.resend.calls())
	}
	if d.resend.keys[0] != d.resend.keys[2] {
		t.Fatalf("key changed with the content: %q vs %q", d.resend.keys[0], d.resend.keys[2])
	}
	if run := d.lastRun("scheduled"); run.Status != "sent" {
		t.Fatalf("run row after rerun = %+v", run)
	}
}

// ---- M2 ----

func TestDigestMissingSenderKeyFailsClosedAndVisibly(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.cfg.ResendAPIKey = ""
	d.h.resend = newResendClient(d.cfg, nil)
	w := d.run()
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("run = %d %s, want 503", w.Code, w.Body.String())
	}
	if d.resend.calls() != 0 {
		t.Fatalf("called the provider without a key")
	}
	run := d.lastRun("scheduled")
	if run.Status != "failed" || !strings.Contains(run.Reason, "RESEND_API_KEY is not configured") {
		t.Fatalf("run row = %+v", run)
	}
	// Even with nobody subscribed, a missing key is reported, so the unit
	// fails visibly instead of hiding until the first opt-in.
	d2 := newDigestHarness(t)
	d2.cfg.ResendAPIKey = ""
	d2.h.resend = newResendClient(d2.cfg, nil)
	if w := d2.run(); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no-subscriber run without a key = %d, want 503", w.Code)
	}
	// The Mail app can see it.
	status := d.ownerCall(http.MethodGet, "/api/email/digest", "")
	var st map[string]any
	_ = json.Unmarshal(status.Body.Bytes(), &st)
	if st["sender_configured"] != false {
		t.Fatalf("status = %v, want sender_configured:false", st)
	}
}

// ---- M3 ----

func TestDigestThrottleRetriesOnceThenSends(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.resend.statuses = []int{429}
	if w := d.run(); w.Code != http.StatusOK {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 2 || d.lastRun("scheduled").Status != "sent" {
		t.Fatalf("calls=%d run=%+v, want a retry then sent", d.resend.calls(), d.lastRun("scheduled"))
	}
}

func TestDigestClientErrorIsNotRetried(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.resend.statuses = []int{422}
	if w := d.run(); w.Code != http.StatusBadGateway {
		t.Fatalf("run = %d, want 502", w.Code)
	}
	if d.resend.calls() != 1 {
		t.Fatalf("attempts = %d, want 1 (a 4xx is not retried)", d.resend.calls())
	}
}

// ---- M4, M8 ----

func TestDigestFrontListUnavailableIsSkippedNotFailed(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.front.status = http.StatusInternalServerError
	w := d.run()
	if w.Code != http.StatusOK {
		t.Fatalf("run = %d %s, want 200", w.Code, w.Body.String())
	}
	if d.resend.calls() != 0 {
		t.Fatalf("sent without a front list")
	}
	if run := d.lastRun("scheduled"); run.Status != "skipped" || run.Reason != "front_unavailable" {
		t.Fatalf("run row = %+v", run)
	}
	// Once the paper is back, an explicit rerun the same day sends.
	d.front.status = http.StatusOK
	d.run()
	if d.resend.calls() != 1 {
		t.Fatalf("rerun after recovery sent %d, want 1", d.resend.calls())
	}
}

func TestDigestNoAddressOfRecordIsSkipped(t *testing.T) {
	d := newDigestHarness(t)
	if err := d.store.SetDigestSubscription(context.Background(), gateTestOwner, true, d.h.digestNow()); err != nil {
		t.Fatal(err)
	}
	d.run()
	if d.resend.calls() != 0 {
		t.Fatalf("sent with no address of record")
	}
	if run := d.lastRun("scheduled"); run.Status != "skipped" || run.Reason != "no_address" {
		t.Fatalf("run row = %+v", run)
	}
}

func TestDigestTooFewStoriesSendsNothing(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.front.set(digestFrontRows(digestTestNow)[:2])
	d.run()
	if d.resend.calls() != 0 {
		t.Fatalf("sent a digest of fewer than 3 stories")
	}
	if run := d.lastRun("scheduled"); run.Status != "skipped" || run.Reason != "too_few" {
		t.Fatalf("run row = %+v", run)
	}
}

// ---- M6 through the handler ----

func TestDigestSecondDayDoesNotRepeatYesterdaysStories(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.run()
	if d.resend.calls() != 1 {
		t.Fatalf("day 1 sent %d", d.resend.calls())
	}
	day2 := digestTestNow.Add(24 * time.Hour)
	d.setNow(day2)
	rows := digestFrontRows(day2) // same ids, fresh timestamps, same revisions
	d.front.set(rows)
	d.run()
	if d.resend.calls() != 1 {
		t.Fatalf("day 2 repeated yesterday's stories (%d mails)", d.resend.calls())
	}
	if run := d.lastRun("scheduled"); run.RunDate != "2026-10-21" || run.Status != "skipped" || run.Reason != "too_few" {
		t.Fatalf("day 2 row = %+v", run)
	}
	// A revised story and two new ones make a digest.
	rows[0].LatestRevision = 2
	rows[0].VersionCount = 2
	extra := digestFrontRows(day2)
	extra[1].PublicationID, extra[1].Route.Bare = "n1", "/pub/texture/slug-n1"
	extra[2].PublicationID, extra[2].Route.Bare = "n2", "/pub/texture/slug-n2"
	d.front.set([]FrontRow{rows[0], extra[1], extra[2]})
	d.setNow(day2.Add(30 * time.Minute))
	d.run() // same date: the skipped row is retriable
	if d.resend.calls() != 2 {
		t.Fatalf("calls = %d, want the new and revised stories sent", d.resend.calls())
	}
	if !strings.Contains(d.resend.payloads[1].Text, "Updated") {
		t.Fatalf("revised story not marked Updated:\n%s", d.resend.payloads[1].Text)
	}
}

// ---- M9 ----

func TestDigestStartedRunIsClosedAtBootNotResumed(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	ctx := context.Background()
	if _, claimed, err := d.store.ClaimDigestRun(ctx, gateTestOwner, "2026-10-20", digestKindScheduled, d.h.digestNow()); err != nil || !claimed {
		t.Fatalf("claim = %v %v", claimed, err)
	}
	n, err := d.store.CloseOpenDigestRuns(ctx, d.h.digestNow())
	if err != nil || n != 1 {
		t.Fatalf("CloseOpenDigestRuns = %d %v, want 1", n, err)
	}
	if d.resend.calls() != 0 {
		t.Fatalf("closing at boot sent mail")
	}
	run := d.lastRun("scheduled")
	if run.Status != "interrupted" || !strings.Contains(run.Reason, "interrupted by a restart") {
		t.Fatalf("run row = %+v", run)
	}
}

// ---- M12, M14 ----

func TestDigestRunRouteRefusesEverythingButAHostServiceWithTheCredential(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	const token = "digest-gate-test-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	call := func(mutate func(*http.Request)) int {
		req := httptest.NewRequest(http.MethodPost, DigestRunPath, nil)
		mutate(req)
		w := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(d.h.HandleDigestRun)).ServeHTTP(w, req)
		return w.Code
	}
	cases := map[string]func(*http.Request){
		"guest on the tap": func(r *http.Request) {
			r.RemoteAddr = gateTestGuestAddr
			r.Header.Set("X-Internal-Caller", "true")
			r.Header.Set(hostauth.Header, token)
		},
		"loopback without the marker": func(r *http.Request) { r.Header.Set(hostauth.Header, token) },
		"loopback without the credential": func(r *http.Request) {
			r.Header.Set("X-Internal-Caller", "true")
		},
		"loopback with a wrong credential": func(r *http.Request) {
			r.Header.Set("X-Internal-Caller", "true")
			r.Header.Set(hostauth.Header, "wrong-credential-0123456789abcdef0123456789")
		},
		"a forwarded user request": func(r *http.Request) {
			r.Header.Set("X-Internal-Caller", "true")
			r.Header.Set(hostauth.Header, token)
			r.Header.Set("X-Authenticated-User", gateTestOwner)
		},
	}
	for name, mutate := range cases {
		if code := call(mutate); code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, code)
		}
	}
	if d.resend.calls() != 0 {
		t.Fatalf("a refused call sent %d mails", d.resend.calls())
	}
	if code := call(func(r *http.Request) {
		r.Header.Set("X-Internal-Caller", "true")
		r.Header.Set(hostauth.Header, token)
	}); code != http.StatusOK {
		t.Fatalf("host service with the credential: %d, want 200", code)
	}
	if d.resend.calls() != 1 {
		t.Fatalf("the permitted call sent %d mails, want 1", d.resend.calls())
	}
}

func TestDigestRunRouteMethod(t *testing.T) {
	d := newDigestHarness(t)
	req := httptest.NewRequest(http.MethodGet, DigestRunPath, nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	d.h.HandleDigestRun(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", w.Code)
	}
}

func TestDigestOwnerRoutesNeedTheOwnerSession(t *testing.T) {
	d := newDigestHarness(t)
	// No identity at all.
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(m, "/api/email/digest", nil)
		w := httptest.NewRecorder()
		d.h.HandleDigest(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s without identity = %d, want 401", m, w.Code)
		}
	}
	// A guest bound to the owner cannot subscribe (or unsubscribe) the owner.
	for _, m := range []string{http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(m, "/api/email/digest", nil)
		asGuest(req)
		req.Header.Set("X-Authenticated-Email", "attacker@example.com")
		w := httptest.NewRecorder()
		d.h.HandleDigest(w, req)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "owner session required") {
			t.Errorf("guest %s = %d %s, want 403 owner session required", m, w.Code, w.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, DigestSendNowPath, nil)
	asGuest(req)
	w := httptest.NewRecorder()
	d.h.HandleDigestSendNow(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("guest send-now = %d, want 403", w.Code)
	}
	d.run()
	if d.resend.calls() != 0 {
		t.Fatalf("a guest request caused mail")
	}
}

func TestDigestSubscribeNeedsAVerifiedAddressAndMasksIt(t *testing.T) {
	d := newDigestHarness(t)
	req := httptest.NewRequest(http.MethodPut, "/api/email/digest", nil)
	setInternalOwner(req, gateTestOwner) // no X-Authenticated-Email
	w := httptest.NewRecorder()
	d.h.HandleDigest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("subscribe without an address = %d, want 400", w.Code)
	}
	d.subscribe()
	w = d.ownerCall(http.MethodGet, "/api/email/digest", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), gateTestOwnerMail) {
		t.Fatalf("status leaks the full address: %s", w.Body.String())
	}
	var st struct {
		Subscribed       bool   `json:"subscribed"`
		AddressMasked    string `json:"address_masked"`
		SenderConfigured bool   `json:"sender_configured"`
		Enabled          bool   `json:"enabled"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if !st.Subscribed || st.AddressMasked != "o***@example.com" || !st.SenderConfigured || !st.Enabled {
		t.Fatalf("status = %+v", st)
	}
}

// ---- K1 ----

func TestDigestKillSwitch(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.cfg.DigestOff = true
	w := d.run()
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "disabled") {
		t.Fatalf("run with the kill switch = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 0 || d.front.hitCount() != 0 {
		t.Fatalf("kill switch did not stop the run (mails %d, front reads %d)", d.resend.calls(), d.front.hitCount())
	}
	if w := d.ownerCall(http.MethodPost, DigestSendNowPath, ""); w.Code != http.StatusConflict {
		t.Fatalf("send-now with the kill switch = %d, want 409", w.Code)
	}
}

// ---- X1 ----

func TestDigestPreviewSendsAndWritesNothing(t *testing.T) {
	d := newDigestHarness(t)
	w := d.ownerCall(http.MethodPost, DigestSendNowPath+"?preview=1", "") // not even subscribed
	if w.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", w.Code, w.Body.String())
	}
	var pv struct {
		Preview  bool   `json:"preview"`
		Status   string `json:"status"`
		RankedBy string `json:"ranked_by"`
		Subject  string `json:"subject"`
		Text     string `json:"text"`
		Stories  []struct {
			PublicationID string `json:"publication_id"`
			Link          string `json:"link"`
			Rank          int    `json:"rank"`
		} `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pv); err != nil {
		t.Fatal(err)
	}
	if !pv.Preview || pv.Status != "ok" || pv.RankedBy != "importance" || len(pv.Stories) != 4 || pv.Subject == "" || pv.Text == "" {
		t.Fatalf("preview = %+v", pv)
	}
	if pv.Stories[0].PublicationID != "s1" || pv.Stories[0].Link != "https://choir.news/pub/texture/slug-s1" || pv.Stories[0].Rank != 1 {
		t.Fatalf("preview lead = %+v", pv.Stories[0])
	}
	if d.resend.calls() != 0 {
		t.Fatalf("preview sent mail")
	}
	runs, _ := d.store.LatestDigestRuns(context.Background(), gateTestOwner, 5)
	if len(runs) != 0 {
		t.Fatalf("preview wrote run rows: %+v", runs)
	}
}

func TestDigestSendNowNeedsSubscriptionIsLimitedAndLeavesTheScheduledRunAlone(t *testing.T) {
	d := newDigestHarness(t)
	if w := d.ownerCall(http.MethodPost, DigestSendNowPath, ""); w.Code != http.StatusConflict {
		t.Fatalf("send-now while unsubscribed = %d, want 409", w.Code)
	}
	d.subscribe()
	w := d.ownerCall(http.MethodPost, DigestSendNowPath, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "digest-msg-") {
		t.Fatalf("send-now = %d %s", w.Code, w.Body.String())
	}
	if d.resend.calls() != 1 || d.resend.payloads[0].To[0] != gateTestOwnerMail {
		t.Fatalf("send-now mail = %+v", d.resend.payloads)
	}
	if w := d.ownerCall(http.MethodPost, DigestSendNowPath, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second send-now within the hour = %d, want 429", w.Code)
	}
	if d.resend.calls() != 1 {
		t.Fatalf("limited send-now still sent")
	}
	// After the hour it is allowed again.
	d.setNow(digestTestNow.Add(61 * time.Minute))
	d.front.set(digestFrontRows(d.h.digestNow()))
	if w := d.ownerCall(http.MethodPost, DigestSendNowPath, ""); w.Code != http.StatusOK {
		t.Fatalf("send-now after the hour = %d %s", w.Code, w.Body.String())
	}
	// A manual send is not the scheduled series: the day's scheduled digest
	// still goes, with the same stories (no-repeat ledger untouched).
	d.run()
	if d.resend.calls() != 3 {
		t.Fatalf("scheduled run after manual sends: %d mails, want 3 in all", d.resend.calls())
	}
	if run := d.lastRun("scheduled"); run.Status != "sent" || len(run.StoryIDs) != 4 {
		t.Fatalf("scheduled row = %+v", run)
	}
	// Unsubscribing makes send-now refuse again.
	d.ownerCall(http.MethodDelete, "/api/email/digest", "")
	if w := d.ownerCall(http.MethodPost, DigestSendNowPath, ""); w.Code != http.StatusConflict {
		t.Fatalf("send-now after unsubscribe = %d, want 409", w.Code)
	}
}

// The status response reports the last run so the Mail app can show it.
func TestDigestStatusReportsLastRun(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.run()
	w := d.ownerCall(http.MethodGet, "/api/email/digest", "")
	var st struct {
		LastRun *struct {
			RunDate           string   `json:"run_date"`
			Kind              string   `json:"kind"`
			Status            string   `json:"status"`
			ProviderMessageID string   `json:"provider_message_id"`
			StoryIDs          []string `json:"story_ids"`
			RankedBy          string   `json:"ranked_by"`
		} `json:"last_run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || st.LastRun == nil {
		t.Fatalf("status = %s (%v)", w.Body.String(), err)
	}
	lr := st.LastRun
	if lr.RunDate != "2026-10-20" || lr.Kind != "scheduled" || lr.Status != "sent" || lr.ProviderMessageID == "" || len(lr.StoryIDs) != 4 || lr.RankedBy != "importance" {
		t.Fatalf("last_run = %+v", lr)
	}
}
