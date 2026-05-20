# Stable Platform, Divergent Computers

Date: 2026-05-17
Status: architecture design doc and MissionGradient input
Audience: human reviewers, coding agents, and future MissionGradient runs

## Summary

Choir should keep the platform substrate relatively stable while allowing every
user computer to diverge.

Platform-level changes still use the conservative path:

```text
GitHub main -> CI -> NixOS build/switch -> deploy -> staging proof
```

User-level changes should happen inside the user's own computer:

```text
user computer source ref
  -> candidate/background VM source ref
  -> source patch or app change package
  -> matched runtime Go binary + Svelte UI bundle
  -> verifier contracts
  -> promote into that user's active computer
```

Publishing user source is not a platform deploy. It is source/artifact
publication. Another user can import the app change package into their own candidate
computer, rebase it onto their own divergent source ref, rebuild a new
runtime/UI pair, verify it, and adopt it. The platform host binaries do not need
to change merely because users publish or install app change packages.

This is especially important for the automatic newspaper. Much of it should be
built in user land: develop apps, ingestion paths, app change packages, VText
workflows, and reader surfaces inside user/candidate computers; then promote
selected results to the platform computer or publish records/artifacts through
platformd. Host substrate changes are still needed for new shared capabilities,
such as gateway APIs or VM/runtime protocols, but they should be the exception,
not the default way newspaper work ships.

The important exception is the **platform computer**. It has two roles: it is
the public logged-out product surface, and it is the default base image/template
forked when a new user creates a computer. Updating it should usually be a
platform-computer promotion, not a host NixOS deploy, unless the underlying
substrate changes.

## Decision Matrix

Use this matrix before choosing an implementation path:

| Change | Correct path |
| --- | --- |
| Host, security, auth, vmctl, provider boundary, platformd service shape | Platform substrate deploy |
| One user's runtime/UI/source | User-computer candidate promotion |
| Share an app/change with others | App change package publication |
| Install another user's app/change | Recipient candidate adoption and rebuild |
| Logged-out public product or default new-user base | Platform-computer promotion |
| Selected private artifacts become public | Public projection/publication |

If a change seems to fit multiple rows, first identify the state owner. Host
substrate, user computer, platform computer, package registry, and public
projection are different targets with different rollback semantics.

## Smallest Real V0

Build this first, not the whole vision:

```text
user A active source ref
user A candidate computer development
user A AppChangePackage export/publication
user B candidate import/rebase/build/verify/adopt
platform computer candidate/adoption/promotion
matched runtime/UI build records
foreground-tail merge/rebase record
rollback profiles
product-visible inspection
Trace evidence
human-readable certificate
```

Do not build a full registry first. Do not use host deploy as proof of
user-computer promotion. Do not call the package installed before recipient
rebuild, verification, owner approval, and adoption.

MissionGradient alignment: this is not a proof ladder. The mission object is
one real migrating `AppChangePackage`, and the following are observations along
that object's state-transition trajectory:

```text
bootstrap platform control-plane code
  -> user A candidate computer develops app and exports source package
  -> user A publish
  -> user B candidate import/rebase/build/verify/adopt
  -> platform computer candidate/adoption/promotion
  -> rollback + Trace + human-readable report
```

Do not complete these as independent wins. Preserve one continuous app-change
identity, source lineage, authority chain, rebuild evidence, rollback profile,
and Trace/run-acceptance trail. The product meaning is simpler: make the first
app migrate.

The podcasting app should be the first real app-crossing event, not a sterile
toy and not a detour into a full media product. Keep it small enough that source
lineage, adoption, rebuild, verification, rollback, and promotion remain the
main work. Make it real enough that the crossing matters.

Podcast payload floor:

- one runtime API endpoint;
- one Svelte route, window, or panel;
- one persisted podcast, subscription, or episode record;
- one UI/runtime protocol contract;
- one test proving that the UI/runtime contract matches;
- optional provider-free fixture/search path if it fits the same runtime/UI
  boundary;
- no audio upload or media pipeline yet;
- no provider credentials;
- no private user content in source refs.

