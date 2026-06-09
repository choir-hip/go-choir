# Mission Report: Wire Community News

Date: 2026-06-09

## Mission Goal And Artifact

Run `docs/mission-wire-community-news-v0.md` as MissionGradient and move
Community Wire toward the public source-to-VText news instance of the Choir
Community Cloud.

The real artifact is:

```text
Community Cloud source artifacts
-> platform processor/reconciler/researcher notes and requests
-> VText-agent-authored Article/Report.vtexts
-> Wire.vtext public edition
-> Wire app renderer over the edition VText graph
```

## Initial Substrate Inspection

Required context read at mission start:

- `AGENTS.md`
- `docs/missiongradient-method.md`
- `docs/mission-wire-community-news-v0.md`
- `docs/choir-wire-source-to-vtext-spec-2026-06-09.md`
- `docs/glossary.md`
- `docs/computer-ontology.md`
- `docs/wire-news-system-learning-saga-2026-06-09.md`

Initial `git status --short` was clean.

## Problem Checkpoint: Legacy Wire Product Truth

Problem:

The active Wire product still contains legacy Global Wire / StoryGraph /
SourceMaxx behavior that can present seeded or compatibility data as product
truth. This violates the current Wire requirements contract because the app
must render VText-owned articles and an edition VText over real source
artifacts, not hardcoded preview stories, seeded StoryGraph records, source
manifest stand-ins, or renamed compatibility shims.

Evidence from code inspection on 2026-06-09:

- `frontend/src/lib/GlobalWireApp.svelte` initializes three hardcoded preview
  stories and unauthenticated preview mode uses them as the front page.
- `internal/store/global_wire.go` contains `defaultGlobalWireStories`,
  `globalWireSeedState = "seeded-source-neighborhood"`, and
  `ensureDefaultGlobalWireStories`, which auto-seeds owner-scoped story graph
  records, seed source ContentItems, style VTexts, and projection VTexts.
- `internal/runtime/global_wire.go` reports story responses as
  `durable-storygraph` or `durable-storygraph+source-network-vtexts`,
  combining seeded graph records with indexed VTexts.
- `internal/store/global_wire_test.go`, `internal/runtime/global_wire_test.go`,
  and `frontend/tests/global-wire-app.spec.js` still assert old seed, preview,
  SourceMaxx/source-network, and StoryGraph-derived behavior.
- `cmd/sourcecycled/main.go`, `internal/cycle/sourcemaxx.go`,
  `internal/sourceapi/types.go`, and `cmd/sourcecycled/main_test.go` still
  expose SourceMaxx naming and compatibility surfaces.

Belief-state update:

The cleanest first cut is not to build more source ingestion. The first
behavior-changing slice should delete fake front-page authority and make the
Wire app/API show an honest empty or VText-indexed state. That preserves the
artifact topology: VTexts and source artifacts are real; seeded stories are not.

Remaining error field:

- The runtime still needs a Community Wire edition-VText truth path.
- The current `/api/global-wire/stories` route is a compatibility story-list
  shape, not an edition VText graph.
- Source daemon terminology and dispatch types still use SourceMaxx.
- Telegram ingestion still requires a proper API path; preview HTML scraping
  remains a legacy behavior to delete.
- Staging proof is still unrun for this mission.

Next executable probe:

Remove the frontend preview stories and backend auto-seeding path, then update
focused tests so absence of live VText-owned articles is represented honestly
instead of filled with seeded stories.

## Evidence Ledger

- Initial mission context read: local file inspection, 2026-06-09.
- Initial worktree state: clean `git status --short`, 2026-06-09.
- Problem checkpoint evidence: code search and focused file inspection listed
  above.
- Docs-first checkpoint commit: `87f7df56`.
- First behavior slice: backend story reads no longer auto-seed fake stories;
  the authenticated stories endpoint returns `community-wire-vtext-index`; the
  frontend no longer contains hardcoded preview stories and renders an honest
  empty edition state when no VText-indexed articles exist.
- Focused tests:
  - `nix develop -c go test ./internal/store -run 'TestGlobalWireStoriesDoNotSeedFakeFrontPage'`
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireStories(ReturnsHonestEmptyState|IndexesSourceNetworkVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest)'`
  - `nix develop -c go test ./internal/store -run '^$'`
  - `nix develop -c go test ./internal/runtime -run '^$'`
  - `npm run build` in `frontend/`
