package agentcore

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Review round 2, finding 3: the caller (the host) gave up while the guest was
// still working on the mark. Failure list, written first:
//
//  L1. the marker is written for a caller that is gone, so the host's kill
//      lands on a guest whose crash boot reads as planned;
//  L2. the refusal leaves the refresh hold, the producer's restart block or the
//      capsules this request paused in place.

func TestAMarkIsNotWrittenForACallerThatHasGone(t *testing.T) {
	f := newPauseFixture(t)
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatalf("phase 1: %d %s", code, body)
	}
	f.rt.waitCapsulePauses()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The host gives up while the guest drains: after the acks, before the write.
	f.rt.beforeMarkerWriteHook = cancel
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/planned-restart?allow_busy=1", bytes.NewBufferString(`{"target":"761d94c5"}`)).WithContext(ctx)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	f.handler.HandleInternalPlannedRestart(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("answered 200 to a caller that had gone: %s", rec.Body.String())
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(f.rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("a marker was written for a caller that had gone: %v", err)
	}
	if f.rt.refreshHoldsNewWork(time.Now()) {
		t.Fatal("the refresh hold outlived a mark that wrote nothing")
	}
	f.rt.snapshots.mu.Lock()
	blocked := time.Now().Before(f.rt.snapshots.blockUntil)
	f.rt.snapshots.mu.Unlock()
	if blocked {
		t.Fatal("the producer's restart block outlived a mark that wrote nothing")
	}
	pauseEventually(t, "the capsule to thaw", func() bool { return !f.fake.IsPaused(f.capsuleID) })
}
