# Self-Development Update Architecture

Date: 2026-10-14, late (for 2026-10-15). Mutation class of this report:
green (design only; no code, no deploy, no secrets). Everything it proposes
to build is orange or red, named per step. Code was read at `origin/main`
d9af8c9d5 and on the local branches `selfdev-land` (afc8a85b9) and
`selfdev-patch-export` (c8db35df5). Neither branch is on `origin`. Claims I
did not confirm in code or evidence are marked *unverified*.

Owner direction (2026-10-14, late; recorded in
`docs/reports/owner-notes-2026-10-14.md` §13), paraphrased: self-development
runs autonomously, with no approval, until the whole loop is proven; approvals
come after that. Updates are not published straight to GitHub (perhaps to a
branch). A guest makes an upgrade and publishes it to the host. Another guest
can download it, and so can the host. Pushing an update to everyone takes
human approval. Work out the real prerequisites, including app packages and
other updates, and work efficiently.

**Owner ruling (2026-10-14, after this report): adoption is not a copy of
a host-built binary.** Computers diverge: each carries its own
self-developed changes. A binary built from computer A's base plus A's
patch is therefore not a valid build for computer B. Binary updates come
back through a more modular architecture, app packages (S7), where a
package can be built and swapped independently of each computer's own
divergence. This changes the report as follows:

- **Station A is unchanged.** The host builds the originating computer's
  own tree (its base plus its patch), so host build authority still holds
  for the computer that made the change.
- **Station B is not "serve host-built bytes".** Until app packages exist,
  an adopter takes the release as source. The patch is applied to the
  adopter's own tree and built for that adopter's own base, through its
  own capsule and gate, or by the host for that adopter. A patch that
  does not apply cleanly is a conflict for the adopter's engineering desk,
  never a silent overwrite.
- **S7 (app packages) moves onto the critical path** for binary adoption.
  "Not needed to prove the loop" (below) stays true only for source
  adoption.
- **Open decision 2 ("host-built bytes for adopters") is resolved:** source
  adoption now, binary adoption per package after S7.

**Owner ruling (2026-10-14, later): the distribution plan.**

1. **Only source is ever published.** Binaries come only from the trusted
   builder, the host, built from published source; builds are reproducible
   and host-signed. Users never pass binaries to one another. This holds in
   every phase: anyone who distributes code on the public system publishes
   its source.
2. **Station B, source adoption, is a proof, not a product.** The host
   builder from Station A builds the adopter's tree plus the patch. There is
   one test and no investment in rebasing patches across diverged
   computers.
3. **App packages (S7) are the real distribution path.** A package (a
   frontend module plus an optional backend service, with its own build
   recipe) is published as source. The host builds it once, and any computer
   installs that build whatever its own divergence.
4. **There are two lanes.**
   - **Apps** travel as packages.
   - **Core platform changes** (runtime, desks, kernel) go upstream as a
     platform release: the owner's merge to `main` and a deploy, the one
     human-approved push to everyone.

   Core changes are never adopted directly between diverged computers.

**Order:** Station A, then a minimal Station B, then S7 packages ahead of
any broader adoption work. Station C (the host adopts a core change) follows
the core lane.

## The answer in one page

