package imageblob

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The asset view of the store (docs/reports/texture-media-and-autoradio-
// design-2026-10-20.md section 4): a digest names bytes, the store says what
// kind they are and what Content-Type serves them, and it re-hashes on read.
// Only images are served now; audio and video are typed and refused.

func TestAssetReadsByDigestWithItsKindAndContentType(t *testing.T) {
	s := New(t.TempDir())
	data := pngBytes(t)
	ref, _, err := s.Put(data)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.TrimPrefix(ref, "artifact:sha256:")
	asset, err := s.Asset(digest)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Kind != KindImage || asset.ContentType != "image/png" || asset.Digest != digest || asset.Ref != ref || string(asset.Data) != string(data) {
		t.Fatalf("asset = %+v", asset)
	}
	if !s.Has(digest) || s.Has(strings.Repeat("0", 64)) || s.Has("../x") {
		t.Fatal("Has misreports presence")
	}
}

func TestAssetRefusesBadDigestsMissingTamperedAndUnservableBytes(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ref, _, err := s.Put(pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.TrimPrefix(ref, "artifact:sha256:")

	for _, bad := range []string{"", "../../etc/passwd", strings.ToUpper(digest), digest[:63], digest + "0", digest[:63] + "g", "sha256/" + digest} {
		if _, err := s.Asset(bad); !errors.Is(err, ErrBadDigest) {
			t.Errorf("Asset(%q) = %v, want ErrBadDigest", bad, err)
		}
	}
	if _, err := s.Asset(strings.Repeat("0", 64)); !errors.Is(err, ErrNotFound) {
		t.Errorf("a digest not in the store = %v, want ErrNotFound", err)
	}

	// Bytes that no longer match their name are never served.
	if err := os.WriteFile(filepath.Join(dir, "sha256", digest), []byte("swapped"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Asset(digest); !errors.Is(err, ErrCorrupt) {
		t.Errorf("tampered blob = %v, want ErrCorrupt", err)
	}

	// Bytes that hash right but are not a servable kind (an audio or video
	// file placed here by hand) are refused, not sniffed into a type.
	text := []byte("RIFF....WAVEfmt not an image")
	other := filepath.Join(dir, "sha256", sha256Hex(text))
	if err := os.WriteFile(other, text, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Asset(sha256Hex(text)); !errors.Is(err, ErrKindNotSupported) {
		t.Errorf("unservable kind = %v, want ErrKindNotSupported", err)
	}
}

func TestMediaKindsBeyondImagesAreTypedButNotYetSupported(t *testing.T) {
	if !KindImage.Supported() {
		t.Fatal("images must be supported")
	}
	for _, k := range []Kind{KindAudio, KindVideo} {
		if k.Supported() {
			t.Errorf("%s is a typed TODO (texture media design T1/T4), not yet supported", k)
		}
	}
	if KindOfContentType("image/png") != KindImage || KindOfContentType("video/mp4") != KindVideo || KindOfContentType("audio/mpeg") != KindAudio || KindOfContentType("text/plain") != "" {
		t.Fatal("KindOfContentType maps the wrong kinds")
	}
}
