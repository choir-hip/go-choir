# What the paper's tape holds, and how much of it compaction and coalescing would remove (2026-10-30)

Status: measurement only, mutation class green (docs). The production host
was read, never changed: sealed `tape-seg` segment files were read in place,
one process, throttled, metadata out only. No Dolt server, VM, or service
was touched. Follows
[tape-window-sizing-2026-10-30.md](tape-window-sizing-2026-10-30.md), whose
finding 3 (the guest event row has no large body; the large body is the
`projection_batch_recorded` payload) this note opens up. Serves the same
data-architecture question (retention window, what a restore window must
keep, what may be compacted after it).

Labels: **[V]** verified by a command in this run (Node B, 2026-10-30 05:13 to
05:36 UTC); **[I]** inferred or extrapolated, basis stated. MB and GB mean
10^6 and 10^9 bytes. "Op" is one `ProjectionOp` inside a
`projection_batch_recorded` payload
(`internal/computerevent/projection_batch.go`); an "event" is one batch, one
host append receipt, one guest tape row. Day is a UTC date of the event's
`occurred_at`.

Privacy: the owner computer's segments were read as a user's data. The
analyzer printed only table names, op kinds, object kinds, `choir.event`
event-kind names, counts, byte sizes and timestamps; keys were hashed in
memory and never printed; no row, document text, or value left the process.
[V by construction: the output schema is fixed in the program; the program
and its JSON output stayed in the session scratchpad outside the repo.]

## Headline

1. **A day of the paper's tape is 339,090 events carrying 480,446 ops
   and 1.535 GB of op bytes.** 91.9% of events hold exactly one op (p95 three
   ops, p99 twelve, max 279); two kinds of single-op event are 77.4% of all
   events: one `choir.event` log line (40.3%) or one `run_memory_entries`
   row (37.1%). Payload mean 4,697 bytes; 96.4% of it is op bytes, the batch
   envelope is 170 bytes [V, 2026-10-29]. A second day (10-27) agrees within
   a few points on every share below [V].
2. **Little of it is churn.** 15.8% of ops (75,960) overwrite a key already
   written that day; those superseded writes are 274.9 MB, 17.9% of op bytes.
   Nearly all of it is five kinds: `choir.run` (124.7 MB), `choir.source_schedule`
   (46.7 MB, 66 keys written 83 times each), `choir.actor_wake_outbox`
   (23.2 MB), `choir.story_library_head` (21.3 MB: one key written 997 times
   at 21 KB each), `choir.agent` (19.3 MB) [V].
3. **Classified, the bytes are four near-equal quarters**: acts 20.5%,
   mutable working state 24.3%, append-only working-state logs (run memory,
   command receipts, lifecycle and run-transition events) 29.9%, trace
   telemetry 25.3% [V counts, I classification, rule and doubts in section 3].
   Only the mutable-state quarter is churn; the two log quarters have one
   write per key and never shrink under "latest value per key".
4. **What survives compaction outside the window** (one-day span, bytes of
   ops / events, 10-29): latest value per key, everything else whole: **82.4%
   of bytes, 92.9% of events**. Also dropping trace telemetry: 57.0% / 59.6%.
   Also dropping the append-only state logs: 27.1% / 19.1%. Acts only: 20.5% /
   12.2% [V counts under the I classification]. Compaction alone is a 17.6%
   byte cut and a 7.1% event cut; the event and receipt count is moved
   by retiring the log classes, not by merging rewrites.
5. **Coalescing is the event and receipt lever**: one event per 1 s window
   leaves 61,827 events (81.8% fewer host append receipts, 81.8% fewer pin
   receipts); one per 250 ms leaves 159,008 (53.1% fewer); one per 5 s
   leaves 14,643 (95.7% fewer) [V from `occurred_at`]. It removes the
   per-event fixed cost, not op bytes: segment bytes fall 36.5% at 1 s and
   23.7% at 250 ms, and dedup of rewrites inside a window removes 0.6% of op
   bytes at 1 s and 0.1% at 250 ms [V]. The fixed cost of an event is about
   10.4 KB across segment, host receipt row and guest row, 2.3 times the
   4.5 KB of ops it carries [V seg, V previous note for the rows, I sum].
6. **Rate.** Mean 3.92 events per second over the day (4.45 over the 21 hours
   the computer was writing); peak minute 569 events (9.48 per second); peak
   second 18; 71.6% of seconds hold at least one event [V]. 10-27: mean 3.99,
   peak minute 627 (10.45 per second), peak second 21 [V].
7. **The telemetry batcher is not active on the paper.** All 339,090 events
   of 10-29 (and all 344,758 of 10-27) carry the authority
   `authority:vm-local-projection`; none carries
   `authority:vm-local-projection-telemetry`, the mark the sealer writes
   (`internal/store/projection_tape.go`, `telemetry_sealer.go`). The 112,986
   trace-line ops (23.5% of ops, 25.3% of bytes) each went out as their own
   acknowledged event [V]. The batcher's lane alone, at its default 1 s window,
   would have removed 18.6% of the day's events [I, upper bound: the sealer's
   64-op and 256 KB caps are not modelled].
8. **The owner computer (computer-0333*) is a different object.** Seven days
   (10-23 to 10-29): 1,271 events, 1,293 ops, 1.80 MB. 67.7% of ops and 81.1%
   of bytes are `choir.event` copies of desktop and media UI state; 32.2% are
   state snapshots of one key each; 2 ops (0.2% of bytes) are acts. One
   desktop interaction writes four events (three `desktop.*.updated` log lines
   and one workspace snapshot) [V counts, I same-instant]. Coalescing at 1 s
   leaves 436 of 1,271 events (65.7% fewer) [V].

## Method

