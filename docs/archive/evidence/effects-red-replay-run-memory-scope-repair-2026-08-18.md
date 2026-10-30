# Effects replay run-memory scope repair — 2026-08-18

**Boundary:** execute and verify the corrected residue-import slice. Not whole-computer replay acceptance, checkpoint, restore, retry, promotion, qualified consensus, or effect authorization. Effects remain **OFF**.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Repair landing

The source repair committed as `7bdc0a7ceb063d29542f2aa90560439db97d8aee`
(`fix: scope residue memory to canonical runs`). `residueRunScopes` now queries
canonical `choir.run` objects by their persisted `computer_id` metadata/body,
then joins run-memory rows by the canonical owner/run pair. Legacy SQL `runs`
rows remain a compatibility source. The focused fixture now uses `CreateRunOG`,
the production writer whose object-graph storage `computer_id` column is empty
while the canonical body/metadata carry the computer identity; it also checks a
foreign computer and a legacy SQL run.

Landing evidence:

- CI: <https://github.com/choir-hip/go-choir/actions/runs/32186509122>
  (the first attempt had unrelated pre-existing race-shard flakes; failed jobs
  were rerun and the workflow completed successfully).
- Successful CI head SHA: `7bdc0a7ceb063d29542f2aa90560439db97d8aee`.
- Node B deploy job: `95878530936`.
- Staging `/health`: proxy `deployed_commit=7bdc0a7ceb063d29542f2aa90560439db97d8aee`,
  deployed `2026-08-18T21:49:42Z`.
- Focused local proof:

```text
go test ./internal/store -run TestSnapshotResidueRuntimeIncludesCanonicalRunMemory -count=1
ok   github.com/yusefmosiah/go-choir/internal/store

go test ./internal/store -run 'Test(ImportResidueSnapshot|SnapshotResidueRuntime)' -count=1
ok   github.com/yusefmosiah/go-choir/internal/store

go test ./internal/store -count=1
ok   github.com/yusefmosiah/go-choir/internal/store
```

## Same-computer corrected import

The retained computer was refreshed through the owner-scoped product path after
the repair landed:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- refresh idempotency key:
  `effects-red-replay-run-memory-canonical-scope-repair-20260818`
- lifecycle receipt: `01a016dc-161d-7b44-9611-a1bd674e9e7e`
- realization epoch: `315 -> 316`
- lifecycle: `active -> active`
- mode: still `propose_only`; no effects were armed and no mail was sent

The explicit owner-authorized residue-import route then returned:

```json
{
  "computer_id": "computer-03335285269bdba4f94377e56879f9e6",
  "desktops": 1,
  "sessions": 0,
  "objects": 413,
  "edges": 111,
  "run_memory_entries": 1083,
  "start_intents": 1,
  "operations": 1,
  "texture_mutations": 1,
  "appended": true
}
```

This is one deliberate state-as-of-now event-chain append. It is not a deletion,
SQL reversal, or blind retry of the self-development operation.

## Replay result

Owner-authorized replay completeness was then run against the same computer:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6
```

Captured at `2026-08-18T21:54:20.640804481Z`, sequence `3342`:

- live and replay canonical event head matched:
  `0a2362bf34b5e1d06559baea407fb564751390b45f10cfeba57779265e188980`
- live and replay desired/effective event heads matched
- live and replay desired/effective state commitments matched
- `run_memory`: `live_count=1083`, `replay_count=1083`,
  `live_only_count=0`, `replay_only_count=0`, `different_count=0`
- result: `not_equivalent`
- probe digest: `9119e98975991815da70ecdb77270dde306342ca5ebcd8ca5b6aeb8b6a8c67e3`

The corrected canonical-run scope repair is therefore verified for the
run-memory contract: all current run-memory rows are selected and replayed
without row omissions or overlapping-field differences.

Whole-computer replay is **not** eligible. The remaining differences are:

- `dolt:texture:content_root`
- `dolt:texture:table:texture_document_aliases`

`texture_document_aliases` remains non-empty and `EmptyUntilSupported` without a
reducer. No checkpoint, restore, candidate, bundle, self-development retry,
self-promote, qualified-consensus transition, or mail send is authorized by
this receipt.

## Decision and next action

Accept this as a narrow deployed acceptance of canonical run-memory residue
selection and replay, not as whole-computer restore acceptance. Preserve the
new residue event and all lifecycle receipts. Keep
`texture_document_aliases` `EmptyUntilSupported`; any reducer or import design
requires its own docs-first authorization and proof. Do not bind a checkpoint,
rematerialize, restore, retry self-development, self-promote, invoke qualified
consensus, or send mail while replay eligibility is false.

**Mutation class:** red. Protected surfaces: retained-computer lifecycle,
event-chain append, replay projection, run-memory scope, and replay eligibility.

**Rollback:** platform rollback is `git revert 7bdc0a7ceb063d29542f2aa90560439db97d8aee`.
The corrected residue snapshot is an event-chain append and must not be deleted
or SQL-reversed. Product-state recovery remains acceptance-fenced.

**Heresy delta:** discovered — the corrected source boundary restores exact
run-memory completeness (`1083/1083`) while the computer remains ineligible on
an independent unsupported table; introduced — none; repaired — the canonical
run-memory scope omission.

**Conjecture delta:** the canonical-run provenance diagnosis is confirmed in the
same-computer product path. The prior omission hypothesis is closed for current
run-memory rows. The whole-computer restore hypothesis remains unproven because
`texture_document_aliases` still lacks an authorized reducer-backed projection.
