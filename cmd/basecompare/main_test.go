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

func TestRunComparesStdinObservationSetAsEquivalent(t *testing.T) {
	set := baseCurrentStateObservationSet()

	var stdout, stderr bytes.Buffer
	code := run(nil, bytes.NewReader(mustMarshalObservationSet(t, set)), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}

	result := decodeEquivalenceResult(t, stdout.Bytes())
	if result.Status != computerversion.EquivalenceEquivalent {
		t.Fatalf("status = %q, want %q; result=%#v", result.Status, computerversion.EquivalenceEquivalent, result)
	}
	if len(result.Differences) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("equivalent result reported differences/unsupported: %#v", result)
	}
}

func TestRunComparesLeftAndRightFilesAndReportsTamperedProjection(t *testing.T) {
	left := baseCurrentStateObservationSet()
	right := tamperBlobObservationValue(left)
	right.Name = "base-file-projection-tampered"

	root := t.TempDir()
	leftPath := filepath.Join(root, "left.json")
	rightPath := filepath.Join(root, "right.json")
	writeObservationSetFile(t, leftPath, left)
	writeObservationSetFile(t, rightPath, right)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--left", leftPath, "--right", rightPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit=%d, want 1; stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	result := decodeEquivalenceResult(t, stdout.Bytes())
	if result.Status != computerversion.EquivalenceNotEquivalent {
		t.Fatalf("status = %q, want %q; result=%#v", result.Status, computerversion.EquivalenceNotEquivalent, result)
	}
	if len(result.Differences) != 1 {
		t.Fatalf("differences = %#v, want exactly one tampered blob difference", result.Differences)
	}
	diff := result.Differences[0]
	if diff.Kind != computerversion.ObservationBlobSet || diff.Key != blobRefFixture() || diff.Reason != "observation values differ" {
		t.Fatalf("difference = %#v, want blob value mismatch for %s", diff, blobRefFixture())
	}
	if diff.Left == "" || diff.Right == "" || diff.Left == diff.Right {
		t.Fatalf("difference did not preserve distinct left/right values: %#v", diff)
	}
}

func TestRunRejectsStdinCollisionAndInvalidJSON(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		stdin        string
		stderrSubstr string
	}{
		{
			name:         "left and right cannot both read stdin",
			args:         []string{"--right", "-"},
			stdin:        string(mustMarshalObservationSet(t, baseCurrentStateObservationSet())),
			stderrSubstr: "--left and --right cannot both read stdin",
		},
		{
			name:         "invalid json",
			stdin:        `{not-json`,
			stderrSubstr: "read left observation set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run exit=%d, want 2; stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.stderrSubstr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.stderrSubstr)
			}
		})
	}
}

func baseCurrentStateObservationSet() computerversion.ObservationSet {
	blobRef := blobRefFixture()
	contentHash := strings.Repeat("a", 64)
	return computerversion.ObservationSet{
		Name: "base-current-state",
		Version: computerversion.ComputerVersion{
			CodeRef:            "git:basecompare-test-runtime",
			ArtifactProgramRef: "base-journal:test-cursor",
		},
		Observations: []computerversion.Observation{
			computerversion.FileManifestObservation("/base/items/base_item_notes", `{"item_id":"base_item_notes","name":"notes.md","kind":"file","version_id":"base_version_notes","blob_ref":"`+blobRef+`","content_hash":"`+contentHash+`"}`),
			{
				Kind:  computerversion.ObservationBlobSet,
				Key:   blobRef,
				Value: `{"blob_ref":"` + blobRef + `","size_bytes":21,"sha256":"` + contentHash + `"}`,
			},
		},
	}
}

func tamperBlobObservationValue(set computerversion.ObservationSet) computerversion.ObservationSet {
	set.Observations = append([]computerversion.Observation(nil), set.Observations...)
	for i := range set.Observations {
		if set.Observations[i].Kind == computerversion.ObservationBlobSet {
			set.Observations[i].Value = strings.Replace(set.Observations[i].Value, strings.Repeat("a", 64), strings.Repeat("b", 64), 1)
			return set
		}
	}
	panic("fixture has no blob observation")
}

func blobRefFixture() string {
	return "sha256:" + strings.Repeat("1", 64)
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
