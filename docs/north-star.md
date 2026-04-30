# Choir North Star

**Last updated:** 2026-04-30

The Automatic Computer already exists in deployed form: web desktop, backend
services, appagents, and NixOS-on-NixOS VM infrastructure. The current task is to
stabilize the deployed system around versioned living documents, background VM
execution, publication, and later citation/compute economics.

Read [docs/current-architecture.md](current-architecture.md) first. It is the
streamlined architecture memo for the current phase.

## Product Frame

Choir is a web desktop with apps. Some apps grow into appagents; most can remain
plain display/control surfaces. The first appagent is `vtext`: a durable,
versioned living document that accumulates user edits, appagent synthesis, worker
findings, evidence, artifacts, and later publication history.

The dark factory behind the desktop contains researchers, supers, cosupers,
background VMs, evidence, artifacts, tests, previews, and Trace. Its job is to
advance living documents and produce publishable artifacts without making raw
agent orchestration the primary UI.

## Sequence

1. Stabilize `vtext`, researcher, super, user edits, and Trace.
2. Add ingestion skills for URLs, YouTube transcripts, text/Markdown/PDF/EPUB
   uploads, and later multimedia display apps whose content can be transcluded
   into `vtext`.
3. Add publication.
4. Add Pretext-based rendering/transclusion.
5. Add citation mechanics.
6. Add CHIPS and citation/compute economics.

Do not implement CHIPS, wallets, staking, token billing, or public citation
scoring yet. Do preserve document versions, provenance, evidence, artifacts,
citations/citation candidates, VM/model attribution, publication boundaries, and
compute accounting where available.

## Anti-Collapse Rules

- Chat history is not canonical state; `vtext` versions are.
- Worker updates are not document patches; `vtext` owns document synthesis.
- Mutable super/cosuper work belongs in background VM forks, not the live desktop.
- Platform Dolt is a ledger for platform-visible facts, not a hot-path message
  bus.
- Providers are adapters; no LLM or search provider is architecturally required.
