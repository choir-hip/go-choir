package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultPublicPolicyID          = "policy-public-inbound-v0"
	DefaultTrustedWorkflowPolicyID = "policy-trusted-workflow-v0"
	DefaultRootAliasID             = "alias-choir-news-000"
)

// Store is maild's SQLite-backed durable state. It maintains a global routing
// database for aliases, policies, whitelists, and webhook events, plus a
// per-user mailbox database for messages, attachments, drafts, and risk
// alerts. This enforces storage-level isolation between users.
type Store struct {
	routingDB *sql.DB

	mu          sync.Mutex
	mailboxes   map[string]*sql.DB
	storageRoot string
}

// EmailAlias is a resolved local-part alias.
type EmailAlias struct {
	ID              string
	Domain          string
	LocalPart       string
	CanonicalNumber int
	TargetType      string
	TargetID        string
	Visibility      string
	ReceivePolicyID string
}

// EmailReceivePolicy controls whether a resolved alias accepts inbound mail.
type EmailReceivePolicy struct {
	ID                     string
	Name                   string
	AllowPublicInbound     bool
	AllowAttachments       bool
	RequireSenderWhitelist bool
	RequireSecretAlias     bool
	AllowAutoAgentRead     bool
	AllowAutoAgentWrite    bool
	AllowAutoOutboundSend  bool
	QuarantineByDefault    bool
}

// WebhookEvent is the durable receipt record for a verified provider webhook.
type WebhookEvent struct {
	ID                string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	RawPayload        string
	ReceivedAt        time.Time
}

// EmailMessage is a normalized mailbox row.
type EmailMessage struct {
	ID                        string
	Provider                  string
	ProviderMessageID         string
	ProviderEventID           string
	Direction                 string
	MailboxOwnerID            string
	AliasID                   string
	FromAddress               string
	FromDisplay               string
	Subject                   string
	TextBody                  string
	HTMLBody                  string
	RawHeadersJSON            string
	AuthenticationResultsJSON string
	TrustStatus               string
	ReadAt                    string
	ReceivedAt                string
	SentAt                    string
	CreatedAt                 string
	HasAttachments            bool
}

// EmailRecipient is a normalized message recipient visible to the mailbox owner.
type EmailRecipient struct {
	Kind    string
	Address string
	Display string
}

// EmailAttachment is message attachment metadata.
type EmailAttachment struct {
	ID                   string
	MessageID            string
	ProviderAttachmentID string
	Filename             string
	ContentType          string
	SizeBytes            int64
	StorageRef           string
	Status               string
	CreatedAt            string
}

// EmailSourcePacket is the owner-visible safe source envelope for MAS handoff.
type EmailSourcePacket struct {
	ID             string
	MessageID      string
	TrustLabel     string
	ProvenanceJSON string
	TextRef        string
	CreatedAt      string
}

// EmailIngressEvent is a recorded owner-triggered MAS handoff.
type EmailIngressEvent struct {
	ID                    string
	MessageID             string
	SourcePacketID        string
	OwnerID               string
	ConductorSubmissionID string
	Status                string
	CreatedAt             string
	CompletedAt           string
}

// EmailAliasSummary is an owner-scoped address exposed to the Email app.
type EmailAliasSummary struct {
	ID         string
	Address    string
	LocalPart  string
	Domain     string
	Visibility string
}

// EmailDraft is an appagent-controlled outbound draft. Sending is a later
// explicit owner action against the current version.
type EmailDraft struct {
	ID                string
	OwnerID           string
	FromAliasID       string
	FromAddress       string
	ToJSON            string
	CcJSON            string
	BccJSON           string
	Subject           string
	TextBody          string
	HTMLBody          string
	ReplyToMessageID  string
	SourceKind        string
	SourceRef         string
	Status            string
	Version           int
	VersionHash       string
	SentMessageID     string
	ProviderMessageID string
	CreatedAt         string
	UpdatedAt         string
}

// EmailApprovalToken binds one approval channel to one exact draft version.
type EmailApprovalToken struct {
	ID                string
	Token             string
	DraftID           string
	OwnerID           string
	Version           int
	VersionHash       string
	ApprovalEmail     string
	Status            string
	ProviderMessageID string
	CreatedAt         string
	ExpiresAt         string
	UsedAt            string
}

// EmailRiskAlert records a structured provider-backed alert for a blocked
// email action. Risky text is stored only as bounded evidence.
type EmailRiskAlert struct {
	ID                string
	OwnerID           string
	RiskKind          string
	SourceRef         string
	Snippet           string
	ProviderMessageID string
	CreatedAt         string
}

// TrustedWorkflowAliasConfig describes a narrow plus-code alias that can create
// a pending workflow handoff when a whitelisted authenticated sender uses it.
type TrustedWorkflowAliasConfig struct {
	OwnerID       string
	Domain        string
	LocalPart     string
	SenderAddress string
}

// StoreStats is a safe operational summary for health reporting.
type StoreStats struct {
	Aliases                int `json:"aliases"`
	Messages               int `json:"messages"`
	QuarantinedAttachments int `json:"quarantined_attachments"`
	WebhookEvents          int `json:"webhook_events"`
	IngressEvents          int `json:"ingress_events"`
}

