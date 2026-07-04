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

func TestRunDefaultsRightToLeftAndEmitsEquivalentVMStateManifestResult(t *testing.T) {
	set := vmStateManifestObservationSet(1)

	var stdout, stderr bytes.Buffer
	exitCode := run(nil, bytes.NewReader(mustMarshalObservationSet(t, set)), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run exit=%d, want 0; stderr=%s stdout=%s", exitCode, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	result := decodeEquivalenceResult(t, stdout.Bytes())
	if result.Status != computerversion.EquivalenceEquivalent {
		t.Fatalf("status = %q, want %q; result=%#v", result.Status, computerversion.EquivalenceEquivalent, result)
	}
	if len(result.Differences) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("equivalent result reported differences/unsupported: %#v", result)
	}
}

func TestRunComparesLeftAndRightFilesAndReportsVMStateManifestMismatch(t *testing.T) {
	left := vmStateManifestObservationSet(1)
	right := vmStateManifestObservationSet(2)
	right.Name = "right-vm-state"

	root := t.TempDir()
	leftPath := filepath.Join(root, "left.json")
	rightPath := filepath.Join(root, "right.json")
	writeObservationSetFile(t, leftPath, left)
	writeObservationSetFile(t, rightPath, right)

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--left", leftPath, "--right", rightPath}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("run exit=%d, want 1; stderr=%s stdout=%s", exitCode, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	result := decodeEquivalenceResult(t, stdout.Bytes())
	if result.Status != computerversion.EquivalenceNotEquivalent {
		t.Fatalf("status = %q, want %q; result=%#v", result.Status, computerversion.EquivalenceNotEquivalent, result)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("not_equivalent vm_state_manifest result reported unsupported capabilities: %#v", result.Unsupported)
	}
	if len(result.Differences) != 1 {
		t.Fatalf("differences = %#v, want exactly one vm_state_manifest mismatch", result.Differences)
	}
	diff := result.Differences[0]
	if diff.Kind != computerversion.ObservationVMStateManifest || diff.Key != vmStateManifestKey() || diff.Reason != "observation values differ" {
		t.Fatalf("difference = %#v, want vm_state_manifest value mismatch for %s", diff, vmStateManifestKey())
	}
	if diff.Left != vmStateManifestValue(1) || diff.Right != vmStateManifestValue(2) {
		t.Fatalf("difference left/right = (%q, %q), want seeded manifest values", diff.Left, diff.Right)
	}
}

func TestRunNarrowsDurableFileManifestClaimUnderVMManagerCapability(t *testing.T) {
	set := fileManifestObservationSet()

	var stdout, stderr bytes.Buffer
	exitCode := run(nil, bytes.NewReader(mustMarshalObservationSet(t, set)), &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("run exit=%d, want 1; stderr=%s stdout=%s", exitCode, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	result := decodeEquivalenceResult(t, stdout.Bytes())
	if result.Status != computerversion.EquivalenceNarrowed {
		t.Fatalf("status = %q, want %q; result=%#v", result.Status, computerversion.EquivalenceNarrowed, result)
	}
	if len(result.Differences) != 0 {
		t.Fatalf("narrowed file_manifest result reported observation differences: %#v", result.Differences)
	}
	if len(result.Unsupported) != 2 {
		t.Fatalf("unsupported = %#v, want left and right file_manifest capability gaps", result.Unsupported)
	}
	for _, unsupported := range result.Unsupported {
		if unsupported.Kind != computerversion.ObservationFileManifest {
			t.Fatalf("unsupported = %#v, want only file_manifest capability gaps", result.Unsupported)
		}
		if unsupported.Reason == "" {
			t.Fatalf("unsupported capability should include a reason: %#v", unsupported)
		}
	}
}

func TestRunReturnsInputErrorBeforeEmittingEquivalenceClaim(t *testing.T) {
	validSet := vmStateManifestObservationSet(1)
	tests := []struct {
		name         string
		args         []string
		stdin        string
		stderrSubstr string
	}{
		{
			name:         "invalid json",
			stdin:        `{not-json`,
			stderrSubstr: "read left observation set",
		},
		{
			name:         "left and right both read stdin",
			args:         []string{"--right", "-"},
			stdin:        string(mustMarshalObservationSet(t, validSet)),
			stderrSubstr: "--left and --right cannot both read stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("run exit=%d, want 2; stderr=%s stdout=%s", exitCode, stderr.String(), stdout.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty because no equivalence claim should be emitted", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.stderrSubstr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.stderrSubstr)
			}
		})
	}
}

