package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineering ids minted after the vocabulary cutover must be fixed points of
// the frozen rule, or a from-genesis replay rewrites them and the live store
// stops being replayable
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md,
// rerun 8). Renaming them must not invalidate records already written under
// the legacy co-super- spelling. Failure modes pinned: a digest ref minted
// with the V1 infix; a legacy digest ref refused after the rename; a digest
// ref with a different digest accepted under either spelling.
func TestEngineeringDigestRefsAreServingVocabularyAndAcceptLegacy(t *testing.T) {
	grant, err := grantAttestationRef(types.EngineeringGrantPolicyAttestation{AssignmentID: "assignment-vocab", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := executionAttestationRef(types.EngineeringExecutionAttestation{AssignmentID: "assignment-vocab", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	fate, err := fateStepRef(types.EngineeringCapsuleFateStep{AssignmentID: "assignment-vocab", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{grant, execution, fate} {
		if !IsServingVocabularyLeaf(ref) {
			t.Fatalf("digest ref %q is not serving vocabulary", ref)
		}
		if !engineeringDigestRefMatches(ref, ref) {
			t.Fatalf("digest ref %q does not match itself", ref)
		}
		_, digest, _ := strings.Cut(ref, ":")
		kind, _, _ := strings.Cut(strings.TrimPrefix(ref, "engineering-"), ":")
		legacy := "co-super-" + kind + ":" + digest
		if !engineeringDigestRefMatches(legacy, ref) {
			t.Fatalf("legacy ref %q refused against %q", legacy, ref)
		}
		other := legacy[:len(legacy)-1] + "0"
		if other == legacy {
			other = legacy[:len(legacy)-1] + "1"
		}
		if engineeringDigestRefMatches(other, ref) {
			t.Fatalf("ref with a different digest %q accepted against %q", other, ref)
		}
	}
	if engineeringDigestRefMatches("co-super-grant:"+strings.SplitN(execution, ":", 2)[1], execution) {
		t.Fatal("legacy ref of another kind accepted")
	}
}

// A report recorded before the rename replays under the command id that
// recorded it, not the id the caller derives today.
func TestReplayRecordedReportUsesRecordedLegacyCommandID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, 0, "assignment-legacy-report", 1, types.EngineeringAssignmentVerification, true, "cap", "capsule")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap")); err != nil {
		t.Fatal(err)
	}
	req := assignmentReportRequest(open, 2, "report-legacy", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictPass)
	req.CommandID = "co-super-report:" + open.AssignmentID + ":report-legacy"
	req.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(req)
	reported, err := s.RecordEngineeringAssignmentReport(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.ReplayRecordedEngineeringAssignmentReport(ctx, f.ownerID, f.computerID, open.AssignmentID, 1, "report-legacy", "engineering-report:"+open.AssignmentID+":report-legacy")
	if err != nil || !replay.Replay || replay.Receipt.CommandID != req.CommandID || replay.Receipt.CommandDigest != reported.Receipt.CommandDigest {
		t.Fatalf("legacy report replay: %+v %v", replay, err)
	}
}
