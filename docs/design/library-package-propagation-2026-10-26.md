# The Choir Library: how an accepted change reaches other computers and the platform (2026-10-26)

Status: **design, green** (docs only, no code). This is an architectural
design that a build will follow, so it is reviewed like the change it
designs: **tier 2** (Devin and DeepSeek). It is station SR9 of
[the readiness metamission](../definitions/choir-selfdev-readiness-metamission-2026-10-24.md),
and its goal file is [choir-sr9-library-2026-10-26.md](../definitions/choir-sr9-library-2026-10-26.md).

- **Revision 2.** This revision answers review 257 (both seats requested
  changes; the disposition is in section 21). It also adds the owner's
  2026-10-26 02:0x question about the platform base image (section 10.5).
  Review 265 (Devin, one seat) approved it with six required edits, which
  are applied here (section 21, last row).
- **Base.** Written on lane `lane/sr9-design` from `origin/main` at
  `355c603ad`.
- **Starting point.** The read-only investigation of 2026-10-25 (the SR9
  propagation report) mapped the existing release system with file:line
  refs. This design replaces that report's recommendation wherever the
  owner's 2026-10-26 rulings differ from it.

Evidence labels:

- **[V]** read at file:line on this tree;
- **[D]** stated in a cited design or ledger entry;
- **[H]** a hypothesis, with what would settle it.

## 0. The name, in one place

| | |
|---|---|
| Product name | **the Choir Library** (short form: **the Library**) |
| Owner ruling | 2026-10-26: "to a mainstream audience, we can say that this is the choir library". The earlier working names (the Commons, Exchange, Shelf) are retired. |
| Where the name appears | this table; the Updates app and catalog UI copy, through one frontend constant `LIBRARY_NAME` (S8); prose in docs |
| Where it does not appear | object kinds, routes, CLI verbs, settings and Go packages. These use functional names (`package`, `platform`, `catalog`, `subscription`, `adoption`), so a rename is one constant plus a docs edit. |

## 1. The answer in one page

1. **The unit for adoption is a package release: one package's own
   change.**
   - It is a content-addressed record of one package directory
     (`frontend/src/packages/<name>/`), taken as a whole git tree.
   - With it travel the tree it replaced (its parent), the review diff,
     the manifest, the platform base it was built on, the publisher's
     receipts and settled precommitments, and its provenance.
   - The release *content* can only be files under that one directory,
     and the host checks this on the bytes it stores. So a release can
     never carry the publisher's other unupstreamed changes.
   - Whether the publisher's *operation* changed anything else is a
     publisher-side check. If that check fails, the adopter's build and
     probation catch it (section 3.2).
2. **A change outside a package is never adopted between computers.**
   Shell, Go, desks, nix, store schema, generated kinds and app.css are all
   such changes.
   - Such a change can still be *published* to the Library as a
     **platform proposal**: a record at owner scope that the owner shares
     with the platform.
   - A platform admin's act, "propose for the platform", queues it. A
     pull-based exporter then opens a lane on main.
   - The reviewed landing loop decides. The deploy updates the base image
     for every computer, and every carrier then reports `upstreamed`.
   - This retires `scripts/selfdev-promote` as FP0's hand step (section
     10.5).
3. **Private by default; publishing is the owner's act.**
   - **Scopes.** A release is visible only to its owner and the owner's
     own computers until the owner widens it. The wider scopes are named
     owners or computers, the platform (platform admins only), or
     everyone.
   - **Request-carried acts.** A publish instruction inside the owner's
     own request is the owner's act.
   - **Admin bypass.** A private release is invisible even to a platform
     admin.
   - **The live fix.** Today's unscoped reads are fixed first (S1, built
     on `lane/lib-s1`).
