# Branching tape: branches of one computer, a primary ref, no merge (2026-10-31)

Status 2026-10-31: design only, nothing built. Mutation class of this
document: green. The changes it orders are orange to red. **The doctrine
delta in §10 needs owner ratification.** Revision 2 replaces the first
version (6401b01b8), in which the losing side of a split became a fork.

Owner, 2026-10-31:

- "There are going to be edge-case states where both computers are running
  simultaneously, even in the best case... be prepared for forking a VM on
  the host, or forking it locally."
- "We might still maintain merging through publication, even if we do have,
  in some sense, a branching tape... security, stability, simplicity, and
  scalability. In this case simplicity is probably the most important."
- "imagine a network issue where you end up having activity on both local
  and host. One of those two gets committed as primary, but you want the
  other one to really be the primary one. You shouldn't have to go through
  the whole publication process to make that happen."
- "First, we should be thinking about what would the circumstance be where
  that happens."

Sources: [computer ontology](../computer-ontology.md),
[one tape](one-tape-durability-2026-10-31.md) §3-§5,
[object graph redesign](object-graph-redesign-2026-10-30.md) §5.4 and §11,
[data system frame](choir-data-system-frame-2026-10-30.md) (single writer,
fenced).

## 0. The recommendation

A computer keeps one identity (ComputerID, keys) and may hold several
**branches**. Each branch is a linear hash chain from a common ancestor.
Exactly one branch is **primary**, named by a ref: computer → branch. Three
rules hold:

- **One writer per branch:** a writer lease per (computer, branch) with an
  epoch.
- **Switching primary is an owner act:** move the ref, fence the old
  writer, re-realize.
- **Branch contents never merge.** Combining work from two branches goes
  through publication and adoption.

Forks stay separate computers.

```text
computer C   main: e1─e2─e3─e4─e5─e6        (Mac, lease main@4)
                          └─ b7: h4─h5        (host, lease b7@1)   primary ──► b7
owner act "make main primary": fence b7, primary := main, host re-realizes from
snapshot ≤ e3 + replay main to e6. b7 stays as a retained, non-primary branch.
```

## 1. When does each side end up with activity?

The lease design (§2) assumes: the lease authority is the home host's
control database (Node B); the Mac can **check out** a lease (no expiry);
a follower never writes; a handoff fences first.

| # | Circumstance | Lease holder | Mac activity | Host activity | Owner wants as primary | Prevented by the lease? | Likelihood |
|---|---|---|---|---|---|---|---|
| 1 | Mac offline with the lease checked out (plane, café); the owner opens choir.news on the phone and **chooses to work there** | Mac (main) | owner in Texture, the owner's desks and agents | phone edits, mail triage, scheduled jobs of the personal computer | **usually the Mac** (most of the work), sometimes the host; it depends on the day | yes, if the phone stays read-only. A split happens only if the owner picks "work here anyway", which opens a host branch | **high**, the main real case |
| 2 | Lease revoked from the phone while the Mac sleeps or is offline (left at home, lost) | host after revoke (new branch); the Mac still believes it holds main | whatever ran before sleep; any work after waking offline | everything after the revoke | **usually the host** (a deliberate act); the Mac if it held unsynced Texture work the owner forgot | no: an offline Mac cannot learn of a revoke. Narrowed by revalidating the lease on wake whenever the network is up | medium-low |
| 3 | Network partition during a move (handoff in flight) | nobody between the fence and the grant | none | none | n/a | yes: fence first, then grant; the cost is a short unavailability, not a split | medium, harmless |
| 4 | Host restarts; a stale Mac follower thinks it is the writer | the host | none legitimately | normal | host | yes: a follower holds no lease and its host refuses appends. A handback counts only once the home host acknowledges it | low |
| 5 | Clock or expiry mistakes | n/a | n/a | n/a | n/a | yes, because the lease has no timer. Adding one would bring this case back | none, by design |
| 6 | A bug or an operator error: the home control database restored from an old backup (the epoch rolls back), a cold move done by hand, an import race | both write **the same branch** | some | some | whichever side carries the owner's work, usually decided by size | no. Detected by a signed equivocation proof (§3.3) | low, not zero |
| 7 | Deliberate: "try a risky reorganization on the Mac while the host keeps running; keep it wholesale if it works" | host on main; the Mac on its own branch | the experiment | normal | the experiment, if it worked | n/a: this is a branch by choice | low today |
| 8 | The paper and other `public_platform` and `critical_protected` computers | always the host | never checked out | 24/7 | host | yes: they never check out | ~none (only case 6) |

