package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func eiInsert(t *testing.T, s *Store, digest string, seq uint64, kind, status, pinReceipt, pins string, at time.Time) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
		VALUES (?, ?, ?, 'p', ?, 'a', ?, ?, 'r', ?, ?, 'd', 'e', 'c', 'f', 1, 0, ?)`,
		digest, trimTestComputer, seq, kind, pinReceipt, pins, "key-"+digest, status, at)
	if err != nil {
		t.Fatal(err)
	}
}

// Failure 1: a store's replay of an event with no payload pins wrote JSON
// null into payload_pin_receipt_digests_json where the live append wrote "[]".
func TestPrepareStoresNoPayloadPinsAsEmptyArrayNotNull(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-test",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt:     time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey: "genesis-test", ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PayloadCommitment: storeTestDigest('a'), PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: storeTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment = commitment
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	for _, pins := range [][]string{nil, {}} {
		if _, err := s.db.Exec(`DELETE FROM computer_event_index`); err != nil {
			t.Fatal(err)
		}
		request := computerevent.CASRequest{
			Event: event, EventDigest: digest, EventArtifactDigest: digest,
			EventPinReceiptDigest: storeTestDigest('c'), PayloadPinReceiptDigests: pins, Input: input, Next: next,
			PinIntentCommitment: pinIntent,
		}
		if err := s.Prepare(ctx, request); err != nil {
			t.Fatal(err)
		}
		var stored string
		if err := s.db.QueryRow(`SELECT payload_pin_receipt_digests_json FROM computer_event_index WHERE event_digest=?`, digest).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != "[]" {
			t.Fatalf("pins %#v stored as %q, want []", pins, stored)
		}
	}
}

// Failure 4: the diagnostic names the differing rows and columns of the
// event index, ignoring the timestamp columns the witness ignores.
func TestEventIndexFingerprintsNameDifferingColumnsAndCountDisposableRows(t *testing.T) {
	ctx := context.Background()
	live, _ := trimTestOpen(t)
	replay, _ := trimTestOpen(t)
	t0 := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	batch := string(computerevent.EventProjectionBatchRecorded)
	// Equal rows (timestamps differ), a null/[] pair, one changed column,
	// one live-only row, one replay-only row, and a disposable backlog row on
	// the live side only (at or below the boundary 10).
	eiInsert(t, live, "same", 11, "k", "finalized", "r", "[]", t0)
	eiInsert(t, replay, "same", 11, "k", "finalized", "r", "[]", t0.Add(time.Hour))
	eiInsert(t, live, "pins", 12, "k", "finalized", "r", "[]", t0)
	eiInsert(t, replay, "pins", 12, "k", "finalized", "r", "null", t0)
	eiInsert(t, live, "chg", 13, "k", "finalized", "r1", "[]", t0)
	eiInsert(t, replay, "chg", 13, "k", "finalized", "r2", "[]", t0)
	eiInsert(t, live, "lonly", 14, "k", "prepared", "r", "[]", t0)
	eiInsert(t, replay, "ronly", 15, "k", "finalized", "r", "[]", t0)
	eiInsert(t, live, "backlog", 5, batch, "finalized", "r", "[]", t0)

	lf, ld, err := live.ListEventIndexFingerprints(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	rf, rd, err := replay.ListEventIndexFingerprints(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ld != 1 || rd != 0 || len(lf) != 4 || len(rf) != 4 {
		t.Fatalf("live %d rows (%d disposable), replay %d rows (%d disposable); want 4 (1) and 4 (0)", len(lf), ld, len(rf), rd)
	}
	by := func(fps []EventIndexFingerprint) map[string]EventIndexFingerprint {
		m := map[string]EventIndexFingerprint{}
		for _, f := range fps {
			m[f.EventDigest] = f
		}
		return m
	}
	l, r := by(lf), by(rf)
	if l["same"].RowDigest == "" || l["same"].RowDigest != r["same"].RowDigest {
		t.Fatal("rows differing only in prepared_at must have equal digests")
	}
	if l["pins"].RowDigest != r["pins"].RowDigest {
		t.Fatal("null and [] pin lists must have equal digests, as the witness reads them")
	}
	if l["chg"].RowDigest == r["chg"].RowDigest {
		t.Fatal("a changed column must change the row digest")
	}
	var diff []string
	for name, d := range l["chg"].FieldDigests {
		if d != r["chg"].FieldDigests[name] {
			diff = append(diff, name)
		}
	}
	if len(diff) != 1 || diff[0] != "event_pin_receipt_digest" {
		t.Fatalf("differing columns = %v", diff)
	}
	if l["lonly"].Sequence != 14 || l["lonly"].Status != "prepared" || l["lonly"].EventKind != "k" {
		t.Fatalf("live-only row = %+v", l["lonly"])
	}
	if _, ok := l["chg"].FieldDigests["prepared_at"]; ok {
		t.Fatal("timestamp columns are not hashed by the witness and must not be fingerprinted")
	}
}

// Failure 5: the object-graph diagnostic read at "now". Its transaction
// variant reads the caller's snapshot.
func TestListObjectGraphFingerprintsTxReadsTheSnapshotNotNow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	insert := func(id string) {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, 'run', 'o', 'c', 'v1', 'h', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, FALSE, '')`, id); err != nil {
			t.Fatal(err)
		}
	}
	insert("obj:before")
	tx, err := s.ReadSnapshotTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	insert("obj:after")
	got, err := s.ListObjectGraphFingerprintsTx(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, f := range got {
		ids[f.CanonicalID] = true
	}
	if !ids["obj:before"] || ids["obj:after"] {
		t.Fatalf("snapshot listing = %v, want obj:before only", ids)
	}
}

