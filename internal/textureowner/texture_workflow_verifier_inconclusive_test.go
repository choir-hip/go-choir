package textureowner

import (
	"errors"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure mode (telemetry batching design 3.3): a crash loses the newest
// tool.result events of a run. The workflow verifier reads tool.result as
// evidence ("worker run creation is backed by parent tool results", a
// successful artifact write, a successful verification command), so a
// legitimately backed run that a restart interrupted would fail verification
// because ruled telemetry loss removed the evidence. Such a failure must be
// reported as inconclusive, not failed; any other failure, and a failure on a
// trajectory no restart touched, stays a failure.
func TestVerifierReportsInconclusiveForInterruptedTrajectory(t *testing.T) {
	missing := fmt.Errorf("missing successful artifact write tool result")
	interrupted := []types.RunRecord{
		{RunID: "run-a", State: types.RunCompleted},
		{RunID: "run-b", State: types.RunCancelled, Error: "interrupted_by_restart"},
	}
	clean := []types.RunRecord{
		{RunID: "run-a", State: types.RunCompleted},
		{RunID: "run-b", State: types.RunCancelled, Error: "cancelled by the owner"},
	}

	err := toolEvidenceError(missing, interrupted)
	if !errors.Is(err, ErrVerificationInconclusive) {
		t.Fatalf("missing tool evidence on an interrupted trajectory = %v, want ErrVerificationInconclusive", err)
	}
	if !errors.Is(err, missing) {
		t.Fatalf("the underlying failure is lost: %v", err)
	}
	if err := toolEvidenceError(missing, clean); errors.Is(err, ErrVerificationInconclusive) || err != missing {
		t.Fatalf("missing tool evidence on a trajectory no restart touched = %v, want the original failure", err)
	}
	if err := toolEvidenceError(nil, interrupted); err != nil {
		t.Fatalf("no failure became %v", err)
	}
	if !trajectoryInterruptedByRestart(interrupted) || trajectoryInterruptedByRestart(clean) || trajectoryInterruptedByRestart(nil) {
		t.Fatal("trajectoryInterruptedByRestart misreads the run fates")
	}
}
