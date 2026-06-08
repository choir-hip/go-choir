# Mission: Global Wire Hard Cutover To Real VText Newsroom

**Status:** active MissionGradient draft for the next Global Wire run.  
**Created:** 2026-06-08  
**Supersedes for execution:** `docs/mission-global-wire-living-vtext-newsroom-v1.md`
for the next implementation run. Earlier docs remain evidence, not authority.

## Short Goal String

```text
/goal Run docs/mission-global-wire-hard-cutover-real-newsroom-v0.md and ship the real Global Wire newsroom cutover.
```

## Mission Identity

Global Wire must stop presenting old seed/demo ontology as if it were a
newsroom.

The next mission is a hard cutover. Remove seeded StoryGraph front-page
behavior, delete user-visible StoryGraph language, delete fake source
neighborhoods, and remove any fallback that makes three deterministic stories
look like a working product. Global Wire should either show real article
VTexts backed by real SourceItems or honestly show that no live articles are
available yet.

The product target is not a dashboard, story row database, source chronology,
style control panel, outline generator, or mock newspaper. It is a source-rich
VText newsroom:

```text
many multilingual sources
-> durable SourceItems with full available body/snapshot metadata
-> long-running processors over source flow and continuity state
-> researchers for evidence gaps
-> reconcilers over existing article VTexts and source changes
-> VText agents that own publication-quality article documents
-> Global Wire as a clean newspaper collection view over article VTexts
-> VText app for full reading, editing, source traversal, versioning, and
   publication/forking
```

## Why The Previous Mission Stopped

The prior Choir-in-Choir run was useful but insufficient.

What it proved:

- the multiagent product path can create a mission VText, delegate a worker,
  produce a candidate commit, and publish a reviewable AppChangePackage;
- the worker did delete Global Wire UI detritus at the intent level;
- Codex review caught malformed package transfer and false verification;
- Codex salvaged and landed the deletion payload on GitHub/staging;
- sourcecycled now ingests large volumes and has queue supersession instead of
  stale FIFO backlog.

What it did not prove:

- trusted autonomous adoption of worker patches;
- real article quality;
- reliable VText-agent article ownership;
- front-page ranking by importance, novelty, and prominence;
- full source-body availability in opened source windows;
- removal of old seeded StoryGraph substrate from the user-facing product.

Current high-value evidence:

- deployed `/api/global-wire/stories` can still return exactly three seeded
  records with `source: durable-storygraph`;
- those records expose seeded source URLs such as
  `choir://global-wire/source/source-port-authority`;
- source viewer text still says normalized seed evidence can stand in until
  live Source Service ingestion replaces it;
- this is not a live newsroom and should not be treated as one.

## Cognitive Transform Set

Current obstacle:
The implementation keeps preserving old compatibility paths that make the
product appear alive while hiding the fact that the real article pipeline is
not producing enough publication-grade VTexts.

Selected transforms:

1. **Hard-cutover transform** - compatibility with fake ontology is now a bug,
   not a safety net. Delete seeded product paths and let missing real output be
   visible.
2. **Truth-surface transform** - the UI should report the truth of the
   pipeline. An empty real newsroom is better than a populated mock newsroom.
3. **Article-ownership transform** - ask "which VText agent owns this article
   and why does this version exist?" before accepting any article display.
4. **Source-body transform** - a source is not a title, headline, URL, or seed
   label. A source item must expose actual ingested body, reader snapshot,
   media/transclusion handles, or a precise unavailable reason.
5. **Deletion-bias transform** - remove bespoke surfaces and old concepts
   unless they are required by the new object. No "maybe keep it" language for
   detritus already identified by product review.
6. **Provider-capacity transform** - high-volume model access is an external
   mission resource, not a product compromise. Do not lower the target because
   the current provider budget is exhausted; wait for capacity or run only
   deterministic/source-service work.

Changed route:

- Delete old seeded StoryGraph front-page behavior before adding polish.
- Treat "3 stories" as a failure state unless those stories are real current
  VText articles.
- Make no-live-articles an honest product state.
- Drive article creation through VText agents, not Global Wire handlers.
- Require source windows/transclusions to show body-backed sources or explicit
  body-unavailable state.
- Delay broad autonomous agent loops until cheap high-volume model access is
  available.

## Hard Invariants

