# The actor wake outbox list reads every outbox row ever written (2026-10-23)

Date: 2026-10-23. Station: FP4 follow-up (lane `lane/fp4-outbox-scan`), from
FP0's reading of the paper on D4 build `ef450642b`. Mutation class of this
record: green. No runtime code changed here, nothing run against Node B, a
computer or staging.

Evidence labels: **[V]** verified by reading the code at the cited
`file:line` on this checkout (`ef450642b`), or by running the cited test
locally; **[M]** FP0's measurement on the paper, not re-measured here;
**[H]** hypothesis, with the observation that settles it.

## Evidence

**[M]** After a cold boot at 23:55 the paper's `/health` answered 503
`store_stalled` from 00:02:55 to 00:05:06. The store-stall holder log printed
twice, a minute apart, the same goroutine, `runnable`, in
`internal/objectgraph.(*DoltStore).ListObjects`, inside
`dolt/go/store/nbs.(*archiveReader).readByteSpan` (the guest decompressing
Dolt archive chunks; the VMM thread idle, four vCPUs busy; store about 34 GiB
used, 26 GiB live). Then, at 00:04:35, `runtime: actor wake outbox list
wakes=0 dur=48.196s`.

**[M]** The same console line over the evening, `wakes` and `dur`:

| time | wakes | duration |
|---|---|---|
| 20:34:57 | 1 | 2.405s |
| 20:46:51 | 1 | 3.159s |
| 20:55:52 | 4 | 2.491s |
| 21:04:13 | 1 | 5.619s |
| 22:17:56 | 1 | 7.032s |
| 22:30:29 | 0 | 5.844s |
| 23:00:47 | 0 | 4.375s |
| 23:55:51 (right after boot) | 0 | 4.828s |
| 00:04:35 | 0 | 48.196s |

The line is logged only at or above 2 s (`wakeLatencyLogAfter`,
`internal/agentcore/runtime.go:2698`), so the table is the slow tail, not
every sweep.

## What the line measures

**[V]** `sweepActorWakes` (`internal/agentcore/runtime.go:2719`) starts the
clock at `:2727`, then calls `TakeAnnouncedActorWakes` (`:2729`, a signalled
sweep) or `ListUnprojectedActorWakes` (`:2732`), and logs at `:2741`. The
clock therefore covers the wait for the store's engine mutex as well as the
query. A `dur` of 48 s with 0 wakes is the sum of the two, and the log cannot
say which.

## The query

**[V]** `ListUnprojectedActorWakes` (`internal/store/lifecycle.go:877`) calls
`ogListAllByMetadata(ctx, "choir.actor_wake_outbox", "projected", "false")`
(`internal/store/graph_store.go:134-167`), which pages
`ListObjectsByMetadataPage` (`internal/objectgraph/dolt_store.go:1336`) 256
rows at a time until a page is empty. With 0 to 4 open wakes the first page is
short but non-empty only when there is a wake; the loop always ends on an
empty page, so a sweep with k open wakes runs the query `ceil(k/256) + 1`
times, i.e. twice for k from 1 to 255 (one page of k rows, then an empty
page), once for k = 0.

**[V]** For this kind and field no fast path applies:

- `metadataColumnFor` (`dolt_store.go:576`) serves only the fields in
  `metadataIndexColumns` (`dolt_store.go:220-226`): `run_id`, `trajectory_id`,
  `channel_id`, and three commitment-record fields. `projected` is not one.
- `derivedMetadataRead` (`internal/objectgraph/derived_keys.go:367`) serves
  only the families in `derivedFamilies` (`derived_keys.go:76`) and
  `derivedFamilyForMetadataRead` (`:106`): `run.agent_id`,
  `commitment_record.addressee`, `source_ref.doc_id`. The outbox is not one.

So `metadataPageSQL` (`dolt_store.go:1261-1274`) returns the JSON form:

```sql
SELECT <all 12 columns incl. body, metadata> FROM og_objects
WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.projected')) = 'false'
  AND canonical_id > ? ORDER BY canonical_id ASC LIMIT ?
```

**[V]** `EXPLAIN` of exactly that text, run locally by
`TestScratchWakeScan` against a store with 3000 closed and 4 open wakes
(scratch test, not committed), prints:

```
Limit(256)
 └─ Project (all 12 columns)
     └─ TopN(Limit: [256]; og_objects.canonical_id ASC)
         └─ Filter (json_unquote(json_extract(convert(og_objects.metadata, json), '$.projected')) = 'false')
             └─ IndexedTableAccess(og_objects)
                 ├─ index: [og_objects.object_kind, og_objects.og_rec_conversation_id, og_objects.canonical_id]
                 ├─ filters: [{[choir.actor_wake_outbox, ...], [NULL, ∞), (, ∞)}]
                 └─ columns: [canonical_id object_kind owner_id ... body metadata ...]
```

Read from the bottom: the only index use is the `object_kind` prefix of an
unrelated generated-column index, so the access path is "every row of the
kind". Every row is read in full (`body` and `metadata` included), the JSON is
parsed per row, the filter keeps the open ones, and a `TopN` sorts the
survivors by `canonical_id` because that index does not order by it. The
cost is proportional to all outbox rows ever written, not to the open ones.
Nothing in the outbox is deleted: a wake is marked `projected` (or given a
fate), never removed (`MarkActorWakeProjected`, `lifecycle.go:~919`;
`markActorWakeFate`, `actor_wake_signal.go:~190`).

