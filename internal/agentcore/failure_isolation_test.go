//go:build comprehensive

package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Worker Failure Isolation Tests (VAL-CHOIR-009, VAL-CHOIR-010) ---
//
// These tests verify that worker failures are isolated: a failing child
// worker does not crash the parent task, other sibling workers, or the
// runtime itself. Parents receive error notifications and can continue.
//
// Feature requirements:
//
//   - Worker failure sends error message to parent
//   - Parent task continues running (not crashed)
//   - Error includes loop_id and error message
//   - Parent can spawn replacement worker if needed
//   - Other sibling workers unaffected by one failure
//   - Failed task transitions to failed state with error details
//   - loop.failed event emitted with error details
//   - Runtime health remains ready or degraded (not failed)
//   - Parent can cancel running child runs (VAL-CHOIR-010)
//   - Cancelled task transitions to cancelled state
//   - loop.cancelled event emitted

// failureIsolationSetup creates a fresh Runtime with a configurable provider
// for testing failure scenarios.
func failureIsolationSetup(t *testing.T, provider provideriface.Provider) (*Runtime, *APIHandler, string) {
	t.Helper()

	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/%s.db", dir, t.Name())

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	cfg := provideriface.Config{
		ComputerID:          "autoputer-failure-test",
		StorePath:           dbPath,
		ProviderTimeout:     500 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	// Create a parent task that stays running for the duration of the test.
	parentRec, err := rt.StartRun(context.Background(), "parent task for isolation tests", "user-alice")
	if err != nil {
		t.Fatalf("create parent task: %v", err)
	}

	// Wait for parent to start running.
	time.Sleep(50 * time.Millisecond)

	return rt, handler, parentRec.RunID
}

// waitForTaskState polls until the task reaches a terminal state or times out.
func waitForTaskState(t *testing.T, rt *Runtime, taskID string, timeout time.Duration) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			task, _ := rt.Store().GetRun(ctx, taskID)
			t.Fatalf("timeout waiting for task %s (state=%s)", taskID[:8], task.State)
		default:
		}
		task, err := rt.Store().GetRun(ctx, taskID)
		if err != nil {
			t.Fatalf("get task %s: %v", taskID, err)
		}
		if task.State.Terminal() {
			return task
		}
		time.Sleep(30 * time.Millisecond)
	}
}

// --- VAL-CHOIR-009: Worker Failure Isolation ---

