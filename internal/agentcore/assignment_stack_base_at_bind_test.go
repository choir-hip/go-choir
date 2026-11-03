package agentcore

// Problem 30 (docs/problems/selfdev-bind-reads-a-stack-head-on-the-old-base-2026-11-03.md):
// pool's assignment-19dc1256 bound at 21:45:07Z, during the boot of a
// planned refresh onto 3c0af9920, on HEAD 35b5cba: the projection commit of
// the applied stack on 70160304a. The booted base's objects were fetched at
// 21:45:22Z and the stack was grafted onto it then, but the capsule was
// already pinned to the stack on the old base. Freeze composed the
// cumulative patch from that commit's stack base, 70160304a, and the updater
// refused the apply (base_moved_since_freeze) after 40 minutes of work.
//
// Ways the fix can fail, written first:
//   G1 the bind reads HEAD while the source projection says the stack is not
//      on the booted base (objects missing, stack_base_moved, apply pending,
//      unrebuildable) and binds a capsule whose Freeze base is not booted;
//   G2 a transient state (the booted base not fetched yet, an apply in
//      flight) cancels the request instead of letting it wait and retry;
//   G3 a state that will not pass by waiting (the stack's patch conflicts
//      with the booted base, the fetch gave up) leaves the request waiting
//      until its deadline instead of telling management at bind;
//   G4 once the projection settles (the graft), the retry binds on the
//      graft and Freeze records the booted base, the base the updater wants;
//   G5 the wait spawns a capsule, binds, or holds the saga mutex.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// newStackedBindFixture is pool before the refresh: a self-development
// stack (change) applied on the booted base X, HEAD its projection commit,
// and an ordinary request whose Cast read that HEAD.
func newStackedBindFixture(t *testing.T, change map[string]string) *baseAtBindFixture {
	t.Helper()
	return newBaseAtBindFixtureWith(t, "", func(f *baseAtBindFixture) {
		operations, err := selfdev.NewStore(f.s, f.s)
		if err != nil {
			t.Fatal(err)
		}
		f.rt.selfdevOperations = operations
		f.rt.selfdevUpdaterRoot = t.TempDir()
		patch, _ := releasePatch(t, f.repo, change)
		applyOperation(t, f.rt, f.rt.selfdevUpdaterRoot, f.computer, "op-stack", f.x, patch)
	})
}

// commitElsewhere makes a commit on base in a clone of repo; repo does not
// get its objects until fetchFrom. It is a platform build the guest has not
// fetched yet.
func commitElsewhere(t *testing.T, repo, base string, files map[string]string) (commit, remote string) {
	t.Helper()
	remote = t.TempDir()
	releaseGit(t, remote, "clone", "-q", "--no-checkout", repo, ".")
	releaseGit(t, remote, "checkout", "-q", "-b", "platform", base)
	commit = commitFiles(t, remote, files)
	if sourceproj.HasCommit(context.Background(), repo, commit) {
		t.Fatal("fixture: the guest repository already has the new base")
	}
	return commit, remote
}

func commitFiles(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, dir, "add", "-A")
	releaseGit(t, dir, "commit", "-q", "-m", "platform")
	return releaseGit(t, dir, "rev-parse", "HEAD")
}

// freezeAndApply is what the bound capsule's round does to the base: the
// engineer changes b.txt on the pinned source, Freeze composes the
// cumulative patch from the pinned commit's stack base (composeFreezePatch),
// and the updater requires that base to be the booted base commit
// (updater.checkStateCompatibility, base_moved_since_freeze).
func (f *baseAtBindFixture) freezeAndApply(t *testing.T, sourceRef, booted string) (freezeBase string, applyErr string) {
	t.Helper()
	pinned := capsule.SourceArtifactCommit(sourceRef)
	patch := stagedPatchFrom(t, f.repo, pinned, map[string]string{"frontend/public/b.txt": "B\n"})
	staged := stageFreeze(t, patch, pinned)
	if err := composeFreezePatch(context.Background(), f.repo, staged); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if !strings.EqualFold(staged.SourceBaseCommit, booted) {
		return staged.SourceBaseCommit, "base_moved_since_freeze: release base commit " + staged.SourceBaseCommit + " does not match booted base commit " + booted
	}
	return staged.SourceBaseCommit, ""
}

func (f *baseAtBindFixture) retryWakes() int {
	n := 0
	for _, wake := range *f.wakes {
		var content delegatedAssignmentSpawnDeadline
		if wake.kind == delegatedAssignmentSpawnDeadlineUpdateKind && json.Unmarshal([]byte(wake.content), &content) == nil && content.Retry > 0 {
			n++
		}
	}
	return n
}

