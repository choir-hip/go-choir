//go:build linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Source-only freeze (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md
// 3.1). Failure modes, decided before the code:
//
//   - a platform change whose capsule staged no release tree is refused with
//     "no frozen runtime artifacts" (apply-proof-3), so no Go candidate can
//     freeze;
//   - the opposite: a capsule with neither a tree nor a patch freezes an
//     empty bundle;
//   - a capsule that still stages a release tree (a test binary) loses its
//     checks, or its tree becomes the candidate;
//   - a valid shot plan does not satisfy screens_required, or an invalid one
//     is dropped silently instead of refusing the Freeze;
//   - a plan rides on a package candidate, which is shot in its capsule.

type sourceOnlyLayout struct {
	source map[string]string // workspace/platform-relative
	tree   bool              // stage a release tree (frontend + binary)
	screen bool
	plan   string // contents of /var/lib/artifact/shot-plan.json
	// shots are screens by name with their mtime relative to the source
	// files (which are an hour old): a before shot with an offset above -1h
	// was shot after the first source edit.
	shots     map[string]time.Duration
	sourceRef string
	// baseDirs are platform-relative directories that exist on the pinned
	// base (the source lower root), so adding to one is a change to an
	// existing surface that needs a before and after pair.
	baseDirs []string
}