4. **Discovery is a catalog view over listed releases.** It shows packages
   and tracks (a publisher's line of a package), per-package version history
   and search, always filtered to what the caller may see (section 5).
5. **Adoption is per computer, by subscription, and deterministic.**
   - **Subscriptions.** A subscription names a package, a track and a
     mode (`auto`, `propose` or `pinned`).
   - **The class floor.** The proxy enforces it when the subscription is
     written, from the class vmctl reports, and signs the class into the
     owner act.
   - **The job.** A deterministic adoption job, with no model call:
     1. a three-way tree check;
     2. a fixed capsule recipe;
     3. the adopter's own screenshots;
     4. Freeze;
     5. the adopter's own gate decides.
   - **Conflicts.** Only a conflict becomes engineering work (section 6).
6. **Install means a build on the adopter now, and data later.**
   - First-party packages are compiled with the shell, so adopting one
     today rebuilds the adopter's SPA in its capsule.
   - Auto mode also needs one host build per adoption: standing
     consensus requires a host-build vote.
   - Runtime install (S12) builds a module once per release on the host.
     Install then becomes a tape fact, with no rebuild and no restart
     (section 7).
7. **Every apply is on probation, with one forward revert.**
   - A failed probation swaps back to the previous package tree as a new
     forward operation, once.
   - Two failures on different computers suspend a release.
   - The publisher can withdraw it.
   - The security authority can yank it, which sets the package aside and
     never contains a computer. The precedence between a crash restart, a
     planned restart and probation is stated in section 8.5.
8. **Main stays the stable ring, and no host holds a GitHub push
   credential.**
   - A package release or platform proposal reaches main only through
     the pull exporter.
   - The exporter is a coding agent, or a CI job with its own credential
     (sections 10.2, 10.5).
   - The exporter refuses when main's package tree is no longer the
     release's parent.
   - No clean rebase applies without approval (owner: "not yet").
9. **The Updates app is a read-only projection** of every update applied
   to the computer. Approvals stay where owner acts live today (section 11).
10. **Protected computers adopt last, and never automatically at first.**
    - Only test-class computers may be `auto`, because standing consensus
      admits only the test tier.
    - Primary computers and the owner's computers get proposals, ordered
      by ring soak.
    - The paper is `pinned` and adopts only by a manual act, last.
    - Primary `auto` needs a separate design that changes the consensus
      tier gate (SR8 stage 4), which is not in this design (section 12).

The build has fifteen steps, S0 to S14, plus S4b. Each lands on its own,
behind a default-off switch, with a deployed proof on the test computers
named in section 18.

- **S1.** Owner-scoped reads, a security fix to what is live today.
- **S2 to S5.** The first end-to-end loop: publish; share; export
  (including platform proposals and the pull exporter); deterministic
  propose-mode adoption.
- **S6 and S7.** Package-aware containment, then probation.
- **S8.** The Updates app.
- **S9.** Auto mode on test computers.
- **S10 and S11.** The support window, signed listings and yanks.
- **S12.** Runtime install.
- **S13.** The cross-owner sandboxed frame.
- **S14.** The widening to protected computers as proposals, and the
  deletion of retired code.

## 2. What exists, and what this design keeps, redesigns or deletes

The 2026-10-25 report holds the full map. In short [V]:

- **`choir.software_release`** (`internal/release/record.go`) is one
  applied operation's source patch against a platform base commit.
  - The owner publishes it through the proxy (`internal/proxy/releases.go`)
    to corpusd, host only (`internal/platform/releases.go:76-81`).
  - The kind is registered at `internal/objectgraph/registry.go:196`.
  - `Declarations.Packages` is reserved and must be empty (`record.go:117`,
    `:203`).
- **Under U1 the patch is cumulative over the stack**
  (`internal/sourceproj/compose.go`, `Cumulative`).
- **P8 adoption** (`internal/agentcore/release_adoption.go:116-219`, routed
  as `/api/computers/<id>/self-development/adoptions`) verifies seven rungs
  and then hands the work to the desks as a prompt
  (`adoptionRequestText`, `:336`, submitted at `:199`).
- **Reads are unscoped** on main: `releases.go:71` and `:290`, and
  `proxy/releases.go:185` (an admin-scope pass), `:192` and `:204`.
- **Packages exist at build time only:** `frontend/src/packages/{image,reader}`,
  `choir-package.json` v1, `check-packages.mjs`, `package-boundary-plugin.mjs`
  and `frontend/src/lib/app-sdk.ts`.
- **Main is reached** by `scripts/selfdev-promote`, then FP0's push and the
  landing loop. Containment then retires the stack as `upstreamed`
  (`internal/sourceproj/sourceproj.go:76-103`, `contained.go`).

| Existing piece | Disposition |
|---|---|
| `release.Record` verification (`VerifyID`, `VerifyPatch`, `PatchedTree`) and the secret scan `sourcePatchAddedSecrets` (`internal/agentcore/api_self_development_patch.go:162`) | **Keep and reuse.** The verification code moves into the new `internal/pkgrelease`, and the scan stays where it is. |
| `choir.software_release` and P8 adoption (`choir release publish/adopt`, `adoptionRequestText`, `choir.release_adoption`) | **Keep working, unchanged, until S14.** A same-owner adoption keeps working after S1. Cross-owner P8 reads break at S1 (section 4.3), as intended. S14 deletes the desk-prompt apply, the publish route and the verbs, and retires both kinds (readable, no writer), once S9 has proved the new loop. Nothing is retired behind a default-off deploy (review 257 F2). |
| `Declarations.Packages` reservation | **Superseded** by the new kinds. The field is never filled. |
| `scripts/selfdev-promote` | **Retired as FP0's hand step** by the pull exporter (S4, S4b). The exporter reuses its containment prediction. The script stays as a fallback until S14 deletes it. |
| P4 standing consensus (`internal/platform/standing_consensus.go`, enforcement at `:141` and `:232`; the proxy reads the class from vmctl at `internal/proxy/self_development.go:273-295`) | **Reuse** it as the auto gate on test computers (S9). Its test-tier refusal **stays** and is the floor: primary `auto` is out of scope (section 12). |
| Host builder (`internal/hostbuild`) | **Reuse** it for the auto-path build vote (S9), the S12 module build, and packages that later carry Go. |
| M9a platform-follow push (dark since a47122de0) | **Deletion candidate** in S14, after a citer search. |
| `internal/platformrelease` lineage fields with no callers | **Reuse** `CompatibilityRange` for the SDK range (S12). Delete the rest in S14 after a citer search. |
| `source.rebase_offer` (U4, unbuilt) | **Unchanged.** Packages avoid it: as a tree entry before S12 (S6), and as data after S12. |

## 3. The unit: a package release

### 3.1 The record

- **Kind and storage.** `choir.package_release`, in Store B, writer
  corpusd (`WriterCorpusd`). It is immutable and written insert-if-absent.
- **The release id** is the sha256 of the canonical JSON (RFC 8785, as
  today).
- **Schema** `choir.package_release.v1`:

```text
PackageRelease {
  schema:     "choir.package_release.v1"
  package:    { name, manifest_version }           // name = directory name; manifest_version = the manifest's semver (declared, never compared)
  track:      <publisher owner id>                 // a publisher's line of this package (3.4)
  parent:     { kind: "base",    commit, tree }    // the package subtree in a platform base commit
            | { kind: "release", release_id, tree }// or a previously listed release on the same track
  parent_patch?: { sha256, bytes }                 // base parent only: binary diff empty-tree -> the full-path tree holding only root/ at the parent (3.2, check 3)
  source: {
    root:          "frontend/src/packages/<name>"
    tree:          <git tree id of the subtree at root>
    tree_patch:    { sha256, bytes }   // binary diff empty-tree -> the full-path tree holding only root/...
    review_patch:  { sha256, bytes }   // binary diff parent subtree -> subtree, same paths, for review
    changed_paths: [...]                // the publisher's claim of the operation's paths (publisher-side, 3.2)
    built_on:      <platform base commit the publisher built and verified it on>
  }
  manifest:   { sha256, abi, requires_sdk?, kinds_count, opens_count }   // requires_sdk required from S12
  build?:     { module_digest, builder_receipt_digest }                  // S12 only
  publisher:  { owner_id, computer_id }
  operation:  { operation_id, bundle_digest, decision_event, materialization_receipt,
                checkpoint_ref, verifier_refs[] }
  precommitments: [ { record_id, kind, question, p_yes_permille, settled, resolve_record_id } ]
  declarations: { test_command? }
}
```

**Not carried:**

- **Evidence assets.** The publisher's screenshots render the publisher's
  own data, and the rule is that evidence is never published [D,
  metamission station TE]. Adopters capture their own (6.4). A reviewer
  who wants the publisher's captures opens them on the publisher's
  computer, through its existing visual routes, by operation id.
- **Cumulative patches.**
- **Executable bytes.**

**Blobs.** The two patches live in a new host content namespace,
`package`. It is separate from `contentNamespaceRelease`, so the
software_release cap is untouched.

- The cap starts at **2 MiB**, the same as today.
- S2 measures real packages. The cap is raised only after that
  measurement, in a reviewed change (L27).

### 3.2 Deriving the release: what is host-verified and what is publisher-side

`choir package publish --computer C --operation OP --package NAME`
drafts the record on C, the guest, as `releaseDraft` does today
(`internal/agentcore/api_release.go:45-120`).

**Publisher-side (guest) checks.** These are the publisher's own checks:
the host has no copy of the publisher's repository, so it cannot redo
them (review 257 F1).

1. **Applied only.** OP must be applied.
2. **Operation confinement.** The paths OP changed must all lie under
   `frontend/src/packages/NAME/`.
   - The guest finds them by comparing the effective source just before
     OP's applied event with the source just after it. The "before" side
     is sourceproj's ref for the previous applied event, or the base.
   - Any other path refuses with `package_change_not_confined`, and the
     refusal names the outside paths. Those paths are a platform
     proposal (10.5).
3. **Own change only.** The parent subtree P (NAME before OP) must equal
   either the package subtree in the stack's platform base, or the tree
   of a release listed on the publisher's own track.
   - Otherwise the draft refuses with `package_parent_unpublished`,
     naming the earlier operation.
   - The publisher's changes to other paths never matter, because only
     NAME's subtree is carried.

**Host-verified checks.** corpusd runs these on the bytes it stores, so
they are not circular.

1. **The content is confined.** corpusd applies `tree_patch` to the empty
   tree. Every path in the result must lie under `root`. The result may
   contain no symlink, no submodule, no mode other than 100644 or 100755,
   and no `.gitattributes`.
2. **The tree matches.** corpusd extracts the subtree at `root` and
   requires its id to equal `source.tree` (review 257 F8: compare the
   subtree, not the full-path tree).
3. **The review patch matches.** `review_patch`, applied to the parent
   subtree, yields the same subtree. Where the parent subtree comes from
   (review 265, finding 2):
   - **A parent release:** from that release's stored `tree_patch`, which
     corpusd already holds and checked when it listed that release.
   - **A base parent:** the publisher carries the parent subtree as one
     more blob, `parent_patch`, in the `package` namespace (an empty-tree
     diff, like `tree_patch`). corpusd applies it to the empty tree, runs
     checks 1 and 2 on the result, and requires the subtree id at `root`
     to equal `parent.tree`. It then requires `parent.tree` to equal the
     tree the host itself holds for that base: `git rev-parse
     <parent.commit>:<root>` in the deploy checkout's object store.
   - **What the host holds** [V]. The deploy keeps `/opt/go-choir` as a
     full clone, created by `git clone` and moved by `git fetch` of each
     deploy commit, then `git reset --hard` and `git clean -ffdx`
     (`.github/scripts/node-b-ci-deploy:219-236`). Reset and clean change
     the worktree, never the object store, and every deployed main
     commit is an ancestor of the current deploy, so its objects stay
     reachable there. corpusd already reads that object
     store, read only, for the P5 host build (`CORPUSD_HOST_BUILD_SOURCE_DIR=/opt/go-choir`
     with git on its path, `nix/node-b.nix:1176-1281`; the builder checks
     out a computer's base commit from it, `internal/builder/request.go:38-50`).
     So this check needs one more read-only git command in a process that
     already runs git there. It needs no new host capability, no network
     fetch and no mirror. The worktree itself is never read, because it
     holds only the current deploy.
   - **A base the host does not hold** (never deployed, or the store
     lost it) refuses with `package_parent_unverifiable`. Nothing is
     listed on the publisher's word.
   - **The live store is checked, not assumed.** The code above shows how
     the store is made, not that the live one was made that way. S2's
     corpusd runs `git rev-parse --is-shallow-repository` on the store at
     start and reports the answer in its health detail, so FP0 reads it
     without SSH before S2's proof. A shallow store would refuse every
     base parent older than the deploy, which fails closed.
   - The internal/platform package has no git code today. The tree read
     lives in `internal/pkgrelease`, beside the subtree extraction that
     checks 1 and 2 already need.
4. **No secrets.** The secret scan runs over `tree_patch`.
5. **Manifest rules.** The manifest validates, and before S12 any
   `kinds` refuses with `package_kinds_need_platform` (first-party kinds
   are generated into `internal/apppackage/firstparty_gen.go`, outside
   the package).

**What a false publisher-side claim can do.** If a lying or buggy guest
claims that a mixed operation was confined, the release still carries
only NAME's subtree. The omitted files simply do not travel. What they
can do is make the package depend on a shell change the adopter lacks,
and the adopter's own build catches that (`build_failed`, 6.4), or
probation does (section 8). No path outside the root can ride a
release.

**A later upgrade, not required.** Bind operation confinement to the
owner's approval receipt. The `selfdev.approve` owner act names the
bundle digest, the bundle manifest names the patch sha, and a host holding
the stack base could re-derive the paths. That needs the publisher's
previous applied patches on the host, so it is recorded as a residual.

### 3.3 Changes outside a package

- **Never adopted, only proposed.** A change with any path outside one
  package directory is never adopted by another computer. It can be
  published only as a **platform proposal** (10.5), and it reaches other
  computers only when main merges it and the base image deploys.
- **Why.** The owner ruled that core changes are never adopted directly
  between diverged computers [D,
  `docs/reports/selfdev-update-architecture-2026-10-15.md:46-71`].
- **Mixed candidates are common today.** T1a's Reader redesign touched
  `app.css`, `read.html`, `reader.js`, `reader-chrome.css` and `theme.ts`
  beside its package [V, the T1a goal file's `now.candidate`].
- **Two steering moves:**
  - the cast names the package root as its confinement, and Verify
    checks the touched paths;
  - theming and shell hooks become `app-sdk.ts` exports, landed once
    through main.
- **When a candidate still mixes both:** publish the package part only if
  the package stands alone, and propose the rest to the platform.

### 3.4 Tracks, sequence numbers and history

- **A track is one publisher's line of a package.** Its id is the
  publisher's owner id.
- **First-party packages also have track `choir`, which is main.** Its
  "releases" are the package subtrees of deployed base commits. Nobody
  publishes to `choir` through the Library: it moves only through the
  exporter and the landing loop.
- **Each track has a linear history.** A new release's parent must be the
  current head: the latest listed release on the track that is not
  withdrawn, or the base subtree for a first release. A stale parent
  refuses with `track_moved`.
- **corpusd assigns `seq`** (1, 2, ...) per (package, track) when it
  lists a release. The sequence number belongs to the listing, not to the
  record.
- **A variant of a first-party package keeps its name.** Adopting it
  replaces the adopter's directory, so window ids, aliases and routes are
  unchanged.
- **A new app is a new package name** in the publisher's namespace (owner
  2026-10-24). The check refuses a name that collides with an existing
  one (P1-F3).

## 4. Privacy and publication

### 4.1 Scopes, and who may change them

| Scope | Who can see the listing, the record and the blobs | Who can adopt |
|---|---|---|
| `owner` (the default) | the publisher's owner and that owner's computers | the owner's computers |
| `shared` | the owner, plus named owner ids and named computer ids | the named owners' computers, **once S13 lands** |
| `platform` | the owner, plus platform admins (10.5) | nobody: a platform-scoped item reaches computers only through main |
| `public` | every authenticated owner on this cloud | everyone's computers, once S13 lands |

Scopes combine as a set. For example, `{owner, platform}` means the owner's
computers adopt it and platform admins can see it.

**Platform admins see `platform` and `public` items only.** An `owner`
release, or a `shared` one that does not name them, is invisible to a
platform admin, as it is to everyone else. The API key `admin` scope
grants no cross-owner reads (it is per user, and S1 pins this).

**Every scope change is an owner act.**

- It goes through the proxy, with an owner-act receipt
  (`internal/owneract`) whose subject binds `{release_id, scopes,
  shared_with}`.
- corpusd verifies the receipt before it writes the listing.
- No desk and no guest credential can widen a scope.

**A publish intent carried by the request.**

- The owner's self-development request may carry `publish: {scope:
  owner}`, and that is the owner's act.
- The intent is bound into the request's owner-act subject [H: whether
  the run-start and Texture request paths mint an owner-act receipt
  today; if they do not, S2 adds the act kind `request.submit` with the
  intent in its subject. It is settled by reading
  `internal/proxy`'s run-start handler].
- **Discharged once, for one operation** (review 257 F15). The runtime
  publishes the first operation on that request's trajectory that applies
  and passes probation, cites the receipt (verified through
  `owneract.Verified`), and records the discharge. A reworked or retried
  request does not publish again.
- **A desk can never add or change the intent.**
- **Default: only `owner` scope.** `shared`, `platform` and `public`
  need a direct owner act (owner question Q2).

**FP0 and coding agents** may perform a publish act with the owner's key
only under an explicit owner instruction, which is recorded in the act's
reason.

### 4.2 The listing

**Kind `choir.release_listing`**, in Store B, writer corpusd. There is one
object per record id, for both package releases and platform proposals
(field `record_kind`). It is versioned: each change is a new version that
names its predecessor.

```text
ReleaseListing {
  record_kind: package_release | platform_proposal
  record_id, package?, track, seq?
  scopes: [owner, shared, platform, public]
  shared_with: { owners: [...], computers: [...] }
  state: listed | withdrawn | suspended | yanked
  platform_state?: offered | proposed | exported | landed | declined     // 10.5
  state_reason, state_actor: owner_act:<receipt> | admin_act:<receipt> | host:probation | host:security:<advisory> | host:deploy
  version: <n>, prev_version_digest
  signature: <platform-control signature over the canonical listing>   // S11; unsigned before
}
```

Visibility and state live only in the listing, so the record never
changes.

### 4.3 Owner-scoped reads: S1, settled

S1 is built on `lane/lib-s1` (`38910e5de`, revised in `5200659a2` and
`b5de0b205`).

- **The code default is `owner`.** Only an explicit
  `CHOIR_RELEASE_READS=open` opens the reads, and that is the one-act
  rollback. Unset, empty or any other value means `owner`.
- **The admin scope has no cross-owner bypass.** The proxy always
  forwards the key's own owner. Host-internal callers that name no owner
  stay unscoped.
- **The routes:**

  | Route | What S1 does |
  |---|---|
  | `GET /api/releases` (proxy) | names the authenticated owner. A filtered list returns **200 with an empty list**, never 404. |
  | `GET /api/releases/<id>` | carries the owner. `computer_id=host` remains only as the host-internal sentinel. |
  | corpusd's single read (`/internal/platform/releases/<id>`) | requires the object's owner to equal the reader's owner. A guest's owner is the one vmctl binds its address to, never a parameter. An owner that cannot be resolved fails closed with 503. An unseen release answers **404** with the same body as a missing one. |
  | corpusd's blob read | answers only to an owner who published a release naming that blob. |

- **The blob leg is a unit test only.** There is no client blob route
  (`proxy/releases.go` serves only the list and single reads), and
  `objectgraph.ListFilter.OwnerID` already exists.
- **A named transitional break.** Under `owner`, the P8 cross-owner
  adoption reads answer 404 for another owner's release. Same-owner P8
  adoption is unaffected. (Correction, S3: S3's scopes are on package
  release listings only. `choir.software_release` has no listing, so its
  cross-owner reads stay 404 until S14 retires the kind.)
- **The rule for the new kinds** is the listing's scope set.

### 4.4 Evidence and private data

- **No publisher screenshots in any listing.** The adopter captures its
  own (6.4). Revision 1's same-owner screenshot reference is dropped,
  because another computer cannot resolve it (review 257 F14).
- **Widening shows what leaves.** A widening to `shared`, `platform` or
  `public` shows the full path list and the size of the review diff, and
  needs `--confirm-paths`. The secret scan runs again at widening.
- **Adopter outcomes** are visible to the adopter's owner. The publisher
  sees them only as counts across owners.

## 5. Discovery: the catalog

The catalog is a read-only view computed from the listings the caller can
see. It stores nothing of its own.

- **`GET /api/catalog/packages?q=&track=&limit=`** returns one row per
  (package, track):
  - the head release (seq, manifest version, title, icon and description);
  - the release count;
  - the last publish time;
  - the state.
- **`GET /api/catalog/packages/<name>/releases?track=`** returns one
  track's version history, newest first.
- **`GET /api/catalog/releases/<id>`** returns the record, verified
  against its id, together with its listing.
- **Search** is a substring match over the name, the manifest title and
  description, and the track's display name.
- **Base tracks.** For first-party packages the catalog also shows the
  `choir` track, read from the deployed base's manifests.

## 6. Adoption

### 6.1 Subscriptions, and the class floor at the proxy

**Kind `choir.package_subscription`**, on the adopter's tape. It is
registered `Writer: system:runtime`, written only on a verified owner-act
subscription request, the same pattern as `choir.computer_setting`
(`registry.go:109`; settings are booleans, `computer_setting.go:205`).

Why not the owner writer (review 265, finding 6, correcting revision 2,
which said there was none): `WriterOwner` exists
(`internal/objectgraph/writer.go:64`), but it is never a kind's writer.
A kind admits it only with `OwnerEdits` (`writerAdmits`, `writer.go:212`).
An owner edit of a subscription would come through the owner's edit path,
not through the proxy, so it would carry no proxy-signed `computer_tier`
and would skip the class floor below. So the kind is `WriterRuntime`
without `OwnerEdits`, and the guest refuses a write that has no receipt
(review 257 F15 and Devin 7).

```text
PackageSubscription { package: <name> | "*", track: <owner id> | "own", mode: auto | propose | pinned,
                      pin_release_id?, admit_owners: [<owner id>] (default: [own owner]),
                      owner_act: <receipt carrying computer_tier> }
```

**The class floor is enforced at the proxy** (review 257 F3). On a
subscription write:

1. The proxy reads the computer's class from vmctl
   (`LookupComputerContext`), the way `mintModeOwnerAct` does for the
   standing arm (`internal/proxy/self_development.go:273-295`).
2. It refuses a mode the class does not admit.
3. It signs `computer_tier` into the act's subject.

The guest stores the receipt and refuses a subscription without one.
Gate time re-checks through the standing arm, whose receipt also pins the
tier and expires.

| Class | Default | Highest mode the floor admits |
|---|---|---|
| `test` | `propose` until S9, then `auto` | `auto` (and auto also needs an armed `standing_consensus` mode) |
| `primary` | `propose` | `propose`: primary `auto` needs SR8 stage 4's own design (section 12) |
| `premium_always_on`, `critical_protected` (the owner's computers) | `propose`, with the adopter's screenshots | `propose` |
| `public_platform` (the paper) | `pinned` | `pinned`: adoption only by a manual owner act, last |

**A subscription is a standing intent.** A crash closes the occurrence
that is in flight, never the subscription. Two `interrupted` occurrences
of one (computer, release) close that release for that computer (8.5).

### 6.2 The tick and the offer

The runtime ticks every 5 minutes, and once after boot, once the crash
closures and the boot reconcile (8.5) have run.

1. It lists the visible listings, newer than its cursor, that match its
   subscriptions.
2. It coalesces them to the head release per (package, track).
3. It opens at most one adoption per package at a time. A newer head
   supersedes an open adoption that has not yet reached `applying`.
4. A `pinned` subscription is skipped. Ring soak (8.2) holds back offers
   to the classes above test.

### 6.3 The three-way tree check

Let L be the adopter's current package subtree, P the release's parent
subtree, and R the release subtree.

| Case | Action |
|---|---|
| L = R | `already_current` |
| L = P | fast-forward: build (6.4) |
| L equals an ancestor release's subtree on the same track | fast-forward to R. The skipped releases are named. |
| otherwise | `conflict`: an engineering work item (6.6). The adopter's tree is never overwritten. |

### 6.4 The deterministic adoption job

The job runs as a runtime-owned assignment of the new kind
`package_adopt`, and it makes **no model call**.

- **No desks.** It is not cast by management, and engineering's model
  does not run it.
- **An open seam.** [H: whether a capsule can be opened and frozen under
  a runtime-owned assignment. U4's `rebase` kind is the precedent for a
  new assignment kind. It is settled by reading the capsule executor's
  open and Freeze entry points in S5's first commit. If it fails, the
  fallback is a host-build-only job: the host builds the base plus the
  cumulative patch, as P5 layered applies do, and the visual capture
  runs after the apply in probation.]

The steps:

1. **Verify.** The existing seven rungs, plus:
   - the host-verified properties of 3.2, checked again;
   - the listing is `listed` and visible;
   - the subscription's receipt allows its mode.
2. **Place.** Replace the root in the capsule's effective source with
   R. Extract the subtree and compare its id.
3. **Build and check.** Run `npm run build` (which runs the package check
   and the boundary plugin), then the declared test command.
4. **Capture.** Take the adopter's own screenshots at the design bar's
   two widths.
5. **Freeze.** The bundle manifest records the lineage `{adoption_id,
   release_id, package_root}`. The projection commit carries the trailers
   `Choir-Package-Root` and `Choir-Package-Release` (S6 derives its entry
   from them).
6. **Gate** (6.5).

The Go binary is reused, because the stack is frontend-only (U1 step 3).

### 6.5 The gate

- **`propose`.** The candidate waits for approval on the ordinary path:
  `choir self-dev approve`, or Texture.
- **`auto`** (S9, test only). Standing consensus decides with its two
  seats.
  - **Seat one: the capsule-verifier seat.** It **requires a host build
    of this operation** (`standing_consensus.go:508-511`: no job means a
    refusal). So before S12, every auto adoption costs one host build
    (Go and frontend in one flake evaluation) and one unit of the
    builder's per-computer daily quota
    (`internal/hostbuild/service.go:59-62`) (review 257, Devin 3).
    - The cost is [H: the minutes per build; settled by S9's first
      measurement on T-B].
    - After S12 the build is one content-addressed module per release,
      shared by every adopter.
  - **Seat two: the independent seat.** For package adoptions it becomes
    a deterministic policy, `package-adopt-v1`. It passes when all of
    these hold:
    - the release is listed and visible;
    - the change is a fast-forward;
    - the build and the checks pass;
    - the captures are not blank.
- **Auto-apply is never auto-promote.**

### 6.6 Conflicts go to engineering

A `conflict` opens an engineering work item through the carried-patch
machinery.

1. The review patch P to R is offered at `/selfdev/carried/`.
2. L is the starting tree.
3. The result is an ordinary candidate, always `propose`.

## 7. Install and update: build time now, runtime later

| | Before S12 (`svelte-inline/v1`) | After S12 (`module/v1`) |
|---|---|---|
| What adoption changes | the adopter's source tree and its SPA build | the installed package set (a tape fact) |
| Who builds | the adopter's capsule. Auto mode also needs one host build per adoption (6.5). | the host, once per release [H: the build time; settled in S12] |
| Restart | the ordinary apply | none |
| Base move | the tree entry (S6) | untouched apart from the SDK range check |
| Kinds | refused (3.2) | registered at install |
| Same origin | yes (the owner's own approved code) | yes for the owner's own code; cross-owner code runs only in the S13 frame |

S12's ABI is a framework-free `mount(target, ctx)`:

- Svelte is bundled into the module;
- the SDK reaches the module only through `ctx.sdk`;
- the manifest's `requires_sdk` field is required;
- `app-sdk.ts` exports `SDK_VERSION`;
- out of range, the base package wins.

## 8. Probation, health signals, revert, suspend, withdraw, yank

### 8.1 Probation

Every applied adoption enters a 10-minute probation window. Its
precommitments are `update_healthy_within: 10m` and `no_restore_within:
7d`.

| Signal | Source | Fails when |
|---|---|---|
| health | `/health` returns 200, and the served build equals the target | either is false at the end of the window |
| crash | the boot kind | a markerless boot after the applied fact, inside the window (8.5) |
| package smoke | the host's visual capture of the package window, run after the apply | the capture is blank, shows a console error, or the window does not open |
| owner signal | the owner restores or reverts within 7 days | settles `no_restore_within` false; never triggers an automatic action |

The host runs the smoke check and records the result. A crashed guest
therefore cannot report a pass for itself.

### 8.2 Health signals across computers

corpusd records one `choir.package_outcome` (writer corpusd) per
(release, computer): `healthy`, `reverted` or `interrupted`. The outcomes
drive two things.

- **Ring soak orders the offers.** Releases are offered first to test
  computers. A release is offered to `primary` computers only after k = 2
  `healthy` outcomes on test computers, and to the owner's computers only
  after k = 2 on primary computers. Offers above test are always
  proposals.
- **Automatic suspension** (8.3).

### 8.3 Revert, suspend and withdraw

- **Forward revert, once.**
  - A failed probation applies the previous subtree as a new forward
    operation through the same job, under the `revert` rule: a return to
    a previously approved tree needs no new approval.
  - The tape is not rewound, so data written since the apply stays.
  - The revert happens at most once per adoption, and it is recorded
    before it acts.
  - A failed revert, or a crash during it, ends in `revert_failed_hold`
    and the owner is told. Nothing further runs automatically (LU's F19
    rule).
- **Suspend.** Two `reverted` outcomes on distinct computers set the
  release to `suspended` (actor `host:probation`), which stops new offers.
  Carriers are offered a revert, and the publisher's owner reinstates the
  release with an owner act.
- **Withdraw.** The publisher's act, from `listed` or `suspended`. The
  release leaves the catalog heads and the track head and stays in the
  history. (Correction, S3: it is not terminal. A reinstate lists it again
  while its parent is still the head of the track without it; once a
  release was published past it, a reinstate would fork the track and
  refuses `track_moved`.)
- **Protected computers.** On the owner's computers and the paper, a
  failed probation *proposes* the revert instead of applying it, until the
  owner rules (Q4).

### 8.4 Security yank (the one forced change)

The security authority (Q5) can yank a release. A yank is monotone and
host-side, so a restore of the adopter cannot undo it.

What a yank does to every carrier:

- sets the package aside, to the previous safe tree on its track or to
  the base tree;
- applies even under `pinned`;
- tells the owner in the Updates app;
- gives engineering a work item to rework any customization.

A yank never stops or contains a computer. That follows the owner's
order: security, then availability, then customizations.

### 8.5 Crash, planned restart and probation: the precedence and the boot reconcile

This section answers review 257 F5. The crash rule closes open *runs and
obligations* at a markerless boot. Adoption rows are a separate kind, so
the boot needs a stated reconcile.

**The boot reconcile.** It runs after the crash closures and before the
first tick. It is one bounded, indexed read of the computer's
non-terminal adoption rows (`objectgraph.ObjectsByKindOwnerComputerPageSQL`,
keyset-paged; never an `og_objects` scan). For each row it folds, in this
order:

1. **The applied fact.** Is there a `materialization_applied` event for
   the adoption's operation on the tape? This is the durable fact, and it
   wins over the row's own state.
2. **The assignment's fate.** The fate of the `package_adopt`
   assignment, for example "interrupted by a restart".
3. **The host's outcome.** The host-written `choir.package_outcome` for
   this (release, computer), read through the computer's capability.

**The precedence.** The rows are read top to bottom, and the first that
matches decides. Together they cover every non-terminal state of 15.2,
under both boot kinds, with the applied fact present or absent (review
265, finding 5, added the planned-restart rows with the fact present).

| Boot kind | The row was in | The applied fact | Result |
|---|---|---|---|
| any | any | present, and the host outcome is `healthy` or `reverted` | that outcome |
| any | `awaiting_approval` | absent | stays `awaiting_approval`: a durable decision, not work, as self-development candidates are today |
| any | `revert_proposed` | present | stays `revert_proposed`: a decision waiting for the owner (8.3) |
| any | `failed` | present | the one forward revert is opened as *new work* after the boot settles (on protected computers it is proposed, 8.3) |
| markerless | `reverting` | present | `revert_failed_hold` (8.3). Nothing further runs automatically. |
| marker present | `reverting` | present | **one resume** of the revert, because the marker is consumed first. If it crashes, the next boot is markerless, which gives the row above. |
| markerless (a crash) | any other state | present | **probation failed (crash)**. One forward revert is opened as *new work* after the boot settles, under the restart bounds. It is not a resume. If that revert crashes the computer, the next markerless boot finds `reverting` and ends in `revert_failed_hold`. |
| marker present (a planned restart) | any other state, for example `applying` or `probation` | present | **the probation window restarts once from the boot.** The fact wins over the row: an apply that landed before a planned restart, with its row write lost (the row still reads `applying`), enters probation as it would have. |
| any | `building`, with a terminal assignment or older than 60 minutes | absent | closed with the assignment's fate, or `interrupted (stale)` |
| markerless | `offered`, `building`, `frozen`, `admitted`, `approved` or `applying` | absent | `interrupted`. It counts one strike for (computer, release). The next tick may offer it once more, and a second `interrupted` closes the release for this computer. |
| marker present | `offered`, `building`, `frozen`, `admitted`, `approved` or `applying` | absent | **one resume**, because the marker is consumed first. If the resumed work crashes, the next boot is markerless, which gives the row above. |
| any | `probation`, `failed`, `reverting` or `revert_proposed` | absent | cannot arise, since each is written after the applied fact. The row closes `interrupted (inconsistent)` with no strike, and the owner is told. Nothing runs. |

The host projects its outcome onto the guest row through this reconcile
and the next tick. The host is never a second writer of the row: the row's
one writer is `system:runtime`.

## 9. Security

- **Same origin.** A package runs in the shell's origin with the owner's
  session, so only the owner's own approved code adopts same-origin.
  - Cross-owner adoption is refused (`cross_owner_before_frame`, at the
    S5 gate) until S13's opaque-origin `<iframe sandbox="allow-scripts">`
    lands, with a capability-scoped postMessage SDK bridge.
  - Before then, `shared` and `public` listings are for reading and
    review only.
- **The boundary checker is architectural, not a security boundary.**
  Authority stays in the backend.
- **Third-party imports (S13).** A third-party package may import its own
  files, the bundled Svelte, and npm dependencies from the shell's locked
  set. A new npm dependency is refused, because the lockfile is a recipe
  path (`internal/hostbuild/types.go:179-215`). The SDK reaches it only
  through `ctx.sdk`, and the runtime-reach token rules apply.
- **Signing.**
  - S11: corpusd signs every listing version with the platform-control
    key, and adopters verify the signature, the seq and the previous
    digest.
  - S13: TUF-style roles per namespace, for publishers outside this
    cloud. The custody of the root key is Q3.
- **Source only, and no binary crosses between computers.** Before S12
  the adopter builds its own SPA. From S12 the host builds the module.
- **The secret scan** runs at publish and at every widening.

## 10. Main, the platform base, and the support window

### 10.1 Main is the stable ring

- Main holds the `choir` track.
- The host holds no GitHub push credential (owner 2026-10-26).
- Main changes only through the existing reviewed human or coding-agent
  path, which the pull exporter (10.5) now automates up to the review.

### 10.2 Exporting a package release

`choir package export --release <id> --worktree DIR --lane NAME [--onto origin/main]`
is a client-side command, run by the exporter (10.5) or by hand.

1. Fetch the record and its blobs with the caller's own key, and verify
   them.
2. Refuse a package that is not first-party, unless the caller passes
   `--new-package`, for an owner's new app the owner chose to upstream.
   Third-party packages never go to main.
3. **Refuse if `--onto` has moved** (review 257, Devin 2). If `--onto`'s
   package subtree differs from the release's parent subtree P, refuse
   with `onto_moved`. Otherwise the export would silently revert a
   first-party change that main made after the release was cut. The
   publisher cuts a new release on the new base, or engineering merges
   the two.
4. Replace the package directory with R verbatim, and commit with these
   trailers:
   - `Choir-Package-Release`
   - `Choir-Package-Track`
   - `Choir-Publisher-Computer`
   - `Choir-Publisher-Operation`
   - `Choir-Tree`
5. Predict containment and print the PR body. Push nothing.

A CI check, `scripts/check-package-export`, runs on any commit with a
`Choir-Package-Release` trailer. It requires the package subtree to be
byte-identical to the named release subtree.

### 10.3 Base moves under adopted packages (before S12)

S6 adds package-aware answers, derived from the S5 trailers (6.4). They do
not need a new stack entry kind written by hand (review 257 F7).

**`upstreamed`.** The base's package subtree equals R. The stack entry
retires, and the lock entry's source becomes `base`.

- Today's containment test checks the *whole stack* (`contained.go`). A
  computer with other unpromoted entries therefore stays at
  `stack_base_moved` for those entries.
- S6 retires package entries one by one, by subtree equality.

**`package_base_moved`.** The base moved, and its subtree is still P (or
older). R re-materializes byte-identically onto the new base.

- **Self-development on other paths stays open.** This is a hard
  requirement (review 257 F7), not an [H]. The projection rebuilds on the
  new base with R's subtree in place. That tree is the approved content,
  so capsules cast on it. Only the *served* build waits for approval of
  the re-materialized candidate, because the owner said "not yet" to
  clean rebases without approval (Q1).
- A test pins this: a cast on another path is admitted while a
  re-materialization awaits approval.

**`package_behind_base`.** The base's subtree moved past P, and it is not
R. This is a conflict (6.6).

### 10.4 The one-week support window (one rule)

**The rule.** The support window is the set of platform bases deployed in
the last 7 days (owner ruling Q1). It is the only window in this design
(review 257 F12). It applies to two bases:

- a release's `built_on` base;
- the adopter's booted base.

**Inside the window.** An offer is made in the subscription's mode.

**Outside it.**

- If either base is outside, the offer is `propose`-only and flagged
  stale.
- If the adopter's base is outside, the adopter gets no new offers until
  it moves to a newer base.
- A security yank still reaches it.

**Retention.** Blobs are kept for at least 7 days after the last carrier
retires.

### 10.5 Proposals to the platform, and the pull exporter (owner, 2026-10-26 02:0x)

The owner asked: "we should have a way as admin to, from the library ...
Is it part of the library admin system to update the base platform
image?" The answer: yes. The Library is where a change is *proposed* for
the platform. The base image is updated only through the reviewed
landing loop, which the proposal feeds.

**Two record kinds can be proposed.**

1. **A package release** (3.1), already shared with scope `platform`.
2. **A platform proposal**, for a change outside packages: shell CSS, the
   window manager, Go.
   - Kind `choir.platform_proposal`, Store B, writer corpusd, immutable,
     schema `choir.platform_proposal.v1`.
   - Its fields are `{base_commit B, own_patch {sha256, bytes},
     changed_paths, publisher, operation receipts, precommitments}`.
   - **Its own change only.** The draft computes OP's own delta (the
     source before OP to the source after OP), applies it to the stack's
     platform base B, and stores the delta on B.
   - If the delta does not apply cleanly on B (it depends on an earlier
     change that is not yet upstreamed), the draft refuses with
     `proposal_depends_on_unupstreamed`, naming that operation, which must
     be proposed first.
   - This derivation is publisher-side. The review on the exported lane is
     the gate.
   - It is published at scope `owner` (`choir platform publish
     --computer C --operation OP`) and shared to `platform` by an owner
     act.
   - **Never adopted.** Subscriptions and the adoption job refuse this
     record kind.

**The states** (`platform_state` on the listing):

```text
(owner shares to platform)  -> offered      owner act
offered -> proposed                         admin act "propose for the platform" (a platform admin)
offered|proposed -> declined                admin act, with a reason
proposed -> exported                        the exporter records the lane and commit (admin-scoped key)
proposed|exported -> landed                 host, at deploy: the new base commit contains the record (containment)
landed is terminal; withdrawn/yanked on the listing also stop the exporter
```

**Platform admins.**

- A platform admin is a user id in the host list `CHOIR_PLATFORM_ADMINS`.
  The default is the owner's account only.
- FP0 acts as platform admin with a key the owner mints (Q7).
- An admin act is an owner-act receipt with the act kind
  `platform.propose` or `platform.decline`, minted by the proxy only for a
  platform admin.

**The pull exporter.** It is pull-based, and it runs anywhere except on
the host's credentials.

- **Where it runs.** It is either a coding agent (FP0) running
  `choir platform proposals` and `choir platform export`, or a scheduled
  GitHub Actions workflow, `library-export.yml`.
  - The workflow uses a read-only proposals key from Actions secrets, and
    the workflow's own `GITHUB_TOKEN` to push `lane/proposal-<id>` and
    open a PR.
  - It is never a push credential on Node B.
- **What it does for each `proposed` item:**
  - **a package release:** the 10.2 export, including the `onto_moved`
    refusal;
  - **a platform proposal:** applies `own_patch` onto `--onto`. It
    refuses if the patch does not apply cleanly (it never merges), and it
    runs `selfdev-promote`'s containment prediction. It commits with the
    trailers `Choir-Platform-Proposal`, `Choir-Publisher-Computer` and
    `Choir-Publisher-Operation`, and records `exported`.
- **What then happens:**
  - review by the tier of the change's class: tier 1 for a package, and
    the class's own tier for a platform change;
  - the merge, by FP0 or the owner;
  - the landing loop;
  - the deploy updates the base image for every computer;
  - every carrier reports `upstreamed`, by whole-stack containment for a
    platform proposal (so a computer with other unpromoted entries
    retires only the contained ones);
  - corpusd marks the item `landed`.
- **What this retires.** It retires `scripts/selfdev-promote` as FP0's
  hand step. FP0's remaining job is the review and the merge.

## 11. The Updates app

The Updates app is a first-party package, `updates`, and it is read-only,
per the owner's rulings of 2026-10-24.

**What it lists:**

- platform deploys;
- package adoptions, with the adopter's screenshots;
- reverts, suspensions, withdrawals, yanks and set-asides;
- the platform proposals the owner made, with their `platform_state`;
- pending proposals, as links to the approval surface. The app itself
  has no approve button.

**Notices:**

- on opening the desktop, the updates since it was last opened;
- on opening an app, the updates to that app since it was last seen.

"Last seen" is the kind `choir.updates_seen`, registered `Writer:
system:runtime` with `OwnerEdits: true` and `IdentityExternalKey` (the
form `package_kinds.go:254` requires).

**The API** is `GET /api/computers/<id>/updates`, a projection over tape
facts that stores nothing new. Desks read it through a read-only verb.

## 12. Protected computers

The paper (`public_platform`) and the owner's computers
(`premium_always_on`, `critical_protected`) adopt last, and never
automatically at first.

- **The class floor** (6.1) is enforced at the proxy and again by the
  test-tier refusal of standing consensus.
- **Ring soak** (8.2) orders their proposals after test and primary.
- **Reverts are proposed** on these computers, not applied (8.3).
- **Never used for proof steps.**
- **The paper adopts** only by a manual owner act, after the owner's
  computers.
- **A security yank still reaches them**, as a set-aside, never a stop.
- **Busy computers.** Adoption defers to idle, and the always-busy
  planned-refresh exception does not extend to package adoptions.
- **Primary `auto` is out of scope** (review 257 F4). It would need a
  design that changes standing consensus's test-tier gate for package
  adoptions: SR8 stage 4, as its own station. Revision 1 listed it in
  S14; it is removed.

## 13. Data model

| Kind | Store | Registration (writer, identity) | Mutable | Purpose |
|---|---|---|---|---|
| `choir.package_release` | Store B | `WriterCorpusd`, external key = release id | no | the adoption unit (3.1) |
| `choir.platform_proposal` | Store B | `WriterCorpusd`, external key = proposal id | no | a change outside packages, proposed for main (10.5) |
| `choir.release_listing` | Store B | `WriterCorpusd`, external key = record id | versioned | scopes, state, platform_state (4.2) |
| `choir.package_outcome` | Store B | `WriterCorpusd`, external key = (release id, computer id) | no | health signals (8.2) |
| `choir.package_subscription` | guest tape | `WriterRuntime`, external key = (package, track); written only on a verified owner act | versioned | standing intent (6.1) |
| `choir.package_adoption` | guest tape | `WriterRuntime`, external key = digest(computer, release) | state machine | one adoption (15.2) |
| `choir.updates_seen` | guest tape | `WriterRuntime` with `OwnerEdits: true`, `IdentityExternalKey` | versioned | the notice cursor (11) |
| package lock | derived, not stored | — | — | `/health` `effective_release.packages`, derived from the base manifests, the applied adoptions and the S5 trailers. One state authority: the tape. |

- **Retired in S14:** `choir.software_release` and `choir.release_adoption`
  (readable, with no writer).
- **Each kind-adding step carries a skip test** (review 257 F10). Rows of
  the new kind, replayed under the previous release's registry, project
  as inert rows, and boot succeeds (`projectObject` accepts any kind,
  `internal/store/project.go:231-271`).
- **Until SR7's cross-version replay gate lands,** this per-step test is
  the dependency. Once it lands, SR7's gate supersedes these tests.

## 14. APIs and CLI verbs

**Owner-facing routes, through the proxy.** Every write is an owner act
unless marked otherwise.

| Route | Verb | Act |
|---|---|---|
| `POST /api/computers/<id>/packages/releases` | `choir package publish` | owner act. Creates a listing at scope `owner`. |
| `POST /api/computers/<id>/platform/proposals` | `choir platform publish` | owner act. Creates a listing at scope `owner`. |
| `PATCH /api/catalog/records/<id>/listing` `{scopes, shared_with, confirm_paths}` or `{state}` | `choir package share`, `choir package withdraw`, `choir package reinstate` (and `choir platform share` for proposals) | owner act |
| `POST /api/platform/proposals/<id>/propose` and `.../decline` | `choir platform propose`, `choir platform decline` | admin act (a platform admin) |
| `GET /api/platform/proposals?state=` | `choir platform proposals` | read; platform admins and the records' owners |
| `POST /api/platform/proposals/<id>/exported` `{lane, commit}` | (the exporter) | admin-scoped key |
| `GET /api/catalog/packages`, `.../packages/<name>/releases`, `.../releases/<id>` | `choir package search`, `choir package history`, `choir package show` | read, scoped (4.3) |
| `PUT` and `DELETE /api/computers/<id>/packages/subscriptions/<package>` | `choir package subscribe`, `choir package unsubscribe` | owner act. The proxy enforces the class floor and signs the tier. |
| `GET /api/computers/<id>/packages/{subscriptions,lock,adoptions[/<id>]}` | `choir package subscriptions`, `choir package lock`, `choir package adoptions` | read |
| `POST /api/computers/<id>/packages/adoptions` `{release_id}` | `choir package adopt` | owner act |
| `POST /api/computers/<id>/packages/adoptions/<id>/revert` | `choir package revert` | owner act |
| `GET /api/computers/<id>/updates` | `choir updates` | read |
| (client only) | `choir package export`, `choir platform export` | none: local git; nothing is pushed |

**corpusd routes, host only:** `POST /internal/platform/packages/releases`,
`POST /internal/platform/platform/proposals`,
`PUT /internal/platform/listings/<id>`,
`POST /internal/platform/packages/outcomes`, and
`POST /internal/platform/packages/yank` (S11).

**Approvals.** A frozen adoption is approved with the existing
`choir self-dev approve` and `reject`.

## 15. States and transitions

### 15.1 Listing

```text
(publish)      -> listed{owner}
listed(S)      -> listed(S')          owner act
listed         -> withdrawn           owner act (publisher)
listed         -> suspended           host: 2 reverted outcomes on distinct computers
suspended      -> listed              owner act (reinstate)
listed|suspended -> withdrawn         owner act (withdraw)
withdrawn      -> listed              owner act (reinstate), only while its parent heads the track without it (S3)
listed|suspended|withdrawn -> yanked  host security authority; forces set-aside
platform_state: offered -> proposed -> exported -> landed | declined   (10.5)
```

### 15.2 Adoption (the adopter's tape)

```text
offered -> skipped(pinned) | already_current | refused(<reason>) | conflict -> engineering_item
offered -> building -> frozen -> awaiting_approval (propose) | admitted (auto)
awaiting_approval -> rejected | approved
approved|admitted -> applying -> probation -> healthy
probation -> failed -> reverting -> reverted | revert_failed_hold
probation -> failed -> revert_proposed (protected classes)
offered|building|frozen|admitted|approved|applying -> interrupted (crash before the applied fact; 8.5)
any not yet applying -> superseded
healthy -> upstreamed | set_aside (yank) | replaced
```

Refusal reasons: `not_visible`, `not_listed`, `digest_mismatch`,
`blob_mismatch`, `content_not_confined`, `secret_found`,
`class_mode_refused`, `outside_support_window`, `build_failed`,
`checks_failed`, `capture_blank`, `cross_owner_before_frame`,
`record_kind_not_adoptable`.

### 15.3 Lock entry

```text
base -> adopted(R) -> upstreamed (base) | reverted (previous) | set_aside (security) | adopted(R')
adopted(R) -> package_base_moved -> adopted(R) re-materialized (approval, Q1) | conflict (package_behind_base)
```

## 16. Failure modes, written before code

| # | Failure | Stop | Proof |
|---|---|---|---|
| L1 | A release carries the publisher's other changes | Only the root subtree is carried, and the parent must be the base or the track head | unit test with a two-operation stack; deployed on T-A in S2 |
| L2 | **A release's content reaches outside the root** (a guest-forged blob) | corpusd applies `tree_patch` to the empty tree and refuses any path outside the root, then compares the subtree id (host-verified, 3.2) | unit test: a forged blob with `frontend/src/app.css` |
| L2b | **A guest falsely claims that a mixed operation was confined** | Not detectable on the host (it holds no publisher repository). Nothing outside the root travels anyway, and a package that depends on a missing shell change fails the adopter's build or probation. Stated as publisher-side. | S5 test: a package that imports a missing SDK export fails with `build_failed` |
| L3 | A symlink, submodule or mode trick | refused on the host at publish, and again at place | fixtures |
| L4 | A listing or record is visible to another owner, or to an admin, without scope | owner and scope filtering; a single read answers 404; a list answers 200 with nothing | S1 (built), S3, S4 (the platform scope) |
| L5 | A replayed old listing | versioned listings, signed from S11 | S11 |
| L6 | A desk widens a scope or publishes | owner-act receipts; the request intent is read from the receipt and bound to one operation at discharge | test |
| L7 | A model-mediated step fails | the job makes zero model calls | provider-call counter = 0 |
| L8 | The adopter's own changes are overwritten | the three-way check gives `conflict` | test, and the S5 leg |
| L9 | A crash mid-adoption resumes | 8.5's precedence: `interrupted` before the applied fact; two strikes close the release | tests for each row of 8.5 |
| L9b | A crash after the applied fact leaves a broken package | the applied fact wins; probation fails on the crash; one forward revert as new work | test: kill after `materialization_applied`, before the row write; the same with the planned-restart marker present, where probation restarts |
| L9c | A `building` row with no worker | closed by the assignment's fate, or by the 60-minute stale rule | test |
| L10 | A revert loop | one revert, then a hold | test |
| L11 | Probation passes but the app is broken | the host smoke check | S7 leg |
| L12 | Probation fails falsely | the cost is one reversible revert; outcomes are recorded | weak measure |
| L13 | Auto on a non-test computer | the class floor at the proxy subscription write, plus the test-tier gate of standing consensus | tests across all five classes |
| L14 | Same-origin code across owners | `cross_owner_before_frame` at the S5 gate | S5 test; S13 lifts it |
| L15 | A mixed candidate is published as a package | the publisher-side refusal names the paths and points to `choir platform publish` | S2 leg |
| L16 | An export reshapes the tree | verbatim write plus the CI check | S4 |
| L16b | **An export reverts a later change on main** | `onto_moved` when `--onto`'s subtree differs from P | S4 test |
| L17 | Base moves strand adopters | S6's re-materialization keeps self-development open; S12 removes the cause | S6 leg |
| L18 | A base SDK change breaks a package | before S12, the build fails; after S12, `requires_sdk` | S5, S12 |
| L19 | A subscription storm, or host-build exhaustion on the auto path | coalescing, one adoption open per package, the 5-minute tick, and the builder's per-computer daily quota (auto adoptions count against it; S9 measures) | test, and S9's measurement |
| L20 | Store B is unavailable | the tick ends with nothing applied | test |
| L21 | Private data leaves in a widening | no publisher screenshots; `--confirm-paths`; the scan runs again | S3 test |
| L22 | A yank is undone by a restore | yank state is host-side, and the gate reads it. The restore runs on the current binary, so SR7 is not involved. | S11 |
| L23 | A parent is a withdrawn release | the track head excludes withdrawn releases | test |
| L24 | Two tracks compete for one package on one computer | one track per package per computer | test |
| L25 | A package declares kinds before S12 | `package_kinds_need_platform` | test |
| L26 | Adoption on a busy owner's computer | deferred to idle | test |
| L27 | The blob cap is too small | the refusal names the size; the cap is raised only after S2 measures | S2 measurement |
| L28 | A platform proposal is adopted directly | subscriptions and the job refuse `record_kind_not_adoptable` | test |
| L29 | A platform proposal carries an earlier, un-upstreamed change | the own delta must apply cleanly on B, or `proposal_depends_on_unupstreamed` | test |
| L30 | A non-admin proposes for the platform, or an admin reads a private record | the proxy mints `platform.propose` only for `CHOIR_PLATFORM_ADMINS`; scope filtering | S4 test |
| L31 | The exporter holds more authority than it should | a read-only proposals key, plus a write only of `exported`; the GitHub token is the workflow's own; nothing on Node B | S4 review of the workflow |
| L32 | Retiring the old path breaks the only live path | nothing is retired before S14 | review 257 F2 |
| L33 | A new kind poisons an older binary | each step's skip test (13) | per step |

## 17. Build plan

**How every step lands:**

- alone, in a lane on current main;
- tests first, then the cheap checks;
- reviewed at its tier, then through the landing loop;
- behind a default-off switch, set by FP0 (owner-delegated). Host
  switches go in `nix/node-b.nix`; guest switches are per-computer
  settings.

**Kind-adding steps** also carry the skip test (13).

**Test computers.** Section 18 names T-A, T-B, T-C and T-D. Every proof
runs on choir.news.

### S0. Problem records first (green, tier 0)

**Scope:** problem records for:

- unscoped release reads;
- the cumulative release;
- model-mediated adoption;
- candidates that cross package boundaries.

**Proof:** docs only.

### S1. Owner-scoped release reads (red, tier 1). Built: `lane/lib-s1` `b5de0b205`

**Scope:** as in section 4.3, already built:

- the default is `owner`;
- `open` is only an explicit env rollback;
- admin has no bypass;
- host-internal callers stay unscoped;
- the P8 cross-owner break is named;
- a filtered list returns 200 with nothing;
- the blob check is a unit test.

**Tests:** in the lane, with `TestAdminKeyReadsOnlyItsOwnersReleases`
among them.

**Rollback:** `CHOIR_RELEASE_READS=open`, then restart corpusd.

**Proof:**

- T-A's releases list with b@c's key;
- d@e's key lists none of them (200, empty) and gets 404 on a known id;
- T-B, minted for this step, reads them through its own capability.

T-B joins `test_computers` only through the enabling change of section
18, and only once its precondition holds: T-B is named by computer id in
`CHOIR_VISUAL_CAPTURE_COMPUTERS`, so `test_class_idle` keeps it.

### S2. Package releases and owner-scope publish (red, tier 2)

**Scope:**

- `internal/pkgrelease/`, new: the record, canonical form, the host checks
  of 3.2, and subtree extraction;
- the base-parent check of 3.2 check 3: the `parent_patch` blob, and the
  read-only tree read (`git rev-parse <commit>:<root>`) from the deploy
  checkout's object store that the host build already uses, refusing
  `package_parent_unverifiable` when the base is not there, and the
  store's shallow flag in corpusd's health detail;
- `internal/platform/packages.go`, new: publish and listing, at scope
  `owner` only;
- the `package` content namespace;
- the kinds `choir.package_release` and `choir.release_listing`;
- the guest draft, `internal/agentcore/api_package_release.go`, with the
  publisher-side checks;
- `internal/proxy/packages.go`;
- `cmd/choir/package.go` (`publish`, `show`, `history`, `search`);
- the owner-act `request.submit` intent, if the [H] in 4.1 needs it;
- the setting `packages.publish`.

`packages.publish` is a rollout switch, not a security control (review
274). The guest enforces it: with the setting off, the guest refuses to
draft (`packages_publish_off`), so a stock computer sends nothing to
publish. Neither the proxy nor corpusd can read it, because it lives in the
computer's own settings. A compromised guest can still draft with the
setting off, and an owner act would then publish it. The controls that
hold against that guest are the owner act and the host checks of 3.2.

**Tests first:** L1, L2, L3, L6, L15, L23, L25, L27, `track_moved`, a
`parent_patch` whose tree differs from the host's base tree (refused), a
base commit the store does not hold (refused with
`package_parent_unverifiable`), and the skip test.

**Rollback:** the setting off, or a revert. Rows stay inert.

**Proof:**

- T-A publishes a package-confined applied operation;
- `show` verifies it;
- a mixed operation is refused, with its paths named;
- d@e's key sees nothing;
- the blob sizes are measured.

### S3. Sharing scopes, withdraw and reinstate (red, tier 2)

**Scope:**

- listing versions;
- `share`, `withdraw` and `reinstate`;
- `--confirm-paths`;
- the 404 and 200-empty rules for every scope.

The cross-owner adoption refusal moves to S5, where the gate exists
(Devin 5).

**Tests first:** L4 across scopes, L21, L24, and the history.

**Rollback:** `packages.share` off. Reads fall back to `owner`.

**Never git-revert S3** (review 291 F3). Roll back with `packages.share`
off on the computers and `CHOIR_PACKAGE_SHARE_READS=owner` on corpusd
(then restart corpusd). A revert to S2 is worse than either:

- S2 counts every listing object of a track, the versions it drops as
  invalid included, against its 1000-listing cap, so a track with that many
  answers `track_full` on every read and every publish;
- S2's listing check refuses a widened version, so its chain stops at the
  last owner-only version: a release withdrawn after being shared reads as
  `listed` again and can head the track;
- a publish under S2 ignores a shared head (it does not see that version)
  and can take a seq already given, forking the track.

**Proof:**

- the release shared with d@e is visible to d@e's key, and not to a key
  of a third account;
- after a withdraw it leaves the catalog heads, and its history shows
  `withdrawn`.

**As built** (lane `lane/lib-s3`; corrections to this design, found
building it):

- **The rollback is two switches.** `packages.share` is a guest setting,
  which neither the proxy nor corpusd can read. It gates the acts: the
  proxy asks the named computer's `GET .../packages/share-gate` before
  every share, withdraw or reinstate, as publish asks for the draft. The
  read side's fallback is corpusd's `CHOIR_PACKAGE_SHARE_READS=owner`
  (unset, empty or `scoped` honours each listing's scopes; any other value
  is `owner`). With `packages.share` off everywhere, the default, nothing
  is widened, so reads are owner scope by construction. A git revert is
  never the rollback (the note under **Rollback** above names why).
- **Index-backed reads** (review 291 F1, F2). The listing's `widened`
  field and the release's `release_id` are registered lookup keys
  (`og_lookup_keys`, backfilled online at corpusd start), so another
  owner's catalog pages the widened listing versions by the key's primary
  key, and a foreign single read seeks the release by id. With nothing
  widened, a catalog reads one empty key page. Until the backfill switches
  the keys on, a catalog shows no other owner's track and a foreign read is
  404: the narrower answer, never a scan. A catalog still reads every
  widened version on the cloud, through the index; a per-reader index is
  the next step if sharing grows.
- **A receipt acts once** (review 291 F4): a receipt already recorded as
  a version's `state_actor` refuses `owner_act_replayed`. A scope change
  keeps `state_reason`, which says why the release is in its state.
- **`--confirm-paths` names a digest**, the `paths_digest` that the
  widening's `confirm_paths_required` refusal shows beside every path and
  the review diff's size. It binds the release id and the paths, so a
  confirmation never carries over to another release.
- **Not built in S3, refused by name** (`scope_not_built`): the `platform`
  scope (S4), and `shared_with.computers`, since no computer reads
  packages before S5. Sharing names owners.
- **The act's gate computer** is any of the owner's computers named by
  `--computer`, so a withdraw still works when the publishing computer is
  gone.
- **L24 in S3** is the read half: two owners' tracks of one package never
  mix in a catalog row, a history (one track; the reader's own unless
  `track` names another) or the track head a publish drafts on, and a
  shared release is never a parent. The per-computer half (one track per
  package per computer) is S5's subscription rule.
- **Tracks page, never cap** (review residual bunch279-I2): every track
  read pages to its end by canonical id, a publish past the old
  1000-listing cap lists, and history pages newest first by `before_seq`.
  No store schema change was needed.
- **The catalog names the reader** (`"reader"`: the caller's own owner
  id), so an owner can tell another which id to share with.

### S4. Export, the platform scope, "propose for the platform", and the pull exporter, for package releases (red, tier 2: a new visibility scope and admin act)

**Scope:**

- the `platform` scope and `CHOIR_PLATFORM_ADMINS`;
- the act kinds `platform.propose` and `platform.decline`;
- the routes `/api/platform/proposals`;
- `choir package export` and `choir platform proposals|propose|decline|export`;
- `scripts/check-package-export`;
- `library-export.yml` (scheduled, PR-opening; uses the workflow's own
  token and a read-only proposals key);
- the deploy-time `landed` mark.

**Tests first:**

- L16, L16b (`onto_moved`), L30, L31;
- third-party packages refused;
- `--new-package`;
- the one-byte tree check;
- an admin cannot see a private record.

**Rollback:**

- the `platform` scope off (host env `CHOIR_PLATFORM_SCOPE=off`);
- the workflow disabled;
- or a revert.

**Proof:**

- T-A's release, shared to `platform`, is proposed by the admin act;
- the exporter, as FP0 first and then the workflow once Q7's key exists,
  opens the lane, and the check passes;
- the lane is reviewed at tier 1 and lands;
- T-A reports `upstreamed` and corpusd marks the release `landed`.

**Precondition** (review 257 F11): T-A's stack holds only this
operation. Today's containment is whole-stack, so FP0 reads T-A's
projection first. The general case is S6.

### S4b. Platform proposals for changes outside packages (red, tier 2)

**Scope:**

- the kind `choir.platform_proposal`;
- `choir platform publish`, with the own-delta derivation;
- the exporter's patch path (selfdev-promote's containment prediction,
  ported);
- `record_kind_not_adoptable`.

**Tests first:** L28, L29, a refusal that never merges, and the skip
test.

**Rollback:** the setting `platform.proposals` off, or a revert.

**Proof:**

- T-A's next shell or CSS change (for example the T1a app.css part) is
  published, shared to `platform`, proposed, exported, reviewed and
  landed;
- every computer's next refresh carries it, and T-A reports `upstreamed`.

`scripts/selfdev-promote` is no longer FP0's hand step after this.

### S5. Subscriptions, the tick, and the deterministic adoption job, propose mode (red, tier 2)

**Scope:**

- the kinds `choir.package_subscription` and `choir.package_adoption`;
- `internal/agentcore/package_adoption.go` (the tick, the three-way check,
  `package_adopt` and its recipe, the 8.5 boot reconcile);
- the capsule seam (the [H] in 6.4);
- the proxy subscription write with the class floor and the signed tier;
- the CLI;
- `cross_owner_before_frame`;
- the setting `packages.adopt`.

**Nothing old is retired here.** P8 keeps working until S14 (review 257
F2).

**Tests first:**

- L2b, L7, L8, L9, L9b, L9c, L13 (the proxy half), L14, L18, L19, L20, L26,
  L28;
- `already_current`, a multi-release fast-forward, and `superseded`;
- off is inert;
- the skip test.

**Rollback:** `packages.adopt` off. The tick stops, open adoptions close
`interrupted`, and applied adoptions stay ordinary applied operations.

**Proof:**

- T-B subscribes to `reader` on T-A's track in propose mode;
- after T-A publishes, with no adopt command, T-B shows an adoption that
  is awaiting approval, with T-B's own screenshots, and the provider-call
  counter reads 0;
- FP0 approves, T-B serves R, and the lock names R;
- the negative legs:
  - a `pinned` subscription adopts nothing;
  - a local edit on T-B gives `conflict`;
  - a tampered blob is refused;
  - an `auto` subscription on T-D (if it is primary class) is refused at
    the proxy;
  - an adoption of d@e's shared release is refused
    `cross_owner_before_frame`.

### S6. The package lock and package-aware containment (red, tier 2)

**Scope:**

- `internal/sourceproj`: package entries derived from the S5 trailers;
  `upstreamed`, `package_base_moved` and `package_behind_base` per
  package; casts admitted during a re-materialization (a hard
  requirement);
- `/health` `effective_release.packages`;
- the vmctl list column.

**Tests first:**

- L17;
- the byte-identical re-materialization;
- a cast on another path is admitted while approval is pending;
- `package_behind_base` gives a conflict;
- a crash between the apply and the projection is reconciled;
- per-entry retirement with another unpromoted entry present.

**Rollback:** `packages.lock` off, which brings back today's behaviour.

**Proof:**

- with T-B carrying R, a deploy lands;
- T-B reports `package_base_moved`, and a cast on another path succeeds;
- the re-materialization is approved;
- the S4 export lands, and T-A and T-B report `upstreamed`.

### S7. Probation, the smoke check, outcomes and forward revert (red, tier 2)

**Scope:**

- the probation precommitment kinds;
- the host smoke check, and `choir.package_outcome`;
- the `revert` rule;
- suspension;
- proposed reverts on protected computers;
- the setting `packages.probation`.

**Tests first:**

- L9b, L10, L11;
- one revert;
- `revert_failed_hold`;
- suspension needs distinct computers;
- the tape is not rewound;
- the skip test.

**Rollback:** the setting off.

**Proof:**

- T-A publishes a `reader` release that throws on mount;
- T-B and T-C both adopt it (propose, approved), and both fail the smoke
  check and revert once;
- two `reverted` outcomes suspend the release.

T-C exists for this leg. A computer adopting its own release is not used
(review 257 F9, Devin 6).

### S8. The Updates app (orange, tier 1)

**Scope:**

- `frontend/src/packages/updates/`;
- `GET /api/computers/<id>/updates`;
- `choir.updates_seen`;
- the notices;
- `LIBRARY_NAME`;
- the desk read verb.

**Tests first:**

- every update type appears once;
- the app has no write route;
- notices fire once;
- desks read the same list;
- the skip test.

**Rollback:** a revert.

**Proof:**

- T-B lists the S5 adoption, the S6 re-materialization, the S7 revert,
  the deploy, and T-A's platform proposal state;
- reopening the desktop shows only what is new.

### S9. Auto mode on test computers (red, tier 2)

**Scope:**

- the `package-adopt-v1` seat;
- the host-build seat kept and costed, with auto adoptions submitting the
  host build;
- ring soak on offers;
- test-class default `auto`.

**Tests first:**

- L13 (the gate half), L19 (quota);
- a non-fast-forward is never auto;
- dissent leaves the adoption awaiting approval;
- a primary-class adopter gets a proposal only.

**Rollback:** `packages.auto` off.

**Proof:**

- T-B on `auto`, with an armed standing mode, adopts T-A's next release
  with no human act and passes probation;
- the host-build wall time and the quota use are recorded;
- T-D, if primary class, gets the same release only as a proposal after
  the soak.

### S10. The support window (orange, tier 1)

**Scope:** the one rule of 10.4, and retention.

**Tests first:** the 7-day boundaries; base deploy times [H: the source;
settled by reading the platform release record].

**Rollback:** the setting off.

**Proof:** using a test-only clock seam, a release whose `built_on` is
outside the window is offered to T-B as a proposal only.

### S11. Signed listings and the security yank (red, tier 2)

**Scope:**

- the listing signatures;
- the yank route;
- the forced set-aside, with an Updates entry and an engineering item.

**Tests first:**

- L5, L22;
- the computer is never stopped;
- `pinned` does not block a yank;
- a yank reaches an adopter outside the window.

**Rollback:** the signature check off. The yank route stays unused.

**Proof:** a yank of T-A's release sets T-B's `reader` aside within one
tick, and a restore of T-B to a pre-yank head re-asserts the set-aside.

### S12. Runtime install (P5) (red, tier 2)

**Scope:**

- the `module/v1` ABI and its alias;
- the package-only host build;
- `requires_sdk` and `SDK_VERSION`;
- the rebased S7-0 reducer, build store and loader;
- kind registration at install;
- packages leave the source stack.

**Tests first:**

- P5's rows;
- L18 (after S12);
- install, restore and reinstall;
- the SDK fallback;
- bytes verified before they are mounted;
- the skip test.

**Rollback:** `packages.runtime_install` off, which falls back to the S5
build path.

**Proof:**

- T-B installs T-A's release with no build and no restart;
- a deploy then leaves it untouched;
- a release that declares a kind writes it.

### S13. Cross-owner adoption: the frame and namespace signing (red, tier 2)

**Scope:**

- the frame and its bridge;
- the third-party import allowlist;
- the TUF-style roles;
- lifting `cross_owner_before_frame` for framed packages.

**Tests first:**

- no cookie and no `/api` reach from inside the frame;
- an ungranted capability is refused;
- a namespace key cannot sign another namespace;
- an expired timestamp is refused.

**Rollback:** the setting off.

**Proof:**

- T-D (d@e) adopts T-A's shared release inside the frame;
- an attempt to read the session from inside the frame fails.

### S14. Widening as proposals, and deletion (orange and red per item, tier 1 each)

**Scope:**

- **Widening.** Ring-soaked proposals reach primary computers and the
  owner's computers. The paper adopts only by a manual act, last.
- **Deletions, each after a citer search:**
  - P8's desk-prompt apply (`adoptionRequestText`), the
    `software_release` publish route, and `choir release publish|adopt`.
    Both kinds are retired.
  - `scripts/selfdev-promote`.
  - M9a's dark path.
  - The unused `platformrelease` fields.

**Tests first:**

- the class table end to end;
- no citer remains for any deletion;
- the new loop proved in S9.

**Rollback:** per item, a revert.

**Proof:**

- a release goes from test (auto) to primary and the owner's computers
  (as proposals);
- the paper is not touched until the owner acts.

### Order and parallelism

1. S0, then S1 (built).
2. S2.
3. S3, S4 and S4b in parallel after S2.
4. S5 after S2.
5. S6 after S5.
6. S7 after S5.
7. S8 can start on S5's data.
8. S9 after S7.
9. S10 and S11 after S5.
10. S12 after S6, in parallel with S9 to S11.
11. S13 after S12.
12. S14 last.

**The first loop without FP0 hand work** is S2, S4 and S5, about a day of
lanes. **The whole system** takes several days, and S12 and S13 are the
long poles.

## 18. The test computers, and the enabling change

**What the code says about class** [V, review 257 F3]:

- vmctl derives a computer's class in `warmnessClassForOwnership`
  (`internal/vmctl/warmness_policy.go:120-133`);
- a `test` computer belongs to an ephemeral account (domains
  `example.com` and `example.test`, `nix/node-b.nix:1873`), or is listed
  in `internal/computertiers/computer-tiers.json` under `test_computers`;
- a paid account's computer is `premium_always_on`, and the only paid
  account is the owner's.

b@c and d@e are test accounts the owner made, but their domains are not
ephemeral and `test_computers` is empty. So **their computers read as
`primary` by the code.**

- FP0 confirms the live class (`warmness_class` in the compute status)
  before S5.
- If confirmed, standing consensus and `auto` refuse them today, and
  every "test" leg in this design needs the enabling change below.

**The enabling change** (orange, tier 1; it lands with S1's deploy wave or
just after T-B is minted):

- add T-A, T-B and T-C to `test_computers` in `computer-tiers.json`, each
  with a note, by computer id;
- never list T-D. It stays `primary` for the S5 and S9 primary legs.
- never make b@c or d@e a test *account*. Both accounts are `primary` by
  the code, and that stays so: the list names computers, not accounts, so
  b@c's other computers and d@e's computer keep their class.

**Effects of that change:**

- the computers become `test` class, so they are hibernated first under
  memory pressure, and the standing arm admits them;
- **they become subject to a retention rule that destroys computers**
  (review 265, finding 1). Revision 2 said no retention deletion applied.
  That was wrong. It cited only the ephemeral-account rule
  (`internal/vmctl/retention_prune.go:574`). A second rule,
  `test_class_idle`, is active on Node B
  (`VMCTL_RETENTION_TEST_IDLE_MODE=active`, `VMCTL_RETENTION_TEST_IDLE_MIN_AGE=168h`,
  `nix/node-b.nix:1884-1885`) [V]. It destroys a test-class computer's
  state and its ownership (`internal/vmctl/retention_test_class.go:173-215`),
  and listing a computer in `test_computers` is enough to make it test
  class (`warmness_policy.go:129`).
- **What the rule keeps** (`testIdleKeepReasons`, the same checks again
  at destroy time). A test-class computer is kept when any of these holds:
  - it is held (a maintenance hold);
  - a per-computer selector in vmctl's environment names it by VM id or
    computer id (`selectorEnvKeyRE`, `retention_test_class.go:111`: the
    `VM_*_ONLY`, `VMCTL_*_ONLY`, `CHOIR_*_COMPUTERS` and `CHOIR_*_ONLY`
    lists). A `class:test` or `*` token names no computer and protects
    none;
  - it is not an interactive computer, or not on the account's primary
    desktop;
  - it is running, so not stopped, hibernated or failed;
  - it was stopped to recover, or was never active;
  - it was active within the last 7 days;
  - its directory holds a recovery that has not finished, or a young
    quarantine.

  The companion `quarantine_age_out` rule also covers test-class
  computers. It removes only the quarantined image of a recovery that
  finished more than 7 days ago, never the computer.
- **Today's exposure.** T-A is named by two selectors today
  (`CHOIR_VISUAL_CAPTURE_COMPUTERS` and `CHOIR_CLAUDE_DELEGATE_COMPUTERS`,
  `nix/node-b.nix:1769,1813`), so it is kept. If T-B and T-C are further
  desktops of b@c, the "not on the primary desktop" check keeps them too.
  That is a detail of the rule's present shape, so this design does not
  rely on it. A test computer can sit hibernated for more than a week
  between steps, so the protection must be explicit.
- **Rollback**: delete the lines.

**Precondition of the enabling change** (review 265, finding 1). By the
deploy that carries the `computer-tiers.json` change, in that deploy or
an earlier one, T-A, T-B and T-C are each named by computer id in a
per-computer selector in vmctl's environment. vmctl reads the selectors
at start (`cmd/vmctl/main.go:758`), so one deploy carrying both is enough.

- The selector is `CHOIR_VISUAL_CAPTURE_COMPUTERS`. T-A is already in it,
  and T-B and T-C need the visual capture grant anyway. Use explicit
  computer ids, not `class:test`, which protects nothing.
- A maintenance hold is not used. A held computer is skipped by deploy
  refresh (`internal/vmctl/deploy_refresh.go:414`), and S6's proof needs
  a deploy to reach T-B.
- FP0 reads the active retention plan after the deploy
  (`/internal/vmctl/retention-plan`, as `scripts/proofs/fp0-vm-retention.sh`
  does). Each of T-A, T-B and T-C must show verdict `keep` with reason
  `selector_named`.
  If any of them does not, revert the tiers change first.
- T-C, minted by S7, is added to the selector in the same change that
  lists it in `test_computers`, never after.
- Removing a test computer from that selector later, for example as a
  rollback of its capture grant, takes it out of `test_computers` first.
- **Class of the precondition.** The selector entry is a line of vmctl's
  environment in `nix/node-b.nix`, so it is red by the repo's classes. It
  is tier 1: additive, rolled back by deleting the id, and visible in the
  retention plan within minutes of the deploy.

**The test computers:**

- **T-A: `computer-6c50d6e566143ae32cdb660faa60e24d`** (b@c), the
  publisher. Its `source.projection` is on.
- **T-B: a second b@c computer, minted at S1** (review 257, Devin 5).
  - class `test` (after the enabling change);
  - `source.projection` on;
  - an owner-minted API key with `owner:acts`, which only the owner can
    mint, signed in;
  - the visual capture grant, by computer id in
    `CHOIR_VISUAL_CAPTURE_COMPUTERS`, which also keeps it from
    `test_class_idle` (the precondition above);
  - the `packages.*` switches turned on by FP0 as each step lands;
  - no standing work.
- **T-C: a third b@c computer**, `test`, set up like T-B, including the
  selector entry. It is minted by S7 at the latest, and serves as the
  second adopter for suspension (review 257 F9).
- **T-D: d@e's computer**, the second owner. Class `primary`, never listed
  in `test_computers`.
  - d@e's key is used in S1, S2 and S3 for privacy;
  - T-D itself is used in S5 for the cross-owner refusal (and, while it
    is primary class, for the class-floor refusal), in S9 as the primary
    proposal leg, and in S13 for framed adoption.

## 19. Open questions that are the owner's

1. **Q1. Re-materialization authority.** Is re-applying an adopted
   package's byte-identical tree on a new base (after its build and
   probation pass) a clean rebase that needs approval under "not yet"?
   Default: yes. Recommendation: no, because the approved content is
   unchanged.
2. **Q2. Who publishes.** May a publish intent carried in a request name
   a scope beyond `owner`? Default: no. May FP0 or a coding agent publish
   under the owner's standing direction? Default: yes, at `owner` scope
   only.
3. **Q3. Signing root custody (a credential question).** Where does the
   namespace root key live: on the host, offline, or on the owner's
   hardware key? Only S13 needs the answer.
4. **Q4. Automatic revert on protected computers.** Default: the revert
   is proposed, not applied.
5. **Q5. Yank authority.** Default: the owner, and FP0 on the owner's
   instruction.
6. **Q6. Merging exports on a standing policy without a per-change
   review.** Default: no. Every export gets its tier's review.
7. **Q7. Platform admins and the exporter's credential.** Who is in
   `CHOIR_PLATFORM_ADMINS`? Default: the owner's account only, with FP0
   using an owner-minted admin key. Does the owner want the scheduled
   GitHub Actions exporter? It needs the owner to mint a read-only
   proposals key and store it as an Actions secret; the workflow's own
   token pushes `lane/*` and opens the PR. Default: FP0 runs the exporter
   by hand until the owner provides the key.

## 20. Class and heresy delta

**Mutation class:** green (this document). The design governs red steps,
so its review is tier 2.

**Protected surfaces the build will touch:**

- read auth, and a new admin-visible scope;
- Store B kinds and owner and admin acts;
- a new apply path and its capsule seam;
- the boot-path projection and the boot reconcile;
- run acceptance;
- a new code-loading path.

**Heresy delta:**

- **discovered:**
  - unscoped release reads (live);
  - the cumulative release;
  - model-mediated adoption;
  - candidates that cross package boundaries;
  - (revision 2) the "test" computers of b@c and d@e are `primary` by the
    code;
  - (revision 2) a host re-check from guest-derived blobs is circular.
- **introduced:** none.
- **repaired:** none yet. S1, which is built, repairs the first item.

## 21. Review 257 disposition (revision 2)

| Finding | Disposition |
|---|---|
| DeepSeek F1 / Devin 1, the confinement re-check is circular | **Fixed** with DeepSeek's second option. The content's confinement is host-verified on the stored bytes (L2). Operation confinement is stated as publisher-side, and the adopter's build and probation catch a false claim (L2b). Binding to the approval receipt is recorded as a later upgrade (3.2). |
| F2, S5 retires the old path behind a default-off switch | **Fixed.** All retirement moves to S14, after S9 (section 2, S5, S14, L32). |
| F3, class premise and where the floor is enforced | **Fixed.** Section 18 names the code fact (b@c and d@e read as `primary`) and the enabling change (`test_computers`), and notes retention is unaffected. The floor is at the proxy subscription write, with the tier signed (6.1). |
| F4, primary auto against the consensus floor | **Fixed.** Primary auto is dropped from this design (sections 8.2, 12, S14, and the goal), and left to SR8 stage 4 as its own design. |
| F5, crash and restart precedence | **Fixed.** Section 8.5 has the boot reconcile (applied fact, then assignment fate, then host outcome; bounded indexed read) and the precedence table, including planned restarts, `building` rows and `awaiting_approval` (L9, L9b, L9c). |
| F6, S1 fails open, the blob leg, list 404 | **Fixed** per the S1 settlement (4.3, S1): the default is `owner`; list answers 200 with nothing; blob is a unit test; the ListFilter [H] is dropped. |
| F7, the `package_tree` entry and the cast [H] | **Fixed.** The entry is derived from the S5 trailers. Casts during a re-materialization are a hard requirement with a test (10.3, S6). |
| F8, tree-id prefix | **Fixed.** Extract the subtree at the root and compare (3.2, 6.4). |
| F9 / Devin 6, the S7 suspension proof | **Fixed.** T-C is the second adopter, and self-adoption is not used. |
| F10, rollback assumes SR7 | **Fixed.** Every kind-adding step carries a skip test, which SR7's gate later supersedes (13). |
| F11, S4 upstreamed rests on whole-stack containment | **Fixed.** The precondition is stated in S4, and per-entry retirement is in S6. |
| F12, two windows; the blob cap | **Fixed.** One window over two bases (10.4). The cap stays at 2 MiB in a separate `package` namespace until it is measured. |
| F13, "the host is the sole builder" | **Fixed** in the goal file's `must_preserve`: source only, no binary crosses between computers, and the host builds from S12. |
| F14, owner-scope screenshot references | **Fixed.** Dropped. The publisher's captures stay on the publisher's computer (3.1, 4.4). |
| F15 / Devin 7, writer vocabulary; intent bound to one operation | **Fixed.** Registrations use `WriterRuntime`, `WriterCorpusd` and `OwnerEdits` per kind (13). The intent is discharged once, for one operation (4.1). |
| Devin 2, export can revert a later main change | **Fixed** (`onto_moved`, 10.2, L16b). |
| Devin 3, a host build is needed on the auto path | **Fixed.** The cost is stated in 6.5 and S9, and quota use is in L19. |
| Devin 4, the S1 spec gaps (admin bypass, mechanics, P8 break) | **Fixed** (4.3). |
| Devin 5, T-B timing; the S3 refusal leg | **Fixed.** T-B is minted at S1, and the refusal leg moves to S5. |
| Devin nits: consensus line refs, `computer_setting.go:205`, the `/self-development/adoptions` route, where the secret scan lives, a separate blob namespace, the request-receipt bind, the dead `choir release adopt` verb | **Fixed.** The refs are corrected. The namespace is split. The bind is stated as an [H] for S2. The verb stays live until S14, so it is never dead. |
| DeepSeek, the acceptance cannot prove "zero model calls" by trace | **Fixed.** The proof uses L7's provider-call counter. |
| DeepSeek, the yank-survives-restore leg presumes SR7 | **Rejected with reason.** The restore runs on the current binary to an earlier tape head, so no older binary reads a newer tape. SR7 is not involved (L22). |
| DeepSeek, #5's primary leg names no primary computer | **Fixed.** T-D, while it is primary class, is that leg (18, S9). |
| Owner addition 02:0x, the platform base image | **Added**: section 10.5, steps S4 and S4b, the `platform` scope, admin acts, the pull exporter and Q7. |
| Review 265 (Devin, one seat, approved with six required edits) | **All six applied.** (1) Section 18 no longer says retention never deletes: it names `test_class_idle` and its keeps, and makes naming T-A, T-B and T-C in `CHOIR_VISUAL_CAPTURE_COMPUTERS` (checked in the retention plan) a precondition of the enabling change; b@c and d@e stay `primary` accounts and T-D is never listed. (2) Host check 3 names its mechanism: a `parent_patch` blob for a base parent, checked against the tree id the host reads from the deploy checkout's object store, which the host build already reads (3.2, S2). (3) and (4) The goal file says S14, not S5, and T-D, not T-C. (5) Section 8.5's table is read first-match and covers every non-terminal state; a planned restart with the applied fact present restarts probation. (6) Section 6.1 names `WriterOwner` and says why the subscription does not use it. |
