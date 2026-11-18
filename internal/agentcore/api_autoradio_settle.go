package agentcore

// Station 2: the run sheet's op log is the air log. When the sheet's reducer
// applies an op that ends a desk entry which carries a forecast claim, the
// forecast is settled once, by rule, from that op. The settler is this path
// acting on an op the owner's session posted with by: phone; the programme route has no path
// to it (a scan test holds that) and nothing a model writes reaches it.
//
// Outcomes (owner, 2026-11-17), from ended.outcome as the reducer stored it:
//
//	finished       heard through: the forecast came true
//	skipped, cut   the forecast was false
//	left, failed   no verdict: the listener closed the app or signed out, or
//	               the entry failed to play. Nothing is settled; an
//	               autoradio.no_verdict record keeps the op and its position
//
// An op whose own time (ended.at, never later than its arrival) is after the
// forecast's deadline is recorded as late and settles nothing. A phone that
// is offline delivers later and settles on arrival with the op's own time.
// An entry the phone never closes (a crash, a kill) sends no op: its forecast
// lapses at the deadline, unsettled.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/runsheet"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	kindNoVerdict = "autoradio.no_verdict"

	noVerdictLeft   = "left"
	noVerdictFailed = "failed"
	noVerdictLate   = "late"
	// superseded: the phone put another entry on air while this one was on
	// air, so this one's end op was lost (a killed app); the reducer closed it
	// as left. refused: the claim could not be settled by the rule (a missing
	// or foreign claim, a segment that does not match, a validation refusal).
	// unplaced: the programme route could not put the entry on the sheet, so no
	// op can ever end it.
	noVerdictSuperseded = "superseded"
	noVerdictRefused    = "refused"
	noVerdictUnplaced   = "unplaced"
	// desk_caused: the op that ended the entry was the desk's, or came through
	// the trusted in-process door. The forecaster never settles.
	noVerdictDeskCaused = "desk_caused"
	// no_outcome: the phone ended the entry as heard without saying how
	// (no ended.outcome). Nothing in the op says it was heard through.
	noVerdictNoOutcome = "no_outcome"

	// settleMaxBackdate bounds how far back an op's own ended.at may reach:
	// arrival minus this. The listener's own device can still backdate within it.
	settleMaxBackdate = 7 * 24 * time.Hour
)

func noVerdictRecordID(claimID, opID string) string { return claimID + ":noverdict:" + opID }

// claimTag returns the value of the first tag with the prefix, or "".
func claimTag(claim types.CommitmentRecord, prefix string) string {
	if claim.Calibration == nil || claim.Calibration.Claim == nil {
		return ""
	}
	for _, t := range claim.Calibration.Claim.Tags {
		if v, ok := strings.CutPrefix(t, prefix); ok {
			return v
		}
	}
	return ""
}

// settleEnded settles or records every desk entry the applied op ended.
// before is the sheet the op was applied to and after the result. It returns
// only a failed tape read or write (the caller evicts the sheet and does not
// acknowledge the op); a claim it cannot use is logged and left alone.
//
// Rule 1 is checked here, not left to the shape of the programme route's ops:
// an ending settles only when the op that caused it was posted over the
// untrusted HTTP path by the listener's session (by phone or listener). An
// ending caused by a desk op, or by any op of the trusted in-process door the
// desk uses, writes a no-verdict record (desk_caused) and nothing else, so a
// forecaster that cut its own entry could not settle its own forecast.
func (ls *liveSheet) settleEnded(ctx context.Context, env sheetEnv, before, after runsheet.RunSheet, op runsheet.Op, now time.Time, trusted bool) error {
	byListener := !trusted && (op.By == runsheet.ByPhone || op.By == runsheet.ByListener)
	was := map[string]runsheet.State{}
	for _, e := range before.Entries {
		was[e.ID] = e.State
	}
	for _, e := range after.Entries {
		if was[e.ID] != runsheet.StateOnAir || (e.State != runsheet.StateHeard && e.State != runsheet.StateSkipped) {
			continue
		}
		if e.PlacedBy != runsheet.ByDesk || e.Forecast == nil || e.Forecast.ClaimID == "" {
			continue
		}
		// An aired on_air for another entry closed this one: its own end op
		// was lost. No verdict, with that reason.
		superseded := op.Op == runsheet.OpAired && op.State == runsheet.StateOnAir && op.ID != e.ID
		if !byListener {
			if err := ls.writeNoVerdict(ctx, env, e.Forecast.ClaimID, before.PositionSeconds, e, op, endedOutcome(e), noVerdictDeskCaused, now); err != nil {
				return err
			}
			continue
		}
		if err := ls.settleEntry(ctx, env, before.PositionSeconds, e, op, now, superseded); err != nil {
			return err
		}
	}
	return nil
}

