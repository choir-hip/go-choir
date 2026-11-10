---
definition_version: 4
readiness: drafted
review: {reviewer: none, frozen_ref: none, verdict: none, evidence_ref: none}
---

# Branching tape: the first three do-now items as a mission proposal (2026-11-10)

Status: proposal, readiness `drafted`. Not on `docs/ACTIVE.md` or
`docs/mission-graph.yaml`; it needs owner ratification of the doctrine delta
in [branching-tape-2026-10-31.md](branching-tape-2026-10-31.md) §10 before
anything executes. Mutation class of this document: green.

This continues the 2026-10-31 design. It does not restate it. The design's
§11 names three "do now" items, so they are taken from it, not derived. What
this document adds: each item checked against the code as of `origin/main`
2026-11-10, the physics of a branch, two findings that change the items, and
the goal-file shape.

## 1. The three items (from design §11)

1. **Writer lease with an epoch**, `computer_writer_leases` keyed
   `(computer_id, branch_id='main')`, checked in `compareAndSwapTx`;
   `branch_id` and `writer_epoch` added to `eventHeadReceiptFields`.
2. **Snapshot roots record `head_digest` (and `branch_id`)**; `RecoveryBase`
   and retention choose only roots whose `(sequence, digest)` is on the chain
   being recovered.
3. **`branch_id DEFAULT 'main'` columns** on `computer_event_heads` and
   `computer_event_append_receipts`, plus `computer_branches` and
   `computer_primary_ref` with the single `main` row.

## 2. Each item against the present code

Source key: [C] read in code this session; [D] stated by the design and not
re-measured; [A] assumed.

### Item 1: writer lease and receipt fields

What exists [C]:

- The append path is `ComputerEventCAS.compareAndSwapTx`
  (`internal/platform/computer_events.go:245`). It reads the head `FOR UPDATE`
  (`:354`), reduces, compares the guest projection, signs, then inserts or
  updates the head with `WHERE computer_id=? AND sequence=? AND
  canonical_event_head=?` (`:290`). There is no lease read.
- A fence already exists in a different form: the append capability carries
  `RevocationEpoch`, and `event_capability.go:134` refuses when
  `head.CredentialRevocationEpoch != capability.RevocationEpoch`. That epoch
  is a credential revocation counter stored in the head row, bumped by
  credential-envelope consumption (`credential_envelope.go:235`, `+1`). It is
  not a writer or realization epoch. The realization epoch exists only as
  values in lifecycle receipts (`lifecycle_control.go:23-24`,
  `computer_lifecycle_operations.prior_realization_epoch`), written by the
  lifecycle path, never read by the append.
- `internal/tapefollow/store.go:43` already hard-codes `Branch = "main"` with
  the comment "the writer lease's branch_id". So one consumer has named the
  column before it exists.
- No `computer_writer_leases` table, no `branch_id`, no `writer_epoch` anywhere
  in `internal/platform` [C: grep]. `internal/store/lease.go` and
  `internal/lease` are the custodian grant leases (kind `choir.lease`), a
  different thing; the name collision should be avoided (call the table
  `computer_writer_epochs`).

What would break [C]:

- **The design's claim that the verifier "tolerates the extra fields" is
  false.** `EventHeadReceiptVerifier` (`internal/computerevent/event_receipt.go`)
  calls `receipt.RequireKindFields(...)` with exactly twelve names, and
  `RequireKindFields` (`receipt.go:161`) returns "kind field count mismatch"
  when `len(KindFields) != len(names)`. Adding `branch_id` and `writer_epoch`
  to the signed fields makes every new receipt fail verification in any guest
  running the old verifier. The verifier is in the guest binary.
