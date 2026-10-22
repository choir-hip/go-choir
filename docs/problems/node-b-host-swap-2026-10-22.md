# Node B host swap: what depends on it, what changes when it goes

Date: 2026-10-22. Lane: `lane/fp9-remove-swap` (FP0, FP9 Q4). Class: red (host
config; build only, FP0 lands and deploys).

## Decision provenance

Owner, 2026-10-22 (`docs/reports/owner-notes-2026-10-14.md` §40, FP9 Q4):
"default. Prob remove swap. Swap seems to lead to compounding problems".
Earlier owner direction (2026-10-09,
`docs/problems/node-b-memory-overcommitted-swap-2026-10-09.md`:4, :53):
"swapping is service degradation and is not tolerated". Density finding L15
(`docs/reports/vm-density-limits-2026-10-14.md:327`) rejected swap/zswap.

## State today

- `nix/disks.nix:29-33` mounts `/swap` (btrfs subvolume `@swap`, `noatime`);
  `nix/disks.nix:35-37` declares `swapDevices = [{ device = "/swap/swapfile"; }]`,
  a 16 GiB file. About 186 MiB in use, 31.8 GiB RAM.
- `nix/node-b.nix:2319` sets `vm.swappiness = 1`.
- The 2026-10-09 record (fix direction 4, :67-69) said swap goes only after
  the budget, admission and kill order hold, "otherwise removing swap converts
  degradation into OOM kills". Items 1-3 landed on 2026-10-09 (see below), and
  the record names removal as "the next step" (:88).

## Dependency table

| # | Place | What it assumes about host swap | Changes when swap goes |
|---|-------|---------------------------------|------------------------|
| 1 | vmctl admission, `internal/vmctl/host_admission.go:19` | `MemAvailable >= floor + headroom + reserved + need`. Reads only `MemAvailable` (`pressure_reclaim.go:453`, `hostbuild/service.go:713`). No `SwapFree` or `SwapTotal` read anywhere in Go (grep). | Nothing. `MemAvailable` never counted swap. |
| 2 | The floor | `VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT=15` (`nix/node-b.nix:1400`); 15% of MemTotal is the 4779 MiB floor. Not a swap number. | Nothing. |
| 3 | Guest cgroups (FP8 jailer) | `internal/vmmanager/confinement.go:333-336` sets only `pids.max` and `cpu.weight` on the jailed leaf. Swap is bounded by the ancestor: the vmctl unit has `MemorySwapMax = "0"` (`nix/node-b.nix:1225`), and cgroup v2 `memory.swap.max` is hierarchical. `vm-density-limits` L3 also relies on "guests cannot swap" (:314). | Nothing. Guests already never swap; the paper's 8 GiB guest (`VM_INTERACTIVE_MEM_MIB=8192`, :1305-1311) behaves the same. |
| 4 | Units with `MemorySwapMax = "0"` | platform-dolt (:881), corpus-dolt (:924), checkpointd (:1091), vmctl (:1225), dolt-squash (:1671), nix-daemon (:2316), store-compaction (`internal/vmctl/store_compaction.go:150`). | No-op. Already unswappable. |
| 5 | `go-choir-offhost-backup`, `nix/node-b.nix:386-393` | `MemorySwapMax = "4G"`; comment says restic exceeding the line "fails loudly instead of paging". Restic measured about 13 GB against `MemoryMax=12G`. | The 4G swap allowance disappears. A run that outgrows 12G is killed at `MemoryMax` either way; the 4G of paging only delayed that. The comment goes stale (follow-up F2). No functional regression: it fails at `MemoryMax` either way. |
| 6 | Units with no swap limit: gateway (:1857), proxy (:736), auth (:691), maild (:808), corpusd (:953), goproxy (:2035, `MemoryMax=1G`), front-importance (:1137, `MemoryMax=512M`), sshd, systemd | Cold anonymous pages of these can be paged out today (the ~186 MiB). | Those pages stay resident (a few hundred MiB at most). Under cgroup `MemoryMax` the two capped units are OOM-killed instead of paging first. Both are non-critical and already `OOMScoreAdjust` 500/1000. |
| 7 | `vm.swappiness = 1`, `nix/node-b.nix:2319` | Tuning for a swapped host. | Inert with `SwapTotal=0`. Leave it: it is harmless, and a rollback to swap then keeps the low-swappiness behavior. (F3: delete it later.) |
| 8 | oomd / earlyoom / zram / zswap in `nix/` | None configured (grep of `nix/` and `flake.nix`). NixOS ships `systemd-oomd` with `enableRootSlice`/`enableSystemSlice`/`enableUserSlices` off, so it monitors nothing. Unverified on the host. | Nothing expected. FP0 confirms with `oomctl` (probe step 4). |
| 9 | Balloon and hotplug, `VM_BALLOON_FREE_PAGE_REPORTING=1` (:1324-1328), `VM_MEMORY_HOTPLUG=on` (:1341); `internal/vmctl/memory_hotplug.go:908` (`hostBelowMemoryFloor`) | Uses host `MemAvailable` against the floor. No swap term. | Nothing. |
| 10 | Pressure reclaim, `pressure_reclaim.go:21-27` | PSI memory `some` avg and `MemAvailable`. | PSI will not be inflated by swap IO any more. Thresholds unchanged. |
| 11 | Deploy activation, `.github/workflows/ci.yml:1311-1323` (`switch-to-configuration switch`) | `swapDevices` becomes a `swap-swapfile.swap` unit. A switch that drops the entry stops that unit: `swapoff /swap/swapfile`, which pages the in-use ~186 MiB back in. | `swapoff` runs live during the deploy. With about 14 GB available it should take seconds. If `swapoff` fails (pages cannot be placed), the unit stays failed and the switch reports it (non-zero exit risk). Mitigation: FP0 runs `swapoff /swap/swapfile` by hand before the deploy and confirms `SwapTotal: 0`. Bounded by the sequencing below. |
| 12 | Docs and probes that read swap | `scripts/dolt-liveness-probe.sh:115` reads `VmSwap` from `/proc/<pid>/status` as a diagnostic (reads 0). `scripts/vmm_confinement_verify.py:155` reads the leaf's `memory.swap.max` (reads ancestor-derived `max`, unchanged). `docs/evidence/playwright-capsule-memory-2026-10-15/run.sh:24` sets its own. `docs/problems/store-a-weekly-squash-2026-10-20.md:74` and `no-off-host-backup-2026-10-20.md:528` record past swap as evidence, history only. | None. |
| 13 | RES-13 R2 and FP9 vault-canary / secret-in-swap reasoning | Searched `docs/**` for "secret in swap", "vault canary" and swap; no document makes a reasoning step that depends on swap existing. Where swap would matter for secrets, removal helps: secrets in guest and gateway memory can no longer reach a disk-backed swapfile. | A mild gain. FP9 may now state "no host swap" as a fact once FP0 confirms `SwapTotal: 0` on the host. |
| 14 | `/swap` mount (`nix/disks.nix:29-33`) and 16 GiB file | Only used as the swap backing. | Kept (decision below). |

