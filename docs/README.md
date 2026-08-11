# Choir Documentation

This is a routing document, not a second doctrine or a list of every useful
file. Choir documentation has four lanes:

1. **Semantics** — durable terms, invariants, and authority boundaries.
2. **NOW** — dated, evidence-scoped facts about the checked-out system and
   staging.
3. **ACTIVE** — the currently confirmed work surface.
4. **History** — an opt-in historical corpus for retired designs, missions, and
   evidence; it is visible in the worktree but never current authority.

Do not infer current authority from a document's existence, filename, or
archive location. `docs/choir-doctrine.md` is the apex doctrine and `AGENTS.md`
is the operating contract.

## Default Reading Packet

Read these ten documents or views before broad architecture or product work.
They are the whole default packet; historical material is opt-in.

1. [`README.md`](../README.md) — human and developer entry point.
2. [`AGENTS.md`](../AGENTS.md) — operating contract and mutation/evidence
   ceremony.
3. [`choir-doctrine.md`](choir-doctrine.md) — apex doctrine.
4. [`semantic-registry.md`](semantic-registry.md) — compact, non-overriding
   map of the doctrine's stable semantics.
5. [`NOW.md`](NOW.md) — dated facts and freshness limits.
6. [`ACTIVE.md`](ACTIVE.md) — confirmed active/completed Definitions and
   work-state caveats.
7. [`computer-ontology.md`](computer-ontology.md) — persistent computer,
   candidate, promotion, and rollback contract.
8. [`runtime-invariants.md`](runtime-invariants.md) — runtime authority and
   causality contract.
9. [`texture-agentic-invariants-2026-06-13.md`](texture-agentic-invariants-2026-06-13.md)
   — canonical artifact contract.
10. [`source-external-data-publication.md`](source-external-data-publication.md)
    — source, provenance, and publication contract.

Views point to source authority; none can override doctrine or a domain contract.
Read the relevant contract before touching its protected surface.

## Authority map

- `choir-doctrine.md` is apex doctrine; `choir-vision.md` is the product
  north star and defers to doctrine.
- `semantic-registry.md` is a compact derived map; domain contracts govern only
  their stated scope and cannot override doctrine.
- `NOW.md` holds dated, evidence-scoped observations. Stale observations become
  unknown; follow its links for fuller current architecture and platform state.
- `ACTIVE.md` is the curated work view. A promoted working Definition is the
  sole current authority root; completed Definitions remain evidence.
- `mission-graph.yaml` is discovery metadata. Definitions own their state; the
  graph and Git history are not executable authority.
- `archive/` is searchable historical material, never current doctrine,
  executable mission authority, or part of the default packet. Verify retained
  claims against current doctrine, contracts, Definitions, and observed state.
- Bounded completion receipts remain in the locations required by their
  Definitions; Git history is the forensic recovery surface for anything else.
- `doc-authority-manifest.yaml` is navigation metadata, not doctrine or a
  historical catalog.

When a semantic, current-state, or active-work claim changes, update its source
authority and this compact view together. Do not add another orientation page.
