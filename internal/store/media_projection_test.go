package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Review findings on the media-state projection (bunch 188,
// docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md).

func boundMediaStore(t *testing.T, computerID string) *Store {
	t.Helper()
	productStore := openProjectStore(t)
	prepareGenesis(t, productStore, computerID, "genesis-"+computerID)
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	return productStore
}

func insertUntapedRecent(t *testing.T, s *Store, identity string) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO media_recents (owner_id, media_kind, media_identity_hash, media_identity, opened_at)
		VALUES ('owner-media', 'pdf', ?, ?, ?)`, mediaIdentityHash(identity), identity,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

func saveTheme(t *testing.T, s *Store, mode string) {
	t.Helper()
	if _, err := s.SaveUserPreference(context.Background(), types.UserPreference{
		OwnerID: "owner-media", PreferenceKey: "theme", Value: map[string]any{"mode": mode},
	}); err != nil {
		t.Fatal(err)
	}
}

// Reopen adopts another workspace (a restore or a base install), which may
// hold media rows that are not on this tape. The seed must be derived again
// for it: the first write after Reopen records the adopted rows.
func TestReopenReseedsMediaStateForTheAdoptedWorkspace(t *testing.T) {
	s := boundMediaStore(t, "computer-media-reopen")
	saveTheme(t, s, "dark") // seeds (empty) and writes

	path := s.path
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	insertUntapedRecent(t, s, "file:/adopted.pdf")

	before := s.telemetryStats.tapeOps.Load()
	saveTheme(t, s, "light")
	if got := s.telemetryStats.tapeOps.Load() - before; got < 3 {
		t.Fatalf("the first write after Reopen appended %d ops; want the adopted workspace's rows (recent and theme) seeded before the write", got)
	}
}

// An unscoped residue import records every row of the family, so it
// counts as the seed: the next write is a single op.
func TestUnscopedResidueImportCountsAsTheMediaStateSeed(t *testing.T) {
	s := boundMediaStore(t, "computer-media-import-seed")
	insertUntapedRecent(t, s, "file:/old.pdf")
	if _, err := s.ImportResidueSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := s.telemetryStats.tapeOps.Load()
	saveTheme(t, s, "dark")
	if got := s.telemetryStats.tapeOps.Load() - before; got != 1 {
		t.Fatalf("the write after an unscoped residue import appended %d ops, want 1 (no second seed)", got)
	}
}

// A media-state op larger than the per-op cap is refused before anything is
// pinned: no tape event, and the row does not exist.
func TestOversizedMediaStateWriteIsRefusedBeforeTheTape(t *testing.T) {
	ctx := context.Background()
	s := boundMediaStore(t, "computer-media-oversized")
	saveTheme(t, s, "dark") // the seed, so the next write is the oversized op alone
	head, err := s.Head(ctx, "computer-media-oversized")
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	_, err = s.UpsertMediaRecent(ctx, types.MediaRecent{
		OwnerID: "owner-media", Kind: "pdf", Identity: "file:/huge.pdf",
		Title: strings.Repeat("x", 8<<20+1), // over the 8 MiB per-op cap
	})
	if err == nil {
		t.Fatal("an op over the per-op cap was accepted")
	}
	after, err := s.Head(ctx, "computer-media-oversized")
	if err != nil || after == nil || after.Sequence != head.Sequence {
		t.Fatalf("the refused write moved the tape: %#v -> %#v (%v)", head, after, err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM media_recents`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("media_recents rows = %d (%v), want 0", n, err)
	}
}