**Why a full scan, in the task's terms:** a missing index for this predicate
(the filter is a JSON predicate on `metadata`, not on a column), an unindexed
filter, and a sort by `canonical_id` the chosen index does not provide. There
is a limit, but it applies after the scan and the sort, so it bounds the
result, not the work.

## How often it runs

**[V]** `runActorWakeProjector` (`internal/agentcore/actor_wake_projector.go:88`)
sweeps with `announced = (reason == "signal")`. The whole-outbox read
(`ListUnprojectedActorWakes`) therefore runs:

1. at start (`reason = "start"`): once per boot;
2. at every audit (`reason = "audit"`): every `actorWakeAuditInterval`, one
   minute (`actor_wake_projector.go:19`), whenever the signal does not wake the
   loop sooner;
3. at every retry or `more` pass (`reason = "retry"` or `"more"`);
4. at a signalled sweep whose commit did not name its rows or overflowed
   (`TakeAnnouncedActorWakes` returns `full`, `actor_wake_signal.go:102-110`;
   `signalActorWakes` is called at `internal/store/lifecycle.go:1189`);
5. at every `GET` of the obligations surface (`HandleObligations`,
   `internal/agentcore/obligations_surface.go:245`).

The E6 change ([`research-more-slow-and-hidden-2026-10-21`](research-more-slow-and-hidden-2026-10-21.md))
moved the per-wake read onto the announced-key path (case 4 aside). It left
the full read at one per minute plus boot, and that is the line in the table:
the 2 to 7 s rows are audit and start sweeps, not wake latency.

## What it holds, and what waits behind it

**[V]** The query runs on the store's read handle, a `sql.DB` configured with
`SetMaxOpenConns(1)` (`internal/store/texture.go:353`; created at
`internal/store/store.go:779-782`), and the read `DoltStore` shares the write
store's engine mutex (`store.go:812-813`, `ShareEngineMutex`). Each page takes
the mutex for the whole query (`dolt_store.go:1341`, `LockAs`) and releases it
only after the rows are scanned (`:1342` defer). With no open wakes, the one
and only page is the whole scan, so the mutex is held for the entire read of
the kind. Every other store operation queues behind it: the `/health` probes
for desk pending mutations, self-development active operations and the
effective release (the three `unanswered` names in the stall log), the desks'
reads, and the writers.

## What this does and does not explain

**[V]** The scan is a real, avoidable cost that scales with the age of the
paper, runs at least once a minute and holds the one connection and the engine
mutex while it runs. It is a likely contributor to the 2 to 7 s lines.

**[H] The 48 s line is not shown to be this scan.** The holder log at
00:02:55 to 00:05:06 names `ListObjects`, not `ListObjectsByMetadataPage`:
a different query (`ORDER BY updated_at DESC LIMIT ?` with kind, owner,
computer and tombstone filters, `dolt_store.go:719-762`), which takes the
engine mutex unlabelled (`s.engineMu.Lock()` at `:723`, so it has no op name in
`EngineMutexStats`). The wake list's clock includes its wait for the mutex, so
the most direct reading is that the sweep was a victim of the `ListObjects`
holder for about two minutes. What settles it: the engine-mutex counters and
live holders in `/health` detail (`EngineMutex`, `EngineMutexLive`,
`internal/agentcore/api.go:998-999`) name the outbox read as
`(*DoltStore).ListObjectsByMetadataPage choir.actor_wake_outbox $.projected json`
and every `ListObjects` as one unlabelled op. This record's fix labels
`ListObjects` by kind so the next stall names its holder.

**[H]** The outbox row count on the paper. Not read from the paper. After the
fix lands, the first validation pass logs `derived keys
actor_wake_outbox.projected: validation pass starts over N rows`, which is N.

## Options considered

1. **Use an existing index.** None exists for `projected`; `idx_og_key_index_lookup`
   `(key_name, key_value, canonical_id)` on the existing `og_key_index` side table
   does. That is the "derived key" path.
2. **A small open-wakes set in memory.** Seeded by one full read at boot,
   kept current by the writers. Process-local; the audit sweep exists because
   a signal can be missed, so the set would have to be re-proved by the very
   read it replaces.
3. **Bound and page the scan.** Does not reduce the rows read per page (the
   `TopN` still sorts the whole kind per page); only splits the mutex hold.
4. **A new `og_objects` generated column and index.** Costs journal bytes on
   every `og_objects` write
   ([`paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22`](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md))
   and a schema change needs its own deploy. Not chosen.

Chosen: option 1, a fourth derived-key family, `actor_wake_outbox.projected`,
on the side table that already exists. Code only: no `og_objects` index or
column, no DDL. It adds one `og_key_index` row per outbox row (a `REPLACE` in
the writer's transaction, as the other three families do) and rewrites it once
when a wake is marked. It is read only after the online validation pass has
compared it with `og_objects`; until then, and after any withdrawal, the read
keeps today's JSON path. It does not touch the wake protocol, delivery
semantics, the outbox rows or their metadata.

Class: **orange** (store query, object-graph read path; rollback: revert the
commit; the side table's extra rows are inert and ignored by older binaries,
which do not read the new family).

## Heresy delta

- discovered: the full outbox list is a JSON scan of an append-only kind; the
  stall holder at 00:03 to 00:05 is `ListObjects`, which has no engine-mutex op
  label; `ListExhaustedActorWakes` (`actor_wake_signal.go:222`, the
  obligations surface) is the same shape on `$.fate` and is left as is (it is
  read on an owner `GET`, not on a loop).
- introduced: none yet.
- repaired: none yet.
