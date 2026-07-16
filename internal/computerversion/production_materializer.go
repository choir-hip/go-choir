package computerversion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

const (
	ProductionMaterializerName = "production-computer-constructor"
	SandboxRuntimeArtifactName = "sandbox-runtime.tar"
)

// ArtifactPayloadReader verifies immutable artifact bytes and reads payloads
// needed by the semantic generator. Implementations must bind reads to digest.
type ArtifactPayloadReader interface {
	ArtifactContentVerifier
	ReadArtifact(context.Context, string, string) ([]byte, error)
}

type ConstructionIdentity struct {
	RealizationID string `json:"realization_id"`
	ComputerKind  string `json:"computer_kind"`
	OwnerID       string `json:"owner_id,omitempty"`
	DesktopID     string `json:"desktop_id,omitempty"`
	WorkerID      string `json:"worker_id,omitempty"`
	CandidateID   string `json:"candidate_id,omitempty"`
}

type ConstructedLaunchRequest struct {
	Identity    ConstructionIdentity      `json:"identity"`
	Version     ComputerVersion           `json:"computer_version"`
	CodeClosure CodeClosure               `json:"code_closure"`
	Disk        diskinstantiation.Receipt `json:"disk_instantiation"`
}

type BootReceipt struct {
	VMID     string    `json:"vm_id"`
	HostURL  string    `json:"host_url"`
	Epoch    int64     `json:"epoch"`
	Healthy  bool      `json:"healthy"`
	BootedAt time.Time `json:"booted_at"`
}

type LiveConstructionObservation struct {
	State    ObservationSet                           `json:"state"`
	Geometry diskinstantiation.RuntimeGeometryReceipt `json:"geometry"`
}

// ConstructedLauncher is the product lifecycle boundary. Launch must attach the
// receipt's device without creating or mutating it. Observe must use the live
// product path, not the generator's staging directory or host-side device.
type ConstructedLauncher interface {
	Launch(context.Context, ConstructedLaunchRequest) (BootReceipt, error)
	Observe(context.Context, ConstructedLaunchRequest, BootReceipt) (LiveConstructionObservation, error)
	Commit(context.Context, BootReceipt, ComputerVersion, diskinstantiation.Receipt) error
	Destroy(context.Context, BootReceipt) error
}

type ResolvedConstructionInputs struct {
	Code    CodeClosure     `json:"code"`
	Program ArtifactProgram `json:"artifact_program"`
}

type ConstructionResult struct {
	Identity    ConstructionIdentity       `json:"identity"`
	Realization Realization                `json:"realization"`
	Inputs      ResolvedConstructionInputs `json:"inputs"`
	Disk        diskinstantiation.Receipt  `json:"disk_instantiation"`
	Boot        BootReceipt                `json:"boot"`
	Equivalence EquivalenceResult          `json:"equivalence"`
}

// ProductionMaterializer owns semantic resolution and generation. Disk owns
// only realization-local block mechanics; Launcher owns boot and live readback.
type ProductionMaterializer struct {
	Identity  ConstructionIdentity
	Inputs    ImmutableInputResolver
	Artifacts ArtifactPayloadReader
	Blobs     *blob.Store
	Disk      diskinstantiation.Backend
	DiskPlan  diskinstantiation.Plan
	Launcher  ConstructedLauncher
}

var _ Materializer = (*ProductionMaterializer)(nil)

func (m *ProductionMaterializer) Materialize(ctx context.Context, version ComputerVersion, manifest CapabilityManifest) (Realization, error) {
	result, err := m.Construct(ctx, version, manifest)
	if err != nil {
		return Realization{}, err
	}
	return result.Realization, nil
}

