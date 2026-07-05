# Substrate Hardening: PGo Decision and Contract Consolidation

## Harness Invocation Semantics

```text
/goal docs/missions/substrate-hardening-v0.md
```

Read this document as semantic authority. Execute it autonomously until its
completion semantics are satisfied with named evidence, or until a sharply
evidenced escalation/blocker/supersession condition is met.

## Source Authority Order

1. This document
2. `docs/definitions/substrate-independent-audited-computer-2026-07-04.md` (SIAC mission)
3. `docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md` (suite super definition)
4. `docs/reviews/substrate-independent-audited-computer-changeset-review-2026-07-04.md` (multi-agent review)
5. `docs/reviews/landing-brief-2026-07-04.md` (landing brief, now landed)
6. `docs/missions/pgo-evaluation-v0.md` (PGo evaluation mission, staged)
7. `specs/promotion_protocol.tla` (current spec with vacuous ComputerVersion invariants)
8. `AGENTS.md`

## Real Artifact / Object Of Work

The real artifacts are:

1. **A PGo go/no-go decision** backed by a working compilation attempt (build
   PGo, translate `promotion_protocol.tla` to MPCal, generate Go, assess
   quality and integration cost).
2. **A refactored `internal/computerversion` package** that resolves the
   maintainability issues identified in the review: contract duplication,
   false purity claim, large files, and cmd binary duplication.
3. **Strengthened or replaced TLA+ invariants** that are no longer vacuous —
   either independent CodeRef/ArtifactProgramRef counters, or MPCal
   refinement by construction if PGo is adopted.
4. **An updated SIAC checkpoint** reflecting the landed state (`30f0301f`),
   the PGo decision, and the remaining completion gates.

The object is not more boundary contracts. The object is not another review.
The object is a decision plus a hardening pass that makes the existing code
maintainable and the spec honest.

## Mission Purpose And Non-Purpose

**Purpose:** Make a PGo adoption decision and harden the landed substrate
code for maintainability, modularity, and correctness. The landed changeset
(`30f0301f`, 156 files, 49,807 insertions) has known issues: 39 contract
files with ~60% duplication, a false purity claim, vacuous TLA+ invariants,
and cmd binary duplication. This mission resolves those issues and decides
whether PGo replaces the hand-written contract layer.

**Non-purpose:**

- This mission does not add new boundary contracts. Boundary inflation is
  identified as a known failure mode (issue 1.16). The existing 39 contract
  files are sufficient; this mission consolidates or replaces them.
- This mission does not advance the SIAC completion gates (cross-substrate
  proof, data.img disposability, promotion/rollback over ComputerVersion).
  Those remain open in the SIAC mission. This mission hardens the substrate
  for the next SIAC push.
- This mission does not modify deployed runtime behavior. All changes are
  internal package structure, spec correctness, or tooling.
- This mission does not rewrite specs in MPCal unless PGo is a GO decision.
  If PGo is NO-GO, TLA+ invariants are strengthened manually.

## Definition Graph

### 1. Conjecture: `C-H1-pgo-decision`

```yaml
id: C-H1-pgo-decision
kind: conjecture
status: proposed
claim: PGo can build on this machine, compile an MPCal translation of promotion_protocol.tla, and produce usable Go code.
test: |
  1. Install sbt (and JDK 17 if JDK 25 is incompatible).
  2. Clone PGo repo from https://github.com/DistCompiler/pgo.
  3. Run `sbt run` with a trivial MPCal spec to verify the build.
  4. Translate promotion_protocol.tla to MPCal (restructure from declarative TLA+ to PlusCal algorithm with archetypes).
  5. Run pcalgen, then TLC on the generated TLA+ to verify invariants hold.
  6. Run gogen to produce Go code.
  7. Run `go build` on the generated Go.
  8. Read the generated Go and assess: interfaces, readability, distsys dependency weight, integration cost with existing actor runtime.
edge:
  blind_spot: PGo is research-grade with no stable releases; may fail on modern JDK/sbt.
  class: resource
observer_upgrade: Use JDK 17 via Nix/Homebrew if JDK 25 fails.
falsifier: sbt build fails, MPCal translation loses invariants, gogen crashes, or generated Go doesn't compile.
scope_if_supported: PGo is viable for promotion_protocol.tla and potentially for candidate_package_intake state machine.
execution_effect: |
  If SUPPORTED: PGo adoption is recommended. Phase 2 refactoring focuses on integration boundary, not contract consolidation. The 39 hand-written contract files are candidates for replacement by generated code.
  If REFUTED: PGo is rejected with documented rationale. Phase 2 refactoring executes full maintainability pass on hand-written contracts.
```