- **Where the data is.** `/var/lib/go-choir/platform-artifacts/tape-seg/<computer>/seg-NNNNNNNN.seg`
  (root-owned, mode 640): the paper is `computer-1355b83bced1613eb048ee28d66688a8`
  (989 files, 494 sealed segments and one open), the owner computer
  `computer-03335285269bdba4f94377e56879f9e6` (8.4 MB). Format:
  `internal/tapestore/format.go` (32-byte header; records `CHRC`, version,
  namespace byte, 32-byte digest, length, crc). Each event is four records in
  this order: two pin receipts (namespace 3), the event envelope (namespace
  1), the payload (namespace 2) [V from record order].
- **The analyzer.** A standard-library Go program (about 700 lines), cross-built
  for linux/amd64, copied to the host's `/tmp`, run once per range with
  `sudo systemd-run --scope -q -p MemoryMax=2G -p CPUQuota=100% nice -n 19
  ionice -c3`, a 400 ms pause after each segment, `posix_fadvise` sequential
  then don't-need on each file so the page cache is not displaced. It lists
  the `.seg` files, skips any whose mtime is before the range start (the
  mtime of a sealed segment is its last append), parses the event envelope for
  `occurred_at`, `authority_ref`, `privacy_class` and `payload_commitment`,
  joins it to the payload record by digest, decodes the payload as a
  `ProjectionBatch` (all payloads are plaintext JSON: 0 opaque payloads in
  the three runs [V]) and keeps only events whose `occurred_at` is inside the
  range. It did not verify record crcs or digests (not needed for counts; a
  torn record would end a segment early, and no segment did: 34 unmatched
  events per day (23 on 10-27) are the pairs split across a segment boundary, 0.01%).
- **Keys.** The key of an op is the primary key of the row the projector
  upserts (`internal/store/project.go`): `og_objects` and `og_edges` by
  `canonical_id` / `edge_id` (ON DUPLICATE KEY UPDATE with the full row, so
  the last op per key reproduces the row), `run_memory_entries` by
  `entry_id`, `texture_agent_mutations` by (owner, computer, doc, loop),
  `desktop_*` by (owner, desktop), `user_preferences`, `media_*` and
  `texture_document_aliases` by their natural keys. `choir_events`-stamped
  ops (keyed by tape position, never compactable) are 0 in all three runs
  [V]. Keys were hashed with FNV-64a together with a span index; at about 1.5
  million keys per scenario a collision is expected below one [I].
- **Spans.** For each of seven spans (whole range, 1 day, 1 hour, 10 minutes,
  1 minute, 1 second, 250 ms) the program keeps, per (span, key), the number
  of ops, the byte size and event index of the last one, and a hash of the
  last body and metadata; "latest value per key per span" is the sum of the
  last sizes. The same pass counts the non-empty windows of 100 ms, 250 ms,
  1 s and 5 s from `occurred_at` (an event's coalesced window is the window of
  its timestamp).
- **Op bytes.** The exact length of the op's raw JSON inside the payload
  (kind, canonical id, table, body). The `body` of an `og_objects` op is the
  whole object JSON, whose own `body` field is base64, so about a quarter of
  those bytes is encoding overhead, not content [I from `objectgraph.Object`].
- **Ranges.** Paper: 2026-10-29 00:00 to 24:00 UTC (the last complete day;
  45 segments, 2.84 GB read; 339,090 events, against 339,120
  `projection_batch_recorded` host receipts for that date in the previous
  note: 30 fewer; the 34 boundary events not joined, less a few seconds'
  difference between receipt `created_at` and event `occurred_at` at
  midnight [V counts, I reason]) and 2026-10-27 as a replicate (344,758 events against
  344,776). Owner: 2026-10-23 to 10-30 (82 segment files, 6.4 MB).
