# Devin review 385: the projection wedge in guest /health (2026-11-02)

The reviewer was one Devin seat, on orange commit 12e7a81f6 (branch health-projection-wedge), a guest security surface. The verdict was **approve**, with no blocking findings; high confidence on correctness and concurrency, medium on the error-content caveat.

Devin verified that the wedge mutex is a leaf under `a.mu` (a health probe cannot block behind an append), that every wedge-class return follows the head capture, that the HTTP code stays 200 and nothing on the host reads `status` from a 200 guest body, and that the log rate limit is correct. The Security-Effect claim ("none; read-only field, status code and auth unchanged") was judged accurate.

Findings and adjudication:

1. A wedge clears only on a successful append, so a repair that aligns heads without an append (or a post-commit non-wedge error) leaves /health degraded until the next append. **Accepted as a residual**: informational only, and every computer in this state appends again soon; a heads recheck in WedgeStatus is the follow-up.
2. The first refusal of a post-CAS wedge reports the pre-append heads. **Accepted (nit)**: later refusals report the split.
3. A raw Prepare error (at most 200 characters) reaches the unauthenticated guest /health. Today's Prepare errors carry no row content. **Deferred**: an error-kind allowlist as hardening.
4. The degraded path keeps only Content-Type from the base headers. **Accepted (nit)**.
5. Test gaps: a non-wedge error neither counts nor clears; a non-200 base passes through; a prepare-kind wedge reaching /health; LastError updating. **Deferred** with finding 1's follow-up.
6. and 7. Stale fields after clear (unobservable) and began/cleared pairs on a flapping wedge. **Accepted (nits)**.

Raw output: the session scratchpad (devin-wedge/out/devin.out), not retained in the repo.
