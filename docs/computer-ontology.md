# Choir Computer Ontology

**Status:** canonical architecture vocabulary
**Last updated:** 2026-07-10

This document names the durable object that Choir operates on.

Choir does not primarily give each user a sandbox. It gives each user a
persistent **computer**: a stateful private machine-world where apps, agents,
files, package installs, source trees, local builds, Dolt state, prompts,
runtime services, and user preferences can diverge from the platform baseline.

"Sandbox" remains a code/service name where the implementation currently uses
it. It should not be the product ontology. A sandbox sounds disposable. A Choir
computer is allowed to be durable, personal, divergent, useful, backed up,
forked, merged, published from, and updated over time.

## Cloud Boundary

Choir clouds are deployment and ownership boundaries. A cloud contains NixOS
host infrastructure, platform computers, user computers, source systems,
policy, and publication/subscription boundaries.

Use these terms:

- **Choir Community Cloud**: the public/shared Choir deployment, including
  `choir.news`, World Wire (formerly Universal Wire), public publication
  surfaces, public user computers, and Community Cloud platform computers.
- **Private Choir Cloud**: a customer-controlled deployment with its own NixOS
  host or host cluster, platform computer(s), many user computers, private
  source systems, policy, and optional publication or subscription links to the
  Community Cloud.
- **Host**: the NixOS machine or host cluster running infrastructure services.
- **Platform computer**: a persistent computer owned by a cloud itself rather
  than an individual user.
- **User computer**: a persistent computer owned by a person or service account
  inside a cloud.
- **Capsule effect bundle**: frozen speculative effects bound to a ComputerID
  and base event head; the only self-development candidate.

Do not model a customer Private Choir Cloud as just a tenant row in the
Community Cloud. A private cloud may have a thousand employees, its own NixOS
hosts, its own platform computers, and its own private user computers.

Host-side daemons may still exist for edge routing, auth, gateway, lifecycle,
publication, or source-service work. Product authority remains scoped to the
relevant cloud and computer: platform-level semantic work belongs to a platform
computer, user-level semantic work belongs to a user computer, and speculative
self-development effects remain inside guest capsules until accepted.

## Core Object

```text
Computer =
  VM/OS/runtime state
  + Dolt/app state
  + source/build state
  + content blobs
  + artifact/provenance graph
  + route identity
```

A computer is not one database, one git checkout, one VM snapshot, or one
browser session. It is a product object composed from several ledgers with
different merge laws.

## Current Implementation Status

| Layer | Status now | Claim boundary |
| --- | --- | --- |
| Persistent user computer and VM lifecycle | **Live** | A long-lived computer is identified by stable ComputerID; a realization is replaceable machine state. Existing lifecycle/status remain projections and actuators. |
| Worker/background and candidate VM mutation | **Deleted by clean cutover** | Generic delegated agents are durable runs/trajectories and perform bounded effects only through guest-local capsules. |
| `internal/computerversion` constructor/verifier | **Live audited construction substrate** | ComputerVersion is an immutable reconstruction checkpoint at an event head, not the evolving computer or semantic promotion authority. |
| Computer event authority | **Implemented source candidate; deployed proof pending** | One guest appender, corpusd head CAS, embedded projection, immutable event artifacts, privacy, and recovery are effects-off pending the active Definition's landing gate. |
| Capsules | **Implemented source candidate; deployed proof pending** | Guest-local namespaces, cgroup, seccomp, Landlock, capability broker, transaction tape, and fail-closed admission are effects-off pending deployment. |
| Features adoption and activation | **Deleted by clean cutover** | AppChangePackage/AppAdoption/lineage records are not self-development authority or product fallback. |
| Self-development acceptance/materialization | **Implemented source candidate; deployed proof pending** | Public CLI/API operations, external decision, guest updater, checkpoint, route projection, rejection, restart/reconstruction, and rollback require G1 acceptance and deployed gates. |

Do not collapse a code-present substrate into a live product claim. A worker
VM, forked desktop, AppChangePackage, capsule, frozen effect bundle,
ComputerVersion checkpoint, realization, and route projection are different
objects with different authority.

## Self-Development Candidate Contract

The user experiences one stable computer identified by `ComputerID` and its
canonical event chain. A self-development candidate is a frozen,
content-addressed `CapsuleEffectBundle` bound to the computer, base event head,
trajectory, capsule identity, source tree, offline build inputs, runtime
artifacts, tests, verifier receipts, and resource receipts. It is inert until
an authorized acceptance event.

Capsules are ephemeral guest-local effect chambers. They do not own semantic
state, event ordering, acceptance, materialization, checkpoint publication, or
route projection. Candidate VMs, desktops, routes, mutable branches,
AppChangePackages, AppAdoption/lineage records, host daemons, and host repair
are not self-development candidates or fallbacks.

