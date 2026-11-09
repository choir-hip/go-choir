package textureowner

// textureMedia is the desk's reach to Files and the asset store (station L8).
// Ways it could fail, written before the code:
//   - a PNG the engineer saved in Files is not stored, or is stored under a
//     digest that differs from its bytes' sha256;
//   - a path that climbs out, or a symlink under Files that points outside
//     it, reads a file the owner never put in Files;
//   - a non-image (a text file, an SVG) is stored as an image;
//   - a {{file}} card is made for a missing file or a directory.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
)

func TestTextureMediaImportsAFilesImageAndRefusesTheRest(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 30, 20))); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "Reports"), 0o755))
	must(os.WriteFile(filepath.Join(root, "Reports", "chart.png"), buf.Bytes(), 0o644))
	must(os.WriteFile(filepath.Join(root, "Reports", "notes.txt"), []byte("hello"), 0o644))
	must(os.WriteFile(filepath.Join(root, "Reports", "chart.svg"), []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), 0o644))
	must(os.WriteFile(filepath.Join(root, "Reports", "summary.pdf"), []byte("%PDF-1.4"), 0o644))
	must(os.WriteFile(filepath.Join(outside, "secret.png"), buf.Bytes(), 0o644))
	must(os.Symlink(outside, filepath.Join(root, "Reports", "link")))

	m := &textureMedia{images: imageblob.New(filepath.Join(t.TempDir(), "blobs")), filesRoot: root}
	ctx := context.Background()

	digest, w, h, err := m.ImportImage(ctx, "Reports/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	if digest != hex.EncodeToString(sum[:]) || w != 30 || h != 20 || !m.HasImage(digest) {
		t.Fatalf("digest %s %dx%d has=%v", digest, w, h, m.HasImage(digest))
	}
	for path, want := range map[string]string{
		"Reports/notes.txt":       "not usable as an image",
		"Reports/chart.svg":       "SVG is not stored",
		"Reports/missing.png":     "not in Files",
		"Reports":                 "not a file",
		"Reports/link/secret.png": "outside Files",
		"../x.png":                "inside Files",
		"/etc/hosts":              "relative",
	} {
		if _, _, _, err := m.ImportImage(ctx, path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ImportImage(%q) = %v, want %q", path, err, want)
		}
	}
	mime, err := m.FileInfo(ctx, "Reports/summary.pdf")
	if err != nil || mime != "application/pdf" {
		t.Fatalf("FileInfo = %q, %v", mime, err)
	}
	for _, path := range []string{"Reports", "Reports/gone.pdf", "Reports/link/secret.png", "../x.pdf"} {
		if _, err := m.FileInfo(ctx, path); err == nil {
			t.Errorf("FileInfo(%q) accepted", path)
		}
	}
}