// TestFailureIsolation_FailedWorkerSendsErrorToParent verifies that when a
// child worker fails, the parent receives an error notification via the
// channel system (VAL-CHOIR-009, expected behavior #1).
func TestFailureIsolation_FailedWorkerSendsErrorToParent(t *testing.T) {
	t.Parallel()
	provider := &provider.StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: fmt.Errorf("simulated worker failure: invalid tool invocation"),
	}
	rt, _, parentID := failureIsolationSetup(t, provider)
	ctx := context.Background()

	// Spawn a child that will fail.
	child, err := rt.StartCoagentRun(ctx, parentID, "execute invalid command", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn child: %v", err)
	}

	// Wait for the child to reach a terminal state.
	task := waitForTaskState(t, rt, child.RunID, 10*time.Second)

	// Verify the child is in failed state.
	if task.State != types.RunFailed {
		t.Fatalf("child state: got %q, want failed", task.State)
	}

	// Verify the error message is populated.
	if task.Error == "" {
		t.Fatal("child error should not be empty")
	}
	if !strings.Contains(task.Error, "simulated worker failure") {
		t.Errorf("child error: got %q, want to contain 'simulated worker failure'", task.Error)
	}

	// Verify error message posted to parent channel. Run state is committed
	// before the parent notification is visible, so poll for the channel fact.
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, _, err := rt.ChannelRead(parentID, 0)
		if err != nil {
			t.Fatalf("parent channel read: %v", err)
		}
		for _, msg := range msgs {
			if msg.From == child.RunID && msg.Role == "error" {
				if !strings.Contains(msg.Content, "simulated worker failure") {
					t.Errorf("error message content: got %q, want to contain 'simulated worker failure'", msg.Content)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no error message found in parent channel from failed child")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestFailureIsolation_ConcurrentFailuresAndSuccesses verifies that when
// multiple workers run concurrently and some fail while others succeed,
// results are correctly separated (VAL-CHOIR-009, VAL-CHOIR-008).
func TestFailureIsolation_ConcurrentFailuresAndSuccesses(t *testing.T) {
	t.Parallel()
	provider := &conditionalFailProvider{
		delay:      50 * time.Millisecond,
		failPrefix: "fail",
		result:     "Completed successfully.",
	}
	rt, _, parentID := failureIsolationSetup(t, provider)
	ctx := context.Background()

	// Spawn 5 children: 3 succeed, 2 fail.
	ids := make([]string, 5)
	objectives := []string{
		"research topic A",
		"fail task B",
		"analyze data C",
		"fail task D",
		"summarize E",
	}

	for i, obj := range objectives {
		rec, err := rt.StartCoagentRun(ctx, parentID, obj, "user-alice", nil)
		if err != nil {
			t.Fatalf("spawn child %d: %v", i, err)
		}
		ids[i] = rec.RunID
	}

	// Wait for all to complete.
	for i, id := range ids {
		task := waitForTaskState(t, rt, id, 10*time.Second)
		expectedState := types.RunCompleted
		if strings.Contains(objectives[i], "fail") {
			expectedState = types.RunFailed
		}
		if task.State != expectedState {
			t.Errorf("child %d (%q): state got %q, want %q", i, objectives[i], task.State, expectedState)
		}
	}

	expectedRoleByChild := make(map[string]string, len(ids))
	for i, id := range ids {
		if strings.Contains(objectives[i], "fail") {
			expectedRoleByChild[id] = "error"
		} else {
			expectedRoleByChild[id] = "result"
		}
	}

	// Verify parent channel has exactly one terminal notification per child.
	// Run state is committed before parent notification is posted, so poll for
	// the channel/event-log condition rather than assuming terminal state means
	// delivery is already visible.
	var resultCountByChild map[string]int
	var errorCountByChild map[string]int
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, _, err := rt.ChannelRead(parentID, 0)
		if err != nil {
			t.Fatalf("parent channel read: %v", err)
		}
		resultCountByChild = make(map[string]int)
		errorCountByChild = make(map[string]int)
		for _, msg := range msgs {
			if _, ok := expectedRoleByChild[msg.From]; !ok {
				continue
			}
			switch msg.Role {
			case "result":
				resultCountByChild[msg.From]++
			case "error":
				errorCountByChild[msg.From]++
			}
		}
		ready := true
		for _, id := range ids {
			switch expectedRoleByChild[id] {
			case "result":
				ready = ready && resultCountByChild[id] == 1 && errorCountByChild[id] == 0
			case "error":
				ready = ready && errorCountByChild[id] == 1 && resultCountByChild[id] == 0
			}
		}
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, id := range ids {
		switch expectedRoleByChild[id] {
		case "result":
			if resultCountByChild[id] != 1 || errorCountByChild[id] != 0 {
				t.Errorf("child %s result/error messages: got %d/%d, want 1/0", id, resultCountByChild[id], errorCountByChild[id])
			}
		case "error":
			if errorCountByChild[id] != 1 || resultCountByChild[id] != 0 {
				t.Errorf("child %s error/result messages: got %d/%d, want 1/0", id, errorCountByChild[id], resultCountByChild[id])
			}
		}
	}
}

// --- VAL-CHOIR-010: Task Cancellation ---

// TestCancellation_CancelRunningTask verifies that a running task can be
// cancelled and transitions to cancelled state (VAL-CHOIR-010).
func TestCancellation_CancelRunningTask(t *testing.T) {
	t.Parallel()
	// Use a slow provider so the task stays running.
	provider := provider.NewStubProvider(5 * time.Second)
	rt, _, parentID := failureIsolationSetup(t, provider)
	ctx := context.Background()

	// Spawn a child with a long-running task.
	child, err := rt.StartCoagentRun(ctx, parentID, "long running analysis", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn child: %v", err)
	}

	// Wait for the task to start running.
	time.Sleep(100 * time.Millisecond)

	// Verify it's running.
	task, _ := rt.Store().GetRun(ctx, child.RunID)
	if task.State != types.RunRunning {
		t.Fatalf("child should be running, got %q", task.State)
	}

	// Cancel the task via the runtime.
	err = rt.CancelRun(ctx, child.RunID, "user-alice")
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}

	// Wait for the task to reach terminal state.
	task = waitForTaskState(t, rt, child.RunID, 5*time.Second)

	if task.State != types.RunCancelled {
		t.Errorf("cancelled task state: got %q, want cancelled", task.State)
	}
}

// TestCancellation_CancelNonExistentTask verifies that cancelling a
// non-existent task returns an appropriate error.
func TestCancellation_CancelNonExistentTask(t *testing.T) {
	t.Parallel()
	provider := provider.NewStubProvider(50 * time.Millisecond)
	rt, _, _ := failureIsolationSetup(t, provider)
	ctx := context.Background()

	err := rt.CancelRun(ctx, "non-existent-run-id", "user-alice")
	if err == nil {
		t.Error("expected error when cancelling non-existent task")
	}
}

// TestCancellation_CancelOtherUsersTask verifies that cancelling another
// user's task returns an error (ownership check).
func TestCancellation_CancelOtherUsersTask(t *testing.T) {
	t.Parallel()
	provider := provider.NewStubProvider(5 * time.Second)
	rt, _, parentID := failureIsolationSetup(t, provider)
	ctx := context.Background()

	child, _ := rt.StartCoagentRun(ctx, parentID, "task owned by alice", "user-alice", nil)
	time.Sleep(100 * time.Millisecond)

	// Try to cancel as a different user.
	err := rt.CancelRun(ctx, child.RunID, "user-bob")
	if err == nil {
		t.Error("expected error when cancelling another user's task")
	}
}

// TestCancellation_SiblingUnaffectedByCancel verifies that cancelling one
// running task does not affect other running runs (VAL-CHOIR-010).
func TestCancellation_SiblingUnaffectedByCancel(t *testing.T) {
	provider := provider.NewStubProvider(500 * time.Millisecond)
	rt, _, parentID := failureIsolationSetup(t, provider)
	ctx := context.Background()

	// Spawn 3 children.
	child1, _ := rt.StartCoagentRun(ctx, parentID, "task 1", "user-alice", nil)
	child2, _ := rt.StartCoagentRun(ctx, parentID, "task 2", "user-alice", nil)
	child3, _ := rt.StartCoagentRun(ctx, parentID, "task 3", "user-alice", nil)

	time.Sleep(100 * time.Millisecond)

	// Cancel only child2.
	err := rt.CancelRun(ctx, child2.RunID, "user-alice")
	if err != nil {
		t.Fatalf("cancel child2: %v", err)
	}

	// Wait for child2 to be cancelled.
	task2 := waitForTaskState(t, rt, child2.RunID, 5*time.Second)
	if task2.State != types.RunCancelled {
		t.Errorf("child2 state: got %q, want cancelled", task2.State)
	}

	// Wait for children 1 and 3 to complete normally.
	task1 := waitForTaskState(t, rt, child1.RunID, 10*time.Second)
	task3 := waitForTaskState(t, rt, child3.RunID, 10*time.Second)

	if task1.State != types.RunCompleted {
		t.Errorf("child1 state: got %q, want completed (should be unaffected)", task1.State)
	}
	if task3.State != types.RunCompleted {
		t.Errorf("child3 state: got %q, want completed (should be unaffected)", task3.State)
	}

	// Children 1 and 3 should have results.
	if task1.Result == "" {
		t.Error("child1 should have a result")
	}
	if task3.Result == "" {
		t.Error("child3 should have a result")
	}
}

// TestCancellation_CancelViaAPI verifies that the cancel API endpoint works
// correctly (VAL-CHOIR-010).
func TestCancellation_CancelViaAPI(t *testing.T) {
	t.Parallel()
	provider := provider.NewStubProvider(5 * time.Second)
	rt, handler, parentID := failureIsolationSetup(t, provider)
	child, err := rt.StartCoagentRun(context.Background(), parentID, "cancellable via api", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn cancellable child: %v", err)
	}

	w := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+child.RunID+"/cancel", ``, "user-alice")
	if w.Code != http.StatusOK {
		t.Fatalf("cancel API status: got %d, want 200; body: %s", w.Code, w.Body.String())
	}
	cancelled, err := rt.GetRun(context.Background(), child.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get cancelled child: %v", err)
	}
	if cancelled.State != types.RunCancelled {
		t.Errorf("task state after cancel: got %q, want cancelled", cancelled.State)
	}
}

// --- VAL-CHOIR-014: Recovery After Autoputer Restart ---

// --- Helper types ---
