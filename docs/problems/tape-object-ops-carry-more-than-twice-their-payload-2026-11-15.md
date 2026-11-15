# Tape object ops carry more than twice their payload

Date: 2026-11-15. Class: green (a read-only record; no code, no tests).
Form: [docs/memo-problem-documentation-first.md](../memo-problem-documentation-first.md).
Station: the store-once goal, lane `event-envelope-record`.

## Problem

On the largest computer, about five hours after boot, the `telemetry_batching`
counters disagree by a factor the code was not expected to produce:

- `event_payload_bytes_by_kind` sums to 13.65 MB over 9,289 events (mean about
  1,469 bytes). `tool.result` is 9.44 MB of it (1,985 events, mean 4,754),
  `loop.progress` 1.70 MB, `desk.cell` 1.67 MB.
- `tape_op_bytes_by_object_kind` for `choir.event` is 31.64 MB over 9,893 ops
  (mean 3,198 bytes), each written once (`object_rewrites`). That is 2.3 times
  the payload.
- In the same window the tape took 182.7 MB (`tape_bytes`), of which object ops
  were 120.3 MB and `choir.run` 39.0 MB.

Question: what is the encoding that turns a payload into an op more than twice
its size, does it apply to every object kind, and how much of it survives on
disk. A second reading of the earlier belief ("Body is `[]byte`, so it is
base64 in the op") is checked below.

## Evidence

All of this is read from code at the commit this branch was cut from. Nothing
was run, compiled or measured on a live computer.

### 1. What each counter measures

- `event_payload_bytes_by_kind` is `len(rec.Payload)`, the event's payload
  alone, taken at the top of `AppendEvent`
  (`internal/store/store.go:1415`, `recordAppend` at
  `internal/store/telemetry_stats.go:146-165`). It is counted before the event
  is sequenced, stamped or encoded, and before the object envelope exists.
  Events written through other paths are not in it (see "count mismatch" below).
- `tape_op_bytes_by_object_kind` and `object_rewrites` count `len(op.Body)` of
  each `object_recorded` op, by the kind parsed from the op's canonical id
  (`internal/store/tape_write_counters.go:151-154`; bytes bumped at lines 230 and
  236 of the same file; `addRewrite` at
  `internal/store/object_rewrite_counters.go:128-148`). `op.Body` is the whole
  marshalled object, not a payload. Counted when the append has returned, so
  before sealing and compression.
- `tape_bytes` and `tape_bytes_by_family` are `len(payload)` of the marshalled
  `ProjectionBatch`, passed to `recordTapeEvent` at
  `internal/store/projection_tape.go:266`. That is the plaintext batch JSON
  handed to the appender (lines 256 and 258), before redaction, compression,
  encryption and the envelope. The family is the family of the event's first op
  (`tape_write_counters.go:62-68` comment).

So none of the three numbers is post-compression, and the 2.3 ratio compares a
payload-only count with a whole-op count.

### 2. How a `choir.event` op is encoded

Every writer of a `choir.event` object builds it the same way and marshals it
with `encoding/json`:

- `buildEventObject` (`internal/store/graph_store.go:1257-1291`) serialises the
  whole `types.EventRecord` as the object Body (`json.Marshal(rec)`, line 1274),
  builds a separate Metadata map from the same record, hashes both
  (`ContentHash`, line 1281), and fills the object.
- The three op builders then do `json.Marshal(obj)` on the object:
  `eventObjectOp` (`internal/store/event_table.go:645-656`, used by the stamped
  acknowledged write at `event_table.go:684-694` and by the telemetry sealer at
  `internal/store/telemetry_sealer.go:126-139`) and the generic `interceptOG`
  (`internal/store/projection_tape.go:105-116`) for everything else, including
  the lifecycle path that folds a `req.Event` into a batch
  (`internal/store/lifecycle.go:3513-3519`). The result is `ProjectionOp.Body`,
  a `json.RawMessage` (`internal/computerevent/projection_batch.go:30-35`).
- Because `Body` of the op is `json.RawMessage`, the op does not base64 or
  string-escape the object: the object JSON sits inline in the batch. The
  earlier reading ("Body is `[]byte` in the op") is a half truth: the op's Body
  is raw JSON, but the object's own Body is `[]byte`.
- `objectgraph.Object` (`internal/objectgraph/object.go:23-36`) has
  `Body []byte` (`json:"body,omitempty"`) and `Metadata json.RawMessage`.
  `encoding/json` encodes `[]byte` as base64. So the event record JSON is
  base64'd inside the object, a one third growth (4 output bytes per 3 input
  bytes) of everything in it, payload included. Metadata is raw JSON and is
  not inflated.
