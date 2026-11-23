package snapshot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

// Ways the page-digest reader (restore of a VersionPageDigest record) could
// fail, written before the reader:
//
//  D1. a record of either version restores through the same install, and a
//      record whose form disagrees with its version, or that carries neither
//      form, is refused (not accepted unverified, not left to Verify, which an
//      install may omit);
//  D2. a plaintext byte flipped before sealing (the capture sealed other bytes
//      than it read) is refused by the digest, as are pages out of order inside
//      a chunk, chunks out of order, and a short file; a flipped sealed byte is
//      refused by the seal;
//  D3. the geometry is inside the digest: the same bytes under another page
//      size do not verify;
//  D4. the digest is checked before Verify opens the file, and a refused
//      candidate falls back to the next older snapshot and leaves no file;
//  D5. a version-1 record still restores and is still verified by Verify.

// v2Case is one record on an otherwise empty tape.
type v2Case struct {
	f     *fixture
	base  Record // the version-2 record the capture made
	v1    Record // the same chunks recorded as a version-1 record (legacyV1)
	plain []byte // the state file's plaintext
}

func newV2Case(t *testing.T) *v2Case {
	t.Helper()
	f := newFixture(t, 2*ChunkSize+ChunkSize/2)
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	dek, err := f.cipher.ExportKeyForEscrow(context.Background(), testComputer)
	if err != nil {
		t.Fatal(err)
	}
	var plain []byte
	for _, d := range ref.Record.Manifest.Files[0].Chunks {
		sealed, err := f.chunks.GetChunk(context.Background(), d)
		if err != nil {
			t.Fatal(err)
		}
		p, err := filecas.OpenChunk(dek, testComputer, d, sealed)
		if err != nil {
			t.Fatal(err)
		}
		plain = append(plain, p...)
	}
	if len(plain) != int(ref.Record.PlaintextSize) || len(plain) < 2*ChunkSize+20*int(ref.Record.PageSize) {
		t.Fatalf("test premise: %d bytes of plaintext for %d recorded", len(plain), ref.Record.PlaintextSize)
	}
	return &v2Case{f: f, base: ref.Record, v1: f.legacyV1(t, ref), plain: plain}
}

func (c *v2Case) digestOf(plain []byte, pageSize int, pageCount int64) string {
	d := NewPageDigest(pageSize, pageCount)
	_, _ = d.Write(plain)
	return d.Sum()
}

// v2 is the version-2 record of plain, sealed and put on the host, with the
// digest over digestPlain (the bytes the capture says it read).
func (c *v2Case) v2(t *testing.T, plain, digestPlain []byte) Record {
	t.Helper()
	dek, err := c.f.cipher.ExportKeyForEscrow(context.Background(), testComputer)
	if err != nil {
		t.Fatal(err)
	}
	rec := c.base
	rec.Version, rec.WitnessRoot, rec.SchemaHash = VersionPageDigest, "", ""
	rec.PageDigest = c.digestOf(digestPlain, c.base.PageSize, c.base.PageCount)
	entry := filecas.FileEntry{Path: StateFileName, Mode: 0o600, Size: int64(len(plain))}
	for off := 0; off < len(plain); off += ChunkSize {
		sealed, digest, err := filecas.SealChunk(dek, testComputer, plain[off:min(off+ChunkSize, len(plain))])
		if err != nil {
			t.Fatal(err)
		}
		if err := c.f.chunks.PutChunk(context.Background(), digest, sealed); err != nil {
			t.Fatal(err)
		}
		entry.Chunks = append(entry.Chunks, digest)
	}
	manifest, err := filecas.BuildManifest(testComputer, []filecas.FileEntry{entry}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec.Manifest = *manifest
	return rec
}

// only puts rec alone on a fresh tape, so a refusal cannot fall back.
func (c *v2Case) only(t *testing.T, rec Record) {
	t.Helper()
	c.f.tape.mu.Lock()
	c.f.tape.events, c.f.tape.payloads = nil, map[string][]byte{}
	c.f.tape.mu.Unlock()
	c.append(t, rec, "event-v2")
}

func (c *v2Case) append(t *testing.T, rec Record, id string) {
	t.Helper()
	plaintext, err := jsonMarshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: testComputer,
		EventKind: computerevent.EventSnapshotRecorded, PrivacyClass: "private",
	}
	if _, _, err := c.f.tape.AppendNewPrivatePayload(context.Background(), event, computerevent.TransitionInput{}, plaintext, RecordMediaType, c.f.cipher); err != nil {
		t.Fatal(err)
	}
}

