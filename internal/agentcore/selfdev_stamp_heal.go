package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// ErrStampHealRunning is returned when a stamp heal is already in flight on
// this runtime.
var ErrStampHealRunning = errors.New("self-development stamp heal: another heal is running")

// SelfDevelopmentStampHealProbe is the part of a replay probe the heal
// reports: whether the live and replayed projections agree, and the computer's
// replay eligibility.
type SelfDevelopmentStampHealProbe struct {
	LiveHeadSequence   uint64   `json:"live_head_sequence"`
	ReplayHeadSequence uint64   `json:"replay_head_sequence"`
	Equivalent         bool     `json:"equivalent"`
	Eligible           bool     `json:"eligible"`
	Reason             string   `json:"reason,omitempty"`
	Differences        []string `json:"differences,omitempty"`
}

// SelfDevelopmentStampHealReport is the heal's dry-run and apply answer.
type SelfDevelopmentStampHealReport struct {
	ComputerID      string                         `json:"computer_id"`
	Mode            string                         `json:"mode"` // "dry_run" or "applied"
	Probe           SelfDevelopmentStampHealProbe  `json:"probe"`
	RowsExamined    int                            `json:"rows_examined"`
	Rows            []selfdev.StampDivergence      `json:"rows"`
	MissingInReplay []string                       `json:"missing_in_replay"`
	Appended        int                            `json:"appended"`
	After           *SelfDevelopmentStampHealProbe `json:"after,omitempty"`
}

func stampHealProbe(report ReplayCompletenessReport) SelfDevelopmentStampHealProbe {
	probe := SelfDevelopmentStampHealProbe{
		Equivalent: report.Result.Equivalent(), Eligible: report.Eligibility.Eligible, Reason: report.Eligibility.Reason,
	}
	if report.LiveHead != nil {
		probe.LiveHeadSequence = report.LiveHead.Sequence
	}
	if report.ReplayHead != nil {
		probe.ReplayHeadSequence = report.ReplayHead.Sequence
	}
	for _, difference := range report.Result.Differences {
		probe.Differences = append(probe.Differences, difference.Key)
	}
	return probe
}

// HealSelfDevelopmentIntentStamps is the explicit operator act for a computer
// that stamped last_intent_error before RecordIntentError went on the tape
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md). It
// is reached only from the host-peer route; no boot step calls it.
//
// It replays the tape into a disposable store, selects the live
// self_development_operations rows whose last_intent_error or updated_at
// differ from the replayed row, and, when apply is set, appends one
// state-preserving projection batch carrying the live values
// (selfdev.Store.ApplyStampHeal, under the store's writeMu). verify re-probes
// after the append. A second run selects no rows and appends nothing.
func (rt *Runtime) HealSelfDevelopmentIntentStamps(ctx context.Context, computerID string, apply, verify bool) (SelfDevelopmentStampHealReport, error) {
	if rt == nil || rt.selfdevOperations == nil {
		return SelfDevelopmentStampHealReport{}, fmt.Errorf("%w: self-development operations are not configured", ErrReplayCompletenessUnavailable)
	}
	if !rt.selfdevStampHealMu.TryLock() {
		return SelfDevelopmentStampHealReport{}, ErrStampHealRunning
	}
	defer rt.selfdevStampHealMu.Unlock()
	computerID = strings.TrimSpace(computerID)

	probe, err := rt.openReplayProbe(ctx, computerID)
	if err != nil {
		return SelfDevelopmentStampHealReport{}, err
	}
	defer probe.close()
	replayOperations, err := selfdev.NewStore(probe.replayStore, probe.replayStore)
	if err != nil {
		return SelfDevelopmentStampHealReport{}, err
	}
	replayed, err := replayOperations.ListAll(ctx, computerID)
	if err != nil {
		return SelfDevelopmentStampHealReport{}, fmt.Errorf("stamp heal: read replayed operations: %w", err)
	}
	plan, err := rt.selfdevOperations.PlanStampHeal(ctx, computerID, replayed)
	if err != nil {
		return SelfDevelopmentStampHealReport{}, fmt.Errorf("stamp heal: read live operations: %w", err)
	}
	report := SelfDevelopmentStampHealReport{
		ComputerID: computerID, Mode: "dry_run", Probe: stampHealProbe(probe.report),
		RowsExamined: plan.RowsExamined, Rows: plan.Rows, MissingInReplay: plan.MissingInReplay,
	}
	if !apply {
		return report, nil
	}
	if hook := rt.selfdevStampHealAfterPlan; hook != nil {
		hook()
	}
	appended, err := rt.selfdevOperations.ApplyStampHeal(ctx, plan)
	if err != nil {
		return report, err
	}
	report.Mode, report.Appended = "applied", appended
	if verify && appended > 0 {
		after, err := rt.openReplayProbe(ctx, computerID)
		if err != nil {
			return report, fmt.Errorf("stamp heal: verify after append: %w", err)
		}
		defer after.close()
		summary := stampHealProbe(after.report)
		report.After = &summary
	}
	return report, nil
}
