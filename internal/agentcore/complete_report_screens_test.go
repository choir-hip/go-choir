package agentcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management's judge reads the assignment Report, not the Freeze result, so
// the platform appends the screens a Freeze saved. Ways it can fail, written
// first: a Freeze with screens leaves the Report bare; a Freeze with none
// (or no Freeze) adds a heading with an empty list; the engineer's own text
// is replaced instead of kept; a ref in the list is not the bundle's digest.

func completeReportSummary(t *testing.T, screens []capsule.ScreenFile, freeze bool) string {
	t.Helper()
	ctx := context.Background()
	f := newAdmissionFixture(t, 1)
	f.open(0)
	bound := f.bind(0)
	computerID := bound.Binding.ComputerID
	operations, err := selfdev.NewStore(f.s, f.s)
	if err != nil {
		t.Fatal(err)
	}
	updaterRoot := t.TempDir()
	filesRoot := t.TempDir()
	if freeze {
		bundle := transaction.CapsuleEffectBundle{
			BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
			TrajectoryRef: "trajectory-x", CapsuleIdentity: "capsule-x", VerifierReceipts: []string{}, Screens: screens,
		}
		bundle.ContentDigest, err = bundle.ComputeContentDigest()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := computerevent.CanonicalJSON(bundle)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(updaterRoot, "incoming", bundle.ContentDigest)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bundle.draft.json"), raw, 0o400); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		if _, err := f.s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
			"op-report", computerID, store.EngineeringFreezeIdempotencyKey(bound.AssignmentID), strings.Repeat("d", 64), "trajectory-x", "capsule-x", strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), bundle.ContentDigest, strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateFrozen, now, now); err != nil {
			t.Fatal(err)
		}
	}
	rec := &types.RunRecord{
		RunID: bound.BoundRunID, OwnerID: bound.Binding.OwnerID, ComputerID: computerID, AgentID: bound.Binding.AssignedAgentID,
		Metadata: map[string]any{"assignment_id": bound.AssignmentID, "assignment_attempt": fmt.Sprint(bound.Binding.Attempt)},
	}
	exec := capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0)
	f.rt.capsuleExecutor = exec
	r := &rlmCallReduction{active: true, rec: rec, mb: f.rt, scope: ReductionScope{RunID: rec.RunID},
		toolCtx: &CapsuleToolCtx{Executor: exec, OperationStore: operations, ComputerID: computerID, UpdaterRoot: updaterRoot, FilesRoot: filesRoot}}
	if _, err := r.commitCompleteIntent(ctx, yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentComplete, Result: "partial", Verdict: "none", Summary: "my own words: built the slide stage",
	}); err != nil {
		t.Fatal(err)
	}
	after := f.get(0)
	if len(after.ReportRefs) == 0 {
		t.Fatal("no report was recorded")
	}
	report, err := f.s.GetEngineeringAssignmentReportByCanonicalID(ctx, after.ReportRefs[len(after.ReportRefs)-1])
	if err != nil {
		t.Fatal(err)
	}
	return report.Summary
}

func TestCompleteReportCarriesTheScreensTheFreezeSaved(t *testing.T) {
	_, screens, _ := screenFixture(t)
	got := completeReportSummary(t, screens, true)
	if !strings.HasPrefix(got, "my own words: built the slide stage") {
		t.Fatalf("the engineer's text was not preserved: %q", got)
	}
	want := "Screens saved at Freeze (platform-attached):\n- home.png: artifact:sha256:" + screens[0].SHA256 + " (40x30, not saved)"
	if !strings.Contains(got, want) {
		t.Fatalf("report lacks the platform-attached screens\n got: %q\nwant: %q", got, want)
	}
}

func TestCompleteReportWithoutScreensHasNoScreensList(t *testing.T) {
	for name, tc := range map[string]struct {
		screens []capsule.ScreenFile
		freeze  bool
	}{"freeze without screens": {nil, true}, "no freeze": {nil, false}} {
		got := completeReportSummary(t, tc.screens, tc.freeze)
		if got != "my own words: built the slide stage" {
			t.Errorf("%s: report summary = %q, want the engineer's text alone", name, got)
		}
	}
}
