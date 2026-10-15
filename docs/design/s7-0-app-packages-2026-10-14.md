# S7-0a: The App-Package Seam

Date: 2026-10-14. Station: S7 app packages
([definition](../definitions/choir-appdev-s7-app-packages-2026-10-01.md)),
first slice. Branch: `feat/s7-0-app-packages` (not pushed). Mutation class:
this document is **green**. The code it describes is **orange** (new guest
API routes, new frontend loading path, new computer-owned state written
through the existing projection path). It writes onto the canonical tape only
as a client of the existing object-graph projection, and it changes no code
on a protected surface. Because it does write canonical state, it carries the
full red ceremony (§11).

## 1. What this slice is, and what it is not

The owner's distribution plan (2026-10-14, recorded at the top of
[selfdev-update-architecture-2026-10-15.md](../reports/selfdev-update-architecture-2026-10-15.md)):

- only source is published;
- the host is the sole builder, and its builds are reproducible and
  host-signed;
- users never pass binaries to each other;
- apps travel as packages, while core platform changes go upstream through
  the owner's merge;
- a package (a frontend module plus an optional backend service, with its
  own build recipe) is built once by the host and can be installed on any
  computer, whatever that computer's own divergence.

The S7 definition's owner direction (2026-10-14) makes S7-0 "convert every
existing first-party app into a package". That conversion needs a seam to
convert *into*. This slice, **S7-0a**, builds that seam and proves it with a
fixture package (`packages/hello`). The conversion of Texture, Mail,
Calendar, Files, Autopaper, Web Lens and the other registry entries is
**S7-0b** onward (§10). The fixture is a test object, not a new app, so the
rule "no new app before the existing ones are converted" is kept.

S7-0a delivers:

1. a versioned package manifest format, validated in Go;
2. a host build receipt format and the `static/v1` recipe, the only
   recipe whose output is the source itself;
3. install, enable, disable and purge on one computer, as typed objects on
   the computer's tape, folded by a pure reducer;
4. package app data on the tape, purged according to the manifest's
   restore requirements;
5. the guest API that serves an enabled package's frontend bytes by digest;
6. the desktop loading one enabled package's frontend module dynamically,
   beside the static `APP_REGISTRY`, after checking the bytes' digest.

It does not deliver:

- a backend service (designed in §7, refused at install);
- the host build service (designed in §8; P5 owns the host builder on
  `feat/p5-host-build-layered-apply`);
- package update or data migration (S7-1);
- an install UI (the API is the surface; a Settings pane is S7-0b);
- deployed proof (the branch is not pushed).

## 2. Decisions taken in this slice

**D1. Package presence is a fact on the computer's tape, not a member of the
computer's release closure.** The S7 definition (2026-10-01) says "the
package is a release member" and "dynamic package discovery must be derived
from the accepted, content-addressed release manifest". The owner's ruling of
2026-10-14 says a package is built once by the host and installed on any
computer whatever its divergence. A per-computer release closure is exactly
what diverges. So in this slice:

- **presence** (which packages are installed and enabled, at which build) is
  the fold of package lifecycle objects on the computer's tape;
- **bytes** are content-addressed by build digest and served only when an
  enabled package on the tape binds that digest.

The definition's invariants still hold. Discovery has no second writable
authority: the tape is the authority, and there is no mutable URL or
database row. A pinned-head restore selects package presence and package
data by replay. Bytes cannot be swapped under a fixed digest. Status:
proposal by this slice's author (orchestrator-settled, standing question 1).
It needs owner ratification before S7 claims the definition's acceptance.

**D2. Lifecycle and data are object-graph objects on the existing projection
path, not a new event kind or table.** Two kinds are registered:

- `choir.app_package_event`: immutable, one per lifecycle act;
- `choir.app_package_datum`: one per (package, key), and tombstoned on
  purge.