- `ProjectionBatch` is then marshalled (`projection_tape.go:212-221`) and that
  plaintext is what `tape_bytes` counts.

Fields that sit in an op, per `choir.event`:

| Part | Where | Encoding |
| --- | --- | --- |
| Object envelope: `canonical_id`, `object_kind`, `owner_id`, `content_hash`, `created_at`, `updated_at` (and `computer_id`, `version_id`, `tombstone`, `superseded_by` when set) | `object.go:23-36` | plain JSON |
| Op wrapper: `kind`, `canonical_id` again, `table` when stamped | `projection_batch.go:30-35` | plain JSON; the canonical id appears twice (op and object) |
| `body` | the whole `EventRecord` JSON: `event_id`, `seq`, `stream_seq`, `ts`, `loop_id`, `agent_id`, `channel_id`, `owner_id`, `trajectory_id`, `kind`, `phase`, `payload` (`internal/types/task.go:422-463`) | base64 of that JSON |
| `metadata` | `event_id`, `seq`, `stream_seq`, `run_id`, `agent_id`, `channel_id`, `trajectory_id`, `kind`, `phase`, `timestamp` (`graph_store.go:1258-1269`) | plain JSON |

Duplication: ten of the twelve event-record fields are written again in
metadata (event id, seq, stream seq, run, agent, channel, trajectory, kind,
phase, timestamp), and `owner_id` is in the envelope and in the record. The
canonical id is derived from the content hash, and the content hash is in the
object too. Nothing else is duplicated by the encoder itself. The
`content_hash` is a function of body and metadata
(`object.go:97-110`), so it is derivable but stored.

### 3. The arithmetic for the mean event (estimate)

The byte sizes of the fixed fields below are my estimates from field names and
typical id shapes, not measured. The payload mean is a counter; the op mean is
a counter; the rest is reconstruction.

For the all-kinds mean (payload 1,469; op 3,198):

| Part | Bytes (estimate) |
| --- | --- |
| payload, as counted | 1,469 |
| event record fields around the payload (ids, `seq`, `ts`, keys, quotes) | about 400 |
| record JSON before base64 | about 1,870 |
| after base64 (times 4/3) | about 2,490 |
| metadata (same ten fields again) | about 380 |
| object envelope and op wrapper (ids, hash, two timestamps, keys) | about 330 |
| op total | about 3,200 |

The reconstruction lands on the observed 3,198, but it was solved for with one
free number (the 400); it shows that base64 plus a fixed per-op cost of about
700 bytes is enough to explain the figure, not that these are the true parts.
Of the 1,730 bytes of excess over the payload, on this estimate about 620 is
base64 growth, about 400 the event-record wrapper, about 380 the metadata copy
and about 330 the object and op envelope. The fixed part (about 1,100) weighs
most on small events, which is why the all-kinds mean ratio is 2.2.

For the mean `tool.result` (payload 4,754) the same parts give about 4,754 +
400 = 5,150, base64 about 6,870, plus about 710 fixed, about 7,600 bytes, a
ratio of about 1.6. That is a prediction, not a reading: the per-kind op byte
line for `tool.result` is not in the figures given. (The 31.64 MB over 9,893
ops does not split by event kind in `tape_op_bytes_by_object_kind`, which is
keyed by object kind; a per-event-kind op size does not exist as a counter.)

Count mismatch: 9,893 `choir.event` ops against 9,289 counted events. The
payload counter lives only in `AppendEvent` (the single `recordAppend` call,
`store.go:1415`). `buildEventObject` has a second consumer that does not pass
through it: the lifecycle path at `lifecycle.go:3513`, which takes an event
that the caller built. Whether that is the 604 extra ops was not verified;
it is the only code path I found that writes a `choir.event` without
`recordAppend`. Per op rather than per sum the ratio is about 2.18, not 2.32.

### 4. Compression and sealing, and what reaches the disk

- The tape batch is marshalled to `payload` (`projection_tape.go:212`) and, when
  a cipher is bound, handed to `AppendNewPrivatePayload`
  (`projection_tape.go:234-256`; `internal/computerevent/appender.go:719-733`),
  else to `AppendNewPayload` (`projection_tape.go:258`,
  `appender.go:628`).
