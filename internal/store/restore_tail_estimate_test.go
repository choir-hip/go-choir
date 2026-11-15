package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The restore act's acceptance answer estimates a tail from the local index:
// the target's sequence less the newest finalized snapshot at or below it.
// Ways it could fail: it takes a snapshot above the target (a tail that is
// negative or zero), a prepared or another computer's snapshot, or the newest
// snapshot instead of the one at or below the target; it needs the tape; or an
// event the index does not hold reads as found.
// Written after the function it covers, and checked by hand against each way.
func TestRestoreTailEstimateReadsTheLocalIndex(t *testing.T) {
	ctx := context.Background()
	s, err := OpenFresh(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	insert := func(digest, computer string, seq int, kind computerevent.EventKind, status string) {
		t.Helper()
		_, err := s.DB().ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at, finalized_at) VALUES (?, ?, ?, '', ?, '', '', '[]', '', ?, ?, '', '', '', '', 1, 0, ?, ?)`,
			digest, computer, seq, string(kind), digest, status, time.Now().UTC(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, other := computerevent.EventSnapshotRecorded, computerevent.EventArtifactProduced
	insert("d10", "c1", 10, snapshot, "finalized")
	insert("d40", "c1", 40, snapshot, "finalized")
	insert("d60", "c1", 60, other, "finalized")
	insert("d70", "c1", 70, snapshot, "finalized")
	insert("d90", "c1", 90, snapshot, "prepared")
	insert("e55", "c2", 55, snapshot, "finalized")
	insert("d100", "c1", 100, other, "finalized")

	for _, tc := range []struct {
		name, computer, digest string
		target, base           uint64
		found                  bool
	}{
		{"between two snapshots", "c1", "d60", 60, 40, true},
		{"at a snapshot", "c1", "d70", 70, 70, true},
		{"above the newest finalized one (the prepared one and another computer's do not count)", "c1", "d100", 100, 70, true},
		{"below every snapshot", "c1", "d10", 10, 10, true},
		{"an event the index does not hold", "c1", "nope", 0, 0, false},
		{"another computer's event", "c2", "d60", 0, 0, false},
	} {
		target, base, found, err := s.RestoreTailEstimate(ctx, tc.computer, tc.digest)
		if err != nil || found != tc.found || target != tc.target || base != tc.base {
			t.Errorf("%s: target=%d base=%d found=%t err=%v, want %d %d %t", tc.name, target, base, found, err, tc.target, tc.base, tc.found)
		}
	}
	insert("f5", "c3", 5, other, "finalized")
	if target, base, found, err := s.RestoreTailEstimate(ctx, "c3", "f5"); err != nil || !found || target != 5 || base != 0 {
		t.Errorf("a computer with no snapshot: target=%d base=%d found=%t err=%v, want 5 0 true", target, base, found, err)
	}
}
