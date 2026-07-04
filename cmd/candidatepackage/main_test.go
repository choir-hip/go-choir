package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRunEmitsCandidatePackageFromEvidenceRootOutputAndRealization(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, map[string]any{
		"base_fixture": map[string]any{"journal_path": filepath.Join(fixture.root, "base.sqlite")},
		"self_check":   map[string]any{"status": "equivalent"},
	})
	realizationPath := writeJSONFile(t, fixture.dir, "realization.json", fixture.realization)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-command-test",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--source-computer-id", "computer:source-command-test",
		"--source-candidate-id", "candidate:source-command-test",
		"--candidate-source-ref", "git:source-command-test",
		"--evidence-ref", "artifact:review-note",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	manifest := decodeCandidatePackage(t, stdout.Bytes())
	assertCandidatePackageSuccess(t, manifest, fixture, rootOutputPath, realizationPath)

	var secondStdout, secondStderr bytes.Buffer
	secondCode := run([]string{
		"--id", "candidate-package-command-test",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--source-computer-id", "computer:source-command-test",
		"--source-candidate-id", "candidate:source-command-test",
		"--candidate-source-ref", "git:source-command-test",
		"--evidence-ref", "artifact:review-note",
	}, &secondStdout, &secondStderr)
	if secondCode != 0 {
		t.Fatalf("second run exit = %d, want 0; stdout=%s stderr=%s", secondCode, secondStdout.String(), secondStderr.String())
	}
	secondManifest := decodeCandidatePackage(t, secondStdout.Bytes())
	if secondManifest.PackageManifestSHA256 != manifest.PackageManifestSHA256 {
		t.Fatalf("package hash changed across identical inputs: first=%q second=%q", manifest.PackageManifestSHA256, secondManifest.PackageManifestSHA256)
	}
}

