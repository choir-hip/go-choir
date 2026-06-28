package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHandleInternalRuntimeRefreshRequiresInternalCaller(t *testing.T) {
	handler := &APIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/refresh", nil)
	w := httptest.NewRecorder()

	handler.HandleInternalRuntimeRefresh(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestHandleInternalRuntimeRefreshSchedulesServiceRestart(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "restart-called")
	runtimeRestartCommandMu.Lock()
	previous := runtimeRestartCommand
	runtimeRestartCommand = func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "printf called > \"$1\"", "sh", marker)
	}
	runtimeRestartCommandMu.Unlock()
	t.Cleanup(func() {
		runtimeRestartCommandMu.Lock()
		runtimeRestartCommand = previous
		runtimeRestartCommandMu.Unlock()
	})

	handler := &APIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/refresh", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()

	handler.HandleInternalRuntimeRefresh(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "called" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("restart command was not called before deadline")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
