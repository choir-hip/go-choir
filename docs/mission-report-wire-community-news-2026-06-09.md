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
- Prompt orchestration repair deploy: commit
  `cbfd0637ab921947bfd5652fbe47411dca20ef79` passed CI run `27221392006`;
  deploy job `80377182065` succeeded; staging `/health` reported proxy and
  sandbox commit `cbfd0637ab921947bfd5652fbe47411dca20ef79`, deployed at
  `2026-06-09T16:45:15Z`.
- Counter-evidence after `cbfd0637`: the authenticated staging `Command
  prompt` was reprobed with the same Community Wire product-path proof request.
  The product still opened a VText document for the prompt and reported
  `tool loop iteration 2: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`. Global Wire remained at
  `0 articles`, `community-wire-vtext-index`, and the honest empty edition
  state. The local exact initial tool-choice repair is therefore insufficient
  as a product fix; the operational handoff must not depend on another VText
  provider turn to create the persistent-super request.
- Deterministic super handoff repair implemented locally: prompt-bar VText
  materialization still creates the user prompt VText seed, but when the prompt
  is classified as execution/proof work it now casts the request to persistent
  super from runtime with VText channel context and uses the resulting super
  run as the initial loop. The VText provider turn is skipped for this initial
  operational handoff.
- Local verification for the deterministic super handoff:
  - `nix develop -c go test ./internal/runtime -run 'TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|TestHandlePromptBarVTextRouteCompletesConductorSynchronously|TestInitialVTextToolChoiceUsesExactTools|TestRequestSuper|TestVTextRequestSuper'`
  - `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice)'`
  - `nix develop -c scripts/go-test-runtime-shards`
- Deterministic handoff repair deploy: commit
  `7b7bba73b000d2eb9ab01a1b5d4b88387a989351` passed CI run
  `27222135633`; deploy job `80379785630` succeeded; staging `/health`
  reported proxy and sandbox commit
  `7b7bba73b000d2eb9ab01a1b5d4b88387a989351`, deployed at
  `2026-06-09T16:58:10Z`.
- Authenticated staging reprobe after `7b7bba73`: the same live `Command
  prompt` request now produced visible persistent-super evidence in the activity
  feed: `5bd6de97-3b58-408c-bf89-c42c81b083de reported a blocker` with
  `Role: super`, summary `Runtime fallback: Super failed before worker
  delegation/packa...`. The prompt still did not complete the Wire flow:
  another run `44d6fe75-c74b-4597-897b-8d6db9269ab5` reported `tool loop
  iteration 0: gateway call failed: gateway client: fireworks: status 412
  Precondition Failed (sanitized)`, the VText window remained in a failed or
  drafting state, Compute Monitor showed `0 running runs`, and Global Wire
  remained at `0 articles` with `community-wire-vtext-index`.
- Local root-cause repair for the super blocker: provider precondition fallback
  in the tool loop was only attempted when a `tool_choice` was present.
  Persistent-super inbox runs do not use an initial tool choice, so a Fireworks
  412 on the first super call bypassed the fallback list and failed before
  `request_worker_vm` or `start_worker_delegation`. The local repair applies
  provider precondition fallback to any provider 412/precondition error with
  configured fallbacks, and gives legacy Fireworks pro/flash policies a
  DeepSeek-pro fallback path before platform fallback.
- Local verification for the provider precondition repair:
  - `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop(RelaxesExactInitialToolChoiceAfterProviderPrecondition|TriesMultipleProviderPreconditionFallbacks|TriesProviderPreconditionFallbackWithoutToolChoice)|TestProviderPreconditionFallbackSelectionsUseDeepSeekProForFlash|TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper'`
  - `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice|ProviderPrecondition|ModelPolicy|RunToolLoop)'`
