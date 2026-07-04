package computerversion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	VMManagerSubstrateFirecracker = "firecracker/vmmanager"

	StateClassDurableLegacyOpaque = "durable_legacy_opaque"
	StateClassCodeArtifact        = "code_artifact"
	StateClassCache               = "cache"
	StateClassEphemeral           = "ephemeral"
	StateClassUnknown             = "unknown"
)

// VMManagerScopedPath names the narrow, non-lifecycle Firecracker/vmmanager
// path currently available to this package: it can classify the host-side VM
// state inputs that an existing vmmanager launch would use, but it does not
// launch, stop, resume, copy, or mutate a VM.
type VMManagerScopedPath struct {
	VMID               string `json:"vm_id"`
	PersistentDir      string `json:"persistent_dir,omitempty"`
	DataImagePath      string `json:"data_image_path,omitempty"`
	KernelImagePath    string `json:"kernel_image_path,omitempty"`
	RootfsPath         string `json:"rootfs_path,omitempty"`
	StoreDiskPath      string `json:"store_disk_path,omitempty"`
	ComputerKind       string `json:"computer_kind,omitempty"`
	OwnerID            string `json:"owner_id,omitempty"`
	DesktopID          string `json:"desktop_id,omitempty"`
	WorkerID           string `json:"worker_id,omitempty"`
	CandidateID        string `json:"candidate_id,omitempty"`
	Epoch              int64  `json:"epoch,omitempty"`
	DataImageClass     string `json:"data_image_class"`
	PersistentDirClass string `json:"persistent_dir_class"`
	BootArtifactClass  string `json:"boot_artifact_class"`
}

// Normalize fills the explicit state classes that bound what this scoped
// materializer may claim. data.img remains legacy opaque durable state: the
// manifest can classify it, but cannot turn it into typed user-state evidence.
func (p VMManagerScopedPath) Normalize() VMManagerScopedPath {
	p.VMID = strings.TrimSpace(p.VMID)
	p.PersistentDir = strings.TrimSpace(p.PersistentDir)
	p.DataImagePath = strings.TrimSpace(p.DataImagePath)
	p.KernelImagePath = strings.TrimSpace(p.KernelImagePath)
	p.RootfsPath = strings.TrimSpace(p.RootfsPath)
	p.StoreDiskPath = strings.TrimSpace(p.StoreDiskPath)
	p.ComputerKind = strings.TrimSpace(p.ComputerKind)
	p.OwnerID = strings.TrimSpace(p.OwnerID)
	p.DesktopID = strings.TrimSpace(p.DesktopID)
	p.WorkerID = strings.TrimSpace(p.WorkerID)
	p.CandidateID = strings.TrimSpace(p.CandidateID)
	p.DataImageClass = normalizeStateClass(p.DataImageClass, StateClassDurableLegacyOpaque)
	p.PersistentDirClass = normalizeStateClass(p.PersistentDirClass, StateClassDurableLegacyOpaque)
	p.BootArtifactClass = normalizeStateClass(p.BootArtifactClass, StateClassCodeArtifact)
	return p
}

// ObservationSet returns a single substrate-scoped VM-state manifest observation
// for version. This is deliberately weaker than a durable-state observation: it
// records the vmmanager boundary and legacy opaque state classes, not user-state
// equivalence.
func (p VMManagerScopedPath) ObservationSet(name string, version ComputerVersion) (ObservationSet, error) {
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("vmmanager scoped path: invalid computer version")
	}
	p = p.Normalize()
	if p.VMID == "" {
		return ObservationSet{}, fmt.Errorf("vmmanager scoped path: vm id is required")
	}
	if p.PersistentDir == "" && p.DataImagePath == "" {
		return ObservationSet{}, fmt.Errorf("vmmanager scoped path: persistent dir or data image path is required")
	}
	value, err := canonicalVMManagerManifest(p)
	if err != nil {
		return ObservationSet{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = "vmmanager-scoped-path"
	}
	return ObservationSet{
		Name:     name,
		Version:  version,
		Required: []ObservationKind{ObservationVMStateManifest},
		Observations: []Observation{{
			Kind:  ObservationVMStateManifest,
			Key:   "vmmanager:" + p.VMID,
			Value: value,
		}},
	}, nil
}

