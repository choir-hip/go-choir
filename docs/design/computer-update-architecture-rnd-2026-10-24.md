# Updating computers that diverge: an R&D design (2026-10-24)

Status: **design research, green.** No code, no deploy, no staging access.
Written overnight for the owner by an FP0 R&D worker. Everything it
proposes to build is orange or red and is named that way per slice.
Code was read at `origin/main` 911be16b1.

Owner brief (2026-10-24, about 05:45 UTC, close to verbatim): "We really
need to be able to issue security updates from the host. Beyond that, I
think we should embrace all the automatic computers diverging and updating
on their own terms, having the option to update, and we can update stock
software, and then the app packages should all be able to update at their
own pace, so they could subscribe to auto updates. If you're using an app
package that someone else made, it could be updated, but ideally you would
be able to choose which version to use. I think there are Nix-style
patterns here that we should be thinking about. [...] And I wonder if in
any way this is something that interacts with PSA."

Labels. **[V path:line]** means read in code at 911be16b1. **[H]** means a
hypothesis: derived from code but not observed on staging, with the probe
that would settle it. **[P]** means a prior-art claim, sourced in the
Sources section. **[S]** marks speculation.

Builds on, and does not repeat:
[selfdev-update-architecture-2026-10-15](../reports/selfdev-update-architecture-2026-10-15.md)
(the release record, Stations A to C, P9 and P13),
[patch-pipeline-time-to-patch-2026-10-15](../reports/patch-pipeline-time-to-patch-2026-10-15.md)
(lanes, the watcher, SLOs by severity),
[lu-guest-updates-design-2026-10-14](../reports/lu-guest-updates-design-2026-10-14.md)
(pull, drain, rings),
the S7 app-package definition and
[app-package-carries-app-desk-2026-10-21](app-package-carries-app-desk-2026-10-21.md),
and the archived 6-model panel of 2026-09-08
([choir-platform-update-system-consensus](../archive/reports/choir-platform-update-system-consensus-2026-09-08.md)),
whose "stratified rebase" this design sharpens.

---

## The answer in one page

1. **Today a computer cannot accumulate its own changes, and a platform
   deploy silently undoes the ones it has.**
   - **Applies do not compound.** The strongest evidence came in tonight
     on staging
     ([successive-selfdev-applies-do-not-compound](../problems/successive-selfdev-applies-do-not-compound-2026-10-24.md),
     on lane `fp0-apprewrite`). The capsule seeds from the guest's
     committed source tree, and no apply ever writes to that tree. So
     every new candidate is built from the creation commit. On b@c's
     computer, the Files candidate would have reverted the Slides
     change that had already been applied. The last apply wins.
   - **A deploy drops a computer's own Go change.** Such a change runs
     only while the booted base image is the one it was built for. The
     next deploy that changes the guest image drops it at boot with a
     log line and no tape event. Meanwhile the computer's own frontend
     keeps serving beside the platform's new runtime.
   - **Divergence is a hand-set vmctl flag that nothing derives.** A
     computer flagged divergent gets no updates at all, security fixes
     included.
   - (Section 1.)
2. **The computer's own patch stack over a moving base is the data
   model.** A computer's effective source is the platform base plus its
   applied patches, in order. That stack is a projection of the tape:
   every apply extends it, restore rewinds it, and a platform or security
   update rebases it or rebuilds it, with a visible conflict path.
3. **Split the computer into a floor the host owns and a lock the
   computer owns.**
   - The *floor* is the guest kernel, guest userspace, updater, capsule
     broker and boot wrapper. The host always updates it, with a
     bounded time-to-patch, and it never moves backward, not even on
     restore.
   - The *lock* is everything the computer chose: its base, its patch
     stack, and its app packages with their pins.
   - A computer's identity is (floor, lock, tape head). That is the NixOS
     split between the system closure and the profile generation, with
     Android's rollback index and Flatpak's runtime/app split on top.
4. **A security fix in code the computer has changed is a rebuild, not a
   graft.** Choir's Go binary is static, so Guix-style reference
   rewriting has nothing to rewrite.
   - **Toolchain or module fix** (the common Go case). Rebuild the
     computer's *unchanged* stack with the fixed toolchain or module
     overlay. No rebase is needed.
   - **Fix in Choir code.** Replay the stack onto the fixed base.
   - **Clean replay.** If the replay is clean and passes an
     advisory-specific check (for example `govulncheck` on the built
     binary), it goes out under a security evidence class within the
     SLO.
   - **Conflict.** It goes to the computer's engineering desk with a
     deadline, through the computer's ordinary gate. It never takes the
     security fast path.
   - **Past the deadline on a critical fix.** The stack is **parked, not
     dropped**, *if* stock code can read the computer's state. If it
     cannot, the computer is contained or stopped instead (owner
     question Q3). Every outcome is recorded as patched, contained or
     stopped.
5. **Stock software and app packages are subscriptions.** Each lock entry
   has a channel, a policy (`auto`, `propose`, `pinned`), and a resolved
   digest. Owner policies are standing intents that survive crashes;
   each update is one occurrence. Packages by other people are published
   as source, built only by the host, and signed TUF-style so a
   publisher's key can sign only its own namespace.
6. **Restore restores choices, not vulnerabilities.** Restoring to an
   earlier head restores that head's lock, stack and state on the
   *current* floor. If the old build cannot run there, it is rebuilt for
   it. The restore is recorded as a new transaction that references the
   old head; it is never claimed byte-identical to that head.
7. **PSA fits naturally.** Each update decision is a precommitment ("after
   update X this computer is healthy within 10 minutes and is not
   restored within 7 days"). Rules settle it from receipts the updater
   and tape already produce. The scored history of updates by channel,
   publisher and package is the context the management desk reads when
   it decides the next one. A subscription is a standing precommitment
   that resolves on every tick. (Section 5; part of this is speculative
   and marked.)

**Recommendation:** architecture B (lock plus patch stack over a monotone
floor) for the platform runtime, with C's per-package model for apps
only.

**Smallest first slice (U1, red, tier 2: the boot path and a tape
projection): make applies compound on one computer.**
- The guest source repo becomes a projection of the tape: the base
  commit, plus one commit per applied operation, in order. It is
  reconciled from the tape at boot and after every apply.
- The next capsule then seeds from the effective source with no change
  to the capsule.
- Freeze's binary-reuse check compares the running binary's source with
  the effective source by binary-input paths, instead of comparing
  commit ids.
- Restore rewinds the projection to the restored head.
- A base move with a non-empty stack refuses capsule preflight
  *visibly*, with a named reason and a work item. It never silently
  builds on the old base.
- Divergence becomes derivable: it is a non-empty stack.

**U2** then records the boot wrapper's silent layer drop as a
`release_parked` fact and adds a fleet exposure report. (Section 4.6.)

---

## 1. The current state, verified

### 1.1 What a computer runs

- **The exec decision is made at boot by the guest wrapper.**
  - It runs the base image's `bin/autoputer` by default
    [V nix/autoputer-vm.nix:239].
  - It runs a layered release's own binary only when three things hold:
    `current/layering-entrypoint` exists [V :140-142]; the release's
    `base_image_manifest_digest` equals the booted image manifest's
    sha256 [V :150-157]; and the boot-loop guard has not tripped (three
    starts in 60 s) [V :159-195].
  - On a base mismatch it writes one line to
    `choir-updater/layering-diag.log` and serves the base runtime
    [V :154-156]. No event is appended and no desk is told.
- **The served frontend is the `current/` release's own copy.**
  - `ComputerSurface` serves `CHOIR_UPDATER_ROOT/current/frontend`
    [V internal/autoputer/computer_surface.go:22] and falls back to the
    immutable baseline only when that has no `index.html` [V :83-108].
  - The updater copies a release's built frontend into the release
    directory at stage time [V internal/updater/updater.go:575-587].
  - So after a base mismatch the computer runs **the platform's new Go
    runtime with its own old frontend**. That is a mixed state that no
    one built or tested.
- **Genesis.** A computer without `current/` imports the base image's
  release once, as a copied `genesis-baseline` release
  [V internal/agentcore/rematerialize.go:580-634;
  internal/updater/updater.go:711, 728-735]. A second import with a
  different digest is refused as an idempotency conflict [V :729-733].
  - **[H1]** Taken together, this suggests a computer that never
    self-developed keeps serving its genesis frontend copy after later
    deploys, so platform frontend fixes would not reach it. I did not
    find a path that re-imports the baseline when the base moves.
    Tonight's evidence (b@c's frontend-only changes survived a deploy and
    the platform build still updated) fits H1 and does not refute it.
    Probe: on a computer created before the last frontend-changing
    deploy that has never self-developed, compare the served `index.html`
    build marker with the deployed commit. If H1 holds, it is a
    current-state defect in its own right and goes in a problem record
    first.

