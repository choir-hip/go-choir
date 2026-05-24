# Promotion, Distribution, And Publishing: Theory And Practice For Choir

Date: 2026-05-17
Status: strategy/design report
Operator: Codex

## Executive Thesis

Choir is reaching the boundary where "promotion" can no longer mean one thing.
The system now has working pieces for worker export, verifier-backed source
patch import, owner approval, personal computer mutation, VText publication, and
platform deployment. The hard next move is to separate these into typed
transitions with different authority, evidence, rollback, and distribution
rules.

The deep object is not a merge button. It is a controlled state transition
between ledgers:

```text
private/candidate computer state
  -> typed artifact proposal
  -> verifier contracts and provenance
  -> owner/platform decision
  -> scoped adoption, publication, distribution, or deployment
  -> rollback and continuation evidence
```

The most important correction is this:

- A worker export is a portable claim about candidate work, usually a manifest
  plus patchset or artifact bundle.
- A promotion is acceptance of that artifact into a particular target ledger.
- A distribution is making an artifact available to other computers or servers.
- A deployment is routing live traffic or computers to a selected artifact.
- A publication is a content-side projection from private state into
  platform-visible public memory.

Choir should keep GitHub as the official platform source ledger, but should not
force every personal-computer update, UI bundle, app package, or candidate
artifact through full GitHub Actions plus NixOS switch. Instead, it needs a
promotion controller, artifact registry, signed/attested binary distribution,
per-computer version profiles, and eventually Node A/Node B route-level
cutovers. NixOS remains the host truth for kernel, service hardening, VM image
construction, and platform baseline configuration. It should not be the only
runtime update mechanism.

The stronger version is: keep the platform substrate relatively stable and make
user-level evolution happen inside user computers. A user can request changes to
their runtime Go source or Svelte UI source, build and test the resulting
runtime/UI pair in a background candidate VM, and promote that pair into their
own active computer without changing platform binaries or triggering a platform
deploy. Publishing that source package to other users is a content/source
distribution event, not a host deployment event.

## Current Ground Truth

The current repo already states the right ontology. Choir gives each user a
persistent computer, not a disposable sandbox. A computer combines VM/runtime
state, Dolt/app state, source/build state, blobs, provenance, and route
identity. The canonical rule in [computer-ontology.md](computer-ontology.md)
is:

```text
Promote typed artifacts, not opaque machine accidents.
```

[current-architecture.md](current-architecture.md) and
[runtime-invariants.md](runtime-invariants.md) split promotion into personal and
platform/public paths:

- Personal promotion changes one user's active computer. It can promote a local
  Go binary, Svelte build, app bundle, prompt, package install, Dolt branch,
  file/blob, or generated artifact without global deploy.
- Platform/public promotion changes shared state: official Choir source,
  public packages, publication artifacts, shared app/agent packages, platform
  runtime, or public artifact graph state.

The product path recently proved a meaningful but narrower result. Candidate
`a8822bb7-8d7c-48f0-adba-6799825a72e4` from trajectory
`d6a51c7d-af23-4de5-b00a-0bdd791e46fe` produced an exported patchset, imported
it into a server-owned promotion workspace, ran verifier contract
`product-safe-patch-import`, recorded owner approval, and produced promotion
commit `120b21e29c8e4f437e87b60fbb6025a70dc41c53`. The report is at:

```text
.gstack/evidence/mission-platform-promotion-substrate-v0-20260517T172324Z/
```

That commit was not pushed to `origin/main`; local `main` remained at
`c51d66847383d3c295537eabbcbab7346d4bee2b`. This is not a failure. It exposes
the precise frontier: Choir can now promote into a user/candidate workspace
with product evidence, but does not yet have a full platform distribution and
deployment substrate for that candidate.

## Cognitive Transform Pass

Current uncertainty or obstacle:

How should Choir convert private or candidate computer changes into stable,
shared, public, or platform changes without collapsing personal freedom,
security, consent, uptime, provenance, and rapid iteration into one brittle
GitHub/NixOS deployment path?

Selected transforms:

1. Depth Extraction / Esoteric Upgrade. The banal version is "merge the
   candidate." The deep version is "perform a typed state transition under
   evidence and authority." The load-bearing variable is not the diff; it is the
   promotion certificate.
2. Boundary Transform. Move the boundary between user computer, candidate
   computer, platform source, artifact registry, and server route. Many apparent
   conflicts disappear when the target ledger is explicit.
3. State Machine Transform. Name states and stuck states. Exported, verified,
   approved, distributed, deployed, adopted, rolled back, and published are
   different states.
4. Security / Adversarial Transform. Every executable promotion is a supply
   chain boundary. Candidate computers can be compromised, stale, wrong, or
   locally useful but globally inappropriate.
5. Systems / Controller Transform. Node A/B, verifier contracts, Trace, VText,
   CI, binary cache, and route pointers are parts of one release controller:
   sensors, actuators, policy, and rollback.

Route-changing insights:

- The next durable object should be a typed `PromotionProposal`, not a generic
  patchset or PR.
