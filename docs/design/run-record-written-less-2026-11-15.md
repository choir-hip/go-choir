# The source-entities list leaves the run record (T2, first slice)

Date: 2026-11-15, rewritten after the counter's first reading
(`6387e5dff`, largest computer, about 15 minutes after boot). Class of this
document: green (design only). The change it designs is red: it touches
`choir.run`, a kernel kind that run acceptance and the evidence surfaces read.

Station: T2 of `docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`.
Background: `docs/design/store-once-2026-11-14.md` (R2).

Reading rule: every claim about code carries the file and line I read in this
worktree. What I did not read or could not run is in section 9. Anything not
in the code or the reading is marked **invented**. Nothing here was run.

An earlier draft of this file chose "drop one write" (the `updated_at`-only
write). The reading moved the weight to one metadata key, so that mechanism is
withdrawn and is not part of this one. One mechanism only.

## 1. The reading, and the one mechanism

Reading (counter `run_writes`, 179 `choir.run` writes, 1.606 MB of body bytes):

| Fact | Value |
| --- | --- |
| `metadata` bytes | 1.454 MB, 90% of the body |
| `texture_available_source_entities` as the largest metadata key | 93 writes, 1,200,748 bytes, 74.8% of all run-record body bytes, mean 12.9 KB a write |
| writes in which that key CHANGED (`metadata_key_changes`) | 19 |
| `activation/pending/new` | 25 ops, 299 KB, about 12.0 KB a write |
| `activation/running/updated_at_only` | 23 ops, 296 KB, about 12.9 KB a write |
| `activation/running/metadata+state` | 19 ops, 286 KB |
| `activation/completed/finished_at+metadata+state` | 20 ops, 215 KB |
| `labels/running/labels` | 41 ops, 339 KB, about 8.3 KB a write |
| `create_og`, `update_og` lines | about 2.3 KB a write (runs without the list) |

**Mechanism: the list is written once, as its own immutable object of a new
kind, and the run record names it with a short reference instead of carrying
it.** Section 3 shows why the list cannot be derived from objects that
already exist. Sections 4 to 6 give the form, the failures and the proof.

## 2. The list in code: writers, readers, lifetime

### 2.1 Where it is written

The key is `textureAvailableSourceEntitiesKey` (`internal/textureowner/texture_task_types.go:23`).
Every write of it in non-test code:

| Site | What it does |
| --- | --- |
| `internal/textureowner/texture_agent_revision.go:666-679` | builds the list at the start of a Texture revision run: the current revision's sources (`decodeTextureSourceEntities(currentRevision.SourceEntities)`, `:661`) plus media sources found in the revision's content (`registerTextureMediaSourceEntities`, `:662`) plus `evidenceSourceEntitiesFromCommitmentRecords(..., 12)` (`:667`), merged in order, then `metadata[key] = sourceEntities` (`:675`). `metadata` here is a local map decoded from the current revision's metadata (`:598`). |
| `texture_agent_revision.go:680-688` | merges `req.SourceEntities`, a field of the request that is not stored elsewhere (`:34`, tagged `json:"-"`), into the same local map (`:683`). |
| `texture_agent_revision.go:789-797` | copies every key of `durableMetadataKeys` from that local map into `runMetadata`, and `textureAvailableSourceEntitiesKey` is one of those keys (`texture_task_types.go:66`). That copy is how the list reaches the run record. |
| `texture_agent_revision.go:823` | `rt.Core.StartRunWithMetadata(ctx, agentPrompt, ownerID, runMetadata)` creates the run with the list inside its metadata. |
| `texture_evidence_sources.go:457-476` | `mergeTextureSourceEntitiesIntoAvailableContext` and `mergeTextureSourceEntitiesIntoRunMetadata`: no caller outside a test (grep over `internal`; the only call is `texture_evidence_sources_test.go:364`). |

So the only writer of the list into a run record is the run's own start. I
found no code that changes it after the run exists.

**The 19 changes.** The counter compares the raw bytes of the key before and
after a write. At the start the value is a typed `[]textureSourceEntity`
marshalled in struct field order; any later read-modify-write of the run
(`updateRunOGOnce` decodes the body into `map[string]any` and marshals it
again, `internal/store/graph_store.go:748`, `:783`) writes the same
content with keys in sorted order. Different bytes, same list. The number 19
equals the `activation/running/metadata+state` count in the same reading (19
ops), which is where the first re-encode would fall. **This is a hypothesis;
I could not read the two bodies.** Section 5.2 gives the test that settles
it. If it is false and the list truly changes during a run, the form in
section 4 still holds with one more object write per real change (open
question 1).

