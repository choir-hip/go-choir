package store

import (
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The content witness reads computer_event_index without its timestamp
// columns (a replay writes fresh wall-clock values).
const (
	eventIndexPreparedAtColumn  = "prepared_at"
	eventIndexFinalizedAtColumn = "finalized_at"
	// EventIndexPinListColumn is the JSON-array column whose null and "[]"
	// are one empty list.
	EventIndexPinListColumn = "payload_pin_receipt_digests_json"
)

// EventIndexIgnoredColumns are the columns of computer_event_index left out
// of the witness: the one list the extractor and the row fingerprints share.
func EventIndexIgnoredColumns() map[string]struct{} {
	return map[string]struct{}{eventIndexPreparedAtColumn: {}, eventIndexFinalizedAtColumn: {}}
}

// ProjectionBatchTrimBoundary is the highest sequence whose finalized
// projection_batch_recorded row the trim at head is defined to delete.
func ProjectionBatchTrimBoundary(head uint64) uint64 { return projectionBatchTrimBoundary(head) }

// EventIndexWitnessFilter is the SQL predicate that keeps the rows of
// computer_event_index the replay probe compares at a pinned head whose trim
// boundary is boundary: every row except a finalized projection_batch_recorded
// row at or below it. The trim deletes those rows incrementally (the last
// slice, at each trim point) and by the catch-up trim (the backlog, from two
// minutes after an open), so whether a given store has deleted them yet is a
// matter of when its catch-up ran, not of its history. Rows above the
// boundary, and every other kind and status, stay in the comparison. Empty
// when nothing is deletable yet.
func EventIndexWitnessFilter(boundary uint64) string {
	if boundary == 0 {
		return ""
	}
	return fmt.Sprintf("NOT (event_kind = '%s' AND status = 'finalized' AND sequence <= %d)",
		computerevent.EventProjectionBatchRecorded, boundary)
}
