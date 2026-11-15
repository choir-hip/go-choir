# Lossless coalescing of tape writes: design record (2026-11-12)

Station S5 of [storage solutions part 2](../definitions/choir-storage-solutions-2-2026-11-12.md),
after owner ruling D8. Built on `lane/s5-coalesce` (2026-11-13; see "Built"
at the end): the setting `tape.group_commit` and both loss windows are deleted
and coalescing is always on. The text below is the design as written on
2026-11-12, kept as the record; where it speaks of the setting or of
`run_memory_batch.go` as present, they are gone. Inputs: the
[write census](tape-write-census-2026-11-12.md), the
[group commit design of 2026-10-31](../design/tape-group-commit-2026-10-31.md),
the [switch audit](switch-audit-2026-11-03.md) and
[default-off audit](../problems/default-off-switches-audit-2026-11-03.md), the
code in `internal/store/tape_group_commit.go`, `run_memory_batch.go`,
`projection_tape.go`, and the branch `origin/lane/appender-wedge` (read at
`f191b735c`).

Tags: **[O]** observed in the code or a document, with source. **[I]**
inferred, basis stated.

## 1. The ruling and the two loss windows

On 2026-11-03 the owner ruled `tape.group_commit` deleted with its code: "i
dont like the sound of losing events just before a crash. those are among the
most important events!" [O switch-audit-2026-11-03.md:769-771;
default-off-switches-audit-2026-11-03.md:106]. The code is still in the tree
[O]. On 2026-11-12 (D8) the owner asked "group commit, I decided it off but it
should be on, huh?", read as: coalescing yes, no loss window, no setting, the
two windows stay deleted [O storage-solutions-2 owner_rulings].

The two windows are both acknowledgement-before-durability, behind the one
setting [O tape-group-commit-2026-10-31.md sections 3.3 and 9]:

1. **The telemetry window stretched from 1 s to 10 s.** With the setting on,
   the sealer's window became `telemetryGroupWindow` = 10 s and its clamp
   maximum rose from 2 s to 10 s (`tape_group_commit.go:35`,
   `telemetry_sealer.go`). Telemetry records are acknowledged to their callers
   when queued. A crash within the window drops up to 10 s of trace lines.
   (Telemetry is already crash-lossy by the 2026-10-14 ruling for a 1 s
   window; the stretch is what was new.)
2. **Run-memory `message` entries joined that queue.**
   `run_memory_batch.go`: `message` entries were queued like telemetry, read
   back from a tail cache, and made durable only when the next acknowledged
   write, a read, or the window sealed them. The call returned before the
   entry was on the tape. A crash lost the newest turns of a run, and a
   damaged-run marker (`ErrRunMemoryLost`) existed because a queued entry that
   could not be sealed was lost after its caller had been answered. This is
   the window the owner objected to: the agent record is the most important
   data.

