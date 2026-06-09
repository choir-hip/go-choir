# Choir Documentation Index

Last reviewed: 2026-06-09

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
- `docs/wire-news-system-learning-saga-2026-06-09.md` - current learning record
  for the news/Wire ontology correction: Wire is platform-level in the
  Community Cloud, reusable in Private Clouds, and personalized in user
  computers.
- `docs/choir-strategy-overview-2026-06-09.md` - high-level strategy overview:
  own your AI cloud, own the learning, private clouds, Wire, VText, and radio.
- `docs/choir-deck-treatment-and-faq-2026-06-09.md` - current deck/FAQ treatment
  for export after the Wire terminology correction.
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
- `docs/source-publication-consolidation-2026-06-06.md` - cleanup ledger for
  the deleted platform Dolt/publication/retrieval/citation research report.
- `docs/public-identity-and-custom-domains.md` - public handle, route, and
  custom domain roadmap.
- `docs/current-architecture.md` - current product/runtime architecture,
  including the surviving service-topology signal from deleted older sketches.
- `docs/code-docs-reconciliation-2026-06-06.md` - code-to-core-docs review
  notes from the 2026-06-06 full-codebase reconciliation pass.
- `docs/intended-architecture-next-2026-06-06.md` - target architecture for
  the next week-plus of work; not current-state proof.
- `docs/frontend-app-building-api.md` - current frontend app registry, preview,
  theme, and shell contract.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/source-external-data-publication.md` - canonical contract for external
  data ingestion, source cleaning, VText source metadata, transclusion,
  publication policy, and export.
- `docs/news-system-current-state-and-improvements-2026-06-06.md` - current
  source/news code-state and code-review note. Use it for sourcecycled,
  source_search, VText source-service refs, News app gaps, and rough future
  directions; do not treat its improvement list as accepted mission scope.
- `docs/choir-wire-source-to-vtext-spec-2026-06-09.md` - current Wire
  requirements contract: Community Wire, Private Wire reuse, platform/user
  computer authority, VText ownership, source artifacts, and deletion of legacy
  graph/source-maxxing behavior.
- `docs/mission-wire-community-news-v0.md` - current MissionGradient for landing
  the public Community Wire news product on the corrected ontology.
- `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md` -
  historical product/architecture spec for the superseded Global Wire +
  StoryGraph/VText framing. Do not use it as current ontology.
- `docs/vtext-styleguide-system-research-2026-06-06.md` - research synthesis
  for VText-native `Style.vtext` support, client corpus ingestion, learned style
  memory, edit feedback, style review, and optional future fine-tuning.
- `docs/vtext-styleguide-sources-review-2026-06-06.md` - full source-by-source
  review of the styleguide/voice corpus with concise signal summaries for each URL.
- `docs/vtext-styleguide-source-theme-synthesis-2026-06-06.md` - theme synthesis
  across all styleguide and anti-slop sources with consensus, controversy, and
  outlier breakdown.
- `docs/implementation-scope.md` - near-term implementation scope.
- `docs/north-star.md` - long-range product direction.
- `docs/mission-campaign-compiler-selfdev-v0.md` - current next
  Choir-in-Choir benchmark: Campaign Compiler as a Choir-native control layer
  over campaigns, mission geometry, work orders, evidence packets, cognitive
  transform invocations, candidate computers, promotion, and reentry.
- `docs/legacy-promotion-experiments-learnings.md` - consolidated lessons from
  pruned patchset-promotion experiments.
- `docs/old-docs-review-2026-06-06.md` - cleanup ledger for old docs reviewed
  on 2026-06-06, including mined insights from deleted proof/checklist files.
- `docs/mid-age-docs-review-2026-06-06.md` - cleanup ledger for existing docs
  last committed from 2026-05-24 through 2026-06-02.
- `docs/architecture-consolidation-2026-06-06.md` - cleanup ledger for mining
  older architecture sketches into `docs/current-architecture.md`.
- `docs/mission-apps-and-changes-store-sweep-v0.md` - retained state for the
  Apps & Changes product path; historical portfolio inputs were pruned.

## Current Mission Family

- `docs/mission-platform-source-service-vtext-publication-campaign-v1.md` -
  active Source Service / VText source metadata / publication campaign. Its
  requirements contract is `docs/source-external-data-publication.md`.
- `docs/mission-campaign-compiler-selfdev-v0.md` is the primary current
  self-development mission surface.
- `docs/mission-choir-grand-deformation-v0.md` - broad Choir-in-Choir deformation sketch.
- `docs/mission-run-memory-v0.md` - run-memory/compaction mission.
- `docs/mission-web-surface-rationalization-v0.md` - Obscura/browser surface rationalization mission.
- `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md` -
  historical draft MissionGradient for the superseded Global Wire / Style.vtext
  collaborative StoryGraph trajectory. Do not use it as current ontology.

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

- `docs/PROJECT-STATE.md` was deleted during the 2026-06-06 cleanup because it
  had become only a pointer to newer docs.
- Old Mission 1/2/3/5/6/7 milestone docs were deleted after live signal was folded into `docs/project-goals.md`, `docs/glossary.md`, `docs/adr-dolt-as-canonical-state.md`, and the canonical architecture docs. Use git history for the removed originals.
- Top-level `TODOS.md`, `PROJECT-GOALS.md`, and `PROJECT-GLOSSARY.md` were removed after extraction.
- The old API/VText hard-cutover checklist was deleted during the 2026-06-06
  mid-age cleanup. Its durable review lessons were folded into
  `docs/old-docs-review-2026-06-06.md` and
  `docs/mid-age-docs-review-2026-06-06.md`.

Do not delete historical docs during ordinary feature work. Label, index, or update them. Delete only when a cleanup mission explicitly proves they are junk or duplicated.

## Active Cleanup Notes

The 2026-05-14 cleanup applied the docs-state report's recommended root-doc and
old-mission cleanup. Remaining cleanup work is intentionally narrower:

- gradually fold durable lessons from dated proof/evidence files into canonical
  architecture/invariant docs when they become current;
- use `docs/old-docs-review-2026-06-06.md` for the 2026-06-06 extraction of
  lessons from deleted proof/checklist/research snapshots;
- use `docs/mid-age-docs-review-2026-06-06.md` for the 2026-06-06 extraction
  of lessons from deleted 2026-05-24 through 2026-06-02 snapshots;
- use `docs/architecture-consolidation-2026-06-06.md` for the 2026-06-06
  extraction of service-topology lessons from deleted architecture sketches;
- use `docs/source-publication-consolidation-2026-06-06.md` for the 2026-06-06
  extraction of platform Dolt/publication/citation lessons from deleted
  research;
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
