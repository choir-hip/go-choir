package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestMaxOutputTokensForModelUsesSupportedModelCatalog(t *testing.T) {
	if got := maxOutputTokensForModel("glm-5.2"); got != 131072 {
		t.Fatalf("glm-5.2 max tokens = %d, want 131072", got)
	}
	if got := maxOutputTokensForModel("gpt-5.5"); got != 65536 {
		t.Fatalf("gpt-5.5 max tokens = %d, want 65536", got)
	}
	if got := maxOutputTokensForModel("unknown-model"); got != 65536 {
		t.Fatalf("unknown model max tokens = %d, want safe default 65536", got)
	}
}

// --- Bedrock Provider Tests ---

func TestBedrockProviderRequiresRegion(t *testing.T) {
	_, err := NewBedrockProvider(BedrockConfig{
		Region:    "",
		ModelID:   "test-model",
		AuthToken: "test-token",
	})
	if err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("expected region error, got: %v", err)
	}
}
func TestBedrockProviderCallError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"Service unavailable"}`))
	}))
	defer server.Close()

	p := &BedrockProvider{
		region:    "us-east-1",
		modelID:   "test-model",
		authToken: "test-token",
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		},
		anthropicV: "bedrock-2023-05-31",
	}

	_, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "test"}}}},
		MaxTokens: 1024,
	})
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
	// Error should be sanitized (no raw response body leaked).
	if strings.Contains(err.Error(), "Service unavailable") {
		t.Errorf("error message should be sanitized, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sanitized") {
		t.Errorf("error should mention sanitized, got: %v", err)
	}
}


// --- Z.AI Provider Tests ---

func TestZAIProviderRequiresAPIKey(t *testing.T) {
	_, err := NewZAIProvider(ZAIConfig{
		APIKey:  "",
		ModelID: "glm-4.7",
	})
	if err == nil || !strings.Contains(err.Error(), "api key") {
		t.Fatalf("expected api key error, got: %v", err)
	}
}
func TestZAIProviderCallError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	p := &ZAIProvider{
		apiKey:     "bad-key",
		modelID:    "glm-4.7",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	_, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "test"}}}},
		MaxTokens: 1024,
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	// Error should be sanitized.
	if strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error message should be sanitized, got: %v", err)
	}
}


// --- Resolve Provider Tests ---

func TestResolveProviderUsesSelectedProvider(t *testing.T) {
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "test-bedrock-token")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("ZAI_API_KEY", "test-zai-key")

	p, err := ResolveProvider(ProviderConfig{
		BedrockModels:    []string{"us.anthropic.claude-sonnet-4-6"},
		ZAIModels:        []string{"glm-5.1"},
		SelectedProvider: "zai",
	})
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "zai" {
		t.Errorf("expected zai, got: %s", p.Name())
	}
}
func TestResolveProviderReturnsNilWhenNoCredentials(t *testing.T) {
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "")
	t.Setenv("ZAI_API_KEY", "")
	t.Setenv("FIREWORKS_API_KEY", "")

	p, err := ResolveProvider(ProviderConfig{
		BedrockModels:   []string{"us.anthropic.claude-sonnet-4-6"},
		ZAIModels:       []string{"glm-5.1"},
		FireworksModels: []string{"accounts/fireworks/models/deepseek-v4-flash"},
	})
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil provider when no credentials, got: %s", p.Name())
	}
}
func TestFireworksProviderFromEnvMissingKey(t *testing.T) {
	t.Setenv("FIREWORKS_API_KEY", "")

	_, err := NewFireworksProviderFromEnv("accounts/fireworks/models/deepseek-v4-flash")
	if err == nil || !strings.Contains(err.Error(), "api key") {
		t.Errorf("expected api key error, got: %v", err)
	}
}
func TestChatGPTProviderRetriesUnauthorizedAfterForceRefresh(t *testing.T) {
	authPath := writeTestCodexAuth(t, "old-access", "refresh-123", time.Now().UTC())
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected refresh method: %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(oauthRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
		})
	}))
	defer refreshServer.Close()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		switch call {
		case 1:
			if got := r.Header.Get("Authorization"); got != "Bearer old-access" {
				t.Fatalf("first auth header = %q, want old access token", got)
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"expired"}`))
			return
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer new-access" {
				t.Fatalf("retry auth header = %q, want refreshed access token", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_retry\",\"model\":\"gpt-5.5\"}}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Recovered\"}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_retry\",\"model\":\"gpt-5.5\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n")
			return
		default:
			t.Fatalf("unexpected extra call %d", call)
		}
	}))
	defer server.Close()

	p, err := NewChatGPTProvider(ChatGPTConfig{
		ModelID:  "gpt-5.5",
		BaseURL:  server.URL + "/responses",
		AuthPath: authPath,
	})
	if err != nil {
		t.Fatalf("create chatgpt provider: %v", err)
	}
	p.httpClient = server.Client()
	p.auth.refreshURL = refreshServer.URL
	p.auth.httpClient = refreshServer.Client()

	resp, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Hi"}}}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("chatgpt call: %v", err)
	}
	if resp.Text != "Recovered" {
		t.Fatalf("text = %q, want Recovered", resp.Text)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
}