- Provider precondition repair deploy: commit
  `c14d10e66ead343efac6d6b299b3634cbf7eaea7` passed CI run
  `27222759295`; deploy job `80382014794` succeeded; staging `/health`
  reported proxy and sandbox commit
  `c14d10e66ead343efac6d6b299b3634cbf7eaea7`, deployed at
  `2026-06-09T17:09:16Z`.
- Authenticated staging reprobe after `c14d10e6`: the same product prompt no
  longer stopped only at the first super provider precondition failure. The
  activity feed showed `5bd6de97-3b58-408c-bf89-c42c81b083de called
  start_worker_delegation`, proving the super path got past the initial
  Fireworks 412 into a worker-delegation attempt. The product still did not
  create a Wire edition: the feed then showed the same super agent reported a
  blocker with summary `Runtime fallback: Super failed before worker
  delegation/packa...`, Compute Monitor showed `0 running runs`, and Global
  Wire remained at `0 articles`. The fallback summary is now suspicious because
  it says "before worker delegation" even though the feed observed
  `start_worker_delegation`; the next problem is likely delegation terminal
  evidence/fallback classification, not initial provider precondition fallback.
- Local worker-delegation fallback repair: runtime fallback synthesis still
  looked for the deprecated `delegate_worker_vm` result when deciding whether a
  failed super had preserved worker evidence. Current super uses
  `start_worker_delegation`, so a successful start result could be followed by
  the stale "before worker delegation/package" fallback. The local repair treats
  both `start_worker_delegation` and `delegate_worker_vm` as worker-delegation
  evidence, picks the latest successful result across both names, and uses
  neutral "worker delegation" wording in VText-visible fallback updates.
- Local verification for worker-delegation fallback repair:
  - `nix develop -c go test ./internal/runtime -run 'TestRuntimeSynthesizes(VTextBlockerWhenSuperFailsBeforeDelegation|WorkerDelegationUpdateAfterStartWorkerDelegation)|TestSuperFailureAfterDelegateWorkerVMSynthesizesVTextWorkerUpdate'`
  - `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice|ProviderPrecondition|ModelPolicy|RunToolLoop|WorkerDelegation|DelegateWorker|SuperFailure)'`
- Worker-delegation fallback repair deploy: commit
  `4cf18f6dc282c906090ceace406575cb99fc67c2` passed CI run
  `27223242171`; deploy job `80383782637` succeeded; staging `/health`
  reported proxy and sandbox commit
  `4cf18f6dc282c906090ceace406575cb99fc67c2`, deployed at
  `2026-06-09T17:18:17Z`.
- Authenticated staging reprobe after `4cf18f6d`: the same product prompt
  reached a real worker run and produced a more precise VText dashboard:
  `Community Wire Staging Proof -- Dashboard`, super run
  `da234dfc-91d1-47b3-9216-3d61150bb16a`, worker run
  `bf18da24-002b-4875-9535-a227a83c7175`, state `cancelled`, and
  `AppChangePackages: 0`. The dashboard names a critical unresolved
  authentication blocker: deployed `choir.news` is live, but the worker VM
  cannot authenticate to Global Wire and VText product APIs because
  `X-Authenticated-User` is a trusted proxy header and the worker's gateway
  token is not a user identity. The worker spawned co-super
  `3e5e6046-1526-4966-a05e-d5ea74d86611`, but it remained pending with zero
  output. A later separate super run
  `850dd4e5-04e8-4ef8-a025-decf996a77b9` hit gateway connection refused
  before delegation, but the cancellation certificate for worker run
  `bf18da24-002b-4875-9535-a227a83c7175` is the primary blocker evidence.
- Problem checkpoint after `4cf18f6d`: product-path worker delegation now
  reaches a worker, but the worker lacks a sanctioned authenticated product API
  path for owner/platform-scoped Global Wire and VText operations on staging.
  Do not fix this before documenting it: the next behavior change must define
  or repair the worker/candidate authentication contract rather than asking the
  worker to spoof trusted proxy headers or use internal/test routes.
