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
