# NOW — Evidence-Scoped Current State

**Status:** manually refreshed bootstrap view. It is a dated observation, not
doctrine and not a promise of feature readiness.

## Observation

| Observed at | Scope | Result | Evidence | Freshness rule |
| --- | --- | --- | --- | --- |
| 2026-07-10 00:20:09 UTC | Public health endpoint | `GET https://choir.news/health` returned HTTP 200 with `status: ok`, proxy service, and `vmctl_routing: enabled` / `vmctl_status: ok`. The response identified deployed commit `14f56211f6163408abd21a629423c31f48fd4c8f`, deployed at `2026-07-09T05:42:19Z`. | Recorded health response during the documentation-authority reduction. | Re-observe before making any staging availability, routing, or deployed-commit claim. A health response alone is not product-path acceptance. |
| 2026-07-10 00:20:09 UTC | Public lifecycle counters | The same response reported historical `api.resolve` errors and a maximum resolve duration of 60,053 ms. The counters have no request-window or cause attribution in this observation. | Same response. | Treat as a diagnostic lead only; do not call it a current incident or a repaired condition without a scoped probe. |
| 2026-07-09 local checkout | Source tree | Local `HEAD` was `21b159be94b3d020430fb1f8e494f6b98134fb69`; it is not the deployed health commit above. | `git rev-parse HEAD` and `git log -1` in this worktree. | Git state changes with every commit; never infer staging identity from the checkout. |

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
or a current-state claim is used to choose a technical route. The next phase of
the documentation-authority reduction will make these rows generated from
structured evidence where practical. Until then, this document has no authority
beyond the exact observations and times above.
