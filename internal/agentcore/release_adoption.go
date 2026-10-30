package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Source adoption (P8, Station B minimal;
// docs/archive/reports/p6-p8-release-adopt-2026-10-15.md). An owner asks this
// computer to adopt a published release as source. Before any state change
// the computer verifies the record against its id and against the
// published copy, verifies the patch bytes, scans them, and checks the patch
// applies to its OWN source tree. Only then does it stage the record and the
// patch (read only, content addressed), ask its own desks through the
// owner's prompt path, and record the adoption on its tape. Engineering
// applies the patch in its own capsule; the computer's own gate decides. No
// binary crosses: not A's, not the host's.

// ReleaseSource reads published releases from the host (corpusd) with this
// computer's own capability.
type ReleaseSource interface {
	FetchRelease(ctx context.Context, computerID, releaseID string) (json.RawMessage, error)
	FetchReleasePatch(ctx context.Context, computerID, patchSHA256 string) ([]byte, error)
}

var (
	// ErrReleaseNotPublished: the host has no published release by that id.
	ErrReleaseNotPublished = errors.New("release not published")
	// ErrReleasePatchUnavailable: the host content store has no such patch.
	ErrReleasePatchUnavailable = errors.New("release patch unavailable")
	// errAdoptionStagedTampered: staged adoption bytes no longer verify.
	errAdoptionStagedTampered = errors.New("staged adoption bytes no longer match the verified release")
)

// WithReleaseSource installs the host release reader.
func WithReleaseSource(source ReleaseSource) RuntimeOption {
	return func(rt *Runtime) { rt.releaseSource = source }
}

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

// ReleaseAdoptionView is an adoption with what is derived from the
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
}

func adoptionID(computerID, releaseID string) string {
	return "adoption-" + computerevent.DigestBytes([]byte(strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(releaseID)))[:24]
}

// adoptRelease runs the verification ladder and, only when every rung
// holds, stages, asks the desks and records. created is false for a repeat
// of an existing adoption (A12).
func (rt *Runtime) adoptRelease(ctx context.Context, ownerID, computerID string, request adoptReleaseRequest) (ReleaseAdoptionView, bool, *adoptionRefusal) {
	if rt.releaseSource == nil || rt.promptSubmitter == nil || rt.store == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || strings.TrimSpace(rt.releaseRepo()) == "" {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "adoption_unavailable", "this computer has no release source, prompt path, update root or source repository")
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	if !computerevent.IsSHA256(releaseID) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadRequest, "release_id_invalid", "release_id must be a sha256")
	}
	if existing, err := rt.store.GetReleaseAdoption(ctx, ownerID, computerID, adoptionID(computerID, releaseID)); err == nil {
		return rt.adoptionView(ctx, existing), false, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "%v", err)
	}

	// 1. The record the owner adopts hashes to the id the owner named (A1).
	supplied, err := release.Decode(request.Record)
	if err == nil {
		err = release.VerifyID(supplied, releaseID)
	}
	if errors.Is(err, release.ErrDigestMismatch) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "release_digest_mismatch", "%v", err)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "release_record_invalid", "%v", err)
	}
	// 2-3. It is published, and the published copy is the same bytes and
	// still hashes to the id (A2, A3).
	publishedRaw, err := rt.releaseSource.FetchRelease(ctx, computerID, releaseID)
	if errors.Is(err, ErrReleaseNotPublished) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusNotFound, "release_not_published", "the host has no published release %s", releaseID)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadGateway, "release_source_unavailable", "%v", err)
	}
	published, err := release.Decode(publishedRaw)
	if err == nil {
		err = release.VerifyID(published, releaseID)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadGateway, "published_record_digest_mismatch", "the host's copy of %s does not verify: %v", releaseID, err)
	}
	if !release.Same(supplied, published) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "release_record_mismatch", "the record differs from the published one")
	}
	// 5. The patch bytes are the record's (A3, A4).
	patch, err := rt.releaseSource.FetchReleasePatch(ctx, computerID, published.Source.PatchSHA256)
	if errors.Is(err, ErrReleasePatchUnavailable) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_unavailable", "the host content store has no patch %s", published.Source.PatchSHA256)
	}
	if errors.Is(err, release.ErrPatchMismatch) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_digest_mismatch", "%v", err)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadGateway, "release_source_unavailable", "%v", err)
	}
	if err := release.VerifyPatch(published, patch); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_digest_mismatch", "%v", err)
	}
	// 6. Nothing secret enters this computer's tree.
	if kinds := sourcePatchAddedSecrets(patch); len(kinds) > 0 {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "patch_secret_refused", "added lines carry secret content (%s)", strings.Join(kinds, ", "))
	}
	// 7. It applies to this computer's OWN tree, checked without writing to
	// the repository (A5, A7). A conflict is refused, never merged.
	repo := rt.releaseRepo()
	head, err := release.Head(ctx, repo)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "source_unavailable", "%v", err)
	}
	tree, err := release.PatchedTree(ctx, repo, head, patch)
	if errors.Is(err, release.ErrPatchDoesNotApply) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_does_not_apply", "%v", err)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "source_unavailable", "%v", err)
	}

	// Every rung held. Stage, ask the desks, record.
	staged, err := rt.stageAdoption(releaseID, published, patch)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_stage", "%v", err)
	}
	id := adoptionID(computerID, releaseID)
	submission, err := rt.promptSubmitter(ctx, ownerID, adoptionRequestText(releaseID, published, head == published.Source.BaseCommit), id)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadGateway, "desks_unavailable", "the adoption request did not reach this computer's desks: %v", err)
	}
	adoption := store.ReleaseAdoption{
		AdoptionID: id, OwnerID: ownerID, ComputerID: computerID, ReleaseID: releaseID,
		PublisherComputerID: published.Publisher.ComputerID, PublisherOperationID: published.Operation.OperationID,
		PatchSHA256: published.Source.PatchSHA256, ReleaseBaseCommit: published.Source.BaseCommit, ReleaseTreeHash: published.Source.TreeHash,
		AdopterBaseCommit: head, AdopterTreeHash: tree, SameBase: head == published.Source.BaseCommit,
		TestCommand: published.Declarations.TestCommand, StagedDir: staged,
		TrajectoryID: submission.TrajectoryID, DocID: submission.DocID, SubmissionID: submission.SubmissionID,
		RequestedAt: time.Now().UTC(),
	}
	if err := rt.store.PutReleaseAdoption(ctx, adoption); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "the request reached the desks (trajectory %s) but the adoption was not recorded: %v", submission.TrajectoryID, err)
	}
	return rt.adoptionView(ctx, adoption), true, nil
}

