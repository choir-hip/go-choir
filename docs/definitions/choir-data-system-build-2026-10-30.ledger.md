# Ledger: choir-data-system-build-2026-10-30

Append-only. One terse entry per pass. Never edit earlier entries.

- 2026-10-30 ~06:15 UTC (FP0). Goal authored on the owner's word ("start
  building this ... iron out the kinks first"). The selection goal was
  superseded. Frame review round 1 at 3f4dafb7e:
  - Devin: accept_with_notes. Five paragraph-scale fixes, F1 to F5: the PSA
    proof bound, replay semantics against the §12 stake, a closed effect
    set, outbox backpressure, and fence before stop.
  - Sonnet: send_back. Seven blocking findings, B1 to B7:
    - B1, custody and audit overclaim; the DEK sits on disk.
    - B2, what the host reads; the class is writer-declared.
    - B3, PSA cadence, signing and per-object keys.
    - B4, the effect set and host-mediator barrier; file-sync loss.
    - B5, refusal, writer epoch, fsync and receipt durability; semantic
      kinds stay synchronous.
    - B6, class-pure events, chain integrity, restore composition and
      release.
    - B7, deletion safety: pending uploads, refcount, quarantine.
  - DeepSeek: still running.

  Output is in .agentic-consensus/frame-r1/out/. The overlap is high, and
  the next move is adjudication plus frame revision 2.
- 2026-10-30 ~06:30 UTC (FP0). Owner: "Remember, we are working at agentic
  speed. We can't let code review slow us down. We reviewed the plan a bunch
  of times for a reason. So we can do the migration in bulk, and let runtime
  checks be the verifier, not code review." Recorded in the goal:
  - D0 folds round 1 once; there is no round 2 and no Codex;
  - D1 and D2 run without panels;
  - B1 to B7 are built in bulk, enabled for the proof ring only;
  - verification is by B0's drills, the hard gates, the differential and
    deployed acceptance;
  - the CLAUDE.md review tiers are superseded for this mission.
- 2026-10-30 ~06:45 UTC (FP0). Owner: "Skip tonight's backup. Accelerate the
  migration cutover. Yes, do the fleet cutover, and delete old data.
  Downtime is acceptable ... Better to rapidly get it 85% there, and iterate
  for the last mile."
  - The backup is skipped: the timer was stopped and a runtime drop-in
    added (skip-during-cutover.conf).
  - The goal is restructured:
    - Phase 1 runs D0 (fold round 1, once), D1 (the build sheet), C0 (the
      drills), C1 to C3 in parallel (guest SQLite, PostgreSQL, tape v2),
      C4 (the migration, on d@e first) and C5 (the fleet cutover and
      deletion of the old data).
    - Phase 2, I1 to I7, iterates the rest.
  - Owner gates are removed.
  - The last off-host copy of the old data is the restic snapshot of
    10-29 03:28 UTC.
