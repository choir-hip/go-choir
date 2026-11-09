package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The resume side of the sandbox pause
// (docs/design/sandbox-pause-resume-2026-11-09.md, Decisions 6-9): the boot
// reconcile's decision, the resume as an ordinary Spawn seeded with the
// preserved upper, close-on-any-error, and the gate on the run's activation.

const (
	// capsuleResumeAdmissionTries bounds the host build reservation's retry.
	capsuleResumeAdmissionTries = 6
	capsuleResumeAdmissionRetry = 10 * time.Second
)

// reconcilePausedAssignment is the pause guard every entry to the
// assignment reconcile passes first (Decision 7). It reports true when the
// pause machinery owns the assignment and the ordinary reconcile must keep
// it and arm nothing:
//
//   - the capsule is paused in this process, or its pause is in progress:
//     the unpause machinery owns it;
//   - a resume is armed or in flight this boot;
//   - a planned platform-update boot found a spared (or same-boot residual)
//     pause record: one resume is armed.
//
// On a crash boot, or any boot whose marker is not a platform update, a
// spared record is deleted (residue torn down first) and the ordinary path
// closes the assignment "interrupted by a restart".
func (rt *Runtime) reconcilePausedAssignment(ctx context.Context, a types.EngineeringAssignment, crashBoot bool) bool {
	p := rt.capsulePauser()
	if p == nil {
		return false
	}
	capsuleID := strings.TrimSpace(a.Binding.CapsuleID)
	if capsuleID == "" {
		return false
	}
	rt.pauses.mu.Lock()
	resuming := rt.pauses.resuming[capsuleID]
	_, inMemory := rt.pauses.entries[capsuleID]
	rt.pauses.mu.Unlock()
	if resuming || inMemory {
		return true
	}
	look := rt.capsulePauseState(a)
	switch look.Where {
	case capsule.PauseLive:
		return true
	case capsule.PauseSpared, capsule.PauseResidual:
		_, planned := rt.BootWasPlannedRestart()
		if crashBoot || !planned || rt.bootRestart.Reason != PlannedRestartPlatformRefresh {
			if err := p.DiscardPause(ctx, capsuleID); err != nil {
				log.Printf("runtime: discard pause of capsule %s: %v", capsuleID, err)
			}
			return false
		}
		rt.armPausedResume(a)
		return true
	}
	return false
}

// armPausedResume arms the one resume of a paused capsule this boot. The
// resume runs in the background (the boot's Start must return), once per
// capsule; a second arm while it runs or after it consumed the record is a
// no-op (failure 9).
func (rt *Runtime) armPausedResume(a types.EngineeringAssignment) {
	capsuleID := strings.TrimSpace(a.Binding.CapsuleID)
	rt.pauses.mu.Lock()
	if rt.pauses.resuming == nil {
		rt.pauses.resuming = map[string]bool{}
	}
	if rt.pauses.resuming[capsuleID] {
		rt.pauses.mu.Unlock()
		return
	}
	rt.pauses.resuming[capsuleID] = true
	rt.pauses.resumeWG.Add(1)
	rt.pauses.mu.Unlock()
	go func() {
		defer rt.pauses.resumeWG.Done()
		defer func() {
			rt.pauses.mu.Lock()
			delete(rt.pauses.resuming, capsuleID)
			rt.pauses.mu.Unlock()
		}()
		rt.runPausedResume(a)
	}()
}

// waitPausedResumes waits for the resumes armed this boot to complete or
// close.
func (rt *Runtime) waitPausedResumes() { rt.pauses.resumeWG.Wait() }

func (rt *Runtime) pausedResumesInFlight() bool {
	rt.pauses.mu.Lock()
	defer rt.pauses.mu.Unlock()
	return len(rt.pauses.resuming) > 0
}

