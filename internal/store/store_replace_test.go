package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestReplaceWorkspaceRefusesEmptyQuarantine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	live, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	if _, _, err := live.ReplaceWorkspace(""); !errors.Is(err, ErrWorkspaceReplaceRequiresQuarantine) {
		t.Fatalf("ReplaceWorkspace error=%v, want ErrWorkspaceReplaceRequiresQuarantine", err)
	}
}

func TestReplaceWorkspaceDropsRetiredResidueAndUnsupportedRows(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "runtime.db")
	live, err := Open(path)
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

	quarantine := filepath.Join(root, "quarantine")
	fresh, receipt, err := live.ReplaceWorkspace(quarantine)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fresh.Close() })
	if receipt.QuarantineDir != quarantine {
		t.Fatalf("quarantine dir=%q, want %q", receipt.QuarantineDir, quarantine)
	}
	if _, err := os.Stat(receipt.QuarantinedMarker); err != nil {
		t.Fatalf("quarantined marker missing: %v", err)
	}
	if _, err := os.Stat(receipt.QuarantinedWorkspace); err != nil {
		t.Fatalf("quarantined workspace missing: %v", err)
	}

	var retired int
	if err := fresh.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'app_adoptions'`).Scan(&retired); err != nil {
		t.Fatalf("query retired table: %v", err)
	}
	if retired != 0 {
		t.Fatalf("retired table survived replacement: count=%d", retired)
	}
	var extraColumn int
	if err := fresh.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'computer_event_index' AND column_name = 'supervision_transaction_json'`).Scan(&extraColumn); err != nil {
		t.Fatalf("query extra column: %v", err)
	}
	if extraColumn != 0 {
		t.Fatalf("retired column survived replacement: count=%d", extraColumn)
	}
	var prefs int
	if err := fresh.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_preferences`).Scan(&prefs); err != nil {
		t.Fatalf("query preferences: %v", err)
	}
	if prefs != 0 {
		t.Fatalf("unsupported rows survived replacement: count=%d", prefs)
	}
}
