# A compact copy staged on the paper's 51 GiB store holds Node B under the deploy floor until a deploy restarts the paper (2026-11-22)

Status: open. Nothing on Node B was changed or restarted. No fix is built.

## Problem

The paper's store is rewritten compactly once it has gone seven days without
it (`internal/store/compact.go`, `compactLayoutMaxAge`). The copy is written
beside the store while the paper runs and is **staged until the next planned
restart**, which swaps it in. Until then the paper's data volume holds the
store twice. The paper's store is 51 GiB, so Node B has about 50 GiB free
while a copy is staged, and the deploy's disk preflight refuses a deploy under
60 GiB (`scripts/node-b-deploy-disk-preflight`). A planned restart of the
paper happens only in a deploy. So the staged copy blocks the one thing that
would consume it.

The preflight's own arithmetic allows 14 GiB for "guest store compaction
peak". That was the store's size when the floor was set (2026-10-22). The
store is now 51 GiB and doubles in about eleven days
(`the-papers-store-doubles-in-eleven-days-2026-11-13.md`).

## Evidence

- Paper console, 2026-11-22: boot at 10:17:34Z (the deploy of `c27a50d98`);
  "store: compact copy: due by layout, 0 of 13383190 pages free, live
  ~54817546240 bytes, copying" at 10:47:57Z; "wal bound: checkpoint after
  compact copy" at 11:16:47Z. **0 pages free: this copy reclaims nothing.** It
  is due only because the file's layout is seven days old.
- Node B free space: 100 GiB at 10:02Z, 95 GiB at 10:18Z, 52 GiB at 11:59Z,
  49 GiB at 12:08Z. The paper's `data.img` holds 104 GiB of a 128 GiB image.
- `compact.go`, header comment and `RebaseOntoCompactCopy`: the swap happens
  "at the next planned restart, before the store opens"; "a crash boot never
  swaps: the copy waits for a planned one". A staged copy is replaced by a
  fresher one when its tail passes half the replay bound; it is not dropped
  while the paper runs.
- `node-b-deploy-disk-preflight`: the floor is the larger of
  `DEPLOY_MIN_FREE_KIB` and nix's live min-free plus 36 GiB, so it cannot be
  set under 60 GiB by hand. That guard is deliberate (2026-09-17 deploy loop).
- This was first read as a second snapshot capture. It is not: the snapshot
  producer's next capture is due near 04:00Z on 2026-11-23
  (`paper-snapshot-aborts-at-its-time-budget-and-its-scratch-copy-blocks-deploys-2026-11-22.md`).

## Belief

Every store-sized operation on the paper (the daily snapshot's scratch copy,
the weekly compact copy, and the rollback original kept after a swap) costs
Node B about 50 GiB, and Node B has about 100 GiB free at rest. One at a time
fits under the image's 128 GiB; none of them fits under the deploy floor. The
disk is the binding constraint on landing anything, and it tightens as the
store grows. The compact copy is the worst of the three because it does not
end by itself.

## 12:43Z: hibernated computers are refused on wake

The same shortage now refuses wakes, so this is an outage for any hibernated
computer whose data image may still grow more than the host has above its
floor, not only a blocked deploy.

- Node B journal, 12:43:50Z to 12:44:03Z (four attempts): "vmctl: host
  admission refused computer=computer-59c902c5… class=primary: its data image
  may still grow 19784 MiB; the host state disk has 50461 MiB, 17693 MiB above
  its floor", and the proxy's "bearer wake failed". The phone app's proof run
  (trainp7) saw it as "HTTP 503: computer_waking_failed" on Textures, Files
  and imported pages. This is the drill account's computer.
- The refusal is correct: admitting it could take the host under its floor.
- The owner was notified at 12:49Z. Nothing was changed on Node B.

## What it blocks

The deploy of F0 (`34bc283ee`) and every deploy after it, until the paper
restarts as planned or the staged copy is removed.

## Options (none taken)

1. The owner frees space on Node B (not verified as reclaimable:
   `vm-state/candidate-fleet-bf04bbc7…`, 19.7 GiB, last written 2026-11-10).
   12 GiB would clear the floor today.
2. A reviewed change to the preflight: when a deploy will restart the paper
   and a compact copy is staged, the compaction term is already spent, so the
   floor for that deploy is min-free + deploy + regrowth + margin (46 GiB).
   The host cannot see the guest's staged copy today; the guest would have to
   report it. A gate change, with its trailers.
3. A reviewed change to compaction: do not stage a store-sized copy for layout
   alone when no pages are free, or stage it only when a planned restart is
   announced. Red (the store's file).
4. The structural one: the store should not need store-sized copies. What is
   in the 51 GiB is not measured by table.

## Where Node B's disk is (read-only `du`, 2026-11-22 12:10Z)

476 GiB disk, 417 GiB used. `/nix` 39. `/var/lib/go-choir` 390, of which:

| What | GiB |
| --- | --- |
| the paper's data image (store 51, staged compact copy 51) | 104 |
| the paper's tape segments (`platform-artifacts/tape-seg/computer-1355b…`) | 89 |
| snapshot chunks (`platform-artifacts/sha256/file-cas-chunks`) | 43 |
| capture bodies (`platform-artifacts/capture-bodies`) | 41 |
| every other computer's tape segments | 21 |
| platform updates, roots, guest releases, tools, other VMs | about 40 |

One computer, the paper, holds about 330 of the 390. Its tape is larger than
its store, and each daily snapshot adds about 6.6 GB of new chunks (79% of
chunks were new after one day). Whether old snapshot chunks and tape segments
under a snapshot are ever retired was not checked here.

## Remaining error

Not confirmed: that the staged copy's check has finished (the check has its
own four-hour budget), and how much Node B frees after the swap and the
guest's trim. Not measured: which tables hold the 51 GiB.
