package agentcore

// The guest side of a package decision (P1 of
// docs/design/package-operation-decision-2026-11-06.md): classify the frozen
// patch, ask the poster with the patch and a claimed platform base, and check
// that the decision names the draft content digest this computer derived
// itself, in place of the host build job a platform decision names.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// standingClassification is an operation's kind and, for a package, what the
// request and the decision check need.
type standingClassification struct {
	kind string
	name string
	// patchBase is the bundle's base commit (usually a projection commit in
	// the guest's repo); claimedBase the platform base the poster drafts over.
	patchBase, claimedBase string
	patch                  []byte
	sourcePatchSHA256      string
}

// standingClassify classifies the operation by its frozen patch with the
// bundle's effects in agreement (platform.ClassifyOperation). An operation
// with no source patch is a platform operation.
func (rt *Runtime) standingClassify(ctx context.Context, operation selfdev.Operation, rawBundle []byte) (standingClassification, error) {
	base, patch, err := rt.boundSourcePatch(operation)
	switch {
	case errors.Is(err, errNoSourcePatch):
		patch = nil
	case err != nil:
		return standingClassification{}, err
	}
	kind, name, err := platform.ClassifyOperation(patch, rawBundle)
	if err != nil {
		return standingClassification{}, err
	}
	out := standingClassification{kind: kind, name: name, patchBase: base, patch: patch}
	if kind != platform.ConsensusKindPackage {
		return out, nil
	}
	out.claimedBase = strings.ToLower(strings.TrimSpace(base))
	if stackBase, ok := sourceproj.StackBase(ctx, rt.releaseRepo(), base); ok {
		out.claimedBase = strings.ToLower(strings.TrimSpace(stackBase))
	}
	out.sourcePatchSHA256 = computerevent.DigestBytes(patch)
	return out, nil
}

// consensusDecisionKind is the kind a decision carries; a decision with no
// kind is a platform decision.
func consensusDecisionKind(decision *computerevent.Receipt) string {
	if decision != nil {
		if kind, _ := decision.KindFields["kind"].(string); kind == platform.ConsensusKindPackage {
			return platform.ConsensusKindPackage
		}
	}
	return platform.ConsensusKindPlatform
}

// verifyPackageDecisionDraft is the guest's decision check for a package
// decision: it names this package, patch and claimed base, and its draft
// content digest is the one this computer derives from the same patch in its
// own repository, on the parent the decision names. A poster that voted on
// another draft is refused.
func (rt *Runtime) verifyPackageDecisionDraft(ctx context.Context, c standingClassification, decision *computerevent.Receipt) error {
	if decision == nil || consensusDecisionKind(decision) != platform.ConsensusKindPackage {
		return fmt.Errorf("consensus decision is not a package decision")
	}
	block, _ := decision.KindFields["package"].(map[string]any)
	str := func(m map[string]any, key string) string { v, _ := m[key].(string); return v }
	if block == nil || str(block, "name") != c.name || str(block, "source_patch_sha256") != c.sourcePatchSHA256 || str(block, "claimed_platform_base") != c.claimedBase {
		return fmt.Errorf("consensus decision does not bind this package, source patch and platform base")
	}
	parent, _ := block["parent"].(map[string]any)
	var headID, headTree string
	if parent != nil && str(parent, "kind") == pkgrelease.ParentRelease {
		headID, headTree = str(parent, "release_id"), str(parent, "tree")
	}
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{
		Repo: rt.releaseRepo(), Name: c.name, Base: c.patchBase, Patch: c.patch, PlatformBase: c.claimedBase,
		HeadReleaseID: headID, HeadTree: headTree,
	})
	if err != nil {
		return fmt.Errorf("the package draft cannot be made on this computer: %w", err)
	}
	mine, err := source.Facts(c.name).Digest()
	if err != nil {
		return err
	}
	if named := str(block, "draft_digest"); named != mine {
		return fmt.Errorf("consensus decision names package draft %q, not the draft this computer derived (%s)", named, mine)
	}
	return nil
}
