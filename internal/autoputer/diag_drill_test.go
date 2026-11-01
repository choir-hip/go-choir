package autoputer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// The isolation drill load (/internal/diag/drill): a host-peer-only route
// that makes this guest a noisy neighbour on demand, for the guest isolation
// drills (docs/definitions/choir-guest-isolation-2026-11-01.md), run on d@e
// only. Ways it can fail, written before the code:
//
//  D1. an in-guest caller (or anything not the host peer) starts a load;
//  D2. a request names an unbounded duration or size, or an unknown kind,
//      and the load runs anyway;
//  D3. two drills run at once, doubling a load the operator sized;
//  D4. the scratch file outlives the drill (it ends, fails or is cancelled),
//      leaking guest disk;
//  D5. a write drill fills the data disk (free space under twice its size);
//  D6. the reply does not say what the load did (bytes, operations, the
//      latency it saw), so the drill cannot be read from the guest's side.

func drillRequest(method, query string) *http.Request {
	return hostPeerRequest(method, "/internal/diag/drill?"+query)
}

func TestDrillRefusesNonHostPeer(t *testing.T) {
	d := newDiagDrill(t.TempDir())
	rec := httptest.NewRecorder()
	d.ServeHTTP(rec, nonHostPeerRequest(http.MethodPost, "/internal/diag/drill?kind=cpu&seconds=1"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("D1: an in-guest caller got %d, want 403", rec.Code)
	}
	rec = httptest.NewRecorder()
	d.ServeHTTP(rec, drillRequest(http.MethodGet, "kind=cpu&seconds=1"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("a GET started a load: %d, want 405", rec.Code)
	}
}

func TestDrillRefusesUnboundedOrUnknown(t *testing.T) {
	d := newDiagDrill(t.TempDir())
	for _, q := range []string{
		"kind=cpu&seconds=0",
		"kind=cpu&seconds=301",
		"kind=cpu",
		"kind=io-write&seconds=1&mib=0",
		"kind=io-write&seconds=1&mib=2049",
		"kind=io-read&seconds=1",
		"kind=fork-bomb&seconds=1",
		"kind=cpu&seconds=1&threads=0",
		"kind=cpu&seconds=1&threads=65",
	} {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, drillRequest(http.MethodPost, q))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("D2: %q got %d, want 400", q, rec.Code)
		}
	}
}

func TestDrillRunsOneAtATime(t *testing.T) {
	d := newDiagDrill(t.TempDir())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, drillRequest(http.MethodPost, "kind=cpu&seconds=2&threads=1"))
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !d.busy() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rec := httptest.NewRecorder()
	d.ServeHTTP(rec, drillRequest(http.MethodPost, "kind=cpu&seconds=1&threads=1"))
	if rec.Code != http.StatusConflict {
		t.Errorf("D3: a second drill got %d, want 409", rec.Code)
	}
	wg.Wait()
}

func TestDrillIOLeavesNoScratchAndReportsWhatItDid(t *testing.T) {
	dir := t.TempDir()
	d := newDiagDrill(dir)
	for _, kind := range []string{"io-write", "io-read"} {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, drillRequest(http.MethodPost, "kind="+kind+"&seconds=1&mib=4"))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", kind, rec.Code, rec.Body.String())
		}
		var got drillReport
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Kind != kind || got.Bytes <= 0 || got.Ops <= 0 || got.P50Micros <= 0 || got.P99Micros < got.P50Micros || got.Seconds <= 0 {
			t.Errorf("D6: %s reported %+v", kind, got)
		}
		left, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(left) != 0 {
			t.Errorf("D4: %s left %d entries in the scratch directory", kind, len(left))
		}
	}
}

func TestDrillCancelledLeavesNoScratch(t *testing.T) {
	dir := t.TempDir()
	d := newDiagDrill(dir)
	req := drillRequest(http.MethodPost, "kind=io-write&seconds=30&mib=4")
	ctx, cancel := contextWithCancelAfter(req, 200*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	start := time.Now()
	d.ServeHTTP(rec, req.WithContext(ctx))
	if time.Since(start) > 5*time.Second {
		t.Errorf("a cancelled drill ran on for %s", time.Since(start))
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("D4: a cancelled drill left %d entries", len(left))
	}
}

func TestDrillWriteRefusesWithoutHeadroom(t *testing.T) {
	d := newDiagDrill(t.TempDir())
	d.free = func(string) (int64, error) { return 3 << 20, nil } // 3 MiB free
	rec := httptest.NewRecorder()
	d.ServeHTTP(rec, drillRequest(http.MethodPost, "kind=io-write&seconds=1&mib=2"))
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("D5: a 2 MiB drill with 3 MiB free got %d, want 507", rec.Code)
	}
}

func contextWithCancelAfter(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(r.Context())
	time.AfterFunc(d, cancel)
	return ctx, cancel
}