// OpenStore opens a maild store with a global routing database. Per-user
// mailbox databases are opened on demand via mailboxForOwner.
func OpenStore(dbPath string, storageRoot string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open routing sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping routing sqlite: %w", err)
	}
	return &Store{
		routingDB:   db,
		mailboxes:   make(map[string]*sql.DB),
		storageRoot: storageRoot,
	}, nil
}

// Close closes the routing database and all cached mailbox databases.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var firstErr error
	s.mu.Lock()
	for ownerID, db := range s.mailboxes {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(s.mailboxes, ownerID)
	}
	s.mu.Unlock()
	if s.routingDB != nil {
		if err := s.routingDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// mailboxForOwner returns the per-user mailbox database for the given owner,
// opening and caching it on first access.
func (s *Store) mailboxForOwner(ownerID string) (*sql.DB, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.mailboxes[ownerID]; ok {
		return db, nil
	}
	dir := filepath.Join(s.storageRoot, "users", ownerID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create mailbox dir for %s: %w", ownerID, err)
	}
	dbPath := filepath.Join(dir, "mail.db")
	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open mailbox sqlite for %s: %w", ownerID, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mailbox sqlite for %s: %w", ownerID, err)
	}
	if err := ensureMailboxSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure mailbox schema for %s: %w", ownerID, err)
	}
	s.mailboxes[ownerID] = db
	return db, nil
}

// MailboxForOwner returns the per-user mailbox database for the given owner.
// It is exported for use by external tooling (e.g. maildctl tests).
func (s *Store) MailboxForOwner(ownerID string) (*sql.DB, error) {
	return s.mailboxForOwner(ownerID)
}

// EnsureSchema creates the routing schema and seeds defaults. Per-user
// mailbox schemas are created on demand when a mailbox is opened.
func (s *Store) EnsureSchema(cfg *Config) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS email_receive_policies (
			id text primary key,
			name text not null,
			allow_public_inbound integer not null,
			allow_attachments integer not null,
			require_sender_whitelist integer not null,
			require_secret_alias integer not null,
			allow_auto_agent_read integer not null,
			allow_auto_agent_write integer not null,
			allow_auto_outbound_send integer not null,
			quarantine_by_default integer not null,
			created_at text not null
		)`,
		`CREATE TABLE IF NOT EXISTS email_aliases (
			id text primary key,
			domain text not null,
			local_part text not null,
			canonical_number integer,
			target_type text not null,
			target_id text not null,
			visibility text not null,
			receive_policy_id text not null,
			created_at text not null,
			disabled_at text,
			unique(domain, local_part),
			foreign key(receive_policy_id) references email_receive_policies(id)
		)`,
		`CREATE TABLE IF NOT EXISTS email_sender_whitelist (
			id text primary key,
			owner_id text not null,
			alias_id text not null,
			sender_address text not null,
			created_at text not null,
			disabled_at text,
			unique(alias_id, sender_address)
		)`,
		`CREATE TABLE IF NOT EXISTS email_webhook_events (
			id text primary key,
			provider text not null,
			provider_event_id text not null,
			provider_message_id text,
			event_type text not null,
			raw_payload text not null,
			received_at text not null,
			unique(provider, provider_event_id)
		)`,
		`CREATE TABLE IF NOT EXISTS email_provider_message_index (
			provider text not null,
			provider_message_id text not null,
			owner_id text not null,
			created_at text not null,
			UNIQUE(provider, provider_message_id)
		)`,
		`CREATE TABLE IF NOT EXISTS email_approval_token_index (
			token text primary key,
			owner_id text not null,
			draft_id text not null,
			created_at text not null
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.routingDB.Exec(stmt); err != nil {
			return fmt.Errorf("routing schema migration: %w", err)
		}
	}
	if err := s.seedDefaults(cfg); err != nil {
		return err
	}
	if err := s.migrateLegacySharedMailbox(); err != nil {
		return fmt.Errorf("migrate legacy shared mailbox: %w", err)
	}
	return s.repairCachedMailboxes()
}

