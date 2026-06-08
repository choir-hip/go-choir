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
- Global Wire must be launchable as a normal desktop icon, not only through
  the app switcher, tray, prompt, or restored window state;
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

checkpoint 2026-06-07T21:11Z: current run corrected the actively generated
VText article/projection body shape before continuing source expansion or UI
work. The root cause was not only rendering: backend Global Wire constructors
were still writing projection-review scaffolding, StoryGraph ids, approval
notes, style-source lines, claim lists, and source-manifest-like text into
reader-facing VText bodies. The implemented slice changes projection-review
drafts to article revision drafts with source-entity metadata and native
`source:` refs; changes projection approvals to normal article revisions
without an appended approval section; changes composed/replacement style
projection documents to article revisions with source entities and no visible
graph/projection scaffolding; and removes claim-list/style-source prose from
store/client fallback article bodies. Local proof passed
`nix develop -c go test -tags comprehensive ./internal/runtime -run
'TestHandleGlobalWireStyleSourcesComposeAndReplace|TestHandleGlobalWireReconciliationRecordsDecisionWithoutMutatingStoryGraph|TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText'
-count=1` and `npm run build` in `frontend`.

residual truth after the 21:11Z slice: this makes current direct Global Wire
article/projection VText output less wrong, but it does not complete the
desired architecture. Article ownership still needs to normalize around VText
agents producing and revising publication-quality prose from source
briefs/reconciler updates. Broad source ingestion remains priority one:
GDELT, large RSS/Atom catalogs, long-tail Telegram/social, Hacker News and
broader tech sources, science, finance, industry media, and multilingual/local
sources. Do not hardcode source trust tiers; learn track records over time and
let models apply soft contextual judgment. Existing deployed/stored bad VTexts
may remain until migrated or regenerated, so staging proof must inspect newly
generated revisions, not only old documents.

next executable proof after landing this slice: trigger the projection-review
draft/approval or style-compose path on staging, open the resulting VText, and
verify the article body has no visible projection/review/source-manifest
scaffolding while source refs open through the native VText source system.

CI failure checkpoint 2026-06-07T21:16Z: pushed behavior commit
`2c7b7d45939f865913f5effeb21272588949f359` failed CI run
`27104991253` in `Go Test (internal/runtime shard 3)`, job
`79992421865`. The failing test was
`TestHandleGlobalWireStoriesIndexesSourceNetworkVTextHeads`, which still
expected an indexed source-network story's visible claims to contain
`Style.vtext: Global Wire`. That expectation is the old visible-provenance
contract. Under the corrected architecture, style/source provenance belongs in
revision metadata, citations, and source/style transclusions, not reader-facing
claim prose. Staging deploy was skipped because the aggregate Go gate failed.
Next fix: update the test to assert the new provenance separation contract
rather than restoring visible style-source claims.

Deploy failure checkpoint 2026-06-07T21:23Z: pushed fix commit
`573e45361a09ebc83b515ad49e31aa32531b7ebe` passed push CI run
`27105097845`, but deploy was skipped because the head commit was test-only.
Manual forced deploy run `27105154984` reran CI successfully and then failed
the Node B staging deploy job `79992994427` while building the host NixOS
closure. The failing derivation was `sourcecycled-0.1.0`; build output:
`internal/sources/rss.go:14:2: cannot find module providing package
golang.org/x/net/html/charset: import lookup disabled by -mod=vendor`.
Root cause: the earlier RSS charset parser fix added an import that is present
in `go.mod`/`go.sum` but missing from `vendor/`, while the Nix package build
uses vendored modules. Next fix: regenerate/update `vendor/` for
`golang.org/x/net` and verify the sourcecycled/Nix build path, then push and
rerun CI/deploy.

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
Current shipped checkpoint `e6a634c1` on 2026-06-07 updates the shared coagent VText
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
metadata. Shipped checkpoint `e6a634c1` further normalizes the
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
CI run `27104452596` passed for
`e6a634c1c4866f151244a7ac6379e44f7e4bdde6`: Go vet/build,
integration-tagged smoke, non-runtime tests, all four runtime shards,
frontend build, aggregate gate, and staging deploy. FlakeHub run
`27104452597` also passed for the same SHA.