func (ls *liveSheet) settleEntry(ctx context.Context, env sheetEnv, sheetPosition float64, e runsheet.Entry, op runsheet.Op, now time.Time, superseded bool) error {
	claimID := e.Forecast.ClaimID
	claim, err := env.led.GetCommitmentRecord(ctx, ls.ownerID, env.computer, claimID)
	if err != nil {
		return err
	}
	// The entry must be the segment the claim is about: a claim id on an entry
	// that is not that segment's own entry settles nothing. It is recorded as
	// no verdict (refused), not only logged.
	if claim == nil || !calibration.IsClaim(claim) || claim.Calibration.Agent != calibration.AutoradioAgent ||
		programmeEntryID(claimTag(*claim, "seg:")) != e.ID {
		log.Printf("autoradio settle: entry %s names claim %s, which is not that entry's forecast; nothing settled", e.ID, claimID)
		return ls.writeNoVerdict(ctx, env, claimID, sheetPosition, e, op, endedOutcome(e), noVerdictRefused, now)
	}
	settledID := calibration.SettlementRecordID(claimID, types.CalibrationScorerRule, calibration.AutoradioHeardRule)
	prior, err := env.led.GetCommitmentRecord(ctx, ls.ownerID, env.computer, settledID)
	if err != nil {
		return err
	}
	if prior != nil {
		return nil // settled once; a settlement never changes
	}
	outcome := endedOutcome(e)
	// The op's own time is what the deadline is judged on: not its arrival,
	// and not before the claim was made. The listener's own device says it, so
	// it can backdate; it may reach back at most settleMaxBackdate before the
	// arrival.
	opAt := now
	if e.Ended != nil && e.Ended.At != "" {
		if t, perr := time.Parse(time.RFC3339Nano, e.Ended.At); perr == nil && !t.After(now) {
			opAt = t
		}
	}
	if floor := now.Add(-settleMaxBackdate); opAt.Before(floor) {
		opAt = floor
	}
	if c, perr := time.Parse(time.RFC3339Nano, claim.Provenance.CommittedAt); perr == nil && opAt.Before(c) {
		opAt = c
	}
	reason := ""
	switch {
	case superseded:
		reason = noVerdictSuperseded
	case claim.Calibration.Claim != nil && claim.Calibration.Claim.Deadline != "" && deadlinePassed(claim.Calibration.Claim.Deadline, opAt):
		reason = noVerdictLate
	case op.Op == runsheet.OpAired && op.Ended == nil && e.State == runsheet.StateHeard:
		// The reducer synthesises Ended{finished} for aired heard; the op
		// itself said nothing, so a true settlement would have no evidence.
		reason = noVerdictNoOutcome
	case outcome == runsheet.OutcomeLeft:
		reason = noVerdictLeft
	case outcome == runsheet.OutcomeFailed:
		reason = noVerdictFailed
	case outcome == runsheet.OutcomeFinished && e.State != runsheet.StateHeard:
		// A skipped entry that says finished contradicts itself: true needs the
		// entry heard as well as the outcome finished.
		reason = noVerdictRefused
	}
	if reason != "" {
		return ls.writeNoVerdict(ctx, env, claimID, sheetPosition, e, op, outcome, reason, now)
	}
	heard := outcome == runsheet.OutcomeFinished
	_, err = calibration.SettleAutoradio(ctx, env.led, ls.ownerID, env.computer, claimID, calibration.AutoradioHeardRule, heard, op.OpID, opAt)
	var bad *calibration.ValidationError
	if errors.As(err, &bad) {
		log.Printf("autoradio settle: claim %s refused: %v", claimID, err)
		return ls.writeNoVerdict(ctx, env, claimID, sheetPosition, e, op, outcome, noVerdictRefused, now)
	}
	return err
}