There is one bootstrap amnesty. Adding the source-lineage promotion control
plane itself may require the normal platform substrate deploy path. After that
bootstrap deploy, moving the podcast payload from user A to user B to the
platform computer must not use host deploy as proof.

All app/runtime/UI mutation in the migration should happen inside candidate
computers. User A's active computer is the base and authority context, not the
scratchpad. User B's active computer and the official platform computer are
adoption targets. They receive changes only through candidate verification,
owner/platform approval, promotion, and rollback records.

The migration should lightly force foreground-tail accounting: before user B
adopts the package, advance B's active source ref with a tiny unrelated
no-conflict change, then record the merge/rebase result. A conflict is not
required for v0.

If the path succeeds early, prefer strengthening the crossing evidence over
expanding podcast features. The mission wins when Choir has a new faculty: a
user-computer app can become portable, adoptable, rebuildable, verifiable,
rollbackable, and promotable to the platform computer.

## Codebase Anchors

This design now extends the current AppChangePackage/adoption product surfaces:

- [internal/runtime/app_promotion.go](../internal/runtime/app_promotion.go)
  owns package publication, recipient adoption, mandatory recipient builds,
  promote/rollback records, and product-visible events.
- [internal/runtime/tools_shipper.go](../internal/runtime/tools_shipper.go)
  exposes `publish_app_change_package` for worker/candidate source deltas.
- [internal/runtime/api_app_promotion.go](../internal/runtime/api_app_promotion.go)
  registers `/api/app-change-packages`, `/api/computers/*/source-lineage`,
  `/api/computers/*/adoptions`, and `/api/adoptions/*`.
- [internal/runtime/api.go](../internal/runtime/api.go) registers
  `/api/trace/*` and `/api/run-acceptances/*`.
- [internal/platform/service.go](../internal/platform/service.go) already
  demonstrates artifact manifests, provenance, consent/review, route, and
  rollback records for VText publication.

The old `/api/promotions` patchset queue is no longer a current product proof
path. Historical records may remain readable through Trace/storage for audit,
but new source movement should use AppChangePackage -> adoption -> recipient
build -> promote/rollback.

## Core Objects

### Platform Substrate

The platform substrate is the stable shared host layer:

- NixOS host configuration;
- Caddy, auth, proxy, gateway, vmctl, platformd, platform Dolt service shape;
- Firecracker/guest image construction and VM lifecycle protocol;
- provider credential boundary;
- public API contracts and authority boundaries;
- base source and default release artifacts.

This layer is changed through GitHub `main`, CI, deploy, health identity, and
staging proof. It should move deliberately.

### User Computer

A user computer is a persistent private machine-world. It has:

- private Dolt/app state;
- VText documents and appagent state;
- files, uploads, generated media, and build outputs;
- source/build state for local runtime and UI changes;
- artifact/provenance records;
- route identity and rollback profile.

Each user computer needs its own source lineage. The default should be a
Choir-managed source ref, not a GitHub fork the user has to understand.

### Candidate Computer

A candidate computer is a fork of an active computer for risky or background
mutation. It can modify source, install packages, build binaries, change UI, run
tests, and produce promotion evidence without mutating the active computer.

Promotion means the candidate's typed deltas are accepted into a target
computer after verification and rollback evidence.

For the source-lineage promotion path, candidate computers are mandatory for
mutable app/runtime/UI work. Active computers are bases, review contexts, and
promotion targets. They are not development scratchpads.

### Platform Computer

The platform computer is the default computer distribution shipped by Choir. It
has two product roles.

First, it is the public surface for signed-out visitors and platform-owned
routes:

- public desktop/newspaper/radio surfaces;
- publication readers;
- logged-out exploration;
- auth-on-mutation entry points;
- platform-owned apps and routes.

Second, it is the default VM/image/template that new user computers fork from
when a new account is created:

```text
platform computer image
  -> new user computer
  -> user's active source/data/runtime lineage
```

Initially this can be admin-controlled. Later it can support reviewer approval,
maintainer groups, voting, consensus, multiple platform computers, channels, or
public governance. It should still use the same candidate/promotion discipline
as other computers.

