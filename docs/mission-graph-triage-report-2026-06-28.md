# Mission Graph Triage Report — 2026-06-28

**Mission:** M19 — Mission Graph Triage
**Conjecture:** "The 27 open_handoff missions in the mission graph can be
triaged and consolidated."
**Date:** 2026-06-28
**Status:** complete

## Conjecture Verdict

**Supported.** All 27 open_handoff missions were classified. 18 of the 27
were resolved (15 superseded, 3 settled); 9 remain active open loops. The
graph is materially cleaner: open_handoff dropped from 27 to 9, and every
superseded node carries an explicit successor link and triage note.

## Strong Definitive Statement

The mission graph is no longer an open-loop graveyard. After this triage,
every remaining `open_handoff` node is a genuinely active mission with a
documented relationship to the runtime refactor critical path or the
post-choir-in-choir force multiplier. The 18 resolved nodes are preserved as
historical evidence with explicit successor links — no nodes were deleted,
and the DAG remains indexable.

## Summary Counts

| Classification | Count | Disposition |
|---|---|---|
| active | 9 | remain `open_handoff`, annotated with triage notes |
| superseded | 15 | marked `superseded` with successor link |
| settled (was stale) | 3 | marked `settled` — completed reports / self-declared done |
| **total** | **27** | |

Graph status distribution before and after:

| status | before | after |
|---|---|---|
| settled | 40 | 43 |
| open_handoff | 27 | 9 |
| superseded | 2 | 17 |
| planned | 9 | 9 |
| working | 4 | 4 |

## Active Missions (9)

These remain `open_handoff` and should be tracked in the mission suite.

| id | kind | note |
|---|---|---|
| texture-product-loop-recovery-v0 | spine | Texture product-loop repair (prompt→V1→researcher/super→V2+). Cutover portion absorbed by M8 Texture extraction. Absorbs texture-hard-cutover-v0 and vtext-live-cadence-repair-v3. |
| m5-wire-on-settlement | spine | M5 route-switch evidence gate (maxProc>1, zero accounting leaks). Deferred until M8 durable actors. Absorbs wire-settlement-substrate-v0 (M5a). |
| computer-recovery-system-monitor-v0 | side | System Monitor app + recovery substrate. Observability overlaps M20/M22; app UI is distinct. Post-choir-in-choir. |
| conductor-url-source-routing-h029-v0 | side | Source intake into Texture (YouTube, podcast, PDF) + H029 conductor prompt fix. Overlaps surface-ontology-cleanup and source-system-loop8. |
| deploy-impact-isolation-cache-v1 | side | Deploy speed (host-service split, cache). Partially complete. Low-priority dev-infra. |
| node-b-storage-retention-v0 | side | vm-state retention + recovery-snapshot budget. Nix-store piece settled. Low-priority ops. |
| source-system-loop8-simplify-v0 | side | Source/VText/publication quality (stabilize UI, professional exports, modularize). Absorbs 3 superseded source missions. Depends on M8. |
| super-console-real-zot-cutover-v0 | side | Real zot process in each computer + terminal color fix. Concrete scoped fix. Absorbs super-console-source-mount-promotion-v0. |
| surface-ontology-cleanup-h027-h029-v0 | docs_truth | H027-H029 cleanup (Trace/Terminal/Browser as normal apps). Trace deletion overlaps M8; H029 overlaps conductor-url-source-routing. |

## Superseded Missions (15)

Each is marked `superseded` in the YAML with a successor link. No nodes
deleted; historical evidence preserved.

| id | successor |
|---|---|
| texture-hard-cutover-v0 | texture-product-loop-recovery-v0 + M8 (Texture extraction) |
| doc-truth-drift-context-v0 | docs-truth-system-v1 (settled) + M15 |
| choir-grand-deformation-v0 | mission suite M8→M9→M10 |
| global-wire-living-vtext-newsroom-v1 | mission-3 (ingestion rebuild, 3c-distribution) |
| platform-source-service-vtext-publication-campaign-v1 | mission-3 + source-system-loop8-simplify-v0 |
| source-system-hard-review-2026-06-06 | source-system-loop8-simplify-v0 |
| super-console-source-mount-promotion-v0 | super-console-real-zot-cutover-v0 + M9 |
| vtext-client-ready-source-transclusion-pretext-v0 | source-system-loop8-simplify-v0 |
| vtext-live-cadence-repair-v3 | texture-product-loop-recovery-v0 |
| vtext-source-entities-multimedia-transclusion-v0 | conductor-url-source-routing-h029-v0 + source-system-loop8-simplify-v0 |
| web-surface-rationalization-v0 | road-ahead §5b (Web Lens) + M8 (browser extraction) |
| wire-autonomous-ingestion-v1 | mission-3 (self-declared archived) |
| wire-settlement-substrate-v0 | m5-wire-on-settlement (consolidated back into M5) |
| geometry | road-ahead-2026-06-27 (vision operationalized) |
| demo-stability-foundations-v0 | road-ahead §5 UI/UX (stale; specific bugs likely fixed) |

