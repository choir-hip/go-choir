package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Visual drill, docs/problems/engineering-completion-path-cluster-2026-10-20.md
// failure 2: receipts, then three timed-out build cells that changed the
// worktree with no receipt, then Complete("blocked") was refused because the
// latest receipt did not observe the frozen tree. A fate that claims nothing
// about the subject (observed) binds anyway and records both digests; a claim
// about the subject (certified: completed, Freeze) is still refused.
func TestBindGrantedReceiptsReceiptlessMutationBlocksOnlyACertifiedClaim(t *testing.T) {
	ctx := context.Background()
	subject := filepath.Join(t.TempDir(), "workspace", "platform")
	if err := os.MkdirAll(subject, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := func() string {
		t.Helper()
		d, err := digestCanonicalSubjectTree(ctx, subject)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(subject, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README")
	source := digest()
	write("toolchain-recorded")
	atToolchain := digest()
	write("page.html")
	atWrite := digest() // the last cited receipt observes this tree
	// Three timed-out builds mutate the worktree and mint no receipt.
	write("frontend-dist-partial")
	frozen := digest()
	if frozen == atWrite || atWrite == atToolchain || atToolchain == source {
		t.Fatalf("test trees do not differ: %s %s %s %s", source, atToolchain, atWrite, frozen)
	}

	mint := func(ref, at, tree string) ExecutionReceipt {
		return ExecutionReceipt{ReceiptRef: ref, AgentRunID: "run", CapabilityHandleDigest: "handle", CapsuleID: "capsule",
			Command: ref, WorktreeDigest: tree, SourceTreeDigest: source, OccurredAt: at}
	}
	issued := map[string]ExecutionReceipt{
		"toolchain": mint("toolchain", "2026-10-20T12:00:01Z", atToolchain),
		"write":     mint("write", "2026-10-20T12:00:02Z", atWrite),
	}
	open := func(ref string) (ExecutionReceipt, error) {
		if r, ok := issued[ref]; ok {
			return r, nil
		}
		return ExecutionReceipt{}, fmt.Errorf("executor receipt unavailable: %s", ref)
	}
	refs := []string{"toolchain", "write"}

	// Complete("completed"): refused, naming both digests.
	_, err := bindGrantedReceipts(open, refs, SubjectCertified, "run", "handle", "capsule", source, frozen)
	if err == nil || !strings.Contains(err.Error(), "does not bind the final frozen subject") ||
		!strings.Contains(err.Error(), atWrite) || !strings.Contains(err.Error(), frozen) {
		t.Fatalf("certified binding of a receipt-less mutation = %v", err)
	}
	// The terminal saga ends an assignment on this refusal alone (R1's
	// race), so it is typed; an authenticity refusal is not.
	if !errors.Is(err, ErrFinalSubjectNotCertified) {
		t.Fatalf("the final-subject refusal is not ErrFinalSubjectNotCertified: %v", err)
	}
	if _, foreignErr := bindGrantedReceipts(open, refs, SubjectCertified, "another-run", "handle", "capsule", source, frozen); foreignErr == nil || errors.Is(foreignErr, ErrFinalSubjectNotCertified) {
		t.Fatalf("a foreign-run refusal = %v; want a refusal that is not the final-subject one", foreignErr)
	}
	// Complete("blocked"): binds, and reports the latest receipt's digest for
	// the side-by-side record next to the frozen digest.
	bound, err := bindGrantedReceipts(open, refs, SubjectObserved, "run", "handle", "capsule", source, frozen)
	if err != nil {
		t.Fatalf("observed binding refused an honest failure: %v", err)
	}
	if len(bound.Receipts) != 2 || bound.LatestWorktreeDigest != atWrite {
		t.Fatalf("observed binding = %+v, want two receipts and latest digest %s", bound, atWrite)
	}
	// A receipt that observes the frozen tree still certifies it.
	issued["rebuilt"] = mint("rebuilt", "2026-10-20T12:00:03Z", frozen)
	if _, err := bindGrantedReceipts(open, []string{"toolchain", "rebuilt"}, SubjectCertified, "run", "handle", "capsule", source, frozen); err != nil {
		t.Fatalf("certified binding of a covering receipt: %v", err)
	}
}

// The relaxed binding still authenticates every cited receipt: a false
// citation is not an honest report.
func TestBindGrantedReceiptsObservedStillBindsRunHandleCapsuleAndSource(t *testing.T) {
	base := ExecutionReceipt{AgentRunID: "run", CapabilityHandleDigest: "handle", CapsuleID: "capsule",
		SourceTreeDigest: "source", WorktreeDigest: "w1", OccurredAt: "2026-10-20T12:00:01Z"}
	for _, tc := range []struct {
		name   string
		mutate func(*ExecutionReceipt)
		want   string
	}{
		{"run", func(r *ExecutionReceipt) { r.AgentRunID = "other" }, "run"},
		{"handle", func(r *ExecutionReceipt) { r.CapabilityHandleDigest = "other" }, "handle"},
		{"capsule", func(r *ExecutionReceipt) { r.CapsuleID = "other" }, "capsule"},
		{"source", func(r *ExecutionReceipt) { r.SourceTreeDigest = "other" }, "frozen source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receipt := base
			tc.mutate(&receipt)
			_, err := bindGrantedReceipts(func(string) (ExecutionReceipt, error) { return receipt, nil },
				[]string{"r"}, SubjectObserved, "run", "handle", "capsule", "source", "frozen")
			if err == nil || !strings.Contains(err.Error(), "does not bind the exact run, handle, capsule, and frozen source: "+tc.want) {
				t.Fatalf("observed binding of a foreign %s receipt = %v", tc.name, err)
			}
		})
	}
	_, err := bindGrantedReceipts(func(ref string) (ExecutionReceipt, error) {
		return ExecutionReceipt{}, fmt.Errorf("executor receipt unavailable: %s", ref)
	}, []string{"capsule-exec:sha256:typo"}, SubjectObserved, "run", "handle", "capsule", "source", "frozen")
	if err == nil || !strings.Contains(err.Error(), "capsule-exec:sha256:typo") {
		t.Fatalf("observed binding of an unknown ref = %v", err)
	}
	if _, err := bindGrantedReceipts(nil, []string{"r"}, SubjectBinding("anything-else"), "run", "handle", "capsule", "source", "frozen"); err == nil {
		t.Fatal("an unknown binding was accepted")
	}
}

// Zero receipts: a fate that cites nothing records (the observed binding has
// nothing to certify); a certified claim needs evidence. Duplicates collapse
// and the latest receipt is by time, not by citation order.
func TestBindGrantedReceiptsEmptyDuplicateAndOrder(t *testing.T) {
	if got, err := bindGrantedReceipts(nil, nil, SubjectObserved, "run", "handle", "capsule", "source", "frozen"); err != nil || len(got.Receipts) != 0 {
		t.Fatalf("observed binding of no receipts = %+v, %v", got, err)
	}
	if _, err := bindGrantedReceipts(nil, nil, SubjectCertified, "run", "handle", "capsule", "source", "frozen"); err == nil || !strings.Contains(err.Error(), "capsule execution evidence is required") {
		t.Fatalf("certified binding of no receipts = %v", err)
	}
	early := ExecutionReceipt{ReceiptRef: "early", AgentRunID: "run", CapabilityHandleDigest: "handle", CapsuleID: "capsule", SourceTreeDigest: "source", WorktreeDigest: "A", OccurredAt: "2026-10-20T12:00:01Z"}
	late := ExecutionReceipt{ReceiptRef: "late", AgentRunID: "run", CapabilityHandleDigest: "handle", CapsuleID: "capsule", SourceTreeDigest: "source", WorktreeDigest: "B", OccurredAt: "2026-10-20T12:00:09Z"}
	open := func(ref string) (ExecutionReceipt, error) {
		if ref == "early" {
			return early, nil
		}
		return late, nil
	}
	got, err := bindGrantedReceipts(open, []string{"late", "early", "late"}, SubjectObserved, "run", "handle", "capsule", "source", "frozen")
	if err != nil || len(got.Receipts) != 2 || got.LatestWorktreeDigest != "B" {
		t.Fatalf("out-of-order citation = %+v, %v", got, err)
	}
}

// A certified granted receipt keeps its canonical bytes (and so its digest):
// the observed fields are omitted when empty.
func TestGrantedExecutionReceiptCertifiedBytesUnchanged(t *testing.T) {
	raw, err := json.Marshal(GrantedExecutionReceipt{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "subject_binding") || strings.Contains(string(raw), "latest_receipt_worktree_digest") {
		t.Fatalf("certified granted receipt gained observed fields: %s", raw)
	}
	raw, _ = json.Marshal(GrantedExecutionReceipt{SubjectBinding: string(SubjectObserved), LatestReceiptWorktreeDigest: "x"})
	if !strings.Contains(string(raw), `"subject_binding":"observed"`) {
		t.Fatalf("observed granted receipt does not say so: %s", raw)
	}
}
