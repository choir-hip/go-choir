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
`projectSelfDevelopmentOperation` (`internal/store/project.go:371-460`),
fed by `computerevent.SelfDevelopmentOperationProjection`
(`internal/computerevent/projection_batch.go:106-137`) which has 27 body
fields and no `last_intent_error`. The projector's `INSERT ... ON DUPLICATE
KEY UPDATE` (`project.go:432-455`) writes the 27 columns and no
`last_intent_error`; the column keeps its DDL default `''` on replay and
survives any projected update on the live side.

| Write site | What it writes | On the tape in production? | Reproduced by replay? |
|---|---|---|---|
| `selfdev/operations.go:263` (`Start` insert) | all request columns | Yes. Guarded: `if s.projection != nil` at `:257` returns through `appendOperationProjection` (`:557`). The insert is the unbound test seam only | Yes |
| `operations.go:385` (`RecordAppliedBaseline` insert) | genesis baseline row | Yes. Same guard at `:379` | Yes |
| `operations.go:432` (`StartRollback` insert) | rollback row | Yes. Same guard at `:426` | Yes |
| `operations.go:495` (`Transition` UPDATE) | 22 columns incl. `last_intent_error`, `updated_at` | Yes. The projection branch at `:450` returns first; the UPDATE is the unbound seam only | Every column but `last_intent_error`. In projection mode `Transition` carries the old stamp in memory but `appendOperationProjection` drops it; harmless, because the projector never touches the column, so live keeps it |
| `operations.go:196` (`BindStartIntent`) | `self_development_start_intents` | Yes, same pattern (`:190`) | Yes, all four columns (`project.go:363`) |
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
  (`rematerialize.go:196, WitnessContentMatches`), so older checkpoints
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

## Fix (2026-10-23, lane fp0-selfdev-ops-replay; red)

Decision from FP0 (conservative default): the stamp keeps setting
`updated_at`, as today, so obligations_surface's idle time keeps its
meaning. Both `last_intent_error` and `updated_at` travel on the tape.

What changed:

- `computerevent.SelfDevelopmentOperationProjection` gains
  `LastIntentError` (`json:"last_intent_error,omitempty"`). While empty the
  key is absent, so an event written before the field and an event with no
  stamp are the same bytes and both project an empty column.
- `store/project.go`: the projector's existing-row `SELECT`, its
  `INSERT ... ON DUPLICATE KEY UPDATE` and the same-snapshot comparison
  (`reflect.DeepEqual` over the struct) carry the column.
  `store/residue_import.go` reads the column too, so a residue snapshot does
  not drop a stamp.
- `selfdev/operations.go`: `appendOperationProjection` sends
  `operation.LastIntentError` (so a `Transition` snapshot never wipes the
  stamp). `RecordIntentError` takes `writeMu` and, when a projection sink is
  bound, reads the row, returns without an append if the trimmed and
  truncated reason equals the stored one, and otherwise appends a
  state-preserving update (`ExpectedState` = the row's current state) with
  the new reason and `updated_at = now`. A missing row is still
  `ErrOperationNotFound`. The unbound direct UPDATE stays as the test seam
  and is unchanged.

Tests (written first, failing before the change):

- `agentcore/replay_selfdev_operations_test.go`:
  `TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence`
  with three subtests (a stamp, a clear after success, a stamp replaced by
  another reason): a control replay is equivalent, then the probe after the
  writes is equivalent with identical live and replay rows.
  `TestSelfDevelopmentIntentErrorUnchangedReasonAppendsNothing`: a clear of
  an empty stamp and a repeat of the same reason append nothing and leave
  `updated_at` alone; a new reason appends exactly one event and moves
  `updated_at`; a missing operation is `ErrOperationNotFound`.
- `store/project_selfdev_intent_error_test.go`: an event body without the
  key projects an empty column (and an empty stamp is not spelled in the
  body); a state-preserving event sets the stamp; a later one clears it.

Rollback: revert the commit. A tape that already holds events with the new
key replays on the old binary with the key ignored, the column empty, and
the computer replay-ineligible again, as before this fix; nothing is
corrupted. Mutation class red; Tier 2 review (Devin and DeepSeek).

Heresy delta (fix): repaired: the `RecordIntentError` writer outside the
tape. Introduced: none known; each distinct stamp is now a signed tape event
(a freeze or verify loop that changes its reason every attempt grows the
tape by one event per attempt; an unchanged reason does not).

### Residual: operator heal for historic stamps (not built)

A computer that stamped before this fix (t71; probably any computer where a
freeze or verify was refused) is still inequivalent after the deploy: no
past event carries its `last_intent_error` or its stamp-written
`updated_at`. The heal is one explicit operator act per computer, after the
fixed binary is serving (an old binary's projector drops the field):

1. Read the live `self_development_operations` rows for the computer and
   run the replay probe; select the rows whose `last_intent_error` or
   `updated_at` differ from the replayed row. Only rows that differ.
2. For each, build the snapshot the projector reads
   (`SelfDevelopmentOperationProjection`) from the live row, with the live
   `last_intent_error` and `updated_at` (formatted as `RecordIntentError`
   does, RFC3339Nano) and `ExpectedState` = the row's state.
3. Append them as one projection batch through the bound appender under the
   selfdev store's `writeMu`, from an owner/operator-authorized route. Not a
   boot step, not a resumer: no planned-restart marker applies.
4. Run the replay probe again: equivalent, then the computer is eligible.
   Re-running the heal selects no rows, so it is idempotent.

### Store.Reopen and the OG interceptor (code reading and check; fixed below)

Reading: `BindProjectionTape` installs the OG mutation interceptor on the
store's current `ogStore` (`store/projection_tape.go:38-40`). `Store.Reopen`
replaced `s.ogStore` and `s.ogReadStore` with those of a freshly opened
store, which has no interceptor, and left `s.projectionTape` set. After a
`Reopen`, `ProjectionTapeBound()` still answered true (so the desktop, alias,
mutation and run-memory writers kept routing through the tape), but
`og_objects` and `og_edges` writes took the `s.intercept == nil` branch in
`objectgraph/dolt_store.go:655, 1055, 1150, 1865` and became direct SQL with
no event. `agentcore/rematerialize.go:262-265` calls `Reopen` on a bound
store and then `RebindProjection`, which swaps only the appender's
projection store.

Check: `store/reopen_projection_tape_test.go`,
`TestReopenKeepsTheObjectGraphOnTheProjectionTape` (commit 88f8d5e06). It
failed before the fix: the second write left the head sequence unchanged.

## Reopen fix (2026-10-23, lane fp0-selfdev-ops-replay; red)

`Store.Reopen` now installs the bound tape's interceptor
(`s.projectionTape.interceptOG`, the same tape) on the adopted
`live.ogStore` before it assigns `s.og`, `s.ogStore` and `s.ogReadStore`
(`store/store.go`, in `Reopen`). `ogReadStore` takes no interceptor; it never
wrote through one.

Why no window and no fail-closed path:

- Ordering at boot is safe. The only boot-time `Reopen` is the retained-store
  rebase (`autoputer/projection_base.go:182-196`, reached from
  `materializeProjectionBaseIfNeeded` at `autoputer/run.go:366`), and the
  bind is at `run.go:428`, after it. There the tape is unbound, the new
  branch does nothing, and `BindProjectionTape` installs the interceptor on
  the adopted store as it always did.
- The runtime reopen (`rematerialize.go:262`) runs on a bound store. The
  interceptor is set on the new store before `Reopen` publishes it, the
  store is closed from `Close` until `Reopen` returns (its SQL handles are
  closed, so any write in that interval fails and cannot reach SQL), and the
  interceptor reads only the tape, not the swapped fields. There is no
  interval in which the adopted store is reachable and uninterceptable, so
  clearing the bound state would only break the desktop, alias, mutation and
  run-memory writers that did survive. `ProjectionTapeBound()` now matches
  what the object graph honours.
- Tests: `TestReopenedStoreRoutesConcurrentObjectGraphWritesThroughTheTape`
  starts writers on both doors (the service `s.og` through
  `PutComputerSetting`, and `ogStore.PutObject`) the moment `Reopen`
  returns, under `-race`, and requires one tape event per write (without the
  fix: 0 of 12). `agentcore.TestRematerializeFromTapeKeepsObjectGraphAndOperationWritesOnTheTape`
  runs `RematerializeFromTape` end to end on a tape-bound runtime
  (`rematerialize.go:262`), then writes through the object graph (one event;
  without the fix 0), starts an operation and stamps it on the rebound
  operation store, and requires a replay-equivalent computer.

Rollback: revert the `Reopen` hunk; the object graph then loses its
interceptor on `Reopen` as before. Mutation class red; tier 2 review.

Heresy delta (Reopen fix): repaired: an OG writer outside the tape after a
live rematerialize, reachable in a running process (reproduced through
`RematerializeFromTape`, not only at the store). Introduced: none.

### Second Reopen defect: the vocabulary write fence (found in round 1, fixed in round 2)

`Open` builds the object-graph store with
`SetWriteValidator(s.vocabWriteGuard)`, a method value bound to the store
being opened (`store/store.go:809`, `store/texture.go:184`). `Reopen` opens a
temporary store, adopts its handles, then zeroes it (`*live = Store{}`). The
validator the adopted `ogStore` kept was bound to that zeroed store, so
`vocabCutover.Load()` was false and the fence was disarmed after any
`Reopen`, whatever `s.vocabCutover` said. Round 1 recorded it with a skipped
repro (`TestReopenKeepsTheVocabularyWriteFenceArmed`) and did not fix it.
Tier 2 review (bunch 155) blocked on it.

## Round 2: the vocabulary fence fix (2026-10-23; red)

### 1. Is the boot path reachable? Yes.

`materializeProjectionBaseIfNeeded` (`autoputer/projection_base.go`, called
at `autoputer/run.go:366`) is handed the retained store `db`, which `Open`
armed from its migration report. When the retained store is behind the
advertised watermark (`PlanRecovery` returns `RecoveryRebase`, local sequence
< W), the code does `live.Close()`, `RebaseRetainedStore`, and then
`live.Reopen(storePath)` (`projection_base.go:172-196`) on that same
post-cutover store. After that `Reopen` the validator reads a zeroed store, so
the fence is off for the rest of the process: the later
`MigrateAndFenceServingVocabulary` sets `s.vocabCutover` true, but the
validator never reads `s`. The binding at `run.go:428` is irrelevant to the
fence. At runtime the same defect is reached through
`rematerialize.go:262`. DeepSeek's claim holds. Whether a given computer takes
the rebase branch at boot depends on its local sequence against the
watermark; it is a crash-boot path for a computer that fell behind, not every
boot. Reproduced at the store (`TestReopenKeepsTheVocabularyWriteFenceArmed`)
and through `RematerializeFromTape`
(`TestRematerializeFromTapeKeepsObjectGraphAndOperationWritesOnTheTape`); the
boot rebase itself has no unit test (it needs a platform fake), and its
`Reopen` is the same call the store test covers.

### 2. What would the fence refuse that passes today? (static reading)

Re-arming changes behaviour only after a `Reopen`: every process that never
reopened already runs the armed guard on every post-cutover computer, so a
writer the guard refuses is refused in the common case today, not newly.
Static survey (a Sonnet subagent plus my reading of `vocabWriteGuard`
(`vocab_migrate_og.go:921`), `ogRoleFieldsFromJSON` (`:842`), `v2LiveSet`
(`vocabmigrate/fence.go:38`); no ledger, no SQL, no paper or owner computer):

- The deterministic desk-role carriers write only values in the V2 live set:
  `choir.agent` profile and role (`graph_store.go:422-423`, values from
  `agentprofile.Canonical` through `resolveRunIdentity`,
  `runtime.go:699-736`); `choir.run` agent_profile, agent_role and
  requested_by_profile; `choir.work_item` authority_profile;
  `choir.channel_message` and `choir.event` role (`channel_store.go:42-50`);
  `choir.worker_update` role; the grant attestation role
  (`engineering_assignment_runtime.go:644`). `choir.run_memory_entry.role` is
  skipped (`ogRoleSkipKinds`). `TestVocabWriteGuardAcceptsEveryRegisteredRoleAfterCutover`
  pins that every role `agentprofile` registers is admitted.
- Edges are not a carrier: `DoltStore.PutEdge` does not call the validator,
  and every edge written has empty metadata.
- Candidates that can in principle be refused, none shown to occur: (a)
  `choir.run_acceptance` authority_profile and evidence profile/role fall back
  to the literal `"loop"` or a task type when a run has no AgentProfile
  (`trace_helpers.go:218-229`; `resolveRunIdentity` sets both on every run
  that starts normally); (b) model-authored free-form JSON whose key is
  exactly one of the eight role keys: cell-saved evidence metadata
  (`yaegikernel/choir.go:654`, `tools_evidence.go:115`), coagent packet action
  inputs (`types/evidence.go:138`; `rejectReportAuthorityInputs` blocks ID
  keys only), Texture revision metadata and body nodes, commitment answers.
  No writer was found that sets such a key.

Verdict: no writer was found that a post-cutover computer writes today and
the fence would newly refuse. The candidates above are the same risk the
armed guard already carries on every process that has not reopened. Static
reading cannot rule out a rare model-authored key; FP0 should weigh that
against a silent fail-open on a protected surface. Rollback is a revert of
the two lines in `Reopen`.

### 3. The fix and the Open/Reopen field audit

`Store.Reopen` now calls `live.ogStore.SetWriteValidator(s.vocabWriteGuard)`
before `*live = Store{}`, and sets `s.vocabCutover` when `live.vocabCutover`
is set. This differs on purpose from `s.vocabCutover.Store(live...)`: it never
lowers an armed fence, so a store cut over in this process stays armed even if
the adopted workspace has no report (fail closed); the adopted workspace's
report still arms it when `s` was not. Audit of what `Open` binds on the
store it returns, against what `Reopen` carries over:

- Copied by `Reopen`: `db`, `readDB`, `path`, `textureDB`, `texturePath`,
  `doltConnector`, `og`, `ogStore`, `ogReadStore`.
- Bound to `live` by `Open` and now rebound to `s`: the write validator.
  `vocabCutover` is merged as above.
- Bound to the tape by `BindProjectionTape` and carried since round 1: the
  mutation interceptor.
- Shared between the two object-graph stores (`ShareEngineMutex`,
  `InheritMetadataColumns`, `ShareDerivedKeys`): they refer to the stores, not
  to `live`, and are rebuilt together in `Open`.
- Reset by `Reopen` already: the deposit-upcast ledger and handle, the vocab
  replay tracker, `dirtyGen`, `closedOK`, the checkpoint cadence.
- Not touched by `Open`, kept on `s`: `kernelMode`, `retainReplayDeposits`,
  `derivedColumns`, the telemetry sealer, `projectionTape`.
- `objectgraph.Service` carries only `Durable`; nothing else is configured on
  it after `Open`.

Tests: `TestReopenKeepsTheVocabularyWriteFenceArmed` (unskipped),
`TestReopenAfterTheFencedCutoverKeepsTheFenceArmedAndAdmitsLiveRoles` (a real
fenced cutover, then a refused unknown role and admitted live roles), and the
rematerialize end-to-end test now checks that `UpsertAgentOG` refuses an
unknown role and admits `engineering` after the flip. Each fails without the
change.

### 4. Intent-error reason and UTF-8 (low)

`RecordIntentError` now runs `strings.ToValidUTF8` after the 512-byte cut and
before the unchanged-reason comparison, so a cut inside a multi-byte rune, or
invalid bytes in a model-written reason, leave a stored value equal to what a
repeat produces (otherwise the repeat would append every time).
`TestSelfDevelopmentIntentErrorReasonIsValidUTF8AfterTheCutAndRepeatsAppendNothing`.

### 5. A same-reason retry no longer moves updated_at (low)

Before round 1 every `RecordIntentError` set `updated_at`, even for the same
reason. Now an unchanged reason appends nothing and leaves `updated_at`
alone. `obligations_surface.go:199-202` computes `idle_seconds` from
`updated_at` and marks an open operation `stalled` after 30 minutes
(`selfdevStalledAfter`). Effect: a refusal loop that repeats the same text no
longer keeps resetting the clock, so the operation becomes `stalled` 30
minutes after the last change (a new reason, a clear, or a state transition)
instead of staying fresh while it retries. That is closer to the meaning of
the field, and it is a behaviour change from before round 1 that FP0
approved only in the sense of "a stamp that changes still sets `updated_at`";
a looping same-reason refusal will now show up as stalled.

## Operator heal and a corrupt vocabulary sidecar (2026-10-23, lane fp0-selfdev-heal; record first, green)

Two items from FP0 that follow this record. This section is written before
the tests (yellow) and the code (red); the commits are ordered the same way.

### A. The operator heal for historic stamps (design as built)

The "Residual: operator heal for historic stamps" steps above are the design;
what is added here is the shape of the route and the three places where the
steps needed a decision.

- Route: `/internal/self-development/operation-stamp-heal`, registered beside
  `/internal/store/derived-columns` (`autoputer/run.go`), host peer only
  (`server.HostPeerCaller`: the guest's own loopback children get 403).
  `GET` is the dry run: it replays, selects, and appends nothing. `POST`
  applies. Any other method is 405. Never a boot step, never a resumer: no
  code path calls it but the HTTP route, so no planned-restart marker is
  involved.
- Dry-run output shape (JSON): `computer_id`; `mode` (`dry_run` or
  `applied`); `probe` (the replay probe at the time of the plan:
  `live_head_sequence`, `replay_head_sequence`, `equivalent`, `eligible`,
  `reason`, and the differing witness keys); `rows_examined`; `rows` (each:
  `operation_id`, `state`, `live_last_intent_error`,
  `replay_last_intent_error`, `live_updated_at`, `replay_updated_at`,
  `differs`, a subset of `["last_intent_error","updated_at"]`);
  `missing_in_replay` (live rows with no replayed row: reported, never healed,
  because a state-preserving snapshot of a row the replay does not have is
  refused); and, for `POST`, `appended` (rows in the one batch) and `after`
  (a second probe, unless `?verify=0`).
- Selection is by the two columns only (`last_intent_error`, `updated_at`),
  compared as the strings `scanOperation` produces on each side. A row that
  differs in some other column is not selected; the probe's remaining
  differences in `after` name it.
- Atomicity decision: the plan is separate from the apply. The plan reads
  the live rows and keeps, per selected row, the full live `Operation` and
  its state. The apply takes the selfdev store's `writeMu`, refuses with
  `ErrConflict` when a selected row's stamp itself moved since the plan (a
  newer stamp must not be overwritten by an older snapshot), and appends one
  batch of state-preserving snapshots with `ExpectedState` = the state the
  plan saw. A row whose state moved since the plan is therefore refused by the
  projector's `ExpectedState` check (`store/project.go`,
  `ErrProjectionMismatch`), the batch is not committed, and the route answers
  409 with nothing appended. `Transition` carries the stamp unchanged, so a
  state move reaches `ExpectedState` and a stamp move reaches the
  `ErrConflict` check; between them the two ways a stale plan could damage a
  row are refused.
- Concurrency: one heal at a time per runtime (a second concurrent call
  answers 409); the selfdev `writeMu` serialises it against `Transition`,
  `Start` and `RecordIntentError`.
- Idempotence: after a successful apply the replayed rows carry the live
  values, so the next plan selects nothing and a `POST` appends nothing (no
  batch is built for an empty plan). A stamp that a fixed binary wrote after
  the probe's head can look divergent for one plan, because the probe is
  taken first; healing it appends a snapshot equal to the live row, which is
  harmless, and the following run selects nothing.
- Cost: the plan runs the replay probe. On t71 that was about 20 s; on a
  computer whose chain replays slowly (the paper) it is the full-chain replay,
  which did not answer in 10 minutes. Do not run it on the paper without a
  published base. `POST` runs a second probe for `after`; `?verify=0` skips it.
- Mutation class: red (tape and evidence, self-development). The route
  appends signed tape events. Rollback: stop calling it; the appended
  snapshots are ordinary state-preserving events and an old binary replays
  them with the `last_intent_error` key ignored (the computer returns to the
  replay-ineligible state it was in). Review tier 2.

### B. A corrupt vocabulary sidecar must not leave the fence off

`Open` (`store/store.go`, after `SetWriteValidator`) and
`OpenTextureWorkspace` (`store/texture.go`) arm the vocabulary write fence
with `if rep, err := s.loadVocabMigrationReport(); err == nil && rep != nil`.
A sidecar that exists but does not decode (truncated by a disk fault, damaged
by a hand edit) gives `err != nil`, the condition is false, and the store
opens with `vocabCutover` false: `vocabWriteGuard` then passes every write.
That is a silent fail-open on a protected surface (bunch 157, Devin finding 1).
A missing file is the only case that means "never cut over" (`nil, nil`).

Decision: open with the fence armed and log the error. The open is not
refused. Reasons:

- Boot path. `Open` runs at `autoputer/run.go` (the store open before the tape
  bind at `:428`), and `OpenTextureWorkspace`, `Reopen` (`projection_base.go`
  at boot, `rematerialize.go` at runtime) and the projection-base rebuilder
  open stores the same way. Refusing the open would turn a damaged sidecar into
  a store that cannot be opened at all, so the computer cannot reach the
  health listener, cannot be refreshed, and cannot be inspected by an operator
  (no-SSH operability): a crash loop with no surface. Arming and logging keeps
  the process up and the failure visible.
- Fail-closed is the cheap direction. The sidecar is written only by the
  cutover (`persistVocabMigrationReport`: temp file, fsync, rename), so a
  sidecar that exists, damaged or not, means a cutover began on this
  workspace. A store that cut over has V2 rows; the armed guard refuses
  non-V2 role values, which is the correct posture for it. The cost of arming
  wrongly is a refused write of a stale role (the same refusal every cut-over
  computer already has); the cost of not arming is V1 spellings entering a V2
  graph silently.
- The corruption does not go unnoticed later. `MigrateAndFenceServingVocabulary`
  (`run.go:695` and `:767`, `rematerialize.go:204`, `projectionbase/rebuilder.go:390`)
  still returns the load error, as before, and the boot paths treat it as a
  refusal (`boot.fatalf("vocabulary migration refused")`). That is an
  existing, visible stop at the migration step, after the store is open and
  the fence is armed, not a change made here. A repair of the sidecar is an
  operator act and is not built here.
- `Reopen` already carries an armed fence over (`live.vocabCutover` merges into
  `s`), so arming in `Open` makes the corrupt-sidecar case hold after a
  `Reopen` as well.

Test (written first): a store whose sidecar is corrupt opens, and its fence
is armed (an unknown role is refused); a missing sidecar leaves it disarmed
(control); `OpenTextureWorkspace` behaves the same; the fence survives
`Reopen`. Rollback: revert the helper; the open returns to ignoring the load
error.

### Heresy delta (this section)

- Discovered: none beyond the two items named by FP0 and the review.
- Introduced: a host-peer route that appends tape events on an explicit
  operator call (the heal), by design; each run that selects rows grows the
  tape by one event.
- Repaired: the silent fail-open of the vocabulary write fence on a corrupt
  sidecar (`Open` and `OpenTextureWorkspace`); the means to make a computer
  that stamped before the `RecordIntentError` fix replay-equivalent again
  (not yet run on t71: that is an operator act after the deploy).

### Built (code landed after the tests)

- `store/vocab_migrate.go` `armVocabFenceFromReport`, used by `Open` and
  `OpenTextureWorkspace`.
- `selfdev/operations_heal.go`: `ListAll`, `PlanStampHeal`, `ApplyStampHeal`
  (`ErrStampHealUnbound`); `operationProjectionOp` is the shared builder
  `appendOperationProjection` now calls.
- `agentcore/selfdev_stamp_heal.go`: `Runtime.HealSelfDevelopmentIntentStamps`
  (plan = replay probe + row comparison; apply = one batch; verify = a second
  probe; `ErrStampHealRunning` for a concurrent call).
- `autoputer/operation_stamp_heal.go`, registered in `autoputer/run.go` beside
  the derived-columns route. 409 for a refused or concurrent heal, 503 when
  the store is unbound or the probe unavailable.
- Tests: `agentcore/replay_selfdev_heal_test.go` (equivalence before and after,
  dry run appends nothing, one batch, second run selects nothing, state moved
  refused by `ExpectedState`/`ErrProjectionMismatch`, stamp moved refused,
  unbound store, concurrent heal), `autoputer/operation_stamp_heal_test.go`
  (host peer only, GET plans and POST applies, 409/500/405),
  `store/vocab_sidecar_fence_test.go`.

Operator use after the deploy (not run here; no live-computer mutation in this
lane): `GET` the route on t71 (computer-71afbf14) from the host peer and read
`rows` and `probe`; `POST` it; the answer's `after.equivalent` should be true
and a following `GET` should list no rows.

### Round 2 (bunch 163: Devin accept with lows; DeepSeek one blocking; red)

**1. Blocking (DeepSeek 1): the heal could append an event the replay cannot
apply.** Round 1's record said "a row that differs in some other column is
not selected". That was false: `PlanStampHeal` compared only the two stamp
columns, so a row whose state (or any other column) had diverged from the tape
was selected with its stamp. The live append is validated against the live
table, where `ExpectedState` = the live state passes; replay validates the
same event against the replayed row (`store/project.go`, `ErrProjectionMismatch`
in `Finalize`), where the state differs, and a canonical event that fails to
apply is a boot fatal loop with no rollback. Reproduced first: the new test
changed a row's state with raw SQL and the heal's verification probe failed
with `replay finalize sequence 7: ... state is failed, expected executing`.
Fix: `PlanStampHeal` compares every `Operation` field by name
(`operationFields`) and selects a row only if it equals the replayed row in
every field except `last_intent_error` and `updated_at`. Any other row that
differs is reported under `refused_diverged` (`operation_id`, `reason`
`state_diverged` or `row_diverged`, `live_state`, `replay_state`,
`different_fields`, `replay_values`) and nothing is appended for it. A refused
row is not an error: the healthy rows in the same plan are still healed, a
second run still selects none, and the refused row is the operator's to
repair by another act. Test:
`TestSelfDevelopmentIntentStampHealRefusesARowWhoseStateDivergedFromTheReplay`.

**2. Low (DeepSeek 2): the stamp-moved recheck.** The recheck under `writeMu`
now compares `last_intent_error` and `updated_at` (an A, B, A reason has the
planned text and a later `updated_at`). Test:
`...ReturnedToItsPlannedTextWithALaterUpdatedAt`; it fails with the old
text-only comparison. The recheck applies only while the row is in the planned
state: a state move also moves `updated_at`, and is left to the projector's
`ExpectedState` check, so that check stays the authoritative refusal for state. An `expected_updated_at` on the projection op, checked
beside `ExpectedState`, is not carried, and not because it is costly: it
cannot be right. The check would run again at replay, against the replayed
row, whose `updated_at` is by construction not the live one (that difference
is what the heal repairs), so the heal event itself would fail at replay. The
recheck under `writeMu` is enough because every writer of the table
(`Start`, `StartRollback`, `RecordAppliedBaseline`, `Transition`,
`RecordIntentError`, `BindStartIntent`) takes the same `writeMu` as
`ApplyStampHeal`, which holds it from the recheck through the append, and
`residue_import` and the projector run only at boot or replay. A second
`selfdev.Store` instance (after a rematerialize rebind) has its own `writeMu`;
the heal reads `rt.selfdevOperations` at the time of the call, so it holds the
rebound store's mutex, which is the one new writers use. A residual race
between an old store instance still in flight and the new one is the existing
rematerialize overlap, not new here.

**3. Doc corrections (DeepSeek 3, 5; Devin 1).**

- Section B said the open "keeps the process up". Corrected: *the open
  survives; boot still stops at the migration refusal* (`boot.fatalf` at
  `run.go:698` and `:770`, `rematerialize.go:204`, `rebuilder.go:390`). That
  crash loop on a corrupt sidecar is pre-existing and unchanged by this lane.
  Arming at open narrows the fail-open window (including the replay deposit
  upcast, `vocab_upcast_deposit.go:755`); it does not make the boot survive.
  A repair or removal route for the sidecar is separate scope.
- The rollback line said an older binary "replays" the heal events. Precise
  statement: an older binary *replays them safely and the computer is still
  not equivalent*: the ops are the same kind, the `last_intent_error` key is
  ignored, `ExpectedState` exists, and `updated_at` is applied; the computer
  stays replay-ineligible, exactly as before the fix.
- The replay probe's capture holds the appender mutex for the whole live
  extract (`HoldAppends` around the per-table hashing, `replay_completeness.go`),
  so tape appends wait for that window; `/health` after boot is the static
  default handler and takes no such lock. The replay phase itself runs on a
  disposable store and dry-run appender and holds neither the store write
  connection nor the engine mutex. Never run the heal on the paper (or any
  computer with no published base): the paper's replay did not answer in 10
  minutes and its extract window is base-independent. This is now in the
  handler's comment as well.

**4. DeepSeek 6: 503 until serving.** The route is registered before
`s.Start()`, so it was reachable during the boot replay and relied on the
appender's head check to refuse. It now answers 503 until `rt.Start` has
returned (`runtimeServing` in `autoputer/run.go`, set in both boot branches),
after the host-peer check, so a non-host caller still gets 403. Tests cover GET
and POST before serving.

**5. Devin 3: the 503 branches.** The endpoint test now covers
`ErrStampHealUnbound`, `ErrReplayCompletenessUnavailable` and a wrapped one,
each 503.

**Not changed (Devin 2).** The apply runs on the request context: a caller
that disconnects after the batch commits but before the verify probe returns
an error with the event already on the tape. A retry converges (the next plan
selects nothing). Benign; left as is.

Heresy delta (round 2): discovered: the round-1 heal could append a
replay-unappliable event on a computer whose rows diverged beyond the stamp;
introduced: none; repaired: that, the doc claims above, the pre-serving
window.
