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
	for _, want := range []string{"capsule_go_eval", "choir.Inbox", "choir.Spawn", "choir.Complete", "do not exist in this mode"} {
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
}
