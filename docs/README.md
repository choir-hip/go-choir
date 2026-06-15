# Choir Documentation Index

Last reviewed: 2026-06-13

This directory contains Choir Doctrine, operating contracts, active Parallax
paradocs, domain invariant docs, descriptive architecture docs, proof artifacts,
and historical pointers. Do not treat every file here as equally current.

The old documentation audit was pruned during the Campaign Compiler cleanup.
Current docs should point at `choir.news`, AppChangePackage/adoption source
movement, and the durable-actor mission portfolio as the active
rearchitecture spine.

## Documentation State Taxonomy

Use these buckets when reading or editing docs:

- **Canonical doctrine** defines the current optimization target, protected
  invariants, evidence semantics, heresy inventory, and supersession rule.
- **Operating contract** defines how agents work in this repo and inherits
  canonical doctrine.
- **Active mission portfolio / paradocs** define executable work. They inherit
  doctrine; they do not override it unless explicitly promoted.
- **Domain invariants** define durable rules for specific subsystems such as
  computers, VText, source, promotion, or runtime authority.
- **Current descriptive architecture** describes code/staging reality and
  labeled target hardening. It should be corrected when it conflicts with code,
  staging, or doctrine.
- **Current mission docs / paradocs** define active or recently stopped
  Parallax work. They are runnable/inspectable mission context, not global
  architecture unless promoted into canonical docs.
- **Evidence artifacts** preserve proof, dogfood, blocker, or next-frontier findings from specific runs. Keep them as evidence, but do not treat them as current instructions when they contradict canonical docs.
- **Historical signal** may contain useful design history or old constraints, but must be read through the current architecture.
- **Stale/dangerous docs** contain outdated operational instructions, provider/credential references, or old continuation flows. Extract any live signal, then replace or delete them.

Supersession rule: `docs/choir-doctrine.md` is the apex doctrine. `AGENTS.md`
is the operating contract. Mission docs and reviews are evidence unless their
learning has been promoted into doctrine or a domain invariant. Old
MissionGradient docs are historical for new work unless a current paradoc
explicitly promotes one as source form.

## Authority Layers

- **Canonical doctrine:** `docs/choir-doctrine.md`.
- **Operating contract:** `../AGENTS.md`.
- **Active mission portfolio:** `docs/mission-portfolio-2026-06-11.md` and the
  current mission paradoc.
- **Mission graph:** `docs/mission-graph.yaml` is the machine-readable mission
  DAG and mission-corpus index. It links active paradocs, historical
  mission-shaped docs, and dependency/status metadata; it is not a second
  mission ledger, and historical entries remain evidence unless a current
  paradoc promotes them.
- **Assertion register:** `docs/conjecture-assertion-ledger-2026-06.md` is the
  canonical home for supported assertions, invariant candidates, and open
  hyperthesis edges.
- **Heresy detector manifest:** `docs/heresy-detectors.md` defines detector
  families and baseline vocabulary. Counts are evidence, not ontology.
- **Domain invariants:** `docs/computer-ontology.md`,
  `docs/vtext-agentic-invariants-2026-06-13.md`,
  `docs/runtime-invariants.md`, `docs/source-external-data-publication.md`,
  and promotion/source-specific doctrine where explicitly current.
- **Current descriptive architecture:** `docs/current-architecture.md` and
  `docs/platform-os-app-state.md`.
- **Historical evidence:** dated proof, review, dogfood, MissionGradient, and
  superseded mission reports.
- **Superseded/dangerous-if-current:** docs that normalize retired root
  ontologies such as StoryGraph-as-root, personal writing/publishing system as
  root, Trace app, raw Terminal app, Browser-as-source-gathering, parent/child
  control, or continuation-level as target doctrine.
  Historical docs may still quote those terms as evidence, but they should
  label them as retired vocabulary rather than present-tense product ontology.

## Canonical Current Docs

