# A test computer's replay is not equivalent in self_development_operations (2026-10-23)

Date: 2026-10-23, 07:15 UTC. Found by: FP0, running D10's proof (F-c deploy
1, `a49ae398a`) step 1 and step 5 on test computer computer-71afbf14…
(batching-e2e-1022, class test). Status: open, problem record before any
fix. Mutation class of a fix: red (tape and evidence, self-development).

## Evidence

`GET /api/computers/<id>/self-development/replay-completeness` on
computer-71afbf14 answered in about 20 s, at 07:10 and again at 07:11
after an operator refresh (crash boot). The full report is
`_runs/deployD10/t71-replay.json`.

- `eligibility`: `{manifest_version: 4, eligible: false, reason:
  "reconstructed projection is not equivalent to live state"}`
- `result.status`: `not_equivalent`, with two differences, both
  `kind: dolt_head`:
  - `dolt:texture:content_root`, live `sha256:441f795b…` against
    reconstructed `sha256:0117656…`
  - `dolt:texture:table:self_development_operations`, live
    `sha256:47c0cd36…` against reconstructed `sha256:0bc2031b…`

Nothing in the report names `choir_events`. D10's own checks pass: the
manifest is version 4, and the empty table is neither unknown, missing nor
observed.

## Not caused by D10 (from the diff)

D10's diff (`663023143..a49ae398a`, 32 files under `internal/`) touches
neither `self_development_operations` nor `internal/selfdev`. No reading
from before the deploy exists for this computer, so it is not yet proved
that the difference predates D10.

## Hypothesis (from code, not yet confirmed)

`internal/selfdev/operations.go` writes `self_development_operations` with
raw SQL:

- inserts at lines 263, 385 and 432;
- a state-machine `UPDATE` at line 495;
- `UPDATE ... SET last_intent_error=?` at line 524.

The tape projection also inserts rows (`internal/store/project.go:432`). If
any of these direct writes has no tape event that the projection replays,
the live table and the replayed one diverge. `last_intent_error` and
`updated_at` are likely candidates. A replay that is not equivalent makes
the computer replay-ineligible, and an ineligible computer refuses
checkpoint publication: self-development apply, platform-follow's
post-apply checkpoint and owner recovery checkpoints. So this bears
directly on self-development on real computers.

## Open

- Which rows and columns differ: compare the live table with a replayed
  copy, by operation_id and column.
- Which write paths have no tape event.
- Whether the paper and the owner's computer are also ineligible. The
  paper's report did not answer in 10 minutes (it replays the whole
  chain). The proof should not run that report on the paper: see the D10
  ledger entry.
- Whether the right fix is to put the write on the tape, or to classify the
  column as live-only in the witness manifest.

## Heresy delta

- Discovered: a table in the replay witness has writers outside the tape
  (hypothesis). A read-only proof step defaults to the paper and runs a
  full replay there.
- Introduced: none.
- Repaired: none.

## Investigation (2026-10-23, lane fp0-selfdev-ops-replay; read-only, no fix)

Status after this section: the hypothesis is confirmed for one write path
and reproduced locally. Row-level confirmation on computer-71afbf14 is
still open (the report carries hashes only).

### Write map

