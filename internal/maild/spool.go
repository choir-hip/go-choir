package maild

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SpoolStatus indicates the lifecycle state of a message in the host spool queue.
type SpoolStatus string

const (
	SpoolStatusIncoming  SpoolStatus = "incoming"
	SpoolStatusDelivered SpoolStatus = "delivered"
	SpoolStatusFailed    SpoolStatus = "failed"
)

// SpooledMessage represents an in-flight email message stored in the host MTA spool.
type SpooledMessage struct {
	ID          string      `json:"id"`
	ComputerID  string      `json:"computer_id"`
	Recipient   string      `json:"recipient"`
	Sender      string      `json:"sender"`
	Subject     string      `json:"subject"`
	MessageID   string      `json:"message_id"`
	RawPath     string      `json:"raw_path"`
	Attempts    int         `json:"attempts"`
	NextRetryAt time.Time   `json:"next_retry_at"`
	Status      SpoolStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	DeliveredAt *time.Time  `json:"delivered_at,omitempty"`
}

// SpoolQueue provides an fsync'd, RFC 5321 compliant store-and-forward queue
// for in-flight email messages destined for persistent guest computers.
type SpoolQueue struct {
	mu        sync.Mutex
	spoolDir  string
	db        *sql.DB
}

// NewSpoolQueue initializes the spool queue directory structure and index database.
func NewSpoolQueue(spoolDir string) (*SpoolQueue, error) {
	spoolDir = filepath.Clean(spoolDir)
	incomingDir := filepath.Join(spoolDir, "incoming")
	if err := os.MkdirAll(incomingDir, 0o700); err != nil {
		return nil, fmt.Errorf("spool: create incoming dir: %w", err)
	}

	dbPath := filepath.Join(spoolDir, "spool.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)")
	if err != nil {
		return nil, fmt.Errorf("spool: open db: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS spool_queue (
		id TEXT PRIMARY KEY,
		computer_id TEXT NOT NULL,
		recipient TEXT NOT NULL,
		sender TEXT NOT NULL,
		subject TEXT NOT NULL,
		message_id TEXT NOT NULL,
		raw_path TEXT NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		next_retry_at DATETIME NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		delivered_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_spool_status_retry ON spool_queue (status, next_retry_at);
	CREATE INDEX IF NOT EXISTS idx_spool_computer ON spool_queue (computer_id, status);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("spool: init schema: %w", err)
	}

	return &SpoolQueue{
		spoolDir: spoolDir,
		db:       db,
	}, nil
}

// Close closes the underlying spool database.
func (q *SpoolQueue) Close() error {
	if q == nil || q.db == nil {
		return nil
	}
	return q.db.Close()
}

// Enqueue stores the raw EML bytes atomically and durably (fsync) and commits
// the spool record to SQLite. Returns the unique spool record ID.
func (q *SpoolQueue) Enqueue(ctx context.Context, computerID, recipient, sender, subject, messageID string, rawEML []byte) (string, error) {
	if q == nil || q.db == nil {
		return "", fmt.Errorf("spool: queue uninitialized")
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return "", fmt.Errorf("spool: computer_id is required")
	}

	spoolID, err := newRandomID("spool-")
	if err != nil {
		return "", err
	}

	incomingDir := filepath.Join(q.spoolDir, "incoming")
	tmpFile, err := os.CreateTemp(incomingDir, ".msg-*")
	if err != nil {
		return "", fmt.Errorf("spool: create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(rawEML); err != nil {
		return "", fmt.Errorf("spool: write raw eml: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("spool: fsync raw eml: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("spool: close temp eml: %w", err)
	}
	tmpFile = nil

	finalPath := filepath.Join(incomingDir, spoolID+".eml")
	if err := os.Rename(tmpName, finalPath); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("spool: install raw eml: %w", err)
	}

	// Fsync incoming directory so the directory entry is durable
	if dir, err := os.Open(incomingDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	now := time.Now().UTC()
	query := `
	INSERT INTO spool_queue (
		id, computer_id, recipient, sender, subject, message_id, raw_path, attempts, next_retry_at, status, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?);
	`
	if _, err := q.db.ExecContext(ctx, query, spoolID, computerID, recipient, sender, subject, messageID, finalPath, now, string(SpoolStatusIncoming), now); err != nil {
		_ = os.Remove(finalPath)
		return "", fmt.Errorf("spool: insert record: %w", err)
	}

	return spoolID, nil
}

// FetchPending returns a batch of messages ready for delivery.
func (q *SpoolQueue) FetchPending(ctx context.Context, maxBatch int) ([]*SpooledMessage, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("spool: queue uninitialized")
	}
	if maxBatch <= 0 {
		maxBatch = 50
	}

	now := time.Now().UTC()
	query := `
	SELECT id, computer_id, recipient, sender, subject, message_id, raw_path, attempts, next_retry_at, status, created_at, delivered_at
	FROM spool_queue
	WHERE status = ? AND next_retry_at <= ?
	ORDER BY created_at ASC
	LIMIT ?;
	`
	rows, err := q.db.QueryContext(ctx, query, string(SpoolStatusIncoming), now, maxBatch)
	if err != nil {
		return nil, fmt.Errorf("spool: query pending: %w", err)
	}
	defer rows.Close()

	var results []*SpooledMessage
	for rows.Next() {
		var msg SpooledMessage
		var statusStr string
		var deliveredAt sql.NullTime
		if err := rows.Scan(
			&msg.ID, &msg.ComputerID, &msg.Recipient, &msg.Sender, &msg.Subject,
			&msg.MessageID, &msg.RawPath, &msg.Attempts, &msg.NextRetryAt,
			&statusStr, &msg.CreatedAt, &deliveredAt,
		); err != nil {
			return nil, fmt.Errorf("spool: scan message: %w", err)
		}
		msg.Status = SpoolStatus(statusStr)
		if deliveredAt.Valid {
			msg.DeliveredAt = &deliveredAt.Time
		}
		results = append(results, &msg)
	}
	return results, rows.Err()
}

// MarkDelivered records that a message was accepted by the guest Maildir.
func (q *SpoolQueue) MarkDelivered(ctx context.Context, id string) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("spool: queue uninitialized")
	}
	now := time.Now().UTC()
	query := `
	UPDATE spool_queue
	SET status = ?, delivered_at = ?
	WHERE id = ?;
	`
	res, err := q.db.ExecContext(ctx, query, string(SpoolStatusDelivered), now, id)
	if err != nil {
		return fmt.Errorf("spool: mark delivered: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("spool: message %s not found", id)
	}
	return nil
}

// RecordAttemptFailure updates the attempt count and schedules exponential backoff.
func (q *SpoolQueue) RecordAttemptFailure(ctx context.Context, id string, nextRetryDelay time.Duration) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("spool: queue uninitialized")
	}
	nextRetry := time.Now().UTC().Add(nextRetryDelay)
	query := `
	UPDATE spool_queue
	SET attempts = attempts + 1, next_retry_at = ?
	WHERE id = ?;
	`
	_, err := q.db.ExecContext(ctx, query, nextRetry, id)
	return err
}

