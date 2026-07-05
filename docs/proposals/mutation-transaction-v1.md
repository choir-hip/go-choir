# MutationTransaction: Multi-Ledger Transaction Semantics v1

> v1 incorporates review feedback from Codex and Cursor (v0 review round).
> Changes: split V ledger into substrate identity components, content-address
> the route ledger, incremental O(n) audit, forward-delta compensation with
> conflict detection, completed evidence types, import circularity fix,
> per-ledger reachability definitions, cross-ledger invariant checks.

## Problem

An autoputer's state is spread across multiple heterogeneous ledgers:

| Ledger | Store | Content-addressed by |
|---|---|---|
| VM/runtime (V) | kernel, rootfs, Nix store, EROFS base | per-component content hash |
| Dolt/app state (D) | Dolt database | commit hash |
| Source/build (S) | git repository | SHA |
| Content blobs (B) | blob store (SHA-256) | blob set Merkle root |
| Artifact/provenance graph (A) | graph store | graph root hash |
| Route identity (R) | route configuration | content-addressed config hash |

The ontology defines `W = (V, D, S, B, A, R)` (`docs/computer-ontology.md:184`).
Today, the journal (`internal/base/journal`) records a tamper-evident hash
chain of events, but those events only encode file manifest and blob set
changes. There is no typed representation for a transition that touches
multiple ledgers simultaneously. The `StateGenerator` only supports
`file_manifest` and `blob_set` observations — it explicitly does not produce
Dolt, VM state, or promotion certificates (`internal/computerversion/state_generator.go:125`).

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

The transaction envelope is a merkle-DAG node. Each ledger delta is a link
to a content-addressed ledger state. The transaction itself is
content-addressed. The tape becomes a merkle DAG, not just a hash chain.

## Design

### The state vector

The autoputer's state at any point is a tuple of ledger heads, following the
ontology's `W = (V, D, S, B, A, R)`. The V ledger is split into its substrate
identity components because `ErofsVerityRoot` alone is not the whole V ledger
— the kernel, rootfs, and Nix store are also part of VM/runtime identity
(`internal/computerversion/vmmanager_boundary.go:24`).

```go
// ComputerState is the complete state vector of an autoputer.
// Each field is a content-addressed reference to a ledger's state.
// This is the "state vector" in the algebraic accounting sense.
//
// The tuple follows the ontology W = (V, D, S, B, A, R) where V is
// split into substrate identity components.
type ComputerState struct {
    // V ledger (VM/runtime) — substrate identity components
    KernelImageHash    Hash    // kernel image content hash
    RootfsHash         Hash    // initrd/rootfs content hash
    StoreDiskHash      Hash    // Nix store disk content hash
    ErofsVerityRoot    Hash    // autoputer base EROFS dm-verity root hash

    // D ledger (Dolt/app state)
    DoltCommit         Hash    // Dolt commit hash

    // S ledger (source/build)
    SourceSHA          Hash    // git SHA

    // B ledger (content blobs)
    BlobSetRoot        Hash    // Merkle root of blob set

    // A ledger (artifact/provenance graph)
    ArtifactGraphRoot  Hash    // artifact graph root hash

    // R ledger (route identity) — content-addressed, NOT opaque
    RouteConfigHash    Hash    // sha256(canonical RouteConfig)
}
```

The route ledger is content-addressed, not opaque. This is critical for
substrate independence: two substrates producing the same tape must produce
the same route config hash. The route config describes *what* the route
should be, not *where* it lives:

```go
// RouteConfig is the content-addressed route configuration.
// The host performs the actual route update, but the tape records
// what the route should be, not where it lives.
type RouteConfig struct {
    OwnerID     string
    Hostname    string
    Port        int
    TLSCertHash Hash    // content-addressed TLS cert
    Protocol    string  // "http" | "https" | "vsock"
}

// RouteConfigHash = sha256(canonical_json(RouteConfig))
```

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
// The conservation law is: state_at(parent)[L] = delta[L].before
// for every ledger L, and state_at(this)[L] = delta[L].after.
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
    // Encoded as DAG-CBOR for canonical encoding.
    Hash        Hash<MutationTransaction>
}

type TxKind string

const (
    TxKindMutation   TxKind = "mutation"   // regular state transition
    TxKindPromotion  TxKind = "promotion"  // candidate → active promotion
    TxKindImport     TxKind = "import"     // opaque legacy import
    TxKindRollback   TxKind = "rollback"   // compensating transaction
    TxKindNoOp       TxKind = "noop"       // no state change (audit marker)
)
```

### Ledger deltas and evidence

```go
// LedgerDelta records the transition of a single ledger from one
// content-addressed state to another, with evidence of how the
// transition was performed.
type LedgerDelta struct {
    Ledger      LedgerKind
    Before      Hash            // ledger state at parent entry
    After       Hash            // ledger state after this delta
    Evidence    DeltaEvidence   // how the transition was performed
}

