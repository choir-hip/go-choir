package journal

import (
	"database/sql"
	"fmt"
)

const selectOwnerCursorSQL = `SELECT cursor_seq FROM device_cursors WHERE owner_id = ? AND device_id = ?`
const upsertOwnerCursorSQL = `INSERT INTO device_cursors (owner_id, device_id, cursor_seq) VALUES (?, ?, ?)
    ON CONFLICT(owner_id, device_id) DO UPDATE SET cursor_seq = excluded.cursor_seq`

func migrateDeviceCursorsOwnerScope(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(device_cursors)`)
	if err != nil {
		return fmt.Errorf("journal: inspect cursor schema: %w", err)
	}
	defer rows.Close()

	hasOwnerID := false
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("journal: scan cursor schema: %w", err)
		}
		if name == "owner_id" {
			hasOwnerID = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("journal: read cursor schema: %w", err)
	}
	if hasOwnerID {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("journal: begin cursor schema migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`CREATE TABLE device_cursors_owner_scope (
		owner_id TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL,
		cursor_seq INTEGER NOT NULL,
		PRIMARY KEY (owner_id, device_id)
	)`); err != nil {
		return fmt.Errorf("journal: create owner-scoped cursor table: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO device_cursors_owner_scope (owner_id, device_id, cursor_seq)
		SELECT '', device_id, cursor_seq FROM device_cursors`); err != nil {
		return fmt.Errorf("journal: copy legacy cursors: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE device_cursors`); err != nil {
		return fmt.Errorf("journal: drop legacy cursor table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE device_cursors_owner_scope RENAME TO device_cursors`); err != nil {
		return fmt.Errorf("journal: rename owner-scoped cursor table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("journal: commit cursor schema migration: %w", err)
	}
	committed = true
	return nil
}

func (j *SQLiteJournal) Cursor(deviceID string) int64 {
	return j.CursorForOwner("", deviceID)
}

func (j *SQLiteJournal) CursorForOwner(ownerID, deviceID string) int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	var seq int64
	if err := j.db.QueryRow(selectOwnerCursorSQL, ownerID, deviceID).Scan(&seq); err != nil {
		return 0
	}
	return seq
}

func (j *SQLiteJournal) SetCursor(deviceID string, seq int64) error {
	return j.SetCursorForOwner("", deviceID, seq)
}

func (j *SQLiteJournal) SetCursorForOwner(ownerID, deviceID string, seq int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if seq < 0 {
		return fmt.Errorf("journal: cursor seq must be non-negative, got %d", seq)
	}
	var head int64
	if err := j.db.QueryRow(selectHeadSQL).Scan(&head); err != nil {
		return fmt.Errorf("journal: read head: %w", err)
	}
	if seq > head {
		return fmt.Errorf("journal: cursor seq %d exceeds journal head %d", seq, head)
	}
	_, err := j.db.Exec(upsertOwnerCursorSQL, ownerID, deviceID, seq)
	return err
}
