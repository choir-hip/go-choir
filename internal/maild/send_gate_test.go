package maild

// E1 send-gate tests. Each test names the failure mode it covers from
// docs/archive/problems/maild-send-gate-defects-2026-10-14.md ("FM n").

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

const (
	gateTestOwner     = "user-root"
	gateTestOwnerMail = "owner@example.com"
	gateTestAuthserv  = "mx.provider.test"
	gateTestGuestAddr = "10.200.115.2:41000"
)

// gateResend records every provider call so a test can assert "nothing was
// sent" rather than infer it from a status code.
type gateResend struct {
	mu       sync.Mutex
	payloads []resendSendRequest
	server   *httptest.Server
}

func newGateResend(t *testing.T) *gateResend {
	t.Helper()
	g := &gateResend{}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider payload: %v", err)
		}
		g.mu.Lock()
		g.payloads = append(g.payloads, payload)
		n := len(g.payloads)
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-` + string(rune('0'+n)) + `"}`))
	}))
	t.Cleanup(g.server.Close)
	return g
}

func (g *gateResend) calls() []resendSendRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]resendSendRequest(nil), g.payloads...)
}

// newGateHandler wires a store, a recording provider and a vmctl stub that
// binds gateTestGuestAddr to gateTestOwner (S1a).
func newGateHandler(t *testing.T) (*Handler, *Store, *gateResend) {
	t.Helper()
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	cfg.TrustedAuthservIDs = []string{gateTestAuthserv}
	resend := newGateResend(t)
	vmctl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup-guest" {
			// Trace events after an approved send resolve the owner's
			// computer; a guest-refused send must never get here.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if body["remote_addr"] != gateTestGuestAddr {
			_, _ = w.Write([]byte(`{"found":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"found":true,"user_id":"` + gateTestOwner + `","computer_id":"computer-1"}`))
	}))
	t.Cleanup(vmctl.Close)
	cfg.ResendBaseURL = resend.server.URL
	cfg.VmctlURL = vmctl.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.server.Client())
	return h, store, resend
}

func gateDraft(t *testing.T, store *Store) EmailDraft {
	t.Helper()
	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), gateTestOwner, alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Gate",
		TextBody:    "Body the owner reviewed.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return draft
}

// asGuest makes req look like the owner's computer over its tap: a
// non-loopback source that vmctl binds to the owner. Headers a guest can
// set are set too, to prove they do not change the class.
func asGuest(req *http.Request) {
	req.RemoteAddr = gateTestGuestAddr
	req.Header.Set("X-Authenticated-User", gateTestOwner)
	req.Header.Set("X-Internal-Caller", "true")
}

// asOwnerSession makes req look like the proxy forwarding the owner's
// session (loopback in tests is TEST-NET-1, the httptest default).
func asOwnerSession(req *http.Request) {
	setInternalOwner(req, gateTestOwner)
	req.Header.Set("X-Authenticated-Email", gateTestOwnerMail)
}

func serveDrafts(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(h.HandleDrafts)).ServeHTTP(w, req)
	return w
}

