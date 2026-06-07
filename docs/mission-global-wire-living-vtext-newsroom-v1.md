# Mission: Global Wire Living VText Newsroom

**Status:** active MissionGradient draft after visual/product correction.  
**Spec:** `docs/choir-global-wire-living-vtext-newsroom-spec-2026-06-07.md`  
**Supersedes:** the earlier source-volume-named mission and the first broad-source draft.
**Created:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-living-vtext-newsroom-v1.md and ship the living VText newsroom on staging.
```

## Mission Identity

Global Wire is a living VText newsroom.

The mission is not to fix the current surface incrementally. The current
surface exposed a deeper failure: article stubs, visible metadata, source
lists, related VText lists, a "My Edit" section, and poor responsive
typography show that the architecture is not yet normalized around VText as
the article owner.

The mission succeeds only when broad source flow produces real article VTexts
through the existing VText agent workflow, with native source and related-VText
transclusions, and the Global Wire app acts as a clean collection surface.

## Real Object

```text
broad multilingual source ingestion
-> durable SourceItems with source embedding/transclusion handles
-> processors with live context over source flow
-> researchers for evidence gaps
-> VText agents that own article creation/revision
-> reconcilers watching ongoing stories and corpus tensions
-> article VTexts with real prose, source transclusions, related-VText
   transclusions, citations, and version-local provenance