func deadlinePassed(deadline string, at time.Time) bool {
	d, err := time.Parse(time.RFC3339, deadline)
	return err == nil && at.After(d)
}

// endedOutcome is the outcome the reducer stored on an ended entry. The entry
// state is read only when no ended was stored (an old sheet), never to make a
// left ending finished (a left ending is stored with state heard). Whether the
// op itself stated the outcome is judged separately (settleEntry, no_outcome).
func endedOutcome(e runsheet.Entry) runsheet.Outcome {
	if e.Ended != nil {
		return e.Ended.Outcome
	}
	if e.State == runsheet.StateSkipped {
		return runsheet.OutcomeSkipped
	}
	return runsheet.OutcomeFinished
}

func (ls *liveSheet) writeNoVerdict(ctx context.Context, env sheetEnv, claimID string, sheetPosition float64, e runsheet.Entry, op runsheet.Op, outcome runsheet.Outcome, reason string, now time.Time) error {
	pos, src := sheetPosition, "sheet"
	// The op's own position belongs to the entry the op names. An ending
	// another entry's op caused (a takeover, a cut) carries the sheet's.
	if op.PositionSeconds != nil && op.ID == e.ID {
		pos, src = *op.PositionSeconds, "op"
	}
	last := 0
	endedAt := ""
	if e.Ended != nil {
		last, endedAt = e.Ended.LastBlock, e.Ended.At
	}
	rec, err := noVerdictRecord(ls.ownerID, env.computer, "sheet:"+ls.sheetID, claimID, op.OpID, e.ID, reason, string(outcome), last, endedAt, pos, src, now)
	if err != nil {
		return err
	}
	if _, err := env.led.AppendCommitmentRecord(ctx, ls.ownerID, env.computer, rec); err != nil {
		return fmt.Errorf("no-verdict record: %w", err)
	}
	return nil
}

// noVerdictRecord builds an autoradio.no_verdict notice: the forecast was not
// settled, and why. It is not a settlement and carries no outcome of the
// forecast; the listener's later op can still settle the claim.
func noVerdictRecord(ownerID, computer, runID, claimID, opID, entryID, reason, outcome string, last int, endedAt string, pos float64, posSource string, now time.Time) (types.CommitmentRecord, error) {
	body, _ := json.Marshal(map[string]any{
		"claim_id": claimID, "op_id": opID, "entry_id": entryID, "reason": reason, "outcome": outcome,
		"last_block": last, "position_seconds": pos, "position_source": posSource, "ended_at": endedAt,
	})
	label, ok := types.AutoradioKindLabel(kindNoVerdict)
	if !ok {
		return types.CommitmentRecord{}, errors.New("autoradio.no_verdict is not in the record table")
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: noVerdictRecordID(claimID, opID), Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Directive: &types.CommitmentDirective{
			Subtype: types.CommitmentDirectiveNotice, Objective: "no verdict (" + reason + ") for " + entryID, Body: string(body),
		},
		Provenance: types.CommitmentProvenance{
			AgentID: autoradioDesk + ":" + ownerID, ModelID: autoradioDesk, RunID: runID,
			CommittedAt: now.UTC().Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings(),
		},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: computer, RelatedIDs: []string{claimID},
		SubjectRefs: map[string]string{
			"desk": autoradioDesk, "record_kind": kindNoVerdict, "claim_id": claimID, "op_id": opID, "entry_id": entryID,
			"reason": reason, "outcome": outcome, "last_block": strconv.Itoa(last),
			"position_seconds": strconv.FormatFloat(pos, 'f', -1, 64),
		},
	}, nil
}

// settleNow is the arrival time an op is judged against; a variable so a test
// can move the clock past a deadline.
var settleNow = time.Now
