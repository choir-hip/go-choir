# App packages, replanned under I27 (2026-10-24)

Status: **design, green.** No code. Written by an FP0 design worker on lane
`lane/fp0-s7-replan` from `origin/main` at `d7437aabc`. Goal file:
[choir-appdev-s7-app-packages-2026-10-01.md](../definitions/choir-appdev-s7-app-packages-2026-10-01.md).

Owner direction (ledger 19:3x and FP0's relay): app packages come first. They
are the platform upgrade the self-development program needs before its first
real loop: a coding agent asks a test computer, through the choir CLI, for one
Reader app that does what the PDF and EPUB readers do. They are also the base
of tent pole 1, where apps become app agents with their own RLM desks (doctrine
`I26`). The owner expects packages in an hour or two of agent work, so the
slices below are sized for parallel lanes that land in hours, and the first
slice is as small as it can be while still real.

Evidence labels: **[V]** read at file:line on this tree; **[B]** read on the
paused branch `origin/feat/s7-0-app-packages`; **[L]** read in the conductor's
log `/Users/wiz/Dev/lanes/_runs/apprewrite/log.md` (outside the repo);
**[H]** hypothesis, with what would settle it.

## The answer in one page

1. **The first slice (P1) is frontend only and takes one or two hours.** A
   first-party app becomes a directory `frontend/src/packages/<name>/` holding
   a manifest `choir-package.json` and its Svelte entry. The shell finds the
   packages at build time with `import.meta.glob`, so no registry edit is
   needed to add an app. A check script, run by `npm run build`, refuses a
   malformed manifest, a name or alias that collides with a static app, and
   an import from outside the package other than the shell's app SDK. P1
   converts one real app, the image viewer, to prove the format. The Reader
   is then the first package that self-development writes.
2. **The I27 delta (P2) replaces `package_kv` with package-declared kinds.**
   The manifest declares its kinds (`choir.app.<package>.<kind>`) and their
   schemas. A generated table (with a CI drift check) registers them at boot
   through one exported seam onto the write gate's registry. Each kind's
   writer is `desk:<package>`, with owner edits admitted, so until the desk
   exists the only writer is the owner's own window, through one generic
   object route. Replay already projects any kind without a registry lookup,
   so a rollback leaves inert rows, not poison.
3. **Convert the rest after the Reader, in parallel.** S7-0's "convert every
   existing app first" would hold the self-development loop for a day or more
   while it proves nothing that one conversion does not. One conversion (P1),
   then the Reader by self-development, then the other viewers and apps in
   parallel lanes of under an hour each. Texture and Settings stay static.
4. **The paused branch is a parts bin, not a base.** It is 2,248 commits
   behind main and conflicts in four files. Its manifest validation, build
   receipt, hash-chained lifecycle reducer, build store and verified frontend
   loader are reusable for the later runtime-install slice (P5). Its
   `package_kv/v1` data model is the second home `I27` forbids.
5. **App packages are not the long pole of the loop.** Beyond U1 and U1b, the
   loop also needs these: a way for an accepted change to reach main, a
   Texture document that follows the run past the decision, checkpoints that
   ordinary desk work cannot make ineligible, and model routing that is not a
   hand-edited file. They are listed with sizes in section 5.

## 1. What exists

### 1.1 On main (`d7437aabc`)

| Path | What it is | State |
|---|---|---|
| `frontend/src/lib/apps/registry.ts` | Static `APP_REGISTRY` of 17 apps (`files` to `settings`, Texture among them, `:55-255`); every lookup, launcher list and heavy set derives from that array (`:257-283`) [V] | live; the only app registry |
| `frontend/src/lib/media-utils.js:4-12` | `MEDIA_FILE_ROUTES`, the static table of which app opens which file type (`pdf` and `epub` route to their own apps) [V] | live |
| `frontend/src/lib/PdfApp.svelte` (842 lines), `EpubApp.svelte` (892), `ImageApp.svelte` (420) | app components; their imports reach `./live-events.js`, `./media-utils.js` and others by relative path [V] | live |
| `internal/objectgraph/registry.go:36-52` | `KindRegistration` with `Writer` and `OwnerEdits` (ARCH-04) [V] | live |
| `internal/objectgraph/registry.go:72-185`, `:301` | `DefaultRegistry()`, compiled; `writeRegistry` is a private package variable built from it [V] | live; no runtime registration |
| `internal/objectgraph/writer.go:73-90` | `ValidKindWriter` accepts `desk:<name>` (`^desk:[a-z][a-z0-9_-]{1,31}$`) [V] | live |
| `internal/objectgraph/writer.go:222-256` | `CheckWriteAs`, the write gate, reads `writeRegistry`; mode defaults to observe (`:136-147`) [V] | live |
| `internal/objectgraph/writer.go:270-296` | `RegisterKindChecked`: one valid writer per kind, no second writer [V] | exists; only tests call it; no exported way to reach `writeRegistry` |
| `internal/objectgraph/dolt_store.go:630,643,1138` | every `og_*` write passes `CheckWriteAs`; an unregistered kind is refused (`registry.go:315-329`) [V] | live |
| `internal/store/project.go:231-271` | `projectObject`, the replay projector, upserts any kind with no registry lookup [V] | live; the property that makes package kinds safe to roll back |
| `internal/appdesk/appdesk.go` (372 lines) | the package-to-desk join: `Manifest`, typed `Validate`, `Derive` (fold of install and disable events), `Activate`, `CHOIR_APP_DESKS` (off by default) [V `:1-22`, `:243-300`] | on main, no importer outside its tests |
| `internal/deskmanifest/manifest.go:55-56,489-560` | `KindApp` and `NewAppDesk`, ARCH-02's app-desk rows [V] | live |
| `internal/appverbs/appverbs.go` | per-app verb registry [V] | no callers, off |

There is no package manifest, no package directory and no runtime package
route on main.

### 1.2 On the paused branch (`d9340a9a0`, `5fe486f52`, `4bc762102`, 2026-10-14)

`git merge-tree` against main conflicts in four files
(`docs/definitions/choir-appdev-s7-app-packages-2026-10-01.md`,
`frontend/src/lib/PromptSurface.svelte`, `internal/objectgraph/registry.go`,
`internal/store/store.go`); the branch base is 2,248 commits behind main.

| Branch file | What it does [B] | Verdict |
|---|---|---|
| `docs/design/s7-0-app-packages-2026-10-14.md` (542) | design D1 to D6, manifest v1, build receipt, lifecycle, backend and host build designs, 32 failure modes | **Reuse** D1 (presence is a tape fact; owner ruling D1 since), D3 (seq and prev chain), D4 (install takes a digest), D6 (verify bytes before running them), and failure modes F1 to F17 and F23 to F32. **Replace** §5.4 (`package_kv/v1`) and F19 to F22 under `I27` |
| `internal/apppackage/manifest.go` (352) | strict decode, `Validate`, name grammar, `reservedNames` (`:75-92`), digests | **Reuse with changes**: drop `data.store`; add `kinds`, `opens`, `app` (window fields) and `origin`; reserved names become "held by the static registry or another package", because a converted app keeps its own id |
| `internal/apppackage/build.go`, `buildstore.go` (479) | `static/v1` build receipt; content-addressed build store, re-verified on every read | **Reuse** for P5, unchanged in intent |
| `internal/apppackage/reducer.go` (376) | pure fold of lifecycle events with binding checks; effect `DeleteData` | **Reuse** for P5 without the data effect: purge marks the package's kinds retired (readable, not writable) as `appdesk` does, and never deletes another package's data |
| `internal/store/app_packages.go` (251) | lifecycle events as `choir.app_package_event` objects; datum objects | **Reuse** the event half (`:36-140`). **Drop** the datum half (`:141-251`): that is the `package_kv` silo |
| `internal/autoputer/app_packages.go` (426) | routes: inventory, catalog, install, enable, disable, purge, `data/{key}`, `builds/{hex}/{path}` | **Reuse** all but `data/{key}`, which P2's object route replaces |
| `internal/objectgraph/registry.go` (+5) | registers `choir.app_package_event` and `choir.app_package_datum` without a `Writer` | **Conflicts**: main requires a writer per kind, and the datum kind is the silo. P5 registers the event kind with `Writer: system:runtime` |
| `internal/store/store.go` (+1) | `appPackageMu` | trivial; re-add in P5 |
| `internal/autoputer/run.go` (+3) | registers the routes | re-add in P5 |
| `frontend/src/lib/apps/packages.ts` (223), `PackageAppFrame.svelte` (140) | runtime inventory fetch; ids `package:<name>`; framework-free `mount(target, ctx)` module loaded from a verified `blob:` URL | **Reuse** for installed packages in P5. In-tree first-party packages (P1) do not need it: they are built with the shell |
| `frontend/src/lib/{AppHost,Desktop,PromptSurface}.svelte`, `registry.ts` (+19) | hook the runtime inventory into the shell | **Rewrite** on main's versions in P5; `PromptSurface` conflicts |
| `packages/hello/*`, `frontend/tests/app-packages.spec.js` | fixture package and Playwright spec | **Reuse** in P5 as the installed-package fixture |

What the branch did not have, and main now requires: a writer per kind
(ARCH-04), a desk member (ARCH-03, `internal/appdesk`), and `I27`.

## 2. The I27 delta

`I27`: one home per kind; each kind belongs to the package that declares it
and its schema travels with that package; extension only; a different app is
a new app in its own namespace, and data that already has a kind stays on that
kind. The branch's `package_kv/v1` (`choir.app_package_datum`, keyed
`(package, key)` with an opaque 64 KiB JSON value) is named as a planned
second home in
[data-has-more-than-one-home-2026-10-24.md](../problems/data-has-more-than-one-home-2026-10-24.md)
(violation 9) and in `H038`. It goes.

