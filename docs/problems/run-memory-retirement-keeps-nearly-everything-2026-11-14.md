# Run memory retirement keeps nearly everything (2026-11-14)

## Problem

On the paper, `run_memory_entries` is at least 11.2 GB, with 2.5 GB of indexes,
in a 47.5 GB guest store. The daily retirement job
(`internal/store/run_memory_retire.go`) removed 4 runs on its last full pass.
The rule keeps a run's entries unless the run has a non-cursor entry, its newest
entry is older than 7 days, it is not among the newest 2 runs of its agent, its
run record is found, the record says completed, failed or cancelled, and the
run finished more than 7 days ago.

## Evidence

Census (`run_memory_retirement` on the observability endpoint, commit
79dd71b57), paper, 2026-11-14T08:59Z, one pass over 151,550 runs,
5,135,711 entries, 64,958 agents (51,803 with one or two runs):

| disposition | runs | entries | retirable entries |
| --- | --- | --- | --- |
| kept_newest_entry_in_window | 39,893 | 2,005,236 | |
| kept_newest_two_of_agent_7_30d | 60,451 | 1,621,515 | 1,536,498 |
| kept_newest_two_of_agent_over_30d | 2,468 | 78,641 | 62,479 |
| kept_run_record_not_found | 44,792 | 1,388,005 | 1,339,446 |
| kept_only_cursor_entries | 3,938 | 20,229 | |
| kept_finished_in_window | 8 | | |
| retired / passivated / blocked / unreadable / over cap | 0 | | |

Two exclusions hold 57% of all entries: "newest two runs of the agent" and
"run record not found". Code reading (file:line at commit 79dd71b57):

- The record lookup is `GetRunByOwnerOG` (`graph_store.go:678`), which builds
  the canonical id from owner and run id only (`getRunObjectByOwnerOG`,
  `graph_store.go:633`). A run bound to a trajectory is written by the
  lifecycle path under a computer-scoped canonical id
  (`lifecycleCanonicalID`, `lifecycle.go:445`: suffix is
  `StableSuffixFromKey(computerID + NUL + runID)`; written at `lifecycle.go:3398`,
  `store.go:421` `persistLifecycleRunWithEvent`). The retirement lookup
  never builds that id, so every lifecycle run reads as "not found". Other
  callers know the trap: `unionRunLabelsOnce` (`store.go:2465`) falls back to
  `lifecycleGetObject` with the computer id. Run entries carry no computer id.
  The run record's metadata does carry `run_id`, an indexed column
  (`objectgraph/sql_store.go:40`), and the existing tests seed runs only with
  `CreateRunOG` (legacy ids), which is why nothing caught it.
- The newest-two protection (`run_memory_retire.go:156-170`) is for the carry
  of a Texture conversation. The only carry is `carryActorConversation`
  (`agentcore/run_memory.go:164`), gated at `run_memory.go:146` by the
  `actor_rewarm_carry_conversation` flag and the document custodian profile,
  which `texture_agent_revision.go:751` sets for a document's agent. The
  source is `LatestActorRunMemoryEntries` (`store/run_memory.go:292`), which
  lists run records of that agent id and takes the newest completed or
  passivated one holding entries. The agent id of every such agent is
  `deskmanifest.DocumentInbox(docID)` (`deskmanifest/document_custodian.go:21`),
  a durable string on the entries of a run (not on every one: a
  `texture_cell_author_receipt` was written with no agent id, now fixed, and the
  group query takes `MAX(agent_id)` so a stored empty id cannot hide it).
  One-shot workers have other agent ids and are never a carry source.

## Belief

1. "Run record not found" is a lookup bug, not absent records. Nearly all
   44,792 runs have a lifecycle record the lookup cannot see. Confidence 0.8;
   the remainder after the fix is unknown until the paper is read.
