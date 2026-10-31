package store

// Compaction that reaches the paper (docs/design/paper-compaction-2026-10-31.md).
// Ways it can fail, written before the code:
//
//   - P1: a grown file with no freelist is never copied, so every index stays
//     spread across it and no snapshot chunk survives an hour; or a small
//     file, or one compacted a day ago, is copied again and again;
//   - P2: the staged copy ages past the replay bound and the boot drops it,
//     so compaction never lands; or the bound is a bare event count that no
//     longer means a time;
//   - P3: a fresh staged copy is rewritten every day (21 GB of IO for
//     nothing), or a stale one is kept and then dropped at the boot;
//   - P4: the copy fills the disk: a restage holds the old copy, the
//     original and the new one at once;
//   - P5: the copy starts when the guest has no memory to give it;
//   - P6: a copy that fails or is cut short leaves a staged file, or a
//     sidecar, or touches the original;
//   - P7: the copy's witness differs from the original's at the same head,
//     so the replay-eligibility gate refuses the computer after the rebase;
//   - P8: the layout clock restarts on a discard or a crash boot, or never
//     restarts after a rebase.

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// withCompactKnobs sets the cadence knobs for one test.
func withCompactKnobs(t *testing.T, minBytes int64, maxAge time.Duration) {
	t.Helper()
	oldMin, oldAge := compactLayoutMinBytes, compactLayoutMaxAge
	compactLayoutMinBytes, compactLayoutMaxAge = minBytes, maxAge
	t.Cleanup(func() { compactLayoutMinBytes, compactLayoutMaxAge = oldMin, oldAge })
}

func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

// P1: the layout trigger.
func TestCompactLayoutAgeTriggersACopyWithNoFreelist(t *testing.T) {
	cases := []struct {
		name     string
		minBytes int64
		marker   func(statePath string)
		want     bool
	}{
		{"never compacted", 1, func(string) {}, true},
		{"compacted long ago", 1, func(p string) { writeLayoutMarker(p, time.Now().Add(-8*24*time.Hour), 1) }, true},
		{"compacted a day ago", 1, func(p string) { writeLayoutMarker(p, time.Now().Add(-24*time.Hour), 1) }, false},
		{"file below the size floor", 1 << 40, func(string) {}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withCompactKnobs(t, tc.minBytes, 7*24*time.Hour)
			s, _ := compactTestStore(t, 5, 100, 0) // nothing deleted: no freelist
			tc.marker(s.statePath)
			done, err := s.PrepareCompactCopy(context.Background())
			if err != nil || done != tc.want {
				t.Fatalf("PrepareCompactCopy = %v, %v; want staged=%v", done, err, tc.want)
			}
			if exists(s.statePath+compactStagedSuffix) != tc.want {
				t.Fatalf("staged copy present = %v, want %v", !tc.want, tc.want)
			}
		})
	}
}

// P2: the bound is a replay time, and a tail past the old 50,000-event bound
// is replayed, not dropped.
func TestCompactRebaseBoundIsAReplayTime(t *testing.T) {
	want := uint64(compactReplayEventsPerSecond) * uint64(compactRebaseMaxReplay/time.Second)
	if compactRebaseMaxTail != want {
		t.Fatalf("compactRebaseMaxTail = %d, want rate x time = %d", compactRebaseMaxTail, want)
	}
	if compactRebaseMaxTail < 4*50000 {
		t.Fatalf("bound %d does not cover a day of the paper's events (~340,000)", compactRebaseMaxTail)
	}
	// A tail inside the bound is swapped in, however it compares with the old
	// event count.
	s, path := compactTestStore(t, 5, 10, 3000)
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	for seq := uint64(6); seq <= 60; seq++ {
		compactTestAppend(t, s, seq)
	}
	old := compactRebaseMaxTail
	compactRebaseMaxTail = 100
	defer func() { compactRebaseMaxTail = old }()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	outcome, err := RebaseOntoCompactCopy(path, true)
	if err != nil || !strings.HasPrefix(outcome, "rebased") || !strings.Contains(outcome, "55 events to replay") {
		t.Fatalf("RebaseOntoCompactCopy = %q, %v", outcome, err)
	}
}

// P3: a fresh staged copy is kept; a copy whose tail passed half the bound is
// replaced by one at the current head.
func TestCompactRestagesOnlyAStaleCopy(t *testing.T) {
	old := compactRebaseMaxTail
	compactRebaseMaxTail = 10
	defer func() { compactRebaseMaxTail = old }()
	s, _ := compactTestStore(t, 5, 10, 3000)
	ctx := context.Background()
	if done, err := s.PrepareCompactCopy(ctx); err != nil || !done {
		t.Fatalf("first copy = %v, %v", done, err)
	}
	side1, _ := readCompactSidecar(s.statePath+compactStagedSuffix, s.statePath+compactSidecarSuffix)
	compactTestAppend(t, s, 6) // tail 1 of 10: fresh
	if done, err := s.PrepareCompactCopy(ctx); err != nil || done {
		t.Fatalf("fresh copy was rewritten: %v, %v", done, err)
	}
	for seq := uint64(7); seq <= 12; seq++ {
		compactTestAppend(t, s, seq) // tail 7 of 10: past half
	}
	if done, err := s.PrepareCompactCopy(ctx); err != nil || !done {
		t.Fatalf("stale copy was kept: %v, %v", done, err)
	}
	side2, ok := readCompactSidecar(s.statePath+compactStagedSuffix, s.statePath+compactSidecarSuffix)
	if !ok || side2.Sequence != 12 || side1.Sequence != 5 {
		t.Fatalf("sidecars %+v then %+v (ok=%v), want sequence 5 then 12", side1, side2, ok)
	}
}

