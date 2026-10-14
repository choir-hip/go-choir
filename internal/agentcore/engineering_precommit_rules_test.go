package agentcore

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Engineering's rule precommits on the product path (failure modes R1-R12,
// types/commitment_rules.go): the reducer refuses malformed or late ones,
// stamps the assignment onto the record, and the terminal transition settles
// what it decides.

func rulePrecommitIntent(t *testing.T, local string, p float64, rule map[string]any) yaegikernel.StagedIntent {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"question": local, "resolver": "rule", "rule": rule,
		"distribution": map[string]float64{"yes": p, "no": 1 - p},
	})
	if err != nil {
		t.Fatal(err)
	}
	return yaegikernel.StagedIntent{LocalID: local, Kind: yaegikernel.IntentPrecommit, Precommit: string(body)}
}

// R2, R3 and R1 at the reducer's pre-commit check.
func TestEngineeringPrecommitCellValidation(t *testing.T) {
	engineering := ReductionScope{FromRole: agentprofile.Engineering}
	good := rulePrecommitIntent(t, "tests", 0.7, map[string]any{"kind": "tests_pass"})
	if err := validateCellIntents(engineering, []yaegikernel.StagedIntent{good}); err != nil {
		t.Fatalf("valid rule precommit refused: %v", err)
	}
	for name, in := range map[string]yaegikernel.StagedIntent{
		"no rule":      {LocalID: "x", Kind: yaegikernel.IntentPrecommit, Precommit: `{"question":"q","distribution":{"yes":0.5,"no":0.5},"resolver":"management"}`},
		"dogmatic":     rulePrecommitIntent(t, "d", 1, map[string]any{"kind": "tests_pass"}),
		"unknown kind": rulePrecommitIntent(t, "k", 0.5, map[string]any{"kind": "lines_changed"}),
		"no minutes":   rulePrecommitIntent(t, "m", 0.5, map[string]any{"kind": "freeze_within"}),
	} {
		if err := validateCellIntents(engineering, []yaegikernel.StagedIntent{in}); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	freeze := yaegikernel.StagedIntent{LocalID: "f", Kind: yaegikernel.IntentFreeze, BuildRecipeRef: "r", TestReceipts: []string{"t"}, DependencyToolchainRefs: []string{"d"}}
	if err := validateCellIntents(engineering, []yaegikernel.StagedIntent{good, freeze}); err == nil || !strings.Contains(err.Error(), "before the evidence") {
		t.Fatalf("precommit beside Freeze = %v, want refused", err)
	}
	// Only engineering's precommits are rule-settled.
	management := ReductionScope{FromRole: agentprofile.Management}
	if err := validateCellIntents(management, []yaegikernel.StagedIntent{good}); err == nil || !strings.Contains(err.Error(), "engineering's") {
		t.Fatalf("management rule precommit = %v, want refused", err)
	}
	plain := yaegikernel.StagedIntent{LocalID: "p", Kind: yaegikernel.IntentPrecommit, Precommit: `{"question":"q","distribution":{"yes":0.5,"no":0.5},"resolver":"research"}`}
	if err := validateCellIntents(management, []yaegikernel.StagedIntent{plain}); err != nil {
		t.Fatalf("management plain precommit refused: %v", err)
	}
}

// R9: only source files under workspace/platform count.
func TestCapsuleSourcePathsKeepsOnlySourceFiles(t *testing.T) {
	got := capsuleSourcePaths([]capsule.FileChange{
		{Path: "workspace/platform/frontend", Mode: os.ModeDir | 0o755},
		{Path: "workspace/platform/frontend/src/a.ts", Mode: 0o644},
		{Path: "workspace/platform/internal/x.go", Kind: capsule.ChangeDeleted, Mode: 0o644},
		{Path: "var/lib/artifact/release/bin/autoputer", Mode: 0o755},
		{Path: "tmp/gocache/ab", Mode: 0o644},
	})
	if strings.Join(got, ",") != "frontend/src/a.ts,internal/x.go" {
		t.Fatalf("source paths = %v", got)
	}
}

// Record creation through the reducer, then automatic settlement when the
// assignment is cancelled past its freeze bound (the timeout path).
func TestEngineeringPrecommitRecordedThenSettledAtTerminal(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-rule-precommit", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	run.Metadata[runMetadataModel] = "test-model-1"
	r := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &run, scope: ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID, FromRole: agentprofile.Engineering, ChannelID: run.AgentID,
		RunID: run.RunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CellID: "cell-eng-1",
	}}
	intents := []yaegikernel.StagedIntent{
		// R1: a model-supplied assignment and anchor are overwritten.
		rulePrecommitIntent(t, "freeze", 0.8, map[string]any{"kind": "freeze_within", "within_minutes": 1, "assignment_id": "assignment-elsewhere", "since": "2030-01-01T00:00:00Z"}),
		rulePrecommitIntent(t, "completes", 0.9, map[string]any{"kind": "completes"}),
		rulePrecommitIntent(t, "verify", 0.6, map[string]any{"kind": "verify_pass"}),
		rulePrecommitIntent(t, "paths", 0.9, map[string]any{"kind": "paths_confined", "paths": []string{"/workspace/platform/frontend/"}}),
	}
	for _, in := range intents {
		if _, err := r.commitEngineeringPrecommit(ctx, in); err != nil {
			t.Fatalf("precommit %s: %v", in.LocalID, err)
		}
	}
	listed, err := s.ListRulePrecommitRecords(ctx, seed.OwnerID, seed.ComputerID, assignment.AssignmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 4 {
		t.Fatalf("listed %d precommits, want 4", len(listed))
	}
	for _, rec := range listed {
		rule := rec.Precommit.Rule
		if rule.AssignmentID != assignment.AssignmentID || rule.Since != assignment.CreatedAt.UTC().Format(time.RFC3339Nano) ||
			rec.Addressee != "" || rec.Provenance.AgentID != run.AgentID || rec.Provenance.ModelID != "test-model-1" {
			t.Fatalf("precommit record not stamped by the runtime: rule=%+v addressee=%q provenance=%+v", rule, rec.Addressee, rec.Provenance)
		}
		if rule.Kind == types.PrecommitRulePathsConfined && strings.Join(rule.Paths, ",") != "frontend" {
			t.Fatalf("paths not normalized: %v", rule.Paths)
		}
	}
	// A replayed cell mints nothing new.
	if _, err := r.commitEngineeringPrecommit(ctx, intents[0]); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.ListRulePrecommitRecords(ctx, seed.OwnerID, seed.ComputerID, assignment.AssignmentID); len(again) != 4 {
		t.Fatalf("replay minted a duplicate: %d records", len(again))
	}

	// The assignment ends with no freeze (types' TestSettleRuleAtTerminal
	// pins the timeout wording past the bound).
	c1eCancel(t, rt, c1eGet(t, s, assignment), "deadline expired", false)

	all, err := s.ListCommitmentRecords(ctx, seed.OwnerID, seed.ComputerID, "", 500)
	if err != nil {
		t.Fatal(err)
	}
	verdicts := map[string]types.CommitmentRecord{}
	for _, rec := range all {
		if rec.Resolve != nil && rec.Provenance.AgentID == types.RuleScorerAgentID {
			verdicts[rec.Scores[0].ScorerModelID] = rec
		}
	}
	want := map[string]string{"rule:freeze_within": "no", "rule:completes": "no", "rule:verify_pass": "no", "rule:paths_confined": "unresolved"}
	for scorer, answer := range want {
		got, ok := verdicts[scorer]
		if !ok || got.Resolve.Verdict != answer {
			t.Fatalf("%s settled %+v, want %s", scorer, got.Resolve, answer)
		}
	}
	if excerpt := verdicts["rule:freeze_within"].Observation.Excerpt; !strings.Contains(excerpt, "no Freeze") || !strings.Contains(excerpt, "deadline expired") {
		t.Fatalf("freeze_within excerpt = %q, want no Freeze and the reason", excerpt)
	}
	if verdicts["rule:completes"].Discrepancy != types.DiscrepancyContradicted {
		t.Fatalf("completes at p=0.9 that did not complete = %s", verdicts["rule:completes"].Discrepancy)
	}

	sayDo := types.RulePrecommitSayDo(all, "engineering:")
	if o := sayDo.Overall; o.Committed != 4 || o.No != 3 || o.Unresolved != 1 || o.Open != 0 {
		t.Fatalf("say–do = %+v", o)
	}

	// R2: a precommit on an ended assignment is refused.
	late := rulePrecommitIntent(t, "late", 0.5, map[string]any{"kind": "completes"})
	late.LocalID = "late"
	if _, err := r.commitEngineeringPrecommit(ctx, late); err == nil {
		t.Fatal("precommit on a terminal assignment accepted")
	}
}