## What OOMs instead of swapping

Without swap, a host shortfall that swap used to absorb for a while becomes
reclaim of page cache first, then a global OOM kill. The kernel kills the
process with the highest `oom_badness`: RSS (plus page tables) plus
`oom_score_adj` times MemTotal/1000, so adj 1000 adds about the whole of RAM
and adj -900 makes a process effectively unkillable.

Order the kernel would pick (from nix and Go; live values are FP0's to read,
probe step 3):

| Order | Process | oom_score_adj | Source |
|-------|---------|---------------|--------|
| first | nix-daemon (builds), checkpointd, front-importance | +1000 | `nix/node-b.nix:2317`, :1096, :1149 |
| next | ordinary (unprotected) Firecracker guests | +500 | `internal/vmmanager/process_control.go:22`, `host_capacity.go:36` |
| next | goproxy | +500 | `nix/node-b.nix:2064` |
| middle | gateway, proxy, auth, maild, corpusd, corpus-dolt, dolt-squash, offhost-backup | 0 (unset) | no `OOMScoreAdjust` in those units |
| late | protected-class Firecracker guests (owner, `critical_protected`, `public_platform`) | -400 | `host_capacity.go:37`, set by vmctl's process reconcile (:128) |
| late | platform-dolt (Store A) | -500 | `nix/node-b.nix:882` |
| last | vmctl | -900 | `nix/node-b.nix:1230` |

Findings:

- Gateway, proxy, auth, maild and corpusd sit at 0, above the guests'
  and Dolt A's protected tier but below the +1000 and +500 tier. A global
  OOM takes a build, the checkpoint worker or an ordinary guest first.
  Corpus-dolt (adj 0, up to `MemoryMax=7G`) scores about its RSS, far below
  an ordinary guest's RSS plus the +500 bonus. Gateway and proxy have no
  `MemoryMax` at all (F1).
- Protected computers (the paper if classed `public_platform`) sit at -400.
  An 8 GiB guest at -400 scores about 8 GiB minus 12.4 GiB, below zero, so it
  is clamped to 1: effectively last, in the same bucket as Dolt A.
