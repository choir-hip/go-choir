# FP4's proof script: step 5 can only fail and step 7 can pass without B (2026-10-22)

Found 04:35 UTC by the self-development track monitor, reading
scripts/proofs/fp4-selfdev-admission.sh (origin/main fd58229aa) against the
log lines in FP4's code (origin/lane/b10-selfdev-b1fix e71cf6259). Nothing
was run. Mutation class of this record: green. The fix is yellow (the proof
script).

## Evidence

The script greps the guest's `recent_log`. That is the runtime's ring of
every standard-logger line (`CaptureBootLog`,
internal/agentcore/observability.go), served on /api/runtime/observability.

| step | regex the script expects | log line in FP4's code | effect |
| --- | --- | --- | --- |
| 2 | `waits behind` | engineering_admission.go:184 `runtime: engineering admission: assignment %s waits behind %s (%s); retry %d in %s` | sound |
| 4 | `planned boot: queued engineering request .* kept and resumed` | engineering_admission.go:231 `runtime: planned boot: queued engineering request %s kept and resumed` | sound |
| 5 | `engineering.*(admitted\|bound)\|(admitted\|bound).*engineering` | none. No log line in internal/agentcore or internal/store prints "admitted" or "bound" when an engineering request binds (git grep on the lane). | **always FAIL**, even when B binds at once |
| 7 | `interrupted_by_restart\|interrupted by a restart` | actorruntime/handler.go:93 `actorruntime: boot follows a crash or stop; pre-boot work is interrupted_by_restart`. Every crash boot prints it, whether or not B was queued. B's own cancel (`interruptAssignmentAtCrashBoot`, engineering_assignment_fate.go:669) is silent by design (D7) and logs nothing. | **false PASS** possible |

## Why it matters

Standing question 6: success is the artifact, not a narration. Steps 5 and
7 make claims about B's assignment row, and the row is readable:

- `GET /api/trajectories/<trajectory_id>` returns `co_super_assignments[]`
  with `disposition`, `disposition_reason`, `capsule_disposition` and
  `terminal_at`.
- `GET /api/trajectories/<trajectory_id>/events` returns
  `co_super_assignment_opened` / `_bound` / `_reported` with `created_at`.

The monitor read both through the host-internal guest path on 10-22. They
are read-only.

## Repair shape (for FP0; the monitor does not edit scripts)

- Step 5: read B's `co_super_assignment_bound` event time and A's
  `terminal_at`. PASS when B binds after A's terminal_at, within the nudge
  bound (seconds, not the minute retry). Keep the log grep as INFO only.
- Step 7: read B's row after the crash boot. PASS when its disposition is
  cancelled and its reason is `interrupted_by_restart`, and B never got a
  `co_super_assignment_bound` event. The generic actorruntime line is INFO.
- Steps 2 and 4: also read the rows. In step 2, A is bound with capsule
  not revoked, and B is open with no `_bound` event. In step 4, B is still
  open after the boot. Keep the log lines as corroboration.

## Update, 04:45 UTC: the repair's event join never matches real events

lane/fp0-fp4-proof-fix 4571778e9 judges the rows and events, as above. It
finds a row's bound events by
`.work_item_id == $row.binding.assigned_work_item_id` (`fp4_bound_events`).
Real tape data does not carry those fields.

Evidence: computer-ad1b46d4's trajectory dbadbdfd…, read 04:20 UTC through
the same two routes. Every `co_super_assignment_opened`, `_bound` and
`_reported` event has `"work_item_id": null`. Both assignment rows have
`binding.assigned_work_item_id: ""`. In jq, `null == ""` is false, so the
join matches no event:

- steps 2 and 4: "B never bound" passes whether or not B bound (**false
  PASS**);
- step 5: "B has no co_super_assignment_bound event" always (**always
  FAIL**);
- step 7: the "never bound" half passes vacuously.

The same events carry the assignment id in `command_id`:
`engineering-bind:<assignment_id>` (engineering_assignment_runtime.go:645
on the FP4 lane, unchanged by FP4). Opens and reports use
`engineering-open:<id>` and `engineering-report:<id>:<report id>`. Joining
on `command_id == "engineering-bind:" + .assignment_id` matched each row
to its own bound event on the real data.

