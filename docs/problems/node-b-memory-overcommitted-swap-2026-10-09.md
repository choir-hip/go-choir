# Node B memory is overcommitted; swap is full and builds share the host with guests

Date: 2026-10-09. Status: open. Owner direction: "we have to make sure we
stop swapping. swapping is a service degradation. we cant tolerate it."
Operational invariant O11 (memory budgets). Mutation class of a fix: red
(host OS, VM lifecycle, deploy routing).

## Evidence (06:35–06:45 UTC, during forced deploy run 37892708774)

**Host state:**
- 12 cores, 31 GiB RAM.
- Load average 61 / 55 / 35.
- Swap: `/swap/swapfile` 16 GiB, **SwapFree 30 MB**.
- `Committed_AS` 37.6 GB vs `CommitLimit` 33.1 GB.
- Since boot (210 days): `pswpout` 1.11 B pages (~4.2 TiB), `pswpin` 0.56 B.
- Memory PSI at the time was low (`full avg60=0.12`). The swapped pages
  are mostly cold, left from the 16 GiB-guest period
  ([`guest-store-history-bloat-and-memory-shape`](guest-store-history-bloat-and-memory-shape-2026-10-09.md)).
  The load is CPU: CI.

**What was running:**
- Two concurrent `nix build .#nixosConfigurations.go-choir-b…toplevel`
  (forced deploy, plus the next push run's prebuild), plus Go compiles.
- corpus Dolt at 3.7 GB RSS; platform Dolt at 1.0 GB RSS.
- Firecracker guests: the owner at 3.3 GB (cap 4 GiB); others at 1.2,
  0.4, 0.2 and 0.05 GB.

**Declared ceilings** (`nix/node-b.nix`; live `systemctl show`):

| Consumer | Ceiling |
|---|---|
| corpus Dolt | MemoryHigh 18 GiB / MemoryMax 20 GiB |
| checkpointd | MemoryMax 16G (oneshot, `OOMScoreAdjust=1000`) |
| nix-daemon (CI builds on the production host) | **none**; `max-jobs=12`, `cores=0` (all) |
| Interactive guests | 4 GiB each since 50c7074b; **no cap on how many run at once** |
| Kernel | `vm.swappiness=60`; 16 GiB swap file (`nix/disks.nix`) |

The ceilings sum to far more than RAM. Nothing reserves memory for guests
or for platform services.

**Related:** `nix.settings.min-free` is 120 GB, but the root fs has 107 GB
free. Every build therefore starts a Nix GC; see the 2026-10-09 note that
`nix shell` triggered auto-GC.

## Problem

There is no host memory budget:
- CI builds, checkpoint maintenance and the corpus database may each
  take most of RAM;
- the count of running guests is unbounded;
- the kernel resolves the overcommit by swapping guests and services out.

Owner direction: swapping is service degradation and is not tolerated.

## Fix direction

1. **Budget.** Declare a host memory budget in `nix/node-b.nix`:
   - reserve for platform services;
   - a bounded slice for builds (`nix-daemon` MemoryHigh/MemoryMax,
     `max-jobs`/`cores` limits);
   - tighter Dolt and checkpointd ceilings;
   - the remainder for guests.
2. **Admission.** vmctl admits running guests against the guest share
   (count × 4 GiB); beyond it, it hibernates idle guests or refuses.
3. **Kill order.** Under pressure, builds and the checkpoint worker are
   killed before guests and platform services (`OOMScoreAdjust`).
4. **Swap.** Turn swap off (or `swappiness` ≈ 1 as a step) only after
   1–3 hold. Without them, removing swap converts degradation into
   OOM kills.
5. **Builds off the host (longer term).** The production host should not
   compile.

## Applied (2026-10-09 ~07:20–08:00 UTC)

- **Deploy 16806d8e (run 37895020834, success)** applied the budget.
  Verified live:
  - vmctl `MemoryLow=14G`;
  - nix-daemon `MemoryHigh 6G / MemoryMax 7G`;
  - checkpointd `4G / 6G`;
  - `vm.swappiness=1`.
- **corpus Dolt** still read 18G/20G from two `systemctl set-property`
  drop-ins in `/etc/systemd/system.control/go-choir-corpus-dolt.service.d/`
  (`50-MemoryHigh.conf`, `50-MemoryMax.conf`). Those outrank the Nix unit.
  - Moved to `/var/tmp/systemd-control-backup-20261009/`, then `daemon-reload`.
  - Now `MemoryHigh 5G / MemoryMax 7G / MemorySwapMax 0`, at 4.2 GB in use.
  - **Rollback:** move the files back and `daemon-reload`.
- **Swap use** fell from 15 GiB to 5 GiB as the 16 GiB guests went away.
  The swap file itself is still present; removing it is the next step,
  once usage drains.
- **Admission:** vmctl pressure reclaim is already `mode=active`
  (hibernates idle, unprotected guests under memory pressure). No new
  admission code was needed.
