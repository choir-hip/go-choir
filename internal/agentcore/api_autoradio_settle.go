package agentcore

// Station 2: the run sheet's op log is the air log. When the sheet's reducer
// applies an op that ends a desk entry which carries a forecast claim, the
// forecast is settled once, by rule, from that op. The settler is this path
// acting on the listener's device-keyed op; the programme route has no path
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
func (ls *liveSheet) settleEnded(ctx context.Context, env sheetEnv, before, after runsheet.RunSheet, op runsheet.Op, now time.Time) error {
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
		if err := ls.settleEntry(ctx, env, before.PositionSeconds, e, op, now); err != nil {
			return err
		}
	}
	return nil
}

func (ls *liveSheet) settleEntry(ctx context.Context, env sheetEnv, sheetPosition float64, e runsheet.Entry, op runsheet.Op, now time.Time) error {
	claimID := e.Forecast.ClaimID
	claim, err := env.led.GetCommitmentRecord(ctx, ls.ownerID, env.computer, claimID)
	if err != nil {
		return err
	}
	// The entry must be the segment the claim is about: a claim id on an entry
	// that is not that segment's own entry settles nothing.
	if claim == nil || !calibration.IsClaim(claim) || claim.Calibration.Agent != calibration.AutoradioAgent ||
		programmeEntryID(claimTag(*claim, "seg:")) != e.ID {
		log.Printf("autoradio settle: entry %s names claim %s, which is not that entry's forecast; nothing settled", e.ID, claimID)
		return nil
	}
	settledID := calibration.SettlementRecordID(claimID, types.CalibrationScorerRule, calibration.AutoradioHeardRule)
	prior, err := env.led.GetCommitmentRecord(ctx, ls.ownerID, env.computer, settledID)
	if err != nil {
		return err
	}
	if prior != nil {
		return nil // settled once; a settlement never changes
	}
	outcome := runsheet.OutcomeFinished
	if e.Ended != nil {
		outcome = e.Ended.Outcome
	} else if e.State == runsheet.StateSkipped {
		outcome = runsheet.OutcomeSkipped
	}
	opAt := now
	if e.Ended != nil && e.Ended.At != "" {
		if t, perr := time.Parse(time.RFC3339Nano, e.Ended.At); perr == nil && !t.After(now) {
			opAt = t
		}
	}
	if c, perr := time.Parse(time.RFC3339Nano, claim.Provenance.CommittedAt); perr == nil && opAt.Before(c) {
		opAt = c
	}
	reason := ""
	switch {
	case claim.Calibration.Claim != nil && claim.Calibration.Claim.Deadline != "" && deadlinePassed(claim.Calibration.Claim.Deadline, opAt):
		reason = noVerdictLate
	case outcome == runsheet.OutcomeLeft:
		reason = noVerdictLeft
	case outcome == runsheet.OutcomeFailed:
		reason = noVerdictFailed
	}
	if reason != "" {
		return ls.writeNoVerdict(ctx, env, *claim, sheetPosition, e, op, string(outcome), reason, now)
	}
	heard := outcome == runsheet.OutcomeFinished
	_, err = calibration.SettleAutoradio(ctx, env.led, ls.ownerID, env.computer, claimID, calibration.AutoradioHeardRule, heard, op.OpID, opAt)
	var bad *calibration.ValidationError
	if errors.As(err, &bad) {
		log.Printf("autoradio settle: claim %s refused: %v", claimID, err)
		return nil
	}
	return err
}

func deadlinePassed(deadline string, at time.Time) bool {
	d, err := time.Parse(time.RFC3339, deadline)
	return err == nil && at.After(d)
}

func (ls *liveSheet) writeNoVerdict(ctx context.Context, env sheetEnv, claim types.CommitmentRecord, sheetPosition float64, e runsheet.Entry, op runsheet.Op, outcome, reason string, now time.Time) error {
	pos, src := sheetPosition, "sheet"
	if op.PositionSeconds != nil {
		pos, src = *op.PositionSeconds, "op"
	}
	last := 0
	endedAt := ""
	if e.Ended != nil {
		last, endedAt = e.Ended.LastBlock, e.Ended.At
	}
	body, _ := json.Marshal(map[string]any{
		"claim_id": claim.RecordID, "op_id": op.OpID, "entry_id": e.ID, "reason": reason, "outcome": outcome,
		"last_block": last, "position_seconds": pos, "position_source": src, "ended_at": endedAt,
	})
	label, ok := types.AutoradioKindLabel(kindNoVerdict)
	if !ok {
		return errors.New("autoradio.no_verdict is not in the record table")
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: noVerdictRecordID(claim.RecordID, op.OpID), Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Directive: &types.CommitmentDirective{
			Subtype: types.CommitmentDirectiveNotice, Objective: "no verdict (" + reason + ") for " + e.ID, Body: string(body),
		},
		Provenance: types.CommitmentProvenance{
			AgentID: autoradioDesk + ":" + ls.ownerID, ModelID: autoradioDesk, RunID: "sheet:" + ls.sheetID,
			CommittedAt: now.UTC().Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings(),
		},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: env.computer, RelatedIDs: []string{claim.RecordID},
		SubjectRefs: map[string]string{
			"desk": autoradioDesk, "record_kind": kindNoVerdict, "claim_id": claim.RecordID, "op_id": op.OpID, "entry_id": e.ID,
			"reason": reason, "outcome": outcome, "last_block": strconv.Itoa(last),
			"position_seconds": strconv.FormatFloat(pos, 'f', -1, 64),
		},
	}
	if _, err := env.led.AppendCommitmentRecord(ctx, ls.ownerID, env.computer, rec); err != nil {
		return fmt.Errorf("no-verdict record: %w", err)
	}
	return nil
}

// settleNow is the arrival time an op is judged against; a variable so a test
// can move the clock past a deadline.
var settleNow = time.Now
