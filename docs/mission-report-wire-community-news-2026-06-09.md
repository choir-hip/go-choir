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

## Run State

status: checkpoint_incomplete

current artifact state:

- Problem documented before behavior changes.
- First deletion slice implemented locally: fake frontend preview stories and
  backend read-time story seeding are removed from the active app/API path.
- Existing deeper legacy SourceMaxx, style-source, publication, newsletter, and
  autoradio compatibility routes remain for later deletion/replacement.

what was proven:

- The legacy fake/seeded front-page behavior remains in active code and tests.
- The local app can now render an honest empty Community Wire state without
  seeded preview articles.
- Focused store/runtime tests and frontend production build pass.

unproven or partial claims:

- No staging acceptance proof yet.
- No source-cycle proof yet.
- No VText edition rendering proof yet.
- No commit/push/deploy proof yet for the first behavior slice.

next step:

- Commit the first behavior-changing deletion slice, push, monitor CI/deploy,
  then verify staging identity and product-path behavior before increasing
  realism toward edition VText rendering.
