package maild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uploadAttachment(t *testing.T, h *Handler, ownerID, filename, contentType string, body []byte) (EmailAttachmentMeta, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/email/attachments", bytes.NewReader(body))
	req.Header.Set("X-Choir-Filename", filename)
	req.Header.Set("Content-Type", contentType)
	setInternalOwner(req, ownerID)
	w := httptest.NewRecorder()
	h.HandleAttachments(w, req)
	var meta EmailAttachmentMeta
	if w.Code == http.StatusCreated {
		if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
			t.Fatalf("decode upload: %v", err)
		}
	}
	return meta, w.Code
}

func TestAttachmentUploadStagesBytesAndMetadata(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	body := []byte("hello attachment bytes")
	meta, code := uploadAttachment(t, h, "user-root", "note.txt", "text/plain", body)
	if code != http.StatusCreated {
		t.Fatalf("upload status = %d", code)
	}
	if meta.ID == "" || meta.Filename != "note.txt" || meta.SizeBytes != int64(len(body)) || meta.SHA256 == "" {
		t.Fatalf("meta = %+v", meta)
	}
	sum := sha256.Sum256(body)
	if meta.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %q", meta.SHA256)
	}
	// Bytes on disk under the owner's outbound dir.
	att, err := store.GetOutboundAttachment(context.Background(), "user-root", meta.ID)
	if err != nil {
		t.Fatalf("GetOutboundAttachment: %v", err)
	}
	if att.Status != attachmentStatusStaged {
		t.Fatalf("status = %q", att.Status)
	}
	got, err := store.ReadOutboundAttachmentBytes("user-root", att)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("stored bytes mismatch: %v", err)
	}
}

func TestAttachmentUploadEnforcesPerFileCap(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.AttachmentMaxBytes = 16
	h := NewHandler(cfg, store)
	_, code := uploadAttachment(t, h, "user-root", "big.bin", "application/octet-stream", bytes.Repeat([]byte("x"), 17))
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", code)
	}
}

func TestAttachmentUploadRequiresFilename(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	_, code := uploadAttachment(t, h, "user-root", "", "text/plain", []byte("x"))
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestAttachmentFilenameSanitized(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	meta, code := uploadAttachment(t, h, "user-root", "../../etc/passwd", "text/plain", []byte("x"))
	if code != http.StatusCreated {
		t.Fatalf("status = %d", code)
	}
	if meta.Filename != "passwd" {
		t.Fatalf("filename = %q", meta.Filename)
	}
}

func TestAttachmentDownloadAndDelete(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	body := []byte("download me")
	meta, _ := uploadAttachment(t, h, "user-root", "dl.txt", "text/plain", body)

	req := httptest.NewRequest(http.MethodGet, "/api/email/attachments/"+meta.ID, nil)
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	h.HandleAttachments(w, req)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), body) {
		t.Fatalf("download status=%d body=%q", w.Code, w.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/email/attachments/"+meta.ID, nil)
	setInternalOwner(del, "user-root")
	dw := httptest.NewRecorder()
	h.HandleAttachments(dw, del)
	if dw.Code != http.StatusOK {
		t.Fatalf("delete status = %d", dw.Code)
	}
	// Bytes removed.
	if _, err := store.GetOutboundAttachment(context.Background(), "user-root", meta.ID); err == nil {
		t.Fatal("attachment still present after delete")
	}
}

func TestAttachmentIsolationAcrossOwners(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	meta, _ := uploadAttachment(t, h, "user-root", "secret.txt", "text/plain", []byte("secret"))
	// Another owner cannot read it.
	req := httptest.NewRequest(http.MethodGet, "/api/email/attachments/"+meta.ID, nil)
	setInternalOwner(req, "user-other")
	w := httptest.NewRecorder()
	h.HandleAttachments(w, req)
	if w.Code == http.StatusOK {
		t.Fatal("cross-owner attachment read succeeded")
	}
}

func createDraftWithAttachments(t *testing.T, store *Store, ownerID string, ids []string) EmailDraft {
	t.Helper()
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(context.Background(), ownerID, alias, createDraftRequest{
		FromAddress:   "000@choir.news",
		ToAddresses:   []string{"friend@example.com"},
		Subject:       "with attachment",
		TextBody:      "see attached",
		AttachmentIDs: ids,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return draft
}

func TestDraftCreateBindsAttachmentsAndCoversVersionHash(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	meta, _ := uploadAttachment(t, h, "user-root", "a.txt", "text/plain", []byte("aaa"))

	draft := createDraftWithAttachments(t, store, "user-root", []string{meta.ID})
	if draft.AttachmentsJSON == "" {
		t.Fatal("draft missing attachments_json")
	}
	refs := loadDraftAttachmentRefs(draft)
	if len(refs) != 1 || refs[0].ID != meta.ID || refs[0].SHA256 != meta.SHA256 {
		t.Fatalf("refs = %+v", refs)
	}
	// Attachment row flipped to bound.
	att, err := store.GetOutboundAttachment(context.Background(), "user-root", meta.ID)
	if err != nil || att.Status != attachmentStatusBound {
		t.Fatalf("bound status = %v %q", err, att.Status)
	}
	// Version hash covers the attachment set: a draft with no attachments and a
	// draft with a different attachment must hash differently.
	draftNoAtt := createDraftWithAttachments(t, store, "user-root", nil)
	if draft.VersionHash == draftNoAtt.VersionHash {
		t.Fatal("version hash ignores attachments")
	}
}

func TestDraftCreateRejectsUnstagedOrMissingAttachment(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	// Missing id.
	alias, _ := store.ResolveAlias(context.Background(), "choir.news", "000")
	_, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news", ToAddresses: []string{"f@e.com"},
		Subject: "s", TextBody: "b", AttachmentIDs: []string{"email-attachment-nope"},
	})
	if err == nil {
		t.Fatal("expected error for missing attachment")
	}
	// Bound attachment cannot be re-bound.
	meta, _ := uploadAttachment(t, h, "user-root", "b.txt", "text/plain", []byte("b"))
	createDraftWithAttachments(t, store, "user-root", []string{meta.ID})
	_, err = store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news", ToAddresses: []string{"f@e.com"},
		Subject: "s", TextBody: "b", AttachmentIDs: []string{meta.ID},
	})
	if err == nil {
		t.Fatal("expected error re-binding a bound attachment")
	}
}

