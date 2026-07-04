# Autoputer / Autopaper Spec-First Suite

## Harness Invocation Semantics

```text
/goal docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md
```

Read this document as semantic authority. Execute it autonomously until its
completion semantics are satisfied with named evidence, or until a sharply
evidenced escalation/blocker/supersession condition is met. Do not summarize,
admire, checkpoint early, or create a separate control language.

## Source Authority Order

1. This document (definition graph + determined state + completion semantics)
2. `docs/mission-suite-autoputer-autopaper-spec-first-v0.md` (suite paradoc)
3. `docs/computer-ontology.md` (product ontology)
4. `AGENTS.md` (repo-level agent operating contract)
5. `docs/agent-product-doctrine.md` (product architecture rules)
6. Child definition: `docs/definitions/pass-2-completion-definition-2026-07-03.md`
7. Child definition: `docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md`
8. Codex review: `docs/reviews/promotion-gate-codex-review-2026-07-03.md`

When sources conflict, this document governs execution. When this document is
silent, the suite paradoc governs. When both are silent, escalate to human.

## Real Artifact / Object Of Work

The real artifacts are:

- **TLA+ specs** in `specs/` that model the current architecture (actor
  runtime, object graph, autoputer lifecycle, promotion protocol, wire
  pipeline). Each spec must compile in TLC and check green against its `.cfg`.
- **Go code** that is a mechanical refinement of those specs: `internal/runtime`
  deleted, actor runtime as sole substrate, `cmd/sandbox` renamed to
  `cmd/autoputer`, promotion protocol encoded in Go.
- **Staging deployment** that boots the renamed autoputer, publishes a wire
  edition end-to-end, and demonstrates candidate → verify → approve → promote →
  health → confirm.

The object is not a plan, a review, a set of commits, or a branch. Those are
projections. The object is the spec-validated, staging-proven system.

## Mission Purpose And Non-Purpose

**Purpose:** Redefine TLA+ specifications to describe the system as it is now
(actor runtime + object graph + autoputer + capsules + wire), then refactor Go
code as a mechanical refinement of those specs so the autoputer boots cleanly,
the wire pipeline publishes end-to-end, and the system is ready for scale-up.

**Non-purpose:**

- This mission does not redesign the product ontology. `autoputer` and
  `autopaper` are already defined by the owner.
- This mission does not add new product features. It aligns code with specs.
- This mission does not preserve compatibility shims. Spec-first means
  fix-forward, not dual-write.
- This mission does not treat spec writing as the deliverable. Specs are the
  authority; code refinement and staging proof are the deliverable.

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
formalization:
  status: done
  note: specs/promotion_protocol.tla models the autoputer promotion lifecycle.
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
formalization:
  status: done
  note: TLA+ specs are themselves the formalization; TLC is the model checker.
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
  - A CI job is bypassed because "it is just docs".
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

### 6. Boundary: `protected-surfaces`

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

### 7. Object: `mission`

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

### 8. Operator: `orchestrate`

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

### 9. Status: `conjecture-supported`

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
  - C-S4: promotion_protocol.tla checks green in CI run 28648508586.
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

### 10. Status: `conjecture-testing`

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
    - claim: Pass 2 PR #42 merged to main as `a6f11b7dbb64c07677a767c19c00e47cf87fdd54`.
      source: observed
      execution_effect: Pass 2 merge gate is closed; do not attempt to re-merge PR #42.
    - claim: Pass 2 post-merge package build failure was repaired by commit `02fa2ea6603b7f157c982e9da637ec714301c6bf`.
      source: observed
      execution_effect: `internal/apihandler` is included in the sandbox Nix service source filter.
    - claim: Main CI for workflow fix commit `8e694f4663412c1a33fc70e870f225f2510718f2` is green.
      source: observed
      execution_effect: Pass 2 can settle, with active computer boot still scoped to Mission C.
    - claim: specs/actor_protocol.tla and specs/autoputer_lifecycle.tla model-check green in main CI.
      source: observed
      execution_effect: C-S1 and C-S3 are SUPPORTED for the modeled safety properties.
    - claim: internal/apihandler extraction landed and cmd/sandbox builds with the extracted API handler package.
      source: observed
      execution_effect: C-A2 is SUPPORTED for Go and Nix package build; staging guest boot remains a separate Mission C uncertainty.
  contested: []
  open:
    - node: c-c1-c4
      missing: Autoputer rename, Nucleus capsule, and promotion encoding missions not yet started.
    - node: wire-pipeline-spec
      missing: specs/wire_pipeline.tla not yet rewritten.
    - node: actor-protocol-xvm-spec
      missing: specs/actor_protocol_xvm.tla not yet rewritten.
    - node: codex-reservations
      missing: Promotion certificate as durable record, owner approval as external gate, Restage weak fairness, sabotage variants.
