package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

// fixedTime is a deterministic timestamp for all generator tests.
var generatorFixedTime = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

// mkCreateEvent builds a create event with a payload describing a file item
// whose content is content (the blob ref is derived from content).
func mkCreateEvent(eid, iid, parent, name string, kind model.ItemKind, vid string, content []byte) model.Event {
	hash := sha256Hex(content)
	p := basetree.Payload{
		Name:         name,
		ParentItemID: model.ItemID(parent),
		Kind:         kind,
		VersionID:    model.VersionID(vid),
		BlobRef:      model.BlobRef("sha256:" + hash),
		ContentHash:  hash,
	}
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventCreate,
		Kind:        kind,
		CursorSeq:   0, // assigned by journal.Append
		PayloadJSON: p.JSON(),
		CreatedAt:   generatorFixedTime,
	}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// buildGeneratorFixture creates a MemJournal and blob store populated with a
// small filesystem tree:
//
//	config.yaml       (file, "key: value\n")
//	data/             (folder)
//	data/notes.txt    (file, "hello world\n")
//	data/settings.json (file, `{"enabled":true}`)
//
// It returns the journal, blob store, and the ComputerVersion that identifies
// this state.
func buildGeneratorFixture(t *testing.T) (journal.Journal, *blob.Store, ComputerVersion) {
	t.Helper()

	jrn := journal.NewMemJournal()
	blobDir := t.TempDir()
	blobs, err := blob.NewStore(blobDir)
	if err != nil {
		t.Fatalf("create blob store: %v", err)
	}

	// Put file contents into the blob store.
	configContent := []byte("key: value\n")
	notesContent := []byte("hello world\n")
	settingsContent := []byte(`{"enabled":true}`)

	configRef, err := blobs.Put(configContent)
	if err != nil {
		t.Fatalf("put config blob: %v", err)
	}
	notesRef, err := blobs.Put(notesContent)
	if err != nil {
		t.Fatalf("put notes blob: %v", err)
	}
	settingsRef, err := blobs.Put(settingsContent)
	if err != nil {
		t.Fatalf("put settings blob: %v", err)
	}

	// Build events. The journal assigns CursorSeq and ParentEventID.
	// Folder: data/
	folderEvent := mkCreateEvent("base_evt_folder_1", "base_item_folder_data", "", "data", model.KindFolder, "base_ver_folder_1", nil)
	folderEvent.PayloadJSON = basetree.Payload{
		Name:         "data",
		Kind:         model.KindFolder,
		VersionID:    "base_ver_folder_1",
		ParentItemID: "",
	}.JSON()
	if _, err := jrn.Append(folderEvent); err != nil {
		t.Fatalf("append folder event: %v", err)
	}

	// File: config.yaml
	configEvent := mkCreateEvent("base_evt_config_1", "base_item_config", "", "config.yaml", model.KindFile, "base_ver_config_1", configContent)
	configEvent.PayloadJSON = basetree.Payload{
		Name:         "config.yaml",
		Kind:         model.KindFile,
		VersionID:    "base_ver_config_1",
		BlobRef:      configRef,
		ContentHash:  sha256Hex(configContent),
		ParentItemID: "",
	}.JSON()
	if _, err := jrn.Append(configEvent); err != nil {
		t.Fatalf("append config event: %v", err)
	}

	// File: data/notes.txt
	notesEvent := mkCreateEvent("base_evt_notes_1", "base_item_notes", "base_item_folder_data", "notes.txt", model.KindFile, "base_ver_notes_1", notesContent)
	notesEvent.PayloadJSON = basetree.Payload{
		Name:         "notes.txt",
		Kind:         model.KindFile,
		VersionID:    "base_ver_notes_1",
		BlobRef:      notesRef,
		ContentHash:  sha256Hex(notesContent),
		ParentItemID: "base_item_folder_data",
	}.JSON()
	if _, err := jrn.Append(notesEvent); err != nil {
		t.Fatalf("append notes event: %v", err)
	}

	// File: data/settings.json
	settingsEvent := mkCreateEvent("base_evt_settings_1", "base_item_settings", "base_item_folder_data", "settings.json", model.KindFile, "base_ver_settings_1", settingsContent)
	settingsEvent.PayloadJSON = basetree.Payload{
		Name:         "settings.json",
		Kind:         model.KindFile,
		VersionID:    "base_ver_settings_1",
		BlobRef:      settingsRef,
		ContentHash:  sha256Hex(settingsContent),
		ParentItemID: "base_item_folder_data",
	}.JSON()
	if _, err := jrn.Append(settingsEvent); err != nil {
		t.Fatalf("append settings event: %v", err)
	}

	version := ComputerVersion{
		CodeRef:            CodeRef("code-abc-123"),
		ArtifactProgramRef: ArtifactProgramRef("artifact-xyz-789"),
	}

	return jrn, blobs, version
}

