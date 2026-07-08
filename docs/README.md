# Choir Documentation Index

Last reviewed: 2026-07-08

This directory contains Choir Doctrine, operating contracts, active Definition
mission documents (`/goal <doc>.md`), archived Parallax-era and MissionGradient-era
paradocs, domain invariant docs, descriptive architecture docs, proof artifacts,
and historical pointers. Do not treat every file here as equally current.

> Era note (2026-07-08): the repo has moved from MissionGradient to Parallax to
> Definition as the long-running mission form. `skills/definition/SKILL.md` is the
> current authority for executable missions; `skills/parallax/SKILL.md` and
> `docs/agent-parallax-rules.md` are legacy reference. The current umbrella
> mission is `docs/definitions/og-dolt-heresy-completion-2026-07-08.md`.

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
  computers, Texture, source, promotion, or runtime authority.
- **Current descriptive architecture** describes code/staging reality and
  labeled target hardening. It should be corrected when it conflicts with code,
  staging, or doctrine.
- **Current mission docs / definition docs** define active or recently stopped
  Definition (`/goal`) work. They are runnable/inspectable mission context, not
  global architecture unless promoted into canonical docs.
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
  `docs/texture-agentic-invariants-2026-06-13.md`,
  `docs/runtime-invariants.md`, `docs/source-external-data-publication.md`,
  and promotion/source-specific doctrine where explicitly current.
- **Current descriptive architecture:** `docs/current-architecture.md` and
  `docs/platform-os-app-state.md`.
- **Historical evidence:** dated proof, review, dogfood, MissionGradient, and
  superseded mission reports.
- **Superseded/dangerous-if-current:** docs that normalize retired root
  ontologies such as retired StoryGraph-as-root, personal writing/retired publishing system as
  root, retired Trace app, retired raw Terminal app, Browser-as-source-gathering, parent/child
  control, or retired continuation-level as target doctrine.
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
- `docs/archive/mission-geometry.md` - high-level mission geometry: Choir as statistical/symbolic/evolutionary learner and automatic computer -> newspaper -> radio -> capital vector.
- `docs/why-texture-2026-06-15.md` - explanatory support document for Texture:
  the artifact layer for directed autonomous results and compounding learning.
- `docs/why-texture-background-2026-06-15.md` - historical/background support
  for the Texture rename, multi-agent transcript failure, web desktop
  deduction, and safe self-development context.
- `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` - current umbrella
  Definition mission: complete the OG/Dolt cutover, heresy elimination, and
  past-mission open edges. Executable via `/goal`.
- `docs/mission-texture-hard-cutover-v0.md` - superseded Parallax-era Texture
  mission; remaining work folded into the texture-product-loop-recovery mission.
- `docs/parallax-design-2026-06-11.md` - historical mission-discipline design for
  the Parallax era; new broad work uses `skills/definition/SKILL.md`.
- `docs/mission-portfolio-2026-06-11.md` - current durable-actor
  rearchitecture mission portfolio and execution order.
- `docs/archive/choir-agentic-depth-canonical.md` - canonical run-depth vocabulary for
  MissionGradient-era MissionBag, Sweep, Leap, Fly, Cycle, and
  worker/verifier/orchestrator roles; historical reference only; new missions use
  Definition.
- `docs/missiongradient-method.md` - legacy MissionGradient run-geometry method.
  Use only as historical baseline/fallback for old mission documents; new broad
  work uses Definition via `skills/definition/SKILL.md`.
- `docs/cognitive-transform-portfolio.md` - transform portfolio entrypoint for route-changing reframes; canonical skill lives at `skills/cognitive-transform-portfolio/SKILL.md`.
- `docs/computer-ontology.md` - canonical vocabulary for persistent user computers, ledger split, personal promotion, platform/public promotion, and update algebra.
- `docs/archive/wire-news-system-learning-saga-2026-06-09.md` - current learning record
  for the news/Wire ontology correction: Wire is platform-level in the
  Community Cloud, reusable in Private Clouds, and personalized in user
  computers.
