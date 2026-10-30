package autoputer

import (
	"os"
	"path/filepath"
	"reflect"
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

// B12b (docs/archive/problems/reeng-cut2-breakage-2026-10-13.md): buildRuntimeConfig
// copies the runtime config field by field, and a field it leaves out is
// read from the guest's environment and then dropped (CHOIR_PROXY_URL was).
// Every string field LoadConfig sets must survive the copy, except the ones
// the copy takes from the computer's own config.
func TestBuildRuntimeConfigKeepsEveryLoadedStringField(t *testing.T) {
	fromComputer := map[string]bool{"ComputerID": true, "StorePath": true}
	var loaded provideriface.Config
	v := reflect.ValueOf(&loaded).Elem()
	for i := 0; i < v.NumField(); i++ {
		if f := v.Field(i); f.Kind() == reflect.String && f.CanSet() {
			f.SetString("set-" + v.Type().Field(i).Name)
		}
	}
	t.Setenv("CHOIR_PLATFORM_URL", "")
	got := reflect.ValueOf(buildRuntimeConfig(Config{ComputerID: "vm-test", StorePath: "/store"}, loaded, "/files"))
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if v.Field(i).Kind() != reflect.String || fromComputer[name] {
			continue
		}
		if got.Field(i).String() != v.Field(i).String() {
			t.Errorf("buildRuntimeConfig drops %s: got %q, loaded %q", name, got.Field(i).String(), v.Field(i).String())
		}
	}
}
