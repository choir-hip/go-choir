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
	"sort"
	"strings"
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
	// OutDir is where the exported closure blob + evidence receipt land.
	OutDir string
	// CodeCommit is the repo commit the installable was built from.
	CodeCommit string
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
	diskDigest, err := fileSHA256(req.BaseStoreDiskPath)
	if err != nil {
		return nil, fmt.Errorf("digest base store disk: %w", err)
	}
	base := BaseIdentity{
		Contract:                 "choir-guest-image-v1",
		BuildCommit:              commit,
		GuestImageManifestDigest: manifestDigest,
		StoreDiskSHA256:          diskDigest,
		StorePaths:               basePaths,
	}
	baseSet := map[string]bool{}
	for _, p := range basePaths {
		baseSet[p] = true
	}

	// 2. Build/eval the app-layer installable.
	outputPath, err := buildResult(ctx, req.Installable, req.ResultLink)
	if err != nil {
		return nil, err
	}
	drvPath, err := evalDrvPath(ctx, req.Installable)
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
		Base:               base,
		RuntimePath:        outputPath,
		ClosurePaths:       delta,
		ExportedDigest:     exportedDigest,
		ExportedPath:       blobPath,
		DerivationPath:     drvPath,
		OutputPath:         outputPath,
		CodeCommit:         req.CodeCommit,
		StoreSchemaVersion: storeschema.Version,
		BuiltAt:            nowRFC3339(),
	}
	if err := res.WriteReceipt(filepath.Join(req.OutDir, "builder-receipt.json")); err != nil {
		return nil, err
	}
	return res, nil
}
