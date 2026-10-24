package agentcore

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// U1b (docs/design/computer-update-architecture-rnd-2026-10-24.md, "U1b
// design"): at boot, and when the owner turns the projection on, the guest
// fetches the booted base's objects if its source repository lacks them, then
// reconciles so an empty stack moves to that commit. Behind the same switch as
// U1 (sourceproj.Enabled); with it off none of this runs.

// sourceBaseFetchTuning is the pacing of the fetch; tests replace it.
type sourceBaseFetchTuning struct {
	// Jitter is the most a first network attempt is delayed, so computers that
	// boot together after a deploy do not open together (N9).
	Jitter time.Duration
	// Backoff is the wait after each failed attempt that may pass (N1); the
	// attempts end when it runs out.
	Backoff []time.Duration
	// GateWait is the most the cast gate waits for an attempt in flight.
	GateWait time.Duration
	// Options overrides the fetch options (tests).
	Options func() sourceproj.FetchOptions
}

func defaultSourceBaseFetchTuning() sourceBaseFetchTuning {
	return sourceBaseFetchTuning{
		Jitter:   20 * time.Second,
		Backoff:  []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour},
		GateWait: 45 * time.Second,
	}
}

func (rt *Runtime) sourceBaseFetchSettings() sourceBaseFetchTuning {
	if rt.sourceBaseFetchTune != nil {
		return *rt.sourceBaseFetchTune
	}
	return defaultSourceBaseFetchTuning()
}

// startSourceProjection is the boot and setting trigger: reconcile as U1 does
// (it moves HEAD when the objects are already here), then make sure the
// booted base's objects are, and reconcile again if they had to be fetched.
func (rt *Runtime) startSourceProjection(ctx context.Context, reason string) {
	if rt == nil || !sourceproj.Enabled() {
		return
	}
	rt.reconcileSourceProjectionLogged(ctx, reason)
	rt.ensureSourceBaseObjects(ctx)
}

// ensureSourceBaseObjects runs the fetch sequence once per process at a time.
func (rt *Runtime) ensureSourceBaseObjects(ctx context.Context) {
	booted, repo := rt.bootedBaseCommit(), rt.releaseRepo()
	if booted == "" || repo == "" || sourceproj.HasCommit(ctx, repo, booted) {
		return // nothing to fetch, and no network is used (the common case)
	}
	rt.sourceBaseFetchMu.Lock()
	if rt.sourceBaseFetchRunning {
		rt.sourceBaseFetchMu.Unlock()
		return
	}
	rt.sourceBaseFetchRunning = true
	rt.sourceBaseFetchMu.Unlock()
	defer func() {
		rt.sourceBaseFetchMu.Lock()
		rt.sourceBaseFetchRunning = false
		rt.sourceBaseFetchMu.Unlock()
	}()

	tune := rt.sourceBaseFetchSettings()
	options := sourceproj.DefaultFetchOptions
	if tune.Options != nil {
		options = tune.Options
	}
	for attempt := 0; ; attempt++ {
		done := rt.beginSourceBaseAttempt()
		if attempt == 0 && tune.Jitter > 0 {
			if !sleepBaseFetch(ctx, time.Duration(rand.Int63n(int64(tune.Jitter)))) {
				rt.endSourceBaseAttempt(done)
				return
			}
		}
		result := sourceproj.EnsureBaseObjects(ctx, repo, booted, options())
		rt.endSourceBaseAttempt(done)
		rt.recordSourceBaseFetch(result, attempt)
		if result.Outcome == sourceproj.FetchFetched {
			rt.reconcileSourceProjectionLogged(ctx, "base_fetch")
			return
		}
		if result.Done() || attempt >= len(tune.Backoff) || !sleepBaseFetch(ctx, tune.Backoff[attempt]) {
			// The last word on /health: the state now says why.
			if result.Outcome != sourceproj.FetchDisabled {
				rt.reconcileSourceProjectionLogged(ctx, "base_fetch")
			}
			return
		}
	}
}

func sleepBaseFetch(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (rt *Runtime) beginSourceBaseAttempt() chan struct{} {
	done := make(chan struct{})
	rt.sourceBaseFetchMu.Lock()
	rt.sourceBaseFetchInflight = done
	rt.sourceBaseFetchMu.Unlock()
	return done
}

func (rt *Runtime) endSourceBaseAttempt(done chan struct{}) {
	rt.sourceBaseFetchMu.Lock()
	if rt.sourceBaseFetchInflight == done {
		rt.sourceBaseFetchInflight = nil
	}
	rt.sourceBaseFetchMu.Unlock()
	close(done)
}

// waitSourceBaseFetch lets the cast gate wait, bounded, for an attempt that is
// running now. It starts none, and does not wait through a back-off (5).
func (rt *Runtime) waitSourceBaseFetch(ctx context.Context) {
	if rt == nil || !sourceproj.Enabled() {
		return
	}
	rt.sourceBaseFetchMu.Lock()
	inflight := rt.sourceBaseFetchInflight
	rt.sourceBaseFetchMu.Unlock()
	if inflight == nil {
		return
	}
	wait := rt.sourceBaseFetchSettings().GateWait
	if wait <= 0 {
		return
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-inflight:
	case <-timer.C:
	case <-ctx.Done():
	}
}

// recordSourceBaseFetch keeps the last attempt for /health's reason, logs when
// the outcome changes, and writes a fact for a fetch and for a failure.
func (rt *Runtime) recordSourceBaseFetch(result sourceproj.FetchResult, attempt int) {
	rt.sourceBaseFetchMu.Lock()
	previous := rt.sourceBaseFetchLast
	copied := result
	rt.sourceBaseFetchLast = &copied
	rt.sourceBaseFetchMu.Unlock()
	if previous == nil || previous.Outcome != result.Outcome || previous.Reason != result.Reason {
		log.Printf("source base fetch (attempt %d): %s commit=%s remote=%s in %s %s", attempt+1, result.Outcome, shortDigest(result.Commit), result.Remote, result.Duration, result.Reason)
	}
	var kind string
	switch result.Outcome {
	case sourceproj.FetchFetched:
		kind = sourceproj.FactBaseFetched
	case sourceproj.FetchFailed, sourceproj.FetchPermanent:
		if previous != nil && previous.Outcome == result.Outcome && previous.Reason == result.Reason {
			return // one fact per distinct failure, not one per retry
		}
		kind = sourceproj.FactBaseFetchFailed
	default:
		return
	}
	rt.appendSourceProjectionFactRecord(sourceProjectionFact{
		At: time.Now().UTC().Format(time.RFC3339Nano), Kind: kind, Trigger: "base_fetch", State: string(result.Outcome), Reason: result.Reason,
		BootedBase: result.Commit,
	})
}

// withBaseFetchReason adds the last failed fetch to a result that is short of
// the booted base's objects, so /health and the cast say why.
func (rt *Runtime) withBaseFetchReason(result sourceproj.Result) string {
	if result.State != sourceproj.StateBaseObjectsMissing && !(result.State == sourceproj.StateBaseMoved && strings.Contains(result.Reason, "not in the guest source repository")) {
		return result.Reason
	}
	rt.sourceBaseFetchMu.Lock()
	last := rt.sourceBaseFetchLast
	rt.sourceBaseFetchMu.Unlock()
	if last == nil || last.Commit != result.BootedBase {
		return result.Reason
	}
	switch last.Outcome {
	case sourceproj.FetchFailed, sourceproj.FetchPermanent, sourceproj.FetchSkipped:
		return result.Reason + "; the fetch of it " + string(last.Outcome) + ": " + last.Reason
	}
	return result.Reason
}
