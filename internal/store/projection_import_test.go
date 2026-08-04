package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func testProjectionImportManifest(t *testing.T) ProjectionImportV1 {
	t.Helper()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectionImportSnapshot{
		Trajectory:       types.TrajectoryRecord{TrajectoryID: "trajectory-import", OwnerID: "owner-import", ComputerID: "computer-supervision", Kind: types.TrajectoryKindDocument, Status: types.TrajectorySettled, SubjectRefs: map[string]string{"doc_id": "document-import"}, LifecycleVersion: 7, ReducerSeq: 7, TerminalArtifactHeadRef: "revision-import", CreatedAt: now, UpdatedAt: now},
		IntentRevisionID: "revision-import",
		WorkItems:        []types.WorkItemRecord{{WorkItemID: "work-import", TrajectoryID: "trajectory-import", OwnerID: "owner-import", ComputerID: "computer-supervision", Status: types.WorkItemCompleted, CreatedAt: now, UpdatedAt: now}},
		Agents:           []types.AgentRecord{{AgentID: "texture:document-import", OwnerID: "owner-import", ComputerID: "computer-supervision", SandboxID: "computer-supervision", Profile: "texture", Role: "texture", ChannelID: "document-import", CreatedAt: now, UpdatedAt: now}},
		Updates:          []types.CoagentSourcePacket{},
		Document:         types.Document{DocID: "document-import", OwnerID: "owner-import", ComputerID: "computer-supervision", TrajectoryID: "trajectory-import", Title: "Imported document", CurrentRevisionID: "revision-import", CreatedAt: now, UpdatedAt: now},
		HeadRevision:     types.Revision{RevisionID: "revision-import", DocID: "document-import", OwnerID: "owner-import", ComputerID: "computer-supervision", AuthorKind: types.AuthorUser, AuthorLabel: "owner-import", VersionNumber: 3, Content: "imported content", ParentRevisionID: "revision-parent", TrajectoryID: "trajectory-import", RevisionHash: storeTestDigest('a'), CreatedAt: now},
		SourceEntities:   []TextureSourceEntityGraphRecord{}, SourceRefs: []TextureSourceRefGraphRecord{},
	}
	sortProjectionImportSnapshot(&snapshot)
	objects, edges, err := buildProjectionImportObjects(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := projectionImportDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ProjectionImportV1{Schema: ProjectionImportV1Schema, ReducerVersion: computerevent.SupervisionReducerV1, OwnerID: snapshot.Trajectory.OwnerID, ComputerID: snapshot.Trajectory.ComputerID, SourceDoltCommit: "dolt-import-commit", SourceProjectionDigest: sourceDigest, LegacyLifecycleWatermark: 7, CutoverBarrier: ProjectionImportCutoverBarrier{WritesDisabledAt: now, ActiveRunIDs: []string{}, ActiveAttemptIDs: []string{}, PendingDeliveryIDs: []string{}, SlotClaimIDs: []string{}, AgentMutationIDs: []string{}, CommandReceiptIDs: []string{}, QuiescenceReceiptRef: "artifact:sha256:" + storeTestDigest('c'), DrainReceiptRefs: []string{"artifact:sha256:" + storeTestDigest('d')}}, Objects: objects, Edges: edges, ExplicitRefusals: []ProjectionImportRefusal{}, Snapshot: snapshot}
	manifest.ProjectionDigest, err = projectionImportManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.SourceProjectionDigest, err = projectionImportDigest(manifest.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ProjectionDigest, err = projectionImportManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestProjectionImportReducerMaterializesFieldEquivalentObjects(t *testing.T) {
	productStore := openTestStore(t)
	manifest := testProjectionImportManifest(t)
	normalized := manifest.Snapshot
	normalized.WorkItems = append([]types.WorkItemRecord(nil), normalized.WorkItems...)
	normalized.Agents = append([]types.AgentRecord(nil), normalized.Agents...)
	normalized.Updates = append([]types.CoagentSourcePacket(nil), normalized.Updates...)
	normalized.SourceEntities = append([]TextureSourceEntityGraphRecord(nil), normalized.SourceEntities...)
	normalized.SourceRefs = append([]TextureSourceRefGraphRecord(nil), normalized.SourceRefs...)
	sortProjectionImportSnapshot(&normalized)
	normalizedDigest, err := projectionImportDigest(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if normalizedDigest != manifest.SourceProjectionDigest {
		t.Fatalf("fixture source digest = %s, normalized = %s", manifest.SourceProjectionDigest, normalizedDigest)
	}
	transaction, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := productStore.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.applySupervisionProjectionTx(context.Background(), tx, 1, computerevent.ZeroHead, storeTestDigest('e'), time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC), transaction); err != nil {
		_ = tx.Rollback()
		t.Fatalf("apply import: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	document, err := productStore.GetLifecycleDocument(context.Background(), manifest.OwnerID, manifest.ComputerID, manifest.Snapshot.Document.DocID)
	if err != nil {
		t.Fatal(err)
	}
	if document != manifest.Snapshot.Document {
		t.Fatalf("document differs after import: got=%+v want=%+v", document, manifest.Snapshot.Document)
	}
	revision, err := productStore.GetLifecycleRevision(context.Background(), manifest.OwnerID, manifest.ComputerID, manifest.Snapshot.HeadRevision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Content != manifest.Snapshot.HeadRevision.Content || revision.ParentRevisionID != manifest.Snapshot.HeadRevision.ParentRevisionID || revision.RevisionHash != manifest.Snapshot.HeadRevision.RevisionHash {
		t.Fatalf("revision differs after import: got=%+v want=%+v", revision, manifest.Snapshot.HeadRevision)
	}
	if err := transaction.Validate(); err != nil {
		t.Fatalf("identical import replay transaction changed: %v", err)
	}

	changed := manifest
	changed.SourceDoltCommit = "changed-dolt-commit"
	changed.ProjectionDigest, err = projectionImportManifestDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedTransaction, err := NewProjectionImportTransaction(changed, "artifact:sha256:"+changed.ProjectionDigest, transaction.CommandID)
	if err != nil {
		t.Fatal(err)
	}
	if changedTransaction.CommandDigest == transaction.CommandDigest {
		t.Fatal("changed import digest retained the original command digest")
	}
}

func TestProjectionImportBuilderRefusesActiveLegacyWorkBeforeMutation(t *testing.T) {
	productStore := openTestStore(t)
	start := seedHistoricalLifecycleFixture(t, productStore)
	_, err := productStore.BuildProjectionImportV1(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, ProjectionImportBuildOptions{
		SourceDoltCommit: "dolt-import-commit", WritesDisabledAt: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
		QuiescenceReceiptRef: "artifact:sha256:" + storeTestDigest('a'),
		DrainReceiptRefs:     []string{"artifact:sha256:" + storeTestDigest('b')},
	})
	if err == nil || !strings.Contains(err.Error(), "active attempts") {
		t.Fatalf("active legacy trajectory import error = %v, want active attempts refusal", err)
	}
	if _, err := productStore.GetSupervisionProjectionSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("precondition failure mutated supervision projection: %v", err)
	}
}

func rehashProjectionImportManifest(t *testing.T, manifest *ProjectionImportV1) {
	t.Helper()
	snapshot := manifest.Snapshot
	sortProjectionImportSnapshot(&snapshot)
	var err error
	manifest.SourceProjectionDigest, err = projectionImportDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Objects, manifest.Edges, err = buildProjectionImportObjects(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ProjectionDigest, err = projectionImportManifestDigest(*manifest)
	if err != nil {
		t.Fatal(err)
	}
}

func rehashProjectionImportDigest(t *testing.T, manifest *ProjectionImportV1) {
	t.Helper()
	var err error
	manifest.ProjectionDigest, err = projectionImportManifestDigest(*manifest)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProjectionImportManifestRejectsRehashedNestedScopeAndLinkage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProjectionImportV1)
	}{
		{
			name: "cross owner assignment",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.WorkItems[0].OwnerID = "other-owner"
			},
		},
		{
			name: "wrong revision computer",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.HeadRevision.ComputerID = "other-computer"
			},
		},
		{
			name: "wrong document trajectory",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.Document.TrajectoryID = "other-trajectory"
			},
		},
		{
			name: "wrong document revision linkage",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.Document.CurrentRevisionID = "other-revision"
			},
		},
		{
			name: "cross owner agent",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.Agents[0].OwnerID = "other-owner"
			},
		},
		{
			name: "cross owner update",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.Updates = []types.CoagentSourcePacket{{
					UpdateID: "update-import", OwnerID: "other-owner", ComputerID: manifest.ComputerID,
					AgentID: manifest.Snapshot.Agents[0].AgentID, TargetAgentID: manifest.Snapshot.Agents[0].AgentID,
					TrajectoryID: manifest.Snapshot.Trajectory.TrajectoryID, WorkItemID: manifest.Snapshot.WorkItems[0].WorkItemID,
				}}
			},
		},
		{
			name: "cross owner source entity",
			mutate: func(manifest *ProjectionImportV1) {
				manifest.Snapshot.SourceEntities = []TextureSourceEntityGraphRecord{{CanonicalID: "source-entity-import", OwnerID: "other-owner"}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := testProjectionImportManifest(t)
			test.mutate(&manifest)
			rehashProjectionImportManifest(t, &manifest)
			if _, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command"); err == nil {
				t.Fatal("self-consistently rehashed invalid nested snapshot was accepted")
			}
		})
	}
}

