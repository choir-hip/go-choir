# Forced modularity: build design (2026-11-04)

Status: build design. Green (this document). The slices below are orange and
red; each names its own class. Owner rulings of 2026-11-04 are settled and
quoted; this design implements them and does not re-argue them. Read first:
[S12](s12-runtime-install-build-2026-11-04.md),
[S6](s6-package-lock-build-2026-11-04.md),
[the Library security intent](library-app-store-security-intent-2026-11-04.md),
[the modularity check](modularity-check-build-2026-11-04.md), and the
remat-serve problem record on branch `remat-serve`
(`docs/problems/rematerialized-stack-never-served-2026-11-04.md`, worktree
`/Users/wiz/Dev/go-choir-remat-serve`, in review). Standing questions
(`docs/standing-questions.md`) are answered at the end.

## Rulings

1. "We need to get rid of adopt because the library install, that's the way
   that it works. Adopt was just an early iteration of library install."
2. "I want automatic rebase to work the same. I don't want test computers to
   work differently than other computers. Then that makes them not a good
   test." And: "If it doesn't work correctly, we need to fix it."
3. "Key is that we need to have forced modularity. The base is the base, and
   apps are a separate layer. We need to enforce that." The key priority.
4. "We need forced updates for sure." Refined twice. First: "No, publisher
   cannot force an update to their app. That is itself a security issue.
   Right? ... forced updates definitely have to go through the platform, for
   sure." Then: "I feel like forced updates are something that is a break
   glass emergency thing that I would be doing manually. ... I want almost
   everything automatic except for forced update."
5. The Library's Base image tab is removed; the base is operated through
   code. Another builder owns that change. Nothing here depends on it except
   that the Library keeps no base surface.

The target model, in one line: **main updates the base, the Library updates
apps, nothing else changes a computer.**

## Current state, verified

An app reaches a computer three ways today. The second is the defect.

**A. Bundled in the base.** `frontend/src/packages/` on main holds three
packages: `image`, `library-next`, `reader`. Their manifests say
`origin: first_party` and ABI `choir.app-frontend/svelte-inline/v1`
(`frontend/src/packages/image/choir-package.json:5,13`;
`frontend/scripts/check-packages.mjs:21`). The shell build bundles them.
The modularity gate already builds each of them a second way, as a
`module/v1` installed module through the real loader
(`docs/design/modularity-check-build-2026-11-04.md`, section "What modularity
means here", item 3). Other first-party apps are not packages: they are
`frontend/src/lib/<Name>App.svelte` components, which the platform itself
treats as apps (`internal/agentcore/engineer_lineage.go:46-47,55-60`).

**B. As app source in the computer's source stack.** An engineering capsule
edits `/workspace/platform`; `collectSourcePatchFiles` renders every file
under it into `source.patch`
(`internal/capsule/source_patch.go:53`), with no path policy beyond
`sourceRelOK`, which only refuses `.` and `..` segments
(`internal/capsule/overlay_deletions.go:54`). The host build refuses recipe,
git metadata and store-schema paths and nothing under
`frontend/src/packages/` (`internal/hostbuild/types.go:202-223`, authority
`AdmitPatchedTree` at `:229`, early check `service.go:392`). So app source
rides the stack and the served build. A computer that builds an app serves it
this way. Publishing then starts from the applied stack operation:
`packageReleaseDraft` requires the operation to be `applied` and reads its
bound source patch (`internal/agentcore/api_package_release.go:34-80`).
Confinement exists in one direction only: a release draft refuses paths
outside its package root (`package_change_not_confined`, same file `:29`;
`pkgrelease.OutsideRoot`, `internal/pkgrelease/publisher.go:55`).

**C. As an S12 runtime-installed module.** `installPackage` verifies the
release, builds `module/v1` on the computer, stores it by digest and writes
an install record (`internal/agentcore/package_install.go:111`). The shell
reads `GET /api/packages/installed` (`internal/agentcore/package_runtime_api.go:23,73`)
at sign-in and on mount (`frontend/src/lib/Desktop.svelte:165,1662`) and
mounts through `PackageAppFrame` (`frontend/src/lib/apps/PackageAppFrame.svelte`,
`packages.ts`, `registry.ts`). This path never touches the stack.

**S6 exists only because of B.** `internal/sourceproj/packages.go` gives each
`frontend/src/packages/<name>` the stack touches its own entry and state
(`PackageRootOf` `:61`, `packageGraft` `:126`) so that an app in the stack
does not strand the shell on a base move. The remat-serve problem record
shows the cost: a stack holding `library-next` yields
`package_behind_base` with `conflict: true`, and its sibling roots
`package_base_moved`; that stack is `stack_rematerialized` and was never
served. S12 itself says it "takes Library installs out of the stack" and that
S6 and S12 "touch no file in common"
(`s12-runtime-install-build-2026-11-04.md`, "How S12 meets S6"). Forced
modularity removes the last reason S6 has.

**The names.** "Adopt" survives in four layers:

- the proxy: route case `"adoptions"` (`internal/proxy/self_development.go:53,77,94`),
  the method-to-action mapping that turns a POST into action `"adopt"`
  (`:459`), the scope and owner-act arms (`:473`, `:497`);
- the guest: `handleReleaseAdoptions` and `adoptReleaseRequest`
  (`internal/agentcore/api_release.go:69-132`,
  `package_install.go:33-92`), the route parser
  (`internal/agentcore/api_self_development.go:264-268`);
- the store: `store.ReleaseAdoption` with `Mode` `runtime` or the legacy
  source mode (`internal/store/release_adoption.go:48-107`), the object kind
  `choir.release_adoption` (`internal/objectgraph/registry.go:128`) and two
  field kinds (`:267-268`), and the S6 provenance read of source-mode
  records (`internal/agentcore/source_projection.go:591-608`);
- the clients: `cmd/choir/package.go:154-180`, the Library
  (`frontend/src/packages/library-next/catalog.js:104-114,308-310`,
  `App.svelte:11,204,284,460,907`), `scripts/choir-shot.mjs` and its test,
  CI text, and docs.

Not the same thing, and kept: `vmctl`/`vmmanager` "adopt a surviving VM
process" (`internal/vmctl/ownership.go:367`,
`internal/vmmanager/manager.go:1597`), and the CI "first adoption" comment
(`.github/workflows/ci.yml:572,589`). The CLAUDE.md phrase "computers adopt
onto their own tapes" describes the Autopaper object graph; it is a different
act and is out of scope here (see Decisions, D4).

## Design

### 1. Where an app lives

An app exists on a computer only as an installed module: an install record on
the tape, a module in the module store, a launcher entry. This includes the
computer that built it.

**Self-development that changes an app is a package operation, not a source
operation.** Today a package change goes capsule, freeze, owner approval, host
build, restart, then a publish that needs its own owner-act receipt. New: the capsule edits
`frontend/src/packages/<name>/` as it does now (the capsule keeps its full
workspace so it can build and test against the real shell), but when the
frozen source patch touches package paths the operation is a package
operation:

- a patch is *package-only* (every path under one root
  `frontend/src/packages/<name>/`) or *platform-only* (no package path). A
  mixed patch is refused at freeze with the split named (the engineer cast
  two operations). One operation, one layer.
- a package-only operation has no host build, no restart, no stack entry. Its
  verifiers are the package checks the host build already runs
  (`scripts/check-packages.mjs`, the boundary plugin, the contract runner
  `frontend/scripts/package-contracts.mjs`) plus a `module/v1` build.
- on acceptance the operation's effect is: publish the release (the existing
  draft and `publish` path), then install it locally (`installPackage`). Both
  steps are existing code. The publish act is what makes the release a
  record the installer verifies byte for byte, the same way every other
  installer does; the local install is not special.
- the operation reaches `applied` with the install record as its effect, so a
  restore to a pre-install head folds to a computer without the app
  (`s12-...md` D4).

The source stack carries only platform operations. It never holds a path
under `frontend/src/packages/`.