-> rebuildable VText/source indexes for discovery/performance
-> Global Wire newspaper collection surface
-> normal VText app for full article reading/editing/source traversal
```

## Cognitive Transform Set

Current obstacle: the implementation has treated article output as a generated
view rather than as a living VText document owned by a VText agent.

Selected transforms:

1. **Article ownership transform:** ask "which VText agent owns this article
   and what caused this version?" instead of "what story row generated this
   text?"
2. **Transclusion transform:** sources and related VTexts are not lists; they
   are graph objects that should enter the article through native VText/source
   transclusion.
3. **Living story transform:** ongoing stories have versions. New source flow
   should create update requests to VText agents, not detached stubs.
4. **Publication-quality transform:** an outline, source manifest, metadata
   dump, or claim list is not progress unless it is clearly an internal brief.
   The article must read as an article.
5. **Source-breadth transform:** hundreds of items from a narrow registry is
   not enough. Source diversity by language, region, medium, beat, sector,
   community, and long-tail social perspective is the first realism axis.
   Source categories are observability metadata, not hardcoded authority.
6. **Collection-surface transform:** Global Wire is the newspaper surface. The
   VText app is the article reader/editor/source traversal surface.

Changed route:

- Start with source breadth and source proof, not UI polish.
- Normalize article lifecycle through VText agents before trying to index or
  render more article rows.
- Require native source and related-VText transclusion in article VTexts.
- Treat reconcilers as ongoing story monitors that message/request VText agent
  updates.
- Fix typography and mobile banner defects as product-quality gates, but do
  not mistake surface cleanup for the architecture being delivered.

## Priority 1: Broad Multilingual Sources

The deployed source service currently proves the substrate, not the target:
one GDELT source, ten RSS/official feeds, and three Telegram public-preview
feeds is not enough.

The first implementation phase must research and expand sources in many
languages.

Acceptance direction:

- maintain GDELT/global-event ingestion;
- add many RSS/Atom feeds across languages, regions, beats, communities, and
  sectors;
- add many Telegram/public-channel sources where policy-compliant, with an
  explicit bias toward long-tail local, regional, conflict, community,
  technology, finance, and social-sentiment channels;
- include official, local, regional, specialist, financial/economic,
  conflict/crisis, science/health, climate, culture, technology, industry,
  hacker/community, open-source, labor, policy, academic, trade, market,
  shipping/logistics, energy, agriculture, and long-tail social sources;
- include Hacker News and comparable technical community surfaces, plus
  non-English technology, science, finance, industrial, and regional media;
- add many Telegram/public-preview channels for local perspective, social
  sentiment, weak signals, rumor surfaces, and sources ignored by established
  outlets; articles must corroborate these rather than treating them as
  standalone publication support;
- expose source registry counts by medium, language, region, beat, community,
  and sector;
- expose latest-cycle active source count, failed source count, per-source item
  counts, latency, freshness, and errors;
- keep hundreds of SourceItems per 15 minutes as a low floor, not the finish
  line.
- do not hardcode source trust tiers or static source standing in the registry;
  track record should be learned over time from outcomes, corroboration,
  corrections, freshness, error history, and researcher/model judgment.
- use prompting and model context to reason about already-known source
  reputations when useful, but keep that reasoning soft, inspectable, and
  revisable rather than encoding permanent tiers in config.

## Priority 2: VText-Owned Article Lifecycle

The current generated VText output is not acceptable:

- `v0` reads as the finished article even though it is only a projection/stub;
- metadata appears in the document body;
- source manifest is plain text instead of transcluded sources;
- related VTexts are listed instead of transcluded;
- "My Edit" appears as a section even though VText is natively editable.

Correct lifecycle:

```text
processor/reconciler/researcher/user intent
-> VText agent receives brief + source handles + related VText handles + style
-> VText agent creates/revises normal article VText
-> article version contains prose + transclusions + citations + provenance
-> Global Wire indexes/displays article excerpt
-> full reading/editing/source traversal happens in VText
```

Do not create a separate Global Wire article store that owns the canonical
article.

## Priority 3: Living Updates And Reconcilers

Ongoing stories get updated as the world changes.

Processors notice developments from source flow. Reconcilers compare new
source state against existing article VTexts and related VTexts. When an
article needs an update, correction, qualification, or follow-up, the
reconciler sends a request to the owning VText agent.

The update produces a normal new VText version. Corrections are good; they are
evidence of living versioned publication.

## Priority 4: UI And Typography

Global Wire UI must stop looking like a dashboard or typography stress test.

Required product fixes:

- remove the old source-volume product label;
- use serif article headlines;
- avoid huge sans headline blocks;
- make normal desktop widths readable, not only wide screens;
- keep mobile inside the Choir desktop/web shell but make it responsive;
- no cards, no border-line grid, no nested panel scrolling;
- source chronology should be quiet and useful, not a heavy dashboard;
- every article has a compact VText affordance;
- no repeated "Open in VText" labels;
- VText mobile banner/menu must not overlap buttons or labels;
- VText article rendering must hide metadata sludge and render source/related
  VText transclusions natively.

## Hard Invariants

- Every article is a normal VText.
- VText agents own article creation and revision.
- User edits are normal user-owned VText revisions/forks/publications.
- No Global Wire "My Edit" section or edit subsystem.
- Platform articles change only through normal VText version/update paths.
- Sources must use native source embedding/transclusion.
- Related VTexts must be transcluded where editorially useful.
- `Style.vtext` is a citeable editorial source selected intelligently.
- News remains non-oracle and provenance-rich.
- Processors, reconcilers, researchers, and VText agents use the shared agent
  harness with role profiles/tool policies.
- The Global Wire app is a collection surface, not the article editor.
- Product-path staging proof is required before claiming behavior.

## Delivery Evidence

Required proof:

- source registry expanded substantially beyond 14 configured sources;
- source registry summarized by type, language, region, beat, and outlet class;
- latest source cycle proves active source count, per-source item counts,
  failures, latency, freshness, and item volume;
- GDELT/global event, RSS/Atom, Telegram/public-channel, Hacker News or
  comparable technical community, official/public, specialist, industry,
  finance, science, and regional source categories are actually running;
- processors receive source batches and preserve source handles;
- researchers are requested and return source-backed packets;
- VText agents create/revise article VTexts as owners, not helper tools;
- article VTexts contain real prose, source transclusions, related VText
  transclusions, and citations;
- reconcilers detect update/correction/follow-up needs and message/request
  VText agent revisions;
- user edits/forks happen through normal VText flows;
- Global Wire UI and VText article view pass visual/product checks on normal
  desktop, wide desktop, and mobile-in-desktop-shell sizes across Futuristic
  Noir, Carbon Fiber Kintsugi, and London Salmon;
- staging health/build identity, CI/deploy status, and product-path
  browser/API acceptance proof are recorded.

## Forbidden Shortcuts

- Do not use the old source-volume label as a product name.
- Do not treat 14 sources as adequate.
- Do not count one high-volume source as source diversity.
- Do not encode permanent trust tiers or source standing in config.
- Do not let long-tail social feeds become article support without
  corroboration, research, or explicit uncertainty.
- Do not call outlines, manifests, or claim lists articles.
- Do not display metadata in article prose.
- Do not list sources/related VTexts where transclusion is required.
- Do not build a Global Wire edit subsystem.
- Do not use VText as a text-generation subroutine while Global Wire owns the
  article.
- Do not claim UI proof while normal desktop widths or mobile menus are broken.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 user visual/product review confirmed the current
Global Wire/VText output is wrong-object work: poor desktop typography,
normal-width layout failure, mobile issues, article stubs, visible metadata,
source lists, related VText lists, a "My Edit" section, and no native source
transclusion. 2026-06-07 source architecture correction removed static source
trust tiers/standing from the registry design and expanded the registry toward
broad RSS/Atom plus long-tail Telegram/social evidence. Staging deployed commit
`1d4029c5` and sourcecycled loaded 170 configured sources. Parser fix commit
`9613991f` handled non-UTF RSS charsets and common malformed entity text.
Source-health commit `a4370fd4` added cycle-linked fetch records,
`source_health`, and `/internal/source-service/global-wire/latest`. Commit
`e8728cb6` attempted the first article-surface normalization: remove old
source-volume naming, drop hardcoded source standing fields, make Global Wire
headlines serif/readable, remove the visible fork button/edit scaffold, carry
source entity handles into VText context, and rewrite seeded Global Wire VTexts
away from metadata/manifest bodies toward prose with inline source refs.

current artifact state: staging source service now runs the expanded registry:
1 GDELT, 110 RSS/Atom, and 59 Telegram public-preview sources across 15
language tags, with tech, science, industry, finance, regional, conflict, and
long-tail social/sentiment sources. This is materially better than the initial
tiny registry but still short of the intended breadth: it needs more
non-English tech/science/finance/industry coverage, Hacker News or comparable
technical community sources, and more long-tail Telegram/social inputs. The
source-health deployed cycle
`cycle_387ee9430ee4e637d7d124a8` at `2026-06-07T19:46:26Z` loaded 170
configured sources, completed in about 5 seconds, produced 3,893 deduped
SourceItems, reported 156 successful fetches, 14 failed fetches, 142
item-producing sources, 99 processor requests, and 1 reconciler request.
Global Wire has a clean-ish collection surface but still exposes old naming and
weak typography; opened article VTexts are projection/stub documents rather
than real living articles. Local browser proof for `e8728cb6` showed the
preview no longer displayed the old source-volume label, `My Edit`, `Source
Manifest`, or visible story metadata; three article rows had compact VText
affordances; headline font was serif; desktop, medium, mobile, and opened
VText mobile checks had no horizontal overflow. This is local evidence only.
Follow-up runtime fix commit `46298119` changed graph-candidate promotion
revisions so promotion/source evidence is carried in structured revision
metadata and `source_entities`, while the visible VText body remains article
prose with native source refs instead of `source_content_id` lines, manifest
mutation text, fork disclaimers, or raw provenance dumps.
Current working checkpoint on 2026-06-07 updates the shared coagent VText
handoff route: processor/reconciler `spawn_agent(role=vtext)` now starts a
Global Wire VText revision run with source-network metadata, creates any
initial handoff as `artifact_kind=source_brief` with `article_version=false`
instead of a fake article `v0`, derives native `source_entities` from
ContentItem/source-service handles, writes source refs into the brief, and
prompts the VText agent to write the canonical publishable article revision
with source refs/transclusions rather than a manifest or outline.

what shipped: prior work shipped source service substrate, processor/reconciler
handoff scaffolding, some VText agent usage, and a cleaner newspaper preview.
Those are substrate only. Latest shipped code also removes the old failed
promotion-body contract that made platform story revisions read like internal
metadata. Current unshipped source changes further normalize the
processor/reconciler-to-VText handoff around VText ownership and native source
entities, while preserving legacy `source_maxx_*` metadata only for
continuation compatibility.

what was proven: source service can ingest GDELT/RSS/Telegram-class items at
substantially larger breadth; staging can deploy and show Global Wire; the
screenshots prove the current article/VText object is not acceptable. Local
validation checked working RSS/Atom and Telegram public-preview URLs before
adding them; `nix develop -c go test ./internal/sources ./internal/cycle`
passed after removing static source tiers/standing from config. CI run
`27102504563` passed and deployed `1d4029c5` to Node B. CI run `27102661286`
passed and deployed parser fix `9613991f`. CI run `27102854328` passed and
deployed source-health proof surface `a4370fd4`; Node B git identity matched
that commit and the new endpoint returned source health for the latest cycle.
Local targeted proof for the promotion revision fix passed
`nix develop -c go test ./internal/runtime -run TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText -count=1`.
Local shard proof passed
`nix develop -c env SHARD_INDEX=0 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`.
GitHub CI run `27103946463` passed for
`4629811947f803a1104b33706186cb9e032ca83e`: Go vet/build, non-runtime tests,
integration-tagged smoke, all four runtime shards, aggregate gate, and staging
deploy. FlakeHub run `27103946461` also passed for the same SHA. The CI
frontend build job was skipped for `46298119` because that final fix was
runtime-only; the earlier frontend-changing commit `e8728cb6` had passed local
`npm run build` before push.
Current local proof for the coagent handoff checkpoint passed
`nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText|TestSystemPromptForGlobalWireProfilesLoadsSharedHarnessPrompts|TestSystemPromptForGlobalWireVTextRunsRequiresArticleHead' -count=1`.
Current local frontend proof passed `npm run build` in `frontend`.

