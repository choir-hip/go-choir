# Capsule Runtime v3 — Consensus Synthesis

**Status:** Synthesis of 5-agent consensus panel review of v3 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, opencode, omp-gpt55, omp-gemini35 (codex failed)
**Outputs:** `/tmp/capsule-v3-consensus/`

## Headline

**Architecture sound (5/5). Two fatal-as-written issues, several doc gaps.**
All 5 agents agree the v3 architecture is sound. opencode said
"implementation-ready, no fatal flaws." The other 4 found 2 fatal-as-written
issues and several doc gaps that need fixing before implementation.

## Fatal-as-written (3/5 agents)

### F1. CommitEpoch check breaks system after first commit

**devin (headline), omp-gpt55, cursor.** The broker checks
`cap.CommitEpoch == broker.currentCommitEpoch` on **every** RPC.
`CommitManifest` increments `CommitEpoch` after each commit. So after the
first commit, every outstanding capability has a stale `CommitEpoch` and is
rejected. Since `MintCapability` is "once per agent-run per capsule, rare,"
there's no re-mint path. The system deadlocks after the first commit.

**Fix (converged):** `CommitEpoch` should only gate commit/diff verbs (where
stale view matters), not exec/read/write. Sessions survive commits;
`commitEpoch` is audit metadata for sessions, not a validation key.

### F2. Researcher wildcard routing is structurally broken

**cursor (headline), omp-gpt55.** `MintCapability` takes one `capsuleID`
argument. The researcher example passes `"*"`. But
`executor.GetCapsule(cap.CapsuleID)` is the only routing mechanism — and
`GetCapsule("*")` doesn't resolve to anything. There's no mechanism for
wildcard-scoped capabilities to route to a concrete capsule at call time.

**Fix:** Need explicit wildcard routing — executor-side expansion (resolve
`*` to the list of capsule IDs at call time via `ResolveTarget`).

## Strong consensus (4/5)

### S1. Super verbs in broker VerbSet is wrong

**cursor, devin, omp-gpt55, omp-gemini35.** `RoleVerbSets[RoleSuper]`
includes `spawn_capsule`, `destroy_capsule`, `mint_capability`, etc. These
are executor/host-control-plane verbs, not broker verbs.

**Fix:** Move super lifecycle verbs out of broker VerbSet. Super's broker
verbs are diagnostic reads only.

### S2. ExecResult missing SessionID field

**omp-gpt55, omp-gemini35, cursor.** `ExecParams` accepts `SessionID` and
broker creates one if missing, but `ExecResult` has no `SessionID` field.

**Fix:** Add `SessionID string` to `ExecResult`.

### S3. Network policy contradiction

**devin, omp-gpt55.** Decision doc says "air-gapped, no network namespace."
Implementation doc still has nftables/per-capsule network namespace section.

**Fix:** Delete the nftables section from the implementation doc.

### S4. mtime+size fast-path is unsafe against adversarial capsule

**omp-gpt55.** A workload can modify a file without changing size and
restore mtime, causing the commit diff to miss real changes.

**Fix:** Use mtime+size as cache hint only, never authoritative. For commit
capture, hash content.

## Minor (1-2 agents)

- ExecRequest.Timeout vs TimeoutMS mismatch (cursor, devin)
- SpawnSpec.CpuPeriod missing from implementation doc (cursor, devin)
- RevocationMessage not in RPC method table (cursor, omp-gemini35)
- Broker seccomp allowlist not enumerated (omp-gpt55, cursor)
- acquireOp doesn't check Quiescing/Frozen states (omp-gemini35)
- capabilityID referenced but undefined in session binding (omp-gpt55)

## What's Good

All 5 agents confirmed:
- **Snapshot diff** is correct and sound.
- **Ed25519 capabilities** are sound.
- **Role-based verb sets** are sound.
- **Bind-mounted broker** is sound.
- **Memory admission fix** is correct.
- **Privilege separation** (with CAP_DAC_OVERRIDE + CAP_FOWNER retained) is
  correct and reconciled with overlayfs copy-up.
- **Tape idempotency** is correct.
- **Manifest walker safety** is correct.
- **Manifest atomic write** is correct.

## Recommendation

**The v3 architecture is sound. The docs need another reconciliation pass.**
Before implementation:

1. **Fix CommitEpoch check semantics** (F1) — only gate commit/diff verbs.
2. **Fix researcher wildcard routing** (F2) — executor-side expansion.
3. **Move super verbs out of broker VerbSet** (S1).
4. **Add SessionID to ExecResult** (S2).
5. **Delete nftables section** (S3).
6. **Fix mtime+size fast-path** (S4) — cache hint only.
7. **Fix ExecRequest.Timeout vs TimeoutMS** (minor).
8. **Add CpuPeriod to impl doc SpawnSpec** (minor).
9. **Add sync_auth_epoch to broker method table** (minor).
10. **Add Quiescing/Frozen checks to acquireOp** (minor).
11. **Add CapabilityID field to Capability struct** (minor).

None of these are architectural changes. They're specification fixes.

## Raw Outputs

- Manifest: `/tmp/capsule-v3-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v3-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v3-prompt.md`
