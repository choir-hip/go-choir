# SH frozen-candidate panel review — 2026-10-08

Panel: agentic-consensus default panel, convergent mode. 9 of 10 answered
(codex, devin [tool-gated, no review], claude-opus, omp gpt-6.1-sol,
gpt-6-luna, gemini-3.8-flash, muse-spark, deepseek-v4.1-flash,
glm-5.3-flash; opencode failed: invalid API key). Subjects: `c78979b8`
(slice 1, pushed), `c990bf50` (slice 2, unpushed), `992727a8` (slice 3
design). Raw outputs kept outside the repo (session scratchpad).

Verdict (unanimous among reviewing members): **slices 1–2 ship after named
fixes; slice 3 design approve with changes.**

## Findings and disposition

| # | Finding (raised by) | Severity | Disposition |
|---|---|---|---|
| F1 | Guest `EnsureCustodianEscrow` skips when any custodian record exists without comparing digest; a pre-genesis replacement realization escrows nothing and mints genesis under an unescrowed key (claude, codex, sol, muse) | blocker | fix: compare digest; pre-genesis mismatch uploads (replacement); post-genesis mismatch is a loud error |
| F2 | Pre-genesis replacement: head read and UPDATE not atomic; `RowsAffected` unchecked so a losing writer gets success (all) | blocker/major | fix: one transaction, escrow row locked, head read inside; require 1 row else conflict. Residual: a concurrent zombie pre-genesis realization racing genesis is fenced by vmctl single-realization epochs, not by this lock |
| F3 | Transient platform errors (credential acquire, Head, required escrow) now refuse promptly and fail the start instead of systemd retry inside the host wait (claude, deepseek) | major | fix: retryable sites exit for systemd restart (no refusal); typed refusal only for deterministic failures |
| F4 | "Readiness served" inferred from `pending=false`, which clears before lifecycle reconcile and runtime start (gemini, luna, sol) | major | fix: gate records that a ready answer was actually served; hold the window otherwise. Pre-existing early un-gating noted, not changed |
| F5 | Refused fresh-start condition is cleared then re-stored; a crash between loses `FreshRealization` (sol) | major | fix: replace condition atomically (store refusal over the old one; clear only on admit) |
| F6 | 300 s Retry-After dropped by `refusalFromCondition` (deepseek, sol, muse) | minor | fix: per-kind retry hint |
| F7 | Replay-only base-refusal path still holds the 5-minute window (luna, claude D5, muse D6) | minor | fix: route through `startupFailer` |
| F8 | Durable `privacy_key_unavailable` cannot reopen except via maintenance bypass (claude D4, muse D3, luna) | minor | accepted for now; slice 3 obligation: reopen when a key source exists |
| F9 | Head sequence 0 sentinel (deepseek D5) | minor | assumption recorded: genesis writes sequence >= 1 |

## Slice 3 design changes required

- `request_commitment` is an issuance identifier, not proof the caller owns
  that issuance: any holder of the computer capability could claim the
  slot with its own recipient key (DoS + wrong-recipient audit). Bind the
  recipient key into the one-time credential exchange (claude hybrid,
  sol/codex prefer alternative (a)); or require envelope-possession proof.
- Crash safety: an in-memory ephemeral key plus consume-once bricks a
  realization that crashes after delivery. Recover by re-issuing the
  envelope (claude) or persisting the ephemeral key until the key file is
  durable (gemini).
- Verify the delivered key against the tape (decrypt one known private
  artifact) before writing it; write via temp file, fsync, rename.
- Transparency entry in the same transaction as the reservation, before
  unwrap. Delivery stays off the canonical tape as authority; the guest may
  append a non-secret delivery lifecycle receipt after recovery.
- Admission reopens a durable key condition when a verified key source
  exists.
