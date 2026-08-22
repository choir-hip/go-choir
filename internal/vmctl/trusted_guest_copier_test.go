package vmctl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrustedGuestCopierCopiesPlainKey(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-test-123"
	vmDir := filepath.Join(root, vmID)
	if err := os.MkdirAll(vmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	computerID := strings.Repeat("c", 64)
	// Create quarantine plain JSON file.
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i)
	}
	keyFile := map[string]any{"version": 1, "computer_id": computerID, "key": base64.RawStdEncoding.EncodeToString(keyBytes)}
	raw, _ := json.Marshal(keyFile)
	quarantine := filepath.Join(vmDir, "data.img.quarantine-1-abc123")
	staging := filepath.Join(vmDir, "data.img.staging-1-abc123")
	if err := os.WriteFile(quarantine, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staging, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	token := RecoveryFencingToken{
		Audience: "vmctl", Operation: "recover_current",
		ComputerID: computerID, OwnerID: "owner-1", VMID: vmID,
		RouteGeneration: 7, CanonicalHead: strings.Repeat("a", 64),
		RecoveryGeneration: 1, Nonce: "nonce-1", Expiry: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano),
	}
	copier := TrustedGuestCopier{StateRoot: root}
	if err := copier.CopyPrivacyKey(context.Background(), token, quarantine, staging); err != nil {
		t.Fatalf("CopyPrivacyKey plain failed: %v", err)
	}
	// Verify staging now contains same key.
	data, err := os.ReadFile(staging)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	_ = json.Unmarshal(data, &got)
	_ = json.Unmarshal(raw, &want)
	if got["computer_id"] != want["computer_id"] || got["key"] != want["key"] {
		t.Fatalf("staging key mismatch: got %#v want %#v", got, want)
	}
	info, _ := os.Stat(staging)
	if info.Mode().Perm() != 0o400 {
		t.Fatalf("staging mode=%o want 0400", info.Mode().Perm())
	}
}

func TestTrustedGuestCopierRejectsCrossVM(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-test-123"
	vmDir := filepath.Join(root, vmID)
	os.MkdirAll(vmDir, 0o755)
	quarantine := filepath.Join(vmDir, "data.img.quarantine-1-abc123")
	staging := filepath.Join(vmDir, "data.img.staging-1-abc123")
	os.WriteFile(quarantine, []byte(`{"version":1,"computer_id":"`+strings.Repeat("c", 64)+`","key":"`+base64.RawStdEncoding.EncodeToString(make([]byte, 32))+`"}`), 0o644)
	os.WriteFile(staging, []byte("{}"), 0o644)
	token := RecoveryFencingToken{
		Audience: "vmctl", Operation: "recover_current",
		ComputerID: strings.Repeat("c", 64), OwnerID: "owner-1", VMID: "other-vm",
		RouteGeneration: 1, CanonicalHead: strings.Repeat("a", 64),
		RecoveryGeneration: 1, Nonce: "n", Expiry: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano),
	}
	copier := TrustedGuestCopier{StateRoot: root}
	if err := copier.CopyPrivacyKey(context.Background(), token, quarantine, staging); err == nil {
		t.Fatal("cross-VM copy was allowed")
	}
}
