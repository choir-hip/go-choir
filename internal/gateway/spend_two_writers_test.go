package gateway

// Station G1 of docs/design/gateway-handoff-without-a-gap-2026-11-20.md: the
// two spend files are safe for two writers (two gateway processes run at
// once during a handoff). Two PaidSpend or two AnthropicRelay values on one
// file stand in for two processes. A save runs on a flusher, so every test
// that reads the file calls Flush first.
//
// Failure list, written before the code:
//  1. Two PaidSpend values on one file record calls in turn: the file's day
//     total is the sum of both (today each save overwrites the other's).
//  2. The same for the relay, per computer.
//  3. One process saves after the UTC day turned and the other has not
//     rolled: no call is counted on the wrong day.
//  4. A temp file left by a killed writer (and a lock file left behind)
//     does not block or corrupt the next save.
//  5. A process that starts after another has saved loads the merged total.
//  6. Refusal refuses once the merged total passes the budget, though this
//     process alone is under it.
//  7. A lock that cannot be taken within its deadline keeps the delta; the
//     next save that gets the lock writes it, and no call is lost or counted
//     twice.
//  8. The save leaves no temp file behind.
//
// Added after the first review (both seats sent G1 back):
//  9. A spend file that is present but bad (truncated JSON, empty, wrong
//     type, no day) is never overwritten from memory, by either ledger; the
//     delta is kept, the save returns an error, the status says so, the log
//     line is rate-limited, and the cap check still counts memory plus delta.
//     Once the file is good the kept delta merges into it.
// 10. A file whose day is ahead of this process's clock never rolls this
//     process backwards: b.Snapshot() and b.Refusal() still see the later
//     day's spend (the earlier test asserted the file only).
// 11. A guest's Record does not wait for the file lock at all (the save runs
//     on a flusher, outside the ledger mutex).
// 12. Calls recorded while a save is between its snapshot and its fold into
//     memory are neither lost nor doubled.
// 13. Two real ledgers per file, written from goroutines concurrently: the
//     final file holds every call (paid and relay).
// 14. The relay's lock timeout keeps its delta; a relay whose clock is
//     behind the file's day keeps its own memory (spentToday is not zeroed).
// 15. A lock file that cannot be opened (wrong owner or mode; here it is a
//     directory) is surfaced in Persistence(), not silent.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

type flusher interface{ Flush() error }

func mustFlush(t *testing.T, f flusher) {
	t.Helper()
	if err := f.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func readPaidFile(t *testing.T, path string) paidSpendRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec paidSpendRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func readRelayFile(t *testing.T, path string) relaySpendRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec relaySpendRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func noTempFiles(t *testing.T, dir string) {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func holdLock(t *testing.T, path string) (release func()) {
	t.Helper()
	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	return func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN); lf.Close() }
}

// stopSaver ends a ledger's flusher before its temp directory is removed, so
// a retrying flusher cannot recreate a lock file in a directory being deleted.
func stopSaver(t *testing.T, p *spendSaver) { t.Cleanup(p.stop) }

func shortLockWait(t *testing.T, d time.Duration) {
	old := spendLockWait.Swap(int64(d))
	t.Cleanup(func() { spendLockWait.Store(old) })
}

var badSpendFiles = map[string]string{
	"truncated": `{"day":"2026-11-09","today":{"river":{"usd":`,
	"empty":     ``,
	"wrongtype": `{"day":5,"today":[]}`,
	"array":     `[]`,
	"noday":     `{"today":{}}`,
}

func TestPaidSpendTwoWritersSumTheirCalls(t *testing.T) { // 1, 8
	dir := t.TempDir()
	path := filepath.Join(dir, "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestSpend(t, path, clk), newTestSpend(t, path, clk)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1_000_000})
	mustFlush(t, a)
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 1_000_000})
	mustFlush(t, b)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1_000_000})
	mustFlush(t, a)
	d := readPaidFile(t, path).Today["river"]
	if d == nil || d.Calls != 3 || d.InputTokens != 3_000_000 {
		t.Fatalf("file day total is not the sum of both writers: %+v", d)
	}
	if !near(d.Computers["vm-a"], 2*d.Computers["vm-b"]) || d.Computers["vm-b"] <= 0 {
		t.Fatalf("per-computer split lost: %+v", d.Computers)
	}
	noTempFiles(t, dir)
}

