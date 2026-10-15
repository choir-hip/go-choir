// Package hostbuild is the host build service (P5,
// docs/reports/p5-host-build-layered-apply-2026-10-15.md): a computer sends
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
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

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

// recipePrefixes are the build recipe and dependency set: a patch touching
// them would change what the host evaluates or fetches (H2).
var recipeFiles = map[string]bool{
	"flake.nix": true, "flake.lock": true, "go.mod": true, "go.sum": true,
	"go.work": true, "go.work.sum": true,
	"frontend/package.json": true, "frontend/package-lock.json": true,
}

var recipeDirs = []string{"nix/", "vendor/"}

// PatchPaths returns the repo-relative paths a unified diff names, from its
// git headers and its ---/+++ lines. A patch naming no path is unreadable.
func PatchPaths(patch []byte) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || p == "/dev/null" {
			return
		}
		if i := strings.IndexByte(p, '\t'); i >= 0 {
			p = p[:i]
		}
		p = strings.TrimPrefix(strings.TrimPrefix(p, "a/"), "b/")
		p = path.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(patch))
	scanner.Buffer(make([]byte, 64<<10), MaxPatchBytes+1)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			fields := strings.Fields(strings.TrimPrefix(line, "diff --git "))
			for _, f := range fields {
				add(f)
			}
		case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			add(line[4:])
		case strings.HasPrefix(line, "rename from "), strings.HasPrefix(line, "rename to "):
			add(line[len("rename from "):])
		case strings.HasPrefix(line, "copy from "), strings.HasPrefix(line, "copy to "):
			add(line[len("copy from "):])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("host build: read patch: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("host build: the patch names no file")
	}
	return out, nil
}

// RefusePatchPaths returns the reason code and the offending path when the
// patch touches the recipe, the dependency set, or the store schema; or a
// path escaping the repository.
func RefusePatchPaths(paths []string) (reason, offending string) {
	for _, p := range paths {
		if p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
			return ReasonPatchUnreadable, p
		}
		if recipeFiles[p] {
			return ReasonRecipeChangeRefused, p
		}
		for _, dir := range recipeDirs {
			if strings.HasPrefix(p, dir) {
				return ReasonRecipeChangeRefused, p
			}
		}
		if strings.HasPrefix(p, "internal/storeschema/") {
			return ReasonStoreEpochRefused, p
		}
	}
	return "", ""
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
