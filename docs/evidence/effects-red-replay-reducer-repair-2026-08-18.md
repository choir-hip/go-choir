# Effects replay reducer repair checkpoint — 2026-08-18

**Boundary:** implement. Not checkpoint. Not restore. Not retry. No live effect.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red. The repair changes event projection, replay eligibility, and
protected checkpoint/restore gates. The retained computer is not mutated by this
checkpoint.

## Problem documented first

The retained staging computer `computer-03335285269bdba4f94377e56879f9e6` returned
HTTP 200 from owner-authorized replay completeness, but the result was
`not_equivalent` and `eligible=false`. Four non-empty behavior-bearing tables had
no reducer-backed replay authority:

- `run_memory_entries`
- `self_development_operations`
- `self_development_start_intents`
- `texture_agent_mutations`

The source diagnosis found an existing graph-backed run-memory adapter that was not
wired into serving. The self-development operation and Texture mutation stores had
no equivalent event-projection reducer/read path. The direct SQL rows therefore
remained a second state authority. Evidence: the parent Definition's
`effects-red-replay-source-diagnosis-2026-08-18` receipt and
`docs/evidence/effects-red-computer-surface-boot-2026-08-18.md`.

## Follow-up problem: legacy Texture residue omitted before sequence 3222

The deployed replay transport and route-deadline repairs exposed a semantic failure at
sequence `3222`: `Texture mutation disappeared`. Source inspection narrows the
causal path without mutating the retained computer:

- `internal/textureowner/coagent_injection.go` and `internal/agentcore/runtime_persistence.go`
  reserve an empty `computer_id` for pre-lifecycle-work-item Texture mutation rows;
  `internal/store/texture.go` documents that empty scope as legacy-only.
- `internal/store/residue_import.go:snapshotResidueRuntime` currently selects
  `texture_agent_mutations WHERE computer_id=?`, so an owner-authorized residue
  import for the named computer omits those legacy rows even when they remain in
  the live projection.
- A later guarded Texture transition can therefore be present on the canonical
  tape while its predecessor is absent in a fresh event replay. The reducer then
  correctly fails closed at the first guarded transition rather than inventing a
  prior state.

The public owner path exposes the sequence-3222 error but not the internal event
payload, so the event's empty scope is an evidence-backed causal inference rather
than a directly fetched payload claim. The focused repair must prove this lineage
with a replay fixture and then with a fresh same-computer staging read. The repair
boundary is narrow: include current-computer and empty legacy rows in future
owner-scoped residue imports, and add an explicit **replay-only** compatibility
seed for a missing empty-scope guarded mutation from its full snapshot. Live
finalization must retain the missing-row rejection and all expected-state/revision
guards.

**Follow-up problem delta:** discovered — the legacy Texture mutation scope is
excluded from the reducer residue import, which can strand a later canonical
transition; introduced — none claimed; repaired — none. Effects remain OFF; no
checkpoint, restore, retry, promotion, or live send is authorized.

## Replay-only legacy Texture repair implementation

The source repair was committed as `750a145fcdd9663517b776fde1ce9c83e5bd7f5b`
(`fix: repair legacy texture replay projection`) after the problem receipt above.
It makes the narrow migration boundary executable:

- owner-scoped residue import now includes the named computer and empty
  `computer_id` legacy Texture rows;
- `ComputerEventAppender` uses an explicit `ReplayBatchProjectionStore` seam
  only for disposable canonical reconstruction;
- replay projection may seed a missing empty-scope guarded Texture row from its
  full snapshot, while live `FinalizeBatch` still rejects every missing guarded
  predecessor and retains state/revision checks.

Regression coverage proves the residue query includes an owner-scoped legacy row,
strict live finalization rejects a missing legacy transition, replay finalization
seeds it, and the appender dispatches the replay-only finalizer. Local proof:

```text
go test ./internal/store -run 'TestImportResidueSnapshotIncludesOwnerLegacyTextureMutation|TestFinalizeBatchRejectsMissingLegacyTextureTransition|TestFinalizeReplayBatchSeedsMissingLegacyTextureTransition' -count=1
go test ./internal/store -count=1
go test ./internal/computerevent -count=1
go test ./internal/agentcore -run 'TestReplayCompleteness' -count=1
```

The source repair is pushed and awaits CI/deployment, same-computer refresh, and
fresh owner-authorized replay-completeness evidence. Effects remain OFF; no
checkpoint, restore, retry, promotion, or live send is authorized.

**Repair heresy delta:** discovered — legacy Texture rows were omitted from the
residue import and stranded a guarded transition; introduced — an explicit
replay-only compatibility seam and empty-scope seed, with strict live behavior
unchanged; repaired — source-level replay reconstruction for this documented
legacy lineage, pending staging proof.

## Decision and conjecture delta

The approved next action is a reducer-backed replacement through the existing
computer event-projection batch seam. This checkpoint authorizes implementation; it
does not claim implementation or replay acceptance.

- **Prior belief:** replay can be eligible only after all four table families are
  event- or receipt-derived; no safe production replacement was wired.
- **Implementation conjecture:** versioned typed row snapshots, one reducer per
  table family, and a co-moving owner-authorized residue import can make the event
  chain the sole semantic authority while preserving the current SQL projection as
  an audit/read model. Every production writer must use the bound projection tape.
- **Proof obligation:** after landing and staging deployment, an owner-authorized
  residue import followed by replay completeness must show equivalent live/replay
  observations, equal non-nil heads, and `eligible=true`. Source tests alone cannot
  prove that retained-computer result.

## Protected surfaces and forbidden actions

The repair may touch only the event projection batch schema, its reducers, the
production writer bindings, replay-manifest classification, focused tests, and the
existing owner-scoped residue-import route. It must preserve:

- event-chain sequence, head, receipt, and CAS invariants;
- Texture canonical-write authorization and expected-state/revision guards;
- self-development operation state and idempotency semantics;
- fail-closed checkpoint, restore, and effects eligibility gates;
- owner/computer scoping of residue import.

While eligibility is false, do not SQL-empty any table, bind a checkpoint,
rematerialize, restore, retry self-development, self-promote, enable effects,
CAS `qualified_consensus`, or send mail. The OwnerRecovery checkpoint and the
constructed freeze remain non-authoritative for promotion.

## Admissible evidence and rollback

Admissible implementation evidence is focused reducer, idempotency, replay, and
runtime-writer coverage plus a successful package build. Admissible acceptance
evidence is the deployed staging identity, owner-authorized residue-import receipt,
and a fresh replay-completeness artifact from the same computer. No local test or
source inspection may be promoted to staging acceptance.

Rollback is a source revert of the repair commit before any retained-computer
residue import. If a deployed repair is rejected, keep the existing computer
state and fail closed; do not delete rows or rewrite the event chain. A successful
residue import is itself a forward event transaction and requires its own receipt;
its event head is not removed by source rollback.

## Heresy delta

- **Discovered:** four behavior-bearing direct-write authorities were outside the
  replay reducer set; the existing run-memory graph adapter was unwired.
- **Introduced:** none by this documentation checkpoint.
- **Repaired:** none yet; source implementation and deployed proof remain pending.

**Status:** checkpoint recorded before the source repair commit. Effects remain OFF;
no candidate, bundle, checkpoint, restore, retry, or live send is authorized.
