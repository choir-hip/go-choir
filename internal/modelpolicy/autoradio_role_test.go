package modelpolicy

// The autoradio role (owner 2026-11-11: voice prompts go to the Autoradio desk
// on a fast model; then "River DeepSeek rather than Bedrock Haiku"). Ways it
// could fail, written before the code:
//
//  1. a policy file with [roles.autoradio] is refused as an unknown section
//     (the role is not a registry profile, and the package desk's own name
//     app:autoradio cannot head a section), so an owner cannot override it;
//  2. a computer whose policy file predates the role (no section) resolves it
//     to the default pool, so a voice turn runs the slow free members first;
//  3. the chain has devin, beam or Synthetic in it (owner 2026-11-11), or the
//     continuation chain behind it, so a refused voice turn walks slow models;
//  4. a computer's policy section does not override the compiled pool;
//  5. the chain's order is not River then Bedrock Haiku.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func autoradioManager(t *testing.T, policy string) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewManager(ManagerConfig{PolicyPath: path})
}

func chainOf(m *Manager, role string) []string {
	var out []string
	for _, s := range m.PoolChain(context.Background(), "owner", role) {
		out = append(out, s.Provider+"/"+s.Model)
	}
	return out
}

func TestAutoradioRoleNormalizesAsABareRole(t *testing.T) {
	if got := NormalizeRole(" Autoradio "); got != AutoradioRole {
		t.Fatalf("NormalizeRole(autoradio) = %q", got)
	}
}

func TestAutoradioRoleResolvesToItsPoolWithNoPolicySection(t *testing.T) {
	// (2) a current policy file with no autoradio section.
	m := autoradioManager(t, defaultPolicyText(provideriface.Config{}))
	got := chainOf(m, AutoradioRole)
	want := []string{memRiver, "bedrock/us.anthropic.claude-haiku-5-5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("(2)(3)(5) chain = %v, want %v", got, want)
	}
	for _, g := range got {
		if strings.HasPrefix(g, "devin/") || strings.HasPrefix(g, "opencode-go/") || strings.HasPrefix(g, "synthetic/") {
			t.Fatalf("(3) a member the owner ruled out is in the chain: %v", got)
		}
	}
}

func TestAutoradioRoleSectionIsAcceptedAndOverrides(t *testing.T) {
	// (1)(4) an owner-edited file naming the role one model.
	m := autoradioManager(t, "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.autoradio]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\nreasoning = \"low\"\n")
	if got := chainOf(m, AutoradioRole); strings.Join(got, ",") != memSynthetic {
		t.Fatalf("(4) chain = %v", got)
	}
	pol, _ := m.Load(context.Background(), "owner")
	if len(pol.Diagnostics) != 0 {
		t.Fatalf("(1) the section drew diagnostics: %v", pol.Diagnostics)
	}
	// And a pool for the role.
	m = autoradioManager(t, "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.autoradio]\nmodel = \"pool:synthetic-luna\"\nreasoning = \"low\"\n")
	if got := chainOf(m, AutoradioRole); len(got) != 2 || got[0] != memSynthetic {
		t.Fatalf("(4) pool override chain = %v", got)
	}
}
