# Modularity check build (2026-11-04)

Status: build plan and record, branch `modularity` (on `s12-runtime-install`).
Yellow and orange: CI, tests and frontend tooling; no guest runtime, shell
runtime or Go change. Owner, 2026-11-04: "we should have some check that our
update isn't breaking any apps. Basically, we should have some verifiable way
to assert modularity."

## What modularity means here

A package is modular when three things hold, and each one has a check that
leaves a receipt.

1. **The boundary holds.** A package's code reaches only its own files, the
   shell SDK (`frontend/src/lib/app-sdk.ts`), `svelte` and the dependencies
   that `frontend/package.json` declares. The package boundary plugin
   (`frontend/scripts/package-boundary-plugin.mjs`) already enforces this on
   the resolved module graph, in the shell build and in the computer's
   module/v1 build (S12). The check builds every package both ways, so a
   boundary break fails the gate.
2. **The SDK is a versioned contract.** The SDK surface (every exported name
   with its type) is recorded as a lock per `SDK_VERSION`
   (`frontend/src/lib/sdk-surface/<version>.json`). A change to the surface
   needs a version bump of the right kind:
   - **major**: a name leaves, a value's type changes so that the new one
     does not fit where the old one was used, or a type changes in a way
     that is not equivalent;
   - **minor**: a name is added, a value's type widens (the new one still
     fits every old use), or a type is reworded but equivalent;
   - **patch**: the surface is unchanged.

   Each package's own SDK use must fit inside what its manifest asks for: the
   SDK names it imports (the `sdk_imports` that the module/v1 build records)
   must all exist in the lowest SDK version its `requires_sdk` admits.
   With S12's load-time check (`requires_sdk` in range and every
   `sdk_imports` name present), this proves: a platform update cannot remove
   or retype a name an installed app uses unless the major version moves,
   and a moved major refuses the app cleanly ("This app was not opened")
   instead of letting it break.
3. **Every package behaves under the new shell.** Each package carries a
   smoke contract (`choir-contract.json` beside its manifest). The gate opens
   each package in the freshly built shell the way a person does (the app
   switcher, the launcher entry, the window), waits for the contract's ready
   selector inside the window, checks its assertions, and fails on any
   uncaught exception or console error. It does this twice per package:
   - **in-tree**: the package as the shell build bundles it
     (`svelte-inline/v1`);
   - **installed**: the package built to one module/v1 module by the
     computer's own build script and served as an installed package, so the
     shell's real loader verifies its digest, `requires_sdk` and SDK names
     and mounts it in `PackageAppFrame`. This is the path every Library
     install takes on a computer.

Each run writes a receipt
(`choir.modularity-receipt/v1`): the commit, the SDK version and surface
digest and the lock verdict, and one row per package with its tree digest,
contract digest, SDK names used, the lowest SDK version those names need, the
module digest, and pass or fail per mode with the errors seen.

## Failure modes (written before the code)

### The surface lock

