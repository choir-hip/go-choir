package capsule

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

// The capsule broker's own log, kept per capsule where the observability
// route can return it (docs/problems/engineering-cell-never-returns-2026-11-03.md:
// the broker's stderr went to the journal only, so a broker that went
// silent for 46 minutes left no broker-side evidence on the product path).
//
// What the broker writes to stderr is its own operational log: the socket
// and merged paths, readiness, the session worker's hardening failures
// before it is ready, the egress forwarder's and the Claude delegate's
// state (never the token), and a Go runtime fatal or panic with its stack.
// Cell output, job output (the job's log file) and the Claude process's
// stderr (the delegate's own file) never reach it.

const (
	// brokerLogLines is how many of a broker's last lines are kept.
	brokerLogLines = 200
	// brokerLogLineMax bounds one kept line, in bytes.
	brokerLogLineMax = 512
	// brokerLogFatalMax bounds the pinned fatal lines: the first lines that
	// say why a Go process died, which a goroutine dump after them would
	// scroll out of the tail.
	brokerLogFatalMax = 20
	// brokerLogsEndedKept is how many ended capsules' logs are kept.
	brokerLogsEndedKept = 4
)

// brokerLogTail is a bounded ring of one broker's log lines.
type brokerLogTail struct {
	mu    sync.Mutex
	cap   int
	lines []string
	fatal []string
	// streamDone is closed when the broker's stderr reached EOF.
	streamDone chan struct{}
	streamOnce sync.Once
}

func newBrokerLogTail(lines int) *brokerLogTail {
	if lines <= 0 {
		lines = brokerLogLines
	}
	return &brokerLogTail{cap: lines, streamDone: make(chan struct{})}
}

// endStream marks the broker's stderr read to its end.
func (t *brokerLogTail) endStream() {
	if t == nil {
		return
	}
	t.streamOnce.Do(func() { close(t.streamDone) })
}

// waitStream waits up to d for the stderr to be read to its end.
func (t *brokerLogTail) waitStream(d time.Duration) {
	if t == nil {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-t.streamDone:
	case <-timer.C:
	}
}

// isFatalLine is a line that says why a Go process died.
func isFatalLine(line string) bool {
	for _, prefix := range []string{"fatal error:", "panic:", "runtime:", "capsule broker launcher:"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func clipBrokerLogLine(line string) string {
	if len(line) > brokerLogLineMax {
		return line[:brokerLogLineMax] + "..."
	}
	return line
}

func (t *brokerLogTail) addLine(line string) {
	if t == nil {
		return
	}
	line = clipBrokerLogLine(strings.TrimRight(line, "\r\n"))
	if line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > t.cap {
		// Copy down so the backing array does not grow with the stream.
		t.lines = append(t.lines[:0], t.lines[len(t.lines)-t.cap:]...)
	}
	if isFatalLine(line) && len(t.fatal) < brokerLogFatalMax {
		t.fatal = append(t.fatal, line)
	}
}

// snapshot returns copies of the pinned fatal lines and the tail.
func (t *brokerLogTail) snapshot() (fatal, lines []string) {
	if t == nil {
		return nil, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.fatal...), append([]string(nil), t.lines...)
}

// last returns the last kept line, or "".
func (t *brokerLogTail) last() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.lines) == 0 {
		return ""
	}
	return t.lines[len(t.lines)-1]
}

// copyBrokerLog reads the broker's stderr until EOF or a read error: each
// line to the tail (clipped) and every byte to echo (the journal, as
// before). It never stops early on a long line, so the pipe never fills and
// the broker never blocks writing its own stderr.
func copyBrokerLog(r io.Reader, tail *brokerLogTail, echo io.Writer) {
	if echo == nil {
		echo = io.Discard
	}
	br := bufio.NewReaderSize(r, 4096)
	var line []byte
	clipped := false
	for {
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > 0 {
			_, _ = echo.Write(chunk)
			if !clipped {
				room := brokerLogLineMax + 1 - len(line)
				if len(chunk) > room {
					line = append(line, chunk[:room]...)
					clipped = true
				} else {
					line = append(line, chunk...)
				}
			}
			if chunk[len(chunk)-1] == '\n' {
				tail.addLine(string(line))
				line, clipped = line[:0], false
			}
		}
		if err == nil || errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if len(line) > 0 {
			_, _ = echo.Write([]byte("\n"))
			tail.addLine(string(line))
		}
		return
	}
}

// CapsuleBrokerLog is one capsule broker's recent log on the observability
// route.
type CapsuleBrokerLog struct {
	CapsuleID string `json:"capsule_id"`
	// Live is true while the capsule exists.
	Live bool `json:"live"`
	// Exit is the broker's exit status when it died before its capsule's
	// destroy; empty while it runs and when the destroy stopped it.
	Exit string `json:"exit,omitempty"`
	// FirstFatal are the first lines that say why a Go process died, kept
	// even when the tail scrolled past them.
	FirstFatal []string `json:"first_fatal,omitempty"`
	Lines      []string `json:"lines"`
}

type brokerLogEntry struct {
	id   string
	tail *brokerLogTail
	exit string
}

// brokerLogRegistry holds the live capsules' broker logs and the last few
// ended ones. It has its own lock, so a read never waits on the executor's.
// The zero value is ready.
type brokerLogRegistry struct {
	mu    sync.Mutex
	live  map[string]*brokerLogEntry
	order []string // live ids, oldest first
	ended []*brokerLogEntry
}

func (r *brokerLogRegistry) register(id string) *brokerLogTail {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live == nil {
		r.live = map[string]*brokerLogEntry{}
	}
	if entry, ok := r.live[id]; ok {
		return entry.tail
	}
	entry := &brokerLogEntry{id: id, tail: newBrokerLogTail(brokerLogLines)}
	r.live[id] = entry
	r.order = append(r.order, id)
	return entry.tail
}

// tailOf is a live capsule's tail, or nil.
func (r *brokerLogRegistry) tailOf(id string) *brokerLogTail {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.live[id]; ok {
		return entry.tail
	}
	return nil
}

func (r *brokerLogRegistry) setExit(id, exit string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.live[id]; ok {
		entry.exit = exit
	}
}

// end moves a capsule's log to the ended list; exit, when not empty,
// replaces the recorded exit.
func (r *brokerLogRegistry) end(id, exit string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.live[id]
	if !ok {
		return
	}
	delete(r.live, id)
	for i, liveID := range r.order {
		if liveID == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	if exit != "" {
		entry.exit = exit
	}
	r.ended = append(r.ended, entry)
	if n := len(r.ended); n > brokerLogsEndedKept {
		r.ended = append([]*brokerLogEntry(nil), r.ended[n-brokerLogsEndedKept:]...)
	}
}

// snapshot returns the ended logs, oldest first, then the live ones.
func (r *brokerLogRegistry) snapshot() []CapsuleBrokerLog {
	r.mu.Lock()
	entries := append([]*brokerLogEntry(nil), r.ended...)
	live := len(entries)
	for _, id := range r.order {
		entries = append(entries, r.live[id])
	}
	exits := make([]string, len(entries))
	for i, entry := range entries {
		exits[i] = entry.exit
	}
	r.mu.Unlock()
	out := make([]CapsuleBrokerLog, 0, len(entries))
	for i, entry := range entries {
		fatal, lines := entry.tail.snapshot()
		out = append(out, CapsuleBrokerLog{CapsuleID: entry.id, Live: i >= live, Exit: exits[i], FirstFatal: fatal, Lines: lines})
	}
	return out
}
