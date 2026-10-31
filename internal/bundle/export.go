package bundle

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// ExportOptions selects what to export.
type ExportOptions struct {
	ComputerID string
	Out        string
	// FromSeq, when >= 0, starts the range at FromSeq instead of the
	// snapshot's head. It is needed when the computer has no snapshot root
	// (0 exports the whole tape from genesis).
	FromSeq int64
	// Root names a snapshot root; "" takes the newest.
	Root     string
	OwnerID  string
	Producer Producer
	Now      func() time.Time
}

const receiptPage = 1024

// Export writes the bundle of opts.ComputerID to opts.Out from the platform's
// control database and artifacts root. It reads only; files already present
// under opts.Out are not rewritten (an event file is read back and checked,
// so a torn one is replaced). manifest.json is written last.
func Export(ctx context.Context, db *sql.DB, artifactsRoot string, opts ExportOptions) (Manifest, error) {
	id := opts.ComputerID
	if !safeComputerID(id) {
		return Manifest{}, fmt.Errorf("bundle: invalid computer id")
	}
	if opts.Out == "" || artifactsRoot == "" {
		return Manifest{}, fmt.Errorf("bundle: output directory and artifacts root are required")
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	if dead, err := tombstoned(ctx, db, id); err != nil {
		return Manifest{}, err
	} else if dead {
		return Manifest{}, fmt.Errorf("bundle: computer %s tape is tombstoned", id)
	}
	snap, err := readSnapshotRoot(ctx, db, id, opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	if snap == nil && (opts.Root != "" || opts.FromSeq < 0) {
		return Manifest{}, fmt.Errorf("bundle: computer %s has no snapshot root (pass --from-seq 0 for the whole tape)", id)
	}
	// The head is read before the receipts: every receipt up to it is
	// committed with it, and appends after it are outside this bundle.
	head, err := readHead(ctx, db, id, false)
	if err != nil {
		return Manifest{}, err
	}
	if head == nil {
		return Manifest{}, fmt.Errorf("bundle: computer %s has no tape head", id)
	}
	start := opts.FromSeq
	if start < 0 {
		start = snap.HeadSequence
	}
	headSeq := int64(head.Sequence)
	if start > headSeq {
		return Manifest{}, fmt.Errorf("bundle: range start %d is past the head %d", start, headSeq)
	}
	m := Manifest{
		FormatVersion: FormatVersion, ComputerID: id, OwnerID: opts.OwnerID, SnapshotRoot: snap,
		Tape:     Tape{StartSeq: start, HeadSeq: headSeq, HeadDigest: head.CanonicalEventHead, Head: *head},
		Producer: opts.Producer,
	}
	switch {
	case start == 0:
		m.Tape.StartHead = computerevent.ZeroHead
	default:
		// A store that was itself imported holds no receipt at its range
		// start; the first event's previous head then stands for it.
		if m.Tape.StartHead, err = receiptDigestAt(ctx, db, id, start); err != nil {
			return Manifest{}, err
		}
	}
	if err := os.MkdirAll(opts.Out, 0o750); err != nil {
		return Manifest{}, err
	}
	reader := tapestore.NewReader(artifactsRoot)
	x := exporter{reader: reader, id: id, out: opts.Out, seenPayload: map[string]bool{}, seenPin: map[string]bool{}}
	prev := m.Tape.StartHead
	err = writeAtomicFrom(filepath.Join(opts.Out, ReceiptsName), 0o640, func(w io.Writer) error {
		bw := bufio.NewWriter(w)
		enc := json.NewEncoder(bw)
		after := start
		for after < headSeq {
			if err := ctx.Err(); err != nil {
				return err
			}
			rows, err := readReceipts(ctx, db, id, after, headSeq, receiptPage)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("bundle: receipts end at %d before the head %d", after, headSeq)
			}
			for _, row := range rows {
				if row.Sequence != after+1 {
					return fmt.Errorf("bundle: receipt sequence gap after %d (next %d)", after, row.Sequence)
				}
				event, err := x.event(row)
				if err != nil {
					return err
				}
				if prev == "" {
					prev = event.PreviousHead
					m.Tape.StartHead = prev
				}
				if event.PreviousHead != prev {
					return fmt.Errorf("bundle: event %d does not chain to %s", row.Sequence, prev)
				}
				if err := x.artifacts(row, event); err != nil {
					return err
				}
				if err := enc.Encode(row); err != nil {
					return err
				}
				m.Counts.Receipts++
				prev = row.EventDigest
				after = row.Sequence
			}
		}
		return bw.Flush()
	})
	if err != nil {
		return Manifest{}, err
	}
	if m.Tape.StartHead == "" { // empty range from an imported start
		m.Tape.StartHead = head.CanonicalEventHead
	}
	if prev != "" && prev != head.CanonicalEventHead {
		return Manifest{}, fmt.Errorf("bundle: the range ends at %s, not the head %s", prev, head.CanonicalEventHead)
	}
	m.Counts.Events, m.Bytes.Events = x.counts.Events, x.bytes.Events
	m.Counts.Payloads, m.Bytes.Payloads = x.counts.Payloads, x.bytes.Payloads
	m.Counts.PinReceipts, m.Bytes.PinReceipts = x.counts.PinReceipts, x.bytes.PinReceipts
	if snap != nil {
		seen := map[string]bool{}
		for _, digest := range snap.Chunks {
			if seen[digest] {
				continue
			}
			seen[digest] = true
			if !validDigest(digest) {
				return Manifest{}, fmt.Errorf("bundle: snapshot root names an invalid chunk digest")
			}
			n, err := exportChunk(artifactsRoot, id, opts.Out, digest)
			if err != nil {
				return Manifest{}, err
			}
			m.Counts.Chunks++
			m.Bytes.Chunks += n
		}
	}
	m.CreatedAt = now().UTC()
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := writeAtomic(filepath.Join(opts.Out, ManifestName), append(raw, '\n'), 0o640); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

type exporter struct {
	reader      *tapestore.Reader
	id, out     string
	seenPayload map[string]bool
	seenPin     map[string]bool
	counts      Totals
	bytes       Totals
}

// event returns the event the row names, from the out dir when a verified
// copy is there, otherwise from the host tape, and writes it.
func (x *exporter) event(row ReceiptRow) (computerevent.Event, error) {
	if !validDigest(row.EventDigest) {
		return computerevent.Event{}, fmt.Errorf("bundle: receipt %d has an invalid event digest", row.Sequence)
	}
	path := filepath.Join(x.out, EventsDir, row.EventDigest)
	raw, err := os.ReadFile(path)
	fresh := err != nil || computerevent.DigestBytes(raw) != row.EventDigest
	if fresh {
		raw, _, err = x.reader.Read(x.id, tapestore.NSEvent, row.EventDigest)
		if err != nil {
			return computerevent.Event{}, fmt.Errorf("bundle: read event %d: %w", row.Sequence, err)
		}
	}
	event, err := decodeEvent(raw, row.EventDigest)
	if err != nil {
		return computerevent.Event{}, err
	}
	if event.ComputerID != x.id || int64(event.Sequence) != row.Sequence || string(event.EventKind) != row.EventKind || event.IdempotencyKey != row.IdempotencyKey || event.RequestCommitment != row.RequestCommitment {
		return computerevent.Event{}, fmt.Errorf("bundle: event %d does not match its append receipt", row.Sequence)
	}
	if fresh {
		if err := writeAtomic(path, raw, 0o640); err != nil {
			return computerevent.Event{}, err
		}
	}
	x.counts.Events++
	x.bytes.Events += int64(len(raw))
	return event, nil
}

func (x *exporter) artifacts(row ReceiptRow, event computerevent.Event) error {
	payloads, err := eventPayloadDigests(event)
	if err != nil {
		return err
	}
	for _, digest := range payloads {
		if x.seenPayload[digest] {
			continue
		}
		x.seenPayload[digest] = true
		n, err := x.copyTape(PayloadsDir, digest, tapestore.NSPayload, tapestore.NSEvent)
		if err != nil {
			return fmt.Errorf("bundle: payload %s of event %d: %w", digest, row.Sequence, err)
		}
		x.counts.Payloads++
		x.bytes.Payloads += n
	}
	_, pins, err := headReceipt(row)
	if err != nil {
		return err
	}
	for _, digest := range pins {
		if x.seenPin[digest] {
			continue
		}
		x.seenPin[digest] = true
		n, err := x.copyTape(PinsDir, digest, tapestore.NSReceipt)
		if err != nil {
			return fmt.Errorf("bundle: pin receipt %s of event %d: %w", digest, row.Sequence, err)
		}
		x.counts.PinReceipts++
		x.bytes.PinReceipts += n
	}
	return nil
}

// copyTape writes the tape object (the first namespace that holds it, as
// the platform's FetchPayload probes) unless the out dir already has it.
func (x *exporter) copyTape(dir, digest string, namespaces ...tapestore.NS) (int64, error) {
	path := filepath.Join(x.out, dir, digest)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return info.Size(), nil
	}
	var firstErr error
	for _, ns := range namespaces {
		raw, _, err := x.reader.Read(x.id, ns, digest)
		if err != nil {
			if firstErr == nil || !errors.Is(err, tapestore.ErrNotFound) {
				firstErr = err
			}
			continue
		}
		return int64(len(raw)), writeAtomic(path, raw, 0o640)
	}
	return 0, firstErr
}

func exportChunk(root, computerID, out, digest string) (int64, error) {
	dst := filepath.Join(out, ChunksDir, digest)
	if info, err := os.Stat(dst); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return info.Size(), nil
	}
	n, err := copyVerified(hostChunkPath(root, computerID, digest), dst, digest, 0o640)
	if err != nil {
		return 0, fmt.Errorf("bundle: snapshot chunk %s: %w", digest, err)
	}
	return n, nil
}