type LedgerKind string

const (
    LedgerKernel       LedgerKind = "kernel"
    LedgerRootfs       LedgerKind = "rootfs"
    LedgerStoreDisk    LedgerKind = "store_disk"
    LedgerErofsBase    LedgerKind = "erofs_base"
    LedgerDolt         LedgerKind = "dolt"
    LedgerSource       LedgerKind = "source"
    LedgerBlobs        LedgerKind = "blobs"
    LedgerArtifactGraph LedgerKind = "artifact_graph"
    LedgerRoute        LedgerKind = "route"
)

// DeltaEvidence is a sum type: each ledger has its own evidence format.
// This is the "receipt" in the triple-entry accounting sense — it proves
// the transition actually happened and was authorized.
type DeltaEvidence struct {
    // Exactly one is set, matching the LedgerKind.
    Kernel         *KernelImageEvidence    `json:",omitempty"`
    Rootfs         *RootfsBuildEvidence    `json:",omitempty"`
    StoreDisk      *StoreDiskBuildEvidence `json:",omitempty"`
    ErofsBase      *ErofsBuildEvidence     `json:",omitempty"`
    Dolt           *DoltMergeEvidence      `json:",omitempty"`
    Source         *GitCommitEvidence      `json:",omitempty"`
    Blobs          *BlobSetEvidence        `json:",omitempty"`
    ArtifactGraph  *GraphMergeEvidence     `json:",omitempty"`
    Route          *RouteUpdateEvidence    `json:",omitempty"`
    NoOp           *NoOpEvidence           `json:",omitempty"`
    Materialization *MaterializationEvidence `json:",omitempty"`
    Import         *ImportEvidence         `json:",omitempty"`
}
```

### Evidence types (completed per reviewer feedback)

```go
// DoltMergeEvidence proves a Dolt branch merge.
type DoltMergeEvidence struct {
    Branch             string           // branch name that was merged
    MergeBase          Hash             // common ancestor commit
    ConflictResolution string           // "none" | "manual" | "strategy:X"
    MergeCommit        Hash             // the merge commit hash
    TreeHash           Hash             // Dolt tree hash at merge commit
    TableHashes        map[string]Hash  // per-table root hash (v1 addition)
}

// GitCommitEvidence proves a git commit was made.
// Uses revert commits for compensation, never git reset --hard.
type GitCommitEvidence struct {
    Branch      string
    ParentSHA   Hash
    CommitSHA   Hash
    TreeHash    Hash
    PatchHash   Hash        // hash of the diff (for quick comparison)
    AuthorID    AgentID     // who authored the change (v1 addition)
    CommitterID AgentID     // who committed (v1 addition)
}

// BlobSetEvidence proves new blobs were added to the blob store.
type BlobSetEvidence struct {
    Added            []Hash  // new blob refs (content-addressed, immutable)
    SetMerkleRoot    Hash    // Merkle root of the complete blob set after addition
    BlobMetadataRefs []Hash  // refs to Dolt/artifact records for metadata (v1)
    // Blobs are never removed — they're content-addressed and immutable.
    // "Removal" is a set-membership change, not a deletion.
}

// ErofsBuildEvidence proves an EROFS base image was built from the tape.
// Uses TapeSliceStart/End instead of TapeHead to avoid circularity (v1).
type ErofsBuildEvidence struct {
    TapeSliceStart  Hash        // first journal entry included in this build
    TapeSliceEnd    Hash        // last journal entry included (this tx or prior)
    BuilderVersion  string      // "mkfs.erofs-1.8" or "go-erofs-v0.3"
    BuilderFlags    string      // pinned flags for reproducibility
    ImageHash       Hash        // SHA-256 of the EROFS image bytes
    VerityRoot      Hash        // dm-verity root hash
    VerityOffset    int64       // where hash tree starts in the image
    ImageSize       int64       // image size in bytes
    OverlayDiffHash Hash        // hash of overlay diff included (v1, for compensation)
}

// KernelImageEvidence proves a kernel image was selected/updated.
type KernelImageEvidence struct {
    ImageHash    Hash    // SHA-256 of kernel image bytes
    Version      string  // kernel version string
    BuilderRef   string  // Nix derivation or build reference
}

// RootfsBuildEvidence proves a rootfs/initrd was built.
type RootfsBuildEvidence struct {
    ImageHash    Hash
    NixClosure   string  // Nix closure path
    BuilderRef   string
}