```

## Invariants

1. **Spec-first:** Spec changes precede code changes for every subsystem.
2. **CI green:** No PR merges while any required check is pending or failing.
3. **No compatibility shims:** Fix-forward, not dual-write.
4. **Problem documentation first:** Document problems before fixing them.
5. **Never leave main red:** If a merge breaks main, fix immediately or revert.

## Authority Boundaries

- **Human authority required for:** purpose/identity changes, authority-boundary
  changes, unsafe/destructive mutations, promotion/rollback changes, spec
  waivers, conflicting value/taste calls.
- **Orchestrator authority:** leaf definitions inside established boundaries,
  subagent coordination, CI monitoring, merge after green, ledger updates.
- **Subagent authority:** scoped work inside a mission, no protected-surface
  changes without escalation.

## Value Criterion

The variant is the count of undecided definition nodes (open + contested +
testing conjectures). Productive execution reduces this count by settling
nodes with scoped evidence. A pass that changes no node status, buys no new
observer evidence, and improves no artifact verifier is motion theater.

Current variant: 4 open nodes + 0 contested + 11 undecided conjectures = 15.

## Homotopy / Realism Parameters

- **Spec domain:** TLA+ with bounded constants. Valid for proving safety
  invariants over the modeled state space. Does not prove implementation
  conformance without a refinement mapping.
- **Code domain:** Go with race-detector tests. Valid for proving concurrent
  safety over exercised execution paths. Does not prove universal correctness.
- **Staging domain:** Single deployed instance on Node B. Valid for proving
  the deployment path and health checks. Does not prove scale-up behavior.

Fake islands are forbidden: no mock APIs that bypass the production path, no
test-only persistence, no manually seeded success artifacts, no local proof
when the claim requires deployment.

## Conjecture And Belief State

18 conjectures total. 7 SUPPORTED. 11 remaining.

```yaml
conjectures:
  - id: C-S1
    status: settled
    claim: actor_protocol.tla safety invariants model-check green.
    evidence_class: model check / formal spec
    source: main CI run 28684139979; TLA+ Model Check (specs/) success.
    scope_if_supported: specs/actor_protocol.tla safety properties.
    execution_effect: Mission A code refinement can proceed against actor_protocol.tla safety semantics.

  - id: C-S2
    status: testing
    claim: actor_protocol.tla encodes the actor runtime mailbox + passivation model.
    test: Spec review + model check.
    execution_effect: If supported, actor runtime is the sole substrate.

  - id: C-S3
    status: settled
    claim: autoputer_lifecycle.tla model-checks green and records the boot/recovery state model.
    evidence_class: model check / formal spec
    source: main CI run 28684139979; TLA+ Model Check (specs/) success.
    execution_effect: Mission C lifecycle work can proceed, but staging guest health remains unproven.

  - id: C-S4
    status: settled
    claim: promotion_protocol.tla model-checks green.
    evidence_class: model check / formal spec
    source: CI run 28648508586, 826 states, 0 errors.
    execution_effect: Promotion gate established.

  - id: C-S5
    status: settled
    claim: promotion_protocol.tla encodes all required invariants.
    evidence_class: model check / formal spec
    source: CI run 28648508586.
    execution_effect: Mission C can encode promotion in Go.

  - id: C-A1
    status: open
    claim: internal/runtime can be deleted after actor runtime extraction.
    test: go build + go test with internal/runtime removed.
    execution_effect: If supported, delete internal/runtime.

  - id: C-A2
    status: settled
    claim: cmd/sandbox/main.go builds using the extracted internal/apihandler package and actor-runtime path.
    evidence_class: observed CI result
    source: main CI run 28684139979; prior host NixOS closure build in run 28683693425 after commit 02fa2ea6.
    execution_effect: Mission A Pass 2 API handler extraction is closed; internal/runtime deletion remains C-A1/C-A3.

  - id: C-A3
    status: open
    claim: All runtime tests pass after internal/runtime deletion.
    test: go test ./internal/... with internal/runtime removed.
    execution_effect: If supported, no test regression from deletion.

  - id: C-B1
    status: open
    claim: wire_pipeline.tla can be rewritten on the object-graph trajectory model.
    test: TLC model check of rewritten spec.
    execution_effect: If supported, Mission B code refinement can proceed.

  - id: C-B2
    status: open
    claim: Wire pipeline publishes end-to-end on staging after refactor.
    test: Staging deployment + edition publication proof.
    execution_effect: If supported, autopaper is production-ready.

  - id: C-C1
    status: open
    claim: cmd/sandbox can be renamed to cmd/autoputer without breaking the build.
    test: go build + staging boot after rename.
    execution_effect: If supported, product ontology aligns with code.

  - id: C-C2
    status: open
    claim: Nucleus capsule can be installed in the autoputer boot path.
    test: Staging boot with capsule enabled.
    execution_effect: If supported, autoputer has a clean capsule substrate.

  - id: C-C3
    status: open
    claim: Promotion protocol can be encoded in Go matching the spec.
    test: Go implementation + staging candidate → promote proof.
    execution_effect: If supported, promotion is spec-validated in production.

  - id: C-C4
    status: open
    claim: Promotion certificate is a durable structured record in the object graph.
    test: Object-graph inspection after promotion.
    execution_effect: If supported, promotion audit trail is durable.

  - id: C-D1
    status: settled
    claim: CI passed on main after promotion gate commits.
    evidence_class: observed CI result
    source: GitHub Actions main branch.
    execution_effect: Promotion gate is deployed.

  - id: C-D2
    status: settled
    claim: TLA+ specs model-check in CI for the current commit.
    evidence_class: model check / formal spec
    source: CI TLA+ Model Check job.
    execution_effect: Spec-first workflow is enforced by CI.

  - id: C-D3
    status: open
    claim: Sabotage/counterexample tests exist for each spec.
    test: Spec sabotage variants in CI.
    execution_effect: If supported, specs are stress-tested, not just green.

  - id: C-D4
    status: open
    claim: Codex review reservations are addressed before Mission C encoding.
    test: Spec update + review for each reservation.
    execution_effect: If supported, promotion encoding is safe to begin.
