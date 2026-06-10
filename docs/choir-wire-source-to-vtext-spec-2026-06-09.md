# Choir Wire Source-To-VText Spec

Date: 2026-06-09 (amended same day: explicit activation model, invariant 21)

Status: current requirements contract for Wire/news work

This spec supersedes the legacy Global Wire, StoryGraph, source-manifest, and
source-maxxing ontology for active work.

## Core Object

Wire is the reusable source-to-VText substrate.

It runs in the Choir Community Cloud and in Private Choir Clouds. It turns
source artifacts into VText-owned articles, reports, memos, briefings, and
edition VTexts through processors, reconcilers, researchers, and VText agents.

```text
source artifacts
  -> processor/reconciler/researcher notes and evidence
  -> VText-agent-authored Article/Report/Memo.vtext
  -> Edition.vtext
  -> app/radio/search renderers over VTexts and source artifacts
```

The app is a renderer and control surface over Wire VTexts. It is not the source
of truth for stories, sources, styles, relationships, ranking, or publication.

## Cloud And Computer Ownership

Wire is always scoped to a cloud and computer.

```text
Choir Community Cloud
  NixOS Host(s)
  Community Platform Computer(s)
    Community Wire
      public source artifacts
      platform processors/reconcilers/researchers
      public VText agents
      public Article/Report.vtexts
      public Edition.vtexts
      public indexes and publication records
  User Computers
    user processors/reconcilers
    personal editions, forks, alerts, notes, style.vtexts
  Candidate Computers

Private Choir Cloud
  client-owned NixOS Host(s)
  Private Platform Computer(s)
    Private Wire
      private sources + subscribed public sources
      firm processors/reconcilers/researchers
      firm/matter/team VTexts
      private indexes and egress policy
  User Computers
    role-specific processors/reconcilers
    user-owned editions, briefings, forks, alerts
  Candidate Computers
```

The public news product is Community Wire. It is platform-level work inside the
Choir Community Cloud, not a user-level feature inside one user's computer.

Private Choir Clouds reuse the same substrate on their own NixOS host or host
cluster with their own platform computers, user computers, candidate computers,
private sources, model/search policy, and publication/subscription boundaries.

## Invariants

1. Wire is reusable public/private infrastructure, not a bespoke news dashboard.
2. Community Wire is owned by a Choir Community Cloud platform computer.
3. Private Wire instances are owned by Private Choir Cloud platform computers.
4. Personalization runs in user computers and creates user-owned VTexts,
   forks, alerts, notes, preferences, and style.vtexts.
5. Articles, reports, memos, briefings, and editions are normal editable VTexts.
6. Only VText agents write VText versions.
7. Processors, reconcilers, researchers, supers, and coding agents may read,
   query, research, execute, write durable notes/evidence/messages, and request
   VText work. They must not write canonical VText versions.
8. External sources are durable source artifacts/ContentItems with media,
   extracted text, metadata, provenance, hashes, timestamps, selectors, and
   rights/robots/auth notes when available. They are not forced to be VTexts.
9. Sources are transcluded into VTexts through native source systems, not listed
   as source manifests.
10. Related VTexts are transcluded where editorially useful, not listed as graph
    edges or metadata.
11. Version provenance lives in VText versions. Source citations are
    per-version, not per-document.
12. Transclusions record version semantics: pinned revision, live head, or
    live-with-review.
13. Public and private source artifacts must never be casually co-mingled.
    Visibility and egress policy are source-artifact and VText-version
    properties.
14. Indexes are caches over VTexts and source artifacts. They are not ontology.
15. News is non-oracle and provenance-rich: uncertainty, contradiction,
    concurrence, update pressure, and correction history are first-class.
16. No hardcoded source trust tiers. Track source behavior over time and let
    agents reason softly about provenance, consistency, and observed record.
17. Delete legacy StoryGraph and source-maxxing/source-maxx ontology from active
    product behavior, APIs, runtime/store types, tests, active docs, and
    user-visible copy. Do not hide, rename, quarantine, or preserve it behind
    compatibility shims.