// TestStateGeneratorRoundTrip proves the generator works: it takes a
// ComputerVersion + journal + blob store and writes concrete filesystem state
// to a directory. The FirecrackerStateExtractor then reads that directory back
// and produces observations. The test verifies the generated files match the
// expected content and that the extractor reads them back correctly.
func TestStateGeneratorRoundTrip(t *testing.T) {
	jrn, blobs, version := buildGeneratorFixture(t)

	// Generate state into a target directory (simulating a Firecracker
	// persistent dir).
	firecrackerDir := t.TempDir()
	ctx := context.Background()

	gen := StateGenerator{Journal: jrn, Blobs: blobs}
	if err := gen.Generate(ctx, version, firecrackerDir); err != nil {
		t.Fatalf("generate state: %v", err)
	}

	// Verify the generated files exist with correct content.
	expectedFiles := map[string]string{
		"config.yaml":        "key: value\n",
		"data/notes.txt":     "hello world\n",
		"data/settings.json": `{"enabled":true}`,
	}
	for relPath, expectedContent := range expectedFiles {
		full := filepath.Join(firecrackerDir, relPath)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("read generated %q: %v", relPath, err)
			continue
		}
		if string(data) != expectedContent {
			t.Errorf("content mismatch for %q: got %q, want %q", relPath, string(data), expectedContent)
		}
	}

	// Verify the folder exists.
	folderInfo, err := os.Stat(filepath.Join(firecrackerDir, "data"))
	if err != nil {
		t.Fatalf("stat generated folder: %v", err)
	}
	if !folderInfo.IsDir() {
		t.Error("generated data/ is not a directory")
	}

	// Now extract observations from the generated directory.
	obs, err := (FirecrackerStateExtractor{PersistentDir: firecrackerDir}).Extract(ctx, ExtractRequest{
		Name:    "firecracker-state",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract from generated state: %v", err)
	}

	// Verify the observation set has file_manifest and blob_set observations.
	hasFileManifest := false
	hasBlobSet := false
	for _, o := range obs.Observations {
		switch o.Kind {
		case ObservationFileManifest:
			hasFileManifest = true
		case ObservationBlobSet:
			hasBlobSet = true
		}
	}
	if !hasFileManifest {
		t.Error("observation set missing file_manifest")
	}
	if !hasBlobSet {
		t.Error("observation set missing blob_set")
	}

	t.Logf("round-trip proven: generated %d files, extracted %d observations",
		len(expectedFiles), len(obs.Observations))
}

