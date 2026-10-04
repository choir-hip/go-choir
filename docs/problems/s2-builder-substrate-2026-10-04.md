# S2 finding: builder-substrate decision — privileged-builder-capsule is not a drop-in; host-service is the evidence-backed selection

**Date:** 2026-10-04
**Status:** evidence-complete for the substrate decision.
**Mutation class of this record:** green. The substrate landing is `red`.
**Station receiving it:** S2 (choir-appdev-s2-layering-runtime-from-release-2026-10-01).
**Depends on:** effect plane proof — landed 2026-10-04 (`f61de45b`, op
`selfdev-b72a48565061c35c0a22246cb6fc06c3` FROZEN, bundle `9d2be524`).

## The decision

S0b narrowed the builder to two branches: **host-side service** vs
**privileged-builder-capsule** (scoped-guest-service weakened — gateway
maild is the only candidate in-guest host endpoint and does not run Nix).

## Why privileged-builder-capsule is not viable as a drop-in

Source evidence (`internal/capsule/executor.go`, `internal/capsule/namespace.go`):

1. **Every capsule is `NS_USER` + `NS_MNT` + `NEWNET` + `NS_PID` + `NS_UTS`
   + `NS_IPC` + `NS_CGROUP` confined** (`CreateCapsuleNamespaces`,
   `namespace.go:37-38`). `NEWNET` air-gaps it: no interfaces, so
   `nix build` inside a capsule cannot fetch flake inputs / substituters.
   A build must be fully vendored — a different, heavier input contract
   than a host builder gets.

2. **`/nix/store` is forced read-only.** `prepareCapsuleRoot`
   (`executor.go:1848-1854`) bind-mounts `/nix/store` recursive then
   remounts `RDONLY|NOSUID|NODEV`. The merged root is an overlay — the
   upper is writable, so a writable bind *over* `/nix/store` is
   mechanically possible inside the private `NS_MNT`, but no spawn knob
   exposes it, and the harden step is unconditional.

3. **No privileged/net tier exists.** `ResourceTier` has only
   small/medium/large (memory/CPU/pids) — there is no code path to drop
   `NEWNET`, relax `NS_USER`, or grant a writable store. Making the
   builder a capsule is **new privileged-capsule surface area**, not a
   configuration of an existing one.

4. **Store mutation authority.** A writable store inside one capsule is a
   private store; per-computer app-layer closures need to *resolve against
   the booted base's store-path layout*, which is exactly what the EROFS
   base already provides read-only.

## Host-side service is the evidence-backed selection

- The base image already carries `nix-2.34.7` (`bin/nix-store`,
  `bin/nix-daemon`) in the booted EROFS store.
- A host builder runs `nix build`/`nix store export` with real network +
  a writable host store, produces the app-layer closure + its base-image
  identity, and hands S2 the closure + derivation/input evidence the
  contract requires (`metamission:327-329`).
- It needs **no new privileged-capsule surface** and preserves the
  "no writable guest-global store or daemon" boundary — the guest still
  never gets a writable store; materialization unpacks the closure at a
  GC-rooted data-disk path.

## Decision

**Select host-side service as the S2 builder substrate.** The
privileged-builder-capsule branch is feasible only as new privileged-
capsule machinery (drop NEWNET, allow a writable private store, relax
NS_USER) — that is a larger substrate change than the station intends and
duplicates what the host already provides. Landing the host-service
builder before closure-consuming acceptance is the convergent move.
