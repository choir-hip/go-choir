# The Choir data system, one frame (2026-10-30)

Status: frame for owner argument, then panel review (Devin, DeepSeek and a
Sonnet subagent; Codex once all three accept). Class: green as a document;
it designs red surfaces (tape, keys, deletion, restore, boot path), so it is
reviewed like a red change. Owner direction for this document: "zoom out and
really design the system from the big picture so that it makes sense"
(2026-10-30).

The two detailed designs, object storage
([object-storage-2026-10-30.md](object-storage-2026-10-30.md), 13 revisions,
2,100 lines) and history, restore and audit
([computer-history-restore-audit-2026-10-30.md](computer-history-restore-audit-2026-10-30.md),
970 lines), kept growing because nothing above them said what the system is.
This is that frame. They get cut down to fit it, not the other way round.

## 1. The scale we design for

Owner, 2026-10-30: "user computers will look closer to the paper in a few
months. ~4 agents running 24/7 won't be so rare."

| | Today [V] | Design target |
|---|---|---|
| Paper events per day | 308 K mean, 742 K peak day (10-22) | the same, per heavy user computer |
| Other computers | 248 together, 10.5 K per day | a mix: most light, some paper-like |
| Bytes per event | payload 4,657 B, plus about 5.7 KB of overhead (envelope 1,512 B, two pin receipts 1,790 B, host receipt 2,362 B) | payload plus at most 200 B |
| Host round trips | one synchronous append per event | one per batch |
| Control database growth from the tape | 1.07 GB per day of per-event receipt rows (19 GB of the 19.1 GB projected PostgreSQL control size) | one index row per 64 MiB segment |
| Paper guest store | 20.8 to 27.9 GB projected in SQLite, of which 22.2 GB is the guest's copy of the event log | about 5.7 GB: state, not history |

Sources: [tape-window-sizing](../evidence/tape-window-sizing-2026-10-30.md),
[new-engine-storage-measurement](../evidence/new-engine-storage-measurement-2026-10-30.md)
(state C at 4 KiB: objects 5.3 GB, key index 0.35 GB, events 22.2 GB).

