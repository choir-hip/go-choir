package computerversion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestBaseBlobStoreObservationSetReadsFilesystemStore(t *testing.T) {
	version := baseSliceComputerVersion()
	root := t.TempDir()
	store := newBaseBlobStore(t, root)
	leftRef, err := store.Put([]byte("alpha"))
	if err != nil {
		t.Fatalf("put alpha: %v", err)
	}
	rightRef, err := store.Put([]byte("beta"))
	if err != nil {
		t.Fatalf("put beta: %v", err)
	}

	reopened := newBaseBlobStore(t, root)
	observations, err := BaseBlobStoreObservationSet(context.Background(), "blob-store", version, reopened, []model.BlobRef{rightRef, leftRef})
	if err != nil {
		t.Fatalf("blob observation set: %v", err)
	}
	if observations.Version != version {
		t.Fatalf("version = %#v, want %#v", observations.Version, version)
	}
	if got := observations.RequiredKinds(); len(got) != 1 || got[0] != ObservationBlobSet {
		t.Fatalf("required kinds = %#v", got)
	}
	if len(observations.Observations) != 2 {
		t.Fatalf("expected two blob observations, got %d", len(observations.Observations))
	}
	if observations.Observations[0].Key != string(leftRef) || observations.Observations[1].Key != string(rightRef) {
		t.Fatalf("observations not sorted by blob ref: %#v", observations.Observations)
	}
}

func TestBaseBlobStoreObservationSetRejectsMissingBlob(t *testing.T) {
	missing := model.BlobRef("sha256:" + strings.Repeat("0", 64))
	_, err := BaseBlobStoreObservationSet(context.Background(), "missing", baseSliceComputerVersion(), newBaseBlobStore(t, t.TempDir()), []model.BlobRef{missing})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing blob error, got %v", err)
	}
}

func TestBaseBlobStoreObservationSetRejectsCorruptBlob(t *testing.T) {
	store := newBaseBlobStore(t, t.TempDir())
	ref, err := store.Put([]byte("valid"))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	corruptBlobFile(t, store.Root(), ref)

	_, err = BaseBlobStoreObservationSet(context.Background(), "corrupt", baseSliceComputerVersion(), store, []model.BlobRef{ref})
	if err == nil || !strings.Contains(err.Error(), "blob: corrupt") {
		t.Fatalf("expected corrupt blob error, got %v", err)
	}
}

func TestBaseBlobStoreObservationSetMismatchFailsEquivalence(t *testing.T) {
	version := baseSliceComputerVersion()
	leftStore := newBaseBlobStore(t, t.TempDir())
	rightStore := newBaseBlobStore(t, t.TempDir())
	leftRef, err := leftStore.Put([]byte("left"))
	if err != nil {
		t.Fatalf("put left: %v", err)
	}
	rightRef, err := rightStore.Put([]byte("right"))
	if err != nil {
		t.Fatalf("put right: %v", err)
	}
	left, err := BaseBlobStoreObservationSet(context.Background(), "left", version, leftStore, []model.BlobRef{leftRef})
	if err != nil {
		t.Fatalf("left observations: %v", err)
	}
	right, err := BaseBlobStoreObservationSet(context.Background(), "right", version, rightStore, []model.BlobRef{rightRef})
	if err != nil {
		t.Fatalf("right observations: %v", err)
	}

	result := EquivalenceChecker{}.CheckObservationSets(left, right)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected blob mismatch to fail equivalence, got %#v", result)
	}
}

func newBaseBlobStore(t *testing.T, root string) *blob.Store {
	t.Helper()
	store, err := blob.NewStore(root)
	if err != nil {
		t.Fatalf("new blob store: %v", err)
	}
	return store
}

func corruptBlobFile(t *testing.T, root string, ref model.BlobRef) {
	t.Helper()
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	path := filepath.Join(root, hexDigest[:2], hexDigest)
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("blob file missing before corruption: %s", path)
		}
		t.Fatalf("corrupt blob file: %v", err)
	}
}
