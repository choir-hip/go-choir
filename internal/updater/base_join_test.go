package updater

// A self-development release that is not host-built (a plain release) now
// declares the base commit its candidate was authored against
// (docs/problems/selfdev-apply-has-no-base-join-2026-10-26.md). Ways the join
// could fail, written first:
//   - a candidate frozen on an older base installs over a newer booted base
//     and reverts that base's fixes (the defect);
//   - the refusal does not name both commits, or is not the typed
//     base_moved_since_freeze reason;
//   - a case difference refuses a release on the same commit;
//   - a short commit is treated as a prefix match and passes;
//   - a release that declares no base commit (every older manifest) is
//     refused instead of applying as before;
//   - a replay of an operation the journal already settled as applied is
//     refused after the booted base moved.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	joinBaseA = "0123456789abcdef0123456789abcdef01234567"
	joinBaseB = "89abcdef0123456789abcdef0123456789abcdef"
)

func joinEngine(t *testing.T, bootedCommit string) (*Updater, string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	path, _ := compatTestBase(t, bootedCommit)
	engine, _ := compatEngine(t, root, path, "")
	return engine, root, path
}

func joinRequest(t *testing.T, root, operation, baseCommit string) ApplyRequest {
	t.Helper()
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", operation, "idem-"+operation, "plain release "+operation)
	request.Manifest.BaseCommit = baseCommit
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	return request
}

func TestPlainReleaseBaseJoin(t *testing.T) {
	cases := []struct {
		name    string
		booted  string
		release string
		refused bool
	}{
		{"same commit applies", joinBaseA, joinBaseA, false},
		{"release names the commit in upper case", joinBaseA, strings.ToUpper(joinBaseA), false},
		{"booted base names the commit in upper case", strings.ToUpper(joinBaseA), joinBaseA, false},
		{"older base is refused", joinBaseB, joinBaseA, true},
		{"a short commit is a different string, not a prefix match", joinBaseA, joinBaseA[:12], true},
		{"a release that declares no base commit applies on any base", joinBaseB, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, root, _ := joinEngine(t, tc.booted)
			request := joinRequest(t, root, "op-join", tc.release)
			result, err := engine.Apply(context.Background(), request)
			if !tc.refused {
				if err != nil || result.Outcome != "applied" {
					t.Fatalf("apply = %+v err=%v, want applied", result, err)
				}
				return
			}
			if err == nil || result.Outcome != "refused" {
				t.Fatalf("apply = %+v err=%v, want a pre-mutation refusal", result, err)
			}
			for _, want := range []string{"base_moved_since_freeze", tc.release, tc.booted} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal %q does not name %q", err, want)
				}
			}
			if _, statErr := os.Lstat(filepath.Join(root, "releases", request.Manifest.ContentDigest)); !os.IsNotExist(statErr) {
				t.Fatal("release staged despite refusal")
			}
			if _, statErr := os.Lstat(filepath.Join(root, "current")); !os.IsNotExist(statErr) {
				t.Fatal("current pointer created despite refusal")
			}
		})
	}
}

// A booted base that carries no build_commit cannot prove a join: a release
// that declares one is refused with the reason, never silently applied.
func TestPlainReleaseRefusedWhenBootedBaseNamesNoCommit(t *testing.T) {
	engine, root, path := joinEngine(t, joinBaseA)
	if err := os.WriteFile(path, []byte("contract=choir-guest-image-v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := engine.Apply(context.Background(), joinRequest(t, root, "op-nocommit", joinBaseA))
	if err == nil || !strings.Contains(err.Error(), "base_moved_since_freeze") || !strings.Contains(err.Error(), joinBaseA) {
		t.Fatalf("apply err=%v, want a base_moved_since_freeze refusal naming the release's commit", err)
	}
}

// A guest with no booted base wired (the updater was not given a manifest)
// cannot join and keeps the plain file-release path.
func TestPlainReleaseAppliesWhereNoBaseIsWired(t *testing.T) {
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	engine, _ := compatEngine(t, root, "", "")
	if result, err := engine.Apply(context.Background(), joinRequest(t, root, "op-unwired", joinBaseA)); err != nil || result.Outcome != "applied" {
		t.Fatalf("apply = %+v err=%v, want applied", result, err)
	}
}

// A replay of an operation the journal settled as applied returns that
// outcome, even after the booted base moved: the join judges a new apply
// only (the crash-boot settlement and the recorded outcome are never
// refused by it).
func TestAppliedJournalReplayIsNotRefusedAfterTheBaseMoved(t *testing.T) {
	engine, root, path := joinEngine(t, joinBaseA)
	request := joinRequest(t, root, "op-replay", joinBaseA)
	if result, err := engine.Apply(context.Background(), request); err != nil || result.Outcome != "applied" {
		t.Fatalf("first apply = %+v err=%v", result, err)
	}
	if err := os.WriteFile(path, []byte("contract=choir-guest-image-v1\nbuild_commit="+joinBaseB+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result, err := engine.Apply(context.Background(), request); err != nil || result.Outcome != "applied" {
		t.Fatalf("replay after the base moved = %+v err=%v, want the recorded applied outcome", result, err)
	}
	journaled, found, err := ReadJournalOutcome(root, request.IdempotencyKey)
	if err != nil || !found || !journaled.Terminal || journaled.Result.Outcome != "applied" {
		t.Fatalf("journal = %+v found=%v err=%v", journaled, found, err)
	}
}
