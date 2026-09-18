package maild

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Outbound attachment lifecycle: staged (uploaded, unbound) -> bound (draft
// create) -> sent (draft send). Staged bytes live on disk under
// StorageRoot/attachments/outbound/<ownerID>/<id>; metadata lives in the
// owner's per-mailbox email_attachments table (direction='outbound').
const (
	attachmentDirectionOutbound = "outbound"
	attachmentStatusStaged      = "staged"
	attachmentStatusBound       = "bound"
	attachmentStatusSent        = "sent"

	// DefaultAttachmentMaxBytes caps a single uploaded file (15 MiB).
	DefaultAttachmentMaxBytes = 15 << 20
	// DefaultDraftAttachmentMaxBytes caps total attachment bytes per draft
	// (25 MiB, under Resend's ~40 MB request ceiling).
	DefaultDraftAttachmentMaxBytes = 25 << 20
	// MaxDraftAttachments caps the number of attachments per draft.
	MaxDraftAttachments = 10
	// stagedAttachmentTTL is how long an unbound staged attachment is kept
	// before the lazy sweeper removes it.
	stagedAttachmentTTL = 24 * time.Hour
)

// EmailAttachmentMeta is the owner-facing attachment record returned by the
// upload endpoint and embedded in draft/message responses.
type EmailAttachmentMeta struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256,omitempty"`
	Status      string `json:"status,omitempty"`
}

// draftAttachmentRef is the hash-bound record stored in
// email_drafts.attachments_json and covered by version_hash.
type draftAttachmentRef struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
}

func (h *Handler) attachmentMaxBytes() int64 {
	if h != nil && h.cfg != nil && h.cfg.AttachmentMaxBytes > 0 {
		return h.cfg.AttachmentMaxBytes
	}
	return DefaultAttachmentMaxBytes
}

func (h *Handler) draftAttachmentMaxBytes() int64 {
	if h != nil && h.cfg != nil && h.cfg.DraftAttachmentMaxBytes > 0 {
		return h.cfg.DraftAttachmentMaxBytes
	}
	return DefaultDraftAttachmentMaxBytes
}

// HandleAttachments routes /api/email/attachments and
// /api/email/attachments/{id}.
func (h *Handler) HandleAttachments(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := authenticatedInternalOwner(w, r)
	if !ok {
		return
	}
	if r.URL.Path == "/api/email/attachments" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleAttachmentUpload(w, r, ownerID)
		return
	}
	const prefix = "/api/email/attachments/"
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if id == "" || id == r.URL.Path || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.handleAttachmentDownload(w, r, ownerID, id)
	case http.MethodDelete:
		h.handleAttachmentDelete(w, r, ownerID, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleAttachmentUpload stores raw request bytes as a staged outbound
// attachment. Mirrors the /api/files PUT convention: raw body +
// X-Choir-Filename + Content-Type, not multipart.
func (h *Handler) handleAttachmentUpload(w http.ResponseWriter, r *http.Request, ownerID string) {
	maxBytes := h.attachmentMaxBytes()
	filename := sanitizeAttachmentFilename(r.Header.Get("X-Choir-Filename"))
	if filename == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-Choir-Filename is required"})
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" || strings.EqualFold(contentType, "application/x-www-form-urlencoded") {
		contentType = "application/octet-stream"
	}
	// Lazy GC of orphaned staged attachments before admitting a new one.
	_ = h.store.SweepStaleStagedAttachments(r.Context(), ownerID, stagedAttachmentTTL)

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read upload"})
		return
	}
	if int64(len(body)) > maxBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("attachment exceeds %d bytes", maxBytes)})
		return
	}
	if len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "attachment body is empty"})
		return
	}
	sum := sha256.Sum256(body)
	meta := EmailAttachmentMeta{
		ID:          "email-attachment-" + uuid.NewString(),
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   int64(len(body)),
		SHA256:      hex.EncodeToString(sum[:]),
		Status:      attachmentStatusStaged,
	}
	if err := h.store.StageOutboundAttachment(r.Context(), ownerID, meta, body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to stage attachment"})
		return
	}
	writeJSON(w, http.StatusCreated, meta)
}

