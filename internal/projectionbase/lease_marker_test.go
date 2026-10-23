package projectionbase

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The stream_seq lease's untrust marker at the projection-base swap points
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 3;
// tests 3, 5). Ways it can fail, written before the code:
//  1. A base is published without the marker, so a node that installs it
//     trusts a lease another lineage outran (the epoch-2 interlude).
//  2. Only one install entry point marks: InstallPinnedBase (a checkpoint's
//     pinned base) is installDescriptor's other caller.
//  3. Rebase flips an unmarked workspace into the live path.
//  4. The marker is installed and the first allocation of the installed store
//     still trusts a lease (the base carried one) and reuses a number.

type leaseBaseFixture struct {
	ctx        context.Context
	computerID string
	digests    []string
	src        *fakeBaseSource
}

func newLeaseBaseFixture(t *testing.T) *leaseBaseFixture {
	t.Helper()
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-lease-marker"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 11)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)
	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(artifactsRoot, computerID, digests[0])))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	return &leaseBaseFixture{ctx: ctx, computerID: computerID, digests: digests,
		src: &fakeBaseSource{sequence: 1, baseRef: result.Descriptor.BlobSHA256, descriptor: result.Descriptor, blob: blob, disk: disk}}
}

func requireMarker(t *testing.T, storePath, what string) {
	t.Helper()
	present, err := storeschema.LeaseUntrusted(choirstore.TextureWorkspacePath(storePath))
	if err != nil || !present {
		t.Fatalf("%s left no lease untrust marker in the workspace (present=%v err=%v)", what, present, err)
	}
}

// A published base carries the marker as a tar entry in its workspace
// subtree, whatever the workspace held.
func TestPublishedBaseCarriesTheLeaseUntrustMarker(t *testing.T) {
	fx := newLeaseBaseFixture(t)
	reader := tar.NewReader(bytes.NewReader(fx.src.blob))
	var entries []string
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, header.Name)
		if path.Base(header.Name) == storeschema.LeaseUntrustedFile {
			body, _ := io.ReadAll(reader)
			if !strings.Contains(string(body), "base") {
				t.Fatalf("the blob's marker does not say it is a base: %s", body)
			}
			if path.Dir(header.Name) == "." {
				t.Fatalf("the marker is at the blob root, not inside the workspace: %s", header.Name)
			}
			return
		}
	}
	t.Fatalf("the published base carries no %s; entries:\n%s", storeschema.LeaseUntrustedFile, strings.Join(entries, "\n"))
}

// Publishing marks the blob and not the scratch store it was packed from: the
// marker is a tar entry, and a directory with no receipt is left alone.
func TestPublishDirDoesNotMarkADirectoryWithoutAStoreReceipt(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "plain", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, _, err := NewPublisher(filepath.Join(root, "artifacts")).PublishDir(src)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(filepath.Join(root, "artifacts", "sha256", Namespace, digest))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(blob))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(header.Name, storeschema.LeaseUntrustedFile) {
			t.Fatalf("a directory that is not a store workspace was marked: %s", header.Name)
		}
	}
	if _, err := os.Stat(filepath.Join(src, "plain", storeschema.LeaseUntrustedFile)); err == nil {
		t.Fatal("PublishDir wrote the marker into its source directory")
	}
}

// Test 5. InstallVerifiedBase leaves the marker in the installed workspace.
func TestInstallVerifiedBaseLeavesTheMarker(t *testing.T) {
	fx := newLeaseBaseFixture(t)
	installDir := t.TempDir()
	if _, err := InstallVerifiedBase(fx.ctx, fx.src, installDir, "runtime.db", fx.computerID, fx.digests[2], 3); err != nil {
		t.Fatal(err)
	}
	requireMarker(t, filepath.Join(installDir, "runtime.db"), "InstallVerifiedBase")
}

// Test 5. InstallPinnedBase, as well as InstallVerifiedBase, leaves it.
func TestInstallPinnedBaseLeavesTheMarker(t *testing.T) {
	fx := newLeaseBaseFixture(t)
	installDir := t.TempDir()
	if _, err := InstallPinnedBase(fx.ctx, fx.src, installDir, "runtime.db", fx.computerID, fx.src.baseRef, fx.digests[2], 3); err != nil {
		t.Fatal(err)
	}
	requireMarker(t, filepath.Join(installDir, "runtime.db"), "InstallPinnedBase")
}

// RebaseRetainedStore flips a staged workspace into the live path; the marker
// is in the staged workspace before the flip and survives it.
func TestRebaseRetainedStoreLeavesTheMarker(t *testing.T) {
	fx := newLeaseBaseFixture(t)
	liveDir := t.TempDir()
	if _, err := InstallVerifiedBase(fx.ctx, fx.src, liveDir, "runtime.db", fx.computerID, fx.digests[2], 3); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(liveDir, "runtime.db")
	// A computer that had been running: the marker is gone, as after an exact
	// seed cleared it.
	if err := storeschema.ClearLeaseUntrusted(choirstore.TextureWorkspacePath(storePath)); err != nil {
		t.Fatal(err)
	}
	if _, err := RebaseRetainedStore(fx.ctx, fx.src, storePath, fx.computerID, fx.digests[2], 3); err != nil {
		t.Fatalf("rebase refused: %v", err)
	}
	requireMarker(t, storePath, "RebaseRetainedStore")
}

// Test 3 (the base half). An installed base opens untrusted, and its first
// allocation is the exact seed, whatever lease the base carried: the lease is
// in the base's rows, the marker in its workspace.
func TestInstalledBaseOpensUntrustedAndSeedsExact(t *testing.T) {
	fx := newLeaseBaseFixture(t)
	installDir := t.TempDir()
	if _, err := InstallVerifiedBase(fx.ctx, fx.src, installDir, "runtime.db", fx.computerID, fx.digests[2], 3); err != nil {
		t.Fatal(err)
	}
	s, err := choirstore.Open(filepath.Join(installDir, "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.AllocateEventSeq(fx.ctx, newLeaseProbeEvent()); err != nil {
		t.Fatal(err)
	}
	st := s.StreamSeqLeaseStats()
	if st.Trusted || st.SeededFrom != "exact" {
		t.Fatalf("an installed base seeded from %s (trusted=%v, reason %q), want the exact seed", st.SeededFrom, st.Trusted, st.UntrustReason)
	}
	// The base carries no lease row (the rebuilder replays a tape with no
	// lease ops), and no_lease is reported before the marker.
	if st.UntrustReason != "no_lease" {
		t.Fatalf("untrust reason = %q, want no_lease", st.UntrustReason)
	}
}

func newLeaseProbeEvent() *types.EventRecord {
	return &types.EventRecord{EventID: "lease-probe", RunID: "run-probe", OwnerID: "owner-probe", Kind: types.EventRunProgress}
}
