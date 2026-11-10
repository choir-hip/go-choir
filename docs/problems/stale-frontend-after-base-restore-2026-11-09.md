# A computer restored to base keeps serving the frontend of the day it was restored

Date: 2026-11-09. Status: open, not fixed. Reported by the owner on staging
("b@c is on old code. settings is old. the app dock is old").

## What the owner saw

b@c's desktop showed the old Settings window ("Desktop preferences", one
Account line) and the old app dock on the left. The owner also noted that
the candidates b@c builds in self-development show the current code.

## Evidence (read-only, 2026-11-09 about 22:55 UTC)

- b@c's observability reports the platform binary at the deployed commit
  (`cb149a273`), marker `genesis-baseline`, not layered, not diverged,
  source projection `base` at the same commit.
- The frontend b@c serves is a different bundle from d@e's
  (`app-h2oMqBB3.js` against `app-D7tQ9fzZ.js`). Its files carry
  `last-modified: Sun, 08 Nov 2026 01:05:40 GMT`. That is the minute of
  b@c's base restore (modular architecture ledger, 2026-11-08, "b@c
  cleaned", rollback-373448e1).
- b@c's bundle has none of the strings "Add another account",
  `add_account` or "Restart to update". d@e's bundle has all three.
- d@e serves a layered self-development release (marker `selfdev-…`), which
  is grafted onto each new base at boot. Its frontend files are dated today.

## Cause (from the code)

- The surface serves `$CHOIR_UPDATER_ROOT/current/frontend` whenever it has
  an `index.html`, and falls back to the deployed baseline only when it does
  not (`internal/autoputer/computer_surface.go`, `resolvedRoot`).
- A base restore copies the baseline of that moment into the updater store
  and points `current` at it (`internal/agentcore/selfdev_base_restore.go`,
  `updater.StageBaselineRelease`).
- At boot, `ensureServingBaseline` imports the baseline only when `current`
  is missing or invalid (`internal/agentcore/rematerialize.go:568`). A valid
  copy from an older deploy is kept as it is.

So after a base restore the binary follows every deploy, and the frontend
stays at the restore's commit until the next self-development apply or
restore. Capsules build from the source projection, which is at the current
base, so candidates look current. This matches both things the owner saw.

## Not yet checked

- Whether a computer that never ran self-development has the same stale
  copy. The same only-when-missing import would apply to it. The signed-out
  page comes from the proxy, so it does not show this.
- The owner's own computer and the paper. Read their served bundle dates.
- The on-disk state of b@c's `current` link was not read; the evidence is
  the served files and the code.

## Consequence for the account switcher report

b@c's launcher predates the switcher. After the owner added b@c as a second
account, the page ran b@c's old frontend, which has no account rows at all.
That alone explains "no ability to switch back"
([account switcher problem](account-switcher-no-switch-back-2026-11-09.md)).

## Remedy

- For b@c now (an owner act, not taken): any new base restore restages
  `current` from the deployed baseline. An applied self-development round
  does the same through the graft.
- Fix direction (not built): a `current` that is a pure base import
  (`releaseIsPureBaseImport` already tests for it) and whose content digest
  differs from the booted baseline's is restaged at boot, or the surface
  serves the booted baseline directly whenever `current` is a pure base
  import. The second deletes a copy instead of keeping it fresh. Boot path
  and serving join: red, two review seats, proved on d@e (restore to base,
  deploy, check the bundle date).

## Failure list for the fix

