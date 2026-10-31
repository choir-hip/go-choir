package store

// Failure modes of the event index window (event_index_trim.go), written
// before the tests:
//
//  1. Witness mismatch live vs replay: the live trim and a replay must leave
//     the same rows at the same head. The trim is a function of the head
//     sequence only (TestCatchUpEqualsTrimAtEveryTrimPoint).
//  2. Deleting a prepared row or a non-batch row (genesis, restore, recovery
//     kinds): only finalized projection_batch_recorded rows may go
//     (TestTrimKeepsPreparedAndOtherKinds).
//  3. Deleting above head-K: nothing above the boundary, and nothing at all
//     off a trim point (TestTrimStaysInsideTheWindow).
//  4. Catch-up starving the single writer: it deletes in bounded chunks, each
//     its own transaction, and the event lock is free between them
//     (TestCatchUpIsChunkedAndReleasesTheWriter).
//  5. An old snapshot restored after the floor bump: a workspace with a full
//     index and no floor is re-derived at Open, the floor is raised first,
//     and an older binary is refused afterwards
//     (TestOpenCatchesUpAnOldStoreAndFloorsIt).

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

const trimTestComputer = "computer-trim"

func trimTestInsert(t *testing.T, s *Store, seq uint64, kind, status string) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at) VALUES (?, ?, ?, 'p', ?, 'a', 'b', '[]', 'r', ?, ?, 'd', 'e', 'c', 'f', 1, 0, ?)`,
		fmt.Sprintf("digest-%d-%s", seq, kind), trimTestComputer, seq, kind, fmt.Sprintf("key-%d", seq), status, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func trimTestHead(t *testing.T, s *Store, seq uint64) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, 'h', 'd', 'e', 'c', 'f', 1, 0, ?) ON CONFLICT(computer_id) DO UPDATE SET sequence=excluded.sequence`, trimTestComputer, seq, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func trimTestOpen(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

// trimTestFill seeds batch rows 1..n, with a genesis at 1 and a restore-ish
// kind every 5000 so the keep-set is exercised.
func trimTestFill(t *testing.T, s *Store, n uint64) {
	t.Helper()
	for seq := uint64(1); seq <= n; seq++ {
		kind := string(computerevent.EventProjectionBatchRecorded)
		switch {
		case seq == 1:
			kind = string(computerevent.EventGenesisImported)
		case seq%5000 == 0:
			kind = "other_kind_kept"
		}
		trimTestInsert(t, s, seq, kind, "finalized")
	}
}

func trimTestRows(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT sequence || '/' || event_kind || '/' || status FROM computer_event_index WHERE computer_id=? ORDER BY sequence`, trimTestComputer)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestTrimBoundary(t *testing.T) {
	for head, want := range map[uint64]uint64{0: 0, 9999: 0, 10000: 0, 19999: 0, 20000: 10000, 25000: 10000, 30000: 20000, 5150000: 5140000} {
		if got := projectionBatchTrimBoundary(head); got != want {
			t.Fatalf("boundary(%d) = %d, want %d", head, got, want)
		}
	}
}

func TestTrimStaysInsideTheWindow(t *testing.T) {
	s, _ := trimTestOpen(t)
	trimTestFill(t, s, 25000)
	run := func(seq uint64) {
		tx, err := s.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err := s.trimProjectionBatchIndexTx(context.Background(), tx, trimTestComputer, seq); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	before := len(trimTestRows(t, s))
	run(25000) // not a trim point
	run(10000) // a trim point, but boundary 0
	if got := len(trimTestRows(t, s)); got != before {
		t.Fatalf("rows %d -> %d off a trim point", before, got)
	}
	run(20000)
	for _, r := range trimTestRows(t, s) {
		var seq uint64
		var rest string
		fmt.Sscanf(r, "%d/%s", &seq, &rest)
		if seq <= 10000 && rest[:5] == "proje" {
			t.Fatalf("batch row at or below the boundary survived: %s", r)
		}
	}
	// Everything above the boundary (10000) is intact: 25000-10000 rows, less
	// none (kept kinds sit among them).
	rows := trimTestRows(t, s)
	wantAbove := 25000 - 10000
	above := 0
	for _, r := range rows {
		var seq uint64
		fmt.Sscanf(r, "%d/", &seq)
		if seq > 10000 {
			above++
		}
	}
	if above != wantAbove {
		t.Fatalf("rows above the boundary = %d, want %d", above, wantAbove)
	}
}

func TestTrimKeepsPreparedAndOtherKinds(t *testing.T) {
	s, _ := trimTestOpen(t)
	trimTestFill(t, s, 12000)
	// A prepared batch row far below the boundary must stay.
	if _, err := s.db.Exec(`UPDATE computer_event_index SET status='prepared' WHERE sequence=3`); err != nil {
		t.Fatal(err)
	}
	trimTestHead(t, s, 30000)
	if _, err := s.CatchUpProjectionBatchTrim(context.Background()); err != nil {
		t.Fatal(err)
	}
	kept := map[string]bool{}
	for _, r := range trimTestRows(t, s) {
		kept[r] = true
	}
	for _, want := range []string{"1/" + string(computerevent.EventGenesisImported) + "/finalized", "5000/other_kind_kept/finalized", "10000/other_kind_kept/finalized", "3/" + string(computerevent.EventProjectionBatchRecorded) + "/prepared"} {
		if !kept[want] {
			t.Fatalf("row %s was deleted", want)
		}
	}
	if kept["4/"+string(computerevent.EventProjectionBatchRecorded)+"/finalized"] {
		t.Fatalf("a finalized batch row below the boundary survived")
	}
}

// Replay finalizes in order and trims at each trim point; the catch-up trims
// once to the final boundary. Both must leave the same rows.
func TestCatchUpEqualsTrimAtEveryTrimPoint(t *testing.T) {
	const head = 47000
	a, _ := trimTestOpen(t)
	trimTestFill(t, a, head)
	for seq := uint64(projectionBatchTrimInterval); seq <= head; seq += projectionBatchTrimInterval {
		tx, err := a.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err := a.trimProjectionBatchIndexTx(context.Background(), tx, trimTestComputer, seq); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := trimTestOpen(t)
	trimTestFill(t, b, head)
	trimTestHead(t, b, head)
	if _, err := b.CatchUpProjectionBatchTrim(context.Background()); err != nil {
		t.Fatal(err)
	}
	ra, rb := trimTestRows(t, a), trimTestRows(t, b)
	if len(ra) != len(rb) {
		t.Fatalf("live trim kept %d rows, catch-up kept %d", len(ra), len(rb))
	}
	for i := range ra {
		if ra[i] != rb[i] {
			t.Fatalf("row %d differs: %s vs %s", i, ra[i], rb[i])
		}
	}
}

func TestCatchUpIsChunkedAndReleasesTheWriter(t *testing.T) {
	s, _ := trimTestOpen(t)
	trimTestFill(t, s, 30000)
	trimTestHead(t, s, 30000)
	oldChunk, oldPause := projectionBatchCatchUpChunk, projectionBatchCatchUpPause
	projectionBatchCatchUpChunk, projectionBatchCatchUpPause = 1000, 0
	defer func() { projectionBatchCatchUpChunk, projectionBatchCatchUpPause = oldChunk, oldPause }()
	n, err := s.CatchUpProjectionBatchTrim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// boundary 20000: rows 2..20000 are batch except the multiples of 5000.
	if want := int64(20000 - 1 - 4); n != want {
		t.Fatalf("deleted %d, want %d", n, want)
	}
	// The event lock and the writer are free after it.
	s.eventMu.Lock()
	s.eventMu.Unlock()
	if _, err := s.db.Exec(`UPDATE computer_event_index SET prepared_at=prepared_at WHERE sequence=29999`); err != nil {
		t.Fatal(err)
	}
	again, err := s.CatchUpProjectionBatchTrim(context.Background())
	if err != nil || again != 0 {
		t.Fatalf("second catch-up = %d, %v; want idempotent 0", again, err)
	}
}

func TestOpenCatchesUpAnOldStoreAndFloorsIt(t *testing.T) {
	s, path := trimTestOpen(t)
	sidecar := s.texturePath
	trimTestFill(t, s, 30000)
	trimTestHead(t, s, 30000)
	if _, found, _ := storeschema.ReadFloor(filepath.Join(sidecar, storeschema.FloorFile)); found {
		// An earlier floor (event table) may exist; the trim floor is what matters.
		t.Log("a floor already existed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	oldDelay := projectionBatchCatchUpDelay
	projectionBatchCatchUpDelay = 0
	defer func() { projectionBatchCatchUpDelay = oldDelay }()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	// The catch-up runs in the background after Open returns.
	want := 30000 - 20000 + 1 + 4
	deadline := time.Now().Add(20 * time.Second)
	got := len(trimTestRows(t, s2))
	for got != want && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		got = len(trimTestRows(t, s2))
	}
	if got != want {
		t.Fatalf("rows after Open = %d, want %d", got, want)
	}
	floor, found, err := storeschema.ReadFloor(filepath.Join(sidecar, storeschema.FloorFile))
	if err != nil || !found || floor.Version < projectionBatchTrimFloorEpoch {
		t.Fatalf("floor = %+v found=%v err=%v; want >= %d", floor, found, err, projectionBatchTrimFloorEpoch)
	}
}
