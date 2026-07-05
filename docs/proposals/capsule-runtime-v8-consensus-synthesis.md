# Capsule Runtime v8 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v8 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v8-consensus/`

## Headline

**2/5 ready, 3/5 almost — convergence achieved, remaining issues are code sketch sync.**
opencode and omp-gemini35 say implementation-ready. The other 3 found the same
two issues: design doc Executor struct missing globalRevokedCaps, and seccomp
code sketch missing argument filtering. No fatal flaws, no architectural issues.

## Strong Consensus (4/5) — two mechanical fixes

### S1. Design doc Executor struct missing globalRevokedCaps

**cursor, devin, omp-gpt55, omp-gemini35.** Added to implementation doc but not
design doc's Executor struct sketch.

### S2. Seccomp code sketch missing argument filtering

**cursor, devin, omp-gpt55, omp-gemini35.** The seccomp code example only shows
syscall-name denylist, not the socket(AF_*) argument filtering that the prose
describes. An engineer following the sketch would block all sockets (breaking
AF_UNIX) or none.

## Minor (1-2/5)

- CLOSE_RANGE_CLOEXEC wording (devin) — marks CLOEXEC, doesn't close
- RegisterCapsule/RegisterActiveRun not in design doc (omp-gpt55, omp-gemini35)
- Namespace matrix unclear (omp-gpt55)
- Executor restart recovery (opencode)

## What's Good

All 5 agents confirmed the v8 architecture is sound. No fatal flaws. The
remaining issues are the smallest found across all 8 rounds — two code sketch
syncs and one wording clarification. The design has converged.

## Raw Outputs

- Manifest: `/tmp/capsule-v8-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v8-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v8-prompt.md`