func TestChatGPTAuthRefreshesStaleToken(t *testing.T) {
	authPath := writeTestCodexAuth(t, "old-access", "refresh-123", time.Now().UTC().Add(-2*time.Hour))
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)
		if !strings.Contains(body, "grant_type=refresh_token") || !strings.Contains(body, "refresh_token=refresh-123") {
			t.Fatalf("unexpected refresh body: %s", body)
		}
		if !strings.Contains(body, "client_id=test-client") {
			t.Fatalf("refresh body must send client_id: %s", body)
		}
		if strings.Contains(body, "account_id") {
			t.Fatalf("refresh body must not send account_id: %s", body)
		}
		_ = json.NewEncoder(w).Encode(oauthRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
		})
	}))
	defer refreshServer.Close()

	now := time.Now().UTC()
	auth := NewChatGPTAuth(ChatGPTAuthOptions{
		Path:          authPath,
		RefreshURL:    refreshServer.URL,
		ClientID:      "test-client",
		RefreshBefore: 30 * time.Minute,
		HTTPClient:    refreshServer.Client(),
		Now:           func() time.Time { return now },
	})

	header, err := auth.Header(context.Background())
	if err != nil {
		t.Fatalf("auth header: %v", err)
	}
	if header != "Bearer new-access" {
		t.Fatalf("header = %q, want refreshed token", header)
	}

	var updated codexAuthFile
	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read auth: %v", err)
	}
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("decode auth: %v", err)
	}
	if updated.Tokens.AccessToken != "new-access" || updated.Tokens.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected updated tokens: %+v", updated.Tokens)
	}
}

func TestChatGPTAuthDoesNotRefreshUnexpiredJWTAccessToken(t *testing.T) {
	now := time.Now().UTC()
	authPath := writeTestCodexAuth(t, testJWTWithExpiry(t, now.Add(10*24*time.Hour)), "refresh-123", now.Add(-2*time.Hour))
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("refresh endpoint should not be called for an unexpired access token")
	}))
	defer refreshServer.Close()

	auth := NewChatGPTAuth(ChatGPTAuthOptions{
		Path:          authPath,
		RefreshURL:    refreshServer.URL,
		RefreshBefore: 30 * time.Minute,
		HTTPClient:    refreshServer.Client(),
		Now:           func() time.Time { return now },
	})

	header, err := auth.Header(context.Background())
	if err != nil {
		t.Fatalf("auth header: %v", err)
	}
	if !strings.HasPrefix(header, "Bearer ") {
		t.Fatalf("header = %q, want bearer token", header)
	}
}

func TestChatGPTAuthRefreshFailureDoesNotReturnStaleToken(t *testing.T) {
	authPath := writeTestCodexAuth(t, "old-access", "refresh-123", time.Now().UTC().Add(-2*time.Hour))
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer refreshServer.Close()

	now := time.Now().UTC()
	auth := NewChatGPTAuth(ChatGPTAuthOptions{
		Path:          authPath,
		RefreshURL:    refreshServer.URL,
		RefreshBefore: 30 * time.Minute,
		HTTPClient:    refreshServer.Client(),
		Now:           func() time.Time { return now },
	})

	header, err := auth.Header(context.Background())
	if err == nil {
		t.Fatalf("expected refresh error, got header %q", header)
	}
	if strings.Contains(header, "old-access") {
		t.Fatalf("stale access token leaked into header after refresh failure")
	}
}