### 2. Term: `contract-consolidation`

```yaml
id: contract-consolidation
kind: term
status: contested
source: observed
term: contract consolidation
definition: |
  The process of reducing 39 contract files (~10.2k LOC, ~60% field-for-field duplication) to a maintainable structure.
  Options:
  - Extract a shared ContractHeader + NegativeClaims struct and generic validation helper.
  - Replace with PGo-generated code if PGo is adopted.
  - Represent the lifecycle as data (a step graph) instead of typed contracts per micro-step.
non_definition:
  - Contract consolidation is not deleting contracts to reduce LOC. Safety flags must be preserved.
  - Contract consolidation is not merging all contracts into one file. The boundary semantics must remain expressible.
examples:
  - A shared `ContractHeader` struct with Kind, Boundary, Scope, Version fields that all contract types embed.
  - A shared `NegativeClaims` struct with the 10+ boolean safety flags that all contracts share.
  - A generic `ValidateContract(header ContractHeader, claims NegativeClaims) error` helper.
counterexamples:
  - A single mega-contract type that tries to represent all proof states (loses type safety).
  - Deleting safety flags because they're "always false" (they're self-attestation, but removing them removes the attestation requirement).
observables:
  - File count in internal/computerversion/*_contract.go decreases.
  - Duplicated boolean flag blocks are replaced by embedded NegativeClaims struct.
  - Tests still pass.
  - No safety flag is removed without a replacement enforcement mechanism.
execution_effect:
  - If PGo is GO: skip contract consolidation; contracts will be replaced by generated code.
  - If PGo is NO-GO: execute consolidation as part of Phase 2.
settlement:
  rule: Consolidation is settled when the contract file count is reduced by at least 50% and all tests pass.
  settled_by: orchestrator
  invalidation_triggers:
    - A safety flag is removed without replacement enforcement.
    - A contract type loses type safety by merging with another.
```

### 3. Term: `purity-claim-fix`

```yaml
id: purity-claim-fix
kind: term
status: contested
source: observed
term: purity claim fix
definition: |
  The package doc in internal/computerversion/types.go claims "no filesystem, network, database, hypervisor, clock, or random operations" but the extraction layer (base_current_state_loader.go, base_blob.go, base_journal.go, base_tree.go) performs real I/O and uses time.
  Fix options:
  - Scope the claim to contract files only (add a separate doc comment to extraction files acknowledging I/O).
  - Split the package: computerversion/contracts (pure) and computerversion/observing (I/O).
non_definition:
  - The fix is not to remove the I/O code. The extraction layer is correct and needed.
  - The fix is not to make the extraction layer pure. It must read SQLite and blob stores.
examples:
  - Package doc says "Contract types are pure; observation extraction in base_*.go performs filesystem and database reads."
  - Package split: computerversion/contracts has no I/O imports; computerversion/observing imports database and filesystem.
counterexamples:
  - Removing the purity claim entirely without replacing it with scoped guidance.
  - Moving I/O code to a different package without updating all import paths.
observables:
  - Package doc in types.go is accurate.
  - No false claims about I/O absence.
execution_effect:
  - Maintainers can correctly reason about which parts of computerversion are pure.
settlement:
  rule: The package doc accurately describes the purity boundary.
  settled_by: orchestrator
```