Both reach the tape as `projection_batch_recorded` events through the store's
object-graph interceptor, the path that `choir.computer_setting` and the
commitment ledger already use. Reasons:

- no new `computerevent` kind and no new VM-local table, so the replay
  eligibility manifest is unchanged (`og_objects` is already
  `event_projection`);
- the replay projector accepts any object kind (`internal/store/project.go`,
  `projectObject`). A binary from before S7 that replays a tape holding
  package objects therefore projects inert rows and never poisons. A new op
  kind would poison every older binary on rollback and would force a
  `storeschema.Version` bump.

**D3. The reducer is pure, and the events form a hash chain.** Each event
carries `seq` (1, 2, 3 ... per computer) and `prev` (the digest of the event
before it). `apppackage.Reduce` refuses a gap, a reordering or a broken
chain. Inventory reads fold all events. No cached "head" object exists
beside them, so the events are the single authority (standing question 5).

**D4. Install takes a digest, never bytes.** The install API accepts only a
`build_digest`. The guest resolves it from a trusted package source (§6):
in S7-0a, the catalog directory shipped inside the guest release; later, the
host content-addressed cache. No request body carries package bytes. "Users
never pass binaries" therefore holds by construction, not by policy.

**D5. The frontend ABI does not depend on any framework.** A package frontend
is one ES module whose `mount(target, ctx)` export returns `{ destroy() }`.
It does not share the shell's Svelte runtime. That runtime changes with every
computer's divergence, which is exactly what a host-built-once package
cannot depend on.

**D6. The desktop verifies bytes before executing them.** The shell fetches
the module, computes its SHA-256 in the browser, compares it with the digest
bound on the tape, and imports the verified bytes from a `blob:` URL. The
guest also verifies the digest when it serves the file. A mismatch at
either end refuses activation.

## 3. Manifest: `choir.app-package/v1`

File: `choir-package.json` at the root of the package source tree.

```json
{
  "schema": "choir.app-package/v1",
  "name": "hello",
  "version": "0.1.0",
  "display": { "title": "Hello", "icon": "👋", "description": "S7-0a fixture package" },
  "source": { "digest": "sha256:<64 hex>" },
  "frontend": {
    "entry": "frontend/index.js",
    "abi": "choir.app-frontend/v1",
    "window": { "singleton": true, "width": 520, "height": 360 }
  },
  "backend": null,
  "build": { "recipe": "static/v1", "outputs": ["frontend/index.js"] },
  "data": { "store": "package_kv/v1", "schema_version": 1, "reads_schema_versions": [1] },
  "restore": { "data_scope": "computer_tape", "on_purge": "delete_data" },
  "grants": []
}
```

Field rules (enforced by `apppackage.Manifest.Validate`, strict decode with
unknown fields refused):

