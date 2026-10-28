# Data architecture consensus panel, 2026-10-28

Status: evidence for docs/definitions/choir-data-architecture-selection-2026-10-28.md.
It is not a ratification. The goal's decision rule still governs, meaning the
census, the bake-off and the d@e differential. Owner request: "Run agentic
consensus — Devin and DeepSeek — in divergent, lateral, then convergent...
review what they got between rounds." Panel: Devin (account default) and
omp-deepseek-v41-flash. Both were allowed to read the repo and search the web.
Raw outputs are in `.agentic-consensus/data-arch/r{1,2,3}/`, which is
gitignored session diagnostics. FP0's between-round reviews are in
`r1-synthesis.md` and `r2-synthesis.md` in the same folder. They are
condensed below.

## Round 1, divergent

The panel produced about 110 options. FP0 grouped them into ten families:

1. The tape is truth and every store is a projection.
2. A small embedded authority engine (SQLite, libSQL, LMDB or bbolt).
3. The store moved out of the guest (host-side per computer, a multitenant
   SQLite runtime, or a Neon-style page server).
4. The VM is the database (Firecracker snapshots).
5. Shared-tier engines (Postgres, rqlite, FoundationDB, CRDT gossip, XTDB,
   per-service stores).
6. Content-addressed object stores (a CAS directory, SeaweedFS, MinIO,
   Garage, OCI, Tahoe).
7. Retrieval (Postgres for everything, Tantivy segments as artifacts,
   scan-first DuckDB, per-computer FTS5, entity graph first).
8. Memory control (a PSI controller, free-page reporting, snapshot sharing,
   static allocation, a thin guest with a host worker pool).
9. Deploys (projection cutover, blue-green, in-guest re-exec, lease
   handoff, live migration).
10. Cross-cutting framings (physics-first scoring, crypto-shredding first,
    telemetry secession, the tape as a transparency log, no GC ever).

FP0 flagged five gaps:

- Nobody measured what dominates resident memory.
- The paper may need different physics.
- Texture and capsules under a store that holds megabytes.
- The migration path.
- An undefined "zero downtime".

## Round 2, lateral

Both panelists converged independently on these points:

- **Authority is refs; content is objects**, the Git and Maildir pattern.
  Texture history is already a parent chain addressed by content
  (internal/store/texture.go:1235). The largest win does not depend on the
  engine: it is deleting duplicated event JSON, history nothing reads, and
  copied projections.
- **Two data physics, one substrate.** The paper ingests and publishes.
  Personal computers are desks. The paper's 26.5 GB is a corpus-tier archive
  and should not size the per-computer engine.
- **The tape is the WAL; the embedded store is the heap.** Durability
  belongs at the tape append with a receipt from off the host.
- **Memory is page cache and the shape of operations, not engine bytes.**
  Measured on the paper, host RSS is about 8.1 GiB:
  - about 4.7 GiB of guest page cache, of which 4.2 GiB is cold Dolt files;
  - 0.7 to 2.6 GiB of daemon anonymous memory, including Dolt table indexes
    at about 25 MiB of heap per GiB of live store, a floor that grows with
    history.

  Sources: start-refused-for-memory-no-reclaim-2026-10-26.md,
  daemon-memory-res18-2026-10-22.md and guest-memory-split-balloon-2026-10-14.md.
- **Zero downtime is a property of where the session is anchored, not of the
  store.**
- **Migration means re-derivation from the tape, not moving data.**

FP0 fact-check: Devin called DAMON page-cache reclaim "proven at 1.32 GiB".
The source calls that one uncontrolled reading. A separate run returned
nothing in a 16-minute idle window. Returning guest page cache to the host is
**unproven**. Free-page reporting works only for freed anonymous memory.
Balloon inflation is not built.

## Round 3, convergent: the two verdicts agree

