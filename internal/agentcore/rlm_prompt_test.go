package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func testCoSuperRun() *types.RunRecord {
	return &types.RunRecord{
		RunID:        "run-rlm-prompt",
		AgentProfile: agentprofile.CoSuper,
		OwnerID:      "user-alice",
	}
}

// TestCoSuperPromptSwitchesToSealedGoUnderRLM proves the model-facing schema
// cutover: under actuator=rlm the CoSuper prompt teaches capsule_go_eval plus
// the choir package and retracts the JSON file/exec tools; under tools the
// legacy prompt is byte-identical.
func TestCoSuperPromptSwitchesToSealedGoUnderRLM(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rlmPrompt, err := rt.systemPromptForRun(testCoSuperRun())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"capsule_go_eval", "choir.Inbox", "choir.Spawn", "choir.Complete", "No other JSON tool exists in this mode"} {
		if !strings.Contains(rlmPrompt, want) {
			t.Errorf("RLM prompt missing %q", want)
		}
	}
	if strings.Contains(rlmPrompt, "The tool catalog is the complete authority: capsule_exec") {
		t.Error("RLM prompt still carries the legacy JSON tool catalog")
	}

	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorTools)
	toolsPrompt, err := rt.systemPromptForRun(testCoSuperRun())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolsPrompt, "The tool catalog is the complete authority: capsule_exec") {
		t.Error("tools prompt lost the legacy catalog")
	}
	if strings.Contains(toolsPrompt, "choir.Spawn") {
		t.Error("tools prompt leaks RLM orchestration surface")
	}
	// The tools-actuator fallback desk is capsule effects only: none of the
	// five retired overlay names may be presented as reachable.
	for _, retired := range []string{"update_coagent", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "record_assignment_result"} {
		if strings.Contains(toolsPrompt, retired) {
			t.Errorf("tools prompt still names retired tool %q as reachable", retired)
		}
	}
}

// TestRLMPromptOmitsRetiredToolNames guards the same invariant on the RLM
// overlay: the sealed-Go prompt must never present a retired JSON tool name
// as callable.
func TestRLMPromptOmitsRetiredToolNames(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rlmPrompt, err := rt.systemPromptForRun(testCoSuperRun())
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"update_coagent", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "record_assignment_result"} {
		if strings.Contains(rlmPrompt, retired) {
			t.Errorf("RLM prompt still names retired tool %q", retired)
		}
	}
}

// TestCoSuperPromptIsModelIndependent is the standing one-prompt guard: the
// assembled system prompt must be byte-identical for two runs that differ
// only in model selection metadata. A per-model fork anywhere in prompt
// assembly fails this test.
func TestCoSuperPromptIsModelIndependent(t *testing.T) {
	rt := &Runtime{}
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)

	runA := testCoSuperRun()
	runA.Metadata = map[string]any{"model": "deepseek-v4.1-flash", "llm_policy_overlay_id": "roster-a"}
	runB := testCoSuperRun()
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
