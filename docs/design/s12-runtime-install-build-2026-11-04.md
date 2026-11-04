# S12 build: runtime install (2026-11-04)

Status: build plan and record of what landed on branch `s12-runtime-install`,
red (a new code-loading path, code from published releases, an owner-act
route). Builds "### S12" of
[the Library design](library-package-propagation-2026-10-26.md) and P5 of
[the app packages replan](app-packages-replan-2026-10-24.md). It replaces the
source-patch install of
[package-install-2026-11-04](package-install-2026-11-04.md). Owner,
2026-11-04: "I thought apps were a layer already. Didn't we solve for this
with app packages?" and "I want all this".

## What it does

The owner presses Install in the Library (or `POST
/api/computers/<id>/self-development/adoptions`, unchanged). The proxy reads
and verifies the listed release and its `tree_patch` from the catalog as that
owner, as before. The computer then:

1. verifies the record against the id and the bytes against the record;
2. installs the release whoever published it (D6);
3. builds the package into one ES module (ABI `choir.app-frontend/module/v1`)
   with its own locked dependencies, offline (`internal/pkgruntime`);
4. writes the module to its module store by SHA-256;
5. records the install on its tape (`choir.release_adoption`, `mode:
   runtime`, with the module digest, `requires_sdk` and the manifest).

The shell reads `GET /api/packages/installed` at sign-in and whenever the
Library signals a change, adds each package to the launcher, and opens it in
`PackageAppFrame`, which fetches `GET /api/packages/modules/<sha>.js`,
refuses bytes whose SHA-256 is not the listed digest, checks `requires_sdk`
and every SDK name the module imports, and mounts the verified bytes.

No source change, no self-development apply, no SPA build and no restart.
Uninstall writes one more record; the app leaves the launcher at once.
Because installed packages are not in the source stack, a base move leaves
them untouched: L17 has no cause for Library installs.

## Decisions

### D1. Where the build happens: on the installing computer

