package agentcore

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Per-desk resource bounds for package desks (MA3.9). They bound what a desk
// does to the computer's own storage and write path, and how often it wakes.
// They are not a spend ceiling: model cost is attributed per desk and watched
// (llmcost, /api/costs), never capped (R8).

// ErrDeskQuota refuses a package desk's write past its object or storage
// quota, or past its write rate.
var ErrDeskQuota = errors.New("desk quota exceeded")

// ErrDeskWakeRate refuses a run of a package desk past its wake rate.
var ErrDeskWakeRate = errors.New("desk wake rate exceeded")

// deskLimits are one computer's per-desk bounds. A zero field takes the
// default, so the zero value is the shipped limits.
type deskLimits struct {
	// Objects is the most live objects one package desk keeps for the owner,
	// across its kinds.
	Objects int
	// Bytes is the most body bytes those objects hold.
	Bytes int64
	// WritesPerHour is the most Put and Retract a desk commits in a rolling
	// hour.
	WritesPerHour int
	// WakesPerHour is the most runs a package desk starts in a rolling hour.
	WakesPerHour int
}

const (
	defaultDeskObjects       = 10000
	defaultDeskBytes   int64 = 64 << 20
	defaultDeskWrites        = 600
)

func (l deskLimits) withDefaults() deskLimits {
	if l.Objects <= 0 {
		l.Objects = defaultDeskObjects
	}
	if l.Bytes <= 0 {
		l.Bytes = defaultDeskBytes
	}
	if l.WritesPerHour <= 0 {
		l.WritesPerHour = defaultDeskWrites
	}
	if l.WakesPerHour <= 0 {
		l.WakesPerHour = deskmanifest.MaxWakesPerHour
	}
	return l
}

func (rt *Runtime) limitsForDesks() deskLimits {
	if rt == nil {
		return deskLimits{}.withDefaults()
	}
	return rt.deskLimits.withDefaults()
}

// deskRateWindow counts events per key in a rolling hour. The zero value is
// ready. It lives in memory: a restart forgets it, which only lets a desk
// that was at its bound go on for one more window.
type deskRateWindow struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

const deskRateWindowSpan = time.Hour

// take records n events under key at now, unless that puts the key past limit
// in the rolling hour; then it records nothing and reports the count held.
func (w *deskRateWindow) take(key string, n, limit int, now time.Time) (held int, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.events == nil {
		w.events = map[string][]time.Time{}
	}
	cut := now.Add(-deskRateWindowSpan)
	kept := w.events[key][:0]
	for _, at := range w.events[key] {
		if at.After(cut) {
			kept = append(kept, at)
		}
	}
	if len(kept)+n > limit {
		w.events[key] = kept
		return len(kept), false
	}
	for i := 0; i < n; i++ {
		kept = append(kept, now)
	}
	w.events[key] = kept
	return len(kept), true
}

// admitDeskWake counts one run of an app desk against its wake rate. A
// built-in desk, which is no package's, is never counted.
func (rt *Runtime) admitDeskWake(profile string, now time.Time) error {
	if _, ok := appDeskPackage(profile); !ok {
		return nil
	}
	limit := rt.limitsForDesks().WakesPerHour
	if held, ok := rt.deskRates.take("wake:"+canonicalDeskName(profile), 1, limit, now); !ok {
		return fmt.Errorf("%w: desk %s started %d runs in the last hour (at most %d)", ErrDeskWakeRate, profile, held, limit)
	}
	return nil
}
