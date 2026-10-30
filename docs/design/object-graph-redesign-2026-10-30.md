# The object graph, redesigned (2026-10-30)

Status: proposal. The doc is green; the work it designs is red (store schema,
tape, Texture canonical writes, auth). It is reviewed like the change it
designs before a build follows it. Amended the same evening (§11): the
phase-1 panel, the owner's direction, and what phase 0 measured.

Scope: the per-computer object graphs, the Autopaper object graph on the
host, and the bridge between them. It builds on the data system frame
(`docs/design/choir-data-system-frame-2026-10-30.md`), which already settled
the write path (§4: apply locally, ship in batches) and the four event
classes (§5). This doc settles what the frame left open for the graph.

Judged on: security, stability, simplicity, scalability, and ease of
self-development (the engineering desk on a guest builds a new app with new
data types, and other computers may adopt what it publishes).

The phase-1 cutover was a port: it moved Dolt-shaped structures onto SQLite
and PostgreSQL. This is the redesign the new stack allows.

## 1. The short version

Keep I27's logical model: one writer per kind, schemas that travel with
their package, a substrate that stores kinds without knowing them, one model
in two scopes. Change everything under it:

1. **Kinds become package data, installed by an act.** Not Go literals
   compiled into two registries. The platform's own kinds become the first
   package. A guest-built app declares its kinds, schema, writer desk,
   indexes, class and publish policy in its manifest. Installing it is an
   owner-accepted act on the tape. No platform release is needed.
2. **The substrate stores each class in its own shape.** Records, logs,
   queues and cells (heads, counters, leases) stop sharing one upsert table.
   Each kind still has one home. The class picks the shape.
3. **Indexes and the witness are derived, never authoritative.** Indexes are
   declared by the package and built by the database, which removes
   `og_key_index`, its boot validator, `og_lookup_keys`, the capture
   projections and their reconcilers. The state witness hashes logical
   content incrementally, so adding an index or an app never changes it.
4. **Authority is enforced at one verb boundary, with real identity.** The
   writer gate enforces and the schema is checked on every write. The host
   never stamps itself as the writer of a guest's object. Every computer
   signs what it ships with its own key.
5. **The Autopaper graph holds immutable, signed versions.** A publication
   is an act on the publisher's tape, then a signed envelope the host
   verifies against that tape. The host learns kinds from published
   packages. Readers learn of changes from a feed cursor woken by
   `NOTIFY`, not by re-querying.
6. **The bridge is one path, not many.** The bridge carries batched signed
   tape segments, signed publications and adoption acts that name a version
   by hash. The bespoke publish routes, the generic object PUT, IP-bound
   forwarding and the per-event round trips are all removed.

Standing pat is the floor, and it works today. The case for moving is
measured below: about two thirds of the paper's store and all of the 16 GB
of host receipts are bookkeeping that this design removes. Two live
authority gaps close only when the writer is enforced. And the engineering
desk cannot ship a data type at all today.

## 2. What runs today (measured 2026-10-30, commit 16e6d80d7)

### The paper's store, 30.7 GB of SQLite

Per-kind payload in `og_objects` (body plus metadata bytes, from a private
reflink snapshot):

| Kind | Rows | Payload |
|---|---|---|
| `choir.event` (run telemetry) | 2.49 M | 4.86 GB |
| `choir.lifecycle_command` | 58.6 K | 0.73 GB |
| `choir.source_ref` | 199 K | 0.55 GB |
| `choir.texture_revision` | 46.9 K | 0.52 GB |
| `choir.run` | 72.2 K | 0.48 GB |
| `choir.commitment_record` | 132 K | 0.42 GB |
| `choir.inbox_command` | 126 K | 0.25 GB |
| everything else (31 kinds) | ~0.9 M | ~0.9 GB |

- The total is about 8.9 GB across 4.1 M objects. **No object of any kind
  has ever been tombstoned or superseded**: nothing retires.
- Beside the graph sit 5.15 M `computer_event_index` rows and 1.72 M
  `run_memory_entries` rows. With the indexes, they hold most of the other
  roughly 20 GB. The per-table byte count is owed: it was stopped when the
  paper's own validator stalled its store.
- The frame's 5.7 GB estimate assumed retention. Phase 1 deferred
  retention. That, not SQLite, is why the paper did not shrink.

### The guest graph

- **One upsert table holds four kinds of data.**
  - records: documents, runs, captures, verdicts;
  - logs: `choir.event`, lifecycle events and commands, inbox commands;
  - queues: `actor_wake_outbox` at 158 K rows, never pruned, plus inbox
    deliveries and mailboxes;
  - cells: about 7 head, cursor and sequence kinds, claims and leases.

  A counter bump rewrites a whole object and costs a whole tape event.
- **Kinds are code.**
  - The registry is 92 Go literals, all still tagged `StoreTypeDolt`.
  - Package kinds go through a generated Go file whose package list is
    empty.
  - `RegisterManifest` has no caller outside tests.
  - So a new kind needs a platform release.
- **Authority is advisory.**
  - The writer gate defaults to `observe`, and no Nix file sets it.
  - Runtime kinds (51 of 60 guest kinds) admit any writer.
  - The schema is checked only on the owner's HTTP route.
- **Derived data is maintained by hand.**
  - `og_key_index` serves four lookups, with a 944-line validator. It makes
    a full pass at every boot while holding the single write connection,
    and it caused the paper's `store_stalled` at 14:18 today.
  - Five generated columns are hard-wired to one kind
    (`commitment_record`).
  - The lifecycle stream loads the computer's whole graph every second, per
    open stream (`lifecycle.go:1969`, `api_trajectory.go:197`).