| # | Failure | Guard |
|---|---|---|
| L1 | a name leaves the SDK at a minor or patch bump | compare against the latest lock: removal needs major |
| L2 | a value's type changes incompatibly (a new required parameter, a narrower parameter, a wider return) without a major bump | assignability of the new type to the old, by the TypeScript checker, strict |
| L3 | a name is added with no bump, so a package built against it declares `^1.0` and an older shell refuses it only by name | added names need minor |
| L4 | the surface changes and no new lock is written, or the lock for the current version differs from the source | the current version's lock must exist and equal the extracted surface |
| L5 | `SDK_VERSION` goes backwards, or skips the lock of an older version | versions compare as semver; the newest lock may not be newer than `SDK_VERSION` |
| L6 | the extractor reads nothing (a moved file, a broken program) and an empty surface "matches" | floor: at least 30 names, at least 1 referenced type, and `SDK_VERSION` itself must be among the names |
| L7 | the lock carries machine paths or depends on the TypeScript print of `SDK_VERSION` itself | paths rewritten relative to `src/lib`; `SDK_VERSION`'s type recorded as `string`; a test refuses `/Users/` or `/home/` in a lock |
| L8 | a type referenced by the surface but not exported from it (for example `ChoirAppDefinition`, which `getAppDefinition` returns) changes and nobody sees | referenced local types are expanded into the lock and compared like exported ones |
| L9 | a package imports a name that is not in the lock | `sdk_imports` must be a subset of the current lock |
| L10 | a package's `requires_sdk` admits a shell that lacks a name it imports | the lowest version `requires_sdk` admits must hold every `sdk_imports` name |
| L12 | a lock is deleted and written again with a smaller surface; with one lock there is no previous step to catch it (review B1) | `--base REF`: every lock that exists at merge-base(REF, HEAD) must be unchanged in the working tree; only new locks may be added; a base that cannot be read fails. CI passes the pull request's base, or on main the live staging commit |
| L13 | a fabricated intermediate lock already holds a break, so the last step reads as no change, or a new lock is slotted below a released one (review 2, M1) | every adjacent pair of locks is step-checked like the last step; a lock added since the merge base must sort above every lock the base holds |
| L14 | a change loosens the checker or repoints its base in the same push (review 2, M2) | CI runs the base's copy of `sdk-surface.mjs` as well as the head's (as the guest-epoch check does); the workflow contract test pins the base step's case arms, its inputs and `LOCK_BASE` |
| L11 | the guard itself is wrong (says minor for a removal) | negative tests on synthetic surfaces: removal, incompatible change, widening, addition, rewording |

### The contracts

| # | Failure | Guard |
|---|---|---|
| C1 | a new package lands with no contract | every directory under `src/packages` must have a valid `choir-contract.json` |
| C2 | a malformed contract passes as empty | strict schema; unknown fields fail |
| C3 | the app does not open (no launcher entry, no window) | each step waits with a bound and names what was missing |
| C4 | the app throws or logs a console error while opening | `pageerror` and `console.error` after the open click fail the row; the browser's own "Failed to load resource" lines for the stubbed backend are recorded apart, as network, not failures |
| C5 | the ready selector never appears | bounded wait, failure names the selector and the window's text |
| C6 | a contract passes vacuously (its selector matches shell chrome) | every selector is resolved inside the app window's content (`[data-window-content]`) |
| C7 | the run checks a stale shell | the gate builds the shell from the working tree into a fresh directory at the start of the run |
| C8 | shell boot errors are blamed on a package, or hidden | errors before the open click are the shell's row; a shell `pageerror` fails the run |
| C9 | the installed path differs from the in-tree path (an app works bundled but not as a module) | each package also runs as an installed module through the real loader; the frame must reach `data-package-state="mounted"` |
| C10 | the runner itself cannot fail (a check that never fails proves nothing) | negative tests: a missing ready selector, a console error, an uncaught exception, a module that imports a name the shell lacks |
| C11 | the receipt is missing or partial when a test fails | the receipt is written in `afterAll` whatever the result; a row that did not run says so |
| C12 | the receipt cannot be tied to what was checked | it carries the commit, the surface digest, each package's tree and contract digests and its module digest |
| C13 | the backend stub drifts from the shell's boot needs and every app "fails" | the stub answers the boot routes (`/auth/session`, `/api/shell/bootstrap`, `/api/desktop/state`, `/api/preferences/theme`, `/api/packages/installed`) and 404 for the rest; a request it does not answer before the open click fails the row as stub drift |
| C15 | an app whose backend calls all 404 passes on its empty state (review N1) | a request nothing answered after the open click fails the row, unless the contract stubs it (`api`) or lists it in `allow_unanswered` with a reason |
| C16 | the gate is switched off by a change that does not select it (review N2), or skipped because a dependency is missing (N4) | a change to `ci.yml` or the classifier selects the frontend lane; the workflow contract test asserts the job, its trigger, its base, no `continue-on-error`, and that `check` needs and requires it; a missing build dependency fails the spec |
| C17 | package code reaches the SDK through the shim's fallback global, using names its `sdk_imports` never lists | the boundary plugin refuses the identifier and any string holding `__choirAppSdk` in package code (both copies) |
| C14 | the gate is slow and gets skipped | one shell build (about 5 s), one module build per package (1 to 3 s), about 3 s per mode per package; one CI job beside the frontend build |