**Nothing waits on an owner act (owner, 2026-11-04):** "I don't want to
approve each version!!! The whole point of library is so I don't have to
approve. Builds in test computers can be installed. We shouldn't block on
human approval where possible." A package operation is decided by automated
acceptance, and the owner is never asked:

- the Freeze gates: the screens, and for an update the before and after pair
  (referenced in section 1a, built by another builder);
- the shot review of those screens;
- the package contracts, the modularity runner in both modes (bundled
  in-tree and installed `module/v1`), plus `check-packages.mjs` and the
  boundary plugin;
- the verifier and consensus decision (`decisionpolicy`, corpusd as the
  decision poster, `internal/platform/standing_consensus.go`).

On a pass the release publishes, installs locally and the candidate overlay
is removed. This no-approval path applies to **the builder's own computer
only**: nobody approves a version on the computer that built it. Other
installers do not get the update by this path; each follows their own update
setting (section 5). On a fail nothing publishes and the installed version
stands. A build made on a test computer publishes and installs like any
other.

*Which decision mode applies, and where the code does not fit.* The modes are
`off`, `audit_only`, `propose_only`, `accept_once`, `qualified_consensus` and
`standing_consensus` (`internal/platform/self_development_modes.go:22-30`).
The closest is `standing_consensus`, the autonomous consensus decision. It
does not fit as written: it needs a verified owner act to arm
(`standingArmRefusal`, `standing_consensus.go:127-139`), only on a test-tier
computer (`:142`), never on a platform computer (`:144`), and it expires within
`StandingConsensusMaxWindow`, 24 hours
(`self_development_modes.go:33`). Publishing is separately gated: the publish
route demands an `ActPackagePublish` owner-act receipt per release
(`internal/platform/packages.go:233`, `owneract.go:81`). Default: package-only
operations use the **`standing_consensus` decision policy as the rule for
every computer whose mode is not `off` or `audit_only`**, with no arm, no
expiry and no tier test (the same removal ruling 2 makes for clean rebase,
R1). The accepted decision, bound to the operation's bundle digest, is the
authority for the publish and the local install, replacing the per-release
owner-act receipt for operations of this kind. Platform operations keep their
current mode behavior; this design changes only how a package operation is
decided. Slice M3 owns the change.

**The publish gate stays; its switch goes.** `packages.publish` is a
computer setting, default on (`internal/store/computer_setting.go:161,248`),
consulted by the draft and media routes
(`api_package_release.go:47`, `api_package_media.go:89`). With publish now the
only way for an app to exist on its builder's own computer, a setting that
can refuse it is a switch that breaks the model. Delete it (the owner
forbids new switches; this one is old, and ruling 3 makes it contradictory).

### 1a. Building a new version of an existing app: the overlay, and before and after

Owner, 2026-11-04: "For anything which is modifying anything that already
exists, like making an updated version of an app, you should take a
screenshot before and after. Or we should think about, while it's being
built, it should be built as like an overlay so that both of them exist. And
then on freeze and publication, that's where it would supplant it. But
regardless, in order to publish, you should need to show before and after."

Both halves fit M3. They are one rule: an update to an existing app never
replaces the installed version while it is being built, and never publishes
without before and after.

- **The candidate runs beside the installed version.** When a package
  operation changes a package the computer already has installed (or ships in
  the base), the capsule's `module/v1` build is installed locally as a
  *candidate instance*: an install record with a candidate marker, a distinct
  launcher name (`<name> (candidate)`), and its own module digest. The
  installed set is already a fold of per-computer records and the shell
  already keys runtime definitions on `install_id|sha256`, so two
  definitions of one package coexist without a remount of the first
  (`s12-...md` D4 and "Review round 1"). Installs are per computer, so the
  candidate is visible only to the owner by construction; no sharing, no
  catalog listing. The installed version is untouched and keeps serving.
  A first version of a new app has nothing to sit beside, so it needs no
  overlay and no "before" screen (it still shows its "after").
- **Freeze and publication supplant.** On freeze the candidate is a frozen
  tree; on acceptance (automated, section 1) the effect is the one in section 1: publish the release
  on the computer's track, install it as the real package (this is what
  replaces the installed version, through the ordinary install-replaces-track
  rule), then remove the candidate instance. The candidate record is never
  a published fact; a restore to a head before the supplant folds to the old
  version, with the candidate gone or present as the tape says.
- **Before and after are required to publish an update.** A publish of a
  release whose parent is an existing release or base package (not the empty
  tree, `pkgrelease.Parent`) must carry before and after screens of the
  package; the host refuses it otherwise. The release already binds up to
  screenshots and one video by sha256 (`internal/pkgrelease/media.go`,
  `Record.Media`, `record.go:84`). The requirement at freeze is another
  builder's work and is referenced here, not designed: M3 only guarantees the
  two things that work needs. The before screen is taken from the installed
  version and the after screen from the candidate, which is why both must
  exist at once; and the draft route reads the package operation's frozen
  tree and its media, not an applied stack operation.

