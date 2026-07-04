package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

func TestBaseCurrentStateObservationSetBindsJournalTreeAndBlob(t *testing.T) {
	version := baseSliceComputerVersion()
	blobs := newBaseBlobStore(t, t.TempDir())
	ref, contentHash := putBaseBlob(t, blobs, []byte("hello audited computer"))
	jr := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, ref, contentHash))

	observations, err := BaseCurrentStateObservationSet(context.Background(), "base-current", version, jr, blobs)
	if err != nil {
		t.Fatalf("current state observations: %v", err)
	}
	kinds := observations.RequiredKinds()
	if len(kinds) != 2 || kinds[0] != ObservationBlobSet || kinds[1] != ObservationFileManifest {
		t.Fatalf("required kinds = %#v", kinds)
	}
	if len(observations.Observations) != 2 {
		t.Fatalf("expected file manifest and blob observation, got %#v", observations.Observations)
	}
}

func TestOpenBaseCurrentStateSourceLoadsExistingPathsReadOnly(t *testing.T) {
	version := baseSliceComputerVersion()
	root := t.TempDir()
	blobRoot := filepath.Join(root, "blobs")
	blobs := newBaseBlobStore(t, blobRoot)
	ref, contentHash := putBaseBlob(t, blobs, []byte("from existing paths"))
	journalPath := filepath.Join(root, "journal.sqlite")
	jr := newSQLiteJournalAtPathWithEvent(t, journalPath, baseCreateEventWithBlob(1, ref, contentHash))
	if err := jr.Close(); err != nil {
		t.Fatalf("close writable journal: %v", err)
	}

	source, err := OpenBaseCurrentStateSource(BaseCurrentStatePaths{JournalPath: journalPath, BlobRoot: blobRoot})
	if err != nil {
		t.Fatalf("open current state source: %v", err)
	}
	defer source.Close()
	observations, err := source.ObservationSet(context.Background(), "base-current-source", version)
	if err != nil {
		t.Fatalf("source observations: %v", err)
	}
	if got := observations.RequiredKinds(); len(got) != 2 || got[0] != ObservationBlobSet || got[1] != ObservationFileManifest {
		t.Fatalf("required kinds = %#v", got)
	}
	if _, err := source.journal.Append(baseCreateEventWithBlob(2, ref, contentHash)); err == nil {
		t.Fatal("expected read-only source journal append to fail")
	}
}

func TestOpenBaseCurrentStateSourceDoesNotCreateMissingBlobRoot(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "journal.sqlite")
	jr := newSQLiteJournalAtPathWithEvent(t, journalPath, baseCreateEvent(1, "a"))
	if err := jr.Close(); err != nil {
		t.Fatalf("close writable journal: %v", err)
	}
	missingBlobRoot := filepath.Join(root, "missing-blobs")

	_, err := OpenBaseCurrentStateSource(BaseCurrentStatePaths{JournalPath: journalPath, BlobRoot: missingBlobRoot})
	if err == nil {
		t.Fatal("expected missing blob root to fail")
	}
	if _, statErr := os.Stat(missingBlobRoot); !os.IsNotExist(statErr) {
		t.Fatalf("loader created missing blob root or unexpected stat error: %v", statErr)
	}
}

func TestBaseCurrentStateObservationSetRejectsMissingReferencedBlob(t *testing.T) {
	missing := model.BlobRef("sha256:" + strings.Repeat("0", 64))
	jr := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, missing, strings.Repeat("0", 64)))

	_, err := BaseCurrentStateObservationSet(context.Background(), "missing-blob", baseSliceComputerVersion(), jr, newBaseBlobStore(t, t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing referenced blob to be rejected, got %v", err)
	}
}

func TestBaseCurrentStateObservationSetMismatchFailsEquivalence(t *testing.T) {
	version := baseSliceComputerVersion()
	leftBlobs := newBaseBlobStore(t, t.TempDir())
	leftRef, leftHash := putBaseBlob(t, leftBlobs, []byte("left content"))
	leftJournal := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, leftRef, leftHash))
	left, err := BaseCurrentStateObservationSet(context.Background(), "left", version, leftJournal, leftBlobs)
	if err != nil {
		t.Fatalf("left observations: %v", err)
	}

	rightBlobs := newBaseBlobStore(t, t.TempDir())
	rightRef, rightHash := putBaseBlob(t, rightBlobs, []byte("right content"))
	rightJournal := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, rightRef, rightHash))
	right, err := BaseCurrentStateObservationSet(context.Background(), "right", version, rightJournal, rightBlobs)
	if err != nil {
		t.Fatalf("right observations: %v", err)
	}

	result := EquivalenceChecker{}.CheckObservationSets(left, right)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected composite mismatch to fail equivalence, got %#v", result)
	}
}

func newSQLiteJournalWithEvent(t *testing.T, event model.Event) *journal.SQLiteJournal {
	t.Helper()
	jr := newSQLiteJournalAtPathWithEvent(t, t.TempDir()+"/base.sqlite", event)
	t.Cleanup(func() {
		if err := jr.Close(); err != nil {
			t.Fatalf("close sqlite journal: %v", err)
		}
	})
	return jr
}

func newSQLiteJournalAtPathWithEvent(t *testing.T, path string, event model.Event) *journal.SQLiteJournal {
	t.Helper()
	jr, err := journal.NewSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open sqlite journal: %v", err)
	}
	if _, err := jr.Append(event); err != nil {
		_ = jr.Close()
		t.Fatalf("append event: %v", err)
	}
	return jr
}

func putBaseBlob(t *testing.T, store interface {
	Put([]byte) (model.BlobRef, error)
}, data []byte) (model.BlobRef, string) {
	t.Helper()
	ref, err := store.Put(data)
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	sum := sha256.Sum256(data)
	return ref, hex.EncodeToString(sum[:])
}

func baseCreateEventWithBlob(seq int64, ref model.BlobRef, contentHash string) model.Event {
	payload := basetree.Payload{
		Name:         "notes.md",
		ParentItemID: "base_item_root",
		Kind:         model.KindFile,
		VersionID:    model.VersionID("base_ver_notes_blob"),
		BlobRef:      ref,
		ContentHash:  contentHash,
	}
	return model.Event{
		EventID:     "base_evt_blob_create",
		OwnerID:     "owner",
		ItemID:      "base_item_notes_blob",
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventCreate,
		Kind:        model.KindFile,
		BlobRef:     ref,
		CursorSeq:   seq,
		PayloadJSON: payload.JSON(),
		CreatedAt:   baseSliceTime,
	}
}
