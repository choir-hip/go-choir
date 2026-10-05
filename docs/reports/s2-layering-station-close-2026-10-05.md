# S2 — Layering Runtime From Release: Station Close Report

**Date:** 2026-10-05
**Station:** `choir-appdev-s2-layering-runtime-from-release-2026-10-01`
**Terminal receipt:** `s2-station-terminal-2026-10-05` (station file)
**Mutation class:** red (guest boot path, updater trust boundary) — closed
under the frozen transition contract `s2-layered-transition-v1`.

## Outcome

S2 **closed**. The layered guest release path is live: per-computer app-layer
Nix closures materialize on the guest data disk under the private store,
execute at original store paths over the read-only EROFS base + overlay, swap
through the updater authority in one transaction (executable + frontend +
state compat + event head), and apply without a VM reboot. CI mints, signs,
and pushes app-layer releases to tracking computers when the only runtime
change is the autoputer package (`deploy_app_layer=true`).

## Acceptance evidence (all deployed-proof class)

| # | Criterion | Receipt |
|---|---|---|
| 1 | No-reboot apply, backend+frontend | Patch-built release `57605ac`: same fc pid `189418`, boot `c643bbca`; built-bytes frontend join (`dataset.choirBuildCommit`, `__CHOIR_BUILD__`, `s2-frontend-marker` meta — impossible to forge via offer injection, inline SPA omitted) |
| 2 | CI-driven push + t2h | CI run `37378327268` (`4ef44901`): `app-layer push: 2/2 healthy, 0 skipped, time-to-healthy=131s`; guest `current/` → release `211ef63a…` marker `app-layer-4ef4490123ee` |
| 3 | Refusal fences | Six incompatible offers refused pre-mutation, reason strings asserted, pins unchanged, each discharged |
| 4 | Post-disposal+reboot base-absent refusal | Builder env disposed + guest reboot (boot `7c036b4b`, replay-rebuilt projection); mismatched offer refused pre-mutation, predecessor serving |
| 5 | Store boundary | `/mnt/persistent/choir-updater/store/<hash>-autoputer`, 12 GC roots, `/nix/store` ro erofs+overlay, no daemon — guest console confirms exec over private store |
| 6 | Rollback | Retained predecessor `2811c779` restored: same fc `3123628`, boot `7f725ae1`, exec reverted with pointer |

Criterion-2 continuity pins: one Firecracker process
(`firecracker_spawned` 21:47:46Z → hibernate 22:18:25Z) and one guest boot
id `2bab2058` across the 21:52 push — runtime restart only.

## Consensus record

- Authoring panel: accept after send-back round (`frozen_ref main@653d975c`).
- Acceptance panels: 7–1, 6–0, 4–2 send-backs — each named precise re-legs,
  each ran green.
- Loop-break (lateral): reframe to the frozen `(B,U,R)` transition contract;
  landed as `s2-layered-transition-v1` + base-keyed release dirs (`304eac64`).
- Close round 1: **4 approve / 3 send-back** — named criterion-2 CI-origin
  and the discharge-append wedge.
- Close round 2: **6 approve / 1 send-back** — sole send-back named only
  receipt completion (fc/boot pins for the CI push), discharged by the
  continuity pins above; no mechanism gap remained.

## What S2 leaves downstream

- `(B,U,R)` transition tuple: base closure, base-resident updater, target
  runtime. Same-commit transitions are no-ops; layering requires R
  base-absent from B.
- Builder-produced closures (host `choir-builder`, `app-layer-closure.nar`
  + receipt) — S6 consumes the materialization contract for full releases.
- Provenance binding: `code_commit` derived by the builder,
  `builder_receipt_digest`, buildinfo check before pointer swap (S2-c).
- Pending-transition discharge hardened twice: `fae12950` refused-outcome
  journal + `7c0897c2` bounded retry on the failed-event append.

## Open residuals (problem docs, not station blockers)

- `s2-postswap-restart-loop-kills-vm-2026-10-05` — a health-failing release
  can kill FC mid-restore under host pressure; boot-loop guard exists,
  durable fix open.
- `s2-app-layer-offer-bind-gaps-2026-10-05` — canonical-head bootstrap for
  never-committed computers needs an owner-authority decision.
- `s2-refused-apply-wedges-pending-transition-2026-10-05` — bounded retry
  lands the second occurrence; persistent store outage still fails closed
  (documented residual).
- `node-b-deploy-disk-headroom-2026-10-04` — first artifact-GC sweep
  reclaimed 35.4GB; cadence is an S0 decision.

## Heresy delta

- **discovered:** baseline-fallback serving ambiguity (serving health alone
  did not prove the release's frontend); `materialization_failed` append
  can itself fail mid-replay.
- **introduced:** none recorded beyond the receipted restart-loop hazard.
- **repaired:** frontend-by-digest join, refusal wedge (twice-hardened),
  stale-nar reuse class (base-keyed dirs), six in-run finds.

## Rollback

Git revert of slice commits (`9f5aa8a0 c8fb7834 8f06b3d9 e1c3924b e527c169
a3da83c4` + fixes); product path is retained-release restore through the
pinned head — exercised live by criterion 6.
