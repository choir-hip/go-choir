# Effects replay run-memory import mismatch — 2026-08-18

**Boundary:** diagnose. Not a checkpoint, restore, retry, promotion,
qualified consensus, or effect authorization. Effects remain OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Authorized actuation

The prior docs-first deployed-probe receipt authorized exactly one deliberate
owner-scoped residue import on the retained computer, to exercise the deployed
canonical owner/run scope repair. The runtime identity was:

- staging `/health`: proxy `3359035bec8d0788d5e5ba0c61d87b0608323a9c`
- CI: <https://github.com/choir-hip/go-choir/actions/runs/32172934697>
- Node B deploy job: `95833299284`
- computer: `computer-03335285269bdba4f94377e56879f9e6`
- prior retained realization epoch: `313`
- effects mode: `propose_only`, generation `1`

The exact command was:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer import-residue-snapshot \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --host https://choir.news
```

The product route returned:

```json
{
  "computer_id": "computer-03335285269bdba4f94377e56879f9e6",
  "desktops": 1,
  "sessions": 0,
  "objects": 413,
  "edges": 111,
  "run_memory_entries": 922,
  "start_intents": 1,
  "operations": 1,
  "texture_mutations": 1,
  "appended": true
}
```

This was one event-chain append. The event was not deleted or SQL-reversed.

## Replay observation

The owner-authorized replay probe ran immediately afterward:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879e9f6 \
  --timeout 10m
```

Captured at `2026-08-18T19:23:50.020618186Z`:

- live and replay sequence: `3326`
- live and replay canonical event head:
  `3950851aaedce6a999b6cd6ac9be07656106ff1c1775f4de4348bc63d2167fbe`
- live and replay desired/effective event head:
  `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`
- live and replay desired/effective state commitment:
  `40df35913994fab47d2dd2c450a7f9d3958ea639ec9fb2002b8b8073534fe091`
- result: `not_equivalent`
- probe digest: `cc8e60711b86cffd8ffbecb5190a3ea31c33110f052090136644a5ee7dc339cd`

The exact remaining differences were:

```text
live   dolt:texture:content_root
       sha256:66ed1236b5f5f3b6e85429ddaee342e02484e54697e4f7b2ff8b5f025ae3a6b6
replay sha256:07c6c3f86b8822aa6810a764ad1b27cb9c6fbcdc47631d57031b1327c55acaac

live   dolt:texture:table:run_memory_entries
       sha256:5f0d3ac2b2f4dde488b377e0f4ac3a30e9d48dd096d91d1c64bf5a4d07223023
replay sha256:2cc0fb90167fc713f4760fe50253d089bf98d28cbacade4399ddce1326efd874

live   dolt:texture:table:texture_document_aliases
       sha256:d09d55cc2861385dfd1383ae924742ed8f145b92c67c60715ab7562a62a007ac
replay sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

The event heads match, so the append and chain replay completed. The
projection is still not equivalent. Replay eligibility remains false:

```text
reason: behavior-bearing direct-write tables are non-empty without reducers
unsupported_tables: [texture_document_aliases]
requires_non_nil_heads: true
```

## Problem classification

The import route reported 922 selected run-memory rows and appended a
projection batch, but the resulting replay table hash does not equal the live
hash. The observation proves that the deployed source repair was exercised; it
does **not** identify whether the mismatch is omitted live rows, a payload
normalization difference, a pre-existing projection overlap, or a writer race.
The current replay report exposes table hashes rather than row identities, so
no narrower source diagnosis is justified yet.

`texture_document_aliases` is an independent known blocker: the manifest still
classifies it `EmptyUntilSupported`, while the live table is non-empty. This
receipt does not authorize an alias reducer or cleanup.

## Next safe probe

Before another runtime repair or residue append, add a read-only,
owner-authorized diagnostic or focused fixture that compares, for the exact
computer:

1. live `run_memory_entries` row identities and canonical field fingerprints;
2. the `run_memory_entry_recorded` operation bodies in the appended projection
   batch and their replayed row fingerprints; and
3. row counts and any duplicate/overlap behavior across prior projection
   batches.

Do not append another residue snapshot, SQL-empty tables, remove response
bounds, bind a checkpoint, rematerialize, restore, retry self-development,
self-promote, invoke qualified consensus, or send mail while eligibility is
false. Preserve the retained computer, event chain, and rollback refs.

**Mutation class:** red observation. Protected surfaces: event append, replay
projection, replay eligibility, and retained-computer state.

**Rollback:** no product-state rollback is authorized. Platform rollback for
the deployed source repair remains `git revert
3359035bec8d0788d5e5ba0c61d87b0608323a9c`; the residue event is an event-chain
fact and must not be deleted or SQL-reversed.

**Heresy delta:** discovered — the explicit corrected residue import still
leaves a run-memory replay mismatch; introduced — none; repaired — none yet.

**Conjecture delta:** the canonical owner/run scope repair is proven exercised
by the import count and matching event heads, but replay equivalence remains
unproven. The exact row-level cause is unresolved. The alias table remains an
independent unsupported direct-write surface.