## Settled Missions (3, were stale/completed)

| id | reason |
|---|---|
| doctrine-conformance-findings-2026-06-13 | completed doctrine conformance pass; historical evidence |
| report-universal-wire-production-recovery-2026-06-10 | incident resolved; backpressure fix shipped (27f4eaf8) |
| run-memory-v0 | paradoc self-declares "completed in repo"; lessons mined into reviews |

## Consolidation Clusters

The triage identified four natural consolidation clusters. The dominant
mission in each cluster remains active; the others are superseded into it.

1. **Texture cutover / product loop** — dominant: `texture-product-loop-recovery-v0`.
   Absorbs `texture-hard-cutover-v0`, `vtext-live-cadence-repair-v3`. The
   V-name deletion residue is absorbed by M8 (Texture extraction).

2. **Source / VText / publication quality** — dominant:
   `source-system-loop8-simplify-v0`. Absorbs
   `platform-source-service-vtext-publication-campaign-v1`,
   `source-system-hard-review-2026-06-06`,
   `vtext-client-ready-source-transclusion-pretext-v0`. The source-service
   pipeline piece is covered by the active ingestion rebuild (mission-3).

3. **Wire settlement** — dominant: `m5-wire-on-settlement` (M5). Absorbs
   `wire-settlement-substrate-v0` (M5a), `wire-autonomous-ingestion-v1`,
   `global-wire-living-vtext-newsroom-v1`. The active Wire pipeline is
   mission-3; M5 is the deferred evidence gate.

4. **Super Console / zot** — dominant: `super-console-real-zot-cutover-v0`.
   Absorbs `super-console-source-mount-promotion-v0` (promotion piece → M9).

## Missions to Add to the Mission Suite

The 9 active open_handoff missions are not yet tracked as discrete items in
`docs/mission-suite-2026-06-28.md`. Recommended additions (sequenced after
the runtime refactor / choir-in-choir unless noted):

- **M25: Texture product-loop repair** — `texture-product-loop-recovery-v0`.
  Sequence after M8 (Texture extraction). The cutover is absorbed by M8; the
  product-loop behavior repair remains.
- **M26: M5 wire-settlement evidence gate** — `m5-wire-on-settlement`.
  Sequence after M8/M9/M10. The production falsifier cycle (maxProc>1).
- **M27: Source/VText/publication quality (Loop 8)** —
  `source-system-loop8-simplify-v0`. Sequence after M8 (modularization).
- **M28: Texture source intake + H029 conductor fix** —
  `conductor-url-source-routing-h029-v0`. Can start the conductor prompt
  default fix now; source-intake substrate after M8.
- **M29: Surface ontology cleanup H027-H029** —
  `surface-ontology-cleanup-h027-h029-v0`. Sequence after M8 trace deletion.
- **M30: Super Console real zot cutover** — `super-console-real-zot-cutover-v0`.
  Concrete scoped fix; can start now (no runtime-refactor dependency for the
  zot-binary + color-contrast fix).
- **M31: System Monitor app + recovery substrate** —
  `computer-recovery-system-monitor-v0`. Post-choir-in-choir (road-ahead §5).
  Observability substrate overlaps M20/M22.
- **M32: Node B vm-state retention** — `node-b-storage-retention-v0`.
  Low-priority ops; can start anytime.
- **M33: Deploy impact isolation + cache** — `deploy-impact-isolation-cache-v1`.
  Low-priority dev-infra; partially complete; can start anytime.

## Files Changed

- `docs/mission-graph.yaml` — 18 node status updates (15 → superseded,
  3 → settled) with triage comments and successor links; 9 active nodes
  annotated with triage notes. No nodes deleted. DAG edges preserved.

## Files Created

- `docs/mission-graph-triage-report-2026-06-28.md` — this report.

## Invariants Honored

- No mission nodes were deleted. Superseded nodes retain their `path`,
  `ledger`, `depends_on`, `enables`, and `sources` for historical
  discoverability.
- Historical evidence is preserved; `kind: evidence` nodes that were
  completed reports were marked `settled`, not removed.
- The graph remains a DAG — no edges were added or removed, only statuses
  and annotations changed.
- The graph remains an index, not a second ledger; Parallax State in each
  paradoc remains the source of current mission state.
