//go:build linux

package capsule

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// R9-1 (docs/problems/freeze-replays-a-changed-tree-2026-10-26.md): a
// second Freeze reads the live tree to compare it with the frozen candidate.
// The read must give the digest the receipts and the frozen subject use, the
// capsule's diff, and change no state: an active capsule stays active, one
// the terminal saga froze stays frozen.
func TestInspectGrantedWorktreeReadsTheLiveTreeAndChangesNothing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	merged := filepath.Join(root, "merged")
	upper := filepath.Join(root, "upper")
	for _, dir := range []string{filepath.Join(merged, "workspace", "platform"), filepath.Join(upper, "workspace", "platform")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "App.svelte"), []byte("after\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	caps := &Capsule{ID: "capsule-inspect", State: StateActive, MergedDir: merged, UpperDir: upper}
	capability := &Capability{
		CapabilityID: "cap-inspect", Handle: "handle-inspect", AgentRunID: "run-inspect",
		AgentRole: RoleEngineering, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour),
	}
	executor := &Executor{
		stateDir:     t.TempDir(),
		capsules:     map[string]*Capsule{caps.ID: caps},
		capabilities: map[capKey]*Capability{{AgentRunID: "run-inspect", Handle: "handle-inspect"}: capability},
		revokedCaps:  map[string]bool{},
	}
	want, err := digestCapsuleWorktree(ctx, caps)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []CapsuleState{StateActive, StateFrozen} {
		caps.State = state
		live, err := executor.InspectGrantedWorktree(ctx, "run-inspect", "handle-inspect")
		if err != nil {
			t.Fatalf("%s: inspect: %v", state, err)
		}
		if live.Digest != want {
			t.Fatalf("%s: inspect digest %s, want the worktree digest %s", state, live.Digest, want)
		}
		found := false
		for _, change := range live.Changes {
			if change.Path == "workspace/platform/App.svelte" && change.Kind == ChangeAdded {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: inspect diff %+v does not name the upper's App.svelte", state, live.Changes)
		}
		if caps.State != state {
			t.Fatalf("inspect moved the capsule from %s to %s", state, caps.State)
		}
	}

	// The read follows the tree: an edit moves the digest.
	if err := os.WriteFile(filepath.Join(merged, "workspace", "platform", "App.svelte"), []byte("edited again\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if live, err := executor.InspectGrantedWorktree(ctx, "run-inspect", "handle-inspect"); err != nil || live.Digest == want {
		t.Fatalf("inspect after an edit = %s, %v; want a new digest", live.Digest, err)
	}
	if _, err := executor.InspectGrantedWorktree(ctx, "run-other", "handle-inspect"); err == nil {
		t.Fatal("inspect under another run's capability passed")
	}
	caps.State = StateDestroyed
	if _, err := executor.InspectGrantedWorktree(ctx, "run-inspect", "handle-inspect"); err == nil {
		t.Fatal("inspect of a destroyed capsule passed")
	}
}
