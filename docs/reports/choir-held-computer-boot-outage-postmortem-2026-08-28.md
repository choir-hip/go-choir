# Post-Mortem: Held Computer Boot Outage and Recovery, 2026-08-27/28

<report_id: choir-held-computer-boot-outage-postmortem-2026-08-28>
<window: 2026-08-27T00:00Z - 2026-08-28T15:00Z>
<computer: computer-03335285269bdba4f94377e56879f9e6 (owner 5bd6de97-3b58-408c-bf89-c42c81b083de)>
<restored_at: 2026-08-28T14:26Z (epoch 828, commit 56fcf9f8)>
<status: restored; residual substrate risk documented>

## 0. Executive Summary

The owner's staging computer became unbootable in the browser and then, after
the first recovery phase cleared the host-side holds, entered a ~7-hour guest
death loop in which every boot of the autoputer process died 20-100 seconds
after listen. Eleven platform commits were landed between 2026-08-28T01:00Z and
13:36Z; each removed exactly one unbounded scan from `Runtime.Start`, and each
death moved to the next scan behind it. The computer was restored at epoch 828
on commit `56fcf9f8` and has been stable since 14:26Z (verified: owner API 200s,
12/12 status probes over 3 minutes, no recycle).

One substrate defect explains all ten death modes: the object graph
(`og_objects`) stores records as opaque LONGBLOB JSON bodies, and boot-path
queries filtered or materialized those bodies via `JSON_EXTRACT` table scans or
full-graph snapshot loads on a 4 GiB guest. A doom-loop amplifier made each
crash worse than the last: crash-loop passivation minted new Super tombstones
and passivated runs on every failed boot, so the scan set grew every cycle
(observed: `delivered-pending-runs=1280`, `candidates` growing 27 -> 41 -> 43).

The substrate defect is **repaired only on the Super-rewarm path**. Multiple
boot- and request-path body scans remain live (`ListPendingLifecycleUpdates`,
`listWorkerUpdateObjects`, mailbox backlog, passivated-spawned-work sweep).
Today's stable boot is survival under current data volume, not proof the class
is closed. A clustering assessment and substrate repair are required before the
next boot-path change (see Section 7).

## 1. Impact

- Owner could not boot their computer from the browser GUI from the first
  observed BIOS-hang (2026-08-27) until 2026-08-28 ~14:26Z.
- All owner API calls returned 502 `failed to resolve user autoputer` whenever
  the guest was dead or the host had marked the computer `degraded`.
- Effects mission work (supervised self-development, candidate A authorship)
  was fully blocked; the mission was interrupted to run this recovery.
- No data loss: the persistent data image was never modified or replaced; every
  fix was a code deploy plus product-path start/refresh. Nine problem receipts
  were committed before or with each fix.

## 2. Timeline (UTC)

### Phase 0 - Origin (2026-08-19/20, context)
The supervised self-development effects mission left nine undelivered CoSuper
cancel producer reports. The Super continuation repair (`9bc99f90`) and
storm-claim repair (`3654d925`, CI `32302197967`) stopped the storm, but the
storm period plus residual looping Supers (200-iteration failures) minted a
large population of passivated/tombstoned runs and worker-update packets.
Measured on 08-28: `delivered-pending-runs=1280`. This corpus became the fuel
for every later scan death.

### Phase 1 - Held computer, GUI boot hang (08-27)
- Problem doc `docs/problems/held-computer-boot-crash-loop-and-resolve-race-2026-08-28.md`
  (first observed 08-27, deployed `c4b7a9a5`): browser stuck at
  `COMPUTER BOOT IS STILL PENDING`; `bootstrap.resolve` errors 3/5, max 15s.
- Deep cause documented: maintenance hold
  (`protect-live-guest-during-hang-diagnosis`, `held_by: ox-alpha`,
  `held_at: 2026-08-27T23:34:36Z`) combined with a Texture-reconcile fatal and
  a vmctl resolve race kept the host from ever completing resolve within the
  browser's 15s probe window. The guest was in places actually healthy - the
  host could not say so.