// handleAttachmentDownload serves staged/bound/sent outbound attachment bytes.
func (h *Handler) handleAttachmentDownload(w http.ResponseWriter, r *http.Request, ownerID, id string) {
	att, err := h.store.GetOutboundAttachment(r.Context(), ownerID, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	data, err := h.store.ReadOutboundAttachmentBytes(ownerID, att)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "attachment bytes unavailable"})
		return
	}
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", att.Filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleAttachmentDelete unstages a staged (unbound) attachment.
func (h *Handler) handleAttachmentDelete(w http.ResponseWriter, r *http.Request, ownerID, id string) {
	if err := h.store.DeleteStagedAttachment(r.Context(), ownerID, id); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if err == errAttachmentNotStaged {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "attachment is bound to a draft"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete attachment"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

var errAttachmentNotStaged = errors.New("attachment is not staged")

// sanitizeAttachmentFilename strips path separators and control characters so
// a client-supplied name cannot escape the owner attachment directory.
func sanitizeAttachmentFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}

// --- Store: staged outbound attachment persistence ---

// outboundAttachmentDir returns the per-owner staged byte directory.
func (s *Store) outboundAttachmentDir(ownerID string) string {
	return filepath.Join(s.storageRoot, "attachments", "outbound", ownerID)
}

// StageOutboundAttachment writes bytes to disk and records a staged row.
func (s *Store) StageOutboundAttachment(ctx context.Context, ownerID string, meta EmailAttachmentMeta, body []byte) error {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return err
	}
	dir := s.outboundAttachmentDir(ownerID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create attachment dir: %w", err)
	}
	storageRef := filepath.Join("attachments", "outbound", ownerID, meta.ID)
	abs := filepath.Join(s.storageRoot, storageRef)
	if err := os.WriteFile(abs, body, 0o600); err != nil {
		return fmt.Errorf("write attachment bytes: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO email_attachments (
		id, message_id, provider_attachment_id, filename, content_type,
		size_bytes, storage_ref, status, direction, sha256, created_at
	) VALUES (?, '', '', ?, ?, ?, ?, ?, ?, ?, ?)`,
		meta.ID, meta.Filename, meta.ContentType, meta.SizeBytes, storageRef,
		attachmentStatusStaged, attachmentDirectionOutbound, meta.SHA256, now)
	if err != nil {
		_ = os.Remove(abs)
		return fmt.Errorf("insert staged attachment: %w", err)
	}
	return nil
}

// GetOutboundAttachment loads one outbound attachment row for the owner.
func (s *Store) GetOutboundAttachment(ctx context.Context, ownerID, id string) (EmailAttachment, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return EmailAttachment{}, err
	}
	var a EmailAttachment
	err = db.QueryRowContext(ctx, `SELECT
		id, coalesce(message_id, ''), coalesce(provider_attachment_id, ''), filename,
		content_type, coalesce(size_bytes, 0), coalesce(storage_ref, ''),
		status, created_at
		FROM email_attachments
		WHERE id = ? AND direction = ?`, id, attachmentDirectionOutbound).
		Scan(&a.ID, &a.MessageID, &a.ProviderAttachmentID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StorageRef, &a.Status, &a.CreatedAt)
	if err != nil {
		return EmailAttachment{}, err
	}
	return a, nil
}

// ReadOutboundAttachmentBytes reads staged/bound bytes from storage_ref.
func (s *Store) ReadOutboundAttachmentBytes(ownerID string, att EmailAttachment) ([]byte, error) {
	ref := strings.TrimSpace(att.StorageRef)
	if ref == "" {
		return nil, fmt.Errorf("attachment has no stored bytes")
	}
	// Constrain reads to the owner's outbound directory.
	clean := filepath.Clean(ref)
	expected := filepath.Join("attachments", "outbound", ownerID)
	if clean != expected && !strings.HasPrefix(clean, expected+string(filepath.Separator)) {
		return nil, fmt.Errorf("attachment storage ref out of bounds")
	}
	return os.ReadFile(filepath.Join(s.storageRoot, clean))
}

// DeleteStagedAttachment removes a staged (unbound) attachment row + bytes.
func (s *Store) DeleteStagedAttachment(ctx context.Context, ownerID, id string) error {
	att, err := s.GetOutboundAttachment(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if att.Status != attachmentStatusStaged {
		return errAttachmentNotStaged
	}
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM email_attachments WHERE id = ? AND status = ?`, id, attachmentStatusStaged); err != nil {
		return fmt.Errorf("delete staged attachment: %w", err)
	}
	if att.StorageRef != "" {
		_ = os.Remove(filepath.Join(s.storageRoot, filepath.Clean(att.StorageRef)))
	}
	return nil
}

// SweepStaleStagedAttachments deletes staged outbound attachments older than
// ttl. Lazy GC invoked on upload; returns the count removed.
func (s *Store) SweepStaleStagedAttachments(ctx context.Context, ownerID string, ttl time.Duration) int {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return 0
	}
	cutoff := time.Now().UTC().Add(-ttl).Format(time.RFC3339Nano)
	rows, err := db.QueryContext(ctx, `SELECT id, coalesce(storage_ref, '')
		FROM email_attachments
		WHERE direction = ? AND status = ? AND created_at < ?`,
		attachmentDirectionOutbound, attachmentStatusStaged, cutoff)
	if err != nil {
		return 0
	}
	type staleRow struct{ id, ref string }
	var stale []staleRow
	for rows.Next() {
		var r staleRow
		if err := rows.Scan(&r.id, &r.ref); err == nil {
			stale = append(stale, r)
		}
	}
	_ = rows.Close()
	removed := 0
	for _, r := range stale {
		if _, err := db.ExecContext(ctx, `DELETE FROM email_attachments WHERE id = ? AND status = ?`, r.id, attachmentStatusStaged); err == nil {
			removed++
			if r.ref != "" {
				_ = os.Remove(filepath.Join(s.storageRoot, filepath.Clean(r.ref)))
			}
		}
	}
	return removed
}

