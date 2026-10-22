# Trace lines leave the tape and the store (R7): design (2026-10-22)

Date: 2026-10-22. Station: FP1, step R7 of
[`paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22`](../problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md),
section "Root cause, second investigation". Lane
`lane/fp1-trace-segment-design`, from `origin/lane/fp1-journal-root-cause`
`73718ec65`. Every `file:line` below is on that checkout unless it names
another ref.

Mutation class of this document: green (docs only; no code, nothing run on
any computer, no SQL against any store). Mutation class of the change it
designs: **red** (Trace/evidence, the run event read path, sequence
allocation, run acceptance evidence, the tape write path). Section 9 gives
each slice its class.

Evidence labels, as in the problem record: **[V]** verified by reading the
code at the cited `file:line`; **[M]** measured, in a cited record, not
re-measured here; **[H]** hypothesis or derivation, with the observation
that settles it.

Owner request (2026-10-22): "we can't continue with the paper writing a
gigabyte every 10 to 20 minutes. We gotta investigate, get to the bottom of
that, and resolve it, re-architect whatever we need to do." And: "Anything
which improves resource utilization on a forward and ongoing basis should
be prioritized over stability, availability, or uptime in the short term."

## Answer in brief

1. **Where trace lines live.** In an append-only segment log on the
   guest's persistent disk, beside the store workspace and never inside it:
   `<store dir>/trace-log/`, one segment per hour or 64 MiB. It is a new
   package, `internal/tracelog`, that copies the fail-stop write rules of
   the host segment writer (`internal/tapestore/appender_failstop_test.go`
   S1 to S9) but not its digest index. Trace lines stop being tape events,
   Dolt rows and host files. The host tape segments and a second Dolt
   database were considered and rejected (section 3).
2. **What moves.** Nine of the ten batched kinds: `loop.delta`,
   `loop.progress`, `loop.retry`, `loop.compaction.started`, `tool.invoked`,
   `tool.result`, `desk.cell`, `desk.egress` and
   `texture.agent_revision.progress`. `loop.compaction.completed` and all
   32 acknowledged kinds stay on the tape. `tool.result` is evidence for run
   acceptance, so acceptance copies each line it cites into its own
   record, which is a tape write (section 2). Keeping `tool.result` on the
   tape instead is open decision O2.
3. **What a crash may lose.** If the process dies, nothing is lost, because
   each line is written to the page cache before `AppendEvent` returns. If
   the VM or the kernel dies, the lines since the last fsync are lost: at
   most about 1 s. The fsync runs once a second and before every
   acknowledged tape append, so a loss is always the newest lines of a run,
   never a gap before a later tape event. This is the owner's ruling for
   batching ("a crash may lose up to the batch window of telemetry", owner
   notes §27 item 18), applied to a different sink. Taking trace lines off
   the signed tape is not ruled yet: open decision O1.
4. **Witness, replay, restore.** The log is outside the Dolt workspace, so
   it is outside the content witness, replay eligibility, the schema epoch
   and the replay manifest. None of those change. Live and replay agree,
   because neither has the new lines in Dolt. A restore swaps the workspace
   and leaves the log alone. The log then records a restore boundary, and
   its high-water marks keep `seq` and `stream_seq` from being reused. Once
   an hour, a sealed segment's digest goes on the tape as one small object,
   so a segment is tamper-evident while it is kept, and the fact that it
   existed outlives its deletion.
5. **Readers.** No route, cursor or response shape changes. Each store
   event-list method adds a log half and merges it with what it reads today,
   deduplicating by `event_id`. Old `choir.event` rows stay where they are,
   in `og_objects` and, after F-c deploy 2, in `choir_events`. The CLI, the
   frontend, Playwright helpers, scripts, Texture and acceptance all read
   through those methods or the live bus, so none of them changes.
6. **Retention.** Whole sealed segments are deleted, oldest first, once
   they pass 7 days or the log passes its byte cap: the smaller of 2 GiB
   and 10% of the persistent disk. Nothing is compacted and nothing is
   rewritten. Deleting a segment is not a tape act, because trace lines are
   not state.
7. **Numbers on the paper** **[H]**, from the problem record's model
   (1,810 KiB/s total). After R1 to R4, R7 saves **about 411 KiB/s**: 2.57
   trace lines/s at 160 KiB each. After F-c deploy 2 it saves about
   213 KiB/s. Either way the journal ends at **about 824 KiB/s (483 MiB per
   10 minutes)**, against 1,060 MiB today and 724 MiB after R1 to R4. A
   1 GiB GC pass then comes every 21 minutes of journal, which the 5-minute
   tick turns into a pass every 25 minutes, against every 10 minutes today.
   The GC-caused 503 bursts become about 2.5 times rarer. The tape loses
   2.57 of about 7 events/s, which also means about 2.2 GB/day less
   permanent Dolt history and about 10 fewer host records per second.
   R7 does not end the 503s. The bounded `/health` (lane
   `fp1-health-bounded`) and a GC that does not hold the write connection
   do that.
8. **Rollout.** Seven slices, each landable alone (section 9). The first
   three are inert: counters, the library, then the readers, seeds and
   evidence copy. Then the writer behind a per-computer setting,
   `store.trace_log` (default off), on one test computer. Then the paper.
   Then compression. Turning the setting off is the live rollback. The
   readers keep reading the log whatever the setting says, so nothing is
   hidden. There is no schema epoch bump and no hard rollback floor
   (section 8).

## 0. The problem, and what changed since F-c

The second investigation measured the paper at 14.0 tape ops/s in 10.8 tape
events/s **[M]** (problem record §1). Of these, 6.02/s are standalone
`choir.event` appends, and 5.70/s of those are the ten batched trace kinds.
Each one costs about 160 KiB of journal on a 100,000-row store (§2,
local probe), so trace lines are about 912 KiB/s of the 1,810 KiB/s
model, close to 53% of it. The cost is per op, not per byte: one path in
each of the 13 prolly trees of `og_objects`, plus about 47 KiB of prepare
and finalize per tape event.

F-c (`docs/design/choir-event-table-f-c-2026-10-22.md`) moves the same ten
kinds into `choir_events`, with 4 trees. The owner accepted its defaults
at 13:20 today (`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md`,
commit `0bf8a097d`) **[V]**. Deploy 1 is built on `lane/fp1-event-table`
and deploy 2 on `lane/fp1-event-table-d2`. Neither is on `main` yet
**[V]** (`git log origin/main..origin/lane/fp1-event-table-d2`: 38
commits). Its local probe measured 142.5 KiB to 74.4 KiB per unbatched
event, a ratio of **0.52** (`lane/fp1-event-table-d2`, F-c design, F17
table) **[M]**. So F-c roughly halves a trace line's cost. It does not
remove the tape event, the host records, the permanent history or the
witness weight, and it gives no way to delete old lines.

F-c §7 declined "no telemetry in Dolt at all" for three reasons. This
design answers each one:

| F-c §7 objection | Answer here |
|---|---|
| "The trace and acceptance would fate-share with corpusd" (reading from the tape) | The log is guest-local. It is read in-process by the same `Store` methods, with no host hop (section 3) |
| "A local log is new state with no replay, witness or restore story" | Trace lines are not state: they are reclassified the way the ontology's ledger split already classifies the actor recovery log (section 7.1). Replay and witness need nothing, because the log is outside them. Restore is in section 7.3 |
| "Acceptance cites events by id and `stream_seq` as evidence" | Acceptance copies each cited line into its own tape record (section 2.3). Each sealed segment's digest goes on the tape (section 4.5) |

R1 to R4 are being built on `lane/fp1-journal-cuts`, which is not pushed
yet **[V]**: `git log origin/lane/fp1-journal-cuts` finds nothing. This
design assumes they land as the problem record specifies them:

- **R1.** `provider_call_started` is gone, and
  `texture.agent_revision.progress` goes on the bus only.
- **R2.** No `inbox_cursor` entry is written when the cursor did not move.
- **R3.** `tool.invoked` is folded into `tool.result`, or goes live-only.
- **R4.** `desk.egress` is folded into `desk.cell`.