- Sealed path (`internal/computerevent/privacy.go:310-350`): redaction of the
  plaintext, then zstd (`SpeedDefault`) of the redacted plaintext when it is
  over 256 bytes and saves at least 5%
  (`privacy.go:47-50, 75-84, 334-336`), then XChaCha20-Poly1305, then the
  ciphertext is base64'd again into the envelope JSON (`privacy.go:349`). The
  envelope metadata names the codec (`privacy.go:34-39`). So one batch is
  compressed as a unit, not per op, and a final base64 adds a third to the
  compressed size on disk.
- I found compression only in `privacy.go`; the unsealed `AppendNewPayload`
  path had no compression at any line I read. Whether production computers
  always have a cipher bound (so always take the sealed path) was not
  verified.
- So the counters (`tape_bytes`, per-kind and per-family bytes) are all
  pre-compression and pre-seal. What survives on disk of the encoding
  overhead cannot be derived from code: zstd removes much of the repeated
  metadata keys and ids, and re-compresses base64 text to roughly its 6 bits
  per character (so most of the base64 growth is recoverable in principle),
  but the ratio depends on the data. Then the envelope base64 adds about a
  third back. The on-disk size of a batch is unknown without measuring it.

### 5. Does the overhead apply to every object kind

- The base64 growth of the object's Body applies to every object written
  through `interceptOG` or `eventObjectOp`, because they all `json.Marshal` an
  `objectgraph.Object` (`projection_tape.go:105-116`, `event_table.go:645`).
  Edges (`object_edge_recorded`) are marshalled from `objectgraph.Edge`, which
  has no `[]byte` field (`object.go:38-46`), so they have no base64.
- The metadata copy is per kind: kind-specific. `lifecycleMetadata` and the
  other builders write their own maps (for example `lifecycle.go:3500`), so the
  duplication of record fields in metadata is not a fixed ten fields for every
  kind; it was read in full only for `choir.event`.
- Run-memory entries are a different op kind (`run_memory_entry_recorded`),
  not an object op: the message is a string field
  (`RunMemoryEntryProjection.MessageJSON string`, `projection_batch.go:49`;
  built at `internal/store/run_memory.go:239-249`), so it is JSON-string
  escaped (every quote and backslash gains a backslash), not base64'd. Their
  bytes are in `run_memory_bytes_by_kind`, not in the object op counter.
- If the whole 120.3 MB of object op bytes were encoded like an event, the
  base64 share would be about a quarter of an op when the envelope and
  metadata are a quarter of it: one third of the Body, where Body is the
  rest. For an envelope plus metadata fraction between 10% and 40% of an
  op, the base64 growth is between 22% and 15% of the op (the Body is
  3/4 of the op after base64 growth for a 25% fixed part). On 120.3 MB that is
  about 18 to 27 MB, 10% to 15% of 182.7 MB. This is an estimate with
  an assumed range, not a reading; the true share depends on each kind's
  Body fraction, which the counters do not give. It excludes the metadata and
  envelope duplication, which could be of similar size.

### 6. Is a tool result written twice on the tape

Yes, in two different ops, from two different writers, as the same content
in different encodings:

- Writer 1, the event: `internal/toolregistry/batch_executor.go:103-148`. The
  `tool.result` payload carries `output` (the capped `visibleOutput`),
  `output_len`, the call's `arguments`, and, when the output has a projection,
  also `full_output` (the durable, uncapped form), `full_output_sha256` and
  `full_output_len` (lines 113-137). So inside one event the model-visible
  output and the durable output can both be present: for a projected output
  that is a second copy within the payload itself.
- Writer 2, run memory: the same `ToolResult.Output` (that `visibleOutput`,
  returned at `batch_executor.go:150`) goes into the next provider message by
  `buildToolResultContent` (`internal/toolregistry/toolloop.go:2328-2345`,
  `"content": result.Output`). The loop appends that message through
  `appendMessageWithContext` (`toolloop.go:723-724`) to
  `runMemoryManager.afterAppendMessage` -> `appendMessage`
  (`internal/agentcore/run_memory.go:357-361, 416-422`) -> `AppendRunMemoryEntry`
  -> `appendRunMemoryTapeLocked` (`internal/store/run_memory.go:215-262`), which
  writes the message as `MessageJSON`.
- So the visible tool output exists in the `tool.result` payload (once, as
  `output`, escaped as a JSON string inside the payload, then base64'd inside
  the object) and in the run-memory entry (as a JSON string inside the message,
  which is itself a JSON string in `message_json`: escaped twice). They are the
  same characters, not the same bytes on the tape: encodings differ, and the
  event is counted among the object ops, the entry among the run-memory ops.
  The run memory message also carries other content (the tool-use id and the
  type wrapper) and is sanitised (`sanitizeStoreText`, `run_memory.go:150`);
  whether sanitising alters a given output was not read.
