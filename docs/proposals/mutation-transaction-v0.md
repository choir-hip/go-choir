# MutationTransaction: Multi-Ledger Transaction Semantics v0

## Problem

An autoputer's state is spread across multiple heterogeneous ledgers:

| Ledger | Store | Content-addressed by |
|---|---|---|
| Dolt/app state | Dolt database | commit hash |
| Source/build | git repository | SHA |
| Content blobs | blob store (SHA-256) | blob set Merkle root |
| EROFS base | EROFS image + dm-verity | verity root hash |
| Artifact/provenance graph | graph store | graph root hash |
| Route identity | host route table | route ref (opaque) |

Today, the journal (`internal/base/journal`) records a tamper-evident hash
chain of events, but those events only encode file manifest and blob set
changes. There is no typed representation for a transition that touches
multiple ledgers simultaneously. The `StateGenerator` only supports
`file_manifest` and `blob_set` observations — it explicitly does not produce
Dolt, VM state, or promotion certificates.

This means:
- A transition that installs a package (EROFS base change) AND updates app
  state (Dolt commit) cannot be represented as a single audited transaction.
- There is no way to verify that all ledgers are consistent after a transition.
- There is no rollback mechanism that coordinates across ledgers.
- The promotion protocol (`PromoteAppAdoption`) only handles the source-build
  ledger, not the full computer.

## Prior art

Four threads converge on the same problem:

### 1. Event sourcing + CQRS + Saga

The practitioner's consensus for multi-store transaction coordination:

- **Aggregate = unit of consistency.** Each ledger is an aggregate. Invariants
  are enforced within an aggregate. Cross-aggregate changes are coordinated
  via events, not distributed transactions.
- **Saga = coordination pattern.** A multi-ledger transaction is a sequence
  of local commits, one per ledger, with compensating actions for rollback.
  No distributed locks. Each step commits locally and immediately.
- **Event log = source of truth.** Current state is derived by replaying
  events. Projections (read models) are built from the event log.

Choir already has the event log (journal) and the projection (basetree.Derive).
The missing piece is that journal entries don't encode multi-ledger deltas.

### 2. Cross-store ACID (Epoxy, ScalarDB)

Academic systems providing ACID across heterogeneous stores:

- **Epoxy** (VLDB 2023): optimistic concurrency control across Postgres,
  Elasticsearch, MongoDB, GCS, MySQL. Atomic commit without 2PC — only
  requires durable writes. Versioning metadata in record headers.
- **ScalarDB** (VLDB 2023): universal transaction manager for polystores.
  Abstracts each database as a multi-dimensional map. Coordinator with 2PC
  or saga-style compensation.

Choir doesn't need real-time ACID across stores. It needs **replayable audit
consistency** — the ability to verify, after the fact, that all ledgers
reached a consistent state. The saga pattern is sufficient.

### 3. Double-entry / triple-entry accounting

The accounting tradition's approach to multi-party consistency:

- **Double-entry**: every transaction has equal debits and credits. The
  invariant `Σdebits = Σcredits` is the conservation law. State is a vector;
  transactions are delta vectors; invariants are linear constraints.
- **Triple-entry** (Grigg, 2005): adds cryptographically signed receipts
  shared between parties. The receipt IS the transaction evidence. "Turns
  opinions of firm owners into facts agreed between firms."
- **Algebraic accounting** (k3labs, 2025): formalizes the ledger as a state
  vector in ℝⁿ, transactions as delta vectors, evolution as vector addition.
  Invariants are linear constraints on the state vector. Proves determinism,
  reconstructability, and invariant preservation.

In Choir, the "parties" are the **ledgers**. A transition touching Dolt +
git + blobs + EROFS is a multi-party transaction. The "signed receipt" is
the **journal entry** — it records what each ledger agreed to. The invariant
is: `materialize(tape_at_head) = consistent_state_across_all_ledgers`.

### 4. IPLD / Merkle DAG

IPFS's "thin waist" for authenticated data structures:

- Every object is content-addressed (identified by its hash)
- Links between objects are hashes (merkle-links)
- Any data structure can be represented as a merkle DAG
- Objects can be served over untrusted channels (hash verifies content)
- Branching and merging work naturally (git proved this)

