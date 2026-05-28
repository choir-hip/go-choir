package maild

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.From != "000@choir.news" || payload.To[0] != "friend@example.com" || payload.Text != "Approved body." {
			t.Fatalf("payload = %+v", payload)
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
		TextBody:    "Approved body.",
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

	req = httptest.NewRequest(http.MethodPost, "/api/email/drafts/"+draft.ID+"/send", strings.NewReader(`{"version_hash":"`+draft.VersionHash+`"}`))
	setInternalOwner(req, "user-root")
	w = httptest.NewRecorder()
	h.HandleDrafts(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("second send status = %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
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
	if !strings.Contains(payload.Text, draft.VersionHash) || strings.Contains(payload.Text, "user-root") {
		t.Fatalf("approval payload text = %q", payload.Text)
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
