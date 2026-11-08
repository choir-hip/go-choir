package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestPublishAutoWorkNeverWaitsOnTheOwner: the owner, 2026-11-08: "there
// should be no request for human approval for the paper ... Never should it
// ask for human approval." The paper's work is the publish-auto pipeline: an
// intake task opening and feeding stories. A disclosure there that the
// rules and System 1 do not clear is refused with its reason, and no owner
// tap (pending decision) is minted; the same disclosure on other work still
// waits on the owner. Ways this could fail, written first: the paper still
// mints a pending ask; publish-auto work is let through unchecked (a leak);
// the owner tap disappears from ordinary work.
func TestPublishAutoWorkNeverWaitsOnTheOwner(t *testing.T) {
	f := newDisclosureFixture(t, "paper-no-ask", 62841)
	defer disclosure.SetModeForTest(disclosure.ModeEnforce)()
	f.escalating()
	labels := types.NewLabelSet(types.OwnerPrivateLabel("autopaper"), types.OwnerPrivateLabel("capture"))
	seed := func(runID, role string) toolregistry.ExecutionContext {
		t.Helper()
		rec := types.RunRecord{RunID: runID, AgentID: "agent-" + runID, OwnerID: f.owner, ComputerID: f.computer, ChannelID: "ch-" + runID,
			AgentProfile: agentprofile.Management, AgentRole: role, State: types.RunRunning, Prompt: "x"}
		if err := f.s.CreateRun(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
		f.rt.taintRun(runID, labels)
		if _, err := f.rt.flushRunLabels(context.Background(), f.owner, f.computer, runID); err != nil {
			t.Fatal(err)
		}
		return toolregistry.ExecutionContext{RunID: runID, AgentID: rec.AgentID, OwnerID: f.owner, ComputerID: f.computer, RunRecord: &rec}
	}

	// A task run is its task inbox's agent in the task role.
	task := seed("run-intake-task", agentprofile.ManagementTask)
	task.RunRecord.AgentID = types.ManagementTaskDesk + ":mt-paper"
	task.AgentID = task.RunRecord.AgentID
	err := f.gate(task, agentprofile.Management, disclosure.SinkDeskMessage, "texture:new", "cast {\"publish\":\"auto\"}")
	if err == nil || strings.Contains(err.Error(), "waits on the owner") || !strings.Contains(err.Error(), "never asks the owner") {
		t.Fatalf("intake task's unclear Cast: %v; want a refusal that mints no owner ask", err)
	}
	for _, e := range f.events(t) {
		if e.Phase == disclosurePhasePending {
			t.Fatalf("publish-auto work minted an owner ask: %+v", e)
		}
	}

	// Ordinary work keeps the owner tap.
	desk := seed("run-management-desk", agentprofile.Management)
	if err := f.gate(desk, agentprofile.Management, disclosure.SinkDeskMessage, "texture:new", "cast"); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("the management desk's unclear Cast: %v; want the owner tap", err)
	}
}