// G1, G2, G4, G5: the incident's order. Stack on X, planned refresh to Y
// whose objects the guest has not fetched yet, bind, Freeze, apply.
func TestStackedRequestBoundBeforeTheBootedBaseIsFetchedWaitsThenFreezesOnIt(t *testing.T) {
	f := newStackedBindFixture(t, map[string]string{"frontend/public/a.txt": "A\n"})
	if base, ok := sourceproj.StackBase(context.Background(), f.repo, gitHead(t, f.repo)); !ok || base != f.x {
		t.Fatalf("fixture: HEAD is not the stack's projection commit on X (%s %t)", base, ok)
	}
	y, remote := commitElsewhere(t, f.repo, f.x, map[string]string{"frontend/public/next.txt": "the refreshed platform\n"})
	f.refreshTo(y)
	if err := f.wake(); err != nil {
		t.Fatalf("bind during the boot: %v", err)
	}
	got := f.get(0)
	if got.Disposition == types.EngineeringAssignmentBound {
		base, applyErr := f.freezeAndApply(t, got.Binding.SourceArtifactRef, y)
		t.Fatalf("G1: the request bound on %s while the booted base %s was not fetched; Freeze records base %s and the apply is refused: %s",
			got.Binding.SourceArtifactRef, y, base, applyErr)
	}
	if got.Disposition != types.EngineeringAssignmentOpen || got.CapsuleDisposition != types.EngineeringCapsuleUnbound || len(f.capsule.spawned) != 0 {
		t.Fatalf("G2/G5: the request must wait open and unbound with no capsule: %s %s (%q), spawned %d", got.Disposition, got.CapsuleDisposition, got.DispositionReason, len(f.capsule.spawned))
	}
	if f.retryWakes() != 1 {
		t.Fatalf("G2: the waiting request must arm one admission retry, got %d", f.retryWakes())
	}

	// The base fetch lands; the projection grafts the stack onto Y.
	releaseGit(t, f.repo, "fetch", "-q", remote, "platform:refs/platform/fetched")
	if err := f.wake(); err != nil {
		t.Fatalf("bind after the fetch: %v", err)
	}
	bound := f.get(0)
	if bound.Disposition != types.EngineeringAssignmentBound {
		t.Fatalf("G4: not bound once the booted base is here: %s (%q)", bound.Disposition, bound.DispositionReason)
	}
	if len(f.capsule.spawned) != 1 || f.capsule.spawned[0].SourceArtifactRef != bound.Binding.SourceArtifactRef {
		t.Fatalf("G4: the capsule and the binding disagree: %d spawned", len(f.capsule.spawned))
	}
	base, applyErr := f.freezeAndApply(t, bound.Binding.SourceArtifactRef, y)
	if applyErr != "" {
		t.Fatalf("G4: Freeze records base %s; the apply would be refused: %s", base, applyErr)
	}
}

// G3: a stack whose patch touches a path the new base changed cannot be
// grafted (stack_base_moved); waiting will not help, so the bind tells
// management now, not the updater forty minutes later.
func TestStackedRequestOnAMovedBaseIsRefusedAtBind(t *testing.T) {
	f := newStackedBindFixture(t, stackChange())
	moved := commitOnTop(t, f.repo, f.x, movedBaseChange())
	f.refreshTo(moved)
	// The saga's error is the cancel's cause, as for any open that fails
	// before it binds; a redelivered wake finds the request terminal.
	if err := f.wake(); err != nil {
		t.Logf("bind: %v", err)
	}
	got := f.get(0)
	if got.Disposition == types.EngineeringAssignmentBound {
		base, applyErr := f.freezeAndApply(t, got.Binding.SourceArtifactRef, moved)
		t.Fatalf("G1: bound on a stack the booted base moved under; Freeze records base %s: %s", base, applyErr)
	}
	if !got.Disposition.Terminal() || !strings.Contains(got.DispositionReason, "stack_base_moved") || len(f.capsule.spawned) != 0 {
		t.Fatalf("G3: the request must end at bind with the projection's reason: %s (%q), spawned %d", got.Disposition, got.DispositionReason, len(f.capsule.spawned))
	}
}