func TestPaidSpendRefusalSeesTheOtherWritersSpend(t *testing.T) { // 6
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestSpend(t, path, clk), newTestSpend(t, path, clk)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 5_000_000})
	mustFlush(t, a)
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 5_000_000})
	mustFlush(t, b)
	// b saved last, so it sees both at once; a sees b's spend at its next save.
	a.Record("river", "vm-a", riverModel, provider.Usage{})
	mustFlush(t, a)
	if a.Refusal("river", riverModel) == nil || b.Refusal("river", riverModel) == nil {
		t.Fatalf("merged total is over budget but a process still admits: a=%+v b=%+v", a.Snapshot().Providers["river"], b.Snapshot().Providers["river"])
	}
}

func TestPaidSpendLateStarterLoadsMergedTotal(t *testing.T) { // 5
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestSpend(t, path, clk), newTestSpend(t, path, clk)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 100_000})
	mustFlush(t, a)
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 100_000})
	mustFlush(t, b)
	if p := newTestSpend(t, path, clk).Snapshot().Providers["river"]; p.Calls != 2 {
		t.Fatalf("late starter loaded %+v", p)
	}
}

func TestPaidSpendDayTurnsInOneWriterOnly(t *testing.T) { // 3, 10
	path := filepath.Join(t.TempDir(), "spend.json")
	clkA := &spendClock{time.Date(2026, 11, 9, 23, 59, 0, 0, time.UTC)}
	clkB := &spendClock{clkA.t}
	a, b := newTestSpend(t, path, clkA), newTestSpend(t, path, clkB)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1000}) // day 09
	mustFlush(t, a)
	clkA.t = clkA.t.Add(2 * time.Minute) // A is on day 10, B still on 09
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10_000_000})
	mustFlush(t, a)                                                          // day 10: over river's $1 budget
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 1000}) // day 09, saved after the turn
	mustFlush(t, b)
	rec := readPaidFile(t, path)
	if rec.Day != "2026-11-10" || rec.Today["river"].Calls != 1 {
		t.Fatalf("day 10 should hold exactly A's one call: %+v", rec.Today["river"])
	}
	if rec.PrevDay != "2026-11-09" || rec.Prev["river"].Calls != 2 {
		t.Fatalf("B's late day-09 call must land on day 09: %+v %+v", rec.PrevDay, rec.Prev["river"])
	}
	// B's memory must be the file's, not a day rolled backwards over it.
	snap := b.Snapshot()
	if snap.Day != "2026-11-10" || snap.Providers["river"].Calls != 1 || snap.PreviousDay != "2026-11-09" || snap.Previous["river"].Calls != 2 {
		t.Fatalf("b's memory after the other writer turned the day: %+v", snap)
	}
	if b.Refusal("river", riverModel) == nil {
		t.Fatal("b admits although day 10's spend is over budget (its clock is behind the file)")
	}
}

func TestPaidSpendFileAheadOfClockIsNotRolledBackwards(t *testing.T) { // 10
	path := filepath.Join(t.TempDir(), "spend.json")
	clkA := &spendClock{time.Date(2026, 11, 10, 0, 5, 0, 0, time.UTC)}
	clkB := &spendClock{time.Date(2026, 11, 9, 23, 55, 0, 0, time.UTC)} // a step back across midnight
	a, b := newTestSpend(t, path, clkA), newTestSpend(t, path, clkB)
	a.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10_000_000})
	mustFlush(t, a)
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 1})
	mustFlush(t, b)
	if b.Refusal("river", riverModel) == nil {
		t.Fatalf("a backward clock step emptied today's spend: %+v", b.Snapshot())
	}
	// And a later save on b does not lose a's day-10 spend from the file.
	b.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 1})
	mustFlush(t, b)
	if rec := readPaidFile(t, path); rec.Day != "2026-11-10" || rec.Today["river"].Calls != 2 || rec.Prev["river"].Calls != 1 {
		t.Fatalf("file after b's second save: %+v", rec)
	}
}

