package computerevent

import (
	"testing"
	"time"
)

// Ways the replay rate could be wrong: recorded from a replay of a few
// events (noise); never recorded, so the producer assumes a constant forever;
// a later replay of a few events overwrites a good measurement; a fast live
// boot replay overwrites the slow restore replay, so the snapshot bound
// relaxes (review round 1, finding 3); the restore's own dry-run replay is
// recorded on a copy and discarded.
func TestReplayRateIsRecordedOnlyFromARealReplay(t *testing.T) {
	a := &ComputerEventAppender{}
	if _, ok := a.ReplayRate(); ok {
		t.Fatal("a rate before any replay")
	}
	a.recordReplayRate(50, time.Second)
	if _, ok := a.ReplayRate(); ok {
		t.Fatal("50 events is noise, not a rate")
	}
	a.recordReplayRate(33332, 294*time.Second)
	rate, ok := a.ReplayRate()
	if !ok || rate < 113 || rate > 114 {
		t.Fatalf("rate=%v ok=%v want 113.4", rate, ok)
	}
	a.recordReplayRate(10, time.Millisecond)
	if again, _ := a.ReplayRate(); again != rate {
		t.Fatalf("a replay of 10 events overwrote the rate: %v", again)
	}
}

// A fast boot replay never raises the rate a slow restore replay set; the
// slowest measurement stands, in either order.
func TestReplayRateKeepsTheSlowestMeasurement(t *testing.T) {
	a := &ComputerEventAppender{}
	a.recordReplayRate(1080, 3*time.Second) // 360/s, a live boot replay
	a.recordReplayRate(33332, 294*time.Second)
	if r, _ := a.ReplayRate(); r > 114 {
		t.Fatalf("rate %v: the slow restore replay must stand after a fast one", r)
	}
	a.recordReplayRate(2000, 5*time.Second) // 400/s afterwards
	if r, _ := a.ReplayRate(); r > 114 {
		t.Fatalf("rate %v: a later fast replay relaxed the bound", r)
	}
}

// The restore replays through a dry-run copy of the appender; its rate is the
// restore path's rate and must land on the parent.
func TestDryRunReplayRateIsRecordedOnTheParent(t *testing.T) {
	parent := &ComputerEventAppender{}
	dry := &ComputerEventAppender{rateParent: parent}
	dry.recordReplayRate(33332, 294*time.Second)
	if r, ok := parent.ReplayRate(); !ok || r > 114 {
		t.Fatalf("parent rate=%v ok=%v: the dry-run's rate was discarded", r, ok)
	}
}
