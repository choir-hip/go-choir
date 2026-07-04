package computerversion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDoltHeadSnapshotObservationSetEmitsObjectGraphLinkedDoltHead(t *testing.T) {
	version := doltHeadSnapshotVersion()
	objectGraph := objectGraphSnapshotFixtureForVersion(t, version)
	objectGraphPayload := decodeObjectGraphHeadPayload(t, mustObjectGraphCanonicalHead(t, objectGraph))
	snapshot := DoltHeadSnapshot{
		RepoRoot:    t.TempDir(),
		Database:    " objectgraph ",
		CommitHash:  " dolt-commit-123 ",
		ObjectGraph: &objectGraph,
		Derivation:  "local fixture",
	}

	observations, err := snapshot.ObservationSet("dolt fixture", version)
	if err != nil {
		t.Fatalf("dolt head observations: %v", err)
	}

	if observations.Name != "dolt fixture" {
		t.Fatalf("name = %q, want dolt fixture", observations.Name)
	}
	if observations.Version != version {
		t.Fatalf("version = %#v, want %#v", observations.Version, version)
	}
	assertObservationBundleKinds(t, observations.Required, []ObservationKind{ObservationDoltHead})
	if len(observations.Observations) != 1 {
		t.Fatalf("observations = %#v, want one dolt_head", observations.Observations)
	}
	observation := observations.Observations[0]
	if observation.Kind != ObservationDoltHead || observation.Key != "dolt:objectgraph:head" {
		t.Fatalf("observation = %#v, want dolt_head dolt:objectgraph:head", observation)
	}
	payload := decodeDoltHeadPayload(t, observation.Value)
	if payload.Database != "objectgraph" || payload.CommitHash != "dolt-commit-123" {
		t.Fatalf("dolt head identity = database:%q commit:%q, want objectgraph/dolt-commit-123", payload.Database, payload.CommitHash)
	}
	if payload.ObjectGraphHead != objectGraphPayload.Head || payload.ObjectCount != objectGraphPayload.ObjectCount || payload.EdgeCount != objectGraphPayload.EdgeCount {
		t.Fatalf("dolt head objectgraph link = head:%q objects:%d edges:%d, want head:%q objects:%d edges:%d", payload.ObjectGraphHead, payload.ObjectCount, payload.EdgeCount, objectGraphPayload.Head, objectGraphPayload.ObjectCount, objectGraphPayload.EdgeCount)
	}
}

func TestDoltHeadSnapshotRejectsMissingCommitHashAndProductionState(t *testing.T) {
	base := DoltHeadSnapshot{
		RepoRoot:   t.TempDir(),
		Database:   "objectgraph",
		CommitHash: "dolt-commit-123",
	}
	tests := []struct {
		name    string
		mutate  func(DoltHeadSnapshot) DoltHeadSnapshot
		wantErr string
	}{
		{
			name: "missing commit hash",
			mutate: func(snapshot DoltHeadSnapshot) DoltHeadSnapshot {
				snapshot.CommitHash = "  "
				return snapshot
			},
			wantErr: "commit hash is required",
		},
		{
			name: "contains production",
			mutate: func(snapshot DoltHeadSnapshot) DoltHeadSnapshot {
				snapshot.ContainsProduction = true
				return snapshot
			},
			wantErr: "production state is not admissible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func doltHeadSnapshotVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:dolt-head-snapshot", ArtifactProgramRef: "tape:org/dolt-head-snapshot@fixture"}
}

type doltHeadTestPayload struct {
	Database        string `json:"database"`
	CommitHash      string `json:"commit_hash"`
	ObjectGraphHead string `json:"object_graph_head"`
	ObjectCount     int    `json:"object_count"`
	EdgeCount       int    `json:"edge_count"`
	Derivation      string `json:"derivation"`
}

func decodeDoltHeadPayload(t *testing.T, value string) doltHeadTestPayload {
	t.Helper()

	var payload doltHeadTestPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode dolt head payload: %v\n%s", err, value)
	}
	return payload
}

func mustObjectGraphCanonicalHead(t *testing.T, snapshot ObjectGraphSnapshot) string {
	t.Helper()

	value, err := snapshot.CanonicalHead()
	if err != nil {
		t.Fatalf("object graph canonical head: %v", err)
	}
	return value
}
