# Source And Publication Consolidation - 2026-06-06

**Status:** cleanup ledger  
**Deleted source:** `docs/platform-dolt-publication-retrieval-citation-research-2026-05-16.md`

## Why Delete

The deleted file was a long research/design input for the first platform Dolt,
publication, retrieval, and citation mission. Its central contract has since
been promoted into:

- [current-architecture.md](current-architecture.md)
- [source-external-data-publication.md](source-external-data-publication.md)
- [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md)

Keeping the research report as an indexed current doc made the source system
look less settled than it is. The current authority is now the source and
publication contract.

## Retained Insights

- **Publication is a trust-domain transition, not page rendering.** Private
  computers own mutable private truth; platform/public ledgers own selected
  immutable projections.
- **Exact refs beat mutable labels.** Published versions, source spans,
  retrieval manifests, and citation edges should anchor to immutable refs and
  hashes, not titles or current document heads.
- **Embeddings and indexes are derived caches.** Retrieval rows should preserve
  source version/span/chunk identity so search indexes can be rebuilt.
- **Citation edges need lifecycle state.** Candidate, asserted, verified,
  disputed, retracted, and superseded states prevent the graph from becoming
  permanent citation-shaped spam.
- **Redaction creates a new projection hash.** Do not publish private revision
  ids and rely on access control to hide omitted content.
- **Platform Dolt is a service database.** Browser and user-computer runtimes
  write through product/platform APIs; platform Dolt should not become a
  browser-accessible database or live private editor.
- **Single-primary platform writes are fine.** Dolt branches help proposals and
  review, but Choir still needs product-level admission, consent, verification,
  conflict, rollback, and publication policy.
- **Provenance is product structure.** VText revisions, source artifacts,
  agents, worker runs, verifier contracts, publication events, and citations
  should be explainable as entities, activities, and agents.

## Current Authority

Use [source-external-data-publication.md](source-external-data-publication.md)
for source ingestion, source entities, transclusion, publication, export,
retrieval, and citation requirements.

Use [current-architecture.md](current-architecture.md) for the platformd and
Platform Dolt boundary.