- Therefore item 1 is two releases, not one: (a) ship a verifier that accepts
  fourteen fields, or twelve *or* fourteen, and wait for every computer to
  carry it (the ring deploy, then the paper last); (b) only then let the host
  sign fourteen. Under `Ring-Expedite` for everything (CLAUDE.md, 2026-11-09)
  the rings collapse, but a computer that has not refreshed still holds the
  old verifier, so (b) must be gated by a correctness refusal (host refuses to
  sign fourteen until the computer reports the verifier version), not by a
  switch. That is the same shape as the existing release-skew refusal. The
  alternative that avoids the skew entirely: put `branch_id`/`writer_epoch` in
  the receipt envelope's unsigned-by-kind common fields, which is a bigger
  change to a protected format. Not recommended.
- `existingComputerEventReceipt` (`:332`) replays a stored receipt; stored
  receipts have twelve fields and must stay verifiable forever. The verifier
  must accept both shapes permanently, with `branch_id` absent meaning
  `main` and `writer_epoch` absent meaning 0.
- Cost on the hot path: one extra indexed primary-key read inside the existing
  serializable transaction (the head row is already read `FOR UPDATE`; the
  epoch can live on the head row itself, see §4 alternative).

What is missing: the table, the check, the fence-bump operation (today the
bump is a side effect of credential consumption), and the rule for who
increments it. The data-system frame (`choir-data-system-frame-2026-10-30.md`
line 35) says the writer epoch is "the realization epoch, held in one lease
row per computer" and travels in the append capability. That is the intended
substrate; the code has not built it.

### Item 2: snapshot roots carry the head digest

What exists [C]:

- `computer_snapshot_roots(computer_id, root, head_sequence, chunks,
  created_at)`, `PRIMARY KEY(computer_id, root)`
  (`snapshot_roots.go:18-27`). No digest, no branch.
- `RecordSnapshotRoot` prunes by `ORDER BY head_sequence DESC, created_at
  DESC, root DESC LIMIT retention` (`:93`); retention is 4, or 2 above 2048
  chunks (`:33-37`). `LiveSnapshotRoots` (`:105`) and `RecoveryBase`
  (`file_cas.go:80`) order by `head_sequence` alone.
- The guest already has the digest at the registration site:
  `snapshot.go:320` sets `record.StoreHead = got.head.CanonicalEventHead`, and
  `:334` calls `registrar.RegisterSnapshotRoot(ctx, manifest.Root,
  record.StoreHeadSequence, chunks)`, passing the sequence and dropping the
  head. The HTTP body (`HandleSnapshotRoot`, `snapshot_roots.go:128`) takes
  `ComputerID, Root, HeadSequence, Chunks`.

What would break [C]:

- Adding the field is additive to the PUT body (the decoder is
  `decodeFileCASJSON`; whether it uses `DisallowUnknownFields` is not
  determined, and if it does, an old host rejects a new guest body: ship the
  host first). Old rows have no digest. The rule for a null-digest root must be
  decided: treat it as usable only for `main` and only while exactly one
  branch exists, which is true everywhere today.
- Validation needs the chain: "is `(seq, digest)` on this branch's chain" is a
  lookup of the append receipt at `seq` (`computer_event_append_receipts`
  carries `event_digest` and the per-sequence unique key), comparing
  `canonical_event_head` for that sequence. Careful: the root's
  `StoreHead` is the head *after* applying that event; the receipt stores
  `event_digest`, not the post-event head. The post-event head is the hash
  derived by `Reduce`; to compare it the host would need the head after each
  event. `computer_event_append_receipts` does not hold it [C: DDL read at
  `computer_events.go:33-48`: `event_digest` only]; the signed receipt holds
  `desired_event_head`/`effective_event_head`, and canonical head is in the
  head row only for the tip. So the check should compare on the receipt's
  `event_digest` at `head_sequence` (the guest records the digest of the last
  applied event, `StoreHeadDigest`, or the host derives it): this is a design
  detail the item must settle, and it shapes the schema (store the *event
  digest at head_sequence*, not `canonical_event_head`). **Finding:** the
  design says `head_digest` without saying which digest; the two differ, and
  only the event digest is checkable against the receipt table.
