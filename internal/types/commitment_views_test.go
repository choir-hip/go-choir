package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// R4 acceptance: derived accrual over seeded Precommit + linked Resolve
// records returns per-agent/per-doc accrual without mutating any stored
// record — the view is a read, never a second write authority.

func seedCommitmentRecords() []CommitmentRecord {
	committed := time.Now().UTC().Add(-100 * time.Hour).Format(time.RFC3339Nano)
	recent := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	resolvedAt := time.Now().UTC().Add(-50 * time.Hour).Format(time.RFC3339Nano)
	resolve := func(target, recordID, verdict string, sc DiscrepancyClass, disagreement bool) CommitmentRecord {
		return CommitmentRecord{
			SchemaID:    CommitmentRecordSchemaV1,
			RecordID:    recordID,
			ParentID:    target,
			Discrepancy: sc,
			Observation: CommitmentObservation{
				Excerpt: "outcome: " + verdict, SourceRef: target, ObservedAt: resolvedAt,
			},
			Scores: []CommitmentScore{
				{ScorerModelID: "resolver-a", Answers: map[string]string{"outcome": verdict}, ScoredAt: resolvedAt},
				{ScorerModelID: "resolver-b", Answers: map[string]string{"outcome": "split"}, Disagreement: disagreement, ScoredAt: resolvedAt},
			},
			Provenance: CommitmentProvenance{AgentID: "resolver-desk", ResolvedAt: resolvedAt},
			RelatedIDs: []string{target},
		}
	}
	return []CommitmentRecord{
		{ // open claim, still fresh — top-level
			SchemaID:    CommitmentRecordSchemaV1,
			RecordID:    "act-open-1",
			Prediction:  CommitmentPrediction{Hypothesis: "alpha wins", CommittedAt: recent, Questions: []TypedQuestion{{Question: "q", Weight: 2}}},
			Discrepancy: DiscrepancyUnresolved,
			Provenance:  CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1", CommittedAt: recent},
			Addressee:   "desk-b",
		},
		{ // open claim, past the overdue bound
			SchemaID:    CommitmentRecordSchemaV1,
			RecordID:    "act-overdue-1",
			Prediction:  CommitmentPrediction{Hypothesis: "beta holds", CommittedAt: committed, Questions: []TypedQuestion{{Question: "q", Weight: 3}}},
			Discrepancy: DiscrepancyUnresolved,
			Provenance:  CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1", CommittedAt: committed},
		},
		{ // falsified claim
			SchemaID:    CommitmentRecordSchemaV1,
			RecordID:    "act-falsified-1",
			Prediction:  CommitmentPrediction{Hypothesis: "gamma ships", CommittedAt: committed, Questions: []TypedQuestion{{Question: "q", Weight: 5}}},
			Discrepancy: DiscrepancyUnresolved,
			Provenance:  CommitmentProvenance{AgentID: "desk-b", ContextRef: "doc-2", CommittedAt: committed},
		},
		{ // confirmed claim
			SchemaID:    CommitmentRecordSchemaV1,
			RecordID:    "act-confirmed-1",
			Prediction:  CommitmentPrediction{Hypothesis: "delta lands", CommittedAt: committed},
			Discrepancy: DiscrepancyUnresolved,
			Provenance:  CommitmentProvenance{AgentID: "desk-a", ContextRef: "doc-1", CommittedAt: committed},
		},
		// Resolutions: the falsified target takes the scored record;
		// confirmed takes a second resolution verdict.
		resolve("act-falsified-1", "res-f-1", "wrong", DiscrepancyContradicted, true),
		resolve("act-confirmed-1", "res-c-1", "right", DiscrepancyConfirmed, false),
	}
}

func TestAccrualByAgentDerivesStampsWithoutMutation(t *testing.T) {
	records := seedCommitmentRecords()
	before, _ := json.Marshal(records)
	accrual := AccrualByAgent(records, time.Now().UTC())
	byID := map[string]CommitmentAccrual{}
	for _, a := range accrual {
		byID[a.AgentID] = a
	}
	a := byID["desk-a"]
	if a.Committed != 3 || a.Open != 2 || a.Confirmed != 1 {
		t.Fatalf("desk-a accrual = %+v", a)
	}
	b := byID["desk-b"]
	if b.Committed != 1 || b.Contradicted != 1 {
		t.Fatalf("desk-b accrual = %+v", b)
	}

	// Weight: act-open-1 (2) + act-overdue-1 (3) + act-confirmed-1 (0)
	// for a; the falsified act's weight (5) accrues under desk-b.
	if a.MaterialityWeight != 5 {
		t.Fatalf("desk-a weight = %v", a.MaterialityWeight)
	}
	after, _ := json.Marshal(records)
	if string(before) != string(after) {
		t.Fatal("accrual mutated stored records")
	}
}

