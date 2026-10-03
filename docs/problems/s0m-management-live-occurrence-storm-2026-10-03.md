# S0m finding: persistent-Management live-occurrence storm — substrate dispatch defect

Written 2026-10-03. Mutation class: red (protected surface — persistent
Management reconcile + lifecycle control re-drive + serialized Dolt engine).

## Symptom

On guest `ebfd2e98` (staging), after a VM refresh, the staging HTTP API returns
502 for tens of minutes, one Dolt process sits ~43% CPU, and the journal shows
`persistent Management live occurrence received → bound run=<resident> →
terminal` repeating dozens of times across many distinct trajectories, with
interleaved `redrive-N … deferrals=9,10 cause=defer unprocessed occurrence:
Texture activation`. The stranded-bound probe's released control `1ed83383`
released its claim back to `pending` (release works) but never re-bound inside
the window — its re-drive is queued behind the herd.

## Root cause — verified in source (not hypothesized)

Three compounding substrate facts, each confirmed by reading the code:

1. **One serialized engine.** Every `DoltStore` query path takes
   `engineMu.Lock()` (`dolt_store.go` throughout). It is *not* an overcautious
   lock: the comment documents embedded-Dolt shared-buffer races on concurrent
   queries (`dolt_store.go:62–66`). So a process-wide mutex is currently
   **required for correctness**, and every write/occurrence holds it against
   the HTTP API's reads.

2. **Per-occurrence O(history) resolve.** `ResolvePersistentManagementLiveOccurrence`
   (`management_controller.go:2942`) takes `managementReconcileMu`, then calls
   `listPendingPersistentManagementLifecycleControls` **and**
   `listPendingPersistentManagementDirectives`. Each delegates to
   `ListAllPendingLifecycleUpdates` (`lifecycle.go:1987`) which calls
   `ListPendingLifecycleUpdates` with `limit = maxInt` — an owner-wide
   `ListJSONBodyFieldsByKindOwner` scan over `worker_update` (scanCap) followed
   by a per-pending-row `GetCoagentSourcePacket`/`GetObject`. So **each**
   `coagent_result` occurrence = owner-history scan + N point reads, under two
   serializing mutexes. N obligations × O(history) serialized.

3. **Unbounded re-drive issuance.** `wakeUpdatedCoagent` emits one
   `coagent_result` occurrence per freed packet (`bindTerminalRunOutcome`,
   `research_checkpoint_fallback.go:57-59`), and `scheduleContinuation` mints a
   durable `not_before` row per watchdog. On boot, the ~2009 stale continuations
   plus N freed packets all fire at once. Deferred occurrences
   (`ErrDeferUnprocessed`, "Texture activation") reschedule via `not_before`,
   re-adding to the herd faster than drain — and `deferralBackoff` reportedly
   uses a reset `attempts` counter (fixed ~500ms), not the defer_count, so no
   exponential decay.

This is the **third** live-lock family in the reconcile/redrive path after
`886e5ce1` (exhausted-recast) and `3b0a1ed2`/`b7f59cc9`. Each prior fix returned
a terminal verdict for ONE trigger class; the storm recurs on the next because
the substrate has no convergence invariant.

## Consensus synthesis (divergent panel, 2026-10-03)

Independent analyses by gpt-6-sol, gemini-3.8, gpt-6-terra, and claude-opus on
`.agentic-consensus/agentic-consensus-20261002-233322` (raw outputs in
`.agentic-consensus/mgmt-storm-*.out`). They converge on the mechanism above and
generate ~30 option families across these dimensions:

- **Where scheduling authority lives**: actor tape vs. durable obligation table
  vs. a purpose-built dispatch queue vs. an external broker.
- **Push vs. pull**: per-packet `coagent_result` occurrences vs. a
  level-triggered per-desk drain ("doorbell, not identity").
- **Granularity**: per-packet event vs. one bounded FIFO batch.
- **Storage**: prove Dolt concurrency / keep `engineMu`+fairness / add a
  versioned read projection / replace the store.
- **Overload**: pace/coalesce, one-outstanding-wake gate, or explicit terminal
  fate (`delivery_attempts_exhausted` generalized to a recovery budget).

**Consensus sharpest proposal** (all four independently): the durable
obligation table — not a content-hash re-scan — should be the wake authority.
A `coagent_result` occurrence should be an O(1) doorbell, not an identity that
must be re-matched by scanning all pending rows. Two concrete sub-moves get the
broadest support:

- **Identity-bearing occurrence + O(1) resolve.** Carry the canonical
  `UpdateID`/dispatch key in the occurrence and fetch it by primary key
  (`GetCoagentSourcePacket`), or index `(owner,computer,content_hash)` — either
  removes the N+1 scan. (sol #4/#5, gemini #2, terra #4/#13, claude #7)
- **Bounded drain gate + obligation discharge state.** At most one outstanding
  wake per desk; a reconcile claims an indexed FIFO page and either discharges,
  retries-on-state, or scores `delivery_attempts_exhausted` — never loops.
  (sol #1/#3, gemini #3/#4, terra #5/#6/#7, claude #6/#14)

**Dissent that matters:** gemini + claude warn that an O(1) lookup alone may
only *accelerate the churn* — if bound runs still never consume/terminalize,
faster resolve just spins faster. So the load fix and the
discharge-convergence fix must land together: O(1) resolve AND a discharge
invariant, not resolve alone. Claude also flags that the O(history) amplifier
may be **terminal-row history**, not pending count — pruning/archival may be
required, not just faster scans.

**Hidden assumption to challenge (all four):** should Management reconcile be
actor-occurrence-driven at all? A `sha256:` occurrence that must re-derive its
obligation by scanning is a *second, competing state authority* alongside the
durable pending table — the source of every salted-`#redrive-N` and unbind
sweep. The durable table with a level trigger is the candidate replacement.

## Next boundary (substrate, not per-trigger)

Define the convergence invariant: **every pending obligation must, on
processing, either bind, discharge, or score a bounded terminal fate — never
loop a Management occurrence without changing obligation state, and never scan
history to find it.** Candidate shape (panel consensus): a durable
per-desk dispatch gate (`generation`/`draining`/`retry_at`) + indexed FIFO
claim page + O(1) occurrence resolve + HTTP reads on a versioned projection so
the writer can saturate without 502s. Boot quarantines stale continuations into
a paced drain cursor rather than mass-firing.

## Evidence

- Guest journal `go-choir-vmctl` 2026-10-03 ~01:55–02:20+.
- Trajectory `ffc400c2-d70e-58b3-b00c-c2a0917af6b9` (staging).
- Probe `docs/evidence/s0m-stranded-bound-2026-10-03.json` (carrier cancelled
  → claim released → no rebind in window).
- Code refs: `dolt_store.go:62`, `management_controller.go:1393,2942,3057`,
  `lifecycle.go:1908,1987`, `research_checkpoint_fallback.go:26`,
  `handler.go:472`.
