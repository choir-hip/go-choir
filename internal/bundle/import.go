package bundle

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// ImportResult says what an import changed.
type ImportResult struct {
	ComputerID string `json:"computer_id"`
	// HeadBefore is the target's head sequence before the import, -1 when it
	// had none.
	HeadBefore           int64 `json:"head_before"`
	HeadAfter            int64 `json:"head_after"`
	ReceiptsInserted     int64 `json:"receipts_inserted"`
	FilesWritten         int64 `json:"files_written"`
	SnapshotRootInserted bool  `json:"snapshot_root_inserted"`
}

const insertBatch = 500

// Import writes the verified bundle in dir into a platform store: its files
// under artifactsRoot (chunks in the file CAS layout, tape objects in the
// legacy layout the platform's tape reader serves) and its rows into the
// control database db (opened through dbx.OpenPostgres, schema bootstrapped).
//
// It is idempotent. It refuses (ErrRefused, the target unchanged) when the
// target's head for the computer does not chain: a head behind the range's
// start (a gap), a head at the start with another digest, a head inside or
// past the range with another digest than the bundle's at that sequence. A
// fork is a new computer, never a merge.
//
// Files land before rows, so a row never names a missing file; a crash
// between them leaves only unreferenced content-addressed files. The rows
// land in one serializable transaction that re-reads the head under lock.
// Run it with the target's corpusd stopped, or at least the computer idle:
// a racing append makes the import refuse, never interleave.
func Import(ctx context.Context, dir string, db *sql.DB, artifactsRoot string) (ImportResult, error) {
	if db == nil || artifactsRoot == "" {
		return ImportResult{}, fmt.Errorf("bundle: target database and artifacts root are required")
	}
	m, err := Verify(dir)
	if err != nil {
		return ImportResult{}, err
	}
	id := m.ComputerID
	res := ImportResult{ComputerID: id, HeadBefore: -1}
	if dead, err := tombstoned(ctx, db, id); err != nil {
		return res, err
	} else if dead {
		return res, refusef("computer %s tape is tombstoned on the target", id)
	}
	head, err := readHead(ctx, db, id, false)
	if err != nil {
		return res, err
	}
	plan, err := planImport(ctx, db, dir, m, head)
	if err != nil {
		return res, err
	}
	if head != nil {
		res.HeadBefore = int64(head.Sequence)
	}
	if err := checkSnapshotRoot(ctx, db, m); err != nil {
		return res, err
	}
	written, err := importFiles(dir, artifactsRoot, m)
	if err != nil {
		return res, err
	}
	res.FilesWritten = written
	rows, err := dbx.Retry(ctx, 5, func() (ImportResult, error) {
		return importRows(ctx, db, dir, m, head, plan)
	})
	if err != nil {
		return res, err
	}
	res.ReceiptsInserted, res.SnapshotRootInserted, res.HeadAfter = rows.ReceiptsInserted, rows.SnapshotRootInserted, rows.HeadAfter
	return res, nil
}

// importPlan: receipts with sequence > insertAfter are inserted; advance says
// the head row moves to the bundle's head.
type importPlan struct {
	insertAfter int64
	advance     bool
}

func planImport(ctx context.Context, db *sql.DB, dir string, m Manifest, head *computerevent.Head) (importPlan, error) {
	start, end := m.Tape.StartSeq, m.Tape.HeadSeq
	if head == nil {
		var n int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, m.ComputerID).Scan(&n); err != nil {
			return importPlan{}, fmt.Errorf("bundle: count target receipts: %w", err)
		}
		if n > 0 {
			return importPlan{}, refusef("target holds %d receipts of %s but no head", n, m.ComputerID)
		}
		if m.SnapshotRoot != nil && start > m.SnapshotRoot.HeadSequence {
			return importPlan{}, refusef("range starts at %d, after its snapshot at %d: an empty target has nothing to chain it to", start, m.SnapshotRoot.HeadSequence)
		}
		return importPlan{insertAfter: start, advance: true}, nil
	}
	t := int64(head.Sequence)
	switch {
	case t < start:
		return importPlan{}, refusef("target head %d is behind the range start %d (a gap)", t, start)
	case t == start:
		if head.CanonicalEventHead != m.Tape.StartHead {
			return importPlan{}, refusef("target head at %d is %s, the bundle chains from %s (a fork)", t, head.CanonicalEventHead, m.Tape.StartHead)
		}
	case t <= end:
		want, err := bundleDigests(dir, start, t)
		if err != nil {
			return importPlan{}, err
		}
		if want[t] != head.CanonicalEventHead {
			return importPlan{}, refusef("target head at %d is %s, the bundle has %s there", t, head.CanonicalEventHead, want[t])
		}
		// Every target receipt the range overlaps must be the bundle's.
		rows, err := readReceipts(ctx, db, m.ComputerID, start, t, int(t-start)+1)
		if err != nil {
			return importPlan{}, err
		}
		for _, row := range rows {
			if want[row.Sequence] != row.EventDigest {
				return importPlan{}, refusef("target event at %d is %s, the bundle has %s", row.Sequence, row.EventDigest, want[row.Sequence])
			}
		}
	default:
		got, err := receiptDigestAt(ctx, db, m.ComputerID, end)
		if err != nil {
			return importPlan{}, err
		}
		if got != m.Tape.HeadDigest {
			return importPlan{}, refusef("target is past the bundle head %d and its event there is %q, not %s", end, got, m.Tape.HeadDigest)
		}
		return importPlan{insertAfter: end}, nil
	}
	return importPlan{insertAfter: t, advance: end > t}, nil
}

