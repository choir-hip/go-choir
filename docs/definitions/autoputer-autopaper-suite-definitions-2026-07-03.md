# Super Definition: Autoputer / Autopaper Spec-First Suite

**Status:** live semantic authority  
**Date:** 2026-07-03  
**Governed by:** `definitions` skill  
**Scope:** `docs/mission-suite-autoputer-autopaper-spec-first-v0.md` and all child missions (S, A, B, C, D)  
**Overnight production:** this document is the root authority for autonomous agents continuing the suite while the human is away.

---

## Executive Determined State

### Settled claims

- **Mission thesis:** TLA+ is the spec. Code is a refinement of the compiled, model-checked spec.
- **Predecessor settled:** `docs/mission-autoputer-before-autopaper-v0.md` was the direct predecessor; it is now superseded by this suite.
- **Promotion gate established:** `specs/promotion_protocol.tla` checks green in CI (run `28648508586`).
- **Codex review completed:** `docs/reviews/promotion-gate-codex-review-2026-07-03.md` — verdict: approve with reservations.
- **Pass 2 in flight:** branch `mission-a-api-handler-extraction`, PR #42, contains Mission S (actor + lifecycle specs), Mission A (helper + API extraction), Mission D (review artifact).

### Contested / open nodes

- **C-S1:** `actor_protocol.tla` safety invariants — pending PR #42 CI.
- **C-S3:** `autoputer_lifecycle.tla` reproduces boot failure — pending PR #42 CI.
- **C-A2:** `cmd/sandbox/main.go` builds using only actor runtime + extracted helpers — pending PR #42 CI.
- **C-C1/C-C2/C-C3/C-C4:** Autoputer rename/promotion/capsule — not yet started.

---

## Definition Graph

### 1. Term: `autoputer`

```yaml
id: autoputer
kind: term
status: settled
source: user-stated
term: autoputer
definition: A persistent, owner-identified computer that runs user agents, stores durable state in the object graph, can fork candidate computers, and promotes a candidate to active after a verified, approved, healthy promotion protocol.
non_definition:
  - "sandbox" is the old service name, not the product ontology.
  - "VM" is the hosting substrate, not the computer itself.
  - "candidate" is a possible future autoputer, not the active autoputer.
examples:
  - A running Choir deployment serves one active autoputer per owner.
  - A candidate autoputer is forked from the active one, modified, and promoted.
  - An autoputer survives process restart because its state is in the object graph.
counterexamples:
  - A throwaway container with no durable state.
  - A single goroutine that dies when the process restarts.
  - The `sandbox` binary name (implementation detail).
observables:
  - Durable object-graph records exist for the owner.
  - A candidate can be forked and promoted.
  - Health checks pass on port 8085.
execution_effect:
  - All new code uses the term `autoputer` in package/binary names where appropriate.
  - `sandbox` is treated as a legacy term and renamed incrementally.
settlement:
  rule: Defined by owner in docs/computer-ontology.md and suite paradoc.
  settled_by: human
  invalidation_triggers:
    - Owner redefines the product ontology.
```

### 2. Term: `autopaper`

```yaml
id: autopaper
kind: term
status: settled
source: user-stated
term: autopaper
definition: The automatic newspaper produced by the autoputer: a wire pipeline that fetches sources, processes items through agents, drafts Texture documents, and publishes editions.
non_definition:
  - Autopaper is not a separate service; it is the publication output of the autoputer.
  - The wire pipeline is the mechanism, not the product.
examples:
  - A daily edition published by the autoputer.
  - A wire trajectory that produces a Texture article.
counterexamples:
  - A manual CMS article.
  - A social-media cross-post.
observables:
  - A wire pipeline run produces a publishable edition.
  - Editions are stored in the object graph.
execution_effect:
  - Mission B redesigns the wire pipeline on the object graph.
  - Mission C does not build a separate autopaper service.
settlement:
  rule: Defined by owner in suite paradoc.
  settled_by: human
```

### 3. Object: `spec`

