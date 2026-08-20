# Staging Evidence: Stale Historical Blocked Run Prevents Super Reconciliation

- Date: 2026-08-20
- Mutation Class: Red
- Computer ID: `computer-03335285269bdba4f94377e56879f9e6`
- Affected Subsystem: `internal/agentcore/super_controller.go` (`reconcilePersistentSuperActor`), `internal/store/store.go` (`latestLifecycleRunByAgent`)

## Summary

Following the deployment of `3620b336` (Texture rewake narrow fix) and `b4b2cbce` (Solitaire candidate A), live probing on staging computer `computer-03335285269bdba4f94377e56879f9e6` (epoch 340-342) showed that new operations (`selfdev-d21a7fcb5ad2c4826f6397cd860a8eac` on trajectory `d8ccd11b`) successfully opened work item `c1539f6d` and queued lifecycle control `1860565c`, but no new Persistent Super run was ever started by `reconcilePersistentSuperActor`.

Investigation of the live database via `/api/runs` revealed the exact root cause:
1. On 2026-08-19 at 13:11:40Z, run `d3d05452-ef55-4e28-ac89-1029bceb32d1` (agent `super:5bd6de97-3b58-408c-bf89-c42c81b083de`) entered state `blocked` due to a transient provider gateway failure.
2. Later on 2026-08-19 (17:45Z to 23:40Z), numerous newer Super runs were created and executed (`f009f383` completed, `b57705fd`..`f515dd0f` failed).
3. `reconcilePersistentSuperActor` checks if Super is currently blocked via `rt.latestActiveRunByAgent(ctx, ownerID, agentID)`.
4. `latestActiveRunByAgent` calls `store.GetLatestActiveLifecycleRunByAgent`, which scans the object graph with `match = types.RunState.Active` (`pending`, `running`, `blocked`).
5. Because all the newer runs (`f515dd0f`, `bab919a0`, etc.) are in state `failed` (terminal), they do NOT match `RunState.Active`.
6. Therefore, the query returns `d3d05452` from 13:11:40Z as the "latest active" run.
7. `reconcilePersistentSuperActor` evaluates `if active.State == types.RunBlocked { return &active, nil }` and exits early, treating the persistent Super as blocked by an obsolete 7-hour-old run that had already been superseded by over 15 subsequent runs.

## Live Evidence
- `d3d05452-ef55-4e28-ac89-1029bceb32d1`: `state: "blocked"`, `agent_id: "super:5bd6de97-3b58-408c-bf89-c42c81b083de"`, `created_at: "2026-08-19T13:11:40.267Z"`
- `f515dd0f-ae2a-4bf4-9a64-4cbbf9f6ea02`: `state: "failed"`, `agent_id: "super:5bd6de97-3b58-408c-bf89-c42c81b083de"`, `created_at: "2026-08-19T23:19:19Z"`
- `d8ccd11b-6691-5f4a-9cfe-98c688b38d60`: `work_item_id: "c1539f6d"`, `control_id: "1860565c"`, `kind: "control_queued"`, `status: "open"`, not delivered because `reconcilePersistentSuperActor` short-circuits on `d3d05452`.

## Repair Strategy
`latestActiveRunByAgent` must ensure that the found active/blocked run is actually the latest overall run for the agent. If any terminal run exists with `CreatedAt` or `UpdatedAt` after the blocked run, the blocked run is obsolete/superseded and must not prevent reconciliation of new work.