- Behaviour change on today's single chain: none for a correct chain. The
  point of the item is that a root off the chain is refused at record time and
  skipped at recovery. That is a new refusal on `RecoveryBase`; a computer
  whose newest root is off-chain today would silently recover from it, so
  landing the check could surface an existing latent defect as a refused boot.
  It must be proved on d@e before the ring, and the refusal's fallback is the
  next-newest on-chain root, never "no base".

### Item 3: columns and the two tables

What exists [C]: both tables are `CREATE TABLE IF NOT EXISTS` DDL blocks in
`computer_events.go:18-56`; the repo already adds columns with
`ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (`standing_consensus.go:94`), so
the pattern is established. `computer_event_heads` is `PRIMARY KEY
(computer_id)`; append receipts `PRIMARY KEY (computer_id, idempotency_key)`
plus `UNIQUE (computer_id, sequence)` and `UNIQUE (computer_id,
event_digest)`.

What would break: nothing, if the column is only a defaulted attribute with no
key change. But a column nobody reads is the "code that I asked for and
thought was running" pattern the owner objects to (CLAUDE.md, 2026-11-03).
Item 3 alone is inert: it earns its place only because items 1 and 2 read and
write `branch_id` (the lease key and the snapshot root's chain check). It
should not land as a separate commit ahead of them; it lands with item 1.
`receipt_compaction.go` and `bundle/import.go` (`planImport`) read the receipt
table by `(computer_id, sequence)`; they keep working while there is one
branch, and the "a fork" refusal string at `import.go:176` is where the first
branch-aware divergence report would go (later, not now).

The design's "16 GB table rewrite" claim [D] is unmeasured here; I could not
reach production. In PostgreSQL 11+ a constant-default `ADD COLUMN` is
metadata-only [A: engine behavior, not measured on this schema], so the cost
this item buys is avoiding a later *key* change, not a column add.

## 3. The physics of a branch

Computed from code unless marked.

**Bytes at a branch (same computer).** A branch row set above `base_sequence`
is empty at open: one `computer_branches` row, one lease row, one ref row
(order of 10^2 bytes each, [A]). The tape prefix is shared by reference; the
design's table scheme stores only rows above the base. Snapshot chunks are
shared: `SealChunk` derives a synthetic nonce from HMAC(plaintext) under a key
derived from the DEK and ComputerID (`filecas.go:184-200`), so a branch with
the same DEK and ComputerID re-seals an unchanged 4 MiB chunk
(`snapshot.ChunkSize = 4<<20`) to the identical digest and the platform stores
it once. Copy cost at a branch: 0 bytes of chunk data. Growth afterwards: the
chunks the branch rewrites. The paper's store changes "most of its page
ranges between snapshots" (`snapshot_roots.go:34-37`, 31 GB, about 25 GB per
root), so a long-lived branch of a large, hot computer approaches a full
second copy after one snapshot; for a personal computer that is mostly idle
the unique tail is small [A]. The design's claim that branching is for
personal computers and not for always-busy ones is therefore a physics
statement, not a policy preference.

**Bytes for a fork (separate computer).** A fork has its own ComputerID and
DEK; digests bind to both, so it shares no ciphertext. Cost at fork: one full
snapshot, equal to the parent's store size (31 GB for the paper, about 7,900
chunks of 4 MiB, a chunk list of about 0.5 MB of hex digests, computed from
31 GB / 4 MiB and 64 bytes per digest). That is the storage reason the design
prefers branches to forks for split cases.

**Time to branch.** Opening the branch record: one control-database
transaction, milliseconds [A]. Making a *realization* serve it: restore the
newest on-chain snapshot at or below the fork point plus replay to the branch
head. For a branch opened at the current head with a snapshot at it, replay is
zero events and the cost is snapshot fetch/restore of the store size; the
boot envelope that already binds the paper (15-minute credential envelope,
CLAUDE.md "Go to the physics") is the bound. A switch of primary is that
re-realization plus the planned restart; the commit itself is one
serializable transaction. I did not measure restore time; the repo has the
restore drills but I did not run them.

**Identity re-minted.** At a branch: a branch id (UUIDv7, host-minted), a
lease epoch, a realization id, a fresh append capability. Not re-minted:
ComputerID, DEK, signing keys, credential envelope seed. At a fork: ComputerID,
DEK (so all chunk digests), signing identity, event chain genesis
(`genesis_imported` citing parent head and snapshot root); the design's
`computer_lineage` row. The security-monotone facts (revocation epoch,
tombstones) must be computer-wide at a branch: today
`credential_revocation_epoch` is a column of `computer_event_heads`, which
item 3 makes per branch. **Finding:** after item 3 the revocation epoch is
physically per `(computer, branch)` while invariant I-d says it is
computer-wide. Item 3 must either move it to a computer-level row now (a
change to the `event_capability.go:134` comparison) or add a read of
`max(over branches)`. Leaving it where it is and adding branch columns around
it recreates the un-revoke hazard the design lists in §12, deferred but
structurally scheduled.

## 4. What the items cost and what they buy

| Item | Change size | Release shape | Silent risk it closes | New refusal it adds |
|---|---|---|---|---|
| 1 writer epoch + receipt fields | medium: new table, one read in the append tx, verifier + signer in two releases | verifier first, signer after, by refusal on skew | two writers on one chain; receipts that cannot name a branch | append refused on stale epoch |
| 2 root digest | small: one field in the PUT body, one column, one chain lookup | host first, then guest | restore of off-chain state | recovery skips off-chain roots; record refuses them |
| 3 columns and tables | small, but inert alone | lands inside item 1 | later key rewrite | none |

A smaller shape for item 1, to consider before building: the lease may be
two columns on `computer_event_heads` (`writer_epoch`, `branch_id`) instead
of a table, because the head row is already locked `FOR UPDATE` in the append
transaction (no extra read) and `PRIMARY KEY (computer_id)` makes
"exactly one head per computer" explicit. The cost is that a second branch
needs a new key, which the design already defers to "when the first branch
opens". A separate table is the design's choice and keeps the head table
unchanged; the one-row-per-computer fact is identical either way. Decision for
the builder: the table, so that the later move to `(computer_id, branch_id)`
is a key change on a small table and not on the head table.

## 5. Mission proposal (goal-file shape)

```yaml
definition_version: 4
readiness: drafted
goal: >
  Every receipt the host signs names its branch and the writer epoch that
  signed it, an append from a stale writer is refused, and a snapshot root
  can only restore the chain it came from, so the first branch can be opened
  later without a silent wrong-state restore, a double writer, or a rewrite
  of the receipt tables.
