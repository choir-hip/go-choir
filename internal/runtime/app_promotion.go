package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type publishAppChangePackageInput struct {
	PackageID                   string          `json:"package_id,omitempty"`
	AppID                       string          `json:"app_id,omitempty"`
	Visibility                  string          `json:"visibility,omitempty"`
	SourceComputerID            string          `json:"source_computer_id"`
	SourceCandidateID           string          `json:"source_candidate_id"`
	SourceActiveRef             string          `json:"source_active_ref,omitempty"`
	CandidateSourceRef          string          `json:"candidate_source_ref,omitempty"`
	SourceLedgerRepo            string          `json:"source_ledger_repo,omitempty"`
	SourceLedgerBaseRef         string          `json:"source_ledger_base_ref,omitempty"`
	SourceLedgerCandidateRef    string          `json:"source_ledger_candidate_ref,omitempty"`
	SourceLedgerCommitSHA       string          `json:"source_ledger_commit_sha,omitempty"`
	RuntimeSourceDelta          string          `json:"runtime_source_delta"`
	UISourceDelta               string          `json:"ui_source_delta"`
	AppProtocolContract         string          `json:"app_protocol_contract"`
	SourceRuntimeArtifactDigest string          `json:"source_runtime_artifact_digest,omitempty"`
	SourceUIArtifactDigest      string          `json:"source_ui_artifact_digest,omitempty"`
	VerifierContracts           json.RawMessage `json:"verifier_contracts,omitempty"`
	ProvenanceRefs              json.RawMessage `json:"provenance_refs,omitempty"`
	TraceID                     string          `json:"trace_id,omitempty"`
}

type createAppAdoptionInput struct {
	AdoptionID                string `json:"adoption_id,omitempty"`
	PackageID                 string `json:"package_id"`
	TargetComputerKind        string `json:"target_computer_kind,omitempty"`
	TargetCandidateID         string `json:"target_candidate_id,omitempty"`
	CandidateSourceRef        string `json:"candidate_source_ref,omitempty"`
	RouteProfile              string `json:"route_profile,omitempty"`
	DefaultBaseProfile        string `json:"default_base_profile,omitempty"`
	TraceID                   string `json:"trace_id,omitempty"`
	ForegroundTailMergeResult string `json:"foreground_tail_merge_result,omitempty"`
	MergeStrategy             string `json:"merge_strategy,omitempty"`
}

type verifyAppAdoptionInput struct {
	TargetActiveSourceRefAtCutover string          `json:"target_active_source_ref_at_cutover,omitempty"`
	ForegroundTailMergeResult      string          `json:"foreground_tail_merge_result,omitempty"`
	MergeStrategy                  string          `json:"merge_strategy,omitempty"`
	MergeConflicts                 json.RawMessage `json:"merge_conflicts,omitempty"`
	Async                          bool            `json:"async,omitempty"`
}

type appAdoptionVerificationState struct {
	rec        types.AppAdoptionRecord
	pkg        types.AppChangePackageRecord
	cutoverRef string
}

func (rt *Runtime) EnsureComputerSourceLineage(ctx context.Context, ownerID, computerID, kind, activeRef string) (types.ComputerSourceLineageRecord, error) {
	if rt == nil || rt.store == nil {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("source lineage: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if ownerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("source lineage: owner_id is required")
	}
	if computerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("source lineage: computer_id is required")
	}
	if rec, err := rt.store.GetComputerSourceLineage(ctx, ownerID, computerID); err == nil {
		return rec, nil
	} else if err != store.ErrNotFound {
		return types.ComputerSourceLineageRecord{}, err
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = computerKindForID(computerID)
	}
	activeRef = strings.TrimSpace(activeRef)
	if activeRef == "" {
		activeRef = activeSourceRefForComputer(computerID, kind)
	}
	return rt.store.UpsertComputerSourceLineage(ctx, types.ComputerSourceLineageRecord{
		OwnerID:         ownerID,
		ComputerID:      computerID,
		ComputerKind:    kind,
		ActiveSourceRef: activeRef,
		RouteProfile:    safeRefPart(ownerID) + "/" + safeRefPart(computerID),
	})
}

