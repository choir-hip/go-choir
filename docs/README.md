# Choir Documentation Index

Last reviewed: 2026-05-14

This directory contains canonical architecture docs, active MissionGradient missions, proof artifacts, and older milestone notes. Do not treat every file here as equally current.

For the current documentation audit and cleanup recommendations, read `docs/docs-state-report-2026-05-14.md`.

## Documentation State Taxonomy

Use these buckets when reading or editing docs:

- **Canonical current docs** define present operating context and implementation invariants. They override older proof, mission, and project-state files.
- **Current mission docs** define active or recently stopped MissionGradient work. They are runnable/inspectable mission context, not global architecture unless promoted into canonical docs.
- **Evidence artifacts** preserve proof, dogfood, blocker, or next-frontier findings from specific runs. Keep them as evidence, but do not treat them as current instructions when they contradict canonical docs.
- **Historical signal** may contain useful design history or old constraints, but must be read through the current architecture.
- **Stale/dangerous docs** contain outdated operational instructions, provider/credential references, or old continuation flows. Extract any live signal, then replace or delete them.

## Canonical Current Docs

- `../README.md` - operational entrypoint for humans and agents.
- `../AGENTS.md` - repo-level agent operating contract.
- `docs/mission-geometry.md` - high-level mission geometry: Choir as statistical/symbolic/evolutionary learner and automatic computer -> newspaper -> radio -> capital vector.
- `docs/docs-state-report-2026-05-14.md` - current documentation audit and cleanup recommendation matrix.
- `docs/current-architecture.md` - current product/runtime architecture.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/implementation-scope.md` - near-term implementation scope.
- `docs/north-star.md` - long-range product direction.
- `docs/mission-choir-in-choir-controller-v0.md` - active next staging-first MissionGradient for promotion-level and continuation-level self-development acceptance.

## Current Mission Family

- `docs/mission-choir-in-choir-controller-v0.md` - active/stopped controller mission and latest invariant-level blocker.
- `docs/mission-run-acceptance-verification-v0.md` - completed export-level acceptance mission and evidence record.
- `docs/mission-choir-grand-deformation-v0.md` - broad Choir-in-Choir deformation sketch.
- `docs/mission-choir-in-choir-deformation-v0.md` - earlier deformation mission.
- `docs/mission-candidate-world-promotion-v0.md` - candidate-world promotion mission.
- `docs/mission-promotion-queue-v0.md` - promotion queue product bridge mission.
- `docs/mission-run-memory-v0.md` - run-memory/compaction mission.
- `docs/mission-web-surface-rationalization-v0.md` - Obscura/browser surface rationalization mission.

Read the current mission first, then use the older mission docs as background.

## Proof And Evidence Artifacts

Files named `*-proof-2026-05-13.md`, `*-dogfood-2026-05-13.md`, `*-blocker-2026-05-13.md`, and `*-next-frontier-2026-05-13.md` are run evidence, diagnostics, or next-frontier notes. They are valuable history but are not automatically current operating instructions.

High-value recent proofs include:

- `docs/live-playwright-worker-dogfood-proof-2026-05-13.md`
- `docs/prompt-product-path-worker-promotion-proof-2026-05-13.md`
- `docs/promotion-queue-product-bridge-2026-05-13.md`
- `docs/run-control-memory-synthesis-proof-2026-05-13.md`
- `docs/context-limit-recovery-proof-2026-05-13.md`
- `docs/web-surface-rationalization-proof-2026-05-13.md`

When proof docs contradict `README.md`, `AGENTS.md`, `current-architecture.md`, or `runtime-invariants.md`, treat the contradiction as stale evidence unless a newer mission explicitly promotes it.

## Historical Or Stale Docs

- `docs/PROJECT-STATE.md` is a stale/dangerous historical snapshot. It contains old operational/provider/credential references and should be replaced by a short historical pointer or deleted after live signal is extracted.
- `docs/mission-1-deploy-pipeline.md` through `docs/mission-7-cogent-integration.md` are historical milestone docs unless explicitly reactivated. Mission 5/6/7 are likely delete-after-extraction candidates.
- Top-level `TODOS.md`, `PROJECT-GOALS.md`, and `PROJECT-GLOSSARY.md` should not remain root-level long-term. See `docs/docs-state-report-2026-05-14.md` for recommended extraction/move targets.
- `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` and `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` are useful audits from an earlier API cutover.
- `docs/choir-origin-main-change-report-2026-05-10.md` is a historical change report.

Do not delete historical docs during ordinary feature work. Label, index, or update them. Delete only when a cleanup mission explicitly proves they are junk or duplicated.

## Active Cleanup Notes

This pass updated the repository entrypoints and added a current docs index rather than rewriting every proof artifact. The unresolved cleanup work is to gradually fold durable lessons from dated proof files into canonical architecture/invariant docs, then mark the source proof as historical.

The next documentation cleanup should focus on:

- moving/updating `PROJECT-GLOSSARY.md` into `docs/glossary.md`;
- promoting `TODOS.md`'s SQLite/Dolt note into an ADR or runtime invariant, then deleting the root TODO;
- extracting live content from `PROJECT-GOALS.md`, then deleting or moving it;
- replacing or deleting `docs/PROJECT-STATE.md`;
- deleting old Mission 1/2/3/5/6/7 docs only after the docs-state report's extraction targets are handled;
- gradually folding durable lessons from dated proof files into canonical architecture/invariant docs, then leaving the proof docs as evidence artifacts.