// bindDraftAttachments validates staged attachment ids, enforces caps, marks
// them bound to the draft, and returns the hash-bound refs. Runs inside the
// caller's draft-create transaction boundary via the mailbox db.
func (s *Store) bindDraftAttachments(ctx context.Context, ownerID, draftID string, ids []string, maxTotal int64) ([]draftAttachmentRef, error) {
	ids = cleanAttachmentIDs(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > MaxDraftAttachments {
		return nil, fmt.Errorf("too many attachments: max %d", MaxDraftAttachments)
	}
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return nil, err
	}
	refs := make([]draftAttachmentRef, 0, len(ids))
	var total int64
	for _, id := range ids {
		var a EmailAttachment
		var sha string
		err := db.QueryRowContext(ctx, `SELECT
			id, filename, content_type, coalesce(size_bytes, 0), coalesce(sha256, ''), status
			FROM email_attachments
			WHERE id = ? AND direction = ?`, id, attachmentDirectionOutbound).
			Scan(&a.ID, &a.Filename, &a.ContentType, &a.SizeBytes, &sha, &a.Status)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment %s not found", id)
		}
		if err != nil {
			return nil, fmt.Errorf("load attachment %s: %w", id, err)
		}
		if a.Status != attachmentStatusStaged {
			return nil, fmt.Errorf("attachment %s is not staged", id)
		}
		total += a.SizeBytes
		refs = append(refs, draftAttachmentRef{
			ID: a.ID, Filename: a.Filename, ContentType: a.ContentType,
			SizeBytes: a.SizeBytes, SHA256: sha,
		})
	}
	if total > maxTotal {
		return nil, fmt.Errorf("attachments exceed %d bytes total", maxTotal)
	}
	// Mark bound inside a transaction so a partial bind cannot strand rows.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	for _, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE email_attachments
			SET status = ?, draft_id = ?
			WHERE id = ? AND direction = ? AND status = ?`,
			attachmentStatusBound, draftID, id, attachmentDirectionOutbound, attachmentStatusStaged)
		if err != nil {
			return nil, fmt.Errorf("bind attachment %s: %w", id, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, fmt.Errorf("attachment %s is not staged", id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return refs, nil
}

// loadDraftAttachments decodes the draft's bound attachment refs.
func loadDraftAttachmentRefs(draft EmailDraft) []draftAttachmentRef {
	var refs []draftAttachmentRef
	if strings.TrimSpace(draft.AttachmentsJSON) == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(draft.AttachmentsJSON), &refs)
	return refs
}

// verifyAndEncodeDraftAttachments re-reads bound bytes, verifies each sha256
// against the hash-bound ref (tamper check), and returns Resend payloads.
func (s *Store) verifyAndEncodeDraftAttachments(ownerID string, refs []draftAttachmentRef) ([]resendAttachment, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make([]resendAttachment, 0, len(refs))
	for _, ref := range refs {
		att, err := s.GetOutboundAttachment(context.Background(), ownerID, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("attachment %s unavailable", ref.ID)
		}
		data, err := s.ReadOutboundAttachmentBytes(ownerID, att)
		if err != nil {
			return nil, fmt.Errorf("attachment %s bytes unavailable", ref.ID)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), ref.SHA256) {
			return nil, fmt.Errorf("attachment %s content changed since approval", ref.ID)
		}
		out = append(out, resendAttachment{
			Filename: ref.Filename,
			Content:  base64.StdEncoding.EncodeToString(data),
		})
	}
	return out, nil
}

// markDraftAttachmentsSent flips bound attachment rows to sent and links them
// to the stored outbound message so Sent renders them.
func (s *Store) markDraftAttachmentsSent(ctx context.Context, ownerID, draftID, messageID string, refs []draftAttachmentRef) error {
	if len(refs) == 0 {
		return nil
	}
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := db.ExecContext(ctx, `UPDATE email_attachments
			SET status = ?, message_id = ?
			WHERE id = ? AND direction = ?`,
			attachmentStatusSent, messageID, ref.ID, attachmentDirectionOutbound); err != nil {
			return fmt.Errorf("mark attachment %s sent: %w", ref.ID, err)
		}
	}
	return nil
}

func cleanAttachmentIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