func (rt *Runtime) PublishAppChangePackage(ctx context.Context, ownerID string, in publishAppChangePackageInput) (types.AppChangePackageRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: owner_id is required")
	}
	in.SourceComputerID = strings.TrimSpace(in.SourceComputerID)
	in.SourceCandidateID = strings.TrimSpace(in.SourceCandidateID)
	if in.SourceComputerID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: source_computer_id is required")
	}
	if in.SourceCandidateID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: source_candidate_id is required")
	}
	if strings.TrimSpace(in.RuntimeSourceDelta) == "" && strings.TrimSpace(in.UISourceDelta) == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: at least one source delta is required")
	}
	if err := rejectPrivateSourcePayload(in.RuntimeSourceDelta + "\n" + in.UISourceDelta + "\n" + in.AppProtocolContract); err != nil {
		return types.AppChangePackageRecord{}, err
	}
	appID := strings.TrimSpace(in.AppID)
	if appID == "" {
		appID = "podcast"
	}
	visibility := normalizePackageVisibility(in.Visibility)
	sourceLineage, err := rt.EnsureComputerSourceLineage(ctx, ownerID, in.SourceComputerID, "", in.SourceActiveRef)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	candidateRef := strings.TrimSpace(in.CandidateSourceRef)
	if candidateRef == "" {
		candidateRef = candidateSourceRefForComputer(in.SourceComputerID, sourceLineage.ComputerKind, in.SourceCandidateID)
	}
	if !strings.Contains(candidateRef, "/candidates/") {
		return types.AppChangePackageRecord{}, fmt.Errorf("publish app change package: candidate_source_ref must be a candidate ref")
	}
	packageID := strings.TrimSpace(in.PackageID)
	if packageID == "" {
		packageID = uuid.NewString()
	}
	runtimeDeltaHash := sha256Hex(in.RuntimeSourceDelta)
	uiDeltaHash := sha256Hex(in.UISourceDelta)
	contractHash := sha256Hex(in.AppProtocolContract)
	sourceRuntimeDigest := strings.TrimSpace(in.SourceRuntimeArtifactDigest)
	if sourceRuntimeDigest == "" {
		sourceRuntimeDigest = "sha256:" + digestParts("source-runtime", ownerID, in.SourceComputerID, in.SourceCandidateID, runtimeDeltaHash)
	}
	sourceUIDigest := strings.TrimSpace(in.SourceUIArtifactDigest)
	if sourceUIDigest == "" {
		sourceUIDigest = "sha256:" + digestParts("source-ui", ownerID, in.SourceComputerID, in.SourceCandidateID, uiDeltaHash)
	}
	manifest := map[string]any{
		"package_id":                     packageID,
		"app_id":                         appID,
		"owner_id":                       ownerID,
		"source_computer_id":             in.SourceComputerID,
		"source_candidate_id":            in.SourceCandidateID,
		"source_active_ref":              sourceLineage.ActiveSourceRef,
		"candidate_source_ref":           candidateRef,
		"runtime_source_delta_sha256":    runtimeDeltaHash,
		"ui_source_delta_sha256":         uiDeltaHash,
		"app_protocol_contract_sha256":   contractHash,
		"source_runtime_artifact_digest": sourceRuntimeDigest,
		"source_ui_artifact_digest":      sourceUIDigest,
		"visibility":                     visibility,
		"recipient_build_required":       true,
		"source_ledger_repo":             firstNonEmptyPromotion(strings.TrimSpace(in.SourceLedgerRepo), rt.cfg.SourceLedgerRepo),
		"source_ledger_base_ref":         strings.TrimSpace(in.SourceLedgerBaseRef),
		"source_ledger_candidate_ref":    firstNonEmptyPromotion(strings.TrimSpace(in.SourceLedgerCandidateRef), candidateRef),
		"source_ledger_commit_sha":       strings.TrimSpace(in.SourceLedgerCommitSHA),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	rec := types.AppChangePackageRecord{
		PackageID:                   packageID,
		OwnerID:                     ownerID,
		AppID:                       appID,
		Status:                      packageStatusForVisibility(visibility),
		Visibility:                  visibility,
		SourceComputerID:            in.SourceComputerID,
		SourceCandidateID:           in.SourceCandidateID,
		SourceActiveRef:             sourceLineage.ActiveSourceRef,
		CandidateSourceRef:          candidateRef,
		RuntimeSourceDelta:          in.RuntimeSourceDelta,
		UISourceDelta:               in.UISourceDelta,
		RuntimeSourceDeltaSHA256:    runtimeDeltaHash,
		UISourceDeltaSHA256:         uiDeltaHash,
		PackageManifestSHA256:       sha256Hex(string(manifestJSON)),
		AppProtocolContract:         in.AppProtocolContract,
		AppProtocolContractSHA256:   contractHash,
		SourceRuntimeArtifactDigest: sourceRuntimeDigest,
		SourceUIArtifactDigest:      sourceUIDigest,
		ManifestJSON:                manifestJSON,
		VerifierContractsJSON:       rawJSONOrFallback(in.VerifierContracts, "[]"),
		ProvenanceRefsJSON:          rawJSONOrFallback(in.ProvenanceRefs, "[]"),
		TraceID:                     strings.TrimSpace(in.TraceID),
	}
	rec, err = rt.store.UpsertAppChangePackage(ctx, rec)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppChangePackagePublished, "package", map[string]any{
		"package_id":               rec.PackageID,
		"app_id":                   rec.AppID,
		"status":                   rec.Status,
		"source_computer_id":       rec.SourceComputerID,
		"source_candidate_id":      rec.SourceCandidateID,
		"candidate_source_ref":     rec.CandidateSourceRef,
		"package_manifest_sha":     rec.PackageManifestSHA256,
		"source_ledger_repo":       manifest["source_ledger_repo"],
		"source_ledger_ref":        manifest["source_ledger_candidate_ref"],
		"source_ledger_commit_sha": manifest["source_ledger_commit_sha"],
		"recipient_build_required": true,
		"continuous_app_change":    true,
	})
	return rec, nil
}

