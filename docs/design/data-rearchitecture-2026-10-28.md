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
- Store A commits per write: 120,335 commits, 50.9 GiB [V]; its weekly squash
  OOMs at 10G [V];
- the paper's 503s are mostly the online GC holding the one write
  connection (81% of non-200 samples) [V], and the engine mutex second [V].

Every symptom this week (GC OOM, store_stalled, two-hour capture, deploys
blocked on disk) is one disease: mutable versioned storage for immutable data,
plus copies of copies. More hardware moves the cliff; it does not remove it.

## Target: three tiers, one truth

1. **The tape is the only source of truth**: append-only, content-addressed,
   hash-chained, signed (exists). Everything else is a derived projection,
   rebuildable and never checkpointed as if it were state.
2. **Immutable content in a content-addressed store on the host, encrypted
   per owner**: events, captures, stories, revisions, published objects,
   stored once and referenced by hash; computers hold read capabilities, not
   copies. The corpus already does this for captures [V]; extend that
   pattern. Publication to the Autopaper object graph becomes a shared ref.
3. **Small authority state per computer in a small transactional store**:
   runs, obligations, lifecycle, routes, leases (megabytes). Candidate:
   SQLite in WAL mode (one writer, concurrent readers by construction, small
   memory, no GC, no ICU cgo) [H]. Keep Dolt only where versioning is the
   product (self-development candidate worlds; Texture revision diffs if
   needed). Store A gets the same split: platform state with an audit log,
   not per-write commits.

## What it buys

- Checkpoint = tape head + authority snapshot (MB) + refs: capture in
  seconds; witness = hash chain + authority hash; the S2 clone path becomes
  the fallback.
- Memory follows the working set, not history: no Dolt GC, smaller guests,
  higher density (the margin).
- Stability: no contention by construction (WAL), no GC on the write
  connection, disk growth linear in new content.
- Security: fewer copies of private data; per-owner keys make deletion a
  crypto-erase (key escrow exists); least-privilege read capabilities; a
  small auditable authority store. Caution: cross-owner dedup leaks
  possession; dedup within an owner, and across owners only for published
  public objects.

## Existing replacements first (standing rule)

Built and off for every computer (switches/registry.yaml, grandfathered):
`store_event_table`, `store_debounced_checkpoint` (would cut Store A's
commit-per-write), `store_trace_log`; and the `choir_events` tape-stamped
routing. Read why each is off and walk it before writing new storage code.

## Path (each step leaves a working system)

1. Measure: read-only per-table size census of the paper's captured base,
   Store A and the corpus. Confirms or kills the 12 GiB tape-index hypothesis.
2. Stop the bleeding with what exists: walk `store_debounced_checkpoint` and
   `store_event_table` from d@e; a projection-base retention policy (latest
   few per computer plus epoch roots; S4 parts dedup).
3. The tape index stops storing `event_json`; reads come from tape segments.
4. Immutable kinds move to the content-addressed store (paper stories and
   source entities first).
5. The authority-store engine decision (SQLite WAL vs small Dolt): a design
   note and a divergent panel; it touches self-development and Texture.
6. Checkpoints become tape head + authority snapshot; the heavy capture
   path retires.

## Owner questions

1. Where is Dolt's versioning load-bearing: self-development candidate worlds
   only, or Texture revision diffs too?
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

- Per-computer authority state: SQLite (WAL) embedded in the VM. One writer
  per computer is the model; WAL keeps readers and the writer from
  blocking each other; the page cache is fixed-size and there is no GC.
- Shared platform state: Postgres on its own machine eventually (many
  writers, replication, built-in full text, pgvector). Alternatives kept
  open: SQLite per service if data ownership is strict; FoundationDB as
  the later scale-out option.
- Immutable content: a content-addressed object store (local disk now,
  self-hosted S3-compatible such as MinIO later), stored once, verified by
  hash.
- Retrieval: indexes derived from the log and the content store, one per
  question kind: full text (Postgres first), vectors (Qdrant already runs
  on Node B at 6333; pgvector up to tens of millions), entity graph and
  time series as Postgres tables first. Every index is rebuildable.
- The log off-host: tape segments and SQLite WAL (Litestream-style) stream
  to the object store; Postgres WAL archiving for point-in-time recovery;
  a log bus (NATS JetStream or Redpanda) for the corpus ingest stream
  later. Losing a host then loses nothing.
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
