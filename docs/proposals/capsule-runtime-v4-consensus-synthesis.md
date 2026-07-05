# Capsule Runtime v4 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v4 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v4-consensus/`

## Headline

**1/5 says ready, 4/5 say not ready.** New architectural issue found:
host/guest authority boundary contradiction. Plus spec contradictions in
diagnostic verbs, commit_transaction, and revocation blast radius.

## New Architectural Issue (1/5, but load-bearing)

### A1. Host/guest authority boundary contradiction

**omp-gpt55.** The executor holds the Ed25519 private key on the Firecracker
host (outside guest kernel), but also directly manages namespaces, cgroups,
overlay mounts inside the guest. A host process can't create Linux namespaces
inside the guest kernel. Needs two-plane split: HostAuthority (host) +
Executor (guest) + vsock transport.

## Strong Consensus (4/5)

### S1. Super diagnostic verbs contradiction

**cursor, devin, omp-gpt55, omp-gemini35.** inspect_capsule_raw/extract_diff/
list_capsules listed as both broker verbs AND host-side bypass tools.

### S2. commit_transaction is both broker-gated and host control-plane

**cursor, devin, omp-gpt55.** Named as CommitEpoch-gated broker verb but also
declared a host control-plane verb.

### S3. AuthEpoch global revocation blast radius

**omp-gpt55, omp-gemini35, devin.** Bumping capsule-wide AuthEpoch revokes
ALL agents sharing that capsule. Need per-capability revocation.

## Medium Consensus (2-3/5)

### S4. AuthEpoch not restored on broker restart

**devin.** Broker crash → currentAuthEpoch resets to 0 → all revoked caps pass.

### S5. Session binding to commitEpoch could reintroduce F1

**cursor, omp-gpt55.** If enforced, sessions die on every commit. Make
audit-only.

### S6. SessionID dropped from bash tool JSON output

**devin, omp-gemini35.** Bash tool doesn't return session_id.

### S7. CLONE_NEWNET still in namespace example

**omp-gpt55.** Contradicts "no network namespace" decision.

## What's Good

All 5 agents confirmed the core architecture is sound. opencode said
"implementation-ready, no fatal flaws." The issues are spec contradictions
and one architectural boundary issue, not fundamental design flaws.

## Raw Outputs

- Manifest: `/tmp/capsule-v4-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v4-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v4-prompt.md`
