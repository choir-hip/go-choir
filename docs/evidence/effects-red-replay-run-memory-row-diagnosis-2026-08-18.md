# Effects replay run-memory row diagnosis — 2026-08-18

**Boundary:** diagnose. Not repair, replay acceptance, checkpoint, restore,
retry, promotion, qualified consensus, or effect authorization. Effects remain
OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Deployed diagnostic landing

The owner-authorized replay diagnostic was implemented in source commit
`37aff2e2b1ff27389b93b067940a9a799d75996a` (`feat: diagnose run-memory replay
drift`). It adds a content-safe row comparison to the replay-completeness
report: row counts, identity overlap, row digests, and differing field names;
provider-facing message, summary, and details content are never returned.

Landing evidence:

- CI: <https://github.com/choir-hip/go-choir/actions/runs/32177637011>
  (the first race shard attempt timed out in the pre-existing
  `internal/server.TestHealthHandlerIncludesAddrAfterStart`; the failed shard
  was rerun and the workflow completed successfully).
- Node B deploy job: `95850968977`.
- Staging `/health`: proxy `deployed_commit=37aff2e2b1ff27389b93b067940a9a799d75996a`,
  deployed `2026-08-18T20:12:36Z`.
- Focused local proof:

```text
go test ./internal/store ./internal/agentcore -run 'TestRunMemoryEntryFingerprintsHashRowsWithoutContent|TestCompareReplayRunMemoryClassifiesProjectionDrift|TestReplayCompletenessUsesDisposableProjectionWithoutMutatingLiveStore' -count=1
ok   github.com/yusefmosiah/go-choir/internal/store
ok   github.com/yusefmosiah/go-choir/internal/agentcore

go test ./internal/agentcore -run '^TestReplayCompleteness' -count=1
ok   github.com/yusefmosiah/go-choir/internal/agentcore
```

## Same-computer deployed probe

The retained computer was refreshed through the owner-scoped product path so
its guest loaded the deployed diagnostic:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- refresh idempotency key:
  `effects-red-replay-run-memory-diagnostic-refresh-20260818`
- lifecycle receipt: `01a01685-08ec-7ed9-85c4-bf5b032cf365`
- realization epoch: `313 -> 314`
- lifecycle: `active -> active`
- mode: still `propose_only`; no effects were armed and no mail was sent

Owner-authorized replay-completeness was then run against the same computer:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --timeout 10m
```

The deployed report was captured at `2026-08-18T20:18:58.484061513Z` with
schema version `3`. Live and replay heads both reached sequence `3331` and
matched. The semantic result remained `not_equivalent`; the differences were:

- `dolt:texture:content_root`
- `dolt:texture:table:run_memory_entries`
- `dolt:texture:table:texture_document_aliases`

Eligibility remained false because `texture_document_aliases` is a non-empty
`EmptyUntilSupported` table. No checkpoint, restore, candidate, bundle,
self-development retry, self-promote, qualified-consensus transition, or mail
send is authorized by this receipt.

## New row-level fact

The diagnostic returned this content-safe comparison:

```json
{
  "live_count": 1083,
  "replay_count": 922,
  "live_only_count": 161,
  "replay_only_count": 0,
  "different_count": 0
}
```

This is stronger than the prior table-hash mismatch:

1. Every one of the `922` replayed rows overlaps a live row by `entry_id`.
2. None of those overlapping rows differs in any canonical field fingerprint.
3. Exactly `161` live rows have no corresponding replay row.
4. The authorized residue import appended `922` run-memory projection rows;
   the live table was not changed by SQL cleanup or event reversal.

Therefore the current mismatch is **omission**, not row normalization or
replay mutation of the imported rows. The exact lineage of the `161` omitted
rows (canonical run identity, sequence, and the prior projection batch that
should contain them) is still unresolved. The bounded sample currently exposes
only entry IDs and digests, so no claim is made yet about whether the omitted
rows are legacy/unscoped records or rows created after the residue snapshot.

`texture_document_aliases` remains a separate unsupported direct-write
surface; this receipt does not authorize its import or a reducer design.

## Decision and next action

Accept this as a new docs-first source diagnosis. Before another residue append
or any runtime repair, extend the owner-authorized diagnostic or use a focused
fixture to group the `161` live-only rows by run ID, owner, agent, and sequence,
and compare those groups with the canonical `run_memory_entry_recorded`
operation bodies and prior projection batches. The next probe must distinguish
unselected legacy rows from rows added after the sequence-3326 residue event.
Do not append another snapshot, SQL-empty tables, delete or reverse the
existing residue event, remove response bounds, bind a checkpoint,
rematerialize, restore, retry self-development, self-promote, invoke qualified
consensus, or send mail while eligibility is false.

**Mutation class:** red. Protected surfaces: retained-computer lifecycle,
event-chain replay, run-memory projection, and replay eligibility. The refresh
was an owner-authorized lifecycle actuation; the diagnostic was read-only and
appended no event.

**Rollback:** revert this documentation commit if necessary. The deployed
source rollback is `git revert 37aff2e2b1ff27389b93b067940a9a799d75996a`; no
product-state or event-chain rollback is authorized by this receipt. Preserve
the retained computer, existing event chain, and lifecycle receipt.

**Heresy delta:** discovered — the deployed corrected residue snapshot
replays an exact subset of the live run-memory table (`922/1083`) and omits
`161` rows; introduced — none; repaired — none.

**Conjecture delta:** the prior projection-body and normalization hypothesis is
rejected for the overlapping rows. The remaining source-lineage hypothesis is
open: the omitted rows may be outside the canonical run selection or may have
been created after the residue snapshot. Exact row provenance remains unpaid.
