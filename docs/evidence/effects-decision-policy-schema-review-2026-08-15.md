# Effects decision-policy schema independent review — 2026-08-15

**Candidate:** `docs/evidence/effects-decision-policy-schema-2026-08-15.md` at `ee408100`, then repaired in the same file by addendum.

**Panel:** `.agentic-consensus/effects-decision-policy-schema-20260815/` (gitignored diagnostics)

**Mode:** convergent. Required verdict: ACCEPT | REPAIR | REJECT | ESCALATE

## Verdicts

| Reviewer | Verdict |
|---|---|
| Devin | ACCEPT |
| OMP Gemini 3.6 | ACCEPT |
| OMP Cursor Grok 4.5 | ACCEPT |
| OMP GPT-5.6 Sol | REPAIR |

Six rejection bars passed on all four completed routes. Adjudication: **REPAIR**, because Sol's gaps are authorization-critical for a red cutover (blast-radius fields, unfrozen first-policy numbers, ballot attestation, policy-selection ordering proof, two reducer stages, digest encoding, irreversible dispatch contract).

The schema file now includes that addendum. Next action is review of the **repaired** freeze, not implementation.

Effects remain OFF. Do not delete `external-owner:` / `accept_once` / `awaiting_approval`. Do not rematerialize. Do not invent `choir computer create`.
