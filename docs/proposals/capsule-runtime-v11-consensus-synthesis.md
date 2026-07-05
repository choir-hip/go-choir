# Capsule Runtime v11 — Consensus Synthesis

**Status:** Synthesis of 3-agent consensus panel review of v11 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** devin, omp-gpt55, omp-gemini35
**Outputs:** `/tmp/capsule-v11-consensus/`

## Headline

**0/3 ready — two more stale references + seccomp AND/OR bug.**
All 3 agents found issues. The architecture is confirmed sound (no fatal
flaws), but two mechanical issues remain.

## Strong Consensus (3/3) — two mechanical fixes

### S1. Two more stale "No network namespace" references in decision doc

**devin, omp-gemini35.** Lines 373-374 (Q3) and 465-466 (Q24) still say
"No network namespace" — wrapped across line breaks, so line-based grep
missed them.

### S2. Seccomp AND/OR logic bug

**devin, omp-gemini35, omp-gpt55.** Multiple seccomp.Rule entries in a
single Args slice are ANDed, not ORed. The filter requires arg[0] == 2
&& arg[0] == 10 && ... simultaneously — impossible, so the filter never
triggers. Each denied family needs its own SyscallGroup.

## Minor (1-2/3)

- CLONE_NEWUSER ambiguity in namespace sketch (omp-gpt55, omp-gemini35)
- seccomp API field names don't match actual library (omp-gpt55)

## What's Good

All 3 agents confirmed the v11 architecture is sound. No fatal flaws. The
remaining issues are mechanical: two wrapped stale references and a seccomp
logic bug. Both are small fixes.

## Process Note

devin noted: the line-based grep verification missed wrapped references.
The fix is to use multiline grep (`rg -U`) for verification, not
line-based grep. This is a methodology fix, not just a content fix.

## Raw Outputs

- Manifest: `/tmp/capsule-v11-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v11-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v11-prompt.md`