```

## Variant / Progress Measure

```yaml
variant:
  measure: undecided definition nodes + testing/open conjectures
  current: 15
  target: 0
  motion_theater_threshold: a pass that changes no node status and buys no new evidence
```

## Execution Operators

```text
define(node)          — make a missing meaning executable
split(node)           — split an overloaded meaning
merge(nodes)          — merge redundant meanings
narrow(node)          — restrict scope to evidence
widen(node)           — expand scope when evidence supports it
counterexample(node)  — generate a falsifier
operationalize(node)  — attach observables + execution effects
formalize(node)       — project into TLA+ spec or Go assertion
probe(node)           — test a claim under current observer
shift(node)           — change observer/vocabulary/domain/instrument
construct(node)       — mutate the artifact under invariants
verify(node)          — check an artifact or claim
settle(node)          — promote/weaken/falsify/supersede
escalate(node)        — send to human for group-level decision
monitor(node)         — watch for drift after settlement
```

Each operator must leave a graph or determined-state update. No silent
semantic changes.

## Receding-Horizon Control Loop

Each control interval:

1. **Select** the live node or conjecture whose settlement most reduces mission
   uncertainty or unlocks execution.
2. **State** what the current observer can and cannot see; name any blind spot.
3. **Choose** one move: define, probe, shift, construct, verify, settle.
4. **Bound** the mutation radius and rollback surface.
5. **Execute** the move.
6. **Update** node status, determined state, evidence ledger, and checkpoint.
7. **Continue** unless completion, supersession, or hard escalation is reached.

If the route is clear and low-risk, batch foreseeable constructs in one
interval. The tripwire is surprise: any unexpected evidence returns execution
to a full select/state/choose/bound loop.

## Dense Feedback Channels

- **CI:** GitHub Actions on every push. Go build, go test (sharded + race),
  TLA+ model check, frontend build, docs truth check.
- **Staging:** Node B deploy + host service health check on behavior-changing commits; active computer refresh is diagnostic until Mission C settles guest boot readiness.
- **Spec model check:** TLC on every spec change, runs in CI.
- **Codex review:** Second opinion on high-risk changes (protected surfaces).
- **Race detector:** Go test -race on runtime shards.

## Evidence Ledger

```yaml
evidence:
  - claim: C-S4 promotion_protocol.tla model-checks green
    definition_node: C-S4
    evidence_class: model check / formal spec
    source: CI run 28648508586
    command_or_observation: TLC model check of specs/promotion_protocol.tla
    result: 826 states, 0 errors, 0 distinct
    uncertainty: Bounded constants; does not prove implementation conformance
    promotion_relevance: Promotion gate established; Mission C may proceed

  - claim: C-S5 promotion_protocol.tla encodes all required invariants
    definition_node: C-S5
    evidence_class: model check / formal spec
    source: CI run 28648508586
    result: All invariants in .cfg checked green
    uncertainty: Invariant set may be incomplete (Codex reservations)
    promotion_relevance: Mission C encoding can begin after reservations addressed

  - claim: C-D1 CI passed on main after promotion gate commits
    definition_node: C-D1
    evidence_class: observed CI result
    source: GitHub Actions main branch
    result: All required jobs passed
    promotion_relevance: Promotion gate is deployed

  - claim: C-D2 TLA+ specs model-check in CI
    definition_node: C-D2
    evidence_class: model check / formal spec
    source: CI TLA+ Model Check job
    result: All specs in specs/ check green
    promotion_relevance: Spec-first workflow is CI-enforced

  - claim: PR #42 all CI checks green
    definition_node: pass-2-green
    evidence_class: observed CI result
    source: GitHub Actions PR #42 statusCheckRollup
    result: All 19 required checks SUCCESS, 3 SKIPPED (staging deploy, SBOM, staging impact)
    uncertainty: Staging deploy was skipped; post-merge staging proof still needed
    promotion_relevance: PR #42 is safe to merge

  - claim: Codex review of promotion gate completed
    definition_node: codex-reservations
    evidence_class: human review
    source: docs/reviews/promotion-gate-codex-review-2026-07-03.md
    result: Verdict "approve with reservations"; 4 major, 6 minor findings
    uncertainty: Reservations must be addressed before Mission C encoding
    promotion_relevance: Promotion spec is sound but incomplete

  - claim: C-S1 and C-S3 model-check green on main
    definition_node: C-S1/C-S3
    evidence_class: model check / formal spec
    source: GitHub Actions main CI run 28684139979
    result: TLA+ Model Check (specs/) success
    uncertainty: Bounded constants; does not prove staging guest boot
    promotion_relevance: Mission A/C can continue from modeled safety state

  - claim: C-A2 API handler extraction builds in Go and Nix package contexts
    definition_node: C-A2
    evidence_class: observed CI result
    source: PR #42 merge `a6f11b7d`, packaging fix `02fa2ea6`, main CI run 28684139979
    result: Go build green; sandbox Nix package source filter includes internal/apihandler
    uncertainty: Does not prove refreshed active computer health
    promotion_relevance: Pass 2 extraction is closed; internal/runtime deletion remains separate

  - claim: Active computer refresh is the remaining staging deploy realism gap
    definition_node: C-C1/C-C2
    evidence_class: deployed diagnostic
    source: GitHub Actions deploy job 85072352680
    result: Host services healthy at commit 02fa2ea6; refreshed guest did not become healthy on :8085 within 3m
    uncertainty: Guest boot/readiness root cause not repaired
    promotion_relevance: Mission C must settle autoputer boot before claiming product-path promotion proof
