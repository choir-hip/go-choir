package runtime

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

	"github.com/yusefmosiah/go-choir/internal/server"
)

const productAPIToolMaxBodyBytes = 1 << 20

func newProductAPIRequestTool(rt *Runtime) Tool {
	type args struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body,omitempty"`
	}
	return Tool{
		Name:        "product_api_request",
		Description: "Call an allowed authenticated product API route in the current runtime using the run owner as the authenticated user. This is for foreground super product-path orchestration; it refuses internal, test, agent, prompt-config, and raw event mutation routes.",
		Parameters: jsonSchemaObject(map[string]any{
			"method": map[string]any{"type": "string", "enum": []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}},
			"path":   map[string]any{"type": "string", "description": "Absolute API path, optionally with query string, such as /api/universal-wire/stories."},
			"body":   map[string]any{"type": "object", "description": "Optional JSON body for POST/PUT requests."},
		}, []string{"method", "path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode product_api_request args: %w", err)
			}
			if rt == nil {
				return "", fmt.Errorf("product_api_request missing runtime")
			}
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("product_api_request is only available to foreground super")
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
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
			req := httptest.NewRequest(method, path, body)
			req = req.WithContext(ctx)
			req.Header.Set("X-Authenticated-User", ownerID)
			if ownerEmail := stringFromToolContext(ctx, toolCtxOwnerEmail); ownerEmail != "" {
				req.Header.Set("X-Authenticated-Email", ownerEmail)
			}
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			mux := server.NewServer("runtime-product-api-tool", "0")
			RegisterRoutes(mux, NewAPIHandler(rt))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
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
			return toolResultJSON(result)
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
		// Temporary compatibility shim during the Texture route cutover.
		"/api/vtext/",
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