- Local repair after the worker auth blocker: foreground super now has a
  narrow `product_api_request` tool for active-computer product API
  orchestration. The tool dispatches through the runtime's browser-public route
  table, injects the authenticated owner from run context inside the runtime,
  and refuses `/internal/*`, `/api/test/*`, `/api/agent/*`, prompt-config
  routes, and paths outside the product-path allowlist. Super prompt policy now
  tells foreground super to use that tool for authenticated product API
  orchestration instead of delegating a worker to impersonate a browser session
  or hand-setting trusted proxy headers. Worker/candidate repo and package work
  still delegates to worker VMs.
- Local verification for the product API tool repair:
  - `nix develop -c go test ./internal/runtime -run 'TestProductAPIRequestTool|TestSuperSystemPromptIncludesDelegationPolicy|TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|TestDefaultAgentToolRegistryByRole'`
  - `nix develop -c go test ./internal/runtime -run 'Test(ProductAPIRequestTool|HandlePromptBar|InitialVText|RequestSuper|GlobalWire|VText|WorkerDelegation|DelegateWorker|SuperFailure|DefaultAgentToolRegistry|SuperSystemPrompt|CoagentUpdate)'`
  - `nix develop -c scripts/go-test-runtime-shards`
- Foreground-super product API repair deploy: commit
  `8e9ff96bb3d01f9cf69ca73b184921f09878ea05` passed CI run
  `27224095377`; deploy job `80386812136` succeeded. Staging `/health`
  reported proxy and sandbox commit
  `8e9ff96bb3d01f9cf69ca73b184921f09878ea05`, deployed at
  `2026-06-09T17:33:55Z`.
- New staging problem discovered after the `8e9ff96b` deploy: authenticated
  Chrome proof submitted the Community Wire proof prompt through the live
  `Command prompt`. Two variants created prompt VText documents instead of
  surfacing a new persistent-super/product-api orchestration run within the
  observation window:
  `6aaae8b6-9edc-4ae2-8d66-a12f6edf40c1` for the "using product paths only"
  wording, and `90fe95ef-4ecc-478e-93ac-2044ae18105c` for the exact
  operational proof wording covered by the local prompt-bar handoff test. After
  a 90-second deployed observation window, the desktop still showed the prompts
  as VText documents with no visible new super handoff or active worker. The
  visible Community Wire dashboard still carried the earlier worker-auth
  blocker and StoryGraph-source-refresh blocker; no Wire edition was created.
  The external verifier also confirmed that direct `curl` with a spoofed
  `X-Authenticated-User` header receives `401`, preserving the proxy trust
  boundary. This means the foreground `product_api_request` tool may be
  deployed, but the product prompt path is not yet reliably reaching a super run
  that can use it.
- Prompt handoff observability repair: commit
  `a42e1afca9c5ba5cb26c3de4abe4b41779ccbaaf` propagates
  `initial_loop_id` from the prompt-bar conductor decision into the opened VText
  app context and exposes it as `data-vtext-initial-loop-id` on the VText
  product surface. The backend prompt-bar handoff test now uses the deployed
  "using product paths only" wording. Local verification:
  - `nix develop -c go test ./internal/runtime -run 'TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper'`
  - `npm --prefix frontend run build`
  - `npm --prefix frontend run e2e -- desktop-shell-core.spec.js -g "prompt bar routes normal input through conductor and opens vtext"` was attempted after starting `vite preview` on `127.0.0.1:4173`, but the local backend API was not running and auth registration failed with `register/begin failed: 500`; this did not exercise the new assertion.
- CI/deploy for `a42e1afca9c5ba5cb26c3de4abe4b41779ccbaaf`: CI run
  `27225029886` succeeded, including frontend build, all runtime shards,
  non-runtime Go tests, integration-tagged smoke, Go vet/build, and deploy.
  Deploy job `80390109123` succeeded in 31s. Staging `/health` reported proxy
  and sandbox commit `a42e1afca9c5ba5cb26c3de4abe4b41779ccbaaf`, deployed at
  `2026-06-09T17:51:13Z`.
