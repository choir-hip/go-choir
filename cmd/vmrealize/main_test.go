package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestRunEmitsRealizationForExistingVMManagerPaths(t *testing.T) {
	persistentDir, dataImagePath, kernelImagePath, rootfsPath := createVMRealizeFixturePaths(t)
	var stdout, stderr bytes.Buffer

	code := run(validVMRealizeArgs(
		"--id", "realization-fixture-1",
		"--materializer", "fixture-vmmanager-materializer",
		"--vm-id", "vm-realize-1",
		"--persistent-dir", persistentDir,
		"--data-image", dataImagePath,
		"--kernel-image", kernelImagePath,
		"--rootfs", rootfsPath,
		"--epoch", "7",
	), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	realization := decodeRealization(t, stdout.Bytes())
	if realization.ID != "realization-fixture-1" {
		t.Fatalf("realization id = %q, want %q", realization.ID, "realization-fixture-1")
	}
	if realization.Version.CodeRef != "git:vmrealize-test" {
		t.Fatalf("code_ref = %q, want %q", realization.Version.CodeRef, "git:vmrealize-test")
	}
	if realization.Version.ArtifactProgramRef != "tape:owner/vmrealize-test" {
		t.Fatalf("artifact_program_ref = %q, want %q", realization.Version.ArtifactProgramRef, "tape:owner/vmrealize-test")
	}
	if realization.Capabilities.Materializer != "fixture-vmmanager-materializer" {
		t.Fatalf("capability materializer = %q, want %q", realization.Capabilities.Materializer, "fixture-vmmanager-materializer")
	}
	if realization.Capabilities.Substrate != computerversion.VMManagerSubstrateFirecracker {
		t.Fatalf("capability substrate = %q, want %q", realization.Capabilities.Substrate, computerversion.VMManagerSubstrateFirecracker)
	}
	if len(realization.Capabilities.Supported) != 1 || realization.Capabilities.Supported[0] != computerversion.ObservationVMStateManifest {
		t.Fatalf("capability supported = %#v, want exactly vm_state_manifest", realization.Capabilities.Supported)
	}
	assertUnsupportedCapabilities(t, realization.Capabilities.Unsupported, []computerversion.ObservationKind{
		computerversion.ObservationFileManifest,
		computerversion.ObservationBlobSet,
		computerversion.ObservationDoltHead,
		computerversion.ObservationObjectGraphHead,
		computerversion.ObservationProvenanceAnswer,
		computerversion.ObservationLiveProcessContinuity,
	})

	observations := realization.Observations
	if observations.Name != "realization-fixture-1" {
		t.Fatalf("observation set name = %q, want %q", observations.Name, "realization-fixture-1")
	}
	if observations.Version != realization.Version {
		t.Fatalf("observation version = %#v, want realization version %#v", observations.Version, realization.Version)
	}
	if len(observations.Required) != 1 || observations.Required[0] != computerversion.ObservationVMStateManifest {
		t.Fatalf("required = %#v, want exactly vm_state_manifest", observations.Required)
	}
	if len(observations.Observations) != 1 {
		t.Fatalf("observations = %#v, want exactly one observation", observations.Observations)
	}
	observation := observations.Observations[0]
	if observation.Kind != computerversion.ObservationVMStateManifest {
		t.Fatalf("observation kind = %q, want %q", observation.Kind, computerversion.ObservationVMStateManifest)
	}
	if observation.Key != "vmmanager:vm-realize-1" {
		t.Fatalf("observation key = %q, want %q", observation.Key, "vmmanager:vm-realize-1")
	}
	payload := decodeVMStateManifestPayload(t, observation.Value)
	if payload.Substrate != computerversion.VMManagerSubstrateFirecracker {
		t.Fatalf("payload substrate = %q, want %q", payload.Substrate, computerversion.VMManagerSubstrateFirecracker)
	}
	if payload.VMID != "vm-realize-1" {
		t.Fatalf("payload vm_id = %q, want %q", payload.VMID, "vm-realize-1")
	}
	if payload.PersistentDir != persistentDir {
		t.Fatalf("payload persistent_dir = %q, want %q", payload.PersistentDir, persistentDir)
	}
	if payload.DataImagePath != dataImagePath {
		t.Fatalf("payload data_image_path = %q, want %q", payload.DataImagePath, dataImagePath)
	}
	if payload.KernelImagePath != kernelImagePath {
		t.Fatalf("payload kernel_image_path = %q, want %q", payload.KernelImagePath, kernelImagePath)
	}
	if payload.RootfsPath != rootfsPath {
		t.Fatalf("payload rootfs_path = %q, want %q", payload.RootfsPath, rootfsPath)
	}
	if payload.Epoch != 7 {
		t.Fatalf("payload epoch = %d, want 7", payload.Epoch)
	}
	if payload.DataImageClass != computerversion.StateClassDurableLegacyOpaque {
		t.Fatalf("payload data_image_class = %q, want %q", payload.DataImageClass, computerversion.StateClassDurableLegacyOpaque)
	}
	if payload.PersistentDirClass != computerversion.StateClassDurableLegacyOpaque {
		t.Fatalf("payload persistent_dir_class = %q, want %q", payload.PersistentDirClass, computerversion.StateClassDurableLegacyOpaque)
	}
	if payload.BootArtifactClass != computerversion.StateClassCodeArtifact {
		t.Fatalf("payload boot_artifact_class = %q, want %q", payload.BootArtifactClass, computerversion.StateClassCodeArtifact)
	}
	assertFileContent(t, dataImagePath, "vm data image fixture")
	assertFileContent(t, kernelImagePath, "kernel image fixture")
	assertFileContent(t, rootfsPath, "rootfs fixture")
}

func TestRunUsesMaterializerAsDefaultRealizationID(t *testing.T) {
	persistentDir, dataImagePath, _, _ := createVMRealizeFixturePaths(t)
	var stdout, stderr bytes.Buffer

	code := run(validVMRealizeArgs(
		"--materializer", "fixture-default-id-materializer",
		"--vm-id", "vm-default-id",
		"--persistent-dir", persistentDir,
		"--data-image", dataImagePath,
	), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	realization := decodeRealization(t, stdout.Bytes())
	if realization.ID != "fixture-default-id-materializer" {
		t.Fatalf("realization id = %q, want materializer fallback %q", realization.ID, "fixture-default-id-materializer")
	}
}

func TestRunRejectsMissingRequiredFlagsBeforeJSON(t *testing.T) {
	persistentDir, _, _, _ := createVMRealizeFixturePaths(t)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "missing vm id",
			args: validVMRealizeArgs(
				"--persistent-dir", persistentDir,
			),
			wantStderr: "--vm-id is required",
		},
		{
			name: "missing code ref",
			args: []string{
				"--vm-id", "vm-missing-code-ref",
				"--persistent-dir", persistentDir,
				"--artifact-program-ref", "tape:owner/vmrealize-test",
			},
			wantStderr: "--code-ref is required",
		},
		{
			name: "missing artifact program ref",
			args: []string{
				"--vm-id", "vm-missing-artifact-program-ref",
				"--persistent-dir", persistentDir,
				"--code-ref", "git:vmrealize-test",
			},
			wantStderr: "--artifact-program-ref is required",
		},
		{
			name: "missing state path",
			args: validVMRealizeArgs(
				"--vm-id", "vm-missing-state-path",
			),
			wantStderr: "--persistent-dir or --data-image is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			assertNoJSON(t, stdout.Bytes())
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunRequireExistingGuardsMissingPathsAndCanBeDisabled(t *testing.T) {
	root := t.TempDir()
	missingPersistentDir := filepath.Join(root, "missing-persistent")
	missingDataImagePath := filepath.Join(root, "missing-data.img")
	missingKernelImagePath := filepath.Join(root, "missing-vmlinux")
	missingRootfsPath := filepath.Join(root, "missing-rootfs.ext4")
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "missing persistent dir",
			args: validVMRealizeArgs(
				"--vm-id", "vm-missing-persistent",
				"--persistent-dir", missingPersistentDir,
				"--require-existing=true",
			),
			wantStderr: "--persistent-dir",
		},
		{
			name: "missing data image",
			args: validVMRealizeArgs(
				"--vm-id", "vm-missing-data-image",
				"--data-image", missingDataImagePath,
				"--require-existing=true",
			),
			wantStderr: "--data-image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			assertNoJSON(t, stdout.Bytes())
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
			if !strings.Contains(stderr.String(), "no such file") {
				t.Fatalf("stderr = %q, want nonexistent path error", stderr.String())
			}
		})
	}

	var stdout, stderr bytes.Buffer
	code := run(validVMRealizeArgs(
		"--id", "synthetic-realization",
		"--vm-id", "vm-synthetic-1",
		"--persistent-dir", missingPersistentDir,
		"--data-image", missingDataImagePath,
		"--kernel-image", missingKernelImagePath,
		"--rootfs", missingRootfsPath,
		"--require-existing=false",
	), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	realization := decodeRealization(t, stdout.Bytes())
	if realization.ID != "synthetic-realization" {
		t.Fatalf("realization id = %q, want %q", realization.ID, "synthetic-realization")
	}
	if len(realization.Observations.Observations) != 1 || realization.Observations.Observations[0].Key != "vmmanager:vm-synthetic-1" {
		t.Fatalf("observations = %#v, want one vmmanager:vm-synthetic-1 observation", realization.Observations.Observations)
	}
	payload := decodeVMStateManifestPayload(t, realization.Observations.Observations[0].Value)
	if payload.PersistentDir != missingPersistentDir {
		t.Fatalf("payload persistent_dir = %q, want %q", payload.PersistentDir, missingPersistentDir)
	}
	if payload.DataImagePath != missingDataImagePath {
		t.Fatalf("payload data_image_path = %q, want %q", payload.DataImagePath, missingDataImagePath)
	}
	if payload.KernelImagePath != missingKernelImagePath {
		t.Fatalf("payload kernel_image_path = %q, want %q", payload.KernelImagePath, missingKernelImagePath)
	}
	if payload.RootfsPath != missingRootfsPath {
		t.Fatalf("payload rootfs_path = %q, want %q", payload.RootfsPath, missingRootfsPath)
	}
	assertPathMissing(t, missingPersistentDir)
	assertPathMissing(t, missingDataImagePath)
	assertPathMissing(t, missingKernelImagePath)
	assertPathMissing(t, missingRootfsPath)
}

type vmStateManifestPayload struct {
	Substrate          string `json:"substrate"`
	VMID               string `json:"vm_id"`
	PersistentDir      string `json:"persistent_dir"`
	DataImagePath      string `json:"data_image_path"`
	KernelImagePath    string `json:"kernel_image_path"`
	RootfsPath         string `json:"rootfs_path"`
	Epoch              int64  `json:"epoch"`
	DataImageClass     string `json:"data_image_class"`
	PersistentDirClass string `json:"persistent_dir_class"`
	BootArtifactClass  string `json:"boot_artifact_class"`
}

func createVMRealizeFixturePaths(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	persistentDir := filepath.Join(root, "persistent")
	if err := os.Mkdir(persistentDir, 0o700); err != nil {
		t.Fatalf("create persistent dir: %v", err)
	}
	dataImagePath := filepath.Join(root, "data.img")
	if err := os.WriteFile(dataImagePath, []byte("vm data image fixture"), 0o600); err != nil {
		t.Fatalf("write data image fixture: %v", err)
	}
	kernelImagePath := filepath.Join(root, "vmlinux")
	if err := os.WriteFile(kernelImagePath, []byte("kernel image fixture"), 0o600); err != nil {
		t.Fatalf("write kernel image fixture: %v", err)
	}
	rootfsPath := filepath.Join(root, "rootfs.ext4")
	if err := os.WriteFile(rootfsPath, []byte("rootfs fixture"), 0o600); err != nil {
		t.Fatalf("write rootfs fixture: %v", err)
	}
	return persistentDir, dataImagePath, kernelImagePath, rootfsPath
}

func validVMRealizeArgs(args ...string) []string {
	out := []string{
		"--code-ref", "git:vmrealize-test",
		"--artifact-program-ref", "tape:owner/vmrealize-test",
	}
	out = append(out, args...)
	return out
}

func decodeRealization(t *testing.T, data []byte) computerversion.Realization {
	t.Helper()
	var realization computerversion.Realization
	if err := json.Unmarshal(data, &realization); err != nil {
		t.Fatalf("decode realization json: %v; stdout=%s", err, string(data))
	}
	return realization
}

func decodeVMStateManifestPayload(t *testing.T, value string) vmStateManifestPayload {
	t.Helper()
	var payload vmStateManifestPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode vm_state_manifest payload json: %v; value=%s", err, value)
	}
	return payload
}

func assertUnsupportedCapabilities(t *testing.T, got []computerversion.UnsupportedCapability, want []computerversion.ObservationKind) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unsupported capabilities = %#v, want kinds %#v", got, want)
	}
	byKind := make(map[computerversion.ObservationKind]computerversion.UnsupportedCapability, len(got))
	for _, unsupported := range got {
		if unsupported.Reason == "" {
			t.Fatalf("unsupported capability %q has empty reason", unsupported.Kind)
		}
		byKind[unsupported.Kind] = unsupported
	}
	for _, kind := range want {
		if _, ok := byKind[kind]; !ok {
			t.Fatalf("unsupported capabilities = %#v, missing kind %q", got, kind)
		}
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s content = %q, want %q", path, string(data), want)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("path %s exists, want missing", path)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v, want not exist", path, err)
	}
}

func assertNoJSON(t *testing.T, data []byte) {
	t.Helper()
	if len(bytes.TrimSpace(data)) != 0 {
		t.Fatalf("stdout = %s, want no JSON", string(data))
	}
}
