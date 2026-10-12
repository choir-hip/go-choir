package computerversion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	embedded "github.com/dolthub/driver/v2"
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

func TestDoltExtractorIgnoredColumnsPreserveSemanticDrift(t *testing.T) {
	ddl := "CREATE TABLE records (id INT PRIMARY KEY, status VARCHAR(64) NOT NULL, prepared_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)"
	workspaceA := openTestDoltWorkspace(t, "testdb", ddl,
		[]string{"INSERT INTO records (id, status, prepared_at, updated_at) VALUES (1, 'active', '2026-01-01 00:00:00', '2026-01-01 00:00:00')"})
	workspaceB := openTestDoltWorkspace(t, "testdb", ddl,
		[]string{"INSERT INTO records (id, status, prepared_at, updated_at) VALUES (1, 'active', '2027-01-01 00:00:00', '2027-01-01 00:00:00')"})
	workspaceC := openTestDoltWorkspace(t, "testdb", ddl,
		[]string{"INSERT INTO records (id, status, prepared_at, updated_at) VALUES (1, 'changed', '2026-01-01 00:00:00', '2026-01-01 00:00:00')"})

	version := ComputerVersion{CodeRef: "test-code-ref", ArtifactProgramRef: "test-artifact-ref"}
	extractor := func(workspace string) ObservationSet {
		observations, err := (DoltStateExtractor{
			WorkspacePath: workspace,
			Database:      "testdb",
			IgnoredContentColumns: map[string]map[string]struct{}{
				"records": {"prepared_at": {}, "updated_at": {}},
			},
		}).Extract(context.Background(), ExtractRequest{Name: "ignored-columns", Version: version})
		if err != nil {
			t.Fatal(err)
		}
		return observations
	}
	tableObservation := func(observations ObservationSet) string {
		for _, observation := range observations.Observations {
			if observation.Key == "dolt:testdb:table:records" {
				return observation.Value
			}
		}
		t.Fatal("records table observation missing")
		return ""
	}

	semanticA := tableObservation(extractor(workspaceA))
	timestampOnly := tableObservation(extractor(workspaceB))
	semanticDrift := tableObservation(extractor(workspaceC))
	if semanticA != timestampOnly {
		t.Fatalf("ignored timestamp changed content hash: got %s want %s", timestampOnly, semanticA)
	}
	if semanticA == semanticDrift {
		t.Fatalf("semantic column change did not change content hash: %s", semanticDrift)
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

