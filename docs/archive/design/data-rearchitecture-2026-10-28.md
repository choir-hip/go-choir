# Data rearchitecture for resource optimality, stability and security (2026-10-28)

Status: direction, not ratified. Mutation class of the work it proposes: red
(store substrate, checkpoints, tape). Author: FP0, from the owner's prompt of
2026-10-28: "the data architecture remains the major bottleneck. As long as we
have regular memory and storage issues we aren't ready to build autoradio...
throw more hardware at the problem isn't the solution because efficiency is
margins". Inputs: docs/design/three-dolt-stores-2026-10-28.md (with FP0's
correction), docs/problems/paper-never-checkpointed-2026-10-27.md,
docs/problems/store-a-weekly-squash-2026-10-20.md,
docs/design/store-engine-concurrency-2026-10-21.md. [V] verified, [H]
hypothesis.

## Diagnosis

Most of our data is immutable (events, captures, stories, source entities,
revisions) and we keep it in a versioned, mutable SQL engine, then copy it:

- an event lives in tape segments on the host [V], again as full JSON in the
  computer's `computer_event_index` (about 12 GiB of the paper's 26.5 GB) [H],
  again in every projection base (183 bases, about 58 GiB) [V], again in a
  capture clone (about 72 GiB transient growth) [V], and the witness reads the
  store about twice to checkpoint it (1.5 TB read for the paper) [V];
- Store A retained 120,335 commits, 50.9 GiB [V]; snapshot commits are
  cadence/debounced, not necessarily one per SQL write; its weekly squash
  OOMs at 10G [V];
- the paper's 503s are mostly the online GC holding the one write
  connection (81% of non-200 samples) [V], and the engine mutex second [V].

The shared architectural problem is unnecessary retained version history,
duplicated payloads and expensive, insufficiently bounded operations.
Immutable rows are not intrinsically a bad fit for SQL. Individual stalls
still need their own mechanism evidence; this is not one proven engine bug.

## Target: three tiers, one truth

1. **The tape is the canonical semantic history of a computer**:
   append-only, hash-chained and signed. Target projections are rebuildable;
   snapshots accelerate recovery rather than create another history authority.
   This is not yet complete: actor recovery, files, keys and direct-write
   state have gaps (the portability inventory). Shared control, identity,
   corpus and vault state also need their own explicit recovery contracts.
2. **Substantial immutable content in a content-addressed store, encrypted
   per owner**: reference bodies by hash; keep small metadata in transactional
   rows when useful. One canonical home does not prohibit bounded guest caches,
   replicas or backups. The corpus already externalizes capture bodies [V].
3. **Small transactional current state per computer**: runs, obligations,
   document heads and refs. Size is a census hypothesis, not established MB.
   SQLite WAL is a candidate, not a selection; LMDB, MDBX, bbolt and LSM
   candidates remain open. Host lifecycle, route and lease authority remains
   host control state. Dolt's table-root witness needs a replacement; current
   Texture history uses explicit revisions and self-development uses capsules,
   not a demonstrated production requirement for Dolt branch/merge.

## What it buys

- Target checkpoint = tape head + current-state snapshot + content closure +
  projector/schema/release identity and recoverable keys; actor state must
  have an explicit inclusion or reconstruction contract. Small/fast capture
  and a substrate-neutral witness are hypotheses to prove, not shipped gains.
- Memory: delete unnecessary history and payload duplication, then measure
  engine, query, maintenance and guest file-cache peaks. A cache target is
  not a bound on whole-VM memory.
- Stability: WAL permits concurrent snapshot readers and one writer, not
  contention-free operation. Bound reader lifetimes, transactions, checkpoint
  progress and disk headroom; other engines have different reclamation costs.
- Security: fewer uncontrolled private copies and least-privilege reads.
  Crypto-erase requires destroying all applicable key protectors, including
  recovery/backup copies; escrow existing is not proof of that property.
  Dedup within an owner, and across owners only for published public objects.

## Existing replacements first (standing rule)