func trimTestFillRange(t *testing.T, s *Store, lo, hi uint64) {
	t.Helper()
	_, err := s.db.Exec(`WITH RECURSIVE seqs(n) AS (SELECT ? UNION ALL SELECT n+1 FROM seqs WHERE n < ?)
INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
SELECT 'digest-' || n, ?, n, 'p',
  CASE WHEN n % 5000 = 0 THEN 'other_kind_kept' ELSE ? END,
  'a', 'b', '[]', 'r', 'key-' || n, 'finalized', 'd', 'e', 'c', 'f', 1, 0, ? FROM seqs
WHERE n % 50 = 0 OR (n + 2) % ? < 5`,
		lo, hi, trimTestComputer, string(computerevent.EventProjectionBatchRecorded), time.Now().UTC(), projectionBatchTrimInterval)
	if err != nil {
		t.Fatal(err)
	}
}

func eventIndexDigest(t *testing.T, s *Store, head uint64, filtered bool) string {
	t.Helper()
	e := computerversion.SQLiteStateExtractor{
		DB: s.ReadDB(), Database: "texture",
		IgnoredContentColumns: map[string]map[string]struct{}{"computer_event_index": {"prepared_at": {}, "finalized_at": {}}},
	}
	if filtered {
		e.RowFilters = map[string]string{"computer_event_index": EventIndexWitnessFilter(ProjectionBatchTrimBoundary(head))}
	}
	d, err := e.Digests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return d.Tables["computer_event_index"]
}

// Failure 2 (cause 1 of the problem record): a base snapshot taken before the
// catch-up trim finished keeps the older backlog; the live store, whose
// catch-up finished, deleted it. After the tail passes the next trim points
// the two indexes differ only by rows at or below the trim boundary, which
// the trim defines as disposable, and the probe must not call that a
// difference, nor hide one above the boundary.
func TestProbeIndexDigestIgnoresTrimBacklogAndKeepsRowsAboveBoundary(t *testing.T) {
	ctx := context.Background()
	const base, head = 35000, 50000
	live, _ := trimTestOpen(t)
	snap, _ := trimTestOpen(t)
	for _, s := range []*Store{live, snap} {
		trimTestFill(t, s, base)
		trimTestHead(t, s, base)
	}
	// The live catch-up finished before the snapshot was captured; the base
	// snapshot was copied before it ran.
	if _, err := live.CatchUpProjectionBatchTrim(ctx); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*Store{live, snap} {
		trimTestFillRange(t, s, base+1, head)
		for seq := uint64(40000); seq <= head; seq += projectionBatchTrimInterval {
			tx, err := s.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			if err := s.trimProjectionBatchIndexTx(ctx, tx, trimTestComputer, seq); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
		trimTestHead(t, s, head)
	}
	if eventIndexDigest(t, live, head, false) == eventIndexDigest(t, snap, head, false) {
		t.Fatal("precondition: the unfiltered index digests should differ by the untrimmed backlog")
	}
	if a, b := eventIndexDigest(t, live, head, true), eventIndexDigest(t, snap, head, true); a != b {
		t.Fatalf("filtered digests differ by trim backlog alone: %s vs %s", a, b)
	}
	// A difference above the boundary is still a difference.
	if _, err := snap.db.Exec(`UPDATE computer_event_index SET event_pin_receipt_digest='zzz' WHERE sequence=45000`); err != nil {
		t.Fatal(err)
	}
	if a, b := eventIndexDigest(t, live, head, true), eventIndexDigest(t, snap, head, true); a == b {
		t.Fatal("the filter hid a difference in a row above the trim boundary")
	}
}
