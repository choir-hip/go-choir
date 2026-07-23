package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestHandleTexturePromptEvalPinsOverlayAcrossTextureRoute(t *testing.T) {
	t.Parallel()
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"

[roles.texture]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "glm-medium.toml"), []byte(`
[overlay]
expires_at = "`+future+`"

[roles.texture]
provider = "zai"
model = "glm-5.2"
reasoning = "medium"

[roles.researcher]
provider = "zai"
model = "glm-5.2"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	core, handler := promptEvalTestSetup(t, policyPath)
	body := `{"text":"Write a briefing about new AI infra in 2026 with live evidence.","model_policy_overlay_id":"glm-medium"}`
	w := promptEvalHandlerRequest(handler.HandleTexturePromptEval, http.MethodPost, "/api/evals/texture-prompt", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp texturePromptEvalStartResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SubmissionID == "" || resp.DocID == "" {
		t.Fatalf("response handles = %+v", resp)
	}
	if resp.Provider != "zai" || resp.Model != "glm-5.2" || resp.ReasoningEffort != "medium" {
		t.Fatalf("texture arm resolution = %+v", resp)
	}
	if resp.StatusURL != "/api/prompt-bar/submissions/"+resp.SubmissionID {
		t.Fatalf("status url = %q", resp.StatusURL)
	}

	conductor, err := core.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("GetRun conductor: %v", err)
	}
	if got := promptEvalMetadataString(conductor.Metadata, modelpolicy.MetadataPolicyOverlayID); got != "glm-medium" {
		t.Fatalf("conductor overlay = %q; metadata=%+v", got, conductor.Metadata)
	}

	runs, err := core.Store().ListLifecycleRunsByChannel(context.Background(), "user-alice", "sandbox-test", resp.DocID, 20)
	if err != nil {
		t.Fatalf("ListRunsByChannel: %v", err)
	}
	var textureRun *types.RunRecord
	for i := range runs {
		if agentprofile.Canonical(runs[i].AgentProfile) == agentprofile.Texture {
			textureRun = &runs[i]
			break
		}
	}
	if textureRun == nil {
		t.Fatalf("expected a texture run on doc channel %s; runs=%d", resp.DocID, len(runs))
	}
	if got := promptEvalMetadataString(textureRun.Metadata, modelpolicy.MetadataPolicyOverlayID); got != "glm-medium" {
		t.Fatalf("texture run overlay = %q; metadata=%+v", got, textureRun.Metadata)
	}
	if got := promptEvalMetadataString(textureRun.Metadata, modelpolicy.MetadataModel); got != "glm-5.2" {
		t.Fatalf("texture run model = %q, want glm-5.2; metadata=%+v", got, textureRun.Metadata)
	}
}

func TestHandleTexturePromptEvalRejectsMissingOverlay(t *testing.T) {
	t.Parallel()
	_, handler := promptEvalTestSetup(t, "")
	w := promptEvalHandlerRequest(handler.HandleTexturePromptEval, http.MethodPost, "/api/evals/texture-prompt", `{"text":"hello"}`, "user-alice")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func promptEvalTestSetup(t *testing.T, policyPath string) (*agentcore.Runtime, *Handler) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "runtime.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	bus := events.NewEventBus()
	core := agentcore.New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(dir, "prompts"),
		ModelPolicyPath:     policyPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0), agentcore.WithContentService(contentowner.NewService(s, bus)))
	core.SetDispatchActor(func(ctx context.Context, ownerID, computerID, _ string, kind, content, _, _ string) error {
		if kind != "initial_dispatch" || strings.TrimSpace(content) == "" {
			return nil
		}
		rec, err := s.GetLifecycleRun(ctx, ownerID, computerID, strings.TrimSpace(content))
		if err != nil {
			rec, err = s.GetRunByOwner(ctx, ownerID, strings.TrimSpace(content))
		}
		if err != nil {
			return nil
		}
		go core.ExecuteActivationSync(ctx, &rec)
		return nil
	})
	t.Cleanup(func() {
		core.Stop()
		_ = s.Close()
	})
	return core, NewHandler(core)
}

func promptEvalHandlerRequest(handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}
