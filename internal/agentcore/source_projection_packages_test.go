package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// S6 (docs/design/s6-package-lock-build-2026-11-04.md): the 2026-11-04 case
// through the runtime. An applied package-only change (a local library-next),
// then a deploy whose base carries a different library-next: an ordinary
// engineering Cast on another path is admitted, with a ledger warning that
// names the package conflict; /health shows the entry.
func TestPackageLockAdmitsAnOrdinaryCastWhenTheBaseTakesAnotherVersionOfThePackage(t *testing.T) {
	const computerID = "computer-package-lock"
	const owner = "owner-package-lock"
	const root = "frontend/src/packages/library-next"
	t.Setenv("CHOIR_OWNER_ID", owner)
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	ours := map[string]string{root + "/App.svelte": "<h1>our library</h1>\n", root + "/choir-package.json": "{\"name\":\"library-next\",\"launcher_order\":115}\n"}
	patch, _ := releasePatch(t, repo, ours)
	applyOperation(t, rt, updaterRoot, computerID, "op-library", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatal(err)
	}
	r := releaseGit(t, repo, "rev-parse", "HEAD:"+root)
	if err := rt.store.PutReleaseAdoption(ctx, store.ReleaseAdoption{OwnerID: owner, ComputerID: computerID, AdoptionID: "install-1", ReleaseID: "release-ours",
		TrajectoryID: "trajectory-install", Kind: "package", Package: "library-next", Action: "install", ReleaseTreeHash: r}); err != nil {
		t.Fatal(err)
	}

	exported := commitOnTop(t, repo, base, map[string]string{
		root + "/App.svelte": "<h1>platform library</h1>\n", root + "/Icon.svelte": "<svg/>\n",
		root + "/choir-package.json": "{\"name\":\"library-next\",\"launcher_order\":12}\n",
	})
	setBootedBase(t, rt, exported)
	cast := castLedgerRef{OwnerID: owner, ComputerID: computerID, CastRecordID: "cast-calendar", ManagementAgentID: persistentManagementAgentID(owner), TrajectoryID: "trajectory-calendar"}
	_, warning, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{}, cast)
	if err != nil || warning == nil {
		t.Fatalf("an ordinary Cast on another path must be admitted: %v %v", warning, err)
	}
	if h := warning.Prediction.Hypothesis; !strings.Contains(h, root) || !strings.Contains(h, "package_behind_base") || !strings.Contains(h, "also approves") {
		t.Fatalf("the warning must name the package conflict: %s", h)
	}
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateRematerialized || releaseGit(t, repo, "rev-parse", "HEAD:"+root) != r {
		t.Fatalf("status %+v", status)
	}
	identity := rt.EffectiveRelease(ctx)
	if len(identity.Packages) != 1 {
		t.Fatalf("/health effective_release.packages: %+v", identity.Packages)
	}
	if e := identity.Packages[0]; e.Root != root || e.State != sourceproj.PackageBehindBase || !e.Conflict || e.ReleaseID != "release-ours" || e.Tree != r {
		t.Fatalf("/health entry: %+v", e)
	}
	// A rebase Cast is not needed: the stack is not stranded.
	if _, _, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{Notice: "n", RebaseOf: "op-library", Base: exported}, cast); err == nil || !strings.Contains(err.Error(), "rebase_not_needed") {
		t.Fatalf("rebase: %v", err)
	}
}

// The Cast warning names every package_behind_base root, says which kind
// each is, and claims byte-identity only for the shell when a package entry
// is live (tier 2 review, finding 1).
func TestRematerializedGraftDetailNamesEveryPackageEntry(t *testing.T) {
	const conflict, merged, placed = "frontend/src/packages/a", "frontend/src/packages/b", "frontend/src/packages/c"
	detail := rematerializedGraftDetail(sourceproj.Result{Packages: []sourceproj.PackageEntry{
		{Root: conflict, State: sourceproj.PackageBehindBase, Conflict: true},
		{Root: merged, State: sourceproj.PackageBehindBase},
		{Root: placed, State: sourceproj.PackageBaseMoved},
		{Root: "frontend/src/packages/d", State: sourceproj.PackageUpstreamed},
	}})
	for _, want := range []string{conflict + " (package_behind_base, conflict)", "over the base's version", merged + " than this change did (package_behind_base, merged)", placed + " are unchanged"} {
		if !strings.Contains(detail, want) {
			t.Errorf("missing %q in %s", want, detail)
		}
	}
	if strings.Contains(detail, "packages/d") || strings.Contains(detail, "Elsewhere") || strings.Contains(detail, "changed no path the change touches, so the graft is byte-identical") {
		t.Errorf("false or extra clause: %s", detail)
	}
	if got := rematerializedGraftDetail(sourceproj.Result{Packages: []sourceproj.PackageEntry{{Root: "frontend/src/packages/d", State: sourceproj.PackageUpstreamed}}}); !strings.Contains(got, "byte-identical at those paths") {
		t.Errorf("no live entry: %s", got)
	}
}