- **The witness is coupled to the schema.** Witness v3 hashes every row
  plus all `sqlite_master` text, so any new index or table changes every
  computer's witness, and there is no migration ladder
  (`storeSchemaVersion = 1`).
- **Conditional writes are not atomic in the database.** With the tape
  bound, the compare runs in a separate read transaction. In-process
  mutexes close the window (`sql_store.go:1412`).

### The Autopaper graph (host, PostgreSQL `graph`)

- 673 K objects and 507 K edges, 2.3 GB. It has the same generic
  `og_objects` shape, but with TEXT metadata and no generated columns. It
  carries the side tables `og_lookup_keys` (158 K rows) and
  `proj_capture_index` and `proj_capture_terms` (838 K rows), which are a
  hand-built inverted index because Dolt lacked a segmenter.
- **Writes go through two paths.**
  - Bespoke typed routes, one per publishable thing (Texture, captures,
    verdicts, relations, entities, releases, packages, listings).
  - A generic object PUT that guests can reach, guarded only by a
    four-kind denylist.

  corpusd stamps every write with itself as the writer
  (`objectgraph_store.go:938`). That makes per-kind single-writer
  unenforceable on the host. It also likely lets a computer write
  `choir.publication`, `choir.publication_version` and `choir.public_route`
  objects for its owner without the publish path. This is inferred from
  code and not yet drilled.
- **Integrity.**
  - Upserts overwrite in place.
  - `content_hash` is checked on generic PUT and never on read.
  - Review and attestation records are hard-coded `approve` and `passed`,
    authored by the subject.
  - No publication records the tape position of the act that made it.
- **Readers.**
  - There is no change feed.
  - A front page of 100 costs about 500 queries.
  - A published Texture tab re-runs the resolve chain (6 or more queries)
    every 60 s.
  - Nothing is cached.
  - The capture projection is a separate transaction, and a 10-minute
    `COUNT(*)` reconcile repairs it.
- **Bodies.**
  - Three file CASes sit outside the transaction.
  - Capture bodies have no GC namespace.
  - Object bodies are read back without a hash check.

### The bridge

- **The live cost per append.** Measured over 15 minutes, from 14:16 to
  14:31 UTC, with PostgreSQL statistics:
  - the fleet made 8,674 appends, or 9.6 per second;
  - the control database committed 121,612 transactions, or about 14
    commits per append;
  - the 54-row key-history table took 34,697 sequential scans, about 4 per
    append;
  - in the same window the graph database returned 4.6 M tuples, including
    15 sequential scans of `og_objects` that read 3.4 M rows, for 892
    inserted objects.
- **Every guest write is about five sequential host round trips** (head,
  head, pin payload, pin event, append), made under one appender mutex.
  The host then:
  - re-runs the reducer in a SERIALIZABLE transaction;
  - signs a receipt;
  - stores it as a 2.6 KB row (6.19 M rows, 16 GB);
  - takes one of 64 global hash-bucket locks, shared across computers.
- **Identity.**
  - Events are not signed by the computer. The host signs everything with
    one key.
  - There is no writer epoch.
  - A capability renews up to 15 minutes past expiry.
  - Tape routes use a signed capability, while graph routes use the
    source IP through vmctl and the proxy: two authentication systems.
- **Publication and adoption.**
  - A publication is not an act on the publisher's tape. The proxy pulls
    the document back from the guest.
  - Adoption exists only for source releases, by hash, with no publisher
    signature.
  - corpusd never learns `choir.app.*` kinds.

## 3. Five root causes

Each one shows up on both sides of the bridge.

1. **One physical shape for four kinds of data.** Records, logs, queues and
   cells all ride one versionless upsert table, because Dolt made new
   tables and DDL expensive. The result is unbounded growth: nothing
   retires, wakes are never deleted, counters cost whole objects. Hot paths
   get hard-wired per kind.
2. **Kinds are code.** The registry is compiled into two binaries that
   diverge. A new data type needs a guest release, a corpusd release and a
   bespoke route. Self-development stops at the data layer.
3. **Authority is advisory.** The gate observes. The host launders writer
   identity. A denylist guards writes. Two authentication systems coexist.
   No computer identity reaches the shared graph.
4. **The tape is synchronous per event.** Every write waits on several host
   round trips under one mutex, and each event leaves about 2.6 KB of
   receipt on the host forever.
5. **Derived data is hand-maintained.** Side tables, boot validators,
   reconcilers, dual writes and a witness that hashes DDL each patch a
   database that could not index JSON, segment text, or add a column
   safely. The new databases do all three natively.

## 4. What stays

I27's logical model stays, word for word:

- one writer per kind;
- one home per kind;
- package-declared schemas;
- extension, not restructuring, of another package's kind;
- the substrate stores, replays and restores kinds without knowing them;
- the graph is the tape's projection;
- the Autopaper graph is the same model in another scope, and an
  organization is a third scope.

The frame's write path, classes, keys and restore rules also stay. This
design is the physical layer and the protocols under them.

One deliberate extension: I27 says every meaningful kind of data is a graph
kind. This design keeps that, and adds that **a kind declares its class**.
Then the substrate can store a queue as a queue and a counter as a cell
while the kind keeps one writer and one home.

## 5. The design

### 5.1 A kind is package data

