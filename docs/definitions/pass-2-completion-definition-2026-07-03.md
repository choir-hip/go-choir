# Definition: Pass 2 Completion — Autoputer/Autopaper Spec-First Suite

**Status:** under_deliberation → proposed settlement  
**Date:** 2026-07-03  
**Governed by:** `definitions` skill (live semantic control)  
**Orchestrator:** Devin (current session)  
**Next execution:** this document is executable by any agent with the same repo context.

---

## Determined State

### Settled claims

- **C-S4 SUPPORTED** — `promotion_protocol.tla` is model-checked green on `main` (CI run `28648508586`, 826 states, 0 errors).
- **C-S5 SUPPORTED** — `promotion_protocol.tla` encodes `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`, and liveness `EveryCommittedPromotionSettles` / `SystemProgress`.
- **C-D1 SUPPORTED** — CI passed on `main` after the promotion gate commits.
- **C-D2 SUPPORTED** — TLA+ specs model-check in CI.
- **Codex review completed** — `docs/reviews/promotion-gate-codex-review-2026-07-03.md` verdict: approve with reservations. Major findings documented before Mission C encodes promotion logic.
- **Pass 2 work landed on branch `mission-a-api-handler-extraction`** — combined branch containing:
  - Mission S: `specs/actor_protocol.tla` + `.cfg`, `specs/autoputer_lifecycle.tla` + `.cfg`, updated `specs/README.md`.
  - Mission A: `internal/provideriface/model_policy.go` + test, moved from `internal/runtime`; `internal/apihandler/api.go` wrapping runtime handlers.
  - Mission D: Codex review artifact.
- **PR #42 opened** — `https://github.com/choir-hip/go-choir/pull/42`.

### Observed (in progress)

- CI run `28651170805` on PR #42:
  - `TLA+ Model Check (specs/)` — **pass** (11s).
  - `Docs Truth Check` — **pass**.
  - Go build/test/race jobs — **pending** (expected to complete within ~5 minutes).

---

## Definition Graph

### Node 1: `pass-2-green`

```yaml
id: pass-2-green
kind: status
status: under_deliberation
source: observed
term: Pass 2 is green
non_definition:
  - "PR exists" alone does not make Pass 2 green.
  - "TLA+ passed" alone does not make Pass 2 green.
  - "Go tests passed locally" does not make Pass 2 green (local ICU env is broken).
examples:
  - PR #42 CI run shows all required jobs passed (not pending, not skipped, not failed).
  - PR #42 is approved and merged into `origin/main`.
  - Post-merge `main` CI run passes.
  - Staging deploy impact is either skipped (no runtime behavior change) or green.
counterexamples:
  - Any CI job on PR #42 fails or is cancelled.
  - PR is merged with a red TLA+ or Go test gate.
  - Post-merge `main` CI fails.
observables:
  - `gh pr checks 42` returns all required jobs as `pass`.
  - `gh pr merge 42 --squash` succeeds.
  - Post-merge `gh run list --workflow=ci.yml --branch=main` shows the merge commit CI as `success`.
execution_effect:
  - Once settled, Pass 2 is closed and Pass 3 begins.
  - The suite variant drops from 14 to 12 conjectures (C-S1, C-S3, C-A2 become SUPPORTED).
settlement:
  rule: "PR #42 CI fully green and PR merged to main with post-merge CI green."
  settled_by: orchestrator
  invalidation_triggers:
    - Any required CI job fails.
    - Merge conflict requires rebase.
    - Post-merge CI fails.
```

### Node 2: `combined-branch-vs-split`