### 2.1 What replaces it

A manifest declares kinds:

```json
"kinds": [
  {
    "kind": "choir.app.slides.deck",
    "identity": "external_key",
    "versioned": true,
    "owner_edits": true,
    "schema_version": 1,
    "reads_schema_versions": [1],
    "schema": { "type": "object", "required": ["title", "slides"],
                "properties": { "title": { "type": "string", "maxLength": 200 },
                                "slides": { "type": "array" } } }
  }
],
"uses_kinds": ["choir.content_item"]
```

Rules, enforced in Go (`apppackage.Validate`) and in the frontend check script:

- **Namespace.** Every declared kind is `choir.app.<package>.<kind>`, where
  `<package>` is the manifest's `name`. A package cannot declare a kind in
  another namespace. A converted first-party app keeps a kind it already
  writes under its current name only through an explicit `legacy_kinds` list,
  validated one by one against `DefaultRegistry` (the default of
  `app-package-carries-app-desk` §7.2).
- **One writer.** Each declared kind is registered with `Writer:
  desk:<package>` and `OwnerEdits` from the manifest. Before the package has a
  desk, the only live writer is the owner, through the package's window. When
  the desk lands (P4), it is the one agent writer, with no change to the kind.
- **Schema.** A small JSON Schema subset (`type`, `required`, `properties`,
  `items`, `enum`, `maxLength`, `maximum`). The body is validated before the
  write. `KindRegistration.Schema` records `pkg:<name>@<schema_version>` so
  the registry names where the schema lives. `reads_schema_versions` carries
  the branch's compatibility window over to kinds.
