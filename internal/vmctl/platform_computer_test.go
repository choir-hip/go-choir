package vmctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUniversalWirePlatformRuntimeEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "platform-wire-runtime.env")
	env := UniversalWirePlatformRuntimeEnv{
		RuntimeBaseURL: "http://127.0.0.1:8085",
		OwnerID:        UniversalWirePlatformOwnerID,
	}
	if err := WriteUniversalWirePlatformRuntimeEnv(path, env); err != nil {
		t.Fatalf("WriteUniversalWirePlatformRuntimeEnv: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(raw)
	for _, want := range []string{
		"SOURCE_SERVICE_RUNTIME_BASE_URL=http://127.0.0.1:8085",
		"SOURCE_SERVICE_RUNTIME_OWNER_ID=universal-wire-platform",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in %q", want, content)
		}
	}
}

func TestComputerKindForOwnershipPlatform(t *testing.T) {
	own := &VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		WarmnessClass: WarmnessClassPublicPlatform,
	}
	if got := computerKindForOwnership(own); got != "platform" {
		t.Fatalf("computerKindForOwnership() = %q, want platform", got)
	}
}

func TestWarmnessClassProtectedIncludesPublicPlatform(t *testing.T) {
	if !warmnessClassProtected(WarmnessClassPublicPlatform) {
		t.Fatal("expected public_platform warmness class to be protected from idle reclaim")
	}
}

func TestEnsureUniversalWirePlatformComputerBootsStableVM(t *testing.T) {
	mgr := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:8085",
			Epoch:   3,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)

	env, err := reg.EnsureUniversalWirePlatformComputer(t.Context())
	if err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer: %v", err)
	}
	if env.RuntimeBaseURL != "http://127.0.0.1:8085" {
		t.Fatalf("RuntimeBaseURL = %q", env.RuntimeBaseURL)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("expected one boot, got %d", len(mgr.boots))
	}
	if mgr.boots[0].VMID != UniversalWirePlatformVMID {
		t.Fatalf("boot VMID = %q, want %q", mgr.boots[0].VMID, UniversalWirePlatformVMID)
	}
	if mgr.boots[0].ComputerKind != "platform" {
		t.Fatalf("ComputerKind = %q, want platform", mgr.boots[0].ComputerKind)
	}
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	own := reg.ownerships[key]
	if own == nil || own.WarmnessClass != WarmnessClassPublicPlatform {
		t.Fatalf("expected public_platform ownership, got %#v", own)
	}
}