- GitHub is the official source ledger for platform code, but distribution can
  be an artifact ledger with provenance and signatures.
- Direct binary distribution is legitimate for speed, but only inside a trust
  region. Platform-global binary promotion should require source provenance,
  trusted builder provenance, or a deliberate emergency policy.
- Node A/B only helps uptime after state ownership is explicit. Without platform
  Dolt replication, artifact replication, auth/session continuity, and VM route
  ownership, Node A/B can improve stateless edge deploys before it can safely
  migrate active user computers.
- Content publishing is easier because published content is inert and immutable.
  But its exact-ref, artifact-manifest, provenance, consent, and rollback model
  is the seed for code promotion.

Changed plan:

- Implementation: build a promotion/distribution control plane around typed
  artifacts and release/adoption records before optimizing the podcast product
  candidate.
- Verifier/evidence: require manifest digests, source/build provenance,
  verifier contracts, install plan, rollback plan, and Trace/run-acceptance refs
  for every executable artifact.
- Scope: start with source patch, Svelte frontend bundle, Go service binary, and
  VText publication artifacts. Defer OS/kernel and arbitrary VM snapshot
  promotion until the ledgers are mature.
- Stopping condition: staging can take one worker export, produce a reviewable
  artifact proposal, build or import the correct artifact type, verify it,
  publish/distribute it through a product API, adopt it in a scoped target, and
  expose rollback evidence.

Next high-information action:

Create a unified promotion artifact schema and controller that can represent
the already-working VText publication path and the harder source/build/binary
path without pretending they have the same risk.

## The Current Source Promotion Path

The source/build code is centered on [internal/shipper/shipper.go](../internal/shipper/shipper.go),
[internal/promotion/promotion.go](../internal/promotion/promotion.go), and
[internal/runtime/promotion_queue.go](../internal/runtime/promotion_queue.go).

`shipper.ExportPatchset` requires:

- clean repo;
- non-empty delta from `base_sha`;
- `base_sha` is an ancestor of worker `HEAD`;
- manifest fields: run id, trace id, VM id, base SHA;
- binary git diff patchset;
- optional checks recorded in the manifest and export report.

`promotion.PrepareIntegrationCandidate` imports that patchset into an isolated
integration branch and runs verifier contracts. It does not mutate the
destination branch.

`Runtime.VerifyPromotionCandidateInWorkspace` is the product-safe verifier path:
browser callers provide only owner/candidate ids; the server chooses the source
repo and workspace root. It currently uses product contract
`product-safe-patch-import`, whose required check is:

```text
git diff --check HEAD~1..HEAD
```

`PromotePromotionCandidateInWorkspace` then applies an approved verified
candidate inside the server-owned workspace. This is why the recent proof was
real promotion-level evidence for a product path, but not a global platform
deployment. The canonical branch that moved was the workspace target, not
`origin/main`.

This is the right local primitive. The next problem is making its output travel
as a platform artifact.

## The Current Publication Path

The content publishing path is ahead in several respects. The relevant code is
[internal/platform/service.go](../internal/platform/service.go),
[internal/platform/types.go](../internal/platform/types.go), and
[internal/proxy/platform_publish.go](../internal/proxy/platform_publish.go).

`PublishVText` already does many things the code promotion system will need:

- reads a selected private VText revision through the proxy;
- checks ownership;
- computes content hash, source revision hash, projection hash, and route path;
- writes the content blob to platform artifact storage;
- records artifact manifests, blobs, retrieval sources, spans, citations,
  consent records, review records, provenance entities/activities/edges,
  verifier attestations, rollback refs, and public routes in platform Dolt;
- commits the platform Dolt transaction.

Content publishing is easier because the artifact is inert. A published VText
snapshot can be immutable, routeable, searchable, cited, proposed against, and
disabled without executing arbitrary code. That simplicity is why it is the
right model to recurse from.

But content publishing is not a toy. It already contains the conceptual pieces
that code promotion needs:

```text
selected private ref
  -> hash/projection
  -> artifact manifest/blob
  -> consent/review
  -> provenance
  -> route
  -> rollback ref
```

Executable promotion should reuse that shape, then add build, compatibility,
runtime, deployment, and supply-chain constraints.

## Why Code And Binary Promotion Is Harder

Content publishing moves inert state across a trust boundary. Code promotion
moves behavior across a trust boundary.

The extra risks are:

- Execution risk. A binary or UI bundle can run with user data, provider access,
  app APIs, filesystem access, network access, or future privileges.
- Pairing risk. The Go runtime and Svelte UI are not independent binaries. They
  both contain app surfaces and must agree on routes, schemas, component
  contracts, capabilities, and feature flags.
- Compatibility risk. A Go runtime, Svelte UI, app package, or guest image can
  assume schemas, routes, APIs, browser assets, VM capabilities, or migration
  state that differ across users.
- State migration risk. A publication can be hidden. A bad runtime may mutate
  private Dolt, files, prompts, app state, or foreground-tail updates before
  rollback.
- Multi-version risk. Different users and candidate computers will need
  different versions. The platform cannot assume one global binary.
