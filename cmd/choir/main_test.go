package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientTimeoutPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want time.Duration
	}{
		{name: "default", want: 75 * time.Second},
		{name: "environment", env: "90s", want: 90 * time.Second},
		{name: "flag overrides environment", env: "90s", args: []string{"--timeout=2m"}, want: 2 * time.Minute},
		{name: "valid flag ignores invalid environment", env: "eventually", args: []string{"--timeout=2m"}, want: 2 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, tt.env)
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			args := append([]string{"--api-key=choir_sk_test"}, tt.args...)
			c, err := newClient(fs, args, io.Discard, io.Discard)
			if err != nil {
				t.Fatalf("newClient() error = %v", err)
			}
			if c.http.Timeout != tt.want {
				t.Fatalf("timeout = %s, want %s", c.http.Timeout, tt.want)
			}
		})
	}
}

func TestClientRejectsInvalidTimeout(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		args    []string
		wantErr string
	}{
		{name: "invalid environment", env: "eventually", wantErr: "$CHOIR_TIMEOUT must be a valid duration"},
		{name: "zero environment", env: "0s", wantErr: "$CHOIR_TIMEOUT must be greater than zero"},
		{name: "negative flag", env: "90s", args: []string{"--timeout=-1s"}, wantErr: "--timeout must be greater than zero"},
		{name: "invalid flag overrides environment", env: "90s", args: []string{"--timeout=soon"}, wantErr: "--timeout must be a valid duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, tt.env)
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			args := append([]string{"--api-key=choir_sk_test"}, tt.args...)
			_, err := newClient(fs, args, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("newClient() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRequestTimeoutCancelsDelayedServer(t *testing.T) {
	t.Setenv(timeoutEnvVar, "")
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer stub.Close()

	started := time.Now()
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--api-key=choir_sk_test", "--host=" + stub.URL, "--timeout=30ms"}, &out, &errOut)
	elapsed := time.Since(started)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "context deadline exceeded") {
		t.Fatalf("stderr = %q, want context deadline exceeded", errOut.String())
	}
	if elapsed > time.Second {
		t.Fatalf("request elapsed = %s, want cancellation within 1s", elapsed)
	}
}

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
func TestTrajectoryCancelPostsBodylessEscapedRequest(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.URL.EscapedPath(); got != "/api/trajectories/traj%2Fwith%20space/cancel" {
			t.Errorf("escaped path = %q, want /api/trajectories/traj%%2Fwith%%20space/cancel", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("body = %q, want empty", body)
		}
		_, _ = io.WriteString(w, `{"trajectory_id":"traj/with space","status":"cancelled","cancelled_run_ids":[]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--api-key=choir_sk_test", "--host=" + stub.URL, "traj/with space"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
}

func TestTrajectoryCancelPrintsServerJSON(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectory_id":"traj-1","status":"cancelled","cancelled_run_ids":["run-1","run-2"]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--api-key=choir_sk_test", "--host=" + stub.URL, "traj-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp struct {
		TrajectoryID    string   `json:"trajectory_id"`
		Status          string   `json:"status"`
		CancelledRunIDs []string `json:"cancelled_run_ids"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if resp.TrajectoryID != "traj-1" || resp.Status != "cancelled" || !reflect.DeepEqual(resp.CancelledRunIDs, []string{"run-1", "run-2"}) {
		t.Fatalf("response = %+v, want cancelled traj-1 with run-1 and run-2", resp)
	}
}

func TestTrajectoryCancelRequiresID(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--api-key=choir_sk_test"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "choir trajectory cancel: trajectory id required") {
		t.Fatalf("stderr = %q, want trajectory id usage error", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestTrajectoryCancelReportsServerError(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "cannot cancel trajectory", http.StatusConflict)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--api-key=choir_sk_test", "--host=" + stub.URL, "traj-1"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "choir trajectory cancel traj-1: http 409: cannot cancel trajectory") {
		t.Fatalf("stderr = %q, want server error", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestTrajectoryGetCompatibility(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/trajectories/traj-1" {
			t.Errorf("path = %q, want /api/trajectories/traj-1", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"trajectory_id":"traj-1","status":"live"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "--api-key=choir_sk_test", "--host=" + stub.URL, "traj-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if resp["trajectory_id"] != "traj-1" || resp["status"] != "live" {
		t.Fatalf("response = %v, want live traj-1", resp)
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

func TestAPIKeyEnvironmentSecretNeverAppearsInHelp(t *testing.T) {
	const secret = "choir_sk_verifier_known_secret_must_not_render"
	t.Setenv(apiKeyEnvVar, secret)

	fs := flag.NewFlagSet("secret-redaction", flag.ContinueOnError)
	var flagHelp bytes.Buffer
	fs.SetOutput(&flagHelp)
	if _, err := newClient(fs, []string{"--help"}, io.Discard, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("newClient help error = %v, want flag.ErrHelp", err)
	}
	if strings.Contains(flagHelp.String(), secret) {
		t.Fatalf("shared client flag help leaked environment secret: %s", flagHelp.String())
	}
	if !strings.Contains(flagHelp.String(), "$CHOIR_API_KEY") {
		t.Fatalf("shared client flag help omitted safe environment hint: %s", flagHelp.String())
	}

	for _, args := range [][]string{
		{"wire", "stories", "--help"},
		{"texture", "read", "--help"},
		{"run", "list", "--help"},
		{"computer", "status", "--help"},
		{"api-key", "list", "--help"},
		{"api-key", "create", "--help"},
		{"api-key", "revoke", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		run(args, &stdout, &stderr)
		if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
			t.Fatalf("%v help leaked environment secret: stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
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

func TestRunListHitsAgentLoopsEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/loops" {
			t.Errorf("path = %q, want /api/agent/loops", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.URL.Query().Get("limit"); got != "7" {
			t.Errorf("limit = %q, want 7", got)
		}
		_, _ = io.WriteString(w, `{"runs":[{"loop_id":"run-123","state":"running"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "list", "--api-key=choir_sk_test", "--host=" + stub.URL, "--limit=7"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp["runs"].([]any)) != 1 {
		t.Fatalf("runs = %v, want one run", resp["runs"])
	}
}

func TestRunCancelPostsAgentCancelRequest(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/cancel" {
			t.Errorf("path = %q, want /api/agent/cancel", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["loop_id"] != "run-123" {
			t.Fatalf("loop_id = %q, want run-123", body["loop_id"])
		}
		_, _ = io.WriteString(w, `{"loop_id":"run-123","state":"cancelled"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "cancel", "--api-key=choir_sk_test", "--host=" + stub.URL, "run-123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["state"] != "cancelled" {
		t.Fatalf("state = %v, want cancelled", resp["state"])
	}
}

func TestComputerLifecycleCommandsUseProductComputeAPI(t *testing.T) {
	var requests []struct {
		method string
		path   string
		action string
	}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := ""
		if r.Method == http.MethodPost {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode lifecycle body: %v", err)
			}
			action = body["action"]
		}
		requests = append(requests, struct {
			method string
			path   string
			action string
		}{method: r.Method, path: r.URL.Path, action: action})
		_, _ = io.WriteString(w, `{"ok":true,"status":"ok","current_computer":{"state":"stopped"}}`)
	}))
	defer stub.Close()

	for _, command := range []string{"status", "stop", "start"} {
		var out, errOut bytes.Buffer
		code := run([]string{"computer", command, "--api-key=choir_sk_test", "--host=" + stub.URL}, &out, &errOut)
		if code != 0 {
			t.Fatalf("computer %s code = %d, stderr=%s", command, code, errOut.String())
		}
	}
	want := []struct {
		method string
		path   string
		action string
	}{
		{method: http.MethodGet, path: "/api/compute/status"},
		{method: http.MethodPost, path: "/api/compute/recovery", action: "stop_current_computer"},
		{method: http.MethodPost, path: "/api/compute/recovery", action: "wake_current_computer"},
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("lifecycle requests = %+v, want %+v", requests, want)
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