// VMManagerCapabilityManifest declares the only capability this scoped boundary
// can honestly support today. Durable file/blob/objectgraph/provenance claims
// must still be proven by typed artifact-program observations, not by vmmanager
// launch metadata or data.img presence.
func VMManagerCapabilityManifest(materializer string) CapabilityManifest {
	materializer = strings.TrimSpace(materializer)
	if materializer == "" {
		materializer = "firecracker-vmmanager-scoped"
	}
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    VMManagerSubstrateFirecracker,
		Supported:    []ObservationKind{ObservationVMStateManifest},
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "vmmanager launch state does not prove typed file-manifest state"},
			{Kind: ObservationBlobSet, Reason: "vmmanager launch state does not prove content-addressed blob integrity"},
			{Kind: ObservationDoltHead, Reason: "vmmanager launch state does not prove Dolt ledger head equivalence"},
			{Kind: ObservationObjectGraphHead, Reason: "vmmanager launch state does not prove object graph head equivalence"},
			{Kind: ObservationProvenanceAnswer, Reason: "vmmanager launch state does not answer provenance queries"},
			{Kind: ObservationLiveProcessContinuity, Reason: "vmmanager scoped manifest does not prove live-process continuity"},
		},
	}
}

// VMManagerScopedMaterializer wraps an explicit vmmanager state classification
// as a Materializer without invoking the vmmanager lifecycle surface.
type VMManagerScopedMaterializer struct {
	ID    string
	State VMManagerScopedPath
}

var _ Materializer = VMManagerScopedMaterializer{}

func (m VMManagerScopedMaterializer) Materialize(ctx context.Context, version ComputerVersion, manifest CapabilityManifest) (Realization, error) {
	if err := ctx.Err(); err != nil {
		return Realization{}, err
	}
	if strings.TrimSpace(manifest.Materializer) == "" {
		return Realization{}, fmt.Errorf("vmmanager scoped materializer: manifest must name materializer")
	}
	if strings.TrimSpace(manifest.Substrate) != VMManagerSubstrateFirecracker {
		return Realization{}, fmt.Errorf("vmmanager scoped materializer: manifest substrate %q does not match %q", manifest.Substrate, VMManagerSubstrateFirecracker)
	}
	observations, err := m.State.ObservationSet(m.ID, version)
	if err != nil {
		return Realization{}, err
	}
	if missing := manifest.MissingRequired(observations.RequiredKinds()); len(missing) > 0 {
		return Realization{}, fmt.Errorf("vmmanager scoped materializer: manifest %q lacks required capability %q", manifest.Materializer, missing[0].Kind)
	}
	id := strings.TrimSpace(m.ID)
	if id == "" {
		id = manifest.Materializer
	}
	return Realization{
		ID:           id,
		Version:      version,
		Capabilities: manifest,
		Observations: observations,
	}, nil
}

func normalizeStateClass(value, fallback string) string {
	value = strings.TrimSpace(value)
	switch value {
	case StateClassDurableLegacyOpaque, StateClassCodeArtifact, StateClassCache, StateClassEphemeral, StateClassUnknown:
		return value
	case "":
		return fallback
	default:
		return StateClassUnknown
	}
}

func canonicalVMManagerManifest(p VMManagerScopedPath) (string, error) {
	payload := struct {
		Substrate          string `json:"substrate"`
		VMID               string `json:"vm_id"`
		PersistentDir      string `json:"persistent_dir,omitempty"`
		DataImagePath      string `json:"data_image_path,omitempty"`
		KernelImagePath    string `json:"kernel_image_path,omitempty"`
		RootfsPath         string `json:"rootfs_path,omitempty"`
		StoreDiskPath      string `json:"store_disk_path,omitempty"`
		ComputerKind       string `json:"computer_kind,omitempty"`
		OwnerID            string `json:"owner_id,omitempty"`
		DesktopID          string `json:"desktop_id,omitempty"`
		WorkerID           string `json:"worker_id,omitempty"`
		CandidateID        string `json:"candidate_id,omitempty"`
		Epoch              int64  `json:"epoch,omitempty"`
		DataImageClass     string `json:"data_image_class"`
		PersistentDirClass string `json:"persistent_dir_class"`
		BootArtifactClass  string `json:"boot_artifact_class"`
	}{
		Substrate:          VMManagerSubstrateFirecracker,
		VMID:               p.VMID,
		PersistentDir:      p.PersistentDir,
		DataImagePath:      p.DataImagePath,
		KernelImagePath:    p.KernelImagePath,
		RootfsPath:         p.RootfsPath,
		StoreDiskPath:      p.StoreDiskPath,
		ComputerKind:       p.ComputerKind,
		OwnerID:            p.OwnerID,
		DesktopID:          p.DesktopID,
		WorkerID:           p.WorkerID,
		CandidateID:        p.CandidateID,
		Epoch:              p.Epoch,
		DataImageClass:     p.DataImageClass,
		PersistentDirClass: p.PersistentDirClass,
		BootArtifactClass:  p.BootArtifactClass,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("vmmanager scoped path: encode manifest: %w", err)
	}
	return string(data), nil
}