func writeTestCodexAuth(t *testing.T, accessToken, refreshToken string, lastRefresh time.Time) string {
	t.Helper()
	path := t.TempDir() + "/auth.json"
	data, err := json.Marshal(codexAuthFile{
		AuthMode: "chatgpt",
		Tokens: codexAuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			AccountID:    "acct-test",
		},
		LastRefresh: lastRefresh.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("marshal auth: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	return path
}

func testJWTWithExpiry(t *testing.T, exp time.Time) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{"exp": exp.Unix()})
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// --- Bridge Provider Tests ---

type mockLLMProvider struct {
	name    string
	isReal  bool
	resp    *LLMResponse
	err     error
	called  bool
	lastReq *LLMRequest
}

func (m *mockLLMProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	m.called = true
	m.lastReq = &req
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	resp, err := m.Call(ctx, req)
	if err != nil {
		return nil, err
	}
	// Emit a single text delta chunk for the mock.
	if resp.Text != "" {
		onChunk(StreamChunk{
			Type:  "content_block_delta",
			Delta: resp.Text,
			Index: 0,
		})
	}
	return resp, nil
}

func (m *mockLLMProvider) Name() string { return m.name }
func (m *mockLLMProvider) IsReal() bool { return m.isReal }
func TestBridgeProviderExecuteFailure(t *testing.T) {
	mock := &mockLLMProvider{
		name:   "failing-provider",
		isReal: true,
		err:    fmt.Errorf("upstream timeout"),
	}

	bridge := NewBridgeProvider(mock)

	task := &types.RunRecord{
		RunID:   "task-2",
		OwnerID: "user-1",
		Prompt:  "This should fail",
	}

	var events []struct {
		kind    types.EventKind
		phase   string
		payload json.RawMessage
	}
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		events = append(events, struct {
			kind    types.EventKind
			phase   string
			payload json.RawMessage
		}{kind, phase, payload})
	}

	err := bridge.Execute(context.Background(), task, emit)
	if err == nil {
		t.Fatal("expected error from failed provider call")
	}
	if !strings.Contains(err.Error(), "failing-provider") {
		t.Errorf("error should mention provider name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "upstream timeout") {
		t.Errorf("error should wrap original error, got: %v", err)
	}

	// Should have emitted a failure event.
	var lastPayload map[string]string
	if err := json.Unmarshal(events[len(events)-1].payload, &lastPayload); err != nil {
		t.Fatalf("unmarshal last event: %v", err)
	}
	if lastPayload["status"] != "failed" {
		t.Errorf("expected last event status 'failed', got: %s", lastPayload["status"])
	}
}

func TestBridgeProviderEventsDistinguishRealFromStub(t *testing.T) {
	// This test verifies that the events emitted by the bridge provider
	// contain a "real":"true" marker that distinguishes them from the
	// stub provider's "provider":"stub" marker.
	mock := &mockLLMProvider{
		name:   "bedrock",
		isReal: true,
		resp: &LLMResponse{
			Text:         "real response",
			Model:        "claude-sonnet",
			StopReason:   "end_turn",
			Usage:        Usage{InputTokens: 5, OutputTokens: 3},
			ProviderName: "bedrock",
		},
	}

	bridge := NewBridgeProvider(mock)
	task := &types.RunRecord{RunID: "t1", Prompt: "test"}
	var collected []map[string]string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		var m map[string]string
		_ = json.Unmarshal(payload, &m)
		collected = append(collected, m)
	}

	_ = bridge.Execute(context.Background(), task, emit)

	// Every event should have real="true" and a non-stub provider name.
	for i, ev := range collected {
		if ev["real"] != "true" {
			t.Errorf("event %d: expected real=true, got %s", i, ev["real"])
		}
		if ev["provider"] == "stub" {
			t.Errorf("event %d: provider should not be 'stub'", i)
		}
	}
}

// --- Helper types ---

// rewriteTransport redirects requests from the original URL to the test
// server URL, preserving the path and headers.
type rewriteTransport struct {
	target   string
	original string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := strings.Replace(req.URL.String(), t.original, t.target, 1)
	newReq, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq = newReq.WithContext(req.Context())
	// Copy headers.
	for k, vs := range req.Header {
		for _, v := range vs {
			newReq.Header.Add(k, v)
		}
	}
	return http.DefaultClient.Do(newReq)
}

func TestZAIProvider_GLM52_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	if os.Getenv("ZAI_API_KEY") == "" {
		t.Skip("ZAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewZAIProviderFromEnv("glm-5.2")
	if err != nil {
		t.Fatalf("NewZAIProviderFromEnv: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.Call(ctx, LLMRequest{
		Messages: []Message{{
			Role: "user",
			Content: []Block{{
				Type: "text",
				Text: "Reply with exactly: glm-5.2-ok",
			}},
		}},
		MaxTokens: 32,
		Model:     "glm-5.2",
	})
	if err != nil {
		t.Fatalf("glm-5.2 call failed: %v", err)
	}
	if strings.TrimSpace(resp.Text) == "" {
		t.Fatal("expected non-empty glm-5.2 response text")
	}
	if resp.Model != "glm-5.2" {
		t.Fatalf("model = %q, want glm-5.2", resp.Model)
	}
}

// --- Redaction Tests ---

func TestRedactModel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Bedrock-style IDs: "us.anthropic.<long-model>"
		{"us.anthropic.claude-sonnet-4-6", "us.anthropic.clau***-4-6"},
		// Simple model name without dots
		{"simple-model", "simple-model"},
		// 4-part model ID: 3+ parts → first.second.<redacted last>
		{"a.b.c.d", "a.b.***"},
		// 2-part
		{"a.b", "a.***"},
	}
	for _, tc := range tests {
		got := redactModel(tc.input)
		if got != tc.expected {
			t.Errorf("redactModel(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
func TestErrorSanitization(t *testing.T) {
	// Verify that HTTP errors from providers do not include raw response bodies.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error with secret=abc123"}`))
	}))
	defer server.Close()

	p := &BedrockProvider{
		region:    "us-east-1",
		modelID:   "test-model",
		authToken: "test-token",
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		},
		anthropicV: "bedrock-2023-05-31",
	}

	_, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "test"}}}},
		MaxTokens: 1024,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret=abc123") {
		t.Errorf("error should not contain raw response body: %v", err)
	}
}

// --- Tool Use Content Block Tests ---

