package agentcore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/packagepath"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// reasonPackageSeedMissing is the Cast refusal for an installed package whose
// tree patch was not kept (installed before installs kept it).
const reasonPackageSeedMissing = "package_seed_missing"

var castPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// packageSeedCommit is the commit a Cast that names an installed package
// casts its capsule from: the source projection's HEAD with the package's
// subtree replaced by the installed release's tree, so the package operation
// drafts as an update of the installed head (docs/design/
// forced-modularity-build-2026-11-04.md, Deferred item 2). A package that is
// not installed has no seed ("" and nil): it is a new package on HEAD. An
// install whose tree patch is gone refuses, naming the seed.
func (rt *Runtime) packageSeedCommit(ctx context.Context, ownerID, computerID, name, head string) (string, error) {
	if !castPackageName.MatchString(name) {
		return "", fmt.Errorf("the spec names package %q, which is not a package name", name)
	}
	installs, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return "", fmt.Errorf("package %s: the installs are unreadable: %w", name, err)
	}
	current := currentPackageInstalls(installs)[name]
	if current == nil {
		return "", nil
	}
	patch, err := rt.packageModules.ReadPatch(current.PatchSHA256)
	if errors.Is(err, pkgruntime.ErrNotFound) {
		return "", fmt.Errorf("%s: package %s is installed from release %s, whose tree patch %s this computer did not keep, so a capsule cannot be seeded with the installed version; install the release again to keep it",
			reasonPackageSeedMissing, name, current.ReleaseID, current.PatchSHA256)
	}
	if err != nil {
		return "", fmt.Errorf("package %s: the installed tree patch is unreadable: %w", name, err)
	}
	seed, err := sourceproj.SeedCommit(ctx, rt.releaseRepo(), head, packagepath.Root+"/"+name, patch)
	if err != nil {
		return "", fmt.Errorf("package %s: the seed commit cannot be made: %w", name, err)
	}
	return seed, nil
}

// seedPackagePreflight replaces the preflight of HEAD with the preflight of
// the package's seed commit when the package is installed.
func (rt *Runtime) seedPackagePreflight(ctx context.Context, ownerID, computerID, name string, head capsule.SourcePreflight) (capsule.SourcePreflight, error) {
	commit := capsule.SourceArtifactCommit(head.ArtifactRef)
	if commit == "" {
		return capsule.SourcePreflight{}, fmt.Errorf("the source %q names no commit to seed", head.ArtifactRef)
	}
	seed, err := rt.packageSeedCommit(ctx, ownerID, computerID, strings.TrimSpace(name), commit)
	if err != nil || seed == "" {
		return head, err
	}
	return rt.sourcePreflighter().PreflightSourceCommit(ctx, seed)
}
