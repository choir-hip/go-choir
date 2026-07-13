package apihandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

const (
	productAPIToolName         = "product_api_request"
	productAPIToolMaxBodyBytes = 1 << 20
)

// RegisterProductAPIRequestTool registers the foreground Super product API
// tool against the canonical server route table.
func RegisterProductAPIRequestTool(s *server.Server, registry *toolregistry.ToolRegistry) error {
	if s == nil {
		return fmt.Errorf("register product_api_request: nil server")
	}
	if registry == nil {
		return fmt.Errorf("register product_api_request: nil registry")
	}
	if _, exists := registry.Lookup(productAPIToolName); exists {
		return fmt.Errorf("register product_api_request: tool %q already registered", productAPIToolName)
	}
	if err := registry.Register(newProductAPIRequestTool(s)); err != nil {
		return fmt.Errorf("register product_api_request: %w", err)
	}
	return nil
}

func newProductAPIRequestTool(s *server.Server) toolregistry.Tool {
	type args struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body,omitempty"`
	}
	return toolregistry.Tool{
		Name:        productAPIToolName,
		Description: "Call an allowed authenticated product API route in the current runtime using the run owner as the authenticated user. This is for foreground super product-path orchestration; it refuses internal, test, agent, prompt-config, and raw event mutation routes.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"method": map[string]any{"type": "string", "enum": []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}},
			"path":   map[string]any{"type": "string", "description": "Absolute API path, optionally with query string, such as /api/universal-wire/stories."},
			"body":   map[string]any{"type": "object", "description": "Optional JSON body for POST/PUT requests."},
		}, []string{"method", "path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode product_api_request args: %w", err)
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			if execution.Profile != agentprofile.Super {
				return "", fmt.Errorf("product_api_request is only available to foreground super")
			}
			ownerID := execution.OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("product_api_request missing owner context")
			}
			method := strings.ToUpper(strings.TrimSpace(in.Method))
			if method == "" {
				method = http.MethodGet
			}
			path, err := normalizeProductAPIToolPath(in.Path)
			if err != nil {
				return "", err
			}
			if err := validateProductAPIToolRoute(method, path); err != nil {
				return "", err
			}
			var body io.Reader
			if len(in.Body) > 0 && string(in.Body) != "null" {
				if len(in.Body) > productAPIToolMaxBodyBytes {
					return "", fmt.Errorf("product_api_request body exceeds %d bytes", productAPIToolMaxBodyBytes)
				}
				body = bytes.NewReader(in.Body)
			}
			req := httptest.NewRequest(method, path, body).WithContext(ctx)
			req.Header.Set("X-Authenticated-User", ownerID)
			if execution.OwnerEmail != "" {
				req.Header.Set("X-Authenticated-Email", execution.OwnerEmail)
			}
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()
			respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, productAPIToolMaxBodyBytes+1))
			if readErr != nil {
				return "", fmt.Errorf("read product_api_request response: %w", readErr)
			}
			truncated := false
			if len(respBody) > productAPIToolMaxBodyBytes {
				respBody = respBody[:productAPIToolMaxBodyBytes]
				truncated = true
			}
			result := map[string]any{
				"method":       method,
				"path":         path,
				"status_code":  resp.StatusCode,
				"content_type": resp.Header.Get("Content-Type"),
				"body":         strings.TrimSpace(string(respBody)),
				"allowed_by":   "product_api_request_allowlist",
			}
			if truncated {
				result["truncated"] = true
			}
			if resp.StatusCode >= 400 {
				result["error"] = "product API returned non-2xx status"
			}
			out, err := json.Marshal(result)
			if err != nil {
				return "", err
			}
			return string(out), nil
		},
	}
}

func normalizeProductAPIToolPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("path must not contain newlines")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("parse product API URL: %w", err)
		}
		raw = u.RequestURI()
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("path must be absolute")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", fmt.Errorf("parse product API path: %w", err)
	}
	if u.Path == "" {
		return "", fmt.Errorf("path must include an API route")
	}
	return u.RequestURI(), nil
}

func validateProductAPIToolRoute(method, requestURI string) error {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return fmt.Errorf("method %q is not allowed", method)
	}
	u, err := url.ParseRequestURI(requestURI)
	if err != nil {
		return err
	}
	path := u.Path
	for _, blocked := range []string{
		"/internal/",
		"/api/agent/",
		"/api/prompts",
		"/api/test/",
	} {
		if path == strings.TrimSuffix(blocked, "/") || strings.HasPrefix(path, blocked) {
			return fmt.Errorf("product_api_request refuses non-product route %s", path)
		}
	}
	for _, allowed := range []string{
		"/api/prompt-bar",
		"/api/universal-wire/",
		"/api/texture/",
		"/api/trace/",
		"/api/app-change-packages",
		"/api/app-change-packages/",
		"/api/computers/",
		"/api/adoptions",
		"/api/adoptions/",
		"/api/continuations",
		"/api/continuations/",
		"/api/run-acceptances",
		"/api/run-acceptances/",
	} {
		if path == strings.TrimSuffix(allowed, "/") || strings.HasPrefix(path, allowed) {
			return nil
		}
	}
	return fmt.Errorf("product_api_request route %s is not in the product-path allowlist", path)
}