- This is the code-level answer. The share of run memory in bytes that is a
  tool output was not measured here.

### 7. Old binaries and differently encoded ops

- Projection batches carry `version` and `projector_version`; `Validate`
  accepts exactly V1/V1 and V2/V2 and refuses anything else as "unsupported
  version" (`projection_batch.go:366-369`). It refuses an op `kind` not
  in its list as "unknown kind" (`projection_batch.go:386`), so a new op
  kind is a refusal on an old binary (the batch fails; for a sealed batch on
  replay this is a poison-event path, see `ErrProjectionPoison`,
  `projection_batch.go:50`).
- A new field on an existing op is not refused: `DecodeProjectionBatch` uses
  plain `json.Unmarshal`, with no unknown-field refusal
  (`projection_batch.go:414-423`). The stamp `table` was introduced that way;
  `internal/store/event_table_old_binary_test.go:15-50` records that an
  earlier binary passes `Validate` on a stamped object op, ignores the stamp,
  and upserts the row into `og_objects`, which is why that deploy became a
  rollback floor.
- The "stored result" lifecycle command form (`lifecycle_command_forms`) uses
  an additive, optional field with no version: `RevisionID` on
  `LifecycleStoredResult` (`internal/types/lifecycle.go:661-680`). Its comment
  says a binary that predates the field ignores it and reads `Revision` as
  absent, so a writer may land only after a release that reads it is the
  deployed one. The reader prefers the embedded copy and else reads the
  revision by id (`internal/store/lifecycle.go:1361-1373`); the writer counts
  every command's form (`lifecycle.go:1384-1388`, `2760-2780`).
- The compact receipt format has numbered formats (`compactFormatV1`,
  `compactFormatJSONB`, `internal/computerevent/receipt_compact.go:46-51`),
  with a write allowance for format 2 (`receipt_compact.go:88-92`). I read only the constants,
  not the read-side refusal; what an older binary does with a format 2
  receipt is not verified here.
- Result: there is no per-op encoding version and no upcast path for an object
  op. The object Body encoding (`[]byte` as base64) is implicit in the
  `objectgraph.Object` struct tag, and every reader (`decodeChoirEventRow`
  `json.Unmarshal`s `obj.Body` into `EventRecord`, `event_table.go:101`)
  assumes it. What an old binary would do with an op whose object Body was
  encoded another way is: `json.Unmarshal` into `[]byte` of a non-base64
  string fails, and `decodeChoirEventRow` refuses with a decode error
  (`event_table.go:101-102`); for other kinds the reaction was not read.

## Belief

Held, with confidence from code reading only:

1. The 2.3 ratio is an encoding effect, not a double write of events. Each
   `choir.event` is one op (`object_rewrites` says written once). The op is the
   whole object: base64 of the event record JSON (one third growth on the
   payload and its record fields), a plain copy of ten of those fields in
   metadata, and the object and op envelope. High confidence in the mechanism,
   high that base64 is on the event record (the struct tag is explicit); medium
   on the split of the 3,198 bytes, since the fixed field sizes are estimates.
2. The earlier reading is correct in effect (base64 of a Body) but names the
   wrong layer: it is `objectgraph.Object.Body`, not `ProjectionOp.Body`. The
   op's own Body is raw JSON.
3. All three byte counters are pre-compression and pre-seal. The 182.7 MB is
   not the disk figure; the disk figure is zstd over whole batches, then
   base64'd again for the envelope. How much of the encoding overhead survives
   is unknown (low confidence on any number).
4. The base64 growth applies to every object kind that goes through
   `json.Marshal(objectgraph.Object)`, so to most of the 120.3 MB; my estimate
   is 10% to 15% of 182.7 MB for the base64 growth alone. Metadata duplication
   and envelope are on top and kind-specific.
5. A tool result's visible output is on the tape twice in the sense of two
   independent writers (the `tool.result` event and the run-memory message),
   the same characters in different escaping; and in the event it may be
   present a third time as `full_output` when a projection exists. Medium
   confidence: the last is read only from the writer, not from a tape.
6. No object op has a version or an upcast path; a change to the encoding is
   not versioned by anything in the op. The two precedents (the `table` stamp
   and `RevisionID`) are additive fields that old binaries ignore, with a
   deploy-order rule written in comments and a test.

## Remaining error

- The sizes of the fixed fields (event record wrapper about 400, metadata about
  380, envelope about 330) are estimates. The sum was made to fit the observed
  mean, so the fit is not evidence for the parts.
