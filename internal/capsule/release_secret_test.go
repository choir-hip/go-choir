//go:build linux

package capsule

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStageGrantedReleaseRefusesSecrets(t *testing.T) {
	for name, relative := range map[string]struct {
		path    string
		content string
	}{
		"secret path":    {path: ".env.production", content: "ordinary"},
		"secret content": {path: "config.txt", content: "api_key=abcdefghijklmnop"},
	} {
		t.Run(name, func(t *testing.T) {
			merged := t.TempDir()
			upper := t.TempDir()
			for _, root := range []string{merged, upper} {
				if err := os.MkdirAll(filepath.Join(root, "var/lib/artifact/release/bin"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "var/lib/artifact/release/bin/sandbox"), []byte("sandbox"), 0o755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "var/lib/artifact/release", filepath.FromSlash(relative.path))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(relative.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			capability := &Capability{CapabilityID: "cap-1", Handle: "grant-1", CapsuleID: "capsule-1", TargetCapsule: "capsule-1", AgentRunID: "cosuper-1", AgentRole: RoleCoSuper, Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour)}
			if err := SignCapability(capability, privateKey, "test-key"); err != nil {
				t.Fatal(err)
			}
			executor := &Executor{
				capsules:     map[string]*Capsule{"capsule-1": {ID: "capsule-1", UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20}},
				capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-1", Handle: "grant-1"}: capability},
				revokedCaps:  map[string]bool{}, publicKey: publicKey,
			}
			_, _, err = executor.StageGrantedRelease("cosuper-1", "grant-1", t.TempDir())
			if err == nil || !strings.Contains(err.Error(), "refuses secret") {
				t.Fatalf("secret release error = %v", err)
			}
		})
	}
}

func TestStageGrantedReleaseStagesRelativeUpperdirPaths(t *testing.T) {
	merged := t.TempDir()
	upper := t.TempDir()
	for _, root := range []string{merged, upper} {
		path := filepath.Join(root, "var/lib/artifact/release/bin/sandbox")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("sandbox"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-success", Handle: "grant-success", CapsuleID: "capsule-success",
		TargetCapsule: "capsule-success", AgentRunID: "cosuper-success", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-success": {
			ID: "capsule-success", UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20,
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-success", Handle: "grant-success"}: capability},
		revokedCaps: map[string]bool{}, publicKey: publicKey,
	}
	files, staged, err := executor.StageGrantedRelease("cosuper-success", "grant-success", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "bin/sandbox" || staged == "" {
		t.Fatalf("staged release files=%+v path=%q", files, staged)
	}
	if content, err := os.ReadFile(filepath.Join(staged, "bin/sandbox")); err != nil || string(content) != "sandbox" {
		t.Fatalf("staged sandbox = %q, %v", content, err)
	}
}
