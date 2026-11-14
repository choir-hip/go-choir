# Store once: where a computer's bytes should live

Written 2026-11-14T00:07Z by the lead (not delegated), on the owner's
request: "we are storing stuff redundantly, and it's not strategic ... what
are some strategic refactors that would make the system much more
efficient". Status: design, revision 2 after a two-seat review at 2026-11-14T01:25Z (both REPAIR; what they found is in "What the review corrected"). Nothing here is built. Every number
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

## What the review corrected

Revision 1 ranked the work from a counter that lied and from a reading of
the code that was not done. Both seats read the code and found:

1. **The per-kind table was wrong.** `tapeFamily`
   (`internal/store/tape_write_counters.go:58`) gives a whole batch's bytes
   to the kind of its first op. A Texture write is a batch of
   `[document, revision, sources, lifecycle command]`; the document is about
   300 bytes and has no body (`internal/types/texture.go:58`). The 170 KB
   belongs to the revision and what rides with it. Every "bytes per kind"
   figure in revision 1 is by event, not by op, and cannot rank anything.
2. **Most of the kinds named are written once.** `choir.commitment_record`
   is mint-once, `choir.capture_label` never overwrites a version,
   `choir.texture_revision` is addressed by its content. There is nothing to
   stop rewriting. Of the list, only `choir.run` is written whole on each
   change (`internal/store/graph_store.go:575`, `:713`).
3. **Retirement already runs.** Telemetry and run memory are retired daily
   by tape ops, on by default (`internal/store/telemetry_retire.go:88`,
   `run_memory_retire.go:45`): tool results after 7 days, trace kinds after
   14, a finished run's memory after 7 days except each agent's newest two.
   Freed pages are reclaimed (`compact.go:978`). "Seven retire ops in three
   hours" was the daily job not having run in the window, not disuse. So the
   43.8 GiB store is what remains after retirement, or retirement is not
   working on the paper; revision 1 could not tell which.
4. **Sealed payloads are already compressed.** Each payload over 256 bytes
   is zstd-compressed before it is sealed (`internal/computerevent/privacy.go:335`),
   and ciphertext does not compress again. "Compress per segment" would do
   nothing.
5. **Receipts have more readers than a retry.** Replay reads one receipt row
   per sequence (`internal/platform/event_replay.go:44`), a restore applies
   up to 720,000 tail events, and checkpoints look receipts up by id for
   targets of any age. Segments are not contiguous in sequence.

Claim C72 (more than 35% of tape bytes are rewrites of the five kinds) is
settled false on this evidence.

## Where the redundancy actually is

Read from code by the reviewers and checked at the lines cited; shares are
not measured until the per-op counters exist.

| Duplicate | Where | Why it is a second copy |
| --- | --- | --- |
| The carried conversation tail | `carryActorConversation`, `internal/agentcore/run_memory.go:165` | When an agent starts a new run it copies the previous run's message tail into new run-memory entries, body and all. Run memory is 62% of tape events (census) and about 190 messages per run started. |
| The lifecycle command's stored result | `internal/store/lifecycle.go:2689` | Each lifecycle command embeds the document, the revision (body included), the trajectory and the events it produced, in the same batch that writes those objects. Replay already rebuilds them when the stored result is absent (`lifecycle.go:1306`). |
| A revision's plain text | `internal/types/texture.go:135` | Every revision stores the structured body and the plain text derived from it. |
| The run record | `internal/store/graph_store.go:575` | Written whole about 19 times per run; prompt, result and metadata each time. |
| Head objects that embed a growing list | `choir.story_library_head` (census section 2.3) | About 21 KB rewritten roughly a thousand times a day. |
| Tool output | run-memory message and `tool.result` event | Same content in two kinds; about 5% of tape bytes. |

## Three causes, restated

1. **A byte has more than one home.** The same content is written under
   two kinds, or copied forward from one run to the next. This, not
   whole-object rewriting, is the main source of repeated bytes.
2. **A few mutable objects are written whole.** The run record and head
   objects that embed lists.
3. **Every event pays a fixed toll.** Unchanged from revision 1: about six
   corpusd calls, a receipt row and an envelope per event; the census puts
   the fixed part near 3.5 KB of a 5.7 KB event. Coalescing (landing now)
   is the lever, and it also makes run-memory appends one event per burst
   instead of one per message.

What fills the guest store is still unknown, and is now a sharper question:
what survives a retirement that runs daily.

## The frame

Unchanged, and both seats accepted it:

1. **A byte has one home.** A later op that needs earlier content names it.
2. **The tape records what changed.**
3. **The tape is the one archive and stays whole.** The guest store is a
   working set; a snapshot is a checkpoint of it; receipts are an index.

Two constraints the review added, which every refactor here obeys:

- **Replay applies op bodies as written and stores their content hash
  without recomputing it** (`internal/store/project.go:241`,
  `internal/objectgraph/object.go:97`). So a refactor either leaves the
  body out of the tape and rebuilds it deterministically inside the store's
  transaction, proved equal by a witness test, or it does not change the
  body at all.
