# d@e rebuilt from its tape differs from live on the event index only

Recorded 2026-11-14T21:59Z. Not repaired. Cause not known.

## Problem

Replay-completeness on d@e answers `not_equivalent`, so a checkpoint of d@e
is not eligible. Two readings ten minutes apart differ on exactly the same two
keys: `sqlite:texture:table:computer_event_index` and
`sqlite:texture:content_root` (which includes it). Every other table digest
is equal at the pinned head, the object graph is equal, and run memory is
equal.

A second defect sits in the report itself: when the result is not equivalent,
the object-graph diagnostic lists live objects read at "now", not in the
probe's snapshot, so on a busy computer it shows every write made during the
probe as `live_only` or `changed`.

## Evidence

- First reading, 21:46:29Z to 21:51:14Z, d@e on a layered release over
  c1ea7a418: object graph 122,293 live against 122,265 replayed, 28 live-only
  and 9 changed. All 37 samples carry `live_updated_at` inside the probe's
  own window (21:46:29Z to 21:47:28Z, one at 21:50:11Z).
- Second reading, 21:54:02Z to 21:58:22Z, head 109,002 on both sides, 216
  tail events: object graph 122,293 against 122,293, nothing live-only; the
  only changed objects are two `choir.platform_poll` rows updated during the
  probe. The 28 of the first reading all agree now.
- Both readings: `og_objects`, `og_edges` and `choir_events` digests equal;
  run memory 75,985 and 76,000 entries with no difference; the event index
  and the content root differ.
- Code: `internal/agentcore/replay_completeness.go` pins the live side in one
  read snapshot under `HoldAppends`; the diagnostic calls
  `ListObjectGraphFingerprints` on the live store outside it. The index is
  hashed without its timestamp columns and trimmed as a function of the head
  sequence (`internal/store/event_index_trim.go`).
- The readings are kept outside the repo; the report carries digests only.

## Belief

Not caused by stored-result step 2: a slim lifecycle command or its revision
replaying differently would change `og_objects`, and that digest is equal;
index rows come from tape events, not object bodies. Whether the index
difference is older than today is unknown: d@e has no earlier successful
reading (the check was refused for scratch space until the boot prune freed
10.6 GB at 21:31Z), and the one recorded `equivalent` on staging predates the
index trim of 2026-10-31. Candidates, none checked: the head-sequence trim and
the catch-up trim two minutes after boot; receipt or pin columns on tail rows;
rows left by the 21:46:09Z restart, when d@e applied its own layered release.

## Remaining error

- Which index rows or columns differ. The probe needs a per-row diff of
  `computer_event_index`, as it has for run memory and the object graph.
- Whether a quiet computer past 100,000 events is equivalent; that separates
  the trim logic from d@e's own state.
- d@e runs its own applied patch rebased onto c1ea7a418 (build identity
  1d9316e72, not a repo commit). The patch was not read, so that it leaves
  the lifecycle code alone is not confirmed.
- The diagnostic should read its samples in the snapshot.

## Code reading, 22:18Z (nothing run, no live computer touched)

- The probe replays a tail onto the newest snapshot (`restore_snapshot.go:117`),
  not from sequence 1; a snapshot copies the index whole
  (`snapshot_to.go:424`). Most replay-side rows are therefore an old live
  index, and only tail rows go through replay's own write path.
- Confirmed from code, not yet by a test: live normalizes a no-payload event's
  payload pin digests to `[]` (`appender.go:974`, `:1056`); replay leaves the
  slice nil (`internal/platform/event_replay.go:96`) and `Prepare` marshals it
  to `null`. The probe hashes the raw text. Whether d@e's tail holds such an
  event is not known.
- Suspected: the incremental trim removes one slice at a time and leaves older
  backlog to the catch-up that starts two minutes after boot. A base snapshot
  taken before a catch-up finished, or the staged store's own catch-up racing
  the four-minute probe, would leave batch rows on one side only.
- Not a cause, from code: the timestamp columns (excluded), receipt digests,
  any time-based trim condition.

A builder lane (`lane/event-index-diff`) is writing the tests for both, the
fix for the first, and a per-row diff of the index in the probe report with
the object-graph samples read in the snapshot.