The script's self-test (fp4-selfdev-admission-test.sh) builds its fixtures
with `work_item_id` set on events and rows. That is why it passes. Fixtures
copied from a real `/events` page (null work_item_id, command_id set) would
have caught it.

Caveat: this data came from the delegate's build (219535ef…), not from
FP4's. FP4's scope (engineering_admission.go, engineering_assignment_runtime.go,
engineering_assignment_fate.go, continuation_schedule.go, api.go,
objectgraph/writer.go) does not touch the lifecycle event writer. So the
same shape is expected, but it is not yet observed on FP4's build.

## Check of the repair at 5a84c7512 against the real tape, 04:40 UTC

FP0 asked for a read-only check of lane/fp0-fp4-proof-fix at 5a84c7512
(a9bb72487 fixtures, 5a84c7512 join on
`command_id == "engineering-bind:" + assignment_id`, work_item_id fallback
only when non-empty, and steps 2, 4 and 7 fail when A has no bound event).

Method. Three real trajectories from computer-ad1b46d4 (dbadbdfd…,
be257da2…, b5475247…: 9 assignment rows, 87 events) were read with
read-only GETs through the host-internal guest path. They were then served
to the lane's own script by a fake curl, the same way its fixture test
works. No network, no Node B call from the script, no key. A stand-in
computer id was used, so the delegate guard was never exercised. Evidence
log: _runs/digests/selfdev-monitor-fp4-proof-fix-replay-20261022T044006Z.log
on the owner's Mac.

Results:

- **The join matches the real data.** `fp4_bound_events` matched each of
  the 9 rows to exactly one bound event, its own
  `engineering-bind:<assignment_id>`. `fp4_unmatched_bound` classed every
  other bound event in the trajectory as another assignment's (n-1 per
  trajectory).
- Real shapes: all 9 bound events (and every engineering event) have
  `work_item_id: null`, and all 9 rows have
  `binding.assigned_work_item_id: ""`. So the fallback never fires on this
  tape. The fixtures in a9bb72487 now use these shapes.
- Step 2 on dbadbdfd with A=f138ade6 (failed) and B=415c4263 (completed):
  "join found 1 bound event for A" PASS; A not live FAIL; B not waiting
  FAIL. That is correct for these rows.
- Step 2 on a 4-row trajectory with no ids: FAIL, "cannot name exactly one
  row". It refuses to pick, as designed.
- Step 5 with real nanosecond times (A terminal_at
  10:23:49.728789822Z, B bound 10:25:02.310193551Z): the script computes
  72.5814 s, against 72.581404 s by Python. It FAILs at the default 30 s
  and PASSes at 90 s. B bound before A's terminal_at: FAIL. Across two
  trajectories (4571.89 s): PASS at a 5000 s bound.
- Steps 4 and 7 on the same rows: the A check PASSes and B FAILs (completed,
  not open or cancelled). Correct.
- The lane self-test passes on macOS: 29 of 29.

Shapes the fixtures get wrong or do not cover. None of these is a defect
in the judging.

1. Times. The fixtures use whole seconds or three-digit fractions. Real
   times have nine fractional digits. The script parses both (checked
   above). A nine-digit fixture would keep it that way.
2. No real queued or crash-cancelled row exists on this tape. The `open` /
   `unbound` and `cancelled` / `interrupted_by_restart` values come from code,
   not tape (types enums; `CancelEngineeringAssignment` sets
   `DispositionReason = req.Reason` verbatim). They are first observed at
   the drill.
3. Step 7 may false-FAIL (hypothesis). On a crash boot whose trajectory is
   no longer live, the reconcile cancels with "restart completed terminal
   trajectory assignment fate", not `interrupted_by_restart`
   (engineering_assignment_fate.go about :499-509 on the FP4 lane). That
   failure is conservative, not a false pass.
4. One events page. The script reads `?limit=1000`, and the store caps a
   page at 1000 (internal/store/lifecycle.go ListLifecycleEventPage). It
   does not follow `next_cursor`. A trajectory over 1000 events could hide
   B's bound event, and "B never bound" would then false-PASS. The real
   trajectories had 18 to 38 events, so the risk is low. Asserting
   `watermark == next_cursor` would close it.
5. The product-API path (choir.news with a test key) was not exercised. It
   serves the same handler (HandleTrajectoryDetail), so the same JSON is
   expected, but it was not observed.
