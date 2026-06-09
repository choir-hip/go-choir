# MissionGradient: Wire Community News

Date: 2026-06-09

## Goal String

```text
/goal Run docs/mission-wire-community-news-v0.md as MissionGradient.
```

## Objective

Land Community Wire as the public source-to-VText news instance of the Choir
Community Cloud.

Requirements contract:
[choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md).

## Required Launch Context

The operator may start this mission in a fresh thread using only the goalstring.
Therefore the worker must begin by reading this mission document and the
requirements contract above before making behavior changes.

Required context reads:

- [AGENTS.md](../AGENTS.md) for repo operating contract, staging proof, and
  problem-documentation-first rules;
- [glossary.md](glossary.md) for current Community Cloud, Private Cloud,
  platform computer, user computer, candidate computer, and Wire vocabulary;
- [computer-ontology.md](computer-ontology.md) for host/computer/candidate
  lineage and promotion boundaries;
- [wire-news-system-learning-saga-2026-06-09.md](wire-news-system-learning-saga-2026-06-09.md)
  for the news failure history and the platform-level realization;
- [choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md)
  for product/architecture requirements.

Do not treat old Global Wire, StoryGraph, source-maxxing, source-ledger, or
Style.vtext-control documents as current requirements unless this mission or the
Wire spec explicitly mines them as historical failure evidence.

At mission start, run `git status --short` and classify dirty paths. The docs
created for this Wire ontology/mission are intentional mission context. Preserve
unrelated user/agent work. Do not overwrite or revert dirty files unless the
mission explicitly owns them and the diff has been read.

Before the first behavior-changing code commit, perform the repo-required
problem-documentation-first step: document the current fake/legacy Wire problem,
evidence, belief-state update, and remaining error field in the mission report
or a focused problem checkpoint doc. The fix commit(s) come after that
documentation checkpoint.

The shipped product should show live-updated important news from many public
sources in Wire by rendering a published edition VText that transcludes
VText-agent-authored articles/reports. Those VTexts transclude real source
artifacts. Platform processors and reconcilers run under Community Cloud
platform-computer authority. Userland personalization is designed but not
required to ship in this mission unless it falls out naturally.

This mission tables newsletter/email delivery, Autoradio, TTS/STT, vector DB,
deterministic clustering, native mobile, and automatic capital.

## Real Artifact

The artifact is not a dashboard and not a legacy graph object.

The real artifact is:

```text
Community Cloud source artifacts
-> platform processor/reconciler/researcher notes and requests
-> VText-agent-authored Article/Report.vtexts
-> Wire.vtext public edition
-> Wire app renderer over the edition VText graph
```

The app may use indexes for speed. Indexes are rebuildable caches over VTexts
and source artifacts.

## Value Criterion

Minimize divergence between the public news product and the Wire/VText-native
ontology while increasing source breadth, source depth, article quality, update
freshness, and readable newspaper presentation.

Loss increases when:

- hardcoded mock/seed stories appear as product stories;
- source labels masquerade as full sources;
- Wire owns article prose outside VText;
- platform and user-computer ownership are blurred;
- legacy StoryGraph, source-maxxing, source-ledger, source-network rename
  shims, Global-Wire-as-ontology, or style-control ontology remains visible or
  authoritative;
- rankings are fake deterministic placeholders;
- article VTexts contain outlines/status/source manifests instead of
  publishable prose;
- source transclusions fail to open into source artifacts;
- update/version propagation silently changes meaning;
- tests protect old fake behavior.

## Quality Bar

Quality level: excellent.

The standard is:

- make it work: live source intake, real source artifacts, VText-authored
  articles/reports, real edition VText, product-path proof;
- make it nice: clean readable newspaper typography, no detritus, source-rich
  prose, strong update semantics.

## Hard Invariants

1. Wire is reusable source-to-VText infrastructure.
2. Community Wire is platform-level work owned by a Choir Community Cloud
   platform computer, not a user-computer feature.
3. Private Wire reuse must remain possible; do not build one-off public-news
   code that cannot run in a Private Choir Cloud over private sources.
4. Personalization belongs in user computers and creates user-owned VTexts,
   forks, notes, alerts, preferences, and style.vtexts.
