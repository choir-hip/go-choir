# Choir Documentation Index

Last reviewed: 2026-05-13

This directory contains canonical architecture docs, active MissionGradient missions, proof artifacts, and older milestone notes. Do not treat every file here as equally current.

## Canonical Current Docs

- `../README.md` - operational entrypoint for humans and agents.
- `../AGENTS.md` - repo-level agent operating contract.
- `docs/current-architecture.md` - current product/runtime architecture.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/implementation-scope.md` - near-term implementation scope.
- `docs/north-star.md` - long-range product direction.
- `docs/mission-run-acceptance-verification-v0.md` - active staging-first mission for docs cleanup and durable run acceptance verification.

## Current Mission Family

- `docs/mission-run-acceptance-verification-v0.md` - current mission.
- `docs/mission-choir-grand-deformation-v0.md` - broad Choir-in-Choir deformation sketch.
- `docs/mission-choir-in-choir-deformation-v0.md` - earlier deformation mission.
- `docs/mission-choir-in-choir-controller-v0.md` - controller/run-memory continuation direction.
- `docs/mission-candidate-world-promotion-v0.md` - candidate-world promotion mission.
- `docs/mission-promotion-queue-v0.md` - promotion queue product bridge mission.
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

- `docs/PROJECT-STATE.md` is a historical snapshot. It is useful for origin story and old milestone context, but it contains stale status, old mission numbering, and references to earlier assumptions.
- `docs/mission-1-deploy-pipeline.md` through `docs/mission-7-cogent-integration.md` are historical milestone docs unless explicitly reactivated.
- `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` and `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` are useful audits from an earlier API cutover.
- `docs/choir-origin-main-change-report-2026-05-10.md` is a historical change report.

Do not delete historical docs during ordinary feature work. Label, index, or update them. Delete only when a cleanup mission explicitly proves they are junk or duplicated.

## Active Cleanup Notes

This pass updated the repository entrypoints and added a current docs index rather than rewriting every proof artifact. The unresolved cleanup work is to gradually fold durable lessons from dated proof files into canonical architecture/invariant docs, then mark the source proof as historical.

The next documentation cleanup should focus on:

- bringing `docs/current-architecture.md` and `docs/runtime-invariants.md` up to the run-geometry vocabulary: candidate worlds, vsuper, verifier contracts, promotion, and compaction/run memory;
- moving obsolete milestone-status language out of `docs/PROJECT-STATE.md` or splitting it into an archive;
- adding short ADRs once the run acceptance verifier and staging product proof have landed.