### 2.2 Where it is read

| Reader | Reads | Evidence |
| --- | --- | --- |
| R1 `coagentTextureSourceEntities` | the parent run's list, then merges content items named by the coagent request | `internal/textureowner/coagent_route.go:416-418` |
| R2 `buildAppagentRevisionMetadata` | the run's list, only to compute `selfDevelopmentJoinFromSourceEntities` (the operation, bundle, receipt and event-head ids of entities of kind `operation`, `capsule_bundle`, `receipt`, `event_head`) and merge the join into the revision's metadata; then deletes the key from the persisted revision | `internal/textureowner/texture_revision_metadata.go:75-81`; `texture_evidence_sources.go:797-816` |
| R3, R4 | the local revision-metadata map, not the run record (`texture_agent_revision.go:681`; `texture_media_sources.go:196`) | |
| the model's input | **not read.** `buildAgentRevisionRequest` reads `seed_prompt`, `owner_prompt`, `source_path`, ... and never this key (`texture_agent_revision.go:940-1030`); `runContextForRun` reads other keys (`internal/agentcore/run_context.go:54-102`) | |
| observability, run acceptance, replay | no reader of the key in Go (`grep` of the repo: the constant appears only in `internal/textureowner/texture_task_types.go`, tests, the counter's name list and docs). The run API serves the run's `Metadata` as is (`internal/agentcore/api.go:603`); I found no consumer in the repo's non-Go files. | |

Two readers in Go read the key from a run record: R1 and R2. Both can run
long after the start (R2 at the run's completion, R1 whenever a coagent of
that run routes).

### 2.3 Lifetime

Per run. A later run of the same document does not start from it: the
persisted revision metadata drops the key (`texture_revision_metadata.go:76-81`,
with the comment "recomputed from the actual revision's source_entities"), and
the next run rebuilds the list at its own start (`:661-688`). The key's other
appearances in `durableMetadataKeys` loops (`texture_revision_metadata.go:26`,
`:38`; `texture_structure.go:55`) copy it between maps that the delete at `:81`
then strips.

## 3. Can the list be derived from objects that already exist?

I looked for a per-source object whose set is this list, so that the list
need not be stored at all.

- `choir.source_entity` exists (`internal/store/texture_source_graph.go:15`),
  content-hash identity (`internal/objectgraph/registry.go:76`), keyed by
  owner scope, source kind and target (`:60-69`). It is written when a
  revision commits its sources (`internal/textureowner/tools_texture.go:122-148`).
  It holds the sources a committed revision used. The list also holds sources
  no revision has used yet (evidence from commitment records, media found in
  content), and the kind is listed per owner for display
  (`ListTextureSourceEntitiesByScope`, `texture_source_graph.go:105`); putting
  uncited candidates into it changes what that listing means.
- The commitment records are existing objects and the list's evidence part is
  a projection of them, "rebuilt each turn" (`texture_evidence_sources.go:746-748`).
  But it reads only the latest 12 (`:734`, `texture_agent_revision.go:667`),
  so the set at a later read is not the set at the start; and building an
  entity can call the content service (`sourceEntityFromCoagentPacketSource(..., rt, ...)`,
  `:761`), so recomputing is not a pure read.
- `req.SourceEntities` is a request input stored nowhere.

R1 and R2 read the list after the start. A recomputation at read time would
give a different list, and the requirement is that their outputs are byte-equal
to today's. **The list is not derivable.** It needs a stored home, and the
smallest one is a single object written once.

## 4. The form

**A new kind, `choir.run_source_entities` (name invented), content-hash
identity, one object per distinct list, written once by the object op that
exists, and named from the run record by one short metadata key
(`texture_source_entities_ref`, invented).**

- **The object.** Body: the list exactly as `mergeTextureSourceEntities`
  returns it, marshalled from the typed `[]types.SourceEntity` (struct order,
  `internal/types/texture_provenance.go:132`). Identity: the content hash, the
  shape of `choir.run_memory_entry` (`registry.go:92`), so it is immutable: a
  changed list is another object, never a rewrite. Registered next to
  `choir.run_memory_entry` (`registry.go:92`). A registered kind is required:
  an unregistered write is refused (`internal/objectgraph/registry.go:383-388`,
  `internal/objectgraph/sql_store.go:422-426`). The kind joins the lists that
  name `choir.run_memory_entry` today: `internal/kernelservice/kernelservice.go:69`,
  `internal/store/snapshot_to.go:400`, `internal/store/vocab_migrate_og.go:130`,
  `internal/vocabmigrate/families.go:182` (whether the last two need it I did
  not trace; open question 4).
- **The write.** In `texture_agent_revision.go` at `:675`/`:683`, where the
  list is final, the handler writes the object with the declared kernel writer
  (as `UnionRunLabels` does, `internal/store/store.go:2451`; the writer gate
  admits a kernel kind, `internal/objectgraph/writer.go:326-327`) **before**
  `StartRunWithMetadata`. The key `textureAvailableSourceEntitiesKey` leaves
  `durableMetadataKeys` (`texture_task_types.go:66`) so the loop at `:789`
  stops copying the list, and `runMetadata["texture_source_entities_ref"]` is
  set instead to the object's canonical id. The run record gains about 140
  bytes (key plus canonical id; estimate, invented) and loses the list.
- **The reads.** One function loads a run's list: inline key present (a run
  from before this change) returns it as today; else the ref present returns
  the object's list; else none. R1 (`coagent_route.go:418`) and R2
  (`texture_revision_metadata.go:75`) call it. R2's function receives a
  context it ignores today (`buildAppagentRevisionMetadata(_ context.Context, ...)`,
  `:16`) and the handler holds the store. A ref whose object is missing is an
  error the caller logs, not an empty list (failure 5).
