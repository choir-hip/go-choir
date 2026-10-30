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

## Revision 2 (2026-10-30): review round 1 folded in

This section is binding. Where text further down conflicts with it, this
section wins. Review round 1 ran at 3f4dafb7e:

- Devin: accept_with_notes, F1 to F5.
- Sonnet: send_back, B1 to B7.
- DeepSeek v4.1 flash: send_back, B1 to B7.

All three outputs are in `.agentic-consensus/frame-r1/out/`. The three
seats converged, and every finding below is adopted unless it is marked
*phase 2*. Phase 1 gets the fleet onto the new system (goal stations C0 to
C5); phase 2 is the I-stations in the goal. The owner directed that no
further review rounds run.

### The rules, amended

1. **Single writer, fenced.**
   - The writer epoch is the realization epoch, held in one lease row per
     computer in the control database.
   - The epoch travels in the append capability and is checked inside
     the append transaction. Renewing a capability never crosses an
     epoch.
   - The order is always: fence (bump the epoch), then stop the old VM,
     then read the head, then boot.
   - The computer ships a batch only after its local commit is fsynced
     (SQLite `synchronous=FULL` for the outbox).
   - The host mints capabilities after the credential-envelope exchange
     (`credential_envelope.go:216-241`); that closes the earlier "[I] who
     mints".
   - Today's refusal path is at `appender.go:733-743`. The
     prepare-then-recover path already closes the old crash window.
   - Single writer is policy plus code, not cryptography. *Phase 2*: each
     batch is signed by a computer key registered at genesis, so an
     export can tell computer-written events from host-written ones.
2. **The barrier, made closed.**
   - Gate-bearing semantic kinds stay synchronous: append, then apply.
     These are `effect_*`, `materialization_*`, `rollback_*`, `key_*`,
     lifecycle, restore and checkpoint. Only projection batches use the
     outbox.
   - Effects take the egressgrant classes (`internal/egressgrant`: read,
     idempotent-write, visible-write, money, irreversible).
   - *Phase 2* (I2): every egress passes a host mediator that checks the
     epoch and `receipted_head >= cause_seq`. The mediators are the
     gateway, maild, publish, vmctl apply and the owner websocket.
     Information-flow effects (model, tool, search and fetch calls) carry
     a request manifest in the agent record, and that manifest is
     receipted first.
   - The outbox holds events only. A waiting effect is an in-memory
     continuation, and a crash closes it as "interrupted".
   - The outbox applies backpressure: an agent step blocks when the
     unreceipted depth or age passes a bound.
   - Files are covered by file-sync events until phase 2 events every
     file write, so the loss bound for files is the sync interval.
   - `desk.egress` moves to the agent record.
3. **Restore by table composition, never rewinding acts or the agent
   record.**
   - Events are class-pure: the flusher splits a transaction's operations
     by class into separate events in the same batch.
   - Restoring to P takes the working-state tables from the snapshot at
     or before P, plus replay up to P across all classes. Act and
     agent-record tables are carried forward unchanged from the head. No
     events after P are replayed.
   - Act and agent-record kinds reference only acts, records or blobs.
     Pointer kinds are working state; a document head is one, so a rewind
     moves a document's head back to its revision at P, and the later
     revisions stay as history. Readers tolerate absent subjects.
   - Idempotency receipts are classed with the act they guard.
   - A rewind boots the release that was in force at P, or the owner's
     pick.
   - Restore is refused while a self-development apply is pending or a
     capsule is live.
   - A restore is a crash-class boot: it closes open work and never
     resumes.
   - The restore runs as an idempotent vmctl job.
   - All of this is *phase 2* (I1), except the migration snapshot, which
     is phase 1.
