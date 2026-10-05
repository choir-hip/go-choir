# S0-1: storage lifecycle — the dead-dir reaper can't reap, and platform-artifacts has no GC

## Symptom

Node B `/var/lib/go-choir` sits at 80% (`df -h`: 373G/476G used, 97G free).
The two growth surfaces behave differently:

- `platform-artifacts/` = **78G** with no GC at all: `sha256/file-cas-chunks`
  48G, `sha256/projection-base` 15G, plus `computer-event-payload` 4.3G,
  `pin-receipts` 3.9G, `computer-event` 2.0G, `og` 2.5G.
- `vm-state/` = 35G across 13 owned dirs; **zero orphans** today.

## Finding 1 — orphan reaper exists but was unprunable (FIXED)

`vmctl` retention-prune is wired, env-gated, and **already active on Node B**
(`VMCTL_RETENTION_PRUNE_MODE=active` in `nix/node-b.nix`, confirmed in the live
`/proc/<pid>/environ`). It runs inside `runReclaimSweep` every idle sweep.

But `PruneRetention` calls `authorizeLifecycleRoute(ctx, guard, userID,
desktopID)` for *every* candidate. Orphan state-dir candidates have empty
`user_id`/`desktop_id`, so the route guard always returns false and orphans are
silently skipped — unprunable forever even in `active` mode. Owned ephemeral
candidates were unaffected (they have a real route to authorize).

Also, the orphan classifier only accepted `vm-*` dir names; `candidate-fleet-*`
and `candidate-control-*` dirs (the probe-era fleet) were never classified as
orphans even if their ownership was removed.

## Finding 2 — the "dead" fleet computer is the live owner computer (must preserve)

`candidate-fleet-e15cb89f25d963c220319b7b` shows `state=active`,
`last_active=2026-10-04T12:53`, but its console logs were 5 weeks stale and
its `computer_url` initially looked dead. It is **not** a zombie to reap: its
`computer_id` is `computer-03335285…`, the owner guest computer from S2's
`start.deploy_identity`, and its `data.img` (31G) is the owner computer's
persistent store. Verified live 2026-10-05: `GET /health` on
`10.200.206.2:8085` returns 200 and its Firecracker process is running.

The stale *appearance* had two substrate causes: the deploy's active-VM
refresh leg boots the canary and `transitionVM` marks it `active` (so
`last_active` refreshes on a computer that was idle, not dead), and there
was no per-VM serial capture to show it was alive — the `console-b14-*.log`
tail was 5 weeks old because no serial sink existed (S0-2 fix). The
stale-active *reconciliation* residual (an ops mint/refresh bumps
`last_active` without a health gate) is a separate gap, not a reaper target
— and this computer must_preserve regardless.

## Finding 3 — platform-artifacts GC does not exist (the real fire)

`internal/recovery/scrub.go` is integrity-only (verifies digests, deletes
nothing). `internal/platform/file_cas_gc.go` has `GCFileChunks` — a correct
reachability GC over `computer_file_roots` — but has **zero non-test callers**.

The live-set is distributed across DB refs (no single live-set table):
- `file-cas-chunks`/`file-cas-roots`: `computer_file_roots` (latest + recent).
- `projection-base`: `computer_replay_watermarks.base_ref` + `.descriptor.json`
  sidecar (pair); superseded bases have no ref → deletable past grace.
- `platform-update`: route-slot `current_code_ref`/`artifact_program_ref` URIs,
  rollback-target receipts, and in-flight/accepted-update event payloads.
- `computer-event`/`computer-event-payload`/`pin-receipts`: the retained
  append-receipt chain — live by chain, not by reachability; needs a chain
  retention policy, not a refcount sweep.
- `og`: `og_objects.body_ref` exact reachability.

## Mutation class

Orange → red-adjacent (deletion of host state). The orphan-auth/prefix fix is
surgical; the platform-artifacts GC must be reachability-exact + dry-run-first
+ grace-gated (in-flight pin/append/apply) so it never deletes a live or
not-yet-committed artifact.
