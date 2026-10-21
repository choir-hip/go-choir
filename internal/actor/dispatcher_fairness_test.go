package actor

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: the paper
// logged texture:799c… waiting 19 minutes with every slot full, because the
// pending projection came back in agent-id order and each freed slot went to
// the first due agent in that order. Failure modes the order must close:
//  1. an agent whose update has waited longest is served after agents that
//     became due later (an id that sorts late starves under load);
//  2. a deferred update jumps the queue: it is due again from its
//     not_before, not from when it was first created;
//  3. a scheduled update that is not yet due appears at all.

func appendAt(t *testing.T, l *SQLiteLog, id, to string, created, notBefore time.Time) {
	t.Helper()
	u := Update{UpdateID: id, ToAgentID: to, FromAgentID: "test", Kind: "k", Content: "c", CreatedAt: created, NotBefore: notBefore}
	if _, err := l.Append(context.Background(), u); err != nil {
		t.Fatalf("append %s: %v", id, err)
	}
}

func TestPendingAgentsComeOldestDueFirst(t *testing.T) {
	l := openKernelLog(t)
	now := time.Now().UTC()
	appendAt(t, l, "z1", "texture:z", now.Add(-19*time.Minute), time.Time{})
	appendAt(t, l, "m1", "management:m", now.Add(-2*time.Minute), time.Time{})
	appendAt(t, l, "m2", "management:m", now.Add(-30*time.Second), time.Time{})
	appendAt(t, l, "a1", "engineering:a", now.Add(-1*time.Minute), time.Time{})
	// Created long ago but deferred until 20 seconds ago: due since then.
	appendAt(t, l, "d1", "research:d", now.Add(-time.Hour), now.Add(-20*time.Second))
	// A scheduled deadline that has come due 5 minutes ago.
	appendAt(t, l, "s1", "source:s", now.Add(-time.Hour), now.Add(-5*time.Minute))
	// Not due yet.
	appendAt(t, l, "f1", "agent:future", now.Add(-time.Hour), now.Add(time.Minute))

	got, err := l.PendingAgents(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"texture:z", "source:s", "management:m", "engineering:a", "research:d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending order = %v\nwant oldest due first %v", got, want)
	}
}

// With one slot and a steady stream of newer work for agents whose ids sort
// first, the agent that has waited longest gets the next slot.
func TestDispatcherServesTheLongestWaitingAgentNext(t *testing.T) {
	l := openKernelLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	appendAt(t, l, "z1", "texture:zz", now.Add(-10*time.Minute), time.Time{})
	for i := 0; i < 5; i++ {
		appendAt(t, l, fmt.Sprintf("a%d", i), fmt.Sprintf("agent:a%d", i), now.Add(-time.Duration(5-i)*time.Second), time.Time{})
	}

	var mu sync.Mutex
	var order []string
	done := make(chan struct{})
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, mem []byte) ([]byte, error) {
		mu.Lock()
		order = append(order, agentID)
		n := len(order)
		mu.Unlock()
		if n == 6 {
			close(done)
		}
		return mem, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{MaxConcurrent: 1, PollInterval: 10 * time.Millisecond})
	go d.Run(ctx)
	defer d.Stop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("activations = %v; want 6", order)
	}
	mu.Lock()
	defer mu.Unlock()
	if order[0] != "texture:zz" {
		t.Fatalf("first activation = %s (order %v); want the agent waiting 10 minutes", order[0], order)
	}
}

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: after a
// boot, every source poll came due at once and each held a slot for network
// I/O up to the 90 s poll timeout, so desks waited behind RSS fetches. A lane
// gives agents with an id prefix their own cap. Failure modes:
//  1. a laned activation takes a shared slot, so a desk waits behind it;
//  2. the lane runs more than its cap at once;
//  3. a laned agent waits for a shared slot when its lane has room.
func TestLaneKeepsSourcePollsOutOfDeskSlots(t *testing.T) {
	l := openKernelLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	appendAt(t, l, "s1", "source:rss:a", now.Add(-3*time.Minute), time.Time{})
	appendAt(t, l, "s2", "source:rss:b", now.Add(-2*time.Minute), time.Time{})
	appendAt(t, l, "d1", "texture:desk", now.Add(-1*time.Minute), time.Time{})

	release := make(chan struct{})
	var mu sync.Mutex
	started := map[string]bool{}
	running, maxSources := 0, 0
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, mem []byte) ([]byte, error) {
		mu.Lock()
		started[agentID] = true
		laned := strings.HasPrefix(agentID, "source:")
		if laned {
			running++
			if running > maxSources {
				maxSources = running
			}
		}
		mu.Unlock()
		if laned {
			<-release
			mu.Lock()
			running--
			mu.Unlock()
		}
		return mem, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{MaxConcurrent: 1, PollInterval: 10 * time.Millisecond,
		Lanes: []Lane{{Prefix: "source:", MaxConcurrent: 1}}})
	go d.Run(ctx)
	defer d.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		desk, src := started["texture:desk"], started["source:rss:a"]
		mu.Unlock()
		if desk && src {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("started = %v; the desk must run while a source poll holds the lane", started)
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	if started["source:rss:b"] {
		mu.Unlock()
		t.Fatal("the second source ran while the lane (cap 1) was full")
	}
	mu.Unlock()
	close(release)
	deadline = time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		b := started["source:rss:b"]
		mu.Unlock()
		if b {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the second source never ran after the lane freed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if maxSources > 1 {
		t.Fatalf("lane ran %d sources at once, cap 1", maxSources)
	}
}
