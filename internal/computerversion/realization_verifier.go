package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

const IndependentRealizationVerifierName = "independent-production-realization-verifier"

type RealizationVerificationReceipt struct {
	ID                 string                                   `json:"verification_receipt_id"`
	Verifier           string                                   `json:"verifier"`
	Version            ComputerVersion                          `json:"computer_version"`
	Identity           ConstructionIdentity                     `json:"identity"`
	DiskPlan           diskinstantiation.Plan                   `json:"disk_plan"`
	Disk               diskinstantiation.Receipt                `json:"disk_receipt"`
	RealizationID      string                                   `json:"realization_id"`
	ConstructionSHA256 string                                   `json:"construction_sha256"`
	ObservationSHA256  string                                   `json:"observation_sha256"`
	DiskReceiptID      string                                   `json:"disk_receipt_id"`
	RuntimeGeometry    diskinstantiation.RuntimeGeometryReceipt `json:"runtime_geometry"`
	VMID               string                                   `json:"vm_id"`
	Epoch              int64                                    `json:"epoch"`
	VerifiedAt         time.Time                                `json:"verified_at"`
}

func (r RealizationVerificationReceipt) Validate() error {
	if !r.Version.Valid() || strings.TrimSpace(r.RealizationID) == "" || strings.TrimSpace(r.VMID) == "" || r.RealizationID != r.VMID || r.Epoch <= 0 || r.VerifiedAt.IsZero() {
		return fmt.Errorf("realization verifier: receipt identity is incomplete")
	}
	if r.Verifier != IndependentRealizationVerifierName || r.DiskReceiptID == "" || !validDigest(r.ConstructionSHA256) || !validDigest(r.ObservationSHA256) {
		return fmt.Errorf("realization verifier: receipt evidence is incomplete")
	}
	if r.RuntimeGeometry.FilesystemBytes == 0 || r.RuntimeGeometry.FilesystemBlockSize == 0 || r.RuntimeGeometry.AvailableBytes > r.RuntimeGeometry.FilesystemBytes {
		return fmt.Errorf("realization verifier: runtime geometry is invalid")
	}
	if r.Identity.RealizationID != r.RealizationID || r.Identity.ComputerKind != "candidate" || r.Identity.OwnerID == "" || r.Identity.DesktopID == "" || r.Identity.CandidateID != r.Identity.DesktopID {
		return fmt.Errorf("realization verifier: candidate ownership identity is invalid")
	}
	if r.DiskPlan.LogicalBytes != 32<<30 || r.DiskPlan.Allocation.MaxAllocatedBytes != 2<<30 || r.DiskPlan.Allocation.MinimumAvailableBytes != 2<<30 {
		return fmt.Errorf("realization verifier: production disk policy mismatch")
	}
	if err := diskinstantiation.VerifyReceipt(r.DiskPlan, r.Disk); err != nil || r.Disk.ID != r.DiskReceiptID {
		return fmt.Errorf("realization verifier: disk evidence is invalid: %w", err)
	}
	if err := diskinstantiation.VerifyRuntimeGeometry(r.DiskPlan, r.Disk, r.RuntimeGeometry); err != nil {
		return fmt.Errorf("realization verifier: runtime geometry evidence: %w", err)
	}
	payload, err := verificationReceiptPayload(r)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	if r.ID != "verification:sha256:"+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("realization verifier: receipt hash mismatch")
	}
	return nil
}

type IndependentRealizationVerifier struct {
	Inputs    ImmutableInputResolver
	Artifacts ArtifactPayloadReader
	Blobs     *blob.Store
	Disk      diskinstantiation.Backend
	Launcher  ConstructedLauncher
	Now       func() time.Time
}