// bundleDigests maps sequence to event digest for from < sequence <= to.
// The overlap of a target head inside the range is bounded by the range.
func bundleDigests(dir string, from, to int64) (map[int64]string, error) {
	out := map[int64]string{}
	err := forEachReceipt(dir, func(row ReceiptRow) (bool, error) {
		if row.Sequence > to {
			return false, nil
		}
		if row.Sequence > from {
			out[row.Sequence] = row.EventDigest
		}
		return true, nil
	})
	return out, err
}

func forEachReceipt(dir string, visit func(ReceiptRow) (bool, error)) error {
	f, err := os.Open(filepath.Join(dir, ReceiptsName))
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), maxReceiptLine)
	for scanner.Scan() {
		var row ReceiptRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return fmt.Errorf("bundle: receipt line: %w", err)
		}
		more, err := visit(row)
		if err != nil || !more {
			return err
		}
	}
	return scanner.Err()
}

// checkSnapshotRoot refuses a target that holds the same root with another
// declaration.
func checkSnapshotRoot(ctx context.Context, db *sql.DB, m Manifest) error {
	if m.SnapshotRoot == nil {
		return nil
	}
	existing, err := readSnapshotRoot(ctx, db, m.ComputerID, m.SnapshotRoot.Root)
	if err != nil || existing == nil {
		return err
	}
	if existing.HeadSequence != m.SnapshotRoot.HeadSequence || strings.Join(existing.Chunks, ",") != strings.Join(m.SnapshotRoot.Chunks, ",") {
		return refusef("target declares snapshot root %s with another head or chunk list", m.SnapshotRoot.Root)
	}
	return nil
}

// importFiles writes every content-addressed file of the bundle the target
// does not already hold. Tape objects the target's reader already serves
// (from a segment or a legacy file) are left alone.
func importFiles(dir, root string, m Manifest) (int64, error) {
	var written int64
	if m.SnapshotRoot != nil {
		for _, digest := range m.SnapshotRoot.Chunks {
			dst := hostChunkPath(root, m.ComputerID, digest)
			ok, _, err := presentVerified(dst, digest)
			if err != nil {
				return written, err
			}
			if ok {
				continue
			}
			if _, err := copyVerified(filepath.Join(dir, ChunksDir, digest), dst, digest, 0o640); err != nil {
				return written, err
			}
			written++
		}
	}
	reader := tapestore.NewReader(root)
	for _, kind := range []struct {
		sub string
		ns  tapestore.NS
	}{{EventsDir, tapestore.NSEvent}, {PayloadsDir, tapestore.NSPayload}, {PinsDir, tapestore.NSReceipt}} {
		entries, err := os.ReadDir(filepath.Join(dir, kind.sub))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return written, err
		}
		for _, entry := range entries {
			digest := entry.Name()
			if !entry.Type().IsRegular() || !validDigest(digest) {
				continue // temp files of an interrupted export
			}
			if _, _, err := reader.Read(m.ComputerID, kind.ns, digest); err == nil {
				continue
			}
			if kind.ns == tapestore.NSPayload {
				// The platform's payload fetch also probes the event namespace.
				if _, _, err := reader.Read(m.ComputerID, tapestore.NSEvent, digest); err == nil {
					continue
				}
			}
			dst := filepath.Join(root, filepath.FromSlash(kind.ns.LegacyRel(digest)))
			if _, err := copyVerified(filepath.Join(dir, kind.sub, digest), dst, digest, 0o640); err != nil {
				return written, err
			}
			written++
		}
	}
	return written, nil
}