18. Delete fake source ledgers, source chronology/search detritus, style.vtext
    controls, seed source neighborhoods, durable-storygraph labels, hardcoded
    three-story fallbacks, and Global-Wire-as-ontology behavior.
19. Style.vtexts are VText/source artifacts for writing. They are not Wire UI
    controls. Examples matter more than rule bullets.
20. Wire app views must work in Future Noir, Carbon Kintsugi, and London Salmon
    through the shared OS-wide theme system.
21. Wire story work is activated by source ingestion, never by human input
    surfaces. Source arrival dispatches processor runs; processors request
    researchers and VText agents; editions update through the approval path.
    The prompt bar, Command prompt, and any other human input surface never
    creates, triggers, or supplies a Wire story. Their only roles near Wire
    are editorial supervision (approve, correct, retract, annotate) and
    explicitly marked debugging harnesses excluded from the product path.

## Source System

Source quantity and depth are load-bearing.

Community Wire should ingest copious, multilingual, mixed-authority public
sources faster than the current 15-minute baseline where source APIs allow it.
The target shape is hundreds of items per cycle, with growth toward
live/as-they-arrive ingestion.

Required Community Wire source classes:

- RSS/Atom feeds with full article/readability import where allowed;
- GDELT global feeds/APIs as broad multilingual discovery and event/signal
  firehose;
- Telegram channels through proper Telegram API paths only; public preview HTML
  scraping is not an accepted fallback;
- Hacker News and broader tech/open-source sources;
- science, finance, industry, logistics, health, energy, security, climate,
  politics, culture, regional, and long-tail sources;
- non-English sources and sources ignored by established outlets.

Private Wire source classes may include:

- private documents, PDFs, DOCX, slide decks, EPUBs, datasets, images, audio,
  video, transcripts, emails, and internal databases;
- private RSS/Atom or monitored websites;
- subscribed Community Wire VTexts/source artifacts;
- public web/source APIs allowed by the private cloud's policy.

Public source artifacts and private source artifacts use the same mechanics but
different ownership, visibility, retention, and egress policy.

Useful source fields:

```text
source_artifact_id
owning_cloud_id
owning_computer_id
visibility: public | private_cloud | user_private | restricted
egress_policy: publishable | needs_redaction | never_publish
source_class
source_id
canonical_url
media_type
language
published_at
fetched_at
hashes
selectors
body/extract refs
rights/robots/auth notes
provenance refs
```

## Agent Foliation

The shared agent harness remains uniform. Role prompts and capability policy
shape behavior; the core loop should not fork by role unless a
correctness/security/resource invariant requires it.

### Activation

Activation is explicit. Nothing in this section is initiated by the prompt bar.

**Architecture checkpoint (2026-06-10):** full activation matrix, workstream
order, and negative proofs are in
[universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md).
This section summarizes; on conflict, the checkpoint doc wins until code lands.

```text
sourcecycled fetch
  -> source artifact persisted with fetch provenance
  -> ingestion event (artifact id, source id, fetch provenance,
     content hash, dedupe key)
  -> platform processor run dispatched on the event only
  -> processor spawns VText agent only (watch-lists for low-signal items)
  -> VText autoregressive loop:
       edit_vtext revisions; spawn researcher on doc channel;
       request_super_execution when needed; worker deliveries wake next step
  -> autonomous publish (Community Cloud / Universal Wire — no operator gate)
  -> reconciler on debounced post-publish batch, scheduled sweep,
     corpus-change (user edit/fork on published platform docs) — never per cycle
  -> reconciler emits VText wake requests on doc_id (including edition
     universal-wire/Wire.vtext) — never edit_vtext
```

The ingestion event is the only entry point for story creation. Human input
surfaces dispatch editorial-supervision work only — not ingestion or processor
runs. Scheduled reconciliation and corpus-change signals are lawful reconciler
causes; reconciler never holds the VText pen.