Every kind is declared in a package manifest. The platform's own kinds are
the `choir` package, shipped with the release, and are the same mechanism
as an app's kinds.

```json
{
  "package": "reading-list",
  "version": 3,
  "writer": "desk:reading-list",
  "kinds": {
    "choir.app.reading-list.item": {
      "class": "record",
      "retention": "act",
      "schema": { "$ref": "schemas/item.v3.json" },
      "schema_compat": [1, 2, 3],
      "indexes": [
        { "name": "by_url",    "fields": ["url"], "unique": true },
        { "name": "by_status", "fields": ["status", "added_at"] }
      ],
      "search": ["title", "notes"],
      "edges": { "cites": "choir.source_ref" },
      "publish": "owner-act"
    },
    "choir.app.reading-list.opened": {
      "class": "log",
      "retention": "telemetry",
      "ttl_days": 14
    }
  }
}
```

**Installing a package.**
- Installing is an act, `package_installed`, carrying the manifest digest.
  Uninstalling is also an act.
- The registry is a projection of those acts, so replay and restore
  rebuild it, and a restore to a point before the install has no such
  kind.
- The engineering desk builds the package in a capsule. The owner accepts
  the install as a self-development act behind the barrier. The app desk
  then becomes the writer of its namespace.

**Schemas.**
- A schema evolves additively within a kind: new optional fields, listed in
  `schema_compat`.
- A breaking change is a new kind plus a migration act, which I27 already
  calls a migration.
- The substrate validates every write against the declared schema, on every
  path, not only the owner's HTTP route.

**Indexes.**
- Indexes are declarations. The substrate turns them into database indexes,
  as below. Nobody writes DDL by hand, and a guest-built package cannot
  inject SQL: names are generated, and fields are JSON paths checked
  against the schema.

**Class floors.**
- The frame's rule 6 class floor applies. The host refuses a batch that
  labels a kept kind (commitment records, Texture revisions, owner acts,
  publications, judgments) below its floor. A package cannot declare those
  kinds anyway, because they belong to `choir`.

### 5.2 Storage by class (guest, SQLite)

| Class | Table shape | On the tape | Retires |
|---|---|---|---|
| **record** | `records(id PK, kind, schema_v, version, owner, content_hash, body JSONB, body_ref, created_at, updated_at, deleted)` | as put and delete ops, class-pure | by declared retention, or never (acts) |
| **edge** | `edges(from_id, kind, to_id, created_at, deleted, PK(from_id, kind, to_id))` | with its record | with its endpoint |
| **log** | `log_entries(kind, stream, seq, at, body, PK(kind, stream, seq))` | agent-record logs yes; telemetry logs no | TTL per kind; agent-record logs leave the store when their stream closes (frame §5) |
| **queue** | `queue_items(queue, id, enqueued_at, available_at, claimed_by, claim_until, body)`; consume is delete | as working state, inside the window | on consume; a crash boot closes open claims ("interrupted by a restart") |
| **cell** | `cells(kind, key, value, version, PK(kind, key))`; compare-and-set in the transaction | as small set ops | with the owning record, or never |
| **device state** | `device_state(key, value)` | no (I27 named exception) | when replaced |

**Records.**
- `body` is SQLite JSONB.
- Bodies over 16 KiB move to the computer's CAS (I27 already says blob
  bytes live there), with `body_ref` holding the digest.
- `metadata` disappears. Today it duplicates body fields, and the
  duplicate also flows into the content hash.

**Declared indexes** become partial expression indexes:

```sql
CREATE INDEX ix_<hash> ON records(json_extract(body, '$.url'))
  WHERE kind = 'choir.app.reading-list.item' AND deleted = 0;
```

- They are always consistent, because the database maintains them in the
  write's own transaction. That removes the reason for `og_key_index`,
  which was rollback safety under Dolt's upserts.
- `search` fields feed an FTS5 table maintained in the same transaction.
- The four `og_key_index` families and the five `commitment_record`
  columns become ordinary declarations in the `choir` package.

**Conditional writes.**
- The compare and the mutation run in one SQLite transaction (`version` on
  records, `version` on cells), together with the outbox append.
- This removes the in-process mutexes that close the window today.

**How today's kinds land:**
- `actor_wake_outbox` becomes a queue, with an index of open rows only.
- `inbox_delivery`, `coagent_mailbox` and `worker_update` become queues.
- `commitment_record` stays a record (an act). Its delivery becomes a
  queue item that points at it.
- `inbox_head`, `conversation_head`, the sequence kinds and
  `story_library_head` become cells.
- `stream_seq_lease` goes away with the telemetry sealer, as the census
  item 2 shows.
- `choir.event`, `lifecycle_event`, `lifecycle_command` and
  `inbox_command` become logs. Each is classed telemetry or agent record
  per owner question 1.
- `run_memory_entry` becomes an agent-record log that leaves the store at
  run close, and stays on the tape.
- `desktop_session` and `browser_session` become device state.
- The roughly 20 typed tables beside the graph (`texture_revisions`,
  `choir_events`, `agent_evidence`, `co_super_slots`, the media and
  desktop tables) fold into their kinds. I27's second-homes list (H038) is
  then empty.

### 5.3 The witness is logical and incremental

- The witness is a multiset hash over each class's logical rows: for
  records, `(id, kind, version, content_hash)`; for cells, `(kind, key,
  version, value_hash)`; for queue items and logs, their keys and hashes.
- It is kept per kind in the write's own transaction, so reading it costs
  nothing at any size.
