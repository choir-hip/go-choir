# Node A — shared NixOS host

Owner decision, 2026-10-09: node-a is shared. Choir keeps using it as Node
B's Nix remote builder; the rest of the machine is free for another
project, which will also run NixOS.

## The machine

- Address 51.81.93.94 (`eno1`, DHCP). choir-ip.com (147.135.24.51) also
  reached it through the provider's front address; that site was the old
  Choir mirror and was taken down on 2026-10-09 (owner-approved).
- 12 cores, 31 GiB RAM, ~950 GB btrfs on software RAID (`/` and `/data`
  subvolumes; `nix/node-a-disks.nix`, `nix/node-a-hardware.nix`).
- NixOS config in this repo: `nixosConfigurations.go-choir-a`
  (`nix/node-a.nix`). Firewall: port 22 only.

## What Choir needs from it

Only the builder. Node B connects as root over `ssh-ng` with a key that
can run nothing but `nix-daemon --stdio`
(`nix/modules/choir-nix-builder-host.nix`), and sends builds while a deploy
runs: a few minutes per deploy, idle otherwise. If node-a is unreachable,
Node B builds locally, so deploys keep working, just with more load on the
production host.

## Rules for another project on this host

1. **Keep the builder module.** If your project takes over the NixOS
   config, import `choir-nix-builder-host.nix` (copy it if you use another
   repo). It adds one restricted root key and the `system-features` Node B
   requests. Without it, Choir silently falls back to local builds.
2. **Keep the Nix daemon available.** Do not set `nix.enable = false` or
   remove root's ability to run `nix-daemon --stdio`.
3. **Share the Nix store.** A store GC is fine at any time; Choir builds
   are reproducible and re-fetch from cache.nixos.org.
4. **Leave headroom during Choir deploys.** A deploy build can use most
   cores for a few minutes. If your workload is latency-sensitive, cap
   Choir with `nix.settings.max-jobs` / `cores` on this host, or tell the
   Choir owner.
5. **Ports.** Choir needs only 22. Ports 80/443 and everything else are
   yours.

## Changing who manages the config

Today this repo deploys node-a. To hand management to another project:
move `nix/node-a-hardware.nix`, `nix/node-a-disks.nix` and the builder
module into that project's flake, keep the root SSH key for the owner, and
delete `go-choir-a` from this flake in the same week so two repos never
fight over the host.

## Handover state (2026-10-09 16:45Z)

- systemd (PID 1) had crashed with SIGBUS on 2026-08-23 22:54Z and frozen;
  from then until the reboot nothing could start or stop services (the
  old stack kept running, choir-ip.com included). Rebooted with sysrq into
  the minimal config, owner-approved. Up and `running`.
- Removed: the old Choir mirror (services, its VM, Caddy for
  choir-ip.com), `/var/lib/go-choir` (221 GB control-plane Dolt from
  May–August plus ~5 GB), old system generations (store GC freed 129 GB).
  Disk: 39 GB used, 907 GB free.
- Root keys: the owner's key and the builder key (declared). The old CI
  deploy key was removed from `/root/.ssh/authorized_keys`; backup at
  `/root/.ssh/authorized_keys.pre-share-20261009`.
- Still present, not covered by the approval: `/data` (23 GB, the `@data`
  btrfs subvolume: March-era per-user data and snapshots from the earlier
  Choir), and small `/var/lib/{qdrant,searx,marco}` directories.
- Verified: Node B built an uncached derivation on node-a over the builder
  key and copied the result back.

## Deploying from this repo

```sh
nix run nixpkgs#nixos-rebuild -- test   --flake .#go-choir-a \
  --target-host root@node-a --build-host root@node-a
# check `ssh node-a true` from a new connection, then:
nix run nixpkgs#nixos-rebuild -- switch --flake .#go-choir-a \
  --target-host root@node-a --build-host root@node-a
```

`test` activates without changing the boot entry, so a bad config is gone
after a reboot.
