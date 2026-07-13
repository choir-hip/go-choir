package apihandler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestProductAPIRequestToolUsesCanonicalServerAndRunOwner(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-tool-test", "0")
	var gotRequest struct {
		method      string
		requestURI  string
		ownerID     string
		ownerEmail  string
		contentType string
		body        string
	}
	canonical.HandleFunc("/api/texture/documents", func(w http.ResponseWriter, r *http.Request) {
		gotRequest.method = r.Method
		gotRequest.requestURI = r.URL.RequestURI()
		gotRequest.ownerID = r.Header.Get("X-Authenticated-User")
		gotRequest.ownerEmail = r.Header.Get("X-Authenticated-Email")
		gotRequest.contentType = r.Header.Get("Content-Type")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		gotRequest.body, _ = body["title"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"owner_id":%q,"server":"canonical"}`, r.Header.Get("X-Authenticated-User"))
	})

	registry := toolregistry.NewToolRegistry()
	if err := RegisterProductAPIRequestTool(canonical, registry); err != nil {
		t.Fatalf("register product_api_request: %v", err)
	}
	tool, ok := registry.Lookup(productAPIToolName)
	if !ok {
		t.Fatal("canonical Super registry missing product_api_request")
	}
	if tool.Description != "Call an allowed authenticated product API route in the current runtime using the run owner as the authenticated user. This is for foreground super product-path orchestration; it refuses internal, test, agent, prompt-config, and raw event mutation routes." {
		t.Fatalf("description changed: %q", tool.Description)
	}
	methodSchema := tool.Parameters["properties"].(map[string]any)["method"].(map[string]any)
	if got := fmt.Sprint(methodSchema["enum"]); got != "[GET POST PUT DELETE]" {
		t.Fatalf("method schema enum = %s", got)
	}

	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID:      "run-product-api",
		AgentID:    "agent-super-product-api",
		OwnerID:    "user-product-api",
		OwnerEmail: "owner@example.com",
		Profile:    agentprofile.Super,
	})
	raw, err := registry.Execute(ctx, productAPIToolName, json.RawMessage(`{
		"method":" post ",
		"path":"https://ignored.example/api/texture/documents?source=tool",
		"body":{"title":"Product API tool owner proof"}
	}`))
	if err != nil {
		t.Fatalf("product_api_request: %v", err)
	}
	if gotRequest.method != http.MethodPost || gotRequest.requestURI != "/api/texture/documents?source=tool" {
		t.Fatalf("canonical request method/URI = %q %q", gotRequest.method, gotRequest.requestURI)
	}
	if gotRequest.ownerID != "user-product-api" || gotRequest.ownerEmail != "owner@example.com" {
		t.Fatalf("canonical request owner headers = %q %q", gotRequest.ownerID, gotRequest.ownerEmail)
	}
	if gotRequest.contentType != "application/json" || gotRequest.body != "Product API tool owner proof" {
		t.Fatalf("canonical request content type/body = %q %q", gotRequest.contentType, gotRequest.body)
	}

	var result struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		StatusCode  int    `json:"status_code"`
		ContentType string `json:"content_type"`
		Body        string `json:"body"`
		AllowedBy   string `json:"allowed_by"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode product_api_request response: %v\n%s", err, raw)
	}
	if result.Method != http.MethodPost || result.Path != "/api/texture/documents?source=tool" || result.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected product API result: %+v", result)
	}
	if result.ContentType != "application/json" || result.AllowedBy != "product_api_request_allowlist" {
		t.Fatalf("unexpected product API metadata: %+v", result)
	}
	if result.Body != `{"owner_id":"user-product-api","server":"canonical"}` {
		t.Fatalf("result body did not come from canonical server: %s", result.Body)
	}
}

func TestRegisterProductAPIRequestToolRejectsNilAndDuplicateRegistration(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-registration-test", "0")
	registry := toolregistry.NewToolRegistry()
	if err := RegisterProductAPIRequestTool(nil, registry); err == nil || !strings.Contains(err.Error(), "nil server") {
		t.Fatalf("nil server error = %v", err)
	}
	if err := RegisterProductAPIRequestTool(canonical, nil); err == nil || !strings.Contains(err.Error(), "nil registry") {
		t.Fatalf("nil registry error = %v", err)
	}
	if err := RegisterProductAPIRequestTool(canonical, registry); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if err := RegisterProductAPIRequestTool(canonical, registry); err == nil || !strings.Contains(err.Error(), `tool "product_api_request" already registered`) {
		t.Fatalf("duplicate registration error = %v", err)
	}
	if registry.Size() != 1 {
		t.Fatalf("registry size after duplicate = %d, want 1", registry.Size())
	}
}