- vmctl's `MemoryLow=14G` reserves guest memory against reclaim from other
  cgroups; it does not add swap and does not change.
- Within-cgroup kills (a unit exceeding `MemoryMax`) behave as today, except
  the units that used to page before hitting the limit (items 5 and 6) now
  die at the limit.
- The failure the owner named, "compounding": swap IO inflates PSI, which
  trips pressure reclaim, which hibernates computers, which writes more
  disk. Removing swap cuts that loop at the source.

Is the paper's 8 GiB guest safe? Yes, and unchanged: it never had swap
(item 3), and its admission is `MemAvailable` based (items 1 and 2).

## Decision on the `/swap` mount and file

Keep the `fileSystems."/swap"` mount and the file. Reasons: rollback is a
one-line re-add of `swapDevices`; the mount costs nothing; deleting a 16 GiB
NOCOW file on a btrfs RAID1 root is a separate disk operation that belongs in
the disk-free mission, not in a swap-policy change. The cost is 16 GiB of
btrfs space that stays allocated until the file is deleted by hand later (F4).

## Change

`nix/disks.nix`: delete `swapDevices`. Nothing else.

## Verdict: standalone deploy

Safe as a standalone deploy, with one hand step and one caveat:

1. The dependent budget (ceilings, `OOMScoreAdjust`, `MemorySwapMax=0` on
   guests and on all big services, admission by `MemAvailable`) already
   landed on 2026-10-09 and is still in the tree. The record's own
   precondition holds.
2. Guests and every large service are already swap-free, so removal changes
   behavior only for a few hundred MiB of cold pages in small services.
3. The only live-risk step is `swapoff` of about 186 MiB during the switch.
   FP0 does that by hand first (below) so the switch has nothing to stop.
4. Restart behavior: no service unit changes, so no restarts are caused by
   this change; the switch only stops one `.swap` unit. A crash or
   planned-restart marker is not involved.

### Rollback

Re-add the `swapDevices = [{ device = "/swap/swapfile"; }];` block to
`nix/disks.nix` (the subvolume, mount and file are untouched) and deploy;
or live: `swapon /swap/swapfile`. Git revert of the red commit is the repo
path.

## FP0 steps and probe (not run by this lane)

1. Before: `free -m`, `cat /proc/meminfo | grep -E 'MemAvailable|Swap'`,
   `swapon --show`; record the top swap users:
   `for p in /proc/[0-9]*; do awk '/^VmSwap/{s=$2} /^Name/{n=$2} END{if(s>0)print s,n}' $p/status; done | sort -rn | head`.
2. `swapoff /swap/swapfile` by hand, confirm `SwapTotal: 0 kB`, and that
   vmctl `/health` and the paper are healthy.
3. Read live OOM scores: for vmctl, gateway, proxy, auth, corpusd, platform-dolt,
   corpus-dolt and each firecracker pid, `cat /proc/<pid>/oom_score_adj
   /proc/<pid>/oom_score`; compare to the table above.
4. `oomctl` (confirm no monitored slices) and `systemctl status systemd-oomd`.
5. Deploy; confirm the switch stopped `swap-swapfile.swap` and exited 0,
   `swapon --show` empty, `SwapTotal: 0`, `vm.swappiness` unchanged, and
   `/swap` still mounted.
6. Soak: watch memory PSI `some avg60` and `MemAvailable` through a normal
   busy period and one deploy-refresh; no oom-kill lines in `journalctl -k`.

## Follow-ups (not in this lane)

- F1: no `MemoryMax` or `OOMScoreAdjust` on gateway, proxy and auth. With no
  swap a runaway one OOMs the host rather than thrashing it. Give each a
  generous ceiling and adj to place them in the late bucket.
- F2: update the stale offhost-backup comment (`nix/node-b.nix:386-393`) and
  drop `MemorySwapMax = "4G"` (no-op now).
- F3: delete `vm.swappiness` and the stale "Host swap is bounded" sentence
  (`nix/node-b.nix:1305`) once removal has soaked.
- F4: delete the 16 GiB swapfile and the `@swap` subvolume once the soak is
  clean (reclaims 16 GiB).

## Heresy delta

discovered: gateway/proxy/auth have no memory ceiling (F1); stale swap
comments (F2, F3). introduced: none. repaired: host swap, an owner-named
service degradation, removed.

Conjecture delta: with the 2026-10-09 budget in place, removing host swap
changes only a few hundred MiB of cold small-service pages and the compounding
swap-IO to PSI to hibernate loop; admission is unaffected. Wrong if the soak
shows any oom-kill of a service not in the +1000/+500 tier.