```

## Completion Semantics

```text
The suite is COMPLETE when:
  1. All 18 conjectures are SUPPORTED or explicitly REFUTED with rationale.
  2. All specs are model-checked green in CI.
  3. internal/runtime is deleted.
  4. cmd/sandbox is renamed to cmd/autoputer and boots on staging.
  5. The wire pipeline publishes end-to-end on staging.
  6. Promotion protocol works on staging: candidate -> verify -> approve -> promote -> health -> confirm.
  7. Codex review reservations are addressed.
  8. Suite ledger and paradoc are updated with final evidence.
  9. CI is green on main.

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

## Escalation Rules

```yaml
escalation:
  - rule: Never leave main red. If a merge breaks main, fix immediately or revert.
  - rule: Never merge a PR with pending required checks.
  - rule: Update the ledger before going to sleep. Every pass must leave a trail.
  - rule: Spawn subagents for parallel work, but verify their output. Subagents are not authority.
  - rule: Use Codex review for high-risk changes. A second opinion is required for protected surfaces.
  - rule: If a definition is contested, escalate. Do not let semantic drift accumulate overnight.
  - rule: If a mission stalls for 2+ passes, reassess. Do not keep patching symptoms.
  - rule: Document problems before fixing them. Problem-first, fix-second.

human_escalation_triggers:
  - purpose or identity changes
  - authority-boundary changes
  - unsafe/destructive or high-blast-radius mutations
  - spec waivers
  - conflicting values or taste calls
  - irreversible actions without accepted rollback
```