func TestProjectionImportManifestRejectsAlteredGraphAndDigest(t *testing.T) {
	t.Run("altered object", func(t *testing.T) {
		manifest := testProjectionImportManifest(t)
		manifest.Objects[0].Body = []byte(`{"tampered":true}`)
		manifest.Objects[0].ContentHash = computerevent.DigestBytes(manifest.Objects[0].Body)
		rehashProjectionImportDigest(t, &manifest)
		if _, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command"); err == nil {
			t.Fatal("altered canonical object was accepted")
		}
	})
	t.Run("altered edge", func(t *testing.T) {
		manifest := testProjectionImportManifest(t)
		manifest.Edges[0].ToID = "other-node"
		hash, err := projectionImportDigest(struct{ Kind, From, To string }{manifest.Edges[0].Kind, manifest.Edges[0].FromID, manifest.Edges[0].ToID})
		if err != nil {
			t.Fatal(err)
		}
		manifest.Edges[0].ContentHash = hash
		rehashProjectionImportDigest(t, &manifest)
		if _, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command"); err == nil {
			t.Fatal("altered canonical edge was accepted")
		}
	})
	t.Run("duplicate canonical id", func(t *testing.T) {
		manifest := testProjectionImportManifest(t)
		manifest.Snapshot.Agents = append(manifest.Snapshot.Agents, manifest.Snapshot.Agents[0])
		rehashProjectionImportManifest(t, &manifest)
		if _, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command"); err == nil {
			t.Fatal("duplicate canonical id was accepted")
		}
	})
	t.Run("wrong source projection digest", func(t *testing.T) {
		manifest := testProjectionImportManifest(t)
		manifest.SourceProjectionDigest = storeTestDigest('z')
		rehashProjectionImportDigest(t, &manifest)
		if _, err := NewProjectionImportTransaction(manifest, "artifact:sha256:"+manifest.ProjectionDigest, "import-command"); err == nil {
			t.Fatal("wrong source projection digest was accepted")
		}
	})
}