The transaction envelope should be a merkle-DAG node. Each ledger delta is a
link to a content-addressed ledger state. The transaction itself is
content-addressed. The tape becomes a merkle DAG, not just a hash chain.

## Design

### The state vector

The autoputer's state at any point is a tuple of ledger heads:

```go
// ComputerState is the complete state vector of an autoputer.
// Each field is a content-addressed reference to a ledger's state.
// This is the "state vector" in the algebraic accounting sense.
type ComputerState struct {
    DoltCommit       Hash    // Dolt commit hash (app state)
    SourceSHA        Hash    // git SHA (source/build state)
    BlobSetRoot      Hash    // Merkle root of blob set (content blobs)
    ErofsVerityRoot  Hash    // dm-verity root hash (EROFS base image)
    ArtifactGraphRoot Hash   // artifact/provenance graph root
    RouteRef         RouteRef // route identity (opaque, host-managed)
}
```

This is the "balance sheet" of the computer. Every ledger has a position.
The state is fully determined by the tape head — no opaque references, no
substrate-specific paths, no "go look at data.img."

### The transaction

A `MutationTransaction` is a journal entry that records a multi-ledger
transition. It follows the saga pattern: each ledger delta is a local commit,
and the transaction as a whole is the coordination evidence.

```go
// MutationTransaction is a multi-ledger transition recorded in the journal.
// It follows the saga pattern: each ledger delta is a local commit with
// before/after hashes and evidence. The transaction is the coordination
// evidence that proves all ledgers reached a consistent state.
//
// This is the "journal entry" in the double-entry accounting sense: it
// records what changed in every ledger, with cryptographic evidence.
// The conservation law is: materialize(parent)[L] = delta[L].before
// for every ledger L, and materialize(this)[L] = delta[L].after.
type MutationTransaction struct {
    // Hash chain (existing journal structure)
    Parent      Hash<MutationTransaction>
    Sequence    uint64
    Timestamp   uint64

    // Saga step metadata (who, what, why)
    Author      AgentID     // which agent initiated this transition
    CapsuleID   string      // which capsule produced the changes
    Kind        TxKind      // mutation, promotion, import, rollback
    Message     string      // human-readable commit message

    // Multi-ledger deltas (double-entry: record before AND after for each)
    Deltas      []LedgerDelta

    // Verification evidence (saga completion proof)
    VerifierResults []VerifierResult

    // Promotion certificate (present when Kind == "promotion")
    PromotionCertificate *PromotionCertificate

    // Computed hash (merkle-link: hash of parent + all fields above)
    Hash        Hash<MutationTransaction>
}

type TxKind string

const (
    TxKindMutation   TxKind = "mutation"   // regular state transition
    TxKindPromotion  TxKind = "promotion"  // candidate → active promotion
    TxKindImport     TxKind = "import"     // opaque legacy import
    TxKindRollback   TxKind = "rollback"   // compensating transaction
)

// LedgerDelta records the transition of a single ledger from one
// content-addressed state to another, with evidence of how the
// transition was performed.
type LedgerDelta struct {
    Ledger      LedgerKind
    Before      Hash        // ledger state at parent entry
    After       Hash        // ledger state after this delta
    Evidence    DeltaEvidence  // how the transition was performed
}

type LedgerKind string

const (
    LedgerDolt          LedgerKind = "dolt"
    LedgerSource        LedgerKind = "source"
    LedgerBlobs         LedgerKind = "blobs"
    LedgerErofsBase     LedgerKind = "erofs_base"
    LedgerArtifactGraph LedgerKind = "artifact_graph"
    LedgerRoute         LedgerKind = "route"
)

// DeltaEvidence is a sum type: each ledger has its own evidence format.
// This is the "receipt" in the triple-entry accounting sense — it proves
// the transition actually happened and was authorized.
type DeltaEvidence struct {
    // Exactly one of the following is set, matching the LedgerKind.
    Dolt        *DoltMergeEvidence    `json:",omitempty"`
    Source      *GitCommitEvidence    `json:",omitempty"`
    Blobs       *BlobSetEvidence      `json:",omitempty"`
    ErofsBase   *ErofsBuildEvidence   `json:",omitempty"`
    ArtifactGraph *GraphMergeEvidence `json:",omitempty"`
    Route       *RouteUpdateEvidence  `json:",omitempty"`
}

// DoltMergeEvidence proves a Dolt branch merge.
type DoltMergeEvidence struct {
    Branch          string  // branch name that was merged
    MergeBase       Hash    // common ancestor commit
    ConflictResolution string // "none" | "manual" | "strategy:X"
    MergeCommit     Hash    // the merge commit hash
    TreeHash        Hash    // Dolt tree hash at merge commit
}

// GitCommitEvidence proves a git commit was made.
type GitCommitEvidence struct {
    Branch          string
    ParentSHA       Hash
    CommitSHA       Hash
    TreeHash        Hash
    PatchHash       Hash    // hash of the diff (for quick comparison)
}

// BlobSetEvidence proves new blobs were added to the blob store.
type BlobSetEvidence struct {
    Added           []Hash  // new blob refs (content-addressed, immutable)
    SetMerkleRoot   Hash    // Merkle root of the complete blob set after addition
    // Blobs are never removed — they're content-addressed and immutable.
    // "Removal" is a set-membership change, not a deletion.
}

// ErofsBuildEvidence proves an EROFS base image was built from the tape.
type ErofsBuildEvidence struct {
    TapeHead        Hash        // tape head used as input
    BuilderVersion  string      // "mkfs.erofs-1.8" or "go-erofs-v0.3"
    BuilderFlags    string      // pinned flags for reproducibility
    ImageHash       Hash        // SHA-256 of the EROFS image bytes
    VerityRoot      Hash        // dm-verity root hash
    VerityOffset    int64       // where hash tree starts in the image
    ImageSize       int64       // image size in bytes
}

// GraphMergeEvidence proves an artifact/provenance graph merge.
type GraphMergeEvidence struct {
    AddedNodes      []Hash      // new graph node refs
    AddedEdges      []GraphEdge // new edges
    GraphRoot       Hash        // new graph root hash
}

// RouteUpdateEvidence proves a route pointer was updated.
type RouteUpdateEvidence struct {
    OldRoute        RouteRef
    NewRoute        RouteRef
    UpdatedBy       AgentID     // who authorized the route change
    // Route updates are host-level operations. The evidence records
    // that the host acknowledged the route change, not that the agent
    // performed it directly.
}

// VerifierResult records the outcome of a verification check.
type VerifierResult struct {
    Verifier        string      // verifier name/capsule ID
    Kind            string      // "read-only-capsule" | "equivalence-check" | "smoke-test"
    Passed          bool
    Evidence        Hash        // hash of verifier output (logs, diffs, etc.)
    Duration        uint64      // milliseconds
}

// PromotionCertificate is the signed evidence that a candidate transition
// was approved for promotion to active state.
type PromotionCertificate struct {
    PromotionID     string
    Kind            string      // "personal" | "platform" | "publication"
    OwnerID         string
    BaseComputerID  string      // computer being promoted from
    CandidateComputerID string  // computer being promoted (may be same VM)
    BaseRefs        ComputerState   // state at cutover base
    CandidateRefs   ComputerState   // state after promotion
    VerifierResults []VerifierResult
    ApprovedBy      AgentID
    ApprovedAt      uint64
    RollbackUntil   uint64      // timestamp; after this, rollback requires explicit mission
}
```

