# Devin review: in-place prepared recovery in the event appender (2026-11-02)

The reviewer was one Devin seat, on red-class commit 504d778b3. The verdict was **approve**, with no blocking findings and high confidence on the mechanics.

Devin traced every writer of prepared rows. The dry-run appenders run on a separate store, the boot reconstruct runs under replay mode, and the sealer and group-commit paths run under `a.mu`. Re-running the platform CAS is idempotent: the platform answers from its existing receipt by `(computer_id, idempotency_key)` before the guarded head UPDATE.

Findings and adjudication:

1. A recovery that keeps failing stalls every append for up to 60 s. **Adopted**: after a failed recovery, appends refuse fast for a 30 s cooldown.
2. The live re-CAS branch is new in production. **Accepted**: the first in-place repair on a real computer is the proof; watch for the `in-place prepared recovery` log line.
3. The guard coverage is correct but thin. **Accepted**: the remaining race is pre-existing, not introduced here.
4. Test gaps. **Adopted in part**: a test that a reconstruct blocks the repair. **Deferred**: the cancelled-ctx discard test.
5. The retry reuses a possibly-dead ctx. **Rejected as harmless**: the orphan is already cleared.
6. The `livePayloads` entry is left after an in-place finalize. **Deferred**: it is tiny and bounded.