| Field | Rule |
| --- | --- |
| `schema` | exactly `choir.app-package/v1`; any other value is refused, so a future v2 cannot be half-read |
| `name` | `^[a-z][a-z0-9-]{1,39}$`; must not be a reserved app id (every static `APP_REGISTRY` id, plus shell names) |
| `version` | `MAJOR.MINOR.PATCH`, numeric |
| `display` | title is required, icon is at most 8 runes, description at most 200 |
| `source.digest` | `sha256:` + 64 lowercase hex; the tree digest of every source file except `choir-package.json` (§3.1) |
| `frontend.entry` | relative slash path, no `..`, no leading `/`, no backslash; must appear in `build.outputs` |
| `frontend.abi` | exactly `choir.app-frontend/v1` |
| `backend` | null, or `{kind: "service", entry, abi: "choir.app-backend/v1"}`; a valid manifest may declare one, but S7-0a install refuses it |
| `build.recipe` | one of the known recipes; S7-0a knows `static/v1` |
| `build.outputs` | non-empty, sorted, unique, safe paths |
| `data.store` | `none` or `package_kv/v1` |
| `data.schema_version` | ≥ 1 when the store is not `none`; 0 when it is |
| `data.reads_schema_versions` | sorted and unique, and must include `schema_version` |
| `restore.data_scope` | `computer_tape` (package data lives on the computer's tape, so a pinned-head restore rebuilds it by replay). Nothing else is accepted in v1 |
| `restore.on_purge` | `delete_data` or `retain_data` |
| `grants` | must be empty in v1, because S4's egress grant model has not landed |

The **manifest digest** is the SHA-256 of the manifest's canonical JSON
(`computerevent.CanonicalJSON`), written `sha256:<hex>`.

### 3.1 Source tree digest

`choir-tree/v1` hashes the sorted list of `(relative path, sha256(bytes))`
for every regular file under the package root:

- `choir-package.json` is excluded, because it holds the digest;
- dot-files are excluded;
- symlinks are refused.

The digest is the SHA-256 of a canonical JSON array of those pairs.
`apppackage.SourceDigest(dir)` computes it. `VerifySource(dir, manifest)`
compares it with the manifest. A test runs that comparison against the
in-repo fixture, so an edit to `packages/hello` without a manifest bump fails
CI.

## 4. Build receipt: `choir.app-package-build/v1`

```json
{
  "schema": "choir.app-package-build/v1",
  "package": "hello",
  "version": "0.1.0",
  "manifest_digest": "sha256:…",
  "source_digest": "sha256:…",
  "recipe": "static/v1",
  "builder": "choir-host-builder/static-v1",
  "files": [
    { "path": "frontend/index.js", "sha256": "sha256:…", "size": 1234, "media_type": "text/javascript" }
  ]
}
```

- The **build digest** is the SHA-256 of the receipt's canonical JSON. Files
  are sorted by path, so the digest is deterministic.
- `static/v1` copies each declared output verbatim from the source tree. Its
  output is a function of the source alone, so any party can rebuild it and
  compare digests.
- A host signature (`receiptsigner`) is a §8 obligation. S7-0a accepts
  unsigned builds **only** from the release-shipped catalog, whose bytes are
  already inside the signed release closure. It never accepts them from a
  request.

## 5. Lifecycle: events, reducer, data

### 5.1 Event `choir.app-package-event/v1`

| Field | Meaning |
| --- | --- |
| `seq`, `prev` | per-computer chain (D3) |
| `op` | `install`, `enable`, `disable` or `purge` |
| `package` | the package name |
| `manifest`, `build`, `manifest_digest`, `build_digest` | on `install` only; the full manifest and receipt are embedded, so replay needs nothing outside the tape to know what was installed |
| `retained_data_schema_version` | on `purge` with `retain_data`: the data schema version left behind |
| `actor` | `owner:<id>` |
| `at` | RFC 3339, informational only; the reducer never reads time |

### 5.2 States and transitions

```text
(absent) --install--> installed --enable--> enabled
                         ^                    |
                         |                 disable
                       enable                 v
                         +---------------- disabled
installed | disabled --purge--> (absent)
```

| From | Op | To | Refusal |
| --- | --- | --- | --- |
| absent | install | installed | backend declared; grants declared; digests do not bind (§5.3); retained data has a schema version the new manifest cannot read |
| present | install | — | `ErrAlreadyInstalled`; update is S7-1 |
| installed, disabled | enable | enabled | — |
| enabled | enable | — | `ErrInvalidTransition` |
| enabled | disable | disabled | — |
| installed, disabled | disable | — | `ErrInvalidTransition` |
| installed, disabled | purge | absent | — |
| enabled | purge | — | `ErrInvalidTransition`: disable first, so the window is closed and any service stopped before data goes |
| absent | enable, disable, purge | — | `ErrNotInstalled` |

`Reduce(events)` returns an `Inventory`:

- `seq` and `head`;
- `packages` (name → installed record);
- `retained` (name → the data schema version a `retain_data` purge left
  behind).

`Apply` also returns the transition's **effect**. Today the only effect is
`DeleteData`, set when a purge runs under `delete_data`. The store carries
the effect out in the same atomic batch as the event.

### 5.3 Install binding checks

All of these run in the pure reducer:

- `manifest.Validate()` passes;
- `manifest_digest == manifest.Digest()`;
- `build.manifest_digest == manifest_digest`;
- `build.source_digest == manifest.source.digest`;
- `build_digest == build.Digest()`;
- the build recipe equals the manifest recipe;
- the build file paths equal `manifest.build.outputs`;
- `event.package == manifest.name`.

### 5.4 Package data (`package_kv/v1`)

- A datum is `{package, key, value (JSON), schema_version}`. The key matches
  `^[a-z0-9][a-z0-9._-]{0,63}$`. The value is at most 64 KiB.
- A write requires the package to be **enabled** and its manifest to declare
  `package_kv/v1`. It is stamped with the manifest's `schema_version`.
- Reads are allowed while installed or enabled. A datum whose schema version
  is not in the reads window is refused, never silently returned.
- Purge under `delete_data` tombstones every datum of the package in the
  same batch as the purge event. Under `retain_data` the datums stay. The
  inventory records the retained schema version, and a later install whose
  `reads_schema_versions` excludes it is refused.
- Purge removes data from the computer's **current state**. The tape is
  immutable, so a restore to a pre-purge head brings the data back. That is
  the meaning of `data_scope: computer_tape`. Erasing the history itself
  (crypto-shredding private payloads) is a named residual (R3), not a v1
  claim.

### 5.5 Concurrency

- Every lifecycle act and every datum write runs under one store mutex
  (`appPackageMu`). Each one reads the current fold, validates, and writes
  its batch.
- A not-exists condition on the event's canonical id (derived from `seq`)
  backs the mutex up.
- The guest runtime is the only writer, so the mutex is sufficient. The
  condition exists for defence in depth and for tests.

## 6. Bytes: the guest build store and package sources

- **Build store** (`apppackage.BuildStore`): a content-addressed directory,
  `<root>/builds/<build hex>/` holding `build.json` plus the files. `Put`
  verifies every file's digest and size against the receipt, writes into a
  temporary directory and renames it into place. `Open` re-verifies the
  digest on every read, so a tampered file is refused rather than served.
  Default root: `<store dir>/app-packages`.
- **Package source** (`apppackage.Source`): `Lookup(build_digest)` returns
  `(Build, files)`. S7-0a implements `CatalogSource`, a read-only directory
  of host-built packages with the same layout. Its default is
  `$CHOIR_UPDATER_ROOT/current/app-packages`, so the catalog is part of the
  signed release. `CHOIR_APP_PACKAGE_CATALOG` overrides it. The host CAS
  source is S7-0c.
- **Install path**:
  1. resolve the build from the local build store, then from the sources;
  2. check its manifest and digests (the reducer runs the same checks);
  3. `Put` it into the local build store;
  4. append the install event.

  Bytes are stored before the event, so the tape never names a build that is
  not on disk. Builds are never deleted in S7-0a, so a restore to any head
  finds its bytes (garbage collection is R4).

## 7. Backend service (designed, not built)

- **Artifact.** A host-built, statically linked Linux executable for the
  **base image's** architecture and ABI, not for the computer's app layer.
  It depends only on the non-forkable base, which is what lets one build run
  on every diverged computer. Recipe `go-service/v1`: `CGO_ENABLED=0`,
  pinned toolchain, `-trimpath`, reproducible.
- **Process.** The guest runtime supervises one process per enabled package
  as a transient unit. It runs under the capsule broker's existing sandbox
  primitives: its own cgroup with a memory ceiling, seccomp, Landlock with
  no filesystem writes, and no network except the S4 grants the manifest
  declares.
- **Interface.** The service listens on a unix socket handed to it. The
  runtime reverse-proxies `/api/app-packages/<name>/svc/*` to that socket
  while the package is enabled and refuses the path otherwise.
- **State.** The service has no disk of its own. It reads and writes only
  through the package data API, over a capability-scoped socket, so its
  state is on the tape and restore covers it. A service that needs a real
  database is a later recipe with its own restore contract.
- **Lifecycle.**
  - enable appends its event, then starts the service;
  - disable appends its event, then stops it;
  - at boot, the services of enabled packages start, because a package
    service is a standing owner intent, like an Autopaper source schedule
    (CLAUDE.md "Restarts End Work");
  - a service that crashes the computer twice is disabled with that cause.
- **Why not WASM first.** A WASI component would be more portable still, but
  it cannot yet reach the sockets and libraries the first converted backends
  (Mail, Calendar) use today. This is open decision O2.

## 8. Host build hook (designed, not built)

The host builds packages from **published source** only. The flow joins
P5's host builder (`internal/builder`, `cmd/choir-builder`, extended on
`feat/p5-host-build-layered-apply`) as a second job kind; it is not a
parallel builder.