func TestPaidSpendLeftoverTempAndLockDoNotBlock(t *testing.T) { // 4
	dir := t.TempDir()
	path := filepath.Join(dir, "spend.json")
	for _, n := range []string{"spend.json.tmp", "spend.json.123.tmp", "spend.json.lock"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{half"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := newTestSpend(t, path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1000})
	mustFlush(t, s)
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 1 {
		t.Fatalf("save after a killed writer: %+v", rec)
	}
}

func TestPaidSpendLockTimeoutKeepsTheDelta(t *testing.T) { // 7, 11
	shortLockWait(t, 50*time.Millisecond)
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	other := newTestSpend(t, path, clk)
	other.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 1000})
	mustFlush(t, other)
	s := newTestSpend(t, path, clk)
	stopSaver(t, &s.saver)
	release := holdLock(t, path)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1000})
	if err := s.Flush(); err == nil {
		t.Fatal("a flush through a held lock reported success")
	}
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 1 {
		t.Fatalf("a save went through a held lock: %+v", rec.Today["river"])
	}
	if p := s.Persistence(); p.UnsavedCalls != 1 || p.LastError == "" {
		t.Fatalf("status while unsaved: %+v", p)
	}
	release()
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1000})
	mustFlush(t, s)
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 3 {
		t.Fatalf("kept delta lost or doubled: %+v", rec.Today["river"])
	}
	if c := s.Snapshot().Providers["river"].Calls; c != 3 {
		t.Fatalf("memory after merge: %d", c)
	}
	if p := s.Persistence(); p.UnsavedCalls != 0 || p.LastError != "" || p.LastSaveAt == "" {
		t.Fatalf("status after the save landed: %+v", p)
	}
}

func TestPaidSpendRecordDoesNotWaitForTheLock(t *testing.T) { // 11
	shortLockWait(t, 3*time.Second)
	path := filepath.Join(t.TempDir(), "spend.json")
	s := newTestSpend(t, path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	stopSaver(t, &s.saver)
	release := holdLock(t, path)
	defer release()
	start := time.Now()
	for i := 0; i < 5; i++ {
		s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
		_ = s.Refusal("river", riverModel)
		_ = s.Snapshot()
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Record/Refusal/Snapshot waited %v on a held file lock", d)
	}
}

func TestPaidSpendCallsDuringASaveAreNeitherLostNorDoubled(t *testing.T) { // 12
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	s := newTestSpend(t, path, clk)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	mustFlush(t, s)
	hook := func() {
		spendSaveWindowHook.Store(nil)
		for i := 0; i < 5; i++ { // while the save is between snapshot and fold
			s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
		}
	}
	spendSaveWindowHook.Store(&hook)
	defer spendSaveWindowHook.Store(nil)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	mustFlush(t, s)
	mustFlush(t, s) // the five that arrived during the first save
	if c := s.Snapshot().Providers["river"].Calls; c != 7 {
		t.Fatalf("memory calls %d, want 7", c)
	}
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 7 {
		t.Fatalf("file calls %d, want 7", rec.Today["river"].Calls)
	}
}

func TestPaidSpendConcurrentWritersKeepEveryCall(t *testing.T) { // 13
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestSpend(t, path, clk), newTestSpend(t, path, clk)
	const n = 50
	var wg sync.WaitGroup
	for _, s := range []*PaidSpend{a, b} {
		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func(s *PaidSpend) {
				defer wg.Done()
				for i := 0; i < n; i++ {
					s.Record("river", "vm", riverModel, provider.Usage{InputTokens: 1})
					_ = s.Refusal("river", riverModel)
				}
			}(s)
		}
	}
	wg.Wait()
	mustFlush(t, a)
	mustFlush(t, b)
	a.Record("river", "vm", riverModel, provider.Usage{InputTokens: 1}) // a's next save takes b's in
	mustFlush(t, a)
	d := readPaidFile(t, path).Today["river"]
	if d.Calls != 4*n+1 || d.InputTokens != 4*n+1 {
		t.Fatalf("file holds %d calls, want %d", d.Calls, 4*n+1)
	}
	if c := a.Snapshot().Providers["river"].Calls; c != 4*n+1 {
		t.Fatalf("a's memory holds %d calls, want %d", c, 4*n+1)
	}
}

