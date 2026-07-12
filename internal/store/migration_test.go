package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestOpenDoesNotReplayLegacyRelationalRows(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open private store: %v", err)
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO agents (agent_id, owner_id, sandbox_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, "legacy-agent", "owner-test", "sandbox-test", now, now); err != nil {
		_ = s.Close()
		t.Fatalf("seed retired relational row: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close private store: %v", err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen private store: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := s.db.PingContext(ctx); err != nil {
		t.Fatalf("private store health: %v", err)
	}
	if _, err := s.ogGetByKey(ctx, ogKindAgent, "agent_id", "legacy-agent"); !errors.Is(err, objectgraph.ErrNotFound) {
		t.Fatalf("canonical object graph lookup error = %v, want %v", err, objectgraph.ErrNotFound)
	}
}
