package capsule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// docs/problems/capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md H9:
// a refused-egress retry loop (52,434 records) wrote a log line to the serial
// console and an open/append/close to the ledger per record, and starved the
// guest. Ways the fix could fail, pinned below:
//   - ledger records are lost (the ledger is evidence: one line per record,
//     including after Close and under concurrent proxy goroutines);
//   - the ledger file is left open after the capsule ends (Close must flush
//     and close it) or is only flushed at Close (a crash would lose it);
//   - the suppressed count is wrong, or the final count is not logged at Close;
//   - a host never gets its first line (a second host must log at once, and
//     a new reason on a known host too);
//   - the process log is still one line per connection;
//   - a data race between proxy goroutines calling Record (run with -race).

type logCapture struct {
	mu    sync.Mutex
	lines []string
}

func (c *logCapture) logf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func (c *logCapture) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func refusedRecord(host string) EgressRecord {
	return EgressRecord{Time: time.Unix(0, 0).UTC(), CapsuleID: "cap1", Method: "CONNECT", Host: host, Port: 443,
		Outcome: EgressOutcomeRefused, Reason: "host is not on the npm ecosystem allowlist"}
}

func ledgerLines(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(b, []byte("\n"))
}

func TestEgressSinkRefusalBurstKeepsLedgerAndLogsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := newEgressSink(path, lc.logf, clk.now, 10*time.Second)
	for i := 0; i < 50000; i++ {
		s.Record(refusedRecord("registry.npmjs.org"))
	}
	lines := lc.snapshot()
	if len(lines) > 5 {
		t.Fatalf("burst logged %d lines, want <= 5", len(lines))
	}
	if len(lines) == 0 || !strings.Contains(lines[0], "registry.npmjs.org") || !strings.Contains(lines[0], "not on the npm ecosystem allowlist") {
		t.Fatalf("first refusal not logged with host and reason: %v", lines)
	}
	s.Record(refusedRecord("evil.example.com"))
	after := lc.snapshot()
	if len(after) != len(lines)+1 || !strings.Contains(after[len(after)-1], "evil.example.com") {
		t.Fatalf("second host did not get its own first line: %v", after)
	}
	s.Close()
	if got := ledgerLines(t, path); got != 50001 {
		t.Fatalf("ledger has %d lines, want 50001", got)
	}
}

func TestEgressSinkSummaryAfterIntervalAndAtClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := newEgressSink(path, lc.logf, clk.now, 10*time.Second)
	for i := 0; i < 100; i++ { // 1 first line + 99 suppressed
		s.Record(refusedRecord("registry.npmjs.org"))
	}
	if n := len(lc.snapshot()); n != 1 {
		t.Fatalf("before the interval: %d lines, want 1", n)
	}
	clk.advance(11 * time.Second)
	s.Record(refusedRecord("registry.npmjs.org")) // 100 suppressed now, summary due
	lines := lc.snapshot()
	if len(lines) != 2 || !strings.Contains(lines[1], "100") || !strings.Contains(lines[1], "registry.npmjs.org") {
		t.Fatalf("summary with count 100 expected, got %v", lines)
	}
	for i := 0; i < 7; i++ {
		s.Record(refusedRecord("registry.npmjs.org"))
	}
	if n := len(lc.snapshot()); n != 2 {
		t.Fatalf("within the interval: %d lines, want 2", n)
	}
	s.Close()
	lines = lc.snapshot()
	if len(lines) != 3 || !strings.Contains(lines[2], "7") {
		t.Fatalf("final count 7 expected at Close, got %v", lines)
	}
	if got := ledgerLines(t, path); got != 108 {
		t.Fatalf("ledger has %d lines, want 108", got)
	}
}

func TestEgressSinkLedgerFlushedWithinASecondAndClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	s := newEgressSink(path, lc.logf, time.Now, 10*time.Second)
	s.Record(refusedRecord("a.example"))
	deadline := time.Now().Add(3 * time.Second)
	for ledgerLinesQuiet(path) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("record not on disk within 3s without Close")
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.Close()
	s.Close() // idempotent
	// A record after Close (a proxy goroutine finishing late) is still kept.
	s.Record(refusedRecord("a.example"))
	if got := ledgerLines(t, path); got != 2 {
		t.Fatalf("ledger has %d lines after late record, want 2", got)
	}
}

func ledgerLinesQuiet(path string) int {
	b, _ := os.ReadFile(path)
	return bytes.Count(b, []byte("\n"))
}

func TestEgressSinkConcurrentRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	s := newEgressSink(path, lc.logf, time.Now, 10*time.Second)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				s.Record(refusedRecord(fmt.Sprintf("h%d.example", g%3)))
			}
		}(g)
	}
	wg.Wait()
	s.Close()
	if got := ledgerLines(t, path); got != 4000 {
		t.Fatalf("ledger has %d lines, want 4000", got)
	}
}
