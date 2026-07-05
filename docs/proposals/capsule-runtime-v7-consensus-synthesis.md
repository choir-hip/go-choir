# Capsule Runtime v7 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v7 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v7-consensus/`

## Headline

**2/5 ready, 3/5 almost — one new security issue (AF_VSOCK), rest is doc hygiene.**
opencode and omp-gemini35 say implementation-ready, no fatal flaws. The other
3 found one new security issue and minor doc hygiene items.

## New Security Issue (1/5, potentially serious)

### A1. AF_VSOCK not blocked by seccomp

**cursor.** The seccomp network enforcement blocks AF_INET/AF_INET6/AF_NETLINK
but not AF_VSOCK. A compromised cosuper could open `socket(AF_VSOCK)` and dial
`CID_HOST` directly, reaching the HostAuthority's listener. This bypasses the
entire two-plane security model. CID is per-VM, not per-process.

**omp-gpt55** also noted: inherited vsock fds are another vector — need fd
hygiene (close_range/FD_CLOEXEC).

## Strong Consensus (3/5) — doc hygiene

### S1. package classifier vs cmd/capsule-host

**devin, omp-gpt55, omp-gemini35.** Implementation doc still says `package
classifier` in the sketch, but classifier.go is now under cmd/capsule-host/.

## Minor (1-2/5)

- Decision doc line 20-21 says "needs re-run" (omp-gpt55)
- Classifier unknown-path policy unresolved (omp-gpt55)
- Executor struct missing globalRevokedCaps field (devin)
- io.device is wrong cgroup v2 term (omp-gpt55)
- RegisterCapsule/RegisterActiveRun RPCs not sketched (omp-gemini35)
- logFile vs revocationLog naming (cursor)

## What's Good

All 5 agents confirmed the v7 architecture is sound. 2/5 said
implementation-ready. The AF_VSOCK gap is the one substantive issue — it's a
real security gap that needs closing. The rest is doc hygiene.

## Raw Outputs

- Manifest: `/tmp/capsule-v7-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v7-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v7-prompt.md`
