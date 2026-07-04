# Substrate-Independent Audited Computer Mission

## Harness Invocation Semantics

```text
/goal docs/definitions/substrate-independent-audited-computer-2026-07-04.md
```

Read this document as executable semantic authority. Execute autonomously until
its completion semantics are satisfied with named evidence, or until a sharply
evidenced escalation, blocker, or supersession condition is met. Do not treat
this document as a vision memo, task list, or suggestion. Its definitions govern
what future implementation work is allowed to mean.

## Source Authority Order

1. This document (definition graph + determined state + completion semantics)
2. Owner statement, 2026-07-04: full success means Firecracker, Cloud Hypervisor,
   Nucleus, containers, and other virtualization/container technologies are
   abstracted away; Choir can create a user-isomorphic audited computer with
   substrate independence.
   This mission is a refinement and rephrasing of the autoputer goal; autopaper
   is tabled until the autoputer works correctly.
3. `docs/computer-ontology.md` (canonical computer/product ontology)
4. `docs/memo-artifact-program-doctrine-2026-06-28.md` (artifact program
   doctrine)
5. `docs/vision-choir-category-texture-transclusion-v0.md` (audited computer
   vision)
6. `specs/promotion_protocol.tla` (promotion safety model)
7. `docs/agent-product-doctrine.md` (authority boundaries and product-path
   verification)
8. `AGENTS.md` (repo operating contract)
9. Existing suite definition:
   `docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md`

When this document conflicts with older docs that treat a VM, sandbox,
`data.img`, or any specific hypervisor as the product object, this document
governs for this mission. When this document is silent, follow the computer
ontology and artifact program doctrine. Escalate only for group-level authority
changes, unsafe mutation, or owner value decisions.

## Mutation Class

Authoring this definition is **yellow**: it changes future optimization pressure
and mission authority but does not change runtime behavior.

Future execution of this mission is **red** when it touches VM lifecycle,
materialization, candidate computers, promotion/rollback, persistent state,
Texture canonical writes, gateway/provider calls, staging deploy routing, or
run acceptance. Red execution must use the ceremony in `AGENTS.md`: conjecture
delta, protected surfaces, admissible evidence class, rollback path, and heresy
delta.

## Real Artifact / Object Of Work

The real object is not Firecracker, Cloud Hypervisor, Nucleus, a container, a
VM image, a disk snapshot, or a process supervisor.

The real object is a **user-isomorphic audited computer**:

```text
ComputerVersion = (CodeRef, ArtifactProgramRef)
```

where:

- `CodeRef` inventories the Choir interpreter/runtime closure that computes the
  computer;
- `ArtifactProgramRef` names the typed, ordered, tamper-evident transaction
  program that represents durable user/product state;
- materializers compute substrate-specific realizations from that pair;
- all user-observable durable semantics are preserved across acceptable
  substrate realizations.

Implementation artifacts are subordinate projections:

- Go interfaces, packages, and services that make the substrate boundary real;
- TLA+ or property models that define substrate-independent equivalence;
- materializer implementations for Firecracker and at least one non-Firecracker
  substrate or projection;
- extract/replay/round-trip verifiers;
- promotion/rollback records over `ComputerVersion`, not over opaque images;
- staging proof that an authenticated user's computer can survive substrate
  abstraction without losing audited state.

## Mission Purpose And Non-Purpose

**Purpose:** Make Choir able to create, run, verify, fork, promote, recover, and
project a user's audited computer from substrate-independent state, so the
choice of Firecracker, Cloud Hypervisor, Nucleus, container, host-process,
FileProvider, mobile projection, or future substrate is an implementation
detail with recorded capabilities and limits.

This mission is the active refinement of the autoputer goal inside the broader
Autoputer / Autopaper suite. It does not compete with that suite; it sharpens the
autoputer object so the rest of the suite has the right substrate. Autopaper is
intentionally tabled until the autoputer can be trusted as a
substrate-independent audited computer.

**Non-purpose:**

- This mission is not a hypervisor migration.
- This mission is not a Firecracker defense or replacement project.
- This mission is not a Nucleus naming exercise.
- This mission is not a containerization project.
- This mission is not an fsck automation loop.
- This mission is not a Universal Wire rescue path.
- This mission does not claim byte-identical VM images as the primary proof.
- This mission does not preserve `data.img` as canonical user state.
- This mission does not declare all process memory, sockets, terminal sessions,
  and in-flight jobs persistent unless a transaction type and replay law exists
  for them.

## Cognitive Transform Selection

Selected transforms and route-changing effects:

```yaml
transforms:
  - name: Object transform
    why: The apparent object is a VM or data image; the real object is the user-observable computer state computed from CodeRef and ArtifactProgramRef.
    changed_definition: Define ComputerVersion and UserIsomorphism as the mission object.

  - name: Category error transform
    why: Treating Firecracker, Cloud Hypervisor, Nucleus, or container as the computer repeats the sandbox category error.
    changed_scope: Substrates become materializers with capability manifests, not product identity.

  - name: Invariant transform
    why: Substrate independence is meaningless unless durable user semantics survive substrate changes.
    changed_verifier: Completion requires cross-substrate equivalence checks, not only boot success.

  - name: Homotopy transform
    why: A low-resolution proof must preserve the topology of the final system; fake local-only or toy substrates do not count.
    changed_evidence_plan: Early proofs may use limited ledgers, but must retain CodeRef + ArtifactProgramRef + Materializer + Equivalence structure.

  - name: Failure mode transform
    why: The known failure is opaque ext4 corruption being treated as product recovery.
    changed_forbidden_collapse: fsck repair is not audited-computer recovery unless state can be re-extracted or replayed.
```

## Definition Graph

### 1. Object: `audited-computer`

```yaml
id: audited-computer
kind: object
status: settled
source: owner-stated + authority-docs
term: audited computer
definition: A persistent Choir computer whose durable user/product state is represented by typed, ordered, tamper-evident transactions and can be audited, replayed, materialized, verified, forked, promoted, and recovered from named version references.
non_definition:
  - A VM instance.
  - A Firecracker microVM.
  - A Cloud Hypervisor guest.
  - A Nucleus capsule.
  - A container.
  - A `data.img` file.
  - A backup snapshot with no typed mutation history.
examples:
  - A user computer reconstructed from CodeRef plus ArtifactProgramRef onto a fresh substrate with matching ledger-level observable state.
  - A candidate computer forked by tape/program reference and later promoted through per-ledger verification and health-window semantics.
counterexamples:
  - Copying a 32 GiB ext4 image and calling it a migrated computer.
  - Repairing ext4 with fsck and calling the result audited without transaction/replay evidence.
observables:
  - CodeRef is recorded.
  - ArtifactProgramRef/tape head is recorded.
  - Durable ledgers can be replayed or checked against materialized state.
  - Provenance answers who changed what, under what code, with what inputs and outputs.
execution_effect:
  - New implementation work must make durable state more typed, replayable, and substrate-independent.
  - Substrate-specific code is allowed only behind a materializer boundary or explicitly marked legacy.
formalization:
  status: required
  note: Project into a TLA+ or property model for cross-substrate equivalence and promotion over ComputerVersion.
settlement:
  rule: Settled by owner statement plus computer ontology and artifact program doctrine.
  settled_by: human
  invalidation_triggers:
    - Owner redefines the product object away from audited computers.
```

### 2. Object: `computer-version`

```yaml
id: computer-version
kind: object
status: settled
source: artifact-program-doctrine
definition: The substrate-independent version identity of a computer: a pair of references `(CodeRef, ArtifactProgramRef)` sufficient to compute durable ledger state and materialize a usable projection.
non_definition:
  - A VM ID.
  - A process ID.
  - A hostname.
  - A disk image path.
  - A Firecracker snapshot.
examples:
  - `(git_commit + nix_closure + sbom, transaction_tape_head)`.
  - A rollback target named by code version and program/tape version.
counterexamples:
  - `/var/lib/go-choir/vm-state/vm-.../data.img`.
observables:
  - Stored references exist in durable state.
  - Materializer can resolve them or reports a typed missing input.
execution_effect:
  - Promotion and rollback should converge on moving route pointers between ComputerVersions.
formalization:
  status: required
  note: Every substrate-independent proof depends on ComputerVersion equality semantics.
settlement:
  rule: Settled by doctrine equation `computer = choir_code_vN(artifact_program_vM)`.
  settled_by: evidence
```

### 3. Term: `substrate`

```yaml
id: substrate
kind: term
status: settled
source: owner-stated
term: substrate
definition: A concrete execution/materialization technology used to realize or project a ComputerVersion, such as Firecracker, Cloud Hypervisor, Nucleus, a container runtime, host process, FileProvider projection, mobile document provider, or future backend.
non_definition:
  - The product computer.
  - The source of truth for durable user state.
  - A universal proof of auditability.
examples:
  - Firecracker running the full VM projection.
  - A container materializing a limited server/process projection.
  - FileProvider materializing a file-tree projection.
counterexamples:
  - `data.img` treated as canonical because Firecracker boots it.
observables:
  - A materializer implementation declares its substrate and capabilities.
  - Substrate-specific state is classified as durable, ephemeral, or cache.
execution_effect:
  - Code must keep substrate-specific APIs behind materializer/capability boundaries.
settlement:
  rule: Owner explicitly named Firecracker, Cloud Hypervisor, Nucleus, virtualization, and container technology as things to abstract away.
  settled_by: human
```

### 4. Object: `materializer`

```yaml
id: materializer
kind: object
status: settled
definition: A component that takes a ComputerVersion plus substrate capability manifest and produces a substrate-specific realization or projection with declared equivalence scope.
non_definition:
  - A raw VM launcher.
  - A disk copier.
  - A backup restore script.
  - A hypervisor-specific lifecycle manager with no replay/equivalence contract.
examples:
  - FirecrackerFullMaterializer computes a VM image and boots it.
  - ContainerProcessMaterializer computes a process/container projection for a scoped service.
  - FileProjectionMaterializer computes a Finder/mobile file view from the same manifest.
counterexamples:
  - Calling `BootVM(data.img)` with no CodeRef, ArtifactProgramRef, or verification.
observables:
  - Interface accepts ComputerVersion or resolved ledger heads.
  - Capability manifest records supported ledgers, unsupported state classes, isolation semantics, and equality verifier.
  - Optional observation sources, including future `ebpf-*` sources, declare kernel/substrate requirements, privilege scope, and PII/retraction path before their events can become Trace evidence.
  - Output includes materialization identity and evidence references.
execution_effect:
  - First implementation work should define the materializer contract before adding another substrate.
formalization:
  status: testing
  note: Interface-level contract is tested with projection and scoped Firecracker/vmmanager materializers; full lifecycle/refinement formalization remains open.
settlement:
  rule: Settled for the interface boundary after `ProjectionMaterializer` and `VMManagerScopedMaterializer` capability manifests exist with focused equivalence/narrowing tests.
  settled_by: evidence
```

### 5. Term: `user-isomorphic`

```yaml
id: user-isomorphic
kind: term
status: proposed
source: owner-stated
term: user-isomorphic
definition: Two materializations are user-isomorphic when they preserve the same durable, user-observable computer semantics under a declared observation set, even if their implementation substrates, process layouts, device models, image bytes, and performance differ.
non_definition:
  - Byte-identical ext4 image.
  - Same hypervisor.
  - Same process memory.
  - Same transient sockets or terminal scrollback unless explicitly modeled.
  - A UI screenshot that merely looks similar.
examples:
  - Firecracker and Cloud Hypervisor realizations expose the same file manifest, Dolt/app state, object graph, permissions, route identity, and provenance answers for a ComputerVersion.
  - Full VM and FileProvider projection agree on the file ledger while the FileProvider correctly declares it does not realize processes.
counterexamples:
  - A container boots but loses installed packages or app state.
  - A rebuilt VM answers health but has different file ownership, missing blobs, or changed Dolt head.
observables:
  - Equivalence checker compares ledger roots and declared user-observable contracts.
  - Any unsupported dimension is declared in the capability manifest and excluded from the claim scope.
execution_effect:
  - Success claims must say which observation set is preserved.
  - Substrate comparisons must not claim more than their declared capabilities prove.
formalization:
  status: required
  note: Define `Observe(substrate, ComputerVersion, ObservationSet)` and an equivalence relation over observations.
settlement:
  rule: Settled after the mission defines observation sets and implements at least one checker.
  settled_by: evidence
```

### 6. Object: `artifact-program`

```yaml
id: artifact-program
kind: object
status: settled
source: artifact-program-doctrine
definition: The typed mutation transaction history that computes the computer's durable state under a CodeRef.
non_definition:
  - A static graph snapshot alone.
  - An opaque backup.
  - A log with no typed replay semantics.
examples:
  - A hash-chained transaction tape with file writes, Dolt commits, config changes, blob ingests, promotions, and code adoption events.
counterexamples:
  - `data.img` plus journalctl logs.
observables:
  - Transactions have author, order, type, inputs, outputs, and code version.
  - Tape/program head is content-addressed or tamper-evident.
execution_effect:
  - New persistent state classes require transaction types or explicit non-persistent classification.
formalization:
  status: required
  note: Tape ordering and replay semantics must be checkable.
settlement:
  rule: Settled by doctrine and owner statement.
  settled_by: evidence
```

### 7. Boundary: `persistent-ephemeral-cache`

```yaml
id: persistent-ephemeral-cache
kind: boundary
status: proposed
definition: Every state byte in a materialization must be classified as persistent product state, ephemeral working state, or reconstructible cache.
non_definition:
  - A vague best-effort backup rule.
  - Treating unknown state as persistent by default.
  - Treating unknown state as cache by default.
examples:
  - Dolt app state is persistent.
  - Go module cache is cache.
  - A live TCP socket is ephemeral unless a resumable protocol transaction exists.
  - `data.img` is a cache only after all persistent bytes inside it are represented elsewhere.
counterexamples:
  - Saying `data.img` is disposable before raw persistent writes are captured or explicitly out of scope.
observables:
  - Materializer manifests list mount roots/state classes.
  - Extract/replay verifier reports unknown or unclassified state.
execution_effect:
  - Unknown persistent-looking state blocks full substrate-independence claims.
formalization:
  status: required
  note: Classification should become schema and verifier output.
settlement:
  rule: Settled per state class by transaction coverage and verifier evidence.
  settled_by: evidence
```

### 8. Invariant: `substrate-independence`

```yaml
id: substrate-independence
kind: invariant
status: proposed
definition: The durable semantics of a ComputerVersion are independent of the substrate used to materialize or project it, within the declared capability and observation set of that substrate.
non_definition:
  - Every substrate supports every capability.
  - Every materialization is byte-identical.
  - Hypervisor snapshots are enough.
examples:
  - Same ComputerVersion produces equivalent durable observations on Firecracker and another full-computer substrate.
  - Same file manifest projects equivalently through VM mount and FileProvider.
counterexamples:
  - A Cloud Hypervisor migration that still treats its disk image as canonical.
  - A container projection that claims full-computer equivalence while excluding package installs and process services.
observables:
  - Cross-substrate equivalence tests pass.
  - Unsupported capabilities are explicit and block overbroad claims.
execution_effect:
  - Implementation may add substrates only after the materializer/equivalence boundary is defined.
formalization:
  status: required
  note: Cross-substrate equivalence model required before completion.
settlement:
  rule: Settled only by cross-substrate evidence, not by model agreement.
  settled_by: evidence
```

### 9. Target Conjecture: `data-img-disposable`

```yaml
id: data-img-disposable
kind: target_conjecture
status: proposed
definition: A `data.img` file is disposable for a state class only when deleting it cannot delete durable user/product state because the same state can be reconstructed from CodeRef and ArtifactProgramRef.
non_definition:
  - A repaired ext4 image.
  - A backed-up ext4 image.
  - A sparse copied ext4 image.
examples:
  - Delete cache image; materialize from ComputerVersion; ledger roots match.
counterexamples:
  - Delete image and lose user files, Dolt state, installed packages, or provenance.
observables:
  - Rebuild-from-program proof.
  - Extract/replay comparison.
  - No unknown persistent state remains in the image.
execution_effect:
  - Agents must not call `data.img` a cache for a state class until this target conjecture is proven for that class.
formalization:
  status: required
  note: At minimum property/integration tests for delete-and-rebuild on scoped fixtures.
settlement:
  rule: Settled per state class, then per computer, then platform-wide.
  settled_by: evidence
```

### 10. Invariant: `route-over-computer-version`

```yaml
id: route-over-computer-version
kind: invariant
status: proposed
definition: Active/candidate/published route identity should eventually point to ComputerVersion records and promotion certificates, not directly to substrate-specific VM IDs or image paths.
non_definition:
  - Removing VM IDs from implementation internals prematurely.
  - Ignoring current service names.
examples:
  - Owner route flips from active ComputerVersion to candidate ComputerVersion after per-ledger verification and health-window confirmation.
counterexamples:
  - Route points to a Firecracker VM ID and rollback means restoring an image path.
observables:
  - Promotion records name CodeRef, ArtifactProgramRef, ledger roots, capability manifests, verifier results, and rollback ComputerVersion.
execution_effect:
  - Promotion work must converge with substrate-independence work.
formalization:
  status: candidate
  note: Extend or sibling `specs/promotion_protocol.tla` with ComputerVersion and materializer variables.
settlement:
  rule: Settled when promotion spec and implementation records encode route-over-version.
  settled_by: formal-check + staging proof
```

### 11. Evidence Class: `cross-substrate-equivalence-proof`

```yaml
id: cross-substrate-equivalence-proof
kind: evidence_class
status: proposed
definition: Evidence that two or more materializers/projections for the same ComputerVersion produce equivalent observations under a declared observation set.
non_definition:
  - One substrate boots.
  - A screenshot matches.
  - Health endpoint returns 200 without ledger comparison.
examples:
  - Firecracker and container/file projection agree on manifest root, blob set, Dolt head, object graph head, and provenance query answers where both declare support.
  - Two nodes materialize the same ComputerVersion and produce matching ledger roots.
counterexamples:
  - Firecracker snapshot restored on another host with no transaction tape verification.
observables:
  - Exact command or product-path trace.
  - Observation set schema.
  - Materializer capability manifests.
  - Diff artifact for mismatches.
execution_effect:
  - Completion requires this evidence class.
settlement:
  rule: Settled when checker exists and has passing/failing fixture coverage.
  settled_by: evidence
```

### 12. Forbidden Collapse: `substrate-swap-as-success`

```yaml
id: substrate-swap-as-success
kind: forbidden_collapse
status: settled
definition: Replacing Firecracker with Cloud Hypervisor, Nucleus, containers, or any other backend does not by itself advance the mission unless it improves or proves substrate-independent audited-computer semantics.
examples:
  - Cloud Hypervisor with opaque canonical disk image is still the wrong object.
  - Container runtime with untyped persistent volume is still the wrong object.
execution_effect:
  - Hypervisor/container choices are deferred until the materializer contract can evaluate them.
settlement:
  rule: Settled by owner statement and current design consensus.
  settled_by: human
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: Full mission success means substrate technologies are abstracted away behind audited-computer semantics.
      source: owner-stated
      execution_effect: Do not optimize for a specific hypervisor as the product.

    - claim: Choir's product object is a persistent computer, not a sandbox or VM.
      source: docs/computer-ontology.md
      execution_effect: Use computer/candidate/platform terminology for product semantics.

    - claim: A computer is composed from heterogeneous ledgers with different merge laws.
      source: docs/computer-ontology.md
      execution_effect: Do not force every state change through one storage abstraction.

    - claim: Shared platform changes must become typed artifacts; opaque snapshots are hard to merge into divergent computers.
      source: docs/computer-ontology.md
      execution_effect: Platform updates must be represented as typed deltas/ledgers.

    - claim: The ideal user should not care whether their computer is backed by Firecracker, host-process fallback, NixOS image, worktree, or later substrate.
      source: docs/computer-ontology.md
      execution_effect: Substrate is implementation state that must be recorded and verified, not exposed as product identity.

    - claim: Artifact program doctrine states `computer = choir_code_vN(artifact_program_vM)`.
      source: docs/memo-artifact-program-doctrine-2026-06-28.md
      execution_effect: Define ComputerVersion as CodeRef plus ArtifactProgramRef.

    - claim: Current `data.img` is an opaque cache aspiration, but it is not actually disposable until persistent state is represented outside it.
      source: doctrine + observed Pass 3 filesystem corruption evidence in suite definition
      execution_effect: Treat `data.img` as legacy canonical for uncovered state until extract/replay proves otherwise.

    - claim: Passes 15-23 form a substrate-symptom cluster on opaque `data.img` state: capacity, host-image gauge, resume coalescing, duplicate Firecracker launches, and ext4 repair all patched symptoms of state living in a mutable image.
      source: docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md Pass 15 through Pass 23 + external read-only reviews, 2026-07-04
      execution_effect: Treat the Pass 23 fsck repair as protected-data rescue, not substrate-independent audited-computer progress.

    - claim: Promotion protocol already models computers as heterogeneous ledgers and promotion as atomic route flip guarded by prepare/verify/approval/freshness/health.
      source: specs/promotion_protocol.tla
      execution_effect: Substrate-independent promotion should refine or extend this model, not replace it with image copying.

  contested:
    - node: materializer
      issue: Exact Go package/API boundary is not settled.
      next_resolution_step: Read current vmmanager/vmctl ownership boundaries and define a minimal Materializer interface with no runtime behavior change.

    - node: user-isomorphic
      issue: Observation set must be specific enough to verify and narrow enough not to overclaim process-level identity.
      next_resolution_step: Define observation-set tiers: file-ledger, structured-app-ledger, full-computer-durable, live-process-continuity.

    - node: persistent-ephemeral-cache
      issue: Current persistent image contains mixed state; not all durable bytes have transaction coverage. Future eBPF or filesystem probes can inform classification, but they are observation sources, not artifact-program proof.
      next_resolution_step: Build inventory/extractor proof on a copy or fixture before declaring classes disposable; later, capability-scoped write tracing may identify uncovered mutable paths.

  open:
    - node: substrate-independent-spec
      missing: Formal model or property contract for Materialize/Observe equivalence.

    - node: first-non-firecracker-materializer
      missing: Choice of second substrate/projection for first equivalence proof.
```

## Invariants

