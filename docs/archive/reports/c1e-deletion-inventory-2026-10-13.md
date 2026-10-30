# C1e inventory: work items, packets and path B (2026-10-13)

Read-only inventory on main 18d01cc0 for station C1e of the re-engineering
metamission (scope: `docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md`,
`scope_split`). Counts are matching lines per file (`grep -c`), production
code only; test files and `.claude/worktrees/` copies are excluded, and the
deletion gate must exclude those copies too.

## 1. Terms as the code uses them

- **Work item**: `types.WorkItemRecord` (`internal/types/trajectory.go:72-96`),
  objectgraph kind `choir.work_item` (`internal/objectgraph/registry.go:68`),
  store API `internal/store/trajectory.go:146-337` plus
  `GetLifecycleWorkItem`. Runs point at one through the metadata keys
  `lifecycle_work_item_id` and `work_item_ids`; settlement carries
  `RequireNoOpenWorkItems`. `internal/workitem/fingerprint.go` is called only
  from `runtime.go:2373`.
- **Packet**: the delivery object `types.CoagentSourcePacket`
  (`internal/types/evidence.go:197`), kind `choir.worker_update`
  (`registry.go:80`), with `UpdateID`, `Disposition`, `DeliveredToRunID` and
  the work-item bindings `ProducerWorkItemID`/`TargetWorkItemID`. Distinct
  from the packet *body* `types.CoagentSourcePacketPayload` (`evidence.go:60`),
  which is still the live Report content (`choir.ReportPacket`,
  `inbox_records.go:497-498`; 21 references in `rlm_reduce.go`).
- **Path B**: a document cast with no parent run retargets its engineering
  report to `texture:<doc>` and mints `work:texture-supervision:<doc>`
  (`store/engineering_assignments.go:1747-1757`, mint `:1765-1823`, callers
  `:2258`, `:2393`, `:2722`). Texture side: `texture_controller.go:71,
  593-607, 642`, `texture_agent_revision.go:238-283`, `selfdev_texture_join.go`,
  `texture_lifecycle_create.go:84-120`, `actorruntime/handler.go:602-629`. It
  also carries the recast and Jev judge (`engineering_desk.go:44-50, 197-279,
  405`; `selfdev_candidate_judgment.go`).
- **L5**: sub-researchers as runs on records. **L7**: run acceptance on the
  fold, the conversation and round state folded from inbox records
  (`store/inbox.go:723` `foldConversation`).

## 2. Where work items and packets still live

Totals: work items in 58 files (about 1,846 lines, `WorkItem|work_item|workItem`);
packets in 47 files on the broad pattern, 31 on the delivery object alone.

| Subsystem | Files (work / packet lines) | On the Autopaper path? |
| --- | --- | --- |
| Store | `lifecycle.go` (6,194 lines) 209/90; `lifecycle_control_delivery.go` 43/50; `lifecycle_control_issue.go` 31/6; `lifecycle_texture_target.go` 25; `lifecycle_commit_act.go` 12/12; `texture_turn.go` 55/7; `trajectory.go` 53; `graph_store.go` 58/26; `store.go` packet 66 | shared |
| Engineering assignments | `store/engineering_assignments.go` (2,735 lines) 53/9, binding `ParentWorkItemID`/`AssignedWorkItemID` (`types/engineering_assignment.go:80-81, 130`), authority checks `:650-1017`, report packets `:1719`; `engineering_assignment_seed.go` 7; `engineering_evidence.go` 9; `engineering_assignment_runtime.go` 24; `tools_engineering_assignment.go` 7/8 | no |
| Management | `management_controller.go` (3,293 lines) 78/81: packet control activation, recovery, directive mailbox; still runs for engineering returns and directives | mixed |
| Self-development | `selfdev_texture_join.go` (`RequireNoOpenWorkItems` `:73`); `engineering_desk.go` 19; `selfdev_candidate_judgment.go` | no |
| Research | `research_children.go` none (L5 is the all-children barrier `:136-235`); `research_checkpoint_fallback.go` 4/17 (synthesized terminal-outcome packets) | yes |
| Texture | `texture_controller.go` 34/6 (records route or work item `:394-398`); `texture_turn_runtime.go` 17; `texture_agent_revision.go` 23; `texture_handoff.go` 10; `texture_lifecycle_api.go` 19/5; `tools_worker_update.go` 28/14 plus 69 `update_coagent` | yes (dual-route residue) |
| Runtime core | `runtime.go` 78/4: spawned work items `:1290-1486`, passivation `:2481`, assigned-work sweep `:2694-2786`, terminal-outcome binding `:1664`, `:4275`; `rlm_reduce.go` 7/38 | yes |
| actorruntime | `handler.go`: producer-report occurrence wakes `:324-333`, `:714-920`; `adapter.go:259` | yes |
| yaegikernel | engineering still has `Assign` (stub, `broker.go:355-377`), `Message`, `Emit`, `Inbox`, `Updates`, `Emits`; Texture and management have `Updates` (`choir.go:294, 317-334, 806-851`) | mixed |
| Types and registry | `types/lifecycle.go` 33/8; `types/trajectory.go` 12; `objectgraph/registry.go`; `vocabmigrate/*` (historical decode) | shared |
| CLI and scripts | `cmd/choir/trace.go:655-676`; `scripts/reeng_invariants.mjs` 42 (I2/I3 read work items and updates; I7 `:1057-1083` asserts none on a records route); `scripts/reeng_tracer.mjs:841-846` | instruments |
| Frontend | none | — |

