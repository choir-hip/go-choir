package agentcore

// Behavioural tests of what the production constructor hands the capture
// (review of lane snapshot-capture-s3, 2026-11-23). Failures, written before
// the fix:
//
//  W1. the capture's free-space guard runs at the snapshot package's default of
//      one minute, so a write burst can fill the volume inside that window while
//      a stalled upload holds the reader (and its per-64-MiB check) back;
//  W2. the pin deadline the producer holds does not reach the capture: a stalled
//      upload then pins the WAL until the whole-capture budget;
//  W3. the WAL growth remembered from a capture raises the free space the next
//      attempts need for ever: after one burst every later attempt refuses;
//  W4. a process restart, or the store being replaced, leaves a stale growth.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// stalledRuntime is a production-shaped runtime whose chunk host never answers a
// request until the capture gives up on it: an uploader that has stalled.
func stalledRuntime(t *testing.T, rows int) snapshotDeps {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	client, err := computerevent.NewHTTPClient(srv.URL, srv.Client(),
		func(context.Context) (string, error) { return "capability", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
	st := newStreamStore(t, rows)
	rt := &Runtime{
		store:                 st,
		eventAppender:         &computerevent.ComputerEventAppender{},
		privateArtifactCipher: streamCipher(t),
		eventPayloadReader:    client,
		cfg:                   provideriface.Config{ComputerID: streamComputer, StorePath: filepath.Join(t.TempDir(), "store")},
	}
	deps, ok := rt.snapshotDepsForRuntime()
	if !ok {
		t.Fatal("the constructor refused a production-shaped runtime")
	}
	return deps
}

// W1: the guard that sees the free space while the reader is blocked runs at the
// producer's interval, not the default minute. The store is small (the stream's
// own 64 MiB check never runs), the host never answers, so only the guard can end
// the capture before the test's deadline.
func TestTheCapturesFreeSpaceGuardRunsAtTheProducersInterval(t *testing.T) {
	if snapshotFreeCheckEvery != 10*time.Second {
		t.Fatalf("snapshotFreeCheckEvery = %s, want compaction's 10s", snapshotFreeCheckEvery)
	}
	prevEvery, prevFree := snapshotFreeCheckEvery, snapshotFreeBytes
	t.Cleanup(func() { snapshotFreeCheckEvery, snapshotFreeBytes = prevEvery, prevFree })
	snapshotFreeCheckEvery = 50 * time.Millisecond
	snapshotFreeBytes = func(string) (uint64, error) { return 1 << 20, nil } // under the floor

	deps := stalledRuntime(t, 20)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started := time.Now()
	_, err := deps.Take(ctx, 10*time.Minute)
	if err == nil || !strings.Contains(err.Error(), "fell under the") {
		t.Fatalf("err = %v, want the free-space guard's refusal (ctx %v)", err, ctx.Err())
	}
	if took := time.Since(started); took > 15*time.Second {
		t.Fatalf("the guard ended the capture after %s: it did not run at the producer's interval", took)
	}
}

// W2: the producer's pin deadline reaches the capture. A store of more than the
// uploader's buffers (13 chunks) blocks the reader behind the stalled host, and
// only the deadline ends the read transaction.
func TestThePinDeadlineTheProducerHoldsReachesTheCapture(t *testing.T) {
	prev := snapshotPinDeadline
	t.Cleanup(func() { snapshotPinDeadline = prev })
	snapshotPinDeadline = 400 * time.Millisecond

	deps := stalledRuntime(t, 7500) // about 60 MiB: the reader fills every buffer and blocks
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := deps.Take(ctx, 30*time.Minute)
	if !errors.Is(err, snapshot.ErrPinDeadline) {
		t.Fatalf("err = %v (ctx %v), want ErrPinDeadline", err, ctx.Err())
	}
}

func growthRef(growth int64) snapshot.Ref {
	return snapshot.Ref{EventID: "event-snapshot", Record: snapshot.Record{StoreHeadSequence: 50, PlaintextSize: 1},
		Stats: snapshot.Stats{PinWALStart: 100, PinWALEnd: 100 + growth}}
}

// W3: across ticks. A capture whose WAL grew a lot raises the next attempt's
// need to twice that growth; when that refuses, the memory is cleared, so the
// next tick asks only for the planned headroom and the producer is not stuck.
func TestRememberedWALGrowthRaisesTheNeedOnceAndDoesNotRatchet(t *testing.T) {
	h, p, now := dueHarness(t, "interval")
	big := int64(10) << 30
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) { h.takes++; return growthRef(big), nil }
	h.free = snapshotWALHeadroom(0) + snapshotFreeFloor
	if outcome, detail := h.run(p, now); outcome != snapshotTaken {
		t.Fatalf("first tick: %s %q", outcome, detail)
	}
	// A later tick, due again, with only the planned headroom free.
	h.newestAt, h.head = now.Add(-48*time.Hour), 300
	later := now.Add(48 * time.Hour)
	outcome, detail := h.run(p, later)
	if outcome != snapshotRefused || !strings.Contains(detail, "WAL headroom") {
		t.Fatalf("second tick: %s %q, want a refusal asking for twice the last growth", outcome, detail)
	}
	if h.takes != 1 {
		t.Fatalf("takes = %d", h.takes)
	}
	// The same free space on the next tick: the memory was spent by the refusal.
	if outcome, detail := h.run(p, later.Add(time.Minute)); outcome != snapshotTaken || h.takes != 2 {
		t.Fatalf("third tick: %s %q takes=%d: the remembered growth ratcheted", outcome, detail, h.takes)
	}
}

// W4: forget (the store was replaced) and a new process both ask for the planned
// headroom, not for a growth measured on another store or lost with the process.
func TestForgetAndARestartAskForThePlannedHeadroom(t *testing.T) {
	h, p, now := dueHarness(t, "interval")
	big := int64(10) << 30
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) { h.takes++; return growthRef(big), nil }
	h.free = snapshotWALHeadroom(0) + snapshotFreeFloor
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatal("first tick not taken")
	}
	p.forget()
	h.newestAt = now.Add(-48 * time.Hour)
	if outcome, detail := h.run(p, now.Add(48*time.Hour)); outcome != snapshotTaken {
		t.Fatalf("after forget: %s %q", outcome, detail)
	}
	// A fresh producer is a restarted process: nothing is remembered.
	fresh := &snapshotProducer{}
	if outcome, detail := h.run(fresh, now.Add(96*time.Hour)); outcome != snapshotTaken {
		t.Fatalf("fresh producer: %s %q", outcome, detail)
	}
}