func stageSourceOnly(t *testing.T, l sourceOnlyLayout) (*StagedRelease, error) {
	t.Helper()
	merged := filepath.Join(t.TempDir(), "merged")
	upper := t.TempDir()
	if err := os.MkdirAll(merged, 0o700); err != nil {
		t.Fatal(err)
	}
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
	if l.tree {
		write("var/lib/artifact/release/frontend/index.html", "<html>computer</html>", 0o644)
		write("var/lib/artifact/release/bin/autoputer", "capsule test build", 0o755)
	}
	for rel, content := range l.source {
		write("workspace/platform/"+rel, content, 0o644)
	}
	if l.screen {
		write("var/lib/artifact/screens/home.png", string(testPNG(t, 1100, 750, false)), 0o644)
	}
	for name, offset := range l.shots {
		write("var/lib/artifact/screens/"+name, string(testPNG(t, 1100, 750, false)), 0o644)
		at := time.Now().Add(offset)
		if err := os.Chtimes(filepath.Join(merged, "var/lib/artifact/screens", name), at, at); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	for rel := range l.source {
		if err := os.Chtimes(filepath.Join(merged, "workspace/platform", rel), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if l.plan != "" {
		write("var/lib/artifact/shot-plan.json", l.plan, 0o644)
	}
	for _, dir := range l.baseDirs {
		if err := os.MkdirAll(filepath.Join(filepath.Dir(merged), "source-lower", platformSourcePrefix, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-so", Handle: "grant-so", CapsuleID: "capsule-so",
		TargetCapsule: "capsule-so", AgentRunID: "run-so", AgentRole: RoleEngineering,
		Verbs: RoleVerbSets()[RoleEngineering], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-so": {
			ID: "capsule-so", State: StateFrozen, UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20,
			Spec: SpawnSpec{SourceArtifactRef: l.sourceRef},
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "run-so", Handle: "grant-so"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	incoming := t.TempDir()
	if err := os.Chmod(incoming, 0o700); err != nil {
		t.Fatal(err)
	}
	staged, err := executor.StageGrantedRelease(context.Background(), "run-so", "grant-so", incoming)
	if staged != nil {
		t.Cleanup(func() { _ = os.RemoveAll(staged.TemporaryRoot) })
	}
	return staged, err
}

const soPlan = `{"views":[{"name":"home","path":"/","viewport":"desktop","steps":[{"op":"wait_for","selector":"main"}]}]}`

func TestStageGrantedReleaseShipsSourceOnlyWithNoReleaseTree(t *testing.T) {
	pinned := "capsule-source-git:" + strings.Repeat("a", 40) + ":sha256:" + strings.Repeat("d", 64)
	goChange := map[string]string{"internal/agentcore/x.go": "package agentcore\n"}

	staged, err := stageSourceOnly(t, sourceOnlyLayout{source: goChange, screen: true, sourceRef: pinned})
	if err != nil {
		t.Fatalf("a Go change with no release tree must freeze as source: %v", err)
	}
	if len(staged.Files) != 0 {
		t.Fatalf("a source-only freeze staged files: %+v", staged.Files)
	}
	if b := staged.Binary; b == nil || b.Origin != ReleaseBinaryHostBuild || b.SHA256 != "" {
		t.Fatalf("provenance = %+v, want host_build with no sha", staged.Binary)
	}
	if staged.SourcePatchSHA == "" || staged.SourceBaseCommit != strings.Repeat("a", 40) {
		t.Fatalf("source patch %q base %q", staged.SourcePatchSHA, staged.SourceBaseCommit)
	}
	if _, err := os.Stat(filepath.Join(staged.TemporaryRoot, "bin", "autoputer")); err == nil {
		t.Fatal("a source-only freeze staged a binary beside the patch")
	}
}

func TestStageGrantedReleaseARefusedFreezeStillNeedsSourceOrTree(t *testing.T) {
	// Neither a release tree nor a source patch: the old refusal stands.
	_, err := stageSourceOnly(t, sourceOnlyLayout{})
	if err == nil || !strings.Contains(err.Error(), "no frozen runtime artifacts") {
		t.Fatalf("an empty freeze = %v", err)
	}
}

func TestStageGrantedReleaseWithATreeAndAPatchKeepsTodaysChecks(t *testing.T) {
	pinned := "capsule-source-git:" + strings.Repeat("a", 40) + ":sha256:" + strings.Repeat("d", 64)
	staged, err := stageSourceOnly(t, sourceOnlyLayout{source: map[string]string{"internal/agentcore/x.go": "package agentcore\n"}, tree: true, screen: true, sourceRef: pinned})
	if err != nil {
		t.Fatalf("a capsule that built a release tree and a patch must still freeze: %v", err)
	}
	if len(staged.Files) != 2 || staged.Binary == nil || staged.Binary.Origin != ReleaseBinaryCapsuleBuilt {
		t.Fatalf("the tree path changed: files=%+v binary=%+v", staged.Files, staged.Binary)
	}
	if staged.SourcePatchSHA == "" {
		t.Fatal("the patch was dropped beside the tree")
	}
}

func TestStageGrantedReleaseShotPlanSatisfiesTheScreensRule(t *testing.T) {
	pinned := "capsule-source-git:" + strings.Repeat("a", 40) + ":sha256:" + strings.Repeat("d", 64)
	goChange := map[string]string{"internal/agentcore/x.go": "package agentcore\n"}

	// No screen, no plan: screens_required.
	if _, err := stageSourceOnly(t, sourceOnlyLayout{source: goChange, sourceRef: pinned}); err == nil || !strings.Contains(err.Error(), ScreensRequiredReason) {
		t.Fatalf("a Go change with neither screens nor a plan = %v", err)
	}
	// A valid plan satisfies it (and before_after_required).
	staged, err := stageSourceOnly(t, sourceOnlyLayout{source: goChange, plan: soPlan, sourceRef: pinned})
	if err != nil {
		t.Fatalf("a valid plan must satisfy the screens rule: %v", err)
	}
	if staged.ShotPlan == nil || len(staged.ShotPlan.Views) != 1 || len(staged.Screens) != 0 {
		t.Fatalf("plan=%+v screens=%v", staged.ShotPlan, staged.Screens)
	}
	// An invalid plan refuses the Freeze, never dropped.
	for name, plan := range map[string]string{
		"bad viewport": `{"views":[{"name":"home","path":"/","viewport":"tv"}]}`,
		"unknown key":  `{"views":[{"name":"home","path":"/","viewport":"phone","script":"x"}]}`,
		"not json":     `views`,
	} {
		if _, err := stageSourceOnly(t, sourceOnlyLayout{source: goChange, plan: plan, screen: true, sourceRef: pinned}); err == nil || !strings.Contains(err.Error(), "shot plan") {
			t.Errorf("%s: a bad plan was admitted or not named: %v", name, err)
		}
	}
}

func TestStageGrantedReleaseRefusesAPlanOnAPackageCandidate(t *testing.T) {
	pinned := "capsule-source-git:" + strings.Repeat("a", 40) + ":sha256:" + strings.Repeat("d", 64)
	pkg := map[string]string{"frontend/src/packages/notes/App.svelte": "<p>x</p>\n"}
	_, err := stageSourceOnly(t, sourceOnlyLayout{source: pkg, plan: soPlan, screen: true, sourceRef: pinned})
	if err == nil || !strings.Contains(err.Error(), "platform source change") {
		t.Fatalf("a plan on a package candidate = %v", err)
	}
}

// The late-before rule judges screens the capsule shot, and only where the
// diff needs a before and after pair (needsBeforeAfterPair): a late before
// shot on a change to an existing surface is refused; the same shots on a
// diff that needs no pair (a new Go file, a new package) are not. A plan
// bundle's before shots come from the running build on the platform, so the
// rule is never applied to one (design 3.3, Freeze rule); the plan arm is the
// `!planned` term in stageScreens, which a package candidate cannot exercise
// here (a plan on a package candidate is refused).
func TestStageGrantedReleaseJudgesLateBeforeShotsOnlyWhereAPairIsRequired(t *testing.T) {
	pinned := "capsule-source-git:" + strings.Repeat("a", 40) + ":sha256:" + strings.Repeat("d", 64)
	existing := map[string]string{"frontend/src/packages/notes/Extra.svelte": "<p>x</p>\n"}
	baseDirs := []string{"frontend/src/packages/notes"}
	newPackage := map[string]string{"frontend/src/packages/fresh/App.svelte": "<p>x</p>\n"}
	goChange := map[string]string{"internal/agentcore/x.go": "package agentcore\n"}
	shots := map[string]time.Duration{"01-before-home.png": -2 * time.Hour, "01-after-home.png": -time.Minute}
	late := map[string]time.Duration{"01-before-home.png": -30 * time.Minute, "01-after-home.png": -time.Minute}
	if _, err := stageSourceOnly(t, sourceOnlyLayout{source: existing, baseDirs: baseDirs, shots: shots, sourceRef: pinned}); err != nil {
		t.Fatalf("control: a timely before shot was refused: %v", err)
	}
	_, err := stageSourceOnly(t, sourceOnlyLayout{source: existing, baseDirs: baseDirs, shots: late, sourceRef: pinned})
	if err == nil || !strings.Contains(err.Error(), BeforeAfterRequiredReason) {
		t.Fatalf("an add to an existing package with a late before shot = %v, want before_after_required", err)
	}
	for name, source := range map[string]map[string]string{"a new package": newPackage, "a new Go file": goChange} {
		if _, err := stageSourceOnly(t, sourceOnlyLayout{source: source, shots: late, sourceRef: pinned}); err != nil {
			t.Errorf("%s needs no pair, so a late before shot must not refuse it: %v", name, err)
		}
	}
}
