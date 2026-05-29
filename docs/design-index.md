# Design documents index

**Updated:** 2026-05-26

Approved platform designs (implement in phase order):

| Document | Scope |
|----------|--------|
| [design-search-provider-plane-v1.md](./design-search-provider-plane-v1.md) | Gateway search: durable health, exponential backoff, parallel fan-out |

**Related missions (execution tracking):**

- `mission-search-provider-plane-v1.md` — **active** deploy/prove search provider plane (use `/goal` in doc)
- `mission-research-runtime-evidence-cadence-v1.md` — evidence projections, gateway policy (downstream)
- VText cadence follow-up should use `mission-vtext-live-cadence-repair-v3.md`.

**Implementation order:** P0 search plane -> VText live cadence repair.