Options were a host package-only build (the Library design's choice) and a
prebuilt module in the release. A prebuilt module breaks the goal file's
`must_preserve` (source only; no binary crosses between computers). A host
build needs the host builder (`internal/hostbuild`, Nix, quotas) plumbed for a
new job kind: days, not hours.

The installing computer already has what a build needs: `node` on the
runtime's PATH and its locked frontend dependencies (`$CHOIR_FRONTEND_DEPS`,
the same tree a capsule's frontend build links). A module build of one
package takes 1 to 3 seconds (measured: image 16 KB in 0.8 s, library-next
83 KB in 1.0 s, reader 3.6 MB in 2.2 s, pdf.js inlined). So the computer
builds from the verified source, and only source crosses between computers.

What runs: `pkgrelease.Materialize` writes the verified tree (only regular
files under the package root; the content checks of `VerifyContent` first)
into a private directory, beside the SDK shim and the two embedded scripts
(`build-module.mjs`, and `package-boundary-plugin.mjs`, a byte copy of the
frontend's, held equal by a Go test). The build compiles the package; it runs
no package code and never the publisher's declared commands. The package
boundary plugin refuses imports that leave the package, and the shell's own
source is not in the build directory at all.

What the build is not: an OS sandbox. It runs as the autoputer user with that
user's file access. "No network" holds by construction (locked dependencies
symlinked in, `configFile`, `envFile` and `publicDir` off, no npm, no install
scripts, URL and `data:` imports refused, a built module with no imports),
not by a network namespace. No package code runs, but a compromised locked
dependency or a Vite or Rollup bug could act as the autoputer user during a
build. That is the same supply-chain trust the shell's own SPA build already
places in those dependencies; the host build (deferred) is where an enforced
sandbox belongs.

The host build stays the later optimization (one content-addressed module per
release, shared by every adopter). The module digest is recorded per install
now, so a host build can be compared against computer builds when it lands.

### D2. The ABI: `module/v1`

One self-contained ES module (Svelte and the package's declared dependencies
bundled; its CSS inlined and added on mount, removed on destroy) that exports:

- `mount(target, ctx)` returning `{ update(props), destroy() }`;
- `abi`, `requires_sdk`, and `sdk_imports` (the SDK names the package imports,
  collected at build).

`ctx` is `{ sdk, package, release, windowId, props, launchContext, dispatch }`.
The package's `import { x } from '../../lib/app-sdk'` resolves at build time to
a shim whose named imports are read from `ctx.sdk` at call time (Rollup
synthetic named exports; the shell also sets `globalThis.__choirAppSdk` so a
package that reads the SDK at module evaluation still finds it). The shim
implements `getLaunchFile` and `launchFileStore` against the module's own
Svelte context, which the wrapper fills from `ctx.launchContext`. Component
events (`launchapp`, `opentexture` and the rest AppHost forwards) go out
through `ctx.dispatch`.

Existing first-party packages (`svelte-inline/v1`) build to `module/v1`
unchanged: the spec builds image, reader and library-next on every run.

### D3. `requires_sdk`

- `frontend/src/lib/sdk-version.ts` exports `SDK_VERSION = '1.0.0'` (re-exported
  by `app-sdk.ts`); MINOR rises when the SDK gains a name, MAJOR when one
  changes or leaves.
- The manifest takes an optional `app.requires_sdk`, `^MAJOR.MINOR` (Go
  `apppackage` and `check-packages.mjs` both validate it). Absent means
  `^1.0`, the SDK the svelte-inline packages were written against, so every
  existing release installs.
- The shell loads a module only when `SDK_VERSION` is in range and every name
  in `sdk_imports` is in the SDK. Out of range, an in-tree package of the
  same name wins (the base package); with none, the window says which SDK the
  package needs.

### D4. Store layout and discovery

- Module store: `$CHOIR_PACKAGE_STORE`, default
  `$CHOIR_UPDATER_ROOT/packages` (`/mnt/persistent/choir-updater/packages`):
  persistent, outside the source tree and the served release, with no Nix
  change. Layout `modules/<sha256>.js`, written once (0444, temp and rename)
  and re-verified on every read.
- The install record is the installed fact, on the tape, so a restore to a
  pre-install head folds to a set without the package. The installed set is
  the fold of runtime records, newest first: per package, the latest runtime
  record, when it is an install. The record is written only after the module
  is stored; a crash between leaves an unreferenced module, never an install
  without one.
- A record whose module left the store (a restore onto a fresh store) lists
  `available: false`; Install again rebuilds it (the proxy supplies the tree).
- Routes (the generic `/api/*` forward, owner from the proxy):
  `GET /api/packages/installed` and `GET /api/packages/modules/<sha>.js`. A
  module is served only while an install in force names it.
- Shell: `packages.ts` holds the installed set and the verified loader;
  `registry.ts` adds or removes the installed apps in place and bumps
  `registryRevision`, which `AppHost` and the launcher follow. An installed
  package whose name an in-tree package holds replaces it while installed
  (the owner reviewing another track); a static app is never replaced.

### D5. One path: the source-patch install is deleted

Runtime install covers every first-party package release the Library lists
(kinds are already refused at publish, `package_kinds_need_platform`), so the
adoptions route now does only this. Deleted: the staging under
`<updater root>/adoptions/`, the owner prompt to the desks
(`SetPromptSubmitter`, `textureowner.SubmitOwnerPrompt`), the capsule's
`/selfdev/adoption` mount (`capsule.SpawnSpec.AdoptionDir` and its
`computecapsule` refusal), `ReleaseAdoptionForTrajectory`, and the
three-way against the source tree. Source-patch install records written before
this stay listed with `state: source`; what they did is in the source tree.
No switch: rollout is by deploy ring (d@e first), rollback is a revert
(owner, 2026-11-03).

### D6. Security: same origin, any listed release (owner ruling, 2026-11-04)

A module runs in the shell's origin with the installer's session: it can call
every `/api` route the installer can. `PackageAppFrame` is not a sandbox; it is a
mount point with verified bytes. The earlier decision (replan P5, Library
design 7, L14) kept another owner's code out until the S13 sandboxed frame,
behind `403 cross_owner_before_frame`. The owner overruled it on 2026-11-04:
"No, I don't want it to work only for me!!! Installing published apps is the
essential feature. And I don't want more switches!!!" and "The whole point is
to enable the library as an app store ... realistically, this is the same risk
as GitHub." So:

- **Any signed-in user can install any listed release.** Releases are already
  public (`Listing.VisibleTo`). The cross-owner refusal is deleted from the
  proxy and the computer. This is a widening of the guest surface
  (`security_epoch` 39), by ruling, with no switch, no setting and no
  confirmation flag, and the forwarded body gains no field (an unrefreshed
  guest decodes with `DisallowUnknownFields`);
- everything else holds: the release must be listed (a withdrawn, suspended or
  yanked release is refused for install), the record must hash to its id, the
  bytes to the record, the module must build offline inside the package
  boundary, `requires_sdk` must be met;
- an uninstall never depends on the catalog: the proxy forwards only
  `{release_id, action}`, with no listing gate and no blob fetch, and the
  computer derives the package from its own in-force install record. A
  withdrawn, yanked or garbage-collected release can always be uninstalled;
- the module that runs is exactly the module verified (digest at fetch,
  import from a `blob:` URL of those bytes). This bounds the module graph,
  not what the module does at run time: same-origin code can still inject a
  `<script>` element or fetch anything the installer can. That is the accepted
  app-store risk, the same as running code from GitHub;
- the Library's install dialog names who published the release (owner and
  computer) as ordinary store information. There is no warning gate;
