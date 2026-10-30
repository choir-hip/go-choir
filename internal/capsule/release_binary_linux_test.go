//go:build linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Freeze stages the running binary for a change with no binary input, and
// refuses to for a Go change or a source pinned to another commit
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md,
// A1-A4, A8). Ways it could fail: the reused file is not the running bytes;
// the provenance is missing or names the wrong origin; a desk-built binary is
// replaced; a Go change ships the running binary; the error does not say the
// desk must build.
func TestStageGrantedReleaseReusesRunningBinaryForANonGoChange(t *testing.T) {
	runningPayload := []byte("\x7fELF running autoputer")
	sum := sha256.Sum256(runningPayload)
	runningSHA := hex.EncodeToString(sum[:])
	runningPath := filepath.Join(t.TempDir(), "autoputer")
	if err := os.WriteFile(runningPath, runningPayload, 0o755); err != nil {
		t.Fatal(err)
	}
	const commit = "0123456789abcdef0123456789abcdef01234567"
	running := func() (RunningBinary, error) {
		return RunningBinary{Path: runningPath, SHA256: runningSHA, Commit: commit}, nil
	}

	type layout struct {
		sourceFiles map[string]string // workspace/platform-relative
		builtBinary bool
		sourceRef   string
	}
	stage := func(t *testing.T, l layout) (*StagedRelease, error) {
		t.Helper()
		merged := filepath.Join(t.TempDir(), "merged")
		upper := t.TempDir()
		write := func(rel, content string, mode os.FileMode) {
			for _, root := range []string{merged, upper} {
				path := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), mode); err != nil {
					t.Fatal(err)
				}
			}
		}
		write("var/lib/artifact/release/frontend/index.html", "<html>computer</html>", 0o644)
		if l.builtBinary {
			write("var/lib/artifact/release/bin/autoputer", "capsule build", 0o755)
		}
		for rel, content := range l.sourceFiles {
			write("workspace/platform/"+rel, content, 0o644)
		}
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		capability := &Capability{
			CapabilityID: "cap-reuse", Handle: "grant-reuse", CapsuleID: "capsule-reuse",
			TargetCapsule: "capsule-reuse", AgentRunID: "run-reuse", AgentRole: RoleEngineering,
			Verbs: RoleVerbSets[RoleEngineering], ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := SignCapability(capability, privateKey, "test-key"); err != nil {
			t.Fatal(err)
		}
		executor := &Executor{
			capsules: map[string]*Capsule{"capsule-reuse": {
				ID: "capsule-reuse", State: StateFrozen, UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20,
				Spec: SpawnSpec{SourceArtifactRef: l.sourceRef},
			}},
			capabilities:  map[capKey]*Capability{{AgentRunID: "run-reuse", Handle: "grant-reuse"}: capability},
			revokedCaps:   map[string]bool{},
			publicKey:     publicKey,
			runningBinary: running,
		}
		incoming := t.TempDir()
		if err := os.Chmod(incoming, 0o700); err != nil {
			t.Fatal(err)
		}
		staged, err := executor.StageGrantedRelease(context.Background(), "run-reuse", "grant-reuse", incoming)
		if staged != nil {
			t.Cleanup(func() { _ = os.RemoveAll(staged.TemporaryRoot) })
		}
		return staged, err
	}
	pinned := "capsule-source-git:" + commit + ":sha256:" + strings.Repeat("d", 64)
	frontendChange := map[string]string{"frontend/public/choir-drill/x.txt": "token"}

	staged, err := stage(t, layout{sourceFiles: frontendChange, sourceRef: pinned})
	if err != nil {
		t.Fatalf("frontend-only freeze refused reuse: %v", err)
	}
	if len(staged.Files) != 2 || staged.Files[0].Path != "bin/autoputer" || staged.Files[0].SHA256 != runningSHA || staged.Files[0].Mode != 0o555 {
		t.Fatalf("staged files = %+v, want the running binary first", staged.Files)
	}
	if b := staged.Binary; b == nil || b.Origin != ReleaseBinaryReusedRunning || b.SHA256 != runningSHA || b.RunningCommit != commit || b.SourceBaseCommit != commit {
		t.Fatalf("provenance = %+v", staged.Binary)
	}
	if content, err := os.ReadFile(filepath.Join(staged.TemporaryRoot, "bin/autoputer")); err != nil || string(content) != string(runningPayload) {
		t.Fatalf("staged binary = %q, %v", content, err)
	}

	staged, err = stage(t, layout{sourceFiles: frontendChange, sourceRef: pinned, builtBinary: true})
	if err != nil {
		t.Fatalf("desk-built release refused: %v", err)
	}
	if b := staged.Binary; b == nil || b.Origin != ReleaseBinaryCapsuleBuilt || b.SHA256 == runningSHA {
		t.Fatalf("desk-built binary replaced or unrecorded: %+v", staged.Binary)
	}

	_, err = stage(t, layout{sourceFiles: map[string]string{"internal/agentcore/x.go": "package agentcore"}, sourceRef: pinned})
	if err == nil || !strings.Contains(err.Error(), "must contain executable bin/autoputer") || !strings.Contains(err.Error(), "internal/agentcore/x.go") {
		t.Fatalf("Go change shipped the running binary or did not name why: %v", err)
	}

	other := "capsule-source-git:" + strings.Repeat("b", 40) + ":sha256:" + strings.Repeat("d", 64)
	_, err = stage(t, layout{sourceFiles: frontendChange, sourceRef: other})
	if err == nil || !strings.Contains(err.Error(), "pinned to "+strings.Repeat("b", 40)) {
		t.Fatalf("a source at another commit shipped the running binary: %v", err)
	}
}