```yaml
id: spec
kind: object
status: settled
source: user-stated
term: TLA+ spec
definition: A formal, model-checked module in specs/ that is the source of truth for a subsystem. Every spec must compile in TLC and check green against its .cfg before the corresponding code is refactored.
non_definition:
  - A spec is not documentation.
  - A spec is not a test.
  - A spec is not a suggestion.
examples:
  - specs/promotion_protocol.tla
  - specs/actor_protocol.tla
  - specs/autoputer_lifecycle.tla
counterexamples:
  - An English description of the protocol.
  - A Go test that happens to pass.
observables:
  - File exists in specs/.
  - Corresponding .cfg exists.
  - CI TLA+ Model Check job passes.
execution_effect:
  - Code changes must match the spec.
  - Specs are deleted only if replaced by a more precise spec.
settlement:
  rule: Spec-first workflow in suite paradoc.
  settled_by: human
  invalidation_triggers:
    - Owner approves a deviation from spec-first.
```

### 4. Boundary: `spec-vs-code`

```yaml
id: spec-vs-code
kind: boundary
status: settled
source: user-stated
term: spec-first boundary
definition: The spec is written before the code. The code is a refinement of the spec. If the code violates the spec, either fix the code or explicitly weaken the spec with documented rationale.
non_definition:
  - The spec is not adjusted to make the current code pass.
  - The code is not merged while violating the spec.
examples:
  - A spec invariant is proven before the Go code enforces it.
  - A counterexample in TLC drives a code fix.
  - A spec weakening is approved in a PR with a rationale section.
counterexamples:
  - A Go hack is merged because "the spec is too hard".
  - A spec is silently changed to match a bug.
observables:
  - PRs reference spec changes before code changes.
  - CI runs TLC before Go tests.
  - Spec README explains the layering.
execution_effect:
  - Mission S work precedes Mission A/B/C work for each subsystem.
  - Subagents must not change code to match an unwritten spec.
settlement:
  rule: Suite invariants section.
  settled_by: human
  invalidation_triggers:
    - Emergency patch requiring spec waivers.
```

### 5. Invariant: `ci-green`

```yaml
id: ci-green
kind: invariant
status: settled
source: user-stated
term: CI green
definition: Every commit that changes tracked files must pass the GitHub Actions CI workflow. For behavior-changing commits, this includes go build, go test (including race detector), TLA+ model check, frontend build, and docs truth check. Staging deploy and health check are required for behavior-changing commits.
non_definition:
  - "Go tests passed" alone is not CI green.
  - "TLC passed" alone is not CI green.
  - "It worked on my machine" is not CI green.
examples:
  - PR #42 shows all required jobs as pass.
  - Post-merge main CI run shows success.
  - Staging deploy reports the correct SHA and passes health checks.
counterexamples:
  - A commit is merged while a CI job is pending.
  - A CI job is bypassed because "it is just docs" (docs-only commits follow the docs truth checker workflow).
  - A race-detector failure is ignored.
observables:
  - `gh pr checks <n>` returns pass.
  - `gh run view <id>` shows all jobs complete.
  - Staging health endpoint reports the deployed SHA.
execution_effect:
  - No PR is merged until CI is fully green.
  - Mission D runs continuously as guard.
settlement:
  rule: Landing loop in AGENTS.md.
  settled_by: human
  invalidation_triggers:
    - Emergency break-glass procedure.
```

### 6. Status: `conjecture-supported`

```yaml
id: conjecture-supported
kind: status
status: settled
source: observed
term: conjecture supported
definition: A conjecture (e.g., C-S4) is SUPPORTED when there is observed evidence that satisfies its acceptance criteria and no live contradictions.
non_definition:
  - "I believe it is true" is not SUPPORTED.
  - "It passed once" is not SUPPORTED if the CI has since changed.
  - "A subagent said so" is not SUPPORTED unless the subagent's evidence is verifiable.
examples:
  - C-S4: `promotion_protocol.tla` checks green in CI run 28648508586.
  - C-D2: TLA+ specs model-check in CI for the current commit.
counterexamples:
  - A conjecture is marked SUPPORTED based on a local test run.
  - A conjecture is marked SUPPORTED while a related CI job is red.
observables:
  - CI run ID and job result.
  - Spec file and .cfg.
  - Ledger entry with evidence.
execution_effect:
  - SUPPORTED conjectures reduce the suite variant count.
  - New work can build on SUPPORTED conjectures.
settlement:
  rule: Evidence must be observed and verifiable.
  settled_by: orchestrator
  invalidation_triggers:
    - New CI failure contradicts the claim.
    - Spec is weakened after the claim.
```

