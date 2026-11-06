package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/packagepath"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// The forced-modularity boot migration (docs/design/forced-modularity-build-2026-11-04.md,
// section 4, "Build notes: M3 boot migration" and "Review r1"). A computer
// whose applied stack holds package source reports stack_package_path and
// refuses casts. This moves that source out: each package the stack changed
// becomes a runtime install on the owner's track (or is kept, named, when it
// cannot be installed under today's contract), and the served build becomes
// the booted base. Attempts are per (stack operation, booted base) pair, from
// the boot projection pass; never a resumer. The attempt record bounds
// attempts and is what /health, the ledger and the Library's set-aside list
// show.
//
// Locks: packageMigrationMu, then currentReleaseMu (every writer of
// `current` takes it), then packageInstallMu inside an install. An attempt
// holds the first two from the read of `current` through the differential
// (PM13).

const (
	packageMigrationSchema = "choir.package_migration.v1"
	packageMigrationDir    = "package-migration"
	// packageMigrationAgentID commits the migration's ledger record: a
	// system-committed observation, like the Cast warning.
	packageMigrationAgentID = "package-migration"
	// packageMigrationMaxInterrupted closes a pair whose attempts a restart
	// cut this many times (a migration that crashes the computer).
	packageMigrationMaxInterrupted = 2
	// packageMigrationMaxDeferred closes a pair whose attempts a transient
	// cause deferred this many times (PM20).
	packageMigrationMaxDeferred = 3
	packageMigrationTrigger     = "package_migration"
	// packageMigrationReasonMixed refuses a stack that also changes platform
	// paths: a platform-only stack needs a host build (named residual).
	packageMigrationReasonMixed = "package_migration_mixed"
	// packageMigrationReasonUnmigrated refuses a platform transition over a
	// stack that still holds package source (PM14).
	packageMigrationReasonUnmigrated = "package_stack_unmigrated"
	// packageMigrationReasonDrifted refuses a rollback whose computer moved
	// since the migration (PM16).
	packageMigrationReasonDrifted = "package_migration_drifted"
	// packageMigrationKeptBuiltin keeps a package that is a built-in app: the
	// boot migration never replaces one (PM23).
	packageMigrationKeptBuiltin = "builtin_shadow"
	// packageMigrationKeptForReview keeps a package whose release needs an
	// install review: no seat runs at boot (PM20).
	packageMigrationKeptForReview = "kept_for_review"
)

// Attempt outcomes.
const (
	pmRunning     = "running"
	pmDone        = "done"
	pmRefused     = "refused"
	pmFailed      = "failed"
	pmDeferred    = "deferred"
	pmInterrupted = "interrupted"
	pmRolledBack  = "rolled_back"
)

// Package outcomes.
const (
	pmInstalled  = "installed"
	pmUpstreamed = "upstreamed"
	pmKept       = "kept"
)

var (
	errPackageMigrationNotFound = errors.New("package_migration_not_found")
	errPackageMigrationState    = errors.New("package_migration_not_rolled_back")
	errPackageMigrationDrifted  = errors.New(packageMigrationReasonDrifted)
)

// packageMigrationRecord is one pair's attempts.
type packageMigrationRecord struct {
	Schema      string `json:"schema"`
	Pair        string `json:"pair"`
	OperationID string `json:"operation_id"`
	BootedBase  string `json:"booted_base"`
	StackBase   string `json:"stack_base"`
	PatchSHA256 string `json:"patch_sha256"`
	// Intents are the releases any attempt of the pair set out to install,
	// written before each install (PM17): an install a crash cut short is
	// still found and undone.
	Intents []packageMigrationIntent `json:"intents,omitempty"`
	// ClosedUndone: the pair closed before a swap and its intents were
	// undone (once).
	ClosedUndone bool                      `json:"closed_undone,omitempty"`
	Attempts     []packageMigrationAttempt `json:"attempts"`
}

type packageMigrationIntent struct {
	Name      string `json:"name"`
	ReleaseID string `json:"release_id"`
}

// packageMigrationAttempt is one attempt and its snapshot.
type packageMigrationAttempt struct {
	N           int    `json:"n"`
	Trigger     string `json:"trigger"`
	PlannedBoot bool   `json:"planned_boot,omitempty"`
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at,omitempty"`
	// Step is the last step reached: classify, snapshot, packages, swap,
	// differential, rollback.
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	// The snapshot: the release served before (pinned at releases/<digest>),
	// the kept projection commit of the whole stack, and the installs in
	// force (package to release id).
	PreReleaseDigest string                    `json:"pre_release_digest,omitempty"`
	PreReleaseMarker string                    `json:"pre_release_marker,omitempty"`
	KeptRef          string                    `json:"kept_ref,omitempty"`
	KeptCommit       string                    `json:"kept_commit,omitempty"`
	InstallsBefore   map[string]string         `json:"installs_before,omitempty"`
	Packages         []packageMigrationPackage `json:"packages,omitempty"`
	// TargetReleaseDigest is the booted baseline release, resolved once and
	// recorded before the swap (PM18).
	TargetReleaseDigest string `json:"target_release_digest,omitempty"`
	// ServedReleaseDigest is the release current names after the swap.
	ServedReleaseDigest string                        `json:"served_release_digest,omitempty"`
	Differential        *packageMigrationDifferential `json:"differential,omitempty"`
}

