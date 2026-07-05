# Capsule Runtime v5 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v5 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v5-consensus/`

## Headline

**2/5 ready, 3/5 almost — converging on doc hygiene + 3 spec gaps.**
opencode and omp-gemini35 say implementation-ready, no fatal flaws. The
other 3 found stale decision doc entries and 3 spec gaps.

## Strong Consensus (4/5)

### S1. Stale decision doc entries

**cursor, devin, omp-gpt55, omp-gemini35.** Q12-Q14, Q18-Q20 still have
old v3/v4 language contradicting v5 resolutions in the same file.

## Medium Consensus (3/5)

### S2. HostAuthority revocation persistence

**cursor, devin, omp-gpt55.** No disk persistence for revoked-capability
set. If HostAuthority restarts, all revocations are lost.

### S3. vsock auth / threat model overclaim

**devin, opencode.** vsock is guest-kernel — LPE can impersonate Executor.
Threat model overclaims. Should downgrade to "protects against user-space
compromise; microVM is primary for kernel compromise."

## Minor (1-2/5)

- Network isolation enforcement (omp-gpt55)
- Wildcard researcher revocation (omp-gpt55, omp-gemini35)
- sync_auth_epoch misnomer (devin, omp-gemini35)
- Classifier placement (omp-gpt55)
- MintCapability signature (omp-gemini35)
- Host-process fallback substrate (cursor)

## What's Good

All 5 agents confirmed the core v5 architecture is sound. 2/5 said
implementation-ready. The issues are doc hygiene and spec gaps, not
architectural flaws.

## Raw Outputs

- Manifest: `/tmp/capsule-v5-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v5-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v5-prompt.md`