func TestProductAPIRequestToolRejectsUnauthorizedAndDisallowedRequests(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-rejection-test", "0")
	registry := toolregistry.NewToolRegistry()
	if err := RegisterProductAPIRequestTool(canonical, registry); err != nil {
		t.Fatalf("register product_api_request: %v", err)
	}
	superCtx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		OwnerID: "user-product-api",
		Profile: agentprofile.Super,
	})

	for _, tc := range []struct {
		name string
		args string
		want string
	}{
		{name: "internal", args: `{"method":"GET","path":"/internal/runtime/runs/run-1"}`, want: "refuses non-product route"},
		{name: "test", args: `{"method":"GET","path":"/api/test/texture"}`, want: "refuses non-product route"},
		{name: "agent", args: `{"method":"GET","path":"/api/agent/loops"}`, want: "refuses non-product route"},
		{name: "prompt config", args: `{"method":"GET","path":"/api/prompts/super"}`, want: "refuses non-product route"},
		{name: "raw event", args: `{"method":"POST","path":"/api/events"}`, want: "not in the product-path allowlist"},
		{name: "invalid method", args: `{"method":"PATCH","path":"/api/texture/documents"}`, want: `method "PATCH" is not allowed`},
		{name: "empty path", args: `{"method":"GET","path":""}`, want: "path must not be empty"},
		{name: "relative path", args: `{"method":"GET","path":"api/texture/documents"}`, want: "path must be absolute"},
		{name: "newline path", args: `{"method":"GET","path":"/api/texture/documents\nX-Evil: true"}`, want: "path must not contain newlines"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := registry.Execute(superCtx, productAPIToolName, json.RawMessage(tc.args)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}

	workerCtx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		OwnerID: "user-product-api",
		Profile: agentprofile.CoSuper,
	})
	if _, err := registry.Execute(workerCtx, productAPIToolName, json.RawMessage(`{"method":"GET","path":"/api/universal-wire/stories"}`)); err == nil || !strings.Contains(err.Error(), "only available to foreground super") {
		t.Fatalf("non-Super error = %v", err)
	}
	missingOwnerCtx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{Profile: agentprofile.Super})
	if _, err := registry.Execute(missingOwnerCtx, productAPIToolName, json.RawMessage(`{"method":"GET","path":"/api/universal-wire/stories"}`)); err == nil || !strings.Contains(err.Error(), "missing owner context") {
		t.Fatalf("missing-owner error = %v", err)
	}

	oversizedBody := `{"method":"POST","path":"/api/texture/documents","body":{"value":"` + strings.Repeat("x", productAPIToolMaxBodyBytes) + `"}}`
	if _, err := registry.Execute(superCtx, productAPIToolName, json.RawMessage(oversizedBody)); err == nil || !strings.Contains(err.Error(), "body exceeds 1048576 bytes") {
		t.Fatalf("oversized-body error = %v", err)
	}
}

func TestProductAPIRequestToolCapsResponseAndReportsHTTPError(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-response-cap-test", "0")
	canonical.HandleFunc("/api/trace/oversized", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", productAPIToolMaxBodyBytes+1)))
	})
	registry := toolregistry.NewToolRegistry()
	if err := RegisterProductAPIRequestTool(canonical, registry); err != nil {
		t.Fatalf("register product_api_request: %v", err)
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		OwnerID: "user-product-api",
		Profile: agentprofile.Super,
	})
	raw, err := registry.Execute(ctx, productAPIToolName, json.RawMessage(`{"method":"GET","path":"/api/trace/oversized"}`))
	if err != nil {
		t.Fatalf("product_api_request: %v", err)
	}
	var result struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
		Truncated  bool   `json:"truncated"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode product_api_request response: %v", err)
	}
	if result.StatusCode != http.StatusBadGateway || !result.Truncated || result.Error != "product API returned non-2xx status" {
		t.Fatalf("unexpected capped error result: status=%d truncated=%t error=%q", result.StatusCode, result.Truncated, result.Error)
	}
	if len(result.Body) != productAPIToolMaxBodyBytes || strings.Trim(result.Body, "x") != "" {
		t.Fatalf("capped body length/content = %d/%q", len(result.Body), result.Body[:min(len(result.Body), 32)])
	}
}
