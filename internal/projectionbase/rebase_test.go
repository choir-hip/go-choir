package projectionbase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestRebaseRetainedStoreReplacesPrefixWithoutInPlaceOverwrite(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-rebase-retained"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 9)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)

	rebuild := func(target string) *fakeBaseSource {
		t.Helper()
		rebuilder, err := NewRebuilder(Config{
			ComputerID: computerID, TargetHead: target, ArtifactsRoot: artifactsRoot,
			ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := rebuilder.Run(ctx, disk)
		if err != nil {
			t.Fatal(err)
		}
		blob, err := os.ReadFile(result.BlobPath)
		if err != nil {
			t.Fatal(err)
		}
		return &fakeBaseSource{sequence: result.Descriptor.Sequence, baseRef: result.Descriptor.BlobSHA256, descriptor: result.Descriptor, blob: blob, disk: disk}
	}

	w1 := rebuild(digests[0])
	liveDir := t.TempDir()
	if _, err := InstallVerifiedBase(ctx, w1, liveDir, "runtime.db", computerID, digests[2], 3); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(liveDir, "runtime.db")
	originalWorkspace := choirstore.TextureWorkspacePath(storePath)

	w2 := rebuild(digests[1])
	descriptor, err := RebaseRetainedStore(ctx, w2, storePath, computerID, digests[2], 3)
	if err != nil {
		t.Fatalf("rebase refused: %v", err)
	}
	if descriptor.Sequence != 2 {
		t.Fatalf("rebased W = %d, want 2", descriptor.Sequence)
	}

	installed, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = installed.Close() })
	head, err := installed.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("rebased head unreadable: %v", err)
	}
	if head.Sequence != 2 {
		t.Fatalf("rebased head sequence %d, want W=2 (prefix skipped)", head.Sequence)
	}
	if _, err := os.Stat(originalWorkspace); err != nil {
		t.Fatalf("flipped workspace missing: %v", err)
	}
	entries, err := os.ReadDir(liveDir)
	if err != nil {
		t.Fatal(err)
	}
	foundQuarantine := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "restore-quarantine-") {
			foundQuarantine = true
		}
		if strings.HasPrefix(entry.Name(), "restore-staging-") {
			t.Fatalf("staging dir leaked: %s", entry.Name())
		}
	}
	if !foundQuarantine {
		t.Fatal("original realization was not quarantined")
	}
}
