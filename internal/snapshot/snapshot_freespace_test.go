package snapshot

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errTestLowDisk = errors.New("test: free space under the floor")

// Failure mode (seat 1 review, 2026-11-12): the read transaction pins the WAL
// for up to 90 minutes while free space is checked only before the attempt, so
// a snapshot could push the guest to ENOSPC. A check during the capture must
// abort it with the named cause, and leave no root and no event.
func TestTheCaptureAbortsWhenFreeSpaceFallsUnderTheFloor(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	f.cfg.FreeSpaceEvery = 10 * time.Millisecond
	calls := 0
	f.cfg.FreeSpace = func() error {
		calls++
		if calls >= 3 {
			return errTestLowDisk
		}
		return nil
	}
	// A slow stream: only the free-space check can end it.
	f.cfg.pageHook = func(int64) { time.Sleep(time.Millisecond) }
	started := time.Now()
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, errTestLowDisk) {
		t.Fatalf("err = %v, want the free-space cause", err)
	}
	if time.Since(started) > 10*time.Second {
		t.Fatalf("the capture ran %s past a failing check", time.Since(started))
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}
