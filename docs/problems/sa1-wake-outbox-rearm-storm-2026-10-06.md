# SA slice 1: wake-outbox re-arm storm — restart re-feeds the herd (owner guest)

Written 2026-10-06. Mutation class: red (protected surface — actor wake outbox,
lifecycle obligation convergence, persistent Management reconcile, serialized
Dolt engine). Supersedes nothing; extends
`s0m-management-live-occurrence-storm-2026-10-03.md` (fourth recurrence, same
substrate defect) with the restart-survival mechanism confirmed in source and
in the guest trace.

## Symptom (staging, owner computer `computer-03335285`)

The owner guest reboots frequently under host-pressure reclaim. Every boot:

1. `migrateActorWakeOutboxAsync` (`internal/agentcore/runtime.go:2602`) runs
   `Store.MigrateActorWakeOutbox` (`internal/store/lifecycle.go:973`) on the
   full object graph — six kinds (`worker_update`, `work_item`,
   `lifecycle_cancel_intent`, `engineering_assignment`, `tex_rev`, `run`) —
   an O(all objects) scan under the serialized store.
2. For each minted wake key that already exists AND is marked `projected`,
   the migration re-arms it (re-puts the outbox object with fresh hash) when
   its source obligation still derives open — `lifecycle.go:1006-1027`.
   Boot log receipts (owner guest `candidate-fleet-e15cb89f`, console.log.*):
   - 2026-10-05 23:33:47 `minted 2106 pending wakes`
   - 2026-10-06 00:22:23 `minted 2106 pending wakes`
   - 2026-10-06 03:11:44 `minted 2118 pending wakes`
   - 2026-10-06 03:51:22 `minted 2117 pending wakes`
   - 2026-10-06 05:07:09 `minted 2117 pending wakes`
   - 2026-10-06 05:35:48 `minted 2117 pending wakes`
   - 2026-10-06 06:17:33 `minted 2117 pending wakes`
   - 2026-10-06 07:04:45 `minted 2124 pending wakes`
   Same ~2.1k set each boot: almost entirely re-arms, not fresh obligations.
3. The projector (`sweepActorWakeOutbox`, 500ms tick) dispatches each wake.
   Pending `coagent_result` controls bound to long-dead delivery runs mint
   "persistent Management live occurrence received → bound run=<resident>"
   pairs every ~3s for hours (console tail 11:43–12:43Z, run ids
   `4ac32295`, `cefbf1b9`; `from=texture:<doc>` are August-era desks).
   Dead wakes whose backing occurrence is gone dispose at ~1/2s
   (`disposed dead wake result:sha256:…` / `cell:…:packet`), one CAS each,
   while genuine owner work queues behind the herd.

## Root cause — the re-arm cycle has no terminal edge

`MigrateActorWakeOutbox` is idempotent by key but **not convergent by
obligation**: disposing a dead wake (`MarkActorWakeProjected`,
`lifecycle.go:928`) marks the *wake row* projected while leaving the source
obligation (`worker_update` with `Disposition=pending`, open v1
`work_item`, `owner_revision` wake for a stale doc) untouched. Next boot the
migration sees obligation-open + wake-projected and re-arms it. The disposed
wake never discharged its obligation, so the loop is:

```
boot → re-arm ~2.1k wakes → dispatch: dead wake → mark projected
     → obligation still pending → next boot re-arms → …
```

Three compounding amplifiers (unchanged from the 10-03 doc):

- **Per-occurrence O(history) resolve** under `managementReconcileMu` +
  `engineMu` (`ResolvePersistentManagementLiveOccurrence` →
  `ListAllPendingLifecycleUpdates` owner-wide scan + N point reads). N
  obligations × O(history) serialized.
- **No terminal fate for stale obligations.** Nothing ever marks an
  obligation `delivery_attempts_exhausted`/`expired` or checks whether its
  trajectory/work item is still capable of consuming. 199+ live document
  trajectories (oldest 2026-08-16) each keep obligations derivable-open.
