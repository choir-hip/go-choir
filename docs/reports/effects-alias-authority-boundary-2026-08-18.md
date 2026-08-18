# Effects Alias Authority Boundary - Progress Report

**Report date:** 2026-08-18
**Scope:** `computer-03335285269bdba4f94377e56879f9e6`, operation `selfdev-b090bcd72d300fed17cb3f5a142f8595`
**Authority:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Status:** blocked on owner architecture authority; effects remain OFF
**Mutation class:** red architecture boundary, documentation-only

## Executive verdict

The canonical run-memory repair is narrow and accepted: at replay sequence `3342`, the retained computer has matching event heads and exact `1083/1083` run-memory equivalence. Whole-computer replay is still `not_equivalent` and ineligible because `texture_document_aliases` is non-empty and classified `EmptyUntilSupported`; `content_root` also differs and its causality is not yet proven.

This pass documents the alias-authority problem and stops. It does not add a reducer, projection, residue import, manifest reclassification, checkpoint, restore, retry, promotion, or mail send. No candidate or bundle exists, no effect is armed, and no live mail was sent.

The next action is dated owner ratification of alias authority and scope. Consensus selected the honest blocked boundary, but consensus cannot substitute for owner architecture authority.

## Problem documented

`texture_document_aliases` is durable state written outside the event-derived projection. The table is keyed by `(owner_id, source_path)` and has no `computer_id`, while replay eligibility evaluates it as a computer-local restore surface. The repository has no alias projection operation, reducer case, or residue-import path. An object-graph `document_alias` kind and `ogEdgeDocAlias` constant exist, but they are latent connection opportunities, not a settled authority.

The exact live alias row count, row provenance, cross-computer intent, dangling mappings, and complete indirect writer graph remain unpaid. `content_root` differs alongside the alias table; whether that difference is solely downstream of aliases remains unverified.

This is an authority and scope problem, not a reason to empty SQL, append an unproven snapshot, or weaken the replay gate.

## Evidence

The owner-authorized deployed replay receipt records:

- staging source identity: `7bdc0a7ceb063d29542f2aa90560439db97d8aee`;
- CI run: `32186509122`;
- Node B deploy job: `95878530936`;
- retained computer refresh receipt: `01a016dc-161d-7b44-9611-a1bd674e9e7e`;
- retained computer epoch: `316`, mode `propose_only`, generation `1`;
- replay sequence: `3342`;
- run-memory rows: `1083` live, `1083` replay, zero live-only, zero replay-only, zero differing;
- whole-computer result: `not_equivalent`, eligibility `false`;
- no candidate or bundle, effects OFF, no mail.

Durable evidence:

- `docs/evidence/effects-red-replay-run-memory-scope-repair-2026-08-18.md`
- `docs/problems/effects-red-replay-texture-alias-authority-2026-08-18.md`
- `internal/agentcore/replay_eligibility.go`
- `internal/store/texture.go`
- `internal/computerevent/projection_batch.go`
- `internal/store/project.go`
- `internal/store/residue_import.go`

## Boundary decision

A divergent architecture panel considered distinct authority families. The subsequent five-agent convergent review selected the following boundary:

- keep `texture_document_aliases` `EmptyUntilSupported`;
- keep replay eligibility, checkpointability, restore, retry, promotion, and effects closed;
- select no relational, object-graph, or derived implementation family;
- obtain owner ratification before any implementation design;
- run fresh consensus on the frozen owner-ratified design before runtime or event-chain mutation.

The prior panel dissent favored preselecting a relational projection because the current SQL table is convenient. That is not adopted: existing storage is evidence of an implementation opportunity, not proof of authority.

## Durable changes

The following tracked surfaces now agree:

1. The Definition's `now` block is `blocked_incomplete`, names the alias authority question, records the sequence-3342 reconciliation, and starts `next_action` with owner ratification across eight decision areas.
2. The Definition's top-level `receipts` list contains the alias problem receipt as a distinct item with durable proof references.
3. The mission graph keeps the executable entrypoint `working` because the repository's R5 rule requires that shape; its note explicitly records the Definition's `blocked_incomplete` alias state. The graph status is structural and does not authorize effects.
4. The document-authority manifest witnesses the durable alias problem receipt.
5. The new problem receipt records rollback, heresy delta, conjecture delta, forbidden shortcuts, and the separate read-only CLI diagnostic opportunity.

## Required owner decisions

Before an `Implement` slice, the owner must ratify:

1. authority family;
2. owner-global versus computer-scoped identity;
3. canonical path and privacy law;
4. alias lifecycle and delete semantics;
5. event identity, idempotency, projector, rollback, and fate-sharing law;
6. exact residue-import scope and handling of dangling or foreign rows;
7. complete writer coverage, cutover tests, restart durability, and deployed reclassification proof;
8. whether current rows are product state, legacy residue, or a deprecated cache.

Until then, no runtime alias work is authorized.

## Verification

- `./scripts/doccheck --mode full --json /tmp/choir-doccheck-alias-2.json --report /tmp/choir-doccheck-alias-2.md` completed with `0 warnings` and `41 info findings` across `407 docs`.
- `git diff --check -- docs/definitions/choir-supervised-self-development-effects-2026-08-11.md docs/problems/effects-red-replay-texture-alias-authority-2026-08-18.md docs/mission-graph.yaml docs/doc-authority-manifest.yaml` passed with no output.
- Post-correction review: four substantive agents accepted with no blocker; a fifth runner produced no verdict because read-only approval was unavailable. The substantive reviews verified receipt placement, `now` state, graph R5 overlay, manifest witness, and source claims.
- Worktree classification: five intentional documentation paths only - three modified tracked docs, the new problem receipt, and this local report. No Go/runtime/generated scratch path is present.
- Docs-only landing: commit `fb4bcb0378a17a07f8831011b2722aee22d68ea6` is on `origin/main`; CI run `32194076180` and Docs Truth Check job `95894423488` succeeded. Staging deploy was skipped because no runtime path changed.

This is documentation proof only. It does not prove whole-computer replay, checkpointability, restore, self-development retry, qualified consensus, or any external effect.

## Rollback and learning

Rollback is a documentation revert. No product state, VM-local projection, or event-chain state changed in this boundary.

**Heresy delta:** discovered - owner/path-keyed alias state is independently unsupported for whole-computer replay and its authority and scope are unresolved; introduced - none; repaired - none.

**Conjecture delta:** the narrow run-memory provenance repair is confirmed at sequence `3342`; the whole-computer restore conjecture remains unproven until alias authority is settled and a reconstructible implementation is independently verified.

**Next realism axis:** owner-ratified alias identity and scope, then restart-durable projection/import fidelity. Read-only diagnostics do not authorize implementation.
