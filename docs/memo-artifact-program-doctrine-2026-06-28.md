# Memo: Artifact Program Doctrine

**Date:** 2026-06-28
**Mutation class:** green (doctrine documentation, no runtime behavior change)
**Status:** proposed — awaiting promotion into Choir Doctrine

## The Assertion

A user's computer is a computation, not a state blob:

```
computer = choir_code(artifact_program)
```

- `choir_code` is the choir source code at a specific version (a Nix closure, a git commit)
- `artifact_program` is the mutation transaction history — the textures, the paragraphs
- The output is a running computer, deterministically

The current `data.img` is an opaque cache of this computation. It is not the source of truth. The artifact program is the source of truth.

## Graph vs Program

The prior framing — "artifact graph" — describes a static structure: nodes and edges, a snapshot to replicate. This is insufficient.

The correct framing is "artifact program": a recipe that computes. A graph has structure; a program has structure *and execution*. The distinction matters for mutation:

- A graph needs external mutation — someone writes a new version
- A program mutates itself through execution — each step produces the next state

The mutation transaction IS the program step. The program is a transcript of computation, not a snapshot.

## Textures Are Programs

A texture revision is not a static document version. It is a computation step — a transaction that reads current state, applies a transformation, and writes the next state. A texture is both a graph (it has structure, lineage, edges to sources and other textures) and a program (it computes, it transforms, it produces output).

Para-graphs: alongside graphs, beyond graphs. Each texture revision is a paragraph in the program. The program is the narrative of the computer's evolution, written in the language of mutation transactions, executed by the choir runtime.

## The Choir Source Code Is the Key

The computation `f` in `f(artifact_program) -> running_computer` is not a fixed, eternal function. It is the choir codebase itself. When choir changes, `f` changes. `f_v1(program)` and `f_v2(program)` produce different computers from the same inputs.

This is exactly Nix. The derivation changes when the nix expression changes, even if the inputs are identical. The nix expression IS the program. The store paths are the outputs. You don't replicate outputs — you replicate the expression + inputs and rebuild.

The full equation with versioning:

```
computer = choir_code_vN(artifact_program_vM)
```

The "current state" is a pair of pointers: `(choir_code_version, program_version)`, like `(nixpkgs_rev, git_commit)`.

Two kinds of version transitions:

```
(choir_code_vN, program_vM) --transaction--> (choir_code_vN, program_vM+1)
(choir_code_vN, program_vM) --deploy-------> (choir_code_vN+1, program_vM)
```

Both produce a new computer when computed. Both are auditable.

## Why This Matters for Auditability and Compliance

### Complete provenance

Every state change is a transaction in the program. Every transaction has:
- An author (which actor, which device, which user)
- A timestamp (ordered, not wall-clock)
- A type (file write, Dolt mutation, config change, promotion)
- Inputs (what state was read)
- Outputs (what state was written)
- The choir code version that executed it

This is a complete audit tape by construction. You don't build a separate audit system — the program transcript IS the tape. Compliance questions become queries:

- "What changed in this user's computer between time A and time B?" → replay the tape
- "Who authorized this state change?" → inspect the transaction author
- "What code version was running when this change happened?" → inspect the choir_code version
- "Can we reproduce the state at any point in time?" → yes, replay the program to that version

### Reproducibility

Because the computer is a deterministic computation over a versioned program, any historical state is reproducible:

```
computer_at_time_T = choir_code_vN(artifact_program_vM)
```

where `(vN, vM)` is the version pair at time T. This is stronger than a backup — a backup tells you what the state was; a program replay tells you *why* the state was that way, because the tape explains every step.

### Tamper evidence

Content-addressing makes the program tamper-evident:
- Each transaction references the content hash of the previous transaction
- Each blob is content-addressed (changing the bytes changes the hash)
- The choir code version is a git commit hash
- Modifying any historical transaction changes all downstream hashes

This is a Merkle chain — the same integrity property that Git and Nix provide, extended to user state.

## Why This Matters for Replication and Distribution

### Replication = replicate inputs, not outputs

You don't copy `data.img` (the build output). You copy `choir_code` (already done via Nix) and `artifact_program` (the tape + blobs). Then you compute a fresh `data.img` on the target node.

