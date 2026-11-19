package actor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// A dispatcher given no cap runs twelve activations at once on every
// computer (owner, 2026-11-19: "I want everything on"; the paper's owner
// raised it from eight on 2026-10-27, and the setting that did it is gone).
// Failure modes: it admits eight and leaves due work waiting, or it admits
// more than twelve.
func TestDispatcherDefaultsToTwelveSharedSlots(t *testing.T) {
	l := openKernelLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	for i := 0; i < 15; i++ {
		appendAt(t, l, fmt.Sprintf("u%d", i), fmt.Sprintf("agent:a%d", i), now.Add(-time.Duration(20-i)*time.Second), time.Time{})
	}
	release := make(chan struct{})
	var mu sync.Mutex
	running, peak := 0, 0
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
		mu.Unlock()
		return mem, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 10 * time.Millisecond})
	go d.Run(ctx)
	defer d.Stop()
	defer close(release)

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		r := running
		mu.Unlock()
		if r >= 12 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("running = %d with 15 agents due, want 12", r)
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if peak != 12 {
		t.Fatalf("peak = %d, want 12 (the default shared cap)", peak)
	}
}
