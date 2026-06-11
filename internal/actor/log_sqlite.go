package actor

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLiteLog is the durable update log over a SQLite database. It can share
// the runtime's database or use its own file; the schema is self-contained.
type SQLiteLog struct {
	db *sql.DB
}

// NewSQLiteLog initializes the schema and returns a log over db.
func NewSQLiteLog(db *sql.DB) (*SQLiteLog, error) {
	const schema = `
CREATE TABLE IF NOT EXISTS actor_updates (
  update_id     TEXT PRIMARY KEY,
  to_agent_id   TEXT NOT NULL,
  from_agent_id TEXT NOT NULL DEFAULT '',
  kind          TEXT NOT NULL DEFAULT '',
  content       TEXT NOT NULL DEFAULT '',
  trajectory_id TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMP NOT NULL,
  processed_at  TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_actor_updates_backlog
  ON actor_updates(to_agent_id, created_at) WHERE processed_at IS NULL;
CREATE TABLE IF NOT EXISTS actor_snapshots (
  agent_id   TEXT PRIMARY KEY,
  memory     BLOB,
  updated_at TIMESTAMP NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("actor log schema: %w", err)
	}
	return &SQLiteLog{db: db}, nil
}

func (l *SQLiteLog) Append(ctx context.Context, u Update) (bool, error) {
	res, err := l.db.ExecContext(ctx, `
INSERT INTO actor_updates (update_id, to_agent_id, from_agent_id, kind, content, trajectory_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(update_id) DO NOTHING`,
		u.UpdateID, u.ToAgentID, u.FromAgentID, u.Kind, u.Content, u.TrajectoryID, u.CreatedAt.UTC())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (l *SQLiteLog) Unprocessed(ctx context.Context, agentID string) ([]Update, error) {
	rows, err := l.db.QueryContext(ctx, `
SELECT update_id, to_agent_id, from_agent_id, kind, content, trajectory_id, created_at
FROM actor_updates
WHERE to_agent_id = ? AND processed_at IS NULL
ORDER BY created_at, update_id`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Update
	for rows.Next() {
		var u Update
		if err := rows.Scan(&u.UpdateID, &u.ToAgentID, &u.FromAgentID, &u.Kind, &u.Content, &u.TrajectoryID, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (l *SQLiteLog) MarkProcessed(ctx context.Context, agentID, updateID string) error {
	_, err := l.db.ExecContext(ctx, `
UPDATE actor_updates SET processed_at = ?
WHERE update_id = ? AND to_agent_id = ? AND processed_at IS NULL`,
		time.Now().UTC(), updateID, agentID)
	return err
}

func (l *SQLiteLog) AgentsWithBacklog(ctx context.Context) ([]string, error) {
	rows, err := l.db.QueryContext(ctx, `
SELECT DISTINCT to_agent_id FROM actor_updates WHERE processed_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var agentID string
		if err := rows.Scan(&agentID); err != nil {
			return nil, err
		}
		out = append(out, agentID)
	}
	return out, rows.Err()
}

func (l *SQLiteLog) SaveSnapshot(ctx context.Context, agentID string, memory []byte) error {
	_, err := l.db.ExecContext(ctx, `
INSERT INTO actor_snapshots (agent_id, memory, updated_at) VALUES (?, ?, ?)
ON CONFLICT(agent_id) DO UPDATE SET memory = excluded.memory, updated_at = excluded.updated_at`,
		agentID, memory, time.Now().UTC())
	return err
}

func (l *SQLiteLog) LoadSnapshot(ctx context.Context, agentID string) ([]byte, error) {
	var memory []byte
	err := l.db.QueryRowContext(ctx, `
SELECT memory FROM actor_snapshots WHERE agent_id = ?`, agentID).Scan(&memory)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return memory, err
}
