# Global Wire Mission and News System Review

**Date:** 2026-06-08  
**Repository:** `/Users/wiz/go-choir`  
**Mission reviewed:** `docs/mission-global-wire-living-vtext-newsroom-v1.md`  
**Current deployed behavior commit:** `7dc516014d501e067733a0afe76b6fba6a054aee`  
**Final mission checkpoint commit:** `cfefce66d617342d6e7c33b65a9f28aa8fa50f02`

## Executive Assessment

Global Wire crossed the important architecture boundary during this mission.
It is no longer just a deterministic demo surface with three fake-ish story
objects and visible metadata sludge. The deployed system now has a broad
source service, source-cycle health, shared-harness processor and reconciler
runs, VText-agent-owned article revisions, native source references, related
VText references, a Global Wire desktop icon, and a cleaner newspaper-style
collection surface that opens normal editable VTexts.

That is real progress. It is also not yet the finished newsroom.

The strongest evidence is that fresh live source cycles now produce article
VTexts from processors/reconcilers and VText agents, and recent articles on
staging contain canonical native source references such as
`[The Guardian](source:src_12ede5dccf6756d9)`. The system is processing a
211-source registry across GDELT, RSS/Atom, and Telegram public-preview
sources, with 15 language tags and 36 vertical tags. The public product path
opens Global Wire, opens article VTexts, renders native source refs, and
renders related VText refs without the earlier `Source Manifest`, `My Edit`,
or metadata-scaffold sections.

The remaining weaknesses are now higher-order newsroom problems rather than
the initial wrong-object failure: processor/reconciler failure handling,
duplicate story heads, article quality, source selection quality, learned
source track record, reconciliation over existing published VTexts, and
durable editorial update/correction loops.

## What Changed

### 1. The Source System Became Real Enough To Matter

The mission started with the user correctly identifying that a 14- or
16-source surface was not a news system. The source system was expanded to:

- `211` configured sources;
- `1` GDELT/global event stream;
- `137` RSS/Atom feeds;
- `73` Telegram public-preview channels;
- `15` language tags: `ar`, `be`, `de`, `en`, `es`, `fa`, `fr`, `ha`, `id`,
  `ja`, `multi`, `pt`, `ru`, `uk`, `zh`;
- `36` vertical tags, including conflict, technology, open source, science,
  finance, markets, semiconductors, logistics, energy, agriculture, climate,
  health, public safety, internet culture, and regional sentiment.

This matters because Global Wire now has a live source substrate rather than
a toy evidence ledger. It includes long-tail Telegram/social inputs, technical
community sources, science, finance, industry, official/public sources, and
regional feeds.

The implementation also removed static source tiers or hardcoded source
standing. That is the right direction. Source reputation should be learned
over time from corrections, corroboration, freshness, error history, and
researcher/model judgment, not frozen into config.

### 2. Source Health Became More Honest

The mission found and fixed an important observability bug: `304 not_modified`
cache hits were being counted as source failures. That made the owner-facing
health surface falsely pessimistic. The source-health classifier now treats
`ok` and `not_modified` as successful fetch outcomes, while actual HTTP
errors, parser errors, timeouts, and other error statuses remain failures.

The system also hardened RSS parsing against illegal XML control bytes after
`rss:euronews_fr` exposed a parser failure. That kind of fix matters because
large source ingestion spends a lot of time dealing with messy feeds, not
idealized XML.

### 3. Global Wire Became A Desktop App Surface

Global Wire now appears as a normal Choir desktop icon. The collection surface
shows live source-service status instead of only seeded story-neighborhood
counts. The UI moved away from the earlier dashboard/card/panel failure and
toward a newspaper-like surface with compact VText affordances.

The most important UI correction was conceptual: Global Wire is not the
article editor. It is the collection view. Reading, editing, versioning,
source traversal, and related VText graph walking happen in the normal VText
app.

### 4. Article Ownership Moved To VText Agents

This was the central architecture correction.

Earlier output treated articles as generated Global Wire projections or
seeded stubs. The mission corrected the lifecycle:

```text
source cycle
-> processor/reconciler shared-harness runs
-> researcher requests when needed
-> VText agent receives source handles, brief, style context, and intent
-> VText agent writes/revises the normal article VText
-> Global Wire indexes and displays the article
-> VText app owns full reading/editing/source traversal
```

The system now distinguishes a non-article source brief from the first real
article revision. The seed handoff remains `artifact_kind=source_brief` and
`article_version=false`; the VText agent's `edit_vtext` article head becomes
`artifact_kind=article_revision` and `article_version=true`.

That distinction is essential. It prevents a prompt or source brief from
masquerading as the published article object.

### 5. Native Source And Related VText Transclusion Advanced

The mission removed the worst article-body anti-patterns:

- no visible `Source Manifest`;
- no visible `My Edit`;
- no raw related-VText list;
- no visible source-handle inventory as the article;
- no platform-specific edit subsystem inside the article body.

