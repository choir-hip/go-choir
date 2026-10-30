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