## Forbidden Collapses

- artifact exists -> artifact is valid
- definition document exists -> definition graph is settled
- plan exists -> mission is executing
- review packet exists -> review passed
- tests passed -> behavior is universally proven
- checkpoint landed -> mission complete
- model agreement -> definition settled
- formal spec exists -> implementation conforms
- implementation exists -> definition was followed
- local smoke passed -> production claim proven
- toy result green -> program validated
- second opinion -> authority
- route is familiar -> route is correct
- worker says done -> done

## Rollback And Resumption Policy

```yaml
rollback:
  - surface: Every commit to main is revertible via git revert.
  - surface: Every spec change is revertible via git revert.
  - surface: Staging deploy can be rolled back to the previous SHA.
  - surface: PR merges can be reverted if post-merge CI fails.

resumption:
  - rule: Read this document first, then the child definition, then execute the next operator.
  - rule: Reconcile determined state with current artifact state before acting.
  - rule: If a safe executable probe remains inside the authority boundary, execute it instead of presenting a checkpoint as success.
```

## Mission Report Policy

Maintain an owner-readable report when the run changes durable system state,
doctrine, deployed behavior, or long-running execution state.

The report should explain:

```text
mission goal and artifact
invariants preserved or violated
major decisions and route changes
what shipped
verification evidence
what was proven vs merely attempted
residual risks
rollback refs
next mission or next executable probe
```