- The construction is LtHash (lattice-based, homomorphic, built for exactly
  this). A plain sum of SHA-256 values is cheaper but forgeable by a
  generalized-birthday search, and PSA needs the witness to hold against a
  motivated writer.
- **What it excludes:** DDL, indexes, FTS tables and device state. Adding
  an app or an index changes no existing witness, and the witness no longer
  pins the schema.
- **What it replaces:**
  - the O(rows) v3 scan under the append hold on every platform update
    (census item 6);
  - the schema-text coupling that makes any DDL change a restore hazard.
- A snapshot is still verified by recomputing the witness from its rows,
  once, off the serving path.

### 5.4 The tape carries kind operations, batched and signed

The frame §4 write path is adopted as written:
1. One SQLite transaction applies the change and appends to the outbox.
2. A flusher ships batches.
3. The host checks the epoch and the chain, appends to a segment, and signs
   one receipt per batch.
4. The barrier holds effects until their cause is receipted.

The graph adds three things:

- **Ops are per class, not per table.** An op is one of: `record.put`,
  `record.delete`, `edge.put`, `log.append`, `queue.enqueue`,
  `queue.consume`, `cell.set`. Today's `ProjectionOp.Table` row snapshots
  mirror Dolt tables. A replayer that knows the classes needs no knowledge
  of kinds, which is I27's substrate neutrality made literal.
- **Batches are signed by the computer key.** The frame scheduled this for
  phase 2. The bridge needs it now, because the Autopaper graph verifies
  publications against the publisher's own signed tape.
- **One epoch fences the computer, and the capability carries it.** A
  capability never renews across an epoch, and the 15-minute renewal grace
  goes away.

What this removes:
- the per-event receipt rows: 16 GB becomes one row per batch plus the
  segment index;
- the pin receipts;
- the prepared-row dance;
- `RecoverPrepared`;
- the dry-run poison guard (the batch is already committed locally);
- the appender mutex across the network;
- the 64 global CAS buckets.

### 5.5 The Autopaper graph: immutable signed versions

PostgreSQL `graph`, redesigned:

```text
packages  (package, version, manifest JSONB, manifest_hash, publisher, signature, seq)
versions  (version_hash PK, object_id, kind, schema_v, publisher, owner, scope,
           seq BIGINT UNIQUE,                 -- the feed position
           tape_ref (computer, event_seq, batch_receipt_digest),
           body JSONB | body_ref, search tsvector, signature, published_at)
heads     (object_id PK, version_hash, publisher, seq)   -- latest version, moved only by its publisher
edges     (from_version, kind, to_object, publisher, seq)
routes    (path PK, object_id, owner, seq)               -- host-owned kind
takedowns (object_id, act, seq)                          -- host-owned kind
blobs     (digest PK, size, first_seq)                   -- one CAS, rows in the same transaction
```

**Versions are immutable.**
- A version is inserted once, keyed by its hash, and never updated.
- `heads` moves only by a signed act of the same publisher.
- Upserts in place go away, and so does "immutability by convention".

**Every version cites its act.**
- `tape_ref` names the publish act on the publisher's tape and the batch
  receipt that covers it.
- The host already holds that tape, so it verifies the citation before it
  inserts the version.
- A publication is then provably "published at event N of computer C". A
  precommitment claim can cite that.

**The host learns kinds from packages.**
- A version of kind K is admitted only if a published package declares K
  with a publish policy that allows it, and the version validates against
  that schema.
- corpusd needs no release for a new app's kinds.

**Host-owned kinds are written only by host verbs.**
- Routes, takedowns and listings are written only by the host.
- A route binds a path in the owner's namespace to an object that owner
  published. It is claimed first come, per owner.

**Reviews and attestations become real.**
- They are separate kinds, written by their real writers (a reviewer desk,
  the owner, a verifier).
- They are never stamped by the subject.

**Derived data is native.**
- Declared indexes become PostgreSQL expression indexes on `body` (JSONB).
  `search` fields fill `tsvector` with GIN.
- This deletes `og_lookup_keys`, `proj_capture_index`,
  `proj_capture_terms`, the backfill, the 10-minute reconciler and the
  window ladder.

**Readers follow a feed.**
- `seq` is a total order of everything published.
- A publish does `NOTIFY graph_feed, seq`. Subscribers hold a cursor and
  read `WHERE seq > cursor`.
- The front, search and the published-tab views are caches keyed by
  `(object_id, head seq)`, refreshed by the feed.
- A front page becomes one query against a front table that the feed
  maintains, instead of about 500.

**Captures are publications too.** The paper's captures become ordinary
signed versions, published in batches: one envelope, one signature, many
versions. Capture bodies go to the one blob store, under its GC.

**Scope is a column.** It is `public` for the Autopaper, `org:<id>` for
the third I27 scope, and `share:<grant>` for direct sharing. Read checks
filter by scope.

### 5.6 The bridge: one path

| Need | Path | Proof |
|---|---|---|
| Ship state | `POST /tape/batches` | computer signature, epoch, chain continuity; host batch receipt |
| Publish | publish act on the tape, then `POST /graph/versions` (a signed envelope, possibly many versions) | the host verifies the signature, the package policy, the schema, and that the `tape_ref` is in its receipted chain |
| Follow | `GET /graph/feed?after=seq&kinds=…&publishers=…`, woken by a long-poll or server-sent events that NOTIFY drives | `seq` order; signatures carried through |
| Adopt | an adoption act on the adopter's tape naming `version_hash` and the publisher signature; the body is copied into the adopter's CAS | the adopter verifies the hash and the signature before the act |
| Blobs | `PUT /blobs/{digest}` (pending until a receipted event or a version references it, frame rule 4) | digest |

