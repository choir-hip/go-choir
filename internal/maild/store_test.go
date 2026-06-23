package maild

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, *Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &Config{
		Port:             DefaultPort,
		DBPath:           filepath.Join(dir, "mail.db"),
		StorageRoot:      filepath.Join(dir, "mail"),
		PrimaryDomain:    "choir.news",
		RootOwnerID:      "user-root",
		ResendBaseURL:    DefaultResendBaseURL,
		WebhookMaxBytes:  DefaultWebhookMaxBody,
		APIMaxBytes:      DefaultAPIMaxBody,
		ProviderMaxBytes: DefaultProviderMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	store, err := OpenStore(cfg.DBPath, cfg.StorageRoot)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return store, cfg
}

func TestLegacySharedMailboxMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mail.db")
	storageRoot := filepath.Join(dir, "mail")

	// Create a legacy shared database (pre-multi-tenancy schema) with all tables
	// in the single DBPath database.
	legacyDB, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer legacyDB.Close()

	legacySchema := []string{
		`CREATE TABLE IF NOT EXISTS email_messages (
			id text primary key,
			provider text not null,
			provider_message_id text,
			provider_event_id text,
			direction text not null,
			mailbox_owner_id text not null,
			alias_id text,
			from_address text not null,
			from_display text,
			subject text not null,
			text_body text,
			html_body text,
			raw_headers_json text,
			raw_message_ref text,
			authentication_results_json text,
			trust_status text not null,
			read_at text,
			received_at text,
			sent_at text,
			created_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_message_recipients (
			id text primary key,
			message_id text not null,
			kind text not null,
			address text not null,
			display text
		)`,
		`CREATE TABLE IF NOT EXISTS email_attachments (
			id text primary key,
			message_id text not null,
			provider_attachment_id text,
			filename text not null,
			content_type text not null,
			content_disposition text,
			content_id text,
			size_bytes integer,
			storage_ref text,
			status text not null,
			created_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_source_packets (
			id text primary key,
			message_id text not null,
			attachment_id text,
			trust_label text not null,
			provenance_json text not null,
			text_ref text,
			created_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_ingress_events (
			id text primary key,
			message_id text not null,
			source_packet_id text,
			owner_id text not null,
			conductor_submission_id text,
			status text not null,
			created_at text not null,
			completed_at text
		)`,
		`CREATE TABLE IF NOT EXISTS email_drafts (
			id text primary key,
			owner_id text not null,
			from_alias_id text not null,
			from_address text not null,
			to_json text not null,
			cc_json text,
			bcc_json text,
			subject text not null,
			text_body text,
			html_body text,
			reply_to_message_id text,
			source_kind text,
			source_ref text,
			status text not null,
			version integer not null,
			version_hash text not null,
			sent_message_id text,
			provider_message_id text,
			created_at text not null,
			updated_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_draft_approval_events (
			id text primary key,
			draft_id text not null,
			owner_id text not null,
			version integer not null,
			version_hash text not null,
			event_type text not null,
			provider_message_id text,
			created_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_draft_approval_tokens (
			id text primary key,
			token text not null unique,
			draft_id text not null,
			owner_id text not null,
			version integer not null,
			version_hash text not null,
			approval_email text not null,
			status text not null,
			provider_message_id text,
			created_at text not null,
			expires_at text not null,
			used_at text
		)`,
		`CREATE TABLE IF NOT EXISTS email_risk_alerts (
			id text primary key,
			owner_id text not null,
			risk_kind text not null,
			source_ref text,
			snippet text,
			provider_message_id text,
			created_at text not null
		)`,
	}
	for _, stmt := range legacySchema {
		if _, err := legacyDB.Exec(stmt); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Insert legacy rows for two owners.
	for _, owner := range []string{"user-a", "user-b"} {
		msgID := "msg-" + owner
		_, err = legacyDB.Exec(`INSERT INTO email_messages (
			id, provider, provider_message_id, provider_event_id, direction,
			mailbox_owner_id, alias_id, from_address, from_display, subject,
			text_body, html_body, raw_headers_json, raw_message_ref,
			authentication_results_json, trust_status, read_at, received_at,
			sent_at, created_at
		) VALUES (?, 'resend', ?, ?, 'inbound', ?, 'alias-1', 'sender@example.com', 'Sender', 'Hello', 'text body', 'html body', '{}', 'raw-ref', '{}', 'untrusted', NULL, ?, NULL, ?)`,
			msgID, "provider-"+msgID, "event-"+msgID, owner, now, now)
		if err != nil {
			t.Fatalf("insert message for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_message_recipients (
			id, message_id, kind, address, display
		) VALUES (?, ?, 'to', 'recipient@example.com', 'Recipient')`,
			"recipient-"+msgID, msgID)
		if err != nil {
			t.Fatalf("insert recipient for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_attachments (
			id, message_id, provider_attachment_id, filename, content_type,
			content_disposition, content_id, size_bytes, storage_ref, status, created_at
		) VALUES (?, ?, 'att-1', 'file.txt', 'text/plain', 'attachment', 'cid-1', 42, 'attachments/file.txt', 'quarantined', ?)`,
			"attachment-"+msgID, msgID, now)
		if err != nil {
			t.Fatalf("insert attachment for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_source_packets (
			id, message_id, attachment_id, trust_label, provenance_json, text_ref, created_at
		) VALUES (?, ?, ?, 'UNTRUSTED_EXTERNAL_EMAIL', '{"provider":"resend"}', 'text-ref', ?)`,
			"source-"+msgID, msgID, "attachment-"+msgID, now)
		if err != nil {
			t.Fatalf("insert source packet for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_ingress_events (
			id, message_id, source_packet_id, owner_id, conductor_submission_id, status, created_at, completed_at
		) VALUES (?, ?, ?, ?, 'sub-1', 'completed', ?, ?)`,
			"ingress-"+msgID, msgID, "source-"+msgID, owner, now, now)
		if err != nil {
			t.Fatalf("insert ingress event for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_drafts (
			id, owner_id, from_alias_id, from_address, to_json, cc_json, bcc_json,
			subject, text_body, html_body, reply_to_message_id, source_kind,
			source_ref, status, version, version_hash, sent_message_id,
			provider_message_id, created_at, updated_at
		) VALUES (?, ?, 'alias-1', '000@choir.news', '["to@example.com"]', '', '',
			'Draft', 'draft body', '', '', '', '', 'draft', 1,
			'hash-1', '', '', ?, ?)`,
			"draft-"+owner, owner, now, now)
		if err != nil {
			t.Fatalf("insert draft for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_draft_approval_events (
			id, draft_id, owner_id, version, version_hash, event_type, provider_message_id, created_at
		) VALUES (?, ?, ?, 1, 'hash-1', 'sent', NULL, ?)`,
			"approval-event-"+owner, "draft-"+owner, owner, now)
		if err != nil {
			t.Fatalf("insert approval event for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_draft_approval_tokens (
			id, token, draft_id, owner_id, version, version_hash, approval_email,
			status, provider_message_id, created_at, expires_at, used_at
		) VALUES (?, ?, ?, ?, 1, 'hash-1', 'owner@example.com', 'active', NULL, ?, ?, NULL)`,
			"token-"+owner, "token-value-"+owner, "draft-"+owner, owner, now, now)
		if err != nil {
			t.Fatalf("insert approval token for %s: %v", owner, err)
		}
		_, err = legacyDB.Exec(`INSERT INTO email_risk_alerts (
			id, owner_id, risk_kind, source_ref, snippet, provider_message_id, created_at
		) VALUES (?, ?, 'UNTRUSTED_LINK', 'ref-1', 'suspicious link', 'provider-1', ?)`,
			"risk-"+owner, owner, now)
		if err != nil {
			t.Fatalf("insert risk alert for %s: %v", owner, err)
		}
	}

	legacyDB.Close()

	// Now open the same database with the multi-tenant store. EnsureSchema
	// should run the migration.
	cfg := &Config{
		Port:             DefaultPort,
		DBPath:           dbPath,
		StorageRoot:      storageRoot,
		PrimaryDomain:    "choir.news",
		RootOwnerID:      "user-root",
		ResendBaseURL:    DefaultResendBaseURL,
		WebhookMaxBytes:  DefaultWebhookMaxBody,
		APIMaxBytes:      DefaultAPIMaxBody,
		ProviderMaxBytes: DefaultProviderMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	store, err := OpenStore(cfg.DBPath, cfg.StorageRoot)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Verify each owner can see their migrated data via the new API.
	for _, owner := range []string{"user-a", "user-b"} {
		msgs, err := store.ListMessages(context.Background(), owner, "inbox", 10)
		if err != nil {
			t.Fatalf("ListMessages %s: %v", owner, err)
		}
		if len(msgs) != 1 {
			t.Fatalf("owner %s messages = %d, want 1", owner, len(msgs))
		}
		if msgs[0].Subject != "Hello" {
			t.Fatalf("owner %s subject = %q, want Hello", owner, msgs[0].Subject)
		}
		draft, err := store.GetDraft(context.Background(), owner, "draft-"+owner)
		if err != nil {
			t.Fatalf("GetDraft %s: %v", owner, err)
		}
		if draft.Subject != "Draft" {
			t.Fatalf("owner %s draft subject = %q, want Draft", owner, draft.Subject)
		}
	}

	// Verify migration is recorded.
	var migrationCompleted string
	if err := store.routingDB.QueryRow(`SELECT completed_at FROM maild_migrations WHERE name = 'shared_to_per_owner_mailbox'`).Scan(&migrationCompleted); err != nil {
		t.Fatalf("migration record missing: %v", err)
	}
	if migrationCompleted == "" {
		t.Fatalf("migration completed_at empty")
	}

	// Verify idempotency: running EnsureSchema again does not fail.
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema second pass: %v", err)
	}
	msgs, err := store.ListMessages(context.Background(), "user-a", "inbox", 10)
	if err != nil {
		t.Fatalf("ListMessages after idempotency: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("user-a messages after idempotency = %d, want 1", len(msgs))
	}
}

func TestEnsureSchemaSeedsRootAlias(t *testing.T) {
	store, _ := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if alias.TargetID != "user-root" {
		t.Fatalf("TargetID = %q, want user-root", alias.TargetID)
	}
	if alias.ReceivePolicyID != DefaultPublicPolicyID {
		t.Fatalf("ReceivePolicyID = %q, want %q", alias.ReceivePolicyID, DefaultPublicPolicyID)
	}
	policy, err := store.GetReceivePolicy(context.Background(), DefaultTrustedWorkflowPolicyID)
	if err != nil {
		t.Fatalf("GetReceivePolicy trusted workflow: %v", err)
	}
	if policy.AllowPublicInbound || !policy.RequireSenderWhitelist || !policy.RequireSecretAlias || !policy.AllowAutoAgentRead {
		t.Fatalf("trusted workflow policy = %+v", policy)
	}
}

func TestEnsureSchemaReconcilesRootAliasOwner(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.RootOwnerID = "real-founder"
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema second pass: %v", err)
	}
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if alias.TargetID != "real-founder" {
		t.Fatalf("TargetID = %q, want real-founder", alias.TargetID)
	}
}

func TestEnsureSchemaRepairsActiveApprovalTokensForSentDrafts(t *testing.T) {
	store, cfg := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Stale approval token repair",
		TextBody:    "Already sent.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(context.Background(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	mbDB, err := store.mailboxForOwner("user-root")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	if _, err := mbDB.ExecContext(context.Background(), `UPDATE email_drafts SET status = 'sent' WHERE id = ?`, draft.ID); err != nil {
		t.Fatalf("force sent draft: %v", err)
	}

	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema repair: %v", err)
	}
	repaired, err := store.GetDraftApprovalToken(context.Background(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if repaired.Status != "stale_sent" || repaired.UsedAt == "" {
		t.Fatalf("repaired token = %+v, want stale_sent with used_at", repaired)
	}
}

func TestEnsureSchemaRepairsRejectedDrafts(t *testing.T) {
	store, cfg := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Rejected repair",
		TextBody:    "Do not send.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(context.Background(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	if err := store.UseDraftApprovalToken(context.Background(), token.ID, "rejected"); err != nil {
		t.Fatalf("UseDraftApprovalToken rejected: %v", err)
	}

	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema repair: %v", err)
	}
	repaired, err := store.GetDraft(context.Background(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if repaired.Status != "draft_rejected" {
		t.Fatalf("draft status = %q, want draft_rejected", repaired.Status)
	}
	drafts, err := store.ListDrafts(context.Background(), "user-root", 10)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("repaired rejected draft still listed: %+v", drafts)
	}
}

func TestRecordWebhookEventIdempotent(t *testing.T) {
	store, _ := newTestStore(t)
	event := WebhookEvent{
		ID:                "event-row-1",
		Provider:          providerResend,
		ProviderEventID:   "evt-1",
		ProviderMessageID: "email-1",
		EventType:         "email.received",
		RawPayload:        `{"id":"evt-1"}`,
		ReceivedAt:        time.Now(),
	}
	created, err := store.RecordWebhookEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordWebhookEvent first: %v", err)
	}
	if !created {
		t.Fatalf("first insert created = false")
	}
	created, err = store.RecordWebhookEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordWebhookEvent duplicate: %v", err)
	}
	if created {
		t.Fatalf("duplicate insert created = true")
	}
	count, err := store.CountWebhookEvents(context.Background())
	if err != nil {
		t.Fatalf("CountWebhookEvents: %v", err)
	}
	if count != 1 {
		t.Fatalf("webhook count = %d, want 1", count)
	}
}

func TestConfigureTrustedWorkflowAlias(t *testing.T) {
	store, _ := newTestStore(t)
	alias, err := store.ConfigureTrustedWorkflowAlias(context.Background(), TrustedWorkflowAliasConfig{
		OwnerID:       "user-root",
		Domain:        "choir.news",
		LocalPart:     "000+invite-test",
		SenderAddress: "sender@example.com",
	})
	if err != nil {
		t.Fatalf("ConfigureTrustedWorkflowAlias: %v", err)
	}
	if alias.TargetID != "user-root" || alias.LocalPart != "000+invite-test" || alias.ReceivePolicyID != DefaultTrustedWorkflowPolicyID {
		t.Fatalf("alias = %+v", alias)
	}
	ok, err := store.IsSenderWhitelisted(context.Background(), "user-root", alias.ID, "sender@example.com")
	if err != nil {
		t.Fatalf("IsSenderWhitelisted: %v", err)
	}
	if !ok {
		t.Fatalf("sender was not whitelisted")
	}
}
