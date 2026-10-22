package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// FP4 round 4, P1
// (docs/problems/frozen-candidate-lost-when-complete-cites-a-duplicate-execution-ref-2026-10-22.md).
// A Complete that cites one execution ref twice made one recorded command
// per ref; after the durable freeze the granted binder collapses duplicates,
// the count check failed, and a frozen capsule left the model no cell to
// retry in. Ways the fix can fail, written first: a repeated ref still
// yields two commands; dedupe reorders refs (the proposition digest is
// ordered); dedupe drops a distinct ref; an empty or blank ref survives; the
// count check still wraps a nil error ("%!w(<nil>)") and names no cause.

func TestCompleteExecutionRefsCollapseRepeats(t *testing.T) {
	got := completeExecutionRefs([]string{" capsule-exec:sha256:a ", "capsule-exec:sha256:b", "", "capsule-exec:sha256:a", "  ", "capsule-go-eval:sha256:c", "capsule-exec:sha256:b"})
	want := []string{"capsule-exec:sha256:a", "capsule-exec:sha256:b", "capsule-go-eval:sha256:c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completeExecutionRefs = %q, want %q (first occurrence order, repeats and blanks dropped)", got, want)
	}
}

func TestExecutionEvidenceCountErrorNamesTheCause(t *testing.T) {
	err := executionEvidenceCountError("assignment command evidence unavailable after durable freeze", 5, 4, nil)
	if err == nil {
		t.Fatal("a count mismatch with no resolver error returned no error")
	}
	if strings.Contains(err.Error(), "%!w") || !strings.Contains(err.Error(), "5") || !strings.Contains(err.Error(), "4") {
		t.Fatalf("count mismatch error does not name the counts: %v", err)
	}
	cause := errors.New("capsule execution evidence requires frozen capsule")
	wrapped := executionEvidenceCountError("assignment command evidence unavailable after durable freeze", 5, 0, cause)
	if !errors.Is(wrapped, cause) {
		t.Fatalf("a resolver error is not wrapped: %v", wrapped)
	}
}

// seedExecutionReceipt writes a durable execution receipt the executor
// opens by ref, bound to capsuleID and runID, and returns the ref.
func seedExecutionReceipt(t *testing.T, root, runID, capsuleID, command string) string {
	t.Helper()
	receipt := capsule.ExecutionReceipt{
		AgentRunID: runID, CapabilityHandleDigest: strings.Repeat("a", 64), CapsuleID: capsuleID,
		Command: command, Cwd: "/work", StdoutDigest: objectgraph.SHA256([]byte("out")), StderrDigest: objectgraph.SHA256([]byte("")),
		WorktreeDigest: objectgraph.SHA256([]byte("tree")), SourceTreeDigest: objectgraph.SHA256([]byte("tree")),
		OccurredAt: "2026-10-22T14:00:00Z",
	}
	canonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.ReceiptRef = "capsule-exec:sha256:" + computerevent.DigestBytes(canonical)
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(receipt.ReceiptRef))
	dir := filepath.Join(root, "receipts", "execution")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return receipt.ReceiptRef
}

// P1 through its real caller (review round 4b, both reviewers): a Complete
// that cites one execution ref twice commits one command per distinct ref.
// A partial report resolves its receipts without a frozen capsule, through
// the same refs commitCompleteIntent builds, so this pins that line.
func TestCompleteCitingOneRefTwiceRecordsOneCommandPerReceipt(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.open(0)
	bound := f.bind(0)
	stateDir := t.TempDir()
	exec := capsule.NewExecutor(stateDir, t.TempDir(), "", 0)
	f.rt.capsuleExecutor = exec
	ref := seedExecutionReceipt(t, stateDir, bound.BoundRunID, bound.Binding.CapsuleID, "go vet ./...")
	other := seedExecutionReceipt(t, stateDir, bound.BoundRunID, bound.Binding.CapsuleID, "go build ./...")

	rec := &types.RunRecord{
		RunID: bound.BoundRunID, OwnerID: bound.Binding.OwnerID, ComputerID: bound.Binding.ComputerID,
		AgentID:  bound.Binding.AssignedAgentID,
		Metadata: map[string]any{"assignment_id": bound.AssignmentID, "assignment_attempt": fmt.Sprint(bound.Binding.Attempt)},
	}
	r := &rlmCallReduction{active: true, rec: rec, mb: f.rt, toolCtx: &CapsuleToolCtx{Executor: exec}, scope: ReductionScope{RunID: rec.RunID}}
	terminal, err := r.commitCompleteIntent(context.Background(), yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentComplete, Result: "partial", Verdict: "none", Summary: "progress, citing one receipt twice",
		ExecutionRefs: []string{ref, other, ref},
	})
	if err != nil {
		t.Fatalf("a Complete citing one ref twice failed: %v", err)
	}
	if terminal {
		t.Fatal("fixture: a partial report is not terminal")
	}
	after := f.get(0)
	if len(after.ReportRefs) == 0 {
		t.Fatal("no report was recorded")
	}
	report, err := f.s.GetEngineeringAssignmentReportByCanonicalID(context.Background(), after.ReportRefs[len(after.ReportRefs)-1])
	if err != nil {
		t.Fatal(err)
	}
	var cited []string
	for _, command := range report.Commands {
		cited = append(cited, command.ExecutionRef)
	}
	if !reflect.DeepEqual(cited, []string{ref, other}) {
		t.Fatalf("recorded commands cite %q, want one per distinct receipt in first-cited order %q", cited, []string{ref, other})
	}
}
