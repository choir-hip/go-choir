package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestPutGetRoundTrip(t *testing.T) {
	s := tempStore(t)
	data := []byte("hello choir base blobs")
	ref, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !ref.Valid() {
		t.Fatalf("ref invalid: %q", ref)
	}
	sum := sha256.Sum256(data)
	wantHex := hex.EncodeToString(sum[:])
	if string(ref) != "sha256:"+wantHex {
		t.Fatalf("ref: got %q want sha256:%s", ref, wantHex)
	}

	got, err := s.Get(ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data: got %q want %q", got, data)
	}
}

func TestPutIdempotent(t *testing.T) {
	s := tempStore(t)
	data := []byte("idempotent content")
	ref1, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put1: %v", err)
	}
	ref2, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put2: %v", err)
	}
	if ref1 != ref2 {
		t.Fatalf("refs differ: %q vs %q", ref1, ref2)
	}
	// Only one file should exist on disk.
	hexDigest := strings.TrimPrefix(string(ref1), "sha256:")
	path := filepath.Join(s.Root(), hexDigest[:2], hexDigest)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat blob file: %v", err)
	}
}

func TestHas(t *testing.T) {
	s := tempStore(t)
	ref, err := s.Put([]byte("has-me"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	ok, err := s.Has(ref)
	if err != nil {
		t.Fatalf("Has: %v", err)
	}
	if !ok {
		t.Fatal("Has returned false for existing blob")
	}
	missing := model.BlobRef("sha256:" + strings.Repeat("0", 64))
	ok, err = s.Has(missing)
	if err != nil {
		t.Fatalf("Has missing: %v", err)
	}
	if ok {
		t.Fatal("Has returned true for missing blob")
	}
}

func TestGetNotFound(t *testing.T) {
	s := tempStore(t)
	missing := model.BlobRef("sha256:" + strings.Repeat("a", 64))
	_, err := s.Get(missing)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCorruptDetection(t *testing.T) {
	s := tempStore(t)
	data := []byte("original content")
	ref, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Corrupt the file on disk.
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	path := filepath.Join(s.Root(), hexDigest[:2], hexDigest)
	if err := os.WriteFile(path, []byte("tampered!"), 0o644); err != nil {
		t.Fatalf("corrupt file: %v", err)
	}
	_, err = s.Get(ref)
	if !errors.Is(err, ErrCorruptBlob) {
		t.Fatalf("expected ErrCorruptBlob, got %v", err)
	}
}

func TestStat(t *testing.T) {
	s := tempStore(t)
	data := []byte("stat me")
	ref, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	b, err := s.Stat(ref)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if b.BlobRef != ref {
		t.Fatalf("BlobRef: got %q want %q", b.BlobRef, ref)
	}
	if b.SizeBytes != int64(len(data)) {
		t.Fatalf("SizeBytes: got %d want %d", b.SizeBytes, len(data))
	}
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	if b.SHA256 != hexDigest {
		t.Fatalf("SHA256: got %q want %q", b.SHA256, hexDigest)
	}
}

func TestSharding(t *testing.T) {
	s := tempStore(t)
	// Put several blobs and confirm they land in different shard dirs.
	refs := make([]model.BlobRef, 0, 10)
	for i := 0; i < 10; i++ {
		ref, err := s.Put([]byte{byte(i)})
		if err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
		refs = append(refs, ref)
	}
	shards := make(map[string]bool)
	for _, ref := range refs {
		hexDigest := strings.TrimPrefix(string(ref), "sha256:")
		shards[hexDigest[:2]] = true
	}
	if len(shards) < 2 {
		t.Fatalf("expected multiple shard dirs, got %d", len(shards))
	}
}

func TestNewStoreEmptyDir(t *testing.T) {
	_, err := NewStore("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestGetInvalidRef(t *testing.T) {
	s := tempStore(t)
	_, err := s.Get(model.BlobRef("not-a-ref"))
	if err == nil {
		t.Fatal("expected error for invalid ref")
	}
}