**What goes away:**
- the generic object PUT;
- the bespoke typed publish routes;
- the proxy pulling documents back from the guest;
- source-IP authentication for writes;
- `trustedInternalCaller` forwarding;
- `bindTapCaller` for graph writes.

**Identity.**
- One identity: the computer key, minted at genesis and registered in
  control (the key history table exists).
- One transport capability, bound to the epoch.

**Adopting across packages.**
- If an adopted version's kind belongs to a package the adopter does not
  have, the package is offered first: a package is itself a published
  object.
- Adoption is then two acts, install and adopt, both owner-accepted by
  default.

**Trust on the reader's side.**
- An adopter takes versions only from publishers in its trust set (the
  paper for captures, by default).
- Agents reading the shared graph see the publisher and signature of
  everything they read. That replaces open, unattributed reads of shared
  captures.

### 5.7 Walkthrough: the engineering desk ships an app

The owner asks for a reading list.

1. **Build.** The engineering desk builds `reading-list` in a capsule: its
   UI, its desk prompt, and the manifest in §5.1.
2. **Test.** In the capsule it installs the package into a throwaway copy
   of the store and runs the app's tests against the declared indexes.
3. **Install.** The owner accepts. The `package_installed` act is
   receipted. The substrate creates two expression indexes and one FTS
   table. No existing witness changes and no platform code changes.
4. **Use.** `desk:reading-list` writes items. The schema is validated on
   every write, and a write by any other writer is refused.
5. **Publish.** The owner publishes the list. The substrate:
   - appends a publish act;
   - publishes the package, if the host lacks it;
   - publishes the versions.

   The host verifies them and inserts them, and the feed wakes
   subscribers.
6. **Adopt.** Another computer adopts the list: install act, then adopt act
   by hash.

Today steps 3 to 6 each need a platform change.

## 6. What goes away

These are line counts of non-test code from the census and the maps.
They are rough estimates.

| Item | Est. LOC |
|---|---|
| Telemetry's four homes (`event_table`, `telemetry_sealer`, `telemetry_stats`, `tracelog`, classification) | ~6,000 |
| `stream_seq` lease, seed and newest-headers | ~1,200 |
| Projection-job, base and pin machinery with no worker | ~3,000 |
| `og_key_index` and its validator | ~1,100 |
| Host `og_lookup_keys`, capture projections, `captureterms`, reconcile, window ladder | ~1,700 |
| Per-event receipts, pins, prepared recovery, dry-run guard (after 5.4) | ~1,500 (inferred) |
| Bespoke publish routes and the generic PUT (after 5.5 and 5.6) | ~2,000 (inferred) |
| Vocab migration and fence (if the vocab migration is complete everywhere) | ~2,300 |
| Two diverging store implementations become one interface plus two thin dialects | ~800 (inferred) |

Total: about 15 to 19 K lines of non-test code, plus a similar amount of
tests.

## 7. The sequence

Each phase ships on d@e first, then widens by ring (progressive delivery).
Each phase's gate is runtime, not review.

**Phase 0: fixes that need no redesign (this week, each one small)**

- Remove the guest's reach to the generic object and edge writes. This
  landed on 2026-10-30 with
  `docs/problems/guest-can-write-public-routes-2026-10-30.md`: zero guest
  writes are allowed, at corpusd and at the proxy.
- Stop corpusd stamping itself as the writer. This waits for phase 3,
  where the writer comes from the signed envelope.
- Escrow unwrap: one transaction for status plus the audit append, and the
  key unsealed after the commit.
- Lifecycle stream: read by trajectory, not the whole graph.
- **The snapshot producer and the recovery-path fix, landed together.**
  - All three reviewers found that nothing in production takes a snapshot.
  - Devin found that the missing-disk check at
    `autoputer/projection_base.go:151` is always true, because
    `store.Open` makes the `.texture` sidecar. So a lost disk falls to an
    unbounded replay from genesis.
  - Fixing the check alone would turn that slow replay into a refusal for
    any chain over 10,000 events, because no snapshot exists. They ship
    together.
  - The paper's image cannot hold a second 30 GB copy until C7, so a
    snapshot must stream into chunks rather than `VACUUM INTO` a temporary
    file.
  - C7's deletion of the old data waits for one snapshot per computer.
- The paper's boot validator: persist the validated state, so a boot does
  not repeat the full pass. Replacing `og_key_index` with generated columns
  waits for phase 1. Witness v3 hashes every `sqlite_master` entry,
  indexes included, so any DDL change makes restores to earlier
  checkpoints fail their witness match. The logical witness (§5.3) has to
  come first.
- Host: JSONB plus expression and GIN indexes, then drop the side tables.
  Cache the front and resolve by head.
- Telemetry: stop writing `choir.event` to the graph. After owner question
  1 is answered, retire the paper's telemetry rows (a measured share of the
  4.86 GB).

**Phase 1: kinds as data, storage by class (guest)**

- The manifest format, `package_installed`, and the registry as a
  projection.
- The `choir` package expressed in it.
- The class tables, the migration of today's kinds by class, the logical
  witness, and the enforced writer and schema.
- Gate: a differential on d@e (each kind's logical set equal before and
  after), and a test app shipped from a capsule with zero platform change.

