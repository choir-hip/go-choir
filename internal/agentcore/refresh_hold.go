package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Refresh hold (docs/problems/self-opened-rounds-starve-the-deploy-refresh-2026-11-03.md).
// A busy guest refuses the host's planned-restart request, which is right
// while a capsule is live. But management opens the next self-development
// round as soon as it judges the last one, and admission binds the new
// capsule within seconds, so the refresh never finds the gap between rounds
// and the computer starts new work on a stale build for hours.
//
// The host's request is how the guest learns an update is waiting: there is
// no other route. The guest remembers the last one, and while it is fresh,
// engineering admission holds new requests (engineeringWaitRefresh). A held
// request stays open and unbound: it holds no capsule, so the next request
// finds the computer idle between rounds and writes the planned-restart
// marker, and the planned boot keeps and resumes the request
// (resumeQueuedEngineeringRequestsAtPlannedBoot). No restart semantics
// change: the refusals for a live capsule, an apply, a candidate awaiting its
// decision and running runs are untouched, and a crash boot still closes the
// request.
//
// Bounds: the hold lapses refreshAskFresh after the host's last request (the
// host forgot the computer: refreshed, failed or held), and one episode of
// the hold delays admissions at most refreshHoldMax (a refresh that does not
// land must not stop engineering); the admission that finds the bound spent
// goes ahead, and the next round boundary starts a new episode. While the
// ask is fresh the refusal says holds_new_work, and vmctl keeps asking once
// a minute instead of backing off, so the drained gap is seen at the next
// sweep.

const (
	// refreshAskFresh is how long one host request keeps the hold: several
	// one-minute sweeps, so a slow sweep does not drop it.
	refreshAskFresh = 5 * time.Minute
	// refreshHoldMax bounds how long admissions wait for one target, from
	// the first admission held for it.
	refreshHoldMax = 20 * time.Minute
)

// engineeringWaitRefresh is the Why of a request held for a pending deploy
// refresh.
const engineeringWaitRefresh = "refresh_pending"

// refreshHoldState is the guest's memory of the host's last refresh request.
// It is in memory: a restart ends it, and the host asks again.
type refreshHoldState struct {
	mu          sync.Mutex
	target      string
	lastAskedAt time.Time
	// askedSince is when the current episode's first ask arrived: how long
	// the update has been waiting.
	askedSince time.Time
	// heldSince is when the first admission was held for target; zero until
	// one is.
	heldSince time.Time
}

// refreshHoldNow is the clock the hold reads: admission's, so the request
// and the admission it holds agree.
func (rt *Runtime) refreshHoldNow() time.Time { return rt.engineeringAdmissionNow() }

// noteRefreshAsked records the host's request to restart onto target. A new
// target is a new refresh: its hold starts unspent.
func (rt *Runtime) noteRefreshAsked(target string, now time.Time) {
	target = strings.TrimSpace(target)
	if rt == nil || target == "" {
		return
	}
	h := &rt.refreshHold
	h.mu.Lock()
	defer h.mu.Unlock()
	// A new target, or an ask after the hold lapsed (the host stopped asking
	// for a while), starts a new episode with an unspent bound (review 1).
	if h.target != target || h.lastAskedAt.IsZero() || now.Sub(h.lastAskedAt) > refreshAskFresh {
		h.target, h.heldSince, h.askedSince = target, time.Time{}, now
	}
	h.lastAskedAt = now
}

// clearRefreshHold forgets the request: the host withdrew it.
func (rt *Runtime) clearRefreshHold() {
	if rt == nil {
		return
	}
	h := &rt.refreshHold
	h.mu.Lock()
	h.target, h.lastAskedAt, h.heldSince, h.askedSince = "", time.Time{}, time.Time{}, time.Time{}
	h.mu.Unlock()
}

// freshLocked reports whether the host's last ask still holds new work.
func (h *refreshHoldState) freshLocked(now time.Time) bool {
	return h.target != "" && !h.lastAskedAt.IsZero() && now.Sub(h.lastAskedAt) <= refreshAskFresh
}