Built and off for every computer (switches/registry.yaml, grandfathered):
`store_event_table`, `store_debounced_checkpoint` (guest Dolt history off the
write path), `store_trace_log`; and the `choir_events` tape-stamped routing.
Store A's snapshot control is separate. Read why each is off and walk it
before writing replacement storage code.

## Path (each step leaves a working system)

1. Measure: read-only per-table size census of the paper's captured base,
   Store A and the corpus. Confirms or kills the 12 GiB tape-index hypothesis.
2. Stop the bleeding with what exists: walk `store_debounced_checkpoint` and
   `store_event_table` from d@e; a projection-base retention policy (latest
   few per computer plus epoch roots; S4 parts dedup).
3. The tape index stops storing `event_json`; reads come from tape segments.
4. Immutable kinds move to the content-addressed store (paper stories and
   source entities first).
5. Compare authority engines on the same logical workload and on idiomatic
   physical layouts; compare host control and corpus separately. The
   [expanded alternatives research](../evidence/storage-engine-alternatives-2026-10-29.md)
   keeps SQL, COW KV and LSM candidates open.
6. Prove portable checkpoints and retire the heavy capture path only after
   equivalent state, witness, recovery and content/key completeness are shown.

## Owner questions

1. Which current-state and host engines meet the semantic/recovery gates with
   the best lifetime resource use, including the replacement Dolt witness?
2. Dedup across owners for published objects only, within an owner for
   everything else?
3. Autoradio waits for step 4 (the paper stops being the sizing problem)?
   FP0 recommends yes.

## Switch audit (2026-10-28, from docs/evidence/switched-off-inventory-2026-10-26.md)

- `store.debounced_checkpoint` [V]: guest Dolt history commit off the write
  path (60 s / 10,000 ops). Shipped in deploy R (0bf8a097d, seven review
  rounds). The only missing proof is the kill -9 durability drill on a
  throwaway test computer. Next action: run the drill, then walk R0 -> R5.
  This is a resource win on the paper's write path and history growth now.
- `store.trace_log` [V]: no reader; R7 (docs/design/trace-segment-log-2026-10-22.md,
  "trace lines leave the tape and the store") S3-S6 not landed. Building R7
  is a step-3-class cut: trace lines out of the store.
- `store.event_table` [V]: ten telemetry kinds to `choir_events` (F-c); off
  pending the stream_seq lease readiness, the host grant
  `CHOIR_EVENT_TABLE_COMPUTERS`, and the R7 decision (R7 may make it
  redundant).

So step 2 starts with the kill -9 drill for `store.debounced_checkpoint`,
and steps 3-4 should begin by finishing R7 rather than new storage code.

## Working hypothesis after the owner discussion (2026-10-28 afternoon)

To be tested by docs/definitions/choir-data-architecture-selection-2026-10-28.md,
not yet decided. The owner's aim: intelligibility (hold the whole system in
one head), resource efficiency, stability and security; storage designed
and operated in-house on proven open-source parts, not novel engines.

- Per-computer state: SQLite WAL is the initial relational hypothesis;
  concurrent readers and a single writer fit the model. It still needs a
  checkpoint policy and whole-VM memory measurement. LMDB, MDBX, bbolt,
  RocksDB/Pebble and adjacent candidates are open for research, not rejected
  for requiring additional setup.
- Shared state: PostgreSQL is the initial hypothesis; Oracle MySQL/InnoDB
  and MariaDB/InnoDB are first-class alternatives. Choose control and corpus
  separately. Two independent instances are the review preference for blast
  radius; logical databases/schemas alone do not provide equivalent isolation.
- Immutable bodies: local CAS first, with an object-store API for a later
  self-hosted backend. The earlier MinIO suggestion is superseded by the
  research/amendments in the selection goal; no backend is deployed by this note.
- Retrieval: SQL graph and full text first where they meet the query contract;
  include all index maintenance in comparisons. Qdrant removal/pgvector later
  is the owner direction below, not evidence that PostgreSQL has won selection.
