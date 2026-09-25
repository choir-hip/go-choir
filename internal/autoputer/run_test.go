package autoputer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestBuildRuntimeConfigDerivesCanonicalModelPolicyPath(t *testing.T) {
	got := buildRuntimeConfig(Config{ComputerID: "vm-test"}, provideriface.Config{}, "/files")
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

func TestSelfDevelopmentUpdaterOptionRejectsRelativeSocket(t *testing.T) {
	t.Setenv("CHOIR_UPDATER_ROOT", t.TempDir())
	t.Setenv("CHOIR_UPDATER_SOCKET", "relative.sock")
	_, _, err := selfDevelopmentUpdaterOption()
	if err == nil {
		t.Fatal("expected relative socket to fail")
	}
}

func TestSelfDevelopmentRouteOptionRejectsNonHTTPURL(t *testing.T) {
	t.Setenv("RUNTIME_VMCTL_URL", "/var/run/vmctl.sock")
	t.Setenv("CHOIR_OWNER_ID", "owner-test")
	_, _, err := selfDevelopmentRouteOption()
	if err == nil {
		t.Fatal("expected non-http vmctl URL to fail")
	}
}

func TestSelfDevelopmentVerifierOptionRejectsRelativeSocket(t *testing.T) {
	t.Setenv("CHOIR_VERIFIER_AUTHORITY_SOCKET", "authority.sock")
	_, _, err := selfDevelopmentVerifierOption()
	if err == nil {
		t.Fatal("expected relative verifier socket to fail")
	}
}