- Local browser proof against `http://127.0.0.1:5173/`: Global Wire opened,
  `storyCount` was `0`, `[data-global-wire-empty-state]` was visible, seed text
  count was `0`, `Port backlog recedes` count was `0`, and
  `data-global-wire-data-source` was `community-wire-vtext-index`.
- First pushed CI run for `205125c9596fc62ff4dd196fa0e48a1e80362b3b`
  (`27216692179`) failed runtime shards because legacy route tests still
  assumed implicit `story-supply-resilience` seeding. Root cause: tests were
  depending on read-time product seeding that the behavior slice intentionally
  removed.
- CI fix: legacy route tests now create explicit store-backed Wire story,
  source, style, article, and projection fixtures. This preserves the new
  product invariant while keeping older route tests meaningful.
- Local verification after CI fix:
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire(StyleSourcesComposeAndReplace|SourceRefreshCreatesCandidateWithoutMutatingStoryGraph|FetchCycleCreatesRegistryAndRefreshEvidence|SourceRefreshClassifiesNoVisibleChangeWithoutCandidate|PromotesClassifiedRefreshIntoStoryGraphAndPlatformVText|ReconciliationRecordsDecisionWithoutMutatingStoryGraph)'`
  - `nix develop -c scripts/go-test-runtime-shards`
- Fixture repair commit: `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`.
- Replacement CI for `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`:
  run `27217127841`, success. Build Frontend and Deploy were skipped because
  the commit changed docs/tests only.
- Forced staging deploy run for the same SHA: workflow dispatch
  `27217273257`, success. Deploy job `80362634048` completed in 6m11s.
- Staging identity proof:
  `curl -fsS https://choir.news/health` returned proxy and sandbox build
  commit `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`, deployed at
  `2026-06-09T15:34:43Z`.
- Staging browser proof against `https://choir.news/`: opened Global Wire via
  the Desk menu. The frontend build commit was
  `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`; `[data-global-wire-app]` was
  visible; story count was `0`; `[data-global-wire-empty-state]` was visible
  with "No Wire edition articles yet"; app data source was
  `community-wire-vtext-index`; counts for `SourceMaxx newsroom`,
  `seed source neighborhood`, `Port backlog recedes`, and `StoryGraph desk`
  were all `0`. Screenshot evidence was written outside the repo at
  `/tmp/choir-staging-global-wire-open-proof.png`.
- Second behavior slice: `/api/global-wire/stories` now recognizes the
  canonical Community Wire edition alias `global-wire/Wire.vtext`. Platform
  VText articles are no longer indexed merely because they are recent platform
  documents; they must be transcluded by that edition VText through a
  `vtext:<doc_id>` reference. The response reports
  `community-wire-edition-vtext` and edition metadata when an edition exists.
- Focused verification for the edition gate:
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireStories(ReturnsHonestEmptyState|DoesNotIndexUntranscludedPlatformVTexts|IndexesEditionTranscludedVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest)'`
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  - `nix develop -c go test ./internal/runtime -run '^$'`
  - `npm run build` in `frontend/`
- Edition-gate landing loop: commit
  `f6707096cabfdf7e860ceb35483b8335191429f2` pushed to `origin/main`; CI run
  `27218260845` succeeded; deploy job `80366180237` succeeded in 26s.
- Staging identity proof after `f6707096`:
  `curl -fsS https://choir.news/health` returned proxy and sandbox build
  commit `f6707096cabfdf7e860ceb35483b8335191429f2`, deployed at
  `2026-06-09T15:50:58Z`.
- Staging browser proof after `f6707096`: opened Global Wire via the Desk menu.
  The backend/runtime was `f6707096` by `/health`; the frontend build commit
  remained `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd` because deploy impact
  skipped frontend rebuild for a backend-only change. Global Wire still showed
  zero stories, visible "No Wire edition articles yet", data source
  `community-wire-vtext-index`, and zero occurrences of `SourceMaxx newsroom`,
  `seed source neighborhood`, `Port backlog recedes`, and `StoryGraph desk`.
  Screenshot evidence was written outside the repo at
  `/tmp/choir-staging-global-wire-edition-gate-proof.png`.
- Product-path edition update slice: commit
  `90839193d04bfd1321d0424ae86930aac437efd5` adds a publication approval path
  that copies an approved projection VText into the platform owner and updates
  `global-wire/Wire.vtext` with a `vtext:<doc_id>` transclusion. Local focused
  tests, `TestHandleGlobalWire`, and `scripts/go-test-runtime-shards` passed.
