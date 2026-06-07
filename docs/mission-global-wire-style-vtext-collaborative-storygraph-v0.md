# Mission: Global Wire / Style.vtext Collaborative StoryGraph

**Status:** draft MissionGradient mission  
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Current context:** `docs/news-system-current-state-and-improvements-2026-06-06.md`,
`docs/vtext-styleguide-system-research-2026-06-06.md`,
`docs/intended-architecture-next-2026-06-06.md`

## Goal String

```text
/goal Run docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md as a MissionGradient mission. Build the Global Wire / Style.vtext collaborative StoryGraph product trajectory: live source evidence -> StoryGraph -> Story VTexts -> Style.vtext projections -> News app views -> user-owned edits/contributions -> research/reconciliation-ready state. Start from the lowest honest resolution of the whole object, then keep increasing resolution along the highest-value axes until a ship-worthy product slice is deployed and proven, or a true blocker remains after root-cause investigation, Computer Use/browser/product-path probes where useful, cognitive transforms, and serious alternative solution routes that reconceive the problem or architecture rather than patching around it. Preserve all invariants in docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md: every story is a normal editable VText; user edits create user-owned versions/forks and do not mutate platform stories; Style.vtext is a citeable source artifact that can be selected/composed/replaced; news is non-oracle and provenance-rich; graph nodes are story headlines with source-neighborhood semantics; all app views work in Future Noir, Carbon Kintsugi, and London Salmon. Use staging/product-path proof for platform behavior and update the mission doc with checkpoint/resumption state before stopping.
```

## Real Artifact

A production-shaped collaborative story system in Choir:

```text
source ingestion/search
-> StoryGraph
-> platform Story VTexts
-> Style.vtext projections
-> News app reader/editor/graph/evidence/contribution views
-> user-owned VText forks and publishable user versions
-> research/reconciliation-ready contribution records
```

The mission is not complete when a demo exists. The mission continues up
resolution axes until the product slice is ship-worthy or a true blocker is
proven.

## Value Criterion

Minimize divergence between the shipped product path and the final dual-object
topology while increasing useful product resolution. The product improves when:

- source evidence is durable and inspectable;
- StoryGraphs preserve provenance, related stories, contradictions, and change
  state;
- Story VTexts are ordinary VTexts, not bespoke cards;
- `Style.vtext` projections alter framing and salience without altering
  evidence;
- users can fork/edit/publish their own versions without mutating platform
  stories;
- contributions create research/reconciliation-ready artifacts;
- the News app makes the story readable and the graph navigable in all themes;
- staging evidence proves the product path, not just local code existence.

## Hard Invariants

- Every story must be viewable as a normal VText.
- Story editing by a user must create a user-owned VText version/fork.
- Platform story mutation must remain separate from user edits.
- Story VTexts must preserve citations/transclusions to raw sources,
  sourcecycled items, other Story VTexts, user artifacts, and `Style.vtext`
  sources where applicable.
- `Style.vtext` must be treated as a source artifact, not app config.
- Public news must remain non-oracle: claims, uncertainty, source standing,
  contrary evidence, and changes over time remain inspectable.
- Graph default nodes are Story VTexts/headlines. Sources/claims/entities are
  overlays.
- All UI must work in Future Noir, Carbon Kintsugi, and London Salmon.
- Low resolution must preserve final topology. Do not replace topology with a
  fake ladder such as a static mock, a style-only demo, or a news-only feed.
- Use product-path/staging proof before claiming platform behavior.

## Homotopy Axes

Increase these axes continuously as evidence allows:

- **Source axis:** seed source set -> 24/7 sourcecycled ingest -> source
  standing and broader live corpus.
- **StoryGraph axis:** simple story grouping -> durable StoryGraph -> claims,
  tensions, timeline, contradictions, related stories.
- **VText axis:** story display -> normal VText view -> user-owned editable fork
  -> publishable user version.
- **Style axis:** hand-authored `Style.vtext` seeds -> cited style source refs
  -> style composition/replacement -> proposed style revisions.
- **Projection axis:** wire projection -> multiple projections -> projection
  comparison/review -> Autoradio-ready projection traversal.
- **Contribution axis:** add source/comment -> research task -> source/claim
  review -> reconciliation-ready queue.
- **Graph axis:** story list -> headline-node neighborhoods -> source/claim
  overlays -> recency/prominence/tension semantics.
- **Theme/UI axis:** one working view -> all views in all three themes -> mobile
  and desktop polish.
- **Evidence axis:** local proof -> product-path proof -> staging deploy proof
  -> 24/7 soak/acceptance evidence.

The mission should always choose the next move by asking which axis most
reduces product unreality while preserving invariants.

## Cognitive Transform Baseline

Before major implementation and before stopping on any nontrivial blocker,
apply route-changing cognitive transforms. Useful starting transforms:

- **Depth extraction:** news is not a feed; style is not a tab. The real object
  is source-grounded public reality projected through citeable authored styles
  into editable VTexts.
- **Topology preservation:** every step must preserve ingestion, StoryGraph,
  VText, Style.vtext, app view, contribution, and future reconciliation edges.
- **Ownership boundary:** platform story and user-owned story must diverge
  cleanly.
- **Source generalization:** `Style.vtext`, Story VTexts, user notes, and raw
  news items are all source artifacts.
- **UI projection:** themes are user skins over one object, not separate product
  modes.