- **Health and load.** The brief's address `10.201.66.2:8085` has no route on
  Node B; the paper's guest answers at `10.201.67.2:8085` (its `/health`
  names `computer-1355b83b...`, `running_runs` 8, `persistent_disk` 28.9%).
  That endpoint is jittery with nothing of mine running: 15 probes returned
  two 4-second timeouts and two answers above 2 s, and one 503 after 1.0 s
  appeared before any job [V]; the public `https://choir.news/health` was
  200 in 0.16 s throughout. During the 1-day runs load average was
  2.4 to 3.9 (12 cores) and the probe pattern matched the baseline (a few
  slow answers, an occasional 503/timeout, never a run of three). One
  attempt did cross the limit and was stopped: a 3-day paper range (CPU
  quota, no pause) was killed 51 s in when the 1-minute load average read 8.4
  (7.5 right after the kill, 5.0 a minute later; whether the job or the
  kernel's btrfs read workers caused it is not known). Every later run added
  the 400 ms pause and stayed at 3.9 or below. A 3-day span is
  therefore not measured; section 2 bounds it instead.
- **Cleanup.** The binary, the class file and every output on the host were
  removed (`ls /tmp | grep` for them is empty, checked twice); no systemd
  scope remains; nothing was written under `/var/lib/go-choir` [V].

## 1. What an event holds (paper, 2026-10-29)

Event envelope in the segment: 1,558 bytes (event record) plus two pin
receipts of 941 bytes each plus a 46-byte payload record header, 3,487 bytes
of fixed cost per event before the payload; the payload body adds the
170-byte batch envelope. Per-event total in the segment: 8,184 bytes
(3,657 fixed plus 4,527 op bytes) for the in-range events [V]; the 45
segments as read hold 539.9 MB of event records, 652.5 MB of receipts and
1,649.5 MB of payload records over 346,581 events (8,200 per event, edge
events of the neighbouring days included).

Batch shape [V]: 311,631 events with one op (91.9%), 6,292 with two, 13,040
with three or four, 3,376 with five to eight, 2,975 with nine to sixteen,
1,359 with seventeen to thirty-two, 269 with thirty-three to sixty-four, 148
above sixty-four (max 279). Payload bytes per event: mean 4,697, p50 2,187,
p90 8,925, p95 13,112, p99 49,436, max 1.31 MB. Op bytes: mean 3,195, p50
1,974, p95 10,607, p99 20,376, max 310,388.

The 52 distinct batch shapes (set of kinds in the batch): the top two are
single-op batches. One `choir.event` alone: 136,782 events (40.3%), 458 MB of
payload (28.8%). One `run_memory_entries` row alone: 125,926 events (37.1%),
330 MB (20.7%). Then `actor_wake_outbox` alone 12,231, `source_schedule`
alone 7,177, `texture_agent_mutations` alone 6,996, `choir.run` alone 6,226;
the multi-op shapes are run steps (`agent`, `event`, `run` together: 4,161
events; `actor_wake_outbox`, `agent`, `run`: 4,154) and Texture and inbox
commits (`commitment_record`, `conversation_head`, `inbox_command`,
`inbox_head`: 2,201) [V].

### 1.1 By projected table

Every op is one of four tables on this day; the other op kinds
(`desktop_state_recorded`, self-development, aliases, media, preferences)
have 0 ops [V].

| Projected table | Ops | Share of ops | Op bytes (MB) | Share of op bytes | Events holding it | Share of events |
|---|---|---|---|---|---|---|
| og_objects | 338,822 | 70.5% | 1,216.8 | 79.3% | 300,411 | 88.6% |
| run_memory_entries | 125,926 | 26.2% | 308.4 | 20.1% | 125,926 | 37.1% |
| og_edges | 8,702 | 1.8% | 5.5 | 0.4% | 5,674 | 1.7% |
| texture_agent_mutations | 6,996 | 1.5% | 4.3 | 0.3% | 6,996 | 2.1% |
| **total** | 480,446 | 100% | 1,535.0 | 100% | 339,090 | (a batch can hold several families) |

The `og_objects` row is 70.5% of ops and 79.3% of op bytes and is 29 object
kinds (49 rows below, `choir.event` split by event kind); `choir.event` objects alone are 141,739 ops (29.5%) and 442 MB (28.8%).

### 1.2 By object kind

All kinds at or above 0.1% of op bytes. `choir.event` is split by the event
kind in its metadata (the names are the runtime event vocabulary,
`internal/types/event_durability.go`). Class is section 3's rule and
carries its doubts.

| Table : object kind (`choir.event` split by its event kind) | Class | Ops | Share of ops | Op bytes (MB) | Share of op bytes | Events holding it | Share of events | Mean op bytes |
|---|---|---|---|---|---|---|---|---|
| run_memory_entries | STATELOG | 125,926 | 26.2% | 308.4 | 20.1% | 125,926 | 37.1% | 2,449 |
| choir.event/tool.result | TELEM | 33,380 | 6.9% | 208.8 | 13.6% | 33,380 | 9.8% | 6,255 |
| choir.run | STATE | 18,753 | 3.9% | 170.6 | 11.1% | 18,753 | 5.5% | 9,099 |
| choir.event/loop.progress | TELEM | 42,138 | 8.8% | 94.6 | 6.2% | 42,138 | 12.4% | 2,246 |
| choir.lifecycle_command | STATELOG | 4,703 | 1.0% | 81.5 | 5.3% | 4,533 | 1.3% | 17,322 |
| choir.event/desk.cell | TELEM | 33,475 | 7.0% | 77.2 | 5.0% | 33,475 | 9.9% | 2,305 |
| choir.source_ref | ACT | 16,444 | 3.4% | 68.4 | 4.5% | 2,821 | 0.8% | 4,157 |
| choir.texture_revision | ACT | 4,025 | 0.8% | 58.5 | 3.8% | 3,855 | 1.1% | 14,534 |
| choir.commitment_record | ACT | 13,072 | 2.7% | 54.4 | 3.5% | 11,717 | 3.5% | 4,161 |
| choir.source_schedule | STATE | 7,177 | 1.5% | 47.2 | 3.1% | 7,177 | 2.1% | 6,572 |
| choir.actor_wake_outbox | STATE | 24,460 | 5.1% | 46.3 | 3.0% | 23,136 | 6.8% | 1,893 |
| choir.inbox_command | STATELOG | 11,447 | 2.4% | 35.0 | 2.3% | 11,447 | 3.4% | 3,055 |
| choir.source_entity | ACT | 7,798 | 1.6% | 29.0 | 1.9% | 2,821 | 0.8% | 3,712 |
| choir.agent | STATE | 15,620 | 3.3% | 23.3 | 1.5% | 14,911 | 4.4% | 1,491 |
| choir.support_verdict | ACT | 6,843 | 1.4% | 21.4 | 1.4% | 2,744 | 0.8% | 3,130 |
| choir.story_library_head | STATE | 997 | 0.2% | 21.3 | 1.4% | 997 | 0.3% | 21,342 |
| choir.citation_check | ACT | 6,794 | 1.4% | 21.0 | 1.4% | 2,740 | 0.8% | 3,096 |
| choir.management_task | STATE | 3,919 | 0.8% | 12.1 | 0.8% | 3,178 | 0.9% | 3,093 |
| choir.trajectory | STATE | 6,476 | 1.3% | 11.3 | 0.7% | 6,306 | 1.9% | 1,744 |
| choir.capture_disposition | STATE | 6,803 | 1.4% | 11.0 | 0.7% | 2,331 | 0.7% | 1,619 |
| choir.conversation_head | STATE | 7,107 | 1.5% | 10.9 | 0.7% | 6,196 | 1.8% | 1,532 |
| choir.inbox_head | STATE | 10,728 | 2.2% | 9.6 | 0.6% | 9,432 | 2.8% | 898 |
| choir.lifecycle_event | STATELOG | 4,703 | 1.0% | 9.5 | 0.6% | 4,533 | 1.3% | 2,021 |
| choir.event/loop.completed | STATELOG | 5,020 | 1.0% | 8.4 | 0.5% | 5,020 | 1.5% | 1,673 |
| choir.event/loop.submitted | STATELOG | 5,063 | 1.1% | 8.0 | 0.5% | 5,063 | 1.5% | 1,580 |
| choir.event/loop.started | STATELOG | 5,067 | 1.1% | 7.9 | 0.5% | 5,067 | 1.5% | 1,550 |
| choir.event/desk.egress | TELEM | 3,853 | 0.8% | 7.6 | 0.5% | 3,853 | 1.1% | 1,972 |
| choir.texture_document | ACT | 4,025 | 0.8% | 6.3 | 0.4% | 3,855 | 1.1% | 1,570 |
| choir.event/channel.message | ACT | 2,264 | 0.5% | 6.2 | 0.4% | 2,264 | 0.7% | 2,744 |
| choir.event_membership | ACT | 3,362 | 0.7% | 6.1 | 0.4% | 3,362 | 1.0% | 1,802 |
| choir.event/texture.agent_revision.started | ACT | 3,491 | 0.7% | 6.0 | 0.4% | 3,491 | 1.0% | 1,720 |
| choir.channel_message | ACT | 2,264 | 0.5% | 5.7 | 0.4% | 2,264 | 0.7% | 2,520 |
| og_edges | ACT | 8,702 | 1.8% | 5.5 | 0.4% | 5,674 | 1.7% | 633 |
| choir.event/texture.document_revision.created | ACT | 2,864 | 0.6% | 5.4 | 0.4% | 2,864 | 0.8% | 1,882 |
| choir.inbox_cursor | STATE | 5,289 | 1.1% | 5.2 | 0.3% | 5,289 | 1.6% | 985 |
| choir.event/texture.agent_revision.completed | ACT | 2,865 | 0.6% | 5.2 | 0.3% | 2,865 | 0.8% | 1,799 |
| choir.capture_label | ACT | 1,098 | 0.2% | 4.7 | 0.3% | 1,098 | 0.3% | 4,245 |
| choir.event/system1.judgment | ACT | 1,363 | 0.3% | 4.3 | 0.3% | 1,363 | 0.4% | 3,162 |
| texture_agent_mutations | STATE | 6,996 | 1.5% | 4.3 | 0.3% | 6,996 | 2.1% | 609 |
| choir.world_event | ACT | 2,906 | 0.6% | 3.8 | 0.2% | 2,906 | 0.9% | 1,302 |
| choir.event/story.entities | ACT | 647 | 0.1% | 1.7 | 0.1% | 647 | 0.2% | 2,685 |
| other (11 kinds under 0.1% of bytes) | mixed | 519 | 0.1% | 1.5 | 0.1% | 519 | 0.2% | 2,847 |

## 2. Churn

Definition: an op "overwrites" if its key was already written earlier in the
same span. Latest-per-key bytes use the size of the last op per key, so they
are what a compacted copy would hold if the last op were kept verbatim.

By class on 10-29 [V counts, I class]:

| Class | Ops | Share of ops | Op bytes (MB) | Share of op bytes | Distinct keys (day) | Latest per key per day (MB) | Latest per key per hour (MB) |
|---|---|---|---|---|---|---|---|
| ACT | 91,096 | 19.0% | 314.2 | 20.5% | 88,338 | 309.8 | 310.4 |
| STATE | 114,329 | 23.8% | 373.1 | 24.3% | 41,127 | 102.6 | 127.6 |
| STATELOG | 162,035 | 33.7% | 458.8 | 29.9% | 162,035 | 458.8 | 458.8 |
| TELEM | 112,986 | 23.5% | 388.9 | 25.3% | 112,986 | 388.9 | 388.9 |

Overall: 404,486 distinct keys for 480,446 ops, so 15.8% of ops overwrite; the
superseded ops are 274.9 MB of 1,535.0 MB (17.9%) at a one-day span and
249.2 MB (16.2%) at one hour. 0 rewrites were byte-identical in body and
metadata on the paper (every rewrite carries a real change) [V]; the owner
computer has 13 byte-identical rewrites of 60 ops (`user_preferences` 4,
`media_recents` 4, `media_progress` 5) [V].

The 14 kinds with any overwrite on 10-29 (ordered by bytes saved per day).
Ops-per-key columns: over all keys of the kind, then over the keys written at
least twice [V]:

| Kind | Class | Ops | Distinct keys | Ops overwriting a same-day key | Ops per key p50 / p95 / max (all keys) | p50 / p95 (rewritten keys only) | Op bytes (MB) | Latest per key per day (MB) | Latest per key per hour (MB) | Saved per day (MB) |
|---|---|---|---|---|---|---|---|---|---|---|
| choir.run | STATE | 18,753 | 5,130 | 72.6% | 4 / 4 / 12 | 4 / 4 | 170.6 | 45.9 | 48.5 | 124.7 |
| choir.source_schedule | STATE | 7,177 | 66 | 99.1% | 83 / 165 / 167 | 83 / 165 | 47.2 | 0.4 | 9.3 | 46.7 |
| choir.actor_wake_outbox | STATE | 24,460 | 12,232 | 50.0% | 2 / 2 / 2 | 2 / 2 | 46.3 | 23.1 | 23.1 | 23.2 |
| choir.story_library_head | STATE | 997 | 1 | 99.9% | 997 / 997 / 997 | 997 / 997 | 21.3 | 0.0 | 0.4 | 21.3 |
| choir.agent | STATE | 15,620 | 2,743 | 82.4% | 3 / 16 / 1,217 | 4 / 19 | 23.3 | 4.0 | 5.0 | 19.3 |
| choir.management_task | STATE | 3,919 | 1,213 | 69.0% | 4 / 4 / 4 | 4 / 4 | 12.1 | 4.0 | 6.7 | 8.1 |
| choir.trajectory | STATE | 6,476 | 2,250 | 65.3% | 2 / 6 / 75 | 2 / 6 | 11.3 | 3.9 | 4.9 | 7.4 |
| choir.inbox_head | STATE | 10,728 | 2,887 | 73.1% | 1 / 7 / 4,428 | 3 / 11 | 9.6 | 2.6 | 3.2 | 7.1 |
| choir.texture_document | ACT | 4,025 | 1,267 | 68.5% | 2 / 7 / 40 | 3 / 7 | 6.3 | 2.0 | 2.6 | 4.3 |
| choir.conversation_head | STATE | 7,107 | 4,362 | 38.6% | 2 / 2 / 8 | 2 / 2 | 10.9 | 6.7 | 10.1 | 4.2 |
| choir.capture_disposition | STATE | 6,803 | 4,017 | 41.0% | 2 / 2 / 4 | 2 / 2 | 11.0 | 6.9 | 10.9 | 4.1 |
| choir.inbox_cursor | STATE | 5,289 | 2,724 | 48.5% | 1 / 5 / 265 | 3 / 8 | 5.2 | 2.7 | 3.2 | 2.5 |
| texture_agent_mutations | STATE | 6,996 | 3,500 | 50.0% | 2 / 2 / 6 | 2 / 2 | 4.3 | 2.3 | 2.3 | 2.0 |
| choir.stream_seq_lease | STATE | 3 | 1 | 66.7% | 3 / 3 / 3 | 3 / 3 | 0.0 | 0.0 | 0.0 | 0.0 |

The other 38 kinds (362,093 ops, 1,155.6 MB: 75% of op bytes) have exactly
one op per key, every key distinct [V]. Notes on the shape of the churn
[I unless marked]:

- `choir.run`: four writes per run (p50 = p95 = 4) at a mean of 9.1 KB each;
  the final row is 27% of the bytes written. One run reaches 12 writes.
- `choir.source_schedule`: 66 schedule keys, 83 writes per key per day (p95
  165), 6.6 KB per write; the schedule is rewritten about every 15 to 17 minutes
  and its latest values are 437 KB for all 66 keys for the whole day.
- `choir.story_library_head`: one key written 997 times, 21.3 KB each: a
  counter whose body carries a list. A per-hour span keeps 21 of the
  997 (one per active hour, 449 KB).
- `choir.actor_wake_outbox`: exactly two writes per key (enqueue and done):
  the churn is 50%, not more, however long the span.
- `choir.agent`: median 3 writes per key, p95 16, one key 1,217 times;
  `choir.inbox_head`: one key 4,428 times (898 bytes each).
- `texture_agent_mutations`: two writes per key at p50 and p95 (max 6): a
  start and a completion [V].

Span effect, all kinds, latest op per key per span, 10-29 and the 10-27
replicate [V]:

| Compaction span (latest op per key per span, all kinds) | Ops kept | Share of ops | Op bytes kept (MB) | Share of op bytes | Events with a kept op | Share of events | Bytes share on 10-27 |
|---|---|---|---|---|---|---|---|
| 1 day | 404,486 | 84.2% | 1,260.1 | 82.1% | 315,171 | 92.9% | 81.7% |
| 1 hour | 414,834 | 86.3% | 1,285.8 | 83.8% | 317,065 | 93.5% | 83.5% |
| 10 minutes | 425,100 | 88.5% | 1,323.5 | 86.2% | 321,871 | 94.9% | 86.1% |
| 1 minute | 447,186 | 93.1% | 1,408.0 | 91.7% | 332,013 | 97.9% | 92.0% |
| 1 second | 476,133 | 99.1% | 1,526.3 | 99.4% | 338,422 | 99.8% | 99.4% |
| 250 ms | 479,036 | 99.7% | 1,533.4 | 99.9% | 339,032 | 100.0% | 99.8% |

Reading the spans. A one-hour span keeps 83.8% of bytes against 82.1% for a
day: the churn is fast, not slow, so a compaction pass does not need a long
span. Spans of one second or less remove 0.6% or less: **two writes to one
key inside a coalescing window are rare**, so coalescing is a framing
saving, not a dedup saving. Beyond a day: the 12 kinds that rewrite objects
created before the day (`created_at` earlier than 10-29: 5,365 of the 10,728
`inbox_head` ops, 2,876 of the 15,620 `agent` ops, and so on) have 82.5 MB of
latest values on 10-29, so joining days can remove at most 6.5% of the day's
survivors, and 3.6 points of that is `choir.run`, which is 99.6% same-day
(84 pre-day ops of 18,753) [I, bound]. The 3-day range that would measure it
was not run (see Health and load).

## 3. Classification

Rule. Each kind gets the class of the first test it meets:

- **ACT**: the row is a durable owner or agent act, or evidence a later reader
  needs as history; losing an old value loses a fact not recoverable from any
  other key's latest value. Texture documents and revisions, precommitment
  (`choir.commitment_record`), the source graph and the judgments over it
  (`source_ref`, `source_entity`, `support_verdict`, `citation_check`,
  `capture_label`), inter-agent channel messages, `world_event` and its
  membership, graph edges, and the acknowledged event kinds the code itself
  names owner acts or evidence (`texture.*` revision events, `system1.judgment`,
  `channel.message`, `story.entities`, `file.changed`).
- **TELEM**: trace and derived copies. `choir.event` kinds that
  `internal/types/event_durability.go` classes as `batched` trace lines
  (`tool.result`, `loop.progress`, `desk.cell`, `desk.egress`, `loop.retry`,
  `loop.compaction.*`), and the owner computer's UI-state event copies
  (`desktop.*.updated`, `theme.updated`, `media.*.updated`), which duplicate
  a snapshot op written beside them.
- **STATE**: one row per key whose write is a full-row snapshot of a mutable
  current value (status, lease, cursor, head, schedule, outbox, assignment,
  desktop layout, preferences). Test: the type's own comment calls it a
  head, cursor, lease, cache, outbox or row, and its keys are rewritten
  somewhere in the data. Only the latest value is read.
- **STATELOG**: working state kept as an append-only series of unique keys:
  run memory entries, command and idempotency receipts
  (`lifecycle_command`, `inbox_command`), `lifecycle_event`, and the run
  transition events (`loop.submitted`, `.started`, `.completed`, `.failed`,
  `.cancelled`, `activation.passivated`). They serve resume, idempotent replay
  and run bookkeeping; none is a product act. "Latest per key" cannot shrink
  them; only a retirement rule (run closed, window passed) can.

Uncertain classifications, and what each moves. Base: tier C keeps 57.0% of
bytes and 59.6% of events, tier D 27.1% and 19.1% [V arithmetic on the 10-29
groups; event moves are upper bounds because a moved op's batch may already be
kept].

| Uncertain | Bytes | Why it may be the other class | Effect if kept as evidence |
|---|---|---|---|
| `choir.event/tool.result` (TELEM) | 208.8 MB, 13.6% | the code says run acceptance and the Texture workflow verifier read it as evidence | tier C 57.0% to 70.6% of bytes, 59.6% to at most 69.5% of events |
| `run_memory_entries` (STATELOG) | 308.4 MB, 20.1% | "run memory" is working state by the owner's list, but it is the audit record of what each agent saw and said | tier D 27.1% to 47.2% of bytes, 19.1% to at most 56.3% of events |
| `lifecycle_command`, `inbox_command` (STATELOG) | 81.5 MB + 35.0 MB, 7.6% | receipts for idempotent replay, but `lifecycle_command` embeds the stored result (events, document, revision, turn) and `inbox_command` may be the record of a delegation | tier D 27.1% to 34.7% of bytes, 19.1% to at most 23.8% of events |
| `source_ref`, `source_entity`, `support_verdict`, `citation_check`, `capture_label` (+override), `world_event`, `event_membership`, `og_edges` (ACT) | 160.4 MB, 10.5% | derived graph content, recomputable at paid model cost; acts only if the graph is the product | tier E and D fall by up to 10.5 points of bytes |
| `choir.texture_document` (ACT) | 6.3 MB, 0.4% | 68.5% of its ops overwrite (a head pointer), but documents are acts by the owner's list | moving it to STATE saves 4.3 MB |
| `loop.*` transition events (STATELOG) | 24.4 MB, 1.6% | duplicate the `choir.run` row's transitions [I] | none that matters |
| `desktop.*`, `theme`, `media.*` events (TELEM, owner computer only) | 1.0 MB | code comments call them acknowledged owner-act product events | owner computer only: tier C keeps 1.5% of bytes if they are telemetry, 82.6% if they are kept |

The two doubts that matter are `tool.result` and `run_memory_entries`; they
are 33.7% of bytes between them. Everything else moves the answer by under
11 points.

## 4. What survives compaction outside a window

Tiers, cumulative, over the 10-29 day. "Kept" is the ops and events that must
exist after the pass; the host append receipt count equals the events kept (one
per event), the pin receipts twice that [V: namespace 3 records = 2 x
events]. Bytes are op bytes (the figure compaction can rewrite or drop); the
fixed 3,657 bytes per kept event is added in the next table. [V counts, I the
class assignment.]

| Tier (span = one UTC day) | Events kept | Share | Ops kept | Share | Op bytes kept (MB) | Share |
|---|---|---|---|---|---|---|
| A  none (as written) | 339,090 | 100% | 480,446 | 100% | 1,535.0 | 100% |
| A' latest per key, all kinds | 315,171 | 92.9% | 404,486 | 84.2% | 1,260.1 | 82.1% |
| B  state to latest, everything else whole | 315,171 | 92.9% | 407,244 | 84.8% | 1,264.4 | 82.4% |
| C  B and trace telemetry dropped | 202,185 | 59.6% | 294,258 | 61.2% | 875.5 | 57.0% |
| D  C and append-only state logs dropped | 64,825 | 19.1% | 132,223 | 27.5% | 416.7 | 27.1% |
| E  acts only | 41,305 | 12.2% | 91,096 | 19.0% | 314.2 | 20.5% |

The same tiers with a one-hour span (A' is the same as a one-day span within
2 points):

| Tier (span = one hour) | Events kept | Share | Ops kept | Share | Op bytes kept (MB) | Share |
|---|---|---|---|---|---|---|
| A' latest per key, all kinds | 317,065 | 93.5% | 414,834 | 86.3% | 1,285.8 | 83.8% |
| B  state to latest, everything else whole | 317,065 | 93.5% | 417,207 | 86.8% | 1,289.5 | 84.0% |
| C  B and trace telemetry dropped | 204,079 | 60.2% | 304,221 | 63.3% | 900.6 | 58.7% |
| D  C and append-only state logs dropped | 66,890 | 19.7% | 142,186 | 29.6% | 441.8 | 28.8% |
| E  acts only | 41,305 | 12.2% | 91,096 | 19.0% | 314.2 | 20.5% |

Replicate day, 10-27 (every share within 2.3 points of 10-29) [V]:

| Tier (span = one UTC day), 10-27 | Events kept | Share | Ops kept | Share | Op bytes kept (MB) | Share |
|---|---|---|---|---|---|---|
| A  none (as written) | 344,758 | 100% | 479,526 | 100% | 1,385.9 | 100% |
| A' latest per key, all kinds | 319,169 | 92.6% | 400,632 | 83.5% | 1,131.8 | 81.7% |
| B  state to latest, everything else whole | 319,169 | 92.6% | 403,064 | 84.1% | 1,135.6 | 81.9% |
| C  B and trace telemetry dropped | 208,042 | 60.3% | 291,937 | 60.9% | 767.6 | 55.4% |
| D  C and append-only state logs dropped | 70,140 | 20.3% | 126,747 | 26.4% | 357.1 | 25.8% |
| E  acts only | 44,678 | 13.0% | 79,886 | 16.7% | 253.3 | 18.3% |

If kept events stay as written (no repack), each kept event also keeps its
3,657 bytes of segment overhead, so the segment-level share is: tier B 87.1%
(2,417 MB of 2,775 MB), C 58.2% (1,615 MB), D 23.6% (654 MB), E 16.8%
(465 MB) [I from V unit costs]. If the survivors are repacked into a few
snapshot events, segment bytes approach the op-byte columns above (82.4%,
57.0%, 27.1%, 20.5%) [I]. Cross-check on the earlier note's receipt bound:
`commitment_record` is held by 11,717 events, 3.46% of the day, against the
earlier "at most 3.3%" (10,823 records of 326,662 events) [V].

What the table says. (1) "Latest value per key" removes the superseded 17.9%
of bytes but only 7.1% of events (23,919): most superseded ops sit in
multi-op batches that also carry a unique-key op, and most of the 311,631
single-op events are unique-key log lines. (2) The two big cuts are class
retirements, not compactions: dropping the 112,986 trace-line events is a
further 25.4 points of bytes and 33.3 points of events; dropping the
state-log events is a further 29.9 points of bytes and 40.5 points of events.
(3) What a restore window has to keep is therefore decided by policy for two
classes (trace lines, run memory and receipts), not by the amount of churn.

## 5. Rate, peaks and coalescing

### 5.1 Rate (10-29, 86,400 seconds) [V]

| Measure | 10-29 | 10-27 |
|---|---|---|
| Events | 339,090 | 344,758 |
| Mean per second, whole day | 3.92 | 3.99 |
| Mean per second over the hours with writes | 4.45 (21 h) | 4.16 (23 h) |
| Hours with no event | 20, 21 UTC (hour 22: 2,647) | 18 UTC |
| Seconds with at least one event | 61,827 (71.6%) | 57,932 (67.1%) |
| Events per non-empty second p50 / p95 / p99 | 5 / 12 / 14 | 5 / 13 / 15 |
| Peak second (top five all equal) | 18 | 21 |
| Peak 10-second window | 119 (11.9 per second) | 131 |
| Events per minute p50 / p95 / p99 | 252 / 394 / 463 | 236 / 465 / 519 |
| Peak minute | 569 (9.48 per second) | 627 (10.45 per second) |
| Peak minute over day mean | 2.4 x | 2.6 x |

Hourly events on 10-29: 15,714; 17,075; 16,249; 16,346; 18,699; 18,768;
19,909; 19,839; 16,834; 17,119; 14,537; 16,122; 15,479; 15,002; 15,244; 15,223;
15,624; 14,819; 14,800; 12,448; 0; 0; 2,647; 10,593. The computer wrote nothing
for about 2.5 hours from 20:00 UTC (hour 22 is partial); the cause was not looked into (a restart
or update would explain it [I]). Because of that hole the "day" is not a
steady-state day: a 24-hour steady mean at the 4.45 per second rate is 384,000.

### 5.2 Coalescing guest writes

All guest writes in one window become one event with the union of the ops
(op bytes unchanged). Windows are counted from `occurred_at`; a window with
no event produces none. Segment bytes use the fixed 3,657 bytes per event
measured above [V] plus 1,535.0 MB of op bytes; the baseline 2,775 MB
reproduces the measured payload plus overhead exactly.

| Coalescing window (all guest writes) | Events per day | Fewer events | Mean batches merged per event | Host append receipts | Pin receipts | Segment bytes per day (MB) | Fewer segment bytes |
|---|---|---|---|---|---|---|---|
| as written (no coalescing) | 339,090 | - | - | 339,090 | 678,180 | 2,775 | - |
| 1 event per 100 ms | 249,207 | 26.5% | 1.4 | 249,207 | 498,414 | 2,446 | 11.8% |
| 1 event per 250 ms | 159,008 | 53.1% | 2.1 | 159,008 | 318,016 | 2,117 | 23.7% |
| 1 event per 1 s | 61,827 | 81.8% | 5.5 | 61,827 | 123,654 | 1,761 | 36.5% |
| 1 event per 5 s | 14,643 | 95.7% | 23.2 | 14,643 | 29,286 | 1,589 | 42.8% |

Effects not in the table [I unless marked]:

- Dedup inside the window is negligible: latest-per-key inside a 1 s window
  keeps 99.4% of op bytes, inside 250 ms 99.9% [V, section 2].
- The guest tape row (4,409 bytes per event at 4 KiB pages, previous note [V])
  and the host receipt row (2,362 bytes logical, 3,139 in PostgreSQL with
  indexes [V previous note]) fall with the event count. At 1 s the 277,263
  events no longer written are about 1.01 GB per day of segment, 0.65 GB of
  host receipt rows (0.87 GB in PostgreSQL) and 1.22 GB of guest rows: about
  2.9 GB per day across the three layers, against a 2.8 GB segment day today.
- Peak load on the receipt path falls with the window: at most 1 event per
  second at 1 s (it was up to 18), at most 4 per second at 250 ms.
- It is not free: every acknowledged write waits for its window to close, up
  to 250 ms or 1 s of added latency per write, unless the window closes early
  on a quiet system. Not measured.
- Coalescing and retirement act on different terms (events versus op bytes);
  they were not run jointly, and the joint figure is not the product of the
  two [I].

Two narrower designs, each coalescing one lane and writing every other event
at once. The trace-only lane is what the telemetry sealer is built to do
(Headline item 7): the 112,986 events whose every op is trace telemetry
(33.3% of events). The low-authority lane is every event with no act and no
mutable-state op (trace and state logs: 245,849 events, 72.5%):

| Window | Trace-only lane coalesced: events per day | Fewer | Low-authority lane coalesced: events per day | Fewer |
|---|---|---|---|---|
| 100 ms | 328,095 | 3.2% | 287,361 | 15.3% |
| 250 ms | 314,416 | 7.3% | 223,936 | 34.0% |
| 1000 ms | 275,979 | 18.6% | 149,084 | 56.0% |
| 5000 ms | 240,479 | 29.1% | 107,702 | 68.2% |

Coalescing everything at 1 s (61,827) beats coalescing the low-authority lane
at 1 s (149,084) because the 93,241 events that hold an act or a
mutable-state op (27.5%) stay one per event in the lane design. [V counts;
the sealer's own caps of 64 ops and 256 KB per batch are not modelled.]

## 6. The owner computer, seven days (10-23 to 10-29)

Small and quiet: 1,271 events, 1,293 ops (max 6 per batch; 1,265 events hold
one op), op bytes 1.80 MB, payload mean 1,584 bytes. Authority and privacy
class are uniform (`authority:vm-local-projection`, `owner`). Events per
day: 150, 76, 169, 377, 342, 5, 152 [V]. Peak minute 61 events (1.02 per
second), peak second 10, mean 0.0021 per second [V].

| Table : kind | Class | Ops (7 d) | Op bytes | Share of bytes | Events holding it | Distinct keys (per-day span, summed) | Max ops on one key in a day |
|---|---|---|---|---|---|---|---|
| choir.event/desktop.window_placement.updated | TELEM | 279 | 458,790 | 25.5% | 279 | 279 | 1 |
| choir.event/desktop.app_instances.updated | TELEM | 279 | 456,824 | 25.4% | 279 | 279 | 1 |
| choir.event/desktop.driver_lease.updated | TELEM | 279 | 456,419 | 25.4% | 279 | 279 | 1 |
| desktop_workspaces(+instances,placements,sessions) | STATE | 279 | 211,977 | 11.8% | 279 | 6 | 81 |
| choir.stream_seq_lease | STATE | 76 | 63,156 | 3.5% | 76 | 6 | 30 |
| user_preferences | STATE | 14 | 38,342 | 2.1% | 12 | 3 | 9 |
| choir.event/theme.updated | TELEM | 8 | 35,436 | 2.0% | 8 | 8 | 1 |
| choir.event/media.recent.updated | TELEM | 16 | 28,702 | 1.6% | 16 | 16 | 1 |
| choir.event/media.progress.updated | TELEM | 14 | 20,974 | 1.2% | 14 | 14 | 1 |
| media_recents | STATE | 24 | 13,792 | 0.8% | 18 | 5 | 9 |
| media_progress | STATE | 22 | 8,787 | 0.5% | 19 | 5 | 11 |
| choir.event/file.changed | ACT | 2 | 3,030 | 0.2% | 2 | 2 | 1 |
| texture_document_aliases | STATE | 1 | 389 | 0.0% | 1 | 1 | 1 |

Classes: 875 TELEM ops (1.46 MB, 81.1% of bytes), 416 STATE ops (0.34 MB,
18.7%), 2 ACT ops (3.0 KB, 0.17%), 0 STATELOG [V counts, I class]. Survival
at a one-day span: state to latest (tier B) 897 events (70.6%) and 1.48 MB
(82.6%); trace dropped (tier C, and D, which are equal here) 22 events
(1.7%) and 26.8 KB (1.5%); acts only 2 events. Across the whole seven days
as one span, tier C is 7 events, 9.6 KB (0.5%). Coalescing: 845 events at
100 ms, 595 at 250 ms (53.2% fewer), 436 at 1 s (65.7% fewer), 287 at 5 s
(77.4% fewer). Churn is real but tiny: `desktop_workspaces` was rewritten 81
times on one key in a day, `choir.stream_seq_lease` 30, `user_preferences` 9,
`media_*` up to 11.

Caveat: the host receipt table for these days (previous note, all event
kinds) has 1,975 events for 10-23 to 10-29, while the segments hold 1,271
`projection_batch_recorded` events. The remaining 704 (about 100 per day)
are not in these segments; the cause is not established [V counts]. Events
that are not projection batches (`file_root_committed`, `artifact_produced`,
`key_revoked`) are written as one-file-per-digest pins, not segments
(previous note section 2.3), and their rate on the paper (about 80
`file_root_committed` per day) fits an explanation [I].

## 7. Conclusion

Of what the paper writes in a day (339,090 events, 1.535 GB of op bytes),
about **82% of bytes and 93% of events survive "latest value per key"**
outside a window, because only 15.8% of ops overwrite and the rest are unique
keys, mostly log lines. What survives depends on policy for two classes the
tape does not need as state: **trace telemetry** (25% of bytes, 33% of events:
dropping it leaves 57% and 60%) and **append-only working-state logs** (30% of
bytes, 40% of events: dropping those leaves 27% of bytes and 19% of events).
**Acts alone are 20.5% of bytes and 12.2% of events**, under a class
assignment whose two doubtful members, `tool.result` and `run_memory_entries`,
decide 34 points of bytes. The churning mutable state is real but small:
373 MB per day collapsing to 103 MB, 17.6 points of the whole.

**Coalescing guest writes cuts events and receipts by 81.8% at 1 s, 53.1% at
250 ms and 95.7% at 5 s**, and segment bytes by 36.5%, 23.7% and 42.8%,
because an event costs 10.4 KB of fixed overhead for 4.5 KB of ops. It does
not reduce op bytes: simultaneous writes are almost never to the same key.
For the receipt and row counts the window is the lever; for retained bytes,
the retirement of two classes is; compaction of rewritten keys is the
smallest of the three.

Residual risks and the next measurement:

- One day, replicated by a second; the paper's rate was not stationary
  since 10-13 (90 k to 742 k events per day, previous note), and 10-29 has a
  2.5 hour hole. Shares are stable between the two days; absolute bytes are not.
- The class assignment is mine from the registry, type comments and
  `event_durability.go`, not an owner ruling; section 3 lists the doubts.
- Whether `created_at` survives a latest-per-key rewrite was not checked: the
  projector's ON DUPLICATE KEY UPDATE list omits `created_at`
  (`internal/store/project.go`), so a compacted copy must carry the original
  first-write value [I, not tested].
- Op bytes are JSON, with base64 bodies; a binary encoding would shrink the
  bytes column by roughly a quarter on object bodies [I].
- Not measured: a multi-day compaction span (bounded at 6.5% of survivors),
  the added write latency of coalescing, and the joint effect of coalescing
  and retirement. A next probe would run a 3-day paper range with the same
  throttle, only when the paper's health baseline is quiet.
- The trace-telemetry batcher exists and is not running on the paper (one
  line of evidence: no telemetry-authority event in 684,000 events of two
  days). Whether it is off by setting, by flag or by a refused boot was not
  looked up; a single read of the paper's `telemetry.batch` setting would
  settle it.
