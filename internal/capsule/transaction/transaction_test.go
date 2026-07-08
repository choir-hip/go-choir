package transaction

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestClassifierBasic(t *testing.T) {
	c := NewClassifier()
	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified, Mode: 0o644},
		{Path: "/tmp/cache.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/unknown/path.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	result := c.Classify(changes)

	if len(result.Groups[LedgerDolt]) != 1 {
		t.Errorf("Dolt group: expected 1, got %d", len(result.Groups[LedgerDolt]))
	}
	if len(result.Groups[LedgerSource]) != 1 {
		t.Errorf("Source group: expected 1, got %d", len(result.Groups[LedgerSource]))
	}
	if len(result.Ignored) != 1 {
		t.Errorf("Ignored: expected 1, got %d", len(result.Ignored))
	}
	if len(result.Unknown) != 1 {
		t.Errorf("Unknown: expected 1, got %d", len(result.Unknown))
	}
	if !result.HasUnknown() {
		t.Error("HasUnknown should be true")
	}
	if result.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestClassifierRejectsUnknown(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/unknown/path.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	record, err := builder.BuildTransactionFromDiff("capsule-1", changes)
	if err != nil {
		t.Fatalf("build transaction: %v", err)
	}
	if !record.Rejected {
		t.Error("record should be rejected due to unknown paths")
	}
	if record.RejectReason == "" {
		t.Error("reject reason should not be empty")
	}
}

func TestTransactionBuilderAcceptsKnownPaths(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified, Mode: 0o644},
		{Path: "/var/lib/blob/abc123", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	record, err := builder.BuildTransactionFromDiff("capsule-1", changes)
	if err != nil {
		t.Fatalf("build transaction: %v", err)
	}
	if record.Rejected {
		t.Errorf("record should not be rejected, but: %s", record.RejectReason)
	}
	if record.CapsuleID != "capsule-1" {
		t.Errorf("capsule ID: got %q, want %q", record.CapsuleID, "capsule-1")
	}
	if record.ClassifierV != "v1" {
		t.Errorf("classifier version: got %q, want %q", record.ClassifierV, "v1")
	}
	if len(record.Groups["Dolt"]) != 1 {
		t.Errorf("Dolt group: expected 1, got %d", len(record.Groups["Dolt"]))
	}
	if len(record.Groups["Source"]) != 1 {
		t.Errorf("Source group: expected 1, got %d", len(record.Groups["Source"]))
	}
	if len(record.Groups["Blob"]) != 1 {
		t.Errorf("Blob group: expected 1, got %d", len(record.Groups["Blob"]))
	}
}

func TestTapeAppendAndVerify(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	// Append 3 valid transactions.
	for i := 0; i < 3; i++ {
		changes := []capsule.FileChange{
			{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeModified, Mode: 0o644},
		}
		record, err := builder.BuildTransactionFromDiff("capsule-1", changes)
		if err != nil {
			t.Fatalf("build transaction %d: %v", i, err)
		}
		hash, err := tape.Append(record)
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if hash == "" {
			t.Errorf("append %d: empty hash", i)
		}
	}

	if tape.Len() != 3 {
		t.Errorf("tape length: expected 3, got %d", tape.Len())
	}

	// Verify the chain.
	if err := tape.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Head should be the last entry's hash.
	entries := tape.Entries()
	if tape.Head() != entries[2].Hash {
		t.Errorf("head: got %q, want %q", tape.Head(), entries[2].Hash)
	}
}

func TestTapeRejectsRejectedRecords(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	changes := []capsule.FileChange{
		{Path: "/unknown/path.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
	}
	record, err := builder.BuildTransactionFromDiff("capsule-1", changes)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !record.Rejected {
		t.Fatal("record should be rejected")
	}

	_, err = tape.Append(record)
	if err == nil {
		t.Fatal("expected error appending rejected record, got nil")
	}
	if tape.Len() != 0 {
		t.Errorf("tape should be empty, got %d", tape.Len())
	}
}

func TestTapeTamperDetection(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	// Append 3 transactions.
	for i := 0; i < 3; i++ {
		changes := []capsule.FileChange{
			{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeModified, Mode: 0o644},
		}
		record, _ := builder.BuildTransactionFromDiff("capsule-1", changes)
		tape.Append(record)
	}

	// Verify intact.
	if err := tape.Verify(); err != nil {
		t.Fatalf("verify before tamper: %v", err)
	}

	// Tamper: modify the second entry's record.
	entries := tape.Entries()
	entries[1].Record.CapsuleID = "tampered"

	// Write the tampered entries back.
	tape.mu.Lock()
	tape.entries = entries
	tape.mu.Unlock()

	// Verify should detect the tamper.
	err := tape.Verify()
	if err == nil {
		t.Fatal("expected tamper detection, got nil")
	}
}

func TestTapeReset(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeModified, Mode: 0o644},
	}
	record, _ := builder.BuildTransactionFromDiff("capsule-1", changes)
	tape.Append(record)

	if tape.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", tape.Len())
	}

	tape.Reset()

	if tape.Len() != 0 {
		t.Errorf("after reset: expected 0, got %d", tape.Len())
	}
	if tape.Head() != "" {
		t.Errorf("after reset: head should be empty, got %q", tape.Head())
	}
}

func TestTapeChainIntegrity(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	// Each entry must link to the previous.
	var prevHash string
	for i := 0; i < 5; i++ {
		changes := []capsule.FileChange{
			{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeModified, Mode: 0o644},
		}
		record, _ := builder.BuildTransactionFromDiff("capsule-1", changes)
		record.Timestamp = time.Now().UTC().Add(time.Duration(i) * time.Second)
		hash, err := tape.Append(record)
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}

		entries := tape.Entries()
		entry := entries[i]
		if entry.PrevHash != prevHash {
			t.Errorf("entry %d: prevHash %q != expected %q", i, entry.PrevHash, prevHash)
		}
		if entry.Hash != hash {
			t.Errorf("entry %d: hash mismatch", i)
		}
		if entry.Index != i {
			t.Errorf("entry %d: index %d != expected %d", i, entry.Index, i)
		}
		prevHash = hash
	}

	// Full verification.
	if err := tape.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestTapeMarshalUnmarshal(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)
	tape := NewTape()

	for i := 0; i < 3; i++ {
		changes := []capsule.FileChange{
			{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeModified, Mode: 0o644},
		}
		record, _ := builder.BuildTransactionFromDiff("capsule-1", changes)
		tape.Append(record)
	}

	data, err := tape.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tape2 := NewTape()
	if err := tape2.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if tape2.Len() != 3 {
		t.Errorf("unmarshaled tape length: expected 3, got %d", tape2.Len())
	}

	// The unmarshaled tape should verify.
	if err := tape2.Verify(); err != nil {
		t.Fatalf("verify unmarshaled tape: %v", err)
	}

	// Heads should match.
	if tape.Head() != tape2.Head() {
		t.Errorf("head mismatch: %q != %q", tape.Head(), tape2.Head())
	}
}
