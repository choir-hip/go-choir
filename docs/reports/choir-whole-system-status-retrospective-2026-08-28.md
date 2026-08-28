# Whole-System Status Retrospective, 2026-08-28

<report_id: choir-whole-system-status-retrospective-2026-08-28>
<as_of: 2026-08-28T15:00Z>
<staging: https://choir.news, deployed_commit 56fcf9f894cca7758754cb80a0a4c677c6a1a39c (deployed 13:59:32Z)>
<computer: computer-03335285269bdba4f94377e56879f9e6 - active, epoch 828>

## 1. One-Paragraph State

Staging is healthy and the owner computer boots again: GUI bootstrap succeeds,
owner APIs return 200, and the guest has been continuously `active` on epoch
828 since ~14:26Z on commit `56fcf9f8`. The 40-hour held-computer outage is
closed (see companion post-mortem
`choir-held-computer-boot-outage-postmortem-2026-08-28.md`). The supervised
self-development effects mission remains **interrupted** at the same step it
paused: capsule-authored candidate A has not been written; the pre-A checkpoint
fence (`99949fe2...`) is unchanged. The dominant open engineering risk is the
unrepaired half of the object-graph body-scan substrate.

## 2. Surface Status

| surface | state | evidence |
|---|---|---|
| Proxy / vmctl (choir.news) | ok | `/health` `vmctl_status ok`, routing enabled |
| Node B deploy | `56fcf9f8` @ 13:59:32Z (CI `33176174696`) | `/health build` |
| Owner computer | `active`, epoch 828, GUI boot works | `choir computer status`; 12/12 stability probes 14:37-14:40Z |
| Owner API | 200 (`computer status`, refresh, bootstrap) | product-path probes this afternoon |
| Repo HEAD | `56fcf9f8` = staging = guest | `git log`, `/health build.deployed_commit` |
| CI | green at HEAD; two flake failures today (receipted, not patched) | Section 4 |

## 3. Mission Ledger

| mission | state |
|---|---|
| Private Go Actor Kernel | completed, owner-ratified (`6cdbd3e6`) |
| Genesis Computer Surface | complete with staging E2E receipts (`b5390b8d`, `53f80af4`) |
| Account recovery (held computer) | closed 08-27 (`76497032`, break-glass disclosed `e49bcb65`) |
| Held-computer boot outage | closed 08-28 14:26Z; post-mortem committed |
| Supervised self-development effects (`docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`) | **interrupted / resuming** - see Section 6 |

## 4. CI and Flakes (last 24h)

Green at HEAD. Failures in the window were substrate flakes, receipted and not
patched:

- `33159818337` (`fb1c9e93`) failed; force-deploy `33160584506` succeeded.
- `33169544732` (`59acb782`) failed on race shard 4
  `TestCancelRunTrajectoryDrainsMoreThanOneActivePage` (Dolt scan timeout);
  the same test passed on adjacent shards and locally.
- Two `concurrency` cancels (`33148725175`, `33146514765`, `33144207842`,
  `33143458281`) were push-superseded runs, not failures.
- Known CI-substrate issues carried in the landing definition: Heresy Detector
  `apt-get install ripgrep` stall (08-19, run `32293816782`) and the race-shard
  Dolt timeout class. Neither is a Choir product heresy.

## 5. Problem Registry Delta (this window)

Ten problem receipts written 08-27/28 (all `docs/problems/`), clustered into
four families:

1. **Held-computer surface family** (08-27): boot crash loop + resolve race;
   deep cause = live desktop bound to a sealed-computer lifecycle under
   maintenance hold.
2. **Health-truth family** (08-28 morning): single-probe degrade; host-vs-guest
   disk measurement mismatch.
3. **Boot-scan crash family** (08-28, 7 receipts): every `Runtime.Start` phase
   that materialized LONGBLOB bodies or full-graph snapshots killed the 4 GiB
   guest; death moved scan-to-scan as each was removed.