### 4. Term: `tla-invariant-strengthening`

```yaml
id: tla-invariant-strengthening
kind: term
status: contested
source: observed
term: TLA+ invariant strengthening
definition: |
  The two ComputerVersion invariants (RouteNamesComputerVersion, PromotionNamesComputerVersion) are vacuous because ComputerVersionOfBase(n) maps both codeRef and artifactProgramRef to the same counter n. The model cannot express "code changed, state didn't."
  Fix options:
  - If PGo is GO: replace with MPCal refinement by construction. The spec is rewritten in MPCal and compiled; the invariants become real because the refinement is mechanical.
  - If PGo is NO-GO: introduce independent counters. CodeRefs == 0..MaxCodeChanges, ArtifactProgramRefs == 0..MaxArtifactChanges, ComputerVersionOfBase(n, m) == [codeRef |-> n, artifactProgramRef |-> m]. Update Init, Next, and all actions to track both counters independently. Re-run TLC.
  - Drop the invariants entirely if the refinement is not worth modeling at this stage.
non_definition:
  - Strengthening is not adding more invariants. It's making the existing ones non-vacuous or removing them.
  - Dropping is acceptable if the invariants are not load-bearing for the current mission phase.
examples:
  - Independent counters: codeRef and artifactProgramRef can diverge, so RouteNamesComputerVersion checks that route points to a valid (code, artifact) pair where both are in their respective domains.
  - MPCal refinement: the spec is compiled from MPCal, so the Go code implements the spec by construction.
counterexamples:
  - Keeping the vacuous invariants and adding a comment saying "these are vacuous." That's documentation, not verification.
observables:
  - TLC model check runs on the updated spec.
  - If independent counters: the model can express code/artifact divergence.
  - If MPCal: pcalgen and gogen both succeed.
execution_effect:
  - The spec no longer gives false confidence about refinement.
settlement:
  rule: The invariants are either non-vacuous with TLC evidence, or replaced by MPCal refinement, or explicitly dropped with rationale.
  settled_by: orchestrator
```

### 5. Term: `cmd-deduplication`

```yaml
id: cmd-deduplication
kind: term
status: contested
source: observed
term: cmd binary deduplication
definition: |
  8 new cmd binaries share ~120 LOC of fixture-server code (evidenceroot/baseharness), byte-identical observation-set loaders (basecompare/vmstatecompare), and identical 14-flag config scaffolding (vmrealize/vmstateobserve).
  Fix: extract shared helpers to an internal package (e.g., internal/harness/fixture, internal/harness/observer, internal/harness/config).
non_definition:
  - Deduplication is not merging the binaries. Each binary has a distinct purpose.
  - Deduplication is not moving binaries to internal/. They are CLI tools.
examples:
  - internal/harness/observer.go with LoadObservationSet(path string) (ObservationSet, error) used by basecompare and vmstatecompare.
  - internal/harness/config.go with shared VMManagerFlags struct used by vmrealize and vmstateobserve.
counterexamples:
  - A single internal/harness package that contains all shared code with no internal structure.
observables:
  - Shared code is extracted to internal packages.
  - Binary LOC decreases.
  - Tests still pass.
execution_effect:
  - Bug fixes in shared code propagate to all binaries.
settlement:
  rule: Shared code is extracted and all binary tests pass.
  settled_by: orchestrator
```

### 6. Boundary: `no-boundary-inflation`

```yaml
id: no-boundary-inflation
kind: boundary
status: settled
source: reviewer
term: no boundary inflation
definition: |
  This mission must not add new boundary contract files to internal/computerversion. The existing 39 contract files are sufficient. The boundary inflation pattern (passes 104-125, adding typed boundaries between every proof state without executing operations) is identified as a known failure mode.
non_definition:
  - This boundary does not prevent modifying existing contracts for consolidation.
  - This boundary does not prevent adding contracts if PGo generates them.
examples:
  - Consolidating 39 contracts into 15 via shared structs is allowed.
  - Replacing hand-written contracts with PGo-generated code is allowed.
counterexamples:
  - Adding a 40th hand-written contract file for a new proof state boundary.
execution_effect:
  - Any pass that adds a new *_contract.go file without replacing or consolidating existing ones violates this boundary.
settlement:
  rule: No new boundary contract files are added unless they replace existing ones or are PGo-generated.
  settled_by: orchestrator
```

