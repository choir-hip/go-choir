package selfdevprotocol

// Witness v3, the SQLite content witness. Ways this could fail:
//
//  1. a v1 or v2 witness validates, or is judged, instead of being refused
//     with the version named;
//  2. a v3 witness whose content root, schema hash or derivability digest was
//     altered validates;
//  3. the judge verifies two witnesses that differ in one table or in the
//     schema, or does not name the difference;
//  4. a witness built from an extraction differs from one built from its
//     observations (live and replay roots must be one computation).

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func testWitnessV3(database string) VMLocalContentWitness {
	return testWitnessV3Tables(database, map[string]string{"agents": strings.Repeat("b", 64)})
}

func testWitnessV3Tables(database string, tables map[string]string) VMLocalContentWitness {
	w, err := NewWitnessV3(database, strings.Repeat("a", 64), tables)
	if err != nil {
		panic(err)
	}
	return w
}

func TestWitnessV3RefusesOlderVersionsWithAReason(t *testing.T) {
	for _, version := range []int{0, 1, 2, 4} {
		w := testWitnessV3("texture")
		w.WitnessVersion = version
		err := w.Validate()
		if err == nil || !strings.Contains(err.Error(), "not judgeable") {
			t.Fatalf("version %d: Validate = %v, want a version refusal", version, err)
		}
		got := JudgeCheckpointWitness(DepositModeUpcast, w, testWitnessV3("texture"))
		if got.Status != CheckpointVerdictUnverified || got.Reason != VerdictReasonWitnessVersion {
			t.Fatalf("version %d: judged %+v, want unverified / witness_version", version, got)
		}
	}
}

func TestWitnessV3DetectsAlteration(t *testing.T) {
	for name, alter := range map[string]func(*VMLocalContentWitness){
		"content root": func(w *VMLocalContentWitness) { w.ContentRoot = strings.Repeat("0", 64) },
		"schema hash":  func(w *VMLocalContentWitness) { w.SchemaHash = strings.Repeat("0", 64) },
		"table digest": func(w *VMLocalContentWitness) { w.Tables["agents"] = strings.Repeat("0", 64) },
		"derivability": func(w *VMLocalContentWitness) { w.DerivabilityDigest = strings.Repeat("0", 64) },
		"no tables":    func(w *VMLocalContentWitness) { w.Tables = map[string]string{} },
	} {
		w := testWitnessV3("texture")
		w.Tables = map[string]string{"agents": w.Tables["agents"]}
		alter(&w)
		if err := w.Validate(); err == nil {
			t.Fatalf("%s: altered witness validated", name)
		}
	}
}

func TestJudgeNamesASchemaOnlyDifference(t *testing.T) {
	a := testWitnessV3("texture")
	b, err := NewWitnessV3("texture", strings.Repeat("c", 64), a.Tables)
	if err != nil {
		t.Fatal(err)
	}
	got := JudgeCheckpointWitness(DepositModeUpcast, a, b)
	if got.Status != CheckpointVerdictNotDerivable || len(got.DifferingTables) != 1 || got.DifferingTables[0] != schemaPseudoTable {
		t.Fatalf("schema-only difference judged %+v", got)
	}
}

func TestWitnessFromExtractionEqualsWitnessFromItsObservations(t *testing.T) {
	db := dbxtest.SQLite(t)
	if _, err := db.Write.Exec(`CREATE TABLE agents (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO agents VALUES (1, 'a'), (2, 'b')`); err != nil {
		t.Fatal(err)
	}
	extractor := computerversion.SQLiteStateExtractor{DB: db.Read, Database: "texture"}
	digests, err := extractor.Digests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	direct, err := WitnessFromDigests(digests)
	if err != nil {
		t.Fatal(err)
	}
	set, err := extractor.Extract(context.Background(), computerversion.ExtractRequest{
		Name: "t", Version: computerversion.ComputerVersion{CodeRef: "c", ArtifactProgramRef: "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	viaObservations, err := WitnessFromObservationSet(set)
	if err != nil {
		t.Fatal(err)
	}
	if err := WitnessContentMatches(direct, viaObservations); err != nil {
		t.Fatalf("the two witness paths disagree: %v", err)
	}
	if direct.DerivabilityDigest != viaObservations.DerivabilityDigest {
		t.Fatal("derivability digests differ between the two paths")
	}
}
