# Object-Graph Body-Scan Root Cause Clustering — 2026-08-28

- Date: 2026-08-28
- Mutation class: Red (assessment; repairs are red)
- Trigger: AGENTS.md Root Cause Clustering rule (3+ same-subsystem bugs in one week)
- Problem cluster: `docs/problems/held-computer-*2026-08-28.md` (9 receipts) + `docs/problems/held-computer-boot-crash-loop-and-resolve-race-2026-08-28.md`
- Post-mortem: `docs/reports/choir-held-computer-boot-outage-postmortem-2026-08-28.md`

## The Cluster

Between 2026-08-28T07:12Z and 13:27Z the retained staging computer crashed on
every boot, each time in a different call site, each receipted as its own
problem:

| # | receipt (suffix) | killing call site |
|---|---|---|
| 1 | super-rewarm-scan-crash | per-trajectory `ListAllRunsByState(passivated)` |
| 2 | boot-terminal-outcome-scan-crash | `reconcileTerminalRunOutcomes` keyset materialization |
| 3 | super-rewarm-listall-passivated-crash | rewarm passivated keyset, twice |
| 4 | super-rewarm-snapshot-crash | `ReadObjectSnapshot` full-graph load |
| 5 | super-rewarm-worker-update-scan-crash | `ListObjects(worker_update)` all bodies |
| 6 | boot-work-item-sweep-snapshot-crash | `ReadObjectSnapshot` x20 in work-item sweep |
| 7 | super-rewarm-validate-body-scan-crash | `ListObjectsByOwnerAndBody` per-tombstone `JSON_EXTRACT` |

## Common Cause

One substrate defect, seven symptoms:

**`og_objects` stores records as opaque LONGBLOB JSON bodies with only a
`(object_kind, owner_id)` index. Every query shape that filters or audits by
record content materializes or parses bodies at O(corpus) cost, and
`Runtime.Start` runs several such queries synchronously on a 4 GiB guest.**

Three concrete anti-patterns recurred across the seven receipts:

1. `ReadObjectSnapshot` — load every object of every kind in scope into Go.
2. `ListAllRunsByState` / `ogListAllByMetadata` — keyset scans holding full
   bodies (or `JSON_EXTRACT(CAST(metadata AS JSON))` per row).
3. `ListObjectsByOwnerAndBody` — per-row
   `JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON))` plus `SELECT body`.

The amplifier that made this a death loop rather than a slow boot: each failed
boot passivates interrupted runs and mints new empty-trajectory Super
tombstones, so every retry scans strictly more rows than the last.

## Existing Replacement (proven, partially wired)

`56fcf9f8` introduced the correct pattern on the Super-rewarm path and it is
proven on the live corpus (the 08-28 boot that survived):

- `ListJSONBodyFieldsByKindOwner` — extracts only named JSON fields plus
  `canonical_id, computer_id`; no `SELECT body`, no LONGBLOB materialization.
- Callers filter candidate canonical IDs in Go, then `GetObject` (single-row
  point read) only the matches; the decoded body remains the validation
  authority.

**This replacement is not yet wired into the remaining call sites.** Per the
clustering rule, the next action is substrate-level connection of the proven
pattern, not a ninth symptom patch:

1. `internal/store/lifecycle.go` `ListPendingLifecycleUpdates` — boot-path
   pending-mailbox selection; body scan live (`$.target_agent_id`,
   `$.disposition`, `$.delivered_to_loop_id` empty).
2. `internal/store/lifecycle_control_delivery.go` `listWorkerUpdateObjects` —
   delivered-controls page used by Super rewarm validation and terminal
   outcome enqueue; body scan live (`$.delivered_to_loop_id`).

After both land, no `choir.worker_update` query may use
`ListObjectsByOwnerAndBody`/`ReadObjectSnapshot`; any future call site must use
field-extract + `GetObject`.

## Scope Boundaries

- In scope: the two worker-update call sites above (the boot path that killed
  epochs 811-826 and remains live on the restored boot).
- Out of scope (separate receipts/follow-ups, same substrate family):
  `ListAllRunsByState`/`ogListAllByMetadata` run-family scans, mailbox backlog
  paging (`ListCoagentMailboxBacklogAll`), `sweepPassivatedSpawnedCoagentWork`,
  and true generated-column indexes (end state beyond field-extract).
- Non-fixes (unchanged): no SSH mutation, no iteration-cap change, no computer
  wipe, no boot-time backfill of unbounded scans.

## Verification Contract

- Existing behavioral tests for pending selection and delivered-controls pages
  must pass unchanged (`internal/store`, `internal/agentcore` shards).
- Staging acceptance: owner refresh boots the guest onto the repair commit;
  `runtime: started` visible in boot logs; boot stable >= 3 minutes; owner API
  200s; GUI bootstrap 200.
- Rollback: revert commit restores the body-scan path (no schema/migration
  involved; both paths read the same canonical objects).
