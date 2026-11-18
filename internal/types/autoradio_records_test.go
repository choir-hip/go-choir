package types

import (
	"strings"
	"testing"
)

// Failure list for the Autoradio record table (learning-engine design 12.2),
// written before the code:
//   - a record kind of the table has no label, or a label other than
//     "autoradio_" plus the word after the dot of the kind;
//   - the table drifts from design 12.2 (a kind is missing or extra);
//   - a record of record_kind autoradio.* with no owner-private label, or the
//     label of another kind, passes CheckAutoradioLabel;
//   - a record of an unknown autoradio.* kind passes;
//   - a record of another desk is touched by the check.

func TestAutoradioRecordKindsTable(t *testing.T) {
	want := []string{
		"autoradio.turn", "autoradio.request", "autoradio.forecast", "autoradio.settlement",
		"autoradio.stretch_start", "autoradio.segment", "autoradio.stretch", "autoradio.air",
		"autoradio.conflict", "autoradio.lesson", "autoradio.strike", "autoradio.clear", "autoradio.note",
		"autoradio.sheet_op", "autoradio.sheet_snapshot", "autoradio.no_verdict", "autoradio.notebook_patch",
	}
	if len(AutoradioRecordKinds) != len(want) {
		t.Fatalf("table has %d kinds, design 12.2 has %d", len(AutoradioRecordKinds), len(want))
	}
	seen := map[string]bool{}
	for i, k := range AutoradioRecordKinds {
		if k.Kind != want[i] {
			t.Fatalf("kind %d = %q, want %q", i, k.Kind, want[i])
		}
		word := strings.TrimPrefix(k.Kind, "autoradio.")
		if k.Label != "autoradio_"+word {
			t.Fatalf("%s label = %q", k.Kind, k.Label)
		}
		if seen[k.Kind] {
			t.Fatalf("duplicate kind %s", k.Kind)
		}
		seen[k.Kind] = true
		label, ok := AutoradioKindLabel(k.Kind)
		if !ok || label != OwnerPrivateLabel(k.Label) {
			t.Fatalf("AutoradioKindLabel(%s) = %q %v", k.Kind, label, ok)
		}
	}
	if _, ok := AutoradioKindLabel("autoradio.nope"); ok {
		t.Fatal("unknown kind has a label")
	}
}

func TestCheckAutoradioLabel(t *testing.T) {
	for _, k := range AutoradioRecordKinds {
		label, _ := AutoradioKindLabel(k.Kind)
		rec := CommitmentRecord{SubjectRefs: map[string]string{"desk": "app:autoradio", "record_kind": k.Kind}}
		if err := CheckAutoradioLabel(rec); err == nil {
			t.Fatalf("%s with no label passed", k.Kind)
		}
		rec.Provenance.Labels = []string{OwnerPrivateLabel("autoradio_other")}
		if err := CheckAutoradioLabel(rec); err == nil {
			t.Fatalf("%s with another kind's label passed", k.Kind)
		}
		rec.Provenance.Labels = []string{label}
		if err := CheckAutoradioLabel(rec); err != nil {
			t.Fatalf("%s with its label: %v", k.Kind, err)
		}
		rec.Provenance.Labels = []string{LabelPlatform, label}
		if err := CheckAutoradioLabel(rec); err != nil {
			t.Fatalf("%s with its label and another atom: %v", k.Kind, err)
		}
	}
	unknown := CommitmentRecord{SubjectRefs: map[string]string{"record_kind": "autoradio.nope"}, Provenance: CommitmentProvenance{Labels: []string{OwnerPrivateLabel("autoradio_nope")}}}
	if err := CheckAutoradioLabel(unknown); err == nil {
		t.Fatal("unknown autoradio kind passed")
	}
	other := CommitmentRecord{SubjectRefs: map[string]string{"record_kind": "texture.note"}}
	if err := CheckAutoradioLabel(other); err != nil {
		t.Fatalf("another desk's record refused: %v", err)
	}
	if err := CheckAutoradioLabel(CommitmentRecord{}); err != nil {
		t.Fatalf("record with no subject refs refused: %v", err)
	}
}