func TestBedrockProviderCallWithToolUse(t *testing.T) {
	// When the provider returns a tool_use content block in the response,
	// the LLMResponse should preserve it as a ToolCall.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request includes tools if present.
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Not strictly required for this test, but verify we can parse it.

		resp := anthropicResponse{
			ID: "msg_tool_test",
			Content: []anthropicResponseBlock{
				{Type: "text", Text: "Let me look that up."},
				{Type: "tool_use", ID: "toolu_01", Name: "read_file", Input: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
			StopReason: "tool_use",
			Usage:      anthropicUsage{InputTokens: 50, OutputTokens: 30},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &BedrockProvider{
		region:    "us-east-1",
		modelID:   "us.anthropic.claude-sonnet-4-6",
		authToken: "test-bearer-token",
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		},
		anthropicV: "bedrock-2023-05-31",
	}

	resp, err := p.Call(context.Background(), LLMRequest{
		System:    "You are helpful.",
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Read the test file"}}}},
		MaxTokens: 4096,
	})
	if err != nil {
		t.Fatalf("bedrock call: %v", err)
	}

	// Verify text content is preserved.
	if resp.Text != "Let me look that up." {
		t.Errorf("text: got %q, want 'Let me look that up.'", resp.Text)
	}

	// Verify stop reason is tool_use.
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason: got %q, want tool_use", resp.StopReason)
	}

	// Verify tool calls are extracted from content blocks.
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "toolu_01" {
		t.Errorf("tool call id: got %q, want toolu_01", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool call name: got %q, want read_file", resp.ToolCalls[0].Name)
	}
	if string(resp.ToolCalls[0].Arguments) != `{"path":"/tmp/test.txt"}` {
		t.Errorf("tool call arguments: got %q, want {\"path\":\"/tmp/test.txt\"}", string(resp.ToolCalls[0].Arguments))
	}
}

func TestZAIProviderCallWithToolUse(t *testing.T) {
	// Same test for Z.AI provider.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:    "msg_zai_tool",
			Model: "glm-4.7",
			Content: []anthropicResponseBlock{
				{Type: "tool_use", ID: "toolu_02", Name: "search", Input: json.RawMessage(`{"query":"golang testing"}`)},
			},
			StopReason: "tool_use",
			Usage:      anthropicUsage{InputTokens: 40, OutputTokens: 20},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &ZAIProvider{
		apiKey:     "test-zai-key",
		modelID:    "glm-4.7",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	resp, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Search for golang testing"}}}},
		MaxTokens: 4096,
	})
	if err != nil {
		t.Fatalf("zai call: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason: got %q, want tool_use", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "toolu_02" {
		t.Errorf("tool call id: got %q, want toolu_02", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("tool call name: got %q, want search", resp.ToolCalls[0].Name)
	}
}

func TestBedrockProviderCallWithMultipleToolUse(t *testing.T) {
	// Provider returns multiple tool_use blocks in one response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID: "msg_multi_tool",
			Content: []anthropicResponseBlock{
				{Type: "tool_use", ID: "toolu_10", Name: "read_file", Input: json.RawMessage(`{"path":"/a"}`)},
				{Type: "tool_use", ID: "toolu_11", Name: "read_file", Input: json.RawMessage(`{"path":"/b"}`)},
				{Type: "tool_use", ID: "toolu_12", Name: "search", Input: json.RawMessage(`{"q":"test"}`)},
			},
			StopReason: "tool_use",
			Usage:      anthropicUsage{InputTokens: 30, OutputTokens: 60},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &BedrockProvider{
		region:    "us-east-1",
		modelID:   "test-model",
		authToken: "test-token",
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		},
		anthropicV: "bedrock-2023-05-31",
	}

	resp, err := p.Call(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "read both files"}}}},
		MaxTokens: 4096,
	})
	if err != nil {
		t.Fatalf("bedrock call: %v", err)
	}

	if len(resp.ToolCalls) != 3 {
		t.Fatalf("tool_calls: got %d, want 3", len(resp.ToolCalls))
	}

	// Verify each tool call is preserved in order.
	names := []string{"read_file", "read_file", "search"}
	for i, tc := range resp.ToolCalls {
		if tc.Name != names[i] {
			t.Errorf("tool_calls[%d].name: got %q, want %q", i, tc.Name, names[i])
		}
	}
}
func TestBridgeProviderCallWithToolsEndTurn(t *testing.T) {
	// When the inner provider returns end_turn, CallWithTools should
	// return an end_turn response without tool calls.
	mock := &mockLLMProvider{
		name:   "test-provider",
		isReal: true,
		resp: &LLMResponse{
			ID:         "msg_end",
			Text:       "The answer is 42.",
			StopReason: "end_turn",
			Usage:      Usage{InputTokens: 10, OutputTokens: 20},
			ToolCalls:  nil,
		},
	}

	bridge := NewBridgeProvider(mock)

	req := provideriface.ToolLoopRequest{
		System:    "You are helpful.",
		Messages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"What is the answer?"}]}`)},
		MaxTokens: 4096,
	}

	resp, err := bridge.CallWithTools(context.Background(), req)
	if err != nil {
		t.Fatalf("call with tools: %v", err)
	}

	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason: got %q, want end_turn", resp.StopReason)
	}
	if resp.Text != "The answer is 42." {
		t.Errorf("text: got %q, want 'The answer is 42.'", resp.Text)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("tool_calls: got %d, want 0", len(resp.ToolCalls))
	}
}
func TestValidateMediaRequestRejectsTextOnlyModelImages(t *testing.T) {
	req := LLMRequest{
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Messages: []Message{{
			Role:    "user",
			Content: []Block{{Type: "image", Source: &MediaSource{Kind: "url", URL: "https://example.com/screen.png"}}},
		}},
	}
	err := validateMediaRequest(req.Model, req)
	if err == nil || !strings.Contains(err.Error(), "text-only") {
		t.Fatalf("error = %v, want text-only rejection", err)
	}
}

func TestValidateMediaRequestRejectsUnresolvedArtifactRefs(t *testing.T) {
	req := LLMRequest{
		Model: "accounts/fireworks/models/kimi-k2p6",
		Messages: []Message{{
			Role:    "user",
			Content: []Block{{Type: "image", Source: &MediaSource{Kind: "artifact_ref", Ref: "screenshot-1"}}},
		}},
	}
	err := validateMediaRequest(req.Model, req)
	if err == nil || !strings.Contains(err.Error(), "requires gateway artifact resolver") {
		t.Fatalf("error = %v, want artifact resolver blocker", err)
	}
}

func TestValidateMediaRequestRejectsMalformedBase64(t *testing.T) {
	req := LLMRequest{
		Model: "gpt-5.5",
		Messages: []Message{{
			Role:    "user",
			Content: []Block{{Type: "image", Source: &MediaSource{Kind: "base64", MIMEType: "image/png", Data: "not standard base64"}}},
		}},
	}
	err := validateMediaRequest(req.Model, req)
	if err == nil || !strings.Contains(err.Error(), "valid standard base64") {
		t.Fatalf("error = %v, want base64 validation blocker", err)
	}
}

func TestValidateMediaRequestRejectsRelativeImageURL(t *testing.T) {
	req := LLMRequest{
		Model: "gpt-5.5",
		Messages: []Message{{
			Role:    "user",
			Content: []Block{{Type: "image", Source: &MediaSource{Kind: "url", URL: "/local/screen.png"}}},
		}},
	}
	err := validateMediaRequest(req.Model, req)
	if err == nil || !strings.Contains(err.Error(), "absolute http(s) URL") {
		t.Fatalf("error = %v, want absolute URL validation blocker", err)
	}
}
func TestZAIProviderStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"overloaded"}`))
	}))
	defer server.Close()

	p := &ZAIProvider{
		apiKey:     "test-zai-key",
		modelID:    "glm-5-turbo",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	_, err := p.Stream(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "test"}}}},
		MaxTokens: 1024,
	}, func(chunk StreamChunk) {})
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
	if strings.Contains(err.Error(), "overloaded") {
		t.Errorf("error should be sanitized, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sanitized") {
		t.Errorf("error should mention sanitized, got: %v", err)
	}
}

