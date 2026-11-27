# The source-entities list is not stored in the run record (T2, first slice)

Standing: built | kind: happened | settled-by: 8d81b7fe5 fix(texture) the source-entities list is no longer stored in the run record; the join ids are (first slice T2)

Date: 2026-11-15, round 1 repair. Class of this document: green (design
only). The change it designs is red: it touches the start of a Texture run,
whose record run acceptance and the evidence surfaces read.

Station: T2 of `docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`.
Background: `docs/design/store-once-2026-11-14.md` (R2). Reading rule: every
claim about code carries the file and line I read in this worktree; what I
could not read or run is in section 8; anything not in code or the counter's
reading is marked **invented**. Nothing here was run.

## 1. The reading and the outcome

Counter reading (`6387e5dff`, largest computer, 15 minutes after boot): 179
`choir.run` writes, 1.606 MB of body bytes; `metadata` is 90%;
`texture_available_source_entities` is the largest metadata key in 93 writes,
1,200,748 bytes, 74.8% of all run-record body bytes; it changed in 19 writes.

**Outcome of round 1: the list is not stored at all. The four join ids that
the only live reader needs are stored instead, and they already are durable
run-metadata keys.** The first two drafts added a new object kind to hold the
list. Reading the two readers again shows the kind is not needed, so it is
deleted, with every finding that depended on it (section 7).

## 2. The list in code

### 2.1 The one writer

- The list is built at the start of a Texture revision run:
  `texture_agent_revision.go:661-679` builds it from the current revision's
  sources, media sources in its content and the 12 latest commitment records,
  and stores it in the local map `metadata` (`:675`). That map is the
  revision's metadata decoded at `:598`.
- `texture_agent_revision.go:680-688` merges `req.SourceEntities` (a request
  field, `json:"-"`, `:34`). **The shadowing:** inside the `if`,
  `sourceEntities, changedSourceEntities := mergeTextureSourceEntities(decodeAvailableTextureSourceEntities(metadata), req.SourceEntities)`
  declares new variables, so the outer `sourceEntities` of `:666` is not
  updated, but the inner one is written to `metadata[key]` at `:683`. The
  merge starts from `decodeAvailableTextureSourceEntities(metadata)`, which
  reads the value `:675` stored. **So today's value in the run record is the
  merged list, including `req.SourceEntities`.** Byte-equality is against
  that merged value, and the one reliable way to name it is
  `decodeAvailableTextureSourceEntities(metadata)` after both blocks.
- The copy into the run: the loop at `:789-797` copies each key of
  `durableMetadataKeys` from `metadata` into `runMetadata`. The list key is
  one of them (`texture_task_types.go:66`), and so are the four join keys
  (`:67-70`). `StartRunWithMetadata` (`:823`) creates the run.
- No other writer: `mergeTextureSourceEntitiesIntoRunMetadata`
  (`texture_evidence_sources.go:468-477`) has no caller outside a test
  (`texture_evidence_sources_test.go:364`).

### 2.2 The readers