Failure modes: the candidate replaces the installed version early (it is a
separate record and name; the supplant is only the acceptance's effect);
the candidate leaks to another user (installs are per computer and the
candidate is never published); a crash leaves a candidate (it is closed at
boot as "interrupted by a restart", never resumed, and the installed version
stands); a publish of an update with no before screen (refused by the host
and by freeze). Proof on staging, added to M3's: on d@e a candidate of
`image` runs beside the installed `image`, both open in the launcher, the
update refuses to publish without a before screen and publishes with one, and
after acceptance only the new version remains, with no owner act in the
path.

### 2. Enforcement, both directions

Three layers on the stack side, so a path cannot get in by any door:

1. **Capsule (early, helpful).** `collectSourcePatchFiles`
   (`internal/capsule/source_patch.go:53`) already refuses by name
   ("refused, never skipped"). It classifies the whole change set first; a
   mixed set is refused with both lists named. This is where the engineer
   learns the rule.
2. **Host build (authority).** Add `PackagePathRefused` to `refusePath`
   (`internal/hostbuild/types.go:202`): any cleaned path under
   `frontend/src/packages/<name>/` (using `sourceproj.PackageRootOf` logic,
   reimplemented in-package because of the import direction) is refused
   with a reason code. `AdmitPatchedTree` (`:229`) applies it to the tree git
   committed, so no patch spelling hides a path. `RefusePatchPaths`
   (`service.go:392`) gives the early answer. This is the same place
   `internal/storeschema/` is already refused (`types.go:209`).
3. **Stack and corpusd.** `sourceproj` apply/reconcile treats a package path
   in a patch as `ErrPackagePath` (a correctness refusal, like release
   skew). The corpusd acceptance gates that bind a bundle
   (`internal/platform/standing_consensus.go`, `clean_rebase.go`) check the
   bundle's source patch paths too, so a hand-built bundle cannot bypass the
   host. This is the third door; it exists because the bundle is signed by
   the computer and corpusd is the only authority that is not.

On the package side:

4. **A release refuses non-package paths.** Already true at draft
   (`api_package_release.go:29`) and at host verification
   (`pkgrelease.VerifyHost` records the same: "a release can never carry a
   path outside its package root", `internal/pkgrelease/record.go:5-10`).
   What remains: the draft must not depend on an applied stack operation
   (it does today, `:34-80`); it reads the package operation's frozen tree.
5. **A package reaches the base only through the SDK.** Present: the boundary
   plugin (`frontend/scripts/package-boundary-plugin.mjs`, embedded copy in
   `internal/pkgruntime/package-boundary-plugin.mjs`, held equal by a Go
   test), the SDK surface lock
   (`frontend/src/lib/sdk-surface/1.0.0.json`, `frontend/scripts/sdk-surface.mjs`)
   and the per-package contracts (`choir-contract.json`). What remains, from
   the modularity check's own residuals:
   - the SDK global by a computed name (`globalThis['__choir'+'AppSdk']`)
     gets past the boundary plugin. Architectural not security; for the
     module/v1 path remove the `__choirAppSdk` global entirely (the shim
     reads `ctx.sdk`) once the three first-party packages no longer need the
     fallback;
   - the mount `ctx` and props shape are not in the lock; add them;
   - the contract runner is not wired into probation or clean rebase;
   - Go-side layout drift of `pkgruntime` against the gate's replica (N3).
   These are named, not built by this design except the global's removal and
   the `ctx` lock, which are slice M5.

### 3. The bundled first-party packages

Decision: **they are base-shipped first-party packages, loaded through the
same runtime boundary.** `image`, `reader` and `library-next` stay in
`frontend/src/packages/` on main, owned by main. The shell stops bundling
them as `svelte-inline` and loads them as `module/v1` modules the base ships
(built by the frontend build, served beside the shell, verified by digest by
the same loader). Justification from the code:

- the gate already builds every one of them both ways and requires the
  installed path to reach `data-package-state="mounted"` (modularity check
  C9). The loader is proven on them.
- one load path has one set of failure modes, and a base package then proves
  the same contract an installed one does on every deploy. A bundled path
  keeps a second ABI (`svelte-inline/v1`) alive only for them.
- a base-shipped package is the base's: only main changes it. A computer can
  *replace* it with an installed track (`registry.ts` already lets an
  installed package win by name while installed, and a static app is never
  replaced; that rule is kept as "the base copy is the fallback").
- cost: a deploy that changes a package now changes a shipped module, which
  is a base change, so it rides ring walk like any base change.

Not decided here, and listed in D3: the `lib/*App.svelte` first-party apps
are not packages. They stay base code. The refusal in section 2 covers
`packages/*` only, so an engineer can still edit `lib/SlidesApp.svelte` into
the stack. That edit is a *platform* operation and subject to the platform
path's cadence. Converting them to packages is the long-run work that makes
forced modularity complete; until then the rule is "package paths cannot
enter the stack", not "no app code can".

### 4. Migration at the cutover

Downtime is acceptable and there are no dual paths. At the deploy that lands
M2:

1. Boot-time fold (a one-shot at the migration, no resumer): for each
   computer whose applied stack's cumulative patch touches
   `frontend/src/packages/<name>`: (a) if the stack's subtree equals a
   listed release's tree (`annotatePackageReleases` finds it,
   `source_projection.go:591`; legacy source-mode install record), install
   that release at runtime; (b) otherwise draft, publish and install a
   release of the subtree on that computer's own track (the computer's
   `packages.publish` act is performed by the platform as the migration, with
   the migration's own decision record; no per-release owner act); (c) then remove the package paths from the stack by composing
   a new platform-only stack from the operations' non-package hunks.
2. A computer whose stack is package-only becomes a stack of nothing: its
   served build is the booted base. Its apps run as installed modules.
3. Test arms and the test computers are migrated first (d@e and the arms),
   then the owner's computer. Which computers hold app source is not read
   from code; the migration reports it from `/health`
   (`source_projection.packages`) and `sourceproj.Result.Packages`.
4. The migration is verified by the differential: the launcher lists the same
   apps before and after; the served shell build equals the booted base.
5. `store.ReleaseAdoption` source-mode rows are converted to install records
   with `mode: runtime`; the legacy "Earlier source install" rendering and
   `Mode` field are deleted with them. Keep-set: the owner's install history
   survives as install records.

### 5. Updates

**Base: main, by ring walk.** Unchanged.

**Apps: the Library, and a per-user update setting.** Owner, 2026-11-04: "No,
I don't think upgrades get installed automatically. That should be a per user
setting. But people will want to approve upgrades manually... and there's
security issues on both sides. Hmm, this is complex but we can't just rug
users with forced updates except for security." So an installer other than
the builder chooses:

- **manual (the default).** When a newer listed release of the same track
  exists, the Library shows "Update available" with what changed (the
  release's review patch and its before/after screens), and the user presses
  Update. The press is the existing install path (`installPackage`): it
  builds offline on the installer's computer and keeps the old module until
  the new one stores.
- **automatic.** At the next load or refresh the computer installs the newest
  eligible release of the same track through the same path. A release is
  eligible only after the **soak**: listed for 24 hours (a constant, not a
  setting). The soak is the default and applies **only to users who chose
  automatic**. It never asks anyone to approve anything, it does not delay a
  publish, and a manual user is not subject to it (they decide when).
- A track change (a different publisher) is never an update in either mode;
  it is an install.

Where the setting lives: a **user preference**, not a platform switch and not
a computer setting. Preferences are owner-scoped key and value rows that
are written as events on the user's tape and projected
(`store.SaveUserPreference`, `internal/store/media.go:153`;
`projectUserPreference`, `internal/store/media_projection.go:116`; the
shell already reads one at `/api/preferences/theme`,
`internal/apihandler/routes.go:43`). The key is `packages.update_policy`,
value `{"default": "manual", "apps": {"<package>": "automatic"}}`: a default
and a per-app override, so a user can choose per app and one global toggle
is not the only shape. Absent means manual. The Library offers the choice on
each installed app and one default. The guest reads it when it compares
installs with track heads; an unreadable preference is manual.

Security on both sides, which is why neither mode is the whole answer:

- *automatic* lets a compromised or malicious publisher account reach every
  automatic installer at their next load. The soak and the platform yank
  narrow that window; they do not close it.
- *manual* leaves users on a known-vulnerable version when a release fixes a
  security flaw and they do not press Update, or press it late.

The remedy for both is the same single override: the platform's break-glass
force or yank (below). It is the only thing that overrides a user's manual
choice, and it never runs automatically. Everything else respects the setting.
Nothing in this section forces an update on anyone.

**Forced update and yank: platform only, manual, break glass.** The ruling
excludes every other design:

- the publisher cannot force. A compromised publisher account that could
  force a release onto every installer is exactly the supply-chain attack the
  security intent exists to guard against (`library-app-store-security-intent-2026-11-04.md`,
  "Remediation": "yank or force-update the release", by the platform). A
  publisher can publish a new release; installers take it by pressing Update (manual)
  or by the automatic path above, after the soak. A publisher can *request* a forced update by
  writing a note; the request only notifies the platform owner and executes
  nothing.
- no automation triggers it: no scanner, no threshold, no publisher call.

The act: one authenticated owner act by the platform owner, a CLI or API
call, no GUI. Who "the platform" is in code: there is no platform-admin role
in the package routes today (listing acts are owner acts bound by a receipt,
`internal/platform/packages.go:239-269`; the yanked state is already written
as "set aside by the security authority" with no writer,
`packages_share.go:176`, `pkgrelease/listing.go:187`). The precedent for a
manual platform act is the key-escrow operator gate in corpusd: a host
transport, the host credential, and a named operator with a token compared in
constant time (`internal/platform/key_escrow_http.go:296-318`, named
operators from `CHOIR_KEY_ESCROW_OPERATORS`). Default: **the yank/force
route is a corpusd route behind the same gate** (a separate operator list,
`CHOIR_PLATFORM_OPERATORS`), reached by `ssh` to the host or the CLI through
the host transport. It requires a reason. It is the only route that can write
state `yanked`.

What it records: a new listing version in the release's chain, state
`yanked` or a new `forced` marker, `state_reason` the operator's reason,
`state_actor` `platform:<operator>`, chained by `prev_version_digest` like
every listing (`listing.go:50-56`). That chain is the record; each installer
also writes a tape record (`install` with `forced: true` and the listing
version it acted on) so the owner sees it.

What installers do, at their next load or refresh, no choice and no switch:

- yank: the computer uninstalls the release (uninstall never depends on the
  catalog, `s12-...md` D6) and, if the track has an eligible head newer than
  the yanked release, installs it, otherwise the package's base copy shows
  (or the launcher entry is removed). A yanked release is also unavailable to
  new installs (the proxy already refuses a yanked or withdrawn release).
- force-update: the computer installs the named release, even over the
  soak; the old release is uninstalled.
- the owner sees it on the Library's installed list: "Removed by the
  platform: <reason>" or "Updated by the platform: <reason>", with the
  release it moved to, at the next open. Their data is not deleted.

Yank is the existing state, force-update is the same act with a target
release; both are one route. A forced update never changes the base.

### 6. Clean rebase on every computer

Ruling 2: remove the tier restriction. Two places, both written for ring R1:

- `cleanRebaseEligible` in the guest returns false for a platform computer
  and for any computer not in `computertiers.TestComputerIDs()`
  (`internal/agentcore/clean_rebase.go:83-100`; the comment at `:49`;
  the field `cleanRebaseTier` in `runtime.go:484`).
- the corpusd gate refuses `platform` and not `test`
  (`internal/platform/clean_rebase.go:109,112`), through
  `computerTierForCleanRebase` (`:74-81`) and `SetCleanRebaseTier`
  (`:60-65`), also `owner_acts.go:119`.

The mode allowlist at `internal/platform/clean_rebase.go:135` admits
`accept_once`, `qualified_consensus`, `standing_consensus`, `propose_only`
and refuses `off` and `audit_only`. Under ruling 2 the question is whether
those two refusals stay. They do: an `off` or `audit_only` computer has
turned self-development off or to observe; a rebase there is a platform
mutation of a computer that declined mutation. That is a correctness
refusal. Everything else about the mode set is unchanged.

**Build:** delete the tier branch in both places and the tier file's
`TestComputers` role for this purpose (`computertiers.TestComputerIDs` keeps
its retention-rule role; the clean rebase stops using it). Delete the test
hooks `cleanRebaseTier` and `SetCleanRebaseTier`; tests drive the real gate
with a real computer. Build on the remat-serve branch: its
`rebaseStackBase` (`internal/agentcore/rebase_notice.go`, new on that branch)
makes a `stack_rematerialized` stack served on an old base reach the clean
rebase, with the package-conflict case sent to the notice and an open
operation holding the pair.

**Does the platform computer take it?** The ruling says the test computers
must not work differently from "other computers". Default: yes, the
platform computer (the paper) takes the clean rebase too, because its
alternative is the owner act, which it never gets (a computer that is always
busy, CLAUDE.md "Restarts End Work"). It is subject to every correctness
refusal, and `critical_protected` and `public_platform` computers use the
planned marker-backed refresh already built for them. D5 flags this.

**What still refuses:** crash (a crash restart closes an attempt as
"interrupted by a restart" and sends the pair to the notice,
`clean_rebase.go:213-250`), live capsule (the open-operation hold), release
skew (`cleanRebaseJobs.DeployedBase`), a rebase notice already minted for
the pair, a patch that does not apply (`sourceproj.Reapply`), a failed host
build, a stack on a moved base that is not the applied operation's base.
One attempt per (operation, booted base).

**Risk on the owner's computer.** The clean rebase applies a platform change
over the owner's own change with no owner act. The proof is mechanical: the
patch applies unchanged and the host build passes. A merge that applies,
builds and behaves wrongly is not caught (the file header and the modularity
check's seam both say so). The contract runner, once wired into the clean
rebase after the host build (`package-contracts.mjs --dist ...`, as the
modularity doc's "Where else the runner runs"), would turn the build into a
behavior smoke; that wiring is slice R2. A restart follows the materialize
(planned, marker-backed).

**How much rarer.** With apps out of the stack, the stack holds platform
changes only. The pool evidence shows the common cause of a moved base is a
deploy that touches packages: `library-next` at `package_behind_base`,
`files-next` and `slides-next` at `package_base_moved`, the shell untouched
(remat-serve problem record, "Observation"). Those entries vanish. What
remains are platform self-development changes, which by count are a small
fraction of a computer's work; the rebase is attempted only when a deploy
moves a path the stack also moved. I did not measure the share on the fleet;
that is the first thing the migration's `/health` pass reports (the count of
stacks with package entries versus shell-only entries).

### 7. Deletions

Deleted, with no shim:

- **Adopt, everywhere.** Proxy `case "adoptions"` and the `"adopt"` action
  and scope arms (`internal/proxy/self_development.go:53-57,77,94,459-462,473,497`)
  become `installs`: `GET/POST /api/computers/<id>/packages/installs[/<id>]`,
  served by the guest as an install route, not under `self-development`
  (installs are not self-development operations). The guest route parser
  (`api_self_development.go:264-268`), `handleReleaseAdoptions`
  (`api_release.go:69-132`), `adoptReleaseRequest`, `adoptionRefusal`,
  `adoptionKindPackage`, `adoptionView`, `ReleaseAdoptionView`
  (`package_install.go`), all renamed `install`. Store: `ReleaseAdoption`
  becomes `PackageInstall`, kind `choir.package_install`; the legacy source
  mode and `ReleaseAdoptionModeRuntime` go (every row is runtime after the
  migration); `objectgraph/registry.go:128,267-268` updated with a one-shot
  kind rename in the migration, since the registry and the table both name
  the kind. `source_projection.go:591-608` (source-mode provenance) is
  deleted. `cmd/choir/package.go:154,156,180`, the Library
  (`catalog.js`, `App.svelte`), `scripts/choir-shot.mjs` and its test,
  `internal/agentcore/capsule_install_fixture_test.go`,
  `internal/proxy/package_install_test.go`, and every doc that names the
  route are updated. The `adoption`/`adoption_id` query cleanups in
  `frontend/src/App.svelte:292-293` go. The release-adopt text in
  `agentcore/runtime.go:448-452` and `effective_release.go:25,33` is
  rewritten; those P6/P8 docs are archived already.
- **S6 package containment**: `internal/sourceproj/packages.go`
  (`packageGraft`, `packageEntries`, states and `PackageEntry`), its use in
  `reconcileStack`, `Result.Packages`, the `package_behind_base` warning in
  the Cast gate, `packagelock_test.go`, `source_projection_packages_test.go`,
  `publisher` draft's `graftBefore` `package_parent_unpublished` branch, and
  the S6 doc status line. `PackageRootOf` survives as the refusal's
  classifier (moved to a small `internal/packagepath`).
- **Source-patch app flow**: the `package_change_not_confined`
  "earlier unpublished rounds of this stack" machinery, which exists only
  because package changes ride the stack.
- **`packages.publish` setting**.
- **`svelte-inline/v1` for first-party packages** (after M4): the ABI string
  in `check-packages.mjs`, the bundled-package path in the shell registry.
- **Tier use in clean rebase**: `cleanRebaseTier`, `SetCleanRebaseTier`,
  `computerTierForCleanRebase`, and the refusal strings.
- **Legacy install UI**: "Earlier source install", `same_base` field
  (`s12-...md`, "Review round 1").
- **The Base image tab** (ruling 5; the other builder).

## Slices

Each slice is by package boundary. None uses a switch; rollout is by ring
(d@e, then the rest), rollback is a revert. Proof is a product path on
staging: a test that runs through the proxy, the guest and the shell, or a
deploy's own `/health`, never an ad-hoc screenshot. The data-system rules
apply: runtime checks are the verifier.

**M1. Install is the only name (orange, tier 1, Devin).**
Rename adopt to install across proxy, guest, store, CLI, Library, fixtures,
docs; one-shot kind rename in the migration. Packages: `internal/proxy`,
`internal/agentcore`, `internal/store`, `internal/objectgraph`, `cmd/choir`,
`frontend/src/packages/library-next`, `scripts`. Staging proof: the Library
installs and uninstalls through the new route on d@e and the install list
shows the pre-existing records; the old route answers 404 (`TestProxy...`
loops over listed, withdrawn and gone releases as S12's do). Rollback: revert,
which restores the old route and kind.

**M2. The stack refuses package paths (red, tier 2: Devin and DeepSeek).**
Capsule classifier, `hostbuild.refusePath`, sourceproj apply, corpusd bundle
check. Packages: `internal/capsule`, `internal/hostbuild`, `internal/sourceproj`,
`internal/platform`. The wall goes up before the migration needs it, but the
migration (M3) must land with it, because a computer whose stack holds
package source would otherwise be refused at reconcile. So M2 and M3 deploy
together. Staging proof: a capsule on d@e edits a package and a shell file in
one change and freezes refused with both lists; a hand-made bundle with a
package patch is refused at corpusd; the host build refuses it at
`AdmitPatchedTree`. Rollback: revert; the stack accepts package paths again.

**M3. Package operations, the candidate overlay and the migration (red, tier 2).**
Package-only freeze: no host build, no owner act; automated acceptance (Freeze
gates, shot review, package contracts, consensus) decides, and on a pass
publish then install is the effect (decision mode: section 1, `standing_consensus`
policy without arm, expiry or tier); draft reads the frozen tree; candidate overlay and before/after screens for updates (section 1a); boot one-shot migration; `packages.publish`
setting deleted. Packages: `internal/agentcore`, `internal/capsule/transaction`,
`internal/selfdev`, `internal/platform`. Staging proof: on a test arm, an
engineer builds an app change end to end with no owner act; the operation reaches `applied`
with an install record, the launcher shows the app, a base deploy follows and
the app is untouched with no restart (S12's "staging acceptance", currently
deferred). Migration proof: the differential on every computer that holds
package entries today, and the served build equals the booted base.
Rollback: revert and restore the pre-migration snapshot; the migration is
the one irreversible step and it runs once per computer, owner-visible.

**M4. Base packages load as modules (orange, tier 1).**
Build the three first-party packages into shipped modules; the shell loads
them through the runtime loader. Packages: `frontend`, `internal/pkgruntime`
(shared build scripts). Proof: the modularity gate in installed mode becomes
the shell's real path; `frontend/tests/package-modularity.spec.js` and the
contract receipts on CI; staging opens each of the three. Rollback: revert.

**M5. SDK residuals (yellow/orange, tier 0 or 1).**
Remove the `__choirAppSdk` fallback; lock the mount `ctx` and props shape;
wire the contract runner after the clean rebase host build and into
installation probation. Packages: `frontend/scripts`, `frontend/src/lib`.
Proof: negative tests in the gate (a package that reads the global fails).
Rollback: revert.

**R1. Clean rebase on every computer (red, tier 2).**
On top of the remat-serve branch: delete the tier gates in the guest and in
corpusd; keep mode and correctness refusals. Packages: `internal/agentcore`,
`internal/platform`, `internal/computertiers`. Proof: e2e on a non-test
computer fixture (the existing `clean_rebase_e2e_test.go` runs with the real
gate and a fixture outside the tier file); on staging, a deploy that moves
a platform path on d@e's stack results in a clean rebase and a restart; a
conflicting deploy results in the notice. Rollback: revert, which restores
the tier gate. Order: R1 can land before M2 to M5; it needs only remat-serve.

**F1. Update policy, "Update available" and the platform yank (red, tier 2).**
The `packages.update_policy` user preference (default manual, per-app override),
the Library's "Update available" with what changed and the Update press, the
soak and the load-time update for users who chose automatic, the corpusd operator route, listing writer,
install record fields, the Library line. Packages: `internal/platform`,
`internal/pkgrelease`, `internal/agentcore`, `cmd/choir`, `frontend/src/packages/library-next`.
Proof: on d@e, publish release 2 on a track; a manual installer sees "Update
available" with the change and is not updated until the press; an installer
who chose automatic for that app is updated at the next load only after the
soak (a test clock constant, a compile-time test hook is not allowed, so the
proof uses a real 24h-old listing created by the harness on the tape); the operator yanks release 2 by CLI; the installer's next load
removes it and shows "Removed by the platform: <reason>". A publisher
request writes a note and executes nothing (asserted). Rollback: revert; a
yank already written stays in the chain and is honoured by listing readers.

Ordering: M1 and R1 first, in parallel. Then M2 with M3 together. M4 and M5 any
time after M1. F1 after M3 (it needs the install path to be the only path).

## Failure modes

| # | Failure | Guard |
|---|---|---|
| X1 | a package path enters the stack by a spelling the classifier misses (rename, mode change, a symlink) | the authority is `AdmitPatchedTree` on git's committed tree, not the patch text (`hostbuild/types.go:229`) |
| X2 | a mixed change strands an engineer | refusal names both lists; the engineer casts two operations |
| X3 | migration drops an app's unpublished work | step (b) publishes before step (c) removes; the stack is rebuilt from non-package hunks only after the install record exists |
| X4 | migration re-runs | the kind rename and stack rebuild are idempotent by tree id; no resumer; a crash mid-migration closes the work and the next boot's fold recomputes from the tape |
| X5 | an update (manual or automatic) installs a build that fails | the old module stays until the new one stores (S12); the module build is offline and fails closed |
| X6 | a forced update loops (a yanked head, no eligible head) | no head: the package leaves the launcher; the base copy shows if one exists; no retry |
| X7 | the platform operator token leaks | host transport and host credential, as the key-escrow gate; the act is recorded with the operator name and is not reachable from a guest |
| X8 | a clean rebase silently changes behavior on the owner's computer | named residual until R2's contract run after the build; the rollback is the product restore, not a revert of main |
| X9 | an app update changes data schema | out of scope; the data lives in the app's objects, not the module |

## Standing questions (short answers)

Provenance: every ruling is quoted. Settled-decision conformance: no switches
are added; the old setting is deleted; the Library's Base tab is another
builder's. Deletion citers: section "Deletions" lists the consumers of each
removed symbol; each slice's first step is a grep of its own list. Single
state authority: the install record on the tape is the installed fact; the
listing chain is the catalog fact. Artifact-verified success: each slice's
proof is a product path or a receipt. Fate-sharing: an install record is
written after the module is stored. Restart durability: no resumers; a
crash closes the work. No-SSH operability: the yank is the one act that
needs the host, deliberately. Registry hygiene: the object kind rename
updates `objectgraph/registry.go`.

## Decisions for the owner (each with a default)

- **D1. The migration publishes a computer's app source as a release on that
  computer's track.** Default: yes, on the computer's own track, decided by the
  migration's own record, with no owner act. Alternative:
  drop the unpublished app source. Default stays; it keeps the keep set.
- **D2. Auto-update soak.** A compromised publisher is the threat; automatic
  update with no soak delivers a bad release at every load. Default: 24 hours
  listed before an *automatic* update, applying only to users who chose
  automatic. Still open for the owner to confirm, change or drop. The update
  setting itself defaults to manual.
- **D3. `lib/*App.svelte` apps.** They are app code in the base and can
  still enter the stack as platform edits (`engineer_lineage.go:46-47`). The
  target model, read strictly, says they are not allowed to. Default: allowed
  until converted to packages; conversion is a named follow-up, not part of
  this build.
- **D4. "Adopt" in the Autopaper object graph.** The CLAUDE.md language of
  computers adopting published objects onto their tapes is a different
  mechanism. Default: this design deletes the package-install name only and
  leaves object-graph adoption alone; if the owner means to rename that
  too it is a separate pass.
- **D5. The platform computer takes the clean rebase.** The code has
  an explicit exemption (`internal/platform/clean_rebase.go:109`, "a platform computer
  never applies a clean rebase without the owner"), written for the paper.
  Default: remove it, per ruling 2, with the correctness refusals and the
  marker-backed refresh for always-busy computers. If the owner wants the
  paper to keep an owner act, it is a one-line policy and the ruling is not
  fully honoured.
- **D6. Installed manifests must say `first_party`.** The manifest validator
  refuses any other origin (`internal/apppackage/manifest.go:151`, "installed
  packages are P5"), and the export tool refuses non-first-party on main
  (`cmd/choir/package_export.go:190`). A third-party installed app therefore
  carries a `first_party` origin today, which makes the field meaningless.
  Default: M3 adds `origin: published` for Library releases, accepted by the
  runtime install and refused by `main` export, and base packages stay
  `first_party`.
- **D7. Decision mode for package operations.** Section 1 names
  `standing_consensus` as the closest mode and removes its arm, expiry and
  tier test for package operations. Default: do that, on every computer not
  `off` or `audit_only`. Alternative: keep the arm for platform operations
  and add a new mode for package operations; rejected as a second mode for
  the same decision. The only owner act left in this design is the platform
  break-glass yank or force.

## Build notes: M2+M3 (2026-11-05)

Landed in one commit (MA5.M2+M3):

- **The wall, all doors (M2).** `internal/packagepath` is the one classifier
  (`RootOf`, `Classify`, reason `package_path_refused`). Door 1: Freeze
  refuses a mixed change set (platform and a package, or two packages) with
  both lists named (`capsule.ClassifySourcePatchFiles`, `ErrMixedLayers`).
  Door 2: `hostbuild.refusePath` refuses any package path, so both
  `RefusePatchPaths` (early) and `AdmitPatchedTree` (authority, git's
  committed record) refuse it. Door 3: `sourceproj` reconcile refuses a live
  stack whose patch changes a package path (`stack_package_path`, nothing is
  moved, `package_paths` in `/health`), and `Reapply` never carries one;
  corpusd refuses a bundle whose ordered file effects name a package path, in
  the consensus poster's host-build seat and in the clean rebase gate
  (`selfdevprotocol.RefuseBundlePackagePaths`). A stack the booted base
  already carries is retired as before.
- **S6 deleted.** `internal/sourceproj/packages.go` (entries, states, the
  package graft) is gone; the S4 path graft stays (`pathGraft`).
  `Result.Packages`, `/health effective_release.packages`, the Cast warning's
  package sentences, the rebase notice's package-conflict branch and the
  source-mode provenance annotation are deleted.
- **The draft reads the package operation's frozen bundle.** The stack
  machinery (`packageSourceBefore`, `AppliedRefs`, `Stack`, `BeforeBase`,
  `graftBefore`, the sibling exemption) is deleted: the package before the
  operation is the base's, the patch must lie under one package root, and a
  capsule cast from a projection commit names the stack base as `built_on`
  and as a base parent. The draft and media routes admit a decided operation
  (accepted, materializing, applied). Freeze no longer composes a
  package-only patch into the cumulative stack patch.
- **`packages.publish` deleted** (setting, readers, registry entry).
- **Before and after at the host.** corpusd refuses to list an update
  (parent not the empty tree) without a before and after screen of one view
  (`pkgrelease.RequireBeforeAfter`, `before_after_required`).
- D6 needed nothing: `origin: published` already landed with P0.

Deferred, named (each is M3 scope that did not land here):

1. **The package operation's decision and effect.** A package-only frozen
   operation still walks the platform pipeline and is refused at the host
   build (`package_path_refused`). Still to build: decide it by the
   `standing_consensus` policy with no arm, expiry or tier on every computer
   not `off` or `audit_only` (D7), and make its effect publish then install
   locally with the decision as the publish authority instead of the
   per-release owner act; the operation reaches `applied` with the install
   record.
2. **The candidate overlay** (section 1a): the candidate install record, its
   `<name> (candidate)` launcher name, the supplant on acceptance and the
   boot close as "interrupted by a restart". Also: a capsule's workspace
   holds the base's copy of a package, not the installed track head, so an
   update of an installed head refuses `package_parent_unpublished` at draft
   until the capsule is seeded from the installed version.
3. **The boot migration** (section 4). Until it lands, a computer whose
   applied stack holds package source reports `stack_package_path` (with the
   paths) and refuses casts; its served build is unchanged. Self-development
   is paused until gate G anyway. The legacy source-mode install records
   (`installStateSource`, "Earlier source install") go with the migration.

## Build notes: M3 boot migration (2026-11-06)

Written before the code. This is deferred item 3 and the legacy-record part
of section 4 step 5. Observed on d@e (read-only product API, 2026-11-06):
`effective_release.source_projection.state = stack_package_path`, operation
`selfdev-534017c7e5002466af83df914e6b7d50`, stack base `91fb7c57`, booted
`5c83a08f`. The cumulative patch is package-only: it adds
`frontend/src/packages/slides-next/{App.svelte,choir-package.json}`, and that
manifest is the deleted v1 schema (`choir.app-package/v1`). d@e has no
runtime installs. Its served SPA is the release's own copy (non-layered,
built on `91fb7c57`), so d@e serves a frontend four days behind its base.

### What the migration is

One attempt per (stack operation, booted base) pair, started from the boot
source-projection pass (and again after a base fetch lands), never by a
resumer. It runs only when reconcile says `stack_package_path` and the booted
base's objects are in the guest repository. It reads only the tape and the
bound bundle; nothing it writes is read back as input except its own attempt
record, which only bounds attempts.

1. **Classify** the stack patch (`packagepath.Classify`). A patch with any
   platform path is *mixed*: refused for the pair (`package_migration_mixed`),
   nothing moved. Rebuilding a platform-only stack needs a host build and a
   corpusd receipt kind of its own; it is a named residual, not this slice.
2. **Snapshot.** The pre-migration release stays pinned at
   `releases/<digest>` (nothing in the updater collects releases); the
   stack's deterministic projection commit is kept at
   `refs/choir/kept/<operation>`; the installs in force are listed. The
   attempt record (`<updater root>/package-migration/<pair>.json`, written by
   rename) holds all three and the step reached.
3. **Per package root** in the patch: if the booted base already holds the
   identical subtree, it is *upstreamed* (nothing to install). Otherwise the
   root's own patch is drafted into a release record on the owner's track
   (`pkgrelease.DraftPackage`, the operation's decision as its operation
   fields, deterministic, so a re-run names the same release id) and
   installed at run time through `installPackage` (trigger `migration`). A
   refusal that is the package's own (manifest, build, review gate: any 4xx)
   marks it *kept*: its bytes stay at the kept ref and in
   `package-migration/<pair>/<name>.patch`, and the record names the reason.
   A server-side failure (5xx, git, store) fails the attempt.
4. **Swap.** The served build becomes the booted base: `current` moves to
   the booted image's baseline release (restaged when that release is
   already pinned; otherwise `current` is removed and the existing baseline
   import runs, exactly a fresh computer's first boot). The running binary
   is the base's already (a package-only release is never layered), so no
   restart is needed: the computer surface reads `current/frontend` per
   request.
5. **Reconcile** the projection (now an empty stack) and check the
   **differential**:
   - D1: every package root the stack changed is installed (an install in
     force with the drafted release id and its module present), upstreamed,
     or kept with a reason;
   - D2: the served release is the booted base's baseline (`current`'s
     content digest equals the baseline manifest of the booted release
     root), or `current` is absent and the surface serves that root;
   - D3: the projection is no longer diverged;
   - D4: every install in force before is still in force, unless the
     migration replaced its package.
   A failed differential rolls back automatically (below) and the attempt
   is `failed`.
6. **Record** the outcome on the attempt record, on `/health`
   (`effective_release.package_migration`) and as one ledger record
   (system-committed observation, like the Cast warning), so the owner sees
   it.

Publishing the drafted release (section 4 step 1b) waits for deferred item
1: the publish authority for a package operation is its decision, and the
migration must use that same authority rather than mint a second one. The
release record and its blobs are deterministic and the install carries
them, so a later publish names the same release. Step 1a (find an existing
listing) is an optimization over the same bytes and also needs the catalog;
it waits with 1b.

### Failure modes (each has a test)

| # | Failure | Guard |
|---|---|---|
| PM1 | a crash after some installs, before the swap | the record is non-terminal at the next boot and is closed "interrupted by a restart"; the stack is still the old one; installs are idempotent by release id; a new attempt starts only on a boot that followed a planned restart (or the next deploy, a new pair); two interrupted attempts close the pair |
| PM2 | a crash after `current` is removed, before the import | the surface falls back to the booted release root; the boot's own surface pass imports the baseline; the projection then reads an empty stack, so no new attempt starts |
| PM3 | a re-run after success | the stack is empty, so reconcile never says `stack_package_path`; the pair's record says done |
| PM4 | an app that cannot be installed under today's contract (d@e's v1 manifest) | kept, named, its bytes kept; the computer is not held at `stack_package_path` forever by a package it cannot load |
| PM5 | a transient failure (5xx, git, the updater) | the attempt fails, its own installs are uninstalled (trigger `migration_rollback`), nothing is swapped; the pair is closed, and the next deploy is a new pair |
| PM6 | a mixed stack | refused for the pair, nothing moved; one attempt per deploy |
| PM7 | an operation is open | no attempt (not counted); the next trigger checks again |
| PM8 | the differential fails after the swap | automatic rollback from the snapshot; `failed` |
| PM9 | the owner wants the old state back | the owner's `package-migration-rollback` act runs `rollbackPackageMigration`: restage the pinned release, uninstall the migration's installs, record `rolled_back`; the pair is not migrated again (the next deploy is a new pair) |
| PM10 | two triggers at once | one mutex; the second sees the record |
| PM11 | the booted base objects are missing | no attempt until the fetch lands |
| PM12 | the base already carries the package | upstreamed, nothing installed |

Restart rules: the attempt is an obligation, closed at boot when found open.
The pair's migration is a standing platform intent (the stack must hold no
package path), so a later boot's attempt is new work, like a schedule's next
tick; the planned-boot requirement and the two-interruption bound keep a
migration that crashes the computer from looping.

The legacy source-mode install records are history: a tape is replayed, not
rewritten, so they are not converted to runtime records (a runtime record
would fold as an install in force with no module). `installView` shows them
as `replaced`, the `source` state and the Library's "source" branch are
deleted. `Mode` stays, because the fold needs it to leave those rows out.

### Residuals after the migration build (named, not built)

- **Mixed stacks.** A stack with platform paths is refused
  (`package_migration_mixed`). Serving base plus the platform hunks needs
  the clean-rebase pipeline with a new corpusd receipt kind (the patch is the
  old patch minus the package roots, not the same bytes). No computer is
  known to hold one; the migration's refusal reports any that does.
- **Publish.** Section 4 step 1a/1b: the drafted release is installed, not
  published. It waits for deferred item 1's authority.
- **The rollback act** is built: `POST /api/computers/<id>/self-development/package-migration-rollback`
  with `{"pair": "<effective_release.package_migration.pair>"}`, a
  rollback-class owner act at the proxy, gated exactly like the sibling
  `rollbacks` route: an owner session, or a key that carries `owner_acts`
  and `computer:self_development:rollback`; agent and admin keys are
  refused. The guest checks the owner again. `{"rearm": "<operation id>"}`
  on the same route lifts the owner's hold (PM15). See review r1 below.
- **Deferred item 2's seed fix (the capsule seeds from the installed head).**
  Not built, because the obvious fix breaks platform operations. Two facts:
  (1) an install keeps no source on the computer: `installPackage` stores
  the module and records `patch_sha256`, but the tree patch bytes are
  dropped, so there is nothing to seed from; (2) a capsule cast from a seed
  commit (projection HEAD plus the installed package trees) would make every
  platform operation's cumulative patch (`composeFreezePatch`, stack base to
  seed plus change) carry the installed package paths, and the stack would
  refuse it as mixed. The fix needs both: the install keeps its tree patch in
  the module store, and the seed applies only to a Cast whose spec names the
  package (a package operation is declared at Cast, not discovered at
  Freeze), with Freeze composing a package operation against the seed
  commit and a platform operation against the projection commit.
- **H1 (base computers keep their genesis frontend).** The migration serves
  the booted base by importing the baseline when `current` is absent, which
  is what a fresh computer does. Whether an older base computer's `current`
  genesis copy is refreshed at a deploy is still the open H1 of the update
  architecture design; the migration neither fixes nor depends on it.

### Review r1 (2026-11-06): two seats sent it back; failure modes PM13-PM24

Written before the r1 code. Each row has a test or names why not.

| # | Failure | Guard |
|---|---|---|
| PM13 | another writer moves `current` while the migration runs: a selfdev apply, a resumed platform update, a tape restore, the boot baseline import (Devin 1, scenarios B and C) | one guest lock, `currentReleaseMu`, held by every in-process writer of `current` (the apply calls, the restore's restage, the baseline import) and by the migration from the snapshot to the end of the differential; the migration re-reads `current` under it and stops (deferred) if it no longer names the stack's release |
| PM14 | a resumed platform update swaps `current` before the migration runs, so the projection reads `base` and the package source is gone with no record (Devin 1, scenario A) | under the same lock, a platform transition is refused (`package_stack_unmigrated`) while `current` names a release whose stack changes package paths and the projection does not say it is upstreamed; the refusal is a correctness refusal like release skew. A tape restore is an owner act and is not refused; restoring away from a package stack is named in the residuals |
| PM15 | the owner rolls back, and the next deploy (a new pair, no record) migrates the stack again (S2) | the rollback writes a hold per operation (`package-migration/hold-<op>.json`); no attempt starts for that operation on any base while it stands; `/health` shows `held`. The owner lifts it with `{"rearm": "<operation id>"}` (the next boot migrates) or takes the stack off with the self-development rollback to a prior applied head |
| PM16 | the owner's rollback restores over newer work (Devin 2, S1) | refused 409 `package_migration_drifted` unless `current` is the release the attempt served and no operation is open; it runs under `currentReleaseMu` |
| PM17 | an interrupted attempt's installs are never undone (S3) | the record (not the attempt) lists every install intent (package, release id) before `installPackage` runs; undo and rollback uninstall every in-force install of those release ids with trigger `migration`, whichever attempt made it, including one made by an install that crashed before its attempt saw the answer |
| PM18 | a crash after the swap closes the attempt with no way back (S4) | the target release digest is recorded before the swap; the rollback accepts a `done` attempt, or an `interrupted` one at `swap` or `differential` whose target is `current` |
| PM19 | a record write fails after the swap and the swap is left unrestored (S5) | after the swap every failure path restores the snapshot |
| PM20 | a transient refusal is kept for good, or burns the pair (S6, Devin 4) | kept only for the package's own permanent refusals (the draft and record reasons, content and secret refusals, a failed build, uses beyond grants, a review rejection, a built-in name not the owner's); a release that needs an install review is kept for review without running seats at boot; every other refusal (5xx, a seat unavailable, attempts exhausted, a store or git failure) ends the attempt `deferred`, undoes its installs, swaps nothing, and does not close the pair: a new attempt starts on the next planned boot, three deferrals close it |
| PM21 | wiring gaps or a transient snapshot read burn the pair (Devin 5) | `deferred`, as PM20 |
| PM22 | a kept app leaves no product trace (Devin 6) | `GET .../packages/installs` carries `set_aside` (package, reason, pair) and the Library lists it with its reason and an "Install" button; the owner's `POST .../self-development/package-migration-install {"pair","package"}` (an install-class owner act) drafts the kept bytes again and installs them through the ordinary path, seats included |
| PM23 | a migrated built-in app shadows the base copy and stops getting base updates (S7) | a package that is a built-in app is kept (`builtin_shadow`), not installed; the base copy serves; the owner may install the kept copy deliberately (PM22) |
| PM24 | bookkeeping: the close-once flag set before the scan, a stale running attempt ignored by mayStart, unsynced patch files, the baseline digest derived twice, restages without the store floor (Devin 7, 8, 9, 3) | the flag is set only after a clean scan and a running attempt found under the lock is closed first; patch files are fsynced; the baseline manifest is resolved once per attempt and the differential compares against it; every migration restage passes the store schema floor |

As built (r1), where the code went past the table:
- PM13: the migration takes `currentReleaseMu` before it reads the applied
  stack, so the read of `current`, the snapshot, the swap and the
  differential are one critical section. The swap is
  `updater.StageBaselineRelease` (stages `releases/<digest>` from the
  trusted baseline root, store floor checked) then `RestagePinnedRelease`
  (one rename of `current`); `current` is never removed.
- PM14: an unreadable applied stack also refuses the platform update (it
  cannot be shown to hold no package source).
- PM17: a pair closed by its bounds (two interruptions, three deferrals)
  uninstalls its intents once (`closed_undone`), since its caller saw the
  stack still served.
- PM20: with no verdict signer, a release that needs a review is also kept
  for review at boot (`install_review_deferred`), not deferred.
- PM20: a deferred or interrupted pair retries once per boot that followed a
  planned restart (a process-local tried set), never twice in one boot.
- PM22: the proxy maps `package-migration-install` to its own action,
  `migration-install`: POST only, the install scope, the owner-act gate,
  and the body passed through (the guest drafts from its own kept bytes;
  nothing is read from the catalog).

### Review r2 (2026-11-06): one blocking item; failure modes PM25-PM30

Written before the r2 code.

| # | Failure | Guard |
|---|---|---|
| PM25 | a transient record write (EMFILE, ENOSPC) inside `step()`, or a rename or write failure in the stage or swap, ends the attempt `failed`; `failed` closes the pair, and with PM14 refusing platform updates over the unmigrated stack no new pair forms: a transient fault wedges the computer (Devin r2, 1) | every I/O-class cause is `deferred` (record writes, stage and swap I/O), after a restore too; `failed` is kept for causes a retry cannot change: the store floor's refusal (`updater.ErrStoreFloorRefused`, new), a corrupt pinned release, a differential mismatch after a swap. A failed pair's kept packages stay on the set-aside list. Test: a record write fails at the packages step, then the next planned boot finishes |
| PM26 | `ensureServingBaseline` read `current` before taking the lock, so a checkpoint bind could capture the pre-swap manifest (Devin r2, 4) | the read and the import are one critical section under `currentReleaseMu`. The updater daemon's `HoldApplyKey` reads `current` in another process, where the guest lock cannot reach; it is safe because PM7 refuses an attempt while an operation is open, so a swap never lands under a journal the fence is judging (at worst a different dead journal reads `unknown` instead of `nothing installed`: tape noise, not a wrong transition) |
| PM27 | a tape restore restages `current`, and a base-fetch-triggered migration in the window before the restart swaps it to the baseline, so the restore does not take (Devin r2, 5) | a restore that restaged `current` sets a process flag under the lock; while it stands (until the restart, which is a new process) no attempt starts |
| PM28 | the rollback of an attempt cut at the swap whose swap never landed (or was restored) is refused as drifted, so the owner cannot hold the operation (Devin r2, 2) | for an attempt that did not end done (interrupted, deferred or failed), `current` equal to the snapshot release is admitted: the rollback writes the hold, undoes the intents and ends `rolled_back` |
| PM29 | the platform-update refusal admitted an apply in flight (`Pending`): fail open where an unreadable stack refuses (Devin r2, 6) | refused until it settles |
| PM30 | the owner's set-aside install held `packageMigrationMu` across the install review (minutes), stalling the boot machinery (Devin r2, 3) | the record is read under the lock; the draft and the install run without it; the record is re-read under the lock to mark the package installed |

### Review r3 (2026-11-06): one blocking item; PM31

| # | failure | answer |
|---|---|---|
| PM31 | the differential could not tell "read and mismatched" from "could not read" (installs unreadable, module bytes unreadable, `current` unreadable, reconcile error), so a transient read ended `failed`, closed the pair and left the stack refusing every update (Devin r2, 1) | each check records a mismatch or an unknown; an attempt with unknowns and no mismatch defers (bounded, planned boots only) and restores the snapshot; `failed` stays for a comparison that read and failed |

### Owner ruling 2026-11-06: a parked operation does not hold the migration; PM32, PM33

Owner, 2026-11-06: "that shouldnt stop anything." On d@e the only open operation was `selfdev-ccf792fc`, `awaiting_approval` since 2026-11-03 (mode propose_only, never approved), so PM7 refused every attempt and `/health` `package_migration` stayed null.

| # | failure | answer |
|---|---|---|
| PM32 | PM7 refused an attempt for any open operation, so a candidate nobody decides on held the migration forever | `selfdev.StackChangingStates` = `accepted`, `materializing`, `rollback_pending`, `degraded`; `Store.StackChangingOperation` replaces `OpenOperation` in `migratePackageStack` and in the owner rollback's drift guard. **Block**: `accepted` (decided; the materializer's next step swaps `current`), `materializing` (the updater is installing), `rollback_pending` (a rollback is installing), `degraded` (applied with an unsettled tail; the stack just changed). **Do not block**: `awaiting_approval`, and the pre-decision states `requested`, `executing`, `frozen`, `verified` (a desk run is building in a capsule; nothing touches the stack or `current` until a decision). Tests: `TestPackageMigrationParkedOperationDoesNotBlock`, `TestPackageMigrationStackChangingOperationBlocksAndHealthShowsWhy`. PM26's argument (a swap never lands under a journal the fence is judging) still holds: a journal exists only from `materializing` on. |
| PM33 | five early returns (PM11 base objects missing, top nil or pending, restore restart pending, stack moved, PM7) returned nil or a logged error, so an operator could not see why no attempt ran | an in-memory `waiting` (reason, since, at) on the `package_migration` view, set at each of those returns and cleared when the pass gets past them or finds nothing to migrate. With no attempt record yet the view is `{waiting}` alone. It is never written to the attempt record, so it consumes no budget. Tests: `TestPackageMigrationBaseObjectsMissingShowsAWaitingReason` and the blocking test above |