1. **One object carries a change everywhere: a release record.** It is
   content-addressed and holds the base commit, the source patch, the build
   the host produced from them, the verification receipts and the
   precommitment settlements. The metamission already names it ("base
   revision + patch stack + pinned inputs + build recipe + tests +
   receipts"). The record is published to the host as an object in the
   Autopaper object graph (Store B), and its bytes are kept in a host content
   store. Adopting a release is not a copy onto the tape: each computer runs
   it through its own gate.
2. **"Autonomous" is a standing consensus policy, not a missing check.** The
   code already has the policy: `reversible-selfdev-v1` with
   `human_seat: "absent"`, two verification seats, and the author recused. It
   also has a reducer (`decisionpolicy.Reduce`). What is missing is a mode
   that stays armed across candidates, an actuator outside the guest that
   collects the ballots and posts the decision, and ballots that are signed
   rather than plain hashes. Today no path accepts a candidate without an
   owner credential.
3. **A Go change does not run today because the computer mints a plain
   release.** The host builder can already build from a base commit plus a
   patch (`choir-builder --source-patch`). The guest updater and boot wrapper
   already run a layered release's own binary and frontend. The missing join
   is the S2-f residual. Freeze's patch goes to the host builder, and the
   materializer mints a layered release from what the builder produces.
   After that, the capsule's own binary is only a test artifact.
4. **The host is the build authority.** Adopting computers run bytes the host
   built from source, never bytes another guest built. This is how the
   owner's "download it" fits the owner's settled 2026-10-01 "source only"
   rule for S8 (see §6, decision 2).
5. **GitHub stays the platform's source of truth and becomes the fleet-push
   approval surface.** The host registry is the source of truth for
   computer releases. "The host adopts a release" ends as a commit on a
   `release/*` branch that the owner merges into `main`, which then runs the
   existing Landing Loop. That merge is the one human approval the owner
   asked for. Approval UI is not needed to prove the loop.
6. **App packages (S7) are not a prerequisite for proving the loop.** They
   are a prerequisite for several publishers composing changes, and for
   adopting one app without adopting another computer's whole runtime. A
   package rides inside the release record, so building the record first
   costs S7 nothing.
7. **The minimal proving sequence is one landing plus three stations.**
   Station 0 lands the capsule build fixes and runs the existing tracer.
   Station A: a test computer makes a Go change that runs, with no human act.
   Station B: a second test computer adopts that release. Station C: the host
   adopts it through a release branch and one owner merge. Each station has
   a probe in §4.

## 1. The target architecture

### 1.1 The release record

A **release** is the unit that moves between computers and the host. Its
identity is the sha256 of a canonical JSON record that contains:

- **source:** `base_commit` (a commit on the platform's history),
  `source_patch_sha256`, and the patched tree's git tree hash. Freeze already
  writes `source.patch`, its sha256 and its base commit into the frozen
  bundle (`internal/capsule/executor.go`, `emitSourcePatch`), and proves that
  the patch applies.
- **build:** the host builder's receipt digest, the app-layer closure digest
  and the base image manifest digest it was built against. The fields already
  exist on `ReleaseManifest` (`internal/updater/updater.go:45-90`) and in the
  builder receipt (`internal/builder/receipt.go`).
- **evidence:** the operation id, bundle digest, verifier references, the
  consensus or owner decision receipt, the applied, checkpoint and route
  receipts on the publisher's tape, and engineering's rule-settled
  precommitments (`tests_pass`, `paths_confined`, `verify_pass`,
  `completes`; `internal/types/commitment_rules.go`). The precommitments make
  the publisher's say–do record part of what an adopter reads (owner notes
  §1 and §10: self-development is built on precommitment scoring).
- **declarations:** the test command, the store-schema and work-state-epoch
  window the release reads, and, once S7 exists, the package manifest.

A release says nothing about which computers should run it. That is decided
by each adoption and by the fleet push.

### 1.2 End to end

Each step below names where it lives and what exists today.

1. **Capsule change (guest).** Texture Casts the request to management, and
   management Casts engineering. Engineering edits `/workspace/platform` in a
   capsule, runs its checks, stages its precommitments and Freezes. Lives in:
   the guest, on the tape. Exists: yes, but Freeze has never been reached on
   the records path. The fix is the unlanded `selfdev-land` branch
   (`docs/archive/reports/selfdev-readiness-2026-10-15.md`, B1).
2. **Frozen candidate (guest).** Freeze opens the operation, writes the
   bundle to `/mnt/persistent/choir-updater/incoming/<digest>/` with
   `source.patch`, and management's Verify records a verdict. Exists: yes.
3. **Host build (host).** *New.* The guest asks the host build service for
   `(base_commit, source.patch)`. The builder applies the patch, builds the
   app layer (runtime and frontend from one flake evaluation) against the
   computer's booted base image, and stores the closure in the host release
   store. It returns the builder receipt. The capsule's own `bin/autoputer`
   stops being a release input. It stays a test artifact. Exists: the builder
   library and CLI (`internal/builder`, `cmd/choir-builder --source-patch`),
   with no service, no route and no caller (S2-f residual,
   `docs/archive/problems/s2-selfdev-runtime-changes-binary-carried-2026-10-04.md`).
4. **Autonomous accept (corpusd).** *New.* On a computer the owner has armed
   for standing autonomy, a consensus actuator collects signed ballots from
   the two verification seats, reduces them under `reversible-selfdev-v1`,
   and posts the decision with a consensus authority reference. See §1.3.
   Lives in: corpusd (it already holds the mode table and signs authority
   receipts), and the decision event goes on the tape.
5. **Apply with the new code running (guest).** The materializer mints a
   **layered** release from the builder's closure. Today it mints a plain
   release from the bundle's runtime files
   (`internal/agentcore/self_development_materializer.go:387-392`). The
   updater stages it, swaps `current`, and restarts the runtime unit under a
   planned-restart marker with reason `self_development_apply`. The boot
   wrapper then executes the layer's binary, and the computer surface serves
   the layer's frontend (`nix/autoputer-vm.nix:124-223`;
   `internal/autoputer/computer_surface.go`). Then come the health probe,
   `materialization_applied`, the checkpoint and the route. Exists: everything
   after the mint. The layered-checkpoint identity defect must be closed for
   a second change or a restore to work
   (`docs/archive/problems/layered-checkpoint-identity-gate2-2026-10-10.md`).
