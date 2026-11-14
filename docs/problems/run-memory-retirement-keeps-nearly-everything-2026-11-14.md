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
  a durable string on every entry. One-shot workers have other agent ids and
  are never a carry source.

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

## R-1: the smallest correct rule change

1. **Fix the lookup.** `selectRunMemory` reads the run record through a
   helper that tries the owner-scoped id, and on a miss reads the run by its
   `run_id` metadata (indexed) and takes the owner's non-tombstoned record
   (newest `updated_at` when several). It reads, never writes.
2. **A run with no record after that retires** once its newest entry is past
   the window. Nothing reads such a run: carry sources and resumes need its
   record. The disposition is `retired_no_run_record`. Seven days is the
   guard for a record written after its first entry.
3. **Newest-two protection narrows to document-custodian agents**
   (`deskmanifest.IsDocumentInbox(agent_id)`), still the newest
   `runMemoryKeepPerAgent` by newest entry. Every other agent's runs are
   judged by state and finish time alone. `agents_with_one_or_two_runs` keeps
   its meaning (all agents); the overlap block counts protected runs.
4. Everything else is unchanged: terminal state, finished past the window,
   cursors never retired, the explicit run list on the op (replay removes the
   same rows), 5,000 runs per pass, 10 per op, duty-cycle rest.
5. **Catch-up.** When a pass names the full cap and left more runs
   (`not_reached_over_cap` > 0), the same daily wake runs the next pass after
   a short gap, repeating until a pass retires nothing or no run was left over.
   Each pass keeps its own duty-cycle rest between ops; the pass's group read
   is a covering-index scan. No setting.

As built: `runRecordForRetirement` (owner-scoped id, then `run_id` metadata,
owner's live copies only, a non-terminal copy wins); `retired_no_run_record`
replaces `kept_run_record_not_found`; `kept_run_record_unreadable` keeps a
record that does not decode; the protected set is built only for agents where
`deskmanifest.IsDocumentInbox(agent_id)`; `RetireRunMemoryBacklog` repeats
passes (one minute gap) until a pass retires nothing or leaves no run over the
cap, and `StartTelemetryRetirement` calls it in place of one pass.

Expected on the paper if belief 1 holds: of the 60,451 + 2,468 runs held by
the newest-two rule, only document agents' runs remain protected; of 44,792
no-record runs, those with a terminal record past the window retire
(`retired`) and any still unseen retire as `retired_no_run_record`. Backlog is
about 2.94 M retirable entries (1,536,498 + 62,479 + 1,339,446) in 107,711
runs (60,451 + 2,468 + 44,792): 22 passes at 5,000 runs. Upper bound: runs of
document custodian agents (at most two each) and runs whose record is
passivated or blocked stay, and the census does not say how many.
Each pass is paced at least 250 ms per op of 10 runs, so 500 ops is two to
ten minutes; back to back that is a few hours, not 21 days.

## Rollback

Revert the commit. Retired entries stay retired (each is on the sealed tape
and in the retire op's explicit run list); a replay of the same tape reaches
the same store either way.