// StoreDiskBuildEvidence proves a Nix store disk was built.
type StoreDiskBuildEvidence struct {
    DiskHash     Hash
    NixClosure   string
    ErofsRoot    Hash    // EROFS root hash of the store disk
}

// GraphMergeEvidence proves an artifact/provenance graph merge.
type GraphMergeEvidence struct {
    AddedNodes      []Hash      // new graph node refs
    AddedEdges      []GraphEdge // new edges
    GraphRoot       Hash        // new graph root hash
}

type GraphEdge struct {
    From    Hash
    To      Hash
    Kind    string
    Label   string
}

// RouteUpdateEvidence proves a route configuration was updated.
// The host performs the actual update; the tape records the content-addressed
// config that the host acknowledged.
type RouteUpdateEvidence struct {
    OldConfigHash   Hash        // sha256(canonical_json(old RouteConfig))
    NewConfigHash   Hash        // sha256(canonical_json(new RouteConfig))
    UpdatedBy       AgentID     // who authorized the route change
    HostAcknowledged bool       // host confirmed the route update
    AcknowledgedAt  uint64      // when host confirmed
}

// NoOpEvidence records that a ledger was not changed in this transaction.
type NoOpEvidence struct {
    Reason  string  // why this ledger was not touched
}

// MaterializationEvidence proves that a ledger state was materialized
// (e.g., EROFS base was built and mounted). Distinct from ledger mutation
// evidence — materialization is the act of realizing a state, not changing it.
type MaterializationEvidence struct {
    LedgerState     Hash        // the ledger state that was materialized
    MaterializedAt  uint64      // when materialization occurred
    Substrate       string      // "firecracker" | "container" | "host" | "device"
    SubstrateNodeID string      // which substrate instance performed it
    Verification    Hash        // hash of verification output (mount check, etc.)
}

// ImportEvidence proves that opaque legacy state was imported as an
// initial state. This is the "opening balance" — it can't be proven from
// first principles, but it's recorded as a typed starting point.
type ImportEvidence struct {
    SourceImageHash  Hash        // hash of the source image (e.g., data.img)
    ExtractionMethod string      // "mount-ro" | "copy" | "tar-extract"
    FsckStatus       string      // result of filesystem check
    ExtractionTime   uint64      // when extraction occurred
    ClassifierVersion string     // version of the path classifier
    IncludedPaths    []string    // paths classified into typed ledgers
    ExcludedPaths    []string    // paths excluded (opaque/runtime-only)
    DerivedLedgerRoots map[LedgerKind]Hash // roots derived from import
    Operator         AgentID     // who authorized the import
    Confidence       string      // "high" | "medium" | "opaque"
    Caveat           string      // human-readable caveat about trust
}
```

### Verifier results (completed per reviewer feedback)

```go
// VerifierResult records the outcome of a verification check.
// v1: expanded from just Passed bool to include contract ref, I/O hashes,
// verifier code ref, and failure semantics.
type VerifierResult struct {
    Verifier        string      // verifier name/capsule ID
    Kind            string      // "read-only-capsule" | "equivalence-check" | "smoke-test"
    ContractRef     Hash        // hash of verifier contract spec (v1)
    InputStateRoot  Hash        // ComputerState hash given as input (v1)
    OutputArtifactHash Hash     // hash of verifier output artifact (v1)
    VerifierCodeRef Hash        // hash of verifier code (for reproducibility) (v1)
    Passed          bool
    FailureSemantics string     // "hard-fail" | "soft-warn" | "inconclusive" (v1)
    Evidence        Hash        // hash of full verifier output (logs, diffs, etc.)
    Duration        uint64      // milliseconds
}
```

### Promotion certificate

```go
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
∀ ledger L in T.Deltas:
  1. state_at(P)[L] == T.Deltas[L].Before     (chain consistency)
  2. reachable(T.Deltas[L].Before,
               T.Deltas[L].After,
               T.Deltas[L].Evidence)            (evidence validity)
  3. state_at(T)[L] == T.Deltas[L].After        (materialization consistency)
```

**Per-ledger reachability definition (v1):**

| Ledger | Reachability definition |
|---|---|
| Dolt | `After` is a descendant of `Before` (commit ancestry) |
| git | `After` is a descendant of `Before` (commit ancestry) |
| Blobs | `After`'s set is a superset of `Before`'s set (set inclusion) |
| EROFS base | `After == verity_root(materialize(tape[0..T]))` (derived, not reachable) |
| Kernel/Rootfs/StoreDisk | `After == content_hash(materialize(tape[0..T]))` (derived) |
| Artifact graph | `After`'s graph includes all nodes/edges from `Before` (graph inclusion) |
| Route | atomic replacement (no reachability constraint, just equality) |

**Cross-ledger invariants (v1 addition):**

The audit also checks cross-ledger consistency:

```
4. If SourceSHA changed AND ErofsVerityRoot did not change → FAILURE
   (source changes require EROFS base rebuild)