## 3. Records equivalents that exist

`store/inbox.go`: `CommitInboxRecords`, `InboxHead`, `ReaderCursor`,
`UnreadFor`, `ConversationState`/`foldConversation`, `HypothesisStates`,
`OpenRounds`, `RebuildInboxHeads`/`VerifyInboxHeads`.
`agentcore/inbox_records.go`: `recordsDesk` covers Texture, research and
management; `recordsRoute` has no engineering branch; `commitRecordsRoute`
refuses engineering casts at `:360`. Kernel `choir.Records()`; `Cast`, `Ask`,
`Note`, `Report`, `ReportPacket` on all desks. `capsule.Types.Records`
exists (`capsule/types.go:131`) and `desk_session_worker.go:300` passes
records into the frame. The materializer already writes an awaiting-decision
record (`self_development_materializer.go`).

## 4. Gates

- Heresy Detector (`ci.yml`): `scripts/check-heresies.sh --fail-on-regression`
  over `docs/heresy-detectors.md`; rows marked `enforce: zero` fail CI. New
  rows need no script change: the natural home for the C1e gate.
- Vocabulary Gates: writer purity, decode roots, test hang guards; none
  touches work items or packets.
- The gate cut1 already specifies: `WorkItem|work_item`, `CoagentSourcePacket`,
  `update_coagent`, `Assign`, `choir.Message`, `choir.Emit`,
  `textureSupervision`, and the retired wire and budget names (the wire names
  already have zero hits). Trap: `CoagentSourcePacket` also matches the live
  `CoagentSourcePacketPayload`; rename the Report body first or match
  `\bCoagentSourcePacket\b`.
- Runtime counterpart: I7 in `scripts/reeng_invariants.mjs`.

## 5. Dependency order

1. Engineering conversation on records: engineering in `recordsDesk` and
   `recordsRoute` (management to engineering), capsule `Records`, drop
   `Inbox`/`Updates`/`Emits`/`Assign`/`Message`/`Emit` from engineering,
   remove the refusal at `inbox_records.go:360`, engineering's Report to
   management instead of `buildEngineeringReturnPacket`.
2. Records-authorized assignments: conversation and round authority instead
   of `ParentWorkItemID`/`AssignedWorkItemID`; remove
   `RequireNoOpenWorkItems` (seed `:36`, `selfdev_texture_join.go:73`).
3. Delete path B (retarget, supervision mint, `selfdev_texture_join.go`,
   recast and judge, Texture exclusions, the handler entry, the path-B
   branch in `run_acceptance.go:166-188`).
4. Management Verify (today only in the verifier slot, `choir.go:423`), bound
   to the operation id and bundle digest the engineering Report names; then
   the self-development round trip on records.
5. L5 sub-researchers as runs (independent; can run beside the deletions).
6. L7 run acceptance on the fold (after path B).
7. Autopaper residue: Texture's work-item fallbacks, `Updates` and the
   terminal-outcome packets, spawned work items and the assigned-work sweep,
   the actorruntime occurrence wakes, management's packet activation. This
   step touches the live Autopaper path and needs its own tracer runs.
8. Store and types last (`lifecycle*.go`, the trajectory work-item API, the
   packet tables, the types, the registry kinds, `cmd/choir/trace.go`, the
   tracer and checker inputs), then enforce the gate.

Riskiest coupling: `store/lifecycle.go` and `management_controller.go` are
shared with the live Autopaper path (Texture and management still receive
terminal outcomes as packets; actorruntime wakes Texture on producer-report
occurrences). Second risk: the I2/I3 checkers read work items and packets;
re-express them on records before the tables go, or C1e loses its own
acceptance instrument.
