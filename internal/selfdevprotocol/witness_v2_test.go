package selfdevprotocol

// Witness v2, the hash-only checkpoint bind
// (docs/problems/paper-never-checkpointed-2026-10-27.md, S3). Written before
// the code. Ways this could fail:
//
//  1. a host without v2 accepts a v2 witness as a v1 one (it must refuse:
//     the v1 schema and table maps stay empty, so v1 validation fails);
//  2. an unknown witness version validates;
//  3. the judge verifies a v2 checkpoint whose hashable table differs from
//     the rebuild, or misses a table only one side has;
//  4. the judge refuses a v2 checkpoint because an operational table (wall-
//     clock columns, cannot match by construction) differs;
//  5. the judge compares a v2 checkpoint against a rebuilt witness with no
//     Dolt hashes and calls it not derivable (it cannot judge: unverified);
//  6. restore's witness match accepts a v2 checkpoint against a v1
//     reconstruction, or a v2 reconstruction with another table hash.

import (
	"encoding/json"
	"strings"
	"testing"
)

func v2Fixture(t *testing.T, tables map[string]string) VMLocalContentWitness {
	t.Helper()
	w, err := WitnessV2FromDoltHashes("texture", "headcommit0000000000000000000000", tables, []string{"computer_event_index", "computer_event_projection_heads"})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func v2Tables() map[string]string {
	return map[string]string{
		"documents":  "nitgc4or1bbo8qm6lppo4cljmjcqt3gd",
		"run_memory": "ev7qhlv4l4fnj7lcg0utdne9ovc0qug0",
	}
}

func TestWitnessV2IsRefusedByAV1Reader(t *testing.T) {
	w := v2Fixture(t, v2Tables())
	if err := w.Validate(); err != nil {
		t.Fatalf("v2 witness does not validate on a v2 reader: %v", err)
	}
	if len(w.Schema) != 0 || len(w.Tables) != 0 {
		t.Fatal("a v2 witness carries v1 schema/table maps; a v1 reader would judge it as v1")
	}
	// A v1 reader decodes the JSON into its own struct, which has no version
	// and no dolt maps; its Validate needs schema and table hashes.
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Schema map[string]string `json:"schema"`
		Tables map[string]string `json:"tables"`
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	if len(old.Schema) != 0 || len(old.Tables) != 0 {
		t.Fatalf("a v1 reader sees v1 hashes in a v2 witness: %s", raw)
	}
}

func TestWitnessUnknownVersionRefused(t *testing.T) {
	w := v2Fixture(t, v2Tables())
	w.WitnessVersion = 3
	if err := w.Validate(); err == nil {
		t.Fatal("witness version 3 validated")
	}
}

func TestWitnessV2TamperedDigestRefused(t *testing.T) {
	w := v2Fixture(t, v2Tables())
	w.DoltTables["documents"] = "00000000000000000000000000000000"
	if err := w.Validate(); err == nil {
		t.Fatal("a v2 witness whose table hash changed after sealing validated")
	}
}

func TestJudgeWitnessV2(t *testing.T) {
	checkpoint := v2Fixture(t, v2Tables())

	rebuilt := v1WitnessWithDoltTables(t, v2Tables())
	if v := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, rebuilt); v.Status != CheckpointVerdictVerified {
		t.Fatalf("equal hashes: %+v", v)
	}

	differ := v2Tables()
	differ["run_memory"] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	v := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, v1WitnessWithDoltTables(t, differ))
	if v.Status != CheckpointVerdictNotDerivable || len(v.DifferingTables) != 1 || v.DifferingTables[0] != "run_memory" {
		t.Fatalf("differing hashable table: %+v", v)
	}
	if v := JudgeCheckpointWitness(DepositModeRetained, checkpoint, v1WitnessWithDoltTables(t, differ)); v.Status != CheckpointVerdictUnverified || v.Reason != VerdictReasonBaseUpcast {
		t.Fatalf("retained deposit mode: %+v", v)
	}

	extra := v2Tables()
	extra["live_only"] = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if v := JudgeCheckpointWitness(DepositModeUpcast, v2Fixture(t, extra), rebuilt); v.Status != CheckpointVerdictNotDerivable {
		t.Fatalf("a table only the checkpoint has: %+v", v)
	}

	noHashes := rebuilt
	noHashes.DoltTables = nil
	if v := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, noHashes); v.Status != CheckpointVerdictUnverified || v.Reason != VerdictReasonWitnessVersion {
		t.Fatalf("rebuild without dolt hashes: %+v", v)
	}
}

// The operational tables are not in DoltTables at all, on either side, so
// their wall-clock columns cannot make a verdict.
func TestWitnessV2ExcludesOperationalTables(t *testing.T) {
	tables := v2Tables()
	tables["computer_event_index"] = "cccccccccccccccccccccccccccccccc"
	if _, err := WitnessV2FromDoltHashes("texture", "h", tables, []string{"computer_event_index"}); err == nil {
		t.Fatal("a v2 witness hashed an operational table")
	}
}

func TestWitnessContentMatchesV2(t *testing.T) {
	want := v2Fixture(t, v2Tables())
	if err := WitnessContentMatches(v2Fixture(t, v2Tables()), want); err != nil {
		t.Fatalf("equal v2 witnesses: %v", err)
	}
	other := v2Tables()
	other["documents"] = "dddddddddddddddddddddddddddddddd"
	if err := WitnessContentMatches(v2Fixture(t, other), want); err == nil || !strings.Contains(err.Error(), "documents") {
		t.Fatalf("v2 table mismatch: %v", err)
	}
	if err := WitnessContentMatches(v1WitnessWithDoltTables(t, v2Tables()), want); err == nil {
		t.Fatal("a v1 reconstruction matched a v2 checkpoint")
	}
}

// v1WitnessWithDoltTables is a host base's witness: v1 row hashes, plus the
// Dolt table hashes the host records alongside them.
func v1WitnessWithDoltTables(t *testing.T, tables map[string]string) VMLocalContentWitness {
	t.Helper()
	digest := strings.Repeat("a", 64)
	w := VMLocalContentWitness{
		Database:    "texture",
		ContentRoot: digest,
		Schema:      map[string]string{"documents": digest},
		Tables:      map[string]string{"documents": digest},
		DoltHead:    "x",
	}
	sealed, err := SealWitness(w)
	if err != nil {
		t.Fatal(err)
	}
	sealed.DoltTables = tables
	if err := sealed.Validate(); err != nil {
		t.Fatalf("a v1 witness with dolt tables attached: %v", err)
	}
	return sealed
}