staging identity proof: deploy job `79989704732` completed successfully and
reported proxy, sandbox upstream, and platformd build/deployed commit
`4629811947f803a1104b33706186cb9e032ca83e`, deployed at
`2026-06-07T20:32:11Z`. Node B `/opt/go-choir` also reports that exact git
HEAD with a clean worktree. The deploy job verified service health and public
frontend asset graph.

staging product-path proof: unauthenticated `https://choir.news` browser proof
loaded the deployed signed-out desktop preview with no console errors, no
horizontal overflow at 1440px, no old source-volume label, no `My Edit`, and no
`Source Manifest`; screenshot saved outside the repo at
`/tmp/choir-news-global-wire-staging-desktop.png`. This is only signed-out
surface proof. Authenticated Global Wire proof remains blocked because public
API calls correctly returned 401 and the local Chrome profile does not have
the Codex Chrome Extension installed (`check-extension-installed.js --json`
reported installed=false for selected profile `Default`), so Codex could not
take over an authenticated browser session without bypassing the product path.

unproven or partial claims: full source health, learned source track-record
state, per-source proof surfaces, VText agent producing publication-quality
article heads from the new handoff route, native source transclusion rendering
inside those resulting article revisions, related VText transclusion, living
updates, reconciler-driven revisions on existing articles, and
responsive/typographic quality on authenticated staging.