4. **Deletion by ownership.** Each blob class has one owner:
   - private blobs: the computer, which deletes by act;
   - public bodies: the Autopaper object graph, by a graph takedown act;
     a tenant never deletes them;
   - seed and release blobs: the platform, by release rule.

   How deletion works:
   - Uploads land as pending with a time limit. A receipted event that
     references them promotes them. That handles crash orphans such as
     chunks uploaded before a root commit.
   - At window expiry the computer computes the release set from a
     reference-count table: references of the expiring snapshots minus
     those of the surviving snapshots and live state.
   - Expiry of a non-act class is authorized by the class declaration, so
     it needs no act from the computer and an off computer cannot leak
     forever.
   - A physical purge waits at least 7 days in quarantine and respects
     legal holds. A lost delete act is recomputed, and a wrong one can be
     recovered inside the quarantine.
   - A destroyed computer is crypto-erased, then released by one platform
     act.
   - The rest of this is *phase 2* (I3). Phase 1 keeps today's CAS layout
     and deletes only what C5 names.
5. **Sealed at rest, honestly scoped.**
   - "Sealed" means at rest on disks, backups and blob stores. It does
     not protect against live root on the host.
   - The host reads in plaintext, and only these:
     - envelopes;
     - control kinds (verification decisions, lifecycle receipts);
     - gateway traffic and inbound mail in transit, with no bodies
       persisted;
     - the public and platform tiers;
     - the object graph;
     - sizes for quotas.
   - Before release, audit is detective for sanctioned unwrap paths only.
     An operator with root can unwrap offline. The computer key is also a
     file on the guest data image (`autoputer/run.go:335-359`).
   - Phase 1 seals new tape payloads under the computer key from the
     first new event.
   - *Phase 2* (I6):
     - rotate each computer key, take the key off the image and deliver
       it into guest memory at every boot;
     - put the escrow private key under an off-host KEK;
     - per-object keys with a key commitment (XChaCha20-Poly1305 does not
       commit to its key);
     - per-tenant wrapping keys, so a deletion beats old backups;
     - reads of envelopes by internal callers are audited.
6. **Class fixed by the registry.**
   - A kind's class is set by the registry per release, not by the writer
     at run time.
   - The host enforces a floor for kept kinds: commitment records,
     Texture revisions, owner acts, sent messages, publications,
     restores, deletions and source-graph judgments. A batch that labels
     one of these lower is refused. That closes the "hide it by
     retention" attack.
   - The source graph's judgments are acts. Devin, Sonnet and DeepSeek
     all said so.

### The PSA proof, stated honestly

- A receipt plus its covering anchor proves that a record existed no later
  than that anchor.
- Backdating is bounded by the anchor cadence. Tape and audit heads are
  anchored hourly, and *phase 2* (I5) adds a record-only Merkle root every
  5 minutes.
- Against the host itself before release, integrity rests on the audit
  trail. It becomes enforceable when computer-signed batches land and the
  owner's keys leave escrow.
- An outcome act carries the receipt reference of its precommitment, so
  "predates" is a property of the chain and not a claim about clocks.
- *Phase 2*: a precommitment whose outcome is time-sensitive waits for
  anchor coverage before the outcome effect, and record-witness bundles
  are kept off-host.

### The five things, adjusted

- **PostgreSQL** is one instance with two databases, control and the
  object graph. It splits into two instances only if measured load
  demands it.
- **Telemetry** goes to the host journal with short retention. That is an
  operational log, not a data home.
- **Batch receipts** live inside the tape segments. The control database
  holds only the segment index, one row per segment.
- **The audit log** absorbs the escrow transparency log
  (`platform/key_escrow.go:295-335`). An export carries the audit root, the
  log size and a consistency proof against the previous export.
- **Per-class tape streams.** Each class gets its own segment stream under
  one global sequence, and one batch receipt binds all stream heads. Expiry
  deletes whole segments and never rewrites one. *Phase 2* (I4); phase 1
  writes one stream with class tags on each event.

### Scale, corrected

- Each heavy computer costs about 8 GB of tape for a 30-day window, plus a
  5.7 GB snapshot base, plus deltas that are not yet measured.
- Node B had 99 GB free at 05:57 UTC on 10-30. After the cutover deletes
  the old data, the free space should be about 300 GB [I].
- The point at which an off-host blob backend becomes necessary is
  therefore about 15 to 20 heavy computers, not 40.
- A disk alarm and a shed policy come with the backend: shed snapshots
  first, then non-act tape, before PostgreSQL's write-ahead log fills.
