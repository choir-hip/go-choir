package agentcore

import (
	"context"
	"testing"
	"time"
)

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
