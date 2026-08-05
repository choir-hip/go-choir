package agentcore

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

	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/promptstore"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedDurableTextureSubject(t *testing.T, s *store.Store, ownerID, docID string) string {
	t.Helper()
	agentID := currentTextureAgentID(docID)
	now := time.Now().UTC()
	req := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "test-start:" + ownerID + ":" + docID,
		TrajectoryID: "test-trajectory:" + ownerID + ":" + docID, Kind: types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "test-work:" + ownerID + ":" + docID, Objective: "process durable updates", AssignedAgentID: agentID},
		InitialDocument: types.Document{DocID: docID, Title: "Durable test subject"},
		InitialRevision: types.Revision{
			RevisionID: "test-revision:" + ownerID + ":" + docID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial durable test content",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: "sandbox-test", SandboxID: "sandbox-test",
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("seed durable Texture subject: %v", err)
	}
	return req.TrajectoryID
}

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

	bus := events.NewEventBus()
	rt := New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0), WithContentService(contentowner.NewService(s, bus)))
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

func runtimeHandlerRequest(t *testing.T, handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	req := authenticatedRequest(method, path, body, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
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
		rec, err := h.rt.GetRun(context.Background(), taskID, "user-1")
		if err != nil {
			t.Fatalf("get task status: %v", err)
		}
		if rec.State.Terminal() {
			return rec.State
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not complete within %v", taskID, timeout)
	return ""
}

// waitForEvents polls ListEvents until all expected event kinds are present
// or the deadline expires. This avoids races where the run state becomes
// terminal before the final event is persisted (common under -race).
func waitForEvents(t *testing.T, s *store.Store, runID string, expectedKinds []types.EventKind, timeout time.Duration) []types.EventRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	needed := make(map[types.EventKind]bool, len(expectedKinds))
	for _, k := range expectedKinds {
		needed[k] = true
	}
	var evts []types.EventRecord
	for time.Now().Before(deadline) {
		var err error
		evts, err = s.ListEvents(context.Background(), runID, 200)
		if err != nil {
			t.Fatalf("list events: %v", err)
		}
		for _, ev := range evts {
			delete(needed, ev.Kind)
		}
		if len(needed) == 0 {
			return evts
		}
		time.Sleep(20 * time.Millisecond)
	}
	for kind := range needed {
		t.Errorf("missing expected event kind: %s", kind)
	}
	return evts
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

	bus := events.NewEventBus()
	rt := New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0), WithContentService(contentowner.NewService(s, bus)))

	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, s
}

func testPeerRuntime(t *testing.T, primary *Runtime, sharedStore *store.Store) *Runtime {
	t.Helper()
	if primary == nil || sharedStore == nil {
		t.Fatal("primary runtime and shared store are required")
	}
	bus := events.NewEventBus()
	peer := New(
		primary.cfg, sharedStore, bus, provider.NewStubProvider(0),
		WithContentService(contentowner.NewService(sharedStore, bus)),
	)
	setTestDispatch(peer, sharedStore)
	t.Cleanup(peer.Stop)
	return peer
}

// setTestDispatch sets a test dispatch function that executes runs
// asynchronously. Production uses the actor runtime (actorruntime.New);
// tests use this minimal dispatch that calls ExecuteActivationSync in a
// goroutine. This is test infrastructure, not production code.
func setTestDispatch(rt *Runtime, s *store.Store) {
	rt.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		switch kind {
		case "initial_dispatch":
			runID := strings.TrimSpace(content)
			if runID == "" {
				return nil
			}
			go func() {
				rec, err := s.GetLifecycleRun(ctx, ownerID, computerID, runID)
				if err != nil {
					rec, err = s.GetRunByOwner(ctx, ownerID, runID)
				}
				if err != nil {
					return
				}
				rt.ExecuteActivationSync(ctx, &rec)
			}()
		case "coagent_result":
			// Synchronous: the boot sweep needs the reconcile to
			// complete before the test checks the result.
			agent, err := s.GetAgentByScope(ctx, ownerID, computerID, toAgentID)
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
		cfg: provideriface.Config{
			SandboxID:           "sandbox-prompt-test",
			PromptRoot:          promptRoot,
			SupervisionInterval: time.Hour,
		},
		promptStore: promptstore.New(promptRoot),
		modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
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

func runtimeTestTextureBodyDoc(t *testing.T, docID, revisionID, content string) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-" + docID + "-" + revisionID},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": "p-" + docID + "-" + revisionID},
				Content: []texturedoc.Node{{Type: "text", Text: content}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