5. Articles/reports and editions are VTexts.
6. Article/report/edition writing and revision is owned by VText agents.
7. Processors and reconcilers do not write canonical VText prose directly.
8. Processors, reconcilers, researchers, supers, and coding agents may write
   durable notes/evidence/messages in their computer scope and request VText
   work.
9. External sources are durable source artifacts/ContentItems, not forced
   VTexts.
10. Sources are transcluded into VTexts through native source systems.
11. Related VTexts are transcluded VTexts.
12. Public/private source visibility and egress policy are preserved.
13. Transclusions preserve version semantics: pinned, live, or
    live-with-review.
14. Indexes are caches and must be rebuildable from VTexts/source artifacts.
15. Excise and delete legacy StoryGraph, SourceMaxx/source-maxxing/source-maxx,
    source-ledger, source-network rename shims, seed source neighborhoods,
    source chronology/search detritus, style-control panels, durable-storygraph
    labels, and hardcoded three-story fallback behavior from active product
    behavior, APIs, runtime/store types, tests, active docs, and user-visible
    copy.
16. Telegram ingestion uses proper Telegram API paths. Public preview HTML
    scraping is not an accepted fallback.
17. No hardcoded source trust tiers.
18. Wire app works in Future Noir, Carbon Kintsugi, and London Salmon with
    OS-wide theme only.
19. Staging proof on `https://choir.news` is required for behavior-changing
    completion.

## Current Belief State

Evidence from code/doc review on 2026-06-09:

- `configs/sources.json` contains 211 configured sources: 137 RSS, 73 Telegram,
  1 GDELT, with broad language/vertical tags.
- RSS ingestion currently stores feed summaries/excerpts, not consistently full
  article bodies.
- Telegram ingestion currently scrapes public preview HTML instead of using the
  Telegram API; this is legacy behavior to remove, not a fallback to preserve.
- GDELT ingestion currently uses one GKG source as metadata firehose, not enough
  for the desired public-source breadth/depth.
- Hacker News ingestion is not present as a first-class source adapter.
- `GlobalWireApp.svelte` still has hardcoded preview stories.
- `internal/store/global_wire.go` still auto-seeds three legacy graph stories
  and seed source ContentItems.
- `internal/runtime/global_wire.go` still mixes source-network-renamed VText
  articles with seeded legacy graph fallback.
- Tests still assert the old three-story/durable-storygraph behavior.
- Backend routes still contain graph candidate, style-source compose/replace,
  source-refresh, publication, autoradio, and dry-run newsletter detritus.
- Source/document import tooling is stronger than source daemon ingestion:
  researchers can import URL/PDF/DOCX/EPUB/PPTX/HTML content, but sourcecycled
  does not yet consistently produce full source artifacts.
- Existing Dolt-backed non-canonical agent state exists through
  `agent_evidence`, run memory, and `submit_coagent_update`, but it should be
  regularized as agent notebook/checkpoint behavior rather than bypassing VText
  authority.

Highest-impact uncertainty:

- The cleanest hard cutover from old Global Wire graph/fallback behavior to
  Community Wire edition-VText rendering without preserving fake compatibility
  behavior.

Update after 2026-06-09 staging slices:

- Hardcoded frontend preview stories and read-time store auto-seeding have been
  removed from the active product path.
- `/api/global-wire/stories` now indexes platform articles only when the
  canonical `global-wire/Wire.vtext` edition transcludes them, and publication
  artifact approval can copy an approved projection VText into platform scope
  and append it to that edition.
- Authenticated staging after commit
  `02c799074c65c1698dfac0c0973effd3b1c400de` shows `0 articles`,
  `community-wire-vtext-index`, and the empty edition state instead of
  owner-scoped stored seed stories.
- Remaining highest-impact uncertainty is now the positive path: prove that a
  real product source/review/publication cycle, not a test fixture, produces an
  article VText and updates `global-wire/Wire.vtext` on staging.
- Authenticated staging after commit
  `465c9cffb65548b54f834fd9e84737b52cabbc31` proved a new blocker in that
  positive path. An owner submitted a live `Command prompt` asking Community
  Wire to run the existing source-refresh/research/projection/publication flow,
  create or approve an Article VText, update `global-wire/Wire.vtext`, and
  leave evidence IDs. The product opened a VText document for the prompt,
  created a revision, and then reported a Fireworks 412 gateway blocker instead
  of supervising the operational Wire flow. The immediate uncertainty is now
  prompt-bar/conductor/VText handoff for operational proof requests.