- The §12 window stake below covers the tape only.

### The cutover, adjusted to the owner's 06:40 UTC direction

- Rollback is fix-forward. The old data is deleted once the differential
  and keep-set checks are green, so there is no read-only week. That also
  removes the plaintext residual DeepSeek noted.
- Before it migrates, each computer checks that its local head equals the
  host head.
- The first new-tape event commits to the archive digest and the old head.
- The pre-cutover precommitment records are migrated as acts in the
  state, so their content survives. Their timing evidence stays in the
  sealed archive.
- The off-host restic snapshot of 2026-10-29 03:28 UTC is the last copy of
  the old system.

### §12, restated

- Per-event overhead on projection batches is at most 200 B. Semantic
  kinds keep their full envelope.
- Restore cost is the snapshot load plus replay of at most one snapshot
  interval. Snapshots are taken every 10,000 events or every hour,
  whichever comes first.
- The other stakes in §12 stand.

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

What the paper's tape holds, from
[paper-tape-composition](../evidence/paper-tape-composition-2026-10-30.md)
(2026-10-29, replicated on 10-27 within 2.3 points):

- **Volume.** 339 K events carry 1.535 GB of row operations. 92% of events
  hold a single operation.
- **Today's overhead.** Each event costs about 10.4 KB of fixed overhead,
  counting its segment record, its host receipt row and its guest row,
  against the 4.5 KB of operations it carries.
- **Little churn.** Only 15.8% of operations overwrite a key written the
  same day, so "keep the latest value per key" alone still keeps 82% of the
  bytes.
- **Coalescing.** Merging the writes of each 1 s window into one event would
  cut events and receipts by 82%. That gain is the fixed cost per event; the
  frame's slim envelope and batch receipt (§4) remove most of the same cost
  without merging events.
- **The telemetry batcher is off.** It already exists
  (`internal/store/telemetry_sealer.go`), but none of two days' 684 K events
  went through it.

What a heavy computer costs at the target, with telemetry off the tape (§5):

