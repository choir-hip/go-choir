package autoputer

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
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

// Network kinds (guest isolation I5). Ways they can fail, written first:
//
//  N1. a target outside 192.88.99.0/24 (a third party, a hostname, loopback,
//      IPv6, port 0) is dialled;
//  N2. seconds, count or streams exceed their bounds;
//  N3. net-conns does not hold the connections it opened, or does not report
//      how many opened and how many were refused;
//  N4. net-flood reports no bytes, or runs past its seconds.

func TestDrillNetRefusesOutsideTargetsAndBounds(t *testing.T) {
	d := newDiagDrill(t.TempDir())
	for _, q := range []string{
		"kind=net-conns&seconds=5&count=10",
		"kind=net-conns&seconds=5&count=10&target=8.8.8.8:53",
		"kind=net-conns&seconds=5&count=10&target=127.0.0.1:80",
		"kind=net-conns&seconds=5&count=10&target=192.88.100.1:80",
		"kind=net-conns&seconds=5&count=10&target=10.98.0.1:80",
		"kind=net-conns&seconds=5&count=10&target=10.99.0.2:80",
		"kind=net-conns&seconds=5&count=10&target=192.88.98.2:80",
		"kind=net-conns&seconds=5&count=10&target=example.com:80",
		"kind=net-conns&seconds=5&count=10&target=[::1]:80",
		"kind=net-conns&seconds=5&count=10&target=192.88.99.2:0",
		"kind=net-flood&seconds=5&target=169.254.169.254:80",
		"kind=net-conns&seconds=61&count=10&target=192.88.99.2:80",
		"kind=net-flood&seconds=61&target=192.88.99.2:80",
		"kind=net-conns&seconds=5&count=0&target=192.88.99.2:80",
		"kind=net-conns&seconds=5&count=12001&target=192.88.99.2:80",
		"kind=net-flood&seconds=5&streams=17&target=192.88.99.2:80",
		"kind=net-flood&seconds=5&streams=0&target=192.88.99.2:80",
	} {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, drillRequest(http.MethodPost, q))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("N1/N2: %q got %d, want 400", q, rec.Code)
		}
	}
	if d.busy() {
		t.Fatal("a refused drill left the drill marked running")
	}
}

func TestDrillNetParseTarget(t *testing.T) {
	for _, ok := range []string{"192.88.99.2:9000", "192.88.99.255:1"} {
		if got, err := parseDrillTarget(ok); err != nil || got != ok {
			t.Fatalf("allowed target %q refused: %q %v", ok, got, err)
		}
	}
}

// The allowlist block is not routable on a developer host, so the run tests
// call the load functions on a loopback listener, past the target check that
// the test above pins.
func TestDrillNetConnsHoldsAndReports(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	var live, peak atomic.Int64
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n := live.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			go func() { _, _ = io.Copy(io.Discard, c); live.Add(-1); c.Close() }()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rep := drillNetConns(ctx, ln.Addr().String(), 300)
	if rep.Opened != 300 || rep.Failed != 0 {
		t.Fatalf("N3: report %+v", rep)
	}
	if peak.Load() != 300 {
		t.Errorf("N3: listener saw %d held at once, want 300", peak.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for live.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if live.Load() != 0 {
		t.Errorf("N3: %d connections left open after the drill", live.Load())
	}
}

func TestDrillNetFloodMovesBytesAndStops(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, c); c.Close() }()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	rep := drillNetFlood(ctx, ln.Addr().String(), 2)
	if rep.Bytes <= 0 || rep.Opened != 2 {
		t.Errorf("N4: report %+v", rep)
	}
	if time.Since(start) > 4*time.Second {
		t.Errorf("N4: flood ran %s past a 1 s window", time.Since(start))
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
