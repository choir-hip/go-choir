---
id: 20261128T1410Z-key-read-lane-lands-first-run
recorded: 2026-11-28T14:10Z
who: lead
kind: landing
p: 0.75
settle_by: 2026-11-28T20:00Z
evidence_source: the land-queue report for lane session-unreadable (attempts, CI conclusion on the pushed commit, staging commit)
status: settled
settled: 2026-11-28T12:05Z
outcome: true
evidence: land-queue report for session-unreadable; attempts 1, pushed be8b0dc6e, CI green with no re-run (538 s), staging serves it; shared-code tests in the landing 2012 pass, 3 skip, 0 fail
brier: 0.0625
---
Lane `session-unreadable` (cut down after four reviews; accepted by both seats on 6a28b7b85) lands on the queue's first attempt: both phone apps compile, the shared-code tests pass in the landing, CI on the pushed commit is green with no re-run, and staging serves it. The lane touches 29 files of shared sign-in code and was rebased once over the Activity fix, so a test that reads source text could still disagree with main.
