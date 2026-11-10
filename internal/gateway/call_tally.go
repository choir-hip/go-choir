package gateway

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// callTally counts the calls each computer makes through the gateway, by
// provider, and writes one line per computer and provider each hour. It is
// visibility, never a limit: it refuses nothing and delays nothing (owner,
// 2026-11-10: too many model calls is a code problem to fix, not a limit to
// impose; docs/problems/gateway-inference-bucket-self-imposed-2026-11-10.md).
// The zero value is ready to use. The window closes lazily, on the first
// call after the hour ends, so there is no goroutine to leak.
type callTally struct {
	mu     sync.Mutex
	hour   time.Time
	counts map[callTallyKey]*callTallyCount
	now    func() time.Time     // tests only; nil means time.Now
	logf   func(string, ...any) // tests only; nil means log.Printf
}

type callTallyKey struct{ computerID, provider string }

type callTallyCount struct {
	calls       int
	inputTokens int64
}

// Call records one admitted call by computerID to provider ("search" and
// "source_api" name those routes).
func (t *callTally) Call(computerID, provider string) { t.add(computerID, provider, 1, 0) }

// InputTokens adds a finished call's whole prompt to the computer's hour.
func (t *callTally) InputTokens(computerID, provider string, n int) {
	t.add(computerID, provider, 0, int64(n))
}

func (t *callTally) add(computerID, provider string, calls int, tokens int64) {
	if t == nil {
		return
	}
	now := time.Now()
	if t.now != nil {
		now = t.now()
	}
	hour := now.UTC().Truncate(time.Hour)
	t.mu.Lock()
	var lines []string
	if t.counts == nil || !hour.Equal(t.hour) {
		lines = t.flushLocked()
		t.hour, t.counts = hour, map[callTallyKey]*callTallyCount{}
	}
	k := callTallyKey{computerID, provider}
	c := t.counts[k]
	if c == nil {
		c = &callTallyCount{}
		t.counts[k] = c
	}
	c.calls += calls
	c.inputTokens += tokens
	logf := t.logf
	t.mu.Unlock()
	if logf == nil {
		logf = log.Printf
	}
	for _, l := range lines {
		logf("%s", l)
	}
}

func (t *callTally) flushLocked() []string {
	var keys []callTallyKey
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
	return lines
}
