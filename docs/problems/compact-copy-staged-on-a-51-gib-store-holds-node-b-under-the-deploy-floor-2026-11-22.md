# A compact copy staged on the paper's 51 GiB store holds Node B under the deploy floor until a deploy restarts the paper (2026-11-22)

Status: the layout trigger is fixed (see "Fix"); the staged copy was consumed by a deploy and the shortage is over (see "What was done"); the preflight constant remains.

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
the compact copy, and the rollback original kept after a swap) costs
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

## 13:28Z: the copy is staged; refused deploys still cost disk

- Paper console, 12:12:33Z: "store: compact copy: staged 51220185088 bytes at
  sequence 13617852 in 1h24m36s (copy 28m54s), for the next planned restart".
  So the check is finished and the copy waits, as read from the code.
- Node B free: 50 GiB at 12:50Z, 44 GiB at 13:28Z. `/nix` went from 39 to 43
  GiB: each landing's CI prebuilds the deploy's closure on Node B before the
  preflight refuses the deploy (runs 45019936090 and 45022429116). The rest is
  the paper's own growth.
- Decision (lead): no landing that triggers CI's deploy path until the disk is
  resolved; work continues in lanes. Docs commits do not build.

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

## Fix (2026-11-22, option 3 of the options above)

What changed: the layout trigger is deleted from `internal/store/compact.go`
(`compactLayoutMaxAge`, `compactLayoutMinBytes`, the layout branch of
`compactReason`, the `.layout.json` marker and its writes in the rebase and
the header/body split). A copy is due only by free pages (10% or 2 GiB) or
when `store.compact_force` is on, so a store with no free pages is never
copied for the age of its layout. On the paper that copy reclaimed nothing
(0 of 13,383,190 pages free).

Evidence for the layout claim: only the synthetic bench in
`docs/design/paper-compaction-2026-10-31.md`, never measured on the paper.
One day after a snapshot on a layout about seven days old, 2,708 of 13,124
chunks were already held (line 75 of the snapshot record of the same day).
**Unmeasured, the residual:** what the removal does to the daily snapshot's
upload. Chunks new per capture over the week after this deploys would settle it.

Not changed: a copy already staged (the paper's, due by layout on 2026-11-22)
is swapped in at a planned restart if on chain with a short tail, and dropped
otherwise (`RebaseOntoCompactCopy`; a test pins this). A stale
`state.db.layout.json` is left on disk and read by nothing; no boot sweep of
such leftovers exists, so none was added.

What remains: the freelist-triggered copy is still store-sized and still
staged until a planned restart, so a store that does fragment will hold the
volume twice again. The deploy preflight's 14 GiB "guest store compaction
peak" is still a constant, not the store's size. Options 1, 2 and 4 are
untouched; the snapshot's own scratch copy is a separate record.

## What was done, 2026-11-22 afternoon (the owner's word for each act)

- **The paper's three oldest snapshot roots were removed** (2026-11-17, 19 and
  20; the roots of 11-21 and 11-22 were kept), and the 26,124 chunks only they
  held were given an old modification time so the hourly artifact collector
  deleted them through its own logged path. Cost: a restore of the paper to a
  checkpoint before 2026-11-21 12:00Z has no snapshot base. The rows as they
  were, the list of roots and the list of chunks are on Node B under
  `/root/paper-snapshot-roots-removed-20261122/`.
- **Five computers were removed** through vmctl's own remove call and their
  disk folders deleted. Kept: the paper, the owner's own computer and the two
  test accounts. The removed computers' tape segments and snapshot chunks are
  still on the host (goal file decision Q2, default keep).
- No host service was restarted by hand.
- **The staged copy was consumed by a deploy.** At 16:28Z Node B had 70 GiB
  free and the deploy of `22192b13b` passed the preflight; the paper's planned
  restart at 16:47Z rebased onto the copy ("55367938048 -> 51220189184 bytes,
  44520 events to replay"). Free space: 88 GiB at 16:58Z, 168.5 GiB at 18:13Z
  after the guest's trim and the collector's sweep; the paper's data image is
  54.2 GiB. That deploy job was marked incomplete for another reason
  (`deploy-marked-incomplete-while-the-paper-was-refreshed-twice-2026-11-22.md`).

## Where the rest of the disk is (read-only, 16:00Z)

- Postgres, 41 GiB: `control` 29, of which the event receipt table is 20 of
  heap and 8 of indexes with about 12 live (receipt compaction has run; the
  dead space returns only by a rewrite under an exclusive lock: decision Q3,
  default no); `graph` 12.
- A hibernated computer's image keeps blocks its guest freed: one test
  computer uses 8.4 GiB in the guest and holds 19.8 on the host.
- What the paper's 51 GiB store is made of is not known: the by-table reading
  is cut off at about 95 s with 15 GiB attributed. A background reading by
  object kind is reviewed and queued to land (`lane/store-land`).

## Corrections to what the lead said that day

A computer was called a dead candidate and then said to be on an older guest;
it was the primary computer of a test account, hibernated, on the current
release. It was not deleted. The first reading of the disk fall as a second
snapshot capture was also wrong (above).

The mission that removes the store-sized copies:
`docs/definitions/choir-storage-efficiency-2026-11-22.md`.