6. **Publish (guest to host).** *New.* After `applied`, the guest (or the
   owner's CLI) publishes the release record. The record becomes a Store B
   object of a new kind. The patch and the closure are already in the host
   release store from step 3, so publishing moves no large bytes. See §1.5.
7. **Adopt (other computers).** *New.* A computer lists releases, and its
   owner (or, under that computer's standing policy, its management desk)
   requests one. corpusd mints an offer bound to the adopter's current
   realization and event head, at the moment of adoption. The adopter's own
   gate decides, and the same apply path as step 5 runs. See §1.6.
8. **Host adopts (host, then GitHub).** *New, small.* The host already holds
   the record and the bytes, so it "downloads" by construction. Making a
   release the platform is a fleet effect. It is exported as a commit on a
   `release/<id>` branch, and the owner merges it. The Landing Loop builds
   the platform from it. See §1.7 and §2.
9. **Fleet push (vmctl).** The new platform becomes each tracking computer's
   pending update under station LU (pull, quiet moment or drain, rings,
   health). Computers that diverged (running a release the platform does not
   contain) are held, never overwritten. Exists: designed (LU1a and LU2
   reviewed, LU5 drafted; `docs/archive/definitions/choir-ledger-lu-guest-updates-2026-10-14.md`).

### 1.3 What "autonomous" maps to in code

Five modes are defined in `internal/platform/self_development_modes.go:18-22`:

- `off` and `audit_only` do not open candidates.
- `propose_only` opens candidates but cannot approve them.
- `accept_once` is a one-shot owner grant bound to one operation, its bundle,
  its heads and its commitments. It expires and is consumed back to
  `propose_only`.
- `qualified_consensus` needs the same one-operation binding, plus a policy
  digest and a consensus receipt digest.

Acceptance happens only in `decideSelfDevelopmentOperation`, which requires a
host-sourced authenticated user (`internal/agentcore/api.go:164-173`). The
decision verifier accepts only an `external-owner:` or a consensus authority
reference (`self_development_decision_binding.go:22-47`). Management's Verify
goes no further than `awaiting_approval` (`tools_capsule.go:296-409`). **No
code path accepts a candidate without a human credential today.**

The policy for autonomy already exists: `internal/decisionpolicy/reversible-selfdev-v1.json`.
It sets `human_seat: "absent"`, effect class `reversible_computer_local`,
and the blast radius "this computer's release pointer". It requires 2
accepts in the verification domain, from the seat `capsule-verifier` (verifier
refs bound to the capsule's build, test and runtime receipts) and the seat
`independent-reviewer` (an agent that did not author the bundle). The author
is recused. Doctrine C14 allows exactly this: a human is a policy-selected
seat, not a universal gate.

Four gaps stand between that policy and a working autonomous accept:

- **A standing mode.** `qualified_consensus` binds one operation, so someone
  has to re-arm it for every candidate. Add a standing variant bound to a
  policy digest, a computer and an expiry, but not to an operation. The
  owner arms it once per computer, and each candidate is reduced under it.
  The one-operation subject binding moves from the mode into the ballot,
  where the policy's `subject_binding` already puts it.
- **An actuator outside the guest.** Something has to collect the ballots,
  call `decisionpolicy.Reduce`, and post the decision as
  `consensus:<receipt>`. It should run in corpusd, next to the mode table and
  the authority signer, or as a vmctl sweep. It should not run in the guest:
  the guest is the subject, so it must not be the actuator. Nothing outside
  tests builds a `ConsensusBundle` today.
- **Signed ballots.** Ballot "attestation" is a sha256 over a preimage, not
  a signature (`internal/decisionpolicy/digest.go:74-81,104-129`), so a ballot
  can be fabricated.
  - The `capsule-verifier` ballot can be produced mechanically from the rule
    settlements (`tests_pass` and `paths_confined` settled `true` by scorer
    `system:rule`) plus the Freeze receipts, and signed by the guest receipt
    signer.
  - The `independent-reviewer` ballot is management's Verify verdict, or
    Jev's verdict on the frozen candidate (32febcf6). It is signed by that
    desk's key.
  - Both seats run inside the same guest, so the independence is weak. That
    is acceptable on test computers and is a named residual everywhere else
    (decision 1).
- **Close the guest-arming hole first.** corpusd's mode POST appears to
  accept any caller that holds the `event:append` capability
  (`self_development_modes.go:364`). The guest's own credential carries that
  scope (`credential_envelope.go:211`), and `GuestCredentials.SetSelfDevelopmentMode`
  exists (`internal/selfdev/credentials.go:386`). *Unverified end to end*,
  but if it holds, a guest could arm its own mode. A standing autonomous mode
  makes that hole fatal. Only an owner credential may arm, and that is a
  precondition of P3.

The policy still cites the legacy `CoSuper` profile names in its seat
eligibility text. The profiles map to engineering (author) and to
management or Jev (reviewer). Rename them when the standing mode lands.

### 1.4 Running the new code: binary and frontend

What a computer runs is decided at boot by `autoputerRuntimeExec`
(`nix/autoputer-vm.nix:90-223`):

- By default it runs the base image's `bin/autoputer` from the EROFS store.
- It runs a layered release's binary when three things hold:
  `current/layering-entrypoint` exists, the release's
  `base_image_manifest_digest` equals the booted base, and the boot-loop
  guard has not tripped.
- The served frontend is the release's `frontend/`.
- A plain release, which is what self-development mints today, carries no
  entrypoint. So the base binary runs, and only the frontend changes.

The design keeps that wrapper and changes what self-development produces:

- **The host builds, the guest applies.** The builder output is the same
  kind of layer the S2 station proved for platform pushes (CI's platform push
  of layers was deleted in a47122de, after version skew and offer bind gaps
  in delivery, not because the layer format was wrong). It
  carries runtime and frontend, so one apply moves both, atomically
  (S2-e: the entrypoint pointer lives inside the release directory).
- **A layer is bound to its base.** The builder builds against the base the
  computer booted. Adopters on the same base reuse the same closure. An
  adopter on a different base needs a rebuild, and if the patch does not
  apply to that base's commit, a rebase candidate. That rebase is an ordinary
  engineering change (S6 addendum, item 3).
- **A platform deploy must not silently drop a computer's own release.**
  Today a new base makes the wrapper fall back to the base runtime with a
  diagnostic line. That is correct for safety and wrong for a diverged
  computer: it loses the owner's change. Two cases:
  - If the new platform *contains* the release (station C below), falling
    back is exactly right.
  - If it does not, the computer is diverged and must be held. Divergence
    must be derived from the tape, not from the manual vmctl flag that
    `deploy_refresh.go:267-276` reads (S6 addendum, items 1 and 2).
- **Planned restart stays planned.** The apply writes the marker with reason
  `self_development_apply` and consumes it once at boot. That is already
  bounded at three starts per apply (`planned_restart.go:215`), so a release
  that crashes on start ends in a crash boot that closes work. No loop. The
  work-state epoch must stay compatible across a self-development release.
  LU2 (the persisted epoch and its migrations) is what makes an
  epoch-bumping release safe, and until LU2 lands, a release that bumps the
  epoch must be refused.

### 1.5 Publish: is a release just another published object?

Yes, for the record. No, for the bytes and for what adoption means.

**What the object graph offers** (read in code):

- Kinds are an open string, but the registry is closed: `CheckWrite` refuses
  any kind that is not in `DefaultRegistry` (`internal/objectgraph/registry.go:64-303`).
  Adding `choir.software_release` (owner: the platform publication writer)
  and two edges is a registry entry plus a writer, which
  `TestEveryRegisteredKindHasAWriter` enforces.
- Three edges from the retired app-package design are still registered and
  unwritten: `package_source_computer`, `adoption_package` and
  `adoption_target_computer` (`registry.go:201-203`). Reuse their names or
  delete them in the same change. Do not leave both.
- Canonical object ids include the owner (`object.go:66-77`). The same patch
  published by two owners therefore has two ids. That is acceptable, because
  a publication is a publisher's attestation, and the record's own content
  digest (§1.1) is the deduplication key.
- Any bound guest may list and get objects through the generic routes
  (`internal/platform/objectgraph_handlers.go:57-60`). Discovery therefore
  needs no new host route at first: `GET /internal/platform/objects?kind=choir.software_release`.

**Bytes do not belong in Dolt rows** (the captures "104 G lesson",
`internal/platform/captures.go:23-36`). The platform-update offer code
already has the right store: corpusd blob staging under
`sha256/platform-update/<sha>` with a 2 GiB cap
(`internal/platform/platform_update.go:300-382`). Generalize that store into
the release store for patches and closures. The other content store,
`internal/filecas`, is sealed per computer and cannot be shared.

**Adoption is not Autopaper-style adoption.** Doctrine says shared objects
are "adopted onto the subscriber's tape". In code, nothing copies an object
onto a reader's tape yet: shared captures are read in place. For software,
the ontology is explicit: "Import/adoption does not accept, materialize, or
route self-development" (`docs/computer-ontology.md`, Computer Lineage).
Adopting a release therefore appends `adoption_requested(release ref)` to the
adopter's tape, and the release then goes through the adopter's own gate.
The object is referenced, not copied.

**Restore boundary.** Store B is outside every computer's restore set. A
computer that restores to a head before its adoption stops running the
release, but the publication stays. That is correct: retraction is a forward
act by the publisher.

### 1.6 Adopt: another computer pulls a release

1. **List.** `choir release list` reads the registry. Management can read it
   too, as data.
2. **Request.** An owner act (`choir release adopt --computer B --release R`),
   or, under B's standing policy, B's management desk. The release id is
   bound to B's computer id and current event head.
3. **Offer minted at take time.** corpusd mints an offer for B's realization
   and head from R's record and R's closure in the release store. The minting
   code is in `internal/platform/platform_update.go:151-262`. Minting when B
   asks, not when CI deploys, closes the S2-g bind gaps ("offer binds a
   different realization", "base event head is stale";
   `docs/archive/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md`). If B's base
   differs from R's build base, the host builder rebuilds R for B's base
   first. If the patch does not apply, adoption refuses with a typed reason,
   and B can open a rebase candidate.
4. **B's gate.** Under B's policy, the verification seats check the record:
   the digests, the builder receipt, the base binding, the publisher's
   receipts and settled precommitments. They also run R's declared test
   command in a B capsule against B's source checkout plus the patch. In the
   first slice the seats only check, and the test run comes in the next
   realism step.
5. **Apply.** The same materializer and updater path as §1.2 step 5, with the
   same events. The decision's authority reference names the policy (or the
   owner) and the release. `ApplyPlatformUpdate`
   (`internal/agentcore/platform_update.go`) already writes
   `EffectAccepted`, materialization, checkpoint and route events for an
   offer. LU5's "one apply path" is the place to make the selfdev, adopt and
   platform kinds a single `ReleaseApply` transaction.

B never receives an executable built by A. B runs what the host built from
the source in R. Refusals happen before B's state changes: an altered patch
or closure hash, a missing blob, a record signature replayed as a platform
offer, or an adoption bound to another computer or head.

### 1.7 The host adopts, and the fleet push

"The host can download it" is true as soon as A publishes, because the host
built R. Making R part of the platform is a different act. It changes the
base image every new computer boots, and through LU it becomes the pending
update of every tracking computer. That is "pushing an update to everyone",
which the owner wants gated by a human. So host adoption has two steps:

- **H1, autonomous: export.** The record becomes a commit on GitHub branch
  `release/<release-id>`:
  - The commit is `base_commit` plus the patch, with trailers
    `Choir-Release: <id>`, `Choir-Operation: <op>`, `Choir-Computer: <id>`
    and `Choir-Bundle: <digest>`.
  - CI runs its checks on the branch. Today `ci.yml` triggers only on
    pushes to `main` and on pull requests into `main` (`ci.yml:16-19`).
    P10 therefore adds `release/**` to the push trigger, a small yellow
    change. Branch CI still does not deploy: the `deploy-staging` job runs
    only on `main`.
  - For the first slices, the export is a CLI command on the owner's
    machine (`choir release export R`), using the owner's existing `gh`
    login. Nothing on a computer or the host holds a GitHub credential.
  - The read-only patch route on `selfdev-patch-export` (c8db35df5) is the
    piece this reuses.
- **H2, human: merge.** The owner merges `release/<id>` into `main`. The
  Landing Loop builds and deploys the platform from that commit, and LU
  carries it to the rings. The merge is the fleet-push approval. GitHub
  branch protection on `main` keeps it human.

The merge commit's SHA differs from R's synthetic commit, and the platform
build embeds the commit in `build.json`, so the bytes will not be identical.
*Unverified* whether the builder's synthetic commit is reproducible (fixed
author and date). Equivalence must therefore be shown by the git **tree**
(R's patched tree equals, or is contained in, the merged tree), not by
commit SHA or closure bytes. That is why the release record carries the tree
hash.

After H2, A and B run R as a layer on the old base. At their next LU update
they boot the new base, which contains R. The wrapper's base check makes
them fall back to the base runtime, and that is now the right code. With
divergence derived by tree containment, they read as tracking, not
diverged.

### 1.8 Identity

The Landing Loop already warns that "platform deploy SHA and computer
effective identity can diverge on purpose". This architecture makes that
divergence routine, so identity must say which release a computer runs.

Today, guest `/health` reports `build`, `release_digest`, the schema and
reducer versions, and `layering_bootguard` (`internal/agentcore/api.go:123-149`).
`/api/acceptance/execution-identity` signs the executable and image digests.
No health surface reports an event head, a CodeRef or a content witness:
those exist only in self-development checkpoints
(`internal/selfdevprotocol/control.go:74-99`).

Needed, small and orange:

- `/health` and `vmctl list` report the effective release id, the release
  tree hash, the booted base digest, the effective event head and the
  CodeRef, next to the target platform commit.
- The checkpoint's content witness stays in the checkpoint. A restore proof
  joins event head, content witness, the VM-local projection and route, as
  LU5 already says.
- CI's "already current" test (`ci.yml:1489-1496`, `.build.commit` equals
  the target) must learn "effective tree contains target tree", or every
  self-developed computer reads as stale forever.

### 1.9 Who writes the code is independent of the update path

The update path starts at Freeze. Any author that produces a frozen bundle
with its receipts works: engineering's own sub-RLM cells on the default
model pools, or the in-capsule Claude delegate. The owner ruled on the
delegate on 2026-10-14 (top of
`docs/reports/claude-agent-sdk-in-choir-2026-10-14.md`): option T now, as
an owner-only, time-bounded exception to I19 in one test capsule. The
delegate is an experiment. Nothing in this architecture depends on it, and
no station below requires it.

**The long-term path: Anthropic models through Choir's own harness.** The
owner's long-term direction is the API, called through the gateway like any
other provider, from Choir's multi-agent harness, never through `claude -p`
or the Agent SDK. In that shape:

- Engineering's capsule cells ask the gateway for the model as they do
  today. The credential stays on the host (I19 holds without an exception),
  and capsule egress stays closed to `api.anthropic.com`.
- Precommitment records, desk messaging and the RLM desks are already the
  harness. The delegate's added value was its tool loop, and the engineering
  desk's cells already supply that.
- The gateway today has a Bedrock provider that speaks the Anthropic
  messages format (`internal/provider/provider.go:342`, `BedrockProvider`;
  Claude models in `internal/modelcatalog/catalog.go`). It has no direct
  Anthropic API provider. Adding one, or using Bedrock, is a provider
  routing change (red) that belongs to the model-policy work, not to this
  path.
- Its cost belongs in the LR cost levers, because an Opus-class engineering
  desk is priced per token.

## 2. GitHub's role

**GitHub is the source of truth for the platform. It is a mirror, not the
source of truth, for computer releases.** The two are different authorities
(`docs/computer-ontology.md`: "Computer evolution, source sharing, and
platform deployment are different authorities"). The rule in CLAUDE.md,
"GitHub `origin/main` is the source of truth for tracked deployed files",
stays true: the host never deploys a release that is not a commit on `main`.
The host registry holds releases that may never reach GitHub, such as a
change that only some computers adopt.

**How a release becomes a commit:** through §1.7 H1. Branch name
`release/<release-id>` (the readiness report's `selfdev/<op>` name keys on
the operation, but a release can outlive its operation and be adopted
widely, so key on the release). The commit is cut from `base_commit`,
rebased onto `main` only when it applies cleanly; a conflicting rebase is a
new engineering candidate, not a hand edit. The repo is public, so the
export must run the Freeze secret scan over the patch, refuse paths git
ignores (`node_modules`, `dist`), and refuse when `base_commit` is not on
GitHub (a computer seeded from a local build). These are the readiness
report's failure modes for export, and they carry over unchanged.

**How human PRs flow back to computers:** a human PR merges to `main`, CI
deploys, and the new platform becomes every tracking computer's pending
update under LU. A computer that runs its own release is diverged and is
held, never overwritten. Its management sees "platform update available,
rebase needed" and opens a rebase candidate, which goes through the same
gate as any change. CI should also publish each deployed platform build into
the same registry: the deploy already writes `platform-release.json`
(`ci.yml:1855`, `cmd/platformrelease`). A platform release is then a release
with an empty patch, published by the platform, and one list and one apply
path serve both.

The SDK report's "Slice 0" flow (the owner's session pushes a branch, and a
computer adopts that commit) remains a valid way to bring outside work in:
it is an adoption of a release whose record was built by the host from a
GitHub commit instead of a capsule.

**CI and staging deploys** stay as they are: `main` only, with the Landing
Loop. Release branches get CI checks but no deploy.

**Deploy SHA versus effective identity.** After this architecture, staging
`/health` at the proxy reports the platform's deployed commit. Each
computer reports its effective release and tree (§1.8). "Current" means the
effective tree contains the platform tree. Two computers can run different
code on one deploy on purpose, and the identity report says which code each
one runs.

## 3. Prerequisites in dependency order

Sizes: S is under a day of agent work, M is one to three days, L is more.
Status: landed, branch (local unless noted), designed, or missing.

**P0. Freeze is reachable.** The capsule produces its release tree: the
running binary is reused when no Go input changed, plus the C toolchain and
2 GiB engineering capsules. `self-dev start` is deleted. Station:
readiness B1 and B0, under C1e and Gate 2. Status: branch `selfdev-land`
(876865b24..afc8a85b9), not on `origin`. Size: S to land, then a red deploy.
Everything depends on it.

**P1. The records-path tail is proven.** Verify, decision, apply, served,
restore, on today's code, with the tracer approving as owner. Station:
readiness B2, Gate 2 track A. Status: implemented, not exercised since C1e
B1. The last pass was M11 rerun 13 on the packet-era path. Size: one tracer
run, plus whatever it finds (expect at least one finding).

**P2. Patch export route.** A read-only route serves `source.patch` from a
frozen bundle with its sha256 and base commit. Status: branch
`selfdev-patch-export` (c8db35df5). Size: S, an orange deploy. The host build
service and the GitHub export both read this route.

**P3. Owner-only arming.** Confirm or refute that the guest can arm its own
mode through `event:append`, and if it can, require an owner credential for
every mode write. Station: new, under S1 remainder or Gate 2. Status:
missing (the hole is *unverified*). Size: S, red (authority). Must land
before P4.

**P4. Standing autonomous acceptance.** A standing consensus mode bound to
a policy digest, a computer and an expiry. A corpusd actuator. Signed
ballots from rule settlements and from management's Verify or Jev. Station:
Gate 2, replacing its "owner decision in product UI" leg for test computers;
C14. Status: policy and reducer landed; mode, actuator and signatures
missing. Size: M, red (canonical event acceptance).

**P5. Go changes run.** A host build service (unit plus guest route through
the proxy) taking `(base_commit, patch)`. The materializer mints a layered
release from the builder's closure, with closure transport through the
release store. Fix layered-checkpoint identity (layered SPA underivable).
Station: S2-f residual and Gate 2 track B, then the S6 full release (S6's
acceptance leg "fetch the changed backend endpoint after activation" is
this). Status: builder and updater landed, wiring missing, checkpoint
identity open. Size: L, red (updater, checkpoint, a new host build surface).
Security note: the host builds an unreviewed, guest-authored patch. It must
build in the Nix sandbox as an unprivileged builder user with no secrets, no
network except fixed-output fetches, and a timeout. That is new attack
surface on the trust root, and a target for station LT.

**P6. Identity reporting.** `/health` and `vmctl list` show the effective
release, tree, base, event head and CodeRef. CI's "current" test learns tree
containment. Station: the Landing Loop's self-development completion
obligation, and LU5's identity item. Status: partial (`release_digest`
only). Size: S to M, orange. Needed by every probe below, so it lands with
P5.

