package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func mediaIdentityHash(identity string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(identity)))
	return hex.EncodeToString(sum[:])
}

// UpsertMediaProgress stores cross-device progress for a media source.
func (s *Store) UpsertMediaProgress(ctx context.Context, rec types.MediaProgress) (types.MediaProgress, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	rec.Kind = strings.TrimSpace(rec.Kind)
	rec.Identity = strings.TrimSpace(rec.Identity)
	if rec.OwnerID == "" || rec.Kind == "" || rec.Identity == "" {
		return types.MediaProgress{}, fmt.Errorf("owner_id, kind, and identity are required")
	}
	if rec.PlaybackRate <= 0 {
		rec.PlaybackRate = 1
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	row := computerevent.MediaProgressProjection{
		OwnerID:           rec.OwnerID,
		MediaKind:         rec.Kind,
		MediaIdentityHash: mediaIdentityHash(rec.Identity),
		MediaIdentity:     rec.Identity,
		PositionSeconds:   rec.CurrentTime,
		DurationSeconds:   rec.Duration,
		PlaybackRate:      rec.PlaybackRate,
		UpdatedByDevice:   strings.TrimSpace(rec.UpdatedByDevice),
		UpdatedAt:         rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	err := s.writeMediaState(ctx, computerevent.ProjectionOpMediaProgress, row, func(ex sqlExecer) error {
		return execMediaProgress(ctx, ex, row)
	}, nil)
	if err != nil {
		return types.MediaProgress{}, fmt.Errorf("upsert media progress: %w", err)
	}
	return rec, nil
}

// GetMediaProgress returns progress for a media source.
func (s *Store) GetMediaProgress(ctx context.Context, ownerID, kind, identity string) (types.MediaProgress, error) {
	ownerID = strings.TrimSpace(ownerID)
	kind = strings.TrimSpace(kind)
	identity = strings.TrimSpace(identity)
	if ownerID == "" || kind == "" || identity == "" {
		return types.MediaProgress{}, fmt.Errorf("owner_id, kind, and identity are required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT owner_id, media_kind, media_identity, position_seconds, duration_seconds, playback_rate, updated_by_device, updated_at
		   FROM media_progress
		  WHERE owner_id = ? AND media_kind = ? AND media_identity_hash = ?`,
		ownerID,
		kind,
		mediaIdentityHash(identity),
	)
	return scanMediaProgress(row)
}

// UpsertMediaRecent records a recently opened media source.
func (s *Store) UpsertMediaRecent(ctx context.Context, rec types.MediaRecent) (types.MediaRecent, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	rec.Kind = strings.TrimSpace(rec.Kind)
	rec.Identity = strings.TrimSpace(rec.Identity)
	if rec.OwnerID == "" || rec.Kind == "" || rec.Identity == "" {
		return types.MediaRecent{}, fmt.Errorf("owner_id, kind, and identity are required")
	}
	if rec.OpenedAt.IsZero() {
		rec.OpenedAt = time.Now().UTC()
	}
	row := computerevent.MediaRecentProjection{
		OwnerID:           rec.OwnerID,
		MediaKind:         rec.Kind,
		MediaIdentityHash: mediaIdentityHash(rec.Identity),
		MediaIdentity:     rec.Identity,
		Title:             rec.Title,
		FileName:          rec.FileName,
		FilePath:          rec.FilePath,
		SourceURL:         rec.SourceURL,
		MediaType:         rec.MediaType,
		ContentID:         rec.ContentID,
		OpenedAt:          rec.OpenedAt.UTC().Format(time.RFC3339Nano),
	}
	err := s.writeMediaState(ctx, computerevent.ProjectionOpMediaRecent, row, func(ex sqlExecer) error {
		return execMediaRecent(ctx, ex, row)
	}, func() ([]computerevent.ProjectionOp, error) {
		return s.pruneMediaRecentsOps(ctx, row)
	})
	if err != nil {
		return types.MediaRecent{}, fmt.Errorf("upsert media recent: %w", err)
	}
	return rec, nil
}

// ListMediaRecents returns recent media sources for an owner and optional kind.
func (s *Store) ListMediaRecents(ctx context.Context, ownerID, kind string, limit int) ([]types.MediaRecent, error) {
	ownerID = strings.TrimSpace(ownerID)
	kind = strings.TrimSpace(kind)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT owner_id, media_kind, media_identity, title, file_name, file_path, source_url, media_type, content_id, opened_at
	            FROM media_recents
	           WHERE owner_id = ?`
	args := []any{ownerID}
	if kind != "" {
		query += ` AND media_kind = ?`
		args = append(args, kind)
	}
	query += ` ORDER BY opened_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query media recents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []types.MediaRecent
	for rows.Next() {
		rec, err := scanMediaRecent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media recents: %w", err)
	}
	return out, nil
}

// SaveUserPreference stores an owner-scoped preference value.
func (s *Store) SaveUserPreference(ctx context.Context, rec types.UserPreference) (types.UserPreference, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	rec.PreferenceKey = strings.TrimSpace(rec.PreferenceKey)
	if rec.OwnerID == "" || rec.PreferenceKey == "" {
		return types.UserPreference{}, fmt.Errorf("owner_id and preference_key are required")
	}
	if rec.Value == nil {
		rec.Value = map[string]any{}
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(rec.Value)
	if err != nil {
		return types.UserPreference{}, fmt.Errorf("marshal preference value: %w", err)
	}
	row := computerevent.UserPreferenceProjection{
		OwnerID:       rec.OwnerID,
		PreferenceKey: rec.PreferenceKey,
		ValueJSON:     string(raw),
		UpdatedAt:     rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	err = s.writeMediaState(ctx, computerevent.ProjectionOpUserPreference, row, func(ex sqlExecer) error {
		return execUserPreference(ctx, ex, row)
	}, nil)
	if err != nil {
		return types.UserPreference{}, fmt.Errorf("save user preference: %w", err)
	}
	return rec, nil
}

// GetUserPreference returns a stored preference.
func (s *Store) GetUserPreference(ctx context.Context, ownerID, key string) (types.UserPreference, error) {
	ownerID = strings.TrimSpace(ownerID)
	key = strings.TrimSpace(key)
	if ownerID == "" || key == "" {
		return types.UserPreference{}, fmt.Errorf("owner_id and preference_key are required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT owner_id, preference_key, value_json, updated_at
		   FROM user_preferences
		  WHERE owner_id = ? AND preference_key = ?`,
		ownerID,
		key,
	)
	return scanUserPreference(row)
}

func scanMediaProgress(row interface{ Scan(...any) error }) (types.MediaProgress, error) {
	var rec types.MediaProgress
	var updatedAt string
	err := row.Scan(&rec.OwnerID, &rec.Kind, &rec.Identity, &rec.CurrentTime, &rec.Duration, &rec.PlaybackRate, &rec.UpdatedByDevice, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.MediaProgress{}, ErrNotFound
		}
		return types.MediaProgress{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.MediaProgress{}, fmt.Errorf("parse media progress updated_at: %w", err)
	}
	rec.UpdatedAt = parsed
	return rec, nil
}

func scanMediaRecent(row interface{ Scan(...any) error }) (types.MediaRecent, error) {
	var rec types.MediaRecent
	var openedAt string
	if err := row.Scan(&rec.OwnerID, &rec.Kind, &rec.Identity, &rec.Title, &rec.FileName, &rec.FilePath, &rec.SourceURL, &rec.MediaType, &rec.ContentID, &openedAt); err != nil {
		return types.MediaRecent{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, openedAt)
	if err != nil {
		return types.MediaRecent{}, fmt.Errorf("parse media recent opened_at: %w", err)
	}
	rec.OpenedAt = parsed
	return rec, nil
}

func scanUserPreference(row interface{ Scan(...any) error }) (types.UserPreference, error) {
	var rec types.UserPreference
	var raw, updatedAt string
	if err := row.Scan(&rec.OwnerID, &rec.PreferenceKey, &raw, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.UserPreference{}, ErrNotFound
		}
		return types.UserPreference{}, err
	}
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return types.UserPreference{}, fmt.Errorf("unmarshal preference value: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.UserPreference{}, fmt.Errorf("parse preference updated_at: %w", err)
	}
	rec.Value = value
	rec.UpdatedAt = parsed
	return rec, nil
}
