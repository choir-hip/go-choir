//go:build linux

package capsule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The executor half of the unfinished snapshot (unfinished_patch_test.go
// lists the ways the derivation can fail). Here:
//
//  U9.  The tree is read while the capsule can still change it (it must be
//       quiesced, its cgroup frozen, first).
//  U10. A capsule that is gone or being destroyed panics or yields a patch.

// U9, U10.
func TestSnapshotUnfinishedSourcePatchFreezesThenReadsTheCapsule(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	merged, upper := filepath.Join(root, "merged"), filepath.Join(root, "upper")
	writeTree(t, filepath.Join(root, "source-lower"), map[string]string{"workspace/platform/a.go": "package a\n"})
	writeTree(t, merged, map[string]string{"workspace/platform/a.go": "package a\n// unfinished\n"})
	writeTree(t, upper, map[string]string{"workspace/platform/a.go": "package a\n// unfinished\n"})
	cgroup := &testCapsuleCgroup{}
	caps := &Capsule{ID: "capsule-unfinished", State: StateActive, UpperDir: upper, MergedDir: merged, Cgroup: cgroup}
	executor := &Executor{capsules: map[string]*Capsule{caps.ID: caps}, revokedCaps: map[string]bool{}}

	got, err := executor.SnapshotUnfinishedSourcePatch(context.Background(), caps.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cgroup.frozen || caps.State != StateFrozen {
		t.Fatalf("the tree was read from a capsule that was not frozen: cgroup frozen %v, state %s", cgroup.frozen, caps.State)
	}
	if strings.Join(got.Files, ",") != "a.go" || !strings.Contains(string(got.Patch), "+// unfinished") {
		t.Fatalf("patch = %v\n%s", got.Files, got.Patch)
	}
	again, err := executor.SnapshotUnfinishedSourcePatch(context.Background(), caps.ID)
	if err != nil || again.SHA256 != got.SHA256 {
		t.Fatalf("a retry on the frozen capsule gave %s (%v), want %s", again.SHA256, err, got.SHA256)
	}

	if _, err := executor.SnapshotUnfinishedSourcePatch(context.Background(), "capsule-gone"); !errors.Is(err, ErrCapsuleNotSnapshottable) {
		t.Fatalf("a gone capsule gave %v", err)
	}
	caps.State = StateDestroying
	if _, err := executor.SnapshotUnfinishedSourcePatch(context.Background(), caps.ID); !errors.Is(err, ErrCapsuleNotSnapshottable) {
		t.Fatalf("a destroying capsule gave %v", err)
	}
	_ = os.RemoveAll(root)
}
