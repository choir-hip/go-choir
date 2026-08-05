package sandbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
func TestHoldSupervisionWritesDisabledForReplayRestoresRequestedMode(t *testing.T) {
	const envName = "CHOIR_SUPERVISION_WRITES_DISABLED"
	tests := []struct {
		name         string
		before       string
		beforeSet    bool
		wantDisabled bool
	}{
		{name: "enabled unset", wantDisabled: false},
		{name: "enabled empty", beforeSet: true, wantDisabled: false},
		{name: "disabled", before: "1", beforeSet: true, wantDisabled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.beforeSet {
				t.Setenv(envName, test.before)
			} else {
				_ = os.Unsetenv(envName)
				t.Cleanup(func() { _ = os.Unsetenv(envName) })
			}

			disabledAfterReplay, restore := holdSupervisionWritesDisabledForReplay()
			if disabledAfterReplay != test.wantDisabled {
				t.Fatalf("disabledAfterReplay = %t, want %t", disabledAfterReplay, test.wantDisabled)
			}
			if got := os.Getenv(envName); got != "1" {
				t.Fatalf("replay mode = %q, want disabled", got)
			}

			restore()
			got, gotSet := os.LookupEnv(envName)
			if gotSet != test.beforeSet || got != test.before {
				t.Fatalf("restored mode = (%q, %t), want (%q, %t)", got, gotSet, test.before, test.beforeSet)
			}
		})
	}
}
func TestResolveStartupSupervisionReleaseUsesImmutableGuestFloorBeforeBaselineImport(t *testing.T) {
	raw := []byte("contract=choir-guest-image-v1\nbuild_commit=test\nsandbox=/nix/store/sandbox-test\n")
	guestManifestPath := filepath.Join(t.TempDir(), "guest-image-manifest")
	if err := os.WriteFile(guestManifestPath, raw, 0o444); err != nil {
		t.Fatal(err)
	}

	manifest, err := resolveStartupSupervisionRelease(t.TempDir(), guestManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Marker != "immutable-guest-image" ||
		manifest.ContentDigest != computerevent.DigestBytes(raw) ||
		manifest.Supervision == nil {
		t.Fatalf("startup supervision release = %+v", manifest)
	}
}
func TestResolveStartupSupervisionReleaseRefusesMalformedExistingCurrent(t *testing.T) {
	guestManifestPath := filepath.Join(t.TempDir(), "guest-image-manifest")
	if err := os.WriteFile(guestManifestPath, []byte("contract=choir-guest-image-v1\nsandbox=/nix/store/sandbox-test\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		createTarget bool
	}{
		{name: "release missing manifest", createTarget: true},
		{name: "dangling current"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "releases", "candidate")
			if test.createTarget {
				if err := os.MkdirAll(filepath.Join(target, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "bin", "sandbox"), []byte("candidate"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(root, "current")); err != nil {
				t.Fatal(err)
			}
			if _, err := resolveStartupSupervisionRelease(root, guestManifestPath); err == nil {
				t.Fatal("malformed existing current used immutable fallback")
			}
		})
	}
}

func TestResolveStartupSupervisionReleaseRejectsUnboundGuestManifest(t *testing.T) {
	guestManifestPath := filepath.Join(t.TempDir(), "guest-image-manifest")
	if err := os.WriteFile(guestManifestPath, []byte("contract=wrong\nsandbox=/tmp/sandbox\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveStartupSupervisionRelease(t.TempDir(), guestManifestPath); err == nil {
		t.Fatal("unbound guest image manifest was accepted")
	}
}

func TestBuildRuntimeConfigDerivesCanonicalModelPolicyPath(t *testing.T) {
	got := buildRuntimeConfig(Config{SandboxID: "vm-test"}, provideriface.Config{}, "/files")
	if got.ModelPolicyPath != "/files/System/model-policy.toml" {
		t.Fatalf("ModelPolicyPath = %q, want canonical files path", got.ModelPolicyPath)
	}
}

func TestComputerCredentialEnvelopeRemainsUntilExplicitDurableConsumption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "computer-event-envelope")
	if err := os.WriteFile(path, []byte("encoded-envelope\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	encoded, err := readComputerCredentialEnvelopeOwned(path, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "encoded-envelope" {
		t.Fatalf("credential = %q", encoded)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("credential disappeared before durable consumption: %v", err)
	}
	if err := eraseComputerCredentialEnvelopeOwned(path, uint32(os.Getuid())); err != nil {
		t.Fatal(err)
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
	if _, err := readComputerCredentialEnvelopeOwned(path, uint32(os.Getuid())); err == nil {
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
