package calibration

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// AutoradioKindTake is the segment kind of a take (design 11.5): the DJ's
// lead-in, then the publisher's own audio. Its forecast is the story's, with
// the claim's project "take" and the tag take:<take_id>.
const AutoradioKindTake = "take"

// TakeOutcome is what a listener's computer reports to a take's publisher
// record: that this listener's forecast on the take settled heard or skipped.
type TakeOutcome struct {
	TakeID  string
	Outcome string // "heard" or "skipped"
	ClaimID string
}

// TakeOutcomeFromSettlement is the one seam between settlement and the
// publisher's track record (docs/reference/AUTORADIO-TAKES.md). Given a take's
// forecast claim and the autoradio.settlement record of it, it says what to
// report: heard when the settlement is true, skipped when false. It returns
// ok=false for anything that is not a verdict on a take: a claim of another
// kind, a settlement of another claim, one that is not an Autoradio settlement,
// and a settlement with no verdict (partial or unresolvable), which is not
// reported (design 11.5, step 6).
//
// It reads records and writes nothing; the caller (the settlement path of the
// listener's computer, built on lane/learn-settle) posts the result to
// POST /api/platform/takes/{take_id}/outcome.
func TakeOutcomeFromSettlement(claim, settlement types.CommitmentRecord) (TakeOutcome, bool) {
	if checkServerForecast(claim, AutoradioAgent) != nil || claim.Calibration.Claim.Project != AutoradioKindTake {
		return TakeOutcome{}, false
	}
	if !IsAutoradioSettlement(settlement) ||
		settlement.Calibration == nil || settlement.Calibration.Settlement == nil || settlement.Calibration.Settlement.ClaimID != claim.RecordID {
		return TakeOutcome{}, false
	}
	var takeID string
	for _, tag := range claim.Calibration.Claim.Tags {
		if id, ok := strings.CutPrefix(tag, "take:"); ok && types.TakeIDRe.MatchString(id) {
			takeID = id
		}
	}
	if takeID == "" {
		return TakeOutcome{}, false
	}
	switch settlement.Calibration.Settlement.Outcome {
	case types.CalibrationOutcomeTrue:
		return TakeOutcome{TakeID: takeID, Outcome: "heard", ClaimID: claim.RecordID}, true
	case types.CalibrationOutcomeFalse:
		return TakeOutcome{TakeID: takeID, Outcome: "skipped", ClaimID: claim.RecordID}, true
	}
	return TakeOutcome{}, false
}
