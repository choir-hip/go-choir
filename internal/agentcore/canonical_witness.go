package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The canonical witness (witness version 4) is the stored witness's extractor
// (replayStateExtractor) with the event index read the way a live store and a
// replay of its tape can agree on, for a checkpoint at a head whose trim
// boundary is boundary:
//
//   - A finalized projection_batch_recorded row at or below the boundary is
//     left out. The trim defines those rows as disposable and nothing reads
//     them (event_index_trim.go): replay and restore read the host tape, head
//     and continuity come from computer_event_projection_heads. A live store
//     deletes them in two steps at two times (the last slice at each trim
//     point, the older backlog in a catch-up two minutes after an open); a base
//     snapshot copied before a catch-up finished carries the backlog, and a
//     staged store's own catch-up may or may not have run when the witness is
//     read. Rows above the boundary and every other kind and status stay in
//     the digest, so a real loss or addition is still refused.
//   - A null in payload_pin_receipt_digests_json is the empty list "[]". A
//     replay by an earlier build wrote null where the live append wrote "[]"
//     (the cold-recovery path, autoputer/projection_base.go, replays a tail
//     with no witness gate and was the likely producer); such rows live on in
//     stores and in base snapshots.
//
// The replay-completeness probe uses this same extractor on both sides, so
// "equivalent" and "the checkpoint's witness matches" are one test.
//
// The strict form (version 3) stays for checkpoints bound before it and for
// snapshots: a version 1 snapshot record carries a witness of its raw pages,
// which VerifySnapshotFile recomputes strictly (LiveStoreWitness); a version 2
// record carries a page digest instead and no witness.
func canonicalWitnessExtractor(db *sql.DB, boundary uint64) computerversion.SQLiteStateExtractor {
	e := replayStateExtractor(db)
	e.EmptyJSONArrayColumns = map[string]map[string]struct{}{
		"computer_event_index": {choirstore.EventIndexPinListColumn: {}},
	}
	e.RowFilters = map[string]string{}
	if filter := choirstore.EventIndexWitnessFilter(boundary); filter != "" {
		e.RowFilters["computer_event_index"] = filter
	}
	return e
}

// canonicalSnapshotDigests extracts the canonical digests of db in one read
// snapshot. hook runs first inside it and returns the sequence of the head the
// snapshot names (a bind and the probe read the live head there, under the
// append hold); the trim boundary of that sequence then sets the event index
// filter, and the single-computer invariant of the filter is checked. It
// returns the digests and the boundary they were taken at.
func canonicalSnapshotDigests(ctx context.Context, db *sql.DB, hook func(ctx context.Context, tx *sql.Tx) (uint64, error)) (computerversion.StateDigests, uint64, error) {
	var empty computerversion.StateDigests
	if db == nil {
		return empty, 0, errors.New("self-development checkpoint: store is not open")
	}
	var boundary uint64
	e := canonicalWitnessExtractor(db, 0)
	e.SnapshotHook = func(ctx context.Context, tx *sql.Tx) error {
		sequence, err := hook(ctx, tx)
		if err != nil {
			return err
		}
		boundary = choirstore.ProjectionBatchTrimBoundary(sequence)
		if filter := choirstore.EventIndexWitnessFilter(boundary); filter != "" {
			// The map is shared with the copy Digests makes.
			e.RowFilters["computer_event_index"] = filter
		}
		return choirstore.CheckEventIndexSingleComputerTx(ctx, tx)
	}
	digests, err := e.Digests(ctx)
	if err != nil {
		return empty, 0, err
	}
	return digests, boundary, nil
}

// reconstructedWitness is the witness of a reconstructed store in the form the
// checkpoint's witness names: version 3 is read strictly, as it always was;
// version 4 in the canonical form at the boundary the checkpoint's own head
// sequence defines. A boundary that is not that head's is refused: the
// checkpoint commits to it, and a larger one would hide more rows.
func reconstructedWitness(ctx context.Context, db *sql.DB, want selfdevprotocol.VMLocalContentWitness, targetSequence uint64) (selfdevprotocol.VMLocalContentWitness, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	switch want.WitnessVersion {
	case selfdevprotocol.WitnessVersionSQLite:
		return storeWitness(ctx, db, replayStateExtractor(db))
	case selfdevprotocol.WitnessVersionCanonical:
		if expected := choirstore.ProjectionBatchTrimBoundary(targetSequence); want.EventIndexTrimBoundary != expected {
			return empty, fmt.Errorf("%w: rematerialize: checkpoint event index boundary %d is not the trim boundary %d of its head sequence %d",
				selfdevprotocol.ErrWitnessMismatch, want.EventIndexTrimBoundary, expected, targetSequence)
		}
		digests, boundary, err := canonicalSnapshotDigests(ctx, db, func(context.Context, *sql.Tx) (uint64, error) { return targetSequence, nil })
		if err != nil {
			return empty, err
		}
		return selfdevprotocol.WitnessFromDigestsCanonical(digests, boundary)
	}
	return empty, selfdevprotocol.RefuseWitnessVersion(want.WitnessVersion)
}
