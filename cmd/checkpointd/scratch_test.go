package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Failure modes, written before the code
// (docs/problems/checkpointd-scratch-leaks-superseded-generations-2026-10-15.md):
// S1 the current generation's directory is deleted (breaks a retry's resume);
// S2 another computer's directory is deleted, including one whose id extends
// this one's; S3 anything not shaped <computer>-<int> is deleted; S4 a cleanup
// error fails the job; S5 a newer generation's directory is deleted.
func TestPruneSupersededScratch(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"computer-aa-3", "computer-aa-4", "computer-aa-5", "computer-aa-6",
		"computer-aab-2",    // S2: an id that extends computer-aa
		"computer-bb-1",     // S2: another computer
		"computer-aa-notes", // S3: not a generation
		"computer-aa-",      // S3
		"computer-aa-4x",    // S3
		"computer-aa-x-2",   // S3
	} {
		if err := os.MkdirAll(filepath.Join(root, name, "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "computer-aa-1"), []byte("file, not dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	removed := pruneSupersededScratch(root, "computer-aa", 5)

	sort.Strings(removed)
	if got, want := removed, []string{"computer-aa-3", "computer-aa-4"}; !equalStrings(got, want) {
		t.Fatalf("removed = %v, want %v", got, want)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	sort.Strings(left)
	want := []string{
		"computer-aa-", "computer-aa-1", "computer-aa-4x", "computer-aa-5", "computer-aa-6",
		"computer-aa-notes", "computer-aa-x-2", "computer-aab-2", "computer-bb-1",
	}
	if !equalStrings(left, want) {
		t.Fatalf("left = %v, want %v", left, want)
	}

	// S4: a missing root is not an error and removes nothing.
	if got := pruneSupersededScratch(filepath.Join(root, "absent"), "computer-aa", 9); len(got) != 0 {
		t.Fatalf("missing root removed %v", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