func TestZAIProviderStreamWithToolUse(t *testing.T) {
	// Verify streaming handles tool_use content blocks correctly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_tool_001","model":"glm-5-turbo","usage":{"input_tokens":50}}}`,
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Checking weather..."}}`,
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}`,
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_001","name":"get_weather","input":{}}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" \"SF\"}"}}`,
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":1}`,
			`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
		}

		for _, event := range events {
			fmt.Fprintln(w, event)
			fmt.Fprintln(w)
		}
	}))
	defer server.Close()

	p := &ZAIProvider{
		apiKey:     "test-zai-key",
		modelID:    "glm-5-turbo",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	var chunks []StreamChunk
	resp, err := p.Stream(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Weather in SF?"}}}},
		Tools: []ToolDef{
			{Name: "get_weather", Description: "Get weather", InputSchema: map[string]any{"type": "object"}},
		},
		MaxTokens: 200,
	}, func(chunk StreamChunk) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("zai stream tool use: %v", err)
	}

	// Verify text content.
	if resp.Text != "Checking weather..." {
		t.Errorf("Text = %q, want %q", resp.Text, "Checking weather...")
	}

	// Verify tool calls extracted.
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "tool_use")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "toolu_001" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", resp.ToolCalls[0].ID, "toolu_001")
	}
	if resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "get_weather")
	}

	// Verify tool input JSON accumulated correctly.
	var args map[string]string
	if err := json.Unmarshal(resp.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}
	if args["location"] != "SF" {
		t.Errorf("location = %q, want %q", args["location"], "SF")
	}

	// Verify tool_call_delta chunks were emitted.
	toolDeltas := 0
	for _, c := range chunks {
		if c.ToolCallDelta != "" {
			toolDeltas++
		}
	}
	if toolDeltas != 2 {
		t.Errorf("expected 2 tool_call_delta chunks, got %d", toolDeltas)
	}
}
func TestOpenAICompatibleExactToolChoiceSerialization(t *testing.T) {
	chatBody, err := json.Marshal(openAIChatCompletionRequest{
		Model:      "accounts/fireworks/models/deepseek-v4-pro",
		Messages:   []openAIChatMessage{{Role: "user", Content: "start"}},
		ToolChoice: openAIChatToolChoice("function:request_super_execution"),
	})
	if err != nil {
		t.Fatalf("marshal chat body: %v", err)
	}
	var chatRaw map[string]any
	if err := json.Unmarshal(chatBody, &chatRaw); err != nil {
		t.Fatalf("unmarshal chat body: %v", err)
	}
	chatChoice, ok := chatRaw["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("chat tool_choice = %#v, want object", chatRaw["tool_choice"])
	}
	if chatChoice["type"] != "function" {
		t.Fatalf("chat tool_choice.type = %#v, want function", chatChoice["type"])
	}
	fn, ok := chatChoice["function"].(map[string]any)
	if !ok || fn["name"] != "request_super_execution" {
		t.Fatalf("chat tool_choice.function = %#v, want request_super_execution", chatChoice["function"])
	}

	responsesBody, err := json.Marshal(openAIRequest{
		Model:      "gpt-5.5",
		Input:      []openAIItem{{Role: "user", Content: "start"}},
		ToolChoice: openAIResponsesToolChoice("function:request_super_execution"),
		Store:      false,
	})
	if err != nil {
		t.Fatalf("marshal responses body: %v", err)
	}
	var responsesRaw map[string]any
	if err := json.Unmarshal(responsesBody, &responsesRaw); err != nil {
		t.Fatalf("unmarshal responses body: %v", err)
	}
	responsesChoice, ok := responsesRaw["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("responses tool_choice = %#v, want object", responsesRaw["tool_choice"])
	}
	if responsesChoice["type"] != "function" || responsesChoice["name"] != "request_super_execution" {
		t.Fatalf("responses tool_choice = %#v, want exact function", responsesChoice)
	}
}
func TestFireworksProviderPreservesReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body openAIChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Messages) != 3 {
			t.Fatalf("messages = %#v", body.Messages)
		}
		if body.Messages[2].ReasoningContent != "previous hidden reasoning" {
			t.Fatalf("request reasoning_content = %q", body.Messages[2].ReasoningContent)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"id":"chatcmpl_reasoning",
			"model":"accounts/fireworks/models/kimi-k2p6",
			"choices":[{
				"finish_reason":"stop",
				"message":{
					"role":"assistant",
					"reasoning_content":"new hidden reasoning",
					"content":"visible answer"
				}
			}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`)
	}))
	defer server.Close()

	p := &FireworksProvider{
		apiKey:     "fw-test-key",
		modelID:    "accounts/fireworks/models/kimi-k2p6",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	resp, err := p.Call(context.Background(), LLMRequest{
		System: "system",
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "continue"}}},
			{Role: "assistant", ReasoningContent: "previous hidden reasoning", Content: []Block{{Type: "text", Text: "previous answer"}}},
		},
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("fireworks call: %v", err)
	}
	if resp.Text != "visible answer" {
		t.Fatalf("text = %q", resp.Text)
	}
	if resp.ReasoningContent != "new hidden reasoning" {
		t.Fatalf("reasoning_content = %q", resp.ReasoningContent)
	}
}
func TestFireworksProviderStreamPreservesReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_stream_reasoning\",\"model\":\"accounts/fireworks/models/kimi-k2p6\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_stream_reasoning\",\"model\":\"accounts/fireworks/models/kimi-k2p6\",\"choices\":[{\"delta\":{\"reasoning_content\":\"hidden \"}}]}\n\n")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_stream_reasoning\",\"model\":\"accounts/fireworks/models/kimi-k2p6\",\"choices\":[{\"delta\":{\"reasoning_content\":\"reasoning\",\"content\":\"answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := &FireworksProvider{
		apiKey:     "fw-test-key",
		modelID:    "accounts/fireworks/models/kimi-k2p6",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	var reasoningDeltas []string
	resp, err := p.Stream(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}},
	}, func(chunk StreamChunk) {
		if chunk.ReasoningDelta != "" {
			reasoningDeltas = append(reasoningDeltas, chunk.ReasoningDelta)
		}
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if resp.Text != "answer" {
		t.Fatalf("text = %q", resp.Text)
	}
	if resp.ReasoningContent != "hidden reasoning" {
		t.Fatalf("reasoning_content = %q", resp.ReasoningContent)
	}
	if got := strings.Join(reasoningDeltas, ""); got != "hidden reasoning" {
		t.Fatalf("reasoning deltas = %q", got)
	}
}

func TestFireworksProviderStreamParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_tool\",\"model\":\"accounts/fireworks/models/deepseek-v4-flash\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"record_status\",\"arguments\":\"\"}}]}}]}\n\n")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_tool\",\"model\":\"accounts/fireworks/models/deepseek-v4-flash\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"status\\\":\"}}]}}]}\n\n")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl_tool\",\"model\":\"accounts/fireworks/models/deepseek-v4-flash\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"ok\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := &FireworksProvider{
		apiKey:     "fw-test-key",
		modelID:    "accounts/fireworks/models/deepseek-v4-flash",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	resp, err := p.Stream(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Record ok."}}}},
		Tools: []ToolDef{{
			Name:        "record_status",
			Description: "Record status.",
			InputSchema: map[string]any{"type": "object"},
		}},
		MaxTokens: 1024,
	}, func(StreamChunk) {})
	if err != nil {
		t.Fatalf("fireworks stream tool call: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Fatalf("stop = %q", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "record_status" {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
	var args map[string]string
	if err := json.Unmarshal(resp.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatalf("args: %v", err)
	}
	if args["status"] != "ok" {
		t.Fatalf("status = %q", args["status"])
	}
}

func TestBedrockProviderStreamFallback(t *testing.T) {
	// Bedrock provider falls back to non-streaming Call.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID: "msg_br_stream_001",
			Content: []anthropicResponseBlock{
				{Type: "text", Text: "Bedrock non-streaming fallback"},
			},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &BedrockProvider{
		region:    "us-east-1",
		modelID:   "test-model",
		authToken: "test-token",
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		},
		anthropicV: "bedrock-2023-05-31",
	}

	var chunks []StreamChunk
	resp, err := p.Stream(context.Background(), LLMRequest{
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "test"}}}},
		MaxTokens: 1024,
	}, func(chunk StreamChunk) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("bedrock stream fallback: %v", err)
	}
	if resp.Text != "Bedrock non-streaming fallback" {
		t.Errorf("Text = %q, want %q", resp.Text, "Bedrock non-streaming fallback")
	}
	// Should emit at least a message_start, content_block_delta, and message_stop.
	if len(chunks) < 3 {
		t.Errorf("expected at least 3 chunks, got %d", len(chunks))
	}
}
// --- Helper: capturing LLM provider ---

