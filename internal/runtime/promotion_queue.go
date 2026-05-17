package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/promotion"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// QueuePromotionCandidate records a candidate-world patchset for later
// verifier-contract execution and explicit promotion.
func (rt *Runtime) QueuePromotionCandidate(ctx context.Context, rec types.PromotionCandidateRecord) (types.PromotionCandidateRecord, error) {
	if rt == nil || rt.store == nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("queue promotion candidate: runtime store is unavailable")
	}
	if strings.TrimSpace(rec.OwnerID) == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("queue promotion candidate: owner_id is required")
	}
	if strings.TrimSpace(rec.CandidateID) == "" {
		rec.CandidateID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.PromotionCandidateQueued
	}
	if strings.TrimSpace(rec.DestinationBranch) == "" {
		rec.DestinationBranch = "main"
	}
	candidate, err := candidateWorldFromRecord(rec)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	candidateBytes, err := json.Marshal(candidate)
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("marshal promotion candidate: %w", err)
	}
	rec.CandidateJSON = candidateBytes
	if len(rec.ContractsJSON) == 0 {
		rec.ContractsJSON = json.RawMessage(`[]`)
	}
	if len(rec.ReportJSON) == 0 {
		rec.ReportJSON = json.RawMessage(`{}`)
	}

	rec, err = rt.store.UpsertPromotionCandidate(ctx, rec)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidateQueued, nil)
	return rec, nil
}

// VerifyPromotionCandidate imports the queued worker patchset into an
// integration branch and executes its verifier contracts. Canonical state is
// not mutated.
func (rt *Runtime) VerifyPromotionCandidate(ctx context.Context, ownerID, candidateID, repoPath string) (types.PromotionCandidateRecord, error) {
	if rt == nil || rt.store == nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("verify promotion candidate: runtime store is unavailable")
	}
	rec, err := rt.store.GetPromotionCandidate(ctx, ownerID, candidateID)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	contracts, err := verifierContractsFromRecord(rec)
	if err != nil {
		return rec, err
	}
	return rt.verifyPromotionCandidateInRepo(ctx, rec, repoPath, "", contracts)
}

// VerifyPromotionCandidateInWorkspace imports a queued worker patchset into a
// server-owned integration workspace derived from the canonical source repo.
// This is the product-safe verifier path: callers provide only the candidate
// id, never a repo path or arbitrary verifier commands.
func (rt *Runtime) VerifyPromotionCandidateInWorkspace(ctx context.Context, ownerID, candidateID string) (types.PromotionCandidateRecord, error) {
	if rt == nil || rt.store == nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("verify promotion candidate in workspace: runtime store is unavailable")
	}
	rec, err := rt.store.GetPromotionCandidate(ctx, ownerID, candidateID)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	if rec.Status == types.PromotionCandidatePromoted || rec.Status == types.PromotionCandidateRejected {
		return rec, fmt.Errorf("verify promotion candidate in workspace: candidate status %q cannot be verified", rec.Status)
	}
	repoPath, reportPath, err := rt.preparePromotionWorkspace(ctx, rec)
	if err != nil {
		return rt.failPromotionCandidate(ctx, rec, err)
	}
	return rt.verifyPromotionCandidateInRepo(ctx, rec, repoPath, reportPath, productPromotionVerifierContracts(rec))
}

func (rt *Runtime) verifyPromotionCandidateInRepo(ctx context.Context, rec types.PromotionCandidateRecord, repoPath, reportPath string, contracts []promotion.VerifierContract) (types.PromotionCandidateRecord, error) {
	candidate, err := candidateWorldFromRecord(rec)
	if err != nil {
		return rec, err
	}
	if len(contracts) == 0 {
		return rec, fmt.Errorf("verify promotion candidate: at least one verifier contract is required")
	}

	report, verifyErr := promotion.PrepareIntegrationCandidate(ctx, promotion.PrepareOptions{
		RepoPath:          repoPath,
		ManifestPath:      rec.ManifestPath,
		PatchsetPath:      rec.PatchsetPath,
		IntegrationBranch: rec.IntegrationBranch,
		DestinationBranch: rec.DestinationBranch,
		ReportPath:        reportPath,
		CommitMessage:     commitMessageForCandidate(rec),
		Candidate:         candidate,
		Contracts:         contracts,
	})
	if report != nil {
		rec = updateRecordFromPromotionReport(rec, report)
	}
	if verifyErr != nil {
		if rec.Status == "" || rec.Status == types.PromotionCandidateQueued || rec.Status == types.PromotionCandidateIntegrated {
			rec.Status = types.PromotionCandidateVerificationFailed
		}
		rec.Error = verifyErr.Error()
		if updated, updateErr := rt.store.UpdatePromotionCandidate(ctx, rec); updateErr == nil {
			rec = updated
		}
		rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidateFailed, verifyErr)
		return rec, verifyErr
	}
	rec.Status = types.PromotionCandidateVerified
	rec.Error = ""
	rec, err = rt.store.UpdatePromotionCandidate(ctx, rec)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidateVerified, nil)
	return rec, nil
}

