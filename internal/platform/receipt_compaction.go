package platform

// Append-receipt compaction (docs/design/append-receipt-trim-2026-10-31.md).
// computer_event_append_receipts is the host's sequence index of every
// computer's tape: replay, restore's tape scan, the tape sweep and bundle
// export all walk it gapless from sequence 1, so no row is unread and none is
// deleted here. What is reduced is the stored body: event_head_receipt_json
// is about 1.5 KB of canonical JSON a row, two thirds of it a respelling of
// the row's own columns. For the computers PLATFORM_RECEIPT_COMPACT_COMPUTERS
// names, it is stored in the lossless compact form
// (computerevent.CompactHeadReceipt), at the append and, for older rows, by a
// paced backfill. Every reader expands through this file.
//
// Ways it could fail, written before the code (the computerevent codec has
// its own list; tests name these by number):
//
//  C1. a reader reads the column raw and misreads a compact value: every
//      reader selects appendReceiptRowColumns and calls expandAppendReceipt;
//      TestAppendReceiptJSONReadersExpand fails on a raw reader;
//  C2. the backfill rewrites a row it then cannot expand: the update is a
//      compare-and-set on the old body, and the batch re-reads what it wrote
//      and expands it against the original bytes before it commits;
//  C3. a dry run writes: it never runs an UPDATE;
//  C4. one batch holds a long transaction or much memory: a batch is
//      receiptCompactionBatch rows in one short transaction, with a rest of
//      at least a second and four times the batch's own time after it;
//  C5. a corpusd restart rescans the whole paper chain: progress is durable
//      per computer and mode (computer_event_receipt_compaction_v2);
//  C6. rollback to a binary that cannot expand: mode expand rewrites the
//      named computers' compact rows back to full JSON, and stops writing
//      compact; run it to completion before reverting;
//  C7. a selector typo compacts another computer: only exact ids listed are
//      touched, and the parsed set is logged at start;
//  C8. an idempotent retry, a replay page, a sweep walk, a checkpoint or a
//      bundle reads a compact row differently from the full one: each is
//      tested against the full-JSON baseline;
//  C9. a rewrite that is not lossless leaves the tape sweep's stored
//      reference chunks (tape_sweep_ref_chunks, tape_sweep_refs.go) stale: a
//      chunk's anchor is the event digest, which does not commit to the
//      receipt body, so the sweep cannot see an edited pin. This pass is
//      lossless (C2), so the chunks stay valid
//      (TestTapeSweepWatermarkSurvivesReceiptCompaction). Any other writer
//      that changes or deletes a receipt body must delete the computer's
//      chunks and be added to TestAppendReceiptRowWritersAreKnown.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// EnvReceiptCompactComputers selects the computers and their mode:
// "id=active,id=dry-run,id=expand" (a bare id is active).
const EnvReceiptCompactComputers = "PLATFORM_RECEIPT_COMPACT_COMPUTERS"

const (
	ReceiptCompactActive = "active"
	ReceiptCompactDryRun = "dry-run"
	ReceiptCompactExpand = "expand"
)

// appendReceiptRowColumns are the columns a reader of the stored receipt
// selects, in scanAppendReceipt's order.
const appendReceiptRowColumns = `computer_id, sequence, event_kind, event_digest, request_commitment, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json`

const receiptCompactionDDL = `
CREATE TABLE IF NOT EXISTS computer_event_receipt_compaction (
  computer_id VARCHAR(128) NOT NULL,
  mode VARCHAR(16) NOT NULL,
  through_sequence BIGINT NOT NULL,
  rows_rewritten BIGINT NOT NULL,
  rows_kept BIGINT NOT NULL,
  bytes_before BIGINT NOT NULL,
  bytes_after BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id)
);
-- The codec-2 walk's own progress. A release that does not know it (step 1, or
-- main) writes only computer_event_receipt_compaction, so it cannot move this
-- one past rows it did not compact.
CREATE TABLE IF NOT EXISTS computer_event_receipt_compaction_v2 (
  computer_id VARCHAR(128) NOT NULL,
  mode VARCHAR(16) NOT NULL,
  through_sequence BIGINT NOT NULL,
  rows_rewritten BIGINT NOT NULL,
  rows_kept BIGINT NOT NULL,
  bytes_before BIGINT NOT NULL,
  bytes_after BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id)
);`