### The conservation law (trial balance)

For every `MutationTransaction` in the journal, the following invariant
must hold:

```
∀ ledger L in Deltas:
  materialize(L, Parent.Hash)[L]  == Deltas[L].Before
  materialize(L, this.Hash)[L]    == Deltas[L].After
  reachable(Deltas[L].Before, Deltas[L].After, Deltas[L].Evidence)
```

In words:
1. The `Before` hash must match the ledger state at the parent entry
2. The `After` hash must match the ledger state after applying this delta
3. The `After` must be reachable from `Before` by applying the evidence

This is the **double-entry conservation law** applied to computer state. The
"trial balance" is the audit that checks this invariant for every entry in
the chain.

### The audit (trial balance)

```go
// AuditTape walks the journal from genesis to head and verifies the
// conservation law for every entry. This is the "trial balance" — an
// automated check that the computer's books balance.
func AuditTape(journal Journal, head Hash) (AuditResult, error) {
    entries, err := journal.VerifyChain(head)  // existing tamper-evidence check
    if err != nil {
        return AuditResult{}, err
    }

    result := AuditResult{}
    for _, entry := range entries {
        for _, delta := range entry.Deltas {
            // Check 1: Before matches parent state
            parentState := materializeState(entry.Parent)
            if parentState[delta.Ledger] != delta.Before {
                result.Failures = append(result.Failures, AuditFailure{
                    Entry:   entry.Hash,
                    Ledger:  delta.Ledger,
                    Reason:  "before hash does not match parent state",
                })
                continue
            }

            // Check 2: After is reachable from Before via evidence
            if !verifyReachable(delta.Before, delta.After, delta.Evidence) {
                result.Failures = append(result.Failures, AuditFailure{
                    Entry:   entry.Hash,
                    Ledger:  delta.Ledger,
                    Reason:  "after hash is not reachable from before via evidence",
                })
                continue
            }

            // Check 3: After matches materialized state at this entry
            thisState := materializeState(entry.Hash)
            if thisState[delta.Ledger] != delta.After {
                result.Failures = append(result.Failures, AuditFailure{
                    Entry:   entry.Hash,
                    Ledger:  delta.Ledger,
                    Reason:  "after hash does not match materialized state",
                })
            }
        }
    }
    return result, nil
}
```

