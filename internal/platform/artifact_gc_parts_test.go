package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// S4 (docs/problems/paper-never-checkpointed-2026-10-27.md): a parts-form
// base is a manifest blob naming content-addressed parts under
// projection-base-parts. GC keeps every part some on-disk manifest names
// and sweeps an old part no manifest names. No base is live any more (the host
// keeps none), so every manifest is collected, and its parts outlive it by one
// run: the manifest is still on disk when this run's part live set is read.
// Ways this could fail:
//  1. a part is deleted in the same run that deletes its only manifest;
//  2. a part survives the run after its last manifest is gone;
//  3. an orphan part (no manifest) survives.
func TestRunArtifactGCKeepsPartsNamedByAnyManifest(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)
	digestOf := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	write := func(ns, name string, body []byte) string {
		dir := filepath.Join(artifactsRoot, "sha256", ns)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	manifest := func(parts ...string) []byte {
		var b strings.Builder
		b.WriteString(`{"format":"choir.projection-base.manifest/v1","entries":[`)
		for i, p := range parts {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"path":"f%d","type":"file","mode":420,"size":1,"sha256":"%s"}`, i, p)
		}
		b.WriteString("]}")
		return []byte(b.String())
	}
	livePart, sharedPart, deadOnlyPart, orphanPart := digestOf("live"), digestOf("shared"), digestOf("dead-only"), digestOf("orphan")
	liveManifest := manifest(livePart, sharedPart)
	deadManifest := manifest(sharedPart, deadOnlyPart)
	liveBase, deadBase := digestOf(string(liveManifest)), digestOf(string(deadManifest))
	seedReplayWatermark(t, store, "computer-parts", 7, liveBase)
	liveBlob := write("projection-base", liveBase, liveManifest)
	deadBlob := write("projection-base", deadBase, deadManifest)
	paths := map[string]string{}
	for _, p := range []string{livePart, sharedPart, deadOnlyPart, orphanPart} {
		paths[p] = write("projection-base-parts", p, []byte("x"))
	}

	if _, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}
	for _, blob := range []string{deadBlob, liveBlob} {
		if _, err := os.Stat(blob); !os.IsNotExist(err) {
			t.Fatalf("manifest blob kept (no base is live): %v", err)
		}
	}
	for _, p := range []string{livePart, sharedPart, deadOnlyPart} {
		if _, err := os.Stat(paths[p]); err != nil {
			t.Fatalf("part named by an on-disk manifest deleted: %v", err)
		}
	}
	if _, err := os.Stat(paths[orphanPart]); !os.IsNotExist(err) {
		t.Fatalf("orphan part kept: %v", err)
	}

	// Next run: every manifest is gone, so every part is unreachable.
	if _, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{livePart, sharedPart, deadOnlyPart} {
		if _, err := os.Stat(paths[p]); !os.IsNotExist(err) {
			t.Fatalf("part of a deleted base kept: %v", err)
		}
	}
}

// A manifest-prefixed blob that does not parse makes the parts live set
// unprovable: the sweep refuses rather than deleting parts it may name.
func TestRunArtifactGCRefusesOnTornManifest(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)
	baseDir := filepath.Join(artifactsRoot, "sha256", "projection-base")
	partsDir := filepath.Join(artifactsRoot, "sha256", "projection-base-parts")
	for _, d := range []string{baseDir, partsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	torn := filepath.Join(baseDir, strings.Repeat("c", 64))
	if err := os.WriteFile(torn, []byte(`{"format":"choir.projection-base.manifest/v1","entries":[`), 0o644); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(partsDir, strings.Repeat("d", 64))
	if err := os.WriteFile(part, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(part, old, old)
	if _, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err == nil {
		t.Fatal("sweep ran with an unprovable parts live set")
	}
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("part deleted under a torn manifest: %v", err)
	}
}

// Review 370 (Devin 1): rolling back S4 drops the last manifest, and the
// parts it named must then be swept, not wedge behind the empty-live-set
// guard. The parts live set is read off the disk (not a database), so a
// clean scan of an existing base directory that names no part is proof. A
// base directory that does not exist proves nothing and keeps the guard.
func TestRunArtifactGCSweepsPartsWhenNoManifestRemains(t *testing.T) {
	for _, tc := range []struct {
		name      string
		baseDir   bool
		wantSwept bool
	}{{"base dir scanned, no manifests", true, true}, {"base dir absent", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			store, root := openTestPlatformStore(t)
			ensureGCRouteTables(t, store)
			artifactsRoot := filepath.Join(root, "artifacts")
			service := NewService(store, artifactsRoot, "")
			old := time.Now().Add(-2 * time.Hour)
			if tc.baseDir {
				if err := os.MkdirAll(filepath.Join(artifactsRoot, "sha256", "projection-base"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(artifactsRoot, "sha256", "projection-base-parts")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			part := filepath.Join(dir, strings.Repeat("c", 64))
			if err := os.WriteFile(part, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(part, old, old); err != nil {
				t.Fatal(err)
			}
			if _, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(part)
			if swept := os.IsNotExist(err); swept != tc.wantSwept {
				t.Fatalf("part swept = %v (stat err %v), want %v", swept, err, tc.wantSwept)
			}
		})
	}
}
