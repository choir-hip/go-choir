# An apply's checkpoint replays the whole tape into the guest's /tmp, fills it, and kills the runtime (2026-10-26)

Status: documented; no fix chosen. Mutation class of a fix: red (self-development
apply, checkpoint/witness, boot path). Found by FP0 reading vmctl's log and the
guest console after b@c stopped answering.

## What happened (b@c, computer-6c50d6e5…, VM candidate-fleet-bf04bbc7…)

- 11:18:03 UTC: FP0 approved Reader round 7 (selfdev-83fbdfa9). The operation went
  to `materializing` at 11:18:10.
- 11:19:05: the runtime (pid 2866, up since the 10:51:46 boot) opened a second store
  under `/tmp/choir-replay-completeness-*` and began replaying the computer event
  tape into it: page fetches after=36377 … after=44569 (11:20:07 to 11:22:46, 2-3.4 s
  per 1024-event page).
- About 11:26:34 (console uptime 2088 s): Dolt `errors.Fatalf`, "fatal error: write
  /tmp/choir-replay-completeness-786065405/runtime.texture/texture/.dolt/noms/…: no
  space left on device". A Dolt Fatalf is not recoverable: the runtime process died.
  Stack: `selfdevReconcileDrain` → `reconcileSelfDevelopmentMaterialization` →
  `materializeSelfDevelopmentOperation` (self_development_materializer.go:481) →
  `recordMaterializationApplied` (:683) → `bindCheckpointAtHead`
  (checkpoint_restore_bindings.go:54) → `openReplayProbe` (replay_completeness.go:318)
  → `ReconstructThroughTarget` → `FinalizeReplayBatch` (store/computer_events.go:88,242).
- The temp root is removed only by the probe's own close (replay_completeness.go:245),
  which never ran, so /tmp stayed full: at 11:30 and later the guest's updater could
  not write `/tmp/choir-overlay-probe-*` either ("no space left on device").
- vmctl: health failed from 11:26:37; route kept active during grace; the 11:30:25
  deploy refresh to train 10 was deferred ("guest did not answer the planned-restart"),
  then skipped (state degraded); 11:35:28 "wedged VM … refused connections for 5m0s;
  stopped, it boots again on resume". b@c reads `stopped` (epoch 39).

## Why now

The same path ran for the window-manager applies at 09:01 and 09:28 and passed. The
probe replays the entire tape on every apply, so its scratch grows with the tape;
b@c's tape crossed what its /tmp holds (about 45k+ events). Every later apply on b@c,
and on any computer whose tape is that large, fails the same way.

## Scope beyond b@c (to check before any fix)

`bindCheckpointAtHead` has three callers: the self-development apply
(self_development_materializer.go:683), rematerialize (rematerialize.go:484) and the
platform update path (platform_update.go:388). If the platform update path runs on a
computer with a long tape (the paper's tape is far larger than b@c's), it would hit
the same limit. Not yet traced whether platform_update runs on the paper today.

## Facts the fix must respect

- A full replay per apply is O(tape) work and scratch; the cost grows without bound.
- The probe's scratch sits in the guest's /tmp and is not cleaned when the process
  dies; a dead probe leaves the computer unable to write /tmp at all.
- Dolt's Fatalf cannot be recovered in process; the replay must not be able to
  exhaust the volume the live runtime needs.
- Restart rules (CLAUDE.md): the crash boot closes the open operation; nothing
  resumes it.

## Candidate directions (not chosen)

1. Bound the probe: scratch on the persistent volume with a size check before start,
   refuse (named error, apply still recorded, checkpoint deferred) rather than crash.
2. Incremental witness: replay only from the newest pinned replay base
   (c374be0f9 pins replay bases) instead of from genesis.
3. Clean the scratch at boot (stale choir-replay-completeness-* roots).