**P7. Release record and registry.** A new object kind, its writer and
publish route, the release blob store (generalized from the platform-update
blobs), and `choir release list|show|publish`. Station: S8, re-scoped (S8's
settled source-only decision stays, read through decision 2). Status:
missing. The object graph, the hashing and the blob store pattern exist.
Size: M, red (shared publication surface, outside every restore set).

**P8. Adoption pull.** A guest route and `choir release adopt`. The offer is
minted at take time for the adopter's head and realization. The host
rebuilds when the base differs. The adopter's gate runs under its own policy
and applies through the same path. Station: S8 adoption, plus the S2-g bind
gaps, plus LU5. Status: missing. The offer, mint and apply code exist for
operator-delivered platform updates. Size: M to L, red.

**P9. Derived divergence.** A computer is diverged exactly when its
effective release's tree is not contained in the platform's. The guest
refuses a platform-follow offer with `divergent_release`. Deploy refresh
reads this instead of the manual flag. Station: S6 addendum, items 1 and 2.
Status: designed. Size: M, red (deployment routing). Needed before any
persistent computer runs its own release across a deploy. Station C's probe
needs the "tracking again" half.

**P10. Release export to a GitHub branch.** `choir release export` on the
owner's machine: secret scan, ignore check, base on GitHub, trailers, push
`release/*`. Station: replaces readiness B3's draft-PR step. Status:
missing. P2 is its read side. Size: S, orange (a CLI command, plus the
`release/**` CI trigger, which is yellow; no deploy).

