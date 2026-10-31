# Branching tape: a tree of linear chains, joined by publication (2026-10-31)

Status 2026-10-31: design only, nothing built. Mutation class of this
document: green. The changes it orders are orange to red. **The doctrine
delta in §4 needs owner ratification.**

Owner, 2026-10-31: "we may need to start branching our tape now... There
are going to be edge-case states where both computers are running
simultaneously, even in the best case. Maybe we should design it so you
could have a VM running on the host and one running on your local machine
simultaneously, intentionally. And be prepared for forking a VM on the
host, or forking it locally." Then: "We might still maintain merging
through publication, even if we do have, in some sense, a branching
tape... remember our four S's: security, stability, simplicity, and
scalability. In this case simplicity is probably the most important."

Sources: [computer ontology](../computer-ontology.md) (forked computer,
realization, capsule), [one tape](one-tape-durability-2026-10-31.md) §3-§5,
[object graph redesign](object-graph-redesign-2026-10-30.md) §5.4 and §11,
[data system frame](choir-data-system-frame-2026-10-30.md) (single writer,
fenced), [hypervisor seam](../problems/vmmanager-has-no-hypervisor-seam-2026-10-21.md).

## 0. The recommendation in one paragraph

Branching happens **between** chains, never **inside** one. Each ComputerID
keeps one linear, hash-chained tape (`sequence`, `previous_head`, host CAS on
`canonical_event_head`). The "branching tape" is the tree those chains form
through **lineage**: a fork's genesis names a point on its parent's chain
and shares the parent's prefix by reference. Concurrency on one computer is
prevented by a **writer lease** (the planned writer epoch, made a lease).
When prevention fails (split-brain), the losing suffix is kept as an
**orphan branch** and can be turned into a fork. Results return to the
computer only by publication and adoption, never by automatic state merge.
There are no merge events, no vector clocks and no CRDT reducers.

```text
P (computer, one chain)   e1─e2─e3─e4─e5─e6─e7        writer: lease epoch 7
                                  │        └─ orphan o5'─o6' (epoch 6, Mac, demoted)
                                  │              └─(owner act)→ fork F2
F1 (fork, own ComputerID)        g1─f2─f3   genesis cites (P, seq 3, digest e3)
F1 publishes object X ──► Autopaper graph ──► P adopts X (an ordinary event on P)
```

## 1. The cases

| Case | What it is | Handling |
|---|---|---|
| A. Accidental split-brain during a move | Two realizations of one ComputerID both append, briefly. Possible today: each host has its own `computer_event_heads` row, and `bundle import` refuses a non-chaining range (`internal/bundle/import.go:118-131`) but nothing stops the old side from serving | Prevented by the lease (§2.3). When it happens anyway, the side whose receipts carry the current epoch is the computer; the other suffix becomes an orphan branch (§5.2) |
| B. Intentional replicas of one computer (Node B and the Mac) | One identity, one owner, two realizations running at once | **One writer, one or more followers.** The lease holder appends; a follower serves reads from bundle deltas and takes writes only after a handoff. Offline Mac: the Mac checks the lease out before going offline; the host follows. If both must write while apart, that is a fork (case C) by definition |
| C. A fork on the host or locally | A sibling computer (ontology, 2026-10-01): new ComputerID, own chain, fresh identity, lineage to a parent checkpoint | Unchanged in doctrine. New here: the lineage record, prefix sharing by reference, and the snapshot key inheritance (§2.4) |
| D. A speculative branch inside one computer | A self-development capsule producing a `CapsuleEffectBundle` | **Does not become a tape branch.** A capsule owns no event ordering (ontology, Self-Development Candidate Contract); its bundle returns by an accepted event. It already has the shape this design generalizes: a branch returns only through an artifact plus an authority act. Unify the rule, not the data structure |

## 2. Data structures

### 2.1 Events and heads: unchanged

