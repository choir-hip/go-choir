package agentcore

// A desk loop parked on a provider quota is shown to engineering,
// management and Texture (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md).
// Failure modes, written down for this test:
//
//  Q1 the park leaves no record management can read, so a parked loop
//     reads as dead or stuck;
//  Q2 an engineering assignment's park never reaches Texture's line;
//  Q3 the resume is never told: the owner's line and management's pack say
//     "waiting" after the loop resumed;
//  Q4 the wait names no provider, model or expected reset time.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestProviderQuotaParkIsShownAndCleared(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-quota", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	now := time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)
	waitTestClock(rt, &now)

	rec := run
	rec.AgentProfile = agentprofile.Engineering
	rec.Metadata = cloneMetadata(run.Metadata)
	rec.Metadata["assignment_id"] = assignment.AssignmentID
	rec.Metadata["assignment_attempt"] = int(assignment.Binding.Attempt)
	hook := rt.providerQuotaWaitHook(&rec)
	const model = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	reset := now.Add(32 * time.Minute)
	wait := types.NewProviderQuotaWait("run", "synthetic", model, now, reset, false, 30*time.Minute, "provider quota exhausted (synthetic)")
	hook(ctx, wait, "")

	views := rt.providerWaitViews(seed.OwnerID, seed.ComputerID)
	if len(views) != 1 || views[0].Kind != types.WaitKindProviderQuota || views[0].Class != types.WaitClassContention ||
		views[0].Provider != "synthetic" || views[0].Model != model || !views[0].ExpectedUntil.Equal(reset) || !strings.Contains(views[0].Subject, assignment.AssignmentID) {
		t.Fatalf("Q1/Q4: provider waits = %+v", views)
	}
	mgmt := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: persistentManagementAgentID(seed.OwnerID), OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if mgmt == nil || len(mgmt.ProviderWaits) != 1 || len(mgmt.Waits) != 1 || mgmt.Waits[0].Wait == nil || mgmt.Waits[0].Wait.Kind != types.WaitKindProviderQuota {
		t.Fatalf("Q1: management's pack = %+v", mgmt)
	}
	texture := actingCommitmentPackForDesk(ctx, rt, toolregistry.ExecutionContext{AgentID: "texture:x", OwnerID: seed.OwnerID, ComputerID: seed.ComputerID})
	if texture == nil || len(texture.ProviderWaits) != 1 {
		t.Fatalf("Q1: Texture's pack = %+v", texture)
	}

	// The re-check after the announce threshold tells Texture once.
	now = now.Add(2 * time.Minute)
	hook(ctx, wait, "")
	relays := waitRelays(t, s, seed)
	if len(relays) != 1 || !strings.Contains(relays[0].Directive.Body, "Synthetic quota to reset") || relays[0].SubjectRefs["wait_kind"] != string(types.WaitKindProviderQuota) {
		t.Fatalf("Q2: relays = %+v", relays)
	}

	// The quota reset: the loop resumed.
	now = now.Add(30 * time.Minute)
	hook(ctx, nil, toolregistry.ProviderWaitResumed)
	if views := rt.providerWaitViews(seed.OwnerID, seed.ComputerID); len(views) != 0 {
		t.Fatalf("Q3: provider waits after the resume = %+v", views)
	}
	relays = waitRelays(t, s, seed)
	var ended bool
	for _, r := range relays {
		if r.SubjectRefs["wait_phase"] == types.WaitRelayEnd && strings.Contains(r.Directive.Body, "the provider quota reset") {
			ended = true
		}
	}
	if !ended {
		t.Fatalf("Q3: no end line: %+v", relays)
	}
	av := rt.assignmentWaitViews(seed.OwnerID, seed.ComputerID)
	if len(av) != 1 || av[0].Wait != nil || av[0].WaitedMinutes[string(types.WaitKindProviderQuota)] != 32 {
		t.Fatalf("Q3: assignment view after the resume = %+v", av)
	}
}
