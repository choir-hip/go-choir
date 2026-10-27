package capsule

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// egressSinkLogEvery is the default gap between summary lines for one host.
const egressSinkLogEvery = 10 * time.Second

type egressLogKey struct{ host, outcome string }

type egressLogState struct {
	lastLog    time.Time
	suppressed map[string]int // reason -> records not logged since lastLog
}

// egressSummary is the process-log line that stands for records the log did
// not print one by one. It is JSON, like the per-record lines, so a reader of
// `capsule-egress:` lines can tell the two apart by kind and still count.
type egressSummary struct {
	Kind     string         `json:"kind"`
	Host     string         `json:"host"`
	Outcome  string         `json:"outcome"`
	Count    int            `json:"count"`
	Reasons  map[string]int `json:"reasons"`
	WindowMS int64          `json:"window_ms"`
}

// egressSink receives every EgressRecord of one capsule. Every record goes to
// the ledger file, which is opened once and kept open; each record is one
// unbuffered write, so a record is in the file when Record returns and a
// process stop loses nothing. The process log (the guest journal and serial
// console) gets the first record for each (host, outcome, reason) at once and
// then at most one JSON summary per (host, outcome) per interval, so a retry
// loop of tens of thousands of refused connections is not tens of thousands
// of synchronous console writes (H9 in docs/problems/
// capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md). The ledger is the
// only per-connection record; the log is a digest.
type egressSink struct {
	path     string
	logf     func(format string, args ...any)
	now      func() time.Time
	interval time.Duration

	mu     sync.Mutex
	f      *os.File
	closed bool
	// faulted is set while the ledger cannot be opened or written; its first
	// failure is logged, later ones only counted into dropped, so a faulted
	// disk is one log line, not one per record. A successful write clears it,
	// so each new episode is logged once.
	faulted bool
	dropped int
	seen    map[egressLogKey]map[string]bool
	state   map[egressLogKey]*egressLogState
}

func newEgressSink(path string, logf func(format string, args ...any), now func() time.Time, interval time.Duration) *egressSink {
	if logf == nil {
		logf = log.Printf
	}
	if now == nil {
		now = time.Now
	}
	if interval <= 0 {
		interval = egressSinkLogEvery
	}
	return &egressSink{
		path: path, logf: logf, now: now, interval: interval,
		seen:  map[egressLogKey]map[string]bool{},
		state: map[egressLogKey]*egressLogState{},
	}
}

// Record writes the record to the ledger and, subject to aggregation, the log.
func (s *egressSink) Record(record EgressRecord) {
	line, err := json.Marshal(record)
	if err != nil {
		s.logf("capsule-egress: record for %s not encodable: %v", record.Host, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeLedgerLocked(append(line, '\n'))
	s.logLocked(record, line)
}

func openLedger(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func (s *egressSink) writeLedgerLocked(line []byte) {
	if s.closed {
		// A proxy goroutine finishing after the capsule ended: the record is
		// still evidence, so append it directly.
		f, err := openLedger(s.path)
		if err == nil {
			_, err = f.Write(line)
			_ = f.Close()
		}
		s.noteLedgerLocked(err)
		return
	}
	if s.f == nil {
		// Retried on every record until it succeeds: a transient failure
		// (EMFILE under a connection flood, a mount not ready) costs only the
		// records written while it lasts.
		f, err := openLedger(s.path)
		if err != nil {
			s.noteLedgerLocked(err)
			return
		}
		s.f = f
	}
	if _, err := s.f.Write(line); err != nil {
		_ = s.f.Close()
		s.f = nil // reopened on the next record
		s.noteLedgerLocked(err)
		return
	}
	s.noteLedgerLocked(nil)
}

// noteLedgerLocked records the outcome of one ledger write: the first failure
// of an episode is logged, later ones are counted, and a success ends the
// episode with the count.
func (s *egressSink) noteLedgerLocked(err error) {
	if err == nil {
		if s.faulted {
			s.logf("capsule-egress: ledger %s writable again; %d records were not written", s.path, s.dropped)
			s.faulted, s.dropped = false, 0
		}
		return
	}
	s.dropped++
	if !s.faulted {
		s.faulted = true
		s.logf("capsule-egress: ledger %s not writable: %v; retrying on each record, failures counted", s.path, err)
	}
}

func (s *egressSink) logLocked(record EgressRecord, line []byte) {
	key := egressLogKey{record.Host, record.Outcome}
	now := s.now()
	reasons := s.seen[key]
	if reasons == nil {
		reasons = map[string]bool{}
		s.seen[key] = reasons
	}
	st := s.state[key]
	if st == nil {
		st = &egressLogState{lastLog: now, suppressed: map[string]int{}}
		s.state[key] = st
	}
	if !reasons[record.Reason] {
		reasons[record.Reason] = true
		s.logf("capsule-egress: %s", line)
		return
	}
	st.suppressed[record.Reason]++
	if now.Sub(st.lastLog) >= s.interval {
		s.summaryLocked(key, st, now)
	}
}

func (s *egressSink) summaryLocked(key egressLogKey, st *egressLogState, now time.Time) {
	sum := egressSummary{Kind: "summary", Host: key.host, Outcome: key.outcome, Reasons: st.suppressed, WindowMS: now.Sub(st.lastLog).Milliseconds()}
	for _, n := range st.suppressed {
		sum.Count += n
	}
	line, _ := json.Marshal(sum)
	s.logf("capsule-egress: %s", line)
	st.suppressed = map[string]int{}
	st.lastLog = now
}

// Close logs any suppressed counts and closes the ledger file. It is safe to
// call more than once.
func (s *egressSink) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	now := s.now()
	keys := make([]egressLogKey, 0, len(s.state))
	for key := range s.state {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].host != keys[j].host {
			return keys[i].host < keys[j].host
		}
		return keys[i].outcome < keys[j].outcome
	})
	for _, key := range keys {
		if st := s.state[key]; len(st.suppressed) > 0 {
			s.summaryLocked(key, st, now)
		}
	}
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	if s.faulted {
		s.logf("capsule-egress: ledger %s still not writable at close; %d records were not written", s.path, s.dropped)
	}
}