### 7. Status: `conjecture-testing`

```yaml
id: conjecture-testing
kind: status
status: settled
source: observed
term: conjecture testing
definition: A conjecture is TESTING when the work to decide it is in progress but has not yet produced the required evidence.
non_definition:
  - TESTING is not a placeholder for "probably true".
  - TESTING must have an observable test in progress.
examples:
  - C-S1 while PR #42 CI is running.
  - C-A2 while helper/API extraction is being compiled.
counterexamples:
  - A conjecture is left TESTING with no active work.
  - A conjecture is marked TESTING to avoid deciding it.
observables:
  - Active PR/branch with related changes.
  - CI job running.
  - Subagent report with evidence.
execution_effect:
  - TESTING conjectures consume the suite budget.
  - They must be decided (SUPPORTED or REFUTED) within the pass budget.
settlement:
  rule: Must have an active, observable test.
  settled_by: orchestrator
  invalidation_triggers:
    - Test stalls without evidence.
    - Test is abandoned.
```

### 8. Object: `mission`

```yaml
id: mission
kind: object
status: settled
source: user-stated
term: mission
definition: A scoped work unit with a clear spec or code target, a set of conjectures, and a deliverable. Missions S, A, B, C, D are the five missions in the suite.
non_definition:
  - A mission is not a task list.
  - A mission is not a vague goal.
examples:
  - Mission S: rewrite TLA+ specs.
  - Mission A: delete old internal/runtime.
  - Mission C: rename sandbox to autoputer and encode promotion.
counterexamples:
  - "Fix bugs".
  - "Improve performance".
observables:
  - Mission doc in docs/.
  - Conjecture list.
  - Deliverable files.
execution_effect:
  - Work is grouped by mission.
  - Subagents are assigned per mission.
settlement:
  rule: Suite paradoc.
  settled_by: human
```

### 9. Boundary: `protected-surfaces`

```yaml
id: protected-surfaces
kind: boundary
status: settled
source: user-stated
term: protected surfaces
definition: Subsystems that must not be changed without a spec update and explicit human authority. Includes Texture canonical writes, corpusd sync contract, source entity graph, promotion/rollback, VM lifecycle, auth/session, gateway/provider calls.
non_definition:
  - Protected surfaces are not "hard to change"; they are authority-bound.
  - A protected surface cannot be bypassed by a subagent.
examples:
  - Promotion logic is updated only after promotion_protocol.tla is green.
  - Texture canonical writes require a spec change.
counterexamples:
  - A subagent patches a promotion bug without updating the spec.
  - A gateway provider call is rerouted without approval.
observables:
  - PR touches protected files.
  - Spec update precedes code change.
  - Human review is requested.
execution_effect:
  - Red/black ceremony required for protected surfaces.
  - Subagents must escalate before touching these.
settlement:
  rule: AGENTS.md mutation classes + suite paradoc.
  settled_by: human
```

### 10. Operator: `orchestrate`

```yaml
id: orchestrate
kind: operator
status: settled
source: user-stated
term: orchestrate
definition: The orchestrator coordinates subagents, verifies CI, merges approved work, and updates the ledger and paradoc. The orchestrator does not do all the work itself.
non_definition:
  - Orchestrator is not a bottleneck.
  - Orchestrator is not a passive dispatcher.
examples:
  - Spawn Mission S subagent to draft a spec.
  - Spawn Codex review subagent.
  - Merge PR #42 after CI green.
  - Update suite ledger with conjecture status.
counterexamples:
  - Orchestrator writes all the code.
  - Orchestrator ignores a subagent's blocker.
observables:
  - Subagents are running.
  - PRs are open.
  - Ledger is updated.
execution_effect:
  - Parallel mission execution.
  - Clean handoffs.
settlement:
  rule: Suite orchestration plan.
  settled_by: human
```

### 11. Forbidden Collapse: `artifact-vs-valid`

```yaml
id: artifact-vs-valid
kind: forbidden_collapse
status: settled
source: skill
term: artifact exists -> artifact is valid
definition: Do not treat the existence of a file, PR, or spec as proof that it is correct.
examples:
  - A spec file exists but has not model-checked green.
  - A PR is open but CI is red.
  - A review file exists but no action was taken.
execution_effect:
  - Every artifact must be verified by an observable.
```