- **Full-graph boot scan.** The migration lists every object of six kinds on
  every boot regardless of need — SA cost in its own right (the same boot
  that re-arms 2.1k wakes also ran a 30s `reconcile_terminal_run_outcomes`
  with 39 candidates).

Observed while writing: the guest restarted 4× in 18 min (13:00–13:18Z)
under pressure reclaim; each restart re-ran the whole scan.

## Fix shape (adopted from the 10-03 divergent-panel consensus + v5.1 additions)

1. **Terminal fate as a recorded act.** At dispatch time, an obligation
   whose trajectory is no longer live, or whose bound-delivery attempts
   exceed a budget, gets a durable disposition on the tape
   (`delivery_attempts_exhausted` / `expired`) — a canonical state change,
   never a delete. The migration's "still open" predicate must consult that
   disposition, so discharged obligations stop re-arming.
2. **One-shot migration.** Gate `MigrateActorWakeOutbox` behind a versioned
   marker object: run once per store generation, not per boot. (Deleting it
   outright is the alternative once the cutover is complete; the marker is
   chosen so a missed boot after an old release still converges.)
3. **O(1) occurrence resolve.** `coagent_result` occurrences carry the
   canonical `UpdateID`; resolve by `GetCoagentSourcePacket` point-read
   instead of the owner-wide pending scan. (S0m consensus: load fix and
   discharge fix must land together — resolve alone spins faster.)
4. **Per-desk dispatch gate + paced drain.** At most one outstanding
   Management wake; a reconcile claims an indexed FIFO page and discharges,
   retries-on-state, or scores a terminal fate — never loops without
   changing obligation state.

## Acceptance (deployed, owner guest)

- Restart converges `pending` wake backlog to a bounded floor within a
  stated window and holds it for 24 h (no regrowth on later boots).
- `migrate`-minted wake count on boot ≈ 0 after the marker lands.
- The SMG probe's legs 1–3 pass on the owner computer (an activation opens,
  binds, and reports without being starved).
- No obligation is deleted: every discharge is a tape-visible act.

## Landed fix (2026-10-06)

Commit pending (same mission). The four named mechanisms landed together:

1. **One-shot migration gate.** `MigrateActorWakeOutbox` is now gated on a
   versioned `choir.migration_marker` object (`actor-wake-outbox:v2`): the
   scan runs once per marker version, not per boot. Marker writes last so a
   crash mid-pass reruns the idempotent pass cleanly.
2. **Stale-obligation expiry as a recorded act.** New command
   `ExpireStaleLifecyclePacket` (`internal/store/lifecycle_expire.go`) CAS-
   discharges a pending packet to `UpdateCancelled` with an
   `update_expired` lifecycle event, and marks the packet's deterministic
   wake row projected in the same commit. The migration calls it for
   unbound pending packets whose authority is closed — settled/cancelled
   work item, non-live trajectory, missing directive record.
3. **O(1) occurrence resolve.** `FindPendingLifecyclePacketsByOccurrence`
   (`internal/store/lifecycle_control_delivery.go`) hashes identity fields
   from a metadata scan and point-fetches only content-matched rows.
   `ResolvePersistentManagementLiveOccurrence` uses it — no per-row body
   fetches, no work-item hydration during match.
4. **Paced drain.** `sweepActorWakeOutbox` caps dispatches at 4 per target
   desk and 64 total per 500 ms tick.

Two adjacent defects fixed in the same change:
- `ReconcileUpdateDelivery` rejected empty `TrajectoryID`, making
  computer-scoped directives (record-native) permanently unbindable — the
  reconcile now tolerates a nil trajectory (events sequence on the packet's
  own reducer position).
- `listPendingPersistentManagementLifecycleControls` aborted the whole
  reconcile on one poisoned pending row; validation is now per-packet, and
  stale packets expire instead of starving the desk.

Deferred within slice 1 (documented residuals): the `wakes`-per-boot figure
after this deploy is expected ≈ 0 *because* the marker suppresses rescan;
true convergence proof is the 24 h acceptance window on the owner guest.
The disposable-computer SMG failure seen during this slice (Texture desk
runs exhausting the 1.2M-token budget — separate root cause, recorded in
`docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json`) is NOT this
storm and needs its own problem record if it reproduces.

