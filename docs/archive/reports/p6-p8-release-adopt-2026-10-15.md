# P6 to P8: Effective Identity, the Release Record, Source Adoption (Station B, minimal)

Date: 2026-10-15. Branch `feat/p6-p8-release-adopt` from `origin/main`
133de45fa, with `selfdev-patch-export` (c8db35df5, 17fbe5171) cherry-picked
as the source of patches. Mutation class: **red** (shared publication in
Store B, outside every restore set; a new host content namespace; a new
guest effect path that starts self-development from another computer's
source; capsule inputs; a vmctl write route). This commit is docs only: the
design, the failure modes the code must hold (written before any test), and
the ceremony. Code follows on the same branch, each commit naming the
failure modes it closes.

Sources: `docs/archive/reports/selfdev-update-architecture-2026-10-15.md` (P6, P7,
P8, §1.1, §1.5, §1.6, §1.8, Station B) and its owner rulings at the top:

1. **Only source is ever published.** Binaries come only from the host
   builder, built from published source. Users never pass binaries to one
   another.
2. **The host is the sole builder.** A release record may name the host's
   build digest; it never carries executable bytes.
3. **Station B is a one-test proof of source adoption onto the adopter's
   own tree,** not a product. No investment in rebasing across diverged
   computers: a patch that does not apply is refused, not merged.
4. **App packages (S7) come later.** The record reserves a `packages`
   field and nothing reads it.

P5 (host build, layered apply) is being built in parallel on
`feat/p5-host-build-layered-apply` (design
`docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md`). This branch does
not build anything on the host and does not touch the materializer. The
seams are named in "Seams with P5" below.

## What exists today, read in code