### Phase 2 - Unhold and product-path recover (08-28 00:00-06:00)
| commit | change |
|---|---|
| `eb27cac8` | treat maintenance hold as benign on Texture reconcile |
| `34896b7e` | short-circuit held computer from resolve readiness wait |
| `3c25ea25` / `445c8fc2` | owner-scoped `recover` lifecycle action (unhold+start) in proxy + durable intent + CLI |
| `47739e0a`, `49077489` | recover proof + landing status docs |

Recover receipt `01a046ee`: epoch 804 `stopped` -> 805 `active` at 05:54:09Z;
three matching bootstrap 200s; `researcher_count=3`.

### Phase 3 - Degrade whack-a-mole (08-28 06:00-07:12)
- `held-computer-persistent-image-critically-full-2026-08-28.md`: host-side
  disk measurement reported 98.1 GiB used against a 32 GiB cap - the metric
  summed allocated blocks of the whole VM state directory and compared it to
  the `data.img` virtual size. Guest-ext4 fullness was 11.0/31.2 GiB. False
  alarm, but it fired `critical=true` into compute status.
- `held-computer-single-health-fail-degrades-2026-08-28.md`: one failed 3s
  vmctl health probe wrote `degraded` with no consecutive-failure threshold,
  502ing all API traffic against a guest that was merely booting or briefly
  stalling.
- Fixes: `fdb0759c` (detach vmctl resolve from inbound request cancel),
  `757af7e1` (keep active lookup during unhealthy-route grace; epoch 809).

### Phase 4 - The guest death loop (08-28 07:12-13:27)
Each row: deployed commit, epoch, time-from-listen to death, and the scan that
killed it. Every death is a separate problem receipt in `docs/problems/`.

| t (deploy) | commit | epoch | death | killing scan |
|---|---|---|---|---|
| 07:10 | `757af7e1` | 809 | ~100s; systemd loop ~3s downtime | per-trajectory Super reconcile; `ListAllRunsByState(passivated)` x2 per trajectory; abort on first `ErrLifecycleInvalidTransition` |
| 07:49 | `04fd704d` | 811 | ~90s | terminal-outcome audit: `ListAllRunsByState` + `ogListAllByMetadata` full materialization of every terminal run |
| 08:41 | `9023abbb` | 814 | ~85s | terminal-outcome audit: `reconcileTerminalRunOutcomes` -> `ListObjectsByMetadataPage` `JSON_EXTRACT(CAST(metadata AS JSON))` keyset scan
| 09:17 | `3a38a6e8` | 816 | ~35s | `rewarm_persistent_super`: `ListAllRunsByState(passivated)` full passivated keyset, twice |
| 10:07 | `fb1c9e93` | 817 | ~25s | `ReadObjectSnapshot` of the whole object graph during delivered-control lookup (`candidates=27`) |
| 11:18 | `15e6d6d0` | 821 | ~20s | `ListObjects(worker_update)` SELECTing every body (cap 65536) per empty-trajectory Super tombstone; gen-2 died in passivate `ListRunsByState` `JSON_EXTRACT` |
| 12:48 | `093c270a` | 824 | sweep | `sweep_open_work_item_actors open_items=20`: `ListAllPendingLifecycleUpdates` -> `ReadObjectSnapshot` up to 20x. First boot to log `rewarm dispatched` + `boot terminal outcome owner-scoped` |
| 13:21 | `82cbd2b7` | 826 | ~5 min | four `validate run=` false positives, each `ListObjectsByOwnerAndBody($.delivered_to_loop_id)`: `JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON))` over every worker_update body |
| 13:36 | `56fcf9f8` | 828 | **none** | Super rewarm switched to `CountPendingDeliveredWorkerUpdatesByRun` (canonical-index, no body) + `GetCoAgentSourcePacket` (`GetObject` by ID) |

Intervening commits `d17457f1` (10:21 CI, replace snapshot with kind listing),
`5e5e8671` (11:28 CI, delivered-controls by delivered_to_run) are folded into
the rows above; both were necessary steps but each still carried a body scan.

CI attempt notes: run `33169544732` (`59acb782`) failed on a race-shard
`TestCancelRunTrajectoryDrainsMoreThanOneActivePage` Dolt scan timeout (flake;
same test passed on adjacent shards and locally - receipted, not patched). The
commit was superseded by later green runs.