func vmStateManifestObservationSet(epoch int64) computerversion.ObservationSet {
	return computerversion.ObservationSet{
		Name: "left-vm-state",
		Version: computerversion.ComputerVersion{
			CodeRef:            "git:vmstatecompare-test-runtime",
			ArtifactProgramRef: "vmmanager:test-cursor",
		},
		Required: []computerversion.ObservationKind{computerversion.ObservationVMStateManifest},
		Observations: []computerversion.Observation{{
			Kind:  computerversion.ObservationVMStateManifest,
			Key:   vmStateManifestKey(),
			Value: vmStateManifestValue(epoch),
		}},
	}
}

func fileManifestObservationSet() computerversion.ObservationSet {
	return computerversion.ObservationSet{
		Name: "durable-file-state",
		Version: computerversion.ComputerVersion{
			CodeRef:            "git:vmstatecompare-test-runtime",
			ArtifactProgramRef: "base-journal:test-cursor",
		},
		Required: []computerversion.ObservationKind{computerversion.ObservationFileManifest},
		Observations: []computerversion.Observation{
			computerversion.FileManifestObservation("/home/alice/note.txt", "sha256:"+strings.Repeat("a", 64)),
		},
	}
}

func vmStateManifestKey() string {
	return "vmmanager:vmstatecompare-test-vm"
}

func vmStateManifestValue(epoch int64) string {
	payload := struct {
		Substrate          string `json:"substrate"`
		VMID               string `json:"vm_id"`
		PersistentDir      string `json:"persistent_dir"`
		DataImagePath      string `json:"data_image_path"`
		KernelImagePath    string `json:"kernel_image_path"`
		RootfsPath         string `json:"rootfs_path"`
		StoreDiskPath      string `json:"store_disk_path"`
		Epoch              int64  `json:"epoch"`
		DataImageClass     string `json:"data_image_class"`
		PersistentDirClass string `json:"persistent_dir_class"`
		BootArtifactClass  string `json:"boot_artifact_class"`
	}{
		Substrate:          computerversion.VMManagerSubstrateFirecracker,
		VMID:               "vmstatecompare-test-vm",
		PersistentDir:      "/var/lib/choir/vmstatecompare-test-vm",
		DataImagePath:      "/var/lib/choir/vmstatecompare-test-vm/data.img",
		KernelImagePath:    "/nix/store/kernel/vmlinux",
		RootfsPath:         "/nix/store/rootfs/rootfs.ext4",
		StoreDiskPath:      "/var/lib/choir/vmstatecompare-test-vm/nix-store.ext4",
		Epoch:              epoch,
		DataImageClass:     computerversion.StateClassDurableLegacyOpaque,
		PersistentDirClass: computerversion.StateClassDurableLegacyOpaque,
		BootArtifactClass:  computerversion.StateClassCodeArtifact,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func mustMarshalObservationSet(t *testing.T, set computerversion.ObservationSet) []byte {
	t.Helper()
	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal observation set: %v", err)
	}
	return data
}

func writeObservationSetFile(t *testing.T, path string, set computerversion.ObservationSet) {
	t.Helper()
	if err := os.WriteFile(path, mustMarshalObservationSet(t, set), 0o600); err != nil {
		t.Fatalf("write observation set %s: %v", path, err)
	}
}

func decodeEquivalenceResult(t *testing.T, data []byte) computerversion.EquivalenceResult {
	t.Helper()
	var result computerversion.EquivalenceResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode equivalence result: %v\n%s", err, string(data))
	}
	return result
}
