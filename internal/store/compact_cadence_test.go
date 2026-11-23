package store

// Compaction that reaches the paper (docs/design/paper-compaction-2026-10-31.md).
// Ways it can fail, written before the code:
//
//   - P1: a file with no freelist is copied because of the age of its layout
//     (removed 2026-11-22: a store-sized copy for a gain the paper did not
//     show); or a leftover .layout.json changes what is due;
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
//   - P8: removing the layout trigger changes what the boot does with a copy
//     that was already staged by it, or a rebase writes a layout marker.

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

func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

// P1: layout age alone never makes a copy due. A file with no freelist is
// not copied however old its layout is and whatever a leftover .layout.json
// says; the freelist and forced triggers are tested in compact_test.go and
// compact_force_test.go.
func TestCompactLayoutAgeNeverMakesACopyDue(t *testing.T) {
	// A file larger than the old 1 GiB floor and older than the old 7 days.
	const pages, pageSize = 13383190, 4096
	// This is the assertion that discriminates: the removed code returned
	// "layout" here for any file over 1 GiB with no marker or a marker older
	// than 7 days (checked against the removed code on the build VM).
	for _, pages := range []int64{1 << 18 /* exactly 1 GiB */, 1<<18 + 1, pages} {
		if got := compactReason(pages, 0, pageSize, false); got != "" {
			t.Fatalf("compactReason(%d pages, 0 free) = %q; want none", pages, got)
		}
	}
	cases := map[string]func(statePath string){
		"no marker": func(string) {},
		"marker from a year ago": func(p string) {
			raw := []byte(`{"compacted_at":"2025-11-22T00:00:00Z","sequence":1}`)
			if err := os.WriteFile(p+".layout.json", raw, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"marker that is not JSON": func(p string) {
			if err := os.WriteFile(p+".layout.json", []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, leftover := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := compactTestStore(t, 5, 100, 0) // nothing deleted: no freelist
			leftover(s.statePath)
			done, err := s.PrepareCompactCopy(context.Background())
			if err != nil || done || exists(s.statePath+compactStagedSuffix) {
				t.Fatalf("PrepareCompactCopy = %v, %v; want a quiet skip", done, err)
			}
		})
	}
	// A stale marker does not stop the freelist trigger either.
	s, _ := compactTestStore(t, 5, 10, 3000)
	if err := os.WriteFile(s.statePath+".layout.json", []byte(`{"compacted_at":"2099-01-01T00:00:00Z","sequence":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy with a freelist and a stale marker = %v, %v", done, err)
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

// P8: a copy staged before the layout trigger was removed (the paper's, "due
// by layout") is handled exactly as before at a planned restart: swapped in
// when it is on the chain and its tail is short, dropped when the tail is
// past the bound, left alone by a crash boot. No layout marker is written by
// any of it, and a leftover one changes nothing.
func TestCompactRebaseOfAnAlreadyStagedCopyIsUnchanged(t *testing.T) {
	stale := []byte(`{"compacted_at":"2020-01-01T00:00:00Z","sequence":1}`)
	// A crash boot does not swap.
	s, path := compactTestStore(t, 5, 10, 3000)
	statePath := s.statePath
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := RebaseOntoCompactCopy(path, false); err != nil || outcome != "" || !exists(statePath+compactStagedSuffix) {
		t.Fatalf("crash boot = %q, %v", outcome, err)
	}
	// A planned boot that swaps, with a stale marker beside the file.
	if err := os.WriteFile(statePath+".layout.json", stale, 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := RebaseOntoCompactCopy(path, true)
	if err != nil || !strings.HasPrefix(outcome, "rebased onto the compact copy at sequence 5") {
		t.Fatalf("planned boot = %q, %v", outcome, err)
	}
	if raw, _ := os.ReadFile(statePath + ".layout.json"); string(raw) != string(stale) {
		t.Fatalf("the rebase rewrote the layout marker: %q", raw)
	}
	// A just-swapped store is not due again: the new file has no free pages.
	swapped := openRawDB(t, statePath)
	var pageCount, freelist, pageSize int64
	for q, dst := range map[string]*int64{"PRAGMA page_count": &pageCount, "PRAGMA freelist_count": &freelist, "PRAGMA page_size": &pageSize} {
		if err := swapped.QueryRow(q).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	_ = swapped.Close()
	if freelist != 0 || compactReason(pageCount, freelist, pageSize, false) != "" {
		t.Fatalf("after the swap: %d pages, %d free; want none free and not due", pageCount, freelist)
	}
	// A planned boot drops a copy whose tail is past the bound.
	s, path = compactTestStore(t, 5, 10, 3000)
	statePath = s.statePath
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	for seq := uint64(6); seq <= 20; seq++ {
		compactTestAppend(t, s, seq)
	}
	old := compactRebaseMaxTail
	compactRebaseMaxTail = 5
	defer func() { compactRebaseMaxTail = old }()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := RebaseOntoCompactCopy(path, true); err != nil || strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("a copy past the bound = %q, %v; want it dropped", outcome, err)
	}
	if exists(statePath+compactStagedSuffix) || exists(statePath+compactSidecarSuffix) {
		t.Fatal("the copy past the bound was kept")
	}
	if exists(statePath + ".layout.json") {
		t.Fatal("a layout marker appeared")
	}
}

// The interrupted-swap finish path and a copy dropped for being off the chain
// write no layout marker either (the marker writes after finishSwap are gone).
func TestCompactRebaseWritesNoLayoutMarkerOnEveryPath(t *testing.T) {
	t.Run("finishes an interrupted swap", func(t *testing.T) {
		s, path := compactTestStore(t, 5, 10, 3000)
		statePath := s.statePath
		if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(statePath, statePath+precompactSuffix); err != nil {
			t.Fatal(err)
		}
		outcome, err := RebaseOntoCompactCopy(path, false)
		if err != nil || !strings.HasPrefix(outcome, "finished an interrupted rebase") {
			t.Fatalf("RebaseOntoCompactCopy = %q, %v", outcome, err)
		}
		if exists(statePath + ".layout.json") {
			t.Fatal("a layout marker appeared")
		}
	})
	t.Run("drops a copy off the chain", func(t *testing.T) {
		s, path := compactTestStore(t, 5, 10, 3000)
		statePath := s.statePath
		if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		// The store moved to another history after the copy (as in
		// TestCompactRebaseRefusesAMisfitCopy): event 6 does not follow it.
		if _, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
VALUES ('digest-fork', ?, 6, 'another-head', 'k', 'a', 'b', '[]', 'r', 'key-fork', 'finalized', 'd', 'e', 'c', 'f', 1, 0, ?)`, compactTestComputer, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`UPDATE computer_event_projection_heads SET sequence=6, canonical_event_head='fork-6'`); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if outcome, err := RebaseOntoCompactCopy(path, true); err != nil || !strings.HasPrefix(outcome, "discarded") {
			t.Fatalf("RebaseOntoCompactCopy = %q, %v; want discarded", outcome, err)
		}
		if exists(statePath + ".layout.json") {
			t.Fatal("a layout marker appeared")
		}
	})
}