- Local repair after documenting that blocker: initial VText runs for prompts
  that clearly require execution, verification, staging proof, product-path
  proof, source-refresh, or publication-flow work now require
  `request_super_execution` first. Ordinary writing and factual prompts still
  start with `edit_vtext`, and scheduled worker-wake turns remain free to
  choose the next tool.
- Deployed counter-evidence after that repair: commit
  `cbfd0637ab921947bfd5652fbe47411dca20ef79` passed CI and deployed to
  staging, but the same authenticated Community Wire proof prompt still opened
  a VText document and hit the Fireworks 412 blocker. The next repair should
  create the persistent-super handoff deterministically in runtime for
  operational proof prompts, not depend on another VText provider turn.
- Local deterministic handoff repair: prompt-bar VText materialization still
  creates the user prompt VText seed, but execution/proof prompts now cast the
  request directly to persistent super from runtime with VText channel context
  and use the resulting super run as the initial loop.
- Deployed counter-evidence after deterministic handoff repair: commit
  `7b7bba73b000d2eb9ab01a1b5d4b88387a989351` passed CI and deployed to
  staging. The authenticated prompt now produced a visible persistent-super
  activity blocker (`5bd6de97-3b58-408c-bf89-c42c81b083de`, `Role: super`,
  `Runtime fallback: Super failed before worker delegation/packa...`), proving
  the initial handoff moved. The flow still did not create a Wire edition:
  companion run `44d6fe75-c74b-4597-897b-8d6db9269ab5` reported `tool loop
  iteration 0: gateway call failed: gateway client: fireworks: status 412
  Precondition Failed (sanitized)`, Compute Monitor showed `0 running runs`, and
  Global Wire remained at `0 articles`.

## Homotopy Parameters

Preserve topology while increasing resolution along these axes:

- source breadth: 211 configured sources -> more RSS/Atom, Telegram API
  channels, HN, broader GDELT modes, science/finance/industry/long-tail
  multilingual sources;
- source depth: feed summaries -> full readability/source artifacts where
  allowed;
- cloud/computer ownership: user-level blur -> Community Cloud platform
  computer authority;
- article ownership: store-generated prose -> VText-agent-owned versions;
- edition truth: frontend/API story list -> edition VText transclusions;
- reconciliation: newest batch only -> corpus-level article/edition review;
- personalization design: platform recommendation -> user-level
  processor/reconciler/VText authorship;
- ranking: hardcoded prominence -> agentic editorial prominence with source
  overlap/novelty/contradiction reasoning;
- UI: visible detritus -> clean Wire renderer;
- verification: local tests -> staging product-path proof over live source
  cycles.

A lower-resolution version is acceptable only if it is the same object: source
artifacts, VText-authored articles/reports, edition VText, app renderer. Fake
seeded stories are not an acceptable low-resolution version.

## Expected Route

This is not a checklist; it is a likely route through the topology.

1. Document the current fake legacy-graph/seed-front-page problem before
   behavior changes.
2. Replace public Wire product truth with edition VText and VText/source
   indexes.
3. Delete old StoryGraph, SourceMaxx/source-maxxing/source-maxx,
   Global-Wire-as-ontology, seed/fallback/product routes, runtime/store types,
   tests, and active-architecture docs; historical docs may remain only as
   clearly superseded records, not active references.
4. Delete frontend hardcoded preview stories as product behavior.
5. Replace tests that protect old behavior with tests that require honest
   empty/live states and VText-index rendering.
6. Improve source ingestion:
   - RSS/Atom full article/readability import where allowed;
   - Telegram API ingestion as the only accepted Telegram ingestion path;
   - broader GDELT use and clearer metadata/source artifact handling;
   - Hacker News adapter;
   - expanded source registry.
7. Ensure platform processors/reconcilers create/update article/report VTexts
   through VText agents and preserve source handles.
8. Ensure article/report VTexts cite/transclude source artifacts natively.
9. Ensure `Wire.vtext` transcludes article/report VTexts with version semantics.
10. Ensure Wire app renders the edition VText graph cleanly on desktop/mobile
    and all three themes.
