# Mission: Node A — Full Mirror Deployment and Update

**Date:** 2026-06-28
**Status:** active — restoring correct config to main, then updating deployed node
**Class:** red — deployment routing, protected surface (production mirror)
**Spec lineage:**
- `docs/choir-base-product-spec-2026-06-06.md` §Node A / Node B, Landmark 4
- `docs/choir-base-research-report-2026-06-06.md` §Resource Strategy For Node A / Node B

## Problem Statement

Node A (choir-ip.com, 51.81.93.94) is a full Choir mirror deployed from the
`codex/redesign-hard-cutover-node-a` branch (commit `870d4d48`). It runs all
8 host services (auth, gateway, maild, platformd, platform-dolt, proxy,
sandbox, vmctl) plus Caddy serving `choir-ip.com`.

The Node A NixOS config files (`nix/node-a.nix`, `nix/node-a-hardware.nix`,
`nix/node-a-disks.nix`) were never merged to main. A prior attempt to add
Node A infrastructure to main created WRONG config files that treated Node A
as an unprovisioned offsite backup replica, overwriting the real configs
with a minimal SSH-only config that would have destroyed all running
services if deployed.

The real Node A config imports `nix/node-b.nix` (full service stack) and
overrides:
- hostname: `go-choir-a`
- Caddy virtualHosts: `choir-ip.com` (instead of `choir.news`)
- auth service env: `AUTH_RP_ID=choir-ip.com`, `AUTH_RP_ORIGINS=https://choir-ip.com`
- maild service env: `MAILD_PRIMARY_DOMAIN=choir-ip.com`

## Current State

- Node A is live at 51.81.93.94, running NixOS 26.05 (Yarara)
- Hostname: `choiros-a` (config override: `go-choir-a`)
- All 8 Choir services running
- Caddy serving choir-ip.com
- Repo at `/opt/go-choir` at commit `da3576a` (behind main)
- 31GB RAM, 12 cores, 953GB btrfs on md126
- SSH alias: `node-a` (from `~/.ssh/config`)

## Conjecture

Restoring the original Node A config files from the
`codex/redesign-hard-cutover-node-a` branch to main, then pulling main
on Node A and running `nixos-rebuild switch --flake /opt/go-choir#go-choir-a`
will update Node A to the latest main without breaking running services.

## Protected Surfaces

- **Deployment routing:** Node A serves choir-ip.com with its own Caddy
  config. The deploy must preserve the hostname and virtualHost overrides.
- **Running services:** all 8 services must continue running through the
  rebuild. `nixos-rebuild switch` handles service restarts atomically.
- **Auth config:** `AUTH_RP_ID` and `AUTH_RP_ORIGINS` must remain
  `choir-ip.com`, not `choir.news`.
- **Mail config:** `MAILD_PRIMARY_DOMAIN` must remain `choir-ip.com`.

## Rollback Path

If the rebuild breaks services, `nixos-rebuild switch --rollback` restores
the previous generation (generation 36 as of 2026-05-29). The previous
system closure remains in the nix store.

## Implementation Plan

1. Restore `nix/node-a.nix`, `nix/node-a-hardware.nix`,
   `nix/node-a-disks.nix` from `870d4d48` to main
2. Fix `flake.nix` `nixosConfigurations.go-choir-a` to match original
   (specialArgs with goChoirPackages, buildCommit, sourceRepoRemote,
   guestRunner; modules: hardware, disks, node-a)
3. Delete wrong files: `nix/node-a-rehearsal-report-template.json`,
   `scripts/node-a-backup-pull`, `scripts/node-a-restore-rehearsal`
4. Update deploy-impact-classify: Node A files ignored for Node B deploys
   (Node A has its own deploy path via SSH)
5. Commit, push to main
6. SSH to node-a: `cd /opt/go-choir && git pull origin main`
7. `nixos-rebuild switch --flake /opt/go-choir#go-choir-a`
8. Verify: `curl https://choir-ip.com/health`, check service status

## Evidence Class

- **Health endpoint:** `curl https://choir-ip.com/health` returns JSON
  with service status and deployed commit
- **Service status:** `systemctl status go-choir-*` on node-a
- **Build identity:** health endpoint reports deployed commit SHA

## Acceptance Criteria

- `nix build .#nixosConfigurations.go-choir-a.config.system.build.toplevel`
  succeeds
- `nixos-rebuild switch` on node-a completes without errors
- All 8 services running after rebuild
- `curl https://choir-ip.com/health` returns `status: ok`
- Health endpoint reports the latest main commit SHA

## Heresy Delta

- `discovered`: Node A config files were never merged to main; a prior
  attempt created wrong configs that would have destroyed running services
  if deployed (caught before deployment, no damage)
- `introduced`: 0 (restoring original configs, not introducing new behavior)
- `repaired`: Node A config files now on main, matching what's deployed

## Residual Risks

- Node A repo at `/opt/go-choir` is at commit `da3576a`, significantly
  behind main. The `git pull` will bring many changes; the rebuild may
  surface build issues from the large delta.
- The `codex/redesign-hard-cutover-node-a` branch may have other changes
  not in main that Node A depends on. Need to verify the rebuild succeeds
  before switching.
