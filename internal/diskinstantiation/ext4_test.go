package diskinstantiation

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExt4BackendFreshSparseGeometryAndReconstruction(t *testing.T) {
	mkfs, err := exec.LookPath("mkfs.ext4")
	if err != nil {
		t.Skip("mkfs.ext4 is unavailable")
	}
	dumpe2fs, err := exec.LookPath("dumpe2fs")
	if err != nil {
		t.Skip("dumpe2fs is unavailable")
	}

	backend := Ext4Backend{WorkRoot: t.TempDir(), MkfsBinary: mkfs, Dumpe2fsBinary: dumpe2fs}
	plan := testPlan()
	populate := func(_ context.Context, root string) error {
		if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, "files", "proof.txt"), []byte("constructed"), 0o644)
	}

	first, err := backend.Instantiate(context.Background(), plan, populate)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceipt(plan, first); err != nil {
		t.Fatal(err)
	}
	if first.Geometry.AllocatedBytes >= first.Geometry.DeviceLogicalBytes || first.Geometry.AllocatedBytes > plan.Allocation.MaxAllocatedBytes {
		t.Fatalf("sparse allocation violated: %+v", first.Geometry)
	}
	if _, err := backend.Instantiate(context.Background(), plan, populate); err == nil {
		t.Fatal("expected realization identity reuse refusal")
	}

	reclaim, err := backend.Reclaim(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if reclaim.InstantiationReceiptID != first.ID || reclaim.ReclaimedBytes == 0 {
		t.Fatalf("invalid reclaim receipt: %+v", reclaim)
	}
	if _, err := os.Stat(first.DevicePath); !os.IsNotExist(err) {
		t.Fatalf("reclaimed device remains available: %v", err)
	}

	retried, err := backend.Instantiate(context.Background(), plan, populate)
	if err != nil {
		t.Fatalf("reconstruct same realization after complete reclaim: %v", err)
	}
	if _, err := backend.Reclaim(context.Background(), retried); err != nil {
		t.Fatalf("reclaim retried realization: %v", err)
	}

	secondPlan := plan
	secondPlan.RealizationID = "vm-candidate-2"
	second, err := backend.Instantiate(context.Background(), secondPlan, populate)
	if err != nil {
		t.Fatal(err)
	}
	if second.DevicePath == first.DevicePath || second.ID == first.ID {
		t.Fatalf("reconstruction reused realization evidence: first=%+v second=%+v", first, second)
	}
	if err := VerifyReceipt(secondPlan, second); err != nil {
		t.Fatal(err)
	}
}

func TestExt4BackendChurnReclaimReconstructionBound(t *testing.T) {
	mkfs, err := exec.LookPath("mkfs.ext4")
	if err != nil {
		t.Skip("mkfs.ext4 is unavailable")
	}
	dumpe2fs, err := exec.LookPath("dumpe2fs")
	if err != nil {
		t.Skip("dumpe2fs is unavailable")
	}
	debugfs, err := exec.LookPath("debugfs")
	if err != nil {
		t.Skip("debugfs is unavailable")
	}
	backend := Ext4Backend{WorkRoot: t.TempDir(), MkfsBinary: mkfs, Dumpe2fsBinary: dumpe2fs}
	plan := testPlan()
	populate := func(context.Context, string) error { return nil }
	initial, err := backend.Instantiate(t.Context(), plan, populate)
	if err != nil {
		t.Fatal(err)
	}
	payloadPath := filepath.Join(t.TempDir(), "cache.bin")
	pattern := bytes.Repeat([]byte("choir-cache-churn-"), 2048)
	payload := bytes.Repeat(pattern, (32<<20)/len(pattern)+1)[:32<<20]
	if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	writeCmd := exec.Command(debugfs, "-w", "-R", "write "+payloadPath+" /cache.bin", initial.DevicePath)
	if output, err := writeCmd.CombinedOutput(); err != nil {
		t.Fatalf("debugfs churn write: %v: %s", err, output)
	}
	churned, err := backend.Inspect(t.Context(), initial)
	if err != nil {
		t.Fatal(err)
	}
	if churned.AllocatedBytes <= initial.Geometry.AllocatedBytes {
		t.Fatalf("cache churn did not increase host allocation: initial=%d churned=%d", initial.Geometry.AllocatedBytes, churned.AllocatedBytes)
	}
	deleteCmd := exec.Command(debugfs, "-w", "-R", "rm /cache.bin", initial.DevicePath)
	if output, err := deleteCmd.CombinedOutput(); err != nil {
		t.Fatalf("debugfs churn delete: %v: %s", err, output)
	}
	afterDelete, err := backend.Inspect(t.Context(), initial)
	if err != nil {
		t.Fatal(err)
	}
	reclaim, err := backend.Reclaim(t.Context(), initial)
	if err != nil {
		t.Fatal(err)
	}
	if reclaim.ReclaimedBytes != afterDelete.AllocatedBytes || reclaim.ReclaimedBytes == 0 {
		t.Fatalf("reclaim did not receipt physical churn allocation: reclaim=%+v after_delete=%+v", reclaim, afterDelete)
	}
	reconstructed, err := backend.Instantiate(t.Context(), plan, populate)
	if err != nil {
		t.Fatal(err)
	}
	if reconstructed.Geometry.AllocatedBytes > plan.Allocation.MaxAllocatedBytes || reconstructed.Geometry.AllocatedBytes >= afterDelete.AllocatedBytes {
		t.Fatalf("reconstruction did not reset churn allocation within bound: reconstructed=%+v after_delete=%+v", reconstructed.Geometry, afterDelete)
	}
}
