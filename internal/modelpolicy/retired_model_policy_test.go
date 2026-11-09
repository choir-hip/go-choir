package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

// An owner policy file that pins a retired model (d@e's names
// openrouter/stealth/crouton-alpha for the texture role, retired by OpenRouter
// 2026-11-09) neither crashes the computer nor strands the role: the file
// parses, the role runs on the default pool with its fallbacks, and the run
// says so in llm_policy_error
// (docs/problems/crouton-retired-inkling-gated-2026-11-09.md).
func TestPolicyPinningRetiredCroutonDegradesToDefaultPoolVisibly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	text := "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nprovider = \"openrouter\"\nmodel = \"stealth/crouton-alpha\"\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := NewManager(ManagerConfig{PolicyPath: path}).EnrichMetadata(context.Background(), "owner", agentprofile.Texture, map[string]any{"trajectory_id": "t1"})
	if meta[MetadataAssignment] != "retired_remapped" || meta[MetadataModel] == "" || meta[MetadataModel] == "stealth/crouton-alpha" {
		t.Fatalf("metadata = %+v, want a default-pool member", meta)
	}
	if f, _ := meta[MetadataFallbacks].(string); f == "" {
		t.Fatalf("no fallbacks: %+v", meta)
	}
	note, _ := meta[MetadataPolicyError].(string)
	if !strings.Contains(note, "stealth/crouton-alpha") || !strings.Contains(note, "default pool") {
		t.Fatalf("no visible note: %q", note)
	}
}
