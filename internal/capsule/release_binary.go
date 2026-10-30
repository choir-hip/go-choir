package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option A).
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
	// ReuseProof says how a reused binary was shown to match the source when
	// its commit is not the source's: ReuseProofBinaryInputsEqual, "no
	// binary-input path differs between the two commits" (U1, section 4.6).
	// Empty means the commits are equal.
	ReuseProof string `json:"reuse_proof,omitempty"`
}

// ReuseProofBinaryInputsEqual is the proof of a reused binary whose running
// commit differs from the capsule source commit.
const ReuseProofBinaryInputsEqual = "binary_inputs_equal"

// BinaryInputDiffer lists the paths that differ between two commits of the
// capsule's source repository. It fails when either commit is not there.
type BinaryInputDiffer func(ctx context.Context, fromCommit, toCommit string) ([]string, error)

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
	_, err := PlanReleaseBinaryReuseWith(context.Background(), changes, sourceArtifactRef, running, nil)
	return err
}

// PlanReleaseBinaryReuseWith is PlanReleaseBinaryReuse with U1's looser test.
// With differ nil it is the old rule: the running binary's commit equals the
// capsule source's. With a differ, a commit that differs is accepted when no
// binary-input path differs between the two commits, and the returned proof
// is ReuseProofBinaryInputsEqual. A differ that fails (a commit not in the
// repository) means the desk builds: the running binary is never assumed to
// match.
func PlanReleaseBinaryReuseWith(ctx context.Context, changes []FileChange, sourceArtifactRef string, running RunningBinary, differ BinaryInputDiffer) (string, error) {
	if touched := SourceChangesTouchingBinary(changes); len(touched) > 0 {
		shown := touched
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return "", fmt.Errorf("the change touches binary inputs (%s%s), so the release needs bin/autoputer built from the workspace source", strings.Join(shown, ", "), moreSuffix(len(touched)-len(shown)))
	}
	sourceCommit := SourceArtifactCommit(sourceArtifactRef)
	if sourceCommit == "" {
		return "", fmt.Errorf("the capsule source is not pinned to a git commit, so the running binary cannot be shown to match it; build bin/autoputer")
	}
	if !computerSHA256(running.SHA256) {
		return "", fmt.Errorf("the running binary identity is unavailable; build bin/autoputer")
	}
	runningCommit := strings.ToLower(strings.TrimSpace(running.Commit))
	if !isHexCommit(runningCommit) {
		return "", fmt.Errorf("the running binary has no release commit (%q); build bin/autoputer", running.Commit)
	}
	if runningCommit == sourceCommit {
		return "", nil
	}
	if differ == nil {
		return "", fmt.Errorf("the running binary was built from %s but the capsule source is pinned to %s; build bin/autoputer", runningCommit, sourceCommit)
	}
	paths, err := differ(ctx, runningCommit, sourceCommit)
	if err != nil {
		return "", fmt.Errorf("the running binary was built from %s and the capsule source is pinned to %s; they could not be compared (%v); build bin/autoputer", runningCommit, sourceCommit, err)
	}
	var inputs []string
	for _, p := range paths {
		if BinaryInputPath(p) {
			inputs = append(inputs, p)
		}
	}
	if len(inputs) > 0 {
		sort.Strings(inputs)
		shown := inputs
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return "", fmt.Errorf("the running binary was built from %s but the capsule source %s differs in binary inputs (%s%s); build bin/autoputer", runningCommit, sourceCommit, strings.Join(shown, ", "), moreSuffix(len(inputs)-len(shown)))
	}
	return ReuseProofBinaryInputsEqual, nil
}

// reuseGate is the live switch for U1's looser reuse test. The autoputer
// builds its capsule executor before the per-computer source.projection
// setting is loaded and the setting can change at runtime, so the answer is
// read each time Freeze builds its differ, never latched at construction
// (docs/problems/freeze-replays-a-changed-tree-2026-10-26.md).
// The zero value has no gate and is off.
type reuseGate struct{ fn atomic.Pointer[func() bool] }