11. Prove staging behavior: live source status, VText creation/update, edition
    update, source transclusion open, no fake stories.

## Forbidden Shortcuts

- Do not add new mock routes, fake stories, or seed articles to make the UI
  look full.
- Do not keep StoryGraph or SourceMaxx/source-maxxing as hidden product
  authority under renamed labels such as source-network.
- Do not claim RSS sources are full articles when they are feed summaries.
- Do not claim Telegram API ingestion when only preview scraping ran.
- Do not hardcode ranking/prominence to make front-page ordering look plausible.
- Do not write article prose in Wire app/backend outside VText ownership.
- Do not let processors, reconcilers, researchers, or supers write VTexts.
  They may read VTexts/files/source artifacts and message VText agents. Only
  VText agents write VText versions.
- Do not create source manifest or related-story list sections when native
  transclusion is available.
- Do not spend the mission on newsletter/email.
- Do not leave tests asserting old seed behavior.

## Evidence Requirements

Completion requires named evidence for:

- source registry count and source classes after expansion;
- source cycle producing live items from RSS, Telegram API, GDELT, and HN;
- source artifacts containing full/extracted article content where allowed, not
  only headlines;
- platform processor/reconciler runs using shared harness and issuing
  VText/research requests;
- VText agent-created article/report version with source transclusions;
- `Wire.vtext` edition revision transcluding article/report VTexts;
- app rendering edition VText graph with no hardcoded preview stories;
- every article/report openable in VText;
- source transclusion opening native source viewer/content artifact;
- update semantics visible when a current revision differs from a transcluded
  revision;
- responsive UI proof in Future Noir, Carbon Kintsugi, and London Salmon;
- staging deploy SHA identity and acceptance proof.

## Anti-Goodhart Checks

If the final product shows more articles but they are old, fake, uncited,
shallow, or not VText-owned, the mission failed.

If the product shows hundreds of source items but articles do not synthesize and
cite them, the mission failed.

If the app is visually clean but backed by fake stories, the mission failed.

If the backend creates a new parallel object that future agents must reconcile
with VText truth, the mission failed.

If platform and user-computer personalization authority are blurred, the mission
failed.

If source breadth improves but RSS/Telegram/GDELT/HN claims are not proven by
source-cycle evidence, the mission failed.

## Dense Feedback

Use short receding-horizon loops:

- inspect code/data/product evidence;
- update mission report and belief state when a problem is found;
- patch or delete the implicated layer;
- run focused tests;
- deploy/push when behavior changes;
- verify staging product path;
- update report and mission checkpoint.

Use Computer Use/browser product-path probes where screenshots, responsive
layout, source windows, or authenticated Wire behavior matter.

## Rollback Policy

Every behavior-changing commit must be independently reviewable.

Prefer deletion commits that remove fake product behavior before replacement
commits, when the deletion can leave an honest empty state.

If a cutover breaks staging, rollback to the last known deployed SHA and
preserve evidence in the mission report.

Do not edit tracked files directly on Node B as source/config shortcut. Push
through `origin/main` and monitor CI/deploy.

## Mission Report

Create and maintain:

```text
docs/mission-report-wire-community-news-2026-06-09.md
```

At mission end, save a PDF copy to:

```text
~/Library/Mobile Documents/com~apple~CloudDocs/mission reports/
```

## Run Checkpoint And Resumption State

status: checkpoint_incomplete

last checkpoint:

- Wire/cloud/computer ontology specified.
- Problem-documentation-first checkpoint committed as `87f7df56`.
- First behavior slice committed as `205125c9`: active frontend preview stories
  and backend read-time story seeding were removed.
- CI fixture repair committed as `a89f8a48`: legacy route tests now seed the
  story/source/style/article/projection fixtures they need explicitly.
- Forced staging deploy completed for
  `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd` in workflow run
  `27217273257`. Staging `/health` reported proxy and sandbox at that SHA.
- Deployed browser proof opened Global Wire on `https://choir.news/`: zero
  stories, honest empty state visible, data source
  `community-wire-vtext-index`, and no `SourceMaxx newsroom`,
  `seed source neighborhood`, `Port backlog recedes`, or `StoryGraph desk`
  text.
- Second behavior slice implemented locally: Global Wire now uses
  `global-wire/Wire.vtext` as the canonical Community Wire edition alias, and
  platform article VTexts must be transcluded by that edition before they enter
  the front-page story response.