// PurgeDelivered deletes the raw EML file and removes the spool record after
// confirmation that a guest FileRootCommitted checkpoint covers the mailbox.
func (q *SpoolQueue) PurgeDelivered(ctx context.Context, id string) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("spool: queue uninitialized")
	}
	var rawPath string
	err := q.db.QueryRowContext(ctx, "SELECT raw_path FROM spool_queue WHERE id = ? AND status = ?", id, string(SpoolStatusDelivered)).Scan(&rawPath)
	if err != nil {
		return err
	}

	if rawPath != "" {
		_ = os.Remove(rawPath)
	}
	_, err = q.db.ExecContext(ctx, "DELETE FROM spool_queue WHERE id = ?", id)
	return err
}

// PurgeDeliveredBefore deletes all delivered spool messages for computerID whose
// delivery occurred at or before the given checkpoint time (e.g. FileRootCommitted).
func (q *SpoolQueue) PurgeDeliveredBefore(ctx context.Context, computerID string, checkpointTime time.Time) (int, error) {
	if q == nil || q.db == nil {
		return 0, fmt.Errorf("spool: queue uninitialized")
	}
	query := `
	SELECT id, raw_path
	FROM spool_queue
	WHERE computer_id = ? AND status = ? AND delivered_at <= ?;
	`
	rows, err := q.db.QueryContext(ctx, query, computerID, string(SpoolStatusDelivered), checkpointTime.UTC())
	if err != nil {
		return 0, fmt.Errorf("spool: query purgeable: %w", err)
	}
	defer rows.Close()

	var purged int
	var ids []string
	for rows.Next() {
		var id, rawPath string
		if err := rows.Scan(&id, &rawPath); err != nil {
			return purged, err
		}
		if rawPath != "" {
			_ = os.Remove(rawPath)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if _, err := q.db.ExecContext(ctx, "DELETE FROM spool_queue WHERE id = ?", id); err == nil {
			purged++
		}
	}
	return purged, nil
}

func newRandomID(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
