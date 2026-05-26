# Design documents index

**Updated:** 2026-05-26

Approved platform designs (implement in phase order):

| Document | Scope |
|----------|--------|
| [design-search-provider-plane-v1.md](./design-search-provider-plane-v1.md) | Gateway search: durable health, exponential backoff, parallel fan-out |
| [design-vtext-platform-v3.md](./design-vtext-platform-v3.md) | VText single writer, persistent workers, context packet, chyron, trajectories |

**Related missions (execution tracking):**

- `mission-search-provider-plane-v1.md` — **active** deploy/prove search provider plane (use `/goal` in doc)
- `mission-research-runtime-evidence-cadence-v1.md` — evidence projections, gateway policy (downstream)
- `mission-vtext-lineage-aware-runtime-cadence-v2.md` — VText cadence (blocked on search plane deploy)

**Implementation order:** P0 search plane → P1–P4 VText platform (see v3 doc).