### 12. Forbidden Collapse: `tests-pass-vs-proven`

```yaml
id: tests-pass-vs-proven
kind: forbidden_collapse
status: settled
source: skill
term: tests passed -> behavior is universally proven
definition: CI green is strong evidence, not a proof of universal correctness.
examples:
  - A model with bounded constants may not catch all real-world behaviors.
  - A passing test does not cover all failure modes.
execution_effect:
  - Continue to add sabotage variants and stress tests.
```

### 13. Forbidden Collapse: `checkpoint-vs-complete`

```yaml
id: checkpoint-vs-complete
kind: forbidden_collapse
status: settled
source: skill
term: checkpoint landed -> mission complete
definition: A landed checkpoint is progress, not completion.
examples:
  - PR #42 is a checkpoint, not the whole suite.
  - A green spec is a checkpoint; the code refinement is still needed.
execution_effect:
  - Keep the variant count and next steps visible.
```

---

## Completion Semantics

```text
The suite is COMPLETE when:
  1. All 18 conjectures are SUPPORTED or explicitly REFUTED with rationale.
  2. All specs are model-checked green in CI.
  3. internal/runtime is deleted.
  4. cmd/sandbox is renamed to cmd/autoputer and boots on staging.
  5. The wire pipeline publishes end-to-end on staging.
  6. Promotion protocol works on staging: candidate -> verify -> approve -> promote -> health -> confirm.
  7. Suite ledger and paradoc are updated with final evidence.
  8. CI is green on main.

The suite is BLOCKED when:
  1. A conjecture remains TESTING for more than one pass without evidence.
  2. A spec fails to model-check and the fix requires a group-level decision.
  3. A protected surface must be changed without spec authority.
  4. CI is red on main.
  5. A subagent reports a blocker that the orchestrator cannot resolve.

The suite is in PROGRESS when:
  1. At least one conjecture is being tested or one mission is active.
  2. CI is green on the latest relevant commit.
  3. The ledger and paradoc are up to date.
```

---

## Execution Policy for Overnight Production

1. **Never leave main red.** If a merge breaks main, either fix it immediately or revert it.
2. **Never merge a PR with pending required checks.** Wait for CI or abort the merge.
3. **Update the ledger before going to sleep.** Every pass must leave a trail.
4. **Spawn subagents for parallel work, but verify their output.** Subagents are not authority.
5. **Use Codex review for high-risk changes.** A second opinion is required for protected surfaces.
6. **If a definition is contested, escalate.** Do not let semantic drift accumulate overnight.
7. **If a mission stalls for 2+ passes, reassess.** Do not keep patching symptoms.
8. **Document problems before fixing them.** Problem-first, fix-second.

---

## Child Definition Documents

- `docs/definitions/pass-2-completion-definition-2026-07-03.md` — specific completion criteria for Pass 2.

---

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: TLA+ is the spec; code is a refinement.
      source: user-stated
      execution_effect: Spec changes precede code changes.
    - claim: Promotion gate is established (C-S4, C-S5, C-D1, C-D2 SUPPORTED).
      source: observed
      execution_effect: Mission C promotion encoding can begin once Pass 2 is merged.
    - claim: Codex review of promotion gate is approve with reservations.
      source: reviewer
      execution_effect: Address reservations before encoding promotion certificate and approval boundary.
    - claim: Pass 2 branch is `mission-a-api-handler-extraction`, PR #42.
      source: observed
      execution_effect: Merge once CI green; do not start Pass 3 on main before this.
  contested:
    - node: pass-2-green
      issue: PR #42 CI is still running; cannot settle until all jobs pass.
      next_resolution_step: monitor CI run 28651240293.
  open:
    - node: c-c1-c4
      missing: Autoputer rename, Nucleus capsule, and promotion encoding missions not yet started.
    - node: wire-pipeline-spec
      missing: specs/wire_pipeline.tla not yet rewritten.
    - node: actor-protocol-xvm-spec
      missing: specs/actor_protocol_xvm.tla not yet rewritten.
```

---

*This is the super definition for the suite. Any agent resuming work must read this file first, then the relevant child definition, then execute the next operator.*
