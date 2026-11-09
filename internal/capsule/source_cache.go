package capsule

// The pinned source a capsule mounts is a shared, content-addressed,
// read-only overlay lower (owner, 2026-11-09: "sandboxes should be cow when
// possible; near instant startup"). It used to be copied and digest-walked
// into every capsule's own source-lower (~741 ms on d@e). Now it is built once
// per subject digest into <state>/capsule-source-cache/<key>/, written under a
// temporary name and renamed into place so a crash never leaves a half tree,
// and every capsule on that digest mounts the same tree. A hit is a stat: the
// tree was copied, digest-verified against the durable binding's subject
// digest and made read-only at build, it is root-owned, and the capsule cannot
// write to a lower (copy-up goes to its upper), so it is trusted, not walked.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const (
	sourceCacheDirName = "capsule-source-cache"
	sourceCacheKeep    = 4
	sourceLowerRefName = "source-lower-ref"
)

// sourceCacheKey is the directory name of a subject digest.
func sourceCacheKeyFor(subjectDigest string) string {
	sum := sha256.Sum256([]byte(subjectDigest))
	return hex.EncodeToString(sum[:16])
}

// writeSourceLowerRef records, in the capsule's state directory, where its
// pinned source lower lives, for the readers of the pinned base bytes (the
// change list, the source patch).
func writeSourceLowerRef(base, lowerRoot string) error {
	return os.WriteFile(filepath.Join(base, sourceLowerRefName), []byte(lowerRoot), 0o644)
}

// pinnedSourceLowerRoot is the root of a capsule's pinned source lower
// (holding workspace/platform), given its merged dir: the shared cache tree
// its state names, else the capsule's own <state>/source-lower.
func pinnedSourceLowerRoot(mergedDir string) string {
	base := filepath.Join(mergedDir, "..")
	if raw, err := os.ReadFile(filepath.Join(base, sourceLowerRefName)); err == nil {
		if ref := strings.TrimSpace(string(raw)); ref != "" && filepath.IsAbs(ref) {
			return ref
		}
	}
	return filepath.Join(base, "source-lower")
}
