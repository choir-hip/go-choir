//go:build linux

package capsule

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistRevocationReceiptIsStructuredDurableAndRequiresAbsentCapsule(t *testing.T) {
	state := t.TempDir()
	e := NewExecutor(state, t.TempDir(), filepath.Join(t.TempDir(), "missing-broker"), 1<<20)
	digest := "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e.capsules["live"] = &Capsule{ID: "live"}
	if _, err := e.PersistRevocationReceipt("run", digest, "live", "intent"); err == nil {
		t.Fatal("live capsule acknowledged absent")
	}
	delete(e.capsules, "live")
	if err := os.MkdirAll(filepath.Join(state, "live", "root"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := e.PersistRevocationReceipt("run", digest, "live", "capsule-revoke-intent:exact"); err == nil {
		t.Fatal("restart residue acknowledged absent from reconstructed in-memory map")
	}
	if err := e.CleanupOrphanedCapsule(context.Background(), "live"); err != nil {
		t.Fatal(err)
	}
	receipt, err := e.PersistRevocationReceipt("run", digest, "live", "capsule-revoke-intent:exact")
	if err != nil || !receipt.CapsuleAbsent || receipt.AssignmentCapabilityDigest != digest || receipt.ReceiptRef == "" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	path := filepath.Join(state, "receipts", "revocation", receiptArtifactName(receipt.ReceiptRef))
	if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0222 != 0 {
		t.Fatalf("durable receipt artifact: %v %v", info, err)
	}
}
