package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunRequiresAPIKey asserts the CLI fails fast with a clear error when
// no API key is supplied, without making a network request.
func TestRunRequiresAPIKey(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "") // isolate from a developer's real key
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "api key required") {
		t.Fatalf("stderr = %q, want it to mention api key required", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on auth failure", out.String())
	}
}

// TestRunRejectsBadKeyPrefix asserts the CLI rejects a key without the
// choir_sk_ prefix before any network call.
func TestRunRejectsBadKeyPrefix(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--api-key=not-a-choir-key"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "must start with") {
		t.Fatalf("stderr = %q, want it to mention prefix requirement", errOut.String())
	}
}

// TestWireStoriesHitsAPI starts a stub server that asserts the CLI sends the
// Bearer API key header, and returns a canned wire stories response that the
// CLI must decode and print as JSON.
func TestWireStoriesHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/universal-wire/stories" {
			t.Errorf("path = %q, want /api/universal-wire/stories", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[{"id":"s1","headline":"Test headline","dek":"test dek","story_texture_doc_id":"doc-1","source_state":"fresh"}],"source":"universal-wire-edition-texture","diagnostics":{"texture_edition":"present"}}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp wireStoriesResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if len(resp.Stories) != 1 || resp.Stories[0].Headline != "Test headline" {
		t.Fatalf("stories = %+v, want one story with headline Test headline", resp.Stories)
	}
	if resp.Source != "universal-wire-edition-texture" {
		t.Fatalf("source = %q, want universal-wire-edition-texture", resp.Source)
	}
}

// TestWireDiagnosticsPrintsOnlyDiagnostics asserts the diagnostics subcommand
// decodes and prints the diagnostics field.
func TestWireDiagnosticsPrintsOnlyDiagnostics(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[],"source":"universal-wire-edition-texture","diagnostics":{"texture_edition":"missing","reason":"alias not bootstrapped"}}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "diagnostics", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var diag map[string]string
	if err := json.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("decode diagnostics: %v; stdout=%s", err, out.String())
	}
	if diag["texture_edition"] != "missing" {
		t.Fatalf("texture_edition = %q, want missing", diag["texture_edition"])
	}
}

// TestTrajectoriesHitsAPI asserts the trajectories command hits the right
// path and decodes the list.
func TestTrajectoriesHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/trajectories" {
			t.Errorf("path = %q, want /api/trajectories", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectories":[{"trajectory_id":"traj-1","kind":"ingestion"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectories", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp trajectoriesListResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp.Trajectories) != 1 || resp.Trajectories[0].TrajectoryID != "traj-1" {
		t.Fatalf("trajectories = %+v, want one traj-1", resp.Trajectories)
	}
}

// TestTrajectoriesDecodesObjectSettlementRule asserts the trajectories
// command handles settlement_rule as the JSON object the API actually
// returns (e.g. {"require_no_open_work_items":true}), not a string.
func TestTrajectoriesDecodesObjectSettlementRule(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectories":[{"trajectory_id":"traj-1","kind":"document","status":"live","subject_refs":{"channel_id":"ch-1"},"settlement_rule":{"require_no_open_work_items":true}}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectories", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp trajectoriesListResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp.Trajectories) != 1 || resp.Trajectories[0].Status != "live" {
		t.Fatalf("trajectories = %+v, want one live trajectory", resp.Trajectories)
	}
	if !strings.Contains(string(resp.Trajectories[0].SettlementRule), "require_no_open_work_items") {
		t.Fatalf("settlement_rule = %s, want the rule object passed through", resp.Trajectories[0].SettlementRule)
	}
}

