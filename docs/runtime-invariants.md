# Runtime Invariants

**Last updated:** 2026-05-14

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

Personal computer changes are not automatically platform behavior changes. A
user-local app, prompt, theme, package install, Go binary, Svelte build, or
Dolt/app-state change may be promoted into that user's computer without global
CI/deploy, provided the personal promotion records lineage, typed deltas,
verifier evidence, and rollback.

`https://choir.news` is the acceptance environment for vmctl, gateway,
live model/search calls, background/candidate computers, platform promotion, rollback,
auth/session, and Choir-in-Choir claims.

## Computer Lifecycle And Reclaim

For the concrete current and future policy, read
[vm-priority-policy.md](archive/vm-priority-policy.md). This section records the
invariants that policy must preserve.

Active user computers should stay warm when capacity allows. Fixed idle timeout
is only a coarse safety valve; pressure-aware lifecycle policy must observe host
memory, CPU, I/O, disk, PID headroom, VM inventory, and protected-work state
before proposing reclaim.

Private computer warmup must begin only after identity is proven. A signed-out
public desktop view must not allocate or hydrate a private user computer.
Post-auth prewarm may start immediately after register/login/session proof, but
it must use the same authenticated product route and proxy/vmctl authority as
normal bootstrap.

Lifecycle policy should classify running work by warmness class, including
public platform computer, primary user computer, premium always-on primary
computer, candidate/background computer, ordinary worker computer, and critical
protected verifier/promotion/publication work. Browser-public health may expose
only aggregate counts, timing summaries, and policy names for these classes; it
must not expose user ids, VM ids, desktop ids, emails, prompt text, credentials,
or gateway tokens.

Pressure-aware policy supports both dry-run observation and active reclaim.
Dry-run mode may report aggregate pressure, protected counts, and ranked
candidate summaries through health without changing VM state. Active reclaim
may hibernate only a bounded number of ranked, unprotected, idle candidates
when host pressure crosses configured thresholds, and it must remain controlled
by the fast rollback knob `VMCTL_PRESSURE_RECLAIM_MODE=off|dry-run|active`.

Foreground active computers outrank background/candidate computers for
retention. Recent activity, unknown last-active state, and verifier, promotion,
rollback, or publication work are protected from pressure reclaim in the current
implementation. Active reclaim must continue expanding protection to live
prompt submissions, LLM calls, file writes, verifier runs, promotions, and
publication actions as those states become first-class lifecycle signals.

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

`super` is the per-user privileged orchestration root. It can request `vmctl`
resources such as background/candidate computer forks and promotions.

`vsuper` is the sovereign worker inside a background/candidate computer or candidate world. It
can mutate candidate state within scope and can spawn subordinate cosupers
inside its own VM boundary. It cannot promote canonical state.

`cosuper` is a durable execution co-agent, usually in a background computer. Only
`super` or `vsuper` authority can retired-lease cosuper work; cosupers coordinate within
their assigned work but do not create more privileged execution roots.

`worker` is the general category for delegated agents such as researcher, super,
cosuper, and future specialized workers with their own tools.

## Computer Model

The product object is a persistent user **computer**, not a disposable sandbox.
Use `sandbox` only for the current runtime service/process name.

`active_computer` is the user's primary desktop computer. It hosts visible apps,
appagents, per-user embedded Dolt, private app state, local files, prompts, and
user-specific runtime state. It should stay stable and responsive.

`background_computer` is a fork of the user's active computer. Risky mutable
work goes there: code edits, package installs, tests, builds, deploy prep,
generated files, and anything that can destabilize the active desktop.

Background work returns artifacts, findings, branch/commit refs, previews, test
results, and proposed merges. A background computer can merge back into active
state, publish a typed package, or be promoted to active while the previous
active snapshot remains available for rollback.

Candidate computers are background mutation contexts. They are allowed to break,
install dependencies, run tests, build alternate runtimes, and fail. They produce
deltas and evidence. They do not mutate canonical foreground state directly.