func TestDraftAttachmentCaps(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.AttachmentMaxBytes = 1 << 20
	cfg.DraftAttachmentMaxBytes = 10
	h := NewHandler(cfg, store)
	var ids []string
	for i := range 3 {
		meta, _ := uploadAttachment(t, h, "user-root", fmt.Sprintf("f%d.txt", i), "text/plain", bytes.Repeat([]byte("x"), 5))
		ids = append(ids, meta.ID)
	}
	alias, _ := store.ResolveAlias(context.Background(), "choir.news", "000")
	_, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		FromAddress: "000@choir.news", ToAddresses: []string{"f@e.com"},
		Subject: "s", TextBody: "b", AttachmentIDs: ids,
	})
	if err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("expected total-cap error, got %v", err)
	}
}

func TestSendApprovedDraftTampersFailsVersionBoundary(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	meta, _ := uploadAttachment(t, h, "user-root", "doc.txt", "text/plain", []byte("original"))
	draft := createDraftWithAttachments(t, store, "user-root", []string{meta.ID})

	// Tamper the staged bytes after approval.
	att, _ := store.GetOutboundAttachment(context.Background(), "user-root", meta.ID)
	if err := os.WriteFile(filepath.Join(cfg.StorageRoot, att.StorageRef), []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	_, err := h.sendApprovedDraft(context.Background(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", "")
	if err == nil || !strings.Contains(err.Error(), "attachment") {
		t.Fatalf("expected attachment tamper refusal, got %v", err)
	}
}

func TestSendApprovedDraftEncodesAttachments(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := []byte("attachment payload")
	meta, _ := uploadAttachment(t, h, "user-root", "doc.txt", "text/plain", body)
	draft := createDraftWithAttachments(t, store, "user-root", []string{meta.ID})

	resp, err := h.sendApprovedDraft(context.Background(), "user-root", draft.ID, draft.VersionHash, "owner_click_approved", "")
	if err != nil {
		t.Fatalf("sendApprovedDraft: %v", err)
	}
	if resp.Status != "sent" {
		t.Fatalf("status = %q", resp.Status)
	}
	if len(payload.Attachments) != 1 || payload.Attachments[0].Filename != "doc.txt" {
		t.Fatalf("resend attachments = %+v", payload.Attachments)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Attachments[0].Content)
	if err != nil || !bytes.Equal(decoded, body) {
		t.Fatalf("resend attachment content mismatch")
	}
	// Attachment row marked sent and linked to the outbound message.
	att, _ := store.GetOutboundAttachment(context.Background(), "user-root", meta.ID)
	if att.Status != attachmentStatusSent || att.MessageID != resp.MessageID {
		t.Fatalf("sent attachment = %+v", att)
	}
}

func TestSweepStaleStagedAttachments(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	meta, _ := uploadAttachment(t, h, "user-root", "old.txt", "text/plain", []byte("old"))
	// Age the row beyond the TTL.
	db, _ := store.mailboxForOwner("user-root")
	old := "2000-01-01T00:00:00Z"
	if _, err := db.Exec(`UPDATE email_attachments SET created_at = ? WHERE id = ?`, old, meta.ID); err != nil {
		t.Fatalf("age row: %v", err)
	}
	if n := store.SweepStaleStagedAttachments(context.Background(), "user-root", stagedAttachmentTTL); n != 1 {
		t.Fatalf("swept = %d", n)
	}
}
