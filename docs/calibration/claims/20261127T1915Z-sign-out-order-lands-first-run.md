---
id: 20261127T1915Z-sign-out-order-lands-first-run
recorded: 2026-11-27T19:15Z
who: lead
kind: landing
p: 0.75
settle_by: 2026-11-28T02:00Z
evidence_source: the CI run on main for the landed commit (gh run list --commit <sha>) and the commit staging serves
status: open
---
The sign-out order fix (lane sign-out-order, accepted by both seats at af5d69455)
lands on its first queue run: gates pass, CI on main is green and staging serves
the commit. The suite passed locally with one mail-seal test skipped; the risk is
a gate or CI Swift job that runs that test, or a timing test that flakes on CI.
