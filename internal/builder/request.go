package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Request names the app-layer installable to build and the booted base it
// must resolve against.
type Request struct {
	// Installable is a flake app-layer installable, e.g. ".#autoputer" or
	// ".#app-layer". It is built/evaluated on the host.
	Installable string
	// ResultLink is the GC-root symlink nix build writes outside the repo.
	ResultLink string
	// BaseManifestPath is the deployed guest-image-manifest file whose
	// digest is the base identity (contract=choir-guest-image-v1).
	BaseManifestPath string
	// BaseStoreDiskPath is the deployed storedisk.erofs; its sha256 is the
	// second base-identity leg.
	BaseStoreDiskPath string
	// BaseStoreDiskBasePath is the layered image's base layer
	// (guest-bases/<base_id>/storedisk-base.erofs). When set,
	// BaseStoreDiskPath is the app layer and the identity is the per-layer
	// pair: the app layer's sha256 and the base's id (never re-read: a base is
	// content-keyed by its store hash and never rewritten). Empty for a
	// single-image base, which is hashed whole as before.
	BaseStoreDiskBasePath string
	// OutDir is where the exported closure blob + evidence receipt land.
	OutDir string
	// CodeCommit is the repo commit the installable was built from. When
	// SourceDir is set the builder DERIVES it (git rev-parse HEAD) and
	// this field is ignored — S2-c makes the builder the authority on what
	// it built; a caller-supplied commit is not provenance.
	CodeCommit string
	// SourceDir is the checkout the installable is evaluated against. The
	// builder derives CodeCommit from it and records whether the tree was
	// dirty; when empty the caller's CodeCommit is used with
	// provenance=caller.
	SourceDir string
	// SourcePatch is a unified-diff file applied to a detached worktree at
	// BaseCommit (or CodeCommit) before nix evaluates. This is the S2-f
	// self-dev→builder join: capsules ship source patches, not binaries.
	SourcePatch string
	// SourcePatchBaseCommit is the commit the patch was authored against
	// (the capsule's pinned source snapshot). Required when SourcePatch is
	// set; the builder checks out exactly this commit before applying.
	SourcePatchBaseCommit string
	// BuildTimeout bounds the `nix build` step (default 60 minutes). The
	// caller's context bounds the whole build.
	BuildTimeout time.Duration
	// AdmitPatchedTree decides whether the committed patched tree may be
	// evaluated. It sees the changes git actually committed (paths and
	// modes, read in the clone before nix runs); its error stops the build
	// and is wrapped, so a caller recovers its own type with errors.As.
	// Links and git metadata are refused before it is called.
	AdmitPatchedTree func([]PatchedChange) error
}

// parseBaseManifest reads the Nix-generated guest-image-manifest and returns
// its store-path bindings and the fields needed for base identity.
func parseBaseManifest(path string) (commit string, storePaths []string, digest string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, "", fmt.Errorf("read base manifest: %w", err)
	}
	digest = digestSHA256Hex(data)
	var seen map[string]bool
	seen = map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if k == "build_commit" {
			commit = strings.TrimSpace(v)
		}
		if storePathRe.MatchString(strings.TrimSpace(v)) {
			v = strings.TrimSpace(v)
			if !seen[v] {
				seen[v] = true
				storePaths = append(storePaths, v)
			}
		}
	}
	sort.Strings(storePaths)
	return commit, storePaths, digest, nil
}

var baseIDPattern = regexp.MustCompile(`^[0-9a-df-np-sv-z]{32}$`)

// baseDiskIdentity is the store-disk leg of the base identity. A single image
// (basePath empty) is the sha256 of the whole file. A layered image is the
// app layer's sha256 plus the base's id, read from its <base_id> directory so
// the multi-GB base is not read on every build; a base that does not sit in
// such a directory is hashed whole.
func baseDiskIdentity(diskPath, basePath string) (disk, baseID, baseSHA string, err error) {
	disk, err = fileSHA256(diskPath)
	if err != nil {
		return "", "", "", err
	}
	if basePath == "" {
		return disk, "", "", nil
	}
	if id := filepath.Base(filepath.Dir(basePath)); baseIDPattern.MatchString(id) {
		return disk, id, "", nil
	}
	baseSHA, err = fileSHA256(basePath)
	if err != nil {
		return "", "", "", err
	}
	return disk, "", baseSHA, nil
}