- `docs/archive/choir-strategy-overview-2026-06-09.md` - high-level strategy overview:
  own your AI cloud, own the learning, private clouds, Wire, Texture, and radio.
- `docs/archive/choir-deck-treatment-and-faq-2026-06-09.md` - current deck/FAQ treatment
  for export after the Wire terminology correction.
- `docs/archive/vm-priority-policy.md` - current and future VM/computer warmness,
  reclaim, always-on, and uptime-tier policy.
- `docs/platform-os-app-state.md` - current common platform/default computer
  state ledger for the OS substrate, desktop shell, app catalog, app boundaries,
  proof anchors, and known UX/app gaps. Keep it updated as platform app state
  changes; later divergent user computers should expose their own equivalent
  state records.
- `docs/archive/project-goals.md` - current goal continuum and extracted live signal from older project/mission docs.
- `docs/archive/glossary.md` - canonical vocabulary for current product/runtime terms.
- `docs/archive/adr-dolt-as-canonical-state.md` - Dolt/SQLite state-boundary decision.
- `docs/archive/public-identity-and-custom-domains.md` - public handle, route, and
  custom domain roadmap.
- `docs/current-architecture.md` - current product/runtime architecture,
  including the surviving service-topology signal from deleted older sketches.
- `docs/archive/choir-rearchitecture-durable-actors-2026-06-11.md` - target durable-actor
  rearchitecture conjecture (trajectories/work items replacing parent/child run
  control, Go-channel mailboxes replacing DB-table channels, continuation
  synthesis deletion). Conjecture-program target, not yet cut over; current code
  still uses parent/child runs and continuations during the transition.
- `docs/archive/choir-role-free-actor-protocol-2026-06-11.md` - target prompt/identity
  doctrine: obligations and authority envelopes instead of persona/"you are X"
  framing. Bounded profiles (super/vsuper/researcher/...) remain as authority
  envelopes; only the persona-prompt layer is retired, on the same cutover.
- `docs/system-v1-one-cut-2026-06-11.md` - derisking pseudocode for the v1 cut
  of the durable-actor model (durable schema, settlement, transactional send).
- `docs/intended-architecture-next-2026-06-06.md` - target architecture for
  the next week-plus of work; not current-state proof.
- `docs/frontend-app-building-api.md` - current frontend app registry, preview,
  theme, and shell contract.
- `docs/runtime-invariants.md` - implementation invariants and authority boundaries.
- `docs/source-external-data-publication.md` - canonical contract for external
  data ingestion, source cleaning, Texture source metadata, transclusion,
  publication policy, and export.
- `docs/archive/news-system-current-state-and-improvements-2026-06-06.md` - current
  source/news code-state and code-review note. Use it for sourcecycled,
  source_search, Texture source-service refs, News app gaps, and rough future
  directions; do not treat its improvement list as accepted mission scope.
- `docs/mission-*-v*.md` and `docs/definitions/*-*.md` - mission and definition
  documents. Use `docs/mission-graph.yaml` and the portfolio/definition state
  to identify the current spine mission instead of updating this index for every
  mission transition.
- `docs/choir-universal-wire-style-vtext-dual-object-spec-2026-06-07.md` - (`texture-cutover-allow:` historical filename retained; deletion receipt: `texture-hard-cutover-v0`)
  historical product/architecture spec for the superseded Universal Wire +
  retired StoryGraph/Texture framing. Do not use it as current ontology.
- `docs/archive/vtext-styleguide-system-research-2026-06-06.md` - research synthesis (`texture-cutover-allow:` historical filename retained; deletion receipt: `texture-hard-cutover-v0`)
  for Texture-native style support, client corpus ingestion, learned style
  memory, edit feedback, style review, and optional future fine-tuning.
