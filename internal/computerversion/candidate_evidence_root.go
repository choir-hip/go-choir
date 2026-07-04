package computerversion

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	EvidenceRootSourceLocalCandidate   = "local_candidate"
	EvidenceRootSourceStagingCandidate = "staging_candidate"
)

// CandidateEvidenceRootManifest is the admission contract for a root that may be
// sampled by ProductFixtureRoot. The manifest is deliberately stricter than the
// observer: it records sampling authority and path containment before any caller
// reads Base, vmmanager, or promotion evidence.
type CandidateEvidenceRootManifest struct {
	ID                    string             `json:"id"`
	RootPath              string             `json:"root_path"`
	Source                string             `json:"source"`
	AuthorizedForSampling bool               `json:"authorized_for_sampling"`
	ContainsProduction    bool               `json:"contains_production"`
	TouchesDeployedRoute  bool               `json:"touches_deployed_route"`
	Fixture               ProductFixtureRoot `json:"fixture"`
	EvidenceRefs          []string           `json:"evidence_refs,omitempty"`
}

// Validate rejects candidate evidence roots that are not explicitly authorized,
// are not declared candidate sources, contain production state, touch deployed
// routes, escape the declared root path, or cannot feed ProductFixtureRoot.
func (m CandidateEvidenceRootManifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("candidate evidence root: id is required")
	}
	root, err := cleanRequiredPath("candidate evidence root: root path", m.RootPath)
	if err != nil {
		return err
	}
	if !validEvidenceRootSource(m.Source) {
		return fmt.Errorf("candidate evidence root: unsupported source %q", m.Source)
	}
	if !m.AuthorizedForSampling {
		return fmt.Errorf("candidate evidence root: sampling authorization is required")
	}
	if m.ContainsProduction {
		return fmt.Errorf("candidate evidence root: production state is not admissible")
	}
	if m.TouchesDeployedRoute {
		return fmt.Errorf("candidate evidence root: deployed route mutation is not admissible")
	}
	if !m.Fixture.Version.Valid() {
		return fmt.Errorf("candidate evidence root: invalid fixture computer version")
	}
	if m.Fixture.Promotion.Candidate != m.Fixture.Version {
		return fmt.Errorf("candidate evidence root: promotion candidate does not match fixture version")
	}
	paths := []struct {
		label string
		path  string
	}{
		{label: "base journal", path: m.Fixture.Base.JournalPath},
		{label: "base blob root", path: m.Fixture.Base.BlobRoot},
		{label: "vm persistent dir", path: m.Fixture.VM.PersistentDir},
		{label: "vm data image", path: m.Fixture.VM.DataImagePath},
		{label: "vm kernel image", path: m.Fixture.VM.KernelImagePath},
		{label: "vm rootfs", path: m.Fixture.VM.RootfsPath},
		{label: "vm store disk", path: m.Fixture.VM.StoreDiskPath},
	}
	if m.Fixture.DoltHead != nil && strings.TrimSpace(m.Fixture.DoltHead.RepoRoot) != "" {
		paths = append(paths, struct {
			label string
			path  string
		}{label: "dolt repo root", path: m.Fixture.DoltHead.RepoRoot})
	}
	for _, item := range paths {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		if err := requirePathWithin(root, item.label, item.path); err != nil {
			return err
		}
	}
	if m.Fixture.Base.JournalPath == "" || m.Fixture.Base.BlobRoot == "" {
		return fmt.Errorf("candidate evidence root: base journal and blob root are required")
	}
	if m.Fixture.VM.PersistentDir == "" && m.Fixture.VM.DataImagePath == "" {
		return fmt.Errorf("candidate evidence root: vm persistent dir or data image is required")
	}
	if err := m.Fixture.Promotion.Validate(); err != nil {
		return fmt.Errorf("candidate evidence root: promotion certificate: %w", err)
	}
	if m.Fixture.ObjectGraph != nil {
		if err := m.Fixture.ObjectGraph.Validate(); err != nil {
			return fmt.Errorf("candidate evidence root: object graph: %w", err)
		}
	}
	if m.Fixture.DoltHead != nil {
		if err := m.Fixture.DoltHead.Validate(); err != nil {
			return fmt.Errorf("candidate evidence root: dolt head: %w", err)
		}
	}
	return nil
}

// ProductFixtureRoot returns the admitted fixture observer input. Call Validate
// before using the returned value to read any evidence roots.
func (m CandidateEvidenceRootManifest) ProductFixtureRoot() (ProductFixtureRoot, error) {
	if err := m.Validate(); err != nil {
		return ProductFixtureRoot{}, err
	}
	return m.Fixture, nil
}

func validEvidenceRootSource(source string) bool {
	switch strings.TrimSpace(source) {
	case EvidenceRootSourceLocalCandidate, EvidenceRootSourceStagingCandidate:
		return true
	default:
		return false
	}
}

func cleanRequiredPath(label, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return filepath.Clean(abs), nil
}

func requirePathWithin(root, label, path string) error {
	child, err := cleanRequiredPath("candidate evidence root: "+label, path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, child)
	if err != nil {
		return fmt.Errorf("candidate evidence root: %s path relation: %w", label, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("candidate evidence root: %s path escapes declared root", label)
	}
	return nil
}
