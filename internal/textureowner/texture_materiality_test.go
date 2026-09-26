package textureowner

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// R4: the materiality projection lands on the texture doc's evidence
// surface as commitment_materiality entities — falsified first, then
// overdue, then top-level open claims; falsified stays visible forever.

func materialityTestRecords() []types.CommitmentRecord {
	old := time.Now().UTC().Add(-100 * time.Hour).Format(time.RFC3339Nano)
	fresh := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	resAt := time.Now().UTC().Add(-10 * time.Hour).Format(time.RFC3339Nano)
	return []types.CommitmentRecord{
		{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "act-open",
			Prediction:  types.CommitmentPrediction{Hypothesis: "alpha stands", CommittedAt: fresh},
			Discrepancy: types.DiscrepancyUnresolved,
			Provenance:  types.CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1", CommittedAt: fresh}},
		{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "act-overdue",
			Prediction:  types.CommitmentPrediction{Hypothesis: "beta holds", CommittedAt: old},
			Discrepancy: types.DiscrepancyUnresolved,
			Provenance:  types.CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1", CommittedAt: old}},
		{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "act-falsified",
			Prediction:  types.CommitmentPrediction{Hypothesis: "gamma ships", CommittedAt: old},
			Discrepancy: types.DiscrepancyUnresolved,
			Provenance:  types.CommitmentProvenance{AgentID: "desk-b", ContextRef: "doc-1", CommittedAt: old}},
		{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "res-f",
			ParentID: "act-falsified", RelatedIDs: []string{"act-falsified"},
			Discrepancy: types.DiscrepancyContradicted,
			Observation: types.CommitmentObservation{Excerpt: "gamma failed", SourceRef: "act-falsified", ObservedAt: resAt},
			Provenance:  types.CommitmentProvenance{AgentID: "resolver", ResolvedAt: resAt}},
	}
}

func TestMaterialitySourceEntitiesSurfacesThreeClasses(t *testing.T) {
	entities := materialitySourceEntities(materialityTestRecords())
	byLabel := map[string]types.SourceEntity{}
	for _, e := range entities {
		if e.Kind != commitmentMaterialityKind {
			t.Fatalf("unexpected kind %q", e.Kind)
		}
		byLabel[e.Label] = e
	}
	var falsified, overdue, open bool
	for _, e := range entities {
		switch e.Target.ItemID {
		case "act-falsified:falsified":
			falsified = true
			if e.Evidence.State != "available" {
				t.Fatalf("falsified entity state %q", e.Evidence.State)
			}
		case "act-overdue:overdue":
			overdue = true
		case "act-open:top_claim":
			open = true
		default:
			t.Fatalf("unexpected entity target %q", e.Target.ItemID)
		}
	}
	if !falsified || !overdue || !open {
		t.Fatalf("entities = %+v", entities)
	}
	// Falsified first: projection order holds on the wire.
	if entities[0].Target.ItemID != "act-falsified:falsified" {
		t.Fatalf("first entity is %q, falsified must lead", entities[0].Target.ItemID)
	}
	// Pre-R4 records without CommittedAt never go overdue.
	records := append(materialityTestRecords(), types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "act-legacy",
		Prediction:  types.CommitmentPrediction{Hypothesis: "legacy"},
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1"},
	})
	for _, e := range materialitySourceEntities(records) {
		if e.Target.ItemID == "act-legacy:overdue" {
			t.Fatal("unstamped legacy record marked overdue")
		}
	}
}
