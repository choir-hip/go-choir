package maild

// Failure modes this file guards: the scheduled digest must retry inside its
// slot hour. On 2026-10-30 the 12:00 UTC run found corpusd down, recorded
// skipped/front_unavailable and nothing ever tried again. Written before the
// retry code.
//
//	R1  Double send. Retries (a timer firing every 10 minutes from 12:00 to
//	    12:50) must send at most once per owner and slot: a sent row is final,
//	    and the Resend Idempotency-Key is the same on every attempt.
//	R2  Retry storm. Only a transient end (front_unavailable, too_few, failed,
//	    interrupted) is retried; a deliberate skip (no_address)
//	    is final for the slot. The retries are bounded to the slot
//	    hour: a failed row is not reopened at 13:00 or later, and nothing loops
//	    inside the process (the timer is the clock, six fires at most).
//	R3  Retry after a later slot already sent. Slots are keyed by UTC date. A
//	    failed row of day D is never reopened on day D+1, and D+1's own run is
//	    a new slot that sends its own mail; D's closed slot sends nothing.
//	R4  Crash mid-send counted wrong. A started row left by a crash is closed
//	    "interrupted by a restart" at boot (never resumed). Inside the slot
//	    hour the next timer fire is new work and may reopen it; the Resend key
//	    is the same, so a send that did land is deduplicated by the provider
//	    and one that did not goes out. Outside the hour it stays interrupted.
//	R5  Restart during the retry window. Same as R4: boot never sends; only
//	    the next scheduled fire inside the hour does, and only when the slot
//	    is still unsent.
//	R6  The final outcome is invisible. The last row for the slot carries the
//	    final status and reason (sent, or the transient reason of the last
//	    attempt) for the Mail app status and the journal.
//	R7  The host timer fires once only. It must list 12:00 through 12:50 UTC
//	    in 10-minute steps and stay Persistent = false.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDigestRetryAfterFrontOutageSendsOnceInTheHour(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.front.status = http.StatusInternalServerError
	d.run() // 12:00, paper down
	d.setNow(digestTestNow.Add(10 * time.Minute))
	d.run() // 12:10, still down
	if d.resend.calls() != 0 {
		t.Fatalf("sent while the front list was down")
	}
	d.front.status = http.StatusOK
	d.setNow(digestTestNow.Add(20 * time.Minute))
	d.run() // 12:20 sends
	for i := 3; i <= 5; i++ {
		d.setNow(digestTestNow.Add(time.Duration(i*10) * time.Minute))
		reply := decodeRunReply(t, d.run())
		if len(reply.Outcomes) != 1 || reply.Outcomes[0].Reason != "already_ran" {
			t.Fatalf("fire %d outcomes = %+v", i, reply.Outcomes)
		}
	}
	if d.resend.calls() != 1 {
		t.Fatalf("resend calls = %d, want exactly 1 (R1)", d.resend.calls())
	}
	if run := d.lastRun("scheduled"); run.Status != "sent" || run.ProviderMessageID == "" {
		t.Fatalf("final row = %+v (R6)", run)
	}
}

func TestDigestRetryFinalOutcomeIsRecordedWhenTheHourEndsDown(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.front.status = http.StatusInternalServerError
	for i := 0; i <= 5; i++ {
		d.setNow(digestTestNow.Add(time.Duration(i*10) * time.Minute))
		d.run()
	}
	if got := d.front.hitCount(); got != 6 {
		t.Fatalf("front hits = %d, want 6 (bounded, R2)", got)
	}
	d.setNow(digestTestNow.Add(60 * time.Minute)) // 13:00, slot closed
	reply := decodeRunReply(t, d.run())
	if len(reply.Outcomes) != 1 || reply.Outcomes[0].Reason != "slot_closed" {
		t.Fatalf("13:00 outcomes = %+v, want slot_closed (R2)", reply.Outcomes)
	}
	if d.resend.calls() != 0 {
		t.Fatalf("a closed slot sent mail")
	}
	if run := d.lastRun("scheduled"); run.Status != "skipped" || run.Reason != "front_unavailable" {
		t.Fatalf("final row = %+v (R6)", run)
	}
}

func TestDigestRetryProviderFailureIsRetriedWithTheSameKey(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.resend.statuses = []int{500, 500} // first fire: both attempts fail
	if w := d.run(); w.Code != http.StatusBadGateway {
		t.Fatalf("run = %d, want 502", w.Code)
	}
	d.setNow(digestTestNow.Add(10 * time.Minute))
	d.run()
	if d.resend.calls() != 3 {
		t.Fatalf("resend calls = %d, want 3", d.resend.calls())
	}
	if d.resend.keys[0] != d.resend.keys[2] {
		t.Fatalf("idempotency key changed across fires: %v (R1)", d.resend.keys)
	}
	if run := d.lastRun("scheduled"); run.Status != "sent" {
		t.Fatalf("final row = %+v", run)
	}
}

