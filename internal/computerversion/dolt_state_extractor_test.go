package computerversion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	embedded "github.com/dolthub/driver"
)

// openTestDoltWorkspace creates a temporary embedded Dolt workspace with
// the named database, applies the given schema DDL, inserts the given rows,
// commits, and returns the workspace path. The caller uses the path to
// construct a DoltStateExtractor.
func openTestDoltWorkspace(t *testing.T, database, ddl string, inserts []string) string {
	t.Helper()
	root := t.TempDir()

	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	rootDB.SetMaxOpenConns(1)
	rootDB.SetMaxIdleConns(1)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + database); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true", root, database)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		_ = dbConnector.Close()
	})

	if ddl != "" {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("apply ddl: %v", err)
		}
	}
	for _, ins := range inserts {
		if _, err := db.Exec(ins); err != nil {
			t.Fatalf("insert: %v\n  query: %s", err, ins)
		}
	}
	if _, err := db.Exec("CALL DOLT_COMMIT('-Am', 'test fixture')"); err != nil {
		// "nothing to commit" is fine for empty workspaces
		_ = err
	}

	return root
}

// TestDoltExtractorRoundTrip proves the DoltStateExtractor can connect to a
// live embedded Dolt database, query the HEAD commit hash, and produce a
// valid ObservationSet with dolt_head observations.
func TestDoltExtractorRoundTrip(t *testing.T) {
	workspace := openTestDoltWorkspace(t, "testdb",
		"CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)",
		[]string{
			"INSERT INTO items (id, name) VALUES (1, 'alpha')",
			"INSERT INTO items (id, name) VALUES (2, 'beta')",
		},
	)
	version := ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: "test-artifact-ref",
	}
	ctx := context.Background()

	extractor := DoltStateExtractor{
		WorkspacePath: workspace,
		Database:      "testdb",
	}
	obs, err := extractor.Extract(ctx, ExtractRequest{
		Name:    "dolt-roundtrip",
		Version: version,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if obs.Name != "dolt-roundtrip" {
		t.Errorf("name: got %q, want %q", obs.Name, "dolt-roundtrip")
	}
	if obs.Version != version {
		t.Errorf("version mismatch: got %+v, want %+v", obs.Version, version)
	}
	if len(obs.Observations) == 0 {
		t.Fatal("no observations produced")
	}

	// Must have exactly one dolt_head observation for the head.
	hasHead := false
	schemaCount := 0
	for _, o := range obs.Observations {
		if o.Kind != ObservationDoltHead {
			t.Errorf("unexpected kind %q for key %q", o.Kind, o.Key)
		}
		if o.Key == "dolt:testdb:head" {
			hasHead = true
			if o.Value == "" {
				t.Error("head observation value is empty")
			}
			// Verify the value is valid JSON with database and commit_hash.
			var payload doltHeadPayload
			if err := json.Unmarshal([]byte(o.Value), &payload); err != nil {
				t.Fatalf("decode head payload: %v", err)
			}
			if payload.Database != "testdb" {
				t.Errorf("payload database: got %q, want %q", payload.Database, "testdb")
			}
			if payload.CommitHash == "" {
				t.Error("payload commit hash is empty")
			}
		}
		if strings.HasPrefix(o.Key, "dolt:testdb:schema:") {
			schemaCount++
		}
	}
	if !hasHead {
		t.Error("missing dolt_head observation for dolt:testdb:head")
	}
	if schemaCount == 0 {
		t.Error("expected at least one schema observation, got none")
	}
	t.Logf("dolt extractor roundtrip: %d observations (1 head + %d schemas)",
		len(obs.Observations), schemaCount)
}

// TestDoltExtractorCrossSubstrateEquivalence proves SIAC Gate 4 for the
// Dolt/app-state ledger: the same Dolt database content, initialized in two
// independent workspace directories, produces equivalent dolt_head
// observations when extracted by the DoltStateExtractor.
//
// This is a TRUE cross-substrate proof because:
//  1. Two independent Dolt workspaces are created in separate temp dirs.
//  2. The same schema and data are applied to both.
//  3. Both are committed independently.
//  4. The extractor reads from each independently.
//  5. Equivalence is verified by comparing the extracted content observations.
//
// The dolt:{db}:head observation (commit hash) is excluded from the
// cross-substrate comparison because independent Dolt commits with identical
// data produce different commit hashes (timestamp inclusion). The content
// observations (schema hashes, table content hashes, content root hash) are
// deterministic and must match.
func TestDoltExtractorCrossSubstrateEquivalence(t *testing.T) {
	ddl := "CREATE TABLE agents (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, status VARCHAR(64) NOT NULL DEFAULT 'active')"
	inserts := []string{
		"INSERT INTO agents (id, name, status) VALUES ('agent-1', 'Alpha', 'active')",
		"INSERT INTO agents (id, name, status) VALUES ('agent-2', 'Beta', 'idle')",
	}

	workspaceA := openTestDoltWorkspace(t, "choir", ddl, inserts)
	workspaceB := openTestDoltWorkspace(t, "choir", ddl, inserts)

	version := ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: "dolt:choir:head",
	}
	ctx := context.Background()

	obsA, err := DoltStateExtractor{
		WorkspacePath: workspaceA,
		Database:      "choir",
	}.Extract(ctx, ExtractRequest{Name: "substrate-a", Version: version})
	if err != nil {
		t.Fatalf("extract from substrate A: %v", err)
	}
	obsB, err := DoltStateExtractor{
		WorkspacePath: workspaceB,
		Database:      "choir",
	}.Extract(ctx, ExtractRequest{Name: "substrate-b", Version: version})
	if err != nil {
		t.Fatalf("extract from substrate B: %v", err)
	}

	// Filter out the substrate-specific head observation (commit hash).
	// Independent Dolt commits with identical data produce different commit
	// hashes. The content observations are the cross-substrate signal.
	obsA = filterDoltHeadObservation(obsA)
	obsB = filterDoltHeadObservation(obsB)

	// Materialize both under substrate-specific capability manifests.
	realizationA, err := ProjectionMaterializer{
		ID:           DoltStateMaterializer,
		Observations: obsA,
	}.Materialize(ctx, version, DoltStateCapabilityManifest("", "embedded-dolt/workspace-a"))
	if err != nil {
		t.Fatalf("materialize substrate A: %v", err)
	}
	realizationB, err := ProjectionMaterializer{
		ID:           DoltStateMaterializer,
		Observations: obsB,
	}.Materialize(ctx, version, DoltStateCapabilityManifest("", "embedded-dolt/workspace-b"))
	if err != nil {
		t.Fatalf("materialize substrate B: %v", err)
	}

	// Verify non-identical substrate identities.
	if realizationA.Capabilities.Substrate == realizationB.Capabilities.Substrate {
		t.Fatal("substrates must be non-identical for cross-substrate proof")
	}

	// Run the equivalence checker.
	result := EquivalenceChecker{}.CheckRealizations(realizationA, realizationB)
	if result.Status != EquivalenceEquivalent {
		t.Errorf("expected equivalence, got %s", result.Status)
		for _, d := range result.Differences {
			t.Errorf("  diff: kind=%s key=%s left=%s right=%s reason=%s",
				d.Kind, d.Key, d.Left, d.Right, d.Reason)
		}
		for _, u := range result.Unsupported {
			t.Errorf("  unsupported: kind=%s reason=%s", u.Kind, u.Reason)
		}
	}
	if !result.Equivalent() {
		t.Fatal("equivalence check failed: result is not equivalent")
	}

	t.Logf("cross-substrate Dolt equivalence proven: %s (%s) == %s (%s) for %s@%s",
		realizationA.Capabilities.Materializer, realizationA.Capabilities.Substrate,
		realizationB.Capabilities.Materializer, realizationB.Capabilities.Substrate,
		version.CodeRef, version.ArtifactProgramRef)
}

