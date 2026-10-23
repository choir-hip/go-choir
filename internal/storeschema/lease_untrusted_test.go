package storeschema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stream_seq lease's untrust marker and the epoch bump
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, sections 3
// and 8). Ways it can fail, written before the code:
//  1. The epoch stays at 2, so an epoch-2 binary's rewrite of the receipt is
//     indistinguishable from the epoch this design reads (test 1, 2).
//  2. The marker write is not atomic, or leaves a temporary file in the
//     workspace, which a base would then carry.
//  3. A reader mistakes a directory, an empty file or a corrupt file for
//     "no marker": every unreadable answer must read as present (fail closed).
//  4. Clearing a marker that is not there fails, or clearing leaves the file.
//  5. The marker body is not valid JSON, so a base cannot carry it as a tar
//     entry the same way the store writes it.

func TestVersionIsEpochThreeForTheLease(t *testing.T) {
	if Version != 3 {
		t.Fatalf("storeschema.Version = %d, want 3 (the stream_seq lease)", Version)
	}
}

func TestLeaseUntrustedFileName(t *testing.T) {
	if LeaseUntrustedFile != "stream-seq-lease-untrusted.json" {
		t.Fatalf("LeaseUntrustedFile = %q", LeaseUntrustedFile)
	}
}

func TestLeaseUntrustedWriteReadClear(t *testing.T) {
	dir := t.TempDir()
	present, err := LeaseUntrusted(dir)
	if err != nil || present {
		t.Fatalf("a workspace without a marker reads present=%v err=%v", present, err)
	}
	if err := WriteLeaseUntrusted(dir, "receipt"); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	present, err = LeaseUntrusted(dir)
	if err != nil || !present {
		t.Fatalf("after a write the marker reads present=%v err=%v", present, err)
	}
	// Writing again is idempotent: every swap point may write it.
	if err := WriteLeaseUntrusted(dir, "install"); err != nil {
		t.Fatalf("rewrite marker: %v", err)
	}
	if err := ClearLeaseUntrusted(dir); err != nil {
		t.Fatalf("clear marker: %v", err)
	}
	present, err = LeaseUntrusted(dir)
	if err != nil || present {
		t.Fatalf("after a clear the marker reads present=%v err=%v", present, err)
	}
	if err := ClearLeaseUntrusted(dir); err != nil {
		t.Fatalf("clearing an absent marker must succeed: %v", err)
	}
}

func TestLeaseUntrustedWriteLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLeaseUntrusted(dir, "receipt"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != LeaseUntrustedFile {
			t.Fatalf("the marker write left %q beside the marker", entry.Name())
		}
	}
}

func TestLeaseUntrustedReasonIsReadable(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLeaseUntrusted(dir, "receipt"); err != nil {
		t.Fatal(err)
	}
	reason, present, err := LeaseUntrustedReason(dir)
	if err != nil || !present || reason != "receipt" {
		t.Fatalf("reason = %q present=%v err=%v, want receipt", reason, present, err)
	}
}

// Every unreadable marker reads as present: the answer that closes.
func TestLeaseUntrustedUnreadableMarkerReadsPresent(t *testing.T) {
	for name, build := range map[string]func(path string) error{
		"directory": func(path string) error { return os.Mkdir(path, 0o755) },
		"empty":     func(path string) error { return os.WriteFile(path, nil, 0o644) },
		"corrupt":   func(path string) error { return os.WriteFile(path, []byte("{not json"), 0o644) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := build(filepath.Join(dir, LeaseUntrustedFile)); err != nil {
				t.Fatal(err)
			}
			present, err := LeaseUntrusted(dir)
			if err != nil {
				t.Fatalf("an unreadable marker must read present, not fail: %v", err)
			}
			if !present {
				t.Fatalf("a %s marker read as absent: the lease would be trusted", name)
			}
		})
	}
}

func TestLeaseUntrustedWriteFailsWhenTheMarkerPathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, LeaseUntrustedFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteLeaseUntrusted(dir, "receipt"); err == nil {
		t.Fatal("a marker write that could not publish returned nil")
	}
}

func TestLeaseUntrustedWriteFailsInAMissingWorkspace(t *testing.T) {
	if err := WriteLeaseUntrusted(filepath.Join(t.TempDir(), "absent"), "receipt"); err == nil {
		t.Fatal("a marker write into a workspace that does not exist returned nil")
	}
}

// The publisher carries the marker as a tar entry; its bytes are the same
// JSON the store writes.
func TestLeaseUntrustedBodyIsTheFileContent(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLeaseUntrusted(dir, "base"); err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, LeaseUntrustedFile))
	if err != nil {
		t.Fatal(err)
	}
	body := LeaseUntrustedBody("base")
	if string(body) != string(onDisk) {
		t.Fatalf("LeaseUntrustedBody = %q, file = %q", body, onDisk)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("marker body is not JSON: %v", err)
	}
	if !strings.Contains(string(body), "base") {
		t.Fatalf("marker body does not name its reason: %s", body)
	}
}

// A written receipt never clears a marker: only the seeder does.
func TestWritingTheReceiptDoesNotClearTheMarker(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLeaseUntrusted(dir, "receipt"); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	present, err := LeaseUntrusted(dir)
	if err != nil || !present {
		t.Fatalf("Write cleared the marker: present=%v err=%v", present, err)
	}
}
