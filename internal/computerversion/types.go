// Package computerversion defines the substrate-independent value vocabulary
// for Choir audited computers.
//
// The package is intentionally pure: it performs no filesystem, network,
// database, hypervisor, clock, or random operations. It gives higher layers a
// small contract for naming the durable computer version, declaring materializer
// capabilities, and carrying observation sets into equivalence checks.
package computerversion

import (
	"context"
	"strings"
)

// CodeRef identifies the Choir interpreter/runtime closure that computes a
// computer. Concrete refs may be git commits, Nix closures, SBOM refs, or a
// compound ref defined by a later resolver.
type CodeRef string

// Valid reports whether the code ref is present. The ref format is deliberately
// open at this layer because different materializers may resolve different
// concrete code-ref schemes.
func (r CodeRef) Valid() bool { return strings.TrimSpace(string(r)) != "" }

// ArtifactProgramRef identifies the typed, ordered, tamper-evident program/tape
// that represents durable user/product state.
type ArtifactProgramRef string

// Valid reports whether the artifact-program ref is present.
func (r ArtifactProgramRef) Valid() bool { return strings.TrimSpace(string(r)) != "" }

// ComputerVersion is the substrate-independent identity of a Choir computer.
// It names the code that computes the computer and the artifact program that
// carries durable state.
type ComputerVersion struct {
	CodeRef            CodeRef            `json:"code_ref"`
	ArtifactProgramRef ArtifactProgramRef `json:"artifact_program_ref"`
}

// Valid reports whether both sides of the computer identity are present.
func (v ComputerVersion) Valid() bool {
	return v.CodeRef.Valid() && v.ArtifactProgramRef.Valid()
}

// ObservationKind identifies a class of user-observable state that can be
// compared under a declared equivalence scope.
type ObservationKind string

const (
	ObservationFileManifest          ObservationKind = "file_manifest"
	ObservationBlobSet               ObservationKind = "blob_set"
	ObservationDoltHead              ObservationKind = "dolt_head"
	ObservationObjectGraphHead       ObservationKind = "object_graph_head"
	ObservationProvenanceAnswer      ObservationKind = "provenance_answer"
	ObservationLiveProcessContinuity ObservationKind = "live_process_continuity"
	ObservationVMStateManifest       ObservationKind = "vm_state_manifest"
	ObservationPromotionCertificate  ObservationKind = "promotion_certificate"
)

// Valid reports whether the kind is one of the substrate-independent observation
// classes currently understood by this package.
func (k ObservationKind) Valid() bool {
	switch k {
	case ObservationFileManifest,
		ObservationBlobSet,
		ObservationDoltHead,
		ObservationObjectGraphHead,
		ObservationProvenanceAnswer,
		ObservationLiveProcessContinuity,
		ObservationVMStateManifest,
		ObservationPromotionCertificate:
		return true
	default:
		return false
	}
}

// ObservationSource names an instrument that can produce observations for a
// materializer. Sources such as ebpf-* are observation inputs only: they do not
// become artifact-program state or equivalence proof by themselves.
type ObservationSource struct {
	Name                 string `json:"name"`
	Kind                 string `json:"kind"`
	KernelVersionFloor   string `json:"kernel_version_floor,omitempty"`
	PrivilegeScope       string `json:"privilege_scope,omitempty"`
	RequiresPIIRedaction bool   `json:"requires_pii_redaction,omitempty"`
}

// Valid reports whether the source has enough identity to be auditable.
func (s ObservationSource) Valid() bool {
	return strings.TrimSpace(s.Name) != "" && strings.TrimSpace(s.Kind) != ""
}

// UnsupportedCapability records an observation class a materializer cannot
// claim. A checker must narrow or fail claims that require unsupported classes.
type UnsupportedCapability struct {
	Kind   ObservationKind `json:"kind"`
	Reason string          `json:"reason"`
}

// CapabilityManifest declares what a materializer can observe or realize. It is
// a claim boundary: unsupported classes narrow the equivalence scope.
type CapabilityManifest struct {
	Materializer       string                  `json:"materializer"`
	Substrate          string                  `json:"substrate"`
	Supported          []ObservationKind       `json:"supported"`
	Unsupported        []UnsupportedCapability `json:"unsupported,omitempty"`
	ObservationSources []ObservationSource     `json:"observation_sources,omitempty"`
}