func (m *ProductionMaterializer) Construct(ctx context.Context, version ComputerVersion, manifest CapabilityManifest) (result ConstructionResult, err error) {
	if err := m.validate(version, manifest); err != nil {
		return ConstructionResult{}, err
	}
	code, err := m.Inputs.ResolveCode(ctx, version.CodeRef)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: resolve immutable code input: %w", err)
	}
	if code.Ref != version.CodeRef {
		return ConstructionResult{}, fmt.Errorf("production materializer: resolved code ref mismatch")
	}
	program, err := m.Inputs.ResolveArtifactProgram(ctx, version.ArtifactProgramRef)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: resolve immutable artifact program: %w", err)
	}
	if program.Ref != version.ArtifactProgramRef {
		return ConstructionResult{}, fmt.Errorf("production materializer: resolved artifact program ref mismatch")
	}
	if err := code.Verify(); err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: verify immutable CodeRef binding: %w", err)
	}
	resolved := ResolvedConstructionInputs{Code: code, Program: program}
	if len(resolved.Code.Artifacts) != 1 || resolved.Code.Artifacts[0].Name != SandboxRuntimeArtifactName {
		return ConstructionResult{}, fmt.Errorf("production materializer: CodeRef must resolve exactly one complete sandbox runtime artifact")
	}
	for _, artifact := range resolved.Code.Artifacts {
		if err := m.Artifacts.VerifyArtifact(ctx, artifact.URI, artifact.SHA256); err != nil {
			return ConstructionResult{}, fmt.Errorf("production materializer: verify code artifact %q: %w", artifact.Name, err)
		}
	}
	entries, err := loadVerifiedJournalEntries(ctx, m.Artifacts, resolved.Program, version)
	if err != nil {
		return ConstructionResult{}, err
	}
	for _, kind := range []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest} {
		if !manifest.Supports(kind) {
			return ConstructionResult{}, fmt.Errorf("production materializer: manifest lacks required capability %q", kind)
		}
	}

	plan := m.DiskPlan
	plan.RealizationID = strings.TrimSpace(m.Identity.RealizationID)
	var expected ObservationSet
	diskReceipt, err := m.Disk.Instantiate(ctx, plan, func(ctx context.Context, root string) error {
		filesRoot := filepath.Join(root, "files")
		if err := os.Mkdir(filesRoot, 0o755); err != nil {
			return err
		}
		if err := GenerateFromEvents(ctx, entries, m.Blobs, resolved.Program, version, filesRoot); err != nil {
			return err
		}
		expected, err = FilesystemProjectionObservationSet(ctx, "constructor-output", version, filesRoot)
		if err != nil {
			return err
		}
		return WriteConstructionStateManifest(root, version, expected)
	})
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: instantiate disk: %w", err)
	}
	keepDisk := false
	diskReclaimSafe := true
	defer func() {
		if !keepDisk && diskReclaimSafe {
			if _, cleanupErr := m.Disk.Reclaim(context.Background(), diskReceipt); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("production materializer: reclaim failed disk: %w", cleanupErr))
			}
		}
	}()
	if err := diskinstantiation.VerifyReceipt(plan, diskReceipt); err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: verify disk receipt: %w", err)
	}
	inspectedGeometry, err := m.Disk.Inspect(ctx, diskReceipt)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: independently inspect disk receipt: %w", err)
	}
	if inspectedGeometry != diskReceipt.Geometry {
		return ConstructionResult{}, fmt.Errorf("production materializer: disk receipt differs from independent inspection")
	}

	launchRequest := ConstructedLaunchRequest{
		Identity:    m.Identity,
		Version:     version,
		CodeClosure: resolved.Code,
		Disk:        diskReceipt,
	}
	boot, err := m.Launcher.Launch(ctx, launchRequest)
	if err != nil {
		if strings.TrimSpace(boot.VMID) != "" {
			diskReclaimSafe = false
		}
		return ConstructionResult{}, fmt.Errorf("production materializer: launch constructed computer: %w", err)
	}
	diskReclaimSafe = false
	keepBoot := false
	defer func() {
		if !keepBoot {
			if cleanupErr := m.Launcher.Destroy(context.Background(), boot); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("production materializer: destroy failed boot: %w", cleanupErr))
			} else {
				diskReclaimSafe = true
			}
		}
	}()
	if !boot.Healthy || strings.TrimSpace(boot.VMID) == "" || strings.TrimSpace(boot.HostURL) == "" {
		return ConstructionResult{}, fmt.Errorf("production materializer: launcher returned unhealthy or incomplete boot receipt")
	}

	live, err := m.Launcher.Observe(ctx, launchRequest, boot)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: live product readback: %w", err)
	}
	postBootGeometry, err := m.Disk.Inspect(ctx, diskReceipt)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: inspect post-boot allocation: %w", err)
	}
	diskReceipt, err = diskinstantiation.RefreshAllocatedGeometry(plan, diskReceipt, postBootGeometry)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: verify post-boot allocation: %w", err)
	}
	if err := diskinstantiation.VerifyRuntimeGeometry(plan, diskReceipt, live.Geometry); err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: live capacity readback: %w", err)
	}
	observed := live.State
	equivalence := (EquivalenceChecker{}).CheckObservationSets(expected, observed)
	if !equivalence.Equivalent() {
		return ConstructionResult{}, fmt.Errorf("production materializer: live readback is not equivalent: %+v", equivalence.Differences)
	}
	vmObservation, err := constructionVMObservationSet(version, diskReceipt, boot, live.Geometry)
	if err != nil {
		return ConstructionResult{}, err
	}
	combined, err := CombineObservationSets("production-construction", version, observed, vmObservation)
	if err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: combine construction evidence: %w", err)
	}
	if err := m.Launcher.Commit(ctx, boot, version, diskReceipt); err != nil {
		return ConstructionResult{}, fmt.Errorf("production materializer: commit lifecycle evidence: %w", err)
	}
	result = ConstructionResult{
		Identity: m.Identity,
		Realization: Realization{
			ID:           m.Identity.RealizationID,
			Version:      version,
			Capabilities: manifest,
			Observations: combined,
		},
		Inputs:      resolved,
		Disk:        diskReceipt,
		Boot:        boot,
		Equivalence: equivalence,
	}
	keepDisk = true
	keepBoot = true
	return result, nil
}