**What justifies a cheap switch.**

- Cases 1 and 2 are real. The owner's wish there is a judgment made after
  the fact, so a switch must be cheap and must lose nothing.
- Case 6 needs the same tool as a repair.
- Case 7 is a bonus.
- All of them are **personal computers, two branches, a tail of hours to
  days, mostly owner and desk activity plus a few external effects**.
  Always-busy computers never branch except by bug.

**Scope that follows (simplicity first).**

- Up to a few live branches per computer (a soft cap of 4), almost always
  2.
- Lineage depth is almost always 1.
- Four operations: switch primary, retire a branch, publish from a branch,
  fork from a branch.
- No merge.
- No concurrent writers on one branch.
- No automatic primary choice beyond a fixed default (the branch the home
  host serves, for route continuity), with the switch offered on the
  owner's surface at once.

## 2. Objects and invariants

- **Computer**: ComputerID, owner, keys. Unchanged, and shared by all its
  branches.
- **Branch**: `(computer_id, branch_id)`, with a base `(base_branch_id,
  base_sequence, base_digest)`. `main` has no base. Branch ids are UUIDv7,
  minted by the host that opens the branch, so no coordination is needed.
- **Primary ref**: `computer → branch_id`, with a generation. It routes the
  computer and is the default branch for every reader.
- **Writer lease**: `(computer_id, branch_id) → epoch, holder (host key id,
  realization id), state {held, checked_out, frozen}`. Only the holder of
  the current epoch appends to that branch.
- **Realization**: a VM serving exactly one branch, as writer or follower.

Invariants:

- I-a: at most one writer per branch per epoch.
- I-b: exactly one primary.
- I-c: no content crosses branches except by publication and adoption.
- I-d: **security-monotone facts are computer-wide, not per branch**: key
  revocations (`credential_revocation_epoch`), tombstones and lease epochs.
  A switch never un-revokes a key.

## 3. Data structures

### 3.1 Events: unchanged

`computerevent.Event` keeps one `PreviousHead` and a `Sequence`. An event
belongs to the branch whose chain it extends. Its `previous_head` already
pins it to one position, so a captured event of one branch cannot be
appended to another past the fork point. Adding a field would change every
digest (`CanonicalBytes` serializes all fields) and buys nothing here.
`Reduce` (`reducer.go:69`) is unchanged. Sequence is **per branch**: both
branches carry sequence 5 after a fork at 3.

The guest appender is unchanged too. `HeadCAS.Head(computerID)` and
`EventsPage(computerID, after)` answer for the branch named by the
realization's capability, so the guest never learns branch ids.

### 3.2 Host tables

```text
computer_branches(computer_id, branch_id, base_branch_id, base_sequence,
  base_digest, status {active, frozen, retired}, keep_set_bearing,
  opened_by_receipt, created_at)                    PK (computer_id, branch_id)
computer_primary_ref(computer_id PK, branch_id, generation, receipt_json)
computer_writer_leases(computer_id, branch_id, epoch, holder_host_key_id,
  holder_realization_id, state)                     PK (computer_id, branch_id)
computer_event_heads            PK (computer_id) -> (computer_id, branch_id)
computer_event_append_receipts  + branch_id DEFAULT 'main';
  UNIQUE (computer_id, sequence)  -> (computer_id, branch_id, sequence)
  PK (computer_id, idempotency_key) -> (computer_id, branch_id, idempotency_key)
  UNIQUE (computer_id, event_digest) stays (digests differ after the fork)
```

- **Shared prefix stored once.** A branch's rows are only those above
  `base_sequence`. Its chain is its own rows plus its base branch's chain to
  `base_sequence`. Pins, payloads and segment files are content-addressed,
  so they are never duplicated.
- **Idempotency is ancestry-aware.** A key counts as used on branch B if it
  is in B's rows or in an ancestor's rows at or below the base.
  `existingComputerEventReceipt` (`computer_events.go:314`) today keys on
  `(computer_id, idempotency_key)`. Unchanged, it would hand branch B a
  receipt from branch A for a deterministic key (`snapshot:<root>`), which
  is a silent wrong answer.
- The per-batch receipt table of redesign §5.4, not yet built, takes
  `branch_id` in its keys from the start. The old receipts table gets only
  the column now (in PostgreSQL, a constant default adds no rewrite). The
  key changes wait until the first branch is opened, built `CONCURRENTLY`
  (rule 5: do not rework the old system ahead of need).

### 3.3 Receipts and signatures