staging identity proof: deploy job `79989704732` completed successfully and
reported proxy, sandbox upstream, and platformd build/deployed commit
`4629811947f803a1104b33706186cb9e032ca83e`, deployed at
`2026-06-07T20:32:11Z`. Node B `/opt/go-choir` also reports that exact git
HEAD with a clean worktree. The deploy job verified service health and public
frontend asset graph.
Follow-up deploy job `79991079599` completed successfully for `e6a634c1`.
Deploy logs reported proxy, sandbox upstream, and platformd build/deployed
commit `e6a634c1c4866f151244a7ac6379e44f7e4bdde6`, deployed at
`2026-06-07T20:52:55Z`. Node B `/opt/go-choir` reports that exact git HEAD
with a clean worktree, and direct health checks on ports 8082 and 8086 returned
that commit for proxy/sandbox/platformd.

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
After the Chrome extension became available, authenticated staging browser
proof loaded the Choir desktop at `https://choir.news/` on deployed
`e6a634c1` with no browser console errors and no horizontal overflow in the
observed narrow desktop shell. This proof also confirmed the residual product
failure remains for existing article documents: the opened Global Wire VText
still showed `v0`, visible style/story metadata in prose, a `Projection`
section, and claim-list structure instead of a finished article with native
source/related-VText transclusions. That is not a regression from this
checkpoint; it is the next article-quality/VText-agent output target.

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

next executable probe: trigger a processor/reconciler-to-VText handoff through
the product path or an approved internal newsroom control, then inspect the
resulting VText revision metadata/source entities and rendered source
refs/transclusions on authenticated staging. If that path produces another
stub/outline, root-cause the VText agent edit prompt/tool result path before
touching UI again.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: user screenshots from 2026-06-07 at 14:36-14:39 show
the UI/article/VText failures; staging source service latest observed cycle
had hundreds of items but only 14 configured sources.

rollback refs: prior deployed behavior commit
`4629811947f803a1104b33706186cb9e032ca83e` remains the rollback target for the
coagent handoff checkpoint. Current deployed behavior commit is
`e6a634c1c4866f151244a7ac6379e44f7e4bdde6`.

## Checkpoint 2026-06-07T21:45Z: deployed article-body normalization failed on existing VTexts

objective: finish the Global Wire article body normalization mission on
staging, not merely in constructors or tests.

what changed: commits through `e09895ea64557bb7564fe2b5dd77ff2c34cb1042`
were pushed and deployed. CI run `27105344630` passed all Go/runtime/frontend
gates and completed Node B staging deploy. Deploy log reports proxy, sandbox,
and platformd build/deployed commit
`e09895ea64557bb7564fe2b5dd77ff2c34cb1042`; public frontend serves asset
`index-DyjtbGv3.js`.

evidence: authenticated Chrome product-path proof claimed an existing
`https://choir.news/` tab and inspected the open Choir desktop. Global Wire was
open and each front-page article had an icon-only `Open article VText` button.
The open Global Wire article VText for `Port backlog recedes as carriers warn
of uneven inland recovery` still rendered `v0` and still contained reader-facing
scaffolding: `Style source: Style.vtext: Global Wire`, `Projection`, `Claims`,
`Source Manifest`, `Related VTexts`, `Non-oracle note`, and `My Edit`.

root cause hypothesis: the code normalized new seed/projection/review body
constructors, but `ensureDefaultGlobalWireStories` returns as soon as any
owner-scoped `global_wire_story_graphs` rows exist. Existing durable seeded
article documents therefore keep their old current revisions forever unless a
new VText revision is explicitly created. This is a persistence/migration
failure, not just a frontend rendering issue.

remaining error field: shipped staging is not acceptable yet because the
owner-visible article VText remains the wrong object. The next fix must create
normal VText revisions for stale Global Wire seed/projection documents when
their current body contains known scaffolding, preserving history through
parent revision links and structured metadata/source entities rather than
mutating old revisions or platform stories.

next step: implement an idempotent stale-body repair path in the store seed
ensure flow, test it against a preexisting stale revision fixture, push, deploy,
and rerun authenticated staging proof.

## Checkpoint 2026-06-07T22:08Z: stale article repair deployed and proven, deeper newsroom mission remains

objective: verify the stale-body repair on staging through the authenticated
product path and fold the latest architecture corrections into the next
mission state.

what changed: repair commit `8524ea3676b706b6fbe1195293b288498467c032`
was pushed to `origin/main`. CI run `27105920988` passed Go vet/build,
integration-tagged smoke, non-runtime tests, all four runtime shards, and the
aggregate gate. Staging deploy job `79995032501` completed successfully.
Node B `/opt/go-choir` reports git HEAD
`8524ea3676b706b6fbe1195293b288498467c032`; direct health checks on
platformd port 8086 and proxy/sandbox port 8082 returned that same deployed
commit.

product-path evidence: authenticated Comet browser proof opened
`https://choir.news/`, switched to the existing Global Wire window, and clicked
the first story's compact VText affordance. The opened article window for
`Port backlog recedes as carriers warn of uneven inland recovery` rendered
`v1`, with `v0` available as an older version. The visible body was prose:
headline, lead paragraph, follow-up paragraph, and a source-neighborhood
sentence with native source buttons for `Port authority throughput bulletin`,
`Rail dwell dashboard`, `Regional exporters report delays`, and `Ambient
corpus: shipping and retail filings`. The visible body no longer contained the
old scaffold tokens `Style source`, `Story id`, `Projection`, `Claims`,
`Source Manifest`, `Related VTexts`, `Non-oracle note`, or `My Edit`.