### 1.2 The update channels that exist

| Channel | Trigger and authority | What it carries | Reaches | Cannot carry |
|---|---|---|---|---|
| **Platform deploy refresh** | push to `main` → CI → vmctl deploy refresh under a planned-restart marker [V internal/vmctl/deploy_refresh.go:17-37] | guest kernel, guest userspace, base `autoputer` binary, base frontend (in the image) | active, unheld computers not flagged `divergent` [V deploy_refresh.go:414-421]; deferred busy ones by the retry sweep [V deploy_refresh_retry.go:267]; stopped ones on next boot | a diverged computer's own code (its layer is dropped, §1.1); frontend to any computer that already has `current/` (H1); anything to a `divergent`-flagged computer while it stays up |
| **Platform-follow push (M9a)** | corpusd mints a platform-control-signed offer; the guest applies it as accepted → started → applied → checkpoint → route [V internal/agentcore/platform_update.go:28-33, 75-273] | a whole release (runtime and frontend), with a 5-minute expiry [V internal/selfdevprotocol/platform_update.go:139-140] | only `auto` + `tracking` + no diverged components [V :128-134] | anything to a diverged computer. CI no longer sends these offers: the app-layer push was dropped in a47122de0 (2026-10-09) |
| **Self-development apply** | the computer's own capsule → frozen bundle → owner or consensus decision → materializer → updater | Go change: host-built layered release [V internal/agentcore/self_development_materializer.go:335-362]; frontend-only: plain release | that one computer | a second change on top of the first (each capsule seeds from the creation commit, so the last apply wins, F5a); survival across a base change (dropped at boot, §1.1) |
| **Publish / adopt** | owner act; record = base commit + git patch + receipts + settled precommitments [V internal/release/record.go:49-118] | source only, never bytes [V record.go:7-9] | an adopter whose HEAD the patch applies to (`git apply --cached` on a scratch index [V internal/release/git.go:15-18, 49-88]); then the adopter's own capsule and gate | conflicts (refused `patch_does_not_apply`, "never a merge" [V internal/agentcore/release_adoption.go:180-191]); app packages (`Packages` must be empty [V record.go:116-117]) |
| **App package** | designed (S7, ARCH-03, owner ruling D1: install is a tape fact) | frontend module, optional backend, app desk manifest | nothing yet: all apps are compiled into one SPA and one binary [V frontend/src/lib/apps/registry.ts:53-65]; `internal/appdesk` is pure and unwired [V internal/appdesk/appdesk.go:17-22, 328] | everything, today |
| **Host-only patch lanes** | `patch-watch.yml` (daily, report-only) and `flake-bump.yml` (weekly nixpkgs PR, never merges) [V .github/workflows/patch-watch.yml:1-9; flake-bump.yml:1-16] | detection and a PR; delivery is still a deploy | host, then the deploy-refresh row above | anything to a diverged computer (the time-to-patch design does not mention divergence) |

### 1.3 Exactly where divergence stops updates

- **F1. Divergence is a manual flag nothing derives.**
  - `DivergenceStatus` is set only by an internal vmctl endpoint
    [V internal/vmctl/ownership.go:258-270; handlers.go:658].
  - The release witness states this plainly: "vmctl DivergenceStatus is a
    manual flag; no divergent_release refusal exists"
    [V internal/agentcore/selfdev_release_witness.go:220-227].
  - `ComputerLineage`, `IsTracking` and `CanFastForwardComponent` exist
    but have no callers outside their own file, and `LastRebasedAt` is
    never written [V internal/platformrelease/types.go:54-90].
- **F2. A flagged computer gets nothing.** Deploy refresh skips
  `divergent` [V deploy_refresh.go:420-421], the retry sweep forgets it
  [V deploy_refresh_retry.go:267], and the operator refresh refuses it
  without `force` [V handlers.go:783-788]. A diverged computer that stays
  up keeps an old guest kernel indefinitely. That violates requirement
  (a) by construction.
- **F3. An unflagged diverged computer loses its change silently.** Most
  diverged computers are unflagged, because nothing sets the flag (F1).
  They take the refresh, and the boot wrapper drops their layer (§1.1).
  The tape still says the operation applied. The effective-release
  identity would show `layered: false` after the boot, but no event
  records the transition [H: inferred from autoputer-vm.nix:150-157;
  probe: drill a test computer through a base-changing deploy and read
  its tape and `/health`].
- **F4. The floor and the runtime share one identity.** The layer is
  bound to the sha256 of the whole guest-image manifest
  [V autoputer-vm.nix:151-152; internal/updater/updater.go:268-279]. A
  kernel-only security patch therefore invalidates every computer's own
  Go change, even though a static Go binary does not care which kernel
  point release it runs on.
- **F5. The source tree never moves.** The guest repo
  `files/Source/platform` is cloned and checked out at the deployed
  commit once, when the data image is created
  [V internal/vmmanager/manager.go:3086-3125]. Nothing advances it. The
  host builder refuses a build whose patch base is not the booted image's
  build commit (`base_commit_mismatch`), or whose booted base is not the
  host's current image (`base_moved`)
  [V internal/hostbuild/service.go:386-399]. So after any deploy, every
  computer created before it cannot host-build a Go change. (This is P13
  of the 10-15 report, still missing.) A second change on top of an
  applied one is also authored against the seed, not against the first
  change.
- **F5a. Successive applies do not compound (staging, 2026-10-24).**
  This is the strongest single piece of evidence for the data model.
  - **The record.** The problem record is on lane `fp0-apprewrite`:
    [successive-selfdev-applies-do-not-compound-2026-10-24](../problems/successive-selfdev-applies-do-not-compound-2026-10-24.md).
  - **Where the capsule seeds from.** `PreflightSourceSnapshot` with an
    empty ref selects "the current clean committed source tree"
    [V internal/capsule/executor.go:311-314].
  - **What Freeze records.** Its patch base is the commit recovered from
    that pin [V executor.go:2061-2066].
  - **Nothing writes the tree.** No apply commits into
    `files/Source/platform` (F5).
  - **What that did on staging.** On b@c's computer, Slides
    (`selfdev-41ea6ea6…`) applied at 04:27. The next candidate, Files,
    was then built from base 0cbc52edc and carried the **old** Slides
    CSS: the coordinator rebuilt the pristine base and got the same
    hash. Approving Files would have reverted Slides. A frontend-only
    candidate rebuilds the whole SPA from the capsule's source, and
    the apply replaces the served frontend with it, so the last apply
    wins.
  - **The second effect, after the 05:02 refresh to 824b84106.** The
    capsule source stayed at 0cbc52edc. Freeze's binary-reuse check
    compares the running binary's commit with the capsule source commit
    [V internal/capsule/release_binary.go:133-155], so a frontend-only
    change had to build `bin/autoputer` from the **old** base (about 7
    minutes). Applying it would pin the computer to an old-base layered
    binary: the shape of
    [guest-runtime-fix-never-reaches-layered-computers](../problems/guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md),
    produced by self-development itself.
  - **A related protocol wrinkle, out of scope here.** After an approval
    consumes `accept_once`, a reject of the next candidate is refused
    with 409 until the owner re-arms `propose_only` (same record).
  - **What it means.** Self-development is single-slot today. A
    persistent computer cannot accumulate changes, which is the product
    promise. The drills never showed this, because each drill mints a
    fresh computer and applies once.
- **F6. There is no rebase anywhere.** Adoption refuses on conflict; the
  platform-follow path refuses divergence; no code rebases source
  (verified by grep: the `rebase` hits in `internal/projectionbase`,
  `internal/recoveryplan` and `internal/agentcore/restore_base.go` are
  about the event store, and the one in `textureowner` is about document
  revisions).
