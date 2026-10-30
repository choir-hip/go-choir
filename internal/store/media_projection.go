package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
// first tape-bound write of the family in a process (and the first after a
// Reopen) therefore seeds: an op for every existing row of the three tables,
// in bounded batches, then its own op, so the write still wins for its key.
// Each seed batch is an idempotent upsert set; a crash part-way leaves the
// flag unset and the next process seeds again. mediaStateMu is held across
// every write's snapshot and appends, so no write lands between a seed's read
// and its append. media_recents keeps the newest mediaRecentsPerKind rows per
// (owner, media_kind); the prune is a delete op on the tape.

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execMediaProgress(ctx context.Context, ex sqlExecer, row computerevent.MediaProgressProjection) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO media_progress (
			owner_id, media_kind, media_identity_hash, media_identity,
			position_seconds, duration_seconds, playback_rate, updated_by_device, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (owner_id, media_kind, media_identity_hash) DO UPDATE SET
			media_identity = excluded.media_identity,
			position_seconds = excluded.position_seconds,
			duration_seconds = excluded.duration_seconds,
			playback_rate = excluded.playback_rate,
			updated_by_device = excluded.updated_by_device,
			updated_at = excluded.updated_at`,
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
		ON CONFLICT (owner_id, media_kind, media_identity_hash) DO UPDATE SET
			media_identity = excluded.media_identity,
			title = excluded.title,
			file_name = excluded.file_name,
			file_path = excluded.file_path,
			source_url = excluded.source_url,
			media_type = excluded.media_type,
			content_id = excluded.content_id,
			opened_at = excluded.opened_at`,
		row.OwnerID, row.MediaKind, row.MediaIdentityHash, row.MediaIdentity,
		row.Title, row.FileName, row.FilePath, row.SourceURL, row.MediaType, row.ContentID, row.OpenedAt,
	)
	return err
}

func execUserPreference(ctx context.Context, ex sqlExecer, row computerevent.UserPreferenceProjection) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO user_preferences (owner_id, preference_key, value_json, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (owner_id, preference_key) DO UPDATE SET
		   value_json = excluded.value_json,
		   updated_at = excluded.updated_at`,
		row.OwnerID, row.PreferenceKey, row.ValueJSON, row.UpdatedAt,
	)
	return err
}

func projectMediaProgress(ctx context.Context, tx sqlExecer, body json.RawMessage) error {
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

func projectMediaRecent(ctx context.Context, tx sqlExecer, body json.RawMessage) error {
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

func projectUserPreference(ctx context.Context, tx sqlExecer, body json.RawMessage) error {
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

const (
	// mediaStateSeedBatchBytes is the op-body budget of one seed batch, far
	// below the tape's record ceiling (tapestore.MaxRecordBytes, 64 MiB).
	mediaStateSeedBatchBytes = 1 << 20
	// mediaStateMaxOpBytes is the largest single media-state op. A bigger
	// write is refused before anything is pinned, so no row can grow past
	// what one batch can carry.
	mediaStateMaxOpBytes = 8 << 20
	// mediaRecentsPerKind is the most media_recents rows kept per (owner,
	// media_kind): ListMediaRecents never returns more than 100.
	mediaRecentsPerKind = 100
)

// ErrMediaStateOpTooLarge is a media-state row too large for one tape op.
var ErrMediaStateOpTooLarge = errors.New("store: media-state op exceeds the per-op cap")

// mediaStateOp builds one op and preflights its size.
func mediaStateOp(kind string, row any) (computerevent.ProjectionOp, error) {
	body, err := json.Marshal(row)
	if err != nil {
		return computerevent.ProjectionOp{}, fmt.Errorf("store: marshal %s: %w", kind, err)
	}
	if len(body) > mediaStateMaxOpBytes {
		return computerevent.ProjectionOp{}, fmt.Errorf("%w: %s is %d bytes (cap %d)", ErrMediaStateOpTooLarge, kind, len(body), mediaStateMaxOpBytes)
	}
	return computerevent.ProjectionOp{Kind: kind, Body: body}, nil
}

// chunkMediaStateOps splits ops into batches whose bodies total at most the
// budget; an op is never split, so a batch holds at least one op.
func chunkMediaStateOps(ops []computerevent.ProjectionOp, budget int) [][]computerevent.ProjectionOp {
	var batches [][]computerevent.ProjectionOp
	var current []computerevent.ProjectionOp
	size := 0
	for _, op := range ops {
		n := len(op.Body) + len(op.Kind)
		if len(current) > 0 && size+n > budget {
			batches = append(batches, current)
			current, size = nil, 0
		}
		current = append(current, op)
		size += n
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// writeMediaState is the one write path of the family. Unbound stores keep
// direct SQL (the test seam, as for desktops); a bound store appends the op,
// seeding first if this tape has not yet recorded the family's rows. The seed
// goes out in bounded batches, each an idempotent upsert set; the write and
// its follow ops (a recents prune) ride in the last one. A crash between
// batches leaves the seed flag unset, so the next process seeds again.
func (s *Store) writeMediaState(ctx context.Context, kind string, row any, direct func(sqlExecer) error, follow func() ([]computerevent.ProjectionOp, error)) error {
	op, err := mediaStateOp(kind, row)
	if err != nil {
		return err
	}
	s.mediaStateMu.Lock()
	defer s.mediaStateMu.Unlock()
	var followOps []computerevent.ProjectionOp
	if follow != nil {
		if followOps, err = follow(); err != nil {
			return err
		}
	}
	tape := s.projectionTape
	if tape == nil {
		if err := direct(s.db); err != nil {
			return err
		}
		for _, f := range followOps {
			if err := applyMediaStateOpDirect(ctx, s.db, f); err != nil {
				return err
			}
		}
		return nil
	}
	final := append([]computerevent.ProjectionOp{op}, followOps...)
	seeding := s.mediaStateSeededTape != tape
	var batches [][]computerevent.ProjectionOp
	if seeding {
		seed, _, err := s.snapshotMediaStateOps(ctx, "")
		if err != nil {
			return err
		}
		batches = chunkMediaStateOps(seed, mediaStateSeedBatchBytes)
	}
	if n := len(batches); n > 0 && batchBodyBytes(batches[n-1])+batchBodyBytes(final) <= mediaStateSeedBatchBytes {
		batches[n-1] = append(batches[n-1], final...)
	} else {
		batches = append(batches, final)
	}
	for _, batch := range batches {
		if err := tape.appendOps(ctx, batch); err != nil {
			return err
		}
	}
	if seeding {
		s.mediaStateSeededTape = tape
	}
	return nil
}

func batchBodyBytes(ops []computerevent.ProjectionOp) int {
	n := 0
	for _, op := range ops {
		n += len(op.Body) + len(op.Kind)
	}
	return n
}

// applyMediaStateOpDirect is the unbound seam for a follow op.
func applyMediaStateOpDirect(ctx context.Context, ex sqlExecer, op computerevent.ProjectionOp) error {
	switch op.Kind {
	case computerevent.ProjectionOpMediaRecentDelete:
		return projectMediaRecentDelete(ctx, ex, op.Body)
	default:
		return fmt.Errorf("store: media-state follow op %q has no direct seam", op.Kind)
	}
}

func projectMediaRecentDelete(ctx context.Context, ex sqlExecer, body json.RawMessage) error {
	var key computerevent.MediaRecentDeleteProjection
	if err := json.Unmarshal(body, &key); err != nil {
		return fmt.Errorf("decode media recent delete: %w", err)
	}
	if strings.TrimSpace(key.OwnerID) == "" || strings.TrimSpace(key.MediaKind) == "" || strings.TrimSpace(key.MediaIdentityHash) == "" {
		return fmt.Errorf("media recent delete identity (owner_id, media_kind, media_identity_hash) is required")
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM media_recents WHERE owner_id = ? AND media_kind = ? AND media_identity_hash = ?`,
		key.OwnerID, key.MediaKind, key.MediaIdentityHash); err != nil {
		return fmt.Errorf("project media recent delete: %w", err)
	}
	return nil
}

