# Runtime Invariants

**Last updated:** 2026-04-30

This file captures implementation invariants. For full context, read
[docs/current-architecture.md](current-architecture.md).

## Development Tooling Boundary

Choir must not depend on any coding-agent convenience layer. Development tooling
used during this phase must not become a runtime dependency, product concept,
user-visible feature, or required repository structure.

## Deployment Source Of Truth

GitHub is the source of truth for tracked files deployed to Node B.

Do not edit or sync git-tracked files directly into `/opt/go-choir` on Node B.
Runtime secrets, service environment files, guest images, and generated Nix
artifacts may live in designated runtime paths, but source/config changes must
land through git and the GitHub Actions deploy flow.

## Agent Roles

`conductor` routes top-level user and connector input. It does not mutate
workspace state.

`app` is a user-facing desktop surface. It does not have to be an appagent.

`appagent` owns one user-facing app domain and mutates only typed app state
through product APIs.

`vtext` is the primary appagent and single writer for canonical document
versions. Workers do not write canonical `vtext` text and do not send patches to
`vtext`. They emit updates: findings, evidence, source refs, artifacts,
branches/commits, previews, tests, questions, constraints, or proposals.

`researcher` reads local context and the web, writes findings/evidence to Dolt,
and does not own document text.

`super` is the per-user privileged orchestration root. It can request `vmctl`
resources such as background VM forks and promotions.

`cosuper` is a durable execution co-agent, usually in a background VM. Only
`super` can spawn cosupers; cosupers coordinate within their assigned work but
do not create more privileged execution roots.

`worker` is the general category for delegated agents such as researcher, super,
cosuper, and future specialized workers with their own tools.

## VM Model

`active_vm` is the user's primary desktop VM. It hosts visible appagents,
per-user embedded Dolt, and private app state. It should stay stable and
responsive.

`background_vm` is a fork of the user's active VM. Risky mutable work goes
there: code edits, package installs, tests, builds, deploy prep, generated files,
and anything that can destabilize the active desktop.

Background work returns artifacts, findings, branch/commit refs, previews, test
results, and proposed merges. A background VM can merge back into active state or
be promoted to active while the previous active snapshot remains available for
rollback.

Shared worker VMs are not a current invariant. They may become a later cost
optimization, but the immediate model is active VM plus capacity-gated background
VM forks, including for free users while capacity allows.

`platform_vm_pool` is a platform-level pool for public/unauthenticated and shared
serving work. It is needed during the publication pass so published `vtext`
artifacts can be served without hydrating private user VMs.

## Super-Tier Execution Policy

`super` and `cosuper` should not edit the live desktop directly. They may inspect
or control it through typed APIs, but mutable workspace changes should happen in
background VM forks.

Do not over-design locks, leases, or predeclared edit scopes as the core safety
model. The current safety model is VM placement, typed app APIs, durable
provenance, Trace visibility, and merge/promotion review.

## State Placement

Per-user embedded Dolt holds private product state: app graph, appagent state,
`vtext` document/version content, prompts, local trajectories, findings, evidence
metadata, and publication staging metadata.

The per-user snapshot filesystem holds files, working trees, uploads, large
media, build artifacts, generated outputs, and filesystem aliases or materialized
shortcuts for Dolt-backed `vtext` documents.

Platform Dolt holds platform-visible facts: accounts, VM lifecycle/capacity,
routing records, publication records, public artifact metadata, citation graph,
compute accounting, and later CHIPS state.

Platform Dolt is a ledger, not the network.

## Messaging

Live actors should receive hot-path payloads over in-memory queues, channels,
direct transport, or relays. They should not normally wake and then query Dolt
before acting.

Durable handoff/control records exist for recovery, replay, audit, provenance,
and important handoff durability.

```text
append durable handoff -> deliver hot-path payload -> process turn -> commit effects/events -> ack
```

Do not mark an important handoff consumed before the actor has committed a result
or explicit failure.

Cross-VM routing should use direct transport or a relay, not platform-Dolt
polling.

## Trace

Trace should show trajectories, not isolated loops.

A trajectory starts with user or connector input and continues through conductor
routing, appagent ownership, worker delegation, VM execution, findings,
artifacts, versions, and publication candidates.

Trace should make causality visible without forcing the user to read every raw
message.
