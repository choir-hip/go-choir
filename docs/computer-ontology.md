# Choir Computer Ontology

**Status:** canonical architecture vocabulary
**Last updated:** 2026-07-08

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
host infrastructure, platform computers, user computers, candidate computers,
source systems, policy, and publication/subscription boundaries.

Use these terms:

- **Choir Community Cloud**: the public/shared Choir deployment, including
  `choir.news`, World Wire (formerly Universal Wire), public publication
  surfaces, public user computers, and Community Cloud platform computers.
- **Private Choir Cloud**: a customer-controlled deployment with its own NixOS
  host or host cluster, platform computer(s), many user computers, candidate
  computers, private source systems, policy, and optional publication or
  subscription links to the Community Cloud.
- **Host**: the NixOS machine or host cluster running infrastructure services.
- **Platform computer**: a persistent computer owned by a cloud itself rather
  than an individual user.
- **User computer**: a persistent computer owned by a person or service account
  inside a cloud.
- **Candidate computer**: a speculative fork of a platform computer or user
  computer. A candidate is a forked
  `ComputerVersion = (CodeRef, ArtifactProgramRef)` — forked by tape/program
  reference — never a VM or desktop instance (see the H031
  candidate-computer-as-VM heresy in [choir-doctrine.md](choir-doctrine.md)).

Do not model a customer Private Choir Cloud as just a tenant row in the
Community Cloud. A private cloud may have a thousand employees, its own NixOS
hosts, its own platform computers, and its own private user computers.

Host-side daemons may still exist for edge routing, auth, gateway, lifecycle,
publication, or source-service work. Product authority should remain scoped to
the relevant cloud and computer: platform-level semantic work belongs to a
platform computer, user-level semantic work belongs to a user computer, and
candidate mutation belongs to a candidate computer.

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

The user experiences one active computer at a time. Choir may create candidate
computers to explore risky mutations, long-running work, app changes, package
installs, new Go binaries, new Svelte builds, generated media, or semantic data
changes. A candidate computer can later be discarded, archived, merged into the
active computer, promoted as the active computer, or packaged for publication.

A candidate computer is identified by its forked ComputerVersion
`(CodeRef, ArtifactProgramRef)`, never by a VM or desktop instance. It is
materialized on demand — as a VM, a container, or a narrower projection,
according to the chosen materializer's capability manifest — and its
speculative *effects* execute in capsules (the effect chambers in
`internal/capsule` + `internal/runtime/tools_capsule.go`), whose transactions
append to the candidate's tape. There is no background VM or desktop kept warm
waiting to be switched to; promotion moves the route pointer between
ComputerVersions (invariant `route-over-computer-version`).

When the implementation substrate is VM-backed, computer liveness follows this
policy: active primary computers outrank candidate/background work,
and future always-on computers must be modeled as a first-class lifecycle class
rather than a cosmetic account flag.

## Dolt Store Taxonomy

The Dolt substrate is split into two stores that must never be conflated (see
D-STORES and D-WIRE in
[docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md)):

- **World-wire store:** platform `ObjectGraphStore` at
  `internal/platform/objectgraph_store.go`, served by `corpusd`. It is moving to
  sql-server mode now (multi-writer); no data migration is needed.
- **VM-local embedded store:** one embedded Dolt workspace per user VM at
  `internal/objectgraph/dolt_store.go`, shared by all capsules in that VM.
  Promotion (fork/promote/rollback) is an operation on this embedded store, not
  a property of the world-wire store and not a separate promotion workspace.

Branch isolation on the VM-local embedded store is under test (D-PROMO). The
current `DoltPromotionAdapter` is tag-only interim and must not be enabled in any
production promotion flow until the conjecture settles. Rollback on a shared
main branch via `DOLT_RESET --hard` is not an admissible production mechanism
(I4); rollback is a route flip or an isolated-branch operation.

## Ledger Split

Do not force every change through one storage abstraction.

| Ledger | Owns | Typical promotion |
| --- | --- | --- |
| VM/OS/runtime | machine image, installed packages, running services, local caches, process environment | snapshot/cutover, rebuild from typed inputs, or discard |
| Dolt/app state | textures, appagent state, prompts, traces, run memory, theme records, file metadata, promotion records | Dolt branch/commit merge with app invariants on the VM-local embedded store; current adapter is tag-only interim while D-PROMO branch-isolation testing settles |
| Source/build | Go code, Svelte code, tests, Nix/package recipes, app bundles | git-like patch/commit or typed package import |
| Blob/content store | uploaded files, generated media, PDFs, audio, images, patch artifacts | content-addressed hash plus Dolt/artifact metadata |
| Artifact/provenance graph | claims, citations, source anchors, verifier results, trace refs, promotion certificates | graph merge with provenance completeness checks |
| Route identity | which computer currently serves the user or public endpoint | atomic pointer update with rollback pointer |

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
      -> active computer U1
      -> candidate computer C1