// P4: a restage never holds three copies. With room for the original and one
// copy but not for the old copy as well, the stale copy goes first.
func TestCompactRestageDropsTheStaleCopyBeforeRefusing(t *testing.T) {
	old := compactRebaseMaxTail
	compactRebaseMaxTail = 10
	defer func() { compactRebaseMaxTail = old }()
	s, _ := compactTestStore(t, 5, 10, 3000)
	ctx := context.Background()
	if done, err := s.PrepareCompactCopy(ctx); err != nil || !done {
		t.Fatalf("first copy = %v, %v", done, err)
	}
	for seq := uint64(6); seq <= 12; seq++ {
		compactTestAppend(t, s, seq)
	}
	oldFree := compactFreeBytes
	defer func() { compactFreeBytes = oldFree }()
	// Room for a copy only once the stale one is gone (the live size is
	// ~40 KB here; a staged copy is about as big).
	compactFreeBytes = func(string) (int64, error) {
		if exists(s.statePath + compactStagedSuffix) {
			return 1, nil
		}
		return 1 << 30, nil
	}
	done, err := s.PrepareCompactCopy(ctx)
	if err != nil || !done {
		t.Fatalf("restage with the stale copy dropped = %v, %v", done, err)
	}
	// And with no room even then, it refuses and leaves the original alone.
	for seq := uint64(13); seq <= 24; seq++ {
		compactTestAppend(t, s, seq)
	}
	compactFreeBytes = func(string) (int64, error) { return 1, nil }
	done, err = s.PrepareCompactCopy(ctx)
	if done || err == nil || !strings.Contains(err.Error(), "free") {
		t.Fatalf("PrepareCompactCopy = %v, %v; want a free-space refusal", done, err)
	}
	if exists(s.statePath+compactStagedSuffix) || exists(s.statePath+compactSidecarSuffix) || exists(s.statePath+compactStagingSuffix) {
		t.Fatal("a stale copy or staging file was left after the refusal")
	}
	if rows, seq, _ := compactTestRows(t, s.statePath); rows != 10 || seq != 24 {
		t.Fatalf("original: %d rows at %d", rows, seq)
	}
}

// P5: no memory, no copy.
func TestCompactCopyRefusesWithoutMemory(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	old := compactAvailableMemory
	compactAvailableMemory = func() (int64, bool) { return 1 << 20, true }
	defer func() { compactAvailableMemory = old }()
	done, err := s.PrepareCompactCopy(context.Background())
	if done || err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("PrepareCompactCopy = %v, %v; want a memory refusal", done, err)
	}
	if exists(s.statePath+compactStagingSuffix) || exists(s.statePath+compactStagedSuffix) {
		t.Fatal("a copy was started")
	}
	// Unknown memory (not linux) does not block.
	compactAvailableMemory = func() (int64, bool) { return 0, false }
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("with unknown memory: %v, %v", done, err)
	}
}

// P6: a copy cut short leaves the original authoritative and nothing staged.
func TestCompactCopyCutShortLeavesNothing(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	old := compactCopyBudget
	compactCopyBudget = time.Nanosecond
	defer func() { compactCopyBudget = old }()
	done, err := s.PrepareCompactCopy(context.Background())
	if done || err == nil {
		t.Fatalf("PrepareCompactCopy = %v, %v; want a failure", done, err)
	}
	for _, suffix := range []string{compactStagingSuffix, compactStagedSuffix, compactSidecarSuffix} {
		if exists(s.statePath + suffix) {
			t.Fatalf("%s left behind", suffix)
		}
	}
	if rows, seq, _ := compactTestRows(t, s.statePath); rows != 10 || seq != 5 {
		t.Fatalf("original: %d rows at %d", rows, seq)
	}
}

// P7: the copy answers the witness exactly as the original does.
func TestCompactCopyKeepsTheWitness(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	ctx := context.Background()
	live, err := computerversion.SQLiteStateExtractor{DB: s.ReadDB(), Database: "texture"}.Digests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if done, err := s.PrepareCompactCopy(ctx); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	copyDB := openRawDB(t, s.statePath+compactStagedSuffix)
	defer copyDB.Close()
	got, err := computerversion.SQLiteStateExtractor{DB: copyDB, Database: "texture"}.Digests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != live.ContentRoot || got.SchemaHash != live.SchemaHash {
		t.Fatalf("witness differs: live %s/%s, copy %s/%s", live.SchemaHash, live.ContentRoot, got.SchemaHash, got.ContentRoot)
	}
}

// P8: the layout clock restarts on a rebase and only then.
func TestCompactLayoutClockRestartsOnARebaseOnly(t *testing.T) {
	// A crash boot does not swap, so it does not restart the clock.
	s, path := compactTestStore(t, 5, 10, 3000)
	statePath := s.statePath
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := RebaseOntoCompactCopy(path, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := readLayoutMarker(statePath); ok {
		t.Fatal("a crash boot restarted the layout clock")
	}
	// A planned boot that discards the copy does not either.
	if err := os.WriteFile(statePath+compactSidecarSuffix, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RebaseOntoCompactCopy(path, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := readLayoutMarker(statePath); ok {
		t.Fatal("a discard restarted the layout clock")
	}
	// A planned boot that swaps does.
	s, path = compactTestStore(t, 5, 10, 3000)
	statePath = s.statePath
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := RebaseOntoCompactCopy(path, true); err != nil || !strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("RebaseOntoCompactCopy = %q, %v", outcome, err)
	}
	m, ok := readLayoutMarker(statePath)
	if !ok || time.Since(m.CompactedAt) > time.Minute || m.Sequence != 5 {
		t.Fatalf("layout marker %+v, ok=%v", m, ok)
	}
}