// restore installs with a Verify that counts its calls.
func (c *v2Case) restore(t *testing.T) (Record, string, int, error) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "state.db")
	cfg := c.f.restoreConfig(dest)
	calls := 0
	cfg.Verify = func(context.Context, Record, string) error { calls++; return nil }
	rec, err := Restore(context.Background(), cfg)
	return rec, dest, calls, err
}

func (c *v2Case) refused(t *testing.T, wantErr string) {
	t.Helper()
	_, dest, calls, err := c.restore(t)
	if err == nil {
		t.Fatalf("a record that should be refused (%s) restored", wantErr)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("refused for the wrong reason: %v (want %q)", err, wantErr)
	}
	if calls != 0 {
		t.Fatalf("Verify ran %d time(s) on a record the install refuses first", calls)
	}
	if _, err := os.Lstat(dest); err == nil {
		t.Fatal("a refused restore left a state file")
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".snapshot-restore-*"))
	if len(left) != 0 {
		t.Fatalf("a refused restore left staging files %v", left)
	}
}

// D1, D5
func TestPageDigestRecordRestoresAndVersionOneStillDoes(t *testing.T) {
	c := newV2Case(t)
	// An older capture's version-1 record, verified by Verify as today.
	c.only(t, c.v1)
	rec, dest, calls, err := c.restore(t)
	if err != nil || rec.Version != Version || calls != 1 {
		t.Fatalf("version 1 restore = %+v, verify calls %d, %v", rec, calls, err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, c.plain) {
		t.Fatal("version 1 restored other bytes")
	}
	// The capture's own record is version 2, and its digest is that of the
	// plaintext the chunks hold.
	if c.base.Version != VersionPageDigest || c.base.PageDigest != c.digestOf(c.plain, c.base.PageSize, c.base.PageCount) {
		t.Fatalf("the capture's record %+v does not digest the pages it uploaded", c.base)
	}
	c.only(t, c.v2(t, c.plain, c.plain))
	rec, dest, calls, err = c.restore(t)
	if err != nil || rec.Version != VersionPageDigest || rec.PageDigest == "" || rec.WitnessRoot != "" || calls != 1 {
		t.Fatalf("version 2 restore = %+v, verify calls %d, %v", rec, calls, err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, c.plain) {
		t.Fatal("version 2 restored other bytes")
	}
}

// D1: the old-form and new-form matrix, and a record with neither.
func TestRecordFormMatrix(t *testing.T) {
	c := newV2Case(t)
	good2 := c.v2(t, c.plain, c.plain)
	cases := []struct {
		name string
		rec  func() Record
		want string // empty: restores
	}{
		{"v1 with its witness", func() Record { return c.v1 }, ""},
		{"v2 with its digest", func() Record { return good2 }, ""},
		{"v1 with neither form", func() Record { r := c.v1; r.WitnessRoot, r.SchemaHash = "", ""; return r }, "carries no witness"},
		{"v1 without a schema hash", func() Record { r := c.v1; r.SchemaHash = ""; return r }, "carries no witness"},
		{"v1 without a witness root", func() Record { r := c.v1; r.WitnessRoot = ""; return r }, "carries no witness"},
		{"v1 that also carries a digest", func() Record { r := c.v1; r.PageDigest = good2.PageDigest; return r }, "version 1 record carries a page digest"},
		{"v2 with neither form", func() Record { r := good2; r.PageDigest = ""; return r }, "page digest"},
		{"v2 with a malformed digest", func() Record { r := good2; r.PageDigest = "not-a-digest"; return r }, "page digest"},
		{"v2 that also carries a witness root", func() Record { r := good2; r.WitnessRoot = c.v1.WitnessRoot; return r }, "version 2 record carries a witness"},
		{"v2 that also carries a schema hash", func() Record { r := good2; r.SchemaHash = c.v1.SchemaHash; return r }, "version 2 record carries a witness"},
		{"v2 without page geometry", func() Record {
			r := good2
			r.PageSize, r.PageCount, r.PagesPerChunk = 0, 0, 0
			return r
		}, "page geometry"},
		{"version 3", func() Record { r := good2; r.Version = 3; return r }, "version 3"},
		{"version 0", func() Record { r := good2; r.Version = 0; return r }, "version 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.only(t, tc.rec())
			if tc.want != "" {
				c.refused(t, tc.want)
				return
			}
			if _, _, calls, err := c.restore(t); err != nil || calls != 1 {
				t.Fatalf("restore = %v, verify calls %d", err, calls)
			}
		})
	}
}

// D2
func TestPageDigestRefusesWhatTheCaptureDidNotRead(t *testing.T) {
	c := newV2Case(t)
	ps := c.base.PageSize
	flip := func(b []byte, at int) []byte { o := bytes.Clone(b); o[at] ^= 0x40; return o }
	swapPages := func(b []byte, i, j int) []byte {
		o := bytes.Clone(b)
		pi, pj := bytes.Clone(o[i*ps:(i+1)*ps]), bytes.Clone(o[j*ps:(j+1)*ps])
		copy(o[i*ps:], pj)
		copy(o[j*ps:], pi)
		return o
	}
	swapChunks := func(b []byte) []byte {
		o := bytes.Clone(b)
		copy(o[:ChunkSize], b[ChunkSize:2*ChunkSize])
		copy(o[ChunkSize:2*ChunkSize], b[:ChunkSize])
		return o
	}
	t.Run("a plaintext byte flipped before sealing", func(t *testing.T) {
		c.only(t, c.v2(t, flip(c.plain, ChunkSize+5*ps+17), c.plain))
		c.refused(t, "page digest")
	})
	t.Run("the last byte flipped before sealing", func(t *testing.T) {
		c.only(t, c.v2(t, flip(c.plain, len(c.plain)-1), c.plain))
		c.refused(t, "page digest")
	})
	t.Run("two pages swapped inside a chunk", func(t *testing.T) {
		c.only(t, c.v2(t, swapPages(c.plain, 3, 9), c.plain))
		c.refused(t, "page digest")
	})
	t.Run("two chunks out of order", func(t *testing.T) {
		c.only(t, c.v2(t, swapChunks(c.plain), c.plain))
		c.refused(t, "page digest")
	})
	t.Run("a short file", func(t *testing.T) {
		c.only(t, c.v2(t, c.plain[:len(c.plain)-ps], c.plain))
		c.refused(t, "manifest size does not match the record")
	})
	t.Run("a flipped sealed byte", func(t *testing.T) {
		rec := c.v2(t, c.plain, c.plain)
		c.only(t, rec)
		d := rec.Manifest.Files[0].Chunks[1]
		c.f.chunks.mu.Lock()
		c.f.chunks.chunks[d][len(c.f.chunks.chunks[d])/2] ^= 0x01
		c.f.chunks.mu.Unlock()
		c.refused(t, "open chunk")
	})
}

// D3: the page size and count are bound into the digest.
func TestPageDigestBindsTheGeometry(t *testing.T) {
	c := newV2Case(t)
	half := c.base.PageSize / 2
	rec := c.v2(t, c.plain, c.plain)
	// The same bytes, claimed as twice the pages of half the size, with the
	// digest of the original geometry.
	rec.PageSize, rec.PageCount, rec.PagesPerChunk = half, c.base.PageCount*2, c.base.PagesPerChunk*2
	c.only(t, rec)
	c.refused(t, "page digest")
	if a, b := c.digestOf(c.plain, 4096, 100), c.digestOf(c.plain, 4096, 101); a == b {
		t.Fatal("the page count is not in the digest")
	}
	if a, b := c.digestOf(c.plain, 4096, 100), c.digestOf(c.plain, 2048, 100); a == b {
		t.Fatal("the page size is not in the digest")
	}
}

// D4: a refused newest candidate falls back to the older snapshot.
func TestPageDigestFailureFallsBackToTheOlderSnapshot(t *testing.T) {
	c := newV2Case(t)
	c.append(t, c.v2(t, c.plain, c.plain), "event-v2-ok")
	bad := c.v2(t, c.plain, c.plain)
	bad.PageDigest = strings.Repeat("0", 64)
	c.append(t, bad, "event-v2-bad")
	rec, _, calls, err := c.restore(t)
	if err != nil {
		t.Fatal(err)
	}
	if rec.PageDigest == strings.Repeat("0", 64) || calls != 1 {
		t.Fatalf("restored the refused record or ran Verify %d times", calls)
	}
}

// A version 2 record's head tie lives in Verify, so an install with no Verify
// refuses it (a version 1 record keeps today's behaviour, tested above by every
// restore that sets none).
func TestVersionTwoRecordIsRefusedWhenThereIsNoVerify(t *testing.T) {
	c := newV2Case(t)
	c.only(t, c.v2(t, c.plain, c.plain))
	dest := filepath.Join(t.TempDir(), "state.db")
	noVerify := c.f.restoreConfig(dest)
	noVerify.Verify = nil
	_, err := Restore(context.Background(), noVerify)
	if err == nil || !strings.Contains(err.Error(), "needs a Verify") {
		t.Fatalf("a version 2 record installed with no Verify: %v", err)
	}
	if _, statErr := os.Lstat(dest); statErr == nil {
		t.Fatal("the refused install left a state file")
	}
	// Version 1 with no Verify installs as before.
	c.only(t, c.v1)
	noVerify = c.f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	noVerify.Verify = nil
	if _, err := Restore(context.Background(), noVerify); err != nil {
		t.Fatalf("version 1 with no Verify: %v", err)
	}
}

// The digest is checked before Verify runs: a record whose digest fails never
// reaches Verify, and a record whose digest holds reaches it with a file whose
// bytes are the digested pages.
func TestPageDigestIsCheckedBeforeVerifyOpensTheFile(t *testing.T) {
	c := newV2Case(t)
	bad := c.v2(t, c.plain, c.plain)
	bad.PageDigest = strings.Repeat("1", 64)
	c.only(t, bad)
	c.refused(t, "page digest") // refused asserts Verify ran zero times

	good := c.v2(t, c.plain, c.plain)
	c.only(t, good)
	cfg := c.f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	var seen string
	cfg.Verify = func(_ context.Context, rec Record, path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		d := NewPageDigest(rec.PageSize, rec.PageCount)
		_, _ = d.Write(data)
		seen = d.Sum()
		return nil
	}
	if _, err := Restore(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if seen != good.PageDigest {
		t.Fatalf("Verify saw a file digesting to %s, the record names %s", seen, good.PageDigest)
	}
}

// Extra pages: appended past the recorded count, with the record unchanged
// (the manifest no longer matches it) and with the record's geometry grown to
// match but the digest of the original pages.
func TestPageDigestRefusesExtraPages(t *testing.T) {
	c := newV2Case(t)
	ps := c.base.PageSize
	extra := append(bytes.Clone(c.plain), make([]byte, ps)...)
	t.Run("the record still names the original size", func(t *testing.T) {
		c.only(t, c.v2(t, extra, c.plain))
		c.refused(t, "manifest size does not match the record")
	})
	t.Run("the record names the longer file and the original digest", func(t *testing.T) {
		rec := c.v2(t, extra, c.plain)
		rec.PlaintextSize += int64(ps)
		rec.PageCount++
		c.only(t, rec)
		c.refused(t, "page digest")
	})
}