func (g *reuseGate) set(fn func() bool) {
	if fn == nil {
		g.fn.Store(nil)
		return
	}
	g.fn.Store(&fn)
}

func (g *reuseGate) on() bool {
	fn := g.fn.Load()
	return fn != nil && (*fn)()
}

// differ is nil (the old rule) while the gate is off.
func (g *reuseGate) differ(sourceDir string) BinaryInputDiffer {
	if !g.on() {
		return nil
	}
	return GitPathDiffer(sourceDir)
}

// gitDiffTimeout bounds the reuse comparison: Freeze must not hang on git.
const gitDiffTimeout = 120 * time.Second

func scrubbedGitEnv() []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}

// GitPathDiffer lists the paths that differ between two commits of the git
// repository at sourceDir. Both must be full commit ids; anything else is
// refused before git sees it.
func GitPathDiffer(sourceDir string) BinaryInputDiffer {
	return func(ctx context.Context, from, to string) ([]string, error) {
		if !isHexCommit(from) || !isHexCommit(to) {
			return nil, fmt.Errorf("commit ids are required")
		}
		// Same promise as sourceproj's git: a caller's GIT_* must not redirect
		// the repository (a diff of the wrong repo would read as "no binary input
		// differs"), config must not change the answer, and a hung git is bounded.
		ctx, cancel := context.WithTimeout(ctx, gitDiffTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-C", sourceDir, "-c", "core.hooksPath="+os.DevNull, "diff-tree", "-r", "--no-renames", "--name-only", "-z", from, to, "--")
		cmd.Env = scrubbedGitEnv()
		out, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				return nil, fmt.Errorf("git diff-tree: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
			}
			return nil, fmt.Errorf("git diff-tree: %w", err)
		}
		var paths []string
		for _, p := range strings.Split(string(out), "\x00") {
			if p != "" {
				paths = append(paths, p)
			}
		}
		return paths, nil
	}
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
		if !isHexCommit(provenance.RunningCommit) {
			return fmt.Errorf("reused release binary must name the running commit")
		}
		if provenance.RunningCommit != provenance.SourceBaseCommit {
			// U1: a differing commit is accepted only with the proof that no
			// binary input differs. A bundle without it still needs equality.
			if provenance.ReuseProof != ReuseProofBinaryInputsEqual || !isHexCommit(provenance.SourceBaseCommit) {
				return fmt.Errorf("reused release binary must name the running commit, equal to the source base commit or proven equal in binary inputs")
			}
		} else if provenance.ReuseProof != "" {
			return fmt.Errorf("reused release binary carries a reuse proof for equal commits")
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
	runningBinaryMu sync.Mutex
	runningBinary   *RunningBinary
)

// CurrentRunningBinary identifies this process's executable. On Linux it
// reads /proc/self/exe, which names the executed file even when the path it
// was started from has since been replaced. A successful identity is cached:
// a process's executable does not change while it runs. A failed read is
// not cached, so a transient error does not disable reuse for the process.
func CurrentRunningBinary() (RunningBinary, error) {
	runningBinaryMu.Lock()
	defer runningBinaryMu.Unlock()
	if runningBinary != nil {
		return *runningBinary, nil
	}
	exe := "/proc/self/exe"
	if runtime.GOOS != "linux" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return RunningBinary{}, err
		}
	}
	input, err := os.Open(exe)
	if err != nil {
		return RunningBinary{}, fmt.Errorf("open running executable: %w", err)
	}
	defer input.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, input); err != nil {
		return RunningBinary{}, fmt.Errorf("hash running executable: %w", err)
	}
	runningBinary = &RunningBinary{Path: exe, SHA256: hex.EncodeToString(hash.Sum(nil)), Commit: strings.TrimSpace(buildinfo.Commit)}
	return *runningBinary, nil
}