**Phase 2: the tape protocol (frame I0 plus computer signing)**

- Batched, signed segments; epoch fencing; the barrier for the gate-bearing
  kinds.
- Receipts become one row per batch.
- Gate: kill -9 under load with no acknowledged write lost, a stale epoch
  refused, and the receipt bytes per event measured.

**Phase 3: the Autopaper graph and the bridge**

- `versions`, `heads`, the feed, packages on the host, signed publish with
  `tape_ref`, and adoption acts.
- Captures move to publications.
- Gate: the paper publishes only through the new path for 24 hours with
  the front equal, and an adoption round trip between two computers.

**Phase 4: retention and snapshots (frame I1, I3, I4)**

- Class expiry, snapshot cadence, and one CAS with deletion by ownership.
- This partly runs alongside phases 1 to 3.

Phases 1 and 2 are independent and can run in parallel builders, split by
package boundary. Phase 3 needs both: package declarations and computer
signatures.

## 8. Predictions (for the precommitment ledger)

1. The paper's `state.db` after phase 0 telemetry retirement plus phase 1
   class retention: 6 GB, interval [3, 12].
2. Host control receipts at 7 days after phase 2: 0.5 GB [0.1, 2], from
   16 GB.
3. Guest record put latency, p50, outside a barrier, after phase 2: 3 ms
   [1, 15]. The baseline is owed; it is measured on d@e before phase 2.
4. A capsule-built app with two kinds and one index installs on d@e with
   zero platform code change and no deploy, by the end of phase 1: yes,
   0.8.
5. Front page database queries after phase 3: at most 5 per page, from
   about 500.
6. The paper's boot to healthy stays under 30 s, with no boot-time
   full-store pass after phase 0: 0.9.
7. Net non-test lines deleted across phases 0 to 4: 15 K [8 K, 25 K].

## 9. Owner questions (each has a default; none blocks)

1. **Is `choir.event` agent record or telemetry?** It is 4.86 GB on the
   paper.
   - Default: split by event kind. The kinds that run acceptance and the
     Texture verifier read are agent record, kept and on the tape. Progress
     and trace kinds are telemetry: local, 14 days.
2. **Who may accept a package install?**
   - Default: the owner, per install.
   - Later: an owner-granted budget for the engineering desk, recorded as
     a precommitment.
3. **Does an adoption follow the publisher or pin a version?**
   - Default: pin a version. Following is its own subscription act.
4. **Organization scope.**
   - Default: the column exists from phase 3; read access control comes
     with the first organization.

## 10. Alternatives considered

- **Stand pat.** It runs today. The cost is the measured growth (nothing
  retires), the two authority gaps, and a self-development path that stops
  at the data layer. Phase 0 alone is worth doing even if nothing else is.
- **A typed table per kind.** It gives better typing and constraints. It
  costs DDL generated from guest-built packages, an `ALTER` ladder per
  package version, a witness tied to DDL, and a substrate that knows its
  kinds. At our sizes (millions of rows per computer), expression indexes
  on JSONB perform like column indexes. Declared views give desks a typed
  SQL surface without making it authoritative.
- **PostgreSQL in every guest.** It brings an operational process per
  computer, memory per VM, and a harder snapshot. SQLite's one-file
  property is what makes snapshots, restores and the witness simple.
- **Keep the generic table and only add retention.** That fixes size, but
  not authority, kinds as code, the synchronous tape, or the bridge. It is
  phase 0 plus phase 4 without phases 1 to 3.

## 11. Amendment (2026-10-30 evening): the panel, the owner, and what ran

This section supersedes the earlier sections where they conflict. Sources:
the phase-1 panel (Devin, DeepSeek v4.1 flash, Claude Sonnet; scratchpad
`panel-phase1/run1`, prompt on snapshots and phase 1), the owner's direction
of the afternoon (ledger, 2026-10-30 ~16:40 and after), and the runtime record
in `docs/definitions/choir-data-system-build-2026-10-30.ledger.md`.

### 11.1 Phase 0, as it stands

- **Snapshots are streamed by the guest** (choice (b), all three seats). One
  read transaction on the read pool reads the head first, streams the
  database pages through `sqlite_dbpage` in 4 MiB page ranges, computes the
  witness in the same transaction, seals each range under the computer key,
  and skips ranges the host already holds (a HEAD probe per chunk).
  `filecas.SealChunk` uses a synthetic nonce, so an unchanged page range
  seals to the same digest and is not sent again. There is no temporary copy
  and no size cap. The format is the version 1 record plus page geometry.
- **The recovery-path fix landed with it**: the missing-disk test reads
  whether the legacy workspace holds a Dolt database, not whether the
  directory exists.
- **The boot validator is not persisted; it reads on the read pool.** The
  pass compares inside a read transaction and repairs only differing ids on
  the write connection. It went from 47 to 81 rows a second on the paper.
  The rest of the cost is row layout (§11.4).