func TestPaidSpendBadFileIsNeverOverwritten(t *testing.T) { // 9
	for name, content := range badSpendFiles {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "spend.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
			s := newTestSpend(t, path, clk)
			stopSaver(t, &s.saver)
			s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10_000_000}) // over $1
			if err := s.Flush(); err == nil {
				t.Fatal("a flush over a bad file reported success")
			}
			if got, _ := os.ReadFile(path); string(got) != content {
				t.Fatalf("bad file was overwritten: %q", got)
			}
			// The cap still counts what this process knows: memory plus delta.
			if s.Refusal("river", riverModel) == nil {
				t.Fatal("cap check forgot this process's own spend")
			}
			p := s.Persistence()
			if p.UnsavedCalls != 1 || p.LastError == "" || p.LastErrorAt == "" {
				t.Fatalf("status: %+v", p)
			}
			// Repeated failures log once a minute, not per call.
			for i := 0; i < 5; i++ {
				s.Record("river", "vm-a", riverModel, provider.Usage{})
				_ = s.Flush()
			}
			if p := s.Persistence(); p.ErrorLogs != 1 || p.UnsavedCalls != 6 {
				t.Fatalf("log lines %d (want 1), unsaved %d (want 6)", p.ErrorLogs, p.UnsavedCalls)
			}
			// The operator repairs the file: the kept delta merges, nothing is lost.
			good := paidSpendRecord{Day: "2026-11-09", Today: map[string]*providerDay{"river": {Calls: 3, USD: 0.5}}}
			raw, _ := json.Marshal(good)
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			mustFlush(t, s)
			if d := readPaidFile(t, path).Today["river"]; d.Calls != 3+6 {
				t.Fatalf("after repair the file holds %d calls, want 9", d.Calls)
			}
			if p := s.Persistence(); p.UnsavedCalls != 0 || p.LastError != "" {
				t.Fatalf("status after repair: %+v", p)
			}
		})
	}
}

