package maild

// Account deletion in maild (docs/design/account-create-delete-2026-11-23.md,
// the owner's rulings of 2026-11-23): remove everything maild holds for one
// account: its mailbox database, its staged and bound outbound attachments,
// its numbered address and every alias that targets it, its sender
// whitelist, the provider-message and approval-token indexes, the stored
// webhook payloads of its messages, its owner address and its digest rows.
//
// The route is host-internal (the auth service's purge runner) and the store
// call is destructive, so it refuses a request that does not carry the
// deletion record's request time at least 72 hours old (purgeguard). The
// numbered address is not issued again: the number sequence only grows.
//
// Not reached, by construction: copies held by the mail provider (Resend)
// and by the recipients' own mail systems.
//
// Ways this could go wrong, written before the code (tests in
// account_purge_test.go):
//  1. the purge runs with no deletion time, a young one or a future one;
//  2. it removes another account's mailbox, alias, index row or webhook
//     payload;
//  3. it removes the root owner's mail, or the shared root alias;
//  4. an owner id with a path separator reaches outside the storage root;
//  5. a second run fails, or a half run is not finished by the next;
//  6. an alias, number or attachment of the account survives.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// AuthPurgeAccountPath is the host-internal route the purge runner calls.
const AuthPurgeAccountPath = "/internal/maild/auth/purge-account"

// AccountPurgeCounts is what a purge removed.
type AccountPurgeCounts struct {
	Aliases       int64 `json:"aliases"`
	AliasReceipts int64 `json:"alias_receipts"`
	Whitelist     int64 `json:"whitelist"`
	ProviderIndex int64 `json:"provider_index"`
	WebhookEvents int64 `json:"webhook_events"`
	ApprovalIndex int64 `json:"approval_index"`
	OwnerAddress  int64 `json:"owner_address"`
	Digest        int64 `json:"digest"`
	// LegacyRows are the account's rows in the shared tables from before
	// mailboxes were split per account (legacySharedMailTables).
	LegacyRows  int64 `json:"legacy_rows"`
	MailboxDir  bool  `json:"mailbox_dir"`
	Attachments bool  `json:"attachments_dir"`
}

// legacySharedMailTables are the shared tables of the routing database from
// before mailboxes were split per account, each with the condition that picks
// one owner's rows. migrateLegacySharedMailbox copies out of them and leaves
// them, so an account's old mail can still be there; a table that does not
// exist is skipped. The children of a message (recipients, attachments,
// source packets) are keyed by the message and go before the messages.
// email_draft_edit_requests was never in the shared database here; it is
// listed because internal/unprovedpurge (MaildKeys) lists it.
var legacySharedMailTables = []struct{ table, where string }{
	{"email_message_recipients", "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)"},
	{"email_attachments", "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)"},
	{"email_source_packets", "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)"},
	{"email_messages", "mailbox_owner_id = ?"},
	{"email_ingress_events", "owner_id = ?"},
	{"email_drafts", "owner_id = ?"},
	{"email_draft_approval_events", "owner_id = ?"},
	{"email_draft_approval_tokens", "owner_id = ?"},
	{"email_risk_alerts", "owner_id = ?"},
	{"email_draft_edit_requests", "owner_id = ?"},
}

