# Capsule Runtime v12 — Consensus Synthesis

**Status:** Synthesis of 3-agent consensus panel review of v12 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** devin, omp-gpt55, omp-gemini35
**Outputs:** `/tmp/capsule-v12-consensus/`

## Headline

**1/3 ready, 2/3 almost — seccomp API sketch doesn't match real library.**
omp-gemini35 says implementation-ready. devin says largely ready with minor
caveats. omp-gpt55 found the seccomp code sketch doesn't match the actual
`elastic/go-seccomp-bpf` API and `ActionErrno` needs an errno payload.

## Strong Consensus (2/3) — seccomp API sketch

### S1. Seccomp code sketch doesn't match real library API

**omp-gpt55, omp-gemini35.** The sketch uses `Args []seccomp.Rule{Index,
Op, Value}` but the actual library uses `NamesWithCondtions` and
`Condition{Argument, Operation, Value}`. Also `ActionErrno` without an
errno payload can make syscalls appear to return success.

## Minor (1/3)

- CLONE_NEWUSER privilege-separation sequence underspecified (devin)
- gVisor "Rejected for MVP" stale text (omp-gpt55)
- close_range fallback for kernels < 5.9 (omp-gemini35)

## What's Good

All 3 agents confirmed the v12 architecture is sound. No fatal flaws. The
remaining issue is that the seccomp code sketch is illustrative pseudocode,
not copy-paste-ready against the real library. This is an implementation-time
concern, not a design flaw.

## Raw Outputs

- Manifest: `/tmp/capsule-v12-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v12-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v12-prompt.md`