func (rt *Runtime) failPromotionCandidate(ctx context.Context, rec types.PromotionCandidateRecord, cause error) (types.PromotionCandidateRecord, error) {
	if cause == nil {
		cause = fmt.Errorf("promotion candidate failed")
	}
	rec.Status = types.PromotionCandidateVerificationFailed
	rec.Error = cause.Error()
	if rt != nil && rt.store != nil {
		if updated, updateErr := rt.store.UpdatePromotionCandidate(ctx, rec); updateErr == nil {
			rec = updated
		}
		rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidateFailed, cause)
	}
	return rec, cause
}

func (rt *Runtime) preparePromotionWorkspace(ctx context.Context, rec types.PromotionCandidateRecord) (repoPath, reportPath string, err error) {
	if strings.TrimSpace(rec.BaseSHA) == "" {
		return "", "", fmt.Errorf("promotion workspace: base_sha is required")
	}
	if !promotionArtifactPathAllowed(rt, rec.ManifestPath) {
		return "", "", fmt.Errorf("promotion workspace: manifest_path must point to a server-owned promotion artifact")
	}
	if !promotionArtifactPathAllowed(rt, rec.PatchsetPath) {
		return "", "", fmt.Errorf("promotion workspace: patchset_path must point to a server-owned promotion artifact")
	}
	sourceRepo := strings.TrimSpace(rt.cfg.PromotionSourceRepo)
	if sourceRepo == "" {
		return "", "", fmt.Errorf("promotion workspace: RUNTIME_PROMOTION_SOURCE_REPO is not configured")
	}
	root := strings.TrimSpace(rt.cfg.PromotionWorkspaceRoot)
	if root == "" {
		return "", "", fmt.Errorf("promotion workspace: RUNTIME_PROMOTION_WORKSPACE_ROOT is not configured")
	}
	candidateDir, err := safePromotionChildPath(root, rec.CandidateID)
	if err != nil {
		return "", "", err
	}
	if err := os.RemoveAll(candidateDir); err != nil {
		return "", "", fmt.Errorf("promotion workspace: clear candidate workspace: %w", err)
	}
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		return "", "", fmt.Errorf("promotion workspace: create candidate workspace: %w", err)
	}
	repoPath = filepath.Join(candidateDir, "repo")
	if _, err := runPromotionGit(ctx, "", "clone", "--no-checkout", sourceRepo, repoPath); err != nil {
		return "", "", fmt.Errorf("promotion workspace: clone canonical source: %w", err)
	}
	if _, err := runPromotionGit(ctx, repoPath, "config", "user.name", "Choir Promotion Verifier"); err != nil {
		return "", "", err
	}
	if _, err := runPromotionGit(ctx, repoPath, "config", "user.email", "promotion-verifier@choir.local"); err != nil {
		return "", "", err
	}
	destination := strings.TrimSpace(rec.DestinationBranch)
	if destination == "" {
		destination = "main"
	}
	if _, err := runPromotionGit(ctx, repoPath, "switch", "-C", destination, rec.BaseSHA); err != nil {
		return "", "", fmt.Errorf("promotion workspace: checkout base %s: %w", rec.BaseSHA, err)
	}
	return repoPath, filepath.Join(candidateDir, "promotion-report.json"), nil
}

func productPromotionVerifierContracts(rec types.PromotionCandidateRecord) []promotion.VerifierContract {
	target := "go-choir"
	if strings.TrimSpace(rec.Summary) != "" {
		target = rec.Summary
	}
	return []promotion.VerifierContract{{
		ContractID:        "product-safe-patch-import",
		Target:            target,
		Purpose:           "Verify the worker export imports cleanly against canonical go-choir base without mutating canonical state.",
		Invariants:        []string{"browser callers do not provide repo paths", "canonical branch remains unchanged during verification"},
		RequiredChecks:    []string{"git diff --check HEAD~1..HEAD"},
		CapabilityProfile: "server-owned-promotion-workspace",
		EvidencePaths:     []string{rec.ManifestPath, rec.PatchsetPath},
	}}
}

