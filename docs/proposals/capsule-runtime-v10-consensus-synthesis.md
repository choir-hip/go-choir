# Capsule Runtime v10 — Consensus Synthesis

**Status:** Synthesis of 3-agent consensus panel review of v10 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** devin, omp-gpt55, omp-gemini35 (cursor timed out, opencode failed, codex failed)
**Outputs:** `/tmp/capsule-v10-consensus/`

## Headline

**0/3 ready — stale "no network namespace" references everywhere.**
All 3 agents found the same issue: the CLONE_NEWNET fix was applied to ~6
locations but ~7 stale "no network namespace" references were left behind.
The implementation doc's Section 8 still says "No network namespace is
created" — directly contradicting v10.

## Strong Consensus (3/3) — one mechanical fix

### S1. Stale "no network namespace" references

**devin, omp-gpt55, omp-gemini35.** All found the same cluster of stale
references:
- Implementation doc Section 8, library stack table, summary line
- Design doc "What Stays" section, v6 Decisions section
- Decision doc defense-in-depth list, custom Go runtime components

## Minor (1-2/3)

- seccomp OpEq multi-value needs expansion to individual rules (devin,
  omp-gemini35, omp-gpt55)
- Struct note too narrow — omits non-concurrency fields too (omp-gpt55)
- CLONE_NEWUSER conflict — workload has no user namespace but code sketch
  includes it (omp-gpt55)

## What's Good

All 3 agents confirmed the v10 architecture is sound. No fatal flaws in the
architecture. The only issue is stale text. The fix is mechanical.

## Raw Outputs

- Manifest: `/tmp/capsule-v10-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v10-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v10-prompt.md`
