package actorruntime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Review round 4c (DeepSeek 1 / Sonnet E1): a fate wake that finds the live
// saga holding the assignment is deferred by the dispatcher (re-delivered
// with backoff), never consumed.
func TestFateDeadlineHeldSagaIsADispatcherDeferral(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fate-defer.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	adapter := New(provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)
	handler := newActorHandler(adapter.Runtime, nil)
	handler.bootAt = time.Now().UTC().Add(-time.Hour)
	handler.fateDeadline = func(context.Context, string, string, string, string) error {
		return fmt.Errorf("resume: %w", agentcore.ErrAssignmentSagaHeld)
	}
	mailbox := scopedActorMailboxID("owner-defer", "autoputer-test", "management:owner-defer")
	u := actor.Update{UpdateID: "fate-1", ToAgentID: mailbox, Kind: "assigned_engineering_fate_deadline",
		Content: `{"assignment_id":"assignment-x","attempt":1}`, CreatedAt: time.Now().UTC()}
	_, err = handler.HandleUpdate(ctx, mailbox, u, nil)
	if !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("a held-saga fate wake returned %v, want a dispatcher deferral", err)
	}
}

// The fix's assumption (review round 4c, Sonnet's probe inverted): after the
// G9 fate wake of an assignment was consumed, a plain schedule of the same
// content dedups into nothing, and the redrive hook mints a new generation.
func TestFateRedriveAfterAConsumedWakeIsDeliverable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fate-redrive.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	adapter := New(provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)
	const owner, computer, agent = "owner-redrive", "autoputer-test", "management:owner-redrive"
	content := `{"assignment_id":"assignment-r","attempt":1}`
	if err := adapter.dispatchRedrive(ctx, owner, computer, agent, "assigned_engineering_fate_deadline", content, "trajectory-r", ""); err != nil {
		t.Fatal(err)
	}
	mailbox := scopedActorMailboxID(owner, computer, agent)
	pending, err := adapter.log.Unprocessed(ctx, mailbox)
	if err != nil || len(pending) != 1 {
		t.Fatalf("fixture: G9 wake rows = %d, %v", len(pending), err)
	}
	if err := adapter.log.MarkProcessed(ctx, mailbox, pending[0].UpdateID); err != nil {
		t.Fatal(err)
	}
	later := time.Now().UTC().Add(time.Minute)
	if err := adapter.schedule(ctx, owner, computer, agent, "assigned_engineering_fate_deadline", content, "trajectory-r", "", later); err != nil {
		t.Fatal(err)
	}
	if rows, _ := adapter.log.Unprocessed(ctx, mailbox); len(rows) != 0 {
		t.Fatalf("fixture: the plain schedule was expected to dedup into the consumed wake, got %d rows", len(rows))
	}
	if err := adapter.scheduleRedrive(ctx, owner, computer, agent, "assigned_engineering_fate_deadline", content, "trajectory-r", "", later); err != nil {
		t.Fatal(err)
	}
	if rows, _ := adapter.log.Unprocessed(ctx, mailbox); len(rows) != 1 {
		t.Fatalf("the redrive after a consumed fate wake left %d deliverable rows, want 1", len(rows))
	}
}