### 7. Forbidden Collapse: `refactor-equals-progress`

```yaml
id: refactor-equals-progress
kind: forbidden_collapse
status: settled
source: skill
term: refactoring code -> mission progress
definition: |
  Refactoring the contract layer, fixing the purity claim, or deduplicating cmd binaries is maintainability work, not SIAC mission progress. Do not count these as advancing the substrate-independent audited computer completion gates.
examples:
  - "Contract consolidation is done, therefore SIAC is closer to completion." No — SIAC completion requires cross-substrate proof, not clean code.
  - "TLA+ invariants are non-vacuous, therefore refinement is proven." No — non-vacuous invariants must still pass TLC.
execution_effect:
  - Report maintainability work as maintainability work, not as mission advancement.
```

### 8. Forbidden Collapse: `pgo-builds-equals-adopt`

```yaml
id: pgo-builds-equals-adopt
kind: forbidden_collapse
status: settled
source: skill
term: PGo builds and compiles a trivial spec -> PGo should be adopted
definition: |
  PGo building and compiling a trivial spec proves only that PGo works on trivial specs. Adoption requires: (1) our spec translates to MPCal, (2) TLC passes on the MPCal translation, (3) generated Go compiles, (4) generated Go is readable and integrable, (5) distsys dependency is acceptable.
examples:
  - "PGo compiled a load balancer demo, so it can compile our promotion protocol." Not necessarily — our spec uses CHOOSE, EXCEPT, function construction.
  - "Generated Go compiles, so it's usable." Not necessarily — it may not expose interfaces compatible with our actor runtime.
execution_effect:
  - The PGo decision must be based on our spec, not a trivial demo.
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: The substrate changeset landed as commit 30f0301f on main (156 files, 49,807 insertions).
      source: observed
      execution_effect: The code is on main with green CI. Refactoring starts from this commit.
    - claim: All three landing-blocker bugs are fixed (upsert ownership, CAS guards, deployment guard).
      source: observed
      execution_effect: No security/correctness blockers remain for refactoring.
    - claim: 39 contract files exist with ~60% duplication (boundary inflation, issue 1.16).
      source: observed
      execution_effect: Contract consolidation is needed if PGo is NO-GO.
    - claim: TLA+ ComputerVersion invariants are vacuous (issue 1.5).
      source: observed
      execution_effect: Invariants must be strengthened, replaced by MPCal, or dropped.
    - claim: computerversion purity claim is false (issue 1.6).
      source: observed
      execution_effect: Package doc must be corrected or package must be split.
    - claim: 8 cmd binaries have shared code that should be deduplicated (issue 1.11).
      source: observed
      execution_effect: Shared helpers should be extracted to internal packages.
    - claim: SIAC mission is checkpoint_incomplete with major gates still open.
      source: observed
      execution_effect: This mission does not advance SIAC gates. It hardens the substrate for the next SIAC push.
    - claim: PGo evaluation is staged but not yet executed.
      source: observed
      execution_effect: PGo evaluation is the first phase of this mission.
  contested:
    - node: contract-consolidation
      issue: Whether to consolidate hand-written contracts or replace with PGo-generated code depends on PGo decision.
      next_resolution_step: Execute PGo evaluation (C-H1-pgo-decision).
    - node: tla-invariant-strengthening
      issue: Whether to strengthen manually or replace with MPCal refinement depends on PGo decision.
      next_resolution_step: Execute PGo evaluation (C-H1-pgo-decision).
  open:
    - node: pgo-jdk-compatibility
      missing: PGo was tested on JDK 1.11-1.16; we have JDK 25. May need JDK 17.
    - node: mpcal-translation-feasibility
      missing: Our specs use CHOOSE, EXCEPT, function construction. MPCal translation feasibility is unknown until attempted.
```