func TestDigestRetryDeliberateSkipIsFinal(t *testing.T) {
	d := newDigestHarness(t)
	if err := d.store.SetDigestSubscription(context.Background(), gateTestOwner, true, d.h.digestNow()); err != nil {
		t.Fatal(err)
	}
	d.run() // no address of record: decided for the slot
	d.setNow(digestTestNow.Add(10 * time.Minute))
	reply := decodeRunReply(t, d.run())
	if len(reply.Outcomes) != 1 || reply.Outcomes[0].Reason != "already_ran" {
		t.Fatalf("outcomes = %+v, want already_ran (R2)", reply.Outcomes)
	}
	if d.resend.calls() != 0 {
		t.Fatalf("sent for a deliberate skip")
	}
}

func TestDigestRetryNeverReopensYesterdaysSlot(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	d.front.status = http.StatusInternalServerError
	d.run() // day D 12:00 skipped
	d.front.status = http.StatusOK
	d.setNow(digestTestNow.Add(24 * time.Hour)) // D+1 12:00
	d.front.set(digestFrontRows(digestTestNow.Add(24 * time.Hour)))
	d.run()
	if d.resend.calls() != 1 {
		t.Fatalf("sends = %d, want D+1's own mail only (R3)", d.resend.calls())
	}
	if got, want := d.resend.keys[0], "choir_digest_"+gateTestOwner+"_2026-10-21"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	runs, err := d.store.LatestDigestRuns(context.Background(), gateTestOwner, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.RunDate == "2026-10-20" && r.Status != "skipped" {
			t.Fatalf("yesterday's slot was reopened: %+v", r)
		}
	}
}

func TestDigestRetryInterruptedRowInsideTheHourIsNewWorkOutsideItIsNot(t *testing.T) {
	ctx := context.Background()
	d := newDigestHarness(t)
	d.subscribe()
	if _, claimed, err := d.store.ClaimDigestRun(ctx, gateTestOwner, "2026-10-20", digestKindScheduled, d.h.digestNow()); err != nil || !claimed {
		t.Fatalf("claim = %v %v", claimed, err)
	}
	if n, err := d.store.CloseOpenDigestRuns(ctx, d.h.digestNow()); err != nil || n != 1 {
		t.Fatalf("boot close = %d %v", n, err)
	}
	if d.resend.calls() != 0 {
		t.Fatalf("boot sent mail (R5)")
	}
	d.setNow(digestTestNow.Add(10 * time.Minute))
	d.run()
	if d.resend.calls() != 1 {
		t.Fatalf("next fire inside the hour sent %d, want 1 (R4)", d.resend.calls())
	}

	// Another slot whose interrupted row is found after the hour stays closed.
	d2 := newDigestHarness(t)
	d2.subscribe()
	if _, claimed, err := d2.store.ClaimDigestRun(ctx, gateTestOwner, "2026-10-20", digestKindScheduled, d2.h.digestNow()); err != nil || !claimed {
		t.Fatalf("claim = %v %v", claimed, err)
	}
	_, _ = d2.store.CloseOpenDigestRuns(ctx, d2.h.digestNow())
	d2.setNow(digestTestNow.Add(2 * time.Hour))
	d2.run()
	if d2.resend.calls() != 0 {
		t.Fatalf("sent after the slot hour (R4)")
	}
	if run := d2.lastRun("scheduled"); run.Status != "interrupted" {
		t.Fatalf("row = %+v, want interrupted", run)
	}
}

func TestDigestTimerFiresEveryTenMinutesForTheSlotHour(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "nix", "node-b.nix"))
	if err != nil {
		t.Fatal(err)
	}
	timer := digestNixBlock(t, string(raw), "systemd.timers.go-choir-digest =")
	for _, m := range []string{"00", "10", "20", "30", "40", "50"} {
		if !strings.Contains(timer, `"*-*-* 12:`+m+`:00 UTC"`) {
			t.Errorf("R7: timer lacks 12:%s UTC:\n%s", m, timer)
		}
	}
	if strings.Contains(timer, "13:") {
		t.Errorf("R7: timer reaches past the slot hour:\n%s", timer)
	}
	if !strings.Contains(timer, "Persistent = false;") {
		t.Errorf("R7: timer must stay Persistent = false")
	}
}
