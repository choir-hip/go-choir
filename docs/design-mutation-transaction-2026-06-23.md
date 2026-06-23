# Design: Mutation Transaction for Self-Authoring Systems

This document connects the object graph, the hybrid computer/capsule architecture, and the self-authoring `appchange` system into a single transactional model. It is part of the conceptual refactor docset.

## 1. The problem

Choir is a self-authoring system. Agents can propose changes to the code, prompts, schemas, and object graph. A personal mainframe must make those changes safe, reversible, and verifiable.

The current `appchange` system has bugs because it is not yet a fully durable transaction. Promotion happens without complete capture, rollback refs are missing, and verifier evidence is not always attached. The conceptual refactor gives us the vocabulary to fix this: every change is a morphism on the object graph, and every morphism is a transaction.

## 2. Mainframe lesson

Classical mainframes (CICS, IMS, etc.) established that durable state change requires:

- A **unit of work** that appears atomic from the outside.
- A **durable log** of intent, written before the effect is visible.
- **Resource managers** that know how to commit, rollback, and recover.
- A **two-phase commit** or equivalent protocol.
- **Recovery** by replaying the log after failure.

Choir inherits this discipline but applies it to a distributed personal computer: VM snapshots, NixOS generations, Git commits, Dolt branches, blob manifests, Qdrant aliases, and route identity are its resource managers.

## 3. Modern state of the art

The right model for Choir is a **saga transaction** with durable base refs, idempotent stages, verifier evidence, and rollback pointers. This is not a single ACID transaction across all substrates. It is a protocol of local transactions coordinated by a durable `MutationTransaction` object.

Relevant patterns:

- **Saga pattern:** sequence of local commits with compensations.
- **TCC (Try-Confirm-Cancel):** reserve, verify, then confirm.
- **Event sourcing:** append effects to an immutable log.
- **NixOS:** OS state as a functional, reproducible, rollbackable transaction.
- **CRDTs:** for concurrent user-owned edits without global coordination.

## 4. The `MutationTransaction` object

A mutation transaction is a first-class object in the graph. It records everything needed to understand, verify, promote, or reverse a change.

```text
canonical_id: string
object_kind: choir.mutation_transaction
owner_id: string
computer_id: string
state: pending | staged | verifying | committed | rolled_back | failed
authority: user | agent | system
risk_class: green | yellow | orange | red | black
base_refs:
  vm_snapshot: string | null
  nixos_generation: string | null
  git_commit: string | null
  dolt_commit: string | null
  blob_manifest_ref: string | null
  artifact_graph_ref: string | null
  qdrant_alias_ref: string | null
  route_ref: string | null
stages:
  - stage_id: string
    substrate: nucleus_capsule | candidate_vm | nixos_generation | dolt_branch | git_worktree | qdrant_shadow_collection
    input_refs: [string]
    output_refs: [string]
    policy_hashes: [string]
    effect_class: string
    verifier_refs: [string]
    state: pending | running | succeeded | failed | compensated
captured:
  diffs: [string]
  logs: [string]
  test_results: [string]
  verifier_evidence: [string]
  network_attempts: [string]
  filesystem_writes: [string]
  package_changes: [string]
commit:
  committed_at: timestamp | null
  committed_by: string | null
  promotion_refs: [string]
  rollback_refs: [string]
  ttl_until: timestamp
```

See `@/Users/wiz/go-choir/docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md:234-318` for the earlier transaction schema sketch and rollback rules per substrate.

## 5. The five phases

### 5.1 Begin

Record the owner, authority, base computer, base refs, and risk class. The transaction object is created with state `pending`.

### 5.2 Stage

Choose a substrate. For a code change, this might be a `git_worktree` or a `candidate_vm`. For a schema change, a `dolt_branch`. For a vector index change, a `qdrant_shadow_collection`. Each stage is a local transaction with input and output refs.

### 5.3 Execute

Run the bounded mutation inside the substrate. Capture all effects: source diff, build output, test output, network attempts, filesystem writes, package changes.

### 5.4 Verify

Run independent verifiers, preferably in read-only capsules. Verifier evidence attaches to the transaction object. If verification fails, the transaction moves to `failed` and compensations are scheduled.

### 5.5 Commit or rollback

Promotion is a set of atomic or idempotent operations:

- Route identity: switch a pointer.
- NixOS generation: switch boot profile.
- Dolt: merge branch.
- Git: merge commit.
- Qdrant: switch alias.
- Blob store: add content-addressed blobs; GC later.
- VM snapshot: switch active pointer.

Rollback uses the preserved base refs and, where necessary, the old VM snapshot, old NixOS generation, old Dolt commit, old Git commit, old Qdrant alias, and old route pointer. The transaction remains in the graph as a `rolled_back` object with full provenance.

## 6. Appchange as a mutation transaction

An `AppChangePackage` is a specific kind of `MutationTransaction`. It carries source changes between computers and is adopted by a recipient.

Stages for an app change:

1. **Fork candidate** from active computer; record base refs.
2. **Apply source patch** in a worktree or candidate VM.
3. **Build and run tests** in the candidate.
4. **Run verifier capsules** against the candidate output.
5. **Produce adoption package** with diff, policy hashes, and verifier evidence.
6. **Recipient adoption** applies the package to the active computer and re-verifies.
7. **Promotion** switches the active computer to the new refs.
8. **Rollback TTL** preserves old refs until confidence window.

This matches the existing `AppChangePackage` concept from `README.md` and `AGENTS.md`, but makes the transaction object explicit and durable.

## 7. Connection to the object graph

The mutation transaction is itself an object in the graph. It has edges to:

- The base refs it started from.
- The objects it created or modified.
- The verifier evidence objects.
- The supervision findings that tracked it.
- The user decision that promoted or rejected it.

This means the object graph is not just the product data. It is also the **history of how the system changed itself**. The graph contains its own evolution.

## 8. Connection to supervision

The trajectory supervisor watches mutation transactions. Findings include:

- `staged_without_verifier`: a transaction has no verifier refs attached.
- `promotion_without_rollback_refs`: a commit lacks rollback pointers.
- `failed_without_compensation`: a stage failed but no compensating action was recorded.
- `black_surface_attempt`: a transaction classified as black (irreversible) needs explicit user confirmation.

See `@/Users/wiz/go-choir/docs/design-conductor-supervision-protocol-2026-06-23.md` for the supervision protocol schema.

## 9. First implementation target

The first safe transaction to implement is not a full app change. It is a **schema delta**: adding `choir.source_entity` as a real object in the graph.

Why this first:

- It is bounded.
- It already has a real bug behind it (Texture source citations).
- It does not require a full VM fork.
- It can use a Dolt branch for the schema change and a Qdrant shadow collection for the index.
- It proves the transaction object before we ask it to carry self-authoring code changes.

## 10. Open questions

- What is the canonical serialization of `MutationTransaction` for signing and hashing?
- How do we represent user consent as a durable object in the graph?
- How do we bound the lifetime of a transaction in the graph before it is compacted?
- What is the policy for automatically applying green/yellow transactions vs. requiring user approval for orange/red/black?

These are answered by the first migration, not by this doc.

## 11. Why this reduces entropy

A self-authoring system without durable transactions is just a pile of scripts and manual steps. A self-authoring system with durable transactions has a single shape: every change is a `MutationTransaction` object. The system can inspect itself, verify itself, and rollback itself. That is how the description length shrinks even as the system grows.
