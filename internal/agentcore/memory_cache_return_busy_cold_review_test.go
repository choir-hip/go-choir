package agentcore

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Review of S5 (Devin #2, #3; DeepSeek #2):
//   - a snapshot streaming the store is a reason not to reclaim (the
//     planned-restart read reports it; busy_cold dropped it);
//   - the guest takes one reading about 60 s after a busy_cold act that
//     wrote and logs it as refault_rate_post, so the proof reads the price
//     the act caused, not only the rate at its end; a dry run or a refused
//     act schedules none;
//   - a refused busy_cold act is logged with its reason, as the act is.

func TestCacheReturnBusyColdRefusesASnapshotInFlight(t *testing.T) {
	f := busyColdFixture(t)
	f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) {
		return plannedRestartWork{Running: 1, SnapshotInFlight: true}, nil
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusConflict || body["reason"] != "snapshot_in_flight" || f.writeCount() != 0 {
		t.Fatalf("%d %v with %d writes, want 409 busy snapshot_in_flight and none", code, body, f.writeCount())
	}
}

func TestCacheReturnBusyColdLogsAPostActReading(t *testing.T) {
	f := busyColdFixture(t)
	lines := make(chan string, 8)
	f.rt.cacheReturnPostDelay = 20 * time.Millisecond
	f.rt.cacheReturnLog = func(line string) { lines <- line }
	code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "returned" {
		t.Fatalf("%d %v, want 200 returned", code, body)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case line := <-lines:
			if strings.Contains(line, "refault_rate_post=") {
				if !strings.Contains(line, "mode=busy_cold") || !strings.Contains(line, "refaults_post=") {
					t.Fatalf("post line %q lacks its fields", line)
				}
				return
			}
		case <-deadline:
			t.Fatal("no refault_rate_post line within 5 s")
		}
	}
}

func TestCacheReturnBusyColdDryRunAndRefusalScheduleNoPostReading(t *testing.T) {
	f := busyColdFixture(t)
	lines := make(chan string, 8)
	f.rt.cacheReturnPostDelay = 10 * time.Millisecond
	f.rt.cacheReturnLog = func(line string) { lines <- line }
	postCacheReturn(t, f.rt, hostPeer, `{"mode":"busy_cold","scope":"daemon","dry_run":true}`)
	f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) {
		return plannedRestartWork{LiveCapsules: 1}, nil
	}
	postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	sawRefusal := false
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case line := <-lines:
			if strings.Contains(line, "refault_rate_post=") {
				t.Fatalf("a dry run or refusal scheduled a post reading: %q", line)
			}
			if strings.Contains(line, "outcome=busy") && strings.Contains(line, "reason=live_capsules") {
				sawRefusal = true
			}
		case <-timeout:
			if !sawRefusal {
				t.Fatal("the refusal was not logged with its reason")
			}
			return
		}
	}
}