what remains true: this repair makes the seeded article VText less wrong; it
does not deliver the newsroom. The article is still too thin for publication
quality. The Global Wire app surface still reports only `16 sources` and
`4 source groups`, so it is not wired to the expanded source-service reality
or the intended many-source live newsroom. Existing source labels such as
`lead`, `supporting`, `contrary`, and `context` are editorial/source-neighborhood
roles in the seed data, not learned source reliability tiers; the next
architecture must not hardcode source trust or source standing. Long-tail
Telegram/social feeds remain a priority source class for sentiment, weak
signals, local/community views, rumor surfaces, and perspectives ignored by
established outlets, while article support must come from corroboration,
research, explicit uncertainty, and living corrections.

UI evidence: Comet also confirmed Global Wire is available in the app switcher
and as an open tray/window, but not as a normal left-side desktop icon. The app
registry has `global-wire.launcher.desktopIcon = false`; the next frontend
slice should make Global Wire a desktop icon and verify it in the actual
desktop shell.

next executable mission: source breadth first, then VText-owned article
lifecycle. Connect Global Wire display counts to real source-service health;
add broad multilingual RSS/Atom, GDELT/event, Hacker News/comparable tech
community, science, finance, industry, local/regional, and long-tail Telegram
sources without static trust tiers; then drive processor/reconciler/researcher
briefs into VText agents that produce full article revisions with native
source and related-VText transclusions. Use Comet for authenticated
Computer Use proof unless a stronger product-path browser session is available.

## Checkpoint 2026-06-07T22:24Z: desktop icon and live source-status wiring shaped locally

objective: remove two concrete product lies found in Comet proof: Global Wire
was not a desktop icon, and the app header still presented the seeded
16-source article neighborhood as if it were the live source system.

local change: Global Wire is now configured as a normal desktop-icon app in
the shared app registry. The runtime exposes a neutral
`/api/global-wire/source-status` product route, with the older
`/api/global-wire/sourcemaxx-status` kept only as a compatibility alias. The
Global Wire UI now fetches the neutral route when authenticated and, when the
source service is available, displays live source count, source item count,
processor request count, reconciler request count, and latest cycle id instead
of only the seeded story-manifest count. The source chronology rows still
represent the visible article source neighborhood.

local proof: `nix develop -c go test -tags comprehensive ./internal/runtime
-run
'TestHandleGlobalWireSourceMaxxStatusReportsAggregateHandoffs|TestHandleGlobalWireSourceMaxxStatusResolvesRemoteRuntimeEvidence|TestHandleGlobalWireSourceMaxxStatusReportsUnconfiguredSourceService'
-count=1` passed. `npm run build` in `frontend` passed.

remaining proof: commit, push, monitor CI/deploy, verify Node B deployed
identity, then use Comet to confirm the left desktop icon includes Global Wire
and the authenticated app header reports the source-service count rather than
`16 sources`.

## Checkpoint 2026-06-07T22:30Z: live source status deployed, minor copy defect observed

objective: verify the desktop-icon/live-source-status slice on staging.

what shipped: commit `13b0f6bf709b6f40fb765ccf0a1a706c9504769e` was pushed
to `origin/main`. CI run `27106447613` passed Go vet/build,
integration-tagged smoke, non-runtime tests, all four runtime shards, frontend
build, aggregate gate, and Node B deploy job `79996448462`. Node B health
checks confirmed proxy, sandbox, and platformd deployed commit
`13b0f6bf709b6f40fb765ccf0a1a706c9504769e`.

product-path evidence: authenticated Comet reload of `https://choir.news/`
showed Global Wire in the left desktop icon column between VText and Podcast.
The Global Wire header now shows `170 live sources` and
`532 source items · 23 processors · 1 reconcilers`; source chronology count
shows `532`, and the latest source cycle id is visible. This proves the app is
reading live source-service status instead of only the seeded 16-source story
neighborhood.

remaining error field: the visible copy says `1 reconcilers`. Next tiny fix:
pluralize processor/reconciler labels and simplify the source-cycle line so
the product surface reads cleanly before continuing deeper source/article work.

## Checkpoint 2026-06-07T22:38Z: source-status copy polish deployed; authenticated reread limited by signed-out reload

objective: land the small visible-copy repair identified in the prior
product-path proof without pretending the deeper newsroom is done.

what shipped: commit `9b2918711338d7c9f27d8b4368f8f626efd9392d`
was pushed to `origin/main`. CI run `27106562912` passed Go vet/build,
integration-tagged smoke, non-runtime tests, all four runtime shards,
frontend build, aggregate gate, and deploy job `79996768260`. Node B
`/opt/go-choir` reports git HEAD
`9b2918711338d7c9f27d8b4368f8f626efd9392d`; auth and proxy health probes
responded after deploy. A direct local probe to `127.0.0.1:8080` did not
connect during the post-deploy check, so future staging identity checks should
prefer the known live platform/proxy health ports or the public build identity
route rather than assuming that port is always present.