Later, Choir should support custom computer distributions:

- organization default images for employees;
- school default images for students;
- community distributions with preinstalled apps and different taste;
- expert/hobbyist distros with batteries included;
- user-maintained base images for repeated personal use.

Those distributions are also computers or computer images with source/package
lineage, verifier contracts, and promotion policy. A new user should eventually
be able to choose which base computer to fork, while the official Choir platform
computer remains the default.

### Public Surface For A Private Computer

A user computer should eventually be able to expose a selected public surface,
like a personal website or personal newspaper, without making the whole computer
public.

Conceptually:

```text
private user computer
  -> selected public route/profile
  -> published VText/artifacts/apps
  -> public read surface
```

This is not the short-term implementation target, but the architecture should
preserve the option. The public route must be a projection with explicit
visibility, artifact refs, provenance, and rollback. It must not expose private
Dolt state, secrets, inboxes, prompts, source refs, files, or active mutation
surfaces by accident.

Useful future modes:

- personal website;
- public personal newspaper;
- public project workspace;
- custom-domain surface;
- organization/team public surface;
- invite-only or paid public projection.

Short-term, use platformd publication and platform-computer promotion instead
of building arbitrary public personal-computer hosting. The invariant to
preserve is selective projection, not whole-computer exposure.

## Source Lineage

The source/build ledger should be Git-like, but not user-facing by default.

Recommended v0 backend:

```text
source-ledger.git
  refs/choir/platform/main
  refs/computers/<computer_id>/active
  refs/computers/<computer_id>/candidates/<candidate_id>
  refs/platform-computers/default/active
  refs/platform-computers/default/candidates/<candidate_id>
```

For v0, this should be one platform-managed private bare Git repository or
equivalent source ledger. It should be product-internal, not a GitHub account
requirement for users. GitHub can remain the official platform source ledger and
can later receive mirrors/exports for users who want sovereignty or public
collaboration.

The exact backend can change. The invariant is that source lineage belongs to
the computer, not just the account. A user may have multiple computers.
Candidate refs fork from the active computer ref, not from global `main`, unless
the mission is platform development.

Advanced users may export or mirror their computer source lineage to their own
GitHub fork. That is an option for sovereignty and public collaboration, not the
default product path.

Do not put everything in Git:

- source/build state belongs in Git refs or source packages;
- private VTexts belong in the user's Dolt/app ledger;
- public VText projections belong in platform Dolt plus artifact storage;
- uploads, media, PDFs, audio, images, and build artifacts belong in
  content-addressed blob/artifact storage;
- secrets and provider credentials never belong in user source refs;
- opaque VM snapshots are rollback or archival objects, not semantic source
  deltas.

## App Sharing

Apps span the runtime and the UI:

```text
Go runtime
  -> APIs, appagent tools, schemas, permissions, background jobs

Svelte UI
  -> routes, windows, panels, controls, components, client contracts
```

Therefore an app cannot safely be shared as "user A's binaries." Copying user
A's runtime/UI binaries into user B's computer would overwrite B's unrelated
divergence and smuggle A's private changes into B's world.

There are two related package objects:

```text
SourcePackage
  source/build deltas only

AppChangePackage
  SourcePackage
  + schema migrations
  + capability requests
  + verifier contracts
  + provenance
  + install/adoption policy
```

The implementation object for app sharing should be `AppChangePackage`, because
apps affect more than source files.

```text
AppChangePackage
  manifest
  runtime_source_delta
  ui_source_delta
  schema_migrations
  capability_requests
  app_protocol_contract
  tests
  verifier_contracts
  provenance_refs
  compatibility_constraints
  install_policy
```

Install flow:

```text
user B active source ref
  -> candidate ref
  -> apply/rebase app change package
  -> resolve conflicts
  -> build B-specific runtime Go binary
  -> build B-specific Svelte UI bundle
  -> run app/API/UI verifier contracts
  -> owner review
  -> advance B active computer ref and route profile
```

Prebuilt binaries can be caches for known matching base profiles. They are not
the durable unit of sharing. App change packages and verifier contracts are.

## Lifecycle States

