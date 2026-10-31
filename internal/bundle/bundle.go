// Package bundle exports, verifies and imports a computer bundle: a sealed
// snapshot root, its chunks, and the computer's tape from the snapshot's head
// to the current head (events, payloads, pin receipts, append receipts). One
// mechanism serves off-host backup, recovery from elsewhere and sync to
// another machine.
//
// A bundle is a directory:
//
//	manifest.json        format version 1, computer, snapshot root record,
//	                     tape range (start_seq, head_seq] with the start and
//	                     head digests and the head row, counts and bytes,
//	                     producer and time. Written last: a directory without
//	                     it is an unfinished export.
//	chunks/<digest>      the snapshot's sealed chunks
//	objects.pack, .idx   the tape objects (format 2, objects.go): the
//	                     canonical event bytes of the range, the payloads
//	                     the events' artifact refs name, and the pin
//	                     receipts the append receipts name
//	receipts.jsonl       the append receipts of the range in sequence order
//
// Format 1 kept each tape object as its own file (events/<digest>,
// payloads/<digest>, pins/<digest>); it is still read. Every chunk and tape
// object is named by the sha256 of its bytes, so the format does not depend on the host's storage
// layout (legacy files or tape segments): export reads through the
// platform's tape reader (internal/tapestore), import writes the legacy
// layout, which that reader serves.
//
// Export reads only PostgreSQL rows and sealed immutable host files, never
// the guest, so it works with the computer up or down. The bundle package
// does not import internal/platform: it reads and writes the platform's
// tables and file paths directly, so platform tests can drive it.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// FormatVersion is the bundle format this package writes. It reads
// formats 1 and 2.
const FormatVersion = 2

const (
	ManifestName = "manifest.json"
	ReceiptsName = "receipts.jsonl"
	ChunksDir    = "chunks"
	EventsDir    = "events"
	PayloadsDir  = "payloads"
	PinsDir      = "pins"
)

// SnapshotRoot is one computer_snapshot_roots row.
type SnapshotRoot struct {
	ComputerID   string    `json:"computer_id"`
	Root         string    `json:"root"`
	HeadSequence int64     `json:"head_sequence"`
	Chunks       []string  `json:"chunks"`
	CreatedAt    time.Time `json:"created_at"`
}

// Tape is the range (StartSeq, HeadSeq] of the bundle. StartHead is the
// canonical digest of the event at StartSeq (ZeroHead when StartSeq is 0):
// the first event of the range names it as its previous head. Head is the
// computer_event_heads row at export time; HeadSeq and HeadDigest are its
// sequence and canonical head.
type Tape struct {
	StartSeq   int64              `json:"start_seq"`
	StartHead  string             `json:"start_head"`
	HeadSeq    int64              `json:"head_seq"`
	HeadDigest string             `json:"head_digest"`
	Head       computerevent.Head `json:"head"`
}

// Totals are counts or byte totals of the bundle's files.
type Totals struct {
	Events      int64 `json:"events"`
	Payloads    int64 `json:"payloads"`
	PinReceipts int64 `json:"pin_receipts"`
	Chunks      int64 `json:"chunks"`
	Receipts    int64 `json:"receipts"`
}

// Producer names where a bundle was made.
type Producer struct {
	Host string `json:"host"`
	Tool string `json:"tool"`
}

// Manifest is manifest.json.
type Manifest struct {
	FormatVersion int    `json:"format_version"`
	ComputerID    string `json:"computer_id"`
	// OwnerID is given by the caller: the host's control database does not
	// hold the owner of a computer (vmctl's registry does).
	OwnerID string `json:"owner_id,omitempty"`
	// SnapshotRoot is nil for a whole-tape bundle from genesis and for a
	// delta.
	SnapshotRoot *SnapshotRoot `json:"snapshot_root"`
	// Delta marks a bundle that carries only the events after a previous
	// bundle's head (Tape.StartSeq, Tape.StartHead) and no snapshot: it
	// imports only onto a target already at that head (ExportDelta).
	Delta     bool      `json:"delta,omitempty"`
	Tape      Tape      `json:"tape"`
	Counts    Totals    `json:"counts"`
	Bytes     Totals    `json:"bytes"`
	Producer  Producer  `json:"producer"`
	CreatedAt time.Time `json:"created_at"`
}

// ReceiptRow is one computer_event_append_receipts row, one line of
// receipts.jsonl. EventHeadReceiptJSON is kept byte for byte.
type ReceiptRow struct {
	ComputerID            string    `json:"computer_id"`
	IdempotencyKey        string    `json:"idempotency_key"`
	RequestCommitment     string    `json:"request_commitment"`
	Sequence              int64     `json:"sequence"`
	EventKind             string    `json:"event_kind"`
	EventDigest           string    `json:"event_digest"`
	EventPinReceiptDigest string    `json:"event_pin_receipt_digest"`
	EventHeadReceiptID    string    `json:"event_head_receipt_id"`
	EventHeadReceiptJSON  string    `json:"event_head_receipt_json"`
	CreatedAt             time.Time `json:"created_at"`
}