Transforms must change implementation route, verifier, scope, evidence plan, or
stopping condition. Decorative reframing does not count.

## Receding-Horizon Execution

Operate in short loops:

1. Establish current code reality for Source Service, VText creation/editing,
   frontend app registry/themes, source refs, and publication path.
2. Document any new problem before fixing platform behavior, per AGENTS.md.
3. Choose the next highest-value resolution axis.
4. Implement within a bounded mutation radius.
5. Verify through the strongest available product-path evidence.
6. Update belief state and continue up the next axis.

Do not stop merely because one low-resolution loop works. Continue until the
stopping condition is met.

## Dense Feedback

Use evidence appropriate to the axis:

- Go tests for source/story/projection data contracts and ownership semantics.
- Frontend tests or browser proof for News app view, theme switching, graph
  semantics, VText viewing/editing/forking, and contribution controls.
- Staging proof for any platform behavior claim.
- Source manifest examples that show lead/supporting/contrary/context tiers.
- Projection comparison artifacts that show `Style.vtext` changes framing
  without changing evidence.
- VText refs proving stories cite sources, other Story VTexts, and style
  artifacts where applicable.

## Anti-Goodhart Constraints

- Do not build a static news mock and call it product progress.
- Do not build graph visualization without story-headline node semantics.
- Do not create fake personas.
- Do not let style projections invent evidence.
- Do not hide provenance behind pretty prose.
- Do not bypass VText to render stories as one-off frontend objects.
- Do not mutate platform stories from user edits.
- Do not restrict features to one theme.
- Do not claim source/news reliability from local-only proof when staging
  behavior is required.

## Rollback Policy

- Keep code changes scoped and reversible by feature boundary.
- If platform behavior changes, commit problem documentation first, then code
  fixes.
- Preserve old source/news/VText behavior unless the mission intentionally
  replaces it and proves the replacement.
- If the app surface is too large, keep the data contracts and VText semantics
  intact while reducing UI polish, not topology.

## Stopping Conditions

Stop only when one is true:

- `complete`: A ship-worthy product slice exists and is proven through product
  path/staging evidence: live/source-backed StoryGraph, Story VTexts,
  `Style.vtext` projections, News app views in all themes, user-owned edit/fork
  path, contribution/research-ready path, and documented residual risks.
- `checkpoint_incomplete`: Significant uphill product resolution is landed, but
  authorized time/context ends. Mission doc contains resumable state and the
  next executable probe.
- `blocked_incomplete`: A blocker remains after root-cause probes, Computer
  Use/browser/product-path probes where useful, cognitive transforms, and
  serious alternative solution routes that reconceive the problem or
  architecture rather than patching around it. The report names exact evidence
  and smallest next authority/probe.
- `superseded`: Evidence shows the mission identity is wrong and continuing
  would optimize the wrong object.

## Run Checkpoint & Resumption State

```text
status: draft
last checkpoint: mission authored
current artifact state: no mission execution yet
what shipped: none
what was proven: none
unproven or partial claims: all implementation claims
belief-state changes: none
remaining error field: full dual-object product path is unbuilt
highest-impact remaining uncertainty: where the existing VText/source/news code
  allows the shortest production-shaped StoryGraph -> Story VText -> Style.vtext
  projection -> News app path
next executable probe: inspect current Source Service, VText creation/editing,
  frontend app registry/theme system, source ref rendering, and publication
  paths; then choose the first resolution axis to raise without breaking
  topology
suggested resume goal string: see Goal String above
evidence artifact refs: none yet
rollback refs: git history before mission execution
```

## Execution Problem Record - 2026-06-07

Problem: the repo has source ingestion, ordinary VText creation/editing, source
entity rendering, publication derivatives, and theme presets, but it does not
yet have a user-facing News app surface that preserves the required dual-object
topology:

```text
source evidence -> StoryGraph -> Story VText -> Style.vtext projection
-> news reader/graph/evidence views -> user-owned fork/edit
-> contribution/research-ready record
```

Evidence from initial code inspection:

- `cmd/sourcecycled`, `internal/sources`, and `internal/cycle` provide the
  current Source Service substrate, with browser-public product use still
  mostly absent from the News app path.
- `frontend/src/lib/apps/registry.ts` has no News/Global Wire app entry.
- `frontend/src/lib/VTextEditor.svelte` can create normal owner-scoped VTexts
  and can create private derivatives from published VTexts, but there is no
  News app control that launches story VTexts from a StoryGraph object.
- `frontend/src/lib/theme.ts` defines the three required themes, while no News
  view currently exercises the StoryGraph/evidence/fork/contribution surface
  across those themes.

Belief state: the lowest honest resolution is not a static news mock. It is a
thin but production-shaped News app that carries one or more durable
StoryGraph-shaped story records, source manifests with lead/supporting/contrary
tiers, citeable `Style.vtext` artifacts, projection switching, normal VText
launch/fork semantics, a story-headline graph neighborhood, and a
research/reconciliation-ready contribution queue. The implementation can begin
with seeded source-backed story records only if the topology and product
controls deform cleanly into live Source Service backed records later.

Remaining error field: decide whether the first code slice should introduce a
dedicated backend StoryGraph API immediately, or first ship a frontend
product-surface slice that uses existing VText APIs for owner-owned artifacts
and records the missing durable StoryGraph API as residual risk.
