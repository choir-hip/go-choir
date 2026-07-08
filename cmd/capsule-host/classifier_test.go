//go:build linux

package main

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
)

func TestClassifierBasicClassification(t *testing.T) {
	c := transaction.NewClassifier()

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/refs/heads/main", Kind: capsule.ChangeAdded},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified},
		{Path: "/var/lib/blob/abc123", Kind: capsule.ChangeAdded},
		{Path: "/var/lib/artifact/build.tar.gz", Kind: capsule.ChangeAdded},
		{Path: "/etc/choir/route/default.json", Kind: capsule.ChangeModified},
		{Path: "/boot/vmlinuz-6.13", Kind: capsule.ChangeModified},
	}

	result := c.Classify(changes)

	if len(result.Groups[transaction.LedgerDolt]) != 1 {
		t.Errorf("expected 1 Dolt change, got %d", len(result.Groups[transaction.LedgerDolt]))
	}
	if len(result.Groups[transaction.LedgerSource]) != 1 {
		t.Errorf("expected 1 Source change, got %d", len(result.Groups[transaction.LedgerSource]))
	}
	if len(result.Groups[transaction.LedgerBlob]) != 1 {
		t.Errorf("expected 1 Blob change, got %d", len(result.Groups[transaction.LedgerBlob]))
	}
	if len(result.Groups[transaction.LedgerArtifact]) != 1 {
		t.Errorf("expected 1 Artifact change, got %d", len(result.Groups[transaction.LedgerArtifact]))
	}
	if len(result.Groups[transaction.LedgerRoute]) != 1 {
		t.Errorf("expected 1 Route change, got %d", len(result.Groups[transaction.LedgerRoute]))
	}
	if len(result.Groups[transaction.LedgerVM]) != 1 {
		t.Errorf("expected 1 VM change, got %d", len(result.Groups[transaction.LedgerVM]))
	}
	if len(result.Unknown) != 0 {
		t.Errorf("expected 0 unknown changes, got %d", len(result.Unknown))
	}
}

func TestClassifierUnknownPaths(t *testing.T) {
	c := transaction.NewClassifier()

	changes := []capsule.FileChange{
		{Path: "/opt/random/file.txt", Kind: capsule.ChangeAdded},
		{Path: "/usr/local/bin/custom", Kind: capsule.ChangeAdded},
	}

	result := c.Classify(changes)

	if len(result.Unknown) != 2 {
		t.Errorf("expected 2 unknown changes, got %d", len(result.Unknown))
	}
	if !result.HasUnknown() {
		t.Error("expected HasUnknown to be true")
	}
}

func TestClassifierIgnorePaths(t *testing.T) {
	c := transaction.NewClassifier()

	changes := []capsule.FileChange{
		{Path: "/tmp/session123/cache.txt", Kind: capsule.ChangeAdded},
		{Path: "/run/capsule.pid", Kind: capsule.ChangeAdded},
		{Path: "/var/log/capsule.log", Kind: capsule.ChangeAdded},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified},
	}

	result := c.Classify(changes)

	if len(result.Ignored) != 3 {
		t.Errorf("expected 3 ignored changes, got %d", len(result.Ignored))
	}
	if len(result.Groups[transaction.LedgerSource]) != 1 {
		t.Errorf("expected 1 Source change, got %d", len(result.Groups[transaction.LedgerSource]))
	}
}

func TestClassifierDigestDeterministic(t *testing.T) {
	c := transaction.NewClassifier()

	changes := []capsule.FileChange{
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified},
		{Path: "/var/lib/dolt/refs/heads/main", Kind: capsule.ChangeAdded},
	}

	result1 := c.Classify(changes)
	result2 := c.Classify(changes)

	if result1.Digest != result2.Digest {
		t.Error("digest should be deterministic for same input")
	}
}

func TestClassifierRulesDigest(t *testing.T) {
	c1 := transaction.NewClassifier()
	c2 := transaction.NewClassifier()

	if c1.RulesDigest() != c2.RulesDigest() {
		t.Error("same classifier config should produce same rules digest")
	}
}

func TestTransactionBuilderReject(t *testing.T) {
	c := transaction.NewClassifier()
	builder := transaction.NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/opt/unknown/path", Kind: capsule.ChangeAdded},
	}

	record, err := builder.BuildTransactionFromDiff("capsule-001", changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !record.Rejected {
		t.Error("transaction should be rejected for unknown paths")
	}
	if record.RejectReason == "" {
		t.Error("reject reason should not be empty")
	}
}

func TestTransactionBuilderAccept(t *testing.T) {
	c := transaction.NewClassifier()
	builder := transaction.NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified},
		{Path: "/var/lib/dolt/refs/heads/main", Kind: capsule.ChangeAdded},
	}

	record, err := builder.BuildTransactionFromDiff("capsule-001", changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.Rejected {
		t.Error("transaction should not be rejected for known paths")
	}
	if len(record.Groups["Source"]) != 1 {
		t.Errorf("expected 1 Source change, got %d", len(record.Groups["Source"]))
	}
	if len(record.Groups["Dolt"]) != 1 {
		t.Errorf("expected 1 Dolt change, got %d", len(record.Groups["Dolt"]))
	}
}