**Publication:** Universal Wire (Community Cloud) publishes autonomously.
Per-deployment policy may gate Private Wire instances. Procedural guards
(article-before-edition inclusion, fidelity checks) are acceptance criteria,
not anthropomorphic “approval.”

The processor/reconciler split is direction of motion:

```text
Processor:
  incoming or query-selected source material
  -> candidate understanding
  -> source handles, watch items, research requests, VText requests

Reconciler:
  existing VTexts/source artifacts/notes/history
  -> coherence over time
  -> contradictions, stale claims, related VTexts, update requests

Researcher:
  question
  -> evidence packet/source imports

VText agent:
  request/evidence/style
  -> authored VText version
```

### Processors

Processors exist at platform and user levels.

Platform processors ingest public or private platform source batches and
maintain live understanding across assigned load slices. A slice may be topical,
regional, source-type-based, event-based, or load-balanced; it is not
necessarily a vertical.

User processors personalize by querying, filtering, and researching across the
user's accessible corpus: Community Wire subscriptions, Private Wire artifacts,
user files, user VTexts, and user preferences. They are not deterministic
subscription matchers.

Processors:

- preserve source artifact refs and source handles;
- notice changed beliefs, novelty, overlap, contradictions, and emerging
  questions;
- **spawn VText agents only** when an article, report, briefing, edition, fork,
  or alert should be opened or revised (VText may then spawn researchers);
- **must not spawn researcher or super** directly;
- write durable notes/evidence and watch-lists in checkpoint store for
  low-signal items;
- do not write canonical VText versions.

### Reconcilers

Reconcilers exist at platform and user levels.

Platform reconcilers work over the platform corpus: source artifacts,
processor notes, researcher packets, public or private platform VTexts,
edition VTexts, transclusions, indexes, and publication state.

User reconcilers work over user-owned VTexts, forks, editions, alerts, private
context, preferences, subscriptions, and accessible public/private source
artifacts. They preserve coherence and relevance over time for one user.

Reconcilers identify concurrence, contradiction, duplicate or overlapping
developments, stale claims, version drift, missing context, emergent questions,
and update/correction opportunities.

Reconcilers may read files, VTexts, source artifacts, indexes, and evidence
packets. They write durable notes/evidence/messages. They must not write
VTexts or call `edit_vtext`. They emit **VText wake requests** when VTexts
should be created, revised, corrected, reordered, or updated — including the
edition doc (`universal-wire/Wire.vtext` on Community Cloud).

Lawful reconciler triggers: **debounced post-publish batch** (N or T),
**scheduled sweep**, **corpus-change** from user edit/fork on published
platform docs. Forbidden: per-ingestion-cycle dispatch, processor-submit
dispatch, in-flight draft dispatch.

### Researchers

Researchers provide bounded evidence packets and source imports. They should
have strong tools for web search, source search, URL import, PDF/DOCX/EPUB/PPTX
/HTML extraction, selectors, and content-item recall.

Researchers write evidence and findings. They do not own canonical article,
memo, report, briefing, or edition prose.

### VText Agents

VText agents own writing and revision.

They receive source-backed briefs from processors or reconcilers, use relevant
style.vtexts, request researchers/super when needed, and create publishable
VText versions with native citations/transclusions.

A VText version must be prose or a deliberately designed edition/control
artifact. It must not be a processor brief, outline, status note, source
manifest, or scaffold pretending to be an article.

## Edition VTexts

An edition VText is a curated package of VText transclusions plus optional
editorial framing.

Examples:

- `Wire.vtext`: Universal Wire public front page (`universal-wire/Wire.vtext`
  on Community Cloud after migration);
- `Wire-Tech.vtext`: Community Wire tech/open-source edition;
- `Wire-Science.vtext`: Community Wire science edition;
- `FirmMorningBrief.vtext`: Private Cloud firm-wide morning edition;
- `MatterRiskWire.vtext`: matter-specific private edition;
- `YusefMorningWire.vtext`: user-owned personalized edition;
- `CEOBriefing.vtext`: user-owned executive edition inside a Private Cloud.

