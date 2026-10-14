package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// ReleaseBinaryPath is the release-tree path of the runtime executable.
const ReleaseBinaryPath = "bin/autoputer"

// Release binary origins recorded on a frozen bundle.
const (
	// ReleaseBinaryCapsuleBuilt: the engineering desk built bin/autoputer
	// in the capsule and staged it in the release tree.
	ReleaseBinaryCapsuleBuilt = "capsule_built"
	// ReleaseBinaryReusedRunning: the change touches no binary input, so
	// Freeze staged this computer's running executable instead of a build
	// (docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option A).
	ReleaseBinaryReusedRunning = "reused_running"
)

// ReleaseBinaryProvenance says where a frozen release's bin/autoputer came
// from. A reused binary names the commit it was linked from and the commit
// the capsule's source was pinned to; Freeze only reuses when they are equal.
type ReleaseBinaryProvenance struct {
	Origin           string `json:"origin"`
	SHA256           string `json:"sha256"`
	RunningCommit    string `json:"running_commit,omitempty"`
	SourceBaseCommit string `json:"source_base_commit,omitempty"`
}

// RunningBinary identifies this process's executable: where to read it, its
// content hash, and the commit the linker stamped into it.
type RunningBinary struct {
	Path   string
	SHA256 string
	Commit string
}

// BinaryInputPath reports whether a workspace/platform-relative path is an
// input of the autoputer binary. It mirrors the flake's goServicesSrc filter
// (go.mod, go.sum, and non-test files under cmd/ and internal/) and adds
// conservative extras: vendor/, go.work*, the Nix build recipe, and any
// non-test .go file elsewhere (a new Go package needs one).
func BinaryInputPath(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "/")
	if rel == "" {
		return false
	}
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	base := path.Base(rel)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	switch rel {
	case "go.mod", "go.sum", "go.work", "go.work.sum", "flake.nix", "flake.lock":
		return true
	}
	switch strings.SplitN(rel, "/", 2)[0] {
	case "cmd", "internal", "vendor", "nix":
		return true
	}
	return path.Ext(base) == ".go"
}

const platformWorkspacePrefix = "workspace/platform/"

// SourceChangesTouchingBinary returns the sorted workspace/platform-relative
// paths among changes that are binary inputs. Paths outside the platform
// workspace (the release tree, caches, /tmp) are not source.
func SourceChangesTouchingBinary(changes []FileChange) []string {
	var out []string
	for _, change := range changes {
		p := strings.TrimPrefix(filepath.ToSlash(change.Path), "/")
		if !strings.HasPrefix(p, platformWorkspacePrefix) {
			continue
		}
		rel := strings.TrimPrefix(p, platformWorkspacePrefix)
		if BinaryInputPath(rel) {
			out = append(out, path.Clean(rel))
		}
	}
	sort.Strings(out)
	return out
}

// SourceArtifactCommit returns the commit of a capsule-source-git:<commit>:sha256:<digest>
// pin, or "" for any other source (a prior candidate artifact, a malformed ref).
func SourceArtifactCommit(ref string) string {
	const prefix = "capsule-source-git:"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	raw := strings.TrimPrefix(ref, prefix)
	separator := strings.Index(raw, ":sha256:")
	if separator <= 0 {
		return ""
	}
	commit := strings.ToLower(raw[:separator])
	if !isHexCommit(commit) {
		return ""
	}
	return commit
}

func isHexCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

// PlanReleaseBinaryReuse decides whether Freeze may stage the running binary
// as the release's bin/autoputer. It returns nil when it may, else an error
// that names why the desk must build the binary itself.
func PlanReleaseBinaryReuse(changes []FileChange, sourceArtifactRef string, running RunningBinary) error {
	if touched := SourceChangesTouchingBinary(changes); len(touched) > 0 {
		shown := touched
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return fmt.Errorf("the change touches binary inputs (%s%s), so the release needs bin/autoputer built from the workspace source", strings.Join(shown, ", "), moreSuffix(len(touched)-len(shown)))
	}
	sourceCommit := SourceArtifactCommit(sourceArtifactRef)
	if sourceCommit == "" {
		return fmt.Errorf("the capsule source is not pinned to a git commit, so the running binary cannot be shown to match it; build bin/autoputer")
	}
	if !computerSHA256(running.SHA256) {
		return fmt.Errorf("the running binary identity is unavailable; build bin/autoputer")
	}
	runningCommit := strings.ToLower(strings.TrimSpace(running.Commit))
	if !isHexCommit(runningCommit) {
		return fmt.Errorf("the running binary has no release commit (%q); build bin/autoputer", running.Commit)
	}
	if runningCommit != sourceCommit {
		return fmt.Errorf("the running binary was built from %s but the capsule source is pinned to %s; build bin/autoputer", runningCommit, sourceCommit)
	}
	return nil
}

func moreSuffix(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(" and %d more", n)
}

func computerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

