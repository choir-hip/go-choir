# Choir Documentation

This is a routing document, not a second doctrine or a list of every useful
file. Choir documentation has four lanes:

1. **Semantics** — durable terms, invariants, and authority boundaries.
2. **NOW** — dated, evidence-scoped facts about the checked-out system and
   staging.
3. **ACTIVE** — the currently confirmed work surface.
4. **History** — searchable evidence, retired proposals, ledgers, and reviews.

Do not infer current authority from a document's existence, filename, or
archive location. `docs/choir-doctrine.md` is the apex doctrine and `AGENTS.md`
is the operating contract.

## Default Reading Packet

Read these eleven documents or views before broad architecture or product work.
They are the whole default packet; historical material is opt-in.

1. [`README.md`](../README.md) — human and developer entry point.
2. [`AGENTS.md`](../AGENTS.md) — operating contract and mutation/evidence
   ceremony.
3. [`choir-doctrine.md`](choir-doctrine.md) — apex doctrine.
4. [`semantic-registry.md`](semantic-registry.md) — compact, non-overriding
   map of the doctrine's stable semantics.
5. [`NOW.md`](NOW.md) — dated facts and freshness limits.
6. [`ACTIVE.md`](ACTIVE.md) — confirmed active Definitions and work-state
   caveats.
7. [`computer-ontology.md`](computer-ontology.md) — persistent computer,
   candidate, promotion, and rollback contract.
8. [`runtime-invariants.md`](runtime-invariants.md) — runtime authority and
   causality contract.
9. [`texture-agentic-invariants-2026-06-13.md`](texture-agentic-invariants-2026-06-13.md)
   — canonical artifact contract.
10. [`source-external-data-publication.md`](source-external-data-publication.md)
    — source, provenance, and publication contract.
11. [`definitions/og-dolt-heresy-completion-2026-07-08.md`](definitions/og-dolt-heresy-completion-2026-07-08.md)
    — the one active top-level product Definition.

The compact registry and views point to the source documents; none can override
the doctrine, operating contract, or an explicitly promoted domain contract.
Read a relevant domain contract in depth before touching its protected surface.

## Semantics

- [`choir-doctrine.md`](choir-doctrine.md) is the normative optimization target,
  conjecture set, evidence semantics, and heresy inventory.
- [`semantic-registry.md`](semantic-registry.md) is the small semantic map. It
  deliberately excludes deployment facts, service names, mutable work state,
  and unpromoted architecture decisions.
- Domain contracts above are authoritative only within their stated scope and
  defer to the doctrine on conflict.

## NOW

[`NOW.md`](NOW.md) records evidence-scoped observations with an observation
time, source, and refresh trigger. A stale observation becomes unknown; it is
not silently treated as a live product fact. For fuller current architecture or
desktop/app state, follow its links to
[`current-architecture.md`](current-architecture.md) and
[`platform-os-app-state.md`](platform-os-app-state.md).

## ACTIVE

[`ACTIVE.md`](ACTIVE.md) is the curated work view. The product umbrella remains
the one top-level Definition in the default packet. The supporting
[`documentation-authority-reduction-2026-07-09.md`](definitions/documentation-authority-reduction-2026-07-09.md)
is intentionally outside that packet: it changes documentation navigation only
and cannot override product semantics.

`mission-graph.yaml` remains the committed legacy mission-corpus graph during
the Beads cutover transition. It is not an onboarding list: its historical
nodes, duplicate placeholders, and unverified statuses must not be read as
current work without the ACTIVE view or direct mission evidence.

## Historical Material

Historical missions, ledgers, raw evidence, reviews, and superseded proposals
are being removed from the worktree because generic retrieval treats their
prose as current context. Git history remains the recovery and forensic
surface; it is deliberately not part of the ordinary reading packet.

[`doc-authority-manifest.yaml`](doc-authority-manifest.yaml) is slim
machine-readable navigation metadata, not a second doctrine or a historical
catalog.

## Maintenance

When a semantic, current-state, or active-work claim changes, update its source
authority and the appropriate compact view in the same change. Do not add a
new orientation page to the default packet. See the active documentation
maintenance Definition for the present migration and the future strict-live
checker work.
