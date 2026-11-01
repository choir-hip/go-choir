package store

// The staged copy against the windowed event index (review 2026-10-31, B1):
// the trim deletes the finalized projection_batch_recorded rows below the
// window, 99.3% of the index, so a probe of the index row after the copy's
// head cannot prove a copy older than the window is on the chain. And the
// tables written by direct SQL (B2).

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// compactTestAppendKind records event seq of the given kind in the index only.
func compactTestAppendKind(t *testing.T, s *Store, seq uint64, kind string) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
VALUES (?, ?, ?, ?, ?, 'a', 'b', '[]', 'r', ?, 'finalized', 'd', 'e', 'c', 'f', 1, 0, ?)`,
		"digest-"+compactTestHead(seq), compactTestComputer, seq, compactTestHead(seq-1), kind, "key-"+compactTestHead(seq), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func compactTestSetHead(t *testing.T, s *Store, seq uint64) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE computer_event_projection_heads SET sequence=?, canonical_event_head=?`, seq, compactTestHead(seq)); err != nil {
		t.Fatal(err)
	}
}

func TestCompactRebaseKeepsACopyOlderThanTheTrimWindow(t *testing.T) {
	batch := string(computerevent.EventProjectionBatchRecorded)
	run := func(t *testing.T, afterClose func(t *testing.T, statePath string)) string {
		s, path := compactTestStore(t, 5, 10, 3000)
		if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		// Event 6 is a batch row, the kind the trim deletes; the head then
		// moves far past the window and the trim runs.
		compactTestAppendKind(t, s, 6, batch)
		compactTestAppendKind(t, s, 30010, "k")
		compactTestSetHead(t, s, 30010)
		if n, err := s.CatchUpProjectionBatchTrim(context.Background()); err != nil || n != 1 {
			t.Fatalf("trim = %d, %v; want event 6 deleted", n, err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if afterClose != nil {
			afterClose(t, s.statePath)
		}
		outcome, err := RebaseOntoCompactCopy(path, true)
		if err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	t.Run("kept and swapped", func(t *testing.T) {
		if outcome := run(t, nil); !strings.HasPrefix(outcome, "rebased") {
			t.Fatalf("outcome %q; want a rebase onto the copy", outcome)
		}
	})
	t.Run("a replaced state file is refused", func(t *testing.T) {
		outcome := run(t, func(t *testing.T, statePath string) {
			// A restore writes a new file and renames it over: the anchor is
			// the file's identity, so a replaced file voids it.
			raw, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(statePath+".new", raw, 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(statePath+".new", statePath); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.HasPrefix(outcome, "discarded") {
			t.Fatalf("outcome %q; want discarded", outcome)
		}
	})
	t.Run("over the bound is still refused", func(t *testing.T) {
		old := compactRebaseMaxTail
		compactRebaseMaxTail = 1000
		t.Cleanup(func() { compactRebaseMaxTail = old })
		if outcome := run(t, nil); !strings.Contains(outcome, "bound") {
			t.Fatalf("outcome %q; want discarded over the bound", outcome)
		}
	})
}

// Rows of the tables written by direct SQL, with no op on the tape, since the
// copy was staged are carried over by the swap; a released slot stays released
// (review 2026-10-31, B2).
func TestCompactRebaseCarriesDirectSQLTables(t *testing.T) {
	s, path := compactTestStore(t, 5, 10, 3000)
	now := time.Now().UTC()
	slot := func(name string) {
		if _, err := s.db.Exec(`INSERT INTO co_super_slots (owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at) VALUES ('o', 't', ?, 'run-'||?, 'a', 'r', ?, ?)`, name, name, now, now); err != nil {
			t.Fatal(err)
		}
	}
	slot("released-later")
	slot("updated-later")
	if _, err := s.db.Exec(`INSERT INTO agent_evidence (evidence_id, owner_id, agent_id, kind, content, created_at) VALUES ('ev-before', 'o', 'a', 'k', 'c', ?)`, now); err != nil {
		t.Fatal(err)
	}
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	// After the copy: a new claim, a release, an update, new evidence, and
	// evidence removed.
	slot("claimed-later")
	if _, err := s.db.Exec(`DELETE FROM co_super_slots WHERE slot='released-later'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE co_super_slots SET run_id='run-new' WHERE slot='updated-later'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO agent_evidence (evidence_id, owner_id, agent_id, kind, content, created_at) VALUES ('ev-after', 'o', 'a', 'k', 'c', ?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM agent_evidence WHERE evidence_id='ev-before'`); err != nil {
		t.Fatal(err)
	}
	compactTestAppend(t, s, 6)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := RebaseOntoCompactCopy(path, true); err != nil || !strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("planned boot: %q, %v", outcome, err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(StateDBPath(path), dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	list := func(q string) string {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				t.Fatal(err)
			}
			out = append(out, a)
		}
		return strings.Join(out, ",")
	}
	if got := list(`SELECT slot||'='||run_id FROM co_super_slots ORDER BY slot`); got != "claimed-later=run-claimed-later,updated-later=run-new" {
		t.Fatalf("slots after the swap: %q", got)
	}
	if got := list(`SELECT evidence_id FROM agent_evidence ORDER BY evidence_id`); got != "ev-after" {
		t.Fatalf("evidence after the swap: %q", got)
	}
}