// stageAdoption writes release.json and source.patch under
// <updater root>/adoptions/<release id>/, read only. An existing staging
// is kept when it verifies and replaced when it does not.
func (rt *Runtime) stageAdoption(releaseID string, record release.Record, patch []byte) (string, error) {
	root := filepath.Join(rt.selfdevUpdaterRoot, "adoptions")
	target := filepath.Join(root, releaseID)
	if verifyStagedAdoption(target, releaseID, record.Source.PatchSHA256) == nil {
		return target, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	canonical, err := record.Canonical()
	if err != nil {
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

// adoptedReleaseForTrajectory names the release an operation adopted.
func (rt *Runtime) adoptedReleaseForTrajectory(ctx context.Context, trajectoryID string) string {
	if rt == nil || rt.store == nil || strings.TrimSpace(rt.selfdevRouteOwnerID) == "" {
		return ""
	}
	adoption, err := rt.store.ReleaseAdoptionForTrajectory(ctx, rt.selfdevRouteOwnerID, rt.cfg.ComputerID, trajectoryID)
	if err != nil {
		return ""
	}
	return adoption.ReleaseID
}

// adoptionView derives the operation on the adoption's trajectory and
// whether its frozen patch is the release's.
func (rt *Runtime) adoptionView(ctx context.Context, adoption store.ReleaseAdoption) ReleaseAdoptionView {
	view := ReleaseAdoptionView{ReleaseAdoption: adoption}
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
			view.PatchMatchesRelease = bundle.SourcePatchSHA256 != "" && bundle.SourcePatchSHA256 == adoption.PatchSHA256
		}
	}
	return view
}

// adoptionRequestText is the owner's request to this computer's desks. It
// names the release and where engineering's capsule finds the verified
// patch; it asks for nothing but applying it.
func adoptionRequestText(releaseID string, record release.Record, sameBase bool) string {
	check := "git apply --check --reverse /selfdev/adoption/source.patch (it succeeds only when the patch is applied)"
	if cmd := strings.TrimSpace(record.Declarations.TestCommand); cmd != "" {
		check = cmd + ", and git apply --check --reverse /selfdev/adoption/source.patch"
	}
	base := "This computer's source is the release's base commit, so the patch applies as published."
	if !sameBase {
		base = "This computer's source differs from the release's base commit; the patch was checked to apply to this computer's own tree."
	}
	return strings.Join([]string{
		fmt.Sprintf("Adopt software release release:%s on this computer, as source: change this computer itself by applying that release's patch, and change nothing else.", releaseID),
		fmt.Sprintf("The release was published by computer %s from its operation %s; its patch (sha256 %s) changes: %s.", record.Publisher.ComputerID, record.Operation.OperationID, record.Source.PatchSHA256, strings.Join(record.Source.ChangedPaths, ", ")),
		base,
		"Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision, and incorporate management's answer when it arrives.",
		"Management: Cast it to engineering verbatim; then Verify the frozen bundle that engineering's final Report names, and answer this request.",
		"Engineering: the verified patch is mounted read only in your capsule at /selfdev/adoption/source.patch, with the release record at /selfdev/adoption/release.json.",
		"From /workspace/platform run: git apply /selfdev/adoption/source.patch (or patch -p1 < /selfdev/adoption/source.patch). If it does not apply, report the conflict and do not Freeze.",
		"Deterministic check, to run in the capsule before freezing: " + check + ".",
		"Then build the release tree, freeze the candidate with choir.Freeze and complete.",
	}, " ")
}
