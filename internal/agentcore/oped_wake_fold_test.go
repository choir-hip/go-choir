package agentcore

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ways the wake decision can fail (docs/evidence/c2-opeds-failure-list-2026-11-10.md, X11 to X13):
//
//  - a rejected op-ed wakes the desk by itself;
//  - rejects wake at once, or never, or the backoff does not double and reset;
//  - an accepted op-ed that arrives while the desk works starts a second wake
//    instead of joining the next one.

var foldNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func item(version, author string, verdict types.OpedVerdictValue) opedStoryItem {
	return opedStoryItem{VersionID: version, AuthorLabel: author, Verdict: verdict, Judged: verdict != ""}
}

func TestRejectedOpedDoesNotWakeDesk(t *testing.T) {
	out := foldOpedWake(opedWakeInput{Now: foldNow, Items: []opedStoryItem{
		item("v1", "writer-a", types.OpedRejectedUnclear),
		item("v2", "writer-b", types.OpedRejectedRedundant),
	}})
	if out.Woken || out.Why != types.OpedWakeBelowThreshold {
		t.Fatalf("two rejects by two authors woke or mislabelled: %+v", out)
	}
	if out.Counts.UnreadRejects != 2 || out.Counts.DistinctRejectAuthors != 2 || out.Counts.Threshold != rejectWakeBaseThreshold {
		t.Fatalf("counts = %+v", out.Counts)
	}
	// An unjudged or unavailable op-ed is neither accepted nor a reject.
	out = foldOpedWake(opedWakeInput{Now: foldNow, Items: []opedStoryItem{item("v3", "writer-c", ""), item("v4", "writer-d", types.OpedUnavailable)}})
	if out.Woken || out.Hold || out.Counts.Rejected != 0 || out.Counts.Accepted != 0 {
		t.Fatalf("pending and unavailable op-eds: %+v", out)
	}
}

func TestAcceptedOpedWakesAndCarriesUnreadRejects(t *testing.T) {
	out := foldOpedWake(opedWakeInput{Now: foldNow, Items: []opedStoryItem{
		item("v1", "writer-a", types.OpedRejectedAbusive),
		item("v2", "writer-b", types.OpedAccepted),
	}})
	if !out.Woken || out.Why != types.OpedWakeAccepted || len(out.WakeVersionIDs) != 1 || out.WakeVersionIDs[0] != "v2" {
		t.Fatalf("accepted op-ed: %+v", out)
	}
	// The reject rides along as read (the desk is shown it in this wake).
	if len(out.ReadVersionIDs) != 2 {
		t.Fatalf("read = %v, want both", out.ReadVersionIDs)
	}
	// Already woken for: nothing more.
	again := foldOpedWake(opedWakeInput{Now: foldNow, Items: []opedStoryItem{
		{VersionID: "v2", AuthorLabel: "writer-b", Verdict: types.OpedAccepted, Judged: true, Woken: true},
	}})
	if again.Woken || again.Hold {
		t.Fatalf("a woken op-ed woke again: %+v", again)
	}
}

// X13: while the desk works, an accepted op-ed is held and joins the wake
// that follows, together with any that arrive meanwhile.
func TestOpedCoalescing(t *testing.T) {
	items := []opedStoryItem{item("v1", "writer-a", types.OpedAccepted)}
	busy := foldOpedWake(opedWakeInput{Now: foldNow, Busy: true, Items: items})
	if busy.Woken || !busy.Hold {
		t.Fatalf("busy desk: %+v", busy)
	}
	items = append(items, item("v2", "writer-b", types.OpedAccepted))
	idle := foldOpedWake(opedWakeInput{Now: foldNow, Items: items})
	if !idle.Woken || len(idle.WakeVersionIDs) != 2 {
		t.Fatalf("idle desk: %+v", idle)
	}
}

// X12: the backoff.
func TestRejectBackoffFold(t *testing.T) {
	three := []opedStoryItem{
		item("v1", "writer-a", types.OpedRejectedUnclear),
		item("v2", "writer-b", types.OpedRejectedOffTopic),
		item("v3", "writer-c", types.OpedRejectedRedundant),
	}
	// Three distinct authors pass the first threshold.
	out := foldOpedWake(opedWakeInput{Now: foldNow, Items: three})
	if !out.Woken || out.Why != types.OpedWakeRejectBackoff || len(out.WakeVersionIDs) != 3 {
		t.Fatalf("three authors: %+v", out)
	}
	// The same author three times is one author.
	same := []opedStoryItem{item("v1", "writer-a", types.OpedRejectedUnclear), item("v2", "writer-a", types.OpedRejectedUnclear), item("v3", "writer-a", types.OpedRejectedUnclear)}
	if out := foldOpedWake(opedWakeInput{Now: foldNow, Items: same}); out.Woken || out.Counts.DistinctRejectAuthors != 1 {
		t.Fatalf("one author three times: %+v", out)
	}
	// After a wake that changed nothing, the threshold and the gap double.
	prior := []opedPriorWake{{At: foldNow.Add(-48 * time.Hour), Changed: false}}
	six := append([]opedStoryItem{}, three...)
	out = foldOpedWake(opedWakeInput{Now: foldNow, Items: six, PriorRejectWakes: prior})
	if out.Woken || out.Counts.Threshold != 2*rejectWakeBaseThreshold || out.Counts.NoChangeWakes != 1 {
		t.Fatalf("after a no-change wake, 3 authors: %+v", out)
	}
	six = append(six, item("v4", "writer-d", types.OpedRejectedUnclear), item("v5", "writer-e", types.OpedRejectedUnclear), item("v6", "writer-f", types.OpedRejectedUnclear))
	out = foldOpedWake(opedWakeInput{Now: foldNow, Items: six, PriorRejectWakes: prior})
	if !out.Woken || out.Counts.Threshold != 2*rejectWakeBaseThreshold {
		t.Fatalf("after a no-change wake, 6 authors: %+v", out)
	}
	// The minimum gap doubles too: 30 hours after a no-change wake is under 48.
	recent := []opedPriorWake{{At: foldNow.Add(-30 * time.Hour), Changed: false}}
	out = foldOpedWake(opedWakeInput{Now: foldNow, Items: six, PriorRejectWakes: recent})
	if out.Woken || out.Why != types.OpedWakeGapNotElapsed || out.Counts.GapSeconds != int(2*rejectWakeBaseGap/time.Second) {
		t.Fatalf("inside the doubled gap: %+v", out)
	}
	// A wake that changed something resets both.
	reset := []opedPriorWake{{At: foldNow.Add(-72 * time.Hour), Changed: false}, {At: foldNow.Add(-48 * time.Hour), Changed: true}}
	out = foldOpedWake(opedWakeInput{Now: foldNow, Items: three, PriorRejectWakes: reset})
	if !out.Woken || out.Counts.Threshold != rejectWakeBaseThreshold || out.Counts.NoChangeWakes != 0 {
		t.Fatalf("after a change: %+v", out)
	}
	// A busy desk holds a reject wake too.
	if out := foldOpedWake(opedWakeInput{Now: foldNow, Items: three, Busy: true}); out.Woken || !out.Hold {
		t.Fatalf("busy reject wake: %+v", out)
	}
}