code-level fix: the Global Wire source summary now formats singular/plural
counts through `formatCount`, so `1 reconciler` is rendered instead of
`1 reconcilers`; the latest source-cycle line now renders
`completed · cycle_id` rather than `completed · cycle cycle_id`.

product-path evidence: Comet reload of `https://choir.news/` showed the
Global Wire launcher in the left desktop icon column, proving the desktop-icon
slice survives a fresh page load. The same reload landed in the signed-out
preview state, so the authenticated Global Wire window from the earlier proof
was not available for rereading the source-status copy after the reload. Do
not treat this checkpoint as full authenticated UI proof of the copy polish;
it is deployed code plus signed-out desktop-launcher proof.

remaining error field: the mission remains open. The current shipped product
still falls short of the intended Global Wire newsroom: source breadth must
expand materially across multilingual RSS/Atom, GDELT/event streams, Hacker
News/comparable tech communities, science, finance, industry, local/regional,
and long-tail Telegram/social feeds; processors and reconcilers must use the
shared agent harness to request research and VText-owned article revisions;
articles must become publication-quality living VTexts with native source and
related-VText transclusion; and the newspaper collection surface plus VText
mobile toolbar still need visual/product proof across themes and viewports.

## Checkpoint 2026-06-07T22:52Z: source registry expanded and deployed; feed-quality gaps documented

objective: move the first realism axis upward by increasing actual source
breadth without reintroducing static source trust tiers.

what shipped: commit `1ce64fe6bebd694914ca583899e11bdea25c270f`
expanded `configs/sources.json` from 170 to 211 configured sources: 1 GDELT,
137 RSS/Atom feeds, and 73 Telegram public-preview channels. The added batch
targets Hacker News/community surfaces, non-English/open-source technology,
science/health, finance/economics, semiconductors, data centers, energy,
agriculture, logistics, and long-tail Telegram/social channels. The config has
zero `tier` or `source_standing` fields. `cmd/sourcecycled` now has a
registry-shape regression test that fails if the tracked Global Wire source
config drops below broad coverage, loses required source classes/verticals, or
reintroduces static tier/standing fields.

local proof before push: `jq` counted 211 sources, 137 RSS, 73 Telegram, and
no forbidden tier/standing fields. `nix develop -c go test ./cmd/sourcecycled
./internal/cycle ./internal/sources -count=1` passed. `nix develop -c go build
./cmd/sourcecycled` passed. A bounded local live `sourcecycled` cycle using the
expanded config fetched all 211 sources, recorded 210 `ok` fetches and 1
error, produced 5,314 deduped SourceItems, queued 131 processor requests and 1
reconciler request, and proved sample added sources produced items:
`rss:hn_best` 30, `rss:hn_ask` 20, `rss:semiconductor_digest` 100,
`telegram:androidpolice` 20, and `telegram:sciencealert` 20. Candidate
Telegram channels that fetched but produced zero parsed posts were removed
before commit and replaced with channels that produced parsed SourceItems.

staging proof: CI run `27107122236` for
`1ce64fe6bebd694914ca583899e11bdea25c270f` passed Go vet/build, non-runtime
tests, integration-tagged smoke, all four runtime shards, aggregate gate, and
Node B deploy job `79998280003`. Node B `/opt/go-choir` reports git HEAD
`1ce64fe6bebd694914ca583899e11bdea25c270f`; deployed `configs/sources.json`
reports 211 sources by type: 1 GDELT, 137 RSS, 73 Telegram, with zero
forbidden tier/standing fields. Source Service health reported `status=ok`,
`item_count=49480`, `fetch_count=4740`. Latest deployed source cycle
`cycle_194dad3060e4425410f0483c` started `2026-06-07T22:48:22Z`, completed
`2026-06-07T22:48:28Z`, fetched 211 sources, produced 4,947 new deduped items,
queued 123 processor requests, and queued 1 reconciler request. Runtime
dispatch had a startup-race refusal on attempt 1 while runtime port 8085 was
not yet accepting connections, then recovered enough to submit 7 processor
runs and 1 reconciler run; 116 processor requests remain queued by the
configured dispatch limit rather than by a hard failure.

new documented problems before further fixes: staging logs show several
pre-existing RFI feeds and `rss:arabnews` currently return HTTP 403 from Node
B, and `rss:euronews_fr` returns content containing illegal control character
`U+001B`, causing the RSS parser to fail that feed. These are source-quality
and parser-hardening problems, not blockers for the expanded registry slice:
the latest deployed cycle still completed with 4,947 items. If fixed in this
mission, the first follow-up should sanitize illegal XML control characters in
RSS-like feeds and decide whether 403 sources should stay active, be retried
with a different fetch policy, or be marked temporarily degraded.