| Tier | Verdict (both seats) | Confidence |
|---|---|---|
| Truth and durability | The semantic tape is the only truth. The acknowledged-write boundary is the tape append plus an off-host receipt. WAL shipping (Litestream-style) only speeds things up, because it is asynchronous and can lose recent writes. The tape is never the engine's physical WAL, which would lose substrate-neutral replay. | high |
| Per-computer authority | Refs, leases, runs, lifecycle and Texture heads, in the guest. The projected engine is SQLite WAL, which already runs in every guest as the actor log (internal/actorruntime/adapter.go:105), with `synchronous=NORMAL` and a fixed cache. Dolt with the new data model ("Dolt-small") is the control and may win. | high on shape, medium on engine |
| Immutable content | A host-side CAS encrypted per owner. Dedup within an owner, and across owners only for public objects. Crypto-erase deletes. A local directory now; SeaweedFS later (both seats prefer it to MinIO). | high on model, medium on backend |
| Paper and corpus | The paper keeps a small, ordinary authority store. Its bulk moves to the corpus tier: the tape as log, CAS bodies, Postgres metadata and derived indexes behind corpusd. | medium |
| Shared platform (Store A) | Postgres with an audit table instead of a commit per write. The 120k-commit history is archived read-only, never converted. Partition by computer; no single global state domain (the lesson of Fly's Corrosion outage). | high to 10x |
| Retrieval | Postgres FTS and the entity graph first, pgvector later. Private search runs on FTS5 in the guest. Every index can be rebuilt. No new search or vector engine now. | medium |
| Checkpoint and restore | Tape head + authority snapshot (SQLite backup API) + projector version + a logical root that costs O(tables). Firecracker snapshots stay the hibernate cache behind the planned-restart marker, never the checkpoint. | medium (the witness is unbuilt) |
| Memory | Delete bytes first (event_json, content to CAS). Bound every operation. Measure smaps anon and file, not table bytes. No KSM, no guest swap, and no relying on page-cache return. | medium |
| Deploys | Classify each deploy by layer: an agent change re-execs inside the live VM; a rootfs change gets a new VM, a route flip and a drain; a VMM change gets a drain (Cloud Hypervisor live migration is the escape hatch). The media relay and jitter buffer live at the host edge. Zero downtime means no dropped call, a p99 audio gap of at most 250 ms, no lost utterance, and at most one invisible reconnect per deploy. | medium |

Explicitly rejected for now:

- FoundationDB, CockroachDB, Neon or rqlite;
- a shared or host-side SQLite;
- authority rows in shared Postgres;
- a VM snapshot as the checkpoint;
- Litestream as the acknowledgement;
- KSM;
- Qdrant or Elasticsearch;
- Kafka or NATS before fan-out is measured;
- a custom storage engine;
- leaving a dual-write layer in place after a cutover.

Disagreements the panel settled:

- **Engine swap this quarter?** It is not needed to reach 2 GB. The
  data-model change does most of the work on either engine. Swap only on the
  2x2 scorecard {Dolt, SQLite} x {incumbent model, refs+CAS}.
- **Projection durability:** NORMAL for authority. OFF only for derived
  indexes that can be rebuilt.
- **Store location:** in the guest. SQLite WAL cannot cross the VM boundary,
  and the density win from moving it is unmeasured (the RES-19 bench never
  ran).
- **The paper's store:** it keeps a small authority store. Content leaves it.

The seats disagree on one ordering question. Devin deletes bytes in
production (event_json, then content to CAS) before the bake-off. DeepSeek
builds the harness and runs the 2x2 before any live cutover. **FP0
adjudication:** byte deletion first. It cannot depend on the engine; it is
also the disk win; and the 2x2's incumbent-model cells can still be measured
from a frozen tape window taken before the deletion. That keeps the
attribution DeepSeek wants without leaving production to bleed.

The ordered path, merged:

1. **Census with memory shape.** The procedure exists. Add smaps_rollup,
   MemAvailable, balloon stats and per-file cache residency.
2. **Stop the bleeding with what is built:**
   - the kill -9 drill for `store.debounced_checkpoint`, then a ring walk;
   - Store A squash memory measured per phase on a reflink copy;
   - finishing R7;
   - deleting `telemetry.batch`, which R7 supersedes.
3. **Delete `event_json`** from the tape index and read bodies from tape
   segments.
4. **Move content-hash kinds to CAS plus refs**, with per-owner keys.
5. **Run the projector harness 2x2 on frozen d@e tape**, then the d@e shadow
   differential, then a per-computer migration by replay with frozen Dolt
   files kept as rollback.
6. **Shared tier and deploys:** Store A to Postgres, the corpus split,
   off-host segment shipping, the deploy classifier, the media relay and
   drain.

Kill criteria (merged). Each one names the measured result and what the
panel would change:

- Authority is over 1 GB per computer, or over 20% of bytes: redesign the
  authority model.
- Dolt-small passes every gate and the budgets: keep Dolt this quarter.
- SQLite fails the O(tables) witness, kill -9 or the differential: reject
  it.
- The engine accounts for less than 20% of the win after the data-model
  change: defer the swap.
- The guest and agent floor stays above 2 GB after deletion: thin the guest.
- The tape ack's p99 exceeds 100 ms, or it loses data: redesign shipping.
- Restore tail replay takes over 5 s or over 1 GB: change the checkpoint
  cadence.
- The audio gap exceeds 250 ms: live migration, or renegotiate the
  definition.

Top risks:

- **Migration stall leaving two homes for the data.** This was DeepSeek's
  "most likely". Mitigation: delete the old path in the same mission.
- **A third non-replayable kind beyond `agent_evidence` and
  `co_super_slots`.** This was Devin's "most likely". Mitigation: keep the
  differential harness as a permanent regression test.
- **SQLite has no `DOLT_HASHOF_TABLE`.** The O(tables) witness must be a
  logical per-table root maintained in the same transaction.
- **Page cache is one-way today.** Any ad-hoc scan inflates RSS again until
  the next hibernate.
- **Genesis replay of the paper is about 36 hours.** Restore must always
  start from the newest base plus the tail.

Open owner question raised by the panel: does "zero downtime" mean session
continuity plus a drain at the end of a run (Firecracker suffices), or no VM
restart ever (that would need Cloud Hypervisor live migration)?