- Off-host durability: asynchronous tape/WAL/object shipping can lose recent
  acknowledged writes on host loss. Zero-loss acknowledgements need an
  independently durable receipt before success. The current selection scope
  uses local fsynced tape; off-host receipts remain later work.
- In-house means owning and operating the architecture and building our
  own layer (tape, content addressing, projections, witness, streaming),
  not writing database engines.
- Dolt taught the requirements by failing them: the tape as truth,
  immutable content addressed, small authority state, bounded memory,
  data that can move between hosts.

## Decision (owner, 2026-10-28): remove Qdrant; vectors later via pgvector

Owner: "Should we just remove Qdrant and plan on using PG Vector? ... I
don't think the vectors matter until we have enough data that we've
exhausted our ability to effectively query it with graph traversal through
SQL." FP0 evidence [V]: Qdrant on Node B holds 124 KiB and uses about 5 MB
(an empty index); it listens on host loopback only, while the pipeline
would run in guests (nix/node-b.nix:814-819); the runtime's semantic dedup
(internal/agentcore/qdrant_runtime.go, internal/qdrant: client, pipeline,
Ollama embedder) sits behind a breaker that falls back to content-hash
dedup, which is what runs. Delete: services.qdrant, internal/qdrant, the
runtime wiring, its health and provider entries (orange; tier 1). Vectors,
when graph traversal in SQL stops being enough and an embedding pipeline
exists, use pgvector in the shared Postgres; a dedicated vector engine is
a later replacement behind the same retrieval interface.

## Review and owner-requested reopening (2026-10-29)

The owner requested a deeper alternatives comparison before accepting the
default pair: extra initial setup is acceptable for better long-term resource
utilization or stability. The
[research and primary-source ledger](../evidence/storage-engine-alternatives-2026-10-29.md)
records the positive case, maintenance mechanism and falsifier for each
candidate. It is green research, not ratification or an execution design.

Review perspectives carried forward:

- Do not replace an engine without correcting unnecessary history, payload
  duplication and recovery gaps; do not move every immutable metadata row
  to a custom file format merely because it is immutable.
- Owner clarification: use the same host engine by default, but **separate
  independently operated control and corpus services**. Corpus requirements
  drive that engine choice; do not add a control-only engine without a concrete
  unmet requirement. Separate services should permit corpus to move to its own
  machine and later infrastructure without moving control with it.
- A purpose-built KV graph is credible if audited named query shapes suffice.
  Its atomic secondary indexes, query semantics, migrations and logical
  witness must be included in correctness and resource accounting.
- Compare equal durability first: fully synced commits for irreplaceable
  state; weaker sync only for proven projections with already-durable inputs.
  A process kill is not a VM/OS crash or host/storage-loss proof.
- Current-state rows and projection watermark remain one transaction.
  Prepare, host CAS and local finalize remain a cross-system recovery
  protocol regardless of the engine.
- Fast stored roots are not independent proof that rows are correct.
  Canonical witness maintenance needs schema/crash coverage and an
  independent recomputation path; no new hash construction is selected here.
- Measure sustained maintenance debt, cold-open/restore peaks, post-scan
  guest-cache residency, temporary disk and backup retention. Extra setup is
  acceptable; postponing unbounded work beyond a short benchmark is not.
- Portable recovery must include or explicitly reconstruct actor state,
  files, keys and off-tape kinds, with the source unavailable. Engine choice
  does not make those missing contracts disappear.
- Priorities: security first, stability second, scalability and simplicity
  jointly next. A few more setup steps are acceptable for durable resource
  savings; minimizing engine variety does not mean combining failure domains.
- The [control/corpus resource investigation](../evidence/control-corpus-resource-profile-2026-10-29.md)
  maps current control duties and distinguishes live state, receipt retention
  and Dolt history. Its multi-hour active-paper reconstruction includes actual
  publishing and corpus memory-limit failure. The initial 30-second idle
  sample is explicitly excluded from workload and selection evidence.