remaining error field: source breadth is materially better, but the mission is
still incomplete. The next highest-impact axis is to turn processor/reconciler
handoffs into VText-agent-owned, publication-quality article revisions with
native source and related-VText transclusions, then prove the Global Wire
collection surface opens those living articles cleanly across the product
viewports and themes.

## Checkpoint 2026-06-07T22:59Z: RSS control-byte parser hardening deployed

objective: repair the source-quality issue documented in the previous
checkpoint where `rss:euronews_fr` failed because the feed contained an XML
illegal control byte.

what shipped: commit `4bf7ad5ff1f4d0e03a8ad403b271548de26ef648` strips
XML-forbidden low control bytes before RSS/Atom XML decoding while preserving
tab, newline, carriage return, and the existing charset decoder path. A new
`internal/sources` regression test covers a feed title containing byte
`0x1B` and verifies the item still parses as ordinary text.

proof: local `nix develop -c go test ./internal/sources ./internal/cycle
./cmd/sourcecycled -count=1` passed. A bounded local sourcecycled cycle over
the 211-source registry produced 5,347 deduped items and showed
`rss:euronews_fr|ok|50`; only `rss:arabnews` remained an HTTP 403. CI run
`27107297555` passed Go vet/build, non-runtime tests, integration-tagged
smoke, all four runtime shards, aggregate gate, and Node B deploy job
`79998733503`. Node B `/opt/go-choir` reports git HEAD
`4bf7ad5ff1f4d0e03a8ad403b271548de26ef648`. Latest deployed source cycle
`cycle_f2fd9198e6af3790b377d11d` started `2026-06-07T22:56:02Z`, completed
`2026-06-07T22:56:09Z`, fetched 211 configured sources, produced 4,982
deduped SourceItems, queued 123 processor requests, and queued 1 reconciler
request. Node B sourcecycled logs after deploy show the remaining
`rss:arabnews` 403 but no `rss:euronews_fr` parse error. Direct SQLite
inspection on Node B was unavailable because `sqlite3` is not installed there;
service API and journal evidence are the authoritative deployed proof for
this checkpoint.

remaining error field: one or more feeds can still be temporarily degraded by
upstream 403 behavior, and only the first seven processor requests are
submitted immediately because the staging dispatch limit is seven. The deeper
mission remains unchanged: the source graph is now wider, but Global Wire
still needs VText-agent-owned full article revisions, native source and
related-VText transclusions, living reconciler updates, and product-path UI
proof across the required themes and viewports.

## Checkpoint 2026-06-07T23:17Z: VText article ownership transition fault found

objective: move from source breadth into the article lifecycle, specifically
the processor/reconciler to VText handoff that should produce normal,
publication-quality article revisions.

finding: the current handoff correctly creates a non-article source brief as
the VText seed context, with `artifact_kind=source_brief` and
`article_version=false`. The bug is that these durable metadata fields are
then carried forward into the first VText agent `edit_vtext` mutation. That
means the visible document can be rewritten as article prose while its stored
revision still classifies itself as pre-article source-brief context. This is
an architecture fault, not a typography fault: it confuses the VText-owned
article boundary and can make later renderers, indexes, reconciliation logic,
and product proof treat a real article as a brief.

evidence: `submitVTextAgentRevisionRun` copies durable metadata from the
current revision into the VText run, and `buildAppagentRevisionMetadata`
copies durable metadata from both the parent revision and run into the new
appagent revision. `durableMetadataKeys` includes `artifact_kind`,
`article_version`, and `vtext_version_stage`. The processor/reconciler route
sets the seed brief to `artifact_kind=source_brief`,
`article_version=false`, and `vtext_version_stage=pre_article_brief`, while
the VText prompt requires the first `edit_vtext` call to write a publishable
article. No later normalization currently converts the stored revision
metadata to an article revision for the Global Wire article request.

required correction: when a VText agent run is explicitly a Global Wire
article revision request, the resulting `edit_vtext` revision must mark the
new revision as `artifact_kind=article_revision`, `article_version=true`, and
an article-stage VText version, while preserving source entities, source
network IDs, selected Style.vtext source context, worker-update metadata, and
the normal edit provenance. This must be tested through the VText tool path,
not just by checking prompt text.

remaining error field: after metadata normalization is fixed, the mission
still needs article-body quality and native transclusion proof: article
content should be prose with inline `source:` refs and editorially useful
related-VText transclusions, not outlines, source manifests, raw related-ID
lists, or app-specific edit sections.

## Checkpoint 2026-06-07T23:26Z: VText-owned article metadata transition fixed locally

