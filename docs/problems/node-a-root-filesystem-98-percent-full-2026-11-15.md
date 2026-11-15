# Node A's root filesystem is 98% full

Recorded 2026-11-15T03:00Z, found during the memory investigation
(`node-a-oom-killed-the-agents-build-vm-2026-11-14.md`). Not repaired.

## Problem

Node A's one btrfs filesystem (RAID 1 over two NVMe drives) holds the Nix
store, every VM's disk image, Mike's containers and the Choir off-host
backup. It has 28 GiB left.

## Evidence (read only)

- `df -h /`: 953G size, 920G used, 29G available, 98%. `btrfs filesystem
  usage`: data 914.19 of 942.23 GiB used, free (estimated) 28.04 GiB.
- `du -xsh`: `/nix/store` 543G, `/var/lib/containers` 15G, `/root` 8.3G.
- VM images (allocated / apparent): agents 119G / 129G, runner 47G / 65G,
  ws-owner 9.2G / 65G, ws-01 to ws-04 about 0.27G / 33G each. The images
  are sparse (`home.img` on the runner is NOCOW), so the 160 GiB of
  apparent space not yet allocated can be claimed by guests writing.
- Mike's staging check logged "disk: / is 91% full" at 2026-11-14 06:20Z.
- `nix.gc` runs weekly with `--delete-older-than 14d` (last 2026-11-09,
  50 s, 7.1G written); the store is still 543G.

## Belief

When the filesystem fills, every guest's writes fail at once, Mike's
workstations included, and btrfs slows sharply well before that. The build
VMs write the most (Go build caches and test temp files on their home
disks). It also rules out a disk swapfile as a memory cushion on this host.

## Remaining error

What holds the store live (GC roots: `/root/result`, the system generations,
the auto roots) is not yet known, nor how much a full GC would free. Next,
read only: `nix-store --gc --print-dead | wc -l` with sizes, and the GC
roots list; then a GC (the shared-host rules allow one at any time) and a
`go clean -cache` inside the agents VM when it is idle.