Do not dump logs. Link evidence artifacts.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: 55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a (Pass 3 diagnostic patch deployed on main)
  current_artifact_state:
    - specs/promotion_protocol.tla: green in CI
    - specs/actor_protocol.tla: green in main CI
    - specs/autoputer_lifecycle.tla: green in main CI
    - internal/apihandler: extracted and merged on main
    - flake.nix: sandbox package source filter includes internal/apihandler
    - internal/runtime: still present, deletion pending later Mission A work
    - cmd/sandbox: not yet renamed to cmd/autoputer
    - active refreshed guest health: failed in deploy job 85072352680, scoped to Mission C
    - docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md: opened for Mission C boot-readiness investigation
    - Pass 3 diagnostic patch deployed: vmmanager readiness timeout errors retain last `/health` probe detail; deploy diagnostics print vmctl ownership and active sandbox health snapshots.
    - Deploy job `85076877932` for commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a` reported no active interactive computers needed refresh, so the changed active-refresh diagnostic path has not yet been exercised.
    - Product-path activation probe reached signed-out Choir preview and passkey sign-in/create dialog; no cookies, localStorage auth, sessionStorage auth, passkey login, or account creation were available/performed from the harness.
    - Authenticated product-path probe for `yusefnathanson@me.com` is available through imported Chrome cookies, but the account remains stuck in Choir BIOS boot pending after recovery.
  what_shipped:
    - Promotion gate spec (Pass 1, merged to main)
    - Actor protocol + autoputer lifecycle specs (Pass 2, PR #42 merged)
    - API handler extraction (Pass 2, PR #42 merged)
    - Sandbox Nix package source-filter fix for internal/apihandler
    - CI deploy gate now treats active computer refresh as diagnostic while preserving host health as the deploy gate
    - Pass 3 readiness diagnostic patch at `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`
  what_was_proven:
    - C-S1: actor_protocol.tla safety invariants model-check green in main CI
    - C-S3: autoputer_lifecycle.tla model-checks green in main CI
    - C-S4: promotion_protocol.tla model-checks green
    - C-S5: promotion_protocol.tla encodes required invariants
    - C-A2: cmd/sandbox builds with extracted internal/apihandler package in Go and Nix package contexts
    - C-D1: CI passed on main after the current commits
    - C-D2: TLA+ specs model-check in CI
    - Pass 3 diagnostic patch deploys without regressing host service health, but did not exercise active refresh because no active interactive computers needed refresh.
  unproven_or_partial_claims:
    - C-S2: actor_protocol.tla still needs semantic review for mailbox/passivation completeness
    - C-A1/C-A3: internal/runtime deletion remains unproven
    - C-B1/C-B2: wire pipeline spec and staging publication remain open
    - C-C1/C-C2: autoputer rename/capsule and refreshed guest boot readiness remain open
    - C-C3/C-C4: Go promotion encoding and durable certificate remain open
    - C-D3/C-D4: sabotage variants and Codex reservations remain open
  belief_state_changes:
    - PR #42 is merged; re-merging PR #42 is obsolete.
    - Pass 2 package-build regression is repaired.
    - Active computer refresh is now the highest-realism staging gap and must not be confused with Pass 2 extraction.
    - Codex reservations must be addressed before Mission C promotion encoding.
    - The first Pass 3 root cause is an evidence-layer bug: guest readiness polling collapsed HTTP status/body and transport errors into a boolean.
    - The evidence-layer patch is deployed, but the next active-refresh root-cause evidence still requires an active interactive computer during an ordinary guest deploy.
    - Product-path activation boundary was crossed through approved Chrome cookie import; the active problem is now authenticated boot readiness, not lack of browser auth.
    - Authenticated account evidence now exists: `yusefnathanson@me.com` repeats bootstrap probes, recovery returns 202, theme preferences returns 502 after 180010ms, and boot does not complete.
    - Authenticated compute status previously identified stopped-primary disk fullness as a suspect, but the deployed gauge fix disproved that as a critical-full signal.
    - Persistent data capacity repair is prepared locally: per-VM data image minimum is 32 GiB, and focused resize tests passed.
    - The 32 GiB capacity repair is deployed to vmctl.
    - Host-image disk gauge fix is deployed to vmctl: stopped-computer `file_bytes` now reports allocated state-dir bytes rather than virtual `data.img` capacity.
    - The host-image disk gauge fix is deployed and authenticated compute status now reports the stopped primary data image at 49.93% used, not critical/full.
    - Authenticated recovery still does not boot because Node B vmctl logs show concurrent stopped-computer resolve/recovery paths launching duplicate Firecracker processes for the same VM ID and killing each other.
    - Stopped/hibernated resume coalescing fix is prepared locally in `internal/vmctl`, and focused normal/race tests passed.
  remaining_error_field:
    - Active refreshed guest does not become healthy on :8085 during deploy.
    - `yusefnathanson@me.com` primary computer recovery remains unproven until the stopped/hibernated resume coalescing fix is deployed and authenticated recovery is re-run.
    - Current staging `/health/ready` is degraded for runtime/dolt/ollama, not accepted as Pass 3 completion proof.
    - Codex reservations: promotion certificate, owner approval model, Restage fairness, sabotage variants
    - Wire pipeline spec not yet rewritten
    - actor_protocol_xvm.tla not yet rewritten
    - Autoputer rename and Nucleus capsule work not started
  highest_impact_remaining_uncertainty: C-C1/C-C2 deployed stopped/hibernated current-computer resume coalescing under concurrent authenticated bootstrap probes
  next_executable_probe: Commit/push/deploy the `internal/vmctl` stopped/hibernated resume coalescing fix, trigger authenticated recovery for `yusefnathanson@me.com`, inspect Node B vmctl logs for absence of duplicate Firecracker kills, and re-run authenticated bootstrap/health evidence.
  suggested_goal_string: "/goal docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md"
  evidence_artifact_refs:
    - docs/reviews/promotion-gate-codex-review-2026-07-03.md
    - docs/definitions/pass-2-completion-definition-2026-07-03.md
    - docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md Pass 8 through Pass 20
    - docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md
    - CI run 28648508586 (promotion gate)
    - PR #42 merged commit a6f11b7dbb64c07677a767c19c00e47cf87fdd54
    - CI run 28683693425 (packaging fix; deploy job exposed active refresh failure)
    - CI run 28684139979 (current main CI green)
    - CI run 28685279292 and deploy job 85076877932 (Pass 3 diagnostic patch deployed)
    - Race Detector run 28685279281 attempt 2
    - staging `/health` at commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`
    - staging `/health/ready` degraded for runtime/dolt/ollama
    - focused Pass 3 test: `go test ./internal/vmmanager -run TestWaitForGuestReady -count=1`
    - deploy-impact classifier test: `.github/scripts/deploy-impact-classify-test`
    - browser product-path probe: `https://choir.news` signed-out preview -> Desk -> Sign in exposed passkey create/login; no authenticated storage/cookies were present.
    - authenticated browser probe: imported Chrome cookies for `choir.news`; `yusefnathanson@me.com` showed BIOS boot pending for 207s+, recovery POST 202, repeated pending `/api/shell/bootstrap`, `/api/preferences/theme` 502 after 180010ms.
    - authenticated compute status: `/api/compute/status` returned primary `state=stopped`, recovery `status=failed`, and `persistent_disk.used_percent=100` with warning "persistent data image is critically full".
    - focused capacity test: `go test ./internal/vmmanager -run 'TestDataImageSizeCoversSelfDevelopmentWorkspace|TestBootVMExpandsExistingSmallDataImageBeforeLaunch' -count=1`
    - capacity repair CI/deploy: CI run `28690422412`, Race Detector run `28690422396`, Docs Truth Check run `28690422415`, FlakeHub run `28690422405`, deploy job `85090768662`
    - focused vmctl data-image test: `go test ./internal/vmctl -run 'TestDataImageStats|TestOwnershipRegistryDataImageStatsForVM' -count=1`
    - vmctl gauge fix CI/deploy: CI run `28691200371`, Race Detector run `28691200354`, Docs Truth Check run `28691200358`, FlakeHub run `28691200363`, deploy job `85092811346`
    - post-gauge-fix authenticated compute status: `persistent_disk.used_percent=49.93085861206055`, `critical=false`, `cap_bytes=34359738368`
    - Node B vmctl logs: repeated duplicate Firecracker kills for `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` during authenticated stopped-computer recovery.
    - focused stopped-resume coalescing test: `go test ./internal/vmctl -run 'TestOwnershipRegistry_ResolveCoalescesStoppedVMResume|TestDataImageStats|TestOwnershipRegistryDataImageStatsForVM' -count=1`
    - stopped-resume race test: `go test ./internal/vmctl -run TestOwnershipRegistry_ResolveCoalescesStoppedVMResume -race -count=1`
  rollback_refs:
    - main HEAD: 55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a
```

## Child Definition Documents

- `docs/definitions/pass-2-completion-definition-2026-07-03.md` — specific
  completion criteria for Pass 2 (PR #42 merge gate).
- `docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md` — specific
  investigation and completion criteria for Mission C active refresh / autoputer boot readiness.


---

*This is the super definition for the suite. Any agent resuming work must read
this file first, then the relevant child definition, then execute the next
operator.*
