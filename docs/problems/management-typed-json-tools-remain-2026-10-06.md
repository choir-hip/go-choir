# Management desk still carries typed JSON tools — RLM cutover incomplete

## Evidence

Owner observation 2026-10-06: a `report_to_texture` JSON tool call was
visible on the live staging surface. Source inspection confirms:

- `internal/agentcore/tool_profiles.go` `buildDeskCellRegistry` registers
  `desk_go_eval` for every desk, then adds — for **management only** —
  `report_to_texture` (`RegisterPersistentManagementReportTools`,
  `tools_engineering_assignment.go:132`) and
  `cancel_co_super_assignment` (`RegisterAssignedEngineeringTools`,
  `tools_engineering_assignment.go:46`) when a capsule executor exists.
- The research desk is sealed at `{desk_go_eval}` (SR station); texture is
  sealed at `{desk_go_eval}`; **management is not**.
- Deployed evidence (2026-10-06, guest build 4467a4fb): research schema =
  `["desk_go_eval"]`. Management's schema was not yet probed but carries
  3 registered tools per the code path above (desk_go_eval,
  report_to_texture, cancel_co_super_assignment).
- Prompt audit: `research.yaml`/`rlm_research_runtime.yaml` are RLM-only
  post-12d3adc0 (bidirectional parity verified). Management/engineering
  prompts still teach typed-tool semantics where present.

## Why this is a defect

- Doctrine: semantic acts ride the commitment ledger through in-cell
  `choir.*` verbs with precommitment records — not ad-hoc JSON tool calls.
  `choir.Report`, `choir.ReportPacket`, `choir.Resolve`, `choir.Precommit`,
  `choir.CancelAct`, `choir.Escalate` already export for management
  (`deskModuleSets["management"]`, `internal/yaegikernel/choir.go:177`).
- The typed tools duplicate in-cell capability with worse provenance: the
  JSON path bypasses the cell's sealed source/digest identity and the
  egress ledger's one-call metering.
- `report_to_texture` does carry one real capability the bare verbs lack:
  bound lifecycle-control validation (exact delivered control binding,
  work_disposition). That validation must move inside the host handler
  behind a `choir.*` verb (e.g. `Report`/`ReportPacket` with the persistent
  management binding check), not be deleted.

## Owner directive (2026-10-06)

"Get rid of all of these adhoc json tool calls everywhere" + "make sure
the prompts are all updated for full rlm only".

## Scope decision

SR station is closing concurrently (research cutover). This cutover is
management-desk scope: fold as SR remainder if the station file allows, or
execute as an immediate follow-on mutation before SM/SC — owner directive
makes it in-flight work regardless of station bookkeeping.

## Done when

- `buildDeskCellRegistry` registers exactly `desk_go_eval` for management;
  `report_to_texture`/`cancel_co_super_assignment` deleted end to end;
  their validations live behind choir.* verbs.
- All desk prompts teach cell + choir.* verbs only; no typed-tool names.
- Deployed schema leg: GET /api/prompts/management shows
  `tools: [{desk_go_eval}]` on the live guest.
- Bidirectional parity test covers management/engineering prompt↔exports.
