package guestmem

// D6 S5: the refault guard's window
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.1, 5.5).
//
// busy_cold reclaims on a computer that is working, so it must stop when what
// it frees turns out to be needed. The kernel counts that directly: a file
// page read back after its eviction is a workingset_refault_file. The guard
// reads that counter as a rate per second over the act and the 60 s before
// it, and the kernel keeps no history, so the guest samples the counter
// itself: one read of /proc/vmstat every 10 s, a ring of the last two
// minutes. The sampler holds no state anything else reads, writes nothing,
// and a crash loses only the ring (the next act then has no baseline and
// refuses, fail closed, until a minute of samples exists again).

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// RefaultSampleInterval is how often the window samples the counter.
	RefaultSampleInterval = 10 * time.Second
	// refaultLookback: the rate is over the act and at least this long
	// before it (the design's 60 s).
	refaultLookback = 60 * time.Second
	// refaultMaxBaseAge: a baseline older than this is a stalled sampler,
	// not "the 60 s before"; the guard then refuses.
	refaultMaxBaseAge = 2 * time.Minute
	// refaultActMarkMaxAge: the previous busy_cold act's end is a baseline
	// for this long. Refaults an act causes come in the seconds and minutes
	// after it; the ring's minute alone would have rolled them into its
	// baseline by the next act (review of S5, Devin #2).
	refaultActMarkMaxAge = 30 * time.Minute

	// RefaultLimitPerSecond is the refault guard's limit: workingset file
	// refaults per second, over the act and the 60 s before it, above which
	// busy_cold does not write (or stops writing). 32 pages/s is 128 KiB/s,
	// about 7.5 MiB of evicted cache read back per minute. It is set low on
	// purpose: a guest with gigabytes of MemAvailable evicts nothing on its
	// own, so its baseline is near zero and any sustained refaulting is the
	// working set coming back after an act. Conservative by design: a false
	// stop costs one act's return; a missed stop costs the paper's latency.
	// S2's refault readings or the paper's own may raise it; never the host.
	RefaultLimitPerSecond = 32.0
)

type refaultSample struct {
	at    time.Time
	count int64
}

// RefaultWindow keeps recent workingset_refault_file samples. The zero value
// is ready; ProcRoot and Now override the real guest for tests.
type RefaultWindow struct {
	ProcRoot string
	Now      func() time.Time

	mu      sync.Mutex
	samples []refaultSample
	// actEnd is the end of the last busy_cold act that wrote (the counter
	// then); zero when none.
	actEnd refaultSample
}

// MarkActEnd records the end of a busy_cold act that wrote: the next act's
// guard reads its rate from here (within 30 minutes), so refaults this act
// causes are seen by the next one.
func (w *RefaultWindow) MarkActEnd(at time.Time, count int64) {
	w.mu.Lock()
	w.actEnd = refaultSample{at, count}
	w.mu.Unlock()
}

// ActEnd is the last act end marked.
func (w *RefaultWindow) ActEnd() (time.Time, int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.actEnd.at, w.actEnd.count, !w.actEnd.at.IsZero()
}

// PostRate is the refaults per second from the last act's end to current,
// read now: the post-act reading the guest logs as refault_rate_post.
func (w *RefaultWindow) PostRate(current int64) (float64, bool) {
	at, count, ok := w.ActEnd()
	if !ok {
		return 0, false
	}
	age := w.now().Sub(at)
	if age <= 0 || current < count {
		return 0, false
	}
	return float64(current-count) / age.Seconds(), true
}

func (w *RefaultWindow) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// Run samples every interval until ctx ends. One sample at the start, so a
// baseline exists a minute after boot.
func (w *RefaultWindow) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = RefaultSampleInterval
	}
	w.Sample()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Sample()
		}
	}
}

// Sample reads the counter once. A vmstat without it adds nothing.
func (w *RefaultWindow) Sample() {
	root := w.ProcRoot
	if root == "" {
		root = "/proc"
	}
	raw, err := os.ReadFile(filepath.Join(root, "vmstat"))
	if err != nil {
		return
	}
	if v := statFields(string(raw))["workingset_refault_file"]; v != nil {
		w.Observe(w.now(), *v)
	}
}

// VmstatField reads one counter from /proc/vmstat text; nil when absent.
func VmstatField(vmstat, name string) *int64 {
	return statFields(vmstat)[name]
}

// Observe records one reading and prunes what the guard can no longer use.
func (w *RefaultWindow) Observe(at time.Time, count int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.samples = append(w.samples, refaultSample{at, count})
	cut := 0
	for cut < len(w.samples) && at.Sub(w.samples[cut].at) > refaultMaxBaseAge+RefaultSampleInterval {
		cut++
	}
	if cut > 0 {
		w.samples = append([]refaultSample(nil), w.samples[cut:]...)
	}
}

func (w *RefaultWindow) sampleCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.samples)
}

// Rate is the refaults per second to current, read now, from the older of
// two baselines: the newest ring sample at least 60 s old, and the last
// act's end when it is 60 s to 30 minutes old (so the window covers every
// refault since the previous act). ok is false, and the guard refuses, when
// the ring has no such sample (after boot, or a sampler stalled past two
// minutes: a mark alone is never enough), or when the counter went
// backwards.
func (w *RefaultWindow) Rate(current int64) (float64, bool) {
	if w == nil {
		return 0, false
	}
	now := w.now()
	w.mu.Lock()
	defer w.mu.Unlock()
	var base refaultSample
	found := false
	for i := len(w.samples) - 1; i >= 0; i-- {
		s := w.samples[i]
		age := now.Sub(s.at)
		if age < refaultLookback {
			continue
		}
		if age > refaultMaxBaseAge {
			return 0, false
		}
		base, found = s, true
		break
	}
	if !found {
		return 0, false
	}
	if mark := w.actEnd; !mark.at.IsZero() {
		age := now.Sub(mark.at)
		if age >= refaultLookback && age <= refaultActMarkMaxAge && mark.at.Before(base.at) {
			base = mark
		}
	}
	if current < base.count {
		return 0, false
	}
	return float64(current-base.count) / now.Sub(base.at).Seconds(), true
}
