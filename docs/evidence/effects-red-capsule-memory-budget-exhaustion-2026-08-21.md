# Staging Evidence: Capsule Memory Budget Exhausted by Unreleased Super Capsules

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 357
- Guest build: `e2fc8d2a7ec5617f3afcd6fab912c2c8a34eef73` (deployed `2026-08-21T09:42:30Z`, CI `32468015971`)
- Provider: `chatgpt/gpt-5.6-luna` (healthy)

## Observation

After the broker PATH inheritance fix (`e2fc8d2a`) was deployed and the retained
computer refreshed to epoch 357, every fresh CoSuper implementation assignment
is blocked before capsule launch:

```text
Capsule startup remains blocked by the runtime memory budget:
2 GiB already allocated, with another 1 GiB required.
```

This persists across many Super cycles (10:44–11:15Z+). No CoSuper run appears
in the public run list because spawn fails before the CoSuper run can be bound.

## Root cause analysis

After a guest reboot, the capsule executor's in-memory `vmMemoryUsed` counter
starts at 0. The observed 2 GiB usage means **two 1-GiB capsules were spawned
within the current process lifetime** and never destroyed:

1. The persistent Super holds `capsule_spawn` (default `MemoryMaxMB=1024`)
   and spawns its own capsule to inspect state before calling
   `assign_co_super`.
2. `assign_co_super` then opens an assignment whose CoSuper run spawns a
   second 1-GiB capsule (`coSuperAssignmentMemoryMax = 1 << 30`).

With `memoryTotal = MemTotal × 3/4 ≈ 3 GiB`, the second spawn exceeds budget:
`used(2 GiB) + requested(1 GiB) > total(3 GiB)`.

The Super's own capsule is never destroyed before the CoSuper spawn attempt,
and the boot-time reconciliation (`reconcileCoSuperAssignmentCapsulesAfterRestart`,
reinstated in `dc266292`) only revokes capsules tied to non-terminal CoSuper
assignments — it does not touch Super-owned capsules spawned via
`capsule_spawn`.

## Required repair direction

The Super's working capsule must be destroyed (or its budget released) before
`assign_co_super` spawns the CoSuper capsule, or the executor must support
budget sharing/transfer between a Super capsule and its delegated CoSuper
capsule. The correct substrate-level fix should be designed rather than
patched: the current per-capsule admission accounting does not model the
parent→child delegation relationship.

Effects remain OFF; no candidate artifact, bundle, proposal, promotion, or
live state write exists. Rollback: revert through origin/main + CI/deploy;
checkpoint `99949fe2` remains untouched.