`computerevent.Event` keeps a single `PreviousHead` and a per-computer
`Sequence`; `Reduce` keeps its `Sequence+1` and `PreviousHead ==
CanonicalEventHead` check (`internal/computerevent/reducer.go:69`). No
parent list, no replica id in the event body. Reasons:

- `Event.CanonicalBytes` serializes every field, so a new field changes the
  digest of every new event and needs `SchemaVersion` 2 and a second decode
  root (`decode.go`). That cost buys nothing without merge events.
- The reducer's desired/effective/pending state is a single register with
  one pending transition (`reducer.go:79-140`). It cannot merge two pending
  self-development transitions; a DAG would need a per-register lease
  anyway.

"The head" of a computer stays one value: the `canonical_event_head` whose
receipt carries the current writer epoch.

### 2.2 Who wrote it: the receipt, not the event

The writer's identity is the pair (host signer key, writer epoch). Each host
already signs an `EventHeadReceipt` per append with its `platform-control`
key (`internal/platform/computer_events.go:257-262`). Add `writer_epoch` to
`eventHeadReceiptFields` (`computer_events.go:352`). The fields are a map,
so old receipts keep verifying. Two valid receipts for the same
(computer_id, sequence) with different `event_digest` are then a signed
**equivocation proof** that names both writers. That is the split-brain
detector, and it costs one field.

A realization ("replica") has no identity of its own beyond (host key id,
epoch, realization id from vmctl). The computer's identity (ComputerID, the
computer key, the DEK) stays one thing and travels with the lease (Keychain
on a Mac, redesign §11).

### 2.3 The writer lease

One row per computer, in exactly one authority: the control database of the
computer's home host (Node B for every computer today).

```text
computer_writer_leases(computer_id PK, epoch BIGINT, holder_host_key_id,
  holder_realization_id, state {held, handing_off, checked_out, revoked},
  acquired_at, receipt_json)
```

- **Append check:** `compareAndSwapTx` reads the lease in the same
  serializable transaction and refuses when the capability's epoch is not the
  lease's epoch. The order from the frame stays: fence (bump the epoch), stop
  the old writer, read the head, start the new one.
- **Checkout, not expiry.** A Mac that goes offline holds a checked-out lease
  with no timer. A timer would let the home host retake the lease while the
  Mac still writes, which manufactures case A. The home host refuses to start
  a writer for a checked-out computer.
- **Revoke is an owner act.** If the Mac is lost, the owner revokes; the
  epoch bumps; anything the Mac wrote after its last sync becomes an orphan
  branch at the next contact. A revoke is a declared split, not a silent one.
- **Followers** hold no lease. They import deltas (`choir-bundle export
  --since`) and serve reads. A follower that receives a write either forwards
  it to the holder or refuses it; it never appends.
- A fork's chain gets its own lease row (it is a different computer).

### 2.4 Lineage and prefix sharing

```text
computer_lineage(child_computer_id PK, parent_computer_id,
  parent_sequence, parent_head_digest, parent_snapshot_root,
  kind {fork, orphan_fork}, created_by_receipt, created_at)
```

- A fork's genesis is an ordinary `genesis_imported` (sequence 1,
  `previous_head = ZeroHead`), so `reduceGenesis` is unchanged. The lineage
  rides in its payload and `InputArtifactRefs` (the parent snapshot root and
  the parent head receipt). No schema change.
- **Recovery** of a fork is its own snapshot plus its own tail. If the fork
  has no snapshot, it is the parent's snapshot at or before
  `parent_sequence`, plus the parent's tape to `parent_sequence`, plus the
  fork's tape. So the parent's prefix must outlive the parent while a child
  cites it: the tape sweep must read `computer_lineage`, replacing today's
  `AcceptUnrecordedForkLineage` gate (`internal/platform/tape_sweep.go:141-189`,
  "no fork-lineage store exists").
