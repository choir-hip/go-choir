# Self-development candidates build from the computer's creation commit (2026-10-24)

Status: open. Known shape (F5 and F5a in
[computer-update-architecture-rnd-2026-10-24](../design/computer-update-architecture-rnd-2026-10-24.md),
P13 in [selfdev-update-architecture-2026-10-15](../archive/reports/selfdev-update-architecture-2026-10-15.md)).
New: since D39, applying such a candidate is unsafe, not just stale.
Mutation class of a fix: red (boot path, deployment routing, the source a
release is built from). Nothing is fixed here.

## Evidence

- Two five-app candidates on b@c (computer-6c50d6e5), built after b@c had
  refreshed to the current platform, were both built from `0cbc52edc`:
  - v6, selfdev-79dc4bf4, frozen at 11:13 while the guest ran `dc3f5ee3b`;
  - v7, selfdev-37b1b5a5, frozen at 15:20 while the guest ran `b2cebe931`
    (refreshed at 14:33:08).
- The v7 patch (351,537 B, sha256 a70b862f…) applies cleanly to
  `0cbc52edc` and fails on `b2cebe931` (`frontend/src/lib/PodcastApp.svelte:642`).
  `0cbc52edc..b2cebe931` changes four files under `frontend/src/lib`,
  among them D39's `media-utils.js` progress throttle.
- The 04:27 Slides apply is not in that source either. The v7 patch rewrites
  `SlidesApp.svelte` from the pristine `0cbc52edc` file, so applying it would
  also replace the applied Slides with v7's own (the last apply wins).
- At 05:15 b@c's Freeze was refused with "the running binary was built
  from 824b8410 but the capsule source is pinned to 0cbc52edc; build
  bin/autoputer" (`internal/capsule/release_binary.go:153`). So each Freeze
  ships a bin/autoputer built in the capsule from `0cbc52edc`.

## Cause (verified in source)

- `CHOIR_CAPSULE_SOURCE_ROOT` is `/mnt/persistent/files/Source/platform`
  (`nix/autoputer-vm.nix:1060`), on the persistent data image.
- That checkout is written once, by `sourceSeedStagingRoot`
  (`internal/vmmanager/manager.go:3138`). It runs when the data image is
  created, and checks out the vmctl build commit of that moment. No refresh,
  deploy or apply moves it afterwards.
- `PreflightSourceSnapshot` with an empty candidate ref pins the capsule to
  that checkout's HEAD (`internal/capsule/executor.go`, the last branch).

So every engineering candidate, on every computer, is authored against
the creation commit and ships a guest binary built from it.

## Consequence

- Before D39 this was a silent downgrade. Applying a candidate ran the guest
  on the creation-commit binary until the next platform refresh, and every
  guest fix in between stopped reaching that computer.
- Since D39 it is unsafe. The tape of a computer on the current platform
  holds projection ops that a binary from before D39 refuses (the
  forward-fix-only note in the D39 ledger entry). Applying such a candidate
  leaves a computer whose replay, restore or crash-window recovery refuses
  its own tape. Its media writes would also go back to the direct-write
  tables, and checkpoints would turn ineligible again.
- Neither five-app candidate is approvable on b@c, although both pass
  product review. v7 is the better one: the build is clean with no new
  warnings, and it keeps the contracts v6 broke. Both were rejected, and
  both patches are kept (`_runs/apprewrite/v6/patch.diff`,
  `_runs/apprewrite/v7/patch.diff`).
- Any self-development apply on any computer older than D39 has the same
  problem.

## Fix direction (owner design, not built)

The source seed must follow the booted base (P13). That is the first slice of
U1, which makes applies compound. When the platform moves, the checkout has
to move to the booted commit with the computer's applied patches on top. A
plain checkout of the new commit would drop the applied layer from the
source, so the last apply would win (F5a). Until it lands, a self-development
candidate is approvable only on a computer created at the current platform
commit, with no apply since.

## Lesson

I submitted v7 believing it would build on current source. The seed's
behaviour was already written down (F5). One read of the capsule's
source preflight before submitting would have saved a 44-minute round.
Before casting engineering on a computer, check which commit
`files/Source/platform` is at.