// migrateLegacySharedMailbox copies per-owner data from a pre-multi-tenancy
// shared mail.db into per-owner mailbox databases.
//
// Before the multi-tenancy refactor, all messages, drafts, attachments, and
// other owner-scoped rows lived in the single DBPath database alongside the
// routing tables. After the refactor, those rows live in per-owner databases
// under <storageRoot>/users/<ownerID>/mail.db.
//
// This migration is idempotent: it records completion in a
// maild_migrations tracking table in the routing database and skips work if
// the migration has already run. It also becomes a no-op if the routing
// database does not contain the legacy per-owner tables.
func (s *Store) migrateLegacySharedMailbox() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create migration tracking table.
	if _, err := s.routingDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS maild_migrations (
		name text primary key,
		completed_at text not null
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	var completedAt string
	err := s.routingDB.QueryRowContext(ctx,
		`SELECT completed_at FROM maild_migrations WHERE name = 'shared_to_per_owner_mailbox'`).Scan(&completedAt)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check migration status: %w", err)
	}

	// Detect whether the routing database contains the legacy per-owner tables.
	var messagesTable int
	if err := s.routingDB.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'email_messages'`).Scan(&messagesTable); err != nil {
		return fmt.Errorf("detect legacy messages table: %w", err)
	}
	if messagesTable == 0 {
		// Fresh install or already migrated-and-dropped; record completion.
		_, err := s.routingDB.ExecContext(ctx,
			`INSERT OR REPLACE INTO maild_migrations (name, completed_at) VALUES (?, ?)`,
			"shared_to_per_owner_mailbox", time.Now().UTC().Format(time.RFC3339Nano))
		return err
	}

	// Collect distinct owner IDs from all legacy per-owner tables.
	ownerRows, err := s.routingDB.QueryContext(ctx, `
		SELECT owner_id FROM (
			SELECT DISTINCT mailbox_owner_id AS owner_id FROM email_messages
			UNION
			SELECT DISTINCT owner_id FROM email_ingress_events
			UNION
			SELECT DISTINCT owner_id FROM email_drafts
			UNION
			SELECT DISTINCT owner_id FROM email_draft_approval_events
			UNION
			SELECT DISTINCT owner_id FROM email_draft_approval_tokens
			UNION
			SELECT DISTINCT owner_id FROM email_risk_alerts
		)
		WHERE owner_id IS NOT NULL AND owner_id != ''
		ORDER BY owner_id`)
	if err != nil {
		return fmt.Errorf("list legacy owners: %w", err)
	}
	var ownerIDs []string
	for ownerRows.Next() {
		var ownerID string
		if err := ownerRows.Scan(&ownerID); err != nil {
			ownerRows.Close()
			return fmt.Errorf("scan owner id: %w", err)
		}
		ownerIDs = append(ownerIDs, ownerID)
	}
	if err := ownerRows.Err(); err != nil {
		ownerRows.Close()
		return fmt.Errorf("iterate owner ids: %w", err)
	}
	ownerRows.Close()

	// Migrate each owner into their own mailbox database.
	for _, ownerID := range ownerIDs {
		if err := s.migrateOwnerFromLegacySharedDB(ctx, ownerID); err != nil {
			return fmt.Errorf("migrate owner %s: %w", ownerID, err)
		}
	}

	// Record completion.
	_, err = s.routingDB.ExecContext(ctx,
		`INSERT OR REPLACE INTO maild_migrations (name, completed_at) VALUES (?, ?)`,
		"shared_to_per_owner_mailbox", time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("record migration completion: %w", err)
	}
	return nil
}

// migrateOwnerFromLegacySharedDB copies all owner-scoped rows for ownerID from
// the routing database (legacy shared tables) into the owner's per-owner
// mailbox database.
func (s *Store) migrateOwnerFromLegacySharedDB(ctx context.Context, ownerID string) error {
	mbDB, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return fmt.Errorf("open mailbox for owner %s: %w", ownerID, err)
	}

	tx, err := mbDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin per-owner tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log and continue; best-effort rollback.
		}
	}()

	// Disable foreign keys during the bulk copy so we can insert in any order.
	if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}

	// Helper to copy rows from a legacy table into the same-named table in the
	// per-owner database. columnOrder is the explicit column list for both the
	// source SELECT and the target INSERT.
	copyRows := func(tableName, ownerColumn, columnOrder string, ownerFilter string) error {
		where := ownerFilter
		query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", columnOrder, tableName, where)
		rows, err := s.routingDB.QueryContext(ctx, query, ownerID)
		if err != nil {
			return fmt.Errorf("select %s: %w", tableName, err)
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("columns %s: %w", tableName, err)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
		insert := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)", tableName, columnOrder, placeholders)

		for rows.Next() {
			values := make([]any, len(cols))
			valuePtrs := make([]any, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				return fmt.Errorf("scan %s row: %w", tableName, err)
			}
			if _, err := tx.ExecContext(ctx, insert, values...); err != nil {
				return fmt.Errorf("insert %s row: %w", tableName, err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate %s rows: %w", tableName, err)
		}
		return nil
	}

	// Order matters: tables referenced by foreign keys must be inserted first.
	// We disable foreign keys above, but keep a sensible order for clarity.
	migrations := []struct {
		table       string
		ownerColumn string
		columns     string
		filter      string
	}{
		{
			table:       "email_messages",
			ownerColumn: "mailbox_owner_id",
			columns:     "id, provider, provider_message_id, provider_event_id, direction, mailbox_owner_id, alias_id, from_address, from_display, subject, text_body, html_body, raw_headers_json, raw_message_ref, authentication_results_json, trust_status, read_at, received_at, sent_at, created_at",
			filter:      "mailbox_owner_id = ?",
		},
		{
			table:       "email_message_recipients",
			ownerColumn: "message_id",
			columns:     "id, message_id, kind, address, display",
			filter:      "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)",
		},
		{
			table:       "email_attachments",
			ownerColumn: "message_id",
			columns:     "id, message_id, provider_attachment_id, filename, content_type, content_disposition, content_id, size_bytes, storage_ref, status, created_at",
			filter:      "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)",
		},
		{
			table:       "email_source_packets",
			ownerColumn: "message_id",
			columns:     "id, message_id, attachment_id, trust_label, provenance_json, text_ref, created_at",
			filter:      "message_id IN (SELECT id FROM email_messages WHERE mailbox_owner_id = ?)",
		},
		{
			table:       "email_ingress_events",
			ownerColumn: "owner_id",
			columns:     "id, message_id, source_packet_id, owner_id, conductor_submission_id, status, created_at, completed_at",
			filter:      "owner_id = ?",
		},
		{
			table:       "email_drafts",
			ownerColumn: "owner_id",
			columns:     "id, owner_id, from_alias_id, from_address, to_json, cc_json, bcc_json, subject, text_body, html_body, reply_to_message_id, source_kind, source_ref, status, version, version_hash, sent_message_id, provider_message_id, created_at, updated_at",
			filter:      "owner_id = ?",
		},
		{
			table:       "email_draft_approval_events",
			ownerColumn: "owner_id",
			columns:     "id, draft_id, owner_id, version, version_hash, event_type, provider_message_id, created_at",
			filter:      "owner_id = ?",
		},
		{
			table:       "email_draft_approval_tokens",
			ownerColumn: "owner_id",
			columns:     "id, token, draft_id, owner_id, version, version_hash, approval_email, status, provider_message_id, created_at, expires_at, used_at",
			filter:      "owner_id = ?",
		},
		{
			table:       "email_risk_alerts",
			ownerColumn: "owner_id",
			columns:     "id, owner_id, risk_kind, source_ref, snippet, provider_message_id, created_at",
			filter:      "owner_id = ?",
		},
	}

	for _, m := range migrations {
		if err := copyRows(m.table, m.ownerColumn, m.columns, m.filter); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("re-enable foreign keys: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit per-owner migration: %w", err)
	}
	return nil
}

// repairCachedMailboxes runs per-user repairs on all already-opened mailbox
// databases. This is called after EnsureSchema so that a subsequent
// EnsureSchema call (e.g. in tests or on restart) repairs already-opened
// mailboxes.
func (s *Store) repairCachedMailboxes() error {
	s.mu.Lock()
	dbs := make([]*sql.DB, 0, len(s.mailboxes))
	for _, db := range s.mailboxes {
		dbs = append(dbs, db)
	}
	s.mu.Unlock()
	for _, db := range dbs {
		if err := repairSentDraftApprovalTokens(db); err != nil {
			return err
		}
		if err := repairRejectedDrafts(db); err != nil {
			return err
		}
	}
	return nil
}

// ensureMailboxSchema creates per-user tables in a mailbox database and runs
// per-user repairs.
func ensureMailboxSchema(db *sql.DB) error {
	stmts := []string{
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
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("mailbox schema migration: %w", err)
		}
	}
	if err := repairSentDraftApprovalTokens(db); err != nil {
		return err
	}
	return repairRejectedDrafts(db)
}

func repairSentDraftApprovalTokens(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`UPDATE email_draft_approval_tokens
		SET status = 'stale_sent', used_at = coalesce(used_at, ?)
		WHERE status = 'active'
			AND EXISTS (
				SELECT 1 FROM email_drafts d
				WHERE d.id = email_draft_approval_tokens.draft_id
					AND d.owner_id = email_draft_approval_tokens.owner_id
					AND d.status = 'sent'
			)`, now)
	if err != nil {
		return fmt.Errorf("repair sent draft approval tokens: %w", err)
	}
	return nil
}

func repairRejectedDrafts(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`UPDATE email_drafts
		SET status = 'draft_rejected', updated_at = ?
		WHERE status = 'draft_pending_owner_approval'
			AND EXISTS (
				SELECT 1 FROM email_draft_approval_tokens t
				WHERE t.draft_id = email_drafts.id
					AND t.owner_id = email_drafts.owner_id
					AND t.version = email_drafts.version
					AND t.version_hash = email_drafts.version_hash
					AND t.status = 'rejected'
			)
			AND NOT EXISTS (
				SELECT 1 FROM email_draft_approval_tokens t
				WHERE t.draft_id = email_drafts.id
					AND t.owner_id = email_drafts.owner_id
					AND t.version = email_drafts.version
					AND t.version_hash = email_drafts.version_hash
					AND t.status = 'active'
			)`, now)
	if err != nil {
		return fmt.Errorf("repair rejected drafts: %w", err)
	}
	return nil
}

func (s *Store) seedDefaults(cfg *Config) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.routingDB.Exec(`INSERT OR IGNORE INTO email_receive_policies (
		id, name, allow_public_inbound, allow_attachments, require_sender_whitelist,
		require_secret_alias, allow_auto_agent_read, allow_auto_agent_write,
		allow_auto_outbound_send, quarantine_by_default, created_at
	) VALUES (?, ?, 1, 0, 0, 0, 1, 0, 0, 1, ?)`,
		DefaultPublicPolicyID, "public numeric inbound", now); err != nil {
		return fmt.Errorf("seed public policy: %w", err)
	}
	if _, err := s.routingDB.Exec(`INSERT OR IGNORE INTO email_aliases (
		id, domain, local_part, canonical_number, target_type, target_id,
		visibility, receive_policy_id, created_at
	) VALUES (?, ?, '000', 0, 'user', ?, 'public', ?, ?)`,
		DefaultRootAliasID, cfg.PrimaryDomain, cfg.RootOwnerID, DefaultPublicPolicyID, now); err != nil {
		return fmt.Errorf("seed 000 alias: %w", err)
	}
	if _, err := s.routingDB.Exec(`UPDATE email_aliases
		SET domain = ?, local_part = '000', canonical_number = 0, target_type = 'user',
			target_id = ?, visibility = 'public', receive_policy_id = ?, disabled_at = NULL
		WHERE id = ?`,
		cfg.PrimaryDomain, cfg.RootOwnerID, DefaultPublicPolicyID, DefaultRootAliasID); err != nil {
		return fmt.Errorf("reconcile 000 alias: %w", err)
	}
	if _, err := s.routingDB.Exec(`INSERT OR IGNORE INTO email_receive_policies (
		id, name, allow_public_inbound, allow_attachments, require_sender_whitelist,
		require_secret_alias, allow_auto_agent_read, allow_auto_agent_write,
		allow_auto_outbound_send, quarantine_by_default, created_at
	) VALUES (?, ?, 0, 0, 1, 1, 1, 0, 0, 1, ?)`,
		DefaultTrustedWorkflowPolicyID, "trusted plus-code workflow", now); err != nil {
		return fmt.Errorf("seed trusted workflow policy: %w", err)
	}
	return nil
}

// ConfigureTrustedWorkflowAlias creates or updates an owner-scoped plus-code
// alias and whitelists one sender for trusted workflow handoff.
func (s *Store) ConfigureTrustedWorkflowAlias(ctx context.Context, config TrustedWorkflowAliasConfig) (EmailAlias, error) {
	ownerID := strings.TrimSpace(config.OwnerID)
	domain := strings.ToLower(strings.TrimSpace(config.Domain))
	localPart := strings.ToLower(strings.TrimSpace(config.LocalPart))
	senderAddress := strings.ToLower(strings.TrimSpace(config.SenderAddress))
	if ownerID == "" || domain == "" || localPart == "" || senderAddress == "" {
		return EmailAlias{}, fmt.Errorf("owner, domain, local part, and sender address are required")
	}
	if !strings.Contains(localPart, "+") {
		return EmailAlias{}, fmt.Errorf("trusted workflow aliases must use a plus-code local part")
	}
	if _, _, ok := splitEmailAddress(senderAddress); !ok {
		return EmailAlias{}, fmt.Errorf("sender address is invalid")
	}
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return EmailAlias{}, fmt.Errorf("begin trusted workflow alias tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	var existingID, existingOwner string
	err = tx.QueryRowContext(ctx, `SELECT id, target_id FROM email_aliases WHERE domain = ? AND local_part = ?`, domain, localPart).Scan(&existingID, &existingOwner)
	if err != nil && err != sql.ErrNoRows {
		return EmailAlias{}, fmt.Errorf("load alias: %w", err)
	}
	if err == nil && existingOwner != ownerID {
		return EmailAlias{}, fmt.Errorf("alias is owned by a different target")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	aliasID := existingID
	if aliasID == "" {
		aliasID = aliasRowID(domain, localPart)
		if _, err := tx.ExecContext(ctx, `INSERT INTO email_aliases (
			id, domain, local_part, canonical_number, target_type, target_id,
			visibility, receive_policy_id, created_at
		) VALUES (?, ?, ?, ?, 'user', ?, 'unlisted', ?, ?)`,
			aliasID, domain, localPart, canonicalNumberFromLocalPart(localPart), ownerID, DefaultTrustedWorkflowPolicyID, now); err != nil {
			return EmailAlias{}, fmt.Errorf("insert trusted workflow alias: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE email_aliases
		SET target_type = 'user', target_id = ?, visibility = 'unlisted',
			receive_policy_id = ?, disabled_at = NULL
		WHERE id = ?`,
		ownerID, DefaultTrustedWorkflowPolicyID, aliasID); err != nil {
		return EmailAlias{}, fmt.Errorf("update trusted workflow alias: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO email_sender_whitelist (
		id, owner_id, alias_id, sender_address, created_at
	) VALUES (?, ?, ?, ?, ?)`,
		senderWhitelistRowID(aliasID, senderAddress), ownerID, aliasID, senderAddress, now); err != nil {
		return EmailAlias{}, fmt.Errorf("insert sender whitelist: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailAlias{}, fmt.Errorf("commit trusted workflow alias tx: %w", err)
	}
	tx = nil
	return s.ResolveAlias(ctx, domain, localPart)
}

// ResolveAlias resolves a domain/local_part pair.
func (s *Store) ResolveAlias(ctx context.Context, domain, localPart string) (EmailAlias, error) {
	var alias EmailAlias
	err := s.routingDB.QueryRowContext(ctx, `SELECT
		id, domain, local_part, canonical_number, target_type, target_id, visibility, receive_policy_id
		FROM email_aliases
		WHERE domain = ? AND local_part = ? AND disabled_at IS NULL`,
		domain, localPart).Scan(
		&alias.ID,
		&alias.Domain,
		&alias.LocalPart,
		&alias.CanonicalNumber,
		&alias.TargetType,
		&alias.TargetID,
		&alias.Visibility,
		&alias.ReceivePolicyID,
	)
	if err != nil {
		return EmailAlias{}, err
	}
	return alias, nil
}

// GetReceivePolicy returns the receive policy attached to an alias.
func (s *Store) GetReceivePolicy(ctx context.Context, policyID string) (EmailReceivePolicy, error) {
	var policy EmailReceivePolicy
	var allowPublicInbound, allowAttachments, requireSenderWhitelist int
	var requireSecretAlias, allowAutoAgentRead, allowAutoAgentWrite int
	var allowAutoOutboundSend, quarantineByDefault int
	err := s.routingDB.QueryRowContext(ctx, `SELECT
		id, name, allow_public_inbound, allow_attachments, require_sender_whitelist,
		require_secret_alias, allow_auto_agent_read, allow_auto_agent_write,
		allow_auto_outbound_send, quarantine_by_default
		FROM email_receive_policies
		WHERE id = ?`, policyID).Scan(
		&policy.ID,
		&policy.Name,
		&allowPublicInbound,
		&allowAttachments,
		&requireSenderWhitelist,
		&requireSecretAlias,
		&allowAutoAgentRead,
		&allowAutoAgentWrite,
		&allowAutoOutboundSend,
		&quarantineByDefault,
	)
	if err != nil {
		return EmailReceivePolicy{}, err
	}
	policy.AllowPublicInbound = allowPublicInbound != 0
	policy.AllowAttachments = allowAttachments != 0
	policy.RequireSenderWhitelist = requireSenderWhitelist != 0
	policy.RequireSecretAlias = requireSecretAlias != 0
	policy.AllowAutoAgentRead = allowAutoAgentRead != 0
	policy.AllowAutoAgentWrite = allowAutoAgentWrite != 0
	policy.AllowAutoOutboundSend = allowAutoOutboundSend != 0
	policy.QuarantineByDefault = quarantineByDefault != 0
	return policy, nil
}

// IsSenderWhitelisted reports whether sender may use alias for trusted ingress.
func (s *Store) IsSenderWhitelisted(ctx context.Context, ownerID, aliasID, senderAddress string) (bool, error) {
	senderAddress = strings.ToLower(strings.TrimSpace(senderAddress))
	if senderAddress == "" {
		return false, nil
	}
	var count int
	if err := s.routingDB.QueryRowContext(ctx, `SELECT count(*)
		FROM email_sender_whitelist
		WHERE owner_id = ? AND alias_id = ? AND sender_address = ? AND disabled_at IS NULL`,
		ownerID, aliasID, senderAddress).Scan(&count); err != nil {
		return false, fmt.Errorf("check sender whitelist: %w", err)
	}
	return count > 0, nil
}

// ListAliases returns configured aliases for operator inspection.
func (s *Store) ListAliases(ctx context.Context) ([]EmailAlias, error) {
	rows, err := s.routingDB.QueryContext(ctx, `SELECT
		id, domain, local_part, coalesce(canonical_number, 0), target_type,
		target_id, visibility, receive_policy_id
		FROM email_aliases
		ORDER BY domain, local_part`)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	aliases := make([]EmailAlias, 0)
	for rows.Next() {
		var alias EmailAlias
		if err := rows.Scan(
			&alias.ID,
			&alias.Domain,
			&alias.LocalPart,
			&alias.CanonicalNumber,
			&alias.TargetType,
			&alias.TargetID,
			&alias.Visibility,
			&alias.ReceivePolicyID,
		); err != nil {
			return nil, fmt.Errorf("scan alias: %w", err)
		}
		aliases = append(aliases, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return aliases, nil
}

// RecordWebhookEvent stores a verified webhook event idempotently.
func (s *Store) RecordWebhookEvent(ctx context.Context, event WebhookEvent) (bool, error) {
	result, err := s.routingDB.ExecContext(ctx, `INSERT OR IGNORE INTO email_webhook_events (
		id, provider, provider_event_id, provider_message_id, event_type, raw_payload, received_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.Provider,
		event.ProviderEventID,
		event.ProviderMessageID,
		event.EventType,
		event.RawPayload,
		event.ReceivedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, fmt.Errorf("record webhook event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("record webhook rows affected: %w", err)
	}
	return rows == 1, nil
}

// CountWebhookEvents returns the number of stored webhook event rows.
func (s *Store) CountWebhookEvents(ctx context.Context) (int, error) {
	var count int
	if err := s.routingDB.QueryRowContext(ctx, `SELECT count(*) FROM email_webhook_events`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// HasProviderMessage reports whether a provider message was already stored.
// Uses the global provider message index in the routing database so the
// webhook flow can check without knowing the owner.
func (s *Store) HasProviderMessage(ctx context.Context, provider, providerMessageID string) (bool, error) {
	providerMessageID = strings.TrimSpace(providerMessageID)
	if providerMessageID == "" {
		return false, nil
	}
	var count int
	if err := s.routingDB.QueryRowContext(ctx, `SELECT count(*)
		FROM email_provider_message_index
		WHERE provider = ? AND provider_message_id = ?`,
		provider, providerMessageID).Scan(&count); err != nil {
		return false, fmt.Errorf("check provider message index: %w", err)
	}
	return count > 0, nil
}

// recordProviderMessageIndex records that a provider message was stored for an
// owner in the global routing index.
func (s *Store) recordProviderMessageIndex(ctx context.Context, provider, providerMessageID, ownerID string) error {
	providerMessageID = strings.TrimSpace(providerMessageID)
	if providerMessageID == "" || strings.TrimSpace(ownerID) == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.routingDB.ExecContext(ctx, `INSERT OR IGNORE INTO email_provider_message_index (
			provider, provider_message_id, owner_id, created_at
		) VALUES (?, ?, ?, ?)`,
		provider, providerMessageID, ownerID, now)
	if err != nil {
		return fmt.Errorf("record provider message index: %w", err)
	}
	return nil
}

// ListWebhookEvents returns recent provider webhook receipts without raw payloads.
func (s *Store) ListWebhookEvents(ctx context.Context, limit int) ([]WebhookEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.routingDB.QueryContext(ctx, `SELECT
		id, provider, provider_event_id, coalesce(provider_message_id, ''), event_type, '', received_at
		FROM email_webhook_events
		ORDER BY received_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]WebhookEvent, 0)
	for rows.Next() {
		var event WebhookEvent
		var receivedAt string
		if err := rows.Scan(
			&event.ID,
			&event.Provider,
			&event.ProviderEventID,
			&event.ProviderMessageID,
			&event.EventType,
			&event.RawPayload,
			&receivedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		if receivedAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, receivedAt)
			if err != nil {
				return nil, fmt.Errorf("parse webhook received_at: %w", err)
			}
			event.ReceivedAt = parsed
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// Stats returns non-sensitive routing counters for service health. Per-user
// mailbox counts (messages, attachments, ingress events) are not aggregated
// across all mailbox databases.
func (s *Store) Stats(ctx context.Context) (StoreStats, error) {
	var stats StoreStats
	if err := s.routingDB.QueryRowContext(ctx, `SELECT count(*) FROM email_aliases`).Scan(&stats.Aliases); err != nil {
		return StoreStats{}, err
	}
	if err := s.routingDB.QueryRowContext(ctx, `SELECT count(*) FROM email_webhook_events`).Scan(&stats.WebhookEvents); err != nil {
		return StoreStats{}, err
	}
	return stats, nil
}

// ListMessages returns owner-visible messages for a simple v0 folder.
func (s *Store) ListMessages(ctx context.Context, ownerID, folder string, limit int) ([]EmailMessage, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	where := "mailbox_owner_id = ?"
	args := []any{ownerID}
	switch strings.ToLower(strings.TrimSpace(folder)) {
	case "", "inbox":
		where += " AND direction = 'inbound' AND trust_status <> 'quarantined'"
	case "sent":
		where += " AND direction = 'outbound'"
	case "quarantine":
		where += " AND trust_status = 'quarantined'"
	default:
		return nil, fmt.Errorf("unsupported folder %q", folder)
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, `SELECT
		id, provider, coalesce(provider_message_id, ''), coalesce(provider_event_id, ''),
		direction, mailbox_owner_id, coalesce(alias_id, ''), from_address,
		coalesce(from_display, ''), subject, coalesce(text_body, ''),
		coalesce(html_body, ''), coalesce(raw_headers_json, ''),
		coalesce(authentication_results_json, ''), trust_status, coalesce(read_at, ''),
		coalesce(received_at, ''), coalesce(sent_at, ''), created_at,
		EXISTS(SELECT 1 FROM email_attachments a WHERE a.message_id = email_messages.id)
		FROM email_messages
		WHERE `+where+`
		ORDER BY coalesce(received_at, sent_at, created_at) DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	messages := make([]EmailMessage, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

// GetMessage returns an owner-visible message by id.
func (s *Store) GetMessage(ctx context.Context, ownerID, messageID string) (EmailMessage, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return EmailMessage{}, err
	}
	row := db.QueryRowContext(ctx, `SELECT
		id, provider, coalesce(provider_message_id, ''), coalesce(provider_event_id, ''),
		direction, mailbox_owner_id, coalesce(alias_id, ''), from_address,
		coalesce(from_display, ''), subject, coalesce(text_body, ''),
		coalesce(html_body, ''), coalesce(raw_headers_json, ''),
		coalesce(authentication_results_json, ''), trust_status, coalesce(read_at, ''),
		coalesce(received_at, ''), coalesce(sent_at, ''), created_at,
		EXISTS(SELECT 1 FROM email_attachments a WHERE a.message_id = email_messages.id)
		FROM email_messages
		WHERE mailbox_owner_id = ? AND id = ?`, ownerID, messageID)
	return scanMessage(row)
}

// ListRecipients returns stored recipients for an owner-visible message.
func (s *Store) ListRecipients(ctx context.Context, ownerID, messageID string) ([]EmailRecipient, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT r.kind, r.address, coalesce(r.display, '')
		FROM email_message_recipients r
		JOIN email_messages m ON m.id = r.message_id
		WHERE m.mailbox_owner_id = ? AND r.message_id = ?
		ORDER BY CASE r.kind WHEN 'to' THEN 0 WHEN 'cc' THEN 1 WHEN 'bcc' THEN 2 ELSE 3 END, r.address`, ownerID, messageID)
	if err != nil {
		return nil, fmt.Errorf("list recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	recipients := make([]EmailRecipient, 0)
	for rows.Next() {
		var recipient EmailRecipient
		if err := rows.Scan(&recipient.Kind, &recipient.Address, &recipient.Display); err != nil {
			return nil, fmt.Errorf("scan recipient: %w", err)
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return recipients, nil
}

// ListAttachments returns attachment metadata for an owner-visible message.
func (s *Store) ListAttachments(ctx context.Context, ownerID, messageID string) ([]EmailAttachment, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT
		a.id, a.message_id, coalesce(a.provider_attachment_id, ''), a.filename,
		a.content_type, coalesce(a.size_bytes, 0), coalesce(a.storage_ref, ''),
		a.status, a.created_at
		FROM email_attachments a
		JOIN email_messages m ON m.id = a.message_id
		WHERE m.mailbox_owner_id = ? AND a.message_id = ?
		ORDER BY a.created_at`, ownerID, messageID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	attachments := make([]EmailAttachment, 0)
	for rows.Next() {
		var a EmailAttachment
		if err := rows.Scan(&a.ID, &a.MessageID, &a.ProviderAttachmentID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StorageRef, &a.Status, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

// GetSourcePacketForMessage returns the first owner-visible source packet for a message.
func (s *Store) GetSourcePacketForMessage(ctx context.Context, ownerID, messageID string) (EmailSourcePacket, EmailMessage, error) {
	msg, err := s.GetMessage(ctx, ownerID, messageID)
	if err != nil {
		return EmailSourcePacket{}, EmailMessage{}, err
	}
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return EmailSourcePacket{}, EmailMessage{}, err
	}
	var packet EmailSourcePacket
	err = db.QueryRowContext(ctx, `SELECT
		id, message_id, trust_label, provenance_json, coalesce(text_ref, ''), created_at
		FROM email_source_packets
		WHERE message_id = ?
		ORDER BY created_at
		LIMIT 1`, messageID).Scan(&packet.ID, &packet.MessageID, &packet.TrustLabel, &packet.ProvenanceJSON, &packet.TextRef, &packet.CreatedAt)
	if err != nil {
		return EmailSourcePacket{}, EmailMessage{}, err
	}
	return packet, msg, nil
}

// ListIngressEvents returns read-only owner-visible MAS handoff records.
func (s *Store) ListIngressEvents(ctx context.Context, ownerID, messageID string, limit int) ([]EmailIngressEvent, error) {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	where := "owner_id = ?"
	args := []any{ownerID}
	if strings.TrimSpace(messageID) != "" {
		where += " AND message_id = ?"
		args = append(args, messageID)
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, `SELECT
		id, message_id, coalesce(source_packet_id, ''), owner_id,
		coalesce(conductor_submission_id, ''), status, created_at, coalesce(completed_at, '')
		FROM email_ingress_events
		WHERE `+where+`
		ORDER BY created_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list ingress events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]EmailIngressEvent, 0)
	for rows.Next() {
		var event EmailIngressEvent
		if err := rows.Scan(
			&event.ID,
			&event.MessageID,
			&event.SourcePacketID,
			&event.OwnerID,
			&event.ConductorSubmissionID,
			&event.Status,
			&event.CreatedAt,
			&event.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ingress event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// RecordIngressEvent stores an owner-triggered MAS handoff receipt.
func (s *Store) RecordIngressEvent(ctx context.Context, event EmailIngressEvent) error {
	db, err := s.mailboxForOwner(event.OwnerID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO email_ingress_events (
		id, message_id, source_packet_id, owner_id, conductor_submission_id,
		status, created_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.MessageID,
		nullString(event.SourcePacketID),
		event.OwnerID,
		nullString(event.ConductorSubmissionID),
		event.Status,
		event.CreatedAt,
		nullString(event.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("record ingress event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record ingress event rows: %w", err)
	}
	if rows > 0 {
		return nil
	}
	var existing EmailIngressEvent
	err = db.QueryRowContext(ctx, `SELECT
		id, message_id, coalesce(source_packet_id, ''), owner_id,
		coalesce(conductor_submission_id, ''), status, created_at, coalesce(completed_at, '')
		FROM email_ingress_events
		WHERE id = ?`, event.ID).Scan(
		&existing.ID,
		&existing.MessageID,
		&existing.SourcePacketID,
		&existing.OwnerID,
		&existing.ConductorSubmissionID,
		&existing.Status,
		&existing.CreatedAt,
		&existing.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("record ingress event lookup: %w", err)
	}
	if existing.MessageID != event.MessageID ||
		existing.SourcePacketID != strings.TrimSpace(event.SourcePacketID) ||
		existing.OwnerID != event.OwnerID ||
		existing.ConductorSubmissionID != strings.TrimSpace(event.ConductorSubmissionID) {
		return fmt.Errorf("record ingress event: conflicting duplicate id")
	}
	return nil
}

// MarkMessageRead marks a message read for its owner.
func (s *Store) MarkMessageRead(ctx context.Context, ownerID, messageID string, readAt time.Time) error {
	db, err := s.mailboxForOwner(ownerID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE email_messages SET read_at = ? WHERE mailbox_owner_id = ? AND id = ?`, readAt.UTC().Format(time.RFC3339Nano), ownerID, messageID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark read rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessage(row messageScanner) (EmailMessage, error) {
	var msg EmailMessage
	if err := row.Scan(
		&msg.ID,
		&msg.Provider,
		&msg.ProviderMessageID,
		&msg.ProviderEventID,
		&msg.Direction,
		&msg.MailboxOwnerID,
		&msg.AliasID,
		&msg.FromAddress,
		&msg.FromDisplay,
		&msg.Subject,
		&msg.TextBody,
		&msg.HTMLBody,
		&msg.RawHeadersJSON,
		&msg.AuthenticationResultsJSON,
		&msg.TrustStatus,
		&msg.ReadAt,
		&msg.ReceivedAt,
		&msg.SentAt,
		&msg.CreatedAt,
		&msg.HasAttachments,
	); err != nil {
		return EmailMessage{}, err
	}
	return msg, nil
}