- Provenance risk. A candidate-built binary is not equivalent to a trusted
  builder output. For platform-level distribution, the builder identity,
  source inputs, dependency graph, and artifact digest must be verifiable.
- Uptime risk. Today Node B carries staging. Node A exists but is not in the
  route/release loop. Full host switches and vmctl restarts are expensive and
  visible.
- Authority risk. "Works for my computer" is not permission to become the
  default platform.

The result is a tiered policy:

- VText/content publication: first safe target is the platform public ledger.
  Evidence is owner consent, hashes, route, and rollback. Fast path is allowed.
- Reader/content proposal: first safe target is the author review queue.
  Evidence is source refs, transclusions, and delivery record. Fast path is
  allowed.
- Source patch: first safe target is a user workspace or platform PR. Evidence
  is patch manifest, verifier contracts, and review. Fast path is allowed for
  personal promotion and gated for platform promotion.
- Svelte UI bundle: first safe target is a user computer or staging channel.
  Evidence is source commit, build provenance, asset digest, and visual checks.
  Fast path is allowed only when signed and scoped.
- Go service binary: first safe target is a user computer or staging channel.
  Evidence is source commit, build provenance, and API/schema checks. Fast path
  is allowed only when signed and scoped.
- Guest image: first safe target is staging or opt-in ring. Evidence is closure
  digest, kernel/rootfs provenance, and boot/health proof. Fast path should be
  cautious.
- Host NixOS config: first safe target is Node/system generation. Evidence is
  source commit, Nix closure, switch proof, and rollback proof. No shortcut.
- Opaque VM snapshot: first safe target is archival or rollback. Evidence is
  snapshot id, owner, and state lineage. Do not treat this as a semantic merge.

## The App Transfer Problem

There is a deeper complication than "which binary does this computer run?"
Choir apps live across at least two executable surfaces:

```text
runtime Go binary
  -> APIs, appagent tools, schemas, permissions, background jobs

Svelte UI bundle
  -> windows, panels, routes, components, controls, client contracts
```

Those surfaces must match. If user A creates a new app, Choir cannot safely copy
user A's runtime/UI binaries into user B's computer. That would overwrite all of
user B's independent runtime/UI differences. It would also smuggle A's unrelated
changes into B's computer.

The transferable object must instead be a source-level app package:

```text
AppSourcePackage
  app_manifest
  runtime_source_delta
  ui_source_delta
  schema_migrations
  capability_requests
  app_protocol_contract
  tests
  verifier_contracts
  source/provenance refs
  compatibility constraints
```

The recipient computer then imports the package into a candidate world and
builds a new matched pair:

```text
user B current runtime source + app runtime delta -> B runtime candidate binary
user B current UI source      + app UI delta      -> B UI candidate bundle
matched pair verifier contracts
owner review
adoption into B runtime profile
```

This is source distribution plus local rebuild, not binary sharing. The agentic
engineering system is a fit for this because it can rebase, repair conflicts,
write glue code, run tests, and produce a reviewable candidate pair in a
background VM. But the product object must make that explicit. "Install app"
really means:

1. import a source package;
2. rebase it onto this computer's current runtime/UI sources;
3. resolve conflicts in a candidate computer;
4. compile both runtime and UI artifacts;
5. run API/UI/app contract verification;
6. adopt the matched artifact pair or block with precise conflict evidence.

This reframes app sharing. A public app registry should not primarily store
user-built binaries. It should store app source packages, compatibility
metadata, verifier contracts, and optional prebuilt artifacts for known base
profiles. Prebuilt artifacts are accelerators when the recipient profile
matches; source packages are the durable truth when it does not.

## Branch/Fork Model For User Computers

Every user computer needs source lineage. But the default product path should
not require the user to understand GitHub, own a GitHub account, or maintain a
personal fork. Choir can use Git infrastructure behind the product boundary.

The right default unit is a branch or ref per **computer**, not merely per user:

```text
platform main
  -> user A computer source ref
      -> user A candidate refs
  -> user B computer source ref
      -> user B candidate refs
```

Users can have more than one computer, and each computer can diverge. Candidate
worlds then fork from the active computer ref, not from global `main` unless the
target is platform development.

The default storage target can be a Choir-owned private GitHub repository or
Git remote:

```text
choir/go-choir              official platform source ledger
choir/go-choir-computers    private source ledger for user computer refs
```

Using the main repo's branch namespace is acceptable only as an early shortcut
if the repo is private, workflows are filtered to `main`, user branches are
excluded from deploy, and branch/ruleset policy prevents accidental platform
promotion. A separate private source-ledger repo is cleaner because it avoids
polluting the platform repo with user branch sprawl and reduces the blast radius
of branch protection mistakes.

The user-facing model should still be simple:

```text
My computer has its own source lineage.
Installing an app creates a candidate.
If the candidate verifies, my computer advances.
```

Advanced users can later export to their own GitHub fork if they want
sovereignty, portability, or public open-source collaboration. That is an export
option, not the default requirement.

Important boundaries:

- Store source/build state in Git refs.
- Do not store private VTexts, secrets, provider credentials, uploaded files,
  generated media, or arbitrary user data in Git branches.
- Store large blobs in content-addressed artifact storage.
- Store semantic product state in per-computer Dolt or platform Dolt, depending
  on whether it is private or public.
- Store source-package proposals as small, typed deltas that can be rebased
  onto another computer's branch.

This branch model solves the user A to user B app problem:

```text
user A app work
  -> extract app source package delta
  -> publish package manifest and verifier contracts
  -> user B candidate branch from user B active computer ref
  -> apply/rebase app package
  -> compile matched B runtime/UI pair
  -> verify
  -> advance user B active computer ref
```

No binary or full branch from user A overwrites user B. The transferable object
is the app package delta; the recipient computer produces its own verified
runtime/UI pair.

## Stable Platform, Divergent Computers

The platform-level deployment loop should remain conservative:

```text
platform source/config change
  -> GitHub main
  -> CI
  -> NixOS build/switch
  -> Node deploy
  -> staging proof
```

Use this path for host behavior, security boundaries, service definitions,
gateway/provider plumbing, vmctl protocol changes, NixOS configuration,
platform Dolt service shape, default product contracts, and other shared
substrate behavior.

User-level evolution should not require that path:

```text
user request
  -> active user computer source ref
  -> candidate/background VM source ref
  -> runtime Go source patch + Svelte UI source patch
  -> build matched runtime/UI artifacts
  -> verifier contracts
  -> route/adopt into that user's active computer
  -> rollback profile retained
```

If the user publishes the source change, that is a platform-visible source
artifact event:

```text
user source package
  -> public/private package manifest
  -> provenance and verifier contracts
  -> searchable/installable registry entry
```

Another user can pull that source package into their own candidate computer,
rebase it onto their own divergent source ref, build their own matched
runtime/UI pair, verify, and adopt. The platform binaries stay stable. The
platform does not redeploy merely because one user published source text and
another user imported it.

This makes source packages closer to content publications than platform
deployments. They are text/artifact publications with richer verifier contracts.
The expensive host deploy path is reserved for the substrate that makes those
per-computer updates safe.

### The Platform Computer Exception

There is one important exception: the public/logged-out surface is itself a
computer-like product object. Today it may be implemented as the platform
Svelte shell plus platform services, but conceptually it should evolve toward a
**platform computer**:

```text
platform computer
  -> public desktop/newspaper/radio surfaces
  -> publication readers
  -> logged-out exploration
  -> platform-owned apps and routes
```

That platform computer can initially be admin-controlled. Updating it should not
always require a host-level NixOS deploy. It can follow the same candidate
computer process:

```text
admin/platform proposal
  -> platform computer candidate
  -> build runtime/UI pair
  -> verify public routes and rollback
  -> promote platform computer route
```

Over time, control of the platform computer can become richer: admin approval,
maintainer review, delegated reviewers, consensus, multiple platform computers,
channels, or public governance. But the invariant stays the same: host deploys
update the substrate; platform-computer promotion updates the public product
surface.

This also creates a practical Node A/B strategy. Node A/B can first serve stable
host substrate and route to selected platform/user computers. The thing that
changes often is not the host OS; it is which verified computer/artifact profile
a route points at.

## External Concepts Worth Importing

The Update Framework is useful because it treats software updates as an
adversarial distribution problem. Its core pattern is signed metadata over
opaque target files, multiple roles, threshold trust, snapshots, timestamps, and
protections against rollback, freeze, mix-and-match, endless data, and wrong
software installation attacks. Choir does not need to implement full TUF first,
but it should import the mental model: clients install target files only after
signed, timely metadata says those exact files are authorized.

SLSA is useful because it distinguishes levels of build provenance. Build L1 is
provenance exists; Build L2 adds signed provenance from a hosted build platform;
Build L3 requires hardened build controls. Choir should target L1 immediately
for all release artifacts, L2 for platform binaries, and gradually approach L3
for high-risk self-development artifacts.

Sigstore/cosign and GitHub artifact attestations are useful because they make
artifact signatures and provenance practical for binaries, containers, SBOMs,
and blobs. GitHub Actions can generate attestations for binaries and images, and
cosign can sign files/blobs as well as container artifacts.

OCI is useful because its manifest model is content-addressed and descriptor
oriented. Choir does not have to use OCI for everything, but artifact manifests
should follow the same discipline: media type, digest, size, annotations,
subject, and linked blobs.

Nix is useful because closures and generations already encode a strong versioned
system model. Nix binary caches and `nix copy` can avoid rebuilding on deploy
targets. NixOS generations also give host rollback semantics. The bottleneck is
not Nix as a concept; it is using full host rebuild/switch as the only delivery
path for changes that could be service or bundle artifacts.

Blue/green and canary deployment patterns are useful because they separate
building a release from routing production traffic to it. AWS and Google Cloud
both emphasize deploying a new version beside the old one, validating it, then
shifting traffic progressively or atomically with rollback. Node A and Node B
should become Choir's physical expression of that pattern, but only after state
ownership and replication are explicit.