- Edition-gate slice committed as `f6707096`, CI run `27218260845` passed, and
  staging deploy job `80366180237` completed. Staging `/health` reported proxy
  and sandbox at `f6707096cabfdf7e860ceb35483b8335191429f2`.
- Product-path edition update slice committed as `90839193`: approving a
  publication artifact with an approved projection VText now creates a
  platform-owned article VText copy and updates `global-wire/Wire.vtext`.
  Local focused tests, `TestHandleGlobalWire`, and
  `scripts/go-test-runtime-shards` passed. CI run `27219131352` and staging
  deploy job `80369262512` succeeded; staging `/health` reported proxy and
  sandbox at `90839193d04bfd1321d0424ae86930aac437efd5`.
- New authenticated staging evidence after `90839193` found that Global Wire
  can still show three legacy stored seed stories with `seed source
  neighborhood` metadata while the surface reports `community-wire-vtext-index`
  and "awaiting edition VTexts". This proves a remaining owner-scoped
  `GlobalWireStory` fallback still bypasses the edition invariant for
  authenticated users.

current artifact state:

- The deployed Wire app/API path no longer invents seeded front-page stories
  when no VText-owned articles exist.
- The deployed API path now has an edition gate for platform VText articles:
  untranscluded platform VTexts remain invisible to Global Wire, while
  `vtext:<doc_id>` refs in `global-wire/Wire.vtext` include those article
  VTexts in edition order.
- Existing code still contains legacy StoryGraph/SourceMaxx data structures,
  style-source, source-refresh, publication, autoradio, newsletter, and deeper
  compatibility behavior that has not yet been deleted.
- Existing source daemon has broad but shallow source ingestion.
- The front-page response still includes authenticated owner-scoped stored
  `GlobalWireStory` records when no `Wire.vtext` edition exists; this is now
  the immediate blocker.

what shipped:

- Docs-first checkpoint commit `87f7df56`.
- First behavior slice commit `205125c9`.
- Explicit fixture repair commit `a89f8a48`.
- Forced staging deploy for `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`.
- Edition-gated API slice commit `f6707096` deployed to staging.
- Product-path publication approval edition updater commit `90839193` deployed
  to staging.

what was proven:

- Code review identified old seeded paths and source-ingestion gaps.
- Existing docs now distinguish Community Cloud, Private Cloud, platform
  computers, user computers, Wire, and userland personalization.
- Focused local tests and build proved the first deletion slice:
  `nix develop -c go test ./internal/store -run 'TestGlobalWireStoriesDoNotSeedFakeFrontPage'`,
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireStories(ReturnsHonestEmptyState|IndexesSourceNetworkVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest)'`,
  `nix develop -c go test ./internal/store -run '^$'`,
  `nix develop -c go test ./internal/runtime -run '^$'`, and
  `npm run build` in `frontend/`.
- Local browser proof against `http://127.0.0.1:5173/` showed zero stories,
  visible empty edition state, no seed text, no `Port backlog recedes`, and
  `community-wire-vtext-index` as the data source.
- CI run `27217127841` succeeded for the fixture repair.
- Forced deploy run `27217273257` succeeded, including deploy job
  `80362634048`.
- Staging `/health` reported proxy and sandbox build commit
  `a89f8a48807d0f79f05b97e42f08f5ff4c698cfd`.
- Staging browser proof showed the deployed Global Wire empty state, zero
  stories, `community-wire-vtext-index`, and zero occurrences of deleted seed
  texts.
- Local tests proved the edition gate:
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireStories(ReturnsHonestEmptyState|DoesNotIndexUntranscludedPlatformVTexts|IndexesEditionTranscludedVTextHeads|UsesVisibleSourceEntitiesForSourceNetworkManifest)'`,
  `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWire'`,
  `nix develop -c go test ./internal/runtime -run '^$'`, and
  `npm run build` in `frontend/`.
- CI run `27218260845` and deploy job `80366180237` succeeded for `f6707096`.
- Staging `/health` reported proxy and sandbox build commit
  `f6707096cabfdf7e860ceb35483b8335191429f2`.
- Staging browser proof after the backend deploy still showed the honest empty
  Global Wire state with zero legacy seed-text occurrences. The frontend build
  commit stayed at `a89f8a48` because the deploy impact skipped frontend build
  for this backend-only change.