Promotion is not one action. It is a sequence of authority transitions.

App change package lifecycle:

```text
draft
  -> exported
  -> published_private | published_unlisted | published_public
  -> adoption_proposed
  -> candidate_applied
  -> built
  -> verified
  -> owner_approved
  -> adopted
  -> rolled_back | archived
```

Platform computer promotion lifecycle:

```text
proposal
  -> platform_candidate
  -> built
  -> public_route_verified
  -> new_user_fork_verified
  -> approved
  -> active
  -> rolled_back | archived
```

These states should appear in product APIs, Trace, and run acceptance. Avoid
collapsing "published", "built", "verified", "approved", and "adopted" into a
single status.

## Publishing App Change Packages

When a user publishes an app/source change, the platform does not need to
redeploy. Publishing means recording an app change package with provenance:

```text
app change package
  -> manifest
  -> content hashes
  -> compatibility metadata
  -> verifier contracts
  -> consent/review policy
  -> searchable/installable registry entry
```

The policy question "public by default, private by default, or private on paid
tier" is separate from the architecture. The architecture should support:

- private package retained only in the user's computer;
- unlisted package installable by link or explicit grant;
- public package discoverable in a registry;
- paid/private package where publishing or keeping private has product/business
  rules;
- later governance for platform-blessed packages.

The key technical rule is that package publication and package installation do
not imply platform host deployment.

## Automatic Newspaper Build Path

The automatic newspaper should be built mostly through the divergent-computer
model:

```text
admin/user computer
  -> candidate computer
  -> build ingestion/app/VText/publication capability
  -> verify in user land
  -> promote to platform computer or publish to platformd
```

Examples that can live mostly in user/platform-computer land:

- publication reader refinements;
- source/package experiments for newspaper apps;
- VText workflows;
- ingestion UI and article/feed apps when they use existing gateway capability;
- citation/provenance rendering;
- public route styling and product polish.

Examples that still require platform substrate deploy:

- new gateway APIs or provider credential boundaries;
- vmctl/candidate-computer protocol changes;
- platformd schema or service behavior changes;
- auth/session/routing/security changes;
- base APIs needed by all computers.

This gives the practical short-term objective: make automatic-computer
multitenancy smooth enough that Choir-in-Choir can build newspaper features in
user land, prove them, and promote them to the platform computer without
turning every feature into a host deploy.

## When Platform Deploys Are Required

Use GitHub `main`, CI, NixOS build/switch, deploy, and staging proof for changes
to shared substrate:

- host NixOS configuration;
- firewall, Caddy, systemd units, service hardening;
- auth/session and signing-key behavior;
- provider gateway and credential isolation;
- vmctl protocol and VM lifecycle semantics;
- platformd schema/service boundaries;
- base app/runtime contracts that all computers rely on;
- default platform release artifacts;
- security fixes that must become universal immediately.

Do not use platform deploy merely because:

- one user built a local app;
- one user changed their own runtime or UI source;
- one user published an app change package;
- another user imported and rebuilt that package;
- the public/default platform computer changed in a way that can be promoted
  through computer-image or computer-route promotion.

## Platform Computer Promotion And Distributions

The public logged-out surface and the new-user default image should be treated
as the official platform computer.

Near-term control:

```text
admin proposal
  -> platform computer candidate
  -> build matched runtime/UI pair
  -> verify public routes, logged-out UX, and new-user forkability
  -> switch public route and default-new-user base to promoted profile
  -> retain rollback profile
```

Later control:

- admin plus reviewer approval;
- maintainer groups;
- signed packages from trusted users;
- multiple platform computers or channels;
- organization/school/community default computer distributions;
- public governance for important routes;
- consensus or delegated voting for platform-default changes.

This keeps the host substrate stable while still letting the public product
surface and default user-computer image evolve.

## Promotion Records

Every nontrivial promotion should produce a certificate-like record:

```text
promotion_id
promotion_kind
source_computer_id
candidate_computer_id
target_computer_id
base_source_ref
candidate_source_ref
target_active_source_ref_at_candidate_start
target_active_source_ref_at_cutover
foreground_tail_merge_result
merge_strategy
merge_conflicts
replay_or_rebase_evidence
package_manifest_ref
runtime_artifact_digest
ui_artifact_digest
schema_migration_refs
verifier_contracts
verifier_results
owner_or_platform_decision
old_route_profile
new_route_profile
rollback_profile
residual_risks
trace_refs
run_acceptance_refs
```

The exact tables can evolve. The required product behavior is durable
inspectability: another agent or human reviewer should be able to reconstruct
what changed, why it was allowed, what was built, what was verified, what route
changed, and how to roll back.

Foreground-tail fields are required because active state can change while a
candidate is running. A promotion that verifies a candidate but silently drops
foreground updates is invalid.

## Platform Computer Sub-Objects

Use "platform computer" as the product-level concept, but keep these
implementation records separately inspectable:

```text
PlatformComputer
  canonical source/data/runtime lineage

PlatformRouteProfile
  public routes and the artifact profile serving them

DefaultBaseProfile
  image/profile forked by new user computers

PlatformDistribution
  named channel/version of PlatformComputer + base profile + policy
```

In v0, the official platform computer may update both `PlatformRouteProfile`
and `DefaultBaseProfile`, but they should still be separate records. This
prevents an agent from confusing route switch, base-image update, and source
lineage update.

## V0 Verifier Contracts

Required v0 verifier contracts:

- source refs resolve and the candidate forks from target active ref;
- package manifest hashes match retrieved artifacts;
- no private content, secrets, provider credentials, uploads, or arbitrary user
  data appear in source refs;
- runtime Go build passes;
- Svelte/UI build passes;
- runtime/UI protocol contract hash matches;
- schema migrations are dry-run checked or explicitly marked no-op;
- capability requests are recorded and not silently granted;
- no copied runtime/UI binaries from another user's computer;
- adoption record names rollback source ref, runtime digest, UI digest, and
  route profile;
- foreground-tail merge/rebase result is recorded.

These are stronger than "JSON exists" checks and weaker than a future full
marketplace/security review. They are the right v0 floor.

## Product API Sketch

The current codebase exposes product paths for source-lineage adoption, Trace,
and run acceptance:

```text
GET  /api/computers/{computer_id}/source-lineage
GET  /api/app-change-packages/{package_id}
POST /api/app-change-packages
POST /api/computers/{computer_id}/adoptions
GET  /api/adoptions/{adoption_id}
POST /api/adoptions/{adoption_id}/verify
POST /api/adoptions/{adoption_id}/promote
POST /api/adoptions/{adoption_id}/rollback
GET  /api/trace/*
POST /api/run-acceptances/synthesize
```

The invariant is product-visible source refs, package manifests, verifier
results, artifact digests, adoption records, and rollback profiles. Do not
restore `/api/promotions` as a compatibility success path.

## Role Separation

Promotion is an epistemic chain, not just a build chain.

- Worker: produces candidate changes, build artifacts, and self-report.
- Verifier: independently inspects candidate state and evidence.
- Reporter: writes the human-readable promotion/adoption report.
- Orchestrator/meta-verifier: decides whether the evidence chain is adequate
  for promotion under policy.

The worker's own report may be evidence, but it must not be the only verifier
for adoption or platform-computer promotion.

## Threat Model

Primary threats:

- cross-user source or binary contamination;
- accidental publication of private source, data, files, prompts, or uploads;
- capability escalation through package install;
- verifier spoofing or self-verification;
- route switch without rollback;
- platform substrate deploy used to hide user-local promotion gaps;
- new-user base image forked from an unverified platform computer;
- foreground-tail updates lost during candidate promotion.

V0 does not need a complete marketplace security model, but it must make these
threats visible in records, verifier contracts, and forbidden shortcuts.

## MissionGradient For The Next Mission

### Real Artifact

Build the first source-lineage promotion control plane that lets Choir represent
computer-owned source refs, app change packages, matched runtime/UI builds,
platform-computer candidates, verifier evidence, adoption records, and rollback
profiles through product APIs and Trace. Then prove it by making the first app
migrate from a user A candidate computer to a user B candidate/adoption and
then, if the user B adoption succeeds, into the official platform computer.