```

### start (initial-state receipt, to be completed at execution)

Observed 2026-11-10 from `origin/main`, read-only:

- No branch concept in `internal/platform` (no `branch_id`, no `writer_epoch`,
  no writer-epoch table). `tapefollow.Branch = "main"` is a constant.
- Append fence: `RevocationEpoch` in the capability against the head row
  (`event_capability.go:134`); not a writer lease.
- Receipt verifier requires exactly 12 kind fields (`event_receipt.go`,
  `receipt.go:161`).
- Snapshot roots keyed `(computer_id, root)` with `head_sequence` only.
- Unknown at proposal time: the deployed identity, whether any computer's
  newest snapshot root is off its chain, the current guest verifier version
  distribution across the fleet, `decodeFileCASJSON` strictness. Reconciling
  those is the first action, before any code. Mutation class stays `unknown`
  for that read-only step.
- Dirty work: none in scope.

### finish

- Release A: the verifier accepts both the 12-field and 14-field receipt
  shapes; no signer change. The host reports a verifier version per computer.
- Release B: `computer_writer_epochs(computer_id, branch_id, epoch, state)`
  with the single `main` row per computer; `compareAndSwapTx` refuses an
  append whose capability epoch is not the current epoch; `eventHeadReceiptFields`
  carries `branch_id` and `writer_epoch`; the host signs 14 fields only for a
  computer whose reported verifier accepts them (a correctness refusal on
  skew, not a switch). `branch_id VARCHAR DEFAULT 'main'` on the head and
  receipt tables, `computer_branches` and `computer_primary_ref` with the
  `main` row. Revocation epoch read as computer-wide (§3 finding).
- Release C: `computer_snapshot_roots` gains `head_event_digest` and
  `branch_id`; guest sends them; `RecordSnapshotRoot` refuses an off-chain
  root; `RecoveryBase` and retention choose newest on-chain roots per branch.
- Landing: pushed SHA per release, CI, staging deploy identity, the
  acceptance below, per the Landing Loop. Each commit carries `Ring-Expedite`
  while the 2026-11-09 expedite stands; the trailer is added at commit time,
  not in this docs commit.

### acceptance (artifact-verified, repeatable)

- A1. On d@e, after release B: a stored pre-change receipt (12 fields)
  verifies, a new receipt (14 fields) verifies, and the new receipt names
  `branch_id: main` and the current epoch. Artifact: the two receipt JSONs and
  the verifier's output. An old-guest/new-signer pair is refused by the skew
  rule rather than failing verification.
- A2. A stale-epoch append is refused on d@e (drive: bump the epoch with the
  lifecycle path, replay an append under the old capability). Artifact: the
  refusal and the unchanged head.
- A3. Item 2: record a root with a digest not on the chain; it is refused. Boot
  d@e with its newest root made off-chain (adversarial drill on d@e only);
  recovery uses the next on-chain root. Artifact: boot log naming the chosen
  root and head.
- A4. Differential: for every computer, `RecoveryBase` before and after the
  change returns the same root (all roots on chain today). Any computer where
  it differs is a pre-existing defect, recorded in the ledger, not hidden.
- A5. Hot-path cost: append latency before and after on d@e, one extra
  primary-key read; reported, not gated.

### rollback

- Release A: revert; receipts are unchanged.
- Release B: revert the signer commit first. Receipts already signed with 14
  fields remain valid only for verifiers from release A, so release A is not
  reverted while any 14-field receipt exists; if A must go, new 14-field
  receipts are superseded by a recovery snapshot on an older 12-field head.
  This asymmetry is the main rollback cost and is why A ships and soaks alone.
- Release C: revert; the added columns are nullable and ignored.
- Tables and columns are left in place on revert (additive).

### value

```yaml
value:
  better_means: >
    Reduce the set of silent-wrong-state paths that a second branch would
    open (restore from an off-chain root, a write from a fenced writer, a
    receipt that cannot say which chain it signs) to zero before any branch
    exists, while keeping every existing receipt verifiable, every current
    recovery base unchanged, and append latency within noise.
  goodharting_would_be: >
    Adding the branch_id and writer_epoch columns and fields so a grep finds
    them, with no append ever refused for a stale epoch and no root ever
    refused for being off-chain; or a check that compares head_sequence only
    and calls it a chain check.