objective: ensure processor/reconciler handoffs remain non-article source
briefs until the VText agent writes the article, then classify the resulting
`edit_vtext` revision as the article object.

what changed: Global Wire VText revision runs whose request intent matches
`global_wire_*_article_revision` now normalize the appagent-created revision
metadata at `edit_vtext` commit time. The stored revision becomes
`artifact_kind=article_revision`, `article_version=true`, and
`vtext_version_stage=article_revision`. The correction is intentionally narrow:
it does not change ordinary VText edits, email VTexts, or non-Global-Wire
worker wakeups, and it still preserves source entities, source-network IDs,
selected Style.vtext context, worker-update metadata, and normal edit
provenance.

proof: the comprehensive processor/reconciler harness test now executes the
real VText tool path: a processor spawns a VText article document, the seed
revision remains `source_brief` and `article_version=false`, the VText agent
calls `edit_vtext` with prose containing a native `source:` reference, and the
stored head revision is asserted to be an `article_revision` with
`article_version=true` while keeping source entities, cycle ID, processor key,
and selected Style.vtext rationale. Local proof commands passed:
`nix develop -c go test -tags comprehensive ./internal/runtime -run
TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText
-count=1`,
`nix develop -c go test -tags comprehensive ./internal/runtime -run
'TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText|TestSystemPromptForGlobalWireProfilesLoadsSharedHarnessPrompts|TestSystemPromptForGlobalWireVTextRunsRequiresArticleHead'
-count=1`, and
`nix develop -c go test ./internal/runtime -run
'TestBuildAppagentRevisionMetadata|TestVText|TestGlobalWire' -count=1`.

remaining error field: this fixes the VText ownership boundary for revision
metadata, but does not yet prove that live model-authored Global Wire articles
are long-form publication-quality prose or that related VTexts are transcluded
in the rendered reader. The next realism axis is content quality plus rendered
source/related transclusion proof in Comet on staging.

## Checkpoint 2026-06-07T23:32Z: article body depth remains below product bar

objective: verify the visible Global Wire article object after the VText
ownership metadata correction.

finding: Comet on staging proves the Global Wire desktop icon and collection
surface are present, and the opened article is a normal VText at `v1` with
native source buttons. The content is still below the article bar: the seeded
body is only a headline, dek, one projection paragraph, and a compact
source-neighborhood sentence. It is readable, but it is not yet publication
quality and still behaves like a thin article stub.

evidence: Comet accessibility state for `https://choir.news/` showed the
desktop icon button `Global Wire`, the Global Wire app with `211 live sources`,
`4,982 source items`, `123 processors`, and `1 reconciler`, and an opened
VText window for `Port backlog recedes as carriers warn of uneven inland
recovery` at `v1`. The VText body contained native source buttons for `Port
authority throughput bulletin`, `Rail dwell dashboard`, `Regional exporters
report delays`, and `Ambient corpus: shipping and retail filings`, but the
article text was essentially the deterministic seed generated by
`globalWireStoryVTextContent`.

required correction: the seeded/fallback Global Wire article body should be
substantially more article-like until live VText-agent authored articles
replace it: multiple coherent prose sections, explicit uncertainty/correction
framing, native inline `source:` refs, selected Style.vtext context in prose,
and editorially useful related-VText transclusion instead of raw related-ID
lists. Existing thin seeded articles need a repair trigger so owner computers
advance from the stub body to the fuller fallback body without displaying
metadata manifests or app-specific edit sections.

remaining error field: a deterministic fallback article is still not the final
model-owned newsroom behavior. The mission must continue toward live
processor/reconciler handoffs that cause VText agents to write and revise real
articles from incoming sources. The fallback repair is acceptable only as a
visible product floor, not as the end architecture.

## Checkpoint 2026-06-07T23:40Z: fallback article prose repaired locally

objective: raise the visible fallback article floor while preserving the
architecture that live articles should be written and revised by VText agents.

what changed: the seeded/repaired Global Wire article body now writes a fuller
article instead of the prior thin stub. It includes multiple prose paragraphs,
lead and secondary source framing, uncertainty/correction framing, bounded
working claims, claim-audit and market-read paragraphs, background context,
related-VText prose, and the living-VText revision note. Existing thin articles
that contain the old `The current version keeps ... source neighborhood`
sentence now repair forward into the fuller fallback body. The fallback uses
native inline `source:` refs for available source entities and avoids source
manifests, raw related-ID lists, and app-specific edit sections.

proof: `nix develop -c go test ./internal/store -run
'TestGlobalWireExistingArticleVTextBodyRepair|TestGlobalWireThinArticleVTextBodyRepairsForward'
-count=1` passed. Full `nix develop -c go test ./internal/store -count=1`
passed. Runtime handlers and prompt checks passed with
`nix develop -c go test -tags comprehensive ./internal/runtime -run
'TestHandleGlobalWireStoriesSeedsDurableStoryGraphAndVTexts|TestHandleGlobalWireStoriesIndexesSourceNetworkVTextHeads|TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText|TestSystemPromptForGlobalWireProfilesLoadsSharedHarnessPrompts|TestSystemPromptForGlobalWireVTextRunsRequiresArticleHead'
-count=1`.