`self_development_operations` columns, as written by the live path and as
reproduced by the tape. The projection is
`projectSelfDevelopmentOperation` (`internal/store/project.go:385-460`),
fed by `computerevent.SelfDevelopmentOperationProjection`
(`internal/computerevent/projection_batch.go:106-137`) which has 27 body
fields and no `last_intent_error`. The projector's `INSERT ... ON DUPLICATE
KEY UPDATE` (`project.go:432-455`) writes the 27 columns and no
`last_intent_error`; the column keeps its DDL default `''` on replay and
survives any projected update on the live side.

| Write site | What it writes | On the tape in production? | Reproduced by replay? |
|---|---|---|---|
| `selfdev/operations.go:263` (`Start` insert) | all request columns | Yes. Guarded: `if s.projection != nil` at `:258` returns through `appendOperationProjection` (`:545`). The insert is the unbound test seam only | Yes |
| `operations.go:385` (`RecordAppliedBaseline` insert) | genesis baseline row | Yes. Same guard at `:380` | Yes |
| `operations.go:432` (`StartRollback` insert) | rollback row | Yes. Same guard at `:428` | Yes |
| `operations.go:495` (`Transition` UPDATE) | 22 columns incl. `last_intent_error`, `updated_at` | Yes. The projection branch at `:466-485` returns first; the UPDATE is the unbound seam only | Every column but `last_intent_error`. In projection mode `Transition` carries the old stamp in memory but `appendOperationProjection` drops it; harmless, because the projector never touches the column, so live keeps it |
| `operations.go:196` (`BindStartIntent`) | `self_development_start_intents` | Yes, same pattern (`:187`) | Yes, all four columns (`project.go:363`) |
| **`operations.go:524` (`RecordIntentError` UPDATE)** | `last_intent_error`, `updated_at` | **No. No `s.projection` branch exists; it runs as raw SQL in production** | **No. Neither column, as of this write** |

Production binds the sink: `runtime.go:386-390` and `rematerialize.go:268-272`
call `BindProjectionSink` when `ProjectionTapeBound()`; the tape is bound at
`internal/autoputer/run.go:428`. t71's replayed table is non-empty, which
fits a bound sink.

Callers of `RecordIntentError` in production: `rlm_reduce.go:1502` (a
refused or rejected freeze stamps a reason; a successful freeze clears a
prior stamp), `rlm_reduce.go:1543` (verify refusals and the clear after a
passing verdict), `self_development_standing_consensus.go:70`. Nothing else
assigns `LastIntentError` (no `Transition` mutate does). Consumers of the
value: `obligations_surface.go:197`, `engineering_assignment_fate.go:1673`
(the terminal reason text), `selfdev_release_witness.go:547`, and the freeze
and verify reducers above.

Two columns differ, both from the one writer:

1. `last_intent_error`: live holds the stamp, replay holds `''`.
2. `updated_at`: `RecordIntentError` sets it to its own `now`. If a later
   projected update follows, the tape overwrites it and the two agree again;
   if the stamp is the last write to the row (the common case for a
   computer whose operation stalled in `executing`), live `updated_at` is
   later than the tape's. A clearing write (`reason == ""`) moves
   `updated_at` alone, so even a clear leaves the row inequivalent.

Other texture-database tables the witness hashes (surveyed by a Sonnet
subagent, then spot-checked in the code; per-table hashes on t71 agree for
every other table). Every direct write below is the unbound fallback behind
a tape-bound guard, or a boot or migration step:

- `desktop_workspaces`, `desktop_app_instances`, `desktop_window_placements`:
  `desktop_live.go:70-72` returns through the tape first; SQL at `:96-260` is
  the fallback. `desktop_sessions`: written only by `project.go:201-210`.
- `texture_document_aliases`: `texture.go:782` guard, SQL `:800` fallback.
  `texture.go:887` (`DeleteTextureAliasesByOwner`) has no guard and no
  non-test caller; `:876` is test-only.
- `texture_agent_mutations`: `texture.go:1868, 1986, 2027, 2062, 2099, 2172,
  2209` are each the `else` of an `if s.projectionTape != nil`.
- `run_memory_entries`: `run_memory.go:134`, tape branch `:53-91` returns first.
- `og_objects`, `og_edges`: `objectgraph/dolt_store.go:684, 1060, 1224, 1889`
  run only when `s.intercept == nil`. `vocab_migrate_og.go:770-793, 1046-1056`
  are boot migrations that replay reruns.
- `computer_event_index`, `computer_event_projection_heads`: appender
  bookkeeping; the timestamp columns are in `IgnoredContentColumns`
  (`replay_completeness.go:495-504`).

### Reproduction

`internal/agentcore/replay_selfdev_operations_test.go`,
`TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence`.
It builds a tape-bound runtime (the derivable chain, so projection batches
are pinned and fetchable), starts an operation, moves it to `executing`
through `Transition`, and runs `ReplayCompleteness` as a control: it is
equivalent. It then calls `RecordIntentError` and runs the probe again. Two
subtests (a refusal reason; an empty clearing write). Both fail today with
exactly the t71 difference set:

```
differences [dolt:texture:content_root dolt:texture:table:self_development_operations]
eligibility: Eligible:false  reason: reconstructed projection is not equivalent to live state
live   row: last_intent_error="freeze: refused (no verifier refs)" updated_at=...411845
replay row: last_intent_error=""                                   updated_at=...146107
```

The clearing subtest shows `last_intent_error=""` on both sides and only
`updated_at` apart, which is the second column above. Run:
`go test ./internal/agentcore -run TestSelfDevelopmentIntentErrorStamp -count=1`.

### One cause or two for `content_root`

One. `content_root` is `sha256` over the sorted per-table content hashes
(`computerversion/dolt_state_extractor.go:222-232`). From the 23 per-table
`dolt:texture:table:*` values in `t71-replay.json`, I recomputed both roots:
the live set gives `sha256:441f795b...` and the replay set gives
`sha256:01176564...`, each equal to the root the report states. The two sets
differ in `self_development_operations` and nowhere else, and every
`dolt:texture:schema:*` value matches, so `content_root` has no second
cause on t71. The `run_memory` comparison in the report is also clean (5382
live, 5382 replay, 0 different).

Not shown: which operation rows and columns differ on t71. The report holds
hashes only. A read of the live table on t71 (`operation_id, state,
last_intent_error, updated_at` for each row) would confirm the stamp and
`updated_at` pattern; that is FP0's, not mine (no Node B).

### Which fix fits

Recommended: put the stamp on the tape. Reasons, from doctrine and from how
the witness is used:

- Doctrine `C8` and the Dolt row of the authority table
  (`choir-doctrine.md`, event projection authority): the per-computer
  appender owns semantic ordering, and VM-local Dolt state is a projection
  of the tape that must be reconstructible. The operation record is the
  owner-visible surface of why a candidate is blocked (the comment on
  `Operation.LastIntentError`), and three production consumers read it, so
  it is state, not a log line.
- Classifying it live-only means adding `last_intent_error` (and, because
  of the second column, `updated_at`) to `IgnoredContentColumns`, in both
  construction sites (`replay_completeness.go:489-505` and
  `projectionbase/witness.go:21-35`). That changes the row encoding of
  every non-empty `self_development_operations` table, so the content hash
  of every computer with at least one operation row, and the witness
  recorded in every existing checkpoint, no longer match. Rematerialize
  extracts the reconstructed witness under the current recipe and compares
  it with the checkpoint's recorded one
  (`rematerialize.go:~190, WitnessContentMatches`), so older checkpoints
  would become unrestorable. The F-c design avoided exactly this for
  `choir_events` (`computerversion.OptionalTables`). A restore would also
  drop the stamp silently, since replay cannot produce it.
- Live-only is also the weaker claim: it declares a column outside the
  reproducibility guarantee that the replay gate exists to enforce.

Shape of the fix (not built): add `last_intent_error` to
`SelfDevelopmentOperationProjection` (additive; old events decode to `''`),
to the projector's existing-row `SELECT`, the `INSERT ... ON DUPLICATE KEY
UPDATE` list and `sameSelfDevelopmentOperationProjection`; have
`appendOperationProjection` send `operation.LastIntentError`; give
`RecordIntentError` a projection branch beside `Transition`'s, as a
state-preserving update under `writeMu` (`ExpectedState` = the row's current
state, which the projector already accepts). Decision to take: whether a
stamp should keep bumping `updated_at` (today it does, and
`obligations_surface.go:199` computes idle time from it, so a retrying
refusal makes an operation look active). Coalesce an unchanged reason into
no append, to bound tape volume from retry loops.

Two obligations the code change alone does not meet:

1. Historic stamps. A computer that already holds a stamp (t71, probably any
   computer where a freeze or verify was refused) stays inequivalent after
   the fix, because no past event carries the value. It needs a one-time
   heal: for each row whose live values differ, append a state-preserving
   snapshot that carries the live `last_intent_error` and `updated_at`.
   Make it an explicit operator act, not a boot step (no boot-time scan or
   resumer without a planned-restart marker).
2. Rollback. An old binary replaying a tape with the new field ignores it
   (unknown JSON field) and goes back to inequivalent, not to corrupt state.
   A rollback is safe but leaves the computer replay-ineligible, as today.

Mutation class of the fix: red (tape and evidence; store projection;
self-development). Tier 2 review per CLAUDE.md (two seats): a defect here
would be silent. The test commit and this record are yellow and green.

### Residuals and new finds

- Row-level evidence on t71 is missing (above). Whether the paper and the
  owner's computer are ineligible on the same cause is unmeasured; do not
  run the full report on the paper.
- Found by the survey, read from code, not reproduced: `Store.Reopen`
  (`store/store.go:1113-1115`) swaps in the new store's `ogStore` without
  the mutation interceptor that `BindProjectionTape` installed
  (`projection_tape.go:39`), and `rematerialize.go:262-265` rebinds only the
  appender projection. After a rematerialize in a live process, `og_objects`
  and `og_edges` writes may fall through to direct SQL with no tape event
  until restart. This is a second candidate for the same defect class on
  `og_*` tables, with a different mechanism. Needs its own check against
  the order of `Reopen` and the bind at boot.
- `texture.go:887` `DeleteTextureAliasesByOwner` would bypass the tape if
  anyone called it; it is dead code today.
- The vocabulary migration in `vocab_migrate_og.go` writes `og_*` directly
  at boot; replay equivalence depends on replay rerunning it
  (`rematerialize.go:204`, `projectionbase/rebuilder.go:390`). Unverified for
  the probe's replay path.

### Heresy delta (investigation)

- Discovered: a production writer of a witnessed table outside the tape,
  confirmed and reproduced (`RecordIntentError`); one `og_*` path that may
  lose its interceptor after `Reopen` (unreproduced).
- Introduced: none.
- Repaired: none.