- **No patch op, no new op kind, no store floor.** The object travels as an
  ordinary `object_recorded` op (`internal/store/projection_tape.go:105-120`).

### 4.1 What an old binary does

- **A tape with the new kind.** Replay hands each object op to `projectObject`,
  which decodes the object and writes its header and body with no check of the
  kind (`internal/store/project.go:241-275`). The object lands as an ordinary
  row. The write gate and the read gate treat a kind they do not register as
  admitted: `writer.go:365-368` (`err != nil` continues) and `reader.go:255-257`
  (`err != nil` returns nil). No old code names the kind, so nothing reads
  the row. The tape counters key it by the kind parsed from its id
  (`tape_write_counters.go:101-107`); `ParseCanonicalID` validates the kind
  string's shape only (`object.go:88`), not the registry.
- **A run record without the inline key.** `decodeAvailableTextureSourceEntities`
  on a metadata map lacking the key calls `decodeTextureSourceEntities(nil)`,
  which returns nil (`texture_media_sources.go:352-354`), and
  `selfDevelopmentJoinFromSourceEntities(nil)` returns an empty map
  (`texture_evidence_sources.go:797-798`). No panic, and no error: R1 returns
  only the content-item entities, and R2 writes a revision without the join
  keys. That is a silent loss of the join on a run that started on the new
  binary and ends on the old one (failure 3).
- **The ref survives an old binary's rewrite.** The ref is a key inside
  `RunRecord.Metadata`, a `map[string]any`. The old binary decodes and
  re-marshals the whole map (`graph_store.go:748`, `:783`), so an
  unknown key is kept. This is the difference from moving a struct field.

## 5. Expected saving, and the proof

### 5.1 Saving (estimate; the reading is 15 minutes old)

- Run-record bytes: the key is 1,200,748 of 1,606,000 bytes. Left: about
  405 KB, plus the ref on the 93 writes that carried the list (93 x 140 B =
  about 13 KB, invented size) = about 418 KB. **Factor 3.8 fewer run-record
  body bytes.** The mean write falls from 9.0 KB to about 2.3 KB, which is the
  measured size of the lines that carried no list (`create_og`, `update_og`).
- The new object: one per run that starts with a list, at most 25 in this
  reading (the 25 `pending/new` writes; an assumption that each is a run
  that carried the list), each at most the size of its create write, 299 KB / 25
  = 12.0 KB. At most 300 KB, plus an envelope per object (about 0.5 KB each;
  invented). No dedupe is assumed: the entities may carry per-run fields
  that make equal lists differ.
- **Net: about 1,606 KB falls to about 730 KB (418 + 311), a saving of about
  880 KB, 55% of run-record bytes, factor 2.2.** With dedupe the saving rises
  toward 75%. These are body bytes; the tape carries bodies in base64, a
  third larger, in both columns alike.
- Not saved: the live store holds one body per run either way, so the DB size
  is unchanged. The saving is tape bytes.

### 5.2 Proof

Repo tests (written first, class yellow):
1. Byte-equality of what reads the list. For one evidence sequence, run the
   handler's start twice, once through the inline path and once through the
   object path, and require equal: `agentPrompt`, the revision metadata JSON
   built by `buildAppagentRevisionMetadata`, and the entities
   `coagentTextureSourceEntities` returns. Model input does not read the key
   (section 2.2); the test pins that by comparing the prompt bytes anyway.