1. **Publish.** A computer, or the owner's CLI, publishes the package source
   tree as an S8 source object, `(manifest, tree)`, to the host's
   content-addressed store. The host recomputes `source.digest` and refuses
   a mismatch.
2. **Build.** `choir-builder package --source <digest>` runs the manifest's
   recipe in a sandboxed Nix build:
   - pinned toolchain from the platform flake at a recorded builder revision;
   - no network;
   - `static/v1` copies the outputs;
   - `vite-module/v1` bundles one self-contained ES module, Svelte runtime
     included (D5);
   - `go-service/v1` builds the static executable (§7).

   The output digest depends only on (source digest, recipe, toolchain
   digest).
3. **Sign and store.** The host writes the build receipt, adds `toolchain`
   and `builder_revision`, signs it with a `receiptsigner` key in a
   dedicated `package-builder` domain, and stores receipt plus files in the
   host CAS keyed by build digest.
4. **Install anywhere.** A computer installs by build digest. Its
   `HostCASSource` (S7-0c) fetches the signed receipt and files, the same way
   capsules fetch modules from the Go module proxy. It verifies the
   signature and digests, and then follows §6.
5. **Reproducibility check.** Any builder, including a second host or an
   auditor, can rebuild from the published source and compare build
   digests. A mismatch is a host-builder incident, not a client decision.