### Phase 5 - Restoration (08-28 13:59-14:40)
- 13:36:35Z push `56fcf9f8`; CI `33176174696` green 13:59:35Z; Node B deployed
  13:59:32Z.
- 14:08Z owner refresh, idempotency
  `effects-boot-probe-56fcf9f8-2026-08-28T1408Z`: epoch 827 -> **828**,
  `resulting_lifecycle_state: active`.
- Guest began serving by ~14:26Z (502 -> 404 -> 200 transition at the proxy).
- `choir computer status`: `active`, epoch 828. Stability hold 14:37-14:40Z:
  12/12 probes active, zero recycles. GUI boot works.

## 3. Root Cause

**Substrate.** `internal/objectgraph/dolt_store.go` stores each object as a
LONGBLOB JSON `body`; the only index is `(object_kind, owner_id)`. Three query
shapes on the boot path scale with corpus size and were fatal on a 4 GiB guest:

1. `ReadObjectSnapshot` - materialize every object of every kind for a scope.
2. `ListAllRunsByState` / `ogListAllByMetadata` - keyset scans that
   materialize full run bodies.
3. `ListObjectsByOwnerAndBody` - per-row
   `JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.field')` plus `SELECT
   body`.

`Runtime.Start` synchronously executes historical audits (passivation
reconcile, Super rewarm validation, terminal-outcome audit, work-item sweep)
that invoke these scans over a corpus inflated by 9 days of storm debris. Any
one of them exceeding the guest's memory/CPU budget kills the process; systemd
`Restart=on-failure` (1s) relaunches into the same scan.

**Amplifier (doom loop).** Each failed boot passivates the interrupted runs and
mints new empty-trajectory Super tombstones, so every subsequent boot scans
strictly more data than the last. Observed candidate growth 27 -> 41 -> 43;
`delivered-pending-runs=1280` constant. The loop could not self-heal and could
not be exited by waiting.

**Observability blind spot.** `Runtime.Start` logged no completion marker:
`runtime: started` never appeared in any log of the entire outage. Boot phases
were inferred from `CaptureBootLog` fragments and absence-of-progress. This
cost diagnosis time on every generation.

**User-visible amplifier.** One failed 3s health probe flipped `active` ->
`degraded` (no threshold), and the browser BIOS probe gives up at 15s; a guest
that was booting for 25-100s read as permanently dead at the surface.

## 4. What Went Well

- Problem-documentation-first held: nine problem receipts committed in the
  window, each naming mechanism, evidence table, and non-fixes before or with
  its fix.
- The scan-removal ladder produced a clean causal map: each deploy moved the
  death to exactly the next predicted scan, confirming the diagnosis each time
  without a single speculative patch.
- Product path only: no SSH mutation, no image edit, no computer replacement.
  Recover/refresh/start were all product-surface actions with receipts.
- Guardrails held under pressure: no iteration-cap raise, no computer wipe, no
  cancel of in-flight Supers, no doc push while deploy CI was running.
- A 12-agent consensus panel (10 usable outputs,
  `.agentic-consensus/agentic-consensus-20260828-100109/`) independently
  converged on the substrate diagnosis and the "no 8th symptom patch" rule
  before the successful probe.

## 5. What Went Poorly

- The outage ran ~40 hours end-to-end (GUI-hang 08-27 through restore
  08-28T14:26Z) with ~7 hours of pure crash-loop, across 11 commits.
- Root Cause Clustering per AGENTS.md (3+ same-subsystem bugs) was triggered
  days late; the first five fixes were each defensible individually but the
  clustering assessment was only forced by panel review on 08-28.
- `runtime: started` absence: a one-line log would have turned every
  "did boot complete?" question into a grep.
- The storm residue (1280 delivered-pending runs, tombstone accumulation) from
  08-19/20 was known but never pruned or bounded; it set the size of every
  later scan.
- systemd 1s restart with host grace made a dead-guest loop look `active` from
  outside for long stretches; health truth and boot truth were divorced.

## 6. Falsified Beliefs (heresy delta: discovered, none repaired by this arc)

- "The computer is sealed; live desktops should not depend on it." False in
  practice: the owner's live desktop was bound to a sealed-computer lifecycle
  the whole week.