func (m *ProductionMaterializer) validate(version ComputerVersion, manifest CapabilityManifest) error {
	if !version.Valid() {
		return fmt.Errorf("production materializer: invalid ComputerVersion")
	}
	if m == nil || m.Inputs == nil || m.Artifacts == nil || m.Blobs == nil || m.Disk == nil || m.Launcher == nil {
		return fmt.Errorf("production materializer: inputs, artifacts, blobs, disk, and launcher are required")
	}
	if strings.TrimSpace(m.Identity.RealizationID) == "" {
		return fmt.Errorf("production materializer: realization identity is required")
	}
	if strings.TrimSpace(manifest.Materializer) != ProductionMaterializerName {
		return fmt.Errorf("production materializer: manifest must name %q", ProductionMaterializerName)
	}
	if strings.TrimSpace(manifest.Substrate) == "" {
		return fmt.Errorf("production materializer: substrate is required")
	}
	return nil
}

func loadVerifiedJournalEntries(ctx context.Context, artifacts ArtifactPayloadReader, program ArtifactProgram, version ComputerVersion) ([]journal.Entry, error) {
	var binding *ArtifactProgramEntry
	for i := range program.Entries {
		if program.Entries[i].Kind != ArtifactProgramKindBaseJournal {
			return nil, fmt.Errorf("production materializer: unsupported artifact program entry kind %q", program.Entries[i].Kind)
		}
		if binding != nil {
			return nil, fmt.Errorf("production materializer: multiple base journal artifacts")
		}
		binding = &program.Entries[i]
	}
	if binding == nil {
		return nil, fmt.Errorf("production materializer: base journal artifact is required")
	}
	payload, err := artifacts.ReadArtifact(ctx, binding.ArtifactURI, binding.ContentSHA256)
	if err != nil {
		return nil, fmt.Errorf("production materializer: read base journal artifact: %w", err)
	}
	var entries []journal.Entry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, fmt.Errorf("production materializer: decode base journal artifact: %w", err)
	}
	if err := VerifyJournalArtifactProgram(program, version, entries); err != nil {
		return nil, fmt.Errorf("production materializer: verify base journal binding: %w", err)
	}
	return entries, nil
}

func constructionVMObservationSet(version ComputerVersion, disk diskinstantiation.Receipt, boot BootReceipt, runtime diskinstantiation.RuntimeGeometryReceipt) (ObservationSet, error) {
	payload, err := json.Marshal(struct {
		Disk    diskinstantiation.Receipt                `json:"disk"`
		Boot    BootReceipt                              `json:"boot"`
		Runtime diskinstantiation.RuntimeGeometryReceipt `json:"runtime_geometry"`
	}{Disk: disk, Boot: boot, Runtime: runtime})
	if err != nil {
		return ObservationSet{}, fmt.Errorf("production materializer: encode VM observation: %w", err)
	}
	return ObservationSet{
		Name:     "production-vm",
		Version:  version,
		Required: []ObservationKind{ObservationVMStateManifest},
		Observations: []Observation{{
			Kind:  ObservationVMStateManifest,
			Key:   "vm:" + boot.VMID,
			Value: string(payload),
		}},
	}, nil
}