5. If ErofsVerityRoot changed AND TapeSliceEnd != this.Hash → FAILURE
   (EROFS base must be built from the current tape head, not a stale one)

6. If RouteConfigHash changed AND HostAcknowledged == false → FAILURE
   (route changes require host acknowledgment)
```

### The audit (trial balance) — incremental O(n)

```go
// AuditTape walks the journal from genesis to head and verifies the
// conservation law for every entry. This is the "trial balance" — an
// automated check that the computer's books balance.
//
// v1: incremental replay — O(n) time, O(1) space for state tuple.
// The previous version called materializeState(entry.Parent) inside the
// loop, which was O(n²).
func AuditTape(journal Journal, head Hash) (AuditResult, error) {
    // Step 1: verify hash chain (existing, O(n))
    entries, err := journal.VerifyChain(head)
    if err != nil {
        return AuditResult{}, err
    }

    // Step 2: incremental replay with conservation law check
    state := ComputerState{}  // genesis state (all zero hashes)
    result := AuditResult{}

    for _, entry := range entries {
        // Build a map of which ledgers changed in this entry
        changed := make(map[LedgerKind]bool)
        for _, delta := range entry.Deltas {
            changed[delta.Ledger] = true

            // Check 1: Before matches current state (O(1))
            if state.Get(delta.Ledger) != delta.Before {
                result.Failures = append(result.Failures, AuditFailure{
                    Entry:  entry.Hash,
                    Ledger: delta.Ledger,
                    Reason: "before hash does not match parent state",
                })
                continue
            }

            // Check 2: After is reachable from Before via evidence
            if !verifyReachable(delta.Before, delta.After, delta.Evidence) {
                result.Failures = append(result.Failures, AuditFailure{
                    Entry:  entry.Hash,
                    Ledger: delta.Ledger,
                    Reason: "after hash is not reachable from before via evidence",
                })
                continue
            }

            // Update state incrementally (O(1))
            state.Set(delta.Ledger, delta.After)
        }

        // Check 3: cross-ledger invariants
        if changed[LedgerSource] && !changed[LedgerErofsBase] {
            result.Failures = append(result.Failures, AuditFailure{
                Entry:  entry.Hash,
                Reason: "source changed but EROFS base did not rebuild",
            })
        }
        if changed[LedgerRoute] {
            for _, delta := range entry.Deltas {
                if delta.Ledger == LedgerRoute {
                    if ev, ok := delta.Evidence.Route.(*RouteUpdateEvidence); ok {
                        if !ev.HostAcknowledged {
                            result.Failures = append(result.Failures, AuditFailure{
                                Entry:  entry.Hash,
                                Reason: "route changed but host did not acknowledge",
                            })
                        }
                    }
                }
            }
        }
    }

    // Step 3: verify final state matches materialization (optional, expensive)
    // This is the "full trial balance" — only run on demand or at promotion.
    if result.VerifyMaterialization {
        materialized := materializeAllLedgers(head)
        if materialized != state {
            result.Failures = append(result.Failures, AuditFailure{
                Entry:  head,
                Reason: "materialized state does not match replayed state",
            })
        }
    }

    return result, nil
}
```

**Complexity:** O(n) time for chain verification + incremental replay.
O(1) space for the state tuple. Materialization verification (step 3) is
optional and expensive — run only at promotion boundaries or on demand.
Snapshots every k entries enable O(k + evidence) head audit from a trusted
checkpoint.

### The state vector at any tape head

```go
// ComputerStateAt returns the complete state vector of the computer at
// the given tape head. This is the "balance sheet" — the position of
// every ledger. O(n) in chain length, O(1) in state size.
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
- No opaque route refs

This means: **any substrate that can produce the same ledger hashes is
equivalent.** The tape doesn't say "this state was produced by Firecracker
VM #42." It says "this state has Dolt commit X, git SHA Y, blob set Z,
EROFS verity root W, route config R."

Whether that was materialized on a Firecracker VM, a Cloud Hypervisor VM,
a Docker container, a local process, or a phone — the audit is the same.
The substrate is transparent because the state representation is
content-addressed, not substrate-addressed.

