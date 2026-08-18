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