// closePausedAssignment closes an assignment whose resume failed for any
// reason: its pause residue is deleted and the assignment ends with the
// visible fate "interrupted by a restart" (Decision 8, failure 25).
func (rt *Runtime) closePausedAssignment(ctx context.Context, a types.EngineeringAssignment, cause error) {
	log.Printf("runtime: resume of paused capsule %s failed, closing assignment %s as interrupted by a restart: %v", a.Binding.CapsuleID, a.AssignmentID, cause)
	if p := rt.capsulePauser(); p != nil {
		if err := p.DiscardPause(ctx, a.Binding.CapsuleID); err != nil {
			log.Printf("runtime: discard pause of capsule %s: %v", a.Binding.CapsuleID, err)
		}
	}
	current, err := rt.store.GetEngineeringAssignment(ctx, a.Binding.OwnerID, a.Binding.ComputerID, a.AssignmentID, a.Binding.Attempt)
	if err != nil || current.Disposition.Terminal() {
		return
	}
	if err := rt.interruptAssignmentAtCrashBoot(ctx, current); err != nil {
		log.Printf("runtime: close assignment %s as interrupted by a restart: %v", a.AssignmentID, err)
	}
}

// runPausedResume is the resume (Decision 8): admission, validation,
// residue teardown, a seeded Spawn, the capability, and the release of the
// run. It never uses the bind saga (which would cancel a bound assignment).
func (rt *Runtime) runPausedResume(a types.EngineeringAssignment) {
	ctx := context.Background()
	p := rt.capsulePauser()
	exec := rt.assignedCapsule()
	if p == nil || exec == nil || rt.store == nil {
		return
	}
	capsuleID := strings.TrimSpace(a.Binding.CapsuleID)
	current, err := rt.store.GetEngineeringAssignment(ctx, a.Binding.OwnerID, a.Binding.ComputerID, a.AssignmentID, a.Binding.Attempt)
	if err != nil {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("reload the assignment: %w", err))
		return
	}
	a = current
	if a.Disposition.Terminal() {
		if err := p.DiscardPause(ctx, capsuleID); err != nil {
			log.Printf("runtime: discard pause of capsule %s: %v", capsuleID, err)
		}
		return
	}
	look := p.LookupPause(capsuleID)
	if look.Where == capsule.PauseConsumed && exec.HasCapsule(capsuleID) {
		return // a second resume wake in the same boot: the record is taken
	}
	if look.Err != nil || (look.Where != capsule.PauseSpared && look.Where != capsule.PauseResidual) {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("no usable pause record (%v)", look.Err))
		return
	}
	// A zero remaining deadline closes before any work.
	deadline := rt.assignedEngineeringBoundAt(ctx, a).Add(assignedEngineeringDeadline())
	remaining := deadline.Sub(rt.pauseTune.clock())
	if remaining <= 0 {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("the assignment deadline passed during the pause"))
		return
	}
	want := capsule.PauseRecord{CapsuleID: capsuleID, AssignmentID: a.AssignmentID, Attempt: a.Binding.Attempt, SubjectDigest: a.Binding.SubjectDigest}
	seed, err := p.PrepareResume(ctx, want)
	if err != nil {
		rt.closePausedAssignment(ctx, a, err)
		return
	}
	preflight, err := rt.sourcePreflighter().PreflightSourceSnapshot(ctx, a.Binding.SourceArtifactRef)
	if err != nil {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("stage the subject source: %w", err))
		return
	}
	if preflight.ArtifactRef != a.Binding.SourceArtifactRef ||
		"sha256:"+strings.TrimPrefix(preflight.SubjectDigest, "sha256:") != a.Binding.SubjectDigest {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("the subject drifted from the committed binding"))
		return
	}
	spec := seed.Record.Spec
	spec.SourceArtifactRef, spec.ExpectedSubjectDigest = preflight.ArtifactRef, preflight.SubjectDigest
	// Inputs and the carried patch are staged again as at bind.
	spec.CarriedPatchDir = carriedPatchDirForSpawn(rt.offerCarriedPatchForAssignment(ctx, a, preflight.ArtifactRef))
	inputsDir, removeInputs := rt.stageAssignmentInputs(ctx, a)
	spec.InputsDir = inputsDir
	spec.SeedUpper = seed.UpperDir
	retry := orDuration(rt.pauseTune.resumeRetry, capsuleResumeAdmissionRetry)
	var spawnErr error
	for try := 0; try < capsuleResumeAdmissionTries; try++ {
		_, spawnErr = exec.Spawn(ctx, spec)
		if spawnErr == nil || engineeringHostMemoryWait(a.AssignmentID, spawnErr) == nil {
			break
		}
		time.Sleep(retry)
	}
	removeInputs()
	if spawnErr != nil {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("spawn the resumed capsule: %w", spawnErr))
		return
	}
	opaque := deterministicDocumentAssignmentCapability(a.AssignmentID, a.Binding.Attempt)
	ttl := min(24*time.Hour, deadline.Sub(rt.pauseTune.clock()))
	if ttl <= 0 {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("the assignment deadline passed during the resume"))
		return
	}
	if _, err := exec.MintCapabilityHandle(a.BoundRunID, capsule.PerformerRole(), capsuleID, opaque, ttl, "implementation"); err != nil {
		rt.closePausedAssignment(ctx, a, fmt.Errorf("mint the capability: %w", err))
		return
	}
	if err := p.CompleteResume(seed); err != nil {
		log.Printf("runtime: complete resume of capsule %s: %v", capsuleID, err)
	}
	rt.pauses.mu.Lock()
	if rt.pauses.resumeNotes == nil {
		rt.pauses.resumeNotes = map[string]string{}
	}
	rt.pauses.resumeNotes[a.AssignmentID] = rt.pauseResumeNote(seed.Record)
	rt.pauses.mu.Unlock()
	rt.armAssignedEngineeringDeadline(a)
	log.Printf("runtime: capsule %s of assignment %s resumed after a platform update (upper %d files, %d bytes)", capsuleID, a.AssignmentID, seed.Record.UpperFiles, seed.Record.UpperBytes)
}

