package capsule

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// egressSinkFlushEvery bounds how long a ledger record may sit in the write
// buffer before it reaches the file.
const egressSinkFlushEvery = time.Second

// egressSinkLogEvery is the default gap between summary lines for one host.
const egressSinkLogEvery = 10 * time.Second

type egressLogKey struct{ host, outcome string }

type egressLogState struct {
	suppressed int
	lastLog    time.Time
	lastReason string
}

// egressSink receives every EgressRecord of one capsule. Every record goes to
// the ledger file, which is opened once and written through a buffer flushed
// every second and at Close. The process log (the guest journal and serial
// console) gets the first record for each (host, outcome, reason) at once and
// then at most one count line per (host, outcome) per interval, so a retry
// loop of tens of thousands of refused connections is not tens of thousands
// of synchronous console writes (H9 in docs/problems/
// capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md).
type egressSink struct {
	path     string
	logf     func(format string, args ...any)
	now      func() time.Time
	interval time.Duration

	mu      sync.Mutex
	f       *os.File
	w       *bufio.Writer
	opened  bool
	closed  bool
	seen    map[egressLogKey]map[string]bool
	state   map[egressLogKey]*egressLogState
	stop    chan struct{}
	stopped chan struct{}
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
	s := &egressSink{
		path: path, logf: logf, now: now, interval: interval,
		seen:  map[egressLogKey]map[string]bool{},
		state: map[egressLogKey]*egressLogState{},
		stop:  make(chan struct{}), stopped: make(chan struct{}),
	}
	go s.flushLoop()
	return s
}

func (s *egressSink) flushLoop() {
	defer close(s.stopped)
	t := time.NewTicker(egressSinkFlushEvery)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			if s.w != nil {
				_ = s.w.Flush()
			}
			s.mu.Unlock()
		}
	}
}

// Record writes the record to the ledger and, subject to aggregation, the log.
func (s *egressSink) Record(record EgressRecord) {
	line, _ := json.Marshal(record)
	line = append(line, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeLedgerLocked(line)
	s.logLocked(record, line)
}

func (s *egressSink) writeLedgerLocked(line []byte) {
	if s.closed {
		// A proxy goroutine finishing after the capsule ended: the record is
		// still evidence, so append it directly.
		if f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(line)
			_ = f.Close()
		}
		return
	}
	if !s.opened {
		s.opened = true
		if f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			s.f, s.w = f, bufio.NewWriterSize(f, 32<<10)
		}
	}
	if s.w != nil {
		_, _ = s.w.Write(line)
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
		st = &egressLogState{lastLog: now}
		s.state[key] = st
	}
	if !reasons[record.Reason] {
		reasons[record.Reason] = true
		st.lastLog = now
		s.logf("capsule-egress: %s", line[:len(line)-1])
		return
	}
	st.suppressed++
	st.lastReason = record.Reason
	if now.Sub(st.lastLog) >= s.interval {
		s.summaryLocked(key, st, now)
	}
}

func (s *egressSink) summaryLocked(key egressLogKey, st *egressLogState, now time.Time) {
	s.logf("capsule-egress: %d more %s connections to %s in the last %ds (reason: %s); every one is on the capsule's egress ledger",
		st.suppressed, key.outcome, key.host, int(now.Sub(st.lastLog).Seconds()), st.lastReason)
	st.suppressed = 0
	st.lastLog = now
}

// Close logs any suppressed counts, flushes and closes the ledger file. It is
// safe to call more than once.
func (s *egressSink) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	now := s.now()
	for key, st := range s.state {
		if st.suppressed > 0 {
			s.summaryLocked(key, st, now)
		}
	}
	if s.w != nil {
		_ = s.w.Flush()
	}
	if s.f != nil {
		_ = s.f.Close()
	}
	s.w, s.f = nil, nil
	close(s.stop)
	s.mu.Unlock()
	<-s.stopped
}
