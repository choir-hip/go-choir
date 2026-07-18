# Runtime Invariants

**Last updated:** 2026-07-10

This file captures implementation invariants. For full context, read
[docs/current-architecture.md](current-architecture.md).

## Development Tooling Boundary

Choir must not depend on any coding-agent convenience layer. Development tooling
used during this phase must not become a runtime dependency, product concept,
user-visible feature, or required repository structure.

## Deployment Source Of Truth

GitHub is the source of truth for tracked platform files deployed to Node B.

Do not edit or sync git-tracked files directly into `/opt/go-choir` on Node B.
Runtime secrets, service environment files, guest images, and generated Nix
artifacts may live in designated runtime paths, but source/config changes must
land through git and the GitHub Actions deploy flow.

Platform behavior-changing work must include commit, push to `origin/main`, CI
monitoring, Node B staging deploy monitoring, deployed health/build identity
verification, and a deployed product-path acceptance proof. Documentation-only
commits are exempt from automatic CI/deploy and should remain covered by the
workflow path filters for `docs/**` and top-level `*.md`.

Computer-local self-development is accepted only through the canonical event
protocol. A verified capsule effect bundle remains inert until owner acceptance;
guest materialization, checkpoint publication, and route projection follow the
accepted event and retain rollback receipts. It does not become a VM, branch,
package, lineage record, or route.

`https://choir.news` is the acceptance environment for vmctl, gateway,
live model/search calls, computer lifecycle, self-development, platform
promotion, rollback, auth/session, and Choir-in-Choir claims.

## Computer Lifecycle And Reclaim

This section is the maintained source for the runtime priority invariants.

Active user computers should stay warm when capacity allows. Fixed idle timeout
is only a coarse safety valve; pressure-aware lifecycle policy must observe host
memory, CPU, I/O, disk, PID headroom, VM inventory, and protected-work state
before proposing reclaim.

Private computer warmup must begin only after identity is proven. A signed-out
public desktop view must not allocate or hydrate a private user computer.
Post-auth prewarm may start immediately after register/login/session proof, but
it must use the same authenticated product route and proxy/vmctl authority as
normal bootstrap.

Lifecycle policy should classify running realizations by warmness class,
including public platform computer, primary user computer, premium always-on
primary computer, and critical protected verification/materialization/
publication work. Browser-public health may expose only aggregate counts,
timing summaries, and policy names for these classes; it must not expose user
ids, VM ids, desktop ids, emails, prompt text, credentials, or gateway tokens.

Pressure-aware policy supports both dry-run observation and active reclaim.
Dry-run mode may report aggregate pressure, protected counts, and ranked
realization summaries through health without changing VM state. Active reclaim
may hibernate only a bounded number of ranked, unprotected, idle realizations
when host pressure crosses configured thresholds, and it must remain controlled
by the fast rollback knob `VMCTL_PRESSURE_RECLAIM_MODE=off|dry-run|active`.

Foreground user-computer realizations outrank public/shared serving
realizations for retention. Recent activity, unknown last-active state, and
verification, materialization, rollback, or publication work are protected
from pressure reclaim. Protection must cover live prompt submissions, LLM
calls, file writes, verifier runs, materialization, and publication actions as
those states become first-class lifecycle signals.

Premium always-on primary computers are a first-class lifecycle class. Ordinary
pressure reclaim must not silently demote them into best-effort idle keepalive;
capacity reservation, migration, operator intervention, or an explicit
entitlement policy change is required before they can lose 24/7 service.

## Agent Roles

`conductor` routes top-level user and connector input. It does not mutate
workspace state and does not orchestrate document workers. In the current Texture
path, its only agent delegation target is `texture`.

`app` is a user-facing desktop surface. It does not have to be an appagent.

`appagent` owns one user-facing app domain and mutates only typed app state
through product APIs.

`texture` is the primary appagent and single writer for canonical document
versions. Workers do not write canonical `texture` text and do not send patches to
`texture`. They emit updates: findings, evidence, source refs, artifacts,
branches/commits, previews, tests, questions, constraints, or proposals.

