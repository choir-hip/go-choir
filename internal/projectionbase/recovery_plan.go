package projectionbase

import (
	"fmt"
)

// MaxRecoveryTailEvents is the largest (start, H] a recovery path may apply.
// A retained computer whose remaining work exceeds this bound refuses rather
// than lifetime-replaying the prefix. Publication must keep W close enough to
// H that the tail stays inside this budget.
const MaxRecoveryTailEvents uint64 = 10000

// RecoveryAction is the boot/refresh/rematerialize decision for one retained
// store, advertised watermark, and frozen target head.
type RecoveryAction int

const (
	// RecoveryRefuse is a typed failure: missing required base, stale
	// watermark, or an illegal retained store. Callers must not genesis.
	RecoveryRefuse RecoveryAction = iota
	// RecoveryGenesis is the only genesis path: empty store and no platform chain.
	RecoveryGenesis
	// RecoveryResume continues from a retained head that already descends from W
	// with a tail inside MaxRecoveryTailEvents.
	RecoveryResume
	// RecoveryInstall unpacks W into an empty storeDir, then the caller replays (W,H].
	RecoveryInstall
	// RecoveryRebase stage-installs W beside a non-empty store, swaps, then
	// the caller replays (W,H]. It never overwrites the live SQLite file in place.
	RecoveryRebase
)

func (a RecoveryAction) String() string {
	switch a {
	case RecoveryRefuse:
		return "refuse"
	case RecoveryGenesis:
		return "genesis"
	case RecoveryResume:
		return "resume"
	case RecoveryInstall:
		return "install"
	case RecoveryRebase:
		return "rebase"
	default:
		return "unknown"
	}
}

// RecoveryPlan is the pure local/W/H decision. Installation and replay happen
// after the plan is accepted; this type does not touch disk.
type RecoveryPlan struct {
	Action            RecoveryAction
	LocalSequence     uint64
	WatermarkSequence uint64
	TargetSequence    uint64
	StartSequence     uint64
	TailEvents        uint64
	Reason            string
}

func tailLen(start, target uint64) uint64 {
	if target <= start {
		return 0
	}
	return target - start
}

// PlanRecovery decides how a computer may reach H without lifetime-replaying.
//
//	empty + no chain → genesis
//	non-empty + no chain → refuse
//	chain + missing/stale W → refuse
//	empty + fresh W → install
//	local < W + fresh W → rebase
//	local ≥ W + tail inside bound → resume
func PlanRecovery(emptyStore bool, localSeq uint64, chainExists bool, watermarkSeq, targetSeq uint64) (RecoveryPlan, error) {
	plan := RecoveryPlan{
		LocalSequence:     localSeq,
		WatermarkSequence: watermarkSeq,
		TargetSequence:    targetSeq,
	}
	if !chainExists {
		// "Empty" for genesis is no projected events. store.Open always
		// creates marker files, so directory emptiness is not the signal.
		if emptyStore || localSeq == 0 {
			plan.Action = RecoveryGenesis
			plan.Reason = "no canonical chain; explicit new-computer bootstrap"
			return plan, nil
		}
		plan.Reason = "non-empty store has no canonical chain"
		return plan, fmt.Errorf("%w: %s", ErrBaseRefused, plan.Reason)
	}
	if targetSeq == 0 {
		plan.Reason = "recovery target sequence must be positive"
		return plan, fmt.Errorf("%w: %s", ErrBaseRefused, plan.Reason)
	}
	if watermarkSeq == 0 {
		plan.Reason = "required base is missing for an existing chain"
		return plan, fmt.Errorf("%w: %s", ErrBaseRefused, plan.Reason)
	}
	if watermarkSeq > targetSeq {
		plan.Reason = fmt.Sprintf("watermark %d is after target %d", watermarkSeq, targetSeq)
		return plan, fmt.Errorf("%w: %s", ErrBaseRefused, plan.Reason)
	}

	start := watermarkSeq
	if !emptyStore && localSeq >= watermarkSeq {
		start = localSeq
	}
	plan.StartSequence = start
	plan.TailEvents = tailLen(start, targetSeq)
	if plan.TailEvents > MaxRecoveryTailEvents {
		plan.Reason = fmt.Sprintf("recovery tail %d events exceeds %d; publish a fresher base (local=%d W=%d H=%d)", plan.TailEvents, MaxRecoveryTailEvents, localSeq, watermarkSeq, targetSeq)
		return plan, fmt.Errorf("%w: %s", ErrBaseRefused, plan.Reason)
	}
	if emptyStore {
		plan.Action = RecoveryInstall
		plan.Reason = "empty store installs verified base"
		return plan, nil
	}
	if localSeq < watermarkSeq {
		plan.Action = RecoveryRebase
		plan.Reason = "retained store is behind advertised watermark"
		return plan, nil
	}
	plan.Action = RecoveryResume
	plan.Reason = "retained store already descends from watermark"
	return plan, nil
}