## What it does not prove (named residuals)

- **Behavior past the smoke contract.** A change that keeps every name and
  type and still behaves wrongly (a function that returns the right shape
  with the wrong content) passes unless a contract asserts it. Contracts are
  smoke, not acceptance.
- **The mount ctx and props shape.** `mount(target, ctx)` with `ctx`
  `{ sdk, package, release, windowId, props, launchContext, dispatch }` and
  the forwarded event list are not in the lock. The installed mode exercises
  them for every first-party package, and `runtime-install.spec.js` for a
  fixture; a change that only third-party packages would notice is not seen.
- **Lock history before main.** A lock is immutable from the moment it is
  at the merge base. A lock written and rewritten on one branch before it
  reaches main is not seen; nothing has adopted it yet.
- **The quiet window.** After the ready selector and the assertions, the
  runner waits for network idle (at most 3 s) and 750 ms more. An error a
  package raises later than that (a slow timer, a retry) is not seen; a
  loaded runner makes the window shorter in effect.
- **The SDK global by a computed name.** The boundary refuses
  `__choirAppSdk` written as an identifier, in a string or in template-literal
  text, and `allow_unanswered` refuses blanket entries (fewer than two path
  segments, or a `*` that is not a whole last segment); a name built at
  run time (`globalThis['__choir' + 'AppSdk']`) gets past it. The boundary is
  architectural, not a security one (the package runs same-origin), and the
  fallback itself belongs to S12.
- **The installed-mode build is a replica (review N3).** `buildPackageModule`
  lays out the build directory the way `internal/pkgruntime/pkgruntime.go`
  does and runs the same two embedded scripts. A change to the scripts reruns
  the gate; a change only to the Go layout or invocation does not, and the
  gate would not see the drift. The Go tests cover the Go side, and a change
  to the frontend's boundary plugin now selects the Go lane too, so the
  byte-copy test runs (the spec also asserts the two copies are equal).
- **The base's checker on the first landing.** The base's copy of
  `sdk-surface.mjs` runs only once the base has one; the push that lands it
  is judged by its own copy alone.
- **A real backend.** Contracts run against a stub that answers the boot
  routes; an app's data paths are exercised only as far as a contract stubs
  them (its `api` field).
- **Installed releases from other trees.** CI checks the first-party
  packages in this tree. A release already installed on a computer is
  checked there (the hooks below), not in CI.

## Where else the runner runs

- **Clean rebase** (branch `clean-rebase`): the gate there is "the patch
  applies and the host build passes"; its named residual is "a merge that
  applies and builds but behaves wrongly is not caught". The seam: after the
  host build, run the contract runner on the built shell,
  `node frontend/scripts/package-contracts.mjs --dist <host build dist>
  --receipt <path>`, and refuse the clean rebase (fall through to the notice)
  on a failed receipt. Not wired here: the clean-rebase branch is in review
  and not on this base, and the host builder has no Chromium today.
