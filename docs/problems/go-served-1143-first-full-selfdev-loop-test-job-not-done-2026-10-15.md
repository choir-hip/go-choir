# Go-served drill go-served-1143: first full self-development loop on staging; only go_test_job_done failed (2026-10-15)

Date: 2026-10-15 11:44-12:13 UTC. Drill:
`node scripts/reeng_tracer.mjs --label go-served-1143 --scenario engineering --change go-served --expect-sha 7b8944f03… --timeout-min 60 --apply-min 30 --restore-min 12`
on staging build `7b8944f03` (P4, P5 host build, S1 observe, broker re-dial and
write-replace fixes `bd75f8f53`/`8a74c15db`, Dolt guardrails, patch lanes,
hotplug, Exa). Test computer `computer-5d04503b32c4f29ff28abbfb30d5ca14`
(VM `vm-87e0e5ae76a3a9e5a7238d470aa77a06`), minted and stopped by the tracer.
Evidence: `~/.config/choir-qa/traces/go-served-1143/` (`receipt.json`,
`bundle-a1.json`). Status: documented, evidence only; nothing patched.

## Outcome

`pass=false`, `reached=restore`, `failed=[go_test_job_done]`, `gaps=1`. Every
other check passed: texture_cast_to_management,
management_cast_engineering_on_records, frozen, final_report_names_bundle,
operation_on_request_trajectory, verified_against_named_bundle,
owner_approved, applied, one_planned_restart, route_receipt,
restore_to_pre_apply_head, go_build_job_done, probe_absent_before_apply,
probe_served_after_apply, layered_release_serving,
restore_restarted_into_pre_apply, probe_gone_after_restore.

Timeline: engineering assignment `assignment-62be17a2…` cast 11:45:32; operation
`selfdev-735be93e…` frozen 12:07:18 (bundle `4ac98703…`); final report
`completed` 12:07:25 naming that bundle; owner approval; applied 12:11:59
(effective head `c7a26efa…`, desired `30abb8bc…`); route receipt generation 1;
post-apply `/api/…probe` served by the layered release (`layered: true`,
`code_commit 4e59cfa7…` over `base_commit 7b8944f0…`); restore by
`tape_reconstruct`, witness matched, 567 tail events, build probe gone from the
pre-apply release.

## Acceptance from the ledger (coordinator's four criteria)

- Freeze was called and the operation froze (12:07:18).
- No `not a directory` in the guest console after any cell timeout (count 0);
  the one slow cell (`capsule_go_eval` 30.3 s at 12:04:30) returned `err=<nil>`,
  and there were no broker `i/o timeout` lines.
- No `dispatcher: poison`, no `mutation authority mismatch`.
- The go build job passed; the go test job did not (below).
- `choir.WriteFile` on routes.go: not read from the ledger event by event here;
  the freeze and the layered build carrying the change imply it returned ok.

## The failed check: go_test_job_done

The `jobs` phase read the completing assignment's inbox at 12:08:32: `count 3,
done 2`. One of three capsule jobs was not done when the tracer read it (the
read happens after the final report, so the third job was either still running,
stopped, or never polled to completion). Which job and why was not read from the
ledger: hypothesis, the test job was started, the desk froze on the build, and
the test job was left unpolled. Either the tracer check is early (it reads
before the job settles) or the desk skipped waiting for the test. Owed: read
`bundle-a1.json` job receipts for the three jobs before deciding.

## Other observations

- Post-restore, the probe was gone from the pre-apply release and
  `effective_release` returned to non-layered, but `served_change_then_restored`
  is `false` and the build after restore reads `4e59cfa7…` at the 12:12:43
  probe, then `7b8944f0…` at 12:12:48 (the restart settling); the check
  `probe_gone_after_restore` passed.
- `census_forbidden_objects: 1` in the receipt: not investigated.
- Gap (not new): no product route serves the projection head after a restore.
- Hotplug build noise: a once-a-minute `fc_api` 400 `Device not found` in the
  tracer VM's console (18 in 20 min) on a VM without a virtio-mem device; absent
  from older consoles. Not a failure, but it is new log noise from the merged
  hotplug controller on computers it does not apply to.

Heresy delta: discovered two small (go_test job settle, hotplug probe noise);
introduced none; repaired: the broker desync / write-replace class (no
`not a directory`, Freeze reached).