// packageMigrationPackage is what happened to one package root.
type packageMigrationPackage struct {
	Name      string `json:"name"`
	Root      string `json:"root"`
	Outcome   string `json:"outcome"`
	ReleaseID string `json:"release_id,omitempty"`
	InstallID string `json:"install_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	PatchFile string `json:"patch_file,omitempty"`
	// OwnerInstall is the release the owner installed from the set-aside
	// list (PM22); the package leaves the list.
	OwnerInstall string `json:"owner_install,omitempty"`
}

// packageMigrationDifferential is the check after the swap (design D1-D4).
type packageMigrationDifferential struct {
	Packages   bool `json:"packages"`
	Served     bool `json:"served"`
	Projection bool `json:"projection"`
	Installs   bool `json:"installs"`
	// Inconclusive: a check could not read what it compares (a store, the
	// module bytes, current, the projection). Unless a check that did read
	// found a mismatch, the attempt defers: a retry repairs a read.
	Inconclusive bool     `json:"inconclusive,omitempty"`
	Detail       []string `json:"detail,omitempty"`
	mismatch     bool
}

func (d packageMigrationDifferential) ok() bool {
	return d.Packages && d.Served && d.Projection && d.Installs
}

// unknown records a check that could not read what it compares.
func (d *packageMigrationDifferential) unknown(detail string) {
	d.Inconclusive = true
	d.Detail = append(d.Detail, detail)
}

// mismatched records a check that read and compared, and failed.
func (d *packageMigrationDifferential) mismatched(detail string) {
	d.mismatch = true
	d.Detail = append(d.Detail, detail)
}

// packageMigrationStatus is the /health view: the pair, its last attempt,
// and whether the owner's rollback holds the stack operation (PM15).
type packageMigrationStatus struct {
	Pair        string `json:"pair"`
	OperationID string `json:"operation_id"`
	BootedBase  string `json:"booted_base"`
	Held        bool   `json:"held,omitempty"`
	// Waiting is why no attempt is running now (PM33). It is in memory, per
	// process, recomputed each boot, and is never part of an attempt record.
	Waiting *packageMigrationWaiting `json:"waiting,omitempty"`
	packageMigrationAttempt
}

// packageMigrationWaiting is the visible deferral of an attempt that has not
// started: the reason, when this process first saw it, and the last pass.
type packageMigrationWaiting struct {
	Reason string `json:"reason"`
	Since  string `json:"since"`
	At     string `json:"at"`
}

func packageMigrationPair(operationID, booted string) string {
	return computerevent.DigestBytes([]byte("package-migration\x00" + strings.TrimSpace(operationID) + "\x00" + strings.TrimSpace(booted)))[:32]
}

func (rt *Runtime) packageMigrationRecordPath(pair string) string {
	return filepath.Join(rt.selfdevUpdaterRoot, packageMigrationDir, pair+".json")
}

// packageMigrationHoldPath is the owner's hold on one stack operation: the
// operation is never migrated again, on any booted base, until the owner
// rearms it (PM15).
func (rt *Runtime) packageMigrationHoldPath(operationID string) string {
	name := computerevent.DigestBytes([]byte("package-migration-hold\x00" + strings.TrimSpace(operationID)))[:32]
	return filepath.Join(rt.selfdevUpdaterRoot, packageMigrationDir, "hold-"+name+".json")
}

func (rt *Runtime) packageMigrationHeld(operationID string) bool {
	_, err := os.Stat(rt.packageMigrationHoldPath(operationID))
	return err == nil
}

func (rt *Runtime) loadPackageMigration(pair string) (*packageMigrationRecord, error) {
	raw, err := os.ReadFile(rt.packageMigrationRecordPath(pair))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec packageMigrationRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("package migration record %s: %w", pair, err)
	}
	if rec.Schema != packageMigrationSchema || rec.Pair != pair {
		return nil, fmt.Errorf("package migration record %s: schema or pair mismatch", pair)
	}
	return &rec, nil
}

// savePackageMigration writes the record by rename, so a crash leaves the
// old record or the new one, never a torn one.
func (rt *Runtime) savePackageMigration(rec *packageMigrationRecord) error {
	if rt.packageMigrationWriteFault != nil {
		if err := rt.packageMigrationWriteFault(rec); err != nil {
			return err // tests: a transient write fault
		}
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(rt.packageMigrationRecordPath(rec.Pair), append(raw, '\n')); err != nil {
		return err
	}
	rt.setPackageMigrationStatus(rec)
	return nil
}

func (rt *Runtime) setPackageMigrationStatus(rec *packageMigrationRecord) {
	if rec == nil || len(rec.Attempts) == 0 {
		return
	}
	status := &packageMigrationStatus{Pair: rec.Pair, OperationID: rec.OperationID, BootedBase: rec.BootedBase, Held: rt.packageMigrationHeld(rec.OperationID), packageMigrationAttempt: rec.Attempts[len(rec.Attempts)-1]}
	rt.packageMigrationStatusMu.Lock()
	rt.packageMigrationLast = status
	rt.packageMigrationStatusMu.Unlock()
}

// packageMigrationStatusView is what /health serves; nil before any attempt
// this process saw.
func (rt *Runtime) packageMigrationStatusView() *packageMigrationStatus {
	if rt == nil {
		return nil
	}
	rt.packageMigrationStatusMu.Lock()
	defer rt.packageMigrationStatusMu.Unlock()
	if rt.packageMigrationLast == nil {
		if rt.packageMigrationWait == nil {
			return nil
		}
		return &packageMigrationStatus{Waiting: rt.packageMigrationWait}
	}
	copied := *rt.packageMigrationLast
	copied.Waiting = rt.packageMigrationWait
	return &copied
}

// setPackageMigrationWaiting records why this pass ran no attempt (PM33). The
// reason is logged and shown on /health; it consumes no budget.
func (rt *Runtime) setPackageMigrationWaiting(reason string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rt.packageMigrationStatusMu.Lock()
	since := now
	if rt.packageMigrationWait != nil && rt.packageMigrationWait.Reason == reason {
		since = rt.packageMigrationWait.Since
	}
	rt.packageMigrationWait = &packageMigrationWaiting{Reason: reason, Since: since, At: now}
	rt.packageMigrationStatusMu.Unlock()
	log.Printf("package migration: waiting: %s", reason)
}

func (rt *Runtime) clearPackageMigrationWaiting() {
	rt.packageMigrationStatusMu.Lock()
	rt.packageMigrationWait = nil
	rt.packageMigrationStatusMu.Unlock()
}

func (a *packageMigrationAttempt) end(outcome, reason string) {
	a.Outcome, a.Reason, a.EndedAt = outcome, reason, time.Now().UTC().Format(time.RFC3339Nano)
}

// packageMigrationPairs lists the pairs with a record.
func (rt *Runtime) packageMigrationPairs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(rt.selfdevUpdaterRoot, packageMigrationDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pairs []string
	for _, entry := range entries {
		pair, ok := strings.CutSuffix(entry.Name(), ".json")
		if ok && !entry.IsDir() && packageMigrationPairShape(pair) {
			pairs = append(pairs, pair)
		}
	}
	return pairs, nil
}

// closeInterruptedPackageMigrations closes every attempt a prior process left
// running: it was cut by a restart and is never resumed. The once-per-process
// flag is set only after a scan that closed everything (PM24).
func (rt *Runtime) closeInterruptedPackageMigrations(ctx context.Context) {
	if rt.packageMigrationClosed {
		return
	}
	pairs, err := rt.packageMigrationPairs()
	if err != nil {
		log.Printf("package migration: list the records: %v", err)
		return
	}
	clean := true
	for _, pair := range pairs {
		rec, err := rt.loadPackageMigration(pair)
		if err != nil || rec == nil || len(rec.Attempts) == 0 {
			if err != nil {
				log.Printf("package migration: %v", err)
				clean = false
			}
			continue
		}
		last := &rec.Attempts[len(rec.Attempts)-1]
		if last.Outcome != pmRunning {
			rt.setPackageMigrationStatus(rec)
			continue
		}
		last.end(pmInterrupted, "interrupted by a restart at step "+last.Step)
		if err := rt.savePackageMigration(rec); err != nil {
			log.Printf("package migration: close interrupted attempt: %v", err)
			clean = false
			continue
		}
		rt.recordPackageMigrationLedger(ctx, rec, last)
	}
	rt.packageMigrationClosed = clean
}

// packageMigrationMayStart says whether a new attempt may start for the pair
// and, when not, whether the pair is closed and why. Only the attempts after
// the last rollback count (a rearmed pair starts over). triedHere: this
// process already made an attempt of the pair.
func packageMigrationMayStart(rec *packageMigrationRecord, planned, triedHere bool) (ok, closed bool, why string) {
	if rec == nil {
		return true, false, ""
	}
	attempts := rec.Attempts
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Outcome == pmRolledBack {
			attempts = attempts[i+1:]
			break
		}
	}
	interrupted, deferred := 0, 0
	for _, a := range attempts {
		switch a.Outcome {
		case pmDone, pmRefused, pmFailed:
			return false, true, "the pair's attempt ended " + a.Outcome + "; the next deploy is a new pair"
		case pmInterrupted, pmRunning:
			// Running here is an attempt no scan closed yet: a restart cut it.
			interrupted++
		case pmDeferred:
			deferred++
		}
	}
	switch {
	case interrupted >= packageMigrationMaxInterrupted:
		return false, true, fmt.Sprintf("a restart cut %d attempts; the pair is closed", interrupted)
	case deferred >= packageMigrationMaxDeferred:
		return false, true, fmt.Sprintf("%d attempts were deferred; the pair is closed", deferred)
	case interrupted+deferred > 0 && triedHere:
		return false, false, "this boot already made an attempt; the next one starts on a boot that followed a planned restart"
	case interrupted+deferred > 0 && !planned:
		return false, false, "the last attempt was cut or deferred; a new one starts only on a boot that followed a planned restart"
	}
	return true, false, ""
}

// unmigratedPackageStackRefusal refuses a platform transition (a platform
// update) while the applied stack holds package source the migration has not
// moved out (PM14): the new release does not carry that source, so it would
// leave the computer. "" admits the transition.
func (rt *Runtime) unmigratedPackageStackRefusal(ctx context.Context) string {
	if rt == nil {
		return ""
	}
	in := rt.sourceProjectionInput(ctx)
	if in.TopUnavailable != "" {
		return packageMigrationReasonUnmigrated + ": the applied stack cannot be read (" + in.TopUnavailable + "), so whether it holds package source is unknown"
	}
	if in.Pending {
		// PM29: an apply in flight (current names a selfdev release whose
		// operation has not recorded applied) is unknown until it settles;
		// fail closed, as for an unreadable stack.
		return packageMigrationReasonUnmigrated + ": an apply is in flight (" + in.PendingDetail + "); a platform transition waits until it settles"
	}
	if in.Top == nil || len(in.Top.Patch) == 0 {
		return ""
	}
	paths, err := hostbuild.PatchPaths(in.Top.Patch)
	if err != nil {
		return packageMigrationReasonUnmigrated + ": the applied stack's patch cannot be read: " + err.Error()
	}
	if len(packagepath.Classify(paths).Package) == 0 {
		return ""
	}
	if st := rt.sourceProjectionStatus(); st != nil && st.State == sourceproj.StateUpstreamed && st.OperationID == in.Top.OperationID {
		return ""
	}
	return packageMigrationReasonUnmigrated + ": the applied change " + in.Top.OperationID + " holds package source the forced-modularity migration has not moved out; a platform transition waits until it has (or until the owner rolls that change back)"
}

// migratePackageStackLogged is the trigger form: a failure is logged and
// never blocks boot.
func (rt *Runtime) migratePackageStackLogged(ctx context.Context, trigger string) {
	if rt == nil {
		return
	}
	if _, err := rt.migratePackageStack(ctx, trigger); err != nil {
		log.Printf("package migration (%s): %v", trigger, err)
	}
}

// migratePackageStack runs one attempt when the applied stack holds package
// source and the pair may start. It returns the pair's record (nil when
// there is nothing to migrate).
func (rt *Runtime) migratePackageStack(ctx context.Context, trigger string) (*packageMigrationRecord, error) {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return nil, nil
	}
	rt.packageMigrationMu.Lock()
	defer rt.packageMigrationMu.Unlock()
	rt.closeInterruptedPackageMigrations(ctx)

	result := rt.sourceProjectionStatus()
	if result == nil || result.State != sourceproj.StatePackagePath {
		rt.clearPackageMigrationWaiting() // nothing to migrate
		return nil, nil
	}
	repo := rt.releaseRepo()
	booted := strings.TrimSpace(result.BootedBase)
	if booted == "" || !sourceproj.HasCommit(ctx, repo, booted) {
		// PM11: the base fetch's reconcile triggers again
		rt.setPackageMigrationWaiting("the booted base " + shortDigest(booted) + "'s objects are not on this computer yet; the base fetch triggers the migration again")
		return nil, nil
	}
	// PM13: `current` is read and written under one lock, held to the end.
	rt.currentReleaseMu.Lock()
	defer rt.currentReleaseMu.Unlock()
	in := rt.sourceProjectionInput(ctx)
	top := in.Top
	if top == nil || in.TopUnavailable != "" || len(top.Patch) == 0 || in.Pending {
		why := "the applied stack's top operation is not readable yet"
		switch {
		case in.Pending:
			why = "an apply is in flight; the migration waits for it to settle"
		case in.TopUnavailable != "":
			why = "the applied stack's top operation is unavailable: " + in.TopUnavailable
		}
		rt.setPackageMigrationWaiting(why)
		return nil, nil
	}
	if rt.restoreRestaged.Load() {
		// PM27: a tape restore restaged `current` and the restart into it
		// is pending; the migration waits for the restored boot.
		err := errors.New("a tape restore restaged the served release and the restart into it is pending; no attempt now")
		rt.setPackageMigrationWaiting(err.Error())
		return nil, err
	}
	if top.OperationID != result.OperationID {
		err := fmt.Errorf("the applied stack moved since the projection (%s, was %s); the next reconcile triggers again", top.OperationID, result.OperationID)
		rt.setPackageMigrationWaiting(err.Error())
		return nil, err
	}
	if rt.selfdevOperations != nil {
		// PM7, narrowed by PM32: only an operation that is changing the
		// stack or `current` holds the attempt; a parked one does not.
		if open, found, err := rt.selfdevOperations.StackChangingOperation(ctx, rt.cfg.ComputerID); err != nil {
			return nil, fmt.Errorf("read the stack-changing operation: %w", err)
		} else if found {
			err := fmt.Errorf("operation %s is %s and is changing the stack; no attempt now", open.OperationID, open.State)
			rt.setPackageMigrationWaiting(err.Error())
			return nil, err
		}
	}
	rt.clearPackageMigrationWaiting()
	pair := packageMigrationPair(top.OperationID, booted)
	rec, err := rt.loadPackageMigration(pair)
	if err != nil {
		return nil, err
	}
	if rt.packageMigrationHeld(top.OperationID) {
		// PM15: the owner rolled this operation's migration back.
		if rec != nil {
			rt.setPackageMigrationStatus(rec)
		}
		log.Printf("package migration (%s): %s is held by the owner's rollback; no attempt", trigger, top.OperationID)
		return rec, nil
	}
	_, planned := rt.BootWasPlannedRestart()
	ok, closed, why := packageMigrationMayStart(rec, planned, rt.packageMigrationTried[pair])
	if !ok {
		if closed && rec != nil {
			rt.undoClosedPackageMigration(ctx, rec)
		}
		if rec != nil {
			rt.setPackageMigrationStatus(rec)
		}
		log.Printf("package migration (%s): no attempt for %s on %s: %s", trigger, top.OperationID, shortDigest(booted), why)
		return rec, nil
	}
	if rec == nil {
		rec = &packageMigrationRecord{Schema: packageMigrationSchema, Pair: pair, OperationID: top.OperationID, BootedBase: booted, StackBase: top.Base, PatchSHA256: sourceproj.SHA256(top.Patch)}
	}
	rec.ClosedUndone = false
	rec.Attempts = append(rec.Attempts, packageMigrationAttempt{
		N: len(rec.Attempts) + 1, Trigger: trigger, PlannedBoot: planned,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Step: "classify", Outcome: pmRunning,
	})
	attempt := &rec.Attempts[len(rec.Attempts)-1]
	if rt.packageMigrationTried == nil {
		rt.packageMigrationTried = map[string]bool{}
	}
	rt.packageMigrationTried[pair] = true
	if err := rt.savePackageMigration(rec); err != nil {
		return rec, err
	}
	rt.runPackageMigration(ctx, rec, attempt, top, booted)
	if err := rt.savePackageMigration(rec); err != nil {
		return rec, err
	}
	rt.recordPackageMigrationLedger(ctx, rec, attempt)
	log.Printf("package migration (%s): %s %s on %s: %s %s", trigger, attempt.Outcome, top.OperationID, shortDigest(booted), attempt.Step, attempt.Reason)
	return rec, nil
}

// undoClosedPackageMigration uninstalls, once, what a closed pair installed.
// Its caller saw the stack still served (stack_package_path), so the
// installs only shadow the stack's own source.
func (rt *Runtime) undoClosedPackageMigration(ctx context.Context, rec *packageMigrationRecord) {
	if rec.ClosedUndone || len(rec.Intents) == 0 {
		return
	}
	if failed := rt.undoPackageMigrationInstalls(ctx, rec); len(failed) > 0 {
		log.Printf("package migration: undo the closed pair's installs: %s", strings.Join(failed, "; "))
		return
	}
	rec.ClosedUndone = true
	if err := rt.savePackageMigration(rec); err != nil {
		log.Printf("package migration: %v", err)
	}
}

// runPackageMigration does the steps; it always ends the attempt.
func (rt *Runtime) runPackageMigration(ctx context.Context, rec *packageMigrationRecord, attempt *packageMigrationAttempt, top *sourceproj.Op, booted string) {
	repo := rt.releaseRepo()
	ownerID, computerID := rt.rebaseScope()
	swapped := false
	// fail ends the attempt; after the swap every failure restores (PM19).
	// The outcome is the cause's: an I/O-class cause stays deferred after a
	// restore too, so the pair stays open (PM25).
	fail := func(outcome, reason string) {
		if swapped {
			attempt.Step = "rollback"
			if rerr := rt.restorePackageMigration(ctx, rec, attempt); rerr != nil {
				reason += "; restoring the snapshot: " + rerr.Error()
			}
		} else if outcome != pmRefused {
			if failed := rt.undoPackageMigrationInstalls(ctx, rec); len(failed) > 0 {
				reason += "; undoing the installs: " + strings.Join(failed, "; ")
			}
		}
		attempt.end(outcome, reason)
	}
	step := func(name string) bool {
		attempt.Step = name
		if err := rt.savePackageMigration(rec); err != nil {
			// PM25: a record write is I/O; a transient fault (EMFILE,
			// ENOSPC) must not close the pair.
			fail(pmDeferred, "the attempt record cannot be written: "+err.Error())
			return false
		}
		if rt.packageMigrationAt != nil {
			rt.packageMigrationAt(name) // tests: a crash (panic) or a change at this step
		}
		return true
	}
	// PM21: a wiring gap is deferred, never a closed pair.
	if ownerID == "" || rt.store == nil || rt.selfdevOperations == nil || rt.packageBuilder == nil || strings.TrimSpace(rt.packageModules.Dir) == "" {
		attempt.end(pmDeferred, "this computer has no owner scope, store, operations store, package builder or module store yet")
		return
	}
	if !step("classify") {
		return
	}
	split, err := sourceproj.SplitStack(ctx, repo, top.Base, top.Patch)
	if err != nil {
		fail(pmDeferred, "the stack patch cannot be split: "+err.Error())
		return
	}
	if len(split.Platform) > 0 {
		// PM6: a platform-only stack needs a host build; nothing is moved.
		fail(pmRefused, packageMigrationReasonMixed+": the stack also changes the platform paths "+packagepath.Shown(split.Platform, 8)+"; rebuilding a platform-only stack needs a host build, which this migration does not do; nothing was moved")
		return
	}
	if len(split.Roots) == 0 {
		fail(pmRefused, "the stack changes no package path; nothing to migrate")
		return
	}

	// The snapshot (PM9), every read of it transient (PM21).
	if !step("snapshot") {
		return
	}
	current, err := readCurrentManifestIdentity(rt.selfdevUpdaterRoot)
	if err != nil {
		fail(pmDeferred, "the served release cannot be read: "+err.Error())
		return
	}
	attempt.PreReleaseDigest, attempt.PreReleaseMarker = current.ContentDigest, current.Marker
	kept, err := sourceproj.KeepStack(ctx, repo, *top)
	if err != nil {
		fail(pmDeferred, "the stack cannot be kept: "+err.Error())
		return
	}
	attempt.KeptRef, attempt.KeptCommit = sourceproj.RefKeptPrefix+top.OperationID, kept
	priorInstalls, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		fail(pmDeferred, "the installs cannot be read: "+err.Error())
		return
	}
	attempt.InstallsBefore = map[string]string{}
	for name, item := range currentPackageInstalls(priorInstalls) {
		attempt.InstallsBefore[name] = item.ReleaseID
	}
	operation, err := rt.selfdevOperations.Get(ctx, rt.cfg.ComputerID, top.OperationID)
	if err != nil {
		fail(pmDeferred, "the stack operation cannot be read: "+err.Error())
		return
	}
	// The booted baseline, resolved once for the swap and the differential
	// (PM24), and named before the swap (PM18).
	baselineRoot := rt.packageMigrationBaselineRoot()
	if baselineRoot == "" {
		fail(pmDeferred, "no trusted baseline release root")
		return
	}
	target, err := rt.servingBaselineManifest(ctx, computerID, baselineRoot)
	if err != nil {
		fail(pmDeferred, "the booted baseline manifest: "+err.Error())
		return
	}
	attempt.TargetReleaseDigest = target.ContentDigest

	// Each package root: upstreamed, installed or kept.
	if !step("packages") {
		return
	}
	for _, root := range split.Roots {
		name := strings.TrimPrefix(root, packagepath.Root+"/")
		item := packageMigrationPackage{Name: name, Root: root}
		after, _ := sourceproj.SubtreeAt(ctx, repo, split.Tree, root)
		if inBooted, ok := sourceproj.SubtreeAt(ctx, repo, booted, root); ok && inBooted == after {
			item.Outcome, item.Reason = pmUpstreamed, "the booted base holds this package's tree "+after
			attempt.Packages = append(attempt.Packages, item)
			continue
		}
		patch := split.ByRoot[root]
		item.PatchFile = filepath.Join(packageMigrationDir, rec.Pair, name+".patch")
		if err := writeFileAtomic(filepath.Join(rt.selfdevUpdaterRoot, item.PatchFile), patch); err != nil {
			fail(pmDeferred, "the package patch of "+name+" cannot be kept: "+err.Error())
			return
		}
		deferred := rt.migratePackage(ctx, ownerID, computerID, rec, operation, top.Base, name, patch, &item)
		attempt.Packages = append(attempt.Packages, item)
		if deferred != "" {
			fail(pmDeferred, "package "+name+": "+deferred)
			return
		}
		if !step("packages") {
			return
		}
	}

	// The swap, stage then swap (PM13): the booted baseline is staged at
	// releases/<digest>, then `current` moves to it in one rename. `current`
	// is never absent.
	if !step("swap") {
		return
	}
	floor := rt.storeFloorPath()
	if err := updater.StageBaselineRelease(rt.selfdevUpdaterRoot, baselineRoot, target, floor); err != nil {
		fail(packageMigrationSwapOutcome(err), "the booted baseline cannot be staged: "+err.Error())
		return
	}
	swapped = true
	if err := updater.RestagePinnedRelease(rt.selfdevUpdaterRoot, target.ContentDigest, floor); err != nil {
		fail(packageMigrationSwapOutcome(err), "the booted baseline cannot be served: "+err.Error())
		return
	}
	rt.recordStartupManifest(target)
	attempt.ServedReleaseDigest = target.ContentDigest

	// The differential (D1-D4); a failure restores the snapshot (PM8).
	if !step("differential") {
		return
	}
	diff := rt.packageMigrationDifferential(ctx, ownerID, computerID, attempt)
	attempt.Differential = &diff
	if !diff.ok() {
		if diff.Inconclusive && !diff.mismatch {
			fail(pmDeferred, "the differential could not read what it checks: "+strings.Join(diff.Detail, "; "))
			return
		}
		fail(pmFailed, "the differential failed: "+strings.Join(diff.Detail, "; "))
		return
	}
	var keptNames []string
	for _, p := range attempt.Packages {
		if p.Outcome == pmKept {
			keptNames = append(keptNames, p.Name+" ("+p.Reason+")")
		}
	}
	reason := "the stack's packages are out of the stack and the served build is the booted base " + shortDigest(booted)
	if len(keptNames) > 0 {
		reason += "; set aside, not installed (the Library lists them; source at " + attempt.KeptRef + "): " + strings.Join(keptNames, "; ")
	}
	attempt.end(pmDone, reason)
}

// packageMigrationSwapOutcome classifies a stage or swap failure (PM25): the
// store floor's refusal and a corrupt pinned release do not change on a
// retry (failed); a rename, write or read failure may (deferred).
func packageMigrationSwapOutcome(err error) string {
	if errors.Is(err, updater.ErrStoreFloorRefused) || errors.Is(err, updater.ErrPinnedReleaseCorrupt) {
		return pmFailed
	}
	return pmDeferred
}

// storeFloorPath is the guest store's schema receipt, whose floor every
// restage checks (PM24); "" skips the check.
func (rt *Runtime) storeFloorPath() string {
	if rt == nil || rt.store == nil || strings.TrimSpace(rt.store.TexturePath()) == "" {
		return ""
	}
	return filepath.Join(rt.store.TexturePath(), storeschema.File)
}

// The install refusals that are the package's own and permanent: the package
// is kept (set aside). install_review_deferred keeps it for review; every
// other refusal (an unavailable store, builder or seat, a 5xx) is transient
// and defers the attempt (PM20).
var packageMigrationPermanentRefusals = map[string]bool{
	"release_record_invalid":          true,
	"release_digest_mismatch":         true,
	"release_id_invalid":              true,
	"patch_digest_mismatch":           true,
	"patch_secret_refused":            true,
	"package_content_invalid":         true,
	"package_build_failed":            true,
	"builtin_name_not_yours":          true,
	"install_attempts_exhausted":      true,
	"install_review_refused":          true,
	pkgrelease.ReasonUsesExceedGrants: true,
	installreview.ReasonRejected:      true,
}

// migratePackage drafts one package root into a release on the owner's track
// and installs it. It fills item's outcome, or returns a non-empty reason
// that defers the attempt. A refusal that is the package's own keeps it.
func (rt *Runtime) migratePackage(ctx context.Context, ownerID, computerID string, rec *packageMigrationRecord, operation selfdev.Operation, stackBase, name string, patch []byte, item *packageMigrationPackage) string {
	keep := func(reason string) string {
		item.Outcome, item.Reason = pmKept, reason
		return ""
	}
	if apppackage.IsBuiltinPackage(name) {
		// PM23: a built-in app is never replaced at boot; the owner may
		// install it from the set-aside list.
		return keep(packageMigrationKeptBuiltin + ": " + name + " is a built-in app; the boot migration does not replace one")
	}
	draft, keepReason, deferReason := rt.draftMigrationRelease(ctx, ownerID, computerID, operation, stackBase, name, patch)
	if deferReason != "" {
		return deferReason
	}
	if keepReason != "" {
		return keep(keepReason)
	}
	item.ReleaseID = draft.releaseID
	// PM17: the intent is durable before the install, so an install a crash
	// cuts short is found and undone.
	if !rec.hasIntent(draft.releaseID) {
		rec.Intents = append(rec.Intents, packageMigrationIntent{Name: name, ReleaseID: draft.releaseID})
		if err := rt.savePackageMigration(rec); err != nil {
			return "the install intent cannot be written: " + err.Error()
		}
	}
	if rt.packageMigrationAt != nil {
		rt.packageMigrationAt("install:" + name) // tests: a crash inside the install
	}
	view, _, refusal := rt.installPackage(ctx, ownerID, computerID, packageInstallRequest{
		ReleaseID: draft.releaseID, Record: draft.canonical, TreePatch: draft.treePatch,
		change: installChange{trigger: store.PackageInstallTriggerMigration, deferReview: true},
	})
	if refusal != nil {
		switch {
		case refusal.reason == installReviewDeferred:
			return keep(packageMigrationKeptForReview + ": " + refusal.detail + "; the owner may install it from the Library, which runs the review")
		case packageMigrationPermanentRefusals[refusal.reason]:
			return keep(refusal.Error())
		default:
			return refusal.Error()
		}
	}
	item.Outcome, item.InstallID = pmInstalled, view.InstallID
	if rt.packageMigrationAt != nil {
		rt.packageMigrationAt("installed:" + name)
	}
	return ""
}

func (rec *packageMigrationRecord) hasIntent(releaseID string) bool {
	for _, intent := range rec.Intents {
		if intent.ReleaseID == releaseID {
			return true
		}
	}
	return false
}

type packageMigrationDraft struct {
	releaseID string
	canonical []byte
	treePatch []byte
}

// draftMigrationRelease drafts one package's patch into a release record on
// the owner's track, built on stackBase and naming operation. It returns the
// draft, or a reason to keep the package (its own), or a reason to defer.
func (rt *Runtime) draftMigrationRelease(ctx context.Context, ownerID, computerID string, operation selfdev.Operation, stackBase, name string, patch []byte) (packageMigrationDraft, string, string) {
	var none packageMigrationDraft
	if !objectgraph.ValidPackageName(name) {
		return none, "package_name_invalid: " + name + " is not a package name", ""
	}
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{Repo: rt.releaseRepo(), Name: name, Base: stackBase, Patch: patch, PlatformBase: stackBase})
	if err != nil {
		if pkgrelease.ReasonOf(err) != "" {
			return none, err.Error(), ""
		}
		return none, "", "the release cannot be drafted: " + err.Error()
	}
	record := pkgrelease.Record{
		Schema:  pkgrelease.Schema,
		Package: pkgrelease.Package{Name: name, ManifestVersion: source.ManifestVersion},
		Track:   ownerID,
		Parent:  source.Parent,
		Source: pkgrelease.Source{
			Root: source.Root, Tree: source.Tree, TreePatch: packageBlobRef(source.Blobs.TreePatch), ReviewPatch: packageBlobRef(source.Blobs.ReviewPatch),
			ChangedPaths: source.Paths, BuiltOn: stackBase,
		},
		Manifest:  source.Manifest,
		Publisher: pkgrelease.Publisher{OwnerID: ownerID, ComputerID: computerID},
		Operation: pkgrelease.Operation{
			OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, DecisionEvent: operation.DecisionEvent,
			MaterializationReceipt: operation.MaterializationReceipt, CheckpointRef: operation.CheckpointRef, VerifierRefs: append([]string{}, operation.VerifierRefs...),
		},
	}
	if source.Blobs.ParentPatch != nil {
		parent := packageBlobRef(source.Blobs.ParentPatch)
		record.ParentPatch = &parent
	}
	if err := record.Validate(); err != nil {
		if pkgrelease.ReasonOf(err) != "" {
			return none, err.Error(), ""
		}
		return none, "", "the release record is invalid: " + err.Error()
	}
	releaseID, err := record.ReleaseID()
	if err != nil {
		return none, err.Error(), ""
	}
	canonical, err := record.Canonical()
	if err != nil {
		return none, "", "the release record cannot be encoded: " + err.Error()
	}
	return packageMigrationDraft{releaseID: releaseID, canonical: canonical, treePatch: source.Blobs.TreePatch}, "", ""
}

// undoPackageMigrationInstalls uninstalls every install the pair's attempts
// set out to make (its intents) that is still in force as the migration
// made it (trigger migration): an owner's own later install of the same
// release is left alone (PM17). Best effort: what it cannot undo is named.
func (rt *Runtime) undoPackageMigrationInstalls(ctx context.Context, rec *packageMigrationRecord) []string {
	if rec == nil || len(rec.Intents) == 0 || rt.store == nil {
		return nil
	}
	ownerID, computerID := rt.rebaseScope()
	list, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return []string{"the installs cannot be read: " + err.Error()}
	}
	inForce := currentPackageInstalls(list)
	var failed []string
	for _, intent := range rec.Intents {
		item := inForce[intent.Name]
		if item == nil || item.ReleaseID != intent.ReleaseID || item.Trigger != store.PackageInstallTriggerMigration {
			continue
		}
		if _, _, refusal := rt.uninstallPackageWith(ctx, ownerID, computerID, intent.ReleaseID, installChange{trigger: store.PackageInstallTriggerMigrationRollback}); refusal != nil && refusal.reason != "package_not_installed" {
			failed = append(failed, intent.Name+": "+refusal.Error())
		}
	}
	return failed
}

// restorePackageMigration puts the snapshot back: the pinned pre-migration
// release is served again (its floor checked) and the pair's installs are
// uninstalled. The caller holds currentReleaseMu.
func (rt *Runtime) restorePackageMigration(ctx context.Context, rec *packageMigrationRecord, attempt *packageMigrationAttempt) error {
	var problems []string
	if attempt.PreReleaseDigest != "" {
		current, err := readCurrentManifestIdentity(rt.selfdevUpdaterRoot)
		if err != nil || current.ContentDigest != attempt.PreReleaseDigest {
			if err := updater.RestagePinnedRelease(rt.selfdevUpdaterRoot, attempt.PreReleaseDigest, rt.storeFloorPath()); err != nil {
				problems = append(problems, "restage "+shortDigest(attempt.PreReleaseDigest)+": "+err.Error())
			} else if manifest, _, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, attempt.PreReleaseDigest); err == nil {
				rt.recordStartupManifest(manifest)
			}
		}
	}
	problems = append(problems, rt.undoPackageMigrationInstalls(ctx, rec)...)
	if _, err := rt.ReconcileSourceProjection(ctx, packageMigrationTrigger); err != nil {
		problems = append(problems, "reconcile: "+err.Error())
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// rollbackPackageMigration restores the snapshot of the pair's last attempt
// when it is done, or was cut by a restart at or after the swap (PM18). The
// computer must not have moved since (PM16): `current` is still the release
// the migration served and no operation is open. The stack operation is then
// held: never migrated again on any base until the owner rearms it (PM15).
func (rt *Runtime) rollbackPackageMigration(ctx context.Context, pair string) (*packageMigrationRecord, error) {
	rt.packageMigrationMu.Lock()
	defer rt.packageMigrationMu.Unlock()
	rt.currentReleaseMu.Lock()
	defer rt.currentReleaseMu.Unlock()
	rec, err := rt.loadPackageMigration(pair)
	if err != nil {
		return nil, err
	}
	if rec == nil || len(rec.Attempts) == 0 {
		return nil, fmt.Errorf("%w: package migration %s has no attempt to roll back", errPackageMigrationNotFound, pair)
	}
	last := &rec.Attempts[len(rec.Attempts)-1]
	// What the computer may serve for the rollback to proceed (PM16): the
	// release the attempt served, or, for an attempt that did not end done,
	// the snapshot itself (its swap never landed or was already restored:
	// the rollback then only holds and undoes, PM28).
	served, alreadyRestored := "", ""
	switch {
	case last.Outcome == pmDone:
		served = last.ServedReleaseDigest
	case last.Outcome == pmInterrupted || last.Outcome == pmDeferred || last.Outcome == pmFailed:
		served, alreadyRestored = last.TargetReleaseDigest, last.PreReleaseDigest
	default:
		return rec, fmt.Errorf("%w: package migration %s: the last attempt is %s at step %s; a done attempt, or one that was cut, deferred or failed, is rolled back", errPackageMigrationState, pair, last.Outcome, last.Step)
	}
	if last.PreReleaseDigest == "" {
		return rec, fmt.Errorf("%w: package migration %s: the attempt has no snapshot release", errPackageMigrationState, pair)
	}
	current, err := readCurrentManifestIdentity(rt.selfdevUpdaterRoot)
	if err != nil {
		return rec, fmt.Errorf("%w: the served release cannot be read: %v", errPackageMigrationDrifted, err)
	}
	if current.ContentDigest == "" || (current.ContentDigest != served && current.ContentDigest != alreadyRestored) {
		return rec, fmt.Errorf("%w: the computer serves %s, not the release %s the migration served (nor its snapshot %s); restoring the snapshot would undo a later change", errPackageMigrationDrifted, shortDigest(current.ContentDigest), shortDigest(served), shortDigest(alreadyRestored))
	}
	if rt.selfdevOperations != nil {
		if open, found, err := rt.selfdevOperations.StackChangingOperation(ctx, rt.cfg.ComputerID); err != nil {
			return rec, fmt.Errorf("%w: the open operation cannot be read: %v", errPackageMigrationDrifted, err)
		} else if found {
			return rec, fmt.Errorf("%w: operation %s is open (%s)", errPackageMigrationDrifted, open.OperationID, open.State)
		}
	}
	// The hold first: a restore that half-fails must still never be
	// migrated again behind the owner's back.
	if err := rt.writePackageMigrationHold(rec); err != nil {
		return rec, err
	}
	if err := rt.restorePackageMigration(ctx, rec, last); err != nil {
		return rec, err
	}
	last.end(pmRolledBack, "the pre-migration release "+shortDigest(last.PreReleaseDigest)+" is served again, the migration's installs are uninstalled, and "+rec.OperationID+" is held: it is not migrated again until the owner rearms it ({\"rearm\": \""+rec.OperationID+"\"}) or rolls the change back")
	if err := rt.savePackageMigration(rec); err != nil {
		return rec, err
	}
	rt.recordPackageMigrationLedger(ctx, rec, last)
	return rec, nil
}

func (rt *Runtime) writePackageMigrationHold(rec *packageMigrationRecord) error {
	raw, err := json.Marshal(map[string]string{
		"schema": packageMigrationSchema + ".hold", "operation_id": rec.OperationID, "pair": rec.Pair,
		"at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	return writeFileAtomic(rt.packageMigrationHoldPath(rec.OperationID), append(raw, '\n'))
}

// rearmPackageMigration lifts the owner's hold on an operation; its next
// boot migrates it again (a new attempt; earlier ones do not count).
func (rt *Runtime) rearmPackageMigration(operationID string) error {
	rt.packageMigrationMu.Lock()
	defer rt.packageMigrationMu.Unlock()
	err := os.Remove(rt.packageMigrationHoldPath(operationID))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s is not held", errPackageMigrationNotFound, operationID)
	}
	if err != nil {
		return err
	}
	syncDirBestEffort(filepath.Dir(rt.packageMigrationHoldPath(operationID)))
	if last := rt.packageMigrationStatusView(); last != nil && last.OperationID == operationID {
		rt.packageMigrationStatusMu.Lock()
		if rt.packageMigrationLast != nil {
			rt.packageMigrationLast.Held = false
		}
		rt.packageMigrationStatusMu.Unlock()
	}
	return nil
}

// packageMigrationBaselineRoot is the booted image's release root, trusted
// under /nix/store.
func (rt *Runtime) packageMigrationBaselineRoot() string {
	if strings.TrimSpace(rt.packageMigrationBaseline) != "" {
		return filepath.Clean(rt.packageMigrationBaseline)
	}
	return trustedBaselineReleaseRoot()
}

// packageMigrationDifferential checks the attempt's result (design D1-D4).
func (rt *Runtime) packageMigrationDifferential(ctx context.Context, ownerID, computerID string, attempt *packageMigrationAttempt) packageMigrationDifferential {
	var d packageMigrationDifferential
	installs, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		d.unknown("installs unreadable: " + err.Error())
		return d
	}
	inForce := currentPackageInstalls(installs)
	// D1: every package is installed (module present), upstreamed or kept.
	d.Packages = true
	migrated := map[string]bool{}
	for _, p := range attempt.Packages {
		migrated[p.Name] = true
		switch p.Outcome {
		case pmInstalled:
			item := inForce[p.Name]
			switch {
			case item == nil || item.ReleaseID != p.ReleaseID:
				d.Packages = false
				d.mismatched("D1: " + p.Name + " is not installed in force")
			case !rt.installBytesPresent(*item):
				// The record is in force; its bytes did not read. A
				// retry reinstalls them, so this defers.
				d.Packages = false
				d.unknown("D1: " + p.Name + "'s module bytes did not read")
			}
		case pmUpstreamed:
		case pmKept:
			if strings.TrimSpace(p.Reason) == "" || attempt.KeptCommit == "" {
				d.Packages = false
				d.mismatched("D1: " + p.Name + " is kept with no reason or no kept source")
			}
		default:
			d.Packages = false
			d.mismatched("D1: " + p.Name + " has no outcome")
		}
	}
	// D2: the served release is the booted baseline the attempt resolved.
	if current, err := readCurrentManifestIdentity(rt.selfdevUpdaterRoot); err != nil {
		d.unknown("D2: current unreadable: " + err.Error())
	} else if d.Served = current.ContentDigest == attempt.TargetReleaseDigest; !d.Served {
		d.mismatched("D2: current serves " + shortDigest(current.ContentDigest) + ", not the booted baseline " + shortDigest(attempt.TargetReleaseDigest))
	}
	// D3: the projection is no longer diverged.
	result, err := rt.ReconcileSourceProjection(ctx, packageMigrationTrigger)
	switch {
	case err != nil:
		d.unknown("D3: reconcile: " + err.Error())
	case result.Diverged || result.State == sourceproj.StatePackagePath:
		d.mismatched("D3: the projection is " + string(result.State) + " and diverged")
	default:
		d.Projection = true
	}
	// D4: every install in force before still is, unless migrated.
	d.Installs = true
	names := make([]string, 0, len(attempt.InstallsBefore))
	for name := range attempt.InstallsBefore {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if migrated[name] {
			continue
		}
		if item := inForce[name]; item == nil || item.ReleaseID != attempt.InstallsBefore[name] {
			d.Installs = false
			d.mismatched("D4: the install of " + name + " changed")
		}
	}
	return d
}

// recordPackageMigrationLedger writes one owner-visible ledger record per
// ended attempt, naming each package's outcome and why (PM22). Observational:
// a failure is logged.
func (rt *Runtime) recordPackageMigrationLedger(ctx context.Context, rec *packageMigrationRecord, attempt *packageMigrationAttempt) {
	if rt == nil || rt.store == nil || rec == nil || attempt == nil {
		return
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" {
		return
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	discrepancy := types.DiscrepancyUnresolved
	switch attempt.Outcome {
	case pmDone:
		discrepancy = types.DiscrepancyConfirmed
	case pmRefused, pmFailed:
		discrepancy = types.DiscrepancyContradicted
	}
	var packages []string
	for _, p := range attempt.Packages {
		entry := p.Name + " " + p.Outcome
		if p.Outcome == pmKept {
			entry += " (" + p.Reason + ")"
		}
		packages = append(packages, entry)
	}
	cr := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    fmt.Sprintf("package-migration:%s:%d:%s", rec.Pair, attempt.N, attempt.Outcome),
		Discrepancy: discrepancy,
		Prediction: types.CommitmentPrediction{
			Hypothesis:  fmt.Sprintf("The forced-modularity migration moves the package source of the applied change %s out of the source stack into package installs, and the served build becomes the booted base %s.", rec.OperationID, rec.BootedBase),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{
			Excerpt:    fmt.Sprintf("package migration attempt %d %s at step %s: %s; packages: %s", attempt.N, attempt.Outcome, attempt.Step, attempt.Reason, strings.Join(packages, ", ")),
			SourceRef:  rec.OperationID,
			ObservedAt: stamp,
		},
		Provenance:   types.CommitmentProvenance{AgentID: packageMigrationAgentID, CommittedAt: stamp},
		EvidenceRefs: []string{rec.OperationID},
	}
	if _, err := rt.store.AppendCommitmentRecord(ctx, ownerID, computerID, cr); err != nil {
		log.Printf("package migration: ledger record %s: %v", cr.RecordID, err)
	}
}

// writeFileAtomic writes path by rename, the file and its directory synced
// (PM24), so a crash leaves the old file or the whole new one.
func writeFileAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	syncDirBestEffort(filepath.Dir(path))
	return nil
}

func syncDirBestEffort(dir string) {
	if handle, err := os.Open(dir); err == nil {
		_ = handle.Sync()
		handle.Close()
	}
}

// packageMigrationSetAside is one package a done migration kept: the owner
// sees it in the Library and may install it (PM22).
type packageMigrationSetAside struct {
	Pair        string `json:"pair"`
	OperationID string `json:"operation_id"`
	Package     string `json:"package"`
	Reason      string `json:"reason"`
	KeptRef     string `json:"kept_ref,omitempty"`
	Installable bool   `json:"installable"`
}

// packageMigrationSetAsideList lists the kept packages of every pair whose
// last attempt is done, less those the owner has since installed from it.
func (rt *Runtime) packageMigrationSetAsideList() []packageMigrationSetAside {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return nil
	}
	pairs, err := rt.packageMigrationPairs()
	if err != nil {
		return nil
	}
	out := []packageMigrationSetAside{}
	for _, pair := range pairs {
		rec, err := rt.loadPackageMigration(pair)
		if err != nil || rec == nil || len(rec.Attempts) == 0 {
			continue
		}
		last := rec.Attempts[len(rec.Attempts)-1]
		if last.Outcome != pmDone && last.Outcome != pmFailed {
			continue // a failed pair keeps its kept packages listed (PM25)
		}
		for _, p := range last.Packages {
			if p.Outcome != pmKept || p.OwnerInstall != "" {
				continue
			}
			out = append(out, packageMigrationSetAside{Pair: rec.Pair, OperationID: rec.OperationID, Package: p.Name, Reason: p.Reason, KeptRef: last.KeptRef, Installable: p.PatchFile != ""})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Pair < out[j].Pair
	})
	return out
}

// installSetAsidePackage installs a kept package at the owner's act (PM22):
// its patch is drafted again on the stack base, on the owner's track, and
// installed with the full gate, the install review included. A built-in
// app's owner may replace it from here.
func (rt *Runtime) installSetAsidePackage(ctx context.Context, ownerID, computerID, pair, name string) (PackageInstallView, *installRefusal) {
	// The record is read under the lock; the draft and the install (which
	// may run review seats for minutes) run without it, so the boot
	// machinery never waits on a review (PM30).
	stackBase, operationID, patch, refusal := rt.setAsideSource(pair, name)
	if refusal != nil {
		return PackageInstallView{}, refusal
	}
	if rt.selfdevOperations == nil {
		return PackageInstallView{}, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "this computer has no operations store")
	}
	operation, err := rt.selfdevOperations.Get(ctx, rt.cfg.ComputerID, operationID)
	if err != nil {
		return PackageInstallView{}, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "the stack operation %s: %v", operationID, err)
	}
	draft, keepReason, deferReason := rt.draftMigrationRelease(ctx, ownerID, computerID, operation, stackBase, name, patch)
	if keepReason != "" {
		return PackageInstallView{}, refuseInstall(http.StatusUnprocessableEntity, "package_content_invalid", "%s", keepReason)
	}
	if deferReason != "" {
		return PackageInstallView{}, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "%s", deferReason)
	}
	view, _, refusal := rt.installPackage(ctx, ownerID, computerID, packageInstallRequest{ReleaseID: draft.releaseID, Record: draft.canonical, TreePatch: draft.treePatch})
	if refusal != nil {
		return PackageInstallView{}, refusal
	}
	// Under the lock again: the record is read fresh, and the package leaves
	// the list only if it is still the one set aside.
	rt.packageMigrationMu.Lock()
	defer rt.packageMigrationMu.Unlock()
	if rec, err := rt.loadPackageMigration(pair); err == nil && rec != nil && len(rec.Attempts) > 0 {
		if item := setAsideItem(&rec.Attempts[len(rec.Attempts)-1], name); item != nil {
			item.OwnerInstall = draft.releaseID
			if err := rt.savePackageMigration(rec); err != nil {
				log.Printf("package migration: record the owner's install of %s: %v", name, err)
			}
		}
	}
	return view, nil
}

// setAsideItem is the kept package name of an attempt that lists its kept
// packages (done or failed), or nil.
func setAsideItem(last *packageMigrationAttempt, name string) *packageMigrationPackage {
	if last.Outcome != pmDone && last.Outcome != pmFailed {
		return nil
	}
	for i := range last.Packages {
		if last.Packages[i].Name == name && last.Packages[i].Outcome == pmKept {
			return &last.Packages[i]
		}
	}
	return nil
}

// setAsideSource reads, under the lock, what an owner's install of a
// set-aside package needs: the stack base, the operation and the kept patch.
func (rt *Runtime) setAsideSource(pair, name string) (string, string, []byte, *installRefusal) {
	rt.packageMigrationMu.Lock()
	defer rt.packageMigrationMu.Unlock()
	rec, err := rt.loadPackageMigration(pair)
	if err != nil {
		return "", "", nil, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	if rec == nil || len(rec.Attempts) == 0 {
		return "", "", nil, refuseInstall(http.StatusNotFound, "package_not_set_aside", "no migration %s", pair)
	}
	item := setAsideItem(&rec.Attempts[len(rec.Attempts)-1], name)
	if item == nil || item.PatchFile == "" || item.OwnerInstall != "" {
		return "", "", nil, refuseInstall(http.StatusNotFound, "package_not_set_aside", "migration %s sets aside no installable package %s", pair, name)
	}
	patch, err := os.ReadFile(filepath.Join(rt.selfdevUpdaterRoot, item.PatchFile))
	if err != nil {
		return "", "", nil, refuseInstall(http.StatusConflict, "patch_unavailable", "the kept patch of %s: %v", name, err)
	}
	return rec.StackBase, rec.OperationID, patch, nil
}

// packageMigrationRollbackOwnerAct answers
// POST /api/computers/<id>/self-development/package-migration-rollback
// with {"pair": "<pair>"} (the owner restores the snapshot, PM9, PM16) or
// {"rearm": "<operation id>"} (the owner lifts the hold, PM15). The proxy
// admits it only as the owner's act.
func (h *APIHandler) packageMigrationRollbackOwnerAct(w http.ResponseWriter, r *http.Request, ownerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if scopeOwner, _ := h.rt.rebaseScope(); strings.TrimSpace(ownerID) == "" || strings.TrimSpace(ownerID) != scopeOwner {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "only this computer's owner rolls a migration back", Reason: "not_owner"})
		return
	}
	var body struct {
		Pair  string `json:"pair"`
		Rearm string `json:"rearm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		body.Pair = "invalid"
	}
	if rearm := strings.TrimSpace(body.Rearm); rearm != "" && body.Pair == "" {
		if len(rearm) > 200 || strings.ContainsAny(rearm, " /\\\x00") {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "rearm names an operation id", Reason: "request_invalid"})
			return
		}
		if err := h.rt.rearmPackageMigration(rearm); errors.Is(err, errPackageMigrationNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: err.Error(), Reason: "package_migration_not_held"})
		} else if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		} else {
			writeAPIJSON(w, http.StatusOK, map[string]string{"rearmed": rearm})
		}
		return
	}
	if !packageMigrationPairShape(body.Pair) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "the body names the migration pair, {\"pair\": \"<32 hex>\"} (effective_release.package_migration.pair), or an operation to rearm, {\"rearm\": \"<operation id>\"}", Reason: "request_invalid"})
		return
	}
	rec, err := h.rt.rollbackPackageMigration(r.Context(), body.Pair)
	switch {
	case errors.Is(err, errPackageMigrationNotFound):
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: err.Error(), Reason: "package_migration_not_found"})
	case errors.Is(err, errPackageMigrationDrifted):
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error(), Reason: packageMigrationReasonDrifted})
	case errors.Is(err, errPackageMigrationState):
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error(), Reason: "package_migration_not_rolled_back"})
	case err != nil:
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error(), Reason: "package_migration_restore_failed"})
	default:
		h.rt.setPackageMigrationStatus(rec)
		writeAPIJSON(w, http.StatusOK, rec)
	}
}

