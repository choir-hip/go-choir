# s0m: record-native producer report dies on desk-name ToDesk — 2026-10-02

## Finding

The RN3b record-native report path
(`rlmCallReduction.commitLifecycleReportActIntent`,
internal/agentcore/rlm_reduce.go) passes `StagedIntent.ToDesk` verbatim into
`resolveCoagentUpdateAuthorityWithStore` as `explicitTargetAgentID`. That
resolver's contract is an exact agent id (`texture:<doc_id>`,
`management:<owner>`, `engineering:<uuid>`); a desk name like `texture` fails
`GetAgentByScope` and the intent commit aborts before any record mints.

Production cells address reports by desk name — the overlays document
`choir.Report(toDesk, ...)` / `choir.ReportPacket(toDesk, ...)` with `toDesk`
values like `texture`, and the lifecycle validator
(`validateLifecycleCoagentUpdateAuthority`) independently pins the target to
`currentTextureAgentID(trajectory.SubjectRefs["doc_id"])`. The reducer's
desk-name translation exists for directives (`directiveTargetAgentID`) but the
report path never applied it.

## Evidence

Owner computer `computer-03335285269bdba4f94377e56879f9e6` on
`267998a6` (RN3b + overlay fix), staging acceptance probe
`scripts/s0m_report_acceptance_probe.mjs`:

- Trajectory `16059801-00ea-592d-acc5-fa0720d085b5` events:
  `texture_turn_committed` → `work_opened` → `control_queued`
  (update `1bfb7db9-07f5-4136-9d05-7a8d229a91d7`) → `control_delivered`
  bound to activation → `unbind-stranded-control` after the run ended.
- Bound research run `44267046-a732-4eb8-874d-eff367644b6a`
  (`lifecycle_texture_control`, `lifecycle_control_bindings` to the control)
  completed with empty result and **no** `update_queued` producer-report
  event; no commitment record minted for the report.
- Same shape on the earlier probe: report intents never commit —
  `resolveCoagentUpdateAuthorityWithStore` rejects the desk-name target
  upstream of `CommitLifecycleAct`, and the legacy envelope path is
  unreachable for lifecycle-bound callers (sentinel `errLifecycleActLegacyCaller`
  is never returned for them).

## Impact

Every lifecycle-bound producer report (`choir.Report`/`choir.ReportPacket`
with `toDesk` as a desk name, which is the only documented form) fails on
`staging` — the record-native RN3b path is dead for its primary consumer.

## Repair boundary

This doc only names the problem and the failing contract. The repair
(desk-name → target agent resolution through the caller run's trajectory
document for texture, persistent management id for management, in
`commitLifecycleReportActIntent` before authority resolution) is a separate
change.
