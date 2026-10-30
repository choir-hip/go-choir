package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Ways the local "has a snapshot" answer could fail: it counts a prepared (not
// finalized) event, another computer's event or another kind; it returns the
// oldest rather than the newest; or it needs the tape.
func TestNewestEventOfKindReadsTheLocalIndex(t *testing.T) {
	ctx := context.Background()
	s, err := OpenFresh(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if _, _, found, err := s.NewestEventOfKind(ctx, "c1", computerevent.EventSnapshotRecorded); err != nil || found {
		t.Fatalf("an empty index: found=%t err=%v", found, err)
	}
	insert := func(digest, computer string, seq int, kind computerevent.EventKind, status string, finalized any) {
		t.Helper()
		_, err := s.DB().ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at, finalized_at) VALUES (?, ?, ?, '', ?, '', '', '[]', '', ?, ?, '', '', '', '', 1, 0, ?, ?)`,
			digest, computer, seq, string(kind), digest, status, time.Now().UTC(), finalized)
		if err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	insert("d5", "c1", 5, computerevent.EventSnapshotRecorded, "finalized", at.Add(-48*time.Hour))
	insert("d9", "c1", 9, computerevent.EventSnapshotRecorded, "finalized", at)
	insert("d10", "c1", 10, computerevent.EventFileRootCommitted, "finalized", at.Add(time.Hour))
	insert("d12", "c1", 12, computerevent.EventSnapshotRecorded, "prepared", nil)
	insert("e3", "c2", 3, computerevent.EventSnapshotRecorded, "finalized", at.Add(2*time.Hour))

	seq, when, found, err := s.NewestEventOfKind(ctx, "c1", computerevent.EventSnapshotRecorded)
	if err != nil || !found || seq != 9 || !when.Equal(at) {
		t.Fatalf("seq=%d at=%s found=%t err=%v, want the finalized snapshot at 9", seq, when, found, err)
	}
}
