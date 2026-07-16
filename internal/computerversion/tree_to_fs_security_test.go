package computerversion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

// --- helpers ---------------------------------------------------------------

// newBlobStore creates a fresh blob store in a temporary directory.
func newBlobStore(t *testing.T) *blob.Store {
	t.Helper()
	blobs, err := blob.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create blob store: %v", err)
	}
	return blobs
}

// putBlob stores content in blobs and returns its ref.
func putBlob(t *testing.T, blobs *blob.Store, content []byte) model.BlobRef {
	t.Helper()
	ref, err := blobs.Put(content)
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	return ref
}

// singleFileTree builds a tree containing one live root file named name whose
// version references ref. Used by path-traversal and version-consistency tests.
func singleFileTree(name string, ref model.BlobRef) basetree.Tree {
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_file")] = model.Item{
		ItemID:         "base_item_file",
		OwnerID:        "owner",
		Name:           name,
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_1",
	}
	tree.Versions[model.ItemID("base_item_file")] = model.Version{
		VersionID:   "base_ver_1",
		ItemID:      "base_item_file",
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("x\n")),
	}
	return tree
}

// freshTarget returns a path that does not yet exist under a temp dir.
func freshTarget(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "target")
}

// expectTreeToFSError calls TreeToFS and fails the test if it does not error.
func expectTreeToFSError(t *testing.T, tree basetree.Tree, blobs *blob.Store, target string, wantSubstr string) {
	t.Helper()
	err := TreeToFS(context.Background(), tree, blobs, target)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantSubstr)
	}
	if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error containing %q, got %q", wantSubstr, err.Error())
	}
}

// testRejectsName is the shared body for the path-traversal name tests.
func testRejectsName(t *testing.T, name, wantSubstr string) {
	t.Helper()
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := singleFileTree(name, ref)
	expectTreeToFSError(t, tree, blobs, freshTarget(t), wantSubstr)
}

// --- path traversal tests --------------------------------------------------

func TestTreeToFSPathTraversalDotDot(t *testing.T) {
	testRejectsName(t, "..", "reserved")
}

func TestTreeToFSPathTraversalDotDotSlash(t *testing.T) {
	testRejectsName(t, "../escape.txt", "path separator")
}

func TestTreeToFSPathTraversalAbsolute(t *testing.T) {
	testRejectsName(t, "/etc/passwd", "path separator")
}

func TestTreeToFSPathTraversalWithSeparator(t *testing.T) {
	testRejectsName(t, "a/b", "path separator")
}

func TestTreeToFSPathTraversalEmpty(t *testing.T) {
	testRejectsName(t, "", "empty name")
}

func TestTreeToFSPathTraversalDot(t *testing.T) {
	testRejectsName(t, ".", "reserved")
}

func TestTreeToFSPathTraversalBackslash(t *testing.T) {
	testRejectsName(t, `..\escape.txt`, "path separator")
}

// --- symlink tests ---------------------------------------------------------

func TestTreeToFSSymlinkDirAttack(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := singleFileTree("root.txt", ref)

	// Make targetDir itself a symlink pointing outside the temp tree.
	target := filepath.Join(t.TempDir(), "linktarget")
	if err := os.Symlink(t.TempDir(), target); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	expectTreeToFSError(t, tree, blobs, target, "is a symlink")
}

func TestTreeToFSSymlinkFileAttack(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := singleFileTree("root.txt", ref)

	// Pre-populate targetDir with a symlink file. The non-empty / symlink
	// defenses ensure TreeToFS refuses to write through or over it.
	target := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(target, "evil")); err != nil {
		t.Fatalf("create symlink file: %v", err)
	}
	expectTreeToFSError(t, tree, blobs, target, "")
}

// --- empty dir enforcement -------------------------------------------------

func TestTreeToFSNonEmptyTargetRejected(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := singleFileTree("root.txt", ref)

	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "preexisting.txt"), []byte("nope\n"), 0o600); err != nil {
		t.Fatalf("seed target dir: %v", err)
	}
	expectTreeToFSError(t, tree, blobs, target, "not empty")
}

// --- edge cases ------------------------------------------------------------

func TestTreeToFSEmptyTree(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	target := freshTarget(t)
	if err := TreeToFS(context.Background(), tree, blobs, target); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("target should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("target should be a directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("target should be empty, got %d entries", len(entries))
	}
}

