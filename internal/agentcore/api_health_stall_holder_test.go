package agentcore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// docs/problems/paper-post-refresh-health-windows-2026-10-22.md, "Findings:
// an instrument that names the holder at the stall": a store_stalled 503
// names, from inside the guest and at the moment the reads gave up, the
// goroutine that holds the store's one connection inside the engine, and the
// pools' wait stats; at most once a minute; never delaying the 503.
//
// Ways it could fail: no line at all (the branch logs nothing); the holder
// missing because the snapshot is taken too late or filtered wrongly; the
// health reads themselves (parked in database/sql.(*DB).conn) listed as
// holders; a line per poll during a long stall (log flood); the 503 delayed
// by the capture; an unbounded line; the pool stats missing or zero while
// callers waited.

// lockedBuffer is a log sink the capture goroutine and the test share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) lines(substr string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, line := range strings.Split(b.buf.String(), "\n") {
		if strings.Contains(line, substr) {
			out = append(out, line)
		}
	}
	return out
}

// holdWriteConnInEngine holds the write pool's one connection inside the
// SQLite driver (a long recursive query, interrupted when ctx ends): a raw
// s.db caller, the shape of the unattributed bursts.
func holdWriteConnInEngine(ctx context.Context, db *sql.DB) error {
	var n int64
	return db.QueryRowContext(ctx, "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x < 2000000000) SELECT count(*) FROM c").Scan(&n)
}

func TestHealthStoreStallLogsTheEngineHolderOncePerMinute(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	handler.rt.cfg.ComputerID = "computer-stall-holder"
	sink := &lockedBuffer{}
	previous := log.Writer()
	log.SetOutput(sink)
	t.Cleanup(func() { log.SetOutput(previous) })

	ctx, cancel := context.WithCancel(context.Background())
	held := make(chan struct{})
	go func() {
		defer close(held)
		_ = holdWriteConnInEngine(ctx, productStore.DB())
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-held:
		case <-time.After(10 * time.Second):
			t.Errorf("the query holder did not end after its context was cancelled")
		}
	})
	waitFor(t, "the write connection in use", func() bool { return productStore.DB().Stats().InUse == 1 })

	rec, took := serveHealth(t, handler)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("held store /health status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	if took > 2*time.Second {
		t.Fatalf("/health took %s on a stalled store: the capture delayed the answer", took)
	}
	header := "store_stalled holders: computer=computer-stall-holder"
	waitFor(t, "the stall header line", func() bool { return len(sink.lines(header)) == 1 })
	holders := sink.lines("store_stalled holder g=")
	if len(holders) == 0 {
		t.Fatalf("no holder line; log:\n%s", strings.Join(sink.lines(""), "\n"))
	}
	all := strings.Join(holders, "\n")
	if !strings.Contains(all, "choir=internal/agentcore.holdWriteConnInEngine") {
		t.Fatalf("no holder line names the choir caller holdWriteConnInEngine:\n%s", all)
	}
	if !strings.Contains(all, "engine=") {
		t.Fatalf("holder lines carry no engine frame:\n%s", all)
	}
	for _, line := range holders {
		if strings.Contains(line, "readHealthStore") || strings.Contains(line, "pendingDeskMutations") {
			t.Fatalf("a health read parked in (*DB).conn is listed as a holder: %s", line)
		}
		if len(line) > 600 {
			t.Fatalf("holder line is %d bytes, over its bound: %.200s", len(line), line)
		}
	}
	head := sink.lines(header)[0]
	for _, want := range []string{"write_pool{", "read_pool{", "in_use=1", "wait_count=", "wait_ms="} {
		if !strings.Contains(head, want) {
			t.Fatalf("header %q lacks %q", head, want)
		}
	}
	if strings.Contains(strings.SplitN(head, "read_pool{", 2)[0], "wait_count=0 ") {
		t.Fatalf("write pool wait_count is 0 although three health reads waited: %s", head)
	}

	// A second stall within the minute answers 503 and logs nothing more.
	rec, _ = serveHealth(t, handler)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("second /health status=%d want 503", rec.Code)
	}
	time.Sleep(300 * time.Millisecond)
	if n := len(sink.lines(header)); n != 1 {
		t.Fatalf("%d stall captures within a minute, want 1", n)
	}
}

