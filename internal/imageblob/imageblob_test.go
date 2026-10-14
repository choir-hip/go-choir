package imageblob

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPutGetRoundTripByContentHash(t *testing.T) {
	s := New(t.TempDir())
	data := pngBytes(t)
	ref, mimeType, err := s.Put(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref, "artifact:sha256:") || mimeType != "image/png" {
		t.Fatalf("ref=%q mime=%q", ref, mimeType)
	}
	again, _, err := s.Put(data)
	if err != nil || again != ref {
		t.Fatalf("second put = %q, %v; want the same ref", again, err)
	}
	got, gotMime, err := s.Get(ref)
	if err != nil || !bytes.Equal(got, data) || gotMime != "image/png" {
		t.Fatalf("get = %d bytes, %q, %v", len(got), gotMime, err)
	}
}

// FM12: only images go in; only well-formed refs inside the store come out;
// a blob whose bytes no longer match its name is refused.
func TestStoreRefusesNonImagesBadRefsAndTamperedBlobs(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if _, _, err := s.Put([]byte("#!/bin/sh\necho not an image\n")); err == nil {
		t.Fatal("stored non-image bytes")
	}
	for _, ref := range []string{
		"artifact:sha256:../../etc/passwd",
		"sha256:" + strings.Repeat("a", 64),
		"artifact:sha256:" + strings.Repeat("A", 64),
		"artifact:sha256:" + strings.Repeat("a", 63),
	} {
		if _, _, err := s.Get(ref); err == nil {
			t.Fatalf("Get(%q) succeeded", ref)
		}
	}
	ref, _, err := s.Put(pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.TrimPrefix(ref, "artifact:sha256:")
	if err := os.WriteFile(filepath.Join(dir, "sha256", digest), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get(ref); err == nil || !strings.Contains(err.Error(), "content hash") {
		t.Fatalf("tampered blob err = %v", err)
	}
}

func TestDirForStorePath(t *testing.T) {
	if got := DirForStorePath("/mnt/persistent/state"); got != "/mnt/persistent/state.image-blobs" {
		t.Fatalf("dir = %q", got)
	}
	if DirForStorePath("") != "" {
		t.Fatal("empty store path should give no dir")
	}
}