func (rt *Runtime) CreateAppAdoption(ctx context.Context, ownerID, targetComputerID string, in createAppAdoptionInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("create app adoption: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	targetComputerID = strings.TrimSpace(targetComputerID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("create app adoption: owner_id is required")
	}
	if targetComputerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("create app adoption: target_computer_id is required")
	}
	pkg, err := rt.store.GetAppChangePackageForViewer(ctx, ownerID, strings.TrimSpace(in.PackageID))
	if err != nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("create app adoption: package not found or not visible")
	}
	targetKind := strings.TrimSpace(in.TargetComputerKind)
	if targetKind == "" {
		targetKind = computerKindForID(targetComputerID)
	}
	lineage, err := rt.EnsureComputerSourceLineage(ctx, ownerID, targetComputerID, targetKind, "")
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	targetCandidateID := strings.TrimSpace(in.TargetCandidateID)
	if targetCandidateID == "" {
		targetCandidateID = uuid.NewString()
	}
	candidateRef := strings.TrimSpace(in.CandidateSourceRef)
	if candidateRef == "" {
		candidateRef = candidateSourceRefForComputer(targetComputerID, targetKind, targetCandidateID)
	}
	rec := types.AppAdoptionRecord{
		AdoptionID:                            strings.TrimSpace(in.AdoptionID),
		OwnerID:                               ownerID,
		PackageID:                             pkg.PackageID,
		AppID:                                 pkg.AppID,
		TargetComputerID:                      targetComputerID,
		TargetComputerKind:                    targetKind,
		TargetCandidateID:                     targetCandidateID,
		Status:                                types.AppAdoptionCandidateApplied,
		TargetActiveSourceRefAtCandidateStart: lineage.ActiveSourceRef,
		CandidateSourceRef:                    candidateRef,
		ForegroundTailMergeResult:             strings.TrimSpace(in.ForegroundTailMergeResult),
		MergeStrategy:                         strings.TrimSpace(in.MergeStrategy),
		MergeConflictsJSON:                    json.RawMessage(`[]`),
		VerifierResultsJSON:                   json.RawMessage(`[]`),
		RollbackProfileJSON:                   json.RawMessage(`{}`),
		RouteProfile:                          normalizeRouteProfile(firstNonEmptyPromotion(strings.TrimSpace(in.RouteProfile), lineage.RouteProfile), ownerID, targetComputerID),
		DefaultBaseProfile:                    strings.TrimSpace(in.DefaultBaseProfile),
		TraceID:                               firstNonEmptyPromotion(strings.TrimSpace(in.TraceID), pkg.TraceID),
	}

	// If a Dolt promotion adapter is configured, create a fork tag at the
	// current HEAD. The fork tag is the rollback target: if the promotion
	// fails the health window, the adapter resets to this tag.
	//
	// This maps to the TLA+ ForkCandidate action. The fork tag is stored
	// in the rollback profile for later use by Promote and Rollback.
	//
	// If the adapter is nil or the fork fails, the adoption proceeds
	// without a Dolt fork tag (graceful degradation).
	if rt.promotionAdapter != nil {
		fork, forkErr := rt.promotionAdapter.Fork(ctx, targetCandidateID)
		if forkErr != nil {
			log.Printf("app promotion: create adoption: dolt fork for candidate %s: %v (proceeding without fork tag)", targetCandidateID, forkErr)
		} else {
			rec.RollbackProfileJSON = mergeRollbackProfile(rec.RollbackProfileJSON, map[string]any{
				"dolt_fork_tag":    fork.ForkTag,
				"dolt_fork_commit": fork.ForkCommit,
			})
		}
	}

	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionProposed, "adoption", map[string]any{
		"adoption_id":              rec.AdoptionID,
		"package_id":               rec.PackageID,
		"target_computer_id":       rec.TargetComputerID,
		"target_candidate_id":      rec.TargetCandidateID,
		"candidate_source_ref":     rec.CandidateSourceRef,
		"target_active_source_ref": rec.TargetActiveSourceRefAtCandidateStart,
		"continuous_app_change":    true,
	})
	return rec, nil
}

func (rt *Runtime) VerifyAppAdoption(ctx context.Context, ownerID, adoptionID string, in verifyAppAdoptionInput) (types.AppAdoptionRecord, error) {
	state, err := rt.startAppAdoptionVerification(ctx, ownerID, adoptionID, in)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	return rt.finishAppAdoptionVerification(ctx, ownerID, state)
}

func (rt *Runtime) StartVerifyAppAdoptionAsync(ctx context.Context, ownerID, adoptionID string, in verifyAppAdoptionInput) (types.AppAdoptionRecord, error) {
	state, err := rt.startAppAdoptionVerification(ctx, ownerID, adoptionID, in)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	go func() {
		if _, err := rt.finishAppAdoptionVerification(context.Background(), ownerID, state); err != nil {
			log.Printf("runtime: async app adoption verification adoption=%s: %v", state.rec.AdoptionID, err)
		}
	}()
	return state.rec, nil
}

