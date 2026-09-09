# Restore-Zero Deployed Baseline — 2026-09-09

Class: green (evidence only). Purpose: pre-drill staging observation for
`choir-rlm-restore-zero-2026-09-08.md` slice 3 (deployed proof).

## 1. Staging build identities

- Pre-drill proxy: `3ef4405c91c63bf048e34fcdd7df2d3fb4755bbf` (built 20260908151005).
- During slice 3: proxy reported `25e8783bf0b3fc477b6d5f63e262baee4a94bb52`
  (slice-1 descriptor seam) deployed `2026-09-09T01:48:56Z`; a further deploy
  of the slice-2a tree was in progress at observation time.
- Target computer: `computer-03335285269bdba4f94377e56879f9e6`
  (canonical pre-A staging computer; epoch/effects/fence re-observed at drill).

## 2. Lifetime-replay baseline (old build)

- `choir computer replay-completeness --computer computer-0333... --timeout 10m`
  against the pre-slice build: **client timeout after 600s awaiting headers**.
  The lifetime-replay probe does not complete inside 10 minutes on staging.
  This is the cost baseline the tail-bounded probe must beat post-deploy; it
  is not itself a defect receipt against the new code.

## 3. Base-retention observability (public edge)

- `GET https://choir.news/internal/computers/files/watermark?computer_id=...`
  with owner Bearer key: **403 `internal routes are not available from the
  public edge`**. Watermark/base retention is unobservable without host
  access. Base selection for the drill therefore requires an operator step.

## 4. Drill command inventory (product path, no SSH)

- `choir computer checkpoint --computer ID` → restore-set checkpoint file
  (read-safe bind; publishes a head-scoped platform record only when owner
  recovery control is wired).
- `choir computer rematerialize-from-tape --computer ID --checkpoint-file F`
  → staged base-plus-tail rebuild with base/tail counts in the report.
- `choir computer restore --computer ID --checkpoint-file F` → same engine
  through the restore intent path.
- `choir computer replay-completeness --computer ID` → tail-bounded probe
  with base/tail counts (schema v4).
- `POST /api/computers/{id}/lifecycle/cold-recover {"idempotency_key": ...}`
  → owner-scoped recover_current (no dedicated CLI command exists; the
  product API route is the control).

## 5. Blocked prerequisites (recorded, not worked around)

- **Base publication is host-only.** `choir-rebuild-base` needs the platform
  artifacts disk, the computer privacy key, and a watermark record; none is
  reachable from the public edge (see §3). The drill's "select or publish a
  verified compatible base W" step requires an operator with host access.
  Until a base is published, post-deploy `rematerialize-from-tape` must
  refuse with typed 409 missing-base — that refusal is itself drill evidence.
- **No product-path failure-injection control exists.** There is no scoped
  CLI/API control to corrupt a base, interrupt an install mid-write, or
  present a foreign base to a running computer. The interrupt-and-resume
  sub-proof (acceptance item 8, second half) cannot execute until such a
  control exists or the owner directs a host-side procedure. Refusal-path
  coverage without injection (missing base → 409; route/realization
  unchanged) proceeds regardless.
- **Cold-recover has no CLI command.** The product API route above is the
  named control; drill receipts will cite request/response directly.

## 6. Next

After deploy of the slice-2b tree: re-observe staging identity, run the
probe (expect minutes-scale with base/tail counts), run the missing-base
refusal drill, run owner-scoped recover_current, record cost-by-tail
telemetry from the new report fields.
