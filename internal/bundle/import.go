package bundle

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	// Signatures is the number of receipt signatures verify checked.
	Signatures int64 `json:"signatures"`
}

const insertBatch = 500

// ImportHooks are test seams: deterministic points where a test swaps
// receipts.jsonl to prove every pass reads the bytes verify checked. They
// are nil in production.
var ImportHooks struct {
	// AfterVerify runs once VerifyBundle has returned, before the plan.
	AfterVerify func(dir string)
	// AfterPlan runs once the overlap plan is computed, before files and rows.
	AfterPlan func(dir string)
}

// Import writes the bundle in dir into a platform store: its files under
// artifactsRoot (chunks in the file CAS layout, tape objects in the legacy
// layout the platform's tape reader serves) and its rows into the control
// database db (opened through dbx.OpenPostgres, schema bootstrapped).
//
// It first runs VerifyBundle against trust and refuses (ErrRefused wrapping
// ErrUnauthentic) on any failure before it writes anything. trust is the
// importing host's own trust set, never one the bundle carries.
//
// It is idempotent. It refuses (ErrRefused, the target unchanged) when the
// target's tape of the computer does not chain: another genesis, a head
// behind the range's start (a gap), a head at the start with another digest,
// or an overlap (the target's receipts inside the range) that is not the
// bundle's byte for byte. A fork is a new computer, never a merge.
//
// Files land before rows, so a row never names a missing file; a crash
// between them leaves only unreferenced content-addressed files. Only the
// objects the verified tape references are written. The rows land in one
// serializable transaction that re-reads the head under lock, and commits
// only if receipts.jsonl still has the bytes verify checked. Run it with the
// target's corpusd stopped, or at least the computer idle: a racing append
// makes the import refuse, never interleave.
func Import(ctx context.Context, dir string, db *sql.DB, artifactsRoot string, trust *TrustSet) (ImportResult, error) {
	if db == nil || artifactsRoot == "" {
		return ImportResult{}, fmt.Errorf("bundle: target database and artifacts root are required")
	}
	v, err := VerifyBundle(dir, trust)
	if err != nil {
		return ImportResult{}, fmt.Errorf("%w: %w", ErrRefused, err)
	}
	if h := ImportHooks.AfterVerify; h != nil {
		h(dir)
	}
	m := v.Manifest
	id := m.ComputerID
	res := ImportResult{ComputerID: id, HeadBefore: -1, Signatures: v.Signatures}
	if dead, err := tombstoned(ctx, db, id); err != nil {
		return res, err
	} else if dead {
		return res, refusef("computer %s tape is tombstoned on the target", id)
	}
	if err := checkGenesis(ctx, db, m); err != nil {
		return res, err
	}
	head, err := readHead(ctx, db, id, false)
	if err != nil {
		return res, err
	}
	plan, err := planImport(ctx, db, dir, m, head, v.ReceiptsSHA256)
	if err != nil {
		return res, err
	}
	if h := ImportHooks.AfterPlan; h != nil {
		h(dir)
	}
	if head != nil {
		res.HeadBefore = int64(head.Sequence)
	}
	if err := checkSnapshotRoot(ctx, db, m); err != nil {
		return res, err
	}
	written, err := importFiles(dir, artifactsRoot, m, v.ReceiptsSHA256)
	if err != nil {
		return res, err
	}
	res.FilesWritten = written
	rows, err := dbx.Retry(ctx, 5, func() (ImportResult, error) {
		return importRows(ctx, db, dir, m, head, plan, v.ReceiptsSHA256)
	})
	if err != nil {
		return res, err
	}
	res.ReceiptsInserted, res.SnapshotRootInserted, res.HeadAfter = rows.ReceiptsInserted, rows.SnapshotRootInserted, rows.HeadAfter
	return res, nil
}

// checkGenesis refuses a target whose event 1 of the computer is not the
// bundle's genesis: the same computer id with another genesis is another
// computer.
func checkGenesis(ctx context.Context, db *sql.DB, m Manifest) error {
	if m.Genesis == "" {
		return nil
	}
	got, err := receiptDigestAt(ctx, db, m.ComputerID, 1)
	if err != nil {
		return err
	}
	if got != "" && got != m.Genesis {
		return refusef("target's genesis of %s is %s, the bundle's is %s (another computer under the same id)", m.ComputerID, got, m.Genesis)
	}
	return nil
}

// importPlan: receipts with sequence > insertAfter are inserted; advance says
// the head row moves to the bundle's head.
type importPlan struct {
	insertAfter int64
	advance     bool
}