func TestAccrualByDocScopesOnContextRef(t *testing.T) {
	accrual := AccrualByDoc(seedCommitmentRecords(), time.Now().UTC())
	byCtx := map[string]CommitmentAccrual{}
	for _, a := range accrual {
		byCtx[a.ContextRef] = a
	}
	if byCtx["doc-1"].Committed != 3 || byCtx["doc-2"].Committed != 1 {
		t.Fatalf("doc accrual = %+v", byCtx)
	}
}

func TestProjectMaterialitySurfacesThreeClassesDistinctly(t *testing.T) {
	entries := ProjectMateriality(seedCommitmentRecords(), time.Now().UTC(), 72*time.Hour)
	byClass := map[string][]CommitmentMaterialityEntry{}
	for _, e := range entries {
		byClass[e.Materiality] = append(byClass[e.Materiality], e)
	}
	if len(byClass[MaterialityFalsified]) != 1 || byClass[MaterialityFalsified][0].RecordID != "act-falsified-1" {
		t.Fatalf("falsified = %+v", byClass[MaterialityFalsified])
	}
	if len(byClass[MaterialityOverdue]) != 1 || byClass[MaterialityOverdue][0].RecordID != "act-overdue-1" {
		t.Fatalf("overdue = %+v", byClass[MaterialityOverdue])
	}
	if len(byClass[MaterialityTopClaim]) != 1 || byClass[MaterialityTopClaim][0].RecordID != "act-open-1" {
		t.Fatalf("top = %+v", byClass[MaterialityTopClaim])
	}
	// Falsified stays visible and carries its observation.
	f := byClass[MaterialityFalsified][0]
	if f.Discrepancy != DiscrepancyContradicted || !strings.Contains(f.ObservationExcerpt, "wrong") {
		t.Fatalf("falsified entry loses verdict: %+v", f)
	}
	// Confirmed records are settled evidence, not a materiality class.
	for _, e := range entries {
		if e.RecordID == "act-confirmed-1" {
			t.Fatalf("confirmed record leaked into projection: %+v", e)
		}
	}
}

func TestActingPackCarriesNoScoreFields(t *testing.T) {
	records := seedCommitmentRecords()
	pack := BuildActingPack(records, "desk-a", 0)
	if len(pack.Items) != 3 {
		t.Fatalf("desk-a pack items = %d", len(pack.Items))
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	// The pack's JSON may contain "discrepancy" and observation excerpts,
	// but no score-ish keys anywhere — the epistemic boundary holds on the
	// wire, not just in the struct's field list.
	for _, banned := range []string{`"scores"`, `"scored_at"`, `"scorer_model_id"`} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("acting pack carries score field %s: %s", banned, raw)
		}
	}
	// Honest feedback IS present: the confirmed act carries its
	// resolution's observation excerpt.
	var found bool
	for _, item := range pack.Items {
		if item.RecordID == "act-confirmed-1" {
			found = true
			if item.Discrepancy != DiscrepancyConfirmed || !strings.Contains(item.ObservationExcerpt, "right") {
				t.Fatalf("confirmed item lost feedback: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("desk-a pack missing act-confirmed-1")
	}
}

func TestActingPackIncludesAddressedActs(t *testing.T) {
	records := seedCommitmentRecords()
	pack := BuildActingPack(records, "desk-b", 0)
	ids := map[string]bool{}
	for _, item := range pack.Items {
		ids[item.RecordID] = true
	}
	// desk-b committed act-falsified-1 and is addressed by act-open-1.
	if !ids["act-falsified-1"] || !ids["act-open-1"] {
		t.Fatalf("desk-b pack ids = %v", ids)
	}
}

func TestSupervisionPackCarriesScores(t *testing.T) {
	pack := BuildSupervisionPack(seedCommitmentRecords(), "desk-b", 0)
	var found bool
	for _, item := range pack.Items {
		if item.RecordID == "act-falsified-1" {
			found = true
			if len(item.Scores) != 2 || item.Scores[0].ScorerModelID == "" {
				t.Fatalf("supervision pack lost scores: %+v", item)
			}
			if !item.Disagreement {
				t.Fatal("disagreement not preserved on supervision pack")
			}
		}
	}
	if !found {
		t.Fatal("supervision pack missing falsified act")
	}
}

func TestGateLearningClaimBindsScoredEvidence(t *testing.T) {
	scored := map[string]bool{"rec-1": true}
	got := GateLearningClaim([]string{"commitment://rec-1", "receipt-xyz"}, func(id string) bool { return scored[id] })
	if got != LearningClaimBacked {
		t.Fatalf("backed claim gated as %s", got)
	}
	got = GateLearningClaim([]string{"receipt-xyz"}, func(id string) bool { return scored[id] })
	if got != LearningClaimUnbacked {
		t.Fatalf("unbacked claim passed as %s", got)
	}
	// Canonical-id ref form resolves through the record_id segment.
	got = GateLearningClaim([]string{"choir.commitment_record:rec-1"}, func(id string) bool { return scored[id] })
	if got != LearningClaimBacked {
		t.Fatalf("canonical ref gated as %s", got)
	}
}
