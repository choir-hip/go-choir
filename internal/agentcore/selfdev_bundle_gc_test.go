package agentcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func TestRemoveUnnamedBundlesKeepsWhatAnOpenOperationNamesAndWhatIsYoung(t *testing.T) {
	incoming := t.TempDir()
	now := time.Now()
	make := func(name, marker string, age time.Duration) string {
		dir := filepath.Join(incoming, name)
		if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{filepath.Join(dir, marker), filepath.Join(dir, "evidence", "shot.png")} {
			if err := os.WriteFile(p, []byte("12345"), 0o444); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(filepath.Join(dir, "evidence"), now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(dir, "evidence"), 0o555); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	settled := make("1"+strings.Repeat("a", 63), "bundle.json", 48*time.Hour)
	settledDraft := make("2"+strings.Repeat("a", 63), "bundle.draft.json", 48*time.Hour)
	open := make("3"+strings.Repeat("a", 63), "bundle.json", 48*time.Hour)
	young := make("4"+strings.Repeat("a", 63), "bundle.json", time.Minute)
	// Host builds are the runtime's too: kept while their bundle is named.
	hostBuild := func(name, bundle string, age time.Duration) string {
		dir := filepath.Join(incoming, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		raw := `{"job_id":"` + name + `","bundle_digest":"` + bundle + `"}`
		file := filepath.Join(dir, "host-build.json")
		if err := os.WriteFile(file, []byte(raw), 0o444); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{file, dir} {
			if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	hbOpen := hostBuild("host-build-open", filepath.Base(open), 48*time.Hour)
	hbSettled := hostBuild("host-build-settled", filepath.Base(settled), 48*time.Hour)
	hbYoung := hostBuild("host-build-young", filepath.Base(settled), time.Minute)
	payload := filepath.Join(incoming, "5"+strings.Repeat("a", 63))
	if err := os.MkdirAll(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, d := range []string{settled, settledDraft, open, young} {
			_ = os.Chmod(filepath.Join(d, "evidence"), 0o755)
		}
	})
	removed, freed := removeUnnamedBundles(incoming, map[string]bool{filepath.Base(open): true}, now, 10*time.Minute)
	if removed != 3 {
		t.Fatalf("removed=%d freed=%d", removed, freed)
	}
	for _, gone := range []string{settled, settledDraft, hbSettled} {
		if _, err := os.Lstat(gone); err == nil {
			t.Fatalf("%s survived", gone)
		}
	}
	for _, kept := range []string{open, young, payload, hbOpen, hbYoung} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("%s was removed: %v", kept, err)
		}
	}
}

// B2: the bundle behind a release that is still in releases/ is read by
// source projection, the platform-update gate, clean rebase, rebase offers and
// the effective release. It is kept; the bundle of an applied operation whose
// release is gone is collected.
func TestCollectSettledBundlesKeepsTheBundleBehindASurvivingRelease(t *testing.T) {
	const computerID = "computer-bundle-gc"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	rt.selfdevComputerID = computerID
	ctx := context.Background()
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	other, _ := releasePatch(t, repo, map[string]string{"frontend/public/b.txt": "B\n"})
	goneBundle := releaseFrozenBundle(t, updaterRoot, computerID, "op-gone", base, other)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "op-gone", computerID: computerID, state: selfdev.StateApplied,
		bundleDigest: goneBundle, releaseDigest: strings.Repeat("7", 64)})
	old := time.Now().Add(-48 * time.Hour)
	entries, _ := os.ReadDir(filepath.Join(updaterRoot, "incoming"))
	for _, entry := range entries {
		_ = filepath.WalkDir(filepath.Join(updaterRoot, "incoming", entry.Name()), func(p string, _ os.DirEntry, err error) error {
			if err == nil {
				_ = os.Chtimes(p, old, old)
			}
			return nil
		})
	}
	rt.collectSettledBundles(ctx)
	if _, err := os.Lstat(filepath.Join(updaterRoot, "incoming", goneBundle)); err == nil {
		t.Fatal("the bundle of an applied operation whose release is gone was kept")
	}
	if in := rt.sourceProjectionInput(ctx); in.TopUnavailable != "" || in.Top == nil {
		t.Fatalf("source projection lost its bundle after the collection: %+v", in)
	}
	if refusal := rt.unmigratedPackageStackRefusal(ctx); refusal != "" {
		t.Fatalf("a platform update would be refused after the collection: %s", refusal)
	}
}