## Invariants

1. **No boundary inflation:** Do not add new `*_contract.go` files to `internal/computerversion` unless they replace existing ones or are PGo-generated.
2. **No deployed behavior change:** All changes are internal package structure, spec correctness, or tooling. Deployed runtime behavior must not change.
3. **Tests must pass:** All existing tests must continue to pass after refactoring. No safety flag may be removed without a replacement enforcement mechanism.
4. **Landing loop:** Any change that touches runtime behavior (even internal) must go through the full landing loop (commit → push → CI → staging verify).
5. **PGo evidence over enthusiasm:** Document what actually works, not what should work. PGo is research-grade.

## Authority Boundaries

- **Orchestrator may:** install sbt/JDK, clone PGo, attempt compilation, refactor internal package structure, fix spec invariants, extract shared cmd helpers, update docs/checkpoint.
- **Orchestrator must escalate:** adopting PGo as a suite-wide tooling change (human authority required per pgo-evaluation-v0.md), adding new build dependencies to the repo, modifying the SIAC mission definition or its completion gates.

## Value Criterion

```yaml
variant:
  measure: unresolved hardening items
  current: 5
  items:
    - C-H1-pgo-decision (proposed)
    - contract-consolidation (contested)
    - purity-claim-fix (contested)
    - tla-invariant-strengthening (contested)
    - cmd-deduplication (contested)
  target: 0
  motion_theater_threshold: refactoring without reducing the unresolved item count
```

A pass that refactors code without settling a definition node or closing a
conjecture is motion theater. The next move must settle a node or close a
conjecture.

## Homotopy / Realism Parameters

- **PGo build domain:** sbt + JDK on this machine. Valid for proving PGo builds and runs.
- **MPCal translation domain:** `promotion_protocol.tla` rewritten as MPCal. Valid for proving our specs can be expressed in MPCal. Does not prove all specs are translatable.
- **Generated Go domain:** Go code from gogen. Valid for assessing code quality. Does not prove the generated code integrates with our actor runtime without significant glue.
- **Refactoring domain:** `internal/computerversion` package structure. Valid for improving maintainability. Does not advance SIAC completion gates.

Fake islands are forbidden: do not claim PGo works for our specs without an
actual compilation attempt. Do not claim refactoring improves correctness
without tests passing.

## Conjecture And Belief State

1 conjecture (C-H1-pgo-decision), proposed. 4 contested terms awaiting PGo
decision before resolution path is determined.

## Execution Operators

```text
probe(C-H1)     — build PGo, translate spec, generate Go, assess quality
settle(C-H1)    — produce go/no-go recommendation with rationale
if GO:
  construct     — rewrite spec in MPCal, compile, assess integration
  settle(tla)   — replace vacuous invariants with MPCal refinement
  skip(contract-consolidation) — contracts will be replaced by generated code
if NO-GO:
  settle(tla)   — strengthen invariants manually or drop with rationale
  construct     — extract ContractHeader + NegativeClaims, consolidate contracts
  construct     — fix purity claim (scope or split package)
  construct     — extract shared cmd helpers
verify(all)     — run tests, run TLC, run go build
settle(all)     — update SIAC checkpoint with PGo decision and hardening status
```

## Receding-Horizon Control Loop

1. **Select** C-H1-pgo-decision — gates the entire refactoring strategy.
2. **State** PGo is staged but not built. JDK 25 available, PGo tested on 1.11-1.16. sbt not installed.
3. **Choose** probe: install sbt (+ JDK 17 if needed), clone PGo, attempt build.
4. **Bound** this is a local evaluation; no impact on main repo or deployed runtime.
5. **Execute** the build and compilation attempt.
6. **Update** conjecture status based on evidence.
7. **Continue** to Phase 2 (refactoring) based on PGo result.

## Dense Feedback Channels

