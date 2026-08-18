# Effects replay run-memory scope repair deployed probe — 2026-08-18

**Boundary:** diagnose and authorize the next execute slice. Not replay
acceptance, checkpoint, restore, retry, promotion, qualified consensus, or
effect authorization. Effects remain OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Landing and focused proof

The source repair committed as `3359035bec8d0788d5e5ba0c61d87b0608323a9c`
(`fix: scope residue run memory to canonical runs`) landed through:

- CI: <https://github.com/choir-hip/go-choir/actions/runs/32172934697>
- Node B deploy job: `95833299284`
- staging `/health`: proxy `deployed_commit=3359035bec8d0788d5e5ba0c61d87b0608323a9c`,
  deployed `2026-08-18T19:13:16Z`

The focused canonical-run fixture and the full `internal/store` package suite
passed before the commit:

```text
go test ./internal/store -run 'TestSnapshotResidueRuntimeIncludesCanonicalRunMemory|TestImportResidueSnapshot' -count=1
ok   github.com/yusefmosiah/go-choir/internal/store

go test ./internal/store -count=1
ok   github.com/yusefmosiah/go-choir/internal/store
```

The fixture covers a lifecycle-scoped `choir.run` object with no SQL `runs` row,
a legacy SQL run and memory row, and a foreign computer. It verifies the
owner/run pair selection and decodes both emitted run-memory projection bodies.

## Probe observation

The same retained computer was refreshed after the deployed repair:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- receipt: `01a0164b-993e-7b2e-bbdb-d34bfbd1de53`
- idempotency key: `effects-red-replay-run-memory-scope-refresh-20260818`
- realization epoch: `312 -> 313`
- lifecycle: `active -> active`

The owner-authorized replay read then ran:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --timeout 10m
```

Captured at `2026-08-18T19:16:31.328975597Z`:

- sequence: `3325`
- live and replay canonical event head:
  `93aa56259a059caea14725781848c2e94ee91f1b510ccddc63ba683139760a27`
- live and replay desired/effective event head:
  `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`
- live and replay desired/effective state commitment:
  `40df35913994fab47d2dd2c450a7f9d3958ea639ec9fb2002b8b8073534fe091`
- result: `not_equivalent`
- probe digest: `9dcca67365d7b2bbaab946fb3000617cec0d5e36775ad89387fb1054860f6713`

The remaining differences are unchanged in kind:

```text
live   dolt:texture:table:run_memory_entries
       sha256:5f0d3ac2b2f4dde488b377e0f4ac3a30e9d48dd096d91d1c64bf5a4d07223023
replay dolt:texture:table:run_memory_entries
       sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

live   dolt:texture:table:texture_document_aliases
       sha256:d09d55cc2861385dfd1383ae924742ed8f145b92c67c60715ab7562a62a007ac
replay dolt:texture:table:texture_document_aliases
       sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

Eligibility remains false with `texture_document_aliases` unsupported.

## Why this probe does not verify the repair

The repair is in `snapshotResidueRuntime`, which is called by the explicit
owner-authorized `choir computer import-residue-snapshot` product route. A
computer refresh only reboots the retained guest and preserves its event
chain; this actuation did not run the residue-import route. Therefore the
probe replayed the existing chain and could not observe a new corrected
`run_memory_entry_recorded` snapshot. This is an execution-surface gap, not
evidence that the focused source repair failed.

The previous residue-import evidence documents the same separate actuation:
refresh first, then `choir computer import-residue-snapshot`, then replay
completeness (`docs/evidence/effects-live-residue-import-2026-08-16.md`).

## Decision and next action

Record this as a new deployed-probe limitation. After this docs-first receipt,
execute exactly one owner-authorized residue snapshot import on this same
computer, specifically to exercise the deployed canonical-run scope repair;
then rerun replay-completeness. Treat that event as a deliberate corrected
state-as-of-now projection, not a blind retry. Do not SQL-empty tables, alter
response bounds, bind a checkpoint, rematerialize, restore, retry
self-development, self-promote, invoke qualified consensus, or send mail.
Keep `texture_document_aliases` `EmptyUntilSupported` and effects OFF.

**Mutation class:** red. Protected surfaces: event append, replay projection,
replay eligibility, and retained-computer state. No code or product state was
mutated by the replay read; the refresh advanced the retained computer through
its owner-authorized lifecycle receipt.

**Rollback:** platform rollback is `git revert 3359035bec8d0788d5e5ba0c61d87b0608323a9c`.
The corrective snapshot is an event-chain append and must not be deleted or
SQL-reversed; any product-state recovery remains acceptance-fenced and is not
authorized by this receipt.

**Heresy delta:** discovered — deployed repair verification requires the
separate residue-import actuation; introduced — none; repaired — none yet.

**Conjecture delta:** the source diagnosis and focused fixture remain supported;
live acceptance is unresolved until the corrected snapshot is appended and
replayed.
