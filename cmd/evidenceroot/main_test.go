package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestRunProvisionsLocalCandidateEvidenceRootAndReportsSelfChecks(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--root", root,
		"--name", "evidenceroot-test",
		"--code-ref", "git:evidenceroot-test",
		"--artifact-program-ref", "tape:org/evidenceroot-test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	out := decodeOutput(t, stdout.Bytes())
	rootAbs := mustAbs(t, root)
	manifest := out.Manifest
	if manifest.ID != "evidenceroot-test" {
		t.Fatalf("manifest id = %q, want evidenceroot-test", manifest.ID)
	}
	if manifest.RootPath != rootAbs {
		t.Fatalf("manifest root_path = %q, want %q", manifest.RootPath, rootAbs)
	}
	if manifest.Source != computerversion.EvidenceRootSourceLocalCandidate {
		t.Fatalf("manifest source = %q, want %q", manifest.Source, computerversion.EvidenceRootSourceLocalCandidate)
	}
	if !manifest.AuthorizedForSampling || manifest.ContainsProduction || manifest.TouchesDeployedRoute {
		t.Fatalf("manifest admission flags = authorized:%v contains_production:%v touches_deployed_route:%v", manifest.AuthorizedForSampling, manifest.ContainsProduction, manifest.TouchesDeployedRoute)
	}
	if !reflect.DeepEqual(manifest.EvidenceRefs, []string{"evidenceroot-test:manifest", "evidenceroot-test:observation"}) {
		t.Fatalf("manifest evidence_refs = %#v, want manifest and observation refs", manifest.EvidenceRefs)
	}
	wantVersion := computerversion.ComputerVersion{
		CodeRef:            "git:evidenceroot-test",
		ArtifactProgramRef: "tape:org/evidenceroot-test",
	}
	if manifest.Fixture.Version != wantVersion {
		t.Fatalf("fixture version = %#v, want %#v", manifest.Fixture.Version, wantVersion)
	}
	assertManifestObjectGraphFixture(t, manifest.Fixture.ObjectGraph)
	assertManifestDoltHeadFixture(t, rootAbs, manifest.Fixture.DoltHead, manifest.Fixture.ObjectGraph)

	assertRegularFile(t, filepath.Join(rootAbs, "base.sqlite"))
	assertRegularFile(t, filepath.Join(rootAbs, "auth.sqlite"))
	assertDirectory(t, filepath.Join(rootAbs, "blobs"))
	assertDirectory(t, filepath.Join(rootAbs, "vm", "persist"))
	assertRegularFile(t, filepath.Join(rootAbs, "vm", "data.img"))
	assertRegularFile(t, filepath.Join(rootAbs, "vm", "vmlinux"))
	assertRegularFile(t, filepath.Join(rootAbs, "vm", "rootfs.ext4"))
	if out.BaseFixture.JournalPath != filepath.Join(rootAbs, "base.sqlite") || out.BaseFixture.BlobRoot != filepath.Join(rootAbs, "blobs") || out.BaseFixture.AuthDBPath != filepath.Join(rootAbs, "auth.sqlite") {
		t.Fatalf("base fixture paths = %#v, want paths under %s", out.BaseFixture, rootAbs)
	}
	if out.BaseFixture.ItemID != "base_item_evidenceroot_fixture" {
		t.Fatalf("base fixture item_id = %q, want base_item_evidenceroot_fixture", out.BaseFixture.ItemID)
	}
	assertBaseFixtureObserved(t, out.Observation, out.BaseFixture)

	fixture, err := manifest.ProductFixtureRoot()
	if err != nil {
		t.Fatalf("manifest did not admit ProductFixtureRoot: %v", err)
	}
	reobserved, err := fixture.ObservationSet(context.Background(), manifest.ID)
	if err != nil {
		t.Fatalf("reobserve ProductFixtureRoot: %v", err)
	}
	if !reflect.DeepEqual(out.Observation, reobserved) {
		t.Fatalf("observation was not produced by ProductFixtureRoot:\n got %#v\nwant %#v", out.Observation, reobserved)
	}
	if out.Observation.Name != "evidenceroot-test" {
		t.Fatalf("observation name = %q, want evidenceroot-test", out.Observation.Name)
	}
	if out.Observation.Version != wantVersion {
		t.Fatalf("observation version = %#v, want %#v", out.Observation.Version, wantVersion)
	}
	wantKinds := []computerversion.ObservationKind{
		computerversion.ObservationBlobSet,
		computerversion.ObservationDoltHead,
		computerversion.ObservationFileManifest,
		computerversion.ObservationObjectGraphHead,
		computerversion.ObservationPromotionCertificate,
		computerversion.ObservationVMStateManifest,
	}
	assertObservationKinds(t, out.Observation.Required, wantKinds)
	assertObservationKinds(t, observationKinds(out.Observation.Observations), wantKinds)
	assertObservationObjectGraphHead(t, out.Observation, manifest.Fixture.ObjectGraph)
	assertObservationDoltHead(t, out.Observation, manifest.Fixture.DoltHead, manifest.Fixture.ObjectGraph)

	if out.SelfCheck.Status != computerversion.EquivalenceEquivalent || len(out.SelfCheck.Differences) != 0 || len(out.SelfCheck.Unsupported) != 0 {
		t.Fatalf("self_check = %#v, want clean equivalent", out.SelfCheck)
	}
	if out.SeededMismatch.Status != computerversion.EquivalenceNotEquivalent {
		t.Fatalf("seeded_mismatch status = %q, want %q; result=%#v", out.SeededMismatch.Status, computerversion.EquivalenceNotEquivalent, out.SeededMismatch)
	}
	if len(out.SeededMismatch.Differences) != 1 {
		t.Fatalf("seeded_mismatch differences = %#v, want one VM manifest mismatch", out.SeededMismatch.Differences)
	}
	diff := out.SeededMismatch.Differences[0]
	if diff.Kind != computerversion.ObservationVMStateManifest || diff.Key != "vmmanager:evidenceroot-test-vm" || diff.Reason != "observation values differ" {
		t.Fatalf("seeded_mismatch difference = %#v, want vm_state_manifest value mismatch", diff)
	}
	if diff.Left == "" || diff.Right == "" || diff.Left == diff.Right {
		t.Fatalf("seeded_mismatch did not preserve distinct VM manifest values: %#v", diff)
	}
}

