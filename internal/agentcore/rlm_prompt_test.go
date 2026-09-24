package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func testEngineeringRun() *types.RunRecord {
	return &types.RunRecord{
		RunID:        "run-rlm-prompt",
		AgentProfile: agentprofile.Engineering,
		OwnerID:      "user-alice",
	}
}

// TestEngineeringPromptSwitchesToSealedGoUnderRLM proves the model-facing schema
// cutover: under actuator=rlm the Engineering prompt teaches capsule_go_eval plus
// the choir package; under tools the fallback exposes only one-shot Go eval
// and no in-cell carrier.
func TestEngineeringPromptSwitchesToSealedGoUnderRLM(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rlmPrompt, err := rt.systemPromptForRun(testEngineeringRun())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"capsule_go_eval", "choir.Inbox", "choir.Spawn", "choir.Complete", "No other JSON tool exists in this mode"} {
		if !strings.Contains(rlmPrompt, want) {
			t.Errorf("RLM prompt missing %q", want)
		}
	}
	for _, retired := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir"} {
		if strings.Contains(rlmPrompt, retired) {
			t.Errorf("RLM prompt still presents retired JSON tool %q", retired)
		}
	}

	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorTools)
	toolsPrompt, err := rt.systemPromptForRun(testEngineeringRun())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolsPrompt, "The tools-actuator fallback exposes exactly one JSON tool: capsule_go_eval.") {
		t.Error("tools prompt does not describe the eval-only catalog")
	}
	if strings.Contains(toolsPrompt, "choir.Spawn") {
		t.Error("tools prompt leaks RLM orchestration surface")
	}
	// The tools-actuator fallback is eval-only: none of the retired JSON
	// tool names may be presented as reachable.
	for _, retired := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "update_coagent", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "record_assignment_result"} {
		if strings.Contains(toolsPrompt, retired) {
			t.Errorf("tools prompt still names retired tool %q as reachable", retired)
		}
	}
}

func TestRLMPromptOmitsFreezeMandateForDocumentTrajectory(t *testing.T) {
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rt, _ := testRuntime(t)
	run := testEngineeringRun()
	run.ComputerID = "autoputer-test"
	run.TrajectoryID = "trajectory-document-cast"
	const mandate = "Freeze the capsule diff with choir.Freeze."

	prompt, err := rt.systemPromptForRun(run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, mandate) {
		t.Fatalf("document-cast prompt mandates Freeze: %q", prompt)
	}
}

// TestRLMPromptOmitsRetiredToolNames guards the same invariant on the RLM
// overlay: the sealed-Go prompt must never present a retired JSON tool name
// as callable.
func TestRLMPromptOmitsRetiredToolNames(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rlmPrompt, err := rt.systemPromptForRun(testEngineeringRun())
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "update_coagent", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "record_assignment_result"} {
		if strings.Contains(rlmPrompt, retired) {
			t.Errorf("RLM prompt still names retired tool %q", retired)
		}
	}
}

// TestEngineeringPromptIsModelIndependent is the standing one-prompt guard: the
// assembled system prompt must be byte-identical for two runs that differ
// only in model selection metadata. A per-model fork anywhere in prompt
// assembly fails this test.
func TestEngineeringPromptIsModelIndependent(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)

	runA := testEngineeringRun()
	runA.Metadata = map[string]any{"model": "deepseek-v4.1-flash", "llm_policy_overlay_id": "roster-a"}
	runB := testEngineeringRun()
	runB.Metadata = map[string]any{"model": "muse-spark-1.3-contributor-free", "llm_policy_overlay_id": "roster-b"}

	promptA, err := rt.systemPromptForRun(runA)
	if err != nil {
		t.Fatal(err)
	}
	promptB, err := rt.systemPromptForRun(runB)
	if err != nil {
		t.Fatal(err)
	}
	if promptA != promptB {
		t.Fatal("system prompt differs across model selections; a per-model branch exists in prompt assembly")
	}
}