The fix: at boot, `ensureServingBaseline` (called from
`EnsureComputerSurface`, under `currentReleaseMu`) restages `current` from
the booted baseline when `current` verifies, is a pure base import (marker
`genesis-baseline`, zero accepted head, no layering entrypoint, this
computer's id) and its content digest differs from the booted baseline's
manifest. Ways it could fail, and the handling:

1. A restage on a layered or selfdev release would drop the owner's work.
   Only a pure base import is touched; a layered `current` is left alone
   (it is regrafted onto the new base by the existing path).
2. A restage on every boot. The restage makes `current` equal the booted
   manifest, so the next boot compares equal and does nothing. The one
   exception is a route that answers differently between boots (the
   manifest carries the route's code and artifact refs); that restages
   once per change, and the pinned copies stay.
3. A crash mid-restage. `StageBaselineRelease` stages into a `.stage-`
   temp dir and renames; `RestagePinnedRelease` swaps the `current` link
   with one rename. `current` is never absent. A crash between the two
   leaves the old `current` serving and an extra pinned release; the next
   boot repeats the restage (idempotent: the staged dir is verified, not
   rewritten).
4. The store floor refuses the baseline (a stale copy declares a newer
   epoch than the base). The restage is refused before the swap
   (`ErrStoreFloorRefused`); boot keeps the stale copy, logs why and does
   not fail (a refusal here must not turn a serving computer into a 503).
5. A restage while checkpoint binding runs mid-process. The follow step
   runs only on the boot path (`EnsureComputerSurface`), never on the
   checkpoint path, so a binding never sees `current` move under it.
6. A concurrent migration, base restore or apply swap. In-process writers
   of `current` take `currentReleaseMu`, and the follow takes it too. That
   covers only this process: the updater daemon is a separate process that
   outlives a runtime crash and swaps `current` without the mutex (see 7
   and the residual window below).
7. An apply in flight in the daemon. `HoldApplyKey` arbitrates a dead
   apply by comparing `current` with the journal's prior and target
   digests; a restage that moves `current` to a third digest would turn
   "nothing installed" into "unknown", or overwrite an apply's swap and
   fail a healthy apply. The daemon takes the per-key flock at entry and
   writes the `prepared` journal only after staging, so a journal check
   alone misses an apply that has not journaled yet. `UnsettledApply` now
   treats as unsettled: a journal with no result or unreadable, a per-key
   `*.lock` it cannot take with the daemon's own non-blocking flock, and an
   `operations/` that exists but cannot be listed (a missing one is
   settled). It runs at the start and again after staging, immediately
   before the swap; the swap is also skipped when `current`'s digest is no
   longer the one verified at the start (the staged release stays pinned,
   the startup record names the live `current`).
8. Stale digests bound elsewhere (checkpoint `release_digest`, a revert's
   `prior_release_digest`, `fullEvidenceOperationForRelease`, the apply
   journal's prior target, vmctl's reported release). The old pinned
   release stays under `releases/<digest>` (nothing prunes it), so each of
   these still resolves. A revert to a stale pure base is by design the
   booted base (`releaseIsPureBaseImport`: no file comparison). A tape
   restore onto a checkpoint pinned to the stale copy restages it and the
   next boot refreshes it again. The restage calls `recordStartupManifest`,
   which reports the new release to vmctl.
9. The manifest digest the runtime reports must equal the bytes served.
   The restage uses the same manifest that `StageBaselineRelease` verifies
   against the source, then `recordStartupManifest`, as the base restore
   does.
10. No trusted baseline root (a layered `CHOIR_BASELINE_RELEASE_ROOT`
    outside `/nix/store`, or unset). The step is skipped.
11. Staging copies the whole baseline (binary included) per deploy into
    `releases/`. Disk growth is one baseline per deploy per pure-base
    computer. Recorded as a residual, not handled here.

### Owner ruling: restage on changed content, not on a changed digest

Owner ruling (relayed 2026-11-09): skip the copy when the served content is
unchanged. Whole-release digests differ on every deploy (the binary and
build.json carry the commit stamp), so the restage condition is the content
a pure base import serves out of `current`, not the release digest.

What the runtime reads out of `current` on a pure base import, from the
code: `current/frontend` (the surface, `resolvedRoot`; the screenshot job,
`selfdev_shot_job.go`), `current/release-manifest.json` (the effective
release identity, the checkpoint release digest), and the absence of
`current/layering-entrypoint` (the boot wrapper). Skills and desk modules
come from `CHOIR_BASELINE_RELEASE_ROOT`/`RUNTIME_SKILLS_ROOT`, the binary
from the booted image, packages from the package store. So the compared
content is the `frontend/` tree: path plus sha256 per file, from the
manifest for `current` (already hashed, and verified by
`VerifyCurrentRelease`) and from a walk of `<baseline>/frontend` for the
booted image. The walk reads only the frontend, not the binary.

Added failures:

12. A restage when only the build stamp changed: copies the binary for
    nothing. Not done; the equal frontend digest ends the step.
13. The recorded release names an older build stamp than the running
    binary (accepted). Readers checked: `EffectiveRelease` takes
    BuildCommit from the binary and the release digest and marker from the
    manifest, and for a base release computes the source tree from the
    binary's commit, so the stale stamp changes nothing. The release
    witness compares serving digest against current digest (both unchanged
    here). vmctl stores the reported digest and build commit separately.
    No comparison of manifest stamp against binary was found.
14. A baseline with no readable frontend (no index.html): nothing is
    restaged; the surface falls back as before.
15. A frontend that differs only in file mode: not compared (modes do not
    change served bytes).

### Review round 1 (Sonnet seat) and what it changed

16. The package-migration rollback (PM16) refused a follow: after a done
    migration `ServedReleaseDigest` is T, a follow moves `current` to T',
    and the drift check saw `current != served`. Fixed in
    `rollbackPackageMigration`: when the served release and `current` are
    both pure base imports for this computer, `current` is the same base,
    not a later change. The restore itself restages the pinned snapshot
    whatever `current` is, and the install undo does not read it. A
    selfdev release in `current` is still drift.
17. A vmctl hiccup at boot made `servingBaselineManifest` fall back to
    genesis refs, which a follow would have bound. The follow now reads the
    route through `servingBaselineManifestWithRoute` and keeps the old
    `current` on any route error other than "absent". Other callers of
    `servingBaselineManifest` are unchanged.
18. A skipped follow now logs the journal file that held it.
19. Store floor: a baseline manifest declares no epoch, so the floor
    cannot refuse it by value; a floor file that cannot be read can, and
    the follow then keeps the old `current` and boot continues (tested
    with a corrupt floor file).

### Named residuals

- **Leaked `.stage-*` directories under `releases/`.** `stageReleaseAt`
  removes its temp dir on every return; only a crash leaks one, and after
  `chmodTreeReadOnly` a plain `RemoveAll` of it fails. The updater daemon
  (a second process, `ImportBaseline`) stages into the same directory
  without `currentReleaseMu`, so a boot sweep could delete a live stage.
  Not swept here.
- **Disk growth: one baseline copy per frontend-changing deploy per
  pure-base computer.** Pruning old pure-base releases is not in this
  change. A release may be pruned only when no checkpoint, operation
  (including `prior_release_digest` and the materialization receipt),
  apply journal or package-migration row (`PreReleaseDigest`,
  `ServedReleaseDigest`, `TargetReleaseDigest`) references its digest, and
  it is not `current`.
- **Owner-recovery checkpoint re-bind after a follow.**
  `publishOwnerRecoveryCheckpoint` uses the idempotency key
  `owner-recovery-<canonical head>` and its request body carries the
  release digest. After a follow at the same head, a re-bind sends the new
  digest under the same key, and the control can answer a conflict instead
  of replaying the stored response. Recorded, not handled; the checkpoint
  path deliberately never follows, so only a boot between two binds at an
  unchanged head can produce it.

### Review round 1 (Devin): the daemon is another process

20. **Residual window: check to rename.** After the last check
    (`UnsettledApply`, then the `current` digest) and before
    `RestagePinnedRelease` renames the `current` link, a daemon apply can
    still take its key lock and swap `current`; the follow's rename then
    wins and the daemon's health probe sees an identity mismatch and
    restores the prior release. The window is a few filesystem calls,
    against seconds of staging before this fix. It is accepted because the
    apply ends failed and visible, not silent (the restore puts the stale
    base back, and the next boot follows again), and a runtime that crashed
    mid-apply needs the crash to land in that window. The daemon side would
    close it: the follow would ask the daemon to do the swap under the key
    lock (a baseline-restage call on the updater client), so one process
    owns every write of `current`. That is a daemon protocol change, not
    taken here.
21. A failed swap re-reads the live `current` (`VerifyCurrentRelease`) and
    the startup record names what is actually served.

## Review record and residuals

Three rounds. Sonnet seat r1: REPAIR (the migration rollback was stranded;
a route hiccup bound genesis refs; test gaps). Devin r1: REPAIR (the same
two, plus an apply that holds its key lock before it journals). Devin r2 on
the repairs: ACCEPT, security effect none. Named residuals from r2, not
fixed: the live manifest returned after a moved `current` is not re-checked
for this computer's id; the lock probe cannot see an apply from a daemon
build older than key locks; the rollback drift check now passes any pure
base `current`, including a different baseline or an operator base restore
(bounded: the rollback restores the recorded snapshot); exits before
staging return the entry manifest; disk growth and leaked stage directories
(above). Not yet proved on a live computer.

## Deployed proof failed: the follow never runs (2026-11-10 00:04 UTC)

After 29b9c741b deployed (CI 42722583879 green), b@c booted the new build
and still serves the 2026-11-08 frontend. Its log:

    runtime: base frontend follow skipped: the route cannot be read: vmctl
    client: ComputerVersion route resolution failed (status 403): caller not
    bound to route slot computer: caller 10.208.201.2 is not bound to a live
    computer

The skip is the guard added in review round 1 (a route read error must not
bind genesis refs). The error is not a hiccup: at boot vmctl does not yet
count the computer as live (`internal/vmctl/route_authority.go:423`), so the
route read is refused on every boot and the follow can never run. The tests
used a route that answered or was absent; no test had the refusal a real
boot gets. d@e (layered release) was left alone, as intended.

Belief: the follow should not need the route at all. A plain base copy's
code and artifact refs do not change when its frontend is refreshed, so the
restaged manifest can carry the refs `current` already records. That removes
the boot-time dependency and still never downgrades refs to genesis. This is
the fourth round on this mechanism; the change is to remove the dependency
the earlier rounds kept guarding, not to add another guard.