- **Snapshot chunks.** `filecas.SealChunk(dek, computerID, data)`
  (`internal/snapshot/snapshot.go:435`) binds every chunk digest to the DEK
  and the ComputerID, so a re-keyed fork shares no ciphertext with its parent.
  Without a change, each fork costs one full snapshot (about 25 GB for the
  paper). The fix: a snapshot manifest entry may name `(key_ref, digest)`.
  The fork receives a wrapped, read-only copy of the parent's DEK for the
  roots it inherits, and new page ranges seal under its own DEK. The parent
  rotates its DEK at the fork point, so the fork can never read the parent's
  later chunks. The fork already held that plaintext, so it gains no access
  it lacked. Its signing identity is still freshly minted.

### 2.5 Sequences, idempotency, receipts, segments

All stay per computer and linear. `computer_event_append_receipts` keeps
`UNIQUE (computer_id, sequence)` and `PRIMARY KEY (computer_id,
idempotency_key)`. Orphan branches never enter that table. They live in:

```text
computer_tape_branches(computer_id, branch_id PK, fork_point_sequence,
  fork_point_digest, head_sequence, head_digest, writer_epoch,
  signer_key_id, bundle_ref, status {orphaned, forked, discarded},
  keep_set_bearing BOOL, discovered_at)
```

`bundle_ref` points to a sealed bundle of the suffix (format 2 objects
pack). The working store, receipts table and segments of the computer stay
single-chain, and so do every reader, replay and witness.

## 3. Merge, per projection kind

There is no automatic merge. Per kind, this is what crossing a branch means:

| State | Under the lease (B) | Across a fork or orphan (A, C) |
|---|---|---|
| Append-only logs, telemetry, run memory, traces | one writer, nothing to merge | stays on its branch as evidence; never copied |
| Records keyed by content id (files, published objects) | n/a | a publication; the receiver adopts by digest (dedup is free) |
| Cells, counters, leases, queues (`cell.set`, `queue.consume`) | n/a | never merged: two consumes would be two external effects |
| Texture canonical revisions | one Texture desk writes, on the lease holder | a fork's Texture writes the fork's documents; they return as a published revision the parent's Texture adopts, so it stays the sole writer |
| Desired/effective code state (reducer) | one pending transition at a time | never merged; a fork's code returns as a package or effect bundle that is accepted on the parent |
| External effects already done by a loser (mail sent, paper published) | n/a | cannot be undone; listed on the orphan object from its tape |

Desk authority is unchanged: management, engineering, research and app
desks run only on the lease holder. A follower runs no desks, which also
satisfies "a crash restart never resumes work" without a second rule.

**The evaluated, not recommended option: replicas merge automatically.**
Events would name `parents[]`, carry a hybrid logical clock and a replica
id, keep a per-replica sequence and a version vector per head, and replay
in a deterministic topological order (Kahn, ties by digest). Logs and
content-keyed sets would merge as CRDT unions. Cells would merge by
last-writer-wins on the clock, Texture and the reducer by a per-desk or
per-register lease, and queues not at all. Working it through, it falls
back to per-kind leases for every kind that matters, and adds a merge
witness, equivocation as a normal event, and silent merge bugs. It also
contradicts settled doctrine (§4). Scores below.

## 3a. The four S's, per mechanism

Each mechanism is scored in the owner's order (+ better, 0 neutral, − worse
than today's linear chain):

| Mechanism | Security | Stability | Simplicity | Scalability |
|---|---|---|---|---|
| Writer lease (checkout, no expiry) | + one signer per epoch; equivocation provable | + split-brain prevented, not repaired | 0 one row, one check | 0 one home authority per computer |
| Follower realization | 0 reads only; holds the key (Mac Keychain) | + warm handoff, an off-host copy | 0 reuses bundle deltas | + reads served locally |
| Orphan branch on split | + losing writes kept as signed evidence | + no silent loss; the computer stays one chain | 0 one table, one bundle | 0 bounded by the split's length |
| Fork with lineage and prefix reference | 0 fresh signing identity; inherited read only to the fork point | + recovery independent of the parent once snapshotted | 0 genesis unchanged | + no prefix copy; chunk reuse once keyed by `key_ref` |
| Automatic replica merge (rejected) | − two signers per chain; a forged branch merges | − merge order and witness bugs, silent | − DAG, HLC, vectors, per-kind rules | + offline writes on both sides |

