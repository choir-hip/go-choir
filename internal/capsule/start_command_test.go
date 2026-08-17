package capsule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitForCommandStartTimesOut(t *testing.T) {
	err := waitForCommandStart(context.Background(), 40*time.Millisecond, func() error {
		time.Sleep(300 * time.Millisecond)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err=%v", err)
	}
}

func TestWaitForCommandStartRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	err := waitForCommandStart(ctx, time.Second, func() error {
		time.Sleep(time.Second)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestWaitForCommandStartReturnsStartError(t *testing.T) {
	want := errors.New("boom")
	err := waitForCommandStart(context.Background(), time.Second, func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
