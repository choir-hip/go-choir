package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ways the model provenance could be wrong, written before the code
// (docs/problems/model-provenance-unwired-2026-11-05.md):
//   - it shows the policy's prediction, not what answered;
//   - a model a run switched to mid-run (quota, pool fallback) is missing,
//     or the model it left is shown as if it answered;
//   - research children, engineering runs and the Claude delegate are not
//     reached from the document;
//   - another document's runs leak in;
//   - a document served by one model shows more than that one.

func appendServed(t *testing.T, s *store.Store, run types.RunRecord, rec types.ModelServedRecord) {
	t.Helper()
	rec.Schema = types.ModelServedSchema
	if rec.Purpose == "" {
		rec.Purpose = types.ModelServedPurposeToolLoop
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	ev := &types.EventRecord{EventID: uuid.New().String(), RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID,
		TrajectoryID: run.TrajectoryID, Timestamp: time.Now().UTC(), Kind: types.EventModelServed, Payload: raw}
	if err := s.AppendEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
}

func provenanceRun(t *testing.T, s *store.Store, rec types.RunRecord) types.RunRecord {
	t.Helper()
	now := time.Now().UTC()
	rec.State, rec.CreatedAt, rec.UpdatedAt, rec.FinishedAt = types.RunCompleted, now, now, &now
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func provenanceRole(p types.ModelProvenance, role string) []types.ModelProvenanceModel {
	for _, r := range p.Roles {
		if r.Role == role {
			return r.Models
		}
	}
	return nil
}

func provenanceModel(models []types.ModelProvenanceModel, model string) (types.ModelProvenanceModel, bool) {
	for _, m := range models {
		if m.Model == model {
			return m, true
		}
	}
	return types.ModelProvenanceModel{}, false
}

// A trajectory whose research run fails over from one model to another and
// whose engineering run uses the Claude delegate shows all of them, by role.
func TestDocumentModelProvenanceShowsSwitchesChildrenEngineeringAndDelegate(t *testing.T) {
	ctx := context.Background()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-provenance", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, engRun := c1eOpenBound(t, s, seed, 0)
	report := pinnedAssignmentReport(assignment, types.EngineeringResultCompleted)
	report.DelegateUse = &types.EngineeringDelegateUse{Available: true, SourcePatch: true, Calls: 2, Succeeded: 2, Model: "claude-sonnet-5-5"}
	req := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "engineering-report:" + assignment.AssignmentID + ":" + report.ReportID,
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
	}
	req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
	if _, err := s.RecordEngineeringAssignmentReport(ctx, req); err != nil {
		t.Fatalf("record report: %v", err)
	}
	owner, docID, trajectory := seed.OwnerID, seed.DocID, seed.TrajectoryID

	texture := provenanceRun(t, s, types.RunRecord{RunID: "run-texture", AgentID: "texture:" + docID, ChannelID: docID, OwnerID: owner,
		TrajectoryID: trajectory, AgentProfile: "texture", AgentRole: "texture",
		Metadata: map[string]any{"llm_policy_source": "default_pool", "llm_assignment": "pool"}})
	research := provenanceRun(t, s, types.RunRecord{RunID: "run-research", AgentID: "research:" + docID, ChannelID: docID, OwnerID: owner,
		TrajectoryID: trajectory, AgentProfile: "research", AgentRole: "research"})
	child := provenanceRun(t, s, types.RunRecord{RunID: "run-research-child", AgentID: "research-child-1", ChannelID: "research-child-1", OwnerID: owner,
		RequestedByRunID: research.RunID, AgentProfile: "research", AgentRole: "research"})
	other := provenanceRun(t, s, types.RunRecord{RunID: "run-other-doc", AgentID: "texture:other", ChannelID: "other-doc", OwnerID: owner,
		TrajectoryID: "trajectory-other", AgentProfile: "texture", AgentRole: "texture"})

	appendServed(t, s, texture, types.ModelServedRecord{SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash",
		Models: []types.ModelServedUsage{{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 4, InputTokens: 4000, OutputTokens: 400}}})
	// The research run answered twice on deepseek, then the pool failed it
	// over to luna, and a quota switch moved it to iceman.
	appendServed(t, s, research, types.ModelServedRecord{SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash",
		Models: []types.ModelServedUsage{
			{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 2},
			{Provider: "chatgpt", Model: "gpt-6-luna", Calls: 3},
			{Provider: "openrouter", Model: "iceman", Calls: 1},
		},
		Switches: []types.ModelSwitch{
			{FromProvider: "opencode-go", FromModel: "deepseek-v4.1-flash", ToProvider: "chatgpt", ToModel: "gpt-6-luna", Reason: "model_failover"},
			{FromProvider: "chatgpt", FromModel: "gpt-6-luna", ToProvider: "openrouter", ToModel: "iceman", Reason: "provider_quota"},
		}})
	appendServed(t, s, child, types.ModelServedRecord{SelectedProvider: "openrouter", SelectedModel: "flint",
		Models: []types.ModelServedUsage{{Provider: "openrouter", Model: "flint", Calls: 5}}})
	appendServed(t, s, engRun, types.ModelServedRecord{SelectedProvider: "chatgpt", SelectedModel: "gpt-6",
		Models: []types.ModelServedUsage{{Provider: "chatgpt", Model: "gpt-6", Calls: 7}}})
	appendServed(t, s, other, types.ModelServedRecord{Models: []types.ModelServedUsage{{Provider: "x", Model: "leaked-model", Calls: 1}}})
	// A decision-model (System 1) call made for the research run.
	appendServed(t, s, research, types.ModelServedRecord{Purpose: "system1", Role: types.ModelServedRoleSystem1,
		Models: []types.ModelServedUsage{{Provider: "openrouter", Model: "jev-judge", Calls: 1}}})

	got, err := rt.DocumentModelProvenance(objectgraph.WithReader(ctx, objectgraph.WriterOwner), owner, docID, []string{trajectory})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(got, "", "  ")
	t.Logf("provenance:\n%s", raw)

	if m, ok := provenanceModel(provenanceRole(got, "texture"), "deepseek-v4.1-flash"); !ok || m.Calls != 4 || m.InputTokens != 4000 || len(provenanceRole(got, "texture")) != 1 {
		t.Fatalf("texture = %+v", provenanceRole(got, "texture"))
	}
	researchModels := provenanceRole(got, "research")
	for model, calls := range map[string]int{"deepseek-v4.1-flash": 2, "gpt-6-luna": 3, "iceman": 1, "flint": 5} {
		if m, ok := provenanceModel(researchModels, model); !ok || m.Calls != calls {
			t.Fatalf("research %s = %+v (found %v); all research = %+v", model, m, ok, researchModels)
		}
	}
	if m, _ := provenanceModel(researchModels, "gpt-6-luna"); len(m.Via) != 1 || m.Via[0] != "model_failover" {
		t.Fatalf("luna via = %v, want model_failover", m.Via)
	}
	if m, _ := provenanceModel(researchModels, "iceman"); len(m.Via) != 1 || m.Via[0] != "provider_quota" {
		t.Fatalf("iceman via = %v, want provider_quota", m.Via)
	}
	if len(got.Switches) != 2 || got.Switches[0].Role != "research" || got.Switches[0].RunID != research.RunID {
		t.Fatalf("switches = %+v", got.Switches)
	}
	eng := provenanceRole(got, "engineering")
	if m, ok := provenanceModel(eng, "gpt-6"); !ok || m.Calls != 7 || m.Delegate {
		t.Fatalf("engineering model = %+v", eng)
	}
	if m, ok := provenanceModel(eng, "claude-sonnet-5-5"); !ok || !m.Delegate || m.Calls != 2 || m.Provider != types.ModelProviderClaudeDelegate {
		t.Fatalf("engineering delegate = %+v", eng)
	}
	if m, ok := provenanceModel(provenanceRole(got, types.ModelServedRoleSystem1), "jev-judge"); !ok || m.Calls != 1 {
		t.Fatalf("system1 = %+v", provenanceRole(got, types.ModelServedRoleSystem1))
	}
	for _, r := range got.Roles {
		if _, leaked := provenanceModel(r.Models, "leaked-model"); leaked {
			t.Fatalf("another document's run leaked into role %s", r.Role)
		}
	}
	// The management turn that cast the engineering run is in the lineage
	// (it recorded nothing, so it shows no model).
	if got.Runs < 5 {
		t.Fatalf("runs = %d, want texture, research, child, engineering and its caster", got.Runs)
	}
}

// A document whose runs used only one model shows exactly that one.
func TestDocumentModelProvenanceOneModelShowsExactlyOne(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	const owner, docID, trajectory = "owner-one-model", "doc-one-model", "trajectory-one-model"
	for _, rec := range []types.RunRecord{
		{RunID: "run-one-texture", AgentID: "texture:" + docID, ChannelID: docID, OwnerID: owner, TrajectoryID: trajectory, AgentProfile: "texture", AgentRole: "texture"},
		{RunID: "run-one-research", AgentID: "research:" + docID, ChannelID: docID, OwnerID: owner, TrajectoryID: trajectory, AgentProfile: "research", AgentRole: "research"},
	} {
		run := provenanceRun(t, s, rec)
		appendServed(t, s, run, types.ModelServedRecord{SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash",
			Models: []types.ModelServedUsage{{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 3}}})
	}
	got, err := rt.DocumentModelProvenance(objectgraph.WithReader(ctx, objectgraph.WriterOwner), owner, docID, []string{trajectory})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range got.Roles {
		for _, m := range r.Models {
			seen[m.Provider+"/"+m.Model] = true
			if m.Calls != 3 || m.Runs != 1 {
				t.Fatalf("role %s model %+v", r.Role, m)
			}
		}
	}
	if len(seen) != 1 || !seen["opencode-go/deepseek-v4.1-flash"] || len(got.Switches) != 0 || got.Runs != 2 || got.RunsServed != 2 {
		t.Fatalf("provenance = %+v, want exactly deepseek-v4.1-flash", got)
	}
}

// The runtime's run emitter writes the tool loop's model.served line to the
// tape, even once the run's context is cancelled.
func TestRunEmitterPersistsModelServedAfterCancel(t *testing.T) {
	rt, s := testRuntime(t)
	run := provenanceRun(t, s, types.RunRecord{RunID: "run-emit-served", AgentID: "texture:doc-emit", ChannelID: "doc-emit", OwnerID: "owner-emit", AgentProfile: "texture", AgentRole: "texture"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var tally types.ModelServedTally
	tally.Note("chatgpt", "gpt-6-luna", "", 10, 2, 0)
	raw, _ := json.Marshal(tally.Record(types.ModelServedPurposeToolLoop))
	rt.runEventEmitter(ctx, &run)(types.EventModelServed, "", raw)
	evs, err := s.ListRunEventsOfKind(objectgraph.WithReader(context.Background(), objectgraph.WriterOwner), run.OwnerID, run.RunID, types.EventModelServed)
	if err != nil || len(evs) != 1 {
		t.Fatalf("served lines = %d, %v", len(evs), err)
	}
}
