package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type healthStubProvider struct{}

func (healthStubProvider) Execute(context.Context, *types.RunRecord, provideriface.EventEmitFunc) error {
	return nil
}
func (healthStubProvider) ProviderName() string { return "stub" }

// S2-e: when the layering boot-loop guard refused the applied release's exec
// and the base runtime is serving instead, /health must report
// layering_bootguard at 503 so the updater's identity probe fails closed
// rather than recording a false-healthy apply of a binary that never ran.
func TestHandleHealthReportsLayeringBootguard(t *testing.T) {
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	rt := &Runtime{cfg: provideriface.Config{ComputerID: "computer-guard"}, store: productStore, provider: healthStubProvider{}}
	rt.health = types.HealthReady
	handler := &APIHandler{rt: rt}

	t.Setenv("CHOIR_LAYERING_BOOTGUARD", "some-release-digest")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.HandleHealth(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("bootguarded /health status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status            string `json:"status"`
		LayeringBootguard string `json:"layering_bootguard"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "layering_bootguard" || body.LayeringBootguard != "some-release-digest" {
		t.Fatalf("bootguarded /health body status=%q key=%q", body.Status, body.LayeringBootguard)
	}
}
