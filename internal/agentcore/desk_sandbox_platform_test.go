package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func TestGuestPlatformFollowsTheSandboxSelfTest(t *testing.T) {
	t.Cleanup(yaegikernel.ResetDeskSandboxState)
	t.Cleanup(func() { apppackage.SetPlatformSource(nil) })
	apppackage.SetPlatformSource(GuestPlatform)

	yaegikernel.ResetDeskSandboxState()
	if apppackage.CurrentPlatform().DeskSandbox || apppackage.CurrentPlatform().AdmitsDesksFrom(apppackage.OriginPublished) {
		t.Fatal("a computer whose boot selftest has not passed admitted a published desk")
	}

	passed := yaegikernel.DeskCellSelfTestReceipt{
		Mode:    string(yaegikernel.CellConfinementRequired),
		Workers: []yaegikernel.DeskCellSelfTestWorker{{Role: "research", Profile: "gvisor"}},
		Gvisor:  &yaegikernel.GvisorStatus{Available: true},
		OK:      true,
	}
	yaegikernel.RecordDeskSandboxSelfTest(passed, "boot")
	if !apppackage.CurrentPlatform().AdmitsDesksFrom(apppackage.OriginPublished) {
		t.Fatal("a computer with a passed required-mode gVisor selftest refused a published desk")
	}

	// Process-only (degraded) confinement closes it again.
	passed.Workers[0].Profile = "process"
	yaegikernel.RecordDeskSandboxSelfTest(passed, "api")
	if apppackage.CurrentPlatform().AdmitsDesksFrom(apppackage.OriginPublished) {
		t.Fatal("a degraded computer admitted a published desk")
	}
	if !apppackage.CurrentPlatform().AdmitsDesksFrom(apppackage.OriginFirstParty) {
		t.Fatal("first-party desks are always admitted")
	}
}
