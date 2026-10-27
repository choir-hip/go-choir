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