This is cheaper, safer, and more flexible:
- Cheaper: content-addressed blobs deduplicate; only changed transactions propagate
- Safer: you can verify the rebuild matches the original by comparing content hashes
- More flexible: any node with the inputs can compute the computer; no "source node" needed

### Distribution = the program flows to where computation happens

The artifact program flows to:
- Node-b (the primary computation site — runs the VM)
- Node-a (the standby — can compute a shadow VM for verification or failover)
- Desktop (computes a FileProvider projection — a partial computation over the same program)
- Mobile (computes a Files app projection — another partial computation)

Each site computes a different projection of the same program. The VM is the full computation. The FileProvider domain is a projection that computes only the file layer. They share the same source of truth.

### Consensus = ordering the tape

Multiple devices emit transactions. The consensus protocol orders them:
- Dolt's merge semantics order SQL transactions (structured data)
- The file/blob layer needs an equivalent — last-writer-wins with explicit conflict surfacing (the three-tree reconciliation model from Choir Base), or a CRDT for specific types
- The ordering is the program's execution order; different orderings produce different computers

This is why consensus is foundational, not an afterthought. The program's meaning depends on step ordering. A file write followed by a file delete is different from a file delete followed by a file write.

## The Proof Strategy

### Determinism (f is pure)

Same `choir_code` version + same `program` version → same computer. This is Nix's determinism, extended from the OS layer to the data layer.

Proof: build the same program on two nodes. Compare content hashes of the output `data.img`. If they match, `f` is deterministic. If they don't, the diff reveals hidden inputs.

The OS half already proves this (`rootfs.ext4` and `storedisk.erofs` are deterministic Nix build outputs). The data half needs the same discipline: the file manifest, the Dolt state, the blob materialization must all be deterministic derivations.

### Losslessness (g is total)

Every meaningful state change in the computer is captured as a transaction. If the VM writes a file, that's a transaction. If it writes to the runtime DB, that's a transaction. If it changes a permission, that's a transaction.

Proof: the round-trip test. Extract from VM (`g`), rebuild (`f`), compare. If the rebuilt `data.img` differs from the original, the diff reveals what state the transaction types don't capture. This is an empirical, iterative proof — each gap found is a missing transaction type.

### Integrity (transactions are typed and validated)

Each transaction type specifies its required fields:
- File write: blob hash, path, permissions, ownership, xattrs
- Dolt mutation: SQL statement, affected rows
- Config change: key, value, source

A transaction missing a required field is rejected. This prevents silent state loss — you can't lose a permission change if the file-write transaction type requires permissions to be specified.

### Shadow replication (production proof)

Run the artifact program replication alongside the current `data.img` system. The VM still uses `data.img` as primary state. The program is extracted in shadow mode and replicated to node-a. On node-a, compute a VM from the program and run it as a read-only shadow. Compare shadow state against primary state. Divergence reveals decomposition gaps or non-determinism.

This catches real-world state that property-based tests miss.

## The Hard Parts

### The file manifest

Dolt state is naturally declarative (SQL rows). Base blobs are naturally content-addressed. But filesystem metadata — permissions, ownership, symlinks, hardlinks, xattrs, special files — needs a declarative representation that's lossless. This is where the type system matters most. Get the manifest format right and the build is deterministic. Get it wrong and you have silent state loss.

### Ephemeral vs persistent state

The running computer has state that is NOT part of the artifact program: in-memory process state, network connections, terminal sessions, running daemons. These are computation-in-progress, not program state. Losing them is like losing a browser tab — the user's data is safe, but their working context is gone.

The type system must distinguish:
- **Persistent, meaningful state** → modeled as transactions, part of the program
- **Ephemeral, working state** → not modeled, lost on restart, acceptable
- **Cached, reconstructible state** → not modeled, rebuilt on demand (Go module cache, build artifacts)

Getting this classification wrong in either direction is dangerous. Modeling ephemeral state as persistent creates bloat and consensus overhead. Failing to model persistent state as a transaction causes data loss.

### Concurrent mutation

