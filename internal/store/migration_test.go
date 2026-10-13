package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestOpenDropsAndDoesNotReplayLegacyRelationalRows(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open private store: %v", err)
	}
	now := time.Now().UTC()
	// A pre-retirement computer still carries the legacy agents table.
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE agents (
			agent_id    VARCHAR(255) PRIMARY KEY,
			owner_id    VARCHAR(255) NOT NULL,
			computer_id VARCHAR(255) NOT NULL,
			created_at  DATETIME NOT NULL,
			updated_at  DATETIME NOT NULL
		)`); err != nil {
		_ = s.Close()
		t.Fatalf("create retired relational table: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO agents (agent_id, owner_id, computer_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, "legacy-agent", "owner-test", "autoputer-test", now, now); err != nil {
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
	if testDoltTableExists(t, s, "agents") {
		t.Fatal("retired agents table survived bootstrap")
	}
	if _, err := s.ogGetByKey(ctx, ogKindAgent, "agent_id", "legacy-agent"); !errors.Is(err, objectgraph.ErrNotFound) {
		t.Fatalf("canonical object graph lookup error = %v, want %v", err, objectgraph.ErrNotFound)
	}
}