func (rt *Runtime) startAppAdoptionVerification(ctx context.Context, ownerID, adoptionID string, in verifyAppAdoptionInput) (appAdoptionVerificationState, error) {
	rec, pkg, lineage, err := rt.loadAdoptionPackageLineage(ctx, ownerID, adoptionID)
	if err != nil {
		return appAdoptionVerificationState{}, err
	}
	cutoverRef := strings.TrimSpace(in.TargetActiveSourceRefAtCutover)
	if cutoverRef == "" {
		cutoverRef = lineage.ActiveSourceRef
	}
	if strings.TrimSpace(rec.TargetActiveSourceRefAtCandidateStart) == "" {
		rec.TargetActiveSourceRefAtCandidateStart = lineage.ActiveSourceRef
	}
	rec.TargetActiveSourceRefAtCutover = cutoverRef
	rec.ForegroundTailMergeResult = firstNonEmptyPromotion(strings.TrimSpace(in.ForegroundTailMergeResult), rec.ForegroundTailMergeResult, "no-conflict")
	rec.MergeStrategy = firstNonEmptyPromotion(strings.TrimSpace(in.MergeStrategy), rec.MergeStrategy, "rebase")
	rec.MergeConflictsJSON = rawJSONOrFallback(in.MergeConflicts, "[]")
	rec.RollbackProfileJSON = appAdoptionRollbackProfileJSON(rec, lineage)
	buildReport := appAdoptionBuildReport{Required: true}
	rec.Status = types.AppAdoptionVerifying
	rec.Error = ""
	startedResults := appAdoptionVerificationStartedResults(pkg, rec, buildReport)
	startedResultsJSON, _ := json.Marshal(startedResults)
	rec.VerifierResultsJSON = startedResultsJSON
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return appAdoptionVerificationState{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionVerificationStarted, "adoption", map[string]any{
		"adoption_id":                  rec.AdoptionID,
		"package_id":                   rec.PackageID,
		"target_computer_id":           rec.TargetComputerID,
		"target_candidate_id":          rec.TargetCandidateID,
		"candidate_source_ref":         rec.CandidateSourceRef,
		"target_active_ref_at_cutover": rec.TargetActiveSourceRefAtCutover,
		"recipient_build_required":     buildReport.Required,
		"recipient_build_status":       "started",
		"continuous_app_change":        true,
	})
	return appAdoptionVerificationState{rec: rec, pkg: pkg, cutoverRef: cutoverRef}, nil
}

func (rt *Runtime) finishAppAdoptionVerification(ctx context.Context, ownerID string, state appAdoptionVerificationState) (types.AppAdoptionRecord, error) {
	rec := state.rec
	pkg := state.pkg
	buildReport, buildErr := rt.materializeAppAdoptionCandidate(ctx, pkg, rec, state.cutoverRef)
	if buildErr != nil {
		buildReport.Required = true
		buildReport.Status = "failed"
		buildReport.Error = buildErr.Error()
	} else {
		rec.RuntimeArtifactDigest = buildReport.RuntimeArtifactDigest
		rec.UIArtifactDigest = buildReport.UIArtifactDigest
	}
	results, status, errText := verifierResultsForAppAdoption(pkg, rec, buildReport)
	resultsJSON, _ := json.Marshal(results)
	rec.VerifierResultsJSON = resultsJSON
	if status == "failed" {
		rec.Status = types.AppAdoptionBlocked
		rec.Error = errText
	} else {
		rec.Status = types.AppAdoptionVerified
		rec.Error = ""
	}
	var err error
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	kind := types.EventAppAdoptionVerified
	if rec.Status == types.AppAdoptionBlocked {
		kind = types.EventAppAdoptionBlocked
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, kind, "adoption", map[string]any{
		"adoption_id":                    rec.AdoptionID,
		"package_id":                     rec.PackageID,
		"target_computer_id":             rec.TargetComputerID,
		"runtime_artifact_digest":        rec.RuntimeArtifactDigest,
		"ui_artifact_digest":             rec.UIArtifactDigest,
		"foreground_tail_merge_result":   rec.ForegroundTailMergeResult,
		"target_active_ref_at_cutover":   rec.TargetActiveSourceRefAtCutover,
		"source_runtime_artifact_digest": pkg.SourceRuntimeArtifactDigest,
		"source_ui_artifact_digest":      pkg.SourceUIArtifactDigest,
		"recipient_build_required":       buildReport.Required,
		"recipient_build_status":         buildReport.Status,
		"recipient_build_head_sha":       buildReport.HeadSHA,
		"error":                          rec.Error,
		"continuous_app_change":          true,
	})
	if rec.Status == types.AppAdoptionBlocked {
		return rec, fmt.Errorf("verify app adoption: %s", rec.Error)
	}
	return rec, nil
}

// ApproveAppAdoption records the owner-approval gate: review authorizes a
// verified transition; it does not replace verification. Promotion requires
// this transition — verification alone never makes a change user-visible
// (specs/promotion_protocol.tla ApprovalGate).
func (rt *Runtime) ApproveAppAdoption(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	rec, pkg, _, err := rt.loadAdoptionPackageLineage(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.Status == types.AppAdoptionOwnerApproved {
		return rec, nil
	}
	if rec.Status != types.AppAdoptionVerified {
		return rec, fmt.Errorf("approve app adoption: adoption status %q is not verified", rec.Status)
	}
	rec.Status = types.AppAdoptionOwnerApproved
	rec.Error = ""
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionOwnerApproved, "adoption", map[string]any{
		"adoption_id":           rec.AdoptionID,
		"package_id":            pkg.PackageID,
		"target_computer_id":    rec.TargetComputerID,
		"continuous_app_change": true,
	})
	return rec, nil
}

