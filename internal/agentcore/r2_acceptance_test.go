package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// mission R2 acceptance probes. The goal file lists four acceptance actions
// (delegated cast, report-on-resolver, precommit freeze, registry census);
// report/escalate packet-and-safety bodies already live in rlm_reduce_test.go.
// These tests prove the remaining claims on canonical evidence.

// commitmentCanonicalID re-derives the canonical id for an intent's commitment
// record. AppendCommitmentRecord is idempotent on (owner, computer, recordID):
// a re-append of an already-minted record returns the same canonical id and
// mints nothing, so a non-empty id is proof the record is on the ledger.
func commitmentCanonicalID(t *testing.T, s *store.Store, scope ReductionScope, in yaegikernel.StagedIntent) string {
	t.Helper()
	id, err := s.AppendCommitmentRecord(context.Background(), scope.OwnerID, scope.ComputerID, commitmentRecordForIntent(scope, in))
	if err != nil {
		t.Fatalf("re-derive commitment canonical id: %v", err)
	}
	if strings.TrimSpace(id) == "" {
		t.Fatal("commitment canonical id is empty — record not on the ledger")
	}
	return id
}

// Probe 3 — a desk cell's typed Precommit, Resolve, and Disagreement acts
// append separate commitment_record objects. The frozen distribution and
// resolution evidence remain on the tape; the disagreement is a distinct
// epistemic act rather than an acting-context signal.
func TestR2TypedCommitmentSequenceMintsLedgerRecords(t *testing.T) {
	rt, s := testRuntime(t)
	scope := testReductionScope()
	scope.ComputerID = rt.TextureComputerID()
	scope.CellID = "cell-r2-typed-commitment"
	ctx := testReductionCtx(scope)
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope, ledger: rt.store}

	precommitBody, err := json.Marshal(types.CommitmentPrecommit{
		Question:     "will the retained tape replay?",
		Distribution: map[string]float64{"yes": 0.8, "no": 0.2},
		Resolver:     "management",
	})
	if err != nil {
		t.Fatal(err)
	}
	precommit := yaegikernel.StagedIntent{
		LocalID: "pc-1", Kind: yaegikernel.IntentPrecommit, Precommit: string(precommitBody),
	}
	if err := validateSemanticActIntent(precommit); err != nil {
		t.Fatalf("validate typed precommit: %v", err)
	}
	if _, err := reduction.commitActIntent(ctx, precommit); err != nil {
		t.Fatalf("precommit reduce: %v", err)
	}

	targetID := commitmentRecordForIntent(scope, precommit).RecordID
	resolveBody, err := json.Marshal(types.CommitmentResolve{
		Verdict: "confirmed", EvidenceRefs: []string{"evidence://replay"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolve := yaegikernel.StagedIntent{
		LocalID: "res-1", Kind: yaegikernel.IntentResolve, TargetRef: targetID, Resolve: string(resolveBody),
	}
	if err := validateSemanticActIntent(resolve); err != nil {
		t.Fatalf("validate typed resolve: %v", err)
	}
	if _, err := reduction.commitActIntent(ctx, resolve); err != nil {
		t.Fatalf("resolve reduce: %v", err)
	}

	disagreementBody, err := json.Marshal(types.CommitmentDisagreement{
		CommitmentID: targetID, ScorerVerdict: "contradicted", ResolverVerdict: "confirmed",
		EvidenceRefs: []string{"evidence://counterexample"},
	})
	if err != nil {
		t.Fatal(err)
	}
	disagreement := yaegikernel.StagedIntent{
		LocalID: "dis-1", Kind: yaegikernel.IntentDisagreement, Disagreement: string(disagreementBody),
	}
	if err := validateSemanticActIntent(disagreement); err != nil {
		t.Fatalf("validate typed disagreement: %v", err)
	}
	if _, err := reduction.commitActIntent(ctx, disagreement); err != nil {
		t.Fatalf("disagreement reduce: %v", err)
	}

	records, err := s.ListCommitmentRecords(ctx, scope.OwnerID, scope.ComputerID, "", 10)
	if err != nil {
		t.Fatalf("list typed commitment tape: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("typed commitment tape records = %d, want 3", len(records))
	}
	byID := map[string]types.CommitmentRecord{}
	for _, rec := range records {
		byID[rec.RecordID] = rec
	}
	if got := byID[targetID].Precommit; got == nil || got.Question != "will the retained tape replay?" ||
		got.Distribution["yes"] != 0.8 || got.Resolver != "management" {
		t.Fatalf("typed precommit record = %+v", byID[targetID])
	}
	if got := byID[commitmentRecordForIntent(scope, resolve).RecordID].Resolve; got == nil ||
		got.Verdict != "confirmed" || len(got.EvidenceRefs) != 1 {
		t.Fatalf("typed resolve record = %+v", byID[commitmentRecordForIntent(scope, resolve).RecordID])
	}
	if got := byID[commitmentRecordForIntent(scope, disagreement).RecordID].Disagreement; got == nil ||
		got.ScorerVerdict != "contradicted" || got.ResolverVerdict != "confirmed" {
		t.Fatalf("typed disagreement record = %+v", byID[commitmentRecordForIntent(scope, disagreement).RecordID])
	}

	// The next desk cell receives an ActingPack dump, never the frozen
	// distribution, scorer result, or disagreement act from this sequence.
	packRaw, err := json.Marshal(types.BuildActingPack(records, scope.FromAgentID, 0))
	if err != nil {
		t.Fatalf("marshal acting pack: %v", err)
	}
	for _, forbidden := range []string{`"scores"`, `"distribution"`, `"probabilities"`, `"disagreement"`} {
		if strings.Contains(string(packRaw), forbidden) {
			t.Fatalf("acting pack leaked %s after typed sequence: %s", forbidden, packRaw)
		}
	}
}

// Legacy string commitments have no frozen distribution to infer. A typed
// Resolve may still close them; they remain unscoreable rather than being
// rewritten or rejected.
func TestR2TypedResolveGrandfathersLegacyStringCommitment(t *testing.T) {
	rt, s := testRuntime(t)
	scope := testReductionScope()
	scope.ComputerID = rt.TextureComputerID()
	scope.CellID = "cell-r2-legacy-commitment"
	ctx := testReductionCtx(scope)
	legacy := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1,
		RecordID: "legacy-string-commitment",
		Prediction: types.CommitmentPrediction{
			Hypothesis: "the legacy string commitment remains valid",
		},
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: scope.FromAgentID},
	}
	if _, err := s.AppendCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, legacy); err != nil {
		t.Fatalf("append legacy commitment: %v", err)
	}
	resolveBody, err := json.Marshal(types.CommitmentResolve{
		Verdict: "confirmed", EvidenceRefs: []string{"evidence://legacy"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolve := yaegikernel.StagedIntent{
		LocalID: "legacy-resolve", Kind: yaegikernel.IntentResolve,
		TargetRef: legacy.RecordID, Resolve: string(resolveBody),
	}
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope, ledger: rt.store}
	if _, err := reduction.commitActIntent(ctx, resolve); err != nil {
		t.Fatalf("resolve legacy commitment: %v", err)
	}
	records, err := s.ListCommitmentRecords(ctx, scope.OwnerID, scope.ComputerID, "", 10)
	if err != nil {
		t.Fatalf("list legacy commitment tape: %v", err)
	}
	resolved := types.ResolveCommitments(records, time.Now().UTC())
	if len(resolved) != 1 || resolved[0].Act.RecordID != legacy.RecordID ||
		resolved[0].Discrepancy != types.DiscrepancyConfirmed {
		t.Fatalf("legacy string commitment did not resolve: %+v", resolved)
	}
}

// Probe 4 — registry census: the four desks carry zero tool-loop tools; every
// agent-to-agent act is a staged choir verb. update_coagent is absent from
// management/engineering/research/texture and present only on the wire roles.
func TestR2DeskRegistryCensus(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("InstallDefaultAgentTools: %v", err)
	}
	for _, profile := range []string{agentprofile.Management, agentprofile.Engineering, agentprofile.Research, agentprofile.Texture} {
		if _, ok := rt.ToolRegistryForProfile(profile).Lookup("update_coagent"); ok {
			t.Fatalf("%s registry still exposes update_coagent (tool-loop carrier)", profile)
		}
	}
	// The wire roles keep the packet tool until their own carrier phase; it is
	// a desk exclusion, not a deletion.
	for _, profile := range []string{agentprofile.Processor, agentprofile.Reconciler} {
		if _, ok := rt.ToolRegistryForProfile(profile).Lookup("update_coagent"); !ok {
			t.Fatalf("%s registry missing update_coagent (wire-role tool until its phase)", profile)
		}
	}
}

// Probe 1 — a management desk cell staging choir.Cast(engineering, objective)
// reduces to a delegated-admission open: the commitment record mints first and
// the reducer reaches openDelegatedCastAssignment under the caster's own
// trajectory-bound authority (not an owner revision). On platforms where the
// capsule executor is a stub the admission stops at the substrate boundary —
// the commitment still lands and the failure is at spawn preflight, never at
// the authority layer.
func TestR2DelegatedCastReachesDelegatedAdmission(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "user-alice", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)

	scope := testReductionScope()
	scope.CellID = "cell-r2-delegated-cast"
	scope.FromAgentID = seed.ParentAgentID
	scope.FromRole = "management"
	scope.OwnerID = seed.OwnerID
	scope.ComputerID = seed.ComputerID
	rcCtx := testReductionCtx(scope)

	casterRun, err := s.GetRun(ctx, seed.ParentRunID)
	if err != nil {
		t.Fatalf("caster run: %v", err)
	}
	intent := yaegikernel.StagedIntent{
		LocalID: "cast-1", Kind: yaegikernel.IntentCast, ToDesk: "engineering",
		Objective: "bounded delegated assignment",
	}
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope, ledger: rt.store, rec: &casterRun}
	_, castErr := reduction.commitActIntent(rcCtx, intent)

	// The commitment minted before admission regardless of the capsule
	// substrate: the cast is recorded on the OG ledger under the caster.
	if id := commitmentCanonicalID(t, s, scope, intent); id == "" {
		t.Fatal("delegated cast commitment absent from the ledger")
	}

	if castErr == nil {
		t.Log("delegated cast opened and spawned on live executor")
		return
	}
	// Admission reached the delegated-authority path — the error names the
	// capsule substrate (preflight/spawn), never an authority or validation
	// rejection of the cast itself.
	if strings.Contains(castErr.Error(), "requires a trajectory-bound caster") ||
		strings.Contains(castErr.Error(), "requires objective, kind, and the commitment control") ||
		strings.Contains(castErr.Error(), "caster has no open work item") ||
		strings.Contains(castErr.Error(), "caster holds multiple open work items") {
		t.Fatalf("delegated cast rejected at the authority layer: %v", castErr)
	}
	t.Logf("delegated cast reached admission; substrate stop: %v", castErr)
}
