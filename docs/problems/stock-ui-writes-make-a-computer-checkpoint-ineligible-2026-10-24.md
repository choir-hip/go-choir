# Ordinary use of the stock apps makes a computer checkpoint-ineligible, so it cannot self-develop

Status: open. Recorded 2026-10-24 by FP0. Mutation class of this record:
green. Class of a fix: red (checkpoint and replay projection, new tape event
kinds).

## What happened

At 11:1x UTC the app-rewrite conductor froze a five-app candidate on b@c's
computer (computer-6c50d6e566143ae32cdb660faa60e24d, op selfdev-79dc4bf4).
The required pre-apply checkpoint (`choir computer checkpoint`) was refused
with 409 "replay is ineligible: behavior-bearing direct-write tables are
non-empty without reducers". The CLI does not name the table.

`GET /api/computers/<id>/self-development/replay-completeness` (b@c's key,
11:2x) names it: `eligibility.unsupported_tables = ["media_recents"]`.

## Why, from the code

- The replay manifest (`internal/agentcore/replay_eligibility.go`) classes
  `media_recents`, `media_progress` and `user_preferences` as
  `ReplayEmptyUntilSupported`: allowed only while empty, because no reducer
  rebuilds them from the tape.
- The stock frontend writes all three in ordinary use:
  - `frontend/src/lib/media-utils.js` POSTs `/api/media/recents` when a media
    source is opened and `/api/media/progress` as playback or reading moves
    (handlers in `internal/mediastate/handler.go`, store in
    `internal/store/media.go`);
  - `frontend/src/lib/preferences.js` PUTs `/api/preferences/theme` when the
    owner changes the theme (`user_preferences`).
- So one opened PDF, one played podcast or one theme change makes the
  computer ineligible for every checkpoint, restore and effects transition
  that needs a replay claim, until the tables are emptied. The pre-apply
  checkpoint is required before an owner approval, so self-development on
  that computer is blocked.
- The self-development drills passed because their computers are minted
  fresh and never touch these routes.

## Who it reaches

Every computer whose owner has used Settings' theme or any media surface.
That is likely most real computers, the owner's own included (not probed:
the replay-completeness report is not run on the paper or the owner's
computers). b@c is the first observed case.

## Options (not taken here)

1. Make the three tables event projections: the handlers append computer
   events and a reducer projects them, as the desktop_* tables already do
   (`ReplayEventProjection`). The replay claim then covers them.
2. Reclass them as non-behavior-bearing: excluded from the equivalence
   check, left as they are on restore. Smaller, but it weakens the
   reversibility claim (a restore would not rewind recents, progress or the
   theme) and needs a new manifest class.
3. Name the table in the CLI's 409 (cheap, any time): the refusal should say
   which tables block it.

FP0's lean: 1 for all three tables (they are small, per-owner, last-write-wins
records, the same shape as window placements), with 3 alongside.

## The b@c candidate

The candidate stays at awaiting_approval. Without a fresh checkpoint its only
rollback point is checkpoint 0a310d91 (04:26), from before the Slides apply,
and the rollback route requires a prior checkpoint. The conservative choice
is to hold it until the checkpoint can be taken.

## Evidence

