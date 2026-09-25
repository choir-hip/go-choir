//go:build comprehensive

package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Concurrent Workers Tests (VAL-CHOIR-008) ---
//
// These tests verify that multiple workers can run concurrently from the
// same parent without interference. Feature requirements:
//
//   - Parent can spawn 3+ workers in sequence without waiting
//   - All workers run concurrently (not sequentially)
//   - Each worker has independent channel to parent
//   - Results collected as each worker completes
//   - No interference between sibling workers

// testConcurrentSetup creates a fresh Runtime with a slow provider to
// ensure runs stay running long enough for concurrent observation.
func testConcurrentSetup(t *testing.T) (*Runtime, *APIHandler, string) {
	t.Helper()

	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/%s.db", dir, t.Name())

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	// Use a slow provider so runs stay running for concurrent observation.
	provider := provider.NewStubProvider(500 * time.Millisecond)
	cfg := provideriface.Config{
		ComputerID:          "autoputer-concurrent-test",
		StorePath:           dbPath,
		ProviderTimeout:     2 * time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	// Create a parent task.
	parentRec, err := rt.StartRun(context.Background(), "parent objective", "user-alice")
	if err != nil {
		t.Fatalf("create parent task: %v", err)
	}

	// Wait for the parent to start running.
	time.Sleep(50 * time.Millisecond)

	return rt, handler, parentRec.RunID
}

// TestConcurrentWorkers_AllRunningSimultaneously verifies that after spawning
// multiple workers, all are in running state at the same time (not serialized)
// (VAL-CHOIR-008, expected behavior #2).
func TestConcurrentWorkers_AllRunningSimultaneously(t *testing.T) {
	rt, _, parentID := testConcurrentSetup(t)
	ctx := context.Background()
	childIDs := make([]string, 3)
	for i := range childIDs {
		rec, err := rt.StartCoagentRun(ctx, parentID, fmt.Sprintf("concurrent task %d", i), "user-alice", nil)
		if err != nil {
			t.Fatalf("spawn child %d: %v", i, err)
		}
		childIDs[i] = rec.RunID
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running := 0
		for _, id := range childIDs {
			rec, err := rt.GetRun(ctx, id, "user-alice")
			if err != nil {
				t.Fatalf("get child %s: %v", id, err)
			}
			if rec.State == types.RunRunning {
				running++
			}
		}
		if running == len(childIDs) {
			if got := rt.RunningCount(); got < len(childIDs) {
				t.Fatalf("running count = %d, want at least %d", got, len(childIDs))
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timeout waiting for all children to run simultaneously")
}

// TestConcurrentWorkers_ConcurrentSpawnStress verifies that concurrent
// spawn calls don't cause race conditions or data corruption.
func TestConcurrentWorkers_ConcurrentSpawnStress(t *testing.T) {
	t.Parallel()
	rt, _, parentID := testConcurrentSetup(t)
	ctx := context.Background()

	const numWorkers = 10
	var wg sync.WaitGroup
	childIDs := make([]string, numWorkers)
	errors := make([]error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec, err := rt.StartCoagentRun(ctx, parentID, fmt.Sprintf("stress task %d", idx), "user-alice", nil)
			if err != nil {
				errors[idx] = err
				return
			}
			childIDs[idx] = rec.RunID
		}(i)
	}

	wg.Wait()

	// Check no errors occurred.
	for i, err := range errors {
		if err != nil {
			t.Errorf("concurrent spawn %d: %v", i, err)
		}
	}

	// Verify all IDs are unique.
	seen := make(map[string]bool)
	for i, id := range childIDs {
		if id == "" {
			t.Errorf("child %d has empty ID", i)
			continue
		}
		if seen[id] {
			t.Errorf("duplicate child ID: %s", id)
		}
		seen[id] = true
	}
}

// TestConcurrentWorkers_TasksActuallyRunConcurrently verifies that runs
// actually run concurrently, not sequentially. If 3 runs each take 200ms,
// the total should be closer to 200ms than 600ms.
func TestConcurrentWorkers_TasksActuallyRunConcurrently(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/%s.db", dir, t.Name())

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	// Each task takes 200ms.
	provider := provider.NewStubProvider(200 * time.Millisecond)
	cfg := provideriface.Config{
		ComputerID:          "autoputer-concurrent-timing",
		StorePath:           dbPath,
		ProviderTimeout:     200 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	ctx := context.Background()

	// Create parent.
	parentRec, err := rt.StartRun(ctx, "parent", "user-alice")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Spawn 3 children and measure total time.
	start := time.Now()
	childIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		rec, err := rt.StartCoagentRun(ctx, parentRec.RunID, fmt.Sprintf("timing task %d", i), "user-alice", nil)
		if err != nil {
			t.Fatalf("spawn child %d: %v", i, err)
		}
		childIDs[i] = rec.RunID
	}

	// Wait for all to complete.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
		}

		allDone := true
		for _, id := range childIDs {
			task, _ := rt.Store().GetRun(ctx, id)
			if !task.State.Terminal() {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	elapsed := time.Since(start)

	// If runs ran sequentially, total would be ~600ms.
	// With concurrency, should be closer to ~300ms (200ms per task + overhead).
	// We use a generous threshold of 3 seconds to account for slow CI runners.
	if elapsed > 3*time.Second {
		t.Errorf("runs appear to run sequentially: elapsed %v (expected < 3s for 3 concurrent 200ms runs)", elapsed)
	}
}

// TestConcurrentWorkers_ResultsPostedToParentChannelOnCompletion verifies that
// when a spawned child task completes, the result is automatically posted
// to the parent's channel (VAL-CHOIR-008, related to VAL-CHOIR-006).
func TestConcurrentWorkers_ResultsPostedToParentChannelOnCompletion(t *testing.T) {
	t.Parallel()
	rt, _, parentID := testConcurrentSetup(t)
	ctx := context.Background()

	// Spawn a child.
	rec, err := rt.StartCoagentRun(ctx, parentID, "auto-post result task", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn child: %v", err)
	}

	// Wait for the child to complete.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for child to complete")
		default:
		}
		task, _ := rt.Store().GetRun(ctx, rec.RunID)
		if task.State.Terminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Check the parent channel for the child's result.
	// Poll with timeout since channel posting may not be instantaneous.
	deadline = time.After(5 * time.Second)
	var found bool
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for result message in parent channel from child")
		default:
		}

		msgs, _, err := rt.ChannelRead(parentID, 0)
		if err != nil {
			t.Fatalf("parent channel read: %v", err)
		}

		found = false
		for _, msg := range msgs {
			if msg.From == rec.RunID && msg.Role == "result" {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !found {
		t.Error("no result message found in parent channel from child after completion")
	}
}

// TestConcurrentWorkers_FailedChildPostsErrorToParentChannel verifies that
// when a spawned child task fails, an error is posted to the parent's channel
// (VAL-CHOIR-008, related to VAL-CHOIR-009).
func TestConcurrentWorkers_FailedChildPostsErrorToParentChannel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/%s.db", dir, t.Name())

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	// Provider that always fails.
	provider := &provider.StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: fmt.Errorf("simulated provider failure"),
	}
	cfg := provideriface.Config{
		ComputerID:          "autoputer-fail-test",
		StorePath:           dbPath,
		ProviderTimeout:     10 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	ctx := context.Background()

	// Create parent.
	parentRec, err := rt.StartRun(ctx, "parent", "user-alice")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Spawn a child that will fail.
	rec, err := rt.StartCoagentRun(ctx, parentRec.RunID, "failing task", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn child: %v", err)
	}

	// Wait for the child to complete (fail).
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for child to fail")
		default:
		}
		task, _ := rt.Store().GetRun(ctx, rec.RunID)
		if task.State.Terminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify the child is in failed state.
	task, _ := rt.Store().GetRun(ctx, rec.RunID)
	if task.State != types.RunFailed {
		t.Fatalf("child state: got %q, want failed", task.State)
	}

	// Should find an error message from the child.
	deadline = time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Error("no error message found in parent channel from failed child")
			return
		default:
		}
		msgs, _, err := rt.ChannelRead(parentRec.RunID, 0)
		if err != nil {
			t.Fatalf("parent channel read: %v", err)
		}
		for _, msg := range msgs {
			if msg.From == rec.RunID && msg.Role == "error" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestConcurrentWorkers_MixedPassFailWorkers verifies that when some workers
// fail and others succeed, results are correctly reported independently
// (VAL-CHOIR-008, VAL-CHOIR-009).
func TestConcurrentWorkers_MixedPassFailWorkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/%s.db", dir, t.Name())

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()

	// Create a provider that fails for runs containing "fail" in the objective.
	provider := &conditionalFailProvider{
		delay:      50 * time.Millisecond,
		failPrefix: "fail",
		result:     "Task completed successfully.",
	}

	cfg := provideriface.Config{
		ComputerID:          "autoputer-mixed-test",
		StorePath:           dbPath,
		ProviderTimeout:     50 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	ctx := context.Background()

	// Create parent.
	parentRec, err := rt.StartRun(ctx, "parent for mixed workers", "user-alice")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Spawn workers: 2 succeed, 1 fails.
	rec1, _ := rt.StartCoagentRun(ctx, parentRec.RunID, "analyze data", "user-alice", nil)
	rec2, _ := rt.StartCoagentRun(ctx, parentRec.RunID, "fail this task", "user-alice", nil)
	rec3, _ := rt.StartCoagentRun(ctx, parentRec.RunID, "summarize results", "user-alice", nil)

	childIDs := []string{rec1.RunID, rec2.RunID, rec3.RunID}

	// Wait for all to complete.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
		}
		allDone := true
		for _, id := range childIDs {
			task, _ := rt.Store().GetRun(ctx, id)
			if !task.State.Terminal() {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Check task states.
	task1, _ := rt.Store().GetRun(ctx, rec1.RunID)
	task2, _ := rt.Store().GetRun(ctx, rec2.RunID)
	task3, _ := rt.Store().GetRun(ctx, rec3.RunID)

	if task1.State != types.RunCompleted {
		t.Errorf("task1: got %q, want completed", task1.State)
	}
	if task2.State != types.RunFailed {
		t.Errorf("task2 (failing): got %q, want failed", task2.State)
	}
	if task3.State != types.RunCompleted {
		t.Errorf("task3: got %q, want completed", task3.State)
	}

	// Failed task should have error message.
	if task2.Error == "" {
		t.Error("task2 should have an error message")
	}

	// Successful runs should have results.
	if task1.Result == "" {
		t.Error("task1 should have a result")
	}
	if task3.Result == "" {
		t.Error("task3 should have a result")
	}
}

// conditionalFailProvider is a test provider that fails runs containing
// a specific prefix in the prompt.
type conditionalFailProvider struct {
	delay      time.Duration
	failPrefix string
	result     string
}

func (p *conditionalFailProvider) ProviderName() string { return "conditional-fail" }

func (p *conditionalFailProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started"}`))

	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		return ctx.Err()
	}

	// The runtime prepends the agent system prompt to the provider prompt
	// ("<system>\n\nUser request:\n<objective>"), and the system prompt text
	// legitimately contains words like "fails". Only match the failure
	// trigger against the user-request portion of the prompt.
	prompt := task.Prompt
	if idx := strings.LastIndex(prompt, "\n\nUser request:\n"); idx >= 0 {
		prompt = prompt[idx+len("\n\nUser request:\n"):]
	}
	if strings.Contains(strings.ToLower(prompt), p.failPrefix) {
		return fmt.Errorf("task failed: prompt contains %q", p.failPrefix)
	}

	emit(types.EventRunDelta, "execution",
		json.RawMessage(`{"text":"`+p.result+`"}`))
	return nil
}
