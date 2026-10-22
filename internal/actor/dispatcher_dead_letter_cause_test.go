package actor

import (
	"bytes"
	"context"
	"errors"
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
