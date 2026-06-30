# Parallax: Clear the Docs Checker Warnings

## Status

Open. Not yet started.

## Mission conjecture

If we classify and update the 975 doccheck warnings so that current docs use current vocabulary and historical docs are marked as evidence, then the documentation graph will be consistent with the object-graph refactor and the docs checker will pass.

## Deeper goal

Documentation is the current truth surface of the system. The 975 warnings are entropy: retired vocabulary still appearing in current claims. Clearing them is a key gate before the object-graph refactor can be considered coherent. The deeper goal is to make the docs a reliable map of the system, not a graveyard of outdated terms.

## Witness / spec

Deliver a docs-only commit that reduces the doccheck warnings to zero (or to a documented, intentional residual).

- Update `README.md`, `AGENTS.md`, and `docs/choir-doctrine.md` to use current vocabulary: object graph, texture_doc, source_entity, conductor, trajectory supervisor, mutation transaction, appagent, functor, transclusion.
- Mark historical mission docs as `is_evidence` or add historical annotations so H1/H5 retired-vocabulary warnings stop firing.
- Add evidence docs to `docs/mission-graph.yaml` so R3 "not reachable from current roots" warnings resolve.
- Fix the remaining H3/H4 doctrinal warnings by updating claims to the object-graph model.
- Run `nix develop -c go run ./cmd/doccheck` and verify zero warnings.

## Invariants / qualities / domain ramp

- Do not delete historical mission docs. Demote them to evidence.
- Do not weaken the docs checker rules to make warnings disappear.
- Do not introduce new current claims with retired vocabulary.
- Docs-only change: no code or runtime behavior change.
- Keep the diff reviewable; do not change unrelated prose.

## Authority / bounds

- Green/yellow mutation class: docs and prompt text only.
- No platform behavior change.
- No staging acceptance required; docs-only CI path is sufficient.
- Branch: `docs/clear-checker-warnings`.
- Worktree: `docs-checker`.

## Bridge conjecture + sub-conjectures

- Main conjecture: clearing the warnings proves the conceptual refactor has been internalized at the documentation level.
- Sub-conjecture 1: H1 warnings are caused by current docs using retired vocabulary; updating them will remove most warnings.
- Sub-conjecture 2: H5 warnings are caused by Texture predecessor names in non-historical contexts; marking them as historical will remove them.
- Sub-conjecture 3: R3 warnings are caused by evidence docs not being linked from current roots; adding them to the mission graph will remove them.

## Ledger / move log

- Move 0: Read `cmd/doccheck/main.go` to understand warning rules.
- Move 1: Run doccheck and capture the warning list.
- Move 2: Categorize warnings by rule and by doc.
- Move 3: Update current docs.
- Move 4: Mark historical docs as evidence.
- Move 5: Update mission graph.
- Move 6: Re-run doccheck until clean.
- Move 7: Commit, push, create PR/merge.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/design-attention-unifying-layer-2026-06-23.md` and the doccheck report from 2026-06-23.
- Successor link: this work enables the integrative object-graph migrations by providing a coherent vocabulary.

## Learning state

- Retained: the doccheck warning taxonomy (H1, H3, H4, H5, R3, R5).
- Promoted outward: the classification of docs as current / evidence / historical.

## Settlement

Done when `go run ./cmd/doccheck` reports zero warnings and the docs commit is on `origin/main`.
