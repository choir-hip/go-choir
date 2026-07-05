# Capsule Runtime Implementation Review — v1 Consensus Synthesis

**Date:** 2026-07-05
**Panel:** claude, devin, omp-gpt55, omp-gemini35
**Codex excluded per user request.**

## Consensus Summary

**Verdict:** Safe to commit as explicitly-labeled scaffolding. NOT safe to enable in production. Two security fixes are merge-blocking. The runtime is non-functional (Spawn stubbed, exec tools stubbed) but the primitives (capability signing, manifest diff, classifier) are solid and tested.

## Strong Consensus (4/4 agents agree)

### Merge-Blocking Issues (fix before push)

1. **Path traversal in broker** — ALL 4 agents flagged this as critical.
   `filepath.Join(b.mergedDir, p.Path)` with `p.Path="../../etc/shadow"` escapes the capsule root. Every file verb is affected: read_file, write_file, list_dir, stat, mkdir, remove, exec cwd. Landlock is defense-in-depth but fails open on kernels without it.
   **Fix:** Add `resolveWithin(mergedDir, relPath)` helper, use in all file verbs.

2. **Missing capsule binding check in broker** — ALL 4 agents flagged this.
   Broker verifies signature, expiry, revocation, and verb — but never checks `cap.CapsuleID`/`TargetCapsule` against its own capsule ID. A capability minted for capsule A is accepted by capsule B's broker.
   **Fix:** Add `--capsule-id` flag to broker, check `cap.TargetCapsule == b.capsuleID` on every RPC.

### Structural Assessment (4/4 agree)

3. **Spawn is a TODO stub** — capsules marked StateActive with zero isolation. Acceptable for scaffold IF clearly labeled, but must not be enabled.
4. **Cosuper tools return not_implemented** — acceptable for first PR if labeled.
5. **Primitives are solid** — capability signing, manifest diff, classifier, transaction builder are merge-worthy.

## Medium Consensus (3/4 agents agree)

6. **sync_revoked_caps unauthenticated** (devin, claude, omp-gpt55) — any process that can reach the Unix socket can wipe the revoked set. Fix: require capability or peer credential check.
7. **Revocation log wildcard replay broken** (devin, claude, omp-gpt55) — `Sscanf("%s")` skips whitespace, so empty capsuleID fields shift columns. Wildcard revocations don't survive restart. Fix: use `strings.Split` with field count.
8. **vmMemoryUsed -= 0 on Destroy** (devin, claude, omp-gpt55) — memory budget permanently consumed. Fix: track and subtract the capsule's memory allocation.
9. **Diagnostics SafeOpenFile TOCTOU** (devin, claude, omp-gpt55) — uses EvalSymlinks not openat2. Fix: rename to avoid misleading claim, or implement real openat2.

## Minor Consensus (2/4 agents agree)

10. **FD hygiene broken** (claude, omp-gemini35) — closes /proc/self/fd while iterating it, or closes Go runtime epoll fds. Fix: use close_range or skip the directory fd.
11. **generateCapsuleID predictable** (devin, claude) — `time.Now().UnixNano()`. Fix: use crypto/rand.
12. **Landlock fail-open** (claude, omp-gpt55) — non-fatal warning on failure. Acceptable for now (best-effort by design).

## Single-Agent Findings (noted, not blocking)

- Signed Verbs field ignored (omp-gpt55) — uses role table not signed verbs
- BrokerClient.SyncRevokedCaps panics on nil cap (omp-gpt55)
- HostClient/BrokerClient concurrency race (claude) — shared conn no mutex
- Quiesce busy-spin (devin)
- Destroy doesn't wait for inflight (devin)
- classifyPath non-deterministic map iteration (devin)
- SyncRevokedCaps overwrites per-capsule with global (devin)
- Mint auth bypass for empty CapsuleID (claude)
- Shadowed err in handleExec stdin path (claude)
- Orphan process leaks in handleExec (omp-gemini35)

## Fixes Applied Before Push

Based on consensus, the following fixes are applied before commit:

1. **Path traversal fix** — `resolveWithin` helper in broker, used in all file verbs + exec cwd
2. **Capsule binding check** — `--capsule-id` flag, `cap.TargetCapsule == b.capsuleID` check
3. **sync_revoked_caps auth** — require super capability for revocation sync
4. **Revocation log replay** — use `strings.SplitN` instead of `Sscanf`
5. **vmMemoryUsed fix** — subtract capsule's memory on destroy
6. **generateCapsuleID** — use crypto/rand
7. **generateID entropy error check** — check rand.Read error
8. **classifyPath deterministic order** — iterate sorted kinds, not map

Remaining items (Spawn stub, FD hygiene, diagnostics TOCTOU, workload seccomp, concurrency mutex) are acceptable as TODO for a labeled scaffold PR and will be addressed in follow-up commits.