### Invariants

- Platform substrate remains stable and deploys through GitHub `main`, CI,
  NixOS build/switch, and staging proof when substrate behavior changes.
- Every active user computer has a source lineage ref separate from platform
  `main`.
- Candidate work forks from the target computer's active ref.
- App/runtime/UI mutation happens in candidate computers. Active user computers
  and the active platform computer are not scratchpads.
- App sharing transfers app change package deltas and contracts, not another user's
  binaries or full branch.
- Runtime Go binary and Svelte UI bundle are promoted as a matched pair when an
  app crosses that boundary.
- Private user content, secrets, provider credentials, and uploads are not
  stored in source refs.
- Publishing app change packages does not mutate platform binaries.
- The public/logged-out surface is modeled as a platform computer when the
  change is product-surface or default-image level rather than substrate level.
- New user computers fork from the current official platform computer by
  default.
- Custom organization, school, community, or user distributions are allowed
  later as promoted computer images with their own policy.
- Promotion requires verifier evidence, owner/platform decision, route/adoption
  record, and rollback profile.
- Product proof uses public product APIs and Trace, not internal/test shortcuts.

### Value Criterion

Minimize confusion between platform deployment, source publication, app
installation, personal promotion, platform-computer promotion, and host
substrate change while preserving source lineage, verifier evidence, rollback,
and route identity.

Penalize:

- copying binaries across divergent computers;
- mutating platform `main` for user-local changes;
- hiding app source deltas inside opaque artifacts;
- using a platform deploy to simulate user-computer promotion;
- summaries that claim promotion without source refs, verifier evidence, and
  rollback records;
- fake registries or fake candidate worlds that cannot deform into the real
  system.

### Quality Gradient

Expected quality: solid.

A solid mission result has:

- typed records for source lineage and promotion/adoption evidence;
- a product-visible inspector path;
- at least one real app-change-package or matched-pair candidate path;
- tests around authority boundaries and forbidden cross-user binary copying;
- Trace/run-acceptance evidence;
- clear residual risks.

Substandard work would add labels without changing state ownership, use local
files as fake registries, conflate platform and user refs, or verify only that
JSON exists.

### Homotopy Parameters

Start with a narrow, real projection:

```text
user A candidate computer
one podcast AppChangePackage
user B candidate computer
one matched runtime/UI build record
one verifier contract
one adoption record
one platform computer candidate
```

Then increase realism:

- richer podcast app behavior inside the same source-lineage crossing;
- conflict requiring rebase/agentic repair;
- private versus public package visibility;
- platform computer candidate;
- new-user fork proof from the platform computer image;
- custom organization/school/community distro;
- multiple app packages;
- Node A/B route proof for platform computer only;
- eventually platform-blessed package review.

### Belief State

Current belief:

- Source patch export/import exists for worker candidates.
- VText publication already has artifact manifests, provenance, consent,
  public routes, and rollback refs.
- Product proof recently showed promotion into a server-owned/user workspace,
  not platform `main`.
- The missing layer is computer-owned source lineage plus app change package
  adoption and platform-computer promotion.

Highest-impact uncertainty:

What is the smallest schema/API slice that captures source lineage and
matched-runtime/UI adoption without forcing a premature full app registry?

Next observation:

Inspect AppChangePackage/adoption records, platform publication records, and
runtime source/build assumptions, then improve the smallest current record
shape that can represent one app change package and one computer adoption.

### Investigation And Cognitive Reframing

Before stopping on a blocker, run root-cause probes against:

- source ref ownership and storage location;
- candidate checkout and base-ref selection;
- app runtime/UI contract shape;
- foreground-tail merge/rebase accounting;
- artifact storage and digest identity;
- Trace/run-acceptance synthesis;
- product API authority boundary.

Apply these transforms if blocked:

- Boundary transform: is this platform substrate, user computer, candidate
  computer, platform computer, or package registry?
