package agentcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// U1b wiring (docs/design/computer-update-architecture-rnd-2026-10-24.md,
// "U1b design"): N1, N2, N9, N13 and the cast gate's bounded wait. The git
// behavior itself is pinned in internal/sourceproj.

// movedPlatform makes a remote that is the guest's repository plus one newer
// commit (the platform main the computer booted), and boots the runtime on it.
func movedPlatform(t *testing.T, computerID string) (rt *Runtime, updaterRoot, repo, creation, booted, remote string) {
	t.Helper()
	rt, updaterRoot, repo, creation = projectionRuntime(t, computerID)
	remote = t.TempDir()
	releaseGit(t, remote, "clone", "-q", repo, ".")
	releaseGit(t, remote, "config", "uploadpack.allowAnySHA1InWant", "true")
	if err := os.WriteFile(filepath.Join(remote, "frontend", "public", "later.txt"), []byte("platform moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, remote, "add", "-A")
	releaseGit(t, remote, "commit", "-q", "-m", "platform main moved")
	booted = releaseGit(t, remote, "rev-parse", "HEAD")
	setBootedBase(t, rt, booted)
	writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	return rt, updaterRoot, repo, creation, booted, remote
}

func fastFetch(remote string, backoff ...time.Duration) *sourceBaseFetchTuning {
	return &sourceBaseFetchTuning{
		Backoff: backoff,
		Options: func() sourceproj.FetchOptions {
			return sourceproj.FetchOptions{Remote: remote, Timeout: time.Minute, MinFreeBytes: 1}
		},
	}
}

func factKinds(t *testing.T, updaterRoot string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(updaterRoot, "source-projection", "facts.jsonl"))
	if err != nil {
		return nil
	}
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if i := strings.Index(line, `"kind":"`); i >= 0 {
			rest := line[i+len(`"kind":"`):]
			kinds = append(kinds, rest[:strings.Index(rest, `"`)])
		}
	}
	return kinds
}

func TestBootFetchesTheBootedBaseAndAnEmptyStackMovesToIt(t *testing.T) {
	rt, updaterRoot, repo, creation, booted, remote := movedPlatform(t, "computer-u1b-boot")
	rt.sourceBaseFetchTune = fastFetch(remote)
	ctx := context.Background()

	// Before: U1 as built (B2): the objects are missing and the cast would go on.
	if result, _ := rt.ReconcileSourceProjection(ctx, "boot"); result.State != sourceproj.StateBaseObjectsMissing || gitHead(t, repo) != creation {
		t.Fatalf("before: %+v", result)
	}
	rt.startSourceProjection(ctx, "boot")
	if gitHead(t, repo) != booted {
		t.Fatalf("HEAD = %s, want the booted base %s", gitHead(t, repo), booted)
	}
	if out := releaseGit(t, repo, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Fatalf("work tree not clean: %s", out)
	}
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateBase || status.Head != booted {
		t.Fatalf("status = %+v", status)
	}
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	kinds := strings.Join(factKinds(t, updaterRoot), ",")
	if !strings.Contains(kinds, sourceproj.FactBaseFetched) {
		t.Fatalf("facts = %s", kinds)
	}
	// The move names where it came from: the rollback handle.
	raw, _ := os.ReadFile(filepath.Join(updaterRoot, "source-projection", "facts.jsonl"))
	if !strings.Contains(string(raw), `"from":"`+creation+`"`) {
		t.Fatalf("no fact names the creation commit as from:\n%s", raw)
	}
	// A second boot finds the objects and uses no network.
	var used atomic.Int32
	rt.sourceBaseFetchTune = &sourceBaseFetchTuning{Options: func() sourceproj.FetchOptions { used.Add(1); return sourceproj.FetchOptions{} }}
	rt.startSourceProjection(ctx, "boot")
	if used.Load() != 0 {
		t.Fatal("the objects were present and a fetch was still tried")
	}
}

