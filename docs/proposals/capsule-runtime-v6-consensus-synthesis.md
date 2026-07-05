# Capsule Runtime v6 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v6 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v6-consensus/`

## Headline

**2/5 ready, 3/5 almost — all remaining issues are doc hygiene + seccomp precision.**
opencode and omp-gemini35 say implementation-ready. The other 3 found the same
class of issues: stale references that survived the reconciliation pass, plus a
seccomp precision issue. No architectural issues, no fatal flaws.

## Strong Consensus (4/5) — all mechanical fixes

### S1. Stale sync_auth_epoch at design doc line 282

**cursor, devin, omp-gemini35, omp-gpt55.** Rename was applied to struct and
decision doc but missed in revocation prose.

### S2. Decision doc Q6 still says "Executor (host) holds private key"

**cursor, devin, omp-gemini35, omp-gpt55.** Fixed in implementation doc but
not decision doc.

### S3. Classifier package still in internal/capsule/

**cursor, devin, omp-gemini35, omp-gpt55.** Package layout wasn't updated.

## Medium Consensus (3/5)

### S4. Seccomp can't filter connect/bind by AF (pointer deref)

**cursor, devin, omp-gpt55.** seccomp-BPF can't dereference sockaddr pointers.
Should block socket(AF_INET/AF_INET6/AF_NETLINK) at creation.

## Minor (1-2/5)

- RevokeCapability signature mismatch (cursor, omp-gemini35)
- HostAuthority mint-request authorization unspecified (devin)
- Wildcard revocation persistence format (devin, omp-gpt55)
- devices.deny is v1, not v2 (omp-gpt55)
- HostAuthority struct missing fields (omp-gemini35)

## What's Good

All 5 agents confirmed the v6 architecture is sound. 2/5 said
implementation-ready. All remaining issues are mechanical doc fixes and one
seccomp precision fix. No architectural issues, no fatal flaws.

## Process Note

cursor noted: "The pattern across v3→v4→v5→v6 is the same failure mode
repeating: doc hygiene debt is being fixed by memory, not by verification."
v7 uses grep verification before claiming "reconciled."

## Raw Outputs

- Manifest: `/tmp/capsule-v6-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v6-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v6-prompt.md`