func importRows(ctx context.Context, db *sql.DB, dir string, m Manifest, before *computerevent.Head, plan importPlan) (ImportResult, error) {
	res := ImportResult{HeadAfter: -1}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	head, err := readHead(ctx, tx, m.ComputerID, true)
	if err != nil {
		return res, err
	}
	if (head == nil) != (before == nil) || (head != nil && (head.Sequence != before.Sequence || head.CanonicalEventHead != before.CanonicalEventHead)) {
		return res, refusef("the target head of %s moved during the import", m.ComputerID)
	}
	if head != nil {
		res.HeadAfter = int64(head.Sequence)
	}
	if s := m.SnapshotRoot; s != nil {
		encoded, _ := json.Marshal(s.Chunks)
		r, err := tx.ExecContext(ctx, `INSERT INTO computer_snapshot_roots (computer_id,root,head_sequence,chunks,created_at) VALUES (?,?,?,?,?) ON CONFLICT (computer_id, root) DO NOTHING`,
			s.ComputerID, s.Root, s.HeadSequence, string(encoded), s.CreatedAt)
		if err != nil {
			return res, fmt.Errorf("bundle: insert snapshot root: %w", err)
		}
		n, _ := r.RowsAffected()
		res.SnapshotRootInserted = n == 1
	}
	var batch []ReceiptRow
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		query := `INSERT INTO computer_event_append_receipts (` + receiptColumns + `) VALUES ` + strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?,?,?,?,?),", len(batch)), ",")
		args := make([]any, 0, 10*len(batch))
		for _, r := range batch {
			args = append(args, r.ComputerID, r.IdempotencyKey, r.RequestCommitment, r.Sequence, r.EventKind, r.EventDigest, r.EventPinReceiptDigest, r.EventHeadReceiptID, r.EventHeadReceiptJSON, r.CreatedAt)
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("bundle: insert receipts: %w", err)
		}
		res.ReceiptsInserted += int64(len(batch))
		batch = batch[:0]
		return nil
	}
	err = forEachReceipt(dir, func(row ReceiptRow) (bool, error) {
		if row.Sequence <= plan.insertAfter {
			return true, nil
		}
		batch = append(batch, row)
		if len(batch) == insertBatch {
			return true, flush()
		}
		return true, nil
	})
	if err == nil {
		err = flush()
	}
	if err != nil {
		return res, err
	}
	h := m.Tape.Head
	now := time.Now().UTC()
	var pending any
	if h.PendingTransitionRef != "" {
		pending = h.PendingTransitionRef
	}
	switch {
	case head == nil:
		if _, err := tx.ExecContext(ctx, `INSERT INTO computer_event_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, reducer_version, credential_revocation_epoch, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			h.ComputerID, h.Sequence, h.CanonicalEventHead, h.DesiredEventHead, h.EffectiveEventHead, h.DesiredStateCommitment, h.EffectiveStateCommitment, pending, h.ReducerVersion, h.CredentialRevocationEpoch, now, now); err != nil {
			return res, fmt.Errorf("bundle: insert head: %w", err)
		}
		res.HeadAfter = int64(h.Sequence)
	case plan.advance:
		r, err := tx.ExecContext(ctx, `UPDATE computer_event_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=?, desired_state_commitment=?, effective_state_commitment=?, pending_transition_ref=?, reducer_version=?, credential_revocation_epoch=?, updated_at=? WHERE computer_id=? AND sequence=? AND canonical_event_head=?`,
			h.Sequence, h.CanonicalEventHead, h.DesiredEventHead, h.EffectiveEventHead, h.DesiredStateCommitment, h.EffectiveStateCommitment, pending, h.ReducerVersion, h.CredentialRevocationEpoch, now, h.ComputerID, head.Sequence, head.CanonicalEventHead)
		if err != nil {
			return res, fmt.Errorf("bundle: advance head: %w", err)
		}
		if n, err := r.RowsAffected(); err != nil || n != 1 {
			return res, refusef("the target head of %s moved during the import", m.ComputerID)
		}
		res.HeadAfter = int64(h.Sequence)
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("bundle: commit: %w", err)
	}
	return res, nil
}
