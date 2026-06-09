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
- Locally, the story endpoint now requires the canonical `Wire.vtext` edition
  to transclude platform VText articles before they appear in Global Wire.

unproven or partial claims:

- No source-cycle proof yet.
- No deployed VText edition rendering proof yet; the edition gate is locally
  implemented but not pushed/deployed.
- No AppChangePackage/adoption or run-acceptance record was created in this
  slice; the acceptance level remains staging-smoke-level, not promotion-level.
- Deeper SourceMaxx, style-source, newsletter, and autoradio compatibility
  routes still exist and need replacement or deletion.

next step:

- Commit, push, monitor CI/deploy, and run staging acceptance for the
  edition-gated `Wire.vtext` path. Then continue toward creating/updating the
  edition VText through the product source cycle rather than test fixtures.