// pauseResumeNote is the one run-context turn the resumed run sees before
// its first turn (Decision 9).
func (rt *Runtime) pauseResumeNote(rec capsule.PauseRecord) string {
	var b strings.Builder
	b.WriteString("Your sandbox was paused for a platform update and has resumed. The cell that was running and the kernel state are gone; jobs that were running were interrupted (rerun them); /computer/files was read again and may differ; tool paths in /tmp, $HOME and $GOCACHE that name old store paths may be stale.")
	if now := rt.buildCommit(); rec.ReleaseCommit != "" && now != "" && rec.ReleaseCommit != now {
		b.WriteString(" The platform release changed (" + shortDigest(rec.ReleaseCommit) + " to " + shortDigest(now) + "); files the sandbox copied up from the root (for example under /etc) keep their old-release content.")
	}
	if rec.Spec.ClaudeDelegate {
		b.WriteString(" The Claude delegate process stopped with the restart; start it again if you need it.")
	}
	return b.String()
}

// takePauseResumeNote returns the resume note for a run's assignment, once.
func (rt *Runtime) takePauseResumeNote(rec *types.RunRecord) string {
	if rt == nil || rec == nil {
		return ""
	}
	id := metadataStringValue(rec.Metadata, "assignment_id")
	if id == "" {
		return ""
	}
	rt.pauses.mu.Lock()
	defer rt.pauses.mu.Unlock()
	note := rt.pauses.resumeNotes[id]
	delete(rt.pauses.resumeNotes, id)
	return note
}

// sweepSparedPausesAfterReconcile deletes every spared pause directory that
// no resume was armed for: no open assignment, a close, or a crash boot
// (failure 25).
func (rt *Runtime) sweepSparedPausesAfterReconcile() {
	p := rt.capsulePauser()
	if p == nil {
		return
	}
	keep := map[string]bool{}
	rt.pauses.mu.Lock()
	for id := range rt.pauses.resuming {
		keep[id] = true
	}
	rt.pauses.mu.Unlock()
	if err := p.DeleteSparedPauses(keep); err != nil {
		log.Printf("runtime: delete unresumed spared capsule directories: %v", err)
	}
}

// pausedActivationDeferral is the one choke point for a run's activation
// (r3): every path (initial dispatch, channel_message, coagent_result,
// owner_revision, the management rewarm of a runtime_restarted run) ends in
// ExecuteActivationSyncChecked, which asks here. An assigned engineering
// run whose capsule is paused, waiting for its resume or being resumed does
// not run a turn against an absent or frozen capsule: the occurrence stays
// unprocessed until the resume completes or closes.
func (rt *Runtime) pausedActivationDeferral(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || !assignedEngineeringRun(rec) || rt.capsulePauser() == nil {
		return nil
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, metadataStringValue(rec.Metadata, "assignment_id"), 1)
	if err != nil || assignment.Disposition.Terminal() {
		return nil
	}
	if rt.capsulePausedOrResuming(assignment) {
		return fmt.Errorf("%w: the sandbox of assignment %s is paused for a platform update; the run waits for its resume", ErrActivationOccurrenceMustRemainUnprocessed, assignment.AssignmentID)
	}
	return nil
}
