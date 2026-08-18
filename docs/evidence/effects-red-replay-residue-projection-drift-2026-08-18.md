# Effects replay residue projection drift — 2026-08-18

**Boundary:** diagnose. Not a checkpoint, restore, retry, promotion, qualified consensus, or effect authorization. Effects remain OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Observation

After the scoped replay source repair `e58a0aab` reached staging as the deployed
source identity `dd0b5fa3c427d38cd9486a8398a3d605a7951331`, CI and the Node B
staging deploy both succeeded:

- CI: <https://github.com/choir-hip/go-choir/actions/runs/32165196258>
- Deploy job: `95808279606` (`Deploy to Staging (Node B)`)
- staging `/health`: `deployed_commit=dd0b5fa3c427d38cd9486a8398a3d605a7951331`,
  deployed `2026-08-18T17:49:03Z`

The retained computer was refreshed on the same computer, not replaced or
recreated:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- receipt: `01a015ff-37e6-7bc5-9c62-50cc796c8b7c`
- idempotency key: `effects-red-replay-scoped-refresh-20260818`
- realization epoch: `311 -> 312`
- lifecycle: `active -> active`

The owner-authorized read-only command was:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --timeout 10m
```

Captured at `2026-08-18T17:53:08.327188864Z`. The command completed with a
JSON report, but replay was not equivalent:

- sequence: `3320`
- live and replay canonical event head: `ed474587499367d7ec661bdd68b8dccde6b0aed4a323774ec340ba401106771d`
- live and replay desired/effective event heads: `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`
- live and replay desired/effective state commitment:
  `40df35913994fab47d2dd2c450a7f9d3958ea639ec9fb2002b8b8073534fe091`
- reducer version: `1`
- result: `not_equivalent`
- probe digest: `68c637382d676d407dd8e332fd939369962de9d22bbaa222081c66172eac51c4`

The exact observed differences were:

```text
live   dolt:texture:content_root
       sha256:36724acbaaba565ea997068d510a22c5a18e55ab7c1557cced3852b0cd177c00
replay dolt:texture:content_root
       sha256:6be8cd1bd2d9a0564cb0a89a5d18cef53b685ba51071ae20a30d4e9a24ba68e4

live   dolt:texture:table:run_memory_entries
       sha256:5f0d3ac2b2f4dde488b377e0f4ac3a30e9d48dd096d91d1c64bf5a4d07223023
replay dolt:texture:table:run_memory_entries
       sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

live   dolt:texture:table:texture_document_aliases
       sha256:d09d55cc2861385dfd1383ae924742ed8f145b92c67c60715ab7562a62a007ac
replay dolt:texture:table:texture_document_aliases
       sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

Replay eligibility therefore remained false:

```text
reason: behavior-bearing direct-write tables are non-empty without reducers
unsupported_tables: [texture_document_aliases]
requires_non_nil_heads: true
```

## Source-confirmed boundary

The repository declares `run_memory_entries` an event projection and
`texture_document_aliases` `EmptyUntilSupported`:

- `internal/agentcore/replay_eligibility.go`: `run_memory_entries` is
  `ReplayEventProjection`; `texture_document_aliases` is
  `ReplayEmptyUntilSupported`.
- `internal/store/run_memory.go:52-89`: when the production projection tape is
  bound, a run-memory append is encoded as a
  `run_memory_entry_recorded` projection operation rather than direct SQL.
- `internal/store/residue_import.go:274-307`: the owner-scoped residue snapshot
  can encode existing run-memory rows as replay projection operations, but only
  rows joined to a run for the retained computer are selected.
- `internal/store/texture.go:826-878`: document aliases still have a direct
  insert/upsert path; no event projection reducer currently owns that table.

These facts establish the repair boundary but not the exact row provenance.
The report does not expose which live rows produced either digest, nor whether
those rows were present before the authorized residue import or were appended
after it. The current evidence therefore does **not** justify classifying the
run-memory difference as a failed reducer, a missed residue import, or a
post-import writer race. The alias difference is directly classified as an
unsupported behavior-bearing table under the current manifest.

## Decision and next probe

This is a new replay-readiness blocker. Before another runtime repair, inspect
the exact live row identities and the canonical event/projection operations that
should reproduce them using an owner-authorized product surface or a focused
fixture. The probe must answer both questions:

1. Were the live `run_memory_entries` rows represented by canonical projection
   events for this computer, and if so were they replayed?
2. Is `texture_document_aliases` intended to become an event-backed projection
   or must the pre-launch eligibility manifest keep it empty until a real
   reducer exists?

Do not append another residue snapshot, SQL-empty tables, remove response
bounds, bind a checkpoint, rematerialize, restore, retry self-development,
self-promote, invoke qualified consensus, or send mail while eligibility is
false. Preserve the retained computer, event chain, and rollback refs.

**Rollback:** this is documentation-only; revert the documentation commit if
needed. No product state or event chain was mutated by the read-only probe.
