package maild

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultPublicPolicyID = "policy-public-inbound-v0"
	DefaultRootAliasID    = "alias-choir-news-000"
)

// Store is maild's SQLite-backed durable state.
type Store struct {
	db *sql.DB
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

// StoreStats is a safe operational summary for health reporting.
type StoreStats struct {
	Aliases                int `json:"aliases"`
	Messages               int `json:"messages"`
	QuarantinedAttachments int `json:"quarantined_attachments"`
	WebhookEvents          int `json:"webhook_events"`
	IngressEvents          int `json:"ingress_events"`
}

// OpenStore opens a maild SQLite store.
func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the store.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// EnsureSchema creates the v0 schema and seeds the founder/root alias.
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
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("schema migration: %w", err)
		}
	}
	return s.seedDefaults(cfg)
}

func (s *Store) seedDefaults(cfg *Config) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO email_receive_policies (
		id, name, allow_public_inbound, allow_attachments, require_sender_whitelist,
		require_secret_alias, allow_auto_agent_read, allow_auto_agent_write,
		allow_auto_outbound_send, quarantine_by_default, created_at
	) VALUES (?, ?, 1, 0, 0, 0, 1, 0, 0, 1, ?)`,
		DefaultPublicPolicyID, "public numeric inbound", now); err != nil {
		return fmt.Errorf("seed public policy: %w", err)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO email_aliases (
		id, domain, local_part, canonical_number, target_type, target_id,
		visibility, receive_policy_id, created_at
	) VALUES (?, ?, '000', 0, 'user', ?, 'public', ?, ?)`,
		DefaultRootAliasID, cfg.PrimaryDomain, cfg.RootOwnerID, DefaultPublicPolicyID, now); err != nil {
		return fmt.Errorf("seed 000 alias: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE email_aliases
		SET domain = ?, local_part = '000', canonical_number = 0, target_type = 'user',
			target_id = ?, visibility = 'public', receive_policy_id = ?, disabled_at = NULL
		WHERE id = ?`,
		cfg.PrimaryDomain, cfg.RootOwnerID, DefaultPublicPolicyID, DefaultRootAliasID); err != nil {
		return fmt.Errorf("reconcile 000 alias: %w", err)
	}
	return nil
}

// ResolveAlias resolves a domain/local_part pair.
func (s *Store) ResolveAlias(ctx context.Context, domain, localPart string) (EmailAlias, error) {
	var alias EmailAlias
	err := s.db.QueryRowContext(ctx, `SELECT
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
	err := s.db.QueryRowContext(ctx, `SELECT
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
	if err := s.db.QueryRowContext(ctx, `SELECT count(*)
		FROM email_sender_whitelist
		WHERE owner_id = ? AND alias_id = ? AND sender_address = ? AND disabled_at IS NULL`,
		ownerID, aliasID, senderAddress).Scan(&count); err != nil {
		return false, fmt.Errorf("check sender whitelist: %w", err)
	}
	return count > 0, nil
}

// ListAliases returns configured aliases for operator inspection.
func (s *Store) ListAliases(ctx context.Context) ([]EmailAlias, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
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
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO email_webhook_events (
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
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_webhook_events`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// HasProviderMessage reports whether a provider message was already stored.
func (s *Store) HasProviderMessage(ctx context.Context, provider, providerMessageID string) (bool, error) {
	providerMessageID = strings.TrimSpace(providerMessageID)
	if providerMessageID == "" {
		return false, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*)
		FROM email_messages
		WHERE provider = ? AND provider_message_id = ?`,
		provider, providerMessageID).Scan(&count); err != nil {
		return false, fmt.Errorf("check provider message: %w", err)
	}
	return count > 0, nil
}

// ListWebhookEvents returns recent provider webhook receipts without raw payloads.
func (s *Store) ListWebhookEvents(ctx context.Context, limit int) ([]WebhookEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT
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

// Stats returns non-sensitive mailbox counters for service health.
func (s *Store) Stats(ctx context.Context) (StoreStats, error) {
	var stats StoreStats
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_aliases`).Scan(&stats.Aliases); err != nil {
		return StoreStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_messages`).Scan(&stats.Messages); err != nil {
		return StoreStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_attachments WHERE status = 'quarantined'`).Scan(&stats.QuarantinedAttachments); err != nil {
		return StoreStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_webhook_events`).Scan(&stats.WebhookEvents); err != nil {
		return StoreStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_ingress_events`).Scan(&stats.IngressEvents); err != nil {
		return StoreStats{}, err
	}
	return stats, nil
}

// ListMessages returns owner-visible messages for a simple v0 folder.
func (s *Store) ListMessages(ctx context.Context, ownerID, folder string, limit int) ([]EmailMessage, error) {
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
	rows, err := s.db.QueryContext(ctx, `SELECT
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
	row := s.db.QueryRowContext(ctx, `SELECT
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
	rows, err := s.db.QueryContext(ctx, `SELECT r.kind, r.address, coalesce(r.display, '')
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
	rows, err := s.db.QueryContext(ctx, `SELECT
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
	var packet EmailSourcePacket
	err = s.db.QueryRowContext(ctx, `SELECT
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
	rows, err := s.db.QueryContext(ctx, `SELECT
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
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO email_ingress_events (
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
	err = s.db.QueryRowContext(ctx, `SELECT
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
	result, err := s.db.ExecContext(ctx, `UPDATE email_messages SET read_at = ? WHERE mailbox_owner_id = ? AND id = ?`, readAt.UTC().Format(time.RFC3339Nano), ownerID, messageID)
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
