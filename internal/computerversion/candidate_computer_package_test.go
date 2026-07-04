package computerversion

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCandidateComputerPackageAdmitsLocalEvidenceRootAndHashesStablePackage(t *testing.T) {
	version := candidateComputerPackageVersion()
	manifest := candidateComputerPackageManifest(t, version)

	built, err := BuildCandidateComputerPackage(manifest)
	if err != nil {
		t.Fatalf("build candidate computer package: %v", err)
	}

	if built.Kind != CandidateComputerPackageKind {
		t.Fatalf("kind = %q, want %q", built.Kind, CandidateComputerPackageKind)
	}
	if built.Version != version {
		t.Fatalf("version = %#v, want %#v", built.Version, version)
	}
	wantKinds := []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	}
	assertObservationBundleKinds(t, built.RequiredObservations, wantKinds)
	if !strings.HasPrefix(built.PackageManifestSHA256, "sha256:") || len(built.PackageManifestSHA256) != len("sha256:")+64 {
		t.Fatalf("package_manifest_sha256 = %q, want sha256-prefixed 32-byte digest", built.PackageManifestSHA256)
	}
	if err := built.Validate(); err != nil {
		t.Fatalf("Validate() rejected built package: %v", err)
	}

	rebuilt, err := BuildCandidateComputerPackage(built)
	if err != nil {
		t.Fatalf("rebuild candidate computer package: %v", err)
	}
	if rebuilt.PackageManifestSHA256 != built.PackageManifestSHA256 {
		t.Fatalf("stable package hash = %q, want %q", rebuilt.PackageManifestSHA256, built.PackageManifestSHA256)
	}
}

func TestBuildCandidateComputerPackageRejectsProductionAndDeployedRouteFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateComputerPackageManifest)
		wantErr string
	}{
		{
			name: "package production flag",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.ContainsProduction = true
			},
			wantErr: "production state is not admissible",
		},
		{
			name: "evidence root production flag",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.EvidenceRoot.ContainsProduction = true
			},
			wantErr: "production state is not admissible",
		},
		{
			name: "package deployed route flag",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.TouchesDeployedRoute = true
			},
			wantErr: "deployed route mutation is not admissible",
		},
		{
			name: "evidence root deployed route flag",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.EvidenceRoot.TouchesDeployedRoute = true
			},
			wantErr: "deployed route mutation is not admissible",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := candidateComputerPackageManifest(t, candidateComputerPackageVersion())
			tc.mutate(&manifest)

			built, err := BuildCandidateComputerPackage(manifest)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidateComputerPackage() = package %#v error %v, want error containing %q", built, err, tc.wantErr)
			}
		})
	}
}

func TestCandidateComputerPackageManifestRejectsVersionMismatches(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-candidate", ArtifactProgramRef: "tape:org/foreign-candidate@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateComputerPackageManifest)
		wantErr string
	}{
		{
			name: "evidence root observation version",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.EvidenceRootObservation.Version = foreignVersion
			},
			wantErr: "evidence root observation version does not match package version",
		},
		{
			name: "realization version",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.Realizations[0].Version = foreignVersion
			},
			wantErr: "realization 0: version does not match package version",
		},
		{
			name: "realization observation version",
			mutate: func(m *CandidateComputerPackageManifest) {
				m.Realizations[0].Observations.Version = foreignVersion
			},
			wantErr: "realization 0: observation set version does not match realization version",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
			tc.mutate(&manifest)

			assertCandidateComputerPackageValidateError(t, manifest, tc.wantErr)
		})
	}
}

func TestCandidateComputerPackageManifestRejectsMissingRequiredObservationKind(t *testing.T) {
	manifest := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
	manifest.RequiredObservations = append(manifest.RequiredObservations, ObservationObjectGraphHead)

	assertCandidateComputerPackageValidateError(t, manifest, "required observation \"object_graph_head\" is missing from bundled evidence")
}

func TestCandidateComputerPackageManifestRejectsRealizationCapabilityThatCannotSupportItsRequiredObservation(t *testing.T) {
	manifest := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
	manifest.Realizations[0].Capabilities = CapabilityManifest{
		Materializer: "vmmanager-fixture",
		Substrate:    VMManagerSubstrateFirecracker,
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	assertCandidateComputerPackageValidateError(t, manifest, "realization 0: capability manifest does not support required observations")
	assertCandidateComputerPackageValidateError(t, manifest, string(ObservationVMStateManifest))
}

func candidateComputerPackageVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:candidate-computer-package", ArtifactProgramRef: "tape:org/candidate-computer-package@2026-07-04"}
}