- Authenticated staging reprobe after `a42e1afc`: the live `Command prompt`
  created VText doc `532ffcab-9d0d-4b2e-a364-15b983f4fb90` for the same
  Community Wire proof request. The VText root exposed
  `data-vtext-initial-loop-id="a69ea9f3-32a4-4d49-acd7-974148b8a1e4"`,
  proving the conductor decision carried a persistent-super handoff id into the
  product surface. After an additional 60-second observation window, the
  activity feed still showed only the VText revision event for that doc and no
  visible `product_api_request`, worker, package, or Wire edition update. The
  next blocker is therefore not handoff-id visibility; it is execution/progress
  of the persistent-super run after the handoff id is created.
- Persistent-super inbox starvation repair: commit
  `f78e5519d6ce9db843017f33796829cbaf18f6c3` lets blocked persistent-super
  inbox runs yield to fresh deliveries instead of treating `RunBlocked` as an
  active loop that starves new prompt work. Local verification:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestPersistentSuper(BlockedRunDoesNotStarveFreshInboxDelivery|ProcessesConcurrentInboxDeliveriesInFollowupRun)'`
  - `nix develop -c go test ./internal/runtime -run 'TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper'`
  - `nix develop -c scripts/go-test-runtime-shards`
- CI/deploy for `f78e5519d6ce9db843017f33796829cbaf18f6c3`: CI run
  `27225580546` succeeded, including all runtime shards, non-runtime Go tests,
  integration-tagged smoke, Go vet/build, and deploy. Deploy job
  `80392089714` succeeded in 32s. Staging `/health` reported proxy and sandbox
  commit `f78e5519d6ce9db843017f33796829cbaf18f6c3`, deployed at
  `2026-06-09T18:01:08Z`.
- Authenticated staging reprobe after `f78e5519`: the same Community Wire proof
  prompt created VText doc `2b7ffa7a-58a1-48be-b6f3-cbe5cd5d3d24` with
  `data-vtext-initial-loop-id="ca50cf79-9fef-413b-a15f-6e4507edc639"`. The
  live activity feed then showed foreground super calling `product_api_request`,
  `finish_worker_delegation`, `save_evidence`, and `submit_coagent_update`.
  This proves the prompt path now reaches the foreground product API tool
  instead of stopping at worker browser authentication or a blocked stale super
  loop.
- New staging blocker from the `f78e5519` reprobe: the super's coagent update
  summarized the evidence as `Global Wire staging proof: StoryGraph is seeded
  (3 dossiers...)`. The positive source-to-edition proof still did not create a
  visible `global-wire/Wire.vtext` edition update or rendered article. The next
  problem has moved again: foreground product API orchestration works, but the
  underlying Global Wire source-refresh/reconciliation path is still tied to
  seeded StoryGraph dossier state rather than the Community Wire
  source-artifact -> VText-edition topology.
- Source-native Global Wire repair deploy: commit
  `e75697b77b7422414df7f6653c29fb8b31325401` passed CI run `27226620966`;
  deploy job `80395758795` succeeded. Staging `/health` reported proxy and
  sandbox commit `e75697b77b7422414df7f6653c29fb8b31325401`, deployed at
  `2026-06-09T18:20:11Z`.
