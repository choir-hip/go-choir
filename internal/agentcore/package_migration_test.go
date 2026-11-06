package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// The forced-modularity boot migration
// (docs/design/forced-modularity-build-2026-11-04.md, "Build notes: M3 boot
// migration"). The failure modes PM1-PM12 were written there first; each test
// names the ones it pins.

const (
	pmComputer = "computer-pm"
	pmOwner    = "owner-pm"
)

// pmManifest is a valid v2 manifest of package name.
func pmManifest(name string) string {
	return `{
  "schema": "choir.package/v2",
  "name": "` + name + `",
  "version": "1.0.0",
  "display": {"title": "` + name + `", "icon": "P", "description": "Migrated"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true, "desktopIcon": false, "order": 1}, "window": {"singleton": false, "heavy": false}, "auth": {"preview": "private"}, "theme": {"surface": "standard", "shellDataAttr": "data-x-window", "contentClass": "x-content"}},
    "kinds": [],
  "uses_kinds": [],
  "contributes": {"desks": []},
  "capabilities": []
}
`
}

// pmPackageFiles is one package's files under its root.
func pmPackageFiles(name, manifest, app string) map[string]string {
	root := pkgrelease.Root(name)
	return map[string]string{
		root + "/choir-package.json":  manifest,
		root + "/App.svelte":          app,
		root + "/choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": "[data-app]"}` + "\n",
	}
}

func pmMerge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// pmBuilder is the fake module builder, failing transiently for one package.
type pmBuilder struct {
	fakeModuleBuilder
	transient string
}

func (b *pmBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	if rec.Package.Name == b.transient {
		return pkgruntime.Module{}, errors.New("the module store's disk is gone")
	}
	return b.fakeModuleBuilder.Build(ctx, rec, treePatch)
}

type pmNoService struct{ updater.ServiceManager }
type pmNoHealth struct{ updater.HealthProber }
type pmNoSigner struct{ updater.ReceiptSigner }

type pmEnv struct {
	rt          *Runtime
	repo        string
	base        string
	booted      string
	updaterRoot string
	baseline    string
	builder     *pmBuilder
	preRelease  string
}

// newPMEnv is a computer whose applied stack (one operation on base) holds
// files; the booted base is booted ("" means base). The real updater engine
// serves the baseline import over its socket.
func newPMEnv(t *testing.T, files map[string]string, bootedFiles map[string]string) *pmEnv {
	t.Helper()
	ctx := context.Background()
	rt, updaterRoot, repo, base := projectionRuntime(t, pmComputer)
	// Staged releases are read-only; let the temp dir go.
	t.Cleanup(func() {
		_ = filepath.Walk(updaterRoot, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	e := &pmEnv{rt: rt, repo: repo, base: base, booted: base, updaterRoot: updaterRoot}
	if bootedFiles != nil {
		e.booted = commitOnTop(t, repo, base, bootedFiles)
		setBootedBase(t, rt, e.booted)
	}
	rt.selfdevRouteOwnerID = pmOwner
	rt.selfdevRealizationID = "realization-pm"
	e.builder = &pmBuilder{}
	rt.packageBuilder = e.builder
	rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}

	// The booted image's release root: trusted under the updater's incoming.
	e.baseline = filepath.Join(updaterRoot, "incoming", "baseline-root")
	if err := os.MkdirAll(filepath.Join(e.baseline, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.baseline, "frontend", "index.html"), []byte("<html>booted base</html>"), 0o444); err != nil {
		t.Fatal(err)
	}
	rt.packageMigrationBaseline = e.baseline
	engine, err := updater.New(updaterRoot, pmComputer, "realization-pm", pmNoService{}, pmNoHealth{}, pmNoSigner{})
	if err != nil {
		t.Fatal(err)
	}
	sockDir, err := os.MkdirTemp("", "pm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	socket := filepath.Join(sockDir, "u.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/import-baseline", func(w http.ResponseWriter, r *http.Request) {
		var request updater.BaselineImportRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		manifest, err := engine.ImportBaseline(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(manifest)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	client, err := updater.NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevUpdater = client

	patch, _ := releasePatch(t, repo, files)
	e.preRelease = applyOperation(t, rt, updaterRoot, pmComputer, "op-stack", base, patch)
	result, err := rt.ReconcileSourceProjection(ctx, "boot")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != sourceproj.StatePackagePath {
		t.Fatalf("setup: the stack must report stack_package_path, got %+v", result)
	}
	return e
}

func (e *pmEnv) migrate(t *testing.T) *packageMigrationRecord {
	t.Helper()
	rec, err := e.rt.migratePackageStack(context.Background(), "boot")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return rec
}

// crashAt runs an attempt that dies at the n-th visit of step (a panic is the
// process dying: nothing after it runs), then makes the runtime a new
// process.
func (e *pmEnv) crashAt(t *testing.T, step string, n int) {
	t.Helper()
	seen := 0
	e.rt.packageMigrationAt = func(s string) {
		if s == step {
			seen++
			if seen == n {
				panic("crash")
			}
		}
	}
	func() {
		defer func() {
			if r := recover(); r != "crash" {
				t.Fatalf("the attempt did not reach %s visit %d (recover %v)", step, n, r)
			}
		}()
		_, _ = e.rt.migratePackageStack(context.Background(), "boot")
	}()
	e.rt.packageMigrationAt = nil
	e.newProcess(false)
}

// newProcess makes the runtime a new process, booted after a planned
// restart or not.
func (e *pmEnv) newProcess(planned bool) {
	e.rt.packageMigrationClosed = false
	e.rt.packageMigrationTried = nil
	e.rt.packageMigrationLast = nil
	e.rt.bootRestartPlanned = planned
}

func (e *pmEnv) lastAttempt(t *testing.T, rec *packageMigrationRecord) packageMigrationAttempt {
	t.Helper()
	if rec == nil || len(rec.Attempts) == 0 {
		t.Fatal("no attempt recorded")
	}
	return rec.Attempts[len(rec.Attempts)-1]
}

func (e *pmEnv) record(t *testing.T) *packageMigrationRecord {
	t.Helper()
	rec, err := e.rt.loadPackageMigration(packageMigrationPair("op-stack", e.booted))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func (e *pmEnv) currentDigest(t *testing.T) string {
	t.Helper()
	m, err := readCurrentManifestIdentity(e.updaterRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		t.Fatal(err)
	}
	return m.ContentDigest
}

func (e *pmEnv) baselineManifest(t *testing.T) updater.ReleaseManifest {
	t.Helper()
	m, err := e.rt.servingBaselineManifest(context.Background(), pmComputer, e.baseline)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (e *pmEnv) baselineDigest(t *testing.T) string {
	t.Helper()
	m, err := e.rt.servingBaselineManifest(context.Background(), pmComputer, e.baseline)
	if err != nil {
		t.Fatal(err)
	}
	return m.ContentDigest
}

func (e *pmEnv) inForce(t *testing.T) map[string]*store.PackageInstall {
	t.Helper()
	list, err := e.rt.packageInstalls(context.Background(), pmOwner, pmComputer)
	if err != nil {
		t.Fatal(err)
	}
	return currentPackageInstalls(list)
}

func (e *pmEnv) ledger(t *testing.T) []types.CommitmentRecord {
	t.Helper()
	all, err := e.rt.store.ListCommitmentRecords(context.Background(), pmOwner, pmComputer, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, r := range all {
		if r.Provenance.AgentID == packageMigrationAgentID {
			out = append(out, r)
		}
	}
	return out
}

// PM3, PM12, D1-D4: a package-only stack becomes installs (one package the
// booted base already carries is upstreamed), the served build is the booted
// base, the projection is no longer diverged, the whole stack is kept at a
// ref, the owner sees it on /health and the ledger, and a re-run does nothing.
func TestPackageMigrationMovesAPackageOnlyStackIntoInstallsAndServesTheBootedBase(t *testing.T) {
	same := pmPackageFiles("same-app", pmManifest("same-app"), "<p>same</p>\n")
	hello := pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hello</p>\n")
	e := newPMEnv(t, pmMerge(same, hello), same)
	ctx := context.Background()

	rec := e.migrate(t)
	a := e.lastAttempt(t, rec)
	if a.Outcome != pmDone || a.Differential == nil || !a.Differential.ok() {
		t.Fatalf("attempt = %+v", a)
	}
	outcomes := map[string]string{}
	for _, p := range a.Packages {
		outcomes[p.Name] = p.Outcome
	}
	if outcomes["same-app"] != pmUpstreamed || outcomes["hello-app"] != pmInstalled {
		t.Fatalf("package outcomes = %v", outcomes)
	}
	installs := e.inForce(t)
	if item := installs["hello-app"]; item == nil || item.Trigger != store.PackageInstallTriggerMigration || item.PublisherTrack != pmOwner || !e.rt.installBytesPresent(*item) {
		t.Fatalf("hello-app install = %+v", item)
	}
	if installs["same-app"] != nil {
		t.Fatal("an upstreamed package must not be installed")
	}
	if got, want := e.currentDigest(t), e.baselineDigest(t); got != want || a.ServedReleaseDigest != want {
		t.Fatalf("served %s (attempt says %s), want the booted baseline %s", got, a.ServedReleaseDigest, want)
	}
	served, err := os.ReadFile(filepath.Join(e.updaterRoot, "current", "frontend", "index.html"))
	if err != nil || string(served) != "<html>booted base</html>" {
		t.Fatalf("served index = %q, %v", served, err)
	}
	status := e.rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateBase || status.Diverged || gitHead(t, e.repo) != e.booted {
		t.Fatalf("projection after = %+v head %s", status, gitHead(t, e.repo))
	}
	if kept := releaseGit(t, e.repo, "rev-parse", a.KeptRef); kept != a.KeptCommit || a.PreReleaseDigest != e.preRelease {
		t.Fatalf("snapshot: kept %s vs %s, pre %s vs %s", kept, a.KeptCommit, a.PreReleaseDigest, e.preRelease)
	}
	if _, _, err := updater.ReadPinnedManifest(e.updaterRoot, e.preRelease); err != nil {
		t.Fatalf("the pre-migration release must stay pinned: %v", err)
	}
	identity := e.rt.EffectiveRelease(ctx)
	if identity.PackageMigration == nil || identity.PackageMigration.Outcome != pmDone || identity.PackageMigration.OperationID != "op-stack" {
		t.Fatalf("/health package_migration = %+v", identity.PackageMigration)
	}
	if ledger := e.ledger(t); len(ledger) != 1 || ledger[0].Discrepancy != types.DiscrepancyConfirmed {
		t.Fatalf("ledger = %+v", ledger)
	}

	// PM3: a re-run (and a reboot) does nothing.
	e.rt.packageMigrationClosed = false
	if _, err := e.rt.ReconcileSourceProjection(ctx, "boot"); err != nil {
		t.Fatal(err)
	}
	again, err := e.rt.migratePackageStack(ctx, "boot")
	if err != nil || again != nil {
		t.Fatalf("re-run = %+v, %v", again, err)
	}
	if rec := e.record(t); len(rec.Attempts) != 1 || e.builder.builds != 1 {
		t.Fatalf("a re-run made an attempt or a build: %d attempts, %d builds", len(rec.Attempts), e.builder.builds)
	}
}

// PM4: d@e's case, a package whose manifest is the deleted v1 schema. It
// cannot be installed under today's contract; it is kept, named, with its
// bytes, and the computer is not held at stack_package_path.
func TestPackageMigrationKeepsAPackageItCannotInstall(t *testing.T) {
	v1 := `{"schema": "choir.app-package/v1", "name": "slides-next", "version": "1.0.0"}` + "\n"
	e := newPMEnv(t, pmPackageFiles("slides-next", v1, "<p>slides</p>\n"), nil)
	a := e.lastAttempt(t, e.migrate(t))
	if a.Outcome != pmDone || len(a.Packages) != 1 || a.Packages[0].Outcome != pmKept || a.Packages[0].Reason == "" {
		t.Fatalf("attempt = %+v", a)
	}
	if !strings.Contains(a.Reason, "set aside, not installed") || !strings.Contains(a.Reason, a.KeptRef) {
		t.Fatalf("the done reason must name the kept package and its ref: %s", a.Reason)
	}
	kept, err := os.ReadFile(filepath.Join(e.updaterRoot, a.Packages[0].PatchFile))
	if err != nil || !strings.Contains(string(kept), "choir.app-package/v1") {
		t.Fatalf("the kept patch = %q, %v", kept, err)
	}
	if show := releaseGit(t, e.repo, "show", a.KeptCommit+":frontend/src/packages/slides-next/App.svelte"); show != "<p>slides</p>" {
		t.Fatalf("the kept commit lost the source: %q", show)
	}
	if len(e.inForce(t)) != 0 {
		t.Fatal("a kept package must not be installed")
	}
	if e.currentDigest(t) != e.baselineDigest(t) {
		t.Fatal("the served build must be the booted base")
	}
}

// PM6: a stack that also changes platform paths is refused, nothing moved.
func TestPackageMigrationRefusesAMixedStackAndMovesNothing(t *testing.T) {
	e := newPMEnv(t, pmMerge(pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), map[string]string{"frontend/public/a.txt": "A\n"}), nil)
	a := e.lastAttempt(t, e.migrate(t))
	if a.Outcome != pmRefused || !strings.HasPrefix(a.Reason, packageMigrationReasonMixed) || !strings.Contains(a.Reason, "frontend/public/a.txt") {
		t.Fatalf("attempt = %+v", a)
	}
	if e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatal("a refused migration must move nothing")
	}
	// One attempt per pair: a second trigger does not retry.
	e.migrate(t)
	if rec := e.record(t); len(rec.Attempts) != 1 {
		t.Fatalf("attempts = %d", len(rec.Attempts))
	}
	if ledger := e.ledger(t); len(ledger) != 1 || ledger[0].Discrepancy != types.DiscrepancyContradicted {
		t.Fatalf("ledger = %+v", ledger)
	}
}

// PM5, PM20: a transient failure defers the attempt, undoes its own installs
// and swaps nothing; the pair stays open, retries once per planned boot, and
// closes after three deferrals.
func TestPackageMigrationTransientFailureDefersUndoesItsInstallsAndSwapsNothing(t *testing.T) {
	e := newPMEnv(t, pmMerge(pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), pmPackageFiles("zz-app", pmManifest("zz-app"), "<p>zz</p>\n")), nil)
	e.builder.transient = "zz-app"
	a := e.lastAttempt(t, e.migrate(t))
	if a.Outcome != pmDeferred || !strings.Contains(a.Reason, "zz-app") {
		t.Fatalf("attempt = %+v", a)
	}
	if len(e.inForce(t)) != 0 {
		t.Fatalf("the attempt's install must be undone: %+v", e.inForce(t))
	}
	if e.currentDigest(t) != e.preRelease {
		t.Fatal("a failed attempt must not swap the served build")
	}
	if status := e.rt.sourceProjectionStatus(); status == nil || status.State != sourceproj.StatePackagePath {
		t.Fatalf("the stack must still be there: %+v", status)
	}
	// The same boot's second trigger and a crash boot do not retry.
	if rec := e.migrate(t); len(rec.Attempts) != 1 {
		t.Fatalf("a second trigger in one boot retried: %d attempts", len(rec.Attempts))
	}
	e.newProcess(false)
	if rec := e.migrate(t); len(rec.Attempts) != 1 {
		t.Fatalf("a crash boot retried: %d attempts", len(rec.Attempts))
	}
	// Planned boots retry; the third deferral closes the pair.
	for n := 2; n <= packageMigrationMaxDeferred; n++ {
		e.newProcess(true)
		if rec := e.migrate(t); len(rec.Attempts) != n || e.lastAttempt(t, rec).Outcome != pmDeferred {
			t.Fatalf("planned retry %d = %+v", n, rec.Attempts)
		}
	}
	e.newProcess(true)
	rec := e.migrate(t)
	if ok, closed, why := packageMigrationMayStart(rec, true, false); ok || !closed || len(rec.Attempts) != packageMigrationMaxDeferred {
		t.Fatalf("three deferrals must close the pair: %v %v %s (%d attempts)", ok, closed, why, len(rec.Attempts))
	}
	// A transient cause that clears lets the pair finish (on a fresh record).
	e.builder.transient = ""
	if len(e.inForce(t)) != 0 {
		t.Fatal("deferred attempts must leave no install")
	}
}

// PM21: a wiring gap defers; it never closes the pair.
func TestPackageMigrationWiringGapDefers(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	builder := e.rt.packageBuilder
	e.rt.packageBuilder = nil
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDeferred {
		t.Fatalf("attempt = %+v", a)
	}
	e.rt.packageBuilder = builder
	e.newProcess(true)
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDone {
		t.Fatalf("the planned retry = %+v", a)
	}
}

// PM32 (owner ruling 2026-11-06): a parked operation, one waiting on a
// decision or still being built, changes neither the stack nor `current`, so
// it does not hold the attempt.
func TestPackageMigrationParkedOperationDoesNotBlock(t *testing.T) {
	for _, state := range []string{"awaiting_approval", "requested", "executing", "frozen", "verified"} {
		t.Run(state, func(t *testing.T) {
			e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
			insertReleaseOperation(t, e.rt.store, releaseOpRow{operationID: "op-parked", computerID: pmComputer, state: state, bundleDigest: strings.Repeat("1", 64)})
			rec := e.migrate(t)
			if a := e.lastAttempt(t, rec); a.Outcome != pmDone {
				t.Fatalf("a parked %s operation must not hold the attempt: %+v", state, a)
			}
			if view := e.rt.packageMigrationStatusView(); view == nil || view.Waiting != nil || view.Outcome != pmDone {
				t.Fatalf("health after a done attempt = %+v", view)
			}
		})
	}
}

// PM7 narrowed: an operation that is changing the stack or `current` still
// holds the attempt (not counted), and /health says why.
func TestPackageMigrationStackChangingOperationBlocksAndHealthShowsWhy(t *testing.T) {
	for _, state := range []string{"accepted", "materializing", "rollback_pending", "degraded"} {
		t.Run(state, func(t *testing.T) {
			e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
			insertReleaseOperation(t, e.rt.store, releaseOpRow{operationID: "op-open", computerID: pmComputer, state: state, bundleDigest: strings.Repeat("1", 64)})
			if _, err := e.rt.migratePackageStack(context.Background(), "boot"); err == nil || !strings.Contains(err.Error(), "op-open") {
				t.Fatalf("a %s operation must hold the attempt: %v", state, err)
			}
			if rec := e.record(t); rec != nil {
				t.Fatalf("a held attempt must not be recorded: %+v", rec)
			}
			view := e.rt.packageMigrationStatusView()
			if view == nil || view.Waiting == nil || !strings.Contains(view.Waiting.Reason, "op-open") || view.Waiting.Since == "" {
				t.Fatalf("health must show the waiting reason: %+v", view)
			}
		})
	}
}

// PM33: the base objects missing is a visible wait, and the wait clears when
// the attempt runs.
func TestPackageMigrationBaseObjectsMissingShowsAWaitingReason(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	setBootedBase(t, e.rt, strings.Repeat("a", 40))
	e.rt.sourceProjectionMu.Lock()
	e.rt.sourceProjectionLast.BootedBase = strings.Repeat("a", 40) // the projection saw a base whose objects are not here
	e.rt.sourceProjectionMu.Unlock()
	if _, err := e.rt.migratePackageStack(context.Background(), "boot"); err != nil {
		t.Fatal(err)
	}
	view := e.rt.packageMigrationStatusView()
	if view == nil || view.Waiting == nil || !strings.Contains(view.Waiting.Reason, "base") {
		t.Fatalf("health must show why no attempt ran: %+v", view)
	}
	if rec := e.record(t); rec != nil {
		t.Fatalf("a waiting reason is not an attempt: %+v", rec)
	}
}

// PM1: a crash at each step before the swap leaves the old stack served; the
// next boot closes the attempt "interrupted by a restart"; a crash boot does
// not start another; a planned boot does, and its installs are the same
// release (no second install record); two interruptions close the pair.
func TestPackageMigrationCrashBeforeTheSwapIsClosedAndOnlyAPlannedBootRetries(t *testing.T) {
	for _, tc := range []struct {
		step string
		n    int
	}{{"classify", 1}, {"snapshot", 1}, {"packages", 1}, {"packages", 2}, {"swap", 1}} {
		t.Run(tc.step+"-"+string(rune('0'+tc.n)), func(t *testing.T) {
			e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
			ctx := context.Background()
			e.crashAt(t, tc.step, tc.n)
			if e.currentDigest(t) != e.preRelease {
				t.Fatal("a crash before the swap must leave the old release served")
			}
			installedBefore := len(e.inForce(t))

			// A crash boot: the attempt is closed, no new one starts.
			rec, err := e.rt.migratePackageStack(ctx, "boot")
			if err != nil {
				t.Fatal(err)
			}
			if len(rec.Attempts) != 1 || rec.Attempts[0].Outcome != pmInterrupted || !strings.Contains(rec.Attempts[0].Reason, "interrupted by a restart at step "+tc.step) {
				t.Fatalf("after a crash boot = %+v", rec.Attempts)
			}
			if ledger := e.ledger(t); len(ledger) != 1 || ledger[0].Discrepancy != types.DiscrepancyUnresolved {
				t.Fatalf("the interruption must be on the ledger: %+v", ledger)
			}

			// A planned boot: a fresh attempt finishes; installs are idempotent.
			e.newProcess(true)
			rec = e.migrate(t)
			if a := e.lastAttempt(t, rec); a.Outcome != pmDone || len(rec.Attempts) != 2 {
				t.Fatalf("the planned retry = %+v", rec.Attempts)
			}
			list, err := e.rt.packageInstalls(ctx, pmOwner, pmComputer)
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 {
				t.Fatalf("install records = %d (installed before the crash: %d), want 1", len(list), installedBefore)
			}
			if e.currentDigest(t) != e.baselineDigest(t) {
				t.Fatal("the retry must serve the booted base")
			}
		})
	}
}

// PM1 bound: two interruptions close the pair even on planned boots.
func TestPackageMigrationTwoInterruptionsCloseThePair(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	e.crashAt(t, "packages", 1)
	e.newProcess(true)
	e.crashAt(t, "packages", 1)
	e.newProcess(true)
	rec := e.migrate(t)
	if len(rec.Attempts) != 2 || rec.Attempts[1].Outcome != pmInterrupted {
		t.Fatalf("attempts = %+v", rec.Attempts)
	}
	if ok, closed, why := packageMigrationMayStart(rec, true, false); ok || !closed || !strings.Contains(why, "closed") {
		t.Fatalf("the pair must be closed: %v %s", ok, why)
	}
}

// PM2, PM13: stage then swap. A crash after the baseline is staged and
// before the swap leaves the old release served (current is never absent),
// and a planned retry reuses the staged release. A crash after the swap
// leaves the booted base served; the projection reads an empty stack, so
// nothing starts; the attempt is closed at its step, and the owner may roll
// it back (PM18).
func TestPackageMigrationStageThenSwapNeverRemovesCurrent(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	staged := false
	e.rt.packageMigrationAt = func(step string) {
		if step == "swap" {
			if err := updater.StageBaselineRelease(e.updaterRoot, e.baseline, e.baselineManifest(t)); err != nil {
				t.Fatal(err)
			}
			staged = true
			panic("crash")
		}
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = e.rt.migratePackageStack(ctx, "boot")
	}()
	e.rt.packageMigrationAt = nil
	if !staged || e.currentDigest(t) != e.preRelease {
		t.Fatalf("a crash between stage and swap must leave the old release served (current %s)", e.currentDigest(t))
	}
	e.newProcess(true)
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDone || e.currentDigest(t) != e.baselineDigest(t) {
		t.Fatalf("the planned retry = %+v", a)
	}
}

func TestPackageMigrationCrashAfterTheSwapIsClosedAndRolledBack(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	e.crashAt(t, "differential", 1)
	if e.currentDigest(t) != e.baselineDigest(t) {
		t.Fatal("setup: the swap must have happened")
	}
	if _, err := e.rt.ReconcileSourceProjection(ctx, "boot"); err != nil {
		t.Fatal(err)
	}
	if rec, err := e.rt.migratePackageStack(ctx, "boot"); err != nil || rec != nil {
		t.Fatalf("no attempt may start on an empty stack: %+v %v", rec, err)
	}
	rec := e.record(t)
	a := e.lastAttempt(t, rec)
	if len(rec.Attempts) != 1 || a.Outcome != pmInterrupted || a.Step != "differential" || a.TargetReleaseDigest != e.baselineDigest(t) {
		t.Fatalf("the cut attempt = %+v", rec.Attempts)
	}
	if e.inForce(t)["hello-app"] == nil {
		t.Fatal("the install made before the crash stands")
	}
	// S4: the rollback takes an attempt a restart cut after the swap.
	if _, err := e.rt.rollbackPackageMigration(ctx, rec.Pair); err != nil {
		t.Fatal(err)
	}
	if e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatal("the rollback must restore the snapshot")
	}
}

// PM8: a differential that fails after the swap restores the snapshot.
func TestPackageMigrationFailedDifferentialRestoresTheSnapshot(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	e.rt.packageMigrationAt = func(step string) {
		if step == "differential" {
			// current moves off the booted baseline: D2 reads it and fails.
			// (A module that does not read defers: PM31.)
			if err := updater.RestagePinnedRelease(e.updaterRoot, e.preRelease); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := e.lastAttempt(t, e.migrate(t))
	if a.Outcome != pmFailed || !strings.Contains(a.Reason, "D2") {
		t.Fatalf("attempt = %+v", a)
	}
	if e.currentDigest(t) != e.preRelease {
		t.Fatal("the snapshot's release must be served again")
	}
	if len(e.inForce(t)) != 0 {
		t.Fatal("the migration's install must be undone")
	}
	if status := e.rt.sourceProjectionStatus(); status == nil || status.State != sourceproj.StatePackagePath {
		t.Fatalf("the stack is back: %+v", status)
	}
}

// PM9: the owner's rollback restores the snapshot; the pair is not migrated
// again.
func TestPackageMigrationRollbackRestoresTheSnapshot(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	rec := e.migrate(t)
	if e.lastAttempt(t, rec).Outcome != pmDone {
		t.Fatal("setup: the migration must finish")
	}
	rec, err := e.rt.rollbackPackageMigration(ctx, rec.Pair)
	if err != nil {
		t.Fatal(err)
	}
	if a := e.lastAttempt(t, rec); a.Outcome != pmRolledBack {
		t.Fatalf("attempt = %+v", a)
	}
	if e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatal("the rollback must serve the old release and uninstall the migration's install")
	}
	if status := e.rt.sourceProjectionStatus(); status == nil || status.State != sourceproj.StatePackagePath {
		t.Fatalf("the stack is back: %+v", status)
	}
	if again := e.migrate(t); len(again.Attempts) != 1 {
		t.Fatalf("a rolled-back pair must not be migrated again: %+v", again.Attempts)
	}
	if status := e.rt.packageMigrationStatusView(); status == nil || !status.Held {
		t.Fatalf("/health must show the hold: %+v", status)
	}
	// PM15: a new booted base is a new pair, and the hold still holds.
	newBase := commitOnTop(t, e.repo, e.base, map[string]string{"frontend/public/next.txt": "next\n"})
	setBootedBase(t, e.rt, newBase)
	e.newProcess(true)
	if _, err := e.rt.ReconcileSourceProjection(ctx, "boot"); err != nil {
		t.Fatal(err)
	}
	if got, err := e.rt.migratePackageStack(ctx, "boot"); err != nil || got != nil {
		t.Fatalf("a held operation must not be migrated on a new base: %+v %v", got, err)
	}
	if next, _ := e.rt.loadPackageMigration(packageMigrationPair("op-stack", newBase)); next != nil {
		t.Fatalf("a held operation made an attempt: %+v", next)
	}
	// PM14: the platform transition waits while the stack holds package source.
	if refusal := e.rt.unmigratedPackageStackRefusal(ctx); !strings.HasPrefix(refusal, packageMigrationReasonUnmigrated) {
		t.Fatalf("platform refusal = %q", refusal)
	}
	// The owner rearms: the next boot migrates on the new base.
	if err := e.rt.rearmPackageMigration("op-stack"); err != nil {
		t.Fatal(err)
	}
	e.newProcess(true)
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDone {
		t.Fatalf("the rearmed migration = %+v", a)
	}
	if refusal := e.rt.unmigratedPackageStackRefusal(ctx); refusal != "" {
		t.Fatalf("a migrated computer refuses a platform transition: %s", refusal)
	}
}

// S3: a crash inside the install (the record written, the attempt not yet
// told) leaves an install a retry and a rollback still find.
func TestPackageMigrationCrashInsideTheInstallThenRetryThenRollback(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	e.crashAt(t, "installed:hello-app", 1)
	if e.inForce(t)["hello-app"] == nil {
		t.Fatal("setup: the install record must be written")
	}
	if rec := e.record(t); len(rec.Intents) != 1 || len(rec.Attempts[0].Packages) != 0 {
		t.Fatalf("the intent must be durable and the attempt not told: %+v", rec)
	}
	e.newProcess(true)
	rec := e.migrate(t)
	if a := e.lastAttempt(t, rec); a.Outcome != pmDone || len(rec.Attempts) != 2 {
		t.Fatalf("the retry = %+v", rec.Attempts)
	}
	if _, err := e.rt.rollbackPackageMigration(ctx, rec.Pair); err != nil {
		t.Fatal(err)
	}
	if len(e.inForce(t)) != 0 || e.currentDigest(t) != e.preRelease {
		t.Fatalf("the rollback after a retry must undo the install: %+v", e.inForce(t))
	}
}

// S3: a pair two crashes close before any swap undoes its installs once.
func TestPackageMigrationClosedPairUndoesItsInstalls(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	e.crashAt(t, "installed:hello-app", 1)
	e.newProcess(true)
	e.crashAt(t, "swap", 1)
	if e.inForce(t)["hello-app"] == nil {
		t.Fatal("setup: the install stands")
	}
	e.newProcess(true)
	rec := e.migrate(t)
	if !rec.ClosedUndone || len(e.inForce(t)) != 0 || e.currentDigest(t) != e.preRelease {
		t.Fatalf("the closed pair must undo its installs: %+v %+v", rec, e.inForce(t))
	}
}

// PM16: the rollback refuses a computer that moved since the migration.
func TestPackageMigrationRollbackRefusesADriftedComputer(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	rec := e.migrate(t)
	// Something else moved current (here, back to the pre-migration release).
	if err := updater.RestagePinnedRelease(e.updaterRoot, e.preRelease); err != nil {
		t.Fatal(err)
	}
	h := NewAPIHandler(e.rt)
	path := "/api/computers/" + pmComputer + "/self-development/package-migration-rollback"
	w := releaseRouterCall(t, h, http.MethodPost, path, pmOwner, pmComputer, []byte(`{"pair":"`+rec.Pair+`"}`))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), packageMigrationReasonDrifted) {
		t.Fatalf("a drifted rollback = %d %s", w.Code, w.Body.String())
	}
	if e.inForce(t)["hello-app"] == nil || e.rt.packageMigrationHeld("op-stack") {
		t.Fatal("a refused rollback must change nothing")
	}
}

// PM20: a release that needs an install review is kept for review at boot;
// no seat runs. PM22: it is on the Library's set-aside list, and the owner's
// install from there runs the review gate.
func TestPackageMigrationKeepsAReviewedPackageForReview(t *testing.T) {
	widened := strings.Replace(pmManifest("notes-app"), `"capabilities": []`, `"capabilities": [{"grant":"ui.read.kind","args":["notes"]}]`, 1)
	e := newPMEnv(t, pmPackageFiles("notes-app", widened, "<p>n</p>\n"), nil)
	rec := e.migrate(t)
	a := e.lastAttempt(t, rec)
	if a.Outcome != pmDone || len(a.Packages) != 1 || a.Packages[0].Outcome != pmKept || !strings.HasPrefix(a.Packages[0].Reason, packageMigrationKeptForReview) {
		t.Fatalf("attempt = %+v", a)
	}
	h := NewAPIHandler(e.rt)
	w := releaseRouterCall(t, h, http.MethodPost, "/api/computers/"+pmComputer+"/self-development/package-migration-install", pmOwner, pmComputer, []byte(`{"pair":"`+rec.Pair+`","package":"notes-app"}`))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "install_review_unavailable") {
		t.Fatalf("the owner's install must meet the review gate: %d %s", w.Code, w.Body.String())
	}
}

// PM23, PM22: a built-in app is kept (builtin_shadow), listed as set aside on
// GET installs, and the owner installs it from there; it then leaves the list.
func TestPackageMigrationKeepsABuiltinAndTheOwnerInstallsIt(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("calendar", pmManifest("calendar"), "<p>cal</p>\n"), nil)
	rec := e.migrate(t)
	a := e.lastAttempt(t, rec)
	if a.Outcome != pmDone || len(a.Packages) != 1 || !strings.HasPrefix(a.Packages[0].Reason, packageMigrationKeptBuiltin) {
		t.Fatalf("attempt = %+v", a)
	}
	if len(e.inForce(t)) != 0 {
		t.Fatal("the boot migration must not replace a built-in app")
	}
	h := NewAPIHandler(e.rt)
	installs := "/api/computers/" + pmComputer + "/packages/installs"
	setAside := func() []packageMigrationSetAside {
		w := releaseRouterCall(t, h, http.MethodGet, installs, pmOwner, pmComputer, nil)
		var body struct {
			SetAside []packageMigrationSetAside `json:"set_aside"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil {
			t.Fatalf("GET installs = %d %s", w.Code, w.Body.String())
		}
		return body.SetAside
	}
	if list := setAside(); len(list) != 1 || list[0].Package != "calendar" || list[0].Pair != rec.Pair || !list[0].Installable {
		t.Fatalf("set_aside = %+v", list)
	}
	route := "/api/computers/" + pmComputer + "/self-development/package-migration-install"
	if w := releaseRouterCall(t, h, http.MethodPost, route, "owner-other", pmComputer, []byte(`{"pair":"`+rec.Pair+`","package":"calendar"}`)); w.Code != http.StatusForbidden {
		t.Fatalf("another owner = %d %s", w.Code, w.Body.String())
	}
	w := releaseRouterCall(t, h, http.MethodPost, route, pmOwner, pmComputer, []byte(`{"pair":"`+rec.Pair+`","package":"calendar"}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("the owner's install = %d %s", w.Code, w.Body.String())
	}
	if item := e.inForce(t)["calendar"]; item == nil || item.Trigger != "" || item.PublisherTrack != pmOwner {
		t.Fatalf("the owner's install = %+v", item)
	}
	if list := setAside(); len(list) != 0 {
		t.Fatalf("an installed package must leave the list: %+v", list)
	}
}

// PM9 through the owner's route: only the owner, only a named pair, only a
// done attempt.
func TestPackageMigrationRollbackRouteIsTheOwnersAct(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	rec := e.migrate(t)
	h := NewAPIHandler(e.rt)
	path := "/api/computers/" + pmComputer + "/self-development/package-migration-rollback"
	body := []byte(`{"pair":"` + rec.Pair + `"}`)
	if w := releaseRouterCall(t, h, http.MethodPost, path, "owner-other", pmComputer, body); w.Code != http.StatusForbidden {
		t.Fatalf("another owner = %d %s", w.Code, w.Body.String())
	}
	if w := releaseRouterCall(t, h, http.MethodPost, path, pmOwner, pmComputer, []byte(`{"pair":"nope"}`)); w.Code != http.StatusBadRequest {
		t.Fatalf("a malformed pair = %d %s", w.Code, w.Body.String())
	}
	if w := releaseRouterCall(t, h, http.MethodPost, path, pmOwner, pmComputer, []byte(`{"pair":"`+strings.Repeat("a", 32)+`"}`)); w.Code != http.StatusNotFound {
		t.Fatalf("an unknown pair = %d %s", w.Code, w.Body.String())
	}
	w := releaseRouterCall(t, h, http.MethodPost, path, pmOwner, pmComputer, body)
	if w.Code != http.StatusOK || e.currentDigest(t) != e.preRelease {
		t.Fatalf("the owner's rollback = %d %s", w.Code, w.Body.String())
	}
	if w := releaseRouterCall(t, h, http.MethodPost, path, pmOwner, pmComputer, body); w.Code != http.StatusConflict {
		t.Fatalf("a second rollback = %d %s", w.Code, w.Body.String())
	}
	if status := e.rt.EffectiveRelease(context.Background()).PackageMigration; status == nil || status.Outcome != pmRolledBack {
		t.Fatalf("/health after the rollback = %+v", status)
	}
}

// A source-patch install record from before runtime install is history: it
// reads as replaced and is never in force.
func TestLegacySourceInstallRecordReadsAsReplaced(t *testing.T) {
	rt, _ := testRuntime(t)
	legacy := store.PackageInstall{InstallID: "legacy-1", Package: "hello-app", Kind: installKindPackage, Action: packageActionInstall, TrajectoryID: "trajectory-1"}
	view := rt.installView(legacy, []store.PackageInstall{legacy})
	if view.State != installStateReplaced || view.PatchMatchesRelease {
		t.Fatalf("legacy view = %+v", view)
	}
	if len(currentPackageInstalls([]store.PackageInstall{legacy})) != 0 {
		t.Fatal("a legacy record must not fold as an install in force")
	}
}

// PM25: a transient record write at the packages step defers the attempt;
// the pair stays open and the next planned boot finishes.
func TestPackageMigrationTransientRecordWriteDefersAndRetries(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	fired := false
	e.rt.packageMigrationWriteFault = func(rec *packageMigrationRecord) error {
		if a := rec.Attempts[len(rec.Attempts)-1]; !fired && a.Step == "packages" && len(a.Packages) == 1 {
			fired = true
			return errors.New("too many open files")
		}
		return nil
	}
	rec := e.migrate(t)
	a := e.lastAttempt(t, rec)
	if !fired || a.Outcome != pmDeferred || !strings.Contains(a.Reason, "too many open files") {
		t.Fatalf("attempt = %+v", a)
	}
	if ok, closed, _ := packageMigrationMayStart(rec, true, false); !ok || closed {
		t.Fatal("a transient write must leave the pair open")
	}
	if len(e.inForce(t)) != 0 || e.currentDigest(t) != e.preRelease {
		t.Fatal("a deferred attempt moves nothing")
	}
	e.newProcess(true)
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDone {
		t.Fatalf("the planned retry = %+v", a)
	}
}

// PM25: a transient record write after the swap restores and defers.
func TestPackageMigrationTransientWriteAfterTheSwapRestoresAndDefers(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	fired := false
	e.rt.packageMigrationWriteFault = func(rec *packageMigrationRecord) error {
		if a := rec.Attempts[len(rec.Attempts)-1]; !fired && a.Step == "differential" {
			fired = true
			return errors.New("no space left on device")
		}
		return nil
	}
	a := e.lastAttempt(t, e.migrate(t))
	if a.Outcome != pmDeferred || e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatalf("attempt = %+v (current %s)", a, e.currentDigest(t))
	}
}

// PM25: a failed pair's kept packages stay on the set-aside list.
func TestPackageMigrationFailedPairKeepsItsSetAsideList(t *testing.T) {
	e := newPMEnv(t, pmMerge(pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), pmPackageFiles("calendar", pmManifest("calendar"), "<p>c</p>\n")), nil)
	// A comparison that reads and fails ends failed: current moves off the
	// booted baseline before the differential reads it.
	e.rt.packageMigrationAt = func(step string) {
		if step == "differential" {
			if err := updater.RestagePinnedRelease(e.updaterRoot, e.preRelease); err != nil {
				t.Fatal(err)
			}
		}
	}
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmFailed {
		t.Fatalf("attempt = %+v", a)
	}
	if list := e.rt.packageMigrationSetAsideList(); len(list) != 1 || list[0].Package != "calendar" {
		t.Fatalf("set_aside = %+v", list)
	}
}

// PM28: the rollback of an attempt cut at the swap before it landed holds
// the operation and ends rolled_back.
func TestPackageMigrationRollbackOfAnUnlandedSwapHolds(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	e.crashAt(t, "swap", 1)
	rec, err := e.rt.migratePackageStack(ctx, "boot") // closes the cut attempt
	if err != nil || e.lastAttempt(t, rec).Outcome != pmInterrupted {
		t.Fatalf("setup: %+v %v", rec, err)
	}
	rec, err = e.rt.rollbackPackageMigration(ctx, rec.Pair)
	if err != nil {
		t.Fatal(err)
	}
	if e.lastAttempt(t, rec).Outcome != pmRolledBack || !e.rt.packageMigrationHeld("op-stack") || len(e.inForce(t)) != 0 || e.currentDigest(t) != e.preRelease {
		t.Fatalf("rollback = %+v", rec.Attempts)
	}
	e.newProcess(true)
	if again := e.migrate(t); len(again.Attempts) != 1 {
		t.Fatalf("a held operation retried: %+v", again.Attempts)
	}
}

// PM27: no attempt while a tape restore's restart is pending.
func TestPackageMigrationWaitsForARestoreRestart(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	e.rt.restoreRestaged.Store(true)
	if _, err := e.rt.migratePackageStack(context.Background(), "base_fetch"); err == nil || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("a pending restore restart must hold the attempt: %v", err)
	}
	if rec := e.record(t); rec != nil {
		t.Fatalf("no attempt may be recorded: %+v", rec)
	}
}

// PM31: a differential that cannot read what it checks defers; it never
// closes the pair (review r2). The planned retry reinstalls and finishes.
func TestPackageMigrationUnreadableDifferentialDefers(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	fired := false
	e.rt.packageMigrationAt = func(step string) {
		if step == "differential" && !fired {
			fired = true
			if err := os.RemoveAll(e.rt.packageModules.Dir); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := e.lastAttempt(t, e.migrate(t))
	if !fired || a.Outcome != pmDeferred || a.Differential == nil || !a.Differential.Inconclusive {
		t.Fatalf("attempt = %+v", a)
	}
	if e.currentDigest(t) != e.preRelease || len(e.inForce(t)) != 0 {
		t.Fatal("a deferred differential must restore the snapshot")
	}
	e.newProcess(true)
	if a := e.lastAttempt(t, e.migrate(t)); a.Outcome != pmDone {
		t.Fatalf("the planned retry = %+v", a)
	}
}