// promoteFreshnessCAS enforces the spec's NoStaleCommit invariant: the
// foreground lineage must not have moved since this adoption was verified.
// Evidence about a stale base authorizes nothing — re-verify instead.
// Legacy records without the captured base skip the check.
func promoteFreshnessCAS(rec types.AppAdoptionRecord, lineage types.ComputerSourceLineageRecord) error {
	var rollback map[string]any
	if err := json.Unmarshal(rec.RollbackProfileJSON, &rollback); err != nil {
		return nil
	}
	baseRef, recorded := rollback["lineage_ref_at_verification"].(string)
	if !recorded {
		return nil
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(baseRef) {
		return fmt.Errorf("foreground lineage moved since verification (verified against %q, now %q); re-verify the adoption before promoting", baseRef, lineage.ActiveSourceRef)
	}
	return nil
}

func (rt *Runtime) PromoteAppAdoption(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	rec, pkg, lineage, err := rt.loadAdoptionPackageLineage(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.Status != types.AppAdoptionOwnerApproved {
		return rec, fmt.Errorf("promote app adoption: adoption status %q is not owner_approved; verification alone does not authorize promotion", rec.Status)
	}
	if rec.RuntimeArtifactDigest == "" || rec.UIArtifactDigest == "" {
		return rec, fmt.Errorf("promote app adoption: runtime/ui artifact digests are required")
	}
	rollbackSourceRef := rollbackSourceRefFromProfile(rec.RollbackProfileJSON)
	if rollbackSourceRef == "" {
		return rec, fmt.Errorf("promote app adoption: rollback source ref is required")
	}
	if err := promoteFreshnessCAS(rec, lineage); err != nil {
		return rec, fmt.Errorf("promote app adoption: %w", err)
	}
	rec.Status = types.AppAdoptionAdopted
	rec.Error = ""
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}

	// If a Dolt promotion adapter is configured and we have a fork tag,
	// create a promotion tag at the current HEAD. The promotion tag IS
	// part of the ArtifactProgramRef — it is the tamper-evident reference
	// to the promoted state.
	//
	// This maps to the TLA+ Commit action: it creates the merge tag
	// (promoMergeTag) that identifies the promoted state.
	//
	// If the adapter is nil or the promote fails, the adoption proceeds
	// without a Dolt promotion tag (graceful degradation). The lineage
	// update still happens.
	if rt.promotionAdapter != nil {
		forkTag := stringFromMap(jsonRawToMap(rec.RollbackProfileJSON), "dolt_fork_tag")
		if forkTag != "" {
			promo, promoErr := rt.promotionAdapter.Promote(ctx, rec.TargetCandidateID, forkTag)
			if promoErr != nil {
				log.Printf("app promotion: promote adoption %s: dolt promote: %v (proceeding without promotion tag)", rec.AdoptionID, promoErr)
			} else {
				rec.RollbackProfileJSON = mergeRollbackProfile(rec.RollbackProfileJSON, map[string]any{
					"dolt_promotion_tag": promo.PromotionTag,
					"dolt_merge_commit":   promo.MergeCommit,
				})
				rec, err = rt.store.UpsertAppAdoption(ctx, rec)
				if err != nil {
					return types.AppAdoptionRecord{}, err
				}
			}
		}
	}

	lineage.ActiveSourceRef = rec.CandidateSourceRef
	lineage.RuntimeDigest = rec.RuntimeArtifactDigest
	lineage.UIDigest = rec.UIArtifactDigest
	lineage.RouteProfile = normalizeRouteProfile(firstNonEmptyPromotion(rec.RouteProfile, lineage.RouteProfile), ownerID, rec.TargetComputerID)
	lineage.DefaultBaseProfile = firstNonEmptyPromotion(rec.DefaultBaseProfile, lineage.DefaultBaseProfile)
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = pkg.PackageID
	lineage.LastCandidateRef = rec.CandidateSourceRef
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionPromoted, "adoption", map[string]any{
		"adoption_id":             rec.AdoptionID,
		"package_id":              rec.PackageID,
		"target_computer_id":      rec.TargetComputerID,
		"candidate_source_ref":    rec.CandidateSourceRef,
		"runtime_artifact_digest": rec.RuntimeArtifactDigest,
		"ui_artifact_digest":      rec.UIArtifactDigest,
		"route_profile":           lineage.RouteProfile,
		"default_base_profile":    lineage.DefaultBaseProfile,
		"rollback_source_ref":     rollbackSourceRef,
		"continuous_app_change":   true,
	})
	return rec, nil
}

func (rt *Runtime) RollbackAppAdoption(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	rec, _, lineage, err := rt.loadAdoptionPackageLineage(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	var rollback map[string]any
	if err := json.Unmarshal(rec.RollbackProfileJSON, &rollback); err != nil {
		return rec, fmt.Errorf("rollback app adoption: rollback profile is invalid")
	}
	if stringFromMap(rollback, "previous_active_source_ref") == "" {
		return rec, fmt.Errorf("rollback app adoption: rollback source ref is missing")
	}

	// If a Dolt promotion adapter is configured and we have a fork tag,
	// reset the Dolt working set to the fork tag. This is the atomic
	// rollback operation: DOLT_RESET --hard to the fork tag.
	//
	// This maps to the TLA+ AutoRevert action: it restores the active
	// computer's artifact head to the pre-merge fork tag (promoForkTag).
	//
	// If the adapter is nil or the rollback fails, the lineage restore
	// still happens (graceful degradation). The Dolt reset is best-effort
	// and does not block the lineage rollback.
	if rt.promotionAdapter != nil {
		forkTag := stringFromMap(rollback, "dolt_fork_tag")
		if forkTag != "" {
			if rbErr := rt.promotionAdapter.Rollback(ctx, forkTag); rbErr != nil {
				log.Printf("app promotion: rollback adoption %s: dolt reset to fork tag %s: %v (lineage restore still proceeds)", rec.AdoptionID, forkTag, rbErr)
			}
		}
	}

	lineage.ActiveSourceRef = stringFromMap(rollback, "previous_active_source_ref")
	lineage.RuntimeDigest = stringFromMap(rollback, "previous_runtime_digest")
	lineage.UIDigest = stringFromMap(rollback, "previous_ui_digest")
	lineage.RouteProfile = normalizeRouteProfile(stringFromMap(rollback, "previous_route_profile"), ownerID, rec.TargetComputerID)
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = rec.PackageID
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return rec, err
	}
	rec.Status = types.AppAdoptionRolledBack
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionRolledBack, "adoption", map[string]any{
		"adoption_id":           rec.AdoptionID,
		"package_id":            rec.PackageID,
		"target_computer_id":    rec.TargetComputerID,
		"rollback_source_ref":   lineage.ActiveSourceRef,
		"continuous_app_change": true,
	})
	return rec, nil
}