- **F7. Code identity is one opaque ref.** `CodeClosure` is
  `{source_commit, artifacts}`, and `CodeRef` is its hash
  [V internal/computerversion/input_resolver.go:28-33, 59-76]. Nothing in
  it separates floor from base from patches from packages, so no update
  can be computed from it "ledger by ledger", as the ontology asks
  ([computer-ontology.md](../computer-ontology.md), "Platform Updates To
  Divergent Computers").
- **F8. Route evidence has exactly two classes, and an unknown class
  fails the whole list closed.** `signedProjectionClass` accepts
  platform-follow (`platform` / `computer:platform_follow:update`) and
  owner self-development (`computer:self_development:*`)
  [V internal/vmctl/route_authority.go:221-231]. Tonight's problem record
  shows one unclassifiable slot taking down vmctl's list and deploy
  refresh selection for every computer
  ([selfdev-apply-on-a-constructed-computer-breaks-vmctl-list](../problems/selfdev-apply-on-a-constructed-computer-breaks-vmctl-list-2026-10-24.md)).
  Any new update class in this design has to land in the classifier
  first, or its first use repeats that outage.

### 1.4 What already exists to build on

- One apply path with tape events, planned-restart markers, a bounded
  apply attempt count [V internal/agentcore/planned_restart.go:214-220],
  an updater recovery receipt, and a checkpoint after every apply.
- Host builds of base + patch into a layered closure, keyed on booted base
  and patch base [V internal/agentcore/self_development_host_build.go:197-206, 357-366].
- An effective-release identity that already computes
  `source_base_commit`, the patch digest and the effective `tree_hash`
  [V internal/agentcore/effective_release.go:33-60, 142-143]. Derived
  divergence is mostly a read of this.
- Release records that carry the publisher's rule-settled precommitments
  [V internal/release/record.go:54; internal/agentcore/api_release.go:136-160].
- A closed vocabulary of rule-settled precommit kinds (`tests_pass`,
  `freeze_within`, `paths_confined`, `verify_pass`, `completes`,
  `visual_captured`) [V internal/types/commitment_rules.go:72-101].
- `ComponentRelease.CompatibilityRange` (a semver range per component)
  already exists in the platform release type, unused
  [V internal/platformrelease/types.go:26-33].

---

## 2. Requirements

From the brief:

- **(a) Security from the host, always.** The host can deliver a
  security update to every computer, diverged or not, with a bounded
  time-to-patch, even when the owner is away.
- **(b) Divergence on the computer's terms.** Computers diverge as they
  choose. Stock software updates are optional and chosen by the owner,
  or by the computer's desks under the owner's policy.
- **(c) Independent packages.** App packages update at their own pace by
  subscription, with version choice and pinning, including packages
  authored by others.
- **(d) Every change is a transaction.** Every change is a recorded,
  reversible transaction on the computer's event chain, and restore works
  across updates.

Derived from doctrine and settled rulings, and binding on any design:

- **(e) No second promotion authority** (C3, C8, the 09-08 panel's
  foundational law). An update to a diverged computer goes through the
  same accept → materialize → checkpoint → route path, under its own
  evidence class.
- **(f) Source in, binaries out** (owner notes §16). Only source is
  published, and the host is the only builder. Users never pass bytes to
  each other.
- **(g) Crash ends work, planned restarts resume** (CLAUDE.md). An update
  is a planned restart with a marker. An update that crashes the computer
  lands in a crash boot that closes work. A subscription is a standing
  intent, so the next tick is new work.
- **(h) Time-to-patch is measured.** The patch report's two clocks
  (exposure and pipeline) must include diverged computers, or the SLO
  silently excludes the hard cases.

The tension is (a) against (b). Every design below resolves it the same
first way: by deciding **which bytes the computer can own** and keeping
the rest the host's. The designs differ in how much of the platform
runtime they let a computer own, and in what happens to the owned part
when a fix lands under it.

---

## 3. Prior art: the one idea from each that transfers

Each entry gives the mechanism in brief, the idea that transfers, and
where the analogy breaks.

**Nix and NixOS: closures, profiles, generations, flake.lock** [P1-P5].
- **Mechanism.** A store path hashes all of its build inputs, so
  versions never overwrite each other. A closure is found by scanning
  outputs for other store hashes. A profile is a symlink to a generation,
  and an upgrade builds a new generation and repoints the link. Rollback
  repoints it back, but only while the old generation has not been
  garbage-collected. `flake.lock` pins each input by `rev` and `narHash`.
- **Transfers.** *An update and a rollback are the same operation, a
  pointer swap between immutable generations, and a lock file is the
  computer's identity.* Choir's `current/` swap already is a profile
  switch. What is missing is the lock: a content-addressed record of
  *what the generation was built from*, so an update can be recomputed
  rather than copied. Retention has to be explicit: a garbage-collected
  generation can only be rebuilt, not restored.
- **Breaks.** Nix assumes the user's configuration is source the user
  re-evaluates. A Choir computer's patch stack is that source, but it is
  authored by agents and can conflict with a new base. Nix has no
  conflict story because overlays compose by function application, not
  by text patches.

**Nix overlays** [P4].
- **Mechanism.** An overlay is `final: prev: {...}` applied in order over
  the package set's fixed point, so one replaced package reaches
  everything that depends on it.
- **Transfers.** *A computer's divergence should be expressed as an
  overlay on the platform, not a fork of it.* Where a desk or app has a
  module boundary, an overlay-shaped change ("replace module M") survives
  base moves far better than a text patch to the monolith. This argues
  for growing module boundaries (packages, desk modules) and for steering
  self-development toward them.
- **Breaks.** The Go runtime is one binary, so at that grain only text
  patches exist.

**nixpkgs release branches and backports** [P6].
- **Mechanism.** Fixes land on `master` first and are cherry-picked
  (`-x`) onto `release-YY.MM` by a label-driven action. A stable release
  accepts security fixes, patch-level bumps and compatible changes.
  Mass-rebuild fixes go through `staging`. "Small" channels advance
  faster so that urgent fixes reach users sooner.
- **Transfers.** *A security fix can be backported onto the base a
  computer is on, instead of moving the computer to a new base.* That is
  the only way to honour (b) for a computer pinned to an old platform
  release: its security update must not smuggle in feature changes.
  Choir has one line (`main`), so it needs a **support window**: the set
  of platform bases that still receive backported fixes. Outside the
  window, a security update is a base move.
- **Breaks.** nixpkgs has humans maintaining a few branches. Choir would
  have as many bases as there were deploys that created or rebased
  computers, which is why the window must be small (proposed: the last
  three platform releases, or 30 days).

**Guix grafts** [P7-P9].
- **Mechanism.** A vulnerable package gets a `replacement`. Only the
  replacement is built, and every installed package that references the
  original, directly or indirectly, has the old store file name replaced
  byte for byte with the new one in its files (so name and version
  lengths must match). This is valid only for ABI-compatible fixes, and
  Guix later rebuilds the world in the background, which retires the
  graft.
- **Transfers.** *Separate the fast security path from the correct full
  rebuild, and record the fast path as temporary debt that a later full
  rebuild retires.* For Choir this means a quick, narrow security
  transaction now, and the full rebase of the patch stack when it
  converges.
- **Breaks, and this matters.** Choir's runtime is a statically linked Go
  binary. A Go stdlib or module CVE lives *inside* the binary, with no
  store reference to rewrite (my inference from how Go links; not from
  the Guix sources). So for Choir the graft equivalent is a **rebuild of
  the computer's own patch stack against the fixed toolchain or
  modules**, which is cheap with Nix caching and deterministic, but not
  free. Grafts do transfer literally to the floor (guest userspace shared
  libraries), where the base image rebuild already handles it. A graft
  analog for the frontend is conceivable [S]: an import-map rewrite that
  replaces one content-hashed vendor chunk, valid only when the chunk's
  exports are unchanged. It is not worth building first.

**Android A/B, Virtual A/B and APEX** [P10-P11].
- **Mechanism.** The update is written to the inactive slot, the slot is
  verified, and after boot `update_verifier` calls
  `markBootSuccessful()`. A slot that never becomes successful is
  reverted to after a few tries. APEX modules are signed, versioned
  system modules that update outside full OTAs. Their version code may
  not go down, and the copy in `/data` shadows the one in `/system`.
- **Transfers, two ideas.**
  - *Success is a post-boot health commit, not the end of the install.*
    Choir has most of this already in the updater's health identity
    probe and the boot guard.
  - *A version floor that refuses downgrades*, which Android enforces per
    module and per boot image (rollback protection). Choir's floor
    should be monotone the same way, and restore must respect it.
- **Breaks.** Android devices are not self-modifying, and their
  divergence is limited to data.

**Debian security and unattended-upgrades** [P12-P14].
- **Mechanism.** A separate security archive carries only fixes
  backported to the version in stable, approved by the security team.
  By default `unattended-upgrades` follows only the release and its
  `-security` suite, with a package blacklist and optional scheduled
  reboots.
- **Transfers.** *Security is its own narrow channel that every machine
  follows by default, distinct from feature updates, with an explicit
  opt-out list and a reboot window.* This is the shape requirement (a)
  wants for the platform runtime: a default-on `security` subscription,
  separate from the `stable` feature subscription.
- **Breaks.** Debian machines do not carry local source patches against
  the packages being upgraded. When they do (dpkg conffile prompts), the
  experience is famously poor. That is a warning: a conflict must go to
  an agent with a deadline, not to an interactive prompt.

**Fuchsia packages and TUF** [P15-P19].
- **Mechanism, Fuchsia.** A package is `meta.far` plus content-addressed
  blobs. Its Merkle root pins the whole content, and a package URL
  either names a hash (exact) or not (newest). The system updater checks
  an `epoch.json` that blocks downgrades across incompatible epochs.
- **Mechanism, TUF.** An offline root role delegates to a targets role
  (which can delegate by path pattern), a snapshot role and a
  timestamp role. Signature thresholds, version checks and short expiry
  defeat rollback, freeze and mix-and-match attacks.
- **Transfers, two ideas.**
  - *Packages are content-addressed and pinned by hash. The pin is the
    unit of choice.*
  - *Third-party publishers sign through a delegated role scoped to
    their own namespace*, so their key cannot sign for platform paths.
    Snapshot and timestamp metadata with short expiry stop a withheld or
    replayed update from silently freezing a computer on a vulnerable
    version.
- **Breaks.** Choir's host is both the repository and the only builder,
  so the publisher signs *source* and the host signs *builds*. That
  calls for two signature chains, which TUF's delegation handles (a
  build-attestation role), with in-toto-style provenance as the natural
  extension (not read tonight).

