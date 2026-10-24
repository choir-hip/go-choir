# An older release cannot read a newer tape (2026-10-24)

Status: problem documented; the invariant and its gate are owner-ratified
(2026-10-24, in session); no code yet. Station SR7 of
`docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md`.

## The problem

Restore does not rewrite the tape. It moves a computer's effective head back
and runs the release that matches that earlier point; every event written
after it stays on the tape, because the tape is append-only. So after a
restore, an older binary boots on a tape that holds newer events. If one of
them is a kind or field that binary does not know, the computer may not boot,
or may project wrongly.

Receipt: b@c's five-app candidate selfdev-79dc4bf4 was rejected at 13:2x
(first-pass ledger). It was built from 0cbc52edc, and b@c's tape already held
the D39 projection op kinds (media_progress_recorded, media_recent_recorded,
user_preference_recorded), which that binary cannot read. Approving it would
have left the computer one restore away from unbootable. The restore point was
fine; the binary could not read the tape.

## The invariant (owner, 2026-10-24)

Any release can boot on any tape written by releases inside its support
window, in both directions:

- **Newer reads older.** Mostly holds today (decode roots, migrations).
- **Older reads newer.** Missing. A binary that meets an event kind or field it
  does not know keeps it intact and skips it in projection: it never fails
  boot and never drops the event. Restoring to an older release is then always
  safe; the newer events sit inert until a release that understands them runs.

Two rules follow:

1. A new event kind ships in two releases: first one that keeps and skips it,
   later one that writes it (the same shape as the update rulings' one-week
   support window and floor).
2. Store schema changes are additive only; a destructive change is two-phase
   (stop reading, then remove after the window).

## Mechanical enforcement

A cross-version replay gate in CI: boot the previous release's binary on a tape
the candidate wrote, and require it to boot and project, with the unknown
events skipped and preserved. It would have caught selfdev-79dc4bf4. Passing it
routinely is what makes a change class "proved reversible", the gate the staged
auto-approve path needs (SR8).

**Constraint (owner, 2026-10-24): the gate must not lengthen CI wall clock.**
It runs as its own parallel job, or inside an existing job's slack, and must
finish inside the current critical path. The previous release's binary comes
from a cache (the deployed build, or a pinned artifact), never a second full
build on the critical path. The CI timing baseline is read before the design.

## Open questions (hypotheses until read)

- What an old binary does today on an unknown kind. Hypothesis: the decode
  roots refuse it and boot fails or the projection stops; to be read from the
  decoder and replay code, not assumed.
- Which tapes the gate replays: a fixture tape written by the candidate's own
  tests, or a sample of a staging test computer's tape.
- Fields added to a known kind (a newer field on an old kind) versus whole new
  kinds; both directions need a rule.
- The Dolt projections: which tables an older binary reads that a newer one
  migrated.

## Findings, 23:5x (read-only investigation of origin/main; code read, nothing run)

**Today every unknown thing on the tape stops boot.** Boot replay is
`appender.Reconstruct` (internal/autoputer/run.go ~764); any error aborts boot.

- An unknown event kind: `Event.Validate` rejects kinds outside
  `validEventKinds` (internal/computerevent/event.go ~127), so replay fails.
- An unknown op inside a known `projection_batch_recorded` (the D39 case):
  `ProjectionBatch.Validate` refuses it against the closed `projectionOpKinds`
  map, and `projectOp`'s default case errors (projection_batch.go, store/project.go ~106).
- An unknown envelope field: the decoder drops it into the struct, the digest
  is recomputed from the struct and no longer matches the raw bytes' digest, so
  replay reports a projection mismatch (hypothesis from the code). A newer-only
  envelope field also cannot be written today (`PinEvent` and admission refuse it).
- An unknown field in an op body: dropped from the projection silently; the tape
  keeps it (replay never rewrites tape bytes).
- An unknown object kind inside `object_recorded`: works, it is data. This is
  why app-package kinds are safe and new op and event kinds are not.
- A failed finalize rolls back whole, so a computer whose tape holds an op its
  binary refuses loops at boot on that one event.

**Restore is worse than the problem statement said.** Restore appends a
causal-only `restore_requested`, rebuilds a staged store through the target
(`ReconstructThroughTarget`), swaps it in and restarts into the older release.
At that next boot nothing pins the target: `Reconstruct` runs with no target
head and applies every later event with the older binary. So the later events
are not inert: unknown ones crash boot, and **known ones may re-converge the
computer to head, silently undoing the restore** (hypothesis: no boot-time
restore pin was found; not proved end to end). If it holds, it bears on this
metamission's own finish condition, "restore still rewinds", which earlier
proofs may have checked only before a restart. Probe: on a test computer other
than b@c, restore across a few events, restart it, and read whether the head
and the projection stay at the target.

Supporting staging evidence (one observation): the 2026-10-24 03:5x drill on
computer-2ff1f637 (first-pass ledger) restored to the pre-apply checkpoint at
03:39:07; "the route was gone after the restart" (the release rewound), but
"the operation read awaiting_approval just after the restore and applied after
the restart (unexplained)". That is the predicted split: the restored release
runs, and at its boot the later events re-apply, so the store's projection
returns to head while the code stays at the target. The hypothesis is now
supported by one observation; the probe above decides it.

**Versions that exist.** Event SchemaVersion and ReducerVersion must equal 1;
batches allow (v1, projector 1) or (v2, projector 2); nothing else. No
reads_schema_versions on events. The store has an epoch (storeschema.Version 3)
and a raise-only floor checked at Open and by the updater on Apply and
restorePrior; only the event table writes a floor today. The store has no
migration framework: CREATE IF NOT EXISTS, ADD COLUMN, and DROP of retired
tables at open; extra newer tables and columns are harmless to an older binary.

**CI room.** The critical path is plan, the slowest non-runtime race shard (17
to 21 minutes, one outlier shard), check, deploy (about 20 to 24 minutes). Every
other job has 6 to 19 minutes of slack. A cross-version job needing only `plan`
fits within about 10 minutes without lengthening wall clock. CI never builds the
deploy binary (Node B does); the previous release's SHA is already known to
`plan` (deploy-base-select from /health). Cheapest source of the old reader:
build only a replay harness (computerevent and store) at that SHA on the warm Go
cache, or cache it per green main SHA; not a full autoputer build.

**What the fix must cover:** unknown event kinds (Validate and Reduce), unknown
ops (ProjectionBatch.Validate and projectOp), unknown envelope fields (the raw
digest check), and a boot-time restore pin so a restored computer stays restored.

## Next

1. Read the decoder and replay path for unknown kinds and fields (read-only).
2. Read the CI critical path timing (which jobs bound wall clock).
3. Design the keep-and-skip decoder and the gate; review it like the change it
   designs (tier 2: tape, boot path).
4. Build, prove on staging with a real restore across a kind boundary, then
   the doctrine entry.