func TestUnreachableRemoteRetriesThenStopsAndTheCastGoesOnAsBefore(t *testing.T) {
	rt, updaterRoot, repo, creation, _, _ := movedPlatform(t, "computer-u1b-down")
	var attempts atomic.Int32
	rt.sourceBaseFetchTune = &sourceBaseFetchTuning{
		Backoff: []time.Duration{5 * time.Millisecond, 5 * time.Millisecond},
		Options: func() sourceproj.FetchOptions {
			attempts.Add(1)
			return sourceproj.FetchOptions{Remote: t.TempDir(), MinFreeBytes: 1} // not a repository
		},
	}
	ctx := context.Background()
	rt.startSourceProjection(ctx, "boot")
	// The first attempt plus one per back-off, and no more (N1).
	if got := attempts.Load(); got < 1 || got > 3 {
		t.Fatalf("attempts = %d", got)
	}
	if gitHead(t, repo) != creation {
		t.Fatal("a failed fetch moved HEAD")
	}
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateBaseObjectsMissing || !strings.Contains(status.Reason, "the fetch of it") {
		t.Fatalf("the reason does not say why: %+v", status)
	}
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("B2: the gate must stay open: %v", err)
	}
	if n := strings.Count(strings.Join(factKinds(t, updaterRoot), ","), sourceproj.FactBaseFetchFailed); n != 1 {
		t.Fatalf("%d failure facts for one distinct failure", n)
	}
}

func TestACommitTheRemoteLacksIsNotRetried(t *testing.T) {
	rt, _, _, _, _, remote := movedPlatform(t, "computer-u1b-gone")
	setBootedBase(t, rt, strings.Repeat("a", 40))
	var attempts atomic.Int32
	rt.sourceBaseFetchTune = &sourceBaseFetchTuning{
		Backoff: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Options: func() sourceproj.FetchOptions {
			attempts.Add(1)
			return sourceproj.FetchOptions{Remote: remote, MinFreeBytes: 1}
		},
	}
	rt.startSourceProjection(context.Background(), "boot")
	if attempts.Load() != 1 {
		t.Fatalf("N2: %d attempts for a commit the remote does not have", attempts.Load())
	}
}

func TestSwitchOffFetchesNothingAtBoot(t *testing.T) {
	rt, updaterRoot, repo, creation, booted, remote := movedPlatform(t, "computer-u1b-off")
	rt.sourceBaseFetchTune = fastFetch(remote)
	t.Setenv(sourceproj.EnvSwitch, "")
	rt.startSourceProjection(context.Background(), "boot")
	if gitHead(t, repo) != creation || sourceproj.HasCommit(context.Background(), repo, booted) {
		t.Fatal("N13: something moved with the switch off")
	}
	if _, err := os.Stat(filepath.Join(updaterRoot, "source-projection")); !os.IsNotExist(err) {
		t.Fatal("N13: a fact log appeared")
	}
}

func TestTheCastGateWaitsBoundedForAFetchInFlight(t *testing.T) {
	rt, _, _, _, _, _ := movedPlatform(t, "computer-u1b-wait")
	rt.sourceBaseFetchTune = &sourceBaseFetchTuning{GateWait: 2 * time.Second}
	ctx := context.Background()

	// Nothing in flight: no wait at all.
	start := time.Now()
	rt.waitSourceBaseFetch(ctx)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("the gate waited with no fetch in flight")
	}
	// In flight: it waits for the end of the attempt, not for the bound.
	done := rt.beginSourceBaseAttempt()
	go func() { time.Sleep(100 * time.Millisecond); rt.endSourceBaseAttempt(done) }()
	start = time.Now()
	rt.waitSourceBaseFetch(ctx)
	if d := time.Since(start); d < 80*time.Millisecond || d > 1500*time.Millisecond {
		t.Fatalf("waited %s for a 100ms attempt", d)
	}
	// An attempt that never ends costs the gate only the bound.
	rt.sourceBaseFetchTune = &sourceBaseFetchTuning{GateWait: 150 * time.Millisecond}
	stuck := rt.beginSourceBaseAttempt()
	defer rt.endSourceBaseAttempt(stuck)
	start = time.Now()
	rt.waitSourceBaseFetch(ctx)
	if d := time.Since(start); d < 120*time.Millisecond || d > time.Second {
		t.Fatalf("waited %s against a 150ms bound", d)
	}
}