- **Extension.** A package that adds fields to another package's data
  declares its own kind and an edge to that kind. It never writes the other
  kind directly. `uses_kinds` names the kinds it reads, and is informational
  in v1.
- **Data that already has a kind stays on it.** The Reader declares no kind:
  reading position stays on `media_progress` (`/api/media/progress`, an event
  projection since D39) and the file stays a file. When the file-object lane
  lands `choir.file`, the Reader opens files by object id.

### 2.2 The registry change

Today kinds are compiled into `DefaultRegistry` and the gate reads a private
`writeRegistry` (`registry.go:301`, `writer.go:241`). The smallest safe form
has three parts.

1. **One exported seam**, in a new file `internal/objectgraph/package_kinds.go`
   so it does not collide with lanes editing `registry.go`:
   `RegisterPackageKind(pkg string, k KindRegistration) error`. It refuses a
   kind outside `choir.app.<pkg>.`, any writer but `desk:<pkg>`, and a kind
   already in `DefaultRegistry`. It then calls
   `writeRegistry.RegisterKindChecked(k)`, which already refuses a second
   writer. Registration is idempotent and append-only for the life of the
   process; nothing deregisters.
2. **First-party kinds come from the manifests at build time.** A generator,
   `frontend/scripts/gen-package-kinds.mjs`, reads
   `frontend/src/packages/*/choir-package.json` and writes
   `internal/apppackage/firstparty_gen.go`. It runs with `--check` in CI, as
   `generate-source-contract.mjs --check` already does for the source
   contract (`frontend/package.json:8`). At boot, `apppackage.RegisterFirstParty()`
   calls the seam for each kind, before the store opens for writes. The kinds
   ship in the binary that serves them, so a release that adds a kind is a
   release that can write it.
3. **Installed packages register at install** (P5). The same seam is called
   from the fold of install events on the tape, at boot and on each install
   event, the way `appdesk.Derive` folds desks. A restore to a pre-install head
   folds to a set without the package. Its kinds stay readable and are
   refused by the gate, since nothing registers them.

Why this is safe to roll back: `projectObject` (`store/project.go:231`) projects
any kind with no registry lookup. A binary without the seam replays package
objects as inert `og_objects` rows and refuses only new writes of them.

