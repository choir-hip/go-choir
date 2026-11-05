package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
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
	// A research child's line, written on its parent run (B2).
	appendServed(t, s, research, types.ModelServedRecord{Role: researchChildRole, SelectedProvider: "openrouter", SelectedModel: "flint",
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
	if m, ok := provenanceModel(provenanceRole(got, researchChildRole), "flint"); !ok || m.Calls != 5 {
		t.Fatalf("research children = %+v", provenanceRole(got, researchChildRole))
	}
	for model, calls := range map[string]int{"deepseek-v4.1-flash": 2, "gpt-6-luna": 3, "iceman": 1} {
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
	if got.Runs < 4 {
		t.Fatalf("runs = %d, want texture, research, engineering and its caster", got.Runs)
	}
}

// B2 (review r1): a research run whose two children served two different
// models, one after a quota switch, shows the parent's model, both
// children's, and the switch, through the real child loop.
func TestDocumentModelProvenanceReachesResearchChildren(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	p := &quotaThenOK{}
	rt.provider, rt.researchChildCalls = p, &callSpacer{}
	const owner, docID, trajectory = "owner-children", "doc-children", "trajectory-children"
	research := provenanceRun(t, s, types.RunRecord{RunID: "run-children-research", AgentID: "research:" + docID, ChannelID: docID, OwnerID: owner,
		TrajectoryID: trajectory, AgentProfile: "research", AgentRole: "research"})
	appendServed(t, s, research, types.ModelServedRecord{SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash",
		Models: []types.ModelServedUsage{{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 2}}})
	parent := toolregistry.ExecutionContext{OwnerID: owner, AgentID: research.AgentID, RunID: research.RunID}
	spec := yaegikernel.ResearchChild{Objective: "alpha"}
	first := rt.runResearchChild(ctx, parent, researchLineage{ownerID: owner, selection: provideriface.LLMSelection{Provider: "openrouter", Model: "flint"}}, spec, researchFanOut{})
	second := rt.runResearchChild(ctx, parent, researchLineage{ownerID: owner, selection: provideriface.LLMSelection{Provider: "synthetic", Model: "m"}}, spec, researchFanOut{})
	if first.Error != "" || second.Error != "" || strings.Contains(second.Findings, "synthetic") {
		t.Fatalf("children = %+v / %+v", first, second)
	}
	got, err := rt.DocumentModelProvenance(objectgraph.WithReader(ctx, objectgraph.WriterOwner), owner, docID, []string{trajectory})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(got, "", "  ")
	t.Logf("provenance:\n%s", raw)
	if _, ok := provenanceModel(provenanceRole(got, "research"), "deepseek-v4.1-flash"); !ok {
		t.Fatalf("parent model missing")
	}
	children := provenanceRole(got, researchChildRole)
	if m, ok := provenanceModel(children, "flint"); !ok || m.Calls != 1 {
		t.Fatalf("first child's model missing: %+v", children)
	}
	if len(children) != 2 {
		t.Fatalf("children = %+v, want flint and the chain model the second child switched to", children)
	}
	if len(got.Switches) != 1 || got.Switches[0].Role != researchChildRole || got.Switches[0].FromProvider != "synthetic" ||
		!strings.HasPrefix(got.Switches[0].Reason, "provider_quota") {
		t.Fatalf("switches = %+v, want the second child's quota switch", got.Switches)
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

// N5 (review r1): the judge's served line names the provider the response
// reported, and the transport's provider only when it reported none.
func TestJevServedLineTakesTheProviderFromTheCall(t *testing.T) {
	rt, s := testRuntime(t)
	run := provenanceRun(t, s, types.RunRecord{RunID: "run-jev-provider", AgentID: "research:doc-jev", ChannelID: "doc-jev", OwnerID: "owner-jev", AgentProfile: "research", AgentRole: "research"})
	ctx := context.Background()
	rt.recordJevCostFor(ctx, run.OwnerID, run.RunID, run.AgentID, "system1", "jev-judge", "fireworks", jevUsage{InputTokens: 10, OutputTokens: 1}, 1)
	rt.recordJevCostFor(ctx, run.OwnerID, run.RunID, run.AgentID, "system1", "jev-judge", "", jevUsage{InputTokens: 10, OutputTokens: 1}, 1)
	evs, err := s.ListRunEventsOfKind(objectgraph.WithReader(ctx, objectgraph.WriterOwner), run.OwnerID, run.RunID, types.EventModelServed)
	if err != nil || len(evs) != 2 {
		t.Fatalf("served lines = %d, %v", len(evs), err)
	}
	var providers []string
	for _, ev := range evs {
		var rec types.ModelServedRecord
		if err := json.Unmarshal(ev.Payload, &rec); err != nil || len(rec.Models) != 1 || rec.Role != types.ModelServedRoleSystem1 {
			t.Fatalf("line = %s, %v", ev.Payload, err)
		}
		providers = append(providers, rec.Models[0].Provider)
	}
	if providers[0] != "fireworks" || providers[1] != jevTransportProvider {
		t.Fatalf("providers = %v, want the reported one, then the transport's", providers)
	}
}