- The 604 extra `choir.event` ops over counted events are unexplained; the
  lifecycle fold at `lifecycle.go:3513` is a candidate, not a finding.
- `json.Marshal` of a `json.RawMessage` compacts it and escapes `<`, `>`, `&`
  as unicode escapes. The payload counter counts `len(rec.Payload)` before
  that, so payload and body bytes can differ by a small amount that was not
  estimated.
- What a production computer's sealer state is (the sealed path with zstd, or
  the unsealed path with no compression I could find) was not read.
- The on-disk size and the compression ratio of event, run, run-memory and
  other batches are unknown.
- The metadata and Body shapes of the other object kinds (`choir.run` 39.0 MB
  among them) were not read; the share applied to them is a bound, not a
  reading.
- Whether the run-memory message of a tool result is byte-for-byte the same
  text as the event's `output` (sanitising, the structure around it) is
  unverified, and so is the share of run memory that is tool output.
- What an older binary does with the `table` stamp is a test result recorded in
  the repo, which I read but did not run; the format-2 receipt behaviour was
  not read.

### What would need measuring

Each is read-only. None changes a write path.

1. Split of the mean op (settles the 3,198): decode the tape's last N
   `choir.event` ops from a copy of the tape (or log in the counter the four
   lengths while the op is built: `len(obj.Body)`, `len(obj.Metadata)`, the op
   length, and `len(rec.Payload)`) and sum them by event kind. This gives the
   per-event-kind op size that does not exist today (`tape_op_bytes_by_object_kind`
   is by object kind only) and replaces every estimate in the table above.
2. The 604 extra ops: count `choir.event` ops in the same window by writer
   (acknowledged, stamped, sealer, lifecycle fold) or by checking whether
   the lifecycle path's events are in `events_appended_by_kind`.
3. Disk survival: for a sample of sealed batches, record the plaintext length
   (`PlaintextLength` is already in the envelope metadata,
   `privacy.go:28-40`) and the stored envelope size, by authority family.
   `PlaintextLength` over the stored size is the on-disk ratio; no new code
   path is needed beyond reading the tape files. Also count batches with an
   empty `compression` to see how many fail the 256-byte or 5% test.
4. Share for every kind: for each object kind, the mean of
   (`len(obj.Body)`, `len(obj.Metadata)`, op length) from the same sample as
   item 1, which gives the base64 share (`len(Body)/3`) exactly instead of
   the 10% to 15% bound.
5. Tool result twice: for a sample of tool calls, compare the `output` field
   of the `tool.result` payload with the `content` of the run-memory message of
   the same `call_id`, and count how many have `full_output` as well. Sum the
   bytes of each by field.
6. Production sealer state: read, on the largest computer, whether the
   projection tape has a cipher bound (the `sealer` at
   `projection_tape.go:84`), or count batches with and without a
   `compression` field.

## Measurement added 2026-11-15T18:48Z: base64 does not compress away

A local bench on real data, not on the tape. Input: the 20 newest run
records of the test computer as its run API returned them at about 15:00Z
(mean 23,414 bytes, read-only). Each record was wrapped in a synthetic
object op two ways, with the body as base64 (today's encoding of
`objectgraph.Object.Body`) and with the body as inline JSON, then
compressed with `zstd -3`, alone and in batches of five.

| | plaintext, base64 body | plaintext, inline body | zstd, base64 body | zstd, inline body |
| --- | --- | --- | --- | --- |
| one op a batch | 627,648 | 471,490 | 195,463 | 110,158 |
| five ops a batch | 627,632 | 471,474 | 138,111 | 72,038 |

Plaintext grows by 1.33, as expected. After compression the base64 form
is **1.77 to 1.92 times** the inline form: base64 moves JSON off its byte
boundaries, so the compressor finds fewer repeats. The sealed envelope's
own base64 multiplies both sides equally.

What this does and does not show. It shows the encoding cost survives
compression and grows. It does not measure the tape: the envelope is
synthetic, the compression level and the sealed path's unit are taken from
the code reading above, the records are run records of a Texture-heavy
test computer that still carried the source-entities list, and other
object kinds may compress differently. If object ops are two thirds of
tape bytes and the ratio held for all of them, sealed tape bytes would
fall by something like 30%; that number is an extrapolation, not a
reading.

Remaining error: the same bench on real op bodies of each large kind
(`choir.event`, `choir.run`, `choir.texture_document`), which needs
either a host-side read of sealed segments or a guest-side counter of
compressed bytes per batch with and without the encoding. Neither exists.