## Deployed acceptance findings (2026-10-06, post-d94ce9ef/e3e96067)

Deploy + guest restart confirmed the core fix: migration minted 1870
pending wakes **once** (marker gate held across 3 later starts), expired
257 stale packets, then went quiet — the storm's per-boot re-arm loop is
dead. Two residuals surfaced on the deployed owner guest:

- **Residue wedge:** wakes for packets the occurrence finder can match
  but reconcile's pending view rejects (delivered-but-unconsumed rows,
  missing lifecycle version, missing trajectory / source-record fields)
  surfaced as `exact live Management update N is not pending` deferrals
  — families of salted `#redrive-N` wakes each deferred 20-60+ times.
  Fixed in `41822a4c`: `StalePendingLifecyclePacketReason` gained the
  missing-schema classes; directive lister gained the same stale-expire
  pass; a non-stale validation failure no longer aborts the whole
  controls pass; and `reconcile`'s exact-not-pending error is now the
  terminal sentinel `ErrPersistentManagementExactNotPending`, which
  resolve maps to a bounded discharge (recorded expire when a reason
  classifies, else consume-without-rearm).
- **Guest restart churn:** 4 SIGTERM-driven restarts during a 20-min
  window post-deploy — a guest-infra loop (updater restart-trigger or
  `bindsTo` signer flap), separate substrate, not the wake path.

The owner-guest SMG probe's final verdict is pending a re-run on
`41822a4c` (first probe ran on the residue-wedged build and timed out
with `no bound producer report observed` — evidence
`docs/evidence/smg-rlm-acceptance-owner-postfix-2026-10-06.json`).

## Deployed observation 2026-10-07 (on `19d7913e`, owner `computer-03335285`)

Post-deploy-boot behavior (fresh boot 01:06 by vmctl after reattach health
check skipped):

- `delivered-pending-runs=15` at rewarm → `=14` after Management's first
  turn, then flat through 01:20. Management run `dd52c39d` reactivated and
  is in its own tool loop (iterations 60+ observed 01:42) consuming the
  bound backlog serially — the drain is real, not silent.
- `live occurrence received → bound → terminal` cycles ran ~1/sec through
  01:20:27, then **stopped entirely** — the wake source exhausted its
  stale-supply; no new wakes mint since. The storm is not a perpetual
  motion loop on `19d7913e`; it is a bounded backlog draining.
- A queued `management-open` lifecycle control sits deferred behind the
  draining Management run (`pending lifecycle control ... queued at +7min`,
  `resident lifecycle control bind deferred`). It activates once the drain
  frees the resident slot — the deferred disposition is correct FIFO, not
  a deadlock.
- **Acceptance still owed:** (a) drain completes and `delivered-pending-runs`
  settles at a bounded floor across a stated window; (b) the floor holds
  24h; (c) the SMG probe passes on the owner (legs 1-3: management-open
  activates, delegated report binds, refusal text surfaces). Current run
  `dd52c39d` is the first non-wedged drain; watch for whether it completes
  or stalls at a deterministic residue packet.

## Evidence

- Console log receipts above (8 boots, ~2.1k re-arms each;
  `candidate-fleet-e15cb89f/console.log[.1-.4]`).
- `/api/trajectories` owner census 2026-10-06 13:25Z: 199 live trajectories,
  oldest 2026-08-16 (document-kind, legitimately live; their obligations
  re-derive as open forever).
- `/api/runs` census same window: `management:<owner>` runs passivated
  `runtime_restarted` hourly (12:43, 11:42, 10:42, 09:41, 08:40, 07:29…),
  each `request_source=update_coagent`, each `requested_by` a different
  August-era texture desk — the received→bound storm pairs as run records.
- Code: `internal/agentcore/runtime.go:2525-2616` (sweep + async migrate),
  `internal/store/lifecycle.go:965-1035` (re-arm arm), `:928-963`
  (projected-only disposal), `actorWakeOutboxFromObject` kind arms
  (`worker_update` dead-bound suppression, `work_item` v1-open mint,
  `tex_rev` owner_revision mint).
