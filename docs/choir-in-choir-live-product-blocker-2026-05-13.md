# Choir-in-Choir Live Product Blocker

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`
Status: live local Playwright dogfood now passes with worktree worker fallback; still not full microVM isolation.

## Original Blocker

The next natural deformation is a live product-path prompt:

```text
prompt bar / VText -> request_super_execution -> super -> request_worker_vm -> delegate_worker_vm -> export_patchset -> promotion queue -> owner approval -> internal promotion
```

The local stack can now reach `vmctl`, but on this Mac it runs in host-process fallback mode:

```text
vmctl: Firecracker not available, using host-process sandbox mode
```

In that mode, the worker handle returned by `request-worker` has the same sandbox URL as the foreground runtime:

```text
worker_sandbox_url=http://127.0.0.1:8085
```

That preserves API topology but not mutation isolation. A live product prompt that lets super/vsuper perform mutable coding work could touch canonical local repo or foreground runtime state. That would violate the mission invariant:

```text
Foreground stays stable. Background mutates. Canonical state changes only by promotion.
```

## What Is Still Safe

Safe local probes:

- read-only prompt/VText/super routing;
- `vmctl` reachability and ownership resolution;
- worker handle allocation;
- promotion queue rendering;
- owner approval/rejection;
- verifier/promotion tests in temporary repositories;
- Playwright UI verification.

Unsafe local probe:

- live mutable Choir-in-Choir product prompt where worker/vsuper edits the shared repo while `worker_sandbox_url` equals the foreground sandbox URL.

## Mitigation

The local runtime now refuses same-runtime worker delegation unless `RUNTIME_LOCAL_WORKER_MODE=worktree` is enabled. With that mode enabled, `delegate_worker_vm` creates a git worktree from foreground HEAD, sets worker run `tool_cwd` metadata to that worktree, and queues exports from the worktree rather than the foreground repo directory.

The local launcher also sets `RUNTIME_SUPER_FOREGROUND_MUTATION_MODE=worker_only`, which blocks foreground super from using direct mutable tools without an isolated `tool_cwd`. This preserves the intended product dogfood route: super inspects and delegates; workers mutate candidates.

The local launcher now also sets `RUNTIME_TOOL_CWD` to the repo root by default, so local worktree isolation can derive a foreground base SHA during product-path dogfood runs.

`docs/live-playwright-worker-dogfood-proof-2026-05-13.md` records the live result: the first Playwright attempt failed at the git-repo precondition, the launcher was repaired, and the rerun passed with worker exports and queued promotion candidates.

This reduces the local blocker enough for tightly scoped dogfood, but it does not provide OS-level confinement.

## Required Next Probe

Before treating arbitrary live mutable Choir-in-Choir dogfood as strongly isolated, one of these must be true:

- run on a Firecracker-capable host where worker VMs have distinct sandbox URLs and isolated filesystems;
- or keep local dogfood constrained to the worktree fallback and refuse claims of strong isolation.

The minimal local fallback should expose the same shape as real microVMs:

- worker id;
- VM/sandbox identity;
- base SHA;
- worker branch/worktree path;
- export patchset;
- rollback command;
- verifier contract;
- promotion candidate.

## Current Evidence

Verified today:

- local launcher starts `vmctl`;
- proxy reports `vmctl_routing=enabled`;
- direct `resolve` creates an active primary desktop ownership;
- direct `request-worker` returns a typed worker handle;
- focused Playwright desktop/files/settings suite passes under `vmctl` routing;
- Settings can approve a verified candidate without browser-internal routes;
- runtime/store/promotion tests verify approval is required before internal promotion.
- local foreground-super mutation guard blocks direct mutable tools while allowing worker worktree mutation.

This is enough to continue building the bridge and attempt bounded local product-path dogfood, but not enough to claim Firecracker-grade isolation on this host.