| | Raw per day | Compressed (5.4x, measured on today's segments) |
|---|---|---|
| Inside the window | about 1.4 GB | about 0.26 GB, so a 30-day window is about 8 GB |
| After the window, acts only | | about 60 MB, or 22 GB a year |
| After the window, acts plus the agent record | | about 155 MB, or 56 GB a year |

Node B has 476 GB, so the blob backend must be able to leave Node B before
about 40 heavy computers keep 30-day windows. That is why the tape and
snapshots go through the blob interface (§2) and nothing assumes a local disk.

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
4. **The Autopaper object graph** (PostgreSQL, a second database on the same instance; revision 2). Published objects
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
   (`internal/computerevent/appender.go:224-243`).

   Checked 2026-10-30: single writer is already how the code behaves; the
   frame makes it a rule.
   - An append needs a per-computer `event:append` capability. Internal
     host callers are allowed to read only
     (`internal/platform/event_handlers.go:324-348`).
   - Every appender call site outside `internal/computerevent` is in a
     guest package.
   - The one exception is the host projection-base rebuilder. It replays
     into a scratch store in replay mode (`internal/projectionbase/rebuilder.go:385-396`)
     and goes away with the bases.
   - [I] Who can mint the capability is not traced.
   - The host does delete tape files: the tape sweep removes orphan pins and
     destroyed computers' files (`internal/platform/tape_sweep.go:29-41`).
     Under rule 4 those become acts.
2. **Nothing leaves a computer before its cause is receipted.** Appends are
   batched (§4). An effect that leaves the computer (an email sent, a
   publication, a precommitment that must predate its outcome, a
   self-development accept, anything involving money or credentials) waits for
   the receipt of the batch that records it. A crash loses at most the
   unshipped tail, and that tail never had an effect outside the computer.
3. **Restore is a forward event, and it never rewinds acts or the agent
   record.** Restoring to time P appends a new event; history is not cut.
   Working state and files go back to P. Acts and the agent record (§5) stay:
   a precommitment, a sent email or a publication made after P is still a
   fact after the restore, and so is what the agents saw and said.
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
   also declares its class (act, agent record, working state or telemetry,
   §5) and, for a log kind, when its rows retire from the state, so the
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

There are four classes. The byte shares are measured on the paper's tape
(10-29); the assignment of kinds to classes is the measurement's own, which
§10, question 1 asks the owner to settle.

| Class | Paper bytes | What it is | In the restore window | After the window (users, default) | After the window (the paper) |
|---|---|---|---|---|---|
| **Acts** | 20.5% | owner acts, Texture documents and revisions, precommitment records and outcomes, the source graph and its judgments, sent messages, publications, adoptions, self-development decisions, restores, deletions | full | kept for the life of the computer | kept; public ones also in the Autopaper object graph |
| **Agent record** | 33.7% | what each agent saw and said: run memory (20.1%) and tool results (13.6%), which run acceptance and the Texture verifier read as evidence | full | kept (recommended, §10) | offloaded to the analytical database (later); until then, off-host cold storage |
| **Working state** | 34.1% | mutable state: runs, schedules, heads, outboxes, leases (24.3%, the only part that churns). Bookkeeping logs: command receipts, lifecycle and run-transition events (9.8%) | full: any point can be restored | only the snapshot at the window's start | the same |
| **Telemetry** | 11.7% | trace lines (progress, retries, cell and egress traces), UI-state copies, heartbeats | not on the tape: logs and metrics with short retention | | |

- Restore windows are 7, 14 or 30 days by plan (owner, 2026-10-30).
- "Users never offload their event records" (owner, 2026-10-30) is read as
  covering acts and the agent record. Working state is the computer's
  scratch: its history inside the window is what a restore needs, and
  nothing reads it afterwards.
- The paper's precommitment records are acts. Today they are about 10.8 K per
  day at 3.2 KB, or 35 MB per day kept forever [V].
- State and logs need retirement rules in the state itself, not only on the
  tape. A log kind (run memory, command receipts) declares when its rows
  leave the computer's SQLite, for example when the run closes. Its
  record then lives only on the tape.

  The 5.7 GB paper store in §1 already assumes run memory leaves the store
  (measurement state C).

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
2. replays every act and agent-record event after P on top;
3. sets the file root to its value at P;
4. seals a snapshot;
5. appends one restore event naming that snapshot.

Replay treats a restore event as a load point, so later replays never need
the tape before it. The host's part is an audit entry for the key unwrap.
Acts and the agent record survive. Anything outside the window is refused
with the reason.

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

Sizes are from the [2026-10-29 inventory](../archive/evidence/node-b-storage-inventory-2026-10-29.md)
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

1. **Event classes (§5).** Two choices decide 34 points of the paper's
   bytes.
   - **(a) Is the agent record kept like acts?** **Settled by the owner,
     2026-10-30: "Yes, agent record is valuable in many senses. Keep it."** The agent record is run
     memory and tool results. Kept like acts, it stays forever for users; as
     working state, it is kept only inside the window. Recommended: keep it.
     It is the audit trail of why an agent did what it did, which PSA,
     self-development review and compliance all read. It costs about 34 GB a
     year per heavy computer on top of acts, which an off-host backend
     absorbs.
   - **(b) Does working state leave after the window?** Recommended: yes.
   - **Also:** the classification of the source graph's judgments as acts
     (10.5% of bytes) should be confirmed.
2. **The paper's window and its offload.** Recommended:
   - a 7-day window;
   - the agent record goes off-host to cold storage until the analytical
     database exists, about 95 MB per day compressed;
   - working state leaves after the window, as for users;
   - acts are kept forever.
3. **History before the cutover.** Recommended:
   - keep the old segments and pins as one sealed, compressed archive;
   - the new tape's first event names the old head, so the chain is
     unbroken;
   - the restore window starts at the cutover, and the migration snapshot
     is the base.

   Pre-launch, so the lost replay depth is acceptable.
4. **Single writer.** No owner decision is needed: the code already behaves
   this way (rule 1). The review confirms who can mint the append
   capability.

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
- the barrier adding at most 50 ms at p99 to an effect;
- a heavy computer's 30-day window under 10 GB compressed, with telemetry
  off the tape.
