# A package operation decides, publishes and installs with no owner act (D7 item 1)

Date: 2026-11-06, revision r2. Status: design, for build. Mutation class: red (decision
authority, publishing to the public Library, install on a user's computer).
Mission: [modular architecture](../definitions/choir-modular-architecture-2026-11-04.md).
Parents: [forced-modularity build](forced-modularity-build-2026-11-04.md) D7
and deferred item 1; [apply and revert](selfdev-apply-and-revert-2026-11-06.md).

## 0. Why now

The apply-and-revert release (8a9f79cdf) deleted the owner's approve. Before
it, a package-only operation reached the Library through that approve, then
an owner publish act and an owner install act. D7 was to replace all three
with the consensus decision, and was deferred. So today a package operation
cannot apply at all: on d@e, `selfdev-422c2027` passed independent judgment
and was refused by the poster, `package_path_refused: ... the source stack
carries platform operations only`. The owner ruled this path long ago
(2026-11-04: "I don't want to approve each version!!! The whole point of
library is so I don't have to approve").

## 1. What exists (origin/main 8a9f79cdf; read-only map 2026-11-06)

- Freeze's mixed-layer door refuses a change set that holds a package and a
  platform path (`capsule/source_patch.go:138`, `executor.go:2360`).
- The draft and media routes admit a decided operation and run
  `pkgrelease.DraftPackage` (`agentcore/api_package_release.go:25-186`,
  `api_package_media.go:108`).
- Publish is proxy-driven: an owner session mints `ActPackagePublish`, and
  corpusd's `publishPackage` verifies the receipt, then runs
  `VerifyHostFacts` (confinement, tree and parent, review patch, secret scan,
  manifest, contract fixture), the screenshot and before/after rules, the
  `confirm_paths` digest and the track-head lock
  (`proxy/packages.go:92-250`, `platform/packages.go:~600-760`,
  `pkgrelease/host.go:108-186`).
- Local install is `installPackage` (`agentcore/package_install.go:139`):
  module build, `CheckUses`, install review, store record. The M3 boot
  migration already drafts and installs in-process from the draft's record
  and tree patch (`package_migration.go:884-1335`).
- The poster's only capsule-verifier seat is a hostbuild job on a platform
  source patch (`castHostBuildBallot`, which refuses package paths).
- The materializer's apply requires updater receipts
  (`self_development_materializer.go:~632-870`).
- `gen-package-kinds.mjs` lists every directory under `frontend/src/packages/`
  holding a `choir-package.json` as built-in (`gen-package-kinds.mjs:57-68`).
  Regenerating it for a package that will be published makes the name
  built-in: another owner's install is refused (`builtin_name_not_yours`) and
  the publisher's own computer silently replaces the built-in.
- A package patch is not composed to the stack base at freeze
  (`agentcore/source_projection.go:539-543`), so a bundle's
  `source_patch_base_commit` is usually a projection commit that exists only
  in the guest's repo; corpusd's `GitStore` holds platform commits only
  (`pkgrelease/host.go:11-30`).
- `consensus_decisions` requires a host build job and result digest
  (`standing_consensus.go:85-86`), the receipt always carries a `host_build`
  block (`:420-424`), and the guest refuses a decision naming a job it did not
  submit (`self_development_standing_consensus.go:285-287`).
- The guest-credential route `requireInternalOrBoundGuestWrite` binds a caller
  to vmctl's owner and computer (`platform/authority.go:50-89`).

## 2. Design

### 2.1 Classification

`runStandingConsensus` classifies the frozen **patch** with
`packagepath.Classify`. A package-only operation (one package root, no
platform path) is a package decision. The bundle's `ordered_file_effects`
must agree with the patch: an effect outside the package root refuses the
decision as mixed (Devin r0 W5: a regenerated-identical platform file can
appear as an effect with no diff hunk).

### 2.2 The package seat and the decision row

The guest asks the poster with `kind: package`, the source patch bytes, and
a **claimed platform base**: the stack's `StackBase` commit, which corpusd
holds (package subtrees are identical across stack commits, because the
stack carries no package path). When the computer's stack is empty the claim
must equal the booted base's build commit, derived from
`BaseImageManifestDigest`. corpusd:

1. verifies `sha256(patch) == bundle.source_patch_sha256` and that the
   claimed base is in its store;
2. runs `pkgrelease.DraftPackage` on the patch over the claimed base and
   `VerifyHostFacts`, binding the parent through `hostParent`
   (`host.go:215-243`);
3. computes the **draft content digest**: package name, root, tree, changed
   paths, manifest digest and parent;
4. signs the capsule-verifier ballot over that result and mints the decision.

The decision row gains `kind` (`platform` or `package`), and, for a package
decision, the set the publish route checks against, persisted at decide from
the bundle corpusd verified against `bundle_digest`: the source patch sha and
the frozen screen and video digests. The draft content digest is stored once,
in `host_build_job_id`, and the seat verdict digest in
`host_build_result_digest` (the NOT NULL columns stay; no constraint touches
them); the receipt carries a `package` block in place of `host_build`. The guest's check that the decision names its own
host build job becomes, for a package decision, that it names the draft
content digest the guest computed itself. `Source.BuiltOn` is recorded as
claimed provenance, not a verified fact. With a non-empty stack, a guest can
claim a store commit that holds leftover files under the same package root
(an earlier in-tree package); those files then ship although the capsule
never rendered them. The marginal authority is small (the patch itself could
carry the same bytes), and it is a stated residual (section 5).

Mode `apply`, the platform-computer refusal, the generation, artifact 0 and
the admission gate are unchanged.

### 2.3 The effect: draft once, check, publish, install, record

The materializer gains a package branch for an accepted package decision:

1. `accepted -> materializing`.
2. **Draft once and persist**: the draft record, its three patches and the
   media digests are written to the guest store keyed by the operation. Every
   later step and every retry uses the persisted draft; nothing re-drafts.
3. **Check before anything is public**: `packageBuilder.Build`, `CheckUses`
   and `installReviewGate` on the persisted draft. Any refusal closes the
   operation `failed`, and nothing publishes. After this, a failure past
   publish can only be a crash.
4. **Publish by decision** (2.4).
5. **Install locally** with `installPackage`, trigger `selfdev`, from the
   persisted draft, on the builder's own computer only.
6. **Record**: a new event kind `package_applied` (section 2.5) and the
   operation row `applied`, with the release id and the install id in new
   columns `package_release_id` and `package_install_id`, never in
   `release_digest` (which stays empty, so the release readers never match a
   package operation).

An operation's kind is derived from its bundle's patch classification and
stored on the operation row as `kind` when it freezes, so the materializer,
the revert route, `liveOperationSet` and `latestLandedOperation` branch on a
field, not a recomputation. The install trigger `selfdev` is a new constant
beside automatic, platform, migration and migration_rollback
(`store/package_install.go:91-97`).

**Crash.** The restart rule holds: a crash never resumes work. At boot, a
`materializing` package operation is settled from facts, not resumed: if
corpusd lists a release for this operation and the install record exists, it
records `package_applied`; if the release is listed but there is no install,
it closes `failed` with "published as <release>; not installed here; install
it from the Library"; if nothing is listed, it closes `failed` "interrupted
by a restart". The planned-restart marker does not resume a package branch.

### 2.4 Publish by decision (corpusd)

A new route on the guest credential (`requireInternalOrBoundGuestWrite`):
the owner and computer come from the binding, never from the request. The
request carries the persisted draft record, its patches, the source patch
bytes, the claimed platform base, the media inline and the decision digest.
The route has its own request cap, sized to the publish maxima (eight
screenshots at 2 MiB, one 8 MiB video, three patches and the source patch at
2 MiB each): 40 MiB, not the consensus route's 8 MiB. corpusd:

- loads the decision row: same computer, `kind: package`, admitted, operation
  and bundle digest equal to the record's; the computer is not a platform
  computer; the computer's mode is still `apply` at the decision's
  generation;
- verifies the source patch sha against the one persisted in the decision row, re-derives the draft over
  the claimed base, and refuses unless its draft content digest equals both
  the decision's `package_draft_digest` and the posted record's;
- refuses a record whose `Publisher` or `Track` is not the bound owner;
- checks the owner has a publisher address through the same lookup the proxy
  uses (`proxy/packages_publisher.go:106-156`), as an internal call from
  corpusd; with none it refuses `publisher_address_required`, and the guest
  closes the operation `failed` with that reason;
- accepts media only whose digests are in the decision row's persisted
  screen and video set (the capsule-rendered, shot-reviewed screens), never a
  set the request asserts, and refuses any other;
- runs every `publishPackage` host check, and the track-head lock;
- **consumes** the decision: `published_release_id` is set once; a repeat with
  the identical record returns the same release (200), and any other record
  citing that decision is refused;
- refuses past `not_after` plus a 30-minute publish grace; the guest closes
  the operation `failed` "decision expired before publish".

The decision replaces the `ActPackagePublish` receipt and `confirm_paths`.
What `confirm_paths` gave, a person seeing what becomes public, is replaced
by the bound draft digest and the frozen, shot-reviewed screens; that a
person no longer sees them is the owner's ruling and is section 5's first
risk. An admitted decision whose operation failed before publish still
authorizes publish until the window closes; corpusd cannot see guest state.
That is accepted and bounded by the window and the single consumption.

### 2.5 Events, revert and the platform layer

- `package_applied` and `package_reverted` are new event kinds with their own
  reducer rules. They do not move the platform effective release.
- **Revert of a package operation** uninstalls the install it made
  (`uninstallPackageWith`, tolerating `package_not_installed`,
  `package_migration.go:956`), appends `package_reverted`, and records the
  operation `rolled_back`. It never calls the updater. The published release
  stays in the Library (withdraw is the publisher's act, yank the operator's).
- **Revert of a platform operation** derives its prior by skipping package
  events: when `GetByEffectiveHead(prior)` names a package operation, it
  follows that operation's own prior head until it reaches a platform state
  (an applied platform operation, a base restore, genesis or a platform
  checkpoint). The terminal set is the existing restorable fallback, so a
  prior that lands on another non-operation event is refused as today
  (`api_self_development_revert.go:168-169`); not widened here. A platform
  rollback never undoes a package install;
  `liveOperationSet` tracks the two layers separately.
- `latestLandedOperation` (revert idempotency) counts platform operations
  only for a platform revert and package operations only for a package
  revert.
- Stack bookkeeping: `appliedOperationForRelease` matches on a release digest
  a package operation does not have, and the rebase and clean-rebase readers
  act on named operation ids; a test proves package operations never enter
  them.

### 2.6 The generator

A committed allowlist, `frontend/src/packages/base-packages.json`, names the
base packages. `gen-package-kinds.mjs` and the Go drift test read only listed
directories, so the generated table is deterministic without git and the
same under Nix. A directory not listed is not built-in and its kinds are not
registered (a published package with kinds is refused at publish anyway,
`package_kinds_need_platform`). A repo CI lint, run on main only and not by
the capsule's verifiers, fails when a directory under `frontend/src/packages/`
is not listed, so the base stays honest.

The capsule verifier's own gate is `npm run build`
(`frontend/package.json:8`, `rlm_engineering_runtime.yaml:58-60`). Four
scripts walk the package directories; each is scoped:

- `gen-package-kinds.mjs`: the allowlist only.
- the Go drift test (`apppackage_test.go:434`): the allowlist only.
- `check-kernel.mjs` `builtNames` (`:68-78`): the allowlist only; a published
  package's name is not a kernel built-in.
- `check-packages.mjs`: every directory, because its rules for a manifest are
  a superset of what publish accepts; documented as origin-agnostic.

## 3. Proof

Unit, over the real store: the package seat on a computer with stack
commits (claimed base); a patch whose sha differs from the bundle refused; a
mixed effects/patch bundle refused; a record whose content differs from the
seat-derived draft refused; another owner's publisher or track refused; a
second publish citing the decision with a different record refused, the same
record returning the same release; a publish past the window refused; a
platform computer refused at publish; no publisher address refused; media not
among the frozen screens refused; a build, uses or install-review refusal
before publish publishes nothing; crash settle in each of the three cases; a
package revert uninstalls and never calls the updater; a platform revert with
a package operation between it and its prior finds the platform prior; package
operations absent from stack bookkeeping; the generator under the allowlist,
including a Nix eval of the frontend build.

On d@e, after the deploy: a round adds a tiny new package (no launcher tile);
the operation reaches `applied` with no owner act; the release is in the
Library under the d@e owner; the install exists. That release is the one the
platform operator yanks. Then Revert uninstalls it, and a platform operation
applied after it still reverts.

## 4. Slices

| slice | content | class | review |
| --- | --- | --- | --- |
| P1 | classification, the package seat, the decision row and receipt, the guest check by kind | red | tier 2 |
| P2 | corpusd publish by decision (2.4) | red | tier 2 |
| P3 | the materializer's package branch, crash settle, `package_applied` and `package_reverted`, both revert rules, bookkeeping proof | red | tier 2 |
| P4 | the generator allowlist and lint | yellow | tier 0 |
| P5 later | a host build seat for packages (module build, `CheckUses` in a hostbuild job) | red | tier 2 |

Builder A: P1 and P2 (internal/platform, corpusd). Builder B: P3 and P4
(internal/agentcore, frontend scripts), coding to 2.2 and 2.4 as written.

## 5. Risks

- No person sees what becomes public before it is public (owner ruling,
  2026-11-04). The bound draft digest, the host checks, the secret scan and
  the shot-reviewed screens carry the weight; the publisher can withdraw and
  the operator can yank.
- The module build and `CheckUses` before publish are the guest's own checks;
  a compromised guest can skip them. P5 makes them independent.
- The screens are rendered in the guest's capsule and reviewed by the judge;
  they are digest-bound at freeze, not rendered by the platform.
- A claimed platform base on a computer with a stack can carry leftover
  files under the package root that the capsule never rendered (2.2).
- Two concurrent package operations on one track conflict on the head lock;
  the second closes `failed` and may be proposed again.
- Updates of an installed package need the candidate overlay or a capsule
  seeded from the installed version (deferred item 2); new packages do not.

## 6. Rollback

Revert the P commits. Releases published by decision stay in the Library
(they are ordinary releases); installs stay installed.

## 7. Review adjudication (r0)

Both seats SEND BACK; every blocking finding adopted.

- Decision does not bind what ships (both): the draft content digest in the
  row, re-derived at publish, consumed once (2.2, 2.4).
- Seat cannot read projection commits (both): claimed platform base (2.2).
- No computer-to-owner binding (Sonnet): owner from the guest-credential
  binding; publisher address checked in corpusd (2.4).
- Public screens with no person (Sonnet): media only from the frozen,
  shot-reviewed screens; residual stated (2.4, 5).
- Crash between publish and record (both): draft once and persist; checks
  before publish; settle from facts, never resume (2.3).
- `applied` without receipts, and revert (both): new event kinds; package
  revert uninstalls; platform revert skips package events (2.5).
- Generator field does not exist; git not available under Nix (both):
  committed allowlist (2.6).
- Decision schema and receipt for a job-less decision (Devin W2): adopted (2.2).
- Mixed effects with a package-only patch (Devin W5): adopted (2.1).
- Publish window and mode re-check, platform refusal at publish (Sonnet):
  adopted (2.4).
- Media upload channel (Devin F4): inline, bounded (2.4).

## 8. Review adjudication (r1)

Devin r1: all five r0 blocking findings resolved; one new blocking and seven
non-blocking items, all adopted in r2:

- N1 the 8 MiB consensus cap cannot carry a release: the publish route's own
  40 MiB cap (2.4).
- N2 the patch sha and screen set come from the decision row persisted at
  decide, never from the request (2.2, 2.4).
- N3 claimed base: bound to the booted base commit for an empty stack; the
  leftover-files residual stated (2.2, 5).
- N4 all four directory walkers enumerated and scoped (2.6).
- N5 the operation row's `kind`, and the release and install ids in their own
  columns (2.3).
- N6 the draft digest stored once (2.2).
- N7 the restorable fallback is not widened (2.5).
- N8 the `selfdev` install trigger (2.3).
