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
	preEval := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:src", ExitCode: 0, WorktreeDigest: "pre-eval-digest",
		SourceTreeDigest: sourceDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	preRef := persistTestExecutionReceipt(t, executor, preEval)
	_, err = executor.ResolveGrantedExecutionReceipts(context.Background(), "run-grant", "handle-grant", []string{preRef})
	if err == nil || !strings.Contains(err.Error(), "final subject") {
		t.Fatalf("pre-eval worktree error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(subject, "README"), []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	finalDigest, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if finalDigest == sourceDigest {
		t.Fatal("expected subject digest to change after write")
	}
	final := ExecutionReceipt{
		AgentRunID: "run-grant", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:src", ExitCode: 0, WorktreeDigest: finalDigest,
		SourceTreeDigest: sourceDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	finalRef := persistTestExecutionReceipt(t, executor, final)
	delete(executor.executionReceipts, finalRef)
	granted, err := executor.ResolveGrantedExecutionReceipts(context.Background(), "run-grant", "handle-grant", []string{finalRef})
	if err != nil {
		t.Fatalf("final subject grant: %v", err)
	}
	if len(granted) != 1 || granted[0].ReceiptRef != finalRef || granted[0].GrantedReceiptRef == "" {
		t.Fatalf("granted receipts = %+v", granted)
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
