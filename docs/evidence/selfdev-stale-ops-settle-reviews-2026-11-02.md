# Stale open self-development operations: review rounds 1-4 (2026-11-02)

Branch selfdev-stale-ops-settle (6b025efb8, 8a6cc8829, 779f80b72, 5a963b080), landed squashed. Red: self-development state machine, updater journal, tape, owner acts. Problem and design table: docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md.

- Round 1 (Devin, DeepSeek v4.1 flash): send_back. Abandon refused materializing/rollback_pending; TOCTOU between the decision and the settle; no read classes for the pinned release.
- Round 2 (both): send_back. Pending discharged before the guarded row move (a lost race wedges); "no journal" did not prove nothing was in flight (the daemon journaled after staging); rollback read the pinned manifest before the journal, so applied + unusable release retried forever.
- Round 3 (both): send_back. A, B and C above held. New: a dead journal past the swap with GC'd inputs, and spent apply attempts, had no owner act; the suite was flaky-red from the row-first ordering.
- Substrate step-back after three rounds: one rule. Every settle takes the apply key lock non-blocking; held refuses; a terminal journal is the authority; a dead journal is arbitrated on the updater's current pointer (prior or no journal: failed; target: forward; neither or corrupt: failed, effective release unknown). An old daemon without the .key-lock-v1 marker cannot prove a journal dead, so abandon refuses there.
- Round 4 (Devin, one seat): approve. All round-3 blockers closed; no settle-wrong case found under a proven lock. Non-blocking residuals: a lock_unproven hold with a dead non-terminal journal (old-daemon window, needs the base image to carry this updater build); an arbitrated forward records no applied event when pending is not the op's decision; replayAbandon note mismatch; RestoredPriorEffective on unknown_effective discharges; fence failure log-only; incomplete-journal tamper case; perpetual 60s retry wake on lock_unproven holds.

Raw outputs (off-repo): the session scratchpad rev-staleops, rev-staleops2, rev-staleops3, rev-staleops4.