**P11. LU pull and rings.** The fleet push itself. Station: LU (LU1a and
LU2 reviewed and startable, LU3a after them, LU1b and LU5 drafted). Status:
designed. Size: L across slices, red. Needed for "push to everyone", not for
stations A to C on test computers.

**P12. Desk-cell confinement.** Management, Texture and research cells stop
running as guest root. Station: foundations slot 6. Status: branch
`fix/desk-cell-confinement`, conflicts with `main`. Size: M, red. Needed
before autonomy on any computer whose state matters: an autonomous
reviewer's signing key must not sit where a confused cell running as root
can reach it. On test computers it is a named residual.

**P13. Source seed follows the booted base.** A persistent computer's
`files/Source/platform` stays at its creation commit while its image moves
(readiness §4). Station: new, small, under LU. Status: missing. Size: S to
M. Needed before persistent computers self-develop. Fresh test computers do
not need it.

### Is S7 (app packages) a prerequisite?

**Not for proving the loop.** A release built by the host is a whole app
layer: runtime plus frontend from one flake. The monolith rebuild that S7
avoids is the host builder's job, and Nix caches it. One release can flow
from A to B to the host with no package boundary at all.

**Yes, before the loop is useful for many changes.** Without packages:

- every release is a patch to the whole monolith;
- two independent releases from two publishers conflict as patch stacks;
- adopting "the slides app from computer A" means adopting A's whole
  runtime delta.

