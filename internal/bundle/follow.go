package bundle

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A follower (the owner's Mac, docs/design/one-tape-durability-2026-10-31.md
// section 4a) keeps a verified copy of a computer by pulling bundles over
// HTTP: a bootstrap bundle (the newest snapshot root, its chunks and the tape
// from its head, or the tape from genesis when there is no root), then deltas
// from the head it holds. Each pull is bounded by ExportOptions.MaxEvents, and
// travels as a tar stream (WriteTar, ReadTar) of the same bundle directory, so
// the receiving side verifies exactly what choir-bundle verify checks.

// ExportSince writes a delta bundle of opts.ComputerID to opts.Out: the
// events after sinceSeq, whose event at sinceSeq must be sinceHead (ZeroHead
// when sinceSeq is 0), with no snapshot. It is ExportDelta for a follower
// that holds the head but not the previous bundle's directory.
func ExportSince(ctx context.Context, db *sql.DB, artifactsRoot string, sinceSeq int64, sinceHead string, opts ExportOptions) (Manifest, error) {
	if sinceSeq < 0 || !validDigest(sinceHead) {
		return Manifest{}, fmt.Errorf("bundle: a delta needs a start sequence and head digest")
	}
	if sinceSeq == 0 && sinceHead != computerevent.ZeroHead {
		return Manifest{}, fmt.Errorf("bundle: the head at sequence 0 is the zero head")
	}
	opts.FromSeq, opts.Root = sinceSeq, ""
	opts.delta, opts.sinceHead = true, sinceHead
	return Export(ctx, db, artifactsRoot, opts)
}

// projectHead is the head row as it stood right after the append receipt
// row: what a range that ends before the current head carries.
func projectHead(current computerevent.Head, row ReceiptRow, event computerevent.Event) (computerevent.Head, error) {
	receipt, _, err := headReceipt(row)
	if err != nil {
		return computerevent.Head{}, err
	}
	field := func(name string) string {
		s, _ := receipt.KindFields[name].(string)
		return s
	}
	return computerevent.Head{
		ComputerID:                current.ComputerID,
		Sequence:                  uint64(row.Sequence),
		CanonicalEventHead:        row.EventDigest,
		DesiredEventHead:          field("desired_event_head"),
		EffectiveEventHead:        field("effective_event_head"),
		PendingTransitionRef:      field("pending_transition_ref"),
		DesiredStateCommitment:    field("desired_state_commitment"),
		EffectiveStateCommitment:  field("effective_state_commitment"),
		ReducerVersion:            event.ReducerVersion,
		CredentialRevocationEpoch: current.CredentialRevocationEpoch,
	}, nil
}

// snapshotNamedSeq is the sequence of the first snapshot_recorded event that
// names snap after its head (readNamedSnapshotRoot's rule).
func snapshotNamedSeq(ctx context.Context, db *sql.DB, computerID string, snap *SnapshotRoot) (int64, error) {
	var seq sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MIN(sequence) FROM computer_event_append_receipts WHERE computer_id=? AND idempotency_key=? AND event_kind='snapshot_recorded' AND sequence>?`,
		computerID, "snapshot:"+snap.Root, snap.HeadSequence).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("bundle: read the snapshot's naming event: %w", err)
	}
	return seq.Int64, nil
}

func hostChunkSize(root, computerID, digest string) (int64, error) {
	info, err := os.Stat(hostChunkPath(root, computerID, digest))
	if err != nil {
		return 0, fmt.Errorf("bundle: snapshot chunk %s: %w", digest, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return 0, fmt.Errorf("bundle: snapshot chunk %s is not a sealed file", digest)
	}
	return info.Size(), nil
}

// WriteTar streams the format 2 bundle in dir (manifest m) to w as a tar:
// receipts.jsonl, objects.idx, objects.pack, the snapshot's chunks, and
// manifest.json last, so a stream cut short never carries a manifest. A
// chunk not in dir (ExportOptions.ChunksByReference) is read from the
// host's sealed file under artifactsRoot and hashed as it goes; one whose
// bytes do not hash to its name ends the stream with an error. Memory is one
// copy buffer whatever the bundle's size.
func WriteTar(w io.Writer, dir, artifactsRoot string, m Manifest) error {
	if m.FormatVersion != 2 {
		return fmt.Errorf("bundle: only format 2 bundles stream (this is %d)", m.FormatVersion)
	}
	tw := tar.NewWriter(w)
	for _, name := range []string{ReceiptsName, PackIndexName, PackName} {
		if err := tarFile(tw, name, filepath.Join(dir, name), ""); err != nil {
			return err
		}
	}
	if s := m.SnapshotRoot; s != nil {
		seen := map[string]bool{}
		for _, digest := range s.Chunks {
			if seen[digest] {
				continue
			}
			seen[digest] = true
			if !validDigest(digest) {
				return fmt.Errorf("bundle: invalid chunk digest %q", digest)
			}
			src := filepath.Join(dir, ChunksDir, digest)
			if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) && artifactsRoot != "" {
				src = hostChunkPath(artifactsRoot, m.ComputerID, digest)
			}
			if err := tarFile(tw, ChunksDir+"/"+digest, src, digest); err != nil {
				return err
			}
		}
	}
	if err := tarFile(tw, ManifestName, filepath.Join(dir, ManifestName), ""); err != nil {
		return err
	}
	return tw.Close()
}

// tarFile writes src as name; with digest set, the bytes must hash to it.
func tarFile(tw *tar.Writer, name, src, digest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("bundle: stream %s: %w", name, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bundle: stream %s: not a regular file", name)
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: info.Size(), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
		return err
	}
	h := sha256.New()
	var dst io.Writer = tw
	if digest != "" {
		dst = io.MultiWriter(tw, h)
	}
	if _, err := io.Copy(dst, io.LimitReader(f, info.Size())); err != nil {
		return fmt.Errorf("bundle: stream %s: %w", name, err)
	}
	if digest != "" && hex.EncodeToString(h.Sum(nil)) != digest {
		return fmt.Errorf("bundle: chunk %s does not hash to its name", digest)
	}
	return nil
}

// ErrHistoryMismatch is wrapped when a delta's start is not on the source's
// tape: the follower holds a fork or another computer's history.
var ErrHistoryMismatch = errors.New("bundle: the start head is not on the source's tape")

// ErrTooLarge is returned by ReadTar when the stream passes its byte bound.
var ErrTooLarge = errors.New("bundle: stream exceeds its bound")

// ReadTar unpacks a bundle tar stream (WriteTar) into dir, which should be
// empty: only the bundle's own names are accepted (manifest.json,
// receipts.jsonl, objects.idx, objects.pack, chunks/<sha256>), each at most
// once, as regular files, at most maxBytes in all. Every file is fsynced. It
// does not verify: the caller runs Verify on dir, and a stream that ends
// early has no manifest.
func ReadTar(r io.Reader, dir string, maxBytes int64) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tr := tar.NewReader(r)
	seen := map[string]bool{}
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("bundle: read stream: %w", err)
		}
		name := hdr.Name
		if hdr.Typeflag != tar.TypeReg || !tarNameAllowed(name) || seen[name] {
			return fmt.Errorf("bundle: stream carries an unexpected entry %q", name)
		}
		seen[name] = true
		if hdr.Size < 0 || total+hdr.Size > maxBytes {
			return ErrTooLarge
		}
		total += hdr.Size
		dst := filepath.Join(dir, filepath.FromSlash(name))
		err = writeAtomicFrom(dst, 0o640, func(w io.Writer) error {
			n, err := io.Copy(w, io.LimitReader(tr, hdr.Size))
			if err == nil && n != hdr.Size {
				err = io.ErrUnexpectedEOF
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("bundle: read stream %s: %w", name, err)
		}
	}
	if !seen[ManifestName] {
		return fmt.Errorf("bundle: stream ended without a manifest")
	}
	return nil
}

func tarNameAllowed(name string) bool {
	switch name {
	case ManifestName, ReceiptsName, PackIndexName, PackName:
		return true
	}
	dir, file := path.Split(name)
	return dir == ChunksDir+"/" && validDigest(file) && !strings.Contains(file, "/")
}
