# Node A memory budget: a second build VM without overcommit

2026-11-15. Problem: `docs/problems/node-a-oom-killed-the-agents-build-vm-2026-11-14.md`
(evidence and numbers are there). Patch to Node A's config:
[node-a-memory-budget-2026-11-15.patch](node-a-memory-budget-2026-11-15.patch).
Class: orange (host memory policy, VM sizing; no VM of Mike's restarts).
Not applied; this is the reviewed form.

## Where the config lives

Node A's NixOS config is not in this repo. It is Mike's infra flake: deployed
from `/root/mike-infra` on Node A (not a git checkout), with its source on the
owner's local branch `infra/node-a-in-line-2026-11-14` of the Mike repo
(not pushed; the repo is public, and pushing is the owner's call). Today the
deployed tree evaluates to exactly the running system
(`xasyjjzr...-nixos-system-node-a-26.11.20260701.ec84054`). The branch is
not the same: it also carries Mike's layer ledger, which Node A has never
run, so it must not be copied over Node A. The patch touches only files that
are identical in both, and applies cleanly to both (checked: `patch -p2` on
a copy of the deployed tree gives exactly the evaluated tree; `patch -p1
--dry-run` on the branch is clean).

## What the thing is

A guest's configured memory is a commitment, not an average. Guest RAM is a
shared memfd (`chv-shared-memory.nix`), so every page a guest touches is host
shmem, which the host cannot reclaim without swap. A Linux guest fills its RAM
with page cache within one heavy run and, with no balloon, never returns it.
So the "overcommit while idle" of 2026-11-14 was a delayed out-of-memory kill:
it came the first time both build VMs were full. The question is not how to
overcommit safely tonight but how to get the most build throughput from what
fits.

## Physical bounds

| | MiB |
|---|---|
| RAM (MemTotal 32,630,604 kB) | 31,865 |
| Mike's workstations, fixed (ws-owner 6,144, ws-03 3,072, ws-04 3,072) | 12,288 |
| Host reserve (measured at the kill: 2.5 GiB anon, 0.6 GiB kernel, 0.8 GiB tmpfs; plus 1 GiB page cache) | 5,120 |
| Left for the Choir build VMs | 14,457 |
| The runner tonight, resident until it next restarts | 10,240 |
| Left for the agents VM tonight | 4,217 |

What a heavy suite needs, measured in the runner during a landing's
`-race ./internal/agentcore` with a builder compiling beside it: 3.0 GiB guest
anonymous memory at peak (a builder's `compile` at 1.7 GiB, the test binary at
0.7 to 1.2 GiB), tmpfs at most 0.6 GiB. One heavy job alone needs about
2.5 GiB; two at once about 3.5 GiB. Inside the build VM the stall is disk
IO (PSI full 13 to 22%), not memory (1.4 s of stall in 25 h) or CPU (<1%).
CPU across the host: 12 cores; a heavy suite runs at GOMAXPROCS=4 with -p=1,
so two at once fit and three would contend with the load-sensitive tests
(`timing-tests-fail-under-load-and-cost-a-landing-each-2026-11-14.md`).

## Options

| Option | Bound it gives | Tonight? |
|---|---|---|
| Right-size the VMs so the configured sum fits RAM minus reserve | Hard: a guest cannot exceed its configured RAM, so the sum is the worst case | Yes for the agents VM (idle); the runner only when it is idle |
| Build-time assertion on that sum | A config that would overcommit does not evaluate | Yes |
| OOM order (`oom_score_adj`): Mike's VMs -1000, Choir VMs +300/+500, Node B builds +800 | If the kernel must kill, it never picks Mike's | Yes, at runtime, no restart |
| Keep Mike's VMs out of swap (`memory.swap.max=0`) | Their pages never go to zram | Yes, runtime, no restart |
| zram swap, about 4.7 GiB, zstd | Turns a burst beyond the reserve (Node B builds, Mike's container builds) into slowness | Yes, as a cushion only |
| Disk swapfile | Same cushion | No: the filesystem is 98% full |
| `MemoryMax`/`MemoryHigh` per VM unit | Bounds only host page cache of the disk image (guest RAM is already bounded) | Not needed |
| `MemoryLow` for Mike's VMs | Protects only their host page cache; needs `system.slice` low too (`vmctl-memory-low-protects-nothing-2026-10-24.md`) | Not needed |
| Balloon with free-page reporting (`microvm.balloon`, supported by the pinned microvm.nix) | Idle guests return freed pages, so a smaller reserve per VM | Durable candidate, untested with shared memfd memory |
| One large build VM instead of two | Shares page cache and Go build cache | No: one disk queue, and IO is the stall; the agents VM holds the logins the runner must not |

## Choice

Tonight (the patch): the agents VM at 4 GiB, the runner unchanged at 10 GiB,
so the configured sum is 26,624 MiB plus the 5,120 MiB reserve, 31,744 of
31,865. The config states what runs; nothing waits on a restart to become
true. Plus the assertion, the OOM order, Mike's VMs out of swap, and the zram
cushion. A guard unit sets the runtime parts at boot and every minute, so it
never edits Mike's units and a restarted VM gets its settings back within a
minute.

Why it is safe: each guest is hard-capped at its configured RAM, so with every
guest full the host keeps its 5 GiB reserve, measured 1.2 GiB above what the
host used at the kill. Two overlapping heavy suites run inside guests whose
maxima are already in the sum; an overrun inside a guest is that guest's own
OOM and kills a test there, never anything on the host. A burst on the host
beyond the reserve meets zram first, then the OOM order: a Node B build, then
the agents VM, then the runner, never ws-owner, ws-03 or ws-04.

Durable, when the runner is idle: runner 6 GiB and agents 6 GiB (sum 24,576;
2.1 GiB of slack beyond the reserve), each with room for one heavy suite and
a builder's compile beside it (3.0 GiB measured) plus page cache. Then measure whether the balloon with
free-page reporting returns memory from a shared-memfd guest; if it does, the
reserve can shrink and the VMs grow.

## Throughput

Heavy suites at once: 1 now (the runner only), 2 after the patch and the
agents VM's restart, still 2 after the 6/6 rebalance (CPU is the next bound).
What has to change to use it:

- The heavy-suite lock is a builders' convention (`~/heavy-suite.lock` on
  each VM), not code. Make it one lock per VM: builders run heavy suites on
  `choir-agents` under its lock; landings run on `choir-runner`
  (`LAND_VM=choir-runner LAND_VM_MIRROR=go-choir-mirror.git`). At 4 GiB the
  agents VM holds one heavy job at a time, and the lock must cover the
  compile of a heavy package (1.7 GiB), not only the test run.
- `scripts/land` does not take the lock today, so a builder's heavy suite and
  a landing can overlap on one VM. It should wrap the remote run in
  `flock ~/heavy-suite.lock` (yellow; a separate change).
- The landing's agentcore race package could then run as two shards, one per
  VM (`agentcore-race-suite-runs-at-its-timeout-2026-11-14.md`), now that
  both VMs fit at once.

## Apply (after review; on Node A as root)

0. Pre-checks. The agents VM is idle: `ssh choir-agents 'uptime; pgrep -c -x go; pgrep -c -f "[.]test( |$)"'`
   (both counts 0). Record the baseline for Mike's VMs:
   `systemctl show -p ActiveEnterTimestamp -p NRestarts -p MainPID microvm@ws-owner microvm@ws-03 microvm@ws-04`
   (today: started 2026-10-23 01:03:58, 01:02:58, 01:03:27; PIDs 635435,
   634347, 634862).
1. `cp -a /root/mike-infra /root/mike-infra.before-memory-20261115`
2. Copy the patch to `/root/`, then in `/root/mike-infra`:
   `patch -p2 --dry-run < /root/node-a-memory-budget-2026-11-15.patch` and,
   if clean, the same without `--dry-run`.
3. `nixos-rebuild dry-activate --flake /root/mike-infra#node-a`. Stop if any
   `microvm@` unit is listed to stop or restart. Expected: install-microvm-agents
   restarts (it only relinks `current`), the guard and zram units start.
4. `nixos-rebuild test --flake /root/mike-infra#node-a`, then a fresh
   `ssh node-a` login; check that all five VMs are still active with the
   baseline timestamps and PIDs, and that `ssh choir-runner true` works.
5. `nixos-rebuild switch --flake /root/mike-infra#node-a`.
6. Restart the agents VM at its new size: `systemctl restart microvm@agents`.
   Until this step the agents VM may still grow to 10 GiB, so no builder
   uses it before it.
7. `systemctl start node-a-memory-guard` (or wait one minute).
8. Commit the same patch on the Mike repo branch `infra/node-a-in-line-2026-11-14`
   so the repo matches Node A (local; pushing is the owner's call).

## Proof (observations, no load test)

- `swapon --show`: `/dev/zram0`, about 4.7G.
- `oom_score_adj` of the MainPID of each unit: ws-owner, ws-03, ws-04 -1000;
  agents 500; runner 300; nix-daemon 800 when running.
  `memory.swap.max` of the three Mike units: 0.
- `/var/lib/microvms/agents/booted/bin/microvm-run` carries `size=4096M`;
  inside, `free -m` shows about 3.9 GiB total.
- Mike's VMs untouched: the step-0 timestamps, PIDs and `NRestarts`
  unchanged after step 7, and again a day later.
- The first time a landing on the runner overlaps a heavy suite on the
  agents VM: sample `MemAvailable` and `/proc/pressure/memory` on Node A
  every 10 s for the overlap; expect MemAvailable above 3 GiB throughout, no
  new `oom-kill` line in `journalctl -k`, and each guest's resident memory at
  or under its configured size.

## Rollback

`nixos-rebuild switch --rollback` (or switch the flake in
`/root/mike-infra.before-memory-20261115`). The guard's runtime settings then
stay until reboot; `systemctl revert microvm@ws-owner microvm@ws-03
microvm@ws-04` removes the swap setting without a restart, and writing 0 to
each `oom_score_adj` restores the scores. The agents VM returns to 10 GiB only
on its next restart, which brings back the overcommit unless the runner is
shrunk first; do not restart it at 10 GiB.

## Heresy delta

None against doctrine. It retires the 2026-11-14 belief that the overcommit
was acceptable while Mike is paused: it is unsafe whether or not Mike is
active, because the guests decide when they fill, not the host.