S7's per-package manifest, data-compatibility contract and dynamic loading
are what make adoption granular and composable.

So the order is: release record and adoption first (P7, P8), then S7, which
already says "the package is a release member, not a new deployment
channel". The release record reserves a `packages` field. S7-0 (converting
the existing apps, owner priority 2026-10-14) can proceed in parallel. It
touches the frontend registry and the release manifest, not the update
path, as long as it keeps the S6 rule that package discovery is derived
from the accepted release manifest.

### The other updates the brief named

- **Running applied Go changes: P5.** The real blocker. Large, red. On the
  critical path.
- **Identity reporting: P6.** Small, and needed by every probe.
- **Restore: P1.** Implemented, unexercised on the records path. Restore on
  a *layered* computer needs P5's checkpoint identity fix.
- **Fork re-keying: not a prerequisite.** No fork or re-key code exists (S9
  is design only). The proving sequence uses freshly minted test computers,
  each with its own identity material. Forks matter for red teaming (LT) and
  for fleet experiments, later.
- **S4 egress: not a prerequisite** for changes that add no dependency. The
  host Go module proxy covers Go. A change that adds an npm or Go dependency
  is refused at the first slice. Minimal S4 (lockfile-verified registries)
  comes after.
- **S5 preview: not a prerequisite.** The owner cut it from Gate 2 on
  2026-10-10; restore after apply is the safety net.