func TestTreeToFSOnlyFolders(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_folder_a")] = model.Item{
		ItemID:         "base_item_folder_a",
		OwnerID:        "owner",
		Name:           "alpha",
		Kind:           model.KindFolder,
		CurrentVersion: "base_ver_a",
	}
	tree.Versions[model.ItemID("base_item_folder_a")] = model.Version{
		VersionID: "base_ver_a",
		ItemID:    "base_item_folder_a",
	}
	tree.Items[model.ItemID("base_item_folder_b")] = model.Item{
		ItemID:         "base_item_folder_b",
		OwnerID:        "owner",
		ParentItemID:   "base_item_folder_a",
		Name:           "beta",
		Kind:           model.KindFolder,
		CurrentVersion: "base_ver_b",
	}
	tree.Versions[model.ItemID("base_item_folder_b")] = model.Version{
		VersionID: "base_ver_b",
		ItemID:    "base_item_folder_b",
	}
	target := freshTarget(t)
	if err := TreeToFS(context.Background(), tree, blobs, target); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "alpha")); err != nil {
		t.Errorf("alpha should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "alpha", "beta")); err != nil {
		t.Errorf("alpha/beta should exist: %v", err)
	}
}

func TestTreeToFSDeepNesting(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	const depth = 25
	var parentID model.ItemID
	for i := 0; i < depth; i++ {
		id := model.ItemID("base_item_folder_" + runeName(i))
		verID := model.VersionID("base_ver_" + runeName(i))
		tree.Items[id] = model.Item{
			ItemID:         id,
			OwnerID:        "owner",
			ParentItemID:   parentID,
			Name:           "level" + runeName(i),
			Kind:           model.KindFolder,
			CurrentVersion: verID,
		}
		tree.Versions[id] = model.Version{
			VersionID: verID,
			ItemID:    id,
		}
		parentID = id
	}
	target := freshTarget(t)
	if err := TreeToFS(context.Background(), tree, blobs, target); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}
	// Walk the chain and confirm every level exists.
	cur := target
	for i := 0; i < depth; i++ {
		cur = filepath.Join(cur, "level"+runeName(i))
		if info, err := os.Stat(cur); err != nil || !info.IsDir() {
			t.Fatalf("expected dir at %q: %v", cur, err)
		}
	}
}

func TestTreeToFSMissingParent(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_child")] = model.Item{
		ItemID:         "base_item_child",
		OwnerID:        "owner",
		ParentItemID:   "base_item_ghost",
		Name:           "child.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_child",
	}
	tree.Versions[model.ItemID("base_item_child")] = model.Version{
		VersionID:   "base_ver_child",
		ItemID:      "base_item_child",
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("x\n")),
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "missing parent")
}

func TestTreeToFSCircularParent(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_a")] = model.Item{
		ItemID:         "base_item_a",
		OwnerID:        "owner",
		ParentItemID:   "base_item_b",
		Name:           "a",
		Kind:           model.KindFolder,
		CurrentVersion: "base_ver_a",
	}
	tree.Items[model.ItemID("base_item_b")] = model.Item{
		ItemID:         "base_item_b",
		OwnerID:        "owner",
		ParentItemID:   "base_item_a",
		Name:           "b",
		Kind:           model.KindFolder,
		CurrentVersion: "base_ver_b",
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "cannot resolve path")
}

func TestTreeToFSLiveChildUnderDeletedParent(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	deletedAt := generatorFixedTime
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_parent")] = model.Item{
		ItemID:         "base_item_parent",
		OwnerID:        "owner",
		Name:           "parent",
		Kind:           model.KindFolder,
		CurrentVersion: "",
		DeletedAt:      &deletedAt,
	}
	tree.Items[model.ItemID("base_item_child")] = model.Item{
		ItemID:         "base_item_child",
		OwnerID:        "owner",
		ParentItemID:   "base_item_parent",
		Name:           "child.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_child",
	}
	tree.Versions[model.ItemID("base_item_child")] = model.Version{
		VersionID:   "base_ver_child",
		ItemID:      "base_item_child",
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("x\n")),
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "deleted parent")
}

