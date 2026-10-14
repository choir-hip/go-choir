# C1e B1: Engineering Assignments End "Cancelled" Seconds After Management's Cast

Date: 2026-10-14, 05:20 UTC. Found by the B1 engineering drill
(`node scripts/reeng_tracer.mjs --scenario engineering`, authored tonight
as C1e step 4, commit 6ae126b9 on branch worktree-agent-ab80b7882f2ad21aa,
staged for the next push). Deploy: staging e9d727a7 (C1e B1 + the
homepage change). Status: documented; cause not yet established from the
trace.

## What the drill saw

Test computer `computer-d8ad5936bca79ed2668330c68419c5db` (build
e9d727a7, self-development mode set to propose_only), label
`c1e-b1-engineering-0444`, receipt in
`~/.config/choir-qa/traces/c1e-b1-engineering-0444/`.

| time (UTC) | phase |
|---|---|
| 04:46:35 | the owner request is submitted through the prompt bar |
| 04:46:45 | Texture Casts to management (round 1) |
| 04:47:12 | management Casts engineering on records: assignment-703ad944..., parent is Texture's Cast |
| 04:47:22 | engineering's final Report: `stop_reason: cancelled`, record id `assignment-report:cancel-report:...`, no operation, no bundle digest |
| 04:58:02 / 04:58:13 | a second Cast (assignment-afc3c5a7...), cancelled 11 s later |
| 05:00:05 / 05:00:16 | a third Cast (assignment-979023d5...), cancelled 11 s later |
| 05:16:35 | the drill times out: no Freeze, no Verify, no decision |

So B1's first half works on staging: management casts engineering on
records, with the right parent. The second half doesn't: each assignment
ends with a cancel report about 10 seconds later, and engineering never
freezes.

## Candidates (hypotheses until the ledger says)

- B1 named these open points (station now card, 01:50 UTC):
  `stopCancelledAssignmentRun` is new; "the reconcile and the run-end
  settle can each mint the final Report". A reconcile that sees no
  engineering run yet, or a run that ended at once, may mint the cancel
  report.
- The engineering run may fail to start or end at once (capsule, mode,
  or module surface), and the run-end settle reports it as cancelled.
- Management may cancel its own Cast on its next turn (B1 gave management
  a cancel).

The next step is to read the trace: the assignment rows, the engineering
runs and their events, and who wrote each cancel report.
