package vmctl

import (
	"context"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostboot"
)

// hostBootRoot is the filesystem root the boot-state detector reads ("/" on
// Node B; a fixture tree in tests).
var hostBootRoot = "/"

// HostBootStatePath serves the installed-but-not-booted fact
// (docs/reports/patch-lanes-plan-2026-10-15.md, slice 1).
const HostBootStatePath = "/internal/vmctl/host/boot-state"

// HandleHostBootState handles GET /internal/vmctl/host/boot-state: whether
// Node B runs the kernel, modules, command line and KVM module parameters it
// last activated. Computed per request from /run, /proc and /sys, so it is
// never stale. Internal only: a public "kernel X, reboot pending" names the
// exploits that work, so public /health never carries it.
func (h *Handler) HandleHostBootState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, hostboot.Read(hostBootRoot, time.Now()))
}

// HostBootStateLogInterval is how often vmctl writes the boot-state journal
// line after the one at start.
const HostBootStateLogInterval = 24 * time.Hour

// StartHostBootStateLog writes the boot-state fact as one journal line at
// start and every interval until ctx ends, so a booted kernel that lags the
// activated one shows without anyone calling the route
// (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md, 2026-10-22).
// Read-only; the line stays in the host journal, never on public /health.
func StartHostBootStateLog(ctx context.Context, interval time.Duration, logf func(string, ...any)) {
	if interval <= 0 {
		interval = HostBootStateLogInterval
	}
	write := func() { logf("vmctl: %s", hostboot.Read(hostBootRoot, time.Now()).LogLine()) }
	go func() {
		write()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				write()
			}
		}
	}()
}