Article VTexts now support native inline source references and related VText
references. Related VText refs can open target VTexts in normal VText windows.
The collection surface indexes source neighborhoods from article-visible
native source refs instead of blindly expanding full-cycle source IDs into
hundreds of context rows.

The final source-ref recovery was important. A fresh live article wrote bare
tokens such as `[source:src_718e2f432842de16]` instead of canonical
`[label](source:ENTITY_ID)` refs. The deployed fix normalizes known bare
source tokens at Global Wire VText commit time, and the renderer/index recover
older bare-token article heads.

## Deployed Evidence

The final behavior commit is:

```text
7dc516014d501e067733a0afe76b6fba6a054aee
```

Evidence recorded during the mission:

- CI run `27111583136` passed.
- Deploy job `80010567810` passed.
- FlakeHub run `27111583134` passed.
- `https://choir.news/health`, Node B `/health`, and Node B
  `/opt/go-choir` all reported deployed commit
  `7dc516014d501e067733a0afe76b6fba6a054aee`.
- Deployed time: `2026-06-08T01:52:25Z`.
- Browser proof opened `https://choir.news/`, opened the Global Wire desktop
  icon, observed three compact article VText affordances, opened a normal
  VText article, and observed five native source refs plus one related VText
  ref.
- The browser proof saw no `Source Manifest`, `My Edit`, `Style.vtext Source`,
  or `Source Handles` scaffold strings, no console errors, and no horizontal
  overflow.
- Authenticated API proof returned `15` stories from
  `durable-storygraph+source-network-vtexts`.
- A post-deploy article,
  `UK, France and Germany back Zelenskyy's call for direct ceasefire talks
  with Russia as Putin rejects meeting`, contained two canonical native source
  refs and zero bare source tokens.

Later live staging checks after the checkpoint showed the system continuing
to generate fresh article VTexts with canonical source refs, including
conflict, earthquake, counterintelligence, and politics stories with
article-local lead/context source projections.

## What Is Actually Done

The following are now fair claims:

1. Global Wire has a broad live source substrate, not only a toy deterministic
   signal.
2. GDELT, RSS/Atom, and Telegram public-preview sources are in the configured
   registry.
3. Long-tail Telegram/social perspectives are part of the source surface.
4. Static source tiers/standing were not encoded into the registry.
5. Source health distinguishes cache hits from true failures.
6. Processors and reconcilers run through the shared harness rather than a
   separate bespoke core loop.
7. VText agents can own article revisions.
8. Researchers can participate in article revision sequences.
9. Global Wire can index live VText-owned articles.
10. Article source neighborhoods are no longer allowed to expand the whole
    source cycle as visible support for one article.
11. Native source refs render in VText.
12. Related VText refs render and can open normal VText windows.
13. Global Wire is available as a desktop icon.
14. The public product path has basic staging proof.
15. The mission doc has durable checkpoint/resumption state.

## What Is Not Done

The following should not be claimed yet:

1. Global Wire is not yet a publication-quality newsroom.
2. Processor/reconciler failures are not yet handled well enough.
3. Source reputation is not learned over time yet.
4. Duplicate story heads are not reconciled well enough.
5. Reconciler behavior over existing published article VTexts is still thin.
6. Article quality is inconsistent.
7. Some article bodies still leak dateline/scaffold/style-rationale patterns.
8. Some older article bodies still store bare source tokens, even though the
   renderer and index recover them.
9. Related VText transclusion is still a compact inline ref/snapshot, not a
   mature passage-range transclusion system.
10. Authenticated Comet proof was unreliable during parts of the run because
    Computer Use intermittently reported inactive.
11. The Global Wire source registry is broad, but not yet deep enough for the
    intended international long-tail system.
12. Autoradio is not built and should remain out of scope until VText graph
    indexing, TTS, STT, and narrative path selection are explored.

## System Architecture Review

### The Good Shape

The architecture is now roughly pointed at the right object:

```text
many live sources
-> SourceItems with handles
-> processors watch source flow
-> researchers fill evidence gaps
-> VText agents own article versions
-> reconcilers inspect article/corpus tensions
-> normal VText revisions carry sources, versions, and transclusions
-> Global Wire indexes VTexts into a readable collection
-> VText app handles reading/editing/source traversal
```

This is the right topology. It respects the central VText invariant: a story
is a VText. It also avoids building a parallel "news article" database that
would fight the VText system.

### The Weak Shape

The weak part is the control loop after ingestion.

The source service can now create many SourceItems, but processor/reconciler
runs can fail, duplicate, or generate uneven article heads. Reconciliation is
not yet strong enough to decide when an incoming source batch should:

- revise an existing article;
- create a new article;
- request researcher evidence;
- create a correction;
- downgrade a claim;
- merge two story heads;
- split one story into separate threads;
- keep a source as sentiment/weak-signal context only.

The system has the right components, but the newsroom judgment loop is still
immature.

### The Main Architectural Risk

