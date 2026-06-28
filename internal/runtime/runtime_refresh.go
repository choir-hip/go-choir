package runtime

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

var runtimeRestartCommandMu sync.Mutex
var runtimeRestartCommand = func(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, "systemctl", "restart", "--no-block", "go-choir-sandbox.service")
}

// HandleInternalRuntimeRefresh handles POST /internal/runtime/refresh.
//
// It is deploy machinery for active guest VMs: the host has already installed a
// new sandbox runtime package, and the guest service ExecStartPre knows how to
// fetch it. Restarting the in-guest service is much cheaper than rebooting the
// whole Firecracker VM; the host deploy verifies the cutover by polling guest
// /health until the build commit matches the just-deployed revision. Small
// comment-only edits here are useful probes for the sandbox hot-refresh path.
func (h *APIHandler) HandleInternalRuntimeRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, map[string]string{"status": "restart_scheduled"})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		// Give net/http a short window to flush the 202 before systemd stops this
		// process as part of the restart.
		time.Sleep(100 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runtimeRestartCommandMu.Lock()
		cmd := runtimeRestartCommand(ctx)
		runtimeRestartCommandMu.Unlock()
		if err := cmd.Run(); err != nil {
			log.Printf("runtime: schedule sandbox service restart: %v", err)
		}
	}()
}
