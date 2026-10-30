// Package hostbuild is the host build service (P5,
// docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md): a computer sends
// the source patch of an accepted self-development candidate, and the host,
// the sole builder, builds bin/autoputer (and the frontend, one flake
// evaluation) from the computer's base commit plus that patch, stores the
// exported closure in the platform-update blob namespace, and signs the
// result. The guest mints a layered release from it.
//
// The job record is the only authority for a build's state. A restart of the
// host process ends every job that was queued or running (crash rule:
// restarts end work, they never resume it).
package hostbuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/builder"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Job states.
const (
	StateQueued  = "queued"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
)

// Reason codes a failed job carries. The guest copies the code into the
// operation's terminal error.
const (
	ReasonPatchDigestMismatch     = "patch_digest_mismatch"
	ReasonPatchTooLarge           = "patch_too_large"
	ReasonPatchUnreadable         = "patch_unreadable"
	ReasonRecipeChangeRefused     = "recipe_change_refused"
	ReasonPatchTreeRefused        = "patch_tree_refused"
	ReasonBuildQuotaExhausted     = "build_quota_exhausted"
	ReasonStoreEpochRefused       = "store_epoch_change_refused"
	ReasonBaseMoved               = "base_moved"
	ReasonBaseCommitMismatch      = "base_commit_mismatch"
	ReasonQueueFull               = "queue_full"
	ReasonComputerBusy            = "computer_busy"
	ReasonNotAdmitted             = "not_admitted"
	ReasonBuildFailed             = "build_failed"
	ReasonBuildTimeout            = "build_timeout"
	ReasonInterruptedByRestart    = "interrupted_by_host_restart"
	ReasonHostBuilderUnconfigured = "host_builder_unconfigured"
)

// MaxPatchBytes is the largest patch the host builds (the patch export cap).
const MaxPatchBytes = 2 << 20

// Request is what a guest submits. The patch bytes travel base64 and are
// bound by SourcePatchSHA256; the job id does not include them.
type Request struct {
	ComputerID              string `json:"computer_id"`
	OperationID             string `json:"operation_id"`
	BundleDigest            string `json:"bundle_digest"`
	BaseCommit              string `json:"base_commit"`
	SourcePatchSHA256       string `json:"source_patch_sha256"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest"`
	PatchBase64             string `json:"patch_base64,omitempty"`
}

// requestIdentity is the canonical subject of a job id.
type requestIdentity struct {
	ComputerID              string `json:"computer_id"`
	OperationID             string `json:"operation_id"`
	BundleDigest            string `json:"bundle_digest"`
	BaseCommit              string `json:"base_commit"`
	SourcePatchSHA256       string `json:"source_patch_sha256"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest"`
}

// Validate checks the request's shape (not the patch bytes).
func (r Request) Validate() error {
	switch {
	case strings.TrimSpace(r.ComputerID) == "" || strings.TrimSpace(r.OperationID) == "":
		return fmt.Errorf("host build: computer and operation are required")
	case !computerevent.IsSHA256(r.BundleDigest):
		return fmt.Errorf("host build: bundle digest is required")
	case !isCommit(r.BaseCommit):
		return fmt.Errorf("host build: base commit must be a full commit id")
	case !computerevent.IsSHA256(r.SourcePatchSHA256):
		return fmt.Errorf("host build: source patch sha256 is required")
	case !computerevent.IsSHA256(r.BaseImageManifestDigest):
		return fmt.Errorf("host build: booted base image manifest digest is required")
	}
	return nil
}

