# G4 Immutable Canary Deploy Refresh Blocker

**Observed:** 2026-07-17 on staging Node B while deploying commit `6f66f73e26b0fe422b29700f9c8ad5ae4ca27324`.

**Mutation class:** **red**. Protected surfaces: immutable `ComputerVersion` realizations, deployment routing, active-computer refresh, vmctl restart recovery, and activation receipts.

## Problem

The ownership reload repair did reach Node B far enough to restart vmctl successfully. This proved the retained current-constructed plus detached-legacy rollback pair now loads: vmctl became healthy with 150 ownerships, the constructed canary remained the sole active realization, and its retained publication and rollback records survived.

The deployment correctly refused to publish an activation receipt, but for a second authority mismatch. The deploy workflow selected every active interactive ownership for global runtime refresh. It called `/internal/vmctl/refresh` for the routed constructed canary, then required the guest to report the deployment commit. The guest correctly continued serving the immutable runtime selected by its frozen `ComputerVersion`:

```text
Refreshed candidate-fleet-49ee3bd0ec6f366a164c02d2
Timed out waiting for candidate-fleet-49ee3bd0ec6f366a164c02d2 to serve sandbox runtime 6f66f73e...; observed 7122f279...
```

A constructed `ComputerVersion` is a function input/result boundary. A later global host or sandbox deployment must not rewrite its runtime in place. Doing so would destroy the exact CodeRef/ArtifactProgramRef-to-realization join and make reconstruction evidence false. The deployment refresh loop predates constructed ownerships and currently treats them as mutable ordinary primary computers.

## Current containment

The workflow emitted `/var/lib/go-choir/deploy-failures/29599054091-1.json` and did not advance `/var/lib/go-choir/deploy-receipt.json`, which remains at `3b7f6d11`. vmctl itself is active after loading the repaired registry. The canary remains healthy on its frozen runtime, the generation-1 route remains committed, and no later fleet row moved.

## Required repair

Deployment refresh selection must explicitly exclude `snapshot_kind: constructed-computer-version` ownerships from ordinary guest, VM boot-contract, and sandbox hot-refresh mutation. It must name those rows as preserved immutable realizations, leave their VM/disk/version/publication bindings unchanged, and still health-check the deployed host services. Mutable active interactive computers remain subject to the existing exact deployment-commit refresh check. If the mutable set is empty, the selected active-computer deployment class must still produce an honest receipt stating that the conforming mutable set was empty rather than pretending an immutable realization was updated.

A deterministic workflow test must cover a mixed inventory: one mutable active primary is selected; one constructed active primary is excluded and retains its frozen identity.

**Rollback refs:** prior accepted deployment receipt at `3b7f6d11`; generation-2 frozen route rollback `route-bootstrap:sha256:40f5a9f1da1d08c4d982e998d5fbfb97d7a83efc0e7c1cd0a9132c8f3fc266d0`; retained detached legacy receipt `legacy-detach:sha256:7c51827a9aaebd720d3940eade9be46aaf6fa84e7c2ebc1404b5ba9926679b8f`.

**Heresy delta:** discovered `1` (global deploy attempted to mutate an immutable ComputerVersion realization); introduced `0`; repaired `0`.

**Conjecture delta:** retained rollback-pair restart load moved from falsified to staging-proved. Global deployment non-interference with immutable constructed computers moved from assumed to falsified.