func TestRunRejectsMissingRequiredFlagsBeforeJSON(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "missing root",
			args: []string{
				"--code-ref", "git:evidenceroot-test",
				"--artifact-program-ref", "tape:org/evidenceroot-test",
			},
			wantStderr: "--root is required",
		},
		{
			name: "missing code ref",
			args: []string{
				"--root", filepath.Join(root, "missing-code-ref"),
				"--artifact-program-ref", "tape:org/evidenceroot-test",
			},
			wantStderr: "--code-ref is required",
		},
		{
			name: "missing artifact program ref",
			args: []string{
				"--root", filepath.Join(root, "missing-artifact-program-ref"),
				"--code-ref", "git:evidenceroot-test",
			},
			wantStderr: "--artifact-program-ref is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunRejectsNonEmptyRootWithoutOverwritingExistingContents(t *testing.T) {
	root := t.TempDir()
	sentinelPath := filepath.Join(root, "sentinel.txt")
	sentinelContent := []byte("do not overwrite this evidence root\n")
	if err := os.WriteFile(sentinelPath, sentinelContent, 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--root", root,
		"--name", "non-empty-root-test",
		"--code-ref", "git:evidenceroot-test",
		"--artifact-program-ref", "tape:org/evidenceroot-test",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "must be empty") {
		t.Fatalf("stderr = %q, want non-empty root error", stderr.String())
	}
	got, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("read sentinel after rejected run: %v", err)
	}
	if !bytes.Equal(got, sentinelContent) {
		t.Fatalf("sentinel content = %q, want unchanged %q", got, sentinelContent)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root after rejected run: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel.txt" {
		t.Fatalf("root entries after rejected run = %#v, want only sentinel.txt", entries)
	}
}

func decodeOutput(t *testing.T, data []byte) output {
	t.Helper()
	var out output
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode evidenceroot output: %v\n%s", err, string(data))
	}
	return out
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs %s: %v", path, err)
	}
	return abs
}

func assertRegularFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s mode = %v, want regular file", path, info.Mode())
	}
}

func assertDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat directory %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s mode = %v, want directory", path, info.Mode())
	}
}

