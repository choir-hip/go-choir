// sqlite.go implements a persistent append-only journal backed by SQLite
// (modernc.org/sqlite). The schema stores one row per event plus a
// device_cursors table for per-device sync positions. The hash chain is
// stored alongside each event so VerifyChain can detect tampering even
// if rows are edited directly in the database.

package journal

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS journal_events (
    event_id        TEXT    PRIMARY KEY,
    parent_event_id TEXT    NOT NULL,
    cursor_seq      INTEGER NOT NULL UNIQUE,
    owner_id        TEXT    NOT NULL,
    item_id         TEXT    NOT NULL,
    device_id       TEXT    NOT NULL,
    subject_id      TEXT    NOT NULL,
    event_type      TEXT    NOT NULL,
    kind            TEXT    NOT NULL,
    blob_ref        TEXT    NOT NULL,
    payload_json    TEXT    NOT NULL,
    hash            TEXT    NOT NULL,
    created_at      TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS device_cursors (
    owner_id    TEXT    NOT NULL DEFAULT '',
    device_id   TEXT    NOT NULL,
    cursor_seq  INTEGER NOT NULL,
    PRIMARY KEY (owner_id, device_id)
);
`

const insertSQL = `INSERT INTO journal_events
    (event_id, parent_event_id, cursor_seq, owner_id, item_id, device_id,
     subject_id, event_type, kind, blob_ref, payload_json, hash, created_at)
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`

const selectAllSQL = `SELECT event_id, parent_event_id, cursor_seq, owner_id,
    item_id, device_id, subject_id, event_type, kind, blob_ref, payload_json,
    hash, created_at FROM journal_events ORDER BY cursor_seq`

const selectUpToSQL = `SELECT event_id, parent_event_id, cursor_seq, owner_id,
    item_id, device_id, subject_id, event_type, kind, blob_ref, payload_json,
    hash, created_at FROM journal_events WHERE cursor_seq <= ? ORDER BY cursor_seq`

const selectHeadSQL = `SELECT COALESCE(MAX(cursor_seq), 0) FROM journal_events`
const selectLastByItemSQL = `SELECT event_id FROM journal_events WHERE owner_id = ? AND item_id = ?
    ORDER BY cursor_seq DESC LIMIT 1`

// SQLiteJournal is a persistent append-only event store backed by SQLite.
type SQLiteJournal struct {
	mu sync.Mutex
	db *sql.DB
}

// NewSQLiteJournal opens (or creates) a journal at the given SQLite file
// path. Use ":memory:" for an ephemeral in-database journal.
func NewSQLiteJournal(path string) (*SQLiteJournal, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("journal: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 60000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("journal: configure sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("journal: create schema: %w", err)
	}
	if err := migrateDeviceCursorsOwnerScope(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteJournal{db: db}, nil
}

// Append adds an event to the SQLite journal.
func (j *SQLiteJournal) Append(evt model.Event) (Entry, error) {
	if err := validateEvent(evt); err != nil {
		return Entry{}, err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	ctx := context.Background()
	conn, err := j.db.Conn(ctx)
	if err != nil {
		return Entry{}, fmt.Errorf("journal: get append connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return Entry{}, fmt.Errorf("journal: begin append transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	var head int64
	if err := conn.QueryRowContext(ctx, selectHeadSQL).Scan(&head); err != nil {
		return Entry{}, fmt.Errorf("journal: read head: %w", err)
	}

	if evt.CursorSeq == 0 {
		evt.CursorSeq = head + 1
	}
	if evt.CursorSeq <= head {
		return Entry{}, fmt.Errorf("journal: cursor seq %d is not greater than head %d (append-only)", evt.CursorSeq, head)
	}

	// Look up the previous event for this item.
	var expectedParent string
	if err := conn.QueryRowContext(ctx, selectLastByItemSQL, evt.OwnerID, evt.ItemID).Scan(&expectedParent); err != nil && err != sql.ErrNoRows {
		return Entry{}, fmt.Errorf("journal: read last event for item: %w", err)
	}

	if evt.ParentEventID == "" {
		evt.ParentEventID = model.EventID(expectedParent)
	} else if expectedParent == "" {
		return Entry{}, fmt.Errorf("journal: first event for item %s must not declare parent event %q",
			evt.ItemID, evt.ParentEventID)
	} else if expectedParent != "" && evt.ParentEventID != model.EventID(expectedParent) {
		return Entry{}, fmt.Errorf("journal: parent event %q does not match known predecessor %q for item %s",
			evt.ParentEventID, expectedParent, evt.ItemID)
	}

	// Compute parent hash.
	parentHash := ""
	if expectedParent != "" {
		var ph string
		if err := conn.QueryRowContext(ctx, `SELECT hash FROM journal_events WHERE event_id = ?`, expectedParent).Scan(&ph); err != nil {
			return Entry{}, fmt.Errorf("journal: read parent hash: %w", err)
		}
		parentHash = ph
	}

	hash := computeHash(evt, parentHash)

	_, err = conn.ExecContext(ctx, insertSQL,
		evt.EventID, evt.ParentEventID, evt.CursorSeq, evt.OwnerID, evt.ItemID,
		evt.DeviceID, evt.SubjectID, evt.EventType, evt.Kind, evt.BlobRef,
		evt.PayloadJSON, hash, evt.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Entry{}, fmt.Errorf("journal: insert event: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return Entry{}, fmt.Errorf("journal: commit append: %w", err)
	}
	committed = true

	return Entry{Event: evt, Hash: hash}, nil
}

// Entries returns all journal entries in CursorSeq order.
func (j *SQLiteJournal) Entries() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	rows, err := j.db.Query(selectAllSQL)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanEntries(rows)
}

// EntriesUpTo returns entries with CursorSeq <= maxSeq, in CursorSeq order.
func (j *SQLiteJournal) EntriesUpTo(maxSeq int64) []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	rows, err := j.db.Query(selectUpToSQL, maxSeq)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanEntries(rows)
}

// VerifyChain walks every entry and confirms the hash chain is intact.
func (j *SQLiteJournal) VerifyChain() error {
	entries := j.Entries()
	if entries == nil {
		return fmt.Errorf("journal: failed to read entries for chain verification")
	}
	return verifyChain(entries)
}

// Close releases the database handle.
func (j *SQLiteJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.db.Close()
}

// scanEntries reads rows into a slice of Entry. The rows are already
// ordered by cursor_seq.
func scanEntries(rows *sql.Rows) []Entry {
	var out []Entry
	for rows.Next() {
		var e Entry
		var eventType, kind, blobRef, payloadJSON, hash, createdAt string
		if err := rows.Scan(
			&e.Event.EventID, &e.Event.ParentEventID, &e.Event.CursorSeq,
			&e.Event.OwnerID, &e.Event.ItemID, &e.Event.DeviceID,
			&e.Event.SubjectID, &eventType, &kind, &blobRef,
			&payloadJSON, &hash, &createdAt,
		); err != nil {
			return nil
		}
		e.Event.EventType = model.EventType(eventType)
		e.Event.Kind = model.ItemKind(kind)
		e.Event.BlobRef = model.BlobRef(blobRef)
		e.Event.PayloadJSON = payloadJSON
		e.Hash = hash
		e.Event.CreatedAt = parseTime(createdAt)
		out = append(out, e)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Event.CursorSeq < out[k].Event.CursorSeq })
	return out
}

// parseTime parses a timestamp stored as RFC3339Nano. If parsing fails the
// zero value is returned — the chain hash is computed over the raw string
// encoding, not the parsed time, so a parse failure does not break chain
// verification.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
