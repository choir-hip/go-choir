//go:build linux

package capsule

import (
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
	receipt, err := e.PersistRevocationReceipt("run", digest, "live", "capsule-revoke-intent:exact")
	if err != nil || !receipt.CapsuleAbsent || receipt.AssignmentCapabilityDigest != digest || receipt.ReceiptRef == "" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	path := filepath.Join(state, "receipts", "revocation", receiptArtifactName(receipt.ReceiptRef))
	if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0222 != 0 {
		t.Fatalf("durable receipt artifact: %v %v", info, err)
	}
}