func TestPaidSpendAbsentFileIsEmpty(t *testing.T) { // 9: absent is not bad
	path := filepath.Join(t.TempDir(), "spend.json")
	s := newTestSpend(t, path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	mustFlush(t, s)
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 1 {
		t.Fatalf("%+v", rec)
	}
	// The file vanishes (operator removed it): memory is written back whole.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	mustFlush(t, s)
	if rec := readPaidFile(t, path); rec.Today["river"].Calls != 2 {
		t.Fatalf("%+v", rec.Today["river"])
	}
}

func TestPaidSpendUnopenableLockIsSurfaced(t *testing.T) { // 15
	path := filepath.Join(t.TempDir(), "spend.json")
	if err := os.Mkdir(path+".lock", 0o700); err != nil { // cannot be opened as a file
		t.Fatal(err)
	}
	s := newTestSpend(t, path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	stopSaver(t, &s.saver)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	if err := s.Flush(); err == nil {
		t.Fatal("no error for a lock that cannot be opened")
	}
	if p := s.Persistence(); p.UnsavedCalls != 1 || !strings.Contains(p.LastError, "lock") {
		t.Fatalf("status: %+v", p)
	}
}

func TestPaidSpendSnapshotCarriesPersistence(t *testing.T) { // 15
	path := filepath.Join(t.TempDir(), "spend.json")
	s := newTestSpend(t, path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10})
	mustFlush(t, s)
	if s.Snapshot().Persistence == nil || s.Snapshot().Persistence.LastSaveAt == "" {
		t.Fatalf("snapshot lacks the persistence block: %+v", s.Snapshot().Persistence)
	}
}

func newTestRelay(path string, clk *spendClock) *AnthropicRelay {
	return &AnthropicRelay{spendFile: path, now: clk.now}
}

func TestRelayTwoWritersSumPerComputer(t *testing.T) { // 2, 8
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestRelay(path, clk), newTestRelay(path, clk)
	a.charge("vm-1", relayUsage{InputTokens: 100, OutputTokens: 10})
	mustFlush(t, a)
	b.charge("vm-1", relayUsage{InputTokens: 200})
	b.charge("vm-2", relayUsage{InputTokens: 7})
	mustFlush(t, b)
	a.charge("vm-1", relayUsage{CacheCreationTokens: 1})
	mustFlush(t, a)
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 311 || rec.Spent["vm-2"] != 7 {
		t.Fatalf("file: %+v", rec)
	}
	if a.spentToday("vm-1") != 311 {
		t.Fatalf("a does not see b's spend: %d", a.spentToday("vm-1"))
	}
	noTempFiles(t, dir)
}

func TestRelayCapSeesTheOtherWritersSpend(t *testing.T) { // 6
	path := filepath.Join(t.TempDir(), "relay.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestRelay(path, clk), newTestRelay(path, clk)
	a.dailyTokens, b.dailyTokens = 1000, 1000
	a.charge("vm-1", relayUsage{InputTokens: 600})
	mustFlush(t, a)
	b.charge("vm-1", relayUsage{InputTokens: 600})
	mustFlush(t, b)
	a.charge("vm-2", relayUsage{InputTokens: 1}) // a's next save takes b's spend in
	mustFlush(t, a)
	if !a.budgetSpent("vm-1") || !b.budgetSpent("vm-1") {
		t.Fatal("merged total is over the cap but a process still admits")
	}
}

func TestRelayLateStarterAndDayTurn(t *testing.T) { // 5, 3
	path := filepath.Join(t.TempDir(), "relay.json")
	clkA := &spendClock{time.Date(2026, 11, 9, 23, 59, 0, 0, time.UTC)}
	clkB := &spendClock{clkA.t}
	a, b := newTestRelay(path, clkA), newTestRelay(path, clkB)
	a.charge("vm-1", relayUsage{InputTokens: 5})
	mustFlush(t, a)
	c := newTestRelay(path, clkA)
	c.loadSpend()
	if c.spentToday("vm-1") != 5 {
		t.Fatalf("late starter: %d", c.spentToday("vm-1"))
	}
	clkA.t = clkA.t.Add(2 * time.Minute) // A on day 10; B still on 09
	a.charge("vm-1", relayUsage{InputTokens: 3})
	mustFlush(t, a)
	b.charge("vm-1", relayUsage{InputTokens: 100}) // day 09, saved late
	mustFlush(t, b)
	if rec := readRelayFile(t, path); rec.Day != "2026-11-10" || rec.Spent["vm-1"] != 3 {
		t.Fatalf("a day-09 charge counted on day 10: %+v", rec)
	}
	// B's own clock is still day 09: its memory keeps its own day, never zero.
	if got := b.spentToday("vm-1"); got < 100 {
		t.Fatalf("a relay whose clock is behind the file lost its own day: %d", got)
	}
}

func TestRelayLeftoverTempAndLockDoNotBlock(t *testing.T) { // 4
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.json")
	for _, n := range []string{"relay.json.tmp", "relay.json.9.tmp", "relay.json.lock"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("{half"), 0o600)
	}
	r := newTestRelay(path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	r.charge("vm-1", relayUsage{InputTokens: 4})
	mustFlush(t, r)
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 4 {
		t.Fatalf("file: %+v", rec)
	}
}

func TestRelayLockTimeoutKeepsTheDelta(t *testing.T) { // 14, 11
	shortLockWait(t, 50*time.Millisecond)
	path := filepath.Join(t.TempDir(), "relay.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	other := newTestRelay(path, clk)
	other.charge("vm-2", relayUsage{InputTokens: 9})
	mustFlush(t, other)
	r := newTestRelay(path, clk)
	stopSaver(t, &r.saver)
	release := holdLock(t, path)
	start := time.Now()
	r.charge("vm-1", relayUsage{InputTokens: 4})
	_ = r.spentToday("vm-1")
	if time.Since(start) > time.Second {
		t.Fatal("a charge waited on the file lock")
	}
	if err := r.Flush(); err == nil {
		t.Fatal("flush through a held lock reported success")
	}
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 0 {
		t.Fatalf("a save went through a held lock: %+v", rec)
	}
	if p := r.Persistence(); p.UnsavedCalls != 1 || p.LastError == "" {
		t.Fatalf("status: %+v", p)
	}
	release()
	r.charge("vm-1", relayUsage{InputTokens: 6})
	mustFlush(t, r)
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 10 || rec.Spent["vm-2"] != 9 {
		t.Fatalf("kept delta lost or doubled: %+v", rec)
	}
	if r.spentToday("vm-1") != 10 || r.spentToday("vm-2") != 9 {
		t.Fatalf("memory after merge: %d %d", r.spentToday("vm-1"), r.spentToday("vm-2"))
	}
}

func TestRelayCallsDuringASaveAreNeitherLostNorDoubled(t *testing.T) { // 12
	path := filepath.Join(t.TempDir(), "relay.json")
	r := newTestRelay(path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	r.charge("vm-1", relayUsage{InputTokens: 1})
	mustFlush(t, r)
	hook := func() {
		spendSaveWindowHook.Store(nil)
		for i := 0; i < 5; i++ {
			r.charge("vm-1", relayUsage{InputTokens: 1})
		}
	}
	spendSaveWindowHook.Store(&hook)
	defer spendSaveWindowHook.Store(nil)
	r.charge("vm-1", relayUsage{InputTokens: 1})
	mustFlush(t, r)
	mustFlush(t, r)
	if got := r.spentToday("vm-1"); got != 7 {
		t.Fatalf("memory %d, want 7", got)
	}
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 7 {
		t.Fatalf("file %+v, want 7", rec)
	}
}

func TestRelayConcurrentWritersKeepEveryCharge(t *testing.T) { // 13
	path := filepath.Join(t.TempDir(), "relay.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	a, b := newTestRelay(path, clk), newTestRelay(path, clk)
	const n = 50
	var wg sync.WaitGroup
	for _, r := range []*AnthropicRelay{a, b} {
		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func(r *AnthropicRelay) {
				defer wg.Done()
				for i := 0; i < n; i++ {
					r.charge("vm-1", relayUsage{InputTokens: 1})
					_ = r.spentToday("vm-1")
				}
			}(r)
		}
	}
	wg.Wait()
	mustFlush(t, a)
	mustFlush(t, b)
	a.charge("vm-1", relayUsage{InputTokens: 1}) // a's next save takes b's in
	mustFlush(t, a)
	if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 4*n+1 {
		t.Fatalf("file holds %d, want %d", rec.Spent["vm-1"], 4*n+1)
	}
	if a.spentToday("vm-1") != 4*n+1 {
		t.Fatalf("a's memory %d, want %d", a.spentToday("vm-1"), 4*n+1)
	}
}

func TestRelayBadFileIsNeverOverwritten(t *testing.T) { // 9
	for name, content := range badSpendFiles {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "relay.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
			r := newTestRelay(path, clk)
			stopSaver(t, &r.saver)
			r.dailyTokens = 1000
			r.charge("vm-1", relayUsage{InputTokens: 600})
			if err := r.Flush(); err == nil {
				t.Fatal("flush over a bad file reported success")
			}
			if got, _ := os.ReadFile(path); string(got) != content {
				t.Fatalf("bad file was overwritten: %q", got)
			}
			if r.spentToday("vm-1") != 600 {
				t.Fatalf("memory lost its own spend: %d", r.spentToday("vm-1"))
			}
			r.charge("vm-1", relayUsage{InputTokens: 600})
			if !r.budgetSpent("vm-1") {
				t.Fatal("cap check forgot memory plus delta")
			}
			for i := 0; i < 4; i++ {
				_ = r.Flush()
			}
			if p := r.Persistence(); p.ErrorLogs != 1 || p.UnsavedCalls != 2 || p.LastError == "" {
				t.Fatalf("status: %+v", p)
			}
			raw, _ := json.Marshal(relaySpendRecord{Day: "2026-11-09", Spent: map[string]int64{"vm-9": 50}})
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			mustFlush(t, r)
			if rec := readRelayFile(t, path); rec.Spent["vm-1"] != 1200 || rec.Spent["vm-9"] != 50 {
				t.Fatalf("after repair: %+v", rec)
			}
		})
	}
}

func TestRelayUnopenableLockIsSurfaced(t *testing.T) { // 15
	path := filepath.Join(t.TempDir(), "relay.json")
	if err := os.Mkdir(path+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	r := newTestRelay(path, &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)})
	stopSaver(t, &r.saver)
	r.charge("vm-1", relayUsage{InputTokens: 1})
	if err := r.Flush(); err == nil || !strings.Contains(r.Persistence().LastError, "lock") {
		t.Fatalf("not surfaced: %v %+v", err, r.Persistence())
	}
}