Candidate worlds are the broader substrate-neutral term. A candidate world may
be a computer, worktree, Dolt branch, package branch, or future state branch.

Shared worker computers are not a current invariant. They may become a later cost
optimization, but the immediate model is active computer plus capacity-gated
background computer forks, including for free users while capacity allows.

`platform_vm_pool` is a platform-level pool for public/unauthenticated and shared
serving work. It is needed during the publication pass so published `texture`
artifacts can be served without hydrating private user computers.

## Super-Tier Execution Policy

`super` and `cosuper` should not edit the live desktop directly. They may inspect
or control it through typed APIs, but mutable workspace changes should happen in
background/candidate computer forks.

Do not over-design locks, retired leases, or predeclared edit scopes as the core safety
model. The current safety model is VM placement, typed app APIs, durable
provenance, Trace visibility, and merge/promotion review.

## State Placement

Dolt is the desired canonical store for product state. SQLite may remain for
narrow hot runtime, cache, local compatibility, or transitional implementation
roles only when explicitly justified. Do not introduce new durable product truth
into SQLite by default. The decision record is
[adr-dolt-as-canonical-state.md](archive/adr-dolt-as-canonical-state.md).

Per-user embedded Dolt holds private product state: app graph, appagent state,
`texture` document/version content, prompts, local trajectories, findings, evidence
metadata, and publication staging metadata.

The per-user snapshot filesystem holds files, working trees, uploads, large
media, build artifacts, generated outputs, and filesystem aliases or materialized
shortcuts for Dolt-backed `texture` documents.

Platform Dolt holds platform-visible facts: accounts, VM lifecycle/capacity,
routing records, publication records, public artifact metadata, citation graph,
compute accounting, and later CHIPS state.

Platform Dolt is a ledger, not the network.

Source/build state belongs in git-like source ledgers or typed app/package
bundles. Uploaded/generated files belong in content-addressed blob storage with
Dolt/artifact metadata. Runtime caches and temp files are machine state unless
they are deliberately converted into typed artifacts.

## Promotion

Personal promotion and platform promotion are different invariants.

Personal promotion changes one user's computer. It must preserve active
foreground changes since the candidate fork, record conflicts instead of losing
updates, verify the promoted state, switch routes atomically, and keep a rollback
target for a TTL.

Platform/public promotion changes shared state. It must use verifier contracts,
owner/reviewer decision where required, rollback evidence, and staging/deployed
proof when the change affects deployed platform behavior.

Do not promote opaque VM state as if it were a clean semantic merge. Promote
typed artifacts: Dolt commits/branches, source/build deltas, blob hashes,
artifact graph records, app packages, agent packages, verifier results, and
route-switch certificates.

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

This invariant is the spirit of the target durable-actor model
(`docs/archive/choir-rearchitecture-durable-actors-2026-06-11.md`: "the database
remembers, Go delivers"). Today's `channel_messages`/inbox-poll path is being
replaced by Go-channel mailboxes with activation-on-send; until that cutover
lands, the current per-turn inbox-poll path is what satisfies this invariant in
code. "Channel" in product/UI contexts (a document or trajectory's update
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

Do not claim `promotion-level` without verifier contract evidence plus owner
review and promotion or rollback evidence. Do not claim `continuation-level`
without run-memory/compaction and bounded continuation evidence.
`continuation-level` remains transitional residue, not a target permanent
acceptance class.

`continuation-level` is transitional: per
`docs/archive/choir-rearchitecture-durable-actors-2026-06-11.md`, this acceptance
level is being re-pointed at trajectory/work-item settlement evidence
(portfolio M4). Until that re-pointing lands, the current rule and evidence
requirement above remain in force.

Browser acceptance may use public authenticated product APIs. It must not use
browser-public internal orchestration routes such as `/api/agent/*`,
`/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints.