remaining error field: this is still a deterministic product floor, not the
final newsroom behavior. The next proof must deploy this repair, verify in
Comet that existing article VTexts advance to fuller prose with native source
buttons, and continue toward live VText-agent article generation from the
expanded source network.

## Checkpoint 2026-06-07T23:49Z: fallback article repair deployed; live article proof still partial

objective: deploy the fallback article-depth repair and prove the staging
identity and product surface.

deployed proof: commit `9f7a6603eb432f4098e0cc2b95705910083af77c` passed CI
run `27108134124`: integration smoke, Go vet/build, non-runtime tests, all
four runtime shards, aggregate gate, and Node B deploy job `80000974315`.
FlakeHub publish run `27108134106` also succeeded. `https://choir.news/health`
and Node B `/opt/go-choir` both report deployed commit
`9f7a6603eb432f4098e0cc2b95705910083af77c` with deployed_at
`2026-06-07T23:38:36Z`. On the deployed checkout,
`nix develop -c go test ./internal/store -run
TestGlobalWireThinArticleVTextBodyRepairsForward -count=1` passed.

Comet/product-surface proof: Comet on `https://choir.news/` shows the
authenticated desktop with a `Global Wire` desktop icon. Before this deploy,
opening Global Wire in Comet showed the live source surface with `211 live
sources`, `4,982 source items`, `123 processors`, and `1 reconciler`, and the
opened article VText showed native source buttons. After this deploy, Comet
reloaded to the desktop and still shows the `Global Wire` icon, but the
Computer Use click tool refused to interact after `get_app_state` with
`Computer Use is not active`, so the live owner article could not be reopened
through Comet in this turn. The deployed code and deployed-checkout test prove
the repair path; the actual Comet reopened-article visual proof remains
pending.

additional observed issue: the Comet prompt stream displayed a stale or
separate VText run message saying a run called `edit_vtext` and then hit a
gateway provider HTTP failure. Node B gateway health is currently `ok`, and
gateway logs after deploy show Fireworks inference succeeding. The same logs
also show repeated `gateway: search outage` lines for researcher-style
queries. That search-provider instability is relevant to the research axis and
should be treated as a future documented problem before any search fix.

remaining error field: source breadth, desktop entry, VText-owned article
metadata, and fallback article depth have all moved forward. The mission is
still not complete: live model-authored Global Wire articles need product-path
proof, related VTexts need first-class transclusion rather than prose-only
references, Comet/browser interaction needs a reliable proof path, and gateway
search outages can block researcher-backed reconciliation.

## Checkpoint 2026-06-08T00:17Z: related VText transclusion slice deployed

status: checkpoint_incomplete

objective: move Global Wire article VTexts from prose-only related-story mentions toward native VText graph walking by adding related-VText entities and inline `vtext:` transclusion refs, while preserving native source refs and the clean newspaper surface.

what shipped: commit `de464cf4dc70aaa4a7401f72e8bfc4284f5d5e99` adds metadata-backed related VText refs to Global Wire article revisions. Existing fallback article heads that contain the prior `read alongside the related...` prose now repair forward to article text with inline `vtext:` refs and `related_vtexts` revision metadata. The VText markdown renderer now understands `[label](vtext:<doc_id>)` as a native non-editable related-VText transclusion ref with a compact inline snapshot. The serializer preserves those refs through normal VText editing. The Global Wire preview/open path now passes related VText metadata to VText and no longer synthesizes the old thin `The current version keeps...` article stub.

what was proven locally: `nix develop -c go test ./internal/store -run 'TestGlobalWireExistingArticleVTextBodyRepair|TestGlobalWireThinArticleVTextBodyRepairsForward' -count=1` passed; `nix develop -c go test ./internal/store -count=1` passed; `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText|TestSystemPromptForGlobalWireProfilesLoadsSharedHarnessPrompts|TestSystemPromptForGlobalWireVTextRunsRequiresArticleHead|TestHandleGlobalWireStoriesSeedsDurableStoryGraphAndVTexts|TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText' -count=1` passed; `npm run build` passed; `PLAYWRIGHT_BASE_URL=http://127.0.0.1:4173 npm run e2e -- global-wire-app.spec.js --reporter=line` passed; `npm run e2e -- vtext-source-entities.spec.js:85 --reporter=line` passed.