```

Platform versions, platform computers, and user computers are different levels.

- A platform version is the official Choir baseline: source, services, runtime
  invariants, default apps, default prompts, and upgrade machinery.
- A platform computer is a persistent cloud-owned computer for cloud-level
  agents, cloud-owned artifacts, publication/source systems, and shared indexes.
- A user computer is a persistent fork of that baseline.
- A candidate computer is a speculative fork of a platform or user computer — a
  forked ComputerVersion `(CodeRef, ArtifactProgramRef)`, materialized on
  demand, with speculative effects executing in capsules.
- A published package/change is a typed artifact extracted from a user or
  candidate computer so another computer can import it.

This split is essential. Users must be able to evolve their own computers
quickly without waiting for global CI/deploy. They should also be able to
receive platform updates without losing their local divergence.

## Two Promotion Paths

### Personal Promotion

Personal promotion changes one user's computer.

Examples:

- build a new runtime service binary inside a candidate computer;
- build a new Svelte frontend for that user's computer;
- install packages;
- add a user-local app;
- change a theme;
- add podcast index data;
- update prompts or agent definitions;
- merge a texture/app state branch.

The target is not `origin/main` and not the global staging deployment. The target
is the user's active computer.

Personal promotion needs local evidence:

```text
base computer -> candidate computer
active computer advanced during candidate work
merge/replay foreground tail
verify the candidate-derived computer
switch user route
keep previous active computer as rollback for a TTL
```

### Platform/Public Promotion

Platform/public promotion makes a change available beyond one user's computer.

Examples:

- official Choir source change;
- shared app package;
- shared agent package;
- platform runtime update;
- public theme package;
- publication artifact;
- reusable verifier/tool.

The target may be the official platform baseline, a public package registry, a
newspaper/public artifact graph, or later an economic/capital surface.

Platform/public promotion needs higher ceremony: verifier contracts, provenance,
review, compatibility with divergent user computers, rollback, and possibly
staging/deploy proof.

## Algebraic Promotion

Let a computer be a product of ledgers:

```text
W = (V, D, S, B, A, R)
```

where:

- `V` is VM/OS/runtime state;
- `D` is Dolt/app state;
- `S` is source/build state;
- `B` is blob/content state;
- `A` is artifact/provenance graph state;
- `R` is route identity.

If a candidate forks from base `B0`, while the active computer continues to
change, promotion asks whether the two arrows from the same base have a valid
join:

```text
        C
      /   \
    B0     M
      \   /
        A
```

`B0 -> A` is the active foreground tail.
`B0 -> C` is the candidate delta.
`M` is the merged computer state or an explicit conflict.

Layer-specific joins differ:

- Source/build: git-like three-way merge, patch apply, build/test checks.
- Dolt/app: Dolt merge, table/key conflicts, app invariants.
- Blobs: content-addressed union by hash, metadata conflicts in Dolt/artifacts.
- Artifact graph: provenance-preserving graph merge.
- VM/runtime: usually do not semantically merge opaque running-machine state;
  rebuild, snapshot/cut over, or discard after typed ledgers check out.
- Routes: atomic pointer update only after a promotion certificate exists.

Promotion checks out when an independent verifier can recompute the typed joins,
hashes, conflict list, verifier results, and route transition from durable
records.

## Promotion Certificate

Every nontrivial promotion should produce a durable certificate.

Useful fields:

```text
promotion_id
promotion_kind: personal | platform | publication
owner_id
base_computer_id
base_vm_snapshot
base_dolt_commit
base_source_sha_or_bundle
active_computer_id
active_vm_snapshot_at_cutover
active_dolt_commit_at_cutover
candidate_computer_id
candidate_vm_snapshot
candidate_dolt_commit
candidate_source_sha_or_bundle
blob_hashes
artifact_refs
merge_results
conflicts
verifier_results
old_route
new_route
rollback_until
```

The certificate should prove:

- no foreground update was silently lost;
- no candidate mutation touched canonical state before promotion;
- typed deltas were merged or explicitly conflicted;
- verifier contracts ran against the state being promoted;
- the route switch has a rollback target;
- retrying the same promotion is idempotent.

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

- the code still has a `sandbox` service and many docs say VM;
- users do not yet have fully first-class durable computer lineage;
- candidate worker export now publishes reviewable AppChangePackage evidence
  instead of queuing old patchset promotion candidates;
- promotion-level acceptance is AppChangePackage adoption with mandatory
  recipient Go/Svelte build, verifier contracts, promote/rollback evidence,
  and owner/platform authority;
- docs now distinguish canonical/current/evidence/historical/stale material.

Near target:

- product/docs say `computer` for the user object and reserve `sandbox` for the
  implementation service name;
- active/background/candidate computers have explicit lineage;
- candidate promotion records name the relevant ledgers, not just patch paths;
- personal promotion can switch one user's active computer after typed merge,
  verification, and rollback certificate;
- platform/public promotion remains a separate higher-ceremony path.

Ideal direction:

- users can evolve Choir inside Choir;
- candidate computers run long autonomous work;
- personal promotion is fast and reversible;
- published packages are typed and importable;
- platform updates merge into divergent user computers;
- the artifact graph records who changed what, what verified, what failed, what
  was reused, and what became public memory.

That is the learning economy of artifacts: computers generate candidate
structure, verifiers and owners select, promotions retain, publications expose,
citations connect, and future work can reuse the retained structure.

## Naming Rules

- Use **computer** for the user-facing durable execution object.
- Use **active computer** for the computer currently routed to the user.
- Use **candidate computer** for a speculative fork that may become active,
  merge back, publish a package, or be discarded. The name refers to a forked
  ComputerVersion, not to a VM or desktop instance.
- Use **background computer** when emphasizing long-running off-foreground work.
- Use **sandbox** only for existing service/process names or legacy references.
- Use **VM** or **microVM** only when discussing the implementation substrate.
- Use **candidate world** for the broader speculative state branch when it may
  be a VM, worktree, Dolt branch, package branch, or future substrate.

The user should not have to care whether their computer is currently backed by a
Firecracker VM, host-process fallback, NixOS image, worktree, or later substrate.
The implementation must care, record it, and verify transitions.
