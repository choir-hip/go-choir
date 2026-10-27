package actor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The paper's owner raised its shared slots from 8 to 12 (2026-10-27, "I
// have the tokens for it"): a computer setting changes the cap while the
// dispatcher runs. Failure modes the live change must close:
//  1. a raise does not admit waiting work until a restart;
//  2. a lower cancels or interrupts activations already running;
//  3. after a lower, new work is admitted above the new cap;
//  4. a cap of 0 or less leaves the dispatcher with no slots.
func TestSetSharedSlotsChangesTheCapLive(t *testing.T) {
	l := openKernelLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		appendAt(t, l, fmt.Sprintf("u%d", i), fmt.Sprintf("agent:a%d", i), now.Add(-time.Duration(10-i)*time.Second), time.Time{})
	}

	release := make(chan struct{})
	var mu sync.Mutex
	running, peak, finished := 0, 0, 0
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, mem []byte) ([]byte, error) {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		mu.Unlock()
		<-release
		mu.Lock()
		running--
		finished++
		mu.Unlock()
		return mem, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{MaxConcurrent: 2, PollInterval: 10 * time.Millisecond})
	go d.Run(ctx)
	defer d.Stop()

	waitRunning := func(want int, why string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			mu.Lock()
			r := running
			mu.Unlock()
			if r == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: running = %d, want %d", why, r, want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitRunning(2, "at the starting cap of 2")
	if got := d.SharedSlots(); got != 2 {
		t.Fatalf("SharedSlots = %d, want 2", got)
	}

	d.SetSharedSlots(4)
	waitRunning(4, "after a live raise to 4")

	d.SetSharedSlots(1)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if running != 4 || finished != 0 {
		mu.Unlock()
		t.Fatalf("a lower interrupted work: running = %d finished = %d, want 4 and 0", running, finished)
	}
	mu.Unlock()

	// Free the four; with the cap at 1 the last two run one at a time.
	for i := 0; i < 4; i++ {
		release <- struct{}{}
	}
	mu.Lock()
	peak = 0
	mu.Unlock()
	for i := 0; i < 2; i++ {
		waitRunning(1, "after the lower to 1")
		release <- struct{}{}
	}
	mu.Lock()
	if peak > 1 {
		mu.Unlock()
		t.Fatalf("after the lower to 1, %d ran at once", peak)
	}
	mu.Unlock()

	d.SetSharedSlots(0)
	if got := d.SharedSlots(); got != defaultSharedSlots {
		t.Fatalf("SetSharedSlots(0) left %d slots, want the default %d", got, defaultSharedSlots)
	}
	close(release)
}
