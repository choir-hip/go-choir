package platform

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The scheduled destroyed-computer tape sweep (station S7 of
// docs/definitions/choir-storage-solutions-2-2026-11-12.md): corpusd runs the
// sweep a person ran by hand, on its own, beside the hourly artifact
// collector.
//
// A pass is a dry run of the destroyed sweep, then the apply of that same
// manifest, repeated while an apply was capped. It calls the same dry-run and
// apply as the route, so every refusal, proof, cap and archive is the one a
// person gets: a computer that is registered, routed, owned, not quiet, or
// tombstone-refused stays untouched. The pass adds no setting. It follows
// PLATFORM_TAPE_SWEEP_MODE as the route does: off does nothing, dry-run
// classifies and reports, active also applies.
//
// Two things the pass does not do, on purpose:
//   - it never acknowledges unrecorded fork lineage. The destroyed proof lists
//     nothing without that acknowledgement, so until a lineage record exists
//     or the owner rules, a pass reports the computers it leaves
//     (blocked_by_fork_lineage) and applies nothing;
//   - it never passes no_archive. A destroyed computer's files are archived
//     on Node B (decision D1) before any unlink.
//
// Restarts follow CLAUDE.md: a crash does not resume a pass. The next pass
// starts a new dry run, which lists what is left; apply is idempotent and
// records its progress per computer in the tombstone.

const (
	// TapeSweepScheduleInterval is the cadence: destroyed computers are rare,
	// so a day is enough.
	TapeSweepScheduleInterval = 24 * time.Hour
	// TapeSweepScheduleInitialDelay is the wait after boot when no pass is
	// due later; the artifact collector uses 10 minutes, this one follows it.
	TapeSweepScheduleInitialDelay = 30 * time.Minute
	// TapeSweepScheduleRetry is the wait after a pass that found another
	// sweep running or failed outright.
	TapeSweepScheduleRetry = 6 * time.Hour
	// tapeSweepScheduleMaxRounds bounds the repeats of one pass. A hand run
	// needed five 8 GiB rounds for 26.89 GB; the rest waits for the next pass.
	tapeSweepScheduleMaxRounds = 16

	tapeSweepScheduleStateName = "tape-sweep-schedule.json"
)

// Pass outcomes.
const (
	TapeSweepPassOff        = "off"
	TapeSweepPassBusy       = "busy"
	TapeSweepPassError      = "error"
	TapeSweepPassRefused    = "refused"
	TapeSweepPassBlocked    = "blocked_by_fork_lineage"
	TapeSweepPassNothing    = "nothing"
	TapeSweepPassDryRunOnly = "dry_run_only"
	TapeSweepPassDone       = "done"
	TapeSweepPassStalled    = "stalled"
	TapeSweepPassRoundLimit = "round_limit"
)

// TapeSweepPassResult is what one scheduled pass did, summed over its rounds.
type TapeSweepPassResult struct {
	Outcome          string
	Mode             string
	Rounds           int
	RunIDs           []string
	Computers        int
	DestroyedFound   int
	Listed           int
	BlockedByLineage int
	Refused          string
	Deleted          int
	Bytes            int64
	DeferredByCap    int
	Errors           int
	ComputersDeleted int
	ComputersSkipped int
	Warnings         int
	Archives         []string
	Err              string
	Duration         time.Duration
}

// RunScheduledTapeSweep runs one pass and logs one line.
func (s *Service) RunScheduledTapeSweep(ctx context.Context) TapeSweepPassResult {
	started := time.Now()
	res := s.runScheduledTapeSweep(ctx)
	res.Duration = time.Since(started)
	log.Printf("tape sweep schedule: outcome=%s mode=%s rounds=%d computers=%d destroyed=%d listed=%d blocked_by_fork_lineage=%d deleted=%d bytes=%d deferred_by_cap=%d errors=%d computers_deleted=%d computers_skipped=%d warnings=%d archives=%d runs=%s refused=%q err=%q duration_ms=%d",
		res.Outcome, res.Mode, res.Rounds, res.Computers, res.DestroyedFound, res.Listed, res.BlockedByLineage, res.Deleted, res.Bytes, res.DeferredByCap, res.Errors, res.ComputersDeleted, res.ComputersSkipped, res.Warnings, len(res.Archives), strings.Join(res.RunIDs, ","), res.Refused, res.Err, res.Duration.Milliseconds())
	return res
}

func (s *Service) runScheduledTapeSweep(ctx context.Context) TapeSweepPassResult {
	policy, _ := s.tapeSweepPolicy()
	res := TapeSweepPassResult{Mode: policy.Mode}
	if s == nil || s.store == nil || s.artifactsRoot == "" {
		res.Outcome, res.Err = TapeSweepPassError, "platform store and artifacts root are required"
		return res
	}
	if policy.Mode == ArtifactGCModeOff {
		res.Outcome = TapeSweepPassOff
		return res
	}
	if !s.tapeSweep.passMu.TryLock() {
		res.Outcome = TapeSweepPassBusy
		return res
	}
	defer s.tapeSweep.passMu.Unlock()

	s.tapeSweep.mu.Lock()
	ack, maxDeletes, maxBytes, maxRounds := s.tapeSweep.schedForkAck, s.tapeSweep.schedMaxDeletes, s.tapeSweep.schedMaxBytes, s.tapeSweep.schedMaxRounds
	s.tapeSweep.mu.Unlock()
	if maxRounds <= 0 {
		maxRounds = tapeSweepScheduleMaxRounds
	}
	fail := func(err error) TapeSweepPassResult {
		if errors.Is(err, errTapeSweepBusy) {
			res.Outcome = TapeSweepPassBusy
		} else {
			res.Outcome, res.Err = TapeSweepPassError, err.Error()
		}
		return res
	}
	for res.Rounds < maxRounds {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		res.Rounds++
		dry, err := s.RunTapeSweep(ctx, TapeSweepRequest{Action: TapeSweepActionDryRun, Sweeps: []string{TapeSweepDestroyed}, AcceptUnrecordedForkLineage: ack})
		if err != nil {
			return fail(err)
		}
		res.RunIDs = append(res.RunIDs, dry.RunID)
		res.Computers, res.DestroyedFound = dry.Computers, dry.DestroyedComputers
		res.Listed, res.BlockedByLineage = dry.DestroyedListed, dry.DestroyedBlockedByLineage
		res.Warnings += len(dry.Warnings)
		if dry.DestroyedRefused != "" {
			res.Outcome, res.Refused = TapeSweepPassRefused, dry.DestroyedRefused
			return res
		}
		if dry.DestroyedListed == 0 {
			switch {
			case dry.DestroyedBlockedByLineage > 0:
				res.Outcome = TapeSweepPassBlocked
			case res.Rounds > 1:
				res.Outcome = TapeSweepPassDone
			default:
				res.Outcome = TapeSweepPassNothing
			}
			return res
		}
		if policy.Mode != ArtifactGCModeActive {
			res.Outcome = TapeSweepPassDryRunOnly
			return res
		}
		applied, err := s.RunTapeSweep(ctx, TapeSweepRequest{Action: TapeSweepActionApply, Sweeps: []string{TapeSweepDestroyed}, AcceptUnrecordedForkLineage: ack,
			RunID: dry.RunID, ManifestSHA256: dry.ManifestSHA256, MaxDeletes: maxDeletes, MaxBytes: maxBytes})
		if applied.Apply != nil {
			c := applied.Apply
			res.Deleted += c.Deleted
			res.Bytes += c.BytesDeleted
			res.Errors += c.Errors
			res.ComputersDeleted += c.ComputersDeleted
			res.ComputersSkipped += c.ComputersSkipped
			res.Archives = append(res.Archives, c.Archives...)
			res.DeferredByCap = c.DeferredByCap
		}
		res.Warnings += len(applied.Warnings)
		if err != nil {
			return fail(err)
		}
		if applied.DestroyedRefused != "" {
			res.Outcome, res.Refused = TapeSweepPassRefused, applied.DestroyedRefused
			return res
		}
		if res.DeferredByCap == 0 {
			res.Outcome = TapeSweepPassDone
			return res
		}
		if applied.Apply == nil || applied.Apply.Deleted == 0 || applied.Apply.Errors > 0 {
			res.Outcome = TapeSweepPassStalled
			return res
		}
	}
	res.Outcome = TapeSweepPassRoundLimit
	return res
}

// --- cadence -------------------------------------------------------------

type tapeSweepScheduleState struct {
	LastPass time.Time `json:"last_pass"`
}

func (s *Service) tapeSweepScheduleStatePath() string {
	return filepath.Join(s.tapeSweepLogDir(), tapeSweepScheduleStateName)
}

// markTapeSweepPass records that a pass finished at t, so a restart does not
// restart the day.
func (s *Service) markTapeSweepPass(t time.Time) {
	if err := os.MkdirAll(s.tapeSweepLogDir(), 0o750); err != nil {
		log.Printf("tape sweep schedule: state dir: %v", err)
		return
	}
	if err := writeJSONFileSync(s.tapeSweepScheduleStatePath(), tapeSweepScheduleState{LastPass: t.UTC()}); err != nil {
		log.Printf("tape sweep schedule: state write: %v", err)
	}
}

// tapeSweepScheduleDelay is the wait until the next pass: a day after the
// last finished pass, never sooner than the initial delay after boot, and the
// initial delay when there is no readable record.
func (s *Service) tapeSweepScheduleDelay(now time.Time) time.Duration {
	raw, err := os.ReadFile(s.tapeSweepScheduleStatePath())
	if err != nil {
		return TapeSweepScheduleInitialDelay
	}
	var st tapeSweepScheduleState
	if json.Unmarshal(raw, &st) != nil || st.LastPass.IsZero() {
		return TapeSweepScheduleInitialDelay
	}
	d := st.LastPass.Add(TapeSweepScheduleInterval).Sub(now)
	switch {
	case d < TapeSweepScheduleInitialDelay:
		return TapeSweepScheduleInitialDelay
	case d > TapeSweepScheduleInterval:
		return TapeSweepScheduleInterval
	}
	return d
}

// afterTapeSweepPass marks a finished pass and returns the wait before the
// next. A busy or failed pass is not marked and is retried sooner.
func (s *Service) afterTapeSweepPass(res TapeSweepPassResult, now time.Time) time.Duration {
	if res.Outcome == TapeSweepPassBusy || res.Outcome == TapeSweepPassError {
		return TapeSweepScheduleRetry
	}
	s.markTapeSweepPass(now)
	return TapeSweepScheduleInterval
}

// TapeSweepScheduler runs the pass on its cadence beside the GC runner.
type TapeSweepScheduler struct{ service *Service }

// NewTapeSweepScheduler returns the scheduler for the service.
func NewTapeSweepScheduler(service *Service) *TapeSweepScheduler {
	return &TapeSweepScheduler{service: service}
}

// Start runs passes until ctx is done. There is no setting: the mode the
// sweep already has decides what a pass may do.
func (r *TapeSweepScheduler) Start(ctx context.Context) {
	if r == nil || r.service == nil {
		return
	}
	go func() {
		timer := time.NewTimer(r.service.tapeSweepScheduleDelay(time.Now()))
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				res := r.service.RunScheduledTapeSweep(ctx)
				timer.Reset(r.service.afterTapeSweepPass(res, time.Now()))
			}
		}
	}()
}