## 4. Doctrine delta (needs owner ratification)

Settled: a fork has its own ComputerID, chain and identity, records lineage,
and "its results reach other computers only through publication and
adoption" (`docs/computer-ontology.md`, forked computer).

- **Recommended, and consistent with that:** a replica is not a new object.
  It is a **realization** of the one computer. The ontology already says
  realizations are replaceable and "the user should not have to care which
  realization serves the computer". Two realizations may run at once, but
  only the lease holder writes.
- **New, needing ratification:**
  1. *Follower realizations*: a second, read-only realization serving at the
     same time as the writer.
  2. *Orphan branches, and forks made after the fact*: a demoted
     realization's suffix, written under the parent's ComputerID, is
     archived as evidence on the parent, and an owner act can make a fork
     from it. The fork's identity is minted at conversion, not at
     divergence. This is a second way forks come into being.
  3. *Inherited read keys*: a fork holds a wrapped read-only parent DEK for
     the chunks of roots it inherits, while its signing identity stays fresh.
     This refines "freshly minted identity material".
- **Rejected, and stated plainly:** automatic merge of concurrent replicas
  is a direct merge between chains. It contradicts the settled rule. Taking
  it would need the owner to retire that rule, and this design recommends
  not doing so.

## 5. Replay, restore, restart, sync, retention

### 5.1 Replay and witness

These stay linear. Replay order is sequence order; the content witness
(`LiveStoreWitness`, `agentcore/restore_snapshot.go:212`) is unchanged. A
fork's first state commitment is the parent snapshot's witness at the fork
point, cited in its genesis. Restoring to a point on a branch means
restoring to a point on that computer's chain: nearest snapshot at or below
it, then replay. An orphan branch is never restored in place. It is
materialized only as a fork, in a scratch directory, through the bundle
import path (§6a2 of one tape).

### 5.2 Sync and split detection

1. The follower sends `have = (computer_id, epoch, head_sequence,
   head_digest)`.
2. The holder looks up its receipt at `head_sequence`. If the digest
   matches, it answers with a delta bundle, paged by segment, with import
   committed in chunks and the head advancing last (one tape B4). Memory is
   bounded by the page, not the range.
3. On a mismatch, or a receipt under a stale epoch, find the divergence
   point by binary search over `(sequence, digest)`: O(log n) probes, no
   full scan. Return `diverged{fork_point, ours, theirs}` instead of a bare
   refusal.
4. The stale-epoch side exports `(fork_point, its head]` as a bundle, the
   home host records it in `computer_tape_branches`, and the stale side
   rebases onto the holder's chain through stage-and-swap recovery.
5. The owner sees one object: "N events written on <host> after <time> did
   not reach the computer", with the external effects listed and two
   actions: *make a fork* or *discard after retention*.

### 5.3 Restart rules

These hold per realization. A demoted writer closes its open obligations
with the fate "interrupted: writer lease moved", the same mechanism as a
crash. A handoff is planned, so it may carry the planned-restart marker to
the new holder: one resume per handoff, no loop. A follower has no
obligations to close.

### 5.4 Retention and storage growth

- The shared prefix is referenced, never copied. A fork costs its own tape
  plus the chunks that differ, once chunks carry `key_ref` (§2.4).
- An ephemeral fork is discarded by default (ontology). The sweep deletes its
  tape only when no `computer_lineage` row cites it as a parent below a live
  child's `parent_sequence`.
