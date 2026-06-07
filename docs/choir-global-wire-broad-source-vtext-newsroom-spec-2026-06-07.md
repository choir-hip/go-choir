# Choir Global Wire Broad-Source VText Newsroom Spec - 2026-06-07

**Status:** replacement product/architecture spec for Global Wire after the
newsroom object correction.
**Supersedes:** the earlier Global Wire spec that named the source-volume
target as a product name.

## Purpose

Global Wire is a broad-source AI newsroom inside Choir. It should ingest far
more source material than a human can read, preserve source provenance through
the existing source embedding/transclusion system, and publish real
publication-quality article VTexts.

The shared object is:

```text
broad source ingestion
-> durable SourceItems and source embeddings/transclusions
-> long-running processors
-> existing researchers for targeted evidence work
-> existing VText agents as article owners/writers
-> reconcilers over the VText/source corpus
-> VText-native article versions with transcluded sources and related VTexts
-> lightweight rebuildable indexes over VText/source graph state
-> clean newspaper collection surface
-> normal VText app for reading, editing, revising, forking, and publishing
```

The goal is not a special news object, article stub store, style demo, graph
dashboard, or list of related artifacts. The goal is normal VText publication
fed by a serious source system.

## Core Invariants

- Every article is a normal VText owned by the VText system. Do not introduce a
  separate article ownership class or renderer.
- A finished article is not the first version of a VText. If a processor brief,
  reconciler note, or researcher packet exists first, it is source material or
  an internal note, not the published article unless the VText agent has
  actually developed it into publication-quality work.
- VText is natively editable. Do not build a special "my edits" object inside
  Global Wire. User edits, revisions, forks, and publications use ordinary
  VText flows and user-owned VText versions.
- Platform corrections and updates are ordinary new VText versions through the
  normal candidate/review/version path. No source process, processor,
  reconciler, researcher, VText agent, app view, or user action may silently
  rewrite a platform article.
- Sources must be embedded/transcluded through the existing source system.
  News must not degrade sources into a flat list of labels when the source
  transclusion system can preserve source handles, excerpts, media, standing,
  and version-local provenance.
- Related VTexts should be transcluded where editorially useful, not shown as
  a bare related-links list. The index may discover relationships, but the
  article experience should use VText-native transclusion.
- `Style.vtext` is a citeable editorial source artifact. VText agents select,
  compose, mix, replace, or fork style sources intelligently for the article
  and audience; Global Wire should not run every style across every story.
- News is non-oracle. Claims, uncertainty, contrary evidence, source standing,
  corrections, unanswered questions, contradictions, and change over time must
  remain inspectable through VText/source provenance.
- Processors, reconcilers, researchers, and VText agents use the shared agent
  harness. Add role profiles and tool policies; do not fork the core agent
  loop unless a documented invariant requires it.
- All app views must work in Futuristic Noir, Carbon Fiber Kintsugi, and London
  Salmon, with themes controlled by the OS/desktop theme system, not an
  app-local theme selector.

## Source Breadth

The source target is operational, not a brand.

Early shipped behavior should move beyond toy source counts:

- hundreds of SourceItems per 15 minutes as the first serious target;
- clear provider/source counts in proof, not vague "many sources" language;
- GDELT or equivalent global event feeds;
- many RSS/Atom feeds across regions, languages, and beats;
- many Telegram feeds/channels where allowed and policy-compliant;
- search/provider sources for gap filling;
- curated high-standing feeds for official, local, domain, and specialist
  evidence;
- cadence, backoff, failure, latency, freshness, and dedupe metrics per source
  or provider.

Deterministic infrastructure owns identity, source fetch, dedupe, provider
health, rate limits, source standing metadata, and durable SourceItem storage.
It does not own final article voice.

## SourceItem And Transclusion

`SourceItem` is normalized evidence from source services, web/search providers,
uploads, imported external material, or user-published sources.

Minimum fields:

- stable id;
- source id, source class, provider, and policy standing;
- canonical URL or content ref;
- title, author/publisher/channel, timestamp;
- raw snapshot pointer;
- cleaned text/content pointer;
- source embedding/transclusion handle;
- multimedia handles when present;
- content hash and dedupe/echo metadata;
- fetch-run/provenance metadata;
- language, geography, and topic hints where available.

