# Cross-Substrate Projection Proof Mission

## Harness Invocation Semantics

```text
/goal docs/missions/cross-substrate-proof-v0.md
```

Execute autonomously until SIAC completion gate 4 is satisfied with named
evidence, or until a sharply evidenced escalation, blocker, or supersession
condition is met.

## Source Authority Order

1. `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
   (SIAC definition graph + completion semantics)
2. `specs/promotion_protocol.tla` (promotion safety model)
3. `AGENTS.md` (repo operating contract)
4. This document

## Mission Purpose

Satisfy SIAC completion gate 4:

> The same ComputerVersion is materialized or projected through Firecracker and
> at least one non-identical substrate/projection path, and the equivalence
> checker passes for a declared ObservationSet.

And gate 5:

> A seeded mismatch or unsupported capability causes the equivalence checker to
> fail or narrow the claim, proving the verifier is not ceremonial.

## Mutation Class

**Yellow** for extractor implementations and tests: these add read-only
extraction paths and test infrastructure without changing product behavior or
touching VM lifecycle.

**Orange** if wiring touches runtime extraction from a live Base journal: name
the class and rollback path.

This mission does NOT launch, stop, resume, copy, or mutate any VM. It reads
from filesystem state and typed data structures. It does not touch promotion,
rollback, auth, staging deploy, or run acceptance.

## Current State

### What exists
- `Materializer` interface (`internal/computerversion/types.go`)
- `VMManagerScopedMaterializer` — classifies VM state paths, emits
  `vm_state_manifest` only, does NOT extract `file_manifest`/`blob_set`
- `ProjectionMaterializer` — wraps pre-extracted observations
- `BaseJournalExtractor` — extracts `file_manifest` from a live Base journal
- `BaseEventExtractor` — extracts `file_manifest` from typed events
- `BaseCurrentStateObservationSet` — composes journal + blob store observations
- `CompareBaseCurrentStateToFileProjection` — compares two projections but both
  use `ProjectionMaterializer` with pre-extracted observations
- `BaseSubstrateEquivalenceContract` — contract framework for cross-substrate proof
- `EquivalenceChecker` — pure comparison of observation sets

### What's missing
- A Firecracker extraction path that produces `file_manifest`/`blob_set`
  observations from a VM's persistent directory (not just `vm_state_manifest`)
- A second substrate extraction path that reads the same observations from a
  non-Firecracker substrate
- A cross-substrate proof where both sides extract from real (fixture) data,
  not just pre-extracted observations wrapped in `ProjectionMaterializer`
- A failure proof with seeded mismatches

## Execution Plan

### Phase 1: FirecrackerStateExtractor

Create `FirecrackerStateExtractor` in
`internal/computerversion/firecracker_state_extractor.go`.

This extractor reads `file_manifest` and `blob_set` observations from a
Firecracker VM's persistent directory structure. It does NOT launch a VM. It
reads from:
- The VM's persistent directory (typed file paths)
- The blob store referenced by those files

The extractor produces an `ObservationSet` with `ObservationFileManifest` and
`ObservationBlobSet` observations, tagged with the Firecracker substrate
identity.

### Phase 2: Non-Firecracker Projection Extractor

Create a second substrate extractor that reads the same `file_manifest` and
`blob_set` observations from a non-Firecracker path. Options:
- A Dolt/objectgraph projection (reads from Dolt tables)
- A host-process projection (reads from a local directory tree)
- A container projection (reads from a container filesystem layer)

The simplest safe option is a host-process projection: read the same logical
file tree from a different filesystem path, producing observations with a
different substrate identity. This proves substrate independence without
requiring a second hypervisor.

### Phase 3: Cross-Substrate Equivalence Proof

Create a test that:
1. Creates fixture data representing a ComputerVersion's durable state
2. Extracts observations through the FirecrackerStateExtractor
3. Extracts observations through the non-Firecracker projection extractor
4. Materializes both through appropriate capability manifests
5. Runs `EquivalenceChecker.CheckRealizations` and asserts `EquivalenceEquivalent`
6. Builds a `BaseSubstrateEquivalenceContract` from the proof

### Phase 4: Failure Proof

Create a test that:
1. Seeds a mismatch (corrupt a file, change a blob, alter a manifest entry)
2. Runs the same extraction and comparison
3. Asserts `EquivalenceNotEquivalent` with concrete `Difference` entries
4. Proves the verifier is not ceremonial

### Phase 5: Update SIAC Checkpoint

Update `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
with:
- Gate 4 evidence: cross-substrate proof exists and passes
- Gate 5 evidence: failure proof exists and catches seeded mismatches
- Updated belief state and remaining error field

## Boundaries

- **No VM lifecycle:** This mission does not launch, stop, resume, copy, or
  mutate any VM. All extraction is from filesystem state or typed data.
- **No promotion/rollback:** This mission does not touch promotion or rollback
  semantics.
- **No staging deploy:** This mission does not deploy to staging.
- **No new boundary contracts:** This mission uses the existing
  `BaseSubstrateEquivalenceContract`. No new contract files are added.
- **Read-only extraction:** All extractors are read-only. They do not mutate
  the data they read from.

## Completion Semantics

This mission is complete when:

1. `FirecrackerStateExtractor` exists and produces `file_manifest`/`blob_set`
   observations from fixture VM persistent state.
2. A non-Firecracker projection extractor exists and produces the same
   observation kinds from a different substrate path.
3. A cross-substrate equivalence test passes: same `ComputerVersion` through
   both extractors, `EquivalenceChecker` returns `EquivalenceEquivalent`.
4. A `BaseSubstrateEquivalenceContract` is built from the proof.
5. A failure proof test passes: seeded mismatch causes
   `EquivalenceNotEquivalent` with named differences.
6. SIAC checkpoint is updated with gate 4 and gate 5 evidence.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: checkpoint_incomplete
  last_checkpoint: |
    Mission checkpoint_incomplete. The file-manifest/blob-set slice has
    FirecrackerStateExtractor and HostProjectionExtractor, three passing tests
    (equivalence, failure, narrowed), and a BaseSubstrateEquivalenceContract
    built from the proof. SIAC gates 4 and 5 remain unproven and are the next
    work. All computerversion tests pass (47.7s).
  current_artifact_state:
    - internal/computerversion: 117 Go files, 39 contract files
    - specs/promotion_protocol.tla: non-vacuous invariants, TLC passes
    - SIAC gates 1-3 settled, gate 4 (cross-substrate proof) is next
  what_shipped:
    - commit 0f8b19a8 (substrate hardening)
  what_was_proven:
    - TLA+ invariants are non-vacuous (TLC passes)
    - Purity claim is accurate
    - Shared contract types compile
    - cmd binaries use shared utilities
  unproven_or_partial_claims:
    - Gate 4: cross-substrate proof (this mission)
    - Gate 5: failure proof (this mission)
  next_executable_probe: create FirecrackerStateExtractor
  suggested_goal_string: "/goal docs/missions/cross-substrate-proof-v0.md"
  rollback_refs:
    - commit 0f8b19a8 on main (substrate hardening)
```
