package provider

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// The downscaled output budget (FP0 round 5, F7;
// docs/problems/gateway-request-bodies-unbounded-2026-10-22.md). A
// downscaled image is fresh base64 the call holds until it ends, and the
// upstream request body carries it again, so it is outside both the
// gateway's body budget and the decode slots. imageOutput bounds those
// bytes process-wide and per caller, with the body budget's grant rule:
// waiters are served oldest first, except that a waiter whose caller is
// over its share is passed over (and so are that caller's later waiters),
// and a caller's first reservation always fits within its share. A call
// reserves once (prepareMediaRequest), so no holder waits for more.

var imageOutput = newImageOutputBudget(MaxImageOutputBytes, MaxImageOutputBytesPerCaller)

var imageDecodeWaited, imageDecodeRefused atomic.Int64

type imageOutputBudget struct {
	mu       sync.Mutex
	size     int64
	share    int64
	used     int64
	peak     int64
	waited   int64
	refused  int64
	byCaller map[string]int64
	waiters  []*imageOutputWaiter
}

type imageOutputWaiter struct {
	caller  string
	n       int64
	ready   chan struct{}
	granted bool
}

func newImageOutputBudget(size, share int64) *imageOutputBudget {
	return &imageOutputBudget{size: size, share: min(share, size), byCaller: map[string]int64{}}
}

// acquire reserves n bytes (at most size) for caller, waiting until they
// fit or ctx ends.
func (b *imageOutputBudget) acquire(ctx context.Context, caller string, n int64) error {
	n = max(0, min(n, b.size))
	b.mu.Lock()
	if err := ctx.Err(); err != nil {
		b.refused++
		b.mu.Unlock()
		return fmt.Errorf("%w: %w", ErrImageDecodeWait, err)
	}
	w := &imageOutputWaiter{caller: caller, n: n, ready: make(chan struct{})}
	b.waiters = append(b.waiters, w)
	b.grantLocked()
	if w.granted {
		b.mu.Unlock()
		return nil
	}
	b.waited++
	b.mu.Unlock()
	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if w.granted {
		// Granted as the context ended: give it back.
		b.releaseLocked(caller, n)
	} else {
		for i, o := range b.waiters {
			if o == w {
				b.waiters = append(b.waiters[:i], b.waiters[i+1:]...)
				break
			}
		}
	}
	b.refused++
	b.grantLocked()
	return fmt.Errorf("%w: %w", ErrImageDecodeWait, ctx.Err())
}

func (b *imageOutputBudget) release(caller string, n int64) {
	b.mu.Lock()
	b.releaseLocked(caller, n)
	b.grantLocked()
	b.mu.Unlock()
}

func (b *imageOutputBudget) releaseLocked(caller string, n int64) {
	b.used -= n
	if left := b.byCaller[caller] - n; left > 0 {
		b.byCaller[caller] = left
	} else {
		delete(b.byCaller, caller)
	}
}

func (b *imageOutputBudget) grantLocked() {
	var overShare map[string]bool
	kept := b.waiters[:0]
	for i, w := range b.waiters {
		held := b.byCaller[w.caller]
		if overShare[w.caller] || (held > 0 && held+w.n > b.share) {
			if overShare == nil {
				overShare = map[string]bool{}
			}
			overShare[w.caller] = true
			kept = append(kept, w)
			continue
		}
		if b.used+w.n > b.size {
			kept = append(kept, b.waiters[i:]...)
			break
		}
		b.used += w.n
		b.peak = max(b.peak, b.used)
		b.byCaller[w.caller] = held + w.n
		w.granted = true
		close(w.ready)
	}
	clear(b.waiters[len(kept):])
	b.waiters = kept
}

func (b *imageOutputBudget) inUse() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

// ImageMediaStats is the decode slots and the output budget as the
// gateway's /health reports them. waited counts the waits that did not
// get their slot or bytes at once; refused the waits a call's context
// ended (answered without an upstream call, never a 400).
type ImageMediaStats struct {
	DecodeSlots          int   `json:"decode_slots"`
	DecodeSlotsPerCaller int   `json:"decode_slots_per_caller"`
	DecodeInUse          int   `json:"decode_in_use"`
	DecodeWaited         int64 `json:"decode_waited"`
	DecodeRefused        int64 `json:"decode_refused"`
	OutputLimit          int64 `json:"output_limit"`
	OutputPerCaller      int64 `json:"output_per_caller"`
	OutputInUse          int64 `json:"output_in_use"`
	OutputPeak           int64 `json:"output_peak"`
	OutputWaited         int64 `json:"output_waited"`
	OutputRefused        int64 `json:"output_refused"`
}

// ImageMediaStatus is a snapshot of the decode slots and the output
// budget.
func ImageMediaStatus() ImageMediaStats {
	b := imageOutput
	b.mu.Lock()
	defer b.mu.Unlock()
	return ImageMediaStats{
		DecodeSlots: MaxConcurrentImageDecodes, DecodeSlotsPerCaller: MaxImageDecodesPerCaller,
		DecodeInUse: len(imageDecodeSlots), DecodeWaited: imageDecodeWaited.Load(), DecodeRefused: imageDecodeRefused.Load(),
		OutputLimit: b.size, OutputPerCaller: b.share, OutputInUse: b.used, OutputPeak: b.peak,
		OutputWaited: b.waited, OutputRefused: b.refused,
	}
}
