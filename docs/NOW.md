# NOW — Evidence-Scoped Current State

**Status:** manually refreshed bootstrap view. It is a dated observation, not
doctrine and not a promise of feature readiness.

## Observation

| Observed at | Scope | Result | Evidence | Freshness rule |
| --- | --- | --- | --- | --- |
| 2026-07-10 00:20:09 UTC | Public health endpoint | `GET https://choir.news/health` returned HTTP 200 with `status: ok`, proxy service, and `vmctl_routing: enabled` / `vmctl_status: ok`. The response identified deployed commit `14f56211f6163408abd21a629423c31f48fd4c8f`, deployed at `2026-07-09T05:42:19Z`. | Recorded health response during the documentation-authority reduction. | Re-observe before making any staging availability, routing, or deployed-commit claim. A health response alone is not product-path acceptance. |
| 2026-07-10 00:20:09 UTC | Public lifecycle counters | The same response reported historical `api.resolve` errors and a maximum resolve duration of 60,053 ms. The counters have no request-window or cause attribution in this observation. | Same response. | Treat as a diagnostic lead only; do not call it a current incident or a repaired condition without a scoped probe. |
| 2026-07-09 local checkout | Source tree | Local `HEAD` was `21b159be94b3d020430fb1f8e494f6b98134fb69`; it is not the deployed health commit above. | `git rev-parse HEAD` and `git log -1` in this worktree. | Git state changes with every commit; never infer staging identity from the checkout. |

## Implementation Status Snapshot

These are source-tree classifications, not fresh staging acceptance. “Live”
means a wired product/service path exists in the repository; it does not mean
the path was re-proven on staging at the timestamp above.

| Subsystem | Current classification | Evidence boundary / claim ceiling |
| --- | --- | --- |
| Automatic computer / autoputer | **Live substrate; partial product; terminology cutover pending.** Web desktop, per-user runtime, VM lifecycle, appagents, and host services are wired. Residual `sandbox` names are implementation/service names. | Candidate-as-ComputerVersion routing and personal promotion are not load-bearing; capsules are not part of the default runtime tool path. |
| Choir web UI | **Live.** The frontend registry is the authoritative code inventory; individual apps may still be partial or compatibility surfaces. | App presence does not prove its target workflow is complete. |
| Choir macOS app | **Buildable wrapper; shipment/acceptance unknown here.** | Shares the Svelte UI; do not claim distribution or daily-driver acceptance without a dated product-path proof. |
| Choir CLI | **Code-present, buildable Phase 1.** Submit/read/trajectory/search/Wire/API-key commands exist. | No supported distribution or recorded staging acceptance here; no package, adoption, acceptance, promotion, rollback, or `/goal` verbs. |
| Choir Base / File Provider | **Substantial tested substrate; product wiring incomplete.** Append-only journal, derived tree, blobs, File Provider, and `/api/base/*` helpers exist. | No current deployed service owns the Base API; it is not a replacement canonical store for embedded Dolt. |
| Autopaper | **Tabled concept; no active Definition or canonical implementation.** | Do not infer product status from issue residue. Revival requires a fresh Definition grounded in current Texture, source, publication, and scheduling contracts. |
| `corpusd` | **Code-present and deployment-wired public store/API service.** | Owns service writes and sanitized public reads, not semantic authorship. D-WIRE's move to sql-server is decided but not proven executed by this snapshot; fresh staging health and end-to-end publication were not re-proven. |
| `sourcecycled` | **Code-present and deployment-wired experimental adapter.** | Poll cycle/queue state is in memory and lost on restart. Durable source/publication truth must be projected into owned artifact stores; end-to-end article production is not proven here. |
| Capsules | **Partially implemented and inert in the default product path.** Host/executor/tool code exists, but the default runtime does not install a production capsule executor/tool path. | Treat capsule semantics as target until wiring, isolation, transaction, and product-path evidence exist. |
| Features activation | **Live adoption/lineage protocol; not real served-code activation.** | Approval, freshness, recipient build, and lineage records do not prove a runtime/UI route switch, binary restart, or exercised rollback. |
| `/goal <definition.md>` | **External harness convention.** | Not implemented by Choir CLI, prompt bar, or runtime as an end-to-end Definition runner. |

## Current Reading Rules

- [`current-architecture.md`](current-architecture.md) is the detailed
  Live/Target/Retired architecture memo. It must cite code or fresh staging
  evidence for claims labeled current.
- [`ACTIVE.md`](ACTIVE.md) says what is confirmed as active work; it does not
  imply implementation or deploy status.
- A stale entry in this view is **unknown**, not silently current. Refresh with
  a source, timestamp, and scope rather than editing prose to sound timeless.

## Refresh Contract

Refresh this view when a staging deployment lands, an active Definition changes,
or a current-state claim is used to choose a technical route. Until then, this
document has no authority beyond the exact observations, classifications, and
evidence ceilings above.