### The state vector at any tape head

```go
// ComputerStateAt returns the complete state vector of the computer at
// the given tape head. This is the "balance sheet" — the position of
// every ledger.
func ComputerStateAt(journal Journal, head Hash) (ComputerState, error) {
    entries, err := journal.VerifyChain(head)
    if err != nil {
        return ComputerState{}, err
    }

    // Start from genesis (empty state) and apply deltas
    state := ComputerState{}  // all zero hashes
    for _, entry := range entries {
        for _, delta := range entry.Deltas {
            state.Set(delta.Ledger, delta.After)
        }
    }
    return state, nil
}
```

### Substrate independence

The `ComputerState` is a tuple of content-addressed hashes. It contains no
substrate-specific references:

- No disk image paths
- No VM IDs
- No host filesystem paths
- No Firecracker socket paths

This means: **any substrate that can produce the same ledger hashes is
equivalent.** The tape doesn't say "this state was produced by Firecracker
VM #42." It says "this state has Dolt commit X, git SHA Y, blob set Z,
EROFS verity root W."

Whether that was materialized on a Firecracker VM, a Cloud Hypervisor VM,
a Docker container, a local process, or a phone — the audit is the same.
The substrate is transparent because the state representation is
content-addressed, not substrate-addressed.

This is the formal grounding for the substrate-independent audited computer:
the `ComputerState` tuple IS the computer. The substrate is just the
materialization path.

### Compensation (rollback)

The tape is append-only. A failed transition is not deleted — a compensating
transaction is appended.

```go
// Rollback appends a compensating transaction that reverts the ledgers
// to their state before the target entry. This follows the saga pattern:
// failures produce more transactions, not rollbacks of distributed state.
func Rollback(
    journal Journal,
    targetEntry Hash,    // the entry to roll back
    author AgentID,
    reason string,
) (Hash, error) {
    // 1. Find the target entry and its parent
    target, err := journal.Get(targetEntry)
    if err != nil { return Hash{}, err }

    parent, err := journal.Get(target.Parent)
    if err != nil { return Hash{}, err }

    // 2. Build compensating deltas: revert each ledger to parent state
    deltas := make([]LedgerDelta, 0, len(target.Deltas))
    for _, delta := range target.Deltas {
        compDelta := LedgerDelta{
            Ledger:  delta.Ledger,
            Before:  delta.After,   // current state (after the failed tx)
            After:   delta.Before,  // revert to parent state
            Evidence: buildCompensationEvidence(delta, parent),
        }
        deltas = append(deltas, compDelta)
    }

    // 3. Append the compensating transaction
    rollbackTx := MutationTransaction{
        Parent:    journal.Head(),
        Sequence:  journal.NextSequence(),
        Timestamp: now(),
        Author:    author,
        Kind:      TxKindRollback,
        Message:   fmt.Sprintf("rollback %s: %s", targetEntry, reason),
        Deltas:    deltas,
    }

    return journal.Append(rollbackTx)
}
```