// packageMigrationInstallOwnerAct answers
// POST /api/computers/<id>/self-development/package-migration-install
// {"pair": "<pair>", "package": "<name>"}: the owner installs a package the
// migration set aside (PM22). The proxy admits it as an install-class owner
// act.
func (h *APIHandler) packageMigrationInstallOwnerAct(w http.ResponseWriter, r *http.Request, ownerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	scopeOwner, computerID := h.rt.rebaseScope()
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(ownerID) != scopeOwner {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "only this computer's owner installs a set-aside package", Reason: "not_owner"})
		return
	}
	var body struct {
		Pair    string `json:"pair"`
		Package string `json:"package"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || !packageMigrationPairShape(body.Pair) || !objectgraph.ValidPackageName(body.Package) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "the body names a set-aside package: {\"pair\": \"<32 hex>\", \"package\": \"<name>\"}", Reason: "request_invalid"})
		return
	}
	view, refusal := h.rt.installSetAsidePackage(r.Context(), scopeOwner, computerID, body.Pair, body.Package)
	if refusal != nil {
		writeAPIJSON(w, refusal.status, apiError{Error: refusal.detail, Reason: refusal.reason})
		return
	}
	writeAPIJSON(w, http.StatusCreated, view)
}

func packageMigrationPairShape(pair string) bool {
	if len(pair) != 32 {
		return false
	}
	for _, c := range pair {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