Texture source citation is tri-state (Choir Doctrine I15). Source citations are
`source_ref` nodes only; the `source_embed` node type is removed. `display_mode`
(`numbered_ref` | `expanded_ref`) is a reader-toggleable presentation choice on
the `source_ref` node, not a separate operation. Immaterial sources are marked
with `mark_source_unused` rather than silently dropped. Texture prompts carry no
boolean control-flow branches (Choir Doctrine I16); article-format and citation
guidance is unconditional, driven by the default Style.texture.

`researcher` reads local context and the web, writes findings/evidence to Dolt,
and does not own document text.

`super` is the per-user privileged orchestration root. It may orchestrate
durable delegated runs and capability-bound capsules and inspect their evidence;
it does not directly mutate the computer, host, route, or canonical event state.

`vsuper` is retired. Its aliases, profile, prompt, spawn rules, tool grants, and
runtime paths are deleted; it must not survive as a privileged worker role.

`cosuper` performs scoped effectful work only inside a capability-bound guest
capsule. It has no host, raw VM, route, canonical event, or acceptance authority.

Delegated agents such as researcher, super, cosuper, and future specialized
roles are durable runs/trajectories with scoped tools. They are not worker
computers or worker VMs.

## Computer Model

The product object is one persistent user **computer**, not a disposable
sandbox or a set of active/background/candidate machine forks. Use `sandbox`
only for the current runtime service/process name.

The computer is identified by stable `ComputerID` and canonical event chain. Its
VM/OS/runtime realization is replaceable machine state. Checkpoints, releases,
and route projections are reconstructions of accepted state, not candidate
identity or semantic authority.

Risky self-development effects execute only inside capability-bound guest
capsules. A capsule freezes an inert content-addressed effect bundle at an exact
base event head. Verification and acceptance happen outside the capsule; only
an accepted event authorizes guest materialization, checkpoint publication, and
route projection.

`worker_vm`, `background_computer`, `candidate_vm`, and `candidate_computer` as
forked product objects are retired. Generic delegation uses durable
runs/trajectories and capsules instead.

`platform_vm_pool` is a platform-level pool for public/unauthenticated and shared
serving work. It may serve published artifacts without hydrating private user
computers; it is infrastructure, not a user, worker, background, or candidate
computer.

## Super-Tier Execution Policy

`super` and `cosuper` do not edit the live desktop directly. `super` may inspect
and orchestrate through typed public APIs. Effectful `cosuper` work runs inside
a capability-bound capsule with least-privilege tools and produces an inert
bundle for external verification and acceptance.

Do not use VM placement, branch merge, route switching, or a role name as a
safety or promotion model. Safety comes from capability isolation, typed app
APIs, canonical events, independent verification, scoped acceptance, durable
receipts, and event-derived rollback.

## State Placement

Dolt is the canonical direction for product state. SQLite may remain for
narrow hot runtime, cache, local compatibility, or transitional implementation
roles only when explicitly justified. Do not introduce new durable product truth
into SQLite by default. The current Dolt boundary is maintained here and in
[computer-ontology.md](computer-ontology.md).

Per-user embedded Dolt holds private product state: app graph, appagent state,
`texture` document/version content, prompts, local trajectories, findings, evidence
metadata, and publication staging metadata.

The per-user snapshot filesystem holds files, working trees, uploads, large
media, build artifacts, generated outputs, and filesystem aliases or materialized
shortcuts for Dolt-backed `texture` documents.

The **world-wire Dolt store** (historically called Platform Dolt) owns
public/source object-graph and publication records served through `corpusd`:
public artifacts/manifests, source/retrieval/citation/provenance records, and
public-object consent/review/verifier state. It does **not** own accounts,
auth/session state, VM lifecycle/capacity, candidate identity, personal
promotion rollback, or general compute accounting. Those remain in their
respective host/control or VM-local ledgers until an explicit contract assigns
them elsewhere. The world-wire store is a ledger, not the network or semantic
author of Wire/Texture artifacts.