The program is not single-threaded. Multiple actors (the VM, the desktop client, the mobile client, background jobs) emit transactions concurrently. The consensus protocol must order them consistently across all nodes. Dolt handles this for SQL. The file layer needs an equivalent.

The three-tree reconciliation model from Choir Base handles single-device sync (local tree, remote tree, synced tree). Multi-writer consensus across nodes is a harder problem that may require a stronger consensus protocol (Raft, Paxos) or careful CRDT design.

## Relationship to Existing Architecture

### Nix (OS layer)

The OS half of the equation is already proven. `rootfs.ext4` and `storedisk.erofs` are deterministic Nix build outputs. The artifact program doctrine extends this from the OS to the data layer.

### Dolt (structured state)

Dolt is already a versioned, mergeable artifact graph for structured state. Dolt commits are transactions. Dolt's merge semantics provide consensus for SQL state. Dolt is the natural home for the structured portion of the artifact program.

### Choir Base (file state)

Choir Base's blob/item/version model is the natural home for the file portion of the artifact program. Content-addressed blobs are immutable (no consensus needed). The item/version/metadata layer is mutable and needs the three-tree reconciliation or equivalent.

### Textures (the program)

Textures are already versioned, lineage-tracked revisions. The reframing is: textures are not documents, they are program steps. Each revision is a transaction in the computer's evolution. The texture system is the human-readable projection of the artifact program.

### Mutation transactions (the execution)

Mutation transaction hardening (from the road-ahead) is not just a database integrity concern. It is the integrity of the program itself. A corrupted mutation transaction is a corrupted program step, which produces a corrupted computer. The tape IS the program.

## Naming

"Artifact program" rather than "artifact graph" because:
- A graph is static; a program computes
- A graph is replicated; a program is executed
- A graph has versions; a program has a transcript
- Textures are para-graphs: paragraphs in the program, alongside the graph structure

## SBOM Integration

The `choir_code` component of the equation needs a machine-readable bill of materials. We've integrated **sbomnix** to generate CycloneDX SBOMs from the Nix flake:

```
nix build .#sbom.x86_64-linux.auth    # SBOM for auth service
nix build .#sbom.x86_64-linux.proxy   # SBOM for proxy service
```

Each SBOM lists every dependency and transitive input that goes into building the service — the complete supply chain inventory. SBOMs are generated in CI on every push to main and uploaded as artifacts (90-day retention).

**Migration path to FlakeBOM:** sbomnix generates CycloneDX, the same format as FlakeBOM (Determinate Systems). When we switch to Determinate Nix for enterprise sales, FlakeBOM drops in as a replacement with richer features (CVE pedigree, vendored dependency detection, provenance data). The SBOM format and CI workflow remain compatible.

**Role in the proof:** The SBOM is the inventory of `choir_code` at a specific version. Combined with the artifact program version, it fully determines the computer:

```
computer = choir_code_vN(artifact_program_vM)
```

where `choir_code_vN` is auditable via its SBOM (what dependencies, what versions, what licenses) and `artifact_program_vM` is auditable via its tape (what mutations, what authors, what timestamps). Together they provide complete provenance for any computer state at any point in time.

## The Self-Authoring Program

The tape IS the program. The program writes itself through mutation
transactions. The computer executes, and execution produces new tape entries.
The computer writes its own program. It is self-authoring.

But "self-authoring" alone is just a feedback loop — a system that modifies
itself. The key question is: does the modification improve the system? Does it
learn?

### The Category Bridge: Conjecture Learning

**Texture category (computation):**
- Objects: textures (states)
- Morphisms: transclusions (structure)
- The tape: composition of morphisms over time — the program that computes the
  computer

**Conjecture category (learning):**
- Objects: conjectures (beliefs about what should be)
- Morphisms: evidence/refutation (tests that transform beliefs)
- Learning: composition of evidence over time — updating beliefs

**The bridge:** A texture IS a conjecture. The state of the computer is the
system's belief about what it should be. Each mutation transaction is a
conjecture revision — the system updates its belief about its own state.

- "This file should contain X" is a conjecture. Writing the file tests it. The
  result is a tape entry.
- "This candidate should be promoted" is a conjecture. Running it tests it.
  Promotion or rollback is the verdict.
- "This configuration is correct" is a conjecture. Executing with it tests it.
  A crash is refutation.

