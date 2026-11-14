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

## Code investigation and repair (lane/event-index-diff, same day)

Neither cause below is confirmed on d@e: the deployed proof is a probe
reading after this lands, and its `event_index` field if it still differs.

- Cause 2, a null where live has `[]` (reproduced by a test, not on d@e). The
  live append normalizes `PayloadPinReceiptDigests` with `nonNilStrings`, so
  an event with no payload pins stores `[]` in
  `payload_pin_receipt_digests_json`. The tape replay
  (`platform/event_replay.go`) sets the list only when an event has payload
  pins, so `Store.Prepare` marshalled nil to `null`. The probe hashes the raw
  text. An end to end probe test with one tail event of that kind answered
  not_equivalent on `computer_event_index` and `content_root`, the two keys of
  this record, and is equivalent after `Prepare` writes `[]` for both.
  d@e's own rows: unread. A tail of 216 events on d@e almost certainly holds
  such events, but that is a belief.
- Cause 1, trim timing (mechanism tested, not seen on d@e). The incremental
  trim deletes the last slice at each trim point; the older backlog goes in a
  catch-up two minutes after each open, on the live store and on the probe's
  staged store, and a base snapshot copied before a catch-up finished
  carries the backlog. The two sides then differ by rows the trim defines as
  disposable. The probe now leaves those rows (finalized
  `projection_batch_recorded` at or below the trim boundary of the pinned
  head) out of its index digest on both sides and reports their counts.
- Stores written by an earlier replay keep `null` in that column inside their
  base snapshots; the probe reads null as `[]` there. The stored witness is
  unchanged, so recorded checkpoints still verify.
- The report has `event_index` (schema_version 5): the per-row diff, filled
  only when the result is not equivalent. The object graph diagnostic now
  reads its live rows in the pinned snapshot (telemetry partition rows, which
  are separate files, are still read at the time of the call).

Remaining error: checkpoint binding uses the strict extractor, so on a
store whose lineage mixes null and `[]` the probe can say equivalent and a
bind can still refuse (it fails closed). Whether d@e has such a lineage is
unread.