**Patch stacks: git rebase and rerere, quilt, StGit, Darcs and Pijul**
[P20-P24].
- **Mechanism.**
  - quilt and StGit keep local changes as an ordered, named series over a
    base, with push, pop, refresh and rebase.
  - `git rerere` records a conflict resolution and replays it when the
    same conflict recurs.
  - Darcs treats patches as first-class objects with inverses and
    commutation.
  - Pijul makes a conflict a first-class graph state rather than a failed
    merge.
- **Transfers.** *A computer's divergence is an ordered, identity-bearing
  patch series over a pinned base, and rebasing is replaying the series.*
  A patch that fails to replay becomes a **first-class parked state**
  (Pijul), reported and held, so that it never blocks the security
  update. Resolutions are cached and reusable (rerere): when twenty
  computers adopted the same release, one resolution of its conflict
  with a new base serves all twenty.
- **Breaks.** Patch theory gives commutation for text. Choir also needs
  semantic compatibility: tests, the store schema epoch, desk
  contracts. So a clean replay is necessary but not sufficient, and the
  computer's declared tests are part of the replay.

**Lockfiles and version selection: Cargo, Go MVS, bzlmod** [P25-P28].
- **Mechanism.** Cargo separates ranges (`Cargo.toml`) from exact
  resolutions (`Cargo.lock`). Go's minimal version selection picks the
  maximum of the stated minimums, so a new upstream release changes
  nothing until someone raises a minimum, and upgrade timing is left to
  the user. bzlmod uses minimal version selection (MVS) with a lock and
  lets only the root module override.
- **Transfers.** *The computer's pin lives in a lock, the publisher
  states compatibility as a minimum or a range, and a security fix is a
  raised minimum ("floor") that the resolver honours automatically.* A
  registry "yank" is the host's lever against a bad version.
- **Breaks.** Choir's app packages also carry data and an agent (the app
  desk). Version selection must therefore also respect the data schema
  a package reads, which no lockfile system handles. That has to live in
  the package's data contract (S7 already designs it).

**Flatpak runtimes and Snap channels** [P29-P31].
- **Mechanism, Flatpak.** An app declares the runtime and runtime branch
  it needs. Runtimes are shared, installed separately, and patched within
  an ABI-stable branch for about two years.
- **Mechanism, Snap.** Snap channels are `track/risk/branch`, with a
  user-set `refresh.hold` up to a maximum.
- **Transfers.** *The app pins a platform API branch, not a platform
  build, and the platform patches that branch underneath it.* This is
  exactly what F4 lacks: Choir's layer pins the whole image digest when
  it should pin a guest ABI. The Snap model shows that a hold needs a
  hard maximum and an explicit policy on whether a security update
  overrides it. Neither Snap source settled that, and Choir must decide
  it (Q3).

---

## 4. Candidate architectures

The three candidates share a vocabulary.

- **Floor:** the part of the guest the host owns and the computer cannot
  diverge. That is the guest kernel, guest userspace, updater, capsule
  broker, boot wrapper and the guest ABI they present to the runtime
  (env contract, updater protocol, store schema epoch range). Host
  components (host kernel, Firecracker, vmctl, proxy) are outside every
  computer and always follow the host's lanes.
- **Base:** a platform release, meaning a `main` commit the platform
  built and published (the deploy already writes `platform-release.json`;
  the 10-15 report proposes publishing it into the release registry).
- **Stack:** the computer's own applied changes, in order, each a source
  patch with its operation id.
- **Packages:** app packages with versions, digests, channels and pins.

### 4.1 Architecture A: floor and channels (degrade to stock)

**Data model.** `identity = (floor_ref, runtime_ref, tape_head)`. Here
`runtime_ref` is either a platform release (tracking) or a computer
release (diverged). Divergence is derived: the effective release is not
a platform release.

**Security flow.**
- **Floor fixes** go to everyone through deploy refresh, divergent or not
  (F2 deleted).
- **Runtime fixes** go to tracking computers through platform-follow.
- **A diverged computer** facing a critical runtime fix has its release
  *parked*. It boots the patched stock runtime, and a `release_parked`
  event and a work item are recorded. It stays on stock until the owner
  or engineering re-applies the change on the new base (as a fresh
  self-development operation).

**Stock updates.** The computer is `tracking`, or diverged and therefore
pinned to its own release.

**Packages.** Out of scope for A; it is one unit.

**Restore.** Restoring to a head with a parked release restores the
release. If its base-image binding no longer matches, it parks again
(so restore is mostly useless across floor moves).

**Who decides.** The host decides the floor and whether to park; the
owner decides re-application.

**Failure modes.**
- Every floor move parks every diverged computer while F4 stands, so
  divergence is lost about weekly.
- The owner experiences "my change vanished" unless the surface makes
  parking loud.
- Rewriting the change by hand on each base does not scale.
- **Stock code may not be able to read state the diverged runtime
  wrote** (new object kinds, event kinds, schema). Parking can then
  corrupt data or expose it. This was the sharpest point four of six
  panelists made.
- A does nothing for F5a: applies still do not compound.

**Cost.** Small. Most of it is recording what already happens.

### 4.2 Architecture B: lock and patch stack over a monotone floor (recommended)

**Data model.** One new immutable record, the **computer lock**, which is
content-addressed and committed by an applied event:

```text
ComputerLock {
  floor:     { image_manifest_digest, guest_abi: "v<N>", security_epoch: <int> }
  base:      { platform_release_id, commit }
  stack:     [ { operation_id, patch_sha256, base_commit_authored_on,
                 resolution_of?: <prior patch_sha256> } ... ]   // ordered
  build:     { builder_receipt, closure_digest, entrypoint, toolchain }  // derived
  packages:  { <publisher>/<name>: { version, source_digest, build_digest,
                                     channel, policy, data_schema } }
  policy:    { platform: {channel: stable|security-only, mode: auto|propose|pinned},
               security: {mode: auto|propose, hold_max: <duration>} }
}
```

- `CodeRef` becomes the hash of `(base, stack, build)`.
  `ArtifactProgramRef` is unchanged. ComputerVersion keeps its meaning,
  and the lock is its explanation (it answers F7).
- `security_epoch` is a monotone integer the host raises when a fix must
  not be rolled back, as with Android's rollback index or Fuchsia's
  `epoch.json`.
- Divergence is derived: `diverged ⇔ stack ≠ []` (and per component, the
  paths the stack touches). The vmctl flag and its two skip sites are
  deleted (F1, F2).
- **The effective source is first-class.**
  `effective_source(head) = base.commit + stack[0] + … + stack[n]`,
  applied in order, where the stack is the applied operations on the
  effective chain at that head.
  - **Authority and projection.** The tape is the authority. The guest
    repo `files/Source/platform` is its *projection*: the base commit
    plus one commit per applied operation (trailers
    `Choir-Operation: <id>` and `Choir-Applied-Event: <digest>`), and a
    ref per applied head (`refs/choir/applied/<digest>`).
  - **Reconciliation.** The repo is reconciled at boot and after every
    apply by comparing tree hashes. There is one state authority, so a
    crash between the applied event and the commit is repaired at the
    next boot, never trusted. (Standing question: single state
    authority.)
  - **Consequences.** It falls out that:
    - the next candidate is authored on top of the stack, so applies
      compound (F5a);
    - its patch is relative to the effective source;
    - restore rewinds the projection to the restored head's ref, and
      later commits stay reachable as parked refs;
    - a base move rebases the stack (below).
  - **What it repairs.** F5, F5a and P13 become consequences of one
    model rather than three side fixes.
- **Binary reuse is judged against the effective source.** Freeze may
  reuse the running binary iff the binary-input paths
  (`internal/capsule/release_binary.go:56-101`) are identical between the
  running binary's source and the capsule's effective source. This is a
  `git diff --name-only` over binary inputs, not equality of commit ids.
  - A base binary's source is its build commit.
  - A layered binary's source is its `code_commit` plus the Go part of
    the stack it was built from.
