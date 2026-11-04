package agentcore

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEngineerClockLineFiresAtBudgetMinusTwenty(t *testing.T) {
	budget := 60 * time.Minute
	if got := engineerClockLine(39*time.Minute+59*time.Second, budget); got != "" {
		t.Fatalf("line before minute 40 = %q", got)
	}
	want := "40 minutes used of 60; Freeze within the next 20 minutes or say why in a Report."
	if got := engineerClockLine(40*time.Minute, budget); got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	if got := engineerClockLine(75*time.Minute, budget); !strings.Contains(got, "within the next 0 minutes") {
		t.Fatalf("past budget line = %q", got)
	}
	if engineerClockLine(50*time.Minute, 0) != "" {
		t.Fatal("no budget must give no line")
	}
}

func TestEngineerClockLineOncePerAssignmentAndRevision(t *testing.T) {
	rt := &Runtime{}
	now := time.Now()
	c := &engineerClock{Elapsed: 41 * time.Minute, Budget: 60 * time.Minute, Key: "a1:1"}
	if rt.takeEngineerClockLine(c, now) == "" {
		t.Fatal("first due line not returned")
	}
	if rt.takeEngineerClockLine(c, now) != "" {
		t.Fatal("line repeated for the same assignment")
	}
	c2 := &engineerClock{Elapsed: 41 * time.Minute, Budget: 60 * time.Minute, Key: "a1:1", Revision: 2}
	if rt.takeEngineerClockLine(c2, now) == "" {
		t.Fatal("a new revision must get its own line")
	}
	early := &engineerClock{Elapsed: 5 * time.Minute, Budget: 60 * time.Minute, Key: "a2:1"}
	if rt.takeEngineerClockLine(early, now) != "" {
		t.Fatal("early clock fired")
	}
	if rt.takeEngineerClockLine(early, now) != "" || rt.takeEngineerClockLine(nil, now) != "" {
		t.Fatal("nil or early clock must stay silent and not consume the line")
	}
	late := &engineerClock{Elapsed: 45 * time.Minute, Budget: 60 * time.Minute, Key: "a2:1"}
	if rt.takeEngineerClockLine(late, now) == "" {
		t.Fatal("an early look must not consume the once-line")
	}
}

// C1: the clock and the freeze_minutes settlement share one anchor.
func TestEngineerClockAnchorIsTheSettlementAnchor(t *testing.T) {
	castAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	cast := types.CommitmentRecord{RecordID: "c1"}
	cast.Provenance.CommittedAt = castAt.Format(time.RFC3339Nano)
	assignment := types.EngineeringAssignment{AssignmentID: "a1", CreatedAt: castAt.Add(-3 * time.Minute)}
	anchor := roundClockAnchor(cast, assignment)
	elapsed, _, ok := engineerClockAt(anchor, castAt.Add(41*time.Minute), 60*time.Minute)
	if !ok || elapsed != 41*time.Minute {
		t.Fatalf("elapsed = %v ok=%v, want 41m from the Cast", elapsed, ok)
	}
	// No Cast stamp: the assignment's creation, as the settlement falls back.
	if got := roundClockAnchor(types.CommitmentRecord{}, assignment); got != assignment.CreatedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("fallback anchor = %q", got)
	}
	// C3: an unusable anchor or budget gives no clock.
	if _, _, ok := engineerClockAt("garbage", castAt, time.Hour); ok {
		t.Fatal("garbage anchor gave a clock")
	}
	if _, _, ok := engineerClockAt(anchor, castAt, 0); ok {
		t.Fatal("zero budget gave a clock")
	}
	if _, _, ok := engineerClockAt(anchor, castAt.Add(-time.Minute), time.Hour); ok {
		t.Fatal("a future anchor gave a clock")
	}
}

// C6: nothing of management's prediction rides the clock.
func TestEngineerClockCarriesNoPrediction(t *testing.T) {
	c := engineerClock{Elapsed: 12 * time.Minute, Budget: time.Hour, Revision: 2, Key: "a:1"}
	raw, err := json.Marshal(c.cell())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"predict", "freeze_minutes", "judge", "verify_pass"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("clock frame %s mentions %q", raw, bad)
		}
	}
	if c.cell().ElapsedSec != 720 || c.cell().BudgetSec != 3600 || c.cell().Revision != 2 {
		t.Fatalf("cell clock = %+v", c.cell())
	}
}