func promotionArtifactPathAllowed(rt *Runtime, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	root := promotionArtifactRoot(rt)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func safePromotionChildPath(root, child string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("promotion workspace: root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("promotion workspace: resolve root: %w", err)
	}
	child = sanitizeExportPart(child)
	if child == "" {
		child = "candidate"
	}
	path := filepath.Join(absRoot, child)
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return "", fmt.Errorf("promotion workspace: resolve candidate path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("promotion workspace: candidate path escapes root")
	}
	return path, nil
}

func runPromotionGit(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if strings.TrimSpace(repo) != "" {
		cmd.Dir = repo
	}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(out.String()), err)
	}
	return out.String(), nil
}

// PromotePromotionCandidate applies an already verified candidate to the
// destination branch. The caller must pass approved=true; otherwise canonical
// state is left untouched.
func (rt *Runtime) PromotePromotionCandidate(ctx context.Context, ownerID, candidateID, repoPath string, approved bool) (types.PromotionCandidateRecord, error) {
	if rt == nil || rt.store == nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("promote promotion candidate: runtime store is unavailable")
	}
	rec, err := rt.store.GetPromotionCandidate(ctx, ownerID, candidateID)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	var report promotion.Report
	if len(rec.ReportJSON) == 0 || !json.Valid(rec.ReportJSON) {
		return rec, fmt.Errorf("promote promotion candidate: report_json is missing")
	}
	if err := json.Unmarshal(rec.ReportJSON, &report); err != nil {
		return rec, fmt.Errorf("decode promotion report: %w", err)
	}
	if !report.PromotionApproved {
		return rec, fmt.Errorf("promote promotion candidate: owner approval is required")
	}
	promoted, err := promotion.ApplyVerifiedPromotion(ctx, repoPath, &report, approved)
	if err != nil {
		rec.Error = err.Error()
		if updated, updateErr := rt.store.UpdatePromotionCandidate(ctx, rec); updateErr == nil {
			rec = updated
		}
		return rec, err
	}
	rec = updateRecordFromPromotionReport(rec, promoted)
	rec.Status = types.PromotionCandidatePromoted
	rec.Error = ""
	rec, err = rt.store.UpdatePromotionCandidate(ctx, rec)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidatePromoted, nil)
	return rec, nil
}