// TestCrossSubstrateEquivalenceRealGenerator proves SIAC gate 4 for real: the
// same ComputerVersion is generated into two different substrate directories
// (simulating two different Firecracker VMs on different hosts), observations
// are extracted from both, and the equivalence checker passes.
//
// This is a TRUE cross-substrate proof because:
//  1. The generator produces state from the abstract ComputerVersion (not from
//     copying files between directories).
//  2. The two substrate directories are independent (different temp dirs).
//  3. The extractor reads back from each independently.
//  4. Equivalence is verified by comparing the extracted observations, not by
//     assuming the directories are identical.
func TestCrossSubstrateEquivalenceRealGenerator(t *testing.T) {
	jrn, blobs, version := buildGeneratorFixture(t)
	ctx := context.Background()

	// Generate state into two independent substrate directories.
	substrateADir := t.TempDir()
	substrateBDir := t.TempDir()

	gen := StateGenerator{Journal: jrn, Blobs: blobs}
	if err := gen.Generate(ctx, version, substrateADir); err != nil {
		t.Fatalf("generate to substrate A: %v", err)
	}
	if err := gen.Generate(ctx, version, substrateBDir); err != nil {
		t.Fatalf("generate to substrate B: %v", err)
	}

	// Extract observations from both substrates.
	obsA, err := (FirecrackerStateExtractor{PersistentDir: substrateADir}).Extract(ctx, ExtractRequest{
		Name:    "substrate-a",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract from substrate A: %v", err)
	}
	obsB, err := (FirecrackerStateExtractor{PersistentDir: substrateBDir}).Extract(ctx, ExtractRequest{
		Name:    "substrate-b",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract from substrate B: %v", err)
	}

	// Materialize both under substrate-specific capability manifests.
	// Both use the FirecrackerStateExtractor, but with different substrate
	// identities to prove the equivalence is substrate-independent.
	realizationA, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: obsA,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", "firecracker/host-a/persistent-dir"))
	if err != nil {
		t.Fatalf("materialize substrate A: %v", err)
	}
	realizationB, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: obsB,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", "firecracker/host-b/persistent-dir"))
	if err != nil {
		t.Fatalf("materialize substrate B: %v", err)
	}

	// Verify non-identical substrate identities.
	if realizationA.Capabilities.Substrate == realizationB.Capabilities.Substrate {
		t.Fatal("substrates must be non-identical for cross-substrate proof")
	}

	// Run the equivalence checker.
	result := EquivalenceChecker{}.CheckRealizations(realizationA, realizationB)
	if result.Status != EquivalenceEquivalent {
		t.Errorf("expected equivalence, got %s", result.Status)
		for _, d := range result.Differences {
			t.Errorf("  diff: kind=%s key=%s left=%s right=%s reason=%s",
				d.Kind, d.Key, d.Left, d.Right, d.Reason)
		}
		for _, u := range result.Unsupported {
			t.Errorf("  unsupported: kind=%s reason=%s", u.Kind, u.Reason)
		}
	}
	if !result.Equivalent() {
		t.Fatal("equivalence check failed: result is not equivalent")
	}

	// Build a BaseSubstrateEquivalenceContract from the proof.
	evidence := BaseSubstrateEquivalenceEvidence{
		ClaimScope:                  BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:       "substrate-a-realization",
		ProjectionRealizationRef:    "substrate-b-realization",
		CurrentObservationRef:       "substrate-a-observations",
		ProjectionObservationRef:    "substrate-b-observations",
		EquivalenceEvidenceRef:      "cross-substrate-generator-equivalence-test",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	contract, err := BuildBaseSubstrateEquivalenceContract(realizationA, realizationB, evidence)
	if err != nil {
		t.Fatalf("build substrate equivalence contract: %v", err)
	}
	if contract.EquivalenceStatus != EquivalenceEquivalent {
		t.Errorf("contract equivalence status: expected %s, got %s", EquivalenceEquivalent, contract.EquivalenceStatus)
	}

	t.Logf("cross-substrate equivalence proven via generator: %s (%s) == %s (%s) for %s@%s",
		contract.CurrentMaterializer, contract.CurrentSubstrate,
		contract.ProjectionMaterializer, contract.ProjectionSubstrate,
		version.CodeRef, version.ArtifactProgramRef)
}

// TestCrossSubstrateFailureRealGenerator proves SIAC gate 5: a seeded mismatch
// in one substrate causes the equivalence checker to fail, proving the
// verifier is not ceremonial.
func TestCrossSubstrateFailureRealGenerator(t *testing.T) {
	jrn, blobs, version := buildGeneratorFixture(t)
	ctx := context.Background()

	// Generate state into substrate A (correct).
	substrateADir := t.TempDir()
	gen := StateGenerator{Journal: jrn, Blobs: blobs}
	if err := gen.Generate(ctx, version, substrateADir); err != nil {
		t.Fatalf("generate to substrate A: %v", err)
	}

	// Generate state into substrate B, then corrupt one file.
	substrateBDir := t.TempDir()
	if err := gen.Generate(ctx, version, substrateBDir); err != nil {
		t.Fatalf("generate to substrate B: %v", err)
	}
	corruptPath := filepath.Join(substrateBDir, "data", "notes.txt")
	if err := os.WriteFile(corruptPath, []byte("hello CORRUPTED world\n"), 0o644); err != nil {
		t.Fatalf("corrupt notes.txt: %v", err)
	}

	// Extract from both.
	obsA, err := (FirecrackerStateExtractor{PersistentDir: substrateADir}).Extract(ctx, ExtractRequest{
		Name:    "substrate-a",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract from substrate A: %v", err)
	}
	obsB, err := (FirecrackerStateExtractor{PersistentDir: substrateBDir}).Extract(ctx, ExtractRequest{
		Name:    "substrate-b",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract from substrate B: %v", err)
	}

	realizationA, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: obsA,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", "firecracker/host-a/persistent-dir"))
	if err != nil {
		t.Fatalf("materialize substrate A: %v", err)
	}
	realizationB, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: obsB,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", "firecracker/host-b/persistent-dir"))
	if err != nil {
		t.Fatalf("materialize substrate B: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(realizationA, realizationB)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected not_equivalent, got %s", result.Status)
	}
	if len(result.Differences) == 0 {
		t.Fatal("expected at least one difference, got none")
	}

	// Verify the difference names the corrupted file.
	foundMismatch := false
	for _, d := range result.Differences {
		t.Logf("  diff: kind=%s key=%s reason=%s", d.Kind, d.Key, d.Reason)
		if d.Key == "data/notes.txt" || d.Key == "sha256:"+sha256Hex([]byte("hello world\n")) || d.Key == "sha256:"+sha256Hex([]byte("hello CORRUPTED world\n")) {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Errorf("expected a difference naming data/notes.txt or its blob ref, got: %+v", result.Differences)
	}

	// Verify the contract builder rejects the mismatched evidence.
	evidence := BaseSubstrateEquivalenceEvidence{
		ClaimScope:                  BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:       "substrate-a-realization",
		ProjectionRealizationRef:    "substrate-b-realization",
		CurrentObservationRef:       "substrate-a-observations",
		ProjectionObservationRef:    "substrate-b-observations",
		EquivalenceEvidenceRef:      "cross-substrate-generator-failure-test",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	if _, err := BuildBaseSubstrateEquivalenceContract(realizationA, realizationB, evidence); err == nil {
		t.Fatal("expected contract builder to reject mismatched realizations")
	}

	t.Logf("failure proof confirmed: equivalence checker detected seeded mismatch with %d differences", len(result.Differences))
}

// TestTreeToFSDirect tests the TreeToFS function directly with a hand-built
// tree and blob store, without going through the journal.
func TestTreeToFSDirect(t *testing.T) {
	blobDir := t.TempDir()
	blobs, err := blob.NewStore(blobDir)
	if err != nil {
		t.Fatalf("create blob store: %v", err)
	}

	fileContent := []byte("test file content\n")
	ref, err := blobs.Put(fileContent)
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}

	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_root")] = model.Item{
		ItemID:         "base_item_root",
		OwnerID:        "owner",
		Name:           "root.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_1",
	}
	tree.Versions[model.ItemID("base_item_root")] = model.Version{
		VersionID:   "base_ver_1",
		ItemID:      "base_item_root",
		BlobRef:     ref,
		ContentHash: sha256Hex(fileContent),
	}

	targetDir := t.TempDir()
	ctx := context.Background()
	if err := TreeToFS(ctx, tree, blobs, targetDir); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetDir, "root.txt"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(data) != string(fileContent) {
		t.Errorf("content mismatch: got %q, want %q", string(data), string(fileContent))
	}
}

// TestTreeToFSSkipsTombstones verifies that deleted items are not written.
func TestTreeToFSSkipsTombstones(t *testing.T) {
	blobDir := t.TempDir()
	blobs, err := blob.NewStore(blobDir)
	if err != nil {
		t.Fatalf("create blob store: %v", err)
	}

	content := []byte("alive\n")
	ref, err := blobs.Put(content)
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}

	deletedAt := generatorFixedTime
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_alive")] = model.Item{
		ItemID:         "base_item_alive",
		OwnerID:        "owner",
		Name:           "alive.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_alive",
	}
	tree.Versions[model.ItemID("base_item_alive")] = model.Version{
		VersionID:   "base_ver_alive",
		ItemID:      "base_item_alive",
		BlobRef:     ref,
		ContentHash: sha256Hex(content),
	}
	tree.Items[model.ItemID("base_item_dead")] = model.Item{
		ItemID:         "base_item_dead",
		OwnerID:        "owner",
		Name:           "dead.txt",
		Kind:           model.KindFile,
		CurrentVersion: "",
		DeletedAt:      &deletedAt,
	}

	targetDir := t.TempDir()
	ctx := context.Background()
	if err := TreeToFS(ctx, tree, blobs, targetDir); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}

	// alive.txt should exist.
	if _, err := os.Stat(filepath.Join(targetDir, "alive.txt")); err != nil {
		t.Errorf("alive.txt should exist: %v", err)
	}
	// dead.txt should NOT exist.
	if _, err := os.Stat(filepath.Join(targetDir, "dead.txt")); !os.IsNotExist(err) {
		t.Errorf("dead.txt should not exist, got err=%v", err)
	}
}