Duplicate or echoing items may matter as attention or source-standing evidence.
The system may dedupe display without erasing evidence that many sources are
echoing, contradicting, or amplifying the same claim.

## Processors

Processors are long-running shared-harness agents that receive source flow and
maintain live understanding. They are not fixed vertical agents. A processor
may cover a topic area, geography, source class, event family, load-balanced
firehose slice, or temporary developing situation.

Processors should keep hot context/KV cache across turns where the runtime
permits. When they compact, the compacted state should preserve source handles,
full-content paths, active developments, unresolved questions, watch items,
research requests/results, VText requests/results, and important prior
judgments. Compaction is continuity state, not a replacement for source
provenance.

Processors may request:

- existing researcher agents for bounded evidence work;
- existing VText agents when a real article should be created or revised;
- source fetch/search expansion when a claim is under-sourced;
- reconciler attention when a development touches the wider corpus.

Processors do not own canonical articles.

## Researchers

Researchers are existing Choir evidence agents. They answer bounded questions,
inspect source standing, gather missing context, compare contradictions, and
return source-backed evidence packets. They do not own article voice or
canonical article mutation.

## VText Agents

VText agents are the article owners and writers. They receive processor briefs,
reconciler notes, researcher evidence packets, source transclusion handles,
related VText handles, matched `Style.vtext` sources, and publication context.

They produce or revise normal VTexts with publication-quality article text,
embedded/transcluded sources, transcluded related VTexts where useful, and
version-local source/style provenance.

Quality bar:

- articles should have real reporting structure, not stub headings;
- source material should be meaningfully incorporated and citeable;
- related VTexts and sources should appear through VText transclusion, not
  lists bolted onto a news card;
- style should fit the story and publication need;
- uncertainty and corrections should improve the article rather than be treated
  as embarrassing defects.

## Reconcilers

Reconcilers are shared-harness corpus agents. They are not simply the next
stage after processors. They work over the article/source corpus, including:

- current platform article VTexts;
- existing published VTexts;
- authorized user-owned or user-published VTexts;
- source ledger state;
- processor briefs and compactions;
- researcher evidence packets;
- VText transclusion/index state;
- change history, contradictions, questions, and corrections.

Reconcilers look for consensus, contradictions, duplicated developments,
claim drift, missing evidence, stale articles, new questions, and ideas that
deserve new VTexts. They may request researchers or VText agents. Their output
is notes, questions, article-update requests, or new-article prompts, not
direct mutation of articles.

## VText Graph Index

The graph is implicit in VText markup, source transclusion, related VText
transclusion, versions, citations, publication records, and source handles.
Indexes are rebuildable accelerators over that state.

The index should eventually key:

- VText id and version id;
- per-version source transclusions and source embeddings;
- VText-to-VText transclusions;
- publication, owner, and visibility state;
- correction/update lineage;
- processor/reconciler/research note handles;
- recency, source density, contradiction, open-question, and follow-up signals.

The index is for navigation, discovery, performance, and later voice/radio
traversal. It is not the authority for provenance or article ownership.

## News App Surface

Global Wire is a collection surface for article VTexts, not the article editor.

Required shape:

- clean newspaper columns, not dashboard panels or cards;
- no nested scrolling inside panels;
- no border-line grid; text, spacing, and typography provide structure;
- source chronology column with source breadth/provenance signals;
- every article has a quiet VText affordance to open in the VText app;
- no repeated visible "Open in VText" label text;
- no app-local theme selector;
- no special contribution panel; editing/forking/publishing happens through
  VText;
- mobile remains inside the Choir desktop/web shell, not a native-app fantasy;
- mobile VText menus must not overlap buttons or labels.

Future surfaces may include user-published VText discovery, VText graph
exploration, and audio traversal, but none of those should distract from broad
source ingestion and publication-quality article VTexts.

## Delivery Gates

Do not claim delivery until staging proof shows:

- broad source ingestion substantially beyond the current toy count;
- processors and reconcilers running as shared-harness profiles;
- existing researcher reuse;
- existing VText agent ownership of real article creation/revision;
- article VTexts with source and related-VText transclusions;
- intelligent `Style.vtext` use;
- user edits through normal VText behavior, not a Global Wire edit subsystem;
- clean newspaper UI across the three themes and desktop/mobile sizes;
- staging commit identity, CI/deploy status, and product-path acceptance
  evidence.
