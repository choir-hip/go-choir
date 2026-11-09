package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolresidency"
)

// readToolLayers reports a mounted layer's resident bytes, busy state and last
// eviction, and never fails the request. Failure modes: a busy layer reported
// idle; the status file ignored; the list nil (JSON null) instead of empty.
func TestReadToolLayersReportsResidencyBusyAndEviction(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("mincore is Linux only")
	}
	root := t.TempDir()
	dir := toolresidency.LayerDir(root, "toolchain")
	if err := os.MkdirAll(filepath.Join(dir, "aaa-go"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "aaa-go", "go")
	if err := os.WriteFile(p, make([]byte, 1<<18), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	proc := filepath.Join(root, "proc", strconv.Itoa(42))
	if err := os.MkdirAll(proc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nix/store/aaa-go/bin/go", filepath.Join(proc, "exe")); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	evicted := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	if err := toolresidency.RecordRun(state, "toolchain", evicted, true); err != nil {
		t.Fatal(err)
	}

	got := readToolLayers(t.Context(), root, filepath.Join(root, "proc"), state)
	if len(got) != 1 {
		t.Fatalf("layers = %+v", got)
	}
	l := got[0]
	if l.Layer != "toolchain" || !l.Mounted || l.ResidentBytes <= 0 || l.TotalBytes != 1<<18 ||
		!l.Busy || len(l.BusyPids) != 1 || l.BusyPids[0] != 42 || l.LastEvictedAt != "2026-11-09T10:00:00Z" || l.Error != "" {
		t.Fatalf("layer = %+v", l)
	}
	if none := readToolLayers(t.Context(), t.TempDir(), "/proc", state); none == nil || len(none) != 0 {
		t.Fatalf("no mounts = %#v, want empty non-nil", none)
	}
}

// The route always carries tool_layers (an array, empty here: no layer mounts
// in a test host) and the timer block, and mem_cached_bytes on Linux.
func TestObservabilityCarriesToolLayersAndTimer(t *testing.T) {
	rt := telemetryRuntime(t, "computer-tool-layers")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	if layers, ok := obs["tool_layers"].([]any); !ok {
		t.Fatalf("tool_layers = %#v, want an array", obs["tool_layers"])
	} else if runtime.GOOS != "linux" && len(layers) != 0 {
		t.Fatalf("tool_layers off Linux = %v", layers)
	}
	timer, _ := obs["tool_layer_timer"].(map[string]any)
	if timer["interval_seconds"] != float64(300) || timer["idle_grace_seconds"] != float64(600) {
		t.Fatalf("tool_layer_timer = %v", timer)
	}
	if runtime.GOOS == "linux" {
		if c, _ := obs["mem_cached_bytes"].(float64); c <= 0 {
			t.Fatalf("mem_cached_bytes = %v", obs["mem_cached_bytes"])
		}
	}
}