func TestProjectionImportSchemaConformsToManifestWireShape(t *testing.T) {
	manifest := testProjectionImportManifest(t)
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(manifestJSON, &wire); err != nil {
		t.Fatal(err)
	}
	schemaJSON, err := os.ReadFile(filepath.Join("..", "..", "specs", "projection_import_v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	for key := range wire {
		if _, ok := properties[key]; !ok {
			t.Fatalf("manifest wire field %q is absent from schema", key)
		}
	}
	snapshotSchema := properties["snapshot"].(map[string]any)
	required := map[string]bool{}
	for _, key := range snapshotSchema["required"].([]any) {
		required[key.(string)] = true
	}
	if !required["intent_revision_id"] {
		t.Fatal("snapshot intent_revision_id is absent from schema requirements")
	}
	defs := schema["$defs"].(map[string]any)
	edgeKinds := map[string]bool{}
	for _, kind := range defs["import_edge"].(map[string]any)["properties"].(map[string]any)["kind"].(map[string]any)["enum"].([]any) {
		edgeKinds[kind.(string)] = true
	}
	for _, edge := range manifest.Edges {
		if !edgeKinds[edge.Kind] {
			t.Fatalf("manifest edge kind %q is absent from schema", edge.Kind)
		}
	}
	artifactAlternatives := defs["artifact_ref"].(map[string]any)["oneOf"].([]any)
	if len(artifactAlternatives) != 2 {
		t.Fatalf("artifact ref schema alternatives = %d, want pinned and placeholder refs", len(artifactAlternatives))
	}
}
