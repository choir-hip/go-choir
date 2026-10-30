package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// F1 (docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md):
// /health reads the store on its one-connection write pool. A holder of that
// connection must not turn /health into a silent hang, and a stalled store
// must never answer 200.

// healthAnswerWait bounds how long a test waits for /health: the 1 s store
// budget plus slack for -race. Today's handler never answers while the
// connection is held, so the wait is what fails it.
const healthAnswerWait = 3 * time.Second

func newBoundedHealthFixture(t *testing.T) (*APIHandler, *choirstore.Store) {
	t.Helper()
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = productStore.Close() })
	rt := &Runtime{
		cfg:                 provideriface.Config{ComputerID: "computer-health"},
		store:               productStore,
		provider:            healthStubProvider{},
		selfdevRouteOwnerID: "owner-health",
	}
	rt.health = types.HealthReady
	return &APIHandler{rt: rt}, productStore
}

// serveHealth runs /health and waits a bounded time for the answer.
func serveHealth(t *testing.T, handler *APIHandler) (*httptest.ResponseRecorder, time.Duration) {
	t.Helper()
	rec := httptest.NewRecorder()
	answered := make(chan time.Duration, 1)
	start := time.Now()
	go func() {
		handler.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		answered <- time.Since(start)
	}()
	timer := time.NewTimer(healthAnswerWait)
	defer timer.Stop()
	select {
	case took := <-answered:
		return rec, took
	case <-timer.C:
		t.Fatalf("/health did not answer within %s while the store was held", healthAnswerWait)
		return nil, 0
	}
}

// holdStore takes the write pool's only connection, then starts a read on
// the same pool that waits behind it. The
// returned release frees both and waits, bounded, for the read to end.
func holdStore(t *testing.T, productStore *choirstore.Store) (release func()) {
	t.Helper()
	ctx := context.Background()
	conn, err := productStore.DB().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	readEnded := make(chan struct{})
	go func() {
		defer close(readEnded)
		var one int
		_ = productStore.DB().QueryRowContext(ctx, `SELECT 1`).Scan(&one)
	}()
	waitFor(t, "a waiter on the write connection", func() bool {
		return productStore.PoolStats().Write.InUse > 0 && productStore.DB().Stats().WaitCount > 0
	})
	var released bool
	release = func() {
		if released {
			return
		}
		released = true
		_ = conn.Close()
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-readEnded:
		case <-timer.C:
			t.Errorf("the held object-graph read did not end after the connection was released")
		}
	}
	t.Cleanup(release)
	return release
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// healthReadGoroutines counts the goroutines /health's bounded store reads
// started: those whose stack names readHealthStore. Unrelated goroutines
// (Dolt's, the runtime's, other tests') do not count.
func healthReadGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	count := 0
	for _, stack := range strings.Split(string(buf), "\n\n") {
		if strings.Contains(stack, "readHealthStore") {
			count++
		}
	}
	return count
}