```yaml
invariants:
  - id: I1
    name: Product object is not substrate
    rule: No implementation may redefine the user computer as Firecracker, Cloud Hypervisor, Nucleus, container, host process, or image path.

  - id: I2
    name: ComputerVersion authority
    rule: Durable route, fork, promotion, rollback, and recovery semantics must converge on CodeRef + ArtifactProgramRef.

  - id: I3
    name: Capability-scoped equivalence
    rule: A materializer may claim equivalence only for declared observation sets that it can verify.

  - id: I4
    name: No opaque durable state loss
    rule: A state byte may be discarded only if classified as cache/ephemeral or represented by typed persistent transactions.

  - id: I5
    name: Evidence-scope discipline
    rule: Boot success, health success, local test success, and model agreement cannot be promoted into substrate-independence claims without ledger/projection comparison.

  - id: I6
    name: Promotion compatibility
    rule: Candidate promotion remains guarded by per-ledger prepare/verify, owner approval, freshness CAS, health window, and rollback/confirm semantics.

  - id: I7
    name: Problem documentation first
    rule: New staging or product-path failures discovered during this mission must be documented before repair unless an explicit exception is recorded.
```

## Authority Boundaries

```yaml
authority_boundaries:
  orchestrator:
    may:
      - define leaf terms and interfaces inside this mission
      - create docs and specs
      - implement non-behavior-changing types/checkers/tests
      - delegate read-only or focused work to subagents
    must_escalate:
      - purpose/identity change away from substrate-independent audited computer
      - destructive mutation of production user state
      - choosing to abandon user-isomorphism as the success criterion
      - irreversible promotion/rollback behavior changes without rollback proof

  implementation:
    may:
      - preserve existing Firecracker/vmmanager internals during migration
      - add abstraction seams and verifiers before adding substrates
      - use fixture or candidate computers for proofs
    must_not:
      - silently make a new substrate canonical
      - delete or repair production images as routine recovery
      - claim full substrate independence from local-only fixtures
      - bypass product-path verification for staging claims
```

## Homotopy / Realism Parameters

Low-realism probes are valid only if they preserve this topology:

```text
CodeRef + ArtifactProgramRef -> Materializer -> ObservationSet -> EquivalenceCheck
```

Acceptable simplifications:

- one ledger instead of all ledgers, if declared;
- fixture computer instead of production computer, if claim remains fixture-scoped;
- file projection instead of full VM, if capability manifest excludes process state;
- one non-Firecracker projection instead of full Cloud Hypervisor/Nucleus, if the
  proof still uses the same ComputerVersion and observation checker.

Fake islands:

- mock data not derived from a ComputerVersion;
- copying `data.img` and comparing that copy;
- local boot proof cited as staging/product proof;
- health endpoint proof with no ledger comparison;
- deleting/recreating a volume in a test-only path that does not preserve current
  production state topology.

## Conjecture And Belief State

```yaml
conjectures:
  - id: SI-C1
    status: proposed
    claim: A Materializer boundary can be introduced before any new substrate is added.
    test: Add interface/types/checkers with no runtime behavior change and pass focused tests/docs checks.
    execution_effect: If supported, first PR stays yellow/orange-low rather than red hypervisor work.

  - id: SI-C2
    status: proposed
    claim: The first useful user-isomorphism proof should compare ledger observations, not ext4 bytes.
    test: Define ObservationSet and checker that compares manifest/blob/Dolt/objectgraph roots for two materializations or projection paths.
    execution_effect: If supported, completion criteria avoid ext4 byte-identical rabbit hole.

  - id: SI-C3
    status: proposed
    claim: Existing `internal/base`, `internal/objectgraph`, and Dolt substrates are enough to model the first artifact-program slice.
    test: Build a fixture tape/manifest/blob/object replay without adding a parallel storage substrate.
    execution_effect: If supported, deletion-first/reuse path is preferred over new database/tape invention.

  - id: SI-C4
    status: proposed
    claim: Current production `data.img` contains persistent state classes not yet recoverable from typed transactions.
    test: Extract inventory from safe copy/fixture, compare to known ledgers, classify unknown persistent bytes.
    execution_effect: If supported, `data.img` remains legacy canonical for uncovered classes until coverage exists.

  - id: SI-C5
    status: open
    claim: A non-Firecracker materializer/projection can prove substrate independence before a full second hypervisor backend exists.
    test: Build a file/container/projection materializer for a declared observation set and compare against Firecracker-derived observations.
    execution_effect: If supported, mission can progress without premature Cloud Hypervisor or Nucleus migration.
```

## Variant / Progress Measure

```yaml
variant:
  measure: unresolved definition nodes + unimplemented verifier contracts + unproven observation sets + red behavior surfaces without rollback proof
  current_open_nodes:
    - materializer
    - user-isomorphic
    - persistent-ephemeral-cache
    - substrate-independence
    - data-img-disposable
    - route-over-computer-version
    - cross-substrate-equivalence-proof
  target: 0 open load-bearing nodes and all completion semantics satisfied
  motion_theater_threshold: a pass that adds a substrate, doc, or boot path without reducing unknown persistent state, improving equivalence evidence, or settling a definition node
```

## Execution Operators

```text
define(node)          — make a term/interface/evidence class executable
split(node)           — separate overloaded state/substrate/projection meanings
counterexample(node)  — find a substrate/equivalence case that breaks the definition
formalize(node)       — project a safety/equivalence definition into TLA+, types, properties, or assertions
construct(node)       — mutate docs/specs/code under declared mutation class
verify(node)          — run the scoped checker/proof for the active claim
settle(node)          — promote/weaken/falsify/supersede with evidence
monitor(node)         — watch product/staging behavior for drift after settlement
```

## Receding-Horizon Control Loop

Each control interval:

1. Select the live definition/conjecture whose settlement most reduces substrate
   independence uncertainty.
2. State the active observer and blind spot: code, fixture, staging, production,
   model, or reviewer.
3. Choose one move: define, split, probe, formalize, construct, verify, settle.
4. Classify mutation class and rollback path before mutation.
5. Execute the smallest topology-preserving proof or the largest safe batch of
   independent non-behavior work.
6. Update this document's determined state, evidence ledger, and checkpoint if
   the run is executing under `/goal`.
7. Continue until completion, supersession, or hard escalation.

## Dense Feedback Channels

- **Static docs/spec review:** docs truth checker and source authority review.
- **Focused Go tests:** materializer interfaces, manifest/tape/blob/objectgraph
  properties, extract/replay fixtures.
- **Formal/model checks:** TLA+ or property models for route-over-version,
  materializer equivalence, and promotion compatibility.
- **Safe fixture/candidate computers:** no production mutation for early proofs.
- **Staging product path:** authenticated product APIs and browser evidence only
  when behavior claims require staging.
- **Second opinions:** use only for contested definitions that could change the
  route, verifier, scope, or stopping condition.

## Evidence Ledger

```yaml
evidence:
  - claim: Owner-defined full success requires substrate abstraction and user-isomorphic audited computers.
    definition_node: substrate-independence
    evidence_class: user-stated authority
    source: owner statement, 2026-07-04
    result: Mission reframed away from hypervisor-specific success.
    uncertainty: Exact observation sets still need definition.
    promotion_relevance: Governs mission completion semantics.

  - claim: Computer ontology already says implementation substrate should be hidden from the user and recorded/verified by the implementation.
    definition_node: substrate
    evidence_class: observed file
    source: docs/computer-ontology.md lines 331-345
    result: Existing doctrine supports substrate independence.
    uncertainty: Does not define materializer API.
    promotion_relevance: Settles product vocabulary.

  - claim: Artifact program doctrine defines the versioned equation for computer state.
    definition_node: computer-version
    evidence_class: observed file
    source: docs/memo-artifact-program-doctrine-2026-06-28.md lines 44-59
    result: `(choir_code_vN, artifact_program_vM)` is the intended current-state pair.
    uncertainty: Current implementation is not yet fully decomposed into artifact program.
    promotion_relevance: Grounds ComputerVersion.

  - claim: Promotion spec already treats computers as heterogeneous ledgers with route flip and health window.
    definition_node: route-over-computer-version
    evidence_class: formal spec source
    source: specs/promotion_protocol.tla lines 5-29
    result: Existing formal model is compatible with substrate-independent promotion.
    uncertainty: Spec uses abstract base versions, not yet CodeRef/ArtifactProgramRef.
    promotion_relevance: Defines refinement seam.

  - claim: External read-only reviews converged that eBPF belongs as an optional materializer capability and Trace source, not as artifact-program state or an equivalence proof.
    definition_node: materializer
    evidence_class: reviewer consensus
    source: Devin, Claude, Cursor, and Codex read-only architecture reviews, 2026-07-04
    result: Keep the next probe focused on Materializer/CapabilityManifest/ObservationSet/EquivalenceCheck with a failing mismatch fixture; design schemas so `ebpf-*` observation sources can be declared later.
    uncertainty: Exact eBPF program set, kernel/BTF requirements, and privacy path remain out of scope until the materializer contract exists.
    promotion_relevance: Prevents observability work from becoming a new product identity or false proof.

  - claim: A scoped Firecracker/vmmanager materializer boundary exists without invoking VM lifecycle behavior.
    definition_node: materializer
    evidence_class: focused Go tests
    source: internal/computerversion/vmmanager_boundary.go and internal/computerversion/vmmanager_boundary_test.go
    result: `VMManagerScopedMaterializer` emits a `vm_state_manifest` realization, compares equal for identical scoped VM state, fails for a seeded VM-state mismatch, and narrows durable file/blob claims through unsupported capabilities.
    uncertainty: This is a VM-state classification boundary only; it does not launch Firecracker, sample production data.img, or prove durable user-state equivalence.
    promotion_relevance: Moves completion item 2 for one scoped path while keeping full lifecycle and production proof out of scope.
```

## Completion Semantics

This mission is **COMPLETE** only when all of the following hold with named
evidence:

1. **Definitions settled:** `ComputerVersion`, `ArtifactProgramRef`,
   `Materializer`, `CapabilityManifest`, `ObservationSet`, `UserIsomorphism`,
   `Extract`, `Materialize`, and `EquivalenceCheck` are defined in docs and code
   or specs.
2. **Substrate boundary exists:** Firecracker/vmmanager-specific behavior is
   behind a materializer/capability boundary for at least one scoped path.
3. **Typed durable state slice exists:** At least one persistent state slice
   (minimum: file manifest/blob slice or Dolt/objectgraph slice) is represented
   by typed artifact-program references rather than only by opaque `data.img`.
4. **Cross-substrate/projection proof exists:** The same ComputerVersion is
   materialized or projected through Firecracker and at least one non-identical
   substrate/projection path, and the equivalence checker passes for a declared
   ObservationSet.
5. **Failure proof exists:** A seeded mismatch or unsupported capability causes
   the equivalence checker to fail or narrow the claim, proving the verifier is
   not ceremonial.
6. **Promotion/rollback model updated:** Promotion/rollback semantics name
   ComputerVersion or an explicit refinement path from current abstract versions
   to ComputerVersion.
7. **Staging/product proof exists for any runtime behavior claim:** If the run
   changes behavior, staging proves the relevant product path without bypassing
   approved product APIs.
8. **Documentation and checkpoint are current:** This document or its successor
   records evidence, rollback refs, remaining risks, and next realism axis.

This mission is **BLOCKED** when:

1. A required persistent state class cannot be classified without owner/product
   authority.
2. A substrate boundary change would risk production user state without accepted
   rollback/candidate proof.
3. Equivalence requires a capability that no available substrate/projection can
   expose and no narrower ObservationSet preserves topology.
4. Current code has a substrate-specific dependency that cannot be isolated
   without a larger architecture decision.

This mission is **SUPERSEDED** when owner/product authority replaces
substrate-independent audited computers with a different product object.

A checkpoint is not completion. A single booting VM, a new hypervisor, a green
local test, or a repaired `data.img` is not completion.

## Escalation Rules

Escalate for:

- changing the product object away from substrate-independent audited computers;
- destructive production data mutation;
- deciding that a state class is acceptable to lose;
- changing promotion/rollback authority boundaries;
- selecting a new mandatory infrastructure substrate with operational cost or
  security implications;
- claiming full user-isomorphism while excluding a user-visible durable semantic.

Do not escalate for ordinary interface naming, fixture shape, package location,
or local proof strategy when the invariants above remain intact.

## Forbidden Collapses

- substrate boots -> audited computer exists
- Cloud Hypervisor supports richer devices -> substrate independence is solved
- Firecracker snapshot restored -> durable state is typed
- `data.img` backed up -> `data.img` is disposable
- fsck repaired ext4 -> product recovery is solved
- two screenshots match -> user-isomorphic
- health endpoint is 200 -> ledgers are equivalent
- eBPF trace captured -> durable state is audited
- matching eBPF streams -> user-isomorphic
- byte-identical image differs -> ledger-level equivalence failed
- a model agrees -> definition settled
- a fixture passes -> production claim proven
- a new abstraction exists -> callers obey it
- materializer exists -> every state class is captured
- FileProvider projection works -> full computer substrate independence is done

## Rollback And Resumption Policy

```yaml
rollback:
  - surface: Documentation/spec changes
    path: git revert the commit.
  - surface: Non-behavior code interfaces/checkers
    path: git revert; no product state migration should depend on them until proven.
  - surface: Runtime materializer boundary
    path: preserve existing Firecracker/vmmanager path until cross-substrate proof and staging evidence exist.
  - surface: Production user state
    path: no destructive production mutation without explicit rollback refs, candidate/copy proof, and owner-authorized red ceremony.

resumption:
  - Read this document first.
  - Reconcile current repo state and existing suite definitions.
  - Start with the highest-impact unsettled definition node, not with a hypervisor choice.
  - If safe in-bound probes remain, execute them rather than calling the mission complete.
```

## Mission Report Policy

Maintain an owner-readable report when execution changes durable system state,
runtime behavior, formal specs, or staging deployment behavior. The report must
include:

```text
mission goal and artifact
mutation class and protected surfaces
what definitions settled
what shipped
what evidence proves and does not prove
cross-substrate observation set and results
rollback refs
heresy delta
residual risks
next realism axis
```

## Route Registration Red-Ceremony Assessment

