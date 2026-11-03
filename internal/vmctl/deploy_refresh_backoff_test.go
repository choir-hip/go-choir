package vmctl

import (
	"context"
	"testing"
	"time"
)

// A computer the guest keeps refusing (d@e's held rollback, 2026-10-30) was
// asked every minute for ever, and nothing said it was stuck (review of the
// cutover, F6/F12). Ways the backoff could fail:
//  1. a computer that is merely busy for a few minutes waits longer than the
//     one-minute sweep before it updates;
//  2. a long-refused computer is still asked every minute;
//  3. a long-refused computer is never asked again, so the owner act that
//     clears its refusal never leads to an update;
//  4. the gap grows without a ceiling.
func TestDeployRefreshRetryBacksOffALongRefusedComputer(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)
	entry := func(attempts int, ago time.Duration) deployRefreshPendingEntry {
		return deployRefreshPendingEntry{Attempts: attempts, LastAttemptAt: now.Add(-ago)}
	}
	if !deployRefreshRetryDue(entry(5, time.Minute), now) {
		t.Fatal("a computer busy for five minutes is not asked at the next sweep")
	}
	if deployRefreshRetryDue(entry(deployRefreshRetryBackoffAfter+3, time.Minute), now) {
		t.Fatal("a long-refused computer is still asked every minute")
	}
	if !deployRefreshRetryDue(entry(deployRefreshRetryBackoffAfter+3, 20*time.Minute), now) {
		t.Fatal("a long-refused computer is not asked after its gap")
	}
	if !deployRefreshRetryDue(entry(deployRefreshRetryBackoffAfter+500, deployRefreshRetryMaxGap), now) {
		t.Fatal("the gap grew past its ceiling")
	}

	h, _, fake := newDeployRefreshFixture(t, "refused")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "0ld" }
	fake.outcome = guestMarkBusy
	if _, resp := postDeployRefresh(t, h, `{"user_id":"refused","desktop_id":"primary","target":"761d94c5"}`); resp.Outcome != deployRefreshDeferred {
		t.Fatalf("setup: %+v, want deferred", resp)
	}
	tr := h.deployRefreshState()
	tr.mu.Lock()
	for k, e := range tr.pending {
		e.Attempts = deployRefreshRetryBackoffAfter + 3
		tr.pending[k] = e
	}
	tr.mu.Unlock()
	marks := len(fake.marks)
	h.SweepDeployRefreshPending(context.Background())
	if len(fake.marks) != marks {
		t.Fatal("the sweep asked a long-refused computer again one minute later")
	}
	tr.mu.Lock()
	for k, e := range tr.pending {
		e.LastAttemptAt = e.LastAttemptAt.Add(-2 * deployRefreshRetryMaxGap)
		tr.pending[k] = e
	}
	tr.mu.Unlock()
	h.SweepDeployRefreshPending(context.Background())
	if len(fake.marks) != marks+1 {
		t.Fatal("the sweep never asked the long-refused computer again")
	}
}

// Problem 27: the backoff counts refusals of one target. A new release
// re-targeting an entry restarts it; the same target keeps it. Ways it could
// fail: (1) a new deploy inherits the old release's attempts and waits up to
// an hour; (2) the same target loses its backoff and is asked every minute;
// (3) the ring walk's ensure of a woke_behind entry carries attempts over.
func TestDeployRefreshRetargetResetsBackoffSameTargetKeepsIt(t *testing.T) {
	h, _, fake := newDeployRefreshFixture(t, "busyarm")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "0ld" }
	fake.outcome = guestMarkBusy
	post := func(target string) {
		if _, resp := postDeployRefresh(t, h, `{"user_id":"busyarm","desktop_id":"primary","target":"`+target+`"}`); resp.Outcome != deployRefreshDeferred {
			t.Fatalf("setup: %+v, want deferred", resp)
		}
	}
	post("rel-1")
	tr := h.deployRefreshState()
	tr.mu.Lock()
	for k, e := range tr.pending {
		e.Attempts = 40
		e.FirstDeferredAt = e.FirstDeferredAt.Add(-5 * time.Hour)
		tr.pending[k] = e
	}
	tr.mu.Unlock()

	post("rel-1")
	got := tr.list()[0]
	if got.Attempts != 41 || time.Since(got.FirstDeferredAt) < 4*time.Hour {
		t.Fatalf("same target lost its backoff: %+v", got)
	}
	if deployRefreshRetryDue(got, got.LastAttemptAt.Add(time.Minute)) {
		t.Fatal("same target is asked every minute")
	}

	post("rel-2")
	got = tr.list()[0]
	if got.Target != "rel-2" || got.Attempts != 1 || time.Since(got.FirstDeferredAt) > time.Minute {
		t.Fatalf("re-target kept the old release's count: %+v", got)
	}
	if !deployRefreshRetryDue(got, got.LastAttemptAt.Add(time.Minute)) {
		t.Fatal("a fresh release waits out the old release's backoff")
	}
}

func TestRingEnsureResetsBackoffOnRetarget(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-5 * time.Hour)
	for _, kind := range []string{"", deployRefreshKindWokeBehind} {
		tr := newDeployRefreshTracker()
		tr.enroll(deployRefreshPendingEntry{UserID: "u", DesktopID: "primary", Target: "r1", Kind: kind, FirstDeferredAt: old, LastAttemptAt: now, Attempts: 37})
		tr.ensure(deployRefreshPendingEntry{UserID: "u", DesktopID: "primary", Target: "r2", FirstDeferredAt: now})
		got := tr.list()[0]
		if got.Target != "r2" || got.Attempts != 0 || got.FirstDeferredAt.Before(now) || time.Since(got.FirstDeferredAt) > time.Minute {
			t.Fatalf("kind %q: ensure kept the old count: %+v", kind, got)
		}
		if !deployRefreshRetryDue(got, now.Add(time.Minute)) {
			t.Fatalf("kind %q: re-targeted entry not due", kind)
		}
		tr.ensure(deployRefreshPendingEntry{UserID: "u", DesktopID: "primary", Target: "r2", FirstDeferredAt: now.Add(time.Hour)})
		if g := tr.list()[0]; !g.FirstDeferredAt.Equal(got.FirstDeferredAt) {
			t.Fatalf("same-target ensure reset the entry: %+v", g)
		}
	}
}
