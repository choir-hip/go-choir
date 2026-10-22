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