- **Snapshot cadence is a correctness item, not phase 4** (Devin). The boot
  recovery bound is 10,000 events (`recoveryplan.MaxRecoveryTailEvents`, a
  constant). The paper writes about 660,000 events a day. With a 24-hour
  cadence, a snapshot covers a lost disk only for the first 15 to 20 minutes
  after it is taken; after that the boot refuses the tail and only a code
  change admits it. Snapshotting every 15 minutes would read 31 GB four times
  an hour. The fix is the bound, not the cadence alone:
  1. measure the in-guest replay rate (the forecast R0k-07 is 1,200 events a
     second for a user computer, 600 for the paper; the first measurement,
     d@e at 18:07, is 206 events a second, so a day of the paper's tail is
     about 53 minutes, and why replay is this slow is its own question);
  2. state the bound as replay time (for example 15 minutes at the measured
     rate), not as an event count;
  3. make the cadence fire on the tail as well as on time, so the tail stays
     inside that bound;
  4. until then, the bound needs an operator override for a recovery, so a
     lost disk is not a code change at the moment of loss.

  Done the same evening (ec2887f84, b957ef538): the payload prefetch took
  d@e from 206 to 298 events a second. The rest is apply, about 2.7 ms an
  event. The per-step split (b6a093721) put 10.1 of 11.3 s of apply in
  prepare and finalize, two commits an event, and a replay now skips the
  WAL sync (synchronous NORMAL, then FULL and a checkpoint on every exit).
  That was only part of it: at NORMAL d@e replays at 350 events a second,
  prepare 1.5 s and finalize 6.4 s, so the sync was about 1.7 s and
  finalize's own work, about 1.5 ms an event, is the rest. A profile of
  finalize is the next step. The bound is 720,000
  events, about 40 minutes at 298 a second, and the producer snapshots
  when the tail reaches three quarters of it. Reading vmctl for this found
  a second refusal: its empty-store admission planned against the replay
  watermark rows of the projection-base era, which nothing writes since
  the cutover; the watermark is now the newest snapshot root. The
  lost-disk drill on d@e then recovered the event state but none of the
  files saved in the hour before: the guest synced files on a 15-minute
  ticker that an idle computer, hibernated ten minutes after boot, never
  reached. The guest now syncs at a two-minute scan that finds the tree
  changed (d900a7a46).
- **The first-snapshot lookup is a full scan.** `NewestEventOfKind` walks
  `computer_event_index` by `(computer_id, status, sequence)` and tests the
  kind row by row. On a computer with no snapshot that is every event: about
  12 minutes on the paper's 5.2 million. It ends once one snapshot exists.
  The index it needs is DDL, so it waits for the logical witness; until then
  the producer can remember its own last snapshot in a guest file and fall
  back to a bounded window of the index.
- **Install-time witness check on the boot path** (Devin). The staged
  restore checks the unsealed file against the recorded witness; the boot
  install does not. The chunks are already authenticated (AEAD under the
  computer key, digest-checked, manifest root verified), so the host cannot
  forge one. The check would catch a producer that recorded a witness its
  pages disagree with. It costs one full witness pass at boot, after a lost
  disk only. Wire it in, unconditionally.
- **The file surface fsyncs before it acknowledges** (C0 drill: 313 saves
  read back empty after a SIGKILL; fixed 72640d15c).
- **The checkpoint cadence stays narrow until a checkpoint names a past
  head.** The checkpoint publish is an event appended with a CAS on the
  head it witnessed, so the witness must describe the current head. On a
  busy computer appends land during the read-pool scan, the head moves, and
  the next attempt pins appends for the whole scan and publish (d@e under
  drill load: held 1.131 s on 63 MB; the paper's witness alone takes 4 min
  12 s). The snapshot record already does it the other way: it names the
  head it witnessed and appends at whatever head is current, which is why
  the paper's 31 GB snapshot ran under full load with no hold. Fix, the
  owner's "a checkpoint is a snapshot whose fingerprint is published": a
  checkpoint references a snapshot record (head, witness, manifest root) on
  the tape, so binding one reads no store at all.

### 11.2 Phase 1, split and reordered (all three seats)

Phase 1 becomes two phases.

**Phase 1a (no storage change):**

1. The logical witness (v4) on today's tables. Witness v3 hashes every
   `sqlite_master` entry, so every DDL change, an index included, breaks the
   witness match against every earlier checkpoint. Nothing that adds an
   index or a table lands before v4.
2. The manifest and the install act, on the existing `choir.app-package/v1`
   seam (`internal/apppackage`, JSON Schema already in the tree). Do not
   invent a schema language.
3. The enforced writer and schema on every write. Before the gate flips to
   enforce, run the observe gate's violation log as a census on d@e and the
   paper (Devin's risk 4: an unregistered write path refused under load is a
   self-inflicted outage).

**Phase 1b (storage by class):** the class tables and the migration, after
1a has run on the fleet.

### 11.3 Additions the panel found

- **Immutable records.** Precommitment records and other acts can be
  overwritten at the store level today; the overwrite shows on the tape, but
  mint-once is a caller convention. The record class carries an
  `immutable: true` flag; the write path refuses update and delete for it.
  This is a flag on the record class, not a seventh class.
- **The witness choice: LtHash in the transaction** (Devin, Sonnet), not
  recompute at snapshot with per-class Merkle roots (DeepSeek). The
  decisive property is that the witness is a pure function of the tape's
  logical operations, so **the host recomputes the expected witness from the
  receipted batches** and compares it to the guest's at each checkpoint.
  That turns the witness from guest-attested into host-verified. Spike the
  parameters first (LtHash variants against MuHash3072, which deletes more
  simply). Sonnet's per-stream hash chain for logs is not adopted: the tape
  already orders them.
- **Retirement is a tape operation.** A class expiry or a telemetry trim is
  an event on the tape, so replay and the witness agree on what was dropped.
- **Mandatory quotas per package**: rows, bytes, index count, index path
  depth and full-text fields, declared in the manifest and enforced at write.
  Generated index names stop SQL injection, but an accepted manifest can
  still declare a quadratic index set.
