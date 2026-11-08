package agentcore

import (
	"context"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// GuestPlatform is the isolation THIS computer has, for the guest runtime to
// name as its apppackage platform source (internal/autoputer). The iframe
// sandbox is a build property (apppackage.IframeSandboxShipped); the worker
// sandbox is a reading of this computer: required confinement, gVisor, and an
// isolation selftest that passed (the boot run, or the latest owner run of
// POST /api/runtime/isolation-selftest). A computer whose selftest failed, has
// not run it yet, or runs process-only confinement answers false, so a desk
// from another publisher keeps being refused here. No setting reaches it.
//
// Per caller (each reads apppackage.CurrentPlatform):
//   - guest install, the release record's validation and pkgruntime.Build run
//     in this process, so they use this answer: a non-first-party desk
//     installs only on a computer whose own selftest passed;
//   - host-side publish validation (corpusd, proxy) cannot see a guest's
//     sandbox; those binaries name apppackage.ShippedPlatform, which admits a
//     release to the Library by what the build ships. The guest still decides
//     at install.
func GuestPlatform() apppackage.Platform {
	return apppackage.Platform{
		DeskSandbox: apppackage.IframeSandboxShipped && yaegikernel.DeskSandboxReady(),
	}
}

// runDeskSandboxBootSelfTest is the boot selftest of the worker sandbox. In
// required mode it runs the desk selftest once and records the result; in any
// other mode it records "not ready" and spawns nothing. A computer that
// crashed and rebooted runs it again: the state is per process, never carried.
func (rt *Runtime) runDeskSandboxBootSelfTest(ctx context.Context) {
	if !isolationSelfTestMu.TryLock() {
		return // an owner run is in flight and records its own result
	}
	defer isolationSelfTestMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	st := yaegikernel.RunDeskSandboxBootSelfTest(ctx, deskCellSelfTestConfig(rt.cfg.ComputerID))
	log.Printf("runtime: desk sandbox boot selftest ready=%t reason=%q", st.Ready, st.Reason)
}