2. The newest-two rule only matters for document-custodian agents. For every
   other agent no reader needs an old run: the rewarm source named in run
   metadata (`actor_rewarm_source_loop_id`) is set only from
   `LatestTextureActorMemorySourceRun` (a document agent); `actor_resume_source_loop_id`
   resumes a passivated run, which retirement never touches; `get_run_memory_entry`
   reads only the caller's own run (`tools_evidence.go:57-64`); lifecycle
   `SourceRunID` and management/engineering readers read a live or
   just-finished run, inside the 7 day window.
3. A retired run's memory asked for later answers an empty list
   (`ListRunMemoryEntries`) or not-found (`GetRunMemoryEntry`); the tape still
   holds the entries.

## Remaining error

- Whether the paper's no-record runs resolve after the lookup fix is a
  prediction (C85 in the calibration ledger).
- A Texture agent whose newest two runs are failed or cancelled would have its
  carry source (newest completed) unprotected. This exists today and is not
  changed here; it is named residual.
- Agents ids that look like document inboxes but are not carried from are
  kept (two runs each); that is bounded by the document count.

## R-1: the rule as repaired after round 1 (both seats)

1. **The lookup** (`runRecordForRetirement`) reads through the read pool, never
   the single write connection. It collects the owner-scoped copy and every
   copy whose `run_id` metadata names the run (one page; a second only when the
   first is full), keeps the owner's live copies whose own run id matches
   (a tombstoned or mismatched owner-scoped row is a miss), and decides: a
   copy that does not decode makes the record unreadable (kept); a copy that
   is not terminal wins; among terminal copies the newest `FinishedAt`, else
   `UpdatedAt`. Reads are memoized for the pass.
2. **Carry sources.** For each owner's document custodian agent the newest
   two runs whose record is `completed` or `passivated`, ordered by finish as
   `LatestActorRunMemoryEntries` orders them, keep their entries. Failed,
   cancelled and record-less runs take no slot; an unreadable record is
   protected. `LatestActorRunMemoryEntries` now skips a run holding only the
   kept cursor rows.
3. **No record** retires when the newest entry is over 30 days old (recorded
   runs: 7 days). Absence is not proof of state. The hold can be lowered after
   one census on the paper shows what remains of the runs the old lookup could
   not see. Its ops carry their own reason.
4. **Group query** takes `MAX(owner_id)` and `MAX(agent_id)`; the covering index
   is still the only read (the M8 test pins it). The cell-author receipt now
   carries the run's agent id.
5. **Catch-up** (`RetireRunMemoryBacklog`): passes back to back with a one
   minute gap, at most 32 per wake (the paper's backlog is 22 passes of 5,000
   runs; 32 is half again), `ready()` re-checked between passes, and an error
   in the backlog is logged and the partition GC, incremental vacuum and
   compact copy go on. The census keeps the wake's first pass whole and the
   retired totals of every pass.

Named residual: a late management report for a producer run whose memory was
retired is refused. `persistentManagementHistoricalReportRunStateAllowed`
(`lifecycle.go:3856-3859`) admits any terminal run with no age bound and
`lifecycle.go:3933` reads its memory to authenticate the delivered packets; if
that run finished more than 7 days ago and its memory is retired, the report
gets `ErrLifecycleInvalidTransition`. Reachable today only if a report arrives
days after its run ends; the lifecycle behaviour is not changed here.

### Expected on the paper under the repaired rule

From the census above (belief 1 holding). Upper bounds, the census does not say
how many runs are document agents' carry sources or passivated or blocked:

| class | runs | retirable entries | repaired rule |
| --- | --- | --- | --- |
| newest-2, 7 to 30 d | 60,451 | 1,536,498 | retire if the agent is not a document custodian and its record is terminal and finished over 7 d ago; document agents keep two completed or passivated runs |
| newest-2, over 30 d | 2,468 | 62,479 | the same |
| record not found | 44,792 | 1,339,446 | recorded after the lookup: the same state rules; the rest (no record) wait for a newest entry over 30 d, so up to those with an entry in the 7 to 30 d band stay one more cycle |

Total upper bound unchanged: 107,711 runs, 2.94 M entries, 22 passes of 5,000
inside one wake (cap 32). The 30 day hold on record-less runs can only lower it.
