package selfdevprotocol

// The host's verdict on a checkpoint, written before the code. Ways it could
// fail:
//
//  1. equal witnesses are not verified (a verified checkpoint is the proof
//     the guest no longer makes at bind);
//  2. a mismatch between two stores built in the same deposit mode is not
//     called not_derivable, or does not name the tables;
//  3. residual R1: a mismatch against a live store that kept its recorded
//     deposit bytes (retained), or that did not say, is called not_derivable.
//     The host rebuild always upcasts, so that mismatch can be the deposit
//     mode of a store restore rebuilds correctly; calling it not_derivable
//     would make restore refuse a good restore point;
//  4. the Dolt HEAD (an audit receipt) decides the verdict;
//  5. an unknown deposit mode is accepted into a checkpoint.

import (
	"reflect"
	"strings"
	"testing"
)

func verdictWitness(tables map[string]string) VMLocalContentWitness {
	schema := map[string]string{}
	for table := range tables {
		schema[table] = strings.Repeat("5", 64)
	}
	w := VMLocalContentWitness{
		Database: "texture", ContentRoot: strings.Repeat("7", 64), DoltHead: "head-a",
		Schema: schema, Tables: tables,
	}
	sealed, err := SealWitness(w)
	if err != nil {
		panic(err)
	}
	return sealed
}

func withTable(w VMLocalContentWitness, table, hash string) VMLocalContentWitness {
	tables := map[string]string{}
	for k, v := range w.Tables {
		tables[k] = v
	}
	tables[table] = hash
	out := verdictWitness(tables)
	out.ContentRoot = strings.Repeat("8", 64)
	sealed, err := SealWitness(out)
	if err != nil {
		panic(err)
	}
	return sealed
}

func TestJudgeVerifiesEqualWitnessesInEveryDepositMode(t *testing.T) {
	base := map[string]string{"agents": strings.Repeat("a", 64), "og_objects": strings.Repeat("b", 64)}
	for _, mode := range []string{DepositModeUpcast, DepositModeRetained, ""} {
		checkpoint := verdictWitness(base)
		rebuilt := verdictWitness(base)
		rebuilt.DoltHead = "a-different-dolt-head" // an audit receipt, never the address
		got := JudgeCheckpointWitness(mode, checkpoint, rebuilt)
		if got.Status != CheckpointVerdictVerified || got.Reason != "" || len(got.DifferingTables) != 0 {
			t.Fatalf("mode %q: equal witnesses judged %+v, want verified", mode, got)
		}
	}
}

func TestJudgeNamesTheTablesOfAMismatchInTheSameDepositMode(t *testing.T) {
	base := map[string]string{"agents": strings.Repeat("a", 64), "og_objects": strings.Repeat("b", 64), "runs": strings.Repeat("c", 64)}
	checkpoint := verdictWitness(base)
	rebuilt := withTable(withTable(checkpoint, "og_objects", strings.Repeat("d", 64)), "runs", strings.Repeat("e", 64))
	got := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, rebuilt)
	if got.Status != CheckpointVerdictNotDerivable || got.Reason != VerdictReasonWitnessMismatch {
		t.Fatalf("same-mode mismatch judged %+v, want not_derivable / witness_mismatch", got)
	}
	if want := []string{"og_objects", "runs"}; !reflect.DeepEqual(got.DifferingTables, want) {
		t.Fatalf("differing tables %v, want %v", got.DifferingTables, want)
	}
}

func TestJudgeCountsATableOnlyOneSideHasAsDiffering(t *testing.T) {
	checkpoint := verdictWitness(map[string]string{"agents": strings.Repeat("a", 64), "live_only": strings.Repeat("b", 64)})
	rebuilt := verdictWitness(map[string]string{"agents": strings.Repeat("a", 64)})
	got := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, rebuilt)
	if got.Status != CheckpointVerdictNotDerivable || !reflect.DeepEqual(got.DifferingTables, []string{"live_only"}) {
		t.Fatalf("one-sided table judged %+v", got)
	}
}

// Residual R1: the case the design requires slice 2 to name.
func TestJudgeNamesTheBaseUpcastCaseAndDoesNotCallItNotDerivable(t *testing.T) {
	base := map[string]string{"agents": strings.Repeat("a", 64), "og_objects": strings.Repeat("b", 64)}
	checkpoint := verdictWitness(base)
	rebuilt := withTable(checkpoint, "og_objects", strings.Repeat("d", 64))
	for _, mode := range []string{DepositModeRetained, ""} {
		got := JudgeCheckpointWitness(mode, checkpoint, rebuilt)
		if got.Status != CheckpointVerdictUnverified || got.Reason != VerdictReasonBaseUpcast {
			t.Fatalf("mode %q: mismatch judged %+v, want unverified / base_upcast (R1)", mode, got)
		}
		if !reflect.DeepEqual(got.DifferingTables, []string{"og_objects"}) {
			t.Fatalf("mode %q: R1 verdict must still name the tables, got %v", mode, got.DifferingTables)
		}
	}
}

func TestJudgeRefusesAnInvalidWitnessAsUnverified(t *testing.T) {
	checkpoint := verdictWitness(map[string]string{"agents": strings.Repeat("a", 64)})
	broken := checkpoint
	broken.ContentRoot = "not-a-digest"
	got := JudgeCheckpointWitness(DepositModeUpcast, checkpoint, broken)
	if got.Status != CheckpointVerdictUnverified || got.Reason != VerdictReasonInvalidWitness {
		t.Fatalf("invalid rebuilt witness judged %+v", got)
	}
}

func TestCheckpointRequestRefusesAnUnknownDepositMode(t *testing.T) {
	request := ownerRecoveryTestRequest()
	for _, mode := range []string{"", DepositModeRetained, DepositModeUpcast} {
		request.DepositMode = mode
		if _, _, err := CheckpointFromRequest(request); err != nil {
			t.Fatalf("deposit mode %q refused: %v", mode, err)
		}
	}
	request.DepositMode = "mystery"
	if _, _, err := CheckpointFromRequest(request); err == nil {
		t.Fatal("checkpoint accepted an unknown deposit mode")
	}
}

// The deposit mode is advisory metadata for the host's verdict, not part of
// what the checkpoint commits to: a guest rolled back below the field decodes
// the checkpoint (dropping the unknown field), recomputes the digest and must
// get the same one, or every checkpoint a newer guest bound is unrestorable
// after the rollback (review 329, DeepSeek 5). It stays in the stored
// checkpoint so the host can read it.
func TestDepositModeIsNotPartOfTheCheckpointDigest(t *testing.T) {
	request := ownerRecoveryTestRequest()
	without, _, err := CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{DepositModeRetained, DepositModeUpcast} {
		request.DepositMode = mode
		with, _, err := CheckpointFromRequest(request)
		if err != nil {
			t.Fatal(err)
		}
		if with.Digest != without.Digest {
			t.Fatalf("mode %q changed the digest %s -> %s", mode, without.Digest, with.Digest)
		}
		if with.Request.DepositMode != mode {
			t.Fatalf("the stored checkpoint lost the mode: %q", with.Request.DepositMode)
		}
		// What an older guest does with the checkpoint: drop the field, rebuild.
		stripped := with.Request
		stripped.DepositMode = ""
		again, _, err := CheckpointFromRequest(stripped)
		if err != nil || again.Digest != with.Digest {
			t.Fatalf("a guest that does not know the field computes another digest: %v %s vs %s", err, again.Digest, with.Digest)
		}
	}
}
