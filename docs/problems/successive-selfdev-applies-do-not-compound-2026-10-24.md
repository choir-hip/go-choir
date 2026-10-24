# Successive self-development applies on one computer do not compound (2026-10-24)

Date: 2026-10-24, 05:2x UTC. Found by: the FP0 app-rewrite conductor on the
test computer computer-6c50d6e5 (b@c.com), staging (guest build 0cbc52edc,
then 824b84106 after the 05:02 planned refresh). Run log:
`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md` (outside the repo). Class of
this record: green. Class of a fix: red (capsule source seeding, the apply
path, the restore set). Status: problem recorded, not fixed.

## What happened

1. Slides, operation `selfdev-41ea6ea6668ad096b24b28fdc626ed75`: a
   frontend-only rewrite of `frontend/src/lib/SlidesApp.svelte`. It was
   approved at 04:27:04 and applied at 04:27:21. The served SPA carries the
   new chunk `SlidesApp-3u8sOXpS.js` and `SlidesApp-CRuA8iTq.css`, and it
   still did after the 05:02 platform-update restart. **[V]**
2. Files, operation `selfdev-18e1dc56024443fb56b34687bd59f793`: the next
   request, a rewrite of `FileBrowser.svelte`. Its frozen patch
   (`choir self-dev patch`) has `source_patch_base_commit
   0cbc52edce852c09cc68f8e3004d98b6e8fd21a1` and touches only
   `FileBrowser.svelte`. The frozen release tree (Freeze's change list, in
   the engineering run events) carries
   `frontend/assets/SlidesApp-lEJDYyQw.css`. I rebuilt the pristine
   0cbc52edc frontend locally and got `SlidesApp-lEJDYyQw.css`: the same
   file. That is the **old** Slides. The capsule built the release from the
   base commit without the applied Slides change. **[V]**
3. Approving Files would therefore have reverted Slides. It was rejected at
   05:26:15 for that reason.

## Why (code read)

The engineering capsule's `/workspace/platform` is seeded by
`(*capsule.Executor).PreflightSourceSnapshot(ctx, "")`
(`internal/capsule/executor.go`), which takes "the current clean committed
source tree" (`immutableGitCommitIdentity(e.sourceDir)`). The Slides apply
did not move that tree, so every later operation starts from the platform
base. A frontend-only candidate rebuilds the whole frontend dist from that
source, and the apply replaces the served frontend with it.

A second effect: after the 05:02 platform refresh, the running binary is
824b84106 but the capsule source stays pinned to 0cbc52edc. So even the
frontend-only Files change had to build `bin/autoputer` in the capsule
(Freeze: "the running binary was built from 824b8410… but the capsule source
is pinned to 0cbc52edc…; build bin/autoputer"), about 7 minutes. Once that
layer applies, the computer runs a binary built from the old base. That is
the known [guest-runtime-fix-never-reaches-layered-computers](guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md)
shape. The guest code is identical between those two commits (only
`internal/vmctl` changed), so nothing regresses tonight.

## Consequence

Self-development on one computer is single-slot: the last applied operation
wins. A computer cannot accumulate changes (Slides, then Files, then
Calendar), which is the product promise of a persistent computer that
develops itself. The drills never showed this, because each drill mints a
fresh computer and applies once.

## Related finding (same run)

After an approval, the guest consumes accept_once and writes a propose_only
receipt whose `old_mode` is `accept_once`, with `consumed_operation_id` set.
`verifyDecisionModeReceipt` (`internal/agentcore/api_self_development.go`,
the reject branch) refuses a reject under that receipt: `409 current signed
mode does not authorize this decision`. The owner has to re-arm propose_only
(a new generation) before they can reject the next candidate. **[V: 05:25:47
refusal; 05:26:14 re-arm, gen 4; 05:26:15 reject accepted]**

## What a fix has to do (not decided; failure modes first)

- Seed a capsule from the computer's **effective** source: the base plus
  every applied operation's source patch, in order. Either commit each
  applied patch into the guest's source tree at apply time, or derive the
  subject from the effective release's source. Then a later candidate's
  patch is relative to the effective source, and its release keeps the
  earlier changes.
- Failure modes: a restore to a pre-apply checkpoint must rewind the source
  too, or the next capsule builds on code that is no longer served. A
  platform refresh that moves the base must rebase the applied patches or
  refuse visibly. Never silently drop them. Freeze's binary-reuse check
  must compare against the effective source, not the base commit.
- Until then, an owner on a layered computer can only land several changes
  as one operation.