What a heavy computer costs at the target: about 0.4 GB per day of sealed,
compressed segments (5.4x zstd measured on today's segments), before the event
classes in §5 cut it further. A 30-day window is about 12 GB per heavy
computer. Node B has 476 GB, so the blob backend must be able to leave Node B
before about 25 heavy computers keep 30-day windows. That is why the tape and
snapshots go through the blob interface (§2) and nothing assumes a local disk.

The composition measurement (which tables and kinds make the paper's 300 K
events, how much is the same keys rewritten, what 250 ms and 1 s coalescing
would do) is running and lands as
`docs/evidence/paper-tape-composition-2026-10-30.md`. §5 depends on it.

## 2. Five things

Everything Choir stores is one of these. Each has one home.

1. **A tape per computer.** The computer's history: append-only, hash-chained,
   one writer (the computer), kept on the host, receipted by the host. Each
   event is an envelope the host can read (kind, sequence, hashes, sizes, time,
   class) and a payload only the computer's keys open. Physically: sealed
   64 MiB segments stored as blobs, plus one index row per segment in the
   control database. One copy, host-canonical (owner ruling 2026-10-30, as
   settled in
   [the 2026-08-16 tape design](../archive/choir-unified-event-tape-design-2026-08-16.md):
   "tape is platform-canonical ... Platform must store payloads; guest must be
   able to fetch them on replay").
2. **Blobs.** One content-addressed store for every body: file chunks, tape
   segments, snapshots, captures, published content, attachments, releases.
   Tiers: public, platform, private (sealed per owner), internal. Local packs
   on Node B first; an S3-compatible backend later behind the same interface.
3. **The control database** (PostgreSQL, instance 1). Accounts, sessions,
   computers, leases and writer epochs, routes, releases, quotas, wrapped keys,
   the blob catalog, the tape segment index, input queues.
4. **The Autopaper object graph** (PostgreSQL, instance 2). Published objects
   and their edges, capture metadata, adoption. Bodies are public-tier blobs.
   A second instance keeps the paper's load and a control failure apart.
5. **The platform audit log.** Append-only record of what the host and its
   operators did: key unwraps, restores, deletions, admin reads, deploys,
   exports. A Merkle log, so one tenant's entries can be proven without
   exposing anyone else's. Its root, together with every tape head, is
   anchored outside Choir (RFC 3161 timestamp and a public git commit) on a
   fixed cadence. Stored as its own append-only table; never updated.

Inside each computer: a SQLite database (current state) and a file tree.
Both are the computer's working copy, rebuilt from snapshot plus tape. They
are not a sixth home.

## 3. Six rules

1. **The tape is the truth, and each tape has one writer: its computer.** The
   host keeps, verifies and receipts it; it never writes to a computer's tape.
   Facts that start on the host (an inbound email, a platform notice, an owner
   act sent while the computer is off) wait in the computer's input queue and
   enter the tape when the computer appends them. Each boot gets a writer
   epoch; an append from a stale epoch is refused. With the write path in §4
   (apply locally, then ship, resend at boot) the host's head can only be
   behind the computer's, never ahead, except when the computer's disk is
   lost, which is the broken-computer flow (§6). Today the computer appends to
   the host first and projects second, so a crash between them leaves the
   host ahead and the append path refuses with `ErrNeedsProjectionRepair`
   (`internal/computerevent/appender.go:224-243`). Whether any host service
   writes a computer's tape today is not yet checked; the review checks it.
2. **Nothing leaves a computer before its cause is receipted.** Appends are
   batched (§4). An effect that leaves the computer (an email sent, a
   publication, a precommitment that must predate its outcome, a
   self-development accept, anything involving money or credentials) waits for
   the receipt of the batch that records it. A crash loses at most the
   unshipped tail, and that tail never had an effect outside the computer.
3. **Restore is a forward event, and it never rewinds acts.** Restoring to
   time P appends a new event; history is not cut. Working state and files go
   back to P. Acts (§5) stay: a precommitment, a sent email or a publication
   made after P is still a fact after the restore.
4. **Deletion is an act by whoever holds the reference, never a guess by the
   host.** The computer knows what its state references, so a delete names
   the blobs it releases. Window expiry is an act too. The host never infers
   liveness by scanning for absent references. Derived and cache tiers expire
   by rule because they can be rebuilt.
5. **Sealed at rest; every unwrap is audited.** Tape payloads, snapshots and
   private blobs are sealed with the computer's key before they reach the
   host. Every time the host unwraps a key (boot, restore, export, support),
   that is an entry in the audit log. Public-tier content is plaintext because
   it is published.
6. **One home per kind, and the kind declares its retention class.** Doctrine
   `I27`: every kind has one writer and one home. Each kind's package schema
   also declares whether it is an act, working state or telemetry (§5), so the
   substrate applies retention without knowing the kind.

## 4. The write path

```text
agent step ─► one SQLite transaction: apply the change + add events to the outbox
                 │
   flusher (every 250 ms, or N events, or at once when an effect waits)
                 │  one batch: [events with prev-hash chain], writer epoch
                 ▼
host: check epoch and chain continuity ─► append to the open segment ─► one fsync
                 │
   one signed receipt for the batch (covers every event through the chain)
                 ▼
computer: mark the outbox rows receipted, release any effects waiting on them
```

- The computer applies first and ships second, so an agent never waits on the
  host except at a barrier (rule 2).
- At boot the computer resends its outbox. The host returns the existing
  receipt for anything it already has (same sequence and hash), so resending
  is idempotent.
- A crash boot (no planned-restart marker) still closes open work with "interrupted
  by a restart" (CLAUDE.md). The computer writes those closures; it is the
  only writer.
- Per-event cost becomes the payload plus a slim envelope. The per-event pin
  receipts and per-event host receipt rows go away: the segment is the pin and
  the batch receipt covers its events.

## 5. Event classes and retention

The proposal below waits on the composition measurement and an owner
decision (§10, question 1).

| Class | Examples | In the restore window | After the window (users, default) | After the window (the paper) |
|---|---|---|---|---|
| **Acts** | owner acts, precommitment records and outcomes, sent messages, publications, adoptions, self-development decisions, restores, deletions | full | kept for the life of the computer | kept; public ones also in the Autopaper object graph |
| **Working state** | desk and app row writes, drafts, run state, memory | full fidelity: any point can be restored | compacted to the state at the window's start (the oldest snapshot) | offloaded first to the analytical database (later), then compacted |
| **Telemetry** | progress, heartbeats, token counts, health samples | not on the tape: logs and metrics, short retention | | |

- Restore windows: 7, 14 or 30 days by plan (owner, 2026-10-30).
- "Users never offload their event records" (owner, 2026-10-30) holds for
  acts, which are the records. The proposal reads it as not covering every
  intermediate row write a working agent makes. At paper rates, keeping those
  forever costs about 146 GB per heavy computer per year, compressed.
- The paper's precommitment records are acts. Today they are about 10.8 K per
  day at 3.2 KB, or 35 MB per day kept forever [V].
- Until the analytical database exists, the paper's working-state segments
  leaving its window go off-host to cold storage (about 0.5 GB per day
  compressed today), not onto Node B.

**Snapshots.** A snapshot is the SQLite file plus the file-tree root, stored
through the computer's existing file content store (encrypted chunks under
the computer's key). Successive snapshots share unchanged chunks. Snapshots
are taken on a schedule set by replay time: often enough that restoring to
any point replays at most a few minutes of events (for a heavy computer,
about every six hours). They are also taken before risky acts: a
self-development apply, a restore, a migration. The chunk size for database
snapshots is set by the measured churn. A daily full copy of a 5.7 GB store
over a 30-day window would be 170 GB, so the snapshots must share chunks.

## 6. The flows

**Rewind (restore to P).** The owner asks, from the app or the web. The host
stops the computer, which fences its epoch, and boots a fresh computer from
the current release with a new epoch. That computer:

1. loads the newest snapshot at or before P and replays working-state events
   up to P;
2. replays every act after P on top;
3. sets the file root to its value at P;
4. seals a snapshot;
5. appends one restore event naming that snapshot.

Replay treats a restore event as a load point, so later replays never need
the tape before it. The host's part is an audit entry for the key unwrap.
Acts survive. Anything outside the window is refused with the reason.

**Broken computer.** This is the rewind with P set to the head. The image is
discarded, and a fresh computer is built from the current release, the
newest snapshot and the tape tail. Loss is bounded by rule 2: at most the
unshipped outbox, which caused no outside effects.

**Self-development and PSA.** Mutation runs in a capsule. Accepting is an act
behind the barrier. Undoing an accepted change is a rewind to the snapshot
taken before the apply, or a release rollback, and the accept and the undo
both remain on the tape. A precommitment record is an act whose batch
receipt, and that receipt's place under the next anchor, proves it existed
before its outcome. That proof is what the value proposition rests on.

**Publish and Autopaper.** A publication is an act on the publishing
computer's tape. The publishing computer writes its body to the public tier
and its object and edges to the Autopaper object graph. Adoption is an act
on the adopting computer's tape that names the published object by hash. The
paper is an ordinary computer with a high event rate and a shorter window.

**Compliance export.** For one tenant, the export contains:

- their tapes: acts in full and the window in full;
- their snapshots and blobs, sealed, with the means to open them;
- their audit log entries, each with a Merkle inclusion proof;
- the anchors.

No other tenant's data leaves. A legal hold pauses expiry for named
computers, and lifting the hold is an act.

**Delete an account.** Destroy the owner's key wraps (crypto-erase), then
release their blobs and segments with a deletion act. The audit log keeps
that the account was deleted, not what it held.

## 7. Keys

- Every computer has a key. Tape payloads, snapshots and the computer's
  private blobs are sealed under it.
- The computer key is wrapped by the owner key.
- Before release, the host holds the owner key wrap in escrow, and every
  unwrap is audited (owner, 2026-10-30: "host having some permissions during
  this migration and the pre-release period"). This is the custody the code
  has today
  (`HandleRealizationKeyDelivery`, `internal/platform/key_delivery.go`).
- Later, owner keys are passkeys, or are derived from passkeys, held in the
  owner's phone's trusted execution environment (`ProtectorPasskeyPRF`
  already reserves the slot). The host then cannot unwrap without the owner.
- The frame does not change with that move: only who can perform the unwrap
  changes.
- Today's gap: projection batch payloads reach the host in plaintext
  ([problem](../problems/host-holds-user-state-in-plaintext-2026-10-30.md)).
  Rule 5 closes it at the cutover; the cutover re-seals what exists.

## 8. Where every current folder goes

Sizes are from the [2026-10-29 inventory](../evidence/node-b-storage-inventory-2026-10-29.md)
before that day's deletions; they show which things are large, not today's
exact bytes.

| Today | Size then | Becomes |
|---|---|---|
| Store A, `platform-dolt/platform` (Dolt) | 56.6 GB | control database (PostgreSQL 1); per-event receipt rows become segment index rows |
| `platform-dolt/.choir-backup` | 54.3 GB | gone; PostgreSQL WAL archiving and pack backups go off-host directly |
| Store B, `corpus-dolt/corpus` (Dolt) | 15 GB | Autopaper object graph (PostgreSQL 2); 2.17 GB measured on the same rows |
| guest Dolt stores (the paper's data disk 57 GB) | | guest SQLite holding state only; history lives on the host tape |
| `tape-seg/` | 27.2 GB | sealed tape segments as blobs; the pre-cutover segments become a sealed, compressed archive chained into the new tape's first event |
| `sha256/pin-receipts`, `computer-event`, `computer-event-payload` (legacy pins) | 47 GB | folded into the archive at cutover, then gone |
| `sha256/projection-base`, `projection-base-parts`, `computer-checkpoints`, `checkpoint-scratch` | 61.6 GB | gone; computer-made snapshots replace them |
| `sha256/file-cas-chunks`, `file-cas-roots` | 28.9 GB | private-tier blobs (already sealed per computer); the seed copy each computer carries becomes one shared platform-tier copy |
| `sha256/og` and the top-level `sha256/*.txt` | 0.5 GB | public and platform tier blobs (published content: keep) |
| `capture-bodies/` | 9.6 GB | platform-tier blobs owned by the object graph, or dropped where the corpus has them |
| `mail` and maild | small | inbound mail: a sealed blob plus an input-queue row until the owner's computer appends it; owner's emails kept |
| `sha256/platform-update`, `guest-releases`, `builder/releases` | 19 GB | internal-tier release blobs; expire by rule (current plus rollback) |
| `artifact-gc-log`, `tape-sweep-archive` | 0.8 GB | closed; deletions become acts and audit entries; the old logs are archived |
| `offhost-backup` | | WAL archive, pack backups and the audit anchors |

## 9. What goes away

- Dolt everywhere: Store A, Store B, the guest stores, and the second copy
  of Store A that the nightly backup makes on the same disk.
- Projection bases, the host replay workers that build them, checkpoint JSON
  and its scratch space.
- The guest's copy of the event log (22 GB on the paper).
- Per-event pin receipts, per-event host receipt rows, and the legacy
  per-file pins.
- The synchronous round trip per event.
- The head-divergence repair path, and any host service writing a computer's tape.
- Reachability-inferred deletion of canonical blobs: most of object storage
  §4 (holder resolvers, stream sightings, condemn and release, holder-scoped
  holds). Rule-based expiry stays for derived tiers.
- Host plaintext of user state.

## 10. Owner decisions this frame needs

None of these blocks writing the detailed designs. A recommendation goes with each.

1. **Event classes.** Should "never offload" cover acts only, with working
   state kept in full only inside the restore window? Recommended: yes. The
   measurement in §1 will show how much of the tape is working state.
2. **The paper's window and its offload.** Recommended:
   - a 7-day window;
   - working-state segments go off-host to cold storage until the
     analytical database exists;
   - acts are kept forever.
3. **History before the cutover.** Recommended:
   - keep the old segments and pins as one sealed, compressed archive;
   - the new tape's first event names the old head, so the chain is
     unbroken;
   - the restore window starts at the cutover, and the migration snapshot
     is the base.

   Pre-launch, so the lost replay depth is acceptable.
4. **Single writer.** No owner decision is needed unless the review finds a
   host-written computer event that cannot move to the input queue.

## 11. What happens to the two detailed designs

Both detailed designs are cut to fit the frame:

- **Object storage** keeps:
  - its interface (§2);
  - tenancy and keys (§3);
  - physical layout (§5);
  - backend choice (§6).

  Section 4 is replaced by rule 4. Tape segments and snapshots become blob
  classes. Its cutover merges into the single cutover below.
- **History, restore and audit** keeps:
  - the gap list (G1 to G12);
  - the audit log, anchors and export.

  The restore, broken-computer and key-delivery flows are rewritten as in
  §6 and §7. The two-copy material goes.
- **One cutover** (owner: "measure a dozen times, cut once"). Choir moves
  from the old system to the new one in a single cutover:
  1. drain all writers;
  2. move Store A and Store B to PostgreSQL;
  3. pack the blobs;
  4. each computer boots a migration release that turns its Dolt state into
     SQLite, seals a first snapshot and appends the first event of its new
     tape;
  5. switch.

  Rollback is the previous release, with the old directories kept read-only
  for a week. Choir never runs two data systems.

## 12. Numbers this frame stakes

These are recorded in the precommitment ledger as a new round when the panel
reviews the frame:

- per-event overhead on the tape of at most 200 B;
- control database without tape receipts under 1 GB;
- the paper's SQLite holding state only between 3 and 8 GB;
- restore to any point in a 30-day window within 5 minutes for a heavy
  computer;
- the barrier adding at most 50 ms at p99 to an effect.