- Authenticated staging reprobe after `e75697b`: the live `Command prompt`
  submitted a source-native Community Wire proof request that explicitly asked
  for `/api/global-wire/source-refresh` without a seeded `story_id`, research
  evidence completion, Article VText draft/approval, publication update and
  artifact approval, and `global-wire/Wire.vtext` inclusion using only public
  authenticated product APIs. The product opened VText doc
  `2d097c76-e179-4655-b6e4-c32ffa8c70ec` with
  `data-vtext-initial-loop-id="ce6a3e77-332b-4fb5-af06-c695d136b4cb"`, and
  activity run `5bd6de97-3b58-408c-bf89-c42c81b083de` called
  `source_search` and `product_api_request`. The VText then recorded a blocker
  before the Wire pipeline completed:
  `Runtime memory compaction failed with encoding error (Incorrect string value:
  '\xE2\x80...;...' for column 'summary')`. The document's blocker log states
  that no worker lease, delegation, or AppChangePackage was produced, and the
  status table kept source query, research evidence, Article VText approval,
  publication update, Wire.vtext inclusion, and verifier proof all pending.
  This is the next documented problem before any fix: the source-native product
  path is deployed, but long-running VText/super proof cannot progress reliably
  while run-memory compaction can fail on UTF-8 punctuation in summaries.
- Run-memory compaction persistence repair: commit
  `d4b24de821e4dccfe5bdc65a6f42b1fbd480d106` normalizes run-memory message
  JSON, summary, reason, model, and details text before persistence. Local
  verification passed:
  `nix develop -c go test ./internal/store -run 'TestRunMemoryAppend(NormalizesUnicodeText|ListAndLatest)$' -count=1`,
  `nix develop -c go test ./internal/runtime -run 'TestRunMemory|TestRuntimeRunMemory' -count=1`,
  `nix develop -c go test ./internal/store -count=1`, and
  `nix develop -c scripts/go-test-local`.
- CI/deploy for `d4b24de821e4dccfe5bdc65a6f42b1fbd480d106`: CI run
  `27227525121` succeeded, including all runtime shards, non-runtime tests,
  integration-tagged smoke, Go vet/build, and deploy. Deploy job
  `80398935659` succeeded in 32s. Staging `/health` reported proxy and sandbox
  commit `d4b24de821e4dccfe5bdc65a6f42b1fbd480d106`, deployed at
  `2026-06-09T18:36:42Z`.
- Authenticated staging reprobe after `d4b24de8`: the prior source-native proof
  VText progressed past the compaction blocker and recorded a partial-pass
  pipeline in run `11d7cfe2-db39-4e4f-b6a4-2c805cfb24f9`, with verification
  ref `global-wire-verification-853a2c65`. The evidence chain includes
  `source_content:global-wire-source-service-7a63fa2f-f0b1-5ffb-8275-a20b6df813f1`,
  `refresh_run:global-wire-source-refresh-47387144-46d6-4eb2-8204-f3defbd2f620`,
  `research_evidence:global-wire-research-evidence-414eabbd-4bbb-4243-a97f-c8f98b8e12a7`,
  `research_decision:global-wire-research-decision-d246e8ab-f50c-4abe-98fa-3200038859e2`,
  `publication_update:global-wire-publication-update-1e2cb8c8-22b9-4a91-9568-30fd63fd5608`,
  and `publication_artifact:global-wire-publication-artifact-fec7e799-90bc-492d-8450-03b0a893f042`.
  The proof is not acceptance: the VText reports `Story ID:
  story-supply-resilience` despite the prompt requesting no seeded `story_id`,
  and the verifier says `/api/global-wire/stories` returns empty while the story
  endpoint returns 404 for the approved artifact. This documents the next
  problem before any fix: the orchestration can now get past compaction and
  produce internally linked publication records, but it can still fall back to
  seeded StoryGraph context and produce an approved artifact that is not
  visible through the public edition/story API.

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
- Deployed counter-evidence after `cbfd0637` shows that provider-level exact
  tool-choice steering still leaves the request exposed to the same Fireworks
  412 blocker. The next repair should create the persistent-super request
  deterministically inside the runtime when the prompt is classified as an
  operational execution/proof request, rather than asking the VText model to
  make that handoff.
- The deterministic handoff repair now exists locally and is covered by
  focused prompt-bar tests plus all runtime shards.
