# Object verbs, desk quotas and cost attribution (MA3.8, MA3.9)

Status: built 2026-11-05. Class red (MA3.8, tier 2) and orange (MA3.9).
Plan: [modular-architecture-plan-2026-11-04.md](modular-architecture-plan-2026-11-04.md)
section 7.3, rows MA3.8 and MA3.9; R8 (no spend ceiling, monitor cost).

## What a package desk gets

A package desk (`app:<pkg>`) keeps its data as objects of its own package's
kinds, `choir.app.<pkg>.<kind>`, which the package manifest declares and
`apppackage.RegisterManifest` registers with one writer, `desk:<pkg>`.

| Verb | Path | Authority |
|---|---|---|
| `choir.Put(kind, id, body)` | tray intent `object_put`, committed by the reducer when the cell returns | the writer is `objectgraph.DeskWriter(run record's desk)`; the intent has no writer field |
| `choir.Retract(kind, id)` | tray intent `object_retract` | same; the object is tombstoned, history stays on the tape |
| `choir.Get(kind, id)` | frame action `object_get` | the session's desk, never the cell's claim |
| `choir.List(kind, limit)` | frame action `object_list` | same |

Each object carries `schema_version` (the version of the kind as registered),
the writing run and the run's labels (MA3.5). A read returns them, and folds
`owner_private:app_object` plus the stored labels into the reading run.

## Failure modes (tests written first)

1. A cell supplies a writer. The intent decodes with no such field; the store
   refuses a writer that is not the kind's (`PutDeskObject`).
2. A desk writes a kind that is not its package's: another package's, a
   built-in, an unregistered name in its own namespace. Refused at the reducer
   (`validateObjectIntent`), before any side effect.
3. A built-in desk uses the verbs: refused (`not a package desk`); the verbs
   are in `appDeskVerbs` only.
4. A body breaks the kind's schema, exceeds `MaxObjectBody` (64 KiB), or an id
   is malformed: refused.
5. A desk reads another package's objects or another owner's: refused / not
   found.
6. A Retract leaves the object readable: tombstone, `Get` answers not found.
7. (MA3.9) A desk past its object count, its stored bytes or its write rate is
   committed anyway. The whole cell stays uncommitted (`preflightObjectIntents`
   runs before `commitTray`). A Put that replaces counts the difference; a
   Retract then a Put in one cell nets out.
8. (MA3.9) A desk wakes without bound: `createRunWithMetadata` refuses an app
   desk's run past `deskmanifest.MaxWakesPerHour` in a rolling hour; a manifest
   whose schedules together exceed it is refused at parse.

## MA3.9 numbers

Per package desk and owner: 10,000 live objects, 64 MiB of body bytes, 600
object writes an hour, 120 wakes an hour (the same figure bounds a manifest's
schedules). They bound the computer's own storage and write path, not spend.
The counters are in memory; a restart forgets the rolling windows (a desk at its
bound gets one more window). Object count and bytes are read from the store on
every cell that writes.

Cost: tool-loop events name the desk of the run (`desk`); `llmcost` groups
spend `by_desk` and `by_package` (`system` for built-ins, the package for
`app:<pkg>`); events without a desk (the judge's `jev_judgment`, the gateway's
`execution`) take their run's desk from the run record at read time
(`/api/costs`, at most 400 run lookups a request, the rest `unattributed`).
There is no ceiling and no refusal on spend (R8). The anomaly notice is MA7.7.

## Protocol

`internal/deskprotocol` 4.1.0 to 5.0.0: two closed-enum values on
`BrokerAction` (`object_get`, `object_list`), two on the `Intent*` enum, two
optional fields on `StagedIntent` (`object_kind`, `object_id`), two payloads.
The extractor classifies closed-enum additions as major (the plan said minor).

## Residuals

- Retract of a missing object is the cell's error, and a crash-replayed tray
  never resumes (restart rule), so no Retract is replayed.
- `ReadsSchemaVersions` is not enforced on read: `Get` returns the stored
  `schema_version` and the package reads it. A package that raises its schema
  must keep reading the old versions it declares.
- The write-rate and wake-rate windows are per process.
