package pkgrelease

import (
	"context"
	"fmt"
	"strings"
)

// Install (docs/design/package-install-2026-11-04.md): an owner installs a
// published package release on their own computer. The computer re-checks
// the release content itself (VerifyContent) and plans the change against
// its own source tree (PlanInstall). The change then goes through the
// self-development apply path; nothing here writes to the repository.

// VerifyContent runs the content checks of VerifyHost that need no parent:
// the tree_patch bytes are the record's, applied to the empty tree they are
// confined to the root and make the record's tree there, they add no
// secret, and the manifest validates and is the record's summary.
func VerifyContent(ctx context.Context, rec Record, treePatch []byte) error {
	rec = rec.Normalize()
	if err := rec.Validate(); err != nil {
		return err
	}
	if err := VerifyBlob(rec.Source.TreePatch, treePatch); err != nil {
		return err
	}
	s, err := newScratch(ctx, "")
	if err != nil {
		return fmt.Errorf("pkgrelease: scratch repository: %w", err)
	}
	defer s.close()
	_, err = s.verifiedFull(ctx, rec, treePatch)
	return err
}

// verifiedFull applies the tree patch to the empty tree in s and checks the
// content (VerifyContent); it returns the release's full-path tree.
func (s *scratch) verifiedFull(ctx context.Context, rec Record, treePatch []byte) (string, error) {
	root := rec.Source.Root
	full, err := s.apply(ctx, EmptyTree, treePatch)
	if err != nil {
		return "", refuse(ReasonInvalid, "tree_patch does not apply to the empty tree: %v", err)
	}
	if err := s.confined(ctx, full, root); err != nil {
		return "", err
	}
	tree, present, err := subtreeIn(ctx, s.dir, s.env, full, root)
	if err != nil || !present || tree != rec.Source.Tree {
		return "", refuse(ReasonTreeMismatch, "tree_patch makes %q at %s, the record names %s", tree, root, rec.Source.Tree)
	}
	if kinds := AddedSecrets(treePatch); len(kinds) > 0 {
		return "", refuse(ReasonSecretFound, "tree_patch adds secret content (%s)", strings.Join(kinds, ", "))
	}
	raw, err := gitRun(ctx, s.dir, s.env, nil, "cat-file", "blob", full+":"+root+"/choir-package.json")
	if err != nil || len(raw) > maxManifestBytes {
		return "", refuse(ReasonManifestInvalid, "the release holds no readable %s/choir-package.json", root)
	}
	summary, version, err := CheckManifest(rec.Package.Name, raw, treeFiles(ctx, s.dir, s.env, full, root))
	if err != nil {
		return "", err
	}
	if summary != rec.Manifest || version != rec.Package.ManifestVersion {
		return "", refuse(ReasonManifestInvalid, "the record's manifest summary %+v (version %s) is not the bytes' %+v (version %s)", rec.Manifest, rec.Package.ManifestVersion, summary, version)
	}
	return full, nil
}

// InstallPlan is an install (or uninstall) planned against one commit of a
// computer's own source tree.
type InstallPlan struct {
	// Local is the subtree at the package root in the commit (L);
	// LocalPresent is false when the commit holds no such directory.
	Local        string
	LocalPresent bool
	// Patch turns the commit's tree into one whose package root holds the
	// release's tree (an uninstall: nothing), and changes nothing else. It
	// is a binary diff at full paths with a/ and b/ prefixes, so git apply
	// runs it from the repository root. Empty when there is nothing to do.
	Patch []byte
}

// PlanInstall verifies the release content (VerifyContent) and computes the
// patch from the package root at head in repo to the release's tree, or to
// nothing when uninstall is set. Every object it writes goes to a throwaway
// directory; repo's objects are a read-only alternate. Whether the computer
// may replace what it holds (L) is the caller's decision.
func PlanInstall(ctx context.Context, repo, head string, rec Record, treePatch []byte, uninstall bool) (InstallPlan, error) {
	rec = rec.Normalize()
	if err := rec.Validate(); err != nil {
		return InstallPlan{}, err
	}
	if err := VerifyBlob(rec.Source.TreePatch, treePatch); err != nil {
		return InstallPlan{}, err
	}
	if !gitObjectID.MatchString(head) {
		return InstallPlan{}, fmt.Errorf("pkgrelease: %q is not a commit id", head)
	}
	s, err := newScratch(ctx, repo)
	if err != nil {
		return InstallPlan{}, err
	}
	defer s.close()
	full, err := s.verifiedFull(ctx, rec, treePatch)
	if err != nil {
		return InstallPlan{}, err
	}
	root := rec.Source.Root
	local, present, err := subtreeIn(ctx, s.dir, s.env, head, root)
	if err != nil {
		return InstallPlan{}, err
	}
	plan := InstallPlan{Local: local, LocalPresent: present}
	localFull := EmptyTree
	if present {
		if localFull, err = s.prefixed(ctx, root, local); err != nil {
			return InstallPlan{}, err
		}
	}
	target := full
	if uninstall {
		target = EmptyTree
	}
	if plan.Patch, err = s.diff(ctx, localFull, target); err != nil {
		return InstallPlan{}, err
	}
	return plan, nil
}
