package capsule

import (
	"bytes"
	"encoding/json"
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
//   - the ledger file is left open after the capsule ends (Close must close
//     it), or a record is not on disk when Record returns (a process stop
//     would lose it; review 350);
//   - a failed first open stops the ledger for the capsule's life, silently
//     (review 350 F1: it must retry and say so once);
//   - a summary mixes reasons under one label, or is not JSON, so a reader
//     of capsule-egress lines cannot count (review 350 F2, F3);
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

func TestEgressSinkLedgerWrittenAtOnceAndClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	s := newEgressSink(path, lc.logf, time.Now, 10*time.Second)
	s.Record(refusedRecord("a.example"))
	if got := ledgerLinesQuiet(path); got != 1 {
		t.Fatalf("ledger has %d lines right after Record, want 1 (unbuffered)", got)
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

func TestEgressSinkRetriesAFailedOpenAndSaysSoOnce(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "egress")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil { // a file where the directory must go
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "cap1.jsonl")
	var lc logCapture
	s := newEgressSink(path, lc.logf, time.Now, 10*time.Second)
	defer s.Close()
	s.Record(refusedRecord("a.example"))
	s.Record(refusedRecord("a.example"))
	notWritable := 0
	for _, l := range lc.snapshot() {
		if strings.Contains(l, "not writable") {
			notWritable++
		}
	}
	if notWritable != 1 {
		t.Fatalf("open failure logged %d times, want once: %v", notWritable, lc.snapshot())
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	s.Record(refusedRecord("a.example"))
	if got := ledgerLines(t, path); got != 1 {
		t.Fatalf("after the path became writable the ledger has %d lines, want 1 (the open is retried)", got)
	}
	again := 0
	for _, l := range lc.snapshot() {
		if strings.Contains(l, "writable again; 2 records were not written") {
			again++
		}
	}
	if again != 1 {
		t.Fatalf("recovery with the dropped count logged %d times, want once: %v", again, lc.snapshot())
	}
	// A second episode is logged once again (review 351 F2).
	if err := os.Rename(blocker, blocker+".d"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		s.Record(refusedRecord("a.example"))
	}
	// The held-open file still points at the renamed directory, so these
	// writes succeed; force a reopen through a failing path instead.
	s.mu.Lock()
	_ = s.f.Close()
	s.f = nil
	s.mu.Unlock()
	for i := 0; i < 100; i++ {
		s.Record(refusedRecord("a.example"))
	}
	notWritable = 0
	for _, l := range lc.snapshot() {
		if strings.Contains(l, "not writable") {
			notWritable++
		}
	}
	if notWritable != 2 {
		t.Fatalf("two fault episodes logged %d times, want 2 (one each, not one per record): %v", notWritable, lc.snapshot())
	}
}

func TestEgressSinkSummaryIsJSONWithPerReasonCounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cap1.jsonl")
	var lc logCapture
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := newEgressSink(path, lc.logf, clk.now, 10*time.Second)
	a, b := refusedRecord("h.example"), refusedRecord("h.example")
	b.Reason = "other reason"
	s.Record(a) // first line for reason a
	s.Record(b) // first line for reason b
	for i := 0; i < 3; i++ {
		s.Record(a)
	}
	for i := 0; i < 2; i++ {
		s.Record(b)
	}
	s.Close()
	lines := lc.snapshot()
	if len(lines) != 3 {
		t.Fatalf("want two first lines and one summary, got %v", lines)
	}
	var sum egressSummary
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[2], "capsule-egress: ")), &sum); err != nil {
		t.Fatalf("summary is not JSON: %q: %v", lines[2], err)
	}
	if sum.Kind != "summary" || sum.Count != 5 || sum.Reasons[a.Reason] != 3 || sum.Reasons[b.Reason] != 2 || sum.Host != "h.example" {
		t.Fatalf("summary = %+v, want count 5 split 3 and 2", sum)
	}
	if got := ledgerLines(t, path); got != 7 {
		t.Fatalf("ledger has %d lines, want 7", got)
	}
}
