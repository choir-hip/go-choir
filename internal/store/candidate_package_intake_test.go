package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCandidatePackageIntakeRoundTripKeepsEvidenceOnlyReviewBoundary(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	want := candidatePackageIntakeFixture()
	want.CreatedAt = createdAt
	want.UpdatedAt = createdAt.Add(time.Minute)

	stored, err := s.UpsertCandidatePackageIntake(ctx, want)
	if err != nil {
		t.Fatalf("upsert candidate package intake: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, stored, want)

	loaded, err := s.GetCandidatePackageIntake(ctx, want.OwnerID, want.IntakeID)
	if err != nil {
		t.Fatalf("get candidate package intake: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, loaded, want)

	listed, err := s.ListCandidatePackageIntakes(ctx, want.OwnerID, 10)
	if err != nil {
		t.Fatalf("list candidate package intakes: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed intakes length = %d, want 1: %+v", len(listed), listed)
	}
	assertCandidatePackageIntakeRecord(t, listed[0], want)
}

func TestCandidatePackageIntakeRoundTripPreservesAdoptionReadyAfterOwnerApproval(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	want := candidatePackageIntakeFixture()
	want.IntakeID = "intake-approved-adoption-ready"
	want.Status = types.CandidatePackageIntakeOwnerApproved
	want.OwnerReviewState = types.CandidatePackageOwnerReviewApproved
	want.OwnerReviewRequired = false
	want.AdoptionReady = true
	want.AdoptionBlockersJSON = json.RawMessage(`[]`)
	want.AcceptanceJSON = json.RawMessage(`{"accepted":true,"reason":"owner approved review and no adoption blockers"}`)
	want.TraceID = "trace-adoption-ready-approved"
	want.CreatedAt = createdAt
	want.UpdatedAt = createdAt.Add(time.Minute)

	stored, err := s.UpsertCandidatePackageIntake(ctx, want)
	if err != nil {
		t.Fatalf("upsert adoption-ready candidate package intake: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, stored, want)

	loaded, err := s.GetCandidatePackageIntake(ctx, want.OwnerID, want.IntakeID)
	if err != nil {
		t.Fatalf("get adoption-ready candidate package intake: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, loaded, want)

	listed, err := s.ListCandidatePackageIntakes(ctx, want.OwnerID, 10)
	if err != nil {
		t.Fatalf("list adoption-ready candidate package intakes: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed intakes length = %d, want 1: %+v", len(listed), listed)
	}
	assertCandidatePackageIntakeRecord(t, listed[0], want)
}

func TestCandidatePackageIntakeUpdatePreservesIntakeIDAndReplacesReviewEvidence(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	initial := candidatePackageIntakeFixture()
	initial.IntakeID = "intake-update"
	initial.CreatedAt = time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)
	initial.UpdatedAt = initial.CreatedAt.Add(time.Minute)

	stored, err := s.UpsertCandidatePackageIntake(ctx, initial)
	if err != nil {
		t.Fatalf("upsert initial candidate package intake: %v", err)
	}

	updated := stored
	updated.Status = types.CandidatePackageIntakeOwnerApproved
	updated.OwnerReviewState = types.CandidatePackageOwnerReviewApproved
	updated.OwnerReviewRequired = false
	updated.VerifierContractsJSON = json.RawMessage(`[{"name":"owner-review","state":"passed","evidence_ref":"texture://review/approved"}]`)
	updated.EvidenceRefsJSON = json.RawMessage(`[{"kind":"owner_review","ref":"texture://review/approved","summary":"owner approved package evidence"}]`)
	updated.UpdatedAt = stored.UpdatedAt.Add(2 * time.Hour)
	updated.TraceID = "trace-owner-approved"

	storedUpdate, err := s.UpsertCandidatePackageIntake(ctx, updated)
	if err != nil {
		t.Fatalf("upsert updated candidate package intake: %v", err)
	}
	if storedUpdate.IntakeID != stored.IntakeID {
		t.Fatalf("updated intake id = %q, want %q", storedUpdate.IntakeID, stored.IntakeID)
	}

	loaded, err := s.GetCandidatePackageIntake(ctx, initial.OwnerID, initial.IntakeID)
	if err != nil {
		t.Fatalf("get updated candidate package intake: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, loaded, updated)
}

func TestCandidatePackageIntakeRejectsCrossOwnerUpsertTakeover(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC)
	ownerA := candidatePackageIntakeFixture()
	ownerA.IntakeID = "intake-cross-owner"
	ownerA.OwnerID = "owner-a"
	ownerA.CandidatePackageID = "candidate-package-owner-a"
	ownerA.TraceID = "trace-owner-a"
	ownerA.CreatedAt = createdAt
	ownerA.UpdatedAt = createdAt.Add(time.Minute)

	storedOwnerA, err := s.UpsertCandidatePackageIntake(ctx, ownerA)
	if err != nil {
		t.Fatalf("upsert owner A candidate package intake: %v", err)
	}

	ownerB := ownerA
	ownerB.OwnerID = "owner-b"
	ownerB.CandidatePackageID = "candidate-package-owner-b"
	ownerB.CandidatePackageManifestSHA256 = "sha256:owner-b-takeover"
	ownerB.TraceID = "trace-owner-b-takeover"
	ownerB.UpdatedAt = ownerA.UpdatedAt.Add(time.Hour)
	_, err = s.UpsertCandidatePackageIntake(ctx, ownerB)
	if err == nil {
		t.Fatalf("cross-owner upsert takeover error = nil, want rejection")
	}
	if !strings.Contains(err.Error(), "belongs to a different owner") {
		t.Fatalf("cross-owner upsert takeover error = %q, want owner mismatch", err.Error())
	}

	loadedOwnerA, err := s.GetCandidatePackageIntake(ctx, ownerA.OwnerID, ownerA.IntakeID)
	if err != nil {
		t.Fatalf("get owner A candidate package intake after rejected takeover: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, loadedOwnerA, storedOwnerA)

	ownerBIntakes, err := s.ListCandidatePackageIntakes(ctx, ownerB.OwnerID, 10)
	if err != nil {
		t.Fatalf("list owner B candidate package intakes after rejected takeover: %v", err)
	}
	if len(ownerBIntakes) != 0 {
		t.Fatalf("owner B intakes after rejected takeover = %+v, want none", ownerBIntakes)
	}
}

func TestUpdateCandidatePackageIntakeIfCurrentRejectsStaleUpdatedAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 4, 16, 0, 0, 0, time.UTC)
	initial := candidatePackageIntakeFixture()
	initial.IntakeID = "intake-stale-update"
	initial.CreatedAt = createdAt
	initial.UpdatedAt = createdAt.Add(time.Minute)

	stored, err := s.UpsertCandidatePackageIntake(ctx, initial)
	if err != nil {
		t.Fatalf("upsert initial candidate package intake: %v", err)
	}

	current := stored
	current.Status = types.CandidatePackageIntakeOwnerApproved
	current.OwnerReviewState = types.CandidatePackageOwnerReviewApproved
	current.OwnerReviewRequired = false
	current.AdoptionBlockersJSON = json.RawMessage(`[]`)
	current.VerifierContractsJSON = json.RawMessage(`[{"name":"owner-review","state":"passed","evidence_ref":"texture://review/intake-stale-update/current"}]`)
	current.EvidenceRefsJSON = json.RawMessage(`[{"kind":"owner_review","ref":"texture://review/intake-stale-update/current","summary":"current owner approval"}]`)
	current.AcceptanceJSON = json.RawMessage(`{"accepted":true,"reason":"current owner approval"}`)
	current.TraceID = "trace-intake-current"
	current.UpdatedAt = stored.UpdatedAt.Add(time.Hour)
	current, err = s.UpsertCandidatePackageIntake(ctx, current)
	if err != nil {
		t.Fatalf("upsert current candidate package intake: %v", err)
	}

	stale := stored
	stale.Status = types.CandidatePackageIntakeRejected
	stale.OwnerReviewState = types.CandidatePackageOwnerReviewRejected
	stale.OwnerReviewRequired = false
	stale.AdoptionBlockersJSON = json.RawMessage(`["owner_review_rejected"]`)
	stale.VerifierContractsJSON = json.RawMessage(`[{"name":"owner-review","state":"failed","evidence_ref":"texture://review/intake-stale-update/stale"}]`)
	stale.EvidenceRefsJSON = json.RawMessage(`[{"kind":"owner_review","ref":"texture://review/intake-stale-update/stale","summary":"stale owner rejection"}]`)
	stale.TraceID = "trace-intake-stale"

	got, err := s.UpdateCandidatePackageIntakeIfCurrent(ctx, stale, stored.UpdatedAt)
	if !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale candidate package intake update error = %v, want ErrConcurrentStateChange", err)
	}
	assertCandidatePackageIntakeRecord(t, got, current)

	loaded, err := s.GetCandidatePackageIntake(ctx, stored.OwnerID, stored.IntakeID)
	if err != nil {
		t.Fatalf("get candidate package intake after stale update: %v", err)
	}
	assertCandidatePackageIntakeRecord(t, loaded, current)
}

func TestUpdateAppAdoptionIfCurrentRejectsStaleUpdatedAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 4, 17, 0, 0, 0, time.UTC)
	initial := appAdoptionFixture()
	initial.CreatedAt = createdAt
	initial.UpdatedAt = createdAt.Add(time.Minute)

	stored, err := s.UpsertAppAdoption(ctx, initial)
	if err != nil {
		t.Fatalf("upsert initial app adoption: %v", err)
	}

	current := stored
	current.Status = types.AppAdoptionOwnerApproved
	current.VerifierResultsJSON = json.RawMessage(`[{"name":"owner-review","state":"passed","evidence_ref":"texture://adoption-review/current"}]`)
	current.RollbackProfileJSON = json.RawMessage(`{"state":"owner_approved","evidence_ref":"texture://rollback/current"}`)
	current.TraceID = "trace-adoption-current"
	current.UpdatedAt = stored.UpdatedAt.Add(time.Hour)
	current, err = s.UpsertAppAdoption(ctx, current)
	if err != nil {
		t.Fatalf("upsert current app adoption: %v", err)
	}

	stale := stored
	stale.Status = types.AppAdoptionOwnerReviewRejected
	stale.VerifierResultsJSON = json.RawMessage(`[{"name":"owner-review","state":"failed","evidence_ref":"texture://adoption-review/stale"}]`)
	stale.RollbackProfileJSON = json.RawMessage(`{"state":"rejected","evidence_ref":"texture://rollback/stale"}`)
	stale.TraceID = "trace-adoption-stale"

	got, err := s.UpdateAppAdoptionIfCurrent(ctx, stale, stored.UpdatedAt)
	if !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale app adoption update error = %v, want ErrConcurrentStateChange", err)
	}
	assertAppAdoptionRecord(t, got, current)

	loaded, err := s.GetAppAdoption(ctx, stored.OwnerID, stored.AdoptionID)
	if err != nil {
		t.Fatalf("get app adoption after stale update: %v", err)
	}
	assertAppAdoptionRecord(t, loaded, current)
}
func TestCandidatePackageIntakeRejectsUnsafeOrIncompleteRecords(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		mutate  func(*types.CandidatePackageIntakeRecord)
		wantErr string
	}{
		{
			name: "missing owner",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.OwnerID = ""
			},
			wantErr: "owner_id is required",
		},
		{
			name: "missing candidate package id",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.CandidatePackageID = ""
			},
			wantErr: "candidate_package_id is required",
		},
		{
			name: "missing package hash",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.CandidatePackageManifestSHA256 = ""
			},
			wantErr: "candidate_package_manifest_sha256 is required",
		},
		{
			name: "missing intake boundary",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.IntakeBoundary = ""
			},
			wantErr: "intake_boundary is required",
		},
		{
			name: "pending adoption ready",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.AdoptionReady = true
			},
			wantErr: "adoption_ready requires owner-approved review state",
		},
		{
			name: "owner approved adoption ready with blockers",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.Status = types.CandidatePackageIntakeOwnerApproved
				rec.OwnerReviewState = types.CandidatePackageOwnerReviewApproved
				rec.OwnerReviewRequired = false
				rec.AdoptionReady = true
			},
			wantErr: "adoption_ready requires no adoption blockers",
		},
		{
			name: "invalid adoption blockers json",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.AdoptionBlockersJSON = json.RawMessage(`[{`)
			},
			wantErr: "adoption_blockers_json is not valid JSON",
		},
		{
			name: "invalid verifier contracts json",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.VerifierContractsJSON = json.RawMessage(`{"unterminated"`)
			},
			wantErr: "verifier_contracts_json is not valid JSON",
		},
		{
			name: "invalid evidence refs json",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.EvidenceRefsJSON = json.RawMessage(`not-json`)
			},
			wantErr: "evidence_refs_json is not valid JSON",
		},
		{
			name: "invalid required observations json",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.RequiredObservationsJSON = json.RawMessage(`]`)
			},
			wantErr: "required_observations_json is not valid JSON",
		},
		{
			name: "invalid acceptance json",
			mutate: func(rec *types.CandidatePackageIntakeRecord) {
				rec.AcceptanceJSON = json.RawMessage(`{"accepted":`)
			},
			wantErr: "acceptance_json is not valid JSON",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := candidatePackageIntakeFixture()
			tc.mutate(&rec)

			_, err := s.UpsertCandidatePackageIntake(ctx, rec)
			if err == nil {
				t.Fatalf("upsert candidate package intake error = nil, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("upsert candidate package intake error = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func candidatePackageIntakeFixture() types.CandidatePackageIntakeRecord {
	return types.CandidatePackageIntakeRecord{
		IntakeID:                       "intake-review-1",
		OwnerID:                        "owner-intake-1",
		CandidatePackageID:             "candidate-package-1",
		CandidatePackageManifestSHA256: "sha256:0123456789abcdef",
		SourceComputerID:               "source-computer-1",
		SourceCandidateID:              "source-candidate-1",
		CandidateSourceRef:             "texture://candidate/source/ref",
		IntakeBoundary:                 "evidence_only_owner_review",
		Status:                         types.CandidatePackageIntakeOwnerReviewPending,
		OwnerReviewState:               types.CandidatePackageOwnerReviewRequired,
		OwnerReviewRequired:            true,
		AdoptionReady:                  false,
		AdoptionBlockersJSON:           json.RawMessage(`[{"code":"owner_review_required","message":"owner must review evidence before adoption"}]`),
		VerifierContractsJSON:          json.RawMessage(`[{"name":"manifest-hash","state":"pending","requires":"package hash must match intake evidence"}]`),
		EvidenceRefsJSON:               json.RawMessage(`[{"kind":"candidate_package","ref":"texture://evidence/package-1","sha256":"sha256:0123456789abcdef"}]`),
		RequiredObservationsJSON:       json.RawMessage(`[{"name":"owner_review","required":true},{"name":"hash_match","required":true}]`),
		AcceptanceJSON:                 json.RawMessage(`{"accepted":false,"reason":"owner review pending"}`),
		TraceID:                        "trace-intake-1",
	}
}

func appAdoptionFixture() types.AppAdoptionRecord {
	return types.AppAdoptionRecord{
		AdoptionID:                            "adoption-stale-update",
		OwnerID:                               "owner-adoption-1",
		PackageID:                             "package-adoption-1",
		AppID:                                 "app-adoption-1",
		TargetComputerID:                      "target-computer-1",
		TargetComputerKind:                    "user",
		TargetCandidateID:                     "target-candidate-1",
		Status:                                types.AppAdoptionOwnerReviewPending,
		TargetActiveSourceRefAtCandidateStart: "refs/computers/target-computer-1/active-before-candidate",
		TargetActiveSourceRefAtCutover:        "",
		CandidateSourceRef:                    "refs/computers/target-computer-1/candidates/target-candidate-1",
		ForegroundTailMergeResult:             "no-conflict",
		MergeStrategy:                         "foreground-tail",
		MergeConflictsJSON:                    json.RawMessage(`[]`),
		RuntimeArtifactDigest:                 "sha256:runtime-adoption",
		UIArtifactDigest:                      "sha256:ui-adoption",
		VerifierResultsJSON:                   json.RawMessage(`[{"name":"owner-review","state":"pending","evidence_ref":"texture://adoption-review/pending"}]`),
		RollbackProfileJSON:                   json.RawMessage(`{"state":"pending"}`),
		RouteProfile:                          "primary",
		DefaultBaseProfile:                    "primary",
		TraceID:                               "trace-adoption-1",
	}
}

func assertAppAdoptionRecord(t *testing.T, got, want types.AppAdoptionRecord) {
	t.Helper()

	if got.AdoptionID != want.AdoptionID || got.OwnerID != want.OwnerID || got.PackageID != want.PackageID {
		t.Fatalf("adoption identity fields = (%q, %q, %q), want (%q, %q, %q)", got.AdoptionID, got.OwnerID, got.PackageID, want.AdoptionID, want.OwnerID, want.PackageID)
	}
	if got.AppID != want.AppID || got.TargetComputerID != want.TargetComputerID || got.TargetComputerKind != want.TargetComputerKind || got.TargetCandidateID != want.TargetCandidateID {
		t.Fatalf("adoption target fields = (%q, %q, %q, %q), want (%q, %q, %q, %q)", got.AppID, got.TargetComputerID, got.TargetComputerKind, got.TargetCandidateID, want.AppID, want.TargetComputerID, want.TargetComputerKind, want.TargetCandidateID)
	}
	if got.Status != want.Status {
		t.Fatalf("adoption status = %q, want %q", got.Status, want.Status)
	}
	if got.TargetActiveSourceRefAtCandidateStart != want.TargetActiveSourceRefAtCandidateStart || got.TargetActiveSourceRefAtCutover != want.TargetActiveSourceRefAtCutover || got.CandidateSourceRef != want.CandidateSourceRef {
		t.Fatalf("adoption source refs = (%q, %q, %q), want (%q, %q, %q)", got.TargetActiveSourceRefAtCandidateStart, got.TargetActiveSourceRefAtCutover, got.CandidateSourceRef, want.TargetActiveSourceRefAtCandidateStart, want.TargetActiveSourceRefAtCutover, want.CandidateSourceRef)
	}
	if got.ForegroundTailMergeResult != want.ForegroundTailMergeResult || got.MergeStrategy != want.MergeStrategy {
		t.Fatalf("adoption merge fields = (%q, %q), want (%q, %q)", got.ForegroundTailMergeResult, got.MergeStrategy, want.ForegroundTailMergeResult, want.MergeStrategy)
	}
	assertCandidatePackageIntakeJSON(t, "adoption merge conflicts", got.MergeConflictsJSON, want.MergeConflictsJSON)
	if got.RuntimeArtifactDigest != want.RuntimeArtifactDigest || got.UIArtifactDigest != want.UIArtifactDigest {
		t.Fatalf("adoption artifact digests = (%q, %q), want (%q, %q)", got.RuntimeArtifactDigest, got.UIArtifactDigest, want.RuntimeArtifactDigest, want.UIArtifactDigest)
	}
	assertCandidatePackageIntakeJSON(t, "adoption verifier results", got.VerifierResultsJSON, want.VerifierResultsJSON)
	assertCandidatePackageIntakeJSON(t, "adoption rollback profile", got.RollbackProfileJSON, want.RollbackProfileJSON)
	if got.RouteProfile != want.RouteProfile || got.DefaultBaseProfile != want.DefaultBaseProfile || got.TraceID != want.TraceID || got.Error != want.Error {
		t.Fatalf("adoption metadata fields = (%q, %q, %q, %q), want (%q, %q, %q, %q)", got.RouteProfile, got.DefaultBaseProfile, got.TraceID, got.Error, want.RouteProfile, want.DefaultBaseProfile, want.TraceID, want.Error)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("adoption created at = %s, want %s", got.CreatedAt.Format(time.RFC3339Nano), want.CreatedAt.Format(time.RFC3339Nano))
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("adoption updated at = %s, want %s", got.UpdatedAt.Format(time.RFC3339Nano), want.UpdatedAt.Format(time.RFC3339Nano))
	}
}
func assertCandidatePackageIntakeRecord(t *testing.T, got, want types.CandidatePackageIntakeRecord) {
	t.Helper()

	if got.IntakeID != want.IntakeID || got.OwnerID != want.OwnerID || got.CandidatePackageID != want.CandidatePackageID {
		t.Fatalf("identity fields = (%q, %q, %q), want (%q, %q, %q)", got.IntakeID, got.OwnerID, got.CandidatePackageID, want.IntakeID, want.OwnerID, want.CandidatePackageID)
	}
	if got.CandidatePackageManifestSHA256 != want.CandidatePackageManifestSHA256 {
		t.Fatalf("package hash = %q, want %q", got.CandidatePackageManifestSHA256, want.CandidatePackageManifestSHA256)
	}
	if got.SourceComputerID != want.SourceComputerID || got.SourceCandidateID != want.SourceCandidateID || got.CandidateSourceRef != want.CandidateSourceRef {
		t.Fatalf("source refs = (%q, %q, %q), want (%q, %q, %q)", got.SourceComputerID, got.SourceCandidateID, got.CandidateSourceRef, want.SourceComputerID, want.SourceCandidateID, want.CandidateSourceRef)
	}
	if got.IntakeBoundary != want.IntakeBoundary {
		t.Fatalf("intake boundary = %q, want %q", got.IntakeBoundary, want.IntakeBoundary)
	}
	if got.Status != want.Status || got.OwnerReviewState != want.OwnerReviewState || got.OwnerReviewRequired != want.OwnerReviewRequired {
		t.Fatalf("owner review = (%q, %q, %v), want (%q, %q, %v)", got.Status, got.OwnerReviewState, got.OwnerReviewRequired, want.Status, want.OwnerReviewState, want.OwnerReviewRequired)
	}
	if got.AdoptionReady != want.AdoptionReady {
		t.Fatalf("adoption ready = %v, want %v", got.AdoptionReady, want.AdoptionReady)
	}
	assertCandidatePackageIntakeJSON(t, "adoption blockers", got.AdoptionBlockersJSON, want.AdoptionBlockersJSON)
	assertCandidatePackageIntakeJSON(t, "verifier contracts", got.VerifierContractsJSON, want.VerifierContractsJSON)
	assertCandidatePackageIntakeJSON(t, "evidence refs", got.EvidenceRefsJSON, want.EvidenceRefsJSON)
	assertCandidatePackageIntakeJSON(t, "required observations", got.RequiredObservationsJSON, want.RequiredObservationsJSON)
	assertCandidatePackageIntakeJSON(t, "acceptance", got.AcceptanceJSON, want.AcceptanceJSON)
	if got.TraceID != want.TraceID {
		t.Fatalf("trace id = %q, want %q", got.TraceID, want.TraceID)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created at = %s, want %s", got.CreatedAt.Format(time.RFC3339Nano), want.CreatedAt.Format(time.RFC3339Nano))
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("updated at = %s, want %s", got.UpdatedAt.Format(time.RFC3339Nano), want.UpdatedAt.Format(time.RFC3339Nano))
	}
}

func assertCandidatePackageIntakeJSON(t *testing.T, name string, got, want json.RawMessage) {
	t.Helper()

	if string(got) != string(want) {
		t.Fatalf("%s JSON = %s, want %s", name, got, want)
	}
}