func TestRunEmitsCandidatePackageAppChangeBridgePayload(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, map[string]any{
		"base_fixture": map[string]any{"journal_path": filepath.Join(fixture.root, "base.sqlite")},
		"self_check":   map[string]any{"status": "equivalent"},
	})
	realizationPath := writeJSONFile(t, fixture.dir, "realization.json", fixture.realization)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-command-test",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--source-computer-id", "computer:source-command-test",
		"--source-candidate-id", "candidate:source-command-test",
		"--candidate-source-ref", "git:source-command-test",
		"--evidence-ref", "artifact:review-note",
		"--output", "bridge",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertBridgeOutputDoesNotPublish(t, stdout.Bytes())

	bridge := decodeCandidatePackageAppChangeBridge(t, stdout.Bytes())
	if bridge.Kind != computerversion.CandidatePackageAppChangeBridgeKind {
		t.Fatalf("bridge kind = %q, want %q", bridge.Kind, computerversion.CandidatePackageAppChangeBridgeKind)
	}
	if bridge.CandidatePackageID != "candidate-package-command-test" {
		t.Fatalf("bridge candidate package id = %q, want candidate-package-command-test", bridge.CandidatePackageID)
	}
	if !strings.HasPrefix(bridge.CandidatePackageManifestSHA256, "sha256:") || len(bridge.CandidatePackageManifestSHA256) != len("sha256:")+64 {
		t.Fatalf("bridge package hash = %q, want sha256 digest", bridge.CandidatePackageManifestSHA256)
	}
	if bridge.DirectPublishReady {
		t.Fatalf("bridge direct_publish_ready = true, want false until source deltas are supplied")
	}
	wantBlockers := []string{
		"app_change_package_publish_requires_runtime_or_ui_source_delta",
		"candidate_computer_package_is_evidence_payload_not_product_source_delta",
	}
	if !reflect.DeepEqual(bridge.DirectPublishBlockers, wantBlockers) {
		t.Fatalf("bridge blockers = %#v, want source-delta blockers %#v", bridge.DirectPublishBlockers, wantBlockers)
	}
	wantRequired := []computerversion.ObservationKind{
		computerversion.ObservationBlobSet,
		computerversion.ObservationFileManifest,
		computerversion.ObservationPromotionCertificate,
		computerversion.ObservationVMStateManifest,
	}
	if !reflect.DeepEqual(bridge.RequiredObservations, wantRequired) {
		t.Fatalf("bridge required observations = %#v, want %#v", bridge.RequiredObservations, wantRequired)
	}

	var manifest struct {
		BridgeKind                     string                            `json:"bridge_kind"`
		CandidatePackageID             string                            `json:"candidate_package_id"`
		CandidatePackageManifestSHA256 string                            `json:"candidate_package_manifest_sha256"`
		CandidateComputerVersion       computerversion.ComputerVersion   `json:"candidate_computer_version"`
		SourceComputerID               string                            `json:"source_computer_id"`
		SourceCandidateID              string                            `json:"source_candidate_id"`
		CandidateSourceRef             string                            `json:"candidate_source_ref"`
		SourceLedgerCandidateRef       string                            `json:"source_ledger_candidate_ref"`
		EvidenceRootID                 string                            `json:"evidence_root_id"`
		EvidenceRootSource             string                            `json:"evidence_root_source"`
		RequiredObservations           []computerversion.ObservationKind `json:"required_observations"`
		RecipientBuildRequired         bool                              `json:"recipient_build_required"`
	}
	if err := json.Unmarshal(bridge.ManifestJSON, &manifest); err != nil {
		t.Fatalf("decode bridge manifest_json: %v\n%s", err, string(bridge.ManifestJSON))
	}
	if manifest.BridgeKind != bridge.Kind || manifest.CandidatePackageID != bridge.CandidatePackageID || manifest.CandidatePackageManifestSHA256 != bridge.CandidatePackageManifestSHA256 {
		t.Fatalf("embedded manifest package identity = kind:%q id:%q hash:%q, want bridge identity kind:%q id:%q hash:%q", manifest.BridgeKind, manifest.CandidatePackageID, manifest.CandidatePackageManifestSHA256, bridge.Kind, bridge.CandidatePackageID, bridge.CandidatePackageManifestSHA256)
	}
	if manifest.CandidateComputerVersion != fixture.version {
		t.Fatalf("embedded manifest version = %#v, want %#v", manifest.CandidateComputerVersion, fixture.version)
	}
	if manifest.SourceComputerID != "computer:source-command-test" || manifest.SourceCandidateID != "candidate:source-command-test" || manifest.CandidateSourceRef != "git:source-command-test" || manifest.SourceLedgerCandidateRef != "git:source-command-test" {
		t.Fatalf("embedded manifest lineage = computer:%q candidate:%q source:%q ledger:%q", manifest.SourceComputerID, manifest.SourceCandidateID, manifest.CandidateSourceRef, manifest.SourceLedgerCandidateRef)
	}
	if manifest.EvidenceRootID != fixture.evidence.ID || manifest.EvidenceRootSource != computerversion.EvidenceRootSourceLocalCandidate {
		t.Fatalf("embedded manifest evidence root = id:%q source:%q, want id:%q source:%q", manifest.EvidenceRootID, manifest.EvidenceRootSource, fixture.evidence.ID, computerversion.EvidenceRootSourceLocalCandidate)
	}
	if !manifest.RecipientBuildRequired {
		t.Fatalf("embedded manifest recipient_build_required = false, want true")
	}
	if !reflect.DeepEqual(manifest.RequiredObservations, wantRequired) {
		t.Fatalf("embedded manifest required observations = %#v, want %#v", manifest.RequiredObservations, wantRequired)
	}

	var contracts []struct {
		ContractID string `json:"contract_id"`
		Status     string `json:"status"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal(bridge.VerifierContractsJSON, &contracts); err != nil {
		t.Fatalf("decode bridge verifier_contracts_json: %v\n%s", err, string(bridge.VerifierContractsJSON))
	}
	if len(contracts) != 4 {
		t.Fatalf("verifier contracts len = %d, want 4 package contracts including direct-publish blocker: %#v", len(contracts), contracts)
	}
	var sawPublishBlocker bool
	for _, contract := range contracts {
		if contract.ContractID == "direct-publish-blocked-without-source-delta" && contract.Status == "blocked" && strings.Contains(contract.Summary, "source delta") {
			sawPublishBlocker = true
		}
	}
	if !sawPublishBlocker {
		t.Fatalf("verifier contracts = %#v, want blocked source-delta publish contract", contracts)
	}

	var provenance struct {
		CandidatePackageID             string   `json:"candidate_package_id"`
		CandidatePackageManifestSHA256 string   `json:"candidate_package_manifest_sha256"`
		EvidenceRefs                   []string `json:"evidence_refs"`
		EvidenceRootID                 string   `json:"evidence_root_id"`
		RealizationCount               int      `json:"realization_count"`
		ObservationCount               int      `json:"observation_count"`
	}
	if err := json.Unmarshal(bridge.ProvenanceRefsJSON, &provenance); err != nil {
		t.Fatalf("decode bridge provenance_refs_json: %v\n%s", err, string(bridge.ProvenanceRefsJSON))
	}
	if provenance.CandidatePackageID != bridge.CandidatePackageID || provenance.CandidatePackageManifestSHA256 != bridge.CandidatePackageManifestSHA256 {
		t.Fatalf("provenance package identity = id:%q hash:%q, want bridge id:%q hash:%q", provenance.CandidatePackageID, provenance.CandidatePackageManifestSHA256, bridge.CandidatePackageID, bridge.CandidatePackageManifestSHA256)
	}
	wantRefs := []string{rootOutputPath, realizationPath, "artifact:review-note"}
	if !reflect.DeepEqual(provenance.EvidenceRefs, wantRefs) {
		t.Fatalf("provenance evidence refs = %#v, want %#v", provenance.EvidenceRefs, wantRefs)
	}
	if provenance.EvidenceRootID != fixture.evidence.ID || provenance.RealizationCount != 1 || provenance.ObservationCount != len(fixture.observation.Observations) {
		t.Fatalf("provenance = root:%q realizations:%d observations:%d, want root:%q realizations:1 observations:%d", provenance.EvidenceRootID, provenance.RealizationCount, provenance.ObservationCount, fixture.evidence.ID, len(fixture.observation.Observations))
	}
}

func TestRunEmitsCandidatePackageProductPathAcceptanceContract(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, map[string]any{
		"base_fixture": map[string]any{"journal_path": filepath.Join(fixture.root, "base.sqlite")},
		"self_check":   map[string]any{"status": "equivalent"},
	})
	realizationPath := writeJSONFile(t, fixture.dir, "realization.json", fixture.realization)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-acceptance-command-test",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--source-computer-id", "computer:source-acceptance-command-test",
		"--source-candidate-id", "candidate:source-acceptance-command-test",
		"--candidate-source-ref", "git:source-acceptance-command-test",
		"--evidence-ref", "artifact:acceptance-review-note",
		"--output", "acceptance",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertBridgeOutputDoesNotPublish(t, stdout.Bytes())

	acceptance := decodeCandidatePackageProductPathAcceptance(t, stdout.Bytes())
	if acceptance.Kind != computerversion.CandidatePackageProductPathAcceptanceKind {
		t.Fatalf("acceptance kind = %q, want %q", acceptance.Kind, computerversion.CandidatePackageProductPathAcceptanceKind)
	}
	if acceptance.CandidatePackageID != "candidate-package-acceptance-command-test" {
		t.Fatalf("acceptance package id = %q, want candidate-package-acceptance-command-test", acceptance.CandidatePackageID)
	}
	if !strings.HasPrefix(acceptance.CandidatePackageManifestSHA256, "sha256:") || len(acceptance.CandidatePackageManifestSHA256) != len("sha256:")+64 {
		t.Fatalf("acceptance package hash = %q, want sha256 digest", acceptance.CandidatePackageManifestSHA256)
	}
	if acceptance.Version != fixture.version {
		t.Fatalf("acceptance version = %#v, want %#v", acceptance.Version, fixture.version)
	}
	if acceptance.IntakeBoundary != computerversion.CandidatePackageEvidenceOnlyIntakeBoundary {
		t.Fatalf("intake boundary = %q, want %q", acceptance.IntakeBoundary, computerversion.CandidatePackageEvidenceOnlyIntakeBoundary)
	}
	if !acceptance.OwnerReviewRequired {
		t.Fatalf("owner_review_required = false, want true")
	}
	if acceptance.AdoptionReady {
		t.Fatalf("adoption_ready = true, want false for evidence-only package intake")
	}
	wantBlockers := []string{
		"candidate_package_has_no_product_api_intake_record",
		"app_change_package_publish_requires_runtime_or_ui_source_delta",
		"owner_review_not_recorded",
		"adoption_rollback_boundary_not_bound",
	}
	if !reflect.DeepEqual(acceptance.AdoptionBlockers, wantBlockers) {
		t.Fatalf("adoption blockers = %#v, want %#v", acceptance.AdoptionBlockers, wantBlockers)
	}
	wantStatuses := map[string]string{
		"candidate-package-hash":            "passed",
		"candidate-evidence-non-production": "passed",
		"required-observations-present":     "passed",
		"evidence-only-intake-boundary":     "pending",
		"direct-app-change-publish":         "blocked",
		"adoption-and-rollback-boundary":    "blocked",
	}
	gotStatuses := make(map[string]string, len(acceptance.VerifierContracts))
	for _, contract := range acceptance.VerifierContracts {
		gotStatuses[contract.ContractID] = contract.Status
	}
	if !reflect.DeepEqual(gotStatuses, wantStatuses) {
		t.Fatalf("verifier contract statuses = %#v, want %#v", gotStatuses, wantStatuses)
	}
	wantRequired := []computerversion.ObservationKind{
		computerversion.ObservationBlobSet,
		computerversion.ObservationFileManifest,
		computerversion.ObservationPromotionCertificate,
		computerversion.ObservationVMStateManifest,
	}
	if !reflect.DeepEqual(acceptance.RequiredObservations, wantRequired) {
		t.Fatalf("required observations = %#v, want %#v", acceptance.RequiredObservations, wantRequired)
	}
	wantRefs := []string{rootOutputPath, realizationPath, "artifact:acceptance-review-note"}
	if !reflect.DeepEqual(acceptance.EvidenceRefs, wantRefs) {
		t.Fatalf("evidence refs = %#v, want %#v", acceptance.EvidenceRefs, wantRefs)
	}
}

func TestRunEmitsCandidatePackageIntakeRecord(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, map[string]any{
		"base_fixture": map[string]any{"journal_path": filepath.Join(fixture.root, "base.sqlite")},
		"self_check":   map[string]any{"status": "equivalent"},
	})
	realizationPath := writeJSONFile(t, fixture.dir, "realization.json", fixture.realization)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-intake-command-test",
		"--owner-id", "owner:intake-command-test",
		"--trace-id", "trace:intake-command-test",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--source-computer-id", "computer:source-intake-command-test",
		"--source-candidate-id", "candidate:source-intake-command-test",
		"--candidate-source-ref", "git:source-intake-command-test",
		"--evidence-ref", "artifact:intake-review-note",
		"--output", "intake",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	intake := decodeCandidatePackageIntake(t, stdout.Bytes())
	if intake.IntakeID != "candidate-package-intake-command-test" || intake.OwnerID != "owner:intake-command-test" {
		t.Fatalf("intake identity = id:%q owner:%q", intake.IntakeID, intake.OwnerID)
	}
	if intake.CandidatePackageID != "candidate-package-intake-command-test" {
		t.Fatalf("candidate package id = %q, want command id", intake.CandidatePackageID)
	}
	if !strings.HasPrefix(intake.CandidatePackageManifestSHA256, "sha256:") || len(intake.CandidatePackageManifestSHA256) != len("sha256:")+64 {
		t.Fatalf("intake package hash = %q, want sha256 digest", intake.CandidatePackageManifestSHA256)
	}
	if intake.SourceComputerID != "computer:source-intake-command-test" || intake.SourceCandidateID != "candidate:source-intake-command-test" || intake.CandidateSourceRef != "git:source-intake-command-test" {
		t.Fatalf("intake source refs = computer:%q candidate:%q source:%q", intake.SourceComputerID, intake.SourceCandidateID, intake.CandidateSourceRef)
	}
	if intake.IntakeBoundary != computerversion.CandidatePackageEvidenceOnlyIntakeBoundary {
		t.Fatalf("intake boundary = %q, want %q", intake.IntakeBoundary, computerversion.CandidatePackageEvidenceOnlyIntakeBoundary)
	}
	if intake.Status != types.CandidatePackageIntakeOwnerReviewPending || intake.OwnerReviewState != types.CandidatePackageOwnerReviewRequired || !intake.OwnerReviewRequired {
		t.Fatalf("review state = status:%q owner:%q required:%v", intake.Status, intake.OwnerReviewState, intake.OwnerReviewRequired)
	}
	if intake.AdoptionReady {
		t.Fatalf("adoption_ready = true, want false")
	}
	var blockers []string
	if err := json.Unmarshal(intake.AdoptionBlockersJSON, &blockers); err != nil {
		t.Fatalf("decode blockers: %v", err)
	}
	if !reflect.DeepEqual(blockers, []string{
		"candidate_package_has_no_product_api_intake_record",
		"app_change_package_publish_requires_runtime_or_ui_source_delta",
		"owner_review_not_recorded",
		"adoption_rollback_boundary_not_bound",
	}) {
		t.Fatalf("blockers = %#v", blockers)
	}
	var contracts []computerversion.CandidatePackageProductPathVerifierContract
	if err := json.Unmarshal(intake.VerifierContractsJSON, &contracts); err != nil {
		t.Fatalf("decode verifier contracts: %v", err)
	}
	if len(contracts) != 6 {
		t.Fatalf("verifier contract count = %d, want 6: %#v", len(contracts), contracts)
	}
	var refs []string
	if err := json.Unmarshal(intake.EvidenceRefsJSON, &refs); err != nil {
		t.Fatalf("decode evidence refs: %v", err)
	}
	if !reflect.DeepEqual(refs, []string{rootOutputPath, realizationPath, "artifact:intake-review-note"}) {
		t.Fatalf("evidence refs = %#v", refs)
	}
	var acceptance computerversion.CandidatePackageProductPathAcceptanceContract
	if err := json.Unmarshal(intake.AcceptanceJSON, &acceptance); err != nil {
		t.Fatalf("decode embedded acceptance: %v", err)
	}
	if acceptance.CandidatePackageID != intake.CandidatePackageID || acceptance.IntakeBoundary != intake.IntakeBoundary || acceptance.AdoptionReady {
		t.Fatalf("embedded acceptance = %+v, intake = %+v", acceptance, intake)
	}
	if intake.TraceID != "trace:intake-command-test" {
		t.Fatalf("trace id = %q, want trace:intake-command-test", intake.TraceID)
	}
}

func TestRunRejectsInvalidOutputBeforeSuccessJSON(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, nil)
	realizationPath := writeJSONFile(t, fixture.dir, "realization.json", fixture.realization)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-invalid-output",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
		"--output", "publish",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	assertNoCandidatePackageJSON(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), `--output must be "package", "bridge", "acceptance", or "intake"`) {
		t.Fatalf("stderr = %q, want invalid output error", stderr.String())
	}
}

func TestRunRejectsMissingEvidenceRootOutputBeforeJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-missing-root",
		"--realization", "not-read-before-flag-validation.json",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	assertNoCandidatePackageJSON(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), "--evidence-root-output is required") {
		t.Fatalf("stderr = %q, want missing evidence root error", stderr.String())
	}
}

func TestRunRejectsRealizationVersionMismatch(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, nil)
	mismatched := fixture.realization
	mismatched.Version = computerversion.ComputerVersion{CodeRef: "git:other", ArtifactProgramRef: fixture.version.ArtifactProgramRef}
	mismatched.Observations.Version = mismatched.Version
	realizationPath := writeJSONFile(t, fixture.dir, "mismatched-realization.json", mismatched)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-mismatched-realization",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run exit = 0, want nonzero; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	assertNoCandidatePackageJSON(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), "version does not match package version") {
		t.Fatalf("stderr = %q, want version mismatch error", stderr.String())
	}
}

func TestRunRejectsUnknownFieldsInsideRealizationJSON(t *testing.T) {
	fixture := newCandidatePackageFixture(t)
	rootOutputPath := writeEvidenceRootOutput(t, fixture, nil)
	realizationPath := writeRealizationWithUnknownCapabilityField(t, fixture)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--id", "candidate-package-unknown-realization-field",
		"--evidence-root-output", rootOutputPath,
		"--realization", realizationPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run exit = 0, want nonzero; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	assertNoCandidatePackageJSON(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), "unknown field") || !strings.Contains(stderr.String(), "deployed_route") {
		t.Fatalf("stderr = %q, want strict realization JSON unknown field error", stderr.String())
	}
}

type candidatePackageFixture struct {
	dir         string
	root        string
	version     computerversion.ComputerVersion
	evidence    computerversion.CandidateEvidenceRootManifest
	observation computerversion.ObservationSet
	realization computerversion.Realization
}

func newCandidatePackageFixture(t *testing.T) candidatePackageFixture {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "candidate-root")
	for _, path := range []string{
		root,
		filepath.Join(root, "blobs"),
		filepath.Join(root, "vm", "persist"),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("create fixture dir %s: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "base.sqlite"),
		filepath.Join(root, "vm", "data.img"),
		filepath.Join(root, "vm", "vmlinux"),
		filepath.Join(root, "vm", "rootfs.ext4"),
	} {
		if err := os.WriteFile(path, []byte("candidate package fixture"), 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", path, err)
		}
	}

	version := computerversion.ComputerVersion{
		CodeRef:            "git:candidatepackage-test",
		ArtifactProgramRef: "tape:org/candidatepackage-test",
	}
	evidence := computerversion.CandidateEvidenceRootManifest{
		ID:                    "candidate-root-command-test",
		RootPath:              root,
		Source:                computerversion.EvidenceRootSourceLocalCandidate,
		AuthorizedForSampling: true,
		Fixture: computerversion.ProductFixtureRoot{
			Version: version,
			Base: computerversion.BaseCurrentStatePaths{
				JournalPath: filepath.Join(root, "base.sqlite"),
				BlobRoot:    filepath.Join(root, "blobs"),
			},
			VM: computerversion.VMManagerScopedPath{
				VMID:            "vm-candidatepackage-test",
				PersistentDir:   filepath.Join(root, "vm", "persist"),
				DataImagePath:   filepath.Join(root, "vm", "data.img"),
				KernelImagePath: filepath.Join(root, "vm", "vmlinux"),
				RootfsPath:      filepath.Join(root, "vm", "rootfs.ext4"),
			},
			Promotion: computerversion.PromotionCertificate{
				ID:            "promotion-candidatepackage-test",
				RouteSlot:     "candidatepackage-test-slot",
				Active:        computerversion.ComputerVersion{CodeRef: "git:active", ArtifactProgramRef: "tape:active"},
				Candidate:     version,
				Base:          computerversion.ComputerVersion{CodeRef: "git:base", ArtifactProgramRef: "tape:base"},
				OwnerApproved: true,
				HealthWindow:  computerversion.PromotionHealthConfirmed,
				Ledgers: []computerversion.PromotionLedgerCertificate{{
					Name:  "candidatepackage-test-ledger",
					State: computerversion.PromotionLedgerVerified,
				}},
			},
		},
		EvidenceRefs: []string{"candidate-root-command-test:manifest", "candidate-root-command-test:observation"},
	}
	observation := computerversion.ObservationSet{
		Name:    "candidate-root-command-test",
		Version: version,
		Required: []computerversion.ObservationKind{
			computerversion.ObservationBlobSet,
			computerversion.ObservationFileManifest,
			computerversion.ObservationPromotionCertificate,
		},
		Observations: []computerversion.Observation{
			{Kind: computerversion.ObservationBlobSet, Key: "blob:fixture", Value: "sha256:blob-fixture"},
			{Kind: computerversion.ObservationFileManifest, Key: "base_item_fixture", Value: "sha256:file-fixture"},
			{Kind: computerversion.ObservationPromotionCertificate, Key: "promotion:promotion-candidatepackage-test", Value: "sha256:promotion-fixture"},
		},
	}
	realization := computerversion.Realization{
		ID:           "realization-candidatepackage-test",
		Version:      version,
		Capabilities: computerversion.VMManagerCapabilityManifest("fixture-vmmanager-materializer"),
		Observations: computerversion.ObservationSet{
			Name:     "realization-candidatepackage-test",
			Version:  version,
			Required: []computerversion.ObservationKind{computerversion.ObservationVMStateManifest},
			Observations: []computerversion.Observation{{
				Kind:  computerversion.ObservationVMStateManifest,
				Key:   "vmmanager:vm-candidatepackage-test",
				Value: `{"substrate":"firecracker/vmmanager","vm_id":"vm-candidatepackage-test","persistent_dir":"fixture","data_image_class":"durable_legacy_opaque","persistent_dir_class":"durable_legacy_opaque","boot_artifact_class":"code_artifact"}`,
			}},
		},
	}
	return candidatePackageFixture{dir: dir, root: root, version: version, evidence: evidence, observation: observation, realization: realization}
}

func writeEvidenceRootOutput(t *testing.T, fixture candidatePackageFixture, extra map[string]any) string {
	t.Helper()
	payload := map[string]any{
		"manifest":    fixture.evidence,
		"observation": fixture.observation,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return writeJSONFile(t, fixture.dir, "evidenceroot-output.json", payload)
}

func writeRealizationWithUnknownCapabilityField(t *testing.T, fixture candidatePackageFixture) string {
	t.Helper()
	data, err := json.Marshal(fixture.realization)
	if err != nil {
		t.Fatalf("marshal realization fixture: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode realization fixture for mutation: %v", err)
	}
	capabilities, ok := payload["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("realization capabilities = %#v, want object", payload["capabilities"])
	}
	capabilities["deployed_route"] = "should-be-rejected"
	return writeJSONFile(t, fixture.dir, "unknown-field-realization.json", payload)
}

func writeJSONFile(t *testing.T, dir, name string, value any) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func decodeCandidatePackage(t *testing.T, data []byte) computerversion.CandidateComputerPackageManifest {
	t.Helper()
	var manifest computerversion.CandidateComputerPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode candidate package JSON: %v\n%s", err, string(data))
	}
	return manifest
}

func decodeCandidatePackageAppChangeBridge(t *testing.T, data []byte) computerversion.CandidatePackageAppChangeBridgePayload {
	t.Helper()
	var bridge computerversion.CandidatePackageAppChangeBridgePayload
	if err := json.Unmarshal(data, &bridge); err != nil {
		t.Fatalf("decode candidate package app-change bridge JSON: %v\n%s", err, string(data))
	}
	return bridge
}

func decodeCandidatePackageProductPathAcceptance(t *testing.T, data []byte) computerversion.CandidatePackageProductPathAcceptanceContract {
	t.Helper()
	var acceptance computerversion.CandidatePackageProductPathAcceptanceContract
	if err := json.Unmarshal(data, &acceptance); err != nil {
		t.Fatalf("decode candidate package product-path acceptance JSON: %v\n%s", err, string(data))
	}
	return acceptance
}

func decodeCandidatePackageIntake(t *testing.T, data []byte) types.CandidatePackageIntakeRecord {
	t.Helper()
	var intake types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(data, &intake); err != nil {
		t.Fatalf("decode candidate package intake JSON: %v\n%s", err, string(data))
	}
	return intake
}
func assertCandidatePackageSuccess(t *testing.T, manifest computerversion.CandidateComputerPackageManifest, fixture candidatePackageFixture, rootOutputPath, realizationPath string) {
	t.Helper()
	if manifest.ID != "candidate-package-command-test" {
		t.Fatalf("manifest id = %q, want candidate-package-command-test", manifest.ID)
	}
	if manifest.Kind != computerversion.CandidateComputerPackageKind {
		t.Fatalf("manifest kind = %q, want %q", manifest.Kind, computerversion.CandidateComputerPackageKind)
	}
	if manifest.Version != fixture.version {
		t.Fatalf("manifest version = %#v, want %#v", manifest.Version, fixture.version)
	}
	if manifest.SourceComputerID != "computer:source-command-test" || manifest.SourceCandidateID != "candidate:source-command-test" || manifest.CandidateSourceRef != "git:source-command-test" {
		t.Fatalf("manifest lineage refs = computer:%q candidate:%q source:%q", manifest.SourceComputerID, manifest.SourceCandidateID, manifest.CandidateSourceRef)
	}
	if manifest.ContainsProduction || manifest.TouchesDeployedRoute || manifest.EvidenceRoot.ContainsProduction || manifest.EvidenceRoot.TouchesDeployedRoute {
		t.Fatalf("manifest widened into production/deployed flags: package production=%v route=%v evidence production=%v route=%v", manifest.ContainsProduction, manifest.TouchesDeployedRoute, manifest.EvidenceRoot.ContainsProduction, manifest.EvidenceRoot.TouchesDeployedRoute)
	}
	if !manifest.EvidenceRoot.AuthorizedForSampling || manifest.EvidenceRoot.Source != computerversion.EvidenceRootSourceLocalCandidate {
		t.Fatalf("evidence root admission = authorized:%v source:%q", manifest.EvidenceRoot.AuthorizedForSampling, manifest.EvidenceRoot.Source)
	}
	if !strings.HasPrefix(manifest.PackageManifestSHA256, "sha256:") || len(manifest.PackageManifestSHA256) != len("sha256:")+64 {
		t.Fatalf("package hash = %q, want sha256 digest", manifest.PackageManifestSHA256)
	}
	wantRequired := []computerversion.ObservationKind{
		computerversion.ObservationBlobSet,
		computerversion.ObservationFileManifest,
		computerversion.ObservationPromotionCertificate,
		computerversion.ObservationVMStateManifest,
	}
	if !reflect.DeepEqual(manifest.RequiredObservations, wantRequired) {
		t.Fatalf("required observations = %#v, want %#v", manifest.RequiredObservations, wantRequired)
	}
	if !reflect.DeepEqual(manifest.ReviewContracts, []computerversion.CandidatePackageReviewContract{
		{Name: "evidence-root-admitted", Status: "required", Evidence: "CandidateEvidenceRootManifest validates as non-production and route-inert"},
		{Name: "realization-scoped", Status: "required", Evidence: "Every realization declares capabilities and supports its required observations"},
		{Name: "package-hash", Status: "required", Evidence: "PackageManifestSHA256 hashes the canonical manifest with the hash field cleared"},
	}) {
		t.Fatalf("review contracts = %#v, want default package review contracts", manifest.ReviewContracts)
	}
	if len(manifest.Realizations) != 1 || manifest.Realizations[0].ID != fixture.realization.ID {
		t.Fatalf("realizations = %#v, want bundled realization %q", manifest.Realizations, fixture.realization.ID)
	}
	wantRefs := []string{rootOutputPath, realizationPath, "artifact:review-note"}
	if !reflect.DeepEqual(manifest.EvidenceRefs, wantRefs) {
		t.Fatalf("evidence refs = %#v, want canonical refs %#v", manifest.EvidenceRefs, wantRefs)
	}
}

func assertNoCandidatePackageJSON(t *testing.T, data []byte) {
	t.Helper()
	if len(bytes.TrimSpace(data)) != 0 {
		t.Fatalf("stdout = %s, want no candidate package JSON", string(data))
	}
}

func assertBridgeOutputDoesNotPublish(t *testing.T, data []byte) {
	t.Helper()
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode bridge output for publish flags: %v\n%s", err, string(data))
	}
	assertJSONKeysAbsent(t, payload, map[string]struct{}{
		"contains_production":    {},
		"touches_deployed_route": {},
		"deployed":               {},
		"deployed_route":         {},
		"publish":                {},
		"published":              {},
		"promote":                {},
		"promoted":               {},
		"production":             {},
	})
}

func assertJSONKeysAbsent(t *testing.T, value any, forbidden map[string]struct{}) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if _, ok := forbidden[key]; ok {
				t.Fatalf("bridge output includes deployment/promotion key %q in %#v", key, value)
			}
			assertJSONKeysAbsent(t, child, forbidden)
		}
	case []any:
		for _, child := range value {
			assertJSONKeysAbsent(t, child, forbidden)
		}
	}
}