// JobID is the sha256 of the canonical request identity: a retry of the same
// request names the same job, and a different patch (or base, or operation)
// never does (H8).
func (r Request) JobID() (string, error) {
	canonical, err := computerevent.CanonicalJSON(requestIdentity{
		ComputerID: r.ComputerID, OperationID: r.OperationID, BundleDigest: r.BundleDigest,
		BaseCommit: r.BaseCommit, SourcePatchSHA256: r.SourcePatchSHA256,
		BaseImageManifestDigest: r.BaseImageManifestDigest,
	})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

// Result is what a done job built. It is signed once, at completion, so the
// guest's layered manifest (which binds the builder receipt digest) is the
// same on every pass.
type Result struct {
	JobID                   string   `json:"job_id"`
	ComputerID              string   `json:"computer_id"`
	OperationID             string   `json:"operation_id"`
	BundleDigest            string   `json:"bundle_digest"`
	BaseCommit              string   `json:"base_commit"`
	SourcePatchSHA256       string   `json:"source_patch_sha256"`
	BaseImageManifestDigest string   `json:"base_image_manifest_digest"`
	BaseStoreDiskSHA256     string   `json:"base_store_disk_sha256"`
	CodeCommit              string   `json:"code_commit"`
	CodeCommitSource        string   `json:"code_commit_source"`
	RuntimePath             string   `json:"runtime_path"`
	LayeringEntrypoint      string   `json:"layering_entrypoint"`
	ClosureDigest           string   `json:"closure_digest"`
	ClosureBytes            int64    `json:"closure_bytes"`
	ClosurePaths            []string `json:"closure_paths"`
	BuilderReceiptDigest    string   `json:"builder_receipt_digest"`
	DerivationPath          string   `json:"derivation_path"`
	FlakeLockSHA256         string   `json:"flake_lock_sha256,omitempty"`
	StoreSchemaVersion      uint64   `json:"store_schema_version"`
	BuiltAt                 string   `json:"built_at"`
}

// Digest is the sha256 of the canonical result; the host receipt's artifact
// digest.
func (r Result) Digest() (string, error) {
	canonical, err := computerevent.CanonicalJSON(r)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

// Job is the durable record and the wire answer.
type Job struct {
	JobID                   string                            `json:"job_id"`
	ComputerID              string                            `json:"computer_id"`
	OperationID             string                            `json:"operation_id"`
	BundleDigest            string                            `json:"bundle_digest"`
	BaseCommit              string                            `json:"base_commit"`
	SourcePatchSHA256       string                            `json:"source_patch_sha256"`
	BaseImageManifestDigest string                            `json:"base_image_manifest_digest"`
	State                   string                            `json:"state"`
	Reason                  string                            `json:"reason,omitempty"`
	Detail                  string                            `json:"detail,omitempty"`
	CreatedAt               time.Time                         `json:"created_at"`
	QueueDeadline           time.Time                         `json:"queue_deadline"`
	StartedAt               *time.Time                        `json:"started_at,omitempty"`
	FinishedAt              *time.Time                        `json:"finished_at,omitempty"`
	Admission               string                            `json:"admission,omitempty"`
	Result                  *Result                           `json:"result,omitempty"`
	Receipt                 *selfdevprotocol.AuthorityReceipt `json:"receipt,omitempty"`
}

// Terminal reports whether the job will never change again.
func (j Job) Terminal() bool { return j.State == StateDone || j.State == StateFailed }

// recipeFiles are the build recipe and dependency set: a patch touching them
// would change what the host evaluates or fetches (H2).
var recipeFiles = map[string]bool{
	"flake.nix": true, "flake.lock": true, "go.mod": true, "go.sum": true,
	"go.work": true, "go.work.sum": true,
	"frontend/package.json": true, "frontend/package-lock.json": true,
}

var recipeDirs = []string{"nix", "vendor"}

// RefusalError is a refusal with a reason code. Retry is true for a
// transient refusal (queue_full, computer_busy) that records no job, so the
// retry of the same request can still build (H20).
type RefusalError struct {
	Reason string
	Detail string
	Retry  bool
}

func (e *RefusalError) Error() string { return e.Reason + ": " + e.Detail }

// refusePath returns the reason a patch must not change p: a path escaping
// the repository, the recipe or dependency set (any nix file, any flake
// file, at any depth), git metadata, or the store schema.
func refusePath(p string) (string, string) {
	clean := path.Clean(p)
	switch {
	case p == "" || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/"):
		return ReasonPatchUnreadable, "the patch names a path outside the repository (" + p + ")"
	case path.Base(clean) == ".gitattributes" || path.Base(clean) == ".gitmodules":
		return ReasonPatchTreeRefused, "the patch changes git metadata (" + p + ")"
	case strings.HasPrefix(clean, "internal/storeschema/"):
		return ReasonStoreEpochRefused, "the patch changes the store schema (" + p + "); an epoch-bumping release is refused until LU2"
	}
	recipe := recipeFiles[clean] || strings.HasSuffix(clean, ".nix") ||
		path.Base(clean) == "flake.lock" || path.Base(clean) == "flake.nix"
	for _, dir := range recipeDirs {
		if clean == dir || strings.HasPrefix(clean, dir+"/") {
			recipe = true
		}
	}
	if recipe {
		return ReasonRecipeChangeRefused, "the patch changes the build recipe or the dependency set (" + p + "); the host builds only with the base commit's pinned recipe"
	}
	return "", ""
}

// AdmitPatchedTree is the host's admission of the tree the builder committed
// (builder.Request.AdmitPatchedTree): the authority for H2, H3, H15 and H16.
// It sees git's own record of the change, so no spelling of the patch can
// hide a path from it.
func AdmitPatchedTree(changes []builder.PatchedChange) error {
	if err := builder.RefuseTreeMetadata(changes); err != nil {
		return &RefusalError{Reason: ReasonPatchTreeRefused, Detail: err.Error()}
	}
	for _, c := range changes {
		if reason, detail := refusePath(c.Path); reason != "" {
			return &RefusalError{Reason: reason, Detail: detail}
		}
	}
	return nil
}

// patchPathsTimeout bounds git's reading of a submitted patch.
const patchPathsTimeout = 30 * time.Second

// PatchPaths returns the repo-relative paths a patch changes as git reads
// them (`git apply --numstat -z`, the parser the builder's apply uses): any
// first path segment stripped, quoted names unquoted. It is the early,
// cheap check at submission; the authority is AdmitPatchedTree on the
// committed tree. A patch git cannot read, or that names no path, is
// unreadable.
func PatchPaths(patch []byte) ([]string, error) {
	// Outside any repository: inside one, git apply would skip paths outside
	// the current directory.
	dir, err := os.MkdirTemp("", "choir-hostbuild-numstat-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), patchPathsTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "apply", "--numstat", "-z", "-")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(patch)
	cmd.Env = gitEnv(filepath.Dir(dir))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("host build: git cannot read the patch: %s", truncate(strings.TrimSpace(stderr.String())))
	}
	tokens := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for i := 0; i < len(tokens); i++ {
		parts := strings.SplitN(tokens[i], "\t", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[2] != "" {
			add(parts[2])
			continue
		}
		// A rename or copy: the source and destination follow.
		if i+2 < len(tokens) {
			add(tokens[i+1])
			add(tokens[i+2])
		}
		i += 2
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("host build: the patch names no file")
	}
	return paths, nil
}

// RefusePatchPaths returns the reason code and detail when a path must not
// change.
func RefusePatchPaths(paths []string) (reason, detail string) {
	for _, p := range paths {
		if reason, detail := refusePath(p); reason != "" {
			return reason, detail
		}
	}
	return "", ""
}

// gitEnv is the environment host git runs in: no inherited git variables, no
// system or global config, no repository discovery above ceiling.
func gitEnv(ceiling string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); strings.HasPrefix(key, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_CEILING_DIRECTORIES="+ceiling)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func isCommit(c string) bool {
	if len(c) != 40 && len(c) != 64 {
		return false
	}
	_, err := hex.DecodeString(c)
	return err == nil && strings.ToLower(c) == c
}
