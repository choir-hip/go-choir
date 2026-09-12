package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRosterTaskBytesPinsDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.txt")
	body := []byte("frozen task bytes")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	pinned := hex.EncodeToString(sum[:])
	if _, got, err := rosterTaskBytes(path, pinned); err != nil || got != pinned {
		t.Fatalf("pinned task rejected: %v", err)
	}
	if _, _, err := rosterTaskBytes(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong digest accepted")
	}
}

func TestRosterTellTextCarriesOverlayAndTaskVerbatim(t *testing.T) {
	task := []byte("do the thing\n")
	text := rosterTellText("p5-deepseek-v41-flash", task)
	if !strings.HasPrefix(text, "ROSTER-V1 ") {
		t.Fatalf("tell missing version prefix: %q", text)
	}
	if !strings.Contains(text, "model_policy_overlay_id=p5-deepseek-v41-flash") {
		t.Fatalf("tell missing overlay binding: %q", text)
	}
	if !strings.HasSuffix(text, string(task)) {
		t.Fatalf("tell mutates task bytes: %q", text)
	}
}

func TestRosterPreflightEnforcesResolve(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/model-policy/resolve") {
			gotPath = r.URL.RequestURI()
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/model-policy/resolve") {
			json.NewEncoder(w).Encode(map[string]any{"provider": "opencode-go", "model": "deepseek-v4.1-flash"})
			return
		}
		if r.URL.Path == "/api/runs" {
			json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
	}))
	defer srv.Close()
	task := filepath.Join(t.TempDir(), "task.txt")
	os.WriteFile(task, []byte("task"), 0o644)
	sum := sha256.Sum256([]byte("task"))
	pinned := hex.EncodeToString(sum[:])
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	var stdout bytes.Buffer
	code := runRoster([]string{"preflight", "--overlay-id", "p5-deepseek", "--expect-provider", "opencode-go",
		"--expect-model", "deepseek-v4.1-flash", "--task-file", task, "--expected-task-sha256", pinned}, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("preflight failed: %s", stdout.String())
	}
	if !strings.Contains(gotPath, "overlay_id=p5-deepseek") || !strings.Contains(gotPath, "role=engineering") {
		t.Fatalf("resolve called wrong: %s", gotPath)
	}
	var receipt map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("preflight receipt not JSON: %v", err)
	}
	if receipt["schema"] != "choir.roster_preflight.v1" || receipt["model"] != "deepseek-v4.1-flash" {
		t.Fatalf("preflight receipt wrong: %v", receipt)
	}
}

func TestRosterPreflightRefusesModelMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"provider": "chatgpt", "model": "gpt-5.4-mini"})
	}))
	defer srv.Close()
	task := filepath.Join(t.TempDir(), "task.txt")
	os.WriteFile(task, []byte("task"), 0o644)
	sum := sha256.Sum256([]byte("task"))
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	code := runRoster([]string{"preflight", "--overlay-id", "x", "--expect-provider", "opencode-go",
		"--expect-model", "deepseek-v4.1-flash", "--task-file", task,
		"--expected-task-sha256", hex.EncodeToString(sum[:])}, io.Discard, io.Discard)
	if code == 0 {
		t.Fatal("model mismatch passed preflight (silent fallback shape)")
	}
}

func TestRosterPreflightRefusesLiveEngineeringRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/model-policy/resolve") {
			json.NewEncoder(w).Encode(map[string]any{"provider": "opencode-go", "model": "deepseek-v4.1-flash"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"runs": []any{
			map[string]any{"run_id": "run-done", "agent_profile": "engineering", "state": "completed"},
			map[string]any{"run_id": "run-legacy", "agent_profile": "cosuper", "state": "executing"},
		}})
	}))
	defer srv.Close()
	task := filepath.Join(t.TempDir(), "task.txt")
	os.WriteFile(task, []byte("task"), 0o644)
	sum := sha256.Sum256([]byte("task"))
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	code := runRoster([]string{"preflight", "--overlay-id", "x", "--expect-provider", "opencode-go",
		"--expect-model", "deepseek-v4.1-flash", "--task-file", task,
		"--expected-task-sha256", hex.EncodeToString(sum[:])}, io.Discard, io.Discard)
	if code == 0 {
		t.Fatal("live engineering run did not refuse preflight")
	}
}

func TestRosterCollectWritesTerminalReceipt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/health/ready":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case strings.HasPrefix(r.URL.Path, "/api/runs/"):
			json.NewEncoder(w).Encode(map[string]any{"run_id": "run-1", "disposition": "completed"})
		case strings.HasPrefix(r.URL.Path, "/api/trajectories/"):
			json.NewEncoder(w).Encode(map[string]any{})
		case strings.HasPrefix(r.URL.Path, "/api/costs"):
			json.NewEncoder(w).Encode(map[string]any{})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	artifact := filepath.Join(t.TempDir(), "receipt.json")
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	var stdout bytes.Buffer
	code := runRoster([]string{"collect", "--request-id", "r1", "--overlay-id", "o1",
		"--assignment", "a1", "--run", "run-1", "--trajectory", "t1",
		"--task-sha256", strings.Repeat("a", 64), "--artifact", artifact,
		"--collect-timeout", "1s", "--poll-interval", "10ms"}, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("collect failed: %s", stdout.String())
	}
	raw, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("receipt not written: %v", err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatalf("receipt not JSON: %v", err)
	}
	if receipt["schema"] != "choir.roster_receipt.v1" || receipt["assignment_id"] != "a1" {
		t.Fatalf("receipt wrong: %v", receipt)
	}
	if _, dup := os.Stat(artifact + ".tmp"); !os.IsNotExist(dup) {
		t.Fatal("temp receipt left behind (non-atomic write)")
	}
}
