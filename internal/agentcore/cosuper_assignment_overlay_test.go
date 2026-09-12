package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestOverlayIDNamedInObjective(t *testing.T) {
	for _, tc := range []struct {
		name      string
		objective string
		want      string
	}{
		{"roster tell", "ROSTER-V1 open exactly one implementation assignment with model_policy_overlay_id=p5-deepseek-v41-flash. Inside the capsule", "p5-deepseek-v41-flash"},
		{"spaced assignment", "with model_policy_overlay_id = p5-chatgpt-g56luna for the run", "p5-chatgpt-g56luna"},
		{"quoted", `naming model_policy_overlay_id="p5-x" here`, "p5-x"},
		{"bare mention is not a naming", "pass it as the structured model_policy_overlay_id parameter", ""},
		{"absent", "plain objective with no overlay reference", ""},
		{"malformed skipped", "model_policy_overlay_id= then model_policy_overlay_id=p5-good", "p5-good"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := overlayIDNamedInObjective(tc.objective); got != tc.want {
				t.Fatalf("overlayIDNamedInObjective = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAssignedCoSuperOpenRefusesProseOnlyOverlay(t *testing.T) {
	rt, _ := testRuntime(t)
	req := StartAssignedCoSuperRequest{
		Objective:        "ROSTER-V1 open exactly one implementation assignment with model_policy_overlay_id=p5-chatgpt-g56luna. Inside the capsule.",
		Kind:             types.CoSuperAssignmentImplementation,
		ParentWorkItemID: "work-test",
		ToolCallID:       "call-test",
	}
	_, err := rt.startAssignedCoSuperForParent(context.Background(), types.RunRecord{}, req)
	if err == nil || !strings.Contains(err.Error(), "model_policy_overlay_id=p5-chatgpt-g56luna") {
		t.Fatalf("prose-only overlay open err = %v, want the structured-field refusal", err)
	}

	// With the structured field set the guard passes and validation fails
	// later on the bogus parent, proving the refusal is the guard and not a
	// general open-path break.
	req.ModelPolicyOverlayID = "p5-chatgpt-g56luna"
	_, err = rt.startAssignedCoSuperForParent(context.Background(), types.RunRecord{}, req)
	if err == nil || strings.Contains(err.Error(), "structured field is empty") {
		t.Fatalf("structured overlay open err = %v, want a later parent-validation failure", err)
	}
}
