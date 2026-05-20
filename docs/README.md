# Choir Documentation Index

Last reviewed: 2026-05-20

This directory contains canonical architecture docs, active MissionGradient
missions, proof artifacts, and a small number of historical pointers. Do not
treat every file here as equally current.

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
- `docs/docs-state-report-2026-05-14.md` - current documentation audit and cleanup recommendation matrix.
- `docs/current-architecture.md` - current product/runtime architecture.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/implementation-scope.md` - near-term implementation scope.
- `docs/north-star.md` - long-range product direction.
- `docs/mission-public-desktop-auth-on-mutation-v0.md` - active next
  MissionGradient for public desktop access and auth-on-mutation.
- `docs/mission-pressure-aware-computer-lifecycle-v0.md` - proposed
  MissionGradient for VM/computer warmness, pressure-aware reclaim, recovery,
  security, performance, and boot UX.
- `docs/mission-lifecycle-observability-load-dynamics-v0.md` - proposed
  MissionGradient for adaptive computer lifecycle control: primary-computer
  keepalive, future 24/7 uptime policy, correlated instrumentation,
  progressive/stochastic load, and UX-driven performance optimization.
- `docs/mission-real-media-apps-ux-sweep-v0.md` - proposed MissionGradient for
  real PDF/EPUB readers, stronger Image/Audio/Video apps, Podcast regression,
  and shell/Trace/VText/Files/launcher UX proof.
- `docs/mission-computer-recovery-system-monitor-v0.md` - completed/proposed
  MissionGradient for turning desktop restore recovery into product-grade
  user-computer observability, safe recovery controls, and a first-class
  Compute Monitor app.
- `docs/mission-mobile-real-desktop-overview-v0.md` - completed
  MissionGradient for preserving the same overlapping floating-window desktop
  on mobile and desktop, introducing Shelf/Desk vocabulary and Desktop
  Overview v0.
- `docs/mission-desktop-overview-heavy-session-v0.md` - completed
  MissionGradient for making Desktop Overview spatially useful under heavy
  restored sessions with bounded suspension/recovery controls.
- `docs/mission-desktop-overview-live-spatial-previews-v0.md` - completed
  MissionGradient for turning Desktop Overview into a bounded live spatial
  preview/control surface without WebGPU, fake thumbnails, duplicate app
  mounts, preview privacy leaks, or memory-regression shortcuts.
- `docs/mission-desktop-overview-app-owned-spatial-previews-v0.md` - proposed
  MissionGradient for app-owned Overview preview descriptors, app-specific
  redacted/summary cards, premium spatial polish, and returning-session-style
  staging proof without fake thumbnails or duplicated app mounts.
- `docs/mission-choir-in-choir-controller-v0.md` - staging-first MissionGradient for promotion-level and continuation-level self-development acceptance.
- `docs/mission-sweep-substrate-v0.md` - proposed staging-first MissionGradient for the first real sweep substrate: Codex bootstrap, Choir prompt-bar run, vsuper orchestration, worker/verifier cosuper channel iteration, candidate/export evidence, and UX sweep proof.
- `docs/mission-platform-dolt-publication-retrieval-citation-v0.md` -
  active/landing MissionGradient for the first platform Dolt SQL-server
  service, selected VText publication, public route, retrieval source/span
  manifests, and citation candidates/edges.

## Current Mission Family

- `docs/mission-public-desktop-auth-on-mutation-v0.md` - active next UX/access
  model mission.
- `docs/mission-pressure-aware-computer-lifecycle-v0.md` - proposed next
  reliability/performance/UX mission for warm returning-account computers and
  pressure-aware reclaim.
- `docs/mission-lifecycle-observability-load-dynamics-v0.md` - proposed next
  adaptive lifecycle mission for keeping primary computers warm under capacity,
  preparing an always-on tier, and measuring/improving behavior under real
  product-path dynamics.
- `docs/mission-real-media-apps-ux-sweep-v0.md` - proposed next real readers
  and media-app UX sweep.
- `docs/mission-computer-recovery-system-monitor-v0.md` - completed/proposed
  hardening pass for Compute Monitor, desktop restore recovery, lazy app
  hydration, and safe computer recovery controls.
- `docs/mission-mobile-real-desktop-overview-v0.md` - completed mobile real
  desktop and Desktop Overview v0 proof.
- `docs/mission-desktop-overview-heavy-session-v0.md` - completed shell
  mission for heavy-session Desktop Overview, bounded suspension, recovery,
  and Compute Monitor handoff.
- `docs/mission-desktop-overview-live-spatial-previews-v0.md` - completed
  shell mission for live spatial Overview previews, bounded preview policy,
  suspended/redacted cards, and premium motion without memory/privacy regressions.
- `docs/mission-desktop-overview-app-owned-spatial-previews-v0.md` - proposed
  next shell mission for replacing hard-coded preview heuristics with app-owned
  descriptors and proving a quieter, more spatial Overview under real-session
  clutter.
- `docs/mission-promotion-substrate-preflight-hard-cutover-v0.md` - completed
  preflight hard cutover that made AppChangePackage -> adoption -> recipient
  build -> promote/rollback the current patch movement path.
- `docs/mission-source-lineage-promotion-control-plane-v0.md` - source-lineage
  and divergent-computer promotion design for moving app changes between user
  computers and the platform computer.
- `docs/mission-choir-in-choir-controller-v0.md` - active/stopped controller mission and latest invariant-level blocker.
- `docs/mission-sweep-substrate-v0.md` - proposed next substrate mission for making sweeps real through outer Codex orchestration and inner Choir staging proof.
- `docs/mission-platform-dolt-publication-retrieval-citation-v0.md` -
  active/landing substrate mission for platform Dolt publication/retrieval/
  citation.
- `docs/mission-run-acceptance-verification-v0.md` - historical run acceptance
  mission and evidence record.
- `docs/mission-embedded-dolt-runtime-migration-v0.md` - completed embedded
  Dolt runtime/control migration with staging acceptance and Node B disk
  evidence.
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

- `docs/PROJECT-STATE.md` is now a short historical pointer. The old snapshot was removed because it contained stale operational/provider/credential and continuation instructions.
- Old Mission 1/2/3/5/6/7 milestone docs were deleted after live signal was folded into `docs/project-goals.md`, `docs/glossary.md`, `docs/adr-dolt-as-canonical-state.md`, and the canonical architecture docs. Use git history for the removed originals.
- Top-level `TODOS.md`, `PROJECT-GOALS.md`, and `PROJECT-GLOSSARY.md` were removed after extraction.
- `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` and `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` are useful audits from an earlier API cutover.
- `docs/choir-origin-main-change-report-2026-05-10.md` is a historical change report.

Do not delete historical docs during ordinary feature work. Label, index, or update them. Delete only when a cleanup mission explicitly proves they are junk or duplicated.

## Active Cleanup Notes

The 2026-05-14 cleanup applied the docs-state report's recommended root-doc and
old-mission cleanup. Remaining cleanup work is intentionally narrower:

- gradually fold durable lessons from dated proof/evidence files into canonical
  architecture/invariant docs when they become current;
- keep `docs/deferred-reliability-migrations-2026-05-14.md` as historical
  context for the later sandbox-to-computer hard rename; the runtime/control
  SQLite-to-Dolt cutover itself is now complete and recorded in
  `docs/mission-embedded-dolt-runtime-migration-v0.md`;
- leave proof docs as evidence artifacts unless a cleanup mission explicitly
  indexes, extracts, and deletes them;
- keep `docs/README.md`, `README.md`, and `AGENTS.md` current when missions
  promote new operating rules.
