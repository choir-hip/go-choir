# Node B Nix Store Retention v0 Ledger

## 2026-06-14 — paradoc opened

- Claim: Node B needs a separate Nix-store retention mission rather than
  folding Nix GC policy into the completed VM-state cleanup mission.
- Move: created `docs/mission-node-b-nix-store-retention-v0.md` as the next
  Parallax source program.
- Evidence: read-only probes showed 476G root filesystem with 357G used and
  117G free, `/nix/store` at 243G, `/var/lib/go-choir/vm-state` at 127G,
  system generations 489-496 retained, 9,409 absolute dead `/nix/store` paths,
  and large dead guest-image/playwright/runtime-deps/closure-info paths.
- Safety boundary: no live `nix store gc`, root deletion, guest image deletion,
  or retention config change was performed by this move.
- Expected ΔV: 0 against implementation obligations; this creates the source
  program for the next mission.
- Actual ΔV: 0.

## 2026-06-14 — read-only Nix root classification

- Claim tested: a typed root inventory can separate rollback-critical roots
  from stale/ad hoc roots before any Nix GC, generation pruning, or root
  deletion is authorized.
- Move: added `scripts/node-b-nix-root-report`, ran it against Node B, and
  saved the Markdown evidence packet at
  `docs/evidence/node-b-nix-root-classification-2026-06-14.md`.
- Evidence: captured at `2026-06-14T20:15:21Z`; root filesystem had
  `350.96 GiB` used and `122.16 GiB` available; `/nix/store` apparent size
  remained `243G`; `nix show-config` reported `min-free=0`,
  `max-free=9223372036854775807`, and `auto-optimise-store=true`; dead store
  path count was `9,423`.
- Root classification: keep current system, booted system, running-process
  roots, active service/guest runtime paths, root-owned flake registry cache,
  and the four newest system generations 493-496. Stale rollback candidates
  are generations 489-492. Temporary/ad hoc roots needing formal TTL/owner
  rules include `/tmp/go-choir-*result`, `/tmp/guest-image-*`, and
  `/tmp/go-choir-guest-image-new`. Invalid root:
  `/opt/go-choir/result -> /nix/store/srzb724qbl2s871jjkjja61zdfqwzv2j-prefetch-npm-deps-0.1.0`.
- Proposed policy: keep four system generations; set routine sweep target to
  180 GiB free with a 120 GiB emergency floor; set Nix daemon
  `min-free=120 GiB` and `max-free=180 GiB`; run weekly off-peak store
  optimisation rather than per-build `auto-optimise-store`; formalize result
  roots with type/owner/TTL and delete only through the reviewed service path
  or separately owner-approved manual action.
- Safety boundary: no manual `nix store gc`, root deletion, generation pruning,
  guest image deletion, service restart, or Node B config mutation was
  performed by this move. `nix-store --gc --print-roots` did emit its normal
  stale-temporary-root cleanup messages; no live GC was invoked.
- Verification: `scripts/node-b-nix-root-report --local --format json` passed;
  `.github/scripts/deploy-impact-classify-test` passed after adding the new
  report command to the operator-tooling ignore set.
- Expected ΔV: 4, covering root classification, generation target, GC/free-space
  thresholds, and optimisation cadence.
- Actual ΔV: 4. Remaining V=3: host config, CI/deploy, post-cleanup proof.