// TestTextureRevisionsHitsAPI asserts the texture revisions command GETs the
// revisions endpoint, which returns full content bodies.
func TestTextureRevisionsHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/texture/documents/doc-1/revisions" {
			t.Errorf("path = %q, want /api/texture/documents/doc-1/revisions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"revisions":[{"revision_id":"rev-1","doc_id":"doc-1","content":"hello"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"texture", "revisions", "--api-key=choir_sk_test", "--host=" + stub.URL, "doc-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"content": "hello"`) {
		t.Fatalf("stdout = %q, want revision content", out.String())
	}
}

// TestAPIKeyFromEnv asserts the CLI reads the API key from CHOIR_API_KEY.
func TestAPIKeyFromEnv(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "choir_sk_fromenv")
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_fromenv" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_fromenv", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[],"source":"universal-wire-edition-texture"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
}

// TestNon2xxReturnsError asserts the CLI surfaces a non-2xx response as an
// error with the status code and body.
func TestNon2xxReturnsError(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"authentication required"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "http 401") {
		t.Fatalf("stderr = %q, want it to mention http 401", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on error", out.String())
	}
}

// TestUnknownCommand asserts unknown commands print usage and exit 2.
func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", errOut.String())
	}
}

// TestRunStartPostsToPromptBar asserts the run start command POSTs to
// /api/prompt-bar with the prompt text and decodes the submission response.
func TestRunStartPostsToPromptBar(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/prompt-bar" {
			t.Errorf("path = %q, want /api/prompt-bar", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["text"] != "hello world" {
			t.Fatalf("text = %q, want hello world", body["text"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"submission_id":"sub-123","state":"pending","created_at":"2026-07-06T00:00:00.000Z","status_url":"/api/prompt-bar/submissions/sub-123"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "start", "--api-key=choir_sk_test", "--host=" + stub.URL, "hello", "world"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp promptBarSubmitResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp.SubmissionID != "sub-123" {
		t.Fatalf("submission_id = %q, want sub-123", resp.SubmissionID)
	}
}

// TestRunStartRequiresText asserts the run start command fails when no prompt
// text is provided.
func TestRunStartRequiresText(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"run", "start", "--api-key=choir_sk_test"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "prompt text required") {
		t.Fatalf("stderr = %q, want prompt text required", errOut.String())
	}
}

// TestRunStatusHitsSubmissionEndpoint asserts the run status command GETs the
// submission status endpoint.
func TestRunStatusHitsSubmissionEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/prompt-bar/submissions/sub-123" {
			t.Errorf("path = %q, want /api/prompt-bar/submissions/sub-123", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"submission_id":"sub-123","state":"completed","created_at":"2026-07-06T00:00:00.000Z","updated_at":"2026-07-06T00:01:00.000Z"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "status", "--api-key=choir_sk_test", "--host=" + stub.URL, "sub-123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["state"] != "completed" {
		t.Fatalf("state = %v, want completed", resp["state"])
	}
}

// TestAPIKeyListHitsAuthEndpoint asserts the api-key list command GETs
// /auth/api-keys with the Bearer token.
func TestAPIKeyListHitsAuthEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys" {
			t.Errorf("path = %q, want /auth/api-keys", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"keys":[{"id":"ak_1","label":"CLI key","scopes":["read:texture"],"created_at":"2026-07-06T00:00:00Z"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "list", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	keys, ok := resp["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("keys = %v, want one key", resp["keys"])
	}
}

// TestAPIKeyCreatePostsToAuthEndpoint asserts the api-key create command POSTs
// to /auth/api-keys with label and scopes.
func TestAPIKeyCreatePostsToAuthEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys" {
			t.Errorf("path = %q, want /auth/api-keys", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["label"] != "Devin CLI" {
			t.Fatalf("label = %v, want Devin CLI", body["label"])
		}
		scopes, ok := body["scopes"].([]any)
		if !ok || len(scopes) != 1 || scopes[0] != "read:texture" {
			t.Fatalf("scopes = %v, want [read:texture]", body["scopes"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ak_new","label":"Devin CLI","scopes":["read:texture"],"secret":"choir_sk_newkey"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "create", "--label=Devin CLI", "--scopes=read:texture", "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["secret"] != "choir_sk_newkey" {
		t.Fatalf("secret = %v, want choir_sk_newkey", resp["secret"])
	}
}

// TestAPIKeyRevokeDeletesKey asserts the api-key revoke command DELETEs the
// specified key.
func TestAPIKeyRevokeDeletesKey(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys/ak_123" {
			t.Errorf("path = %q, want /auth/api-keys/ak_123", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "revoke", "--api-key=choir_sk_test", "--host=" + stub.URL, "ak_123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "ak_123") {
		t.Fatalf("stdout = %q, want it to mention ak_123", out.String())
	}
}
