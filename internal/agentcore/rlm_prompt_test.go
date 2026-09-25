package agentcore

import (
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
