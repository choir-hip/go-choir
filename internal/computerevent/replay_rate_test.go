package computerevent

import (
	"testing"
	"time"
)

// Ways the replay rate could be wrong: recorded from a replay of a few
// events (noise); never recorded, so the producer assumes a constant forever;
// a later replay of a few events overwrites a good measurement.
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
