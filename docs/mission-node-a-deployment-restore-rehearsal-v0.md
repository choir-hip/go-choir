# Mission: Node A Deployment — Offsite Replica, Restore Rehearsal, Shadow Compute

**Date:** 2026-06-28
**Status:** active — first executable shape
**Class:** red — deployment routing, protected surface (new deployment target)
**Spec lineage:**
- `docs/choir-base-product-spec-2026-06-06.md` §Node A / Node B, Landmark 4
- `docs/choir-base-research-report-2026-06-06.md` §Resource Strategy For Node A / Node B
- `docs/memo-artifact-program-doctrine-2026-06-28.md` §Shadow replication

## Problem Statement

Node B (choir.news, OVH bare metal) is the sole deployment target. All
canonical state — platform Dolt DB, auth DB, source service DB, mail DB,
VM state (data.img), frontend bundle — lives on a single physical machine
with RAID1 (availability, not backup). There is no offsite replica, no
restore rehearsal, and no shadow compute target.

Before Base claims durability beyond alpha (product spec Landmark 4), Node
A must prove restore from Node B backup artifacts. The artifact program
doctrine requires shadow replication to a second node to prove that
`computer = choir_code(artifact_program)` is deterministic and lossless.

This is the lynchpin: without Node A, the artifact program doctrine is
untested theory. With Node A, every backup can be rehearsed, every restore
can be proven, and every shadow comparison reveals decomposition gaps.

## Current State

- Node B: full NixOS config (`nix/node-b.nix`), CI deploy job, 8 host
  services, Firecracker VM lifecycle, storage reporting/snapshot tooling
- Node A: does not exist. No NixOS config, no flake output, no scripts,
  no CI job, no SSH target

## Conjecture

A minimal NixOS configuration for Node A can be built that:
1. Receives backup artifacts from Node B (Dolt DB, auth DB, source DB,
   mail DB, VM data.img snapshots) via rsync over SSH
2. Verifies hash/size integrity of received artifacts
3. Runs a restore rehearsal: mounts the restored Dolt DB, queries it,
   and confirms structural integrity against Node B's live state
4. Hosts a shadow VM from restored data.img and compares its state
   against the primary VM
5. Reports divergence or confirmation as typed evidence

The restore rehearsal proves the backup artifacts are sufficient to
reconstruct the computer. The shadow comparison proves the artifact
program is deterministic and lossless.

## Protected Surfaces

- **Deployment routing:** adding Node A as a deployment target does not
  change Node B's deploy path. Node A receives backups, not live traffic.
- **Node B SSH:** Node A pulls from Node B using a dedicated read-only
  SSH key. Node B does not push to Node A (pull model, not push).
- **Backup artifacts:** restore rehearsal operates on copies, never on
  Node B's live state. Restore rehearsal is non-destructive to Node B.
- **Shadow VM:** runs on Node A in isolation. No network route to Node B
 's live VMs. Shadow VM is read-only comparison, not a live instance.

## Rollback Path

Node A is additive infrastructure. If the NixOS config or scripts are
wrong, Node A simply fails to build or fails to receive backups. Node B
is unaffected. Rollback = revert the commit; Node A returns to
non-existence. No data on Node B is touched.

## Implementation Plan

### Phase 1: NixOS Configuration + Backup Replication

1. `nix/node-a.nix` — NixOS host config for Node A:
   - SSH server (receive-only, no public services)
   - rsync, jq, curl, dolt, go (for restore rehearsal tooling)
   - Backup storage directories: `/var/lib/go-choir-a/backups/{dolt,auth,source,mail,vm-state}`
   - Restore rehearsal workspace: `/var/lib/go-choir-a/restore-rehearsal/`
   - Shadow VM workspace: `/var/lib/go-choir-a/shadow-vm/`
   - Systemd timer for periodic backup pull + restore rehearsal
   - Firewall: SSH only (port 22), no public ports

2. `nix/node-a-hardware.nix` — generic hardware config (Node A's
   physical hardware is not fixed; uses NixOS hardware scan)

3. `flake.nix` — add `nixosConfigurations.go-choir-a`

4. `scripts/node-a-backup-pull` — pulls backup artifacts from Node B:
   - rsync Dolt DB dump, auth.db, sourcecycled.db, mail.db
   - rsync VM data.img snapshots (typed metadata sidecars)
   - Verifies SHA-256 of each received artifact
   - Records pull manifest with timestamps and hashes

5. `scripts/node-a-restore-rehearsal` — runs restore rehearsal:
   - Mounts restored Dolt DB, runs integrity queries
   - Compares table counts and latest commit hashes against Node B
   - Verifies auth.db, sourcecycled.db, mail.db open and query cleanly
   - Records rehearsal evidence (pass/fail per artifact, divergence report)

### Phase 2: Shadow VM Comparison (after Phase 1 proves out)

6. `scripts/node-a-shadow-compare` — boots a shadow VM from restored
   data.img and compares state against primary:
   - Uses the same guest image artifacts as Node B
   - Boots shadow VM in read-only comparison mode
   - Compares filesystem manifest, Dolt state, blob materialization
   - Reports divergence as typed evidence

### Phase 3: CI Integration

7. CI job: `node-a-restore-rehearsal` — triggered after successful
   Node B deploy, pulls latest backups and runs restore rehearsal.
   Reports evidence as a CI artifact. Non-blocking initially (advisory),
   becomes blocking when rehearsal is stable.

## Evidence Class

- **Backup pull manifest:** typed record of what was pulled, when, with
  what hashes. Admissible as evidence that replication occurred.
- **Restore rehearsal report:** pass/fail per artifact, integrity query
  results, divergence report. Admissible as evidence that backups are
  sufficient to reconstruct state.
- **Shadow comparison report:** (Phase 2) divergence or confirmation
  between shadow and primary. Admissible as evidence for the artifact
  program doctrine's determinism proof.

## Acceptance Criteria

- `nix build .#nixosConfigurations.go-choir-a.config.system.build.toplevel`
  succeeds
- `scripts/node-a-backup-pull` pulls and verifies all backup artifacts
- `scripts/node-a-restore-rehearsal` runs and produces a rehearsal report
- Restore rehearsal confirms: Dolt DB opens, tables query, auth/source/
  mail DBs open and query cleanly
- No Node B state is modified by any Node A operation

## Heresy Delta

- `discovered`: Node B has no offsite backup or restore rehearsal (known
  gap, now being addressed)
- `introduced`: 0 (this mission adds infrastructure, doesn't change
  existing behavior)
- `repaired`: 0 (the gap is being closed, not yet repaired)

## Residual Risks

- Node A's physical hardware is not yet provisioned. The NixOS config
  can be built and tested in a VM, but the restore rehearsal needs a
  real second node to pull from Node B over the internet.
- The backup pull model assumes Node B's SSH is reachable from Node A.
  If Node A is behind NAT, the pull direction may need to reverse.
- Shadow VM comparison (Phase 2) requires the same guest image artifacts
  as Node B. These are large (~GB) and need to be replicated to Node A.
