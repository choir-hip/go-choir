# Node B Deploy Disk Preflight Floor Exceeds Irreducible Occupancy

- **Date:** 2026-08-26
- **Class:** platform problem (CI/deploy substrate), problem-documentation-first receipt
- **Status:** documented before fix; fix commit references this receipt
- **Mutation class of the follow-up fix:** orange (deploy workflow calibration, no product runtime change); rollback = git revert

## Problem

The Node B staging deploy is blocked by `scripts/node-b-deploy-disk-preflight`:
root filesystem available space (116.3 GiB) is below the required floor
(`DEPLOY_MIN_FREE_KIB` default 125829120 KiB = 120 GiB), and the script's
bounded reclaim cannot close the gap because the disk is full of *legitimate,
irreducible occupancy*, not garbage.

## Evidence

- CI run `32924736378` (docs commit `5706a176`), job "Deploy to Staging (Node B)",
  failed 2026-08-26T03:00:07Z:
  `root_available_kib=122037488 min_required_kib=125829120` — short by ~3.7 GiB.
- Bounded reclaim inside the preflight was a no-op:
  - `POST /internal/vmctl/reclaim`: `vms_reclaimed:0, retention_pruned:0`,
    decision `observe`, reason "host pressure below reclaim threshold"
    (memory available 62.4%, state dir available 24.5%).
  - `GET /internal/vmctl/retention-plan`: "no VM state matched the retention
    prune policy" — `candidates:0, orphan_state_dirs:0` over 148 ownerships.
  - `journalctl --vacuum-size=256M`: freed 0B. `nix store gc`: 0 paths deleted.
- Direct occupancy (read-only SSH diagnosis, 2026-08-26):
  - `/var/lib/go-choir` = 342G of 355G used:
    - `platform-dolt/platform` = 224G (canonical event store; required).
    - `vm-state/candidate-fleet-e15cb89...` = 89G — sealed computer
      `computer-03335285269bdba4f94377e56879f9e6` (`held=true`,
      `premium_always_on`); protected historical evidence artifact.
    - `vm-state/candidate-fleet-d03dacaa...` = 22G — **active** computer
      `computer-4c20ff4a21a021c4306d8c783be0037d`
      (`user_id=universal-wire-platform`, epoch 12406, last active
      2026-08-25T07:28Z). Not reclaimable; it is the live platform computer.
    - Remaining ~145 hibernated candidate-fleet state dirs are ~12M each.
  - `platform-artifacts` = 3.2G content-addressed platform artifacts (CAS).
- The product path has no state-dir deletion for a stopped registered
  computer: `/internal/vmctl/remove` drops only the ownership row (and
  requires a computer-version route); retention prune is the designed
  garbage collector and correctly declines live ownerships.

## Belief Update

The 120 GiB floor was calibrated when the host had reclaimable slack. The
platform has since grown into the slack with irreducible occupancy (sealed
evidence artifact + live platform computer + 224G canonical store). The
binding constraint is now **host disk capacity (476G, 76% used)**, not
reclaimable garbage. A preflight floor that assumes reclaimable headroom
will permanently block deploys even though the actual build footprint
(NixOS closure + guest images, single-digit GiB) fits easily in the
remaining ~116 GiB.

## Fix Direction (separate commit)

Calibrate the default floor to 100 GiB — still ~30x the observed build
footprint — with a comment naming this receipt. Residual risk: the host
disk will hit a real wall as the platform grows further; the durable fix is
disk expansion on Node B (owner/infrastructure decision) and/or a
product-path lifecycle for sealed/dead computer state (overhauls-scale).