Each ledger has its own compensation mechanism:
- **Dolt**: `dolt checkout <before-commit>` (native Dolt operation)
- **git**: `git reset --hard <before-sha>` (native git operation)
- **blobs**: no-op (blobs are immutable; set membership reverts)
- **EROFS base**: mount old base (verity root = `before` hash)
- **artifact graph**: revert to old graph root (graph is append-only; revert = use old root)
- **route**: host updates route pointer to old ref

### The self-transition flow with multi-ledger transactions

```
1. Agent spawns capsule with own overlay + Dolt branch
2. Capsule runs experiment (writes to overlay, commits to Dolt branch)
3. Agent verifies results in read-only capsule
4. Agent builds MutationTransaction:
   a. Compute overlay diff → file manifest entries + new blobs
   b. Merge Dolt branch → record DoltMergeEvidence
   c. Commit git changes → record GitCommitEvidence
   d. Add new blobs → record BlobSetEvidence
   e. (EROFS base delta is recorded AFTER materialization, step 6)
5. Agent commits transaction to journal (tape append)
6. Agent materializes new EROFS base from updated tape:
   a. StateGenerator.Generate(tape_head, stagingDir)
   b. ErofsImageBuilder.Build(stagingDir, base.erofs)
   c. VeritySealer.Seal(base.erofs) → verity root
   d. Record ErofsBuildEvidence in a follow-up transaction
7. Agent swaps base (unmount old, mount new, reset overlay)
8. Audit: verify conservation law holds for all entries
```

Steps 4-5 commit the Dolt/source/blob deltas. Step 6 materializes the new
EROFS base and records the erofs_base delta. These are separate saga steps
because EROFS materialization may fail (and the Dolt commit should survive
as a valid branch point even if materialization is retried later).

### Migration from opaque data.img

Existing autoputers have opaque `data.img` that cannot be proven to derive
from any tape. Migration is an `import` transaction:

```go
// ImportLegacyDataImg creates a tape entry that imports the contents of
// an opaque data.img as the initial state. This is the "opening balance"
// for the computer — it can't be proven from first principles, but it's
// recorded as a typed starting point.
func ImportLegacyDataImg(
    journal Journal,
    dataImgPath string,
    doltCommit Hash,    // current Dolt commit (if known)
    sourceSHA Hash,     // current git SHA (if known)
) (Hash, error) {
    // 1. Walk data.img contents, classify into ledgers
    // 2. Hash all files → blob set
    // 3. Record Dolt commit (opaque — trust the existing state)
    // 4. Record source SHA (opaque)
    // 5. Build initial EROFS base from classified contents
    // 6. Record import transaction with kind=import
    importTx := MutationTransaction{
        Parent:   journal.Head(),
        Sequence: journal.NextSequence(),
        Kind:     TxKindImport,
        Message:  "import legacy data.img as initial state",
        Deltas: []LedgerDelta{
            {Ledger: LedgerDolt, Before: ZeroHash, After: doltCommit,
             Evidence: DeltaEvidence{Dolt: &DoltMergeEvidence{
                 Branch: "legacy-import", MergeBase: ZeroHash,
                 ConflictResolution: "opaque-import",
                 MergeCommit: doltCommit,
             }}},
            {Ledger: LedgerSource, Before: ZeroHash, After: sourceSHA,
             Evidence: DeltaEvidence{Source: &GitCommitEvidence{
                 Branch: "legacy-import", ParentSHA: ZeroHash,
                 CommitSHA: sourceSHA,
             }}},
            {Ledger: LedgerBlobs, Before: ZeroHash, After: blobSetRoot,
             Evidence: DeltaEvidence{Blobs: &BlobSetEvidence{
                 Added: blobHashes, SetMerkleRoot: blobSetRoot,
             }}},
            {Ledger: LedgerErofsBase, Before: ZeroHash, After: verityRoot,
             Evidence: DeltaEvidence{ErofsBase: &ErofsBuildEvidence{
                 TapeHead: journal.Head(), BuilderVersion: "mkfs.erofs-1.8",
                 VerityRoot: verityRoot, ImageHash: imageHash,
             }}},
        },
    }
    return journal.Append(importTx)
}
```

