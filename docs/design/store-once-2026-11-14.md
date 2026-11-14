# Store once: where a computer's bytes should live

Written 2026-11-14T00:07Z by the lead (not delegated), on the owner's
request: "we are storing stuff redundantly, and it's not strategic ... what
are some strategic refactors that would make the system much more
efficient". Status: design, unreviewed. Nothing here is built. Every number
is marked measured or inferred.

## What one event costs today

Measured on the paper between its 21:05Z boot and 23:58Z on 2026-11-13
(`telemetry_batching` in its observability, Node B `btrfs filesystem du`,
control database):

| Layer | What it holds | Size for the paper | Growth |
| --- | --- | --- | --- |
| Tape segments (host) | every event, sealed | 72.9 GiB | 3.6 GB/day (438 MB in 2.9 h, 76,832 events, 5.7 KB each) |
| Receipts (host Postgres) | one row and five index entries per event | about 14 GB | about 1.4 GB/day (2.2 KB per event) |
| Guest store `state.db` | the projection of every event, kept forever | 43.8 GiB | about 2 GiB/day |
| Snapshot chunks (host) | page images of the whole guest store | 37.5 GiB | 4.8 GB sealed per snapshot |

About 168 GiB for one computer, growing about 7 GB a day, for a product
whose published content is a few hundred megabytes. The doctrine already
says which of these is the truth: the tape is canonical, the guest store is
a projection rebuilt by replay, a snapshot is a checkpoint of the projection,
and receipts are the host's side of the tape head. The other three layers
are meant to be caches or indexes of the first. None of them behaves like
one: each keeps everything, forever, at one-event granularity.

## Three causes

### 1. The tape records states, not changes

An `object_recorded` op carries the whole object. A mutable object is
written whole each time one field changes. Measured, same window:

| Object kind | Ops | Bytes | Per op | Share of tape bytes |
| --- | --- | --- | --- | --- |
| `choir.run` | 4,308 | 69.0 MB | 16.0 KB | 17% (76.1 MB by event) |
| `choir.texture_document` | 463 events | 78.9 MB | 170 KB | 18% |
| `choir.commitment_record` | 2,142 | 35.1 MB by event | about 16 KB per event | 8% |
| `choir.capture_label` | 469 events | 11.6 MB | 25 KB | 3% |
| `choir.lifecycle_command` | not counted | 20.6 MB | | 5% |

228 runs started in the window, so each run object was written about 19
times. `RunRecord` (`internal/types/task.go:89`) holds the run's `Prompt`,
`Result` and `Metadata`; every state change writes the prompt again. A
Texture document is written at 170 KB per revision beside a separate
`choir.texture_revision` op. Why it is 170 KB is inferred, not read: the
object most likely carries the body or its history.

This is the largest single cause on the tape: **about half of tape bytes are
unchanged content being written again.** Where an object embeds something
that grows, the cost is quadratic in the object's lifetime.

### 2. Each layer keeps everything

- The guest store never forgets. Retire ops exist and are almost unused: 1
  `run_memory_retire_recorded` and 6 `object_retire_recorded` against 48,179
  run-memory entries and 62,300 object writes in the window. 42,879 of the
  run-memory entries are conversation messages, about 190 per run started.
  What fills the 43.8 GiB by table is not measured yet
  (`lane/store-size`); run memory and `choir.event` rows are the inference.
- A snapshot images the whole guest store, so it costs what the store costs.
- A receipt is a row per event, with a signed receipt JSON and five indexes,
  kept for every event the computer has ever appended. Its jobs are
  idempotency for a retry and proof of the head. The first needs minutes of
  history, the second needs the chain's end.

### 3. Every event pays a fixed toll

One tape event today is 6.03 corpusd calls, a pin receipt, a head receipt,
a sealed envelope and a compression frame of its own. 88% of tape events
carry a single op. Payloads are compressed one at a time with no shared
dictionary, which does little for a 600-byte JSON body. Lossless coalescing
(S5, in review) attacks the count; nothing attacks the per-payload
compression.

A fourth, smaller cause: the same content is written twice in different
kinds. A tool's output is a run-memory message and also the payload of a
`tool.result` telemetry event (4,857 events, 20.1 MB, 4.1 KB each). Measured
share: about 5% of tape bytes, so it is not where to start.

## The frame

Three rules, each a restatement of doctrine that the storage layer does not
yet obey.

1. **A byte has one home.** Content of any size (a prompt, a message, a tool
   output, a document body, a capture) is stored once, by digest, in the
   computer's content store. Everything else names the digest.
2. **The tape records what changed.** An op on a mutable object carries the
   fields that changed. No object embeds a list that grows; a growing list
   is its own append-only kind.