func planImport(ctx context.Context, db *sql.DB, dir string, m Manifest, head *computerevent.Head, receiptsSHA string) (importPlan, error) {
	start, end := m.Tape.StartSeq, m.Tape.HeadSeq
	if head == nil {
		var n int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, m.ComputerID).Scan(&n); err != nil {
			return importPlan{}, fmt.Errorf("bundle: count target receipts: %w", err)
		}
		if n > 0 {
			return importPlan{}, refusef("target holds %d receipts of %s but no head", n, m.ComputerID)
		}
		if m.Delta && start > 0 {
			return importPlan{}, refusef("a delta from %d needs a target at that head; the target has no tape", start)
		}
		if m.SnapshotRoot != nil && start > m.SnapshotRoot.HeadSequence {
			return importPlan{}, refusef("range starts at %d, after its snapshot at %d: an empty target has nothing to chain it to", start, m.SnapshotRoot.HeadSequence)
		}
		if end == start {
			// No receipt is signed: nothing authenticates the head row.
			return importPlan{}, refusef("an empty range cannot start a tape on an empty target: no signed receipt authenticates its head")
		}
		return importPlan{insertAfter: start, advance: true}, nil
	}
	t := int64(head.Sequence)
	if t < start {
		return importPlan{}, refusef("target head %d is behind the range start %d (a gap)", t, start)
	}
	if t == start && head.CanonicalEventHead != m.Tape.StartHead {
		return importPlan{}, refusef("target head at %d is %s, the bundle chains from %s (a fork)", t, head.CanonicalEventHead, m.Tape.StartHead)
	}
	upTo := min(t, end)
	atUpTo, err := compareOverlap(ctx, db, dir, receiptsSHA, m.ComputerID, start, upTo)
	if err != nil {
		return importPlan{}, err
	}
	if t <= end && t > start && head.CanonicalEventHead != atUpTo {
		return importPlan{}, refusef("target head at %d is %s, the bundle has %s there", t, head.CanonicalEventHead, atUpTo)
	}
	if t > end {
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

// compareOverlap streams the bundle's receipts in (from, to] beside the
// target's and refuses unless each pair is the same receipt: the same event,
// idempotency key, request commitment, kind, pin receipt and head receipt,
// the head receipt compared over its canonical bytes (a host may store
// another JSON spelling or the compact form). Memory is one page of target
// rows.
func compareOverlap(ctx context.Context, db *sql.DB, dir, receiptsSHA, computerID string, from, to int64) (string, error) {
	if to <= from {
		return "", nil
	}
	var last string
	// A target that was itself imported from a later snapshot holds no
	// receipts below its range start; the overlap is what it holds.
	var lowest sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MIN(sequence) FROM computer_event_append_receipts WHERE computer_id=? AND sequence>? AND sequence<=?`, computerID, from, to).Scan(&lowest); err != nil {
		return "", fmt.Errorf("bundle: read target overlap: %w", err)
	}
	if !lowest.Valid {
		return "", refusef("target holds no receipt of %s in (%d, %d] under its head", computerID, from, to)
	}
	from = lowest.Int64 - 1
	var page []ReceiptRow
	after := from
	err := forEachReceipt(dir, receiptsSHA, func(row ReceiptRow) (bool, error) {
		if row.Sequence <= from {
			return true, nil
		}
		if row.Sequence > to {
			return false, nil
		}
		if len(page) == 0 {
			var err error
			if page, err = readReceipts(ctx, db, computerID, after, to, receiptPage); err != nil {
				return false, err
			}
			if len(page) == 0 {
				return false, refusef("target has no receipt at %d under its head (a gap in the target)", after+1)
			}
		}
		target := page[0]
		page = page[1:]
		after = target.Sequence
		if target.Sequence != row.Sequence {
			return false, refusef("target receipts skip from %d to %d", row.Sequence-1, target.Sequence)
		}
		same, err := sameReceipt(row, target)
		if err != nil {
			return false, err
		}
		if !same && target.EventDigest != row.EventDigest {
			return false, refusef("target receipt at %d is not the bundle's (event %s, the bundle has %s: a fork)", row.Sequence, target.EventDigest, row.EventDigest)
		}
		if !same {
			return false, refusef("target receipt at %d is not the bundle's (the same event %s under another receipt)", row.Sequence, row.EventDigest)
		}
		last = row.EventDigest
		return true, nil
	})
	if err != nil {
		return "", err
	}
	if after != to {
		return "", refusef("the bundle's overlap with the target ends at %d, not %d", after, to)
	}
	return last, nil
}

func sameReceipt(a, b ReceiptRow) (bool, error) {
	if a.ComputerID != b.ComputerID || a.IdempotencyKey != b.IdempotencyKey || a.RequestCommitment != b.RequestCommitment || a.Sequence != b.Sequence ||
		a.EventKind != b.EventKind || a.EventDigest != b.EventDigest || a.EventPinReceiptDigest != b.EventPinReceiptDigest || a.EventHeadReceiptID != b.EventHeadReceiptID {
		return false, nil
	}
	ca, err := canonicalReceipt(a.EventHeadReceiptJSON)
	if err != nil {
		return false, fmt.Errorf("bundle: receipt %d: %w", a.Sequence, err)
	}
	cb, err := canonicalReceipt(b.EventHeadReceiptJSON)
	if err != nil {
		return false, refusef("target receipt %d is unreadable: %v", b.Sequence, err)
	}
	return bytes.Equal(ca, cb), nil
}

func canonicalReceipt(raw string) ([]byte, error) {
	var r computerevent.Receipt
	if err := r.UnmarshalJSON([]byte(raw)); err != nil {
		return nil, err
	}
	return r.CanonicalBytes()
}

// forEachReceipt visits receipts.jsonl in order and always reads the whole
// file, even when visit stops early, so that every pass (plan, files, rows)
// is bound to the bytes verify checked: it refuses unless the sha256 of the
// file read is want. A visit error is returned first; a pass that decided
// anything from bytes other than the verified ones never returns success.
func forEachReceipt(dir, want string, visit func(ReceiptRow) (bool, error)) error {
	f, err := os.Open(filepath.Join(dir, ReceiptsName))
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	tee := io.TeeReader(f, h)
	scanner := bufio.NewScanner(tee)
	scanner.Buffer(make([]byte, 64<<10), maxReceiptLine)
	stopped := false
	for scanner.Scan() {
		var row ReceiptRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return fmt.Errorf("bundle: receipt line: %w", err)
		}
		more, err := visit(row)
		if err != nil {
			return err
		}
		if !more {
			stopped = true
			break
		}
	}
	if !stopped {
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	// Hash the rest of the file the visit did not need.
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return refusef("receipts.jsonl changed after it was verified")
	}
	return nil
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

// importFiles writes every content-addressed file the verified bundle
// references and the target does not already hold: the snapshot's chunks,
// then for each receipt its event, payloads and pin receipts. An object in
// the pack that no receipt names is never written. Tape objects the
// target's reader already serves (from a segment or a legacy file) are left
// alone.
func importFiles(dir, root string, m Manifest, receiptsSHA string) (int64, error) {
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
	objs, err := openObjects(dir, m.FormatVersion, false)
	if err != nil {
		return written, fmt.Errorf("bundle: open tape objects: %w", err)
	}
	defer objs.close()
	reader := tapestore.NewReader(root)
	done := newSeen(objs)
	put := func(sub string, ns tapestore.NS, digest string) error {
		if done.has(sub, digest) {
			return nil
		}
		done.add(sub, digest)
		if _, _, err := reader.Read(m.ComputerID, ns, digest); err == nil {
			return nil
		}
		if ns == tapestore.NSPayload {
			// The platform's payload fetch also probes the event namespace.
			if _, _, err := reader.Read(m.ComputerID, tapestore.NSEvent, digest); err == nil {
				return nil
			}
		}
		raw, err := objs.get(sub, digest)
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(ns.LegacyRel(digest))), raw, 0o640); err != nil {
			return err
		}
		written++
		return nil
	}
	err = forEachReceipt(dir, receiptsSHA, func(row ReceiptRow) (bool, error) {
		if err := put(EventsDir, tapestore.NSEvent, row.EventDigest); err != nil {
			return false, err
		}
		raw, err := objs.get(EventsDir, row.EventDigest)
		if err != nil {
			return false, err
		}
		event, err := decodeEvent(raw, row.EventDigest)
		if err != nil {
			return false, err
		}
		payloads, err := eventPayloadDigests(event)
		if err != nil {
			return false, err
		}
		for _, digest := range payloads {
			if err := put(PayloadsDir, tapestore.NSPayload, digest); err != nil {
				return false, err
			}
		}
		_, pins, err := headReceipt(row)
		if err != nil {
			return false, err
		}
		for _, digest := range pins {
			if err := put(PinsDir, tapestore.NSReceipt, digest); err != nil {
				return false, err
			}
		}
		return true, nil
	})
	return written, err
}

func importRows(ctx context.Context, db *sql.DB, dir string, m Manifest, before *computerevent.Head, plan importPlan, receiptsSHA string) (ImportResult, error) {
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
	err = forEachReceipt(dir, receiptsSHA, func(row ReceiptRow) (bool, error) {
		if row.Sequence <= plan.insertAfter {
			return true, nil
		}
		// created_at is not signed. The signed issued_at is the time the
		// host stamped on the receipt and the row together, so the row
		// never carries the bundle's value.
		receipt, _, err := headReceipt(row)
		if err != nil {
			return false, err
		}
		issued, err := time.Parse(time.RFC3339Nano, receipt.IssuedAt)
		if err != nil {
			return false, fmt.Errorf("bundle: receipt %d issued_at: %w", row.Sequence, err)
		}
		row.CreatedAt = issued.UTC()
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
		// A changed file fails here too: the rows inserted are the rows
		// verified, or nothing commits.
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
