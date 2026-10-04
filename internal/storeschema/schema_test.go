package storeschema

import (
	"os"
	"path/filepath"
	"testing"
)

// The receipt is the S2-d contract between the running store and the
// updater's pre-mutation gate: Open must write it inside the workspace and
// readers must decode exactly what was written — a skewed path or format
// silently disables the refusal.
func TestWriteThenReadReceipt(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}
	receipt, found, err := Read(filepath.Join(dir, File))
	if err != nil || !found {
		t.Fatalf("Read found=%v err=%v", found, err)
	}
	if receipt.Schema != Name || receipt.Version != Version {
		t.Fatalf("receipt = %+v, want schema=%s version=%d", receipt, Name, Version)
	}
}

func TestReadAbsentReceiptReportsNotFound(t *testing.T) {
	_, found, err := Read(filepath.Join(t.TempDir(), File))
	if err != nil || found {
		t.Fatalf("absent receipt found=%v err=%v, want found=false", found, err)
	}
}

func TestReadRejectsCorruptReceipt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(filepath.Join(dir, File)); err == nil {
		t.Fatal("corrupt receipt decoded without error")
	}
}
