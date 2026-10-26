//go:build linux

package capsule

import (
	"context"
	"errors"
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
		AgentRole: RoleEngineering, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour),
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

// The visual drill's case through the real resolver: receipts, then a
// receipt-less mutation of the worktree. A completed claim (certified) is
// refused; a blocked one (observed) is granted, and its granted receipt
// records the frozen digest and the latest receipt's digest side by side.
func TestResolveGrantedExecutionReceiptsForObservedBindsAfterReceiptlessMutation(t *testing.T) {
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
	caps := &Capsule{ID: "capsule-observed", State: StateFrozen, MergedDir: merged, SourceSnapshotDigest: sourceDigest}
	capability := &Capability{
		CapabilityID: "cap-observed", Handle: "handle-observed", AgentRunID: "run-observed",
		AgentRole: RoleEngineering, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour),
	}
	executor := &Executor{
		stateDir:          state,
		capsules:          map[string]*Capsule{caps.ID: caps},
		capabilities:      map[capKey]*Capability{{AgentRunID: "run-observed", Handle: "handle-observed"}: capability},
		revokedCaps:       map[string]bool{},
		executionReceipts: map[string]ExecutionReceipt{},
		grantedReceipts:   map[string]GrantedExecutionReceipt{},
	}
	if err := os.WriteFile(filepath.Join(subject, "page.html"), []byte("page\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	observedByReceipt, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	receipt := persistTestExecutionReceipt(t, executor, ExecutionReceipt{
		AgentRunID: "run-observed", CapabilityHandleDigest: computerevent.DigestBytes([]byte("handle-observed")), CapsuleID: caps.ID,
		Command: "go_eval:write", ExitCode: 0, WorktreeDigest: observedByReceipt,
		SourceTreeDigest: sourceDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	// Timed-out build cells change the tree and mint no receipt.
	if err := os.WriteFile(filepath.Join(subject, "dist-partial"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	frozen, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err = executor.ResolveGrantedExecutionReceipts(ctx, "run-observed", "handle-observed", []string{receipt}); err == nil || !strings.Contains(err.Error(), "final frozen subject") {
		t.Fatalf("completed-style binding of a receipt-less mutation = %v", err)
	}
	granted, err := executor.ResolveGrantedExecutionReceiptsFor(ctx, "run-observed", "handle-observed", []string{receipt}, SubjectObserved)
	if err != nil || len(granted) != 1 || granted[0].GrantedReceiptRef == "" {
		t.Fatalf("blocked-style binding = %+v, %v", granted, err)
	}
	opened, err := executor.OpenGrantedExecutionReceipt(granted[0].GrantedReceiptRef)
	if err != nil {
		t.Fatalf("open granted receipt: %v", err)
	}
	if opened.SubjectBinding != "observed" || opened.FinalSubjectDigest != frozen || opened.LatestReceiptWorktreeDigest != observedByReceipt ||
		opened.Execution.WorktreeDigest != observedByReceipt || opened.FinalSubjectDigest == opened.LatestReceiptWorktreeDigest {
		t.Fatalf("granted receipt does not record both digests: %+v", opened)
	}
}

// R1 (docs/problems/engineering-completion-path-cluster-2026-10-20.md): the
// pre-check binds the cited receipts against the live tree before the
// terminal saga's freeze intent, with the same rules, and changes nothing: it
// neither freezes the capsule nor mints a granted receipt.
func TestPreflightGrantedExecutionReceiptsChecksTheLiveTreeAndChangesNothing(t *testing.T) {
	state := t.TempDir()
	merged := filepath.Join(t.TempDir(), "root")
	subject := filepath.Join(merged, "workspace", "platform")
	if err := os.MkdirAll(subject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "App.svelte"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sourceDigest, err := digestCanonicalSubjectTree(ctx, subject)
	if err != nil {
		t.Fatal(err)
	}
	caps := &Capsule{ID: "capsule-preflight", State: StateActive, MergedDir: merged, SourceSnapshotDigest: sourceDigest}
	capability := &Capability{
		CapabilityID: "cap-preflight", Handle: "handle-preflight", AgentRunID: "run-preflight",
		AgentRole: RoleEngineering, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour),
	}
	executor := &Executor{
		stateDir:          state,
		capsules:          map[string]*Capsule{caps.ID: caps},
		capabilities:      map[capKey]*Capability{{AgentRunID: "run-preflight", Handle: "handle-preflight"}: capability},
		revokedCaps:       map[string]bool{},
		executionReceipts: map[string]ExecutionReceipt{},
		grantedReceipts:   map[string]GrantedExecutionReceipt{},
	}
	handleDigest := computerevent.DigestBytes([]byte("handle-preflight"))
	if err := os.WriteFile(filepath.Join(subject, "App.svelte"), []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := digestCanonicalSubjectTree(ctx, subject)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Minute)
	build := persistTestExecutionReceipt(t, executor, ExecutionReceipt{
		AgentRunID: "run-preflight", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "npm run build", WorktreeDigest: edited, SourceTreeDigest: sourceDigest, OccurredAt: base.Format(time.RFC3339Nano),
	})
	// Round 5c: the latest cited receipt observed the pre-edit tree.
	stale := persistTestExecutionReceipt(t, executor, ExecutionReceipt{
		AgentRunID: "run-preflight", CapabilityHandleDigest: handleDigest, CapsuleID: caps.ID,
		Command: "go_eval:capture", WorktreeDigest: sourceDigest, SourceTreeDigest: sourceDigest, OccurredAt: base.Add(time.Second).Format(time.RFC3339Nano),
	})

	err = executor.PreflightGrantedExecutionReceipts(ctx, "run-preflight", "handle-preflight", []string{build, stale}, SubjectCertified)
	if !errors.Is(err, ErrFinalSubjectNotCertified) {
		t.Fatalf("certified pre-check with a stale latest receipt = %v, want ErrFinalSubjectNotCertified", err)
	}
	if err := executor.PreflightGrantedExecutionReceipts(ctx, "run-preflight", "handle-preflight", []string{build, stale}, SubjectObserved); err != nil {
		t.Fatalf("observed pre-check of the same receipts: %v", err)
	}
	if err := executor.PreflightGrantedExecutionReceipts(ctx, "run-preflight", "handle-preflight", []string{build}, SubjectCertified); err != nil {
		t.Fatalf("certified pre-check of a receipt that observed the live tree: %v", err)
	}
	if err := executor.PreflightGrantedExecutionReceipts(ctx, "run-other", "handle-preflight", []string{build}, SubjectObserved); err == nil {
		t.Fatal("a pre-check under another run's capability passed")
	}
	if caps.State != StateActive {
		t.Fatalf("the pre-check left the capsule %s, want active", caps.State)
	}
	if len(executor.grantedReceipts) != 0 {
		t.Fatalf("the pre-check minted %d granted receipt(s)", len(executor.grantedReceipts))
	}
	if entries, _ := os.ReadDir(filepath.Join(executor.artifactRoot(), "receipts", "granted")); len(entries) != 0 {
		t.Fatalf("the pre-check persisted %d granted receipt artifact(s)", len(entries))
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
