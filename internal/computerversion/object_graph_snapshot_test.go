package computerversion

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestObjectGraphSnapshotObservationSetEmitsDeterministicObjectGraphHead(t *testing.T) {
	version := objectGraphSnapshotVersion()
	snapshot := objectGraphSnapshotFixture(t)
	reordered := ObjectGraphSnapshot{
		Objects: []objectgraph.Object{snapshot.Objects[2], snapshot.Objects[0], snapshot.Objects[1]},
		Edges:   []objectgraph.Edge{snapshot.Edges[1], snapshot.Edges[0]},
	}

	observations, err := snapshot.ObservationSet("objectgraph fixture", version)
	if err != nil {
		t.Fatalf("objectgraph observations: %v", err)
	}
	reorderedObservations, err := reordered.ObservationSet("objectgraph fixture", version)
	if err != nil {
		t.Fatalf("reordered objectgraph observations: %v", err)
	}

	assertObservationBundleKinds(t, observations.Required, []ObservationKind{ObservationObjectGraphHead})
	if len(observations.Observations) != 1 {
		t.Fatalf("observations = %#v, want one object graph head", observations.Observations)
	}
	if len(reorderedObservations.Observations) != 1 {
		t.Fatalf("reordered observations = %#v, want one object graph head", reorderedObservations.Observations)
	}
	observation := observations.Observations[0]
	if observation.Kind != ObservationObjectGraphHead || observation.Key != "objectgraph:head" {
		t.Fatalf("observation = %#v, want object_graph_head objectgraph:head", observation)
	}
	if !strings.HasPrefix(decodeObjectGraphHeadPayload(t, observation.Value).Head, "sha256:") {
		t.Fatalf("object graph head observation value = %s, want sha256 head", observation.Value)
	}
	if observation.Value != reorderedObservations.Observations[0].Value {
		t.Fatalf("object graph head changed when object/edge input order changed:\noriginal  %s\nreordered %s", observation.Value, reorderedObservations.Observations[0].Value)
	}

	payload := decodeObjectGraphHeadPayload(t, observation.Value)
	if payload.ObjectCount != 3 || payload.EdgeCount != 2 {
		t.Fatalf("object graph counts = objects:%d edges:%d, want objects:3 edges:2", payload.ObjectCount, payload.EdgeCount)
	}
	for i := 1; i < len(payload.Objects); i++ {
		if payload.Objects[i-1].CanonicalID > payload.Objects[i].CanonicalID {
			t.Fatalf("objects are not canonical-id sorted in head payload: %#v", payload.Objects)
		}
	}
	for i := 1; i < len(payload.Edges); i++ {
		if payload.Edges[i-1].EdgeID > payload.Edges[i].EdgeID {
			t.Fatalf("edges are not edge-id sorted in head payload: %#v", payload.Edges)
		}
	}
}

