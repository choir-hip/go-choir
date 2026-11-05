package yaegikernel

import (
	"fmt"
	"sync"
	"time"
)

// Restart intensity (MA6.2): a desk whose workers keep ending on their own
// (a crash, the kernel's OOM kill, a refused syscall that kills the process)
// is not given a new worker for every activation that asks. After
// crashLoopMax such endings within crashLoopWindow the spawn is refused until
// the oldest ages out, with the cause named, so a hostile or broken cell
// cannot turn the spawner into a fork bomb with a model behind it. An ending
// the host caused (its kill on a timeout, a graceful close) is not a crash.
// Nothing here resumes work: the refused activation fails visibly, as a
// crash restart closes work (CLAUDE.md, Restarts End Work).
const (
	crashLoopMax    = 5
	crashLoopWindow = time.Minute
)

type crashLedger struct {
	mu     sync.Mutex
	now    func() time.Time
	byRole map[string][]time.Time
	last   map[string]string
}

var workerCrashes = &crashLedger{now: time.Now, byRole: map[string][]time.Time{}, last: map[string]string{}}

func (l *crashLedger) record(role, cause string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.byRole[role] = append(l.prune(role), l.now())
	l.last[role] = cause
}

func (l *crashLedger) prune(role string) []time.Time {
	cut := l.now().Add(-crashLoopWindow)
	kept := l.byRole[role][:0:0]
	for _, t := range l.byRole[role] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	l.byRole[role] = kept
	return kept
}

// admit returns an error when role is in a crash loop.
func (l *crashLedger) admit(role string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.prune(role)
	if len(kept) < crashLoopMax {
		return nil
	}
	retry := kept[0].Add(crashLoopWindow).Sub(l.now()).Round(time.Second)
	return fmt.Errorf("desk worker crash loop: %d %s workers ended on their own within %s (last: %s); no new worker for %s", len(kept), role, crashLoopWindow, l.last[role], retry)
}

func (l *crashLedger) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.byRole = map[string][]time.Time{}
	l.last = map[string]string{}
}