| Reader | What it needs | Evidence |
| --- | --- | --- |
| R2 `buildAppagentRevisionMetadata` | only the four join ids: `join := selfDevelopmentJoinFromSourceEntities(decodeAvailableTextureSourceEntities(rec.Metadata))`, then `mergeSelfDevelopmentJoinIntoMetadata(meta, join)` (`texture_revision_metadata.go:75-76`). The join is a pure function of the list: it reads `entity.Kind` and `Target.PublicRecordID` or `FilePath` and does no I/O (`texture_evidence_sources.go:797-816`). The merge sets a key only when the value is non-empty and overrides what was there (`:818-827`). | |
| R1 `coagentTextureSourceEntities` | the parent run's list plus content items (`coagent_route.go:416-418`) | |
| the model's input | not the list and not the join keys. The revision prompt reads `seed_prompt`, `owner_prompt`, `source_path`, ... (`texture_agent_revision.go:940-1030`). R1's list does reach a coagent prompt (`coagent_route.go:64,70`), see R1. | |
| observability, run acceptance, replay | no reader of the key in Go (the constant is named only in `texture_task_types.go`, `texture_agent_revision.go`, `texture_evidence_sources.go`, `texture_revision_metadata.go`, tests, the counter's name list). The run API serves `Metadata` as is (`api.go:603`); no consumer found in the repo's other files. | |

**R1 is dead code on this key.** Its function `ensureCoagentTextureRevisionRoute`
refuses unless the caller profile is `processor` or `reconciler`
(`coagent_route.go:44-48`: "no live profile can enter this path"). Its only
caller is `EnsureTextureHandoff` for the kinds `source_open` and `corpus_wake`
(`texture_handoff.go:110-114`), and no non-test code builds a `HandoffRequest`
of those kinds: the two construction sites use `HandoffKindUserPrompt` with the
front-door profile (`prompt_bar.go:402-403`, `api_texture_prompt_eval.go:123-125`).
The processor/reconciler pipeline was deleted (`3c963edf1`). And the parent
of R1 is a processor or reconciler run, which the writer in section 2.1 never
starts, so its metadata never carried the list. R1 needs no list and gets none.

**R2 can be served without the list.** If the four join keys are in the run
record from the start, R2 finds them where it already looks: it copies every
durable key from the run's metadata into the revision's metadata
(`texture_revision_metadata.go:38-44`), and the join keys are durable keys.
The merge at `:76` then adds nothing, because `decodeAvailableTextureSourceEntities`
returns nil for a record with no list (`texture_media_sources.go:352-354`) and
the join of nil is empty (`texture_evidence_sources.go:797-798`).

### 2.3 Lifetime

Per run. The persisted revision metadata drops the key
(`texture_revision_metadata.go:76-81`, "recomputed from the actual revision's
source_entities"); the next run rebuilds the list at its own start.

## 3. The mechanism

At the start of a Texture revision run, after both merges
(`texture_agent_revision.go:688`):

1. `final := decodeAvailableTextureSourceEntities(metadata)`, the one variable
   holding the list that today goes into the run record (section 2.1).
2. `join := selfDevelopmentJoinFromSourceEntities(final)` is derived there, and
   `mergeSelfDevelopmentJoinIntoMetadata(runMetadata, join)` sets the non-empty
   join keys in the run's metadata **after** the loop at `:789` and after the
   `req.Provenance` copy that follows it. The override precedence is the one R2
   applies today at completion: a join value from the list replaces a value
   inherited from the parent revision and a value a caller put in provenance.
3. `textureAvailableSourceEntitiesKey` leaves `durableMetadataKeys`
   (`texture_task_types.go:66`), so the loop stops copying the list into the
   run. The local map keeps the key for `:681`.

R2 (`texture_revision_metadata.go:75-76`) and R1 (`coagent_route.go:418`) do
not change. Nothing new is stored: no object, no kind, no op, no reference,
no loader; the run record carries the list-derived join values in the keys it
already may carry.

**Every consumer of `durableMetadataKeys`, and what changes for each.** The
list is read in four places (`texture_task_types.go:43`), so dropping the list
key from it is not a run-record-only change:

| Consumer | Today | After |
| --- | --- | --- |
| run start, `texture_agent_revision.go:789-797` | copies the list and the join keys from the local map into the run | copies the join keys (now list-derived) and no longer the list |
| R2 parent-revision copy, `texture_revision_metadata.go:26-33` | copies the parent revision's durable keys, list included, into the new revision | no longer copies the list; R2 deleted it from the revision anyway (`:81`) |
| R2 run copy, `texture_revision_metadata.go:38-44` | copies the run's durable keys, list included | copies the join keys the run now holds; no list to copy |
| `carryForwardDurableTextureMetadata`, `texture_structure.go:35-76` (loop `:55`), reached from `carryForwardTextureMetadataForRecord` (`texture_structure.go:19`), which `texture_agent_revision.go:252`, `texture_evidence_embed.go:230` and `revise_edit.go:99` call, and from rebase (`texture.go:1152`) | a legacy parent revision whose persisted metadata still holds the list passes it to the child revision | no longer passes it. Before this change it did copy the key from a head revision that held it into owner-revise, tool and embed revisions (`revise_edit.go:99`, `texture_evidence_embed.go:230`, `texture_agent_revision.go:252`, rebase `texture.go:1152`), so a legacy list could persist across those; after, post-deploy revisions stop carrying it. The join keys are durable and the same loop still copies them, so no join id is lost. It matches the rule in R2's own comment that the list is run-time context and not durable revision metadata |

The join keys now also appear in Texture run records that did not carry them
before. `store.ListRunsBySelfDevelopmentOperation` (`store.go:687-720`) queries
that key, but it has no non-test caller on main, and it is documented as
returning the top-level Management run; a future caller would also see
Texture runs. Named, not prevented. The test-only helper `mergeTextureSourceEntitiesIntoRunMetadata`
(`texture_evidence_sources.go:468-477`) is deleted, so no code can write the
key into a run record again; its one test sets the key on the record directly.

What the run record loses: the list. What it keeps: the four join values that
its only live reader derived from the list, when the list had entities of the
kinds `operation`, `capsule_bundle`, `receipt` or `event_head`. What the model
sees: unchanged, since it never read either.

### 3.1 Why this is safe in both directions

- **Old binary on a run started by the new one (rollback).** Old R2 finds no
  list, computes an empty join, merges nothing, and the join keys are already
  in `meta` from the durable-key copy of the run's metadata
  (`texture_revision_metadata.go:38-44`). The revision metadata is the same.
- **New binary on a run started by the old one (deploy).** The record has
  the list inline and possibly inherited join keys (the join keys are durable,
  so the old start copied the parent revision's values). R2 is unchanged, so
  it computes the join from the inline list and overrides the inherited value
  exactly as today.
- So there is no two-step landing, and none is needed: the change is
  correct in both directions on its own, and the old tape stays readable
  because no stored shape changes.

The rollback path, as read: automatic rollback is `restorePrior`, which calls
`service.RecoveryRestart` (`internal/updater/updater.go:1266-1275`) and writes
no marker, so the boot closes open runs (`internal/agentcore/runtime.go:3066-3083`).
Markers are written by `WritePlannedRestartMarker` (`planned_restart.go:483`)
and through `writeHostPlannedRestartMarker` (`telemetry_batching.go:118-153`),
which is reached from the platform refresh (`planned_restart.go:846`) and from
the self-development restore restart (`self_development_restore_restart.go:90`),
a real rollback into an older release that is marker-backed. A run resumed
under the other binary in either case still produces the same revision
metadata by the two bullets above, so correctness does not depend on the
rollback path.

A comment at the R1 gate (`coagent_route.go:44-48`) names this design, so a
future re-enabling of the route knows that a parent's list is no longer
stored. That comment is the only edit at R1.

## 4. Expected saving (estimate)

- The list is 1,200,748 of 1,606,000 body bytes. Left: about 405 KB. The join
  keys add 0 to about 0.5 KB a write when the list had self-development
  entities (invented bound; a join id is a record id or file path). So 405 to
  450 KB: **factor 3.6 to 4.0 fewer run-record body bytes, a saving of about
  1.2 MB, 75%.** No object is added, so there is nothing to subtract.
- The mean write falls from 9.0 KB to about 2.3 KB, the measured size of the
  lines that never carried the list (`create_og`, `update_og`).
- Not saved: the live store holds one body per run either way. The saving is
  tape bytes (the tape carries bodies in base64, a third larger, in both
  columns).
- Stated without designing them: `activation/running/updated_at_only`
  (23 ops, 296 KB) falls to about 53 KB at the floor, a saving of about 240 KB.
  `labels/running/labels` (41 ops, 339 KB, 8.3 KB a write) falls to at most
  about 94 KB, saving at most 245 KB; the reading does not say how many of
  the 93 list-carrying writes are label writes, so the saving is between 0 and
  245 KB, and what stays in a label write that carried no list is mostly the
  prompt (up to 16 KB on Texture runs) and the rest of `metadata`.

## 5. Failure list

1. **R2 loses or changes a join id.** The join is computed from the same list
   by the same function, and the precedence over the parent revision's value
   is the same. Prevented by proof test 1 (a list with one entity of each of
   the four kinds, and a parent revision that has its own join keys).
2. **The merged list is not the stored list.** Today the stored list is the
   merge of `req.SourceEntities` into the list (section 2.1; the shadowing
   affects only the local variable). The design reads it from `metadata`
   after both blocks, so it is the stored one. Proof test 1 runs with a
   non-empty `req.SourceEntities`, with entities that carry join kinds.
3. **A run started on the old binary and finished on the new.** Its record
   has the inline list and no join keys; R2 is unchanged and computes the
   join from the inline list. Proof test 2 (a record that has been through one
   `updateRunOGOnce` re-encode, `graph_store.go:748`, `:783`).
4. **A run started on the new binary and finished on the old** (rollback).
   Old R2 gets the join keys from the durable-key copy; section 3.1. Proof
   test 3 runs R2 on a record with join keys and no list and compares.
5. **Replay of old tape, and an old binary on new tape.** No stored shape
   changes: runs from before carry the list, runs after carry the keys, both
   are `choir.run` bodies written as before. The tape is not rewritten.
6. **R1 reads a parent's list.** R1 is unreachable in production and its
   parent never carried a list (section 2.2). It is left as is; the failure is
   a future re-enabling with a processor-style parent, whose metadata would
   have no list, so R1 would return only content items.
7. **A join id larger than the assumed 0.5 KB.** A join id is a record id or
   a file path. Bounded by what the entity's `Target` holds; the counter shows
   it per write (`run_writes` bytes by field). Not prevented, measured.
8. **A legacy key in an old revision's metadata.** An older revision may
   still carry the list in its persisted metadata. The local flow at
   `:666` and `:681` reads it as today; the list reaches `final` and the join,
   and does not reach the run. Proof test 1 includes such a revision.
9. **Model input byte-equal.** The revision prompt does not read the key or
   the join keys (section 2.2). Proof test 1 compares `agentPrompt` before
   and after for the same evidence sequence.
10. **A client reading the list from the run API.** The API serves `Metadata`
    as is (`api.go:603`); new runs lack the key. No consumer found. Named,
    not prevented.
11. **The key comes back into a run record.** Prevented by deleting the
    test-only helper (section 3) and by the gate test in proof test 4.

Not failures of this design, because it adds no stored object: a crash between
two writes, a missing object, retirement of the run and an object together,
restore or fork carrying both, and concurrent writers of a new object. They
were the failures of the earlier drafts and are removed with them.

## 6. Proof

Repo tests (written first, class yellow):
1. Join and merge equality. For one evidence sequence, build the run's
   metadata with the change and compare to a golden taken from today's code on
   `origin/main`: the revision metadata JSON from `buildAppagentRevisionMetadata`
   and `agentPrompt`. Inputs: a non-empty `req.SourceEntities` with join-kind
   entities, evidence entities, a parent revision with its own join keys, and
   a revision whose metadata still carries the legacy key. (After the change
   there is no inline writer to run twice, so the golden is recorded first.)
2. An old-shape run record (inline list, possibly inherited join keys),
   JSON round-tripped as `Store.UpdateRunOG` does (`graph_store.go:714`; the
   unexported `updateRunOGOnce` is not callable from the package), gives the
   same revision metadata through R2.
3. A new-shape run record (join keys, no list) gives the same revision
   metadata through R2.
4. `textureAvailableSourceEntitiesKey` is absent from
   `durableMetadataKeys`, and a run started with a non-empty merged list and a
   non-empty `req.SourceEntities` has stored metadata that lacks the key and
   holds the join keys, and a body smaller than before. (The start code still
   assigns the key in its local map at `:675` and `:683`, so a rule on source
   assignments would fail on the design itself.)
5. A re-encode test (an explanation of the counter's 19 changed writes, not a
   safety test): marshal a typed list, decode to `map[string]any`, marshal
   again; different bytes, equal content.
6. The whole uncached suites of `internal/textureowner` and `internal/agentcore`
   (and every package that imports them); `scripts/go-test-runtime-shards`.

Deployed, on the test computer after the staging deploy, scoped to runs
created after the deploy (runs already open keep writing the list while they
finish, and the counter's key list keeps the old name for that reason):
- the largest-key entry for `texture_available_source_entities` stops
  growing after the deploy SHA, and the `activation/running/updated_at_only`
  mean falls (no factor is promised: the counter has no profile dimension, so
  its lines mix runs; a profile dimension is a separate change).
- A controlled document with a known evidence set ends a Texture run with a
  revision whose join keys equal the expected ids (not compared with a run on
  the previous release: the evidence sequence moves between runs).
- Whole-tape replay of the test computer's tape into a fresh store: every
  `choir.run` object has an equal content hash, none missing. (Nothing new is
  stored, so this is the check that nothing else moved.)
- An accepted run before and after: the acceptance record's fields equal.

## 7. Round 1 findings and what was done

| Seat and item | Finding | Verified | Done |
| --- | --- | --- | --- |
| DeepSeek B1 | object written before the admission gates, no store API for it | true (`runtime.go:1497-1511`; `ogPut` unexported `graph_store.go:91`) | moot: the object is deleted; nothing is written before admission |
| DeepSeek B2 | design stored the pre-merge list, shadowing at `:674-688` | the shadowing is real; today's stored value is the merged list (section 2.1) | the join is computed from the one variable after both merges; proof test 1 with non-empty `req.SourceEntities` |
| DeepSeek B3 / Claude S6 | a missing object is logged and the operation continues | true for the old design | moot: no object, no loader, no ref |
| DeepSeek B4 / Claude S2 | the four kind lists are the wrong set | true (`vocabmigrate/families.go` is frozen; `snapshot_to.go:400` is a table row) | moot: no kind is registered, so no manifest claim, no formula, no frozen entry, no table entry |
| Claude B1 / DeepSeek S5 | rollback marker unverified | read (section 3.1) | stated as fact; no two-step landing needed because the change is correct in both directions without it |
| DeepSeek S1 | `writes_per_object == 1` proof | true | moot: no object |
| DeepSeek S2 | capsule copy after a rollback drops the new kind | true for the old design | moot: no new kind |
| Claude S3 | R2 only needs the join | true and extended: R2 needs nothing stored but the keys | mechanism of section 3 |
| (asked) | can R1 be served without the stored list | yes: R1 is unreachable and its parent never carried the list (section 2.2) | R1 untouched |
| Claude S4, DeepSeek S3 | anchor on `updateRunOnce`; test 1 unfaithful | `updateRunOGOnce` decodes into `RunRecord` (`graph_store.go:748`), re-marshals (`:783`); `Metadata` is `map[string]any` | anchors fixed; proof tests 1 to 3 use a golden and a re-encoded record |
| Claude S5 | the dual-shape loader needs an end | no loader exists now. R2's list read becomes a no-op for new runs; it can be deleted when no active run's start predates the release (the oldest non-terminal run's `created_at` from the obligations surface, `obligations_surface.go:285-293`) | stated in section 3 |
| DeepSeek S4 | "model input not read" is true only of the revision prompt | true (`coagent_route.go:64,70`) | section 2.2 says R1's list reaches a coagent prompt, and R1 is dead |
| DeepSeek S6 | test-only merge helper names the key | true | helper deleted (section 3) |
| DeepSeek S7 | the object has no retire rule | true for the old design | moot |
| DeepSeek nits | `run_write_counters.go` anchors; confusable name | | moot or fixed |

## 7a. Round 2 findings and what was done

Both seats ACCEPT with no blocking finding.

| Seat and item | Finding | Done |
| --- | --- | --- |
| DeepSeek S1, Claude S2 | the fourth `durableMetadataKeys` consumer (`carryForwardDurableTextureMetadata`) and R2's parent-revision copy are unnamed | the consumer table in section 3; proof test 1 includes a legacy parent revision through `carryForwardDurableTextureMetadata` |
| DeepSeek S2 | "no join keys of its own" is false: the join keys are durable and inherited | section 3.1 reworded; proof test 2 fixture carries inherited join keys |
| DeepSeek S3 | marker enumeration wrong; self-development restore is marker-backed | section 3.1 corrected |
| Claude S1 | join keys now reach Texture run records; `ListRunsBySelfDevelopmentOperation` | named in section 3; it has no non-test caller |
| DeepSeek S4, Claude S4 | deployed proof not deterministic; counter has no profile dimension; scope to runs after deploy | section 6 rewritten |
| DeepSeek S5 | proof 2's anchor is unexported | section 6, test 2 |
| DeepSeek S6, Claude S3 | gate test should be behavioural and narrower | section 6, test 4 |
| DeepSeek S7 | delete `mergeTextureSourceEntitiesIntoAvailableContext` with its sibling | both helpers are deleted in the build |
| DeepSeek S8 | the run-event-ledger checkpoint serializer would show a different `metadata=` line if it were ever enabled | named boundary: `serializeRunEventLedgerForCheckpoint` (`run_memory.go:1308-1311`) is unreachable today (`canCheckpointRunEventLedger` needs reason `continuation_selection`, `:592`); not changed |
| Claude S5 | pin last-wins and precedence | proof test 1 cases: two entities of one join kind (last wins); the list-derived value beats a different value on the parent revision and on the run's inherited copy; empty ids stay absent |
| DeepSeek nits | keep the counter key list; R1 pointer comment; empty and absent identical | counter list kept; comment added at R1; tested |

## 8. Open questions and what I could not verify

Open questions: none left open by design. Decisions made: R1 untouched;
the test-only helper deleted; no two-step landing; no new object.

Could not verify:
- That no processor-style run ever carried a list (the writer in section 2.1
  is the only one I found; the old pipeline is deleted, but old tape may hold
  runs I cannot read).
- The size and frequency of join ids in the paper's runs.
- How many of the 93 list-carrying writes are label writes.
- That the 19 changes are re-encodes. The code explains them (section 2.1,
  proof test 5); I could not read the two bodies.
- The caller of `WritePlannedRestartMarker` at `telemetry_batching.go:138`,
  and the full rollback path of `.github/scripts/node-b-ci-deploy` (the
  script's rollback section is an incomplete-receipt record, line 44; I did
  not trace it).
- A consumer of the run API's `Metadata` outside the repo.
- Everything was read; nothing was compiled or run.