// refreshHoldsNewWork reports, for the host's refusal, whether the guest
// holds new work for the refresh it was just asked for: the ask is fresh, so
// the next round boundary waits for the update.
func (rt *Runtime) refreshHoldsNewWork(now time.Time) bool {
	if rt == nil {
		return false
	}
	h := &rt.refreshHold
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.freshLocked(now)
}

// takeRefreshHold reports the target an admission at now waits for, starting
// the bound at the first admission it holds. The bound is per episode, not
// per target (review 1): an admission that finds it spent goes ahead and
// resets it, so that round binds on the current build, and the next round
// boundary holds again for at most refreshHoldMax. No work waits longer than
// the bound at a time, and every boundary gives the refresh another gap: a
// refresh that missed one (a 60-minute management activation, a candidate
// awaiting its decision) is not given up on for the rest of the release.
func (rt *Runtime) takeRefreshHold(now time.Time) (string, bool) {
	if rt == nil {
		return "", false
	}
	h := &rt.refreshHold
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.freshLocked(now) {
		return "", false
	}
	if !h.heldSince.IsZero() && now.Sub(h.heldSince) >= refreshHoldMax {
		log.Printf("runtime: refresh hold: the update to %s has not landed after holding new engineering for %s; this round starts on the current build and the next round boundary holds again",
			h.target, refreshHoldMax)
		h.heldSince = time.Time{}
		return "", false
	}
	if h.heldSince.IsZero() {
		h.heldSince = now
		log.Printf("runtime: refresh hold: new engineering waits for the update to %s (asked %s); held at most %s",
			h.target, h.lastAskedAt.Format(time.RFC3339), refreshHoldMax)
	}
	return h.target, true
}

// engineeringRefreshWaitRecordID is the one record of one assignment attempt
// waiting for one update.
func engineeringRefreshWaitRecordID(assignmentID string, attempt uint64, target string) string {
	return fmt.Sprintf("engineering-admission:%s:%d:waits-for-update:%s", assignmentID, attempt, target)
}

// engineeringRefreshWaitRecord tells the owner and management why the round
// has not started: it waits for the platform update, starts by itself after
// it, and nothing needs re-casting.
func engineeringRefreshWaitRecord(assignment types.EngineeringAssignment, target string, now time.Time) types.CommitmentRecord {
	stamp := now.UTC().Format(time.RFC3339Nano)
	return types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    engineeringRefreshWaitRecordID(assignment.AssignmentID, assignment.Binding.Attempt, target),
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis: fmt.Sprintf("engineering assignment %s is waiting for the update to %s: the host has a platform update for this computer, and the round starts on the updated build instead of the stale one. The computer takes the update between rounds (a planned restart), and the assignment stays open and starts by itself after it; if the update has not landed after %s it starts on the current build and the next round waits for the update again. Nothing needs re-casting.",
				assignment.AssignmentID, target, refreshHoldMax),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{
			Excerpt:    "waiting for the update to " + target,
			SourceRef:  assignment.AssignmentID,
			ObservedAt: stamp,
		},
		Provenance: types.CommitmentProvenance{
			AgentID:     engineeringHoldObserverAgentID,
			ContextRef:  assignment.Binding.TrajectoryID,
			CommittedAt: stamp,
		},
		Addressee:    persistentManagementAgentID(assignment.Binding.OwnerID),
		EvidenceRefs: []string{assignment.AssignmentID},
	}
}

// recordEngineeringRefreshWait writes the wait record. It is observational
// and idempotent: a ledger failure is logged, a repeat appends nothing.
func (rt *Runtime) recordEngineeringRefreshWait(ctx context.Context, assignment types.EngineeringAssignment, target string) {
	if rt == nil || rt.store == nil {
		return
	}
	rec := engineeringRefreshWaitRecord(assignment, target, rt.refreshHoldNow())
	if _, err := rt.store.AppendCommitmentRecord(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, rec); err != nil {
		log.Printf("runtime: engineering admission: refresh wait record for %s: %v", assignment.AssignmentID, err)
	}
}