```yaml
route_registration_assessment:
  status: assessed_not_mutated
  mutation_class_if_executed: red
  active_conjecture_delta:
    discovered:
      - Persistent Base API route registration is no longer blocked by handler or storage mechanics; it is blocked by deployed service/auth/session/routing authority.
    introduced: []
    repaired:
      - The prior ambiguity between "no observation mechanics" and "no product wiring" is repaired by `OpenPersistentHandler` plus handler-to-observation tests.
  protected_surfaces_if_executed:
    - auth/session and API key validation, because `/api/base/*` requires `read:base` and `write:base` scopes through `APIKeyValidator`.
    - staging deploy routing, because registering deployed routes changes which public service answers Base API paths.
    - persistent product state, because configured Base journal/blob roots would become writable product storage.
    - run acceptance/product-path verification, because a deployed route claim would need staging evidence, not only in-process handler tests.
  admissible_evidence_class_before_execution:
    - focused cmd/service config tests proving explicit journal/blob paths and route registration without network/staging.
    - integration test proving auth store + Base API scopes + persistent handler route through the selected service.
    - staging proof only after a behavior-changing route-registration commit is intentionally made and deployed.
  rollback_path_if_executed:
    - keep `OpenPersistentHandler` unused by deployed cmd services until the registration decision is made.
    - if route registration is added and fails, revert that service-registration commit; existing Firecracker/vmmanager paths remain current materialization.
    - do not migrate or delete existing user state as part of first registration.
  heresy_delta:
    discovered:
      - Base API exists as handler/client/sync vocabulary but is not yet a deployed persistent service path.
      - Desktop sync persists only local synced-state JSON and talks to a remote Base API; it is not the server-side Base persistence owner.
    introduced: []
    repaired:
      - Product wiring is now separated from handler observation proof.
  decision:
    - Do not register deployed Base API routes in this yellow/orange checkpoint.
    - Next safe probe should be a lower-risk local product-path harness or focused cmd/service test that proves explicit config and route registration without touching staging or production auth/session.
```

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: checkpoint_incomplete
  last_checkpoint: Pass 125 complete; human landing override accepted. Pass 126 boundary-inflation ceremony was not executed and is superseded by the landing brief's Phase 0 correctness fixes: intake ownership upsert, intake transition optimistic concurrency, and deployed write-route guard. Do not add further boundary-only contracts before landing these fixes.
  active_red_ceremony: null
  completed_red_ceremonies:
    - pass: 125
      mutation_class: red
      conjecture_delta: Existing runtime-equivalence retry proof can close the runtime-durable gap's retry obligation only if a handoff validates that it follows the admitted extraction handoff, compares source/runtime file_manifest and blob_set observations for the same ComputerVersion, and preserves every downstream proof gate.
      protected_surfaces:
        - runtime-durable proof gap boundary
        - runtime-durable gap extraction handoff boundary
        - runtime-equivalence retry boundary
        - runtime file/blob observation extraction boundary
        - VM lifecycle boundary
        - durable computer mutation boundary
        - deployed routing boundary
        - package-publication boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a runtime-equivalence retry contract can be admitted under the runtime-durable extraction handoff only when versions, artifact refs, source-provenance refs, extraction refs, boundary refs, observation sets, file_manifest/blob_set requirements, no-mutation flags, and downstream proof requirements align
        - focused internal/computerversion negative tests proving extraction handoff drift, retry drift, missing refs, mismatch status, observation scope drift, authority drift, no-mutation drift, and protected-surface/completion claims are rejected
      rollback_path:
        - git revert of the local computerversion runtime-durable gap retry handoff code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later VM lifecycle, staging, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, VM lifecycle state, promotion execution state, durable computer state, or run-acceptance records may be mutated in this pass
      heresy_delta:
        discovered:
          - Runtime-equivalence retry proof predates the runtime-durable gap and needs a second handoff after extraction admission before it can close the gap's retry obligation.
        introduced: []
        repaired:
          - BaseRuntimeDurableGapRetryHandoffContract now connects the existing BaseRuntimeEquivalenceRetryContract to the runtime-durable extraction handoff, preserving downstream proof obligations instead of treating scoped runtime file/blob equivalence as full-substrate or completion authority.
    - pass: 124
      mutation_class: red
      conjecture_delta: Existing runtime file/blob extraction proof can be reused after the runtime-durable proof gap only if a handoff validates that extraction satisfies the gap's required runtime file/blob obligation while preserving retry, staging, promotion, package-publication, run-acceptance, full-substrate, and completion obligations.
      protected_surfaces:
        - runtime-durable proof gap boundary
        - runtime file/blob observation extraction boundary
        - runtime-equivalence retry boundary
        - VM lifecycle boundary
        - durable computer mutation boundary
        - deployed routing boundary
        - package-publication boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a runtime file/blob extraction contract can be admitted under a runtime-durable proof gap only when versions, artifact refs, source-provenance refs, runtime boundary refs, materializer/substrate identities, file_manifest/blob_set observations, and no-mutation flags align
        - focused internal/computerversion negative tests proving gap drift, extraction drift, missing refs, authority drift, missing proof obligations, and protected-surface/completion claims are rejected
      rollback_path:
        - git revert of the local computerversion runtime-durable gap extraction handoff code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later VM lifecycle, staging, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, VM lifecycle state, promotion execution state, durable computer state, or run-acceptance records may be mutated in this pass
      heresy_delta:
        discovered:
          - Existing runtime file/blob extraction proof predates the runtime-durable gap and needs an explicit handoff before it can satisfy the gap's extraction obligation.
        introduced: []
        repaired:
          - BaseRuntimeDurableGapExtractionHandoffContract now connects the existing BaseRuntimeFileBlobExtractionContract to the runtime-durable proof gap, preserving runtime-equivalence retry and downstream proof obligations instead of treating extraction as equivalence success.
    - pass: 123
      mutation_class: red
      conjecture_delta: A runtime-durable proof gap can consume the narrowed runtime-equivalence reentry and local file/blob substrate proof summary only if it preserves the gap between local durable-state proof and runtime substrate proof; it must require runtime file/blob extraction and retry evidence instead of upgrading either input to completion authority.
      protected_surfaces:
        - runtime-durable proof gap boundary
        - runtime-equivalence reentry boundary
        - local substrate proof summary boundary
        - durable-state equivalence boundary
        - runtime substrate proof boundary
        - VM lifecycle boundary
        - deployed routing boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving narrowed runtime-equivalence reentry can be bound to local file/blob substrate proof summary while preserving runtime file/blob extraction, equivalence retry, staging, promotion, run-acceptance, and full-substrate proof obligations
        - focused internal/computerversion negative tests proving reentry drift, local summary drift, gap drift, missing refs, no-mutation flag drift, and protected-surface/completion claims are rejected
      rollback_path:
        - git revert of the local computerversion runtime-durable proof gap code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later VM lifecycle, staging, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, VM lifecycle state, promotion execution state, durable computer state, or run-acceptance records may be mutated in this pass
      heresy_delta:
        discovered:
          - Local file/blob substrate proof and narrowed runtime-equivalence reentry are complementary evidence, not a runtime substrate proof.
        introduced: []
        repaired:
          - BaseRuntimeDurableProofGapContract now records the open runtime-durable proof gap without granting runtime equivalence success, VM lifecycle, deployed routing, production, package publication, promotion, run-acceptance, full-substrate, or completion authority.
    - pass: 122
      mutation_class: red
      conjecture_delta: Runtime-equivalence reentry can consume a runtime-materialization bridge only if it preserves the existing narrowed equivalence result and continues to deny VM lifecycle, durable computer, deployed routing, production, package publication, promotion, run-acceptance, full-substrate, and completion authority.
      protected_surfaces:
        - runtime-equivalence reentry boundary
        - runtime-materialization ceremony bridge boundary
        - runtime-equivalence boundary
        - durable-state equivalence boundary
        - VM lifecycle boundary
        - deployed routing boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the runtime-materialization bridge can be bound to the existing narrowed runtime-equivalence boundary without VM lifecycle, durable-computer, deployed-route, production, package, promotion, run-acceptance, full-substrate, or completion authority
        - focused internal/computerversion negative tests proving bridge drift, equivalence-boundary drift, unsupported-observation drift, missing refs, no-mutation flag drift, and protected-surface/completion claims are rejected
      rollback_path:
        - git revert of the local computerversion runtime-equivalence reentry code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later VM lifecycle, staging, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, VM lifecycle state, promotion execution state, durable computer state, or run-acceptance records may be mutated in this pass
      heresy_delta:
        discovered:
          - A runtime-equivalence boundary remains narrowed after the bridge; bridge admissibility is not equivalence success.
        introduced: []
        repaired:
          - BaseRuntimeEquivalenceReentryContract now records narrowed runtime-equivalence reentry from the runtime-materialization bridge without granting VM lifecycle, deployed routing, production, package publication, promotion, run-acceptance, full-substrate, or completion authority.
    - pass: 121
      mutation_class: red
      conjecture_delta: Runtime-materialization ceremony evidence can consume source-provenance/materializer readiness only as a bridge to an existing scoped runtime ceremony contract; the bridge must not mutate VM lifecycle, durable computer state, production state, routing, promotion, run acceptance, or completion state.
      protected_surfaces:
        - runtime-materialization ceremony bridge boundary
        - source-provenance/materializer readiness boundary
        - runtime materialization boundary
        - VM lifecycle boundary
        - deployed routing boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving source-provenance/materializer readiness can be bound to an existing runtime-materialization ceremony contract without VM lifecycle, production, routing, promotion, run-acceptance, full-substrate, or completion authority
        - focused internal/computerversion negative tests proving readiness drift, runtime ceremony drift, missing refs, no-mutation flag drift, VM lifecycle/deployed-route/production/promotion/run-acceptance/package/full-substrate/completion claims are rejected
      rollback_path:
        - git revert of the local computerversion runtime-materialization ceremony bridge code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later VM lifecycle, staging, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, VM lifecycle state, promotion execution state, durable computer state, or run-acceptance records may be mutated in this pass
      heresy_delta:
        discovered:
          - Opening the runtime-materialization ceremony path still does not mean VM lifecycle mutation or deployed runtime behavior occurred; it only makes scoped runtime evidence admissible.
        introduced: []
        repaired:
          - BaseRuntimeMaterializationBridgeContract now records admissible runtime-materialization ceremony evidence from source-provenance/materializer readiness without granting VM lifecycle, deployed routing, production, package publication, promotion, run-acceptance, full-substrate, or completion authority.
    - pass: 120
      mutation_class: red
      conjecture_delta: Source-provenance/materializer readiness can consume a scoped durable-state-slice probe only if it validates existing source-provenance readiness and materializer boundary evidence while continuing to deny runtime materialization and downstream authority.
      protected_surfaces:
        - source-provenance/materializer readiness boundary
        - durable-state-slice probe boundary
        - materializer boundary
        - runtime materialization boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the scoped durable-state-slice probe can open only source-provenance/materializer readiness while runtime materialization, promotion, production, run-acceptance, full-substrate, and completion authority remain denied
        - focused internal/computerversion negative tests proving durable-slice probe drift, source-provenance readiness drift, materializer boundary drift, missing refs, no-mutation flag drift, runtime/durable-computer/package/promotion/run-acceptance/production mutation claims, full-substrate claims, and completion claims are rejected
        - local://pass120-base-source-materializer-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseSourceMaterializerReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion source-provenance/materializer readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later runtime materialization, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, runtime materialization state, durable computer state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A scoped durable-state-slice probe is still below runtime materialization; it can only authorize another readiness boundary that binds source provenance and materializer evidence.
        introduced: []
        repaired:
          - BaseSourceMaterializerReadinessContract now records source-provenance/materializer readiness from a scoped durable-state-slice probe without granting runtime materialization, durable-computer mutation, promotion, production, run-acceptance, full-substrate, or completion authority.
    - pass: 119
      mutation_class: red
      conjecture_delta: A durable-state-slice probe can consume durable-state-slice readiness only if it binds to a validated typed file-manifest/blob-content durable slice and preserves every downstream proof obligation without materializing runtime state or claiming completion.
      protected_surfaces:
        - durable-state-slice probe boundary
        - durable-state-slice readiness boundary
        - runtime materialization boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving durable-state-slice readiness can be consumed only with a matching typed durable-state-slice contract and no runtime, durable computer, promotion, production, run-acceptance, full-substrate, or completion authority
        - focused internal/computerversion negative tests proving readiness drift, durable-slice drift, missing probe evidence, no-mutation flag drift, runtime/durable-computer/package/promotion/run-acceptance/production mutation claims, full-substrate claims, and completion claims are rejected
        - local://pass119-base-durable-state-slice-probe-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceProbeContract -count=1
      rollback_path:
        - git revert of the local computerversion durable-state-slice probe code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later runtime materialization, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, runtime materialization state, durable computer state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Readiness for a durable-state-slice probe is not the durable-state-slice proof; it needs a separate binding to the typed file-manifest/blob-content durable slice before later materializer work can rely on it.
        introduced: []
        repaired:
          - BaseDurableStateSliceProbeContract now records the scoped durable-state-slice probe result while denying runtime materialization, durable-computer mutation, promotion, production, run-acceptance, full-substrate, and completion authority.
    - pass: 118
      mutation_class: red
      conjecture_delta: A durable-state-slice readiness boundary can consume post-settlement handoff evidence only if it preserves residual proof obligations, requires explicit durable-slice prerequisites, and denies downstream runtime, promotion, production, run-acceptance, full-substrate, and completion authority.
      protected_surfaces:
        - durable-state-slice readiness boundary
        - post-settlement handoff boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving post-settlement handoff can open only typed durable-state-slice readiness without executing runtime materialization, promotion, production mutation, or run acceptance
        - focused internal/computerversion negative tests proving invalid handoff state, missing handoff refs, missing durable-slice prerequisites, no-mutation flag drift, runtime/durable-computer/package/promotion/run-acceptance/production mutation claims, full-substrate claims, and completion claims are rejected
        - local://pass118-base-durable-state-slice-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion durable-state-slice readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later durable-state materialization, promotion execution, or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, runtime materialization state, durable computer state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A post-settlement handoff is not itself durable-state evidence; it can only authorize a narrower readiness boundary for the next typed durable state slice probe.
        introduced: []
        repaired:
          - BaseDurableStateSliceReadinessContract now records typed durable-state-slice readiness from post-settlement handoff evidence without granting runtime materialization, durable-computer mutation, promotion, production, or run-acceptance authority.
    - pass: 117
      mutation_class: red
      conjecture_delta: A post-settlement handoff boundary can consume promotion settlement only if it records residual proof obligations and the next substrate-independence probe as handoff evidence, not as promotion execution, production mutation, run acceptance, full-substrate proof, or completion.
      protected_surfaces:
        - post-settlement handoff boundary
        - promotion-settlement boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving promotion settlement can produce a blocked handoff back to substrate-independence proof work without executing promotion or synthesizing run acceptance
        - focused internal/computerversion negative tests proving settlement drift, missing residual proof refs, missing no-mutation flags, promotion/run-acceptance/production/full-substrate/completion claims are rejected
        - local://pass117-base-post-promotion-settlement-handoff-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePostPromotionSettlementHandoffReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion post-settlement handoff code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion execution or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Promotion settlement can hand control back to substrate-independence proof work only by preserving residual proof obligations, not by claiming completion.
        introduced: []
        repaired:
          - BasePostPromotionSettlementHandoffReadinessContract now records post-settlement handoff obligations without granting downstream promotion, production, or run-acceptance authority.
    - pass: 116
      mutation_class: red
      conjecture_delta: A promotion-settlement boundary can consume blocked/no-op promotion results only if it records operator settlement of the result as review evidence, not as promotion execution, production mutation, run acceptance, full-substrate proof, or completion.
      protected_surfaces:
        - promotion-settlement boundary
        - promotion-result boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving blocked/no-op promotion results can produce settlement records without executing promotion or synthesizing run acceptance
        - focused internal/computerversion negative tests proving result drift, mismatched settlement decisions, missing settlement refs, missing no-mutation flags, promotion/run-acceptance/production/full-substrate/completion claims are rejected
        - local://pass116-base-promotion-settlement-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePromotionSettlementContract -count=1
      rollback_path:
        - git revert of the local computerversion promotion-settlement code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion execution or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A blocked/no-op promotion result can be settled as reviewed evidence while still not executing promotion or granting run-acceptance authority.
        introduced: []
        repaired:
          - BasePromotionSettlementContract now records operator settlement of blocked/no-op promotion outcomes without granting downstream promotion, production, or run-acceptance authority.
    - pass: 115
      mutation_class: red
      conjecture_delta: A promotion-result boundary can consume promotion-execution readiness only if it records blocked/no-op outcome evidence separately from promotion execution, production mutation, run acceptance, full-substrate proof, and completion.
      protected_surfaces:
        - promotion-result boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving promotion-execution readiness can produce blocked/no-op promotion result records without executing promotion or synthesizing run acceptance
        - focused internal/computerversion negative tests proving promotion readiness drift, invalid outcomes, missing result refs, missing no-mutation flags, promotion/run-acceptance/production/full-substrate/completion claims are rejected
        - local://pass115-base-promotion-result-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePromotionResultContract -count=1
      rollback_path:
        - git revert of the local computerversion promotion-result code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion execution or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A blocked/no-op promotion result can be typed after promotion readiness while still not executing promotion or granting run-acceptance authority.
        introduced: []
        repaired:
          - BasePromotionResultContract now records blocked/no-op promotion outcomes without granting downstream promotion, production, or run-acceptance authority.
    - pass: 114
      mutation_class: red
      conjecture_delta: A promotion-execution readiness boundary can consume package-publication proof only if it records promotion prerequisites as readiness evidence, not as promotion execution, production mutation, run acceptance, full-substrate proof, or completion.
      protected_surfaces:
        - promotion-execution readiness boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production publication state
        - rollback boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving package-publication proof can produce promotion-execution readiness without executing promotion or synthesizing run acceptance
        - focused internal/computerversion negative tests proving publication proof drift, missing promotion readiness refs, missing no-mutation flags, promotion/run-acceptance/production/full-substrate/completion claims are rejected
        - local://pass114-base-promotion-execution-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePromotionExecutionReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion promotion-execution readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion execution or run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Promotion-execution readiness can be typed after publication proof while still not executing promotion or granting run-acceptance authority.
        introduced: []
        repaired:
          - BasePromotionExecutionReadinessContract now records promotion-execution readiness without granting downstream promotion, production, or run-acceptance authority.
    - pass: 113
      mutation_class: red
      conjecture_delta: A package-publication proof boundary can consume package-publication readiness only if it records external publication proof refs and still separates package proof from promotion execution, run acceptance, full-substrate proof, and completion.
      protected_surfaces:
        - package-publication proof boundary
        - package-publication execution boundary
        - promotion execution boundary
        - run-acceptance record boundary
        - production publication state
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving package-publication readiness can produce package-publication proof without promotion or run-acceptance authority
        - focused internal/computerversion negative tests proving readiness drift, missing proof refs, missing no-mutation flags, promotion/run-acceptance/full-substrate/completion claims, and publication-state mutation claims are rejected
        - local://pass113-base-package-publication-proof-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePackagePublicationProofContract -count=1
      rollback_path:
        - git revert of the local computerversion package-publication proof code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Package-publication proof can be typed after readiness while still not executing promotion or granting run-acceptance authority.
        introduced: []
        repaired:
          - BasePackagePublicationProofContract now records package-publication proof refs without granting downstream publication, promotion, or run-acceptance authority.
    - pass: 112
      mutation_class: red
      conjecture_delta: A package-publication readiness boundary can consume promotion/rollback review readiness only if it packages publication prerequisites as readiness evidence, not as package publication, promotion execution, run acceptance, full-substrate proof, or completion.
      protected_surfaces:
        - package-publication readiness boundary
        - package-publication execution boundary
        - promotion execution boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving promotion/rollback review readiness can produce package-publication readiness without publication or promotion authority
        - focused internal/computerversion negative tests proving promotion review drift, missing publication refs, missing no-mutation flags, package publication/promotion/run-acceptance/full-substrate/completion claims are rejected
        - local://pass112-base-package-publication-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePackagePublicationReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion package-publication readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later publication/promotion/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Package-publication readiness can be typed after promotion review while still not publishing a package or granting promotion execution authority.
        introduced: []
        repaired:
          - BasePackagePublicationReadinessContract now records package-publication readiness without granting downstream publication, promotion, or run-acceptance authority.
    - pass: 111
      mutation_class: red
      conjecture_delta: A promotion/rollback review boundary can consume owner approval only if it records promotion and rollback review readiness as prerequisites, not as promotion execution, package publication, run acceptance, full-substrate proof, or completion.
      protected_surfaces:
        - promotion/rollback review boundary
        - promotion execution boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving an approved owner-decision contract can produce promotion/rollback review readiness without downstream execution authority
        - focused internal/computerversion negative tests proving owner rejection, owner approval drift, missing promotion/rollback review refs, missing no-mutation flags, promotion/publication/run-acceptance/full-substrate/completion claims are rejected
        - local://pass111-base-promotion-rollback-review-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePromotionRollbackReviewContract -count=1
      rollback_path:
        - git revert of the local computerversion promotion/rollback review code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Promotion/rollback review readiness can be typed after owner approval while still not executing promotion or mutating ledgers.
        introduced: []
        repaired:
          - BasePromotionRollbackReviewContract now records promotion/rollback review readiness without granting downstream execution authority.
    - pass: 110
      mutation_class: red
      conjecture_delta: An owner-approval boundary can record an owner approve/reject decision only if it requires a passing verifier result plus matching owner-review packet, preserves reject as blocking evidence, and keeps promotion, package publication, run acceptance, full-substrate proof, and completion separate.
      protected_surfaces:
        - owner approval boundary
        - verifier-contract satisfaction boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a passing verifier result plus owner-review packet can produce approved and rejected owner-decision contracts without downstream execution authority
        - focused internal/computerversion negative tests proving owner-review/verifier drift, failing verifier results, invalid decisions, missing decision refs, missing no-mutation flags, promotion/publication/run-acceptance/full-substrate/completion claims are rejected
        - local://pass110-base-owner-approval-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseOwnerApprovalContract -count=1
      rollback_path:
        - git revert of the local computerversion owner-approval code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, promotion execution state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Owner approval can be typed as local review evidence while still not executing promotion, publishing packages, synthesizing run acceptance, or completing the mission.
        introduced: []
        repaired:
          - BaseOwnerApprovalContract now records approved/rejected owner decisions without granting downstream execution authority.
    - pass: 109
      mutation_class: red
      conjecture_delta: A verifier-result boundary can record verifier pass/fail outcome only if pass becomes verifier evidence without owner approval, promotion execution, package publication, run acceptance, full-substrate proof, or completion, and fail remains explicit blocking evidence.
      protected_surfaces:
        - verifier-result boundary
        - verifier-contract satisfaction boundary
        - owner approval boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving verifier readiness can produce pass and fail verifier-result contracts without downstream execution authority
        - focused internal/computerversion negative tests proving readiness drift, invalid verdicts, missing result refs, missing no-mutation flags, owner approval, promotion/publication/run-acceptance/full-substrate/completion claims are rejected
        - local://pass109-base-verifier-result-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseVerifierResultContract -count=1
      rollback_path:
        - git revert of the local computerversion verifier-result code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, owner approval state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A verifier pass is still not owner approval, promotion execution, package publication, run acceptance, full-substrate proof, or completion; a verifier fail must remain blocking evidence.
        introduced: []
        repaired:
          - BaseVerifierResultContract now records verifier pass/fail outcomes while preserving downstream authority boundaries.
    - pass: 108
      mutation_class: red
      conjecture_delta: A verifier-readiness boundary can consume owner-review readiness only if it packages verifier inputs and preserves the difference between readiness and verifier-contract satisfaction, owner approval, promotion execution, package publication, run acceptance, full-substrate proof, and completion.
      protected_surfaces:
        - verifier-readiness boundary
        - verifier-contract satisfaction boundary
        - owner approval boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving owner-review readiness can produce a verifier-readiness packet with required verifier input refs and no satisfaction/execution authority
        - focused internal/computerversion negative tests proving owner-review drift, missing verifier refs, missing no-mutation flags, verifier satisfaction, owner approval, promotion/publication/run-acceptance/full-substrate/completion claims are rejected
        - local://pass108-base-verifier-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseVerifierReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion verifier-readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later verifier/promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, owner approval state, verifier satisfaction state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Verifier readiness is not verifier satisfaction; inputs can be ready for verifier review while still forbidding approval, promotion, publication, and run acceptance.
        introduced: []
        repaired:
          - BaseVerifierReadinessContract now makes verifier input readiness explicit without granting verifier satisfaction or downstream execution authority.
    - pass: 107
      mutation_class: red
      conjecture_delta: An owner-review readiness boundary can package post-smoke handoff evidence for human/owner review only if it preserves the difference between review readiness and actual owner approval, promotion execution, package publication, verifier-contract satisfaction, run acceptance, full-substrate proof, and completion.
      protected_surfaces:
        - owner-review readiness boundary
        - owner approval boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
        - verifier-contract boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a blocked post-smoke handoff can produce an owner-review readiness packet with required evidence refs and no approval/execution authority
        - focused internal/computerversion negative tests proving handoff drift, missing evidence refs, missing no-mutation flags, owner approval, promotion/publication/run-acceptance/verifier/full-substrate/completion claims are rejected
        - local://pass107-base-owner-review-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseOwnerReviewReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion owner-review readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later owner-review/promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, owner approval state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Review readiness is not owner approval; a packet can be complete enough for review while still forbidding promotion, publication, verifier satisfaction, and run acceptance.
        introduced: []
        repaired:
          - BaseOwnerReviewReadinessContract now makes the owner-review packet boundary explicit without granting owner approval or downstream execution authority.
    - pass: 106
      mutation_class: red
      conjecture_delta: A post-smoke handoff readiness boundary can consume product-path staging smoke evidence only if it records downstream prerequisite refs and remains blocked from promotion execution, package publication, run-acceptance synthesis, full-substrate proof, and completion.
      protected_surfaces:
        - post-smoke handoff readiness boundary
        - owner review boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
        - verifier-contract boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving staging smoke evidence can produce a blocked post-smoke handoff readiness contract with explicit owner-review, promotion/rollback, publication, verifier-contract, and run-acceptance prerequisites
        - focused internal/computerversion negative tests proving smoke contract drift, failed smoke, identity mismatch flags, missing prerequisites, mutation flags, and downstream/completion claims are rejected
        - local://pass106-base-post-smoke-handoff-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBasePostSmokeHandoffReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion post-smoke handoff code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/publication/run-acceptance ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, package publication state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A passed staging smoke record still does not authorize owner approval, promotion execution, package publication, or run-acceptance synthesis by itself.
        introduced: []
        repaired:
          - BasePostSmokeHandoffReadinessContract now makes the post-smoke downstream authority handoff explicit and blocked until owner review, promotion/rollback review, package-publication proof, verifier-contract proof, and run-acceptance proof exist.
    - pass: 105
      mutation_class: red
      conjecture_delta: A staging-smoke evidence boundary can record product-path staging probe success and build/route identity only when it consumes staging-readiness evidence and preserves separate promotion, package-publication, run-acceptance, full-substrate, and completion gates.
      protected_surfaces:
        - staging smoke evidence boundary
        - staging deployment and health identity boundary
        - deployed route identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving product-path staging probe/build/route identity evidence can be recorded from a staging-readiness contract without promotion, publication, run-acceptance, full-substrate, or completion claims
        - focused internal/computerversion negative tests proving readiness drift, non-product-path probes, build identity mismatch, route identity mismatch, failed health, promotion/package/run-acceptance/full-substrate/completion claims, and manual success seeding are rejected
        - local://pass105-base-staging-smoke-evidence-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseStagingSmokeEvidenceContract -count=1
      rollback_path:
        - git revert of the local computerversion staging-smoke evidence code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later promotion/publication ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Staging-smoke evidence can record a product-path observation and build/route identity without creating a run-acceptance record or promoting the candidate.
        introduced: []
        repaired:
          - BaseStagingSmokeEvidenceContract now separates product-path staging smoke evidence from promotion, package-publication, run-acceptance, full-substrate, and completion authority.
    - pass: 104
      mutation_class: red
      conjecture_delta: A staging-readiness boundary can authorize a staging smoke probe from a bounded runtime-equivalence retry contract only if it preserves separate deployment-health, route-identity, promotion, package-publication, run-acceptance, and completion gates.
      protected_surfaces:
        - staging readiness evidence boundary
        - staging deployment and health identity boundary
        - deployed route identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving bounded runtime-equivalence retry evidence can produce staging-readiness without claiming deployed health, route identity, promotion, package publication, run acceptance, full substrate independence, or completion
        - focused internal/computerversion negative tests proving missing runtime equivalence, retry contract drift, deployment mutation, route identity claim, staging health claim, protected-surface claims, and completion claims are rejected
        - local://pass104-base-staging-readiness-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseStagingReadinessContract -count=1
      rollback_path:
        - git revert of the local computerversion staging-readiness code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later staging smoke ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Runtime equivalence evidence can be sufficient to authorize a staging smoke probe while still being insufficient to claim staging health, route identity, promotion, package publication, run acceptance, or completion.
        introduced: []
        repaired:
          - BaseStagingReadinessContract now separates staging-smoke permission from deployed health, route identity, promotion authority, package publication, run acceptance, full substrate independence, and completion.
    - pass: 103
      mutation_class: red
      conjecture_delta: A runtime equivalence retry gate can make the narrowed vmmanager path constructive only when source/provenance and typed runtime file/blob ObservationSets compare equivalent for the same ComputerVersion; mismatches must remain explicit not-equivalent evidence rather than downstream authority.
      protected_surfaces:
        - runtime equivalence retry evidence boundary
        - runtime file/blob observation extraction boundary
        - staging deployment and health identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving equivalent source/runtime file_manifest and blob_set ObservationSets produce a retry contract without downstream claims
        - focused internal/computerversion negative tests proving mismatches, unsupported/narrowed results, source/extraction drift, missing file/blob scope, protected-surface claims, and completion claims are rejected
        - local://pass103-base-runtime-equivalence-retry-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceRetryContract -count=1
      rollback_path:
        - git revert of the local computerversion runtime equivalence retry code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Typed runtime file/blob observation extraction is necessary but insufficient until compared against the source/provenance file/blob observation set.
        introduced: []
        repaired:
          - BaseRuntimeEquivalenceRetryContract now turns typed runtime file/blob extraction into a bounded runtime-equivalence claim only after source/runtime observations compare equivalent, while leaving every downstream proof gate open.
    - pass: 102
      mutation_class: red
      conjecture_delta: A runtime file/blob extraction boundary can accept a typed runtime ObservationSet only when it carries file_manifest and blob_set observations for the same ComputerVersion after a narrowed runtime-equivalence boundary, while rejecting opaque VM/data-image evidence and downstream protected-surface claims.
      protected_surfaces:
        - runtime file/blob observation extraction boundary
        - runtime equivalence evidence boundary
        - VM lifecycle evidence boundary
        - staging deployment and health identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a typed runtime ObservationSet with file_manifest and blob_set observations can be bound to the prior narrowed runtime-equivalence boundary
        - focused internal/computerversion negative tests proving vm_state_manifest-only, opaque data.img-dependent, version-drifted, boundary-drifted, protected-surface, and completion claims are rejected
        - local://pass102-base-runtime-file-blob-extraction-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseRuntimeFileBlobExtractionContract -count=1
      rollback_path:
        - git revert of the local computerversion runtime extraction boundary code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Runtime equivalence can only become constructive after typed runtime file/blob observations exist; the prior vmmanager runtime materialization/equivalence evidence remains insufficient by itself.
        introduced: []
        repaired:
          - BaseRuntimeFileBlobExtractionContract now rejects vm_state_manifest-only and opaque-data-image-dependent runtime evidence as durable file/blob proof, requiring typed file_manifest/blob_set observations before runtime equivalence can be retried.
    - pass: 101
      mutation_class: red
      conjecture_delta: A runtime-equivalence boundary can convert vmmanager-only runtime evidence into an explicit narrowed equivalence result, preventing VM metadata or opaque data.img presence from being mistaken for typed durable file/blob state equivalence.
      protected_surfaces:
        - runtime equivalence evidence boundary
        - runtime materialization evidence boundary
        - VM lifecycle evidence boundary
        - staging deployment and health identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving vmmanager-scoped runtime evidence produces a narrowed runtime-equivalence boundary when file_manifest and blob_set support are missing
        - focused internal/computerversion negative tests proving equivalent/not_equivalent results, missing unsupported file/blob capabilities, source/ceremony drift, and staging/promotion/package/run-acceptance/full-substrate/completion claims are rejected
        - local://pass101-base-runtime-equivalence-boundary-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceBoundaryContract -count=1
      rollback_path:
        - git revert of the local computerversion runtime-equivalence boundary code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - Runtime materialization evidence can be accepted while runtime equivalence remains explicitly narrowed because vmmanager does not observe typed durable file/blob state.
        introduced: []
        repaired:
          - BaseRuntimeEquivalenceBoundaryContract now rejects equivalent, not_equivalent, or malformed narrowed runtime-equivalence results and records the vmmanager gap as unsupported file_manifest/blob_set observations instead of durable-state proof.
    - pass: 100
      mutation_class: red
      conjecture_delta: A typed runtime-materialization ceremony gate can accept vmmanager-scoped Realization evidence only when it is bound to the same ComputerVersion and typed artifact-program ref as BaseSourceProvenanceReadinessContract, while preserving all downstream staging, promotion, package-publication, run-acceptance, full-substrate, and completion gates.
      protected_surfaces:
        - runtime materialization evidence boundary
        - VM lifecycle evidence boundary
        - staging deployment and health identity boundary
        - promotion/rollback boundary
        - package-publication boundary
        - run-acceptance record boundary
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the ceremony gate binds BaseSourceProvenanceReadinessContract to a Realization for the same ComputerVersion and artifact-program ref
        - focused internal/computerversion negative tests proving version drift, wrong source contract, missing refs, missing vm_state_manifest observations, unsupported capability manifests, durable file/blob runtime claims, VM lifecycle/Firecracker boot claims, staging/promotion/package/run-acceptance/full-substrate/completion claims, and production mutation claims are rejected
        - local://pass100-base-runtime-materialization-ceremony-contract-tests.jsonl
        - go test -json ./internal/computerversion -run TestBuildBaseRuntimeMaterializationCeremonyContract -count=1
      rollback_path:
        - git revert of the local computerversion runtime-materialization ceremony gate code/test/documentation changes
        - discard any candidate computer or candidate-world artifact generated by a later ceremony before promotion
        - no production canonical state, Texture canonical documents, promotion ledgers, gateway/provider routing, deployed services, auth/session state, or run-acceptance records were mutated in this pass
      heresy_delta:
        discovered:
          - A vmmanager-scoped Realization can be evidence for the runtime-materialization boundary only if a separate contract prevents it from becoming durable-state equivalence, staging proof, promotion authority, or completion.
        introduced: []
        repaired:
          - BaseRuntimeMaterializationCeremonyContract now blocks vmmanager-scoped Realization evidence from carrying durable file/blob runtime claims, unsupported capability manifests, VM lifecycle/Firecracker boot claims, staging/promotion/package-publication/run-acceptance/full-substrate claims, or completion claims.
  completed_non_red_ceremonies:
    - pass: 98
      mutation_class: yellow
      conjecture_delta: A non-runtime source/provenance readiness contract can bind the local substrate proof summary to the typed durable-state slice for the same ComputerVersion, proving file/blob source provenance is carried into the readiness boundary before any red runtime materialization ceremony is opened.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving source/provenance readiness binds local proof summary and durable-state slice contracts for the same ComputerVersion, artifact-program ref, observation scope, user provenance semantics, and refs
        - focused internal/computerversion negative tests proving version drift, artifact-program drift, wrong kinds/scopes/boundaries, missing refs, unsafe proof flags, protected-surface claims, missing provenance semantics, full-computer/data.img disposability claims, runtime/staging/promotion/package claims, and completion claims are rejected
      rollback_path:
        - git revert of the local computerversion Base source/provenance readiness code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The durable-state slice validator still accepted substrate-equivalence contracts that omitted explicit no-runtime/no-opaque flags or carried newer Firecracker/full-substrate/completion claims; durable-state validation now rejects those unsafe equivalence contracts.
        introduced: []
        repaired:
          - Local source/provenance readiness now binds the local proof summary to the typed durable-state/provenance slice for the same ComputerVersion and artifact-program ref before any red runtime materialization ceremony is considered.
    - pass: 97
      mutation_class: yellow
      conjecture_delta: A local substrate proof summary can bind the strengthened Base substrate-equivalence contract to the Base substrate-reentry-readiness contract for the same ComputerVersion, proving the local file/blob substrate-equivalence slice is summarized without converting that slice into runtime proof, staging proof, VM lifecycle proof, promotion authority, package publication, or mission completion.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the proof summary binds the substrate-equivalence and reentry-readiness contracts for the same ComputerVersion and refs
        - focused internal/computerversion negative tests proving version drift, wrong kinds/scopes/boundaries, missing refs, unsafe proof flags, protected-surface claims, missing remaining runtime/staging/promotion gaps, and completion claims are rejected
      rollback_path:
        - git revert of the local computerversion Base local substrate proof summary code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The substrate-reentry-readiness validator still accepted older substrate-equivalence contracts that omitted explicit no-runtime/no-opaque flags or carried newer Firecracker/full-substrate/completion claims; reentry now rejects those unsafe substrate contracts.
        introduced: []
        repaired:
          - A local proof summary now binds strengthened substrate-equivalence and reentry-readiness contracts while preserving runtime, staging, promotion, package publication, full-substrate, and completion gaps as required rather than silently upgrading local proof.
    - pass: 96
      mutation_class: yellow
      conjecture_delta: The scoped Base substrate-equivalence contract can be strengthened to carry the same no-runtime, no-opaque-data-img, no-Firecracker-boot, no-full-substrate, no-completion, no-mutation safety boundary as newer local contracts, without changing runtime behavior.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the substrate-equivalence contract emits explicit no runtime materialization, no opaque data.img dependency, no Firecracker boot, no full-substrate-independence, no completion, no protected-surface claims, and no-mutation flags
        - focused internal/computerversion negative tests proving the builder rejects missing no-runtime/no-opaque evidence, Firecracker boot claims, full substrate-independence claims, completion claims, protected-surface claims, and NoMutation=false
      rollback_path:
        - git revert of the local computerversion Base substrate-equivalence safety code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The older scoped substrate-equivalence contract predated later safety fields, so it could pass without explicitly denying opaque data.img dependency, Firecracker boot, full-substrate-independence, and completion claims.
        introduced: []
        repaired:
          - The scoped substrate-equivalence contract now explicitly emits and requires no-runtime, no-opaque-data-img, no-Firecracker-boot, no-full-substrate, no-completion, and no-mutation safety boundaries.
    - pass: 95
      mutation_class: yellow
      conjecture_delta: A scoped Base substrate-reentry-readiness contract can bind the prior substrate-equivalence contract to the calibrated equivalence evidence-set for the same ComputerVersion, authorizing only local substrate-equivalence reentry without claiming full substrate independence, deployed behavior, VM lifecycle, or completion.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds substrate-equivalence and calibration contracts for the same version, required file_manifest/blob_set scope, non-identical current/projection materializer identities, equivalent substrate status, equivalent calibration success, non-equivalent or narrowed calibration failure, proof refs, no completion claim, no runtime materialization, no opaque data.img dependency, and no-mutation state
        - focused internal/computerversion negative tests proving wrong contract kinds/scopes/boundaries, status/count mismatch, missing scope, version drift, missing proof refs, completion claims, runtime materialization claims, Firecracker boot claims, full substrate-independence claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base substrate-reentry-readiness code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The local substrate-equivalence contract and checker calibration existed separately, but no typed gate prevented re-entry from being mistaken for completion or deployed substrate independence.
        introduced: []
        repaired:
          - Local substrate-equivalence reentry now has a typed readiness gate that requires both prior substrate equivalence and positive/negative checker calibration while denying completion, deployed, VM, runtime, and full-substrate claims.
    - pass: 94
      mutation_class: yellow
      conjecture_delta: A scoped Base equivalence-evidence-set contract can bind one passing equivalence contract and one failure/narrowing contract for the same ComputerVersion, proving local checker calibration without claiming full substrate independence, deployed behavior, or runtime mutation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds success/failure contracts for the same version, required file_manifest/blob_set scope, equivalent success status, non-equivalent or narrowed failure status, proof refs, no runtime materialization, no opaque data.img dependency, no protected-surface claims, and no-mutation state
        - focused internal/computerversion negative tests proving wrong success/failure kinds, status/count mismatch, missing scope, version drift, missing proof refs, runtime materialization claims, Firecracker boot claims, full substrate-independence claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base equivalence-evidence-set code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - Passing and failing equivalence evidence existed as separate local contracts, but no typed calibration set required both before discussing checker trust.
        introduced: []
        repaired:
          - Base equivalence checker calibration now has a typed evidence-set boundary requiring both successful equivalence and failure/narrowing proof before trust can be named.
    - pass: 93
      mutation_class: yellow
      conjecture_delta: A scoped Base equivalence-failure-boundary contract can bind non-equivalent or narrowed EquivalenceResult evidence for two materializer contracts, proving the checker has teeth without converting failure evidence into substrate-independence, deployed product, or runtime mutation claims.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds left/right materializer-boundary contract identities, required file_manifest/blob_set scope, non-equivalent/narrowed result status, difference or unsupported-capability counts, proof refs, no successful-equivalence claim, no runtime materialization, no opaque data.img dependency for this check, and no-mutation state
        - focused internal/computerversion negative tests proving equivalent results, not_equivalent results without differences, narrowed results without unsupported capabilities, mixed result payloads, wrong materializer contracts, version drift, self-comparison, missing proof refs, successful-equivalence claims, runtime materialization claims, Firecracker boot claims, full substrate-independence claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base equivalence-failure-boundary code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - EquivalenceChecker could produce failure and narrowed outcomes, but no typed local contract prevented seeded mismatch evidence from being collapsed into successful equivalence or full substrate-independence claims.
        introduced: []
        repaired:
          - Base equivalence failure evidence now has a typed no-mutation boundary; seeded differences and unsupported-capability narrowing prove checker teeth without becoming successful equivalence, runtime, deployed, or substrate-independence authority.
    - pass: 92
      mutation_class: yellow
      conjecture_delta: A scoped Base equivalence-check-boundary contract can bind two non-identical Base materializer-boundary contracts and an equivalent EquivalenceResult for one ComputerVersion, proving pure local EquivalenceCheck authority without claiming VM lifecycle, Firecracker boot, full substrate independence, deployed routing, or runtime mutation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds left/right materializer-boundary contract kind/scope/version, non-identical materializer/substrate pairs, file_manifest/blob_set required observations, equivalent result status, proof refs, no runtime materialization, no opaque data.img dependency for this check, and no-mutation state
        - focused internal/computerversion negative tests proving wrong materializer contract kinds/scopes, version drift, identical materializer/substrate self-comparison, non-equivalent or narrowed results, missing proof refs, unsafe materializer contracts, runtime materialization claims, Firecracker boot claims, full substrate-independence claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base equivalence-check-boundary code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - EquivalenceChecker existed, but no typed local contract separated a passing equivalence result over scoped materializer contracts from full substrate-independence, Firecracker lifecycle, or deployed product claims.
        introduced: []
        repaired:
          - Base equivalence checking now has a typed boundary contract; two scoped non-identical materializer contracts plus an equivalent EquivalenceResult cannot be mistaken for full substrate independence, Firecracker lifecycle proof, deployed routing, or runtime mutation.
    - pass: 91
      mutation_class: yellow
      conjecture_delta: A scoped Base materializer-boundary contract can bind a Realization, CapabilityManifest, and file_manifest/blob_set ObservationSet for one ComputerVersion, proving local materializer shape without claiming runtime materialization, VM lifecycle, Firecracker boot, full substrate independence, deployed routing, or runtime mutation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds realization id/version, materializer/substrate names, capability manifest scope, observation-set name/version, file_manifest/blob_set required observations, proof refs, no runtime materialization, no opaque data.img dependency for this realization, and no-mutation state
        - focused internal/computerversion negative tests proving missing realization/capability/observation refs, invalid realization id/version, missing materializer/substrate names, mismatched observation version, empty or incomplete observation sets, unsupported capability manifests, runtime materialization claims, Firecracker boot claims, full substrate-independence claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base materializer-boundary code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - Projection materialization existed, but no typed local contract separated Realization shape and capability scope from VM lifecycle, Firecracker boot, full substrate independence, or deployed product claims.
        introduced: []
        repaired:
          - Base materialization now has a typed local boundary contract; a Realization/CapabilityManifest/ObservationSet tuple cannot be mistaken for runtime materialization, Firecracker boot, VM lifecycle proof, full substrate independence, deployed routing, or runtime mutation.
    - pass: 90
      mutation_class: yellow
      conjecture_delta: A scoped Base extract-boundary contract can bind an ExtractRequest, ComputerVersion artifact-program ref, and file_manifest/blob_set ObservationSet before materialization, proving typed extraction authority without claiming materialization, full-computer continuity, data.img recovery, deployed routing, or runtime mutation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds request version/name, observation-set version/name, typed artifact-program ref, extractor kind, file_manifest/blob_set required observations, proof refs, no opaque data.img dependency for extraction, and no-mutation state
        - focused internal/computerversion negative tests proving missing request/observation refs, invalid request version/name, mismatched observation version, empty or incomplete observation sets, wrong typed artifact-program ref, wrong extractor kind, opaque data.img dependency, materialization claims, full-computer continuity claims, data.img recovery claims, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion Base extract-boundary code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - Extraction existed as an interface and concrete Base helper, but no typed local contract separated extraction authority from materialization, equivalence, deployed routing, or opaque image recovery claims.
        introduced: []
        repaired:
          - Base extraction now has a typed boundary contract; a file_manifest/blob_set ObservationSet produced from a ComputerVersion artifact-program ref cannot be mistaken for materialization, full-computer continuity, data.img recovery, deployed routing, or runtime mutation.
    - pass: 89
      mutation_class: yellow
      conjecture_delta: A scoped Base durable-state-slice contract can consume the Pass 87 equivalence contract and Pass 88 user-isomorphism contract, require typed artifact-program evidence for the file_manifest/blob_set slice, and explicitly reject opaque data.img dependency, full-computer coverage, data.img disposability, and protected-surface mutation claims.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds Pass 87 equivalence kind/status/scope/ref, Pass 88 user-isomorphism kind/status/ref, Base file_manifest/blob_set observations, file_path/file_content/deletion_state/file_provenance semantics, unsupported live_process_continuity, typed artifact-program refs, durable-slice evidence refs, no opaque data.img dependency for this slice, and no-mutation state
        - focused internal/computerversion negative tests proving wrong equivalence/user-isomorphism kinds, non-equivalent status, missing scope/semantics, missing refs, unsupported live-process coverage omission, opaque data.img dependency, full-computer coverage, data.img disposability, protected-surface claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion base durable-state-slice code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The Base file_manifest/blob_set proof chain had equivalence and user-semantics contracts but no typed durable-state-slice boundary, so future readers could still collapse a local slice into a full-computer or data.img-disposability claim.
        introduced: []
        repaired:
          - Base file_manifest/blob_set evidence now has a typed durable-state-slice contract; typed artifact-program slice proof cannot be mistaken for full-computer coverage, data.img disposability, runtime mutation, deployed proof, or protected-surface authorization.
    - pass: 88
      mutation_class: yellow
      conjecture_delta: A scoped Base current-state user-isomorphism contract can consume the Pass 87 file_manifest/blob_set equivalence contract and record exactly the user-visible semantics it proves—file path, file content, deletion state, and file provenance—while explicitly marking live-process/full-computer continuity unsupported and keeping all runtime/deployed/protected surfaces unmutated.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds the Pass 87 equivalence contract kind/ref/status, Base current-state reader identity, Base file projection identity, file_manifest/blob_set observations, file_path/file_content/deletion_state/file_provenance semantics, unsupported live_process_continuity, user-isomorphic status, proof refs, and no-mutation state
        - focused internal/computerversion negative tests proving observation mismatch, unsupported capability narrowing, wrong equivalence contract kind/scope/status, missing required observations, unsafe equivalence contracts, realization version/identity drift, missing evidence refs, protected-surface claims, full-computer continuity claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion base-current-state user-isomorphism code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, Texture canonical write, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Base current-state user-isomorphism now has a typed narrow contract; file/blob semantic agreement cannot be mistaken for live-process continuity, full-computer equivalence, deployed proof, or product wiring.
    - pass: 87
      mutation_class: yellow
      conjecture_delta: A scoped Base current-state reader/file-projection equivalence proof can become typed computerversion evidence for one ComputerVersion, requiring file_manifest and blob_set observations, non-identical materializer or substrate identities, equivalent realization comparison, named observation/realization/equivalence refs, and no-mutation flags, without claiming full substrate independence or touching runtime/deployed surfaces.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the contract binds ComputerVersion, Base current-state reader identity, Base file projection identity, file_manifest/blob_set required observations, equivalence status, proof refs, and no-mutation state
        - focused internal/computerversion negative tests proving observation mismatch, unsupported capability narrowing, identical substrate/materializer self-certification, missing file_manifest or blob_set scope, ComputerVersion drift, observation version drift, empty observations, invalid realization identity, missing evidence refs/scope, unsafe claims, and NoMutation=false are rejected
      rollback_path:
        - git revert of the local computerversion base-substrate-equivalence code/test/documentation changes
        - no runtime behavior mutation, deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, package publication, gateway/provider call, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Base current-state reader/file-projection equivalence now has a typed no-mutation contract; a passing comparison cannot be mistaken for full substrate independence, deployed proof, or product wiring.
    - pass: 86
      mutation_class: yellow
      conjecture_delta: A blocked local Base route-registration readiness packet can accept read-only owner/reviewer authority-review refs, required review checklist coverage, findings, open questions, red-ceremony plan ref, and rollback ref without opening red ceremony, authorizing route registration, touching deployed service/auth/session/staging/production-state/VM/promotion/run-acceptance surfaces, or claiming product wiring.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the authority-review contract binds the blocked readiness contract identity, /api/base/ route prefix, readiness contract ref, owner/reviewer authorization refs, red ceremony plan ref, required prerequisite refs, checklist item refs, reviewer finding refs, open question refs, review report ref, rollback plan ref, blocked prerequisites, no-mutation state, and all unsafe booleans false
        - focused internal/computerversion negative tests proving invalid readiness packets, mismatched identity refs, missing review refs/checklist/findings/report, route-registration authorization claims, red-ceremony opened/approved claims, no-mutation violations, and deployed route/auth/session/staging/production-state/VM/promotion/run-acceptance claims are rejected
      rollback_path:
        - git revert of the local computerversion base-route-registration-authority-review code/test/documentation changes
        - no deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Base route-registration authority review now has a read-only evidence object; owner/reviewer attention and red-ceremony planning cannot be mistaken for red-ceremony approval or deployed route authorization.
    - pass: 85
      mutation_class: yellow
      conjecture_delta: A local Base product-path harness proof can feed a pure route-registration readiness contract that names the missing auth/session scope, deployed service registration, staging build identity, rollback route revert, and production-state boundary evidence required before any deployed route registration can be considered, while remaining blocked and no-mutation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the readiness contract binds harness/observation/comparison identity refs, /api/base/ route prefix, read/write Base scopes, readiness boundary/status, blocked prerequisites, rollback plan ref, no-mutation state, and unsafe booleans all false
        - focused internal/computerversion negative tests proving missing local harness proofs, mismatched identity refs/version, missing route/prerequisite/rollback evidence, route-registration allowed claims, no-mutation violations, and deployed route/auth/session/staging/production-state/VM/promotion/run-acceptance claims are rejected
      rollback_path:
        - git revert of the local computerversion base-route-registration-readiness code/test/documentation changes
        - no deployed route registration, production auth/session mutation, staging deployment claim, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - The Base route-registration path now has a named blocked readiness contract, so local harness success cannot be mistaken for authority to register deployed product routes.
    - pass: 84
      mutation_class: yellow
      conjecture_delta: Existing local Base harness commands can prove an explicit-path, auth-backed, read/write local product-path loop that persists Base API state and re-observes it through computerversion observation/equivalence tooling without registering deployed service routes, touching staging, mutating production auth/session, or claiming substrate completion.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused cmd/baseharness, cmd/baseobserve, and cmd/basecompare tests proving explicit journal/blob/auth paths, local route registration through a harness server, persisted observable fixture output, read-only observation emission, equivalent comparison, and seeded mismatch failure
      rollback_path:
        - git revert of the local base harness/observe/compare command test and documentation changes
        - no deployed service route registration, production auth/session mutation, staging deploy routing, persistent production state mutation, VM lifecycle mutation, promotion/rollback mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - The Base product-path proof now distinguishes local explicit-path harness verification from deployed route registration; a local harness result cannot be mistaken for staging/product route mutation.
    - pass: 83
      mutation_class: yellow
      conjecture_delta: A read-only reviewer checklist/report object can consume the blocked implementation-readiness packet and record review questions/findings without opening red ceremony, approving red ceremony, authorizing implementation, touching code, implementing an executor, publishing a package, making direct publish ready, activating product state, or claiming promotion-level acceptance.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the readiness review contract binds readiness identity, owner/reviewer authorization refs, executor design spec ref, red_ceremony_plan_ref, required_gate_refs, evidence_gate_refs, rollback_drill_ref, review_report_ref, checklist_item_refs, reviewer_finding_refs, and open_question_refs while remaining read-only and unauthorized
        - focused internal/computerversion negative tests proving malformed readiness packets, mismatched evidence refs, missing/unsupported checklist evidence, missing required checklist items, and unsafe red-ceremony/implementation/publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-executor-readiness-review code/test/documentation changes
        - no red ceremony, runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Future package-publication executor implementation now has a pure read-only reviewer checklist/report object; reviewer attention can be recorded without becoming red ceremony approval or implementation authorization.
    - pass: 82
      mutation_class: yellow
      conjecture_delta: A pure implementation-readiness contract can consume the executor design spec and name the red ceremony/evidence gates that must open before code touches package-publication executor surfaces while keeping red_ceremony_opened=false, code_surface_touched=false, implementation_ready=false, executor_implemented=false, executor_allowed=false, actual_package_published=false, direct_publish_ready=false, no_mutation=true, activation_ready=false, promotion_level_claimed=false, and all product activation surfaces blocked.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the readiness contract binds design-spec identity, required red surfaces/evidence refs, red_ceremony_plan_ref, required_gate_refs, evidence_gate_refs, and rollback_drill_ref while remaining blocked until red ceremony
        - focused internal/computerversion negative tests proving malformed design specs, mismatched evidence refs, missing/unsupported required gates, missing readiness refs, and unsafe implementation/publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-executor-implementation-readiness code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Future package-publication executor implementation now has a pure blocked-until-red-ceremony readiness contract; naming gates cannot be mistaken for opening red ceremony, touching code, or implementing the executor.
    - pass: 81
      mutation_class: yellow
      conjecture_delta: A pure executor design spec object can consume the owner/reviewer review gate and enumerate required red surfaces/evidence for any future package-publication executor while keeping executor_implemented=false, executor_allowed=false, actual_package_published=false, direct_publish_ready=false, no_mutation=true, activation_ready=false, promotion_level_claimed=false, and all product activation surfaces blocked.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the design spec binds review-gate identity, owner/reviewer authorization refs, executor_design_spec_ref, required_red_surfaces, required_evidence_refs, and rollback_plan_ref while remaining unimplemented, non-executable, and non-published
        - focused internal/computerversion negative tests proving malformed review gates, mismatched evidence refs, missing/unsupported required red surfaces, missing design/evidence/rollback refs, and unsafe executor/publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-executor-design-spec code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Future package-publication executor design now has a pure spec object that names red surfaces and evidence before any red implementation exists; the design spec cannot be mistaken for an implemented or authorized executor.
    - pass: 80
      mutation_class: yellow
      conjecture_delta: A pure owner/reviewer review gate can mark a publication preflight packet ready for future red executor design review while keeping executor_allowed=false, actual_package_published=false, direct_publish_ready=false, no_mutation=true, activation_ready=false, promotion_level_claimed=false, and all product activation surfaces blocked.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the review gate binds preflight identity, source_delta_ref, payload_manifest_ref, preflight_check_refs, verifier_contract_refs, owner_authorization_ref, and reviewer_authorization_ref while remaining non-executable and non-published
        - focused internal/computerversion negative tests proving malformed preflight contracts, mismatched evidence refs, missing owner/reviewer authorization refs, and unsafe executor/publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-executor-review-gate code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Future package-publication executor design review readiness now has a pure owner/reviewer-gated review contract; the gate cannot be mistaken for executor permission, package publication, direct publish readiness, or product activation.
    - pass: 79
      mutation_class: yellow
      conjecture_delta: A reviewable package-publication payload can feed a pure executor preflight contract that records required non-mutating checks while keeping executor_allowed=false, actual_package_published=false, direct_publish_ready=false, no_mutation=true, activation_ready=false, promotion_level_claimed=false, and all product activation surfaces blocked.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the preflight contract binds payload identity, source_delta_ref, payload_manifest_ref, preflight_check_refs, and verifier_contract_refs while remaining non-executable and non-published
        - focused internal/computerversion negative tests proving malformed payloads, mismatched identity/source/payload refs, missing preflight_check_refs, and unsafe executor/publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-preflight code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Package-publication executor readiness is now explicitly separated from preflight review evidence; preflight checks cannot be mistaken for permission to run a red publication executor.
    - pass: 78
      mutation_class: yellow
      conjecture_delta: A verifier-bound package-publication proof can be tied to explicit source-delta and payload-manifest refs as a reviewable publication payload candidate without actually publishing a package, making direct publish ready, authorizing activation, mutating AppAdoption, mutating deployed routes, touching auth/session, claiming staging, changing VM lifecycle, claiming promotion-level acceptance, or creating RunAcceptanceRecords.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the payload contract binds the publication proof identity to source_delta_ref and payload_manifest_ref while remaining actual_package_published=false, direct_publish_ready=false, no_mutation=true, activation_ready=false, and promotion_level_claimed=false
        - focused internal/computerversion negative tests proving unbound proof, mismatched identity, missing source_delta_ref/payload_manifest_ref, and unsafe publication/activation claims are rejected
      rollback_path:
        - git revert of the local non-runtime computerversion publication-payload code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Package-publication payload refs now have an inert reviewable contract shape, preventing source-delta or payload-manifest references from being mistaken for actual package publication, direct publish readiness, or product activation.
    - pass: 77
      mutation_class: yellow
      conjecture_delta: The verifier-selected package-publication prerequisite can be bound as a pure reviewable proof contract without actually publishing a package, authorizing activation, mutating AppAdoption, mutating deployed routes, touching auth/session, claiming staging, changing VM lifecycle, claiming promotion-level acceptance, or creating RunAcceptanceRecords.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving the publication proof contract binds the verifier-selected evidence ref while remaining actual_package_published=false, no_mutation=true, activation_ready=false, and promotion_level_claimed=false
        - focused internal/computerversion negative tests proving mismatched identity, non-bindable verifier state, and unsafe proof claims are rejected or remain non-authorizing
      rollback_path:
        - git revert of the local non-runtime computerversion publication-proof code/test/documentation changes
        - no runtime handler, deployed route, data migration, actual package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - Package-publication prerequisite evidence now has an inert proof-contract shape, preventing a verifier-selected publication candidate from being mistaken for actual package publication or product activation.
    - pass: 76
      mutation_class: yellow
      conjecture_delta: A pure non-runtime product-activation verifier can consume the durable activation contract and select package publication as the first safe bindable prerequisite without authorizing activation, AppAdoption mutation, deployed route mutation, auth/session change, staging acceptance, VM lifecycle mutation, promotion-level acceptance, or RunAcceptanceRecord creation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving package-publication evidence becomes the first bindable prerequisite while activation remains blocked
        - focused internal/computerversion negative tests proving missing/mismatched durable identity and unsafe first-slice protected prerequisites are rejected or narrowed to blocked
      rollback_path:
        - git revert of the local non-runtime computerversion verifier-contract code/test/documentation changes
        - no runtime handler, deployed route, data migration, package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - The next activation verifier now picks the first safe bindable prerequisite as package publication proof candidate instead of pretending local acceptance can authorize protected activation surfaces.
    - pass: 75
      mutation_class: yellow
      conjecture_delta: A pure non-runtime computerversion contract can consume the prepared owner activation decision boundary from Candidate Review without authorizing activation, package publication, AppAdoption mutation, deployed route mutation, auth/session change, staging acceptance, VM lifecycle mutation, promotion-level acceptance, or RunAcceptanceRecord creation.
      protected_surfaces:
        - none mutated
      admissible_evidence_class:
        - focused internal/computerversion unit tests proving a valid candidate package, product-path acceptance contract, and owner decision emit a blocked no-mutation durable activation contract
        - focused internal/computerversion negative tests proving mismatched package/hash/version, missing local acceptance id, activation-ready claims, promotion-level claims, and no-mutation=false are rejected
        - focused Candidate Review UI regression proving the prior product-visible decision seam still renders without unexpected failures
      rollback_path:
        - git revert of the local non-runtime computerversion activation-contract code/test/documentation changes
        - no runtime handler, deployed route, data migration, package publication, AppAdoption mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered: []
        introduced: []
        repaired:
          - The owner activation decision seam now has a pure durable contract object that persists the decision as blocked verifier input instead of promotion-level acceptance.
  completed_red_ceremonies:
    - pass: 74
      conjecture_delta: The product-visible Candidate Review surface can expose an owner-controlled activation decision boundary that prepares the next promotion decision from accepted local source-lineage evidence without publishing packages, mutating AppAdoption, mutating deployed routes, touching auth/session, claiming staging acceptance, changing VM lifecycle, or creating run-acceptance records.
      protected_surfaces:
        - Candidate Review product UI activation affordance
        - candidate-package review-surface schema and route parser
        - AppAdoption approve/promote/rollback/roll-forward mutation boundary
        - package publication and deployed route mutation boundary
        - auth/session renewal boundary
        - run-acceptance/product-acceptance terminology boundary
        - staging and VM lifecycle boundary
      admissible_evidence_class:
        - focused frontend tests proving an authenticated owner can prepare an activation decision boundary from the review surface
        - focused frontend negative tests proving the decision boundary does not call AppAdoption, candidate-package mutation, package publication, auth/session mutation, staging, VM lifecycle, or run-acceptance routes
        - focused runtime tests proving the review-surface schema carries activation-decision boundary terms without changing AppAdoption, AppChangePackage, CandidatePackageIntake, RunAcceptanceRecord, or target lineage state
      rollback_path:
        - git revert of the local activation-boundary UI/schema/test/documentation changes
        - no data migration, package publication, AppAdoption mutation, deployed route mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - Existing Features activation uses AppAdoption approve/promote and is intentionally beyond the Candidate Review local-source-lineage acceptance boundary; Candidate Review needs an explicit owner decision seam before any durable activation path can be wired.
        introduced: []
        repaired:
          - Candidate Review now renders a product-visible owner activation decision boundary from the read-only review surface, and focused tests prove preparing it only creates a local decision summary while protected mutation routes remain uncalled.
    - pass: 73
      conjecture_delta: The Candidate Review UI can be backed by a deployed-read, non-promoting runtime route that exposes only the review-surface GET and keeps candidate-package creation, owner-review mutation, publication-draft creation, source-lineage switch, rollback/roll-forward, package publication, AppAdoption mutation, auth/session changes, staging acceptance, VM lifecycle, and run-acceptance boundaries blocked.
      protected_surfaces:
        - deployed runtime route registration
        - candidate-package intake/review-surface route parser
        - candidate-package mutation endpoints
        - package publication and AppAdoption mutation boundary
        - auth/session renewal boundary
        - run-acceptance/product-acceptance terminology boundary
        - staging and VM lifecycle boundary
      admissible_evidence_class:
        - focused runtime route-registration tests proving deployed RegisterRoutes serves review-surface GET for an authenticated owner
        - focused negative tests proving deployed RegisterRoutes rejects candidate-package intake create/review/publication-draft/source-lineage switch/rollback/roll-forward/acceptance routes
        - focused store/runtime assertions proving rejected deployed routes do not mutate candidate-package, AppChangePackage, AppAdoption, RunAcceptanceRecord, auth/session, staging, or VM lifecycle state
        - focused frontend Candidate Review tests proving the UI still consumes only the review-surface GET
      rollback_path:
        - git revert of the local deployed-read route registration/code/test/documentation changes
        - no data migration, package publication, candidate deployed route mutation, auth/session change, staging deployment claim, VM lifecycle mutation, or run-acceptance record is introduced in this pass
      heresy_delta:
        discovered:
          - The UI had a review-surface path but deployed runtime registration lacked a narrow read-only candidate-package route; registering the full opt-in handler would expose mutation endpoints prematurely.
        introduced: []
        repaired:
          - Deployed RegisterRoutes now registers only the read-only candidate-package review-surface GET while focused tests prove the full candidate-package mutation/local acceptance route set stays unavailable and non-mutating through the deployed route table.
    - pass: 72
      conjecture_delta: The non-deployed candidate-package adoption/promotion review surface can be consumed by the smallest product UI/workflow without adding deployed route mutation, package publication, auth/session changes, staging claims, VM lifecycle behavior, or run-acceptance semantics.
      protected_surfaces:
        - product UI app registry and desktop launch surface
        - candidate-package review-surface route consumption
        - auth/session renewal boundary
        - deployed route registration boundary
        - run-acceptance/product-acceptance terminology boundary
        - package publication and AppAdoption mutation boundary
      admissible_evidence_class:
        - focused frontend tests proving the UI fetches only the read-only review surface for an authenticated owner-supplied intake/adoption pair
        - focused frontend tests proving missing IDs do not call the API and unauthenticated fetch failure requests auth without mutating state
        - focused frontend tests proving blocked boundaries and accepted local-source-lineage evidence are visible as review information, not activation/publish/run-acceptance controls
        - frontend build/type check plus existing focused runtime review-surface tests if route contract is touched
      rollback_path:
        - git revert of the local UI consumer code/test/documentation changes
        - no data migration, deployed route registration, package publication, or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Product-visible evidence existed as an API surface only; without a UI/workflow consumer, users could not review the accepted non-deployed candidate-package adoption boundary without raw JSON or agent narration.
        introduced: []
        repaired:
          - Candidate-package review now has a smallest product UI/workflow consumer that opens from the desktop or URL intent, fetches the read-only non-deployed review surface, exposes accepted local-source-lineage evidence, and presents boundary blocks without activation, publication, or run-acceptance controls.
    - pass: 71
      conjecture_delta: Local-source-lineage acceptance evidence can feed a product-visible but non-deployed candidate-package adoption/promotion review surface without publishing packages, mutating deployed routes, touching auth/session or VM lifecycle behavior, creating RunAcceptanceRecords, or claiming staging/deployed acceptance.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - private AppChangePackage draft/adoption-review provenance
        - local-source-lineage acceptance evidence boundary
        - runtime/API opt-in candidate-package review-surface route
        - run-acceptance/product-acceptance terminology boundary
        - deployed route registration boundary
      admissible_evidence_class:
        - focused handler tests proving a non-deployed product-visible review surface returns only after local-source-lineage acceptance evidence is accepted
        - focused negative tests proving incomplete acceptance evidence, rolled-back state, wrong-owner access, non-GET methods, and malformed paths reject without mutation
        - focused runtime/store assertions proving the route creates no RunAcceptanceRecord, package publication, deployed route mutation, auth/session mutation, provider/gateway call, Texture canonical write, VM lifecycle mutation, AppChangePackage mutation, or AppAdoption mutation
        - absence of deployed route registration and staging/production mutation
      rollback_path:
        - git revert of the local review-surface code/test/documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - A local acceptance artifact existed without a product-visible non-deployed review surface consuming it; without that boundary agents had to inspect raw evidence internals or jump prematurely to deployed promotion/run-acceptance semantics.
        introduced: []
        repaired:
          - Product-visible candidate-package adoption/promotion review now has a read-only non-deployed surface that consumes local-source-lineage acceptance evidence while keeping package publication, deployed promotion/route mutation, auth/session, staging, VM lifecycle, and RunAcceptanceRecord boundaries blocked.
    - pass: 70
      conjecture_delta: The complete local candidate-package evidence chain can be summarized as a bounded non-deployed acceptance artifact that consumes owner review, source-lineage switch, and rollback/roll-forward evidence without claiming deployed promotion-level acceptance, package publication, auth/session proof, staging proof, or VM lifecycle settlement.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - private AppChangePackage draft/adoption-review provenance
        - source-lineage switch and rollback/roll-forward verifier records
        - runtime/API opt-in candidate-package acceptance evidence route
        - run-acceptance/product-acceptance terminology boundary
      admissible_evidence_class:
        - focused handler tests proving the acceptance evidence boundary returns an accepted local-source-lineage-evidence artifact only after owner-review, switch, rollback, and roll-forward evidence are present
        - focused negative tests proving missing rollback/roll-forward evidence, rolled-back current state, and wrong-owner requests are rejected without mutation
        - focused runtime/store assertions proving the route creates no RunAcceptanceRecord, package publication, deployed route mutation, auth/session mutation, provider/gateway call, Texture canonical write, VM lifecycle mutation, or full promotion execution
        - absence of deployed route registration and staging/production mutation
      rollback_path:
        - git revert of the local acceptance evidence boundary code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Owner-review, source-lineage switch, and rollback/roll-forward evidence can exist locally, but without a bounded acceptance artifact agents may either ignore the completed local chain or overstate it as deployed promotion-level acceptance.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can summarize the completed owner-review, source-lineage switch, rollback, and roll-forward chain as local-source-lineage evidence while rejecting incomplete/rolled-back/wrong-owner states and keeping package publication, deployed route mutation, VM lifecycle behavior, auth/session, Texture, provider/gateway, and RunAcceptanceRecord boundaries untouched.
    - pass: 69
      conjecture_delta: A source-lineage-switched private candidate-package adoption review can be rolled back to its recorded previous active source ref, and a rolled-back review can be rolled forward again to its candidate source ref, through a bounded non-deployed opt-in transition without publishing packages, mutating deployed routes, touching VM lifecycle behavior, or claiming full product promotion.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - private AppChangePackage draft/adoption-review provenance
        - runtime/API opt-in promotion-switch rollback and roll-forward transition boundaries
        - runtime/store app-adoption and computer-source-lineage persistence boundaries
        - rollback profile freshness/CAS prerequisite state
        - source-lineage rollback and roll-forward verifier state
      admissible_evidence_class:
        - focused handler tests proving source-lineage-switched adoption reviews can rollback only when active lineage still equals the candidate switch ref
        - focused handler tests proving rolled-back adoption reviews can roll forward only when active lineage still equals the recorded rollback target ref
        - focused negative tests proving pending, approved-but-unswitched, stale-lineage, repeated rollback, and stale roll-forward transitions are rejected or no-op bounded
        - focused runtime/store assertions proving package publication, deployed route mutation, VM lifecycle mutation, auth/session mutation, provider/gateway calls, Texture canonical writes, and full promotion execution do not occur
        - absence of deployed route registration and staging/production mutation
      rollback_path:
        - git revert of the source-lineage rollback/roll-forward boundary code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - A source-lineage-only switch can record rollback metadata, but without a bounded rollback/roll-forward consumer the state machine can strand an active computer on a candidate source ref or force agents toward older deployed promotion semantics.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can roll back only a source-lineage-switched adoption to its recorded previous active source ref, and can roll a rolled-back adoption forward only when the active lineage still equals the recorded rollback target ref, while keeping package publication, deployed route mutation, VM lifecycle behavior, auth/session, Texture, and provider/gateway boundaries untouched.
    - pass: 68
      conjecture_delta: An owner-approved private candidate-package adoption review can perform the smallest non-deployed active-computer source-lineage switch without publishing the package, mutating deployed routes, touching VM lifecycle behavior, executing rollback, or claiming full deployed promotion.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - private AppChangePackage draft/adoption-review provenance
        - runtime/API opt-in promotion-switch transition boundary
        - runtime/store app-adoption and computer-source-lineage persistence boundaries
        - owner-approved adoption-review prerequisite enforcement
        - rollback profile and freshness/CAS prerequisite state
      admissible_evidence_class:
        - focused handler tests proving an owner-approved private candidate-package adoption review can update only the target computer source lineage through an in-process opt-in route harness
        - focused negative tests proving pending, rejected, wrong-owner, stale-lineage, missing-candidate-ref, invalid-candidate-ref, and already-switched transitions are rejected or no-op bounded
        - focused runtime/store assertions proving package publication, deployed route mutation, VM lifecycle mutation, rollback execution, auth/session mutation, provider/gateway calls, and Texture canonical writes do not occur
        - absence of deployed route registration and staging/production mutation
      rollback_path:
        - git revert of the promotion-switch boundary code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Private candidate-package adoption reviews can be owner-approved, but no bounded transition consumes that approval into a source-lineage switch without invoking the older deployed promotion path semantics.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can switch only target computer source lineage for owner-approved private candidate-package adoption reviews while keeping package publication, deployed route mutation, VM lifecycle behavior, rollback execution, auth/session, Texture, and provider/gateway boundaries untouched.
    - pass: 67
      conjecture_delta: A private candidate-package publication draft can enter a non-deployed owner adoption/review state machine without publishing the package, promoting an active computer, mutating deployed routes, touching VM lifecycle behavior, or claiming rollback execution.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - AppChangePackage draft status and visibility semantics
        - runtime/API owner adoption-review transition boundary
        - runtime/store app-package and adoption-state persistence boundaries
        - adoption/rollback contract refs as prerequisite verifier state
        - owner scope and draft provenance enforcement
      admissible_evidence_class:
        - focused handler tests proving a private candidate-package draft can enter only non-deployed owner adoption/review state through an in-process opt-in route harness
        - focused negative tests proving non-draft, wrong-owner, missing-contract, malformed-manifest, unsafe-status, deployed-route, promotion, and duplicate/unsafe transitions are rejected or no-op bounded
        - focused runtime/store assertions proving package publication, active-computer promotion, deployed route mutation, VM lifecycle mutation, and rollback execution do not occur
        - absence of deployed route, production state, auth/session, Texture, provider, or gateway mutation
      rollback_path:
        - git revert of the owner adoption-review state-machine code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - A private candidate-package AppChangePackage draft existed, but no owner adoption/review state machine consumed it without jumping to publication, promotion, or VM lifecycle semantics.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can create and resolve owner adoption-review AppAdoption records for private candidate-package publication drafts while keeping package publication, active-computer promotion, deployed route mutation, VM lifecycle behavior, and rollback execution blocked.
    - pass: 66
      conjecture_delta: An adoption-ready candidate-package intake can produce a reviewable non-published AppChangePackage draft candidate without creating an AppAdoption, publishing a package, mutating deployed routes, promoting a computer, touching VM lifecycle behavior, or claiming full product adoption.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - runtime/API publication-candidate transition boundary
        - runtime/store intake-state and app-package draft persistence boundaries
        - AppChangePackage draft status and visibility semantics
        - adoption/rollback contract refs as prerequisite verifier state
        - owner scope and adoption-ready prerequisite enforcement
      admissible_evidence_class:
        - focused handler tests proving adoption-ready intake can create a private draft through an in-process non-deployed route harness
        - focused negative tests proving pending, rejected, not-ready, wrong-owner, malformed-contract, missing-boundary, and duplicate/unsafe transitions are rejected without side effects
        - focused runtime/store assertions proving draft status is not published and no AppAdoption/promotion/deployed-route side effect occurs
        - absence of deployed route, promotion, VM lifecycle, auth/session, Texture, provider, or gateway mutation
      rollback_path:
        - git revert of the publication-consumer code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Candidate-package intake can be adoption-ready, but no consumer turns that readiness state into a reviewable publication/adoption candidate.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can turn an adoption-ready owner-approved intake into a private draft AppChangePackage candidate while keeping package publication, AppAdoption creation, promotion, deployed route mutation, and VM lifecycle behavior blocked.
    - pass: 65
      conjecture_delta: An approved candidate-package intake can bind the minimum adoption/rollback readiness state without creating an AppChangePackage, creating an AppAdoption, mutating deployed routes, promoting a computer, touching VM lifecycle behavior, or claiming full product adoption.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - runtime/API adoption-readiness transition boundary
        - runtime/store intake-state persistence boundary
        - adoption/rollback blockers as verifier state
        - owner scope and approved-review prerequisite enforcement
      admissible_evidence_class:
        - focused handler tests proving adoption/rollback binding through an in-process non-deployed route harness
        - focused negative tests proving pending, rejected, wrong-owner, invalid-contract, and already-ready transitions are rejected or no-op bounded
        - focused store tests proving adoption-ready persistence is allowed only after owner-approved review state and zero blockers
        - local command or test artifact proving no AppChangePackage/AppAdoption/promotion/deployed-route side effect
        - absence of deployed route, promotion, VM lifecycle, auth/session, Texture, provider, or gateway mutation
      rollback_path:
        - git revert of the adoption-boundary code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Candidate-package intake owner review could approve evidence, but approved intake still had no bounded adoption/rollback readiness transition.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can bind adoption and rollback contract refs for owner-approved intake records, remove the readiness blocker, preserve the intake boundary, and keep AppChangePackage publication, AppAdoption creation, promotion, deployed route mutation, and VM lifecycle behavior blocked.
    - pass: 64
      conjecture_delta: A non-deployed owner-review transition endpoint can approve or reject candidate-package intake review state while preserving the evidence-only boundary: no adoption readiness, no AppChangePackage publication, no active-computer promotion, no deployed route mutation, and no VM lifecycle behavior.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - runtime/API owner-review transition boundary
        - runtime/store intake-state persistence boundary
        - owner scope and terminal review-state enforcement
        - adoption/rollback blockers as verifier state
      admissible_evidence_class:
        - focused handler tests proving approve/reject transitions through an in-process non-deployed route harness
        - focused negative tests proving owner-scope rejection, invalid decision rejection, terminal-transition rejection, and adoption-ready remains false
        - local command or test artifact proving no AppChangePackage/AppAdoption/promotion/deployed-route side effect
        - absence of deployed route, promotion, VM lifecycle, auth/session, Texture, provider, or gateway mutation
      rollback_path:
        - git revert of the review-transition code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Candidate-package intake could be created through an opt-in API boundary, but owner review still could not transition through a product-shaped path.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can now approve or reject owner-scoped candidate-package intake review state without publishing, adopting, promoting, mutating deployed routes, or touching VM lifecycle.
    - pass: 63
      conjecture_delta: A non-deployed product/API handler boundary can create and read candidate-package intake records through the existing store while enforcing owner scope and verifier/adoption blockers, without publishing AppChangePackages, creating adoptions, promoting computers, mutating active routes, or touching VM lifecycle.
      protected_surfaces:
        - candidate-computer evidence/package intake
        - runtime/API product-path handler boundary
        - runtime/store product-path persistence boundary
        - owner scope and review-state enforcement
        - adoption/rollback blockers as verifier state
      admissible_evidence_class:
        - focused handler tests proving create/read/list behavior through an in-process non-deployed route harness
        - focused negative tests proving owner-scope rejection, unsafe adoption-ready payload rejection, and no publish/adopt/promote side effect
        - local command or test artifact proving the handler path persists pass62-shaped intake evidence
        - absence of deployed route, promotion, VM lifecycle, auth/session, Texture, provider, or gateway mutation
      rollback_path:
        - git revert of the handler-boundary code and documentation changes
        - no data migration or deployed state change is introduced in this pass
      heresy_delta:
        discovered:
          - Candidate-package intake had a store boundary but no product/API ingress, so owner review could not be exercised through a product-shaped path.
        introduced: []
        repaired:
          - A non-deployed opt-in runtime/API handler boundary can now create, list, and read owner-scoped candidate-package intake records without publishing, adopting, promoting, mutating deployed routes, or touching VM lifecycle.
  current_artifact_state:
    - This document defines the substrate-independent audited computer mission.
    - No deployed runtime behavior, VM lifecycle behavior, promotion behavior, or staging deployment behavior has changed in this checkpoint.
    - Existing Firecracker/vmmanager implementation remains the current materialization path.
    - `internal/computerversion` defines the first non-runtime contract package for `ComputerVersion`, `Extractor`, `Materializer`, `CapabilityManifest`, `ObservationSet`, `EquivalenceCheck`, and scoped `UserIsomorphism`.
    - `internal/computerversion` adapts the existing typed Choir Base tree snapshot into a file-manifest `ObservationSet`.
    - `internal/computerversion` derives that observation set from a typed Base event tape with committed positive cursor positions.
    - `internal/computerversion` materializes already-extracted observation sets into `Realization` objects under declared capabilities.
    - `internal/computerversion` verifies Base `journal.Entry` hash chains before extracting observations.
    - `internal/computerversion` extracts through the existing read-only Base `journal.Journal` interface.
    - Focused tests prove the extractor path against the existing SQLite Base journal backend.
    - `internal/computerversion` authorizes only scoped user-isomorphism claims whose required semantics are explicitly covered and whose observations are equivalent.
    - `internal/computerversion` adapts explicitly selected refs from the existing Base filesystem blob store into `ObservationBlobSet`.
    - `internal/computerversion` composes SQLite-backed Base journal/tree observations with filesystem blob-store integrity observations into one scoped current-state slice.
    - `internal/computerversion` opens existing Base journal/blob paths through a read-only current-state source boundary.
    - `internal/base/journal` exposes a read-only SQLite journal opener that does not apply schema or create a missing database.
    - `internal/base/blob` exposes an existing-root blob-store opener that does not create a missing root.
    - `internal/base/api` has a focused proof that authenticated Base API blob/item writes into a SQLite journal plus filesystem blob root can be reopened through the read-only current-state source and observed as file-manifest plus blob-set evidence.
    - `internal/base/api` has `PersistentHandlerConfig` and `OpenPersistentHandler`, a non-deployed wiring boundary that opens explicit journal/blob paths and returns Base API routes backed by persistent storage.
    - `internal/base/api` now has `RegisterPersistentRoutes`, a local route-registration helper for mounting the persistent Base route tree on an in-process registrar.
    - `internal/server.Server` now exposes `Handle(pattern, http.Handler)` so local harness tests can mount route subtrees without binding ports.
    - Route-registration red ceremony is recorded; deployed registration is intentionally not mutated in this checkpoint.
    - The desktop sync engine consumes remote Base HTTP APIs and stores local synced-state JSON; it does not own the server-side Base SQLite journal/blob root.
    - `cmd/baseharness` is a non-deployed local command that mounts persistent Base API routes from explicit journal/blob/auth DB paths.
    - `cmd/baseobserve` is a read-only local command that emits a JSON `ObservationSet` from existing Base journal/blob roots and explicit `ComputerVersion` refs without creating missing state.
    - `internal/computerversion` declares the narrow Base current-state capability manifest for file-manifest/blob-set observations.
    - Focused tests compare the extracted Base current-state slice against a non-Firecracker file-projection realization and prove a seeded projection mismatch fails equivalence.
    - `internal/computerversion` records the Base current-state reader/file-projection equivalence proof as a typed no-mutation contract for the file-manifest/blob-set slice.
    - `internal/computerversion` records the scoped Base current-state user-isomorphism proof as a typed no-mutation contract covering only file path, file content, deletion state, and file provenance while explicitly excluding live-process/full-computer continuity.
    - `cmd/basecompare` compares ObservationSet JSON as a Base current-state reader realization against a non-Firecracker file-projection realization and returns machine-readable equivalence status.
    - Focused tests prove fixture-derived persistent Base state can be observed with `cmd/baseobserve` and fed into the Base-current-state vs file-projection comparison path.
    - No safe real Base journal/blob root was found in the repo worktree; observed sqlite candidates were unrelated auth/vendor databases or test-only paths.
    - `cmd/baseharness --seed-fixture` creates a local persisted Base blob/item fixture at explicit paths, emits fixture metadata, and exits without listening.
    - Local evidence artifacts record the command chain `cmd/baseharness --seed-fixture` -> `cmd/baseobserve` -> `cmd/basecompare` with an equivalent result over two observations.
    - `specs/promotion_protocol.tla` now names the refinement seam from bounded promotion base versions to explicit `ComputerVersion` records with `codeRef` and `artifactProgramRef`.
    - `RouteNamesComputerVersion` and `PromotionNamesComputerVersion` are TLC-checked promotion invariants.
    - `internal/computerversion` now includes a scoped Firecracker/vmmanager materializer boundary that emits `vm_state_manifest` observations without invoking VM lifecycle behavior.
    - `VMManagerCapabilityManifest` supports only `vm_state_manifest` and explicitly marks file/blob/Dolt/objectgraph/provenance/live-process claims unsupported.
    - Focused tests prove identical scoped vmmanager realizations compare equivalent, a seeded VM-state mismatch fails, durable user-state claims narrow, and invalid inputs are rejected before observations are emitted.
    - `cmd/vmstateobserve` is a non-deployed local command that emits scoped `vm_state_manifest` `ObservationSet` JSON from explicit existing vmmanager fixture paths and `ComputerVersion` refs.
    - Local evidence artifacts record a fixture root with explicit persistent dir and `data.img` path observed by `cmd/vmstateobserve`.
    - `cmd/vmstatecompare` compares scoped `vm_state_manifest` `ObservationSet` artifacts under `VMManagerCapabilityManifest`, returning equivalent, not-equivalent, or narrowed evidence JSON.
    - Local evidence artifacts record an equivalent self-compare and a seeded `data.img` path mismatch through `cmd/vmstatecompare`.
    - `internal/computerversion` now includes a local `PromotionCertificate` observation wrapper over concrete active/base/candidate `ComputerVersion` refs, route slot, owner approval, health-window state, ledger states, rollback ref, and evidence ref.
    - Local evidence artifacts record a promotion certificate fixture as `promotion_certificate` ObservationSet JSON without mutating live promotion behavior.
    - `internal/computerversion` now includes `CombineObservationSets`, a fixture-level bundling boundary that merges same-`ComputerVersion` Base current-state, scoped vmmanager, and promotion-certificate observations without widening any member claim.
    - Local evidence artifacts record `cmd/baseharness --seed-fixture` -> `cmd/baseobserve`, `cmd/vmstateobserve`, and promotion certificate evidence combined under `(pass51-candidate-code, pass51-candidate-artifact)`.
    - `internal/computerversion` now includes `ProductFixtureRoot`, a non-production fixture-root observer that opens explicit Base journal/blob paths read-only, serializes scoped vmmanager state, serializes a local promotion certificate, and bundles all evidence under one `ComputerVersion`.
    - Local evidence artifacts record a deliberately provisioned product-shaped fixture root observed through `ProductFixtureRoot.ObservationSet`.
    - Local discovery found no configured Base/VM root in `BASE_API_JOURNAL_PATH`, `BASE_API_BLOB_ROOT`, `VM_STATE_DIR`, or `VMCTL_OWNERSHIP_PATH`; repo-local sqlite/data-image globbing found only auth/vendor sqlite databases, not an authorized product-shaped root.
    - `internal/computerversion` now includes `CandidateEvidenceRootManifest`, an admission contract that requires explicit candidate source, sampling authorization, no production state, no deployed-route mutation, and Base/VM paths contained under one root before `ProductFixtureRoot` can read evidence.
    - Local evidence artifacts record an authorized `local_candidate` evidence-root manifest that feeds `ProductFixtureRoot` and emits combined fixture-root observations.
    - `cmd/evidenceroot` now provisions an empty local candidate evidence root, seeds Base state through in-process persistent Base API routes, writes scoped VM fixture files, constructs an admitted `CandidateEvidenceRootManifest`, observes through `ProductFixtureRoot`, and emits self-check plus seeded mismatch JSON.
    - Local evidence artifacts record `cmd/evidenceroot` producing an admitted `local_candidate` manifest, four combined observations, an equivalent self-check, and a `not_equivalent` seeded VM-state mismatch.
    - `internal/computerversion` now includes `ObjectGraphSnapshot`, a typed non-production objectgraph slice that validates `objectgraph.Object`/`Edge` content hashes and endpoints, then emits a deterministic `object_graph_head` observation.
    - `ProductFixtureRoot` now optionally combines the typed objectgraph snapshot with Base, vmmanager, and promotion observations; nil `ObjectGraph` preserves the previous narrower fixture scope.
    - `cmd/evidenceroot` now provisions a local objectgraph snapshot for the candidate root, so the command evidence includes `blob_set`, `file_manifest`, `object_graph_head`, `promotion_certificate`, and `vm_state_manifest`.
    - `internal/computerversion` now includes `DoltHeadSnapshot`, a local non-production Dolt head observation wrapper that emits `dolt_head` and links it to the typed objectgraph head/counts without querying production corpusd.
    - `cmd/evidenceroot` now provisions an embedded local Dolt objectgraph repo under the candidate root, writes the typed objectgraph snapshot through `objectgraph.DoltStore`, commits it, queries `HASHOF('HEAD')`, and includes `dolt_head` in the candidate evidence root.
    - `cmd/vmrealize` now emits a `Realization` from explicit vmmanager fixture paths using `VMManagerScopedMaterializer` and `VMManagerCapabilityManifest`, proving the non-lifecycle Firecracker materializer boundary without booting, killing, copying, or mutating VMs.
    - `internal/computerversion` now includes `CandidateComputerPackageManifest`, the minimum reviewable bundle that binds an admitted candidate evidence root, matching evidence-root observations, and scoped realizations under one `ComputerVersion`.
    - `cmd/candidatepackage` now reads `cmd/evidenceroot` output plus one or more realization JSON files and emits a hashed `candidate_computer_package` manifest without publishing, adopting, promoting, or touching deployed routes.
    - `internal/computerversion` now includes `CandidatePackageAppChangeBridgePayload`, an evidence-only bridge shape that emits embeddable AppChangePackage manifest, verifier-contract, and provenance JSON while explicitly marking direct publication blocked without runtime/UI source deltas.
    - `cmd/candidatepackage --output bridge` now emits that bridge payload from the same evidence-root and realization inputs; it does not call product APIs, publish an AppChangePackage, adopt a package, promote, or mutate deployed routes.
    - `internal/computerversion` now includes `CandidatePackageProductPathAcceptanceContract`, a non-mutating evidence-only intake contract that requires owner review, marks adoption not ready, and blocks direct AppChangePackage publication/adoption until a product API boundary and rollback/adoption semantics exist.
    - `cmd/candidatepackage --output acceptance` emits that product-path acceptance contract from the same local evidence-root and realization inputs; it does not call runtime product APIs, create an AppChangePackage record, adopt, promote, or touch deployed routes.
    - `internal/types` now includes `CandidatePackageIntakeRecord`, the evidence-only owner-review record for candidate-computer package intake, with package hash, source refs, intake boundary, owner-review/adoption readiness state, verifier-contract JSON, evidence refs, required observations, acceptance JSON, trace id, and timestamps.
    - `internal/store` now persists candidate-package intake records through `UpsertCandidatePackageIntake`, `GetCandidatePackageIntake`, and `ListCandidatePackageIntakes` without publishing AppChangePackages, creating adoptions, promoting computers, or mutating routes.
    - `cmd/candidatepackage --output intake` emits a non-persisted `CandidatePackageIntakeRecord` payload from the same local evidence-root and realization inputs, with owner review required and adoption not ready.
    - `internal/runtime` now includes `RegisterCandidatePackageIntakeRoutes`, an opt-in non-deployed route helper plus create/list/detail handlers for candidate-package intake records.
    - Focused runtime handler tests prove authenticated create/list/detail behavior through an in-process server route table, owner-scope isolation, missing-auth rejection, owner mismatch rejection, `adoption_ready: true` rejection, and absence of AppChangePackage/AppAdoption side effects.
    - Local evidence artifacts record the handler test run as `local://pass63-candidate-package-intake-handler-tests.jsonl`.
    - `internal/runtime` now includes `ReviewCandidatePackageIntake`, a guarded owner-review transition helper that approves or rejects pending candidate-package intake records while keeping `adoption_ready: false` and preserving non-review adoption blockers.
    - The opt-in route helper now supports `POST /api/candidate-package-intakes/{intake_id}/review` only for local harnesses that mount `RegisterCandidatePackageIntakeRoutes`; deployed `RegisterRoutes` remains unchanged.
    - Focused runtime handler tests prove approve and reject review transitions through an in-process server route table, terminal-transition rejection, invalid decision rejection, missing-auth rejection, owner-scope isolation, and absence of AppChangePackage/AppAdoption side effects.
    - Local evidence artifacts record the review-transition handler test run as `local://pass64-candidate-package-review-transition-tests.jsonl`.
    - `internal/runtime` now includes `BindCandidatePackageIntakeAdoptionBoundary`, a guarded readiness transition helper that binds adoption and rollback contract refs only for owner-approved intake records, removes `adoption_rollback_boundary_not_bound`, and preserves blocking fields for direct AppChangePackage publication, AppAdoption creation, promotion, deployed route mutation, and VM lifecycle behavior.
    - The opt-in route helper now supports `POST /api/candidate-package-intakes/{intake_id}/adoption-boundary` only for local harnesses that mount `RegisterCandidatePackageIntakeRoutes`; deployed `RegisterRoutes` remains unchanged.
    - `internal/store` now permits `adoption_ready: true` only after owner-approved review state, `owner_review_required: false`, and zero adoption blockers; direct intake creation still rejects adoption-ready payloads.
    - Focused runtime and store tests prove adoption-boundary binding, pending/rejected/wrong-owner/missing-ref/already-ready rejection, adoption-ready persistence constraints, direct creation rejection, and absence of AppChangePackage/AppAdoption side effects.
    - Local evidence artifacts record the adoption-boundary handler and store test run as `local://pass65-candidate-package-adoption-boundary-tests.jsonl`.
  what_shipped:
    - docs/definitions/substrate-independent-audited-computer-2026-07-04.md
    - docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md Pass 41
    - docs/production-readiness-checklist.md eBPF/Trace production-readiness scoping
    - internal/base/api/handlers_test.go
    - internal/base/api/persistent.go
    - internal/base/api/persistent_test.go
    - internal/base/blob/store.go
    - internal/base/blob/store_test.go
    - internal/base/journal/sqlite.go
    - internal/base/journal/journal_test.go
    - internal/computerversion/types.go
    - internal/computerversion/equivalence.go
    - internal/computerversion/equivalence_test.go
    - internal/computerversion/base_tree.go
    - internal/computerversion/base_tree_test.go
    - internal/computerversion/base_event.go
    - internal/computerversion/base_event_test.go
    - internal/computerversion/base_journal.go
    - internal/computerversion/base_journal_test.go
    - internal/computerversion/base_blob.go
    - internal/computerversion/base_blob_test.go
    - internal/computerversion/base_current_state.go
    - internal/computerversion/base_current_state_test.go
    - internal/computerversion/base_current_state_loader.go
    - internal/computerversion/base_current_state_projection_test.go
    - internal/computerversion/base_current_state_projection.go
    - internal/computerversion/projection_materializer.go
    - internal/computerversion/projection_materializer_test.go
    - internal/computerversion/user_isomorphism.go
    - internal/computerversion/user_isomorphism_test.go
    - internal/computerversion/base_substrate_equivalence_contract.go
    - internal/computerversion/base_substrate_equivalence_contract_test.go
    - internal/computerversion/base_user_isomorphism_contract.go
    - internal/computerversion/base_user_isomorphism_contract_test.go
    - internal/server/server.go
    - cmd/baseharness/main.go
    - cmd/baseobserve/main.go
    - cmd/baseobserve/main_test.go
    - cmd/basecompare/main.go
    - cmd/basecompare/main_test.go
    - cmd/baseharness/main_test.go
    - specs/promotion_protocol.tla
    - specs/promotion_protocol.cfg
    - internal/computerversion/vmmanager_boundary.go
    - internal/computerversion/vmmanager_boundary_test.go
    - cmd/vmstateobserve/main.go
    - cmd/vmstateobserve/main_test.go
    - cmd/vmstatecompare/main.go
    - cmd/vmstatecompare/main_test.go
    - internal/computerversion/promotion_certificate.go
    - internal/computerversion/promotion_certificate_test.go
    - internal/computerversion/observation_bundle.go
    - internal/computerversion/observation_bundle_test.go
    - internal/computerversion/product_fixture_root.go
    - internal/computerversion/product_fixture_root_test.go
    - internal/computerversion/candidate_evidence_root.go
    - internal/computerversion/candidate_evidence_root_test.go
    - cmd/evidenceroot/main.go
    - cmd/evidenceroot/main_test.go
    - internal/computerversion/object_graph_snapshot.go
    - internal/computerversion/object_graph_snapshot_test.go
    - internal/computerversion/dolt_head_snapshot.go
    - internal/computerversion/dolt_head_snapshot_test.go
    - cmd/vmrealize/main.go
    - cmd/vmrealize/main_test.go
    - internal/computerversion/candidate_computer_package.go
    - internal/computerversion/candidate_computer_package_test.go
    - cmd/candidatepackage/main.go
    - cmd/candidatepackage/main_test.go
    - internal/computerversion/candidate_package_app_change_bridge.go
    - internal/computerversion/candidate_package_app_change_bridge_test.go
    - `cmd/candidatepackage --output acceptance` behavior in cmd/candidatepackage/main.go
    - `cmd/candidatepackage --output acceptance` tests in cmd/candidatepackage/main_test.go
    - `CandidatePackageIntakeRecord` types in internal/types/app_promotion.go
    - candidate-package intake store methods in internal/store/candidate_package_intake.go
    - candidate-package intake store tests in internal/store/candidate_package_intake_test.go
    - `cmd/candidatepackage --output intake` behavior in cmd/candidatepackage/main.go
    - `cmd/candidatepackage --output intake` tests in cmd/candidatepackage/main_test.go
    - candidate-package intake runtime methods in internal/runtime/candidate_package_intake.go
    - opt-in candidate-package intake API handlers in internal/runtime/api_candidate_package_intake.go
    - candidate-package intake handler tests in internal/runtime/candidate_package_intake_test.go
    - candidate-package intake owner-review transition behavior in internal/runtime/candidate_package_intake.go
    - candidate-package intake owner-review route behavior in internal/runtime/api_candidate_package_intake.go
    - candidate-package intake owner-review transition tests in internal/runtime/candidate_package_intake_test.go
    - candidate-package intake adoption-boundary behavior in internal/runtime/candidate_package_intake.go
    - candidate-package intake adoption-boundary route behavior in internal/runtime/api_candidate_package_intake.go
    - candidate-package intake adoption-ready store guard in internal/store/candidate_package_intake.go
    - candidate-package intake adoption-boundary and adoption-ready tests in internal/runtime/candidate_package_intake_test.go and internal/store/candidate_package_intake_test.go
  what_was_proven:
    - Owner success criterion has been translated into executable mission semantics.
    - Hypervisor/container technology is defined as substrate, not product object.
    - External reviews converged on the mission direction, next probe, and eBPF scoping.
    - `ComputerVersion = (CodeRef, ArtifactProgramRef)` is executable Go vocabulary.
    - `Extractor`, `Materializer`, `CapabilityManifest`, `ObservationSet`, `EquivalenceCheck`, and scoped `UserIsomorphism` exist in a non-runtime package.
    - Focused equivalence tests pass for one file-manifest durable slice, one seeded mismatch, unsupported capability narrowing, and mismatched `ComputerVersion`.
    - The existing typed Choir Base tree can be represented as `ObservationFileManifest` entries keyed by stable `ItemID`, with location, deletion state, version, blob, content hash, manifest, and provenance encoded as compared values.
    - Two non-identical projection fixtures over the same `ComputerVersion` pass equivalence for that Base tree slice.
    - A seeded Base tree content/version mismatch fails with a structured difference.
    - A live Base item with no linked version is rejected before it can become observation evidence.
    - A typed Base event tape can be extracted into an `ObservationSet` after deriving the Base tree by `CursorSeq`.
    - Equivalent Base event tapes with different input ordering compare equal because the committed cursor order is authoritative.
    - A seeded Base event content/version mismatch fails with a structured difference.
    - Non-committed or duplicate cursor positions are rejected before extraction.
    - A pure projection materializer can turn extracted Base observations into declared `Realization` objects.
    - Two declared projections over the same extracted Base observations pass through `Materialize` and then `EquivalenceCheck`.
    - A declared projection with unsupported capabilities is rejected before it can claim a realization.
    - Base `journal.Entry` slices are sorted by cursor, hash-chain verified, and then extracted into observations.
    - Tampered journal payloads and broken parent links are rejected before observations are derived.
    - The existing Base `journal.Journal` interface can be read through `BaseJournalExtractor`, chain-verified, extracted, and materialized.
    - A nil journal is rejected before extraction.
    - The existing SQLite Base journal backend can append entries, verify its chain, feed `BaseJournalExtractor`, and materialize a projection.
    - A scoped file-manifest user-isomorphism claim passes only after the compared observations are equivalent and the required semantics are explicitly covered.
    - Required but unclaimed durable semantics narrow the user-isomorphism claim instead of passing.
    - Explicitly unsupported required semantics narrow the user-isomorphism claim instead of passing.
    - Mismatched observations fail the user-isomorphism claim.
    - The existing Base filesystem blob store can be reopened from disk and converted into sorted `ObservationBlobSet` entries for explicitly selected refs.
    - Missing or corrupt blobs are rejected before they can become observation evidence.
    - A seeded blob-set mismatch fails equivalence.
    - A composite Base current-state slice binds SQLite journal-derived file-manifest observations to filesystem blob-store integrity observations.
    - A missing blob referenced by the current-state file manifest is rejected before the composite slice can become evidence.
    - A seeded composite current-state mismatch fails equivalence.
    - `blob.OpenStore` opens an existing blob root without creating missing state.
    - `journal.OpenSQLiteJournalReadOnly` opens an existing SQLite journal without applying schema or creating a missing database.
    - `OpenBaseCurrentStateSource` loads a composite current-state slice from existing journal/blob paths and keeps the journal handle read-only.
    - Authenticated Base API handler writes can create blob and item state in the existing Base journal/blob implementations, then that persisted state can feed `OpenBaseCurrentStateSource` and produce both file-manifest and blob-set observations.
    - `OpenPersistentHandler` validates explicit Base journal/blob paths, opens writable persistent storage, serves Base API routes, and the resulting state can be reopened read-only for current-state observations.
    - Registering persistent Base API routes in a deployed cmd service is a red mutation because it touches auth/session scope validation, staging route behavior, and writable product persistence.
    - `RegisterPersistentRoutes` can mount persistent Base routes on the shared server wrapper in process, preserve `/health`, and serve authenticated Base API requests without deployed routing.
    - `cmd/baseharness` can open a real local auth store, validate a real stored API key, mount persistent Base routes, write blob/item state through the shared server route table, and feed the read-only current-state observation source from the resulting journal/blob paths.
    - `cmd/baseobserve` can reopen the persisted Base journal/blob roots read-only, emit a JSON `ObservationSet`, and refuses missing observation roots without materializing them.
    - The existing Base current-state slice can be materialized through a Base SQLite/journal reader realization and through a non-Firecracker file-projection realization, then compared with `EquivalenceCheck`.
    - A seeded mismatch in the non-Firecracker file projection fails equivalence against the Base current-state realization.
    - The Base current-state capability manifest narrows unsupported live-process continuity instead of allowing a projection proof to imply it.
    - The Base current-state reader/file-projection equivalence proof is now recorded as a typed no-mutation contract that requires file-manifest/blob-set observations, equivalent realizations, non-identical materializer or substrate identity, and named proof refs.
    - The Base current-state user-isomorphism proof is now recorded as a typed no-mutation contract that consumes the equivalence contract and covers file path, file content, deletion state, and file provenance while rejecting live-process/full-computer continuity claims.
    - `cmd/basecompare` accepts one observed Base current-state JSON file/stream for an implicit projection comparison, accepts separate left/right JSON sets for mismatch checks, and exits non-zero for not-equivalent or invalid inputs.
    - A fixture-derived persisted Base root can be written through the persistent Base API, observed through `cmd/baseobserve`, decoded as an `ObservationSet`, and compared equivalent through `CompareBaseCurrentStateToFileProjection`.
    - A tampered right-hand file-projection JSON produces a structured `not_equivalent` result through the local comparison command.
    - `cmd/baseharness --seed-fixture` creates an explicit local journal/blob/auth fixture, writes through the persistent Base API route path, emits fixture metadata, and leaves the state observable through `OpenBaseCurrentStateSource`.
    - The command chain recorded in `local://pass45-base-commands.json` produced `local://pass45-base-observation.json` with one `blob_set` and one `file_manifest` observation, then `local://pass45-base-equivalence.json` with status `equivalent`.
    - `specs/promotion_protocol.tla` now keeps existing bounded promotion state while defining `ComputerVersionOfBase(n) = [codeRef |-> n, artifactProgramRef |-> n]` as the explicit finite-model refinement path from abstract base versions to `ComputerVersion`.
    - TLC checked `RouteNamesComputerVersion` and `PromotionNamesComputerVersion` together with the existing promotion safety/liveness properties: 826 states generated, 318 distinct states found, 0 errors.
    - `VMManagerScopedMaterializer` creates a scoped Firecracker/vmmanager realization for `vm_state_manifest` without importing or invoking vmmanager lifecycle operations.
    - `VMManagerCapabilityManifest` narrows durable file/blob/Dolt/objectgraph/provenance/live-process claims instead of allowing `data.img` or launch metadata to masquerade as user-state equivalence.
    - Focused tests prove identical scoped VM-state manifests compare equivalent, a seeded `DataImagePath` mismatch fails equivalence, unsupported durable file-manifest claims narrow, and invalid ComputerVersion/VMID/path input is rejected before evidence is emitted.
    - `cmd/vmstateobserve` requires explicit `ComputerVersion` refs plus an explicit VM ID and persistent/data path, defaults to verifying supplied state paths exist, and emits no JSON on invalid input.
    - `local://pass48-vmstate-command.json` records a successful command run over a temporary non-production fixture root; `local://pass48-vmstate-observation.json` records one `vm_state_manifest` observation requiring only `vm_state_manifest`.
    - `cmd/vmstatecompare` compares scoped vmmanager observations with capability scoping: identical fixture artifacts return `equivalent`, a seeded `vm_state_manifest` mismatch returns `not_equivalent`, and durable `file_manifest` requirements return `narrowed`.
    - `local://pass49-vmstate-compare.json` records a successful equivalent compare and a failing seeded mismatch compare over the `pass48` vmstate fixture artifact.
    - `PromotionCertificate.ObservationSet` emits candidate-scoped `promotion_certificate` observations over concrete active/base/candidate `ComputerVersion` refs, validates owner approval and ledger/health states, canonicalizes ledger order, and rejects invalid certificates before observations are emitted.
    - Focused tests prove reordered ledger input remains equivalent, seeded candidate/ledger mismatches fail equivalence, and missing approval/duplicate ledgers/unsupported states reject.
    - `local://pass50-promotion-certificate.json` records a concrete-ref promotion certificate fixture with applied source/data/index ledger states and confirmed health window.
    - `CombineObservationSets` rejects invalid or mismatched `ComputerVersion` input, merges and sorts required observation kinds, dedupes identical duplicate observations, rejects conflicting duplicate kind/key values, and preserves member observation semantics.
    - `local://pass51-combined-observation.json` records one combined fixture package with `blob_set`, `file_manifest`, `promotion_certificate`, and `vm_state_manifest` observations under one `ComputerVersion`.
    - `local://pass51-combined-equivalence.json` records `equivalent` for the same combined evidence assembled in a different input order.
    - `ProductFixtureRoot.ObservationSet` emits a combined fixture-root observation set with `blob_set`, `file_manifest`, `promotion_certificate`, and `vm_state_manifest` after opening Base state through the read-only current-state source; it rejects promotion-candidate mismatch, missing Base roots, and VM fixtures lacking both persistent dir and data image.
    - `local://pass52-fixture-root-observation.json` records a product-shaped fixture root under `(pass52-candidate-code, pass52-candidate-artifact)` with four observations and no VM lifecycle or deployed route mutation.
    - `local://pass52-fixture-root-selfcheck.json` records `equivalent` for the emitted fixture-root observation set self-check.
    - Local environment and repo-worktree discovery did not find an authorized non-production product-shaped root to sample; live/staging root sampling remains blocked rather than inferred.
    - `CandidateEvidenceRootManifest` admits only explicitly authorized local/staging candidate roots, rejects production-state and deployed-route flags, rejects evidence paths escaping `RootPath`, rejects invalid fixture/promotion semantics, and returns `ProductFixtureRoot` only after validation.
    - `local://pass54-candidate-evidence-root-manifest.json` records one authorized `local_candidate` root with Base journal/blob paths, vmmanager fixture paths, promotion certificate evidence, and all sampled paths contained under the declared root.
    - `local://pass54-candidate-evidence-root-observation.json` records the admitted root feeding `ProductFixtureRoot` into four observations under `(pass54-candidate-code, pass54-candidate-artifact)`.
    - `cmd/evidenceroot` creates a local admitted candidate evidence root end-to-end from an empty root, returns no JSON on missing required flags or non-empty root errors, preserves existing non-empty root contents, and reports an equivalent self-check plus a seeded `not_equivalent` VM-state mismatch.
    - `local://pass55-evidenceroot-manifest.json` records an admitted `local_candidate` manifest produced by the command with `authorized_for_sampling: true`, `contains_production: false`, and `touches_deployed_route: false`.
    - `local://pass55-evidenceroot-seeded-mismatch.json` records a concrete `vm_state_manifest` difference from the command's seeded mismatch.
    - `ObjectGraphSnapshot` emits a deterministic `object_graph_head` for typed `objectgraph.Object` and `objectgraph.Edge` fixtures, rejects content-hash mismatches, and rejects missing edge endpoints.
    - `ProductFixtureRoot` with nil `ObjectGraph` preserves the prior four-kind observation scope; with `ObjectGraph` it includes `object_graph_head` with typed object/edge counts.
    - `cmd/evidenceroot` now emits a five-kind candidate evidence root, and the command tests verify the manifest's objectgraph snapshot aligns with the `object_graph_head` payload.
    - `local://pass56-objectgraph-observation.json` records `object_graph_head` with two typed objects, one edge, and deterministic head `sha256:d0c05a3a98ed6619e7b7dfb2a4f6ceae19205afaa1295440d9be48f4936999f1`.
    - `local://pass56-objectgraph-manifest.json` records the embedded non-production objectgraph snapshot inside an admitted `local_candidate` evidence-root manifest.
    - `DoltHeadSnapshot` emits a `dolt_head` observation with database, commit hash, linked objectgraph head, object count, and edge count; it rejects missing commit hashes and `contains_production: true`.
    - `ProductFixtureRoot` with `DoltHead` includes `dolt_head`; without optional slices it preserves the narrower Base + VM + promotion scope.
    - `cmd/evidenceroot` now emits a six-kind candidate evidence root and tests verify the embedded Dolt repo path stays under the candidate root, has database `objectgraph`, has a non-empty commit hash, and links to the same objectgraph head/counts.
    - `local://pass57-dolthead-observation.json` records `dolt_head` with commit hash `ugohvt2etcuvnpvr3enlb023larbnm59` and linked objectgraph head `sha256:f07b83df2304baeef2067a623ef758583a46b5d655c104611ecabb64863f736e`.
    - `local://pass57-dolthead-manifest.json` records an admitted `local_candidate` manifest whose `dolt_head.repo_root` is under the candidate fixture root and whose `contains_production` flag is false.
    - `cmd/vmrealize` emits a `Realization` with `firecracker/vmmanager` capability manifest, supported `vm_state_manifest`, unsupported file/blob/Dolt/objectgraph/provenance/live-process claims, and one `vm_state_manifest` observation from explicit local fixture paths.
    - `cmd/vmrealize` rejects missing required flags before success JSON, rejects missing paths when `--require-existing=true`, and with `--require-existing=false` can classify declared paths without creating them.
    - `local://pass58-vmrealize-realization.json` records realization `pass58-vm-realization` over substrate `firecracker/vmmanager`, materializer `pass58-firecracker-scoped`, supported `vm_state_manifest`, six unsupported capabilities, and version `(pass58-code, pass58-artifact)`.
    - `CandidateComputerPackageManifest` validates that a package is non-production, route-inert, tied to one `ComputerVersion`, backed by an admitted evidence root, and bundled with realizations whose capability manifests support their required observations.
    - Focused tests prove package hashing is stable, required observation kinds are canonicalized, production/deployed-route flags are rejected, version mismatches fail, missing required observations fail, and unsupported realization claims fail validation.
    - `cmd/candidatepackage` accepts an evidence-root command output with extra top-level fields plus a strict realization JSON file, emits a hashed package manifest, rejects missing evidence-root inputs, rejects realization version mismatches, and rejects unknown realization fields.
    - `local://pass59-candidatepackage-manifest.json` records `candidate_computer_package` `pass59-candidate-package` with package hash `sha256:e320dce27f61370d8dbbdc65efef2698be0bbae22feec5dcf1da7d9191e0f69d`, six required observation classes, one scoped vmmanager realization, `contains_production: false`, and `touches_deployed_route: false`.
    - `CandidatePackageAppChangeBridgePayload` validates a hashed candidate-computer package, emits AppChangePackage-compatible manifest/provenance/verifier-contract JSON, and preserves `direct_publish_ready: false` with explicit source-delta blockers.
    - Focused tests prove the bridge payload contains package hash, source refs, required observations, recipient-build marker, review contracts, provenance counts/refs, and rejects missing package hashes or invalid package content.
    - `cmd/candidatepackage --output bridge` emits bridge JSON and rejects invalid output modes before success JSON.
    - `local://pass60-appchange-bridge-payload.json` records bridge `candidate_package_app_change_bridge` over `pass60-candidate-bridge` with `direct_publish_ready: false`, two direct-publish blockers, and six required observation classes.
    - `CandidatePackageProductPathAcceptanceContract` validates a candidate-computer package plus bridge, selects `candidate_package_evidence_only_intake` as the only admissible product boundary today, requires owner review, and keeps `adoption_ready: false`.
    - Focused tests prove the acceptance contract emits passed package/evidence/required-observation contracts, pending evidence-only intake, blocked direct publish/adoption contracts, canonical refs, and rejects mismatched/unsafe bridges or invalid package content.
    - `cmd/candidatepackage --output acceptance` emits acceptance JSON from the local package inputs and keeps adoption/publish/promotion behavior out of the command.
    - `local://pass61-product-path-acceptance.json` records `candidate_package_product_path_acceptance` over `pass61-product-path-acceptance` with `candidate_package_evidence_only_intake`, owner review required, `adoption_ready: false`, four adoption blockers, three passed verifier contracts, one pending intake contract, and two blocked publish/adoption contracts.
    - `CandidatePackageIntakeRecord` can persist the evidence-only owner-review boundary with package hash, source refs, owner review state, adoption blockers, verifier-contract JSON, evidence refs, required observations, and the original acceptance JSON.
    - Focused store tests prove insert/get/list round trips preserve the intake record, updates keep the same intake ID while replacing review evidence, and unsafe or incomplete records are rejected before persistence.
    - `cmd/candidatepackage --output intake` emits a review-pending intake payload from the local package evidence, requires `--owner-id`, and still does not publish, adopt, promote, or touch deployed routes.
    - `local://pass62-candidate-package-intake.json` records intake `pass62-candidate-package-intake` with status `owner_review_pending`, owner review state `required`, `adoption_ready: false`, intake boundary `candidate_package_evidence_only_intake`, and the four adoption blockers from the pass61 acceptance contract.
    - `RegisterCandidatePackageIntakeRoutes` mounts candidate-package intake routes only when a local harness opts in; deployed `RegisterRoutes` remains unchanged.
    - The opt-in handlers create, list, and read `CandidatePackageIntakeRecord` values for the authenticated owner, reject mismatched body owner IDs, reject adoption-ready intake payloads, and do not create AppChangePackages or AppAdoptions.
    - `local://pass63-candidate-package-intake-handler-tests.jsonl` records passing runtime handler tests for the non-deployed API boundary.
    - The opt-in review transition route approves or rejects pending `CandidatePackageIntakeRecord` values for the authenticated owner, preserves non-review blockers, keeps `adoption_ready: false`, rejects terminal re-review, and still creates no AppChangePackage or AppAdoption side effects.
    - `local://pass64-candidate-package-review-transition-tests.jsonl` records passing runtime handler tests for the non-deployed owner-review transition boundary.
    - The opt-in adoption-boundary transition route binds adoption and rollback contract refs only after owner approval, removes the final readiness blocker when no other blockers remain, marks `adoption_ready: true`, rejects pending/rejected/wrong-owner/missing-ref/already-ready transitions, and still creates no AppChangePackage or AppAdoption side effects.
    - Store-level adoption-ready persistence is now guarded by owner-approved review state, `owner_review_required: false`, and zero adoption blockers, while the direct intake creation path rejects all adoption-ready payloads.
    - `local://pass65-candidate-package-adoption-boundary-tests.jsonl` records passing runtime and store tests for the non-deployed adoption/rollback readiness boundary.
    - The opt-in publication-draft transition route creates a private draft `AppChangePackageRecord` only from an owner-approved, adoption-ready intake with a bound adoption/rollback envelope and explicit publication contract ref; the draft manifest records publication, adoption, rollback, direct-publish, AppAdoption, promotion, deployed-route, and VM-lifecycle boundaries.
    - Publication-draft creation rejects pending, rejected, not-ready, wrong-owner, missing-publication-contract, missing-boundary, and unrelated-package-collision transitions; successful draft creation still creates no `AppAdoption` and does not publish or promote.
    - `local://pass66-candidate-package-publication-draft-tests.jsonl` records passing runtime and store tests for the non-deployed publication-draft consumer boundary.
  unproven_or_partial_claims:
    - No concrete Firecracker lifecycle materializer has been implemented; the current vmmanager boundary is a scoped manifest/classification materializer only.
    - No current production `data.img` state has been extracted into the new observation contract.
    - No live production Base, Dolt, or objectgraph state has been sampled into this contract.
    - No cmd service currently calls `OpenPersistentHandler` with deployed configuration.
    - The Base API proofs use auth test doubles and in-process handlers; they do not prove deployed auth/session, routing, or production persistence.
    - `cmd/baseharness` is a local harness only; it does not prove deployed service configuration, public routing, or staging auth/session behavior.
    - `cmd/baseobserve` proves extraction/report mechanics only for explicit local paths; it does not prove that a production root has been classified safe to read or sampled.
    - UserIsomorphism is implemented only for scoped claims over declared observation semantics; it is not a full-computer equivalence proof.
    - No non-Firecracker materializer/projection proof exists yet for live product state; current proofs use focused persisted fixtures and local ObservationSet JSON.
    - `data.img` is not yet disposable for full user state.
    - No deployed runtime/product API route, AppChangePackage publication path, adoption flow, run-acceptance path, or promotion path consumes `CandidatePackageIntakeRecord`, `CandidateComputerPackageManifest`, `CandidatePackageAppChangeBridgePayload`, or `CandidatePackageProductPathAcceptanceContract` yet; the new consumers create only private draft package/adoption-review state and non-deployed source-lineage switch state through opt-in local harness routes.
  belief_state_changes:
    - The mission identity is substrate-independent audited computers, not Cloud Hypervisor migration.
    - The first useful proof should compare durable ledgers/observations, not byte-identical ext4 images.
    - The first implementation step has been satisfied as a small non-runtime contract package with a real checker and mismatch fixture.
    - The next implementation step has been satisfied for a current-state-shaped typed Base tree slice, but only with fixtures.
    - `ArtifactProgramRef` now has a concrete first interpretation for this mission: a typed Base event tape cursor can extract a file-manifest observation set.
    - `Materialize` now has a first non-runtime interpretation: a declared projection can produce a scoped `Realization` from extracted observations, but this is not yet a VM/substrate boundary.
    - Hash-chain-aware extraction moves the first slice from unordered event fixtures toward tamper-evident typed tape semantics.
    - Journal-interface extraction moves the proof from manually supplied entries toward existing Base persistence boundaries.
    - SQLite journal proof shows the typed tape path can use the existing persistent Base journal implementation, but still only with focused test data.
    - UserIsomorphism now names the claim boundary above observation equivalence: file-manifest semantics can pass; live-process continuity or other unmodeled durable semantics cannot pass by implication.
    - Base blob-store observation is a lower-blast-radius durable slice than VM/image inspection: it proves content-addressed blob integrity without touching VM lifecycle or production state.
    - Composite Base current-state observation ties event provenance to blob integrity for one scoped slice before any live production sampling.
    - Read-only current-state loading is now a package boundary; the remaining gap is product configuration/wiring, not observation mechanics.
    - Base API handler state can now feed the observation contract in a focused proof; the remaining gap is service registration/configuration, not handler semantics.
    - Persistent Base API wiring can now be expressed as explicit journal/blob path configuration without changing any deployed cmd service.
    - Product route registration is explicitly classified as a red boundary rather than silently blended into yellow/orange local proof work.
    - A local route harness can exercise the same shared server route table shape without mutating staging-facing services.
    - The first concrete local command boundary now exists for manual Base API observation extraction from explicit paths, reducing the gap from route mechanics to product configuration policy.
    - Manual observation extraction now has a read-only command boundary, so the next realism step can compare projection/materializer behavior instead of adding more route plumbing.
    - The first non-Firecracker projection proof over the Base current-state observation slice exists, but remains scoped to focused test data and file-manifest/blob-set observations.
    - The first Base current-state equivalence contract exists, so equivalent file/blob observations now have a typed proof boundary rather than only a raw checker result.
    - The first Base current-state user-isomorphism contract exists, so user-visible file/blob semantics can be claimed narrowly without implying live-process continuity or full-computer substrate independence.
    - The first local observe-to-compare command path exists: persisted fixture root -> read-only ObservationSet JSON -> Base-current-state/file-projection equivalence result.
    - The first durable command-chain evidence now exists outside unit-test stdout, but it remains a local fixture artifact rather than a sampled live product root.
    - Promotion protocol refinement no longer depends on implicit numeric bases only; the formal model now exposes the ComputerVersion seam without changing runtime promotion behavior.
    - Firecracker/vmmanager now has a first scoped materializer/capability boundary for VM-state classification, but not a lifecycle materializer or durable user-state proof.
    - The scoped vmmanager boundary is now executable through a local command over explicit fixture paths, reducing the gap from interface-only classification to reproducible observation artifact.
    - Scoped vmmanager evidence now has a command-level compare path with both equivalent and seeded-mismatch outcomes, but it remains limited to VM-state manifests.
    - Promotion evidence now has a local concrete-ref observation fixture, but no deployed promotion certificate, route record, or rollback path consumes it yet.
    - A fixture-level combined observation package now relates typed Base current-state, scoped vmmanager state, and promotion certificate evidence under one `ComputerVersion`, but it remains local fixture evidence rather than a live product or deployed route proof.
    - A deliberately provisioned non-production product-shaped fixture root can now be observed through one package API; this converts the previous next probe from a doc-level contract into executable local evidence, but still only for fixture state.
    - Discovery of an authorized non-production product-shaped root failed locally, so the next safe move is an explicit candidate-computer evidence-root provisioning contract rather than opportunistic production/staging reads.
    - Candidate evidence-root admission is now executable, so the remaining gap is provisioning or finding an authorized candidate root with real non-production state rather than defining the root contract itself.
    - Candidate evidence-root provisioning now has a local command boundary with positive and seeded-failure evidence; remaining live-product uncertainty is about richer non-production candidate data and lifecycle/materializer semantics, not root admission mechanics.
    - A typed objectgraph state slice is now in the candidate evidence root; it is still an embedded fixture snapshot, not a live Dolt/corpusd head or production objectgraph read.
    - A local embedded Dolt objectgraph commit head is now in the candidate evidence root; this proves fixture-level Dolt/objectgraph head capture, not live corpusd/platform head equivalence.
    - A non-lifecycle Firecracker/vmmanager realization proof now exists as a local command boundary; it still does not boot or resume a VM and still classifies `data.img`/persistent dirs as durable legacy opaque state.
    - A candidate-computer package manifest can now reviewably bundle the admitted evidence root and scoped vmmanager realization outputs under one hash, but remains a local package artifact rather than a product-transfer object.
    - The existing AppChangePackage path can receive candidate-computer evidence as embedded manifest/provenance/verifier JSON, but direct publication remains correctly blocked because the current product path requires runtime/UI source deltas.
    - The candidate-computer evidence product boundary is now named as evidence-only intake plus owner review; direct AppChangePackage publication and product adoption remain blocked until a runtime/UI source-delta or adoption-consumer path exists.
    - Candidate-package evidence-only intake is now a store record and command payload, so the next unknown is not whether the data can be persisted; it is which product/API surface is allowed to create, review, and transition that intake.
    - Candidate-package intake now has a non-deployed product/API handler boundary for create/list/detail owner-review ingress and owner-review transitions without deployed route mutation.
    - Candidate-package intake adoption/rollback readiness can now feed the smallest non-deployed consumer: an opt-in publication-draft route that creates a private draft AppChangePackage candidate while keeping direct publication, AppAdoption creation, promotion, deployed route mutation, and VM lifecycle behavior blocked.
    - Candidate-package publication drafts now have a non-deployed owner adoption/review state machine: an opt-in runtime/API handler can create `owner_review_pending` AppAdoption records from private draft packages and resolve them to `owner_review_approved` or `owner_review_rejected` without publishing, promoting, mutating deployed routes, executing rollback, or touching VM lifecycle behavior.
    - Owner-approved private candidate-package adoption reviews now have the smallest non-deployed active-computer switch consumer: an opt-in runtime/API handler can update the target computer source lineage to the candidate source ref, bind the adoption/package/candidate provenance, and mark the adoption `source_lineage_switched` without publishing the package, mutating deployed routes, executing rollback, touching VM lifecycle behavior, or claiming full product promotion.
    - Source-lineage-switched private candidate-package adoption reviews now have a non-deployed rollback/roll-forward state machine: opt-in runtime/API handlers can restore the recorded previous active source ref only when the current lineage still equals the candidate switch ref, and can roll forward again only when the current lineage still equals the recorded rollback target ref, while keeping package publication, deployed route mutation, full promotion execution, and VM lifecycle behavior blocked.
    - The completed local candidate-package evidence chain now has a bounded non-deployed acceptance artifact: an opt-in runtime/API handler returns local-source-lineage evidence only after owner review, source-lineage switch, rollback, and roll-forward checkpoints are verified, rejects incomplete/rolled-back/wrong-owner states, and keeps package publication, deployed route mutation, promotion-level acceptance, RunAcceptanceRecord creation, auth/session, staging, VM lifecycle, Texture, and provider/gateway boundaries blocked or unproven.
    - Product-visible candidate-package adoption/promotion review now has a read-only non-deployed surface: an opt-in runtime/API handler returns a reviewable surface only after accepted local-source-lineage evidence, embeds that evidence, allows only review/inspect actions, and keeps package publication, deployed promotion/route mutation, auth/session, staging, VM lifecycle, RunAcceptanceRecord creation, AppChangePackage mutation, and AppAdoption mutation blocked.
    - Candidate-package review now has a smallest product UI/workflow consumer: the `candidate-review` desktop app can open from Desk or a URL intent, fetch only the read-only review-surface GET for an intake/adoption pair, render accepted local-source-lineage evidence and blocked boundaries, and request auth rather than touching protected APIs while signed out.
    - Owner clarified that this mission is a refinement/rephrasing of the autoputer goal, not a competing sibling mission.
    - Owner clarified the correct order: table autopaper, get autoputer working correctly first, then resume autopaper.
    - External reviews converged that Passes 15-23 are substrate-symptom evidence, not substrate-independent progress.
    - External reviews converged that eBPF is a future optional observation source/capability, not the next probe.
  remaining_error_field:
    - First live production observation slice is still unsettled: the current-state proof uses production backend implementations with focused test data and in-process handlers, not a deployed user's state.
    - Runtime/product configuration does not yet call the persistent Base API wiring boundary.
    - Persistent/ephemeral/cache classification is incomplete for current images; only the scoped vmmanager manifest now classifies `data.img` and persistent dirs as legacy opaque durable state for one non-lifecycle boundary.
    - Passes 15-23 are a substrate-symptom cluster on opaque `data.img`; the ext4 repair was a protected-data rescue, not audited-computer recovery.
    - eBPF/logging/monitoring belong as optional capability-scoped observation sources feeding Trace after PII retraction, not as completion criteria or equivalence proof.
    - Promotion spec refinement has named `ComputerVersion`, and a local promotion-certificate observation fixture exists, but no deployed route record or rollback path is keyed by real CodeRef/ArtifactProgramRef values yet.
    - Firecracker/vmmanager lifecycle behavior remains outside the materializer contract; the new boundary is classification/observation only.
    - Candidate-package intake exists as a type, store record, local command payload, non-deployed opt-in API handler boundary, owner-review transition endpoint, adoption/rollback readiness endpoint, private publication-draft consumer, private draft owner adoption/review state machine, source-lineage switch consumer, source-lineage rollback/roll-forward state machine, local-source-lineage acceptance evidence boundary, product-visible non-deployed review surface, and smallest product UI/workflow consumer, but no deployed backend route exposes the candidate-package intake handler and no staging proof consumes the UI against live product data yet.
  highest_impact_remaining_uncertainty: Whether the candidate-package review UI can be backed by a deployed but still non-promoting product route without crossing package publication, deployed route mutation for candidates, auth/session changes, staging acceptance, VM lifecycle, or run-acceptance boundaries prematurely.
  next_executable_probe: Define and prove the smallest deployed-read/non-promoting route exposure for the candidate-package review UI while keeping package publication, candidate deployed route mutation, auth/session changes, staging acceptance, VM lifecycle, and run-acceptance boundaries blocked.
  suggested_goal_string: "/goal docs/definitions/substrate-independent-audited-computer-2026-07-04.md"
  evidence_artifact_refs:
    - local://pass45-base-commands.json
    - local://pass45-base-observation.json
    - local://pass45-base-equivalence.json
    - TLC: `/nix/var/nix/profiles/default/bin/nix shell nixpkgs#temurin-jre-bin --command java -XX:+UseParallelGC -cp /tmp/tla2tools.jar tlc2.TLC -deadlock -workers auto promotion_protocol.tla` in `specs/`
    - `go test ./internal/computerversion`
    - local://pass48-vmstate-command.json
    - local://pass48-vmstate-observation.json
    - local://pass49-vmstate-compare.json
    - local://pass50-promotion-command.json
    - local://pass50-promotion-certificate.json
    - local://pass51-combined-command.json
    - local://pass51-combined-observation.json
    - local://pass51-combined-equivalence.json
    - local://pass52-fixture-root-command.json
    - local://pass52-fixture-root-observation.json
    - local://pass52-fixture-root-selfcheck.json
    - local://pass54-candidate-evidence-root-command.json
    - local://pass54-candidate-evidence-root-manifest.json
    - local://pass54-candidate-evidence-root-observation.json
    - local://pass54-candidate-evidence-root-selfcheck.json
    - local://pass55-evidenceroot-command.json
    - local://pass55-evidenceroot-manifest.json
    - local://pass55-evidenceroot-observation.json
    - local://pass55-evidenceroot-selfcheck.json
    - local://pass55-evidenceroot-seeded-mismatch.json
    - local://pass56-objectgraph-command.json
    - local://pass56-objectgraph-manifest.json
    - local://pass56-objectgraph-observation.json
    - local://pass56-objectgraph-selfcheck.json
    - local://pass56-objectgraph-seeded-mismatch.json
    - local://pass57-dolthead-command.json
    - local://pass57-dolthead-manifest.json
    - local://pass57-dolthead-observation.json
    - local://pass57-dolthead-selfcheck.json
    - local://pass57-dolthead-seeded-mismatch.json
    - local://pass58-vmrealize-command.json
    - local://pass58-vmrealize-realization.json
    - local://pass59-evidenceroot-output.json
    - local://pass59-vmrealize-realization.json
    - local://pass59-candidatepackage-command.json
    - local://pass59-candidatepackage-manifest.json
    - local://pass60-appchange-bridge-command.json
    - local://pass60-appchange-bridge-payload.json
    - local://pass61-product-path-acceptance-command.json
    - local://pass61-product-path-acceptance.json
    - local://pass62-candidate-package-intake-command.json
    - local://pass62-candidate-package-intake.json
    - local://pass63-candidate-package-intake-handler-tests.jsonl
    - local://pass64-candidate-package-review-transition-tests.jsonl
    - local://pass65-candidate-package-adoption-boundary-tests.jsonl
    - local://pass66-candidate-package-publication-draft-tests.jsonl
    - local://pass67-candidate-package-adoption-review-tests.jsonl
    - local://pass68-candidate-package-promotion-switch-tests.jsonl
    - local://pass69-candidate-package-promotion-switch-rollback-tests.jsonl
    - local://pass70-candidate-package-acceptance-evidence-tests.jsonl
    - local://pass71-candidate-package-promotion-review-surface-tests.jsonl
    - local://pass72-candidate-review-ui-tests.json
    - local://pass73-deployed-route-tests.jsonl
    - local://pass73-candidate-review-ui-tests.json
    - `go test -json ./internal/runtime -run 'TestCandidatePackageIntake(DeployedRegisterRoutesServesOnlyReviewSurface|PromotionSwitchReviewSurfaceRouteReturnsReadOnlyProductSurface)$' -count=1 -parallel=1`
    - local://pass74-activation-boundary-runtime-tests.jsonl
    - local://pass74-activation-boundary-ui-tests.json
    - `go test -json ./internal/runtime -run 'TestCandidatePackageIntake(PromotionSwitchReviewSurfaceRouteReturnsReadOnlyProductSurface|DeployedRegisterRoutesServesOnlyReviewSurface)$' -count=1 -parallel=1`
    - `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
    - `pnpm --dir frontend build`
    - `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
    - `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
    - `pnpm --dir frontend build`
    - `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch -parallel=1`
    - `scripts/doccheck report-only`
    - local://pass75-durable-activation-contract-tests.jsonl
    - local://pass75-candidate-review-ui-regression.json
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackageDurableActivationContract -parallel=1`
    - `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
    - local://pass76-product-activation-verifier-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackageProductActivationVerifierContract -parallel=1`
    - local://pass77-publication-proof-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationProofContract -parallel=1`
    - local://pass78-publication-payload-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationPayloadContract -parallel=1`
    - local://pass79-publication-preflight-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationPreflightContract -parallel=1`
    - local://pass80-publication-review-gate-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorReviewGateContract -parallel=1`
    - local://pass81-publication-executor-design-spec-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorDesignSpecContract -parallel=1`
    - local://pass82-publication-implementation-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorImplementationReadinessContract -parallel=1`
    - local://pass83-publication-readiness-review-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorReadinessReviewContract -parallel=1`
    - local://pass84-base-product-path-harness-tests.jsonl
    - `go test -json ./cmd/baseharness ./cmd/baseobserve ./cmd/basecompare -parallel=1`
    - local://pass85-base-route-registration-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRouteRegistrationReadinessContract -parallel=1`
    - local://pass86-base-route-registration-authority-review-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRouteRegistrationAuthorityReviewContract -parallel=1`
    - local://pass87-base-substrate-equivalence-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseSubstrateEquivalenceContract -parallel=1`
    - local://pass88-base-user-isomorphism-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseCurrentStateUserIsomorphismContract -parallel=1`
    - local://pass89-base-durable-state-slice-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceContract -parallel=1`
    - local://pass90-base-extract-boundary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseExtractBoundaryContract -parallel=1`
    - local://pass91-base-materializer-boundary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseMaterializerBoundaryContract -parallel=1`
    - local://pass92-base-equivalence-check-boundary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceCheckBoundaryContract -parallel=1`
    - local://pass93-base-equivalence-failure-boundary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceFailureBoundaryContract -parallel=1`
    - local://pass94-base-equivalence-evidence-set-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceEvidenceSetContract -parallel=1`
    - local://pass95-base-substrate-reentry-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseSubstrateReentryReadinessContract -parallel=1`
    - local://pass96-base-substrate-equivalence-strengthened-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseSubstrateEquivalenceContract -parallel=1`
    - local://pass97-base-local-substrate-proof-summary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run 'TestBuildBaseLocalSubstrateProofSummaryContract|TestBuildBaseSubstrateReentryReadinessContract' -parallel=1`
    - local://pass98-base-source-provenance-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run 'TestBuildBaseSourceProvenanceReadinessContract|TestBuildBaseDurableStateSliceContract' -parallel=1`
    - `scripts/doccheck report-only` (Pass 99 ceremony-opening documentation check)
    - local://pass100-base-runtime-materialization-ceremony-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeMaterializationCeremonyContract -count=1`
    - local://pass101-base-runtime-equivalence-boundary-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceBoundaryContract -count=1`
    - local://pass102-base-runtime-file-blob-extraction-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeFileBlobExtractionContract -count=1`
    - local://pass103-base-runtime-equivalence-retry-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceRetryContract -count=1`
    - local://pass104-base-staging-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseStagingReadinessContract -count=1`
    - local://pass105-base-staging-smoke-evidence-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseStagingSmokeEvidenceContract -count=1`
    - local://pass106-base-post-smoke-handoff-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePostSmokeHandoffReadinessContract -count=1`
    - local://pass107-base-owner-review-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseOwnerReviewReadinessContract -count=1`
    - local://pass108-base-verifier-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseVerifierReadinessContract -count=1`
    - local://pass109-base-verifier-result-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseVerifierResultContract -count=1`
    - local://pass110-base-owner-approval-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseOwnerApprovalContract -count=1`
    - local://pass111-base-promotion-rollback-review-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePromotionRollbackReviewContract -count=1`
    - local://pass112-base-package-publication-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePackagePublicationReadinessContract -count=1`
    - local://pass113-base-package-publication-proof-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePackagePublicationProofContract -count=1`
    - local://pass114-base-promotion-execution-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePromotionExecutionReadinessContract -count=1`
    - local://pass115-base-promotion-result-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePromotionResultContract -count=1`
    - local://pass116-base-promotion-settlement-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePromotionSettlementContract -count=1`
    - local://pass117-base-post-promotion-settlement-handoff-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBasePostPromotionSettlementHandoffReadinessContract -count=1`
    - local://pass118-base-durable-state-slice-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceReadinessContract -count=1`
    - local://pass119-base-durable-state-slice-probe-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceProbeContract -count=1`
    - local://pass120-base-source-materializer-readiness-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseSourceMaterializerReadinessContract -count=1`
    - local://pass121-base-runtime-materialization-bridge-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeMaterializationBridgeContract -count=1`
    - local://pass122-base-runtime-equivalence-reentry-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceReentryContract -count=1`
    - local://pass123-base-runtime-durable-proof-gap-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableProofGapContract -count=1`
    - local://pass124-base-runtime-durable-gap-extraction-handoff-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableGapExtractionHandoffContract -count=1`
    - local://pass125-base-runtime-durable-gap-retry-handoff-contract-tests.jsonl
    - `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableGapRetryHandoffContract -count=1`
    - `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
    - `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
    - `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
    - `go test ./internal/runtime ./internal/store -run 'TestCandidatePackageIntakeAdoptionBoundary|TestCandidatePackageIntakeReview|TestCandidatePackageIntakeOptIn|TestCandidatePackageIntakeRoutes|TestCandidatePackageIntakeRoundTripPreservesAdoptionReady|TestCandidatePackageIntakeRejectsUnsafeOrIncompleteRecords'`
    - `go test ./internal/runtime -run 'TestCandidatePackageIntakeReview|TestCandidatePackageIntakeOptIn|TestCandidatePackageIntakeRoutes'`
    - `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntake`
    - `go test ./internal/types ./internal/store ./cmd/candidatepackage ./internal/computerversion ./cmd/evidenceroot ./cmd/vmrealize ./internal/runtime -run 'TestCandidatePackage|TestRunEmitsCandidatePackage|TestBuildCandidatePackage|TestUpsertCandidatePackageIntake'`
    - `go test ./internal/types ./internal/store ./cmd/candidatepackage ./internal/computerversion ./cmd/evidenceroot ./cmd/vmrealize`
    - docs/computer-ontology.md
    - docs/memo-artifact-program-doctrine-2026-06-28.md
    - docs/vision-choir-category-texture-transclusion-v0.md
    - specs/promotion_protocol.tla
    - docs/agent-product-doctrine.md
  rollback_refs:
    - git revert of the commit that adds this document
```

## Child Definition Documents

None yet. Future child definitions should be scoped by proof axis, for example:

- materializer interface and observation-set definition;
- first ledger-slice equivalence proof;
- promotion-over-ComputerVersion formalization;
- first non-Firecracker or projection materializer proof.

---

*This document defines the mission for substrate-independent audited computers.
Any `/goal` run for this mission must abstract substrate technology behind
user-isomorphic audited-computer semantics rather than optimizing a specific VM,
hypervisor, container, or image file.*
