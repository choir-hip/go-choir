package vmctl

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Review of S5 (both seats #1; DeepSeek #2, #4):
//   - a guest that answers 400 to a mode the host sent (an epoch-27 guest
//     answers busy_cold "unsupported mode") is asked again at every pass;
//     it must be unsupported for that mode until its next boot, without
//     losing the idle mode the same guest does support;
//   - a guest's refusal reason (selfdev_active, live_capsules, ...) is lost:
//     the record and the journal line must carry it;
//   - the periodic pass always picks the same first computers under the
//     in-flight limit of two, so a third named computer starves: the oldest
//     last act goes first.

func TestBusyColdA400IsUnsupportedForThatModeThisBoot(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.status = http.StatusBadRequest
	g.report = `{"error":"unsupported mode"}`
	paper := f.paper(g)
	f.busyColdPass(t)
	*f.now = f.now.Add(11 * time.Minute)
	f.busyColdPass(t)
	f.shortBy(5 * gib)
	f.stepZero()
	if g.calls.Load() != 1 {
		t.Fatalf("a guest that answered 400 to busy_cold was asked %d times this boot, want 1", g.calls.Load())
	}
	if recs := f.reg.GuestCacheReturnSummary().Recent; len(recs) != 1 || recs[0].Outcome != "unsupported" {
		t.Fatalf("records %+v, want one unsupported", recs)
	}
	if f.reg.cacheReturn.isUnsupported(paper.VMID, paper.Epoch) {
		t.Fatal("a busy_cold 400 marked the whole route unsupported (idle mode lost)")
	}
	// A new boot clears it.
	f.reg.mu.Lock()
	paper.Epoch++
	f.reg.mu.Unlock()
	*f.now = f.now.Add(11 * time.Minute)
	f.busyColdPass(t)
	if g.calls.Load() != 2 {
		t.Fatalf("after a new boot: %d calls, want 2", g.calls.Load())
	}
}

func TestBusyColdRefusalReasonIsRecordedAndJournaled(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.status = http.StatusConflict
	g.report = `{"outcome":"busy","reason":"selfdev_active","error":"a self-development operation is active"}`
	f.paper(g)
	f.busyColdPass(t)
	recs := f.reg.GuestCacheReturnSummary().Recent
	if len(recs) != 1 || recs[0].Outcome != "busy" || recs[0].Reason != "selfdev_active" {
		t.Fatalf("records %+v, want busy with reason selfdev_active", recs)
	}
	if line := recs[0].JournalLine(); !strings.Contains(line, "reason=selfdev_active") {
		t.Fatalf("journal line lacks the reason: %s", line)
	}
}

func TestBusyColdPeriodicAsksTheOldestLastActFirst(t *testing.T) {
	f := periodicCR(t, map[string]string{"computer-vm-a": GuestCacheReturnActive, "computer-vm-b": GuestCacheReturnActive, "computer-vm-c": GuestCacheReturnActive})
	ga, gb, gc := newMRGuest(t, "a", nil), newMRGuest(t, "b", nil), newMRGuest(t, "c", nil)
	for _, g := range []*mrGuest{ga, gb, gc} {
		g.report = claim(10 * mib)
	}
	f.computer("vm-a", 0, 3*gib, 1*gib, ga)
	f.computer("vm-b", 0, 3*gib, 1*gib, gb)
	f.computer("vm-c", 0, 3*gib, 1*gib, gc)
	f.busyColdPass(t) // two slots: two of the three
	*f.now = f.now.Add(10 * time.Minute)
	f.busyColdPass(t) // the one never asked goes first
	if ga.calls.Load() == 0 || gb.calls.Load() == 0 || gc.calls.Load() == 0 {
		t.Fatalf("after two passes a=%d b=%d c=%d: a named computer starved", ga.calls.Load(), gb.calls.Load(), gc.calls.Load())
	}
}
