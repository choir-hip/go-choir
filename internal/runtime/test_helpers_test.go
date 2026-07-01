package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// listCoagentRunsByRequester returns the runs owned by ownerID whose
// RequestedByRunID provenance points at requesterRunID. It replaces the
// deleted store helper ListChildRuns: callers used that to count/inspect the
// runs spawned on behalf of a requesting run, which is now expressed through
// requester provenance rather than parent/child control links.
func listCoagentRunsByRequester(t *testing.T, s *store.Store, ownerID, requesterRunID string, limit int) []types.RunRecord {
	t.Helper()
	runs, err := s.ListRunsByOwner(context.Background(), ownerID, limit)
	if err != nil {
		t.Fatalf("list runs by owner: %v", err)
	}
	var matched []types.RunRecord
	for _, run := range runs {
		if strings.TrimSpace(run.RequestedByRunID) == requesterRunID {
			matched = append(matched, run)
		}
	}
	return matched
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// testAPISetup creates a fresh Runtime and APIHandler for HTTP handler tests.
func testAPISetup(t *testing.T) (*Runtime, *APIHandler) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
		ObjectGraphStore:    objectgraph.NewMemoryStore(),
	}, s, events.NewEventBus(), NewStubProvider(0))
	setTestDispatch(rt, s)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, handler
}

func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	return req
}

func registeredRuntimeRequest(t *testing.T, handler *APIHandler, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	srv := server.NewServer("runtime-api-test", "0")
	RegisterRoutes(srv, handler)
	req := authenticatedRequest(method, path, body, user)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func textureRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("X-Authenticated-User", "user-1")
	return req
}

func waitForTaskCompletion(t *testing.T, h *APIHandler, taskID string, timeout time.Duration) types.RunState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req := textureRequest(t, http.MethodGet, "/api/agent/status?loop_id="+taskID, nil)
		w := httptest.NewRecorder()
		h.HandleRunStatus(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get task status: status = %d", w.Code)
		}
		var resp runStatusResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.State.Terminal() {
			return resp.State
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not complete within %v", taskID, timeout)
	return ""
}

func testRuntime(t *testing.T) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
		ObjectGraphStore:    objectgraph.NewMemoryStore(),
	}, s, events.NewEventBus(), NewStubProvider(0))

	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, s
}

// setTestDispatch sets a test dispatch function that executes runs
// asynchronously. Production uses the actor runtime (actorruntime.New);
// tests use this minimal dispatch that calls ExecuteActivationSync in a
// goroutine. This is test infrastructure, not production code.
func setTestDispatch(rt *Runtime, s *store.Store) {
	rt.SetDispatchActor(func(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		switch kind {
		case "initial_dispatch":
			runID := strings.TrimSpace(content)
			if runID == "" {
				return nil
			}
			go func() {
				rec, err := s.GetRun(ctx, runID)
				if err != nil {
					return
				}
				rt.ExecuteActivationSync(ctx, &rec)
			}()
		case "coagent_result":
			// Synchronous: the boot sweep needs the reconcile to
			// complete before the test checks the result.
			agent, err := s.GetAgent(ctx, toAgentID)
			if err != nil {
				return nil // agent not found — nothing to wake
			}
			if _, err := rt.ReconcileCoagentWake(ctx, agent.OwnerID, toAgentID); err != nil {
				log.Printf("test dispatch: reconcile coagent wake for %s: %v", toAgentID, err)
			}
		}
		return nil
	})
}

func testPromptRuntime(t *testing.T) *Runtime {
	t.Helper()
	promptRoot := filepath.Join(t.TempDir(), "prompts")
	return &Runtime{
		cfg: Config{
			SandboxID:           "sandbox-prompt-test",
			PromptRoot:          promptRoot,
			SupervisionInterval: time.Hour,
		},
		promptStore:   NewPromptStore(promptRoot),
		modelPolicies: make(map[string]ModelPolicy),
	}
}

func executeWorkerDelegationUntilSettled(t *testing.T, registry *ToolRegistry, ctx context.Context, raw json.RawMessage) (string, error) {
	t.Helper()
	startRaw, err := registry.Execute(ctx, "delegate_worker_vm", raw)
	if err != nil {
		return "", err
	}
	var start map[string]any
	if err := json.Unmarshal([]byte(startRaw), &start); err != nil {
		t.Fatalf("decode async worker start: %v\n%s", err, startRaw)
	}
	if stringMapValue(start, "status") != "worker_run_started" {
		return startRaw, nil
	}
	var original delegateWorkerVMArgs
	_ = json.Unmarshal(raw, &original)
	workerRunID := firstNonEmpty(stringMapValue(start, "worker_run_id"), stringMapValue(start, "loop_id"))
	workerSandboxURL := firstNonEmpty(stringMapValue(start, "worker_sandbox_url"), original.WorkerSandboxURL)
	finishArgs := map[string]any{
		"worker_sandbox_url": workerSandboxURL,
		"worker_run_id":      workerRunID,
		"worker_id":          firstNonEmpty(stringMapValue(start, "worker_id"), original.WorkerID),
		"vm_id":              firstNonEmpty(stringMapValue(start, "worker_vm_id"), original.VMID),
		"profile":            firstNonEmpty(stringMapValue(start, "profile"), original.Profile),
		"objective":          original.Objective,
		"timeout_seconds":    original.TimeoutSeconds,
	}
	deadline := time.Now().Add(10 * time.Second)
	var lastRaw string
	for {
		finishRaw, err := registry.Execute(ctx, "finish_worker_delegation", mustJSON(t, finishArgs))
		if err != nil {
			return "", err
		}
		lastRaw = finishRaw
		var finish map[string]any
		if err := json.Unmarshal([]byte(finishRaw), &finish); err != nil {
			t.Fatalf("decode async worker finish: %v\n%s", err, finishRaw)
		}
		if stringMapValue(finish, "status") != "worker_run_active" {
			return finishRaw, nil
		}
		if time.Now().After(deadline) {
			return lastRaw, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}