belief-state changes: source breadth and VText ownership are the first
architectural blockers. UI cleanup alone cannot solve the wrong object.
`e8728cb6` exposed a test-contract mismatch: the old graph-candidate promotion
test still expects classified promotion provenance and fork-separation copy to
appear in the article body, but the new product invariant says article body
prose should not carry metadata/provenance sludge. Promotion provenance should
remain version-local structured metadata and/or source/transclusion context,
not visible manifesto text.

remaining error field: staging source health shows 14 failures, mostly provider
403s from Node B (`rss:arabnews`, France 24 language feeds, `rss:mining_com`,
RFI language feeds, `rss:sciencemag`, `rss:thehindu_news`) plus one parser
failure for `rss:euronews_fr` with illegal control character U+001B. The
source-health `item_count` sums pre-dedupe fetch item counts, while cycle
`item_count` reports deduped SourceItems; this distinction should be named in
the API before treating counts as owner-facing metrics. Next article lifecycle
work remains: normalize article lifecycle through VText agents; replace
source/related lists with transclusions; remove metadata/edit sludge from
article documents; fix typography and mobile banner overlap. CI run
`27103718864` for behavior commit `e8728cb6` failed before staging deploy:
`Go Test (internal/runtime shard 0)` failed
`TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText`
because the generated platform-story revision no longer contains the old body
strings `"front-page prominence changed"`, the promoted content id as plain
text, and `"User-owned forks, edits, and contributions remain separate"`.
Root-cause hypothesis: the test is asserting the old visible-provenance body
contract instead of the new article-body/provenance separation contract, and
the implementation may also need to ensure the same promotion evidence is
present in revision metadata/source entity context.
Resolved by `46298119`: the test now asserts readable article prose plus
source-ref markup in the body and structured promotion/source evidence in
revision metadata/source entities. Remaining blocker is not CI; it is
authenticated staging product proof and the still-incomplete deeper VText-agent
article ownership/source-transclusion mission. The current checkpoint improves
the VText handoff route, but it does not yet prove that the VText agent's edit
result is a full article with rendered source/related-VText transclusions on
staging.

highest-impact remaining uncertainty: how to turn source-health, corrections,
corroboration, freshness, and researcher/model judgment into learned source
track-record state without hardcoded editorial tiers, while keeping long-tail
Telegram/social inputs valuable as sentiment and lead discovery rather than
standalone publication support.

next executable probe: land/deploy the coagent handoff checkpoint, verify the
deployed commit identity, then install/enable the Codex Chrome Extension or
otherwise provide a usable authenticated staging browser session. Run the
deployed Global Wire/VText proof: open Global Wire on `choir.news`, verify
article columns, open every article through the compact VText affordance,
trigger a processor/reconciler-to-VText handoff or equivalent product-path
prompt, and inspect the resulting VText revision metadata/source entities and
rendered source refs/transclusions.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: user screenshots from 2026-06-07 at 14:36-14:39 show
the UI/article/VText failures; staging source service latest observed cycle
had hundreds of items but only 14 configured sources.

rollback refs: prior deployed behavior commit
`4629811947f803a1104b33706186cb9e032ca83e` remains the rollback target until
the current coagent handoff checkpoint lands and deploys.
