package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Media state (media_progress, media_recents, user_preferences) is an event
// projection: a tape-bound store appends a row snapshot op and the projector
// writes the row with the same upsert the direct-SQL seam uses, so replay
// rebuilds identical rows
// (docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md).
//
// Rows written before the tables were projections are not on the tape. The
// first tape-bound write of the family in a process therefore seeds: its
// batch holds an op for every existing row of the three tables, then its own
// op, so the write still wins for its key. The seed and the write are one
// event, so a crash leaves either both or neither. mediaStateMu is held
// across every bound write's snapshot and append, so no write lands between a
// seed's read and its append.

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execMediaProgress(ctx context.Context, ex sqlExecer, row computerevent.MediaProgressProjection) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO media_progress (
			owner_id, media_kind, media_identity_hash, media_identity,
			position_seconds, duration_seconds, playback_rate, updated_by_device, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			media_identity = VALUES(media_identity),
			position_seconds = VALUES(position_seconds),
			duration_seconds = VALUES(duration_seconds),
			playback_rate = VALUES(playback_rate),
			updated_by_device = VALUES(updated_by_device),
			updated_at = VALUES(updated_at)`,
		row.OwnerID, row.MediaKind, row.MediaIdentityHash, row.MediaIdentity,
		row.PositionSeconds, row.DurationSeconds, row.PlaybackRate, row.UpdatedByDevice, row.UpdatedAt,
	)
	return err
}

func execMediaRecent(ctx context.Context, ex sqlExecer, row computerevent.MediaRecentProjection) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO media_recents (
			owner_id, media_kind, media_identity_hash, media_identity,
			title, file_name, file_path, source_url, media_type, content_id, opened_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			media_identity = VALUES(media_identity),
			title = VALUES(title),
			file_name = VALUES(file_name),
			file_path = VALUES(file_path),
			source_url = VALUES(source_url),
			media_type = VALUES(media_type),
			content_id = VALUES(content_id),
			opened_at = VALUES(opened_at)`,
		row.OwnerID, row.MediaKind, row.MediaIdentityHash, row.MediaIdentity,
		row.Title, row.FileName, row.FilePath, row.SourceURL, row.MediaType, row.ContentID, row.OpenedAt,
	)
	return err
}

