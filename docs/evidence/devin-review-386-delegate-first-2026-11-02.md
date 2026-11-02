# Devin review 386: delegate first for source changes (2026-11-02)

One Devin seat, tier 1, on orange commit 3afbba64d (branch delegate-first-for-source-changes; touches internal/agentcore, a guest security surface). Verdict **approve**; no blocking findings; confidence medium-high (static review).

Verified clean: the delegate_use record is excluded from the report command digest and from evidence differences, so replay, determinism and acceptance are unaffected; nothing changes unless the capsule's network mode is claude; per-attempt reset and concurrency are correct; no token or document content reaches the record or the log. The Security-Effect claim ("none; prompt text and observability counters only") was judged accurate.

Findings and adjudication (all in the best-effort tally; the comparison's trace monitors count delegate calls from the ledger independently, so the tally is cross-checked, not trusted alone):

1. The terminal cell's own delegate calls and successes are folded in after the report is built: delegate_succeeded can undercount, and an all-in-one-cell attempt can log "without delegate" falsely. **Deferred** (residual), cross-checked by the monitors.
2. A cell whose commit fails after its freeze committed is not counted as a patch. **Deferred**; one-line fix noted.
3. Tallies for attempts that end by cancel, orphan close or crash stay in memory. **Deferred**; small and bounded by attempts per process lifetime.
4. The log predicate ignores counted successes. **Deferred** with 1.
5. Only Complete-intent reports carry delegate_use. **Accepted** for the first measurement pass.
6. to 8. Nits (escaped subtype match, empty-id guard, doc wording). **Accepted**.
