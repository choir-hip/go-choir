package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The data volume section of guest observability: statfs on every read, the
// walk only with ?data_volume=detail (a request shape, not a switch), the
// volume root taken from the store's own path, and the last walk shown on a
// plain read.

func readDataVolume(t *testing.T, rt *Runtime, query string) map[string]json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability"+query, nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	raw, ok := obs["data_volume"]
	if !ok {
		t.Fatalf("observability has no data_volume: %s", rec.Body.String())
	}
	var dv map[string]json.RawMessage
	if err := json.Unmarshal(raw, &dv); err != nil {
		t.Fatal(err)
	}
	return dv
}

func TestObservabilityReportsTheDataVolume(t *testing.T) {
	rt := telemetryRuntime(t, "computer-data-volume")
	volume := t.TempDir()
	rt.cfg.StorePath = filepath.Join(volume, "state")
	if err := os.MkdirAll(filepath.Join(volume, "go-build-cache", "ab"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volume, "go-build-cache", "ab", "obj"), make([]byte, 8192), 0o644); err != nil {
		t.Fatal(err)
	}

	plain := readDataVolume(t, rt, "")
	if _, ok := plain["walk"]; ok {
		t.Fatalf("a plain read walked the volume: %s", plain["walk"])
	}
	var root string
	var raw struct {
		Total    uint64 `json:"total_bytes"`
		Free     uint64 `json:"free_bytes"`
		Avail    uint64 `json:"avail_bytes"`
		Reserved uint64 `json:"reserved_bytes"`
	}
	if err := json.Unmarshal(plain["root"], &root); err != nil || root != volume {
		t.Fatalf("root = %s, want the store's directory %s", plain["root"], volume)
	}
	if err := json.Unmarshal(plain["statfs"], &raw); err != nil || raw.Total == 0 || raw.Free < raw.Avail || raw.Reserved != raw.Free-raw.Avail {
		t.Fatalf("statfs = %s", plain["statfs"])
	}

	detail := readDataVolume(t, rt, "?data_volume=detail")
	var walk struct {
		Truncated bool `json:"truncated"`
		Top       []struct {
			Name           string `json:"name"`
			AllocatedBytes uint64 `json:"allocated_bytes"`
			ApparentBytes  uint64 `json:"apparent_bytes"`
			Files          uint64 `json:"files"`
			Truncated      bool   `json:"truncated"`
		} `json:"top"`
		Detail []struct {
			Path     string `json:"path"`
			Children []struct {
				Name string `json:"name"`
			} `json:"children"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(detail["walk"], &walk); err != nil || len(walk.Top) == 0 {
		t.Fatalf("walk = %s", detail["walk"])
	}
	var found bool
	for _, e := range walk.Top {
		if e.Name == "go-build-cache" {
			found = e.Files == 1 && e.ApparentBytes >= 8192 && e.AllocatedBytes >= 8192 && !e.Truncated
		}
	}
	if !found {
		t.Fatalf("go-build-cache row missing or wrong: %s", detail["walk"])
	}
	t.Logf("data_volume sample: %s", dataVolumeSampleJSON(detail))

	// A plain read now shows the last walk instead of walking again.
	if again := readDataVolume(t, rt, ""); len(again["walk"]) == 0 {
		t.Fatal("a plain read after a walk shows no walk")
	}
}

func dataVolumeSampleJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