// capturingLLMProvider captures the LLMRequest before returning a canned response.
type capturingLLMProvider struct {
	name    string
	resp    *LLMResponse
	capture func(LLMRequest)
}

func (c *capturingLLMProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	if c.capture != nil {
		c.capture(req)
	}
	return c.resp, nil
}

func (c *capturingLLMProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	return c.Call(ctx, req)
}

func (c *capturingLLMProvider) Name() string { return c.name }
func (c *capturingLLMProvider) IsReal() bool { return true }

// --- BridgeProvider Streaming Tests ---

// streamingMockProvider is a mock LLM provider that emits multiple text chunks
// during streaming to simulate real SSE behavior.
type streamingMockProvider struct {
	name   string
	chunks []string
	resp   *LLMResponse
}

func (s *streamingMockProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	return s.resp, nil
}

func (s *streamingMockProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	for i, chunk := range s.chunks {
		onChunk(StreamChunk{
			Type:  "content_block_delta",
			Delta: chunk,
			Index: 0,
		})
		_ = i
	}
	// Emit stop event.
	onChunk(StreamChunk{
		Type:       "message_stop",
		StopReason: s.resp.StopReason,
		Usage:      &StreamUsage{InputTokens: s.resp.Usage.InputTokens, OutputTokens: s.resp.Usage.OutputTokens},
	})
	return s.resp, nil
}

func (s *streamingMockProvider) Name() string { return s.name }
func (s *streamingMockProvider) IsReal() bool { return true }
// streamMethodTrackerProvider tracks which methods (Call vs Stream) are invoked.
type streamMethodTrackerProvider struct {
	name         string
	resp         *LLMResponse
	streamCalled bool
	callCalled   bool
	capturedReq  *LLMRequest
}

func (s *streamMethodTrackerProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	s.callCalled = true
	s.capturedReq = &req
	return s.resp, nil
}

func (s *streamMethodTrackerProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	s.streamCalled = true
	s.capturedReq = &req
	// Emit a single delta chunk.
	if s.resp.Text != "" {
		onChunk(StreamChunk{
			Type:  "content_block_delta",
			Delta: s.resp.Text,
			Index: 0,
		})
	}
	return s.resp, nil
}

func (s *streamMethodTrackerProvider) Name() string { return s.name }
func (s *streamMethodTrackerProvider) IsReal() bool { return true }

