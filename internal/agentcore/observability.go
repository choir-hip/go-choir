package agentcore

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// bootLogRing is a bounded, thread-safe ring of recent process log lines.
// It lets boot/reconcile outcomes be served through the product API instead
// of requiring shell access to the guest (standing question #9).
type bootLogRing struct {
	mu    sync.Mutex
	cap   int
	lines []string
}

func newBootLogRing(cap int) *bootLogRing {
	if cap <= 0 {
		cap = 512
	}
	return &bootLogRing{cap: cap}
}

// Write implements io.Writer for log.SetOutput. The standard logger emits one
// Write per Printf call, but a Write may still carry multiple lines.
func (r *bootLogRing) Write(p []byte) (int, error) {
	if r == nil {
		return len(p), nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		r.lines = append(r.lines, line)
		if len(r.lines) > r.cap {
			r.lines = r.lines[len(r.lines)-r.cap:]
		}
	}
	return len(p), nil
}

func (r *bootLogRing) snapshot() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// CaptureBootLog installs a bounded ring on the runtime and redirects the
// standard logger into stderr plus the ring. It must be called before Start so
// boot recovery, assignment reconciliation, and sweep outcomes are captured.
func (rt *Runtime) CaptureBootLog(cap int) {
	if rt == nil {
		return
	}
	ring := newBootLogRing(cap)
	rt.bootLog = ring
	log.SetOutput(io.MultiWriter(os.Stderr, ring))
}

// RecentBootLog returns a copy of the most recent captured log lines.
func (rt *Runtime) RecentBootLog() []string {
	if rt == nil || rt.bootLog == nil {
		return nil
	}
	return rt.bootLog.snapshot()
}