4. **Substrate diagnosis (open)**: one root cause under family 3;
   `d17457f1..56fcf9f8` is a Root Cause Clustering case per AGENTS.md. The
   clustering assessment and substrate repair (worker_update/run projection +
   `GetObject` reads, removal of remaining `ListObjectsByOwnerAndBody` call
   sites) are the required next platform change before further boot-path work.

## 6. Effects Mission - Where It Pauses and Resumes

Pre-interruption state (verified in repo history and staging):

- Family A computer-scoping, residue import, and replay `eligible: true` are
  published; pre-A checkpoint `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`
  is the restore fence (epoch 324 era; unchanged since).
- CoSuper `assignment-97191e37` was cancelled with nine undelivered producer
  reports; the Super continuation storm was stopped by `3654d925` (claim
  semantics), proven live 08-19 (Super `bab919a0` claimed 9 IDs, no restorm).
- Later super-controller work (08-27/28) and the outage consumed the interval.
  Replacement Supers (`e4141127`, `F515dd0f`) died on `max_tokens`/hang; the
  nine cancel reports remain undelivered pending a bound replacement CoSuper.

Resume order (unchanged from the definition; do not shrink criteria):

1. Land the substrate repair + clustering assessment (Section 5) - it is now
   also a boot-stability prerequisite for capsule work.
2. Update the definition `now.next_action` to reflect the outage receipts and
   the resume point; commit docs.
3. Resume candidate A: bind a fresh CoSuper assignment on the restored
   computer, author candidate change A inside the capsule with the
   pre-declared foundation defect, freeze, five capsule-bound bundle refs,
   qualified consensus, promote, play/falsify/supersede, restore-to-fence
   proof per the definition.
4. Effects remain OFF until rehearsal gates pass. No live send, no mail, no
   restore outside the definition's fence semantics.

## 7. Panel Consensus Carried Forward

Convergent 12-agent panel, 10 usable outputs
(`.agentic-consensus/agentic-consensus-20260828-100109/`):

- Consensus: one substrate defect (object-graph body scans) explains all seven
  boot-scan crashes; no further caller-specific patches; route
  `ListPendingLifecycleUpdates` and remaining call sites through the
  canonical-index (`ListJSONBodyFieldsByKindOwner` + `GetObject`) pattern now
  proven on the Super-rewarm path.
- Falsified live: the panel's majority prediction that the `56fcf9f8` boot
  would still die at the work-item sweep. It survived. Survival is credited to
  current data volume, not substrate health (minority view `luna`/`opencode`
  held).
- Unique findings adopted: `ListJSONBodyFieldsByKindOwner` is not a true index
  (still parses bodies; generated columns are the end state); mailbox backlog
  paging and `sweepPassivatedSpawnedCoagentWork` `ListRunsByState` are
  additional live scans; boot may "pass" without exercising the Researcher
  sweep branch, so stability evidence must name the branches exercised.

## 8. Risk Register (top)

| risk | severity | mitigation |
|---|---|---|
| Remaining boot-path body scans re-enter the death loop on data growth | high | Section 5 substrate repair before more boot-path work |
| Storm-debris corpus (1280 delivered-pending runs, tombstone growth) | high | retention/pruning policy; clustering assessment |
| `Runtime.Start` observability (no completion marker historically) | medium | phase timing + `runtime: started` alert (in repair scope) |
| CI race-shard Dolt timeout + heresy install stall | medium | retry/force-deploy playbook; CI substrate, not product |
| Guest 4 GiB headroom vs corpus growth | medium | corpus policy; consider budget SLO per boot phase |
| Effects mission deferral compounding (checkpoint fence now 9 days old) | medium | resume order in Section 6; fence re-validation before capsule work |

## 9. Decision Requests for the Owner

1. Approve the substrate repair as the next red change (with rollback =
   current body-scan path) before candidate A capsule authorship.
2. Approve corpus retention policy direction for delivered worker-update
   packets and passivated Super tombstones.
3. Confirm the effects mission resume point (Section 6) unchanged.
