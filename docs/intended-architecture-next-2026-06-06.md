# Intended Architecture Next - 2026-06-06

**Status:** target architecture, not current-state proof  
**Scope:** intended architecture after the next week-plus of work is written,
deployed, and proven stable.  
**Current-state source of truth:** [current-architecture.md](current-architecture.md)

## Purpose

This document holds the forward architecture so
[current-architecture.md](current-architecture.md) can stay honest about the
repo and staging system that exist today.

Do not cite this document as proof that a behavior exists. A behavior graduates
from this document into current architecture only after code, tests, deploy, and
product-path evidence show that it works.

## Target Shape

Choir should stabilize around this stack:

```text
automatic computer
  -> durable VText/source/news substrate
  -> Choir Base file/sync/share substrate
  -> native desktop control surface
  -> Automatic Radio voice/listening surface
  -> mobile/watch screenless surfaces
```

The server/cloud version remains the authoritative multi-tenant shape. Local
desktop and single-tenant deployments are scaled-down or differently placed
versions of the same architecture, not separate products.

## Source And News

Target:

- Source Service has clear ownership of external source ingestion, fetch
  records, raw snapshots, cleaned items, source health, search, and item
  resolution.
- VText source entities are the document-level representation for citations,
  transclusions, source spans, media timestamps, and other VTexts.
- News becomes a first-class app/newspaper surface over source items, findings,
  clusters, and VText issues/briefs. Its story model should be
  multiperspectival: claims, counterclaims, source positions, evidence gaps,
  confidence changes, corrections, and time history should remain visible
  instead of being flattened into an oracle-style summary. Source provenance
  should use weighted manifests: a readable article may show a few lead
  citations inline, but all material sources/context packets that shaped the
  story remain inspectable by role, weight, recency, source class, and selection
  rationale.
- Research retrieval should treat Source Service as the basis and live web
  search as the expansion/check path. A relevant source-ledger hit should
  usually trigger live external lookup to find missing coverage, check
  freshness, and broaden the claim range, while preserving which claims came
  from the owned ledger versus live search.
- Email/newsletter projection uses the same VText/source artifact path rather
  than a parallel newsletter system.

Graduation evidence:

- staging source search/import proof;
- VText source entity rendering and revision metadata proof;
- news/front-page app proof with real source items;
- citation/export/publication proof for at least one source-backed document.

## VText Style

Target:

- `Style.vtext` artifacts are first-class VTexts, not static prompt snippets. They contain
  human-readable guidance, machine-readable style profiles, corpus manifests,
  exemplars, review notes, anti-model-tic observations, and open questions.
- A person, team, client, or publication may own multiple contextual styles.
  Style is treated as authored IP: versioned, citeable, permissioned,
  composable, and eventually publishable/distributable.
- Client style learning starts from corpus ingestion and VText edit history:
  accepted edits, rejected drafts, explicit notes, positive voice signals, and
  corrective feedback become reviewable candidate style observations that
  propose `Style.vtext` revisions rather than silently mutating an invisible
  profile.
- Style support has two sides: preserve the client's distinctive contextual
  voice, and prevent generic assistant/model tics from laundering that voice
  into average AI-polished prose.
- Multiple scoped `Style.vtext` artifacts can apply to one person or client by document
  type, audience, channel, matter/project, and privacy boundary.
- Generation uses a compact style packet: measured style fingerprint,
  relevant style observations, model-tic warnings, voice-preservation intent,
  and a small set of context-matched exemplars.
- Fine-tuning/adapters are optional later deployment paths after corpus quality,
  privacy policy, and style-eval failures justify them. `Style.vtext` remains
  the control artifact even when a tuned model is used.

Graduation evidence:

- corpus -> `Style.vtext` extraction with cited examples;
- VText generation that applies selected `Style.vtext` artifacts and retrieved exemplars;
- style review output over generated and edited text, including
  voice-preservation and model-tic diagnostics;
- edit-to-candidate-observation flow with reviewable provenance;
- context routing that applies different `Style.vtext` artifacts for at least two document
  types or audiences.

## Choir Base

Target:

- Base is a reconciliation and sharing substrate over private files, source
  artifacts, publication artifacts, and app/computer state projections.
- Multi-tenant server Base is the full shape: per-user isolation, source refs,
  access policy, conflict records, auditability, and recovery.
- Local desktop Base is a scaled-down proof and utility surface, not the source
  of the architecture.
- File Provider integration should expose selected Base/project files into
  macOS/iOS file surfaces without making the File Provider extension the sync
  brain.

Graduation evidence:

- reconciliation-kernel tests over create/update/delete/rename/conflict cases;
- product API proof for owner-scoped files/artifacts;
- staged server proof before claiming multi-user sync;
- File Provider proof only after Apple developer signing/notarization path is
  available.

## Desktop App

Target:

- The Mac desktop app wraps the existing Svelte desktop and adds native host
  capabilities where the web app cannot: file-provider utilities, local VM
  management, native menus/notifications, and local resource status.
- Wails v3 is the preferred experimental wrapper path. It is replaceable by
  Wails v2, Tauri, or Electron if evidence shows v3 blocks product progress.
- Apple Virtualization should plug into `vmctl` as a Darwin VM backend with the
  same lifecycle semantics as the server VM path where feasible.

Graduation evidence:

- buildable signed/notarizable Mac app path;
- Svelte desktop running inside the wrapper;
- native host API boundaries documented and tested;
- local VM lifecycle proof that does not claim server/multi-tenant behavior.

## Automatic Radio And Voice

Target:

- Automatic Radio is the screenless operating surface for the automatic
  computer, not just TTS over articles.
- Playback should continue indefinitely by walking breadth/depth over relevant
  source items, VTexts, podcasts, videos, emails, private work artifacts, and
  agent updates.
- User speech is both control and content. A user can interrupt, redirect,
  monologue, publish a take, or create source material for later retrieval.
- STT/TTS can start as batch or nearline. Realtime voice models are optional
  later, not a v0 requirement.
- Mobile native apps should focus on allowed screenless/radio surfaces, leaving
  virtualization/app-store-like/deep computer behavior server-side.
- Watch is a useful forcing function for screenless control.

Graduation evidence:

- news/source queue can continuously produce fresh relevant items;
- source-ledger and live-search retrieval combine into weighted story manifests;
- STT creates durable transcripts with speaker/time metadata;
- TTS reads VText/source-backed content with queue state;
- user interruptions change queue direction without ending the session;
- private workflow radio has access-policy-aware retrieval.

## Platform And Local Continuum

Target:

Choir should support a continuum:

```text
thin client
  -> server/cloud computer
  -> single-tenant server/workstation node
  -> desktop app controlling a local node
  -> desktop app running local VMs and optionally local models
```

The same product objects should appear across that continuum: computer, source
artifact, VText, Base item, AppChangePackage/adoption, publication, run
acceptance, and rollback refs.

## Graduation Rule

A target architecture item moves into `current-architecture.md` only when the
repo and staging evidence support it. The promotion note should name:

- code paths;
- tests;
- deploy/staging identity where platform behavior changed;
- product-path proof;
- remaining caveats.