W3C PROV, Web Annotation, DataCite, IPFS-style content addressing, and Dolt's
versioned SQL model remain directly relevant to the publication side. They also
suggest that code promotion should keep normalized provenance and exact refs
rather than burying everything in a log paragraph.

## Proposed Architecture

### 1. PromotionProposal As The Common Object

Create a first-class platform object, conceptually:

```text
PromotionProposal
  proposal_id
  proposal_kind
  source_owner_id
  source_computer_id
  source_run_id
  source_trace_id
  base_refs
  candidate_refs
  artifact_manifest_id
  verifier_contract_ids
  verifier_result_ids
  consent_policy_id
  review_state
  target_scope
  distribution_state
  adoption_state
  rollback_refs
  residual_risks
  created_at
  updated_at
```

`proposal_kind` should be a closed initial set:

- `vtext_publication`
- `publication_derivative_proposal`
- `source_patch`
- `app_source_package`
- `frontend_bundle`
- `go_service_binary`
- `matched_runtime_ui_pair`
- `guest_image`
- `app_package`
- `agent_package`
- `nix_closure`
- `computer_route_switch`
- `platform_computer_candidate`
- `platform_computer_route_switch`

The existing publication proposal tables and promotion candidate queue should
not be deleted. The practical route is to wrap them with a common product API
and slowly converge schema names as the system stabilizes.

### 2. ArtifactManifest As The Shared Payload Contract

Every proposal should point to an artifact manifest. The content publication
manifest already has the right seed. Generalize it:

```text
ArtifactManifest
  schema
  artifact_kind
  subject_kind
  subject_id
  media_type
  digest
  byte_size
  storage_ref
  source_refs
  build_refs
  runtime_targets
  capability_profile
  compatibility
  install_plan
  rollback_plan
  verifier_contracts
  attestations
  residual_risks
```

For a VText, `install_plan` is "activate public route." For a frontend bundle,
it is "point selected computer or staging channel at asset digest." For a Go
service binary, it is "install service bundle in selected runtime profile and
restart only the affected service." For a Nix closure, it is "switch system
generation or activate existing closure."

This schema keeps code promotion and content publishing in one conceptual
family without pretending that all artifacts are equally risky.

### 3. GitHub Source Ledger Plus Artifact Distribution Ledger

GitHub should remain the official platform source ledger for tracked code and
config. Platform-global code changes should eventually correspond to a GitHub
commit, PR, or signed source revision. This protects review, history, and CI.

User/computer source refs should be separate from platform release refs. In the
near term they can live in a Choir-owned private GitHub repo or remote so the
product can use GitHub infrastructure without asking users to use GitHub. In the
long term, advanced users should be able to export or mirror their computer
source lineage to a fork they control.

But distribution should move through an artifact ledger:

```text
GitHub source commit
  -> trusted builder
  -> build artifacts
  -> SLSA/in-toto provenance
  -> signature/attestation
  -> artifact registry/cache
  -> promotion proposal
  -> scoped adoption/deployment
```

For personal computers, the ledger can be looser:

```text
candidate computer source/build state
  -> local artifact manifest
  -> verifier contracts
  -> user approval
  -> adopt into that user's active computer
```

Direct candidate-built binaries should be allowed only inside a narrow trust
region: one user, one candidate, explicit rollback, no platform-default
distribution. To become platform-wide, the system should rebuild from source in
a trusted builder or obtain equivalent provenance.

### 4. Binary Fast Path Without Losing Source Truth

The user intuition is correct: a Go runtime or Svelte UI update for one
computer should not require full GitHub Actions plus NixOS rebuild switch. The
safer default is not even "copy signed binaries between users"; it is "publish
source/package deltas, rebase them into the recipient computer, build a matched
runtime/UI pair there, and update that computer's runtime pointer after
verification."

Recommended fast paths:

- User-computer app/source package: publish source deltas and contracts, then
  rebuild in the recipient candidate computer. This is the default app sharing
  path.
- User-computer frontend/runtime pair: build inside that computer's candidate
  VM or trusted per-computer builder, then adopt by runtime profile pointer.
- Platform computer frontend/runtime pair: build in a platform computer
  candidate, verify public routes, then switch the public platform-computer
  route.
- Platform host service binary: build with reproducible inputs, sign/attest
  digest, install under `/var/lib/go-choir/releases/<digest>/`, atomically
  switch a service profile pointer, restart affected service only. Keep old
  digest and service unit rollback command.
- Guest image: build once, copy image blobs by digest, install under a versioned
  image directory, then make vmctl choose image version per computer or channel.
  Restart only VMs that opt into the image.
- Host NixOS config: keep full NixOS switch. This is the right tool for kernel,
  firewall, service definitions, hardening, system packages, and boot-level
  state.

This implies a product-facing distinction between:

```text
platform baseline release
platform computer promotion
personal computer package adoption
staging channel rollout
global production deploy
```

The same artifact may move through several of those states over time.

### 5. NixOS Should Become The Baseline, Not The Only Release Bus

The current GitHub Actions workflow already contains useful optimizations:

