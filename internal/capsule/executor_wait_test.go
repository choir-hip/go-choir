package capsule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitCapsuleProcessTimesOutWhenWaitNeverReturns(t *testing.T) {
	started := time.Now()
	err := waitCapsuleProcess(context.Background(), func() error {
		time.Sleep(time.Second)
		return nil
	}, func() error { return nil }, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "capsule process wait timed out") {
		t.Fatalf("timeout error=%v", err)
	}
	if time.Since(started) > 400*time.Millisecond {
		t.Fatalf("waitCapsuleProcess blocked too long: %s", time.Since(started))
	}
}

func TestWaitCapsuleProcessTimesOutAfterCancelWithoutHangingOnWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := waitCapsuleProcess(ctx, func() error {
		time.Sleep(time.Second)
		return nil
	}, func() error { return nil }, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "capsule process wait timed out after cancel") {
		t.Fatalf("cancel timeout error=%v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if time.Since(started) > 400*time.Millisecond {
		t.Fatalf("waitCapsuleProcess blocked too long after cancel: %s", time.Since(started))
	}
}

func TestWaitCapsuleProcessReturnsWaitError(t *testing.T) {
	want := errors.New("exited")
	err := waitCapsuleProcess(context.Background(), func() error { return want }, nil, time.Second)
	if !errors.Is(err, want) {
		t.Fatalf("wait error=%v", err)
	}
}
