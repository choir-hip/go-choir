# irreversible-email-v1 / human-required-v1 policy-bytes review — 2026-08-16

**Candidate:** `docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md`
**irreversible-email-v1 digest:** `d83c215443638ad0cfd95eb174de61e1f203414d8ce0631cb68f0dca106c6bbc`
**human-required-v1 digest:** `33f5dc442d7fd0d028b76a6792954add090dafcaff87cc2ff1a1829a08df9b96`
**HEAD at review:** `20d2ac4c`

**Panel:** `.agentic-consensus/effects-irreversible-email-v1-20260816/` (gitignored)

## Verdicts

| Reviewer | Status | Verdict |
|---|---|---|
| OMP Gemini 3.6 | ok | ACCEPT (verified both digests) |
| OMP Cursor Grok 4.5 | ok | ACCEPT (verified both digests) |
| OMP GPT-5.6 Sol | ok | ACCEPT (verified both digests) |
| Devin | failed | no verdict (empty output) |

Adjudication: **ACCEPT**. Completeness bars passed: quorum integers frozen; verification 2-of-2 plus required third domain `external_effects`; author recused from verification and external_effects and does not count; same-signer as author or verification signer is refuse; typed bounds present; dispatch contract has all six required fields; recipient / payload_digest / actuator / acceptance_inbox exact-bound; `irreversible-email-v1.human_seat = absent`; `human-required-v1` fails closed when the owner-human seat is absent; restore-does-not-unsend is explicit; both digests match compact canonical bytes.

Rejection bars closed: this ACCEPT does not delete `external-owner:`; does not arm effects; does not rematerialize; does not treat restore as able to unsend; does not let `reversible-selfdev-v1` authorize this subject; does not send mail or wire an outbox as part of this review.

Non-blocking implement constraints named by the panel:

- Concrete owner-controlled acceptance inbox address, provider identity, and credentials are implement-time joins to frozen eligibility/subject rules, not new quorum.
- Signer-provenance equality across independence domains remains hard refuse (do not treat seat labels as eligibility).
- Crash/unknown provider outcomes stay non-green until reconciliation or compensation.
- This ACCEPT does not delete `external-owner:` or arm effects.

**Not a live send.** Trusted-outbox dispatch is the next red slice. Supervision wiring and rehearsal remain unpaid.

Effects remain OFF.