3. **The tape is the archive, and it is the only one.** The guest store is
   the working set a running computer needs. A snapshot is a checkpoint of
   that working set. Receipts cover the tape by segment, not by event.

Rule 3 forces a choice that should be made deliberately: if the guest store
forgets old rows, the tape is the only copy, so the tape cannot also be
truncated behind a snapshot. One archive, kept whole. That is consistent
with the owner's ruling D1 (no tape segment leaves Node B, none dropped).
The lever on the tape's size is then rules 1 and 2, which make it small,
not truncation.

## The refactors, in order

Ordered by bytes saved per unit of risk. Each is its own station with its
own review and deployed proof.

### R1. Stop writing unchanged content (rules 1 and 2)

Per kind, in order of measured bytes: `choir.run`, `choir.texture_document`,
`choir.commitment_record`, `choir.lifecycle_command`, `choir.capture_label`,
`choir.source_schedule`.

For each: read what the object holds, split the immutable large fields
(prompt, body) from the mutable small ones (state, timestamps), and write
the large field once. Two shapes, to be chosen per kind after reading it:

- a large immutable field becomes a digest into the content store, written
  at creation and never again;
- a state change becomes an op that names the object and carries only the
  changed fields, with the revision it applies to.

Replay must produce byte-identical objects, so this is a tape format change
with an upcast for old events: red, two seats, replay-completeness proof on
d@e before the paper. Expected saving: about 40% of tape bytes and a similar
cut in what each snapshot has to carry for these kinds. Inferred from the
table above; to be precommitted in the ledger per kind before each build.

First step, cheap and yellow: add per-kind mean op size and writes-per-object
to the tape counters, so the claim "this kind rewrites itself N times" is
read, not inferred, and a new kind that does it is seen the day it ships.

### R2. The guest store is a working set (rule 3)

Wire the retirement that already exists. For each high-volume kind, name the
moment its rows stop being needed by a running computer:

- run memory: when the run is settled and its trajectory closed, plus the
  window in which a rewarm can still read it;
- `choir.event` telemetry rows: when the snapshot that covers them is named
  by the tape;
- superseded object revisions: when a newer revision is covered likewise.

Retirement is a tape event, so replay reproduces it. Reading something
retired (an old run's trace) goes to the archive: fetch the segment, open it
with the computer's key, answer. That read path does not exist and is the
real work of this station; without it retirement is deletion of evidence,
which is not acceptable. Freed pages must actually leave the file
(`auto_vacuum` or a compacting swap, to be decided from what the store sets
today).

Expected: the paper's store falls from 43.8 GiB to single digits, and
snapshots with it (size and the 37 minutes). The number waits for the
per-table reading. Red.

### R3. Receipts by segment

A sealed segment gets one receipt (its event range, its hash-chain end, its
digest). Per-event rows are kept only for the unsealed tail, which is all a
retry can collide with. 12.6 million rows and 28 GB become thousands of rows.
This is the existing S6; it follows S5 because coalescing changes what a
segment holds. Red.

### R4. Amortise the toll

- Land lossless coalescing (S5).
- Compress sealed payloads with a dictionary trained per kind, or compress
  per segment before sealing. Small JSON bodies with a shared dictionary
  typically shrink several-fold; the factor here is unmeasured. Orange to
  red (a sealed-format change with a version byte).

### R5. One home for conversation content

A telemetry event that describes a tool result names the run-memory entry
instead of carrying the output again. About 5% of tape bytes. After R1.

## What this does not do

- It does not move tape off Node B or drop any segment.
- It does not sample or lose telemetry (owner ruling D8).
- It does not add a setting. A kind's retirement moment is a property of the
  kind, declared by the package that owns it.

## Expected end state, stated as a bet

For a computer as busy as the paper: tape growth under 1.5 GB a day (from
3.6), the guest store under 8 GiB and flat (from 43.8 and rising), a
snapshot under 10 minutes (from 37), receipts under 0.1 GB (from 14). These
are inferences from one 2.9-hour window and one unmeasured store. Each
station precommits its own number in the calibration ledger before it is
built.

## Order of work

1. Now, in flight: updater prune, coalescing (S5), the per-table store
   reading.
2. R1 counters (yellow), then R1 for `choir.run` alone as the proof of the
   pattern, on d@e, with replay completeness.
3. The per-table reading decides whether R2 or the rest of R1 comes next.
4. R3 after S5. R4's compression after R1, since it changes the same bytes.

## Open questions for the owner (defaults taken)

- Reading retired history from the archive makes old traces slower to open
  (a segment fetch and unseal). Default: accept; a running computer's recent
  work stays local.
- R1 changes the tape's op format. Old events stay readable through an
  upcast forever. Default: yes, no rewrite of existing tape.