What P5 owns versus S7: P5 builds **one computer's app layer** (base plus
patch) for that computer. S7 builds **one package** once, for every
computer. They share the builder library, the sandbox, the toolchain pin and
the signer; they differ in job kind and in output identity.

## 9. Frontend loading

- `frontend/src/lib/apps/packages.ts` holds the package app store.
  `refreshInstalledPackages()` calls `GET /api/app-packages` after the
  authenticated desktop is ready. Each enabled package becomes a
  `ChoirAppDefinition` with id `package:<name>`, so it can never collide
  with a static id. Its component is `PackageAppFrame.svelte`.
- `getAppDefinition` consults the static registry first and the package
  store second. The static registry is unchanged and still wins.
- The DeskSheet lists the static desk apps and then the enabled packages.
  Desktop icons for packages wait for S7-0b.
- `PackageAppFrame`:
  1. fetch the module from `GET /api/app-packages/builds/<build>/<entry>`;
  2. compute its SHA-256 with `crypto.subtle` and compare it with the
     inventory digest;
  3. import the bytes from a `blob:` URL;
  4. call `mount(target, ctx)`, where `ctx` carries `package`, `version`,
     `windowId` and a `data` client bound to the package's data API;
  5. call `destroy()` on unmount.

  A digest mismatch, a missing `mount`, or a throw from `mount` all render a
  contained error in the window. None of them reaches the shell.
