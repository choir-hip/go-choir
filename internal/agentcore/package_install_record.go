package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The install record and its staging (docs/design/package-install-2026-11-04.md):
// what an owner asked this computer to install from the Library, the
// read-only staging engineering's capsule mounts at /selfdev/adoption, and
// the view the Library reads. Software-release adoption (P8) is deleted.

var errAdoptionStagedTampered = errors.New("staged install bytes no longer match the verified release")

// PromptSubmission is what the owner's prompt path returns.
type PromptSubmission struct {
	SubmissionID string
	TrajectoryID string
	DocID        string
}

// PromptSubmitter submits an owner request to this computer's desks the way
// the prompt bar does (Texture first). Bound by the autoputer to the
// Texture owner's submit path.
type PromptSubmitter func(ctx context.Context, ownerID, text, commandID string) (PromptSubmission, error)

// SetPromptSubmitter binds the owner's prompt path.
func (rt *Runtime) SetPromptSubmitter(submit PromptSubmitter) {
	if rt != nil {
		rt.promptSubmitter = submit
	}
}

// ReleaseAdoptionView is an install with what is derived from the
// operation on its trajectory.
type ReleaseAdoptionView struct {
	store.ReleaseAdoption
	Operation *AdoptionOperation `json:"operation,omitempty"`
	// PatchMatchesRelease: this computer's frozen patch is byte-identical
	// to the release's (expected when the bases are the same).
	PatchMatchesRelease bool `json:"patch_matches_release"`
}

// AdoptionOperation is the adopter's self-development operation on the
// adoption's trajectory.
type AdoptionOperation struct {
	OperationID       string `json:"operation_id"`
	State             string `json:"state"`
	BundleDigest      string `json:"bundle_digest,omitempty"`
	SourcePatchSHA256 string `json:"source_patch_sha256,omitempty"`
	Error             string `json:"error,omitempty"`
}

// adoptionRefusal is a typed refusal: an HTTP status and a reason code.
type adoptionRefusal struct {
	status int
	reason string
	detail string
}

func (r *adoptionRefusal) Error() string { return r.reason + ": " + r.detail }

func refuseAdoption(status int, reason, format string, args ...any) *adoptionRefusal {
	return &adoptionRefusal{status: status, reason: reason, detail: fmt.Sprintf(format, args...)}
}

type adoptReleaseRequest struct {
	ReleaseID string          `json:"release_id"`
	Record    json.RawMessage `json:"record"`
	// TreePatch and Action are a package install's (package_install.go):
	// the release's tree_patch the proxy read from the catalog, and install
	// (the default) or uninstall. A software release carries neither.
	TreePatch []byte `json:"tree_patch,omitempty"`
	Action    string `json:"action,omitempty"`
}

// stageAdoptionAt stages a canonical record that hashes to releaseID and a
// patch under <updater root>/adoptions/<name>/, read only (a package install
// names its install id, so each attempt and an uninstall stage apart).
func (rt *Runtime) stageAdoptionAt(name, releaseID string, canonical, patch []byte) (string, error) {
	root := filepath.Join(rt.selfdevUpdaterRoot, "adoptions")
	target := filepath.Join(root, name)
	if verifyStagedAdoption(target, releaseID, computerevent.DigestBytes(patch)) == nil {
		return target, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, ".stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "release.json"), canonical, 0o444); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, "source.patch"), patch, 0o444); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Lstat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

func verifyStagedAdoption(dir, releaseID, patchSHA string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return errAdoptionStagedTampered
	}
	raw, err := os.ReadFile(filepath.Join(dir, "release.json"))
	if err != nil || computerevent.DigestBytes(raw) != releaseID {
		return errAdoptionStagedTampered
	}
	patch, err := os.ReadFile(filepath.Join(dir, "source.patch"))
	if err != nil || computerevent.DigestBytes(patch) != patchSHA {
		return errAdoptionStagedTampered
	}
	return nil
}

// adoptionInputDir is the capsule input for an engineering assignment on
// trajectory: the staged adoption, re-verified now (A8). Another
// trajectory gets nothing (A9).
func (rt *Runtime) adoptionInputDir(ctx context.Context, ownerID, computerID, trajectoryID string) (string, error) {
	if rt == nil || rt.store == nil {
		return "", nil
	}
	adoption, err := rt.store.ReleaseAdoptionForTrajectory(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err := verifyStagedAdoption(adoption.StagedDir, adoption.ReleaseID, adoption.PatchSHA256); err != nil {
		return "", fmt.Errorf("%w (%s): %v", errAdoptionStagedTampered, adoption.AdoptionID, err)
	}
	return adoption.StagedDir, nil
}

// adoptionView derives the operation on the adoption's trajectory and
// whether its frozen patch is the release's.
func (rt *Runtime) adoptionView(ctx context.Context, adoption store.ReleaseAdoption) ReleaseAdoptionView {
	view := ReleaseAdoptionView{ReleaseAdoption: adoption}
	if adoption.Kind == adoptionKindPackage {
		// A package install is in place when the computer's tree holds the
		// release at the package root, whatever the bundle's cumulative
		// patch carries.
		view.PatchMatchesRelease = rt.packageInstalledNow(ctx, adoption)
	}
	if rt.selfdevOperations == nil {
		return view
	}
	operations, err := rt.selfdevOperations.ListByTrajectory(ctx, adoption.ComputerID, adoption.TrajectoryID)
	if err != nil || len(operations) == 0 {
		return view
	}
	operation := operations[len(operations)-1]
	view.Operation = &AdoptionOperation{OperationID: operation.OperationID, State: operation.State, BundleDigest: operation.BundleDigest, Error: operation.TerminalError}
	if operation.BundleDigest != "" && strings.TrimSpace(rt.selfdevUpdaterRoot) != "" && operation.State != selfdev.StateRequested && operation.State != selfdev.StateExecuting {
		bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
		if err == nil {
			view.Operation.SourcePatchSHA256 = bundle.SourcePatchSHA256
			if adoption.Kind != adoptionKindPackage {
				view.PatchMatchesRelease = bundle.SourcePatchSHA256 != "" && bundle.SourcePatchSHA256 == adoption.PatchSHA256
			}
		}
	}
	return view
}
