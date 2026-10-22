package actor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// FP4 round 4, P4
// (docs/problems/fp4-r4-report-conflict-guard-scope-dead-letter-2026-10-22.md):
// a wake dead-lettered on its last attempt logged "poison … after N
// attempts" without the handler's error, so a dead letter had no cause.
func TestPoisonOnTheLastAttemptLogsTheHandlerError(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	previous := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(previous) })

	klog := testLog(t)
	cause := errors.New("fate deadline handler: the cause that must be logged")
	sinkSeen := make(chan struct{}, 1)
	handler := HandlerFunc(func(_ context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		if agentID == "sink" {
			select {
			case sinkSeen <- struct{}{}:
			default:
			}
			return memory, nil
		}
		return nil, cause
	})
	rt := NewKernelRuntime(klog, klog, handler, Options{}, DispatcherOptions{PollInterval: 10 * time.Millisecond, MaxAttempts: 1, ErrorSink: "sink"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.StartKernel(ctx)
	defer rt.Stop()
	if err := rt.Send(ctx, Update{UpdateID: "wake-1", ToAgentID: "management", Kind: "assigned_engineering_fate_deadline"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sinkSeen:
	case <-time.After(10 * time.Second):
		t.Fatal("the failing wake never reached the dead-letter sink")
	}
	mu.Lock()
	out := buf.String()
	mu.Unlock()
	if !strings.Contains(out, "poison management/wake-1") || !strings.Contains(out, cause.Error()) {
		t.Fatalf("the dead letter's cause is not logged:\n%s", out)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// Review round 4b (Sonnet F5, both on the wording): the other two poison
// lines. A wake whose attempt budget was spent before the handler ran (its
// deliveries did not complete) and a wake that deferred past MaxDeferrals.
func TestPoisonBeforeTheHandlerSaysTheDeliveriesDidNotComplete(t *testing.T) {
	out := captureDispatcherLog(t)
	klog := testLog(t)
	ctx := context.Background()
	if _, err := klog.Append(ctx, Update{UpdateID: "wake-2", ToAgentID: "management", Kind: "k", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // two deliveries that never completed
		if _, err := klog.RecordAttempt(ctx, "management", "wake-2"); err != nil {
			t.Fatal(err)
		}
	}
	ran := make(chan struct{}, 1)
	sink := make(chan struct{}, 1)
	handler := HandlerFunc(func(_ context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		if agentID == "sink" {
			select {
			case sink <- struct{}{}:
			default:
			}
		} else {
			select {
			case ran <- struct{}{}:
			default:
			}
		}
		return memory, nil
	})
	runDispatcherUntil(t, klog, handler, DispatcherOptions{PollInterval: 10 * time.Millisecond, MaxAttempts: 1, ErrorSink: "sink"}, sink)
	select {
	case <-ran:
		t.Fatal("the handler ran for a wake whose budget was spent")
	default:
	}
	if line := out(); !strings.Contains(line, "poison management/wake-2") || !strings.Contains(line, "did not complete") || !strings.Contains(line, "failed commit") {
		t.Fatalf("the pre-handler poison line does not say the deliveries did not complete:\n%s", line)
	}
}

func TestPoisonAtTheDeferralLimitLogsTheCause(t *testing.T) {
	out := captureDispatcherLog(t)
	klog := testLog(t)
	ctx := context.Background()
	if _, err := klog.Append(ctx, Update{UpdateID: "wake-3", ToAgentID: "management", Kind: "k", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	sink := make(chan struct{}, 1)
	handler := HandlerFunc(func(_ context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		if agentID == "sink" {
			select {
			case sink <- struct{}{}:
			default:
			}
			return memory, nil
		}
		return nil, fmt.Errorf("%w: waiting on the apply checkpoint", ErrDeferUnprocessed)
	})
	runDispatcherUntil(t, klog, handler, DispatcherOptions{PollInterval: 10 * time.Millisecond, MaxDeferrals: 1, ErrorSink: "sink"}, sink)
	// Judge the poison line itself: the earlier "deferred … cause=" line
	// carries the cause too (review round 4c, Sonnet E3 M7a).
	poison := ""
	for _, line := range strings.Split(out(), "\n") {
		if strings.Contains(line, "poison management/wake-3") {
			poison = line
		}
	}
	if !strings.Contains(poison, "waiting on the apply checkpoint") {
		t.Fatalf("the deferral-limit poison line has no cause: %q\n%s", poison, out())
	}
}

func captureDispatcherLog(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	previous := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(previous) })
	return func() string { mu.Lock(); defer mu.Unlock(); return buf.String() }
}

func runDispatcherUntil(t *testing.T, klog *SQLiteLog, handler Handler, opts DispatcherOptions, done <-chan struct{}) {
	t.Helper()
	rt := NewKernelRuntime(klog, klog, handler, Options{}, opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.StartKernel(ctx)
	defer rt.Stop()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the wake never reached the dead-letter sink")
	}
}
