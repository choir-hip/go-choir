package tapestore

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Round 5 (docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md,
// round_5_design): a segment error falls back to the legacy file only when
// that file hashes to the requested digest, and every such read is counted.
// Both enabled-mode P1s of round 4 end in an acknowledged pin whose valid
// legacy copy the reader refused to reach.

func flipSealedRecord(t *testing.T, root string, pl []byte) {
	t.Helper()
	entries := build(t, root, cid, 1, true, rec{NSPayload, pl})
	off := entries[openKey{NSPayload, sha256.Sum256(pl)}].off
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+recHeaderSize+3] ^= 0x10; return b })
}

// FM1: a corrupt sealed record (P1-2's shadowing tail) with an intact legacy
// copy serves the legacy copy, counted.
func TestR5CorruptSealedRecordServesVerifiedLegacy(t *testing.T) {
	root := t.TempDir()
	pl := body("shadowed pin %s", strings.Repeat("s", 80))
	flipSealedRecord(t, root, pl)
	legacyPut(t, root, NSPayload, pl)
	r := NewReader(root)
	if src := mustRead(t, r, cid, NSPayload, pl); src != SourceLegacy {
		t.Fatalf("served from %v, want legacy", src)
	}
	if st := r.Stats(); st.SegmentErrorFallbacks != 1 || st.Corrupt != 1 || st.LegacyHits != 0 {
		t.Fatalf("stats = %+v, want one counted fallback and one corrupt segment read", st)
	}
}

// FM2: the same corruption with no legacy copy fails closed.
func TestR5CorruptSealedRecordWithoutLegacyFailsClosed(t *testing.T) {
	root := t.TempDir()
	pl := body("no legacy copy")
	flipSealedRecord(t, root, pl)
	r := NewReader(root)
	mustCorrupt(t, r, cid, NSPayload, pl)
	if st := r.Stats(); st.SegmentErrorFallbacks != 0 {
		t.Fatalf("stats = %+v, counted a fallback that did not happen", st)
	}
}

// FM3: a legacy copy that does not hash is never served; the segment error
// is returned.
func TestR5RottenLegacyNeverServedOnSegmentError(t *testing.T) {
	root := t.TempDir()
	pl := body("both copies bad")
	flipSealedRecord(t, root, pl)
	p := filepath.Join(root, filepath.FromSlash(NSPayload.LegacyRel(sum(pl))))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("rotten"), 0o640); err != nil {
		t.Fatal(err)
	}
	mustCorrupt(t, NewReader(root), cid, NSPayload, pl)
}

// FM4: P1-1. A corrupt unsealed segment that is not the newest (a recovery
// refusal leaves it; the pin went to legacy) and an index without its
// segment both serve a verified legacy copy.
func TestR5UnreadableSegmentDirectoryServesVerifiedLegacy(t *testing.T) {
	t.Run("corrupt non-final unsealed segment", func(t *testing.T) {
		root := t.TempDir()
		a, pl := body("first"), body("acknowledged through legacy")
		entries := build(t, root, cid, 1, false, rec{NSPayload, a})
		build(t, root, cid, 2, false)
		off := entries[openKey{NSPayload, sha256.Sum256(a)}].off
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+recHeaderSize] ^= 1; return b })
		legacyPut(t, root, NSPayload, pl)
		r := NewReader(root)
		if src := mustRead(t, r, cid, NSPayload, pl); src != SourceLegacy {
			t.Fatalf("served from %v", src)
		}
		if r.Stats().SegmentErrorFallbacks != 1 {
			t.Fatalf("stats = %+v", r.Stats())
		}
	})
	t.Run("index without its segment", func(t *testing.T) {
		root := t.TempDir()
		pl := body("legacy behind a broken listing")
		build(t, root, cid, 1, true, rec{NSPayload, body("x")})
		if err := os.Remove(segPath(root, cid, 1)); err != nil {
			t.Fatal(err)
		}
		legacyPut(t, root, NSPayload, pl)
		if src := mustRead(t, NewReader(root), cid, NSPayload, pl); src != SourceLegacy {
			t.Fatalf("served from %v", src)
		}
	})
}

// FM5: segment-only reads keep reporting the segment error, so the census and
// the repack never count a corrupt segment copy as good.
func TestR5SegmentOnlyReadsStillReportTheError(t *testing.T) {
	root := t.TempDir()
	pl := body("census must see this")
	flipSealedRecord(t, root, pl)
	legacyPut(t, root, NSPayload, pl)
	if _, _, err := NewReader(root).ReadSegment(cid, NSPayload, sum(pl)); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("ReadSegment = %v, want ErrCorrupt", err)
	}
}

// Panel: an unsupported segment version is a downgrade or a format the
// binary does not know, not rot; it fails closed even with a legacy copy.
func TestR5UnsupportedVersionNeverFallsBack(t *testing.T) {
	root := t.TempDir()
	pl := body("written by a newer format")
	build(t, root, cid, 1, true, rec{NSPayload, pl})
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[4] = 0x7f; return b })
	legacyPut(t, root, NSPayload, pl)
	_, _, err := NewReader(root).Read(cid, NSPayload, sum(pl))
	if !errors.Is(err, ErrUnsupportedVersion) || !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Read = %v, want ErrUnsupportedVersion (and ErrCorrupt)", err)
	}
}

// Panel: a legacy read that fails for a reason other than absence or a wrong
// hash is reported with the segment error, not dropped.
func TestR5LegacyIOErrorIsReportedWithTheSegmentError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	root := t.TempDir()
	pl := body("legacy unreadable")
	flipSealedRecord(t, root, pl)
	p := legacyPut(t, root, NSPayload, pl)
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(NSPayload.LegacyRel(p))), 0); err != nil {
		t.Fatal(err)
	}
	_, _, err := NewReader(root).Read(cid, NSPayload, sum(pl))
	if !errors.Is(err, ErrCorrupt) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Read = %v, want the segment error joined with the legacy permission error", err)
	}
}
