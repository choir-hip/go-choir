package computerevent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// In-place catch-up. A running computer whose local projection is strictly
// behind the host's canonical chain, with no prepared row to explain the gap,
// used to refuse every write until a boot replayed the missing tail (the
// paper, 2026-11-02 06:20Z and 2026-11-12 15:26Z; docs/problems/
// event-appender-wedges-clustering-assessment-2026-11-12.md, section 5b). The
// host is authoritative for a linear extension of the local chain, so the
// appender fetches the missing events and applies them through the boot
// replay's own apply step (applyDurableEvent).
//
// The bounds below are deadlines on one attempt, not limits on calls: an
// attempt that stops at a bound keeps its progress and the next append resumes.
const (
	// catchUpMaxPages bounds the pages one attempt fetches. The append holds
	// a.mu for the attempt, so every writer waits for it.
	catchUpMaxPages = 16
	// catchUpTimeout bounds one attempt's wall time.
	catchUpTimeout = 2 * time.Minute
	// catchUpHeadTimeout bounds the local head reads around an attempt.
	catchUpHeadTimeout = 30 * time.Second

	wedgeKindCatchUpIncomplete = "catch-up incomplete"
)

func (a *ComputerEventAppender) catchUpPageBound() int {
	if a.catchUpPages > 0 {
		return a.catchUpPages
	}
	return catchUpMaxPages
}

func headCanonical(h *Head) string {
	if h == nil {
		return ZeroHead
	}
	return h.CanonicalEventHead
}

// catchUpLocked applies the events the host holds beyond the local head and
// returns the local head afterwards. Callers hold a.mu, and have already run
// RecoverPrepared. It refuses (keeping the wedge, changing nothing) when the
// local head is not strictly behind the host's, when it runs under a
// reconstruct (the caller's guard), and on any event that does not extend the
// local chain: it never heals a fork. An attempt that stops at its page or
// time bound returns a wedge error of kind "catch-up incomplete" with its
// progress kept.
func (a *ComputerEventAppender) catchUpLocked(ctx context.Context, local *Head) (*Head, error) {
	if a.replayProjection || a.replayActive || a.reconstructing.Load() {
		return local, nil
	}
	cctx, cancel := a.detached(ctx, catchUpTimeout)
	defer cancel()
	platform, err := a.cas.Head(cctx, a.computerID)
	if err != nil {
		return nil, fmt.Errorf("computer event appender: catch-up host head: %w", err)
	}
	if platform == nil || headSeq(local) >= platform.Sequence {
		// Equal heads need nothing; a local head at or past the host's is a
		// rewind or a fork, which no catch-up may repair.
		return local, nil
	}
	source, ok := a.cas.(EventSource)
	if !ok {
		return local, nil
	}
	paged, isPaged := a.cas.(PagedEventSource)

	started := time.Now()
	startSeq := headSeq(local)
	current := local
	after := startSeq
	applied := 0
	kinds := map[EventKind]struct{}{}
	defer func() {
		// The live append never delivered these events to the post-commit
		// observer: its caller saw an error, or the append never ran here.
		// Deliver each kind once per attempt. The observer only re-arms
		// coalesced reconciles that read committed state, so a repeat across
		// a resumed attempt is harmless; a missed delivery would not be.
		if observer := a.postCommitObserver.Load(); observer != nil {
			for kind := range kinds {
				go (*observer)(kind)
			}
		}
		if applied > 0 {
			log.Printf("computer event appender: in-place catch-up applied %d events seq %d -> %d (host %d) in %s", applied, startSeq, headSeq(current), platform.Sequence, time.Since(started).Round(time.Millisecond))
		}
	}()

	refuse := func(kind string, seq uint64, cause error) error {
		return &wedgeError{kind: kind, err: fmt.Errorf("%w: catch-up from the host at sequence %d: %w", ErrNeedsProjectionRepair, seq, cause)}
	}
	reached := func() bool { return headSeq(current) >= platform.Sequence }
	incomplete := func(why string) error {
		return refuse(wedgeKindCatchUpIncomplete, headSeq(current)+1, fmt.Errorf("%s at sequence %d of %d; the next append resumes", why, headSeq(current), platform.Sequence))
	}

	timedOut := false
	for pages := 0; pages < a.catchUpPageBound() && !reached() && !timedOut; pages++ {
		var page []DurableEvent
		var err error
		if isPaged {
			page, err = paged.EventsPage(cctx, a.computerID, after, EventReplayPageSize)
		} else {
			page, err = source.Events(cctx, a.computerID, after)
		}
		if err != nil {
			return nil, refuse("catch-up failed", after+1, fmt.Errorf("fetch durable chain: %w", err))
		}
		if len(page) == 0 {
			return nil, refuse("catch-up failed", after+1, fmt.Errorf("the host reports head %d but returned no events after %d", platform.Sequence, after))
		}
		for _, record := range page {
			if cctx.Err() != nil {
				// Out of time, or interrupted: keep the progress.
				timedOut = true
				break
			}
			event := record.Request.Event
			if event.Sequence != headSeq(current)+1 || event.PreviousHead != headCanonical(current) {
				return nil, refuse("divergent chain", event.Sequence, fmt.Errorf("event %d does not extend the local chain at %d", event.Sequence, headSeq(current)))
			}
			next, _, err := a.applyDurableEvent(cctx, current, record)
			if err != nil {
				// applyDurableEvent prepares before it finalizes; a failure
				// between them leaves a prepared row. Nothing of this event is
				// committed, and the next RecoverPrepared drops a row that
				// this discard misses.
				a.discardPreparedLocked(cctx, record.Request)
				kind := "catch-up failed"
				if errors.Is(err, ErrProjectionMismatch) {
					kind = "divergent chain"
				}
				return nil, refuse(kind, event.Sequence, err)
			}
			a.dropLivePayload(event)
			current = &next
			after = event.Sequence
			applied++
			kinds[event.EventKind] = struct{}{}
			if reached() {
				break
			}
		}
		if !isPaged {
			break
		}
	}
	if reached() && headSeq(current) == platform.Sequence && *current != *platform {
		return nil, refuse("divergent chain", platform.Sequence, fmt.Errorf("the applied chain head differs from the host's head at %d", platform.Sequence))
	}
	if !reached() {
		if timedOut {
			return nil, incomplete("stopped by its time bound or an interrupt")
		}
		return nil, incomplete("stopped by its page bound")
	}
	hctx, hcancel := a.detached(ctx, catchUpHeadTimeout)
	defer hcancel()
	return a.projection.Head(hctx, a.computerID)
}