- The deterministic handoff repair is deployed and changed staging behavior: the
  live proof prompt now reaches a visible persistent-super blocker instead of
  only a VText drafting blocker. This proves the initial handoff moved, but not
  that operational proof can proceed.
- Local provider precondition fallback repair now covers super runs without
  `tool_choice`, the failure shape observed on staging after `7b7bba73`.
- Deployed reprobe after `c14d10e6` shows the repair moved the product path one
  step forward: super reached `start_worker_delegation`, then failed or was
  classified as failed before durable worker/package evidence reached VText.
- Local fallback classification now recognizes `start_worker_delegation`
  results as worker evidence and should preserve the worker start result instead
  of producing the stale before-delegation summary.
- The worker-delegation fallback repair is deployed and reprobed. It changed
  the evidence from a stale before-delegation summary to a concrete worker
  cancellation certificate. The next blocker is not Wire indexing or VText
  publication logic yet; it is worker/candidate authentication to deployed
  product APIs.
- Local foreground-super product API repair now gives the active super a typed
  owner-context product API path, avoiding both worker browser impersonation
  and model-controlled trusted-header spoofing.
- Local source-native Global Wire repair now lets authenticated
  `/api/global-wire/source-refresh` omit `story_id`, import the Source Service
  top result as a deterministic `source-native-*` review context, create
  claim/research/extraction/projection-review artifacts without persisting a
  seeded `GlobalWireStory`, approve article VText, package a publication
  artifact, and update `global-wire/Wire.vtext` through the existing
  publication artifact approval path.
- Local verification passed for the source-native path:
  `nix develop -c go test ./internal/runtime -run TestHandleGlobalWireSourceNativeRefreshPublishesEditionVText -count=1`,
  the neighboring seeded-story Global Wire tests, and
  `nix develop -c scripts/go-test-local`.
- The source-native Global Wire repair is deployed at `e75697b`, and staging
  health confirms that commit. The first authenticated product-path reprobe got
  as far as source search and foreground product API calls, then hit a
  run-memory compaction encoding blocker before publication evidence could be
  produced.
- The run-memory compaction persistence repair is deployed at `d4b24de8`, and
  staging health confirms that commit. The proof run no longer stops at the
  compaction encoding blocker; it now reaches a partial-pass publication record
  chain.
- The public publication approval guard is deployed at
  `d0ad4ed264a6f256a0e4b979397c24883ac7d3d7`, CI run `27228093837` passed,
  staging deploy job `80400949898` passed, and staging health confirms proxy
  and sandbox at that SHA. The guard prevents public Community Wire artifacts
  from being marked `publication-approved` unless an approved Article VText is
  available for edition publication.
- A fresh authenticated staging proof prompt after `d0ad4ed2` reached the live
  foreground proof path but failed before Global Wire source-native API
  orchestration. Activity run `68f1ed7e-882e-432e-884a-c6ab5bba559a` reported
  `tool loop iteration 2: gateway call failed: gateway client: deepseek:
  status 402 Payment Required (sanitized)`. This is the next documented
  product-path blocker: deployed model/provider policy selected an unavailable
  fallback model for the operational proof run.
- Provider availability fallback repair: commit
  `eba8bd05c5d3ec08662cfdd758e4466e4f5f102c` treats provider billing/
  availability errors such as `402 Payment Required` as model-fallback-worthy
  in the tool loop. Local focused tool-loop tests and `nix develop -c
  scripts/go-test-local` passed. CI run `27228719168` succeeded, deploy job
  `80403153718` succeeded, and staging `/health` reported proxy and sandbox at
  `eba8bd05c5d3ec08662cfdd758e4466e4f5f102c`, deployed at
  `2026-06-09T18:57:53Z`.
