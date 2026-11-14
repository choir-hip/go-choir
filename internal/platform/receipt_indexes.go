package platform

// Indexes of computer_event_append_receipts (docs/design/store-once-2026-11-14.md
// R4, docs/evidence/tape-write-census-2026-11-12.md section 5). The table holds
// one row per appended event and 99.3% of the paper's rows are
// projection_batch_recorded; no reader looks one up by kind. The kind index
// therefore covers every other kind only, and the table keeps its other
// indexes.
//
// Ways it could fail, written before the code (tests name these by number):
//
//  I1. a reader's query cannot use the partial index and walks the table: every
//      reader of event_kind spells the predicate out
//      (receiptKindNotBatchSQL) and TestReceiptKindReadersUseThePartialIndex
//      asserts, from EXPLAIN on a loaded table and for custom and generic
//      plans, that each uses an index and not a sequential scan;
//  I2. the index is built inside a transaction or without CONCURRENTLY and
//      blocks appends on a 19 GB table: Bootstrap does not create it at all
//      (the DDL no longer names it); ensureReceiptIndexes issues each statement
//      on one connection, outside any transaction, with CONCURRENTLY;
//  I3. an interrupted build leaves an INVALID index that IF NOT EXISTS then
//      treats as present: the validity is read from pg_index, an invalid one
//      is dropped and rebuilt;
//  I4. the old index is dropped before the new one is valid: it is dropped only
//      after the new one is read back valid;
//  I5. two corpusd processes build the same index: a session advisory lock
//      held on the connection; the one that does not get it returns, and the
//      next run of the migration finds the work done;
//  I6. a binary rolled back to one that names the old index re-creates it in
//      its boot DDL, non-concurrently, which blocks appends while it builds:
//      the rollback note in the commit says so;
//  I7. a reader of the table by event_head_receipt_id remains after its index
//      is dropped and walks 12M rows: TestNoReaderLooksUpAReceiptByID fails on
//      any non-test source that filters on the column, and
//      TestHeadReceiptLookupsUseTheSequenceIndex reads the plans of the
//      lookups that replaced them.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

const (
	receiptKindPartialIdx = "computer_event_append_kind_partial_idx"
	receiptKindOldIdx     = "computer_event_append_kind_seq_idx"
	// receiptIDOldIdx served the three lookups of the current head's receipt by
	// id (checkpoints.go), which now find the row by (computer, sequence).
	receiptIDOldIdx = "computer_event_append_receipt_id_idx"
	// receiptKindNotBatchSQL is the predicate of the partial index. A reader
	// that filters on event_kind adds it, spelled the same, so that the planner
	// can prove the index applies even for a generic plan, where the kind is a
	// parameter it cannot see. TestReceiptKindPredicateIsABatchKind pins the
	// literal to computerevent.EventProjectionBatchRecorded.
	receiptKindNotBatchSQL = `event_kind <> 'projection_batch_recorded'`

	// The queries of the readers that filter on event_kind. Each names the
	// predicate, and TestReceiptKindReadersUseThePartialIndex runs these very
	// strings through EXPLAIN.
	//
	// receiptKeyRevokedCountSQL: event_replay.go, the credential epoch of a replay page.
	receiptKeyRevokedCountSQL = `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=? AND sequence<=? AND event_kind=? AND ` + receiptKindNotBatchSQL
	// receiptFirstOfKindSQL: verifier_key_pin.go and checkpoints.go, the genesis event.
	receiptFirstOfKindSQL = `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence LIMIT 1`
	// receiptLatestAppliedSQL: clean_rebase.go, the latest apply or rollback event.
	receiptLatestAppliedSQL = `SELECT event_kind, event_digest FROM computer_event_append_receipts WHERE computer_id=? AND event_kind IN (?, ?, ?) AND ` + receiptKindNotBatchSQL + ` ORDER BY sequence DESC LIMIT 1`

	// The head receipt lookups of checkpoints.go: one row by the unique key
	// (computer, sequence).
	receiptAtSequenceSQL       = `SELECT ` + appendReceiptRowColumns + ` FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`
	receiptDigestAtSequenceSQL = `SELECT event_digest, event_head_receipt_id FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`

	// receiptIndexLockKey is the session advisory lock that serializes the
	// index migration across corpusd processes.
	receiptIndexLockKey = 7330212470801
)

// ensureReceiptIndexes builds the partial kind index and then drops the full
// one. Idempotent; safe to run beside appends (CONCURRENTLY, no transaction).
func (s *Store) ensureReceiptIndexes(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(?)`, receiptIndexLockKey).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		// The context may be done; the connection is closed (and the lock
		// with it) either way.
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, receiptIndexLockKey)
	}()
	if err := ensurePartialIndex(ctx, conn, receiptKindPartialIdx, `(computer_id, event_kind, sequence) WHERE `+receiptKindNotBatchSQL); err != nil {
		return err
	}
	if err := dropIndexConcurrently(ctx, conn, receiptKindOldIdx); err != nil {
		return err
	}
	return dropIndexConcurrently(ctx, conn, receiptIDOldIdx)
}

// indexState reports whether the named index exists and is valid.
func indexState(ctx context.Context, conn *sql.Conn, name string) (exists, valid bool, err error) {
	err = conn.QueryRowContext(ctx, `SELECT COALESCE(bool_or(true), false), COALESCE(bool_and(i.indisvalid AND i.indisready), false)
		FROM pg_class c JOIN pg_index i ON i.indexrelid = c.oid
		WHERE c.relname = ? AND c.relnamespace = current_schema()::regnamespace`, name).Scan(&exists, &valid)
	return
}

// ensurePartialIndex creates the index on the receipts table unless a valid one
// is there. An invalid leftover of an interrupted build is dropped first.
func ensurePartialIndex(ctx context.Context, conn *sql.Conn, name, definition string) error {
	exists, valid, err := indexState(ctx, conn, name)
	if err != nil {
		return err
	}
	if exists && valid {
		return nil
	}
	if exists {
		if _, err := conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+name); err != nil {
			return err
		}
	}
	start := time.Now()
	if _, err := conn.ExecContext(ctx, `CREATE INDEX CONCURRENTLY IF NOT EXISTS `+name+` ON computer_event_append_receipts `+definition); err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, valid, err := indexState(ctx, conn, name); err != nil || !valid {
		return fmt.Errorf("create %s: the index is not valid after the build (err=%v)", name, err)
	}
	log.Printf("corpusd receipt indexes: built %s ms=%d", name, time.Since(start).Milliseconds())
	return nil
}

func dropIndexConcurrently(ctx context.Context, conn *sql.Conn, name string) error {
	exists, _, err := indexState(ctx, conn, name)
	if err != nil || !exists {
		return err
	}
	start := time.Now()
	if _, err := conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+name); err != nil {
		return fmt.Errorf("drop %s: %w", name, err)
	}
	log.Printf("corpusd receipt indexes: dropped %s ms=%d", name, time.Since(start).Milliseconds())
	return nil
}
