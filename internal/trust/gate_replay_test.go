package trust

// Failure modes of the gate replay (MA7.8), written before the code:
//  1. an act below I2 is counted as a tap;
//  2. the first act of a desk in a class (no prior record) is not gated;
//  3. an act after a fidelity event with no positive evidence since is not
//     gated, or is not counted under Breaker;
//  4. the tap rate divides by a span under one day.

import (
	"testing"
	"time"
)

func TestGateReplayCountsTaps(t *testing.T) {
	at := func(h int) string { return time.Date(2026, 11, 1, h, 0, 0, 0, time.UTC).Format(time.RFC3339) }
	in := Inputs{Frames: []FrameObservation{
		{Desk: "app:bad", RecordID: "r1", Used: []string{"desk.commit"}, At: at(1)},
		{Desk: "app:bad", RecordID: "r2", Used: []string{"desk.commit"}, At: at(2)},
		{Desk: "app:bad", RecordID: "r3", Used: []string{"desk.commit"}, At: at(3)},
		// I1: not a material act
		{Desk: "app:bad", RecordID: "r4", Used: []string{"desk.act"}, At: at(4)},
	}}
	rep := GateReplay(in, Params{})
	if rep.Acts != 3 || rep.Gated != 3 || rep.Breaker != 2 {
		t.Fatalf("acts %d gated %d breaker %d, want 3 3 2", rep.Acts, rep.Gated, rep.Breaker)
	}
	if rep.Days != 1 || rep.TapsPerDay != 3 {
		t.Fatalf("days %v taps/day %v", rep.Days, rep.TapsPerDay)
	}
	if rep.ByImpact["I2"] != 3 {
		t.Fatalf("by impact %v", rep.ByImpact)
	}
}
