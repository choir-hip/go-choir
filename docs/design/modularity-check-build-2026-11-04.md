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
| L6 | the extractor reads nothing (a moved file, a broken program) and an empty surface "matches" | floor: at least 30 names, and `SDK_VERSION` itself must be among them |
| L7 | the lock carries machine paths or depends on the TypeScript print of `SDK_VERSION` itself | paths rewritten relative to `src/lib`; `SDK_VERSION`'s type recorded as `string`; a test refuses `/Users/` or `/home/` in a lock |
| L8 | a type referenced by the surface but not exported from it (for example `ChoirAppDefinition`, which `getAppDefinition` returns) changes and nobody sees | referenced local types are expanded into the lock and compared like exported ones |
| L9 | a package imports a name that is not in the lock | `sdk_imports` must be a subset of the current lock |
| L10 | a package's `requires_sdk` admits a shell that lacks a name it imports | the lowest version `requires_sdk` admits must hold every `sdk_imports` name |
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
| C13 | the backend stub drifts from the shell's boot needs and every app "fails" | the stub answers exactly the four boot routes (`/auth/session`, `/api/shell/bootstrap`, `/api/desktop/state`, `/api/packages/installed`) and 404 for the rest; the shell row fails first and names it |
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
- **Lock history.** The check compares against the newest lock; rewriting an
  older lock file in the same change is visible only in the diff.
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
  signed-in branch of `onMount` too (one line, on this branch; it belongs on
  `s12-runtime-install`).

## Built

(filled in as the slices land)