- the build is source only, on the installer's own computer.

Planned mitigations, none built, are in
[library-app-store-security-intent-2026-11-04.md](library-app-store-security-intent-2026-11-04.md):
publisher attestation, a security review on the installer's computer before
install, precommitment records per release, and yank and ban remediation. The
S13 sandboxed frame (opaque-origin iframe and an `ctx.sdk` bridge over
`postMessage`, more than a day) is another, and would narrow what an installed
module can reach. Until they land the wall is gone: the installer's trust in the
publisher is the control.

## Failure modes (written before the code; tests named)

| # | Failure | Guard | Test |
|---|---|---|---|
| M1-M5 | Materialize writes other bytes, paths outside the root, a secret, or into a used directory | `pkgrelease.Materialize` | `TestMaterializeWritesTheVerifiedTreeOnly` |
| R1 | the embedded boundary plugin drifts | byte compare | `TestEmbeddedBoundaryPluginIsTheFrontends` |
| R2, R3 | a stored module is not its digest; a partial or writable write | verify on read; temp, sync, 0444, rename | `TestStoreVerifiesEveryRead` |
| R4-R7 | other bytes built; an import leaves the package; not module/v1; requires_sdk lost | builder | `TestBuild*` (real node) |
| F1, F3 | record or bytes not the release's | `VerifyID`, `VerifyBlob` | `TestRuntimeInstallInstallsReplacesUninstallsAndReinstalls` |
| S1 | another owner's code in the shell origin | proxy and computer refusal | the same, and the proxy test |
| S2-S7 | source changed; repeat rebuilds; a track does not replace; uninstall of the wrong release; module lost; failed build leaves state | install ladder | the same |
| S8-S10 | half-install without a builder; module route serves unnamed or tampered bytes; served without the owner | route checks | `TestRuntimeInstallNeedsItsBuilderAndOwner` |
| B1, B2 | a first-party package does not build; the SDK is bundled | build every in-tree package | `frontend/tests/runtime-install.spec.js` |
| L1, L2 | changed bytes run; a missing module | the shell's loader | the same, in Chromium |
| M1, M2 | the mounted app cannot reach the SDK or loses its CSS; destroy leaves DOM or style | wrapper | the same |

## Review round 1 (Devin, DeepSeek), and the modularity check

- **Uninstall always works** (B1): see D6. Tests:
  `TestRuntimeUninstallNeedsNoCatalogRecordOrBuilder` and the proxy uninstall
  loop over listed, withdrawn and gone releases.
- **The fold reads the whole history.** `listReleaseAdoptions` pages until a
  page comes back short (`TestListReleaseAdoptionsReturnsTheWholeHistory`), and
  an attempt id that is already taken gets a `-N` suffix
  (`TestRuntimeInstallNeverReusesAnAdoptionID`). The attempt limit is checked
  before the build, not after.
- **The bytes that run are the bytes verified, with no second module.** The
  boundary plugin refuses any scheme or protocol-relative import (`data:`,
  `https:`, `//`) and any import that does not resolve to a file; the build
  refuses `export *` from the SDK (the shell's SDK-name check could not see
  it) and any chunk with static or dynamic imports
  (`TestBuildRefusesURLImportsAndSDKExportStar`).
