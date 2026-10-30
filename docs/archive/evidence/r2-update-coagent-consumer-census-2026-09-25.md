# R2 Consumer Census — `update_coagent` / CoagentSourcePacket surface

Date: 2026-09-25 · Mission R2 (commitment-ledger-desk-carrier) `delta_o` artifact.
Scope: every non-test file that references the packet/act surface
(`CoagentSourcePacket`, `"update_coagent"`, `worker_updates_*`,
`request_source`). 40 files.

Classification key:
- **produce** — builds/stages a packet or packet metadata.
- **register** — registers the `update_coagent` tool into a registry.
- **meta** — reads `request_source`/`worker_updates_*` metadata to gate
  policy/authority.
- **persist** — stores/replays packet-bearing objects.

Per-file disposition for the semantic-act cutover (Report/Cast carry the act;
the packet schema survives as Report's body; `update_coagent` survives only on
processor/reconciler until their phase):

| file | role | disposition |
|---|---|---|
| types/evidence.go | packet type + schema authority (CoagentSourcePacket, Payload, SchemaV1) | KEEP — schema becomes the Report/act body |
| types/engineering_assignment.go, types/lifecycle.go | assignment/lifecycle types packets ride on | KEEP (independent of tool) |
| agentcore/tools_worker_update.go | registers `update_coagent` + payload schema | MIGRATE — split schema from tool reg; drop tool reg for the 4 desks |
| agentcore/tools_engineering_assignment.go | registers assignment tools; produces packets | MIGRATE — registry surface → verb surface |
| toolregistry/batch_executor.go | sequential-execution list incl. update_coagent | KEEP — processor/reconciler tool path |
| agentcore/coagent_update_packet.go | terminal-outcome → packet projection | KEEP — feeds Report/act bodies |
| agentcore/rlm_reduce.go | castStagedIntent → envelope delivery | KEEP — the Cast carrier |
| agentcore/management_controller.go | produces + reads request_source meta | MIGRATE — management desk → verbs |
| agentcore/runtime.go, api.go, api_producer_reports.go, texture_lifecycle_api.go, texture_tool_api.go, research_checkpoint_fallback.go, wire_reconciler_debounce.go | meta readers / packet producers | KEEP — meta readers unchanged; tool-facing producers migrate with the desk |
| researchtools/researchtools.go | registers research tools (incl. update_coagent) | MIGRATE — research desk → verbs |
| store/* (store.go, lifecycle*.go, texture_turn.go, engineering_assignments.go, graph_store.go) | persistence of packet-bearing objects + meta gates | KEEP — persistence unaffected; obligation shape evolves |
| textureowner/* (handler, texture, texture_agent_revision, texture_controller, texture_evidence_sources, texture_proposals, texture_revision_metadata, texture_turn_runtime, texture_workflow_verifier, tool_loop_policy, tools_texture, api_texture_prompt_eval, value_helpers) | register texture tools; produce/read worker_updates_* + request_source | MIGRATE — texture desk → verbs; keep metadata ingestion continuous during migration (worker_updates_* must not break mid-flight) |
| actorruntime/adapter.go, handler.go | adapter reads request_source meta | KEEP — delivery substrate, meta gate unchanged |

## Findings

- Packet **schema** and **projection** (types/evidence.go,
  coagent_update_packet.go) are KEEP — they become the typed act body; the
  tool registration is the only thing retired on the four desks.
- **MIGRATE** set = the per-desk tool registration + the metadata
  read/write that distinguishes desk-authored acts: agentcore
  {tools_worker_update, tools_engineering_assignment, management_controller},
  researchtools, textureowner (tool_loop_policy + the tool registration +
  revision metadata producers).
- **KEEP** set = the store/persistence layer, the delivery substrate
  (rlm_reduce, actorruntime), the batch executor (processor/reconciler tool
  path), and the packet schema authority.
- Texture's `worker_updates_*` metadata path is the deepest dependency
  (14 refs in texture_revision_metadata.go) — migrate last within texture,
  preserving ingestion continuity.
- No standalone *delete* candidates: nothing references the surface that a
  verb does not replace. The deletion is only the per-desk `update_coagent`
  registry entry, not the packet contract.
EOF
