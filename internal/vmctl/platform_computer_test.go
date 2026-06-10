package vmctl

import (
	"testing"
)

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
			HostURL: "http://10.203.140.2:8085",
			Epoch:   3,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)

	err := reg.EnsureUniversalWirePlatformComputer(t.Context())
	if err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer: %v", err)
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