**Capability-scoped equivalence (v1 refinement):** two substrates are
equivalent for a given capability if the ledgers relevant to that
capability match. Full-computer equivalence requires all ledgers to match.
This allows partial equivalence — e.g., two substrates may agree on
Dolt/source/blobs but differ on kernel version, making them equivalent for
app-state purposes but not for VM/runtime purposes.

### Compensation (rollback) — forward deltas, not blind inverse

The tape is append-only. A failed transition is not deleted — a compensating
transaction is appended. **v1 fix:** compensation is a forward transaction
from current state to target state, not a blind inverse of the target entry.
If later transactions touched the same ledger, the current head is not
`target.After`, so blind inverse deltas would be incorrect.

```go
// Rollback appends a compensating transaction that moves the ledgers
// from their CURRENT state to the target entry's state. This follows
// the saga pattern: failures produce more transactions, not rollbacks
// of distributed state.
//
// v1: forward deltas from current → target, with conflict detection.
// v0 incorrectly used target.After → target.Before (blind inverse).
func Rollback(
    journal Journal,
    targetEntry Hash,    // the entry to roll back to
    author AgentID,
    reason string,
) (Hash, error) {
    // 1. Get current state and target state
    currentState := ComputerStateAt(journal, journal.Head())
    targetState := ComputerStateAt(journal, targetEntry)

    // 2. Detect conflicts: if any ledger diverged between target and
    //    current due to later transactions, we need merge/rebase, not blind revert
    conflicts := detectConflicts(journal, targetEntry, currentState, targetState)
    if len(conflicts) > 0 {
        // For git: create a revert commit (not git reset --hard)
        // For Dolt: dolt revert or checkout + new commit
        // For EROFS: mount old base + DROP overlay upper
        // For blobs: no-op (immutable), set membership reverts
        // For route: host updates to target route config
        return Hash{}, fmt.Errorf("rollback conflicts: %v", conflicts)
    }

    // 3. Build forward deltas: current → target
    deltas := buildForwardDeltas(currentState, targetState, journal, targetEntry)

    // 4. Append the compensating transaction
    rollbackTx := MutationTransaction{
        Parent:    journal.Head(),
        Sequence:  journal.NextSequence(),
        Timestamp: now(),
        Author:    author,
        Kind:      TxKindRollback,
        Message:   fmt.Sprintf("rollback to %s: %s", targetEntry, reason),
        Deltas:    deltas,
    }

    return journal.Append(rollbackTx)
}
```

**Per-ledger compensation mechanisms:**

| Ledger | Compensation | Append-only? |
|---|---|---|
| Dolt | `dolt checkout <target> && dolt commit` (new commit, not reset) | Yes |
| git | `git revert <range>` (revert commit, not reset --hard) | Yes |
| Blobs | no-op (immutable); set membership reverts via set root | Yes |
| EROFS base | mount old base (verity root = target) + **drop overlay upper** | Yes (base is immutable, overlay is ephemeral) |
| Kernel/Rootfs/StoreDisk | mount old images (content hash = target) | Yes (immutable images) |
| Artifact graph | revert to old graph root (graph is append-only; use old root) | Yes |
| Route | host updates route to target config | Yes (host records new acknowledgment) |

**EROFS compensation must also drop the overlay upper layer** (v1 fix from
review): reverting the verity root without dropping the overlay leaves writes
from the failed transaction. The `ErofsBuildEvidence.OverlayDiffHash` field
tracks which overlay diff was included in each build, enabling correct
compensation.

### The self-transition flow with multi-ledger transactions

```
1. Agent spawns capsule with own overlay + Dolt branch
2. Capsule runs experiment (writes to overlay, commits to Dolt branch)
3. Agent verifies results in read-only capsule
4. Agent builds MutationTransaction (saga step 1 — ledger commits):
   a. Compute overlay diff → file manifest entries + new blobs
   b. Merge Dolt branch → record DoltMergeEvidence (with TableHashes)
   c. Commit git changes → record GitCommitEvidence (with AuthorID/CommitterID)
   d. Add new blobs → record BlobSetEvidence (with BlobMetadataRefs)
   e. Record NoOpEvidence for unchanged ledgers (kernel, rootfs, route)
5. Agent commits transaction to journal (tape append)
6. Agent materializes new EROFS base from updated tape (saga step 2):
   a. StateGenerator.Generate(tape_head, stagingDir)
   b. ErofsImageBuilder.Build(stagingDir, base.erofs)
   c. VeritySealer.Seal(base.erofs) → verity root
   d. Record MaterializationEvidence
7. Agent appends EROFS build transaction (saga step 2 — erofs_base delta):
   a. Before = old verity root, After = new verity root
   b. Evidence = ErofsBuildEvidence with TapeSliceStart/End
   c. Cross-ledger invariant: source changed → EROFS must change (checked)
8. Agent swaps base (unmount old, mount new, reset overlay)
9. Audit: verify conservation law holds for all entries
```

