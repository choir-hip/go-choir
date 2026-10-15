# Self-Development Go Drill selfdev-jobs-0654: raced-activation fix verified, engineering stalled in a capsule call and was cancelled (2026-10-15)

Date: 2026-10-15 (drill 06:54-07:40 UTC). Overnight lander, step 0.
`node scripts/reeng_tracer.mjs --label selfdev-jobs-0654 --scenario engineering --change go --expect-sha f704110ce7f9… --timeout-min 45`
on staging build `f704110ce` (docs-only commit over `da80c5fcc`, which carries
raced-activation fix 2, `44944dc72`). Status: documented, evidence only; no
code change.

Evidence: `~/.config/choir-qa/traces/selfdev-jobs-0654/receipt.json`;
tracer guest `vm-6bd99e1298dffecabfa5c4957925962f` console on Node B;
gateway journal for the same VM.

## Outcome

`acceptance.pass=false`, `reached=engineering_final_report`; failed:
frozen, final_report_names_bundle, operation_on_request_trajectory,
verified_against_named_bundle, owner_approved, applied, one_planned_restart,
route_receipt, restore_to_pre_apply_head, go_build_job_done,
go_test_job_done. The drill never reached freeze, so apply and restore were
not exercised.

## What the fix did (the thing under test)

- Console: `texture api: recovered raced activation for doc d1886c52… run
  9752fb21…` at 06:55:11 (the second-path fix firing).
- No `dispatcher: poison`, no `mutation authority mismatch` in the guest
  console. The story reached management (06:55:19) and engineering
  (06:55:39) and Texture later authored turns. The jobs-0458 poison did not
  recur.

## What failed

Assignment `assignment-e5b261a1…` (engineering) ran 22 minutes and ended
`stop_reason: cancelled`, `bundle_digest: null` at 07:18:21 (precommit rules
settled `no`/`unresolved at terminal cancelled`).

- 07:04:30: `slow tool call tool=capsule_go_eval … dur=30.087s err=failed to
  read broker response: read unix @->…/broker.sock: i/o timeout` (a cell
  that ran into its 30 s timeout).
- 07:04:42 (last inference before the gap) to 07:18:07 (next): 13.4 minutes
  with no inference request and no console line; the in-flight call
  (iteration 145) did not return until the assignment's budget ended.
- 07:18:21 cancel; the next run (Texture) continued and authored revisions
  (07:20-07:20:47: `not published: … kind "task"`, expected for a drill).

## Classification (hypothesis until a ledger read confirms it)

This matches item 2 of
[selfdev-go-drill-jobs-0101](selfdev-go-drill-jobs-0101-2026-10-15.md): the
capsule broker RPC channel desynchronizes after a cell hits its timeout
(`broker.sock: i/o timeout`) and later calls misbehave. The repair
(`5c5db2027`, the broker client drops and re-dials its connection after a
transport or framing error) is on branch `fix/selfdev-go-drill-0101`, which
is not on main, so this build does not carry it. The 13-minute hang after the
timeout is consistent with a stale or shifted response, but the run ledger
(turn-commit reasons, controls) was not read here: the cause of the stall is
not confirmed.

Judgment for the lander: not a new failure class and not a regression of
`da80c5fcc` (the raced-activation behaviour improved); the self-development
Go drill remains blocked on the unlanded broker-client fix. Landing of the
remaining queue continued under that judgment.

## Heresy delta

discovered: none new; introduced: none; repaired: none.

## Next

Land `fix/selfdev-go-drill-0101` (red: capsule broker client) with its
deployed proof, then rerun this drill.