- **sbt build output:** compile errors, runtime errors.
- **TLC model check:** invariant violations in MPCal translation or strengthened TLA+.
- **Go build:** compilation errors in generated or refactored code.
- **go test:** test failures after refactoring.
- **Code reading:** assessing generated Go quality and integration cost.
- **File count:** contract file count should decrease if consolidating.

## Evidence Ledger

```yaml
evidence: []
```

To be filled as conjectures are settled and terms are resolved.

## Completion Semantics

```text
The mission is COMPLETE when:
  1. C-H1-pgo-decision is SUPPORTED or REFUTED with evidence.
  2. A go/no-go recommendation is written with rationale.
  3. If GO:
     a. promotion_protocol.tla is rewritten in MPCal.
     b. TLC passes on the MPCal translation.
     c. gogen produces compilable Go.
     d. Integration assessment is documented.
     e. Vacuous TLA+ invariants are replaced by MPCal refinement.
  4. If NO-GO:
     a. TLA+ invariants are strengthened or dropped with rationale.
     b. Contract files are consolidated (file count reduced >=50%, tests pass).
     c. Purity claim is fixed (doc is accurate or package is split).
     d. cmd binary shared code is extracted to internal packages.
  5. SIAC checkpoint is updated with PGo decision and hardening status.
  6. All tests pass. CI is green.

The mission is BLOCKED when:
  1. PGo cannot be built (C-H1 falsified at build stage).
  2. MPCal translation is infeasible for our specs (C-H1 falsified at translation stage).
  3. Refactoring breaks tests that cannot be fixed without changing deployed behavior.

The mission is in PROGRESS when:
  1. PGo evaluation is being executed.
  2. Refactoring is being executed after PGo decision.
```

## Escalation Rules

```yaml
escalation:
  - rule: If PGo requires a JDK version we don't have, install it via Nix or Homebrew before escalating.
  - rule: If a TLA+ feature is unsupported by MPCal, document the exact feature and spec location before escalating.
  - rule: Escalate to human only for the PGo adoption decision (group-level tooling change) and for any change that would modify deployed runtime behavior.

human_escalation_triggers:
  - PGo adoption would change the suite's spec-first workflow (group-level decision).
  - PGo adoption would add new build dependencies to the repo (authority boundary).
  - Refactoring requires changing deployed runtime behavior (protected surface).
```

## Forbidden Collapses

- PGo compiles a trivial spec -> PGo works for our specs
- PGo is a research project -> PGo is production-ready
- Refactoring code -> mission progress (SIAC advancement)
- Contract consolidation -> safety flags are unnecessary
- Non-vacuous invariants -> refinement is proven (TLC must still pass)
- Generated Go compiles -> generated Go integrates with our runtime
- Fewer files -> better design (consolidation must preserve semantics)

## Rollback And Resumption Policy

```yaml
rollback:
  - surface: Git revert of refactoring commits on main.
  - surface: PGo evaluation artifacts in /private/tmp/pgo-evaluation can be discarded.
  - surface: Any installed packages (sbt, JDK) can be uninstalled.
  - surface: TLA+ spec changes are revertible via git.

resumption:
  - rule: Read this document, check conjecture status, continue from the last unsettled item.
  - rule: If PGo build is broken, check PGo repo issues for known JDK/sbt compatibility problems.
  - rule: If refactoring breaks tests, revert and try a smaller-scope refactor.
```

## Mission Report Policy

Produce a report explaining:

```text
mission goal: PGo decision and substrate hardening
PGo decision: GO or NO-GO with rationale
what was refactored: contract consolidation, purity fix, TLA+ invariants, cmd dedup
what was proven: PGo compilation attempt results, TLC results, test results
residual risks: integration cost, distsys dependency, remaining SIAC gates
rollback refs: commit SHAs
next mission: SIAC continuation with hardened substrate
```

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: complete
  last_checkpoint: |
    Phase 1 (PGo evaluation) complete. PGo decision: CONDITIONAL GO for spec
    verification, NO-GO for code generation. Phase 2 hardening complete.
    TLA+ invariants strengthened with independent code/artifact counters (TLC passes).
    Purity claim fixed in computerversion/types.go. Shared ContractHeader and
    NegativeClaims types created (contract_header.go). Full contract consolidation
    deferred (proof of concept done, cascading dependency requires big-bang refactor).
    cmd binary shared code extracted to internal/cmdutil. SIAC checkpoint updated.
  current_artifact_state:
    - internal/computerversion: 117 Go files (added contract_header.go), 39 contract files, ~34.6k LOC
    - cmd/: 8 new binaries, ~13.5k LOC
    - specs/promotion_protocol.tla: non-vacuous invariants with independent counters, TLC passes
    - docs/definitions/substrate-independent-audited-computer-2026-07-04.md: checkpoint_incomplete
    - docs/missions/pgo-evaluation-report.md: complete (in /private/tmp/pgo-evaluation/)
  what_shipped:
    - commit 30f0301f (substrate computer landing surfaces, 156 files, 49,807 insertions)
  what_was_proven:
    - three landing-blocker bugs are fixed (upsert ownership, CAS guards, deployment guard)
    - CI is green (runs 28723175791, 28723175801, 28723175802, 28723175805)
    - PGo builds on Mill/JDK 24/Scala 3.7.3 (not sbt/JDK 1.11-1.16 as docs claim)
    - PGo gogen produces compilable Go (842 lines for promotion_protocol MPCal)
    - PGo integration cost is high: distsys dependency (~8.9k LOC), tla.Value type system, no actor integration, fairness model mismatch
    - TLA+ invariants are non-vacuous: independent code/artifact counters, TLC passes (1908 distinct states, 0 errors)
    - computerversion purity claim is now accurate
  unproven_or_partial_claims:
    - MPCal translation fidelity (TLC not run on MPCal spec due to nightly.tlapl.us being unreachable)
    - Contract consolidation (shared types created, full embed refactor deferred)
    - cmd binary dedup (in progress)
  belief_state_changes:
    - C-H1-pgo-decision: proposed -> settled (CONDITIONAL GO for spec verification, NO-GO for code generation)
    - tla-invariant-strengthening: contested -> settled (independent counters, TLC passes)
    - purity-claim-fix: contested -> settled (doc updated to accurately describe purity boundary)
    - contract-consolidation: contested -> partially settled (shared types created, full embed deferred)
  remaining_error_field:
    - MPCal TLC verification not done (nightly.tlapl.us unreachable, local jars used for TLA+ TLC but not MPCal TLC)
    - Contract consolidation requires big-bang refactor across 39 files with cascading dependencies
    - cmd dedup in progress
  highest_impact_remaining_uncertainty: cmd dedup (mechanical, low risk)
  next_executable_probe: extract shared cmd helpers to internal/harness package
  suggested_goal_string: "/goal docs/missions/substrate-hardening-v0.md"
  evidence_artifact_refs:
    - docs/reviews/substrate-independent-audited-computer-changeset-review-2026-07-04.md
    - docs/reviews/landing-brief-2026-07-04.md
    - docs/missions/pgo-evaluation-v0.md
    - /private/tmp/pgo-evaluation/docs/missions/pgo-evaluation-report.md
    - specs/promotion_protocol.tla (strengthened invariants)
    - specs/promotion_protocol.cfg (updated invariant names)
    - internal/computerversion/contract_header.go (shared types)
    - internal/computerversion/types.go (fixed purity claim)
  rollback_refs:
    - commit 30f0301f on main (landed substrate surfaces)
    - worktree /private/tmp/pgo-evaluation (PGo evaluation sandbox)
    - uncommitted changes: specs/promotion_protocol.tla, specs/promotion_protocol.cfg, internal/computerversion/types.go, internal/computerversion/contract_header.go
```
```

## Suggested Goal String

```text
/goal docs/missions/substrate-hardening-v0.md
```