func (v IndependentRealizationVerifier) Verify(ctx context.Context, plan diskinstantiation.Plan, construction ConstructionResult) (RealizationVerificationReceipt, error) {
	if err := ctx.Err(); err != nil {
		return RealizationVerificationReceipt{}, err
	}
	version := construction.Realization.Version
	if !version.Valid() || construction.Identity.RealizationID != construction.Realization.ID || construction.Identity.ComputerKind != "candidate" || construction.Identity.OwnerID == "" || construction.Identity.DesktopID == "" || construction.Identity.CandidateID != construction.Identity.DesktopID || construction.Boot.VMID != construction.Realization.ID || !construction.Boot.Healthy || construction.Boot.Epoch <= 0 || !construction.Equivalence.Equivalent() {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: construction result is incomplete")
	}
	if v.Inputs == nil || v.Artifacts == nil || v.Blobs == nil || v.Disk == nil || v.Launcher == nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independent inputs, artifacts, blobs, disk, and launcher are required")
	}
	code, err := v.Inputs.ResolveCode(ctx, version.CodeRef)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: resolve CodeRef: %w", err)
	}
	program, err := v.Inputs.ResolveArtifactProgram(ctx, version.ArtifactProgramRef)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: resolve ArtifactProgramRef: %w", err)
	}
	if err := code.Verify(); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: CodeRef binding: %w", err)
	}
	if err := program.Verify(); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: ArtifactProgramRef binding: %w", err)
	}
	if !reflect.DeepEqual(code, construction.Inputs.Code) || !reflect.DeepEqual(program, construction.Inputs.Program) {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independently resolved inputs differ from construction")
	}
	for _, artifact := range code.Artifacts {
		if err := v.Artifacts.VerifyArtifact(ctx, artifact.URI, artifact.SHA256); err != nil {
			return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: code artifact: %w", err)
		}
	}
	plan.RealizationID = construction.Realization.ID
	if plan.LogicalBytes != 32<<30 || plan.Allocation.MaxAllocatedBytes != 2<<30 || plan.Allocation.MinimumAvailableBytes != 2<<30 {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: production disk policy mismatch")
	}
	if err := diskinstantiation.VerifyReceipt(plan, construction.Disk); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: disk receipt: %w", err)
	}
	geometry, err := v.Disk.Inspect(ctx, construction.Disk)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independently inspect disk: %w", err)
	}
	if _, err := diskinstantiation.RefreshAllocatedGeometry(plan, construction.Disk, geometry); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independent disk geometry: %w", err)
	}
	launchRequest := ConstructedLaunchRequest{Identity: construction.Identity, Version: version, CodeClosure: code, Disk: construction.Disk}
	live, err := v.Launcher.Observe(ctx, launchRequest, construction.Boot)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: live product readback: %w", err)
	}
	if err := diskinstantiation.VerifyRuntimeGeometry(plan, construction.Disk, live.Geometry); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: runtime geometry: %w", err)
	}
	entries, err := loadVerifiedJournalEntries(ctx, v.Artifacts, program, version)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independently load artifact program: %w", err)
	}
	verificationRoot, err := os.MkdirTemp("", "choir-realization-verifier-")
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: create scratch realization: %w", err)
	}
	defer os.RemoveAll(verificationRoot)
	filesRoot := filepath.Join(verificationRoot, "files")
	if err := os.Mkdir(filesRoot, 0o700); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: create scratch files root: %w", err)
	}
	if err := GenerateFromEvents(ctx, entries, v.Blobs, program, version, filesRoot); err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independently regenerate semantic state: %w", err)
	}
	expected, err := FilesystemProjectionObservationSet(ctx, "independent-regeneration", version, filesRoot)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: inspect regenerated semantic state: %w", err)
	}
	if result := (EquivalenceChecker{}).CheckObservationSets(expected, live.State); !result.Equivalent() {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: independently observed state differs from immutable program: %+v", result.Differences)
	}
	if result := (EquivalenceChecker{}).CheckObservationSets(expected, semanticConstructionObservations(construction.Realization.Observations)); !result.Equivalent() {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: construction result differs from immutable program: %+v", result.Differences)
	}
	vmObservation, err := constructionVMObservationSet(version, construction.Disk, construction.Boot, live.Geometry)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: reconstruct VM observation: %w", err)
	}
	joined, err := CombineObservationSets("independent-construction-join", version, expected, vmObservation)
	if err != nil {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: join independent construction evidence: %w", err)
	}
	if result := (EquivalenceChecker{}).CheckObservationSets(joined, construction.Realization.Observations); !result.Equivalent() {
		return RealizationVerificationReceipt{}, fmt.Errorf("realization verifier: VM/device construction join differs: %+v", result.Differences)
	}
	constructionHash, err := ConstructionResultSHA256(construction)
	if err != nil {
		return RealizationVerificationReceipt{}, err
	}
	observationBytes, err := json.Marshal(live)
	if err != nil {
		return RealizationVerificationReceipt{}, err
	}
	observationDigest := sha256.Sum256(observationBytes)
	now := v.Now
	if now == nil {
		now = time.Now
	}
	receipt := RealizationVerificationReceipt{
		Verifier: IndependentRealizationVerifierName, Version: version, Identity: construction.Identity, DiskPlan: plan, Disk: construction.Disk, RealizationID: construction.Realization.ID,
		ConstructionSHA256: constructionHash, ObservationSHA256: hex.EncodeToString(observationDigest[:]),
		DiskReceiptID: construction.Disk.ID, RuntimeGeometry: live.Geometry, VMID: construction.Boot.VMID, Epoch: construction.Boot.Epoch, VerifiedAt: now().UTC(),
	}
	payload, err := verificationReceiptPayload(receipt)
	if err != nil {
		return RealizationVerificationReceipt{}, err
	}
	digest := sha256.Sum256(payload)
	receipt.ID = "verification:sha256:" + hex.EncodeToString(digest[:])
	return receipt, receipt.Validate()
}

func ConstructionResultSHA256(construction ConstructionResult) (string, error) {
	payload, err := json.Marshal(construction)
	if err != nil {
		return "", fmt.Errorf("realization verifier: encode construction result: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func semanticConstructionObservations(source ObservationSet) ObservationSet {
	out := ObservationSet{Name: "independent-semantic-expectation", Version: source.Version}
	for _, kind := range source.RequiredKinds() {
		if kind != ObservationVMStateManifest {
			out.Required = append(out.Required, kind)
		}
	}
	for _, observation := range source.Observations {
		if observation.Kind != ObservationVMStateManifest {
			out.Observations = append(out.Observations, observation)
		}
	}
	return out
}

func verificationReceiptPayload(r RealizationVerificationReceipt) ([]byte, error) {
	r.ID = ""
	return json.Marshal(r)
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
