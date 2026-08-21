# Final Consensus Panel Synthesis: Docs Alignment + Successor Definition

Date: 2026-08-21
Panel: 9 models (codex, devin, cursor, opencode, gpt-5.6-sol, gpt-5.6-luna, gemini-3.7, grok-4.6, deepseek)
Source: .agentic-consensus/final-docs-review-20260821/panel/
Verdicts: 1 SHIP (codex) / 4 HOLD (cursor, gemini37, grok46, gpt56-sol) / luna HOLD / others partial

## Convergent blockers — ALL FIXED in commit 0584bce2

### B1. Registry split-brain (all HOLD voters)
ACTIVE/mission-graph/manifest still named candidate-proof-2026-08-20 as the executable spine while the successor said `working`. A `/goal` run following registries would open the superseded file.
**Fixed:** mission-graph now has scheduling-and-candidate-proof as sole `working`+`entrypoint` node; candidate-proof demoted `superseded`/`entrypoint: false`; private-go depends_on retargeted; ACTIVE names the new Definition; NOW records the change; SEM-03 cites it.

### B2. Milestone-forcing heresy reintroduced by successor wording (codex, grok)
"every transition carries a Texture revision" / "each consequential transition has a Texture revision" was H037 (pipeline-stage ontology) in acceptance clothing.
**Fixed:** final_artifact and legibility gate now read: revisions appear wherever semantic state changed, authored by Texture agent, scheduler/assignment lifecycle never calls ApplyTextureTurn; census gate distinguishes turn outcomes from revisions.

### B3. Front matter not closed (luna — parser smoke test failed)
Successor Definition lacked closing `---`, so `/goal` parser would reject it.
**Fixed:** closed; front matter validated via yaml.safe_load.

### B4. Acceptance inheritance incomplete (codex, gemini)
Prior definition had `not_done_when` gates and `landing.required_receipts` that successor dropped; "five real refs" weakened the five specific capsule-bound reference types.
**Fixed:** inherited not_done_when (7 items including two new scheduler-specific ones), landing receipts restored, artifact text now says "five required refs bound to capsule-exec receipts."

## Non-blocker findings worth recording
- Mission-A single-trajectory wording is a scoped proof slice, not final topology (luna) — consistent with Mission B release-gate framing.
- "Verifier read-only" ambiguity flagged for capability code phase.
- Scheduler state-machine needs a durable state-transition table at implementation time (ordinal allocation, FIFO selection, expiry causes, timeout, retryable refusal, restart recovery, duplicate-operation dedup, late-evidence non-reopening) — recorded for Phase 1 implementation spec.
- codex verified Q2 clean under strict duplicate-rejecting YAML parse.

## Where this leaves us

All steering docs are aligned to the ratified architecture. The successor Definition (`docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`) is now:
- schema-complete per panel findings,
- registry-promoted as the sole executable `/goal`,
- free of the milestone-forcing heresy,
- inheriting the prior acceptance unchanged plus two scheduler-specific not_done_when gates.

Ready for owner invocation: `/goal docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`