- docs-only and top-level markdown commits are ignored;
- runtime tests are sharded;
- host NixOS and guest image builds run in parallel;
- host switch is skipped when the host closure is unchanged;
- guest images are installed separately;
- vmctl is restarted after guest image install.

The next throughput improvement is to build closures/artifacts before the
deploy target and make Node B fetch them. Nix supports binary cache
substitution, and the Nix manual describes serving/fetching store paths via the
binary cache mechanism. Choir can use a private cache, `nix copy`, or a
builder-to-node transfer path so Node B spends less time compiling.

Longer term:

- Host NixOS switch should occur only when host configuration or closure changes.
- Service binary and frontend bundle changes should update release pointers and
  restart minimal services.
- Guest image changes should not force all active user computers to restart.
- Per-computer runtime profiles should declare which artifact digests they run.

### 6. Node A / Node B As A Release Controller

Node A should not simply mirror Node B blindly. It should become a controlled
deployment target with an explicit role:

```text
build artifact
  -> deploy to inactive node
  -> health and identity checks
  -> smoke/product proof on inactive route
  -> traffic/route cutover
  -> watch metrics
  -> rollback to previous node or artifact digest
```

Initial scope should be stateless or near-stateless:

- frontend assets;
- proxy/auth/gateway binaries after session/key continuity is proven;
- platformd reads after platform Dolt primary/standby design is proven;
- public publication reader surfaces.

Harder scopes should wait:

- active user VM ownership;
- vmctl state migration;
- per-user embedded Dolt storage;
- candidate computers mid-run;
- provider gateway credentials and run memory.

The first real Node A/B objective should be "public edge and platform reader
zero-downtime deploy," not "migrate every active computer."

### 7. Per-Computer Version Profiles

Many different binaries at once is not an edge case. It is the product.

Each computer should eventually have a version profile:

```text
ComputerRuntimeProfile
  computer_id
  platform_base_id
  frontend_bundle_digest
  sandbox_binary_digest
  vmctl_protocol_version
  app_package_versions
  agent_package_versions
  guest_image_digest
  schema_read_version
  schema_write_version
  adopted_proposals
  rollback_profile_id
```

This lets a user computer opt into a new podcast app package, UI bundle, or
runtime binary without making it the platform default. It also lets platform
operators answer:

- which users run artifact X?
- can artifact X write schema version N?
- can we roll this computer back?
- which artifacts depend on prompt/tool contract Y?
- which computers are eligible for a new platform base?

### 8. Verification Contracts Need To Become Artifact-Specific

The current source promotion verifier contract is intentionally narrow:
`git diff --check HEAD~1..HEAD`. That is good as a substrate proof, but too weak
for real platform promotion.

Verifier contracts should be typed:

- source patch: `git apply`, `git diff --check`, Go tests, frontend build,
  lint, changed-file policy, security scan where relevant;
- frontend bundle: build provenance, asset digest, Playwright screenshots,
  mobile/desktop DOM metrics, route smoke;
- Go service binary: unit tests, API contract tests, health endpoint,
  migration/downgrade policy;
- guest image: boot health, gateway isolation, vmctl lease/resume tests,
  provider-secret absence;
- VText publication: ownership, immutable revision, content/projection hash,
  no private ref leakage, public route resolves;
- app/agent package: permission/capability review, schema install, rollback,
  prompt/tool boundary tests.

The verifier result should be product-visible in Trace and should be machine
readable enough for run acceptance synthesis.

## What This Means For The Podcast App

The podcast app is important, but it should be pressure applied to the promotion
system, not the primary substrate. A podcast discovery/playback improvement is a
good candidate only after the platform can answer:

- is this a user-local app package, a platform app update, or a publication
  feature?
- does it need server-side search provider credentials?
- is its feed index user-private, platform-public, or derived cache?
- can one user adopt it without changing all users?
- can it be rolled back without deleting subscriptions or playback history?

A narrow podcast candidate should probably start as:

- provider-agnostic podcast search interface;
- one free provider adapter or configured-provider fallback;
- durable feed import/subscription in user state;
- one playable episode path;
- no platform-default adoption until package promotion exists.

That candidate is useful because it touches app package, content index,
playback, user state, and publication/radio continuity. But doing it before the
promotion substrate is real will create another candidate that works locally and
cannot travel.

## How Code Promotion Can Improve Content Publishing

The recursion goes both directions. Content publishing teaches code promotion
about exact refs, artifacts, consent, provenance, and rollback. Code promotion
will teach content publishing about channels, staged rollout, verification
contracts, proposals, compatibility, and multi-version adoption.

Useful content-side upgrades borrowed from code promotion:

- publication channels: private draft, unlisted, public stable, subscriber,
  experimental edition;
- verifier contracts for claims/citations, not just service-level attestation;
- rollback as route disable, supersession, retraction, or corrected edition;
- proposal queues with explicit accept/reject/merge semantics;
- signed/exportable publication bundles that can move between Choir instances;
- reader-visible provenance summaries backed by machine-readable PROV-like
  records;
- per-surface version profiles for automatic newspaper, radio, and public
  domain routes.

