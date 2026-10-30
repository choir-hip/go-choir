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

## Trace reading (2026-10-15 18:35 UTC): the test ran and passed; the check counts only jobs

Read from `receipt.json` (`drill_claim.jobs_by_inbox`) and `bundle-a1.json`
(run events of assignment `62be17a2…`). The earlier hypothesis that the
test job was started and left unpolled is **refuted**.

- **The three jobs** were:
  - `go build … ./cmd/autoputer`, **failed**, exit 1, 11:46:56 to 12:03:27
    (16.5 min). Not investigated here.
  - `go vet ./internal/apihandler/`, done.
  - `go build … ./cmd/autoputer`, done, 12:06:54 to 12:07:04.

  No `go test` job was ever started. "count 3, done 2" is the failed
  first build.
- **The requested test ran as a cell.** Engineering ran
  `choir.Exec("sh", ["-c", "cd /workspace/platform && go test ./internal/selfdevprobe/"])`
  in a cell that panics on a non-zero exit.
  - First run: compile error, `selfdevprobe_test.go:21:12: undefined: json`.
    The cell panicked.
  - Second run, after the fix: `ok … 0.003s`, cell exit 0, receipt
    `capsule-go-eval:sha256:6a5db84d…`.
- **The prompt contradicts itself**
  (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`):
  - Line 56 says go test "is a job (choir.Start), never a choir.Exec".
    Line 59 says to run the requested go test "as a second job".
  - Line 36 says a quick command (under 30 s) "runs directly" with
    choir.Exec.
  - Line 61 lets Freeze cite "the cell or job that ran the requested
    tests", and the `tests_pass` rule settles on a check cell that panics
    on failure.
  - Engineering took the line 36 and 61 path. Freeze accepted it, and
    management's Verify passed.
- **Classification: tracer check, not product failure.**
  `checks.go_test_job_done` in `scripts/reeng_tracer.mjs` counts only
  done jobs (`doneJobs(claim.jobs).some(isGoTest)`). The product contract
  admits a receipted check cell. This was a pass the check could not see.

Decision (yellow, tracer only; no product prompt change):

- The check becomes `go_test_passed`. It passes on either:
  - a done go-test job with a receipt; or
  - a successful `capsule_go_eval` result with a `capsule-go-eval:` receipt,
    whose invoked source runs `go test` and panics on a non-zero exit (the
    form line 61 prescribes for a check cell).
- The receipt records which form passed.
- The prompt contradiction (lines 56/59 versus 36/61) is left as a named
  residual, not fixed here. Deciding whether tests must be jobs is a
  product choice, and Freeze already admits cells.

Heresy delta: discovered one (the prompt contradiction); repaired the
false negative in the drill check; introduced none.

**The failed first build** (11:46:56 to 12:03:27): the job tail ends in
`internal/selfdevprobe/selfdevprobe.go:19:6: undefined: json`.
Engineering's own compile error made it fail, which is correct behaviour.
It took 16.5 min because the capsule's Go module cache was cold: the tail is
`go: downloading …` lines. After the fix, the rebuild took 10 s. Residual,
performance only: a capsule's first Go build pays a full module download.

## Proof after the check change: go-served-1825 (2026-10-15 18:22–18:53 UTC)

`node scripts/reeng_tracer.mjs --label go-served-1825 --scenario engineering --change go-served --expect-sha cc720cd20… --timeout-min 60 --apply-min 30 --restore-min 12`
on staging cc720cd20. The tracer minted `computer-f8d6ef41…` and stopped it
(201). Evidence: `~/.config/choir-qa/traces/go-served-1825/`.

- **`pass=true`, `reached=restore`, `failed=[]`: 18 of 18 checks**,
  including `go_build_job_done` and `go_test_passed` (evidence: `job`).
- Engineering took the job path this time. Its two jobs were both done:
  the build, then `sh -c "cd /workspace/platform && go test ./internal/selfdevprobe/ && go vet …"`.
- The new run-events reader worked against the live route:
  `go_test_cells passed=1` on the completing assignment.
- Timeline:
  - management cast engineering at 18:23:15;
  - frozen about 18:39:30;
  - final report 18:39:52;
  - applied 18:53:20, after the planned restart;
  - restored 18:53:42, and the probe was gone by 18:53:47.
- Gap: the known one only. No product route serves the projection head
  after a restore.
