# Divergent panel: actor-wake outbox re-arm (a2b87d69 + a7e31232)

Date: 2026-09-30
Panel: agentic-consensus, mode=divergent, 13 lenses
Artifacts: `.agentic-consensus/agentic-consensus-20260930-170033/` (codex, claude,
cursor, omp-gpt6-sol completed; opencode + 3 OMP models unavailable)
Prompt: `.agentic-consensus/wake-rearm-divergent-prompt.md`
Change under review: commits `a2b87d69` (re-mint wake for delivered-unconsumed
pending control) + `a7e31232` (re-arm projected wakes for still-open obligations).

## Verdict

**The fix addresses the unprocessed sub-strand but is a structural no-op for the
consumed-but-stranded case — which is exactly `cfa90b87`.** Independent
confirmation on staging: after the `a7e31232` deploy the migration minted 3
pending wakes (21:14:26), yet `research:cfa90b87` remained `passivated`. Because
`actor.Runtime.Send` calls `kernel.Notify()` unconditionally (`actor.go:174`),
even a dedup'd re-send drains `Unprocessed(agentID)`. Persistent passivation
after a successful mint+notify means the tape row for `4158e48b` already has
`processed_at` set — the 17:12 fenced activation consumed it, then the run
crashed before the pending lifecycle control discharged.

## Why the re-arm cannot deliver (the consumed-but-stranded gap)

- `actorDispatchUpdateID` (`actorruntime/adapter.go:173`) is deterministic on
  `{owner, computer, to, kind, content, trajectory, from}`. For `coagent_result`
  it returns `types.ActorWakeUpdateID` — identical on every re-dispatch.
- `SQLiteLog.Append` is `ON CONFLICT(update_id) DO NOTHING` (`log_sqlite.go:77`).
- Re-arm re-`Send`s the same `update_id`; `Append` no-ops; `Notify()` fires;
  `Unprocessed(agentID)` returns only `processed_at IS NULL` rows → empty → no
  activation. `MarkActorWakeProjected` then re-marks the row, so every log line
  reports success while the cell stays dark.

The obligation is the **pending lifecycle control** (a stored obligation), not
the tape occurrence (transport). Once its bound run is dead *and* its delivery
occurrence is consumed, replaying the same `update_id` can never re-drive it.

## What the fix got right (panel-acknowledged)

- Correctly separates **binding** (`DeliveredToRunID`) from **discharge**
  (consumption) — binding alone is no longer treated as consumed.
- Mute is now keyed to run residency (`Active()`), matching "Passivated doesn't
  own live slots."
- Migration re-arm is CAS-guarded (`ExpectedContentHash`) and mirrors the
  commit-path re-arm branch — one rule across both paths.
- `Send`'s unconditional `Notify()` means the **unprocessed** sub-strand does
  recover: dedup'd append still drains the mailbox.

## Residual risks raised (beyond the consumed-strand gap)

1. **Boot-only recovery.** Re-arm fires at boot migration; a run that passivates
   *after* boot without rewriting its worker_update leaves the control stranded
   until next reboot. (codex #8, claude #4)
2. **`Active()` includes `Blocked`.** A run blocked awaiting the very control
   bound to it is suppressed → potential deadlock, not strand. (claude #9, codex #9)
3. **Fail-open on unresolvable run.** Resolver treats missing-run, read-error,
   and transient failure identically → mints a wake for a possibly-live run.
   (cursor #5, omp-sol #4)
4. **Whole-history re-arm.** Migration re-arms every projected wake whose source
   still derives — revisions, cancel intents — scaling boot CAS writes with
   total history. Mostly dedup'd; a tape compaction could resend all. (claude #3, codex #12)
5. **Cross-object TOCTOU.** Migration conditions only on the existing wake hash,
   not source/run versions; a concurrent consume/rebind can interleave. (codex #4)
6. **Stale comment.** `lifecycle.go:848-851` claims "adapter mints a fresh
   occurrence id per drain" — false for `coagent_result`. (claude #2, cursor #10)

## Reframe options for the consumed-but-stranded class (divergent)

- **Generation-salted update_id.** Include a re-arm generation (or bound-run
  epoch) in the `update_id` so a re-drive mints a new tape row. Loses
  "same logical wake = same row" and the poison-attempt budget; needs a
  consumer-side ack rule. (omp-sol #9, claude B)
- **Fold re-arm into loss-of-residency.** The passivate/terminalize transition
  batch mints a new-generation wake for each pending control bound to the dead
  run — obligation stays open with a fresh transport row. Removes the resolver's
  ambient read. (claude D, codex #8)
- **Consume-at-commit.** Fold binding+consumption into one fenced commit so
  "delivered but unconsumed" cannot exist. Couples run engine to lifecycle
  commits. (claude A, cursor #3)
- **Obligation invariant.** Every open control has a durably-runnable
  continuation, a fenced executor, or an explicit failure disposition — binding
  records ownership, consumption records discharge, projection records only
  transport. (codex #15, omp-sol #3)

## Recommended next action

The `a7e31232` fix landed and is correct *for its scope* (unprocessed wakes).
`cfa90b87` needs a separate repair for the consumed-but-stranded sub-case —
the pending control must be re-driven via a new-generation occurrence or an
explicit rebind/re-drive command, not a replay of the consumed `update_id`.
That is a distinct defect (transport dedup swallows obligation redrive) and
should be documented before patching, per problem-documentation-first.

## Resolution (landed 08a76896)

The consumed-but-stranded gap is fixed: `sweepActorWakeOutbox` and
`enqueueCanonicalLifecycleControlOccurrences` now dispatch through
`dispatchActorRedrive` / `scheduleActorRedrive`. On the redrive path only,
`dispatchAtMode` consults `SQLiteLog.RedriveFamilyStatus`; when the
deterministic `update_id` family is fully consumed it mints
`<base>#redrive-N`. Ordinary dispatch and scheduled wakes keep pure replay
dedup. Verified by
`TestDispatchAtRedrivesConsumedCoagentResultWithSaltedGeneration` and staging
will confirm via `cfa90b87` reactivation on the next boot migration.
