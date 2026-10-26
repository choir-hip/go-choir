# The Choir Library: how an accepted package change reaches other computers (2026-10-26)

Status: **design, green** (docs only, no code). It is an architectural
design that a build will follow, so it is reviewed like the change it
designs: **tier 2** (Devin and DeepSeek). Station SR9 of
[the readiness metamission](../definitions/choir-selfdev-readiness-metamission-2026-10-24.md).
Goal file: [choir-sr9-library-2026-10-26.md](../definitions/choir-sr9-library-2026-10-26.md).
Written on lane `lane/sr9-design` from `origin/main` at `355c603ad`. It
builds on the read-only investigation of 2026-10-25 (the SR9 propagation
report, which mapped the existing release system with file:line refs) and
replaces that report's recommendation where the owner's rulings of
2026-10-26 differ from it.

Evidence labels: **[V]** read at file:line on this tree; **[D]** stated in a
cited design or ledger entry; **[H]** hypothesis, with what would settle it.

## 0. The name, in one place

| | |
|---|---|
| Product name | **the Choir Library** (short form: **the Library**) |
| Owner ruling | 2026-10-26: "to a mainstream audience, we can say that this is the choir library". Earlier working names (the Commons, Exchange, Shelf) are retired. |
| Where the name appears | this table, the Updates app and catalog UI copy (one frontend constant, `LIBRARY_NAME`, S8), and prose in docs. |
| Where it does not appear | object kinds, routes, CLI verbs, settings, Go packages. They use functional names (`package`, `catalog`, `subscription`, `adoption`), so a rename is one constant and a docs edit. |

## 1. The answer in one page

1. **The unit is a package release: one package's own change.** It is a
   content-addressed record of one package directory
   (`frontend/src/packages/<name>/`) as a whole git tree, together with:
   the tree it replaced (its parent); the review diff from parent to
   tree; the manifest; the platform base it was built and verified on;
   the publisher's receipts, settled precommitments and verifier refs;
   and provenance (publisher computer, owner, operation). It carries only
   the files under that one directory. So it can never carry the
   publisher's other unupstreamed changes. The owner's "it should only
   carry its own change" holds by construction, not by policy.
   (Section 3.)
2. **A change outside a package never rides the Library.** Shell, Go,
   desks, nix, store schema, generated kinds and app.css are platform
   changes, and they reach other computers only through the reviewed main
   path. A candidate that mixes the two is refused at publish, and the
   refusal names the outside paths. The T1a Reader redesign touched
   `app.css`, `read.html`, `reader.js` and `theme.ts` beside its package.
   The fix is to steer engineering toward package boundaries (an SDK seam
   for theming), not to widen the unit. (Sections 3.3 and 9.)
3. **Private by default; publishing is the owner's act.** A release is
   visible only to its owner and the owner's own computers until the
   owner widens it, to named owners or computers, or to everyone. A
   publish instruction inside the owner's own request ("build this and
   publish it to my computers") is the owner's act. It is recorded in that
   request's signed owner-act receipt, and no desk can widen the scope.
   Today's unscoped release list and reads are fixed first. (Section 4.)
4. **Discovery is a catalog view over listed releases.** It shows
   packages, tracks (a publisher's line of a package), per-package version
   history and a text search, always filtered to what the caller may see.
   (Section 5.)
5. **Adoption is per computer, by subscription, deterministic.** A
   subscription names a package, a track and a mode (`auto`, `propose`,
   `pinned`), with a default for each computer class. It is a standing
   intent. A tick offers new releases. A deterministic adoption job, which
   makes no model call, does the rest: a three-way tree check (the
   adopter's current package tree against the release's parent and the
   release), a fixed capsule recipe (place the tree, build, run the
   package checks and the declared test, capture the adopter's own
   screenshots, Freeze), and then the adopter's own gate decides. A
   conflict, and only a conflict, becomes engineering work. This replaces
   P8's desk prompt. (Section 6.)
6. **Install now means a build on the adopter; later it means data.**
   First-party packages are compiled with the shell (`svelte-inline/v1`),
   so adopting one today rebuilds the adopter's SPA in its capsule. The
   Go binary is reused, and no host build is needed. Runtime install (P5)
   adds a host-built, framework-free module per release
   (`choir.app-frontend/module/v1`), loaded from a verified build store.
   Install then becomes a tape fact with no rebuild and no restart, and
   base moves stop touching packages. (Section 7.)
7. **Every apply is on probation, with one forward revert.** A failed
   probation (health, a crash restart, or the package smoke: the window
   opens, renders non-blank and logs no console error) swaps back to the
   previous package tree as a new forward operation, once, never in a
   loop. Two failed probations on different computers *suspend* a release
   automatically. The publisher can *withdraw* it, and the platform's
   security authority can *yank* it. A yank is the only forced change: it
   sets the package aside to its previous or base version, and it never
   contains or stops a computer. (Section 8.)
8. **Main stays the stable ring, reached without a host GitHub push.** A
   coding agent or the owner runs `choir package export`, a client-side
   verb that generalizes `scripts/selfdev-promote`. It verifies a
   first-party package release and writes its tree verbatim onto a lane
   branch on `origin/main`, with a containment prediction and the
   evidence. The agent pushes with its own credential, and the reviewed
   landing loop runs. When the deploy boots, every carrier's stack retires
   as `upstreamed`. No clean rebase applies without approval (owner, "not
   yet"). (Section 10.)
9. **The Updates app is a read-only projection.** It shows every update
   applied to this computer (platform deploys, package adoptions, reverts,
   set-asides), with evidence, plus the notices the owner ruled on. Desks
   can read it. Approvals stay where owner acts already live. (Section 11.)
10. **Protected computers adopt last, never automatically at first.** By
    class: the test computers `auto` (once the standing gate is proved);
    primary `propose`; the owner's computers (`premium_always_on`,
    `critical_protected`) `propose` with evidence; the paper
    (`public_platform`) `pinned`. These are enforced in code, not only as
    defaults. (Section 12.)

The build is fifteen steps, S0 to S14 (section 17). Each lands alone,
behind a default-off switch, with a deployed proof on two test computers.
S1 (owner-scoped reads) is a security fix to what is live today. S2 to S5
give the first end-to-end loop: publish, sharing, export, and
deterministic propose-mode adoption. S6 and S7 add package identity and
containment, then probation with one revert. S8 is the Updates app, and S9
is auto mode with rings. S10 and S11 add the support window, signed
listings and security yanks. S12 to S14 add runtime install, cross-owner
adoption in a sandboxed frame, and the widening to protected computers
with the retired code deleted.

## 2. What exists, and what this design keeps, redesigns or deletes

The 2026-10-25 report holds the full map. In short [V]:

- **`choir.software_release`** (`internal/release/record.go`): one applied
  operation's source patch against a platform base commit. It is
  published by an owner act through the proxy (`internal/proxy/releases.go`)
  to corpusd, host only (`internal/platform/releases.go:76-81`), and its
  kind is registered at `internal/objectgraph/registry.go:196`.
  `Declarations.Packages` is reserved and must be empty (`record.go:117`,
  `:203`).
- **Under U1 the patch is cumulative** over the computer's whole stack
  (`internal/sourceproj/compose.go`, `Cumulative`). A release from a
  computer with two unpromoted changes therefore ships both.
- **P8 adoption** (`internal/agentcore/release_adoption.go:116-219`)
  verifies seven rungs, then hands the work to the desks as a prompt
  (`adoptionRequestText`, `:336`; submitted at `:199`). adopt-1418 failed
  in exactly that step.
- **Reads are unscoped.** corpusd: "Any bound guest may read any published
  release" (`releases.go:71`). The list filters only by an optional
  publisher computer (`:290`). The proxy reads single releases "as the
  host" (`proxy/releases.go:204`), and any caller with the read scope
  lists every owner's releases (`:192`).
- **Packages** exist at build time: `frontend/src/packages/{image,reader}`,
  `choir-package.json` v1, `check-packages.mjs` and
  `package-boundary-plugin.mjs` (the resolved-module boundary), and
  `frontend/src/lib/app-sdk.ts` (the platform API a package may import).
- **Main is reached** by `scripts/selfdev-promote` plus FP0's push and the
  landing loop. Containment then retires the computer's stack as
  `upstreamed` (`internal/sourceproj/sourceproj.go`, states at `:76-103`).

| Existing piece | Disposition |
|---|---|
| `release.Record` verification code (`VerifyID`, `VerifyPatch`, `PatchedTree`, the secret scan) | **Keep**: moves into the new `internal/pkgrelease` package (the package release record), reused as is. |
| `choir.software_release` as a cross-computer unit | **Retire** from propagation (owner 2026-10-14: core changes are never adopted directly between diverged computers). Existing objects stay readable and inert. Publish is refused once S5 lands; the route and verb are deleted in S14. |
| `Declarations.Packages` reservation | **Superseded** by the new kind; the field is never filled. |
| P8 adoption (`POST /api/computers/<id>/self-dev/adoptions`, `choir release adopt`, `adoptionRequestText`, `choir.release_adoption`) | **Delete** the desk-prompt apply (S5). The kind is retired (readable, no writer). The seven-rung ladder is **kept** as the first stage of the deterministic job. |
| Unscoped list and reads | **Fix** in S1, before anything new is built on them. |
| `scripts/selfdev-promote` | **Keep** for platform-class operations (the reviewed main path). For package releases it is generalized by `choir package export` (S4), which reuses its containment prediction. |
| P4 standing consensus (`internal/platform/standing_consensus.go`) | **Reuse** as the auto-mode gate on test computers (S9). Its refusal of non-test computers (`:42`) stays and becomes the class rule's floor. |
| Host builder (`internal/hostbuild`) | **Reuse** for the P5 module build (S12) and for any package that later carries Go (P4 desks, P6 backends). Package releases never touch its refused paths. |
| M9a platform-follow push (`internal/agentcore/platform_update.go`, `internal/selfdevprotocol/platform_update.go`), dark since a47122de0 | **Deletion candidate** in S14, after a citer search (standing question: deletion citers). It is not on this path. |
| `internal/platformrelease` lineage fields with no callers | **Reuse** `CompatibilityRange` for the SDK range (S12); delete the rest in S14 after a citer search. |
| `source.rebase_offer` (U4, designed, unbuilt) | **Unchanged.** U4 is the platform stack's rebase. Packages avoid it: as a snapshot before P5 (S6), and as data after P5 (S12). |

## 3. The unit: a package release

### 3.1 The record

Kind `choir.package_release`, Store B, writer corpusd, immutable,
insert-if-absent (as `software_release` is today). The release id is the
sha256 of the record's canonical JSON (RFC 8785, the
`computerevent.CanonicalJSON` used today). Schema
`choir.package_release.v1`:

```text
PackageRelease {
  schema:     "choir.package_release.v1"
  package:    { name, manifest_version }           // name = directory name; manifest_version = the manifest's semver, declared
  track:      <publisher owner id>                 // a publisher's line of this package (3.4)
  parent:     { kind: "base",    commit, tree }    // the package tree in a platform base commit
            | { kind: "release", release_id, tree }// or a previously listed release on the same track
  source: {
    root:          "frontend/src/packages/<name>"
    tree:          <git tree id of the root subtree>
    tree_patch:    { sha256, bytes }   // binary diff empty-tree -> tree, rooted at root: the package itself
    review_patch:  { sha256, bytes }   // binary diff parent.tree -> tree: what a reviewer reads
    changed_paths: [...]                // every path under root (3.2 refuses any other)
    built_on:      <platform base commit the publisher built and verified it on>
  }
  manifest:   { sha256, abi, requires_sdk?, kinds_count, opens_count }   // requires_sdk optional before S12, required after
  build?:     { module_digest, builder_receipt_digest }                  // S12 only: the host-built runtime module
  publisher:  { owner_id, computer_id }
  operation:  { operation_id, bundle_digest, decision_event, materialization_receipt,
                checkpoint_ref, verifier_refs[] }     // the publisher's receipts, as release.Operation today
  precommitments: [ { record_id, kind, question, p_yes_permille, settled, resolve_record_id } ]
  declarations: { test_command? }                     // one line, as today
}
```

What the record deliberately does **not** carry:

- **Screenshots and other evidence assets.** The publisher's captures
  render the publisher's own data, and TE's rule is "evidence is never
  published" [D, metamission station TE]. An owner may attach named
  assets at publish (section 4.4). Otherwise the adopter's job captures
  its own (section 6.4).
- **A cumulative patch.**
- **Executable bytes.** The S12 module is named by a host build digest,
  never carried, as today's `HostBuild` is.

The two patch blobs live in the host content store under their sha256,
as release patches do now (`releases.go` `handleReleaseBlob`, the
`contentNamespaceRelease` namespace). The cap is raised from 2 MiB to
8 MiB for this namespace only [H: package sizes are unmeasured. The
Reader with its sample PDF module is the largest today; S2 measures it].

### 3.2 Deriving the release from an applied operation (only its own change)

`choir package publish --computer C --operation OP --package NAME`
drafts the record on C (guest), as `releaseDraft` does today
(`internal/agentcore/api_release.go:45-120`), with these rules:

1. **Applied only.** OP must be `applied` (as today, `:52`).
2. **Confinement, re-derived on the host.** The operation's paths are the
   paths changed between the effective source just before OP's applied
   event (sourceproj's ref for the previous applied event, or the base)
   and the effective source just after it. Every one of them must lie
   under `frontend/src/packages/NAME/`. The guest computes this and corpusd
   recomputes it from the two blobs it is given (3.1), so a guest's claim
   is never trusted. Any other path refuses with
   `package_change_not_confined`, naming the outside paths. Those are
   platform changes (3.3).
3. **Own change only.** The parent tree P is the package directory as it
   was before OP. P must equal either the package tree in the stack's
   platform base, or the tree of a release listed on the publisher's own
   track. Otherwise OP sits on an earlier, unpublished change to the same
   package, and the draft refuses with `package_parent_unpublished`,
   naming that operation ("publish operation X first"). The publisher's
   changes to other paths never matter, because the release carries only
   NAME's directory.
4. **The tree is the release.** The tree T is the package directory
   after OP. `tree_patch` is `git diff-tree -p --binary <empty> T`,
   re-rooted at `root`, and `review_patch` is the diff from P to T. corpusd
   applies `tree_patch` onto an empty tree and requires the resulting
   tree id to equal `source.tree` (the `PatchedTree` technique,
   `internal/release/git.go`).
5. **Package rules at publish.** The manifest is in T and validates
   (`internal/apppackage` Go validation; `check-packages.mjs` semantics).
   T has no symlinks, no submodules, no file modes other than 100644 and
   100755, and no `.gitattributes`. The secret scan
   (`sourcePatchAddedSecrets`) runs over `tree_patch`. Before S12, a
   package whose manifest declares `kinds` is refused with
   `package_kinds_need_platform`: first-party kinds are generated into
   `internal/apppackage/firstparty_gen.go`, outside the package, so such
   a change is a platform change until install-time kind registration
   (S12) exists.

### 3.3 Platform changes never ride the Library

A change with any path outside one package directory is a platform
change. It goes through the reviewed main path only: `scripts/selfdev-promote`
(or its successor), review by tier, and the landing loop. The Library
refuses it at publish, and an adopter's job refuses it again (6.4).
There is no "class: platform" listing, and no admission of a platform
change to any subscriber, because the owner ruled that core changes are
never adopted directly between diverged computers [D,
`docs/reports/selfdev-update-architecture-2026-10-15.md:46-71`].

Mixed candidates are the common case today. T1a's Reader redesign scope
was the package plus `frontend/src/app.css`, `frontend/src/reader.js`,
`frontend/reader-chrome.css`, `frontend/read.html` and
`frontend/src/lib/theme.ts` [V, the T1a goal file's `now.candidate`].
There are two answers, both outside the Library's mechanism:

- **Steer requests to boundaries.** A package request names its package
  root as the cast's confinement. Verify checks the touched paths against
  it (the "Verify-time path check" the app-packages replan §5 recorded as
  [H]), and the brief says that shared styling goes through an SDK seam.
- **Grow the SDK.** Theming and shell hooks that packages reach for
  become `app-sdk.ts` exports, each a small platform change landed
  through main once, so that later package changes stay confined.

### 3.4 Tracks, sequence numbers and history

- **A track is one publisher's line of a package.** Its id is the
  publisher's owner id. First-party packages also have the track
  `choir`, which is main itself: its "releases" are the package trees in
  deployed base commits, and nobody publishes to it through the Library.
- **Linear history per track.** A release's parent must be the track head
  at publish time: the latest listed release on that track that is not
  withdrawn, or the base tree for a first release. A stale parent refuses
  with `track_moved`. The publisher's engineering then redoes the change
  on the head, as ordinary self-development.
- **The sequence number belongs to the listing, not the record.**
  corpusd assigns `seq` (1, 2, ...) per (package, track) when it lists a
  release. The record stays immutable. The manifest's semver is the
  publisher's own claim and is shown, but never compared.
- **A variant of a first-party package keeps its name.** b@c's improved
  Reader is package `reader` on track `<b@c's owner id>`. Adopting it
  replaces the adopter's `reader` directory, so window ids, aliases and
  file routes are unchanged. A *new* app is a new package name in the
  publisher's namespace (owner 2026-10-24: "they can just make the Gmail
  app"). The check refuses a name or alias that collides with a static
  app or with another installed package (P1-F3).

## 4. Privacy and publication

### 4.1 Scopes, and who may change them

| Scope | Who can see the listing, the record and the blobs | Who can adopt |
|---|---|---|
| `owner` (the default) | the publisher's owner and that owner's computers | the owner's computers |
| `shared` | plus named owner ids and named computer ids | the named computers, and the named owners' computers, **once S13 lands** (the sandboxed frame) |
| `public` | every authenticated owner on this cloud | everyone's computers, once S13 lands |

- **Owner acts.** Every scope change is an owner act through the proxy,
  with an owner-act receipt (`internal/owneract`) whose subject binds
  `{release_id, scope, shared_with}`. corpusd verifies the receipt before
  it writes the listing. No desk and no guest credential can widen a
  scope.
- **A publish intent carried by the request.** The owner's self-development
  request may carry `publish: {scope: owner}`, for example a CLI flag on
  the request, or a Texture request the owner typed. That intent is part
  of the request's owner-act subject, so it is the owner's act.
  - The runtime discharges it once: when the resulting operation applies
    and passes probation, the runtime publishes with that scope and cites
    the request's receipt.
  - A desk can never add or change the intent. The runtime reads it from
    the receipt, not from a desk's message.
  - The default: a request-carried intent may name only scope `owner`,
    and `shared` or `public` needs a direct owner act on the release
    (owner question Q2).
- **FP0 and coding agents** may perform a publish act only with the
  owner's key, under an explicit owner instruction recorded in the act's
  reason, the way FP0 approves on b@c today.

### 4.2 The listing

Kind `choir.package_listing`, Store B, writer corpusd, one object per
release id, versioned (each change is a new version that names its
predecessor, so history is kept):

```text
PackageListing {
  release_id, package, track, seq
  scope: owner | shared | public
  shared_with: { owners: [...], computers: [...] }
  state: listed | withdrawn | suspended | yanked
  state_reason, state_actor: owner_act:<receipt> | host:probation | host:security:<advisory>
  version: <n>, prev_version_digest
  signature: <platform-control signature over the canonical listing>   // S11; unsigned before
}
```

The record never changes. Visibility and state live only here, so
changing them never changes a release id.

### 4.3 Owner-scoped reads (the S1 fix)

Every read route filters by the caller's owner identity:

- the proxy list `/api/releases` and the new `/api/catalog/...`;
- the single read;
- the patch and tree blobs;
- corpusd's guest read `GET /internal/platform/releases/<id>?computer_id=`,
  which resolves the computer's owner.

A release the caller may not see answers **404**, never 403, so its
existence is not disclosed. The proxy stops reading single releases "as
the host" (`proxy/releases.go:204`) and passes the authenticated owner
instead. For `software_release` (live today) the rule is publisher owner
equals caller owner. For `package_release` it is the listing's scope.

### 4.4 Evidence and private data

- **Screenshots.** At `owner` scope, the publisher's own screenshots may
  be referenced, because only the same owner sees them. At `shared` and
  `public` the record carries none by default. The publisher may attach
  named capture assets in the widening act, which lists them so the owner
  sees exactly what leaves.
- **Source.** Source can carry private data too, for example a fixture
  built from the owner's document. A widening act to `shared` or `public`
  therefore shows the full `changed_paths` list and the review diff size,
  and the CLI asks for `--confirm-paths`. The secret scan runs again at
  widening.
- **Adopter outcomes.** These are visible to the adopter's owner, and to
  the publisher only as counts (n adopted, n reverted) with no adopter
  identity across owners (owner question Q6 of the update design; default
  here).

## 5. Discovery: the catalog

The catalog is a read-only view computed from the listings the caller can
see. It stores nothing of its own.

- `GET /api/catalog/packages?q=&track=&limit=`: one row per (package,
  track), showing the head release (seq, manifest version, title, icon,
  description from the manifest), the number of releases, the last
  publish time and the listing state.
- `GET /api/catalog/packages/<name>/releases?track=`: the version history
  of one track, newest first. Each row has seq, release id, parent, review
  diff size, state and the publisher's settled precommitments.
- `GET /api/catalog/releases/<id>`: the record, verified against its id
  (as `readVerifiedRelease` does in the CLI today), with its listing.
- **Search** is a substring match over package name, manifest title and
  description, and track display name, using Store B metadata. That is
  enough for one cloud. A ranked search waits for real volume.
- **Base tracks.** For each first-party package the catalog also shows
  the `choir` track, read from the deployed base's manifests, so an owner
  can see that their variant is behind the base (6.3).

## 6. Adoption

### 6.1 Subscriptions

Kind `choir.package_subscription`, on the adopter's tape (Dolt, the
computer's own graph). Writer `owner`: it is written only by an owner act
through the proxy, as a computer setting is. Computer settings are
booleans [V, `internal/store/computer_setting.go:204`], so a subscription
needs its own kind.

```text
PackageSubscription { package: <name> | "*", track: <owner id> | "own", mode: auto | propose | pinned,
                      pin_release_id?, admit_owners: [<owner id>] (default: [own owner]) }
```

- **A standing intent** (CLAUDE.md, the Autopaper schedule rule). A crash
  closes the tick or adoption in flight, never the subscription. A
  release whose adoption crashes this computer twice is closed for this
  computer, with that cause.
- **Defaults by class**, applied when the computer has no subscription
  of its own; the switches still have to be on:

  | Class | Default | Can the owner raise it? |
  |---|---|---|
  | `test` | `propose` until S9, then `auto` | yes |
  | `primary` | `propose` | `auto` from SR8 stage 4 |
  | `premium_always_on`, `critical_protected` (the owner's computers) | `propose`, with visual evidence | no `auto` until the owner rules |
  | `public_platform` (the paper) | `pinned` | manual adoption only, last |

  The "can raise" column is enforced in code: a subscription write with a
  mode its class does not admit is refused. A default alone is not
  enough.
- **`pinned`** holds the package at its current tree. Only a security
  yank moves it (8.4).

### 6.2 The tick and the offer

The guest runtime ticks every 5 minutes, and once at boot (after the
crash-rule closures; a tick is new work).

1. It lists, through its own capability, the listings newer than its last
   seen cursor that match a subscription and that it may adopt.
2. It coalesces to the head release per (package, track), because
   intermediate releases are never applied one by one.
3. It opens at most one adoption per package at a time. A newer head that
   arrives while an adoption is open marks the open one `superseded` if
   it has not reached `applying`.
4. `pinned` skips. `propose` and `auto` both open an adoption.

### 6.3 The three-way tree check

Let L be the adopter's current package tree (from its effective source),
P the release's parent tree, and R the release tree.

| Case | Meaning | Action |
|---|---|---|
| L = R | already current | record `already_current`, no operation |
| L = P | fast-forward | build (6.4) |
| L equals an ancestor release's tree on the same track | behind by several releases | build: fast-forward to R. The intermediate releases are named in the offer. |
| otherwise | the adopter has its own changes to this package, or follows another track | `conflict`: an engineering work item (6.6). Never overwrite. |

This check is on trees only. The adopter's base may differ from
`built_on`, and the build in 6.4 is the compatibility test before S12.

### 6.4 The deterministic adoption job

The job replaces `adoptionRequestText` and makes no model call. It runs
as a runtime-owned assignment of a new kind, `package_adopt`, on the
tape, so the fate machinery and the crash rule apply. Management does not
cast it and engineering's model does not run it. The cell is a fixed
recipe [H: whether a capsule can be opened and frozen under a
runtime-owned assignment without an engineering run. U4's `rebase` kind
is the precedent for a new assignment kind. Settled by reading the
capsule executor's open and Freeze entry points in S5's first commit].

1. **Verify.** The existing seven rungs (id equals digest, listed and
   visible, the published copy is byte-equal, both blobs hash, secret
   scan), plus: confinement re-derived (every path under the root), the
   listing state is `listed`, and the computer's class admits the mode.
2. **Place.** Replace the root directory in the capsule's effective
   source with R: delete what is there, apply `tree_patch`, and check that
   the tree id equals R.
3. **Build and check.** Run `npm run build`, which runs
   `check-packages.mjs` and the boundary plugin against the adopter's own
   static registry and other packages. Then run the declared
   `test_command`, if any.
4. **Capture.** Capture the adopter's own screenshots of the package
   window: the existing capsule visual evidence path, at the design bar's
   two widths.
5. **Freeze.** The ordinary bundle. Its manifest names the adoption id
   and release id as lineage. The bundle is content-addressed, so this
   needs no schema change (U4's argument).
6. **Gate** (6.5).

The adopter's operation patch is the diff of the root from L to R
against its stack, which is cumulative on the adopter, exactly as any
candidate is under U1. The Go binary is reused: a frontend-only stack
reuses the running base binary (U1 step 3).

### 6.5 The gate (the adopter's own)

- **`propose`.** The frozen candidate waits for approval on the ordinary
  path. The owner, or FP0 with the owner's standing authority, approves
  through `choir self-dev approve` or Texture, as today. The Updates app
  shows the proposal read-only, with a link.
- **`auto`** (S9, test computers first). Standing consensus decides.
  Today it has two seats: the host-build verifier (which binds the bytes)
  and management's Verify (a model). S9 replaces the second seat for
  package adoptions with a deterministic policy, `package-adopt-v1`:
  - the release is listed and visible;
  - the three-way check is a fast-forward;
  - the build and checks pass;
  - the adopter's captures are non-blank;
  - and, for a ring above test, the release passed probation on k
    computers of the ring below (8.2).
- **Auto-apply is never auto-promote.** Nothing in the Library writes to
  main (section 10).

### 6.6 Conflicts go to engineering

A `conflict` opens an engineering work item on the adopter: "your
package `reader` differs from release R's parent; merge your change onto
R". It uses the carried-patch machinery that U4 reuses:

- the review patch P→R is offered at `/selfdev/carried/`;
- the adopter's L is the starting tree;
- the result is an ordinary candidate through the adopter's gate, always
  `propose`;
- its fate is recorded on the adoption.

Engineering never carries a clean adoption.

## 7. Install and update: build time now, runtime later

| | Before S12 (`svelte-inline/v1`, today's format) | After S12 (`module/v1`, P5) |
|---|---|---|
| What adoption changes | the adopter's source tree and its SPA build (the computer's own frontend layer) | the computer's installed package set (a tape fact) |
| Who builds | the adopter's capsule (`npm run build`); the Go binary is reused | the host, once per release (`hostbuild`, a package-only flake output) [H: build time; settled by S12's first measurement] |
| Restart | the ordinary apply (frontend swap) | none: the shell loads the verified module on the next window open |
| Base move | the adopted tree is a stack entry (S6 handles it) | untouched, apart from the SDK range check |
| Kinds | refused (3.2.5) | registered at install from the install fold (app-packages replan §2.2 item 3) |
| Same-origin | yes (it is the owner's own approved code) | yes for `owner` scope; cross-owner packages run only in the sandboxed frame (S13) |

**The frontend rebuild question.** Before S12, every adoption is a full
SPA build on the adopter. That build is the compatibility check: the
boundary plugin and the build itself catch an SDK export the adopter's
base lacks. It costs one capsule build per adoption, which is minutes,
not seconds [H: measured on T-B in S5]. S12 removes the build:

- the module ABI is a framework-free `mount(target, ctx)`, as the S7-0
  branch had;
- Svelte is bundled into the module;
- `app-sdk.ts` is passed in `ctx.sdk` rather than imported, through a
  build alias;
- the manifest gains a required `requires_sdk`, and `app-sdk.ts` exports
  `SDK_VERSION`. The shell refuses to mount a module outside its range
  and falls back to the base tree's package if there is one.

First-party packages keep `svelte-inline` in main. The Library ships
`module/v1` builds of the same source for adopters.

## 8. Probation, health signals, revert, suspend, withdraw, yank

### 8.1 Probation

Every applied adoption enters probation. The default window is 10
minutes, recorded as a rule-settled precommitment (update design §5.1):
`update_healthy_within: 10m` and `no_restore_within: 7d`.

| Signal | Source | Fails when |
|---|---|---|
| health | `/health` 200, and the served build equals the target (the existing effective-release identity) | either is false at the end of the window |
| crash | the boot kind (planned or markerless) | a markerless boot inside the window |
| package smoke | a host visual capture (`internal/vmmanager` visual capture) of the package window on the adopter, after the apply | blank, a console error, or the window does not open |
| owner signal | the owner restores or reverts inside 7 days | settles `no_restore_within` false. It never triggers an automatic action. |

The host runs the smoke and records it, so a crashed guest cannot report
a pass for itself.

### 8.2 Health signals across computers

corpusd records one `choir.package_outcome` per (release, computer):
`healthy`, `reverted` or `interrupted`. The host writes it from the
probation result. It is visible as section 4.4 says. It feeds two things:

- **Ring soak.** `auto` at ring n requires k healthy outcomes at ring
  n-1. The defaults: k = 2 on test before primary, and the owner's
  computers are never automatic.
- **Automatic suspension** (8.3).

### 8.3 Revert, suspend, withdraw

- **Forward revert, once.** A failed probation applies the previous
  package tree (L before the adoption) as a new forward operation through
  the same job. It is admitted by the `revert` rule: a return to a
  previously approved tree needs no new approval. This is SR8 stage 3.
  - It does not rewind the tape: data written since stays, and kinds
    stay as inert rows (replay projects any kind,
    `internal/store/project.go:231-271`).
  - One revert per adoption, recorded before it acts. If the revert
    fails, or the computer crashes during it, the result is a hold with
    the owner told, never a second automatic step (LU's F19 rule).
  - `choir computer restore` remains the owner's heavier fallback.
- **Suspend (automatic, reversible).** Two `reverted` outcomes on
  distinct computers set the listing to `suspended`, with actor
  `host:probation`. New offers stop. Current carriers keep running and are
  offered a revert (propose). The publisher's owner reinstates with an
  owner act.
- **Withdraw (the publisher's act).** Same effect as suspend. It is
  terminal for that release, though the publisher can publish a fix as
  the next seq.
- **Protected computers.** On the owner's computers and the paper, a
  failed probation *proposes* the revert, with the evidence, instead of
  applying it, until the owner rules (owner question Q4).

### 8.4 Security yank (the one forced change)

The platform's security authority (owner question Q5 names it) sets a
listing to `yanked`, with an advisory. A yank is monotone: Store B is the
host's, so a restore of an adopter never undoes it. Every carrier then
gets a forced set-aside, by the owner's ruling (security, then
availability, then customizations; a forced fix never contains a
computer):

- the package swaps to the previous safe tree on its track, or to the
  base tree;
- `pinned` does not protect against it;
- the owner is told in the Updates app;
- engineering gets a work item to rework any customization.

Nothing is stopped or contained.

## 9. Security

- **Same-origin code.** A package runs in the shell's origin with the
  owner's session (risk R1 of the app-package design). That is acceptable
  only for the owner's own approved code. So:
  - `owner` scope adopts same-origin;
  - cross-owner adoption is refused, whatever the listing scope, until
    S13's sandboxed frame. The frame is an opaque-origin
    `<iframe sandbox="allow-scripts">` whose only API is a postMessage
    bridge to a capability-scoped SDK, with no session cookie and no
    `/api` reach of its own.
  - Listing to `shared` or `public` is allowed before S13, but only for
    reading and review.
- **The package boundary checker is an architectural boundary, not a
  security boundary** (app-packages replan §3.3). It runs at publish (on
  the host, re-derived) and in every adopter build. Authority still lives
  in the backend: the write gate, desk authority, route auth.
- **What a third-party package may import** (S13): its own files, the
  Svelte runtime (bundled), and npm dependencies from a host allowlist
  equal to the shell's locked dependency set. New npm dependencies are
  refused, because `frontend/package.json` and the lockfile are recipe
  paths the host refuses [V, `internal/hostbuild/types.go:179-215`]. The
  SDK reaches it only via `ctx.sdk`. The runtime-reach token rules apply
  (`eval`, `Function`, workers, non-literal `import()`).
- **Namespaces and signing.**
  - Before S11: listings are written only by corpusd, behind the proxy's
    signed owner-act receipts, and adopters read them through their own
    capability.
  - S11: corpusd signs each listing version with the platform-control
    key, and the adopter verifies the signature, the seq and the previous
    version digest. That defends against a replayed "listed" after a
    withdraw.
  - S13: TUF-style roles for publishers outside this cloud (update design
    §4.4). The platform root delegates a targets role per publisher
    namespace, the host signs build attestations under a separate role,
    and a short-expiry timestamp role prevents freeze attacks. The root
    key's custody is owner question Q3.
- **Source only, host builds** (owner 2026-10-14). The S12 module is
  host-built from the published source, and no adopter accepts bytes from
  another computer.
- **Secret scan** at publish and at every widening (4.4).

## 10. Main, the platform base, and the support window

### 10.1 Main is the stable ring

Main holds the `choir` track of every first-party package. The Library
runs ahead of main for owners who opt in. Main stays reviewed, and it is
updated only by the existing human or coding-agent path. The host holds
no GitHub push credential (owner 2026-10-26: "we shouldn't have host
with a GitHub push").

### 10.2 The export step

`choir package export --release <id> --worktree DIR --lane NAME [--onto origin/main]`
is a client-side CLI verb run by a coding agent or the owner on their own
machine:

1. Fetch the record and blobs through the caller's own key, and verify
   them (id, blobs, tree).
2. Refuse unless the package is first-party (it exists in `--onto`), or
   the caller passes `--new-package` for an owner's new app that the
   owner chose to upstream. Third-party packages never go to main.
3. In a lane worktree on `--onto`, replace the package directory with
   the tree (verbatim), and commit with trailers: `Choir-Package-Release`,
   `Choir-Package-Track`, `Choir-Publisher-Computer`,
   `Choir-Publisher-Operation`, `Choir-Tree`.
4. Predict containment, with `selfdev-promote`'s test (each touched path's
   blob and mode equal on `--onto`), which holds trivially because the
   directory is the tree. Then print a PR body with the review diff
   against the base's package tree, the publisher's evidence refs and the
   outcomes.
5. Push nothing. The agent pushes `lane/NAME` with its own credential,
   and review (tier 1) and the landing loop run.

A CI check, `scripts/check-package-export` (yellow), runs on any commit
with a `Choir-Package-Release` trailer. It fails if the package directory
in the commit is not byte-identical to the named release tree, so a
reshaped merge cannot silently strand carriers.

### 10.3 Base moves under adopted packages

Before S12, an adopted package is a stack entry. When a deploy lands and
the base's package tree is not R, sourceproj today answers
`stack_base_moved` and refuses casts. S6 adds a package-aware answer:

- **`upstreamed`.** The base's package tree equals R (the export merged).
  The stack entry retires, and the lock entry's source becomes `base`.
  This is B5's containment test, applied at package granularity: tree
  equality of the root.
- **`package_base_moved`.** The base moved and its package tree is still
  P, or an older tree. R's tree re-materializes onto the new base
  byte-identically, because the stack entry is a tree, not a patch.
  - The job builds it (6.4 steps 2 to 5), and it waits for approval: the
    owner ruled "not yet" on clean rebases without approval, and this
    design treats re-materialization as one until the owner rules (owner
    question Q1, with the recommendation that an identical tree whose
    build and probation pass needs no new approval).
  - Casting is admitted for other paths meanwhile [H: whether sourceproj
    can admit casts with a package entry pending. Settled in S6's tests].
- **`package_behind_base`.** The base's tree moved past P (the
  first-party package changed upstream) and is not R. R would revert the
  base's change, so this is a conflict, handled as in 6.6.

After S12 none of these arise, because packages are not in the source
stack.

### 10.4 The one-week support window

Owner ruling Q1: one week.

- A release is offered for `auto` only to adopters whose booted base was
  deployed within 7 days of the release's `built_on` base.
- Outside that, the offer is `propose` only, flagged stale.
- An adopter whose booted base is older than 7 days gets no new offers
  until it moves base. A security yank still reaches it.
- corpusd retains a release's blobs for at least 7 days after its last
  carrier retires.

## 11. The Updates app

A first-party package, `updates`, read-only, following the owner's
rulings of 2026-10-24, 18:5x and 19:0x.

- **What it shows.** Every update applied to this computer, newest first:
  - platform deploys (an effective-release change at boot);
  - package adoptions, with release, track, seq, review diff and the
    adopter's own screenshots;
  - reverts, suspensions, withdrawals and yanks, with set-asides named;
  - pending proposals, with a link to the approval surface (Texture or
    the CLI). The app has no approve button: "this list of updates
    shouldn't be read-write".
- **Notices.** On opening the desktop, the updates since it was last
  opened. On opening an app updated since it was last seen, a notice in
  that window. "Last seen" is a kind `choir.updates_seen` with writer
  `owner` (the shell writes it on the owner's behalf through owner edits).
- **API.** `GET /api/computers/<id>/updates?since=&limit=` is a projection
  over tape facts: applied events, adoption states, lock changes, boot
  identity changes. It stores nothing new. Desks read it through a
  read-only verb (research and engineering, per the owner).

## 12. Protected computers

The paper (`public_platform`) and the owner's computers
(`premium_always_on`, `critical_protected`) adopt last, and never
automatically at first.

- The class floor in 6.1 is code, not a default.
- Ring soak (8.2) puts them after test and primary.
- Their reverts are proposed (8.3).
- They are never used for proof steps (a metamission invariant).
- The paper adopts only by a manual owner act, after the owner's
  computers.
- A security yank still reaches them, as a set-aside, never a stop.
- Adoption on a busy computer defers to its next idle moment, as deploys
  do. The always-busy exception (a planned, marker-backed refresh) is not
  extended to package adoptions.

## 13. Data model

| Kind | Store | Writer | Identity | Mutable | Purpose |
|---|---|---|---|---|---|
| `choir.package_release` | Store B (corpusd) | corpusd | release id (sha256 of the canonical record) | no | the unit (3.1) |
| `choir.package_listing` | Store B | corpusd (on an owner act or a host authority) | release id | versioned | scope and state (4.2) |
| `choir.package_outcome` | Store B | corpusd (from host probation) | (release id, computer id) | no | health signals (8.2) |
| `choir.package_subscription` | guest tape | owner | (package, track) | versioned | standing intent (6.1) |
| `choir.package_adoption` | guest tape | system:runtime | adoption id = digest(computer, release) | state machine | one adoption (15.2) |
| `choir.updates_seen` | guest tape | owner (owner edits) | (scope: desktop or app id) | versioned | notice cursor (11) |
| package lock | derived, not stored | — | — | — | the projection on `/health` `effective_release.packages`: name to {track, release id, tree, source: base / adopted / set_aside}, from the base manifests plus the applied adoptions. One state authority: the tape. |

Retired: `choir.software_release` (readable, publish refused after S5)
and `choir.release_adoption` (readable, no writer after S5). The
registry entries keep `Writer` for reading old rows, and the write gate
refuses new writes.

## 14. APIs and CLI verbs

Proxy, owner-facing. Every write is an owner act unless marked:

| Route | Verb | Act |
|---|---|---|
| `POST /api/computers/<id>/packages/releases` `{operation_id, package, test_command?}` | `choir package publish` | owner act. Drafts on the guest, then corpusd stores the record and a listing at scope `owner`. |
| `PATCH /api/catalog/releases/<id>/listing` `{scope, shared_with, confirm_paths}` or `{state: withdrawn \| listed}` | `choir package share`, `choir package withdraw`, `choir package reinstate` | owner act (the publisher's owner) |
| `GET /api/catalog/packages`, `.../packages/<name>/releases`, `.../releases/<id>`, `.../releases/<id>/blobs/<sha>` | `choir package search`, `choir package history`, `choir package show` | read, scoped (4.3) |
| `PUT` and `DELETE /api/computers/<id>/packages/subscriptions/<package>` `{track, mode, pin}` | `choir package subscribe`, `choir package unsubscribe` | owner act; refused above the class floor |
| `GET /api/computers/<id>/packages/subscriptions`, `.../packages/lock`, `.../packages/adoptions[/<id>]` | `choir package subscriptions`, `choir package lock`, `choir package adoptions` | read |
| `POST /api/computers/<id>/packages/adoptions` `{release_id}` | `choir package adopt` | owner act: one adoption without a subscription |
| `POST /api/computers/<id>/packages/adoptions/<id>/revert` | `choir package revert` | owner act |
| `GET /api/computers/<id>/updates` | `choir updates` | read |
| (client only) | `choir package export` | none: local git, pushes nothing |

corpusd, internal and host only:

- `POST /internal/platform/packages/releases`
- `PUT /internal/platform/packages/listings/<id>`
- `POST /internal/platform/packages/outcomes`
- `POST /internal/platform/packages/yank` (the security authority, S11)

Guest reads stay `GET` with the computer's own capability, resolved to
its owner.

Approvals of a frozen adoption candidate use the existing `choir self-dev
approve`/`reject`, because the candidate is an ordinary operation.

## 15. States and transitions

### 15.1 Listing

```text
(publish)      -> listed(owner)
listed(s)      -> listed(s')          owner act (widen or narrow)
listed         -> withdrawn           owner act (publisher)            terminal for new offers
listed         -> suspended           host: 2 reverted outcomes on distinct computers
suspended      -> listed              owner act (reinstate)
listed|suspended|withdrawn -> yanked  host security authority          terminal; forces set-aside on carriers
```

### 15.2 Adoption (adopter tape)

```text
offered -> skipped(pinned) | already_current | refused(<reason>) | conflict -> engineering_item
offered -> building -> frozen -> awaiting_approval (propose) | admitted (auto)
awaiting_approval -> rejected | approved
approved|admitted -> applying -> probation -> healthy
probation -> failed -> reverting -> reverted | revert_failed_hold
probation -> failed -> revert_proposed (protected classes)
any non-terminal -> superseded (a newer head on the track, before applying)
any non-terminal -> interrupted (a crash: "interrupted by a restart"; never resumed)
healthy -> upstreamed (the base contains R) | set_aside (yank) | replaced (a later adoption)
```

Refusal reasons: `not_visible`, `not_listed`, `digest_mismatch`,
`blob_mismatch`, `not_confined`, `secret_found`, `class_mode_refused`,
`outside_support_window`, `build_failed`, `checks_failed`,
`capture_blank`, `cross_owner_before_frame`.

### 15.3 Lock entry

```text
base -> adopted(R) -> upstreamed (base) | reverted (previous) | set_aside (security) | adopted(R')
adopted(R) -> package_base_moved -> adopted(R) re-materialized (approval, Q1) | conflict (package_behind_base)
```

## 16. Failure modes, written before code

| # | Failure | Stop | Proof |
|---|---|---|---|
| L1 | A release carries the publisher's other unupstreamed changes | Only the root directory's tree is carried. The parent must be the base or the track head (3.2.3). | Unit test: a two-operation stack where op 2 touches the package and op 1 touches the shell. The release carries only the package. Deployed on T-A in S2. |
| L2 | The guest claims confinement falsely | corpusd re-derives the paths from the blobs and rebuilds the tree from `tree_patch` | Unit test: a guest-forged `changed_paths` |
| L3 | A symlink, submodule or mode trick escapes the root | Refused at publish and at place (3.2.5, 6.4.2) | fixtures |
| L4 | A listing or blob is visible to another owner | Every read is owner-filtered, and an unseen release answers 404 (4.3) | S1 and S3: a second account's key gets 404 on list, read and blob |
| L5 | A replayed old listing re-enables a withdrawn release | Versioned listings with a predecessor digest, signed from S11. Adopters keep the highest version seen. | S11 test |
| L6 | A desk widens a scope or publishes | Scope changes need an owner-act receipt over the exact subject. A request-carried intent is read from the receipt only. | Test: a desk act with a forged intent is refused |
| L7 | A model-mediated step fails (adopt-1418) | The deterministic job makes zero model calls | Test: a provider-call counter reads 0 across an adoption |
| L8 | The adopter's own package changes are overwritten | The three-way check sends `conflict` to engineering | Test, and a deployed leg in S5 |
| L9 | A crash mid-adoption resumes on boot | The crash rule: `interrupted`, and the next tick is new work. Two crashes close the release for this computer. | Test: a markerless boot with an adoption in `applying` |
| L10 | A revert loop | One revert per adoption, recorded before acting, then a hold | Test |
| L11 | Probation passes while the app is broken | The host-run package smoke (non-blank, no console error, the window opens) | S7 deployed leg: a release whose `App.svelte` throws on mount |
| L12 | Probation fails falsely | The cost is one forward revert of a reversible change. Outcomes are recorded, so a flaky smoke shows as reverts across releases. | weak measure |
| L13 | Auto mode on a protected computer | The class floor is enforced at subscription write and at gate time | Test across all five classes |
| L14 | Cross-owner same-origin code | Adoption is refused across owners until S13 | S3 deployed leg |
| L15 | A mixed candidate is published as a package | `package_change_not_confined`, naming the paths | S2 deployed leg |
| L16 | The export reshapes the tree, so carriers stay `stack_base_moved` | Export writes the tree verbatim, and the CI check `check-package-export` verifies it | S4 test plus a real export |
| L17 | Base moves strand adopters hourly | S6's `package_base_moved` re-materializes (approval pending Q1), and S12 removes the cause | S6 deployed leg: deploy between adopt and export |
| L18 | A base SDK change breaks an adopted package | Before S12 the adopter build fails, so `build_failed` and nothing applies. After S12, `requires_sdk`, and the base package wins. | S5 and S12 tests |
| L19 | A subscription storm (many releases quickly) | Coalescing to the head, one open adoption per package, the 5-minute tick, and the host builder's existing per-computer quota for S12 builds | Test |
| L20 | Store B is unavailable mid-tick | The tick ends with nothing applied, and the next tick is new work | Test |
| L21 | Private data in shared evidence | No publisher screenshots in `shared` or `public` unless named in the widening act, and `--confirm-paths` | S3 test |
| L22 | A yank is undone by a restore | Yank state is host-side and monotone, and the gate reads it at apply | S11 test: restore to a pre-yank head, then the set-aside reasserts |
| L23 | A release whose parent is a withdrawn release | The track head excludes withdrawn releases, so publish refuses `track_moved` | Test |
| L24 | Two publishers fight over a name | A track is per owner, so the same name on two tracks is two lines. An adopter follows one track per package, and a subscription to a second track is refused until the first is removed. | Test |
| L25 | A package declaring kinds before S12 | `package_kinds_need_platform` | Test |
| L26 | An adoption on a busy owner's computer | It defers to idle, and the always-busy exception does not apply | Test |
| L27 | The 8 MiB blob cap is too small for a package | The publish refusal names the size. The cap is raised only after S2 measures real packages. | S2 measurement |

## 17. Build plan

Every step:

- lands alone, in a lane on current main, with tests first, the cheap
  checks, review by its tier, and the landing loop;
- is behind a switch that defaults off. Host switches are env selectors
  in `nix/node-b.nix`, set by FP0 (owner-delegated). Guest switches are
  per-computer settings in `internal/store/computer_setting.go`. A
  computer setting stays off unless FP0 or the owner turns it on, test
  computers first.

The two test computers:

- **T-A** is b@c's test computer
  `computer-6c50d6e566143ae32cdb660faa60e24d`, the publisher.
- **T-B** is the second, described in section 18.

Each step's proof runs on choir.news.

### S0. Problem records first (green, tier 0)

- **Scope:**
  - `docs/problems/release-reads-not-owner-scoped-2026-10-26.md`
  - `docs/problems/release-carries-the-cumulative-stack-2026-10-26.md`
  - `docs/problems/release-adoption-is-model-mediated-2026-10-26.md`
  - `docs/problems/selfdev-candidates-cross-package-boundaries-2026-10-26.md`
    (with the T1a Reader scope as evidence)
- **Rollback:** none needed.
- **Proof:** the docs truth checker.

### S1. Owner-scoped release reads (red: auth on a live read path; tier 1: a narrowing, one-act rollback)

- **Scope:**
  - `internal/proxy/releases.go`: pass the caller's owner, and stop
    reading as the host;
  - `internal/platform/releases.go`: list by `OwnerID`, the single read
    and the blob read check the owner (a guest via its computer's owner),
    and an unseen release answers 404;
  - `internal/objectgraph` `ListFilter` with `OwnerID`, if it is missing
    [H].
- **Switch:** `CHOIR_RELEASE_READS=owner` (host env, default `open` when
  it lands, flipped by FP0 right after the deploy proof).
- **Tests first:**
  - another owner's key gets 404 on list, read and blob;
  - the owner sees their own;
  - a guest of owner A reads A's releases and not B's;
  - the host internal caller is unchanged;
  - `open` reproduces today's behaviour byte for byte.
- **Rollback:** `CHOIR_RELEASE_READS=open`.
- **Proof:**
  - T-A's existing releases are listed with b@c's key;
  - a key of a second test account lists none of them, and gets 404 on a
    known release id and its patch blob;
  - T-B (same owner) reads them through its own capability.

### S2. The package release record and owner-scope publish (red: Store B kinds, owner act; tier 2: a visibility defect would be silent)

- **Scope:**
  - `internal/pkgrelease/` (new): the record, canonical form, validation,
    the confinement and own-change derivation, tree and patch helpers
    (moved from `internal/release`);
  - `internal/platform/packages.go` (new): the publish and listing
    routes, re-derivation, insert-if-absent, scope `owner` only;
  - `internal/objectgraph/registry.go`: two kinds;
  - `internal/agentcore/api_package_release.go` (new): the guest draft;
  - `internal/proxy/packages.go` (new): the publish owner act and the
    catalog reads at owner scope;
  - `cmd/choir/package.go` (new): `publish`, `show`, `history`, `search`;
  - the setting `packages.publish`.
- **Tests first:** L1, L2, L3, L6, L15, L23, L25, L27, and
  `track_moved`.
- **Rollback:** setting off (the routes answer 404), or a revert. Rows
  stay inert.
- **Proof:**
  - on T-A, an applied package-confined operation (a T1a change to
    `reader` or `files` that stays in its directory, or a one-line Reader
    manifest description change requested through the CLI) is published;
  - `choir package show` verifies it;
  - a mixed operation is refused with its outside paths named;
  - a second account's key sees nothing;
  - the record's blob sizes are measured (L27).

### S3. Sharing scopes, withdraw and reinstate (red: visibility; tier 2)

- **Scope:** listing versions, `share`, `withdraw`, `reinstate`;
  `--confirm-paths`; evidence attachment; the 404 rule for every scope; and
  the cross-owner adoption refusal (`cross_owner_before_frame`) wired
  into the S5 gate seam.
- **Tests first:** L4 across all three scopes, L21, L24, and the listing
  history kept.
- **Rollback:** setting `packages.share` off. Listings fall back to
  scope `owner` at read time.
- **Proof:**
  - T-A's release is shared with the second test account: that key now
    sees it, and a third key does not;
  - after a withdraw it disappears from catalog heads, but the history
    shows it withdrawn;
  - public listing is shown to work for reading;
  - an adoption attempt from the second account's computer is refused
    `cross_owner_before_frame`, if that account has a computer; otherwise
    this leg moves to S13.

### S4. Export to main (orange: a client CLI verb plus a CI check; tier 1)

- **Scope:** `cmd/choir/package_export.go` (new; it reuses
  `scripts/selfdev-promote`'s containment logic, ported to Go or called
  from it); `scripts/check-package-export` (new) and one CI line; and the
  PR body template.
- **Tests first:**
  - L16;
  - a third-party package is refused;
  - `--new-package` is required for a package absent from `--onto`;
  - the verbatim tree check fails on a one-byte edit.
- **Rollback:** a revert. The verb writes only local git.
- **Proof:**
  - FP0 exports T-A's S2 release, and the lane passes the check;
  - it is reviewed tier 1 and landed;
  - after the deploy, T-A's projection reports `upstreamed` for that
    operation.

  Before S5 this replaces `selfdev-promote` for package-confined
  candidates.

### S5. Subscriptions, the tick, and the deterministic adoption job in propose mode (red: a new apply path that bypasses the desks, a capsule under a runtime assignment, boot-time tick; tier 2)

- **Scope:**
  - `internal/store/package_subscription.go` and
    `internal/store/package_adoption.go` (new kinds);
  - `internal/agentcore/package_adoption.go` (new: the tick, the
    three-way check, the `package_adopt` assignment and its fixed recipe,
    the gate hand-off);
  - the capsule seam for a runtime-owned assignment (the [H] in 6.4);
  - the proxy and CLI `subscribe`, `subscriptions`, `adopt`, `adoptions`;
  - the class floor (6.1);
  - the setting `packages.adopt`;
  - **delete** `adoptionRequestText` and the P8 desk-prompt submit, and
    retire the `choir.release_adoption` writer;
  - refuse `software_release` publish.
- **Tests first:**
  - L7 (zero model calls);
  - L8;
  - L9;
  - L13 (the subscription write half);
  - L18 (build refusal);
  - L19;
  - L20;
  - L26;
  - `already_current`;
  - multi-release fast-forward;
  - `superseded`;
  - off is inert (no tick, no kinds written).
- **Rollback:** setting `packages.adopt` off: the tick stops, open
  adoptions close `interrupted`, and applied adoptions stay as ordinary
  applied operations, revertible by `choir computer restore`. Otherwise
  a revert.
- **Proof:**
  - T-B subscribes `reader` on T-A's track, `propose`;
  - after T-A publishes (S2), and with no adopt command, T-B shows an
    adoption that reaches `awaiting_approval` with its own screenshots and
    zero model calls in its trace;
  - FP0 approves, T-B serves R, and its lock names R;
  - negative legs:
    - a `pinned` subscription adopts nothing;
    - T-B with its own local edit to `reader` gets `conflict` and an
      engineering item;
    - a tampered blob is refused.

### S6. The package lock, identity and package-aware containment (red: boot-path projection; tier 2)

- **Scope:** `internal/sourceproj` (a stack entry of kind `package_tree`;
  the `upstreamed`, `package_base_moved` and `package_behind_base` answers
  at package granularity); `/health` `effective_release.packages`; and the
  vmctl list column.
- **Tests first:**
  - L17;
  - re-materialization yields a byte-identical root;
  - `package_behind_base` is a conflict;
  - casts on other paths are admitted while a re-materialization awaits
    approval (or refused, if the [H] fails, with a named reason);
  - a crash between apply and projection is reconciled.
- **Rollback:** setting `packages.lock` off, which gives today's
  `stack_base_moved` behaviour.
- **Proof:**
  - with T-B carrying R, one ordinary deploy lands, and T-B reports
    `package_base_moved` with a re-materialized candidate awaiting
    approval, which is approved;
  - then S4's export lands, and both T-A and T-B report `upstreamed`.

  This is B5 on two computers, one of them an adopter, with the U4
  pressure measured rather than assumed.

### S7. Probation, the package smoke, outcomes and forward revert (red: run acceptance, host probes, automatic action; tier 2)

- **Scope:**
  - the probation precommitment kinds (`update_healthy_within` and
    `no_restore_within`, in `internal/types/commitment_rules.go`);
  - the host package smoke (vmmanager visual capture) and
    `choir.package_outcome`;
  - the `revert` rule and the forward-revert job;
  - suspension after 2 reverts;
  - protected classes propose their revert;
  - the setting `packages.probation`.
- **Tests first:**
  - L10;
  - L11;
  - L12 recorded;
  - one revert per adoption;
  - `revert_failed_hold`;
  - suspension needs distinct computers;
  - the tape is not rewound by a revert.
- **Rollback:** setting off: no probation, no revert, as in S5.
- **Proof:**
  - T-A publishes a deliberately broken `reader` release (it throws on
    mount);
  - T-B adopts it (propose, approved);
  - probation fails the smoke, T-B reverts once to the previous tree, and
    an outcome `reverted` is recorded;
  - a second test computer reverting suspends it. With only two
    computers, T-A adopting its own broken release back from base counts,
    or the leg waits for a third.

### S8. The Updates app (orange: a new read API and a first-party package; tier 1)

- **Scope:** `frontend/src/packages/updates/` (new), `GET
  /api/computers/<id>/updates`, `choir.updates_seen`, the desktop and
  app-open notices, `LIBRARY_NAME`, and the desk read verb.
- **Tests first:**
  - every update type appears once;
  - no write route exists in the app;
  - notices fire once per update;
  - desks read the same list.
- **Rollback:** a revert. The kind stays inert.
- **Proof:** on T-B, the app lists the S5 adoption, the S6
  re-materialization, the S7 revert and the last deploy, with
  screenshots. Reopening the desktop shows only new updates.

### S9. Auto mode on test computers, with rings (red: decision policy, run acceptance; tier 2)

- **Scope:** the `package-adopt-v1` deterministic seat in standing
  consensus; ring soak from outcomes (k = 2); the class floor at gate
  time; and test-class default `auto`.
- **Tests first:**
  - L13 (the gate-time half);
  - an `auto` adoption without soak at primary is refused;
  - a non-fast-forward is never auto;
  - consensus dissent leaves `awaiting_approval`.
- **Rollback:** setting `packages.auto` off. Everything falls back to
  propose.
- **Proof:** T-B on `auto` adopts T-A's next `reader` release with no
  human act, passes probation, and the outcome is recorded. The same
  release reaches a primary-class computer only as `propose`.

### S10. The support window and staleness (orange: offer policy; tier 1)

- **Scope:** the 7-day rule (10.4), with stale offers propose-only; the
  adopter-too-old refusal; and blob retention.
- **Tests first:** boundaries at 7 days, plus base deploy time read from
  the deploy record [H: the source of a base's deploy time; settled by
  reading the platform release record].
- **Rollback:** setting off.
- **Proof:** a release built on a base older than 7 days (made
  deliberately by publishing from T-A pinned to an old base, or simulated
  through the test-only clock seam) is offered `propose` only on T-B.

### S11. Signed listings and the security yank (red: signing, forced set-aside; tier 2)

- **Scope:** the platform-control signature on listing versions, verified
  by the adopter; `POST /internal/platform/packages/yank`; and the forced
  set-aside to the previous safe tree or the base tree, with an Updates
  app entry and an engineering item.
- **Tests first:**
  - L5;
  - L22;
  - a set-aside never stops or contains the computer;
  - `pinned` does not block it;
  - a yank reaches an adopter outside the window.
- **Rollback:** setting off for the signature check. The yank route is
  host-only and is left unused.
- **Proof:** a yank of T-A's release sets T-B's `reader` aside within one
  tick, the Updates app says so, and a restore of T-B to a pre-yank head
  re-asserts the set-aside.

### S12. Runtime install (P5): host-built modules, install as data (red: a new code-loading path, tape; tier 2)

- **Scope:**
  - `module/v1` ABI and build alias;
  - the package-only host build output;
  - `requires_sdk` and `SDK_VERSION`;
  - the S7-0 branch's lifecycle reducer, build store and verified loader,
    rebased (app-packages replan §1.2);
  - install-time kind registration through `RegisterPackageKind`;
  - packages leave the source stack;
  - kinds are admitted in releases.
- **Tests first:**
  - the P5 goal file's five rows;
  - L18 (after S12);
  - an install, restore and reinstall round trip;
  - an unknown SDK version falls back to the base package;
  - verified bytes before mount.
- **Rollback:** setting `packages.runtime_install` off, which returns to
  the S5 build path. Installs fold to inert.
- **Proof:** T-B installs T-A's release without a frontend build or a
  restart, a deploy then leaves it untouched (no `package_base_moved`),
  and a release with a new kind writes it.

### S13. Cross-owner adoption: the sandboxed frame and namespace signing (red: a security boundary; tier 2)

- **Scope:** the opaque-origin frame and postMessage SDK bridge with
  capability grants; the third-party import allowlist; the TUF-style
  roles (9); and lifting `cross_owner_before_frame` for framed packages.
- **Tests first:**
  - the frame has no cookie and no `/api` reach;
  - the bridge refuses an ungranted capability;
  - a namespace key cannot sign another namespace;
  - an expired timestamp is refused.
- **Rollback:** setting off, which brings back the cross-owner refusal.
- **Proof:** a computer of the second test account (T-C) adopts T-A's
  shared release in the frame, and an attempt to read the owner's
  session from inside the frame fails.

### S14. Widening, and deleting the retired code (orange and red per item; tier 1 each)

- **Scope:**
  - primary to `auto` (SR8 stage 4);
  - the owner's computers stay `propose` with evidence;
  - the paper adopts by manual act, last;
  - delete the `software_release` publish route and `choir release
    publish|adopt`, M9a's dark platform-follow path, and the unused
    `platformrelease` lineage fields, each after a citer search.
- **Tests first:** the class table end to end, and no citer remains for
  each deletion.
- **Rollback:** per item, a revert.
- **Proof:** a release reaches test, then primary automatically, then the
  owner's computers as proposals. The paper is not touched until the
  owner acts.

### Order and parallelism

- S0, then S1, the security fix, at once.
- S2 next.
- S3 and S4 in parallel after S2.
- S5 after S2, and S6 after S5.
- S7 after S5, then S8, which can start earlier on the S5 data.
- S9 after S7.
- S10 and S11 after S5.
- S12 after S6, in parallel with S9 to S11.
- S13 after S12.
- S14 last.

The first end-to-end loop without FP0 hand work is S2, S4 and S5
(publish, offer, approve, export), about a day of lanes. The whole
system is S0 to S14, several days, of which S12 and S13 are the long
poles.

## 18. The test computers

- **T-A: `computer-6c50d6e566143ae32cdb660faa60e24d`** (b@c's test
  computer), the publisher. `source.projection` is on already.
- **T-B: a second computer for b@c**, the adopter. It needs:
  - class `test` and the same owner as T-A, so that `owner` scope covers
    it;
  - to be minted after S2's deploy, so that its seed carries U1b and the
    current packages;
  - `source.projection` on;
  - an owner-minted API key with `owner:acts`. A key cannot mint
    `owner:acts`, so the owner mints it once, signed in (app-packages
    replan §5 item O);
  - the class-test visual capture grant;
  - the switches `packages.publish`, `packages.adopt` and later ones
    turned on by FP0;
  - no Autopaper schedules or other standing work, so that probation
    reads cleanly.
- **For the visibility legs (S1, S3):** an API key of a second test
  account. No computer is needed until S13.
- **T-C (S13 only):** a `test` computer owned by that second account.

## 19. Open questions that are the owner's

1. **Q1. Re-materialization authority.** Is re-applying an adopted
   package's byte-identical tree on a new base (after its build and
   probation pass) a "clean rebase" that needs approval under "not yet"?
   Default: yes, approval needed. Recommendation: no, because the approved
   content is unchanged. It matters hourly until S12.
2. **Q2. Who may publish.** May a request-carried publish intent name
   `shared` or `public`, or only `owner`? Default: `owner` only. Separately,
   may FP0 or a coding agent publish under the owner's standing direction,
   as it approves today? Default: yes, at `owner` scope only.
3. **Q3. Signing root custody (a credential question).** Where does the
   platform root key for namespace delegation live: the host, offline, or
   the owner's hardware key? S11 needs only the existing platform-control
   key. S13 needs this answer.
4. **Q4. Automatic revert on protected computers.** May probation revert
   automatically on the owner's computers and the paper, or only propose?
   Default: propose.
5. **Q5. Yank authority (an authority question).** Who may yank: an
   automatic advisory feed, the owner, FP0? Default: the owner, and FP0
   on the owner's instruction. There is no automatic feed until one
   exists.
6. **Q6. The GitHub path for exports (a credential question).** Exports
   are pushed by the coding agent's own credential as `lane/*` branches
   and merged by FP0 after review, as today. Should a package-class
   export ever merge on a standing policy without a per-change review?
   Default: no, tier 1 review on every export.

## 20. Class and heresy delta

- **Mutation class:** green (this document). The design governs red steps
  (S1, S2, S3, S5, S6, S7, S9, S11, S12, S13), so its review is tier 2.
- **Protected surfaces the build will touch:** auth on read routes,
  Store B kinds and owner acts, a new apply path and capsule seam, the
  boot-path projection, run acceptance (probation and the auto gate), and
  a new code-loading path. Each is named per step above.
- **Heresy delta:**
  - *discovered:*
    - reads of releases are not owner-scoped (live);
    - under U1 a release's patch carries the whole stack;
    - adoption runs a deterministic step through model desks;
    - real self-development candidates routinely cross package
      boundaries (T1a's Reader scope);
    - the publisher's screenshots would leak private data if they rode a
      shared release.
  - *introduced:* none (docs only).
  - *repaired:* none yet. S1 repairs the first, S2 the second, S5 the
    third.