Steps 4-5 commit the Dolt/source/blob deltas. Steps 6-7 materialize the new
EROFS base and record the erofs_base delta. These are **separate saga steps**
because:
- EROFS materialization may fail (Dolt commit survives as valid branch point)
- The EROFS build transaction's hash depends on the prior transaction's hash
  (no circularity — v0 had the EROFS evidence in the same transaction, creating
  a hash circularity)
- Materialization is substrate-specific; ledger commits are substrate-independent

### Migration from opaque data.img

Existing autoputers have opaque `data.img` that cannot be proven to derive
from any tape. Migration is an **import transaction** — the "opening balance"
for the computer.

**v1 fix: split import into two transactions to avoid circularity.**

```
Transaction 1 (import): records Dolt, source, blob deltas
  - Before: all zero (genesis)
  - After: imported Dolt commit, git SHA, blob set root
  - Evidence: ImportEvidence with source image hash, extraction method, fsck
  - EROFS base delta: NOT included (avoid circularity)

Transaction 2 (materialize): records EROFS base built from imported state
  - Before: zero (no prior EROFS base)
  - After: verity root of the materialized base
  - Evidence: ErofsBuildEvidence with TapeSliceStart=genesis, TapeSliceEnd=tx1
  - This transaction's hash depends on tx1's hash, not its own (no circularity)
```

```go
// ImportLegacyDataImg creates two tape entries that import the contents of
// an opaque data.img as the initial state.
func ImportLegacyDataImg(
    journal Journal,
    dataImgPath string,
    doltCommit Hash,
    sourceSHA Hash,
) (Hash, error) {
    // 1. Walk data.img, classify into ledgers
    blobs, blobSetRoot, doltTables, err := classifyLegacyImage(dataImgPath)

    // 2. Transaction 1: import Dolt/source/blobs (no EROFS — avoid circularity)
    importTx := MutationTransaction{
        Parent:   journal.Head(),
        Sequence: journal.NextSequence(),
        Kind:     TxKindImport,
        Message:  "import legacy data.img as initial state (ledgers)",
        Deltas: []LedgerDelta{
            {Ledger: LedgerDolt, Before: ZeroHash, After: doltCommit,
             Evidence: DeltaEvidence{Dolt: &DoltMergeEvidence{
                 Branch: "legacy-import", MergeBase: ZeroHash,
                 ConflictResolution: "opaque-import",
                 MergeCommit: doltCommit, TableHashes: doltTables,
             }}},
            {Ledger: LedgerSource, Before: ZeroHash, After: sourceSHA,
             Evidence: DeltaEvidence{Source: &GitCommitEvidence{
                 Branch: "legacy-import", ParentSHA: ZeroHash,
                 CommitSHA: sourceSHA, AuthorID: "import-operator",
                 CommitterID: "import-operator",
             }}},
            {Ledger: LedgerBlobs, Before: ZeroHash, After: blobSetRoot,
             Evidence: DeltaEvidence{Blobs: &BlobSetEvidence{
                 Added: blobs, SetMerkleRoot: blobSetRoot,
             }}},
            {Ledger: LedgerKernel, Before: ZeroHash, After: ZeroHash,
             Evidence: DeltaEvidence{NoOp: &NoOpEvidence{Reason: "kernel unchanged during import"}}},
            // ... other NoOps for unchanged ledgers
        },
    }
    importTxHash, err := journal.Append(importTx)
    if err != nil { return Hash{}, err }

    // 3. Materialize EROFS base from the imported tape state
    stagingDir, erofsPath, verityRoot, imageHash, err := materializeErofs(journal, importTxHash)

    // 4. Transaction 2: record EROFS base materialization
    materializeTx := MutationTransaction{
        Parent:   importTxHash,
        Sequence: journal.NextSequence(),
        Kind:     TxKindImport,
        Message:  "materialize EROFS base from imported state",
        Deltas: []LedgerDelta{
            {Ledger: LedgerErofsBase, Before: ZeroHash, After: verityRoot,
             Evidence: DeltaEvidence{ErofsBase: &ErofsBuildEvidence{
                 TapeSliceStart: ZeroHash, // genesis
                 TapeSliceEnd:   importTxHash,
                 BuilderVersion: "mkfs.erofs-1.8",
                 BuilderFlags:   "--fixed-time=0 --quiet",
                 ImageHash:      imageHash,
                 VerityRoot:     verityRoot,
                 OverlayDiffHash: ZeroHash, // no overlay at import
             }}},
        },
    }
    return journal.Append(materializeTx)
}
```

