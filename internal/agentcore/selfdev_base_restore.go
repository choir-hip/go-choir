package agentcore

// R3 base restore (docs/design/selfdev-apply-and-revert-2026-11-06.md, 3.5
// and the R3 build note). Reverting an operation whose prior state is a
// genesis or platform-update checkpoint, or an earlier base restore, has no
// earlier applied operation to roll back to: no verifier refs, no checkpoint
// and no route receipt to record the rollback against. The revert instead
// puts the booted base release back as `current`, with no self-development
// overlay, using the mechanics the M3 boot migration proved (stage the
// baseline, restage it under the store floor, record the startup manifest).
//
// The rollback row carries the base manifest digest as its target and the
// reverted operation as its subject; the applied tail binds the baseline
// release, not a verifier certificate: it appends rollback_applied with a
// payload naming the release, publishes no checkpoint and moves no route.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

const baseRestoreSchema = "choir.selfdev.base-restore.v1"

// baseRestoreResult is the rollback_applied payload of a base restore.
type baseRestoreResult struct {
	Schema             string `json:"schema"`
	Outcome            string `json:"outcome"`
	OperationID        string `json:"operation_id"`
	RevertedOperation  string `json:"reverted_operation"`
	TargetEventHead    string `json:"target_event_head"`
	ReleaseDigest      string `json:"release_digest"`
	Marker             string `json:"marker"`
	CodeRef            string `json:"code_ref"`
	ArtifactProgramRef string `json:"artifact_program_ref"`
}

// baseRestoreSelfDevelopmentOperation drives a base-restore rollback. afterCrash
// is the crash-boot settlement: it never swaps, it records the tail when the
// base is already served and closes the operation failed otherwise.
func (rt *Runtime) baseRestoreSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation, afterCrash bool) error {
	baselineRoot := rt.packageMigrationBaselineRoot()
	if baselineRoot == "" {
		return errors.New("materializer: base restore: no trusted baseline release root")
	}
	target, err := rt.servingBaselineManifest(ctx, operation.ComputerID, baselineRoot)
	if err != nil {
		return fmt.Errorf("materializer: base restore: the booted baseline manifest: %w", err)
	}
	if target.ContentDigest != operation.ReleaseDigest {
		return rt.settleSelfDevelopmentRefusal(ctx, operation, fmt.Errorf("%w: the booted base changed since the revert opened (%s, now %s)", updater.ErrApplyRefused, shortDigest(operation.ReleaseDigest), shortDigest(target.ContentDigest)))
	}
	note := ""
	rt.currentReleaseMu.Lock()
	current, currentErr := readCurrentManifestIdentity(rt.selfdevUpdaterRoot)
	if currentErr != nil || current.ContentDigest != target.ContentDigest {
		if afterCrash {
			rt.currentReleaseMu.Unlock()
			return rt.settleSelfDevelopmentRefusal(ctx, operation, fmt.Errorf("%w: interrupted by a restart before the base release was restored", updater.ErrApplyRefused))
		}
		if currentErr == nil && current.LayeringEntrypoint != "" {
			note = "the reverted release was layered: its binary runs until the next restart; the served files are the booted base now"
		}
		floor := rt.storeFloorPath()
		swapErr := updater.StageBaselineRelease(rt.selfdevUpdaterRoot, baselineRoot, target, floor)
		if swapErr == nil {
			swapErr = updater.RestagePinnedRelease(rt.selfdevUpdaterRoot, target.ContentDigest, floor)
		}
		if swapErr != nil {
			rt.currentReleaseMu.Unlock()
			if errors.Is(swapErr, updater.ErrStoreFloorRefused) || errors.Is(swapErr, updater.ErrPinnedReleaseCorrupt) {
				return rt.settleSelfDevelopmentRefusal(ctx, operation, fmt.Errorf("%w: the booted base cannot be served: %v", updater.ErrApplyRefused, swapErr))
			}
			return fmt.Errorf("materializer: base restore: %w", swapErr)
		}
		rt.recordStartupManifest(target)
	}
	rt.currentReleaseMu.Unlock()

	result := baseRestoreResult{
		Schema: baseRestoreSchema, Outcome: "applied", OperationID: operation.OperationID,
		RevertedOperation: selfdev.BaseRestoreSubject(operation), TargetEventHead: operation.BaseHead,
		ReleaseDigest: target.ContentDigest, Marker: target.Marker,
		CodeRef: target.CodeRef, ArtifactProgramRef: target.ArtifactProgramRef,
	}
	if err := rt.appendMaterializationAppliedEvent(ctx, operation, result, computerevent.EventRollbackApplied); err != nil {
		return err
	}
	payload, err := computerevent.CanonicalJSON(result)
	if err != nil {
		return err
	}
	finalHead, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || finalHead == nil {
		return errors.New("materializer: base restore: event head unavailable")
	}
	_, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateRolledBack, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = finalHead.DesiredEventHead, finalHead.EffectiveEventHead
		next.MaterializationReceipt = computerevent.DigestBytes(payload)
		if note != "" {
			next.TerminalError = strings.TrimSpace(note)
		}
		return nil
	})
	if err == nil {
		rt.reconcileSourceProjectionLogged(ctx, string(computerevent.EventRollbackApplied))
	}
	return err
}
