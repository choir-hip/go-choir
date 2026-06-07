# Mission: Global Wire / Style.vtext Collaborative StoryGraph

**Status:** overnight MissionGradient delivery mission, checkpoint-incomplete
after first frontend slice  
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Current context:** `docs/news-system-current-state-and-improvements-2026-06-06.md`,
`docs/vtext-styleguide-system-research-2026-06-06.md`,
`docs/intended-architecture-next-2026-06-06.md`

## Goal String

```text
/goal Run docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md as an overnight MissionGradient delivery mission. The objective is to deliver the spec in docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md, not merely ship another slice. Continue from checkpoint commit 60ef7179 and build the one real Global Wire / Style.vtext collaborative StoryGraph object end to end: live or Source Service-backed SourceItem evidence -> durable StoryGraph records -> platform Story VTexts -> citeable Style.vtext projections -> News app views -> user-owned forks/edits/contributions -> research/reconciliation-ready queues. Work for roughly 8 hours or until the spec is delivered and proven, or until a true blocker remains after serious root-cause investigation. Do not stop after a frontend slice, local test pass, staging smoke, or "ship-worthy product slice" if any required part of the spec remains undelivered and an authorized next probe can still move it uphill. Preserve every invariant in the spec: every story is a normal editable VText; user edits create user-owned versions/forks and do not mutate platform stories; Style.vtext is a citeable source artifact that can be selected/composed/replaced; news is non-oracle and provenance-rich; graph nodes are Story VText headlines with source-neighborhood semantics; all app views work in Future Noir, Carbon Kintsugi, and London Salmon. Apply cognitive transforms before each major route choice and before any stop; prefer backend/product-path realism over UI-only polish once the first surface works; use product-path/staging proof for platform behavior; commit, push, monitor CI/deploy, verify staging identity, run deployed acceptance proof, and update this mission doc with an owner-readable overnight checkpoint/resumption state before stopping.
```

## Real Artifact / Delivery Target

The delivery target is the full dual-object spec in
`docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`, expressed
as a production-shaped collaborative story system in Choir:

```text
source ingestion/search
-> StoryGraph
-> platform Story VTexts
-> Style.vtext projections
-> News app reader/editor/graph/evidence/contribution views
-> user-owned VText forks and publishable user versions
-> research/reconciliation-ready contribution records
```

The mission is not complete when a demo exists, when the app shell renders, or
when a partial slice deploys. Completion means the spec's required object model,
collaboration model, app views, graph semantics, theme requirements, and
source-backed publication loop are implemented to the highest honest overnight
resolution and proven through staging/product-path evidence. If the full spec
cannot be delivered overnight, the stop state must be `checkpoint_incomplete` or
`blocked_incomplete`, never "complete".

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

## Overnight Cognitive Reframe - 2026-06-07

Current obstacle: the first execution stopped after a deployed frontend slice.
That was useful progress but an under-resolution of the mission. The overnight
run must treat that slice as a foothold, not a finish line.

Selected transforms:

1. **Depth extraction / esoteric upgrade** - The banal object is a News app.
   The deep object is a public-reality state machine whose projections are
   editable VTexts. This changes the next route from UI expansion to durable
   SourceItem/StoryGraph/Style.vtext contracts.
2. **Topology preservation** - The app is only the viewing/editing instrument.
   The artifact identity lives in the chain from source evidence to StoryGraph
   to VText to user-owned/reconciliation state. This changes the verifier from
   "does the screen render?" to "can a source-backed story become a normal
   VText, fork, contribution, and reconciliation candidate without crossing an
   ownership boundary?"
3. **Stop-condition inversion** - A shipped slice is the minimum deployed
   foothold, not the overnight stopping condition. This changes the stop rule:
   continue while an authorized next probe can raise Source, StoryGraph, VText,
   Style, Contribution, or Evidence realism.
4. **Backend-first realism after surface proof** - Once the app shell exists,
   UI polish has lower value than durable story/source/contribution state. This
   changes the next implementation route toward backend contracts and frontend
   consumption of those contracts, with seeded data only as fallback.
5. **Sleeping-owner autonomy** - The owner will be asleep, so the run must not
   depend on subjective approval between ordinary safe increments. This changes
   the execution policy: make conservative implementation choices, checkpoint
   evidence, and stop only on invariant-level risk, external authority, or an
   investigated true blocker.

Changed plan:

- implementation: next raise backend/product realism: define durable
  StoryGraph, projection, and contribution records; connect Global Wire to that
  contract; create or reuse VText story/style artifacts through normal VText
  APIs; preserve seeded records only as public-preview fallback.
- verifier/evidence: add Go data-contract tests, frontend contract tests,
  product-path authenticated tests, staging build identity checks, and deployed
  proof for source-backed story -> VText -> fork/contribution behavior.
- scope: keep one real story family if needed, but it must traverse the whole
  topology. Reduce breadth of sources/styles before cutting any topology edge.
- stopping condition: overnight checkpoint may stop after roughly 8 hours or a
  true blocker; "deployed frontend works" is not sufficient.

Next high-information action: inspect current Source Service item/search API,
VText metadata/citation APIs, and available store migration patterns, then
choose the smallest durable StoryGraph backend contract that can feed the
existing Global Wire app and preserve VText ownership semantics.

## Overnight Execution Contract

This is an overnight delivery mission. The expected operating horizon is roughly
8 hours, not a short interactive turn. The agent should continue through
multiple receding-horizon loops while the owner is away, always measuring
progress against the spec rather than against the last shipped slice.

Priority order after the first frontend slice:

0. Spec coverage audit: enumerate every MUST/required object/view/invariant in
   `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`, map it
   to current code/proof, and keep an explicit undelivered-spec list in this
   mission doc.
1. Durable backend contracts for `SourceItem`, `StoryGraph`, `StoryProjection`,
   `Style.vtext` source refs, and contribution/reconciliation records.
2. Product path from source-backed StoryGraph into ordinary VText story/style
   artifacts with owner-scoped fork/edit behavior.
3. Global Wire app consumption of durable records, with seeded frontend records
   retained only as preview/fallback.
4. Evidence and graph semantics richer than the current seeded manifest:
   lead/supporting/contrary/context tiers, claims, tension, related story
   neighborhoods, freshness/change state.
5. Theme/mobile polish only after the product path is behaviorally real.

Autonomous stop rules:

- Do not stop for ordinary test failures; investigate root cause and fix within
  authority.
- Do not stop after any single successful deploy if a clear next realism axis
  remains and is safe to pursue.
- Stop with `checkpoint_incomplete` only when the overnight/time/context window
  is exhausted after useful progress and the mission doc is updated.
- Stop with `blocked_incomplete` only after root-cause probes, product/browser
  probes where useful, cognitive transforms, and at least one serious alternate
  route have failed or crossed an authority/invariant boundary.
- Stop with `complete` only if the full source-backed StoryGraph/VText/style
  projection/user-owned contribution trajectory delivers the spec and is
  deployed/proven at the level defined below.

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

- `complete`: The spec is delivered and proven through
  product-path/staging evidence: live or Source Service backed `SourceItem`
  evidence feeds durable StoryGraph records; StoryGraphs produce normal
  editable Story VTexts; selected `Style.vtext` artifacts are cited by
  projections; News app views consume the durable records and work in all three
  themes; signed-in users can create user-owned forks/edits/contributions
  without mutating platform stories; contribution records are
  research/reconciliation-ready; all required News app views exist; graph
  semantics match the spec; all three themes are verified; residual risks are
  documented. A UI-only, seeded-data, or partial-backend slice is not complete.
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
status: checkpoint_incomplete
last checkpoint: 2026-06-07 Global Wire app slice deployed to staging at
  f3e6a59db82e7a08f27829bfa7f9469104e789fe
current artifact state: Choir has a user-facing Global Wire app registered in
  the Desk/mobile app registry. The app renders seeded StoryGraph-shaped story
  nodes, source manifests with lead/supporting/contrary/context tiers, story
  reader, style projection switcher, StoryGraph headline neighborhood, normal
  VText launch/fork controls, and a user contribution surface.
what shipped:
  - docs/spec/problem checkpoint commit b60c1076
  - product slice commit f3e6a59d
  - `frontend/src/lib/GlobalWireApp.svelte`
  - `frontend/tests/global-wire-app.spec.js`
what was proven:
  - local `npm run build` passed
  - local `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npx playwright test
    tests/global-wire-app.spec.js` passed with 4 public UI tests and the
    deployed-auth proof skipped by default
  - staging CI run 27080878731 passed, including frontend build, Go gates,
    runtime shards, and Deploy to Staging (Node B)
  - FlakeHub publish run 27080878720 passed
  - `https://choir.news/health` reported proxy and sandbox deployed commit
    f3e6a59db82e7a08f27829bfa7f9469104e789fe at 2026-06-07T02:56:36Z
  - deployed public proof
    `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test
    tests/global-wire-app.spec.js` passed 4/4
  - deployed ownership proof
    `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news
    npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
    passed and verified signed-in fork/contribution actions create
    owner-scoped VText documents
  - Browser visual proof on staging observed 3 story nodes, style switcher,
    VText launch, contribution surface, no horizontal overflow, and no
    evidence/graph rail overlap
unproven or partial claims:
  - StoryGraph records are seeded frontend records, not yet durable backend
    StoryGraph objects derived from live Source Service ingestion
  - source evidence is source-shaped and provenance-rich but not yet fetched
    live in the News app product path
  - contribution queue is represented through user-owned VTexts and local app
    state; it is not yet a durable backend reconciliation queue
  - Style.vtext sources open as ordinary VTexts, but style selection/composition
    is not yet backed by durable Style.vtext source refs or permissions
  - no run acceptance record was synthesized in this checkpoint
belief-state changes:
  - the shortest topology-preserving slice is frontend app + existing VText API,
    not a static mock and not a new backend API first
  - existing VText launch/createInitialVersion semantics are sufficient to prove
    the ownership boundary for first-resolution story forks and contributions
  - the next realism axis should move seeded StoryGraph data into a durable
    source-backed backend contract without changing the app topology
remaining error field: the shipped slice is product-shaped and deployed, but
  still needs live Source Service backed StoryGraph persistence, durable
  contribution/reconciliation records, and citeable Style.vtext source relation
  storage before the full dual-object mission can be complete
highest-impact remaining uncertainty: exact backend boundary for
  SourceItem/StoryGraph/Style.vtext projection records that can feed the News
  app while preserving normal VText ownership and publication semantics
next executable probe: define and implement a small backend StoryGraph API or
  store module that reads Source Service items into durable StoryGraph records,
  then have Global Wire consume that contract with the seeded records as a
  fallback only
suggested resume goal string: see Goal String above
evidence artifact refs:
  - GitHub Actions CI run 27080878731
  - FlakeHub publish run 27080878720
  - staging health JSON at 2026-06-07 showing deployed commit f3e6a59d...
  - Playwright commands listed in what was proven
rollback refs:
  - pre-mission base d5bc2193
  - docs checkpoint b60c1076
  - product slice f3e6a59d
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

## Spec Coverage Audit - 2026-06-07 Overnight Resume

Current authoritative state at resume: commit `82e80dd1` contains the deployed
Global Wire frontend slice and this overnight delivery mission rewrite. The
worktree is clean.

Coverage map against
`docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`:

- **SourceItem:** partially present in Source Service/sourcecycled
  (`cmd/sourcecycled`, `internal/sources`, `internal/cycle`) and VText source
  refs, but Global Wire does not yet consume Source Service items as product
  data. Undelivered: browser-public/global-wire SourceItem contract and story
  source normalization in the News app path.
- **StoryGraph:** only represented as seeded frontend records in
  `frontend/src/lib/GlobalWireApp.svelte`. Undelivered: durable backend
  StoryGraph object with source manifest, claims, related Story VText refs,
  edges, timeline, prominence/freshness, and contribution queue refs.
- **Story VText:** normal VText launch/fork behavior is proven through existing
  VText APIs and staging ownership proof. Undelivered: platform Story VTexts
  linked to durable StoryGraph records and source/style citations as backend
  state.
- **Style.vtext:** style sources open as ordinary VTexts from frontend-seeded
  content. Undelivered: citeable style source refs on projections,
  replacement/composition records, and durable relation from projection to
  `Style.vtext`.
- **Story Projection:** seeded projection strings exist. Undelivered: backend
  projection relation `StoryGraph + Style.vtext + context -> Story VText`
  preserving evidence while changing salience/framing.
- **Collaboration/contribution:** signed-in contribution creates owner-scoped
  VTexts. Undelivered: durable contribution/research queue linked to the
  StoryGraph and reconciliation-ready records.
- **News app views:** frontend has front page, story reader, evidence, graph,
  style switcher, and contribution controls. Undelivered: durable data backing,
  explicit Autoradio/Ask hooks, source/claim/timeline overlays, and richer
  required view semantics.
- **Graph semantics:** current graph uses headline nodes, recency-ish tone, and
  prominence sizing from seeded data. Undelivered: durable edge semantics,
  source-neighborhood computation, overlays, and tension/source-quality badges.
- **Themes:** public deployed proof covers all three themes for the current
  surface. Must be repeated after backend/data changes.

Route choice: the next implementation should introduce a small durable Global
Wire backend contract rather than adding more UI-only detail. Seeded frontend
records may remain only as logged-out/public-preview fallback; signed-in and
runtime product paths should load StoryGraph-shaped records from the backend
and write contribution records through that contract.

## Overnight Checkpoint - Durable StoryGraph Slice - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `d9d9e670251c4965e148b64bbadb696486793412`
(`feat: add durable global wire storygraph`)

What changed:

- Added a typed Global Wire domain object in `internal/types/global_wire.go`:
  `GlobalWireStory`, `GlobalWireSourceManifest`, `GlobalWireStyleSource`,
  `GlobalWireContribution`, and source-neighborhood source items.
- Added Dolt-backed runtime tables:
  `global_wire_story_graphs` and `global_wire_contributions`.
- Added store logic that seeds an authenticated owner's durable StoryGraph on
  first open, including normal VText documents for each platform story and each
  citeable `Style.vtext` source. Seeded story revisions are appagent-authored
  and carry metadata saying user edits must fork rather than mutate the
  platform story.
- Added public product API routes:
  - `GET /api/global-wire/stories`
  - `GET /api/global-wire/contributions`
  - `POST /api/global-wire/contributions`
- Updated the Global Wire app so signed-in users load `durable-storygraph`
  records and contribution queues through those API routes; logged-out users
  still see the seeded public preview fallback.
- Updated the app so platform Story VTexts and Style.vtext sources open by
  existing VText document IDs, while forks and contribution drafts still create
  separate user-owned VText documents.
- Extended deployed Playwright proof to assert authenticated durable
  StoryGraph provenance and durable contribution queue state.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npx playwright test tests/global-wire-app.spec.js`
  passed 4 public tests with the guarded authenticated proof skipped.