staging proof: GitHub CI run `27108895850` passed integration smoke, Go vet/build, non-runtime tests, all runtime shards, frontend build, aggregate gate, and Node B deploy job `80003005352`. FlakeHub run `27108895837` succeeded. `https://choir.news/health`, Node B `/health`, and `/opt/go-choir` all reported deployed commit `de464cf4dc70aaa4a7401f72e8bfc4284f5d5e99` with deployed_at `2026-06-08T00:07:37Z`. On the deployed checkout, `nix develop -c go test ./internal/store -run "TestGlobalWireExistingArticleVTextBodyRepair|TestGlobalWireThinArticleVTextBodyRepairsForward" -count=1` passed. A deployed Playwright product-surface probe against `https://choir.news/` double-clicked the `Global Wire` desktop icon, opened the first article VText, and observed `3` stories, `5` native source refs, `1` related-VText ref, `0` `SourceMaxx newsroom` labels, and no `Source Manifest`, `My Edit`, or `The current version keeps` text in the opened article.

partial or unproven claims: the Comet browser remains readable but not clickable through the Computer Use plugin in this turn; both `Comet` and `/Applications/Comet.app` returned `Computer Use is not active` immediately after `get_app_state`, so the Comet authenticated visual proof is still partial. The deployed public browser probe proves the desktop icon, collection surface, VText open path, and renderer behavior, but it does not prove the authenticated owner article with live source counts after repair. The related-VText transclusion is currently a compact inline snapshot/ref, not yet a full passage-range VText-to-VText transclusion with source-span selection or click-to-open behavior. Live model-authored VText articles and reconciler-driven update revisions are still not proven.

remaining error field: source breadth, article metadata, fallback article depth, desktop launch, native source refs, and first related-VText refs are now advanced. The mission still needs live VText-agent-owned article generation/revision from processor/reconciler/researcher handoffs, authenticated Comet/product-path proof, first-class related VText passage transclusion/open behavior, and continued source/research reliability work including the previously observed gateway search outages.

next executable probe: add click/open behavior for `vtext:` refs so related article refs open the target VText document in the normal VText app, then prove it on staging with a deployed browser probe and, if the Computer Use plugin state issue clears, in Comet.

## Checkpoint 2026-06-08T00:28Z: related VText refs now open normal VText windows

status: checkpoint_incomplete

objective: make related-VText transclusions act like first-class VText graph edges from the reader/editor surface: clicking a related article ref should open the target article as a normal VText window instead of remaining a passive inline snapshot.

what shipped: commit `50eeed58b47692f3f1e6518e2797b0d85ad10153` adds pointer/click/keyboard handling for `[data-vtext-related-ref]` entities in `VTextEditor.svelte`. A related ref now launches the VText app with the target document id when authenticated, or with a preview snapshot when signed out, using the same `launchapp` path as other desktop apps. Source refs keep their existing inline source behavior. The Global Wire Playwright coverage now opens the first article and clicks its related VText transclusion, proving a related article opens as another VText surface.

local proof: `npm run build` passed; `PLAYWRIGHT_BASE_URL=http://127.0.0.1:4173 npm run e2e -- global-wire-app.spec.js --reporter=line` passed; `npm run e2e -- vtext-source-entities.spec.js:85 --reporter=line` passed.

staging proof: GitHub CI run `27109268849` passed, including integration smoke, Go vet/build, non-runtime tests, all runtime shards, frontend build, aggregate gate, and Node B deploy job `80003950785`. FlakeHub run `27109268838` succeeded. `https://choir.news/health`, Node B `/health`, and `/opt/go-choir` all reported deployed commit `50eeed58b47692f3f1e6518e2797b0d85ad10153` with deployed_at `2026-06-08T00:21:29Z`.

deployed product-path proof: a headless Playwright probe against `https://choir.news/` double-clicked the `Global Wire` desktop icon, opened the first article VText, observed real article prose, `5` native source refs, `1` related-VText ref, and no old `Source Manifest` or `My Edit` sections. Clicking the related-VText ref increased the VText editor count from `2` to `3` and opened the related article containing `Grid operators add reserve alerts as heat forecast shifts north` and `Forecast changes moved stress from the southern peak window toward northern reserve margins.`

partial or unproven claims: this proves signed-out/public product behavior and normal VText window launch semantics for related refs. It does not yet prove the authenticated owner Comet path because the Computer Use plugin still reports `Computer Use is not active` for Comet after a readable `get_app_state`. It also does not complete the larger newsroom architecture: live VText-agent-owned articles, processor/reconciler/researcher handoffs, authenticated living revisions, source expansion beyond the current staging corpus, and passage-range VText transclusion remain open.

remaining error field: the product floor now has a desktop Global Wire icon, clean article prose, native source refs, native related-VText refs, and click-to-open VText graph traversal. The mission remains incomplete until Global Wire produces and revises publication-quality articles through long-running shared-harness agents over a much broader multilingual source stream, with staging proof that VText agents own the article lifecycle rather than deterministic fallback content.