The tape is both the computation history AND the learning history. They are
the same thing. The program doesn't just compute the computer — it learns the
computer. Each transaction is a step in learning what the computer should be.

### Conjecture Learning in Choir

The codebase already has the infrastructure for this:

- **Conjecture ledger** (`docs/conjecture-assertion-ledger-2026-06.md`):
  tracks discovered/introduced/repaired heresies. A heresy is a conjecture
  that the tape is wrong. A repair is a conjecture that the tape is now right.
  The ledger is the meta-tape — the tape about the tape.

- **Parallax skill**: "the mission document claims that completing an artifact
  will advance a deeper goal, then tests and constructs that claim through
  observer shifts." Each parallax pass is a conjecture tested — a tape
  revision at the mission level.

- **Promotion/rollback**: a conjecture that a candidate computer is better
  than the current one. Execution tests it. Promotion confirms. Rollback
  refutes. The tape records the verdict.

- **Actor runtime**: actor handlers execute activations. Each activation is a
  transaction on the tape — a conjecture tested by execution.

### The Adaptive Mechanism

A self-learning system is one where the program updates itself based on its
own execution. The tape is both input and output:

```
read tape (current state = current beliefs)
  → execute (interact with the world)
  → write tape (updated state = updated beliefs)
  → read tape (current state = current beliefs)
  → ...
```

This is learning. The system adapts because each tape entry is informed by
the result of the previous execution. The computer doesn't just recompute the
same state — it computes a *better* state, because the tape encodes what was
learned from prior executions.

The conjecture ledger makes this explicit: when the system discovers a heresy
(a belief that was wrong), it repairs it (writes a correction to the tape).
The repair is a learning step. The tape after the repair is different from
the tape before — the system has learned.

### The Natural Transformation

The category bridge is a natural transformation between two functors:

- **Compute functor**: `tape → computer` (deterministic replay)
- **Learn functor**: `tape → improved_tape` (conjecture revision)

The natural transformation maps "what state is the computer in?" to "what
does the computer believe it should be?" — and these are the same question.
The computer's state IS its beliefs. Computing the computer IS replaying its
learning.

This is why the tape is not a log. A log records what happened. A tape is
read and written by the program. The program reads its beliefs, acts on them,
observes the results, and writes updated beliefs. That's learning. The tape
is the medium of learning.

## Open Questions

1. **File manifest format**: What declarative representation captures filesystem metadata losslessly? Options: NAR format (Nix), cpio, a custom JSON/protobuf schema, or a Dolt table.

2. **Transaction granularity**: How coarse or fine are mutation transactions? Per-file-write? Per-session? Per-epoch? This affects tape size, consensus overhead, and replay cost.

3. **Consensus protocol**: Dolt provides SQL consensus. What provides file/metadata consensus across nodes and devices? Is the three-tree model sufficient, or do we need Raft?

4. **Shadow replication timeline**: When do we start shadow replication to validate the decomposition before relying on it?

5. **Choir code versioning in the program**: How does the program reference the choir code version? Is it embedded in each transaction, or is it a separate pointer that applies to a range of transactions?

6. **Migration path**: How do we transition from `data.img` as primary to artifact program as primary without risking user data? Shadow replication first, then cutover?

## Lineage

- This doctrine emerged from investigating user logout during deploys (2026-06-28), which led to questioning the single-node deployment model, which led to questioning VM state representation, which led to the artifact program assertion.
- Related: `docs/choir-base-product-spec-2026-06-06.md` (file sync architecture), `docs/computer-ontology.md` (typed artifact decomposition), `docs/choir-doctrine.md` (apex doctrine), `docs/road-ahead-2026-06-27.md` (mutation transaction hardening), `docs/vision-choir-category-texture-transclusion-v0.md` (texture as program), `docs/conjecture-assertion-ledger-2026-06.md` (conjecture tracking infrastructure).
- The self-authoring program / conjecture learning connection emerged from recognizing that the tape is both computation history and learning history — the same category-theoretic structure, viewed from two functors.
- The "promote typed artifacts, not opaque machine accidents" principle in computer-ontology.md is the direct ancestor of this doctrine.
