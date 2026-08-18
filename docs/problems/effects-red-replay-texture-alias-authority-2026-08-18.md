# Texture Document Aliases Lack a Reconstructible Replay Authority

**Status:** blocked on owner architecture authority; effects remain OFF

**Observed:** 2026-08-18

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red architecture problem; this receipt is documentation-only

## Problem

The retained staging computer cannot pass whole-computer replay eligibility because `texture_document_aliases` is non-empty durable state written outside the event-derived projection. The table is keyed by `(owner_id, source_path)` and has no `computer_id`, while replay eligibility evaluates it as a computer-local restore surface. No single reconstructible authority has been settled for this state.

The narrow canonical-run repair is not the problem: at replay sequence `3342`, live and replay event heads match and all `1083` `run_memory_entries` rows match exactly. Whole-computer replay remains `not_equivalent` with differences at:

- `dolt:texture:table:texture_document_aliases` — non-empty and classified `EmptyUntilSupported`;
- `dolt:texture:content_root` — also differs; whether this is solely a consequence of the alias table remains unverified.

This is an authority and scope problem, not permission to make the table empty, change the manifest class, or append an unproven residue snapshot.

## Evidence

The owner-authorized deployed replay receipt records:

- computer: `computer-03335285269bdba4f94377e56879f9e6`;
- sequence: `3342`;
- canonical event head: `0a2362bf34b5e1d06559baea407fb564751390b45f10cfeba57779265e188980`;
- run-memory counts: `1083` live, `1083` replay, `0` live-only, `0` replay-only, `0` differing;
- result: `not_equivalent`;
- no candidate or bundle, no armed effects, and no mail.

Source and gate evidence:

- `internal/agentcore/replay_eligibility.go` classifies `texture_document_aliases` as `ReplayEmptyUntilSupported`.
- `internal/store/texture.go` creates the table and directly reads, upserts, and deletes aliases. The schema has no `computer_id`.
- `internal/computerevent/projection_batch.go` has no alias projection operation kind.
- `internal/store/project.go` has no alias reducer case.
- `internal/store/residue_import.go` has no alias snapshot/import path.
- `internal/objectgraph/registry.go` registers a `document_alias` kind and `internal/store/graph_store.go` names `ogEdgeDocAlias`, but no production writer was found for that latent edge. Its existence is a connection opportunity, not an authority decision.
- The deployed narrow replay receipt is `docs/evidence/effects-red-replay-run-memory-scope-repair-2026-08-18.md`.

The exact live alias row count, row provenance, cross-computer intent, dangling mappings, and complete indirect writer call graph were not established by this documentation-only boundary.

## Consensus disposition

A fresh convergent panel of five independent agents reviewed the source and prior divergent panel. Four selected the honest blocked boundary (A); one recommended a define-only receipt that preselected relational projection (B). The adopted result is A because consensus can supply engineering analysis but cannot supply owner architecture authority.

The panel agreed that a typed relational projection may be a credible later implementation if the owner keeps the SQL table as authority. That is not settled here. Object-graph and derived-view alternatives remain open.

## Required owner decisions before Implement

A dated owner statement must settle, at minimum:

1. **Authority family:** event-backed relational projection, object-graph alias objects/edges, or a derived view of already-replayable document state.
2. **Identity scope:** owner-global aliases versus computer-scoped restore state, including whether the current schema is intentionally owner-global.
3. **Path law:** canonical path/URI normalization, case sensitivity, uniqueness, privacy, and versioning.
4. **Lifecycle law:** upsert, remap, rename, archive, tombstone, and physical-delete semantics.
5. **Event law:** stable event identity, idempotency, projector version, rollback, and fate-sharing with the computer event chain.
6. **Residue law:** exact owner/computer-scoped import selection, timestamp fidelity, dangling and foreign-owner rows, and import rollback behavior.
7. **Cutover proof:** every production writer, restart durability, focused reducer/import tests, deployed replay row evidence, and the exact eligibility reclassification condition.
8. **Scope inventory:** whether the live rows are product state, legacy residue, or a deprecated cache. Deprecation is itself an owner architecture decision.

Until these decisions are ratified and recorded in the Definition, no implementation family is selected.

## Current boundary and forbidden shortcuts

Keep `texture_document_aliases` `EmptyUntilSupported` and keep the eligibility gate closed. Do not:

- add a reducer or projection writer;
- import aliases or append another residue snapshot;
- SQL-empty or SQL-reverse the table;
- change the airworthiness manifest alone;
- replace the retained workspace/computer;
- bind a checkpoint, restore, rematerialize, or retry self-development;
- self-promote, invoke qualified-consensus CAS, or send mail.

The exact run-memory result is a narrow repaired contract. It does not prove whole-computer restore, checkpointability, or effect authorization.

## Separate CLI opportunity

Keep CLI work outside the red alias-authority boundary. The highest-value no-SSH read-only improvement is an owner-scoped alias-residue diagnostic reachable through `choir computer replay-completeness` or a separate inspect command. It should report counts, privacy-safe key/digest summaries, provenance where available, and explicit `EmptyUntilSupported` reasons. Import and replay receipts should also expose selected scope, projector/version, row counts and digests, appended event sequence/head, per-table differences, and an unmistakable `eligibility unchanged` or `eligible` result.

## Disposition, rollback, and learning

**Disposition:** accepted as new problem documentation. The mission is blocked on owner architecture authority, not on a missing symptom patch.

**Rollback:** revert this documentation-only change; no product state, VM-local projection, or event-chain state was changed by this receipt.

**Heresy delta:** discovered — owner-keyed alias state is independently unsupported for whole-computer replay and its authority/scope is unresolved; introduced — none; repaired — none.

**Conjecture delta:** the narrow run-memory provenance repair is confirmed at sequence `3342`; the whole-computer restore conjecture remains unproven until alias authority is settled and a reconstructible implementation is independently verified.

**Next safe action:** obtain dated owner ratification of the decisions above, update the Definition's `next_action`, and run fresh consensus on the frozen design before entering any runtime or event-chain mutation.