- **Host builds take the stack.** The builder accepts `(base commit,
  cumulative patch)`, where the cumulative patch is the diff from
  `base.commit` to the effective source with the new candidate. The
  builder's base-commit check
  [V internal/hostbuild/service.go:396-398] holds unchanged.

**Security flow to a diverged computer.**
1. A fix lands on `main`. The patch watcher or a human classifies it by
   lane (floor or runtime) and severity (S0, S1, S2 from the patch
   report).
2. **Floor fix.** The host builds the new floor, and deploy refresh
   reaches every computer. A computer's layer survives when its
   `guest_abi` is in the new floor's supported range *and* its closure's
   references resolve in the new store. This replaces the whole-digest
   binding (F4) with an ABI contract, in the spirit of the Flatpak
   runtime branch. No rebuild is needed.
3. **Runtime fix.** For each computer whose effective source does not
   contain the fix, the cheapest applicable transform is chosen.
   - **(i) Toolchain or module fix** (Go stdlib, a Go module, an npm
     package). Rebuild the computer's *unchanged* effective source with
     the fixed toolchain or a `go.mod`/lockfile overlay. Nothing is
     rebased. This is likely the most common case for the Go runtime
     [H: the CVE mix is unmeasured].
   - **(ii) Fix in Choir code, base inside the support window.** The host
     cherry-picks the fix onto the computer's `base` (the backport
     pocket) and replays the stack.
   - **(iii) Otherwise.** Replay the stack onto the nearest base that
     contains the fix.

   All three are deterministic and cached: computers with the same
   `(base, stack)` share one build. This is Choir's rebuild-graft.

   **Shadow rebase.** The host runs (iii) continuously in the
   background against each new `main` and records the result
   (`clean` / `conflict`) as a fact. It never applies anything from this
   run. On the day of a critical CVE, conflict debt is already known and
   most candidates are already built, which defuses the
   thundering-herd failure the panel named.
4. **Clean replay, tests pass, and the advisory check passes.** The
   advisory check is the remediation evidence, for example `govulncheck`
   in binary mode on the built layer, or the advisory's own regression
   test. A clean replay is not remediation, because a stack can
   reintroduce or route around a fix (panel).
   - The host mints an offer under a new evidence class
     `computer:platform_follow:security`.
   - The computer applies it on the ordinary path (accept → materialize
     → checkpoint → route) within the SLO. Policy `auto` applies it
     without the owner.
   - **The class is admissible only for the deterministic transforms
     (i) to (iii) of host-signed fix commits.** Nothing agent-authored
     ever rides it. Otherwise the security class becomes a supervision
     bypass under deadline pressure (panel).
   - The route classifier must accept the new class before the first
     offer (F8).
5. **Conflict, failing tests, or a failing advisory check.** A work item
   goes to the computer's engineering desk: "rebase stack onto
   `<base'>`", with a deadline from severity.
   - Engineering resolves it in a capsule, producing a new stack entry
     with `resolution_of`.
   - The resolution goes through **the computer's own gate**, as any
     self-development does.
   - It is published as a release record, so other computers carrying
     the same patch can reuse it (rerere across the fleet). Each reuse
     goes through that computer's own gate too.
6. **Deadline passes on S0 or S1 with no resolution.** The outcome is
   recorded as one of three. These are different guarantees, and the
   panel was right that they must not be blurred:
   - **Patched (park).** The computer moves to `(floor', base',
     stack = [])`, the patched stock runtime, with a `stack_parked`
     event naming the parked entries.
     - This is allowed only if a data-compatibility check passes: stock
       code can read every object kind, event kind and schema epoch the
       stack wrote.
     - The parked stack stays in the lock history. A later rebase, or a
       restore, brings it back.
     - The owner sees "your change X is parked because of security fix
       Y" in Texture.
   - **Contained.** The stack keeps running. The host narrows its
     capabilities (egress, public ingress, publication, gateway
     credentials) until it is patched. This covers exposure, not the
     vulnerable code.
   - **Stopped.** The computer is checkpointed and halted. This is the
     fallback when park is data-incompatible and containment cannot
     cover the advisory.
   - Which of the three is the default is owner question Q3. The panel
     split on it, and so do I. My lean: park when the data check
     passes, otherwise contain; stop only on escape-class floor
     advisories, which do not arise here because the floor is never
     diverged.

**Stock updates.** Platform feature releases are a subscription
(`policy.platform`):
- `auto`: rebase the stack onto each new release, apply it if it
  replays cleanly, and send it to engineering if not.
- `propose`: management drafts the rebase and the owner or the
  consensus policy decides.
- `pinned`: stay on `base` and receive security backports only while the
  base is inside the window. Leaving the window raises a notice and then
  forces a security base move.

The security subscription is separate and on by default (the Debian
`-security` pocket).

**Packages.** These are lock entries (4.4).

**Restore and checkpoints.**
- A checkpoint binds the lock digest.
- Restore to head H applies `lock(H)` with
  `floor = max(current floor, security_epoch floor)`. The floor is never
  restored, because it is host software (restore-set doctrine: platform
  is OUT).
- If H's build cannot run on the current floor (its ABI is out of range,
  or its closure has been garbage-collected), the host rebuilds H's
  `(base, stack)` against the current floor and security pocket. Restore
  restores the computer's *choices* (stack, pins, state), not its
  vulnerabilities.
- **The restore is a new transaction that references H.** It records
  the requested head, the security obligations applied, and the
  resulting effective lock. It is never claimed byte-identical to H
  when a substitution happened.
- **The source projection rewinds to H's ref.** The next capsule builds
  on what is actually served.
- **Some facts are monotone across restore:** the security epoch,
  credential rotations, token revocations and package yanks. Restore
  never undoes them.
- State (tape, Dolt, files) is restored as today.
- Retention keeps the closures of the last N generations and of every
  pinned checkpoint, and a garbage-collected one costs a rebuild rather
  than being lost.

**Who decides.**
- The host decides the floor and security epochs.
- The owner sets the policies once. They are standing intents (they
  survive crashes, and each tick is new work, as with Autopaper source
  schedules).
- The management desk executes within policy, and engineering resolves
  conflicts in capsules.
- The consensus policy (`reversible-selfdev-v1`, human seat absent)
  covers `propose`-mode decisions when the owner is away.
- Texture renders the commitment ledger (pending updates, parked
  patches, deadlines) as the supervision surface.

**Failure modes.**
- **Silent semantic breakage.** The stack replays cleanly but changes
  meaning on the new base. The mitigations are the computer's declared
  tests, a probation window with automatic restore (LU3b), and the PSA
  records in section 5. This is the residual risk that never goes to
  zero.
- **Support window explosion.** Too many distinct bases. Cap it, and
  move computers outside the window forward.
- **Rebase storms.** A large `main` change conflicts with many stacks at
  once. Resolution reuse helps, and rings (LU3a) bound the blast radius.
- **Toolchain drift.** A stack authored with Go 1.N that does not build
  with 1.N+1 sends a security base move to engineering. That is correct
  but costly.
- **New-class outage.** If the classifier does not know
  `platform_follow:security`, the first offer takes down vmctl list
  (F8). Land the classifier first.
- **Floor ABI contract errors.** A floor marked compatible that is not
  produces a boot failure. The boot guard (three starts, then base)
  catches it, and must record it as an event.
- **Parking looks like loss.** It needs a loud surface and a one-act
  "unpark", meaning a restore to the pre-park head with a rebuild.

**Cost.** Medium to large, but every piece maps to an existing seam: the
host builder, the platform-follow apply path, the effective-release
identity, the release record and LU's rings.

### 4.3 Architecture C: everything is a package

**Data model.**
- The platform runtime itself is decomposed into packages: the shell,
  each system desk's module set, each app with its desk, the frontend
  framework.
- A small core (event appender, updater client, kernel module surface)
  stays monolithic.
- `identity = (floor, core_version, {package → digest})`. Divergence is
  per package: a computer forks package P into P'.
- A minimal-version-selection resolver picks versions from the
  computer's pins and the packages' minimums.

**Security flow.**
- A fix to package P is a new P version with a raised minimum, and
  computers on P upgrade per policy.