- An orphan branch is kept 30 days, then discarded, unless
  `keep_set_bearing` is set (it touches the owner's mail, Texture,
  precommitment records or newspaper content). Those wait for an owner act.

## 6. Do now (minimal), ranked by later pain avoided per unit of cost

| # | Change | Files | Cost | Pain avoided |
|---|---|---|---|---|
| 1 | Build S7 (the writer epoch) as the **lease** of §2.3: a `computer_writer_leases` row with the checkout state, checked in `compareAndSwapTx`; and `writer_epoch` in `eventHeadReceiptFields` (check that `event_receipt.go`'s verifier tolerates the extra field) | `internal/platform/computer_events.go:227-262,352`, `internal/platform/lifecycle_control.go`, `internal/platform/credential_envelope.go` | medium (already planned) | the whole of case A, and the basis of B and of signed equivocation proofs |
| 2 | `computer_lineage` table, written at fork creation; the tape sweep reads it instead of `AcceptUnrecordedForkLineage` | `internal/platform/tape_sweep.go:141-233`, new `internal/platform/computer_lineage.go` | small | deleting a prefix a fork's recovery needs (silent, irreversible) |
| 3 | Bundle import returns a structured divergence (`fork_point`, both digests) instead of a string refusal | `internal/bundle/import.go:100-131` | small | a later sync rebuilding divergence detection; the bare refusal loses where the chains split |

Not now: a parent list or replica id in `Event` (a schema version, for a
merge we do not do); `computer_tape_branches` (until the first split);
`key_ref` chunks (until forks are routine).

## 7. Failure modes first

- **Lease.** The home host's control DB is lost, so a lease is lost: no
  writer can start. That fails closed, and the owner reasserts the lease.
  A lease checked out to a Mac that never returns: the owner revokes. Clock
  skew does not matter (no expiry). A capability renewed across an epoch:
  forbidden by the frame rule, and the test is a stale-epoch append refused
  on staging.
- **Equivocation detection.** Two hosts with copies of the same
  `platform-control` key make the proof meaningless, so each host keeps its
  own key, verified against a known host key set (one tape B2).
- **Orphan branch.** It is captured after the stale side's disk is gone: the
  suffix is lost, and only the receipts on the other side (if any) remain.
  It holds an external effect the owner never learns about: the orphan object
  must list the effect kinds from the suffix's tape. It is deleted while
  keep-set-bearing: blocked by the flag.
- **Fork lineage.** The parent is swept below a child's fork point: blocked
  by do-now 2. A forged lineage in a genesis payload: the cited parent head
  receipt must verify.
- **Inherited keys.** The parent forgets to rotate its DEK at the fork, so
  the fork can read the parent's future chunks. The fork act must rotate
  first and refuse otherwise.
- **Follower.** A follower serves a stale read as current: reads carry the
  follower's head and lag. A follower takes a write: the append path refuses
  for want of a lease.

## 8. Open questions

1. Is the home authority always Node B, or can the lease authority itself
   move (an enterprise host, a Mac-only computer)? A Mac-only computer
   needs its own control DB as home.
2. Should a follower run desks read-only (research on the Mac while
   offline)? Research writes reports, so today that is a fork.
3. Is an offline Mac that must write while the host also writes (both
   needed at once) a real use, or is checkout enough? If it is real, it is
   the case where the rejected merge option returns, and it should return as
   per-kind leases plus content-keyed publication, not as a DAG.
4. Can the paper (always busy, `public_platform`) ever check its lease
   out? Probably never, since its schedules are standing intents on the
   home host.
5. Does the inherited read key (§4, item 3) satisfy "freshly minted
   identity material", or must a fork re-encrypt its inherited chunks
   (one full snapshot of cost)?
6. Where does an orphan-made fork's ComputerID come from, and does it get a
   route by default (ontology: an ephemeral fork has none)?
7. Do capsules eventually want tape-backed transaction logs (case D)? If
   so, they are chains with lineage like forks, not branches of the parent's
   chain.