// G3: the booted base's objects never arrive (the fetch gave up), so the
// request ends at bind with the projection's reason, which names the missing
// base and the fetch's outcome.
func TestStackedRequestWhoseBaseFetchGaveUpIsRefusedAtBind(t *testing.T) {
	f := newStackedBindFixture(t, map[string]string{"frontend/public/a.txt": "A\n"})
	y, _ := commitElsewhere(t, f.repo, f.x, map[string]string{"frontend/public/next.txt": "the refreshed platform\n"})
	f.refreshTo(y)
	f.rt.recordSourceBaseFetch(sourceproj.FetchResult{Outcome: sourceproj.FetchPermanent, Commit: y, Reason: "the remote does not have the commit"}, 0)
	// The saga's error is the cancel's cause, as for any open that fails
	// before it binds; a redelivered wake finds the request terminal.
	if err := f.wake(); err != nil {
		t.Logf("bind: %v", err)
	}
	got := f.get(0)
	if !got.Disposition.Terminal() || !strings.Contains(got.DispositionReason, y) || !strings.Contains(got.DispositionReason, "failed_permanent") || len(f.capsule.spawned) != 0 {
		t.Fatalf("G3: the request must end at bind naming the missing base: %s (%q), spawned %d", got.Disposition, got.DispositionReason, len(f.capsule.spawned))
	}
}

// G2, G3: the classification, including the race where the fetch lands
// between the bind's reconcile and the classification (the result still says
// the base is missing; the repository has it now): that retries, never ends
// the request.
func TestSourceProjectionSettlesClassifiesByWhyTheStackIsOffTheBase(t *testing.T) {
	f := newStackedBindFixture(t, map[string]string{"frontend/public/a.txt": "A\n"})
	ctx := context.Background()
	missing := "the stack is based on " + f.x + " but this computer booted " + f.y + "; that commit is not in the guest source repository, so the stack cannot be shown to be upstreamed"
	cases := []struct {
		name   string
		result sourceproj.Result
		want   bool
	}{
		{"apply in flight", sourceproj.Result{State: sourceproj.StatePending}, true},
		{"fetch landed after the reconcile", sourceproj.Result{State: sourceproj.StateBaseMoved, BootedBase: f.y, BootedMissing: true, Reason: missing}, true},
		{"empty stack, fetch landed after the reconcile", sourceproj.Result{State: sourceproj.StateBaseObjectsMissing, BootedBase: f.y, BootedMissing: true}, true},
		{"graft refused on a present base", sourceproj.Result{State: sourceproj.StateBaseMoved, BootedBase: f.y, Reason: "the booted base also changed frontend/public/long.txt"}, false},
		{"unrebuildable", sourceproj.Result{State: sourceproj.StateUnrebuildable, BootedBase: f.y}, false},
		{"repository unavailable", sourceproj.Result{State: sourceproj.StateRepoUnavailable}, false},
	}
	for _, tc := range cases {
		if got := f.rt.sourceProjectionSettles(ctx, tc.result); got != tc.want {
			t.Errorf("%s: settles = %t, want %t", tc.name, got, tc.want)
		}
	}
	absent, _ := commitElsewhere(t, f.repo, f.x, map[string]string{"frontend/public/c.txt": "C\n"})
	off := sourceproj.Result{State: sourceproj.StateBaseObjectsMissing, BootedBase: absent, BootedMissing: true}
	if !f.rt.sourceProjectionSettles(ctx, off) {
		t.Error("a missing base the boot's fetch has not reported on must wait")
	}
	f.rt.recordSourceBaseFetch(sourceproj.FetchResult{Outcome: sourceproj.FetchFailed, Commit: absent, Reason: "network"}, 0)
	if f.rt.sourceProjectionSettles(ctx, off) {
		t.Error("a missing base whose fetch has stopped trying must not wait")
	}
	// Review r2: the fetch lands and records fetched between the
	// classifier's repository look and its read of the fetch state.
	f.rt.recordSourceBaseFetch(sourceproj.FetchResult{Outcome: sourceproj.FetchFetched, Commit: absent}, 1)
	if !f.rt.sourceProjectionSettles(ctx, off) {
		t.Error("a missing base whose fetch recorded fetched must wait for the retry's reconcile")
	}
	f.rt.recordSourceBaseFetch(sourceproj.FetchResult{Outcome: sourceproj.FetchPresent, Commit: absent}, 2)
	if !f.rt.sourceProjectionSettles(ctx, off) {
		t.Error("a missing base whose fetch found it present must wait for the retry's reconcile")
	}
	f.rt.recordSourceBaseFetch(sourceproj.FetchResult{Outcome: sourceproj.FetchFailed, Commit: absent, Reason: "network"}, 3)
	f.rt.sourceBaseFetchMu.Lock()
	f.rt.sourceBaseFetchRunning = true
	f.rt.sourceBaseFetchMu.Unlock()
	if !f.rt.sourceProjectionSettles(ctx, off) {
		t.Error("a missing base whose fetch is between attempts must wait")
	}
}