2. A run record carrying the inline key (old shape) is read by the same
   loader, and a record with a ref to a missing object makes the loader
   return an error that the callers log.
3. Re-encoding: marshal a typed list, decode into `map[string]any`, marshal
   again; assert different bytes and equal decoded content. This settles the
   19-changes hypothesis in the repo; the deployed counter settles it live.
4. A gate test that no Go file outside the loader and the writer names the
   key constants, and that every file listing `choir.run_memory_entry` in a
   kind list also lists the new kind.
5. The whole uncached suites of `internal/textureowner`, `internal/store`,
   `internal/objectgraph`, `internal/kernelservice` and `internal/agentcore`
   (shards: `scripts/go-test-runtime-shards`).

Deployed, on the test computer after the staging deploy:
- `run_writes.top_metadata_key` no longer shows
  `texture_available_source_entities` for new runs, and the mean body bytes of
  `activation/*` lines for new Texture runs falls by a factor of about 3.5 to
  4 (stated: at least 3). The counter's fixed key list gets the ref's name
  (`internal/store/run_write_counters.go:86`; yellow) so the reference shows.
- `object_rewrites["choir.run_source_entities"].ops` is at most the number of
  Texture runs started in the window and `writes_per_object` is 1.
- Whole-tape replay of the test computer's tape into a fresh store: every
  `choir.run` and every `choir.run_source_entities` object has an equal
  content hash and zero are missing.
- An accepted run and a coagent route run before and after, comparing the
  acceptance record and the routed entities.

## 6. Failure list

1. **A reader that still expects the key in the run record.** R1 and R2 are
   the only Go readers (section 2.2); both move to the loader. Detected by
   proof test 4 (the key constant is named in two places) and test 1. The run
   API serves `Metadata` unfiltered (`api.go:603`): a client that read the
   list there would lose it; I found none in the repo (section 9).
2. **A run started on the old binary and finished on the new.** Its record
   carries the list inline; the loader returns it as today. Later writes of
   that run keep the inline list (the new binary rewrites what it read). No
   loss, and no saving for that run.
