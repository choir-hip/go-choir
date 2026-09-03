# Problem Receipt: Sleep-After-Turn Can Mint Unfinalizable Batches (2026-09-03)

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); repair is red
- Status: open, unfixed. Discovered during the seq-138612 outage; confirmed by
  code inspection + independent consensus panel (11 agents,
  `.agentic-consensus/store-review-20260903/`).
- Blocker this unblocks: none active (replay tolerance of
  `internal/store/project.go` keeps boots alive); the generator below can mint
  the next poison at any time.

## The Problem

`SleepAgentMutationAfterTextureTurn` (`internal/store/texture.go`, ~L2530)
appends a sleep op with `require_revision: false` whenever the recorded turn
outcome is non-revision — without checking whether the mutation row already
carries a `revision_id`. If it does (e.g. pre-allocated at creation, seq
138555 pattern), the batch CASes onto the canonical tape and then fails
`FinalizeBatch` forever: prepared-but-unfinalizable. Live fails closed
(correct); every future boot replay dies on it (outage). The replay tolerance
(`textureRevisionPresenceConflict`) keeps the computer booting, but the next
identical sleep appends the next poison.

Contributing asymmetries (consensus-verified in tree):

- Tape path (`texture.go:2532`) vs direct-SQL path (`:2544`, requires
  `revision_id <> ''`): the two writers disagree about which rows may sleep.
- `commitDoltCheckpoint` per finalized event (`computer_events.go:177`) and,
  worse, before history READS (`texture.go:1336` "vm state checkpoint before
  texture history read") — reads grow the store; 138k tape rows / ~6k commits
  is the commit-volume driver behind the 9.8 GB journal.
- `GetHistory` (`texture.go:1314-1401`) resolves bodies via
  `dolt_history_og_objects` + per-revision `AS OF`, coupling feature
  correctness to chunk retention and locking out history compaction.

## Repair Direction (not yet implemented)

1. Pre-CAS dry-run: finalize the batch in a rolled-back transaction before
   `appendOps` dispatches; abort to the caller before the platform accepts an
   unfinalizable event. (Strongest consensus item: "every live-accepted event
   must replay" as CI invariant + runtime assertion.)
2. Sleep-path predicate parity: the after-turn tape path must enforce the same
   revision precondition as the SQL path (or explicitly handle
   revision-carrying rows), with a unit test that appends nothing for the
   seq-138612 shape.
3. Commit discipline: checkpoint at macro-boundaries (never on reads); batch
   projection writes; record covered tape range in checkpoint metadata.
4. History without `AS OF`: immutable revision bodies + parent chain are
   already content-addressed — resolve history through indexed rows; reserve
   `AS OF` for audit. Unblocks unambiguous GC.
5. GC that can fire: host-side offline GC of stopped/sealed images on
   milestone crossing (the manual 9.8G→1.7G runbook), driven by a bloat-factor
   gauge (dir/live > 3x); in-guest GC only below a memory-derived bound, never
   a fixed 5 GiB; a skip must emit an observable event, not `log.Printf`.
6. Scan budgets: no full-body object snapshot on boot passivation or runs-list
   (indexed columns, keyset pagination, background-after-ready); boot scans
   must fit `VM_BOOT_READY_TIMEOUT` on the default shape or it is a sizing
   bug.
7. Hold authority: derive the boot hold bit from ownership at `bootVM`
   (single authority), plus a boot-arg XOR ownership assertion probe.

## Residuals Until Repaired

- Any `SleepAgentMutationAfterTextureTurn` on a revision-carrying pending
  mutation appends another 138612-class poison (boot survives via tolerance;
  the prepared row still needs attention).
- Store re-grows ~10x without (5): the 8 GiB guest is headroom, not hygiene.