- A computer that forked P (P') rebases P' only, a much smaller patch
  surface.
- A fix to the core is a floor-like mandatory update.

**Stock updates and packages.** These are uniform: everything is a
subscription.

**Restore.** Restore applies the package set at H. Per-package restore
becomes possible: roll back only Slides, with its data.

**Failure modes.**
- **Go cannot split.** A static binary has no runtime module loading.
  Desks are yaegi cells, so their *modules* could be packages, but the Go
  host code under them is not. Choir would need plugin boundaries (a
  process per package, or interpreted modules) it does not have.
- **The dependency hell it imports.** MVS keeps resolution tractable, but
  cross-package data contracts (shared object kinds, Store B schema)
  make version sets interact.
- **Huge up-front cost before any security benefit.**
- **Self-development blur.** Self-development would have to target
  packages, which changes the capsule's unit of work.

**Cost.** Very large. It is right for the app layer and wrong for the
core.

### 4.4 The recommended hybrid: B for the platform, C's package model for apps

The platform runtime follows B: one base, one stack and one build, over a
monotone floor with an ABI contract. App packages (S7) follow C's model
as entries in the same lock:

- **Identity.** `<publisher>/<name>@<version>`, with `source_digest`
  (published source), `build_digest` (host build), the platform API
  range it declares (`ComponentRelease.CompatibilityRange` is the
  existing field), and the data schema it reads and writes.
- **Install** is a tape fact (ruling D1). The install event binds
  `build_digest` and the desk manifest digest (ARCH-03). Uninstall
  retires the desk.
- **Subscription.** A channel (`stable`, `beta`, or a publisher's track)
  and a policy (`auto`, `propose`, `pinned`) per package. Each tick is an
  occurrence of the standing intent. An update is an ordinary
  transaction:
  - accept the new `build_digest`;
  - run the data migration (S7's contract: idempotent or reversible to
    the prior compatible release);
  - health-check, checkpoint, and restore automatically on a failed
    probation.
- **Version choice and pinning.** The lock holds an exact digest. Ranges
  live only in the subscription. `pinned` means the digest never moves
  except for a host-issued security minimum (a yank or raised floor) on
  that package.
- **A security minimum on a package** is the package-level floor. The
  owner can hold it for `hold_max`, and then it applies or the package is
  disabled. Disabling keeps its kinds readable but not writable
  (`appdesk.go` semantics, design doc line 74-78).
- **Packages by others.**
  - Source is published to the host registry. The host builds it,
    reproducibly, and only the host builds.
  - Metadata is signed TUF-style. The platform root delegates a targets
    role per publisher namespace (a publisher can sign only
    `<publisher>/*`). The host signs build attestations under a separate
    role. A short-expiry timestamp role prevents freeze and replay.
  - Until the sandboxed frame lands (R1, the same-origin risk), install
    stays limited to the host-built catalog, as D1 already says.
- **Platform moves against packages.** A platform base move checks every
  installed package's declared API range.
  - Out of range: the package update that restores compatibility is
    required, or the base move waits.
  - On a security base move, the package is disabled (kinds read-only)
    rather than blocking security.
- **Rollback of a package** is a restore to the pre-update head. Package
  data is VM-local and so in the restore set. A finer per-package restore
  waits for per-package data scoping [S].

Self-development should be steered toward package boundaries where they
exist. A change expressed as "replace package P's module" survives base
moves; a text patch to the monolith may not. This is the overlay lesson.

### 4.5 Comparison

| | A floor + channels | B lock + stack (rec.) | C all packages |
|---|---|---|---|
| (a) security to diverged | yes, by parking (loses divergence) | yes; rebuild-graft, park only past the deadline | yes, per package; the core is mandatory |
| (b) divergence on own terms | weak: one release, lost on floor moves | strong: stack survives floor moves (ABI) and rebases | strongest, per package |
| (c) independent packages | no | yes, as lock entries (hybrid) | native |
| (d) recorded, reversible | yes, once parking is recorded | yes; restore rebuilds on the current floor | yes |
| Applies compound (F5a) | no | yes: the effective source is the model | per package only |
| New mechanism | park event, derived divergence | lock, backport pocket, host rebase job, security class, ABI contract | plugin runtime, resolver |
| Matches existing seams | high | high | low |
| Cost | S | M-L, sliced | XL |

### 4.6 The smallest first slice and the sequence

**U1 (smallest first slice): applies compound on one computer.** Red
(the boot path, a tape projection, the capsule's subject), tier 2
review. U1 needs no host change, no new evidence class and no base
move. It is the minimum that makes "a persistent computer that develops
itself" true for a computer that stays on one base.

1. **Projection.** After each `materialization_applied` event, the
   runtime commits the operation's frozen `source.patch` onto the guest
   repo's computer branch:
   - the commit carries the operation and applied-event trailers;
   - a ref `refs/choir/applied/<applied-event-digest>` points at it;
   - the work tree is checked out clean, because the capsule preflight
     refuses a dirty tree [V internal/capsule/source_snapshot.go:219-243].

   Nothing in the capsule changes: it already seeds from the clean
   committed HEAD [V executor.go:311-314].
2. **Reconcile.** At boot, and after any restore, the runtime derives
   the expected chain from the tape. That chain is the base commit, then
   the applied operations on the effective chain in order, each with its
   bundle's `source.patch` and `source_patch_base_commit`.
   - It compares the expected tree hash with HEAD's.
   - A mismatch is rebuilt from the tape and recorded as a
     `source_projection_repaired` fact.
   - A projection that cannot be rebuilt (a patch missing from the
     bundle store) refuses capsule preflight with a named reason. It
     never falls back to the base silently.
3. **Binary reuse against the effective source.** In
   `PlanReleaseBinaryReuse`, the commit-equality test
   [V release_binary.go:150-152] becomes "no binary-input path differs
   between the running binary's source and the capsule source". A
   frontend-only candidate on a computer whose stack is frontend-only
   then reuses the running base binary again.
4. **Host build of a Go candidate.** The materializer sends the
   cumulative patch, from `base.commit` to the effective source with the
   candidate, as the one patch the builder already accepts. The builder
   is unchanged.
5. **Base move, visible refusal.** When the booted build commit is not
   the stack's base and the stack is non-empty, capsule preflight
   refuses with `stack_base_moved` (stack base, booted base) and opens
   an engineering work item. The rebase itself is U4.
   - This is strictly better than today, where the capsule silently
     builds an old-base binary (F5a).
   - With an empty stack, the projection simply moves to the booted
     base. Doing so requires the base commit's objects in the guest
     repo; see residual R3.
6. **Divergence derived.** `diverged = stack non-empty`. It is reported
   in the effective-release identity and in vmctl's list beside the old
   flag. Nothing reads it for decisions yet.

**Failure modes, written before code** (CLAUDE.md "Testing"):

- **Fate-sharing.** The projection must not be trusted after a crash
  between the applied event and the git commit. Reconcile, not the
  apply-time commit, is the authority.
- **Patch base after U1.** A patch authored after U1 is relative to a
  computer-local commit, not a platform commit. Publish/adopt must then
  carry the cumulative patch against the platform base, or the stack.
  Today's `release.Record` has one `base_commit` and one patch
  [V internal/release/record.go:84-85]; publishing needs a decision
  (residual R4).
- **Restore between apply and projection.** A restore that lands
  between an apply and its projection must rewind to the restored head.
  A test drives exactly this ordering.
- **Agent edits to the repo.** The repo sits under `files/`, which
  owner or agent file writes could touch. Reconcile treats any
  difference from the tape-derived tree as damage. It does not stage
  that difference as a change.
- **The reject wrinkle.** The `accept_once` reject refusal from the same
  record is separate. It should get its own problem record and fix.

**Proof on staging:**
- On a fresh test computer, apply A (a frontend change), then B (another
  frontend change).
- The served SPA carries both, and B's frozen tree carries A's files.
- Restore to the head after A. The projection's HEAD equals A's ref, and
  a new candidate C is built on A and not on B.
- Reboot. The reconcile is a no-op.
- Force a mismatch (a test-only hook). Reconcile repairs it and records
  the fact.

**Rollback:** revert U1. The projection commits are inert extra commits.
The capsule seeds from HEAD either way, so after a revert the operator
resets the branch to the base commit (one documented command), or leaves
it.

**Then, in order:**

- **U2. Name the loss (orange, with one red step).**
  - **`release_parked`.** The boot wrapper's base-mismatch fallback
    [V autoputer-vm.nix:150-157] writes a marker. The runtime turns it
    into a `release_parked` event (release, booted base, reason) and a
    work item.
  - **A fleet exposure report.** For fix commit C, per computer: the
    floor, whether the executing runtime's effective source contains C,
    and whether the computer is parked, pinned or excluded. This is
    requirement (h).
  - **H1's probe.**
- **U3. Floor ABI contract (red, boot path).** The layer binds a
  `guest_abi` range plus closure resolvability, not the whole image
  digest (F4). Kernel-only and userspace-only fixes stop parking layers.
  This is the highest-yield security change for diverged computers,
  because most real CVEs are floor CVEs (patch report §3). It needs an
  inventory of what the runtime takes from the base (residual R2).
- **U4. Host rebase and rebuild (red).**
  - The host owns every commit, so it replays the stack onto a new base
    (transforms i to iii of §4.2) and returns the rebased stack to the
    guest as a git bundle with the build.
  - Shadow rebase runs in the background.
  - A clean result becomes an offer under
    `computer:platform_follow:security` (classifier first, F8). A
    conflict becomes an engineering work item.
  - This also clears U1's `stack_base_moved` refusals.
- **U5. Policies as standing intents (orange).**
  - `policy.platform` and `policy.security` live on the tape, with an
    owner act to set them, and management executes them.
  - The vmctl flag and its skip sites are deleted (F1, F2).
- **U6. The lock record as CodeRef input; checkpoints bind it (red,
  tier 2: tape and evidence).**
- **U7. App packages as lock entries.**
  - S7-0, with subscription and pin fields from day one.
  - TUF-style publisher delegation when third-party packages open.

U2 and U3 are independent of U1 and of each other. U4 needs U1, U2 and
the classifier change.

---

## 5. Interaction with PSA, precommitment records and in-context RL

PSA is the right mechanism for update decisions, because an update is
exactly a step with a verifiable outcome that the system can observe
without a human: did the computer stay healthy, was it restored, did the
work it was doing keep working. The pieces below are ordered from
concrete (buildable on the existing record substrate) to speculative.

### 5.1 An update is a precommitment, settled by rules (concrete)

Before an apply, the actor that decides (management under `propose`, the
host actuator under `auto`) commits a record:

- **Hypothesis.** "Update X (lock delta D) leaves this computer working."
- **Typed questions, each with a closed rule kind.** Extending the
  existing vocabulary [V internal/types/commitment_rules.go:72-101]; each
  new kind is a reviewed change, because the rules are the ground truth:
  - `update_healthy_within` (the updater's health identity probe green
    within N minutes, settled from the apply receipt);
  - `no_restore_within` (no restore to a pre-update head within N days,
    settled from the tape);
  - `stack_replays_clean` (the rebase job's receipt);
  - `tests_pass` (exists; the declared test command on the new build);
  - `standing_work_continues` (owner-declared invariants such as
    "Autopaper polls keep succeeding", settled from the desk's own
    receipts).
- **Probabilities and assumptions.** For example: "the stack touches
  only `frontend/src/lib/Slides*`; base delta touches no file in the
  stack; p(healthy) = 0.97".

These settle without a model, from receipts the updater, the tape and the
rebase job already produce. The engineering desk's existing say-do
records on a rebase (`tests_pass`, `paths_confined`, `verify_pass`) apply
unchanged to conflict resolution.

### 5.2 A subscription is a standing precommitment (concrete in shape, needs U5)

Subscribing package P to `stable` with `auto` is the claim "updates from
P@stable will be healthy and not restored, p = 0.95". Each tick resolves
one occurrence. This is the same split CLAUDE.md already makes for
Autopaper schedules: the standing intent survives a crash, and its
occurrences do not. When the track record of a subscription diverges from
its stated probability (say three restores in ten updates against
p = 0.95), management proposes a policy change (`auto` → `propose`, or
pin) to the owner, as a Texture item with the record history attached.
The policy decision itself becomes reviewable from data rather than
memory.

### 5.3 Publisher and channel track records (concrete for releases, extension for packages)

A release record already carries the publisher's engineering
precommitments and their rule settlements
[V internal/release/record.go:54; internal/agentcore/api_release.go:136-160],
so an adopter today can see how well the publisher's engineering
predicted its own change. The extension is adopter outcomes: each
adopter's `update_healthy_within` / `no_restore_within` settlements for a
release or package version. These are the fleet's evidence about that
version.
- **Privacy:** these are computer-private by default.
- **Sharing:** publishing them is an owner act, perhaps as an object in
  the Autopaper object graph [S].
- **Reputation:** this is PSA's reputation leg in the most literal form.
  A publisher whose versions are restored often is a publisher whose
  updates a computer should not take on `auto`.

### 5.4 The host's time-to-patch SLO as a precommitment (concrete, extends the patch report §7.4)

The patch report already proposes committing "advisory X running on
Node B by `fix_available_at` + 48 h, p = 0.9". With U2's exposure report
the same record can be about the fleet, diverged computers included:
"advisory X running on every computer not parked by its deadline". The
diverged computers are where that prediction will miss, and the
calibration gap becomes the measured cost of divergence. It is the number
the owner needs to set the support window and the hold maximum.

### 5.5 In-context RL in the management REPL (hypothesis)

When management decides whether to take an update now, defer it to an
idle window, or propose it, its context pack includes the settled records
of earlier updates on the same channel, publisher and touched paths, and
its own calibration on them. The hypothesis is the PSA core claim
applied to updates: a desk with its scored update history in context
makes better take, defer or propose calls than a fixed rule. It is
testable offline: replay the recorded update outcomes against a static
policy (always `auto`, or `auto` only for floor fixes) and compare
restore rate and exposure time. Until that comparison exists, the static
policy is the default and the records only feed the owner's decisions.

### 5.6 Fleet rings as an information supply (speculative)

LU's rings (LU3a) make early computers the observations later rings'
precommitments resolve against. A ring-0 computer's
`update_healthy_within` settlement is evidence for a ring-1 computer's
prediction. Rings become a structured way for computers to learn in
context from each other's outcomes, which is the "correction by
genuinely independent others" of the foundational argument. Whether
computers with different stacks are independent enough for that
evidence to transfer is unknown.

### 5.7 How records could make update decisions worse

- **Goodhart toward never updating.** An actor scored on `no_restore`
  learns to defer updates. Every update record must therefore score
  exposure time too (time from `fix_available_at` to running), and the
  security subscription's deadline is not the actor's to move.
- **Scores in the actor's own context.** Per doctrine (§4.7 of the
  records document), independent scores stay off the acting desk's
  context or are sampled and delayed. Otherwise management learns the
  scorer's quirks (for example, that `no_restore` is never checked past
  7 days).
- **Small samples.** A computer sees a few updates a week, and a Brier
  score over ten events is noise. Fleet-level evidence (5.3, 5.6) helps,
  at a privacy cost.
- **Anchoring.** A desk that predicted "clean" may under-read a failing
  probe. Rule settlement, not model settlement, is the guard, which is
  why every new kind above is rule-settled.
- **Gaming by a publisher.** A publisher can only game records settled by
  receipts it does not control (adopter-side health, the tape). Its own
  say-do records are informative but not sufficient.
- **Sandbagging.** A desk may commit only to trivially true predictions
  ("boots in 3 s, p = 0.99") to earn autonomy. The guard is
  materiality: the required questions per update class are fixed by the
  rule vocabulary, not chosen by the predictor.
- **Records score; they never gate the floor.** The panel was unanimous
  that prediction accuracy must not itself authorize security-sensitive
  execution. Calibration may widen or narrow *rollout size and review
  depth* for stock and package updates. It never decides whether a
  security update applies, and never admits a change to the security
  evidence class.

---

## 6. What the divergent panel added

**The run.** One divergent panel ran on §4's three candidates, with
lenses architect, security, operator, skeptic, historian and newcomer.
Six seats ran and all six answered: claude (opus), devin, codex,
gpt-6.1-sol, gemini-3.8-flash and deepseek-v4.1-flash. The prompt
carried the verified facts of §1 but no repo access. Raw outputs are in
the session scratchpad, not the repo. The panel's outputs are leads; each
point adopted below was checked against code or against this document's
own reasoning.

**Families of options, clustered:**

1. **Artifact plane against policy plane.** This was the panel's main
   axis.
   - Five of six proposed security by *capability attenuation*:
     leases or quarantine tied to a security epoch (egress, ingress,
     publication, credentials), leaving the diverged code alone.
   - The disagreement is real and irreducible: is contained the same as
     patched?
   - gpt-6.1-sol's framing is adopted: a security policy names its
     *required outcome* (patched, contained or stopped), because these
     are different guarantees. (§4.2 step 6, Q3.)
2. **Toolchain/module rebuild without rebase** (claude, gpt-6.1-sol). Most
   Go CVEs are in the stdlib or modules, so the computer's unchanged
   source is rebuilt with a fixed toolchain or overlay. Adopted as
   transform (i).
3. **Divergence behind an extension boundary** (claude, devin, deepseek,
   codex): plugins, Wasm or an ABI'd supervisor with extensions, so
   security code is never forkable.
   - This is the long-run direction for apps (packages) and agrees
     with the overlay lesson.
   - For the Go core it is a large re-architecture. Not adopted now.
4. **Intent replay or regeneration** (four of six). On conflict, a
   capsule re-implements the change from its recorded intent rather than
   rebasing the diff.
   - Useful as engineering's tool for resolving a conflict.
   - Rejected as an automatic path: the code differs from what was
     supervised, and the result is non-deterministic.
5. **Shadow builds and twins** (deepseek, gemini, devin). Pre-build every
   diverged stack against new bases continuously, or run a patched twin
   VM beside the diverged one. The pre-build is adopted as *shadow
   rebase* (§4.2 step 3). Twins cost twice the memory on one host and
   are not adopted.
6. **Distributed builders** (devin: reproducible-build escrow across
   computers). This conflicts with owner ruling §16 (the host is the only
   builder). It is recorded, not adopted.

**Sharpest failure modes, adopted into §4:**

- **Stock cannot read the diverged state** (four of six). This led to
  the data-compatibility check before parking (§4.2 step 6).
- **A clean rebase is not remediation** (gpt-6.1-sol, codex, deepseek,
  devin). This led to the advisory-specific check (§4.2 step 4).
- **The security class as a supervision bypass** (devin). This led to
  the rule that the class covers only deterministic transforms of
  host-signed fixes (§4.2 step 4), with conflict resolutions going
  through the computer's own gate (step 5).
- **The rebuild thundering herd** (gemini, claude). This led to shadow
  rebase.
- **Restore claimed identical when it is not** (gpt-6.1-sol, codex,
  deepseek), and monotone facts such as credential rotation (claude,
  devin). Restore became a new transaction referencing the old head
  (§4.2 restore).
- **C's static-binary problem** (all six): independent packages do not
  exist inside one Go binary. This matches §4.3 and the hybrid.

**Owner away, conflict, critical fix.** The panel's defensible answers
disagree, and that disagreement is the decision the owner has to make:

- park and run patched stock;
- contain, keeping the owner's code;
- backport only the security hunk;
- suspend or stop.

devin's and claude's "agency wins" cases point out that the conflicting
patch may be the very one that removed the vulnerable code, so stock
could be *more* exposed. The advisory check covers that case: it runs on
the diverged build too.

**PSA.**
- **Agreed better:** calibrated per-update predictions settled by
  receipts can route rollout size and review depth, and owner restores
  become free labels.
- **Agreed worse:** sandbagging toward trivial predictions; self-graded
  settlements; a malicious package's test harness inflating trust; and
  emergency deadlines leaving no time to predict.
- **Unanimous:** records must never authorize security-sensitive
  execution (§5.7).

---

## 7. Open questions for the owner

1. **Support window.** How many platform bases receive backported
   security fixes before a computer must move base: three releases, 30
   days, or none (every security fix is a base move, the simplest)?
2. **The floor list.** Is the floor exactly guest kernel, guest
   userspace, updater, capsule broker and boot wrapper, or should the
   event appender and the kernel module surface (desk capabilities) be
   floor too? Code in the floor can never be self-developed.
3. **Critical fix, owner away, conflict unresolved past the deadline.**
   Which outcome is the default, and does the answer differ for
   Autopaper's always-on computer?
   - park the stack: patched stock, only if stock can read the state;
   - contain: keep the code, cut egress, publication and credentials;
   - stop the computer.

   My lean: park when the data check passes, otherwise contain.
4. **Default policies for a new computer.** Security `auto` and platform
   `propose`? Or platform `auto` until the first self-development change,
   then `propose`?
5. **Hold maximum** on a package security minimum before the package is
   disabled. Snap uses 60 to 90 days; for a CVE, days.
6. **Adopter outcome sharing.** Should adopters' update settlements be
   publishable (owner act) to feed publisher reputation, or stay
   computer-private?
7. **What a published release carries once applies compound.** Should
   it carry the operation's own patch against the publisher's
   computer-local commit, the cumulative patch against the platform
   base, or the whole stack? The cumulative patch is the smallest change
   to `release.Record`. The stack composes better for adopters who
   already carry part of it.

---

## Sources

Prior art was read on the web on 2026-10-24, through a research subagent
that fetched each URL. Where a page did not state something, the claim
above is marked as an inference.

- [P1] Dolstra, *The Purely Functional Software Deployment Model* (PhD thesis), https://edolstra.github.io/pubs/phd-thesis.pdf
- [P2] Nix manual, profiles, https://nix.dev/manual/nix/stable/package-management/profiles.html
- [P3] NixOS manual (generations, `nixos-rebuild switch --rollback`, channels), https://nixos.org/manual/nixos/stable/index.html
- [P4] nixpkgs manual, overlays, https://nixos.org/manual/nixpkgs/stable/
- [P5] `nix flake` reference (flake.lock), https://nix.dev/manual/nix/stable/command-ref/new-cli/nix3-flake.html
- [P6] nixpkgs CONTRIBUTING.md (backports, staging, mass rebuilds), https://raw.githubusercontent.com/NixOS/nixpkgs/master/CONTRIBUTING.md
- [P7] Guix manual, Security Updates (grafts), https://guix.gnu.org/manual/en/html_node/Security-Updates.html
- [P8] Guix blog, "Timely delivery of security updates" (2016), https://guix.gnu.org/blog/2016/timely-delivery-of-security-updates/
- [P9] Guix blog, "Grafts, continued" (2020), https://guix.gnu.org/en/blog/2020/grafts-continued/
- [P10] Android, A/B (seamless) system updates, https://source.android.com/docs/core/ota/ab
- [P11] Android, APEX file format, https://source.android.com/docs/core/ota/apex
- [P12] Debian security FAQ, https://www.debian.org/security/faq
- [P13] unattended-upgrades default config, https://raw.githubusercontent.com/mvo5/unattended-upgrades/master/data/50unattended-upgrades.Debian
- [P14] Debian wiki, UnattendedUpgrades, https://wiki.debian.org/UnattendedUpgrades
- [P15] TUF specification, https://theupdateframework.github.io/specification/latest/
- [P16] Fuchsia, packages, https://fuchsia.dev/fuchsia-src/concepts/packages/package
- [P17] Fuchsia, package URLs, https://fuchsia.dev/fuchsia-src/concepts/packages/package_url
- [P18] Fuchsia, software update system, https://fuchsia.dev/fuchsia-src/concepts/packages/software_update_system
- [P19] Fuchsia, OTA, https://fuchsia.dev/fuchsia-src/concepts/packages/ota
- [P20] git-rerere, https://git-scm.com/docs/git-rerere
- [P21] quilt(1), https://linux.die.net/man/1/quilt
- [P22] Stacked Git, https://stacked-git.github.io/
- [P23] Pijul manual, theory, https://pijul.org/manual/theory.html
- [P24] Understanding Darcs, patch theory, https://en.wikibooks.org/wiki/Understanding_Darcs/Patch_theory
- [P25] Cargo, Cargo.toml vs Cargo.lock, https://doc.rust-lang.org/cargo/guide/cargo-toml-vs-cargo-lock.html
- [P26] Russ Cox, "Minimal Version Selection", https://research.swtch.com/vgo-mvs
- [P27] Bazel, modules (bzlmod), https://bazel.build/external/module
- [P28] Bazel, lockfile, https://bazel.build/external/lockfile
- [P29] Flatpak, basic concepts and runtimes, https://docs.flatpak.org/en/latest/basic-concepts.html and https://docs.flatpak.org/en/latest/available-runtimes.html
- [P30] Snapcraft, channels, https://forum.snapcraft.io/t/channels/551
- [P31] Canonical, managing snap updates, https://canonical.com/blog/how-to-manage-snap-updates

Not covered tonight: ChromeOS autoupdate, Uptane, in-toto. Not settled by
the sources: whether a Snap hold yields to a security refresh, and APEX
rollback.

## Class, heresy delta, residuals

- **Class:** green (this document). No runtime, test or prompt change.
- **Heresy delta:**
  - `discovered`:
    - F3: a platform deploy silently drops a computer's own Go change,
      with no tape event, leaving a mixed new-runtime / old-frontend
      state.
    - F4: the floor and the runtime share one identity, so a kernel
      patch invalidates every layer.
    - F2 restated as a security defect: a flagged computer gets no
      floor updates.
    - H1: a possible genesis-frontend freeze, unconfirmed.
    - F5a, the main one, is discovered by the coordinator, not by this
      document: successive applies do not compound; it has its own
      record.
  - `introduced`: none.
  - `repaired`: none.
- **Problem documentation first:** F5a already has a record. F3, F4 and
  (if confirmed) H1 each need a problem record before any U-slice code
  lands. This document is design evidence, not that record.
- **Residuals:**
  - **R1.** The frontend half of the mixed state in F3 has not been
    observed on staging.
  - **R2.** The ABI contract (U3) needs an inventory of what the runtime
    actually takes from the base (env, units, paths, protocol versions).
    None was done tonight.
  - **R3.** The guest repo was cloned from the host's `/opt/go-choir` at
    creation [V internal/vmmanager/manager.go:3112-3116]. It may not
    contain the objects of later platform commits, so moving even an
    empty stack to a new base needs the host to supply them (a git
    bundle at boot, or U4's host-side rebase). Unverified which commits
    a long-lived guest repo holds.
  - **R4.** Publish/adopt after U1 needs Q7 decided.
  - **R5.** Backport-pocket cost is unmeasured, and so is the number of
    distinct bases in the fleet today.
  - **R6.** The CVE mix (toolchain/module against Choir code) that
    makes transform (i) the common case is a hypothesis.