// StageRunningBinary copies the running executable into root as
// bin/autoputer (mode 0555). The hash is taken over the bytes written, and
// the copy is refused unless it equals running.SHA256, so a binary replaced
// after its identity was taken is never staged under the old identity.
func StageRunningBinary(ctx context.Context, root string, running RunningBinary) (FrozenReleaseFile, int64, error) {
	if strings.TrimSpace(running.Path) == "" || !computerSHA256(running.SHA256) {
		return FrozenReleaseFile{}, 0, fmt.Errorf("capsule release: running binary identity is unavailable")
	}
	input, err := os.Open(running.Path)
	if err != nil {
		return FrozenReleaseFile{}, 0, fmt.Errorf("capsule release: open running binary: %w", err)
	}
	defer input.Close()
	target := filepath.Join(root, filepath.FromSlash(ReleaseBinaryPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return FrozenReleaseFile{}, 0, err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return FrozenReleaseFile{}, 0, fmt.Errorf("capsule release: stage running binary: %w", err)
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(output, hash), &contextReader{ctx: ctx, reader: input})
	closeErr := errors.Join(output.Sync(), output.Close())
	digest := hex.EncodeToString(hash.Sum(nil))
	if copyErr != nil || closeErr != nil || digest != running.SHA256 {
		_ = os.Remove(target)
		if copyErr != nil || closeErr != nil {
			return FrozenReleaseFile{}, 0, errors.Join(copyErr, closeErr)
		}
		return FrozenReleaseFile{}, 0, fmt.Errorf("capsule release: running binary content %s does not match its identity %s", digest, running.SHA256)
	}
	if err := os.Chmod(target, 0o555); err != nil {
		_ = os.Remove(target)
		return FrozenReleaseFile{}, 0, err
	}
	return FrozenReleaseFile{Path: ReleaseBinaryPath, SHA256: digest, Mode: 0o555}, size, nil
}

// ValidateReleaseBinary checks a bundle's provenance record against its
// release files. A nil record is a legacy bundle and passes.
func ValidateReleaseBinary(files []FrozenReleaseFile, provenance *ReleaseBinaryProvenance) error {
	if provenance == nil {
		return nil
	}
	var fileSHA string
	for _, file := range files {
		if file.Path == ReleaseBinaryPath {
			fileSHA = file.SHA256
		}
	}
	if fileSHA == "" || provenance.SHA256 != fileSHA {
		return fmt.Errorf("release binary provenance does not bind the release's %s", ReleaseBinaryPath)
	}
	switch provenance.Origin {
	case ReleaseBinaryCapsuleBuilt:
		return nil
	case ReleaseBinaryReusedRunning:
		if !isHexCommit(provenance.RunningCommit) || provenance.RunningCommit != provenance.SourceBaseCommit {
			return fmt.Errorf("reused release binary must name the running commit, equal to the source base commit")
		}
		return nil
	default:
		return fmt.Errorf("release binary origin %q is unknown", provenance.Origin)
	}
}

// CheckReusedBinaryIsRunning refuses a reused binary that is no longer this
// computer's running build (a deploy restarted it between Freeze and the
// check). Capsule-built and legacy binaries are not bound to the running
// build and pass.
func CheckReusedBinaryIsRunning(provenance *ReleaseBinaryProvenance, running RunningBinary) error {
	if provenance == nil || provenance.Origin != ReleaseBinaryReusedRunning {
		return nil
	}
	if running.SHA256 != provenance.SHA256 || strings.ToLower(strings.TrimSpace(running.Commit)) != provenance.RunningCommit {
		return fmt.Errorf("the reused release binary %s (commit %s) is no longer this computer's running build (%s, commit %s)",
			provenance.SHA256, provenance.RunningCommit, running.SHA256, running.Commit)
	}
	return nil
}

var (
	runningBinaryOnce sync.Once
	runningBinary     RunningBinary
	runningBinaryErr  error
)

// CurrentRunningBinary identifies this process's executable. On Linux it
// reads /proc/self/exe, which names the executed file even when the path it
// was started from has since been replaced. The hash is computed once: a
// process's executable does not change while it runs.
func CurrentRunningBinary() (RunningBinary, error) {
	runningBinaryOnce.Do(func() {
		exe := "/proc/self/exe"
		if runtime.GOOS != "linux" {
			exe, runningBinaryErr = os.Executable()
			if runningBinaryErr != nil {
				return
			}
		}
		input, err := os.Open(exe)
		if err != nil {
			runningBinaryErr = fmt.Errorf("open running executable: %w", err)
			return
		}
		defer input.Close()
		hash := sha256.New()
		if _, err := io.Copy(hash, input); err != nil {
			runningBinaryErr = fmt.Errorf("hash running executable: %w", err)
			return
		}
		runningBinary = RunningBinary{Path: exe, SHA256: hex.EncodeToString(hash.Sum(nil)), Commit: strings.TrimSpace(buildinfo.Commit)}
	})
	return runningBinary, runningBinaryErr
}