// fileSHA256 streams a file's sha256.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Build resolves the base identity, builds the app-layer installable, diffs
// its runtime closure against the base store paths, exports only the
// base-absent paths, and writes an evidence receipt. It returns the full
// result; the exported blob carries exactly what the guest must materialize
// at its store paths on the data disk.
func Build(ctx context.Context, req Request) (*ClosureResult, error) {
	if req.Installable == "" || req.BaseManifestPath == "" || req.BaseStoreDiskPath == "" || req.OutDir == "" {
		return nil, fmt.Errorf("builder: installable, base manifest, base store disk, and out dir are required")
	}
	if req.ResultLink == "" {
		req.ResultLink = filepath.Join(req.OutDir, "builder-result")
	}
	if err := os.MkdirAll(req.OutDir, 0o700); err != nil {
		return nil, err
	}

	// 1. Base identity = guest-image-manifest digest + storedisk digest +
	//    the manifest's store-path bindings.
	commit, basePaths, manifestDigest, err := parseBaseManifest(req.BaseManifestPath)
	if err != nil {
		return nil, err
	}
	diskDigest, baseID, baseDigest, err := baseDiskIdentity(req.BaseStoreDiskPath, req.BaseStoreDiskBasePath)
	if err != nil {
		return nil, fmt.Errorf("digest base store disk: %w", err)
	}
	base := BaseIdentity{
		Contract:                 "choir-guest-image-v1",
		BuildCommit:              commit,
		GuestImageManifestDigest: manifestDigest,
		StoreDiskSHA256:          diskDigest,
		StoreDiskBaseID:          baseID,
		StoreDiskBaseSHA256:      baseDigest,
		StorePaths:               basePaths,
	}
	baseSet := map[string]bool{}
	for _, p := range basePaths {
		baseSet[p] = true
		// The manifest names only the base's top-level store paths. Their
		// runtime closures (glibc, icu, ...) are in the booted EROFS store
		// too; when the host store holds them (it built the image), count
		// them as base so the exported delta is only what the base lacks.
		// Best effort: an unknown path keeps the larger, still-correct delta.
		if closure, closureErr := pathInfoClosure(ctx, p); closureErr == nil {
			for _, q := range closure {
				baseSet[q] = true
			}
		}
	}
	// S2-c: the builder is the authority on what it built. Derive the code
	// commit from the source checkout rather than trusting the caller.
	codeCommit := req.CodeCommit
	codeCommitSource := "caller"
	sourceDirty := false
	if req.SourceDir != "" {
		derived, dirty, err := deriveSourceCommit(ctx, req.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("builder: derive source commit: %w", err)
		}
		codeCommit = derived
		codeCommitSource = "derived"
		sourceDirty = dirty
	}
	installable := req.Installable
	patchSHA, patchBase, flakeLock := "", "", ""
	if req.SourcePatch != "" {
		// S2-f: the release is built from the patch's pinned base plus the
		// diff, never the caller's live tree. Worktree-isolate the patch,
		// commit it to a synthetic rev, and evaluate that rev through
		// git+file: so the flake's self.rev and the build receipt agree.
		patchBase = req.SourcePatchBaseCommit
		if patchBase == "" {
			return nil, fmt.Errorf("builder: source patch requires a base commit")
		}
		patchedRef, patchedCommit, sha, cleanup, patchErr := patchedFlakeRef(ctx, req.SourceDir, req.SourcePatch, req.Installable, patchBase, req.AdmitPatchedTree)
		if patchErr != nil {
			return nil, fmt.Errorf("builder: apply source patch: %w", patchErr)
		}
		defer cleanup()
		installable = patchedRef
		patchSHA = sha
		if repo, _, ok := strings.Cut(strings.TrimPrefix(patchedRef, "git+file://"), "#"); ok {
			if lock, lockErr := fileSHA256(filepath.Join(repo, "flake.lock")); lockErr == nil {
				flakeLock = lock
			}
		}
		codeCommit, codeCommitSource, sourceDirty = patchedCommit, "derived-patch", false
	}
	// 2. Build/eval the app-layer installable.
	outputPath, err := buildResult(ctx, installable, req.ResultLink, req.BuildTimeout)
	if err != nil {
		return nil, err
	}
	drvPath, err := evalDrvPath(ctx, installable)
	if err != nil {
		return nil, err
	}
	closure, err := pathInfoClosure(ctx, outputPath)
	if err != nil {
		return nil, err
	}

	// 3. App-layer delta = closure paths NOT already in the base. The guest
	//    must materialize exactly these; base-present paths resolve in place.
	var delta []string
	for _, p := range closure {
		if !baseSet[p] {
			delta = append(delta, p)
		}
	}

	// 4. Export the delta closure to a single narchive blob.
	blobPath := filepath.Join(req.OutDir, "app-layer-closure.nar")
	exportedDigest, err := exportClosure(ctx, delta, blobPath)
	if err != nil {
		return nil, err
	}

	res := &ClosureResult{
		Base:                  base,
		RuntimePath:           outputPath,
		ClosurePaths:          delta,
		ExportedDigest:        exportedDigest,
		ExportedPath:          blobPath,
		DerivationPath:        drvPath,
		OutputPath:            outputPath,
		CodeCommit:            codeCommit,
		CodeCommitSource:      codeCommitSource,
		SourceDirty:           sourceDirty,
		SourcePatchSHA256:     patchSHA,
		SourcePatchBaseCommit: patchBase,
		FlakeLockSHA256:       flakeLock,
		StoreSchemaVersion:    storeschema.Version,
		BuiltAt:               nowRFC3339(),
	}
	if err := res.WriteReceipt(filepath.Join(req.OutDir, "builder-receipt.json")); err != nil {
		return nil, err
	}
	return res, nil
}