- **Guest `/health`** (`internal/agentcore/api.go`, `HandleHealth`) reports
  `build` (the running binary's commit) and `release_digest`, cached once
  at startup from `current/release-manifest.json`. No event head, no
  CodeRef, no tree. `/api/runtime/observability` (the product path the
  tracer reads) reports only `build`.
- **vmctl list** (`internal/vmctl/handlers.go`, `listOwnershipResponses`)
  reports each ownership's `construction_version` (CodeRef and
  ArtifactProgramRef, from vmctl's own route ledger) and a manually set
  `divergence_status`. vmctl never probes a guest from a read path
  (`health_no_guest_probe_test.go`, the 2026-10-10 hung-guest receipt), so
  it cannot ask a guest what it runs at list time.
- **The patch** (cherry-picked): `GET .../operations/<op>/source-patch`
  serves a frozen bundle's `source.patch` with its sha256 and base commit;
  `readBoundSelfDevelopmentBundle`, `readBoundSourcePatch` and the
  added-line secret scan are the reusable internals.
- **The object graph write gate** (`objectgraph.CheckWrite`, 7f287a135)
  refuses any kind `DefaultRegistry` does not register; both durable stores
  call it. `TestEveryRegisteredKindHasAWriter` holds every registered kind
  to a production writer.
- **The host blob store**: corpusd's `sha256/platform-update/<sha>`
  namespace with a 2 GiB streaming upload cap
  (`internal/platform/platform_update.go`, `HandlePlatformUpdateBlob`).
  Guest reads use the computer capability (`event:read`).
- **The capsule source**: an engineering capsule's `/workspace/platform` is
  the immutable tree of `HEAD` of the guest's own source repository
  (`CHOIR_CAPSULE_SOURCE_ROOT`, `/mnt/persistent/files/Source/platform`),
  pinned by commit and canonical digest at preflight. Freeze renders the
  diff against that pin as `source.patch` and records the pin's commit as
  `source_patch_base_commit`.
- **Owner acts** at the proxy: `authorizeOwnerAct` admits only the owner's
  session or a key carrying `owner:acts`.

## The design

### P6. Effective identity

One function, `Runtime.EffectiveRelease(ctx)`, answers "what code is this
computer running and at which head", from the single authorities for each
fact:

| Field | Authority |
|---|---|
| `release_digest`, `marker`, `code_ref`, `artifact_program_ref`, `layered`, `code_commit`, `base_commit`, `closure_digest`, `builder_receipt_digest`, `base_image_manifest_digest` | the release manifest `current` points at (`updater.ReadCurrentManifest`), read live |
| `serving_release_digest` | the manifest read when this process started (what the running binary booted with); differs from `release_digest` only between a pointer swap and the restart |
| `build_commit` | the running binary (`buildinfo`) |
| `operation_id`, `source_base_commit`, `source_patch_sha256`, `tree_hash` | the applied self-development operation whose `release_digest` is the current one, its bound bundle and patch; `tree_hash` is the git tree of `base_commit` plus the patch, computed with a throwaway index and object directory so the source repository is never written |
| `tree_hash` (no self-development release) | `git rev-parse <build_commit>^{tree}` in the guest's source repository, empty with `tree_error` when the commit is not there (the source seed lags the image, P13) |
| `event_head` (`canonical`, `desired`, `effective`, `sequence`) | the guest store's head of the computer's event chain |
| `adopted_release_id` | the release adoption whose trajectory produced the applied operation |

The tree is computed once per release digest and cached; `/health` never
runs git twice for the same release. Both `/health` and
`/api/runtime/observability` carry the block as `effective_release`.

**vmctl list without probing a guest.** The guest reports its identity to
vmctl (`POST /internal/vmctl/computers/effective-release`, admitted only for
a host caller or the tap-bound guest of exactly that computer, like the
route-projection routes). It reports after start and after a restore. vmctl
stores the last report on the ownership (persisted with the registry) and
`vmctl list` shows it as `reported_release` with `reported_at`. It is
labelled a report, not an authority: vmctl's own `construction_version`
stays beside it, and a disagreement between the two is visible, not
reconciled.

### P7. The release record

A new Store B object kind, **`choir.software_release`**, owner
`internal/platform (corpusd)`, registered in `objectgraph.DefaultRegistry`
so the write gate admits it. Its body is the canonical JSON of
`release.Record` (`internal/release`):

```text
schema                 choir.software_release.v1
publisher              computer_id, owner_id
operation              operation_id, bundle_digest, applied (decision event,
                       materialization receipt, checkpoint ref, route
                       certificate, code_ref), verifier_refs
source                 base_commit, patch_sha256, patch_bytes, tree_hash,
                       changed_paths
precommitments         [{record_id, kind, question, p_yes, settled, resolve_record_id}]
                       engineering's rule-settled say-do record for the
                       assignment that froze the bundle
host_build (optional)  builder_receipt_digest, closure_digest, code_commit,
                       base_image_manifest_digest: from the applied release
                       manifest when it is layered (empty until P5 lands)
declarations           test_command (owner-declared at publish), packages
                       (reserved for S7, must be empty)
published_at
```

The **release id** is the sha256 of the record's canonical JSON. The record
says nothing about who should run it.

**Bytes.** The patch goes into the host content store under
`sha256/release/<sha>`. `platformUpdateBlob` and the release store become
two namespaces of one `hostContentStore` (streaming put with digest check,
2 GiB cap, content addressed, an existing blob is kept). The publish route
refuses a patch over the 2 MiB export cap regardless.

**Publish is an owner act from the originating computer.**

```text
owner: choir release publish --computer A --operation op [--test-command ...]
  -> proxy  POST /api/computers/A/releases            owner act, A's owner
  -> guest A GET .../operations/op/release-draft       read only: record + patch
     (op applied; bundle and patch bound; secret scan; tree hash)
  -> proxy  verifies digest(record) and sha(patch)
  -> corpusd PUT  /internal/platform/releases/blobs/<sha>   host only
  -> corpusd POST /internal/platform/releases               host only
     (re-hash the stored blob, re-derive the id, publisher = the computer the
      proxy authorized, insert-if-absent)
```

corpusd's publish route admits only the host (`trustedInternalCaller`): a
guest holding its own computer capability cannot publish. That is what
"owner-authorized" means here: the content comes from the originating
computer's own tape and bundle, the act comes from its owner through the
proxy, and no guest credential reaches the write.

**Reads.** `GET /internal/platform/releases/<id>` and the blob read admit
the host or any bound guest with its own `event:read` capability (a
published release is readable by every computer on this cloud).
`GET /api/releases` and `GET /api/releases/<id>` serve owners through the
proxy.

### P8. Source adoption (Station B, minimal)

```text
owner: choir release adopt --computer B --release R
  -> proxy  GET /api/releases/R (the record the owner adopts)
  -> proxy  POST /api/computers/B/self-development/adoptions {release_id, record}
            owner act, B's owner
  -> guest B, before any state change:
       1 digest(record) == release_id                     release_digest_mismatch
       2 corpusd's published record for R exists and its
         canonical bytes equal the supplied record         release_not_published / release_record_mismatch
       3 the published record re-hashes to R               published_record_digest_mismatch
       4 record valid, packages empty                      release_record_invalid
       5 patch bytes from the host store hash to the
         record's patch_sha256 (and fit 2 MiB)             patch_unavailable / patch_digest_mismatch
       6 added-line secret scan                            patch_secret_refused
       7 git apply --check against B's OWN source HEAD
         (throwaway index; repo untouched)                 patch_does_not_apply
  -> stage source.patch and release.json, content addressed, read only
  -> submit the adoption request to B's own desks (the prompt bar's
     submission path, owner identity), get the trajectory
  -> write choir.release_adoption on B's tape (VM object graph; the
     projection interceptor appends it): release id, B's base commit and
     tree, R's base and tree, trajectory
B's desks: Texture -> management Cast -> engineering assignment on that
  trajectory; the capsule spawn finds the adoption by trajectory, re-hashes
  the staged patch and mounts it read only at /selfdev/adoption/
engineering: git apply in /workspace/platform, run the check, build the
  release tree, Freeze (the bundle's patch is B's diff against B's pin)
management: Verify; B's gate decides (owner approval or B's mode); the
  materializer applies through B's updater; checkpoint; route
```

B never receives a binary from A or from the host. The patch is applied to
B's own tree, in B's capsule, through B's engineering desk and B's gate.
When B's pinned base equals R's base, B's frozen `source_patch_sha256`
equals R's (the patch renderer is deterministic over the same old and new
contents); the adoption read reports `patch_matches_release` and the tracer
requires it. When the bases differ, the patch is still B's own diff and the
check is reported, not required (no rebase work, per ruling 3).