An accepted event changes desired state. A root-owned guest updater stages and
health-checks the immutable release before an applied event advances effective
state. `ComputerVersion = (CodeRef, ArtifactProgramRef)` is then a
reconstruction checkpoint at that effective event head. vmctl may project the
checkpoint into the serving route through its sole route-slot CAS, but the
route is not computer identity or event authority.

The normal computer is long-lived and reconstructible. A realization may be
replaced without changing `ComputerID`; reconstruction verifies the immutable
event chain and receipts, deterministically rebuilds embedded state, and never
reruns a model, tool, or network observation.

## Dolt Store Taxonomy

The Dolt substrate is split into two stores that must never be conflated (see
D-STORES and D-WIRE in
[docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md)):

- **World-wire store:** platform `ObjectGraphStore` at
  `internal/platform/objectgraph_store.go`, served by `corpusd` in sql-server
  mode. Narrow `computer_event_heads`, append/idempotency, mode, and lifecycle
  control tables live beside—but are semantically separate from—world-wire
  objects and route-slot tables. corpusd mechanically performs authenticated
  event-head and platform-control CAS; it does not select semantic events.
- **VM-local embedded store:** one embedded Dolt workspace per user VM at
  `internal/objectgraph/dolt_store.go`. It indexes the externally pinned event
  chain and materializes the accepted effective state; it is never the sole
  durable event copy or an alternate head authority.

The obsolete tag-based embedded-Dolt promotion adapter and its destructive
`DOLT_RESET --hard` rollback are deleted. VM-local embedded Dolt materializes
accepted effective state; it does not fork candidate branches, promote tags, or
act as route authority.

## Ledger Split

Do not force every change through one storage abstraction.

| Ledger | Owns | Transition authority |
| --- | --- | --- |
| Canonical computer events | immutable causal envelopes, accepted/rejected effects, desired/effective heads and commitments | one guest `ComputerEventAppender` requests typed corpusd head CAS; no other writer appends semantic events |
| VM/OS/runtime | realization, installed release, running services, local caches, process environment | root guest updater and lifecycle actuators project an authorized event; realization replacement does not change ComputerID |
| Dolt/app state | textures, appagent state, typed Researcher updates, event index, accepted effective-state materialization | deterministic reducer plus exact event-head/state preconditions; typed Researcher updates fate-share with append |
| Actor recovery log (narrow SQLite) | durable actor updates and compacted activation snapshots used by `internal/actorruntime` | recovery/replay only; never semantic event, trajectory, acceptance, or promotion truth |
| Choir Base (partial SQLite journal + tree/blob substrate) | append-only source/file observations, derived tree, content-addressed blobs, File Provider/materialization support | tested but not canonical computer authority; product wiring must preserve the event/embedded-Dolt boundary |
| Source/build | CodeRef source, offline recipe/toolchain/dependencies, runtime/service artifacts | capsule effect bundle becomes desired code only through an accepted event |
| Blob/content store | event bodies, encrypted/private payloads, generated media, bundles, receipts, releases, checkpoints | content-addressed pin receipts before event-head acknowledgement |
| Artifact/provenance graph | claims, citations, source anchors, verifier results, trace refs | evidence projection; cannot append, accept, or materialize an event |
| ComputerVersion checkpoint | immutable reconstruction inputs at a canonical/effective event head | published only after applied materialization or rollback |
| Route identity | serving projection of an accepted checkpoint | vmctl-only route CAS with exact accepted-event/certificate joins; never semantic promotion authority |

The filesystem is not one thing. Source files under a repo are source/build
state. Uploaded files and generated media are blob state. Runtime caches and temp
files are machine state. Dolt-backed documents may have filesystem aliases, but
the canonical semantic state belongs in Dolt.

Therefore the rule is:

```text
Promote typed artifacts, not opaque machine accidents.
```

## Computer Lineage

Choir should track lineage explicitly:

```text
platform base P0
  -> user computer U0
      -> accepted event E1
          -> checkpoint U1
```

Platform versions, platform computers, and user computers are different levels.

- A platform version is the official Choir baseline: source, services, runtime
  invariants, default apps, default prompts, and upgrade machinery.
- A platform computer is a persistent cloud-owned computer for cloud-level
  agents, cloud-owned artifacts, publication/source systems, and shared indexes.
- A user computer is identified by stable `ComputerID` and its canonical event
  chain. Realizations and checkpoints may change without changing that identity.
- A self-development candidate is an inert `CapsuleEffectBundle` at a base event
  head, not a forked computer or serving route.
- A published package/change is a typed sharing artifact. Import/adoption does
  not accept, materialize, or route self-development.

This split is essential. Computer evolution, source sharing, and platform
deployment are different authorities and cannot substitute for one another.

## Event-Derived Change Paths

### Computer-local self-development

A local self-development operation targets one explicit `ComputerID`; it does
not target `origin/main`, a candidate VM, or an ambient current computer.

```text
canonical base event head
  -> capability-bound capsule work
  -> frozen effect bundle
  -> independent verification
  -> external scoped acceptance event
  -> root guest materialization and health
  -> applied event / effective state
  -> ComputerVersion checkpoint
  -> vmctl route projection
```