func assertBaseFixtureObserved(t *testing.T, set computerversion.ObservationSet, fixture baseFixtureOutput) {
	t.Helper()
	var sawItem, sawBlob bool
	for _, observation := range set.Observations {
		switch {
		case observation.Kind == computerversion.ObservationFileManifest && observation.Key == string(fixture.ItemID):
			sawItem = strings.Contains(observation.Value, string(fixture.BlobRef)) && strings.Contains(observation.Value, fixture.SHA256)
		case observation.Kind == computerversion.ObservationBlobSet && observation.Key == string(fixture.BlobRef):
			sawBlob = strings.Contains(observation.Value, fixture.SHA256)
		}
	}
	if !sawItem || !sawBlob {
		t.Fatalf("missing base fixture observations: sawItem=%v sawBlob=%v fixture=%#v set=%#v", sawItem, sawBlob, fixture, set.Observations)
	}
}

func assertManifestObjectGraphFixture(t *testing.T, snapshot *computerversion.ObjectGraphSnapshot) {
	t.Helper()
	if snapshot == nil {
		t.Fatal("manifest fixture object_graph = nil, want object graph snapshot")
	}
	if len(snapshot.Objects) != 2 || len(snapshot.Edges) != 1 {
		t.Fatalf("manifest fixture object_graph = %d objects and %d edges, want 2 objects and 1 edge", len(snapshot.Objects), len(snapshot.Edges))
	}
	objectIDs := make(map[string]struct{}, len(snapshot.Objects))
	for _, object := range snapshot.Objects {
		if object.CanonicalID == "" {
			t.Fatalf("manifest fixture object has empty canonical id: %#v", object)
		}
		objectIDs[object.CanonicalID] = struct{}{}
	}
	edge := snapshot.Edges[0]
	if _, ok := objectIDs[edge.FromID]; !ok {
		t.Fatalf("manifest fixture object_graph edge from_id = %q, want one of %#v", edge.FromID, objectIDs)
	}
	if _, ok := objectIDs[edge.ToID]; !ok {
		t.Fatalf("manifest fixture object_graph edge to_id = %q, want one of %#v", edge.ToID, objectIDs)
	}
}

func assertManifestDoltHeadFixture(t *testing.T, root string, doltHead *computerversion.DoltHeadSnapshot, snapshot *computerversion.ObjectGraphSnapshot) {
	t.Helper()
	if doltHead == nil {
		t.Fatal("manifest fixture dolt_head = nil, want local Dolt objectgraph head")
	}
	rel, err := filepath.Rel(root, doltHead.RepoRoot)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("manifest fixture dolt_head repo_root = %q, want path under %s", doltHead.RepoRoot, root)
	}
	assertDirectory(t, doltHead.RepoRoot)
	if doltHead.Database != "objectgraph" {
		t.Fatalf("manifest fixture dolt_head database = %q, want objectgraph", doltHead.Database)
	}
	if strings.TrimSpace(doltHead.CommitHash) == "" {
		t.Fatalf("manifest fixture dolt_head commit_hash is empty: %#v", doltHead)
	}
	if doltHead.ContainsProduction {
		t.Fatalf("manifest fixture dolt_head contains production state: %#v", doltHead)
	}
	if doltHead.ObjectGraph == nil {
		t.Fatal("manifest fixture dolt_head object_graph = nil, want embedded objectgraph snapshot")
	}
	gotHead, err := doltHead.ObjectGraph.CanonicalHead()
	if err != nil {
		t.Fatalf("manifest fixture dolt_head object_graph canonical head: %v", err)
	}
	wantHead, err := snapshot.CanonicalHead()
	if err != nil {
		t.Fatalf("manifest fixture object_graph canonical head: %v", err)
	}
	if gotHead != wantHead {
		t.Fatalf("manifest fixture dolt_head object_graph head = %s, want fixture object_graph head %s", gotHead, wantHead)
	}
}