- **LU2 (persisted work-state epoch): a soft prerequisite.** Until it lands,
  the actuator refuses any release whose epoch differs from the running one.
- **SP (second host): not a prerequisite** for proving. It is a
  prerequisite before releases written by agents reach users who are not the
  owner, because a single host is the only copy of the registry.

## 4. The minimal proving sequence

One landing and three stations, each on freshly minted test computers
(`scripts/qa_test_computer.mjs` or the tracer's own mint), never on the
paper or the owner's computer. Each station extends the engineering tracer
(`scripts/reeng_tracer.mjs`) with a scenario. Its receipt is the artifact.
No platform deploy may run during a station run: the tracer's
`--expect-sha` fence marks such a run contaminated.

### Station 0: Freeze, then the tail, as today (P0, P1)

Land `selfdev-land`. Then:

```
node scripts/reeng_tracer.mjs --label selfdev-ab-<HHMM> --scenario engineering \
  --expect-sha <deployed sha> --timeout-min 30 --apply-min 10 --restore-min 12 \
  --require-served --keep-computer
```

Accepted when the receipt exits 0 with `acceptance.pass: true`, including
`operation_frozen`, `owner_approved` (the tracer as owner of its own
computer), `applied`, `one_planned_restart`, `served_change_then_restored`
and `restore_to_pre_apply_head`. This proves today's path for a frontend
change. Nothing new is designed here.

### Station A: one computer upgrades itself, Go change running, no human act (P3, P4, P5, P6)

Scenario `selfdev-go-autonomous`:

1. The tracer mints computer A and arms the standing autonomous mode as
   owner. That is the only owner act.
2. It records the outside agent's precommitments.
3. It starts, with `choir run start`, a request for a Go change: a new
   guest route `GET /api/selfdev-probe` returning a nonce baked into the
   source, plus a visible frontend label.

Accepted when the receipt shows all of the following:

- the operation reached `applied`, and its decision event's authority
  reference is `consensus:<receipt>` under `reversible-selfdev-v1`, with two
  signed verification ballots and no `external-owner:` reference anywhere in
  the operation;
- no approve call in the tracer's own HTTP log;
- the builder receipt names `code_commit_source=derived-patch` with the
  bundle's patch sha256;
- exactly one planned restart with reason `self_development_apply` and no
  crash boot;
- `GET /api/selfdev-probe` returns the nonce, which the base binary cannot
  do;
- the label is served;
- `/health` reports the effective release id and the layered base digest;
- `choir computer restore` to the pre-apply checkpoint removes both the
  route and the label;
- a second autonomous change applies after the restore (the layered
  checkpoint identity regression).

Negative legs:

- a candidate whose `tests_pass` settles false is not accepted, and the
  operation ends `rejected` with the reduced dissent recorded;
- a mode write with the guest's own credential is refused (P3).

### Station B: a second computer adopts the release (P7, P8)

Scenario `release-adopt`:

1. After station A, A publishes R: automatically after `applied`, or with
   `choir release publish`.
2. The tracer mints computer B on the same deploy and arms B's standing
   mode.
3. It runs `choir release adopt --computer B --release R`. The owner's
   choice per computer is expressed by this owner act; management-initiated
   adoption is the next realism step.

Accepted when:

- `choir release list` shows R with A's operation, bundle and settled
  precommitments;
- B's tape shows `adoption_requested(R)`, a consensus decision, and the
  materialization, checkpoint and route events;
- B serves `/api/selfdev-probe` with A's nonce, and B's `/health` reports
  effective release R;
- B's closure digest equals the host builder's for R. No byte fetched by B
  came from A: the host release store's access log names only corpusd as
  the source;
- restoring B to its pre-adoption checkpoint removes the route, while A
  still serves it.

Negative legs, each refused before B's state changes:

- R with an altered patch hash;
- R whose closure blob is missing;
- an adoption request bound to another computer's head.

### Station C: the host adopts the release, with one human approval (P9, P10, P6)

Steps:

1. `choir release export R` pushes `release/<id>` to GitHub. CI on the branch
   passes.
2. The owner merges. That merge is the single human act of the whole
   sequence, and it stands for the fleet push.
3. The Landing Loop deploys.

Accepted when:

- staging reports the merge commit;
- a freshly minted computer C serves `/api/selfdev-probe` with the nonce,
  with no adoption, from the base image;
- after A's and B's next LU update (or a planned refresh), both report
  "tracking" with effective tree containing R's tree, run the base runtime,
  and still serve the route;
- the merge commit's tree contains R's tree, checked with `git diff` of
  R's synthetic commit against the merge, restricted to R's paths and
  empty.

Negative leg: a computer D, with its own unpublished release applied, stays
held as `divergent_release` through the same deploy and keeps serving its
own change.

After station C the loop is proven: a change was made on a computer, ran
there, was adopted by a second computer and became the platform, with
humans arming two test computers and merging once.

## 5. What to stop or defer

- **Stop the GitHub draft-PR script.** The lead already dropped it. Keep
  P2's read route, which P5 and P10 reuse. The export goes to `release/*`
  branches, not PRs. A PR is the owner's choice at merge time.
- **Do not build approval UI or the changes app** (Gate 2 track E) until
  station C passes. Owner acts stay CLI-only: arming, adopt for now, restore
  and merge. When approvals return, they are a policy change
  (`human_seat: "required"`, using the existing `human-required-v1.json`),
  not a new code path.
- **Retire the per-operation `qualified_consensus` arming flags on
  `self-dev mode set`.** The standing mode replaces them. Keep `accept_once`
  for the owner computer.
- **Stop treating the capsule-built `bin/autoputer` as a release input** once
  P5 lands. It remains a test artifact. Readiness follow-ups C and D (build
  budget for Go changes inside the capsule) drop in priority, because the
  authoritative build moves to the host.
- **Do not revive the CI app-layer push** (deleted in a47122de) or the
  operator-delivered M9a offer as a distribution channel. Both are replaced
  by take-time offers (P8) and LU.
- **Defer S5 preview, full S4 egress, S9 forks and LU4** (binary-only swap).
  None is on the proving path.
- **Defer management-initiated adoption, cross-owner adoption and
  adopter-side rebuild in a capsule** (S8's full source-only proof) until
  after station B.
- **Do not make the update path depend on the in-capsule Claude delegate.**
  It is an experiment under option T (§1.9).

## 6. Open owner decisions

1. **Where autonomy may run.** Recommended default: the standing autonomous
   mode may be armed only on computers in the test tier, until P12
   (confinement) lands and stations A to C pass. The paper and the owner's
   computer keep `propose_only` and `accept_once`. On test computers, both
   verification seats running in the same guest is accepted as a named
   residual.
2. **Host-built bytes for adopters.** On 2026-10-01 the owner settled S8 as
   "source only", meaning the adopter rebuilds and no executable crosses
   between computers. Recommended default: amend the reading, not the rule.
   The host builder is part of the platform trust base, like the base
   image. Adopters run bytes the host built from the published source,
   never bytes another guest built. The adopter's own capsule rebuild stays
   the requirement for cross-owner adoption (S8 proper). This needs the
   owner's ratification, because it reinterprets a settled decision
   (standing question 2).
3. **GitHub's place.** Recommended default: `main` stays the only source the
   host deploys from (CLAUDE.md "Git And Staging" unchanged). Releases reach
   GitHub only as `release/*` branches, pushed from the owner's machine with
   the owner's own credentials. The merge to `main` is the fleet-push
   approval. The alternative is a host that deploys straight from its
   registry without GitHub. It changes a standing operating rule and is
   recommended only after SP gives the registry an off-host copy.
4. **Who may start an adoption.** Recommended default: an owner act per
   adoption through station B. Afterwards, a computer's management may adopt
   under its standing policy, only releases published by computers of the
   same owner, and only when the publisher's settled precommitments include
   `tests_pass` and `verify_pass`.

## Conformance with settled decisions

- **Ontology: "results return to any other computer only through
  publication and adoption, never by direct merge".** Kept: A publishes, and
  B and the host adopt.
- **Ontology: "Import/adoption does not accept, materialize, or route".**
  Kept: adoption is a request, and the adopter's own gate accepts.
- **Restore-set boundary (Store B out).** Kept: the record lives in Store B
  and is outside every restore.
- **Doctrine C14 (a human is a policy-selected seat).** This is the basis of
  the autonomous mode.
- **C15 (the computer's UI is computer surface).** Kept: the frontend moves
  with the layer.
- **CLAUDE.md restart rules.** Kept: one marker per planned restart, and a
  crash boot closes work.
- **CLAUDE.md "Deploys never restart a busy computer".** Kept: the
  fleet push is LU's pull.
- **S8 "source only".** See decision 2.
- **S6 owner approval.** The S6 file's `must_preserve` says "Owner approval
  (or a stronger declared consensus policy) gates every commit". The
  standing policy is the declared consensus policy that rule already
  allows. Whether `reversible-selfdev-v1` is "stronger" is the owner's call
  in decision 1.

## Unverified claims, collected

- The guest can arm its own mode through `event:append` (§1.3, P3). Read in
  code but not exercised.
- The host builder's synthetic commit is reproducible (§1.7). Not checked.
- Jev's verdict on a frozen candidate can serve as a signed ballot. Commit
  32febcf6 is cited by Gate 2's `now` card; I did not read its signing path.
- The generic object routes have no per-object size cap (§1.5). Not checked
  against the server-wide body limit.

## Class, heresy delta, residuals

Class of this report: green. Protected surfaces the proposed work touches:
canonical event acceptance (P4), the updater and the checkpoint (P5), a new
host build surface (P5), shared publication (P7), deployment routing (P9,
P11), and authority for mode writes (P3).

Heresy delta:

- **Discovered.** No code path builds a `ConsensusBundle`, and ballots are
  hashes, not signatures. A guest may be able to arm its own mode
  (*unverified*). The registry still carries three unwritten adoption edges
  from the retired package design. CI's "current" test compares commit SHAs,
  which cannot express a self-developed computer. `reversible-selfdev-v1`
  still names the retired CoSuper profile.
- **Introduced.** None.
- **Repaired.** None (design only).

Residual risks:

- The host builds patches no human reviewed.
- Seat independence is weak while both seats run in one guest.
- A single host holds the only copy of the registry.
- An epoch-bumping release before LU2.
- Releases that add dependencies, refused until S4's allowlist.
