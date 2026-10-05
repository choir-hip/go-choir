package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The artifact GC must only remove entries that are both unreachable and past
// grace. A live ref, an in-grace file, and a dry-run all must leave the file.
func TestRunArtifactGCDryRunAndActive(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)

	write := func(ns, name string) string {
		dir := filepath.Join(artifactsRoot, "sha256", ns)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	digestOf := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}

	// file-cas-roots: live = manifest_ref basename rows in computer_file_roots.
	liveRoot := digestOf("live-root")
	deadRoot := digestOf("dead-root")
	manifestRef := "sha256/file-cas-roots/computer-a/" + liveRoot + ".json"
	if err := store.RecordFileRoot(ctx, "computer-a", liveRoot, manifestRef, 1); err != nil {
		t.Fatal(err)
	}
	liveRootPath := write("file-cas-roots/computer-a", liveRoot+".json")
	deadRootPath := write("file-cas-roots/computer-a", deadRoot+".json")

	// projection-base: live = computer_replay_watermarks.base_ref (+ sidecar).
	liveBase := digestOf("live-base")
	deadBase := digestOf("dead-base")
	if err := store.RecordReplayWatermark(ctx, "computer-b", 5, liveBase); err != nil {
		t.Fatal(err)
	}
	liveBlobPath := write("projection-base", liveBase)
	liveSidecarPath := write("projection-base", liveBase+".descriptor.json")
	deadBlobPath := write("projection-base", deadBase)
	deadSidecarPath := write("projection-base", deadBase+".descriptor.json")

	// Dry-run: reports unreachable but deletes nothing.
	dry, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "dry-run", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Namespaces["file-cas-roots"].Deleted == 0 || dry.Namespaces["projection-base"].Deleted == 0 {
		t.Fatalf("dry-run reported no candidates: %+v", dry.Namespaces)
	}
	for _, p := range []string{liveRootPath, deadRootPath, liveBlobPath, liveSidecarPath, deadBlobPath, deadSidecarPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("dry-run deleted %s: %v", p, err)
		}
	}

	// Active: unreachable + past grace deleted; live and sidecars kept.
	active, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{liveRootPath, liveBlobPath, liveSidecarPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("live artifact removed %s: %v", p, err)
		}
	}
	for _, p := range []string{deadRootPath, deadBlobPath, deadSidecarPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dead artifact kept %s: %v", p, err)
		}
	}
	if active.Deleted == 0 {
		t.Fatal("active sweep deleted nothing")
	}
}

// An in-flight write newer than the grace cutoff is never collected even when
// unreachable — pin-before-CAS safety.
func TestRunArtifactGCKeepsInGraceUnreachable(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	dir := filepath.Join(artifactsRoot, "sha256", "projection-base")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	inFlight := filepath.Join(dir, "00aa11bb22cc33dd44ee55ff66778899aabbccddeeff00112233445566778899")
	if err := os.WriteFile(inFlight, []byte("in flight"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inFlight); err != nil {
		t.Fatalf("in-flight write collected: %v", err)
	}
	if report.Namespaces["projection-base"].InGrace == 0 {
		t.Fatalf("in-grace not counted: %+v", report.Namespaces)
	}
}
