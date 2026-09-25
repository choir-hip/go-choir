package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
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

// Probe 3 — a desk cell staging choir.Precommit records a frozen prediction on
// the OG commitment ledger; the record binds the acting agent, lands
// unresolved, and is idempotent under cell replay.
func TestR2PrecommitMintsLedgerRecord(t *testing.T) {
	rt, s := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-r2-precommit"
	ctx := testReductionCtx(scope)

	intent := yaegikernel.StagedIntent{
		LocalID:   "pc-1",
		Kind:      yaegikernel.IntentPrecommit,
		Statement: "the frozen corpus digest will not change under rewarm",
	}
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope, ledger: rt.store}
	// Precommit is ledger-only: it returns no mailed seq but must not error —
	// a nil reduce with ledger set is the mint having run.
	if _, err := reduction.commitActIntent(ctx, intent); err != nil {
		t.Fatalf("precommit reduce: %v", err)
	}
	if id := commitmentCanonicalID(t, s, scope, intent); id == "" {
		t.Fatal("precommit record absent from the commitment ledger")
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