// Supports reports whether this manifest declares support for kind and does not
// explicitly list it as unsupported.
func (m CapabilityManifest) Supports(kind ObservationKind) bool {
	if !kind.Valid() {
		return false
	}
	for _, unsupported := range m.Unsupported {
		if unsupported.Kind == kind {
			return false
		}
	}
	for _, supported := range m.Supported {
		if supported == kind {
			return true
		}
	}
	return false
}

// MissingRequired returns every required observation kind this manifest cannot
// support. The returned slice is empty only when every required class is in
// scope for this materializer.
func (m CapabilityManifest) MissingRequired(required []ObservationKind) []UnsupportedCapability {
	seen := make(map[ObservationKind]struct{}, len(required))
	missing := make([]UnsupportedCapability, 0)
	for _, kind := range required {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		if m.Supports(kind) {
			continue
		}
		missing = append(missing, UnsupportedCapability{Kind: kind, Reason: "capability not declared by materializer"})
	}
	return missing
}

// Observation is one observed fact inside an ObservationSet. Key is scoped by
// Kind; Value is a canonical comparable string such as a content hash, manifest
// root, Dolt head, or provenance answer hash.
type Observation struct {
	Kind  ObservationKind `json:"kind"`
	Key   string          `json:"key"`
	Value string          `json:"value"`
}

// Valid reports whether the observation has a known kind and stable key. Empty
// values are allowed so tombstones or explicit absence can be represented.
func (o Observation) Valid() bool {
	return o.Kind.Valid() && strings.TrimSpace(o.Key) != ""
}

// FileManifestObservation records one file-manifest entry under the current
// first durable-slice proof. The value should be a stable content/metadata hash
// or blob reference chosen by the caller's observation schema.
func FileManifestObservation(path, value string) Observation {
	return Observation{Kind: ObservationFileManifest, Key: path, Value: value}
}

// ObservationSet is the declared observation evidence for one ComputerVersion.
// Required names the observation classes that must be supported before the set
// can be used to claim equivalence.
type ObservationSet struct {
	Name         string            `json:"name"`
	Version      ComputerVersion   `json:"version"`
	Required     []ObservationKind `json:"required"`
	Observations []Observation     `json:"observations"`
}

// RequiredKinds returns the declared required kinds, plus any kinds present in
// observations. Including observed kinds prevents an observation from being
// compared without a capability claim.
func (s ObservationSet) RequiredKinds() []ObservationKind {
	seen := make(map[ObservationKind]struct{}, len(s.Required)+len(s.Observations))
	out := make([]ObservationKind, 0, len(s.Required)+len(s.Observations))
	for _, kind := range s.Required {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	for _, observation := range s.Observations {
		if _, ok := seen[observation.Kind]; ok {
			continue
		}
		seen[observation.Kind] = struct{}{}
		out = append(out, observation.Kind)
	}
	return out
}

// ExtractRequest names the typed artifact program cursor to extract into an
// observation set. The ComputerVersion carries the ArtifactProgramRef; Name is
// only an evidence label for the produced set.
type ExtractRequest struct {
	Name    string          `json:"name"`
	Version ComputerVersion `json:"version"`
}

// Extractor converts typed artifact-program state into an ObservationSet. It is
// the boundary before materialization: extraction must prove what durable state
// is available without relying on opaque substrate images.
type Extractor interface {
	Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error)
}

// Realization is a substrate-specific projection of a ComputerVersion plus the
// observations and capability manifest needed to scope equivalence claims.
type Realization struct {
	ID           string             `json:"id"`
	Version      ComputerVersion    `json:"version"`
	Capabilities CapabilityManifest `json:"capabilities"`
	Observations ObservationSet     `json:"observations"`
}

// Materializer projects a ComputerVersion into a substrate-specific realization
// with declared capabilities. Implementations may launch VMs, produce file
// projections, or build fixture realizations, but all claims flow through the
// returned Realization.
type Materializer interface {
	Materialize(ctx context.Context, version ComputerVersion, manifest CapabilityManifest) (Realization, error)
}
