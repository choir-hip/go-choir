package trust

// Failure modes of the pre-registered units (MA7.9; pre-registration section
// 1, P1 and P2), written before the code:
//  1. a predictor reads an item at or after the cutoff (the label leaks into
//     the lower bound);
//  2. a unit with no act in its window is kept (nothing could fail);
//  3. a unit whose window has not closed is kept as a negative;
//  4. a failure in the window does not label the unit, or one before the
//     cutoff does;
//  5. a class below I2 yields a unit;
//  6. the units depend on record order.

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func preregFixture() Inputs {
	at := func(m time.Month, d, h int) string { return time.Date(2026, m, d, h, 0, 0, 0, time.UTC).Format(time.RFC3339) }
	k := ActionClass{Grant: "desk.commit", Impact: "I2"}
	low := ActionClass{Grant: "desk.act", Impact: "I1"}
	return Inputs{
		OwnerActs: []OwnerAct{
			{Desk: "app:a", Act: "approve", Class: k, At: at(11, 2, 10)},
			{Desk: "app:a", Act: "approve", Class: k, At: at(11, 10, 10)},
			{Desk: "app:a", Act: "approve", Class: k, At: at(11, 24, 10)},
			{Desk: "app:a", Act: "approve", Class: k, At: at(12, 5, 10)},
			{Desk: "app:a", Act: "approve", Class: low, At: at(11, 3, 10)},
			{Desk: "app:a", Act: "approve", Class: low, At: at(11, 11, 10)},
		},
		Frames: []FrameObservation{{Desk: "app:a", RecordID: "f1", Used: []string{"desk.commit"}, At: at(11, 12, 10)}},
	}
}

func TestPreregUnitsDrawsThePreRegisteredUnits(t *testing.T) {
	set := PreregUnits(preregFixture(), Params{})
	wantCut := []time.Time{time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)}
	if !reflect.DeepEqual(set.Cutoffs, wantCut) {
		t.Fatalf("cutoffs %v", set.Cutoffs)
	}
	// 5: only the I2 class; 3: the two late cutoffs are open.
	if len(set.Units) != 2 || set.DroppedOpen != 2 || set.DroppedNoAct != 0 {
		t.Fatalf("units %d open %d no-act %d: %+v", len(set.Units), set.DroppedOpen, set.DroppedNoAct, set.Units)
	}
	a, b := set.Units[0], set.Units[1]
	if a.Class.Impact != "I2" || b.Class.Impact != "I2" {
		t.Fatalf("a class below I2 yielded a unit: %+v", set.Units)
	}
	// 4: the frame on 11-12 labels the 11-09 unit, not the 11-16 one.
	if !a.Positive || b.Positive {
		t.Fatalf("labels %v %v, want true false", a.Positive, b.Positive)
	}
	if a.PriorItems != 1 || a.PriorFailures != 0 || a.WindowActs != 2 || b.PriorItems != 3 || b.PriorFailures != 1 || b.WindowActs != 1 {
		t.Fatalf("counts %+v %+v", a, b)
	}
	if !a.HasRow || !b.HasRow || a.SuccessRate != 1 || b.SuccessRate != 2.0/3 || a.ImpactRank != 2 {
		t.Fatalf("predictors %+v %+v", a, b)
	}
	if b.RecencyHours != (4*24 - 10) {
		t.Fatalf("recency %v", b.RecencyHours)
	}
}

func TestPreregUnitsReadNothingFromTheWindow(t *testing.T) {
	in := preregFixture()
	base := PreregUnits(in, Params{})
	// 1: a failure exactly at the 11-16 cutoff belongs to the window.
	in.Frames = append(in.Frames, FrameObservation{Desk: "app:a", RecordID: "f2", Used: []string{"desk.commit"}, At: "2026-11-16T00:00:00Z"})
	got := PreregUnits(in, Params{})
	if len(got.Units) != 2 {
		t.Fatalf("units %d", len(got.Units))
	}
	if got.Units[1].LCB != base.Units[1].LCB || got.Units[1].Mean != base.Units[1].Mean || got.Units[1].PriorItems != base.Units[1].PriorItems {
		t.Fatalf("the predictor read the window: %+v against %+v", got.Units[1], base.Units[1])
	}
	if !got.Units[1].Positive {
		t.Fatal("a failure at the cutoff did not label its window")
	}
}

func TestPreregUnitsDropsAWindowWithNoAct(t *testing.T) {
	in := preregFixture()
	// 2: move the 11-24 act out of the 11-16 window's reach.
	in.OwnerActs[2].At = "2026-12-04T10:00:00Z"
	set := PreregUnits(in, Params{})
	if set.DroppedNoAct != 1 || len(set.Units) != 1 {
		t.Fatalf("no-act %d units %d", set.DroppedNoAct, len(set.Units))
	}
}

func TestPreregUnitsIndependentOfOrder(t *testing.T) {
	in := preregFixture()
	want := PreregUnits(in, Params{})
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 5; i++ {
		r.Shuffle(len(in.OwnerActs), func(a, b int) { in.OwnerActs[a], in.OwnerActs[b] = in.OwnerActs[b], in.OwnerActs[a] })
		if got := PreregUnits(in, Params{}); !reflect.DeepEqual(got, want) {
			t.Fatalf("order changed the units")
		}
	}
}

func TestPreregUnitsEmpty(t *testing.T) {
	set := PreregUnits(Inputs{}, Params{})
	if set.MaterialItems != 0 || len(set.Units) != 0 || set.Units == nil || set.Cutoffs == nil {
		t.Fatalf("empty set %+v", set)
	}
}