R7 classifies by kind. So whatever kinds R1 to R4 leave are the kinds the
log takes, and the design is the same whether or not each cut lands.

## 1. Owner rulings this design rests on (standing question 1)

| Ruling | Source | Settled by | What it settles here |
|---|---|---|---|
| "Batch run telemetry. Telemetry events may be sealed in small batches instead of one acknowledged write each; a crash may lose up to the batch window of telemetry. The window form goes through a panel." | `docs/reports/owner-notes-2026-10-14.md:618-620` (§27 item 18, 2026-10-20) | owner | Trace lines are loss-tolerant up to a window. The window here is the 1 s group fsync, the same as the sealer's default `W` (`internal/store/telemetry_sealer.go:52`) |
| "Anything which improves resource utilization on a forward and ongoing basis should be prioritized over stability, availability, or uptime in the short term." | owner request, 2026-10-22 (this station's brief) | owner | The ordering: R7 is worth a red change, and its short-term risk is acceptable |
| "we should have a better api to get full multidesk/multiagent traces"; "evaluate the traces, looking for failed tool calls" | `owner-notes-2026-10-14.md:970-976` (§35) | owner | Trace lines must be kept for days, not minutes, and be readable by trajectory. So the log is not a memory ring (section 3) |
| F-c's ten defaults accepted | ledger, 13:20, 2026-10-22 | owner | F-c goes ahead. R7 composes with it (section 6.2) |
| "Track spending, no hard cap" | `owner-notes-2026-10-14.md` §40, DEC-07 | owner | Cost lines (`loop.progress`) must stay readable. They do, in the log, for the retention window. A spend total that lasts longer is open decision O5 |
| Trace lines leave the signed tape | **none** | — | **Not settled.** Ruling 18 allows losing a window; it does not move telemetry off the tape. This is O1. The design proposes it; until the owner rules, R7's writer (slice S4) does not go past a test computer |

Doctrine that applies **[V]**:

- `C7` (`docs/choir-doctrine.md:170-176`). Anything needed for settlement
  or rewarm "becomes durable workstream state, not only code, transcript,
  or Trace". So settlement may not depend on trace lines, and today it does
  not: the commitment ledger's supervision views read commitment records,
  not events (`internal/types/commitment_views.go:528`,
  `internal/textureowner/texture_evidence_sources.go:787`).
- `I13` (`:310-315`). "Trace is an evidence substrate."
- `I23` (`:370-379`). No actor may hide "the existence of inconvenient
  evidence".

Those two are why sealed-segment digests go on the tape (section 4.5), and
why acceptance copies what it cites (section 2.3).

The ontology's ledger split (`docs/computer-ontology.md:168-183`) already
has a guest-local, non-Dolt, non-tape ledger: "Actor recovery log (narrow
SQLite) ... never semantic event, trajectory, acceptance, or promotion
truth". The trace log is a second row of that shape (section 7.1). It is
not a third Dolt store, so the two-store taxonomy (`:135-165`) is not
touched.

## 2. Every kind, classified

### 2.1 The ten batched kinds

Rates are the paper's, from the problem record §1 **[M]**. "After R1 to
R4" assumes those cuts land as specified.

| Kind | Rate today | After R1 to R4 | Durable readers ([V], file:line) | Evidence that acceptance or the ledger depends on? | R7 class |
|---|---|---|---|---|---|
| `loop.delta` | about 0 (not separated in the window) | same | run-memory compaction folds it (`internal/agentcore/run_memory.go:1163`), on a path no production caller reaches (F-c §1.2) | no | **trace log** |
| `loop.progress`, phase `provider_call_started` | 0.9/s | **0** (R1) | none (problem record §1) | no | trace log (moot after R1) |
| `loop.progress`, phase `tool_loop`, `jev_judgment`, `execution` | 0.9/s | 0.9/s | `/api/costs` through `llmcost` (`internal/llmcost/aggregator.go:69`, `internal/apihandler/costs.go:74`, `:96-97`), an estimate over the newest 2,000 to 10,000 events | no. Cost is an estimate (`costs.go:95-102`, telemetry-batching design §3.1) | **trace log**. A lasting spend total is O5 |
| `loop.retry` | small | small | trace export only | no | **trace log** |
| `loop.compaction.started` | small | small | trace export only | no | **trace log** |
| `loop.compaction.completed` | a few per run | same | acceptance checkpoint "compacted" (`internal/agentcore/run_acceptance.go:475-478`) | **yes**, acceptance | **tape** (stays batchable, so F-c and the sealer still apply). Low volume, so no reason to move it |
| `tool.invoked` | 0.84/s | **0** (R3) | the frontend ticker, live only (`frontend/src/lib/PromptSurface.svelte:157`), and trace export | no | trace log (moot after R3) |
| `tool.result` | 0.83/s | 0.83/s, larger if R3 folds the arguments in | run acceptance (`run_acceptance.go:154`, `:284`; `trace_helpers.go:102`); the Texture workflow verifier (`internal/textureowner/texture_workflow_verifier.go:345`, `:378`, `:397`), which **has no production caller**; Playwright `frontend/tests/helpers/run-trace.js:11-40`; scripts (section 6.1) | **yes**, acceptance | **trace log, with the cite copy (2.3)**; O2 keeps it on the tape instead |
| `desk.cell` | 0.84/s | 0.84/s (R4 adds an `egress[]` array) | `cmd/choir/trace.go:363`, `scripts/management_latency_probe.py:295`, `scripts/reeng_ttl_session.mjs:101` | no, diagnostic | **trace log** |
| `desk.egress` | 0.80/s | **0** (R4) | `cmd/choir/trace.go:364`, `scripts/reeng_tracer.mjs:436` | no | trace log (moot after R4) |
| `texture.agent_revision.progress` | 0.59/s | **0** durable (R1, bus only) | the live bus only (`internal/textureowner/texture.go:1698`, `:1766-1806`) | no | trace log (moot after R1) |

So after R1 to R4 the log takes 0.9 + 0.83 + 0.84 = **2.57 lines/s** on the
paper. Without R1 to R4 it takes 5.70/s **[H]**, from the rates above.

### 2.2 The 32 acknowledged kinds: all stay on the tape

Run state (`loop.submitted`, `.started`, `.completed`, `.failed`,
`.blocked`, `.cancelled`, `activation.passivated`), `runtime.health` and
`runtime.degraded`, `channel.message`, `system1.judgment` (a paid fact
replayed from the tape; read by `system1_verb.go:176` as a replay cache),
the email and browser acts, the five Texture canonical-revision and
decision kinds, and the eight product-state kinds
(`internal/types/event_durability.go:47-97`) **[V]**. None moves. They are
0.33/s on the paper, about 53 KiB/s, and they are the events a boundary,
an editor, the lifecycle fold or an owner reads. The lifecycle fold
(`internal/store/lifecycle.go:3328-3336`) builds its event object inside a
conditional batch and never calls `AppendEvent`, so it cannot reach the
log by construction **[V]**.

**The rule, in code.** F-c adds a closed storage class,
`types.EventStorage`, with `StorageObjectGraph` and `StorageEventTable`
(`internal/types/event_storage.go` on `lane/fp1-event-table-d2`) **[V]**.
R7 adds a third value, `StorageTraceLog`, for the nine kinds above. It
copies the discipline of the batching class
(`event_durability.go:23-30`; telemetry-batching design §3.4):

- a parser test fails if any `EventKind` has no storage entry;
- the default for an unnamed kind is the object graph;
- the class belongs to the kind, never to the caller;
- a forcing test over `types.EventKinds()` pins that only
  `StorageTraceLog` kinds are ever written to the log.

F-c's test that pins `StorageEventTable` equal to `Batchable()`
(`event_durability_test.go:158-163` on that lane) is changed to:
`Batchable() == (StorageEventTable or StorageTraceLog)`.

### 2.3 Evidence that must survive: the cite copy

Run acceptance is the one production reader that turns a trace line into
evidence:

1. `SynthesizeRunAcceptance` reads `ListEventsByTrajectory(owner,
   trajectory, 3000)` (`run_acceptance.go:92`).
2. It cites `tool.result` lines by `event:<event_id>`, with a URL of the
   form `/api/runs/{id}/events?after=<seq-1>&limit=1`
   (`run_acceptance.go:327-356`).
3. It upserts the record through the tape (`internal/store/run_acceptance.go:16`)
   **[V]**.

The route is reachable by the management desk through `choir.ProductAPI`
(`internal/agentcore/tools_product_api.go:219-220`;
`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:34`)
**[V]**.

Today the cited line is on the tape for ever. After R7 it is in the log
for the retention window. So, in slice S3, `addEventEvidence`
(`run_acceptance.go:327-339`) also writes three fields into the evidence
ref's `Details`:

- `event_sha256`: the SHA-256 of the line's canonical JSON;
- `event_record`: the line itself, capped at 64 KiB, with
  `event_record_truncated` set when the cap cuts it;
- `trace_segment`: the segment number.

The acceptance record carries its own evidence, and the sealed segment's
tape digest (section 4.5) lets anyone check `event_sha256` against the
segment while it is kept. This makes acceptance better than today even
without R7. Today the record holds a URL whose target a later tape
compaction could remove.

What a reader sees after retention: the URL answers an empty page, and the
acceptance record still holds the line. A synthesis attempted after the
window finds no lines and reports the acceptance blocked, which is the
safe direction (telemetry-batching design §3.3). The verifier's
"inconclusive, not failed" rule for a trajectory interrupted by a restart
(same section) extends to "inconclusive when the trajectory is older than
the log's retention floor". The verifier has no production caller today,
so that is a one-line change, made in the same slice.

## 3. Where the trace lines live

| Option | For | Against | Verdict |
|---|---|---|---|
| **A. Segment log on the guest's persistent disk**, beside the store workspace | No network hop and no host fate-share. Write cost close to 1x the payload. A process crash loses nothing. Retention is `unlink`. Reads stay in-process, so routes do not change. The disk already holds the store, so no new capacity class | New guest-local code with its own crash rules. Not on the signed tape. Not backed up off-host unless the disk backup covers it **[H]** | **Chosen** |
| B. The host tape segments (`internal/tapestore`, `internal/platform/tape_segments_write.go`) | Already fail-stop and reviewed three times. Already on for every computer (owner, §34) | Digest-addressed put and get with no run, owner or trajectory index (`tapestore/format.go:31-37` has only event, payload and receipt namespaces). Every record crosses the guest-to-host pin endpoint. It is the evidence store, with signed receipts per pin, which is the cost R7 removes. Reads would fate-share with the host (standing question 7) | Rejected. Its fail-stop rules are reused, not its store |
| C. A second embedded Dolt database for trace lines | Isolates GC | Writes the same bytes per op. Splits finalize across databases (F-c §7). Needs a second extractor, swap and base | Rejected |
| D. A SQLite file (`modernc.org/sqlite` is already a dependency, `go.mod:30`; used by `internal/actorruntime/adapter.go`) | Indexed queries for free | A B-tree rewrites pages per insert, and retention by `DELETE` writes again. The WAL and checkpointing are a second engine to tune on a guest | Rejected for v1. It is the fallback if the segment index proves too slow (F16) |
| E. A memory-only ring | No disk at all | Every restart erases it. The owner wants trace evaluation over days (§35) | Rejected |

### 3.1 Layout

The log directory is `filepath.Join(filepath.Dir(storePath), "trace-log")`,
a sibling of the store workspace. Restore quarantines and swaps the
workspace and the marker (`internal/agentcore/rematerialize.go:227-258`),
so the log must never sit inside the workspace. A test pins that
(F19). On the guest the store is under `/mnt/persistent`
(`nix/autoputer-vm.nix:873` points the schema receipt at
`/mnt/persistent/state.texture/`) **[V]**, so the log is about
`/mnt/persistent/trace-log/` **[H]**, mode 0700 and owned by the runtime
user. It sits on the same disk and with the same at-rest protection as the
Dolt workspace. Whether that disk is encrypted at rest is not read here.

Files, one per segment:

- `seg-00000123.tlog` is the open or sealed log.
- `seg-00000123.tidx` is written at seal.
- `seg-00000123.tlog.zst` replaces the `.tlog` once compressed (slice S6).

**Rotation.** Seal at whichever comes first: 1 hour, 64 MiB or 50,000
records. The record cap bounds the open segment's in-memory index (F18).

**The segment header.** Magic `CHTRACE1`, a format version, the segment
number, the ComputerID and the creation time.

**A record.** `u32 length | u32 crc32c | u8 type | body`.

- Type 1 is a line: the `types.EventRecord` JSON exactly as the store
  would have written it, plus two envelope fields. `after_tape_seq` is the
  appender's acknowledged head sequence at write time, read from memory.
  `boot_id` names the process.
- Type 2 is a boundary: `{reason, head_seq, at}`, where the reason is one
  of `open`, `crash_recovered`, `restore`, `setting_on` or `setting_off`.
  Readers skip boundaries, except to annotate.
- The record cap is 1 MiB. The largest `tool.result` is about 612 KiB:
  100 KiB of visible output and 512 KiB of durable output
  (`internal/toolregistry/batch_executor.go:155`, `:222`) **[V]**. A larger
  line goes to the tape as today, and a counter records it.

**The `.tidx` file.**

- A summary: the `stream_seq` and timestamp ranges, the record count,
  bytes by kind, the owner set, and the run set with each run's minimum
  and maximum `seq`.
- Two sorted entry lists: `(run_id, owner_id, seq, offset)` and
  `(trajectory_id, stream_seq, offset)`.
- A CRC.

It is written, fsynced and its directory fsynced before the segment
counts as sealed. A sealed segment is never modified. It may only be
compressed into a new file, or deleted.

**What stays in memory.** The open segment's two entry lists (about 1 MB
at the paper's rate of about 9,300 lines per hour), and the summary of
each sealed segment.

### 3.2 The writer, and where it hooks into `AppendEvent`

`Store.AppendEvent` (`internal/store/store.go:2132`) allocates `seq` and
`stream_seq` under `eventMu` (`:2181-2244`) **[V]**. The trace-log branch
sits exactly where the sealer's branch sits (`:2246-2262`):

1. When the setting is on and `rec.Kind.Storage() == StorageTraceLog`, run
   the same `PreflightWrite` refusals (`:2248-2253`), then append the
   record to the log while still holding `eventMu`. The log's order is
   then allocation order.
2. The append is one buffered `write(2)` of a few KB, about 10 µs **[H]**,
   measured by F16.
3. Release `eventMu` and return. No tape op, no Dolt statement and no host
   pin happen.

The log's own mutex is a leaf. The fsync never holds it: the syncer
records the write offset, calls `fdatasync` on the descriptor without the
append mutex, and then publishes that offset as synced.

## 4. Durability and loss

### 4.1 What is synced, when

- Every line is written to the page cache before `AppendEvent` returns, so
  every in-process reader sees it at once. The sealer's read barrier
  (`internal/store/telemetry_reads.go:41-50`) has nothing to seal for log
  kinds.
- `fdatasync` runs once a second while the log is dirty. One second is the
  owner-ruled window, and it equals the sealer's default `W`.
- **The ordering barrier.** Before any acknowledged tape append, the log
  fsyncs if it is dirty. The appender already has a barrier slot that runs
  outside its mutex and outside `eventMu`
  (`internal/computerevent/appender.go:196-215`) **[V]**. Today only the
  sealer installs it (`telemetry_sealer.go:180`). R7 installs one composed
  barrier: the sealer's, if one is configured, then the log's. This keeps
  the batching design's principle 4, "loss is a tail, never a hole"
  (telemetry-batching design §2). Its cost is at most one fsync per
  acknowledged tape event, about 4 to 5 per second on the paper after R7,
  and none when the log is clean. F16 measures it. If it costs more than
  5 ms at p99, the timer alone is the fallback, and losing the hole
  guarantee becomes a named residual.