// waitHealthReadsAtMost polls, bounded, until at most limit health-read
// goroutines remain, and reports the last count.
func waitHealthReadsAtMost(limit int, within time.Duration) int {
	deadline := time.Now().Add(within)
	for {
		n := healthReadGoroutines()
		if n <= limit || time.Now().After(deadline) {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHandleHealthAnswers503NamingHolderWhenStoreStalled(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	holdStore(t, productStore)

	rec, took := serveHealth(t, handler)
	if took > 2*time.Second {
		t.Fatalf("/health took %s on a stalled store; the proxy's probe gives up at 2s", took)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("stalled store /health status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status        string                     `json:"status"`
		Service       string                     `json:"service"`
		ComputerID    string                     `json:"computer_id"`
		RuntimeHealth string                     `json:"runtime_health"`
		Build         map[string]any             `json:"build"`
		StoreStalled  *healthStoreStalledForTest `json:"store_stalled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode 503 body: %v: %s", err, rec.Body.String())
	}
	if body.Status != "store_stalled" || body.Service != "autoputer" || body.ComputerID != "computer-health" {
		t.Fatalf("503 body status=%q service=%q computer=%q", body.Status, body.Service, body.ComputerID)
	}
	if body.RuntimeHealth != string(types.HealthReady) || body.Build == nil {
		t.Fatalf("503 body runtime_health=%q build=%v", body.RuntimeHealth, body.Build)
	}
	if body.StoreStalled == nil || len(body.StoreStalled.Unanswered) == 0 || body.StoreStalled.BudgetMS <= 0 {
		t.Fatalf("503 body store_stalled=%+v", body.StoreStalled)
	}
	// Counts that were not read are absent, never a 0 that reads as idle.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	for _, unknown := range []string{"desk_pending_mutations", "selfdev_active_operations", "live_capsules", "effective_release"} {
		if _, ok := keys[unknown]; ok {
			t.Fatalf("503 body carries %q, which was not read: %s", unknown, rec.Body.String())
		}
	}
}

type healthStoreStalledForTest struct {
	Unanswered []string `json:"unanswered"`
	BudgetMS   int64    `json:"budget_ms"`
}

// The 200 body is today's: the same keys, in runtimeHealthResponse's order.
func TestHandleHealthUnheldStoreAnswers200WithUnchangedBody(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	// A head row, so effective_release.event_head comes from a store read.
	if _, err := productStore.DB().Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, 7, 'canon', 'desired', 'effective', 'dsc', 'esc', 1, 0, ?)`, "computer-health", time.Now().UTC()); err != nil {
		t.Fatalf("seed head: %v", err)
	}

	rec, _ := serveHealth(t, handler)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(keys))
	for key := range keys {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"active_provider", "build", "computer_id", "effective_release", "engine_mutex", "live_capsules", "persistent_disk", "running_runs", "runtime_health", "service", "status"}
	// persistent_disk and engine_mutex depend on the host and the store; keep
	// the comparison to keys that today's handler always or never writes.
	optional := map[string]bool{"persistent_disk": true, "engine_mutex": true, "dolt_gc": true}
	filter := func(in []string) []string {
		out := []string{}
		for _, key := range in {
			if !optional[key] {
				out = append(out, key)
			}
		}
		return out
	}
	if strings.Join(filter(got), ",") != strings.Join(filter(want), ",") {
		t.Fatalf("200 body keys=%v want %v (optional %v)", got, want, optional)
	}
	if _, ok := keys["store_stalled"]; ok {
		t.Fatalf("200 body carries store_stalled: %s", rec.Body.String())
	}
	var decoded runtimeHealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != string(types.HealthReady) || decoded.EffectiveRelease == nil || decoded.EffectiveRelease.EventHead == nil || decoded.EffectiveRelease.EventHead.Sequence != 7 {
		t.Fatalf("200 body status=%q effective_release=%+v", decoded.Status, decoded.EffectiveRelease)
	}
	// Byte-compatible: the body is exactly runtimeHealthResponse encoded.
	var reencoded bytes.Buffer
	if err := json.NewEncoder(&reencoded).Encode(decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded.Bytes(), rec.Body.Bytes()) {
		t.Fatalf("200 body is not runtimeHealthResponse's encoding\n got: %s\nwant: %s", rec.Body.String(), reencoded.String())
	}
}

// Polls during a stall must not pile up goroutines, and none may outlive the
// stall once the holder lets go.
func TestHandleHealthStalledStoreLeaksNoGoroutines(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	if rec, _ := serveHealth(t, handler); rec.Code != http.StatusOK {
		t.Fatalf("warm-up /health status=%d body=%s", rec.Code, rec.Body.String())
	}

	release := holdStore(t, productStore)
	for i := 0; i < 5; i++ {
		rec, _ := serveHealth(t, handler)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("poll %d during the stall: status=%d want 503", i, rec.Code)
		}
	}
	// Five polls of three reads each: every read was waiting for the held
	// connection, so cancelling its context at the deadline ends it while
	// the store is still held.
	if n := waitHealthReadsAtMost(0, 3*time.Second); n != 0 {
		t.Fatalf("%d /health read goroutines remain during the stall: abandoned reads are piling up", n)
	}

	release()
	if n := waitHealthReadsAtMost(0, 5*time.Second); n != 0 {
		t.Fatalf("%d /health read goroutines outlived the stall", n)
	}
	if rec, _ := serveHealth(t, handler); rec.Code != http.StatusOK {
		t.Fatalf("/health after release status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
}