func (rt *Runtime) RollForwardAppAdoption(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	rec, pkg, lineage, err := rt.loadAdoptionPackageLineage(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.Status != types.AppAdoptionRolledBack {
		return rec, fmt.Errorf("roll forward app adoption: adoption status %q is not rolled back", rec.Status)
	}
	if rec.RuntimeArtifactDigest == "" || rec.UIArtifactDigest == "" {
		return rec, fmt.Errorf("roll forward app adoption: runtime/ui artifact digests are required")
	}
	rollbackSourceRef := rollbackSourceRefFromProfile(rec.RollbackProfileJSON)
	if rollbackSourceRef == "" {
		return rec, fmt.Errorf("roll forward app adoption: rollback source ref is required")
	}
	if strings.TrimSpace(rec.CandidateSourceRef) == "" {
		return rec, fmt.Errorf("roll forward app adoption: verified source ref is required")
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(rollbackSourceRef) {
		return rec, fmt.Errorf("roll forward app adoption: foreground lineage moved since rollback (expected %q, now %q); re-verify before rolling forward", rollbackSourceRef, lineage.ActiveSourceRef)
	}
	rec.Status = types.AppAdoptionAdopted
	rec.Error = ""
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	lineage.ActiveSourceRef = rec.CandidateSourceRef
	lineage.RuntimeDigest = rec.RuntimeArtifactDigest
	lineage.UIDigest = rec.UIArtifactDigest
	lineage.RouteProfile = normalizeRouteProfile(firstNonEmptyPromotion(rec.RouteProfile, lineage.RouteProfile), ownerID, rec.TargetComputerID)
	lineage.DefaultBaseProfile = firstNonEmptyPromotion(rec.DefaultBaseProfile, lineage.DefaultBaseProfile)
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = pkg.PackageID
	lineage.LastCandidateRef = rec.CandidateSourceRef
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionPromoted, "adoption", map[string]any{
		"adoption_id":             rec.AdoptionID,
		"package_id":              rec.PackageID,
		"target_computer_id":      rec.TargetComputerID,
		"candidate_source_ref":    rec.CandidateSourceRef,
		"runtime_artifact_digest": rec.RuntimeArtifactDigest,
		"ui_artifact_digest":      rec.UIArtifactDigest,
		"route_profile":           lineage.RouteProfile,
		"default_base_profile":    lineage.DefaultBaseProfile,
		"rollback_source_ref":     rollbackSourceRef,
		"roll_forward":            true,
		"continuous_app_change":   true,
	})
	return rec, nil
}

func (rt *Runtime) loadAdoptionPackageLineage(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, types.AppChangePackageRecord, types.ComputerSourceLineageRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, types.AppChangePackageRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("app adoption: runtime store is unavailable")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, strings.TrimSpace(adoptionID))
	if err != nil {
		return rec, types.AppChangePackageRecord{}, types.ComputerSourceLineageRecord{}, err
	}
	pkg, err := rt.store.GetAppChangePackageForViewer(ctx, ownerID, rec.PackageID)
	if err != nil {
		return rec, pkg, types.ComputerSourceLineageRecord{}, err
	}
	lineage, err := rt.EnsureComputerSourceLineage(ctx, ownerID, rec.TargetComputerID, rec.TargetComputerKind, rec.TargetActiveSourceRefAtCandidateStart)
	return rec, pkg, lineage, err
}

