package types

import (
	"strings"
	"testing"
	"time"
)

// Failure modes, written before the code:
//
//	R1 the phases overlap or leave gaps, so engineer plus platform plus
//	   upstream exceeds the total;
//	R2 a build job's wait is counted twice (as a wait and as engineer time);
//	R3 a choir-shot run is counted as a build, or a delegate wait as one;
//	R4 a round with no jobs reports no engineer time;
//	R5 unknown times (no bind, no Texture Cast) turn into negative or huge phases.
func TestBuildRoundReceiptPhases(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	at := func(m float64) time.Time { return t0.Add(time.Duration(m * float64(time.Minute))) }
	r := BuildRoundReceipt(RoundInputs{
		Outcome:       "apply",
		TextureCastAt: at(0), CastAt: at(1), BindAt: at(3), FreezeAt: at(11), EngineerEndAt: at(11.5), DecisionAt: at(13), EndAt: at(14),
		SpawnS: 60, WaitedS: map[string]int64{"host_memory": 120 + 60}, AdmissionWaitedS: 60, JobWaitedS: 120,
		Jobs: []RoundJob{
			{Kind: RoundJobBuild, Start: at(5), End: at(7)},
			{Kind: RoundJobShot, Start: at(8), End: at(9)},
			{Kind: RoundJobOther, Start: at(9), End: at(10)},
		},
	})
	want := map[string]int64{
		"texture_to_cast": 60, "cast_to_bind": 120, "spawn": 60, "latency": 0,
		"exploration": 120, "build": 120, "shot": 60,
	}
	got := map[string]int64{
		"texture_to_cast": r.TextureToCastS, "cast_to_bind": r.CastToBindS, "spawn": r.SpawnS, "latency": r.CastBindLatencyS,
		"exploration": r.ExplorationS, "build": r.BuildS, "shot": r.ShotS,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d (%+v)", k, got[k], v, r)
		}
	}
	// Window 8 min = exploration 2 + build 2 + shot 1 + other 3, less the 2 min job wait taken from other.
	if r.OtherEngineerS != 60 || r.JobWaitedS != 120 || r.EngineerS != 360 {
		t.Errorf("R2: other %d jobwait %d engineer %d", r.OtherEngineerS, r.JobWaitedS, r.EngineerS)
	}
	if r.FreezeToDecisionS != 120 || r.DecisionToEndS != 60 || r.TotalS != 840 {
		t.Errorf("tail: %+v", r)
	}
	// platform = spawn 60 + waits 180 + freeze-to-decision 120 + latency 0
	if r.PlatformS != 360 || r.PlatformPct != 43 {
		t.Errorf("platform %d %d%%", r.PlatformS, r.PlatformPct)
	}
	if r.EngineerS+r.PlatformS > r.TotalS {
		t.Errorf("R1: engineer %d + platform %d > total %d", r.EngineerS, r.PlatformS, r.TotalS)
	}
	if line := r.Line(); line != "round took 14 min: 6 engineer, 6 platform (43%)" {
		t.Errorf("line = %q", line)
	}
	if got := RoundLineFromRefs(r.Refs()); got != r.Line() {
		t.Errorf("line from refs = %q", got)
	}
	if s := r.Summary(); !strings.Contains(s, "choir-shot jobs 1 min") || !strings.Contains(s, "Waited 3 min: host memory 3 min") {
		t.Errorf("summary = %q", s)
	}
	if RoundJobKind([]string{"sh", "-c", "node scripts/choir-shot.mjs --shots x"}) != RoundJobShot ||
		RoundJobKind([]string{"choir-delegate", "wait", "--park"}) != RoundJobOther || RoundJobKind([]string{"go", "build"}) != RoundJobBuild {
		t.Error("R3: job kinds")
	}
}

func TestBuildRoundReceiptWithoutJobsOrTimes(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	r := BuildRoundReceipt(RoundInputs{Outcome: "reject", CastAt: t0, BindAt: t0.Add(time.Minute), EngineerEndAt: t0.Add(5 * time.Minute), EndAt: t0.Add(6 * time.Minute)})
	if r.ExplorationS != 240 || r.EngineerS != 240 || r.TotalS != 360 || r.FreezeToDecisionS != 60 {
		t.Errorf("R4: %+v", r)
	}
	z := BuildRoundReceipt(RoundInputs{Outcome: "fail", EndAt: t0})
	if z.TotalS != 0 || z.EngineerS != 0 || z.PlatformPct != 0 || z.Line() != "round took 0 min: 0 engineer, 0 platform (0%)" {
		t.Errorf("R5: %+v", z)
	}
}

// The receipt states the round's predictions against what happened, and
// Texture's line says what the clock claims predicted and what they took.
func TestRoundReceiptCarriesPredictions(t *testing.T) {
	preds := []RoundPrediction{
		{Kind: "verify_pass", P: 0.8, Answer: "yes"},
		{Kind: "freeze_minutes", P: 0.7, Estimate: 25, Low: 15, High: 45, Answer: "yes", ActualS: 35 * 60, HasActual: true},
		{Kind: "approval_minutes", P: 0.6, Estimate: 90, Low: 40, High: 240, Answer: "no", ActualS: 300 * 60, HasActual: true, Censored: true},
		{Kind: "owner_approves", P: 0.6},
	}
	refs := RoundReceipt{TotalS: 3000, EngineerS: 2400, PlatformS: 600, PlatformPct: 20}.Refs()
	RoundPredictionRefs(refs, preds)
	line := RoundLineFromRefs(refs)
	if !strings.Contains(line, ", Freeze predicted 25 min (15-45), took 35") || !strings.Contains(line, ", approval predicted 90 min (40-240), none in 300 min") {
		t.Fatalf("line %q", line)
	}
	sum := RoundPredictionsSummary(preds)
	for _, want := range []string{"verify_pass p 0.80: yes", "freeze_minutes 25 min (15-45, p 0.70), took 35 min: yes", "approval_minutes 90 min (40-240, p 0.60), none in 300 min: no", "owner_approves p 0.60: open"} {
		if !strings.Contains(sum, want) {
			t.Fatalf("summary %q lacks %q", sum, want)
		}
	}
	plain := RoundReceipt{TotalS: 3000, EngineerS: 2400, PlatformS: 600, PlatformPct: 20}.Refs()
	if RoundLineFromRefs(plain) != (RoundReceipt{TotalS: 3000, EngineerS: 2400, PlatformS: 600, PlatformPct: 20}).Line() {
		t.Fatal("a round without predictions changed its line")
	}
}