- **The shell.** Sign-out clears the installed set; refreshes are sequenced
  and use `fetchWithRenewal`; an installed record that is unavailable or
  needs an SDK the shell lacks never shadows its in-tree twin; runtime
  definitions are keyed on `install_id|sha256`, so a refresh does not remount
  unchanged open windows. Runtime records carry no `same_base`. Legacy
  source-patch records show a neutral "Earlier source install".
- **F1 (modularity check, branch `modularity`): a page load with a live
  session never read the installed set.** `Desktop.svelte` refreshed it only
  on a sign-in transition after mount, and a session that is live at mount
  makes no transition. `onMount`'s signed-in branch now refreshes too. It
  does not race sign-out clearing, which lives only in the reactive block's
  signed-out branch, and the refresh sequence token drops a stale answer.

Round 2 (Devin approved c3ca51df5). Fixed with the trailers:

- **A signed-out mount kept the previous owner's apps.** Desktop unmounts on
  the legal, recover, email sign-in and device-approve routes and while auth
  is checking or reconnecting; a sign-out landing then never ran the reactive
  sign-out branch. `enterPublicDesktop()` now clears the installed set, so
  every signed-out entry clears it. No test: the shell's specs have no
  signed-in-then-unmounted Desktop harness, and building one is not cheap.
- **Legacy different-base records read "the same base".** `same_base` is
  `omitempty` now, so a stored `false` comes back absent; the Library shows
  "the same base" only for `true`.
- **A dead condition** in the proxy's cross-owner check (uninstall returns
  before it) is removed.

Residuals, recorded and not fixed here:

- **Uninstall needs the self-development operation store.** The adoptions
  route answers 503 when a computer has no `selfdevOperations` store, so such
  a computer can neither install nor uninstall. Every computer the runtime
  builds has one; the "always" in D6 is true of the catalog, not of this
  latent wiring gap.
- **DOM-injected code is outside the verified bytes.** See D6: the build
  bounds the import graph, not `<script>` injection or `fetch` at run time.
  Under S12's owner's-own-code rule this is bookkeeping; S13's frame needs a
  CSP regardless.
- **Unexercised assumption:** the paging loop relies on the store's
  derived-key fast path honouring `limit` the same way; read from the
  signature, not traced to its SQL.

## Built, and deferred

Built: D1 to D6, the Library's answers (`installed`, `replaced`,
`uninstalled`; refusals `package_build_failed`,
`install_unavailable`), the capsule install fixture regenerated from the
handler, and `choir-shot.mjs` serving it.

Deferred, each a named residual:

- **Kinds at install.** A package that declares kinds is still refused at
  publish; registering them from the install fold is the next slice.
- **`opens` and aliases of installed packages.** An installed package is in
  the launcher only; its file routes and aliases do not join
  `PACKAGE_MEDIA_ROUTES`, and it gets no desktop icon.
- **Host build** (one module per release) and the build receipt bound into
  the release record (`build: { module_digest, builder_receipt_digest }`).
- **The sandboxed frame** (S13), which narrows what an installed module can reach (the cross-owner wall itself is gone, D6).
- **Upstreamed runtime installs.** When a base ships a package whose tree is
  an installed release's tree, the install could retire itself; today the
  installed module keeps winning until the owner uninstalls it.
- **Probation, outcomes, yank** (Library 8) apply to runtime installs
  unchanged in design; none is wired.
- **Staging acceptance**: an install on d@e from the Library, a deploy
  after it, and the app still open with no restart.

## How S12 meets S6 (the package lock)

S6 (branch `s6-package-lock`) makes each package root under the applied
source stack its own entry, so a base move re-materializes the stack with the
computer's package kept (L17 for packages in the stack). S12 takes Library
installs out of the stack, so they never become S6 entries. The two cover
different packages:

- S6: packages that are in this computer's source, meaning its own
  self-development edits to `frontend/src/packages/<name>`, and source-patch
  installs from before S12.
- S12: packages the owner installed from the Library.

They touch no file in common. S6 reads `store.ReleaseAdoption` only for
optional provenance (a stack subtree equal to an install's
`release_tree_hash`); runtime records keep that field, and since they change no
source, they never match a stack entry. When both hold a package of the same
name, the shell shows the runtime install (the owner's explicit choice) and
S6 still keeps the source copy correct, so uninstalling falls back to it.
Next step for both: a computer that publishes its own package edit and
installs that release at run time can retire its S6 entry.
