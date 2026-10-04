// Package builder is the S2 host-side builder substrate: it evaluates an
// app-layer Nix closure against a booted guest base image and emits the
// closure, its derivation/input evidence, and the base-image identity it
// resolves against. It is the authority boundary the updater's release
// staging consumes; the guest itself never runs nix and never gets a
// writable store.
//
// The builder runs on the host (Node B), where `nix` is present and the
// store is writable. A privileged-builder-capsule was rejected because
// every capsule is NS_USER+NEWNET confined with a forced read-only
// /nix/store (docs/problems/s2-builder-substrate-2026-10-04.md).
package builder

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// BaseIdentity is the booted guest base image the app closure resolves
// against. GuestImageManifestDigest is sha256 of the Nix-generated
// guest-image-manifest (contract=choir-guest-image-v1), which binds the
// base's autoputer/updater/capsuleBroker/kernel store paths. StorePaths is
// the full store-path closure of the base runtime packages; every path an
// app closure needs must already exist in this set.
type BaseIdentity struct {
	Contract                 string   `json:"contract"`
	BuildCommit              string   `json:"build_commit"`
	GuestImageManifestDigest string   `json:"guest_image_manifest_digest"`
	StoreDiskSHA256          string   `json:"store_disk_sha256"`
	StorePaths               []string `json:"store_paths"`
}

// ClosureResult is the builder's emitted artifact set: the base image the
// closure resolves against, the app closure's runtime store paths, a
// nix-store --export blob of exactly the paths not already in the base, and
// the derivation/input evidence.
type ClosureResult struct {
	Base           BaseIdentity `json:"base"`
	RuntimePath    string       `json:"runtime_path"`
	ClosurePaths   []string     `json:"closure_paths"`
	ExportedDigest string       `json:"exported_digest"`
	ExportedPath   string       `json:"exported_path"`
	DerivationPath string       `json:"derivation_path"`
	OutputPath     string       `json:"output_path"`
	CodeCommit     string       `json:"code_commit"`
	BuiltAt        string       `json:"built_at"`
	// StoreSchemaVersion is the guest persistent-store schema epoch the
	// builder itself was compiled against (store.StoreSchemaVersion). It is
	// exact when the builder binary and the built source come from the same
	// commit — the normal path — and a recorded approximation otherwise
	// (named edge: a cross-commit patch build must carry the source's own
	// epoch, S2-c/S2-f carry that join).
	StoreSchemaVersion uint64 `json:"store_schema_version,omitempty"`
	// CodeCommitSource records how CodeCommit was established: "derived"
	// (git rev-parse HEAD inside SourceDir — S2-c provenance) or "caller"
	// (trusted input, kept only for callers that cannot supply a checkout).
	CodeCommitSource string `json:"code_commit_source,omitempty"`
	// SourceDirty is true when SourceDir carried uncommitted changes at
	// build time; recorded so the release's provenance is honest about
	// reproducibility.
	SourceDirty bool `json:"source_dirty,omitempty"`
}

// nixTimeout caps a single nix invocation. Eval and export on a warm store
// are seconds; a cold eval against the daemon can take a few minutes.
const nixTimeout = 10 * time.Minute

var storePathRe = regexp.MustCompile(`^/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-[A-Za-z0-9+._?=-]+$`)

// runNix invokes a nix subcommand and returns trimmed stdout. Nix 2.x puts
// store/path-info/build under the `nix` CLI.
func runNix(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, nixTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nix", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nix %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// evalOutputPath resolves a flake installable to its store output path
// without building (assumes already-built or substitutable).
func evalOutputPath(ctx context.Context, installable string) (string, error) {
	out, err := runNix(ctx, "eval", "--raw", installable+".outPath")
	if err != nil {
		return "", fmt.Errorf("eval %s.outPath: %w", installable, err)
	}
	if !storePathRe.MatchString(out) {
		return "", fmt.Errorf("eval %s.outPath: invalid store path %q", installable, out)
	}
	return out, nil
}

// evalDrvPath resolves a flake installable to its .drv path (input evidence).
func evalDrvPath(ctx context.Context, installable string) (string, error) {
	out, err := runNix(ctx, "eval", "--raw", installable+".drvPath")
	if err != nil {
		return "", fmt.Errorf("eval %s.drvPath: %w", installable, err)
	}
	if !storePathRe.MatchString(out) {
		return "", fmt.Errorf("eval %s.drvPath: invalid drv path %q", installable, out)
	}
	return out, nil
}

// buildResult builds a flake installable to a result symlink and returns the
// resolved store path. The result symlink lives outside the checkout.
func buildResult(ctx context.Context, installable, resultLink string) (string, error) {
	if _, err := runNix(ctx, "build", installable, "-o", resultLink); err != nil {
		return "", fmt.Errorf("build %s: %w", installable, err)
	}
	return evalOutputPath(ctx, installable)
}

// pathInfoClosure returns the runtime closure (all store paths) for a store
// path, sorted by dependency order.
func pathInfoClosure(ctx context.Context, storePath string) ([]string, error) {
	out, err := runNix(ctx, "path-info", "-r", storePath)
	if err != nil {
		return nil, fmt.Errorf("path-info -r %s: %w", storePath, err)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if storePathRe.MatchString(p) {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("path-info -r %s: empty closure", storePath)
	}
	return paths, nil
}

// exportClosure writes a `nix-store --export` blob containing exactly the
// given store paths to outPath and returns its sha256.
func exportClosure(ctx context.Context, paths []string, outPath string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("export requires at least one store path")
	}
	ctx, cancel := context.WithTimeout(ctx, nixTimeout)
	defer cancel()
	args := append([]string{"--export"}, paths...)
	cmd := exec.CommandContext(ctx, "nix-store", args...)
	// nix-store --export writes a narchive stream to stdout.
	exported, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nix-store --export: %w", err)
	}
	if err := writeFileAtomic(outPath, exported, 0o444); err != nil {
		return "", fmt.Errorf("write exported closure: %w", err)
	}
	return digestSHA256Hex(exported), nil
}