What is not a window, and stays: leader and follower coalescing of writers
that are already blocked (design F1: "a member returns only after the shared
event's CAS and local finalize returned"), and telemetry riding the leader's
event (it shortens telemetry's exposure; it does not lengthen it).

## 2. What "lossless" must mean

A build is lossless only if all of these hold. Each is a test in section 6.

- **L1 No early acknowledgement.** A write call returns nil only after the
  event holding its ops is CAS-committed by the platform, the receipt is
  verified, and the local projection is finalized. No acknowledged write exists
  only in memory. A crash at any instant loses only writes whose callers were
  still waiting, which is today's crash of an in-flight append.
- **L2 Errors are the event's errors.** Every member of a group gets the
  outcome of the shared event. A refusal that pinned nothing
  (`ErrBatchRefused`) splits the group to singles, each with its own answer.
  An unknown outcome is never retried by the group layer (the appender owns
  settling it; section 5).
- **L3 Order.** The ops of a group are in arrival order (riding telemetry
  first, then members). A write issued after another write's acknowledgement
  lands in a later event. Members of one group have no order among themselves
  that any caller can observe. One run's run-memory entries stay serial (their
  `seq` and parent come from the previous entry; the per-run stripe lock
  already holds across the append).
- **L4 The chain is unchanged.** A group is one ordinary
  `projection_batch_recorded` event: one `previous_head` link, one receipt,
  one idempotency key `projection-batch:<event id>`, ops in the existing
  format. Replay applies the ops in order; live finalize and replay agree. No
  reducer, payload format or witness change. Restore granularity becomes one
  group (intermediate heads inside a batch are already not restore targets,
  `projection_batch.go:~99`).
- **L5 No new timer on an acknowledged write.** A lone writer commits at once.
  A group forms only from writers that are already waiting. No linger, no
  window, no quota.
- **L6 Same data on the tape.** The same ops reach the tape; only the number
  of events carrying them falls. Nothing moves off the tape (that is the
  separate decision D3).

Tests that held under the old code and must still hold: no telemetry window
longer than today's 1 s, `runMemoryKindBatched` and the tail cache gone, and a
source scan that no code path returns from a run-memory append before its CAS.

## 3. Where coalescing can happen without holding an acknowledgement

Writers are blocked at three places in `appendOpsAs` today [O census 1.1]:
the write pool (`max_open 1`, 699,568 waits since boot, mean 12 ms), the
telemetry barrier, and the appender mutex `a.mu`, which is held across four
corpusd calls and the finalize. The one natural point is **where the caller
already waits for the appender**: `projectionTape.appendOpsAs`
(`projection_tape.go:158`). The queue exists there; coalescing takes
the waiting writers' ops out of that queue and makes one call for all of them.
Nothing is added to the wait: the caller waits for the commit in flight and
for its own group's commit, instead of for the commit in flight and every
commit queued ahead of it.

The existing implementation does exactly this (leader token, `takeGroup`,
`leadGroup`, `sealRiding`) and is correct for L1 to L6 except for the two
windows, one gap, and one scope question:

- **Delete the windows** (section 1): `telemetryGroupWindow`, the clamp change,
  `run_memory_batch.go`, `runMemoryKindBatched`, the tail cache, the damaged-run
  logic. Run memory then takes the group path with no special case: the per-run
  stripe lock already serialises one run, and different runs queue together.
- **Close the conditional-batch gap** [O census 4.2]: `putBatchChecked`
  evaluates its conditions on committed SQLite state, then `interceptOG` sees
  plain `object_recorded` ops, so `groupableOps` cannot tell a guarded batch
  from an upsert. The design note's F4 ("only unconditional ops group") is
  untrue for object conditions. The condition is checked before the queue wait
  today, so the direct path already has a check-then-append race that a shorter
  queue narrows; but a guarded batch must not share an event with anyone. Carry
  a `conditional` flag from `putBatchChecked` to `appendOps` and send it direct.
  Smaller than re-checking conditions at the leader, and the conservative
  option.
- **Scope of kinds**: keep the closed list (`object_recorded`,
  `object_edge_recorded`, `run_memory_entry_recorded`). Add `desktop_state_recorded`
  and other unguarded upserts only when a counter shows they matter. Retire
  ops, `texture_agent_mutation_recorded` with `ExpectedStates` and the
  self-development ops stay direct.
- **Dry run**: `DryRunProjectionBatch` runs once for the group inside
  `appendDirect` (on the write pool), so a member the dry run refuses fails the
  group before anything is pinned and the split gives that member its own error.
  The dry run validates against committed state, not against earlier members of
  the same group; unconditional upserts do not depend on each other, so this is
  the same as today's.
- **Call-site batching first** [O census 6, row 1]: `appendMessages` and the
  carried-tail loop in `agentcore/run_memory.go` write one event per message.
  Calling `AppendRunMemoryEntries` (the store already writes a run's entries as
  one acknowledged event, `store/run_memory.go:233-264`) from those call sites
  is lossless by construction (one call, one event, returns after the CAS),
  needs no queue, and lowers the base the group size is applied to. It also
  tells us how much of run memory is the carried tail (a census hypothesis, to
  confirm in the run ledger before acting: CLAUDE.md "Read the traces first").

## 4. Expected reduction and the evidence

Base [O census 1.2, 2.2]: the paper appended 129,868 events in six hours on
2026-11-12 (519k a day), 97% under 300 ms after the previous one, the appender
busy 82 to 89% of wall time, p50 gap 142 ms. Of 360,812 events counted by the
guest since boot: run memory 225,061 (62.4%, one entry per event), other
acknowledged single-op objects 73,439 (20.4%), telemetry batches 42,986
(11.9%), acknowledged multi-op events 19,326 (5.4%).

The gap shape (hard floor near 100 ms, no think-time tail) is a queue that is
never empty served at a fixed service time, so there is something to merge
[I]. How much depends on **G**, the number of writers blocked when a commit
ends. The census's arithmetic [I]:

| merge factor G on the 430k single-op acknowledged events | events a day | change |
| --- | ---: | ---: |
| 1 (today) | 519k | |
| 2 | 234k | -55% |
| 4 | 120k | -77% |
| 6 | 82k | -84% |

with telemetry batches (62k a day) going to about zero because they ride, a
realistic range of 100k to 200k, central 130k, and snapshot cadence following
events (a snapshot is due at a 540,000-event tail, so about one per 1.1 days
now and about one per 4 days at 130k) [I census 0.4, 4.5, 4.7].

Two cautions. The 2026-10-31 design predicted only 3% to 35% for acknowledged
writes from a simulation that could not see convoys [O tape-group-commit
section 1], and the census's 75% rests on G, which nothing measured. A
closed-loop effect can also shrink the saving: faster commits let callers do
more work, so ops a day rise and events fall less than the table says.

**The first number to read is now measured.** The counters landed with this
change (`telemetry_batching.appender_depth_at_enter`, `appender_inflight`,
`appender_call_mean_ms`, `tape_events_by_family`, `tape_bytes`,
`corpusd_calls_per_tape_event`; one `tape write summary` log line an hour).
Depth at enter counts callers inside the tape write on arrival, itself
included, so "1" arrived alone. A caller arriving at depth d finds d-1 writers
ahead; the group size at the next commit is about the depth of the newest
arrival minus the commit in flight. Decision rule from the census: build the
coalescer if arrivals at depth 3 or more are at least 60% of arrivals; below a
median of 2, the gain is nearer 40% than 75% and cutting corpusd calls per
event (four to two, census row 3) is the better lever. Read the histogram for
a full day on the paper before building.

Evidence classes for the after-measure (CLAUDE.md "Proof"): events per day from
the host receipts table (metadata) and `tape_events`; per-family volume from
`tape_ops_by_family` (an event that carries riding telemetry or several
writers' families is counted once under `mixed` in `tape_events_by_family`,
so that map no longer reads as a per-family event count); write-pool wait
count and time from `store_pools`; `appender_call_mean_ms`; snapshot spacing
from `snapshot_recorded`; replay seconds per event before and after. Enter the
calibration claims (P(median depth at enter >= 3) = 0.75; P(events a day under
200k after) = 0.7; P(snapshot interval >= 3 days) = 0.6) in
`docs/calibration/claude-ledger.md` before the landing, as the census asks.

## 5. How it sits on the appender wedge lane

`origin/lane/appender-wedge` rewrites the failure half of
`ComputerEventAppender` (`appender.go` +259, new `catchup.go`) and touches
`internal/store/open.go` and `agentcore/runtime.go` for shutdown. It does not
change what a group needs from the appender, and it makes the group layer
simpler. A trial merge of this branch with that one is clean [O git merge-tree].

What the lane gives a coalescer, read from its diff [O]:

- An unclear head CAS error (reset, cancelled caller, 5xx after commit) is no
  longer fatal and no longer discards the prepared row: the appender re-sends
  the same request (`resendUnclearCASLocked`); the host answers a request it
  already committed with the stored receipt. So `AppendNew*` returns nil when
  the event is committed, even if the first reply was lost. A group leader
  sees success and every member returns nil. That is L1 and L2 with no second
  retry in the group layer (the existing F3 rule, "unknown outcome: every member
  gets the error, never retried", only fires when the appender itself could not
  settle it, and the prepared row is kept for `RecoverPrepared`).
- With a receipt in hand, verify and finalize run on a context detached from
  the caller (`detached`, `finalizeTimeout`), so a cancelled caller cannot
  leave a committed event unfinalized. For a group this matters: the leader's
  context is its own `groupCommitTimeout` context, and a cancelled member must
  not fail the finalize of the others (F10 already makes a member that has been
  taken wait for the group's outcome).
- `CASRefusedDefinitely(err)` separates a definite refusal from an unclear one.
  The group layer currently treats every non-`ErrBatchRefused` error as unknown
  and fails all members. That stays correct and conservative. Open: whether a
  definite CAS refusal after pinning should split to singles; a refusal is rare
  and a split would append N times, so the default is no.
- `InterruptDetachedWork` (called from `Runtime.Stop` and `Store.Close`) cancels
  detached recovery. A leader mid-group at shutdown returns an error to every
  member; the planned-restart rules are unchanged (a crash restart closes
  obligations; a marker allows one resume).
- `applyDurableEvent` is the single apply step shared by boot replay and
  in-place catch-up. A group is an ordinary event to it.

Sequencing: S5 depends on S2 (the lane). Land the lane first; the coalescer
then changes only `internal/store` (`projection_tape.go`,
`tape_group_commit.go`, `run_memory.go`, `agentcore/run_memory.go`) and does
not edit `appender.go`. The counters of this change deliberately touch neither
`appender.go` nor `catchup.go`: corpusd calls are counted in
`HTTPClient.doWithResponseLimit`, and depth is counted in `appendOpsAs`.
Three interactions to keep in mind:

1. **Blast radius.** One failed group fails every member where one caller
   failed before. The 2026-11-12 wedge came from one lost reply in more than
   500k CAS a day; with the lane's re-send and 4x fewer events the exposure is
   lower on both counts [I].
2. **`HoldAppends`** (replay completeness, checkpoint restore) holds `a.mu`; a
   leader waits on it like any append. No change.
3. **Prepared-row recovery** sees one prepared row per group. Recovery is by
   event, not by writer, so a group needs nothing new.

## 6. Failure cases a build must test

Keep the existing F1-F15 and W-series tests that remain meaningful; delete the
W tests with `run_memory_batch.go`. Add or re-aim:

| # | Case | Expected |
| --- | --- | --- |
| C1 | A writer is answered before its CAS | impossible: assert in a fault harness that no member returns while the CAS is held |
| C2 | Crash (kill the process) during a group commit, at each of: before pin, after payload pin, after event pin, after CAS before finalize | no acknowledged write lost; restart settles the prepared row; members were all still waiting |
| C3 | Crash between two groups | every write acknowledged before the crash is on the tape and in the projection |
| C4 | `ErrBatchRefused` from the dry run for one member | split to singles in arrival order; the refused member alone fails; riding telemetry sealed first |
| C5 | Unknown CAS outcome, with the wedge lane's re-send succeeding | every member returns nil; one event |
| C6 | Unknown CAS outcome, re-send fails | every member gets the error; prepared row kept; no second append; the next append recovers |
| C7 | Definite CAS refusal after pinning | every member gets the error, nothing retried, prepared row discarded |
| C8 | Member cancelled before it is taken | removed, nothing written, `ctx.Err()` |
| C9 | Member cancelled after it is taken | waits for and returns the group's outcome; event finalized |
| C10 | Causal order across goroutines: B issued after A returns | B lands in a later event than A |
| C11 | Order inside a group | ops in arrival order; same key twice: the later wins, as in two direct writes |
| C12 | Group caps (256 ops, 1 MiB) | an oversized write goes alone; a group never exceeds the caps; the pin size limit is not hit by a group of large tool results |
| C13 | Conditional batch (`PutBatchConditional`) with a concurrent upsert to the same key | goes direct; its condition outcome is the same as with no coalescing |
| C14 | Guarded kinds (retire, `texture_agent_mutation_recorded`, self-development) | never grouped |
| C15 | Run memory: two entries of one run | serial, `seq` dense and parent chain intact; entries of different runs share an event |
| C16 | Run memory carried tail through `AppendRunMemoryEntries` | one event per call, returns after the CAS |
| C17 | Telemetry riding | records ride in enqueue order ahead of members; the window is 1 s; a refused ride is sealed alone first |
| C18 | Barrier and leader deadlock under load (race detector, 32 writers, telemetry on) | no deadlock; lock order flush mutex then appender mutex |
| C19 | Replay equivalence | replay of a tape with grouped events gives the live projection (`equivalent` probe); stamped `pos` agree |
| C20 | Idempotency | a re-sent group request returns the stored receipt; no duplicate event or `stream_seq` across a crash boot |
| C21 | `HoldAppends` during a group | leader waits; no member returns early |
| C22 | Shutdown (`InterruptDetachedWork`, `Store.Close`) mid-group | members get an error, none is acknowledged; the store closes within its deadline |
| C23 | Planned restart | members are synchronous, nothing to drain; telemetry drain unchanged |
| C24 | Latency | a lone writer's latency equals the direct path's; p99 with 8 concurrent writers is bounded by about two group cycles, not queue length |
| C25 | Counters | `tape_events` falls and ops per event rises; `tape_events_by_family` still sums to events |
| C26 | Setting removal | no `tape.group_commit` key in the registry, the boot path or the docs; a stored value is ignored, not an error |
| C27 | d@e drills after landing | chaos s1-s3, restore from snapshot plus tail, local head equals platform head, no event lost (the S4/S5 acceptance in the goal file) |

## 7. Open questions

1. **G is unmeasured.** Read `appender_depth_at_enter` on the paper for a day
   before building (section 4). If the median depth is 1 or 2, change the order:
   call-site batching and fewer corpusd calls per event first.
2. **Is the carried tail the run-memory doubling?** Census hypothesis
   (`carryActorConversation` rewrites the tail as one event per message on each
   rewarm). `run_memory_ops_by_kind` splits kind and reason; confirm in the run
   ledger.
3. **Definite CAS refusal: split or fail the group?** Default fail (section 5).
4. **Do conditional batches need to wait for the leader's state?** Default:
   direct. A leader re-check would let them share an event; not worth it unless
   the counters show conditional batches are a large share.
5. **Replay time per event with larger payloads.** Events fall, payload per
   event grows; measure replay seconds before and after (census 4.7).
6. **Closed loop.** If the paper is throttled by the appender, ops a day rise
   after the change. The success measure is events a day against ops a day, not
   events alone.
7. **Owner confirmation on supersession.** This ships the leader/follower part
   of what the 2026-11-03 ruling deleted, and keeps the ruling for the windows.
   The change must say so. D8's default was "ship coalescing only on the owner's
   word"; the owner's reading in `owner_rulings` is yes, but the build should
   quote it.

## First reading from the paper's counters (2026-11-13T11:30Z)

Counters landed as e392ac824. Read from the paper's
`/api/runtime/observability` about 17 minutes after its 11:13Z boot
(4,785 tape events; one short window, not the day the design asks for).

| measure | paper | d@e (07:54Z, 164 events) |
|---|---|---|
| appender depth at enter: 1 | 179 (4%) | 139 (84%) |
| 2 | 314 (7%) | 24 |
| 3 to 4 | 1,054 (22%) | 2 |
| 5 to 8 | 2,790 (58%) | 0 |
| 9 to 16 | 453 (9%) | 0 |
| mean time in the appender call | 982 ms | 63 ms |
| corpusd calls per tape event | 6.0 | 7.2 |
| ops per tape event: exactly 1 | 4,155 of 4,785 (87%) | 109 of 164 |

What it says:

- On the paper a writer almost always arrives behind others: 89% of appends
  enter at depth 3 or more, most at 5 to 8, and each waits about a second.
  Coalescing the writers already queued would cut the paper's event count
  several-fold, which is what claim C57 forecast from the census. On d@e the
  queue is empty, so d@e cannot measure the gain; the proof has to read the
  paper.
- Each tape event costs six corpusd calls: one append, one payload, two head
  reads and two pins (9,663 heads and 9,659 pins for 4,829 appends). The two
  heads and two pins per append are a second saving, independent of
  coalescing.
- 2,292 of the run-memory ops are `message` and 245 are
  `cell_fate/tray committed`.

Two defects the counters exposed:

- The run-memory key carries free text from the entry's reason: the paper
  shows `cell_fate/reflect: call of reflect.Value.IsNil on string Value` and
  `cell_fate/reflect: slice index out of range`. The key should come from a
  fixed list (the review of e392ac824 noted the reason is not bounded).
- Those two keys are reflect panics inside a cell on the paper. Not traced
  here; an Autopaper finding.

Remaining error: one 17-minute window just after a boot, when queues may be
deeper than in steady state. Read again after a day before fixing the shape.

## Built (lane/s5-coalesce, 2026-11-13)

Coalescing is the behaviour of every store; there is no setting.
`tape.group_commit`, its registry entry, `telemetryGroupWindow`,
`run_memory_batch.go` (the run-memory window, tail cache and damaged-run
marks), `internal/agentcore/tape_group_commit.go` and the planned-restart
window stamp are deleted. A marker written by an older build with a failed
drain under an open window is still read once, so that boot closes its runs.
The opening and carried run-memory messages are one write at the call site.
A guarded batch (`PutBatchConditional`, `PutBatchSegments` with conditions)
takes the direct path through a context flag from `objectgraph`.

Counters: `tape_events_by_family` counts an event once, under its writers'
common family or `mixed`; `tape_ops_by_family` counts every writer's ops under
that writer's own family (riding telemetry as `telemetry`), and sums to
`tape_ops`. The run-memory key takes a reason only from a fixed list; any other
is `<kind>/other`.

Corpusd calls per event, by reading the code: the two head reads were one
read used twice (bind, then `appendLockedInner`); the append reuses the bind
read (`boundHead`). A head that moves between the bind and the CAS fails at the
CAS, as the old read would have failed the reducer. The two pins stay: the
payload pin receipt is bound into the request commitment, the event body holds
that commitment, and the event pin carries the body, so the event pin cannot
precede or merge with the payload pin; the host checks both receipts. One call
per pin needs a corpusd endpoint that pins a payload and an event together.
The finalize `payload` fetch (the sixth call on the sealed path, where the
live plaintext cache of `AppendNewPayload` is not kept by
`AppendNewPrivatePayload`) is a further saving this change does not take.

`telemetry_ridden` (in `group_commit`) is a lower bound on the telemetry that
rode: it counts only records whose carrying event committed.