func execUserPreference(ctx context.Context, ex sqlExecer, row computerevent.UserPreferenceProjection) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO user_preferences (owner_id, preference_key, value_json, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   value_json = VALUES(value_json),
		   updated_at = VALUES(updated_at)`,
		row.OwnerID, row.PreferenceKey, row.ValueJSON, row.UpdatedAt,
	)
	return err
}

func projectMediaProgress(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var row computerevent.MediaProgressProjection
	if err := json.Unmarshal(body, &row); err != nil {
		return fmt.Errorf("decode media progress: %w", err)
	}
	if strings.TrimSpace(row.OwnerID) == "" || strings.TrimSpace(row.MediaKind) == "" || strings.TrimSpace(row.MediaIdentityHash) == "" || strings.TrimSpace(row.UpdatedAt) == "" {
		return fmt.Errorf("media progress identity (owner_id, media_kind, media_identity_hash, updated_at) is required")
	}
	if err := execMediaProgress(ctx, tx, row); err != nil {
		return fmt.Errorf("project media progress: %w", err)
	}
	return nil
}

func projectMediaRecent(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var row computerevent.MediaRecentProjection
	if err := json.Unmarshal(body, &row); err != nil {
		return fmt.Errorf("decode media recent: %w", err)
	}
	if strings.TrimSpace(row.OwnerID) == "" || strings.TrimSpace(row.MediaKind) == "" || strings.TrimSpace(row.MediaIdentityHash) == "" || strings.TrimSpace(row.OpenedAt) == "" {
		return fmt.Errorf("media recent identity (owner_id, media_kind, media_identity_hash, opened_at) is required")
	}
	if err := execMediaRecent(ctx, tx, row); err != nil {
		return fmt.Errorf("project media recent: %w", err)
	}
	return nil
}

func projectUserPreference(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var row computerevent.UserPreferenceProjection
	if err := json.Unmarshal(body, &row); err != nil {
		return fmt.Errorf("decode user preference: %w", err)
	}
	if strings.TrimSpace(row.OwnerID) == "" || strings.TrimSpace(row.PreferenceKey) == "" || strings.TrimSpace(row.UpdatedAt) == "" {
		return fmt.Errorf("user preference identity (owner_id, preference_key, updated_at) is required")
	}
	if strings.TrimSpace(row.ValueJSON) == "" {
		row.ValueJSON = "{}"
	}
	if err := execUserPreference(ctx, tx, row); err != nil {
		return fmt.Errorf("project user preference: %w", err)
	}
	return nil
}

// writeMediaState is the one write path of the family. Unbound stores keep
// direct SQL (the test seam, as for desktops); a bound store appends the op,
// seeding first if this tape has not yet recorded the family's rows.
func (s *Store) writeMediaState(ctx context.Context, kind string, row any, direct func(sqlExecer) error) error {
	tape := s.projectionTape
	if tape == nil {
		return direct(s.db)
	}
	body, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("store: marshal %s: %w", kind, err)
	}
	s.mediaStateMu.Lock()
	defer s.mediaStateMu.Unlock()
	var ops []computerevent.ProjectionOp
	seeding := s.mediaStateSeededTape != tape
	if seeding {
		seed, _, err := s.snapshotMediaStateOps(ctx, "")
		if err != nil {
			return err
		}
		ops = append(ops, seed...)
	}
	ops = append(ops, computerevent.ProjectionOp{Kind: kind, Body: body})
	if err := tape.appendOps(ctx, ops); err != nil {
		return err
	}
	if seeding {
		s.mediaStateSeededTape = tape
	}
	return nil
}

type mediaStateCounts struct {
	progress    int
	recents     int
	preferences int
}

// mediaStateTime returns a stored DATETIME as the RFC 3339 UTC text the
// writers bind, so a seeded row writes back the value it holds.
func mediaStateTime(raw string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano)
	}
	return raw
}

// snapshotMediaStateOps returns an op for every row of the three tables, in
// primary-key order; ownerID, when set, scopes it to one owner (the residue
// import).
func (s *Store) snapshotMediaStateOps(ctx context.Context, ownerID string) ([]computerevent.ProjectionOp, mediaStateCounts, error) {
	var counts mediaStateCounts
	var ops []computerevent.ProjectionOp
	ownerID = strings.TrimSpace(ownerID)
	where := ""
	var args []any
	if ownerID != "" {
		where = ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	appendOp := func(kind string, row any) error {
		body, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("store: marshal media-state seed %s: %w", kind, err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: kind, Body: body})
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT owner_id, media_kind, media_identity_hash, media_identity,
		position_seconds, duration_seconds, playback_rate, updated_by_device, updated_at
		FROM media_progress`+where+` ORDER BY owner_id, media_kind, media_identity_hash`, args...)
	if err != nil {
		return nil, counts, fmt.Errorf("store: list media progress seed: %w", err)
	}
	for rows.Next() {
		var row computerevent.MediaProgressProjection
		if err := rows.Scan(&row.OwnerID, &row.MediaKind, &row.MediaIdentityHash, &row.MediaIdentity,
			&row.PositionSeconds, &row.DurationSeconds, &row.PlaybackRate, &row.UpdatedByDevice, &row.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan media progress seed: %w", err)
		}
		row.UpdatedAt = mediaStateTime(row.UpdatedAt)
		if err := appendOp(computerevent.ProjectionOpMediaProgress, row); err != nil {
			_ = rows.Close()
			return nil, counts, err
		}
		counts.progress++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate media progress seed: %w", err)
	}
	_ = rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT owner_id, media_kind, media_identity_hash, media_identity,
		title, file_name, file_path, source_url, media_type, content_id, opened_at
		FROM media_recents`+where+` ORDER BY owner_id, media_kind, media_identity_hash`, args...)
	if err != nil {
		return nil, counts, fmt.Errorf("store: list media recents seed: %w", err)
	}
	for rows.Next() {
		var row computerevent.MediaRecentProjection
		if err := rows.Scan(&row.OwnerID, &row.MediaKind, &row.MediaIdentityHash, &row.MediaIdentity,
			&row.Title, &row.FileName, &row.FilePath, &row.SourceURL, &row.MediaType, &row.ContentID, &row.OpenedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan media recents seed: %w", err)
		}
		row.OpenedAt = mediaStateTime(row.OpenedAt)
		if err := appendOp(computerevent.ProjectionOpMediaRecent, row); err != nil {
			_ = rows.Close()
			return nil, counts, err
		}
		counts.recents++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate media recents seed: %w", err)
	}
	_ = rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT owner_id, preference_key, value_json, updated_at
		FROM user_preferences`+where+` ORDER BY owner_id, preference_key`, args...)
	if err != nil {
		return nil, counts, fmt.Errorf("store: list user preferences seed: %w", err)
	}
	for rows.Next() {
		var row computerevent.UserPreferenceProjection
		if err := rows.Scan(&row.OwnerID, &row.PreferenceKey, &row.ValueJSON, &row.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan user preferences seed: %w", err)
		}
		row.UpdatedAt = mediaStateTime(row.UpdatedAt)
		if err := appendOp(computerevent.ProjectionOpUserPreference, row); err != nil {
			_ = rows.Close()
			return nil, counts, err
		}
		counts.preferences++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate user preferences seed: %w", err)
	}
	_ = rows.Close()
	return ops, counts, nil
}