func approvalEventCount(t *testing.T, store *Store, draftID string) int {
	t.Helper()
	db, err := store.mailboxForOwner(gateTestOwner)
	if err != nil {
		t.Fatalf("mailbox: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM email_draft_approval_events WHERE draft_id = ?`, draftID).Scan(&n); err != nil {
		t.Fatalf("count approval events: %v", err)
	}
	return n
}

// FM 1, FM 2: a guest bound to the owner cannot approve and send, even with
// the current hash, the internal-caller marker and a guessed credential.
func TestGuestCannotApproveAndSendDraft(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send",
		strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	asGuest(req)
	req.Header.Set(hostauth.Header, "guessed-credential-0123456789abcdef0123456789")
	w := serveDrafts(h, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest send status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "owner session required") {
		t.Fatalf("guest send body = %s", w.Body.String())
	}
	if calls := resend.calls(); len(calls) != 0 {
		t.Fatalf("guest send reached the provider: %+v", calls)
	}
	after, err := store.GetDraft(nilSafeContext(), gateTestOwner, draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if after.Status != "draft_pending_owner_approval" || after.VersionHash != draft.VersionHash {
		t.Fatalf("draft after refused guest send = %+v", after)
	}
	if n := approvalEventCount(t, store, draft.ID); n != 0 {
		t.Fatalf("approval events after refused guest send = %d, want 0", n)
	}

	// Trailing slash is the same route to maild's parser.
	req = httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send/",
		strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	asGuest(req)
	if w := serveDrafts(h, req); w.Code != http.StatusForbidden {
		t.Fatalf("guest send with trailing slash status = %d, want 403", w.Code)
	}
}

// FM 3: the owner's session through the proxy still sends.
func TestOwnerSessionStillApprovesAndSends(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send",
		strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	asOwnerSession(req)
	w := serveDrafts(h, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("owner send status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	if calls := resend.calls(); len(calls) != 1 || calls[0].To[0] != "friend@example.com" {
		t.Fatalf("provider calls = %+v", calls)
	}
}

// FM 4: the guest keeps draft authority: create, list, read.
func TestGuestKeepsDraftAuthority(t *testing.T) {
	h, _, resend := newGateHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts", strings.NewReader(`{
		"to_addresses":["friend@example.com"],
		"subject":"From the desk",
		"text_body":"Proposed reply."
	}`))
	asGuest(req)
	w := serveDrafts(h, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("guest create status = %d; body=%s", w.Code, w.Body.String())
	}
	var created draftResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, path := range []string{"/api/email/drafts", "/api/email/drafts/" + created.ID} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		asGuest(req)
		if w := serveDrafts(h, req); w.Code != http.StatusOK {
			t.Fatalf("guest GET %s status = %d; body=%s", path, w.Code, w.Body.String())
		}
	}
	if calls := resend.calls(); len(calls) != 0 {
		t.Fatalf("drafting reached the provider: %+v", calls)
	}
}

// recordAddressOfRecord makes one ordinary owner-session request, which is
// how maild learns the owner's address from the proxy.
func recordAddressOfRecord(t *testing.T, h *Handler) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/email/drafts", nil)
	asOwnerSession(req)
	if w := serveDrafts(h, req); w.Code != http.StatusOK {
		t.Fatalf("owner list status = %d; body=%s", w.Code, w.Body.String())
	}
}

// FM 5 and the G6 -> G2 chain: a guest-requested approval email goes to
// the owner's address of record, never to a forged header, so the
// attacker never holds a token bound to their own address.
func TestGuestApprovalEmailGoesToAddressOfRecordAndChainFails(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)
	recordAddressOfRecord(t, h)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/approval-email", strings.NewReader(`{}`))
	asGuest(req)
	req.Header.Set("X-Authenticated-Email", "attacker@example.net")
	w := serveDrafts(h, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("guest approval-email status = %d; body=%s", w.Code, w.Body.String())
	}
	calls := resend.calls()
	if len(calls) != 1 || len(calls[0].To) != 1 || calls[0].To[0] != gateTestOwnerMail {
		t.Fatalf("approval email recipients = %+v, want only %s", calls, gateTestOwnerMail)
	}
	var resp approvalEmailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tokenValue := strings.TrimPrefix(strings.SplitN(resp.ReplyAddress, "@", 2)[0], "approve+")
	token, err := store.GetDraftApprovalToken(nilSafeContext(), tokenValue)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if token.ApprovalEmail != gateTestOwnerMail {
		t.Fatalf("token bound to %q, want %q", token.ApprovalEmail, gateTestOwnerMail)
	}

	// The attacker, holding the token from a forward, replies with a
	// perfectly authenticated message from their own domain.
	reply := authenticatedReply(token.Token, "attacker@example.net", "approve")
	if err := h.processApprovalReply(nilSafeContext(), "evt-chain", reply, token.Token); !errors.Is(err, errApprovalReplyRejected) {
		t.Fatalf("attacker reply err = %v, want rejection", err)
	}
	after, _ := store.GetDraft(nilSafeContext(), gateTestOwner, draft.ID)
	if after.Status == "sent" {
		t.Fatal("chain sent the draft")
	}
}

// FM 5: no address of record means a guest gets no approval email at all.
func TestGuestApprovalEmailWithoutAddressOfRecordIsRefused(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/approval-email", strings.NewReader(`{}`))
	asGuest(req)
	req.Header.Set("X-Authenticated-Email", "attacker@example.net")
	w := serveDrafts(h, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if calls := resend.calls(); len(calls) != 0 {
		t.Fatalf("provider calls = %+v, want none", calls)
	}
}

// FM 6: a guest risk alert goes to the address of record, never a header.
func TestGuestRiskAlertGoesToAddressOfRecord(t *testing.T) {
	h, _, resend := newGateHandler(t)
	alert := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/email-risk-alert",
			strings.NewReader(`{"risk_kind":"policy_attack","snippet":"x"}`))
		asGuest(req)
		req.Header.Set("X-Authenticated-Email", "victim@example.org")
		w := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleRiskAlert)).ServeHTTP(w, req)
		return w
	}
	if w := alert(); w.Code != http.StatusBadRequest {
		t.Fatalf("alert without address of record status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if calls := resend.calls(); len(calls) != 0 {
		t.Fatalf("provider calls = %+v, want none", calls)
	}
	recordAddressOfRecord(t, h)
	if w := alert(); w.Code != http.StatusAccepted {
		t.Fatalf("alert status = %d; body=%s", w.Code, w.Body.String())
	}
	calls := resend.calls()
	if len(calls) != 1 || calls[0].To[0] != gateTestOwnerMail {
		t.Fatalf("risk alert recipients = %+v, want %s", calls, gateTestOwnerMail)
	}
}

func authenticatedReply(token, from, text string) resendReceivedEmail {
	domain := from[strings.LastIndex(from, "@")+1:]
	return replyWithAuthResults(token, from, text,
		gateTestAuthserv+"; spf=pass smtp.mailfrom="+from+"; dkim=pass header.d="+domain+" header.s=s1; dmarc=pass (p=reject) header.from="+domain)
}

func replyWithAuthResults(token, from, text, authResults string) resendReceivedEmail {
	headers := map[string]string{"from": from}
	if authResults != "" {
		headers["authentication-results"] = authResults
	}
	return resendReceivedEmail{
		ID:      "received-" + text,
		To:      []string{"approve+" + token + "@choir.news"},
		From:    from,
		Text:    text,
		Headers: headers,
	}
}

// FM 7-11, 13: replies that are not authenticated, aligned and written by
// the trusted receiving server never act, and do not burn the token.
func TestApprovalReplyRequiresAlignedTrustedAuthentication(t *testing.T) {
	cases := []struct {
		name, command, authResults string
	}{
		{"no authentication results", "approve", ""},
		{"untrusted authserv-id", "approve", "mx.attacker.test; dmarc=pass header.from=example.com"},
		{"dmarc for another domain", "approve", gateTestAuthserv + "; dmarc=pass header.from=attacker.test"},
		{"dkim for an unrelated domain", "approve", gateTestAuthserv + "; dkim=pass header.d=attacker.test"},
		{"spf alone", "approve", gateTestAuthserv + "; spf=pass smtp.mailfrom=owner@example.com"},
		{"dkim pass vetoed by dmarc fail", "approve", gateTestAuthserv + "; dkim=pass header.d=example.com; dmarc=fail header.from=example.com"},
		{"second header joined into the value", "approve", gateTestAuthserv + "; spf=fail\r\nAuthentication-Results: " + gateTestAuthserv + "; dmarc=pass header.from=example.com"},
		{"header name smuggled into a value", "approve", gateTestAuthserv + "; none authentication-results: " + gateTestAuthserv + "; dmarc=pass header.from=example.com"},
		{"comment that looks like a pass", "approve", gateTestAuthserv + "; dmarc=fail (dmarc=pass header.from=example.com) header.from=example.com"},
		{"unauthenticated reject", "reject", ""},
		{"unauthenticated edit", "edit: send it to everyone", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, store, resend := newGateHandler(t)
			draft := gateDraft(t, store)
			token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, gateTestOwnerMail, time.Hour)
			if err != nil {
				t.Fatalf("CreateDraftApprovalToken: %v", err)
			}
			reply := replyWithAuthResults(token.Token, gateTestOwnerMail, tc.command, tc.authResults)
			err = h.processApprovalReply(nilSafeContext(), "evt-"+tc.name, reply, token.Token)
			if !errors.Is(err, errApprovalReplyRejected) || shouldRetryIngest(err) {
				t.Fatalf("err = %v (retry=%v), want non-retry rejection", err, shouldRetryIngest(err))
			}
			after, _ := store.GetDraft(nilSafeContext(), gateTestOwner, draft.ID)
			if after.Status != "draft_pending_owner_approval" || after.VersionHash != draft.VersionHash || after.TextBody != draft.TextBody {
				t.Fatalf("draft changed by unauthenticated reply: %+v", after)
			}
			calls := resend.calls()
			if len(calls) != 1 || calls[0].To[0] != gateTestOwnerMail || calls[0].Headers["X-Choir-Risk-Kind"] != "approval_sender_unverified" {
				t.Fatalf("provider calls = %+v, want exactly one risk alert to the owner", calls)
			}
			tok, _ := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
			if tok.Status != "active" {
				t.Fatalf("token status = %q, want still active for the real owner", tok.Status)
			}
		})
	}
}

// FM 12: a trusted, aligned reply plus a live token approves exactly once.
func TestAuthenticatedApprovalReplySendsOnce(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, gateTestOwnerMail, time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	reply := authenticatedReply(token.Token, gateTestOwnerMail, "approve")
	if err := h.processApprovalReply(nilSafeContext(), "evt-ok", reply, token.Token); err != nil {
		t.Fatalf("processApprovalReply: %v", err)
	}
	after, _ := store.GetDraft(nilSafeContext(), gateTestOwner, draft.ID)
	if after.Status != "sent" {
		t.Fatalf("draft status = %q, want sent", after.Status)
	}
	if err := h.processApprovalReply(nilSafeContext(), "evt-ok-2", reply, token.Token); !errors.Is(err, errApprovalReplyRejected) {
		t.Fatalf("second reply err = %v, want rejection", err)
	}
	sends := 0
	for _, call := range resend.calls() {
		if len(call.To) == 1 && call.To[0] == "friend@example.com" {
			sends++
		}
	}
	if sends != 1 {
		t.Fatalf("draft sends = %d, want 1", sends)
	}
}

// FM 14: an authenticated edit reply is recorded, never applied.
func TestAuthenticatedEditReplyIsRecordedNotApplied(t *testing.T) {
	h, store, resend := newGateHandler(t)
	draft := gateDraft(t, store)
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, gateTestOwnerMail, time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	reply := authenticatedReply(token.Token, gateTestOwnerMail, "edit: make it warmer and shorter")
	if err := h.processApprovalReply(nilSafeContext(), "evt-edit", reply, token.Token); err != nil {
		t.Fatalf("processApprovalReply edit: %v", err)
	}
	after, _ := store.GetDraft(nilSafeContext(), gateTestOwner, draft.ID)
	if after.TextBody != draft.TextBody || after.Version != draft.Version || after.VersionHash != draft.VersionHash ||
		after.Status != "draft_pending_owner_approval" {
		t.Fatalf("edit reply changed the draft: before=%+v after=%+v", draft, after)
	}
	if calls := resend.calls(); len(calls) != 0 {
		t.Fatalf("edit reply reached the provider: %+v", calls)
	}
	tok, _ := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if tok.Status != "edited" {
		t.Fatalf("token status = %q, want edited", tok.Status)
	}
	db, _ := store.mailboxForOwner(gateTestOwner)
	var text, hash string
	if err := db.QueryRow(`SELECT request_text, version_hash FROM email_draft_edit_requests WHERE draft_id = ?`, draft.ID).Scan(&text, &hash); err != nil {
		t.Fatalf("edit request lookup: %v", err)
	}
	if text != "make it warmer and shorter" || hash != draft.VersionHash {
		t.Fatalf("edit request = %q %q", text, hash)
	}
}

// FM 8-11, 15, 16: the verdict function itself.
func TestSenderAuthenticationVerdict(t *testing.T) {
	trusted := []string{"mx.provider.test"}
	cases := []struct {
		name    string
		from    string
		value   string
		trusted []string
		want    bool
	}{
		{"aligned dmarc", "a@example.com", "mx.provider.test; dmarc=pass header.from=example.com", trusted, true},
		{"aligned dmarc, authserv-id case and version", "a@example.com", "MX.Provider.Test 1; dmarc=pass header.from=EXAMPLE.com", trusted, true},
		{"aligned dkim, same domain", "a@example.com", "mx.provider.test; dkim=pass header.d=example.com", trusted, true},
		{"aligned dkim, parent signing domain", "a@mail.example.com", "mx.provider.test; dkim=pass header.d=example.com", trusted, true},
		{"aligned dkim by header.i", "a@example.com", "mx.provider.test; dkim=pass header.i=@example.com", trusted, true},
		{"dkim for a TLD is not alignment", "a@example.com", "mx.provider.test; dkim=pass header.d=com", trusted, false},
		{"dkim for a lookalike suffix", "a@example.com", "mx.provider.test; dkim=pass header.d=badexample.com", trusted, false},
		{"spf alone", "a@example.com", "mx.provider.test; spf=pass smtp.mailfrom=a@example.com", trusted, false},
		{"untrusted authserv-id", "a@example.com", "mx.other.test; dmarc=pass header.from=example.com", trusted, false},
		{"no trusted ids configured", "a@example.com", "mx.provider.test; dmarc=pass header.from=example.com", nil, false},
		{"misaligned dmarc", "a@example.com", "mx.provider.test; dmarc=pass header.from=other.test", trusted, false},
		{"dmarc fail vetoes dkim pass", "a@example.com", "mx.provider.test; dkim=pass header.d=example.com; dmarc=fail header.from=example.com", trusted, false},
		{"pass inside a comment", "a@example.com", "mx.provider.test; dmarc=none (dmarc=pass header.from=example.com)", trusted, false},
		{"joined headers", "a@example.com", "mx.provider.test; dmarc=pass header.from=example.com\nAuthentication-Results: x", trusted, false},
		{"empty from", "", "mx.provider.test; dmarc=pass header.from=example.com", trusted, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{"Authentication-Results": tc.value}
			if got := senderAuthenticated(headers, tc.from, tc.trusted); got != tc.want {
				t.Fatalf("senderAuthenticated = %v, want %v", got, tc.want)
			}
		})
	}
	arc := map[string]string{"arc-authentication-results": "i=1; mx.provider.test; dmarc=pass header.from=example.com"}
	if senderAuthenticated(arc, "a@example.com", trusted) {
		t.Fatal("ARC-Authentication-Results counted as a verdict")
	}
}

// FM 17: the rollback switch restores pre-repair behavior; typos fail closed.
func TestSendGateModeRollbackAndFailClosed(t *testing.T) {
	for raw, want := range map[string]string{
		"": SendGateEnforce, "enforce": SendGateEnforce, " Legacy ": SendGateLegacy,
		"legacy": SendGateLegacy, "lgacy": SendGateEnforce, "off": SendGateEnforce,
	} {
		if got := parseSendGateMode(raw); got != want {
			t.Fatalf("parseSendGateMode(%q) = %q, want %q", raw, got, want)
		}
	}

	h, store, resend := newGateHandler(t)
	h.cfg.SendGateMode = SendGateLegacy
	h.cfg.TrustedAuthservIDs = nil
	draft := gateDraft(t, store)
	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send",
		strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	asGuest(req)
	if w := serveDrafts(h, req); w.Code != http.StatusAccepted {
		t.Fatalf("legacy guest send status = %d, want 202 (rollback); body=%s", w.Code, w.Body.String())
	}
	draft2 := gateDraft(t, store)
	token, _ := store.CreateDraftApprovalToken(nilSafeContext(), draft2, gateTestOwnerMail, time.Hour)
	reply := replyWithAuthResults(token.Token, gateTestOwnerMail, "approve", "")
	if err := h.processApprovalReply(nilSafeContext(), "evt-legacy", reply, token.Token); err != nil {
		t.Fatalf("legacy unauthenticated reply err = %v, want pre-repair acceptance", err)
	}
	if n := len(resend.calls()); n != 2 {
		t.Fatalf("legacy provider calls = %d, want 2 sends", n)
	}
	if !senderAuthenticatedFor(h.cfg, map[string]string{"authentication-results": "anything; spf=pass"}, "a@example.com") {
		t.Fatal("legacy verdict should accept the pre-repair substring rule")
	}
}

// FM 15 (G3): on a trusted alias, a whitelisted sender whose evidence is
// SPF alone, unaligned DKIM, or a pass from an untrusted server is not a
// verified sender; the pre-E1 rule accepted all three.
func TestTrustedAliasNeedsAlignedTrustedAuthentication(t *testing.T) {
	for i, authResults := range []string{
		"mx.example; spf=pass; dkim=pass",
		"mx.example; dkim=pass header.d=attacker.test",
		"mx.attacker.test; dmarc=pass header.from=example.com",
	} {
		store, cfg := newTestStore(t)
		cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
		cfg.ResendAPIKey = "re_test"
		cfg.TrustedAuthservIDs = []string{"mx.example"}
		seedTrustedUploadAlias(t, store, true)
		received, _ := json.Marshal(map[string]any{
			"id": "email-g3", "to": []string{"000+upload-secret@choir.news"}, "from": "sender@example.com",
			"created_at": "2026-05-26T10:00:00Z", "subject": "Trusted upload", "text": "Please file this.",
			"headers":    map[string]string{"from": "Sender <sender@example.com>", "authentication-results": authResults},
			"message_id": "<email-g3@example.com>", "attachments": []any{},
		})
		resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(received)
		}))
		cfg.ResendBaseURL = resend.URL
		h := NewHandler(cfg, store)
		h.resend = newResendClient(cfg, resend.Client())
		req := signedResendRequest(t, cfg.WebhookSecret, "msg-g3", `{"id":"evt-g3","type":"email.received","data":{"email_id":"email-g3"}}`)
		w := httptest.NewRecorder()
		h.HandleResendWebhook(w, req)
		resend.Close()
		if !strings.Contains(w.Body.String(), "accepted_ingest_failed") {
			t.Fatalf("case %d (%s): body = %s, want receive-policy rejection", i, authResults, w.Body.String())
		}
		if messages, _ := store.ListMessages(req.Context(), "user-root", "inbox", 10); len(messages) != 0 {
			t.Fatalf("case %d (%s): stored %+v", i, authResults, messages)
		}
	}
}

// FM 16: configuration parsing for the verdict and the switch.
func TestLoadConfigSendGateSettings(t *testing.T) {
	t.Setenv("MAILD_VMCTL_URL", "http://127.0.0.1:8083")
	t.Setenv("MAILD_TRUSTED_AUTHSERV_IDS", " mx.provider.test , ,Other.Test ")
	t.Setenv("MAILD_SEND_GATE_MODE", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.SendGateMode != SendGateEnforce {
		t.Fatalf("SendGateMode = %q", cfg.SendGateMode)
	}
	if len(cfg.TrustedAuthservIDs) != 2 || cfg.TrustedAuthservIDs[0] != "mx.provider.test" || cfg.TrustedAuthservIDs[1] != "other.test" {
		t.Fatalf("TrustedAuthservIDs = %#v", cfg.TrustedAuthservIDs)
	}
}