- State-machine transform: which transition is missing or impossible?
- Invariant transform: which object must not be mutated?
- App-pair transform: does the Go runtime and Svelte UI still match?
- Value-of-information transform: what probe would most reduce uncertainty
  without widening scope?

If a blocker defines an executable next probe inside the current authority
boundary, run that probe instead of ending the mission.

### Dense Feedback

Use:

- Go tests for promotion/source-lineage records;
- API tests for product-visible proposal/adoption inspection;
- Trace events for source ref, package lifecycle, build, verifier, foreground-tail, and adoption moments;
- run acceptance synthesis;
- one visible staging prompt-bar proof after deploy when platform substrate code
  changes;
- screenshots only when the public/platform computer UI path changes.

### Forbidden Shortcuts

- Do not copy user A's runtime/UI binaries into user B's computer.
- Do not develop directly in user A active, user B active, or active platform
  computer. Fork candidate computers first.
- Do not require a GitHub account for default user-computer source lineage.
- Do not store private user content or secrets in Git refs.
- Do not mutate platform `main` to prove a user-local app install.
- Do not use internal/test APIs as acceptance proof.
- Do not call an app change package "installed" until it has been rebased, built,
  verified, and adopted in the recipient computer.
- Do not update the host platform when platform-computer promotion is the real
  target.
- Do not treat a computer distribution as safe for new users until forkability,
  source lineage, and rollback are verified.

### Rollback Policy

Rollback must name:

- previous active source ref;
- previous runtime artifact digest;
- previous UI artifact digest;
- previous route profile;
- candidate ref and package manifest;
- foreground-tail merge/rebase evidence;
- verifier result that justified the promotion;
- command or product action to restore prior route/profile.

For platform substrate deploys, rollback remains git/deploy/NixOS generation
rollback. For computer promotions, rollback is route/profile/source-ref
rollback.

### Stopping Condition

Stop only when one of these is true:

- product APIs and Trace can show a source-lineage promotion/adoption record for
  the first app migration from user A candidate to user B adoption and, if
  feasible, platform computer promotion, as one continuous app-change
  trajectory with rollback;
- a precise invariant-level blocker remains after root-cause probes and
  cognitive reframing;
- an external authority boundary is hit.

Completion evidence should include source refs, package manifest refs, artifact
digests, verifier results, foreground-tail merge/rebase evidence,
route/adoption record, rollback profile, Trace refs, run acceptance, residual
risks, and the next realism axis.

## Proposed Next Mission

Mission file:

```text
docs/mission-source-lineage-promotion-control-plane-v0.md
```

Goal string:

```text
/goal Run docs/mission-source-lineage-promotion-control-plane-v0.md
as a Codex-operated MissionGradient mission: build the simplest real
implementation of divergent-computer app promotion. Implement enough
source-lineage, AppChangePackage, candidate adoption, matched runtime/UI
rebuild, verifier evidence, rollback, Trace, and product inspection to support
one real app crossing between computers. Preserve one continuous migrating
AppChangePackage identity rather than completing independent checklist stages.
Use the existing podcasting app scaffold as the first payload: develop it in a
user A candidate computer, publish it as an AppChangePackage,
import/rebase/build/verify/adopt it in a user B candidate computer, then
promote/adopt it into the official platform computer if B adoption succeeds. Do
not build a marketplace, copy binaries between computers, use platform deploy
to fake user-computer promotion, mutate active computers directly, or stop at
labels/JSON-only records. Finish with a concise promotion/adoption certificate:
what changed, source refs, package refs, artifact digests, verifier results,
route/default-base changes, rollback path, residual risks, and next executable
probe.
```

## Reviewer Questions

- Does the document clearly separate platform substrate deployment from
  per-computer promotion?
- Is the "platform computer" exception clear enough to guide implementation?
- Is app sharing correctly modeled as source/package transfer, not binary
  copying?
- What should the first source-lineage storage backend be: same private repo,
  separate private repo, or self-hosted Git remote?
- What is the smallest product API that makes source refs and adoption records
  inspectable without overbuilding the registry?
- Which verifier contracts are required for a matched runtime/UI pair?
- Which policy should govern public/private package publication, and which
  parts should remain purely technical for v0?