- A window restored from desktop state for a package that is no longer
  enabled shows "not enabled", and nothing is fetched.

## 10. Next slices

- **S7-0b: convert the first-party apps.**
  - Move each `APP_REGISTRY` entry to a package under `packages/` with a
    `vite-module/v1` recipe, starting with the smallest self-contained
    viewers (Image, Audio, Video, PDF) and ending with Texture.
  - Add a Settings "Apps" pane over the install, enable, disable and purge
    API.
  - Map each app's data to the shared object-graph kinds named in the S7
    definition.
  - Retire each static entry as its package lands. The static registry
    shrinks to the shell.
- **S7-0c: the host package build job and `HostCASSource`.**
  - Requires P5's host build service.
  - Signed receipts, and install from the host cache.
- **S7-1: update and data migration.** Install over a present package, a
  migration declared in the manifest, and interrupted-migration recovery
  (the definition's acceptance 3).
- **S7-2: the backend service (§7).**
- **Deployed proof** of the definition's acceptance rows, after D1 is
  ratified.

## 11. Ceremony

- **Mutation class.**
  - Docs: green.
  - Tests: yellow.
  - Runtime: orange. The changes are new guest routes, a new frontend
    loading path, and new registered object kinds written through the
    existing tape interceptor.
  - Code on a protected surface: none changed.
  - Canonical state written: yes, so red ceremony applies.
- **Conjecture delta.** `package-manifest-is-one-release` becomes
  `package-presence-is-a-tape-fact` (D1): one object, the manifest plus the
  build receipt bound into an install event, stays the same object when a
  backend and migrations are added, because they are new receipt fields and
  new ops on the same chain. Evidence class reached: local unit and
  component tests only. No deployed proof.
- **Protected surfaces touched.** The canonical event commit path, as a
  client only, through `ogStore` → projection interceptor →
  `AppendNewPayload`. Nothing else.
- **Rollback.** Revert the branch's commits. Package objects already on a
  computer's tape stay as inert `og_objects` rows that no reverted binary
  reads. Replay stays safe (D2). Disable or purge before revert if a package
  window should vanish at once.
- **Heresy delta.**
  - Discovered: the S7 definition's "release member" mechanism conflicts
    with the 2026-10-14 build-once ruling (D1). The registry still carries
    three unwritten edges from the retired app-package design
    (`package_source_computer`, `adoption_package`,
    `adoption_target_computer`); they are left for S8, which owns adoption.
  - Introduced: none intended. The risk to watch is R1.
  - Repaired: none.

## 12. Failure modes

Each failure mode was written before its test, and each names the test that
covers it.

### Manifest and source

| # | Failure | Guard | Test |
| --- | --- | --- | --- |
| F1 | Unknown schema or unknown field is half-read | strict decode, exact schema | `TestManifestRejects/schema`, `/unknown-field` |
| F2 | Package name collides with a static app or shell name | reserved list; Go test keeps it in sync with `registry.ts` | `TestManifestRejects/reserved-name`, `TestReservedNamesCoverStaticRegistry` |
| F3 | Path escapes the package (`..`, absolute, backslash) | `safeRelPath` | `TestManifestRejects/entry-*` |
| F4 | Entry not among build outputs | cross-field check | `TestManifestRejects/entry-not-output` |
| F5 | Malformed digest | `sha256:` + 64 lowercase hex | `TestManifestRejects/source-digest` |
| F6 | Unknown recipe | closed set | `TestManifestRejects/recipe` |
| F7 | Data window excludes its own schema version; store `none` with a schema version | cross-field check | `TestManifestRejects/data-*` |
| F8 | Unknown restore requirement | closed set | `TestManifestRejects/restore-*` |
| F9 | Grants declared before S4 | refused | `TestManifestRejects/grants` |
| F10 | Fixture source edited without a manifest bump | tree digest | `TestHelloFixtureSourceDigest` |

### Build

| # | Failure | Guard | Test |
| --- | --- | --- | --- |
| F11 | Build binds another manifest, source or recipe | install binding (§5.3) | `TestReduceInstallBinding/*` |
| F12 | Stored or served bytes differ from the receipt | verify on `Put` and on `Open` | `TestBuildStoreRefusesTamper` |
| F13 | Non-deterministic build digest | sorted files, canonical JSON | `TestBuildStaticDeterministic` |

### Lifecycle and data

| # | Failure | Guard | Test |
| --- | --- | --- | --- |
| F14 | Double install treated as an update | `ErrAlreadyInstalled` | `TestReduceTransitions` |
| F15 | Op on an absent package | `ErrNotInstalled` | `TestReduceTransitions` |
| F16 | Purge while enabled | `ErrInvalidTransition` | `TestReduceTransitions` |
| F17 | Lost, duplicated or reordered event read as valid state | seq/prev chain | `TestReduceRefusesBrokenChain` |
| F18 | Backend package installed before §7 exists | refused at install | `TestReduceInstallBinding/backend` |
| F19 | Purge under `delete_data` leaves data | tombstones in the same batch | `TestStoreAppPackageLifecycleOnTape` |
| F20 | Retained data reinstalled under an incompatible schema | `retained` in the inventory | `TestReduceRetainedDataCompat` |
| F21 | Data write to a disabled, absent or data-less package | store check | `TestStoreAppPackageDataGuards` |
| F22 | Oversized value or hostile key | limits | `TestStoreAppPackageDataGuards` |
| F23 | Restore to a pre-install head still shows the package | presence is the fold of tape objects | `TestStoreAppPackageReplayPrefix` (replays a prefix of the recorded tape into a fresh store) |
| F24 | Older binary poisons on package objects | OG objects, no new op kind (D2) | structural; `TestStoreAppPackageReplayPrefix` uses the generic projector |
| F25 | Concurrent lifecycle acts fork the chain | mutex plus not-exists condition | `TestStoreAppPackageConcurrentInstall` |

### Serving and loading

| # | Failure | Guard | Test |
| --- | --- | --- | --- |
| F26 | Serving a build no enabled package binds | inventory join | `TestAppPackageHandlerServesOnlyEnabledBuilds` |
| F27 | Serving a path outside the receipt | receipt file list | same |
| F28 | Install request carries bytes | API accepts a digest only | `TestAppPackageHandlerInstallTakesDigestOnly` |
| F29 | Unauthenticated lifecycle call | `requireAuth` | `TestAppPackageHandlerRequiresAuth` |
| F30 | Browser runs bytes not bound on the tape | browser digest check | Playwright `app-packages.spec.js` "refuses bytes" |
| F31 | Module without `mount`, or a throwing `mount`, breaks the shell | contained error | Playwright "contains a broken module" |
| F32 | Restored window for a disabled package loads code | "not enabled" state | Playwright "disabled package" |

### Residual risks (named, not closed by S7-0a)

| # | Risk | Plan |
| --- | --- | --- |
| R1 | Same-origin package JS has full DOM and cookie reach. S7-0a accepts this only because the only installable source is the host-built catalog shipped in the release. | Isolation (a sandboxed iframe on a package origin, `postMessage` ctx) is required before any third-party package. It is an S7-0c gate. |
| R2 | Multi-file frontend packages: a `blob:` import cannot resolve relative imports. | `vite-module/v1` emits one self-contained module; import maps come later. |
| R3 | Purge does not erase tape history. | crypto-shred of package-data payloads, later. |
| R4 | Builds are never garbage-collected. | GC by reachability from retained checkpoints, later. |
| R5 | D1 is orchestrator-settled. | owner ratification before S7 acceptance. |
| R6 | The catalog directory is not yet populated by the Nix release build. | S7-0b adds `packages/*` builds to the release closure. Until then, installs on staging have no source. |
