package agentcore

// The frontend follows the backend (docs/problems/
// stale-frontend-after-base-restore-2026-11-09.md): at boot a `current` that
// is a pure base import of an older deploy is restaged from the booted
// baseline; nothing else is touched.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

func followBaseRuntime(t *testing.T, computerID string) (*Runtime, string, string) {
	t.Helper()
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() {
		_ = filepath.Walk(updaterRoot, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	baseline := filepath.Join(updaterRoot, "incoming", "baseline-root")
	if err := os.MkdirAll(filepath.Join(baseline, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseline, "frontend", "index.html"), []byte("<html>new deploy</html>"), 0o444); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, selfdevUpdaterRoot: updaterRoot, selfdevComputerID: computerID}
	return rt, updaterRoot, baseline
}

func servedIndex(t *testing.T, updaterRoot string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func currentTarget(t *testing.T, updaterRoot string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(updaterRoot, "current"))
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestBootFollowsTheBaseWhenCurrentIsAStalePureBaseImport(t *testing.T) {
	computerID := "computer-follow-base"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>new deploy</html>" {
		t.Fatalf("served %q, want the booted baseline", got)
	}
	if manifest.ContentDigest == staleDigest || filepath.Base(currentTarget(t, updaterRoot)) != manifest.ContentDigest {
		t.Fatalf("current %s does not carry the returned digest %s", currentTarget(t, updaterRoot), manifest.ContentDigest)
	}
	// The reported digest is the served bytes' digest.
	if startup := rt.startupManifest(); startup.Digest != manifest.ContentDigest {
		t.Fatalf("startup digest %s, want %s", startup.Digest, manifest.ContentDigest)
	}
	if _, err := updater.VerifyCurrentRelease(updaterRoot); err != nil {
		t.Fatalf("restaged current does not verify: %v", err)
	}
	// The stale pin stays, so a binder to its digest still resolves.
	if _, _, err := updater.ReadPinnedManifest(updaterRoot, staleDigest); err != nil {
		t.Fatalf("stale pin lost: %v", err)
	}
	// A second boot restages nothing.
	before := currentTarget(t, updaterRoot)
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if currentTarget(t, updaterRoot) != before {
		t.Fatal("a second boot moved current")
	}
}

// An unchanged frontend across a deploy means no copy: the release digests
// differ (the binary and build stamp changed) but the served bytes do not.
func TestBootDoesNotRestageWhenTheServedFrontendIsUnchanged(t *testing.T) {
	computerID := "computer-follow-unchanged"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	sameDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>new deploy</html>")
	pointCurrent(t, updaterRoot, sameDigest)
	before, err := os.ReadDir(filepath.Join(updaterRoot, "releases"))
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ContentDigest != sameDigest || filepath.Base(currentTarget(t, updaterRoot)) != sameDigest {
		t.Fatalf("current moved to %s although the frontend is unchanged", currentTarget(t, updaterRoot))
	}
	after, err := os.ReadDir(filepath.Join(updaterRoot, "releases"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("a release was staged: %d -> %d", len(before), len(after))
	}
	if rt.startupManifest().Digest != sameDigest {
		t.Fatal("the startup manifest changed")
	}
}

func TestBootLeavesALayeredOrSelfdevCurrentAlone(t *testing.T) {
	computerID := "computer-follow-layered"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	digest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>selfdev round</html>")
	manifest, _, err := updater.ReadPinnedManifest(updaterRoot, digest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Marker = "selfdev-bundle1"
	if manifest, err = updater.FinalizeManifest(manifest); err != nil {
		t.Fatal(err)
	}
	// Re-pin under the selfdev digest.
	releaseDir := filepath.Join(updaterRoot, "releases", manifest.ContentDigest)
	if err := os.CopyFS(releaseDir, os.DirFS(filepath.Join(updaterRoot, "releases", digest))); err != nil {
		t.Fatal(err)
	}
	encoded, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(releaseDir, "release-manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "release-manifest.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	pointCurrent(t, updaterRoot, manifest.ContentDigest)

	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>selfdev round</html>" {
		t.Fatalf("a selfdev release was restaged: served %q", got)
	}
}

func TestBootKeepsAStaleCurrentWhileAnApplyJournalIsUnsettled(t *testing.T) {
	computerID := "computer-follow-journal"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	ops := filepath.Join(updaterRoot, "operations")
	if err := os.MkdirAll(ops, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ops, "apply-1.json"), []byte(`{"phase":"prepared","prior_release_digest":"`+staleDigest+`","result":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>old deploy</html>" {
		t.Fatalf("restaged under an unsettled apply: served %q", got)
	}
	// Settled, it follows.
	if err := os.WriteFile(filepath.Join(ops, "apply-1.json"), []byte(`{"phase":"completed","result":{"outcome":"failed"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>new deploy</html>" {
		t.Fatalf("settled journal did not release the restage: served %q", got)
	}
}

func TestCheckpointPathNeverMovesCurrent(t *testing.T) {
	computerID := "computer-follow-checkpoint"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	manifest, err := rt.ensureServingBaseline(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ContentDigest != staleDigest || !strings.Contains(servedIndex(t, updaterRoot), "old deploy") {
		t.Fatalf("the checkpoint path moved current to %s", manifest.ContentDigest)
	}
}

func TestBootRefusesAForeignComputersCurrent(t *testing.T) {
	rt, updaterRoot, baseline := followBaseRuntime(t, "computer-follow-self")
	other, _ := pinFrontendRelease(t, updaterRoot, "computer-follow-other", "<html>other</html>")
	pointCurrent(t, updaterRoot, other)
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), "computer-follow-self", baseline); err == nil {
		t.Fatal("a current bound to another computer was accepted")
	}
	if got := servedIndex(t, updaterRoot); got != "<html>other</html>" {
		t.Fatalf("foreign current was restaged: %q", got)
	}
}
