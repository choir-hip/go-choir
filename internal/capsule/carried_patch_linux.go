//go:build linux

package capsule

import (
	"context"
	"encoding/hex"
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