```

### homotopy

```yaml
homotopy:
  realism_axis: >
    one chain, one branch 'main', the real append path and the real recovery
    (the same tables, receipts, capabilities and boot), then two branches
    with a real switch, then a Mac branch syncing. This mission stays at the
    first point: single branch, full-resolution mechanisms. The axis is
    deformable because nothing here is a fixture: the tables, the verifier and
    the lease are the ones the multi-branch system uses.
```

### conjecture (the bridge)

```text
If the single-branch tape carries branch id and writer epoch in every signed
receipt, refuses stale epochs, and restores only along its own chain
(A under S), then opening the first real branch later needs no receipt
rewrite and cannot restore the wrong state (G advances).

CLAIM   adding the epoch and the chain check now removes the dominant
        silent-failure paths of a later branch.
TEST    A1-A4 on d@e; later, the first branch opened by hand on d@e is
        restored, switched and retired without a manual repair.
EDGE    missing_oracle: nothing today shows whether any computer's newest
        root is already off-chain, so A4 may pass vacuously on a fleet where
        the question was never asked. Also frame_lock: the design says
        "head_digest" and means a chain check, but receipts store
        event_digest and not the post-event head; a check written against
        the wrong digest passes and checks nothing.
DELTA_O a read-only audit of every computer's roots against its receipts
        (event digest at head_sequence), run before release C; and a
        negative test whose root is built to differ only in digest.