- CI/deploy for `90839193d04bfd1321d0424ae86930aac437efd5`: CI run
  `27219131352` succeeded. Deploy job `80369262512` succeeded in 24s. Staging
  `/health` reported proxy and sandbox commit
  `90839193d04bfd1321d0424ae86930aac437efd5`, deployed at
  `2026-06-09T16:05:13Z`.
- New staging problem discovered after deploy: authenticated Chrome proof
  opened `https://choir.news/`, activated Global Wire from the dock, and saw
  three legacy front-page articles even though the surface label said
  `community-wire-vtext-index` and "awaiting edition VTexts". The visible
  articles included `Port backlog recedes as carriers warn of uneven inland
  recovery`, `Grid operators add reserve alerts as heat forecast shifts north`,
  and `City air monitors show sharp overnight improvement after smoke plume
  disperses`; their metadata still included `seed source neighborhood`. This
  means old durable `GlobalWireStory` records remain a front-page fallback for
  authenticated users when no edition exists. The problem is documented here
  before the next fix, per Problem Documentation First.
- Stored-story fallback removal: commit
  `02c799074c65c1698dfac0c0973effd3b1c400de` removes owner-scoped stored
  `GlobalWireStory` rows from `/api/global-wire/stories` front-page output.
  Local verification passed:
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire(PublicationArtifactApprovalPublishesEditionVText|Stories(ReturnsHonestEmptyState|DoesNotIndexUntranscludedPlatformVTexts|IndexesEditionTranscludedVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest))'`
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  - `nix develop -c scripts/go-test-runtime-shards`
- CI/deploy for `02c799074c65c1698dfac0c0973effd3b1c400de`: CI run
  `27219955936` succeeded. Deploy job `80372239235` succeeded in 37s. Staging
  `/health` reported proxy and sandbox commit
  `02c799074c65c1698dfac0c0973effd3b1c400de`, deployed at
  `2026-06-09T16:19:49Z`.
- Authenticated staging Chrome proof after `02c79907`: opened
  `https://choir.news/`, activated Global Wire from the dock, and inspected the
  product DOM. The Global Wire window showed user `yusefnathanson@me.com`,
  `0 articles`, source `community-wire-vtext-index`, `Front Page`, `awaiting
  edition VTexts`, and `No Wire edition articles yet`. The Global Wire window
  contained no `seed source neighborhood` metadata and no grid/city seed
  headlines. The remaining `Port backlog recedes...` text in the full desktop
  DOM came from minimized VText window titles outside the Global Wire front
  page, not from story cards.
- Source-network metadata cleanup: commit
  `465c9cffb65548b54f834fd9e84737b52cabbc31` changes fresh Community Wire
  publication-approved platform article revisions to use `source_network_*`
  provenance metadata instead of minting new `source_maxx_*` keys. The reader
  remains backward-compatible with old `source_maxx_*` metadata for existing
  VText revisions.
- Local verification for `465c9cffb65548b54f834fd9e84737b52cabbc31`:
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire(PublicationArtifactApprovalPublishesEditionVText|Stories(DoesNotIndexUntranscludedPlatformVTexts|IndexesEditionTranscludedVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest))'`
  - `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`
  - `nix develop -c scripts/go-test-runtime-shards`
- CI/deploy for `465c9cffb65548b54f834fd9e84737b52cabbc31`: CI run
  `27220546359` succeeded. Deploy job `80374232404` succeeded in 34s. Staging
  `/health` reported proxy and sandbox commit
  `465c9cffb65548b54f834fd9e84737b52cabbc31`, deployed at
  `2026-06-09T16:29:49Z`.
- Docs evidence checkpoint: commit
  `b91754952aa404343f974b0f42f90202e66243a5` records the `465c9cff`
  deployment and source-network metadata cleanup proof.
- New staging problem discovered after the `465c9cff` deploy: an authenticated
  owner prompt submitted through the live `Command prompt` asked Community Wire
  to run the existing source-refresh/research/projection/publication flow,
  create or approve an Article VText, update `global-wire/Wire.vtext`, and
  leave evidence IDs without using test/internal routes or seed stories. The
  product path opened a VText document for that prompt, created a VText
  revision, and then reported a blocker:
  `tool loop iteration 2: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`. This proves the next positive
  edition proof is not yet reachable through the foreground owner prompt path:
  the request was routed into VText/document drafting and failed at the VText
  provider boundary instead of being supervised as an operational Wire
  source-to-publication run.