The import transaction is auditable: it records exactly what was imported
and when. The `Before` hashes are zero (no prior state). The `After` hashes
are the imported state. The evidence is "opaque-import" — we trust the
existing state as a starting point, and everything after this is fully
audited.

## Relationship to existing types

### Journal (internal/base/journal)

The existing journal's hash chain is preserved. `MutationTransaction` is a
new entry type that extends the journal's event schema. The existing
`VerifyChain()` continues to work — it checks the hash chain. The new
audit (conservation law check) is an additional verification layer on top
of the existing chain verification.

### StateGenerator (internal/computerversion)

The `StateGenerator` currently produces `file_manifest` and `blob_set`
observations from the tape. With `MutationTransaction`, it would also
produce `ComputerState` — the full state vector — by replaying ledger
deltas. The generator's `Derive` function would be extended to walk
`LedgerDelta` entries, not just file manifest events.

### EquivalenceChecker (internal/computerversion)

The `EquivalenceChecker` compares two `ObservationSet`s. With
`MutationTransaction`, a new comparison is possible: comparing two
`ComputerState` tuples. Two computers are equivalent if their state
vectors match (all ledger hashes are equal). This is strictly stronger
than comparing individual observation kinds.

### Materializer (internal/computerversion)

The `Materializer` interface produces a `Realization` from a
`ComputerVersion`. With `MutationTransaction`, the `Realization` would
include the `ComputerState` tuple. The `AutoputerMaterializer` would
materialize all ledgers, not just the EROFS base.

### Promotion protocol (internal/runtime/app_promotion)

The existing `PromoteAppAdoption` handles the source-build ledger. With
`MutationTransaction`, promotion is a `TxKindPromotion` entry that
includes a `PromotionCertificate` with the full `ComputerState` before
and after. The existing `PromoteAppAdoption` becomes one ledger delta
within a larger promotion transaction.

## Mutation class

Orange — adds new types, new audit logic, new journal entry schema. Does
not change existing journal hash chain verification. Does not change
existing `StateGenerator` file manifest generation (extends it). Rollback
path: don't use `MutationTransaction` entries; existing journal entries
continue to work.

## Open questions

1. Should `MutationTransaction` be a new journal entry type alongside
   existing event types, or should it replace them? (Proposal: new type,
   coexist. Existing events are a special case with only file_manifest
   and blob_set deltas.)
2. How are partial deltas handled? If a transition only touches Dolt and
   not git, does the git delta have `Before == After`? (Proposal: yes.
   Omitting a ledger from `Deltas` means it's unchanged. Including it
   with `Before == After` is also valid and more explicit.)
3. How does the `artifact_graph` ledger work? Is it a Merkle DAG like
   IPLD? What's the graph root hash? (Needs separate design.)
4. How does the `route` ledger interact with host-level route management?
   The host owns the route table. The tape records the route ref as
   evidence, but the host performs the actual update. (Proposal:
   `RouteUpdateEvidence` records that the host acknowledged the change.)
5. Can a single capsule produce multiple `MutationTransaction` entries
   in sequence? (Proposal: yes. Each entry is a saga step. A capsule's
   lifecycle may produce several steps before promotion.)
6. How are concurrent capsules coordinated? If two capsules produce
   conflicting deltas to the same ledger, who wins? (Proposal: the
   journal is append-only and linearized. The first commit wins. The
   second capsule must rebase or produce a compensating transaction.)
7. What is the canonical encoding of `MutationTransaction` for hashing?
   (Proposal: DAG-CBOR, following IPLD. Canonical encoding ensures
   the same transaction always produces the same hash.)
8. Should the `ComputerState` tuple be stored as a snapshot at each
   tape head, or always derived by replay? (Proposal: derived by
   replay. Snapshots are an optimization, not a correctness mechanism.
   The tape is the source of truth.)
