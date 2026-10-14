# G — Affordance audit and runtime-minimalism pass (2026-10-12)

Read-only research against `main` at `a401c878`. Nothing in the repo was edited.
Scope: owner invariants **I1** (every desk's prompt and module descriptions describe its real affordances)
and **I2** (runtime minimalism: no budgets, round caps, runtime scoring, or runtime-owned
research job; decisions move to RLMs and models). Management and engineering are covered in
depth; Texture and research get a short mismatch list because they are being studied separately.

Tags: **[V]** means verified by reading the code at the cited lines. **[I]** means inferred:
a reasonable reading that was not traced end to end or not run. Line numbers are from `a401c878`.

---

## 0. How a desk's prompt is assembled (the map you need for I1)

`systemPromptForRun` (`internal/agentcore/tool_profiles.go:195-310`) concatenates, in this order [V]:

1. **Core prompt:** `internal/promptstore/defaults/core.yaml` (owner-overridable via promptStore).
2. **Temporal context:** `runtimeprompts/overlays/temporal_context.yaml`.
3. **Role prompt** (owner-overridable): `promptstore/defaults/{management,engineering,research}.yaml`. Texture's comes from `textureprompts/texture.yaml`.
4. **Skill context** (`skill_context.go`).
5. **Per-desk overlay:**
   - Texture: `textureprompts/overlays/run_system.yaml`, plus inline strings at `tool_profiles.go:247-253` (the self-development supervision paragraph and "Lifecycle Texture dispatch authority").
   - Management: `runtimeprompts/overlays/rlm_management_runtime.yaml` (`:255-257`).
   - Engineering: `rlm_engineering_runtime.yaml` with template flags `HasSelfDevelopmentOperation` and `CanProposeChange`, plus an inline assignment line (`:258-290`).
   - Research: `rlm_research_runtime.yaml` (`:291-295`).
6. **Generated surface block:** `deskSurfacePrompt` (`tool_profiles.go:568-584`) → `yaegikernel.DeskSurface` (`surface.go:211-214`). This lists every export of the desk's `ChoirScope` with the one-line summaries in `choirFunctionDocs` (`surface.go:28-75`). It is the only part generated from the real surface.
7. **Run context:** `runtimeprompts/overlays/run_context.yaml`, which has the `InCellCarrier` peer-coordination sentence.

Two more prompt sources reach a desk outside the system prompt:

- **Wake/activation user turns:**
  - `lifecycleControlActivationPromptForProfile` (`management_controller.go:1791-1797`) is live (called at `:2121` and `:2158`).
  - `createRunWithMetadata(buildAssignedWorkItemPromptForCarrier(...))` (`runtime.go:2922`).
  - Pending-update wake text is built in `coagent_update_packet.go:294-305`.
- **Tool and verb results:** text the runtime injects into results, such as `next_instruction` (see 1.4) and error hints from `cellErrorHint` (`surface.go:225`).

The actual verb set per desk is `deskModuleSets` (`internal/yaegikernel/choir.go:242-273`), plus universal observation verbs (`choir.go:294-303`: ReadFile, ListDir, Context, Inbox, Updates, Emits, Pack, Help) and verifier-slot-only Verify/InspectBundle (`choir.go:355-358`). The reducer decides what each staged act actually does (`internal/agentcore/rlm_reduce.go`).

**Structural root cause of most I1 drift [V]:** the hand-written overlays duplicate what step 6 already generates, and the module sets were never pruned when acts were retired. There is one partial guard: `TestPromptTaughtVerbsMatchExports` (`yaegikernel/choir_parity_test.go:310`) checks that a verb a prompt teaches is exported. It does not check the reverse (exported but not taught or meant), and it does not check argument shapes, addressing rules, or reducer acceptance.

---

## 1. Affordance mismatches per desk

### 1.1 Management (in depth)

The live surface comes from `deskModuleSets["management"]` (`choir.go:245-247`): Message, Emit, Cast, Ask, Note, Reply, CancelAct, CancelAssignment, Escalate, Precommit, Report, ReportPacket, Resolve, Disagreement, ProductAPI, plus the universal verbs.

| # | Prompt claim (file:line) | Reality (file:line) | Tag |
|---|---|---|---|
| M1 | Assignments open on the document channel: "an owner-authored revision on an engineering-bound document is the cast; the runtime opens the assignment directly" (`promptstore/defaults/management.yaml:11`). The overlay contradicts itself: "An owner-authored revision on an engineering-bound document is the cast trigger, and a management cell stages choir.Cast" (`rlm_management_runtime.yaml:28`). | Two real paths exist: management's own `choir.Cast("engineering")` → `openDelegatedCastAssignment` (`rlm_reduce.go:1936-1959`, `engineering_assignment_runtime.go:455`), and the separate document-cast path that management takes no part in (`engineering_desk.go:99-314`, `selfdev_texture_join.go:41-117`). The prompt describes the path the owner wants dissolved, and describes it as management's trigger when management never sees it. | V |
| M2 | "Tell Texture … with choir.Report("texture", claim, evidenceRefs, "texture"). The toDesk is the profile name "texture", never "texture:<doc>"" (`management.yaml:17`). | For the persistent management run, **toDesk is ignored**. `commitLifecycleReportActIntent` branches on `AgentID == persistentManagementAgentID && TrajectoryID == ""` and calls `persistentManagementBoundReport` without reading `in.ToDesk` (`rlm_reduce.go:1975-2001`). The target is whatever delivered control the run is bound to. On a drain carrier it is "the trajectory of the most recent delivered control" (`tools_engineering_assignment.go:147-169`). Management cannot choose which Texture it reports to. | V (consequence for multi-trajectory drains: I) |
| M3 | Report the result "with choir.Report … and end the turn" (`management.yaml:17,21`). The overlay says "choosing the disposition that matches whether the controlled obligation is actually terminal" but never names the field (`rlm_management_runtime.yaml:29`). | Work settles only through `work_disposition` on a **ReportPacket** packet (`types/evidence.go:68-71`). A claim-only `choir.Report` builds a packet without it (`rlm_reduce.go:1987-1994`), and an empty disposition defaults to `open` (`tools_engineering_assignment.go:134-136`). Management is never told how to complete Texture's work item. | V (that work stays open in practice: I) |
| M4 | The live wake turn says: "use choir.Report to assert work_disposition=completed … Every choir.Report must set work_item_id" (`runtime.go:2957-2963`, via `management_controller.go:1793` and `runtime.go:2922`). | `choir.Report(toDesk, claim, evidenceRefs, resolverID)` has neither parameter (`choir.go:936`). `work_item_id` does not exist on the packet (`types/evidence.go:60-72`), and a report whose actions carry `workitemid` is **rejected** (`tools_engineering_assignment.go:67-72`). It also says "Use choir.Cast to open assignments", which is sent to research too, and research does not cast (see R2). | V |
| M5 | The generated surface lists `choir.Message` and `choir.Emit` (summary "Retired: returns an error…", `surface.go:40-41`) and `choir.Emits` ("Emitted signals addressed to you", `surface.go:36`). | Message and Emit are stubs that always error (`choir.go:445-451`). No live producer of emits remains, so Emits only drains legacy rows (`tools_desk.go:532-575`). Three of the listed names are dead surface. | V (Emits always empty: I) |
| M6 | "Observation: choir.Context …" (`rlm_management_runtime.yaml:17`). The surface summary says Context returns "Your activation identity (agent, run, desk, doc and assignment ids)" (`surface.go:29`). | `Context()` returns only `computer_id`, `activation_id`, `co_super_slot` (`choir.go:779-788`). This is wrong for every desk. | V |
| M7 | "Send at most one desk_go_eval call per turn … batching multiple eval calls in one turn is refused and none run" (`rlm_management_runtime.yaml:13`; same text in the research overlay at `:14`). | Only `capsule_go_eval` batches are refused (`toolregistry/batch_executor.go:33, 154-165`). `desk_go_eval` batches run **sequentially** (`batch_executor.go:147`, `:44-49`). | V (provider-level parallel-call settings not checked) |
| M8 | "Management does not touch the filesystem" alongside "Observation: choir.ReadFile, choir.ListDir" (`rlm_management_runtime.yaml:16-17`). | ReadFile and ListDir are universal exports (`choir.go:295-296`), rooted at the run's `WorkingDir`, or else `os.TempDir()` (`tools_desk.go:645-650`). What management can read on the host is not stated anywhere. | V (actual root at runtime: I) |
| M9 | ProductAPI allowlist includes `/api/computers/` (`rlm_management_runtime.yaml:21`). | Every `/api/computers/*` route requires an `X-Authenticated-Computer` header (`api_self_development.go:84-97, 107-110`). `productAPIRequest` sets only `X-Authenticated-User` and `-Email` (`tools_product_api.go:67-71`), so **every** management call there returns 403. The allowlist entry (`tools_product_api.go:190`) is a dead affordance. **Correction:** this also means management *cannot* start self-development operations through ProductAPI, contrary to `docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md` and to a subagent's first pass. | V |
| M10 | "GET only against /api/texture/" (`rlm_management_runtime.yaml:21`). | `POST …/merge-preview` is also allowed (`tools_product_api.go:181-182`). This is benign but undocumented. | V |
| M11 | "choir.CancelAssignment(target_ref, reason)" (`rlm_management_runtime.yaml:30`). | The signature is `(assignmentID, reason)` (`surface.go:54`, `choir.go:905`). "document-parent assignments are cancelled by the runtime" (`management.yaml:11`) is true only because the document path exists (M1). | V |
| M12 | Coordination verbs Ask, Note, Reply, Escalate, Resolve, Disagreement, CancelAct, and Precommit "{question, distribution, resolver}" (`rlm_management_runtime.yaml:19-20`). | Ask, Note and Escalate from management resolve non-management addressees to `<profile>:<scope.ChannelID>` (`rlm_reduce.go:1770-1790`). The persistent management channel is not a document, so an Ask or Note to `texture` or `research` from management resolves to a non-existent agent and errors (`rlm_reduce.go:1863-1866`). Precommit and Resolve only record (no scorer exists). Nothing in management's prompt says when to use them. | Addressing: I (high). Record-only: V |
| M13 | "claim is one sentence of what you committed to, not a result" (`management.yaml:17`). | The reducer turns the claim into `Summary` and `Claims[0].Text` and Texture receives it as the update's `HumanProjection` (`rlm_reduce.go:1987-1994`, `tools_desk.go:509`). This is a style rule, not an affordance error, but it is the reason Texture sees "I cast X" instead of results. | V |
| M14 | Dead wake prompt `buildPersistentManagementUpdatePrompt` says every packet "is a validated packet.kind=execution_request" and calls management the "persistent super actor" (`management_controller.go:1724-1748`). | It has no non-test caller (grep), so it is dead code. Delete it. | V |
| M15 | "Your turn waits for the cast" (`management.yaml:21`). | True: management parks with `awaiting_delegated_cast` (`management_controller.go:2657-2708`). | V (no mismatch) |
| M16 | The role policy shown on the owner's prompt-settings page reports management `AllowResearchTools`, `AllowEvidenceTools`, `AllowModelDiagnosticTools`, `AllowedSpawnTargets:[research]` (`agentprofile.go:83-93`, surfaced by `prompts.go:124-150`). | None of these exist for the management cell desk (`choir.go:245-247`). They only build a shadowed legacy registry (`tool_profiles.go:402-440`, `:465-470`) that is also `rt.toolRegistry`, the fallback for unknown profiles (`:526`, `:550-555`). The owner-facing description is wrong. | V |

### 1.2 Engineering (in depth)

The live surface is `deskModuleSets["engineering"]` (`choir.go:249-251`): WriteFile, Exec, **Assign**, Message, Emit, Complete, Freeze, Cast, Ask, Note, Reply, CancelAct, Escalate, Precommit, Report, ReportPacket, Resolve, Disagreement; plus Verify and InspectBundle on the verifier slot.

| # | Prompt claim (file:line) | Reality (file:line) | Tag |
|---|---|---|---|
| E1 | "Synchronous Syscalls (immediate effects): … choir.Pack, **choir.Assign**" (`rlm_engineering_runtime.yaml:49`). The surface says "Records a worker assignment and returns its receipt" (`surface.go:42`). | Assign spawns nothing and persists nothing. `handleAssign` puts the payload in an in-memory map and returns `Status:"dispatched"` (`yaegikernel/broker.go:356-377`). The resulting receipt `assign/<id>/<task>` is mixed into `staged_intent_ids` (`broker.go:113-131`, `cmd/capsule-broker/session_worker.go:461`). It is a false affordance (see §3). | V |
| E2 | Staged intent tickets include `"rlm:message:1"` (`rlm_engineering_runtime.yaml:41`). | Message is retired (`choir.go:445`), and Assign receipts are the other thing that shows up there. | V |
| E3 | "choir.ReportPacket(to, packet, "") addresses one agent by **exact durable agent_id**" (`:55`); "addressing only the requesting Management **agent_id**" (`:60`). | The reducer requires a bare desk name. `needTo` runs `agentprofile.Canonical(to)`, which rejects `management:<owner>` (`rlm_reduce.go:166-172`, `agentprofile.go:122-134`). Following the prompt literally fails the cell at commit. The research overlay states the correct rule (bare desk name, `rlm_research_runtime.yaml:20`). | V |
| E4 | The synchronous-syscalls list omits Cast, Ask, Note, Reply, Escalate, Precommit, Resolve, Disagreement and CancelAct. The overlay says "Never start another agent" (`:60`). `run_context.yaml:17` (`InCellCarrier` is true for engineering, `tool_profiles.go:307`) tells engineering to "Coordinate with peers through addressed acts (choir.ReportPacket, choir.Note, **choir.Cast**, choir.Ask)". | All of these are exported to engineering (`choir.go:249-251`). `castOpensEngineeringAssignment` has no caster-profile gate (`rlm_reduce.go:1796-1800`), and `openDelegatedCastAssignment` checks only for a trajectory-bound caster with an open work item (`engineering_assignment_runtime.go:455-484`). The owner rule "engineering never addresses Texture directly" is enforced for Note (`CanMessage`, `rlm_reduce.go:1871-1877`; `agentprofile.go:81`) and for Report (authority check, `tools_worker_update.go:182, 307`), but **not** for Cast or Ask. | Exports: V. Engineering Cast/Ask reaching Texture or a second engineering: I |
| E5 | The surface summary for Context (`surface.go:29`) promises agent, run, desk, doc and assignment ids. The overlay example reads `ctx["co_super_slot"]` (`:21`). | Only `computer_id`, `activation_id`, `co_super_slot` are returned (`choir.go:779-788`). The assignment id is available only in the system prompt line (`tool_profiles.go:276-289`). | V |
| E6 | "choir.Freeze stages the **self-development** freeze" (`:54`), plus template branches `HasSelfDevelopmentOperation` and `CanProposeChange` (`:60`; computed at `tool_profiles.go:259-272` by looking up selfdev operations and the mode). | The freeze mechanics are accurate (`capsule/executor.go:1621` has the "no frozen runtime artifacts" error). The self-development framing is concept scaffolding (§5). The branch text changes what engineering is told based on a runtime lookup of a selfdev operation, which is a runtime routing decision about the prompt. | V |
| E7 | "A verification capsule is offline" (`:9`); `engineering.yaml:10` says the same. | Network is set by the cast (`castWithholdsNetwork`, `rlm_reduce.go:946`) and kept on recast (`engineering_desk.go:258-259`). It is consistent. | V (no mismatch) |
| E8 | Quotas: 16 intents, 16 KiB, one Complete, one Freeze (`:53`). | `MaxIntentsPerCell=16`, `MaxIntentBody=16<<10` (`intent.go:86-87`). Complete is checked (`rlm_reduce.go:111-121, 151-153`). **No one-Freeze-per-cell check exists in `validateCellIntents`** (`rlm_reduce.go:123-126`). | Quotas: V. Freeze count: V (no check in reducer; the tray may check: I) |
| E9 | "Report progress to Management with choir.ReportPacket" (`:60`). | For **document-cast** assignments the parent is `engineering:<doc>`, and the store retargets the report to `texture:<doc>` (`store/engineering_assignments.go:1747-1757, 1778-1823`). Engineering's report then reaches Texture directly, which the prompt does not say and the owner forbids. | V |
| E10 | `engineering.yaml:14`: "A verification pass requires a successful receipt bound to the exact frozen final subject." | Consistent with Verify validation (`rlm_reduce.go:127-136`). | V |

### 1.3 Texture (brief)

- **T1. Research round cap** [V]. "Each owner request … allows a bounded number of research rounds; the runtime drops further rounds" (`tool_profiles.go:252`), and "Research rounds are bounded" (`run_system.yaml:10, 31`). The cap is `textureResearchRoundsPerOwnerRequest = 4` (`textureowner/texture_turn_runtime.go:161`), enforced at `:295-305`, with a runtime note at `:405`. This is an I2 violation; delete it (§2).
- **T2. Self-development supervision paragraph** [V] (`tool_profiles.go:247-249`). "The runtime runs the operation … Do not send controls to verify, approve…". This is concept scaffolding (§5).
- **T3. Dead surface** [V]. The generated surface lists Message, Emit, Emits, and Report, ReportPacket, Precommit, Resolve, Disagreement, Escalate, Reply, CancelAct for Texture (`choir.go:268-270`). The Texture prompts never say what a Texture Report does. `foldTextureDispatch` folds only Cast, Ask and Note (`rlm_reduce.go:813-826`).
- **T4. Missing op in the surface summary** [V]. `ApplyTexture` lists ops apply and decide (`surface.go:39`), but op `email` exists (`texture_cell_author.go:24, 86`) and is taught in `run_system.yaml:23` and `revision_policy.yaml:31, 63`.
- **T5. One interim checkpoint per document, then decide turns** (`run_system.yaml:31`). If this is enforced in `textureowner`, it is a runtime turn-policy decision; check it in the Texture study [I].
- **T6. Dead text** [V]. `buildTextureMediaSourceResearchObjective` (`texture_media_sources.go:886-889`) names `read_content_item`, `import_url_content` and `update_coagent`, and has no callers. Delete it.
- **T7. Conductor** [V] (not a desk, listed for completeness). It still teaches `patch_texture` (`promptstore/defaults/conductor.yaml:16`; `conductor_run.yaml:13`). No `patch_texture` tool is registered anywhere (grep shows only `spawn_agent` at `coagentowner/spawn_tool.go:46`).

### 1.4 Research (brief)

- **R1. `next_instruction` tells research to call a tool that doesn't exist** [code path V, live effect I-high]. Research cells reach `researchDeps.HostEgress` (`tools_desk.go:308-325`), which reuses the legacy tool funcs (`researchtools/host_egress.go:22-80`). `shouldRequireResearchUpdateAfterTool` (`researchtools.go:717-738`) counts `update_coagent` and `web_search` *tool events*. A desk run has only `desk_go_eval` events, so it returns true, and WebSearch, SourceSearch and FetchURL results get `next_instruction: "Submit a concise first update_coagent source packet…"` (`researchtools.go:787, 924, 995, 1023`). This is an I1 violation (a non-existent tool) and an I2 violation (runtime-authored instruction).
- **R2. Cast is exported but the prompt says research doesn't cast** [V]. Research's module set includes Cast (`choir.go:254`), but the overlay says "research does not cast work to other desks" (`rlm_research_runtime.yaml:19`). The wake turn tells research "Use choir.Cast to open assignments" (`runtime.go:2957`).
- **R3. Budget language** [V]. The prompt mentions budgets (`research.yaml:44-46`; overlay `:18`). The ledger is `DefaultResearchEgressMaxCalls=64` and `MaxFetchedBytes=32MiB` (`researchtools/egress_budget.go:46-47`), installed at `tool_profiles.go:476-481`. Under I2 the owner rejects budgets (§2).
- **R4. Overlay list is incomplete** [V]. The overlay's world-access list omits `choir.Research`, `PollSource` and `ReadCapture` (`:18`), though the generated surface and `research.yaml:41` cover Research.
- **R5. Same as M4** [V]. The work_item_id/work_disposition-on-Report error also hits research.
- **R6. Same as M7** [V]. The batching claim is wrong for research too.

### 1.5 Cross-desk

- **X1. Help example** [V]. The Help example `choir.Help("ApplyTexture")` (`surface.go:28`) is shown to every desk. On non-Texture desks it returns "There is no choir.ApplyTexture for this desk".
- **X2. `update_coagent` vocabulary in live wake text** [V]. `coagentUpdateInstruction` (`coagent_update_packet.go:294-305`), `trajectory.go:114`, and the error at `runtime.go:1556` still say `update_coagent`. The tool no longer exists; it now means "records", which is confusing.
- **X3. Dead non-carrier branch** [V]. `buildAssignedWorkItemPrompt` (non-carrier, "call update_coagent") is reachable only from `buildCoagentBacklogPrompt` (`management_controller.go:2244-2250`), which has no non-test callers. Delete it.
- **X4. Retired-verb stubs** [V]. Message and Emit (and the reducer's `IntentMessage` branches at `rlm_reduce.go:104-110, 869-883`, kept "for one release" per `choir.go:440-442`) should leave the module sets now. Otherwise the generated surface keeps advertising them.

---

## 2. Runtime-owned intelligence inventory

Verdicts:

- **Keep**: substrate. Delivery, idempotency, authority boundaries, schema well-formedness, crash safety.
- **Model**: move the decision into an RLM or model, or into a field the model declares.
- **Delete**: remove.

I spot-checked items 1, 9, 10, 21, 33, 44 and 47 at the cited lines; the rest come from a dedicated read-only pass, and their tags are its own.

**Budgets and caps**

| # | Location | What the runtime decides | Verdict |
|---|---|---|---|
| 1 | `textureowner/texture_turn_runtime.go:161,295-309,405` | Caps research rounds at 4 per owner request, silently drops further research controls, and writes a `[runtime: … spent]` note. | **Delete** [V] |
| 2 | `agentcore/tool_profiles.go:252`; `run_system.yaml:10,31` | The prompt text for #1. | **Delete** [V] |
| 3 | `researchtools/egress_budget.go:46-47`; installed `tool_profiles.go:476-481` | Egress budget of 64 calls and 32 MiB per activation. | **Delete** [V]; keep the SSRF/address guards |
| 4 | `agentcore/research_children.go:206,243` | Child research charges the root activation's budget. | **Delete** with #3 [V] |
| 5 | `research_children.go:45,47,149,231` | A 10-minute call window, with each child getting 0.75 of the parent's remaining time. | **Model** [V] |
| 6 | `research_children.go:217,236` | Runtime-written "You have about X … findings are lost" text in child prompts. | **Delete** [V] |
| 7 | `research_children.go:36` | A 1.5 s computer-wide model-call spacer (gateway 429 shaping). | Keep, but revisit; it moves to the gateway [V] |
| 8 | `research_children.go:23` | Concurrency 4. It queues and never refuses. | Keep [V] |
| 9 | `agentcore/runtime.go:4105-4130` | Texture tool-loop budget: 8000 provider calls, 120M tokens, 45 min. | **Delete** [V] |
| 10 | `runtime.go:4134-4152` | Engineering tool-loop budget: 20000 calls, 60 min. | **Delete** [V]; a stuck-loop breaker comes later if needed |
| 11 | `runtime.go:4156-4185`; `textureowner/texture_agent_revision.go:517-533`; `texture_controller.go:1040-1045`; `texture_lifecycle_api.go:192` | Budget overrides and spend carried across activations. | **Delete** with #9 and #10 [V] |
| 12, 13 | `runtime.go:4195`; `textureowner/tool_loop_policy.go:12,32-82` | Duplicate budget and tool-forcing helpers that only tests use. | **Delete** [V] |
| 14 | `runtime.go:374`; `provideriface/config.go:24`; `continuation_schedule.go:411-416` | A 60-minute ActivationBudget, enforced by a durable deadline. | Keep for now, relabeled as a breaker (the only stop for a hung provider) [I] |
| 15 | `runtime.go:1509-1530` | `maxManagementActiveEngineeringSlots = 2` on the retired co-super spawn path. | **Delete** [V] |
| 16 | `researchtools/researchtools.go:656,685-691` | `webSearchAgentResultFloor=40` overrides the model's `max_results`. | **Model** [V] |
| 17 | `researchtools.go:663` | The "Research cadence" procedure written into the `web_search` description. | **Delete** [V] |
| 18 | `toolregistry/toolloop.go:121,123` | `maxCompletionGuardRetries=2`, `maxToolLoopParkResumes=16`. | Keep [V] |
| 19 | `yaegikernel/intent.go:86-88`; `rlm_reduce.go:98-110` | Tray quotas. | Keep [V] |
| 20 | `runtime.go:2983-2990` | Renders `step_budget`/`token_budget` that nothing sets. | **Delete** [I] |

**Retries, watchdogs and breakers**

| # | Location | What the runtime decides | Verdict |
|---|---|---|---|
| 21 | `textureowner/texture_controller.go:623-633,820` | Texture breaker B6: after 3 dead activations, update-driven dispatch stops. | **Delete**: a preemptive breaker, which the owner deferred [V] |
| 22, 23 | `texture_controller.go:765,807`; `management_controller.go:1142,1240` | Poison-message cap: 3 delivery attempts. | Keep [V] |
| 24 | `actor_wake_projector.go:17-18,52-66` | Wake dispatch retry with backoff. | Keep [V] |
| 25, 26 | `management_controller.go:529,540-700` | Resume and fresh-mint watchdogs. | Keep as delivery recovery; they patch a dispatch symptom, so revisit after a substrate fix [V] |
| 27 | `engineering_desk.go:50,213-279`; reason written at `engineering_assignment_fate.go:541` | Restart-recast re-opens a restart-cancelled assignment, up to 3 attempts, with **no planned-restart marker check**. | **Delete** (with path B) or gate on the marker. It looks like a live violation of the crash-restart rule [V code; I trigger timing] |
| 28 | `runtime.go:3474-3497` | Engineering completion guard: injects a "Finish the assignment … choir.Freeze … choir.Complete" reminder, retries, then fails. | **Split**: keep the mechanical fate, delete the injected instruction [V] |
| 29 | `engineering_assignment_runtime.go:893-940` | `reclaimSupersededAssignmentCapsules`: a new assignment auto-cancels other live ones. | **Model**: refuse at capacity and let management choose [I] |
| 30 | `engineering_assignment_fate.go:76-105` | A 5-minute activation grace before a capsule counts as unusable. | Keep [V] |
| 32 | `gatewayruntime/provider.go:201-212` | 429 and transient retry. | Keep [V] |

**Scoring, judgement and validation**

| # | Location | What the runtime decides | Verdict |
|---|---|---|---|
| 33 | `agentcore/rlm_reduce.go:1233-1247` (used `:1185`) | `resolveOutcomeDiscrepancy` maps free-text verdicts through a synonym table ("mostly"→qualified, "held"→confirmed, anything else→unresolved). | **Model**: the resolver emits the typed class and the reducer checks the enum [V] |
| 34 | `selfdev_candidate_judgment.go:40-73,93-128` | Three Jev questions fixed in the runtime, with an all-yes, p≥0.5 pass rule. | **Model**: management owns whether and how to consult Jev. Jev was ratified on 2026-10-10, so this needs the owner's call, which the 2026-10-12 decision "Jev is part of management" supplies [V] |
| 35 | `engineering_desk.go:401-407`; prompt at `tool_profiles.go:249` | The runtime auto-chains frozen → Jev → awaiting approval ("The runtime runs the operation"). | **Delete** the auto-chain; management Casts or consults [V] |
| 36 | `tools_capsule.go:455,531-544`; `types/commitment_views.go:584` | `learningClaimGate` flags only. | Keep [V] |
| 37 | `types/commitment_views.go:385-429`; used `texture_evidence_sources.go:859,867` | Materiality projection with a fixed 72-hour "overdue". | Keep the projection; the threshold goes to the **Model** [V] |
| 38 | `decisionpolicy/reduce.go:33-200` | Quorum counting over attested ballots. | Keep: an authority gate; the judgement is in the ballots [V] |
| 39 | `rlm_reduce.go:97-235`; `coagentpacket/packet.go:118-176`; `yaegikernel/report_packet.go:23` | Schema validation. | Keep. `packet.go:132` ("at least one of claims/sources/…") is borderline [V] |
| 40 | `rlm_reduce.go:1564-1575` | Coerces an implementation verdict to "none". | Keep [V] |
| 41 | `run_acceptance.go:779-930` | Acceptance-level derivation as a diagnostic projection. | Keep [I] |
| — | `tools_engineering_assignment.go:67-72` | `rejectReportAuthorityInputs`: identity fields are forbidden in report actions. | Keep: authority boundary [V] |

**Routing guesses**

| # | Location | What the runtime decides | Verdict |
|---|---|---|---|
| 42 | `texture_turn_runtime.go:208,348` | Merges research openers and continues the request's research work. | Keep: the one-research-desk design [I] |
| 43 | `texture_turn_runtime.go:321` | A management Cast continues the latest open management work. | **Model** [I] |
| 44 | `agentcore/texture_dispatch_fold.go:55` | One management Cast per Texture turn. | **Delete** [V] |
| 45 | `rlm_reduce.go:1770,1796`; `engineering_assignment_runtime.go:246` | Addressing; `castWithholdsNetwork` is parsed from a model-declared field. | Keep [V] |
| 46 | `textureowner/coagent_route.go:578-606,243` | Keyword-based article-style classifier. Reachable only through frozen handoffs. | **Delete** [I] |
| 47 | `textureowner/texture_agent_revision.go:622-662,803-818` | Regexes infer "hard requirements" from the prompt and inject a checklist. This is live via `submitTextureAgentRevisionRun`. | **Delete** [V] |
| — | `tools_engineering_assignment.go:147-169` | A drain-carrier management report goes to "the trajectory of the most recent delivered control". | **Delete**: derive the target from the binding (§4) [V] |

**Runtime-written instructions and forced decisions**

| # | Location | What the runtime decides | Verdict |
|---|---|---|---|
| 48 | `tools_desk.go:815-865` (`answerIdleTextureTrigger`) | The runtime commits an `op:"decide"` in Texture's name when an activation applied nothing. | Keep the mechanism, but record it as a runtime fate rather than a Texture decision [I] |
| 49 | `coagent_update_packet.go:118-119,200-210,294-305`; `runtime.go:2955-2965`; `management_controller.go:1791-1797` | Wake preambles with imperatives, some of them wrong (M4). | **Model**: wake turns carry pointers only, and the desk prompts teach the procedure [V] |
| — | `researchtools/researchtools.go:717-738,783-788,924,995,1023` | `next_instruction` "Submit … update_coagent" injected into research cell results. | **Delete** [V path, I live effect] |
| — | `store/engineering_assignments.go:1702-1717,2706-2712,~2361` | Runtime-authored report kind, notes, and the "progress overdue" and cancel summaries. | Keep as factual observations; never phrase them as instructions [V] |
| — | `tool_profiles.go:247-249,259-272` | Prompt branches on runtime lookups of a selfdev operation. | **Delete** (§5) [V] |

**Dead code**

| # | Location | Verdict |
|---|---|---|
| 50 | `tools_coagent.go:143` / `runtime.go:3532` `awaitRequiredChildRuns`: no writer of `required_child_runs` exists | **Delete** [V] |
| 51 | `runtime.go:1158-1182,1205-1220`: engineering slot claim and `enforceManagementVerifierSequencing`, unreachable since `:1135` refuses engineering | **Delete** [V] |
| 52 | `cycle/synthesize.go`, `cycle/cycle.go:123`, `textureowner/texture_workflow_verifier.go:33`: test-only | **Delete** [V] |
| — | `management_controller.go:1724-1748` `buildPersistentManagementUpdatePrompt`; `:2244-2250` `buildCoagentBacklogPrompt` and `runtime.go:2949` `buildAssignedWorkItemPrompt`; `textureowner/texture_media_sources.go:886` | **Delete** [V] |

**Checked and fine as substrate** [subagent, V]:

- maintenance hold (`runtime.go:789-795,892`)
- Texture audit record (`texture_audit.go:35`)
- obligations surface (`obligations_surface.go:157`)
- modelpolicy
- workitem fingerprints
- research provider-admission authority (`runtime.go:3007`)
- sweep pacing (`runtime.go:2617`)
- prompt-bar URL classifier (`content/content.go:1698`). This is borderline; it is conductor-level.

**I2 additions to the §6 deletion list:**

- #9–#13 (tool-loop budgets): orange.
- #21 (B6 breaker): orange.
- #33 (synonym table): red, because it is the evidence ledger.
- #44 (one management Cast per turn): orange.
- #47 (regex checklist): orange.
- #28 (injected reminder text): orange.
- #50–52 and the dead prompt builders: green/yellow.

---

## 3. `choir.Assign`: what it touches and how to delete it

Verified by a dedicated pass and spot-checked [V].

**What it is:** an in-process stub.

- `ChoirScope.Assign` (`choir.go:426-435`) → `s.call(ActionAssign)` → in-worker `Broker.HandleRequest` (`broker.go:141-167`) → `handleAssign` (`broker.go:356-377`).
- `handleAssign` makes a random id, stores the payload in `b.assignments` (`broker.go:48, 76, 364`), and returns `Status:"dispatched"`.
- After the cell, `DrainReceipts` (`broker.go:113-131`, via `session_loop.go:148-149`) turns each entry into `assign/<id>/<taskID>` in `SessionResult.Receipts`.
- On the capsule path that becomes `GoEvalResult.StagedIntentIDs` (`cmd/capsule-broker/session_worker.go:461`, `capsule/types.go:145`) and is shown to the model next to the real `rlm:*` ids (`tools_capsule.go:742`).
- **No store row, work item, run, tray intent (there is no `IntentAssign`), or spawn.** Nothing downstream reads it.

**Who can reach it:**

- Engineering only (`choir.go:250`). `mutateDenied("Assign")` is a no-op (`choir.go:375-376`).
- `normalizeDeskRole` maps unknown roles to engineering (`choir.go:132-138`), so they get it too.
- Management and Texture handles authorize `ActionAssign` (`choir.go:95`) but do not export it.

**Deletion checklist** (one commit, because parity tests couple these):

- `choir.go`:
  - `:95`: drop `ActionAssign` from handle actions.
  - `:241` comment.
  - `:250`: drop `"Assign"` from the engineering module set.
  - `:307`: drop it from the verbs map.
  - `:425-435`: delete the method.
- `broker_protocol.go`:
  - `:19`: delete `ActionAssign`.
  - `:115-125`: delete `AssignPayload` and `AssignResult`.
  - `:295`: drop it from validation.
- `broker.go`:
  - `:48, :76`: delete the `assignments` map.
  - `:113-131`: delete the assign loop in `DrainReceipts`.
  - `:166-167`: delete the case.
  - `:356-377`: delete `handleAssign`.
- `surface.go:42`: delete the doc entry. Required in the same commit by `TestSurfaceDocsMatchExportsAndSource` (`surface_test.go:76-102`).
- `rlm_engineering_runtime.yaml:49`: drop `choir.Assign`. Required in the same commit by `TestPromptTaughtVerbsMatchExports` (`choir_parity_test.go:310, 332`).
- Tests:
  - `choir_parity_test.go:86-96`: Assign half of `TestChoirParityAssignMessageReceipts`. Keep the Message refusal check.
  - `choir_parity_test.go:187, 211-212, 246-248`.
  - `broker_test.go:79`.

**Optional cascade:**

- Message has been a never-calling stub since P2c (`choir.go:445-447`), so `ActionMessage`, `MessagePayload`, `broker.go:49, 77, 128-131, 168-169, 379-397`, and the whole `DrainReceipts`/`drain` plumbing (`session_loop.go:89-149, 306-338`; `sidecar.go:208, 314`) become test-only.
- `yaegikernel/actor_state.go` (205 LOC) and its test have no external references.

**Must NOT delete:**

- `SessionResult.Receipts` and `GoEvalResult.StagedIntentIDs`: they carry the real `rlm:<kind>:<seq>` receipts.
- `CancelAssignment` and the `EngineeringAssignment` store, types, and Cast → `openDelegatedCastAssignment`. That is the real assignment system and only shares the word.
- `cmd/runtime-ratchet/inventory.go:348`: `x.Assign` there is a Go AST field.

**Correction to `docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md`:** engineering's receipt path is the capsule broker (`session_worker.go:461`), not `tools_desk.go`.

**Class and rollback:** yellow/orange (worker action set, prompt, tests). No durable state to migrate. Rollback is a git revert. Not docs-only, so the full landing loop applies.

---

## 4. Jev and the management path today

**What Jev is** [V]:

- A gateway-routed decision model, `typesafe/jev-1.13`, behind OpenRouter's Decisions endpoint (`internal/gateway/jev.go:19-23`), exposed at `POST /provider/v1/judgments` (`jev.go:233-300`), with a guest client at `internal/gatewayruntime/jev.go:46-81`.
- It has **one caller**: `judgeFrozenCandidate` (`agentcore/selfdev_candidate_judgment.go:185-218`). That runs host-side inside `reconcileEngineeringVerification` (`engineering_desk.go:317-410`, call at `:405`) and is bound at boot (`autoputer/run.go:600-605`).
- It asks three fixed yes/no questions (`does_what_was_asked`, `limited_to_the_objective`, `reversible`; `selfdev_candidate_judgment.go:42-73`). It passes only if every answer is "yes" with p ≥ 0.5 (`:93-128`). The verdict is written as the selfdev verification event.
- **No desk can call Jev** (it is in no module set). **Jev does not run "inside management" today**: it is a runtime-called judge on the document-cast/selfdev path.

**Hold** [V]:

- **No "hold" state exists in code** for Jev or report routing; greps for hold, held and withhold find nothing relevant. "Hold" exists only in docs (master report line 86; the routing review's forward/restate/hold proposal, lines 488-505).
- The owner's decision removes it from the design. There is nothing in code to delete for this.
- `RUNTIME_MAINTENANCE_HOLD` (`runtime.go:739, 789-800, 891-896`) is an unrelated host fence.

**How engineering reports reach Texture today** [V]. There are two paths.

- **Path A, delegated cast. This is already the target shape, minus pass-through.**
  1. Texture `Cast("management")` → control.
  2. Management `Cast("engineering")` → `openDelegatedCastAssignment` (parent is management; `engineering_assignment_runtime.go:545-546`).
  3. Engineering `Complete` → `commitCompleteIntent` (`rlm_reduce.go:1552-1617`) → `recordAssignedEngineeringReport` (`engineering_assignment_fate.go:602`) → `store.RecordEngineeringAssignmentReport` (`store/engineering_assignments.go:2029`), which builds the return packet to `Binding.ParentAgentID` (`:1719-1762`).
  4. The packet lands in management's `choir.Updates()`.
  5. Management `Report`/`ReportPacket` → `persistentManagementBoundReport` (`tools_engineering_assignment.go:123-298`) → `QueueLifecycleUpdate` → Texture's `choir.Updates()` (`tools_desk.go:470-512`).
- **Path B, document cast. Engineering reaches Texture directly, bypassing management.**
  - When the parent is `engineering:<doc>` with no parent run, the store retargets the packet to `texture:<doc>` and work item `work:texture-supervision:<doc>` (`store/engineering_assignments.go:1747-1757`).
  - `textureSupervisionSubject` (`:1778-1823`) mints that Texture agent and work item inside the commit. It is called from the report (`:2258`), cancel (`:2393`), and overdue observation (`:2722`) paths.
  - Texture-side support: `supervisionOnly` (`texture_controller.go:593-607`), the occurrence guard (`:1380-1390`), prefix exclusions (`texture_controller.go:71, 642`; `texture_agent_revision.go:238-241, 283`), the executor-before-supervisor wake rule (`store/lifecycle.go:858-870`), and the prompt paragraph (`tool_profiles.go:247-249`).
  - Entry points:
    - `POST /api/computers/{id}/self-development/operations` → `ensureSelfDevelopmentEngineeringDoc` (`selfdev_texture_join.go:41-117`). It creates a synthetic document whose first revision is marked `AuthorUser` (`:81`). Used by M11: `scripts/m11_selfdev_episode_probe.mjs:432, 534`.
    - Owner `POST /api/texture/lifecycle-documents` with `desk:"engineering"` (`texture_lifecycle_create.go:84-120`), plus later owner revisions via `ownerRevisionTarget` (`texture_agent_revision.go:232-257`).
    - The occurrence consumer `actorruntime/handler.go:602-629` → `ReconcileEngineeringRevisionCast` (`engineering_desk.go:99`).
    - The boot scan `ReconcileEngineeringDesk` (`texture_controller.go:110-125`).

**Runtime-authored text on the way** [V]:

- The engineering report packet: the runtime picks the kind (`execution_result` or `blocker`) and appends an `assignment_id=… attempt=… report_id=… late=…` note (`engineeringReportPacketPayload`, `store/engineering_assignments.go:1702-1717`).
- The runtime-authored "progress overdue: bound assignment … silent since …" observation (`:2706-2712`, triggered from `continuation_schedule.go:333-380`).
- The cancel summary "…; assignment cancelled; late results are evidence-only…" (around `:2361`).
- `HumanProjection` equals the content (`tools_desk.go:509`).

**Gaps that block the target shape:**

- **(a) The relay depends on the run** [I]. `persistentManagementBoundReport` requires one exact delivered control on *this* management run (`tools_engineering_assignment.go:166-169, 213-240`). An engineering result that arrives on a later management run may have no control to report against. The authority should be derived from the delegated assignment's `ParentControlID` and `ParentWorkItemID`.
- **(b) Jev is unreachable for delegated casts** [I]. The judgment chain is keyed by the admitting **revision** (`engineering_desk.go:200-209, 371`). For a delegated cast, `ParentControlID` is a commitment-record id, so a delegated-cast freeze probably never reaches Jev. No test drives it there.
- **(c) M2 and M3 above.** Management can't address a specific Texture and isn't taught to complete work.

**Minimal change: management as translator with pass-through, never hold.**

1. **Red: relay authority from the binding.** Resolve the Texture target and work item for a management report from the delegated assignment's parent control, not from "delivered to this run". This fixes (a) and makes toDesk meaningful (M2).
2. **Red/orange: one pass-through act.** Management either forwards engineering's packet unchanged (same summary, evidence refs and report ref) or restates it with its own `ReportPacket`. That is two outcomes, with no hold.
   - Enforce "never hold" structurally: a management turn cannot settle while an admissible engineering report addressed to it is undisposed. This mirrors the P2d2 directive-inbox completion guard (`docs/archive/texture-directive-inbox-design-2026-10-11.md`).
   - That guard is substrate (an obligation-closure invariant), not judgement, because it does not pick the content.
3. **Red: if Jev is used, it is management's optional system-1.** Management chooses whether to consult Jev from a cell (a choir verb wrapping the gateway judgment). The runtime does not call Jev on its own. Delete the reconcile-driven call at `engineering_desk.go:405` (§2, §6).
4. **Red: dissolve path B.** Delete the retarget and `textureSupervisionSubject` and its three call sites, the Texture-side `supervisionOnly` and prefix plumbing, `desk:"engineering"` lifecycle documents, the engineering branch of `ownerRevisionTarget`, `ReconcileEngineeringRevisionCast`, `ReconcileEngineeringDesk`, `reconcileEngineeringCast` (including the restart recast), `selfdev_texture_join.go`, and the document-cast branch of run acceptance (`run_acceptance.go:174-188`). M11's start moves to owner → Texture → management Cast. The earlier routing review estimates this at about 800 LOC.
5. **Green: prompts.** Rewrite `management.yaml:11` and `rlm_management_runtime.yaml:28` (M1), teach `work_disposition` on `ReportPacket` (M3), and fix the wake text (M4).

**Restart rule** [I, important]. The restart-recast branch (`engineering_desk.go:213-279`) re-opens an assignment whose capsule a restart revoked (reason written at `engineering_assignment_fate.go:541`) and checks **no planned-restart marker**. A crash restart therefore resumes engineering work, up to `engineeringMaxRecastAttempts=3` (`:50`). That contradicts CLAUDE.md ("A crash restart never resumes work"). Deleting path B removes it.

---

## 5. Self-development as a code-level concept

**Keep (genuine guest/host code-layer update machinery), rename toward "update" or "code-update":**

- **Bundle and verifier:**
  - Freeze → `CapsuleEffectBundle` (`capsule/transaction/builder.go:26`; `capsule/executor.go:931, 1399, 1621`).
  - The verifier-slot read-only bundle mount (`/selfdev/bundle`: `yaegikernel/inspect_bundle.go:22`, `capsule/executor.go:358`, `engineering_assignment_runtime.go:659`).
  - Verify intent recording (`rlm_reduce.go:1388-1420`).
  - The independent verifier certificate (`internal/verifierprotocol/verifier.go`, 100 LOC).
- **Decision, apply and restart:**
  - The signed owner/qualified-consensus decision binding (`selfdevprotocol/control.go:25-64`; `agentcore/self_development_decision_binding.go`, 139 LOC).
  - Updater apply, journal and rollback (`internal/updater`, about 2.9k LOC).
  - `self_development_materializer.go` apply, rollback and crash close (`:88`).
  - The planned-restart marker (`planned_restart.go`).
  - Host platform-update push (`platform_update.go`, which says "no owner decision, no selfdev operation", `:30`).
- **Guest control and routing:**
  - Guest control credentials, checkpoint publish and route projection (`internal/selfdev/credentials.go`, `restart_handoff.go`).
  - `vmctl/self_development_route.go`.
  - Restore, rematerialize and tape completeness (`internal/selfdevprotocol`, about 1.07k LOC).
  - Genesis baseline and rollback routes.
- **Policy and lifecycle routes:**
  - The host-signed mode gate (`platform/self_development_modes.go`). Recast it as an "update policy".
  - The lifecycle routes currently hosted under the self-development router (`api_self_development.go:111-145`: replace-workspace, bootstrap-chain, rematerialize, restore, checkpoint, import-residue, producer-reports). Move them out of the selfdev router.

**Dissolve (concept-level scaffolding):**

- **The public start-operation API, `startSelfDevelopmentOperation`** (`api_self_development.go:272`). It writes its own trajectory event as `ActorProfile=Management` with authority "public-self-development-api" (`:368-376`), plus `ensureSelfDevelopmentRun` and `selfdev_texture_join.go` (123 LOC). This is a second owner→engineering entry that duplicates owner→Texture→management→engineering.
- **The 13-state operation machine** (`selfdev/operations.go:17-31, 621-650`). Specifically:
  - the one-open-operation-per-computer lock (`:41`; `api_self_development.go:345-351`);
  - the runtime-driven auto-advance and auto-fail coupling: `failBoundSelfdevOperation` at `engineering_desk.go:244, 272, 286, 363, 392` and `engineering_assignment_fate.go:345`; verified→awaiting_approval replay at `engineering_desk.go:332-336`; requested→executing at `api_self_development.go:1222` and `freeze_candidate.go:59`.
  - Keep the row's receipt fields (heads, verifier refs, decision, materialization and route receipts) as a durable record, or derive them from events.
- **The Jev candidate judge and the automatic judge-on-reconcile** (`selfdev_candidate_judgment.go`, 218 LOC; `engineering_desk.go:401-406`).
- **The runtime-written management observation** `notifySelfDevelopmentDecisionBoundary` (`self_development_materializer.go:858-887`), which writes into management's commitment ledger as "selfdev-reconciler".
- **Prompt special-casing:** `tool_profiles.go:247-249` (Texture supervision), `:259-272` (`HasSelfDevelopmentOperation`/`CanProposeChange`), and the selfdev wording in `rlm_engineering_runtime.yaml:54, 60`. Textual "self-development" mentions in `run_system.yaml:14` and `revision_policy.yaml:19` are a green word change.
- **Texture metadata joins:** `textureowner/texture_task_types.go:22`, `texture_evidence_sources.go:611, 818-849`, `texture_revision_metadata.go:70-71`; the management operation-id stamp (`management_controller.go:373`); and acceptance keyed on the operation (`run_acceptance.go:651-680`, classed C below).
- **Dead code:** `store.ListRunsBySelfDevelopmentOperation` (`store/store.go:1608`), which only tests call.

**Unclear (C):**

- Whether a freeze should auto-open a candidate operation (`freeze_candidate.go:23`).
- Auth scope names `computer:self_development:*` (`auth/handlers.go:1540-1545`; `frontend/src/lib/SettingsApp.svelte:46-64, 177`).
- Health counters (`api.go:245-250, 950-966`).
- Acceptance freeze checkpoints.

---

## 6. Prioritized deletion list

Ordered by value over risk. Each entry gives its mutation class and rollback (git revert unless noted). Red entries need full ceremony.

| P | Change | Class | Why first | Rollback |
|---|---|---|---|---|
| 0 | **Restart-recast without a planned-restart marker** (`engineering_desk.go:213-279`). Document it first (Problem Documentation First), then gate it on the marker or delete it with path B (row 8). | red (vmctl-adjacent restart semantics, run lifecycle) | Possible live violation of a standing CLAUDE.md rule, independent of I1/I2 [I]. | revert |
| 1 | **Prompt truth pass** (green; plus the yellow test below). Fix M1-M4, M6, M7, M9-M11, M14, E2-E5, E9 wording, R2, R5-R6, X1-X2. Delete the dead text: `buildPersistentManagementUpdatePrompt`, `buildCoagentBacklogPrompt`/`buildAssignedWorkItemPrompt`, `buildTextureMediaSourceResearchObjective`. Correct the Context summary or make Context return what it promises. | green (the Context code change is orange) | Cheapest I1 win. M3 and M4 actively teach a broken completion path. | revert |
| 2 | **Delete `choir.Assign`** (§3), plus remove Message, Emit and Emits from all module sets and from the reducer's `IntentMessage` arm. | yellow/orange | False affordance and bug surface. The owner has already decided. | revert; no state |
| 3 | **Prune module sets to intent.** Drop Cast and Ask from engineering, Cast from research, and the dead acts (Report family) from Texture unless they are wanted. Add a reverse parity test: every export is taught or explicitly listed, and every taught addressing rule matches `needTo`. | orange | Enforces "engineering never addresses Texture" structurally (E4) and makes the generated surface the single truth. | revert |
| 4 | **Delete the research `next_instruction` injection** (`researchtools.go:717-738, 783-788, 924, 995, 1023`) and the legacy `buildRegistryForRole` registrations for management (`tool_profiles.go:465-470`; M16). | orange | Runtime-authored instruction naming a nonexistent tool (R1). | revert |
| 5 | **Delete the Texture research round cap** (`texture_turn_runtime.go:155-187, 295-305, 405`) and its prompt text (`tool_profiles.go:252`; `run_system.yaml:10, 31`). | orange (Texture dispatch is next to a red surface; it does not touch canonical writes) | Explicit owner rejection. Circuit breakers are deferred, not preemptive. | revert |
| 6 | **Delete the research egress budget ledger as a policy** (`egress_budget.go:46-47`, `tool_profiles.go:476-481`) and the prompt text (`research.yaml:44-46`, overlay `:18`). Keep the SSRF and address guards in `sourcefetch`. | red (gateway/provider calls) | The owner rejects budgets. Gateway rate limiting stays as substrate on the gateway side. | revert |
| 7 | **Management relay from binding plus pass-through, never hold** (§4 steps 1-2). | red (lifecycle reducer, evidence ledger) | Makes management the translator; fixes M2 and the run-dependent relay. | revert; already-written records stay readable |
| 8 | **Dissolve document-cast path B** (§4 step 4), including the crash-resuming restart recast and `selfdev_texture_join.go`. Move M11's start onto Texture → management. | red (run acceptance, Texture inputs, lifecycle) | Removes direct engineering→Texture, a crash-resume violation, and about 800 LOC. | revert; in-flight document casts get the "interrupted/superseded" fate at the next boot |
| 9 | **Remove the runtime-owned Jev judge and auto-judge** (`selfdev_candidate_judgment.go`; `engineering_desk.go:401-406`). Expose judgment as an optional management verb if wanted. | red (gateway calls, verification evidence) | Runtime scoring is rejected ("scoring requires intelligence"). | revert |
| 10 | **Collapse the selfdev state machine to receipts; rename the keepers to "update"** (§5). Remove the start-operation API, one-open lock, auto-advance/fail, `notifySelfDevelopmentDecisionBoundary`, and `ListRunsBySelfDevelopmentOperation`. | red (checkpoint/route projection adjacency, run acceptance) | Concept recedes; machinery stays. | revert; keep the store columns readable |

**Heresy delta for this research:**

- **Discovered:**
  - Management's report toDesk is ignored, and on drain carriers the target is a runtime heuristic (M2).
  - The live wake text teaches `work_item_id` and `work_disposition` on `choir.Report`, which the API cannot express and the reducer forbids (M4).
  - The research cell gets an `update_coagent` `next_instruction` (R1).
  - The `desk_go_eval` batching claim is false (M7).
  - The `/api/computers/` ProductAPI entry is dead (M9, correcting a prior doc).
  - Engineering is told to address an agent_id the reducer refuses (E3).
  - Engineering's Cast and Ask are ungated (E4, I).
  - A crash-resuming restart recast exists (§4, I).
  - The relay depends on the run (§4, I).
- **Introduced:** none.
- **Repaired:** none.
