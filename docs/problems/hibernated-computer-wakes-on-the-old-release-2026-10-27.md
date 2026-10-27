# A computer hibernated across a deploy wakes on the old release and is not updated (2026-10-27)

Status: open; documented before any fix. Mutation class of a fix: red (VM
lifecycle, vmctl). Related: CLAUDE.md "Later: real hibernate and wake (a
computer with a pending update boots fresh instead of resuming memory)";
docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md
(§3 says a guest-artifact deploy voids snapshots; here one did not).

## Evidence (b@c, computer-6c50d6e5…, VM candidate-fleet-bf04bbc7…; vmctl journal)

- 07:08 refreshed onto train 33 (036a216eb), epoch 50.
- 11:28 deploy refresh retry: "runs the current release f744j1inx4i9…;
  forgotten".
- 11:49:57 hibernated by snapshot, reason idle, epoch 50.
- 12:34 train 34 (1ebe74f85) deployed; it changed guest code
  (internal/sourceproj). The deploy refreshed the paper
  (vm-1669020a…, deploy-image-refresh, 12:33) and logged nothing for b@c.
- 12:35:51 a request woke b@c: "resumed VM … from snapshot (epoch=50)",
  so it ran train 33's guest. Citation attempt 5 was refused at 12:36:56
  for `stack_base_moved`, the very fault train 34 fixes.
- No deferred refresh followed while b@c sat idle (12:37 to 12:57).
- 12:58 FP0 refreshed it by hand (`/internal/vmctl/refresh`, caller
  fp0-b-c-refresh-train34): booted fresh, epoch 51, deploy-image-refresh.

## What this means

A deploy reaches running computers (now, or at their next idle moment) but
not hibernated ones, and a wake resumes the snapshot of the release the
computer slept on. Nothing after the wake notices it is behind. So a
computer that sleeps through a deploy keeps the old guest until something
else restarts it, and a fix shipped for it does not reach it. Cost here:
one owner round refused for a fixed fault.

## Open questions (read-only next probes)

1. Why the snapshot was not voided: did train 34's guest image digest
   change (CI impact class, the guest release id after 12:34)? If it did,
   why did the wake not fall back to a cold boot on a launch-config
   mismatch?
2. Whether the deferred-refresh sweep enrolls a computer that was
   hibernated at deploy time, and why it did not refresh b@c once it was
   awake and idle.

## Correction and cause (13:0x, read-only investigation, Sonnet)

"Nothing after the wake notices it is behind" was wrong. The designed
"resume, then update" path ran:

- Train 34 did produce a new guest release: the deploy jailed
  `guest-releases/p1k2a5lqgb0j3wk7brb1bg8a2w9l5nfs/` at 12:32:38; b@c's
  wake at 12:35:50 jailed train 33's `f744j1inx4i9…`; after FP0's 12:58
  refresh the sweep logged "runs the current release p1k2a5lq…;
  forgotten".
- Release-bound snapshots survive a release change by design
  (`imageMovedSinceLaunch`, `guest_launch_record.go:143`; `releaseRefusal`,
  `guest_release.go:265`, refuses only on retention off, missing or unknown
  release, floor digest, security epoch, ABI, support window). The resume
  marks the computer behind (`snapshot_lifecycle.go:785-788`) and vmctl
  enrolls it `woke_behind` (`enrollResumed`, `deploy_refresh_retry.go:113`).
- The deploy itself skips non-active computers (`selectDeployRefresh`,
  `deploy_refresh.go:280`), as `TestBehindEntryOfAHibernatedComputerIsForgotten`
  pins.
- **The hold:** the sweep's idle gate (`deploy_refresh_retry.go:~437-442`)
  skips silently while `now - LastActiveAt < 2m`, and `LastActiveAt` is
  touched by key traffic every 60 s (`ownership.go:254`, `:285-290`). From
  12:36 to 12:58 the sweep logged nothing for b@c: the only silent branch.
  What traffic kept it fresh is not confirmed (inference).

So the fault is a gate that measures "last touched", not "user absent",
with no log line and no deadline. Fix candidates:

- **A (small, tier 1):** bound the gate: once an entry has been deferred
  past a cap (10 min), stop applying it, and log the first deferral. The
  refresh keeps its planned-restart marker and busy checks. Pins:
  `TestBehindSweepWaitsForTheUserIdleGate`, `TestNonBehindEntryIgnoresTheGate`.
- **B (larger, tier 2):** refuse a behind resume and cold-boot instead
  (`SnapshotFallbackError` from `ResumeVMFromSnapshot`); trades the memory
  resume for a cold boot after every deploy. Pins `TestReleaseBoundResumeEnrolls`.

Recommendation: A, as the standing design (resume, then update when idle) is
right and only its gate is unbounded.

## Revision (13:0x): the likely holder was FP0's own watcher; fix A withdrawn

From 12:36:30 to 12:56 FP0's watcher polled b@c's API every 30 s
(`self-dev list`, `lifecycle events`, b@c's key). That is the traffic that
keeps `LastActiveAt` fresh, so the gate most likely saw a present user and
held, as designed. Fix A (cap the gate) is withdrawn: it would restart a
computer in front of its user, a failure mode `deploy_refresh_behind_test.go`
guards. What remains:

1. The hold is silent: no log line says an update waits on the user-idle
   gate, so the wait looked like a lost update. Small fix: log the first
   hold per entry (orange, vmctl log only).
2. Open design question: should read-only API polling count as a user
   present? An open tab polling has the same effect. Not decided here.
3. FP0 practice: after a deploy, refresh a test computer by hand before a
   proof round, or poll it less often than the gate's 2 minutes.