Not in this delta: a per-package restore, crypto-shredding on purge (the
branch's R3), and the org-shared graph scope.

## 3. The smallest first slice (P1), and the conversion order

### 3.1 P1: a first-party app is a package, frontend only

- **Layout.** `frontend/src/packages/<name>/choir-package.json` and
  `frontend/src/packages/<name>/App.svelte`, plus any files the app needs,
  all inside the directory. It sits under `frontend/` because the Nix
  frontend build takes `src = ./frontend` (`flake.nix:306,347`) and the
  capsule builds the same tree. A top-level `packages/` would need a flake
  fileset change and a nix eval of both systems (open decision 1).
- **Manifest v1 (the contract P1 and P2 share).** `schema:
  "choir.app-package/v1"`, `name`, `version`, `origin: "first_party"`,
  `display {title, icon, description}`, `app {entry, abi:
  "choir.app-frontend/svelte-inline/v1", aliases[], launcher, window, auth,
  theme}` (the fields of `ChoirAppDefinition`, `registry.ts:13-38`), `opens
  [{media_type, extensions[]}]`, and `kinds[]`, `uses_kinds[]`, `desk: null`,
  `backend: null`, `grants: []`. P1 refuses a non-empty `kinds` until P2
  lands. The `origin: "installed"` form (with `source`, `build`) is P5's.
- **Discovery.** A new file, `frontend/src/lib/apps/packages.ts`, reads every
  manifest with `import.meta.glob('../../packages/*/choir-package.json',
  { eager: true })` and every entry with a lazy
  `import.meta.glob('../../packages/*/App.svelte')`. It maps each to a
  `ChoirAppDefinition` whose id is the package name and whose aliases resolve
  to it. `registry.ts` concatenates the static entries and the packages, so
  every list derived from `APP_REGISTRY` (`:257-283`) includes packages with
  no other change. `MEDIA_FILE_ROUTES` (`media-utils.js:4`) adds each
  package's `opens` first.
- **The check** (`frontend/scripts/check-packages.mjs`, added to the `build`
  script, so it runs in CI and in the Nix build, and in the capsule whenever engineering builds with `npm run build` [H: whether the capsule build job calls the npm script or vite directly; settled by reading the job command in an engineering run]). It
  refuses an unknown schema or field; a name outside `^[a-z][a-z0-9-]{1,39}$`;
  a name or alias that equals a static registry id, a shell name or another
  package's name or alias; an `opens` extension claimed twice; and an import
  in the package that leaves its directory, other than `svelte`, a declared
  npm dependency, or `frontend/src/lib/app-sdk.ts`.
- **The app SDK.** `frontend/src/lib/app-sdk.ts` re-exports the shell
  functions a package may use (the auth fetch, live events, media helpers,
  theme). It is the package's platform API, and the start of the "platform API
  range" a lock entry declares later (update design §4.4).
- **The conversion.** Move `ImageApp.svelte` into `frontend/src/packages/image/`,
  point its imports at `app-sdk.ts`, delete the `image` static entry and its
  `MEDIA_FILE_ROUTES` row. The id stays `image`, so saved windows and file
  routes keep working.
- **Why the image viewer.** It is the smallest file-opening app (420 lines)
  and it stays, unlike PDF and EPUB, which the Reader replaces. It is the
  template self-development copies.

**The Reader then.** The self-development request is now "add a package
`reader` with aliases `pdf` and `epub` and `opens` for `.pdf` and `.epub`;
delete the static `pdf` and `epub` entries and their routes". The check
forces both deletions (an alias equal to a static id fails the build). The
change sits inside `frontend/src/packages/reader/` plus two deletions, which
is the package-boundary change that survives base moves (update design §4.4,
"self-development should be steered toward package boundaries").

**The Reader does not need U1 or U1b if it runs on a fresh test computer.**
The capsule seeds from the checkout made when the data image is created
(`self-development-candidates-build-from-the-creation-commit`). A test
computer minted after P1 deploys therefore has the packages in its seed. On
b@c, which was seeded at `0cbc52edc`, it waits for U1b.

### 3.2 Convert the rest before or after the Reader?

**After.** Converting every app first (owner direction 2026-10-14, in the
goal file) was the right order when packages were the point of S7-0. Now the
point is the self-development loop, and three facts change the order:

1. One conversion proves the format, the check and the SDK. Fifteen more
   prove nothing new about the seam.
2. Each conversion is a separate, mechanical lane of under an hour that owns
   one directory and deletes one registry line. They can run in parallel with
   the Reader and with P2, and nothing waits on them.
3. The Reader replaces two static apps. Converting PDF and EPUB first would
   be work thrown away within the hour.

Texture stays static (`app-package-carries-app-desk` §7.3: a system-tier
desk that ships in the image) and so does Settings, which is the owner-act
surface of the shell. `compute-monitor` and `pulse` are shell views [H:
whether they count as apps; settled by the owner's list]. The owner's 10-14
words were "apply that to all the existing apps": this order still does that,
in the same day, after the Reader. It reorders the owner's ruling, so it is
listed under contradictions.

### 3.3 P1 failure modes (written before the code, 2026-10-24)

Each row names the failure, what stops it, and the check that proves it.
`check-packages.mjs` (manifest, names, directory rules, required floor) takes `--root <frontend dir>` so the spec can run it
against a fixture tree and watch it refuse each bad case.

| # | Failure | Stop | Proof |
|---|---|---|---|
| P1-F1 | `choir-package.json` is not JSON, has an unknown or missing field, or the wrong `schema` | the check fails the build (strict allow-list of keys at every level) | spec: bad-json, unknown-field, wrong-schema fixtures |
| P1-F2 | the name breaks `^[a-z][a-z0-9-]{1,39}$`, or differs from its directory name | check | spec: bad-name, dir-mismatch |
| P1-F3 | a name or alias equals a static registry id, a reserved shell or system-desk name (`desk`, `desktop`, `shell`, `prompt`, `system`, `owner`, `management`, `engineering`, `research`), another package's name or alias, or its own name twice | check | spec: static-id, reserved, package-clash, alias fixtures |
| P1-F4 | an `opens` extension is claimed by two packages, or by a route still in the static `MEDIA_FILE_ROUTES` | check | spec: extension-clash fixtures |
| P1-F5 | the package reaches a file outside its directory or the SDK: a relative, absolute, bare-path, `?raw`/`?url`, glob, dynamic, CSS or asset-URL import, or a runtime reach (`eval`, `Function`, workers, `import.meta` other than `env`/`hot`, a non-literal `import()`) | the Vite plugin `scripts/package-boundary-plugin.mjs`, on the resolved module graph: it refuses any module imported from `src/packages/<name>/` that resolves outside that package, except `src/lib/app-sdk.ts`, the svelte runtime and declared npm dependencies; token rules on the compiled module and on package CSS cover what Vite does not resolve through plugins. The first design scanned import syntax with regexes and was sent back twice (bunch201, bunch203): syntax has too many spellings, resolution has one | spec: 40 fixture rows (static, spaced and comment-split, dynamic, glob, `?raw`, `?url`, absolute, traversal, other package, undeclared dependency, eval, `new URL`, `Worker`, CSS `@import` and `url()`) must fail the build, and controls must pass; the image viewer builds |
| P1-F6 | `entry` is missing, outside the directory, or `abi` is unknown | check | spec: missing-entry, bad-abi |
| P1-F7 | `kinds` is non-empty, or `desk`, `backend` or `grants` is set (P2 and later own these) | check refuses until the slice that owns them lands | spec: kinds fixture |
| P1-F8 | `npm run build` is bypassed (vite called directly) and a bad package ships | the loader is defensive: a package whose name is already a static id, or whose entry file is missing, is skipped with a console warning, never crashes the shell, and the static entry wins | spec: browser test loads the shell with no console error from the loader; static entries unchanged |
| P1-F9 | a saved desktop window, a file route or a prompt route for `image` stops resolving after the move | the package id is the old id, with the same icon, name, launcher, window and theme fields | spec: golden object equality on `getAppDefinition('image')`, `mediaRouteForFileName('x.png')`, the launcher click |
| P1-F10 | the check silently finds no static ids (registry layout changes) and so passes every clash | the check fails when it parses fewer static ids than a floor of 10, or no static routes | spec: empty-registry fixture |
| P1-F11 | a source test that reads `src/lib/ImageApp.svelte` by path breaks on the move | the one path in `computer-live-sync.spec.js` follows the file | `computer-live-sync.spec.js` runs |

**What this check is.** The package boundary is an architectural boundary,
not a security boundary. It keeps a package's imports inside its own folder,
the app SDK and declared dependencies, so a package can be moved, replaced or
written by self-development without silent coupling to shell internals. It
does not sandbox the code: a package runs same-origin in the shell's page and
can call any `/api` route the signed-in user can, read `fetch('/src/...')` in
dev, or build an access path at runtime that no static rule can see. Authority
therefore lives in the backend (the write gate, desk authority, route auth),
never in this check; sandboxed third-party packages are P6 (a frame).

## 4. Slice sequence

Sizes are agent working time, excluding review queue and CI wait (CI and the
staging deploy add about 40 to 60 minutes to each landing). "Owns" lists the
files a lane may edit, so that lanes do not collide.

| Slice | What | Class, review tier | Depends on | Owns | Size |
|---|---|---|---|---|---|
| P0 | this design | green, tier 0 | — | this file | done |
| P1 | first-party package format, build-time discovery, check, SDK, image converted (§3.1) | orange, tier 1 (Devin) | — | `frontend/src/packages/**` (new), `frontend/src/lib/apps/packages.ts` (new), `frontend/src/lib/app-sdk.ts` (new), `frontend/scripts/check-packages.mjs` (new), `frontend/package.json` (build script line), `frontend/src/lib/apps/registry.ts` (concat, image entry out), `frontend/src/lib/media-utils.js` (two lines), `frontend/src/lib/ImageApp.svelte` (moved), `frontend/tests/app-packages.spec.js` (new) | 1.5 to 2 h |
| P2a | the registry seam `RegisterPackageKind` with the namespace, writer and built-in checks | red, tier 2 (Devin, DeepSeek): a wrong check silently admits a writer to a canonical kind | — | `internal/objectgraph/package_kinds.go` (+ test) only | 1 h |
| P2b | manifest Go type (ported from the branch without `package_kv`), schema subset validator, kind generator and drift check, boot registration, the generic owner-edit object route `/api/app-packages/{pkg}/objects/{kind}[/{id}]` behind `CHOIR_APP_PACKAGE_OBJECTS` (default off) | red, tier 1 (Devin): additive, behind a switch, one-act rollback | P1's manifest shape (agreed here, so it can start at once), P2a to merge | `internal/apppackage/**` (new), `internal/autoputer/app_package_objects.go` (new), one line in `internal/autoputer/run.go`, `frontend/scripts/gen-package-kinds.mjs` (new), one CI line | 3 to 4 h |
| P3 (×n) | convert one app per lane: audio, video, podcast, slides, calendar, files, browser, email, autopaper, content | orange, tier 0 by default (mechanical move under the P1 check; the build is the review), tier 1 for files and email | P1 | `frontend/src/packages/<app>/**`, that app's registry line and its route row | 20 to 60 min each |
| R | the Reader, built by self-development on a fresh test computer through the choir CLI, owner judges, apply; then landed on main (section 5, item A) | the candidate is orange; its landing on main is tier 1 | P1 deployed; a fresh test computer (or U1b for b@c) | `frontend/src/packages/reader/**`, `pdf` and `epub` out | 1 to 2 h of loop, plus the landing |
| P4 | the desk join (ARCH-03) for first-party packages: `desk` member embedded in the manifest, rows derived from the manifests at boot, `appdesk.Activate`; kinds' writer becomes the live desk | red, tier 2 (a new authority path) | P2b, ARCH-02 rows (on main) | `internal/appdesk/**`, `internal/apppackage/desk.go` (new) | 4 to 6 h [H: the daemon's wake and inbox wiring for an app desk is unmeasured; settled by reading `deskmanifest.RegisterApp`'s callers] |
| P5 | runtime install from the host-built catalog: the branch's lifecycle reducer, build store, routes and verified loader, rebased; kinds and desks of installed packages derived from the install fold; lock-entry fields (channel, policy, pin) from day one (U7) | red, tier 2 (tape, a new code-loading path) | P2b; catalog bytes in the release (branch R6) or the host build job | `internal/apppackage/{reducer,build,buildstore}.go`, `internal/store/app_packages.go`, `internal/autoputer/app_packages.go`, `frontend/src/lib/apps/PackageAppFrame.svelte` | 1 to 1.5 days |
| P6 | package update and data migration, disable and purge proofs, the backend service, sandboxed frame for third-party packages | red, tier 2 | P5 | to be designed | several days |

**Parallel lanes.** P1, P2a and P2b start together today: P2b codes against
the manifest shape in §3.1 and merges after both. P3 lanes start the moment
P1 is on main, one lane per app; they conflict only on one-line deletions in
`registry.ts` and `media-utils.js`, so FP0 merges them in sequence. R runs
when P1 is deployed. P4 and P5 start after P2b and can run beside each other
(different files; both call the P2a seam).

**Collisions with the lanes in flight.** U1 (`internal/sourceproj`,
`agentcore/source_projection.go`, `capsule/release_binary.go`) and U1b
(`vmmanager` seed, base objects): no shared file. The file-object lane
(`internal/autoputer/files.go`, `content`): no shared file. P1 touches
`media-utils.js`, which a file-object frontend change could also touch [H];
P1's edit is two lines at the route table. The evidence and slots lane
(`agent_evidence`, `co_super_slots` to graph kinds) edits
`objectgraph/registry.go`: P2a writes a new file and leaves `registry.go`
alone.

**Staging proofs.**

- P1: CI and the deploy; on a fresh class-test computer, open a `.png` from
  Files and the package-built image window opens; restore a saved desktop
  with an image window; the Playwright `app-packages.spec.js` against
  `choir.news` passes; a deliberately bad manifest fails `npm run build`
  (local, recorded).
- P2a: unit tests; no staging behavior until P2b.
- P2b: with the switch on for one test computer, a test package's kind
  writes through the object route, appears on the tape (a
  `projection_batch_recorded` event), survives a reboot, and a restore to a
  head before the write drops it; a write to `choir.texture_revision`
  through the route is refused; the replay-completeness report stays
  eligible.
- P3: per app, open it from the launcher and from Files.
- R: the operation's patch touches only `frontend/src/packages/reader/**`
  and the two deletions; after apply, a `.pdf` and an `.epub` open in the
  Reader; the pre-apply checkpoint is taken.
- P4: a first-party package desk wakes on a cast, writes its kind, is
  refused another package's kind, and a crash boot closes its open
  obligations "interrupted by a restart".
- P5: the goal file's five acceptance rows.

**Rollback.** P1 and P3: revert; package windows fall back to whatever
static entry the revert restores. P2b: unset `CHOIR_APP_PACKAGE_OBJECTS`, or
revert; objects already written stay as inert rows that replay without a
lookup. P2a: revert (nothing calls it until P2b). P4: `CHOIR_APP_DESKS=off`.
P5: disable packages, then revert; lifecycle objects stay inert (branch D2).

**Honest size.** "App packages in an hour or two" is true of P1: a real
first-party app as a package, and the format self-development builds the
Reader in. Package-declared kinds (P2) add half a day of lanes in parallel,
and desks (P4) about a day. What the goal file calls done (P5 and P6: install
on a computer without rebuilding the monolith, migration and recovery,
disable and purge) is two to four days.

## 5. The other upgrades the loop needs

The loop is this: a coding agent asks a test computer for the Reader through
the choir CLI; engineering builds it in a capsule; the owner judges it on
that computer; an accepted change becomes every computer's base. U1, U1b and
app packages are covered elsewhere.

Evidence comes from the v1 to v7 runs on b@c ([L], with line numbers) and
from main. Sizes are one agent's working time. A to F stop the loop. G to L
make it costly or fragile. M to P are fixed.

| | Upgrade | Evidence | On main | Class, tier | Size |
|---|---|---|---|---|---|
| A | **No path from an accepted candidate to main or other computers** | `choir release publish/adopt` exists (`cmd/choir/release.go:53-109`) and publishes a `choir.software_release` (base commit plus patch, `internal/release/record.go`; `objectgraph/registry.go:183`). Adoption is one computer at a time and refuses on conflict. Nothing makes a commit on `origin/main`; FP0 lands patches by hand from `choir self-dev patch --out` [L 156, 181, 199]; ledger 19:3x | open | small form: a proof script that takes an operation id, checks its base is an `origin/main` ancestor and opens a PR branch for the ordinary landing loop (yellow, tier 0). Real form: host promote with a stock-base advance (red, tier 2; needs update-design Q7) | 2 to 4 h; real form about a week |
| B | **The owner cannot see a candidate before approving** | Freeze captures screenshots (`agentcore/tools_capsule_visual.go`; route in `agentcore/api_self_development_visual.go`). The host grants capture to one computer only (`nix/node-b.nix:1755`), and the CLI has no verb. The conductor judged by building patches locally [L 25, 182]. Owner 19:0x: propose needs screenshots and video | open | grant to `class:test` plus a `choir self-dev visual` verb: red (Node B env), tier 1, one-line rollback | 2 to 4 h; the Texture embed and Updates app 1 to 2 days |
| C | **Engineering's Reports do not reach the Texture document** ("none of engineering's 4 Reports reached it" [L 181]; v7 at 2 revisions [L 199]) | Engineering addresses management only (`deskmanifest/manifest.go:261-262`). A Report becomes a producer-report packet to management (`agentcore/rlm_reduce.go:1123-1170`). With no live management run, "the packet stays durable pending until an independent control opens Management" (`agentcore/management_controller.go:3233-3246`), a gate against wake storms. The one relay that did arrive [L 165] came from a management run already awake [H] | open, no record (the fix's first commit writes it) | a bounded management wake per assignment that relays progress to Texture, or a Texture observation minted from the Report: orange, tier 1 | 4 to 8 h; Texture as the addressee of progress 1 to 2 days |
| D | **The Texture document stops at the decision boundary** | No revision for reject, approve or apply [L 27, 61]. `texture revise` returns 409 once the trajectory settles (`textureowner/texture_agent_revision.go:96-100`) [L 34, 156]. Settling waits on terminal work items (`store/lifecycle.go:5198-5290`), which come before the decision. Record: `texture-supervision-doc-stops-at-the-decision-boundary-2026-10-24.md` | open | keep the work item open until the operation is terminal; deliver decided, applied, failed and restored to Texture: red (Texture canonical writes), tier 2 | 1 to 2 days |
| E | **A research run makes the computer checkpoint-ineligible** | D39 (`7a06ce8ab`, `642c49e3e`) fixed the media tables (`agentcore/replay_eligibility.go:71-73`), and the 409 names its tables (`:293-315`). `agent_evidence` is still empty-until-supported (`:89`), written by direct SQL (`store/texture.go:2263`) from research's `SaveEvidence` (`agentcore/tools_evidence.go:124`), so one saved row refuses the pre-apply checkpoint [L 184, 193]. `co_super_slots` (`:68`) has no live writer (`StartCoagentRun`, `runtime.go:1399`, no non-test caller [H]) | media part fixed; evidence open | **owned by the evidence and slots lane** | about 1 day |
| F | **A reject is refused after an `accept_once` was consumed** | `verifyDecisionModeReceipt` refuses a reject when `old_mode` is `accept_once` or `consumed_operation_id` is set (`agentcore/api_self_development.go:712-713`). After one approval, every reject needs a fresh mode set first [L 75]. Update design §4.6 asks for its own record; none exists yet | open | admit a reject (fail-safe) under a receipt consumed by a different operation; record first: red (self-development authority), tier 1 | 2 to 4 h |
| G | **The model policy is edited by hand** | Engineering's model changes only by `PUT /api/files` on `System/model-policy.toml` (`provideriface/config.go:88-93`), plus an `# owner-edited` comment line (`modelpolicy/model_policy.go:49,576-591`). The conductor swapped and restored it by hand three times [L 94, 186, 197, 200]. `/api/model-policy/` has only `resolve` and `verify` (`agentcore/api.go:298-300`). `I27` target: data-homes violation 1, step 4 | open | a `choir.model_policy` kind (writer `owner`), an owner-act route, `choir model-policy set --desk engineering`; TOML becomes a rendered view: orange (provider routing), tier 1 | 1 day |
| H | **Svelte cannot be written through a Go string** | `WriteFile(path, content string)` (`yaegikernel/choir.go:457`) takes a Go literal inside Go cell source. A backtick ends a raw string, so the model packed Svelte into one escaped, minified line [L 31-33]. The Reader is Svelte | open, no record | `WriteFileFrom(path, ref)` from a staged blob, or `ApplyPatch`: orange, tier 1. The brief's `@@BT@@` placeholder holds meanwhile | 3 to 6 h |
| I | **Work is persisted only at Complete** | A host blip, a refused `partial` Complete after Freeze, or a cancel loses the round [L 126, 169, 172]. A blocked stage reads as failure, and the carry dies with the trajectory [L 134-156] (`agentcore/engineering_assignment_fate.go:1494-1507`, `agentcore/carried_patch.go:63,129-157`). Record: `engineering-work-product-durability-cluster-2026-10-24.md` | open | draft candidates as tape objects: red, tier 2. **Does not block a Reader-sized round** | 3 to 5 days |
| J | **Zombie runs** | When the fate write fails, `handleExecutionError` logs and leaves the run `running` (`agentcore/runtime.go:4604-4620`). Only `choir run cancel` clears it [L 105, 124], and the zombie holds off refresh as busy [L 121]. Record: `engineering-run-death-leaves-bound-assignment-2026-09-29.md` | open | red, tier 2 | 1 to 2 days |
| K | **The idle sweep hibernates a CLI-driven computer** | Idle is measured from `LastActiveAt` (`vmctl/warmness_policy.go:213-220`), and the API-key path (`proxy/api_key_computer_authority.go`) never refreshes it. A key-driven test computer hibernates 30 minutes after it was minted, even mid-apply. Record: `idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md` | open | refresh on an authenticated key request and on a planned restart: red (vmctl), tier 1 | 4 to 8 h |
| L | **Engineering cannot read the owner's material** | The cast is capped at 16 KiB, and `ProductAPI` belongs to management only (`deskmanifest/manifest.go:210`), so the v6 patch could not be handed over [L 196]. A read-only `/computer/files` with package mirrors was agreed (ledger 19:0x, ICRL decision 2); it is designed on `lane/fp0-icrl-design` only | open | red (mounts), tier 2. The Reader needs only a brief | 3 to 5 days |
| M | Busy-polling its own job | D34 `acd0ac5ed` (`agentcore/capsule_jobs.go:302`); the record's header still says "not deployed", which is stale | fixed | — | — |
| N | Cast cut at 4 KiB | D40 `d6de762a9`, `1ced2d5f0`; the 16 KiB cap is at `agentcore/engineering_cast_request.go:35` | fixed | — | — |
| O | Owner acts by key | D41 `705d016ca` (`frontend/src/lib/SettingsApp.svelte:78`). A key still cannot mint `owner:acts` (`auth/handlers.go:1779-1783`), so the owner mints the agent's key once, signed in | fixed (one credential step remains) | — | — |
| P | Platform Dolt GC killed long runs | D36 (`platform-dolt-gc-throttled-at-memory-high-2026-10-24.md`) | fixed | — | — |

Two further things are not platform upgrades but are recorded so they are
not lost:

- Candidates arrive with stray files and broken wiring [L 25, 155, 170, 199].
  P1's import check catches this for packages. A Verify-time check that a
  candidate touches only the paths its cast names would catch more [H].
- Long rounds grow to 230k to 340k input tokens per call [L 103, 117, 180].

**The shortest working loop for the Reader:**

1. P1 lands.
2. A fresh test computer is minted after the deploy, with an owner-minted
   key (O) and the capture grant (B).
3. The brief carries the backtick rule (H).
4. The owner judges from the screenshots and the patch.
5. Item A's PR script lands the patch on main.

C, F and K make a second attempt survive, and D and E make the loop
trustworthy. Beyond P1 this is about a day of parallel lanes: A (small form),
B, C, F, H and K.

## 6. Open decisions, with defaults

1. **Where in-tree packages live.** `frontend/src/packages/<name>/` (no
   flake change) or top-level `packages/<name>/` (a flake fileset change and
   a nix eval of both systems; the natural home once a package carries a desk
   and a backend). Default: **`frontend/src/packages/` now**; move with P4,
   when a package first has non-frontend parts.
2. **The in-tree frontend ABI.** First-party packages are Svelte components
   built with the shell (`svelte-inline/v1`), and installed packages are
   framework-free modules (`mount(target, ctx)`, branch D5). Default: **both,
   by `origin`**. An in-tree package shares the shell's Svelte runtime, which
   is safe because it is built with it.
3. **Kind names.** `choir.app.<package>.<kind>` for every new package kind
   (`appdesk` and `app-package-carries-app-desk` §3), or the bare names the
   data-homes inventory uses as targets (`choir.slides_deck`,
   `choir.calendar_event`, `choir.mail_message`). Default: **the namespaced
   form**; the data-homes targets are renamed when their migrations are
   written. A kind other apps share is still owned by one package (Mail owns
   `choir.app.mail.message`; another mail app reads it, `I27`).
4. **Who writes a package kind before its desk exists.** The owner's window
   through the generic route (writer `owner`, admitted by `OwnerEdits`), or
   `system:runtime` acting for the desk. Default: **the owner**. A
   `system:runtime` writer passes the gate for any caller (`writer.go:206`),
   which would make the package's one writer unenforceable.
5. **Schema language.** A JSON Schema subset in the manifest, or Go types in
   the binary. Default: **the subset**, since installed packages bring no Go.
6. **Retire on purge.** Purge makes a package's kinds read-only and keeps
   them, or deletes per `on_purge`. Default: **retire and keep** for v1
   (`appdesk` semantics); deletion waits for crypto-shredding.
7. **Converting the remaining apps.** Order and scope as in §3.2: after the
   Reader, in parallel, with Texture and Settings static. Default: **as
   written**, reported to the owner as a reordering of the 10-14 ruling.

## 7. Contradictions with the goal file and doctrine

- **Goal file, "goodharting_would_be: a registry entry dynamically added only
  at build time".** P1 is exactly a build-time registry entry. It does not
  claim the goal's acceptance, which stays with P5 and P6. P1 is the package
  *format and boundary* self-development writes to, not runtime install. The
  goal file's `now` card should say so.
- **Goal file, homotopy axis "every existing first-party app converted to a
  package first".** §3.2 converts one app first, then the Reader, then the
  rest the same day.
- **Goal file, `start.source` and `now`.** They are anchored at
  `main@8aa1dce9` and say "Pending S6"; S6 is not a dependency of P1 to P4.
  The goal file needs a refresh at P1's landing (Goal-Station Handoff).
- **Goal file, mechanism "package bytes remain inside the S6 full-release
  closure".** Owner ruling D1 (presence is a tape fact) superseded it for
  installed packages. In-tree first-party packages are inside the release
  closure, as part of the platform frontend, so both hold, by `origin`.
- **Goal file, "each package's app-data migration should target the shared
  object-graph kinds ... (one generic app-document kind, `choir.party`,
  `choir.calendar_event`)".** `I27`'s "kinds belong to their package"
  replaces a shared generic app-document kind. Data shared across apps stays
  on its owner's kind (open decision 3).
- **The branch's §5.4 and F19 to F22** (`package_kv`) contradict `I27` and
  `H038`, as recorded.
- **`app-package-carries-app-desk` §6, "the overlay is ... derived from the
  fold".** For first-party packages, P2 derives kinds from the build-time
  manifests, not from a tape fold. That is consistent with its §7.3 (a
  first-party desk ships in the image and is not in the install fold).
- No contradiction found with `I26`.

## Heresy delta

- discovered: none new. The S7-0 `package_kv` silo was already named (`H038`).
- introduced: none (no code).
- repaired: none; the replan routes `H038`'s package half to P2.