- Authenticated staging reprobe after `eba8bd05`: a fresh source-native
  Community Wire proof prompt created VText doc
  `3138b6db-3167-417d-a8c9-db0297d2e85b` with
  `data-vtext-initial-loop-id="53580323-5b55-4995-97e1-8a08e916ac2d"`. After
  an additional observation window, the document still showed v0,
  `Writing first draft...`, and `Revising...`. The authenticated product
  surface showed no fresh source search, foreground `product_api_request`,
  publication update/artifact, `global-wire/Wire.vtext` inclusion, rendered
  story, or precise new blocker for that loop. The old
  `68f1ed7e...` DeepSeek 402 blocker remained visible as historical activity,
  but no fresh 402 blocker was observed for the `eba8bd05` prompt. This is the
  next documented problem before any fix: provider availability fallback is
  deployed, but the owner-visible VText/proof run can stall in draft state
  without surfacing either product API progress or a blocker.
- Correction from longer observation of the same staging reprobe: the
  `3138b6db-3167-417d-a8c9-db0297d2e85b` activity feed later reported the
  fresh blocker
  `tool loop iteration 2: gateway call failed: gateway client: deepseek:
  status 402 Payment Required (sanitized)`. The operator confirmed DeepSeek
  credits are exhausted and directed active policy to Xiaomi MiMo instead:
  use `mimo-v2.5` for conductor, researcher, processor, and VText, while
  reserving `mimo-v2.5-pro` for vsuper, co-super when no multimodal input is
  needed, and reconciler. The next fix should migrate generated/default model
  policy and runtime fallback away from DeepSeek for these roles.
- MiMo policy migration: commit
  `da1631250afdfb0b2ab6bf1cd0059a3a7179026c` moves generated/default runtime
  policy to Xiaomi MiMo. It uses `mimo-v2.5` for conductor, researcher,
  processor, VText, super, and verifier, and reserves `mimo-v2.5-pro` for
  vsuper, co-super, and reconciler. Local focused runtime tests and
  `nix develop -c scripts/go-test-local` passed. CI run `27230587852`
  succeeded, staging deploy job `80409677926` succeeded, and `/health`
  reported proxy and sandbox at that SHA, deployed at
  `2026-06-09T19:32:06Z`.
- Fresh authenticated staging proof after `da163125` is blocked before prompt
  submission by VM route availability. The Browser product UI shows
  `BOOTSTRAP FAILED (502)` and keeps retrying `VM route returned 502`; the
  same deployed `/health` response reports `status: degraded` and
  `vmctl_status: unavailable`. This changes the immediate blocker from
  DeepSeek provider policy to staging active-computer route availability.
- Final MiMo staging proof: after vmctl recovered, a fresh public prompt-bar
  submission `6cfdf6d6-d1d6-4305-840b-f5960e597f7f` opened VText doc
  `cdc7d469-041d-4622-94d5-ed43f96542df` and initial loop
  `7cf44647-c6de-49bc-96bd-d7e017895404`. Trace completed with conductor,
  super, and VText on `xiaomi/mimo-v2.5`; no DeepSeek call appeared. The
  foreground product path executed source refresh, projection review, research
  evidence, publication update, publication artifact, and artifact approval.
  Evidence ids include source refresh `global-wire-source-refresh-04fc54aa`,
  projection review `global-wire-projection-review-5fc5beb6`, research
  evidence `global-wire-research-evidence-afebe39d`, publication update
  `global-wire-publication-update-adb48a9b`, and publication artifact
  `global-wire-publication-artifact-e8f417e7-de00-4e94-94a1-254b88462e8d`.
- Deployed acceptance proof: `GET /api/global-wire/stories` returned one story
  from `community-wire-edition-vtext`: "The Computer Science Degree Isn't
  Dead". The story article VText is
  `b45dc29b-6ff5-4efb-98b3-895a4afd8968`; the edition is
  `global-wire/Wire.vtext` doc `fb021fa3-16a5-4841-b30c-6e36bd0a10c2`
  revision `a5af660e-02ae-4b6b-8a9e-e34e611b9391`, whose included doc ids
  contain that article VText.

