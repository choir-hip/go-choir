# Dead-code census: agent-facing surfaces and configuration

Agent 5 of 5. Read-only: nothing in the repo was edited, no git state touched. Repo state: `main` at c30147a6 (clean).
Date 2026-10-12. Scope: desk verbs and tools, prompts, profiles/kinds/metadata, env vars and config, Nix.

## Method

- Verbs: read `internal/yaegikernel/choir.go` (`deskModuleSets`, `ChoirExports`), `surface.go` (`choirFunctionDocs`), `broker.go`, the reducer branches in `internal/agentcore/rlm_reduce.go`, and every hand-written prompt (`internal/promptstore/defaults/*.yaml`, `internal/runtimeprompts/overlays/*.yaml`, `internal/textureprompts/**`), per desk, with a bash loop that tests each exported verb against that desk's prompts.
- Typed tools: traced `InstallDefaultAgentTools` -> `buildRegistryForRole` / `buildDeskCellRegistry` -> `toolRegistryForRun` to see which registry a run actually gets, and every `Register*` caller.
- Prompts: read all 17 prompt files and the Go-string prompts (`tool_profiles.go`, `runtime.go` wake text, `run_memory.go`, `skill_context.go`), then grepped retired names.
- Kinds / metadata: for every `types.EventKind`, `LifecycleEventKind`, `runMetadata*` key and `request_intent` / `request_source` value, counted writers versus readers outside tests.
- Env: extracted every `os.Getenv`/`envOr`/`boolOr` key in Go (266), every var assigned in `nix/`, `flake.nix`, `start-services.sh`, `.envrc`, `.github` (232), and diffed both ways, then re-checked each hit by hand (vmctl also injects env at `internal/vmctl/handlers.go:1499`, and the guest receives env through kernel cmdline lines in `nix/autoputer-vm.nix:395-470`).
- Cross-checked against `deadcode -test=false` output (other agent's `dc-notest.txt`) and against `docs/archive/reeng-studies-2026-10-12/G-affordance-audit-runtime-pass.md` (Study G). Where Study G already established a finding I re-verified the cite against current source and say so.
- "Scheduled" = named in `docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md` (Cut 1) or `...cut2-evidence-to-publication...` (Cut 2). Note Cut 1 line 237 says spawn_agent is "registered only for the conductor and the base engineering registry"; in fact `RegisterSpawnTool` returns nil when `AllowedSpawnTargets` is empty (`spawn_tool.go:21`), and Engineering's policy has none, so the conductor is the only live registration.
- Tags: **[V]** verified by reading the cited lines now. **[I]** inferred (no production trace read). Per CLAUDE.md "review the traces before attributing behavior", every claim about what a model does or does not do is a hypothesis; the claims below are about code reachability, not model behavior.

## Totals

- 5 sections, 40 items (12 verbs/tools, 10 prompts, 5 profile/kind/metadata, 7 env/config, 6 Nix).
- Already scheduled in Cut 1 or Cut 2: 7 items fully (1.3, 1.4, 1.5, 2.3, 2.4, 2.5, 2.7), 6 partly (1.7, 1.8, 1.9, 2.6, 3.1, 4.6). Not scheduled: 27.
- Rough removable size (excluding what Cut 1/2 already delete): about 5,000 lines of Go (`internal/qdrant` 2,006 and `agentcore/qdrant_*` 255; `internal/trace` store+query about 690; `tools_coding.go` 275; capsule lifecycle tools about 170; `tools_coagent.go` required-child wait about 175; yaegikernel unreachable files about 960), one Nix service (qdrant), three unread config fields, about 25 env vars, 4 prompt defects that actively instruct a model to call nothing.
- Highest-value finds, in order: (1) the whole typed-tool registry for management/texture/email is built then shadowed; (2) four conductor capsule tools have no registrar; (3) the run-memory compactor tells every desk to call `get_run_memory_entry`, which no desk has; (4) the skill-context sentence names a deleted skill; (5) the Qdrant/Ollama dedup chain, a Nix service and three config fields with zero production caller; (6) the trace persistence flag is never set, so the Dolt trace store is off in production; (7) `required_child_runs` / `required_texture_revisions` gate has no writer.

---

# Section 1. Desk verbs, tools, modules

## 1.1 Typed-tool registries built for management, texture, email and research, then shadowed
- **Location:** `internal/agentcore/tool_profiles.go:404-445` (`buildRegistryForRole`), `:447-545` (`InstallDefaultAgentTools`); `tools_coding.go` (275 lines); `tools_evidence.go:17-66` registrars; `internal/researchtools/researchtools.go:248-265` (`Register`); `internal/modelpolicy/tools_model_verify.go` (70 lines); `agentprofile.go` fields `AllowReadOnlyFiles/AllowResearchTools/AllowEvidenceTools/AllowMemoryTools/AllowModelDiagnosticTools`.
- **What it did:** In the tool-loop era each profile got typed JSON tools: `read_file/glob/grep`, nine research tools (`web_search`, `fetch_url`, `source_search`, `import_*`, `read_content_item*`, `search_wire_corpus`), `save/read/list_evidence`, `get_run_memory_entry`, `verify_model_capability`, `cancel_agent`. `agentprofile.PolicyFor` decides which a profile gets.
- **Why dead [V]:** `InstallDefaultAgentTools` builds management/texture/email/conductor registries from policy (`:466-520`) and then overwrites `toolProfiles[management|texture|research]` with the `desk_go_eval`-only registry (`:541-543`; `deskCarrierLive`, `:359-370`, is true for all three). `toolRegistryForRun` (`:547-558`) returns `toolProfiles[profile]`; the policy-built management registry survives only as `rt.toolRegistry`, the fallback for a canonical profile with no entry (`verifier`, `verifier_multimodal`, `processor`, `reconciler`). No run is created with those profiles (grep of `AgentProfile:` finds none), so the fallback is unreachable in practice, but it would hand a full typed-tool authority to such a run if one existed. Texture's registry is built with `AllowMemoryTools` but is overwritten, and the `agentprofile.go:60-62` comment ("keeps run-memory retrieval so it can recover its own compacted context") is false: Texture's module set has no `RunMemoryEntry`.
- **Live part to keep:** the tool constructors in `researchtools` and `tools_evidence.go` (`newSaveEvidenceTool` etc.) are reused by the cell verbs through `Dependencies.egressToolTable` (`researchtools/host_egress.go:67-79`) and the evidence table (`tools_evidence.go:46-52`). Only the registrations, `read_file/glob/grep`, the JSON schemas/descriptions, and the policy flags are dead.
- **Citers:** `RegisterReadOnlyFileTools` (only `buildRegistryForRole`), `RegisterEvidenceTools`, `RegisterRunMemoryTools`, `RegisterModelDiagnosticTools`, `researchtools.Register`, settings page `rolePolicyFromSpec` (`prompts.go:62-75`) which shows these flags to the owner (Study G M16).
- **Origin:** typed tools since 2026-04/05; research typed surface deleted 2026-10-05 (SR, `desk-surface-cleanup-2026-10-05`), management SMG 2026-10-06, texture R3d 2026-09-26 (`3ed94220`). The registrations were not removed.
- **Scheduled:** no (Cut 1 (7) rewrites prompts but never mentions the registries).
- **Proposal:** remove `read_file/glob/grep`, the Register* calls in `buildRegistryForRole`, `verify_model_capability` registration, the `Allow*Tools` flags and the owner-visible role policy; keep the constructors that the egress tables call. Make the fallback `rt.toolRegistry` nil so an unknown profile fails closed.
- **Risk:** low for removal; the one real risk is the fallback-registry change (check `runtime.go:3352` handles a nil registry by `executeWithProvider`, which it does). Orange (runtime behavior); rollback git revert.

## 1.2 Conductor capsule lifecycle tools with no registrar
- **Location:** `tools_capsule.go:71-83` (`RegisterCapsuleTools`), `:141-257` (`spawn_capsule`, `destroy_capsule`, `list_capsules`), `:640-680` (`inspect_capsule`); about 170 lines. Also `capsule.RoleManagement` handling and `Executor.Spawn/ListOwned/DestroyOwned/ControlHandle` paths they exercise.
- **What it did:** Let the conductor/management agent create an isolated capsule, hold an opaque handle, list, inspect and destroy it (capsule runtime v14, 2026-07-05).
- **Why dead [V]:** `RegisterCapsuleTools` has no caller in non-test code. It was last called in `InstallDefaultAgentTools` and removed by `67a61358` (2026-08-08, "rollforward: restore reviewed supervision runtime"); only `tool_profiles_authority_test.go:82,109,147,227` mentions the names, and those tests assert the tools are absent. Management prompt line `management.yaml:11` says "Generic Engineering spawn and capsule lifecycle authority do not exist".
- **Scheduled:** no.
- **Proposal:** remove the four tools and `RegisterCapsuleTools`; keep `capsule_go_eval`. Check the capsule package for `ListOwned`/`DestroyOwned` having other callers before deleting.
- **Risk:** low. Yellow/orange; tests that assert absence become vacuous and can go.

## 1.3 `choir.Message`, `choir.Emit`, `choir.Emits` and the whole Emit transport
- **Location:** `choir.go:445-451` (stubs), module sets `choir.go:245-273` (every desk lists them), `surface.go:40-41`; plumbing `broker.go:166-170,379-440` (`handleMessage`, `handleEmit`, `SetEmit`), `sidecar.go:262-295`, `desk_session_worker.go:48,321-407` (`serviceEmit`), `transport.go:38-45` (`StreamBrokerEmit`, `StreamBrokerEmitResult`), `broker_protocol.go` (`ActionMessage`, `ActionEmit`, payloads), `tools_desk.go:292-298,532-600,683-720` (`deskEmitSignal`, emission drain into `choir.Emits()`), reducer `IntentMessage` branches `rlm_reduce.go:104-110,869-883`.
- **What it did:** Message and Emit were the original agent-to-agent channel mail; Emit wrote a durable channel row and woke the recipient. `choir.Emits()` returned the cell-start snapshot of emissions addressed to the desk.
- **Why dead [V]:** protocol P2c (`b196d666`, 2026-10-11) turned both verbs into stubs that return "retired" errors before touching the broker (`choir.go:445-451`). The only producer of a `kind:"emit"` envelope is `deskEmitSignal`, reachable only from `broker.handleEmit` via `ActionEmit`, and no `ChoirScope` method calls `ActionEmit` any more. So `Emits()` can only return pre-P2c rows. The generated surface still advertises all three to every desk (they are exported on 4 desks).
- **Citers:** tests (`emit_roundtrip_test.go`, `egress_roundtrip_test.go`, `choir_parity_test.go`) only; prompts mention them solely to say "retired" (`rlm_research_runtime.yaml:19`, `rlm_engineering_runtime.yaml:55`, Study G E2).
- **Scheduled:** Cut 1 (5): "path B, Assign, Message, Emit, Emits deleted" (artifact line, plus the grep gate at the acceptance list).
- **Proposal:** remove with Cut 1; do not delete earlier because `TestPromptTaughtVerbsMatchExports` / `TestSurfaceDocsMatchExportsAndSource` couple the surface, prompt and export in one commit.
- **Risk:** low; orange. Old tray replays are the only reason the reducer kept `IntentMessage`.

## 1.4 `choir.Assign`, the in-memory stub
- **Location:** `choir.go:95,249-251,306,425-435`; `broker.go:48,76,113-131,166-167,356-377` (`handleAssign`); `broker_protocol.go:19,115-125,295`; `surface.go:42`; `rlm_engineering_runtime.yaml:49`.
- **What it did:** Phase 1 of the yaegi kernel (`5e396897`, 2026-08-26) modeled "spawn a subagent": a payload goes into a broker map and returns `Status:"dispatched"`.
- **Why dead [V]:** spawns nothing, persists nothing, has no tray intent (`IntentAssign` does not exist). Its receipt `assign/<id>/<task>` is mixed into `staged_intent_ids` next to the real `rlm:*` ids (`broker.go:113-131`) and shown to the model. Exported to engineering only (`choir.go:250`); unknown roles also map to the engineering surface (`choir.go:132-138`). Study G section 3 holds a full deletion checklist.
- **Scheduled:** Cut 1 (5).
- **Proposal:** remove with Cut 1 using Study G's checklist (`choir_parity_test.go:86-96,187,211-212,246-248`, `broker_test.go:79` change in the same commit). Do not delete `CancelAssignment` or the `EngineeringAssignment` store; they only share the word.
- **Risk:** low; yellow/orange.

## 1.5 `spawn_agent` and `cancel_agent`
- **Location:** `internal/coagentowner/spawn_tool.go` (227 lines), `agentcore/tools_coagent.go:196-259` (`cancel_agent`), registrations `autoputer/run.go:606-625`, `tool_profiles.go:404-445`; residue `prompts.go:138-141`, `batch_executor.go:137-147,176-205`.
- **What it did:** the model-callable way to open a child agent (Texture from the conductor, researcher from management/texture) and to cancel one.
- **Why dead / state [V]:** live only for the conductor (`spawn_agent` role=texture; `cancel_agent` via `AllowCoAgentTools` on the conductor registry). Management/Texture are skipped at `run.go:616-618`; Research and Engineering get no registration because their `AllowedSpawnTargets` are empty. Research/Management/Texture `AllowedSpawnTargets` (`agentprofile.go:68,92,106`) therefore authorize nothing. The tool still documents `processor`/`reconciler` parameters (`spawn_tool.go:57-59`: "For role=texture from processor or reconciler: optional ... `initial_content` ... `source_item_ids`") for pipelines deleted 2026-10-05, and refuses lifecycle Texture with advice naming `open_researcher` (`spawn_tool.go:83-84`), a control that no longer exists. `cancel_agent` is taught nowhere (no prompt grep hit) yet registered on the conductor.
- **Scheduled:** Cut 1 (6b).
- **Proposal:** remove with Cut 1; also delete `AllowedSpawnTargets`/`CanSpawn`, the `allow_coagent_tools` owner-visible flag and the `ProviderPolicy.SupportsPerRunModelOverride` note that cites spawn_agent (`prompts.go:138-141`).
- **Risk:** medium: the conductor's only routing path. Cut 1 replaces it with an owner-emitted Cast; do the registry removal in that commit, not before.

## 1.6 `required_child_runs` / `required_texture_revisions` completion gate
- **Location:** `agentcore/tools_coagent.go:20-195` (`verifyRequiredChildRuns`, `verifyRequiredTextureRevisions`, `awaitRequiredTextureRevisions`, `awaitRequiredChildRuns`), called from `runtime.go:3543`; about 175 lines.
- **What it did:** Before a run could complete, wait up to 5 minutes for a number of child runs / reconciler Texture revisions declared in run metadata. Built for the Universal Wire processor/reconciler fan-out (commit `c791a0ae`, 2026-07-14, origin earlier).
- **Why dead [V]:** no non-test code writes `required_child_runs` or `required_texture_revisions` into any run metadata (grep of the string literals finds only readers and `tools_coagent_completion_test.go`), so `metadataIntValue(...) <= 0` returns immediately for every run. The inner filter requires `request_intent == "universal_wire_reconciler_article_revision"`, which also has no writer (see 3.1). Study G #50 reached the same conclusion.
- **Scheduled:** no.
- **Proposal:** remove the four functions and the `runtime.go:3543` call; delete `tools_coagent_completion_test.go`.
- **Risk:** low; yellow. Rollback git revert.

## 1.7 Batch-executor guards for tools that do not exist
- **Location:** `internal/toolregistry/batch_executor.go:147` (sequential list), `:176-205` (conductor duplicate-Texture skip), `:208-250` (`planSideEffectToolSkips`: `bash`, `spawn_agent` for Management, `update_coagent`), `:262-290` (`toolCallManagementEngineeringSpawnKey`, `normalizeEngineeringSlot`).
- **What it did:** serialized or de-duplicated side-effect tool calls in one provider turn.
- **Why dead [V]:** `bash`, `write_file`, `request_super_execution`, `request_email_draft`, `update_coagent` are registered nowhere in the repo; Management never has `spawn_agent` (so the management-engineering spawn key can never fire); `save_evidence` is not a tool on any live registry (it is a cell verb). Only `capsule_go_eval` / `desk_go_eval` (sequential), and the conductor's `spawn_agent` dedupe, can ever match.
- **Scheduled:** Cut 1 (6b) removes the `spawn_agent` part by deleting the tool; the rest not named.
- **Proposal:** reduce to `desk_go_eval`/`capsule_go_eval` sequencing and the `capsule_go_eval` one-per-turn refusal. Note Study G M7 found the "batching is refused" prompt claim is also false for `desk_go_eval` (it runs sequentially), so decide at the same time which behavior is intended.
- **Risk:** low; orange (tool execution policy).

## 1.8 Ledger acts with no consumer beyond Pack: Precommit, Resolve, Disagreement, Escalate (and CancelAct)
- **Location:** `choir.go:909-988`, `rlm_reduce.go:1102-1200`, `types/commitment.go`, `types/commitment_views.go`, prompt lines `rlm_management_runtime.yaml:19-20`, `rlm_research_runtime.yaml:19-20`.
- **What it did:** Mission R2 (`2f6f490c`, 2026-09-25) gave every desk a commitment ledger: freeze a typed prediction (Precommit), assert a report with a named resolver, resolve it with an evidence verdict, record a scorer/resolver disagreement (Disagreement), surface an issue (Escalate), retract (CancelAct).
- **Why suspect [V for reachability, I for use]:** the scorer does not exist ("Precommit and Resolve only record", Study G M12). `commitment_views.go` readers `AccrualByAgent`, `AccrualByDoc`, `BuildSupervisionPack` are unreachable (`dc-notest.txt`). The remaining readers are `Pack()` (per-desk memory), the learning-claim gate and the materiality projection. Exported to 4 desks; Texture's prompts never teach 7 of them (Report, ReportPacket, Reply, Precommit, Resolve, Disagreement, Escalate, CancelAct) and `foldTextureDispatch` folds only Cast/Ask/Note (Study G T3). Engineering's hand-written prompts teach none of Cast/Ask/Note/Reply/CancelAct/Escalate/Precommit/Resolve/Disagreement although all are exported (verified with the per-desk loop).
- **Scheduled:** CancelAct: Cut 1 (lines 506, 755: "CancelAct and desk Retract are deleted"). Precommit/Resolve/Disagreement/Escalate: no.
- **Proposal:** owner question: these are the product's precommitment mechanism (Cut 1 keeps Reports and hypotheses), so removal is not mine to propose. Minimum: stop exporting them to desks whose prompts never teach them (Texture, Engineering) or teach them in the Cut 1 (7) prompt rewrite and add the reverse parity test.
- **Risk:** red (Trace/evidence ledger) if touched.

## 1.9 Dual paths and exported-but-never-taught verbs
- **Report vs ReportPacket:** `choir.Report(toDesk, claim, evidenceRefs, resolverID)` builds a one-claim packet without `work_disposition` (`rlm_reduce.go:1987-1994`, Study G M3) while `ReportPacket` carries the full schema; both reduce through `IntentReport`. The management prompt tells the model to use `Report` to complete work; the wake text tells it to set `work_item_id` which `Report` cannot carry. Scheduled: Cut 1 (typed Report body).
- **ImportDocument vs ImportURL:** `ImportDocument(url, filePath, query)` is a superset of `ImportURL(url, query)` (`choir.go:567-577`), both mapped to different broker actions. Prompts teach both. Not scheduled; keep unless the content tools are reshaped (owner question).
- **Research exports `Cast`** (`choir.go:254`) while `rlm_research_runtime.yaml:19` says "research does not cast work to other desks"; `castOpensEngineeringAssignment` has no caster-profile gate (Study G E4/R2). Not scheduled. This is an authority gap, not only dead surface: propose removing `Cast` from research (orange/red).
- **PollSource, ReadCapture** are exported to research, appear in the generated block, but no hand-written prompt names them (`research.yaml`, `rlm_research_runtime.yaml`). Cut 2 mentions `PollSource` (line 68, captures). Keep.
- **Texture ReadFile/ListDir/Context/Pack/Help** are exported but never taught; ReadFile roots at the run working dir or `os.TempDir()` (Study G M8). Not scheduled; owner question.
- **Risk:** orange.

## 1.10 `ProductAPI` allowlist entries with no handler
- **Location:** `tools_product_api.go:184-199`, taught at `rlm_management_runtime.yaml:21`.
- **Why dead [V]:** the in-cell call serves from the guest's own router. Grep shows no guest route for `/api/continuations` or `/api/continuations/` (only the allowlist), the trace HTTP handler `trace.NewHTTPHandler` (`internal/trace/query.go:97`) is never mounted (deadcode lists it unreachable), `/api/universal-wire/` exists only in the host proxy (`proxy/handlers.go:836`), and `/api/computers/*` always returns 403 for lack of an `X-Authenticated-Computer` header (Study G M9, verified `api_self_development.go:84-110`). Live: `/api/prompt-bar`, `/api/texture/`, `/api/run-acceptances*`, `/api/autopaper/`.
- **Scheduled:** Cut 2 (lines 109, 234) touches the Texture-write exception only.
- **Proposal:** drop the four dead prefixes and the matching sentence in the management overlay.
- **Risk:** low; orange (product API surface). The `/api/trace/` prefix is tied to item 4.4.

## 1.11 Unreachable yaegikernel files
- **Location (`deadcode -test=false`, `dc-notest.txt`):** `actor_state.go` (205 lines, whole file: `ActorStateManager` durable actor state, 2026-08-26 Phase 2, `6a783351`); `transclusion.go` (104, `ReceiptCollector`, `FormatTextureTransclusion`, Phase 3 `3d88dcc9`); `eval.go` `NewEvaluator`/`Evaluator.Eval`; `sidecar.go` `SidecarRunner`, `ExecuteWorkerStdin/SessionStdin` (process-per-activation sidecar, Phase 0 `94232b6c`); `session_loop.go` `RunSessionLoop*` (including `RunSessionLoopFramed`); `transport.go` `SocketPair`; `inspect_bundle.go` `InspectMountedBundleAt`.
- **What they did:** the yaegi kernel prototype phases 0-3 (in-process evaluator, sidecar subprocess, durable actor state with rewarm and epoch fencing, salient receipt transclusion). The current path is `desk_session_worker.go` / `ExecuteWorkerSessionConn` / `buildSessionWorker`.
- **Why dead [V]:** deadcode finds no production caller; the replacement is wired and used. Several still have tests (`actor_state_test.go`, `transclusion_test.go`, `session_test.go`), which keep them alive only for `go test`.
- **Scheduled:** no (Study G section 3 mentions `actor_state.go`).
- **Proposal:** remove after confirming `eval.go` helpers (`ReuseDisposition`, `DiagnosticKind`) used by the live path are retained; split carefully (about 960 lines).
- **Risk:** low; yellow (tests deleted with code).

## 1.12 Unused tool-loop forcing options
- **Location:** `toolregistry/toolloop.go:172,185,217,255,276,1303,1458` (`WithInitialToolChoice`, `WithTerminalToolSuccesses`, `WithPassivatingToolSuccesses`, `WithRequiredWriteTools`, `WithRequiredWriteToolResult`, `isInitialToolChoicePreconditionError`, `successfulTerminalToolNames`).
- **What they did:** forced a first tool call, made named tools terminal/passivating, and required a durable write tool before a run could end, the typed-tool-era way to force Texture into `patch_texture`/`rewrite_texture`. Last real users removed by `3ed94220` (2026-09-26 R3d) and `2fbecebc`.
- **Why dead [V]:** `deadcode` lists them unreachable; the `...Result` variants (`WithTerminalToolResult`, `WithPassivatingToolResult`) are the live ones.
- **Scheduled:** no (Cut 1 (6) deletes the budgets, not these options).
- **Proposal:** remove the five options and their guard-loop branches. **Risk:** low; orange (tool loop).

---

# Section 2. Prompts that describe things that no longer exist

Each entry quotes the sentence. Cut 1 (7) rewrites the four desk prompts "once for this system"; I mark which items it covers.

## 2.1 Run-memory compactor tells every desk to call a tool no desk has
- `internal/agentcore/run_memory.go:602` "Preserve exact entry_id handles so the future agent can call get_run_memory_entry when details are needed."; `:652` "Use get_run_memory_entry(entry_id) in retrieval_instructions when exact raw content may matter."; `:791` "Compacted raw entry ids available via get_run_memory_entry:".
- **Why dead [V]:** the tool is registered only on the shadowed registries (item 1.1). The cell verb is `choir.RunMemoryEntry`, exported to research only (`choir.go:260-264`); Texture, management and engineering have no retrieval at all. The text is injected into the compaction checkpoint the desk reads after a context rollover, so the mismatch is in the model's context, not just in code. Compaction does run for desk runs (the memory manager is per run). [I] how often a compaction happens on desks in production.
- **Scheduled:** no. **Proposal:** either name `choir.RunMemoryEntry` and export it to the desks that compact, or drop the retrieval sentences. **Risk:** low; green/yellow (prompt text) but compaction schema is parsed, so change only the strings.

## 2.2 Skill context names a deleted skill
- `skill_context.go:13` `runtimePromptSkills = {"mission-gradient", "cognitive-transform-portfolio"}` and `:44` "Use `mission-gradient` for long-running, staging, self-development, or broad architectural work."
- **Why dead [V]:** `skills/mission-gradient` was archived on 2026-10-04 (`f70414a6`); `os.ReadFile` fails and the entry is skipped silently, but the closing sentence is written whenever any entry (cognitive-transform-portfolio) loads, into the conductor, Texture, management and engineering prompts. The remaining skill is a coding-agent skill; whether any desk can read `.../share/go-choir/skills/` (`RUNTIME_SKILLS_ROOT`) is [I]. The block also says "no slash commands", which is true only because desks have none.
- **Scheduled:** no (Cut 1 line 678 "specs and skills in the same commits" is unrelated). **Proposal:** remove `skill_context.go` (129 lines), `RUNTIME_SKILLS_ROOT` env and the skills copy into the package, or at least the dead sentence. Owner question because the repo skills are the owner's coding-agent contract.
- **Risk:** low; green/yellow.

## 2.3 Conductor teaches `patch_texture`
- `internal/promptstore/defaults/conductor.yaml:16` "the durable v1 should be written by the Texture agent with `patch_texture`."; `internal/runtimeprompts/overlays/conductor_run.yaml:13` "the Texture desk writes durable v1 with patch_texture." Also `conductor.yaml:14` and `conductor_run.yaml:9` teach `spawn_agent`, and `conductor_run.yaml:11` "route with the available routing tools".
- **Why dead [V]:** no `patch_texture` tool or verb exists (`rewrite_texture`, `decide_texture` likewise); Texture authors with `choir.ApplyTexture`. Study G T7.
- **Scheduled:** Cut 1 (6b) replaces the conductor path, so these two files go with it. **Proposal:** fix or delete with the conductor change. **Risk:** green.

## 2.4 Engineering overlay: `choir.Assign`, `rlm:message:1`, durable `agent_id`
- `rlm_engineering_runtime.yaml:49` "Synchronous Syscalls (immediate effects): ... choir.Pack, choir.Assign." (Assign is the 1.4 stub); `:41` Staged Intent Tickets `"rlm:complete:1", "rlm:message:1"` (Message retired); `:55` "choir.ReportPacket(to, packet, "") addresses one agent by exact durable agent_id" and `:60` "addressing only the requesting Management agent_id" while the reducer requires a bare desk name (`rlm_reduce.go:166-172`; Study G E3); `run_context.yaml:17` tells engineering to coordinate with `choir.Cast`.
- **Scheduled:** Cut 1 (7). **Proposal:** covered by the rewrite; until then fix the three sentences. **Risk:** green.

## 2.5 Management prompt and overlay
- `management.yaml:11` "Assignments open on the document channel - an owner-authored revision on an engineering-bound document is the cast; the runtime opens the assignment directly. ... document-parent assignments are cancelled by the runtime." and `rlm_management_runtime.yaml:28` "An owner-authored revision on an engineering-bound document is the cast trigger, and a management cell stages choir.Cast". Describes path B, which Cut 1 deletes; contradicts the overlay's own Cast path.
- `rlm_management_runtime.yaml:21` allowlist includes dead routes (item 1.10) and `:30` "choir.CancelAssignment(target_ref, reason)" while the signature is `(assignmentID, reason)`.
- `rlm_management_runtime.yaml:13` "batching multiple eval calls in one turn is refused and none run" (false for `desk_go_eval`, 1.7).
- **Scheduled:** Cut 1 (7)+(5). **Risk:** green.

## 2.6 Wake and runtime-authored turns still say `update_coagent` / `work_item_id`
- `runtime.go:2963` "call update_coagent with work_disposition=completed only when the assigned lifecycle work is fully satisfied" and `:2969` "Every update_coagent call must set work_item_id to the specific item it addresses."; carrier branch at `:2961-2967` tells management/research to "use choir.Report to assert work_disposition=completed", a field `Report` does not have. `buildAssignedWorkItemPromptForCarrier(…, profile == Management || profile == Research)` (`runtime.go:2926`, `management_controller.go:1793`) therefore sends the **non-carrier `update_coagent` text to Texture and Engineering** wakes [I: whether those wakes carry work items today]. Also `coagent_update_packet.go:297-303`, `trajectory.go:114`, `management_controller.go:2261-2262` ("Process the pending update_coagent records... report blockers with update_coagent"), `api.go:689`, `runtime.go:1560` ("wait for update_coagent evidence"). `update_coagent` is named in about 31 non-test Go files (Cut 1 line 100).
- **Dead helpers [V]:** `buildPersistentManagementUpdatePrompt` (`management_controller.go:1724`, says management is the "persistent super actor"), `buildCoagentBacklogPrompt` (`:2244`) and `buildAssignedWorkItemPrompt` (`runtime.go:2953`) have no non-test caller (also Study G M14/X3).
- **Scheduled:** Cut 1 (6) "every update_coagent name deleted". **Proposal:** delete the three dead helpers now (green/yellow, no dependency on the cut); leave the live strings to the cut. **Risk:** low.

## 2.7 Research overlay and runtime-authored instructions
- `researchtools.go:787,924,995,1023`: `next_instruction` "Submit a concise first update_coagent source packet from this search result before any additional search-only turn." injected into `WebSearch`/`FetchURL`/`SourceSearch` results, which research cells receive through `HostEgress` (item 1.1 live part). `researchtools.go:664` the `web_search` description embeds "Research cadence: ... then update_coagent on the next model turn", dead (shadowed).
- `research.yaml:44-46` and `rlm_research_runtime.yaml:18` "budgeted ... egress budget"; Texture prompts `tool_profiles.go:252` and `run_system.yaml:10,31` "Each owner request ... allows a bounded number of research rounds; the runtime drops further rounds"; all describe caps Cut 1 deletes.
- **Scheduled:** Cut 1 (6). **Risk:** green/orange.

## 2.8 Template branch, options and a file with no live trigger
- `revision_policy.yaml:25` `{{if .ExplicitResearchRequest}} The owner explicitly asked for research help...`: `ExplicitResearchRequest` is fed from run metadata `explicit_researcher_request` (`texture_agent_revision.go:823`), which no code writes (introduced `93fc3ada` 2026-06-13, writer later deleted). Dead branch [V].
- `internal/textureprompts/overlays/revision_media_source_research_required.yaml` has no Go function that renders it; the exporter was deleted (`docs/archive/evidence/s3-runtime-declaration-only-dispatch-2026-07-12.md`). Orphan embed file [V]. Companion `buildTextureMediaSourceResearchObjective` (`texture_media_sources.go:886`) names `read_content_item`, `import_url_content`, `update_coagent` and has no caller (Study G T6).
- `RevisionPolicyOptions.DocID` / `RevisionID` appear in the template four times but the surrounding prose does not depend on them [I].
- `run_context.yaml:17` `{{else}}Use only the report or message module exposed in the current tool catalog...`: `InCellCarrier` is false only for conductor, email (no peers), and `TextureDeliveryAgentID` ("use only a report or message module already exposed by your desk") is a branch for a module name that no longer exists.
- **Scheduled:** Cut 1 (7) partly. **Proposal:** delete the dead branch, file and function. **Risk:** green.

## 2.9 Generated surface docs that are wrong or advertise dead names
- `surface.go:29` Context: "Your activation identity (agent, run, desk, doc and assignment ids)" but `Context()` returns `computer_id`, `activation_id`, `co_super_slot` only (`choir.go:779-788`). `surface.go:28` the `Help` example `choir.Help("ApplyTexture")` is shown to all desks, and answers "There is no choir.ApplyTexture for this desk" for 3 of 4. `surface.go:36,40-42` list Emits, Message, Emit, Assign (items 1.3-1.4). `surface.go:39` omits the `email` ApplyTexture op that `run_system.yaml`/`revision_policy.yaml` teach (Study G T4).
- `co_super_slot` (a retired term for the Engineering slot) is the returned key and used in `rlm_engineering_runtime.yaml:21,51`.
- **Scheduled:** Cut 1 (7) generates the verb list; the Context field text not named. **Risk:** green.

## 2.10 Negative documentation of removed tools
- `engineering.yaml` and `rlm_engineering_runtime.yaml:9` "Static host reads, glob, grep, evidence, model diagnostics, agent lifecycle, and capsule lifecycle are unavailable." Tells the model about tools it never had in this mode. Keep or drop with the rewrite. **Risk:** green.

---

# Section 3. Profiles, roles, kinds, metadata keys

## 3.1 Processor/reconciler residue in metadata and routing (emitted nowhere, read in several places)
- **Keys with no non-test writer [V]:** `explicit_researcher_request`; `required_child_runs`, `required_texture_revisions` (1.6); request_intent values `universal_wire_reconciler_article_revision` (read at `tools_coagent.go:62,114`, `coagent_route.go:128`) and `universal_wire_processor_article_revision` / `integrate_worker_findings` (test-only); `runMetadataProcessorKey` / `runMetadataReconcilerScope` and the keys `processor_key`, `reconciler_scope`, `source_network_{cycle,request}_id`, `source_network_request_kind`, `ingestion_handoff_{cycle,request}_id`, `ingestion_handoff_request_kind`, `source_item_ids`, `selected_style_{sources,rationale}`: no code mints them; they are copied from parent run metadata (`textureowner/coagent_route.go:85-201`), carried in three allowlists (`runtime.go:4435-4470` `durableMetadataKeys`, `api.go:78-91`, `texture_task_types.go:26-66`), and read (`wire_publication.go:176,219`, `trajectory.go:66`, `runtime.go:478-482,862,874`, `wirepublish/eligibility.go:129,150`). The only origin is a still-live caller that stamps `source_network_cycle_id` in `wirepublish/request.go:19`. `runMetadataSpawnReused` is written at `runtime.go:1174,1219` on a branch Study G #51 shows unreachable (engineering slot claim, refused at `:1135`).
- **What it did:** SourceCycled processors and reconcilers (Universal Wire ingestion, 2026-05 to 2026-09) carried cycle/handoff identity down the spawn chain. The pipeline was deleted by `3c963edf` (2026-10-05, SR).
- **Scheduled:** partly. Cut 1 (3) deletes `MaybeAutonomousPublishWireArticle`, the `wire_publication` and wire-processor work-item kinds and their boot recovery; Cut 2 replaces per-version publication. The metadata allowlists, `coagent_route.go` classifier (Study G #46: keyword article-style classifier reachable only via frozen handoffs) and `trajectory.go:66` are not named.
- **Proposal:** remove the keys from the three allowlists, the dead intent literals, and `coagent_route.go` handoff metadata copy after Cut 1 lands (they feed the wire publication code Cut 1 deletes). **Risk:** orange (run metadata round-trip for stored runs); keep `agentprofile` `processor`/`reconciler` tokens in `Canonical` for replay (see 3.3).

## 3.2 Event kinds: never emitted, or emitted and never read
- **Defined, no Go emitter or reader, only frontend listeners [V]:** `EventDesktopStateUpdated` ("desktop.state.updated") and `EventComputerStatusUpdated` ("computer.status.updated") (`types/task.go`, both added by the 2026-05-20 live-sync cutover `7e06ba11`; zero Go references anywhere). `ComputeMonitorApp.svelte:45` and `desktop` bundle listen for both strings, so the frontend branch never fires.
- **Read, never emitted [V]:** `EventRunContinuationSelected` / `EventRunContinuationStarted` ("loop.continuation.*") appear only as a `case` in `run_acceptance.go:726`. The emitter was deleted in the M8 dead-code phase (`fa166de1`, 2026-06-28). Acceptance code that counts continuation events always sees zero.
- **Emitted, read nowhere in Go [I]:** `EventChannelMessage` is emitted (`tools_research.go:130` via `emitChannelMessageEvent`, 5 callers) and consumed by the frontend `PromptSurface.svelte:141,161` only; `EventFileChanged`, `EventContentItemCreated` likewise are product-event fan-out to UI listeners (keep). `EventTextureDecisionRecorded` is emitted at `texture_agent_revision.go:1282,1293` with no Go or frontend reader found [I] (frontend grep `texture.decision.recorded`: none).
- **Lifecycle kinds [V]:** `LifecycleActivationReplaced` and `LifecycleOwnerInstructionQueued` ("M1-deleted 2026-09-23") are referenced only by `vocabmigrate/families.go:80,107` as frozen history vocabulary: keep until history is rewritten.
- **Scheduled:** no (Cut 1 deletes lifecycle events tied to work items wholesale).
- **Proposal:** remove the two unused constants plus the dead frontend branch; remove `Continuation*` case; owner-confirm the decision-recorded and channel events. **Risk:** low; yellow.

## 3.3 Agent profiles and the policy table
- `agentprofile.go:125-141`: `processor`, `reconciler` stay `Canonical` so stored runs decode, but `PolicyFor` and `IsLive` refuse them; `verifier`/`verifier_multimodal` are canonical profile names yet no `RunRecord` is ever created with them (verifier is a model role and an Engineering slot). `Email` has a bare policy and an empty registry (live for the email app agent).
- `AllowedSpawnTargets` for Research/Management/Texture and `CanSpawn` (see 1.5), `IsTexture` (deadcode: unreachable), and `AllowCoAgentTools` (cancel_agent) are dead or collapse with Cut 1.
- **Proposal:** keep `processor`/`reconciler` for replay (restore must still decode history); drop `verifier` from `Canonical`'s run-profile set after checking `vocabmigrate`; delete `IsTexture`. **Risk:** red if replay breaks; test with the frozen V1 decoder.

## 3.4 Constants and always-true predicates
- `deskCarrierLive` (`tool_profiles.go:359-370`) returns true for management, texture and research; the `if !deskCarrierLive(...) continue` branch at `:498` and the `default:false` arm only matter for conductor/email/engineering. `buildAssignedEngineeringRegistry` is a one-line wrapper (`:339-341`). `desk_worker_*_other.go` / `_linux.go` split is legitimate. **Proposal:** inline; trivial. Orange? green, no behavior change.

## 3.5 Engineering/management "co-super" vocabulary in live strings
- Lifecycle event string values `co_super_assignment_*` (`types/lifecycle.go:73-77`) and the `co_super_slot` Context key. Renaming touches stored event vocabulary; leave and record as a rename residue, not dead code.

---

# Section 4. Env vars and configuration

Lists derived by diffing Go reads against assignments in `nix/`, `flake.nix`, `start-services.sh`, `.envrc`, `.github`, and injected env (`vmctl/handlers.go:1499`, `autoputer-vm.nix` cmdline parsing). Many "read but unset" variables are legitimate optional overrides with a default; only the behaviorally dead ones are items.

## 4.1 Qdrant / Ollama semantic-dedup chain: zero production caller, plus a Nix service
- **Location:** `internal/qdrant/` (2,006 lines), `agentcore/qdrant_dedup.go` (200), `qdrant_runtime.go` (55), config `QdrantURL`, `OllamaURL`, `OllamaEmbeddingModel`, `QdrantDedupThreshold` (`provideriface/config.go:131-143`, env `QDRANT_URL`, `OLLAMA_URL`, `OLLAMA_EMBEDDING_MODEL`, `QDRANT_DEDUP_THRESHOLD`), `autoputer/run.go:653-660` readiness probes of qdrant and ollama, `nix/node-b.nix:275-299` `services.qdrant`, `docker-compose.qdrant.yml`, `.github` none.
- **What it did:** mission-3b (`a87c3be6`, 2026-06-27) dropped near-duplicate source items before processor dispatch by embedding with Ollama and searching a Qdrant collection.
- **Why dead [V]:** `dedupSourceItemsSemantically` is only called by `dedupSourceItemsWithTimeout`, which has no caller (`deadcode` lists both, plus `closeQdrantPipeline`). The World Wire metamission doc already says it "exists but has zero production caller" (`docs/archive/definitions/choir-world-wire-metamission-2026-10-01.md:37`). Ollama is not provisioned anywhere in Nix or `flake.nix` (grep: none), the node-b comment itself says VMs on the tap network cannot reach `127.0.0.1:6333`, and the `/health/ready` aggregator only reports "degraded".
- **Scheduled:** no.
- **Proposal:** remove the package, the pipeline code, the four config fields, the readiness probes and the Nix qdrant service. Owner question only if semantic dedup is planned for Autopaper (Cut 2 / source schedules); if so keep the package and drop the service until used.
- **Risk:** low; orange (config), the Nix removal changes node-b (deploy-shape, ask before applying).

## 4.2 `RUNTIME_TRACE_PERSISTENCE_ENABLED` is never set, so the Dolt trace store is off in production
- **Location:** `provideriface/config.go:146`, `autoputer/run.go:516-526`, `internal/trace/` (store 469, query 218, redact, about 690+ non-test lines), `agentcore/runtime.go:74-79,837-840,4699-4703`, `runtime_persistence.go:168-170`.
- **What it did:** an append-only observability store for runtime events in the owner's Dolt workspace (M20b, 2026-06-28) plus a GET `/api/trace/events` query handler.
- **Why dead [V]:** default `false`; set in no nix, script, workflow or kernel-cmdline path (`grep -r RUNTIME_TRACE_PERSISTENCE` finds config.go, a comment and a doc), so `traceStore` is nil and every `traceStore.Append` is skipped. `trace.NewHTTPHandler` is never mounted (deadcode: unreachable `NewQueries`, `NewHTTPHandler`, `ServeHTTP`). The only `/api/trace/` mentions are `run_acceptance.go:573` (builds a URL string for a moment link) and the ProductAPI allowlist (1.10).
- **Scheduled:** no. Cut 0 (eyes) may rely on events instead; I did not read Cut 0.
- **Proposal:** owner question: this is the intended observability store per the desk-RLM plan (`docs/archive/desk-rlm-rectification-plan-2026-09-23.md:663` "default false"). Either turn it on and mount the query handler, or delete the store and the flag. Today it is dead weight that looks live.
- **Risk:** red if kept and enabled (protected surface: Trace/evidence); deletion is orange.

## 4.3 Config fields with no behavior
- `ResearchCount` / `RUNTIME_RESEARCHER_COUNT` (default 3): read into `cfg`, logged as `orchestration topology (super=1, researchers=%d)` (`run.go:678`) and echoed as `researcher_count` in `/health` (`api.go:959`) and the proxy compute status (`proxy/compute_status.go:73,680-692`). No frontend or behavior reads it. The "3 researchers" is a topology from the Universal Wire era; today research is one desk per document.
- `SupervisionInterval` / `RUNTIME_SUPERVISION_INTERVAL`: field comment says "legacy reserved configuration"; propagated at `run.go:880`, read nowhere.
- `TextureWakeDebounce` / `RUNTIME_TEXTURE_WAKE_DEBOUNCE` (3s): defaulted and propagated (`run.go:882`), no consumer; the Texture wake path no longer debounces.
- **Scheduled:** no. **Proposal:** remove all three plus the log line and health field. **Risk:** low; orange (health response shape; check `scripts/` and the frontend for `researcher_count` readers: none found).

## 4.4 Env vars set but never read by any Go code [V]
| Var | Set in | Note |
|---|---|---|
| `RUNTIME_SUPER_FOREGROUND_MUTATION_MODE`, `RUNTIME_LOCAL_WORKER_MODE`, `RUNTIME_SELF_URL` | `start-services.sh:115-117` | "super" and worktree-worker modes are retired; first appeared `e759ce3f` 2026-05-13 |
| `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS`, `SOURCE_SERVICE_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS` | `nix/node-b.nix:637-638` | processor dispatch deleted `3c963edf` 2026-10-05 |
| `VMCTL_BASE_BLOB_ROOT`, `VMCTL_PROMOTION_AUTHORITY_PUBLIC_KEY` | `nix/node-b.nix:709-710` | no Go reads either name (`0dc3fea3` 2026-07-16 added the Nix line; the readers moved or were never written) |
| `CHOIR_COMPUTER_KIND` | `autoputer-vm.nix:453` from kernel param `choir.computer_kind` (`vmmanager/manager.go:1688`) | no reader; the param is still passed |
| `CHOIR_KERNEL_CAPABILITY_PROBE` | `autoputer-vm.nix:794` | only `..._PROBE_OUTPUT` is read |
| `RUNTIME_ZAI_MODEL`, `RUNTIME_BEDROCK_MODEL` | `nix/deploy-provider-creds.sh:128,139` | read only by `vmmanager/manager_test.go`; model choice is now `modelpolicy` |
| `SOURCE_SERVICE_RUNTIME_BASE_URL` | guest env via cmdline `choir.source_service_runtime_url` (`autoputer-vm.nix:447`) | read only by `sourcecycled/main_test.go:181` |
- **Scheduled:** no. **Proposal:** delete the assignments (and the vmmanager kernel params for the last three). **Risk:** low; red-adjacent for the two `VMCTL_*` lines (vmctl is a protected surface, but a variable nobody reads cannot change behavior; confirm the signing key is not meant to be read: the public key is a security-relevant value, so ask the owner whether the reader was lost by mistake rather than deleting).

## 4.5 Env vars read by Go and set nowhere (default is the only live branch)
Verified no setter in Go, Nix, scripts, workflows; most documented only in `docs/`: `AUTH_ROTATION_GRACE_TTL`, `AUTOPUTER_FILE_SYNC_INTERVAL` (set in Go test only), `CHATGPT_BASE_URL`, `CHATGPT_CLIENT_ID`, `CHATGPT_SEARCH_BASE_URL`, `CHOIR_PODCAST_SEARCH_URL`, `CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH`, the six `CHOIR_YOUTUBE_*` keys, the `CHOIR_SEARCH_*` tuning knobs (`HEALTH_MEMORY`, `HEALTH_PATH`, `MAX_WAVES`, `MIN_MERGED_RESULTS`, `PROVIDERS_PER_QUERY`, `REQUEST_TIMEOUT_SECONDS`, `WAVE_GRACE_MS`), `CHOIR_ZOT_BASE_URL/MODEL/PROVIDER/REASONING`, `CHOIR_CAPSULE_MEMORY_TOTAL_BYTES`, `CHOIR_KEY_ESCROW_KEY_PATH`, `CHOIR_PLATFORM_CAPABILITY`, `GATEWAY_BEDROCK_MODELS`, `GATEWAY_OPENCODE_GO_MODELS`, `GATEWAY_OPENCODE_ZEN_MODELS`, `GATEWAY_JEV_RECEIPT_PATH`, `GATEWAY_RATE_LIMIT_WINDOW`, `FIREWORKS_BASE_URL`, `OPENCODE_GO_BASE_URL`, `OPENCODE_ZEN_BASE_URL`, `RUNTIME_DOLT_GC_JOURNAL_GIB`, `RUNTIME_DOLT_GC_MILESTONE_GIB`, `RUNTIME_DOLT_GC_DISABLED`, `RUNTIME_PROMPT_ROOT`, `RUNTIME_MODEL_POLICY_PATH`, `SERVER_IDLE_TIMEOUT`, `SERVER_READ_HEADER_TIMEOUT`, `SERVER_READ_TIMEOUT`, `SOURCECYCLED_*` (see 4.7), `VMCTL_OWNERSHIP_PATH`, `VMCTL_PLATFORM_WIRE_BOOT_TIMEOUT`, `VMCTL_RECOVERY_CONDITIONS_PATH`, `VMCTL_RECOVERY_JOBS_URL`, `VMCTL_RECOVERY_QUARANTINE_RETAINED`, `VMCTL_RETENTION_SHADOW_STATE_DIR`, `VMCTL_RETENTION_STATE_DIR`, `VMCTL_VM_STATE_DIR`, `VM_GUEST_PORT`, `VM_KERNEL_PARAMS`, `VM_SOURCE_SEED_REPO`, `CHOIR_BOOTSTRAP_ADMIN_API_KEY`, `PLATFORM_SIGNING_KEY_PATH`, `CORPUSD_CORPUS_DOLT_DSN`, `SOURCE_SERVICE_DOLT_DSN`, `RUNTIME_CORPUSD_URL`, `OPENCODE_GO_API_KEY` (set only by the `deploy-provider-creds.sh` secrets file, not in the repo), `CHOIR_DESK_WORKER_CHILD`, `YAEGI_WORKER_EXEC`, `RLM_CAPTURE_STATE`, `CHOIR_G1_*` (test only).
- **Reading:** these are optional overrides; none changes behavior unless an operator sets it. Several are secrets or paths supplied through a host-side env file (`/var/lib/go-choir/*.env`, not in the repo), so "never set" here means never set in tracked files, [I] for the host. The deletable ones are the ones with an obsolete subject: `CHATGPT_*` (provider base URLs; confirm against `provider.go:2748`), `CHOIR_ZOT_*` (zot terminal app), `SOURCECYCLED_*`, `RUNTIME_DISABLE_TOOLS` (4.6), `CHOIR_G1_*`, `YAEGI_WORKER_EXEC`, `RLM_CAPTURE_STATE`, `CHOIR_DESK_WORKER_CHILD` (read only in a test).
- **Scheduled:** no. **Proposal:** do not bulk-delete. Remove only test-only and retired-subject names; keep operational overrides. Owner question for the host env files.

## 4.6 Feature flags fixed to one value
- `RUNTIME_DISABLE_TOOLS` (`autoputer/run.go:578`): never set in Nix, scripts, workflows or tests, so `toolsEnabled` is always true and the `else` log branch plus the "stub-only mode" path are dead. The comment two lines above also promises `RUNTIME_ENABLE_TOOLS` back-compat, which nothing reads (the string appears only in that comment). **Proposal:** remove the flag, the branch and the comment. Low risk; orange.
- `VMCTL_PLATFORM_WIRE_ENABLED` (`cmd/vmctl/main.go:573`): `nix/node-b.nix:800` sets it `true`; the default-false early return is dead in production. Keep unless the platform wire computer is removed (Cut 1/2 touch Universal Wire).
- `GATEWAY_JEV_JUDGMENTS_ENABLED` (`gateway/config.go:95`): `nix/deploy-provider-creds.sh:162` defaults to `1`, so Jev judgments are always on in a deployed gateway; the code comment still calls the route "experimental". Study G notes Jev's sole caller is `judgeFrozenCandidate`, which Cut 1 (5) removes ("runtime Jev judge ... removed"), after which this route has no caller either. **Proposal:** decide with the Jev question in Cut 1 (decisions "Jev is part of management").
- `RUNTIME_ENABLE_TEST_APIS`: set only in `start-services.sh` (0 by default): local dev only; keep.
- `RUNTIME_DOLT_GC_DISABLED`: never set; keep as an operator kill switch.
- `deskCarrierLive` (3.4) is the in-code analogue.

## 4.7 Dual env names: `SOURCE_SERVICE_*` and `SOURCECYCLED_*`
- `cmd/sourcecycled/main.go:138-194,283-303`, `researchtools.go:73,1147`, `agentcore/universal_wire.go:4` read each setting under both names (`SOURCE_SERVICE_ADDR` / `SOURCECYCLED_ADDR`, `_CONFIG_PATH`, `_DOLT_DSN`, `_OBJECTGRAPH_*`, `_VMCTL_URL`, `_RUNTIME_OWNER_ID`, `SOURCECYCLED_API_URL`). Only `SOURCE_SERVICE_*` is ever set (Nix, kernel cmdline); `SOURCECYCLED_*` exist as the old name (the `corpus-dsn.env` runbook comment in `node-b.nix:624` still says `SOURCECYCLED_DOLT_DSN`). CLAUDE.md retires the names "sourcecycled" and "Universal Wire" and schedules a code rename in `docs/autopaper-sw-design-2026-10-09.md` s9.
- **Scheduled:** the rename is a TODO, not Cut 1/2. **Proposal:** fold the alias deletion into that rename; the runbook comment makes one of them live on node-b today, so confirm `corpus-dsn.env` first. **Risk:** low; red-adjacent (a DSN flip).

---

# Section 5. Nix, build, repo-level artifacts

## 5.1 `go-choir-sourcecycled.service`: defined, deliberately not started
- **Location:** `nix/node-b.nix:613-643`.
- **What it did / state [V]:** the Source Service ingestion daemon. The comment says it is "deliberately NOT in the boot set" since the 2026-10-05 corpus burn (`ee013c9a`); the unit has no `wantedBy`, only an explicit `systemctl start`. Its env still sets the dead dispatch variables (4.4). `cmd/sourcecycled` (built in `flake.nix:183-186`) is the Autopaper ingestion path being renamed.
- **Scheduled:** no. **Proposal:** keep until Autopaper source schedules take over (`/api/autopaper/sources/schedules`, `source_schedule.go`); then remove unit, command and package together. Strip the two dispatch variables now. Owner question.

## 5.2 Binaries in `cmd/` that no Nix package, script, workflow or Taskfile builds or runs
`cmd/basecompare` (241 lines), `cmd/baseharness` (549), `cmd/baseobserve` (290), `cmd/choir-rebuild-base` (383), `cmd/runtime-ratchet` (2,696 incl. inventory), plus the empty directory `cmd/choir-checkpointd/` (untracked, zero files). All five referenced only from `docs/` and their own tests. `cmd/lockscope` is wired into CI (`.github/scripts/deploy-impact-classify`), `cmd/doccheck` into scripts. `choir-rebuild-base` is the recovery rebuild CLI (2026-10-08), possibly operator-run by hand.
- **Scheduled:** no. **Proposal:** owner question for `choir-rebuild-base`; the others (substrate landing harnesses from July, runtime-ratchet from the vocabulary cutover) are candidates for removal with their tests. Another agent may cover `cmd/` more deeply. **Risk:** low.

## 5.3 Repo-root build artifacts committed
`capsule-broker` (32.8 MB executable binary, tracked since `19e2c256` 2026-09-27) and `result` (a symlink into `/nix/store/...-prefetch-npm-deps`, tracked since 2026-04-11). Neither is built from or consumed by anything in the repo (the capsule-broker package is `flake.nix:295`). An empty untracked `configs/` directory also exists locally (nothing in git). **Scheduled:** no. **Proposal:** `git rm` both; add to `.gitignore`. **Risk:** low, but the 32 MB blob stays in history.

## 5.4 `start-services.sh` stale exports
`RUNTIME_SUPER_FOREGROUND_MUTATION_MODE`, `RUNTIME_LOCAL_WORKER_MODE`, `RUNTIME_SELF_URL` (4.4); `RUNTIME_TOOL_CWD` is read (`run.go:580`) and fine. The script also starts `cmd/corpusd` plus a local Dolt, but not `maild`, which the guest expects (`RUNTIME_MAILD_URL`) [I]. **Proposal:** delete the three lines.

## 5.5 Qdrant service in `node-b.nix`
See 4.1 (`services.qdrant`, `nix/node-b.nix:275-299`). The firewall comment "firewall already blocks 6333 externally" would also go.

## 5.6 Things checked and left alone
- Timers `go-choir-checkpointd` (every minute), `go-choir-disk-gc`, `go-choir-platform-dolt-history-audit` are all `wantedBy timers.target` and have live units.
- Guest units `go-choir-verifier-signer`, `go-choir-guest-receipt-signer*`, `go-choir-kernel-capability-probe`, `go-choir-updater`, and the restart/recovery path units are live (verifier signer is mounted at `autoputer/run.go:1040` and listed in `boot_timeline.go:153`).
- `searx` (used by `content.go:355-370` and the gateway `SearXNGProvider`, [I] gateway side not read).

---

# Not examined (limits)
- No production traces were read, so no claim here says a model did or did not call a verb; item 1.8 and the "[I]" tags depend on traces.
- `scripts/` one-off report generators (`generate_*_pdf*.py`, 16 files) and probe scripts (`s0m_*`, `m*_probe.mjs`) were not audited for dead references; Cut 0's `docs/definitions/choir-reeng-cut0-eyes` was not read. `frontend/` listeners for retired events beyond those named in 3.2 were not enumerated. `internal/provider` (`CHATGPT_*`) and `cmd/zot` env vars were checked for reads only.
- `docs/` truth drift is outside this scope.
