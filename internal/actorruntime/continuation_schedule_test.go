package actorruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestActivationBudgetDeadlineIsDurableAndIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "deadline.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	adapter := New(provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)

	const ownerID = "owner-deadline"
	const agentID = "management:owner-deadline"
	const runID = "run-activation-deadline"
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: runID, OwnerID: ownerID, ComputerID: "autoputer-test", AgentID: agentID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create active run: %v", err)
	}

	notBefore := now.Add(time.Minute)
	if err := adapter.schedule(ctx, ownerID, "autoputer-test", agentID,
		"activation_budget_deadline", runID, "", "", notBefore); err != nil {
		t.Fatalf("schedule activation deadline: %v", err)
	}
	mailboxID := scopedActorMailboxID(ownerID, "autoputer-test", agentID)
	updates, err := adapter.log.Unprocessed(ctx, mailboxID)
	if err != nil || len(updates) != 1 {
		t.Fatalf("durable scheduled updates=%+v err=%v", updates, err)
	}
	deadline := updates[0]
	if deadline.Kind != "activation_budget_deadline" || deadline.Content != runID || deadline.ToAgentID != mailboxID || !deadline.NotBefore.Equal(notBefore) {
		t.Fatalf("deadline update=%+v, want exact kind/content/scoped target/not_before", deadline)
	}

	handler := newActorHandler(adapter.Runtime, nil)
	for i := range 2 {
		if _, err := handler.HandleUpdate(ctx, mailboxID, deadline, nil); err != nil {
			t.Fatalf("deadline delivery %d: %v", i+1, err)
		}
	}
	stored, err := s.GetRunByOwner(ctx, ownerID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != types.RunCancelled || stored.Error != "activation budget exceeded: progress deadline reached" {
		t.Fatalf("deadline terminal state=%s error=%q", stored.State, stored.Error)
	}
}
