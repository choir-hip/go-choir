package store

// Off-boot compaction (compact.go). Ways it can fail, written before the
// tests:
//
//   - C1: it copies a file that is not mostly free (needless IO), or skips
//     one that is;
//   - C2: it copies with too little disk and fills the volume;
//   - C3: the copy is not incremental auto_vacuum, so later deletes never
//     return pages;
//   - C4: a crashed earlier copy is trusted, or blocks the next;
//   - C5: a crash boot (no planned-restart marker) swaps; boot stays as it
//     was unless the host planned the restart;
//   - C6: the copy swapped in is not on this store's chain (the store moved
//     to another history), or its tail is past the replay bound, or its
//     sidecar is torn: the boot replays onto the wrong state or for too long;
//   - C7: the swap leaves the original's -wal beside the new file, which
//     SQLite would replay over it, or drops the original's WAL, which the
//     rollback copy needs;
//   - C8: a crash between the renames leaves no state file (the next Open
//     makes an empty store), instead of finishing or undoing the swap;
//   - C9: the stream_seq lease is trusted on the swapped-in file, whose
//     history is older than the lease;
//   - C10: the rollback copy outlives the replay, holding the old size on
//     disk forever;
//   - C11: incremental_vacuum holds the writer past its bound, or runs on a
//     file that is not incremental.

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

const compactTestComputer = "computer-compact-test"

// compactTestStore opens a store holding a chain of n events (heads and
// index rows), a probe table with kept rows, and many freed pages.
func compactTestStore(t *testing.T, n uint64, kept, deleted int) (*Store, string) {
	t.Helper()
	s, path := trimTestOpen(t)
	for seq := uint64(1); seq <= n; seq++ {
		compactTestAppend(t, s, seq)
	}
	if _, err := s.db.Exec(`CREATE TABLE compact_probe (id INTEGER PRIMARY KEY, payload TEXT)`); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("y", 4000)
	tx, _ := s.db.Begin()
	for i := 0; i < kept+deleted; i++ {
		if _, err := tx.Exec(`INSERT INTO compact_probe VALUES (?, ?)`, i, payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM compact_probe WHERE id >= ?`, kept); err != nil {
		t.Fatal(err)
	}
	return s, path
}

func compactTestHead(seq uint64) string {
	return "head-" + string(rune('a'+seq%26)) + "-" + time.Unix(int64(seq), 0).UTC().Format("150405")
}

// compactTestAppend records event seq: an index row whose previous head is
// seq-1's head, and the projection head at seq.
func compactTestAppend(t *testing.T, s *Store, seq uint64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
VALUES (?, ?, ?, ?, 'k', 'a', 'b', '[]', 'r', ?, 'finalized', 'd', 'e', 'c', 'f', 1, 0, ?)`,
		"digest-"+compactTestHead(seq), compactTestComputer, seq, compactTestHead(seq-1), "key-"+compactTestHead(seq), now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, 'd', 'e', 'c', 'f', 1, 0, ?)
ON CONFLICT(computer_id) DO UPDATE SET sequence=excluded.sequence, canonical_event_head=excluded.canonical_event_head`, compactTestComputer, seq, compactTestHead(seq), now); err != nil {
		t.Fatal(err)
	}
}

func compactTestRows(t *testing.T, statePath string) (rows int, seq uint64, av int) {
	t.Helper()
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT COUNT(*) FROM compact_probe`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT sequence FROM computer_event_projection_heads`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA auto_vacuum`).Scan(&av); err != nil {
		t.Fatal(err)
	}
	return rows, seq, av
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// The whole path: a copy made while the store runs, more events after it, a
// planned boot that swaps the copy in and keeps the original as the rollback,
// and the cleanup once the replay is done.
func TestCompactCopyRebasesAPlannedBoot(t *testing.T) {
	s, path := compactTestStore(t, 5, 10, 3000)
	statePath := s.statePath
	// A crashed earlier copy is removed, not trusted (C4).
	if err := os.WriteFile(statePath+compactStagingSuffix, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	done, err := s.PrepareCompactCopy(context.Background())
	if err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	if exists(statePath + compactStagingSuffix) {
		t.Fatal("staging file left behind (C4)")
	}
	// The store keeps running after the copy: events 6 and 7 are the tail the
	// boot replay will bring back from the tape.
	compactTestAppend(t, s, 6)
	compactTestAppend(t, s, 7)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before := fileSize(t, statePath)

	// A crash boot leaves everything as it was (C5).
	outcome, err := RebaseOntoCompactCopy(path, false)
	if err != nil || outcome != "" {
		t.Fatalf("crash boot: %q, %v", outcome, err)
	}
	if fileSize(t, statePath) != before || !exists(statePath+compactStagedSuffix) {
		t.Fatal("a crash boot changed the state file or dropped the staged copy (C5)")
	}

	outcome, err = RebaseOntoCompactCopy(path, true)
	if err != nil || !strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("planned boot: %q, %v", outcome, err)
	}
	if after := fileSize(t, statePath); after*3 > before {
		t.Fatalf("state file %d -> %d bytes, want under a third", before, after)
	}
	rows, seq, av := compactTestRows(t, statePath)
	if rows != 10 || seq != 5 {
		t.Fatalf("swapped file: %d rows at sequence %d, want 10 at 5", rows, seq)
	}
	if av != 2 {
		t.Fatalf("auto_vacuum = %d, want 2 (C3)", av)
	}
	// The original, with its WAL, is the rollback; nothing of it stays beside
	// the new file (C7).
	if !exists(statePath + precompactSuffix) {
		t.Fatal("rollback copy missing")
	}
	if exists(statePath+"-wal") && fileSize(t, statePath+"-wal") > 0 {
		t.Fatal("a WAL sits beside the swapped-in file (C7)")
	}
	if rows, seq, _ := compactTestRows(t, statePath+precompactSuffix); rows != 10 || seq != 7 {
		t.Fatalf("rollback copy: %d rows at %d, want 10 at 7 (C7)", rows, seq)
	}
	// The lease is distrusted on the swapped-in history (C9).
	if !exists(filepath.Join(deriveTextureWorkspacePath(path), storeschema.LeaseUntrustedFile)) {
		t.Fatal("no lease untrust marker (C9)")
	}
	if exists(statePath+compactStagedSuffix) || exists(statePath+compactSidecarSuffix) {
		t.Fatal("staged copy left after the swap")
	}

	// The replay done, the rollback copy goes (C10).
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	s2.removePrecompact()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if exists(statePath + precompactSuffix + suffix) {
			t.Fatalf("rollback copy %q left (C10)", suffix)
		}
	}
}

