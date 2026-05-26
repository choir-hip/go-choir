package searchplane

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// FileHealthStore persists provider health in SQLite.
type FileHealthStore struct {
	db     *sql.DB
	policy BackoffPolicy
}

// OpenFileHealthStore opens or creates a SQLite health database at path.
func OpenFileHealthStore(path string) (*FileHealthStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("search health path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create health db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open health db: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &FileHealthStore{db: db, policy: BackoffPolicyFromEnv()}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *FileHealthStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS provider_health (
  provider TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  cooldown_until TEXT,
  strike_count INTEGER NOT NULL DEFAULT 0,
  window_attempts INTEGER NOT NULL DEFAULT 0,
  window_successes INTEGER NOT NULL DEFAULT 0,
  window_results_total INTEGER NOT NULL DEFAULT 0,
  last_failure_class TEXT,
  last_error_summary TEXT,
  updated_at TEXT NOT NULL
)`)
	return err
}

func (s *FileHealthStore) Snapshot() (map[string]ProviderHealth, error) {
	rows, err := s.db.Query(`SELECT provider, state, cooldown_until, strike_count, window_attempts, window_successes, window_results_total, last_failure_class, last_error_summary, updated_at FROM provider_health`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ProviderHealth{}
	for rows.Next() {
		rec, err := scanHealthRow(rows)
		if err != nil {
			return nil, err
		}
		out[rec.Provider] = normalizeExpiredLocked(rec, time.Now())
	}
	return out, rows.Err()
}

func (s *FileHealthStore) Get(provider string) (ProviderHealth, error) {
	row := s.db.QueryRow(`SELECT provider, state, cooldown_until, strike_count, window_attempts, window_successes, window_results_total, last_failure_class, last_error_summary, updated_at FROM provider_health WHERE provider = ?`, provider)
	rec, err := scanHealthRow(row)
	if err == sql.ErrNoRows {
		return ProviderHealth{Provider: provider, State: StateActive, UpdatedAt: time.Now()}, nil
	}
	if err != nil {
		return ProviderHealth{}, err
	}
	return normalizeExpiredLocked(rec, time.Now()), nil
}

func (s *FileHealthStore) RecordOutcome(outcome Outcome) (ProviderHealth, error) {
	rec, err := s.Get(outcome.Provider)
	if err != nil {
		return ProviderHealth{}, err
	}
	now := outcome.At
	if now.IsZero() {
		now = time.Now()
	}
	rec.UpdatedAt = now
	rec.WindowAttempts++
	if outcome.Class == OutcomeSuccess {
		rec.WindowSuccesses++
		rec.WindowResultsTotal += outcome.Results
	}
	applyOutcome(&rec, outcome, s.policy, now)
	if err := s.upsert(rec); err != nil {
		return ProviderHealth{}, err
	}
	return rec, nil
}

func (s *FileHealthStore) ResetProvider(provider string) (ProviderHealth, error) {
	rec := ProviderHealth{Provider: provider, State: StateActive, UpdatedAt: time.Now()}
	if err := s.upsert(rec); err != nil {
		return ProviderHealth{}, err
	}
	return rec, nil
}

func (s *FileHealthStore) ResetAll() error {
	_, err := s.db.Exec(`DELETE FROM provider_health`)
	return err
}

func (s *FileHealthStore) upsert(rec ProviderHealth) error {
	var cooldown string
	if rec.CooldownUntil != nil {
		cooldown = rec.CooldownUntil.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.Exec(`INSERT INTO provider_health (
  provider, state, cooldown_until, strike_count, window_attempts, window_successes, window_results_total, last_failure_class, last_error_summary, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(provider) DO UPDATE SET
  state=excluded.state,
  cooldown_until=excluded.cooldown_until,
  strike_count=excluded.strike_count,
  window_attempts=excluded.window_attempts,
  window_successes=excluded.window_successes,
  window_results_total=excluded.window_results_total,
  last_failure_class=excluded.last_failure_class,
  last_error_summary=excluded.last_error_summary,
  updated_at=excluded.updated_at`,
		rec.Provider, string(rec.State), nullIfEmpty(cooldown), rec.StrikeCount, rec.WindowAttempts, rec.WindowSuccesses, rec.WindowResultsTotal,
		nullIfEmpty(rec.LastFailureClass), nullIfEmpty(rec.LastErrorSummary), rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

type healthRowScanner interface {
	Scan(dest ...any) error
}

func scanHealthRow(row healthRowScanner) (ProviderHealth, error) {
	var provider, state string
	var cooldown sql.NullString
	var strikes, attempts, successes, results int
	var lastClass, lastErr sql.NullString
	var updated string
	if err := row.Scan(&provider, &state, &cooldown, &strikes, &attempts, &successes, &results, &lastClass, &lastErr, &updated); err != nil {
		return ProviderHealth{}, err
	}
	rec := ProviderHealth{
		Provider:           provider,
		State:              ProviderState(state),
		StrikeCount:        strikes,
		WindowAttempts:     attempts,
		WindowSuccesses:    successes,
		WindowResultsTotal: results,
	}
	if cooldown.Valid && cooldown.String != "" {
		t, err := time.Parse(time.RFC3339Nano, cooldown.String)
		if err == nil {
			rec.CooldownUntil = &t
		}
	}
	if lastClass.Valid {
		rec.LastFailureClass = lastClass.String
	}
	if lastErr.Valid {
		rec.LastErrorSummary = lastErr.String
	}
	if t, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		rec.UpdatedAt = t
	}
	return rec, nil
}

// DefaultHealthStorePath returns the configured SQLite path or the durable gateway default.
func DefaultHealthStorePath() string {
	if p := strings.TrimSpace(os.Getenv("CHOIR_SEARCH_HEALTH_PATH")); p != "" {
		return p
	}
	return "/var/lib/go-choir/gateway/search-health.db"
}

// OpenDefaultHealthStore opens SQLite unless CHOIR_SEARCH_HEALTH_MEMORY=1.
func OpenDefaultHealthStore() (HealthStore, error) {
	if strings.TrimSpace(os.Getenv("CHOIR_SEARCH_HEALTH_MEMORY")) == "1" {
		return NewMemoryHealthStore(), nil
	}
	return OpenFileHealthStore(DefaultHealthStorePath())
}
