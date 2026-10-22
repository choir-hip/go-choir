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

// F23 (docs/design/choir-event-table-f-c-2026-10-22.md section 4.1): deploy 1
// of run telemetry in its own table bumps the epoch 1 -> 2. An older binary
// does not honour ProjectionOp.Table and its replay manifest calls
// choir_events unknown, so the change is not freely backward compatible, and
// the updater must refuse an epoch-1 release on a store that says 2.
func TestVersionCoversTheEventTable(t *testing.T) {
	if Version != 2 {
		t.Fatalf("storeschema.Version = %d, want 2 (choir_events, deploy 1)", Version)
	}
}