```yaml
id: combined-branch-vs-split
kind: boundary
status: proposed
source: operational-preference
term: Accept combined Pass 2 branch
non_definition:
  - "Separate branches per mission" is preferred but not required if the combined branch is reviewable.
  - "One PR per mission" is not a hard suite invariant.
examples:
  - PR #42 title and body clearly enumerate the Mission S/A/C/D contents.
  - Reviewers can identify each mission's changes in the diff.
  - CI passes on the combined branch.
  - Follow-up PRs split remaining work cleanly.
counterexamples:
  - Combined branch mixes unrelated runtime changes with spec changes.
  - Reviewer cannot tell which change belongs to which mission.
  - Combined branch breaks one mission while greening another.
observables:
  - PR #42 body lists Mission S, Mission A (helper + API), Mission D deliverables.
  - Diff stat shows only `specs/`, `internal/apihandler/`, `internal/provideriface/`, `internal/runtime/`, `docs/reviews/`.
execution_effect:
  - Accept combined branch for Pass 2.
  - Pass 3 must return to per-mission branches.
settlement:
  rule: "PR is reviewable, CI green, and body documents the combined scope."
  settled_by: orchestrator
  invalidation_triggers:
    - Human requests split before merge.
    - CI failure in one mission area blocks the others.
```

### Node 3: `liveness-deferred`

```yaml
id: liveness-deferred
kind: invariant
status: proposed
source: reviewer
term: Defer two liveness properties to a later spec pass
non_definition:
  - "Spec is incomplete" is not the same as "spec is wrong".
  - "Model-checks green" does not mean every desired property is proven.
examples:
  - `actor_protocol.tla` safety invariants hold and liveness is documented as deferred.
  - `autoputer_lifecycle.tla` safety invariants hold and `EventuallyHealthy` is removed/replaced.
  - A follow-up Mission S pass explicitly targets the deferred liveness properties.
counterexamples:
  - Safety invariants are weakened to make liveness trivial.
  - Liveness is silently dropped without documentation.
  - Mission C code claims the spec proves deferred properties.
observables:
  - `specs/actor_protocol.cfg` contains a comment explaining deferred liveness.
  - `docs/mission-suite-autoputer-autopaper-spec-first-v0.md` lists deferred liveness as a Pass 2 known limitation.
  - Pass 3 planning includes a Mission S subtask to restore liveness.
execution_effect:
  - Pass 2 closes with safety-only specs for actor and lifecycle.
  - Pass 3 must design crash/re-activation fairness and healthy-boot liveness.
settlement:
  rule: "Deferred liveness is documented and safety invariants remain model-check green."
  settled_by: orchestrator
  invalidation_triggers:
    - Human rejects the deferral.
    - Safety invariants are found insufficient.
```

---

## Completion Semantics

```text
Pass 2 is COMPLETE when:
  1. PR #42 CI is fully green (all required jobs pass).
  2. PR #42 is merged into origin/main.
  3. Post-merge main CI is green.
  4. The suite ledger is updated with Pass 2 decisions and conjecture status.
  5. The suite paradoc is updated to reflect Pass 2 completion and Pass 3 plan.

Pass 2 is INCOMPLETE if any of the above are not satisfied.
```

---

## Forbidden Collapses

- Do not collapse "TLA+ passed" → "Pass 2 complete".
- Do not collapse "PR opened" → "work delivered".
- Do not collapse "Go tests passed in CI" → "no runtime regressions".
- Do not collapse "combined branch" → "single mission".

---

## Next Operators (autonomous execution)

1. **monitor(node: pass-2-green)** — poll `gh pr checks 42` until all jobs pass or fail.
2. **if pass-2-green settles**:
   - `merge(PR 42)` — squash merge to main.
   - `monitor(node: main-ci-green)` — poll post-merge CI.
   - `update_ledger()` — append Pass 2 settlement to `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md`.
   - `update_paradoc()` — mark C-S1, C-S3, C-A2 as SUPPORTED in `docs/mission-suite-autoputer-autopaper-spec-first-v0.md`.
   - `open_pass_3()` — create definition for Pass 3 (wire pipeline + remaining actor runtime defactoring + autoputer rename promotion encoding).
3. **if pass-2-green fails**:
   - `diagnose()` — identify failing job and root cause.
   - `fix()` — commit fix to the same branch.
   - `re_monitor()` — return to step 1.

---

## Human Authority Reserved

Escalate to the human if:
- PR #42 CI fails in a way that requires a group-level decision (e.g., weaken an invariant, split the branch, accept a known regression).
- Post-merge CI fails and rollback is required.
- The user wants to override the combined-branch decision.

---

*This definition is executable. Any agent resuming this session should read this file, verify the determined state, and execute the next operator.*