What was proven on staging:

- Pushed `d9d9e670251c4965e148b64bbadb696486793412` to `origin/main`.
- GitHub Actions CI run `27081439928` completed successfully, including
  runtime shards, non-runtime Go tests, frontend build, and staging deploy.
- FlakeHub publish run `27081439921` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `d9d9e670251c4965e148b64bbadb696486793412`, deployed at
  `2026-06-07T03:25:35Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public tests with the guarded authenticated proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed, proving signed-in durable StoryGraph provenance, owner-scoped fork
  VText creation, contribution VText creation, and durable
  `pending-researcher-review` queue records through public product APIs.

Belief-state changes:

- The backend boundary is now real enough to support product-path proof:
  signed-in News app views are no longer UI-only seeded data.
- Normal VText identity is now present for platform Story VTexts and citeable
  Style.vtext artifacts, not just for user-created forks.
- The next highest-value realism axis is not more graph UI. It is replacing
  seeded source neighborhoods with Source Service-backed `SourceItem` evidence
  and making the projection relation explicit enough for reconciliation.

Remaining error field:

- Source evidence is still seeded by Global Wire store bootstrap, not live or
  Source Service-backed.
- StoryGraph edges and source-neighborhood computation are persisted as related
  story IDs and manifests, but not yet computed from SourceItems or exposed as
  first-class edge records.
- Projection records still live inside the story JSON instead of a separate
  `StoryGraph + Style.vtext + context -> Story VText` relation.
- Contribution records are research/reconciliation-ready queue entries, but no
  researcher reconciliation workflow consumes them yet.
- Autoradio and Ask hooks remain outside this slice.

Next route choice after cognitive transform:

- Apply the **source-substrate inversion** lens: instead of asking how to make
  the seeded graph look more real, ask what minimal Source Service-backed
  `SourceItem` record must exist so the graph can be regenerated without
  changing the app contract.
- Implement a small `global_wire_source_items`/Source Service adapter or reuse
  existing `ContentItem`/source records if their ownership and provenance shape
  matches the spec.
- Preserve the current `/api/global-wire/stories` contract as the app-facing
  boundary while moving seed evidence behind a SourceItem normalization layer.

Rollback refs:

- pre-mission base: `d5bc2193`
- docs/problem checkpoint: `b60c1076`
- frontend slice: `f3e6a59d`
- ownership proof checkpoint: `60ef7179`
- overnight mission rewrite: `82e80dd1`
- backend coverage audit: `bc5e4a8d`
- durable StoryGraph slice: `d9d9e670`

## Overnight Checkpoint - SourceItem-Backed Evidence - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `3a804bc1e864c0b439108234dc4097551c99449a`
(`feat: back global wire sources with content items`)

Route choice and cognitive transform:

- Applied source-substrate inversion: the next resolution increase should make
  evidence ownable/inspectable as SourceItems, not merely make the StoryGraph
  UI richer.
- Reused existing `ContentItem` as the SourceItem substrate because it already
  carries stable ID, owner scope, source type, media type, title, content,
  metadata, provenance, and product API access through `/api/content/items/*`.

What changed:

- Each seeded Global Wire source manifest entry now gets a backing
  `ContentItem` before its Story VText is created.
- `GlobalWireSourceItem` now carries `content_id` and `canonical_url`.
- Story VText seed revisions cite the backing content items with
  `content_item` citations.
- The focused runtime test now proves that a lead StoryGraph source can be
  loaded through `/api/content/items/{content_id}`.
- The guarded deployed Playwright auth proof now verifies:
  - durable StoryGraph provenance;
  - source manifest `content_id`;
  - `ContentItem.app_hint == "global-wire"`;
  - `ContentItem.metadata.schema == "choir.global_wire_source_item.v1"`;
  - owner-scoped fork VTexts;
  - durable contribution queue records.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed after the SourceItem-backed change.

What was proven on staging:

- Pushed `3a804bc1e864c0b439108234dc4097551c99449a` to `origin/main`.
- GitHub Actions CI run `27081554053` completed successfully, including
  runtime shards, non-runtime tests, Go vet/build, and staging deploy. Frontend
  build was skipped because this behavior change did not affect frontend
  artifacts.
- FlakeHub publish run `27081554052` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `3a804bc1e864c0b439108234dc4097551c99449a`, deployed at
  `2026-06-07T03:31:18Z`.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed with the new SourceItem assertions.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded authenticated proof skipped.

Belief-state changes:

- Global Wire now has the minimum SourceItem -> StoryGraph -> Story VText link
  for authenticated users, but the SourceItems are still seed-normalized inside
  the Global Wire bootstrap path.
- The next realism axis should connect real imported/sourcecycled content into
  the same normalization layer rather than adding another table first.

Remaining error field:

- SourceItems are not yet pulled from the running sourcecycled Source Service,
  web search, upload, or user import flow.
- There is still no story clustering/update classifier from new SourceItems.
- Projection records remain embedded in the story object rather than separate
  projection relations.
- Research/reconciliation records exist as a queue, but there is no researcher
  worker path that consumes a contribution and proposes a StoryGraph update.

Next executable probe:

- Add an authenticated product route or app control that imports/promotes an
  existing `ContentItem` into a Global Wire StoryGraph source manifest without
  mutating the platform story from arbitrary user text.
- Prefer reusing `/api/content/items` and the current contribution queue over a
  new SourceItem table unless sourcecycled import requires additional standing
  or dedupe metadata.

## Overnight Checkpoint - User Source Contribution Artifacts - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `e860165ac0fbb7ddab7ab08a2e49a26bae93e85d`
(`feat: link global wire source contributions`)

What changed:

- Added `source_content_id` to `GlobalWireContribution` and
  `global_wire_contributions`, with a bootstrap migration for existing stores.
- `POST /api/global-wire/contributions` now creates a user-owned `ContentItem`
  for `source` and `counter-source` contributions before writing the
  reconciliation queue row.
- The contribution `ContentItem` uses schema
  `choir.global_wire_user_source_contribution.v1`, app hint `global-wire`,
  and provenance `created_from=global_wire_user_contribution`.
- The runtime emits the normal `content_item_created` product event for that
  user-owned source artifact.
- Focused backend and deployed Playwright tests now prove that the
  contribution queue links to the user-owned SourceItem artifact.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `e860165ac0fbb7ddab7ab08a2e49a26bae93e85d` to `origin/main`.
- GitHub Actions CI run `27081658832` completed successfully, including
  runtime shards, non-runtime tests, Go vet/build, and staging deploy.
- FlakeHub publish run `27081658833` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `e860165ac0fbb7ddab7ab08a2e49a26bae93e85d`, deployed at
  `2026-06-07T03:36:35Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed, proving:
  - durable StoryGraph provenance;
  - platform story source manifest `ContentItem` lookup;
  - user-owned fork VText creation;
  - contribution VText creation;
  - durable contribution queue state;
  - contribution `source_content_id` lookup through `/api/content/items/*`.

Belief-state changes:

- The collaboration path now has both a user-owned VText artifact and a
  user-owned SourceItem artifact. This better matches the spec's
  `user contribution -> user-owned VText/source artifact -> research queue`
  topology.
- The system still does not promote arbitrary user source content into platform
  story manifests; that invariant remains intact.

Remaining error field:

- No researcher/reconciler consumes queued source artifacts yet.
- Existing imported `ContentItem`s cannot yet be selected from the app as
  source contributions; the route creates a new text SourceItem from the
  contribution body.
- Sourcecycled/live ingestion is still not feeding the StoryGraph update path.
- Projection relation and graph edge tables remain implicit.

Next executable probe:

- Make projection a first-class relation before adding more UI: persist
  `StoryGraph + Style.vtext + context -> Story VText` records so style
  selection/composition/replacement can be cited and reconciled instead of
  remaining embedded projection text in story JSON.

## Overnight Checkpoint - Style Projection Relation - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `b5aec623c202f58900f8d06b856f945815abcd8f`
(`feat: persist global wire style projections`)

What changed:

- Added `GlobalWireStoryProjection`, the durable relation for
  `StoryGraph + Style.vtext + context -> Story VText`.
- Added `global_wire_story_projections` with style ID, style VText doc ID,
  projection Story VText doc ID, context JSON, projection text, and timestamps.
- StoryGraph seed now persists projection rows for each style source.
- The default wire projection points at the platform Story VText; alternate
  styles create separate projection VText documents.
- `GET /api/global-wire/stories` now returns `projection_vtext_docs`, mapping
  style IDs to their projection Story VText document IDs.
- The Global Wire app now opens the selected style's projection VText when a
  projection document exists.
- Runtime and deployed Playwright tests now prove the `claim-audit-style`
  projection document exists and is loadable through ordinary VText APIs.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `b5aec623c202f58900f8d06b856f945815abcd8f` to `origin/main`.
- GitHub Actions CI run `27081750911` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27081750915` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `b5aec623c202f58900f8d06b856f945815abcd8f`, deployed at
  `2026-06-07T03:41:45Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed, including projection VText lookup for the audit style.

Belief-state changes:

- The core dual-object topology is now present at low honest resolution:
  SourceItem-backed evidence -> durable StoryGraph -> normal Story VTexts ->
  citeable Style.vtext -> durable projection relation -> News app views ->
  user-owned VText/source contribution artifacts -> reconciliation queue.
- The largest remaining realism gap is not shape, but feed and workflow:
  sourcecycled/live imported SourceItems are not yet the source of graph
  creation/update, and no researcher/reconciler consumes queued contributions.

Remaining error field:

- Sourcecycled/live ingestion still does not update StoryGraph records.
- Story clustering, edge classification, and update classification are not
  implemented.
- Existing imported `ContentItem`s are not selectable from the app as source
  contributions.
- Reconciliation remains a queue state, not an active researcher workflow.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Let a user queue an existing imported `ContentItem` as a Global Wire source
  contribution by ID, preserving ownership and avoiding direct platform story
  mutation. This closes the product API side of "uploaded/user-provided
  sources -> contribution queue" before tackling sourcecycled clustering.

## Overnight Checkpoint - Imported Source Contribution Path - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `35068316314c6c3c5c776de4a0ef921caa3a517e`
(`feat: queue imported global wire sources`)

What changed:

- `POST /api/global-wire/contributions` now accepts `source_content_id`.
- Source and counter-source contributions may now reference an existing
  owner-scoped `ContentItem` instead of forcing the route to create a new text
  SourceItem from the contribution body.
- Existing `ContentItem` references are validated against the authenticated
  owner before they can enter the Global Wire contribution queue.
- Cross-owner source reuse is rejected with a bad request response; the platform
  story and source manifest remain unmutated.
- Runtime and deployed Playwright proof now create an imported `ContentItem`,
  queue it as a Global Wire source contribution, and verify the returned
  contribution preserves the imported `source_content_id`.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `35068316314c6c3c5c776de4a0ef921caa3a517e` to `origin/main`.
- GitHub Actions CI run `27081835814` completed successfully, including
  runtime shards, non-runtime tests, Go vet/build, and staging deploy.
- FlakeHub publish run `27081835810` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `35068316314c6c3c5c776de4a0ef921caa3a517e`, deployed at
  `2026-06-07T03:46:26Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped when run from
  `frontend/`.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed when run from `frontend/`, including:
  - durable StoryGraph provenance;
  - platform source `ContentItem` lookup;
  - selected-style projection VText lookup;
  - user-owned fork VText creation;
  - contribution VText creation;
  - newly created source contribution `ContentItem` lookup;
  - imported `ContentItem` contribution by `source_content_id`;
  - durable pending researcher review queue state.

Belief-state changes:

- The contribution side now supports both typed user-authored source artifacts
  and user-imported source artifacts. That closes the API-level path from
  `uploaded/user-provided source -> contribution queue` without granting users
  authority to mutate platform StoryGraph nodes.
- The current system is now a credible low-resolution collaborative object, not
  just a mock: evidence, graph, Story VTexts, Style.vtext projections, app
  views, owner forks, owner source artifacts, and reconciliation-ready queues
  all exist through product APIs.
- The next highest-value realism gap has shifted from artifact topology to feed
  dynamics: live/sourcecycled evidence still does not create or update graph
  neighborhoods.

Remaining error field:

- Sourcecycled/live ingestion still does not update StoryGraph records.
- Story clustering, edge classification, and update classification are not
  implemented.
- The app does not yet expose a picker for existing imported `ContentItem`
  source contributions, even though the API path is proven.
- Reconciliation remains a queue state, not an active researcher workflow.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Inspect the existing sourcecycled/source service surface and choose the
  narrowest product-path bridge that can turn real source service output into
  StoryGraph candidate evidence or document why staging lacks the required live
  service. If sourcecycled is not deployably available, switch to a
  reconciliation workflow slice rather than faking live ingestion.

## Overnight Problem Checkpoint - Source Service Bridge Gap - 2026-06-07

mission status: `problem_documented_before_fix`

Evidence:

- `cmd/sourcecycled` exposes a standalone Source Service daemon with internal
  health, search, and item resolution endpoints:
  `/internal/source-service/health`, `/internal/source-service/search`, and
  `/internal/source-service/items/{id}`.
- `internal/sourceapi` defines source-service result identity as
  `target_kind=source_service_item` plus `item_id`, `source_id`, `fetch_id`,
  content hash, fetched/published times, URL, and bounded body text.
- `internal/runtime/tools_research.go` already has a researcher-only
  `source_search` tool that calls the configured Source Service and presents
  durable `source_service_item` evidence.
- VText source-entity handling already preserves `source_service_item:<id>`
  refs as citeable source entities.
- Global Wire currently has durable SourceItems, StoryGraph stories, projection
  VTexts, owner forks, and contribution queues, but it has no browser-public
  product API that turns Source Service results into Global Wire evidence or
  contribution candidates.

Problem:

- The next realism axis is not another seeded story. It is the bridge from
  real Source Service output into Global Wire's collaborative graph workflow.
- Directly wiring sourcecycled into platform story mutation would violate the
  non-oracle/news provenance invariant and the user-edit ownership invariant.
- A browser-public route must not expose internal source-service endpoints
  directly, and it must not claim live ingestion when the configured service is
  absent or empty on staging.

Intended fix shape:

- Add a narrow authenticated product route that searches the configured Source
  Service through the existing runtime client, converts selected results into
  owner-scoped `ContentItem`s with `source_service_item` provenance, and queues
  them as Global Wire source contribution candidates when requested.
- If Source Service is unconfigured or returns no evidence, surface that as
  explicit provenance-rich unavailability instead of falling back to seeded or
  invented data.
- Keep platform StoryGraph stories unchanged. Research/reconciliation can later
  decide whether a source-service contribution updates a neighborhood.

Remaining alternative:

- If staging lacks a deployable Source Service, the next ship-worthy slice
  should be an explicit reconciliation workflow over the existing contribution
  queue rather than a fake live-ingestion loop.

## Overnight Checkpoint - Source Service Bridge - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `c651e48acb20db99850dcf7d24673454e814de48`
(`feat: bridge source service into global wire`)

What changed:

- Added authenticated `POST /api/global-wire/source-search`.
- The route searches the configured Source Service through the existing runtime
  Source Service client.
- Source Service results are imported into deterministic owner-scoped
  `ContentItem`s with:
  - `source_type=source_service_item`;
  - `app_hint=global-wire`;
  - metadata schema `choir.global_wire_source_service_item.v1`;
  - preserved `source_service_item`, `source_id`, `fetch_id`, content hash,
    evidence level, fetched/published times, verticals, language, and region.
- When requested, the top imported result is queued as a normal Global Wire
  source contribution in `pending-researcher-review`.
- The route reports `unavailable` or `no-evidence` explicitly instead of
  falling back to seeded data.
- Platform StoryGraph stories remain unchanged; Source Service evidence enters
  only as owner-owned source artifacts and contribution candidates.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `c651e48acb20db99850dcf7d24673454e814de48` to `origin/main`.
- GitHub Actions CI run `27081983909` completed successfully, including
  runtime shards, non-runtime tests, Go vet/build, and staging deploy.
- FlakeHub publish run `27081983912` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `c651e48acb20db99850dcf7d24673454e814de48`, deployed at
  `2026-06-07T03:54:27Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped when run from
  `frontend/`.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed when run from `frontend/`.
- A one-off authenticated product-path probe against staging returned
  `status=ok` for `/api/global-wire/source-search` with query
  `port congestion`. It imported two live Source Service/GDELT
  `source_service_item` results as owner-scoped Global Wire `ContentItem`s and
  queued the top result as a pending source contribution. Example evidence:
  - `source=source_service_api`;
  - `source_id=gdelt:15min`;
  - first item `item_id=srcitem_35cc888d00facc96836e4bc1`;
  - first imported content item
    `global-wire-source-service-7920189f-b3f9-5f56-89aa-842e46944e9d`;
  - queued contribution
    `global-wire-contribution-8618b07a-efd9-4c44-bdc4-d118b66997d3`;
  - contribution state `pending-researcher-review`.

Belief-state changes:

- The deployed product now has a real Source Service-backed evidence path into
  Global Wire. This is no longer merely seeded or manually imported evidence.
- The path still preserves the core invariants: live source evidence becomes
  owner-owned artifacts and queued contributions, not platform story mutation.
- The current deployed object covers the whole spec trajectory at low but real
  resolution:
  live Source Service evidence -> owner-scoped SourceItems -> durable
  StoryGraph -> Story VTexts -> Style.vtext projections -> News app views ->
  user-owned edits/contributions -> research/reconciliation-ready state.

Remaining error field:

- Source Service evidence is searched and queued by product request; there is
  not yet an autonomous graph update/classification loop.
- Story clustering, graph edge classification, and update classification remain
  absent.
- Reconciliation remains a queue state, not an active researcher workflow that
  accepts/rejects/merges source-service contributions into graph neighborhoods.
- The app does not yet expose a visible Source Service search/picker control;
  the bridge is currently an API/proof path.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Implement the narrow reconciliation action surface: list queued Global Wire
  contributions with source artifacts, mark one as accepted/rejected with a
  reviewer note, and record a non-mutating reconciliation decision artifact.
  That increases the research/reconciliation axis without pretending the graph
  classifier is solved.

## Overnight Checkpoint - Reconciliation Decision Surface - 2026-06-07

mission status: `checkpoint_incomplete`

commit delivered: `494bd3621e30ebff0b690cb91d22a84c26af90cf`
(`feat: add global wire reconciliation decisions`)

What changed:

- Added `GlobalWireReconciliationDecision`, a durable owner-scoped decision
  artifact over a Global Wire contribution.
- Added `global_wire_reconciliation_decisions`.
- Added authenticated `GET /api/global-wire/reconciliation` to list queued
  contributions, attached source artifacts, and prior decisions.
- Added authenticated `POST /api/global-wire/reconciliation` to record an
  `accepted` or `rejected` decision with reviewer note.
- Accepted decisions update contribution state to
  `accepted-for-graph-review`; rejected decisions update state to
  `rejected-by-review`.
- Decisions do not mutate platform StoryGraph stories or source manifests.
- Runtime tests prove a source contribution can be listed with its SourceItem,
  accepted for graph review, and leave the StoryGraph manifest unchanged.
- Deployed Playwright proof now records a reconciliation decision over a
  user-owned contribution and verifies the source artifact remains attached.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `494bd3621e30ebff0b690cb91d22a84c26af90cf` to `origin/main`.
- GitHub Actions CI run `27082121549` completed successfully, including
  runtime shards, non-runtime tests, Go vet/build, and staging deploy.
- FlakeHub publish run `27082121570` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `494bd3621e30ebff0b690cb91d22a84c26af90cf`, deployed at
  `2026-06-07T04:01:51Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped when run from
  `frontend/`.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed when run from `frontend/`, including:
  - durable StoryGraph provenance;
  - platform source `ContentItem` lookup;
  - selected-style projection VText lookup;
  - user-owned fork VText creation;
  - contribution VText creation;
  - imported source contribution by `source_content_id`;
  - Source Service bridge accepted states;
  - reconciliation decision creation;
  - reconciliation list lookup with attached source artifact.

Belief-state changes:

- The system now has a low-resolution but deployed research/reconciliation
  state, not only a pending queue. A contribution can be reviewed, decided, and
  kept as a citeable artifact for later graph update work.
- The full trajectory remains invariant-preserving: user-owned edits,
  user-owned source artifacts, and reconciliation decisions do not mutate
  platform stories.
- The next missing realism is classification/promotion, not artifact topology:
  the product can ingest, queue, and decide; it does not yet compute graph edge
  updates or merge reviewer decisions into story neighborhoods.

Remaining error field:

- Reconciliation is API-backed but not yet visible as a first-class app control.
- Accepted decisions do not yet create graph update candidates, edge
  classifications, or amended source-neighborhood proposals.
- There is still no autonomous graph update/classification loop.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Add the smallest visible Global Wire reconciliation control: show queued
  source contributions with their attached source artifact, allow accept/reject,
  and reflect decision state without changing the StoryGraph manifest. Then use
  Browser/Playwright proof across the three themes.

## Overnight Checkpoint - Visible Reconciliation Controls - 2026-06-07

mission status: `checkpoint_incomplete`

deployed product commit: `37caf9b55296e898ed1cab31b77583ea11ae1189`
(`feat: show global wire reconciliation controls`)

latest proof commit on `origin/main`: `35bd0827`
(`test: pass reconciliation ids as object`)

What changed:

- Global Wire now loads the reconciliation endpoint for authenticated owners
  instead of showing only the raw contribution list.
- Queued contributions display attached source artifacts from
  `/api/global-wire/reconciliation`.
- The contribution rail exposes visible `Accept` and `Reject` controls.
- Accept/reject calls `POST /api/global-wire/reconciliation`, records a durable
  decision, updates contribution state, and refreshes the queue.
- The UI reflects accepted decisions without mutating the StoryGraph manifest.
- Deployed proof now verifies the visible reconciliation source card and Accept
  control before validating the durable decision through product APIs.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed before the UI commit.
- `npm run build` passed in `frontend/` before the UI commit and again after
  the proof-path adjustments.

What was proven on staging:

- Pushed `37caf9b55296e898ed1cab31b77583ea11ae1189` to `origin/main`.
- GitHub Actions CI run `27082226685` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27082226640` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox deployed at
  `37caf9b55296e898ed1cab31b77583ea11ae1189`, deployed at
  `2026-06-07T04:07:04Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped when run from
  `frontend/`.
- After proof-path fixes, `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against the deployed `37caf9b55296e898ed1cab31b77583ea11ae1189`
  staging build when run from `frontend/`.
- Latest test-only CI run `27082348661` completed successfully and skipped
  staging deploy by impact classification; FlakeHub run `27082348655`
  completed successfully.

Belief-state changes:

- The product slice is now owner-usable through the app surface: source-backed
  contributions can be created, inspected with their source artifact, and
  accepted/rejected for graph review.
- This is the first deployed end-to-end collaborative StoryGraph loop with a
  visible review action:
  source evidence -> contribution artifact -> source artifact -> reconciliation
  decision artifact -> queue state update.
- The system still correctly avoids platform story mutation during review.

Remaining error field:

- Accepted decisions do not yet generate graph update candidates or edge
  classification proposals.
- There is no autonomous feed-to-cluster-to-update loop.
- Source Service search is API/proof-backed, not yet visible as a search/picker
  in the app.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Add a graph-update-candidate layer fed by accepted reconciliation decisions:
  create a non-mutating candidate edge/source-neighborhood proposal that can be
  reviewed before any platform StoryGraph change.

## Problem Checkpoint - Missing Graph Update Candidate Layer - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Accepted reconciliation decisions are durable review artifacts, but they do
  not yet create a reviewable StoryGraph update candidate.
- This leaves the system with evidence ingestion, contribution, and decision
  state, but no non-mutating bridge from accepted evidence into source
  neighborhood semantics, edge classification, projection revision needs, or
  source-manifest proposal state.

Evidence:

- The current reconciliation path records a
  `global_wire_reconciliation_decisions` row and updates the contribution
  research state to `accepted-for-graph-review`.
- Runtime tests prove the StoryGraph manifest remains unchanged after review,
  which protects the invariant but also shows the missing next artifact.
- The spec's ingestion loop requires classifying updates as source manifest
  update, claim change, contradiction, related story edge, projection revision,
  or front-page prominence change before a platform StoryGraph update.

Belief-state update:

- The correct next homotopy step is not to mutate the platform StoryGraph.
- The correct next artifact is an owner-scoped, durable graph-update candidate
  produced from an accepted reconciliation decision and visible in the app.
- Candidate state should be reviewable and cite its source contribution,
  source content item, reconciliation decision, and StoryGraph story id.

Remaining error field:

- No candidate table, API response, or UI surface exists yet.
- No proof currently shows accepted contribution -> candidate proposal ->
  unchanged StoryGraph manifest.

Next executable probe:

- Add a graph-update candidate table/model/API path generated by accepted
  reconciliation decisions, expose candidates in the reconciliation surface, and
  prove the StoryGraph manifest remains unchanged while the candidate captures
  source-neighborhood semantics.

## Overnight Checkpoint - Graph Update Candidates - 2026-06-07

mission status: `checkpoint_incomplete`

deployed product commit: `f883b2b61e54219225cbec0a178c41bb8635084e`
(`feat: add global wire graph update candidates`)

What changed:

- Added durable `global_wire_graph_update_candidates` records.
- Accepted reconciliation decisions now create a non-mutating graph-update
  candidate tied to owner id, story id, contribution id, decision id, and source
  content id.
- Candidate records classify the proposed StoryGraph effect by candidate kind,
  source tier, edge kind, projection action, review status, and rationale.
- `/api/global-wire/reconciliation` now returns graph-update candidates on list
  and includes a candidate in accepted decision responses.
- The Global Wire contribution rail now shows the candidate below an accepted
  decision, separating review state from platform StoryGraph mutation.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `f883b2b61e54219225cbec0a178c41bb8635084e` to `origin/main`.
- GitHub Actions CI run `27082510021` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27082510019` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `f883b2b61e54219225cbec0a178c41bb8635084e`, deployed at
  `2026-06-07T04:22:24Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against staging and verified visible graph candidate semantics plus API
  candidate lineage.

Belief-state changes:

- The research/reconciliation path now has an explicit non-mutating bridge into
  StoryGraph evolution:
  source evidence -> user-owned contribution -> reconciliation decision ->
  graph-update candidate.
- This preserves the core invariant: user contributions and accepted decisions
  still do not mutate platform stories or manifests.
- The remaining missing realism has moved from "decision has nowhere to go" to
  "source discovery is still mostly API/proof-backed and not an owner-visible
  app workflow."

Remaining error field:

- Source Service search/import is still not visible as an app search/picker.
- Candidate review does not yet promote to a platform StoryGraph process.
- There is no autonomous feed-to-cluster-to-update loop.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Add a visible source search/import control backed by
  `/api/global-wire/source-search`, with explicit unavailable/no-evidence states
  and queue-top-result behavior that creates a source-backed contribution.

## Overnight Checkpoint - Visible Source Search - 2026-06-07

mission status: `checkpoint_incomplete`

deployed product commit: `2246b9f468c511bde4af59534d34f91ff29a9531`
(`feat: expose global wire source search`)

What changed:

- Added a visible Source Service search control in the Global Wire contribution
  rail.
- Search calls `/api/global-wire/source-search` with the selected StoryGraph id
  and optional queue-top-result behavior.
- The app now shows explicit `ok`, `no-evidence`, `unavailable`, query-required,
  and sign-in-required states instead of silently falling back to seeded
  evidence.
- Imported source results display as source artifacts when Source Service
  evidence is available.
- If the backend queues the top result, the contribution/reconciliation queue is
  refreshed so the imported SourceItem can move through review.

What was proven locally:

- `npm run build` passed in `frontend/`.
- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.

What was proven on staging:

- Pushed `2246b9f468c511bde4af59534d34f91ff29a9531` to `origin/main`.
- GitHub Actions CI run `27082604455` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27082604446` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `2246b9f468c511bde4af59534d34f91ff29a9531`, deployed at
  `2026-06-07T04:27:20Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against staging and verified the visible source search status path
  alongside StoryGraph, VText, Style.vtext, contribution, reconciliation, and
  graph-candidate behavior.

Belief-state changes:

- The live/source-service evidence path is now owner-visible in the app, not
  only API/proof-backed.
- The current object now covers manual source discovery/import, durable
  StoryGraph, Story VTexts, Style.vtext projections, news app views,
  user-owned contributions, reconciliation decisions, and non-mutating graph
  update candidates.
- Remaining realism is now primarily automation and downstream consumption:
  autonomous ingestion/classification, candidate promotion/review workflow, and
  Ask/Autoradio traversal hooks.

Remaining error field:

- Candidate review does not yet promote to a platform StoryGraph process.
- There is no autonomous feed-to-cluster-to-update loop.
- Autoradio/Ask hooks remain outside this slice.

Next executable probe:

- Add non-oracle Ask/Autoradio hooks that hand the selected story/projection,
  source manifest, and related story neighborhood to existing app routes or
  product events without inventing answers or mutating the StoryGraph.

## Overnight Checkpoint - Ask And Autoradio Hooks - 2026-06-07

mission status: `checkpoint_incomplete`

deployed product commit: `a0b12df9e3fd0cfb26ce3d0e2cc9ff51d53aeb68`
(`feat: add global wire ask hooks`)

What changed:

- Added `Ask Choir` and `Autoradio` actions to the Global Wire story reader.
- Both actions build a grounded prompt from the selected StoryGraph id,
  headline, change/tension state, selected `Style.vtext`, projection text,
  claims, full source manifest tiers, related Story VTexts, and a guardrail
  against mutation or invented facts.
- The actions submit through the browser-public `/api/prompt-bar` product path
  and display the returned submission handle.
- The News app does not synthesize an answer inline and does not mutate the
  StoryGraph; synthesis remains downstream in the ordinary Choir/VText route.

What was proven locally:

- `npm run build` passed in `frontend/`.
- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire|TestHandlePromptBar'`
  passed.

What was proven on staging:

- Pushed `a0b12df9e3fd0cfb26ce3d0e2cc9ff51d53aeb68` to `origin/main`.
- GitHub Actions CI run `27082694589` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27082694588` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `a0b12df9e3fd0cfb26ce3d0e2cc9ff51d53aeb68`, deployed at
  `2026-06-07T04:32:15Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against staging and verified the `Ask Choir` prompt-bar handoff
  included StoryGraph id, source manifest, and `Style.vtext` context.

Belief-state changes:

- The app now has real hooks for asking about a story and requesting a radio
  brief without treating Global Wire itself as an answer oracle.
- The deployed product trajectory now covers:
  live/source-service evidence import -> durable StoryGraph -> Story VTexts ->
  Style.vtext projections -> News app views -> user-owned forks/contributions
  -> reconciliation decisions -> graph-update candidates -> Ask/Autoradio
  handoff.

Remaining error field:

- Candidate review does not yet promote to a platform StoryGraph process.
- There is still no autonomous feed-to-cluster-to-update loop.
- The current Ask/Autoradio hook is a prompt-bar/VText handoff, not a dedicated
  audio playback or radio scheduling subsystem.

Highest-impact remaining uncertainty:

- Whether the next correct realism step is a low-resolution platform
  reconciliation process that can safely promote graph-update candidates, or an
  autonomous ingestion/classification worker that creates candidates from live
  SourceItems before human review.

Next executable probe:

- Design and implement the smallest platform-process-shaped candidate promotion
  or autonomous classification step that preserves the invariant that arbitrary
  user edits never silently mutate platform stories.

## Problem Checkpoint - Missing Platform Candidate Promotion - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Global Wire now produces graph-update candidates from accepted reconciliation
  decisions, but candidates remain review artifacts only.
- There is no explicit platform-process-shaped promotion record or endpoint
  that can take a reviewed candidate, record the platform review decision, and
  apply a bounded StoryGraph update.

Evidence:

- `global_wire_graph_update_candidates` records carry source tier, edge kind,
  projection action, status, and lineage.
- The app can show a candidate as `candidate-review`, but no route exists to
  promote, reject, or apply the candidate through a platform review step.
- This keeps the invariant safe, but leaves the trajectory short of
  "possible platform reconciliation later" from the spec.

Belief-state update:

- The next topology-preserving step is not arbitrary user merge.
- The next real artifact is a platform review/promotion decision that is
  owner-scoped, lineage-rich, and explicit.
- The smallest honest promotion can apply only a bounded source-manifest update
  from the candidate's cited SourceItem, leaving full claim/edge/projection
  regeneration as later work.

Remaining error field:

- No promotion decision table/model exists.
- No API or app control can mark a candidate promoted or rejected.
- No proof shows a platform-process endpoint applying a candidate source to the
  StoryGraph manifest while preserving candidate and decision provenance.

Next executable probe:

- Add a candidate promotion decision artifact and endpoint. Promoting a
  candidate may append its cited SourceItem to the candidate's source tier in
  the durable StoryGraph manifest if absent, update candidate status, and
  record the platform review note. Rejecting a candidate should update only
  candidate status and record the decision.

## Problem Checkpoint - Missing Autonomous Evidence Candidate Loop - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Global Wire can import Source Service evidence when a user searches, can queue
  that evidence for reconciliation, can create graph-update candidates from
  accepted decisions, and can promote candidates through an explicit platform
  review step.
- The system still lacks even a low-resolution product path for "new source
  evidence arrived, classify it against a StoryGraph node, and create a
  reviewable graph-update candidate."
- Without that loop, the trajectory remains short of the spec's 24/7 ingestion
  and publication shape even though the manual contribution/reconciliation path
  is now platform-shaped.

Evidence:

- `/api/global-wire/source-search` imports Source Service results and can queue
  the top result as a pending contribution.
- `/api/global-wire/reconciliation` creates candidates only after an explicit
  accepted review decision over an existing contribution.
- `/api/global-wire/graph-candidates` can promote/reject an existing candidate,
  but no route currently connects source refresh/classification directly to a
  candidate-ready review artifact.

Belief-state update:

- The next topology-preserving move is not a blind auto-publish worker.
- The next honest overnight resolution is a source-refresh/classification
  product path that imports Source Service evidence, records a bounded
  classification decision, and emits a non-mutating graph-update candidate for
  later platform review.
- This preserves the invariant that source arrival never silently rewrites the
  platform StoryGraph.

Remaining error field:

- No durable ingestion/classification run record exists.
- No API or app control can refresh a story from Source Service and return the
  resulting contribution, decision, candidate, and source artifact lineage as a
  single reviewable product-path object.
- No staging proof shows live Source Service evidence progressing to candidate
  review without a human manually assembling every intermediate artifact.

Next executable probe:

- Add a bounded Global Wire source-refresh endpoint and UI control. It should
  load a StoryGraph node, search Source Service using the story headline or
  supplied query, import the top SourceItem, create a contribution, record an
  accepted classification/reconciliation decision whose note names the refresh
  route, create a graph-update candidate, and return all lineage without
  mutating the StoryGraph manifest.

## Overnight Checkpoint - Source Refresh Candidate Loop - 2026-06-07

mission status: `checkpoint_incomplete`

What shipped:

- Added `GlobalWireSourceRefreshRun`, a durable owner-scoped record for a
  bounded source-ingestion/classification pass over one StoryGraph node.
- Added `/api/global-wire/source-refresh`.
- The route loads the selected StoryGraph node, searches Source Service using
  the story headline or supplied query, imports the top result as a
  `source_service_item` ContentItem, creates an accepted contribution and
  reconciliation decision, creates a non-mutating graph-update candidate, and
  records all lineage in the refresh run.
- Reconciliation state now returns `refreshes` alongside contributions,
  decisions, graph-update candidates, and promotion decisions.
- The Global Wire app now exposes a visible `Refresh story evidence` control
  and shows recent source-refresh runs near the contribution/reconciliation
  surface.
- Source refresh does not mutate the platform StoryGraph manifest. Promotion
  remains a separate explicit platform-review action.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `c11e36e781208766316a9c18c736748f7e147104` to `origin/main`.
- GitHub Actions CI run `27083023728` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27083023740` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `c11e36e781208766316a9c18c736748f7e147104`, deployed at
  `2026-06-07T04:49:21Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against staging and verified the source-refresh endpoint returns a
  durable refresh run. When Source Service returns evidence, the proof verifies
  SourceItem import, accepted contribution, accepted decision, graph-update
  candidate, and refresh-run candidate lineage.

Belief-state changes:

- The deployed product trajectory now covers:
  live/source-service evidence import -> durable SourceItem -> source-refresh
  run -> accepted contribution/decision -> graph-update candidate -> explicit
  platform promotion -> StoryGraph source manifest update.
- The refresh path is a low-resolution version of the 24/7 ingestion loop, but
  it is still product-path and provenance-rich rather than a mock feed.
- The candidate/promotion split continues to preserve the invariant that source
  arrival and arbitrary user contribution do not silently mutate platform
  stories.

Remaining error field:

- Source refresh is still request-triggered, not a scheduled 24/7 worker.
- The source-refresh classifier is deliberately low resolution: top Source
  Service result becomes a supporting-source candidate; no claim extraction,
  contradiction classification, source-standing scoring, or story clustering
  exists yet.
- Promotion updates the source manifest only. It does not create projection
  review jobs, update projection Story VTexts, revise front-page prominence, or
  publish an update feed entry.
- Autoradio remains a prompt-bar/VText handoff, not a dedicated audio playback
  or scheduling subsystem.

Highest-impact remaining uncertainty:

- Whether the next realism step should create projection review/revision
  records after StoryGraph evidence changes, or invest in a broader scheduled
  source-refresh worker.

Next executable probe:

- Add a projection-review artifact emitted by graph candidate promotion when
  the candidate's projection action requires review. The artifact should link
  story, candidate, promotion, source content, affected Style.vtext source, and
  status without pretending to regenerate prose automatically.

## Problem Checkpoint - Missing Projection Review After Graph Update - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- The platform can now promote a reviewed graph-update candidate into the
  StoryGraph source manifest, but that update does not create any durable
  projection-review or projection-revision state.
- The spec's publication loop requires StoryGraph updates to feed projection
  jobs and Story VText revisions. Current behavior stops after manifest update
  plus a human-readable promotion record.

Evidence:

- `GlobalWireGraphUpdateCandidate` carries `projection_action`, usually
  `projection-review-required`.
- `/api/global-wire/graph-candidates` records promotion decisions and can append
  the cited SourceItem to the manifest tier.
- No table/model/API response records that any Style.vtext projection or Story
  VText now needs review because of the promoted source.

Belief-state update:

- The right next slice is not automatic story rewriting.
- The smallest invariant-preserving artifact is a projection-review record:
  citeable, owner-scoped, linked to the promoted graph candidate and Style.vtext
  source, and visible in reconciliation state.
- Actual VText regeneration can remain future work; the product should first
  expose that evidence changes imply projection obligations.

Remaining error field:

- No durable projection-review table/model exists.
- No promotion response includes projection review obligations.
- The app cannot show that a promoted source changed the downstream projection
  queue.

Next executable probe:

- Add `GlobalWireProjectionReview` records created during graph-candidate
  promotion for candidates whose `projection_action` is
  `projection-review-required`. Return and display them in reconciliation state
  and prove through runtime and staging product-path tests.

## Overnight Checkpoint - Projection Review Queue - 2026-06-07

mission status: `checkpoint_incomplete`

What shipped:

- Added `GlobalWireProjectionReview`, a durable owner-scoped artifact recording
  that a promoted StoryGraph source update may require review of one or more
  Style.vtext projections.
- Added `global_wire_projection_reviews` with story, candidate, promotion,
  source content, Style.vtext source, projection action, status, and rationale
  lineage.
- `/api/global-wire/reconciliation` now returns `projection_reviews` alongside
  contributions, source items, reconciliation decisions, graph candidates,
  promotion decisions, and source-refresh runs.
- `/api/global-wire/graph-candidates` now creates projection-review records
  when a promoted candidate carries `projection-review-required`.
- Promotion creates one projection-review obligation per attached Style.vtext
  source, preserving the fact that a StoryGraph evidence update may affect
  each projection differently.
- The Global Wire app now shows projection-review obligations below promoted
  candidates.
- The implementation does not regenerate or rewrite projection Story VTexts
  automatically. It records the downstream obligation first.

What was proven locally:

- `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  passed.
- `npm run build` passed in `frontend/`.

What was proven on staging:

- Pushed `5f9b22f1318de293b186627205ec95be49246657` to `origin/main`.
- GitHub Actions CI run `27083146467` completed successfully, including
  runtime shards, non-runtime tests, frontend build, Go vet/build, and staging
  deploy.
- FlakeHub publish run `27083146469` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `5f9b22f1318de293b186627205ec95be49246657`, deployed at
  `2026-06-07T04:55:42Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  passed 4 public/theme tests with the guarded auth proof skipped.
- `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  passed against staging and verified promoted candidates create visible
  `projection-review-required` records returned from reconciliation state.

Current delivered trajectory:

```text
Source Service evidence / user source artifact
-> owner-scoped ContentItem SourceItem
-> source refresh or user contribution
-> reconciliation decision
-> graph-update candidate
-> explicit platform promotion/rejection
-> bounded StoryGraph source-manifest update
-> projection-review obligations for Style.vtext projections
-> Ask/Autoradio prompt-bar handoff with StoryGraph and Style.vtext context
```

Invariants preserved:

- User edits/contributions remain user-owned and do not mutate platform stories.
- Platform StoryGraph mutation happens only through an explicit graph-candidate
  promotion step.
- Source refresh creates candidates, not silent story rewrites.
- Style.vtext remains a citeable source artifact attached to projection
  obligations, not hidden app config.
- The app views and proof continue to cover Future Noir, Carbon Kintsugi, and
  London Salmon.

Remaining error field:

- Projection-review records do not yet create revised ProjectionStory VTexts.
- Promotion does not update existing projection relation text or VText
  revisions.
- Source refresh is request-triggered and low-resolution, not a scheduled 24/7
  ingestion worker.
- No claim extraction, contradiction classifier, story clustering, front-page
  prominence revision, or publication/update feed exists yet.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Add a bounded projection-revision draft path. A reviewer should be able to
  select a projection-review record, create a new ordinary ProjectionStory
  VText draft that cites the StoryGraph, promoted SourceItem, promotion
  decision, and Style.vtext source, and mark the review as `draft-created`
  without mutating the platform story or pretending the draft is final
  publication.

## Problem Checkpoint - Missing Projection Draft Path - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Projection-review records now prove that a promoted StoryGraph evidence
  change creates downstream Style.vtext review obligations, but there is no
  product path that turns one obligation into an ordinary ProjectionStory VText
  draft.
- Without that path, the system still stops short of the spec's loop from
  StoryGraph update to projection jobs to Story VText revisions.

Evidence:

- `GlobalWireProjectionReview` records carry story, candidate, promotion,
  source content, Style.vtext source, projection action, status, and rationale.
- `/api/global-wire/reconciliation` lists projection reviews and the app shows
  them under promoted candidates.
- No endpoint or app control creates a VText draft from a projection-review
  record, and no review state can point to a draft VText.

Belief-state update:

- The next topology-preserving step is not automatic publication and not a
  hidden prose rewrite.
- The next real artifact should be a draft ProjectionStory VText: normal VText,
  appagent-authored, owner-scoped, citation-rich, and linked back to the
  projection review.
- The projection review should move to `draft-created`; platform/public story
  state and user-owned edits remain separate.

Remaining error field:

- No projection-review draft endpoint exists.
- No projection-review state can record a draft Story VText doc id.
- The app cannot create or open a projection draft from a review obligation.
- No staging proof shows promoted evidence leading to an ordinary VText draft
  while preserving platform-story invariants.

Next executable probe:

- Add a bounded projection-draft endpoint and app control. The endpoint should
  create an ordinary VText document from a projection-review record, cite the
  StoryGraph, promoted source ContentItem, promotion decision, and Style.vtext
  source, update the review to `draft-created`, and return the draft doc id for
  opening through the existing VText editor path.

## Problem Checkpoint - Projection Draft Foreground Occludes Continuation - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Deployed authenticated product-path proof can create a projection-review
  draft VText, but the newly opened foreground VText window occludes the Global
  Wire window before the same proof continues into the Ask Choir handoff.
- The failure is a product-path continuity gap in the acceptance trajectory:
  the draft exists and opens, but the proof does not yet demonstrate returning
  from that draft view to the Global Wire control surface for the next action.

Evidence:

- Staging is serving behavior commit
  `9333ce595f465baa89f6fbfe497e1f9b8ac8f052`.
- Public deployed proof passed
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`.
- Authenticated deployed proof failed at
  `tests/global-wire-app.spec.js:286-289`: Playwright resolved
  `[data-global-wire-ask-choir]`, but the foreground VText window title
  `Draft projection: Port backlog recedes as carrier...` intercepted pointer
  events.

Belief-state update:

- The projection draft slice should remain: it creates an ordinary editable
  VText and preserves the non-publication invariant.
- The proof path needs an owner-realistic return/focus step, or the app should
  expose a clearer continuation path after opening a projection draft.
- This is not evidence that the projection-review draft endpoint failed.

Remaining error field:

- No deployed proof yet shows projection draft creation and subsequent Ask
  Choir continuation in one authenticated owner path.
- The app has no Global Wire-specific "return to review" affordance from the
  opened projection draft.

Next executable probe:

- Repair the authenticated acceptance path by using an owner-realistic window
  return/focus operation, or add a small product affordance that keeps the
  Global Wire continuation reachable after opening a projection draft. Then
  rerun deployed authenticated proof before claiming the slice.

## Problem Checkpoint - Projection Draft Button Hit Target Blocked - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- After staging deployed candidate/review provenance attributes, the
  authenticated proof could target the exact graph candidate created during the
  run, but a normal browser click on that candidate's `Draft VText` control
  could not complete.
- Playwright resolved the visible, enabled button, but the enclosing Global
  Wire contribution section intercepted pointer events. That means the product
  surface is not yet honestly owner-clickable at this resolution.

Evidence:

- Staging is serving commit
  `79eafd3ad77933fb814055458f61fbcfed59aa53`.
- Public deployed proof passed
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`.
- Authenticated deployed proof failed at
  `tests/global-wire-app.spec.js:276-281`: the target was
  `[data-global-wire-create-projection-draft]` with a concrete
  `data-global-wire-projection-review-id`, but click retry logs showed
  `<section data-global-wire-contribution>` intercepting pointer events.

Belief-state update:

- The endpoint and data model can create projection draft VTexts, but the
  deployed product path still needs a real clickable control from the
  reconciliation list.
- The acceptance proof should continue using normal Playwright clicks for this
  step; forcing the click or calling the handler directly would hide the
  owner-path defect.

Remaining error field:

- No deployed authenticated proof yet shows the owner clicking `Draft VText`
  from the exact promoted candidate and receiving the draft VText.
- The Global Wire contribution/reconciliation layout has a hit-testing defect
  around nested projection-review buttons.

Next executable probe:

- Fix the Global Wire contribution layout or button stacking so projection
  draft controls are normally clickable. Then rerun local build and deployed
  authenticated proof without force-clicking or direct DOM handler invocation.

## Checkpoint - Projection Draft VText Slice Proven - 2026-06-07

mission status: `checkpoint_incomplete`

What shipped:

- A promoted Global Wire graph candidate now creates durable
  `GlobalWireProjectionReview` obligations for the story's Style.vtext
  projections.
- A reviewer can click `Draft VText` on a projection-review obligation to
  create an ordinary editable VText draft. The review moves to `draft-created`
  and records `draft_story_doc_id`.
- The draft is appagent-authored, citation-rich, linked through
  `global-wire/projection-drafts/<review-id>.vtext`, and explicitly says it is
  a review draft rather than platform publication.
- The News app now exposes candidate and projection-review provenance
  attributes for exact product-path proof, and the contribution list no longer
  nests an internal scroll region that blocked normal clicks.
- The authenticated proof returns to Global Wire through the owner-visible
  window tray after opening the draft VText, then submits the Ask Choir handoff.

Evidence:

- Behavior commit:
  `73af4264daa838c47baab5560364b98ba2dca51a`.
- Required predecessor implementation commit:
  `9333ce595f465baa89f6fbfe497e1f9b8ac8f052`.
- CI run `27083667554`: success, including frontend build, runtime shards,
  Go vet/build, non-runtime tests, integration smoke, and staging deploy.
- FlakeHub run `27083667576`: success.
- Staging health at `2026-06-07T05:23:20Z` reported proxy and sandbox
  `deployed_commit` =
  `73af4264daa838c47baab5560364b98ba2dca51a`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  with 1 passed.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.

Invariants preserved:

- Every story/projection artifact created here is a normal editable VText.
- User edits and user contributions remain owner-scoped artifacts and do not
  mutate platform stories.
- Platform StoryGraph mutation still happens only through explicit graph
  candidate promotion.
- Style.vtext remains a citeable source artifact on projection-review
  obligations; it is not hidden app configuration.
- News remains non-oracle and provenance-rich: the proof traverses source
  content, contribution, reconciliation decision, graph candidate, promotion,
  projection review, draft VText, and Ask Choir prompt context.
- Graph nodes remain story headlines with source-neighborhood semantics.
- The app renders the core Global Wire views in Future Noir, Carbon Kintsugi,
  and London Salmon.

Belief-state update:

- The current ship-worthy slice covers this low-resolution but real product
  loop:
  source evidence -> user-owned contribution -> research/reconciliation queue
  -> graph candidate -> promotion -> projection-review obligation -> ordinary
  ProjectionStory draft VText -> return to News app -> Ask Choir handoff.
- The draft path is intentionally not automatic publication. It proves the
  editable artifact and provenance topology needed before publication/update
  semantics can be made honest.
- Dirty staging data made ordering assumptions invalid; future proofs should
  target exact ids from product responses instead of first visible records.

Remaining error field:

- Projection drafts do not yet revise or publish the platform ProjectionStory
  relation.
- There is no reviewer approval/publish/update-feed flow for a draft
  projection.
- Source refresh is still request-triggered and low-resolution, not a
  scheduled ingestion worker.
- No claim extraction, contradiction classifier, story clustering, front-page
  prominence revision, or research reconciliation workbench exists yet.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Add a reviewer-controlled projection draft review/publish path: open a
  `draft-created` projection review, compare it against the current
  ProjectionStory VText and Style.vtext source, approve/reject it, and on
  approval create the next normal ProjectionStory VText revision or update-feed
  candidate without mutating user-owned forks.

## Problem Checkpoint - Projection Drafts Do Not Update Projection Relation - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- The proven projection-draft path creates an ordinary editable VText and links
  it to a projection review, but the durable
  `global_wire_story_projections` relation still points at the original
  ProjectionStory VText and projection text.
- As a result, the system has reviewable projection drafts but no product path
  for a reviewer to approve/reject a draft and make the platform projection
  relation advance to a new normal Story VText revision.

Evidence:

- `GlobalWireProjectionReview` has `draft_story_doc_id` but no approved
  revision/doc fields or publication/update-feed state.
- `/api/global-wire/projection-reviews` currently handles only draft creation.
- `global_wire_story_projections` stores `story_vtext_doc_id` and
  `projection_text`, but there is no transition from a `draft-created` review
  into a new projection revision or relation update.

Belief-state update:

- The next topology-preserving improvement is not public publishing and not
  automatic story mutation.
- The next real state transition should be reviewer-controlled approval over a
  projection review: approve creates a new revision on the existing
  ProjectionStory VText, updates the projection relation text/doc ref, and
  marks the review `approved`; reject records review state without changing the
  projection relation.

Remaining error field:

- No projection-review approval endpoint exists.
- Projection reviews cannot record the approved projection Story VText doc or
  revision.
- No app control or staging proof demonstrates a draft advancing the durable
  StoryGraph + Style.vtext projection relation.

Next executable probe:

- Add projection-review approval/rejection as an owner-scoped product-path API
  and UI control. Approval should create a normal VText revision with citations
  and provenance, update the existing `GlobalWireStoryProjection`, and preserve
  user-owned fork/edit invariants.

## Projection Approval Relation Slice Proven - 2026-06-07

mission status: `checkpoint_incomplete`

What changed:

- Added reviewer-controlled approval and rejection for Global Wire projection
  reviews.
- Approval now creates a normal VText revision on the existing
  ProjectionStory document, carries draft citations and provenance metadata,
  updates the durable `global_wire_story_projections` relation, and records the
  approved Story VText doc/revision on the projection review.
- Rejection records review state without changing the platform projection
  relation.
- The Global Wire app now exposes approve/reject controls on draft-created
  projection reviews and refreshes the News projection surface after approval.

Evidence:

- Problem-first documentation commit:
  `a84f0014d86a4fd18019f1878ea21e4bb2a9d8b6f`.
- Behavior commit:
  `d549a94cd9e0276a713903bcf83d913358c683c8`.
- CI run `27083852391`: success, including frontend build, runtime shards,
  Go vet/build, non-runtime tests, integration smoke, and staging deploy.
- FlakeHub run `27083852398`: success.
- Staging health at `2026-06-07T05:33:13Z` reported proxy and sandbox
  `deployed_commit` =
  `d549a94cd9e0276a713903bcf83d913358c683c8`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  with 1 passed. The path exercised owner contribution, reconciliation,
  StoryGraph candidate promotion, projection draft creation, projection draft
  approval, approved ProjectionStory VText revision retrieval, and News story
  projection visibility.

Invariants preserved:

- Every story/projection artifact remains a normal editable VText.
- User-owned edits, forks, and contributions remain separate from platform
  stories and are not mutated by approval.
- Approval advances the platform projection relation only through an explicit
  reviewer-controlled product transition.
- Style.vtext remains a citeable source artifact on the projection relation
  and review record.
- News remains non-oracle and provenance-rich: approval carries source,
  candidate, promotion, style, draft, and review provenance into the approved
  revision metadata.
- Graph nodes remain story headlines with source-neighborhood semantics.
- The Global Wire app still renders the core views in Future Noir, Carbon
  Kintsugi, and London Salmon.

Belief-state update:

- The current proven trajectory is now:
  live/source evidence -> StoryGraph story/source neighborhood -> user-owned
  contribution -> reconciliation decision -> graph candidate -> promotion ->
  projection-review obligation -> ordinary ProjectionStory draft VText ->
  reviewer approval -> ordinary ProjectionStory revision -> durable
  StoryGraph + Style.vtext projection relation -> News app view -> Ask Choir
  handoff.
- This closes the largest prior topology gap: projection review is now a
  provenance-preserving state transition rather than a detached draft artifact.
- The approved projection text is intentionally low-resolution and still reads
  like a review artifact. The next quality axis should improve source refresh,
  classification, style composition, or editorial update semantics without
  weakening the VText/provenance invariants.

Remaining error field:

- Source refresh is still request-triggered and low-resolution, not a
  scheduled 24/7 ingestion worker with source-service recency semantics.
- There is still no claim extraction, contradiction classifier, story
  clustering, front-page prominence revision, or dedicated research
  reconciliation workbench.
- Style.vtext can be selected and cited but cannot yet be composed, replaced,
  forked, or proposed through a full style-revision workflow.
- There is no dedicated update-feed/newsletter/publication queue beyond the
  approved projection relation becoming visible in the News app.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Use a cognitive-transform pass before choosing the next overnight axis. The
  highest-value candidates are:
  source-service-backed scheduled ingestion and update classification,
  Style.vtext composition/replacement workflow, or claim extraction plus
  reconciliation-ready research workbench.

## Problem Checkpoint - Source Refresh Lacks Update Classification - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- The current Source Service refresh path imports live/source-service evidence
  and creates a non-mutating graph-update candidate, but every successful
  refresh collapses into the same `candidate-review` state.
- The spec requires the ingestion/publication loop to classify what a new
  SourceItem means before it can honestly drive StoryGraph, projection, front
  page, and research behavior.
- Without a durable classification, reviewers cannot distinguish a simple
  source-manifest update from a claim change, contradiction, related-story
  edge, projection revision, prominence change, or no visible change.

Evidence:

- `HandleGlobalWireSourceRefresh` records refresh status, provider, message,
  source, contribution, decision, and candidate refs.
- `GlobalWireSourceRefreshRun` has no update-classification field.
- `createGlobalWireSourceRefreshArtifacts` always creates a source-kind
  contribution, an accepted reconciliation decision, and a generic
  source-manifest graph candidate.

Belief-state update:

- The next topology-preserving improvement is a low-resolution update
  classifier, not an automatic story rewrite.
- Classification should be durable on refresh runs and should shape the
  generated contribution kind, graph candidate kind, source tier, edge kind,
  projection action, and reviewer message while preserving explicit platform
  review before StoryGraph mutation.

Remaining error field:

- No durable refresh classification exists.
- Source refresh cannot currently represent `no visible change`,
  `claim changed`, `contradiction added`, `related story edge added`,
  `projection revision required`, or `front-page prominence changed`.
- The Global Wire app does not surface refresh classification for reviewers.

Next executable probe:

- Add owner-scoped, Source Service-backed refresh classification with durable
  run fields and visible reconciliation/research queue semantics. The
  classifier can be heuristic at this resolution, but it must preserve the real
  ingestion loop topology and never mutate StoryGraph without platform review.

## Source Refresh Classification Slice Proven - 2026-06-07

mission status: `checkpoint_incomplete`

What changed:

- Added durable update-classification fields to Global Wire source refresh
  runs:
  `update_classification`, `storygraph_action`, and `projection_action`.
- Source Service-backed refresh now classifies the imported SourceItem before
  creating review artifacts.
- `no-visible-change` refreshes import and record the SourceItem but do not
  create contribution, reconciliation, or graph-candidate artifacts.
- Candidate-producing refreshes now shape contribution kind, candidate kind,
  source tier, edge kind, StoryGraph action, and projection action from the
  classification.
- The Global Wire app surfaces refresh classification/action fields in the
  research queue and reloads the durable StoryGraph correctly after projection
  approval.

Evidence:

- Problem-first documentation commit:
  `507408d4cbf275b0e4f77f331338b5865fadfb02`.
- Behavior commit:
  `91f3b1c36dcd22c441e439dc96b0a5ab1f96af58`.
- CI run `27084033464`: success, including frontend build, runtime shards,
  Go vet/build, non-runtime tests, integration smoke, and staging deploy.
- FlakeHub run `27084033473`: success.
- Staging health at `2026-06-07T05:42:49Z` reported proxy and sandbox
  `deployed_commit` =
  `91f3b1c36dcd22c441e439dc96b0a5ab1f96af58`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  with 1 passed. The path now accepts source-refresh responses with durable
  classification fields, then continues through reconciliation, graph
  promotion, projection draft, projection approval, News visibility, and Ask
  Choir.

Invariants preserved:

- Source refresh classification does not mutate the StoryGraph directly.
- Platform StoryGraph mutation still requires explicit graph-candidate
  promotion.
- `no-visible-change` creates no fake review work.
- Candidate-producing classifications remain provenance-rich and owner-scoped
  until platform review.
- Projection changes still require projection review/draft/approval before the
  durable StoryGraph + Style.vtext relation advances.
- User-owned forks, edits, and contributions remain separate from platform
  stories.
- The app still renders core Global Wire views in Future Noir, Carbon Kintsugi,
  and London Salmon.

Belief-state update:

- The proven trajectory now includes a low-resolution but real ingestion
  classifier:
  Source Service SourceItem -> refresh classification -> optional
  research/reconciliation artifacts -> graph candidate -> platform promotion
  -> projection-review obligation -> normal ProjectionStory revision.
- This moves the object closer to the spec's 24/7 ingestion/publication loop
  without pretending to have full claim extraction or scheduled ingestion.
- The classifier is intentionally heuristic. It is a topology-preserving
  placeholder for stronger claim/event/entity extraction, not a final newsroom
  judgment engine.

Remaining error field:

- There is still no scheduled 24/7 source registry/fetch-cycle worker.
- Claim/event/entity extraction is still heuristic and not stored as a rich
  claim set with uncertainty/dispute state.
- Story clustering and related-story edge promotion are low-resolution.
- Front-page prominence classification exists as a candidate kind but does not
  yet apply a bounded prominence update on promotion.
- Style.vtext can be selected and cited, but not yet composed, replaced,
  forked, or proposed through a full style-revision workflow.
- There is no dedicated update-feed/newsletter/researcher publishing queue
  beyond the current reconciliation/projection-review queues.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Apply a cognitive-transform route choice before the next change. The highest
  value candidates are:
  add bounded prominence/related-edge application on graph promotion, add
  Style.vtext composition/replacement as citeable VText artifacts, or add a
  richer claim extraction/research-task record tied to refresh classifications.

## Problem Checkpoint - Classified Promotion Does Not Revise Story Semantics - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Source refresh can now classify a SourceItem as a claim change,
  contradiction, related-story edge, prominence change, source-manifest update,
  or no visible change.
- However, graph-candidate promotion still applies nearly every promoted
  candidate as a source-manifest append. It does not yet apply bounded changes
  to the StoryGraph's claim state, tension/change state, prominence, related
  Story VText refs, or PlatformStory VText revision.
- This leaves the ingestion loop partially real: classification exists, but
  platform review does not yet turn those classifications into the corresponding
  StoryGraph semantics required by the spec.

Evidence:

- `classifyGlobalWireSourceRefresh` produces classification-specific
  `candidate_kind`, `source_tier`, `edge_kind`, `storygraph_action`, and
  `projection_action`.
- `applyGlobalWireGraphCandidate` currently chooses the manifest tier, appends
  the source if missing, sets a generic source state, and writes the StoryGraph.
- Promotion does not create a new normal PlatformStory VText revision for the
  accepted StoryGraph update.

Belief-state update:

- The next topology-preserving improvement is not automatic ingestion
  publication. It is bounded platform-review application: once a reviewer
  promotes a classified candidate, that explicit platform process may revise
  the StoryGraph fields and PlatformStory VText that correspond to the
  classification.

Remaining error field:

- Promoted `claim-changed` candidates do not update claims/change state.
- Promoted `contradiction-added` candidates do not update tension/claim state
  beyond source tier.
- Promoted `front-page-prominence-changed` candidates do not revise prominence.
- Promoted `related-story-edge-added` candidates do not add a related Story
  VText edge.
- Promoted candidates do not create PlatformStory VText revisions.

Next executable probe:

- Make graph-candidate promotion classification-aware. Keep mutation bounded
  and reviewer-controlled, update the StoryGraph semantics, create a normal
  PlatformStory VText revision with provenance/citations, and preserve
  projection-review obligations for downstream Style.vtext projections.

## Classified Promotion PlatformStory Slice Proven - 2026-06-07

mission status: `checkpoint_incomplete`

What changed:

- Graph-candidate promotion now applies bounded StoryGraph semantics based on
  the promoted candidate classification.
- Promoted `claim-changed`, `contradiction-added`,
  `front-page-prominence-changed`, `related-story-edge-added`, and
  `source-manifest-update` candidates update the corresponding StoryGraph
  claim/change/tension/prominence/related/source-neighborhood fields.
- Promotion now creates a normal appagent-authored PlatformStory VText
  revision with parent revision lineage, source/candidate/style citations,
  mutation-boundary metadata, and an explicit owner-fork non-mutation note.
- Projection-review obligations still follow promotion, so Style.vtext
  projections are reviewed separately before durable projection relations
  advance.

Evidence:

- Problem-first documentation commit:
  `4f36b6e8c18cb0e46111a43f7ce93d9c27036f91`.
- Behavior commit:
  `ab3c60c106269987efd3b1b4b122eeaf3adfd1a3`.
- CI run `27084187943`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, and staging deploy.
- FlakeHub run `27084187931`: success.
- Staging health at `2026-06-07T05:50:56Z` reported proxy and sandbox
  `deployed_commit` =
  `ab3c60c106269987efd3b1b4b122eeaf3adfd1a3`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js -g "owner-scoped"`
  with 1 passed.

Invariants preserved:

- Classified StoryGraph mutation happens only after explicit platform
  graph-candidate promotion.
- Platform story revision is a normal VText revision, not a bespoke news-card
  mutation.
- User-owned forks, edits, and contributions remain separate and are not
  rewritten by platform promotion.
- Style.vtext projection changes still require separate projection review,
  draft, and approval before the durable projection relation advances.
- News remains non-oracle and provenance-rich: the PlatformStory revision cites
  the StoryGraph node, graph candidate, promoted source ContentItem, and
  attached Style.vtext artifacts.
- Graph nodes remain Story VText headlines; source, claim, related-edge, and
  prominence changes are overlays/metadata on that headline-node object.

Belief-state update:

- The proven ingestion/publication loop now includes:
  Source Service SourceItem -> refresh classification -> review artifacts ->
  graph candidate -> explicit platform promotion -> classified StoryGraph
  semantic update -> normal PlatformStory VText revision -> projection-review
  obligations -> ProjectionStory VText revision path.
- This closes the prior gap where classification existed but promotion flattened
  all outcomes into a generic source append.
- The implementation is still low-resolution: claim/event/entity extraction is
  heuristic, and related-edge detection is text-match based.

Remaining error field:

- There is still no scheduled 24/7 source registry/fetch-cycle worker.
- Claim/event/entity extraction is not yet a durable structured claim model
  with uncertainty/dispute state.
- Story clustering and related-edge detection are still low-resolution.
- Style.vtext can be selected and cited, but not yet composed, replaced,
  forked, or proposed through a full style-revision workflow.
- There is no dedicated publication/update-feed/newsletter/researcher queue
  beyond the current reconciliation/projection-review queues.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Apply a cognitive-transform route choice before continuing. Highest-value
  remaining routes are:
  Style.vtext composition/replacement as citeable VText artifacts, structured
  claim/research-task records tied to refresh classifications, or scheduled
  Source Service ingestion runs over the story registry.

## Problem Checkpoint - Style.vtext Is Selectable But Not Composable Or Replaceable - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Global Wire style sources are ordinary citeable VTexts and can be selected in
  the News app, but they are still effectively seeded choices.
- The spec requires `Style.vtext` to be an authored source artifact that can be
  selected, replaced, composed, merged, hybridized, forked, published, or
  permissioned.
- Without a product path for composition/replacement, Style.vtext remains closer
  to app configuration than to a user/reviewer-owned source artifact that can
  change the projection relation.

Evidence:

- `GlobalWireStyleSource` records id/title/label/summary/source path/doc id and
  is embedded in each StoryGraph row's `style_sources_json`.
- The app can open a selected style source VText and switch among seeded
  styles.
- There is no `/api/global-wire/style-sources` product path, no composed style
  VText creation, and no relation update that proves a new composed/replaced
  Style.vtext can shape a projection.

Belief-state update:

- The next topology-preserving improvement is a bounded style-source transition:
  compose or replace a Style.vtext through a normal VText artifact, attach it to
  a StoryGraph row, and create a projection relation that cites the composed
  style source and the same StoryGraph evidence.

Remaining error field:

- No durable style composition endpoint exists.
- No durable style replacement endpoint exists.
- No staging proof shows a newly authored Style.vtext source becoming selectable
  and shaping a StoryGraph projection.

Next executable probe:

- Add an owner-scoped Global Wire Style.vtext compose/replace endpoint and app
  controls. Composition/replacement should create a normal Style.vtext document
  with citations to parent style docs, update the story's selectable style
  sources, create a projection VText/relation for the story, and preserve
  evidence/provenance invariants.

## Checkpoint - Style.vtext Compose/Replace Product Path Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has an owner-authenticated
  `/api/global-wire/style-sources` product path for `compose` and `replace`.
- A composed/replacement Style.vtext is a normal VText document/revision with
  citations to parent Style.vtext sources and the StoryGraph headline node.
- The composed/replacement style becomes a selectable StoryGraph style source,
  receives a durable projection relation, and creates a projection Story VText
  that cites the composed Style.vtext and runtime source artifacts.
- The app exposes authenticated Compose and Replace controls in the
  Style.vtext projection surface; public preview remains read-only.

Evidence:

- Problem-first documentation commit:
  `77fb434fd407fb2e719dc1d64f2394e5e76d4438`.
- Behavior commit:
  `8badef03009ff6a8de39f60112437aef53b74b72`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- CI run `27084379921`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27084379922`: success.
- Staging health at `2026-06-07T06:01:17Z` reported proxy and sandbox
  `deployed_commit` =
  `8badef03009ff6a8de39f60112437aef53b74b72`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. This proof clicked the app Compose control, observed
  `/api/global-wire/style-sources` returning 201, loaded the composed
  Style.vtext through `/api/vtext/documents/{doc_id}`, and verified the
  refreshed StoryGraph exposes the new style source plus projection VText doc.

Invariants preserved:

- Every story/projection artifact remains a normal editable VText document or
  revision.
- User-owned forks and contributions remain separate from platform stories.
- Style.vtext is now citeable, selectable, composable, and replaceable through
  a product path rather than only through seeded app configuration.
- News remains non-oracle and provenance-rich: new style and projection VTexts
  cite parent style sources, the StoryGraph headline node, and runtime source
  artifacts.
- Graph nodes remain story headlines with source-neighborhood semantics.
- The Global Wire app continues to pass Future Noir, Carbon Kintsugi, and
  London Salmon view proofs on staging.

Belief-state update:

- The product slice now supports:
  live/source evidence -> StoryGraph -> Story VTexts -> selectable Style.vtext
  projections -> composed/replaced Style.vtext source artifact -> durable
  projection Story VText -> News app visibility -> user-owned edits and
  contributions -> research/reconciliation queue.
- This closes the specific style-source gap recorded above: Style.vtext is no
  longer merely a seeded projection selector.
- The style workflow is still intentionally low-resolution: composition uses
  explicit parent-style selection and authored metadata, not a full merge UI,
  permission model, version graph, or researcher style-review queue.

Remaining error field:

- There is still no scheduled 24/7 source registry/fetch-cycle worker.
- Claim/event/entity extraction is not yet a durable structured claim model
  with uncertainty/dispute state.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- There is no dedicated publication/update-feed/newsletter/researcher queue
  beyond the current reconciliation/projection-review queues.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is structured claim/research-task state tied
  to refresh classifications: turn source-refresh outcomes into durable claim,
  dispute, uncertainty, and research-task records that can feed reconciliation
  and projection review without pretending the News app is an oracle.

## Problem Checkpoint - Claim And Research State Is Still String-Level - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Global Wire currently preserves non-oracle language in story prose and seeded
  claim strings, but claim/update/research state is not yet a durable product
  object.
- Source refresh classification can say `claim-changed`,
  `contradiction-added`, `source-manifest-update`, or
  `projection-revision-required`, but the downstream state is still mostly
  contribution, candidate, projection-review, and story-string mutation.
- The spec requires StoryGraph claim sets with uncertainty/dispute state and
  contribution/research queue refs. Without first-class claim/research records,
  reconciliation cannot clearly distinguish an asserted claim, disputed claim,
  evidence gap, follow-up research task, or projection review obligation.

Evidence:

- `types.GlobalWireStory.Claims` is `[]string`.
- `global_wire_story_graphs.claims_json` stores those strings.
- `GlobalWireSourceRefreshRun` records update classification and graph/projection
  actions, but not structured claim ids, uncertainty state, dispute state,
  research task ids, or evidence-gap records.
- `GlobalWireGraphUpdateCandidate` and `GlobalWireProjectionReview` exist, but
  there is no `GlobalWireClaimRecord` or `GlobalWireResearchTask` product path
  exposed through `/api/global-wire/reconciliation`.

Belief-state update:

- The next topology-preserving improvement is to add low-resolution but durable
  claim/research records generated from source-refresh classification and user
  contribution review. The records should remain explicitly provisional and
  cite SourceItems, refresh runs, contributions, graph candidates, and the
  StoryGraph headline node rather than presenting the News app as an oracle.

Remaining error field:

- No durable Global Wire claim record exists.
- No durable Global Wire research task record exists.
- No staging proof shows source refresh creating reviewable claim/dispute/gap
  state tied to the same StoryGraph and reconciliation queue.

Next executable probe:

- Add owner-scoped claim/research-task storage and API inclusion. Source
  refreshes should create structured records according to classification:
  provisional claim review, dispute/contradiction review, evidence-gap/source
  standing review, or projection-revision follow-up. The News app should show
  these records in the research/reconciliation surface with citations and
  non-oracle status labels.

## Checkpoint - Structured Claim/Research State Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has durable `GlobalWireClaimRecord` and
  `GlobalWireResearchTask` records.
- Candidate-producing source refreshes create provisional claim/research
  artifacts linked to SourceItem, refresh run, contribution, reconciliation
  decision, graph candidate, and StoryGraph headline node.
- Claim records preserve non-oracle state: claim kind, uncertainty state,
  dispute state, evidence gap, source standing, update classification, and
  review status.
- Research tasks preserve follow-up reviewer prompts and open/high-priority
  task state without mutating platform stories.
- `/api/global-wire/reconciliation` now returns `claim_records` and
  `research_tasks`, and the News app reconciliation surface renders them under
  the graph candidate that caused them.

Evidence:

- Problem-first documentation commit:
  `2b0c5f0217b8cb16dc94a1430e2e138042a4749a`.
- Behavior commit:
  `77b44450bb98847197cc4740220bb3582adb6938`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- CI run `27084632043`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27084632040`: success.
- Staging health at `2026-06-07T06:13:49Z` reported proxy and sandbox
  `deployed_commit` =
  `77b44450bb98847197cc4740220bb3582adb6938`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. The proof exercises source refresh and, when a candidate is
  produced, checks `claim_record`, `research_task`, and reconciliation-list
  visibility.

Invariants preserved:

- Source refresh remains non-mutating: it creates review artifacts and graph
  candidates, not platform story rewrites.
- Claim records are provisional and explicitly carry uncertainty/dispute/evidence
  gap state.
- User-owned contributions still do not mutate platform stories.
- Projection review remains separate from graph-candidate promotion.
- News remains provenance-rich and non-oracle: the new records cite SourceItem,
  refresh, contribution, decision, candidate, and StoryGraph context.
- Graph nodes remain Story VText headlines; claims and tasks are overlays and
  reconciliation artifacts, not replacement graph nodes.
- The required theme views continue to pass on staging.

Belief-state update:

- The product slice now supports structured claim/research state tied to the
  source-refresh classification path:
  Source Service evidence -> SourceItem -> refresh classification ->
  contribution/decision/candidate -> provisional claim record -> research task
  -> reconciliation surface.
- This closes the narrow gap where claim and research state were only strings or
  implicit candidate labels.
- It is still low-resolution: extraction is heuristic, records are generated
  from classification and source metadata rather than a full claim parser, and
  research tasks do not yet have a dedicated worker/reviewer lifecycle.

Remaining error field:

- There is still no scheduled 24/7 source registry/fetch-cycle worker.
- Claim/event/entity extraction is durable enough to queue review, but not yet
  a rich structured claim model with entity/event/timeline normalization.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- Research tasks exist, but there is not yet a dedicated researcher work queue
  with assignment, worker evidence packets, completion, or reconciliation merge.
- There is no dedicated publication/update-feed/newsletter queue.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is scheduled/source-registry ingestion:
  create a low-resolution source registry and bounded fetch-cycle record that
  can run over story neighborhoods, produce SourceItems/source-refresh runs, and
  leave durable scheduler/fetch evidence without claiming a full 24/7 newsroom.

## Problem Checkpoint - Source Registry And Fetch Cycles Are Not Durable - 2026-06-07

mission status: `checkpoint_incomplete`

Problem:

- Global Wire can run a bounded source refresh for one StoryGraph node, but it
  does not yet have a source registry or fetch-cycle object.
- The spec's target loop starts with `source registry -> fetch cycles / live
  source ingestion -> normalized SourceItems`. Without a durable registry and
  cycle record, Global Wire still depends on one-off user/UI refreshes and
  cannot prove scheduled or repeated ingestion readiness.
- A real 24/7 worker is not required for the first mission, but the topology
  must preserve that future path: a cycle should name its source scope, story
  neighborhood, query basis, trigger, status, refresh-run refs, source-item
  refs, and residual failure/unavailable state.

Evidence:

- `/api/global-wire/source-refresh` exists for a single story and query.
- `GlobalWireSourceRefreshRun` records one refresh/classification pass.
- There is no `GlobalWireSourceRegistryEntry`, no `GlobalWireFetchCycleRun`,
  no endpoint that runs a bounded cycle over story neighborhoods, and no News
  app surface showing registry/cycle evidence.

Belief-state update:

- The next topology-preserving improvement is a low-resolution owner-scoped
  source registry seeded from StoryGraph nodes and a bounded fetch-cycle product
  path that can run the existing source-refresh classification logic across one
  or more stories. The cycle must honestly record `unavailable` or
  `no-evidence` states when Source Service cannot produce evidence.

Remaining error field:

- No durable source registry exists.
- No durable fetch-cycle run exists.
- No staging proof shows a cycle producing SourceItems/source-refresh runs or
  recording honest failure state across a StoryGraph neighborhood.

Next executable probe:

- Add a bounded `/api/global-wire/fetch-cycles` product path. It should list or
  seed source registry entries from current StoryGraph stories, run Source
  Service search per selected story/query when configured, reuse the same
  non-mutating refresh/classification artifact path, and expose cycle evidence
  in the News app without claiming full 24/7 automation.

## Checkpoint - Bounded Source Registry Fetch Cycles Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has durable `GlobalWireSourceRegistryEntry` and
  `GlobalWireFetchCycleRun` records.
- `/api/global-wire/fetch-cycles` can list registry/cycle evidence or run a
  bounded owner-authenticated cycle over selected StoryGraph headline nodes.
- A cycle seeds/updates source registry entries from StoryGraph headlines,
  searches Source Service when configured, records `unavailable`/`no-evidence`
  states honestly, and reuses the existing non-mutating source-refresh
  classification path when evidence is found.
- The cycle records story ids, registry entry ids, refresh-run ids,
  SourceItem ids, trigger, status, and message.
- The News app exposes a `Run fetch cycle` control and displays recent cycle
  and registry evidence in the source/reconciliation surface.

Evidence:

- Problem-first documentation commit:
  `08a518a2d0bfbb7d5de2f8aa8bcdcf97b10e54f2`.
- Behavior commit:
  `b5c4c4da8cd8889c81d2da4d8c64c6e5481d70a3`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  and `npm run build` in `frontend/`.
- CI run `27084845252`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27084845249`: success.
- Staging health at `2026-06-07T06:24:55Z` reported proxy and sandbox
  `deployed_commit` =
  `b5c4c4da8cd8889c81d2da4d8c64c6e5481d70a3`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. The proof clicked the app `Run fetch cycle` control and
  verified fetch-cycle id, StoryGraph story id, registry entry, and refresh-run
  evidence.

Invariants preserved:

- The fetch cycle is bounded and explicit; it does not claim that a 24/7 worker
  exists.
- Source Service evidence still becomes owner-scoped SourceItems before it
  enters Global Wire review artifacts.
- Fetch cycles reuse the non-mutating refresh/classification path; they do not
  rewrite platform StoryGraph stories directly.
- News remains non-oracle and provenance-rich: cycle, registry, refresh,
  SourceItem, contribution, candidate, claim, and research-task records are
  inspectable.
- Graph nodes remain Story VText headlines; registry entries and fetch cycles
  are scheduler/source overlays.
- The required theme views continue to pass on staging.

Belief-state update:

- The product trajectory now includes:
  source registry -> bounded fetch cycle -> Source Service search ->
  SourceItem -> source-refresh run -> classification/review artifacts ->
  StoryGraph candidate/claim/research task -> News app visibility.
- This closes the narrow gap where ingestion was only manual one-story source
  refresh.
- The implementation remains low-resolution: there is no autonomous scheduler,
  recurrence policy, source standing catalog, dedupe across cycles, or worker
  assignment lifecycle yet.

Remaining error field:

- There is still no autonomous 24/7 scheduler or recurring source-fetch worker.
- Claim/event/entity extraction is durable enough to queue review, but not yet
  a rich structured claim model with entity/event/timeline normalization.
- Source registry entries are seeded from StoryGraph headlines rather than a
  curated source catalog with standing policy.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- Research tasks exist, but there is not yet a dedicated researcher work queue
  with assignment, worker evidence packets, completion, or reconciliation merge.
- There is no dedicated publication/update-feed/newsletter queue.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is a researcher-task lifecycle: allow the
  structured research tasks produced by source refresh/fetch cycles to move
  from `open` to assigned/completed/blocked with evidence packets, then feed
  reconciliation without mutating platform stories directly.

## Problem - Research Tasks Are Passive Queue Entries - 2026-06-07

mission status: `checkpoint_incomplete`

Observed gap:

- Source refresh and fetch cycles can now create structured
  `GlobalWireResearchTask` records, but those records are passive `open` queue
  entries.
- There is no product-path transition for assigning, completing, or blocking a
  task.
- There is no durable researcher evidence packet attached to a task, so a
  completed investigation cannot be cited by the reconciliation surface.
- The News app can show that a task exists, but it cannot show that a researcher
  investigated it or what evidence they produced.

Why this matters:

- The spec asks for a trajectory from source evidence through StoryGraph and
  Story VTexts into user-owned edits/contributions and a
  research/reconciliation-ready state.
- A task record without a lifecycle is not yet reconciliation-ready. It names
  work to do, but it does not carry the result of that work.
- The product invariant is that research evidence may inform reconciliation, but
  must not directly mutate platform stories. Without an explicit evidence
  packet, the system has no honest place to put researcher output.

Belief-state update:

- The next topology-preserving improvement is a low-resolution research-task
  lifecycle: `open -> assigned -> completed|blocked`, with evidence packets
  visible from reconciliation and News.
- This should remain owner-scoped and product-path driven. It should update the
  task/evidence lane only, not rewrite platform Story VTexts or StoryGraph
  candidates.

Remaining error field:

- No endpoint accepts researcher task lifecycle transitions.
- No durable evidence-packet table exists for task output.
- No reconciliation response includes research-task evidence.
- No staging proof shows a task being completed or blocked through the product
  path.

Next executable probe:

- Add a bounded `/api/global-wire/research-tasks` product path that can assign,
  complete, or block a task; persist a `GlobalWireResearchTaskEvidence` packet;
  expose those packets in reconciliation and the News app; and prove on staging
  that completing a task creates reconciliation-visible evidence without
  mutating platform stories.

## Checkpoint - Research Task Lifecycle Evidence Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has durable `GlobalWireResearchTaskEvidence` records.
- `/api/global-wire/research-tasks` can advance owner-scoped research tasks via
  `assign`, `complete`, or `block`.
- Each lifecycle transition creates a reconciliation-visible evidence packet
  with task id, story id, claim id, source content id, status, evidence level,
  summary, reviewer note, and timestamp.
- `/api/global-wire/reconciliation` now returns `research_evidence` beside
  claims and research tasks.
- The News app exposes compact task lifecycle controls and displays evidence
  packets under the claim/research surface.

Evidence:

- Problem-first documentation commit:
  `2a7741f13f19f8e73aeea52475320a9ae1d4177a`.
- Behavior commit:
  `fd57db5816423b4c808e923b20b3705176571c92`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`,
  `npm run build` in `frontend/`, and `git diff --check`.
- CI run `27085059204`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27085059209`: success.
- Staging health at `2026-06-07T06:36:00Z` reported proxy and sandbox
  `deployed_commit` =
  `fd57db5816423b4c808e923b20b3705176571c92`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. The proof created source-refresh research artifacts,
  completed a research task through `/api/global-wire/research-tasks`, and
  verified reconciliation returned completed task evidence.

Invariants preserved:

- Research task lifecycle transitions update only task/evidence records.
- Platform StoryGraph stories and Story VTexts are not mutated by task
  assignment, completion, or blocking.
- Evidence packets remain owner-scoped and provenance-rich; they cite task,
  story, claim, and SourceItem lineage.
- News remains non-oracle: completed research means evidence is ready for
  reconciliation, not that the platform story has been automatically rewritten.
- Graph nodes remain Story VText headlines; research evidence is a review lane
  overlay attached to claim/source neighborhoods.
- The required Future Noir, Carbon Kintsugi, and London Salmon app views
  continue to pass on staging.

Belief-state update:

- The product trajectory now includes:
  source registry -> bounded fetch cycle -> Source Service search ->
  SourceItem -> source-refresh run -> claim/research task ->
  research-task evidence packet -> reconciliation-visible review state.
- This closes the narrow gap where research tasks existed but could not carry
  investigator output.
- The implementation remains low-resolution: completed evidence is visible to
  reconciliation, but there is not yet a dedicated acceptance path that turns a
  research evidence packet into a publication/update decision with reviewer
  contracts and rollback refs.

Remaining error field:

- Completed research evidence does not yet have a first-class reconciliation
  handoff decision separate from contribution acceptance.
- There is still no autonomous 24/7 scheduler or recurring source-fetch worker.
- Claim/event/entity extraction is not yet a rich structured model with
  entity/event/timeline normalization.
- Source registry entries are still seeded from StoryGraph headlines rather
  than a curated source catalog with standing policy.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- There is no dedicated publication/update-feed/newsletter queue.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is a research-evidence reconciliation
  handoff: allow completed `GlobalWireResearchTaskEvidence` packets to be
  reviewed into explicit reconciliation/publication decisions that can propose
  StoryGraph candidate updates or block them, while preserving the invariant
  that no platform story mutates without explicit platform review.

## Problem - Completed Research Evidence Has No Handoff Decision - 2026-06-07

mission status: `checkpoint_incomplete`

Observed gap:

- `GlobalWireResearchTaskEvidence` packets can now show that a researcher
  assigned, completed, or blocked a task.
- Reconciliation can see the evidence packet, but there is no separate reviewer
  decision over that packet.
- A completed evidence packet cannot yet be accepted into a reconciliation lane,
  blocked as insufficient, or linked to the candidate review state as the
  reason a candidate is ready or not ready for platform review.

Why this matters:

- Researcher evidence and reconciliation authority are different product
  transitions. Collapsing them would make "completed research" behave like an
  oracle verdict.
- The spec requires non-oracle, provenance-rich news and explicit user-owned
  edits/contributions moving toward reconciliation-ready state. That requires a
  visible handoff decision, not just task completion.
- The invariant remains: research handoff may update review/candidate state,
  but must not mutate platform stories or Story VTexts without explicit
  platform promotion.

Belief-state update:

- The next topology-preserving improvement is a low-resolution handoff lane:
  completed task evidence -> handoff decision (`accepted-for-review` or
  `blocked`) -> candidate/reconciliation state visible in News.
- The handoff should cite the evidence packet, task, story, claim, SourceItem,
  and candidate lineage.

Remaining error field:

- No durable handoff-decision table exists for task evidence.
- No product endpoint accepts or blocks completed task evidence.
- Candidate readiness can be inferred from source-refresh artifacts, but not
  explicitly justified by a research-evidence handoff.
- News cannot show whether completed research evidence has been accepted into
  reconciliation or blocked as insufficient.

Next executable probe:

- Add a bounded `/api/global-wire/research-evidence` product path that accepts
  or blocks completed task evidence, persists a handoff decision, updates only
  review/candidate status where appropriate, exposes decisions in
  reconciliation and News, and proves on staging that platform stories remain
  unchanged.

## Checkpoint - Research Evidence Handoff Decisions Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has durable `GlobalWireResearchEvidenceDecision` records.
- `/api/global-wire/research-evidence` can accept or block a completed
  `GlobalWireResearchTaskEvidence` packet.
- Accepted evidence creates an explicit handoff decision and may update the
  linked graph-update candidate review status to `research-evidence-accepted`.
- Blocked evidence creates an explicit handoff decision and may update the
  linked graph-update candidate review status to `research-evidence-blocked`.
- `/api/global-wire/reconciliation` now returns `research_decisions`.
- The News app shows evidence handoff controls and renders the resulting
  decision/result state under the research evidence packet.

Evidence:

- Problem-first documentation commit:
  `b802ff9120c8db05122a71d76dcf520f9716b60f`.
- Behavior commit:
  `65777dccf17743d59f59b51066d8ac4695d967cc`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`,
  `npm run build` in `frontend/`, and `git diff --check`.
- CI run `27085208133`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27085208129`: success.
- Staging health at `2026-06-07T06:43:55Z` reported proxy and sandbox
  `deployed_commit` =
  `65777dccf17743d59f59b51066d8ac4695d967cc`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. The proof created source-refresh research artifacts,
  completed a research task, accepted the completed research evidence through
  `/api/global-wire/research-evidence`, and verified reconciliation returned a
  `ready-for-platform-review` handoff decision.

Invariants preserved:

- Research handoff decisions are explicit review artifacts over evidence, not
  oracle verdicts.
- The handoff endpoint does not mutate platform StoryGraph manifests or Story
  VTexts.
- Candidate status may move to a research-evidence review state, but platform
  promotion remains a separate explicit candidate-review path.
- Handoff decisions remain owner-scoped and cite evidence, task, story, claim,
  SourceItem, and candidate lineage.
- News remains provenance-rich and non-oracle: it shows completed evidence and
  reviewer handoff state separately.
- The required Future Noir, Carbon Kintsugi, and London Salmon app views
  continue to pass on staging.

Belief-state update:

- The product trajectory now includes:
  source registry -> bounded fetch cycle -> SourceItem -> source-refresh run ->
  claim/research task -> research evidence packet -> research handoff decision
  -> candidate ready/blocked for platform review.
- This closes the gap where completed research evidence existed but had no
  reviewer-authorized handoff into reconciliation/publication readiness.
- The implementation remains low-resolution: handoff can update candidate
  readiness, but there is not yet a publication/update-feed package that groups
  accepted candidates, Style.vtext projection reviews, and owner-visible
  rollback refs into a release-ready news update.

Remaining error field:

- There is no dedicated publication/update-feed/newsletter queue.
- Platform promotion exists for graph candidates, and projection review exists
  for Style.vtext outputs, but there is no single publication package tying
  accepted research, candidate review, projection review, and rollback refs
  together.
- There is still no autonomous 24/7 scheduler or recurring source-fetch worker.
- Claim/event/entity extraction is not yet a rich structured model with
  entity/event/timeline normalization.
- Source registry entries are still seeded from StoryGraph headlines rather
  than a curated source catalog with standing policy.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is a low-resolution publication/update queue:
  package accepted research-evidence handoff decisions, graph candidate review
  state, Style.vtext projection review state, source lineage, and rollback refs
  into an owner-visible `GlobalWirePublicationUpdate` artifact without
  auto-mutating platform stories.

## Problem - Publication Readiness Is Not Yet A Durable Update Package - 2026-06-07

mission status: `checkpoint_incomplete`

Observed gap:

- Global Wire can ingest source evidence, create candidates, complete research,
  accept research evidence into review, promote graph candidates, and review
  Style.vtext projection drafts.
- Those artifacts remain separate rows and UI fragments. There is no durable
  publication/update package that names the story update, cited evidence,
  candidate state, projection review state, source lineage, and rollback refs.
- The News app cannot yet show an owner-visible "this update is packaged for
  publication/release review" object.

Why this matters:

- The spec's ingestion loop includes `Story VText revisions ->
  publication/update feed`. Candidate readiness and projection review are
  prerequisites, but they are not themselves a publication feed.
- A publication/update queue must remain non-oracle. It should bundle evidence
  and review state for owner/platform review, not silently publish or rewrite
  stories.
- Rollback refs need to be visible before publication pressure, not invented
  after a mutation. At low resolution, they can cite current story/projection
  doc ids and candidate/review ids as rollback anchors.

Belief-state update:

- The next topology-preserving improvement is a low-resolution
  `GlobalWirePublicationUpdate` artifact created from an accepted research
  handoff decision and its linked StoryGraph candidate/projection review state.
- The artifact should be owner-scoped, provenance-rich, and visible in
  reconciliation/News. It should not mutate platform stories or publish a
  newsletter by itself.

Remaining error field:

- No durable publication/update package table exists.
- No product endpoint packages accepted research handoffs into publication
  update artifacts.
- Reconciliation does not return publication/update queue records.
- News cannot display publication readiness, source lineage, or rollback refs
  as a first-class object.

Next executable probe:

- Add `/api/global-wire/publication-updates` with GET/POST. POST should package
  an accepted `GlobalWireResearchEvidenceDecision` into a
  `GlobalWirePublicationUpdate` that cites story id, candidate id, evidence
  decision id, SourceItem id, projection review ids/states, rollback refs, and
  a non-publication status. Expose it in reconciliation and News, prove locally
  and on staging that package creation does not mutate platform stories.

## Checkpoint - Publication Update Package Proven - 2026-06-07

mission status: `checkpoint_incomplete`

Artifact advanced:

- Global Wire now has durable `GlobalWirePublicationUpdate` records.
- `/api/global-wire/publication-updates` supports GET/POST.
- POST packages an accepted `GlobalWireResearchEvidenceDecision` into an
  owner-visible update-feed artifact with story id, candidate id, research
  decision id, evidence id, SourceItem id, projection review ids/states,
  rollback refs, status, summary, and timestamps.
- `/api/global-wire/reconciliation` now returns `publication_updates`.
- The News app can package an accepted research handoff into a publication
  update and render the package status plus rollback-ref count under the
  evidence/reconciliation surface.

Evidence:

- Problem-first documentation commit:
  `55de1feef1c016f686704bd2185df98c65666cbd`.
- Behavior commit:
  `b70747b2290a77aa8b0868f7f7b9e7bdfd1d9861`.
- Local shaping proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`,
  `npm run build` in `frontend/`, and `git diff --check`.
- CI run `27085413013`: success, including runtime shards, Go vet/build,
  non-runtime tests, integration smoke, frontend build, and staging deploy.
- FlakeHub run `27085413008`: success.
- Staging health at `2026-06-07T06:54:35Z` reported proxy and sandbox
  `deployed_commit` =
  `b70747b2290a77aa8b0868f7f7b9e7bdfd1d9861`.
- Public deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium`
  with 4 passed and 1 auth-gated skip.
- Authenticated deployed owner proof passed:
  `GLOBAL_WIRE_AUTH_PROOF=1 PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/global-wire-app.spec.js --project=chromium --grep "owner-scoped"`
  with 1 passed. The proof created source-refresh research artifacts,
  completed research, accepted the research evidence, packaged a publication
  update through `/api/global-wire/publication-updates`, and verified
  reconciliation returned `packaged-for-publication-review`.

Invariants preserved:

- Publication updates are queue/package artifacts, not automatic publication.
- Package creation does not mutate platform StoryGraph manifests or Story
  VTexts.
- User-owned forks and edits remain separate from platform stories.
- The package cites provenance and rollback anchors rather than hiding review
  state behind an oracle result.
- News remains provenance-rich and non-oracle: it shows source/research/handoff
  and publication package state as separate steps.
- The required Future Noir, Carbon Kintsugi, and London Salmon app views
  continue to pass on staging.

Belief-state update:

- The product trajectory now includes:
  source registry -> bounded fetch cycle -> SourceItem -> source-refresh run ->
  claim/research task -> research evidence packet -> research handoff decision
  -> publication update package.
- This closes the gap where accepted research/candidate/projection state had no
  owner-visible publication/update-feed package.
- The implementation remains low-resolution: publication packages are review
  artifacts, not newsletter/public-route publication; rollback refs are
  string anchors over current StoryGraph/VText/candidate/review records rather
  than a full transactional restore plan.

Remaining error field:

- There is still no autonomous 24/7 scheduler or recurring source-fetch worker.
- Claim/event/entity extraction is not yet a rich structured model with
  entity/event/timeline normalization.
- Source registry entries are still seeded from StoryGraph headlines rather
  than a curated source catalog with standing policy.
- Style.vtext composition/replacement is proven, but not yet a full style
  revision workflow with forks, merge conflict handling, permissions, or
  publication review.
- Publication updates do not yet produce newsletter/public-route artifacts or
  Autoradio scripts.
- Autoradio remains a prompt-bar/VText handoff rather than a dedicated audio
  playback or scheduling subsystem.

Next executable probe:

- Highest-value next realism axis is upstream source/claim richness: add a
  low-resolution structured entity/event/timeline extraction artifact tied to
  SourceItems, claim records, and StoryGraph overlays, so publication packages
  can cite more than headline-level claim summaries without making the graph
  object itself a source/entity graph.
