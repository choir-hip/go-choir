package calibration

// Takes' forecast and the settlement seam (docs/reference/AUTORADIO-TAKES.md).
// Ways it can fail, written before the code:
//   1. a take forecast is written with no take id, or a story forecast with one;
//   2. the claim's project is not "take" or it lacks the take:<id> tag;
//   3. a settlement of a story is read as a verdict on a take;
//   4. a settlement of another claim, a hand-written look-alike claim, or a
//      settlement with no verdict (partial, unresolvable) is reported;
//   5. heard and skipped are swapped.

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func takeForecast(t *testing.T, takeID string) types.CommitmentRecord {
	t.Helper()
	in := forecastInput(t, "st-"+takeID[5:9], 0)
	in.Kind, in.TakeID = AutoradioKindTake, takeID
	rec, err := NewAutoradioForecast(in, time.Now())
	if err != nil {
		t.Fatalf("take forecast: %v", err)
	}
	return rec
}

func TestTakeForecastNamesItsTake(t *testing.T) {
	in := forecastInput(t, "st1", 0)
	in.Kind = AutoradioKindTake
	if _, err := NewAutoradioForecast(in, time.Now()); err == nil {
		t.Fatal("a take forecast with no take id was accepted")
	}
	in = forecastInput(t, "st1", 0)
	in.TakeID = "take-0123456789abcdef"
	if _, err := NewAutoradioForecast(in, time.Now()); err == nil {
		t.Fatal("a story forecast with a take id was accepted")
	}
	in.Kind, in.TakeID = AutoradioKindTake, "take-xyz"
	if _, err := NewAutoradioForecast(in, time.Now()); err == nil {
		t.Fatal("a malformed take id was accepted")
	}
	rec := takeForecast(t, "take-0123456789abcdef")
	if rec.Calibration.Claim.Project != "take" {
		t.Fatalf("project = %q, want take", rec.Calibration.Claim.Project)
	}
	found := false
	for _, tag := range rec.Calibration.Claim.Tags {
		found = found || tag == "take:take-0123456789abcdef"
	}
	if !found {
		t.Fatalf("tags %v lack take:<id>", rec.Calibration.Claim.Tags)
	}
}

func TestTakeOutcomeFromSettlement(t *testing.T) {
	claim := takeForecast(t, "take-0123456789abcdef")
	now := time.Now()
	heard, err := NewAutoradioSettlementRecord(claim, AutoradioHeardRule, true, "air-1", now)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := NewAutoradioSettlementRecord(claim, AutoradioHeardRule, false, "air-2", now)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := TakeOutcomeFromSettlement(claim, heard); !ok || got.Outcome != "heard" || got.TakeID != "take-0123456789abcdef" || got.ClaimID != claim.RecordID {
		t.Fatalf("heard settlement = %+v, %v", got, ok)
	}
	if got, ok := TakeOutcomeFromSettlement(claim, skipped); !ok || got.Outcome != "skipped" {
		t.Fatalf("skipped settlement = %+v, %v", got, ok)
	}
	// A story's settlement is not a verdict on a take.
	story, err := NewAutoradioForecast(forecastInput(t, "stq", 1), now)
	if err != nil {
		t.Fatal(err)
	}
	storySettled, err := NewAutoradioSettlementRecord(story, AutoradioHeardRule, true, "air-3", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := TakeOutcomeFromSettlement(story, storySettled); ok {
		t.Fatal("a story settlement was read as a take outcome")
	}
	// A settlement of another claim is not this claim's.
	other := takeForecast(t, "take-fedcba9876543210")
	if _, ok := TakeOutcomeFromSettlement(other, heard); ok {
		t.Fatal("another claim's settlement was read")
	}
	// A settlement with no verdict is not reported.
	partial := heard
	cal := *heard.Calibration
	set := *cal.Settlement
	set.Outcome = types.CalibrationOutcomeUnresolvable
	cal.Settlement = &set
	partial.Calibration = &cal
	if _, ok := TakeOutcomeFromSettlement(claim, partial); ok {
		t.Fatal("an unresolvable settlement was reported")
	}
	// A look-alike claim, not written by the server, is refused.
	fake := claim
	fake.SubjectRefs = map[string]string{"desk": "other"}
	if _, ok := TakeOutcomeFromSettlement(fake, heard); ok {
		t.Fatal("a hand-written claim was read")
	}
	// A record that is not a settlement is not one.
	if _, ok := TakeOutcomeFromSettlement(claim, claim); ok {
		t.Fatal("a claim was read as its own settlement")
	}
}
