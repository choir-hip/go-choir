package gateway

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// callTallySlack is how long after the hour boundary the timer fires, so the
// clock has certainly crossed it.
const callTallySlack = time.Second

// callTally counts the calls each computer makes through the gateway, by
// provider, and writes one line per computer and provider each hour. It is
// visibility, never a limit: it refuses nothing and delays nothing (owner,
// 2026-11-10: too many model calls is a code problem to fix, not a limit to
// impose; docs/problems/gateway-inference-bucket-self-imposed-2026-11-10.md).
//
// The zero value is ready to use. A closed hour is logged by the first call
// after it or by the single timer started with start, whichever comes first;
// the hour's counts are taken out under the lock exactly once, so it is
// logged once. Lines are written after the lock is released.
type callTally struct {
	mu     sync.Mutex
	hour   time.Time
	counts map[callTallyKey]*callTallyCount

	// Tests only; set before use. nil means the real clock, the real timer
	// and log.Printf.
	now   func() time.Time
	after func(time.Duration) <-chan time.Time
	logf  func(string, ...any)
}

type callTallyKey struct{ computerID, provider string }

type callTallyCount struct {
	calls       int
	inputTokens int64
}

// Call records one admitted call by computerID to provider ("search" and
// "source_api" name those routes). A call that fails stays with the provider
// it was admitted to.
func (t *callTally) Call(computerID, provider string) {
	t.update(func(m map[callTallyKey]*callTallyCount) {
		count(m, callTallyKey{computerID, provider}).calls++
	})
}

// Served records a finished call's whole prompt under the provider that
// served it. When a fallback served the call instead of the provider it was
// admitted to, the call moves with its tokens, so one label carries both.
func (t *callTally) Served(computerID, admitted, served string, inputTokens int) {
	if served == "" {
		served = admitted
	}
	t.update(func(m map[callTallyKey]*callTallyCount) {
		if served != admitted {
			if c := m[callTallyKey{computerID, admitted}]; c != nil && c.calls > 0 {
				c.calls--
			}
			count(m, callTallyKey{computerID, served}).calls++
		}
		count(m, callTallyKey{computerID, served}).inputTokens += int64(inputTokens)
	})
}

func count(m map[callTallyKey]*callTallyCount, k callTallyKey) *callTallyCount {
	c := m[k]
	if c == nil {
		c = &callTallyCount{}
		m[k] = c
	}
	return c
}

func (t *callTally) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *callTally) update(f func(map[callTallyKey]*callTallyCount)) {
	if t == nil {
		return
	}
	hour := t.clock().UTC().Truncate(time.Hour)
	t.mu.Lock()
	lines := t.takeClosedLocked(hour)
	if t.counts == nil {
		t.hour, t.counts = hour, map[callTallyKey]*callTallyCount{}
	}
	f(t.counts)
	t.mu.Unlock()
	t.write(lines)
}

// flushClosed logs the open hour if it has ended. The timer calls it.
func (t *callTally) flushClosed() {
	hour := t.clock().UTC().Truncate(time.Hour)
	t.mu.Lock()
	lines := t.takeClosedLocked(hour)
	t.mu.Unlock()
	t.write(lines)
}

// takeClosedLocked removes and formats the open hour's counts when that hour
// is not the current one. Taking them out is what makes a second flush of the
// same hour (timer and a call, or two ticks) a no-op.
func (t *callTally) takeClosedLocked(current time.Time) []string {
	if t.counts == nil || t.hour.Equal(current) {
		return nil
	}
	keys := make([]callTallyKey, 0, len(t.counts))
	for k := range t.counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].computerID != keys[j].computerID {
			return keys[i].computerID < keys[j].computerID
		}
		return keys[i].provider < keys[j].provider
	})
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		c := t.counts[k]
		lines = append(lines, fmt.Sprintf("gateway: calls hour=%s computer=%s provider=%s calls=%d input_tokens=%d",
			t.hour.Format("2006-01-02T15:04Z"), k.computerID, strings.ReplaceAll(k.provider, " ", "_"), c.calls, c.inputTokens))
	}
	t.counts = nil
	return lines
}

func (t *callTally) write(lines []string) {
	logf := t.logf
	if logf == nil {
		logf = log.Printf
	}
	for _, l := range lines {
		logf("%s", l)
	}
}

// start runs the tally's single timer: once an hour, just after the boundary,
// it logs the hour that closed. One goroutine for the process, none per call.
// stop ends it and waits for it; the partial hour then open is dropped, not
// logged (a restart loses at most the last partial hour of counts).
func (t *callTally) start() (stop func()) {
	if t == nil {
		return func() {}
	}
	done, exited := make(chan struct{}), make(chan struct{})
	after := t.after
	if after == nil {
		after = time.After
	}
	go func() {
		defer close(exited)
		for {
			next := t.clock().UTC().Truncate(time.Hour).Add(time.Hour).Add(callTallySlack)
			select {
			case <-after(next.Sub(t.clock())):
				t.flushClosed()
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-exited
	}
}
