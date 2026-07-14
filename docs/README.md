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
11. [`definitions/choir-autoputer-completion-2026-07-14.md`](definitions/choir-autoputer-completion-2026-07-14.md)
    — the sole active top-level product Definition and mission orchestrator.

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
the one top-level Definition in the default packet. The completed
[`documentation-authority-reduction-2026-07-09.md`](definitions/documentation-authority-reduction-2026-07-09.md)
is intentionally outside that packet: it is a deletion/maintenance receipt and
cannot override product semantics.

`mission-graph.yaml` is a minimal discovery index for retained Definitions. A
Definition owns its own state; the graph, Beads, and Git history do not override
it.

## Historical Material

[`archive/`](archive/README.md) is a deliberately restored, searchable corpus
of historical designs, missions, reviews, and hypotheses. It exists because
old thinking can hold future value; it is **not** executable mission authority,
current doctrine, or part of the default reading packet. When a historical
claim matters, verify it against the current doctrine, domain contracts,
Definition, and observed system state before acting on it.

The pre-purge raw process-evidence and ledger corpus remains outside the
worktree; bounded current evidence receipts remain where the active Definition
requires them. Git history is still the forensic recovery surface for material
not represented in the historical corpus.

[`doc-authority-manifest.yaml`](doc-authority-manifest.yaml) is slim
machine-readable navigation metadata, not a second doctrine or a historical
catalog.

## Maintenance

When a semantic, current-state, or active-work claim changes, update its source
authority and the appropriate compact view in the same change. Do not add a
new orientation page to the default packet. The completed documentation
authority Definition records the retention and validation boundary.