The largest risk is that source volume creates visible news output faster than
the reconcilers can maintain article identity and quality. Without stronger
reconciliation, Global Wire can drift into a pile of plausible VTexts rather
than a coherent living newspaper.

The next architecture pass should focus less on adding panels or UI affordances
and more on story identity, update routing, correction routing, and article
quality gates.

## Article Quality Review

The mission moved from outline/stub failure to real prose. That is a major
improvement. But the article bar should remain high.

Good article VTexts should have:

- a real headline;
- a tight dek;
- coherent article prose;
- native source refs embedded where claims are made;
- source uncertainty expressed in prose when necessary;
- related VTexts transcluded where they materially improve the article;
- no internal tool labels;
- no source inventory sections masquerading as article content;
- no `Style.vtext` rationale in reader-facing copy unless it is editorially
  intentional;
- normal VText version history for updates and corrections.

The current system sometimes achieves pieces of that. It does not achieve it
consistently enough.

The fallback deterministic article floor is useful, but it should remain a
floor. It should not become the hidden product. The desired product is
VText-agent-authored article revisions informed by processors, reconcilers,
researchers, Style.vtext sources, and native source transclusion.

## Source System Review

The source expansion was directionally correct:

- broader media types;
- long-tail social/Telegram;
- technical community inputs;
- science and finance;
- industry and logistics;
- multilingual/regional coverage;
- no static trust tiers.

The next problem is not simply "more sources". It is source operations:

- per-source freshness;
- per-source parse quality;
- per-source correction history;
- per-source corroboration behavior;
- per-source language/region/beat coverage;
- per-source failure history;
- learned track record over time;
- soft model judgment about known source reputation;
- clear distinction between evidence, sentiment, rumor, and weak signal.

Long-tail Telegram and social feeds are valuable precisely because they are
not just high-trust authorities. They provide local perspective, weak signals,
sentiment, rumor surfaces, and neglected angles. But they should usually
trigger corroboration and uncertainty, not stand alone as article support.

## UI Review

The UI improved materially from the original busy dashboard.

What is better:

- Global Wire has a desktop icon.
- The app is a collection surface.
- Articles open in VText.
- Compact VText affordances replaced repeated "Open in VText" copy.
- Native source and related VText refs render in article surfaces.
- The old visible metadata/edit/source-manifest scaffold is gone from the
  proven public path.

What still needs work:

- Typography still needs a serious editorial pass at normal desktop widths.
- Mobile is still a Choir web desktop app, not a native app, and must be
  checked in that shell.
- The source column should remain useful but quiet.
- The UI should avoid exposing internal source-cycle or style rationale copy
  unless it is intentionally owner-facing.
- Theme consistency across Futuristic Noir, Carbon Fiber Kintsugi, and London
  Salmon needs a dedicated visual proof pass after the article/reconciler loop
  stabilizes.

## Reliability Review

The final checkpoint documented a concerning latest-cycle status:

```text
cycle_9b6ac02be672bd397a398ded
211 fetches
477 items
3 completed processors
4 failed processors
1 failed reconciler
```

That is the next serious system problem. The source substrate and article
ownership path are now real enough that agent failure handling matters.

The next mission should document the exact processor/reconciler failures before
fixing them, then root-cause whether they are:

- provider/model failures;
- search/gateway failures;
- prompt/tool-contract failures;
- timeout/resource failures;
- source-batch overload;
- bad source handles;
- VText tool call failures;
- researcher handoff failures.

Do not paper over these failures with retries alone. Retries may be useful, but
the system needs observable failure categories and resumption behavior.

## Recommended Next Mission

The next mission should be:

```text
/goal Run docs/mission-global-wire-living-vtext-newsroom-v1.md and harden live newsroom reconciliation: processor/reconciler failure root-cause, article identity, VText update routing, and publication-quality article revisions on staging.
```

Primary objectives:

1. Document the latest processor/reconciler failures before fixing code.
2. Root-cause failure classes using trace/run records, gateway/search evidence,
   and VText tool-call evidence.
3. Add durable failure taxonomy and resumption behavior for source-cycle agent
   runs.
4. Improve reconciler routing over existing article VTexts:
   update, correct, merge, split, request research, or create new article.
5. Reduce duplicate story heads.
6. Strengthen article quality gates without making VText agents subservient to
   deterministic validators.
7. Prove a living update sequence:
   source event -> processor/reconciler -> researcher if needed -> existing
   article VText revised -> Global Wire collection updates -> prior version
   remains available.
8. Run staging browser proof in the Choir desktop shell across the three
   themes after the behavior is stable.

## Bottom Line

This mission delivered a real product slice. It did not deliver the final
newsroom.

The most important correction was conceptual: Global Wire is not a dashboard
and not an article generator. It is a collection surface over living VTexts.
The articles are VTexts owned by VText agents, sourced through native
transclusion, revised over time, and reconciled against a broad live source
world.

That architecture is now present enough to build on. The next step is to make
it reliable and editorially good.