func TestObjectGraphSnapshotRejectsInvalidTypedGraph(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(ObjectGraphSnapshot) ObjectGraphSnapshot
		wantErr string
	}{
		{
			name: "content hash mismatch",
			mutate: func(snapshot ObjectGraphSnapshot) ObjectGraphSnapshot {
				snapshot.Objects[0].ContentHash = "sha256:wrong"
				return snapshot
			},
			wantErr: "content hash mismatch",
		},
		{
			name: "missing edge endpoint",
			mutate: func(snapshot ObjectGraphSnapshot) ObjectGraphSnapshot {
				snapshot.Edges[0].ToID = snapshot.Edges[0].ToID + "-missing"
				return snapshot
			},
			wantErr: "references missing to object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := tt.mutate(objectGraphSnapshotFixture(t))

			err := snapshot.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func objectGraphSnapshotVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:objectgraph-snapshot", ArtifactProgramRef: "tape:org/objectgraph-snapshot@fixture"}
}

func objectGraphSnapshotFixture(t *testing.T) ObjectGraphSnapshot {
	t.Helper()
	return objectGraphSnapshotFixtureForVersion(t, objectGraphSnapshotVersion())
}

func objectGraphSnapshotFixtureForVersion(t *testing.T, version ComputerVersion) ObjectGraphSnapshot {
	t.Helper()

	ownerID := "objectgraph-test-owner"
	computerID := "objectgraph-test-computer"
	versionID := string(version.CodeRef)
	agent := objectGraphObjectFixture(t, objectgraph.ObjectKind("choir.agent"), ownerID, computerID, versionID, []byte("objectgraph agent body"), json.RawMessage(`{"role":"agent","stable":true}`))
	run := objectGraphObjectFixture(t, objectgraph.ObjectKind("choir.run"), ownerID, computerID, versionID, []byte("objectgraph run body"), json.RawMessage(`{"role":"run","stable":true}`))
	artifact := objectGraphObjectFixture(t, objectgraph.ObjectKind("choir.artifact"), ownerID, computerID, versionID, []byte("objectgraph artifact body"), json.RawMessage(`{"role":"artifact","stable":true}`))

	return ObjectGraphSnapshot{
		Objects: []objectgraph.Object{agent, run, artifact},
		Edges: []objectgraph.Edge{
			objectGraphEdgeFixture(t, agent.CanonicalID, run.CanonicalID, objectgraph.EdgeKind("records"), json.RawMessage(`{"relation":"records"}`)),
			objectGraphEdgeFixture(t, run.CanonicalID, artifact.CanonicalID, objectgraph.EdgeKind("produces"), json.RawMessage(`{"relation":"produces"}`)),
		},
	}
}

func objectGraphObjectFixture(t *testing.T, kind objectgraph.ObjectKind, ownerID, computerID, versionID string, body []byte, metadata json.RawMessage) objectgraph.Object {
	t.Helper()

	contentHash := objectgraph.ContentHash(kind, body, metadata)
	canonicalID, err := objectgraph.BuildCanonicalID(kind, ownerID, objectgraph.StableSuffixFromContent(contentHash))
	if err != nil {
		t.Fatalf("build object canonical id: %v", err)
	}
	return objectgraph.Object{
		CanonicalID: canonicalID,
		ObjectKind:  kind,
		OwnerID:     ownerID,
		ComputerID:  computerID,
		VersionID:   versionID,
		ContentHash: contentHash,
		Body:        body,
		Metadata:    metadata,
	}
}

func objectGraphEdgeFixture(t *testing.T, fromID, toID string, kind objectgraph.EdgeKind, metadata json.RawMessage) objectgraph.Edge {
	t.Helper()

	edgeID, err := objectgraph.BuildEdgeID(fromID, toID, kind, metadata)
	if err != nil {
		t.Fatalf("build object graph edge id: %v", err)
	}
	return objectgraph.Edge{EdgeID: edgeID, FromID: fromID, ToID: toID, Kind: kind, Metadata: metadata}
}

type objectGraphHeadTestPayload struct {
	Head        string `json:"head"`
	ObjectCount int    `json:"object_count"`
	EdgeCount   int    `json:"edge_count"`
	Objects     []struct {
		CanonicalID string `json:"canonical_id"`
	} `json:"objects"`
	Edges []struct {
		EdgeID string `json:"edge_id"`
	} `json:"edges"`
}

func decodeObjectGraphHeadPayload(t *testing.T, value string) objectGraphHeadTestPayload {
	t.Helper()

	var payload objectGraphHeadTestPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode object graph head payload: %v\n%s", err, value)
	}
	return payload
}

func assertObservationBundleKinds(t *testing.T, got, want []ObservationKind) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("observation kinds = %#v, want %#v", got, want)
	}
	counts := make(map[ObservationKind]int, len(want))
	for _, kind := range want {
		counts[kind]++
	}
	for _, kind := range got {
		counts[kind]--
	}
	for kind, count := range counts {
		if count != 0 {
			t.Fatalf("observation kinds = %#v, want %#v (difference %s=%d)", got, want, kind, count)
		}
	}
}