var (
	receiptCompactionBatch    = 500
	receiptCompactionPause    = time.Second
	receiptCompactionRest     = 4.0
	receiptCompactionInterval = 10 * time.Minute
	receiptCompactionDelay    = 2 * time.Minute
	// receiptCompactionLoadEvery is how many batches pass between two load
	// lines in the log (first batch included), so the first hours of a walk
	// over the live table can be read from the log.
	receiptCompactionLoadEvery = 100
)

// ReceiptCompactionPolicy maps each named computer to its mode.
type ReceiptCompactionPolicy struct {
	Computers map[string]string
}

// ParseReceiptCompactComputers reads EnvReceiptCompactComputers. Unknown
// modes and malformed ids are refused, not guessed.
func ParseReceiptCompactComputers(raw string) (ReceiptCompactionPolicy, error) {
	policy := ReceiptCompactionPolicy{Computers: map[string]string{}}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		id, mode, found := strings.Cut(item, "=")
		id, mode = strings.TrimSpace(id), strings.TrimSpace(mode)
		if !found {
			mode = ReceiptCompactActive
		}
		if !strings.HasPrefix(id, "computer-") || strings.ContainsAny(id, " \t=") {
			return ReceiptCompactionPolicy{}, fmt.Errorf("%s: %q is not a computer id", EnvReceiptCompactComputers, id)
		}
		switch mode {
		case ReceiptCompactActive, ReceiptCompactDryRun, ReceiptCompactExpand:
		default:
			return ReceiptCompactionPolicy{}, fmt.Errorf("%s: %q has mode %q, not active|dry-run|expand", EnvReceiptCompactComputers, id, mode)
		}
		policy.Computers[id] = mode
	}
	return policy, nil
}

func (p ReceiptCompactionPolicy) String() string {
	if len(p.Computers) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(p.Computers))
	for id := range p.Computers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id + "=" + p.Computers[id]
	}
	return strings.Join(parts, ",")
}

// SetReceiptCompaction installs the policy the append path and the backfill
// read.
func (s *Store) SetReceiptCompaction(policy ReceiptCompactionPolicy) {
	if s == nil {
		return
	}
	copied := ReceiptCompactionPolicy{Computers: make(map[string]string, len(policy.Computers))}
	for id, mode := range policy.Computers {
		copied.Computers[id] = mode
	}
	s.receiptCompaction.Store(&copied)
}

func (s *Store) receiptCompactionMode(computerID string) string {
	if s == nil {
		return ""
	}
	p := s.receiptCompaction.Load()
	if p == nil {
		return ""
	}
	return p.Computers[computerID]
}