unproven or partial claims:

- Source-cycle proof exists for one staging Community Wire article.
- Positive deployed VText edition rendering exists through
  `/api/global-wire/stories`, `community-wire-edition-vtext`, and
  `global-wire/Wire.vtext`.
- The authenticated stored-story fallback blocker is fixed, but only as an
  honest empty-state proof. It is not yet proof that a real product source
  cycle creates and publishes an article into the edition.
- No product-path owner prompt has yet supervised the source-refresh,
  research-evidence, graph-candidate, projection-review, publication-update,
  publication-artifact, publication-approval sequence to completion on staging.
- The first prompt orchestration repair was pushed and deployed, but
  authenticated staging reprobe still hit the VText Fireworks 412 blocker
  before any visible persistent-super handoff.
- The deterministic handoff repair is deployed and reprobed, but the super path
  fails before worker delegation/package work and a VText run still reports
  Fireworks 412. The source-to-edition proof remains blocked.
- The provider precondition repair is deployed and reprobed, but positive
  source-to-edition proof is still blocked at worker delegation evidence. The
  runtime-visible blocker text may be stale or misleading for
  `start_worker_delegation` attempts.
- The worker-delegation fallback repair is deployed and reprobed, but positive
  source-to-edition proof is still blocked because the delegated worker cannot
  authenticate against staging product APIs.
- The foreground-super product API repair is local only until pushed, deployed,
  and reprobed against the authenticated staging prompt.
- The foreground-super product API repair is now deployed at `8e9ff96b`, but
  the authenticated staging prompt reprobe did not reach visible super product
  API orchestration. The next blocker is prompt-bar/VText routing or
  observation of the super handoff after VText document creation, not the
  `product_api_request` tool registration itself.
- The prompt handoff id is now product-visible on staging at `a42e1afc`, but
  the persistent-super run behind that id did not visibly progress within the
  observation window.
- The blocked-super starvation repair is deployed and reprobed at `f78e5519`.
  The proof prompt now reaches foreground super `product_api_request` and
  coagent evidence. Positive edition proof remains blocked by seeded
  StoryGraph/source-refresh state, not by prompt routing, worker auth, or
  foreground product API access.
- The source-native Global Wire repair is committed, pushed, deployed, and
  locally verified, and the compaction persistence blocker is repaired and
  deployed. The authenticated staging prompt still has not completed acceptance
  because the post-fix proof used seeded `story-supply-resilience` context and
  the approved artifact was not visible through `/api/global-wire/stories` or
  the story endpoint.
- The approval guard for that false-positive public artifact state is
  committed, pushed, deployed, and locally verified. The next authenticated
  staging prompt is now blocked earlier by deployed DeepSeek 402 provider
  availability, so no fresh source-native publication/edition visibility proof
  exists after the guard.
- The provider availability fallback repair is committed, pushed, deployed, and
  locally verified. Longer staging observation showed the fresh authenticated
  prompt still surfaced a DeepSeek 402 blocker, so the repair is insufficient
  while generated/default policy continues to select DeepSeek for foreground
  proof roles.
- The MiMo policy migration is committed, pushed, deployed, locally verified,
  and positively reprobed through a source-native Wire publication path.
- No durable `RunAcceptanceRecord` was created. An attempted synthesis from a
  different freshly registered owner could not see the owner-scoped trajectory;
  an acceptance-bound retry kept one browser context but auth expired before
  synthesis and returned `401 authentication required`. This is residual
  run-acceptance/session debt. The proof level is deployed staging product
  proof, not promotion-level.
- Deeper SourceMaxx, style-source, newsletter, and autoradio compatibility
  routes still exist and need replacement or deletion.

next step:

- Repair durable `RunAcceptanceRecord` synthesis for long-lived staging proof
  sessions, then synthesize acceptance from the completed source-native Wire
  trajectory without relying on short-lived Playwright auth state.