- `docs/choir-doctrine.md` - apex doctrine and architecture control document.
- `../README.md` - operational entrypoint for humans and agents.
- `../AGENTS.md` - repo-level agent operating contract.
- `docs/conjecture-assertion-ledger-2026-06.md` - canonical conjecture and
  assertion register: supported assertions, invariant candidates, and open
  blind edges with receipts and invalidation triggers.
- `docs/heresy-detectors.md` - doctrine detector manifest and baseline
  vocabulary for heresy accounting across docs and code.
- `docs/mission-graph.yaml` - machine-readable mission DAG and mission-corpus
  index. Creating or materially re-scoping a paradoc should update this graph
  in the same pass.
- `docs/mission-docs-truth-system-v1.md` - active docs truth system mission:
  focal docs spine, mission DAG, assertion-register wiring, and code/docs
  heresy baseline.
- `docs/mission-geometry.md` - high-level mission geometry: Choir as statistical/symbolic/evolutionary learner and automatic computer -> newspaper -> radio -> capital vector.
- `docs/parallax-design-2026-06-11.md` - current mission-discipline design:
  Parallax conjecture circuits and paradocs for new broad work.
- `docs/mission-portfolio-2026-06-11.md` - current durable-actor
  rearchitecture mission portfolio and execution order.
- `docs/choir-agentic-depth-canonical.md` - canonical run-depth vocabulary for
  MissionGradient-era MissionBag, Sweep, Leap, Fly, Cycle, and
  worker/verifier/orchestrator roles; read through Parallax for new missions.
- `docs/missiongradient-method.md` - legacy run-geometry method. Use only as
  historical baseline/fallback for old mission documents; new broad work uses
  Parallax.
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
- `docs/choir-rearchitecture-durable-actors-2026-06-11.md` - target durable-actor
  rearchitecture conjecture (trajectories/work items replacing parent/child run
  control, Go-channel mailboxes replacing DB-table channels, continuation
  synthesis deletion). Conjecture-program target, not yet cut over; current code
  still uses parent/child runs and continuations during the transition.
- `docs/choir-role-free-actor-protocol-2026-06-11.md` - target prompt/identity
  doctrine: obligations and authority envelopes instead of persona/"you are X"
  framing. Bounded profiles (super/vsuper/researcher/...) remain as authority
  envelopes; only the persona-prompt layer is retired, on the same cutover.
- `docs/system-v1-one-cut-2026-06-11.md` - derisking pseudocode for the v1 cut
  of the durable-actor model (durable schema, settlement, transactional send).
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
  requirements contract: Universal Wire, Private Wire reuse, platform/user
  computer authority, VText ownership, source artifacts, and deletion of legacy
  graph/source-maxxing behavior.
- `docs/mission-*-v*.md` - mission paradocs. Use the portfolio's Parallax
  State to identify the current spine mission instead of updating this index
  for every mission transition.
- `docs/mission-wire-community-news-v0.md` - older Universal Wire product
  mission. Do not resume it as the architecture spine; current Wire proof is
  downstream of M2-M4 in the portfolio.
- `docs/choir-universal-wire-style-vtext-dual-object-spec-2026-06-07.md` -
  historical product/architecture spec for the superseded Universal Wire +
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
- `docs/mission-campaign-compiler-selfdev-v0.md` - retained Choir-in-Choir
  benchmark input. Do not treat it as the current spine when the portfolio
  names an architecture-spine mission.
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
- `docs/mission-web-surface-rationalization-v0.md` - Obscura/Web Lens surface rationalization mission.
- `docs/mission-universal-wire-style-vtext-collaborative-storygraph-v0.md` -
  historical draft mission for the superseded Universal Wire / Style.vtext
  collaborative StoryGraph trajectory. Do not use it as current ontology.

Read the mission portfolio and the current paradoc first. Older
promotion-queue mission docs have been pruned; use the consolidated learnings
doc when that context is needed.

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
