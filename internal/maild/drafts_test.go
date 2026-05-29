package maild

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestDraftCreateRequiresOwnedAliasAndDoesNotSend(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts", strings.NewReader(`{
		"from_address":"000@choir.news",
		"to_addresses":["friend@example.com"],
		"subject":"Choir demo",
		"text_body":"Draft first.",
		"source_kind":"vtext_email_artifact",
		"source_ref":"doc-1:rev-1"
	}`))
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var draft draftResponse
	if err := json.NewDecoder(w.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if draft.Status != "draft_pending_owner_approval" || draft.VersionHash == "" {
		t.Fatalf("draft response: %+v", draft)
	}
	messages, err := store.ListMessages(req.Context(), "user-root", "sent", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("draft creation sent messages: %+v", messages)
	}
}

func TestDraftCreateDefaultsToOwnerNumericAlias(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts", strings.NewReader(`{
		"to_addresses":["friend@example.com"],
		"subject":"Choir demo",
		"text_body":"Draft first.",
		"source_kind":"vtext_email_artifact",
		"source_ref":"doc-1:rev-1"
	}`))
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var draft draftResponse
	if err := json.NewDecoder(w.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if draft.FromAddress != "000@choir.news" || draft.Status != "draft_pending_owner_approval" {
		t.Fatalf("draft response: %+v", draft)
	}
}

func TestDraftSendStoresSentAndPreventsSecondSend(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.From != "000@choir.news" || payload.To[0] != "friend@example.com" || !strings.HasPrefix(payload.Text, "Approved body.\n\n--\n") || !strings.Contains(payload.Text, choirAutomatedEmailSignature) {
			t.Fatalf("payload = %+v", payload)
		}
		if payload.HTML == "" || strings.Contains(payload.HTML, "## Workflow") || !strings.Contains(payload.HTML, "Approved body.") || !strings.Contains(payload.HTML, "<footer") || !strings.Contains(payload.HTML, choirAutomatedEmailSignature) {
			t.Fatalf("payload HTML = %q", payload.HTML)
		}
		if payload.Headers["X-Choir-Maild"] != "v0-approved-draft-send" ||
			payload.Headers["X-Choir-Email-Draft-ID"] == "" ||
			payload.Headers["X-Choir-Email-Draft-Version-Hash"] == "" {
			t.Fatalf("draft send headers = %+v", payload.Headers)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-draft-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news",
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Approved",
		TextBody:    "Approved body.\n\n## Workflow\n\n1. Internal artifact notes must not be sent.",
		SourceKind:  "vtext_email_artifact",
		SourceRef:   "doc-1:rev-1",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send", strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp sendDraftResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode send draft response: %v", err)
	}
	if resp.Status != "sent" || resp.ProviderMessageID != "sent-draft-1" || resp.Draft.Status != "sent" {
		t.Fatalf("send response: %+v", resp)
	}
	if resp.ApprovalEventID == "" {
		t.Fatalf("send response missing approval event: %+v", resp)
	}
	msg, err := store.GetMessage(req.Context(), "user-root", resp.MessageID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if !strings.HasPrefix(msg.TextBody, "Approved body.\n\n--\n") || !strings.Contains(msg.TextBody, choirAutomatedEmailSignature) || msg.HTMLBody == "" || strings.Contains(msg.HTMLBody, "## Workflow") || !strings.Contains(msg.HTMLBody, "<footer") {
		t.Fatalf("stored sent message body = text %q html %q", msg.TextBody, msg.HTMLBody)
	}
	approvalCount, err := store.CountDraftApprovalEvents(req.Context(), draft.ID)
	if err != nil {
		t.Fatalf("CountDraftApprovalEvents: %v", err)
	}
	if approvalCount != 1 {
		t.Fatalf("approval events = %d, want 1", approvalCount)
	}
	messages, err := store.ListMessages(req.Context(), "user-root", "sent", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].Subject != "Approved" {
		t.Fatalf("sent messages = %+v", messages)
	}
	drafts, err := store.ListDrafts(req.Context(), "user-root", 10)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("sent draft still listed in Drafts: %+v", drafts)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send", strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	setInternalOwner(req, "user-root")
	w = httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("second send status = %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestDraftSendMarksActiveApprovalTokensStale(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-draft-with-token"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Approved with token",
		TextBody:    "Approved body.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}

	if _, err := h.sendApprovedDraft(nilSafeContext(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", ""); err != nil {
		t.Fatalf("sendApprovedDraft: %v", err)
	}
	used, err := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if used.Status != "stale_sent" || used.UsedAt == "" {
		t.Fatalf("token after send = %+v, want stale_sent with used_at", used)
	}
}

func TestDraftSendEmitsBoundedEmailAppagentTraceEvents(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-trace-send-1"}`))
	}))
	defer resend.Close()

	var mu sync.Mutex
	var tracePosts []map[string]any
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/runtime/runs/email-run-1/events" {
			t.Fatalf("runtime request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header: %+v", r.Header)
		}
		if r.URL.Query().Get("owner_id") != "user-root" {
			t.Fatalf("owner query = %q", r.URL.Query().Get("owner_id"))
		}
		var post map[string]any
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			t.Fatalf("decode runtime post: %v", err)
		}
		raw, _ := json.Marshal(post)
		if strings.Contains(string(raw), "Approved body.") || strings.Contains(string(raw), "secret-approval-token") {
			t.Fatalf("trace post leaked risky content: %s", raw)
		}
		mu.Lock()
		tracePosts = append(tracePosts, post)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"appended","event_id":"runtime-event-1","kind":"email.draft.sent"}`))
	}))
	defer runtime.Close()

	cfg.ResendBaseURL = resend.URL
	cfg.RuntimeURL = runtime.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news",
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Trace send",
		TextBody:    "Approved body.",
		SourceKind:  "vtext_email_artifact",
		SourceRef:   `{"email_appagent_run_id":"email-run-1","doc_id":"doc-1","revision_id":"rev-1","source_content_hash":"sha256:test"}`,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	resp, err := h.sendApprovedDraft(nilSafeContext(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", "approval-notice-1")
	if err != nil {
		t.Fatalf("sendApprovedDraft: %v", err)
	}
	if resp.ProviderMessageID != "provider-trace-send-1" || resp.ApprovalEventID == "" {
		t.Fatalf("send response = %+v", resp)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(tracePosts) != 2 {
		t.Fatalf("trace post count = %d, want 2; posts=%+v", len(tracePosts), tracePosts)
	}
	if tracePosts[0]["kind"] != emailTraceEventApprovalRecorded || tracePosts[1]["kind"] != emailTraceEventSent {
		t.Fatalf("trace post kinds = %#v, %#v", tracePosts[0]["kind"], tracePosts[1]["kind"])
	}
	for _, post := range tracePosts {
		if post["owner_id"] != "user-root" || post["phase"] != "email_appagent_evidence" {
			t.Fatalf("trace post envelope = %+v", post)
		}
		payload, ok := post["payload"].(map[string]any)
		if !ok {
			t.Fatalf("payload type = %T", post["payload"])
		}
		if payload["authority"] != "email_appagent" || payload["maild_role"] != "transport_evidence" {
			t.Fatalf("trace payload authority = %+v", payload)
		}
		if payload["draft_id"] != draft.ID || payload["draft_version_hash"] != draft.VersionHash ||
			payload["approval_event_type"] != "owner_click_approved" ||
			payload["approval_provider_message_id"] != "approval-notice-1" {
			t.Fatalf("trace payload = %+v", payload)
		}
	}
	sentPayload := tracePosts[1]["payload"].(map[string]any)
	if sentPayload["sent_message_id"] == "" || sentPayload["provider_message_id"] != "provider-trace-send-1" || sentPayload["send_authorized"] != true {
		t.Fatalf("sent trace payload = %+v", sentPayload)
	}
}

func TestDraftSendResolvesOwnerRuntimeThroughVmctlForTraceEvents(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-vmctl-trace-send-1"}`))
	}))
	defer resend.Close()

	wrongRuntimeCalled := false
	wrongRuntime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrongRuntimeCalled = true
		http.Error(w, "wrong runtime", http.StatusNotFound)
	}))
	defer wrongRuntime.Close()

	var mu sync.Mutex
	var tracePosts []map[string]any
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/runtime/runs/email-run-vmctl/events" {
			t.Fatalf("runtime request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("owner_id") != "user-root" {
			t.Fatalf("owner query = %q", r.URL.Query().Get("owner_id"))
		}
		var post map[string]any
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			t.Fatalf("decode runtime post: %v", err)
		}
		mu.Lock()
		tracePosts = append(tracePosts, post)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"appended"}`))
	}))
	defer runtime.Close()

	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/vmctl/resolve" {
			t.Fatalf("vmctl request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing vmctl internal caller header: %+v", r.Header)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode vmctl request: %v", err)
		}
		if req["user_id"] != "user-root" || req["desktop_id"] != "primary" {
			t.Fatalf("vmctl request body = %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"vm_id":"vm-1","user_id":"user-root","desktop_id":"primary","published":true,"sandbox_url":"` + runtime.URL + `","state":"active"}`))
	}))
	defer vmctlServer.Close()

	cfg.ResendBaseURL = resend.URL
	cfg.RuntimeURL = wrongRuntime.URL
	cfg.VmctlURL = vmctlServer.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news",
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Vmctl trace send",
		TextBody:    "Approved body.",
		SourceKind:  "vtext_email_artifact",
		SourceRef:   `{"email_appagent_run_id":"email-run-vmctl","doc_id":"doc-1","revision_id":"rev-1","source_content_hash":"sha256:test"}`,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if _, err := h.sendApprovedDraft(nilSafeContext(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", "approval-notice-1"); err != nil {
		t.Fatalf("sendApprovedDraft: %v", err)
	}
	if wrongRuntimeCalled {
		t.Fatalf("maild posted trace evidence to static MAILD_RUNTIME_URL despite vmctl owner resolution")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(tracePosts) != 2 {
		t.Fatalf("trace post count = %d, want 2; posts=%+v", len(tracePosts), tracePosts)
	}
	if tracePosts[0]["kind"] != emailTraceEventApprovalRecorded || tracePosts[1]["kind"] != emailTraceEventSent {
		t.Fatalf("trace post kinds = %#v, %#v", tracePosts[0]["kind"], tracePosts[1]["kind"])
	}
}

func TestDraftSendRejectsMissingOrStaleVersionHash(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	h := NewHandler(cfg, store)
	alias, err := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news",
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Approved",
		TextBody:    "Approved body.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	for _, body := range []string{`{}`, `{"version_hash":"stale"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send", strings.NewReader(body))
		setInternalOwner(req, "user-root")
		w := httptest.NewRecorder()
		h.HandleDrafts(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("body %s status = %d, want %d; response=%s", body, w.Code, http.StatusConflict, w.Body.String())
		}
	}
	count, err := store.CountDraftApprovalEvents(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("CountDraftApprovalEvents: %v", err)
	}
	if count != 0 {
		t.Fatalf("approval events after rejected send = %d, want 0", count)
	}
}

