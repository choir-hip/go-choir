package agentcore

// Review round 1 of the boot follow: route hiccups, a refused swap, a crash
// between stage and swap, and the package-migration rollback after a follow.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/routeledger"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The real boot answer: vmctl does not count the computer as live yet, so
// every route read is refused (403). The follow must not need the route; the
// restaged manifest carries exactly the refs `current` records, and a route
// that would answer other refs is never consulted.
func TestBootFollowsWithoutConsultingTheRoute(t *testing.T) {
	computerID := "computer-follow-route"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	stale, _, err := updater.ReadPinnedManifest(updaterRoot, staleDigest)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "caller not bound to route slot computer: caller is not bound to a live computer", http.StatusForbidden)
	}))
	defer server.Close()
	rt.selfdevRoute = vmctl.NewClient(server.URL)
	rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID = "owner-follow-route", "primary"
	if _, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID); err != nil {
		t.Fatal(err)
	}

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>new deploy</html>" {
		t.Fatalf("a refused route blocked the follow: served %q", got)
	}
	if manifest.CodeRef != stale.CodeRef || manifest.ArtifactProgramRef != stale.ArtifactProgramRef || manifest.CodeRef == "code:genesis" {
		t.Fatalf("refs %q %q, want current's %q %q", manifest.CodeRef, manifest.ArtifactProgramRef, stale.CodeRef, stale.ArtifactProgramRef)
	}
	if calls.Load() != 0 {
		t.Fatalf("the follow consulted the route %d times", calls.Load())
	}
}

// A current that records genesis refs restages with genesis refs.
func TestBootFollowCarriesGenesisRefsWhenCurrentRecordsThem(t *testing.T) {
	computerID := "computer-follow-genesis"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	// Stage genesis from a trusted copy of the old content.
	oldRoot := filepath.Join(updaterRoot, "incoming", "old-root")
	if err := os.MkdirAll(filepath.Join(oldRoot, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "frontend", "index.html"), []byte("<html>old deploy</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	genesis, err := updater.BuildBaselineManifest(oldRoot, computerID, "code:genesis", "artifact-program:genesis")
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.StageBaselineRelease(updaterRoot, oldRoot, genesis); err != nil {
		t.Fatal(err)
	}
	if err := updater.RestagePinnedRelease(updaterRoot, genesis.ContentDigest); err != nil {
		t.Fatal(err)
	}
	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if servedIndex(t, updaterRoot) != "<html>new deploy</html>" || manifest.CodeRef != "code:genesis" || manifest.ArtifactProgramRef != "artifact-program:genesis" {
		t.Fatalf("served %q refs %q %q", servedIndex(t, updaterRoot), manifest.CodeRef, manifest.ArtifactProgramRef)
	}
}

// A refused stage keeps boot alive and the old current. (A baseline declares
// no store epoch, so the floor cannot refuse by value; a floor that cannot be
// read is the refusal the check can return here.)
func TestBootSurvivesARefusedStageAndKeepsTheOldCurrent(t *testing.T) {
	computerID := "computer-follow-floor"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	store, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rt.store = store
	if err := os.WriteFile(filepath.Join(store.TexturePath(), storeschema.FloorFile), []byte("not a floor"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatalf("a refused follow failed the boot: %v", err)
	}
	if manifest.ContentDigest != staleDigest || servedIndex(t, updaterRoot) != "<html>old deploy</html>" {
		t.Fatalf("the old current was not kept: %s", manifest.ContentDigest)
	}
}

// A crash between stage and swap leaves a complete staged dir for the target;
// the next boot swaps to it.
func TestBootConvergesAfterACrashBetweenStageAndSwap(t *testing.T) {
	computerID := "computer-follow-crash"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	stale, _, err := updater.ReadPinnedManifest(updaterRoot, staleDigest)
	if err != nil {
		t.Fatal(err)
	}
	target, err := updater.BuildBaselineManifest(baseline, computerID, stale.CodeRef, stale.ArtifactProgramRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.StageBaselineRelease(updaterRoot, baseline, target); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>old deploy</html>" {
		t.Fatalf("setup: staging moved current: %q", got)
	}

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ContentDigest != target.ContentDigest || servedIndex(t, updaterRoot) != "<html>new deploy</html>" {
		t.Fatalf("did not converge: %s", manifest.ContentDigest)
	}
	if _, err := updater.VerifyCurrentRelease(updaterRoot); err != nil {
		t.Fatal(err)
	}
}

// changeBootedFrontend is a deploy that changes the frontend of the booted
// baseline.
func (e *pmEnv) changeBootedFrontend(t *testing.T, html string) {
	t.Helper()
	index := filepath.Join(e.baseline, "frontend", "index.html")
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, []byte(html), 0o444); err != nil {
		t.Fatal(err)
	}
}

// PM16 after a follow: the restaged pure base is the same base the migration
// served, not a later change; the rollback restores the snapshot from it.
func TestPackageMigrationRollbackSucceedsAfterTheBootFollowedTheBase(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	rec := e.migrate(t)
	if e.lastAttempt(t, rec).Outcome != pmDone {
		t.Fatal("setup: the migration must finish")
	}
	served := e.currentDigest(t)
	e.changeBootedFrontend(t, "<html>next deploy</html>")
	if _, err := e.rt.ensureServingBaselineAtBoot(ctx, pmComputer, e.baseline); err != nil {
		t.Fatal(err)
	}
	if e.currentDigest(t) == served {
		t.Fatal("setup: the boot must have followed the changed frontend")
	}
	rec, err := e.rt.rollbackPackageMigration(ctx, rec.Pair)
	if err != nil {
		t.Fatalf("rollback after a follow: %v", err)
	}
	if a := e.lastAttempt(t, rec); a.Outcome != pmRolledBack {
		t.Fatalf("attempt = %+v", a)
	}
	if e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatal("the rollback must serve the snapshot and uninstall the migration's install")
	}
}

// A real drift still refuses: a selfdev release in current is a later change.
func TestPackageMigrationRollbackStillRefusesRealDrift(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	rec := e.migrate(t)
	if e.lastAttempt(t, rec).Outcome != pmDone {
		t.Fatal("setup: the migration must finish")
	}
	// The snapshot release is a selfdev release; serving it is not the base.
	if err := updater.RestagePinnedRelease(e.updaterRoot, e.preRelease); err != nil {
		t.Fatal(err)
	}
	if _, err := e.rt.rollbackPackageMigration(ctx, rec.Pair); err == nil {
		t.Fatal("a selfdev release in current must still be drift")
	}
}