- **Probation** (Library design S7, section 8.1 "package smoke": "the window
  does not open, or shows a console error"): the same runner, given the
  computer's installed releases materialized as package trees
  (`--packages-dir`) and the computer's served shell (`--dist`). The installed
  mode is exactly what a computer does with an installed package. Not wired:
  nothing of S7 is built, and an install record does not yet carry its
  contract (it is in the release tree, which `pkgrelease.Materialize`
  writes).

## Findings

- **F1 (2026-11-04, the first run of the installed mode): a page load with a
  live session never reads the installed set.** `Desktop.svelte` calls
  `refreshInstalledPackages()` only in the reactive block that fires when
  `authenticated` changes after mount. A page load whose session is already
  live mounts with `authenticated` true, so `onMount` takes the
  `startAuthenticatedDesktop()` branch and the installed set is never read:
  `GET /api/packages/installed` is not requested, the launcher shows the
  in-tree packages only, and an installed package appears only after the
  Library's change signal or a sign-in in that tab. Evidence: the runner's
  request log for the installed mode shows `/api/shell/bootstrap`,
  `/api/preferences/theme` and `/api/desktop/state` and no
  `/api/packages/installed`; every package failed "the window does not host
  the installed module". `runtime-install.spec.js` tests the loader in
  isolation and could not see it. Fix: read the installed set in the
  signed-in branch of `onMount` too. Fixed upstream on `s12-runtime-install`
  in `c3ca51df5`; this branch's own fix was dropped at the rebase.

## Built

- `frontend/scripts/sdk-surface.mjs`: extracts the surface with the
  TypeScript checker (56 names, 17 referenced local types at 1.0.0), checks
  it against `frontend/src/lib/sdk-surface/<version>.json`, and writes a lock
  with `--write` only when the bump is enough. A lock is never rewritten.
  Values that differ are judged by assignability (new to old: a widening,
  minor); types must be equivalent both ways, or the change is major; texts
  that differ only in how the printer wrote them (union order) need nothing.
- `frontend/src/lib/sdk-surface/1.0.0.json`: the lock for SDK 1.0.0, digest
  `sha256:20757fc2...a867`.
- `frontend/scripts/package-contracts.mjs`: the contract schema, the stub
  backend, the runner (in-tree and installed modes), the receipt, and a CLI
  (`--dist`, `--packages-dir`, `--only`, `--modes`, `--receipt`) for the
  seams.
- `choir-contract.json` for image, reader and library-next.
- `frontend/tests/package-modularity.spec.js`: seven tests (the dependencies
  are installed; the lock and its history; the lock rules on synthetic
  surfaces and temp locks; lock history on a temp repository; the contract
  schema; every package in both modes with L9 and L10; the runner and the
  shell on twelve fixture packages: a good one, a vacuous selector, a missing
  selector, a throw on mount, a console error, a late uncaught throw, an SDK
  name the shell lacks, `requires_sdk ^9.0`, an unanswered request, the same
  request allowed and stubbed, and the SDK global).
- CI: the `Package Modularity` job (Chromium, the lock check, the spec, the
  receipt uploaded as `modularity-receipt-<sha>`); `check` requires it when
  the frontend lane is selected, and the lane now also covers
  `frontend/scripts/*`, the spec, `frontend/playwright.config.js`,
  `internal/pkgruntime/*.mjs`, `ci.yml` and the classifier. The job checks
  out full history and passes the lock-history base to both the lock check
  and the spec.

Measured locally: the whole spec in about 75 s (shell build 5 s, three
packages in two modes about 16 s, the fixture block most of the rest, two of
its cases waiting out the 10 s ready bound on purpose).

## More residuals found while building

- **An added optional parameter reads as no change.** TypeScript holds
  `(kind: any, extra?: number) => string` and `(kind: any) => string`
  assignable both ways, so the lock accepts it without a bump. It cannot
  break an installed package (an older shell ignores the argument), but a
  package that relies on the new parameter gets no version signal.
- **Union order is printer order.** TypeScript prints union members in
  creation order, so an unrelated file can reorder one. The equivalence check
  makes that need no bump, but the extracted digest can then differ from the
  lock's digest while the check passes. The receipt carries both.
- **First-party release trees moved.** Each first-party package directory
  now holds `choir-contract.json`, so its tree differs from a release
  published before this change (upstreamed detection, `Choir-Tree`
  comparisons). Any edit to a package does the same; it is named because
  this one touches all three at once.
- **Pre-existing, not from this branch:** `app-packages.spec.js` case
  `extension-clash-static` expects `pdf` to be a static route, which it no
  longer is since Reader became a package; it fails on this base without
  these changes.