The automatic newspaper should become an evolving public memory system, not a
flat CMS. A code-grade promotion model will help it preserve provenance and
quality as it grows more agentic.

## Recommended Roadmap

### Phase 0: Name The States

Document and expose state transitions:

```text
candidate_started
exported
queued
verified
approved
promoted_to_workspace
packaged
distributed
adopted_by_computer
promoted_to_platform_computer
deployed_to_node
published
rolled_back
superseded
rejected
blocked
```

This alone will reduce confusion in VText, Trace, and run acceptance.

Also name source lineage states:

```text
computer_ref_created
candidate_ref_created
package_delta_extracted
package_delta_applied
matched_pair_built
computer_ref_advanced
```

### Phase 1: Unified Promotion Artifact Records

Add platform/runtime records for:

- artifact manifests;
- promotion proposals;
- verifier contracts/results;
- adoption records;
- release channel pointers;
- rollback profiles.

Wrap the existing promotion candidate queue and publication proposal tables
rather than replacing them immediately.

### Phase 2: Trusted Build And Attestation For Binaries

Add CI/trusted-builder outputs for:

- Svelte frontend bundle;
- Go service binaries;
- maybe guest images.

Generate artifact digests, SBOMs where practical, and SLSA/in-toto-style
attestations. Use GitHub artifact attestations or cosign as the initial
pragmatic mechanism.

### Phase 3: Fast Staging Binary Deploy

Teach staging to adopt signed artifacts without full NixOS switch when host
config is unchanged:

- frontend asset pointer update;
- minimal service binary restart;
- guest image digest install;
- health/build identity endpoint reports artifact digests and source refs.

Keep the full NixOS path for host changes.

### Phase 4: Node A/B Public Edge Rollout

Use Node A for inactive-node deploy of public/edge surfaces:

- deploy artifact to Node A;
- run health and product proof;
- route a preview host or low-risk traffic;
- cut over public edge;
- keep Node B as rollback until bake window closes.

Do not migrate active user computers until VM ownership and state replication
are modeled.

### Phase 5: Platform Computer Promotion

Treat the public/logged-out surface as a platform computer rather than only as
host-deployed static UI:

- create admin-controlled platform computer source refs;
- run public-surface changes through platform-computer candidates;
- verify publication readers, logged-out exploration, auth-on-mutation, and
  rollback;
- switch the public route to the promoted platform computer profile;
- keep host NixOS deploys for substrate changes.

### Phase 6: Personal Computer Package Adoption

Let a user computer adopt a signed frontend/app/runtime artifact through a
product-visible promotion review:

- show artifact manifest and verifier results;
- for apps, import source deltas and build a matched runtime/UI artifact pair
  against that computer's current sources;
- update that computer's runtime profile only after the pair verifies;
- switch route/service pointer atomically;
- retain rollback profile;
- record VText/Trace/run acceptance evidence.

This is the capability that makes "automatic computer" self-development feel
real without risking the whole platform.

### Phase 7: Platform Global Promotion

Only after the scoped path works:

- convert candidate source patches into PRs or direct signed commits;
- require CI, platform verifier contracts, staging proof, owner/platform review;
- distribute signed artifacts;
- progressive Node A/B or channel rollout;
- keep rollback and incident evidence.

## Next Mission Candidate

Proposed mission file:

```text
docs/mission-promotion-distribution-control-plane-v0.md
```

Proposed goal string:

```text
/goal Run docs/mission-promotion-distribution-control-plane-v0.md
as a Codex-operated MissionGradient mission: build the first product-safe
promotion/distribution control plane for Choir. Start from the existing
worker export -> promotion candidate -> server-owned workspace promotion path
and the platform VText publication artifact path. Define and implement a typed
PromotionProposal/ArtifactManifest/AdoptionRecord layer that can represent
source patches, VText publications, app source packages, frontend bundles, Go
service binaries, and matched runtime/UI artifact pairs without conflating
personal promotion, platform/public promotion, distribution, deployment, or app
installation. Give every user computer a Choir-managed source lineage ref by
default, preferably in a private source-ledger remote separate from the platform
release repo; GitHub user forks are optional export/sovereignty targets, not a
default requirement. Keep the platform host substrate relatively stable and
reserve GitHub main + CI + NixOS rebuild/switch for shared substrate behavior.
Treat app sharing as source-level runtime+UI deltas plus contracts that are
rebased, repaired, compiled, and verified inside the recipient computer's
candidate world; do not copy user A's binaries or full branch over user B's
divergent runtime/UI pair. Model the public/logged-out surface as a platform
computer that can itself receive admin-controlled candidate promotions without
host redeploy unless the substrate changes. Preserve GitHub as the official
platform source ledger while adding signed/digest-addressed artifact records
and verifier contracts suitable for fast scoped adoption. Expose the proposal,
artifact, verifier, rollback, source-lineage refs, and adoption evidence through
product APIs and Trace; synthesize run acceptance from those records. Use a
narrow podcast discovery/playback or Trace-readability change only as the
pressure candidate after the substrate exists. Land required fixes through
git/CI/deploy when platform substrate behavior changes; prove on staging
through the visible prompt bar and product APIs. Stop only with VText, Trace,
run-acceptance, artifact manifest, verifier, rollback, source-lineage, platform
computer route, and adoption/distribution evidence, or with a precise blocker
after root-cause probes and the next executable probe.
```