// storedAppendReceipt is the body the append writes: compact for an active
// computer when the round trip holds, the full JSON otherwise.
func (s *Store) storedAppendReceipt(row computerevent.HeadReceiptRow, receiptJSON []byte) string {
	if s.receiptCompactionMode(row.ComputerID) == ReceiptCompactActive {
		if compact, ok := computerevent.CompactHeadReceipt(row, receiptJSON); ok {
			return compact
		}
	}
	return string(receiptJSON)
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanAppendReceipt scans appendReceiptRowColumns and returns the row and
// its stored body.
func scanAppendReceipt(scanner rowScanner) (computerevent.HeadReceiptRow, string, error) {
	var row computerevent.HeadReceiptRow
	var stored string
	err := scanner.Scan(&row.ComputerID, &row.Sequence, &row.EventKind, &row.EventDigest, &row.RequestCommitment, &row.EventPinReceiptDigest, &row.ReceiptID, &stored)
	return row, stored, err
}

// expandAppendReceipt is the one way a reader gets a stored receipt's
// canonical JSON.
func expandAppendReceipt(row computerevent.HeadReceiptRow, stored string) ([]byte, error) {
	return computerevent.ExpandHeadReceipt(row, stored)
}

// scanExpandedAppendReceipt scans a row and expands its receipt.
func scanExpandedAppendReceipt(scanner rowScanner) (computerevent.HeadReceiptRow, []byte, error) {
	row, stored, err := scanAppendReceipt(scanner)
	if err != nil {
		return row, nil, err
	}
	raw, err := expandAppendReceipt(row, stored)
	return row, raw, err
}

// ReceiptCompactionPass is one pass's counts for one computer.
type ReceiptCompactionPass struct {
	ComputerID  string `json:"computer_id"`
	Mode        string `json:"mode"`
	Through     uint64 `json:"through_sequence"`
	Scanned     int64  `json:"scanned"`
	Rewritten   int64  `json:"rewritten"`
	Kept        int64  `json:"kept"`
	BytesBefore int64  `json:"bytes_before"`
	BytesAfter  int64  `json:"bytes_after"`
	Done        bool   `json:"done"`
}

// ReceiptCompactor runs the paced backfill.
type ReceiptCompactor struct {
	store *Store
	sleep func(context.Context, time.Duration) error
	// allowJSONB is the write allowance for format 2, the jsonb spelling of the
	// rows stored while the column was jsonb
	// (docs/problems/old-receipts-do-not-compact-2026-11-14.md). It is true;
	// a test sets it false to see the path of the release that only reads
	// format 2, where a compact row the compactor may not write is kept.
	allowJSONB bool
	// freeSpace reads the free bytes of the volumes the walk writes to.
	freeSpace func(context.Context) (free uint64, from string, known bool)
	// buildRunning reports a maintenance lock on the receipt table
	// (receipt_walk.go).
	buildRunning func(context.Context) (bool, error)
	// lastDeferral is the last reason noteDeferral logged ("" when running).
	lastDeferral string
	// buildCheckFailed is set while the build check cannot be read.
	buildCheckFailed bool
}

func NewReceiptCompactor(store *Store) *ReceiptCompactor {
	c := &ReceiptCompactor{store: store, sleep: sleepContext, allowJSONB: true}
	c.freeSpace = store.receiptWalkFreeSpace
	c.buildRunning = store.receiptBuildRunning
	return c
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Start runs passes every receiptCompactionInterval after a boot delay, in
// one goroutine, until ctx ends.
func (c *ReceiptCompactor) Start(ctx context.Context) {
	if c == nil || c.store == nil {
		return
	}
	go func() {
		if c.sleep(ctx, receiptCompactionDelay) != nil {
			return
		}
		for {
			why := c.deferWalk(ctx, true)
			c.noteDeferral(why)
			if why == "" {
				if _, err := c.RunOnce(ctx); err != nil && ctx.Err() == nil {
					log.Printf("corpusd: receipt compaction: %v", err)
				}
			}
			if c.sleep(ctx, receiptCompactionInterval) != nil {
				return
			}
		}
	}()
}

// RunOnce runs one pass over every named computer, each up to its head at
// the pass start.
func (c *ReceiptCompactor) RunOnce(ctx context.Context) ([]ReceiptCompactionPass, error) {
	p := c.store.receiptCompaction.Load()
	if p == nil || len(p.Computers) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(p.Computers))
	for id := range p.Computers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var passes []ReceiptCompactionPass
	var errs []error
	for _, id := range ids {
		pass, err := c.runComputer(ctx, id, p.Computers[id])
		if pass.Scanned > 0 || err != nil {
			log.Printf("corpusd: receipt compaction %s mode=%s through=%d scanned=%d rewritten=%d kept=%d bytes_before=%d bytes_after=%d done=%v err=%v",
				id, pass.Mode, pass.Through, pass.Scanned, pass.Rewritten, pass.Kept, pass.BytesBefore, pass.BytesAfter, pass.Done, err)
		}
		passes = append(passes, pass)
		if err != nil {
			if ctx.Err() != nil {
				return passes, ctx.Err()
			}
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return passes, errors.Join(errs...)
}

func (c *ReceiptCompactor) runComputer(ctx context.Context, computerID, mode string) (ReceiptCompactionPass, error) {
	db := c.store.db
	pass := ReceiptCompactionPass{ComputerID: computerID, Mode: mode}
	var storedMode string
	var storedAt time.Time
	var cum ReceiptCompactionPass
	err := db.QueryRowContext(ctx, `SELECT mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at FROM computer_event_receipt_compaction_v2 WHERE computer_id=?`, computerID).
		Scan(&storedMode, &cum.Through, &cum.Rewritten, &cum.Kept, &cum.BytesBefore, &cum.BytesAfter, &storedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return pass, err
	}
	if storedMode != mode {
		// A new mode starts its walk over: what an earlier mode did proves
		// nothing about this one.
		cum = ReceiptCompactionPass{}
	}
	// This walk's progress is its own row, which a release that does not know
	// it never writes. Such a release (step 1) walks the same rows under its own
	// row and may have done anything to them meanwhile: kept the jsonb-spelled
	// rows, or expanded compact ones. Any row of the older progress table that is
	// newer than this walk's own, in any mode, restarts this walk from zero: the
	// restart costs a pass over rows that are already compact (they are kept),
	// and the rule needs no reading of what the older release did.
	var legacyAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM computer_event_receipt_compaction WHERE computer_id=?`, computerID).Scan(&legacyAt); err == nil {
		if legacyAt.After(storedAt) {
			cum = ReceiptCompactionPass{}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return pass, err
	}
	head, err := readComputerEventHead(ctx, db, computerID, false)
	if err != nil {
		return pass, err
	}
	if head != nil && cum.Through > head.Sequence {
		// A stale row: the computer was purged and created again under the same
		// id (the purge of an older release does not know this table), and its
		// tape is shorter than the progress says. Walk it from zero.
		cum = ReceiptCompactionPass{}
	}
	pass.Through = cum.Through
	if head == nil || cum.Through >= head.Sequence {
		pass.Done = true
		return pass, nil
	}
	if cum.Through == 0 && !storedAt.IsZero() {
		// A restart: the row is written back to zero here, once and on purpose,
		// so that the saves of the pass can be held to never move it backward.
		if err := c.resetProgress(ctx, computerID, mode); err != nil {
			return pass, err
		}
	}
	walkStart := time.Now()
	startLSN, _ := c.walLSN(ctx)
	batches := 0
	for pass.Through < head.Sequence {
		started := time.Now()
		batch, err := c.batch(ctx, computerID, mode, pass.Through, head.Sequence)
		if err != nil {
			return pass, err
		}
		pass.Scanned += batch.Scanned
		pass.Rewritten += batch.Rewritten
		pass.Kept += batch.Kept
		pass.BytesBefore += batch.BytesBefore
		pass.BytesAfter += batch.BytesAfter
		pass.Through = batch.Through
		if err := c.saveProgress(ctx, computerID, mode, cum, pass); err != nil {
			return pass, err
		}
		if batches%receiptCompactionLoadEvery == 0 {
			c.logLoad(ctx, computerID, pass, startLSN, walkStart)
		}
		batches++
		if batch.Done {
			break
		}
		if why := c.deferWalk(ctx, batches%receiptCompactionLoadEvery == 0); why != "" {
			// Progress is saved; the next pass resumes from it.
			c.noteDeferral(why)
			return pass, nil
		}
		c.noteDeferral("")
		rest := max(receiptCompactionPause, time.Duration(receiptCompactionRest*float64(time.Since(started))))
		if err := c.sleep(ctx, rest); err != nil {
			return pass, err
		}
	}
	pass.Done = true
	return pass, nil
}

// resetProgress writes the walk's row back to zero for the mode, unconditionally.
func (c *ReceiptCompactor) resetProgress(ctx context.Context, computerID, mode string) error {
	_, err := c.store.db.ExecContext(ctx, `INSERT INTO computer_event_receipt_compaction_v2 (computer_id, mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at) VALUES (?,?,0,0,0,0,0,?)
		ON CONFLICT (computer_id) DO UPDATE SET mode=excluded.mode, through_sequence=0, rows_rewritten=0, rows_kept=0, bytes_before=0, bytes_after=0, updated_at=excluded.updated_at`,
		computerID, mode, time.Now().UTC())
	return err
}

// saveProgress records a batch. It never moves the row's through_sequence
// backward in the same mode (a second process, or a pass that began before
// another one saved further, would otherwise undo it); a restart goes through
// resetProgress.
func (c *ReceiptCompactor) saveProgress(ctx context.Context, computerID, mode string, cum, pass ReceiptCompactionPass) error {
	_, err := c.store.db.ExecContext(ctx, `INSERT INTO computer_event_receipt_compaction_v2 (computer_id, mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at) VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT (computer_id) DO UPDATE SET mode=excluded.mode, through_sequence=excluded.through_sequence, rows_rewritten=excluded.rows_rewritten, rows_kept=excluded.rows_kept, bytes_before=excluded.bytes_before, bytes_after=excluded.bytes_after, updated_at=excluded.updated_at
		WHERE computer_event_receipt_compaction_v2.mode <> excluded.mode OR computer_event_receipt_compaction_v2.through_sequence <= excluded.through_sequence`,
		computerID, mode, pass.Through, cum.Rewritten+pass.Rewritten, cum.Kept+pass.Kept, cum.BytesBefore+pass.BytesBefore, cum.BytesAfter+pass.BytesAfter, time.Now().UTC())
	return err
}

type compactionUpdate struct {
	row      computerevent.HeadReceiptRow
	old      string
	new      string
	original []byte
}

// batch handles the rows after through, up to head, one batch. Rows already
// in the target form are only counted.
func (c *ReceiptCompactor) batch(ctx context.Context, computerID, mode string, through, head uint64) (ReceiptCompactionPass, error) {
	out := ReceiptCompactionPass{Through: through}
	tx, err := c.store.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+appendReceiptRowColumns+` FROM computer_event_append_receipts WHERE computer_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`, computerID, through, head, receiptCompactionBatch)
	if err != nil {
		return out, err
	}
	var updates []compactionUpdate
	n := 0
	for rows.Next() {
		row, stored, err := scanAppendReceipt(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		n++
		out.Scanned++
		out.Through = row.Sequence
		original, err := expandAppendReceipt(row, stored)
		if err != nil {
			rows.Close()
			return out, fmt.Errorf("sequence %d: %w", row.Sequence, err)
		}
		target := string(original)
		if mode != ReceiptCompactExpand {
			if compact, ok := computerevent.CompactStoredHeadReceipt(row, original, c.allowJSONB); ok {
				target = compact
			} else if computerevent.IsCompactHeadReceipt(stored) {
				// A compact row this codec may not write (format 2 under
				// codec 1) is kept, not expanded back to full text.
				target = stored
			}
		}
		out.BytesBefore += int64(len(stored))
		out.BytesAfter += int64(len(target))
		if target == stored {
			out.Kept++
			continue
		}
		out.Rewritten++
		updates = append(updates, compactionUpdate{row: row, old: stored, new: target, original: original})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	out.Done = n < receiptCompactionBatch || out.Through >= head
	if mode == ReceiptCompactDryRun || len(updates) == 0 {
		return out, nil
	}
	for _, u := range updates {
		res, err := tx.ExecContext(ctx, `UPDATE computer_event_append_receipts SET event_head_receipt_json=? WHERE computer_id=? AND sequence=? AND event_head_receipt_json=?`, u.new, computerID, u.row.Sequence, u.old)
		if err != nil {
			return out, err
		}
		if affected, err := res.RowsAffected(); err != nil || affected != 1 {
			return out, fmt.Errorf("sequence %d: the row changed under the rewrite", u.row.Sequence)
		}
	}
	// Read back what this transaction wrote and expand it against the
	// original bytes before anything commits.
	for _, u := range updates {
		row, raw, err := scanExpandedAppendReceipt(tx.QueryRowContext(ctx, `SELECT `+appendReceiptRowColumns+` FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`, computerID, u.row.Sequence))
		if err != nil {
			return out, fmt.Errorf("sequence %d: verify: %w", u.row.Sequence, err)
		}
		if row != u.row || !bytes.Equal(raw, u.original) {
			return out, fmt.Errorf("sequence %d: verify: the rewritten receipt does not expand to the original", u.row.Sequence)
		}
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}
