package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestLocalTreeBuilderScan(t *testing.T) {
	root := t.TempDir()
	// Create a small tree:
	//   root/
	//     hello.txt
	//     sub/
	//       notes.md
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "notes.md"), []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := NewLocalTreeBuilder(root, "device-test")
	tree, err := b.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Expect 3 items: hello.txt, sub, sub/notes.md
	if len(tree.Items) != 3 {
		t.Fatalf("Scan: got %d items, want 3", len(tree.Items))
	}

	// Find hello.txt by name.
	var helloID model.ItemID
	var foundHello bool
	for id, it := range tree.Items {
		if it.Name == "hello.txt" {
			helloID = id
			foundHello = true
			if it.Kind != model.KindFile {
				t.Errorf("hello.txt kind: got %q, want file", it.Kind)
			}
		}
	}
	if !foundHello {
		t.Fatal("hello.txt not found in scan")
	}

	// hello.txt must have a version with a blob ref.
	ver, ok := tree.Versions[helloID]
	if !ok {
		t.Fatal("hello.txt has no version")
	}
	if !ver.BlobRef.Valid() || ver.BlobRef == "" {
		t.Errorf("hello.txt blob ref: got %q", ver.BlobRef)
	}
	if ver.ContentHash == "" {
		t.Error("hello.txt content hash empty")
	}

	// The folder "sub" must have no blob ref.
	for _, it := range tree.Items {
		if it.Name == "sub" {
			v := tree.Versions[it.ItemID]
			if v.BlobRef != "" {
				t.Errorf("folder sub blob ref: got %q, want empty", v.BlobRef)
			}
		}
	}

	// RelPathFromID should reconstruct "hello.txt".
	rel := RelPathFromID(tree, helloID)
	if rel != "hello.txt" {
		t.Errorf("RelPathFromID hello.txt: got %q", rel)
	}

	// RelPathFromID for notes.md should be "sub/notes.md".
	for _, it := range tree.Items {
		if it.Name == "notes.md" {
			rel := RelPathFromID(tree, it.ItemID)
			if rel != "sub/notes.md" {
				t.Errorf("RelPathFromID notes.md: got %q, want sub/notes.md", rel)
			}
		}
	}
}

func TestLocalTreeBuilderSkipsHidden(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("h"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := NewLocalTreeBuilder(root, "dev")
	tree, err := b.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tree.Items) != 1 {
		t.Fatalf("Scan: got %d items, want 1 (visible.txt only)", len(tree.Items))
	}
}

func TestLocalTreeBuilderDeterministicIDs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewLocalTreeBuilder(root, "dev")
	t1, _ := b.Scan()
	t2, _ := b.Scan()
	if len(t1.Items) != len(t2.Items) {
		t.Fatalf("rescan item count differs: %d vs %d", len(t1.Items), len(t2.Items))
	}
	for id := range t1.Items {
		if _, ok := t2.Items[id]; !ok {
			t.Errorf("item %s not stable across rescans", id)
		}
	}
}

func TestLocalItemIDFormat(t *testing.T) {
	id := localItemID("foo/bar.txt")
	if !id.Valid() {
		t.Errorf("localItemID produced invalid id: %q", id)
	}
	// Different paths -> different IDs.
	id2 := localItemID("foo/baz.txt")
	if id == id2 {
		t.Error("different paths produced same ItemID")
	}
}
