# Capsule Runtime v9 — Consensus Synthesis

**Status:** Synthesis of 3-agent consensus panel review of v9 design (cursor
timed out, opencode failed, codex failed).
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** devin, omp-gpt55, omp-gemini35
**Outputs:** `/tmp/capsule-v9-consensus/`

## Headline

**1/3 ready, 2/3 almost — abstract Unix socket issue + struct divergence.**
omp-gemini35 says implementation-ready. devin found struct divergence.
omp-gpt55 found a potentially serious issue with abstract Unix sockets.

## New Issue (1/3, potentially serious)

### A1. Abstract Unix sockets bypass mount namespace isolation

**omp-gpt55.** Abstract Unix sockets are scoped by network namespace, not
mount namespace. Without CLONE_NEWNET, a workload could create AF_UNIX
sockets and communicate across capsule boundaries via abstract namespace.

## Medium Consensus (2/3)

### S2. Struct divergence between design and implementation docs

**devin.** Design doc and implementation doc claim "identical Executor
structs" but they're not — design doc has concurrency fields, implementation
doc has exported fields.

## Minor (1/5)

- Broker allowlist missing openat2, clone/execve (omp-gpt55)
- seccomp OpEq multi-value semantics library-specific (devin, omp-gemini35)

## What's Good

All 3 agents confirmed the v9 architecture is sound. No fatal flaws (except
the abstract socket issue which is fixable with CLONE_NEWNET). The design has
converged to the point where remaining issues are small and mechanical.

## Raw Outputs

- Manifest: `/tmp/capsule-v9-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v9-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v9-prompt.md`