- Authenticated Chrome proof after `90839193` showed the remaining fallback
  problem: Global Wire rendered three durable legacy seed stories and visible
  `seed source neighborhood` metadata despite having no active edition.
- Source-network metadata cleanup committed as `465c9cff`: fresh
  publication-approved Community Wire article revisions now use
  `source_network_*` provenance metadata instead of minting new
  `source_maxx_*` metadata. CI run `27220546359` and deploy job
  `80374232404` succeeded; staging `/health` reported proxy and sandbox at
  `465c9cffb65548b54f834fd9e84737b52cabbc31`.
- Docs evidence checkpoint committed as `b9175495`.
- Authenticated owner-prompt proof attempt after `465c9cff` found the next
  blocker: the live command prompt turned the operational Wire proof request
  into a VText draft/revision and the VText run reported `gateway client:
  fireworks: status 412 Precondition Failed (sanitized)`. Global Wire remained
  at the honest empty state; no deployed positive `Wire.vtext` article
  transclusion was created.
- Prompt orchestration repair locally verified:
  `nix develop -c go test ./internal/runtime -run 'TestInitialVTextToolChoiceUsesExactTools|TestHandlePromptBarVTextRouteCompletesConductorSynchronously|TestHandlePromptBarOperationalProofInitialRunRequestsSuperFirst|TestVTextPromptInitialRevisionUsesSingleWriterLoop'`,
  `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice)'`,
  and `nix develop -c scripts/go-test-runtime-shards`.
- Prompt orchestration repair deploy and failed reprobe: commit `cbfd0637`
  passed CI run `27221392006`; deploy job `80377182065` succeeded; staging
  `/health` reported proxy and sandbox at
  `cbfd0637ab921947bfd5652fbe47411dca20ef79`. An authenticated `Command
  prompt` reprobe still opened a VText document and reported the same
  Fireworks 412 blocker before any visible persistent-super execution handoff.
- Deterministic handoff repair locally verified:
  `nix develop -c go test ./internal/runtime -run 'TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|TestHandlePromptBarVTextRouteCompletesConductorSynchronously|TestInitialVTextToolChoiceUsesExactTools|TestRequestSuper|TestVTextRequestSuper'`,
  `nix develop -c go test ./internal/runtime -run 'Test(VText|HandlePromptBar|InitialVText|RequestSuper|ToolChoice)'`,
  and `nix develop -c scripts/go-test-runtime-shards`.
- Deterministic handoff repair deployed and reprobed: commit `7b7bba73` passed
  CI run `27222135633`; deploy job `80379785630` succeeded; staging `/health`
  reported proxy and sandbox at
  `7b7bba73b000d2eb9ab01a1b5d4b88387a989351`, deployed at
  `2026-06-09T16:58:10Z`. Authenticated Chrome evidence showed a visible
  `Role: super` blocker for the prompt and a remaining Fireworks 412 VText
  blocker; Global Wire stayed empty.

unproven or partial claims:

- Real VText creation from current source cycles.
- Telegram API ingestion.
- Removal of Telegram public preview HTML scraping from the Wire ingestion path.
- Positive deployed edition VText graph rendering; the current deployed surface
  is still the honest empty state until a real `global-wire/Wire.vtext`
  includes article transclusions.
- Product-path creation/update of `Wire.vtext` is implemented and locally
  tested through publication artifact approval, but deployed positive proof now
  needs a live owner-prompt orchestration path that supervises the Wire
  source-to-publication sequence rather than routing the request to VText-only
  drafting.
- The first prompt orchestration repair was deployed and reprobed on staging,
  but it still depends on a VText provider turn and failed with Fireworks 412.
- The deterministic handoff repair is deployed and reprobed, but the super run
  fails before worker delegation/package work and the companion VText run still
  reports Fireworks 412. Positive source-to-edition proof remains blocked.

next step:

- Investigate and repair the persistent-super runtime fallback/model routing so
  the authenticated product prompt can delegate the source-to-publication work
  instead of stopping before worker/package activity, then rerun the staging
  product-path proof that creates or approves a real article VText, updates
  `global-wire/Wire.vtext`, and renders the edition-transcluded article in
  Global Wire.
