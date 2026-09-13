//go:build linux

package capsule

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestResolveGrantedExecutionReceiptsBindsFinalSubjectNotPreEval(t *testing.T) {
	state := t.TempDir()
	merged := filepath.Join(t.TempDir(), "root")
	subject := filepath.Join(merged, "workspace", "platform")
	if err := os.MkdirAll(subject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "README"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	caps := &Capsule{ID: "capsule-grant", State: StateFrozen, MergedDir: merged, SourceSnapshotDigest: sourceDigest}
	capability := &Capability{
		CapabilityID: "cap-grant", Handle: "handle-grant", AgentRunID: "run-grant",
		AgentRole: RoleCoSuper, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour),
	}
	executor := &Executor{
		stateDir:          state,
		capsules:          map[string]*Capsule{caps.ID: caps},
		capabilities:      map[capKey]*Capability{{AgentRunID: "run-grant", Handle: "handle-grant"}: capability},
		revokedCaps:       map[string]bool{},
		executionReceipts: map[string]ExecutionReceipt{},
		grantedReceipts:   map[string]GrantedExecutionReceipt{},
	}
	handleDigest := computerevent.DigestBytes([]byte("handle-grant"))
	preOccurrence := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	midOccurrence := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	preEval := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:read", ExitCode: 0, WorktreeDigest: "pre-eval-digest",
		SourceTreeDigest: sourceDigest, OccurredAt: preOccurrence,
	}
	preRef := persistTestExecutionReceipt(t, executor, preEval)
	// A single earlier receipt cannot certify the frozen final subject.
	if _, err = executor.ResolveGrantedExecutionReceipts(context.Background(), "run-grant", "handle-grant", []string{preRef}); err == nil || !strings.Contains(err.Error(), "final frozen subject") {
		t.Fatalf("pre-eval-only error = %v", err)
	}
	// A failing command is a recorded observation, not a binding failure.
	if err := os.WriteFile(filepath.Join(subject, "MARKER"), []byte("carrier-check\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	finalDigest, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if finalDigest == sourceDigest {
		t.Fatal("expected subject digest to change after write")
	}
	mid := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:write", ExitCode: 0, WorktreeDigest: finalDigest,
		SourceTreeDigest: sourceDigest, OccurredAt: midOccurrence,
	}
	midRef := persistTestExecutionReceipt(t, executor, mid)
	vet := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:vet", ExitCode: 1, WorktreeDigest: finalDigest,
		SourceTreeDigest: sourceDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	vetRef := persistTestExecutionReceipt(t, executor, vet)
	granted, err := executor.ResolveGrantedExecutionReceipts(context.Background(), "run-grant", "handle-grant", []string{preRef, midRef, vetRef})
	if err != nil {
		t.Fatalf("multi-cell grant: %v", err)
	}
	if len(granted) != 3 || granted[2].ReceiptRef != vetRef || granted[2].ExitCode != 1 {
		t.Fatalf("granted receipts = %+v", granted)
	}
	// The chronologically latest receipt must bind the frozen subject: an
	// out-of-order citation whose latest receipt predates the freeze refuses.
	stale := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:stale", ExitCode: 0, WorktreeDigest: "pre-eval-digest",
		SourceTreeDigest: sourceDigest, OccurredAt: time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano),
	}
	staleRef := persistTestExecutionReceipt(t, executor, stale)
	if _, err = executor.ResolveGrantedExecutionReceipts(context.Background(), "run-grant", "handle-grant", []string{preRef, staleRef}); err == nil || !strings.Contains(err.Error(), "final frozen subject") {
		t.Fatalf("stale-final error = %v", err)
	}
}

func persistTestExecutionReceipt(t *testing.T, executor *Executor, receipt ExecutionReceipt) string {
	t.Helper()
	canonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.ReceiptRef = "capsule-go-eval:sha256:" + computerevent.DigestBytes(canonical)
	stored, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.persistReceiptArtifact("execution", receipt.ReceiptRef, stored); err != nil {
		t.Fatal(err)
	}
	executor.executionReceipts[receipt.ReceiptRef] = receipt
	return receipt.ReceiptRef
}