// ErrRefused is wrapped by every import refusal (a fork, a differing head,
// a gap): the target was left as it was.
var ErrRefused = errors.New("bundle: import refused")

func refusef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, args...))
}

// safeComputerID is the platform's file CAS component rule
// (internal/platform safeFileCASComponent): the id names a directory.
func safeComputerID(id string) bool {
	return id != "" && id == strings.TrimSpace(id) && id != "." && id != ".." && !strings.ContainsAny(id, `/\`)
}

func validDigest(d string) bool { return computerevent.IsSHA256(d) }

// Host layout (internal/platform file_cas_blobs.go fileCASChunkRef).
func hostChunkPath(root, computerID, digest string) string {
	return filepath.Join(root, "sha256", "file-cas-chunks", computerID, digest)
}

// fileDigest streams path through sha256.
func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// presentVerified reports whether path exists and hashes to digest. A file
// that exists with other bytes is torn or corrupt and reported absent so the
// caller replaces it.
func presentVerified(path, digest string) (bool, int64, error) {
	got, n, err := fileDigest(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return got == digest, n, nil
}

// writeAtomic writes data to path through a temp file, fsync and rename, so
// a crash never leaves a partial object under its final name.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	return writeAtomicFrom(path, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func writeAtomicFrom(path string, perm os.FileMode, fill func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-bundle-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	err = fill(tmp)
	if err == nil {
		err = tmp.Chmod(perm)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// copyVerified copies src to dst, hashing as it goes; dst appears only when
// the bytes hash to digest.
func copyVerified(src, dst, digest string, perm os.FileMode) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	var n int64
	err = writeAtomicFrom(dst, perm, func(w io.Writer) error {
		h := sha256.New()
		var cerr error
		n, cerr = io.Copy(io.MultiWriter(w, h), in)
		if cerr != nil {
			return cerr
		}
		if got := hex.EncodeToString(h.Sum(nil)); got != digest {
			return fmt.Errorf("bundle: %s hashes to %s, want %s", src, got, digest)
		}
		return nil
	})
	return n, err
}

// eventPayloadDigests returns the digests of the event's input then output
// artifact refs, the order of its payload pin receipts (internal/platform
// ValidateEventPins).
func eventPayloadDigests(event computerevent.Event) ([]string, error) {
	out := make([]string, 0, len(event.InputArtifactRefs)+len(event.OutputArtifactRefs))
	for _, refs := range [][]string{event.InputArtifactRefs, event.OutputArtifactRefs} {
		for _, raw := range refs {
			ref, err := computerevent.ParseArtifactRef(raw)
			if err != nil {
				return nil, fmt.Errorf("bundle: event %d artifact ref %q: %w", event.Sequence, raw, err)
			}
			out = append(out, ref.Digest().String())
		}
	}
	return out, nil
}

// decodeEvent decodes canonical event bytes and checks they are canonical
// and hash to digest.
func decodeEvent(raw []byte, digest string) (computerevent.Event, error) {
	if computerevent.DigestBytes(raw) != digest {
		return computerevent.Event{}, fmt.Errorf("bundle: event %s bytes do not hash to their name", digest)
	}
	event, err := computerevent.DecodeHistoricEvent(raw)
	if err != nil {
		return computerevent.Event{}, fmt.Errorf("bundle: event %s: %w", digest, err)
	}
	canonical, err := event.CanonicalBytes()
	if err != nil {
		return computerevent.Event{}, fmt.Errorf("bundle: event %s: %w", digest, err)
	}
	if string(canonical) != string(raw) {
		return computerevent.Event{}, fmt.Errorf("bundle: event %s is not canonical", digest)
	}
	computed, err := event.Digest()
	if err != nil || computed != digest {
		return computerevent.Event{}, fmt.Errorf("bundle: event %s digest does not recompute", digest)
	}
	return event, nil
}

// headReceipt decodes an append receipt's EventHeadReceipt and returns its
// pin receipt digests (event pin first, then the payload pins).
func headReceipt(row ReceiptRow) (computerevent.Receipt, []string, error) {
	var receipt computerevent.Receipt
	if err := receipt.UnmarshalJSON([]byte(row.EventHeadReceiptJSON)); err != nil {
		return receipt, nil, fmt.Errorf("bundle: receipt at %d: %w", row.Sequence, err)
	}
	raw, _ := receipt.KindFields["pin_receipt_digests"].([]any)
	pins := make([]string, 0, len(raw))
	for _, v := range raw {
		s, _ := v.(string)
		if !validDigest(s) {
			return receipt, nil, fmt.Errorf("bundle: receipt at %d names an invalid pin receipt digest", row.Sequence)
		}
		pins = append(pins, s)
	}
	if len(pins) == 0 || pins[0] != row.EventPinReceiptDigest {
		return receipt, nil, fmt.Errorf("bundle: receipt at %d does not name its event pin first", row.Sequence)
	}
	return receipt, pins, nil
}
