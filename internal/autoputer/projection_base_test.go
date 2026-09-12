package autoputer

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsStoreEmpty(t *testing.T) {
	dir := t.TempDir()
	if !isStoreEmpty(dir) {
		t.Fatalf("expected empty temp dir to be empty")
	}

	// Create non-empty marker like .dolt
	if err := os.Mkdir(filepath.Join(dir, ".dolt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if isStoreEmpty(dir) {
		t.Fatalf("expected dir with .dolt to be non-empty")
	}
}

func TestMaterializeProjectionBaseIfNeeded(t *testing.T) {
	storeDir := t.TempDir()
	computerID := "test-computer-1"

	// Create a dummy tar archive representing a ProjectionBase
	var tarBuf strings.Builder
	hasher := sha256.New()
	mw := io.MultiWriter(&tarBuf, hasher)
	tw := tar.NewWriter(mw)

	content := []byte("schema_version=1\n")
	hdr := &tar.Header{
		Name: "runtime.db",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	tarBytes := []byte(tarBuf.String())
	digest := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-cap" {
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/internal/computers/files/watermark" {
			_ = json.NewEncoder(w).Encode(watermarkResponse{
				WatermarkSequence: 100,
				BaseRef:           digest,
			})
			return
		}
		if r.URL.Path == "/internal/computers/events/payload" {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarBytes)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	capability := func(ctx context.Context) (string, error) {
		return "test-cap", nil
	}

	materialized, err := materializeProjectionBaseIfNeeded(context.Background(), storeDir, computerID, server.URL, capability)
	if err != nil {
		t.Fatalf("materializeProjectionBaseIfNeeded failed: %v", err)
	}
	if !materialized {
		t.Fatalf("expected materialized to be true")
	}

	// Verify unpacked file exists
	unpacked := filepath.Join(storeDir, "runtime.db")
	data, err := os.ReadFile(unpacked)
	if err != nil {
		t.Fatalf("read unpacked file failed: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("unpacked content mismatch: got %q, want %q", string(data), string(content))
	}

	// Second run should short-circuit because storeDir is now non-empty
	materialized2, err := materializeProjectionBaseIfNeeded(context.Background(), storeDir, computerID, server.URL, capability)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if materialized2 {
		t.Fatalf("expected second call to short-circuit and return false")
	}
}