func TestCompactCopySkips(t *testing.T) {
	t.Run("not mostly free (C1)", func(t *testing.T) {
		s, _ := compactTestStore(t, 2, 3000, 50)
		if done, err := s.PrepareCompactCopy(context.Background()); done || err != nil {
			t.Fatalf("PrepareCompactCopy = %v, %v; want a quiet skip", done, err)
		}
	})
	t.Run("too little disk (C2)", func(t *testing.T) {
		old := compactFreeBytes
		defer func() { compactFreeBytes = old }()
		compactFreeBytes = func(string) (int64, error) { return 1000, nil }
		s, _ := compactTestStore(t, 2, 10, 3000)
		if done, err := s.PrepareCompactCopy(context.Background()); done || err == nil || !strings.Contains(err.Error(), "free") {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		if exists(s.statePath+compactStagingSuffix) || exists(s.statePath+compactStagedSuffix) {
			t.Fatal("a skipped copy left a file")
		}
	})
}

// A staged copy that no longer fits the store is dropped at the planned boot
// and the original stays (C6).
func TestCompactRebaseRefusesAMisfitCopy(t *testing.T) {
	cases := map[string]func(t *testing.T, s *Store, statePath string){
		"off the chain": func(t *testing.T, s *Store, statePath string) {
			// The store moved to another history after the copy: event 6
			// does not follow the copy's head.
			if _, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
VALUES ('digest-fork', ?, 6, 'another-head', 'k', 'a', 'b', '[]', 'r', 'key-fork', 'finalized', 'd', 'e', 'c', 'f', 1, 0, ?)`, compactTestComputer, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(`UPDATE computer_event_projection_heads SET sequence=6, canonical_event_head='fork-6'`); err != nil {
				t.Fatal(err)
			}
		},
		"tail past the bound": func(t *testing.T, s *Store, statePath string) {
			old := compactRebaseMaxTail
			compactRebaseMaxTail = 1
			t.Cleanup(func() { compactRebaseMaxTail = old })
			compactTestAppend(t, s, 6)
			compactTestAppend(t, s, 7)
		},
		"torn sidecar": func(t *testing.T, s *Store, statePath string) {
			if err := os.WriteFile(statePath+compactSidecarSuffix, []byte(`{"sequence":`), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"size differs": func(t *testing.T, s *Store, statePath string) {
			f, err := os.OpenFile(statePath+compactStagedSuffix, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.Write([]byte("x"))
			_ = f.Close()
		},
	}
	for name, misfit := range cases {
		t.Run(name, func(t *testing.T) {
			s, path := compactTestStore(t, 5, 10, 3000)
			statePath := s.statePath
			if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
				t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
			}
			misfit(t, s, statePath)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			before := fileSize(t, statePath)
			outcome, err := RebaseOntoCompactCopy(path, true)
			if err != nil || !strings.HasPrefix(outcome, "discarded") {
				t.Fatalf("RebaseOntoCompactCopy = %q, %v; want discarded", outcome, err)
			}
			if fileSize(t, statePath) < before || exists(statePath+precompactSuffix) {
				t.Fatal("a misfit copy was swapped in")
			}
			if exists(statePath+compactStagedSuffix) || exists(statePath+compactSidecarSuffix) {
				t.Fatal("a misfit copy was kept")
			}
		})
	}
}

// A crash between the renames: the next boot, planned or not, finishes the
// swap when the staged copy is intact, and undoes it otherwise (C8).
func TestCompactRebaseRecoversAnInterruptedSwap(t *testing.T) {
	for _, staged := range []bool{true, false} {
		name := map[bool]string{true: "finishes", false: "undoes"}[staged]
		t.Run(name, func(t *testing.T) {
			s, path := compactTestStore(t, 5, 10, 3000)
			statePath := s.statePath
			if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
				t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
			}
			compactTestAppend(t, s, 6)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			// The crash came after the original moved aside.
			if err := os.Rename(statePath, statePath+precompactSuffix); err != nil {
				t.Fatal(err)
			}
			if !staged {
				_ = os.Remove(statePath + compactStagedSuffix)
			}
			outcome, err := RebaseOntoCompactCopy(path, false)
			if err != nil {
				t.Fatal(err)
			}
			if !exists(statePath) {
				t.Fatalf("%q: no state file after recovery (C8)", outcome)
			}
			if exists(statePath+"-wal") && fileSize(t, statePath+"-wal") > 0 && staged {
				t.Fatal("the original's WAL sits beside the new file (C7)")
			}
			_, seq, _ := compactTestRows(t, statePath)
			want := map[bool]uint64{true: 5, false: 6}[staged]
			if seq != want {
				t.Fatalf("%q: state file at sequence %d, want %d", outcome, seq, want)
			}
		})
	}
}

func TestCompactSidecarIsJSON(t *testing.T) {
	s, _ := compactTestStore(t, 3, 10, 3000)
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	raw, err := os.ReadFile(s.statePath + compactSidecarSuffix)
	if err != nil {
		t.Fatal(err)
	}
	var side compactSidecar
	if err := json.Unmarshal(raw, &side); err != nil || side.Sequence != 3 || side.Head != compactTestHead(3) || side.Size != fileSize(t, s.statePath+compactStagedSuffix) {
		t.Fatalf("sidecar %s: %+v, %v", raw, side, err)
	}
}

// C11: on a compacted file, deletes return pages with incremental_vacuum, in
// bounded chunks; a file that was not compacted is left alone.
func TestReclaimFreePagesIsBoundedAndOnlyOnIncrementalFiles(t *testing.T) {
	oldChunk, oldPause, oldMax := incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages
	defer func() {
		incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages = oldChunk, oldPause, oldMax
	}()
	incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages = 50, 0, 120

	s, path := compactTestStore(t, 3, 10, 3000)
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := RebaseOntoCompactCopy(path, true); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(StateDBPath(path), dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`WITH RECURSIVE c(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM c WHERE i<1999) INSERT INTO compact_probe (id, payload) SELECT i+100000, hex(zeroblob(1000)) FROM c`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM compact_probe WHERE id >= 100000`); err != nil {
		t.Fatal(err)
	}
	freed, err := (&Store{db: db}).reclaimFreePages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if freed == 0 || freed > int64(incrementalVacuumMaxPages)+int64(incrementalVacuumChunk) {
		t.Fatalf("freed %d pages, want 1..%d", freed, incrementalVacuumMaxPages+incrementalVacuumChunk)
	}
	// A plain (non-incremental) file is left alone.
	plain, _ := compactTestStore(t, 1, 10, 500)
	if n, err := plain.reclaimFreePages(context.Background()); err != nil || n != 0 {
		t.Fatalf("plain file reclaimed %d, %v", n, err)
	}
}

// Boot stays light: compaction is its own operation, never Open's.
func TestOpenNeverCompacts(t *testing.T) {
	s, path := compactTestStore(t, 1, 10, 3000)
	statePath := s.statePath
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before := fileSize(t, statePath)
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if after := fileSize(t, statePath); after*3 <= before {
		t.Fatalf("state file %d -> %d, Open compacted", before, after)
	}
}