func candidateComputerPackageManifest(t *testing.T, version ComputerVersion) CandidateComputerPackageManifest {
	t.Helper()
	root := candidateComputerPackageEvidenceRoot(t, version)
	observation, err := root.Fixture.ObservationSet(context.Background(), root.ID)
	if err != nil {
		t.Fatalf("evidence root observation set: %v", err)
	}
	realization := mustMaterializeVMManagerBoundary(t, "candidate-package-vmmanager", version, root.Fixture.VM, VMManagerCapabilityManifest("candidate-package-vmmanager"))
	return CandidateComputerPackageManifest{
		ID:                      "candidate-computer-package-2026-07-04",
		Version:                 version,
		SourceComputerID:        "source-computer-1",
		SourceCandidateID:       "candidate-1",
		CandidateSourceRef:      "local://candidate-computer-package",
		EvidenceRoot:            root,
		EvidenceRootObservation: observation,
		Realizations:            []Realization{realization},
		EvidenceRefs:            []string{"vmrealize:manifest", "evidenceroot:manifest", "evidenceroot:manifest"},
	}
}

func candidateComputerPackageBuiltManifest(t *testing.T, version ComputerVersion) CandidateComputerPackageManifest {
	t.Helper()
	manifest, err := BuildCandidateComputerPackage(candidateComputerPackageManifest(t, version))
	if err != nil {
		t.Fatalf("build valid candidate computer package: %v", err)
	}
	return manifest
}

func candidateComputerPackageEvidenceRoot(t *testing.T, version ComputerVersion) CandidateEvidenceRootManifest {
	t.Helper()
	root := t.TempDir()
	fixture := ProductFixtureRoot{
		Version:   version,
		Base:      candidateComputerPackageBasePaths(t, root),
		VM:        candidateComputerPackageVMPath(root),
		Promotion: productFixtureRootPromotionCertificate(version),
	}
	return CandidateEvidenceRootManifest{
		ID:                    "candidate-package-evidence-root",
		RootPath:              root,
		Source:                EvidenceRootSourceLocalCandidate,
		AuthorizedForSampling: true,
		Fixture:               fixture,
		EvidenceRefs:          []string{"candidate-package:base", "candidate-package:vm", "candidate-package:promotion"},
	}
}

func candidateComputerPackageBasePaths(t *testing.T, root string) BaseCurrentStatePaths {
	t.Helper()
	blobRoot := filepath.Join(root, "blobs")
	blobs := newBaseBlobStore(t, blobRoot)
	ref, contentHash := putBaseBlob(t, blobs, []byte("candidate computer package base blob"))
	journalPath := filepath.Join(root, "base.sqlite")
	journal := newSQLiteJournalAtPathWithEvent(t, journalPath, baseCreateEventWithBlob(23, ref, contentHash))
	if err := journal.Close(); err != nil {
		t.Fatalf("close writable journal: %v", err)
	}
	return BaseCurrentStatePaths{JournalPath: journalPath, BlobRoot: blobRoot}
}

func candidateComputerPackageVMPath(root string) VMManagerScopedPath {
	return VMManagerScopedPath{
		VMID:               "candidate-package-vm-1",
		PersistentDir:      filepath.Join(root, "vm", "candidate-package-vm-1"),
		DataImagePath:      filepath.Join(root, "vm", "candidate-package-vm-1", "data.img"),
		KernelImagePath:    filepath.Join(root, "boot", "vmlinux"),
		RootfsPath:         filepath.Join(root, "boot", "rootfs.ext4"),
		StoreDiskPath:      filepath.Join(root, "vm", "candidate-package-vm-1", "nix-store.ext4"),
		ComputerKind:       "desktop",
		OwnerID:            "owner-1",
		DesktopID:          "desktop-1",
		WorkerID:           "worker-1",
		CandidateID:        "candidate-package-1",
		Epoch:              7,
		DataImageClass:     StateClassDurableLegacyOpaque,
		PersistentDirClass: StateClassDurableLegacyOpaque,
		BootArtifactClass:  StateClassCodeArtifact,
	}
}

func assertCandidateComputerPackageValidateError(t *testing.T, manifest CandidateComputerPackageManifest, wantErr string) {
	t.Helper()
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("Validate() error = %v, want containing %q", err, wantErr)
	}
}