// The pools' wait stats are on the observability route, so their growth
// shows without a stall; /health's 200 body stays as it is.
func TestObservabilityReportsStorePoolStats(t *testing.T) {
	rt := telemetryRuntime(t, "computer-pool-stats")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	raw, ok := obs["store_pools"]
	if !ok {
		t.Fatalf("observability has no store_pools: %s", rec.Body.String())
	}
	var pools map[string]map[string]any
	if err := json.Unmarshal(raw, &pools); err != nil {
		t.Fatal(err)
	}
	for _, pool := range []string{"write", "read"} {
		for _, key := range []string{"max_open", "open", "in_use", "idle", "wait_count", "wait_duration_ms"} {
			if _, ok := pools[pool][key]; !ok {
				t.Fatalf("store_pools.%s lacks %q: %s", pool, key, raw)
			}
		}
		// One writer connection; the WAL read pool is wider.
		want := "1"
		if pool == "read" {
			want = "4"
		}
		if got := fmt.Sprint(pools[pool]["max_open"]); got != want {
			t.Fatalf("store_pools.%s.max_open=%s want %s", pool, got, want)
		}
	}
}

// The filter and the bounds, on a synthetic dump: an engine goroutine with a
// choir caller is listed with its choir frame and its first two engine
// frames; one parked in (*DB).conn is not; one with no engine frame is not;
// the engine's own background goroutine (no choir frame or creator) is
// counted but not listed; at most storeStallHolderMax lines.
func TestStoreStallHolderLinesFilterAndBound(t *testing.T) {
	engine := `goroutine 7 [select]:
modernc.org/sqlite/lib.Xsqlite3_step(0xc000, {0x1, 0x2})
	/m/sqlite/lib/sqlite_linux_amd64.go:80 +0x1
modernc.org/sqlite.(*stmt).exec(0xc000)
	/m/sqlite/sqlite.go:10 +0x1
modernc.org/sqlite.(*conn).ExecContext(0xc000)
	/m/sqlite/sqlite.go:20 +0x1
database/sql.(*DB).execDC(0xc000)
	/go/src/database/sql/sql.go:1700 +0x1
github.com/yusefmosiah/go-choir/internal/store.(*Store).finalizeBatch(0xc000)
	/r/internal/store/store.go:10 +0x1
created by github.com/yusefmosiah/go-choir/internal/agentcore.(*Runtime).Start in goroutine 1
	/r/internal/agentcore/runtime.go:10 +0x1`
	parked := `goroutine 8 [select]:
database/sql.(*DB).conn(0xc000)
	/go/src/database/sql/sql.go:1300 +0x1
modernc.org/sqlite.(*conn).QueryContext(0xc000)
	/m/sqlite/sqlite.go:10 +0x1
github.com/yusefmosiah/go-choir/internal/store.(*Store).CountPendingAgentMutations(0xc000)
	/r/internal/store/store.go:10 +0x1`
	background := `goroutine 9 [chan receive, 3 minutes]:
modernc.org/sqlite.(*conn).watchInterrupt(0xc000)
	/m/sqlite/sqlite.go:10 +0x1
created by modernc.org/sqlite.newConn in goroutine 1
	/m/sqlite/sqlite.go:5 +0x1`
	plain := `goroutine 10 [running]:
github.com/yusefmosiah/go-choir/internal/agentcore.(*Runtime).loop(0xc000)
	/r/internal/agentcore/runtime.go:10 +0x1`
	lines, matched := storeStallHolderLines(strings.Join([]string{background, engine, parked, plain}, "\n\n"))
	if matched != 2 || len(lines) != 1 {
		t.Fatalf("matched=%d lines=%q, want two engine goroutines matched and the choir one shown", matched, lines)
	}
	want := "store_stalled holder g=7 [select] choir=internal/store.(*Store).finalizeBatch engine=lib.Xsqlite3_step < (*stmt).exec"
	if lines[0] != want {
		t.Fatalf("first line\n got %q\nwant %q", lines[0], want)
	}

	var many []string
	for i := 0; i < 3*storeStallHolderMax; i++ {
		many = append(many, strings.Replace(engine, "goroutine 7 ", fmt.Sprintf("goroutine %d ", 100+i), 1))
	}
	lines, matched = storeStallHolderLines(strings.Join(many, "\n\n"))
	if matched != 3*storeStallHolderMax || len(lines) != storeStallHolderMax {
		t.Fatalf("matched=%d lines=%d, want %d matched and %d shown", matched, len(lines), 3*storeStallHolderMax, storeStallHolderMax)
	}
	long := strings.Replace(engine, "finalizeBatch", strings.Repeat("x", 2000), 1)
	lines, _ = storeStallHolderLines(long)
	if len(lines) != 1 || len(lines[0]) > storeStallHolderLineMax {
		t.Fatalf("long line not bounded: %d bytes", len(lines[0]))
	}
}