`eventHeadReceiptFields` (`computer_events.go:352`) gains `branch_id` and
`writer_epoch`. Each host signs with its own `platform-control` key,
verified against a known host key set (one tape B2). Two valid receipts for
the same `(computer_id, branch_id, sequence)` with different digests are a
signed equivocation proof (case 6). A Mac opening a branch signs the branch
record with its host key. The home accepts it from a host in the
computer's key set, and opening a branch never changes the primary.

### 3.4 Snapshots, witness, restore

- `computer_snapshot_roots` gains `branch_id` and `head_digest`. The guest
  record already carries `StoreHead` (`internal/snapshot/snapshot.go:89-91`).
  Today `RecoveryBase` (`file_cas.go:80`) and the retention query
  (`snapshot_roots.go:93,105`) order by `head_sequence` alone. **With two
  branches, that would restore branch A's state into branch B.** A root is
  usable for branch B only if `(head_sequence, head_digest)` lies on B's
  chain, either B's own or an ancestor's at or below the base. Retention
  counts per branch, and it keeps the newest root at or below each live
  fork point.
- The witness (`LiveStoreWitness`) is per branch, with no change: each
  branch replays linearly, and both branches agree on the shared prefix.
- Restore to a point means a point on a named branch (default: the
  primary).

## 4. Switching primary

Preconditions:

- an owner act receipt;
- the target branch T is fully imported on the home host, with a verified
  head;
- no self-development apply in progress and no live engineering capsule on
  the current primary P. These are correctness refusals, as in the restart
  rules.

The commit point is one serializable transaction in the home control
database:

```text
1  lease(P).epoch += 1, state = frozen          -- P's writer is fenced: its next CAS refuses
2  UPDATE computer_primary_ref SET branch_id=T, generation=g+1
       WHERE computer_id=C AND generation=g     -- CAS on the ref
3  lease(T): keep its holder if T's realization lives (the Mac), else
       epoch += 1, holder = the realization that will serve T
4  append the switch record (from P@head, to T@head, owner act, effects digest §6)
```

After the commit:

- **The realization on T, if one lives, keeps running.** No restart.
- **The realization that served P** is refused at its next append or
  capability renewal (the stale epoch). That refusal is its signal. Its
  open work belongs to P, so it closes with the fate "branch no longer
  primary". Then it takes a **planned restart** with the marker `{reason:
  primary_switch, branch: T}`.
- **At boot it re-realizes on T:** the newest snapshot on T's chain (one at
  or before the fork point always exists while T is live; §3.4), then
  replay of T's chain to its head. Or it imports T's suffix from a bundle
  if the snapshot is local.
- **The marker resumes only T's own obligations,** and only those T's
  writer left open by a planned stop. One resume per marker, as today.
- **A crash anywhere after the commit** is a markerless boot. It closes
  work, and the realization re-reads its branch from the ref.
- **A crash before the commit** changes nothing.
- **The route follows the ref,** so routing reads the primary, never a
  realization's own claim.

The old primary P stays a frozen, non-primary branch. It can be switched
back to, published from, forked from, or retired by an owner act. Switching
back is the same operation.

## 5. Opening branches, detecting splits, sync

- **Opening by choice** (cases 1 and 7): the home host, or the Mac's own
  host, opens a branch at the last head it holds and grants itself that
  branch's lease. This is an explicit act ("work here anyway"). The primary
  ref moves only when the act is made at the home host.
- **The revoke** (case 2) is defined as "open a host branch at the last
  synced main head, make it primary, freeze main at the home host". The
  Mac keeps writing main locally. Nothing is ever relabeled.
- **Sync** works by have/want per branch.
  1. Each side sends `{branch_id: (head_sequence, head_digest)}`.
  2. For each branch the receiver checks its own receipt at that sequence.
     - **A match** gets a delta bundle (`choir-bundle export --since`,
       paged by segment, imported in chunks with the head row advanced
       last, one tape B4).
     - **An unknown branch** gets its suffix from its base.
     - **The same branch with another digest** is equivocation (case 6):
       the receiving home host opens a new branch for the later-epoch
       suffix (UUIDv7), keeps both receipt sets, and shows the owner the
       proof.
  3. Bundle manifests gain `branch_id` and `base`. `planImport`
     (`internal/bundle/import.go:100-131`) returns a structured divergence
     instead of a string refusal.
- **Bounds:** memory is bounded by the page. The bytes are those of the
  unique tails.

## 6. Desks and irreversible effects on a non-primary branch

- Desks run on a branch's writer only. Each branch's Texture is the sole
  writer of that branch's canonical revisions.
- After a switch, the losing branch's work is retained but not live:
  - its Texture drafts and agent runs are visible from the branch view;
  - anything wanted on the primary comes over by publication and adoption.
- **Effects outside the computer cannot be undone:** mail sent, papers
  published to the Autopaper graph, egress, payments, OAuth refresh-token
  rotations.

The surfacing:

- At split detection and on the switch confirmation, the home host
  computes the **effects digest** of each unique tail, from the tape: the
  effect-bearing kinds (egress, mail send, publication, key events).
- The switch screen states it plainly: "the branch you are leaving sent 3
  emails and published 1 edition; they stay sent and stay recorded on that
  branch."
- The switch writes one ordinary event on the new primary, a **branch
  notice** listing those effects by reference to the other branch's tape.
  That is a record, not a merge: the new primary's Mail app knows a reply
  went out without adopting the thread state.
- External state that moved on the losing side (a rotated refresh token, a
  publisher version counter on the Autopaper graph) is the unsafe residue
  (§12).

## 7. Retention and storage

- The shared prefix is stored once.
- A non-primary branch's unique tail is kept until an owner act retires
  it. Retiring deletes that branch's rows above its base, its snapshot
  roots, its lease, and the pins no live chain cites (pin references are
  counted over the receipts that remain).
- A branch that is the base of a live branch keeps its rows up to the
  highest child base.
- The primary cannot be retired.
- The tape sweep (`internal/platform/tape_sweep.go:141-233`) reads
  `computer_branches` and the fork lineage (§8) instead of the
  `AcceptUnrecordedForkLineage` gate.
- A tail marked `keep_set_bearing` (owner mail, Texture, precommitment
  records, newspaper content) is listed, never auto-retired.
- Growth equals the unique tails, measured in hours to days for the
  realistic cases.

## 8. Forks and capsules (cases C and D)

- **A fork** is still a new computer: its own ComputerID, chain and
  signing identity.
  - Lineage: `computer_lineage(child, parent, parent_branch,
    parent_sequence, parent_head_digest, parent_snapshot_root)`.
  - Its genesis is an ordinary `genesis_imported`, citing the parent's
    head receipt and snapshot root in its payload.
  - "Fork from a branch" is the act that turns retained branch work into a
    separate computer.
  - Snapshot chunks: `filecas.SealChunk(dek, computerID, data)` binds
    digests to the key and the ComputerID, so a re-keyed fork shares no
    ciphertext. Chunk entries naming a `key_ref` are a later fix.
  - Branches of one computer share the DEK and the ComputerID, so **their
    snapshot chunks dedup today.** That is a storage reason to prefer a
    branch over a fork for the split cases.
- **A capsule** stays a capsule. It owns no event ordering. Its effect
  bundle returns by an accepted event. It is not a tape branch.

**Rejected: automatic merge of concurrent branches.** It would need parent
lists, hybrid logical clocks, version vectors and per-kind CRDT rules, and
it falls back to per-kind leases for everything that matters (Texture, the
reducer's single pending transition, queues). It contradicts I-c.

## 9. The four S's

Each mechanism is scored in the owner's order (+ better, 0 neutral, −
worse than today's single chain):

| Mechanism | Security | Stability | Simplicity | Scalability |
|---|---|---|---|---|
| Writer lease per branch, no expiry | + one signer per epoch per branch; equivocation provable | + cases 3-5 prevented | 0 one row, one check | 0 |
| Branches of one computer + primary ref | 0 same identity; I-d keeps revocations computer-wide | + nothing from a split is lost | − branch column, ancestry-aware lookups, snapshot chain check | 0 two tails of hours |
| Switch as one transaction + planned restart | + owner act only | + one commit point; crash-safe by reusing the restart rules | 0 reuses snapshot, replay, marker | 0 |
| Losing side becomes a fork (revision 1) | + fresh identity | 0 | − a re-key, a full snapshot (no dedup), and publication to get the work back | − |
| Automatic merge (rejected) | − | − silent merge bugs | − − | + |

## 10. Doctrine delta (needs owner ratification)

- **New:** a computer may have several branches and exactly one primary.
  The ontology's "a user computer is identified by stable ComputerID and
  its canonical event chain" becomes "...and its primary branch". The
  other branches are retained history of the same computer, not forks.
- **New:** switching primary is an owner act. It is distinct from
  publication and adoption (which crosses computers or branches by
  artifact), distinct from product restore (a forward event on one
  branch), and distinct from platform deploy.
- **New:** one writer per branch, fenced by a per-branch lease epoch. A
  realization serves one branch, and a follower realization is read-only.
- **Kept:** no merge of contents. Forks are separate computers with fresh
  identity whose results return only by publication and adoption.
  Capsules are unchanged.
- **New rule I-d:** security-monotone facts (revocations, tombstones,
  epochs) are computer-wide, so a switch never reverses them.

## 11. Do now (minimal), ranked by later pain avoided per unit of cost

| # | Change | Files | Cost | Pain avoided |
|---|---|---|---|---|
| 1 | Build S7 (the writer epoch) as `computer_writer_leases` keyed `(computer_id, branch_id='main')` with the checkout state and no expiry, checked in `compareAndSwapTx`. Put `branch_id` and `writer_epoch` in `eventHeadReceiptFields`, and check that `event_receipt.go`'s verifier tolerates the extra fields | `internal/platform/computer_events.go:227-262,352`, `lifecycle_control.go`, `credential_envelope.go` | medium (planned) | cases 3-6, and every receipt signed from now on names its branch |
| 2 | Snapshot roots record `head_digest` (and `branch_id='main'`); `RecoveryBase` and retention choose roots whose `(sequence, digest)` is on the chain being recovered | `internal/platform/snapshot_roots.go:93,105`, `internal/platform/file_cas.go:80`, the registration in `internal/snapshot/snapshot.go:334` | small | the one silent wrong-state restore that branches (or any bad import) would cause |
| 3 | `branch_id DEFAULT 'main'` column on `computer_event_heads` and `computer_event_append_receipts` (no key change yet), and `branch_id` in the keys of the new per-batch receipt table when it is built; `computer_branches` and `computer_primary_ref` tables created with the single `main` row | `internal/platform/computer_events.go:18-56` | small | a later 16 GB table rewrite, and receipts that cannot say their branch |

Later, when the first branch opens:

- the key changes, built `CONCURRENTLY`;
- ancestry-aware idempotency;
- per-branch replay serving;
- the switch transaction;
- have/want sync;
- the effects digest;
- `computer_lineage` and the sweep, with forks.

Not ever, under this design: a parent list or replica id in `Event`.

## 12. Failure modes and what makes this unsafe

- **Wrong-branch snapshot restore** (§3.4): silent, red. Do-now 2 closes
  it before any branch exists.
- **Wrong-branch idempotency hit** (§3.2): silent. It must be ancestry-aware
  before a second branch accepts appends.
- **Un-revoking by switch:** a key revoked on P is live again on T unless
  revocations are computer-wide (I-d). The `credential_revocation_epoch` in
  `computer_event_heads` is per head today, so it must become
  `max(all branches)` or move to a computer row.
- **Irreversible external drift:**
  - a refresh token rotated on P leaves T's stored token dead, so T's mail
    breaks until the owner re-authorizes;
  - two editions published from two branches under one publisher identity
    can collide in the Autopaper graph's version order. The graph must
    order a publisher's versions by its own sequence, never by branch-local
    state.
  - This residue is why always-busy and publishing computers never branch.
- **Lease authority loss:** the home control database is restored from an
  old backup, so epochs roll back (case 6). Epochs must be anchored in the
  receipts. On restore, the home takes `max(epoch)` seen in any receipt.
- **A Mac branch nobody imports:** the Mac's disk dies with an unsynced
  tail, and the work is lost. Mitigation: continuous delta sync whenever
  online.
- **Switch while P's writer is unreachable:** the fence holds at the home
  host, but P's offline realization keeps writing P locally. That is
  harmless: P is frozen at the home, and the extra tail imports as more of
  P, or as an equivocation if P's lease epoch moved.
- **Branch sprawl:** the soft cap of 4 live branches, and an owner surface
  that lists tails, sizes and effects.

## 13. Open questions

1. Should the default primary after a case-1 reconnect be the lease holder
   (the Mac), not the host branch the phone used? It is a policy on the
   same mechanism. The default here is route continuity plus an immediate
   offer.
2. Does a case-1 phone session even need writes, or is read-only plus
   "work here anyway" rare enough to make case 1 itself rare?
3. Can the home authority move (a Mac-only computer, an enterprise host)?
4. Should the branch notice (§6) let the owner pull specific Texture
   documents over in one click, still as publication and adoption inside
   one computer?
5. Is branching also the cheaper form of "undo to a checkpoint" (open a
   branch at the checkpoint, make it primary, keep the old tail), compared
   with today's forward restore event?