**The adoption read** (`GET .../self-development/adoptions[/<id>]`)
derives status from the single authorities: the adoption object (what was
requested and verified), the self-development operation on its trajectory
(state, bundle), and the bound bundle's patch sha.

## Failure modes (written before the tests)

Identity (P6):

- I1 `/health` reports the release the process started with after a swap,
  so a computer whose `current` moved reads as unchanged (startup cache
  only). The live manifest is `release_digest`; the startup one is
  `serving_release_digest`.
- I2 a self-developed computer reports the base build's tree (the SHA
  alone), so its effective tree cannot be told from the platform's.
- I3 the tree is computed by writing to the guest's source repository
  (index, objects): a read path mutating source state.
- I4 `/health` runs git on every call (latency, a hung git hangs health).
- I5 the event head is read from a cache that a restore did not update.
- I6 vmctl list probes a guest (fate-shares with a hung guest).
- I7 a guest reports another computer's identity to vmctl (not tap-bound
  to the computer it names), or any caller writes a report.
- I8 a report is taken for vmctl authority (overwrites
  `construction_version` or `divergence_status`).

Release record and publish (P7):

- R1 a record whose id is not the digest of its canonical bytes is stored.
- R2 a record names a patch sha whose blob is absent, or whose stored bytes
  hash elsewhere.
- R3 a guest publishes with its own computer capability (no owner act).
- R4 the publisher fields name a computer or owner other than the one the
  proxy authorized.
- R5 an unapplied, rejected or rolled-back operation is published, or a
  patch with secret-bearing added lines.
- R6 the record carries executable bytes, or a non-empty `packages`.
- R7 the write bypasses the object-graph gate (unregistered kind).
- R8 a republish of the same record creates a second object or replaces
  the first.
- R9 a patch over the 2 MiB cap is accepted; an upload over the content
  store cap is buffered in memory.
- R10 a guest reads the release list with another owner's filters, or
  writes a blob.

Adoption (P8):

- A1 a tampered record (any field changed, id kept) is adopted.
- A2 a self-consistent record that was never published is adopted.
- A3 corpusd's stored record or blob was altered after publish and is
  adopted (the adopter trusts the store).