3. **A run started on the new binary and finished on the old (rollback in
   the run's life).** The old binary reads no inline key and returns nothing
   (section 4.1): R2 omits the join keys and R1 loses the evidence entities,
   silently. Prevented by the rollback rule in section 7 (a rollback restart
   carries no resume marker, so the markerless boot closes pending, running,
   passivated and blocked runs: `internal/agentcore/runtime.go:3066-3083`),
   not by code. Detected after the fact by the missing join keys on a
   revision whose run record names a ref.
4. **Replay of old tape.** Old runs carry the list inline; replay writes
   whole objects as given (`project.go:241-275`), the loader accepts both
   forms, and no tape op is re-encoded. The tape is never rewritten. Detected
   by the whole-tape replay in section 5.2.
5. **A ref whose object is missing** (a restore, a fork, a residue import or
   a snapshot that drops the new kind). The snapshot copy empties any table
   not mapped to a kind (`snapshot_to.go:392-398`: "a new table ships empty
   until it is mapped"); the kind lists in section 4 must carry the new kind.
   Prevented by proof test 4. If it still happens, the loader returns an
   error, not an empty list, so R2 does not write a silent empty join, and
   the caller logs it.
6. **A crash between writing the object and writing the run.** The object is
   written before `StartRunWithMetadata`, so the crash leaves an unnamed
   object: harmless, immutable, found by content hash by the next run with
   the same list. The reverse order (a run naming an object not yet written)
   cannot occur. A crash after the run is written is the ordinary restart
   rule: the markerless boot closes the run (`runtime.go:3066-3083`).
7. **Retirement of the run and of the object together.** I found no retire
   job for `choir.run` records (the daily jobs act on telemetry and run
   memory, `internal/store/telemetry_retire.go`, `run_memory_retire.go`; not
   re-read here). The new kind gets no retire rule: it stays while any run
   names it, and a later rule for run records must retire it with them.
   Named so the T3 retirement work does not orphan or drop one without the
   other.
8. **The growing-list gate of T2.** The gate says a new kind that embeds a
   growing list fails. This object embeds a list, but never grows it: it is
   immutable and content-addressed; a longer list is another object. Written
   once per run start (at most 25 times in the reading; 19 is the number of
   re-encodes, not of list changes), not once per write of the run (93). Whole
   lists are rewritten across runs of one document as it accretes evidence,
   which is the same bytes the create write carries today. To make it
   append-only (one object per entity plus an id list) would add a mechanism
   and N writes per start; the default is not to (open question 3).
9. **Concurrent writers.** The object is write-once by identity: two runs
   starting with equal lists write one object, and the upsert by canonical id
   makes the second a no-op rewrite of the same bytes. The run record's
   writers are unchanged (they carry a ref, not a list).
10. **Model input must be byte-equal.** The model does not read the key
    (section 2.2). R1 and R2 outputs are compared byte for byte in proof
    test 1. The one change in encoding is that the object is marshalled from
    the typed list while today the list in the run record is re-encoded by the
    store on the first rewrite; the loader decodes both to the same typed
    list.
11. **A list that changes during a run.** I found no writer (section 2.1).
    If the deployed counter shows real changes, the writer at that site makes
    a new object and updates the ref; the form stands.
12. **A new client of run metadata that expects the key.** The API serves it
    as is. A client written later against the inline list sees only the ref.
    Named, not prevented.

## 7. Rollback

Revert the commit. New runs written under the change keep their ref; the old
binary does not resolve it, so a run in flight across the rollback loses its
list (failure 3). The rollback procedure is therefore a restart without a
resume marker, which closes open runs by the existing crash rule
(`runtime.go:3066-3083`) instead of letting an old binary continue them. The
objects of the new kind stay as unread rows. Rolling forward again resolves
every ref, because the objects are on the tape. Mutation class: red (the run
record and run-acceptance read paths are protected surfaces); the landing
needs conjecture delta, protected surfaces, evidence class (counter and
replay), rollback (this section) and heresy delta (none: no switch, no limit,
no cache), and two seats.

## 8. The other lines in the reading, stated and not designed

With the list out of the run record, a write costs what the lines that never
carried it cost: about 2.3 KB (the `create_og` and `update_og` lines).

- **`activation/running/updated_at_only`, 23 ops, 296 KB (12.9 KB a write).**
  At 2.3 KB a write, about 53 KB: a saving of about 240 KB. The write still
  exists; it costs about 5.6 times less.
- **`labels/running/labels`, 41 ops, 339 KB (8.3 KB a write).** At the same
  floor, about 94 KB: a saving of at most 245 KB. The reading does not say
  how many of the 93 list-carrying writes are label writes, so the real
  saving is between 0 and 245 KB. A label write whose body carried no list
  stays at 8.3 KB; what is left in it is the prompt (up to 16 KB on Texture
  runs, `GET /api/runs/{id}` of d@e on 2026-11-15) and the rest of
  `metadata`.

These are bytes per write, not a claim that the writes should exist.

## 9. Open questions (conservative default each) and what I could not verify

Open questions:
1. Does the list ever really change during a run? Default: assume not, per
   section 2.1 and proof test 3; if the counter shows changes after the
   change, write a new object per change.
2. If the object write fails, does the run start fail? Default: yes, the
   start returns the error like any `StartRunWithMetadata` failure; no inline
   fallback (a fallback is a switch). Whether the caller retries a failed
   start I did not trace.
3. Per-entity objects (append-only) instead of per-list snapshots? Default:
   per-list snapshots; revisit if the deployed reading shows lists that
   overlap heavily and do not dedupe.
4. Which of `vocab_migrate_og.go:130` and `vocabmigrate/families.go:182`
   need the new kind? Default: add it to all four lists in section 4 and let
   the gate test decide which are required.
5. The declared writer for the new kind: kernel, as for the run kinds.
   Default: kernel; the registry's `Writer` assignment for run kinds I did
   not trace.
6. Name and key spelling (`choir.run_source_entities`,
   `texture_source_entities_ref`): invented; default as written.
7. T1 (carry by reference) is parked; this slice does not depend on it.

Could not verify:
- The two bodies around the first re-encode, so the 19 changes as
  re-encoding is a hypothesis.
- How many of the 93 list-carrying writes are label writes (the counter's
  top key is per write, not per line).
- That the 25 `pending/new` writes are 25 runs each carrying the list, so
  the object count and size are upper bounds under that assumption.
- A consumer of the run API's `Metadata` outside Go: I searched the repo's
  files, not deployed clients.
- Whether the run API's current record equals the stored body (earlier note).
- Every enumerated kind list that must name the new kind (section 4 lists
  the four I found).
- Everything was read, nothing was compiled or run; the reading is 15
  minutes after a boot, one computer.