func TestTreeToFSLiveChildUnderFileParent(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("parent\n"))
	childRef := putBlob(t, blobs, []byte("child\n"))
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_parent")] = model.Item{
		ItemID:         "base_item_parent",
		OwnerID:        "owner",
		Name:           "parent.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_parent",
	}
	tree.Versions[model.ItemID("base_item_parent")] = model.Version{
		VersionID:   "base_ver_parent",
		ItemID:      "base_item_parent",
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("parent\n")),
	}
	tree.Items[model.ItemID("base_item_child")] = model.Item{
		ItemID:         "base_item_child",
		OwnerID:        "owner",
		ParentItemID:   "base_item_parent",
		Name:           "child.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_child",
	}
	tree.Versions[model.ItemID("base_item_child")] = model.Version{
		VersionID:   "base_ver_child",
		ItemID:      "base_item_child",
		BlobRef:     childRef,
		ContentHash: sha256Hex([]byte("child\n")),
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "non-folder parent")
}

func TestTreeToFSDuplicatePaths(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := basetree.NewTree()
	for _, id := range []string{"base_item_dup_a", "base_item_dup_b"} {
		tree.Items[model.ItemID(id)] = model.Item{
			ItemID:         model.ItemID(id),
			OwnerID:        "owner",
			Name:           "same.txt",
			Kind:           model.KindFile,
			CurrentVersion: "base_ver_dup",
		}
		tree.Versions[model.ItemID(id)] = model.Version{
			VersionID:   "base_ver_dup",
			ItemID:      model.ItemID(id),
			BlobRef:     ref,
			ContentHash: sha256Hex([]byte("x\n")),
		}
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "duplicate path")
}

func TestTreeToFSUnknownItemKind(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_weird")] = model.Item{
		ItemID:         "base_item_weird",
		OwnerID:        "owner",
		Name:           "weird",
		Kind:           model.ItemKind("unknown"),
		CurrentVersion: "base_ver_weird",
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "unsupported kind")
}

func TestTreeToFSMissingVersion(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_file")] = model.Item{
		ItemID:         "base_item_file",
		OwnerID:        "owner",
		Name:           "root.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_1",
	}
	// No version entry.
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "no version")
}

func TestTreeToFSEmptyBlobRef(t *testing.T) {
	blobs := newBlobStore(t)
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_file")] = model.Item{
		ItemID:         "base_item_file",
		OwnerID:        "owner",
		Name:           "root.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_1",
	}
	tree.Versions[model.ItemID("base_item_file")] = model.Version{
		VersionID: "base_ver_1",
		ItemID:    "base_item_file",
		// BlobRef intentionally empty.
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "empty blob ref")
}

func TestTreeToFSVersionItemIDMismatch(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_file")] = model.Item{
		ItemID:         "base_item_file",
		OwnerID:        "owner",
		Name:           "root.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_1",
	}
	tree.Versions[model.ItemID("base_item_file")] = model.Version{
		VersionID:   "base_ver_1",
		ItemID:      "base_item_other", // mismatch
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("x\n")),
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "does not match item")
}

func TestTreeToFSVersionIDMismatch(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := basetree.NewTree()
	tree.Items[model.ItemID("base_item_file")] = model.Item{
		ItemID:         "base_item_file",
		OwnerID:        "owner",
		Name:           "root.txt",
		Kind:           model.KindFile,
		CurrentVersion: "base_ver_current",
	}
	tree.Versions[model.ItemID("base_item_file")] = model.Version{
		VersionID:   "base_ver_different", // mismatch with CurrentVersion
		ItemID:      "base_item_file",
		BlobRef:     ref,
		ContentHash: sha256Hex([]byte("x\n")),
	}
	expectTreeToFSError(t, tree, blobs, freshTarget(t), "does not match current version")
}

// --- context cancellation --------------------------------------------------