func safeOwnerComponent(ownerID string) bool {
	return ownerID != "" && ownerID != "." && ownerID != ".." && !strings.ContainsAny(ownerID, `/\`) && filepath.Base(ownerID) == ownerID
}

// PurgeAccount removes everything maild holds for ownerID. rootOwnerID is the
// shared root owner that is never purged.
func (s *Store) PurgeAccount(ctx context.Context, ownerID, rootOwnerID string, requestedAt, now time.Time) (AccountPurgeCounts, error) {
	var c AccountPurgeCounts
	ownerID = strings.TrimSpace(ownerID)
	if err := purgeguard.CheckClaim(requestedAt, now); err != nil {
		return c, err
	}
	if !safeOwnerComponent(ownerID) {
		return c, fmt.Errorf("maild: purge: invalid owner id")
	}
	if ownerID == strings.TrimSpace(rootOwnerID) {
		return c, errors.New("maild: purge: the root owner is never purged")
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer func() { _ = tx.Rollback() }()
	exec := func(dst *int64, q string, args ...any) error {
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("maild: purge: %w", err)
		}
		n, _ := res.RowsAffected()
		*dst += n
		return nil
	}
	// Webhook payloads of the account's messages first: they are found
	// through the provider index the next statement deletes.
	if err := exec(&c.WebhookEvents, `DELETE FROM email_webhook_events WHERE provider_message_id IN
		(SELECT provider_message_id FROM email_provider_message_index WHERE owner_id = ?)`, ownerID); err != nil {
		return c, err
	}
	steps := []struct {
		dst *int64
		q   string
	}{
		{&c.AliasReceipts, `DELETE FROM email_alias_receipts WHERE alias_id IN (SELECT id FROM email_aliases WHERE target_id = ? AND canonical_number IS NOT 0)`},
		{&c.Aliases, `DELETE FROM email_aliases WHERE target_id = ? AND canonical_number IS NOT 0`},
		{&c.Whitelist, `DELETE FROM email_sender_whitelist WHERE owner_id = ?`},
		{&c.ProviderIndex, `DELETE FROM email_provider_message_index WHERE owner_id = ?`},
		{&c.ApprovalIndex, `DELETE FROM email_approval_token_index WHERE owner_id = ?`},
		{&c.OwnerAddress, `DELETE FROM email_owner_addresses WHERE owner_id = ?`},
		{&c.Digest, `DELETE FROM digest_sent WHERE owner_id = ?`},
		{&c.Digest, `DELETE FROM digest_runs WHERE owner_id = ?`},
		{&c.Digest, `DELETE FROM digest_subscriptions WHERE owner_id = ?`},
	}
	for _, st := range steps {
		if err := exec(st.dst, st.q, ownerID); err != nil {
			return c, err
		}
	}
	for _, lt := range legacySharedMailTables {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, lt.table).Scan(&n); err != nil {
			return c, fmt.Errorf("maild: purge: %w", err)
		}
		if n == 0 {
			continue
		}
		if lt.table != "email_messages" && strings.Contains(lt.where, "email_messages") {
			// A child table in a database whose messages table is gone has
			// no rows of this owner to find.
			var m int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'email_messages'`).Scan(&m); err != nil {
				return c, fmt.Errorf("maild: purge: %w", err)
			}
			if m == 0 {
				continue
			}
		}
		if err := exec(&c.LegacyRows, `DELETE FROM `+lt.table+` WHERE `+lt.where, ownerID); err != nil {
			return c, err
		}
	}
	if err := tx.Commit(); err != nil {
		return c, err
	}
	// The files go after the rows: a failure here leaves a mailbox with no
	// route to it, which the next run (the runner retries) removes.
	s.mu.Lock()
	if db, ok := s.mailboxes[ownerID]; ok {
		_ = db.Close()
		delete(s.mailboxes, ownerID)
	}
	s.mu.Unlock()
	for _, dir := range []struct {
		path string
		flag *bool
	}{
		{filepath.Join(s.storageRoot, "users", ownerID), &c.MailboxDir},
		{filepath.Join(s.storageRoot, "attachments", "outbound", ownerID), &c.Attachments},
	} {
		if _, err := os.Stat(dir.path); err == nil {
			if err := os.RemoveAll(dir.path); err != nil {
				return c, fmt.Errorf("maild: purge: remove %s: %w", filepath.Base(filepath.Dir(dir.path)), err)
			}
			*dir.flag = true
		}
	}
	log.Printf("maild: operation=purge_account aliases=%d whitelist=%d provider_index=%d webhook_events=%d mailbox_dir=%v attachments_dir=%v",
		c.Aliases, c.Whitelist, c.ProviderIndex, c.WebhookEvents, c.MailboxDir, c.Attachments)
	return c, nil
}

type authPurgeAccountRequest struct {
	OwnerID     string `json:"owner_id"`
	RequestedAt string `json:"deletion_requested_at"`
}

// HandleAuthPurgeAccount handles POST AuthPurgeAccountPath.
func (h *Handler) HandleAuthPurgeAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !requireHostServiceCaller(w, r) {
		return
	}
	var in authPurgeAccountRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	requested, err := time.Parse(time.RFC3339, in.RequestedAt)
	if err != nil {
		requested = time.Time{}
	}
	counts, err := h.store.PurgeAccount(r.Context(), in.OwnerID, h.cfg.RootOwnerID, requested, time.Now().UTC())
	switch {
	case errors.Is(err, purgeguard.ErrNoRecord), errors.Is(err, purgeguard.ErrTooEarly):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case err != nil:
		log.Printf("maild: operation=purge_account result=error error=%q", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "purge failed; it will be retried"})
	default:
		writeJSON(w, http.StatusOK, counts)
	}
}