- A4 patch bytes that do not hash to the record's sha are staged.
- A5 a patch that does not apply to the adopter's own tree reaches the
  desks (a conflict becomes a silent overwrite or a model's merge).
- A6 any refusal above leaves a trace in B's state: a tape event, a run, an
  adoption object or a capsule.
- A7 the patch check writes to B's source repository.
- A8 the capsule mounts staged bytes that changed after verification, or
  mounts them writable (shadowable through the overlay upper).
- A9 an assignment on another trajectory receives the adoption's patch.
- A10 adoption copies A's binary, A's release, or a host closure onto B.
- A11 the adoption request is not the owner's act (agent key without
  `owner:acts`), or it targets a computer the caller does not own.
- A12 a repeated adopt request starts a second request to the desks.

## Ceremony (red)

- **Conjecture delta.** Before: a computer's identity is its build SHA plus
  a release digest; a self-developed change lives and dies on the computer
  that made it. After: a computer reports the release, tree, event head and
  CodeRef it runs; a change can be published as a content-addressed source
  record by its owner and adopted, as source, by another computer through
  that computer's own desks and gate, with tampering refused before any
  state change.
- **Protected surfaces touched.** Shared publication (Store B: a new kind
  and its writer; the host content store's new namespace); a new guest
  path that opens self-development work from published source (the
  adoption request through the prompt bar's submission path); capsule
  inputs (`/selfdev/adoption`, read only); a vmctl write route (identity
  reports, report-only). Not touched: canonical acceptance (the decision
  path and modes are unchanged), the materializer and updater, checkpoint
  and route projection, auth (no new scope: release reads use
  `computer:self_development:read`, publish and adopt use
  `computer:self_development:approve` plus the owner-act gate).
- **Admissible evidence.** Unit tests: the record's digest, validation and
  tamper refusals; the tree hash and apply check against a real git
  repository with the repository unchanged; corpusd publish and read
  (host-only write, blob re-hash, insert-if-absent, gate registration);
  the guest adoption refusals with no store write, no run and no staged
  file; the capsule spawn's trajectory lookup; the proxy's owner-act gate;
  the vmctl report's tap binding and persistence; the effective identity
  block. Deployed proof: `--scenario adopt` on two freshly minted test
  computers on `choir.news` with `--expect-sha`. Local tests prove no
  vmctl, no live guest, no capsule mount (darwin) and no corpusd SQL.
- **Rollback.** `git revert` of the branch. Runtime: the release kind's
  objects are inert (no reader acts on them without an owner act); an
  adoption in flight is withdrawn by rejecting its candidate; an applied
  adoption is reversed with `choir computer restore` to the pre-adoption
  checkpoint (B's own restore; Store B and A are untouched, by the
  restore-set boundary).
- **Heresy delta.** Discovered: `/health`'s `release_digest` is a startup
  cache, so it disagrees with `current` between a swap and a restart (I1);
  vmctl list cannot know what a guest runs without a guest report, and the
  manual `divergence_status` is the only divergence signal (P9 stays
  open); the three retired adoption edges (`package_source_computer`,
  `adoption_package`, `adoption_target_computer`) stay registered and
  unwritten: adoption lives on the adopter's tape as an object, not as a
  Store B edge, so this branch writes none of them and does not delete
  them (deleting registered edges is its own change with its own citer
  sweep). Introduced: the adoption request is model-mediated (Texture,
  management and engineering carry it); a desk that drops it is a visible
  finding, not a silent pass. Repaired: none until the deployed proof.

## Seams with P5

- **`effective_release` is the one identity block.** P5's design adds
  `effective_release` (release digest, layered, code commit, base commit,
  closure digest, builder receipt digest) to `/health`. This branch adds
  exactly that block, with those field names, plus the P6 fields. P5 should
  read `Runtime.EffectiveRelease` instead of adding a second block; the
  merge conflict, if any, is in `runtimeHealthResponse` only.
- **`host_build` in the record** is read from the applied release's
  manifest (`updater.ReadPinnedManifest(root, op.ReleaseDigest)`). On
  `main` the materializer mints plain releases, so it is empty. When P5's
  layered mint lands, the same read fills it with P5's builder receipt and
  closure digests; no change here.
- **The content store.** P5 stores closures in `sha256/platform-update/`
  through its own `storePlatformUpdateBlobFile`. This branch puts the
  streaming put and open behind `hostContentStore`; P5's file ingest can
  become `contentStore("platform-update").PutFile` when the branches meet.
- **Adoption of a Go change.** On this branch an adopted Go change runs only
  after B's own apply, which (until P5) mints a plain release; the base
  binary keeps running (P5's R4). Station B's tracer therefore proves a
  static change. Once P5 lands, B's apply goes to the host builder with B's
  base commit and B's own frozen patch: the host builds B's tree, never A's.

## Station B tracer probe

`node scripts/reeng_tracer.mjs --label <l> --scenario adopt --expect-sha <sha>`:

1. Mint computer A; run the engineering drill on A (static change) to
   `applied`.
2. Publish R from A's operation (`POST /api/computers/A/releases`); read it
   back through `GET /api/releases/<id>`.
3. Mint computer B; arm `propose_only` on B.
4. Negative legs on B, each refused, with B's event head, operation list
   and adoption list unchanged: R's record with one field altered and the
   id kept (`release_digest_mismatch`); a self-consistent unpublished
   record (`release_not_published`).
5. Adopt R on B; follow B's desks to `awaiting_approval`; approve as B's
   owner; follow B's apply.

Accepted when: R lists with A's operation, bundle and settled
precommitments; B's adoption names R and B's trajectory; B's operation is on
that trajectory; B's frozen patch sha equals R's; B applied through one
planned restart; B serves A's token file; B's `/api/runtime/observability`
`effective_release` names B's applied operation with `adopted_release_id`
R and a tree hash equal to R's; both negative legs refused with no state
change.

## Residuals (named, not fixed here)

- The adoption is an object on B's tape, not a typed `adoption_requested`
  event kind; the operation links to it by trajectory.
- The link from the request to the engineering capsule is the trajectory;
  the adoption object is written right after the prompt submit returns, so
  a cast inside that window would spawn without the patch (visible: the
  desk reports no `/selfdev/adoption`).
- B's gate is today's: the owner's approval (or B's armed mode). The
  standing consensus mode is P4.
- No rebase: a patch that does not apply is refused (ruling 3).
- `tree_hash` for a base computer needs the build commit in the guest's
  source repository; a source seed that lags the image reports
  `tree_error` (P13).
- CI's "already current" test still compares commit SHAs; tree containment
  is P9.
- One host holds the only copy of the registry (SP).
- The release list has no pagination cursor beyond `limit`.

## Landed on this branch (not pushed, not deployed)

| Commit | Class | What | Failure modes closed in tests |
|---|---|---|---|
| ab39e9638, 7fe60fe69 | orange | cherry-picks of c8db35df5, 17fbe5171 (patch export route and `choir self-dev patch`) | (their own) |
| a73fe2e5d | green | this design | — |
| bbba80efe | red | `internal/release`; corpusd `hostContentStore`, `/internal/platform/releases`, `choir.software_release` | R1, R2, R3, R4, R7, R8, R9, R10, A3, I3, A5, A7 |
| 8e110c88b | red | `EffectiveRelease` on `/health` and observability; release draft; adoption; `choir.release_adoption`; capsule `/selfdev/adoption`; `SubmitOwnerPrompt` | I1, I2, I4, I5, R5, R6, A1-A10, A12 |
| cb17b0268 | red | proxy publish, `/api/releases`, adoption classifier and owner-act gate | R1-R4, A11 |
| a4ff7619c | red | vmctl `reported_release` and the guest's report | I6, I7, I8 |
| b72c847f0 | orange | `choir release list|show|adoptions|publish|adopt` | A1 at the client |
| dbc528300 | yellow | tracer `--scenario adopt` | the deployed probe |

Not exercised locally (darwin): the capsule's `/selfdev/adoption` bind mount
(linux-only code, compiled with `GOOS=linux`), corpusd against the deployed
corpus-dolt server, vmctl's tap binding on a real tap, and the desks
carrying an adoption request. Those are the deployed probe's job.

Residuals found while building, added to the list above:

- A computer whose updater root has no `current` (no release ever staged)
  reports no `release_digest` and so sends vmctl no report.
- The adoption does not check B's self-development mode first; an
  adoption on an unarmed computer reaches the desks and stalls at Freeze
  (visible in the adoption read as no operation).
- The proxy reads a single release from corpusd as the host with
  `computer_id=host`, because corpusd binds every single-release read to a
  computer name.
- `effective_release.tree_hash` for a self-developed computer is the
  operation's base plus patch; a second self-development change on top of
  an applied one is pinned at the source repository's HEAD (P13), so its
  tree omits the first change.