**A parked operation approved after the migration moved the stack.** It is not applied onto the wrong base:
- The materializer claims the bundle's frozen base commit on the release (`plainReleaseBaseCommit`, `internal/agentcore/self_development_materializer.go:845-862`), and the updater refuses a release whose `base_commit` differs from the booted base's `build_commit` with `ReasonBaseMovedSinceFreeze` (`internal/updater/updater.go:487-500`). A host-built apply is refused `base_commit_mismatch` (`internal/hostbuild/service.go:468`). The migration never changes the booted base, so a candidate frozen before a platform update stays refused as it was.
- A claimed rebase round's candidate must contain the applied change on the booted base (`rebaseContentGate`, `self_development_materializer.go:603`, `internal/agentcore/rebase_content.go:149-188`); a mismatch settles it `failed` and the computer keeps its release. The U4 rebase notice and the carried patch (`internal/agentcore/clean_rebase.go:155`, `carried_patch.go:242`) read the stack through the projection, which after the migration is the booted base, so the next rebase starts from the migrated state.
- Residual, named: a parked candidate frozen on the booted base itself passes the base check; its runtime files were frozen over the pre-migration stack, so applying it can carry the package source back into the stack. The projection then reports `stack_package_path` for the new top operation, a new pair forms, and the migration runs again. This is self-healing, not a wrong-base apply, and is not built against.

Devin r2's non-blocking items 2-7 (a restore flag with no restart pending, the review under `packageInstallMu`, holding a pre-snapshot attempt, a late record write read as interrupted, the closed-pair undo not restaging `current`, and the smaller nits) are residuals of this station, bounded and not wedging.

Named, not built in r1:
- **S8.** The staged baseline release carries the route's current code and
  artifact refs, which can still name the old operation's. Stamping the base
  identity changes what checkpoint binding reads and wants its own design;
  the served bytes are the booted base's either way.
- A tape restore to a checkpoint before the migration brings the package
  stack back on a pair that is already `done`; no attempt starts for it
  (the pair is closed). The owner's way out is the rearm (PM15) only after
  a rollback; for a restore, a code fix (count a restore as a new pair) is
  the residual.
- A mixed stack (refused, PM6) or a held one (PM15) keeps the platform
  update refused (PM14) until the owner rolls the change back with the
  self-development rollback, or rearms the hold. That is deliberate: either
  way out is an owner act, and the source is never dropped silently.
- A rearm takes effect at the next boot; it does not start an attempt.
- The Library's playwright spec does not yet mock `set_aside`; the Go
  tests prove the route and the listing.
