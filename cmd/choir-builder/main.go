// Command choir-builder is the S2 host-side builder entrypoint. It builds a
// Nix app-layer installable against the deployed guest base image, exports
// only the base-absent closure paths, and writes a builder evidence receipt
// the updater release staging consumes. Runs on the host (Node B) where nix
// is present; the guest never runs nix.
//
//	choir-builder -installable .#autoputer \
//	  -base-manifest /var/lib/go-choir/guest/guest-image-manifest \
//	  -base-storedisk /var/lib/go-choir/guest/storedisk.erofs \
//	  -out /var/lib/go-choir/builder/app-layer -commit <sha>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/yusefmosiah/go-choir/internal/builder"
)

func main() {
	installable := flag.String("installable", "", "flake app-layer installable, e.g. .#autoputer")
	baseManifest := flag.String("base-manifest", "", "path to the deployed guest-image-manifest")
	baseDisk := flag.String("base-storedisk", "", "path to the deployed storedisk.erofs")
	out := flag.String("out", "/var/lib/go-choir/builder", "output dir for closure blob + receipt")
	commit := flag.String("commit", "", "repo commit the installable was built from")
	sourceDir := flag.String("source-dir", "", "git checkout the installable is evaluated against; code_commit is derived from it (S2-c)")
	sourcePatch := flag.String("source-patch", "", "unified-diff patch applied to a worktree at the base commit before build (S2-f)")
	sourcePatchBase := flag.String("source-patch-base", "", "commit the source patch was authored against (required with -source-patch)")
	resultLink := flag.String("result-link", "", "nix build result symlink path (default <out>/builder-result)")
	flag.Parse()

	if *installable == "" || *baseManifest == "" || *baseDisk == "" {
		fmt.Fprintln(os.Stderr, "choir-builder: -installable, -base-manifest, -base-storedisk are required")
		flag.Usage()
		os.Exit(2)
	}

	res, err := builder.Build(context.Background(), builder.Request{
		Installable:           *installable,
		ResultLink:            *resultLink,
		SourceDir:             *sourceDir,
		SourcePatch:           *sourcePatch,
		SourcePatchBaseCommit: *sourcePatchBase,
		BaseManifestPath:      *baseManifest,
		BaseStoreDiskPath:     *baseDisk,
		OutDir:                *out,
		CodeCommit:            *commit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-builder: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("runtime_path=%s\nclosure_paths=%d\nexported=%s sha256=%s\nbase_manifest_sha256=%s\nbase_storedisk_sha256=%s\nreceipt=%s\n",
		res.RuntimePath, len(res.ClosurePaths), res.ExportedPath, res.ExportedDigest,
		res.Base.GuestImageManifestDigest, res.Base.StoreDiskSHA256, *out+"/builder-receipt.json")
	fmt.Printf("code_commit=%s (%s, dirty=%t)\n", res.CodeCommit, res.CodeCommitSource, res.SourceDirty)
	if res.SourcePatchSHA256 != "" {
		fmt.Printf("source_patch_sha256=%s base=%s\n", res.SourcePatchSHA256, res.SourcePatchBaseCommit)
	}
}
