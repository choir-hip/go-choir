//go:build linux

package capsule

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// withSourceRefTree runs fn over the read-only tree a source artifact ref
// names: a persisted candidate subject in place, or a pinned git commit
// materialized into a temporary directory that is removed afterwards.
func (e *Executor) withSourceRefTree(ctx context.Context, ref string, fn func(root string) error) error {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, subjectArtifactPrefix):
		root, digest, err := e.subjectArtifactPath(ref)
		if err != nil {
			return err
		}
		actual, err := digestCanonicalSubjectTree(ctx, root)
		if err != nil || actual != digest {
			return fmt.Errorf("%w: %s", ErrSubjectArtifactUnavailable, ref)
		}
		return fn(root)
	case strings.HasPrefix(ref, "capsule-source-git:"):
		raw := strings.TrimPrefix(ref, "capsule-source-git:")
		separator := strings.Index(raw, ":sha256:")
		if separator <= 0 {
			return fmt.Errorf("capsule source artifact ref is invalid")
		}
		commit, digest := raw[:separator], raw[separator+len(":sha256:"):]
		if decoded, err := hex.DecodeString(commit); err != nil || (len(decoded) != 20 && len(decoded) != 32) {
			return fmt.Errorf("capsule source artifact commit is invalid")
		}
		temporary, err := os.MkdirTemp(e.artifactRoot(), ".carried-base-")
		if err != nil {
			return err
		}
		defer removePrivateTree(temporary)
		target := filepath.Join(temporary, "workspace", "platform")
		copied, err := copyImmutableCommitTreeTimed(ctx, e.sourceDir, commit, target, digest, nil)
		if err != nil || copied != digest {
			return fmt.Errorf("capsule source artifact is unavailable or corrupt: %s", ref)
		}
		return fn(target)
	}
	return fmt.Errorf("capsule source artifact ref is invalid")
}

// DeriveCarriedPatch renders the source change between a failed round's base
// (its pinned source ref) and the candidate tree it persisted, as inert data.
// It reads two content-addressed trees and writes nothing durable.
func (e *Executor) DeriveCarriedPatch(ctx context.Context, baseRef, candidateRef string) (CarriedPatch, error) {
	var out CarriedPatch
	err := e.withSourceRefTree(ctx, baseRef, func(baseRoot string) error {
		return e.withSourceRefTree(ctx, candidateRef, func(candidateRoot string) error {
			var diffErr error
			out, diffErr = DiffTreesToPatch(ctx, baseRoot, candidateRoot, CarriedPatchMaxBytes)
			return diffErr
		})
	})
	return out, err
}

// CheckCarriedPatchApplies reports whether a carried patch applies to the
// tree a retry's source ref names.
func (e *Executor) CheckCarriedPatchApplies(ctx context.Context, patch CarriedPatch, retryBaseRef string) error {
	return e.withSourceRefTree(ctx, retryBaseRef, func(root string) error {
		return CheckPatchAppliesToTree(ctx, patch.Patch, patch.Files, root)
	})
}

// ErrCapsuleNotSnapshottable is a capsule that is gone, or in a state its
// tree cannot be read in (destroying, destroyed).
var ErrCapsuleNotSnapshottable = errors.New("capsule cannot be snapshotted")

// SnapshotUnfinishedSourcePatch derives the source patch of a capsule that is
// about to be destroyed without a candidate: a round stopped by its
// activation budget, its deadline or a failed run. The capsule is quiesced
// first (operations drained, cgroup frozen), as Freeze's extraction does, so
// no process can change the tree while it is read; it is left frozen, since
// the caller destroys it next. It writes nothing: the patch is inert data
// for the caller to store. A Claude delegate capsule is scanned for its token
// as Freeze scans it.
func (e *Executor) SnapshotUnfinishedSourcePatch(ctx context.Context, capsuleID string) (CarriedPatch, error) {
	e.mu.RLock()
	caps := e.capsules[capsuleID]
	e.mu.RUnlock()
	if caps == nil {
		return CarriedPatch{}, fmt.Errorf("%w: %s is gone", ErrCapsuleNotSnapshottable, capsuleID)
	}
	caps.mu.RLock()
	state, claude, merged := caps.State, caps.Spec.ClaudeDelegate, caps.MergedDir
	caps.mu.RUnlock()
	switch state {
	case StateActive:
		if err := e.traceStep(caps.ID, "capsule.unfinished.quiesce", func() error { return caps.Quiesce(ctx) }); err != nil {
			return CarriedPatch{}, fmt.Errorf("freeze capsule before its unfinished snapshot: %w", err)
		}
	case StateFrozen:
	default:
		return CarriedPatch{}, fmt.Errorf("%w: %s is %s", ErrCapsuleNotSnapshottable, capsuleID, state)
	}
	if strings.TrimSpace(merged) == "" {
		return CarriedPatch{}, fmt.Errorf("%w: %s has no tree", ErrCapsuleNotSnapshottable, capsuleID)
	}
	changes, err := caps.Diff(ctx)
	if err != nil {
		return CarriedPatch{}, err
	}
	if claude {
		if err := e.refuseClaudeTokenInChanges(ctx, caps, changes); err != nil {
			return CarriedPatch{}, err
		}
	}
	var out CarriedPatch
	err = e.traceStep(caps.ID, "capsule.unfinished.patch", func() (patchErr error) {
		out, patchErr = UnfinishedPatchFromChanges(ctx, merged, changes, CarriedPatchMaxBytes)
		return patchErr
	})
	return out, err
}
