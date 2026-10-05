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

## Finding 2 — the "dead" fleet computer is the owner computer (must preserve)

`candidate-fleet-e15cb89f25d963c220319b7b` shows `state=active` in ownership but
its console logs are 5 weeks stale and its `computer_url` is dead. It is **not**
a zombie to reap: its `computer_id` is `computer-03335285…`, the owner guest
computer from S2's `start.deploy_identity`, and its `data.img` (31G) is the
owner computer's persistent store. The `state=active` is a stale mark — the
deploy's app-layer offer-mint attempt bumped `last_active_at` without real VM
activity. `reconcileLookupReadiness` only degrades on *lookup*; an ops mint
attempt does not degrade it. This is a stale-active reconciliation gap, not a
reaper gap — and it must NOT be reclaimed (must_preserve).

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
