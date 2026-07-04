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

func TestRunEmitsVMStateManifestForExistingExplicitPaths(t *testing.T) {
	persistentDir, dataImagePath := createVMStateFixturePaths(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--vm-id", "vm-fixture-1",
		"--persistent-dir", persistentDir,
		"--data-image", dataImagePath,
		"--epoch", "42",
		"--code-ref", "git:vmstateobserve-test",
		"--artifact-program-ref", "tape:owner/vmstateobserve-test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	set := decodeObservationSet(t, stdout.Bytes())
	if set.Version.CodeRef != "git:vmstateobserve-test" {
		t.Fatalf("code_ref = %q, want %q", set.Version.CodeRef, "git:vmstateobserve-test")
	}
	if set.Version.ArtifactProgramRef != "tape:owner/vmstateobserve-test" {
		t.Fatalf("artifact_program_ref = %q, want %q", set.Version.ArtifactProgramRef, "tape:owner/vmstateobserve-test")
	}
	if len(set.Required) != 1 || set.Required[0] != computerversion.ObservationVMStateManifest {
		t.Fatalf("required = %#v, want exactly vm_state_manifest", set.Required)
	}
	if len(set.Observations) != 1 {
		t.Fatalf("observations = %#v, want exactly one observation", set.Observations)
	}

	observation := set.Observations[0]
	if observation.Kind != computerversion.ObservationVMStateManifest {
		t.Fatalf("observation kind = %q, want %q", observation.Kind, computerversion.ObservationVMStateManifest)
	}
	if observation.Key != "vmmanager:vm-fixture-1" {
		t.Fatalf("observation key = %q, want %q", observation.Key, "vmmanager:vm-fixture-1")
	}
	payload := decodeVMStateManifestPayload(t, observation.Value)
	if payload.Substrate != computerversion.VMManagerSubstrateFirecracker {
		t.Fatalf("payload substrate = %q, want %q", payload.Substrate, computerversion.VMManagerSubstrateFirecracker)
	}
	if payload.VMID != "vm-fixture-1" {
		t.Fatalf("payload vm_id = %q, want %q", payload.VMID, "vm-fixture-1")
	}
	if payload.PersistentDir != persistentDir {
		t.Fatalf("payload persistent_dir = %q, want %q", payload.PersistentDir, persistentDir)
	}
	if payload.DataImagePath != dataImagePath {
		t.Fatalf("payload data_image_path = %q, want %q", payload.DataImagePath, dataImagePath)
	}
	if payload.Epoch != 42 {
		t.Fatalf("payload epoch = %d, want 42", payload.Epoch)
	}
	if payload.PersistentDirClass != computerversion.StateClassDurableLegacyOpaque {
		t.Fatalf("payload persistent_dir_class = %q, want %q", payload.PersistentDirClass, computerversion.StateClassDurableLegacyOpaque)
	}
	if payload.DataImageClass != computerversion.StateClassDurableLegacyOpaque {
		t.Fatalf("payload data_image_class = %q, want %q", payload.DataImageClass, computerversion.StateClassDurableLegacyOpaque)
	}
}

func TestRunRejectsMissingRequiredFlagsBeforeJSON(t *testing.T) {
	persistentDir, _ := createVMStateFixturePaths(t)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "missing vm id",
			args: []string{
				"--persistent-dir", persistentDir,
				"--code-ref", "git:vmstateobserve-test",
				"--artifact-program-ref", "tape:owner/vmstateobserve-test",
			},
			wantStderr: "--vm-id is required",
		},
		{
			name: "missing code ref",
			args: []string{
				"--vm-id", "vm-fixture-1",
				"--persistent-dir", persistentDir,
				"--artifact-program-ref", "tape:owner/vmstateobserve-test",
			},
			wantStderr: "--code-ref is required",
		},
		{
			name: "missing artifact program ref",
			args: []string{
				"--vm-id", "vm-fixture-1",
				"--persistent-dir", persistentDir,
				"--code-ref", "git:vmstateobserve-test",
			},
			wantStderr: "--artifact-program-ref is required",
		},
		{
			name: "missing state path",
			args: []string{
				"--vm-id", "vm-fixture-1",
				"--code-ref", "git:vmstateobserve-test",
				"--artifact-program-ref", "tape:owner/vmstateobserve-test",
			},
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

func TestRunRequireExistingRejectsNonexistentPathsAndCanBeDisabled(t *testing.T) {
	root := t.TempDir()
	missingPersistentDir := filepath.Join(root, "missing-persistent")
	missingDataImagePath := filepath.Join(root, "missing-data.img")
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "missing persistent dir",
			args: validVersionArgs(
				"--vm-id", "vm-missing-persistent",
				"--persistent-dir", missingPersistentDir,
			),
			wantStderr: "--persistent-dir",
		},
		{
			name: "missing data image",
			args: validVersionArgs(
				"--vm-id", "vm-missing-data-image",
				"--data-image", missingDataImagePath,
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
	code := run(validVersionArgs(
		"--vm-id", "vm-synthetic-1",
		"--persistent-dir", missingPersistentDir,
		"--data-image", missingDataImagePath,
		"--require-existing=false",
	), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	set := decodeObservationSet(t, stdout.Bytes())
	if len(set.Observations) != 1 || set.Observations[0].Key != "vmmanager:vm-synthetic-1" {
		t.Fatalf("observations = %#v, want one vmmanager:vm-synthetic-1 observation", set.Observations)
	}
	payload := decodeVMStateManifestPayload(t, set.Observations[0].Value)
	if payload.PersistentDir != missingPersistentDir {
		t.Fatalf("payload persistent_dir = %q, want %q", payload.PersistentDir, missingPersistentDir)
	}
	if payload.DataImagePath != missingDataImagePath {
		t.Fatalf("payload data_image_path = %q, want %q", payload.DataImagePath, missingDataImagePath)
	}
}

func TestRunRejectsInvalidEpochBeforeJSON(t *testing.T) {
	persistentDir, _ := createVMStateFixturePaths(t)
	var stdout, stderr bytes.Buffer

	code := run(validVersionArgs(
		"--vm-id", "vm-invalid-epoch",
		"--persistent-dir", persistentDir,
		"--epoch", "forty-two",
	), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	assertNoJSON(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), "--epoch must be an int64") {
		t.Fatalf("stderr = %q, want invalid epoch error", stderr.String())
	}
}

type vmStateManifestPayload struct {
	Substrate          string `json:"substrate"`
	VMID               string `json:"vm_id"`
	PersistentDir      string `json:"persistent_dir"`
	DataImagePath      string `json:"data_image_path"`
	Epoch              int64  `json:"epoch"`
	DataImageClass     string `json:"data_image_class"`
	PersistentDirClass string `json:"persistent_dir_class"`
}

func createVMStateFixturePaths(t *testing.T) (string, string) {
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
	return persistentDir, dataImagePath
}

func validVersionArgs(args ...string) []string {
	out := []string{
		"--code-ref", "git:vmstateobserve-test",
		"--artifact-program-ref", "tape:owner/vmstateobserve-test",
	}
	out = append(out, args...)
	return out
}

func decodeObservationSet(t *testing.T, data []byte) computerversion.ObservationSet {
	t.Helper()
	var set computerversion.ObservationSet
	if err := json.Unmarshal(data, &set); err != nil {
		t.Fatalf("decode observation set json: %v; stdout=%s", err, string(data))
	}
	return set
}

func decodeVMStateManifestPayload(t *testing.T, value string) vmStateManifestPayload {
	t.Helper()
	var payload vmStateManifestPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode vm_state_manifest payload json: %v; value=%s", err, value)
	}
	return payload
}

func assertNoJSON(t *testing.T, data []byte) {
	t.Helper()
	if len(bytes.TrimSpace(data)) != 0 {
		t.Fatalf("stdout = %s, want no JSON", string(data))
	}
}
