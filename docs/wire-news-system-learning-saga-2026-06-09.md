# Wire News System Learning Saga

Date: 2026-06-09

Status: current learning record

This document records the news-system failures and architecture realization that
led to the current Wire ontology. It is not a mission plan. Its job is to keep
future agents from relearning the same mistake by treating news as a user app,
a dashboard, a source list, or a legacy graph object.

## The Short Version

The news system failed because we kept improving surfaces while the ownership
boundary was wrong.

We were building "Global Wire" as if it were a user-level app with a background
source pipeline. The clearer object is **Wire**: a reusable source-to-VText
substrate. The public news product is the Community Cloud's platform-level Wire
instance. Private Choir Clouds run the same substrate over private and public
sources. User computers run their own processors and reconcilers for
personalization, producing user-owned VTexts.

## Tick-Tock

### 1. Three Stories Looked Like Progress

The first Global Wire surface displayed three story-like items with seed source
labels. This gave a first visual object, but it was not a real newsroom:

- story count was tiny;
- source items were not full articles;
- frontend and backend contained hardcoded/seeded fallback behavior;
- articles were stubs or outlines, not publication-quality VTexts;
- source provenance appeared as metadata or bullet lists instead of native
  source transclusions.

The failure mode was plausible-looking product shape without product truth.

### 2. Source Quantity Became Obviously Load-Bearing

The expected system should ingest hundreds of items per short cycle, from many
source classes and languages. RSS, GDELT, Telegram API, Hacker News, science,
finance, industry media, regional outlets, long-tail social, and ignored sources
are all important.

The old source system had breadth in configuration but shallow depth in output:
feed summaries and seed source placeholders often masqueraded as source
artifacts. Telegram preview scraping was a legacy shortcut and is not acceptable
for the current target. Telegram should use proper API paths.

### 3. StoryGraph Was The Wrong Ontology

The legacy StoryGraph/source-manifest framing kept creating parallel structures:
source ledgers, source neighborhoods, graph nodes, style controls, source search
panels, and story cards. These surfaces competed with the native VText and
source-transclusion system.

The correction is hard deletion, not hiding:

- no StoryGraph product authority;
- no source-maxxing/source-maxx labels;
- no source chronology/search detritus as a fake source explorer;
- no style.vtext control panel in Wire;
- no hardcoded three-story fallback;
- no old source-network rename shim.

VTexts and source artifacts already create the implicit graph. Indexes may cache
that graph for performance, rendering, retrieval, and later radio traversal, but
the index is not the ontology.

### 4. Article Ownership Was Wrong

An article is a VText. It is not a backend story object, a processor brief, a
source manifest, or a dashboard card.

VText agents own article writing and revision. Processors, reconcilers,
researchers, supers, and coding agents may read, research, execute, and write
notes/evidence/messages, but they do not write canonical VText versions. They
message VText agents when article, report, memo, or edition versions should be
created or revised.

This boundary also fixes the bad "My Edit" surface. VTexts are natively
editable and versioned. User edits create ordinary user-owned VText versions or
forks; they are not a separate embedded section inside a platform article.

### 5. The UI Was Revealing The Ontology Bug

The busy Global Wire UI was not merely ugly. It exposed a confused model:

- cards and panels repeated the same small data;
- source chronology and search had no clear job;
- source viewer opened seed metadata instead of full source content;
- style controls appeared as product controls instead of source artifacts;
- article windows displayed manifests and metadata instead of prose;
- mobile toolbar wrapping exposed another layer of product roughness.

The correct Wire UI is a readable edition renderer: newspaper-like columns or a
single mobile column over an edition VText. Sources and related VTexts should be
native transclusions. The app renders the VText graph; it does not own it.

### 6. The Deeper Miss: Platform-Level vs User-Level

The most important realization was not tactical. It was architectural.

News is not a user-level feature inside one user's computer. The public Wire is
platform-level work inside the Choir Community Cloud. It runs under a Community
Cloud platform computer, produces public source artifacts, article VTexts,
edition VTexts, indexes, and publication records, and exposes those artifacts
for users and private clouds to consume.

Personalization belongs in userland. A user computer can run processors and
reconcilers over subscribed public and private corpora, then ask its own VText
agents to create user-owned editions, briefings, forks, and alerts.

Private Choir Clouds also have platform-level and user-level work. A law firm
or biotech company has its own NixOS host or host cluster, its own platform
computer(s), many user computers, candidate computers, private source systems,
private policy, and publication boundaries.

## Current Vocabulary Correction

Use **Wire** for the reusable substrate.

Use **Community Wire** when disambiguating the public Choir Community Cloud
instance.

Use **Private Wire** or a domain name such as Firm Wire, Matter Wire, Research
Wire, Science Wire, or Market Wire for private-cloud instances and editions.

Do not use "Global Wire" as the architecture name. If old code or docs still
say Global Wire, treat that as transitional or historical vocabulary to migrate.

## Current Architecture Shape

```text
Choir Community Cloud
  NixOS Host(s)
  Community Platform Computer(s)
    Wire
      public source artifacts
      platform processors/reconcilers/researchers
      public article/report VTexts
      public edition VTexts
      public indexes
  User Computers
    user processors/reconcilers
    personal editions, forks, alerts, style.vtexts
  Candidate Computers

Private Choir Cloud
  client-owned NixOS Host(s)
  Private Platform Computer(s)
    Wire
      private sources + subscribed public Wire artifacts
      firm processors/reconcilers/researchers
      firm/matter/team VTexts
      private indexes and egress policy
  User Computers
    role-specific personal processors/reconcilers
    user-owned editions, briefings, forks, alerts
  Candidate Computers
```

## Agent Foliation

The processor/reconciler split is not source vs story. It is direction of work.

```text
Processor:
  incoming or query-selected sources -> candidate understanding -> requests

Reconciler:
  existing VTexts/corpus/history -> coherence over time -> requests

Researcher:
  question -> evidence packet/source imports

VText agent:
  request/evidence/style -> authored VText version
```

This split exists at platform level and user level. A user-level processor can
query and research across accessible public/private corpora. It is not a
deterministic subscription filter. A user-level reconciler preserves coherence
across the user's editions, forks, interests, private docs, and watched topics.

## Durable Agent Notes

Choir already has Dolt-backed evidence/checkpoint surfaces such as
`agent_evidence`, run memory, and `submit_coagent_update`. The product concept
should be regularized as an agent notebook/checkpoint store scoped by cloud,
computer, agent, role, run, visibility, and evidence kind.

Do not use file writes as the default memory surface for processors,
reconcilers, or researchers. Do not give non-VText agents VText write authority
to compensate. Their durable notes live in their computer's Dolt-backed agent
state and become inputs to VText agents.

## What The Next Mission Must Preserve

- Wire is platform-level in the Community Cloud.
- Wire is reusable in Private Choir Clouds.
- Personalization is user-level.
- Only VText agents write VTexts.
- Processors and reconcilers exist at platform and user level.
- Source artifacts are real source objects with content/provenance where
  allowed, not headline labels.
- Related VTexts and sources are transcluded, not listed as metadata sludge.
- Indexes are caches.
- StoryGraph/source-maxxing/source-ledger detritus must be deleted from active
  product behavior.