- **No content store outside the tape.** A digest that points at the host's
  payload store would be invisible to the tape sweep when it sits inside a
  sealed batch, and sealed payloads are bound to their event. References
  are to things the store already holds by replay: an entry, an object, a
  revision.

## The refactors, in order

Each is a station with its own review and deployed proof. Order is bytes
saved per unit of risk as far as code reading can tell; the per-op counters
(first item) re-rank them before anything red is built.

### R0. Count honestly (yellow, in progress)

Per-op bytes by kind for every op in a batch; writes per object by kind;
run-memory bytes split into carried and new. Land the per-table store
reading and read d@e and the paper. Read the paper's retirement log lines
and watermarks: did the daily job run, and what did it clear.

### R1. Stop copying forward

- **Carry by reference.** A new run's carried conversation names the source
  run and the entry range it carries instead of copying the bodies. The
  reader that builds the model's messages resolves the reference. The
  retirement rule must then keep a source run's entries while a live run
  names them. Red: the conversation an agent sees must be byte-equal to
  today's (the carry code already promises that for the checkpoint
  message), proved by a test that renders both ways.
- **Drop the stored result's duplicates.** Stop embedding the document,
  revision and events in a lifecycle command when the same batch writes
  them; replay's existing rebuild path becomes the only path. Old events
  with a stored result still replay as they do now. Red, small.
- **Tool output has one home** (was R5): the `tool.result` event names the
  run-memory entry.

### R2. Stop writing mutable objects whole

- **The run record.** A patch op for run updates: the changed fields and
  the content hash of the body it applies to. Live write and replay both
  read the stored body inside the transaction, apply the patch, marshal
  through one canonical encoder and must arrive at the same content hash.
  Creation still writes the whole record. Nothing that reads a run changes.
  Before building, measure the prompt's share of the 16 KB and how much of
  the rest is metadata that changes on every write. An alternative with no
  format change, to be weighed against it: write fewer intermediate states.
- **Head objects with lists**: the list becomes its own append-only kind.
- A revision's derived plain text: compute on read. Its id is derived from
  content, so this one needs care with identity and may not be worth it.

### R3. The guest store, from the reading

No design until the per-table reading and the retirement log are read.
Candidates the reviewers named: event kinds no retirement group covers
(for example `model.served`, which publication provenance reads), Texture
revisions (needed locally for history and diff), lifecycle commands
(halved by R1), free pages on a store that has not been compacted since
its split, and `store.event_table` with `store.telemetry_partitions`, which
exist so that deletes do not dirty pages across the whole file and so
re-inflate snapshots. Those two settings are resolved here: ship on or
delete.

Reading retired history from the tape needs an index from a run or
trajectory to tape positions, kept in the guest store; nothing maps one
today. That index comes before any widening of retirement.

### R4. Receipts, with the retained set named

Not "one row per segment". What must stay per event: every row from the
latest two snapshots to the head (a restore replays them), each with its
event kind; the head receipt of every published checkpoint; every
non-batch event (snapshots, effects, verifications, materializations,
rollbacks, key events); idempotency keys while a retry can exist. What can
go: the wide receipt JSON and four of five indexes for old batch events,
replaced by a slim sequence-to-digest row. Expected floor for the paper is
on the order of 1 to 2 GB, not 0.1. Red; after coalescing.

### R5. Compression: measure before promising

Payloads are already compressed one at a time. A shared dictionary helps
only small plaintexts inside one batch. After coalescing changes what a
batch holds, measure the share of payload bytes in plaintexts under a few
kilobytes; build only if it is large.

## What this does not do

- It does not move tape off Node B or drop any segment.
- It does not sample or lose telemetry (owner ruling D8).
- It does not add a setting.
- It does not rewrite existing tape.

## Expected end state

Revision 1 stated four numbers. The review showed none was supported:
the tape figure leaned on the wrong kinds, the store figure had no reading
behind it, the snapshot figure is a consequence of row counts as much as
bytes (the witness pass alone took 22 to 35 minutes on a 21 GiB store),
and the receipts figure ignored the restore window. They are withdrawn.
Each station precommits its own number in the calibration ledger from the
honest counters, before it is built. What can be said now: coalescing alone
should take the tape from 3.6 to about 2 GB a day (claim C68 covers its
event count); carry by reference is the largest remaining lever by the
census's event shares.

## Order of work

1. Land coalescing, the updater prune and the per-table reading.
2. R0: honest counters, the store reading on both computers, the
   retirement log.
3. Re-rank from R0, re-review this design, then R1 (carry by reference
   first), on d@e with a full replay before the paper.
4. R2, R3 and R4 in the order the numbers give.

## Open questions for the owner (defaults taken)

- Reading retired history from the tape makes old traces slower to open.
  Default: accept.
- Old events are read as they were written, forever. Default: yes, no
  rewrite of existing tape.