High-value implementation slice:

1. Add schema/types for promotion artifact manifests and adoption records.
2. Create product API to inspect one promotion candidate as a typed proposal.
3. Add artifact digest/provenance fields to source patch promotion records.
4. Add a no-op or workspace-scoped adoption record after current promotion.
5. Surface the full chain in Trace and run acceptance.
6. Only then run a small podcast or UI candidate through the path.

## Open Questions

- What is the first artifact registry backend: platform Dolt plus filesystem
  blobs, OCI registry, Nix binary cache, or a thin abstraction over all three?
- Should frontend bundles be served from Nix store paths, platform artifact
  storage, or versioned `/var/lib/go-choir/releases` symlinks?
- What is the minimum acceptable provenance for user-local binary adoption?
- When does a personal computer artifact become publishable to other users?
- How should platform source patches become GitHub PRs or commits from inside
  the product path without handing browser callers arbitrary repo authority?
- What state must be replicated before Node A can serve active private
  computers, not just public/edge surfaces?
- What verifier contracts are required before a candidate can mutate user
  foreground state, not just source files?
- Which rollback operations are safe after a promoted runtime has already
  written new state?

## Practical Recommendations

1. Keep the GitHub/CI/NixOS deployment loop for official platform source and
   host behavior changes.
2. Add an artifact distribution loop for signed, digest-addressed frontend,
   binary, app, agent, and content artifacts.
3. Treat Node A/B as route-level deployment infrastructure, not a substitute for
   artifact provenance.
4. Let personal computers adopt scoped artifacts before global promotion.
5. Require trusted-builder provenance before platform-global binary adoption.
6. Continue using content publication as the simpler proving ground, but allow
   code promotion requirements to enrich publication proposals, channels, and
   rollback.
7. Do not make the podcast app the next substrate. Use it as a pressure test
   after promotion/distribution records can carry it.

## Sources

Local Choir sources:

- [computer-ontology.md](computer-ontology.md)
- [current-architecture.md](current-architecture.md)
- [runtime-invariants.md](runtime-invariants.md)
- [promotion-consent-layer-research-2026-05-15.md](promotion-consent-layer-research-2026-05-15.md)
- [platform-dolt-publication-retrieval-citation-research-2026-05-16.md](platform-dolt-publication-retrieval-citation-research-2026-05-16.md)
- [publication-path-skeleton-2026-05-12.md](publication-path-skeleton-2026-05-12.md)
- [internal/shipper/shipper.go](../internal/shipper/shipper.go)
- [internal/promotion/promotion.go](../internal/promotion/promotion.go)
- [internal/runtime/promotion_queue.go](../internal/runtime/promotion_queue.go)
- [internal/platform/service.go](../internal/platform/service.go)
- [.github/workflows/ci.yml](../.github/workflows/ci.yml)
- [nix/node-b.nix](../nix/node-b.nix)

External references:

- GitHub Actions workflow branch and path filters:
  https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- GitHub protected branches and rulesets:
  https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches
  and
  https://docs.github.com/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets
- The Update Framework specification:
  https://theupdateframework.github.io/specification/v1.0.26/
- SLSA v1.2 specification and Build Track basics:
  https://slsa.dev/spec/v1.2/ and
  https://slsa.dev/spec/v1.2/build-track-basics
- SLSA provenance:
  https://slsa.dev/spec/v1.2/provenance
- in-toto:
  https://in-toto.io/
- Sigstore/cosign blob signing:
  https://docs.sigstore.dev/cosign/signing/signing_with_blobs/
- GitHub artifact attestations:
  https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations
- OCI image manifest and distribution specifications:
  https://specs.opencontainers.org/image-spec/manifest/ and
  https://specs.opencontainers.org/distribution-spec/
- Nix binary cache/substituter documentation:
  https://nix.dev/manual/nix/2.34/package-management/binary-cache-substituter
- NixOS configuration switch and rollback manual:
  https://nixos.org/manual/nixos/stable/#sec-changing-config
- Google Cloud deployment strategies:
  https://docs.cloud.google.com/deploy/docs/deployment-strategies
- Google Cloud canary deployment:
  https://docs.cloud.google.com/deploy/docs/deployment-strategies/canary
- AWS blue/green deployments whitepaper:
  https://docs.aws.amazon.com/whitepapers/latest/blue-green-deployments/welcome.html
- W3C PROV overview:
  https://www.w3.org/TR/prov-overview/
- W3C Web Annotation Data Model:
  https://www.w3.org/TR/annotation-model/
- DataCite schema:
  https://schema.datacite.org/
- IPFS content addressing:
  https://docs.ipfs.tech/concepts/content-addressing/
- Dolt version-control features:
  https://docs.dolthub.com/sql-reference/version-control
