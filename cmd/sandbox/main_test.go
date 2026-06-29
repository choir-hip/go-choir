package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pii"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/sandbox"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/trace"
)

func TestBuildRuntimeConfigPreservesHostServiceURLs(t *testing.T) {
	cfg := sandbox.Config{
		SandboxID: "vm-test",
		StorePath: "/tmp/runtime.db",
	}
	loaded := runtime.Config{
		PromptRoot:           "/prompts",
		SkillsRoot:           "/skills",
		ProviderTimeout:      7 * time.Second,
		SupervisionInterval:  3 * time.Second,
		ResearcherCount:      2,
		TextureWakeDebounce:  250 * time.Millisecond,
		TextureActorParkIdle: 45 * time.Second,
		VmctlURL:             "http://10.200.60.1:8083",
		MaildURL:             "http://10.200.60.1:8087",
		LLMProvider:          "fireworks",
		LLMModel:             "model",
		LLMReasoningEffort:   "low",
		ModelPolicyPath:      "/policy.toml",
	}

	got := buildRuntimeConfig(cfg, loaded, "/files")
	if got.SandboxID != cfg.SandboxID || got.StorePath != cfg.StorePath {
		t.Fatalf("sandbox identity/store not preserved: %+v", got)
	}
	if got.VmctlURL != loaded.VmctlURL {
		t.Fatalf("VmctlURL = %q, want %q", got.VmctlURL, loaded.VmctlURL)
	}
	if got.MaildURL != loaded.MaildURL {
		t.Fatalf("MaildURL = %q, want %q", got.MaildURL, loaded.MaildURL)
	}
	if got.TextureActorParkIdle != loaded.TextureActorParkIdle {
		t.Fatalf("TextureActorParkIdle = %s, want %s", got.TextureActorParkIdle, loaded.TextureActorParkIdle)
	}
}

func TestActorMailboxConfigFromEnvDefaultsToBlockingBackpressure(t *testing.T) {
	got := actorMailboxConfigFromEnv(func(string) string { return "" })

	if !got.BackpressureEnabled {
		t.Fatal("backpressure default disabled")
	}
	if !got.BlockingBackpressure {
		t.Fatal("blocking backpressure default disabled")
	}
	if got.InboxCapacity != 1000 {
		t.Fatalf("InboxCapacity = %d, want 1000", got.InboxCapacity)
	}
	if got.SendTimeout != 5*time.Second {
		t.Fatalf("SendTimeout = %s, want 5s", got.SendTimeout)
	}
}

func TestActorMailboxConfigFromEnvOverrides(t *testing.T) {
	values := map[string]string{
		"RUNTIME_ACTOR_BACKPRESSURE_ENABLED":  "true",
		"RUNTIME_ACTOR_BACKPRESSURE_BLOCKING": "false",
		"RUNTIME_ACTOR_INBOX_CAPACITY":        "42",
		"RUNTIME_ACTOR_SEND_TIMEOUT":          "750ms",
	}
	got := actorMailboxConfigFromEnv(func(key string) string { return values[key] })

	if !got.BackpressureEnabled {
		t.Fatal("backpressure disabled")
	}
	if got.BlockingBackpressure {
		t.Fatal("blocking backpressure enabled")
	}
	if got.InboxCapacity != 42 {
		t.Fatalf("InboxCapacity = %d, want 42", got.InboxCapacity)
	}
	if got.SendTimeout != 750*time.Millisecond {
		t.Fatalf("SendTimeout = %s, want 750ms", got.SendTimeout)
	}
}

func TestActorMailboxConfigFromEnvCanDisableBackpressure(t *testing.T) {
	values := map[string]string{
		"RUNTIME_ACTOR_BACKPRESSURE_ENABLED": "off",
	}
	got := actorMailboxConfigFromEnv(func(key string) string { return values[key] })

	if got.BackpressureEnabled {
		t.Fatal("backpressure enabled")
	}
	if len(actorRuntimeOptionsFromEnv(func(key string) string { return values[key] })) != 1 {
		t.Fatal("disabled backpressure should only configure inbox capacity")
	}
}

func TestTracePersistenceStoreRedactsPayloadBeforeSQLAppend(t *testing.T) {
	inner, err := trace.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open sqlite trace store: %v", err)
	}
	defer func() {
		if err := inner.Close(); err != nil {
			t.Fatalf("close sqlite trace store: %v", err)
		}
	}()
	store := tracePersistenceStore(inner)
	payload := `{"message":"contact alex@example.com or 555-123-4567","safe":"keep this"}`

	ev := trace.Event{
		ID:        "trace-redaction-sandbox-1",
		RunID:     "run-redaction-sandbox",
		EventType: "tool.result",
		OwnerID:   "user-alice",
		Payload:   []byte(payload),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Append(context.Background(), &ev); err != nil {
		t.Fatalf("append trace event: %v", err)
	}

	got, err := inner.Get(context.Background(), ev.ID)
	if err != nil {
		t.Fatalf("get persisted trace event: %v", err)
	}
	stored := string(got.Payload)
	for _, leak := range []string{"alex@example.com", "555-123-4567"} {
		if strings.Contains(stored, leak) {
			t.Fatalf("raw PII leaked into persisted payload: %q in %s", leak, stored)
		}
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassEmail)) {
		t.Fatalf("expected email redaction token in persisted payload: %s", stored)
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassPhone)) {
		t.Fatalf("expected phone redaction token in persisted payload: %s", stored)
	}
	if !strings.Contains(stored, "keep this") {
		t.Fatalf("non-PII payload content was lost: %s", stored)
	}
}

func TestRegisterBaseAPIRoutesServesTrustedProxyRequest(t *testing.T) {
	s := server.NewServer("sandbox-test", "0")
	closeBaseAPI, err := registerBaseAPIRoutes(s, filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatalf("register Base API routes: %v", err)
	}
	defer func() {
		if err := closeBaseAPI(); err != nil {
			t.Fatalf("close Base API journal: %v", err)
		}
	}()
	req := httptest.NewRequest(http.MethodGet, "/api/base/delta?cursor=0", nil)
	req.Header.Set("X-Authenticated-User", "user-base")
	req.Header.Set("X-Authenticated-Scopes", "read:base")
	rr := httptest.NewRecorder()

	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Events []json.RawMessage `json:"events"`
		Cursor int64             `json:"cursor"`
		Head   int64             `json:"head"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Events) != 0 || resp.Cursor != 0 || resp.Head != 0 {
		t.Fatalf("response = %+v, want empty zero delta", resp)
	}
}
