package platform

// The queries of computer_event_append_receipts that choose its indexes
// (docs/design/store-once-2026-11-14.md R4,
// docs/evidence/tape-write-census-2026-11-12.md section 5).
//
// 99.3% of the paper's rows are projection_batch_recorded and no reader looks
// one up by kind, so the kind index is meant to cover every other kind only
// (a partial index, WHERE event_kind <> 'projection_batch_recorded'), and no
// reader looks a receipt up by its id. Every query here is written to be
// correct and index-backed whichever of the two kind indexes exists, so that
// the release that stops creating the old ones, the release that swaps them
// and a revert of either all run the same text:
//
//   - the old full index computer_event_append_kind_seq_idx
//     (computer_id, event_kind, sequence);
//   - the partial index computer_event_append_kind_partial_idx, the same
//     columns WHERE event_kind <> 'projection_batch_recorded'.
//
// Ways it could fail, written before the code (tests name these by number):
//
//  I1. a reader's query cannot use the partial index, or walks a chain: every
//      reader of event_kind spells the predicate out (receiptKindNotBatchSQL),
//      so the planner can prove the partial index applies even for a generic
//      plan, where the kind is a parameter it cannot see; the latest-applied
//      reader is three one-kind seeks, not an IN list ordered by sequence,
//      which a generic plan answers by walking the whole chain backward;
//      TestReceiptKindReadersUseAKindIndex asserts, from EXPLAIN on a table
//      with the paper's density of other kinds across two computers, that each
//      reader uses a kind index in custom and generic plans, with the old full
//      index alone and with the partial one alone;
//  I7. a reader of the table by event_head_receipt_id remains after the index
//      on it has gone: TestNoReaderLooksUpAReceiptByID fails on any source
//      (Go, shell, nix) that filters on the column, and
//      TestHeadReceiptLookupsUseTheSequenceIndex reads the plans of the
//      lookups that replaced them;
//  I6. a binary that still creates the old indexes at boot rebuilds them
//      without CONCURRENTLY, which blocks appends while it builds on a 30 GB
//      table. Bootstrap therefore creates neither; nothing in this release
//      creates or drops an index.

const (
	receiptKindPartialIdx = "computer_event_append_kind_partial_idx"
	receiptKindOldIdx     = "computer_event_append_kind_seq_idx"
	// receiptIDOldIdx served the lookups of the current head's receipt by id
	// (checkpoints.go), which find the row by (computer, sequence) now.
	receiptIDOldIdx = "computer_event_append_receipt_id_idx"

	// receiptKindNotBatchSQL is the predicate of the partial index; a reader
	// that filters on event_kind adds it, spelled the same.
	// TestReceiptKindPredicateIsABatchKind pins the literal to
	// computerevent.EventProjectionBatchRecorded.
	receiptKindNotBatchSQL = `event_kind <> 'projection_batch_recorded'`

	// receiptKeyRevokedCountSQL: event_replay.go, the credential epoch of a
	// replay page.
	receiptKeyRevokedCountSQL = `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=? AND sequence<=? AND event_kind=? AND ` + receiptKindNotBatchSQL
	// receiptFirstOfKindSQL: verifier_key_pin.go and checkpoints.go, the
	// genesis event.
	receiptFirstOfKindSQL = `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence LIMIT 1`
	// receiptLatestAppliedSQL: clean_rebase.go, the latest of three kinds. One
	// seek of the kind index per kind, the newest of the three outside; the
	// arguments are (computer, kind) three times.
	receiptLatestAppliedSQL = `SELECT event_kind, event_digest FROM (` +
		`(SELECT event_kind, event_digest, sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence DESC LIMIT 1) UNION ALL ` +
		`(SELECT event_kind, event_digest, sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence DESC LIMIT 1) UNION ALL ` +
		`(SELECT event_kind, event_digest, sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence DESC LIMIT 1)` +
		`) newest ORDER BY sequence DESC LIMIT 1`

	// The head receipt lookups of checkpoints.go: one row by the unique key
	// (computer, sequence).
	receiptAtSequenceSQL       = `SELECT ` + appendReceiptRowColumns + ` FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`
	receiptDigestAtSequenceSQL = `SELECT event_digest, event_head_receipt_id FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`
)