SCOPE   single-branch correctness of receipts and restores. Not
        multi-branch sync, switch, or effects (design §5-§6).
```

Status: `proposed`. A second conjecture, `proposed`: the lease can live in a
table without hot-path cost; test A5.

### boundaries

- Authority: needs owner ratification of design §10 (a computer may have
  branches, one primary) before release B, because B writes `branch_id` into
  signed receipts, which is a commitment to the vocabulary. Release A alone
  is ratification-free (it only widens the verifier).
- Mutation class per item:
  - Release A (verifier accepts 12 or 14 fields): red, because receipts and
    the guest verifier are protected surfaces; additive, one-act revert, so
    tier 1 by CLAUDE.md's description. During the data-system cutover this
    review tiering was suspended; if that period has ended, one Devin seat.
  - Item 1, release B (epoch check in the append, signed fields): red,
    silent-or-irreversible class (auth/session-adjacent, new concurrency on
    the append path, signed format), tier 2.
  - Item 2, release C (snapshot root chain check): red (boot/restore path),
    tier 2: a wrong refusal prevents a boot. The fallback to the next on-chain
    root is the safety.
  - Item 3 (columns, tables): orange alone; red when read by items 1-2, so it
    is part of release B's class.
  - Proof tooling and this document: green/yellow.
- Invariants: existing receipts stay verifiable forever; the keep set; the
  restart rules (the epoch bump is a fence, not a resume path; no boot-time
  resumers); no switches (the skew refusal is a correctness refusal); no
  ordinary-computer behaviour change except a refusal on a stale writer.
- Exclusions: no second branch is opened, no switch, no sync, no fork, no
  `computer_lineage`, no tape-sweep change, no effects digest, no key changes
  to the receipt table, no doctrine edit (that is the owner's act). Always-
  busy computers (the paper) are touched only by the deploy, last.
- Adversarial drills run on d@e only.

### now (initial card)

```text
status: drafted
slice: A0 read-only audit (roots vs receipts; verifier version spread)
conjecture: the bridge above, proposed
decision pending: table vs head-row columns for the epoch (§4); the event
  digest vs canonical head for the root check (§2 item 2)
belief: items 1+3 are one change; item 2 is independent and can go first
blocker: owner ratification of design §10 before release B
next_action: write and run the A0 audit against a copy of d@e state
```

Because item 2 is independent, small, and closes the one failure the design
calls "silent, red" with no doctrine dependence, the recommended order is
**C (item 2), then A, then B**, not 1-2-3. The design ranks by pain avoided
per cost; the code adds an ordering constraint (A must precede B and soak).

## 6. If this does not converge

The proposal converges on a concrete mission, but one fact would change it:
whether any computer's recovery base is already off its chain (audit A0). If
some are, item 2 becomes a defect fix with its own urgency and ledger entry,
independent of branching. If none are, items 1-3 are preparation for a
feature with no scheduled consumer (the design's own likelihood table rates
case 1, a Mac working offline, "high" but the Mac checkout does not exist);
in that case the observation that decides is whether the Mac-checkout mission
is scheduled: without it, B should wait and only C ships.

## 7. What I did not determine

- Production sizes: the receipt table's size ("16 GB" in the design), per-
  computer store sizes other than the paper's 31 GB, snapshot restore time.
- Whether `decodeFileCASJSON` rejects unknown fields.
- Which computers run which guest verifier build.
- Whether a canonical-head-after-event is derivable on the host from stored
  data other than by replay (the receipt's signed fields hold the desired and
  effective heads, which may be the thing to compare; unverified).
- Whether `credential_revocation_epoch` is the right thing to promote to the
  writer epoch (it already fences capabilities); using it would shrink item 1
  but couples revocation to realization handoff.