- **The hash domain (Devin, the top migration risk).**
  `ContentHash(kind, body, metadata)` covers metadata, and content-hash
  identity, the default, derives the `canonical_id` from it. Folding
  metadata into the body would give every migrated object a new hash and a
  new id, and every edge, reference and publication hash would dangle. The
  migration keeps each existing row's `canonical_id` and `content_hash`
  verbatim, marks the domain with a `hash_v` column, and applies the new
  domain only to writes after the migration.
- **Edges keep `metadata`** unless a census proves it empty for every kind.
  The §5.2 edge shape drops it; today it is NOT NULL and read and written.
- **Device state stays on the tape per kind.** §5.2 put device state off the
  tape. Media progress and user preferences are tape-projected today, and a
  restore keeps them. Only rebuildable caches and per-device UI choices
  leave the tape (I27), and each kind is named.
- **Unspecified until written:** replay-positional validation (a row is
  checked against the schema version installed at its position, not the
  newest), uninstall (refuse while the kind holds rows, or tombstone
  operations on the tape), and a chunked, resumable migration act for a
  breaking schema change (millions of rows on the paper).
- **Computer signing moves to late phase 1**: the witness's security value
  needs signed batches. Phase 2 keeps the protocol.

### 11.4 Layout rule

Put small, often-read columns before large ones. In `og_objects`,
`updated_at` follows `body` and `metadata`, so reading it reads the whole
body and its overflow pages; the read-pool validator spends most of its time
there. The class tables put keys, kind, timestamps and the `hash_v` marker
first, and the body last.

### 11.5 The owner's direction (2026-10-30 afternoon)

- **`choir.event`**: the default stands (owner question 1). Trace and
  progress kinds leave the working store and stay on the sealed tape; the
  trim is a tape event (§11.3), so replay agrees.
- **What the tape is for**: recovery from bad self-development (undo to a
  checkpoint, then rebuild), the audit log (enterprise), the long-term value
  of ordered data (precommitment records, PSA, analysis), and the users'
  agent traces. Those traces are the users' data, not Choir's. They stay
  sealed under the computer's key and preserved.
- **A checkpoint is a snapshot whose fingerprint is published.** Undo is a
  rebuild aimed at an earlier checkpoint: install the nearest snapshot at or
  before it and replay the tape to it.
- **The tape is tamper-evident.** Batches are signed by the computer (late
  phase 1, §11.3), and the heads are anchored outside the host (a named
  phase-2 item; PSA's backdating bound depends on it). Escrow of the
  computer key is the owner's choice.
- **The tape protocol is a replication protocol.** A computer will run on
  the host, on the owner's Mac (a local app; Windows and Linux later), and
  on an enterprise's own host syncing to employees' workstations. Phase 2
  designs the protocol as one writer per epoch with an explicit handoff
  between hosts, not as a host-only append path. Snapshots stay
  hypervisor-neutral (a SQLite file and a sealed manifest, never a disk
  image), because the hypervisor bake-off comes next.
- **Keys travel with the computer**: on a Mac, in the Keychain. A host that
  serves a computer holds its key only while it serves it.
- **Backups**: there is one machine, so nothing is off the host. Sealed
  data (tape payloads, snapshot chunks, the PostgreSQL dumps) copied to a
  second location is the next durability step. The paper runs without a
  backup tonight, by the owner's decision.

### 11.6 `choir.event`, measured before building the split (2026-10-30 evening)

The paper's `choir.event` rows (a private reflink copy at 18:27, sizes and
counts only) total about 4.93 GB. By kind, all time and the last 24 hours:

| kind | all time | last 24 h | reader |
| --- | --- | --- | --- |
| `tool.result` | 1.84 GB | 96 MB | run acceptance, the Texture verifier |
| `loop.progress` | 1.43 GB | 41 MB | `/api/costs` reads the `tool_loop` and `jev_judgment` lines (37 MB of the 24 h) |
| `desk.cell` | 588 MB | 32 MB | the trace tooling (`choir trace`, `/api/runs/{id}/events`) |
| `tool.invoked` | 307 MB | 0 | none: live-only since 2026-10-22 |
| `desk.egress` | 271 MB | 1.9 MB | the trace tooling |
| `texture.agent_revision.progress` | 151 MB | 0 | none: live-only since 2026-10-22 |
| run lifecycle, Texture revisions, judgments, channel | about 0.35 GB | 25 MB | product reads |

The write-side split the owner's default asks for is mostly in place
already: the journal cuts of 2026-10-22 (`types.LiveOnly`) stopped
appending the model-call line, `tool.invoked` and the Texture progress copy.
What is written today, about 198 MB a day, is record (the tool results and
the cost lines) or the trace the repo contract says to read before
attributing behavior (`desk.cell`, `desk.egress`). A new write filter would
save at most about 37 MB a day, and it would take the trace from the only
tool that reads it. It is not built.

The weight is the backlog: about 2.0 GB of rows no reader takes (the
phaseless `loop.progress` lines, `tool.invoked`, the Texture progress copy,
from before the cuts) plus the trace kinds as they age. Removing them is the
retirement op (§11.3), a tape event so replay agrees, with a window: the
trace kinds kept 14 days (the owner's default), `tool.result` kept while a
run's acceptance can still read it. Growth without retention is about
200 MB a day on the paper, about 70 GB a year, so the retirement op is
phase 1's first storage item.
