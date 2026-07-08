# Choir North Star

**Last updated:** 2026-06-09

The Automatic Computer already exists in deployed form: web desktop, backend
services, appagents, and NixOS-on-NixOS VM infrastructure. The product object is
a persistent user computer, not an ephemeral sandbox. The current task is to
stabilize the deployed system around versioned living documents, candidate
computer execution, publication, Wire, and later citation/compute economics.

Read [docs/current-architecture.md](../current-architecture.md) first. It is the
streamlined architecture memo for the current phase.

## Product Frame

Choir is a durable learning control system over versioned artifacts. The web
desktop is the current general-purpose projection of that substrate, not the
whole product ontology. The broader vector is:

```text
automatic computer -> automatic newspaper -> automatic radio -> automatic capital
```

The current concrete language for the newspaper/source layer is **Wire**: a
reusable source-to-VText substrate that runs in the Choir Community Cloud and in
Private Choir Clouds. Universal Wire produces public source-backed VTexts and
editions. Private Wire instances run over private sources and subscribed public
sources. User computers personalize by creating user-owned editions, forks,
briefings, and alerts.

Read [docs/mission-geometry.md](mission-geometry.md) for the high-level frame
and [docs/computer-ontology.md](../computer-ontology.md) for the computer and
promotion ontology.

The automatic computer is the private agentic workspace: a persistent computer
whose runtime, apps, package installs, Dolt state, source/build state, prompts,
files, and local preferences may diverge from the platform baseline. Some
desktop apps grow into appagents; most can remain plain display/control
surfaces. The first appagent is `vtext`: a durable, versioned semantic artifact
that accumulates user edits, appagent synthesis, worker findings, evidence,
artifacts, and later publication history. Its target shape is not flat text but
a multimedia computational essay: prose plus typed snippets for sources,
images, audio, podcasts, video, web captures, PDF/EPUB excerpts, interactive
graphics, animations, Trace evidence, candidate demo videos, and nested VTexts.
Those snippets should be readable in the VText flow and expandable into their
own desktop app windows without losing the reader's place.

The automatic newspaper is the public memory projection: selected VTexts,
sources, corrections, citations, and track records become discoverable,
citeable, disputable, forkable, and reusable. Wire is the near-term substrate
that makes this practical.

The automatic radio is the embodied traversal projection. It is not a pivot away
from `vtext`; it depends on `vtext`. Vtext is the score; radio is the
performance.

The dark factory behind the desktop contains researchers, supers, cosupers,
background/candidate computers, evidence, artifacts, tests, previews, and Trace. Its job is to
advance living artifacts and produce publishable/traversable state without
making raw agent orchestration the primary UI.

## Sequence

1. Stabilize `vtext`, researcher, super, user edits, and Trace.
2. Add ingestion skills for URLs, YouTube transcripts, text/Markdown/PDF/EPUB
   uploads, and later multimedia display apps whose content can be embedded as
   typed `vtext` snippets and expanded into owning app windows.
3. Add publication.
4. Add Pretext-based responsive rendering/transclusion for computational
   essays, source-rich publications, and campaign/candidate approval reports.
5. Add citation mechanics.
6. Add CHIPS and citation/compute economics.

Do not implement CHIPS, wallets, staking, token billing, or public citation
scoring yet. Do preserve document versions, provenance, evidence, artifacts,
citations/citation candidates, VM/model attribution, publication boundaries, and
compute accounting where available.

## Anti-Collapse Rules

- Chat history is not canonical state; `vtext` versions are.
- Worker updates are not document patches; `vtext` owns document synthesis.
- Mutable super/cosuper work belongs in candidate/background computer forks, not the live desktop.
- Platform Dolt is a ledger for platform-visible facts, not a hot-path message
  bus.
- Providers are adapters; no LLM or search provider is architecturally required.
- Personal computer promotion is not the same as platform deploy; do not force
  every user-local app/runtime change through global CI.