func verifierResultsForAppAdoption(pkg types.AppChangePackageRecord, rec types.AppAdoptionRecord, buildReport appAdoptionBuildReport) ([]map[string]any, string, string) {
	results := []map[string]any{}
	add := func(id, status, summary string, details map[string]any) {
		if details == nil {
			details = map[string]any{}
		}
		results = append(results, map[string]any{
			"contract_id": id,
			"status":      status,
			"summary":     summary,
			"details":     details,
			"verified_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
	status := "passed"
	errText := ""
	add("source-refs-resolve", "passed", "package and target candidate source refs are recorded", map[string]any{
		"package_candidate_source_ref": pkg.CandidateSourceRef,
		"target_candidate_source_ref":  rec.CandidateSourceRef,
	})
	manifest := appChangePackageManifest(pkg)
	add("source-ledger-reference", "passed", "package records source-ledger provenance for the candidate source", map[string]any{
		"source_ledger_repo":          stringFromMap(manifest, "source_ledger_repo"),
		"source_ledger_candidate_ref": stringFromMap(manifest, "source_ledger_candidate_ref"),
		"source_ledger_commit_sha":    stringFromMap(manifest, "source_ledger_commit_sha"),
	})
	if strings.TrimSpace(pkg.RuntimeSourceDelta) == "" && strings.TrimSpace(pkg.UISourceDelta) == "" {
		status = "failed"
		errText = "at least one source delta is required"
		add("source-deltas-present", "failed", errText, nil)
	} else {
		add("source-deltas-present", "passed", "one or more source deltas are present", map[string]any{
			"runtime_source_delta_present": strings.TrimSpace(pkg.RuntimeSourceDelta) != "",
			"ui_source_delta_present":      strings.TrimSpace(pkg.UISourceDelta) != "",
		})
	}
	if strings.TrimSpace(pkg.PackageManifestSHA256) == "" {
		status = "failed"
		errText = "package manifest hash is missing"
		add("manifest-hash", "failed", errText, nil)
	} else {
		add("manifest-hash", "passed", "package manifest hash is present", map[string]any{"package_manifest_sha256": pkg.PackageManifestSHA256})
	}
	if rec.RuntimeArtifactDigest == "" || rec.UIArtifactDigest == "" {
		status = "failed"
		errText = "target runtime/UI artifact digests are required"
		add("no-cross-computer-binary-copying", "failed", errText, map[string]any{
			"source_runtime_artifact_digest": pkg.SourceRuntimeArtifactDigest,
			"target_runtime_artifact_digest": rec.RuntimeArtifactDigest,
			"source_ui_artifact_digest":      pkg.SourceUIArtifactDigest,
			"target_ui_artifact_digest":      rec.UIArtifactDigest,
		})
	} else if rec.RuntimeArtifactDigest == pkg.SourceRuntimeArtifactDigest || rec.UIArtifactDigest == pkg.SourceUIArtifactDigest {
		status = "failed"
		errText = "target runtime/UI digests must be rebuilt for the recipient"
		add("no-cross-computer-binary-copying", "failed", errText, map[string]any{
			"source_runtime_artifact_digest": pkg.SourceRuntimeArtifactDigest,
			"target_runtime_artifact_digest": rec.RuntimeArtifactDigest,
			"source_ui_artifact_digest":      pkg.SourceUIArtifactDigest,
			"target_ui_artifact_digest":      rec.UIArtifactDigest,
		})
	} else {
		add("no-cross-computer-binary-copying", "passed", "target artifact digests are recipient-specific", map[string]any{
			"source_runtime_artifact_digest": pkg.SourceRuntimeArtifactDigest,
			"target_runtime_artifact_digest": rec.RuntimeArtifactDigest,
			"source_ui_artifact_digest":      pkg.SourceUIArtifactDigest,
			"target_ui_artifact_digest":      rec.UIArtifactDigest,
		})
	}
	add("foreground-tail-accounted", "passed", "candidate cutover records foreground-tail merge result", map[string]any{
		"target_active_source_ref_at_candidate_start": rec.TargetActiveSourceRefAtCandidateStart,
		"target_active_source_ref_at_cutover":         rec.TargetActiveSourceRefAtCutover,
		"foreground_tail_merge_result":                rec.ForegroundTailMergeResult,
		"merge_strategy":                              rec.MergeStrategy,
	})
	details := map[string]any{
		"status":                  buildReport.Status,
		"workspace_path":          buildReport.WorkspacePath,
		"build_scratch_path":      buildReport.BuildScratchPath,
		"base_sha":                buildReport.BaseSHA,
		"head_sha":                buildReport.HeadSHA,
		"runtime_artifact_digest": buildReport.RuntimeArtifactDigest,
		"ui_artifact_digest":      buildReport.UIArtifactDigest,
		"commands":                buildReport.Commands,
		"error":                   buildReport.Error,
	}
	if buildReport.Status != "passed" {
		status = "failed"
		errText = firstNonEmptyPromotion(buildReport.Error, "recipient runtime/UI build failed")
		add("actual-recipient-runtime-ui-build", "failed", errText, details)
	} else {
		add("actual-recipient-runtime-ui-build", "passed", "runtime/UI digests were hashed from actual recipient build outputs", details)
	}
	return results, status, errText
}

func appAdoptionVerificationStartedResults(pkg types.AppChangePackageRecord, rec types.AppAdoptionRecord, buildReport appAdoptionBuildReport) []map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	manifest := appChangePackageManifest(pkg)
	return []map[string]any{
		{
			"contract_id": "source-refs-resolve",
			"status":      "passed",
			"summary":     "package and target candidate source refs are recorded before recipient verification",
			"details": map[string]any{
				"package_candidate_source_ref": pkg.CandidateSourceRef,
				"target_candidate_source_ref":  rec.CandidateSourceRef,
			},
			"verified_at": now,
		},
		{
			"contract_id": "source-ledger-reference",
			"status":      "passed",
			"summary":     "package records source-ledger provenance before recipient verification",
			"details": map[string]any{
				"source_ledger_repo":          stringFromMap(manifest, "source_ledger_repo"),
				"source_ledger_candidate_ref": stringFromMap(manifest, "source_ledger_candidate_ref"),
				"source_ledger_commit_sha":    stringFromMap(manifest, "source_ledger_commit_sha"),
			},
			"verified_at": now,
		},
		{
			"contract_id": "actual-recipient-runtime-ui-build",
			"status":      "running",
			"summary":     "recipient runtime/UI build started; terminal verifier result has not been recorded yet",
			"details": map[string]any{
				"required":           buildReport.Required,
				"adoption_id":        rec.AdoptionID,
				"target_computer_id": rec.TargetComputerID,
				"candidate_ref":      rec.CandidateSourceRef,
			},
			"verified_at": now,
		},
	}
}

func appAdoptionRollbackProfileJSON(rec types.AppAdoptionRecord, lineage types.ComputerSourceLineageRecord) json.RawMessage {
	rollback := map[string]any{
		"previous_active_source_ref": rec.TargetActiveSourceRefAtCutover,
		"previous_runtime_digest":    lineage.RuntimeDigest,
		"previous_ui_digest":         lineage.UIDigest,
		"previous_route_profile":     lineage.RouteProfile,
		"candidate_source_ref":       rec.CandidateSourceRef,
		"package_id":                 rec.PackageID,
		"adoption_id":                rec.AdoptionID,
		// Freshness base for the promote-time CAS: the foreground lineage ref
		// observed when verification started. Promotion is only valid while
		// the foreground has not moved past this point (specs/
		// promotion_protocol.tla NoStaleCommit).
		"lineage_ref_at_verification": lineage.ActiveSourceRef,
	}
	rollbackJSON, _ := json.Marshal(rollback)
	return rollbackJSON
}

func appChangePackageRequiresRecipientBuild(pkg types.AppChangePackageRecord) bool {
	return true
}

func appChangePackageManifest(pkg types.AppChangePackageRecord) map[string]any {
	var manifest map[string]any
	if err := json.Unmarshal(pkg.ManifestJSON, &manifest); err != nil || manifest == nil {
		return map[string]any{}
	}
	return manifest
}

func (rt *Runtime) emitAppPromotionEvent(ctx context.Context, ownerID, traceID string, kind types.EventKind, phase string, payload map[string]any) {
	if rt == nil || rt.store == nil {
		return
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("runtime: marshal app promotion event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.NewString(),
		OwnerID:      ownerID,
		TrajectoryID: traceID,
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Phase:        phase,
		Payload:      data,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist app promotion event %s: %v", evRec.EventID, err)
	}
}

func packageStatusForVisibility(visibility string) types.AppChangePackageStatus {
	switch normalizePackageVisibility(visibility) {
	case "public":
		return types.AppChangePackagePublishedPublic
	case "unlisted":
		return types.AppChangePackagePublishedUnlisted
	default:
		return types.AppChangePackagePublishedPrivate
	}
}

func normalizePackageVisibility(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "public":
		return "public"
	case "unlisted":
		return "unlisted"
	default:
		return "private"
	}
}

func computerKindForID(computerID string) string {
	if strings.Contains(strings.ToLower(computerID), "platform") {
		return "platform"
	}
	return "user"
}

func activeSourceRefForComputer(computerID, kind string) string {
	part := safeRefPart(computerID)
	if kind == "platform" {
		return "refs/platform-computers/" + part + "/active"
	}
	return "refs/computers/" + part + "/active"
}

func candidateSourceRefForComputer(computerID, kind, candidateID string) string {
	part := safeRefPart(computerID)
	candidatePart := safeRefPart(candidateID)
	if kind == "platform" {
		return "refs/platform-computers/" + part + "/candidates/" + candidatePart
	}
	return "refs/computers/" + part + "/candidates/" + candidatePart
}

func safeRefPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), "-.")
}