- Every Global Wire article is a normal editable VText.
- VText agents own article creation and revision.
- Processor notes, reconciler notes, source briefs, outlines, and research
  packets are not articles unless they are explicitly presented as non-article
  VTexts.
- User edits are ordinary VText edits/forks/versions. No Global Wire "My Edit"
  subsystem.
- Sources are native source items/transclusions, not plain manifest lists.
- Related articles are related/transcluded VTexts, not a bespoke StoryGraph
  relation object shown to the user.
- Corrections and updates are normal new VText versions.
- Global Wire is a collection view. VText is the reader/editor.
- No user-visible `StoryGraph`, `durable-storygraph`,
  `seeded-source-neighborhood`, `source-neighborhood manifest`, source ledger,
  Sources Chronology/search, or bespoke Style.vtext control surfaces.
- Styles are VTexts/sources. Examples matter more than rule bullets. Global
  Wire should not have style radio buttons, `S`, Compose, Replace, or Ask
  controls.
- Source trust is not hardcoded into permanent tiers. Track record is learned
  over time through evidence, corroboration, corrections, freshness, source
  behavior, researcher packets, and model judgment.

## Value Criterion

Minimize fake-newsroom surface area while maximizing real source-backed article
throughput:

```text
loss =
  seeded/mock user-visible paths
+ article stubs presented as articles
+ source titles without source bodies or unavailable reasons
+ article rows not owned by VText documents
+ stale/low-prominence front-page ordering
+ hidden processor/reconciler failure
+ bespoke non-VText editorial state
```

The mission moves uphill when Global Wire shows many real, current,
publication-quality article VTexts with native source transclusions and honest
pipeline status.

## External Resource Precondition

Do not begin a long autonomous agent-heavy run until model access is adequate.
The current Fireworks/DeepSeek access issue and GPT weekly-limit pressure are
real execution constraints.

Acceptable provider capacity routes:

- Fireworks Fire Pass v2 with Kimi K2.6 Turbo, if permitted for personal and
  development use in this workflow and configured without routing production
  hosted-service traffic through a prohibited plan;
- DeepSeek direct pay-as-you-go for V4 Flash/Pro;
- MiniMax M3 token plan for experimental long-context/coding workers;
- Xiaomi MiMo only where its allowed-use terms fit coding-agent usage;
- GPT-5.5 reserved for architecture review, hard root-cause analysis,
  multimodal/product-path judgment, and final PR review.

If provider capacity is not ready, still run deterministic inspections,
source-service probes, docs, and small code deletions. Do not launch processor
or VText-agent article generation at scale on scarce GPT quota.

## Cutover Work

### 1. Delete Old StoryGraph Product Paths

Hard-delete or quarantine user-facing StoryGraph behavior:

- remove seeded StoryGraph front-page fallback from Global Wire;
- remove user-visible `durable-storygraph` source labels;
- remove `seeded-source-neighborhood` freshness/source-state display;
- remove `choir://global-wire/source/source-port-authority` seed source
  behavior from article/source product paths;
- remove or hide seed fixture stories from authenticated owner front pages;
- keep test fixtures only as tests, clearly named as fixtures, never as runtime
  product fallback.

Allowed internal replacement:

- a derived VText/source graph index for performance, ranking, discovery, and
  Autoradio horizon work.

Forbidden replacement:

- a new canonical StoryGraph object that competes with VText articles.

Acceptance:

- authenticated `/api/global-wire/stories` returns either real VText articles
  or an honest empty/no-live-articles state;
- no deployed Global Wire UI or API response shown to users contains
  `durable-storygraph`, `StoryGraph`, `seeded-source-neighborhood`, or seed
  source names as product data.

### 2. Force Real Article VTexts

Global Wire should index article VTexts only when the current revision is a
real article:

- article revision metadata must mark `artifact_kind=article_revision` or an
  equivalent real article state;
- body must contain developed prose, not only headings, bullets, metadata, or
  source inventory;
- source references must be native VText/source refs;
- related VTexts must be native refs/transclusions when editorially relevant;
- article windows should not display metadata sludge, source manifests,
  "Non-oracle note", "My Edit", raw source handle lists, or style rationale as
  body filler.

Acceptance:

- at least one deployed live article opens in VText and reads as article prose;
- source refs in that article open source viewer windows with real body-backed
  source content or explicit unavailable state;
- no brief/stub can appear as a front-page article unless explicitly labeled
  as a non-article note surface outside the article feed.

### 3. Produce Many Articles, Not Three