func assertObservationObjectGraphHead(t *testing.T, set computerversion.ObservationSet, snapshot *computerversion.ObjectGraphSnapshot) {
	t.Helper()
	if snapshot == nil {
		t.Fatal("object graph snapshot is nil")
	}
	for _, observation := range set.Observations {
		if observation.Kind != computerversion.ObservationObjectGraphHead {
			continue
		}
		if observation.Key != "objectgraph:head" {
			t.Fatalf("object graph observation key = %q, want objectgraph:head", observation.Key)
		}
		var payload struct {
			ObjectCount int `json:"object_count"`
			EdgeCount   int `json:"edge_count"`
		}
		if err := json.Unmarshal([]byte(observation.Value), &payload); err != nil {
			t.Fatalf("decode object graph head observation: %v\n%s", err, observation.Value)
		}
		if payload.ObjectCount != len(snapshot.Objects) || payload.EdgeCount != len(snapshot.Edges) {
			t.Fatalf("object graph head counts = objects:%d edges:%d, want objects:%d edges:%d", payload.ObjectCount, payload.EdgeCount, len(snapshot.Objects), len(snapshot.Edges))
		}
		return
	}
	t.Fatalf("observation set missing object_graph_head: %#v", set.Observations)
}

func assertObservationDoltHead(t *testing.T, set computerversion.ObservationSet, doltHead *computerversion.DoltHeadSnapshot, snapshot *computerversion.ObjectGraphSnapshot) {
	t.Helper()
	if doltHead == nil {
		t.Fatal("dolt head snapshot is nil")
	}
	objectGraphPayload := objectGraphHeadPayload(t, snapshot)
	for _, observation := range set.Observations {
		if observation.Kind != computerversion.ObservationDoltHead {
			continue
		}
		wantKey := "dolt:" + doltHead.Database + ":head"
		if observation.Key != wantKey {
			t.Fatalf("dolt head observation key = %q, want %s", observation.Key, wantKey)
		}
		payload := decodeDoltHeadObservationPayload(t, observation.Value)
		if payload.Database != doltHead.Database || payload.CommitHash != doltHead.CommitHash {
			t.Fatalf("dolt head observation identity = database:%q commit:%q, want database:%q commit:%q", payload.Database, payload.CommitHash, doltHead.Database, doltHead.CommitHash)
		}
		if payload.ObjectGraphHead != objectGraphPayload.Head || payload.ObjectCount != objectGraphPayload.ObjectCount || payload.EdgeCount != objectGraphPayload.EdgeCount {
			t.Fatalf("dolt head objectgraph link = head:%q objects:%d edges:%d, want head:%q objects:%d edges:%d", payload.ObjectGraphHead, payload.ObjectCount, payload.EdgeCount, objectGraphPayload.Head, objectGraphPayload.ObjectCount, objectGraphPayload.EdgeCount)
		}
		return
	}
	t.Fatalf("observation set missing dolt_head: %#v", set.Observations)
}

type doltHeadObservationPayload struct {
	Database        string `json:"database"`
	CommitHash      string `json:"commit_hash"`
	ObjectGraphHead string `json:"object_graph_head"`
	ObjectCount     int    `json:"object_count"`
	EdgeCount       int    `json:"edge_count"`
}

func decodeDoltHeadObservationPayload(t *testing.T, value string) doltHeadObservationPayload {
	t.Helper()

	var payload doltHeadObservationPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode dolt head observation: %v\n%s", err, value)
	}
	return payload
}

type objectGraphObservationPayload struct {
	Head        string `json:"head"`
	ObjectCount int    `json:"object_count"`
	EdgeCount   int    `json:"edge_count"`
}

func objectGraphHeadPayload(t *testing.T, snapshot *computerversion.ObjectGraphSnapshot) objectGraphObservationPayload {
	t.Helper()
	if snapshot == nil {
		t.Fatal("object graph snapshot is nil")
	}
	value, err := snapshot.CanonicalHead()
	if err != nil {
		t.Fatalf("object graph canonical head: %v", err)
	}
	var payload objectGraphObservationPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		t.Fatalf("decode object graph canonical head: %v\n%s", err, value)
	}
	return payload
}

func assertObservationKinds(t *testing.T, got, want []computerversion.ObservationKind) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observation kinds = %#v, want %#v", got, want)
	}
}

func observationKinds(observations []computerversion.Observation) []computerversion.ObservationKind {
	kinds := make([]computerversion.ObservationKind, 0, len(observations))
	for _, observation := range observations {
		kinds = append(kinds, observation.Kind)
	}
	return kinds
}