- "A single health fail means dead" - falsified; transient stalls during boot
  are normal and must not demote routing.
- "Host disk usage reflects guest image fullness" - falsified; block
  accounting measured the VM state dir against the image virtual size.
- "One more scan removal will fix boot" - falsified as a strategy; nine scans
  stood between `listen` and `runtime: started`.

## 7. Prevention - Required Before Mission Resumption

1. **Substrate repair (red, blocking for boot-path work).** Give
   `choir.worker_update` (and the run families behind `ListAllRunsByState`) a
   durable, transactionally-maintained projection keyed by canonical ID with
   indexed scalar fields (owner, computer, target_agent_id, disposition,
   delivered_to_run_id). Queries return IDs; bodies are fetched by `GetObject`
   and remain the validation authority. Remove the remaining
   `ListObjectsByOwnerAndBody` / snapshot call sites:
   - `internal/store/lifecycle.go:1318` `ListPendingLifecycleUpdates`
   - `internal/store/lifecycle_control_delivery.go:80` `listWorkerUpdateObjects`
   - `ListLifecycleControlsDeliveredToRunPage` callers (`runtime.go:2948`,
     `super_controller.go:1010`, `super_controller.go:2434`)
   - mailbox backlog paging and `sweepPassivatedSpawnedCoagentWork`
     `ListRunsByState` (per panel review)
   Rollback: current body-scan path (kept until cutover proven).
2. **Boot completion observability.** `Runtime.Start` must log `runtime:
   started` with phase timings; an alert fires when a listen is not followed
   by `runtime: started` within N seconds.
3. **Crash-loop hygiene.** Do not mint new empty-trajectory Super tombstones
   per failed boot; cap passivation-created tombstone growth; consider a boot
   attempt counter that backs off instead of relaunching into the same scan.
4. **Health threshold.** Keep the unhealthy-route grace (`757af7e1`); add
   consecutive-failure thresholding so single-probe stalls never demote
   routing. (Partially landed; verify.)
5. **Disk truth.** Fix `LookupDataImageStats` to report guest-ext4 usage
   against the image cap, or label host-dir occupancy distinctly; `critical`
   must mean ENOSPC-risk on the guest.
6. **Corpus policy.** Decide and implement a bounded retention/pruning policy
   for delivered worker-update packets and passivated Super tombstones so scan
   sets cannot regrow unbounded (9 days of storm grew 1280 rows).
7. **Clustering assessment.** Write the `d17457f1..56fcf9f8` clustering
   assessment (AGENTS.md requirement) before the substrate repair lands; both
   reports reference the same assessment.

## 8. Residual Risk Statement

The restored boot survives because today's scan set fits. `ListPendingLifecycleUpdates`
still body-scans on the boot sweep and Texture boot reconcile; a growth in
pending packets, a Researcher open work item, or the next storm could re-enter
the death loop on a future boot. Treat a recurrence as the same substrate
defect, not a new one.

## 9. References

- Problem receipts: `docs/problems/held-computer-*2026-08-28.md` (9 files),
  `docs/problems/held-computer-boot-crash-loop-and-resolve-race-2026-08-28.md`
- Fix commits: `eb27cac8`, `34896b7e`, `3c25ea25`, `445c8fc2`, `fdb0759c`,
  `757af7e1`, `04fd704d`, `9023abbb`, `3a38a6e8`, `fb1c9e93`, `d17457f1`,
  `15e6d6d0`, `5e5e8671`, `59acb782`, `093c270a`, `82cbd2b7`, `56fcf9f8`
- CI runs (08-28): `33141788363`, `33144950319`, `33147067653`, `33149135092`,
  `33151481361`, `33154063525`, `33157175911`, `33159818337` (fail, flake),
  `33160584506`, `33163075409`, `33164995176`, `33167262447`, `33169544732`
  (fail, flake), `33171067533`, `33173268221`, `33176174696` (green, deployed)
- Consensus panel: `.agentic-consensus/agentic-consensus-20260828-100109/`
- Recovery receipts: LifecycleReceipts `01a046ee` (recover, epoch 805),
  `01a04817` (start, 821), refresh idempotency
  `effects-boot-probe-56fcf9f8-2026-08-28T1408Z` (epoch 828)