The target surface should contain many current articles across beats, not a
three-column demo.

Initial target:

- enough source volume to create or update dozens of article VTexts per day;
- front page shows a ranked subset, not all articles;
- archive/list view can expose more article VTexts later, but the immediate
  mission should at least prove more than the old three records.

Ranking must consider:

- prominence/importance;
- novelty;
- freshness;
- source density and corroboration;
- cross-region relevance;
- update/correction urgency;
- contradiction or unresolved-question value;
- diversity across topic/region/source medium so one source class does not
  dominate the entire front page.

Forbidden shortcut:

- generating dozens of shallow stubs to satisfy count.

Acceptance:

- deployed authenticated product shows more than three real live article
  VTexts when article generation capacity is available;
- if fewer than four real articles exist, Global Wire shows honest status
  explaining the article-generation blocker rather than seed fallback.

### 4. Make Sources Real

For every article-visible source ref:

- resolve to a SourceItem/content item;
- show title, original URL when available, source id, fetch id or equivalent,
  timestamp when available, and body classification;
- show actual body/reader snapshot/media/transclusion content when policy and
  extraction allow;
- show a precise unavailable reason otherwise, such as `feed_summary_only`,
  `metadata_packet`, `policy_disallowed`, `fetch_failed`, `parse_failed`,
  `body_empty`, or `auth_required`.

Do not hide weak sources behind polished labels.

Acceptance:

- source viewer opens without mobile deep-link prompts for internal source
  items;
- real article sources are not seed text;
- source-body unavailable states are visible and machine-readable.

### 5. Keep Processors And Reconcilers Real

Processors:

- ingest routed SourceItems by handle;
- maintain continuity state;
- compact old context;
- request researchers or VText agents when evidence or publication need
  requires it;
- do not own canonical articles.

Reconcilers:

- work over existing article VTexts, source changes, processor notes,
  researcher packets, contradictions, and user-published VTexts when in scope;
- request updates/corrections/new articles from owning VText agents;
- do not mutate article text directly.

Acceptance:

- trace shows processor/reconciler/VText handoff for at least one article
  update;
- reconciler can point to an existing article VText and request a normal new
  version when sources change.

### 6. Honest Product Status

Replace fake readiness with explicit status:

- live source counts;
- recent successful/failed fetches;
- current article count;
- last article generation time;
- queued/submitted/superseded processor work;
- article-generation blocker, if any;
- current provider/model availability blocker, if any.

This status should be quiet and readable, not a dashboard panel explosion.

Acceptance:

- when no real article VTexts are indexable, Global Wire says that plainly;
- status does not expose old source ledger/search detritus.

## Forbidden Shortcuts

- Do not keep seed stories as authenticated fallback.
- Do not rename StoryGraph instead of deleting the user-visible ontology.
- Do not create a parallel article table as the canonical article owner.
- Do not call outlines, briefs, or manifests "articles".
- Do not fake source bodies from titles, summaries, or URLs.
- Do not add source chronology/search back.
- Do not add bespoke Style.vtext controls back.
- Do not hardcode source trust tiers.
- Do not claim "many articles" by counting source items, processor notes, or
  non-article VTexts.
- Do not use scarce GPT quota for broad autonomous loops unless the user
  explicitly approves.

## Evidence Requirements

Before any behavior-changing fix:

1. update this mission doc with the specific problem/evidence checkpoint;
2. implement the smallest hard-cutover change that preserves the real object;
3. run relevant local tests/builds;
4. commit and push;
5. monitor CI and staging deploy;
6. verify staging build identity;
7. run product-path proof against `https://choir.news` or owner sandbox;
8. record trace/API/browser evidence.

Required proof for completion:

- commit SHA on `origin/main`;
- CI and deploy success;
- staging health commit identity;
- authenticated `/api/global-wire/stories` proof;
- browser or product-path proof opening Global Wire and article VTexts;
- source viewer proof for at least three real source refs;
- proof that no seed StoryGraph strings appear in product data/UI;
- proof that the front page has many real article VTexts, or an honest blocker
  state if provider capacity is not yet available.

## Rollback Policy

Each hard deletion should be independently revertible, but rollback should be
framed as restoring old fake behavior. Prefer forward fixes unless the app is
unusable.

Rollback refs to record during execution:

- commit that removes seed front-page fallback;
- commit that removes user-visible StoryGraph language;
- commit that enforces article VText indexing;
- commit that changes source viewer behavior;
- commit that changes processor/reconciler article production.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T12:53Z DeepSeek direct provider cutover
  checkpoint recorded before code/config changes.
current artifact state:
  Current staging can still show three seeded Global Wire records from the old
  durable StoryGraph path. Prior cleanup/deletion and source-service
  improvements landed, but the user-facing article surface is not yet a real
  many-article newsroom. The current gateway/runtime model policy still routes
  core roles to Fireworks-hosted DeepSeek V4 Flash/Pro model ids, and staging
  gateway logs show Fireworks returning 412 Precondition Failed. The user added
  a direct DeepSeek API key to local `.env` after topping up the DeepSeek
  account.
what shipped:
  No code shipped by this mission yet. This document defines the next run and
  records the provider cutover problem before code/config changes.
what was proven:
  The previous run proved product-path worker export is possible but not yet
  trustworthy for adoption. It also proved Global Wire still has old seed
  substrate leakage. Official DeepSeek API docs say the OpenAI-compatible base
  URL is `https://api.deepseek.com`, the current model ids are
  `deepseek-v4-flash` and `deepseek-v4-pro`, tool calls are supported, and
  legacy `deepseek-chat`/`deepseek-reasoner` names are deprecated on
  2026-07-24.
unproven or partial claims:
  Real article throughput, source-body completeness, front-page ranking, and
  VText-agent-owned publication-quality article generation remain unproven.
  Direct DeepSeek gateway calls are not yet proven on Node B. Xiaomi MiMo
  direct calls are not yet implemented or proven.
belief-state changes:
  Compatibility with old seeded StoryGraph behavior is now classified as a
  product bug, not a helpful fallback. Fireworks-hosted DeepSeek should no
  longer be the cheap default provider path while direct DeepSeek API access is
  available.
remaining error field:
  Delete old visible ontology and force Global Wire to show real VText
  articles or honest empty/blocker state. First restore affordable model
  capacity by adding DeepSeek as a first-class gateway provider, lifting
  `DEEPSEEK_API_KEY` to Node B through the existing provider credential script,
  and replacing Fireworks DeepSeek V4 Flash/Pro role defaults with direct
  DeepSeek model ids.
highest-impact remaining uncertainty:
  Whether direct DeepSeek accepts Choir's current OpenAI-compatible chat/tool
  request shape, including tool-choice modes and reasoning/thinking parameters,
  without the Fireworks 412 failure mode.
next executable probe:
  Implement direct DeepSeek provider registration and model policy defaults,
  deploy the DeepSeek key to Node B with `nix/deploy-provider-creds.sh`,
  restart gateway/sandbox as needed, and prove a deployed gateway/runtime call
  uses `deepseek/deepseek-v4-flash` or `deepseek/deepseek-v4-pro`.
suggested resume goal string:
  /goal Run docs/mission-global-wire-hard-cutover-real-newsroom-v0.md and ship the real Global Wire newsroom cutover.
evidence artifact refs:
  Current manual API probe showed `/api/global-wire/stories` returning
  `source: durable-storygraph`, three seed records, and seed source URLs.
  DeepSeek docs consulted: `https://api-docs.deepseek.com/`,
  `https://api-docs.deepseek.com/quick_start/pricing/`, and
  `https://api-docs.deepseek.com/api/create-chat-completion`.
  Xiaomi MiMo docs consulted: `https://platform.xiaomimimo.com/docs/en-US/welcome`,
  `https://platform.xiaomimimo.com/docs/en-US/quick-start/first-api-call`,
  `https://platform.xiaomimimo.com/docs/en-US/quick-start/model`,
  `https://platform.xiaomimimo.com/docs/en-US/api/chat/openai-api`,
  `https://platform.xiaomimimo.com/docs/en-US/api/chat/anthropic-api`,
  `https://platform.xiaomimimo.com/docs/en-US/usage-guide/multimodal-understanding/image-understanding`,
  and `https://platform.xiaomimimo.com/docs/en-US/usage-guide/passing-back-reasoning_content`.
rollback refs:
  None yet for this mission. Preserve prior rollback refs in
  `docs/mission-choir-in-choir-platform-pr-accelerator-v0.md`. Gateway secret
  rollback should restore the previous `/var/lib/go-choir/gateway-provider.env`
  backup if the DeepSeek credential lift breaks provider resolution.
```