// TestDoltExtractorCrossSubstrateFailure proves SIAC Gate 5 for the
// Dolt/app-state ledger: a seeded mismatch in one Dolt database causes the
// equivalence checker to fail, proving the verifier is not ceremonial.
//
// The mismatch is introduced by inserting a different row in substrate B
// before committing, so the table content hashes will differ.
func TestDoltExtractorCrossSubstrateFailure(t *testing.T) {
	ddl := "CREATE TABLE agents (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, status VARCHAR(64) NOT NULL DEFAULT 'active')"

	workspaceA := openTestDoltWorkspace(t, "choir", ddl, []string{
		"INSERT INTO agents (id, name, status) VALUES ('agent-1', 'Alpha', 'active')",
	})
	workspaceB := openTestDoltWorkspace(t, "choir", ddl, []string{
		"INSERT INTO agents (id, name, status) VALUES ('agent-1', 'CORRUPTED', 'active')",
	})

	version := ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: "dolt:choir:head",
	}
	ctx := context.Background()

	obsA, err := DoltStateExtractor{
		WorkspacePath: workspaceA,
		Database:      "choir",
	}.Extract(ctx, ExtractRequest{Name: "substrate-a", Version: version})
	if err != nil {
		t.Fatalf("extract from substrate A: %v", err)
	}
	obsB, err := DoltStateExtractor{
		WorkspacePath: workspaceB,
		Database:      "choir",
	}.Extract(ctx, ExtractRequest{Name: "substrate-b", Version: version})
	if err != nil {
		t.Fatalf("extract from substrate B: %v", err)
	}

	// Filter out the substrate-specific head observation (commit hash).
	obsA = filterDoltHeadObservation(obsA)
	obsB = filterDoltHeadObservation(obsB)

	realizationA, err := ProjectionMaterializer{
		ID:           DoltStateMaterializer,
		Observations: obsA,
	}.Materialize(ctx, version, DoltStateCapabilityManifest("", "embedded-dolt/workspace-a"))
	if err != nil {
		t.Fatalf("materialize substrate A: %v", err)
	}
	realizationB, err := ProjectionMaterializer{
		ID:           DoltStateMaterializer,
		Observations: obsB,
	}.Materialize(ctx, version, DoltStateCapabilityManifest("", "embedded-dolt/workspace-b"))
	if err != nil {
		t.Fatalf("materialize substrate B: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(realizationA, realizationB)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected not_equivalent, got %s", result.Status)
	}
	if len(result.Differences) == 0 {
		t.Fatal("expected at least one difference, got none")
	}

	// The table content hash and content root hash should differ because
	// the data differs.
	foundContentMismatch := false
	for _, d := range result.Differences {
		t.Logf("  diff: kind=%s key=%s reason=%s", d.Kind, d.Key, d.Reason)
		if d.Key == "dolt:choir:table:agents" || d.Key == "dolt:choir:content_root" {
			foundContentMismatch = true
		}
	}
	if !foundContentMismatch {
		t.Errorf("expected a difference on table content or content_root, got: %+v", result.Differences)
	}

	t.Logf("failure proof confirmed: equivalence checker detected seeded Dolt mismatch with %d differences", len(result.Differences))
}