Concurrent causal observations may append while the proposal is open, but
acceptance binds the immutable proposal/bundle plus exact desired and effective
heads and commitments. A changed state projection refuses acceptance; it is not
silently merged. Rejection retains the full event history without applying the
bundle. Rollback selects a prior applied event/checkpoint and rematerializes it;
events are never deleted.

The typed state transition—not a generic filesystem merge—is the authority:

- Source/build effects are ordered in the frozen bundle and rebuilt from
  pinned offline inputs.
- Typed Researcher updates fate-share their exact embedded-Dolt mutation with
  the event append.
- Blobs, private payloads, verifier results, releases, and checkpoints are
  immutable content-addressed artifacts with signed receipts.
- Runtime state is materialized by the root guest updater; opaque running
  machine state is never merged or promoted.
- vmctl changes only the serving route after verifying the accepted-event,
  materialization, checkpoint, verifier, and certificate joins.

### Platform and public change

Platform source changes still land through GitHub main, CI, NixOS deployment,
and staging acceptance. Shared apps, agents, themes, publications, and other
artifacts use their own package/publication protocols. Neither path appends a
computer's acceptance event merely by deploying or importing bytes.

## Event and Projection Receipts

Every nontrivial self-development transition produces independently verifiable
immutable evidence:

```text
computer_id
operation_id
canonical_event_head
desired_event_head
effective_event_head
desired_state_commitment
effective_state_commitment
proposal_event_ref
bundle_digest
verifier_certificate
decision_event_ref
materialization_receipt
checkpoint_receipt
route_projection_certificate
route_transition_receipt
rollback_target
```

The canonical event chain proves ordering, authority, privacy commitments, and
desired/effective transitions. Signed materialization, checkpoint, and route
receipts attest projections of that state. They cannot acknowledge the event
they project. Idempotent retries return the original durable receipt; changed
requests conflict before effects.

## Platform Updates To Divergent Computers

The hard long-term operation is not just candidate promotion. It is letting users
diverge while still receiving platform improvements.

```text
platform P0 -> P1
user     P0 -> U1

update = merge/rebase platform delta P0->P1 into user delta P0->U1
```

This requires typed platform deltas. If a platform change is only an opaque VM
snapshot, it is hard to merge into divergent user computers. If the platform
change is expressed as source/build deltas, Dolt migrations, blob/package
updates, prompt/agent package updates, and artifact schema changes, the update
can be checked ledger by ledger.

The practical rule:

```text
User computers may be opaque and personal.
Shared changes must become typed artifacts.
```

## Current To Ideal Continuum

Current state:

- audited ComputerVersion construction, verification, route CAS, rollback,
  reconstruction, and no-SSH inspection are live;
- the effects-OFF cutover provides one canonical per-computer event appender,
  private payload protocol, guest updater, public self-development API/CLI,
  and capsule-only CoSuper execution; direct VSuper/worker/candidate/package
  mutation paths are deleted or refuse;
- the capsule executor's namespace, cgroup v2, overlayfs, seccomp, Landlock,
  inherited AF_UNIX listener, reconnect, and cleanup path passes the exact
  Node A Linux proof;
- self-development activation and deployed effects remain disabled until the
  C/D/G2 gates bind the exact staging guest, kernel receipt, genesis, and
  proposal path.

Near target:

- one stable ComputerID owns a complete privacy-safe canonical event chain;
- Super cannot mutate directly, CoSuper effects are capsule-only, VSuper
  aliases refuse, and Researcher writes only through its typed update;
- an inert frozen bundle becomes desired state only through scoped external
  acceptance and becomes effective only after verified guest materialization;
- checkpoints and vmctl routes are reconstructible projections with explicit
  rollback receipts.

Ideal direction:

- users can evolve Choir inside Choir without host access or candidate VMs;
- complete audit preserves learning from proposals, refusals, failures,
  rejections, applications, and rollbacks;
- cross-computer publication or divergent-platform reconciliation requires a
  separate owner-ratified protocol and is not inferred from self-development;
- the artifact graph records who changed what, what verified, what failed, what
  was reused, and what became public memory.

## Naming Rules

- Use **computer** for the durable product object identified by ComputerID and
  canonical event chain.
- Use **realization** for replaceable VM/OS/runtime machine state.
- Use **CapsuleEffectBundle** or **frozen effect bundle** for a speculative
  self-development candidate.
- Use **ComputerVersion checkpoint** for immutable reconstruction inputs at an
  event head.
- Use **route projection** for the currently served checkpoint; never call it
  semantic promotion authority.
- Do not use **background computer**, **candidate computer**, **worker VM**, or
  **candidate VM** for current product architecture; those forked-machine
  concepts are retired.
- Use **sandbox** only for existing service/process names or legacy references.
- Use **VM** or **microVM** only for the implementation substrate.

The user should not have to care which realization serves the computer. The
implementation must bind and verify realization, event, checkpoint, and route
transitions.