Edition VTexts should transclude article/report/memo VTexts rather than
duplicating full text unnecessarily.

## Personalization

Personalization is user-level authorship, not platform-level mutation.

Users can own:

- personal edition VTexts;
- personal style.vtexts;
- article/report/memo forks and edits;
- topic/source/language/region preferences;
- alert thresholds;
- reading/writing style preferences;
- user-level processor/reconciler notes and evidence.

The platform corpus remains platform-owned. User agents may assemble a
different package of stories, request different summaries or framing, use user
style.vtexts, and fork/edit articles, but platform article/report versions
remain platform-owned unless a platform workflow revises them.

## Transclusion Version Semantics

Every VText transclusion must know what version relationship it represents.

```text
target_doc_id
selected_revision_id
mode: pinned | live | live_with_review
observed_current_revision_id
accepted_at
accepted_by
update_note
```

Default behavior:

- source transclusions: pinned by source artifact/version where possible;
- historical editions: pinned by VText revision;
- live editions: live-with-review or pinned-with-visible-newer-version marker;
- breaking editions: may use live transclusions if the UI visibly marks
  revision changes.

The renderer should show safe update awareness:

- `v10 transcluded`;
- `v12 current`;
- `2 newer versions available`;
- `latest accepted by reconciler`;
- `latest changes main claim`;
- `edition not yet updated`.

No VText or edition should silently change historical meaning without
version-visible provenance.

## Indexes As Caches

Indexes exist for performance, routing, retrieval, search, personalization, and
later radio readiness. They are rebuildable by walking VTexts and source
artifacts.

Useful caches:

```text
vtext_index:
  cloud_id, computer_id, doc_id, current_revision_id, title, dek,
  kind, topics, regions, language, source_count, updated_at,
  public/private, prominence notes

edition_index:
  cloud_id, computer_id, edition_doc_id, owner_id, visibility,
  current_revision_id, included VText refs, update policy

transclusion_index:
  parent_doc_id, parent_revision_id, target_kind, target_doc_id,
  target_revision_id, mode

source_index:
  source artifact ids, URLs, canonical URLs, source ids, language,
  media type, timestamps, hashes, selectors, body kind, visibility,
  egress policy, source behavior notes

agent_notebook_index:
  cloud_id, computer_id, agent_id, role, run_id, note kind,
  evidence refs, source refs, VText refs, created_at
```

A cache mismatch is an index bug, not product truth.

## Wire App

The Wire app should be a clean renderer over an edition VText graph.

The app should:

- render readable article columns/text, not cards or dashboard panels;
- avoid nested scroll panels;
- avoid visible source ledgers, dashboard detritus, style controls, and
  metadata sludge;
- make every article openable in VText without labeling every button as "Open
  in VText";
- expose source transclusions through native source viewer behavior;
- show update/version awareness without overwhelming the page;
- support desktop and mobile inside the Choir web desktop;
- use shared OS-wide theme, no local theme selector;
- use serif headlines and readable typographic rhythm;
- degrade honestly when no live VTexts exist.

## Ranking And Editorial Judgment

Do not build deterministic fake ranking just to have a ranking function.

Processors and reconcilers should reason intelligently about prominence,
importance, novelty, overlap, concurrence, contradiction, source breadth, source
depth, timeliness, downstream relevance, and user/private-cloud context.

Prominence should be explainable through notes/evidence and improve over time.
Do not hardcode trust tiers or source authority ladders. Do not claim oracle
truth. Preserve the plurality and surface structure.

## Out Of Scope For The Immediate News Mission

- newsletters/email delivery;
- Autoradio/TTS/STT;
- vector database rollout;
- deterministic clustering/embedding dependency;
- native mobile app;
- automatic capital/citation economics;
- hardcoded source trust tiers.

These remain important follow-on work, but the immediate product truth is live
source ingestion, VText-authored articles/reports, edition VText rendering,
userland personalization, and deletion of legacy graph/source-maxxing behavior.