func TestDraftApprovalEmailUsesVerifiedSignupEmailAndReplyToken(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"approval-notice-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Needs approval",
		TextBody:    "Draft body.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/approval-email", strings.NewReader(`{}`))
	setInternalOwner(req, "user-root")
	req.Header.Set("X-Authenticated-Email", "owner@example.com")
	w := httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp approvalEmailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ProviderMessageID != "approval-notice-1" || !strings.Contains(resp.ReplyAddress, "approve+") {
		t.Fatalf("approval response = %+v", resp)
	}
	replyLocal := strings.SplitN(resp.ReplyAddress, "@", 2)[0]
	if len(replyLocal) > 64 {
		t.Fatalf("approval reply local part length = %d, want <= 64: %s", len(replyLocal), resp.ReplyAddress)
	}
	if payload.To[0] != "owner@example.com" || len(payload.ReplyTo) != 1 || payload.ReplyTo[0] != resp.ReplyAddress {
		t.Fatalf("approval payload = %+v response=%+v", payload, resp)
	}
	if !strings.Contains(payload.Text, "From: 000@choir.news") ||
		!strings.Contains(payload.Text, "To: friend@example.com") ||
		!strings.Contains(payload.Text, "Subject: Needs approval") ||
		!strings.Contains(payload.Text, "Draft message:\nDraft body.") ||
		!strings.Contains(payload.Text, "Open in Choir to review and send:") ||
		!strings.Contains(payload.Text, "approve\nreject\nedit: <requested change>") {
		t.Fatalf("approval payload text missing reviewable draft content: %q", payload.Text)
	}
	if strings.Contains(payload.Text, draft.VersionHash) || strings.Contains(payload.Text, "user-root") {
		t.Fatalf("approval payload text = %q", payload.Text)
	}
	if payload.Headers["X-Choir-Email-Draft-ID"] != draft.ID ||
		payload.Headers["X-Choir-Email-Draft-Version-Hash"] != draft.VersionHash {
		t.Fatalf("approval payload headers = %+v", payload.Headers)
	}
}