// TestDoltExtractorCapabilityManifest verifies the capability manifest
// declares dolt_head as supported and other kinds as unsupported.
func TestDoltExtractorCapabilityManifest(t *testing.T) {
	manifest := DoltStateCapabilityManifest("", "")
	if !manifest.Supports(ObservationDoltHead) {
		t.Error("manifest should support dolt_head")
	}
	if manifest.Supports(ObservationFileManifest) {
		t.Error("manifest should not support file_manifest")
	}
	if manifest.Supports(ObservationBlobSet) {
		t.Error("manifest should not support blob_set")
	}
	if manifest.Materializer != DoltStateMaterializer {
		t.Errorf("materializer: got %q, want %q", manifest.Materializer, DoltStateMaterializer)
	}
	if manifest.Substrate != DoltStateSubstrate {
		t.Errorf("substrate: got %q, want %q", manifest.Substrate, DoltStateSubstrate)
	}
}

// TestDoltExtractorRejectsInvalidVersion verifies the extractor rejects
// invalid ComputerVersion inputs.
func TestDoltExtractorRejectsInvalidVersion(t *testing.T) {
	workspace := openTestDoltWorkspace(t, "testdb", "", nil)
	ctx := context.Background()

	_, err := DoltStateExtractor{
		WorkspacePath: workspace,
		Database:      "testdb",
	}.Extract(ctx, ExtractRequest{
		Name:    "test",
		Version: ComputerVersion{}, // invalid
	})
	if err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
}

// TestDoltExtractorRejectsEmptyWorkspace verifies the extractor rejects
// empty workspace paths.
func TestDoltExtractorRejectsEmptyWorkspace(t *testing.T) {
	ctx := context.Background()
	version := ComputerVersion{
		CodeRef:            "test",
		ArtifactProgramRef: "test",
	}

	_, err := DoltStateExtractor{
		WorkspacePath: "",
		Database:      "testdb",
	}.Extract(ctx, ExtractRequest{Name: "test", Version: version})
	if err == nil {
		t.Fatal("expected error for empty workspace, got nil")
	}
}

// TestDoltExtractorRejectsEmptyDatabase verifies the extractor rejects
// empty database names.
func TestDoltExtractorRejectsEmptyDatabase(t *testing.T) {
	workspace := t.TempDir()
	ctx := context.Background()
	version := ComputerVersion{
		CodeRef:            "test",
		ArtifactProgramRef: "test",
	}

	_, err := DoltStateExtractor{
		WorkspacePath: workspace,
		Database:      "",
	}.Extract(ctx, ExtractRequest{Name: "test", Version: version})
	if err == nil {
		t.Fatal("expected error for empty database, got nil")
	}
}

// filterDoltHeadObservation returns a copy of the observation set with the
// substrate-specific dolt:{db}:head observation (commit hash) removed.
// This is used in cross-substrate proofs where the commit hash is expected
// to differ (independent commits) but content observations must match.
func filterDoltHeadObservation(obs ObservationSet) ObservationSet {
	filtered := make([]Observation, 0, len(obs.Observations))
	for _, o := range obs.Observations {
		if strings.HasSuffix(o.Key, ":head") && strings.HasPrefix(o.Key, "dolt:") {
			continue
		}
		filtered = append(filtered, o)
	}
	return ObservationSet{
		Name:         obs.Name,
		Version:      obs.Version,
		Required:     obs.Required,
		Observations: filtered,
	}
}


