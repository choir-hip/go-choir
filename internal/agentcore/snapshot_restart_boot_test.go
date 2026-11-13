package agentcore

import (
	"context"
	"testing"
	"time"
)

// F3: a capture the host's kill cut never returned, so its start is still
// counted. A planned boot clears the count (a planned kill is not a crash);
// a crash boot keeps it (that is what the guard is for).
func TestAPlannedBootClearsTheSnapshotAttemptGuardAndACrashBootKeepsIt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		planned bool
		want    int
	}{{"planned boot", true, 0}, {"crash boot", false, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := testRuntime(t)
			rt.cfg.ComputerID = "computer-attempt-guard"
			for i := 0; i < 2; i++ {
				if err := spendGuestAttempt(rt.cfg.StorePath, snapshotAttemptKey, rt.cfg.ComputerID, snapshotAttemptLimit); err != nil {
					t.Fatal(err)
				}
			}
			rt.bootRestartPlanned = tc.planned
			rt.settleSnapshotAttemptAtBoot()
			n, err := peekGuestAttempts(rt.cfg.StorePath, snapshotAttemptKey, rt.cfg.ComputerID)
			if err != nil || n != tc.want {
				t.Fatalf("attempt count after a %s = %d (%v), want %d", tc.name, n, err, tc.want)
			}
		})
	}
}

// F7: the checkpoint after a cancelled capture runs on a context that is
// alive, bounded, and detached from the cancel.
func TestTheCheckpointAfterACancelledCaptureGetsALiveBoundedContext(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errSnapshotCancelledForRestart)
	cpCtx, stop := checkpointContext(ctx)
	defer stop()
	if cpCtx.Err() != nil {
		t.Fatalf("checkpoint context already ended: %v", cpCtx.Err())
	}
	if dl, ok := cpCtx.Deadline(); !ok || time.Until(dl) > snapshotCheckpointBound {
		t.Fatalf("checkpoint context deadline = %v, %v, want one within %s", dl, ok, snapshotCheckpointBound)
	}
	live, stopLive := context.WithCancel(context.Background())
	defer stopLive()
	same, stopSame := checkpointContext(live)
	defer stopSame()
	if same != live {
		t.Fatal("a live capture's checkpoint should keep its own context")
	}
}