Two narrow SQLite substrates are explicitly transitional/permitted:

- `internal/actorruntime` uses a durable actor update/snapshot log for recovery
  and replay; embedded Dolt remains canonical for semantic app/trajectory state.
- Choir Base uses an append-only journal and derived tree/blob substrate for
  File Provider/materialization work; it is tested but not deployed as a
  competing canonical computer store.

Source/build state belongs in git-like source ledgers or typed app/package
bundles. Uploaded/generated files belong in content-addressed blob storage with
Dolt/artifact metadata. Runtime caches and temp files are machine state unless
they are deliberately converted into typed artifacts.

## Acceptance And Platform Promotion

Computer-local self-development and platform/public promotion are different
authorities.

Computer-local self-development starts with an inert capsule effect bundle at
an exact canonical event head. Independent verification and scoped owner
acceptance append the authorizing event. Guest materialization, checkpoint
publication, and vmctl route projection follow the accepted event and preserve
conflicts, stale-base refusals, exact receipts, and event-derived rollback.
Neither a fork, package, `ComputerVersion`, nor route switch authorizes the
change.

Platform/public promotion changes shared deployed state. It must use verifier
contracts, owner/reviewer decision where required, rollback evidence, and
staging/deployed proof when the change affects deployed platform behavior.

Typed packages and source/build records may support sharing or adoption, but
they never substitute for a computer-local accepted event. Opaque VM state,
mutable branches, route certificates, and lineage pointers are not semantic
promotion artifacts.

## Messaging

Live actors should receive hot-path payloads over in-memory mailboxes (Go
channels), direct transport, or relays. They should not normally wake and then
query Dolt before acting.

Durable handoff/control records exist for recovery, replay, audit, provenance,
and important handoff durability.

```text
append durable handoff -> deliver hot-path payload -> process turn -> commit effects/events -> ack
```

Do not mark an important handoff consumed before the actor has committed a result
or explicit failure.

Cross-VM routing should use direct transport or a relay, not platform-Dolt
polling.

The current actor adapter implements the durable-actor rule: the database
remembers and Go delivers. Sends append to the durable actor log and warm
delivery uses Go-channel mailboxes with activation-on-send. Legacy
`channel_messages` and per-turn inbox-poll code may remain as deletion residue,
but it is not the current execution authority and must receive no new callers.
"Channel" in product/UI contexts (a document or trajectory's update
stream) names a different thing than the Go-channel mailbox described here —
do not conflate the two.

## Trace

Trace should show trajectories, not isolated loops.

A trajectory starts with user or connector input and continues through conductor
routing, appagent ownership, worker delegation, VM execution, findings,
artifacts, versions, and publication candidates.

Trace should make causality visible without forcing the user to read every raw
message.

## Run Acceptance

Run acceptance records are durable verifier objects synthesized from existing
product/control evidence. They must not be manually seeded as success records in
product-path tests.

Acceptance levels are explicit:

- `docs-level`
- `staging-smoke-level`
- `export-level`
- `promotion-level`
- retired `continuation-level`

Do not claim doctrine-level `promotion-level` without verifier contract
evidence, owner review, an observed served route/build cutover, and rollback
proof. The current API may synthesize a package/adoption `promotion-level` from
a promotion event plus a recorded rollback reference; that is protocol evidence
only until the served computer and rollback behavior are observed. Do not claim `continuation-level`
without run-memory/compaction and bounded continuation evidence.
`continuation-level` remains transitional residue, not a target permanent
acceptance class.

`continuation-level` is transitional deletion residue. The target evidence is
trajectory/work-item settlement, but no separate live successor Definition owns
that migration outside the active product umbrella.

Browser acceptance may use public authenticated product APIs. It must not use
browser-public internal orchestration routes such as `/api/agent/*`,
`/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints.
