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
