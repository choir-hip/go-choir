package actorruntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// FP4 round 3b (bunch 51: DeepSeek 3, Devin F1 and F2). The agentcore
// dispatch harness hands the raw "engineering-admission" name to the
// runtime; production scopes every mailbox (owner, computer, agent) and the
// handler parses it back. Ways this can fail: the scoped admission mailbox
// does not parse, so the wake errors before admission; the handler parses it
// but the runtime drops a wake whose mailbox is not the casting agent's; the
// adapter's dispatcher loses the admission lane and admission waits for a
// desk's slot.

// A spawn wake on the scoped admission mailbox goes through the real
// handler into HandleDelegatedAssignmentSpawnDeadline and on into the spawn
// saga, which stops at the absent capsule executor, past admission.
func TestAdmissionMailboxWakeReachesTheSpawnSagaThroughTheHandler(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "admission-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	const computerID = "autoputer-test"
	adapter := New(provideriface.Config{
		ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)

	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-admission-mailbox", computerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-admission-mailbox")
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Assignment.Binding.ParentAgentID == agentcore.EngineeringAdmissionMailbox {
		t.Fatal("fixture: the casting agent is the admission mailbox")
	}

	mailbox := scopedActorMailboxID(seed.OwnerID, computerID, agentcore.EngineeringAdmissionMailbox)
	u := actor.Update{
		UpdateID: "admission-nudge", ToAgentID: mailbox, Kind: "delegated_assignment_spawn_deadline",
		Content:   fmt.Sprintf(`{"assignment_id":%q,"attempt":%d,"nudge":"assignment-gone"}`, open.AssignmentID, open.Binding.Attempt),
		CreatedAt: time.Now().UTC(),
	}
	handler := newActorHandler(adapter.Runtime, nil)
	handler.bootAt = time.Now().UTC().Add(-time.Hour) // the wake is this boot's work
	_, err = handler.HandleUpdate(ctx, mailbox, u, nil)
	if err == nil || !strings.Contains(err.Error(), "capsule authority unavailable") {
		t.Fatalf("a wake on the scoped admission mailbox did not reach the spawn saga: %v", err)
	}
}

// Reverting the admission lane alone must fail a test.
func TestKernelDispatcherHasTheAdmissionLane(t *testing.T) {
	opts := kernelDispatcherOptions("sink")
	for _, lane := range opts.Lanes {
		if lane.Prefix == agentcore.EngineeringAdmissionMailbox {
			if lane.MaxConcurrent != 1 {
				t.Fatalf("admission lane MaxConcurrent = %d, want 1", lane.MaxConcurrent)
			}
			return
		}
	}
	t.Fatalf("the dispatcher has no %q lane: %+v", agentcore.EngineeringAdmissionMailbox, opts.Lanes)
}