// pruneMediaRecentsOps returns a delete op for every recent of the written
// row's (owner, media_kind) past the newest mediaRecentsPerKind, counting the
// written row. Order: opened_at newest first, then the key, so the choice is
// deterministic. Called under mediaStateMu, before the write is applied.
func (s *Store) pruneMediaRecentsOps(ctx context.Context, written computerevent.MediaRecentProjection) ([]computerevent.ProjectionOp, error) {
	type recent struct {
		hash   string
		opened time.Time
	}
	parse := func(raw string) time.Time {
		t, _ := time.Parse(time.RFC3339Nano, raw)
		return t.UTC()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT media_identity_hash, opened_at FROM media_recents
		WHERE owner_id = ? AND media_kind = ? AND media_identity_hash <> ?`,
		written.OwnerID, written.MediaKind, written.MediaIdentityHash)
	if err != nil {
		return nil, fmt.Errorf("store: list media recents for prune: %w", err)
	}
	all := []recent{{hash: written.MediaIdentityHash, opened: parse(written.OpenedAt)}}
	for rows.Next() {
		var hash, opened string
		if err := rows.Scan(&hash, &opened); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("store: scan media recents for prune: %w", err)
		}
		all = append(all, recent{hash: hash, opened: parse(opened)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("store: iterate media recents for prune: %w", err)
	}
	_ = rows.Close()
	if len(all) <= mediaRecentsPerKind {
		return nil, nil
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].opened.Equal(all[j].opened) {
			return all[i].opened.After(all[j].opened)
		}
		return all[i].hash < all[j].hash
	})
	var ops []computerevent.ProjectionOp
	for _, r := range all[mediaRecentsPerKind:] {
		op, err := mediaStateOp(computerevent.ProjectionOpMediaRecentDelete, computerevent.MediaRecentDeleteProjection{
			OwnerID: written.OwnerID, MediaKind: written.MediaKind, MediaIdentityHash: r.hash,
		})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
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
		op, err := mediaStateOp(kind, row)
		if err != nil {
			return fmt.Errorf("store: media-state seed: %w", err)
		}
		ops = append(ops, op)
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