- /tmp/fp0-bc-replay.json (local, FP0's Mac): the replay-completeness report.
- The conductor's log /Users/wiz/Dev/lanes/_runs/apprewrite/log.md, v6 section.

## Design of the fix (2026-10-24, lane `lane/fp0-media-replay`)

Option 1, with option 3 alongside. Mutation class red: new projection op
kinds on the tape, the replay manifest, and the replay claim.

### The live write

The three tables become event projections exactly as `desktop_*` and
`texture_document_aliases` did (ebce5455f); no new mechanism.

- Three projection op kinds in `computerevent` (V2-only, inside the existing
  `projection_batch_recorded` event; no event-kind, schema-version or
  projector-version bump, the same as `texture_document_alias_recorded`):
  `media_progress_recorded`, `media_recent_recorded`,
  `user_preference_recorded`. Each body is a row snapshot whose fields mirror
  the SQL columns verbatim (strings for the DATETIME and JSON columns, as
  `RunMemoryEntryProjection` does), so the reducer never derives a value.
- `store.UpsertMediaProgress`, `UpsertMediaRecent` and `SaveUserPreference`,
  when the projection tape is bound, append the op instead of writing SQL;
  `store/project.go` projects it with the same `INSERT ... ON DUPLICATE KEY
  UPDATE` the direct write used. Unbound stores keep direct SQL (the test
  seam, as for desktops). Last write wins, as before: the reducer is an
  upsert keyed on the table's primary key, and the handlers' responses are
  unchanged.
- The manifest reclasses the three tables `ReplayEventProjection` (not the
  optional class: the tables always exist in the schema, so the extractor
  always observes them). The manifest version stays 4: no class is added,
  as with the alias reclass.

### Rows written before the change

Chosen: **a seed folded into the first tape-bound write of the family in
each process.** The first write to any of the three tables after the tape
is bound reads every row of the three tables and appends one batch holding
an op per existing row followed by the write's own op. A per-store flag,
under a family mutex that every bound write of the three tables holds
across its snapshot and append, makes later writes in the process plain
single-op batches. The same snapshot is added to the owner-scoped residue
import (`choir computer import-residue-snapshot`), which is the explicit
route for a computer whose rows predate the change and that nobody writes.

Why this and not the alternatives:

- **The full residue import alone** is the existing route for legacy rows,
  but it re-records every owner OG object, run memory entry and desktop in
  one event. On a mature computer (b@c, the paper) that is a very large
  batch to seed a few small rows. It stays available, now covering the
  family, but is not the default path.
- **A boot-time seed** puts tape appends on the boot path, runs on every
  boot (crash boots included), and is a restart-path behavior the contract
  asks us not to add. The write-path seed does no boot work.
- **A one-time durable marker** needs a marker table (a new manifest entry,
  schema) or a reserved preference key. The per-process seed needs neither,
  and it also covers the revert case: if an older binary writes direct SQL
  after a rollback, the first write under the new binary records those rows
  again.

Replay stays exact: the seed ops carry the rows' stored values verbatim and
come before the write's op in the same batch, so the write wins for its own
key, and the reducer that applies the batch live is the one replay uses.
The family mutex keeps a concurrent write from landing between the snapshot
and the append (which would let the seed reapply an older value). The cost
is one snapshot of three small tables per process lifetime, only on a
computer that writes media state.

Crash safety: the seed and the write are one event, appended by CAS and
projected inside Finalize's transaction. A crash before the CAS leaves no
trace and the next process seeds again. A crash after the CAS and before
Finalize is the ordinary projection-repair case for any batch. There is no
half-written seed. Re-seeding is idempotent: the ops are upserts of the
current values.

b@c: the candidate's checkpoint becomes possible after deploy and either one
theme or media write by the owner, or one `choir computer
import-residue-snapshot` (the larger event described above).

### Tape volume

Every progress PUT already appends one tape event today (the
`media_progress_updated` product event is acknowledged and goes through the
OG interceptor), and the players PUT on every `timeupdate` (about four a
second while playing). The row op doubles that. Discovered, recorded here,
and offset in the same change: `saveMediaPosition` in
`frontend/src/lib/media-utils.js` sends at most one PUT per source every
few seconds, with a trailing send of the latest position, so a playing
source appends a small fraction of today's events.

### The CLI's 409

`ReplayEligibility.Error()` names the blocking tables (unsupported, retired,
unknown, missing, drifted) in its message, so the guest's 409 and the CLI
both say which tables block a checkpoint, with no CLI change.

### Rollback

Revert the commits. Batches already on a tape with the new op kinds are
refused by an older binary's projector as unknown kinds, so a computer that
wrote media state under the new binary cannot be replayed by the old one;
every op-kind addition carries this exposure (`texture_document_alias_recorded`
did). Once such batches exist, forward-fix rather than roll the binary back.

### Tests (written first, failing before the fix)

In `internal/agentcore` (`media_state_replay_test.go`), on a computer
shaped like production (bound tape, the replay probe):

- a computer that writes a media recent, a media progress and a theme
  through the stock handlers replays eligible and equivalent;
- rows written by direct SQL before the change (pre-change computer) make
  it ineligible, and after one handler write (the seed path) it is eligible
  and equivalent and the old rows are intact; the residue import does the
  same for a computer nobody writes;
- overwrites keep last-write-wins: the GET responses match an unbound
  (direct-SQL) store's for the same sequence, and the replay is eligible;
- the manifest classes the three tables `event_projection`, and the
  ineligible error names the blocking tables.

### Residual: b@c's candidate runs an older binary

The held candidate's guest binary is built from 0cbc52edc, 81 commits behind
the platform (ledger 11:2x). That binary predates the three op kinds. Once
b@c's tape holds a batch with them (its first media or theme write after
this deploys, or a residue import), that binary's projector refuses those
batches as unknown kinds. So after the candidate is applied:

- its manifest still classes the three tables `empty_until_supported`, so
  its checkpoints are refused again, as today;
- any replay it runs over the new batches (a replay-completeness probe, and
  a restore if the restore path replays rather than loading a base at the
  checkpoint head) fails on the unknown kind rather than reporting
  ineligible. Not checked here: whether restore to the pre-apply
  checkpoint, run by the candidate's binary, replays those batches. If it
  does, the rollback route of that apply is broken.

This is the applies-do-not-compound substrate
(docs/problems/engineering-work-product-durability-cluster-2026-10-24.md),
not a defect of this fix, but it decides how b@c's candidate can be
approved. Before approving it after this lands, check the restore path of
an old binary over a tape with the new op kinds, or rebuild the candidate on
current source.
