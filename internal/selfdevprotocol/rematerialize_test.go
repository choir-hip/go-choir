package selfdevprotocol

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRematerializeFromRequestRefusesPinCheckout(t *testing.T) {
	checkpoint, _, err := CheckpointFromRequest(testCheckpointRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	request := testRematerializeRequest(t, checkpoint)
	request.Method = RematerializeMethodPinCheckout
	if err := RematerializeFromRequest(request); err == nil {
		t.Fatal("pin checkout was accepted as a completion method")
	}
}

func TestRematerializeFromRequestRefusesOriginalWorkspaceAccess(t *testing.T) {
	checkpoint, _, err := CheckpointFromRequest(testCheckpointRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	request := testRematerializeRequest(t, checkpoint)
	if err := RematerializeFromRequest(request); err != nil {
		t.Fatalf("valid tape rematerialize refused: %v", err)
	}

	samePath := request
	samePath.StagedMarkerPath = request.OriginalMarkerPath
	if err := RematerializeFromRequest(samePath); err == nil {
		t.Fatal("rematerialize accepted reconstructing into the original marker")
	}

	nested := request
	nested.StagedWorkspacePath = filepath.Join(request.OriginalWorkspacePath, "copy")
	if err := RematerializeFromRequest(nested); err == nil {
		t.Fatal("rematerialize accepted a staged path inside the original workspace")
	}
}

func TestWitnessContentMatchesComparesRootAndTables(t *testing.T) {
	want := testCheckpointRequest(t).VMLocalContentWitness
	got := want
	if err := WitnessContentMatches(got, want); err != nil {
		t.Fatalf("equal witnesses did not match: %v", err)
	}
	other := testWitnessV3Tables("choir", map[string]string{"agents": strings.Repeat("c", 64)})
	if err := WitnessContentMatches(other, want); err == nil || !strings.Contains(err.Error(), "agents") {
		t.Fatalf("a table digest mismatch was accepted or not named: %v", err)
	}
	got.ContentRoot = strings.Repeat("0", 64)
	if err := WitnessContentMatches(got, want); err == nil {
		t.Fatal("content root mismatch was accepted")
	}
}

func testRematerializeRequest(t *testing.T, checkpoint Checkpoint) RematerializeRequest {
	t.Helper()
	return RematerializeRequest{
		ComputerID:            checkpoint.Request.ComputerID,
		Checkpoint:            checkpoint,
		Method:                RematerializeMethodTape,
		OriginalMarkerPath:    "/var/lib/go-choir/vm-state/original/runtime.db",
		OriginalWorkspacePath: "/var/lib/go-choir/vm-state/original/runtime.texture",
		StagedMarkerPath:      "/var/lib/go-choir/vm-state/staged/runtime.db",
		StagedWorkspacePath:   "/var/lib/go-choir/vm-state/staged/runtime.texture",
	}
}