func TestTreeToFSContextCancelled(t *testing.T) {
	blobs := newBlobStore(t)
	ref := putBlob(t, blobs, []byte("x\n"))
	tree := singleFileTree("root.txt", ref)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := TreeToFS(ctx, tree, blobs, freshTarget(t))
	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// --- StateGenerator tests --------------------------------------------------

func TestStateGeneratorNilJournal(t *testing.T) {
	_, blobs, program, version := buildGeneratorFixture(t)
	gen := StateGenerator{Journal: nil, Blobs: blobs, ArtifactProgram: program}
	err := gen.Generate(context.Background(), version, freshTarget(t))
	if err == nil {
		t.Fatal("expected error for nil journal, got nil")
	}
	if !strings.Contains(err.Error(), "nil journal") {
		t.Errorf("expected 'nil journal' error, got %q", err.Error())
	}
}

func TestStateGeneratorNilBlobs(t *testing.T) {
	jrn, _, program, version := buildGeneratorFixture(t)
	gen := StateGenerator{Journal: jrn, Blobs: nil, ArtifactProgram: program}
	err := gen.Generate(context.Background(), version, freshTarget(t))
	if err == nil {
		t.Fatal("expected error for nil blob store, got nil")
	}
	if !strings.Contains(err.Error(), "nil blob store") {
		t.Errorf("expected 'nil blob store' error, got %q", err.Error())
	}
}

func TestStateGeneratorInvalidVersion(t *testing.T) {
	jrn, blobs, program, _ := buildGeneratorFixture(t)
	invalid := ComputerVersion{} // empty CodeRef -> invalid
	gen := StateGenerator{Journal: jrn, Blobs: blobs, ArtifactProgram: program}
	err := gen.Generate(context.Background(), invalid, freshTarget(t))
	if err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
	if !strings.Contains(err.Error(), "invalid computer version") {
		t.Errorf("expected 'invalid computer version' error, got %q", err.Error())
	}
}

func TestGenerateFromEventsTamperedChain(t *testing.T) {
	jrn, blobs, program, version := buildGeneratorFixture(t)
	entries := jrn.Entries()
	if len(entries) == 0 {
		t.Fatal("fixture produced no entries")
	}
	// Tamper with the first entry's hash to break the chain.
	entries[0].Hash = "deadbeef" + strings.Repeat("0", 56)
	err := GenerateFromEvents(context.Background(), entries, blobs, program, version, freshTarget(t))
	if err == nil {
		t.Fatal("expected error for tampered chain, got nil")
	}
	if !strings.Contains(err.Error(), "verify entries") {
		t.Errorf("expected 'verify entries' error, got %q", err.Error())
	}
}

func TestStateGeneratorRejectsCallerTrustedArtifactProgramBinding(t *testing.T) {
	jrn, blobs, _, version := buildGeneratorFixture(t)
	wrongProgram, err := NewJournalArtifactProgram(nil, "fixture/wrong-journal", generatorFixedTime)
	if err != nil {
		t.Fatalf("wrong program: %v", err)
	}
	version.ArtifactProgramRef = wrongProgram.Ref
	gen := StateGenerator{Journal: jrn, Blobs: blobs, ArtifactProgram: wrongProgram}
	err = gen.Generate(context.Background(), version, freshTarget(t))
	if err == nil || !strings.Contains(err.Error(), "base journal hash mismatch") {
		t.Fatalf("caller-trusted journal binding error = %v", err)
	}
}

// --- atomicity tests -------------------------------------------------------

func TestTreeToFSAtomicOnSuccess(t *testing.T) {
	blobs := newBlobStore(t)
	content := []byte("atomic\n")
	ref := putBlob(t, blobs, content)
	tree := singleFileTree("root.txt", ref)
	target := freshTarget(t)

	if err := TreeToFS(context.Background(), tree, blobs, target); err != nil {
		t.Fatalf("tree-to-fs: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target should exist after success: %v", err)
	}
	// Check no temp dirs leaked. Production uses MkdirTemp with a
	// randomized "target.tmp-*" pattern, so we scan the parent for any
	// matching entries instead of a fixed "target.tmp" path.
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatalf("read parent dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(target)+".tmp-") {
			t.Errorf("leaked temp dir: %s", e.Name())
		}
	}
	data, err := os.ReadFile(filepath.Join(target, "root.txt"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestTreeToFSNoPartialOutputOnError(t *testing.T) {
	blobs := newBlobStore(t)
	// A validly-formatted blob ref that does not exist in the store.
	missingRef := model.BlobRef("sha256:" + strings.Repeat("a", 64))
	tree := singleFileTree("root.txt", missingRef)
	target := freshTarget(t)

	err := TreeToFS(context.Background(), tree, blobs, target)
	if err == nil {
		t.Fatal("expected error for missing blob, got nil")
	}
	// targetDir must not exist (no partial output).
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target should not exist after error, got err=%v", err)
	}
	// temp dir must be cleaned up. Production uses MkdirTemp with a
	// randomized "target.tmp-*" pattern, so we scan the parent for any
	// matching entries instead of a fixed "target.tmp" path.
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatalf("read parent dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(target)+".tmp-") {
			t.Errorf("leaked temp dir: %s", e.Name())
		}
	}
}

// runeName returns a stable lowercase alphabetic name for index i (a, b, ...).
func runeName(i int) string {
	const alpha = "abcdefghijklmnopqrstuvwxyz"
	if i < len(alpha) {
		return string(alpha[i])
	}
	return "n" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
