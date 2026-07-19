package sandbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestBuildRuntimeConfigPreservesHostServiceURLs(t *testing.T) {
	cfg := Config{
		SandboxID: "vm-test",
		StorePath: "/tmp/runtime.db",
	}
	loaded := provideriface.Config{
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

func TestBuildRuntimeConfigDerivesCanonicalModelPolicyPath(t *testing.T) {
	got := buildRuntimeConfig(Config{SandboxID: "vm-test"}, provideriface.Config{}, "/files")
	if got.ModelPolicyPath != "/files/System/model-policy.toml" {
		t.Fatalf("ModelPolicyPath = %q, want canonical files path", got.ModelPolicyPath)
	}
}

func TestConsumeComputerCredentialEnvelopeErasesSingleUseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "computer-event-envelope")
	if err := os.WriteFile(path, []byte("encoded-envelope\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	encoded, err := consumeComputerCredentialEnvelopeOwned(path, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "encoded-envelope" {
		t.Fatalf("credential = %q", encoded)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("consumed credential remains readable: %v", err)
	}
}

func TestConsumeComputerCredentialEnvelopeRejectsLooseMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "computer-event-envelope")
	if err := os.WriteFile(path, []byte("encoded-envelope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := consumeComputerCredentialEnvelopeOwned(path, uint32(os.Getuid())); err == nil {
		t.Fatal("mode-0600 bootstrap credential was accepted")
	}
}

func TestRunZotSessionUsesProcessConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZOT_SESSION_ID", "entry-test")
	t.Setenv("ZOT_ROOT_DIR", root)
	t.Setenv("ZOT_USER_ID", "entry@example.com")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := RunZotSession(strings.NewReader("quit\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("RunZotSession code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "zot repair session entry-test") {
		t.Fatalf("stdout = %q, want configured session ID", stdout.String())
	}
	logPath := filepath.Join(root, ".choir", "zot", "sessions", "entry-test", "session.jsonl")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("session log: %v", err)
	}
}