func TestOpenCodeProviderFailsClosedWithoutConversationID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	p, err := NewOpenCodeProvider(OpenCodeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		ModelID: "deepseek-v4.1-flash",
	})
	if err != nil {
		t.Fatalf("NewOpenCodeProvider: %v", err)
	}
	p.httpClient = server.Client()

	_, err = p.Call(context.Background(), LLMRequest{Model: "deepseek-v4.1-flash"})
	if err == nil || !strings.Contains(err.Error(), "conversation_id is required") {
		t.Fatalf("Call error = %v, want required conversation_id", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestOpenCodeProviderUsesFrozenWireShapesAndSessionHeaders(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "deepseek-v4.1-flash", path: "/chat/completions"},
		{name: "glm-5.3-flash", path: "/chat/completions"},
		{name: "hy3", path: "/chat/completions"},
		{name: "muse-spark-1.3-contributor", path: "/responses"},
		{name: "muse-spark-1.3-contributor-free", path: "/responses"},
		{name: "qwen3.7-max", path: "/messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Errorf("path = %q, want %q", r.URL.Path, tt.path)
				}
				if got := r.Header.Get("x-opencode-session"); got != "run-open-code" {
					t.Errorf("x-opencode-session = %q, want run-open-code", got)
				}
				if got := r.Header.Get("User-Agent"); got != "choir-gateway/0.1" {
					t.Errorf("User-Agent = %q, want choir-gateway/0.1", got)
				}
				if tt.path == "/messages" {
					if got := r.Header.Get("x-api-key"); got != "test-key" {
						t.Errorf("x-api-key = %q, want test key", got)
					}
					_ = json.NewEncoder(w).Encode(anthropicResponse{
						ID:         "msg-1",
						Model:      tt.name,
						Content:    []anthropicResponseBlock{{Type: "text", Text: "ok"}},
						StopReason: "end_turn",
					})
					return
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Errorf("Authorization = %q, want bearer test key", got)
				}
				if tt.path == "/responses" {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-1\",\"model\":%q}}\n\n", tt.name)
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
					fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"model\":%q,\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n", tt.name)
					return
				}
				_ = json.NewEncoder(w).Encode(openAIChatCompletionResponse{
					ID:    "chat-1",
					Model: tt.name,
					Choices: []openAIChatChoice{{
						Message:      openAIChatMessage{Content: "ok"},
						FinishReason: "stop",
					}},
				})
			}))
			defer server.Close()

			p, err := NewOpenCodeProvider(OpenCodeConfig{
				APIKey:  "test-key",
				BaseURL: server.URL,
				ModelID: tt.name,
			})
			if err != nil {
				t.Fatalf("NewOpenCodeProvider: %v", err)
			}
			p.httpClient = server.Client()
			resp, err := p.Call(context.Background(), LLMRequest{
				Model:          tt.name,
				ConversationID: "run-open-code",
				Messages:       []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hello"}}}},
			})
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			if resp.Text != "ok" {
				t.Fatalf("response text = %q, want ok", resp.Text)
			}
		})
	}
}

func TestOpenCodeProviderSurfacesIncompleteAndFailedResponses(t *testing.T) {
	t.Run("incomplete carries stop reason and usage", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-inc\",\"model\":\"muse-spark-1.3-contributor-free\"}}\n\n")
			fmt.Fprint(w, "data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp-inc\",\"model\":\"muse-spark-1.3-contributor-free\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"usage\":{\"input_tokens\":12,\"output_tokens\":64}}}\n\n")
		}))
		defer server.Close()

		p, err := NewOpenCodeProvider(OpenCodeConfig{APIKey: "test-key", BaseURL: server.URL, ModelID: "muse-spark-1.3-contributor-free"})
		if err != nil {
			t.Fatalf("NewOpenCodeProvider: %v", err)
		}
		p.httpClient = server.Client()
		resp, err := p.Call(context.Background(), LLMRequest{
			Model:          "muse-spark-1.3-contributor-free",
			ConversationID: "run-inc",
			Messages:       []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hello"}}}},
		})
		if err != nil {
			t.Fatalf("Call: %v", err)
		}
		if resp.StopReason != "max_output_tokens" {
			t.Fatalf("stop_reason = %q, want max_output_tokens", resp.StopReason)
		}
		if resp.Usage.OutputTokens != 64 {
			t.Fatalf("output_tokens = %d, want 64", resp.Usage.OutputTokens)
		}
	})

	t.Run("failed surfaces an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-fail\"}}\n\n")
			fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp-fail\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\"}}}\n\n")
		}))
		defer server.Close()

		p, err := NewOpenCodeProvider(OpenCodeConfig{APIKey: "test-key", BaseURL: server.URL, ModelID: "muse-spark-1.3-contributor-free"})
		if err != nil {
			t.Fatalf("NewOpenCodeProvider: %v", err)
		}
		p.httpClient = server.Client()
		_, err = p.Call(context.Background(), LLMRequest{
			Model:          "muse-spark-1.3-contributor-free",
			ConversationID: "run-fail",
			Messages:       []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hello"}}}},
		})
		if err == nil || !strings.Contains(err.Error(), "server_error") {
			t.Fatalf("Call error = %v, want server_error", err)
		}
	})
}

func TestOpenCodeProviderRejectsUnknownModelBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	p, err := NewOpenCodeProvider(OpenCodeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		ModelID: "deepseek-v4.1-flash",
	})
	if err != nil {
		t.Fatalf("NewOpenCodeProvider: %v", err)
	}
	p.httpClient = server.Client()
	_, err = p.Call(context.Background(), LLMRequest{Model: "unknown-model", ConversationID: "run-open-code"})
	if err == nil || !strings.Contains(err.Error(), "unsupported model") {
		t.Fatalf("Call error = %v, want unsupported model", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}