- Same staging probe confirmed the Global Wire surface stayed at the honest
  empty state while this positive path failed. A direct browser-public API
  attempt through the Chrome automation context was not a usable substitute for
  product proof: page-script fetch primitives were unavailable in that context
  and direct navigation to `/api/global-wire/stories` was blocked by the
  browser profile.
- Prompt orchestration repair implemented locally: initial VText runs for
  prompts that clearly require execution, verification, staging proof,
  product-path proof, source-refresh, or publication-flow work now require
  `request_super_execution` as the first tool instead of forcing an
  `edit_vtext` prelude. Ordinary writing and factual prompts still start with
  `edit_vtext`; scheduled worker-wake runs remain unconstrained.
- Local verification for the prompt orchestration repair:
  - `nix develop -c go test ./internal/runtime -run 'TestInitialVTextToolChoiceUsesExactTools|TestHandlePromptBarVTextRouteCompletesConductorSynchronously|TestHandlePromptBarOperationalProofInitialRunRequestsSuperFirst|TestVTextPromptInitialRevisionUsesSingleWriterLoop'`
  - `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice)'`
  - `nix develop -c scripts/go-test-runtime-shards`

## Run State

status: checkpoint_incomplete

current artifact state:

- Problem documented before behavior changes.
- First deletion slice implemented locally: fake frontend preview stories and
  backend read-time story seeding are removed from the active app/API path.
- CI fix implemented locally: tests that need a legacy story now create explicit
  fixtures instead of relying on product read-time seeding.
- Existing deeper legacy SourceMaxx, style-source, publication, newsletter, and
  autoradio compatibility routes remain for later deletion/replacement.

what was proven:

- The legacy fake/seeded front-page behavior remains in active code and tests.
- The local app can now render an honest empty Community Wire state without
  seeded preview articles.
- Focused store/runtime tests and frontend production build pass.
- Full local runtime shard script passes after explicit fixture repair.
- GitHub CI and a forced staging deploy succeeded for
  `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`.
- Staging health reports both proxy and sandbox at that SHA.
- The deployed public Global Wire UI renders the honest empty Community Wire
  state and no longer exposes the deleted preview/seed front-page text.
- The deployed story endpoint now requires the canonical `Wire.vtext` edition
  to transclude platform VText articles before they appear in Global Wire.
- Counter-evidence found on authenticated staging after `90839193`: the
  endpoint/surface still fell back to owner-scoped stored `GlobalWireStory`
  rows when no edition existed, exposing legacy seed stories for at least one
  authenticated user. That problem was documented first, then fixed by
  `02c79907`.
- Authenticated staging after `02c79907` now renders an honest empty edition
  state for that user instead of stored legacy story rows.
- Fresh publication-approved Community Wire article revisions will no longer
  mint new `source_maxx_*` metadata; existing legacy metadata is still accepted
  as read-only compatibility input while the old records/routes are deleted in
  later slices.
- The live authenticated owner prompt path currently routes the positive Wire
  proof request into VText drafting and exposes a Fireworks 412 provider
  blocker. This is not evidence that the Wire publication chain is impossible;
  it is evidence that the product-level orchestration entry point is wrong or
  under-specified for operational Community Wire proof.
- The local prompt-bar/VText handoff now classifies that operational proof
  shape as a super-execution obligation before the first VText edit, preserving
  VText ownership for ordinary documents while unblocking supervised product
  mutation/proof requests.

unproven or partial claims:

- No source-cycle proof yet.
- No positive deployed VText edition rendering proof yet; staging currently has
  no verified `Wire.vtext` edition with article transclusions to render.
- The authenticated stored-story fallback blocker is fixed, but only as an
  honest empty-state proof. It is not yet proof that a real product source
  cycle creates and publishes an article into the edition.
- No product-path owner prompt has yet supervised the source-refresh,
  research-evidence, graph-candidate, projection-review, publication-update,
  publication-artifact, publication-approval sequence to completion on staging.
- The prompt orchestration repair is only locally verified until it is pushed,
  deployed, and reprobed through the authenticated staging command prompt.
- No AppChangePackage/adoption or run-acceptance record was created in this
  slice; the acceptance level remains staging-smoke-level, not promotion-level.
- Deeper SourceMaxx, style-source, newsletter, and autoradio compatibility
  routes still exist and need replacement or deletion.

next step:

- Land and deploy the prompt-bar/conductor/VText handoff repair, verify that an
  authenticated staging Community Wire proof prompt requests persistent super
  execution first, then use that path to create/update `global-wire/Wire.vtext`
  through the product source cycle rather than test fixtures and prove staging
  renders edition-transcluded VText articles.
