package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

type Runtime = agentcore.Runtime
type conductorDecision = ConductorDecision

func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Authenticated-User", user)
	return req
}

func runtimeHandlerRequest(t *testing.T, handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(method, path, body, user))
	return w
}

func testAPISetup(t *testing.T, maildURLs ...string) (*agentcore.Runtime, *Handler) {
	t.Helper()
	maildURL := ""
	if len(maildURLs) > 0 {
		maildURL = maildURLs[0]
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "texture-owner.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	bus := events.NewEventBus()
	core := agentcore.New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(dir, "prompts"),
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
		MaildURL:            maildURL,
	}, s, bus, provider.NewStubProvider(0), agentcore.WithContentService(contentowner.NewService(s, bus)))
	core.SetDispatchActor(func(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		switch kind {
		case "initial_dispatch":
			runID := strings.TrimSpace(content)
			if runID != "" {
				go func() {
					rec, err := s.GetRun(ctx, runID)
					if err == nil {
						core.ExecuteActivationSync(ctx, &rec)
					}
				}()
			}
		case "coagent_result":
			agent, err := s.GetAgent(ctx, toAgentID)
			if err == nil {
				if _, err := core.ReconcileCoagentWake(ctx, agent.OwnerID, toAgentID); err != nil {
					log.Printf("test dispatch: reconcile coagent wake for %s: %v", toAgentID, err)
				}
			}
		}
		return nil
	})
	if err := core.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install generic core tools: %v", err)
	}
	handler := NewHandler(core)
	if err := RegisterTools(core.ToolRegistryForProfile("texture"), handler); err != nil {
		t.Fatalf("register Texture owner tools: %v", err)
	}
	t.Cleanup(func() {
		core.Stop()
		_ = s.Close()
	})
	return core, handler
}

func textureRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var requestBody *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, requestBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", "user-1")
	return req
}
