package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DeskPackageEvent is one row of the desk package event log: an opaque JSON
// body (internal/appdesk owns its shape) at a sequence the store assigns.
type DeskPackageEvent struct {
	Seq       uint64
	Body      json.RawMessage
	CreatedAt time.Time
}

// AppendDeskPackageEvent appends a fact to the log and returns its sequence.
// Sequences only increase, so the sequence is a desk epoch: a later event is a
// later epoch. The log is append-only; there is no update or delete.
func (s *Store) AppendDeskPackageEvent(ctx context.Context, body json.RawMessage) (uint64, error) {
	if len(body) == 0 || !json.Valid(body) {
		return 0, errors.New("store: a desk package event needs a JSON body")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO desk_package_events (body, created_at) VALUES (?, ?)`, string(body), time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("store: append desk package event: %w", err)
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: desk package event sequence: %w", err)
	}
	return uint64(seq), nil
}

// ListDeskPackageEvents returns the log in sequence order.
func (s *Store) ListDeskPackageEvents(ctx context.Context) ([]DeskPackageEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq, body, created_at FROM desk_package_events ORDER BY seq`)
	if err != nil {
		return nil, fmt.Errorf("store: list desk package events: %w", err)
	}
	defer rows.Close()
	var out []DeskPackageEvent
	for rows.Next() {
		var ev DeskPackageEvent
		var body string
		if err := rows.Scan(&ev.Seq, &body, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan desk package event: %w", err)
		}
		ev.Body = json.RawMessage(body)
		out = append(out, ev)
	}
	return out, rows.Err()
}
