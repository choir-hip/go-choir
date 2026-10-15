# P5 go-served drill p5-go-served-0825: engineering never freezes, apply not exercised (2026-10-15)

Drill: `node scripts/reeng_tracer.mjs --label p5-go-served-0825 --scenario engineering --change go-served --expect-sha 2eae77de6… --timeout-min 60 --apply-min 30 --restore-min 12`
on staging `2eae77de6` (P5 host build enabled in corpusd: `corpusd host build:
enabled (source /opt/go-choir, installable .#autoputer)`). Test computer
`computer-5c80f0f9e16061255bdc83a3ab52c0fa`, VM `vm-ae39488465e478796b0f68a903099efa`.
Status: documented, evidence only.

Outcome: `pass=false`, `reached=engineering_final_report`; failed: frozen,
final_report_names_bundle, operation_on_request_trajectory,
verified_against_named_bundle, owner_approved, applied, one_planned_restart,
route_receipt, restore_to_pre_apply_head, go_build_job_done,
go_test_job_done, probe_served_after_apply, layered_release_serving,
restore_restarted_into_pre_apply, probe_gone_after_restore.

Evidence: three engineering assignments (`7731751…` 08:25:37-08:26:37,
`696f5d3…` ..08:29:25, `fb0e72a…` ..08:33:06) ended `stop_reason: failed`
with no bundle; Texture re-cast to management through round 5; no
`dispatcher: poison`, no `mutation authority mismatch`, no broker
`i/o timeout` in the guest console. The reasons for the failed assignments
were not read from the run ledger (hypothesis: the jobs-0101 A1-A3 class,
desk judgment or capsule environment).

Consequence: P5's apply path (host build, layered release, restore) has not
been exercised on staging. P5 is deployed and dormant until an operation
reaches `accepted`. Owed: a drill that freezes (needs the engineering desk
failures understood first, see
[selfdev-go-drill-jobs-0101](selfdev-go-drill-jobs-0101-2026-10-15.md) and
[selfdev-jobs-0654](selfdev-jobs-0654-raced-activation-verified-engineering-budget-2026-10-15.md)).
Heresy delta: discovered none new, introduced none, repaired none.