- At rotation, at `Store.Close` (beside `closeTelemetry`,
  `store.go:1114-1123`), on graceful shutdown, and when a planned restart
  takes `SetTelemetrySynchronous` (`telemetry_sealer.go:251-268`), the log
  fsyncs and writes nothing new until it is released.

### 4.2 What a crash may lose

| Event | Lost | Visible as |
|---|---|---|
| Process crash (panic, the daemon OOM-killed, a systemd restart) | **nothing**: the page cache survives the process | nothing |
| Planned restart, graceful shutdown | nothing (fsync on close) | a `open` boundary at the next boot |
| VM kill, kernel panic, host power loss | the lines written since the last fsync: **at most about 1 s**, and never a line older than an acknowledged tape event | a `crash_recovered` boundary; gaps in `seq` and `stream_seq`; the boot's "interrupted by a restart" record already says the last window of a run's trace may be missing (telemetry-batching design §5.2) |
| A torn tail (a partial last record) | that record | cut by CRC at reopen (F4) |
| The persistent disk is lost | the whole log | the tape keeps every acknowledged event and the hourly digests (4.5) |

Batching loses its whole open window on any crash, including a process
crash (`telemetry_sealer.go:318-325`). The log loses less.

### 4.3 Failure while writing

These are the tapestore rules (`appender_failstop_test.go` S4, S5, S6, S8):

- The first failed write or fsync poisons the log writer for this process.
  The writer touches no file again.
- **The sink falls back to the tape**: the old path, through the sealer,
  F-c or `og_objects`, whichever is on. No line is refused.
- A `trace_log.poisoned` flag appears in observability and in `/health`'s
  degraded list.
- The next process recovers. It seals the unsealed segment it did not
  write (cutting a torn tail), and starts a segment of its own.
- An advisory lock on the directory stops a second process from writing
  (F12). That process falls back to the tape.

**A full disk** is handled differently. When the persistent disk's free
space falls below `CriticalAvailBytes` (512 MiB,
`internal/persistentdisk/usage.go:13`) **[V]**, the log first deletes its
oldest sealed segments. If space is still short, it **drops** new lines and
counts them in `trace_log.dropped_disk_critical`. It does not fall back to
the tape, because a Dolt write on a full disk is worse. The owner's ruling
makes trace lines loss-tolerant, and his request puts resource utilization
first. This is the conservative default; it is not marked open.

### 4.4 Sequence numbers

Lines keep taking `seq` and `stream_seq` from the same counters as tape
events. The seeds read all sources:

- **Per run** (`store.go:2181-2214`): the maximum of today's
  `og_objects` read, F-c's `choir_events` read, and the log. The log is read
  from the open segment's index and the run set of each sealed summary,
  then one `.tidx` lookup.
- **Store-wide `stream_seq`** (`:2224-2242`): the maximum of today's seed
  and the newest segment's highest `stream_seq`.
- **The gap.** When the log is configured, the seeds add `telemetrySeqGap`
  (512, `telemetry_sealer.go:50`) after any boot, as the sealer does
  (`store.go:2199-2210`, `:2238-2240`). Lines lost to a VM crash may
  already have been delivered on the live bus, so a WebSocket client's
  cursor can be ahead of the durable maximum. The gap keeps that cursor
  from skipping new events (telemetry-batching design §6.1).

After a restore, the log's marks are at least the restored store's
maxima, so the numbers never repeat across the restore. Today only the
in-memory marks protect this (`Reopen`, F-c §3.4) **[V]**, so it is
stronger than today.

### 4.5 Tamper evidence: a sealed-segment digest on the tape

At each seal, the writer appends one acknowledged object,
`choir.trace_segment`. It carries:

- the ComputerID and segment number;
- the SHA-256 of the sealed `.tlog` and of the `.tidx`;
- the record count and bytes by kind;
- the `stream_seq` and timestamp ranges;
- the boot id.

That is about 24 objects a day, an ordinary `og_objects` row each:
negligible journal, and it enters the witness like any row. A segment can
be checked against the tape while it is kept. Once deleted, the tape still
shows that the segment existed and what it held by count (`I23`). Lines in
the open segment (at most an hour) are not yet covered. This digest is part
of the writer slice, S4. Dropping it is open decision O4.

## 5. Retention and compaction

