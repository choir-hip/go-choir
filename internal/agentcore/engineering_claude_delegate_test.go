package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/design/claude-delegate-slice1-2026-10-14.md §3. Failure modes pinned:
//
//	F1/F2 a computer whose delegate is unavailable (no token, not
//	      allowlisted) gets the claude mode;
//	the claude mode overrides a Cast that withholds the network, or one
//	that asks for the ecosystems explicitly;
//	the store refuses a claude binding, or the grant policy digest does not
//	see the mode (claude invisible in evidence);
//	the engineering run does not carry its capsule mode, so the prompt
//	cannot know it (F18), or carrying it breaks the exact bind check.
func TestEngineeringCastNetworkPicksClaudeOnlyWhereTheDelegateIsAvailable(t *testing.T) {
	cases := []struct {
		spec      string
		available bool
		want      string
	}{
		{``, false, types.EngineeringCapsuleNetworkEcosystemProxy},
		{`{"objective":"x"}`, false, types.EngineeringCapsuleNetworkEcosystemProxy},
		{``, true, types.EngineeringCapsuleNetworkClaude},
		{`{"objective":"x"}`, true, types.EngineeringCapsuleNetworkClaude},
		{`{"network":"claude"}`, false, types.EngineeringCapsuleNetworkEcosystemProxy},
		{`{"network":"forbidden"}`, true, types.EngineeringCapsuleNetworkForbidden},
		{`{"network":false}`, true, types.EngineeringCapsuleNetworkForbidden},
		{`{"network":"ecosystem_proxy"}`, true, types.EngineeringCapsuleNetworkEcosystemProxy},
		{`{"network":" L2 "}`, true, types.EngineeringCapsuleNetworkEcosystemProxy},
	}
	for _, tc := range cases {
		if got := engineeringCastNetwork(tc.spec, tc.available); got != tc.want {
			t.Errorf("engineeringCastNetwork(%s, available=%v) = %q, want %q", tc.spec, tc.available, got, tc.want)
		}
	}
}

func TestClaudeBindingIsImplementationOnlyAndVisibleInEvidence(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-claude", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-claude")
	open.Binding.NetworkMode = types.EngineeringCapsuleNetworkClaude
	if open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open); err != nil {
		t.Fatal(err)
	}
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatalf("store refused a claude implementation binding: %v", err)
	}
	verbs := []string{"exec", "read_file"}
	fs := types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay
	if store.ComputeEngineeringGrantPolicyDigest("engineering", verbs, types.EngineeringCapsuleNetworkClaude, fs, true) ==
		store.ComputeEngineeringGrantPolicyDigest("engineering", verbs, types.EngineeringCapsuleNetworkEcosystemProxy, fs, true) {
		t.Fatal("grant policy digest does not distinguish the claude mode from L2")
	}
	agent, err := s.GetAgentByScope(ctx, seed.OwnerID, seed.ComputerID, open.Binding.AssignedAgentID)
	if err != nil {
		t.Fatal(err)
	}
	run := buildAssignedEngineeringRun(opened.Assignment, "run:"+open.AssignmentID, "build it", open.Binding.SourceArtifactRef)
	if err := store.ValidateEngineeringAssignmentRun(opened.Assignment, agent, run); err != nil {
		t.Fatalf("a claude-mode run fails the exact bind check: %v", err)
	}
	if got := metadataStringValue(run.Metadata, "capsule_network_mode"); got != types.EngineeringCapsuleNetworkClaude {
		t.Fatalf("run capsule_network_mode = %q", got)
	}
}

func TestEngineeringPromptCarriesTheDelegateOnlyForAClaudeModeRun(t *testing.T) {
	rt, _ := testRuntime(t)
	prompt := func(mode string) string {
		rec := &types.RunRecord{RunID: "run:assignment-x", OwnerID: "owner-x", ComputerID: rt.cfg.ComputerID,
			AgentProfile: agentprofile.Engineering, AgentID: "engineering:assignment-x",
			Metadata: map[string]any{"assignment_id": "assignment-x", "capsule_network_mode": mode}}
		p, err := rt.systemPromptForRun(rec)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := prompt(types.EngineeringCapsuleNetworkClaude); !strings.Contains(p, "The Claude Delegate (this capsule only)") {
		t.Fatal("claude-mode engineering prompt lacks the delegate section")
	}
	for _, mode := range []string{types.EngineeringCapsuleNetworkEcosystemProxy, types.EngineeringCapsuleNetworkForbidden, ""} {
		if p := prompt(mode); strings.Contains(p, "choir-delegate") || !strings.Contains(p, "Never start another agent or create an alternate candidate world.") {
			t.Fatalf("%q-mode engineering prompt mentions the delegate or lost the rule", mode)
		}
	}
}