- `docs/archive/vtext-styleguide-source-theme-synthesis-2026-06-06.md` - theme synthesis (`texture-cutover-allow:` historical filename retained; deletion receipt: `texture-hard-cutover-v0`)
  across all styleguide and anti-slop sources with consensus, controversy, and
  outlier breakdown.
- `docs/archive/implementation-scope.md` - near-term implementation scope.
- `docs/archive/north-star.md` - long-range product direction.
- `docs/archive/spec-choir-desktop-wails-v3-2026-06-22.md` - native macOS desktop app
  build spec: Wails v3 shell, ASWebAuthenticationSession auth bridge, phase
  plan (Phase 1 implemented, Phase 2 and 7 partially implemented, Phases 3-6 spec'd).
- `cmd/desktop/README.md` - native macOS app setup, build, auth bridge docs,
  and configuration reference.
- `docs/archive/mission-campaign-compiler-selfdev-v0.md` - retained Choir-in-Choir
  benchmark input. Do not treat it as the current spine when the portfolio
  names an architecture-spine mission.
- `docs/legacy-promotion-experiments-learnings.md` - consolidated lessons from
  pruned patchset-promotion experiments.
- `docs/archive/mission-apps-and-changes-store-sweep-v0.md` - retained state for the
  Apps & Changes product path; historical portfolio inputs were pruned.

## Current Mission Family

- `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` is the current
  umbrella Definition mission. It absorbs the incomplete `mission-og-dolt-heresy-
  hard-cutover-v0` and `heresy-eradication-2026-07-07` runs, plus open edges from
  past missions, and is executable via `/goal`.
- `docs/mission-portfolio-2026-06-11.md` and `docs/mission-graph.yaml` define
  the durable-actor spine and portfolio ordering; many of its missions are
  absorbed, superseded, or deferred into the current umbrella or later missions.
  Use `docs/mission-graph.yaml` as the machine-readable source of current status.
- `docs/archive/mission-platform-source-service-vtext-publication-campaign-v1.md` - (`texture-cutover-allow:` historical mission filename retained; deletion receipt: `texture-hard-cutover-v0`)
  active Source Service / Texture source metadata / publication campaign. Its
  requirements contract is `docs/source-external-data-publication.md`.
- `docs/archive/mission-campaign-compiler-selfdev-v0.md` is the primary current
  self-development mission surface.
- `docs/archive/mission-choir-grand-deformation-v0.md` - broad Choir-in-Choir deformation sketch.
- `docs/archive/mission-run-memory-v0.md` - run-memory/compaction mission.
- `docs/archive/mission-web-surface-rationalization-v0.md` - Obscura/Web Lens surface rationalization mission.
- `docs/mission-universal-wire-style-vtext-collaborative-storygraph-v0.md` - (`texture-cutover-allow:` historical mission filename retained; deletion receipt: `texture-hard-cutover-v0`)
  historical draft mission for the superseded Universal Wire / Style artifact
  collaborative retired StoryGraph trajectory. Do not use it as current ontology.

Read the mission portfolio, `docs/mission-graph.yaml`, and the current
definition document first. Older promotion-queue mission docs have been pruned;
use the consolidated learnings doc when that context is needed.

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
- Old Mission 1/2/3/5/6/7 milestone docs were deleted after live signal was folded into `docs/archive/project-goals.md`, `docs/archive/glossary.md`, `docs/archive/adr-dolt-as-canonical-state.md`, and the canonical architecture docs. Use git history for the removed originals.
- Top-level `TODOS.md`, `PROJECT-GOALS.md`, and `PROJECT-GLOSSARY.md` were removed after extraction.
- The old artifact-control hard-cutover checklist was deleted during the 2026-06-06
  mid-age cleanup. Its durable review lessons were folded into

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
  `docs/archive/adr-dolt-as-canonical-state.md`;
- leave proof docs as evidence artifacts unless a cleanup mission explicitly
  indexes, extracts, and deletes them;
- keep `docs/README.md`, `README.md`, and `AGENTS.md` current when missions
  promote new operating rules.