// ReviewPromotionCandidate records the owner's product-visible approve/reject
// decision without mutating canonical git state.
func (rt *Runtime) ReviewPromotionCandidate(ctx context.Context, ownerID, candidateID, decision string) (types.PromotionCandidateRecord, error) {
	if rt == nil || rt.store == nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("review promotion candidate: runtime store is unavailable")
	}
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "approve" && decision != "reject" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("review promotion candidate: decision must be approve or reject")
	}
	rec, err := rt.store.GetPromotionCandidate(ctx, ownerID, candidateID)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	if rec.Status == types.PromotionCandidatePromoted {
		return rec, fmt.Errorf("review promotion candidate: promoted candidates cannot be reviewed")
	}

	report := promotion.Report{}
	if len(rec.ReportJSON) > 0 && json.Valid(rec.ReportJSON) && strings.TrimSpace(string(rec.ReportJSON)) != "{}" {
		if err := json.Unmarshal(rec.ReportJSON, &report); err != nil {
			return rec, fmt.Errorf("decode promotion report: %w", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch decision {
	case "approve":
		if rec.Status != types.PromotionCandidateVerified || report.Status != "verified" {
			return rec, fmt.Errorf("review promotion candidate: approval requires verified status")
		}
		report.PromotionApproved = true
		report.PromotionDecisionAt = now
		rec.Error = ""
	case "reject":
		report.PromotionApproved = false
		report.PromotionDecisionAt = now
		rec.Status = types.PromotionCandidateRejected
		rec.Error = ""
	}
	if data, err := json.Marshal(report); err == nil {
		rec.ReportJSON = data
	}
	rec, err = rt.store.UpdatePromotionCandidate(ctx, rec)
	if err != nil {
		return types.PromotionCandidateRecord{}, err
	}
	rt.emitPromotionQueueEvent(ctx, rec, types.EventPromotionCandidateReviewed, nil)
	return rec, nil
}

func candidateWorldFromRecord(rec types.PromotionCandidateRecord) (promotion.CandidateWorld, error) {
	var candidate promotion.CandidateWorld
	if len(rec.CandidateJSON) > 0 && json.Valid(rec.CandidateJSON) && strings.TrimSpace(string(rec.CandidateJSON)) != "{}" {
		if err := json.Unmarshal(rec.CandidateJSON, &candidate); err != nil {
			return promotion.CandidateWorld{}, fmt.Errorf("decode promotion candidate: %w", err)
		}
	}
	if candidate.CandidateID == "" {
		candidate.CandidateID = rec.CandidateID
	}
	if candidate.OwnerID == "" {
		candidate.OwnerID = rec.OwnerID
	}
	if candidate.ParentRunID == "" {
		candidate.ParentRunID = rec.SourceRunID
	}
	if candidate.CandidateRunID == "" {
		candidate.CandidateRunID = rec.SourceRunID
	}
	if candidate.VMID == "" {
		candidate.VMID = rec.VMID
	}
	if candidate.SnapshotID == "" {
		candidate.SnapshotID = rec.SnapshotID
	}
	if candidate.BaseSHA == "" {
		candidate.BaseSHA = rec.BaseSHA
	}
	if candidate.WorkerHeadSHA == "" {
		candidate.WorkerHeadSHA = rec.WorkerHeadSHA
	}
	if candidate.ManifestPath == "" {
		candidate.ManifestPath = rec.ManifestPath
	}
	if candidate.PatchsetPath == "" {
		candidate.PatchsetPath = rec.PatchsetPath
	}
	if candidate.IntegrationBranch == "" {
		candidate.IntegrationBranch = rec.IntegrationBranch
	}
	if candidate.Purpose == "" {
		candidate.Purpose = rec.Summary
	}
	if candidate.CreatedAt == "" {
		candidate.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return candidate, nil
}

func verifierContractsFromRecord(rec types.PromotionCandidateRecord) ([]promotion.VerifierContract, error) {
	if len(rec.ContractsJSON) == 0 {
		return nil, nil
	}
	var contracts []promotion.VerifierContract
	if err := json.Unmarshal(rec.ContractsJSON, &contracts); err != nil {
		return nil, fmt.Errorf("decode verifier contracts: %w", err)
	}
	return contracts, nil
}

func updateRecordFromPromotionReport(rec types.PromotionCandidateRecord, report *promotion.Report) types.PromotionCandidateRecord {
	if report == nil {
		return rec
	}
	if data, err := json.Marshal(report); err == nil {
		rec.ReportJSON = data
	}
	rec.CandidateID = report.Candidate.CandidateID
	rec.OwnerID = report.Candidate.OwnerID
	if rec.SourceRunID == "" {
		rec.SourceRunID = report.Candidate.ParentRunID
		if rec.SourceRunID == "" {
			rec.SourceRunID = report.Candidate.CandidateRunID
		}
	}
	rec.VMID = report.Candidate.VMID
	rec.SnapshotID = report.Candidate.SnapshotID
	rec.BaseSHA = report.Candidate.BaseSHA
	rec.WorkerHeadSHA = report.Candidate.WorkerHeadSHA
	rec.ManifestPath = report.Candidate.ManifestPath
	rec.PatchsetPath = report.Candidate.PatchsetPath
	rec.IntegrationBranch = report.Candidate.IntegrationBranch
	rec.DestinationBranch = report.Rollback.DestinationBranch
	if data, err := json.Marshal(report.Candidate); err == nil {
		rec.CandidateJSON = data
	}
	if data, err := json.Marshal(report.VerifierContracts); err == nil {
		rec.ContractsJSON = data
	}
	switch report.Status {
	case "verified":
		rec.Status = types.PromotionCandidateVerified
	case "promoted":
		rec.Status = types.PromotionCandidatePromoted
	case "integrated":
		rec.Status = types.PromotionCandidateIntegrated
	case "verification_failed", "integration_failed":
		rec.Status = types.PromotionCandidateVerificationFailed
	}
	return rec
}

func commitMessageForCandidate(rec types.PromotionCandidateRecord) string {
	summary := strings.TrimSpace(rec.Summary)
	if summary == "" {
		summary = strings.TrimSpace(rec.CandidateID)
	}
	if summary == "" {
		summary = "candidate-world patchset"
	}
	return "Integrate candidate: " + summary
}

func (rt *Runtime) emitPromotionQueueEvent(ctx context.Context, rec types.PromotionCandidateRecord, kind types.EventKind, causeErr error) {
	if rt == nil || rt.store == nil || rec.SourceRunID == "" {
		return
	}
	run, err := rt.store.GetRun(ctx, rec.SourceRunID)
	if err != nil || run.OwnerID != rec.OwnerID {
		if err != nil && err != store.ErrNotFound {
			// Best-effort trace emission should not affect queue durability.
			return
		}
		return
	}
	payload := map[string]any{
		"candidate_id":       rec.CandidateID,
		"status":             rec.Status,
		"vm_id":              rec.VMID,
		"integration_branch": rec.IntegrationBranch,
		"destination_branch": rec.DestinationBranch,
	}
	if causeErr != nil {
		payload["error"] = causeErr.Error()
	}
	data, _ := json.Marshal(payload)
	rt.emitEvent(ctx, &run, kind, events.CauseToolExecution, data)
}