The import is auditable: it records exactly what was imported, how, when,
and with what confidence. The `Before` hashes are zero (genesis). The `After`
hashes are the imported state. The evidence is `ImportEvidence` — we trust
the existing state as a starting point, and everything after this is fully
audited.

## Relationship to existing types

### Journal (internal/base/journal)

The existing journal's hash chain is preserved. `MutationTransaction` is a
new entry type that extends the journal's event schema. The existing
`VerifyChain()` continues to work — it checks the hash chain. The new
audit (conservation law check) is an additional verification layer on top
of the existing chain verification.

**Coexistence:** existing event types (file_manifest, blob_set) are a
special case of `MutationTransaction` with only `LedgerBlobs` and a
synthetic file-manifest ledger delta. Migration is gradual: new code uses
`MutationTransaction`, old code continues to produce existing events, and
the auditor handles both.

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

**Capability-scoped equivalence (v1):** two computers may be compared on
a subset of ledgers (e.g., Dolt + source only) for capability-scoped
equivalence, without requiring full-computer equivalence.

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

## Canonical encoding

**DAG-CBOR** (IPLD) is the canonical encoding for `MutationTransaction`.
This ensures:
- Deterministic encoding (same transaction → same bytes → same hash)
- Content-addressed links (CID tags for parent hash and ledger refs)
- Cross-language compatibility (CBOR decoders exist for Go, Rust, JS, Python)
- Alignment with the IPLD/Merkle-DAG framing

**v1 decision:** use DAG-CBOR for new `MutationTransaction` entries only.
Existing journal entries keep their current encoding. The journal's
`VerifyChain()` handles both encodings (dispatched by entry type).

Do NOT use `encoding/json` as the canonical ledger encoding — it is
acceptable for today's internal structs but too weak as the canonical
ledger encoding contract (no canonical key ordering, no binary efficiency,
no CID support).

Explicit `HashAlgorithm` and `CIDVersion` fields should be included in
the transaction envelope to support future hash algorithm migration.

## Mutation class

Orange — adds new types, new audit logic, new journal entry schema. Does
not change existing journal hash chain verification. Does not change
existing `StateGenerator` file manifest generation (extends it). Rollback
path: don't use `MutationTransaction` entries; existing journal entries
continue to work.

## Concrete code changes

### New files

- `internal/computerversion/mutation_transaction.go` — `MutationTransaction`,
  `ComputerState`, `LedgerDelta`, `TxKind`, `LedgerKind`
- `internal/computerversion/mutation_evidence.go` — all evidence types
  (Dolt, git, blobs, EROFS, kernel, rootfs, store, graph, route, noop,
  materialization, import)
- `internal/computerversion/mutation_audit.go` — `AuditTape`,
  `ComputerStateAt`, incremental replay, cross-ledger invariant checks
- `internal/computerversion/mutation_audit_test.go` — tests for audit
  (conservation law, cross-ledger invariants, incremental replay)
- `internal/computerversion/compensation.go` — `Rollback` with forward
  deltas, conflict detection, per-ledger compensation
- `internal/computerversion/compensation_test.go` — tests for rollback
- `internal/computerversion/import_evidence.go` — `ImportEvidence` type,
  `ImportLegacyDataImg`, two-transaction import flow
- `internal/computerversion/import_test.go` — tests for import
- `internal/computerversion/dag_cbor.go` — DAG-CBOR canonical encoding
  for `MutationTransaction`

### Modified files

- `internal/computerversion/types.go` — add `ComputerState` to `Realization`,
  add `Artifacts` field
- `internal/computerversion/state_generator.go` — extend `Derive` to walk
  `LedgerDelta` entries, produce `ComputerState`
- `internal/computerversion/equivalence.go` — add `ComputerState` tuple
  comparison, capability-scoped equivalence
- `internal/base/journal/journal.go` — add `MutationTransaction` as new
  entry type, dispatch by type in `VerifyChain`

## Security considerations

### Threat model

1. **Malicious agent inside the VM** tries to forge a transaction
   - Defense: every transaction is hash-chained to the parent. Forging
     requires breaking SHA-256 or stealing the journal signing key.
   - Defense: the conservation law check detects any ledger state that
     doesn't match the chain.

2. **Malicious agent tampers with EROFS base**
   - Defense: dm-verity seals the base. Any byte modification changes the
     verity root, which is recorded in the tape. The kernel enforces
     dm-verity at the block layer — the agent cannot bypass it without
     kernel exploits.

3. **Malicious agent replays old transactions**
   - Defense: the hash chain includes sequence numbers and timestamps.
     Replay produces a different chain (different parent hash).