| Bound | Default | Where it is set |
|---|---|---|
| Age | 7 days | a constant, with an env override for tests |
| Bytes | min(2 GiB, 10% of the persistent filesystem's total) | the same; on a default 8 GiB disk (`persistentdisk/usage.go:10`) that is about 819 MiB |
| Disk critical | below 512 MiB free: delete the oldest sealed segments first, then drop new lines (4.3) | `persistentdisk.CriticalAvailBytes` |

- **Retention work** runs on the existing 5-minute maintenance tick
  (`internal/autoputer/run.go:727`, `StartPeriodicDoltGC`), as one `stat`
  and `unlink` pass. It holds no Dolt lock and has no store connection, so
  it cannot stall the store.
- **What is deleted**: only whole sealed segments, oldest first, never the
  open one. On Linux a reader holding a descriptor finishes reading a
  segment that was just unlinked (F7).
- **The floor is visible.** The log exposes `retention_floor`, the oldest
  kept timestamp and `stream_seq`, in observability, so a reader can tell
  "older than retention" from "missing".
- **There is no compaction**: the log is append-only, and nothing is
  rewritten. Compression is slice S6. A sealed segment becomes zstd frames
  of about 1 MiB, with the `.tidx` pointing at (frame, offset), and
  `github.com/klauspost/compress` is already a direct dependency
  (`go.mod:19`; used by `internal/platform/tape_sweep_apply.go`) **[V]**.
  Typical JSON compresses 4 to 8 times **[H]**, which at the same byte cap
  holds the 7 days on the paper.
- **A deletion is not a tape act.** Trace lines are not state, and the
  tape holds the segment digest (4.5).
- **Old `choir.event` rows in Dolt are not R7's.** Removing them is the
  tape-surgery and compaction question the owner opened on 2026-10-20
  (owner notes §26), and it needs its own design.

Expected size on the paper **[H]**: 2.57 lines/s after R1 to R4. The mean
line size is not measured; slice S0 measures it. At 2 to 8 KB a line, that
is 0.4 to 1.8 GB a day uncompressed, so 1 to 5 days within 2 GiB before
S6, and 7 days after it. An ordinary computer writes far less, so the age
bound binds first there.

## 6. How every reader moves

### 6.1 The reader map

From a reader survey on this checkout (an Explore pass and my own reads)
and F-c §1 **[V]**:

| Reader | Today | After R7 | Changes outside `internal/store`? |
|---|---|---|---|
| `GET /api/runs/{id}/events` (`internal/agentcore/api_run_events.go:53`), relaxed, through `ListEventsAfter` (`store.go:2294`) | `og_objects` | union with the log (6.2) | none: same cursor (`seq`), same `next_after`, same limit |
| `GET /internal/runtime/runs/{id}/events` (`api.go:404`, `:437`), through `ListEvents` | same | union | none |
| WebSocket catch-up `GET /api/ws?after_seq=` (`internal/agentcore/live_ws.go:79`), through `ListEventsByOwnerAfter` (`store.go:2357`) | same | union; the cursor is still the highest `stream_seq` seen (`frontend/src/lib/Desktop.svelte:986-1042`) | none |
| The live stream after catch-up (`live_ws.go:110`), and Texture's document stream (`internal/textureowner/texture.go:1698`) | the in-memory bus (`internal/events/bus.go:139-197`) | unchanged: every writer publishes after its store write (`runtime.go:4716-4720`) | none |
| `GET /api/costs` (`internal/apihandler/costs.go:74`), through `ListEventsByOwner` (`store.go:2325`), newest by timestamp | same | union (6.2) | none |
| `GET /api/texture/documents/{id}/diagnosis` (`textureowner/texture.go:1969`), `ListEventsByOwner` limit 80, called by `frontend/src/lib/texture.js:220` | same | union | none |
| Run acceptance (`run_acceptance.go:92`), `ListEventsByTrajectory` (`store.go:2421`), consistent | same | union, and the cite copy (2.3) | `run_acceptance.go` (slice S3) |
| Texture workflow verifier (`texture_workflow_verifier.go:126`) | same; no production caller | union; "inconclusive" past the retention floor | the verifier (S3) |
| System1 replay cache (`system1_verb.go:176`), `ListEventsByChannel` (`store.go:2392`) | `og_objects`; acknowledged kind | **no log half**: the kind stays on the tape. F-c renames the method `ListAcknowledgedEventsByChannel` and pins its callers (F-c F10) | none |
| Per-run and stream seeds (`store.go:2181`, `:2224`) | `og_objects` | all sources plus the gap (4.4) | none |
| `choir trace export` (`cmd/choir/trace.go:217`, `:327-360`), which pages `/api/runs/{id}/events?limit=1000` and builds cells and egress from `desk.cell` and `desk.egress` (`:363-364`) | route | route unchanged | none. R4 changes the `desk.*` shape, not R7 |
| `choir trajectory ...`, `choir lifecycle events` (`cmd/choir/commands.go:115-159`) and `choir texture watch` (`cmd/choir/main.go:581`) | read `choir.lifecycle_event` (`api_trajectory.go:122-140`, `ListLifecycleEventPage`) | **not affected**: no `choir.event` | none |
| Texture supervision views (the commitment ledger) | commitment records (`commitment_views.go:528`, `texture_evidence_sources.go:787`) | not affected | none |
| Frontend: `PromptSurface.svelte:157` (`tool.*`, run state), `TextureEditor.svelte:1866`, `ComputeMonitorApp.svelte:44` | live bus plus WS catch-up | unchanged | none. No frontend code calls `/api/runs/*/events` or `/api/costs` |
| Playwright: `frontend/tests/helpers/run-trace.js:11-40` (`tool.result`), used by `texture-deployed-live-search`, `-model-sweep`, `-real-workflow-demo` and `-version-advancement` | route | route unchanged | none |
| Scripts: `reeng_tracer.mjs:436,457`, `reeng_ttl_session.mjs:101`, `management_latency_probe.py:127`, `claude_delegate_probe.py:79`, `l2a_window_pairs.py:157` (`/api/runs/{id}/events`); `cpu_efficiency_probe.py:74` (`/api/costs`) | route | unchanged | none |
| Replay, witness, fingerprints, vocabulary fences, residue import (F-c §1.4) | read Dolt | not affected: log lines are never Dolt rows (section 7) | none |
| `ListEventsByTrajectoryAfter`, `ListEventsOG`, `ListEventsByOwnerOG` | no production caller (F-c §1.7) | deleted by F-c deploy 1. Not made union-aware here | none |

The owner's "better API to get full multidesk traces" (§35) is a natural
reader of the log: a trajectory-ordered read across runs, served from the
`.tidx` trajectory list. It is a follow-on (S7), not part of R7.

### 6.2 Merge rules

These are F-c's union rules (F-c §4.4), with one more half:

- Each half fetches at least the caller's limit.
- Rows are deduplicated by `event_id` before any cut. A duplicate means a
  rollback window (section 8). The copy kept is the Dolt one.
- The cut uses the method's own merge key, with `stream_seq` as the
  tie-break.

| Method | Log half | Merge |
|---|---|---|
| `ListEventsAfter(run, after, limit)` | run index: `seq > after` ascending, first `limit` | ascending `seq` |
| `ListEventsByOwnerAfter(owner, after, limit)` | binary search the segments by `stream_seq` range, then scan forward with an owner filter | ascending `stream_seq` |
| `ListEventsByOwner(owner, limit)` | scan backward from the open segment by timestamp | descending timestamp, as today. F-c's known boundary difference at the cut (callers set `Timestamp` before `eventMu`) applies unchanged |
| `ListEventsByTrajectory(owner, trajectory, limit)` | the trajectory list of each segment whose summary holds it, newest 10,000 | rebuild today's window first (the newest 10,000 by timestamp), then sort ascending by `stream_seq` and keep the first `limit`, exactly as F-c §4.4 does. The window's known oddity (it keeps the oldest 3,000 of the newest 10,000) is kept, and recorded as discovered |

**Fate-sharing (standing question 7).** The log half needs only the guest
process and its disk. It does not need the single Dolt write connection
(`internal/store/texture.go:352-355`) or the engine mutex. The union
still waits for its Dolt half, so a trace read still stalls during a GC
pass. Serving the log half alone when the Dolt half misses a deadline is
a later reader change, not R7.

## 7. Replay, restore, the witness and checkpoint identity

### 7.1 Classification: trace lines are not state

The proposed new row for the ontology's ledger split
(`docs/computer-ontology.md:168-183`):

| Ledger | Owns | Transition authority |
|---|---|---|
| Trace log (guest-local append-only segments) | trace lines of runs: progress, cost estimates, tool calls and results, desk cells | runtime append only. Diagnostic and evidence input; never semantic event, trajectory, settlement, acceptance or promotion truth. Not in the restore set (machine state, like runtime caches). Sealed-segment digests on the tape |

It is written into the ontology only when the owner rules on O1, as part
of the S4 landing (standing question 3).

### 7.2 Replay and the witness

- **Replay** (`ReconstructInto` and `ReconstructThroughTarget` through
  `finalizeBatch`, F-c §3.2) projects tape ops only. Log lines are never
  tape ops, so replay never sees them. It keeps projecting the
  pre-cutover `choir.event` ops exactly as today.
- **The witness.** `DoltStateExtractor` lists the store's tables with
  `SHOW TABLES` and hashes their contents (F-c §3.3; witness construction
  in `internal/projectionbase/witness.go:16` and
  `internal/agentcore/replay_completeness.go:489`) **[V]**. The log is a
  directory of files outside the Dolt database. It is in no table, so it is
  **outside the content witness by construction**. Trace lines stay out of
  the witness. Old trace rows already in `og_objects` and `choir_events`
  stay in it, unchanged.
- **Live against replay**, with the setting on: the live store has no log
  lines in Dolt, and replay produces none. So they are equivalent, and F3
  tests exactly this. The hourly `choir.trace_segment` objects are
  ordinary tape objects, present on both sides.
- **No change** to the replay manifest
  (`internal/agentcore/replay_eligibility.go:44-94`), the extractor's
  table sets, `storeschema.Version`, the vocabulary fences or residue
  import. That is the main simplification over F-c, which needed all of
  them.

### 7.3 Restore to a prior event head

Restore closes the store (`rematerialize.go:228`), quarantines and swaps
the workspace (`:227-258`), and reopens it (`:262`) **[V]**. The log is
outside the swap, so:

- `Store.Close` closes the log (fsync), and `Reopen` reopens it and writes
  a `restore` boundary `{from_head, to_head}`.
- The log keeps lines for events after the restored head. They describe
  what happened, which a restore does not undo, because "events are never
  deleted" (ontology, lineage section). A line whose `after_tape_seq`
  exceeds the restored head belongs to the abandoned branch. Readers may
  label it so; it is never hidden.
- **What the restore fence needs.** It does not read, hash or verify the
  log, and checkpoint identity (event head plus content witness, F-c §3.3)
  is unchanged. It needs only the boundary record and the seeds of 4.4, so
  that numbers do not repeat after the swap.
- A **fork** (a new ComputerID from a parent checkpoint) starts with an
  empty log, because its store path, and so its log path, is its own
  **[H]**. Settled by F20.
- **Self-development capsules** open their own stores, so their logs are
  theirs **[H]**. A candidate never writes the parent's log.

### 7.4 Old tapes, and the transition

Nothing is moved:

- `choir.event` rows written before the cutover stay in `og_objects`, and
  rows F-c deploy 2 wrote stay in `choir_events`.
- Readers merge all sources for as long as old rows exist, which is until
  a retention or compaction decision removes them (5).
- A replay of an old tape rebuilds those rows exactly, and the log neither
  helps nor hinders it.

## 8. The setting, rollback and the floor

**The setting.** `store.trace_log`, a new entry in `KnownComputerSettings`
(`internal/store/computer_setting.go:63-70`), default off. It is an owner
act on the tape (`PutComputerSetting`, `:86-105`), set through
`POST /api/computer-settings`. It applies at once (the pattern of
`internal/agentcore/computer_settings.go:105`) and at boot. It decides only
the sink of `StorageTraceLog` kinds. The readers read the log whenever it
exists, whatever the setting says. No env override ships for production
computers; one for tests is allowed.

**The rollback matrix.**

| From → to | What happens | Repair |
|---|---|---|
| Setting on → off | New lines go to the tape again, by the old path. A `setting_off` boundary is written. Readers still merge the log, so nothing hides | none |
| S4 → S3 (the writer reverted, readers kept) | Same as off. The log stays readable | none |
| S3 or later → a binary from before S3, after lines exist | The old binary cannot see log lines: trace pages for that window lose them, but nothing on the tape is wrong. Its seeds read Dolt only, so it may reissue `stream_seq` numbers the log holds; within one run the reissue is safe, because a crash boot closed the run. When rolled forward, the union shows both and orders them by `stream_seq` and `event_id` | **pre-rollback step**: turn the setting off and wait for one tape event after the off write, so the newest Dolt event carries the store-wide maximum. That is check 1 of F-c §4.5. No witness or tape repair is ever needed |
| S6 → S5 (compression reverted) | The old reader cannot read `.zst` segments | S6's reader ships one release before S6's writer, the expand-then-write shape |

So R7 has **no hard floor**. A rollback below S3 only hides trace lines
and risks a few `stream_seq` duplicates in a loss-tolerant stream. Nothing
on the tape changes shape. That is why no `storeschema` epoch bump is
needed. F-c's floor (its deploy 1, once any op is stamped) still applies
for F-c's own reasons.

## 9. The build plan, in slices

Each slice can land on its own and goes through the full Landing Loop.
Failure-mode tests (section 10) come before the code in each slice, as a
yellow commit.

| Slice | Contents | Class | Protected surfaces | Proof | Rollback |
|---|---|---|---|---|---|
| **S0** counters | `TelemetryStats` (`internal/store/telemetry_stats.go:182-205`) gains three things. (1) `event_payload_bytes_by_kind`, the mean line size. (2) `events_by_sink{tape,trace_log,dropped}` per kind. (3) `tape_choir_event_ops_by_kind`, counted in `AppendEventOG` and the sealer, where the kind is known without decoding. Also the problem record's §6 counter of `og` kinds in `recordTapeEvent` (`:114`), if R1 to R4 have not added it | orange (observability only; nothing branches on it) | none | the paper's **before** readings: two snapshots 600 s apart inside one GC interval: the `store` tree bytes, `/health` `dolt_gc`, `events_appended_by_kind`, `tape_ops_by_kind` and the new counters | revert |
| **S1** library | `internal/tracelog`, standard library only: writer, reader, recovery, seal, `.tidx`, retention, boundary records, an advisory lock. No production caller | orange (new code, unreachable) | none | the failure-mode tests F4 to F7, F12, F13 and F16 to F18 in the package, plus a power-loss harness: a sync hook that truncates every file to its last synced offset (the `SetSyncHookForTest` pattern, `tapestore/appender.go:602`) | revert |
| **S2** readers and seeds (inert) | `Store` opens, closes and reopens the log; the union in every list method; seeds over all sources; `StorageTraceLog` in `types` with its tests; `store.trace_log` registered but not acted on; a `trace_log` observability block (records, bytes by kind, fsyncs, barrier fsyncs and their p99, segments, retained bytes, `retention_floor`, dropped, fallbacks, `poisoned`); the restore boundary | red (the Trace/evidence read path, sequence allocation) | event list routes, WS catch-up, the seeds | with the log empty, every route answers byte-identical pages (differential F2); staging deploy; the readings of S0 do not move | revert; nothing was written |
| **S3** the cite copy | `addEventEvidence` writes `event_sha256`, `event_record` and `trace_segment`; the verifier's "inconclusive past the floor" | red (run acceptance) | run acceptance | F14 on staging: synthesize an acceptance on a test computer and read the record back; the copy is there | revert. Records written in the meantime keep their extra fields, which old readers ignore |
| **S4** the writer, on one test computer | The `AppendEvent` branch; the composed barrier; group fsync; fallback on poison; the drop at disk-critical; retention on the maintenance tick; the hourly `choir.trace_segment`; the setting applied live and at boot | red (Trace/evidence, the tape write path, the guest disk) | the tape (fewer events), Trace, run acceptance | below | setting off |
| **S5** the paper | the owner's setting write on the paper, after S4 passes and after O1 | red (an owner act on a `critical_protected` computer) | as S4 | below | setting off |
| **S6** compression | the zstd reader first, then the zstd seal, in two releases | orange (the guest disk; the log's own format) | trace reads | retained days on the paper reach 7 under the cap; trace pages unchanged | revert the writer; the reader stays |
| S7 follow-on, not R7 | the multi-desk trace API (owner §35) over the `.tidx` trajectory lists; a lasting spend total if O5 says so; retiring F-c deploy 3 for log kinds | — | — | — | — |

### 9.1 The proof for S4 (test computer) and S5 (the paper)

The test computer is minted as F-c and the batching gate did it
(`scripts/qa_test_computer.mjs`, `warmness_class: test`). It runs a
comparable load: the batching gate's load, or a Texture and research
workload.

1. **Before.** S0's readings, over a 498 to 600 s window inside one GC
   interval.
2. **The setting write.** `POST /api/computer-settings
   {"key":"store.trace_log","value":true}`. Check that it takes effect
   with no restart (F15).
3. **After.** The same readings, the same window and a comparable load.
4. **Acceptance.**
   - (a) `events_appended_by_kind` for the log kinds is unchanged in rate:
     the same emission.
   - (b) `events_by_sink.tape` for those kinds is 0, and
     `events_by_sink.trace_log` equals their appended deltas.
   - (c) `tape_events` falls by about the log kinds' rate. On the paper
     after R1 to R4 that is 2.57/s.
   - (d) The journal's MiB per 10 minutes falls by the predicted amount,
     within ±30%. On the paper: 724 to 483 MiB per 10 minutes without F-c,
     608 to 483 with it.
   - (e) Every run's `/api/runs/{id}/events` pages hold the same kinds and
     counts as the appended deltas.
   - (f) `trace_log.dropped`, `fallbacks` and `poisoned` are 0.
   - (g) Barrier fsync p99 is at most 5 ms.
   - (h) A run acceptance synthesized on the test computer carries its
     cite copies.
5. **The crash drill.** Test computer only. Force a VM kill from the host
   through the product path, then read the counters after boot:
   - a `crash_recovered` boundary;
   - lost lines are at most the last second of writes;
   - no line is lost from before the run's last tape event;
   - there are no duplicate `stream_seq` values (an F1 differential over
     the API).
6. **The restore drill.** Test computer only. Run a checkpoint restore to
   a head before the cutover, then check:
   - the witness matches;
   - the `restore` boundary is present;
   - there is no number reuse;
   - pages after the restored head still list the abandoned branch's
     lines.
7. **The paper (S5).** Only steps 1 to 4. No drills on the paper. The
   evidence goes in `docs/evidence/trace-log-<step>-<date>.json`.

## 10. Failure modes, written before any code, each with its test

The harness is the store's tape harness
(`internal/store/telemetry_harness_test.go`: a real `Store`, the real
appender, a recording pin and CAS, `reopen`), unless noted.

| # | Failure | Test |
|---|---|---|
| F1 | `seq` or `stream_seq` is handed out twice across Dolt, `choir_events` and the log, after a reopen, a crash or a restore | append mixed kinds over several runs with the setting on; reopen; simulate a crash (the power-loss hook); `ReconstructThroughTarget`; append more. There is no duplicate `(run, owner, seq)` and no duplicate `stream_seq` across every source. Run it with batching on and off, and with F-c's stamp on and off |
| F2 | a reader forgets a half, or merges or cuts wrongly | differential: one event script into setting off and setting on, plus a toggle in mid-run. Every list method gives identical results for every cursor and limit, including `limit+1` paging in `handleRunEvents`. A source guard puts every `ListEvents*` method on the union-aware list |
| F3 | live and replay differ with the setting on | `ReconstructInto` a fresh store from a tape written with the setting on. The extractors are equivalent and `replayEligibility` is eligible. No log line appears in any table |
| F4 | a torn tail is served, or a partial record breaks the reader | write, cut the file mid-record, reopen. The partial record is cut, earlier records read whole, and new writes go to a new segment (S6 of tapestore) |
| F5 | a failed write or fsync is ignored, or a poisoned writer keeps writing | sync and write hooks fail. The writer is poisoned, touches no file again, the sink falls back to the tape, no line is lost, and the counter moves (tapestore S4, S5) |
| F6 | at disk-critical, the log feeds Dolt instead of dropping | a fake `statfs` below 512 MiB: the oldest sealed segments go first, then lines drop with a counter, and no tape op is written for them |
| F7 | retention deletes the open segment or an unsealed one, or a reader fails on an unlinked segment | age and byte bounds. Only sealed segments, oldest first. A reader holding a descriptor finishes. `retention_floor` moves |
| F8 | a hole: an acknowledged event is durable while an earlier line of its run was lost | the power-loss hook after an acknowledged append: every line before it survives (the barrier) |
| F9 | a replay-only boot or a rebuild writes to the log | `RUNTIME_RECOVERY_REPLAY_ONLY`, and the projection-base rebuilder: the log is untouched |
| F10 | a planned restart or a shutdown loses lines | `SetTelemetrySynchronous(on)` and `Close`: an fsync and nothing unsynced. A markerful boot sees every line |
| F11 | the class leaks: an object-graph kind reaches the log, or the lifecycle fold does | a parser test and a forcing test over `types.EventKinds()`. A source guard: `lifecycle.go` never reaches the log, and only `AppendEvent` writes it |
| F12 | two writers on one directory | a second `Store` over the same path cannot take the lock. It falls back to the tape. The first keeps writing |
| F13 | an oversized line | a record over 1 MiB goes to the tape as today, and a counter records it |
| F14 | acceptance evidence is lost to retention | synthesize, delete the segment, read the acceptance: the cite copy is present and its `event_sha256` matches the copy. A synthesis after the floor reports blocked, and the verifier reports inconclusive |
| F15 | the setting does not apply at once, or is lost across a reboot | API test: it applies with no restart, and a crash boot reads it |
| F16 | the append adds latency under `eventMu`, or the barrier fsync is slow, or reads over many segments are slow | benchmarks: append p99, barrier p99, a run's page across 168 segments, WS catch-up from an hour back. Bars: 50 µs, 5 ms, 50 ms and 50 ms **[H]** |
| F17 | the restore loses the log or reuses numbers | `rematerialize`: the log directory survives; a `restore` boundary is written; F1 holds |
| F18 | the in-memory index grows without bound | rotation at 50,000 records; the memory stays under a cap that the test asserts |
| F19 | the log is placed inside the workspace that restore quarantines | a path test against `rematerialize.go`'s workspace and marker |
| F20 | a fork inherits the parent's log | a fork test or a code read: the fork's store path is its own |
| F21 | the hourly segment digest is missing or wrong | seal; read the `choir.trace_segment` object; hash the files; they match. A crash between the seal and the object append: the next boot appends it for the sealed segment it finds (one per segment, idempotent by canonical id) |
| F22 | a planned restart adds a resumer | none is added. A source guard keeps the boot path's new statements to the open and the seal of an unsealed segment (CLAUDE.md, "Restarts End Work") |

## 11. The numbers

The base is the problem record's model **[H]**: per second, 1,810 KiB
total. Trace lines (the batched kinds) are 5.70/s at 160 KiB, which is
912 KiB/s. The run-memory and other-object terms are as in §2 there.
Rows are cumulative.

| State | Journal KiB/s | MiB per 10 min | Journal reaches 1 GiB in | GC pass every (5-min tick) | Tape events/s | Trace lines on the tape/s |
|---|---|---|---|---|---|---|
| Today | 1,810 | 1,060 | 9.7 min | 10 min (seen: 10 to 20) | 10.8 | 5.70 |
| R1 to R4 | about 1,235 | 724 | 14.2 min | 15 min | about 6.9 to 7.7 | 2.57 |
| R1 to R4 and F-c deploy 2 (unbatched, ratio 0.52) | about 1,037 | 608 | 16.9 min | 20 min | the same | 2.57 (in `choir_events`) |
| **R1 to R4 and R7** (with or without F-c) | **about 824** | **483** | **21.2 min** | **25 min** | **about 4.3 to 5.1** | **0** |
| R7 without R1 to R4 | about 898 | 526 | 19.5 min | 20 min | about 5.1 | 0 |
| R1 to R4, R7 and O2 (`tool.result` stays on the tape, without F-c) | about 957 | 561 | 18.3 min | 20 min | about 5.1 to 5.9 | 0.83 |

How the R1 to R4 row is built: R1 is 238 KiB/s, R2 at most 75, R3 134,
R4 128. The R2 and tape-event ranges depend on how often the cursor
actually moves, which is unmeasured.

**Bytes saved per second by R7 on the paper:**

- 411 KiB/s after R1 to R4;
- 213 KiB/s after R1 to R4 and F-c deploy 2;
- 912 KiB/s alone.

**GC, and the 503s.** Pass time is about 16.6 s per GiB plus 2 s (problem
record, first section) **[M]**. With a pass every 25 minutes at about
1.18 GiB, a pass takes about 22 s, so the store is stopped for about 1.5% of
the time, against about 4% today (a 25 s pass every 10 minutes). In FP0's
hour of probes, at least four 503 bursts sat inside a GC window
(`docs/problems/paper-post-refresh-health-windows-2026-10-22.md`,
"Holders found so far") **[M]**. Those become about 2.5 times rarer. The
other bursts come from other holders of the single write connection under
full load. R7 removes about 2.6 of about 7 tape events a second, and each
tape event is three transactions on that connection (problem record,
"What one tape event writes"). So those bursts should thin by about 33 to
38% **[H]**, settled by the observability goroutine reading named there.
R7 does not end the 503s. The bounded `/health` (lane `fp1-health-bounded`)
and a GC off the write connection do.

**Permanent growth** **[H]**, at about 10 KB of retained Dolt history per
tape event (problem record §3): R7 removes 2.57 events/s, which is about
26 KB/s, or **about 2.2 GB a day** of history GC never reclaims. The host
saves about 10 records a second: four per event, in segments or legacy
files. The log itself writes 0.4 to 1.8 GB a day before compression,
inside a fixed cap, and deletes it.

## 12. Standing questions

1. **Who settled this?** Loss tolerance up to a window: the owner, §27
   item 18. The resource-first priority: the owner, today. F-c's defaults:
   the owner, 13:20. Trace lines off the signed tape: **nobody yet**. That
   is O1, and the writer goes no further than a test computer without it
   (section 1).
2. **Does the topology conform to settled decisions?**
   - Two-store taxonomy: no new Dolt store, so yes.
   - Ledger split: a new row of the "never semantic truth" kind, which the
     ontology already allows by example (the actor recovery log). So yes,
     pending O1.
   - Restore-set boundary: the log is machine state, out of the set; the
     VM-local embedded state stays in. So yes.
   - "Restarts end work": no resumer is added (F22).
   - F-c: it composes (6.2, 7.4).
3. **What references what is deleted?** Nothing is deleted by R7. F-c
   deploy 1 deletes three caller-less methods (F-c §1.7). When O1 is ruled,
   the ontology's ledger table, `event_durability.go`'s comments (`:28-35`),
   the batching design §3.1 and the F-c design's §7 row on "no telemetry
   in Dolt" are updated in the S4 landing. The problem record's R7 row
   points here.
3a. **Registry hygiene.** This is a design document, not a Definition, so
   `docs/ACTIVE.md`, `docs/mission-graph.yaml` and
   `docs/doc-authority-manifest.yaml` are not touched by this commit. The
   build station's goal file and its registry entries are FP0's, made
   when the build is scheduled.
4. **Who consumes this in production?** Section 6.1 lists every production
   reader with `file:line`. Acceptance is the only one that needs evidence
   to last, hence S3. The verifier has no production caller.
5. **What is the single authority?**
   - For an acknowledged event, the tape.
   - For a trace line, the log. One line lands in exactly one sink, chosen
     by the kind and the setting at append time, so the two never disagree
     about a line.
   - For a cited line, the acceptance record's copy, checked against the
     segment digest on the tape.
   - For sequence numbers, the in-memory counters, seeded from the maximum
     of every source plus the gap.
6. **What artifact proves success?** The paper's own readings before and
   after (9.1 steps 1 to 4), in an evidence JSON. Journal MiB per 10
   minutes, `tape_events` and `events_by_sink` are read from product
   routes; a narrated "it should be lower" proves nothing.
7. **What does the read path fate-share with?** The guest runtime and its
   persistent disk for the log half, which is less than today: not the
   Dolt connection, not the engine mutex. The union still waits for its
   Dolt half (6.2).
8. **What happens on restart?** Section 4.2: a process restart loses
   nothing, and a VM crash loses at most about 1 s of trace lines, as a
   tail. Nothing is resumed.
9. **Could an agent do this without SSH?** Yes. The setting is
   `POST /api/computer-settings`. The readings are
   `/api/runtime/observability` and `/health`. The proofs read
   `/api/runs/{id}/events`. The crash drill uses the host's product path
   for a VM kill, on a test computer only.

## 13. Alternatives considered

| Alternative | Why not |
|---|---|
| F-c alone (4 trees) | It halves the per-line cost (ratio 0.52, F17 on the d2 lane) but keeps the tape event, the host records, the permanent history, the witness weight and no deletion. It is the right step now (it is built), and R7 is the end state |
| F-c with telemetry batching | 18.5 KiB per op batched (F17) is close to R7 on the journal, but batching cannot go on the paper before the planned-restart drain fix (`lane/fp1-planned-restart-drain`). It keeps the history, the host records and no deletion. After R7 the sealer serves only `loop.compaction.completed`, so it is nearly moot |
| Host tape segments, a second Dolt, SQLite, a memory ring | Section 3 |
| Trace lines only on the bus (no durable trace at all) | It loses `/api/costs`, trace export, acceptance evidence and the owner's trace evaluation. R1 already does this for the one kind with no durable reader |
| Move the old `choir.event` rows into the log | About 120 KiB of journal per row deleted from 13 trees (F-c §4.3), and live and replay would diverge unless the move went through the tape. Retention of old rows is a separate decision |
| Put each line's digest on the tape (per-line tamper evidence) | That is one tape event per line again, which is the cost removed. The hourly segment digest gives tamper evidence per segment for 24 events a day |

## 14. Not determined from code

- The mean trace-line size on the paper (S0's counter), and so the log's
  bytes a day and the days the cap holds.
- Whether the persistent disk is encrypted at rest, and whether the
  off-host backup covers `/mnt/persistent` outside the workspace.
- The barrier fsync's latency on the paper's virtio disk (F16; S4's gate).
- How often the R2 cursor moves (the 6.9 to 7.7 range of tape events per
  second).
- Whether anything outside this repo reads `choir.event` rows from a guest
  store. Nothing was found in code (F-c §9).
- The fork path's store directory (F20).
- What the non-GC 503 bursts are. This design predicts them thinning by
  33 to 38%; the goroutine reading settles it.

## 15. Open decisions, for an agentic-consensus panel or the owner

| # | Decision | For | Recommended default |
|---|---|---|---|
| **O1** | Trace lines leave the signed tape: the nine `StorageTraceLog` kinds become guest-local diagnostic records, with hourly segment digests on the tape. Ruling 18 allows losing a window, not leaving the tape | **owner** | **Yes.** It is the only lever on permanent history and host records, and doctrine `C7` already keeps settlement off Trace. Until ruled, S4 stays on a test computer |
| **O2** | `tool.result` in the log with the cite copy, or kept on the tape | panel, then the owner | **Log with the cite copy.** It is 0.83/s, about 133 KiB/s (69 with F-c), and acceptance is its one production evidence reader. Keeping it on the tape is the conservative fallback. The panel should check that no other reader decides from `tool.result` (6.1 found none) |
| **O3** | Turn F-c deploy 2 on for the paper now, or wait for R7 | panel | **Turn it on now**, as planned. It is built, and it saves about 198 KiB/s at once while R7 takes 1 to 2 weeks. R7 composes with it (7.4). The cost is a third read half for old rows |
| **O4** | The hourly `choir.trace_segment` digest on the tape | panel | **Yes.** 24 small objects a day, and it keeps `I23`'s "existence of evidence" after deletion |
| **O5** | A lasting spend total: an hourly cost rollup on the tape (one acknowledged object an hour, summed from `loop.progress` cost lines), so spending outlives retention (DEC-07, "track spending") | owner | **Defer to S7.** `/api/costs` already reads only the newest 2,000 to 10,000 events, about 30 minutes on the paper today, so R7 does not shorten what it serves |
| **O6** | Retention: 7 days, and a cap of min(2 GiB, 10% of the disk) | owner (on resource use) | **As stated.** Trace evaluation (§35) needs days. S6 holds the 7 days within the cap on the paper |
| **O7** | The ordering barrier (an fsync before each acknowledged append when the log is dirty), or the 1 s timer alone | panel, on F16's measurement | **The barrier**, if its p99 is at most 5 ms on the paper's disk. Otherwise the timer alone, with "a VM crash may leave a gap of at most 1 s before a later tape event" as a named residual |
| **O8** | At disk-critical, drop lines rather than fall back to the tape | panel | **Drop**, counted (4.3). A Dolt write on a full disk is worse |
| **O9** | The segment-log format (custom framing with a `.tidx`) against SQLite | panel | **Segment log** (section 3). SQLite is the fallback if F16's read bars fail |

## Heresy delta

- **Discovered** (by the reader survey on this checkout; not run):
  - Run-state events folded into a lifecycle batch get no `seq` or
    `stream_seq`. `persistLifecycleRunWithEvent`
    (`internal/store/store.go:1321-1345`) puts the caller's event into the
    conditional batch through `buildEventObject` (`lifecycle.go:3328-3336`)
    and never calls `AppendEvent`. `AppendEvent` is the only allocator
    (`store.go:2181-2244`) **[V]**. The survey found the callers, for
    example `loop.started` (`runtime.go:3504`), `loop.completed` (`:3842`)
    and `activation.passivated` (`:3912`, `:3978`), passing events without
    numbers **[H]**. If so, `ListEventsAfter(after=0)` filters them out
    (`seq > after`), so they are missing from `/api/runs/{id}/events`, trace
    export and WS catch-up. The only test reads through `ListEventsOG`,
    which does not filter (`lifecycle_test.go:278`). R7 does not touch the
    fold. This needs its own problem record before any fix (`I11`).
  - `ListEventsByTrajectory` keeps the oldest `limit` of its newest-10,000
    window (`store.go:2421-2450`). F-c recorded the same thing, and R7 keeps
    it.
  - Acceptance evidence today is a URL into the event list
    (`run_acceptance.go:345-356`), not a copy. Any later tape compaction
    that removes the cited event would silently break it.
- **Introduced** (by the design, once built):
  - Trace lines are guest-local diagnostic records off the signed tape,
    tamper-evident per sealed segment, not per line (pending O1).
  - Event reads are a union of up to three sources until old rows are
    retired.
  - A third storage class, `StorageTraceLog`, beside F-c's two.
- **Repaired** (once built): none by this document. On landing: trace
  lines' share of the journal, tape events, host records and permanent
  history on every computer with the setting on.
