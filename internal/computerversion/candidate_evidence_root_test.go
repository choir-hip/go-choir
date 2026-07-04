package computerversion

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCandidateEvidenceRootManifestProductFixtureRootAdmitsAuthorizedLocalCandidate(t *testing.T) {
	version := candidateEvidenceRootComputerVersion()
	manifest := candidateEvidenceRootManifest(t, version)

	if err := manifest.Validate(); err != nil {
		t.Fatalf("validate authorized local candidate: %v", err)
	}
	fixture, err := manifest.ProductFixtureRoot()
	if err != nil {
		t.Fatalf("product fixture root: %v", err)
	}
	if fixture.Version != version {
		t.Fatalf("fixture version = %#v, want %#v", fixture.Version, version)
	}
	if !reflect.DeepEqual(fixture, manifest.Fixture) {
		t.Fatalf("product fixture root = %#v, want manifest fixture %#v", fixture, manifest.Fixture)
	}
}

func TestCandidateEvidenceRootManifestRejectsMissingAdmissionFieldsAndRuntimeRoots(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateEvidenceRootManifest)
		wantErr string
	}{
		{
			name: "missing authorization",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.AuthorizedForSampling = false
			},
			wantErr: "sampling authorization is required",
		},
		{
			name: "production state flag",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.ContainsProduction = true
			},
			wantErr: "production state is not admissible",
		},
		{
			name: "deployed route flag",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.TouchesDeployedRoute = true
			},
			wantErr: "deployed route mutation is not admissible",
		},
		{
			name: "unsupported source",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.Source = "production_snapshot"
			},
			wantErr: "unsupported source \"production_snapshot\"",
		},
		{
			name: "missing id",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.ID = " \t"
			},
			wantErr: "id is required",
		},
		{
			name: "missing root",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.RootPath = " \t"
			},
			wantErr: "root path is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := candidateEvidenceRootManifest(t, candidateEvidenceRootComputerVersion())
			tc.mutate(&manifest)

			assertCandidateEvidenceRootRejected(t, manifest, tc.wantErr)
		})
	}
}

func TestCandidateEvidenceRootManifestRejectsPathsEscapingDeclaredRoot(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateEvidenceRootManifest, string)
		wantErr string
	}{
		{
			name: "base journal",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.Base.JournalPath = filepath.Join(outsideRoot, "base.sqlite")
			},
			wantErr: "base journal path escapes declared root",
		},
		{
			name: "base blob root",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.Base.BlobRoot = filepath.Join(outsideRoot, "blobs")
			},
			wantErr: "base blob root path escapes declared root",
		},
		{
			name: "vm persistent dir",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.VM.PersistentDir = filepath.Join(outsideRoot, "vm")
			},
			wantErr: "vm persistent dir path escapes declared root",
		},
		{
			name: "vm data image",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.VM.DataImagePath = filepath.Join(outsideRoot, "vm", "data.img")
			},
			wantErr: "vm data image path escapes declared root",
		},
		{
			name: "vm kernel image",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.VM.KernelImagePath = filepath.Join(outsideRoot, "kernel", "vmlinux")
			},
			wantErr: "vm kernel image path escapes declared root",
		},
		{
			name: "vm rootfs",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.VM.RootfsPath = filepath.Join(outsideRoot, "rootfs.ext4")
			},
			wantErr: "vm rootfs path escapes declared root",
		},
		{
			name: "vm store disk",
			mutate: func(m *CandidateEvidenceRootManifest, outsideRoot string) {
				m.Fixture.VM.StoreDiskPath = filepath.Join(outsideRoot, "nix-store.ext4")
			},
			wantErr: "vm store disk path escapes declared root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := candidateEvidenceRootManifest(t, candidateEvidenceRootComputerVersion())
			outsideRoot := t.TempDir()
			tc.mutate(&manifest, outsideRoot)

			assertCandidateEvidenceRootRejected(t, manifest, tc.wantErr)
		})
	}
}

func TestCandidateEvidenceRootManifestRejectsInvalidPromotionEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateEvidenceRootManifest)
		wantErr string
	}{
		{
			name: "promotion candidate mismatch",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.Fixture.Promotion.Candidate = ComputerVersion{CodeRef: "git:other-candidate", ArtifactProgramRef: "tape:org/other-candidate"}
			},
			wantErr: "promotion candidate does not match fixture version",
		},
		{
			name: "invalid promotion certificate",
			mutate: func(m *CandidateEvidenceRootManifest) {
				m.Fixture.Promotion.OwnerApproved = false
			},
			wantErr: "owner approval is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := candidateEvidenceRootManifest(t, candidateEvidenceRootComputerVersion())
			tc.mutate(&manifest)

			assertCandidateEvidenceRootRejected(t, manifest, tc.wantErr)
		})
	}
}

func candidateEvidenceRootComputerVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:candidate-evidence-root", ArtifactProgramRef: "tape:org/candidate-evidence-root@2026-07-04"}
}

func candidateEvidenceRootManifest(t *testing.T, version ComputerVersion) CandidateEvidenceRootManifest {
	t.Helper()
	root := t.TempDir()
	fixture := ProductFixtureRoot{
		Version: version,
		Base: BaseCurrentStatePaths{
			JournalPath: filepath.Join(root, "base", "journal.sqlite"),
			BlobRoot:    filepath.Join(root, "base", "blobs"),
		},
		VM: VMManagerScopedPath{
			VMID:               "candidate-vm-1",
			PersistentDir:      filepath.Join(root, "vm", "candidate-vm-1"),
			DataImagePath:      filepath.Join(root, "vm", "candidate-vm-1", "data.img"),
			KernelImagePath:    filepath.Join(root, "boot", "vmlinux"),
			RootfsPath:         filepath.Join(root, "boot", "rootfs.ext4"),
			StoreDiskPath:      filepath.Join(root, "vm", "candidate-vm-1", "nix-store.ext4"),
			ComputerKind:       "desktop",
			OwnerID:            "owner-1",
			DesktopID:          "desktop-1",
			WorkerID:           "worker-1",
			CandidateID:        "candidate-evidence-root",
			Epoch:              1,
			DataImageClass:     StateClassDurableLegacyOpaque,
			PersistentDirClass: StateClassDurableLegacyOpaque,
			BootArtifactClass:  StateClassCodeArtifact,
		},
		Promotion: productFixtureRootPromotionCertificate(version),
	}
	return CandidateEvidenceRootManifest{
		ID:                    "candidate-evidence-root-2026-07-04",
		RootPath:              root,
		Source:                EvidenceRootSourceLocalCandidate,
		AuthorizedForSampling: true,
		Fixture:               fixture,
		EvidenceRefs:          []string{"evidence:base", "evidence:vm", "evidence:promotion"},
	}
}

func assertCandidateEvidenceRootRejected(t *testing.T, manifest CandidateEvidenceRootManifest, wantErr string) {
	t.Helper()
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("Validate() error = %v, want containing %q", err, wantErr)
	}
	fixture, err := manifest.ProductFixtureRoot()
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("ProductFixtureRoot() error = %v, want containing %q", err, wantErr)
	}
	if fixture.Version.Valid() || fixture.Base.JournalPath != "" || fixture.Base.BlobRoot != "" || fixture.VM.VMID != "" || fixture.Promotion.ID != "" {
		t.Fatalf("ProductFixtureRoot() admitted fixture despite rejection: %#v", fixture)
	}
}
