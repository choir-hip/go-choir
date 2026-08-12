package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestReplaceWorkspaceDropsResidueAndClosesStore(t *testing.T) {
	computerID := "computer-workspace-replace"
	root := t.TempDir()
	storePath := filepath.Join(root, "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := live.DB().ExecContext(ctx, `CREATE TABLE app_adoptions (adoption_id VARCHAR(255) PRIMARY KEY)`); err != nil {
		t.Fatalf("create retired table: %v", err)
	}
	if _, err := live.DB().ExecContext(ctx, `ALTER TABLE computer_event_index ADD COLUMN supervision_transaction_json LONGTEXT NOT NULL DEFAULT ''`); err != nil {
		t.Fatalf("add retired column: %v", err)
	}
	if _, err := live.SaveUserPreference(ctx, types.UserPreference{
		OwnerID:       "owner-1",
		PreferenceKey: "theme",
		Value:         map[string]any{"mode": "dark"},
	}); err != nil {
		t.Fatalf("seed preference: %v", err)
	}

	rt := &Runtime{
		cfg:   provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store: live,
	}
	report, err := rt.ReplaceWorkspace(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if rt.store != nil {
		t.Fatal("runtime kept a store handle after replace")
	}
	if !report.StoreClosed || report.AppendedEvent || report.PublishedCheckpoint {
		t.Fatalf("report flags = %#v", report)
	}
	if _, err := os.Stat(report.Receipt.QuarantinedMarker); err != nil {
		t.Fatalf("quarantined marker missing: %v", err)
	}

	reopened, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var retired int
	if err := reopened.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'app_adoptions'`).Scan(&retired); err != nil {
		t.Fatalf("query retired table: %v", err)
	}
	if retired != 0 {
		t.Fatalf("retired table survived replacement: count=%d", retired)
	}
	var extraColumn int
	if err := reopened.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'computer_event_index' AND column_name = 'supervision_transaction_json'`).Scan(&extraColumn); err != nil {
		t.Fatalf("query extra column: %v", err)
	}
	if extraColumn != 0 {
		t.Fatalf("retired column survived replacement: count=%d", extraColumn)
	}
	var prefs int
	if err := reopened.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_preferences`).Scan(&prefs); err != nil {
		t.Fatalf("query preferences: %v", err)
	}
	if prefs != 0 {
		t.Fatalf("unsupported rows survived replacement: count=%d", prefs)
	}
}

func TestReplaceWorkspaceProductPathDoesNotUseGenesis(t *testing.T) {
	computerID := "computer-workspace-replace-api"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		cfg:   provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store: live,
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/replace-workspace", nil)
	request.Header.Set("X-Authenticated-User", "owner-workspace-replace")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("replace-workspace status=%d body=%s", response.Code, response.Body.String())
	}
	var report WorkspaceReplaceReport
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ComputerID != computerID || report.AppendedEvent || report.PublishedCheckpoint || !report.StoreClosed {
		t.Fatalf("report = %#v", report)
	}
	if rt.store != nil {
		t.Fatal("API replace left the runtime store open")
	}
}

func TestReplaceWorkspaceRejectsNonPost(t *testing.T) {
	computerID := "computer-workspace-replace-get"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	rt := &Runtime{
		cfg:   provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store: live,
	}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/lifecycle/replace-workspace", nil)
	request.Header.Set("X-Authenticated-User", "owner-workspace-replace")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET replace-workspace status=%d body=%s", response.Code, response.Body.String())
	}
	if rt.store == nil {
		t.Fatal("GET replace-workspace closed the store")
	}
}
