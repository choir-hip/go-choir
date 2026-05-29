# Choir Documentation Index

Last reviewed: 2026-05-29

This directory contains canonical architecture docs, active MissionGradient
missions, proof artifacts, and a small number of historical pointers. Do not
treat every file here as equally current.

The old documentation audit was pruned during the Campaign Compiler cleanup.
Current docs should point at `choir.news`, AppChangePackage/adoption source
movement, and Campaign Compiler as the next Choir-in-Choir benchmark.

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
- `docs/choir-agentic-depth-canonical.md` - canonical run-depth vocabulary for MissionGradient, MissionBag, Sweep, Leap, Fly, Cycle, and worker/verifier/orchestrator roles.
- `docs/missiongradient-method.md` - run-geometry method entrypoint for long-running agent work; canonical skill lives at `skills/mission-gradient/SKILL.md`.
- `docs/cognitive-transform-portfolio.md` - transform portfolio entrypoint for route-changing reframes; canonical skill lives at `skills/cognitive-transform-portfolio/SKILL.md`.
- `docs/computer-ontology.md` - canonical vocabulary for persistent user computers, ledger split, personal promotion, platform/public promotion, and update algebra.
- `docs/vm-priority-policy.md` - current and future VM/computer warmness,
  reclaim, always-on, and uptime-tier policy.
- `docs/platform-os-app-state.md` - current common platform/default computer
  state ledger for the OS substrate, desktop shell, app catalog, app boundaries,
  proof anchors, and known UX/app gaps. Keep it updated as platform app state
  changes; later divergent user computers should expose their own equivalent
  state records.
- `docs/project-goals.md` - current goal continuum and extracted live signal from older project/mission docs.
- `docs/glossary.md` - canonical vocabulary for current product/runtime terms.
- `docs/adr-dolt-as-canonical-state.md` - Dolt/SQLite state-boundary decision.
- `docs/platform-dolt-publication-retrieval-citation-research-2026-05-16.md`
  - research/design input for platform Dolt, publication, retrieval, citations,
  provenance, and the future citation economy.
- `docs/public-identity-and-custom-domains.md` - public handle, route, and
  custom domain roadmap.
- `docs/current-architecture.md` - current product/runtime architecture.
- `docs/frontend-app-building-api.md` - current frontend app registry, preview,
  theme, and shell contract.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/implementation-scope.md` - near-term implementation scope.
- `docs/north-star.md` - long-range product direction.
- `docs/mission-campaign-compiler-selfdev-v0.md` - current next
  Choir-in-Choir benchmark: Campaign Compiler as a Choir-native control layer
  over campaigns, mission geometry, work orders, evidence packets, cognitive
  transform invocations, candidate computers, promotion, and reentry.
- `docs/legacy-promotion-experiments-learnings.md` - consolidated lessons from
  pruned patchset-promotion experiments.
- `docs/mission-apps-and-changes-store-sweep-v0.md` - retained state for the
  Apps & Changes product path; historical portfolio inputs were pruned.

## Current Mission Family

- `docs/mission-campaign-compiler-selfdev-v0.md` is the primary current
  self-development mission surface.
- `docs/mission-choir-grand-deformation-v0.md` - broad Choir-in-Choir deformation sketch.
- `docs/mission-run-memory-v0.md` - run-memory/compaction mission.
- `docs/mission-web-surface-rationalization-v0.md` - Obscura/browser surface rationalization mission.

Read the current Campaign Compiler mission first. Older promotion-queue mission
docs have been pruned; use the consolidated learnings doc when that context is
needed.

## Proof And Evidence Artifacts

Files named `*-proof-*.md`, `*-dogfood-*.md`, `*-blocker-*.md`, and
`*-next-frontier-*.md` are run evidence, diagnostics, or next-frontier notes.
They are not automatically current operating instructions. Several old
promotion-queue proof docs were deleted after their lessons were consolidated in
`docs/legacy-promotion-experiments-learnings.md`.

When proof docs contradict `README.md`, `AGENTS.md`, `current-architecture.md`, or `runtime-invariants.md`, treat the contradiction as stale evidence unless a newer mission explicitly promotes it.

## Historical Or Stale Docs

- `docs/PROJECT-STATE.md` is now a short historical pointer. The old snapshot was removed because it contained stale operational/provider/credential and continuation instructions.
- Old Mission 1/2/3/5/6/7 milestone docs were deleted after live signal was folded into `docs/project-goals.md`, `docs/glossary.md`, `docs/adr-dolt-as-canonical-state.md`, and the canonical architecture docs. Use git history for the removed originals.
- Top-level `TODOS.md`, `PROJECT-GOALS.md`, and `PROJECT-GLOSSARY.md` were removed after extraction.
- `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` and `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` are useful audits from an earlier API cutover.

Do not delete historical docs during ordinary feature work. Label, index, or update them. Delete only when a cleanup mission explicitly proves they are junk or duplicated.

## Active Cleanup Notes

The 2026-05-14 cleanup applied the docs-state report's recommended root-doc and
old-mission cleanup. Remaining cleanup work is intentionally narrower:

- gradually fold durable lessons from dated proof/evidence files into canonical
  architecture/invariant docs when they become current;
- old backend-browser proof shards were consolidated into
  `docs/backend-browser-substrate-learnings.md`;
- keep `docs/deferred-reliability-migrations-2026-05-14.md` as historical
  context for the later sandbox-to-computer hard rename; the runtime/control
  SQLite-to-Dolt cutover itself is now complete and reflected in
  `docs/adr-dolt-as-canonical-state.md`;
- leave proof docs as evidence artifacts unless a cleanup mission explicitly
  indexes, extracts, and deletes them;
- keep `docs/README.md`, `README.md`, and `AGENTS.md` current when missions
  promote new operating rules.