// normalizeRouteProfile converts a legacy "route:computerID" RouteProfile to
// the canonical "ownerID/computerID" format. If the profile is empty or
// malformed, a canonical value is synthesized from ownerID/computerID.
func normalizeRouteProfile(profile, ownerID, computerID string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
	}
	if strings.HasPrefix(profile, "route:") {
		legacyID := strings.TrimSpace(strings.TrimPrefix(profile, "route:"))
		if legacyID != "" {
			return safeRefPart(ownerID) + "/" + safeRefPart(legacyID)
		}
		return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
	}
	// Reject anything that is not exactly "owner/computer" with non-empty parts.
	parts := strings.Split(profile, "/")
	if len(parts) == 2 {
		ownerPart := strings.TrimSpace(parts[0])
		computerPart := strings.TrimSpace(parts[1])
		if ownerPart != "" && computerPart != "" {
			return ownerPart + "/" + computerPart
		}
	}
	return safeRefPart(ownerID) + "/" + safeRefPart(computerID)
}

func rejectPrivateSourcePayload(payload string) error {
	lower := strings.ToLower(payload)
	for _, needle := range []string{"api_key=", "secret=", "password=", "private_key", "provider_credentials", "uploaded_file:"} {
		if strings.Contains(lower, needle) {
			return fmt.Errorf("app change package source payload contains forbidden private material marker %q", needle)
		}
	}
	return nil
}

func rawJSONOrFallback(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	return raw
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func digestParts(parts ...string) string {
	return sha256Hex(strings.Join(parts, "\x00"))
}

func rollbackSourceRefFromProfile(raw json.RawMessage) string {
	var rollback map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &rollback) != nil {
		return ""
	}
	return stringFromMap(rollback, "previous_active_source_ref")
}

func firstNonEmptyPromotion(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

// jsonRawToMap unmarshals a json.RawMessage into a map[string]any.
// Returns an empty map if the input is nil or invalid.
func jsonRawToMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

// mergeRollbackProfile merges additional fields into an existing rollback
// profile JSON. If the existing profile is empty or invalid, a new one is
// created with just the additional fields.
func mergeRollbackProfile(existing json.RawMessage, additions map[string]any) json.RawMessage {
	base := jsonRawToMap(existing)
	for k, v := range additions {
		base[k] = v
	}
	out, err := json.Marshal(base)
	if err != nil {
		return existing
	}
	return out
}

func boolFromMap(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	switch value := m[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}