4. **Malicious host tampers with the journal**
   - Defense: the journal is tamper-evident (hash chain). Tampering
     breaks the chain. The agent can detect this by verifying the chain.
   - Limitation: if the host controls the agent's runtime, it can
     suppress verification. This is a fundamental limitation — the host
     is the substrate.

5. **Malicious capsule produces fraudulent evidence**
   - Defense: evidence types are typed and verified. `DoltMergeEvidence`
     is verified by checking Dolt commit ancestry. `GitCommitEvidence` is
     verified by checking git commit ancestry. `ErofsBuildEvidence` is
     verified by re-materializing and comparing verity roots.
   - Defense: `VerifierResult` includes `VerifierCodeRef` — the verifier
     code itself is content-addressed, so a fraudulent verifier can be
     detected by comparing its hash to the expected verifier.

6. **Rollback attack: attacker replays an old compensating transaction**
   - Defense: compensation is a forward transaction from current state,
     not a blind inverse. Replaying an old compensation would fail the
     conservation law (Before hash wouldn't match current state).

7. **Import attack: attacker imports a malicious data.img**
   - Defense: `ImportEvidence` records the source image hash, extraction
     method, and classifier version. The import is auditable — you can
     verify what was imported and how.
   - Limitation: the import is an "opening balance" — you can't prove
     the imported state was derived from first principles. This is
     inherent to migration from opaque state.

8. **Route hijacking: attacker changes the route config**
   - Defense: `RouteUpdateEvidence` requires `HostAcknowledged == true`.
     The host must confirm the route change. The agent cannot change
     the route unilaterally.
   - Defense: `RouteConfigHash` is content-addressed, so the tape records
     exactly what the route should be. Any discrepancy between the tape
     and the actual route is detectable.

### Trust boundaries

```
┌─────────────────────────────────────────────────┐
│  Agent (inside VM, untrusted)                    │
│  ┌───────────────────────────────────────────┐  │
│  │  Capsule (inside VM, untrusted)           │  │
│  │  ┌─────────────────────────────────────┐  │  │
│  │  │  Experiment code (untrusted)        │  │  │
│  │  └─────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────┘  │
│  Evidence produced here is VERIFIED, not trusted │
└─────────────────────────────────────────────────┘
          ↓ vsock/HTTP (authenticated) ↓
┌─────────────────────────────────────────────────┐
│  Host (trusted substrate)                        │
│  ┌───────────────────────────────────────────┐  │
│  │  Journal (tamper-evident hash chain)      │  │
│  │  EROFS base (dm-verity sealed)            │  │
│  │  Route table (host-managed)               │  │
│  └───────────────────────────────────────────┘  │
│  Audit runs here — conservation law enforced     │
└─────────────────────────────────────────────────┘
```

The agent and capsules are **untrusted**. Evidence they produce is
**verified** by the audit, not trusted. The host is the **trusted
substrate** — it enforces dm-verity, manages the journal, and
acknowledges route changes.

## Open questions

1. Should `MutationTransaction` support partial commits (some ledgers
   commit, others are deferred)? (Proposal: no. Each transaction is
   atomic across all included ledgers. Deferred ledgers use `NoOpEvidence`.)

2. How are concurrent capsules coordinated? If two capsules produce
   conflicting deltas to the same ledger, who wins? (Proposal: the
   journal is append-only and linearized. The first commit wins. The
   second capsule must rebase or produce a compensating transaction.)

3. What is the canonical encoding of `RouteConfig` for hashing?
   (Proposal: JSON with sorted keys, per RFC 8785. Alternative: DAG-CBOR.)

4. How does the `artifact_graph` ledger work in detail? Is it a Merkle
   DAG like IPLD? What's the graph root hash? (Needs separate design.)

5. Should the `ComputerState` tuple be stored as a snapshot at each
   tape head, or always derived by replay? (Proposal: derived by
   replay. Snapshots are an optimization, not a correctness mechanism.
   The tape is the source of truth.)

6. How does `MaterializationEvidence` interact with cross-substrate
   proof? If the same tape is materialized on two substrates, do they
   produce the same `MaterializationEvidence`? (Proposal: the
   `Substrate` and `SubstrateNodeID` fields differ, but the
   `LedgerState` hash is the same. This proves substrate independence.)

7. What happens if the host refuses to acknowledge a route change?
   (Proposal: the transaction fails with `HostAcknowledged == false`.
   The agent can retry or escalate to the owner.)

8. How are `VerifierResult` contracts defined and versioned? Is there
   a registry of verifier contracts? (Needs separate design.)