func TestApprovalEmailDraftBodyPreviewIsBounded(t *testing.T) {
	short := approvalEmailDraftBodyPreview("  hello owner  ")
	if short != "hello owner" {
		t.Fatalf("short preview = %q", short)
	}
	if got := approvalEmailDraftBodyPreview(""); got != "(No plain text body.)" {
		t.Fatalf("empty preview = %q", got)
	}
	longBody := strings.Repeat("x", approvalEmailDraftBodyPreviewRunes) + "SECRET_TAIL"
	preview := approvalEmailDraftBodyPreview(longBody)
	if strings.Contains(preview, "SECRET_TAIL") || !strings.Contains(preview, "preview truncated") {
		t.Fatalf("long preview = %q", preview)
	}
}

func TestApprovalReplyApprovesExactDraftVersionOnce(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	sendCount := 0
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"reply-send-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Reply approve",
		TextBody:    "Approved by email.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}

	email := resendReceivedEmail{
		ID:   "received-approval-1",
		To:   []string{"approve+" + token.Token + "@choir.news"},
		From: "owner@example.com",
		Text: "approve",
		Headers: map[string]string{
			"from": "owner@example.com",
		},
	}
	if err := h.processApprovalReply(nilSafeContext(), "event-approval-1", email, token.Token); err != nil {
		t.Fatalf("processApprovalReply: %v", err)
	}
	updated, err := store.GetDraft(nilSafeContext(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if updated.Status != "sent" || updated.ProviderMessageID != "reply-send-1" || sendCount != 1 {
		t.Fatalf("updated=%+v sendCount=%d", updated, sendCount)
	}
	if err := h.processApprovalReply(nilSafeContext(), "event-approval-2", email, token.Token); err == nil {
		t.Fatal("second approval reply succeeded; want one-time token rejection")
	}
}

func TestApprovalReplyDenyRejectsDraftWithoutSending(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	sendCount := 0
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"unexpected-send"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Reply deny",
		TextBody:    "Should not be sent.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}

	email := resendReceivedEmail{
		ID:      "received-deny-reply",
		To:      []string{"approve+" + token.Token + "@choir.news"},
		From:    "owner@example.com",
		Text:    "Deny",
		Headers: map[string]string{"from": "owner@example.com"},
	}
	if err := h.processApprovalReply(nilSafeContext(), "event-deny-reply", email, token.Token); err != nil {
		t.Fatalf("processApprovalReply deny: %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("deny reply sent email; sendCount=%d", sendCount)
	}
	updated, err := store.GetDraft(nilSafeContext(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if updated.Status == "sent" || updated.ProviderMessageID != "" {
		t.Fatalf("deny reply sent draft: %+v", updated)
	}
	used, err := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if used.Status != "rejected" {
		t.Fatalf("token status = %q, want rejected", used.Status)
	}
	var eventType string
	if err := store.db.QueryRowContext(nilSafeContext(), `SELECT event_type FROM email_draft_approval_events WHERE draft_id = ?`, draft.ID).Scan(&eventType); err != nil {
		t.Fatalf("approval event lookup: %v", err)
	}
	if eventType != "email_reply_rejected" {
		t.Fatalf("event_type = %q, want email_reply_rejected", eventType)
	}
}

func TestApprovalReplySenderMismatchBlocksRetryAndSendsRiskAlert(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"risk-alert-sender-mismatch"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Sender mismatch",
		TextBody:    "Do not send from attacker.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}

	email := resendReceivedEmail{
		ID:   "received-approval-attacker",
		To:   []string{"approve+" + token.Token + "@choir.news"},
		From: "attacker@example.com",
		Text: "approve",
		Headers: map[string]string{
			"from": "attacker@example.com",
		},
	}
	err = h.processApprovalReply(nilSafeContext(), "event-approval-attacker", email, token.Token)
	if !errors.Is(err, errApprovalReplyRejected) {
		t.Fatalf("processApprovalReply error = %v, want errApprovalReplyRejected", err)
	}
	if shouldRetryIngest(err) {
		t.Fatalf("sender mismatch should be a blocked non-retry decision: %v", err)
	}
	updated, err := store.GetDraft(nilSafeContext(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if updated.Status == "sent" || updated.ProviderMessageID != "" {
		t.Fatalf("sender mismatch sent draft: %+v", updated)
	}
	if payload.Subject != "[Choir Risk Alert] Email draft blocked" || payload.To[0] != "owner@example.com" {
		t.Fatalf("risk alert payload = %+v", payload)
	}
	if payload.Headers["X-Choir-Risk-Kind"] != "approval_sender_mismatch" {
		t.Fatalf("risk headers = %+v", payload.Headers)
	}
	if strings.Contains(payload.Text, "user-root") || !strings.Contains(payload.Text, "attacker@example.com") {
		t.Fatalf("risk alert text = %q", payload.Text)
	}
}

func TestApprovalReplyEditCreatesNewVersionAndInvalidatesOldToken(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payloads []resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		payloads = append(payloads, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-call-` + string(rune('0'+len(payloads))) + `"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Edit reply",
		TextBody:    "Original body.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	editReply := resendReceivedEmail{
		ID:      "received-edit-reply",
		To:      []string{"approve+" + token.Token + "@choir.news"},
		From:    "owner@example.com",
		Text:    "edit: make it warmer and shorter",
		Headers: map[string]string{"from": "owner@example.com"},
	}
	if err := h.processApprovalReply(nilSafeContext(), "event-edit-reply", editReply, token.Token); err != nil {
		t.Fatalf("processApprovalReply edit: %v", err)
	}
	updated, err := store.GetDraft(nilSafeContext(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if updated.Status != "draft_pending_owner_approval" || updated.Version != draft.Version+1 || updated.VersionHash == draft.VersionHash {
		t.Fatalf("updated draft = %+v, original=%+v", updated, draft)
	}
	if !strings.Contains(updated.TextBody, "make it warmer and shorter") || updated.ProviderMessageID != "" {
		t.Fatalf("updated draft body/provider = %+v", updated)
	}
	if len(payloads) != 1 {
		t.Fatalf("payload count after edit = %d, want fresh approval email; payloads=%+v", len(payloads), payloads)
	}
	if payloads[0].Subject != "Choir email draft needs approval: Edit reply" ||
		payloads[0].Headers["X-Choir-Maild"] != "v0-email-draft-approval" ||
		payloads[0].Headers["X-Choir-Email-Draft-Version-Hash"] != updated.VersionHash {
		t.Fatalf("fresh approval payload = %+v updated=%+v", payloads[0], updated)
	}
	if !strings.Contains(payloads[0].Text, "Draft message:") ||
		!strings.Contains(payloads[0].Text, "Owner approval reply requested edits:") ||
		!strings.Contains(payloads[0].Text, "make it warmer and shorter") {
		t.Fatalf("approval email should embed the current draft body for review: %q", payloads[0].Text)
	}
	if strings.Contains(payloads[0].Text, updated.VersionHash) || strings.Contains(payloads[0].Text, "user-root") {
		t.Fatalf("approval email should keep internal ids and hashes out of human body: %q", payloads[0].Text)
	}
	used, err := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if used.Status != "edited" {
		t.Fatalf("old token status = %q, want edited", used.Status)
	}
	var activeTokenCount int
	if err := store.db.QueryRowContext(nilSafeContext(), `SELECT count(*) FROM email_draft_approval_tokens WHERE draft_id = ? AND status = 'active' AND version_hash = ?`, draft.ID, updated.VersionHash).Scan(&activeTokenCount); err != nil {
		t.Fatalf("active token count query: %v", err)
	}
	if activeTokenCount != 1 {
		t.Fatalf("active token count for edited version = %d, want 1", activeTokenCount)
	}

	approveOld := editReply
	approveOld.ID = "received-approve-old-token"
	approveOld.Text = "approve"
	err = h.processApprovalReply(nilSafeContext(), "event-approve-old-token", approveOld, token.Token)
	if !errors.Is(err, errApprovalReplyRejected) || shouldRetryIngest(err) {
		t.Fatalf("old token approval err=%v retry=%v", err, shouldRetryIngest(err))
	}
	if len(payloads) != 2 {
		t.Fatalf("payload count after old-token approval = %d, want approval + risk alert; payloads=%+v", len(payloads), payloads)
	}
	if payloads[1].Headers["X-Choir-Risk-Kind"] != "approval_token_not_active" {
		t.Fatalf("risk alert payload after old token approval = %+v", payloads[1])
	}
}

func TestApprovalReplyAfterOwnerClickSendIsBlockedNonRetry(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payloads []resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		payloads = append(payloads, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-call-` + string(rune('0'+len(payloads))) + `"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())
	alias, _ := store.ResolveAlias(nilSafeContext(), "choir.news", "000")
	draft, err := store.CreateDraft(nilSafeContext(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Already sent",
		TextBody:    "Owner clicked first.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(nilSafeContext(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	if _, err := h.sendApprovedDraft(nilSafeContext(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", ""); err != nil {
		t.Fatalf("owner click send: %v", err)
	}
	used, err := store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken after owner click: %v", err)
	}
	if used.Status != "stale_sent" {
		t.Fatalf("token status after owner click = %q, want stale_sent", used.Status)
	}

	email := resendReceivedEmail{
		ID:      "received-approve-after-click",
		To:      []string{"approve+" + token.Token + "@choir.news"},
		From:    "owner@example.com",
		Text:    "approve",
		Headers: map[string]string{"from": "owner@example.com"},
	}
	err = h.processApprovalReply(nilSafeContext(), "event-approve-after-click", email, token.Token)
	if !errors.Is(err, errApprovalReplyRejected) || shouldRetryIngest(err) {
		t.Fatalf("approve after click err=%v retry=%v", err, shouldRetryIngest(err))
	}
	if len(payloads) != 2 {
		t.Fatalf("payload count = %d, want send + risk alert; payloads=%+v", len(payloads), payloads)
	}
	if payloads[1].Subject != "[Choir Risk Alert] Email draft blocked" ||
		payloads[1].Headers["X-Choir-Risk-Kind"] != "approval_token_not_active" {
		t.Fatalf("risk alert payload = %+v", payloads[1])
	}
	used, err = store.GetDraftApprovalToken(nilSafeContext(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if used.Status != "stale_sent" {
		t.Fatalf("old token status = %q, want stale_sent", used.Status)
	}
}

func TestRegisteredRoutesDoNotExposeRawEmailSend(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	s := server.NewServer("maild-test", "0")
	RegisterRoutes(s, h)

	req := httptest.NewRequest(http.MethodPost, "/api/email/send", strings.NewReader(`{
		"from_address":"000@choir.news",
		"to_addresses":["friend@example.com"],
		"subject":"raw send",
		"text_body":"This route should not exist."
	}`))
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("registered /api/email/send status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func nilSafeContext() context.Context {
	return context.Background()
}
