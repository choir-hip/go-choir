package computerevent

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Wedge reporting. A computer whose appender keeps refusing appends
// (ErrNeedsProjectionRepair, or a Prepare that keeps failing) used to report
// /health ready, and the first error of the wedge rotated out of the ~500 KB
// guest console within minutes (docs/problems/
// paper-wedged-on-post-cas-projection-failure-2026-11-02.md, next probes 1-2).
// The state here is guarded by its own mutex, never a.mu, so a health probe
// cannot block behind a long append.
const (
	// A wedge is persistent (health degrades) only after both bounds: enough
	// consecutive refusals AND long enough since the first. One transient
	// refusal that the next append's in-place recovery heals never degrades.
	wedgePersistentRefusals = 3
	wedgePersistentAfter    = 2 * time.Minute
	// Repeated refusal lines are logged at most this often; the first
	// refusal of a wedge is always logged, once, by "wedge began".
	wedgeLogInterval = time.Minute
	wedgeErrMax      = 200
)

// WedgeStatus is the appender's refusal state. The zero value means no wedge.
// Error strings are classified and short; they carry no payload content.
type WedgeStatus struct {
	Active      bool
	Persistent  bool
	Since       time.Time
	Consecutive int
	LocalSeq    uint64
	PlatformSeq uint64
	FirstError  string
	LastError   string
}

type wedgeState struct {
	mu          sync.Mutex
	active      bool
	since       time.Time
	consecutive int
	localSeq    uint64
	platformSeq uint64
	firstError  string
	lastError   string
	lastLogged  time.Time
	// heads seen by the append in flight; read by the refusal that follows.
	curLocal, curPlatform uint64
}

// wedgeError marks an append refusal that counts toward a wedge. It keeps the
// original message and chain, and adds a short classified kind.
type wedgeError struct {
	kind string
	err  error
}

func (e *wedgeError) Error() string { return e.err.Error() }
func (e *wedgeError) Unwrap() error { return e.err }

func (a *ComputerEventAppender) wedgeClock() time.Time {
	if a.wedgeNow != nil {
		return a.wedgeNow()
	}
	return time.Now()
}

// WedgeStatus returns the current refusal state. Safe to call from a health
// handler: it takes only the wedge mutex.
func (a *ComputerEventAppender) WedgeStatus() WedgeStatus {
	if a == nil {
		return WedgeStatus{}
	}
	w := &a.wedge
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.active {
		return WedgeStatus{}
	}
	return WedgeStatus{
		Active:      true,
		Persistent:  w.consecutive >= wedgePersistentRefusals && a.wedgeClock().Sub(w.since) >= wedgePersistentAfter,
		Since:       w.since,
		Consecutive: w.consecutive,
		LocalSeq:    w.localSeq,
		PlatformSeq: w.platformSeq,
		FirstError:  w.firstError,
		LastError:   w.lastError,
	}
}

func (a *ComputerEventAppender) noteHeads(local, platform uint64) {
	a.wedge.mu.Lock()
	a.wedge.curLocal, a.wedge.curPlatform = local, platform
	a.wedge.mu.Unlock()
}

// noteAppendResult records a refusal or clears the wedge on success. Errors
// that are not wedge-class (a bad request, a stale expectation) neither start
// nor clear a wedge.
func (a *ComputerEventAppender) noteAppendResult(err error) {
	w := &a.wedge
	if err == nil {
		w.mu.Lock()
		was, since, n := w.active, w.since, w.consecutive
		w.active, w.consecutive, w.firstError, w.lastError = false, 0, "", ""
		w.mu.Unlock()
		if was {
			log.Printf("computer event appender: wedge cleared after %d refusals over %s", n, a.wedgeClock().Sub(since).Round(time.Second))
		}
		return
	}
	kind, ok := classifyWedge(err)
	if !ok {
		return
	}
	now := a.wedgeClock()
	w.mu.Lock()
	w.consecutive++
	w.lastError = kind
	w.localSeq, w.platformSeq = w.curLocal, w.curPlatform
	began := !w.active
	if began {
		w.active, w.since, w.firstError, w.lastLogged = true, now, kind, now
	}
	n, since, local, platform := w.consecutive, w.since, w.localSeq, w.platformSeq
	logContinue := !began && now.Sub(w.lastLogged) >= wedgeLogInterval
	if logContinue {
		w.lastLogged = now
	}
	w.mu.Unlock()
	if began {
		log.Printf("computer event appender: wedge began: %s local_seq=%d platform_seq=%d", kind, local, platform)
	} else if logContinue {
		log.Printf("computer event appender: wedge continues: %s consecutive=%d since=%s local_seq=%d platform_seq=%d", kind, n, since.UTC().Format(time.RFC3339), local, platform)
	}
}

// classifyWedge names a wedge-class refusal with a short, payload-free kind.
func classifyWedge(err error) (string, bool) {
	var we *wedgeError
	if errors.As(err, &we) {
		return we.kind, true
	}
	if !errors.Is(err, ErrNeedsProjectionRepair) {
		return "", false
	}
	switch {
	case errors.Is(err, ErrPayloadSQLBeforeResolve):
		return "finalize: payload resolve failed", true
	case errors.Is(err, ErrProjectionPoison):
		return "finalize: projection poison", true
	}
	return "projection repair required", true
}

// prepareKind keeps the store's constraint message (it names columns, never
// row content), whitespace-normalized and truncated, so the wedge's first
// cause survives console rotation.
func prepareKind(err error) string {
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if len(msg) > wedgeErrMax {
		msg = msg[:wedgeErrMax]
	}
	return fmt.Sprintf("prepare failed: %s", msg)
}
